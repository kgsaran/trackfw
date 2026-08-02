package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

const (
	NamespacingFlat    = "flat"
	NamespacingByAgent = "by_agent"
)

// ProjectConfig holds all configurable paths and governance settings read from trackfw.yaml.
// Absent fields fall back to retrocompatible defaults (v1/v2 projects work unchanged).
type ProjectConfig struct {
	ADRDirs            []string // default: ["docs/adr"]
	REQDir             string   // default: "docs/req"
	RoadmapDir         string   // default: "docs/roadmaps"
	RoadmapNamespacing string   // "flat" (default) or "by_agent"
	Agents             []string // agent names when by_agent mode
	GovernanceMode     string   // "strict" or "lenient"
	LenientUntil       string   // date string YYYY-MM-DD
	WipLimit           int      // default: 1
	WipBySquad         bool
	StaleWIPDays       int // default: 7
	RequireReqInCommit bool

	// v2.4 fields
	LinkFieldsReq     []string          // default: ["REQ:"]
	LinkFieldsADR     []string          // default: ["ADR:"]
	LinkFieldsRoadmap []string          // default: ["Roadmap:"]
	AcceptanceMarkers []string          // default: ["## Acceptance Criteria", "## Critérios de Aceite"]
	Rules             map[string]string // governance rule severities

	// v2.5 fields
	TraceIdField string // frontmatter field for bidirectional REQ↔Roadmap tracing (default: "" = disabled)

	// ML-2A field
	StrictCIPaths bool // strict_ci_paths: true|false (default: false)

	// forge field (ship command)
	Forge string // "github", "gitlab", "bitbucket", "azure" or "" (auto-detect)
}

var (
	instance ProjectConfig
	once     sync.Once
)

// MalformedConfigMessage is written to stderr, verbatim, when trackfw.yaml exists but fails to
// parse as YAML. The wording is intentionally static (not built from the underlying library's
// error text): gopkg.in/yaml.v3, Node's `yaml` and PyYAML each report syntax errors in a
// different format, so the only way for the three CLIs to emit an identical message is for none
// of them to surface the library-native text. See ADR-2026-08-02-parsing-de-config-por-
// biblioteca-yaml-com-normalizacao-para-string-na-fronteira.md (ML-1B addendum).
const MalformedConfigMessage = "trackfw: erro ao carregar \"trackfw.yaml\": YAML malformado. Corrija a sintaxe do arquivo antes de continuar."

// osExit is a var (not a direct os.Exit call) so tests can override it and observe the fatal
// path without terminating the test process.
var osExit = os.Exit

// Load returns the singleton ProjectConfig, reading trackfw.yaml on first call.
// If trackfw.yaml is absent, empty or comments-only, retrocompatible defaults apply silently.
// If trackfw.yaml exists but is not valid YAML, Load prints MalformedConfigMessage to stderr
// and exits with status 1 — a config that cannot be parsed must not be silently discarded (see
// parse's doc comment for why this differs from a merely-unrecognized shape).
func Load() ProjectConfig {
	once.Do(func() {
		instance = defaults()
		data, err := os.ReadFile("trackfw.yaml")
		if err != nil {
			return
		}
		// Pre-check: yaml.Unmarshal into a throwaway node to detect genuine syntax errors
		// before parse() runs. Kept as a separate, cheap decode (config files are small)
		// rather than threading an error return through parse() and its ~20 direct test
		// call sites across the package — containment over signature purity here.
		//
		// hasMultipleDocuments is checked separately: yaml.Unmarshal only decodes the first
		// "---"-delimited document in a stream and silently ignores any trailing ones (no
		// error), while Node's `yaml` (MULTIPLE_DOCS) and PyYAML's yaml.compose() ("expected
		// a single document in the stream") both reject that shape outright — divergence
		// found by ML-1B's cross-CLI audit and closed here so Go doesn't silently read only
		// the first of two pasted-by-mistake documents.
		var probe yaml.Node
		if err := yaml.Unmarshal(data, &probe); err != nil || hasMultipleDocuments(data) {
			fmt.Fprintln(os.Stderr, MalformedConfigMessage)
			osExit(1)
			return
		}
		parse(string(data), &instance)
	})
	return instance
}

// Reset clears the singleton — for use in tests only.
func Reset() {
	once = sync.Once{}
	instance = ProjectConfig{}
}

