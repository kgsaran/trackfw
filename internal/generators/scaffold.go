package generators

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	ProjectType      string // "fullstack" | "frontend" | "backend" | "governance"
	ProjectName      string
	Frontend         string
	Backend          string
	BackendFramework string
	PkgManager       string
	Hooks            string
	CI               string
}

var govDirs = []string{
	"docs/adr",
	"docs/req",
	"docs/roadmaps/backlog",
	"docs/roadmaps/wip",
	"docs/roadmaps/blocked",
	"docs/roadmaps/done",
	"docs/roadmaps/abandoned",
}

func Scaffold(cfg Config) error {
	for _, dir := range govDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
		fmt.Printf("  ✓ %s\n", dir)
	}

	if err := writeTrackfwConfig(cfg); err != nil {
		return err
	}

	if err := generateValidateScript(cfg); err != nil {
		return err
	}

	if err := generateCIWorkflow(cfg); err != nil {
		return err
	}

	if err := generateGitHooks(cfg); err != nil {
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

	return nil
}

// InstallSkills instala os slash commands no projeto atual e a skill global em ~/.claude/skills/trackfw/.
// Arquivos já existentes não são sobrescritos — idempotente.
func InstallSkills() error {
	if err := generateClaudeCommands(); err != nil {
		return err
	}
	return installGlobalSkill()
}

func installGlobalSkill() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("localizando home dir: %w", err)
	}

	skillDir := filepath.Join(home, ".claude", "skills", "trackfw")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return fmt.Errorf("creating %s: %w", skillDir, err)
	}

	skillPath := filepath.Join(skillDir, "SKILL.md")
	if _, err := os.Stat(skillPath); err == nil {
		fmt.Printf("  ✓ ~/.claude/skills/trackfw/SKILL.md (já existe — não sobrescrito)\n")
		return nil
	}

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
4. **Atualize o roadmap após cada ML.** Marque MLs concluídos como ` + "`✅ Concluído`" + ` antes de avançar.
5. **Execute ` + "`trackfw validate`" + ` antes de cada commit.** Zero violations obrigatório.
6. **ADRs antes de decisões arquiteturais.** Qualquer decisão técnica relevante deve ter um ADR (` + "`/trackfw:adr`" + `).

---

## Cadeia de governança

` + "```" + `
ADR         → registra decisões técnicas e arquiteturais
REQ         → especifica requisitos e critérios de aceite
ROADMAP     → detalha implementação em microlotes (MLs) por Waves
backlog     → roadmaps aguardando execução
wip         → roadmap em execução ativa (máximo 1)
blocked     → impedido por dependência ou decisão externa
done        → concluído e validado
abandoned   → descontinuado (exige motivo)
` + "```" + `

---

## Slash commands disponíveis

| Comando | Quando usar |
|---|---|
| ` + "`/trackfw:implement <req>`" + ` | **Início aqui** — orquestra o fluxo completo de implementação |
| ` + "`/trackfw:adr <título>`" + ` | Antes de qualquer decisão arquitetural |
| ` + "`/trackfw:req <título>`" + ` | Antes de qualquer implementação |
| ` + "`/trackfw:roadmap <req>`" + ` | Gera roadmap em microlotes a partir de uma REQ |
| ` + "`/trackfw:move <nome> <estado>`" + ` | Move roadmap entre estados manualmente |
| ` + "`/trackfw:validate`" + ` | Valida governança do projeto |
| ` + "`/trackfw:status`" + ` | Exibe o que está em execução |

---

## Estratégia de microlotes (ML)

Cada roadmap é dividido em **Waves** com **MLs paralelos**:

- MLs dentro da mesma Wave são **independentes** (arquivos distintos, sem conflito)
- Cada ML deve ser autocontido: arquivos exatos, ações exatas, critérios de aceite mensuráveis
- Avance para a próxima Wave somente após todos os MLs da Wave atual estarem ✅
- Protocolo por ML: implementar → validar (build + testes) → atualizar roadmap → commitar

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

	if err := os.WriteFile(skillPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing SKILL.md: %w", err)
	}
	fmt.Printf("  ✓ ~/.claude/skills/trackfw/SKILL.md\n")
	return nil
}

