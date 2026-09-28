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
//
// ML-1E (predicado de idempotência):
//
//   T6 PrePushOnlyInstallsPreCommit → medindo que trackfw-validate: sob pre-push: (sem
//                                      pre-commit:) é tratado como ausente: o hook É
//                                      instalado em pre-commit: e o bloco pre-push: original
//                                      é preservado íntegro; e que uma 2ª chamada é no-op
//                                      (contra-braço: idempotência após correção)
//   T7 CommentNotCounted            → medindo que "# trackfw-validate:" (comentário) fora
//                                      de commands: é tratado como ausente pelo predicado,
//                                      e o hook é instalado normalmente
//
// ML-1F (comentário inline duplica entrada):
//
//   T8 FourArms                     → tabela com 4 braços medindo lefthookValidatePresent:
//                                      limpo→presente, inline-comment→presente (o fix),
//                                      só pre-push→ausente (regressão ML-1E), espaço→presente
//   T9 InlineCommentNoCorruption    → contra-braço de corrupção: arquivo com
//                                      "trackfw-validate: # cmt" já instalado → merge é
//                                      no-op → exatamente 1 ocorrência da chave em commands:

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

// T6 — trackfw-validate: sob pre-push: (sem pre-commit:) → hook instalado em pre-commit:,
// bloco pre-push: preservado íntegro; e 2ª chamada é no-op (contra-braço).
//
// Reconciliação: este teste afirma que quando lefthook.yml contém trackfw-validate: apenas
// sob pre-push: e não tem bloco pre-commit:, lefthookValidatePresent retorna false (o hook
// não está no lugar certo), generateGitHooks instala trackfw-validate: em um novo bloco
// pre-commit:, e o bloco pre-push: original é preservado byte-a-byte; e que uma segunda
// chamada produz output byte-idêntico ao da primeira (o predicado corrigido reconhece
// trackfw-validate: em pre-commit: e retorna early) — medido por contagem de linhas de
// nível 0, substring e comparação de bytes entre run1 e run2.
func TestGenerateLefthookHook_PrePushOnlyInstallsPreCommit(t *testing.T) {
	dir := chdirTemp(t)

	// Fixture: trackfw-validate: está sob pre-push:, sem nenhum pre-commit:.
	// Este é o braço exato do achado: consumidor que roda validate no push em vez do commit.
	prePushWithValidate := "pre-commit:\n  commands:\n    lint:\n      run: golangci-lint run\npre-push:\n  commands:\n    trackfw-validate:\n      run: trackfw validate\n"
	if err := os.WriteFile(filepath.Join(dir, "lefthook.yml"), []byte(prePushWithValidate), 0644); err != nil {
		t.Fatalf("preparar fixture: %v", err)
	}

	// 1ª chamada.
	if err := generateGitHooks(Config{Hooks: "lefthook"}); err != nil {
		t.Fatalf("generateGitHooks (1ª): %v", err)
	}

	run1, err := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	if err != nil {
		t.Fatalf("ler lefthook.yml após 1ª: %v", err)
	}
	content := string(run1)

	// Exatamente um top-level pre-commit: (sem duplicata).
	topLevelPreCommit := 0
	for _, line := range strings.Split(content, "\n") {
		if line == "pre-commit:" {
			topLevelPreCommit++
		}
	}
	if topLevelPreCommit != 1 {
		t.Errorf("esperava exatamente 1 'pre-commit:' de nível 0, obteve %d\nconteúdo:\n%s", topLevelPreCommit, content)
	}

	// trackfw-validate: instalado especificamente dentro do bloco pre-commit: —
	// não apenas em qualquer lugar do arquivo (ex: ainda só sob pre-push:).
	installedInPreCommit := false
	{
		inPC := false
		for _, ln := range strings.Split(content, "\n") {
			if len(ln) > 0 && ln[0] != ' ' && ln[0] != '\t' && ln[0] != '#' {
				inPC = ln == "pre-commit:"
			}
			if inPC && strings.TrimSpace(ln) == "trackfw-validate:" {
				installedInPreCommit = true
				break
			}
		}
	}
	if !installedInPreCommit {
		t.Errorf("trackfw-validate: não foi instalado dentro de pre-commit: (só estava em pre-push:)\nconteúdo:\n%s", content)
	}

	// Bloco pre-push: preservado íntegro.
	if !strings.Contains(content, "pre-push:") {
		t.Errorf("bloco pre-push: foi destruído\nconteúdo:\n%s", content)
	}

	// Contra-braço: 2ª chamada é no-op (idempotência real preservada após correção do predicado).
	if err := generateGitHooks(Config{Hooks: "lefthook"}); err != nil {
		t.Fatalf("generateGitHooks (2ª): %v", err)
	}
	run2, err := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	if err != nil {
		t.Fatalf("ler lefthook.yml após 2ª: %v", err)
	}
	if string(run1) != string(run2) {
		t.Fatalf("idempotência quebrada após correção do predicado — 2ª execução alterou o arquivo:\nrun1:\n%s\nrun2:\n%s", string(run1), string(run2))
	}
}

