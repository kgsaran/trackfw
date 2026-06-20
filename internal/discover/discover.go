package discover

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// InstallGates instala os artefatos de governança num projeto brownfield:
// validate script, hook entry e (se github-actions) CI workflow.
func InstallGates(r DiscoveryResult, rootDir string, w io.Writer) error {
	if err := writeValidateScript(rootDir); err != nil {
		return err
	}
	if err := installHook(r.HookFramework, rootDir, w); err != nil {
		return err
	}
	if r.CISystem == "github-actions" {
		if err := writeCIWorkflow(rootDir); err != nil {
			return err
		}
	}
	return nil
}

func writeValidateScript(rootDir string) error {
	scriptsDir := filepath.Join(rootDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return fmt.Errorf("creating scripts dir: %w", err)
	}
	content := "#!/usr/bin/env bash\nset -euo pipefail\ntrackfw validate\n"
	dest := filepath.Join(scriptsDir, "trackfw-validate.sh")
	if err := os.WriteFile(dest, []byte(content), 0755); err != nil {
		return fmt.Errorf("writing validate script: %w", err)
	}
	return nil
}

func installHook(framework, rootDir string, w io.Writer) error {
	hookEntry := "\npre-commit:\n  commands:\n    trackfw-validate:\n      run: scripts/trackfw-validate.sh\n"
	huskyEntry := "\nscripts/trackfw-validate.sh\n"

	switch framework {
	case "lefthook":
		cfgPath := filepath.Join(rootDir, "lefthook.yml")
		if !fileExists(cfgPath) {
			cfgPath = filepath.Join(rootDir, ".lefthook.yml")
		}
		content, err := os.ReadFile(cfgPath)
		if err != nil {
			return fmt.Errorf("reading lefthook config: %w", err)
		}
		if strings.Contains(string(content), "trackfw") {
			// já configurado — idempotente
			return nil
		}
		f, err := os.OpenFile(cfgPath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("opening lefthook config: %w", err)
		}
		defer f.Close()
		_, err = f.WriteString(hookEntry)
		return err

	case "husky":
		huskyHook := filepath.Join(rootDir, ".husky", "pre-commit")
		if err := os.MkdirAll(filepath.Dir(huskyHook), 0755); err != nil {
			return fmt.Errorf("creating .husky dir: %w", err)
		}
		f, err := os.OpenFile(huskyHook, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0755)
		if err != nil {
			return fmt.Errorf("opening husky pre-commit: %w", err)
		}
		defer f.Close()
		_, err = f.WriteString(huskyEntry)
		return err

	default:
		pkgJSON := filepath.Join(rootDir, "package.json")
		if fileExists(pkgJSON) {
			return installHusky(rootDir, w)
		}
		// Node.js disponível mas sem package.json → husky via npx (funciona em Windows sem lefthook)
		if _, nodeErr := exec.LookPath("node"); nodeErr == nil {
			fmt.Fprintf(w, "ℹ node detected — using husky via npx (no package.json required)\n")
			return installHuskyNPX(rootDir, w)
		}
		return installLefthook(rootDir, w)
	}
}

// installLefthook cria lefthook.yml na raiz e tenta executar "lefthook install".
// Se lefthook não estiver no PATH, imprime instrução e retorna nil (não bloqueante).
func installLefthook(rootDir string, w io.Writer) error {
	const lefthookContent = "pre-commit:\n  commands:\n    trackfw-validate:\n      run: scripts/trackfw-validate.sh\n"

	cfgPath := filepath.Join(rootDir, "lefthook.yml")

	if fileExists(cfgPath) {
		content, err := os.ReadFile(cfgPath)
		if err != nil {
			return fmt.Errorf("reading lefthook.yml: %w", err)
		}
		if strings.Contains(string(content), "trackfw") {
			// já configurado — idempotente
			fmt.Fprintf(w, "✓ lefthook.yml already contains trackfw entry\n")
			return nil
		}
		// appenda entrada ao arquivo existente
		f, err := os.OpenFile(cfgPath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("opening lefthook.yml: %w", err)
		}
		defer f.Close()
		if _, err := f.WriteString("\npre-commit:\n  commands:\n    trackfw-validate:\n      run: scripts/trackfw-validate.sh\n"); err != nil {
			return fmt.Errorf("appending to lefthook.yml: %w", err)
		}
		fmt.Fprintf(w, "✓ trackfw entry appended to lefthook.yml\n")
	} else {
		if err := os.WriteFile(cfgPath, []byte(lefthookContent), 0644); err != nil {
			return fmt.Errorf("writing lefthook.yml: %w", err)
		}
		fmt.Fprintf(w, "✓ lefthook.yml created\n")
	}

	// tenta executar "lefthook install" se disponível
	if _, err := exec.LookPath("lefthook"); err == nil {
		cmd := exec.Command("lefthook", "install")
		cmd.Dir = rootDir
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Fprintf(w, "⚠ lefthook install failed: %s\n", strings.TrimSpace(string(out)))
		} else {
			fmt.Fprintf(w, "✓ lefthook install ran successfully\n")
		}
	} else {
		fmt.Fprintf(w, "⚠ lefthook not found in PATH — run 'lefthook install' after installing it\n")
	}

	return nil
}

