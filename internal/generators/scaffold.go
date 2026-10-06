package generators

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/homedir"
	"github.com/kgsaran/trackfw/internal/pathguard"
	"github.com/kgsaran/trackfw/internal/validator"
	"github.com/kgsaran/trackfw/internal/version"
)

// projectRoot returns the resolved (EvalSymlinks) absolute path of the current
// working directory. This is the root argument to pathguard.RejectSymlinks and
// pathguard.GuardedWrite. On macOS, os.Getwd() may return /tmp/... while
// EvalSymlinks resolves it to /private/tmp/... — passing the unresolved path as
// root causes false "escapes root" errors for files that are genuinely inside the
// project. Falls back to the unresolved cwd if EvalSymlinks fails.
// getwdFn is the single seam through which projectRoot reads the working
// directory. os.Getwd failing is the ONLY error path of projectRoot
// (filepath.EvalSymlinks failing falls back to the unresolved cwd and returns
// nil), so injecting this variable is the portable, deterministic way for a test
// to exercise the "root could not be established" branch of every call site.
//
// 🔴 The seam is at os.Getwd and NOT at projectRoot: internal/pathguard's
// analyser keys provenance on the literal identifier `projectRoot` at the call
// site (approvedResolvers / taintSourceCalls). Turning projectRoot itself into a
// variable would strip resolver provenance from every guard root in this package
// and make P2 report them as unresolved. See vault/notes/
// chdir-removeall-nao-forca-getwd-falhar-no-windows-2026-09-17.md for why a
// filesystem trick (chdir + RemoveAll) is not usable here.
var getwdFn = os.Getwd

