package generators

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// ML-1B (ROADMAP-2026-09-24-treze-rotulos-falham-no-censo-de-windows-e-cinco-sao-setup-que-aborta-
// o-cenario-inteiro.md, grupo G5 da triagem docs/seguranca/2026-09-25-triagem-cluster-windows.md).
//
// Defeito medido: o dreno de stdin do guard usava `IFS= read -r -t 2 -d ''`, um orçamento TOTAL.
// No pipe do MSYS/Git-Bash do Windows a vazão medida é ~14,7 KB/s (o read do bash consome fd
// não-seekable 1 byte por read(2)), então 200 KB não cabem em 2 s: medido rc=142, len=25397. O
// guard saía 0 no no-op e o escritor levava EPIPE — sem nenhum sinal alto.
//
// Correção: o orçamento passa de TOTAL para OCIOSO (laço que renova os 2 s a cada byte que chega),
// e o guard passa a ser FAIL-CLOSED quando o stdin não pôde ser lido por inteiro e não há argv.
//
// Cada teste abaixo traz, no comentário, a frase que diz qual conclusão deste ML ele afirma.
// ---------------------------------------------------------------------------

// runGuardWithPipe executa o guard com um PIPE REAL no stdin — não um strings.Reader — porque o
// que este ML mede é exatamente o comportamento do escritor do outro lado do pipe (EPIPE
// observável). writeFn recebe o lado de escrita e é RESPONSÁVEL por fechá-lo quando o escritor
// termina normalmente: sem o Close, o guard nunca vê EOF e todo teste estouraria o orçamento
// ocioso — passando pelo motivo errado. Os testes de escritor "travado" não fecham de propósito.
func runGuardWithPipe(t *testing.T, dir, scriptPath string, args []string, writeFn func(w *os.File) error) (exitCode int, stdout, stderr string, writeErr error) {
	t.Helper()

	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

	cmd := exec.Command("bash", append([]string{scriptPath}, args...)...)
	cmd.Dir = dir
	cmd.Stdin = pr
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Start(); err != nil {
		t.Fatalf("erro iniciando guard: %v", err)
	}
	_ = pr.Close() // o filho já herdou o fd

	done := make(chan error, 1)
	go func() { done <- writeFn(pw) }()

	waitErr := cmd.Wait()
	_ = pw.Close() // libera o escritor travado, se houver

	select {
	case writeErr = <-done:
	case <-time.After(10 * time.Second):
		t.Fatalf("escritor não terminou em 10s")
	}

	exitCode = 0
	if waitErr != nil {
		exitErr, ok := waitErr.(*exec.ExitError)
		if !ok {
			t.Fatalf("erro executando guard: %v (stderr: %s)", waitErr, errBuf.String())
		}
		exitCode = exitErr.ExitCode()
	}
	return exitCode, outBuf.String(), errBuf.String(), writeErr
}

// largeGuardPayload monta um payload JSON de hook com ~200 KB — o mesmo tamanho do fixture do
// Cenário 65 do check-gates-falsify.sh, que é o rótulo do censo que este ML fecha.
func largeGuardPayload(command string) string {
	return `{"tool_input":{"command":"` + command + `","pad":"` + strings.Repeat("x", 200000) + `"}}`
}

// AFIRMA: que a causa do rótulo
// falsify/git-branch-guard/stdin-drain-before-noop/baseline-writer-clean-large-payload era o
// orçamento TOTAL de `read -t`, e que o orçamento OCIOSO o remove — 200 KB atravessam o dreno
// inteiros, o escritor termina sem EPIPE e o guard mantém o no-op com rc=0 fora de projeto.
func TestGitBranchGuard_StdinDrain_LargePayloadOutsideProject_WriterGetsNoEPIPE(t *testing.T) {
	dir, script := setupGitBranchGuardFixtureWithoutTrackfwYAML(t)
	payload := largeGuardPayload("git push")

	rc, _, stderr, writeErr := runGuardWithPipe(t, dir, script, nil, func(w *os.File) error {
		_, err := w.Write([]byte(payload))
		if cerr := w.Close(); err == nil {
			err = cerr
		}
		return err
	})

	if writeErr != nil {
		t.Fatalf("escritor recebeu erro (EPIPE esperado ausente): %v", writeErr)
	}
	if rc != 0 {
		t.Fatalf("guard rc=%d, esperava 0 (no-op fora de projeto trackfw); stderr: %s", rc, stderr)
	}
}

// AFIRMA: que o laço DRENA e também LÊ — o payload de 200 KB chega inteiro ao passo de parsing, e
// a decisão de bloqueio sobrevive ao tamanho. Sem isto, o teste acima poderia passar com um dreno
// que joga bytes fora, e o guard viraria cego para payload grande dentro de projeto.
func TestGitBranchGuard_StdinDrain_LargePayloadInsideProject_StillBlocks(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)
	payload := largeGuardPayload("git push")

	rc, stdout, _, writeErr := runGuardWithPipe(t, dir, script, nil, func(w *os.File) error {
		_, err := w.Write([]byte(payload))
		if cerr := w.Close(); err == nil {
			err = cerr
		}
		return err
	})

	if writeErr != nil {
		t.Fatalf("escritor recebeu erro: %v", writeErr)
	}
	if rc != 2 {
		t.Fatalf("guard rc=%d, esperava 2 (git push dentro de projeto trackfw)", rc)
	}
	if !strings.Contains(stdout, `"permissionDecision":"deny"`) {
		t.Fatalf("stdout sem deny do schema PreToolUse: %s", stdout)
	}
}

