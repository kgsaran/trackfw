package changelog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixtureChangelog = `# Changelog

Todas as mudanças notáveis deste projeto serão documentadas neste arquivo.

## [Unreleased]

### Added
- feature em desenvolvimento

## [6.10.0] - 2026-08-14

### Added
- comando trackfw changelog

### Fixed
- bug no parser

## [6.9.1] - 2026-08-01

### Fixed
- correção pontual
`

func TestParseSectionsThreeSections(t *testing.T) {
	sections, err := ParseSections(fixtureChangelog)
	if err != nil {
		t.Fatalf("ParseSections: erro inesperado: %v", err)
	}
	if len(sections) != 3 {
		t.Fatalf("esperava 3 seções, obteve %d: %+v", len(sections), sections)
	}

	unreleased := sections[0]
	if unreleased.Version != "Unreleased" {
		t.Errorf("sections[0].Version = %q, esperava %q", unreleased.Version, "Unreleased")
	}
	if unreleased.Date != "" {
		t.Errorf("sections[0].Date = %q, esperava vazio", unreleased.Date)
	}
	if !strings.Contains(unreleased.Body, "feature em desenvolvimento") {
		t.Errorf("sections[0].Body não contém o conteúdo esperado: %q", unreleased.Body)
	}
	if strings.Contains(unreleased.Body, "## [") {
		t.Errorf("sections[0].Body não deve conter cabeçalho de outra seção: %q", unreleased.Body)
	}

	v6100 := sections[1]
	if v6100.Version != "6.10.0" {
		t.Errorf("sections[1].Version = %q, esperava %q", v6100.Version, "6.10.0")
	}
	if v6100.Date != "2026-08-14" {
		t.Errorf("sections[1].Date = %q, esperava %q", v6100.Date, "2026-08-14")
	}
	if !strings.Contains(v6100.Body, "comando trackfw changelog") || !strings.Contains(v6100.Body, "bug no parser") {
		t.Errorf("sections[1].Body não contém o conteúdo esperado: %q", v6100.Body)
	}

	v691 := sections[2]
	if v691.Version != "6.9.1" {
		t.Errorf("sections[2].Version = %q, esperava %q", v691.Version, "6.9.1")
	}
	if v691.Date != "2026-08-01" {
		t.Errorf("sections[2].Date = %q, esperava %q", v691.Date, "2026-08-01")
	}
	if !strings.Contains(v691.Body, "correção pontual") {
		t.Errorf("sections[2].Body não contém o conteúdo esperado: %q", v691.Body)
	}
}

func TestFirstSectionEmptyList(t *testing.T) {
	_, err := FirstSection(nil)
	if err == nil {
		t.Fatal("FirstSection(nil): esperava erro, obteve nil")
	}
	if err.Error() != "CHANGELOG.md has no version sections" {
		t.Errorf("FirstSection(nil): mensagem de erro = %q, esperava %q", err.Error(), "CHANGELOG.md has no version sections")
	}
}

func TestFirstSectionReturnsFirst(t *testing.T) {
	sections, err := ParseSections(fixtureChangelog)
	if err != nil {
		t.Fatalf("ParseSections: erro inesperado: %v", err)
	}
	first, err := FirstSection(sections)
	if err != nil {
		t.Fatalf("FirstSection: erro inesperado: %v", err)
	}
	if first.Version != "Unreleased" {
		t.Errorf("FirstSection.Version = %q, esperava %q", first.Version, "Unreleased")
	}
}

func TestFindVersionExisting(t *testing.T) {
	sections, err := ParseSections(fixtureChangelog)
	if err != nil {
		t.Fatalf("ParseSections: erro inesperado: %v", err)
	}
	got, err := FindVersion(sections, "6.10.0")
	if err != nil {
		t.Fatalf("FindVersion(6.10.0): erro inesperado: %v", err)
	}
	if got.Version != "6.10.0" {
		t.Errorf("FindVersion(6.10.0).Version = %q, esperava %q", got.Version, "6.10.0")
	}
}

func TestFindVersionMissing(t *testing.T) {
	sections, err := ParseSections(fixtureChangelog)
	if err != nil {
		t.Fatalf("ParseSections: erro inesperado: %v", err)
	}
	_, err = FindVersion(sections, "999.0.0")
	if err == nil {
		t.Fatal("FindVersion(999.0.0): esperava erro, obteve nil")
	}
	want := `version "999.0.0" not found in CHANGELOG.md`
	if err.Error() != want {
		t.Errorf("FindVersion(999.0.0): mensagem de erro = %q, esperava %q", err.Error(), want)
	}
}

func TestFindVersionNormalizesVPrefix(t *testing.T) {
	sections, err := ParseSections(fixtureChangelog)
	if err != nil {
		t.Fatalf("ParseSections: erro inesperado: %v", err)
	}
	withV, err := FindVersion(sections, "v6.10.0")
	if err != nil {
		t.Fatalf("FindVersion(v6.10.0): erro inesperado: %v", err)
	}
	withoutV, err := FindVersion(sections, "6.10.0")
	if err != nil {
		t.Fatalf("FindVersion(6.10.0): erro inesperado: %v", err)
	}
	if withV.Version != withoutV.Version || withV.Body != withoutV.Body {
		t.Errorf("FindVersion(v6.10.0) != FindVersion(6.10.0): %+v vs %+v", withV, withoutV)
	}
}

func TestFormatSectionWithDate(t *testing.T) {
	s := Section{Version: "6.10.0", Date: "2026-08-14", Body: "### Added\n- x\n"}
	got := FormatSection(s)
	want := "## [6.10.0] - 2026-08-14\n\n### Added\n- x\n"
	if got != want {
		t.Errorf("FormatSection = %q, esperava %q", got, want)
	}
}

func TestFormatSectionWithoutDate(t *testing.T) {
	s := Section{Version: "Unreleased", Date: "", Body: "### Added\n- x\n"}
	got := FormatSection(s)
	want := "## [Unreleased]\n\n### Added\n- x\n"
	if got != want {
		t.Errorf("FormatSection = %q, esperava %q", got, want)
	}
}

func TestFormatSectionDoesNotDuplicateBlankLineWhenBodyStartsWithNewline(t *testing.T) {
	// Body como vem de ParseSections para uma seção cujo cabeçalho é seguido
	// de linha em branco no CHANGELOG.md original (caso real deste repo):
	// a primeira linha do Body já é vazia.
	s := Section{Version: "Unreleased", Date: "", Body: "\n### Added\n- x\n"}
	got := FormatSection(s)
	want := "## [Unreleased]\n\n### Added\n- x\n"
	if got != want {
		t.Errorf("FormatSection = %q, esperava %q (sem linha em branco duplicada)", got, want)
	}
}

func TestReadMissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := Read(dir)
	if err == nil {
		t.Fatal("Read: esperava erro, obteve nil")
	}
	if err.Error() != "CHANGELOG.md not found — nothing to show" {
		t.Errorf("Read: mensagem de erro = %q, esperava %q", err.Error(), "CHANGELOG.md not found — nothing to show")
	}
}

func TestReadExistingFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte(fixtureChangelog), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	got, err := Read(dir)
	if err != nil {
		t.Fatalf("Read: erro inesperado: %v", err)
	}
	if got != fixtureChangelog {
		t.Errorf("Read: conteúdo divergente do fixture")
	}
}
