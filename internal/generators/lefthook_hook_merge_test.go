package generators

// ML-1C (REQ-2026-09-28-trackfw-init-reexecutado): testes do merge de
// generateLefthookHook. Todos os testes entram pelo caminho cfg.Hooks="lefthook"
// via generateGitHooks — o caminho não-interativo usa Hooks:"none" e não atinge
// generateLefthookHook (quase levou a declarar o defeito inexistente).
//
// Frases de reconciliação (Regra Dura):
//
//   T1 AbsentCreates          → medindo que lefthook.yml é criado com trackfw-validate:
//                                como único comando pre-commit quando o arquivo não existe
//   T2 ConsumerHooksPreserved → medindo que após a função, o arquivo contém exactamente
//                                um top-level "pre-commit:" e o comando do consumidor
//                                (lint) coexiste com trackfw-validate: sob ele
//   T3 Idempotent             → medindo que os bytes de lefthook.yml após a 1ª chamada
//                                são idênticos aos bytes após a 2ª chamada
//   T4 NoneSkipsLefthook      → medindo que generateGitHooks(Hooks:"none") não cria
//                                lefthook.yml (pina o fato de alcançabilidade que quase
//                                tornou o defeito invisível)
//   T5 NoPreCommitAppends     → medindo que quando lefthook.yml tem um bloco pre-push:
//                                mas não tem pre-commit:, após a função o arquivo contém
//                                exatamente um pre-commit: e o pre-push: original é preservado

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// chdirTemp creates a temp dir, chdir into it, and restores on cleanup.
// Reuses the helper already defined in trackfw_config_merge_test.go.
// (chdirTemp is declared once in the package; no redeclaration here.)

