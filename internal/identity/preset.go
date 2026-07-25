package identity

// PresetGreek returns the default identity configuration using the Greek
// pantheon naming scheme. Slugs are HARDCODED here — not derived by calling
// Slugify at runtime — because this is an explicit ADR decision: hardcoding
// avoids depending on Unicode normalization behaving identically across the
// Go, Node.js and Python CLIs.
func PresetGreek() Config {
	return Config{
		SchemaVersion: schemaVersion,
		Agents: map[string]AgentIdentity{
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
	}
}

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
