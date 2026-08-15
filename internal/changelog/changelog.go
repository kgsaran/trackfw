// Package changelog fornece parsing e extração de seções do CHANGELOG.md
// no formato Keep a Changelog (https://keepachangelog.com/en/1.1.0/).
package changelog

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Section representa uma seção de versão do CHANGELOG.md.
// Version é "Unreleased" (sem colchetes) ou "x.y.z". Body é o texto completo
// da seção (incluindo as subseções ### Added etc.), sem a linha de cabeçalho
// "## [...]".
type Section struct {
	Version string
	Date    string
	Body    string
}

// sectionHeaderRE casa cabeçalhos de seção no formato "## [x.y.z] - YYYY-MM-DD"
// ou "## [Unreleased]" (sem data).
var sectionHeaderRE = regexp.MustCompile(`^## \[([^\]]+)\](?: - (\d{4}-\d{2}-\d{2}))?`)

// ParseSections separa o conteúdo de um CHANGELOG.md em Section, uma por
// cabeçalho "## [...]" encontrado. Texto antes da primeira seção (título do
// arquivo, preâmbulo) é descartado.
func ParseSections(content string) ([]Section, error) {
	lines := strings.Split(content, "\n")

	var sections []Section
	var current *Section
	var bodyLines []string

	flush := func() {
		if current != nil {
			current.Body = strings.Join(bodyLines, "\n")
			sections = append(sections, *current)
		}
	}

	for _, line := range lines {
		if m := sectionHeaderRE.FindStringSubmatch(line); m != nil {
			flush()
			current = &Section{Version: m[1], Date: m[2]}
			bodyLines = nil
			continue
		}
		if current != nil {
			bodyLines = append(bodyLines, line)
		}
	}
	flush()

	return sections, nil
}

// FirstSection retorna a primeira seção da lista.
// Erro se a lista vier vazia.
func FirstSection(sections []Section) (Section, error) {
	if len(sections) == 0 {
		return Section{}, fmt.Errorf("CHANGELOG.md has no version sections")
	}
	return sections[0], nil
}

// FindVersion busca a seção com Version igual ao argumento, normalizando um
// prefixo "v"/"V" opcional no argumento do usuário antes de comparar.
func FindVersion(sections []Section, version string) (Section, error) {
	normalized := version
	if len(normalized) > 0 && (normalized[0] == 'v' || normalized[0] == 'V') {
		normalized = normalized[1:]
	}
	for _, s := range sections {
		if s.Version == normalized || s.Version == version {
			return s, nil
		}
	}
	return Section{}, fmt.Errorf("version %q not found in CHANGELOG.md", version)
}

// FormatSection reconstrói o texto formatado de uma seção, reproduzindo o
// cabeçalho original.
func FormatSection(s Section) string {
	dateSuffix := ""
	if s.Date != "" {
		dateSuffix = " - " + s.Date
	}
	body := strings.TrimLeft(s.Body, "\n")
	return fmt.Sprintf("## [%s]%s\n\n%s", s.Version, dateSuffix, strings.TrimRight(body, "\n")+"\n")
}

// Read lê o CHANGELOG.md na raiz informada.
func Read(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("CHANGELOG.md not found — nothing to show")
		}
		return "", err
	}
	return string(data), nil
}