func projectRoot() (string, error) {
	cwd, err := getwdFn()
	if err != nil {
		return "", fmt.Errorf("projectRoot: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		return cwd, nil // fallback: EvalSymlinks fails on missing dirs in path
	}
	return resolved, nil
}

type Config struct {
	ProjectType        string // "fullstack" | "frontend" | "backend" | "governance"
	ProjectName        string
	Frontend           string
	Backend            string
	BackendFramework   string
	PkgManager         string
	Hooks              string
	CI                 string
	BrownfieldMode     bool
	LenientUntil       time.Time // zero value = strict
	WipLimit           int       // default: 1
	WipBySquad         bool      // default: false
	RequireReqInCommit bool      // gera hook commit-msg que exige REQ: em feat/* e fix/*
	Forge              string    // forge platform: "github", "gitlab", "bitbucket", "azure", or "" (omit key)
	AgentConventions   string    // agent_conventions: free-text, multi-line, or "" (omit key)
	// RoadmapNamespacing and Agents are used by generators (claudemd, scaffold) to emit the
	// correct `req new` / `roadmap new` command for by_agent projects with 2+ agents (AC13).
	RoadmapNamespacing string   // "flat" (default) | "by_agent"
	Agents             []string // agent namespaces declared in by_agent projects
}

var govDirs = []string{
	"docs/adr",
	"docs/req",
	"docs/roadmaps/backlog",
	"docs/roadmaps/analyzing",
	"docs/roadmaps/wip",
	"docs/roadmaps/blocked",
	"docs/roadmaps/done",
	"docs/roadmaps/abandoned",
	"vault/notes",
}

// scaffoldRoot resolves a rootDir argument for the exported scaffold functions.
// When rootDir is "" or ".", it resolves to projectRoot() (EvalSymlinks-canonical).
// When rootDir is already absolute it is returned as-is after EvalSymlinks.
func scaffoldRoot(rootDir string) (string, error) {
	if rootDir == "" || rootDir == "." {
		return projectRoot()
	}
	// ML-8A / #402: Abs + EvalSymlinks-with-fallback is exactly what
	// pathguard.ResolveRoot does; this was the third hand-written copy of it.
	// Delegating keeps one implementation of "what a guard root must be".
	return pathguard.ResolveRoot(rootDir)
}

// rejectScaffoldPath guards a path inside root before any write or MkdirAll.
// It delegates to pathguard.RejectAndReport, which is the single site in the
// binary that emits the containment refusal (ML-7B).
func rejectScaffoldPath(root, absTarget string) error {
	// Thin adapter over pathguard.RejectAndReport: it must NOT re-wrap the error
	// (the message already names the refused path) and must NOT print — the single
	// emission point lives in pathguard (ML-7B).
	return pathguard.RejectAndReport(root, absTarget)
}

func Scaffold(cfg Config) error {
	root, err := projectRoot()
	if err != nil {
		return fmt.Errorf("Scaffold: %w", err)
	}
	for _, dir := range govDirs {
		absDir := filepath.Join(root, dir)
		if err := rejectScaffoldPath(root, absDir); err != nil {
			return err
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
		fmt.Printf("  ✓ %s\n", dir)
	}

	if err := generateVaultIndex(); err != nil {
		return err
	}

	if err := generateGitAttributes(); err != nil {
		return err
	}

	// ANTES de writeTrackfwConfig de propósito: writeTrackfwConfig sobrescreve
	// trackfw.yaml com `roadmap_dir: docs/roadmaps` sem ler o valor anterior, então
	// chamar depois dele faria todo projeto brownfield com roadmap_dir próprio
	// receber a linha do layout do mantenedor. Aqui ainda se lê o valor do projeto.
	if err := generateGitIgnore(); err != nil {
		return err
	}

	if err := writeTrackfwConfig(cfg); err != nil {
		return err
	}

	if err := generateValidateScript(cfg); err != nil {
		return err
	}

	if err := GenerateAttentionScripts(""); err != nil {
		return err
	}

	if err := GenerateCredentialGuardScript(""); err != nil {
		return err
	}

	if err := GenerateGitBranchGuardScript(""); err != nil {
		return err
	}

	if err := generateCIWorkflow(cfg); err != nil {
		return err
	}

	if err := generateGitHooks(cfg); err != nil {
		return err
	}

	if err := generateCommitMsgHook(cfg); err != nil {
		return err
	}

	if err := generateClaudeMD(cfg); err != nil {
		return err
	}

	if err := generateClaudeCommands(); err != nil {
		return err
	}

	if cfg.Backend == "java" {
		if err := GeneratePomXML(cfg); err != nil {
			return fmt.Errorf("gerando pom.xml: %w", err)
		}
		fmt.Println("  ✓ pom.xml")
	}

	// Agent hooks (attention signal): injected at init time so a freshly
	// scaffolded project already carries them, matching npm's
	// generators/init.js:scaffold (which calls injectHooksDetected(root) as
	// its last step). Non-fatal like the same call in trackfw update
	// (internal/generators/update.go) — a hook-injection failure must not
	// abort project scaffolding. Ported to close the cross-runtime `init`
	// parity gap surfaced while proving `trackfw update` idempotency
	// byte-identical across Go/Node.js/Python (ML-6H, docs/cli-parity.md
	// "`trackfw update` vs `trackfw update harness`").
	if cwd, err := os.Getwd(); err == nil {
		if err := InjectHooksDetected(cwd); err != nil {
			fmt.Printf("  ⚠ agent hooks: %v\n", err)
		}
	}

	return nil
}

// InstallSkills instala os slash commands no projeto atual e a skill global em ~/.claude/skills/trackfw/.
// Arquivos já existentes não são sobrescritos — idempotente.
func InstallSkills() error {
	return installSkillsInner(false)
}

// ForceInstallSkills re-instala os slash commands e a skill global, sobrescrevendo arquivos existentes.
func ForceInstallSkills() error {
	return installSkillsInner(true)
}

func installSkillsInner(force bool) error {
	if err := generateClaudeCommandsInner(force); err != nil {
		return err
	}
	return installGlobalSkillInner(force)
}

func installGlobalSkillInner(force bool) error {
	home, err := homedir.Dir()
	if err != nil {
		return fmt.Errorf("localizando home dir: %w", err)
	}

	// Guard: resolve home and use it as root for the global skill write.
	// ML-8A / #402: one assignment from one resolver. The previous shape (Abs,
	// then a conditional reassignment from EvalSymlinks) was behaviourally the
	// same but bound absHome to the UNRESOLVED Abs(home), which is what the
	// containment analyser's P2 reads.
	absHome, absErr := pathguard.ResolveRoot(home)
	if absErr != nil {
		return fmt.Errorf("resolving home: %w", absErr)
	}
	skillPath := GlobalClaudeSkillPath(home)
	skillDir := filepath.Dir(skillPath)
	// Derive the resolved skill dir from absHome (already EvalSymlinks-resolved) so the
	// containment check compares paths in the same namespace (macOS /var → /private/var).
	absSkillDir := filepath.Join(absHome, ".claude", "skills", "trackfw")
	if err := rejectScaffoldPath(absHome, absSkillDir); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return fmt.Errorf("creating %s: %w", skillDir, err)
	}

	if _, err := os.Stat(skillPath); err == nil && !force {
		fmt.Printf("  ✓ ~/.claude/skills/trackfw/SKILL.md (já existe — não sobrescrito)\n")
		return nil
	}

	// Guard the leaf file — rejectScaffoldPath above covers the directory but not the file
	// itself (RejectSymlinks walks upward from its argument and never descends below it).
	// If SKILL.md already exists as a symlink pointing outside absHome, os.WriteFile would
	// follow it silently. Guarding the file catches that case.
	absSkillPath := filepath.Join(absHome, ".claude", "skills", "trackfw", "SKILL.md")
	if err := rejectScaffoldPath(absHome, absSkillPath); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectScaffoldPath above
	if err := os.WriteFile(skillPath, GlobalClaudeSkillContent(), 0644); err != nil {
		return fmt.Errorf("writing SKILL.md: %w", err)
	}
	fmt.Printf("  ✓ ~/.claude/skills/trackfw/SKILL.md\n")
	return nil
}

// GlobalADRDir resolves the path of the global-scope ADR directory given a
// home directory. Used by `trackfw adr new/list --scope global` to write/read
// cross-project ADRs outside any single project's `trackfw.yaml`/`adr_dirs`.
// Mirrors GlobalClaudeSkillPath below — same style, same rationale for taking
// home as a parameter (testability with a fixture $HOME).
func GlobalADRDir(home string) string {
	return filepath.Join(home, ".trackfw", "adr")
}

// GlobalClaudeSkillPath resolves the path of the historical, global-scope
// Claude compatibility skill given a home directory. It is not part of the
// catalog-managed integrations manifest — a legacy artifact predating that
// mechanism — so its lifecycle (existence/content) is tracked by direct
// inspection rather than through internal/integrations.
func GlobalClaudeSkillPath(home string) string {
	return filepath.Join(home, ".claude", "skills", "trackfw", "SKILL.md")
}

// GlobalClaudeSkillContent returns the current canonical content of the
// historical global Claude compatibility skill.
func GlobalClaudeSkillContent() []byte {
	content := `---
name: trackfw
description: "trackfw — Governed Software Delivery: ADR → REQ → ROADMAP → kanban"
signature: "📦 trackfw - Governed Delivery"
---

# trackfw — Modo de Operação

Você está operando com o **trackfw**, um framework de governança de entrega de software.
A cadeia obrigatória é: **ADR → REQ → ROADMAP → backlog/wip/blocked/done/abandoned**

---

## Regras invioláveis

1. **Nunca inicie uma implementação sem uma REQ e um ROADMAP.** Se não existirem, crie-os primeiro com ` + "`/trackfw:req`" + ` e ` + "`/trackfw:roadmap`" + `.
2. **Use ` + "`/trackfw:implement`" + ` como ponto de entrada para qualquer implementação.** Este skill orquestra o fluxo completo automaticamente.
3. **Apenas um roadmap em ` + "`wip/`" + ` por vez.** Antes de iniciar um novo, conclua ou mova para ` + "`blocked/`" + ` o atual.
4. **Ciclo de vida do ML — obrigatório:**
   - Ao **iniciar** um ML: edite o roadmap alterando ` + "`**Status:** ⬜ Pendente`" + ` → ` + "`**Status:** 🔄 Em andamento`" + ` e faça commit do roadmap.
   - Ao **concluir** um ML: edite o roadmap alterando ` + "`**Status:** 🔄 Em andamento`" + ` → ` + "`**Status:** ✅ Concluído`" + ` e inclua essa mudança no commit do ML.
   - Ao **analisar** um roadmap antes de iniciar: mova o arquivo de ` + "`backlog/`" + ` para ` + "`analyzing/`" + `; só mova para ` + "`wip/`" + ` ao começar a codificar de fato.
5. **Execute ` + "`trackfw validate`" + ` antes de cada commit.** Zero violations obrigatório.
6. **ADRs antes de decisões arquiteturais.** Qualquer decisão técnica relevante deve ter um ADR (` + "`/trackfw:adr`" + `).
7. **` + GlobalADRsDirective + `**

---

## Protocolo de conclusão de cada ML

` + "```" + `
1. Implementar    → executar ações descritas no ML
2. Build          → comando de build do projeto
3. Testes         → comando de testes do projeto
4. Validate       → trackfw validate
5. Commit         → git commit -m "feat(<escopo>): <descrição>"
6. Push           → git push origin <branch>
7. Roadmap        → marcar ML como ✅ Concluído
` + "```" + `
`
	return []byte(content)
}

// ForceGenerateClaudeCommands re-gera todos os slash commands, sobrescrevendo arquivos existentes.
func ForceGenerateClaudeCommands() error {
	return generateClaudeCommandsInner(true)
}

func generateClaudeCommands() error {
	return generateClaudeCommandsInner(false)
}

// ClaudeCommandsDirPath is the canonical relative path of the directory that holds the
// trackfw slash commands written by init/update. Scaffold doctor (ADR-2026-08-27) uses
// this path to detect the surface and to check individual command files.
const ClaudeCommandsDirPath = ".claude/commands/trackfw"

// claudeCommandsContent returns the template content for every slash command that
// trackfw writes to ClaudeCommandsDirPath. Extracted from generateClaudeCommandsInner
// so scaffold doctor can compare disk content against the current template without
// re-writing anything (ADR-2026-08-27, AC3: no manifest writes).
func claudeCommandsContent() map[string]string {
	return map[string]string{
		"adr.md": `Execute o seguinte comando bash: ` + "`trackfw adr new \"$ARGUMENTS\"`" + `

Se o comando falhar com ` + "`trackfw: command not found`" + ` ou similar, informe ao usuário:

` + "```" + `
trackfw não está instalado. Instale com uma das opções:

  curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
  npm install -g trackfw
  pip install trackfw
` + "```",

		"req.md": `Execute o seguinte comando bash: ` + "`trackfw req new \"$ARGUMENTS\"`" + `

⚠️ Em projetos com ` + "`roadmap_namespacing: by_agent`" + ` e 2+ agentes, use: ` + "`trackfw req new --agent <seu-agente> \"$ARGUMENTS\"`" + `

Se o comando falhar com ` + "`trackfw: command not found`" + ` ou similar, informe ao usuário:

` + "```" + `
trackfw não está instalado. Instale com uma das opções:

  curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
  npm install -g trackfw
  pip install trackfw
` + "```",

		"validate.md": `Execute o seguinte comando bash: ` + "`trackfw validate`" + `

Se o comando falhar com ` + "`trackfw: command not found`" + ` ou similar, informe ao usuário:

` + "```" + `
trackfw não está instalado. Instale com uma das opções:

  curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
  npm install -g trackfw
  pip install trackfw
` + "```",

		"status.md": `Execute o seguinte comando bash: ` + "`trackfw status`" + `

Se o comando falhar com ` + "`trackfw: command not found`" + ` ou similar, informe ao usuário:

` + "```" + `
trackfw não está instalado. Instale com uma das opções:

  curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
  npm install -g trackfw
  pip install trackfw
` + "```",

		"move.md": `Execute o seguinte comando bash: ` + "`trackfw roadmap move $ARGUMENTS`" + `

O formato esperado é: ` + "`<nome-do-roadmap> <estado>`" + `

Estados válidos: ` + "`backlog`, `analyzing`, `wip`, `blocked`, `done`, `abandoned`" + `

Exemplo: ` + "`/trackfw:move meu-roadmap analyzing`" + `

Se o comando falhar com ` + "`trackfw: command not found`" + ` ou similar, informe ao usuário:
trackfw não está instalado. Instale com:
  curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
  npm install -g trackfw
  pip install trackfw`,

		"roadmap.md": `Gere um roadmap de implementação em microlotes para uma REQ do projeto.

## Passos

1. **Listar REQs disponíveis**
   Use Glob para listar ` + "`docs/req/*.md`" + `. Se nenhum arquivo encontrado, informe:
   > Nenhuma REQ encontrada em ` + "`docs/req/`" + `. Crie uma primeiro com ` + "`/trackfw:req`" + `.

2. **Selecionar a REQ**
   - Se ` + "`$ARGUMENTS`" + ` foi fornecido: use como filtro (substring case-insensitive) para encontrar o arquivo
   - Se não foi fornecido ou o filtro não encontrar exatamente um: liste os arquivos disponíveis e pergunte ao usuário qual usar
   - Leia o conteúdo completo do arquivo REQ selecionado

3. **Gerar o roadmap**
   Com base no conteúdo da REQ, gere um roadmap seguindo **estritamente** este formato:

   ` + "````markdown" + `
   ---
   status: backlog
   date: <YYYY-MM-DD>
   req: "docs/req/<arquivo-selecionado>.md"
   squad: ""
   ---

   # Roadmap: <título derivado da REQ>

   > Created: <YYYY-MM-DD> | Status: backlog

   ## Diagnóstico / Contexto
   <resumo do problema, motivação e escopo extraídos da REQ>

   ## Wave 0 — Threat Model
   > Dependencies: none. Blocks all implementation.

   ### ML-0A — Threat model for this roadmap
   **Status:** ⬜ Pendente
   **Files affected:**
   **Actions:**
   1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).
   2. Threat model — who empties this Wave 0 without breaking any written rule, and how?
   3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?
   4. Declared residual — what this design accepts not covering.
   **Acceptance criteria:**
   - [ ] The four sections above answered with evidence, not a one-line assertion
   - [ ] No implementation line written for this ML

   **Gates da wave:**
   ` + "```bash" + `
   # Wave 0 gate — replace this placeholder with a project-specific check before
   # marking ML-0A done. Do not remove the gate; replace its command (AC13).
   # each line runs as a separate sh -c — see docs/cli-parity.md rule 5
   exit 1  # placeholder gate fails closed until ML-0A replaces it — see docs/cli-parity.md
   ` + "```" + `

   ## Wave 1 — <nome descritivo> (<N> MLs em paralelo)
   > Dependências: Independente

   ### ML-1A — <título>
   **Status:** ⬜ Pendente
   **Arquivos afetados:**
   - ` + "`caminho/exato/do/arquivo`" + `
   **Ações:**
   - Descrição detalhada da ação com valores, chaves e comandos exatos
   **Critérios de aceite:**
   - [ ] build sem erros
   - [ ] testes verdes
   **Comandos de validação:** ` + "`<comando de build e teste do projeto>`" + `

   ### ML-1B — <título> (se independente de ML-1A)
   ...

   ## Wave 2 — <nome> (depende de Wave 1)
   > Dependências: Wave 1 completa
   ...
   ` + "````" + `

   **Princípios obrigatórios:**
   - MLs dentro da mesma Wave são **independentes** (arquivos distintos, sem conflito)
   - Cada ML deve ser detalhado o suficiente para execução por um agente sem contexto extra
   - Maximizar paralelismo: agrupe em paralelo tudo que não compartilhar arquivos
   - Waves sequenciais apenas quando há dependência real de resultado
   - Critérios de aceite mensuráveis em cada ML

4. **Salvar o arquivo**
   - Calcule o slug: título em lowercase, espaços → hifens, remova caracteres especiais
   - Crie o arquivo em ` + "`docs/roadmaps/backlog/ROADMAP-<YYYY-MM-DD>-<slug>.md`" + `
   - Preencha ` + "`req:`" + ` com o caminho relativo completo da REQ selecionada
   - Use a data de hoje

5. **Confirmar**
   Informe o caminho do arquivo criado e um resumo das Waves e total de MLs gerados.
`,

		"barrier.md": "Você é o `trackfw_architect`, a única autoridade Git deste projeto. Este comando executa o checklist operacional de liberação de uma wave — nenhum outro agente commita, faz push ou libera a próxima wave.\n" +
			"\n" +
			"## Argumento\n" +
			"\n" +
			"`$ARGUMENTS` no formato `<roadmap> <wave>`. Se ausente ou incompleto, pergunte ao usuário qual roadmap (em `docs/roadmaps/wip/`) e qual número de wave validar.\n" +
			"\n" +
			"---\n" +
			"\n" +
			"## Núcleo determinístico\n" +
			"\n" +
			"Execute primeiro:\n" +
			"```bash\n" +
			"trackfw barrier <roadmap> --wave <n> --trust-local-gates --json\n" +
			"```\n" +
			"\n" +
			"`--trust-local-gates` é obrigatório aqui: roadmaps WIP (modificados localmente, ainda não commitados em\n" +
			"`origin/main`) são marcados como não confiáveis pela CLI direta por padrão, como proteção contra a\n" +
			"execução de gates de roadmaps chegados por PR de terceiro. O slash command aplica esse flag porque\n" +
			"ele representa o fluxo legítimo do arquiteto operando no próprio repositório — não porque os gates\n" +
			"são inspecionados previamente (o diff ainda é responsabilidade do checklist abaixo).\n" +
			"\n" +
			"⚠️ **Não use `--trust-local-gates` ao revisar um roadmap chegado por PR de terceiro** — use a CLI\n" +
			"direta sem o flag (`trackfw barrier <roadmap> --wave <n> --json`) para que os gates sejam marcados\n" +
			"como `not_evaluated` e não executados.\n" +
			"\n" +
			"Este comando é **necessário mas não suficiente**. Ele verifica MLs concluídos, evidências e `trackfw validate`, mas não substitui as inspeções especializadas nem a auditoria de diff abaixo — nenhuma delas é avaliada pelo binário. Consulte a seção `trackfw barrier` em `docs/cli-parity.md` para o contrato completo (estados, exit codes, saída JSON).\n" +
			"\n" +
			"Se o comando retornar exit code não-zero (`blocked` ou erro de resolução): pare, reporte a falha ao usuário e não prossiga no checklist até que a wave passe.\n" +
			"\n" +
			"---\n" +
			"\n" +
			"## Definição de pronto da barrier — checklist completo\n" +
			"\n" +
			"Antes de liberar a próxima wave, confirme cada item com evidência concreta — não presuma:\n" +
			"\n" +
			"1. **Todos os MLs da wave concluídos e marcados** — cada ML da wave está com `**Status:** ✅ Concluído` no roadmap.\n" +
			"2. **Testes unitários e E2E aplicáveis executados** — rode os comandos de validação declarados em cada ML.\n" +
			"3. **Build aplicável sem erros** — rode o comando de build do(s) workspace(s) afetado(s).\n" +
			"4. **Cada critério de aceite inspecionado com evidência** — leia os arquivos modificados e confirme contra os critérios listados, não apenas contra os testes.\n" +
			"5. **Agente code-quality reportou conformidade, performance, robustez e clareza** — invoque o agente `code-quality` quando a mudança introduzir lógica nova, duplicação relevante ou risco de manutenibilidade.\n" +
			"6. **Agente security reportou SAST, privilégios, controle de acesso e camadas aplicáveis** — invoque o agente `security` quando a mudança tocar autenticação, segredos, entrada externa ou permissões.\n" +
			"7. **Gates pré-commit declarados pelo projeto executados** — rode os hooks/gates configurados (lint, format, testes de contrato).\n" +
			"8. **`trackfw validate --json` aprovado** — execute e confirme zero violações.\n" +
			"9. **Diff auditado contra o escopo** — revise o diff completo; confirme que não há alterações de agentes concorrentes nem arquivos fora do escopo do ML (ex: `docs/adr/`, `docs/req/`, `docs/roadmaps/` quando não autorizado ao especialista).\n" +
			"10. **Resultado registrado antes de liberar a próxima wave** — anote no roadmap ou na resposta ao usuário que a wave passou, com a evidência de cada item acima.\n" +
			"\n" +
			"Se qualquer item falhar: bloqueie a próxima wave, identifique o item e o agente responsável, e despache um microlote corretivo. Só repita o checklist depois que o corretivo for concluído.\n" +
			"\n" +
			"---\n" +
			"\n" +
			"## Autoridade Git\n" +
			"\n" +
			"Somente o `trackfw_architect` cria branch, audita diff, commita e faz push. Especialistas entregam trabalho sem commit — cabe a este papel revisar, commitar e sugerir a abertura de PR/MR (sem abrir automaticamente sem autorização do usuário).\n",

		"architect.md": `Você é o guia de arquitetura do trackfw. Ajude o usuário a escolher a stack correta e arquitetar a aplicação em linguagem simples, acessível para times não técnicos.

## Passo 1 — Descoberta de Negócio

Faça ao usuário as seguintes perguntas em linguagem simples, uma por vez:

1. "O que sua aplicação vai fazer? Descreva em 2-3 frases como se fosse explicar para alguém de fora da TI."
2. "Quantas pessoas vão usar esse sistema simultaneamente? (< 10 pessoas / 10-100 pessoas / > 100 pessoas)"
3. "Esse sistema vai para produção de verdade ou é um protótipo para validar uma ideia?"
4. "Você precisa de login/autenticação de usuários? (Sim / Não / Não sei)"
5. "Tem alguma restrição de tecnologia ou preferência da empresa? (ex: só Java, só Microsoft, etc.)"

---

## Passo 2 — Recomendação de Stack

Com base nas respostas, escolha **UM** dos combos pré-validados:

### Combo A — Protótipo Rápido
**Quando usar:** prototipagem, validação de ideia, até ~10 usuários, sem pressão de produção.
- **Frontend:** React + Vite
- **Backend:** FastAPI (Python) ou Express (Node.js)
- **Banco:** SQLite + SQLAlchemy / Prisma
- **Auth:** JWT simples quando necessário
- **Docker:** Dockerfile básico para o backend

### Combo B — Sistema Pequeno/Médio em Produção
**Quando usar:** sistema real, 10-100 usuários, robustez e manutenibilidade.
- **Frontend:** Next.js (SSR + rotas prontas)
- **Backend:** FastAPI (Python) ou NestJS (Node.js)
- **Banco:** PostgreSQL + ORM (SQLAlchemy / Prisma / TypeORM)
- **Auth:** OAuth2 com JWT (Supabase Auth ou Auth0)
- **Docker:** docker-compose com frontend + backend + banco

### Combo C — Enterprise / Java
**Quando usar:** integração com sistemas corporativos, > 100 usuários, exigência de Java.
- **Frontend:** Angular
- **Backend:** Spring Boot
- **Banco:** PostgreSQL + Hibernate
- **Auth:** Spring Security + OAuth2 (Keycloak ou Azure AD)
- **Docker:** docker-compose com todos os serviços

Apresente o combo recomendado com explicação simples do motivo.

---

## Passo 3 — Arquitetura em Camadas (explicação simples)

Explique a arquitetura com uma metáfora de negócio:

"Pense na aplicação como um restaurante:
- **Frontend** = o salão: o que o cliente vê e interage
- **Backend** = a cozinha: onde as regras de negócio acontecem, nunca exposta diretamente
- **Banco de dados** = a despensa: onde os dados ficam guardados, acessada só pela cozinha"

Reforce as **Architecture Directives** já injetadas no CLAUDE.md deste projeto: separação em 3 camadas sem dados em memória (sempre DB + ORM), auth + Docker + .env desde o dia 1, validação em 2 camadas, contrato OpenAPI antes de codar, wave de segurança em todo roadmap e cobertura mínima de testes (60% protótipo / 80% produção).

---

## Passo 4 — Gerar o ADR de Stack

Execute ` + "`/trackfw:adr`" + ` com o título: ` + "`\"Stack e arquitetura em camadas — [nome do projeto]\"`" + `

O ADR deve registrar a stack escolhida (combo e componentes), motivação baseada nas respostas, alternativas descartadas e princípios de arquitetura adotados.

---

## Passo 5 — Próximos Passos

Oriente o usuário:

` + "```" + `
✅ Stack definida. Próximos passos:

1. Crie a REQ da primeira feature com /trackfw:req
2. Gere o roadmap em microlotes com /trackfw:roadmap
3. Inicie a implementação com /trackfw:implement
` + "```",

		"implement.md": `Você é o orquestrador de implementação do trackfw. Siga o fluxo abaixo **sem pular etapas**.

## Argumento

` + "`$ARGUMENTS`" + ` é opcional. Se fornecido, é usado como filtro (substring case-insensitive) sobre os nomes de arquivo das REQs.

---

## Passo 1 — Selecionar a REQ

Use Glob para listar ` + "`docs/req/*.md`" + `.

- Se **nenhum arquivo encontrado**: informe que não há REQs disponíveis e sugira criar com ` + "`/trackfw:req`" + `.
- Se **` + "`$ARGUMENTS`" + ` foi fornecido** e filtra para exatamente uma REQ: use-a diretamente.
- Em **todos os outros casos** (sem argumento, ou argumento ambíguo): apresente a lista de REQs disponíveis e pergunte ao usuário qual deseja implementar.

Leia o conteúdo completo da REQ selecionada.

---

## Passo 2 — Encontrar ou gerar o Roadmap

Verifique se existe um roadmap vinculado à REQ buscando em ` + "`docs/roadmaps/`" + ` (backlog, wip, blocked, done, abandoned) por arquivo cujo nome contenha o slug da REQ.

**Se o roadmap ainda não existe:**
- Informe o usuário: "Nenhum roadmap encontrado para esta REQ. Gerando agora..."
- Execute o fluxo completo de geração do ` + "`/trackfw:roadmap`" + ` (leia o arquivo ` + "`.claude/commands/trackfw/roadmap.md`" + ` para seguir as instruções exatas), passando a REQ já selecionada — não pergunte novamente.
- Salve o roadmap gerado em ` + "`docs/roadmaps/backlog/ROADMAP-<YYYY-MM-DD>-<slug>.md`" + `.

**Se o roadmap existe e já está em ` + "`done/`" + ` ou ` + "`abandoned/`" + `:**
- Informe o usuário e pergunte se deseja criar um novo roadmap ou encerrar.

**Se o roadmap existe em ` + "`backlog/`" + ` ou ` + "`blocked/`" + `:**
- Prossiga para o Passo 3.

**Se já está em ` + "`wip/`" + `:**
- Prossiga diretamente para o Passo 4 (já está em execução).

---

## Passo 3 — Mover roadmap para WIP

Execute:
` + "```bash" + `
trackfw roadmap move <nome-do-roadmap> wip
` + "```" + `

Confirme que o arquivo foi movido para ` + "`docs/roadmaps/wip/`" + `.

---

## Passo 4 — Ler e apresentar o plano

Leia o roadmap (agora em ` + "`wip/`" + `). Apresente ao usuário:
- Título do roadmap
- Total de Waves e MLs
- Lista resumida dos MLs por Wave

Confirme: "Iniciando implementação. Vou executar cada ML em ordem e atualizar o roadmap a cada conclusão."

---

## Passo 5 — Executar cada ML em ordem

Para cada Wave (em sequência), execute os MLs da Wave:

### Para cada ML:

**5a. Anunciar:** informe qual ML está sendo executado (ex: "Executando ML-1A — Criar client.go").

**5b. Implementar:** execute as ações descritas no ML usando suas ferramentas (Read, Write, Edit, Bash). Siga exatamente os arquivos afetados, ações e critérios de aceite listados no roadmap.

**5c. Validar:** execute os comandos de validação do ML. Se falhar, corrija antes de avançar.

**5d. Atualizar o roadmap:** edite o arquivo de roadmap em ` + "`docs/roadmaps/wip/`" + ` substituindo o status do ML:
- ` + "`**Status:** ⬜ Pendente`" + ` → ` + "`**Status:** ✅ Concluído`" + `

**5e. Commitar:**
` + "```bash" + `
git add -A
git commit -m "feat(<escopo>): <descrição do ML>"
` + "```" + `

Só avance para a próxima Wave após todos os MLs da Wave atual estarem ✅.

---

## Passo 6 — Finalizar

Quando todos os MLs estiverem ✅:

**6a.** Execute ` + "`trackfw validate`" + ` — deve passar com zero violations.

**6b.** Mova o roadmap para done:
` + "```bash" + `
trackfw roadmap move <nome-do-roadmap> done
` + "```" + `

**6c.** Faça o commit final:
` + "```bash" + `
git add docs/roadmaps/
git commit -m "docs(trackfw): roadmap <nome> → done"
` + "```" + `

**6d.** Informe o usuário:
` + "```" + `
✅ Implementação concluída.
Roadmap: docs/roadmaps/done/<nome>.md
Próximo passo: abrir PR com gh pr create
` + "```",
	}
}

func generateClaudeCommandsInner(force bool) error {
	ccRoot, ccErr := projectRoot()
	if ccErr != nil {
		return fmt.Errorf("generateClaudeCommandsInner: %w", ccErr)
	}
	absCommandsDir := filepath.Join(ccRoot, ClaudeCommandsDirPath)
	if err := rejectScaffoldPath(ccRoot, absCommandsDir); err != nil {
		return err
	}
	dir := ClaudeCommandsDirPath
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}

	commands := claudeCommandsContent()

	created, skipped := 0, 0
	for filename, content := range commands {
		path := filepath.Join(dir, filename)
		if _, err := os.Stat(path); err == nil && !force {
			skipped++
			continue
		}
		// Guard the leaf file — directory guard above does not detect a symlinked filename.
		absFilePath := filepath.Join(ccRoot, path)
		if err := rejectScaffoldPath(ccRoot, absFilePath); err != nil {
			return err
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectScaffoldPath above
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
		created++
	}
	if skipped > 0 {
		fmt.Printf("  ✓ %s (%d slash commands criados, %d já existiam — não sobrescritos)\n", dir, created, skipped)
	} else {
		fmt.Printf("  ✓ %s (%d slash commands)\n", dir, created)
	}
	return nil
}

// configBlock representa um bloco de configuração extraído do template gerado.
// text inclui: separador blank líder (se houver), linhas de comentário adjacentes
// (P3: comentário = ausente), linha-chave em coluna 0 e corpo (linhas indentadas).
type configBlock struct {
	key  string
	text string
}

// presentTopLevelKeys devolve o conjunto de chaves de nível 0 presentes no
// conteúdo YAML, respeitando as quatro propriedades da ADR:
//
//   - P1: âncora em coluna 0 — chave começa sem espaço/tab.
//   - P2: dois-pontos obrigatórios — compara key+":" para evitar colisão de
//     prefixo (ex: roadmap_dir vs. roadmap_namespacing).
//   - P3: linha comentada = chave ausente — linhas que começam com "#" após
//     TrimSpace são ignoradas.
//
// A função intenciona detectar presença, não ausência — não confundir com
// parseConfigBlocks, que extrai blocos do conteúdo *gerado* (não do existente).
func presentTopLevelKeys(content string) map[string]bool {
	keys := make(map[string]bool)
	for _, line := range strings.Split(content, "\n") {
		// P3: ignora linhas de comentário
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		// P1: âncora em coluna 0 — ignora linhas indentadas e em branco
		if len(line) == 0 || line[0] == ' ' || line[0] == '\t' {
			continue
		}
		// P2: dois-pontos obrigatórios
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		keys[line[:idx]] = true
	}
	return keys
}

// parseConfigBlocks extrai os blocos de configuração do conteúdo *gerado* pelo
// template. Cada bloco cobre:
//
//  1. Um blank líder (separador visual entre blocos), quando presente.
//  2. Linhas de comentário imediatamente adjacentes (sem blank entre elas e a
//     chave) — precedente de generateGitAttributes: comentário que pertence à
//     chave deve aparecer junto quando a chave for acrescentada.
//  3. A linha-chave em coluna 0.
//  4. O corpo: linhas indentadas até a próxima chave de nível 0 (ou EOF).
//
// O cabeçalho do arquivo (linhas antes do primeiro bloco, separadas por blank)
// não faz parte de nenhum bloco e nunca é acrescentado.
func parseConfigBlocks(content string) []configBlock {
	lines := strings.Split(content, "\n")

	// Encontra os índices de todas as linhas-chave de nível 0.
	var keyIndices []int
	for i, line := range lines {
		if len(line) == 0 || line[0] == ' ' || line[0] == '\t' || line[0] == '#' {
			continue
		}
		if strings.Contains(line, ":") {
			keyIndices = append(keyIndices, i)
		}
	}
	if len(keyIndices) == 0 {
		return nil
	}

	// Calcula blockStart para cada chave: a linha-chave mais o contexto
	// imediatamente anterior (comentários adjacentes + um blank líder).
	blockStarts := make([]int, len(keyIndices))
	for ki, keyIdx := range keyIndices {
		start := keyIdx
		// Inclui linhas de comentário imediatamente adjacentes (sem blank entre).
		for start > 0 {
			prev := lines[start-1]
			if strings.HasPrefix(strings.TrimSpace(prev), "#") {
				start--
			} else {
				break
			}
		}
		// Inclui um único blank líder (separador visual entre blocos), quando
		// imediatamente anterior às linhas de comentário ou à própria chave.
		// Precedente: generateGitAttributes/generateGitIgnore usam o mesmo guarda.
		if start > 0 && strings.TrimSpace(lines[start-1]) == "" {
			start--
		}
		blockStarts[ki] = start
	}

	blocks := make([]configBlock, 0, len(keyIndices))
	for ki, keyIdx := range keyIndices {
		// blockEnd = blockStart do próximo bloco, ou EOF.
		var blockEnd int
		if ki+1 < len(keyIndices) {
			blockEnd = blockStarts[ki+1]
		} else {
			blockEnd = len(lines)
		}

		key := lines[keyIdx][:strings.Index(lines[keyIdx], ":")]

		text := strings.Join(lines[blockStarts[ki]:blockEnd], "\n")
		// Garante newline final (equivalente ao guarda de generateGitAttributes
		// linhas 2674-2676 e generateGitIgnore linhas 2779-2781).
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}

		blocks = append(blocks, configBlock{key: key, text: text})
	}
	return blocks
}

