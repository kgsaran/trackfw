package generators

import (
	"fmt"
	"os"
	"time"
)

func NewREQ(title string) error {
	if err := os.MkdirAll("docs/req", 0755); err != nil {
		return err
	}

	slug := toSlug(title)
	date := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("docs/req/REQ-%s-%s.md", date, slug)

	content := fmt.Sprintf(`# REQ: %s

> Date: %s | Status: Open

## Motivation
<!-- Why is this requirement needed? What problem does it solve? -->

## Acceptance Criteria
- [ ]
- [ ]

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR:

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap:
`, title, date)

	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing REQ: %w", err)
	}

	fmt.Printf("✓ created %s\n", filename)
	return nil
}
