package generators

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ML-3D (ROADMAP-2026-09-09-req-nasce-orfa-…): os ramos de generateGitIgnore.
// O ML-3A criou <roadmap_dir>/.trackfw-branch-links.json — estado POR CHECKOUT — e
// o ignorou apenas no .gitignore DESTE repositório. No consumidor o arquivo nascia
// rastreado, e o vínculo de uma máquina passava a governar outra. Estes testes
// afirmam que `trackfw init` fecha isso sem tocar no .gitignore do projeto.
//
// chdirTemp vem de gitattributes_test.go (mesmo pacote).

// Reconciliação: afirma o AC1 (o `init` acrescenta a linha, com o roadmap_dir
// default quando não há trackfw.yaml) e o contra-braço 2 (rodar duas vezes não
// duplica a linha).
func TestGenerateGitIgnore_CriaQuandoAusenteEIdempotente(t *testing.T) {
	dir := chdirTemp(t)
	if err := generateGitIgnore(); err != nil {
		t.Fatalf("generateGitIgnore: %v", err)
	}
	first, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("ler .gitignore: %v", err)
	}
	if !strings.Contains(string(first), "docs/roadmaps/.trackfw-branch-links.json\n") {
		t.Fatalf("linha de ignore ausente no arquivo criado:\n%q", string(first))
	}
	// Segunda execução: no-op, byte a byte.
	if err := generateGitIgnore(); err != nil {
		t.Fatalf("generateGitIgnore (2ª): %v", err)
	}
	second, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if string(second) != string(first) {
		t.Fatalf("init duas vezes duplicou/alterou o arquivo:\n%q", string(second))
	}
	if n := strings.Count(string(second), ".trackfw-branch-links.json"); n != 1 {
		t.Fatalf("esperava 1 menção ao arquivo de vínculo, achei %d:\n%q", n, string(second))
	}
}

// Reconciliação: afirma que o caminho emitido vem do `roadmap_dir` EFETIVO do
// trackfw.yaml do projeto, e não de `docs/roadmaps` fixo — é o único teste que
// falsifica o hardcode (família de defeito do #396). Sem trackfw.yaml não-default,
// o teste passaria idêntico nas duas implementações.
func TestGenerateGitIgnore_UsaRoadmapDirEfetivoNaoODoMantenedor(t *testing.T) {
	dir := chdirTemp(t)
	cfg := "req_dir: governance/reqs\nroadmap_dir: governance/plans\n"
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(cfg), 0644); err != nil {
		t.Fatalf("preparar trackfw.yaml: %v", err)
	}
	if err := generateGitIgnore(); err != nil {
		t.Fatalf("generateGitIgnore: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("ler .gitignore: %v", err)
	}
	if !strings.Contains(string(got), "governance/plans/.trackfw-branch-links.json\n") {
		t.Fatalf("não usou o roadmap_dir do projeto:\n%q", string(got))
	}
	if strings.Contains(string(got), "docs/roadmaps") {
		t.Fatalf("emitiu o layout do mantenedor num projeto com roadmap_dir próprio:\n%q", string(got))
	}
}

// Reconciliação: afirma o contra-braço 1 — o .gitignore preexistente do projeto
// não é sobrescrito, e o append não gruda o bloco na última linha quando o arquivo
// não termina em newline (que corromperia o padrão do projeto em silêncio).
// ML-3C: o append agora inclui os dois blocos (branch-links e leases).
// ML-1B (#646): agora inclui também o bloco agent-memory.
func TestGenerateGitIgnore_AppendPreservaArquivoPreexistenteSemNewlineFinal(t *testing.T) {
	dir := chdirTemp(t)
	path := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(path, []byte("/node_modules\n*.log"), 0644); err != nil {
		t.Fatalf("preparar arquivo: %v", err)
	}
	if err := generateGitIgnore(); err != nil {
		t.Fatalf("generateGitIgnore: %v", err)
	}
	got, _ := os.ReadFile(path)
	want := "/node_modules\n*.log\n" + gitIgnoreBlock("docs/roadmaps") + gitIgnoreLeasesBlock() + gitIgnoreAgentMemoryBlock()
	if string(got) != want {
		t.Fatalf("append incorreto:\ngot:  %q\nwant: %q", string(got), want)
	}
	if err := generateGitIgnore(); err != nil {
		t.Fatalf("generateGitIgnore (2ª): %v", err)
	}
	again, _ := os.ReadFile(path)
	if string(again) != want {
		t.Fatalf("segunda execução duplicou a regra:\n%q", string(again))
	}
}