// installHusky executa npm install --save-dev husky, npx husky init e cria .husky/pre-commit.
// Erros de exec são impressos como aviso (não bloqueantes).
func installHusky(rootDir string, w io.Writer) error {
	// npm install --save-dev husky
	npmInstall := exec.Command("npm", "install", "--save-dev", "husky")
	npmInstall.Dir = rootDir
	if out, err := npmInstall.CombinedOutput(); err != nil {
		fmt.Fprintf(w, "⚠ npm install husky failed: %s\n", strings.TrimSpace(string(out)))
	} else {
		fmt.Fprintf(w, "✓ husky installed via npm\n")
	}

	// npx husky init
	huskyInit := exec.Command("npx", "husky", "init")
	huskyInit.Dir = rootDir
	if out, err := huskyInit.CombinedOutput(); err != nil {
		fmt.Fprintf(w, "⚠ npx husky init failed: %s\n", strings.TrimSpace(string(out)))
	} else {
		fmt.Fprintf(w, "✓ husky initialized\n")
	}

	// cria/append .husky/pre-commit com linha do trackfw
	huskyHook := filepath.Join(rootDir, ".husky", "pre-commit")
	if err := os.MkdirAll(filepath.Dir(huskyHook), 0755); err != nil {
		return fmt.Errorf("creating .husky dir: %w", err)
	}
	f, err := os.OpenFile(huskyHook, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return fmt.Errorf("opening .husky/pre-commit: %w", err)
	}
	defer f.Close()
	if _, err := f.WriteString("\nscripts/trackfw-validate.sh\n"); err != nil {
		return fmt.Errorf("writing .husky/pre-commit: %w", err)
	}
	fmt.Fprintf(w, "✓ trackfw entry added to .husky/pre-commit\n")

	return nil
}

// installHuskyNPX configura husky via npx em projetos sem package.json.
// Adequado para projetos Go/Java/Python em ambientes Windows com Node.js disponível mas sem lefthook.
// Erros de exec são impressos como aviso (não bloqueantes).
func installHuskyNPX(rootDir string, w io.Writer) error {
	// npx husky init — cria .husky/ e instala o handler de hooks
	huskyInit := exec.Command("npx", "husky", "init")
	huskyInit.Dir = rootDir
	if out, err := huskyInit.CombinedOutput(); err != nil {
		fmt.Fprintf(w, "⚠ npx husky init failed: %s\n", strings.TrimSpace(string(out)))
		fmt.Fprintf(w, "  → install husky manually: npx husky init\n")
	} else {
		fmt.Fprintf(w, "✓ husky initialized via npx\n")
	}

	// cria/append .husky/pre-commit com linha do trackfw
	huskyHook := filepath.Join(rootDir, ".husky", "pre-commit")
	if err := os.MkdirAll(filepath.Dir(huskyHook), 0755); err != nil {
		return fmt.Errorf("creating .husky dir: %w", err)
	}
	f, err := os.OpenFile(huskyHook, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return fmt.Errorf("opening .husky/pre-commit: %w", err)
	}
	defer f.Close()
	if _, err := f.WriteString("\nscripts/trackfw-validate.sh\n"); err != nil {
		return fmt.Errorf("writing .husky/pre-commit: %w", err)
	}
	fmt.Fprintf(w, "✓ trackfw entry added to .husky/pre-commit\n")
	return nil
}

