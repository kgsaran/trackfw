package plugins

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const RegistryURL = "https://raw.githubusercontent.com/kgsaran/trackfw-plugins/main/registry.yaml"

type RegistryEntry struct {
	Name        string
	Repo        string
	Description string
	Tags        []string
}

// parseRegistryYAML parseia o YAML flat do registry linha a linha.
// Formato esperado:
//
//	plugins:
//	  - name: trackfw-go-advanced
//	    repo: kgsaran/trackfw-go-advanced
//	    description: "Advanced Go generators"
//	    tags: [go, generators]
func parseRegistryYAML(body string) []RegistryEntry {
	var entries []RegistryEntry
	var current *RegistryEntry

	lines := strings.Split(body, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "plugins:" {
			continue
		}
		if strings.HasPrefix(trimmed, "- name:") {
			if current != nil {
				entries = append(entries, *current)
			}
			current = &RegistryEntry{}
			current.Name = strings.TrimSpace(strings.TrimPrefix(trimmed, "- name:"))
			continue
		}
		if current == nil {
			continue
		}
		if strings.HasPrefix(trimmed, "repo:") {
			current.Repo = strings.TrimSpace(strings.TrimPrefix(trimmed, "repo:"))
		} else if strings.HasPrefix(trimmed, "description:") {
			desc := strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))
			// Remove aspas opcionais
			desc = strings.Trim(desc, `"`)
			current.Description = desc
		} else if strings.HasPrefix(trimmed, "tags:") {
			raw := strings.TrimSpace(strings.TrimPrefix(trimmed, "tags:"))
			// Formato: [go, generators]
			raw = strings.Trim(raw, "[]")
			parts := strings.Split(raw, ",")
			for _, p := range parts {
				tag := strings.TrimSpace(p)
				if tag != "" {
					current.Tags = append(current.Tags, tag)
				}
			}
		}
	}
	if current != nil {
		entries = append(entries, *current)
	}
	return entries
}

// matchesKeyword verifica se a entrada corresponde à keyword (name, description ou tags).
func matchesKeyword(e RegistryEntry, keyword string) bool {
	if strings.Contains(strings.ToLower(e.Name), keyword) {
		return true
	}
	if strings.Contains(strings.ToLower(e.Description), keyword) {
		return true
	}
	for _, tag := range e.Tags {
		if strings.Contains(strings.ToLower(tag), keyword) {
			return true
		}
	}
	return false
}

// Search busca no registry central por keyword (name, description, tags).
// Retorna nil, nil se registry indisponível (rede offline).
func Search(keyword string) ([]RegistryEntry, error) {
	resp, err := http.Get(RegistryURL) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("registry unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry returned status %d", resp.StatusCode)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read registry: %w", err)
	}
	entries := parseRegistryYAML(string(bodyBytes))

	kw := strings.ToLower(keyword)
	var results []RegistryEntry
	for _, e := range entries {
		if matchesKeyword(e, kw) {
			results = append(results, e)
		}
	}
	return results, nil
}

// ResolveRepo: se repo parece um nome do registry (sem "/"), busca no registry e retorna repo real.
// Se já tem "/" → retorna como está (sem chamada de rede).
func ResolveRepo(nameOrRepo string) (string, error) {
	if strings.Contains(nameOrRepo, "/") {
		return nameOrRepo, nil
	}
	entries, err := Search(nameOrRepo)
	if err != nil {
		return "", fmt.Errorf("could not resolve %q from registry: %w", nameOrRepo, err)
	}
	for _, e := range entries {
		if strings.EqualFold(e.Name, nameOrRepo) {
			return e.Repo, nil
		}
	}
	return "", fmt.Errorf("plugin %q not found in registry", nameOrRepo)
}

func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".trackfw", "plugins"), nil
}

func List() ([]string, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

func Install(repo string) error {
	// Resolver nome do registry se não contém "/"
	resolved, err := ResolveRepo(repo)
	if err != nil {
		return err
	}
	repo = resolved

	// repo no formato "user/name" ou "user/name@tag"
	base := repo
	tag := "latest"
	for i, c := range repo {
		if c == '@' {
			base = repo[:i]
			tag = repo[i+1:]
			break
		}
	}
	pluginName := filepath.Base(base)
	assetName := fmt.Sprintf("trackfw-plugin-%s-%s-%s", pluginName, runtime.GOOS, runtime.GOARCH)
	var url string
	if tag == "latest" {
		url = fmt.Sprintf("https://github.com/%s/releases/latest/download/%s", base, assetName)
	} else {
		url = fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", base, tag, assetName)
	}

	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d for %s", resp.StatusCode, url)
	}

	dest := filepath.Join(dir, pluginName)
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755) //nolint:gosec
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	return nil
}

func Remove(name string) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("plugin %q not found", name)
	}
	return os.Remove(path)
}
