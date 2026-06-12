package plugins

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
)

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