func writeTrackfwConfig(cfg Config) error {
	wipLimit := cfg.WipLimit
	if wipLimit <= 0 {
		wipLimit = 1
	}
	wipBySquad := "false"
	if cfg.WipBySquad {
		wipBySquad = "true"
	}

	requireReqInCommit := "false"
	if cfg.RequireReqInCommit {
		requireReqInCommit = "true"
	}

	content := fmt.Sprintf(`# trackfw configuration
# generated: %s

frontend: %s
backend: %s
backend_framework: %s
pkg_manager: %s
hooks: %s
ci: %s
wip_limit: %d
wip_by_squad: %s
require_req_in_commit: %s

# validator rules (off / warning / error)
rules:
  branch_has_wip_roadmap: error

# governance paths (edit to match your project structure)
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: flat
`, time.Now().Format("2006-01-02"), cfg.Frontend, cfg.Backend, cfg.BackendFramework, cfg.PkgManager, cfg.Hooks, cfg.CI, wipLimit, wipBySquad, requireReqInCommit)

	if cfg.Forge != "" {
		content += fmt.Sprintf("forge: %s\n", cfg.Forge)
	}

	if cfg.AgentConventions != "" {
		indented := strings.ReplaceAll(cfg.AgentConventions, "\n", "\n  ")
		content += fmt.Sprintf("agent_conventions: |\n  %s\n", indented)
	}

	if cfg.BrownfieldMode {
		content += fmt.Sprintf("governance_mode: lenient\nlenient_until: %s\n", cfg.LenientUntil.Format("2006-01-02"))
	}

	root, cfgRootErr := projectRoot()
	if cfgRootErr != nil {
		return fmt.Errorf("writeTrackfwConfig: %w", cfgRootErr)
	}
	absConfig := filepath.Join(root, "trackfw.yaml")
	if err := rejectScaffoldPath(root, absConfig); err != nil {
		return err
	}

	existing, readErr := os.ReadFile("trackfw.yaml")
	if readErr != nil {
		if !os.IsNotExist(readErr) {
			return fmt.Errorf("reading trackfw.yaml: %w", readErr)
		}
		// Arquivo ausente: escreve o template completo (caminho de criação).
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if err := os.WriteFile("trackfw.yaml", []byte(content), 0644); err != nil {
			return fmt.Errorf("writing trackfw.yaml: %w", err)
		}
		fmt.Println("  ✓ trackfw.yaml")
		return nil
	}

	// Arquivo presente: merge textual por chave ausente (ADR-2026-09-28).
	// Preserva todo valor já escrito pelo consumidor; acrescenta apenas as chaves
	// que faltam. Zero diff nas linhas pré-existentes quando não há chave nova.
	existingKeys := presentTopLevelKeys(string(existing))
	blocks := parseConfigBlocks(content)

	var missing []string
	for _, b := range blocks {
		if !existingKeys[b.key] {
			missing = append(missing, b.text)
		}
	}

	if len(missing) == 0 {
		// Todas as chaves já presentes: no-op (garante zero diff).
		return nil
	}

	// Acrescenta blocos ausentes.
	// P4 — guarda de newline final: precedente de generateGitAttributes (linhas
	// 2674-2676) e generateGitIgnore (linhas 2779-2781). Sem este guarda, o
	// primeiro bloco acrescentado grudaria na última linha pré-existente.
	out := string(existing)
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	for _, text := range missing {
		out += text
	}

	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.WriteFile("trackfw.yaml", []byte(out), 0644); err != nil {
		return fmt.Errorf("writing trackfw.yaml: %w", err)
	}
	fmt.Println("  ✓ trackfw.yaml")
	return nil
}

