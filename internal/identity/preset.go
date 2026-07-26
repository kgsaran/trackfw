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
		"iac",
		"tooling",
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
		"iac":          {DisplayName: "Dédalo", Slug: "dedalo"},
		"tooling":      {DisplayName: "Prometeu", Slug: "prometeu"},
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
		"iac":          {DisplayName: "Ivaldi", Slug: "ivaldi"},
		"tooling":      {DisplayName: "Loki", Slug: "loki"},
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
		"iac":          {DisplayName: "Rowena", Slug: "rowena"},
		"tooling":      {DisplayName: "Ollivander", Slug: "ollivander"},
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
		"iac":          {DisplayName: "Gendry", Slug: "gendry"},
		"tooling":      {DisplayName: "Qyburn", Slug: "qyburn"},
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
		"iac":          {DisplayName: "Chaves", Slug: "chaves"},
		"tooling":      {DisplayName: "Chapolin", Slug: "chapolin"},
	},
	"pioneers": {
		"architect":    {DisplayName: "Turing", Slug: "turing"},
		"backend":      {DisplayName: "Ritchie", Slug: "ritchie"},
		"frontend":     {DisplayName: "Berners-Lee", Slug: "berners-lee"},
		"qa":           {DisplayName: "Hamilton", Slug: "hamilton"},
		"infra":        {DisplayName: "Torvalds", Slug: "torvalds"},
		"security":     {DisplayName: "Diffie", Slug: "diffie"},
		"dba":          {DisplayName: "Codd", Slug: "codd"},
		"ux":           {DisplayName: "Norman", Slug: "norman"},
		"code-quality": {DisplayName: "Knuth", Slug: "knuth"},
		"data":         {DisplayName: "Hopper", Slug: "hopper"},
		"iac":          {DisplayName: "Hashimoto", Slug: "hashimoto"},
		"tooling":      {DisplayName: "McCarthy", Slug: "mccarthy"},
	},
	"starwars": {
		"architect":    {DisplayName: "Yoda", Slug: "yoda"},
		"backend":      {DisplayName: "Han", Slug: "han"},
		"frontend":     {DisplayName: "Leia", Slug: "leia"},
		"qa":           {DisplayName: "Ahsoka", Slug: "ahsoka"},
		"infra":        {DisplayName: "Chewbacca", Slug: "chewbacca"},
		"security":     {DisplayName: "Vader", Slug: "vader"},
		"dba":          {DisplayName: "R2-D2", Slug: "r2-d2"},
		"ux":           {DisplayName: "Padmé", Slug: "padme"},
		"code-quality": {DisplayName: "Obi-Wan", Slug: "obi-wan"},
		"data":         {DisplayName: "C-3PO", Slug: "c-3po"},
		"iac":          {DisplayName: "Rey", Slug: "rey"},
		"tooling":      {DisplayName: "Babu Frik", Slug: "babu-frik"},
	},
	"tolkien": {
		"architect":    {DisplayName: "Gandalf", Slug: "gandalf"},
		"backend":      {DisplayName: "Aragorn", Slug: "aragorn"},
		"frontend":     {DisplayName: "Arwen", Slug: "arwen"},
		"qa":           {DisplayName: "Legolas", Slug: "legolas"},
		"infra":        {DisplayName: "Gimli", Slug: "gimli"},
		"security":     {DisplayName: "Boromir", Slug: "boromir"},
		"dba":          {DisplayName: "Elrond", Slug: "elrond"},
		"ux":           {DisplayName: "Galadriel", Slug: "galadriel"},
		"code-quality": {DisplayName: "Faramir", Slug: "faramir"},
		"data":         {DisplayName: "Bilbo", Slug: "bilbo"},
		"iac":          {DisplayName: "Aulë", Slug: "aule"},
		"tooling":      {DisplayName: "Celebrimbor", Slug: "celebrimbor"},
	},
	"turma": {
		"architect":    {DisplayName: "Franjinha", Slug: "franjinha"},
		"backend":      {DisplayName: "Cebolinha", Slug: "cebolinha"},
		"frontend":     {DisplayName: "Magali", Slug: "magali"},
		"qa":           {DisplayName: "Mônica", Slug: "monica"},
		"infra":        {DisplayName: "Cascão", Slug: "cascao"},
		"security":     {DisplayName: "Bidu", Slug: "bidu"},
		"dba":          {DisplayName: "Marocas", Slug: "marocas"},
		"ux":           {DisplayName: "Anjinho", Slug: "anjinho"},
		"code-quality": {DisplayName: "Titi", Slug: "titi"},
		"data":         {DisplayName: "Chico", Slug: "chico"},
		"iac":          {DisplayName: "Piteco", Slug: "piteco"},
		"tooling":      {DisplayName: "Nimbus", Slug: "nimbus"},
	},
	"egyptian": {
		"architect":    {DisplayName: "Thoth", Slug: "thoth"},
		"backend":      {DisplayName: "Rá", Slug: "ra"},
		"frontend":     {DisplayName: "Ísis", Slug: "isis"},
		"qa":           {DisplayName: "Hórus", Slug: "horus"},
		"infra":        {DisplayName: "Ptah", Slug: "ptah"},
		"security":     {DisplayName: "Anúbis", Slug: "anubis"},
		"dba":          {DisplayName: "Seshat", Slug: "seshat"},
		"ux":           {DisplayName: "Bastet", Slug: "bastet"},
		"code-quality": {DisplayName: "Maat", Slug: "maat"},
		"data":         {DisplayName: "Osíris", Slug: "osiris"},
		"iac":          {DisplayName: "Imhotep", Slug: "imhotep"},
		"tooling":      {DisplayName: "Khnum", Slug: "khnum"},
	},
}

// presetOrder is the canonical, stable order in which preset names are
// listed (e.g. by PresetNames and in error messages).
var presetOrder = []string{"greek", "norse", "potter", "thrones", "chaves", "pioneers", "starwars", "tolkien", "turma", "egyptian"}

// Preset returns the identity Config for the given themed preset name (one
// of "greek", "norse", "potter", "thrones", "chaves", "pioneers", "starwars",
// "tolkien", "turma", "egyptian"). If name is not a known preset, Preset
// returns an error listing the valid preset names.
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
// greek, norse, potter, thrones, chaves, pioneers, starwars, tolkien, turma,
// egyptian.
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