// Reconciliação: afirma que o reconhecimento é por BASENAME, não pelo caminho
// literal emitido — um projeto que já ignora o arquivo em outro caminho (ou que
// mudou de roadmap_dir depois) não recebe uma segunda linha para o mesmo arquivo.
// ML-3C: o .gitignore agora gerencia duas regras independentes. Um projeto que já
// tem a regra do branch-links recebe apenas a do leases (se ausente).
func TestGenerateGitIgnore_NaoDuplicaQuandoOutroCaminhoJaNomeiaOArquivo(t *testing.T) {
	dir := chdirTemp(t)
	path := filepath.Join(dir, ".gitignore")
	existing := "# ignorado à mão pelo projeto\nlegacy/plans/.trackfw-branch-links.json\n"
	if err := os.WriteFile(path, []byte(existing), 0644); err != nil {
		t.Fatalf("preparar arquivo: %v", err)
	}
	if err := generateGitIgnore(); err != nil {
		t.Fatalf("generateGitIgnore: %v", err)
	}
	got, _ := os.ReadFile(path)
	// Branch-links rule already present → not duplicated.
	if count := strings.Count(string(got), ".trackfw-branch-links.json"); count != 1 {
		t.Fatalf("branch-links foi duplicado: %d ocorrências em:\n%q", count, string(got))
	}
	// Leases rule was not present → appended.
	if !strings.Contains(string(got), ".trackfw/leases/") {
		t.Fatalf("regra de leases não foi adicionada:\n%q", string(got))
	}
	// Agent-memory rule was not present → appended.
	if !strings.Contains(string(got), gitIgnoreAgentMemoryEntry) {
		t.Fatalf("regra de agent-memory não foi adicionada:\n%q", string(got))
	}
	// Second run: no-op (all three rules now present).
	if err := generateGitIgnore(); err != nil {
		t.Fatalf("generateGitIgnore (2ª): %v", err)
	}
	again, _ := os.ReadFile(path)
	if string(again) != string(got) {
		t.Fatalf("segunda execução alterou o arquivo:\n%q", string(again))
	}
}

