package commands

// barrier_contract_test.go — Contrato universal de `trackfw barrier`, congelado em
// docs/cli-parity.md (seção `## trackfw barrier`). Estes testes NÃO implementam produção:
// eles fixam, nos três runtimes, os oito cenários obrigatórios definidos pelo
// ML-1A do roadmap ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.
//
// Mecanismo de pendência: cada teste chama t.Skip(...) como primeira linha. O corpo real
// (fixture + invocação do binário real + asserções do contrato) permanece escrito e
// compilado abaixo do skip — o ML-2A deve REMOVER o t.Skip, não reescrever o teste.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// ────────────────────────────────────────────────────────────────────────────
// Binário real — construído uma única vez por execução do pacote de testes
// ────────────────────────────────────────────────────────────────────────────

var (
	barrierBinaryOnce sync.Once
	barrierBinaryPath string
	barrierBinaryErr  error
)

func barrierFindProjectRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller unavailable")
	}
	dir := filepath.Dir(thisFile)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found: could not determine project root")
		}
		dir = parent
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Nome de executável de teste por SO.
//
// `go build -o <path>` grava o artefato com o nome LITERAL de <path>: medido
// por cross-compile (`GOOS=windows go build -o .../trackfw ./cmd/trackfw`
// produz um PE32+ chamado `trackfw`, sem extensão). No Windows o LookPath de
// os/exec só aceita nomes com extensão do PATHEXT, então esse artefato existe
// e não é executável — o binário nunca roda e o teste falha sem nunca ter
// exercitado o código de produto.
//
// testBinaryNameFor é parametrizado pelo SO justamente para ser falsificável
// nas duas direções sem precisar de uma máquina Windows.
func testBinaryNameFor(goos, base string) string {
	if goos == "windows" {
		return base + ".exe"
	}
	return base
}

// testBinaryName aplica testBinaryNameFor ao SO em que o teste está rodando.
func testBinaryName(base string) string {
	return testBinaryNameFor(runtime.GOOS, base)
}

func TestTestBinaryNameFor_WindowsGetsExeAndPosixIsUnchanged(t *testing.T) {
	cases := []struct {
		goos, base, want string
	}{
		{"windows", "trackfw", "trackfw.exe"},
		{"windows", "git", "git.exe"},
		{"linux", "trackfw", "trackfw"},
		{"darwin", "trackfw", "trackfw"},
		{"freebsd", "trackfw", "trackfw"},
	}
	for _, tc := range cases {
		if got := testBinaryNameFor(tc.goos, tc.base); got != tc.want {
			t.Errorf("testBinaryNameFor(%q, %q) = %q, quer %q", tc.goos, tc.base, got, tc.want)
		}
	}
}

// barrierBinary compila o binário trackfw uma única vez e devolve o caminho.
func barrierBinary(t *testing.T) string {
	t.Helper()
	barrierBinaryOnce.Do(func() {
		projRoot := barrierFindProjectRoot(t)
		dir, err := os.MkdirTemp("", "trackfw-barrier-bin-")
		if err != nil {
			barrierBinaryErr = err
			return
		}
		bin := filepath.Join(dir, testBinaryName("trackfw"))
		cmd := exec.Command("go", "build", "-o", bin, "./cmd/trackfw")
		cmd.Dir = projRoot
		if out, buildErr := cmd.CombinedOutput(); buildErr != nil {
			barrierBinaryErr = fmt.Errorf("go build ./cmd/trackfw failed: %v\n%s", buildErr, out)
			return
		}
		barrierBinaryPath = bin
	})
	if barrierBinaryErr != nil {
		t.Fatalf("could not build trackfw binary: %v", barrierBinaryErr)
	}
	return barrierBinaryPath
}

// ────────────────────────────────────────────────────────────────────────────
// Fixture builder — reproduz exatamente as regras de parsing string-level da
// seção "Roadmap parsing rules" de docs/cli-parity.md.
// ────────────────────────────────────────────────────────────────────────────

