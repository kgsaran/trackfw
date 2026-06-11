package generators

import (
	"fmt"
	"os"
	"strings"
	"time"
)

func NewADR(title string) error {
	if err := os.MkdirAll("docs/adr", 0755); err != nil {
		return err
	}

	slug := toSlug(title)
	date := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("docs/adr/ADR-%s-%s.md", date, slug)

	content := fmt.Sprintf(`# ADR: %s

> Date: %s | Status: Proposed

## Context
<!-- What is the situation that motivates this decision? -->

## Decision
<!-- What was decided? -->

## Consequences
<!-- What are the positive and negative consequences of this decision? -->

## Alternatives Considered
<!-- What other options were evaluated and why were they rejected? -->
`, title, date)

	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing ADR: %w", err)
	}

	fmt.Printf("✓ created %s\n", filename)
	return nil
}

func toSlug(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	return s
}