// T7 — "# trackfw-validate:" como comentário → tratado como ausente, hook instalado.
//
// Reconciliação: este teste afirma que lefthookValidatePresent retorna false quando
// trackfw-validate: aparece apenas em linhas de comentário (fora de commands:), e que
// generateGitHooks instala o hook normalmente — medido verificando que o arquivo
// resultante contém trackfw-validate: como entrada real (indentado sob pre-commit:) e
// não apenas como comentário.
func TestGenerateLefthookHook_CommentNotCounted(t *testing.T) {
	dir := chdirTemp(t)

	// Fixture: trackfw-validate: aparece APENAS como comentário, fora de commands:.
	commentFixture := "pre-commit:\n  commands:\n    lint:\n      run: golangci-lint run\n# trackfw-validate: disabled\n"
	if err := os.WriteFile(filepath.Join(dir, "lefthook.yml"), []byte(commentFixture), 0644); err != nil {
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

	// trackfw-validate: instalado como entrada real (indentado sob pre-commit:).
	// Uma linha "    trackfw-validate:" (4 espaços) indica entrada em commands:.
	found := false
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == "trackfw-validate:" && len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("trackfw-validate: não foi instalado como entrada real (indentada) sob pre-commit:\nconteúdo:\n%s", content)
	}

	// Exatamente um top-level pre-commit: (sem duplicata).
	topLevelPreCommit := 0
	for _, line := range strings.Split(content, "\n") {
		if line == "pre-commit:" {
			topLevelPreCommit++
		}
	}
	if topLevelPreCommit != 1 {
		t.Errorf("esperava exatamente 1 'pre-commit:' de nível 0, obteve %d\nconteúdo:\n%s", topLevelPreCommit, content)
	}
}

// T8 — tabela de 4 braços para lefthookValidatePresent (ML-1F).
//
// Reconciliação: cada braço afirma um comportamento medido de lefthookValidatePresent:
//   arm 1 (clean)          → medindo que "trackfw-validate:" sem comentário é presente (true)
//   arm 2 (inline-comment) → medindo que "trackfw-validate: # cmt" com comentário inline
//                            é presente (true) — este é o bug corrigido no ML-1F
//   arm 3 (pre-push-only)  → medindo que "trackfw-validate:" sob pre-push: sem pre-commit:
//                            é ausente (false) — regressão do fix do ML-1E
//   arm 4 (trailing-space) → medindo que "trackfw-validate:   " com espaço no fim é presente (true)
func TestLefthookValidatePresent_FourArms(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "clean",
			content: "pre-commit:\n  commands:\n    trackfw-validate:\n      run: trackfw validate\n",
			want:    true,
		},
		{
			name:    "inline-comment",
			content: "pre-commit:\n  commands:\n    trackfw-validate: # instalado pelo trackfw\n      run: trackfw validate\n",
			want:    true,
		},
		{
			name:    "pre-push-only",
			content: "pre-push:\n  commands:\n    trackfw-validate:\n      run: trackfw validate\n",
			want:    false,
		},
		{
			name:    "trailing-space",
			content: "pre-commit:\n  commands:\n    trackfw-validate:   \n      run: trackfw validate\n",
			want:    true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := lefthookValidatePresent(tc.content)
			if got != tc.want {
				t.Errorf("lefthookValidatePresent(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

// T9 — contra-braço de corrupção: arquivo com "trackfw-validate: # cmt" já instalado
// não recebe segunda entrada ao executar o merge (ML-1F).
//
// Reconciliação: este teste afirma que o caminho de merge não produz segunda entrada
// "trackfw-validate:" quando o arquivo já contém a chave com comentário inline —
// medido CONTANDO as linhas que começam com "trackfw-validate:" (após TrimSpace) no
// arquivo resultante e exigindo exatamente 1. A contagem literal é o que o handoff
// exige: "não afirme por dedução — conte".
func TestGenerateLefthookHook_InlineCommentNoCorruption(t *testing.T) {
	dir := chdirTemp(t)

	// Fixture: trackfw-validate: JÁ instalado, com comentário inline (o caso do defeito).
	alreadyInstalled := "pre-commit:\n  commands:\n    trackfw-validate: # instalado pelo trackfw\n      run: trackfw validate\n"
	if err := os.WriteFile(filepath.Join(dir, "lefthook.yml"), []byte(alreadyInstalled), 0644); err != nil {
		t.Fatalf("preparar fixture: %v", err)
	}

	// Executa o merge — deve ser no-op (idempotente).
	if err := generateGitHooks(Config{Hooks: "lefthook"}); err != nil {
		t.Fatalf("generateGitHooks: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	if err != nil {
		t.Fatalf("ler lefthook.yml: %v", err)
	}
	content := string(data)

	// Conta todas as linhas que começam com "trackfw-validate:" (após TrimSpace),
	// incluindo tanto a entrada limpa quanto a com comentário inline.
	// O resultado deve ser exatamente 1 — se for 2, o merge duplicou a entrada.
	count := 0
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "trackfw-validate:") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("corrupção detectada: esperava exatamente 1 ocorrência de 'trackfw-validate:' em commands:, obteve %d\nconteúdo:\n%s", count, content)
	}
}
