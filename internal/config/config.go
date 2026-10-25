package config

import (
	"bytes"
	"fmt"
	"github.com/kgsaran/trackfw/internal/homedir"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

const (
	NamespacingFlat    = "flat"
	NamespacingByAgent = "by_agent"

	// LenientDefaultDays is the default number of days added to today when writing
	// lenient_until in trackfw init (brownfield) and trackfw discover. Both sites must use
	// this constant so the contract stays consistent.
	LenientDefaultDays = 30

	// LenientHorizonDays is the maximum number of days from the evaluation instant that
	// lenient_until may sit in the future. A date further than this (e.g. 9999-12-31) is
	// indistinguishable from "lenient forever" and is treated the same as absent — strict.
	// Rationale: 730 days (≈2 years) is long enough for any realistic transition period and
	// forces an explicit renewal decision before the mode can run indefinitely.
	LenientHorizonDays = 730
)

// ScalarTypeError records a trackfw.yaml key that received an unexpected type. The parse
// continues with the key treated as absent (backward-compatible). The validator emits a
// config_value_type violation for each entry. ML-1B/ML-1C, REQ-2026-10-24.
//
// Key uses the qualified form for nested keys (e.g. "credential_guard.mode", "rules.wip_limit").
// Got is the type received: "scalar", "sequence", or "mapping".
// Expected is the type the key requires: "scalar string", "sequence", or "mapping".
type ScalarTypeError struct {
	Key      string // field name in trackfw.yaml (qualified for nested)
	Got      string // type received: "scalar", "sequence", or "mapping"
	Expected string // type required: "scalar string", "sequence", or "mapping"
}

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

	// write_scope field — see ADR-2026-10-23 D3, ML-3A.
	WriteScope WriteScopeConfig

	// agent_models field — see ADR-2026-08-21-versao-do-modelo-por-tier-com-composicao-por-alvo.md.
	// Maps tier name (e.g. "opus", "sonnet") to version string (e.g. "5", "4.6").
	// Absent or empty map → behavior identical to today (tier alias used verbatim).
	AgentModels map[string]string

	// ScalarTypeErrors records all trackfw.yaml keys that received an unexpected type.
	// Scalar keys use stringValRec (table loop). List keys use stringListRec. Nested scalar
	// keys use stringValNested. Container keys and map values recorded inline in parse().
	// ML-1B/ML-1C, REQ-2026-10-24.
	ScalarTypeErrors []ScalarTypeError
}

// CredentialGuardConfig holds the credential_guard.* fields read from trackfw.yaml — see
// ADR-2026-08-05-hook-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes.md.
// Read via a nested mapping under the flat root, same pattern as SyncConfig.
type CredentialGuardConfig struct {
	Mode string // credential_guard.mode: "warn" (default) | "block"
}