func generateValidateScript(cfg Config) error {
	vsRoot, vsErr := projectRoot()
	if vsErr != nil {
		return fmt.Errorf("generateValidateScript: %w", vsErr)
	}
	absScripts := filepath.Join(vsRoot, "scripts")
	if err := rejectScaffoldPath(vsRoot, absScripts); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.MkdirAll("scripts", 0755); err != nil {
		return err
	}

	script := buildValidateScript(cfg)
	path := filepath.Join("scripts", "trackfw-validate.sh")
	// Guard the leaf file — directory guard above does not detect a symlinked script file.
	absValidatePath := filepath.Join(vsRoot, path)
	if err := rejectScaffoldPath(vsRoot, absValidatePath); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectScaffoldPath above
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		return fmt.Errorf("writing validate script: %w", err)
	}
	// AC9 (REQ-2026-08-28): os.WriteFile applies perm only on O_CREATE; for an existing
	// file O_TRUNC rewrites content but leaves the inode mode unchanged. os.Chmod is
	// unconditional and restores 0755 even when the file already existed with 0644.
	// This matches Python's os.chmod behavior (which was already correct) and raises
	// three-runtime parity for the update path.
	if err := os.Chmod(path, 0755); err != nil {
		return fmt.Errorf("setting execute bit on validate script: %w", err)
	}
	fmt.Printf("  ✓ %s\n", path)
	return nil
}

// attentionSignalScript is the canonical content of the attention-signal hook written
// by trackfw init/update and compared by scaffold doctor (ADR-2026-08-27).
// Mirrors npm/src/generators/hooks.js and pypi/trackfw/generators/init_gen.py byte-for-byte.
//
// CRLF normalisation (ML-3B, ML-3E — REQ-2026-09-23 Wave 3 reopening):
//
// python3 path (TOOL/MSG): sed $'s/\r$//' strips the trailing \r that python3 print() appends in
// Windows text mode after [:300]. Placed after [:300] because [:300] executes inside python3 and
// the \r is appended by print() afterwards. Relies on set -o pipefail (line 3): python3 non-zero
// exit propagates through sed so the || fallback fires; without pipefail sed returns 0 and the
// fallback silences, leaving the variable empty. See Wave 0 parecer (2026-09-27).
//
// ROADMAP_DIR path: tr -d '\r' strips all CR from the extracted value.  A trailing \r causes
// mkdir -p to create "docs/roadmaps\r" and the attention signal goes to the wrong directory with
// no error — the signal silences.  Decision — normalise, not reject: rejecting would fall back to
// "docs/roadmaps", which is the same silent-wrong-path outcome.  Other control characters and yaml
// hygiene belong in `trackfw validate/doctor`, which have a user-facing channel this hook does not.
// This normalisation is autocontained (no lib-crlf-normalize.sh — the consumer does not have it).
//
// 🔴 DO NOT use sed $'s/\r//g' here. Measured on Git Bash (GNU sed 4.9, Windows): produces
//    "sed: -e expression #1, char 0: no previous regular expression" and ROADMAP_DIR becomes empty.
//    The form fails even on LF-only input — every Windows consumer loses the custom roadmap_dir.
//    tr -d '\r' works on both macOS (BSD) and Windows (Git Bash). Measured 2026-09-27.
const attentionSignalScript = `#!/usr/bin/env bash
# trackfw attention signal — PreToolUse/BeforeTool hook
set -euo pipefail

INPUT=$(cat)

# Script is intentionally a no-op when executed outside the project root
[ -f "trackfw.yaml" ] || exit 0

if command -v jq &>/dev/null; then
  TOOL=$(echo "$INPUT" | jq -r '.tool_name // ""')
  MSG=$(echo "$INPUT" | jq -r '(.tool_input.question // .tool_input.command // "Agent is executing: \(.tool_name // "unknown")") | .[0:300]')
else
  # sed $'s/\r$//' strips trailing \r added by python3 print() in Windows text mode.
  # Requires set -o pipefail (line 3) so python3 failure propagates and || fallback fires.
  TOOL=$(echo "$INPUT" | PYTHONIOENCODING=utf-8 python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('tool_name',''))" 2>/dev/null | sed $'s/\r$//' || echo "")
  MSG=$(echo "$INPUT" | PYTHONIOENCODING=utf-8 python3 -c "import sys,json; d=json.load(sys.stdin); ti=d.get('tool_input',{}); print((ti.get('question') or ti.get('command') or 'Agent is executing: '+d.get('tool_name','unknown'))[:300])" 2>/dev/null | sed $'s/\r$//' || echo "Agent needs attention")
fi

# tr -d '\r': Windows CRLF in trackfw.yaml corrupts the path; normalise (not reject).
# DO NOT use sed $'s/\r//g' -- fails on Git Bash (GNU sed 4.9): "no previous regular expression".
ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml 2>/dev/null | head -1 | sed 's/^roadmap_dir:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d '"' | tr -d "'" | tr -d '\r' || true)
ROADMAP_DIR=${ROADMAP_DIR:-docs/roadmaps}

case "$ROADMAP_DIR" in
  /*|../*|*/../*|*/..|..) ROADMAP_DIR="docs/roadmaps" ;;
esac

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

TOOL_ESC=$(echo "$TOOL" | tr -d '\000-\037' | sed 's/\\/\\\\/g; s/"/\\"/g')
MSG_ESC=$(echo "$MSG" | tr -d '\000-\037' | sed 's/\\/\\\\/g; s/"/\\"/g')

mkdir -p "$ROADMAP_DIR"
printf '{"tool":"%s","message":"%s","level":"action_required","timestamp":"%s"}\n' \
  "$TOOL_ESC" \
  "$MSG_ESC" \
  "$TIMESTAMP" > "$ROADMAP_DIR/.trackfw-attention.json"

exit 0
`

// attentionCleanupScript is the canonical content of the attention-cleanup hook written
// by trackfw init/update and compared by scaffold doctor (ADR-2026-08-27).
// Mirrors npm/src/generators/hooks.js and pypi/trackfw/generators/init_gen.py byte-for-byte.
// CRLF normalisation: see attentionSignalScript comment block above.
const attentionCleanupScript = `#!/usr/bin/env bash
# trackfw attention cleanup — PostToolUse/AfterTool hook
set -euo pipefail

# Script is intentionally a no-op when executed outside the project root
[ -f "trackfw.yaml" ] || exit 0

# tr -d '\r': Windows CRLF in trackfw.yaml corrupts the path; normalise (not reject).
# DO NOT use sed $'s/\r//g' -- fails on Git Bash (GNU sed 4.9): "no previous regular expression".
ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml 2>/dev/null | head -1 | sed 's/^roadmap_dir:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d '"' | tr -d "'" | tr -d '\r' || true)
ROADMAP_DIR=${ROADMAP_DIR:-docs/roadmaps}

case "$ROADMAP_DIR" in
  /*|../*|*/../*|*/..|..) ROADMAP_DIR="docs/roadmaps" ;;
esac

rm -f "$ROADMAP_DIR/.trackfw-attention.json"
exit 0
`

// GenerateAttentionScripts gera os scripts shell de attention signal/cleanup em
// <rootDir>/scripts. Se rootDir for "", usa o diretório de trabalho atual (mesmo
// comportamento de antes da exportação). O conteúdo gerado é idêntico ao produzido
// por `trackfw init`.
func GenerateAttentionScripts(rootDir string) error {
	root, err := scaffoldRoot(rootDir)
	if err != nil {
		return fmt.Errorf("GenerateAttentionScripts: %w", err)
	}
	rootDir = root
	scriptsDir := filepath.Join(rootDir, "scripts")
	if err := rejectScaffoldPath(root, scriptsDir); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return err
	}

	signalPath := filepath.Join(scriptsDir, "trackfw-attention-signal.sh")
	// Guard the leaf file — directory guard above does not detect a symlinked script file.
	if err := rejectScaffoldPath(root, signalPath); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectScaffoldPath above
	if err := os.WriteFile(signalPath, []byte(attentionSignalScript), 0755); err != nil {
		return fmt.Errorf("writing attention signal script: %w", err)
	}
	// AC9: restore execute bit unconditionally — see generateValidateScript for rationale.
	if err := os.Chmod(signalPath, 0755); err != nil {
		return fmt.Errorf("setting execute bit on attention signal script: %w", err)
	}
	// Mensagem sempre com caminho relativo "scripts/..." — igual ao literal fixo
	// que o Node.js imprime (npm/src/generators/hooks.js:generateAttentionScripts)
	// — independente de rootDir ser "" (cwd, usado por init/update) ou um caminho
	// absoluto (usado por discover --init via InstallGates).
	fmt.Printf("  ✓ %s\n", filepath.Join("scripts", "trackfw-attention-signal.sh"))

	cleanupPath := filepath.Join(scriptsDir, "trackfw-attention-cleanup.sh")
	// Guard the leaf file — directory guard above does not detect a symlinked script file.
	if err := rejectScaffoldPath(root, cleanupPath); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectScaffoldPath above
	if err := os.WriteFile(cleanupPath, []byte(attentionCleanupScript), 0755); err != nil {
		return fmt.Errorf("writing attention cleanup script: %w", err)
	}
	// AC9: restore execute bit unconditionally.
	if err := os.Chmod(cleanupPath, 0755); err != nil {
		return fmt.Errorf("setting execute bit on attention cleanup script: %w", err)
	}
	fmt.Printf("  ✓ %s\n", filepath.Join("scripts", "trackfw-attention-cleanup.sh"))

	return nil
}

// GenerateCredentialGuardScript gera o script shell trackfw-credential-guard.sh em
// <rootDir>/scripts. Se rootDir for "", usa o diretório de trabalho atual. Este ML (1A) só cria
// o script — não o injeta em nenhum hooks.json/settings.json de CLI (isso é escopo da Wave 2, ver
// ROADMAP-2026-08-05-hooks-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes.md).
//
// O script lê o payload bruto de stdin (mesmo mecanismo usado por trackfw-attention-signal.sh, sem
// jq/python3 — grep/sed simples), procura padrão de JWT ou de AWS access key no payload inteiro
// (cobre tanto tool_input.command em PreToolUse quanto o campo de saída em PostToolUse, sem
// diferenciar o evento) e, se encontrar, decide avisar (`credential_guard.mode: warn`, default) ou
// bloquear (`credential_guard.mode: block`, exit 2) — lido de trackfw.yaml via grep simples, sem
// parser YAML completo. Uma correspondência sem nenhum redirecionamento (ex.: impressa em stdout)
// ou redirecionada para um caminho de arquivo comum sempre alerta; só é ignorada quando TODOS os
// alvos de redirecionamento do payload são efêmeros (/dev/null ou um caminho derivado de mktemp,
// incluindo uma variável atribuída via `VAR=$(mktemp...)` antes do redirecionamento).
func GenerateCredentialGuardScript(rootDir string) error {
	root, err := scaffoldRoot(rootDir)
	if err != nil {
		return fmt.Errorf("GenerateCredentialGuardScript: %w", err)
	}
	rootDir = root
	scriptsDir := filepath.Join(rootDir, "scripts")
	if err := rejectScaffoldPath(root, scriptsDir); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return err
	}

	path := filepath.Join(scriptsDir, "trackfw-credential-guard.sh")
	// Guard the leaf file — directory guard above does not detect a symlinked script file.
	if err := rejectScaffoldPath(root, path); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectScaffoldPath above
	if err := os.WriteFile(path, []byte(credentialGuardScript), 0755); err != nil {
		return fmt.Errorf("writing credential guard script: %w", err)
	}
	// AC9: restore execute bit unconditionally.
	if err := os.Chmod(path, 0755); err != nil {
		return fmt.Errorf("setting execute bit on credential guard script: %w", err)
	}
	fmt.Printf("  ✓ %s\n", filepath.Join("scripts", "trackfw-credential-guard.sh"))

	return nil
}