// TestGenerateGitIgnore_LeasesEntryPresent asserts that trackfw init ensures .trackfw/leases/
// is in the .gitignore, preventing accidental commit of machine-specific lease files.
// Asserts: ML-3C AC — git check-ignore -v .trackfw/leases/x.json → rc=0 after scaffold.
// Contra-braço: FAILS on HEAD (leases entry not present in gitIgnoreBlock).
func TestGenerateGitIgnore_LeasesEntryPresent(t *testing.T) {
	dir := chdirTemp(t)
	if err := generateGitIgnore(); err != nil {
		t.Fatalf("generateGitIgnore: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("ler .gitignore: %v", err)
	}
	if !strings.Contains(string(got), ".trackfw/leases/") {
		t.Fatalf(".gitignore não contém a entrada de leases:\n%q", string(got))
	}
	// Second run: idempotent.
	if err := generateGitIgnore(); err != nil {
		t.Fatalf("generateGitIgnore (2ª): %v", err)
	}
	again, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if n := strings.Count(string(again), ".trackfw/leases/"); n != 1 {
		t.Fatalf("esperava 1 entrada de leases, achei %d:\n%q", n, string(again))
	}
}

// TestGenerateGitIgnore_AgentMemoryExactLineDetection asserts that hasGitIgnoreAgentMemoryRule
// detects only the exact pattern */**/.claude/agent-memory/ — not a basename match, not a
// Contains match, and not a commented-out line.
//
// Reconciliation (ML-1B, AC3):
//   - TestAgentMemoryWrongPatternGetsAppended: afirma que a regra de detecção exige igualdade
//     exata, não basename — um padrão mais amplo (**/.claude/agent-memory/) não é reconhecido
//     como presente e o padrão correto é acrescentado.
//   - TestAgentMemoryCommentedPatternGetsAppended: afirma que uma linha comentada
//     (# */**/.claude/agent-memory/) não conta como presente — a regra é acrescentada.
//   - TestAgentMemoryCorrectPatternIsIdempotent: afirma que, com a regra correta já presente,
//     a segunda execução não a duplica.
func TestGenerateGitIgnore_AgentMemoryExactLineDetection(t *testing.T) {
	t.Run("WrongPatternGetsAppended", func(t *testing.T) {
		// Reconciliation: afirma que **/.claude/agent-memory/ (mais amplo, ignora raiz) não
		// é reconhecido como a regra correta — o padrão exato é acrescentado ao arquivo.
		dir := chdirTemp(t)
		wrongPattern := "**/.claude/agent-memory/\n"
		if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(wrongPattern), 0644); err != nil {
			t.Fatal(err)
		}
		if err := generateGitIgnore(); err != nil {
			t.Fatalf("generateGitIgnore: %v", err)
		}
		got, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
		if n := strings.Count(string(got), gitIgnoreAgentMemoryEntry); n != 1 {
			t.Errorf("expected 1 occurrence of correct pattern after append, got %d\n%q", n, string(got))
		}
	})

	t.Run("CommentedPatternGetsAppended", func(t *testing.T) {
		// Reconciliation: afirma que uma linha comentada (# */**/.claude/agent-memory/) não
		// conta como a regra presente — o padrão é acrescentado.
		dir := chdirTemp(t)
		commented := "# " + gitIgnoreAgentMemoryEntry + "\n"
		if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(commented), 0644); err != nil {
			t.Fatal(err)
		}
		if err := generateGitIgnore(); err != nil {
			t.Fatalf("generateGitIgnore: %v", err)
		}
		got, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
		// Count exact non-comment lines matching the pattern (not substring matches in comments).
		exactCount := 0
		for _, line := range strings.Split(string(got), "\n") {
			if strings.TrimSpace(line) == gitIgnoreAgentMemoryEntry {
				exactCount++
			}
		}
		if exactCount != 1 {
			t.Errorf("expected 1 exact-line occurrence of correct pattern after append, got %d\n%q", exactCount, string(got))
		}
	})

	t.Run("CorrectPatternIsIdempotent", func(t *testing.T) {
		// Reconciliation: afirma idempotência quando a regra correta já está presente —
		// a segunda execução não duplica o padrão.
		dir := chdirTemp(t)
		if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(gitIgnoreAgentMemoryEntry+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := generateGitIgnore(); err != nil {
			t.Fatalf("generateGitIgnore: %v", err)
		}
		got, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
		if n := strings.Count(string(got), gitIgnoreAgentMemoryEntry); n != 1 {
			t.Errorf("idempotency: expected 1 occurrence, got %d\n%q", n, string(got))
		}
	})
}

