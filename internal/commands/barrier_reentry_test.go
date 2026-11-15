package commands

// barrier_reentry_test.go — Testes T1–T7 do ML-1A (#485):
// pilha de chaves (roadmap, wave) no barrier para detectar e recusar reentrada.
//
// 🔴 SEGURANÇA OPERACIONAL: todo gate de fixture que reentra usa o fusível de shell
// ([ "${T_FUSE:-0}" -lt 3 ] || exit 99; export T_FUSE=...) num bloco de uma única
// linha — cada sh -c é um processo separado e o export não sobrevive entre invocações.
// O fusível garante que contra o binário antigo os testes reprovam com exit 99
// (no máximo 3 níveis de recursão) em vez de virarem fork bomb.
//
// Contenção de emergência: pkill -9 -x trackfw
// Medição de processos: ps -eo comm | grep -c 'trackfw$'
// Nunca usar pkill -f nem pgrep -f.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// ────────────────────────────────────────────────────────────────────────────
// Helpers locais
// ────────────────────────────────────────────────────────────────────────────

// cleanEnvForReentry returna uma cópia de os.Environ() sem TRACKFW_BARRIER_STACK
// e sem T_FUSE, para que os testes não herdem estado externo.
func cleanEnvForReentry() []string {
	var env []string
	for _, e := range os.Environ() {
		name, _, _ := strings.Cut(e, "=")
		if strings.EqualFold(name, "TRACKFW_BARRIER_STACK") {
			continue
		}
		if strings.EqualFold(name, "T_FUSE") {
			continue
		}
		env = append(env, e)
	}
	return env
}

// runBarrierReentry executa o binário trackfw com os args fornecidos,
// o env fornecido, o diretório de trabalho dado, e um timeout de 60 s.
// Devolve stdout, stderr e exit code.
func runBarrierReentry(t *testing.T, dir string, env []string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	bin := barrierBinary(t) // compila uma vez; timeout não inclui build
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	fullArgs := append([]string{"barrier"}, args...)
	cmd := exec.CommandContext(ctx, bin, fullArgs...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.WaitDelay = 5 * time.Second
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("runBarrierReentry: unexpected error (not ExitError): %v", err)
		}
	}
	return outBuf.String(), errBuf.String(), code
}

// setupReentryFixture monta um diretório temporário com a estrutura de governança
// mínima e um roadmap de fixture. gateLines são as linhas do bloco **Gates da wave:**
// da Wave 1. Devolve (dir, roadmapPath absoluto).
//
// Para reentrada, cada gateLines[i] DEVE ser uma linha única contendo o fusível +
// a chamada ao barrier, porque cada gate é executado em sh -c separado.
func setupReentryFixture(t *testing.T, gateLines []string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	for _, d := range []string{
		"docs/roadmaps/wip", "docs/roadmaps/backlog", "docs/roadmaps/blocked",
		"docs/roadmaps/done", "docs/roadmaps/abandoned", "docs/req", "docs/adr",
	} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatalf("setupReentryFixture mkdirs: %v", err)
		}
	}
	roadmapRel := "docs/roadmaps/wip/ROADMAP-reentry-fixture.md"
	roadmapPath := filepath.Join(dir, roadmapRel)

	var sb strings.Builder
	sb.WriteString("---\nstatus: wip\ndate: 2026-09-30\n")
	sb.WriteString("req: \"" + barrierFixtureREQRel + "\"\n---\n\n")
	sb.WriteString("# Roadmap: Reentry Fixture\n\n")
	sb.WriteString("## Acceptance Criteria\n- [x] fixture criterion\n\n")
	// Wave 0 obrigatória
	sb.WriteString("## Wave 0 — Threat model\n> Dependências: nenhuma.\n\n")
	sb.WriteString("### ML-0A — Threat model\n**Status:** ✅ Concluído\n\n")
	sb.WriteString("**Gates da wave:**\n```bash\necho \"wave0 ok\"\n```\n\n")
	// Wave 1 com os gates fornecidos
	sb.WriteString("## Wave 1 — Fixture Wave\n> Dependências: nenhuma.\n\n")
	if len(gateLines) > 0 {
		sb.WriteString("**Gates da wave:**\n```bash\n")
		for _, g := range gateLines {
			sb.WriteString(g + "\n")
		}
		sb.WriteString("```\n\n")
	}
	sb.WriteString("### ML-1A — Fixture ML\n**Status:** ✅ Concluído\n")
	sb.WriteString("**Critérios de aceite:**\n- [x] fixture\n\n")

	if err := os.WriteFile(roadmapPath, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("setupReentryFixture write roadmap: %v", err)
	}
	writeBarrierREQFixture(t, dir, roadmapRel)
	return dir, roadmapPath
}