// AFIRMA: a decisão de contrato deste ML — quando o stdin NÃO pôde ser lido por inteiro, o guard é
// FAIL-CLOSED. É a segunda direção da falsificação: stdin ilegível precisa produzir recusa VISÍVEL
// (deny no stdout + razão no stderr + exit 2), nunca o rc=0 silencioso que existia antes.
func TestGitBranchGuard_StdinTruncatedInsideProject_RefusesVisibly(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)

	rc, stdout, stderr, _ := runGuardWithPipe(t, dir, script, nil, func(w *os.File) error {
		if _, err := w.Write([]byte(`{"tool_input":{"comm`)); err != nil {
			return err
		}
		// Segura o fd aberto além do orçamento ocioso de 2 s, sem enviar mais nada: é o
		// escritor que "não vai escrever", o caso que o orçamento existe para cortar.
		time.Sleep(4 * time.Second)
		return nil
	})

	if rc != 2 {
		t.Fatalf("guard rc=%d, esperava 2 (fail-closed com stdin truncado); stdout=%q stderr=%q", rc, stdout, stderr)
	}
	if !strings.Contains(stdout, `"permissionDecision":"deny"`) {
		t.Fatalf("recusa não é visível no stdout (schema PreToolUse ausente): %s", stdout)
	}
	if !strings.Contains(stderr, "nao pode ser lido por inteiro") {
		t.Fatalf("stderr não nomeia o motivo da recusa: %s", stderr)
	}
}

// AFIRMA: que o fail-closed foi posto DEPOIS do probe de no-op — fora de projeto trackfw o guard
// continua saindo 0, como manda a ADR-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-
// trackfw.md. É o limite do custo da escolha fail-closed: ela não bloqueia git em repositório
// nenhum fora de projeto trackfw.
func TestGitBranchGuard_StdinTruncatedOutsideProject_StillNoOps(t *testing.T) {
	dir, script := setupGitBranchGuardFixtureWithoutTrackfwYAML(t)

	rc, stdout, stderr, _ := runGuardWithPipe(t, dir, script, nil, func(w *os.File) error {
		if _, err := w.Write([]byte(`{"tool_input":{"comm`)); err != nil {
			return err
		}
		time.Sleep(4 * time.Second)
		return nil
	})

	if rc != 0 {
		t.Fatalf("guard rc=%d, esperava 0 (no-op fora de projeto preservado); stdout=%q stderr=%q", rc, stdout, stderr)
	}
}

// AFIRMA: a isenção por argv escrita no contrato — quando o comando completo chega por argv, o
// stdin não participa da decisão, então o truncamento não pode virar recusa. Sem esta isenção, um
// runtime que passa o comando por argv e deixa o stdin pendurado seria bloqueado sem motivo.
func TestGitBranchGuard_StdinTruncatedWithArgv_DoesNotRefuse(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)

	rc, stdout, stderr, _ := runGuardWithPipe(t, dir, script, []string{"git status"}, func(w *os.File) error {
		if _, err := w.Write([]byte("x")); err != nil {
			return err
		}
		time.Sleep(4 * time.Second)
		return nil
	})

	if rc != 0 {
		t.Fatalf("guard rc=%d, esperava 0 (argv completo isenta o truncamento); stdout=%q stderr=%q", rc, stdout, stderr)
	}
}

// AFIRMA: que o orçamento é OCIOSO e não TOTAL — um escritor que entrega o payload em pedaços
// espaçados por MAIS que o antigo orçamento total (5 pedaços com 1 s de intervalo: >4 s de
// transferência contra os 2 s de orçamento) é lido por INTEIRO. É o teste que distingue a correção
// real de "aumentar o -t": com orçamento total, este payload seria truncado.
func TestGitBranchGuard_StdinDrain_SlowTricklingWriter_IsReadInFull(t *testing.T) {
	dir, script := setupGitBranchGuardFixture(t)
	// 🔴 O comando vem DEPOIS do padding, de propósito: com orçamento TOTAL o pedaço lido para
	// antes de "command", a extração rende vazio e o guard sai 0 — é isso que torna este teste
	// não-vácuo (medido: script antigo rc=0, script novo rc=2, mesma fixture).
	payload := `{"tool_input":{"pad":"` + strings.Repeat("y", 400) + `","command":"git push"}}`

	rc, stdout, stderr, writeErr := runGuardWithPipe(t, dir, script, nil, func(w *os.File) error {
		chunks := 5
		size := (len(payload) + chunks - 1) / chunks
		for i := 0; i < len(payload); i += size {
			end := i + size
			if end > len(payload) {
				end = len(payload)
			}
			if _, err := w.Write([]byte(payload[i:end])); err != nil {
				return err
			}
			time.Sleep(1 * time.Second)
		}
		return w.Close()
	})

	if writeErr != nil {
		t.Fatalf("escritor recebeu erro: %v", writeErr)
	}
	if rc != 2 {
		t.Fatalf("guard rc=%d, esperava 2 (payload entregue em pedaços lentos deve ser lido inteiro); stdout=%q stderr=%q", rc, stdout, stderr)
	}
	if strings.Contains(stderr, "nao pode ser lido por inteiro") {
		t.Fatalf("guard tratou escritor LENTO como truncado — o orçamento ainda é total: %s", stderr)
	}
}
