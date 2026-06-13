package validator

// RuleItem representa uma violação ou warning individual no output JSON.
type RuleItem struct {
	Rule    string `json:"rule"`
	File    string `json:"file"`
	Message string `json:"message"`
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

// messageToRuleItem converte uma mensagem string em RuleItem.
// O campo Rule é preenchido com best-effort extraindo o tipo do artefato (adr/req/roadmap)
// do prefixo da mensagem. File e Rule ficam em branco quando não há padrão identificável.
func messageToRuleItem(msg string) RuleItem {
	return RuleItem{
		Rule:    "",
		File:    "",
		Message: msg,
	}
}

// stringsToRuleItems converte []string em []RuleItem.
func stringsToRuleItems(msgs []string) []RuleItem {
	items := make([]RuleItem, 0, len(msgs))
	for _, m := range msgs {
		items = append(items, messageToRuleItem(m))
	}
	return items
}

// BuildResult constrói um ValidateResult a partir dos slices retornados por Validate().
// lenient indica se o projeto está em modo lenient (governa o campo Mode).
func BuildResult(violations, warnings []string, lenient bool) ValidateResult {
	mode := "strict"
	if lenient {
		mode = "lenient"
	}
	exitCode := 0
	if len(violations) > 0 {
		exitCode = 1
	}

	vItems := stringsToRuleItems(violations)
	wItems := stringsToRuleItems(warnings)

	// Garantir que os slices serializam como [] e não null em JSON.
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