func writeCIWorkflow(rootDir string) error {
	workflowsDir := filepath.Join(rootDir, ".github", "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		return fmt.Errorf("creating workflows dir: %w", err)
	}
	dest := filepath.Join(workflowsDir, "trackfw-validate.yml")
	if fileExists(dest) {
		// idempotente — não sobrescreve
		return nil
	}
	content := `name: trackfw validate
on: [push, pull_request]
jobs:
  governance:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"
      - run: go install github.com/kgsaran/trackfw/cmd/trackfw@latest
      - run: trackfw validate
`
	if err := os.WriteFile(dest, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing CI workflow: %w", err)
	}
	return nil
}

// DiscoveryResult contém a estrutura de governança detectada em um repositório.
type DiscoveryResult struct {
	ADRDirs            []string
	REQDir             string
	RoadmapDir         string
	RoadmapNamespacing string   // "flat" ou "by_agent"
	Agents             []string // detectados das subdirs
	ADRCount           int
	REQCount           int
	RoadmapCount       int
	HasTrackfwYAML     bool
	HasTrackfwLog      bool
	GovernanceScore    int    // 0-100
	HookFramework      string // "lefthook", "husky", "pre-commit", "none"
	CISystem           string // "github-actions", "gitlab", "none"
}

// Scan escaneia rootDir e retorna a estrutura de governança detectada.
func Scan(rootDir string) (DiscoveryResult, error) {
	var r DiscoveryResult

	// 1. trackfw.yaml e .trackfw-log
	r.HasTrackfwYAML = fileExists(filepath.Join(rootDir, "trackfw.yaml"))

	// 2. REQ dir — testa candidatos em ordem de preferência
	for _, candidate := range []string{"docs/req", "docs/requisições", "docs/requirements", "docs/reqs"} {
		full := filepath.Join(rootDir, candidate)
		if dirExists(full) {
			r.REQDir = candidate
			r.REQCount = countMDFiles(full)
			break
		}
	}

	// 3. ADR dirs — procura docs/adr recursivamente
	adrRoot := filepath.Join(rootDir, "docs", "adr")
	if dirExists(adrRoot) {
		subDirs, _ := listSubDirs(adrRoot)
		if len(subDirs) > 0 {
			// tem subdirs: usa cada uma como um adr dir
			for _, sub := range subDirs {
				rel := "docs/adr/" + sub
				r.ADRDirs = append(r.ADRDirs, rel)
				r.ADRCount += countMDFiles(filepath.Join(rootDir, rel))
			}
		} else {
			// plano: docs/adr diretamente
			r.ADRDirs = []string{"docs/adr"}
			r.ADRCount = countMDFiles(adrRoot)
		}
	}

	// 4. Roadmap dir e namespacing
	roadmapRoot := filepath.Join(rootDir, "docs", "roadmaps")
	if dirExists(roadmapRoot) {
		r.RoadmapDir = "docs/roadmaps"

		// detecta by_agent: existe docs/roadmaps/*/wip/ ?
		agentDirs, _ := listSubDirs(roadmapRoot)
		byAgent := false
		for _, sub := range agentDirs {
			// se a subdir contém pelo menos um estado válido → by_agent
			wipDir := filepath.Join(roadmapRoot, sub, "wip")
			backlogDir := filepath.Join(roadmapRoot, sub, "backlog")
			doneDir := filepath.Join(roadmapRoot, sub, "done")
			abandonedDir := filepath.Join(roadmapRoot, sub, "abandoned")
			blockedDir := filepath.Join(roadmapRoot, sub, "blocked")
			if dirExists(wipDir) || dirExists(backlogDir) || dirExists(doneDir) || dirExists(abandonedDir) || dirExists(blockedDir) {
				byAgent = true
				r.Agents = append(r.Agents, sub)
			}
		}

		if byAgent {
			r.RoadmapNamespacing = "by_agent"
			for _, agent := range r.Agents {
				for _, state := range []string{"backlog", "wip", "blocked", "done", "abandoned"} {
					dir := filepath.Join(roadmapRoot, agent, state)
					r.RoadmapCount += countMDFiles(dir)
				}
			}
		} else {
			r.RoadmapNamespacing = "flat"
			for _, state := range []string{"backlog", "wip", "blocked", "done", "abandoned"} {
				dir := filepath.Join(roadmapRoot, state)
				r.RoadmapCount += countMDFiles(dir)
			}
		}

		// detectar .trackfw-log dentro de roadmapDir
		r.HasTrackfwLog = fileExists(filepath.Join(roadmapRoot, ".trackfw-log"))
	}

	// 5. Hook framework
	switch {
	case fileExists(filepath.Join(rootDir, "lefthook.yml")) || fileExists(filepath.Join(rootDir, ".lefthook.yml")):
		r.HookFramework = "lefthook"
	case dirExists(filepath.Join(rootDir, ".husky")):
		r.HookFramework = "husky"
	case fileExists(filepath.Join(rootDir, ".pre-commit-config.yaml")):
		r.HookFramework = "pre-commit"
	default:
		r.HookFramework = "none"
	}

	// 6. CI system
	switch {
	case dirExists(filepath.Join(rootDir, ".github", "workflows")):
		r.CISystem = "github-actions"
	case fileExists(filepath.Join(rootDir, ".gitlab-ci.yml")):
		r.CISystem = "gitlab"
	default:
		r.CISystem = "none"
	}

	// 7. Score
	r.GovernanceScore = calcScore(r)

	return r, nil
}

