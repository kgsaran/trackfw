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
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
	if runtime.GOOS == "windows" {
		t.Skip("shell gate com fusível requer sh POSIX; Windows coberto por windows-known-failures.json")
	}

	// Gate: fusível + chamada barrier sobre o mesmo roadmap/wave.
	// Tudo em UMA linha porque cada gateCommands[i] é um sh -c independente.
	// $T_BIN, $T_DIR, $T_ROADMAP são passados por env (cmd.Env abaixo).
	fusedGate := `[ "${T_FUSE:-0}" -lt 3 ] || { echo fused >"$T_DIR/fuse.txt"; exit 99; }; export T_FUSE=$(( ${T_FUSE:-0} + 1 )); "$T_BIN" barrier "$T_ROADMAP" --wave 1 --trust-local-gates 2>>"$T_DIR/child.err"`

	dir2, roadmapPath2 := setupReentryFixture(t, []string{fusedGate})

	childErr := filepath.Join(dir2, "child.err")
	fuse := filepath.Join(dir2, "fuse.txt")

	env := cleanEnvForReentry()
	env = append(env,
		"T_BIN="+barrierBinary(t),
		"T_DIR="+dir2,
		"T_ROADMAP="+roadmapPath2,
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
	if runtime.GOOS == "windows" {
		t.Skip("shell gate com fusível requer sh POSIX; Windows coberto por windows-known-failures.json")
	}

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
		"T_BIN="+barrierBinary(t),
		"T_DIR="+dir,
		"T_ROADMAP="+roadmapPath,
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
	if runtime.GOOS == "windows" {
		t.Skip("shell gate requer sh POSIX; Windows coberto por windows-known-files.json")
	}

	// Fixture B: roadmap diferente com gate trivial (exit 0); o barrier interno deve passar.
	dirB, roadmapB := setupReentryFixture(t, []string{`echo "inner ok"`})

	// Gate do roadmap A: grava a pilha, chama barrier sobre roadmap B (par diferente),
	// captura o exit code e grava em inner.rc; termina com esse exit code.
	// A pilha é gravada ANTES da chamada para que o gate do B já veja a chave de A.
	innerRC := filepath.Join(dirB, "inner.rc")
	innerErr := filepath.Join(dirB, "inner.err")
	stackFile := filepath.Join(dirB, "stack.txt")

	gateA := `echo "$TRACKFW_BARRIER_STACK" >"` + stackFile + `"; ` +
		`cd "$T_DIR_B" && "$T_BIN" barrier "$T_ROADMAP_B" --wave 1 --trust-local-gates 2>"` + innerErr + `"; ` +
		`rc=$?; echo $rc >"` + innerRC + `"; exit $rc`

	dirA, roadmapA := setupReentryFixture(t, []string{gateA})

	env := cleanEnvForReentry()
	env = append(env,
		"T_BIN="+barrierBinary(t),
		"T_DIR_B="+dirB,
		"T_ROADMAP_B="+roadmapB,
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

	// AC3: stack.txt deve conter o path resolvido do roadmapA (a chave que A colocou na pilha)
	resolvedA, err := filepath.EvalSymlinks(roadmapA)
	if err != nil {
		resolvedA = roadmapA // fallback se EvalSymlinks falhar
	}
	stackContent, err := os.ReadFile(stackFile)
	if err != nil {
		t.Fatalf("T3: stack.txt não encontrado (%v)", err)
	}
	if !strings.Contains(string(stackContent), resolvedA) {
		t.Errorf("T3: stack.txt = %q; esperava conter o path resolvido de roadmapA %q", string(stackContent), resolvedA)
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
// T5 — Backstop: pilha pré-carregada com 4 chaves distintas
// Conclusão afirmada: quando TRACKFW_BARRIER_STACK já tem 4 entradas (nenhuma
// a chave atual), o barrier recusa com "evaluation depth limit exceeded" antes
// de executar qualquer gate (sentinel ausente).
// ────────────────────────────────────────────────────────────────────────────

func TestBarrierReentry_T5_DepthLimitBackstop(t *testing.T) {
	dir, roadmapPath := setupReentryFixture(t, nil)

	// Gate que escreveria um sentinel — deve NÃO ser executado
	sentinel := filepath.Join(dir, "sentinel.txt")
	dir2, roadmapPath2 := setupReentryFixture(t, []string{`touch "` + sentinel + `"`})
	_ = dir
	_ = roadmapPath

	// Pilha com 4 chaves DISTINTAS (nenhuma é roadmapPath2 / wave "1")
	preStack := stackJSON(t, [][2]string{
		{"/fake/roadmap-a.md", "1"},
		{"/fake/roadmap-b.md", "2"},
		{"/fake/roadmap-c.md", "3"},
		{"/fake/roadmap-d.md", "4"},
	})

	env := cleanEnvForReentry()
	env = append(env, "TRACKFW_BARRIER_STACK="+preStack)

	_, stderr, code := runBarrierReentry(t, dir2, env, roadmapPath2, "--wave", "1", "--trust-local-gates")

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
	if runtime.GOOS == "windows" {
		t.Skip("os.Symlink pode falhar no Windows sem permissão de admin")
	}

	// ── 7a: symlink ──────────────────────────────────────────────────────────
	t.Run("symlink", func(t *testing.T) {
		dir, roadmapPath := setupReentryFixture(t, nil)

		// Cria symlink apontando para o mesmo roadmap
		symlinkPath := filepath.Join(dir, "docs/roadmaps/wip/ROADMAP-reentry-sym.md")
		if err := os.Symlink(roadmapPath, symlinkPath); err != nil {
			t.Skip("os.Symlink falhou: " + err.Error())
		}

		// Gate: chama barrier via symlink do mesmo roadmap, mesma wave
		fusedGate := `[ "${T_FUSE:-0}" -lt 3 ] || { echo fused >"$T_DIR/fuse.txt"; exit 99; }; export T_FUSE=$(( ${T_FUSE:-0} + 1 )); "$T_BIN" barrier "$T_SYM_ROADMAP" --wave 1 --trust-local-gates 2>>"$T_DIR/child.err"`
		dir2, roadmapPath2 := setupReentryFixture(t, []string{fusedGate})

		// Cria symlink no dir2 também
		symInDir2 := filepath.Join(dir2, "docs/roadmaps/wip/ROADMAP-reentry-sym.md")
		if err := os.Symlink(roadmapPath2, symInDir2); err != nil {
			t.Skip("os.Symlink falhou no dir2: " + err.Error())
		}

		childErr := filepath.Join(dir2, "child.err")
		fuse := filepath.Join(dir2, "fuse.txt")

		env := cleanEnvForReentry()
		env = append(env,
			"T_BIN="+barrierBinary(t),
			"T_DIR="+dir2,
			"T_SYM_ROADMAP="+symInDir2,
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
			"T_BIN="+barrierBinary(t),
			"T_DIR="+dir,
			"T_ROADMAP="+roadmapPath,
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