// GenerateGlobalCredentialGuardScript gera o script shell trackfw-credential-guard.sh em escopo
// global, em <home>/.trackfw/scripts/trackfw-credential-guard.sh. Destinado a ser referenciado por
// hooks globais de CLI (~/.claude/settings.json, ~/.codex/hooks.json etc.), instalados via
// `trackfw update harness` (ver ROADMAP-2026-08-06, Wave 2) — não é chamado por `trackfw
// init`/`trackfw update` (escopo de projeto), que continuam usando GenerateCredentialGuardScript.
//
// Diferente da variante de projeto, este script não tem a guarda "só roda dentro de um projeto
// trackfw.yaml" — protege qualquer projeto que o usuário abra com o CLI onde o hook global foi
// instalado. Ver globalCredentialGuardScript/credentialGuardGlobalTail para a decisão de design
// sobre a fonte do modo (fallback "block" por padrão, ADR-2026-08-06 emenda 6; respeita
// credential_guard.mode explícito de trackfw.yaml quando presente) e do diretório de attention.
//
// Escreve silenciosamente (sem fmt.Printf) — seu único chamador de produção é UpdateHarness, que
// roda antes de qualquer target por-CLI ser avaliado, inclusive com `--json`; um print aqui vazaria
// texto solto para o stdout antes do JSON e quebraria o parse (mesmo motivo pelo qual
// harnessClaudeSkillTarget escreve via os.WriteFile direto em vez de reusar installGlobalSkillInner,
// que também imprime).
func GenerateGlobalCredentialGuardScript(home string) error {
	if home == "" {
		return fmt.Errorf("home directory vazio")
	}
	// ML-8A / #402: see GenerateGlobalSkill — one assignment from one resolver.
	absHome, err := pathguard.ResolveRoot(home)
	if err != nil {
		return fmt.Errorf("resolving home: %w", err)
	}
	scriptsDir := filepath.Join(absHome, ".trackfw", "scripts")
	if err := rejectScaffoldPath(absHome, scriptsDir); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return err
	}

	path := filepath.Join(scriptsDir, "trackfw-credential-guard.sh")
	// Guard the leaf file — directory guard above does not detect a symlinked script file.
	if err := rejectScaffoldPath(absHome, path); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectScaffoldPath above
	if err := os.WriteFile(path, []byte(globalCredentialGuardScript), 0755); err != nil {
		return fmt.Errorf("writing global credential guard script: %w", err)
	}

	return nil
}

// credentialGuardHeader é o cabeçalho comum aos dois escopos (projeto e global) do hook: shebang,
// set -e e leitura do payload de stdin.
const credentialGuardHeader = `#!/usr/bin/env bash
# trackfw credential guard — PreToolUse/PostToolUse hook
set -euo pipefail

INPUT=$(cat)

`

// credentialGuardProjectGuardBlock só existe na variante de projeto: torna o script um no-op fora
// da raiz de um projeto trackfw. A variante global (GenerateGlobalCredentialGuardScript) não inclui
// este bloco — o objetivo do escopo global é proteger qualquer projeto, com ou sem trackfw.yaml.
const credentialGuardProjectGuardBlock = `# Script is intentionally a no-op when executed outside the project root
[ -f "trackfw.yaml" ] || exit 0

`

// credentialGuardDetectionCore é o núcleo de detecção (padrões JWT/AWS key, checagem de destino
// efêmero de redirecionamento, segunda camada de detecção via conteúdo de arquivo referenciado) —
// idêntico entre a variante de projeto e a global. Nunca duplicar esta lógica em outro lugar; as
// duas variantes do script compõem o conteúdo final a partir deste mesmo bloco.
//
// Segunda camada de detecção (ADR-2026-08-06, emenda 8 de 2026-08-08): quando o payload cru não
// contém o padrão (ex.: `head -c 50 /tmp/token.txt`, sem o JWT literal no comando), o script passa
// a inspecionar o CONTEÚDO de arquivos referenciados — (a) alvos de redirecionamento já capturados
// por REDIRECTS que não sejam efêmeros, e (b) argumentos de arquivo existente quando o comando é um
// dos inspetores comuns (cat/head/tail/jq/grep) — com teto de 1MB para não ler arquivos grandes a
// cada tool call. O nome do comando é extraído do campo JSON "command" do payload (não do primeiro
// token de $RAW: $RAW é o payload JSON inteiro, ex.:
// `{"tool_name":"Bash","tool_input":{"command":"head -c 50 /tmp/x"}}` — o primeiro token
// word-splitted seria o prefixo JSON, não "head"). A extração via
// `sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p'` captura o valor do campo até
// a primeira aspa não escapada; suficiente para o payload plano típico de hook (sem parser JSON
// completo, mesmo espírito do resto do script) — um argumento com aspas internas ("$TMPFILE", por
// exemplo) trunca a captura, mas esse caso já é coberto pela camada 1 (payload cru) na prática. Ver
// nota de vault credential-guard-second-layer-cmd-extraction-json-not-raw-token-2026-08-08.
const credentialGuardDetectionCore = `JWT_PATTERN='eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+'
AWS_KEY_PATTERN='AKIA[0-9A-Z]{16}'

MATCH=""
if printf '%s' "$INPUT" | grep -qE "$JWT_PATTERN"; then
  MATCH="JWT"
elif printf '%s' "$INPUT" | grep -qE "$AWS_KEY_PATTERN"; then
  MATCH="AWS access key"
fi

# The raw payload is JSON: any double quote inside the underlying tool_input.command is
# escaped as \" -- unescape those before scanning for redirect targets, or a quoted target
# like "$TMPFILE" is seen as starting with a literal backslash instead of a variable
# reference.
RAW=$(printf '%s' "$INPUT" | sed 's/\\"/"/g')

# Ignore matches that are only ever written to an ephemeral destination
# (mktemp-derived path or /dev/null). A match with no redirect at all
# (printed to stdout, e.g.) or redirected to a plain file path still
# alerts -- that is the incident this hook guards against.
is_ephemeral_target() {
  local target
  target=$(printf '%s' "$1" | tr -d "\"'" | sed -E 's/[},]+$//')
  case "$target" in
    /dev/null) return 0 ;;
    *mktemp*) return 0 ;;
  esac
  if printf '%s' "$target" | grep -qE '^\$\{?[A-Za-z_][A-Za-z0-9_]*\}?$'; then
    local varname pattern
    varname=$(printf '%s' "$target" | sed -E 's/^\$\{?([A-Za-z_][A-Za-z0-9_]*)\}?$/\1/')
    pattern="*${varname}="'$(mktemp'"*"
    case "$RAW" in
      $pattern) return 0 ;;
    esac
  fi
  return 1
}

REDIRECTS=$(printf '%s' "$RAW" | grep -oE '[0-9]?>>?[[:space:]]*[^[:space:]|&;,:]+' || true)

# Second detection layer: only runs when the payload scan above found nothing -- keeps the common
# case (match already found) cheap and avoids reading files unnecessarily. Files above the size cap
# are skipped silently.
scan_file_for_pattern() {
  local path size
  path=$(printf '%s' "$1" | tr -d "\"'" | sed -E 's/[},]+$//')
  [ -n "$path" ] && [ -f "$path" ] || return 1
  size=$(wc -c < "$path" 2>/dev/null | tr -d '[:space:]')
  size=${size:-0}
  [ "$size" -lt 1048576 ] || return 1
  if grep -qE "$JWT_PATTERN" "$path" 2>/dev/null; then
    MATCH="JWT"
    return 0
  fi
  if grep -qE "$AWS_KEY_PATTERN" "$path" 2>/dev/null; then
    MATCH="AWS access key"
    return 0
  fi
  return 1
}

if [ -z "$MATCH" ] && [ -n "$REDIRECTS" ]; then
  while IFS= read -r line; do
    if [ -z "$line" ]; then
      continue
    fi
    target=$(printf '%s' "$line" | sed -E 's/^[0-9]?>>?[[:space:]]*//')
    if ! is_ephemeral_target "$target"; then
      scan_file_for_pattern "$target" && break
    fi
  done <<< "$REDIRECTS"
fi

if [ -z "$MATCH" ]; then
  CMD_LINE=$(printf '%s' "$RAW" | sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
  if [ -n "$CMD_LINE" ]; then
    set -- $CMD_LINE
    cmd_name="${1:-}"
    case "$cmd_name" in
      cat|head|tail|jq|grep)
        shift
        for token in "$@"; do
          scan_file_for_pattern "$token" && break
        done
        ;;
    esac
  fi
fi

[ -n "$MATCH" ] || exit 0

HAS_REDIRECT=0
EXEMPT=1
if [ -n "$REDIRECTS" ]; then
  while IFS= read -r line; do
    if [ -z "$line" ]; then
      continue
    fi
    HAS_REDIRECT=1
    target=$(printf '%s' "$line" | sed -E 's/^[0-9]?>>?[[:space:]]*//')
    if ! is_ephemeral_target "$target"; then
      EXEMPT=0
    fi
  done <<< "$REDIRECTS"
fi

if [ "$HAS_REDIRECT" -eq 1 ] && [ "$EXEMPT" -eq 1 ]; then
  exit 0
fi

`

// credentialGuardModeResolution lê `credential_guard.mode` de trackfw.yaml (grep simples, sem
// parser YAML completo) e resolve a variável MODE para "warn" ou "block". Compartilhada entre
// credentialGuardProjectTail e credentialGuardGlobalTail — não duplicar a linha de grep em dois
// lugares (ML-1A, ADR-2026-08-06 emenda 6). $DEFAULT_MODE deve estar definida antes deste bloco:
// a variante de projeto define "warn" (comportamento inalterado — já protegida pelo guard
// `[ -f trackfw.yaml ] || exit 0` de credentialGuardProjectGuardBlock, que garante trackfw.yaml
// existir sempre que este bloco roda); a variante global define "block" (o fallback deixa de ser
// "warn" quando não há trackfw.yaml no cwd, ou trackfw.yaml sem a chave credential_guard.mode — um
// guard opt-in que nunca bloqueia por padrão é uma falsa sensação de proteção). Quando
// trackfw.yaml existe com credential_guard.mode explícito (warn ou block), esse valor é respeitado
// em ambas as variantes.
const credentialGuardModeResolution = `MODE=$(grep -A 5 '^credential_guard:' trackfw.yaml 2>/dev/null | grep 'mode:' | head -1 | sed -E 's/^[[:space:]]*mode:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d "\"'" || true)
case "$MODE" in
  warn|block) ;;
  *) MODE="$DEFAULT_MODE" ;;
esac

`

// credentialGuardProjectTail resolve MODE/ROADMAP_DIR a partir de trackfw.yaml (escopo de projeto)
// e grava o attention signal em $ROADMAP_DIR/.trackfw-credential-guard.json. Fallback de MODE:
// "warn" (ver credentialGuardModeResolution).
const credentialGuardProjectTail = `DEFAULT_MODE="warn"
` + credentialGuardModeResolution + `if [ "$MODE" = "block" ]; then
  echo "trackfw-credential-guard: blocked - possible $MATCH detected in tool payload." >&2
  exit 2
fi

echo "trackfw-credential-guard: warning - possible $MATCH detected in tool payload." >&2

# tr -d '\r': Windows CRLF in trackfw.yaml corrupts the path; normalise (not reject).
# DO NOT use sed $'s/\r//g' -- fails on Git Bash (GNU sed 4.9): "no previous regular expression".
ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml 2>/dev/null | head -1 | sed 's/^roadmap_dir:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d '"' | tr -d "'" | tr -d '\r' || true)
ROADMAP_DIR=${ROADMAP_DIR:-docs/roadmaps}

case "$ROADMAP_DIR" in
  /*|../*|*/../*|*/..|..) ROADMAP_DIR="docs/roadmaps" ;;
esac

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
MSG="Possible $MATCH detected in tool payload - review before materializing credentials in plain text."
MSG_ESC=$(echo "$MSG" | tr -d '\000-\037' | sed 's/\\/\\\\/g; s/"/\\"/g')

mkdir -p "$ROADMAP_DIR"
printf '{"tool":"credential-guard","message":"%s","level":"action_required","timestamp":"%s"}\n' \
  "$MSG_ESC" \
  "$TIMESTAMP" > "$ROADMAP_DIR/.trackfw-credential-guard.json"

exit 0
`