// barrierFixtureConfig descreve os eixos que cada cenário varia.
type barrierFixtureConfig struct {
	linkedREQ         bool     // roadmap-level "REQ:" — mantém trackfw validate verde quando true
	mlStatus          string   // ex.: "✅" ou "⬜ Pendente"
	criteriaLines     []string // linhas "- [x] ..." / "- [ ] ..."; ignorado se omitCriteriaBlock
	omitCriteriaBlock bool     // true → ML sem nenhum bloco "**Critérios de aceite:**"
	gateCommands      []string // nil → sem bloco "**Gates da wave:**" (zero gates é legal)
}

// buildBarrierRoadmap monta o texto do roadmap seguindo as seis regras de parsing.
func buildBarrierRoadmap(cfg barrierFixtureConfig) string {
	var b strings.Builder
	b.WriteString("# Roadmap: Barrier Contract Fixture\n\n")
	if cfg.linkedREQ {
		b.WriteString("REQ: REQ-2026-07-29-barrier-fixture\n\n")
	}
	// Bloco de aceite em nível de roadmap — satisfaz wip_acceptance (governança),
	// distinto do bloco por-ML usado pela barrier (rule 4).
	b.WriteString("## Acceptance Criteria\n- [x] fixture roadmap-level criterion\n\n")

	b.WriteString("## Wave 1 — Fixture Wave\n> Dependências: nenhuma\n\n")
	if cfg.gateCommands != nil {
		b.WriteString("**Gates da wave:**\n```bash\n")
		for _, c := range cfg.gateCommands {
			b.WriteString(c + "\n")
		}
		b.WriteString("```\n\n")
	}

	b.WriteString("### ML-1A — Fixture ML\n")
	b.WriteString("**Status:** " + cfg.mlStatus + "\n")
	if !cfg.omitCriteriaBlock {
		b.WriteString("**Critérios de aceite:**\n")
		for _, line := range cfg.criteriaLines {
			b.WriteString(line + "\n")
		}
	}
	b.WriteString("\n")
	return b.String()
}

// setupBarrierFixture escreve a árvore de governança + o roadmap de fixture em um
// diretório temporário e devolve (dir, caminho-absoluto-do-roadmap).
func setupBarrierFixture(t *testing.T, cfg barrierFixtureConfig) (string, string) {
	t.Helper()
	dir := t.TempDir()
	for _, d := range []string{
		"docs/roadmaps/wip", "docs/roadmaps/backlog", "docs/roadmaps/blocked",
		"docs/roadmaps/done", "docs/roadmaps/abandoned", "docs/req", "docs/adr",
	} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatalf("setupBarrierFixture: mkdirs: %v", err)
		}
	}
	roadmapPath := filepath.Join(dir, "docs/roadmaps/wip/ROADMAP-barrier-fixture.md")
	if err := os.WriteFile(roadmapPath, []byte(buildBarrierRoadmap(cfg)), 0644); err != nil {
		t.Fatalf("setupBarrierFixture: write roadmap: %v", err)
	}
	return dir, roadmapPath
}

// ────────────────────────────────────────────────────────────────────────────
// Documento JSON — espelha o contrato de docs/cli-parity.md
// ────────────────────────────────────────────────────────────────────────────

type barrierCheckDoc struct {
	Name     string   `json:"name"`
	Status   string   `json:"status"`
	Evidence []string `json:"evidence"`
	Failures []string `json:"failures"`
	Commands []string `json:"commands,omitempty"`
}

type barrierResultDoc struct {
	Roadmap    string            `json:"roadmap"`
	Wave       string            `json:"wave"` // string since wave label grammar was introduced (ML-2A)
	Status     string            `json:"status"`
	StartedAt  string            `json:"started_at"`
	FinishedAt string            `json:"finished_at"`
	Checks     []barrierCheckDoc `json:"checks"`
	Failures   []string          `json:"failures"`
}

