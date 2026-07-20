package generators

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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

	if err := generateAttentionScripts(); err != nil {
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

func installGlobalSkill() error {
	return installGlobalSkillInner(false)
}

func installGlobalSkillInner(force bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("localizando home dir: %w", err)
	}

	skillDir := filepath.Join(home, ".claude", "skills", "trackfw")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return fmt.Errorf("creating %s: %w", skillDir, err)
	}

	skillPath := filepath.Join(skillDir, "SKILL.md")
	if _, err := os.Stat(skillPath); err == nil && !force {
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

	if err := os.WriteFile(skillPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing SKILL.md: %w", err)
	}
	fmt.Printf("  ✓ ~/.claude/skills/trackfw/SKILL.md\n")
	return nil
}

// ForceGenerateClaudeCommands re-gera todos os slash commands, sobrescrevendo arquivos existentes.
func ForceGenerateClaudeCommands() error {
	return generateClaudeCommandsInner(true)
}

func generateClaudeCommands() error {
	return generateClaudeCommandsInner(false)
}

func generateClaudeCommandsInner(force bool) error {
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

Estados válidos: ` + "`backlog`, `analyzing`, `wip`, `blocked`, `done`, `abandoned`" + `

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

	created, skipped := 0, 0
	for filename, content := range commands {
		path := filepath.Join(dir, filename)
		if _, err := os.Stat(path); err == nil && !force {
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

	if cfg.BrownfieldMode {
		content += fmt.Sprintf("governance_mode: lenient\nlenient_until: %s\n", cfg.LenientUntil.Format("2006-01-02"))
	}

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

func generateAttentionScripts() error {
	if err := os.MkdirAll("scripts", 0755); err != nil {
		return err
	}

	signalScript := `#!/usr/bin/env bash
# trackfw attention signal — PreToolUse/BeforeTool hook
set -euo pipefail

INPUT=$(cat)

# Script e intencionalmente no-op quando executado fora do diretorio raiz do projeto trackfw.
[ -f "trackfw.yaml" ] || exit 0

if command -v jq &>/dev/null; then
  TOOL=$(echo "$INPUT" | jq -r '.tool_name // ""')
  MSG=$(echo "$INPUT" | jq -r '(.tool_input.question // .tool_input.command // "Agent is executing: \(.tool_name // "unknown")") | .[0:300]')
else
  TOOL=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('tool_name',''))" 2>/dev/null || echo "")
  MSG=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); ti=d.get('tool_input',{}); print((ti.get('question') or ti.get('command') or 'Agent is executing: '+d.get('tool_name','unknown'))[:300])" 2>/dev/null || echo "Agent needs attention")
fi

ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml 2>/dev/null | head -1 | sed 's/^roadmap_dir:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d '"' | tr -d "'" || true)
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

	cleanupScript := `#!/usr/bin/env bash
# trackfw attention cleanup — PostToolUse/AfterTool hook
set -euo pipefail

# Script e intencionalmente no-op quando executado fora do diretorio raiz do projeto trackfw.
[ -f "trackfw.yaml" ] || exit 0

ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml 2>/dev/null | head -1 | sed 's/^roadmap_dir:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d '"' | tr -d "'" || true)
ROADMAP_DIR=${ROADMAP_DIR:-docs/roadmaps}
case "$ROADMAP_DIR" in
  /*|../*|*/../*|*/..|..) ROADMAP_DIR="docs/roadmaps" ;;
esac

rm -f "$ROADMAP_DIR/.trackfw-attention.json"
exit 0
`

	signalPath := filepath.Join("scripts", "trackfw-attention-signal.sh")
	if err := os.WriteFile(signalPath, []byte(signalScript), 0755); err != nil {
		return fmt.Errorf("writing attention signal script: %w", err)
	}
	fmt.Printf("  ✓ %s\n", signalPath)

	cleanupPath := filepath.Join("scripts", "trackfw-attention-cleanup.sh")
	if err := os.WriteFile(cleanupPath, []byte(cleanupScript), 0755); err != nil {
		return fmt.Errorf("writing attention cleanup script: %w", err)
	}
	fmt.Printf("  ✓ %s\n", cleanupPath)

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

func generateCommitMsgHook(cfg Config) error {
	if !cfg.RequireReqInCommit {
		return nil
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
		if err := os.MkdirAll(".husky", 0755); err != nil {
			return fmt.Errorf("creating .husky: %w", err)
		}
		path := ".husky/commit-msg"
		if err := os.WriteFile(path, []byte(script), 0755); err != nil {
			return fmt.Errorf("writing husky commit-msg hook: %w", err)
		}
		fmt.Printf("  ✓ %s\n", path)
	case "lefthook":
		lefthookPath := "lefthook.yml"
		existing, _ := os.ReadFile(lefthookPath)
		if !strings.Contains(string(existing), "commit-msg:") {
			addition := "\ncommit-msg:\n  scripts:\n    \"trackfw-req-check.sh\":\n      runner: sh\n"
			if err := os.WriteFile(lefthookPath, append(existing, []byte(addition)...), 0644); err != nil {
				return fmt.Errorf("writing lefthook.yml commit-msg section: %w", err)
			}
		}
		scriptDir := ".lefthook/commit-msg"
		if err := os.MkdirAll(scriptDir, 0755); err != nil {
			return fmt.Errorf("creating %s: %w", scriptDir, err)
		}
		scriptPath := scriptDir + "/trackfw-req-check.sh"
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