// WriteScopeConfig holds the write_scope.* fields read from trackfw.yaml.
// Paths in these lists are appended to the defaults — they never remove the hardcoded list.
// ADR-2026-10-23 D3, ML-3A contract item 6.
type WriteScopeConfig struct {
	FrozenPaths         []string // write_scope.frozen_paths: extra paths frozen for all sessions
	FrozenPathsSubagent []string // write_scope.frozen_paths_subagent: extra paths frozen for subagents only
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

	// ci_trigger field — ADR-2026-08-28 adendo 2026-10-24, REQ-2026-10-24.
	// Accepted values: "pull_request" (default, absent = pull_request) and "manual".
	// CITriggerInvalid is true when the key is present with a value outside the enum
	// or with a non-scalar type (sequence/mapping). CITrigger is "" in that case.
	// CITriggerRawValue holds the raw string for error messages; for non-scalars it holds
	// a human-readable type description.
	CITrigger        string // ci_trigger: pull_request|manual (default: "" = pull_request)
	CITriggerInvalid bool   // true when ci_trigger has an unrecognised or non-scalar value
	CITriggerRawValue string // raw value for violation messages
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

// ParseWriteScopeConfig parses only the write_scope.* fields from arbitrary trackfw.yaml content.
// Returns an error when content is not valid YAML (duplicate keys, syntax errors).
// The caller must treat any error as a deny (fail-closed, D3-F).
// Used by the write-scope guard, which cannot call Load() (Load calls osExit(1) on malformed YAML).
// Like ParseRulesFromContent, reuses parse() so the read logic cannot diverge.
//
// Decision 6 (ML-4C): yaml.Node used internally by parse() does not detect duplicate mapping
// keys; this function uses a first-pass yaml.Unmarshal into interface{} to catch those errors
// before invoking parse(). The two Unmarshal calls are consolidated here so callers see one
// clean API and are not required to do their own validation pass.
func ParseWriteScopeConfig(content string) (WriteScopeConfig, error) {
	// First pass: validate YAML syntax including duplicate mapping keys.
	// yaml.Node silently accepts duplicate keys; interface{} rejects them.
	var raw interface{}
	if err := yaml.Unmarshal([]byte(content), &raw); err != nil {
		return WriteScopeConfig{}, err
	}
	cfg := ProjectConfig{Rules: make(map[string]string)}
	parse(content, &cfg)
	return cfg.WriteScope, nil
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

// ParseDirsFromContent parses req_dir, roadmap_dir, and adr_dirs from an arbitrary YAML string
// (e.g. a git-ref blob obtained via `git show origin/main:./trackfw.yaml`, not the CWD file
// Load() reads). Missing keys fall back to defaults() so that a trackfw.yaml that only sets
// rules: still returns canonical dir paths rather than empty strings.
// Used by ML-2B (scope-redirect guard) to compare origin/main's dir config against disk.
func ParseDirsFromContent(content string) (reqDir, roadmapDir string, adrDirs []string) {
	cfg := defaults()
	parse(content, &cfg)
	return cfg.REQDir, cfg.RoadmapDir, cfg.ADRDirs
}

// ReadAgentConventions reads the `agent_conventions` key directly out of <cwd>/trackfw.yaml,
// bypassing the Load() singleton — same isolation pattern as ParseRulesFromContent, needed here
// because generators (agentfiles.go) inject rules into agent files for a given cwd that is not
// necessarily the process's own working directory the singleton is cached against. The file
// being absent, unreadable, malformed or simply missing the key are all treated the same: return
// "" silently, never an error — this is a best-effort read of an optional, free-text field.
// ReadNamespacingConfig reads roadmap_namespacing and agents from <cwd>/trackfw.yaml,
// bypassing the Load() singleton — same isolation pattern as ReadAgentConventions.
// Returns ("flat", nil) when the file is absent, unreadable, or missing those fields.
// Used by generators that write project files (CLAUDE.md, agentfiles) for an arbitrary cwd.
func ReadNamespacingConfig(cwd string) (namespacing string, agents []string) {
	data, err := os.ReadFile(filepath.Join(cwd, "trackfw.yaml"))
	if err != nil {
		return "flat", nil
	}
	cfg := ProjectConfig{Rules: make(map[string]string), AgentModels: map[string]string{}}
	parse(string(data), &cfg)
	return cfg.RoadmapNamespacing, cfg.Agents
}

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

	// --- Scalar keys — driven by scalarKeyTable; no literal-keyed reads here.
	// The table is the only place to add a new scalar key. ML-1C, REQ-2026-10-24.
	for _, spec := range scalarKeyTable {
		if v, ok := stringValRec(m, spec.key, &cfg.ScalarTypeErrors); ok {
			spec.setter(cfg, v)
		}
	}

	// --- adr_dirs: list of paths (with tilde expansion). Recorded via stringListRec.
	if v, ok := m["adr_dirs"]; ok {
		if items, ok := stringListRec(v, "adr_dirs", &cfg.ScalarTypeErrors); ok {
			for i, s := range items {
				items[i] = ExpandPath(s)
			}
			cfg.ADRDirs = items
		}
	}

	// --- agents: list of agent names. Recorded via stringListRec.
	if v, ok := m["agents"]; ok {
		if items, ok := stringListRec(v, "agents", &cfg.ScalarTypeErrors); ok {
			cfg.Agents = items
		}
	}

	// --- acceptance_markers: list of heading strings. Recorded via stringListRec.
	if v, ok := m["acceptance_markers"]; ok {
		if items, ok := stringListRec(v, "acceptance_markers", &cfg.ScalarTypeErrors); ok {
			cfg.AcceptanceMarkers = items
		}
	}

	// --- link_fields: nested mapping of lists. Container-type recorded; each sub-list recorded.
	if v, ok := m["link_fields"]; ok {
		recordContainerType(v, "link_fields", &cfg.ScalarTypeErrors)
		if lf, ok := v.(map[string]interface{}); ok {
			if items, ok := stringListRec(lf["req"], "link_fields.req", &cfg.ScalarTypeErrors); ok {
				cfg.LinkFieldsReq = items
			}
			if items, ok := stringListRec(lf["adr"], "link_fields.adr", &cfg.ScalarTypeErrors); ok {
				cfg.LinkFieldsADR = items
			}
			if items, ok := stringListRec(lf["roadmap"], "link_fields.roadmap", &cfg.ScalarTypeErrors); ok {
				cfg.LinkFieldsRoadmap = items
			}
		}
	}

	// --- rules: mapping of rule name → severity string.
	// Container-type recorded. Values are iterated in sorted order for deterministic output.
	// A non-string value (e.g. rules.wip_limit: [error]) is recorded with the qualified name.
	// ML-1C, REQ-2026-10-24.
	if v, ok := m["rules"]; ok {
		recordContainerType(v, "rules", &cfg.ScalarTypeErrors)
		if rm, ok := v.(map[string]interface{}); ok {
			keys := make([]string, 0, len(rm))
			for k := range rm {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				rv := rm[k]
				if s, ok := rv.(string); ok {
					cfg.Rules[k] = s
				} else if rv != nil {
					var got string
					switch rv.(type) {
					case []interface{}:
						got = "sequence"
					default:
						got = "mapping"
					}
					cfg.ScalarTypeErrors = append(cfg.ScalarTypeErrors,
						ScalarTypeError{Key: "rules." + k, Got: got, Expected: "scalar string"})
				}
			}
		}
	}

	// --- ci_trigger — REQ-2026-10-24, ADR-2026-08-28 adendo 2026-10-24.
	// Direct presence check (not stringValRec) to distinguish absent (no violation) from
	// present-but-non-scalar. ci_trigger_invalid owns the non-scalar case for this key;
	// config_value_type does not fire for ci_trigger (no double violation). ML-1C.
	if raw, present := m["ci_trigger"]; present {
		if s, ok := raw.(string); ok {
			if s == "" || s == "~" {
				// Empty string (bare key / ci_trigger: "") or "~" (YAML explicit
				// null — yaml.v3 ScalarNode.Value == "~" for ci_trigger: ~).
				// Treat as absent: default pull_request applies, no violation. ML-1E R-2.
			} else {
				cfg.Update.CITriggerRawValue = s
				if s == "pull_request" || s == "manual" {
					cfg.Update.CITrigger = s
				} else {
					cfg.Update.CITriggerInvalid = true
				}
			}
		} else {
			// Non-scalar: sequence or mapping — ci_trigger_invalid owns this path.
			cfg.Update.CITriggerInvalid = true
			cfg.Update.CITriggerRawValue = fmt.Sprintf("non-scalar (%T)", raw)
		}
	}

	// --- credential_guard: nested mapping. Container-type recorded.
	// credential_guard.mode is read via stringValNested so a non-string mode is recorded.
	// A valid mode that is not in {"warn","block"} falls back to the default ("warn") silently —
	// this preserves the enum-guard behavior from ML-1A; the recording is purely for type errors.
	if v, ok := m["credential_guard"]; ok {
		recordContainerType(v, "credential_guard", &cfg.ScalarTypeErrors)
		if cg, ok := v.(map[string]interface{}); ok {
			if mode, ok := stringValNested(cg, "mode", "credential_guard.mode", &cfg.ScalarTypeErrors); ok {
				if mode == "warn" || mode == "block" {
					cfg.CredentialGuard.Mode = mode
				}
			}
		}
	}

	// --- write_scope: nested mapping with two list keys. Container-type and list-types recorded.
	// ADR-2026-10-23 D3, ML-3A contract item 6.
	if v, ok := m["write_scope"]; ok {
		recordContainerType(v, "write_scope", &cfg.ScalarTypeErrors)
		if ws, ok := v.(map[string]interface{}); ok {
			if items, ok := stringListRec(ws["frozen_paths"], "write_scope.frozen_paths", &cfg.ScalarTypeErrors); ok {
				cfg.WriteScope.FrozenPaths = items
			}
			if items, ok := stringListRec(ws["frozen_paths_subagent"], "write_scope.frozen_paths_subagent", &cfg.ScalarTypeErrors); ok {
				cfg.WriteScope.FrozenPathsSubagent = items
			}
		}
	}

	// --- agent_models: flat mapping from tier name to version string.
	// Container-type recorded. Values are iterated in sorted order for deterministic output.
	// A non-string value is recorded with the qualified name. ML-1C, REQ-2026-10-24.
	// See ADR-2026-08-21-versao-do-modelo-por-tier-com-composicao-por-alvo.md.
	if v, ok := m["agent_models"]; ok {
		recordContainerType(v, "agent_models", &cfg.ScalarTypeErrors)
		if am, ok := v.(map[string]interface{}); ok {
			keys := make([]string, 0, len(am))
			for k := range am {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				rv := am[k]
				if s, ok := rv.(string); ok {
					cfg.AgentModels[k] = s
				} else if rv != nil {
					var got string
					switch rv.(type) {
					case []interface{}:
						got = "sequence"
					default:
						got = "mapping"
					}
					cfg.ScalarTypeErrors = append(cfg.ScalarTypeErrors,
						ScalarTypeError{Key: "agent_models." + k, Got: got, Expected: "scalar string"})
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

// scalarKeySpec maps a top-level trackfw.yaml scalar key to its setter function.
// KnownScalarKeys() ranges over this table — so the exported list is derived by
// construction from the reads, with no manual copy. Adding a key here is the only
// required edit; parse() picks it up via the table loop. ML-1C, REQ-2026-10-24.
//
// Excluded: ci_trigger (direct presence check, owned by ci_trigger_invalid).
// Excluded: credential_guard.mode (nested scalar, read by stringValNested).
type scalarKeySpec struct {
	key    string
	setter func(cfg *ProjectConfig, v string)
}

var scalarKeyTable = []scalarKeySpec{
	{"req_dir", func(c *ProjectConfig, v string) { c.REQDir = v }},
	{"roadmap_dir", func(c *ProjectConfig, v string) { c.RoadmapDir = v }},
	{"roadmap_namespacing", func(c *ProjectConfig, v string) { c.RoadmapNamespacing = v }},
	{"governance_mode", func(c *ProjectConfig, v string) { c.GovernanceMode = v }},
	{"lenient_until", func(c *ProjectConfig, v string) { c.LenientUntil = v }},
	{"wip_limit", func(c *ProjectConfig, v string) { c.WipLimit = parseInt(v, c.WipLimit) }},
	{"wip_by_squad", func(c *ProjectConfig, v string) { c.WipBySquad = v == "true" }},
	{"stale_wip_days", func(c *ProjectConfig, v string) { c.StaleWIPDays = parseInt(v, c.StaleWIPDays) }},
	{"require_req_in_commit", func(c *ProjectConfig, v string) { c.RequireReqInCommit = v == "true" }},
	{"trace_id_field", func(c *ProjectConfig, v string) { c.TraceIdField = v }},
	{"strict_ci_paths", func(c *ProjectConfig, v string) { c.StrictCIPaths = v == "true" }},
	{"forge", func(c *ProjectConfig, v string) { c.Forge = v }},
	{"hooks", func(c *ProjectConfig, v string) { c.Update.Hooks = v }},
	{"ci", func(c *ProjectConfig, v string) { c.Update.CI = v }},
	{"backend", func(c *ProjectConfig, v string) { c.Update.Backend = v }},
	{"frontend", func(c *ProjectConfig, v string) { c.Update.Frontend = v }},
	{"pkg_manager", func(c *ProjectConfig, v string) { c.Update.PkgManager = v }},
	{"agent_conventions", func(c *ProjectConfig, v string) { c.Update.AgentConventions = v }},
	{"linear_api_key", func(c *ProjectConfig, v string) { c.Sync.LinearAPIKey = v }},
	{"linear_team_id", func(c *ProjectConfig, v string) { c.Sync.LinearTeamID = v }},
	{"jira_base_url", func(c *ProjectConfig, v string) { c.Sync.JiraBaseURL = v }},
	{"jira_email", func(c *ProjectConfig, v string) { c.Sync.JiraEmail = v }},
	{"jira_token", func(c *ProjectConfig, v string) { c.Sync.JiraToken = v }},
	{"jira_project", func(c *ProjectConfig, v string) { c.Sync.JiraProject = v }},
}

// KnownScalarKeys returns the top-level trackfw.yaml scalar keys registered in scalarKeyTable.
// Derived by construction — adding a key to scalarKeyTable automatically appears here.
// Used by the exhaustive tests in config_scalar_type_test.go and config_ast_coverage_test.go.
// ML-1B/ML-1C, REQ-2026-10-24.
func KnownScalarKeys() []string {
	out := make([]string, len(scalarKeyTable))
	for i, s := range scalarKeyTable {
		out[i] = s.key
	}
	return out
}

// stringValRec reads a scalar string field from a normalized map.
// When the key is present but not a string (sequence or mapping), it appends a
// ScalarTypeError to *errs. Called only from the scalarKeyTable loop in parse(),
// not directly with a string literal. ML-1B/ML-1C, REQ-2026-10-24.
func stringValRec(m map[string]interface{}, key string, errs *[]ScalarTypeError) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		var got string
		switch v.(type) {
		case []interface{}:
			got = "sequence"
		default:
			got = "mapping"
		}
		*errs = append(*errs, ScalarTypeError{Key: key, Got: got, Expected: "scalar string"})
	}
	return s, ok
}

// stringValNested reads a scalar string field from a nested (non-top-level) normalized map,
// recording an error under the qualified key name (e.g. "credential_guard.mode").
// When the key is present but not a string, it appends a ScalarTypeError with the
// qualified name to *errs. ML-1C, REQ-2026-10-24.
func stringValNested(m map[string]interface{}, key, qualifiedKey string, errs *[]ScalarTypeError) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		var got string
		switch v.(type) {
		case []interface{}:
			got = "sequence"
		default:
			got = "mapping"
		}
		*errs = append(*errs, ScalarTypeError{Key: qualifiedKey, Got: got, Expected: "scalar string"})
	}
	return s, ok
}

// stringListRec reads a sequence from an arbitrary value (already extracted from the
// normalized map), recording a ScalarTypeError when the value is a non-null scalar or
// a mapping instead of the expected sequence. A nil or empty-string value (YAML
// null / bare key) is treated as absent — no error recorded. ML-1C, REQ-2026-10-24.
func stringListRec(v interface{}, key string, errs *[]ScalarTypeError) ([]string, bool) {
	if v == nil {
		return nil, false // absent
	}
	// normalizeNode maps YAML !!null scalars (bare key / ~) to "" — treat as absent.
	if s, ok := v.(string); ok && s == "" {
		return nil, false
	}
	items, ok := v.([]interface{})
	if !ok {
		var got string
		switch v.(type) {
		case string:
			got = "scalar"
		default:
			got = "mapping"
		}
		*errs = append(*errs, ScalarTypeError{Key: key, Got: got, Expected: "sequence"})
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

// recordContainerType records a ScalarTypeError when a key that must hold a mapping
// receives a scalar or a sequence instead. nil and "" (YAML null) are treated as
// absent (no error). ML-1C, REQ-2026-10-24.
func recordContainerType(v interface{}, key string, errs *[]ScalarTypeError) {
	if v == nil {
		return
	}
	if s, ok := v.(string); ok && s == "" {
		return // YAML null / bare key — treat as absent
	}
	if _, ok := v.(map[string]interface{}); ok {
		return // correct type
	}
	var got string
	switch v.(type) {
	case []interface{}:
		got = "sequence"
	default:
		got = "scalar"
	}
	*errs = append(*errs, ScalarTypeError{Key: key, Got: got, Expected: "mapping"})
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

// RoadmapStates returns the canonical, closed list of roadmap state names in
// alphabetical order. Callers that only need set membership (e.g. building a
// map[string]bool or iterating all directories) should consume this function.
//
// The order is the lifecycle order: backlog → analyzing → wip → blocked → done → abandoned.
// This order IS contract: it appears in `trackfw status` output (via inventoryBlock) and
// is the reference set for all per-state directory scans. Changing it changes what users see.
// Tests must assert the order, not just the set.
//
// Note: callers with a different priority order (e.g. `trackfw context`, which uses
// contextRoadmapStateOrder with wip-first) keep their own ordered slice and must have a
// set-equality test against RoadmapStates().
//
// Callers with a different order-semantic (e.g. priority search where "wip"
// must come first) must keep their own ordered slice; they must, however, have
// a test that asserts their slice contains the same elements as RoadmapStates().
//
// The function returns a new slice on every call so that callers cannot
// accidentally mutate the shared canonical list.
func RoadmapStates() []string {
	return []string{"backlog", "analyzing", "wip", "blocked", "done", "abandoned"}
}
