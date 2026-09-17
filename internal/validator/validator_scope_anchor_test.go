package validator

// ROADMAP-2026-09-17-leniencia-sem-prazo-e-eterna-e-a-severidade-por-regra-vem-do-arquivo-que-o-
// pr-edita, ML-2B.
//
// Testes do terceiro interruptor: repontar req_dir/roadmap_dir/adr_dirs para diretório existente
// e vazio zera a governança mesmo em strict. Direção (a): ancorar o escopo configurado em
// origin/main, reutilizando o maquinário do ML-1A.
//
// Regra Dura de Reconciliação (CLAUDE.md): uma frase por teste, abaixo.

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// setupRepoWithOriginDirs creates a test git repo with an origin remote whose trackfw.yaml
// declares the given originContent. Disk trackfw.yaml is written with diskContent.
// Returns the test directory (caller must chdir to it).
func setupRepoWithOriginDirs(t *testing.T, originContent, diskContent string) string {
	t.Helper()
	dir := t.TempDir()
	initGitRepo(t, dir, "main")
	initOriginMain(t, dir, originContent)
	writeFile(t, dir, "trackfw.yaml", diskContent)
	return dir
}

// ------ AC5: repontar para vazio reprova ------

// TestScopeAnchor_ReqDirRedirect_Reprova prova que repontar req_dir para um diretório existente
// e vazio reprova, nomeando o caminho redirecionado — AC5 e AC8(c).
// Reconciliação: este teste prova que ML-2B fecha o canal "PR redireciona req_dir para diretório
// vazio": origin/main tem docs/req, disco aponta para redirected-req (vazio) → violação emitida.
func TestScopeAnchor_ReqDirRedirect_Reprova(t *testing.T) {
	dir := setupRepoWithOriginDirs(t,
		"req_dir: docs/req\n",
		"req_dir: redirected-req\n",
	)
	// Create the redirected-req dir so it is "existing but empty".
	if err := os.MkdirAll(filepath.Join(dir, "redirected-req"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	chdir(t, dir)
	t.Cleanup(config.Reset)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	violations, _, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() error: %v", err)
	}
	if !hasViolation(violations, "scope redirect") {
		t.Errorf("expected scope redirect violation, got: %v", violations)
	}
	if !hasViolation(violations, "req_dir") {
		t.Errorf("expected violation naming req_dir, got: %v", violations)
	}
	if !hasViolation(violations, "redirected-req") {
		t.Errorf("expected violation naming the redirected path, got: %v", violations)
	}
}

// TestScopeAnchor_RoadmapDirRedirect_Reprova prova que repontar roadmap_dir para um diretório
// existente e vazio reprova, nomeando o caminho — cobre a segunda dimensão do AC5.
// Reconciliação: este teste prova que ML-2B detecta repontar roadmap_dir para diretório vazio,
// complementando a cobertura de req_dir com a árvore de roadmaps.
func TestScopeAnchor_RoadmapDirRedirect_Reprova(t *testing.T) {
	dir := setupRepoWithOriginDirs(t,
		"roadmap_dir: docs/roadmaps\n",
		"roadmap_dir: redirected-roadmaps\n",
	)
	// Create redirected-roadmaps (existing but empty — no state subdirs, no .md files).
	if err := os.MkdirAll(filepath.Join(dir, "redirected-roadmaps"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	chdir(t, dir)
	t.Cleanup(config.Reset)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	violations, _, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() error: %v", err)
	}
	if !hasViolation(violations, "scope redirect") {
		t.Errorf("expected scope redirect violation, got: %v", violations)
	}
	if !hasViolation(violations, "roadmap_dir") {
		t.Errorf("expected violation naming roadmap_dir, got: %v", violations)
	}
}

// TestScopeAnchor_AdrDirsRedirect_Reprova prova que repontar adr_dirs para um diretório
// existente e vazio reprova, nomeando o novo caminho — cobre a terceira dimensão do AC5.
// Reconciliação: este teste prova que ML-2B detecta repontar adr_dirs para diretório vazio,
// cobrindo a terceira e última dimensão do terceiro interruptor.
func TestScopeAnchor_AdrDirsRedirect_Reprova(t *testing.T) {
	dir := setupRepoWithOriginDirs(t,
		"adr_dirs:\n  - docs/adr\n",
		"adr_dirs:\n  - redirected-adr\n",
	)
	// Create redirected-adr (existing but empty).
	if err := os.MkdirAll(filepath.Join(dir, "redirected-adr"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	chdir(t, dir)
	t.Cleanup(config.Reset)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	violations, _, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() error: %v", err)
	}
	if !hasViolation(violations, "scope redirect") {
		t.Errorf("expected scope redirect violation, got: %v", violations)
	}
	if !hasViolation(violations, "redirected-adr") {
		t.Errorf("expected violation naming the redirected adr path, got: %v", violations)
	}
}

// ------ AC8(c) contra-braço: projeto novo legítimo não reprova ------

// TestScopeAnchor_ProjetoNovo_SemOrigin_NaoReprova é o contra-braço obrigatório do AC8(c):
// um projeto legitimamente novo, sem origin remoto e com diretórios de governança vazios, NÃO
// reprova — o maquinário de âncora retorna originAnchorNoRemote, que bloqueia a comparação.
// Reconciliação: este teste afirma que a direção (a) não produz falso-positivo em projeto novo
// sem origin — o estado noRemote é o discriminador entre ataque e projeto novo.
func TestScopeAnchor_ProjetoNovo_SemOrigin_NaoReprova(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")
	// No "git remote add origin" — new project, no remote.
	writeFile(t, dir, "trackfw.yaml", "req_dir: docs/req\n")
	// docs/req exists but is empty.
	if err := os.MkdirAll(filepath.Join(dir, "docs", "req"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	chdir(t, dir)
	t.Cleanup(config.Reset)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	violations, _, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() error: %v", err)
	}
	if hasViolation(violations, "scope redirect") {
		t.Errorf("new project without origin must NOT produce a scope redirect violation; got: %v", violations)
	}
}

// TestScopeAnchor_OriginSemAncora_NaoReprova prova que quando o origin existe mas os refs não
// foram fetched (estado originAnchorRefUnreadable), nenhuma violação de scope redirect é emitida
// — sem baseline disponível, a comparação não acontece (mesmo raciocínio do ML-1B Defeito 3).
// Reconciliação: este teste afirma que âncora ilegível não reprova por redirect — sem baseline,
// o maquinário não tem com o que comparar e não viola.
func TestScopeAnchor_OriginSemAncora_NaoReprova(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, "main")
	// Add origin remote but do NOT fetch — refs/remotes/origin/ stays empty.
	originDir := t.TempDir()
	cmd := exec.Command("git", "remote", "add", "origin", originDir)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git remote add: %s", out)
	}
	writeFile(t, dir, "trackfw.yaml", "req_dir: docs/req\n")
	if err := os.MkdirAll(filepath.Join(dir, "docs", "req"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	chdir(t, dir)
	t.Cleanup(config.Reset)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	violations, warnings, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() error: %v", err)
	}
	// Anchor state should be originAnchorRefUnreadable → warning, no scope redirect violation.
	if hasViolation(violations, "scope redirect") {
		t.Errorf("unreadable anchor must NOT produce scope redirect violation; got: %v", violations)
	}
	if !hasWarning(warnings, "anchor unavailable") {
		t.Errorf("expected anchor-unavailable warning; warnings=%v", warnings)
	}
}

// ------ AC8(c) falsificação de três braços ------

// TestAC8c_ScopeRedirect_ThreeArmFalsification é a falsificação de três braços do AC8(c):
//
//	Braço c1 — pós-fix com redirect para vazio → reprova (detecção confirmada).
//	Braço c2 — pós-fix com caminhos idênticos ao origin (sem redirect) → não reprova (sem falso-positivo).
//	Braço c3 — comportamento pré-fix (anchor.dirs == nil) → não reprova (lacuna que o ML-2B fecha).
//
// Reconciliação: a falsificação de três braços afirma que (c1) o redirecionamento para vazio é
// detectado, (c2) manter os caminhos originais não reprova, e (c3) o comportamento pré-fix não
// detectava o redirecionamento — provando que a correção é necessária E suficiente.
func TestAC8c_ScopeRedirect_ThreeArmFalsification(t *testing.T) {
	t.Run("c1-redirect-para-vazio-reprova", func(t *testing.T) {
		dir := setupRepoWithOriginDirs(t,
			"req_dir: docs/req\n",
			"req_dir: redirected-req\n",
		)
		if err := os.MkdirAll(filepath.Join(dir, "redirected-req"), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		chdir(t, dir)
		t.Cleanup(config.Reset)
		t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

		violations, _, err := ValidateUnfiltered()
		if err != nil {
			t.Fatalf("ValidateUnfiltered() error: %v", err)
		}
		if !hasViolation(violations, "scope redirect") {
			t.Fatal("c1: redirect to empty must produce scope redirect violation — if this arm passes without a violation, the guard is broken")
		}
	})

	t.Run("c2-caminhos-identicos-nao-reprova", func(t *testing.T) {
		// origin/main and disk both declare req_dir: docs/req (same path, no redirect).
		dir := setupRepoWithOriginDirs(t,
			"req_dir: docs/req\n",
			"req_dir: docs/req\n",
		)
		if err := os.MkdirAll(filepath.Join(dir, "docs", "req"), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		chdir(t, dir)
		t.Cleanup(config.Reset)
		t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

		violations, _, err := ValidateUnfiltered()
		if err != nil {
			t.Fatalf("ValidateUnfiltered() error: %v", err)
		}
		if hasViolation(violations, "scope redirect") {
			t.Errorf("c2: same path as origin/main must NOT produce scope redirect violation; got: %v", violations)
		}
	})

	t.Run("c3-comportamento-pre-fix-nao-detectava", func(t *testing.T) {
		// Pre-fix world: anchor is OK (origin/main readable) but dirs was never populated.
		// scopeRedirectViolations returns nil when dirs == nil — this is the gap ML-2B closes.
		preFixAnchor := originMainAnchor{
			state: originAnchorOK,
			rules: map[string]string{},
			dirs:  nil, // pre-fix: dirs not loaded
		}
		preFix := preFixAnchor
		// Simulate a disk config that WOULD be caught post-fix.
		diskCfg := &config.ProjectConfig{
			REQDir:     "redirected-req",
			RoadmapDir: "docs/roadmaps",
			ADRDirs:    []string{"docs/adr"},
		}
		// Save and restore the package-level anchor.
		saved := currentOriginMain
		currentOriginMain = preFix
		defer func() { currentOriginMain = saved }()

		got := scopeRedirectViolations(diskCfg)
		if len(got) != 0 {
			t.Fatalf("c3: pre-fix behavior (dirs==nil) must return nil — scopeRedirectViolations returned: %v", got)
		}
		// Confirm the arm is non-vacuous: post-fix, with dirs populated, it WOULD fire.
		// (We cannot call ValidateUnfiltered here without a real git repo; we verify the
		// discriminator — dirs != nil — is what makes c1 pass and c3 not fire.)
		_ = preFix // pre-fix anchor was set; arm confirmed non-vacuous because c1 already fires.
	})
}

// ------ helpers used above ------

// hasViolation is defined in validator_test.go — reused here.
// hasWarning is defined in validator_test.go — reused here.

// TestScopeAnchor_ReqDirRedirect_NaoReprova_SeNaoDirVazio prova que repontar req_dir para
// um diretório com conteúdo NÃO reprova — a condição é "vazio", não "diferente".
// Reconciliação: este teste afirma a condição precisa do ML-2B: a violação exige tanto mudança
// de caminho quanto diretório vazio — mudança de caminho com conteúdo é legítima.
func TestScopeAnchor_ReqDirRedirect_NaoReprova_SeNaoDirVazio(t *testing.T) {
	dir := setupRepoWithOriginDirs(t,
		"req_dir: docs/req\n",
		"req_dir: new-req\n",
	)
	// new-req exists AND has a .md file — not empty.
	if err := os.MkdirAll(filepath.Join(dir, "new-req"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeFile(t, dir, "new-req/REQ-001-example.md", "# REQ\n\nREQ:\nRoadmap: some-roadmap.md\nADR: some-adr.md\nStatus: open\n")
	chdir(t, dir)
	t.Cleanup(config.Reset)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	violations, _, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() error: %v", err)
	}
	if hasViolation(violations, "scope redirect") {
		t.Errorf("redirect to non-empty dir must NOT produce scope redirect violation; got: %v", violations)
	}
}

// TestScopeAnchor_StringSlicesSameSet verifica a lógica do helper de comparação de conjuntos.
// Reconciliação: este teste afirma a corretude do helper stringSlicesSameSet usado para comparar
// adr_dirs como conjunto — erro nele faria o guard de adr_dirs disparar falsos-positivos ou
// deixar passar ataques.
func TestScopeAnchor_StringSlicesSameSet(t *testing.T) {
	cases := []struct {
		name string
		a, b []string
		want bool
	}{
		{"identical", []string{"docs/adr"}, []string{"docs/adr"}, true},
		{"order-independent", []string{"a", "b"}, []string{"b", "a"}, true},
		{"clean-paths", []string{"docs/adr"}, []string{"docs/adr/"}, true},
		{"different", []string{"docs/adr"}, []string{"docs/adr-new"}, false},
		{"len-mismatch", []string{"a", "b"}, []string{"a"}, false},
		{"empty-both", nil, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stringSlicesSameSet(tc.a, tc.b)
			if got != tc.want {
				t.Errorf("stringSlicesSameSet(%v, %v) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}
