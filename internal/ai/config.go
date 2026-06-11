package ai

import (
	"bufio"
	"os"
	"strings"
)

// ReadConfig lê as chaves ai_provider, ai_model, ai_api_key de um YAML simples.
// Retorna strings vazias sem erro se o arquivo não existir.
func ReadConfig(path string) (provider, model, apiKey string, err error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", "", nil
		}
		return "", "", "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if val, ok := extractYAMLValue(line, "ai_provider"); ok {
			provider = val
		}
		if val, ok := extractYAMLValue(line, "ai_model"); ok {
			model = val
		}
		if val, ok := extractYAMLValue(line, "ai_api_key"); ok {
			apiKey = val
		}
	}
	return provider, model, apiKey, scanner.Err()
}

func extractYAMLValue(line, key string) (string, bool) {
	prefix := key + ":"
	if !strings.HasPrefix(line, prefix) {
		return "", false
	}
	val := strings.TrimSpace(line[len(prefix):])
	return val, true
}
