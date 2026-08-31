package config

import (
	"bytes"
	"fmt"
	"github.com/kgsaran/trackfw/internal/homedir"
	"os"
	"path/filepath"
	"reflect"
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

	// ML-1A namespaces — see ADR-2026-08-02-caminho-unico-de-leitura-do-trackfw-yaml-com-
	// namespaces-tipados.md. Keys stay flat at the YAML root; these are memory-only groupings
	// populated by the same single parse() below, not a second read of trackfw.yaml.
	Update UpdateConfig
	Sync   SyncConfig

	// credential_guard field — see ADR-2026-08-05-hook-de-guarda-contra-materializacao-de-
	// credenciais-reais-por-subagentes.md.
	CredentialGuard CredentialGuardConfig

	// agent_models field — see ADR-2026-08-21-versao-do-modelo-por-tier-com-composicao-por-alvo.md.
	// Maps tier name (e.g. "opus", "sonnet") to version string (e.g. "5", "4.6").
	// Absent or empty map → behavior identical to today (tier alias used verbatim).
	AgentModels map[string]string
}

// CredentialGuardConfig holds the credential_guard.* fields read from trackfw.yaml — see
// ADR-2026-08-05-hook-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes.md.
// Read via a nested mapping under the flat root, same pattern as SyncConfig.
type CredentialGuardConfig struct {
	Mode string // credential_guard.mode: "warn" (default) | "block"
}

// UpdateConfig holds the fields `trackfw update` reads to decide which git hooks and CI
// workflow to (re)generate. Absent fields default to "" in all three CLIs.
type UpdateConfig struct {
	Hooks      string // hooks: husky|native|... (default: "")
	CI         string // ci: github|gitlab|... (default: "")
	Backend    string // backend: ... (default: "")
	Frontend   string // frontend: ... (default: "")
	PkgManager string // pkg_manager: npm|yarn|pnpm|... (default: "")

	// ML-1A (agentes especialistas aceitam contexto de convenções) field
	AgentConventions string // agent_conventions: free-text, multi-line (default: "")
}

