package identity

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// maxSlugLength is the maximum accepted length for a generated slug.
const maxSlugLength = 40

// diacriticsFolder normalizes Unicode text to NFD and strips combining marks
// (unicode.Mn), which is how accented Latin characters (á, ã, ç, ...) are
// folded to their base ASCII letter. golang.org/x/text is already present in
// go.mod as an indirect dependency (pulled in transitively by other modules),
// so using it here directly does not introduce a genuinely new dependency —
// it only promotes an existing one from indirect to direct.
var diacriticsFolder = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

// Slugify converts input into a normalized slug matching ^[a-z0-9]+(-[a-z0-9]+)*$.
//
// Steps, in exact order:
//  1. Unicode NFD normalization + diacritics removal.
//  2. Lowercase.
//  3. Spaces (including runs of spaces) and underscores become '-'.
//  4. Any character outside [a-z0-9-] is dropped.
//  5. Runs of '-' collapse into a single '-'.
//  6. Leading/trailing '-' are trimmed.
//  7. Empty result is an error.
//  8. Result longer than 40 characters is an error.
//
// Slugify never silently "fixes" degenerate input — it always returns an
// error instead of guessing.
func Slugify(input string) (string, error) {
	folded, _, err := transform.String(diacriticsFolder, input)
	if err != nil {
		return "", fmt.Errorf("identity: falha ao normalizar %q: %w", input, err)
	}

	lowered := strings.ToLower(folded)

	replaced := strings.Map(func(r rune) rune {
		if r == ' ' || r == '_' {
			return '-'
		}
		return r
	}, lowered)

	var filtered strings.Builder
	filtered.Grow(len(replaced))
	for _, r := range replaced {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			filtered.WriteRune(r)
		}
	}

	collapsed := collapseDashes(filtered.String())
	trimmed := strings.Trim(collapsed, "-")

	if trimmed == "" {
		return "", fmt.Errorf("identity: slug vazio para %q", input)
	}
	if len(trimmed) > maxSlugLength {
		return "", fmt.Errorf("identity: slug %q excede o tamanho maximo de %d caracteres (tem %d)", trimmed, maxSlugLength, len(trimmed))
	}

	return trimmed, nil
}

func collapseDashes(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	previousDash := false
	for _, r := range s {
		if r == '-' {
			if previousDash {
				continue
			}
			previousDash = true
		} else {
			previousDash = false
		}
		b.WriteRune(r)
	}
	return b.String()
}
