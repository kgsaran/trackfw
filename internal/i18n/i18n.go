package i18n

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
)

//go:embed locales/en-US.json
var enUS []byte

var (
	once     sync.Once
	messages map[string]interface{}
)

// DetectLocale always returns "en-US". The product emits only English
// regardless of the user's locale environment variables (ML-3E, AC2).
func DetectLocale() string {
	return "en-US"
}

func load() {
	once.Do(func() {
		if err := json.Unmarshal(enUS, &messages); err != nil {
			messages = map[string]interface{}{}
		}
	})
}

// T returns the translation for the dot-separated key.
// vars is an alternating sequence of placeholder name, value pairs for interpolation.
// Example: T("req.new.created", "path", "/docs/req/REQ-001.md")
func T(key string, vars ...string) string {
	load()
	parts := strings.Split(key, ".")
	var cur interface{} = messages
	for _, p := range parts {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return key
		}
		cur = m[p]
	}
	s, ok := cur.(string)
	if !ok {
		return key
	}
	for i := 0; i+1 < len(vars); i += 2 {
		s = strings.ReplaceAll(s, "{{"+vars[i]+"}}", vars[i+1])
	}
	return s
}

// Locale returns the active locale code. Always returns "en-US".
func Locale() string {
	return "en-US"
}
