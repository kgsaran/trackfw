package generators

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const vaultDir = "vault/notes"
const vaultIndexFile = "vault/notes/index.md"

// NewNote cria uma nova nota em vault/notes/<slug>-YYYY-MM-DD.md e linka no index.md.
// Usa toSlug() existente para derivar o slug do título.
// Idempotente: se a nota já existir, retorna erro em vez de sobrescrever.
func NewNote(title string) error {
	if err := os.MkdirAll(vaultDir, 0755); err != nil {
		return fmt.Errorf("criando vault/notes: %w", err)
	}

	slug := toSlug(title)
	date := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("%s-%s.md", slug, date)
	notePath := filepath.Join(vaultDir, filename)

	// Idempotência: falha se nota já existir
	if _, err := os.Stat(notePath); err == nil {
		return fmt.Errorf("nota %q já existe — não sobrescrita", filename)
	}

	body := fmt.Sprintf(`---
title: "%s"
tags: []
date: %s
related: []
---

# %s

## Problem

<!-- Descreva o problema ou situação que motivou esta nota. -->

## Root cause

<!-- Qual foi a causa raiz identificada? -->

## Solution

<!-- Como foi resolvido ou mitigado? O que deve ser feito? -->
`, title, date, title)

	if err := os.WriteFile(notePath, []byte(body), 0644); err != nil {
		return fmt.Errorf("escrevendo nota: %w", err)
	}

	if err := appendNoteToIndex(filename); err != nil {
		return fmt.Errorf("atualizando index.md: %w", err)
	}

	fmt.Printf("created %s\n", notePath)
	return nil
}

// appendNoteToIndex acrescenta uma linha de link para filename no vault/notes/index.md.
// Cria o index.md se não existir. Se o link já estiver presente, não duplica.
func appendNoteToIndex(filename string) error {
	// Garante que index.md existe
	if _, err := os.Stat(vaultIndexFile); os.IsNotExist(err) {
		initial := `# Vault de Conhecimento

> Ponto de entrada de conhecimento do projeto para agentes e pessoas.

## Índice

`
		if err := os.WriteFile(vaultIndexFile, []byte(initial), 0644); err != nil {
			return fmt.Errorf("criando index.md: %w", err)
		}
	}

	// Lê conteúdo atual
	data, err := os.ReadFile(vaultIndexFile)
	if err != nil {
		return fmt.Errorf("lendo index.md: %w", err)
	}

	// Verifica se já está linkado (evita duplicata)
	content := string(data)
	nameWithoutExt := strings.TrimSuffix(filename, ".md")
	// aceita tanto link markdown quanto wikilink
	if strings.Contains(content, "("+filename+")") ||
		strings.Contains(content, "[["+nameWithoutExt+"]]") ||
		strings.Contains(content, "[["+filename+"]]") {
		return nil
	}

	// Acrescenta linha de link
	link := fmt.Sprintf("- [%s](%s)\n", nameWithoutExt, filename)
	f, err := os.OpenFile(vaultIndexFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("abrindo index.md para append: %w", err)
	}
	defer f.Close()
	_, err = f.WriteString(link)
	return err
}

// NoteFiles retorna todos os arquivos .md em vault/notes/ exceto index.md.
// Retorna nil se o diretório não existir.
func NoteFiles() ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(vaultDir, "*.md"))
	if err != nil {
		return nil, fmt.Errorf("listando vault/notes: %w", err)
	}
	var notes []string
	for _, m := range matches {
		if filepath.Base(m) == "index.md" {
			continue
		}
		notes = append(notes, filepath.Base(m))
	}
	return notes, nil
}

// IndexContains retorna true se o index.md referencia filename
// (via link markdown ou wikilink).
func IndexContains(filename string) bool {
	data, err := os.ReadFile(vaultIndexFile)
	if err != nil {
		return false
	}
	content := string(data)
	nameWithoutExt := strings.TrimSuffix(filename, ".md")
	return strings.Contains(content, "("+filename+")") ||
		strings.Contains(content, "[["+nameWithoutExt+"]]") ||
		strings.Contains(content, "[["+filename+"]]")
}

// parseNoteTitle lê a primeira linha de heading H1 do arquivo markdown.
func parseNoteTitle(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return filepath.Base(path)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return filepath.Base(path)
}