func defaults() ProjectConfig {
	return ProjectConfig{
		ADRDirs:            []string{"docs/adr"},
		REQDir:             "docs/req",
		RoadmapDir:         "docs/roadmaps",
		RoadmapNamespacing: "flat",
		WipLimit:           1,
		StaleWIPDays:       7,
		LinkFieldsReq:      []string{"REQ:"},
		LinkFieldsADR:      []string{"ADR:"},
		LinkFieldsRoadmap:  []string{"Roadmap:"},
		AcceptanceMarkers:  []string{"## Acceptance Criteria", "## Critérios de Aceite"},
		Rules: map[string]string{
			"wip_has_req":                "error",
			"wip_acceptance":             "error",
			"wip_limit":                  "error",
			"stale_wip":                  "warning",
			"adr_orphan":                 "warning",
			"ref_targets_exist":          "error",
			"folder_status":              "warning",
			"filename_uniqueness":        "error",
			"blocked_by_draft_adr":       "error",
			"adr_accepted_when_req_done": "error",
		},
	}
}

// parse reads trackfw.yaml content with gopkg.in/yaml.v3 and applies the ~20 known keys onto
// cfg. Only the fields trackfw uses are consumed; unknown keys are ignored.
//
// Normalization to string on the fronteira: every scalar node is read via its raw textual
// value (Node.Value) instead of the value the library would coerce it to (bool/int/float/
// time.Time). This is what keeps "yes", "010" and "2026-08-02" arriving as the literal text
// instead of diverging typed values — see ADR-2026-08-02-parsing-de-config-por-biblioteca-yaml-
// com-normalizacao-para-string-na-fronteira.md. Aliases (*x) are resolved to their anchor's
// node before reading, or the raw text would be the anchor name instead of the value.
//
// hasMultipleDocuments reports whether data contains more than one "---"-delimited YAML
// document. yaml.Unmarshal silently decodes only the first document of a stream, so this check
// exists purely to make Load's fatal path agree with Node and Python on multi-document input —
// see the comment at Load's call site.
func hasMultipleDocuments(data []byte) bool {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(new(yaml.Node)); err != nil {
		return false // let the primary yaml.Unmarshal error (or lack thereof) drive Load
	}
	return dec.Decode(new(yaml.Node)) == nil
}

// parse itself still tolerates yaml.Unmarshal failure by returning early (cfg keeps whatever
// defaults/prior state it had) — genuine syntax errors are caught and turned into a fatal exit
// one layer up, in Load, before parse is ever called with malformed content. This function only
// handles the benign cases: an absent/empty/comments-only document (len(root.Content) == 0,
// valid YAML that simply has no content) and a document whose top-level node parses fine but
// isn't a mapping (valid YAML, unexpected shape — e.g. a bare list) are both left as silent
// no-ops, since neither is a parse failure.
func parse(content string, cfg *ProjectConfig) {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(content), &root); err != nil {
		return
	}
	if len(root.Content) == 0 {
		return
	}
	top := root.Content[0]
	if top.Kind != yaml.MappingNode {
		return
	}

	m := normalizeMapping(top)

	if v, ok := m["adr_dirs"]; ok {
		if items, ok := stringList(v); ok {
			for i, s := range items {
				items[i] = ExpandPath(s)
			}
			cfg.ADRDirs = items
		}
	}
	if v, ok := stringVal(m, "req_dir"); ok {
		cfg.REQDir = v
	}
	if v, ok := stringVal(m, "roadmap_dir"); ok {
		cfg.RoadmapDir = v
	}
	if v, ok := stringVal(m, "roadmap_namespacing"); ok {
		cfg.RoadmapNamespacing = v
	}
	if v, ok := m["agents"]; ok {
		if items, ok := stringList(v); ok {
			cfg.Agents = items
		}
	}
	if v, ok := stringVal(m, "governance_mode"); ok {
		cfg.GovernanceMode = v
	}
	if v, ok := stringVal(m, "lenient_until"); ok {
		cfg.LenientUntil = v
	}
	if v, ok := stringVal(m, "wip_limit"); ok {
		cfg.WipLimit = parseInt(v, cfg.WipLimit)
	}
	if v, ok := stringVal(m, "wip_by_squad"); ok {
		cfg.WipBySquad = v == "true"
	}
	if v, ok := stringVal(m, "stale_wip_days"); ok {
		cfg.StaleWIPDays = parseInt(v, cfg.StaleWIPDays)
	}
	if v, ok := stringVal(m, "require_req_in_commit"); ok {
		cfg.RequireReqInCommit = v == "true"
	}
	if v, ok := m["acceptance_markers"]; ok {
		if items, ok := stringList(v); ok {
			cfg.AcceptanceMarkers = items
		}
	}
	if v, ok := stringVal(m, "trace_id_field"); ok {
		cfg.TraceIdField = v
	}
	if v, ok := stringVal(m, "strict_ci_paths"); ok {
		cfg.StrictCIPaths = v == "true"
	}
	if v, ok := stringVal(m, "forge"); ok {
		cfg.Forge = v
	}
	if v, ok := m["link_fields"]; ok {
		if lf, ok := v.(map[string]interface{}); ok {
			if items, ok := stringList(lf["req"]); ok {
				cfg.LinkFieldsReq = items
			}
			if items, ok := stringList(lf["adr"]); ok {
				cfg.LinkFieldsADR = items
			}
			if items, ok := stringList(lf["roadmap"]); ok {
				cfg.LinkFieldsRoadmap = items
			}
		}
	}
	if v, ok := m["rules"]; ok {
		if rm, ok := v.(map[string]interface{}); ok {
			for k, rv := range rm {
				if s, ok := rv.(string); ok {
					cfg.Rules[k] = s
				}
			}
		}
	}
}