// symlinkOrSkip cria um symlink em link apontando para target. Se a criação
// falhar por falta de privilégio (WinError 1314, ERROR_PRIVILEGE_NOT_HELD, ou
// EPERM), pula o teste. Qualquer outro erro é um t.Fatalf.
// Cópia intencional do helper definido em outros pacotes do repositório —
// símbolos de _test.go não são importáveis entre pacotes no Go.
func symlinkOrSkip(t *testing.T, target, link string) bool { //nolint:unparam
	t.Helper()
	err := os.Symlink(target, link)
	if err == nil {
		return true
	}
	if isSymlinkPrivilegeErrorReentry(err) {
		t.Skipf("guarda de symlink não exercitada: criação de symlink exige privilégio: %v", err)
		return false
	}
	t.Fatalf("os.Symlink(%q, %q): %v", target, link, err)
	return false
}

// isSymlinkPrivilegeErrorReentry reporta se err é uma falha de privilégio de
// symlink (EPERM, EACCES, ou WinError 1314 no Windows sem Developer Mode).
func isSymlinkPrivilegeErrorReentry(err error) bool {
	if os.IsPermission(err) {
		return true
	}
	var e syscall.Errno
	if errors.As(err, &e) && e == 1314 {
		return true
	}
	return false
}

// stackJSON constrói o valor JSON para TRACKFW_BARRIER_STACK a partir de pares
// (roadmap, wave).
func stackJSON(t *testing.T, entries [][2]string) string {
	t.Helper()
	type entry struct {
		Roadmap string `json:"roadmap"`
		Wave    string `json:"wave"`
	}
	var es []entry
	for _, kv := range entries {
		es = append(es, entry{Roadmap: kv[0], Wave: kv[1]})
	}
	b, err := json.Marshal(es)
	if err != nil {
		t.Fatalf("stackJSON: %v", err)
	}
	return string(b)
}

// ────────────────────────────────────────────────────────────────────────────
// T1 — Reentrada direta
// Conclusão afirmada: o barrier recusa antes de executar um gate que chamaria
// a mesma wave do mesmo roadmap, saindo 2 com "reentrant call" no stderr filho.
// ────────────────────────────────────────────────────────────────────────────