func generateClaudeCommands() error {
	dir := ".claude/commands/trackfw"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}

	commands := map[string]string{
		"adr.md": `Execute o seguinte comando bash: ` + "`trackfw adr new \"$ARGUMENTS\"`" + `

Se o comando falhar com ` + "`trackfw: command not found`" + ` ou similar, informe ao usuário:

` + "```" + `
trackfw não está instalado. Instale com uma das opções:

  curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
  npm install -g trackfw
  pip install trackfw
` + "```",

		"req.md": `Execute o seguinte comando bash: ` + "`trackfw req new \"$ARGUMENTS\"`" + `

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

Estados válidos: ` + "`backlog`, `wip`, `blocked`, `done`, `abandoned`" + `

Exemplo: ` + "`/trackfw:move meu-roadmap wip`" + `

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

   ` + "```markdown" + `
   # Roadmap: <título derivado da REQ>

   > Criado em: <YYYY-MM-DD> | Status: ⬜ Backlog

   ## Diagnóstico / Contexto
   <resumo do problema, motivação e escopo extraídos da REQ>

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
   ` + "```" + `

   **Princípios obrigatórios:**
   - MLs dentro da mesma Wave são **independentes** (arquivos distintos, sem conflito)
   - Cada ML deve ser detalhado o suficiente para execução por um agente sem contexto extra
   - Maximizar paralelismo: agrupe em paralelo tudo que não compartilhar arquivos
   - Waves sequenciais apenas quando há dependência real de resultado
   - Critérios de aceite mensuráveis em cada ML

4. **Salvar o arquivo**
   - Calcule o slug: título em lowercase, espaços → hifens, remova caracteres especiais
   - Crie o arquivo em ` + "`docs/roadmaps/backlog/ROADMAP-<YYYY-MM-DD>-<slug>.md`" + `
   - Use a data de hoje

5. **Confirmar**
   Informe o caminho do arquivo criado e um resumo das Waves e total de MLs gerados.`,

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

	created, skipped := 0, 0
	for filename, content := range commands {
		path := filepath.Join(dir, filename)
		if _, err := os.Stat(path); err == nil {
			skipped++
			continue
		}
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

func writeTrackfwConfig(cfg Config) error {
	content := fmt.Sprintf(`# trackfw configuration
# generated: %s

frontend: %s
backend: %s
backend_framework: %s
pkg_manager: %s
hooks: %s
ci: %s

# governance paths (edit to match your project structure)
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: flat
`, time.Now().Format("2006-01-02"), cfg.Frontend, cfg.Backend, cfg.BackendFramework, cfg.PkgManager, cfg.Hooks, cfg.CI)

	if err := os.WriteFile("trackfw.yaml", []byte(content), 0644); err != nil {
		return fmt.Errorf("writing trackfw.yaml: %w", err)
	}
	fmt.Println("  ✓ trackfw.yaml")
	return nil
}

func generateValidateScript(cfg Config) error {
	if err := os.MkdirAll("scripts", 0755); err != nil {
		return err
	}

	script := buildValidateScript(cfg)
	path := filepath.Join("scripts", "trackfw-validate.sh")
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		return fmt.Errorf("writing validate script: %w", err)
	}
	fmt.Printf("  ✓ %s\n", path)
	return nil
}

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
		base += "echo \"→ build check (python)...\"\npython -m py_compile $(find . -name '*.py' -not -path './.venv/*' -not -path './venv/*')\n"
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

func generateGitHubActionsWorkflow(cfg Config) error {
	if err := os.MkdirAll(".github/workflows", 0755); err != nil {
		return err
	}

	content := fmt.Sprintf(`name: trackfw-gate
on:
  pull_request:
    branches: [main]

jobs:
  governance:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install trackfw
        run: |
          curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh

      - name: Governance gate
        run: trackfw validate
`)
	_ = cfg

	path := ".github/workflows/trackfw-gate.yml"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing CI workflow: %w", err)
	}
	fmt.Printf("  ✓ %s\n", path)
	return nil
}

func generateGitLabCIWorkflow(cfg Config) error {
	content := `# trackfw governance gate
trackfw-gate:
  stage: test
  image: alpine:latest
  before_script:
    - apk add --no-cache curl
    - curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
  script:
    - trackfw validate
  only:
    - merge_requests
`
	_ = cfg

	if err := os.WriteFile(".gitlab-ci-trackfw.yml", []byte(content), 0644); err != nil {
		return fmt.Errorf("writing GitLab CI: %w", err)
	}
	fmt.Println("  ✓ .gitlab-ci-trackfw.yml")
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
	if err := os.MkdirAll(".husky", 0755); err != nil {
		return err
	}
	content := "#!/usr/bin/env sh\n. \"$(dirname -- \"$0\")/_/husky.sh\"\n\ntrackfw validate\n"
	path := ".husky/pre-commit"
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		return fmt.Errorf("writing husky hook: %w", err)
	}
	fmt.Printf("  ✓ %s\n", path)
	return nil
}

func generateLefthookHook() error {
	content := `pre-commit:
  commands:
    trackfw-validate:
      run: trackfw validate
`
	if err := os.WriteFile("lefthook.yml", []byte(content), 0644); err != nil {
		return fmt.Errorf("writing lefthook config: %w", err)
	}
	fmt.Println("  ✓ lefthook.yml")
	return nil
}