// T1 — arquivo ausente → cria lefthook.yml com o bloco completo.
//
// Reconciliação: este teste afirma que quando lefthook.yml não existe,
// generateGitHooks(Hooks:"lefthook") cria o arquivo com trackfw-validate:
// como único comando pre-commit — medido lendo o arquivo e contando
// ocorrências de "pre-commit:" e "trackfw-validate:".
func TestGenerateLefthookHook_AbsentCreates(t *testing.T) {
	dir := chdirTemp(t)

	if err := generateGitHooks(Config{Hooks: "lefthook"}); err != nil {
		t.Fatalf("generateGitHooks: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	if err != nil {
		t.Fatalf("lefthook.yml não foi criado: %v", err)
	}
	content := string(data)

	// Exatamente um pre-commit: no arquivo.
	if count := strings.Count(content, "pre-commit:"); count != 1 {
		t.Errorf("esperava exatamente 1 ocorrência de 'pre-commit:', obteve %d\nconteúdo:\n%s", count, content)
	}
	if !strings.Contains(content, "trackfw-validate:") {
		t.Errorf("trackfw-validate: ausente do arquivo criado\nconteúdo:\n%s", content)
	}
	if !strings.Contains(content, "run: trackfw validate") {
		t.Errorf("'run: trackfw validate' ausente do arquivo criado\nconteúdo:\n%s", content)
	}
}

// T2 — arquivo presente com hooks do consumidor → preservados, trackfw-validate acrescentado.
//
// Reconciliação: este teste afirma que quando lefthook.yml já tem um pre-commit:
// com um comando do consumidor (lint) e um pre-push:, após
// generateGitHooks(Hooks:"lefthook"): (a) o arquivo contém exatamente um
// top-level "pre-commit:", (b) "lint:" ainda aparece sob pre-commit:, (c)
// "trackfw-validate:" aparece sob o mesmo pre-commit:, e (d) pre-push: é
// preservado — medido por contagem de ocorrências e substrings, não por
// comparação livre de texto.
func TestGenerateLefthookHook_ConsumerHooksPreserved(t *testing.T) {
	dir := chdirTemp(t)

	consumerContent := "pre-commit:\n  commands:\n    lint:\n      run: golangci-lint run\n\npre-push:\n  commands:\n    test:\n      run: go test ./...\n"
	if err := os.WriteFile(filepath.Join(dir, "lefthook.yml"), []byte(consumerContent), 0644); err != nil {
		t.Fatalf("preparar fixture: %v", err)
	}

	if err := generateGitHooks(Config{Hooks: "lefthook"}); err != nil {
		t.Fatalf("generateGitHooks: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	if err != nil {
		t.Fatalf("ler lefthook.yml: %v", err)
	}
	content := string(data)

	// (a) Exatamente um top-level pre-commit: — sem duplicata que silencia hooks do consumidor.
	topLevelCount := 0
	for _, line := range strings.Split(content, "\n") {
		if line == "pre-commit:" {
			topLevelCount++
		}
	}
	if topLevelCount != 1 {
		t.Errorf("esperava exatamente 1 'pre-commit:' de nível 0, obteve %d\nconteúdo:\n%s", topLevelCount, content)
	}

	// (b) lint: preservado sob pre-commit:.
	if !strings.Contains(content, "lint:") {
		t.Errorf("lint: do consumidor foi destruído\nconteúdo:\n%s", content)
	}

	// (c) trackfw-validate: adicionado.
	if !strings.Contains(content, "trackfw-validate:") {
		t.Errorf("trackfw-validate: não foi adicionado\nconteúdo:\n%s", content)
	}

	// (d) pre-push: preservado.
	if !strings.Contains(content, "pre-push:") {
		t.Errorf("pre-push: do consumidor foi destruído\nconteúdo:\n%s", content)
	}
}

// T3 — idempotente: segunda chamada → zero diff em bytes.
//
// Reconciliação: este teste afirma que os bytes de lefthook.yml após a 1ª
// chamada são byte-idênticos aos bytes após a 2ª chamada — medido por
// comparação direta de string(run1) == string(run2).
func TestGenerateLefthookHook_Idempotent(t *testing.T) {
	dir := chdirTemp(t)

	consumerContent := "pre-commit:\n  commands:\n    lint:\n      run: golangci-lint run\n"
	if err := os.WriteFile(filepath.Join(dir, "lefthook.yml"), []byte(consumerContent), 0644); err != nil {
		t.Fatalf("preparar fixture: %v", err)
	}

	if err := generateGitHooks(Config{Hooks: "lefthook"}); err != nil {
		t.Fatalf("generateGitHooks (1ª): %v", err)
	}
	run1, err := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	if err != nil {
		t.Fatalf("ler lefthook.yml após 1ª: %v", err)
	}

	if err := generateGitHooks(Config{Hooks: "lefthook"}); err != nil {
		t.Fatalf("generateGitHooks (2ª): %v", err)
	}
	run2, err := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	if err != nil {
		t.Fatalf("ler lefthook.yml após 2ª: %v", err)
	}

	if string(run1) != string(run2) {
		t.Fatalf("idempotência quebrada — 2ª execução alterou o arquivo:\nrun1:\n%s\nrun2:\n%s", string(run1), string(run2))
	}
}

// T4 — Hooks:"none" não cria lefthook.yml.
//
// Reconciliação: este teste afirma que generateGitHooks(Hooks:"none") não
// atinge generateLefthookHook — medido por os.Stat retornando os.IsNotExist
// para lefthook.yml após a chamada. Este é o caminho não-interativo que quase
// levou a declarar o defeito inexistente.
func TestGenerateGitHooks_NoneSkipsLefthook(t *testing.T) {
	dir := chdirTemp(t)

	if err := generateGitHooks(Config{Hooks: "none"}); err != nil {
		t.Fatalf("generateGitHooks(none): %v", err)
	}

	path := filepath.Join(dir, "lefthook.yml")
	if _, err := os.Stat(path); err == nil {
		t.Errorf("lefthook.yml foi criado com Hooks:'none' — o caminho não-interativo não deve atingir generateLefthookHook")
	} else if !os.IsNotExist(err) {
		t.Fatalf("erro inesperado ao verificar lefthook.yml: %v", err)
	}
}

// T5 — lefthook.yml sem pre-commit: → bloco completo acrescentado.
//
// Reconciliação: este teste afirma que quando lefthook.yml tem um bloco
// pre-push: mas não tem pre-commit:, após generateGitHooks(Hooks:"lefthook")
// o arquivo contém exatamente um top-level "pre-commit:" e o bloco pre-push:
// original é preservado — medido por contagem de linhas de nível 0 e substring.
func TestGenerateLefthookHook_NoPreCommitAppends(t *testing.T) {
	dir := chdirTemp(t)

	prePushOnly := "pre-push:\n  commands:\n    test:\n      run: go test ./...\n"
	if err := os.WriteFile(filepath.Join(dir, "lefthook.yml"), []byte(prePushOnly), 0644); err != nil {
		t.Fatalf("preparar fixture: %v", err)
	}

	if err := generateGitHooks(Config{Hooks: "lefthook"}); err != nil {
		t.Fatalf("generateGitHooks: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	if err != nil {
		t.Fatalf("ler lefthook.yml: %v", err)
	}
	content := string(data)

	// Exatamente um pre-commit: de nível 0 acrescentado.
	topLevelPreCommit := 0
	for _, line := range strings.Split(content, "\n") {
		if line == "pre-commit:" {
			topLevelPreCommit++
		}
	}
	if topLevelPreCommit != 1 {
		t.Errorf("esperava exatamente 1 'pre-commit:' de nível 0, obteve %d\nconteúdo:\n%s", topLevelPreCommit, content)
	}

	// pre-push: preservado.
	if !strings.Contains(content, "pre-push:") {
		t.Errorf("pre-push: foi destruído ao acrescentar pre-commit:\nconteúdo:\n%s", content)
	}

	// trackfw-validate: presente.
	if !strings.Contains(content, "trackfw-validate:") {
		t.Errorf("trackfw-validate: não foi acrescentado\nconteúdo:\n%s", content)
	}
}