// SyncConfig holds the fields `trackfw sync` reads for Linear/Jira integration. linear_api_key
// and jira_token are mechanical preservation of the current behavior (secrets read from a
// versioned file) — see the ADR's "O que esta decisão explicitamente NÃO decide" section; this
// namespace does not endorse that design.
type SyncConfig struct {
	LinearAPIKey string // linear_api_key (default: "")
	LinearTeamID string // linear_team_id (default: "")
	JiraBaseURL  string // jira_base_url (default: "")
	JiraEmail    string // jira_email (default: "")
	JiraToken    string // jira_token (default: "")
	JiraProject  string // jira_project (default: "")
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

// AgentModelsSource indicates where agent_models was resolved from for a global-scope operation.
type AgentModelsSource int

const (
	// AgentModelsSourceNone: global config absent or has no agent_models; cwd also has none.
	AgentModelsSourceNone AgentModelsSource = iota
	// AgentModelsSourceGlobal: agent_models resolved from ~/.trackfw/trackfw.yaml.
	AgentModelsSourceGlobal
	// AgentModelsSourceProjectOnly: agent_models found in cwd's trackfw.yaml but not global.
	// The value is NOT used; this state triggers the AC4/AC14 advisory message.
	AgentModelsSourceProjectOnly
	// AgentModelsSourceGlobalMalformed: ~/.trackfw/trackfw.yaml exists but has invalid YAML.
	// Non-fatal: MalformedGlobalConfigMessage is emitted; canonical tier is used.
	AgentModelsSourceGlobalMalformed
)

// MalformedGlobalConfigMessage is written to stderr (non-fatal) when ~/.trackfw/trackfw.yaml
// exists but fails to parse as valid YAML. Unlike MalformedConfigMessage (project config, fatal),
// a malformed global config degrades gracefully to canonical tier.
// Must be byte-identical across Go, Node.js and Python — ADR-2026-08-23.
const MalformedGlobalConfigMessage = `trackfw: aviso: "~/.trackfw/trackfw.yaml" tem YAML malformado — config global de modelo ignorada; usando tier canônico.`

// GlobalAgentModelsNoneMessage is emitted to stderr when agent_models is not configured in the
// global config and not found in cwd's trackfw.yaml either. Non-fatal.
// Must be byte-identical across Go, Node.js and Python — ADR-2026-08-23.
const GlobalAgentModelsNoneMessage = `trackfw: agents global: agent_models não configurado em ~/.trackfw/trackfw.yaml — usando tier canônico. Configure em ~/.trackfw/trackfw.yaml para pinar versões.`

// GlobalAgentModelsProjectOnlyMessage is emitted to stderr when agent_models is found in the
// project's trackfw.yaml but not in the global config (AC4/AC14). The value is NOT applied.
// Must be byte-identical across Go, Node.js and Python — ADR-2026-08-23.
const GlobalAgentModelsProjectOnlyMessage = `trackfw: agents global: agent_models configurado em trackfw.yaml do projeto mas não vale para escopo global. Mova a chave para ~/.trackfw/trackfw.yaml.`

// LoadGlobalAgentModels reads agent_models from ~/.trackfw/trackfw.yaml, bypassing the Load()
// singleton. homeDir is the user home directory (e.g. from homedir.Dir()); cwd is the
// working directory used only for the AC14 diagnostic (detect "configured in project, not global").
//
// Never calls osExit. Returns an empty map and the appropriate AgentModelsSource for the caller
// to decide messaging and fallback. Pattern mirrors ReadAgentConventions above.
func LoadGlobalAgentModels(homeDir, cwd string) (map[string]string, AgentModelsSource) {
	globalPath := filepath.Join(homeDir, ".trackfw", "trackfw.yaml")
	data, err := os.ReadFile(globalPath)
	if err != nil {
		// Global file absent or unreadable → check if cwd trackfw.yaml has agent_models (AC14).
		return map[string]string{}, cwdAgentModelsSource(cwd)
	}
	// Global file exists — validate YAML (non-fatal for global config, per AC12).
	var probe yaml.Node
	if err := yaml.Unmarshal(data, &probe); err != nil || hasMultipleDocuments(data) {
		return map[string]string{}, AgentModelsSourceGlobalMalformed
	}
	cfg := ProjectConfig{Rules: make(map[string]string), AgentModels: map[string]string{}}
	parse(string(data), &cfg)
	if len(cfg.AgentModels) > 0 {
		return cfg.AgentModels, AgentModelsSourceGlobal
	}
	// Global file exists but has no agent_models → check if cwd has it (AC14).
	return map[string]string{}, cwdAgentModelsSource(cwd)
}

// cwdAgentModelsSource returns AgentModelsSourceProjectOnly if cwd's trackfw.yaml has
// agent_models configured, or AgentModelsSourceNone otherwise.
func cwdAgentModelsSource(cwd string) AgentModelsSource {
	if cwd == "" {
		return AgentModelsSourceNone
	}
	data, err := os.ReadFile(filepath.Join(cwd, "trackfw.yaml"))
	if err != nil {
		return AgentModelsSourceNone
	}
	cfg := ProjectConfig{Rules: make(map[string]string), AgentModels: map[string]string{}}
	parse(string(data), &cfg)
	if len(cfg.AgentModels) > 0 {
		return AgentModelsSourceProjectOnly
	}
	return AgentModelsSourceNone
}

// ResolveAgentModels returns the agent_models map and any advisory message for the given scope.
// For global scope, reads from ~/.trackfw/trackfw.yaml via LoadGlobalAgentModels.
// For project scope, reads from the cwd's trackfw.yaml via Load().
// The returned warnMsg is non-empty when the caller should emit an advisory to stderr.
// ResolveAgentModels never writes to stderr itself.
func ResolveAgentModels(scope, homeDir, cwd string) (models map[string]string, warnMsg string) {
	if scope != "global" {
		return Load().AgentModels, ""
	}
	m, source := LoadGlobalAgentModels(homeDir, cwd)
	switch source {
	case AgentModelsSourceNone:
		return m, GlobalAgentModelsNoneMessage
	case AgentModelsSourceProjectOnly:
		return m, GlobalAgentModelsProjectOnlyMessage
	case AgentModelsSourceGlobalMalformed:
		return m, MalformedGlobalConfigMessage
	default: // AgentModelsSourceGlobal
		return m, ""
	}
}

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

// ParseRulesFromContent parses only the `rules:` mapping out of arbitrary trackfw.yaml content
// (e.g. a git-HEAD blob obtained via `git show HEAD:./trackfw.yaml`, not the CWD file Load()
// reads) and returns it as name->severity. Used by the validator's HEAD-anchored credential-guard
// rules — see ADR-2026-08-12-severidade-das-regras-de-credential-guard-resolvida-pela-mais-estrita-
// entre-head-e-disco.md — which need `rules:` as it existed at a specific git ref, not the CWD, so
// they cannot go through the Load() singleton (always reads the CWD file and caches it once per
// process). Reuses parse() over an ephemeral ProjectConfig so this and Load() can never diverge on
// how `rules:` is read — purely additive, does not touch Load() or parse() themselves.
func ParseRulesFromContent(content string) map[string]string {
	cfg := ProjectConfig{Rules: make(map[string]string)}
	parse(content, &cfg)
	return cfg.Rules
}

// ReadAgentConventions reads the `agent_conventions` key directly out of <cwd>/trackfw.yaml,
// bypassing the Load() singleton — same isolation pattern as ParseRulesFromContent, needed here
// because generators (agentfiles.go) inject rules into agent files for a given cwd that is not
// necessarily the process's own working directory the singleton is cached against. The file
// being absent, unreadable, malformed or simply missing the key are all treated the same: return
// "" silently, never an error — this is a best-effort read of an optional, free-text field.
func ReadAgentConventions(cwd string) string {
	data, err := os.ReadFile(filepath.Join(cwd, "trackfw.yaml"))
	if err != nil {
		return ""
	}
	cfg := ProjectConfig{Rules: make(map[string]string), AgentModels: map[string]string{}}
	parse(string(data), &cfg)
	return cfg.Update.AgentConventions
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
		CredentialGuard: CredentialGuardConfig{
			Mode: "warn",
		},
		AgentModels: map[string]string{},
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

// initConfigMaps initializes all nil map fields of cfg using reflection.
// Called at the very start of parse() so that every code path — including
// early returns — leaves maps in a writable state, regardless of how the
// caller constructed the ProjectConfig.
//
// Using reflection instead of per-field nil checks means that when a new
// map field is added to ProjectConfig, the write in parse() is safe
// automatically — no second edit to an "initMaps" call site required.
// Alternative (per-field checks in parse, or a manual constructor) would
// require the developer to remember two edits: one for the write, one for
// the init; the reflection approach closes this class by making the
// invariant self-maintaining.
//
// Only the top-level fields of ProjectConfig are walked; nested config
// structs (UpdateConfig, SyncConfig, CredentialGuardConfig) contain no
// map fields and need no special handling.
func initConfigMaps(cfg *ProjectConfig) {
	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		fv := v.Field(i)
		if fv.Kind() == reflect.Map && fv.IsNil() {
			fv.Set(reflect.MakeMap(t.Field(i).Type))
		}
	}
}

// parse itself still tolerates yaml.Unmarshal failure by returning early (cfg keeps whatever
// defaults/prior state it had) — genuine syntax errors are caught and turned into a fatal exit
// one layer up, in Load, before parse is ever called with malformed content. This function only
// handles the benign cases: an absent/empty/comments-only document (len(root.Content) == 0,
// valid YAML that simply has no content) and a document whose top-level node parses fine but
// isn't a mapping (valid YAML, unexpected shape — e.g. a bare list) are both left as silent
// no-ops, since neither is a parse failure.
func parse(content string, cfg *ProjectConfig) {
	initConfigMaps(cfg) // guarantee: all map fields are non-nil before any write

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

	// ML-1A — Update and Sync namespaces. Same normalizeMapping result as above, no second read.
	if v, ok := stringVal(m, "hooks"); ok {
		cfg.Update.Hooks = v
	}
	if v, ok := stringVal(m, "ci"); ok {
		cfg.Update.CI = v
	}
	if v, ok := stringVal(m, "backend"); ok {
		cfg.Update.Backend = v
	}
	if v, ok := stringVal(m, "frontend"); ok {
		cfg.Update.Frontend = v
	}
	if v, ok := stringVal(m, "pkg_manager"); ok {
		cfg.Update.PkgManager = v
	}
	if v, ok := stringVal(m, "agent_conventions"); ok {
		cfg.Update.AgentConventions = v
	}
	if v, ok := stringVal(m, "linear_api_key"); ok {
		cfg.Sync.LinearAPIKey = v
	}
	if v, ok := stringVal(m, "linear_team_id"); ok {
		cfg.Sync.LinearTeamID = v
	}
	if v, ok := stringVal(m, "jira_base_url"); ok {
		cfg.Sync.JiraBaseURL = v
	}
	if v, ok := stringVal(m, "jira_email"); ok {
		cfg.Sync.JiraEmail = v
	}
	if v, ok := stringVal(m, "jira_token"); ok {
		cfg.Sync.JiraToken = v
	}
	if v, ok := stringVal(m, "jira_project"); ok {
		cfg.Sync.JiraProject = v
	}

	// credential_guard field — nested mapping, same shape as link_fields above. An unrecognized
	// mode value (or an absent/malformed credential_guard block) falls back to the safe default
	// ("warn") silently, matching how every other unrecognized-shape field in this parser behaves
	// (e.g. roadmap_namespacing, forge) — no fatal path, no stderr message, for one enum key.
	if v, ok := m["credential_guard"]; ok {
		if cg, ok := v.(map[string]interface{}); ok {
			if mode, ok := stringVal(cg, "mode"); ok && (mode == "warn" || mode == "block") {
				cfg.CredentialGuard.Mode = mode
			}
		}
	}

	// agent_models field — flat mapping from tier name to version string.
	// See ADR-2026-08-21-versao-do-modelo-por-tier-com-composicao-por-alvo.md.
	// An absent, malformed, or empty block leaves AgentModels as the empty map from defaults(),
	// preserving identical behavior to today. A key with an empty string value is stored as-is
	// (the render layer treats empty string as "no pin" — fall back to tier alias).
	if v, ok := m["agent_models"]; ok {
		if am, ok := v.(map[string]interface{}); ok {
			for k, rv := range am {
				if s, ok := rv.(string); ok {
					cfg.AgentModels[k] = s
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

// ExpandPath substitui o prefixo ~ ou ~/ pelo diretório home do usuário (homedir.Dir()).
// Se p não iniciar com ~ ou se falhar ao obter homeDir, retorna o caminho inalterado.
func ExpandPath(p string) string {
	if p == "~" {
		home, err := homedir.Dir()
		if err != nil {
			return p
		}
		return home
	}
	if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~\\") {
		home, err := homedir.Dir()
		if err != nil {
			return p
		}
		return filepath.Join(home, p[2:])
	}
	return p
}