// credentialGuardGlobalTail é a contraparte de credentialGuardProjectTail para o escopo global
// (~/.trackfw/scripts/trackfw-credential-guard.sh, instalado via `trackfw update harness`).
//
// Decisão (ML-1A, ver ADR-2026-08-06 emenda 6 de 2026-08-08 e ROADMAP-2026-08-08, Wave 1): o modo
// em escopo global reusa a MESMA leitura de `credential_guard.mode` de trackfw.yaml que
// credentialGuardProjectTail já faz (credentialGuardModeResolution) — sem exigir trackfw.yaml
// existir (não há o guard `[ -f trackfw.yaml ] || exit 0` da variante de projeto: o objetivo do
// escopo global é proteger qualquer projeto, com ou sem trackfw.yaml). Quando o hook global roda a
// partir do cwd de um projeto com trackfw.yaml e credential_guard.mode explícito, esse valor é
// respeitado (warn ou block) — nenhuma mudança de comportamento para quem já definiu mode: warn
// explicitamente. Em qualquer outro caso (sem trackfw.yaml, ou trackfw.yaml sem essa chave), o
// fallback deixa de ser "warn" e passa a ser "block": um guard opt-in que nunca bloqueia por padrão
// é uma falsa sensação de proteção — o usuário que rodou `trackfw update harness` já demonstrou
// intenção explícita de ter o mecanismo ativo. Superseded a decisão original ("modo global sempre
// warn", opção "b" avaliada na ADR original) — não cria `~/.trackfw/config.yaml` nem nenhuma outra
// segunda fonte de configuração só para isto.
//
// ROADMAP_DIR em escopo global: como não há garantia de trackfw.yaml para ler `roadmap_dir:`, o
// script usa o caminho padrão fixo "docs/roadmaps" relativo ao cwd de onde o hook foi disparado, e
// só grava o attention signal se esse diretório já existir (e só em modo warn — modo block nunca
// grava o attention signal, mesma decisão da variante de projeto). Não cria "docs/roadmaps" em um
// projeto aleatório só para sinalizar isso — isso pareceria ao usuário que o trackfw foi
// "instalado" nesse projeto, o que não é verdade. O texto de warning/block em stderr acontece
// sempre (visível no output do CLI/hook), independente de o diretório de attention existir.
const credentialGuardGlobalTail = `DEFAULT_MODE="block"
` + credentialGuardModeResolution + `if [ "$MODE" = "block" ]; then
  echo "trackfw-credential-guard: blocked - possible $MATCH detected in tool payload." >&2
  exit 2
fi

echo "trackfw-credential-guard: warning - possible $MATCH detected in tool payload." >&2

ROADMAP_DIR="docs/roadmaps"
if [ ! -d "$ROADMAP_DIR" ]; then
  exit 0
fi

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
MSG="Possible $MATCH detected in tool payload - review before materializing credentials in plain text."
MSG_ESC=$(echo "$MSG" | tr -d '\000-\037' | sed 's/\\/\\\\/g; s/"/\\"/g')

mkdir -p "$ROADMAP_DIR"
printf '{"tool":"credential-guard","message":"%s","level":"action_required","timestamp":"%s"}\n' \
  "$MSG_ESC" \
  "$TIMESTAMP" > "$ROADMAP_DIR/.trackfw-credential-guard.json"

exit 0
`

// credentialGuardScript é o conteúdo canônico do shim de escopo de projeto (ML-2A,
// ADR-2026-10-04). A lógica de detecção migrou para `trackfw guard credential` (Go). O shim
// sonda o binário e executa o subcomando. Fail-closed (exit 2) se o binário não for encontrado
// ou não tiver o subcomando guard — o invólucro não deve silenciar falhas. A guarda de projeto
// (`[ -f trackfw.yaml ] || exit 0`) é mantida: o escopo de projeto deve ser no-op fora da
// raiz do projeto trackfw, antes da sonda.
const credentialGuardScript = `#!/usr/bin/env bash
# trackfw-credential-guard.sh — thin wrapper; delegates to ` + "`" + `trackfw guard credential` + "`" + `.
# Script is intentionally a no-op when executed outside the project root
set -euo pipefail
[ -f trackfw.yaml ] || exit 0
if ! command -v trackfw >/dev/null 2>&1; then
  echo "trackfw-credential-guard: trackfw not found in PATH — install trackfw to enable this guard." >&2
  exit 2
fi
if ! trackfw guard --help >/dev/null 2>&1; then
  echo "trackfw-credential-guard: trackfw guard subcommand unavailable — upgrade trackfw to enable this guard." >&2
  exit 2
fi
exec trackfw guard credential
`

// globalCredentialGuardScript é o conteúdo canônico do shim de escopo global
// (~/.trackfw/scripts/trackfw-credential-guard.sh, instalado via `trackfw update harness`).
// Sem a guarda de projeto (`[ -f trackfw.yaml ]`): o escopo global protege qualquer projeto,
// com ou sem trackfw.yaml — a lógica de no-op cross-project é responsabilidade do Go binary.
// Fail-closed (exit 2) se o binário não for encontrado ou não tiver o subcomando guard.
const globalCredentialGuardScript = `#!/usr/bin/env bash
# trackfw-credential-guard.sh — thin wrapper; delegates to ` + "`" + `trackfw guard credential --global` + "`" + `.
set -euo pipefail
if ! command -v trackfw >/dev/null 2>&1; then
  echo "trackfw-credential-guard: trackfw not found in PATH — install trackfw to enable this guard." >&2
  exit 2
fi
if ! trackfw guard --help >/dev/null 2>&1; then
  echo "trackfw-credential-guard: trackfw guard subcommand unavailable — upgrade trackfw to enable this guard." >&2
  exit 2
fi
exec trackfw guard credential --global
`

// GenerateGitBranchGuardScript gera o script shell trackfw-git-branch-guard.sh em
// <rootDir>/scripts. Se rootDir for "", usa o diretório de trabalho atual. Este ML (1A) só cria o
// script canônico (referência Go) — não o injeta em nenhum hooks.json/settings.json de CLI (isso é
// escopo da Wave 3, ver ROADMAP-2026-08-14-bloqueio-tecnico-de-comandos-git-brutos-por-subagente-
// via-deny-hooks-nos-7-runtimes-suportados.md).
//
// Mesmo padrão de GenerateCredentialGuardScript: MkdirAll + WriteFile 0755.
func GenerateGitBranchGuardScript(rootDir string) error {
	root, err := scaffoldRoot(rootDir)
	if err != nil {
		return fmt.Errorf("GenerateGitBranchGuardScript: %w", err)
	}
	rootDir = root
	scriptsDir := filepath.Join(rootDir, "scripts")
	if err := rejectScaffoldPath(root, scriptsDir); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return err
	}

	path := filepath.Join(scriptsDir, "trackfw-git-branch-guard.sh")
	// Guard the leaf file — directory guard above does not detect a symlinked script file.
	if err := rejectScaffoldPath(root, path); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectScaffoldPath above
	if err := os.WriteFile(path, []byte(gitBranchGuardScript), 0755); err != nil {
		return fmt.Errorf("writing git branch guard script: %w", err)
	}
	// AC9: restore execute bit unconditionally.
	if err := os.Chmod(path, 0755); err != nil {
		return fmt.Errorf("setting execute bit on git branch guard script: %w", err)
	}
	fmt.Printf("  ✓ %s\n", filepath.Join("scripts", "trackfw-git-branch-guard.sh"))

	return nil
}

// GenerateGlobalGitBranchGuardScript gera o script shell trackfw-git-branch-guard.sh em escopo
// global, em <home>/.trackfw/scripts/trackfw-git-branch-guard.sh. Destinado a ser referenciado por
// hooks globais de CLI (~/.claude/settings.json, ~/.gemini/settings.json etc.), instalados via
// `trackfw update harness` (mesmo padrão de GenerateGlobalCredentialGuardScript) — não é chamado
// por `trackfw init`/`trackfw update` (escopo de projeto), que continuam usando
// GenerateGitBranchGuardScript.
//
// O conteúdo do script é idêntico entre escopo de projeto e global — ao contrário do
// credential-guard, o git branch guard não depende de trackfw.yaml (nenhuma leitura de
// credential_guard.mode/roadmap_dir): a detecção de `git commit`/`git push`/`git checkout -b`
// bruto e a mensagem de bloqueio são as mesmas em qualquer diretório. As duas funções existem
// separadamente (em vez de uma única com destino parametrizado) só para espelhar exatamente o
// par Generate*/GenerateGlobal* já estabelecido pelo credential-guard — consistência de padrão
// entre os dois guards, não uma necessidade técnica deste guard específico.
//
// Escreve silenciosamente (sem fmt.Printf), mesmo racional de GenerateGlobalCredentialGuardScript:
// evita vazar texto solto para o stdout de comandos que possam rodar com --json.
func GenerateGlobalGitBranchGuardScript(home string) error {
	if home == "" {
		return fmt.Errorf("home directory vazio")
	}
	// ML-8A / #402: see GenerateGlobalSkill — one assignment from one resolver.
	absHome, err := pathguard.ResolveRoot(home)
	if err != nil {
		return fmt.Errorf("resolving home: %w", err)
	}
	scriptsDir := filepath.Join(absHome, ".trackfw", "scripts")
	if err := rejectScaffoldPath(absHome, scriptsDir); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return err
	}

	path := filepath.Join(scriptsDir, "trackfw-git-branch-guard.sh")
	// Guard the leaf file — directory guard above does not detect a symlinked script file.
	if err := rejectScaffoldPath(absHome, path); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectScaffoldPath above
	if err := os.WriteFile(path, []byte(gitBranchGuardScript), 0755); err != nil {
		return fmt.Errorf("writing global git branch guard script: %w", err)
	}

	return nil
}

// gitBranchGuardScript é o conteúdo canônico do shim de escopo de projeto/global do git-branch
// guard (ML-2A, ADR-2026-10-04). A lógica de bloqueio (suporte a 3 formatos de entrada, decisão
// de saída, no-op fora de projeto trackfw, etc.) migrou para `trackfw guard git-branch` (Go). O
// shim sonda o binário e executa o subcomando. Fail-closed (exit 2) se o binário não for
// encontrado ou não tiver o subcomando guard.
const gitBranchGuardScript = `#!/usr/bin/env bash
# trackfw-git-branch-guard.sh — thin wrapper; delegates to ` + "`" + `trackfw guard git-branch` + "`" + `.
set -euo pipefail
if ! command -v trackfw >/dev/null 2>&1; then
  echo "trackfw-git-branch-guard: trackfw not found in PATH — install trackfw to enable this guard." >&2
  exit 2
fi
if ! trackfw guard --help >/dev/null 2>&1; then
  echo "trackfw-git-branch-guard: trackfw guard subcommand unavailable — upgrade trackfw to enable this guard." >&2
  exit 2
fi
if [ $# -gt 0 ]; then exec trackfw guard git-branch --command "$*"; fi
exec trackfw guard git-branch
`

func buildValidateScript(cfg Config) string {
	base := `#!/usr/bin/env sh
# trackfw governance gate — generated by trackfw init
set -e

echo "→ trackfw: validating governance..."
trackfw validate

`
	switch cfg.Backend {
	case "go":
		base += "echo \"→ build check (go)...\"\ngo build ./...\n"
	case "java":
		base += "echo \"→ build check (maven)...\"\nmvn compile -q\n"
	case "node":
		base += fmt.Sprintf("echo \"→ build check (node)...\"\n%s run build\n", cfg.PkgManager)
	case "python":
		base += "echo \"→ build check (python)...\"\npython3 -c \"import pathlib, py_compile; [py_compile.compile(str(p), doraise=True) for p in pathlib.Path('.').rglob('*.py') if '.venv' not in p.parts and 'venv' not in p.parts]\"\n"
	}

	switch cfg.Frontend {
	case "react", "vue", "angular":
		pm := cfg.PkgManager
		if pm == "none" {
			pm = "npm"
		}
		base += fmt.Sprintf("echo \"→ frontend build check...\"\n%s run build\n", pm)
	}

	base += "\necho \"✓ all checks passed.\"\n"
	return base
}

func generateCIWorkflow(cfg Config) error {
	switch cfg.CI {
	case "github-actions":
		return generateGitHubActionsWorkflow(cfg)
	case "gitlab-ci":
		return generateGitLabCIWorkflow(cfg)
	}
	return nil
}

// GitHubActionsWorkflowPath is the canonical relative path of the GitHub Actions
// workflow written by trackfw init/update. Used by scaffold doctor (ADR-2026-08-27)
// to identify this artifact by path without reading the manifest.
const GitHubActionsWorkflowPath = ".github/workflows/trackfw-gate.yml"

// GitLabCIWorkflowPath is the canonical relative path of the GitLab CI file written
// when ci: gitlab-ci is configured. Used by scaffold doctor (ADR-2026-08-27).
const GitLabCIWorkflowPath = ".gitlab-ci-trackfw.yml"