// gitRunInDir executes git with the given args in dir, using an isolated config.
// The gitcfg file path is returned so the caller can pass it to git check-ignore.
func gitRunInDir(t *testing.T, dir string, args ...string) {
	t.Helper()
	gitcfg := filepath.Join(dir, ".test-gitconfig")
	if _, err := os.Stat(gitcfg); os.IsNotExist(err) {
		_ = os.WriteFile(gitcfg, []byte("[user]\n\temail = test@trackfw\n\tname = trackfw test\n[commit]\n\tgpgsign = false\n"), 0600)
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+gitcfg,
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_TERMINAL_PROMPT=0",
		"HOME="+dir,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// TestGenerateGitIgnore_AgentMemoryFourVectorsWithGitCheckIgnore verifies the pattern
// */**/.claude/agent-memory/ with real git check-ignore semantics.
//
// Reconciliation (ML-1B, AC3):
//   - TestVector_SubComum: afirma que sub/.claude/agent-memory/agent-x/note.md é ignorado —
//     o padrão protege subpastas comuns do repositório.
//   - TestVector_SubProfunda: afirma que deep/sub/dir/.claude/agent-memory/x/note.md é
//     ignorado — profundidade arbitrária é coberta pelo **.
//   - TestVector_Aninhada: afirma que .claude/agent-memory/zeus-tf/.claude/agent-memory/nested/note.md
//     é ignorado — o stray aninhado DENTRO da memória legítima da raiz também é coberto.
//   - TestVector_RaizNaoIgnorada: afirma que .claude/agent-memory/zeus-tf/note.md NÃO é ignorado —
//     a memória legítima da raiz permanece visível ao git.
//   - TestIdempotencia: afirma que a segunda execução de generateGitIgnore não duplica a regra.
func TestGenerateGitIgnore_AgentMemoryFourVectorsWithGitCheckIgnore(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := chdirTemp(t)
	gitcfg := filepath.Join(dir, ".test-gitconfig")
	_ = os.WriteFile(gitcfg, []byte("[user]\n\temail = test@trackfw\n\tname = trackfw test\n[commit]\n\tgpgsign = false\n"), 0600)

	gitcmd := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL="+gitcfg,
			"GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_TERMINAL_PROMPT=0",
			"HOME="+dir,
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	gitcmd("init", "-q")

	if err := generateGitIgnore(); err != nil {
		t.Fatalf("generateGitIgnore: %v", err)
	}

	// Create all test paths on disk (git check-ignore needs the files to exist for trailing-slash patterns).
	ignored := []string{
		"sub/.claude/agent-memory/agent-x/note.md",
		"deep/sub/dir/.claude/agent-memory/agent-x/note.md",
		".claude/agent-memory/zeus-tf/.claude/agent-memory/nested/note.md",
	}
	notIgnored := []string{
		".claude/agent-memory/zeus-tf/note.md",
	}
	for _, p := range append(ignored, notIgnored...) {
		if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(p)), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", p, err)
		}
		if err := os.WriteFile(filepath.Join(dir, p), []byte("memory"), 0644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}

	checkIgnore := func(p string, wantIgnored bool) {
		t.Helper()
		cmd := exec.Command("git", "check-ignore", "-q", p)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL="+gitcfg,
			"GIT_CONFIG_SYSTEM=/dev/null",
		)
		err := cmd.Run()
		gotIgnored := err == nil
		if gotIgnored != wantIgnored {
			t.Errorf("git check-ignore %q: ignored=%v, want=%v", p, gotIgnored, wantIgnored)
		}
	}

	t.Run("Vector_SubComum", func(t *testing.T) {
		checkIgnore("sub/.claude/agent-memory/agent-x/note.md", true)
	})
	t.Run("Vector_SubProfunda", func(t *testing.T) {
		checkIgnore("deep/sub/dir/.claude/agent-memory/agent-x/note.md", true)
	})
	t.Run("Vector_Aninhada", func(t *testing.T) {
		checkIgnore(".claude/agent-memory/zeus-tf/.claude/agent-memory/nested/note.md", true)
	})
	t.Run("Vector_RaizNaoIgnorada", func(t *testing.T) {
		checkIgnore(".claude/agent-memory/zeus-tf/note.md", false)
	})

	t.Run("Idempotencia", func(t *testing.T) {
		first, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
		if err := generateGitIgnore(); err != nil {
			t.Fatalf("generateGitIgnore (2ª): %v", err)
		}
		second, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
		if string(first) != string(second) {
			t.Fatalf("segunda execução alterou o arquivo:\n%q", string(second))
		}
		if n := strings.Count(string(second), gitIgnoreAgentMemoryEntry); n != 1 {
			t.Fatalf("padrão duplicado após segunda execução: %d ocorrências", n)
		}
	})
}

