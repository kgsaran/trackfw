package validator

// ROADMAP-2026-09-17-leniencia-sem-prazo-e-eterna-e-a-severidade-por-regra-vem-do-arquivo-que-o-
// pr-edita, ML-2C (corretivo do ML-2B).
//
// Testes do terceiro interruptor: um PR que só reponta req_dir/roadmap_dir/adr_dirs não pode
// zerar a cobertura de governança. Discriminante: artefatos comprometidos em origin/main que
// deixaram de estar visíveis em qualquer escopo do disco — não esvaziamento do novo diretório.
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
// Use setupRepoWithOriginDirsAndArtifacts when origin/main must have committed governance files.
func setupRepoWithOriginDirs(t *testing.T, originContent, diskContent string) string {
	t.Helper()
	dir := t.TempDir()
	initGitRepo(t, dir, "main")
	initOriginMain(t, dir, originContent)
	writeFile(t, dir, "trackfw.yaml", diskContent)
	return dir
}

// setupRepoWithOriginDirsAndArtifacts creates a test git repo with an origin remote whose
// trackfw.yaml declares originContent AND has additional files committed (artifacts map:
// relative-path → content). Disk trackfw.yaml is written with diskContent.
// Required by ML-2C tests: the git-tree baseline must contain committed artifacts for coverage
// comparison to have anything to detect.
func setupRepoWithOriginDirsAndArtifacts(t *testing.T, originContent, diskContent string, artifacts map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	initGitRepo(t, dir, "main")
	initOriginMainWithArtifacts(t, dir, originContent, artifacts)
	writeFile(t, dir, "trackfw.yaml", diskContent)
	return dir
}

// initOriginMainWithArtifacts creates the same "origin" git repo as initOriginMain but also
// commits extra files into origin/main. artifacts maps relative path → file content.
// Needed by ML-2C: the git-tree baseline uses `git ls-tree` on origin/main, so artifacts must
// be committed there, not just written to disk.
func initOriginMainWithArtifacts(t *testing.T, testDir, trackfwContent string, artifacts map[string]string) {
	t.Helper()

	originDir := t.TempDir()
	runIn := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("initOriginMainWithArtifacts git %v in %s: %s", args, dir, string(out))
		}
	}

	runIn(originDir, "init", "-b", "main")
	runIn(originDir, "config", "user.email", "test@test.com")
	runIn(originDir, "config", "user.name", "test")
	writeFile(t, originDir, "trackfw.yaml", trackfwContent)
	for relPath, content := range artifacts {
		writeFile(t, originDir, relPath, content)
	}
	runIn(originDir, "add", ".")
	runIn(originDir, "commit", "-m", "init")

	runIn(testDir, "remote", "add", "origin", originDir)
	runIn(testDir, "fetch", "--depth=1", "--no-tags", "origin",
		"+refs/heads/main:refs/remotes/origin/main")
}

// ------ AC5: repontar para vazio reprova ------