// buildGitHubActionsWorkflowContent returns the template content that trackfw writes
// to GitHubActionsWorkflowPath. isProducer is true when the repository IS the trackfw
// project itself (detected via IsProducerGoMod — go.mod declares
// "module github.com/kgsaran/trackfw"), false for all consumer projects.
//
// Producer branch: compiles from source (go build -o ... ./cmd/trackfw) so that the
// required check `governance-install-script` validates the PR's code, not the
// previously published binary. Does not emit TRACKFW_VERSION (meaningless when
// installing from source). Uses go-version-file: go.mod to track the declared toolchain.
//
// Consumer branch: installs via install.sh (unchanged semantics) and pins
// TRACKFW_VERSION to internal/version.Version (ADR-2026-08-28, REQ-2026-08-28
// AC6/AC7). Scaffold doctor compares disk content against this template so a project
// generated by a different binary version is reported as scaffold-divergent (AC10)
// while one generated by the current binary is not (AC11).
//
// Job id is `governance-install-script` (ML-1A, ROADMAP-2026-09-01): trackfw-gate.yml
// and the sibling workflow written by discover --init (trackfw-validate.yml, see
// BuildDiscoverGitHubActionsWorkflowContent in scaffold_doctor.go) both validate the
// exact same property (`trackfw validate` passes) but through two different install
// mechanisms — both ids are named after the install mechanism each exercises so a
// maintainer reading required_status_checks can tell what each covers without opening
// the YAML. Both branches of this function emit the same job id because the contract
// with required_status_checks is per-job-name, not per-install-mechanism.
func buildGitHubActionsWorkflowContent(isProducer bool) string {
	if isProducer {
		return `name: trackfw-gate
on:
  pull_request:
    branches: [main]

jobs:
  governance-install-script:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    steps:
      - uses: actions/checkout@v7

      - name: ML-1A (severity anchor) — fetch origin/main for trackfw validate
        # ROADMAP-2026-09-17-leniencia-sem-prazo, ML-1A.
        # Without this step, actions/checkout@v7 on pull_request events only fetches the PR's
        # merge commit — origin/main is never written as a local ref. trackfw validate would then
        # see originAnchorRefUnreadable and fail closed (all rules at built-in defaults, disk
        # rules: block ignored). This fetch makes origin/main available so the severity anchor
        # (stricter-of-origin/main-vs-disk) can work as designed.
        #
        # Refspec is config-independent (bare branch name writes only FETCH_HEAD).
        # Cost: 0.09 s measured (1 commit + tree, < 1 MB). fetch-depth: 0 rejected (too costly).
        # Precedent: quality.yml:545-570 (same refspec, same three-branch discrimination).
        #
        # Three-branch discrimination (two-states-one-observable prevention):
        #   Braço 1 — ref absent (fetch failed): ::error:: annotation; trackfw validate will emit
        #             severity-anchor violation (fail closed via Go code).
        #   Braço 2 — ref present, file absent: ::warning:: annotation; disk-only severities.
        #   Braço 3 — ref and file present: anchor active.
        #
        # This step always exits 0 so that trackfw validate always runs (and emits the fail-closed
        # violation if needed). The CI failure surface is trackfw validate, not this fetch step.
        run: |
          FETCH_EXIT=0
          git fetch --depth=1 --no-tags origin "+refs/heads/main:refs/remotes/origin/main" 2>&1 || FETCH_EXIT=$?
          if ! git rev-parse --verify origin/main > /dev/null 2>&1; then
            echo "::error::ML-1A (anchor): origin/main unavailable after fetch (fetch_exit=${FETCH_EXIT}) — trackfw validate will emit severity-anchor violation (fail closed)"
          elif [ -z "$(git ls-tree origin/main -- ./trackfw.yaml 2>/dev/null)" ]; then
            echo "::warning::ML-1A (anchor): trackfw.yaml absent in origin/main — rule severity resolved from disk only (expected for a PR that adds the file for the first time)"
          else
            echo "ML-1A (anchor): origin/main:./trackfw.yaml present — severity anchor active"
          fi

      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod

      - name: Build trackfw from source
        run: go build -o /usr/local/bin/trackfw ./cmd/trackfw

      - name: Governance gate
        run: trackfw validate
`
	}
	return `name: trackfw-gate
on:
  pull_request:
    branches: [main]

jobs:
  governance-install-script:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    env:
      TRACKFW_VERSION: "` + version.Version + `"
    steps:
      - uses: actions/checkout@v7

      - name: Install trackfw
        run: |
          curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh

      - name: Governance gate
        run: trackfw validate
`
}

// buildGitLabCIWorkflowContent returns the template content that trackfw writes to
// GitLabCIWorkflowPath. Cfg-independent; ci: gitlab-ci is the gate at the call site.
// NOT version-independent: pins TRACKFW_VERSION to internal/version.Version — see the
// doc comment on buildGitHubActionsWorkflowContent above for the rationale (AC6/AC7,
// AC10, AC11, AC12).
func buildGitLabCIWorkflowContent(_ Config) string {
	return `# trackfw governance gate
trackfw-gate:
  stage: test
  image: alpine:latest
  timeout: 10 minutes
  variables:
    TRACKFW_VERSION: "` + version.Version + `"
  before_script:
    - apk add --no-cache curl
    - curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
  script:
    - trackfw validate
  only:
    - merge_requests
`
}

func generateGitHubActionsWorkflow(cfg Config) error {
	ghRoot, ghErr := projectRoot()
	if ghErr != nil {
		return fmt.Errorf("generateGitHubActionsWorkflow: %w", ghErr)
	}
	absGHDir := filepath.Join(ghRoot, ".github", "workflows")
	if err := rejectScaffoldPath(ghRoot, absGHDir); err != nil {
		return err
	}

	// D2 (ADR-2026-09-29): the product delivers ONE governance workflow per project.
	// If the discover-installed workflow (DiscoverGitHubActionsWorkflowPath,
	// trackfw-validate.yml) already exists as a regular file, skip writing
	// trackfw-gate.yml. This mirrors the cuidado that
	// refreshDiscoverGitHubActionsWorkflowIfPresent (update.go) already has on
	// the other side — the asymmetry was the defect.
	// Uses discoverWorkflowPresent (os.Lstat) so a symlink at that path is NOT
	// treated as present and gate.yml is still written — safe direction.
	if discoverWorkflowPresent(ghRoot) {
		fmt.Printf("  ℹ %s já existe — %s não será escrito (ADR-2026-09-29 D2)\n",
			DiscoverGitHubActionsWorkflowPath, GitHubActionsWorkflowPath)
		return nil
	}

	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.MkdirAll(".github/workflows", 0755); err != nil {
		return err
	}

	path := GitHubActionsWorkflowPath
	// Guard the leaf file — directory guard above does not detect a symlinked workflow file.
	absWorkflowPath := filepath.Join(ghRoot, path)
	if err := rejectScaffoldPath(ghRoot, absWorkflowPath); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectScaffoldPath above
	if err := os.WriteFile(path, []byte(buildGitHubActionsWorkflowContent(IsProducerGoMod("."))), 0644); err != nil {
		return fmt.Errorf("writing CI workflow: %w", err)
	}
	fmt.Printf("  ✓ %s\n", path)
	return nil
}

func generateGitLabCIWorkflow(cfg Config) error {
	glRoot, glErr := projectRoot()
	if glErr != nil {
		return fmt.Errorf("generateGitLabCIWorkflow: %w", glErr)
	}
	absGLCI := filepath.Join(glRoot, GitLabCIWorkflowPath)
	if err := rejectScaffoldPath(glRoot, absGLCI); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.WriteFile(GitLabCIWorkflowPath, []byte(buildGitLabCIWorkflowContent(cfg)), 0644); err != nil {
		return fmt.Errorf("writing GitLab CI: %w", err)
	}
	fmt.Println("  ✓ .gitlab-ci-trackfw.yml")
	return nil
}

func generateCommitMsgHook(cfg Config) error {
	if !cfg.RequireReqInCommit {
		return nil
	}

	cmhRoot, cmhErr := projectRoot()
	if cmhErr != nil {
		return fmt.Errorf("generateCommitMsgHook: %w", cmhErr)
	}
	// Guard the hook directory for the configured hook manager.
	switch cfg.Hooks {
	case "husky":
		if err := rejectScaffoldPath(cmhRoot, filepath.Join(cmhRoot, ".husky")); err != nil {
			return err
		}
	case "lefthook":
		if err := rejectScaffoldPath(cmhRoot, filepath.Join(cmhRoot, ".lefthook", "commit-msg")); err != nil {
			return err
		}
	}

	script := "#!/bin/sh\n" +
		"# trackfw: require REQ reference in feat/* and fix/* branches\n" +
		"BRANCH=$(git symbolic-ref --short HEAD 2>/dev/null || echo \"\")\n" +
		"case \"$BRANCH\" in\n" +
		"  feat/*|fix/*)\n" +
		"    if ! grep -qE \"^(REQ|req): \" \"$1\"; then\n" +
		"      echo \"ERROR: Commits in feat/* and fix/* branches require a REQ reference.\"\n" +
		"      echo \"  Add to commit body: REQ: REQ-YYYY-MM-DD-your-req-slug\"\n" +
		"      exit 1\n" +
		"    fi\n" +
		"    ;;\n" +
		"esac\n"

	switch cfg.Hooks {
	case "husky":
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if err := os.MkdirAll(".husky", 0755); err != nil {
			return fmt.Errorf("creating .husky: %w", err)
		}
		path := ".husky/commit-msg"
		// Guard the leaf file — directory guard above does not detect a symlinked hook file.
		if err := rejectScaffoldPath(cmhRoot, filepath.Join(cmhRoot, path)); err != nil {
			return err
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectScaffoldPath above
		if err := os.WriteFile(path, []byte(script), 0755); err != nil {
			return fmt.Errorf("writing husky commit-msg hook: %w", err)
		}
		fmt.Printf("  ✓ %s\n", path)
	case "lefthook":
		lefthookPath := "lefthook.yml"
		existing, _ := os.ReadFile(lefthookPath)
		if !strings.Contains(string(existing), "commit-msg:") {
			addition := "\ncommit-msg:\n  scripts:\n    \"trackfw-req-check.sh\":\n      runner: sh\n"
			// Guard lefthook.yml — distinct from the .lefthook/commit-msg directory guard above;
			// that guard never walks lefthook.yml (it is at root level, outside the guarded dir).
			if err := rejectScaffoldPath(cmhRoot, filepath.Join(cmhRoot, lefthookPath)); err != nil {
				return err
			}
			// write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectScaffoldPath above
			if err := os.WriteFile(lefthookPath, append(existing, []byte(addition)...), 0644); err != nil {
				return fmt.Errorf("writing lefthook.yml commit-msg section: %w", err)
			}
		}
		scriptDir := ".lefthook/commit-msg"
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if err := os.MkdirAll(scriptDir, 0755); err != nil {
			return fmt.Errorf("creating %s: %w", scriptDir, err)
		}
		scriptPath := scriptDir + "/trackfw-req-check.sh"
		// Guard the leaf script file — directory guard does not detect a symlinked script file.
		if err := rejectScaffoldPath(cmhRoot, filepath.Join(cmhRoot, scriptPath)); err != nil {
			return err
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectScaffoldPath above
		if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
			return fmt.Errorf("writing lefthook commit-msg script: %w", err)
		}
		fmt.Printf("  ✓ %s\n", scriptPath)
	}
	return nil
}

func generateGitHooks(cfg Config) error {
	switch cfg.Hooks {
	case "husky":
		return generateHuskyHook()
	case "lefthook":
		return generateLefthookHook()
	}
	return nil
}

func generateHuskyHook() error {
	hhRoot, hhErr := projectRoot()
	if hhErr != nil {
		return fmt.Errorf("generateHuskyHook: %w", hhErr)
	}
	absHusky := filepath.Join(hhRoot, ".husky")
	if err := rejectScaffoldPath(hhRoot, absHusky); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.MkdirAll(".husky", 0755); err != nil {
		return err
	}
	content := "#!/usr/bin/env sh\n. \"$(dirname -- \"$0\")/_/husky.sh\"\n\ntrackfw validate\n"
	path := ".husky/pre-commit"
	// Guard the leaf file — directory guard above does not detect a symlinked hook file.
	if err := rejectScaffoldPath(hhRoot, filepath.Join(hhRoot, path)); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectScaffoldPath above
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		return fmt.Errorf("writing husky hook: %w", err)
	}
	fmt.Printf("  ✓ %s\n", path)
	return nil
}

// generateVaultIndex cria vault/notes/index.md se ainda não existir.
// O arquivo é o ponto de entrada do vault de conhecimento do projeto.
func generateVaultIndex() error {
	viRoot, viErr := projectRoot()
	if viErr != nil {
		return fmt.Errorf("generateVaultIndex: %w", viErr)
	}
	absVaultDir := filepath.Join(viRoot, "vault", "notes")
	if err := rejectScaffoldPath(viRoot, absVaultDir); err != nil {
		return err
	}
	indexPath := filepath.Join("vault", "notes", "index.md")
	if _, err := os.Stat(indexPath); err == nil {
		// já existe — idempotente
		return nil
	}
	content := `# Vault de Conhecimento

> Ponto de entrada de conhecimento do projeto para agentes e pessoas.
> Cada nota documenta uma causa-raiz, decisão técnica ou restrição não óbvia.
> Crie notas com: trackfw note new "<título>"

## Índice

<!-- As notas serão listadas abaixo. Exemplo:
- [nome-da-nota-YYYY-MM-DD](nome-da-nota-YYYY-MM-DD.md)
-->
`
	// Guard the leaf file — directory guard above does not detect a symlinked index file.
	absIndexPath := filepath.Join(viRoot, indexPath)
	if err := rejectScaffoldPath(viRoot, absIndexPath); err != nil {
		return err
	}
	// write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectScaffoldPath above
	if err := os.WriteFile(indexPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing vault/notes/index.md: %w", err)
	}
	fmt.Println("  ✓ vault/notes/index.md")
	return nil
}