// runBarrierCLI invoca `trackfw barrier <roadmap> --wave <n> --json` a partir de dir
// e devolve stdout, stderr e o exit code do processo.
func runBarrierCLI(t *testing.T, dir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	bin := barrierBinary(t)
	fullArgs := append([]string{"barrier"}, args...)
	cmd := exec.Command(bin, fullArgs...)
	cmd.Dir = dir
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run trackfw barrier: %v", err)
		}
	}
	return outBuf.String(), errBuf.String(), code
}

// ────────────────────────────────────────────────────────────────────────────
// 1 — wave_verde_passa
// ────────────────────────────────────────────────────────────────────────────

func TestBarrierContract_WaveVerdePassa(t *testing.T) {

	dir, _ := setupBarrierFixture(t, barrierFixtureConfig{
		linkedREQ:     true,
		mlStatus:      "✅",
		criteriaLines: []string{"- [x] build passes", "- [x] tests pass"},
		gateCommands:  nil, // sem bloco de gates — zero gates é legal
	})

	stdout, stderr, code := runBarrierCLI(t, dir, "ROADMAP-barrier-fixture", "--wave", "1", "--json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	var doc barrierResultDoc
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &doc); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}
	if doc.Status != "passed" {
		t.Fatalf("expected status=passed, got %q", doc.Status)
	}

	var gatesCheck *barrierCheckDoc
	for i := range doc.Checks {
		if doc.Checks[i].Name == "gates" {
			gatesCheck = &doc.Checks[i]
		}
	}
	if gatesCheck == nil {
		t.Fatal("expected a 'gates' check in the result document")
	}
	if gatesCheck.Status != "passed" {
		t.Fatalf("expected gates check to be passed, got %q", gatesCheck.Status)
	}
	if len(gatesCheck.Commands) != 0 {
		t.Fatalf("expected empty commands for a wave with no gates block, got %v", gatesCheck.Commands)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// 2 — ml_pendente_bloqueia
// ────────────────────────────────────────────────────────────────────────────

func TestBarrierContract_MLPendenteBloqueia(t *testing.T) {

	dir, _ := setupBarrierFixture(t, barrierFixtureConfig{
		linkedREQ:     true,
		mlStatus:      "⬜ Pendente",
		criteriaLines: []string{"- [x] build passes"},
	})

	stdout, stderr, code := runBarrierCLI(t, dir, "ROADMAP-barrier-fixture", "--wave", "1", "--json")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	var doc barrierResultDoc
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &doc); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}
	if doc.Status != "blocked" {
		t.Fatalf("expected status=blocked, got %q", doc.Status)
	}
	found := false
	for _, c := range doc.Checks {
		if c.Name == "mls_complete" {
			found = true
			if c.Status != "blocked" {
				t.Fatalf("expected mls_complete check blocked, got %q", c.Status)
			}
		}
	}
	if !found {
		t.Fatal("expected a 'mls_complete' check in the result document")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// 3 — evidencia_ausente_bloqueia
// ────────────────────────────────────────────────────────────────────────────

func TestBarrierContract_EvidenciaAusenteBloqueia(t *testing.T) {

	dir, _ := setupBarrierFixture(t, barrierFixtureConfig{
		linkedREQ:     true,
		mlStatus:      "✅",
		criteriaLines: []string{"- [x] build passes", "- [ ] tests pass"}, // ao menos um não marcado
	})

	stdout, stderr, code := runBarrierCLI(t, dir, "ROADMAP-barrier-fixture", "--wave", "1", "--json")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	var doc barrierResultDoc
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &doc); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}
	if doc.Status != "blocked" {
		t.Fatalf("expected status=blocked, got %q", doc.Status)
	}
	found := false
	for _, c := range doc.Checks {
		if c.Name == "acceptance_evidence" {
			found = true
			if c.Status != "blocked" {
				t.Fatalf("expected acceptance_evidence check blocked, got %q", c.Status)
			}
		}
	}
	if !found {
		t.Fatal("expected an 'acceptance_evidence' check in the result document")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// 4 — ml_sem_bloco_de_criterios_bloqueia (caso anti-vacuidade)
// ────────────────────────────────────────────────────────────────────────────

func TestBarrierContract_MLSemBlocoDeCriteriosBloqueia(t *testing.T) {

	dir, _ := setupBarrierFixture(t, barrierFixtureConfig{
		linkedREQ:         true,
		mlStatus:          "✅",
		omitCriteriaBlock: true, // nenhum bloco "**Critérios de aceite:**" — não pode passar vacuamente
	})

	stdout, stderr, code := runBarrierCLI(t, dir, "ROADMAP-barrier-fixture", "--wave", "1", "--json")
	if code != 1 {
		t.Fatalf("expected exit 1 (anti-vacuity), got %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	var doc barrierResultDoc
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &doc); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}
	if doc.Status != "blocked" {
		t.Fatalf("expected status=blocked, got %q", doc.Status)
	}
	found := false
	for _, c := range doc.Checks {
		if c.Name == "acceptance_evidence" {
			found = true
			if c.Status != "blocked" {
				t.Fatalf("expected acceptance_evidence check blocked for missing block, got %q", c.Status)
			}
		}
	}
	if !found {
		t.Fatal("expected an 'acceptance_evidence' check in the result document")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// 5 — gate_falho_bloqueia
// ────────────────────────────────────────────────────────────────────────────

func TestBarrierContract_GateFalhoBloqueia(t *testing.T) {

	dir, _ := setupBarrierFixture(t, barrierFixtureConfig{
		linkedREQ:     true,
		mlStatus:      "✅",
		criteriaLines: []string{"- [x] build passes"},
		gateCommands:  []string{"false"},
	})

	stdout, stderr, code := runBarrierCLI(t, dir, "ROADMAP-barrier-fixture", "--wave", "1", "--json")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	var doc barrierResultDoc
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &doc); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}
	if doc.Status != "blocked" {
		t.Fatalf("expected status=blocked, got %q", doc.Status)
	}
	found := false
	for _, c := range doc.Checks {
		if c.Name == "gates" {
			found = true
			if c.Status != "blocked" {
				t.Fatalf("expected gates check blocked, got %q", c.Status)
			}
			if len(c.Commands) == 0 || c.Commands[0] != "false" {
				t.Fatalf("expected commands=[\"false\"] recorded on the gates check, got %v", c.Commands)
			}
		}
	}
	if !found {
		t.Fatal("expected a 'gates' check in the result document")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// 6 — validate_falho_bloqueia
// ────────────────────────────────────────────────────────────────────────────

func TestBarrierContract_ValidateFalhoBloqueia(t *testing.T) {

	// Wave/ML/gates estão inteiramente verdes; a única falha é de governança
	// (roadmap em wip sem REQ vinculada), que só o check "validate" deve capturar.
	dir, _ := setupBarrierFixture(t, barrierFixtureConfig{
		linkedREQ:     false,
		mlStatus:      "✅",
		criteriaLines: []string{"- [x] build passes"},
	})

	stdout, stderr, code := runBarrierCLI(t, dir, "ROADMAP-barrier-fixture", "--wave", "1", "--json")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	var doc barrierResultDoc
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &doc); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}
	if doc.Status != "blocked" {
		t.Fatalf("expected status=blocked, got %q", doc.Status)
	}
	found := false
	for _, c := range doc.Checks {
		if c.Name == "validate" {
			found = true
			if c.Status != "blocked" {
				t.Fatalf("expected validate check blocked, got %q", c.Status)
			}
		}
		// Os demais checks devem permanecer verdes — prova que o fixture isola a falha.
		if c.Name != "validate" && c.Status != "passed" {
			t.Fatalf("expected only 'validate' to be blocked, but %q is %q", c.Name, c.Status)
		}
	}
	if !found {
		t.Fatal("expected a 'validate' check in the result document")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// 7 — roadmap_ou_wave_inexistente_e_erro_de_uso
// ────────────────────────────────────────────────────────────────────────────

func TestBarrierContract_RoadmapOuWaveInexistenteEErroDeUso(t *testing.T) {

	t.Run("wave_inexistente", func(t *testing.T) {
		dir, _ := setupBarrierFixture(t, barrierFixtureConfig{
			linkedREQ:     true,
			mlStatus:      "✅",
			criteriaLines: []string{"- [x] build passes"},
		})
		stdout, stderr, code := runBarrierCLI(t, dir, "ROADMAP-barrier-fixture", "--wave", "99", "--json")
		if code != 2 {
			t.Fatalf("expected exit 2 (usage error) for nonexistent wave, got %d", code)
		}
		if strings.Contains(stdout, `"status": "blocked"`) || strings.Contains(stdout, `"status":"blocked"`) {
			t.Fatalf("a usage error must never emit a status=blocked result document, got: %s", stdout)
		}
		if !strings.Contains(strings.ToLower(stderr), "wave") && !strings.Contains(stderr, "99") {
			t.Fatalf("usage error must explicitly name the unresolved wave, got stderr: %s", stderr)
		}
	})

	t.Run("roadmap_inexistente", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "docs/roadmaps/wip"), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		stdout, stderr, code := runBarrierCLI(t, dir, "ROADMAP-does-not-exist", "--wave", "1", "--json")
		if code != 2 {
			t.Fatalf("expected exit 2 (usage error) for nonexistent roadmap, got %d", code)
		}
		if strings.Contains(stdout, `"status": "blocked"`) || strings.Contains(stdout, `"status":"blocked"`) {
			t.Fatalf("a usage error must never emit a status=blocked result document, got: %s", stdout)
		}
		if !strings.Contains(strings.ToLower(stderr), "roadmap") && !strings.Contains(strings.ToLower(stderr), "does-not-exist") {
			t.Fatalf("usage error must explicitly name the unresolved roadmap, got stderr: %s", stderr)
		}
	})
}

// ────────────────────────────────────────────────────────────────────────────
// 8 — json_deterministico
// ────────────────────────────────────────────────────────────────────────────

func TestBarrierContract_JSONDeterministico(t *testing.T) {

	dir, _ := setupBarrierFixture(t, barrierFixtureConfig{
		linkedREQ:     true,
		mlStatus:      "✅",
		criteriaLines: []string{"- [x] build passes"},
		gateCommands:  []string{"true"},
	})

	wantOrder := []string{"mls_complete", "acceptance_evidence", "gates", "validate"}

	for run := 0; run < 2; run++ {
		stdout, stderr, code := runBarrierCLI(t, dir, "ROADMAP-barrier-fixture", "--wave", "1", "--json")
		if code != 0 {
			t.Fatalf("run %d: expected exit 0, got %d\nstdout: %s\nstderr: %s", run, code, stdout, stderr)
		}
		var doc barrierResultDoc
		if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &doc); err != nil {
			t.Fatalf("run %d: stdout is not valid JSON: %v\nstdout: %s", run, err, stdout)
		}
		if len(doc.Checks) != len(wantOrder) {
			t.Fatalf("run %d: expected %d checks, got %d", run, len(wantOrder), len(doc.Checks))
		}
		for i, name := range wantOrder {
			if doc.Checks[i].Name != name {
				t.Fatalf("run %d: expected checks[%d].name=%q, got %q", run, i, name, doc.Checks[i].Name)
			}
			if doc.Checks[i].Evidence == nil {
				t.Fatalf("run %d: checks[%d].evidence must never be null", run, i)
			}
			if doc.Checks[i].Failures == nil {
				t.Fatalf("run %d: checks[%d].failures must never be null", run, i)
			}
			if name != "gates" && doc.Checks[i].Commands != nil {
				t.Fatalf("run %d: 'commands' must be present only on the gates check, found on %q", run, name)
			}
			if name == "gates" && doc.Checks[i].Commands == nil {
				t.Fatalf("run %d: 'commands' must be present on the gates check", run)
			}
		}
		if doc.Failures == nil {
			t.Fatalf("run %d: top-level failures must never be null", run)
		}
	}
}