// TestScopeAnchor_ReqDirRedirect_Reprova prova que repontar req_dir para um diretório existente
// e vazio reprova, nomeando o artefato que deixou de ser visto — AC5 e AC8(c).
// Reconciliação: este teste prova que ML-2C fecha o canal "PR redireciona req_dir para vazio":
// origin/main tem docs/req/REQ-001.md comprometido; disco aponta para redirected-req (vazio) →
// REQ-001.md fica ausente do disco → violação emitida nomeando o artefato perdido.
func TestScopeAnchor_ReqDirRedirect_Reprova(t *testing.T) {
	dir := setupRepoWithOriginDirsAndArtifacts(t,
		"req_dir: docs/req\n",
		"req_dir: redirected-req\n",
		map[string]string{
			"docs/req/REQ-001.md": "---\nstatus: Open\n---\n# REQ-001\n",
		},
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
	if !hasViolation(violations, "REQ-001.md") {
		t.Errorf("expected violation naming the lost artifact REQ-001.md, got: %v", violations)
	}
}

// TestScopeAnchor_RoadmapDirRedirect_Reprova prova que repontar roadmap_dir para um diretório
// existente e vazio reprova, nomeando o artefato que deixou de ser visto — segunda dimensão do AC5.
// Reconciliação: este teste prova que ML-2C detecta perda de cobertura em roadmap_dir: origin/main
// tem docs/roadmaps/wip/RM-001.md comprometido; disco aponta para redirected-roadmaps (vazio) →
// RM-001.md fica ausente do disco → violação emitida.
func TestScopeAnchor_RoadmapDirRedirect_Reprova(t *testing.T) {
	dir := setupRepoWithOriginDirsAndArtifacts(t,
		"roadmap_dir: docs/roadmaps\n",
		"roadmap_dir: redirected-roadmaps\n",
		map[string]string{
			"docs/roadmaps/wip/RM-001.md": "# Roadmap\n",
		},
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
	if !hasViolation(violations, "RM-001.md") {
		t.Errorf("expected violation naming the lost artifact RM-001.md, got: %v", violations)
	}
}

// TestScopeAnchor_AdrDirsRedirect_Reprova prova que repontar adr_dirs para um diretório
// existente e vazio reprova, nomeando o artefato que deixou de ser visto — terceira dimensão do AC5.
// Reconciliação: este teste prova que ML-2C detecta perda de cobertura em adr_dirs: origin/main
// tem docs/adr/ADR-001.md comprometido; disco aponta para redirected-adr (vazio) → ADR-001.md
// fica ausente do disco → violação emitida.
func TestScopeAnchor_AdrDirsRedirect_Reprova(t *testing.T) {
	dir := setupRepoWithOriginDirsAndArtifacts(t,
		"adr_dirs:\n  - docs/adr\n",
		"adr_dirs:\n  - redirected-adr\n",
		map[string]string{
			"docs/adr/ADR-001.md": "# ADR-001\n",
		},
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
	if !hasViolation(violations, "ADR-001.md") {
		t.Errorf("expected violation naming the lost artifact ADR-001.md, got: %v", violations)
	}
}

// ------ AC5: fachada (o ataque que o ML-2B não fechava) ------

// TestScopeAnchor_Fachada_Reprova é o fixture central do ML-2C: repontar req_dir para um
// diretório com UM arquivo de fachada (basename diferente) reprova, nomeando o artefato que
// deixou de ser visto. Este é o cenário exato que o ML-2B deixava passar.
// Reconciliação: este teste afirma que a cobertura por git-tree fecha o canal da fachada:
// origin/main tem REQ-real.md comprometido; disco aponta para fachada-req com FACHADA.md
// (basename diferente) → REQ-real.md ausente da union do disco → violação emitida.
func TestScopeAnchor_Fachada_Reprova(t *testing.T) {
	dir := setupRepoWithOriginDirsAndArtifacts(t,
		"req_dir: docs/req\n",
		"req_dir: fachada-req\n",
		map[string]string{
			"docs/req/REQ-real.md": "---\nstatus: Open\n---\n# REQ real\n",
		},
	)
	// fachada-req exists and has ONE file — different basename from the committed artifact.
	if err := os.MkdirAll(filepath.Join(dir, "fachada-req"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeFile(t, dir, "fachada-req/FACHADA.md", "# arquivo de fachada\n")
	// docs/req still exists on disk with the original file (attacker left it; doesn't matter).
	if err := os.MkdirAll(filepath.Join(dir, "docs", "req"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeFile(t, dir, "docs/req/REQ-real.md", "---\nstatus: Open\n---\n# REQ real\n")

	chdir(t, dir)
	t.Cleanup(config.Reset)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	violations, _, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() error: %v", err)
	}
	if !hasViolation(violations, "scope redirect") {
		t.Errorf("expected scope redirect violation for facade attack, got: %v", violations)
	}
	if !hasViolation(violations, "REQ-real.md") {
		t.Errorf("expected violation naming the lost artifact REQ-real.md, got: %v", violations)
	}
}

// TestScopeAnchor_DeleteAndRedirect_Reprova é o caso "delete-and-redirect": o atacante
// remove docs/req/REQ-real.md do disco E cria fachada-req/FACHADA.md, depois muda req_dir.
// Reconciliação: este teste afirma que a baseline git-tree resiste ao delete-and-redirect:
// mesmo com docs/req vazio no disco, origin/main ainda lista REQ-real.md → violação emitida.
func TestScopeAnchor_DeleteAndRedirect_Reprova(t *testing.T) {
	dir := setupRepoWithOriginDirsAndArtifacts(t,
		"req_dir: docs/req\n",
		"req_dir: fachada-req\n",
		map[string]string{
			"docs/req/REQ-real.md": "---\nstatus: Open\n---\n# REQ real\n",
		},
	)
	// fachada-req exists with facade file; docs/req does NOT exist on disk (files deleted).
	if err := os.MkdirAll(filepath.Join(dir, "fachada-req"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeFile(t, dir, "fachada-req/FACHADA.md", "# arquivo de fachada\n")
	// docs/req is intentionally NOT created on disk — attacker deleted the originals.

	chdir(t, dir)
	t.Cleanup(config.Reset)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	violations, _, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() error: %v", err)
	}
	if !hasViolation(violations, "scope redirect") {
		t.Errorf("expected scope redirect violation for delete-and-redirect attack, got: %v", violations)
	}
	if !hasViolation(violations, "REQ-real.md") {
		t.Errorf("expected violation naming REQ-real.md even when deleted from disk, got: %v", violations)
	}
}

// TestScopeAnchor_BroadRootDefeatsGuard_Reprova verifica que um atacante que define roadmap_dir: .
// (raiz do repositório como escopo) não consegue neutralizar a guarda via expansão do disk-union.
// Sem o filtro de ancestral, WalkDir(".") alcançaria docs/req/REQ-quebrada.md e reportaria o
// basename como "visível" — falso negativo. Com o filtro, "." é excluído do union porque é
// ancestral estrito de docs/req, e a perda é detectada corretamente.
// Reconciliação: este teste afirma que o filtro de ancestral no mdBasenamesOnDisk impede que uma
// raiz ampla (.) faça o disk-union absorver os próprios arquivos da âncora, garantindo que a
// substituição de req_dir por fachada/req + roadmap_dir: . ainda detecta a perda de cobertura.
func TestScopeAnchor_BroadRootDefeatsGuard_Reprova(t *testing.T) {
	// origin/main: req_dir=docs/req with one committed artifact.
	// disk:        req_dir=fachada/req  +  roadmap_dir=.  (entire repo as roadmap scope).
	dir := setupRepoWithOriginDirsAndArtifacts(t,
		"req_dir: docs/req\nroadmap_dir: docs/roadmaps\n",
		"req_dir: fachada/req\nroadmap_dir: .\n",
		map[string]string{
			"docs/req/REQ-quebrada.md": "---\nstatus: Open\n---\n# REQ\n",
		},
	)
	// Create fachada/req with a facade file (different basename).
	if err := os.MkdirAll(filepath.Join(dir, "fachada", "req"), 0o755); err != nil {
		t.Fatalf("MkdirAll fachada/req: %v", err)
	}
	writeFile(t, dir, "fachada/req/FACHADA.md", "# fachada\n")
	// docs/req still exists on disk (attacker left original; without ancestor filter the
	// WalkDir(".") in roadmap_dir would find it and swallow the loss signal).
	if err := os.MkdirAll(filepath.Join(dir, "docs", "req"), 0o755); err != nil {
		t.Fatalf("MkdirAll docs/req: %v", err)
	}
	writeFile(t, dir, "docs/req/REQ-quebrada.md", "---\nstatus: Open\n---\n# REQ\n")

	chdir(t, dir)
	t.Cleanup(config.Reset)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	violations, _, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() error: %v", err)
	}
	if !hasViolation(violations, "scope redirect") {
		t.Errorf("broad roadmap_dir:. with facade req_dir must produce scope redirect violation; got: %v", violations)
	}
	if !hasViolation(violations, "REQ-quebrada.md") {
		t.Errorf("violation must name the lost artifact REQ-quebrada.md; got: %v", violations)
	}
}

// ------ AC5 contra-braço: reestruturação legítima não reprova ------

// TestScopeAnchor_ReestruturaLegitima_NaoReprova é o contra-braço obrigatório do ML-2C:
// mover docs/req/ para requisicoes/ LEVANDO os arquivos junto não perde cobertura e não reprova.
// Reconciliação: este teste afirma que o discriminante de cobertura por basename não penaliza
// reestruturação legítima: REQ-real.md existe em requisicoes/ (novo req_dir) → basename presente
// na union do disco → sem perda → sem violação.
func TestScopeAnchor_ReestruturaLegitima_NaoReprova(t *testing.T) {
	dir := setupRepoWithOriginDirsAndArtifacts(t,
		"req_dir: docs/req\n",
		"req_dir: requisicoes\n",
		map[string]string{
			"docs/req/REQ-real.md": "---\nstatus: Open\n---\n# REQ real\n",
		},
	)
	// New location has the same file (moved). Original docs/req does NOT exist on disk.
	if err := os.MkdirAll(filepath.Join(dir, "requisicoes"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeFile(t, dir, "requisicoes/REQ-real.md", "---\nstatus: Open\n---\n# REQ real\n")

	chdir(t, dir)
	t.Cleanup(config.Reset)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	violations, _, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() error: %v", err)
	}
	if hasViolation(violations, "scope redirect") {
		t.Errorf("legitimate restructuring with files moved must NOT produce scope redirect violation; got: %v", violations)
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

// ------ adr_dirs: [] explicitamente vazio ------

// TestScopeAnchor_AdrDirsVazioExplicito_Reprova afirma que setar adr_dirs para lista vazia
// quando origin/main tinha ADRs comprometidos reprova, nomeando o artefato perdido.
// Reconciliação: este teste fecha a lacuna declarada pelo ML-2B para adr_dirs: [] — sob
// discriminante de cobertura, remover todos os adr_dirs equivale a repontar para vazio total:
// ADR-001.md de origin/main não aparece em nenhum escopo do disco → violação emitida.
func TestScopeAnchor_AdrDirsVazioExplicito_Reprova(t *testing.T) {
	dir := setupRepoWithOriginDirsAndArtifacts(t,
		"adr_dirs:\n  - docs/adr\n",
		"adr_dirs: []\n", // explicitly empty
		map[string]string{
			"docs/adr/ADR-001.md": "# ADR-001\n",
		},
	)
	chdir(t, dir)
	t.Cleanup(config.Reset)
	t.Cleanup(func() { currentOriginMain = originMainAnchor{} })

	violations, _, err := ValidateUnfiltered()
	if err != nil {
		t.Fatalf("ValidateUnfiltered() error: %v", err)
	}
	if !hasViolation(violations, "scope redirect") {
		t.Errorf("expected scope redirect violation for adr_dirs: [], got: %v", violations)
	}
	if !hasViolation(violations, "ADR-001.md") {
		t.Errorf("expected violation naming lost artifact ADR-001.md, got: %v", violations)
	}
}

// ------ AC8(c) falsificação de três braços ------

// TestAC8c_ScopeRedirect_ThreeArmFalsification é a falsificação de três braços do AC8(c):
//
//	Braço c1 — pós-fix com redirect para vazio E artefato comprometido → reprova.
//	Braço c2 — pós-fix com caminhos idênticos ao origin (sem redirect) → não reprova.
//	Braço c3 — comportamento pré-fix (anchor.dirs == nil) → não reprova (lacuna que os MLs fecham).
//
// Reconciliação: a falsificação de três braços afirma que (c1) a perda de cobertura é detectada
// mesmo com arquivo comprometido no git-tree, (c2) manter os caminhos originais não reprova, e
// (c3) o comportamento pré-fix não detectava o redirecionamento — provando que a correção é
// necessária E suficiente.
func TestAC8c_ScopeRedirect_ThreeArmFalsification(t *testing.T) {
	t.Run("c1-redirect-para-vazio-com-artefato-reprova", func(t *testing.T) {
		dir := setupRepoWithOriginDirsAndArtifacts(t,
			"req_dir: docs/req\n",
			"req_dir: redirected-req\n",
			map[string]string{
				"docs/req/REQ-001.md": "---\nstatus: Open\n---\n# REQ-001\n",
			},
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
			t.Fatal("c1: redirect to empty with committed artifact must produce scope redirect violation")
		}
		if !hasViolation(violations, "REQ-001.md") {
			t.Errorf("c1: violation must name the lost artifact REQ-001.md, got: %v", violations)
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
		// scopeRedirectViolations returns nil when dirs == nil — this is the gap ML-2B/ML-2C close.
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
		// Confirm the arm is non-vacuous: post-fix, with dirs populated and committed artifacts,
		// c1 already demonstrates the detection fires.
		_ = preFix
	})
}

// ------ helpers used above ------

// hasViolation is defined in validator_test.go — reused here.
// hasWarning is defined in validator_test.go — reused here.

// TestScopeAnchor_AnchorComZeroArtefatos_NaoReprova prova que quando origin/main tem req_dir
// configurado mas SEM artefatos comprometidos nele, mudar o req_dir para outro diretório não reprova.
// Reconciliação: este teste afirma que ausência de artefatos no git-tree da âncora significa ausência
// de cobertura a perder — sem baseline, não há violação de perda (reenquadra o que era o
// TestScopeAnchor_ReqDirRedirect_NaoReprova_SeNaoDirVazio do ML-2B).
func TestScopeAnchor_AnchorComZeroArtefatos_NaoReprova(t *testing.T) {
	// origin/main declares req_dir: docs/req but commits NO files there (empty trackfw.yaml only).
	dir := setupRepoWithOriginDirs(t,
		"req_dir: docs/req\n",
		"req_dir: new-req\n",
	)
	// new-req exists AND has a .md file — but nothing was committed in docs/req on origin/main.
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
		t.Errorf("anchor with zero committed artifacts must NOT produce scope redirect violation; got: %v", violations)
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
