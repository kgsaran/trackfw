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
	Forge              string    // forge platform: "github", "gitlab", "bitbucket", "azure", or "" (omit key)
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

func Scaffold(cfg Config) error {
	for _, dir := range govDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
		fmt.Printf("  ✓ %s\n", dir)
	}

	if err := generateVaultIndex(); err != nil {
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
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("localizando home dir: %w", err)
	}

	skillPath := GlobalClaudeSkillPath(home)
	skillDir := filepath.Dir(skillPath)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return fmt.Errorf("creating %s: %w", skillDir, err)
	}

	if _, err := os.Stat(skillPath); err == nil && !force {
		fmt.Printf("  ✓ ~/.claude/skills/trackfw/SKILL.md (já existe — não sobrescrito)\n")
		return nil
	}

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

   ` + "```markdown" + `
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
			"trackfw barrier <roadmap> --wave <n> --json\n" +
			"```\n" +
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

	if cfg.Forge != "" {
		content += fmt.Sprintf("forge: %s\n", cfg.Forge)
	}

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

// GenerateAttentionScripts gera os scripts shell de attention signal/cleanup em
// <rootDir>/scripts. Se rootDir for "", usa o diretório de trabalho atual (mesmo
// comportamento de antes da exportação). O conteúdo gerado é idêntico ao produzido
// por `trackfw init`.
func GenerateAttentionScripts(rootDir string) error {
	if rootDir == "" {
		rootDir = "."
	}
	scriptsDir := filepath.Join(rootDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return err
	}

	signalScript := `#!/usr/bin/env bash
# trackfw attention signal — PreToolUse/BeforeTool hook
set -euo pipefail

INPUT=$(cat)

# Script is intentionally a no-op when executed outside the project root
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

# Script is intentionally a no-op when executed outside the project root
[ -f "trackfw.yaml" ] || exit 0

ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml 2>/dev/null | head -1 | sed 's/^roadmap_dir:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d '"' | tr -d "'" || true)
ROADMAP_DIR=${ROADMAP_DIR:-docs/roadmaps}

case "$ROADMAP_DIR" in
  /*|../*|*/../*|*/..|..) ROADMAP_DIR="docs/roadmaps" ;;
esac

rm -f "$ROADMAP_DIR/.trackfw-attention.json"
exit 0
`

	signalPath := filepath.Join(scriptsDir, "trackfw-attention-signal.sh")
	if err := os.WriteFile(signalPath, []byte(signalScript), 0755); err != nil {
		return fmt.Errorf("writing attention signal script: %w", err)
	}
	// Mensagem sempre com caminho relativo "scripts/..." — igual ao literal fixo
	// que o Node.js imprime (npm/src/generators/hooks.js:generateAttentionScripts)
	// — independente de rootDir ser "" (cwd, usado por init/update) ou um caminho
	// absoluto (usado por discover --init via InstallGates).
	fmt.Printf("  ✓ %s\n", filepath.Join("scripts", "trackfw-attention-signal.sh"))

	cleanupPath := filepath.Join(scriptsDir, "trackfw-attention-cleanup.sh")
	if err := os.WriteFile(cleanupPath, []byte(cleanupScript), 0755); err != nil {
		return fmt.Errorf("writing attention cleanup script: %w", err)
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
	if rootDir == "" {
		rootDir = "."
	}
	scriptsDir := filepath.Join(rootDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return err
	}

	path := filepath.Join(scriptsDir, "trackfw-credential-guard.sh")
	if err := os.WriteFile(path, []byte(credentialGuardScript), 0755); err != nil {
		return fmt.Errorf("writing credential guard script: %w", err)
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
	scriptsDir := filepath.Join(home, ".trackfw", "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return err
	}

	path := filepath.Join(scriptsDir, "trackfw-credential-guard.sh")
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

ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml 2>/dev/null | head -1 | sed 's/^roadmap_dir:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d '"' | tr -d "'" || true)
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

// credentialGuardScript é o conteúdo canônico do hook de escopo de projeto — espelhado byte-a-byte
// em pypi/trackfw/generators/init_gen.py (string raw) e, com backslashes duplicados/`${...}`
// escapado para o parser de template literal, em npm/src/generators/hooks.js. Ver
// internal/generators/credential_guard_test.go (TestCredentialGuardScript_ParityAcrossStacks), que
// prova a paridade entre os 3 stacks byte-a-byte.
const credentialGuardScript = credentialGuardHeader + credentialGuardProjectGuardBlock + credentialGuardDetectionCore + credentialGuardProjectTail

// globalCredentialGuardScript é o conteúdo canônico do hook de escopo global
// (~/.trackfw/scripts/trackfw-credential-guard.sh, instalado via `trackfw update harness`).
// Reusa o mesmo cabeçalho e o mesmo núcleo de detecção de credentialGuardScript — só a resolução de
// MODE/ROADMAP_DIR (credentialGuardGlobalTail) e a ausência da guarda de "dentro de projeto trackfw"
// diferem. Ver credentialGuardGlobalTail para a decisão de design completa.
const globalCredentialGuardScript = credentialGuardHeader + credentialGuardDetectionCore + credentialGuardGlobalTail

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

// generateVaultIndex cria vault/notes/index.md se ainda não existir.
// O arquivo é o ponto de entrada do vault de conhecimento do projeto.
func generateVaultIndex() error {
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
	if err := os.WriteFile(indexPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing vault/notes/index.md: %w", err)
	}
	fmt.Println("  ✓ vault/notes/index.md")
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