func TestBarrierReentry_T1_DirectReentrance(t *testing.T) {
	// Gate: fusível + chamada barrier sobre o mesmo roadmap/wave.
	// Tudo em UMA linha porque cada gateCommands[i] é um sh -c independente.
	// $T_BIN, $T_DIR, $T_ROADMAP são passados por env (cmd.Env abaixo).
	fusedGate := `[ "${T_FUSE:-0}" -lt 3 ] || { echo fused >"$T_DIR/fuse.txt"; exit 99; }; export T_FUSE=$(( ${T_FUSE:-0} + 1 )); "$T_BIN" barrier "$T_ROADMAP" --wave 1 --trust-local-gates 2>>"$T_DIR/child.err"`

	dir2, roadmapPath2 := setupReentryFixture(t, []string{fusedGate})

	childErr := filepath.Join(dir2, "child.err")
	fuse := filepath.Join(dir2, "fuse.txt")

	env := cleanEnvForReentry()
	env = append(env,
		// ToSlash: Git Bash sh handles forward-slash Windows paths in
		// redirections and binary invocations; backslashes in env vars may
		// confuse the shell. barrierReentryKey normalises back to OS
		// separators internally via filepath.Abs, so key matching is unaffected.
		"T_BIN="+filepath.ToSlash(barrierBinary(t)),
		"T_DIR="+filepath.ToSlash(dir2),
		"T_ROADMAP="+filepath.ToSlash(roadmapPath2),
	)

	_, stderr, code := runBarrierReentry(t, dir2, env, roadmapPath2, "--wave", "1", "--trust-local-gates")

	// Após a correção: outer sai 1 (blocked — gate falhou com exit 2), fuse não dispara.
	if code != 1 {
		t.Errorf("T1: outer exit code = %d, quer 1 (blocked)", code)
	}
	if _, err := os.Stat(fuse); err == nil {
		t.Errorf("T1: fusível disparou — barrier recursou em vez de recusar a reentrada")
	}
	// O stderr do barrier filho deve conter "reentrant call"
	childStderr, _ := os.ReadFile(childErr)
	if !strings.Contains(string(childStderr), "reentrant call") {
		t.Errorf("T1: child.err = %q; quer conter \"reentrant call\"\nouter stderr = %q", string(childStderr), stderr)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T2 — Reentrada por indireção (sh ./reenter.sh)
// Conclusão afirmada: o barrier detecta reentrada mesmo quando o gate chama
// um script intermediário que, por sua vez, invoca o barrier no mesmo par.
// ────────────────────────────────────────────────────────────────────────────

func TestBarrierReentry_T2_IndirectReentrance(t *testing.T) {
	// Gate que chama um script intermediário reenter.sh, que por sua vez invoca
	// barrier no mesmo par — a reentrada é por indireção, não direta.
	fusedGate := `[ "${T_FUSE:-0}" -lt 3 ] || { echo fused >"$T_DIR/fuse.txt"; exit 99; }; export T_FUSE=$(( ${T_FUSE:-0} + 1 )); sh "$T_DIR/reenter.sh"`

	dir, roadmapPath := setupReentryFixture(t, []string{fusedGate})

	// Cria reenter.sh no diretório da fixture
	reenterScript := filepath.Join(dir, "reenter.sh")
	scriptContent := "#!/bin/sh\n" +
		`[ "${T_FUSE:-0}" -lt 3 ] || { echo fused >"$T_DIR/fuse.txt"; exit 99; }; export T_FUSE=$(( ${T_FUSE:-0} + 1 )); "$T_BIN" barrier "$T_ROADMAP" --wave 1 --trust-local-gates 2>>"$T_DIR/child.err"` + "\n"
	if err := os.WriteFile(reenterScript, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("T2: write reenter.sh: %v", err)
	}

	childErr := filepath.Join(dir, "child.err")
	fuse := filepath.Join(dir, "fuse.txt")

	env := cleanEnvForReentry()
	env = append(env,
		"T_BIN="+filepath.ToSlash(barrierBinary(t)),
		"T_DIR="+filepath.ToSlash(dir),
		"T_ROADMAP="+filepath.ToSlash(roadmapPath),
	)

	_, stderr, code := runBarrierReentry(t, dir, env, roadmapPath, "--wave", "1", "--trust-local-gates")

	if code != 1 {
		t.Errorf("T2: outer exit code = %d, quer 1 (blocked)", code)
	}
	if _, err := os.Stat(fuse); err == nil {
		t.Errorf("T2: fusível disparou — barrier recursou por indireção em vez de recusar")
	}
	childStderr, _ := os.ReadFile(childErr)
	if !strings.Contains(string(childStderr), "reentrant call") {
		t.Errorf("T2: child.err = %q; quer conter \"reentrant call\"\nouter stderr = %q", string(childStderr), stderr)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T3 — Gate legítimo aninhado (outro roadmap)
// Conclusão afirmada: o barrier NÃO recusa um gate que chama barrier sobre um
// roadmap diferente — a contenção é específica para o par (roadmap, wave) atual.
// ────────────────────────────────────────────────────────────────────────────

func TestBarrierReentry_T3_LegitimateNested(t *testing.T) {
	// Fixture B: roadmap diferente com gate trivial (exit 0); o barrier interno deve passar.
	dirB, roadmapB := setupReentryFixture(t, []string{`echo "inner ok"`})

	// Gate do roadmap A: grava a pilha, chama barrier sobre roadmap B (par diferente),
	// captura o exit code e grava em inner.rc; termina com esse exit code.
	// A pilha é gravada ANTES da chamada para que o gate do B já veja a chave de A.
	innerRC := filepath.Join(dirB, "inner.rc")
	innerErr := filepath.Join(dirB, "inner.err")
	stackFile := filepath.Join(dirB, "stack.txt")

	// Use printf instead of echo: some sh implementations (macOS /bin/sh with
	// xpg_echo) interpret backslash sequences in echo arguments, which would
	// corrupt the JSON value of TRACKFW_BARRIER_STACK on Windows paths.
	// Apply filepath.ToSlash to hardcoded paths embedded in the shell string
	// so that Git Bash sh receives unambiguous forward-slash paths in redirections.
	gateA := `printf '%s\n' "$TRACKFW_BARRIER_STACK" >"` + filepath.ToSlash(stackFile) + `"; ` +
		`cd "$T_DIR_B" && "$T_BIN" barrier "$T_ROADMAP_B" --wave 1 --trust-local-gates 2>"` + filepath.ToSlash(innerErr) + `"; ` +
		`rc=$?; echo $rc >"` + filepath.ToSlash(innerRC) + `"; exit $rc`

	dirA, roadmapA := setupReentryFixture(t, []string{gateA})

	env := cleanEnvForReentry()
	env = append(env,
		"T_BIN="+filepath.ToSlash(barrierBinary(t)),
		"T_DIR_B="+filepath.ToSlash(dirB),
		"T_ROADMAP_B="+filepath.ToSlash(roadmapB),
	)

	_, stderr, code := runBarrierReentry(t, dirA, env, roadmapA, "--wave", "1", "--trust-local-gates")

	// AC1: outer (roadmap A) deve sair 0 — gate legítimo passou
	if code != 0 {
		t.Fatalf("T3: outer exit code = %d, quer 0 (passed); stderr = %q", code, stderr)
	}

	// AC2: inner.rc deve ser "0" — barrier sobre roadmap B passou
	rcBytes, err := os.ReadFile(innerRC)
	if err != nil {
		t.Fatalf("T3: inner.rc não encontrado (%v) — gate pode não ter chegado a chamar o barrier interno", err)
	}
	if strings.TrimSpace(string(rcBytes)) != "0" {
		innerErrBytes, _ := os.ReadFile(innerErr)
		t.Fatalf("T3: inner.rc = %q, quer \"0\"; inner.err = %q", strings.TrimSpace(string(rcBytes)), string(innerErrBytes))
	}

	// AC3: stack.txt deve conter exactamente uma entrada com o path canónico de
	// roadmapA e Wave "1". A pilha do outer começa vazia; quando o gate executa,
	// o outer já inseriu a sua chave — única entrada esperada.
	// Oráculo exacto: barrierReentryKey armazena EvalSymlinks(Abs(roadmapPath));
	// reproduzimos o mesmo cálculo independentemente para o comparar por string.
	//
	// NOTA Windows: buildChildEnv chama json.Marshal que escapa '\' como '\\'.
	// Medido (scratchpad/json_escape_main.go, Go 1.23 darwin):
	//   raw JSON: [{"roadmap":"C:\\a\\b.md","wave":"1"}]
	//   strings.Contains(raw, C:\a\b.md): false
	//   decoded[0].Roadmap == C:\a\b.md: true
	// Logo: strings.Contains sobre os bytes crus falha no Windows. Decodificar o
	// JSON com json.Unmarshal primeiro e então comparar por string é o padrão correto.
	resolvedA, err := filepath.EvalSymlinks(roadmapA)
	if err != nil {
		resolvedA = roadmapA // fallback se EvalSymlinks falhar
	}
	stackContent, err := os.ReadFile(stackFile)
	if err != nil {
		t.Fatalf("T3: stack.txt não encontrado (%v)", err)
	}
	var stackEntries []struct {
		Roadmap string `json:"roadmap"`
		Wave    string `json:"wave"`
	}
	if jsonErr := json.Unmarshal(stackContent, &stackEntries); jsonErr != nil {
		t.Fatalf("T3: stack.txt não é JSON válido: %v\nconteúdo: %q", jsonErr, string(stackContent))
	}
	if len(stackEntries) != 1 {
		t.Errorf("T3: stack tem %d entradas, quer exactamente 1; entradas = %v", len(stackEntries), stackEntries)
	} else if stackEntries[0].Roadmap != resolvedA || stackEntries[0].Wave != "1" {
		t.Errorf("T3: stack[0] = {Roadmap: %q, Wave: %q}, quer {Roadmap: %q, Wave: %q}",
			stackEntries[0].Roadmap, stackEntries[0].Wave, resolvedA, "1")
	}

	// AC4: inner.err NÃO deve conter "reentrant call" nem "depth limit"
	innerErrBytes, _ := os.ReadFile(innerErr)
	if strings.Contains(string(innerErrBytes), "reentrant call") {
		t.Errorf("T3: inner.err contém \"reentrant call\" — barrier recusou gate legítimo; inner.err = %q", string(innerErrBytes))
	}
	if strings.Contains(string(innerErrBytes), "depth limit") {
		t.Errorf("T3: inner.err contém \"depth limit\" — barrier recusou gate legítimo; inner.err = %q", string(innerErrBytes))
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T4 — Duas execuções sequenciais do mesmo par
// Conclusão afirmada: a pilha é por processo, não persistida; duas chamadas
// independentes do mesmo (roadmap, wave) funcionam normalmente.
// ────────────────────────────────────────────────────────────────────────────

func TestBarrierReentry_T4_SequentialCallsPass(t *testing.T) {
	dir, roadmapPath := setupReentryFixture(t, []string{`echo "ok"`})

	env := cleanEnvForReentry()

	_, stderr1, code1 := runBarrierReentry(t, dir, env, roadmapPath, "--wave", "1", "--trust-local-gates")
	if code1 != 0 {
		t.Errorf("T4: primeira chamada exit code = %d, quer 0; stderr = %q", code1, stderr1)
	}
	_, stderr2, code2 := runBarrierReentry(t, dir, env, roadmapPath, "--wave", "1", "--trust-local-gates")
	if code2 != 0 {
		t.Errorf("T4: segunda chamada exit code = %d, quer 0; stderr = %q", code2, stderr2)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T5 — Backstop: pilha pré-carregada com barrierMaxDepth chaves distintas
// Conclusão afirmada: quando TRACKFW_BARRIER_STACK já tem barrierMaxDepth entradas
// (nenhuma a chave atual), o barrier recusa com "evaluation depth limit exceeded"
// antes de executar qualquer gate (sentinel ausente). O número de entradas é lido
// da constante barrierMaxDepth, de modo que o teste se ajusta automaticamente se
// o limite for revisado.
// ────────────────────────────────────────────────────────────────────────────

func TestBarrierReentry_T5_DepthLimitBackstop(t *testing.T) {
	dir2, roadmapPath2 := setupReentryFixture(t, nil)

	// Gate que escreveria um sentinel — deve NÃO ser executado
	sentinel := filepath.Join(dir2, "sentinel.txt")
	dir3, roadmapPath3 := setupReentryFixture(t, []string{`touch "` + sentinel + `"`})
	_ = dir2
	_ = roadmapPath2

	// Pilha com barrierMaxDepth chaves DISTINTAS (nenhuma é roadmapPath3 / wave "1")
	entries := make([][2]string, barrierMaxDepth)
	for i := 0; i < barrierMaxDepth; i++ {
		entries[i] = [2]string{fmt.Sprintf("/fake/roadmap-%c.md", 'a'+i), fmt.Sprintf("%d", i+1)}
	}
	preStack := stackJSON(t, entries)

	env := cleanEnvForReentry()
	env = append(env, "TRACKFW_BARRIER_STACK="+preStack)

	_, stderr, code := runBarrierReentry(t, dir3, env, roadmapPath3, "--wave", "1", "--trust-local-gates")

	if code != 2 {
		t.Errorf("T5: exit code = %d, quer 2; stderr = %q", code, stderr)
	}
	if !strings.Contains(stderr, "evaluation depth limit exceeded") {
		t.Errorf("T5: stderr = %q; quer conter \"evaluation depth limit exceeded\"", stderr)
	}
	if _, err := os.Stat(sentinel); err == nil {
		t.Errorf("T5: sentinel.txt existe — backstop não impediu execução do gate")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T6 — Pilha malformada (não-JSON)
// Conclusão afirmada: quando TRACKFW_BARRIER_STACK contém JSON inválido, o
// barrier recusa com exit 2 e "is malformed".
// ────────────────────────────────────────────────────────────────────────────

func TestBarrierReentry_T6_MalformedStack(t *testing.T) {
	dir, roadmapPath := setupReentryFixture(t, []string{`echo "ok"`})

	env := cleanEnvForReentry()
	env = append(env, "TRACKFW_BARRIER_STACK=not-json")

	_, stderr, code := runBarrierReentry(t, dir, env, roadmapPath, "--wave", "1", "--trust-local-gates")

	if code != 2 {
		t.Errorf("T6: exit code = %d, quer 2; stderr = %q", code, stderr)
	}
	if !strings.Contains(stderr, "is malformed") {
		t.Errorf("T6: stderr = %q; quer conter \"is malformed\"", stderr)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T7 — Canonicalização: symlink e sufixo 1-b/1b
// Conclusão afirmada: (a) a reentrada via symlink do mesmo roadmap é recusada
// (EvalSymlinks normaliza o path); (b) a reentrada da wave "1-b" contra "1b"
// é recusada (SplitWaveLabel normaliza o sufixo).
// ────────────────────────────────────────────────────────────────────────────

func TestBarrierReentry_T7_Canonicalization(t *testing.T) {
	// No unconditional GOOS skip: symlinkOrSkip in subtest 7a handles privilege
	// errors on Windows (Developer Mode off → ERROR_PRIVILEGE_NOT_HELD → t.Skipf).
	// Subtest 7b (wave label normalisation) uses no symlinks and runs everywhere.

	// ── 7a: symlink ──────────────────────────────────────────────────────────
	t.Run("symlink", func(t *testing.T) {
		dir, roadmapPath := setupReentryFixture(t, nil)

		// Cria symlink apontando para o mesmo roadmap
		symlinkPath := filepath.Join(dir, "docs/roadmaps/wip/ROADMAP-reentry-sym.md")
		if !symlinkOrSkip(t, roadmapPath, symlinkPath) {
			return
		}

		// Gate: chama barrier via symlink do mesmo roadmap, mesma wave
		fusedGate := `[ "${T_FUSE:-0}" -lt 3 ] || { echo fused >"$T_DIR/fuse.txt"; exit 99; }; export T_FUSE=$(( ${T_FUSE:-0} + 1 )); "$T_BIN" barrier "$T_SYM_ROADMAP" --wave 1 --trust-local-gates 2>>"$T_DIR/child.err"`
		dir2, roadmapPath2 := setupReentryFixture(t, []string{fusedGate})

		// Cria symlink no dir2 também
		symInDir2 := filepath.Join(dir2, "docs/roadmaps/wip/ROADMAP-reentry-sym.md")
		if !symlinkOrSkip(t, roadmapPath2, symInDir2) {
			return
		}

		childErr := filepath.Join(dir2, "child.err")
		fuse := filepath.Join(dir2, "fuse.txt")

		env := cleanEnvForReentry()
		env = append(env,
			"T_BIN="+filepath.ToSlash(barrierBinary(t)),
			"T_DIR="+filepath.ToSlash(dir2),
			"T_SYM_ROADMAP="+filepath.ToSlash(symInDir2),
		)

		_, stderr, code := runBarrierReentry(t, dir2, env, roadmapPath2, "--wave", "1", "--trust-local-gates")

		if code != 1 {
			t.Errorf("T7/symlink: outer exit code = %d, quer 1 (blocked); stderr = %q", code, stderr)
		}
		if _, err := os.Stat(fuse); err == nil {
			t.Errorf("T7/symlink: fusível disparou — symlink não foi resolvido para o mesmo path")
		}
		childStderr, _ := os.ReadFile(childErr)
		if !strings.Contains(string(childStderr), "reentrant call") {
			t.Errorf("T7/symlink: child.err = %q; quer conter \"reentrant call\"", string(childStderr))
		}
	})

	// ── 7b: sufixo 1-b vs 1b ────────────────────────────────────────────────
	t.Run("wave_label_normalization", func(t *testing.T) {
		// Monta fixture com Wave "1b" (sufixo sem hífen)
		dir := t.TempDir()
		for _, d := range []string{
			"docs/roadmaps/wip", "docs/roadmaps/backlog", "docs/roadmaps/blocked",
			"docs/roadmaps/done", "docs/roadmaps/abandoned", "docs/req", "docs/adr",
		} {
			if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
				t.Fatalf("T7/wave: mkdirs: %v", err)
			}
		}
		roadmapRel := "docs/roadmaps/wip/ROADMAP-wave-suffix-fixture.md"
		roadmapPath := filepath.Join(dir, roadmapRel)
		childErr := filepath.Join(dir, "child.err")
		fuse := filepath.Join(dir, "fuse.txt")

		// Gate da wave 1b: chama barrier com --wave 1-b (forma canônica equivalente)
		fusedGate := `[ "${T_FUSE:-0}" -lt 3 ] || { echo fused >"$T_DIR/fuse.txt"; exit 99; }; export T_FUSE=$(( ${T_FUSE:-0} + 1 )); "$T_BIN" barrier "$T_ROADMAP" --wave 1-b --trust-local-gates 2>>"$T_DIR/child.err"`

		var sb strings.Builder
		sb.WriteString("---\nstatus: wip\ndate: 2026-09-30\n")
		sb.WriteString("req: \"" + barrierFixtureREQRel + "\"\n---\n\n")
		sb.WriteString("# Roadmap: Wave Label Fixture\n\n")
		sb.WriteString("## Acceptance Criteria\n- [x] fixture\n\n")
		sb.WriteString("## Wave 0 — Threat model\n> Dependências: nenhuma.\n\n")
		sb.WriteString("### ML-0A — TM\n**Status:** ✅ Concluído\n\n")
		sb.WriteString("**Gates da wave:**\n```bash\necho ok\n```\n\n")
		sb.WriteString("## Wave 1b — Suffix wave\n> Dependências: nenhuma.\n\n")
		sb.WriteString("**Gates da wave:**\n```bash\n")
		sb.WriteString(fusedGate + "\n")
		sb.WriteString("```\n\n")
		sb.WriteString("### ML-1bA — Fixture ML\n**Status:** ✅ Concluído\n")
		sb.WriteString("**Critérios de aceite:**\n- [x] fixture\n\n")

		if err := os.WriteFile(roadmapPath, []byte(sb.String()), 0644); err != nil {
			t.Fatalf("T7/wave: write roadmap: %v", err)
		}
		writeBarrierREQFixture(t, dir, roadmapRel)

		env := cleanEnvForReentry()
		env = append(env,
			"T_BIN="+filepath.ToSlash(barrierBinary(t)),
			"T_DIR="+filepath.ToSlash(dir),
			"T_ROADMAP="+filepath.ToSlash(roadmapPath),
		)

		// Primeiro: verifica que a wave "1b" existe (baseline, deve sair 0 se
		// removermos o gate — verificação indireta via gate pass/fail)
		// Com o gate, deve sair 1 (blocked) por reentrada

		_, stderr, code := runBarrierReentry(t, dir, env, roadmapPath, "--wave", "1b", "--trust-local-gates")

		if code != 1 {
			t.Errorf("T7/wave: outer exit code = %d, quer 1 (blocked); stderr = %q", code, stderr)
		}
		if _, err := os.Stat(fuse); err == nil {
			t.Errorf("T7/wave: fusível disparou — sufixo 1b/1-b não foi canonizado para a mesma chave")
		}
		childStderr, _ := os.ReadFile(childErr)
		if !strings.Contains(string(childStderr), "reentrant call") {
			t.Errorf("T7/wave: child.err = %q; quer conter \"reentrant call\"", string(childStderr))
		}
	})
}

// ────────────────────────────────────────────────────────────────────────────
// T7c — Case alias via stack injection: stack tem "1b", call com --wave 1B
// Conclusão afirmada: barrierReentryKey usa target.Label (o header parsed), não
// waveLabel (o CLI arg); quando o stack já contém a chave canônica do header
// ("1b"), uma chamada com --wave 1B deve ser recusada como reentrada (exit 2).
// Sem a correção (F1), waveLabel="1B" → chave "1B" ≠ "1b" → bypass (exit 1).
//
// Usa injeção direta de TRACKFW_BARRIER_STACK (como T5/T6) para isolar o mecanismo
// de chave sem envolver a cadeia de gate recursiva.
// ────────────────────────────────────────────────────────────────────────────

func TestBarrierReentry_T7c_CaseAlias(t *testing.T) {
	// Monta fixture com Wave "1b" (header lowercase).
	dir := t.TempDir()
	for _, d := range []string{
		"docs/roadmaps/wip", "docs/roadmaps/backlog", "docs/roadmaps/blocked",
		"docs/roadmaps/done", "docs/roadmaps/abandoned", "docs/req", "docs/adr",
	} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatalf("T7c: mkdirs: %v", err)
		}
	}
	roadmapRel := "docs/roadmaps/wip/ROADMAP-wave-case-fixture.md"
	roadmapPath := filepath.Join(dir, roadmapRel)

	var sb strings.Builder
	sb.WriteString("---\nstatus: wip\ndate: 2026-09-30\n")
	sb.WriteString("req: \"" + barrierFixtureREQRel + "\"\n---\n\n")
	sb.WriteString("# Roadmap: Wave Case Fixture\n\n")
	sb.WriteString("## Acceptance Criteria\n- [x] fixture\n\n")
	sb.WriteString("## Wave 0 — Threat model\n> Dependências: nenhuma.\n\n")
	sb.WriteString("### ML-0A — TM\n**Status:** ✅ Concluído\n\n")
	sb.WriteString("**Gates da wave:**\n```bash\necho ok\n```\n\n")
	// Wave header "1b" lowercase — target.Label = "1b"
	sb.WriteString("## Wave 1b — Case alias wave\n> Dependências: nenhuma.\n\n")
	sb.WriteString("**Gates da wave:**\n```bash\necho ok\n```\n\n")
	sb.WriteString("### ML-1bA — Fixture ML\n**Status:** ✅ Concluído\n")
	sb.WriteString("**Critérios de aceite:**\n- [x] fixture\n\n")

	if err := os.WriteFile(roadmapPath, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("T7c: write roadmap: %v", err)
	}
	writeBarrierREQFixture(t, dir, roadmapRel)

	// Resolve the roadmap path to the canonical key (same as barrierReentryKey does).
	abs, _ := filepath.Abs(roadmapPath)
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		resolved = abs
	}

	// Pre-load stack with the canonical key for wave "1b" (lowercase, as the header has it).
	// This is what the outer barrier puts in the stack when called with --wave 1b.
	preStack := stackJSON(t, [][2]string{
		{resolved, "1b"},
	})

	env := cleanEnvForReentry()
	env = append(env, "TRACKFW_BARRIER_STACK="+preStack)

	// Call with --wave 1B (uppercase).
	// With F1 bug: barrierReentryKey uses waveLabel="1B" → key.Wave="1B" ≠ "1b" → bypass → exit 1.
	// With F1 fix: barrierReentryKey uses target.Label="1b" (header) → key.Wave="1b" = "1b" → exit 2.
	_, stderr, code := runBarrierReentry(t, dir, env, roadmapPath, "--wave", "1B", "--trust-local-gates")

	if code != 2 {
		t.Errorf("T7c: exit code = %d, quer 2 (reentrant call recusada); stderr = %q", code, stderr)
	}
	if !strings.Contains(stderr, "reentrant call") {
		t.Errorf("T7c: stderr = %q; quer conter \"reentrant call\"", stderr)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T7d — Hardlink: dois nomes, mesmo inode
// Conclusão afirmada: sameRoadmapFile usa os.SameFile (inode+device), de modo
// que um gate que chama barrier com um hardlink do roadmap outer é detectado
// como reentrada mesmo que EvalSymlinks retorne strings diferentes.
// ────────────────────────────────────────────────────────────────────────────

func TestBarrierReentry_T7d_Hardlink(t *testing.T) {
	// Monta fixture com gate trivial (echo ok — passa normalmente se não houver reentrada).
	dir2, roadmapPath2 := setupReentryFixture(t, []string{`echo ok`})

	// Cria hardlink para roadmapPath2 no mesmo diretório.
	hlPath2 := filepath.Join(filepath.Dir(roadmapPath2), "ROADMAP-reentry-hardlink.md")
	if err := os.Link(roadmapPath2, hlPath2); err != nil {
		t.Skipf("T7d: os.Link falhou (%v) — filesystem pode não suportar hardlinks", err)
	}

	// Resolve o hlPath2 para o path canônico (EvalSymlinks não resolve hardlinks —
	// retorna o path como dado). Este é o path que o outer teria colocado na pilha
	// se tivesse sido chamado com o hardlink.
	absHl, _ := filepath.Abs(hlPath2)
	resolvedHl, err := filepath.EvalSymlinks(absHl)
	if err != nil {
		resolvedHl = absHl
	}

	// Resolve também o path original, para comparar strings e confirmar que são DIFERENTES.
	absOrig, _ := filepath.Abs(roadmapPath2)
	resolvedOrig, err := filepath.EvalSymlinks(absOrig)
	if err != nil {
		resolvedOrig = absOrig
	}

	if resolvedHl == resolvedOrig {
		// Filesystem resolveu os dois para o mesmo path (improvável em macOS/Linux via os.Link),
		// o que tornaria o teste trivial — pular.
		t.Skip("T7d: EvalSymlinks resolveu hardlink e original para o mesmo path — skip")
	}

	// Injetar stack com o path do HARDLINK (como se o outer tivesse sido chamado com hlPath2).
	// Call com o path ORIGINAL (roadmapPath2).
	// Com F2 bug (string compare): resolvedHl != resolvedOrig → sem match → bypass → exit 0.
	// Com F2 fix (os.SameFile):   mesmo inode → match → "reentrant call" → exit 2.
	preStack := stackJSON(t, [][2]string{
		{resolvedHl, "1"},
	})

	env := cleanEnvForReentry()
	env = append(env, "TRACKFW_BARRIER_STACK="+preStack)

	_, stderr, code := runBarrierReentry(t, dir2, env, roadmapPath2, "--wave", "1", "--trust-local-gates")

	if code != 2 {
		t.Errorf("T7d: exit code = %d, quer 2 (reentrant call recusada); stderr = %q", code, stderr)
	}
	if !strings.Contains(stderr, "reentrant call") {
		t.Errorf("T7d: stderr = %q; quer conter \"reentrant call\"", stderr)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// T7e — APFS/case-insensitive: mesmo arquivo, basename com case diferente
// Conclusão afirmada: em filesystem case-insensitive (APFS, HFS+), os.SameFile
// retorna true para dois paths que diferem só em case e apontam ao mesmo inode,
// de modo que a reentrada via nome alternativo é detectada.
// O teste é pulado se o filesystem for case-sensitive (detectado via os.SameFile).
// ────────────────────────────────────────────────────────────────────────────

func TestBarrierReentry_T7e_APFSCaseInsensitive(t *testing.T) {
	// Monta fixture com gate trivial.
	dir2, roadmapPath2 := setupReentryFixture(t, []string{`echo ok`})

	// Detecta se o filesystem é case-insensitive usando os.SameFile entre dois paths
	// que diferem só em case. setupReentryFixture cria "ROADMAP-reentry-fixture.md";
	// em APFS, os.Stat("ROADMAP-REENTRY-FIXTURE.MD") retorna o mesmo inode.
	upperPath2 := filepath.Join(filepath.Dir(roadmapPath2), "ROADMAP-REENTRY-FIXTURE.MD")
	fi2Lower, err2Lower := os.Stat(roadmapPath2)
	fi2Upper, err2Upper := os.Stat(upperPath2)
	if err2Lower != nil || err2Upper != nil || !os.SameFile(fi2Lower, fi2Upper) {
		t.Skip("T7e: filesystem é case-sensitive ou path alternativo não acessível — skip")
	}

	// Resolve o path original para o canônico que o barrier colocaria na pilha.
	absOrig, _ := filepath.Abs(roadmapPath2)
	resolvedOrig, err := filepath.EvalSymlinks(absOrig)
	if err != nil {
		resolvedOrig = absOrig
	}

	// Resolve também o path uppercase para confirmar que EvalSymlinks retorna strings
	// diferentes (APFS preserva o case fornecido, não normaliza).
	absUpper, _ := filepath.Abs(upperPath2)
	resolvedUpper, err := filepath.EvalSymlinks(absUpper)
	if err != nil {
		resolvedUpper = absUpper
	}

	if resolvedOrig == resolvedUpper {
		// EvalSymlinks normalizou o case: em NTFS, filepath.EvalSymlinks chama
		// toNorm → normBase → syscall.FindFirstFile, que devolve o nome canônico
		// do arquivo (o case registrado na criação), tornando resolvedOrig ==
		// resolvedUpper. Com isso, sameRoadmapFile detecta a reentrada pelo caminho
		// rápido (string equality), não pelo os.SameFile — o bypass que este teste
		// mede (caminhos com case diferente → strings diferentes → os.SameFile)
		// não ocorre no Windows. Comportamento APFS (EvalSymlinks preserva o case
		// fornecido, logo os dois strings ficam diferentes) não é reproduzível no
		// Windows. // (c) plataforma
		t.Skip("T7e: EvalSymlinks normalizou case via FindFirstFile (NTFS) — comportamento APFS não reproduzível no Windows // (c)")
	}

	// Injetar stack com o path UPPERCASE (como se o outer tivesse sido chamado com upperPath2).
	// Call com o path LOWERCASE (roadmapPath2).
	// Com F2 bug (string compare): resolvedUpper != resolvedOrig → sem match → bypass → exit 0.
	// Com F2 fix (os.SameFile):   mesmo inode (APFS case-insensitive) → match → exit 2.
	preStack := stackJSON(t, [][2]string{
		{resolvedUpper, "1"},
	})

	env := cleanEnvForReentry()
	env = append(env, "TRACKFW_BARRIER_STACK="+preStack)

	_, stderr, code := runBarrierReentry(t, dir2, env, roadmapPath2, "--wave", "1", "--trust-local-gates")

	if code != 2 {
		t.Errorf("T7e: exit code = %d, quer 2 (reentrant call recusada); stderr = %q", code, stderr)
	}
	if !strings.Contains(stderr, "reentrant call") {
		t.Errorf("T7e: stderr = %q; quer conter \"reentrant call\"", stderr)
	}
}