func calcScore(r DiscoveryResult) int {
	score := 0
	if r.ADRCount > 0 {
		score += 20
	}
	if r.REQCount > 0 {
		score += 20
	}
	if r.RoadmapCount > 0 {
		score += 20
	}
	if r.HasTrackfwYAML {
		score += 20
	}
	if r.HasTrackfwLog {
		score += 20
	}
	return score
}

// GenerateYAML gera o conteúdo de um trackfw.yaml calibrado para o DiscoveryResult.
func GenerateYAML(r DiscoveryResult) string {
	var sb strings.Builder
	sb.WriteString("# trackfw configuration — gerado por trackfw discover\n")
	sb.WriteString("# governance_mode: lenient permite validação não-bloqueante durante onboarding\n\n")

	sb.WriteString("governance_mode: lenient\n\n")

	if len(r.ADRDirs) > 0 {
		sb.WriteString("adr_dirs:\n")
		for _, d := range r.ADRDirs {
			sb.WriteString(fmt.Sprintf("  - %s\n", d))
		}
	} else {
		sb.WriteString("adr_dirs:\n  - docs/adr\n")
	}

	if r.REQDir != "" {
		sb.WriteString(fmt.Sprintf("req_dir: %s\n", r.REQDir))
	} else {
		sb.WriteString("req_dir: docs/req\n")
	}

	if r.RoadmapDir != "" {
		sb.WriteString(fmt.Sprintf("roadmap_dir: %s\n", r.RoadmapDir))
	} else {
		sb.WriteString("roadmap_dir: docs/roadmaps\n")
	}

	sb.WriteString(fmt.Sprintf("roadmap_namespacing: %s\n", r.RoadmapNamespacing))

	if len(r.Agents) > 0 {
		sb.WriteString("agents:\n")
		for _, a := range r.Agents {
			sb.WriteString(fmt.Sprintf("  - %s\n", a))
		}
	}

	sb.WriteString(fmt.Sprintf("hooks: %s\n", r.HookFramework))
	sb.WriteString(fmt.Sprintf("ci: %s\n", r.CISystem))

	return sb.String()
}

// GenerateBootstrapLog percorre os arquivos em done/ e gera entradas retroativas no .trackfw-log
// com base no mtime dos arquivos (melhor aproximação disponível).
func GenerateBootstrapLog(r DiscoveryResult, rootDir string) string {
	var sb strings.Builder
	roadmapRoot := filepath.Join(rootDir, r.RoadmapDir)

	appendEntries := func(dir, agent string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			var basename string
			if agent != "" {
				basename = agent + "/" + e.Name()
			} else {
				basename = e.Name()
			}
			sb.WriteString(fmt.Sprintf("%s  %-50s  backlog → done\n",
				info.ModTime().Format("2006-01-02 15:04"),
				basename,
			))
		}
	}

	if r.RoadmapNamespacing == "by_agent" {
		for _, agent := range r.Agents {
			appendEntries(filepath.Join(roadmapRoot, agent, "done"), agent)
		}
	} else {
		appendEntries(filepath.Join(roadmapRoot, "done"), "")
	}

	return sb.String()
}

// helpers

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func countMDFiles(dir string) int {
	n := 0
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
			n++
		}
		return nil
	})
	return n
}

func listSubDirs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	return dirs, nil
}
