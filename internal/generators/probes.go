package generators

import "strings"

// Probe representa um domínio técnico com palavras-chave e perguntas de decisão.
type Probe struct {
	Domain    string
	Keywords  []string
	Questions []Question
}

// Question representa uma pergunta de decisão arquitetural.
type Question struct {
	Text    string
	Options []ProbeOption
}

// ProbeOption representa uma opção de resposta para uma Question.
type ProbeOption struct {
	Label   string
	ADRSlug string // vazio = decisão já tomada; não-vazio = gera ADR Draft
	Decided bool   // true = não gera ADR
}

// ProbesCatalog é o catálogo completo de domínios técnicos detectáveis.
var ProbesCatalog = []Probe{
	{
		Domain:   "authentication",
		Keywords: []string{"login", "auth", "senha", "password", "sso", "jwt", "session", "token", "autenticação", "autenticar"},
		Questions: []Question{
			{
				Text: "How will users authenticate?",
				Options: []ProbeOption{
					{Label: "Local login (email + password)", Decided: true},
					{Label: "SSO (Google, Azure AD, Okta...)", ADRSlug: "sso-provider"},
					{Label: "Both (local + SSO)", ADRSlug: "authentication-strategy"},
					{Label: "Not decided yet", ADRSlug: "authentication-strategy"},
				},
			},
			{
				Text: "How will sessions be managed?",
				Options: []ProbeOption{
					{Label: "JWT (stateless)", Decided: true},
					{Label: "Server-side sessions (cookies)", Decided: true},
					{Label: "Not decided yet", ADRSlug: "session-management"},
				},
			},
		},
	},
	{
		Domain:   "ui",
		Keywords: []string{"tela", "screen", "ui", "frontend", "componente", "component", "design", "layout", "interface"},
		Questions: []Question{
			{
				Text: "Is there an existing UI framework or design system?",
				Options: []ProbeOption{
					{Label: "Yes, already chosen", Decided: true},
					{Label: "No, need to choose a UI framework", ADRSlug: "ui-framework"},
					{Label: "Not relevant for this REQ", Decided: true},
				},
			},
		},
	},
	{
		Domain:   "persistence",
		Keywords: []string{"banco", "database", "db", "tabela", "table", "migração", "migration", "modelo", "model", "persistência", "persist"},
		Questions: []Question{
			{
				Text: "Which database engine will be used?",
				Options: []ProbeOption{
					{Label: "Already decided", Decided: true},
					{Label: "Not decided yet", ADRSlug: "database-engine"},
				},
			},
		},
	},
	{
		Domain:   "api",
		Keywords: []string{"api", "endpoint", "rest", "grpc", "graphql", "rota", "route", "http"},
		Questions: []Question{
			{
				Text: "Which API protocol will be used?",
				Options: []ProbeOption{
					{Label: "REST (already decided)", Decided: true},
					{Label: "gRPC (already decided)", Decided: true},
					{Label: "GraphQL (already decided)", Decided: true},
					{Label: "Not decided yet", ADRSlug: "api-protocol"},
				},
			},
		},
	},
	{
		Domain:   "deploy",
		Keywords: []string{"deploy", "cloud", "container", "kubernetes", "k8s", "docker", "infra", "aws", "gcp", "azure"},
		Questions: []Question{
			{
				Text: "Is the deployment infrastructure already defined?",
				Options: []ProbeOption{
					{Label: "Yes, fully defined", Decided: true},
					{Label: "Cloud provider not decided", ADRSlug: "cloud-provider"},
					{Label: "Container strategy not decided", ADRSlug: "container-strategy"},
				},
			},
		},
	},
	{
		Domain:   "events",
		Keywords: []string{"kafka", "fila", "queue", "notificação", "notification", "evento", "event", "pubsub", "pub/sub", "broker", "sqs", "redis"},
		Questions: []Question{
			{
				Text: "Which event broker will be used?",
				Options: []ProbeOption{
					{Label: "Already decided", Decided: true},
					{Label: "Not decided yet", ADRSlug: "event-broker"},
				},
			},
		},
	},
}

// DetectDomains retorna os probes cujos keywords aparecem na intention (case-insensitive).
// intention é a concatenação de título + motivação da REQ.
func DetectDomains(intention string) []Probe {
	lower := strings.ToLower(intention)
	var matched []Probe
	for _, probe := range ProbesCatalog {
		for _, kw := range probe.Keywords {
			if strings.Contains(lower, strings.ToLower(kw)) {
				matched = append(matched, probe)
				break
			}
		}
	}
	return matched
}