// gitAttributesRuleTarget é o primeiro campo da regra que este gerador mantém.
// A regra casa o BASENAME (padrão sem barra) de propósito: `roadmap_dir` e
// `req_dir` são configuráveis por projeto (trackfw.yaml) e ambos carregam um
// `.trackfw-log` (roadmap.go logPath / req.go appendREQTransitionLog), então um
// caminho fixo nasceria quebrado em quem configurou diretório diferente e
// deixaria o log do req_dir descoberto.
const gitAttributesRuleTarget = ".trackfw-log"

// gitAttributesBlock é o bloco emitido no caminho de criação E no de append.
// Byte-idêntico ao equivalente em npm/src/generators/init.js e
// pypi/trackfw/generators/init_gen.py (regra dura de paridade) e ao
// .gitattributes da raiz deste repositório.
const gitAttributesBlock = `# trackfw: .trackfw-log is append-only — every write lands on the last line, so
# two parallel branches conflict on every merge. merge=union keeps the lines
# from both sides (chronological order is not guaranteed). The pattern has no
# slash, so it matches the file in any directory — roadmap_dir and req_dir both
# carry one, and both are configurable per project.
.trackfw-log merge=union
`

// hasGitAttributesRule decide idempotência: verdadeiro quando ALGUMA linha
// não-comentário tem `.trackfw-log` como primeiro campo delimitado por espaço.
// Predicado deliberadamente sobre o CAMPO, não sobre a string literal da linha
// inteira: `.trackfw-log  merge=union` (dois espaços) ou uma regra manual com
// outro atributo já são "a regra existe" — reescrever por cima seria sobrescrever
// decisão do projeto.
func hasGitAttributesRule(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.Fields(trimmed)[0] == gitAttributesRuleTarget {
			return true
		}
	}
	return false
}

// generateGitAttributes garante a regra `merge=union` para o `.trackfw-log` no
// `.gitattributes` da raiz do projeto. Três ramos, todos idempotentes:
//   - arquivo ausente  → cria com o bloco
//   - existe sem regra → APPEND do bloco (nunca sobrescreve o arquivo do projeto)
//   - existe com regra → no-op
func generateGitAttributes() error {
	gaRoot, gaErr := projectRoot()
	if gaErr != nil {
		return fmt.Errorf("generateGitAttributes: %w", gaErr)
	}
	absGitAttr := filepath.Join(gaRoot, ".gitattributes")
	if err := rejectScaffoldPath(gaRoot, absGitAttr); err != nil {
		return err
	}
	const path = ".gitattributes"
	existing, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("reading .gitattributes: %w", err)
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if err := os.WriteFile(path, []byte(gitAttributesBlock), 0644); err != nil {
			return fmt.Errorf("writing .gitattributes: %w", err)
		}
		fmt.Println("  ✓ .gitattributes")
		return nil
	}

	if hasGitAttributesRule(string(existing)) {
		return nil
	}

	// Arquivo preexistente sem newline final: emendar o bloco direto grudaria a
	// primeira linha do bloco na última linha do projeto, corrompendo a config
	// dele em silêncio.
	out := string(existing)
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	out += gitAttributesBlock
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.WriteFile(path, []byte(out), 0644); err != nil {
		return fmt.Errorf("appending to .gitattributes: %w", err)
	}
	fmt.Println("  ✓ .gitattributes")
	return nil
}

// gitIgnoreRuleTarget é o BASENAME do arquivo que este gerador garante ignorado.
// O predicado de idempotência casa o basename (e não o caminho literal emitido)
// porque `roadmap_dir` é configurável: um projeto que reconfigure o diretório e
// rode `init` de novo receberia uma segunda linha para o MESMO arquivo se o
// reconhecimento fosse por string exata. Mesma lição do hasGitAttributesRule.
const gitIgnoreRuleTarget = validator.BranchLinkFileName

// effectiveRoadmapDir devolve o `roadmap_dir` do trackfw.yaml do diretório
// corrente, ou o default quando o arquivo está ausente/ilegível.
//
// 🔴 Deliberadamente NÃO usa config.Load(): Load é singleton `once.Do` e cacheia
// o primeiro trackfw.yaml lido no processo — num gerador chamado após chdir (e nos
// testes, que rodam vários cwd no mesmo binário) isso devolveria a config de outro
// projeto. config.ParseDirsFromContent é o mesmo parser sem o cache, e já tem o
// contrato de "chave ausente cai no default".
func effectiveRoadmapDir() string {
	data, err := os.ReadFile("trackfw.yaml")
	if err != nil {
		_, roadmapDir, _ := config.ParseDirsFromContent("")
		return roadmapDir
	}
	_, roadmapDir, _ := config.ParseDirsFromContent(string(data))
	return roadmapDir
}

// gitIgnoreBlock é o bloco emitido no caminho de criação E no de append, com o
// `roadmap_dir` EFETIVO do projeto — nunca `docs/roadmaps` fixo (o produto
// presumir o layout do mantenedor é a família de defeito do #396).
func gitIgnoreBlock(roadmapDir string) string {
	return fmt.Sprintf(`# trackfw: branch<->roadmap link written by `+"`trackfw branch new`"+` — PER-CHECKOUT state,
# not versioned. A clone, a fork and CI never have it by design: inference by branch
# name is the fallback for that population. Committing it would let the link recorded
# on one machine govern another, possibly on a different branch.
%s/%s
`, strings.TrimSuffix(roadmapDir, "/"), gitIgnoreRuleTarget)
}

// hasGitIgnoreRule decide idempotência: verdadeiro quando ALGUMA linha
// não-comentário tem um padrão cujo basename é o arquivo de vínculo — qualquer
// caminho, e inclusive a forma negada (`!...`), que é decisão explícita do projeto
// e não deve ser contrariada por append.
func hasGitIgnoreRule(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		pattern := strings.TrimSuffix(strings.TrimPrefix(trimmed, "!"), "/")
		if path.Base(pattern) == gitIgnoreRuleTarget {
			return true
		}
	}
	return false
}

// generateGitIgnore garante que `<roadmap_dir>/.trackfw-branch-links.json` esteja
// ignorado no `.gitignore` da raiz do projeto. Três ramos, todos idempotentes —
// mesmo desenho de generateGitAttributes:
//   - arquivo ausente  → cria com o bloco
//   - existe sem regra → APPEND do bloco (nunca sobrescreve o arquivo do projeto)
//   - existe com regra → no-op
func generateGitIgnore() error {
	giRoot, giErr := projectRoot()
	if giErr != nil {
		return fmt.Errorf("generateGitIgnore: %w", giErr)
	}
	absGitIgnore := filepath.Join(giRoot, ".gitignore")
	if err := rejectScaffoldPath(giRoot, absGitIgnore); err != nil {
		return err
	}
	const gitIgnorePath = ".gitignore"
	block := gitIgnoreBlock(effectiveRoadmapDir())
	existing, err := os.ReadFile(gitIgnorePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("reading .gitignore: %w", err)
		}
		// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
		if err := os.WriteFile(gitIgnorePath, []byte(block), 0644); err != nil {
			return fmt.Errorf("writing .gitignore: %w", err)
		}
		fmt.Println("  ✓ .gitignore")
		return nil
	}

	if hasGitIgnoreRule(string(existing)) {
		return nil
	}

	// Arquivo preexistente sem newline final: emendar o bloco direto grudaria a
	// primeira linha do bloco no último padrão do projeto, transformando os dois
	// numa linha só e corrompendo o .gitignore dele em silêncio.
	out := string(existing)
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	out += block
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.WriteFile(gitIgnorePath, []byte(out), 0644); err != nil {
		return fmt.Errorf("appending to .gitignore: %w", err)
	}
	fmt.Println("  ✓ .gitignore")
	return nil
}

// lefthookValidatePresent reports whether a trackfw-validate: entry already lives
// inside the top-level pre-commit: block of a lefthook.yml file.
//
// It uses the same section-tracking logic as the merge path in generateLefthookHook:
// iterate lines, track inPreCommit by watching column-0 non-comment keys, and look for
// "trackfw-validate:" only while inside that section. This avoids false positives from:
//   - the key appearing under a different top-level block (e.g. pre-push:)
//   - the key appearing inside a comment (# trackfw-validate:)
//
// Inline YAML comments (space + "#" sequence after the key) are stripped before
// comparison so that "trackfw-validate: # installed by trackfw" is treated as present.
// Caveat: if trackfw-validate: ever carried a quoted value containing " #" (e.g.
// trackfw-validate: "run # something"), this truncation would incorrectly strip part
// of the value. Since this key carries no quoted value in practice — only optional
// trailing comments — the truncation is safe. Declaring rather than hiding the edge
// case (mirrors the equivalent caveat in scripts/check-init-preserves-user-config.sh).
func lefthookValidatePresent(content string) bool {
	inPreCommit := false
	for _, line := range strings.Split(content, "\n") {
		// Track top-level sections (column-0, non-whitespace, non-comment).
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' && line[0] != '#' {
			inPreCommit = line == "pre-commit:"
		}
		// Strip inline YAML comment (" #" sequence) before comparing.
		// A leading '#' (full-line comment) is already excluded by the section-tracking
		// guard above. Here we only need to handle trailing inline comments.
		trimmed := strings.TrimSpace(line)
		if idx := strings.Index(trimmed, " #"); idx >= 0 {
			trimmed = strings.TrimSpace(trimmed[:idx])
		}
		if inPreCommit && trimmed == "trackfw-validate:" {
			return true
		}
	}
	return false
}

func generateLefthookHook() error {
	lhRoot, lhErr := projectRoot()
	if lhErr != nil {
		return fmt.Errorf("generateLefthookHook: %w", lhErr)
	}
	absLH := filepath.Join(lhRoot, "lefthook.yml")
	if err := rejectScaffoldPath(lhRoot, absLH); err != nil {
		return err
	}

	const validateEntry = "    trackfw-validate:\n      run: trackfw validate\n"
	const fullBlock = "pre-commit:\n  commands:\n" + validateEntry

	// Read before write — guard that preserves consumer-authored lefthook.yml content.
	existing, _ := os.ReadFile("lefthook.yml")
	existingStr := string(existing)

	// Idempotent: trackfw-validate already installed inside pre-commit: → no-op.
	// Uses the same section-tracking logic as the merge path below to avoid false
	// positives from the key appearing in comments or under a different top-level
	// block (e.g. pre-push:).
	if lefthookValidatePresent(existingStr) {
		return nil
	}

	var out string
	if !presentTopLevelKeys(existingStr)["pre-commit"] {
		// No pre-commit block yet: append the full block with a blank-line separator.
		out = existingStr
		if out != "" && !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		if out != "" {
			out += "\n"
		}
		out += fullBlock
	} else {
		// pre-commit block exists: insert trackfw-validate under its commands: entry.
		lines := strings.Split(existingStr, "\n")
		result := make([]string, 0, len(lines)+2)
		inPreCommit := false
		inserted := false
		for _, line := range lines {
			// Track top-level sections (column-0, non-comment).
			if len(line) > 0 && line[0] != ' ' && line[0] != '\t' && line[0] != '#' {
				inPreCommit = line == "pre-commit:"
			}
			result = append(result, line)
			if inPreCommit && !inserted && strings.TrimSpace(line) == "commands:" {
				result = append(result, "    trackfw-validate:")
				result = append(result, "      run: trackfw validate")
				inserted = true
			}
		}
		if !inserted {
			// pre-commit exists but has no commands: section — warn and skip to avoid
			// corrupting a file with an unknown layout.
			fmt.Println("  ⚠ lefthook.yml: pre-commit block has no 'commands:' section — trackfw-validate not added")
			return nil
		}
		out = strings.Join(result, "\n")
	}

	// Guarda real: leitura de lefthook.yml acima — o merge só acrescenta o que falta.
	// Não há marcador de isenção declarativa aqui: com a leitura detectável na mesma
	// função, a isenção seria redundante e mascararia a perda da guarda se o bloco
	// de leitura fosse extraído para um helper em refatores futuros.
	// Distinção de gates: write-containment-allowed pertence ao check-write-containment/pathguard
	// (contenção de escrita — este sítio). O marcador de preservação de config do consumidor
	// (check-init-preserves-user-config) NÃO deve estar aqui — sua presença enfraquecia aquele gate
	// porque a detecção real é via os.ReadFile, não via marcador declarativo (ML-1C/ML-1D).
	// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
	if err := os.WriteFile("lefthook.yml", []byte(out), 0644); err != nil {
		return fmt.Errorf("writing lefthook config: %w", err)
	}
	fmt.Println("  ✓ lefthook.yml")
	return nil
}
