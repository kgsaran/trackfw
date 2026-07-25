package i18n

import (
	"encoding/json"
	"sort"
	"testing"
)

// flattenKeys walks a decoded JSON locale document and returns every
// dot-separated leaf key path, e.g. "init.prompt.identityPreset".
func flattenKeys(t *testing.T, data []byte) []string {
	t.Helper()
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("invalid locale JSON: %v", err)
	}
	var keys []string
	var walk func(prefix string, node map[string]interface{})
	walk = func(prefix string, node map[string]interface{}) {
		for key, value := range node {
			path := key
			if prefix != "" {
				path = prefix + "." + key
			}
			if child, ok := value.(map[string]interface{}); ok {
				walk(path, child)
				continue
			}
			keys = append(keys, path)
		}
	}
	walk("", raw)
	sort.Strings(keys)
	return keys
}

// TestLocalesShareIdenticalKeySets guards the CLAUDE.md requirement that the
// three embedded locale files (pt-BR, en-US, es-ES) expose exactly the same
// set of i18n.T keys. It reads the package-level embedded byte slices
// directly rather than going through T()/DetectLocale(), because load() is
// gated by sync.Once and freezes the active locale at the first call made
// in the process — you cannot compare locales through T() in the same test
// binary.
func TestLocalesShareIdenticalKeySets(t *testing.T) {
	locales := map[string][]byte{
		"pt-BR": ptBR,
		"en-US": enUS,
		"es-ES": esES,
	}

	keysByLocale := make(map[string][]string, len(locales))
	for name, data := range locales {
		keysByLocale[name] = flattenKeys(t, data)
	}

	reference := keysByLocale["en-US"]
	for name, keys := range keysByLocale {
		if name == "en-US" {
			continue
		}
		missing := diffKeys(reference, keys)
		extra := diffKeys(keys, reference)
		if len(missing) > 0 {
			t.Errorf("locale %s is missing keys present in en-US: %v", name, missing)
		}
		if len(extra) > 0 {
			t.Errorf("locale %s has extra keys not present in en-US: %v", name, extra)
		}
	}
}

// diffKeys returns the elements of a that are not present in b.
func diffKeys(a, b []string) []string {
	set := make(map[string]bool, len(b))
	for _, key := range b {
		set[key] = true
	}
	var diff []string
	for _, key := range a {
		if !set[key] {
			diff = append(diff, key)
		}
	}
	return diff
}