// TestGenerateGitIgnore_AgentMemoryNegationRespected asserts that when the consumer's
// .gitignore already contains a ! negation whose pattern ends in .claude/agent-memory,
// generateGitIgnore respects that opt-out: it must not append the rule (which would
// override the negation under gitignore's last-match-wins semantics).
//
// Reconciliation (ML-3A, AC7 / A-1):
//   - TestNegationPresent_FileUnchanged: afirma que com !lib/.claude/agent-memory/ presente,
//     o arquivo não muda e git check-ignore confirma que a memória não está ignorada —
//     contra a situação pre-fix onde o append sobrescrevia a negação.
//   - TestNegationAbsent_RuleEnters: afirma que sem a negação a regra é acrescentada
//     (cobre o braço de controle).
//
// Contra-braço medido: sem o tratamento de negação (gitIgnoreAgentMemoryNegationLine não
// chamado), TestNegationPresent_FileUnchanged reprova — o arquivo muda e git check-ignore
// retorna 0 (arquivo ignorado).
func TestGenerateGitIgnore_AgentMemoryNegationRespected(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	t.Run("NegationPresent_FileUnchanged", func(t *testing.T) {
		// Reconciliation: afirma que a presença de !lib/.claude/agent-memory/ impede o append
		// e que git check-ignore -q retorna exit code 1 (not ignored) para o caminho optado.
		dir := chdirTemp(t)

		// Pre-seed branch-links and leases so generateGitIgnore finds them present and does
		// not append them — otherwise the file changes even without the agent-memory append.
		initial := gitIgnoreBlock("docs/roadmaps") + gitIgnoreLeasesBlock() + "!lib/.claude/agent-memory/\n"
		if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(initial), 0644); err != nil {
			t.Fatal(err)
		}

		gitcfg := filepath.Join(dir, ".test-gitconfig")
		_ = os.WriteFile(gitcfg, []byte("[user]\n\temail = test@trackfw\n\tname = trackfw test\n[commit]\n\tgpgsign = false\n"), 0600)
		gitRun := func(args ...string) {
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(),
				"GIT_CONFIG_GLOBAL="+gitcfg,
				"GIT_CONFIG_SYSTEM=/dev/null",
				"GIT_TERMINAL_PROMPT=0",
				"HOME="+dir,
			)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v\n%s", args, err, out)
			}
		}
		gitRun("init", "-q")

		// Create the test path on disk (git check-ignore needs the file to exist).
		testPath := "lib/.claude/agent-memory/x/a.md"
		if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(testPath)), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, testPath), []byte("memory"), 0644); err != nil {
			t.Fatal(err)
		}

		if err := generateGitIgnore(); err != nil {
			t.Fatalf("generateGitIgnore: %v", err)
		}

		// File must not have changed.
		got, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
		if string(got) != initial {
			t.Fatalf("file changed despite negation:\ngot:  %q\nwant: %q", string(got), initial)
		}

		// git check-ignore must return exit code 1 (not ignored).
		cmd := exec.Command("git", "check-ignore", "-q", testPath)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL="+gitcfg,
			"GIT_CONFIG_SYSTEM=/dev/null",
		)
		err := cmd.Run()
		var exitErr *exec.ExitError
		if err == nil {
			t.Errorf("lib/.claude/agent-memory/x/a.md must NOT be ignored after negation respected, but git check-ignore returned 0")
		} else if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			t.Fatalf("git check-ignore: unexpected error %v (exit %v)", err, exitErr)
		}
	})

	t.Run("NegationAbsent_RuleEnters", func(t *testing.T) {
		// Reconciliation: afirma que sem negação a regra é acrescentada (braço de controle).
		dir := chdirTemp(t)
		// Pre-seed only branch-links and leases — no agent-memory rule, no negation.
		if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(gitIgnoreBlock("docs/roadmaps")+gitIgnoreLeasesBlock()), 0644); err != nil {
			t.Fatal(err)
		}
		if err := generateGitIgnore(); err != nil {
			t.Fatalf("generateGitIgnore: %v", err)
		}
		got, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
		if !strings.Contains(string(got), gitIgnoreAgentMemoryEntry) {
			t.Errorf("expected agent-memory rule to be added when no negation present:\n%q", string(got))
		}
	})

	// RootAnchoredNegation_RuleEnters asserts that root-anchored negations
	// (!.claude/agent-memory/ and !/.claude/agent-memory/) are NOT treated as opt-outs:
	// generateGitIgnore must still append */**/.claude/agent-memory/ because those
	// negations only un-ignore the root-level directory and are compatible with the rule
	// (issue #646, A-1).
	//
	// Reconciliation (ML-3A2, A-1): afirma que negações ancoradas na raiz não bloqueiam
	// o append; git check-ignore confirma que pypi/.claude/agent-memory/z/a.md é ignorado
	// (pela regra acrescentada) e .claude/agent-memory/z/M.md não é (pela negação da raiz).
	for _, negLine := range []string{"!.claude/agent-memory/", "!/.claude/agent-memory/"} {
		negLine := negLine // capture
		t.Run("RootAnchoredNegation_RuleEnters/"+negLine, func(t *testing.T) {
			dir := chdirTemp(t)

			initial := gitIgnoreBlock("docs/roadmaps") + gitIgnoreLeasesBlock() + ".claude/*\n" + negLine + "\n"
			if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(initial), 0644); err != nil {
				t.Fatal(err)
			}

			gitcfg := filepath.Join(dir, ".test-gitconfig")
			_ = os.WriteFile(gitcfg, []byte("[user]\n\temail = test@trackfw\n\tname = trackfw test\n[commit]\n\tgpgsign = false\n"), 0600)
			gitRun := func(args ...string) {
				t.Helper()
				cmd := exec.Command("git", args...)
				cmd.Dir = dir
				cmd.Env = append(os.Environ(),
					"GIT_CONFIG_GLOBAL="+gitcfg,
					"GIT_CONFIG_SYSTEM=/dev/null",
					"GIT_TERMINAL_PROMPT=0",
					"HOME="+dir,
				)
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("git %v: %v\n%s", args, err, out)
				}
			}
			gitRun("init", "-q")

			// Create both test paths on disk (git check-ignore needs the files to exist).
			subPath := "pypi/.claude/agent-memory/z/a.md"
			rootPath := ".claude/agent-memory/z/M.md"
			for _, p := range []string{subPath, rootPath} {
				if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(p)), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, p), []byte("memory"), 0644); err != nil {
					t.Fatal(err)
				}
			}

			if err := generateGitIgnore(); err != nil {
				t.Fatalf("generateGitIgnore: %v", err)
			}

			// The rule must have been appended (root-anchored negation is not an opt-out).
			got, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
			if !strings.Contains(string(got), gitIgnoreAgentMemoryEntry) {
				t.Fatalf("rule not appended despite root-anchored negation %q:\n%q", negLine, string(got))
			}

			checkIgnore := func(path string, wantIgnored bool) {
				t.Helper()
				cmd := exec.Command("git", "check-ignore", "-q", path)
				cmd.Dir = dir
				cmd.Env = append(os.Environ(),
					"GIT_CONFIG_GLOBAL="+gitcfg,
					"GIT_CONFIG_SYSTEM=/dev/null",
				)
				err := cmd.Run()
				var exitErr *exec.ExitError
				ignored := err == nil
				if wantIgnored && !ignored {
					t.Errorf("%s should be ignored (exit 0) but was not", path)
				} else if !wantIgnored && ignored {
					t.Errorf("%s should NOT be ignored but git check-ignore returned 0", path)
				} else if !wantIgnored && !ignored {
					if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
						t.Fatalf("git check-ignore %s: unexpected error %v", path, err)
					}
				}
			}

			// pypi/.claude/agent-memory/z/a.md must be ignored by the appended rule.
			checkIgnore(subPath, true)
			// .claude/agent-memory/z/M.md must NOT be ignored (negated by root negation).
			checkIgnore(rootPath, false)
		})
	}
}