// normalizeMapping converts a *yaml.Node of Kind MappingNode into a map[string]interface{}
// whose values are strings (scalars), []interface{} of strings (sequences of scalars) or
// map[string]interface{} (nested mappings) — recursively, via normalizeNode.
func normalizeMapping(n *yaml.Node) map[string]interface{} {
	result := make(map[string]interface{}, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := resolveAlias(n.Content[i])
		v := n.Content[i+1]
		result[k.Value] = normalizeNode(v)
	}
	return result
}

// resolveAlias walks alias chains (b: *x) to the anchor's underlying node. Reading .Value on
// an unresolved AliasNode returns the anchor *name*, not the value — this is what would
// corrupt a: &x 3 / b: *x into b == "x" instead of b == "3" if skipped.
func resolveAlias(n *yaml.Node) *yaml.Node {
	for n.Kind == yaml.AliasNode && n.Alias != nil {
		n = n.Alias
	}
	return n
}

// normalizeNode converts a single *yaml.Node into a string (scalar, using the pre-coercion
// raw text in Node.Value), a []interface{} (sequence) or a map[string]interface{} (mapping).
func normalizeNode(n *yaml.Node) interface{} {
	n = resolveAlias(n)
	switch n.Kind {
	case yaml.ScalarNode:
		return n.Value
	case yaml.SequenceNode:
		items := make([]interface{}, 0, len(n.Content))
		for _, c := range n.Content {
			items = append(items, normalizeNode(c))
		}
		return items
	case yaml.MappingNode:
		return normalizeMapping(n)
	default:
		return nil
	}
}

// stringVal reads a scalar string field from a normalized map, tolerating callers that pass
// a non-string (e.g. a mapping under the same key by mistake) by reporting !ok.
func stringVal(m map[string]interface{}, key string) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// stringList converts a normalized sequence ([]interface{} of strings) into []string.
// A present-but-empty sequence yields a non-nil empty slice, distinguishing "present and
// empty" from "absent" for the caller (contract carried over from the inline-list fix).
func stringList(v interface{}) ([]string, bool) {
	items, ok := v.([]interface{})
	if !ok {
		return nil, false
	}
	result := make([]string, 0, len(items))
	for _, it := range items {
		if s, ok := it.(string); ok {
			result = append(result, s)
		}
	}
	return result, true
}

func parseInt(s string, def int) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return def
		}
		n = n*10 + int(c-'0')
	}
	if n == 0 {
		return def
	}
	return n
}

// ExpandPath substitui o prefixo ~ ou ~/ pelo diretório home do usuário (os.UserHomeDir()).
// Se p não iniciar com ~ ou se falhar ao obter homeDir, retorna o caminho inalterado.
func ExpandPath(p string) string {
	if p == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		return home
	}
	if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~\\") {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		return filepath.Join(home, p[2:])
	}
	return p
}
