package identity

import (
	"fmt"
	"strings"
)

// KnownAgentIDs returns the canonical list of agent ids known to trackfw, in
// a stable, deterministic order.
func KnownAgentIDs() []string {
	return []string{
		"architect",
		"backend",
		"frontend",
		"qa",
		"infra",
		"security",
		"dba",
		"ux",
		"code-quality",
		"data",
	}
}

// presets holds the catalog of themed identity presets. DisplayName and Slug
// are HARDCODED here — not derived by calling Slugify at runtime — because
// this is an explicit ADR decision: hardcoding avoids depending on Unicode
// normalization behaving identically across the Go, Node.js and Python CLIs.
var presets = map[string]map[string]AgentIdentity{
	"greek": {
		"architect":    {DisplayName: "Zeus", Slug: "zeus"},
		"backend":      {DisplayName: "Apolo", Slug: "apolo"},
		"frontend":     {DisplayName: "Afrodite", Slug: "afrodite"},
		"qa":           {DisplayName: "Ártemis", Slug: "artemis"},
		"infra":        {DisplayName: "Ares", Slug: "ares"},
		"security":     {DisplayName: "Hades", Slug: "hades"},
		"dba":          {DisplayName: "Poseidon", Slug: "poseidon"},
		"ux":           {DisplayName: "Atena", Slug: "atena"},
		"code-quality": {DisplayName: "Hefesto", Slug: "hefesto"},
		"data":         {DisplayName: "Métis", Slug: "metis"},
	},
	"norse": {
		"architect":    {DisplayName: "Odin", Slug: "odin"},
		"backend":      {DisplayName: "Thor", Slug: "thor"},
		"frontend":     {DisplayName: "Freya", Slug: "freya"},
		"qa":           {DisplayName: "Heimdall", Slug: "heimdall"},
		"infra":        {DisplayName: "Tyr", Slug: "tyr"},
		"security":     {DisplayName: "Vidar", Slug: "vidar"},
		"dba":          {DisplayName: "Njord", Slug: "njord"},
		"ux":           {DisplayName: "Idun", Slug: "idun"},
		"code-quality": {DisplayName: "Bragi", Slug: "bragi"},
		"data":         {DisplayName: "Mimir", Slug: "mimir"},
	},
	"potter": {
		"architect":    {DisplayName: "Dumbledore", Slug: "dumbledore"},
		"backend":      {DisplayName: "Snape", Slug: "snape"},
		"frontend":     {DisplayName: "Luna", Slug: "luna"},
		"qa":           {DisplayName: "Moody", Slug: "moody"},
		"infra":        {DisplayName: "Hagrid", Slug: "hagrid"},
		"security":     {DisplayName: "Kingsley", Slug: "kingsley"},
		"dba":          {DisplayName: "Flitwick", Slug: "flitwick"},
		"ux":           {DisplayName: "Tonks", Slug: "tonks"},
		"code-quality": {DisplayName: "Hermione", Slug: "hermione"},
		"data":         {DisplayName: "Trelawney", Slug: "trelawney"},
	},
	"thrones": {
		"architect":    {DisplayName: "Tyrion", Slug: "tyrion"},
		"backend":      {DisplayName: "Jon", Slug: "jon"},
		"frontend":     {DisplayName: "Sansa", Slug: "sansa"},
		"qa":           {DisplayName: "Arya", Slug: "arya"},
		"infra":        {DisplayName: "Brienne", Slug: "brienne"},
		"security":     {DisplayName: "Varys", Slug: "varys"},
		"dba":          {DisplayName: "Samwell", Slug: "samwell"},
		"ux":           {DisplayName: "Margaery", Slug: "margaery"},
		"code-quality": {DisplayName: "Stannis", Slug: "stannis"},
		"data":         {DisplayName: "Bran", Slug: "bran"},
	},
	"chaves": {
		"architect":    {DisplayName: "Girafales", Slug: "girafales"},
		"backend":      {DisplayName: "Madruga", Slug: "madruga"},
		"frontend":     {DisplayName: "Chiquinha", Slug: "chiquinha"},
		"qa":           {DisplayName: "Florinda", Slug: "florinda"},
		"infra":        {DisplayName: "Barriga", Slug: "barriga"},
		"security":     {DisplayName: "Quico", Slug: "quico"},
		"dba":          {DisplayName: "Clotilde", Slug: "clotilde"},
		"ux":           {DisplayName: "Popis", Slug: "popis"},
		"code-quality": {DisplayName: "Nhonho", Slug: "nhonho"},
		"data":         {DisplayName: "Godinez", Slug: "godinez"},
	},
}

// presetOrder is the canonical, stable order in which preset names are
// listed (e.g. by PresetNames and in error messages).
var presetOrder = []string{"greek", "norse", "potter", "thrones", "chaves"}

// Preset returns the identity Config for the given themed preset name (one
// of "greek", "norse", "potter", "thrones", "chaves"). If name is not a
// known preset, Preset returns an error listing the valid preset names.
//
// The returned Config is always a copy: mutating it does not affect the
// package-level preset table or any other Config returned by a previous
// call to Preset.
func Preset(name string) (Config, error) {
	agents, ok := presets[name]
	if !ok {
		return Config{}, fmt.Errorf("identity: preset desconhecido %q (validos: %s)", name, strings.Join(PresetNames(), ", "))
	}

	copied := make(map[string]AgentIdentity, len(agents))
	for id, agent := range agents {
		copied[id] = agent
	}

	return Config{
		SchemaVersion: schemaVersion,
		Agents:        copied,
	}, nil
}

// PresetNames returns the names of all known presets, in the stable order:
// greek, norse, potter, thrones, chaves.
func PresetNames() []string {
	names := make([]string, len(presetOrder))
	copy(names, presetOrder)
	return names
}

// init verifies at package load time that presetOrder and the presets map
// are kept in sync, and that presetOrder is sorted the way we intend to
// document it (defensive: this is a compile-time-adjacent guard against
// silent drift between the map and the documented order).
func init() {
	if len(presetOrder) != len(presets) {
		panic("identity: presetOrder e presets fora de sincronia")
	}
	seen := make(map[string]bool, len(presetOrder))
	for _, name := range presetOrder {
		if _, ok := presets[name]; !ok {
			panic(fmt.Sprintf("identity: preset %q listado em presetOrder mas ausente de presets", name))
		}
		seen[name] = true
	}
	if len(seen) != len(presetOrder) {
		panic("identity: presetOrder contem nomes duplicados")
	}
}
