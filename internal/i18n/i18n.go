package i18n

import (
	_ "embed"
	"encoding/json"
	"os"
	"strings"
	"sync"
)

//go:embed locales/en-US.json
var enUS []byte

//go:embed locales/pt-BR.json
var ptBR []byte

//go:embed locales/es-ES.json
var esES []byte

var (
	once     sync.Once
	messages map[string]interface{}
	locale   string
)

// DetectLocale infers the locale from LANG / LC_ALL / LANGUAGE environment variables.
// Returns "pt-BR", "es-ES" or falls back to "en-US".
func DetectLocale() string {
	for _, env := range []string{"LANG", "LC_ALL", "LANGUAGE"} {
		val := os.Getenv(env)
		if val == "" {
			continue
		}
		// pt_BR.UTF-8 → pt-BR
		val = strings.Split(val, ".")[0]
		val = strings.ReplaceAll(val, "_", "-")
		lang := strings.Split(val, "-")[0]
		switch lang {
		case "pt":
			return "pt-BR"
		case "es":
			return "es-ES"
		}
	}
	return "en-US"
}

func load() {
	once.Do(func() {
		locale = DetectLocale()
		var data []byte
		switch locale {
		case "pt-BR":
			data = ptBR
		case "es-ES":
			data = esES
		default:
			data = enUS
		}
		if err := json.Unmarshal(data, &messages); err != nil {
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

// Locale returns the active locale code (e.g. "pt-BR").
func Locale() string {
	load()
	return locale
}
