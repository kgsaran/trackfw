package validator

import "regexp"

// RuleItem representa uma violação ou warning individual no output JSON.
type RuleItem struct {
	Rule    string `json:"rule"`
	File    string `json:"file"`
	Message string `json:"message"`
}

// TaggedMsg associa o nome da regra à mensagem de texto que ela gerou.
// É a moeda interna usada por ValidateTagged para propagar rule+msg juntos.
type TaggedMsg struct {
	Rule string
	Msg  string
}

// ValidateSummary contém os contadores e metadados do resultado de validação.
type ValidateSummary struct {
	Violations int    `json:"violations"`
	Warnings   int    `json:"warnings"`
	Mode       string `json:"mode"`
	ExitCode   int    `json:"exit_code"`
}

// ValidateResult é a estrutura raiz do output JSON do `trackfw validate --json`.
type ValidateResult struct {
	Summary    ValidateSummary `json:"summary"`
	Violations []RuleItem      `json:"violations"`
	Warnings   []RuleItem      `json:"warnings"`
}

// fileRe extrai o primeiro token entre aspas duplas de uma mensagem.
// A maioria das mensagens formatadas com %q produz "filename.md".
var fileRe = regexp.MustCompile(`"([^"]+)"`)

// extractFile retorna o primeiro token entre aspas duplas encontrado em msg,
// ou string vazia quando o padrão não está presente (ex: mensagens com %s sem aspas).
func extractFile(msg string) string {
	m := fileRe.FindStringSubmatch(msg)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

// taggedMsgToRuleItem converte um TaggedMsg em RuleItem preenchendo Rule e File.
func taggedMsgToRuleItem(t TaggedMsg) RuleItem {
	return RuleItem{
		Rule:    t.Rule,
		File:    extractFile(t.Msg),
		Message: t.Msg,
	}
}

// taggedMsgsToRuleItems converte []TaggedMsg em []RuleItem.
func taggedMsgsToRuleItems(msgs []TaggedMsg) []RuleItem {
	items := make([]RuleItem, 0, len(msgs))
	for _, m := range msgs {
		items = append(items, taggedMsgToRuleItem(m))
	}
	return items
}

// BuildResult constrói um ValidateResult a partir dos slices retornados por Validate().
// lenient indica se o projeto está em modo lenient (governa o campo Mode).
// Mantido para compatibilidade — Rule e File ficam vazios nesta variante.
func BuildResult(violations, warnings []string, lenient bool) ValidateResult {
	mode := "strict"
	if lenient {
		mode = "lenient"
	}
	exitCode := 0
	if len(violations) > 0 {
		exitCode = 1
	}

	vItems := make([]RuleItem, 0, len(violations))
	for _, m := range violations {
		vItems = append(vItems, RuleItem{Rule: "", File: "", Message: m})
	}
	wItems := make([]RuleItem, 0, len(warnings))
	for _, m := range warnings {
		wItems = append(wItems, RuleItem{Rule: "", File: "", Message: m})
	}

	return ValidateResult{
		Summary: ValidateSummary{
			Violations: len(violations),
			Warnings:   len(warnings),
			Mode:       mode,
			ExitCode:   exitCode,
		},
		Violations: vItems,
		Warnings:   wItems,
	}
}

// BuildResultTagged constrói um ValidateResult com Rule e File preenchidos,
// a partir dos slices de TaggedMsg retornados por ValidateTagged().
func BuildResultTagged(violations, warnings []TaggedMsg, lenient bool) ValidateResult {
	mode := "strict"
	if lenient {
		mode = "lenient"
	}
	exitCode := 0
	if len(violations) > 0 {
		exitCode = 1
	}

	vItems := taggedMsgsToRuleItems(violations)
	wItems := taggedMsgsToRuleItems(warnings)

	if vItems == nil {
		vItems = []RuleItem{}
	}
	if wItems == nil {
		wItems = []RuleItem{}
	}

	return ValidateResult{
		Summary: ValidateSummary{
			Violations: len(violations),
			Warnings:   len(warnings),
			Mode:       mode,
			ExitCode:   exitCode,
		},
		Violations: vItems,
		Warnings:   wItems,
	}
}
