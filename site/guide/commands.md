# Referência de Comandos

Referência completa de todos os comandos do `trackfw`.

---

## `trackfw init`

Inicializa a estrutura de governança no projeto atual via wizard interativo.

```bash
trackfw init [--brownfield] [--ai-tools codex,...]
```

### Flags

| Flag | Descrição |
|------|-----------|
| `--brownfield` | Ativa modo lenient por 30 dias (violações viram warnings) |
| `--ai-tools` | Configura os nove targets de IA em modo não interativo nos três runtimes |

### O que é gerado

- `docs/adr/`, `docs/req/`, `docs/roadmaps/{backlog,wip,blocked,done,abandoned}/`
- `trackfw.yaml` — configuração do projeto
- `scripts/trackfw-validate.sh` — script de validação para CI
- `CLAUDE.md` — contexto para Claude Code (se selecionado)
- `.claude/commands/` — 7 slash commands para Claude Code
- `AGENTS.md`, `.agents/skills/`, `.codex/agents/` e `.codex/hooks.json` — integração Codex (se selecionado)
- `.husky/` ou `lefthook.yml` — git hooks (se selecionado)
- `.github/workflows/trackfw.yml` ou `.gitlab-ci.yml` (se selecionado)
- `pom.xml` Spring Boot 3.3 (se backend=java)

### Exemplo

```bash
$ trackfw init
? Project name: meu-projeto
? Project type: fullstack
? Backend language: java
? Backend framework: Spring Boot
? Git hooks: husky
? CI/CD: GitHub Actions
? AI assistants: Claude Code

✓ Governance structure initialized.
```

---

## `trackfw discover`

Escaneia o repositório e detecta automaticamente a estrutura de governança já existente
(ADRs, REQs, roadmaps, stack).

```bash
trackfw discover
trackfw discover --init
trackfw discover --bootstrap-log
```

### Flags

| Flag | Descrição |
|------|-----------|
| `--init` | Gera um `trackfw.yaml` calibrado para o projeto a partir do que foi detectado |
| `--bootstrap-log` | Cria um `.trackfw-log` retroativo a partir dos arquivos já em `done/` |

---

## `trackfw configure`

Wizard interativo que guia a configuração do `trackfw.yaml`. Gera um arquivo esparso: só grava
os campos que diferem dos defaults.

```bash
trackfw configure
```

---

## `trackfw help`

**Não** é o `--help` genérico gerado pelo framework de CLI (cobra/commander/argparse) — esse
continua disponível separadamente em todo comando (`trackfw --help`, `trackfw <comando>
--help`). `trackfw help` é a superfície explícita de ajuda do projeto: lista os comandos
disponíveis **e** documenta as chaves de configuração do `trackfw.yaml`.

```bash
trackfw help              # lista os comandos e todas as chaves de trackfw.yaml
trackfw help <comando>    # ajuda daquele comando (equivalente a "<comando> --help")
trackfw help <chave>      # documentação de uma chave de trackfw.yaml (tipo, default, exemplo, impacto)
```

Sem argumento, a saída tem duas seções: a lista de comandos disponíveis (nome + descrição curta)
e uma tabela `KEY / DEFAULT / DESCRIÇÃO` com todas as chaves reconhecidas de `trackfw.yaml`
(`adr_dirs`, `roadmap_namespacing`, `wip_limit`, `rules.*`, `trace_id_field`, etc.).

Com um argumento que casa com o nome de uma chave de configuração, mostra tipo, valor default,
descrição, um exemplo de uso em YAML e o impacto prático de alterá-la. Com um argumento que casa
com o nome de um comando, mostra a ajuda desse comando. Um assunto desconhecido — nem comando nem
chave — sai com código de saída diferente de zero e, quando há um nome próximo o suficiente,
sugere a correção.

---

## `trackfw branch new`

Cria uma branch `feat/`, `fix/`, `refactor/`, `chore/` ou `docs/` movendo o gate de governança
`branch_has_wip_roadmap` (já aplicado por `trackfw validate` e `trackfw ship`) para **antes** da
criação da branch, em vez de depois.

```bash
trackfw branch new feat/login-oauth
trackfw branch new fix/corrige-401 --dry-run
```

### Comportamento

| Tipo | Comportamento |
|---|---|
| `feat`, `fix`, `refactor` | Exige roadmap com slug correspondente já em `wip/` ou `done/` — sem correspondência, bloqueia e `git checkout -b` nunca é executado |
| `chore`, `docs` | Isentos do gate de roadmap — a branch é criada normalmente |

### Flags

| Flag | Descrição |
|------|-----------|
| `--dry-run` | Reporta se a branch seria criada ou bloqueada, sem executar o `git` |

Quando permitido, executa `git checkout -b <type>/<slug>`, propagando literalmente a saída e o
exit status do Git.

### Se bloquear

```bash
trackfw req new "title"
trackfw roadmap new "title"
trackfw roadmap move <name> wip
```

---

## `trackfw agents` e `trackfw skills`

Gerenciam agents especialistas e skills de governança com o mesmo contrato nos
CLIs Go/Homebrew, npm e PyPI.

```bash
trackfw agents list|install|uninstall|update [flags]
trackfw skills list|install|uninstall|update [flags]
```

Targets suportados: `claude`, `codex`, `gemini`, `antigravity`, `cursor`,
`copilot`, `windsurf`, `amazonq`, `opencode` e `kiro`.

### Flags

| Flag | Descrição |
|---|---|
| `--targets <csv>` | CLIs de destino; obrigatório para mutações sem TTY |
| `--items <csv>` | IDs do catálogo; o padrão é todos os items |
| `--scope project\|global` | Instala no projeto ou no diretório do usuário |
| `--surface target=surface` | Seleciona uma surface específica; pode ser repetido |
| `--json` | Emite catálogo e deployments em formato determinístico |
| `--force` | Permite substituir/remover conteúdo gerenciado modificado |

Em TTY, `install`, `update` e `uninstall` sem `--targets` abrem uma seleção
interativa. Em CI ou outro ambiente não interativo, omitir `--targets` é erro.

### Exemplos

```bash
# Lista catálogo, formato nativo/fallback e estado; inclui surfaces legadas
trackfw agents list --json

# Instala agents e skills selecionados no projeto
trackfw agents install --targets claude,codex --items architect,backend --scope project
trackfw skills install --targets gemini,kiro --items governance,implement --scope project

# Instala globalmente e seleciona a surface CLI do Kiro
trackfw agents install --targets kiro --scope global --surface kiro=cli

# Inspeciona a surface antiga do Antigravity explicitamente
trackfw agents list --targets antigravity --surface antigravity=legacy-cli

# Atualiza ou remove apenas deployments selecionados
trackfw skills update --targets codex,gemini
trackfw agents uninstall --targets claude --items backend
```

Os estados são `not-installed`, `current`, `outdated` e `modified`. O manifesto
`.trackfw/integrations-manifest.json`, separado por scope, registra ownership,
versão, SHA-256 e claims compartilhados. Arquivos `modified` são preservados por
`update` e `uninstall` até o uso explícito de `--force`. Uninstall nunca remove
arquivo unmanaged nem artefato ainda compartilhado. Uma instalação histórica
com hash conhecido é adotada sem overwrite e aparece como `outdated`; `update`
faz a migração. Conteúdo unmanaged desconhecido nunca é adotado por update,
mesmo com `--force`.

O gate de paridade de identidade é derivado do catálogo canônico de integrações:
toda surface com suporte a agents entra automaticamente na matriz Go/Node/Python.
Surfaces não-default são exercitadas como `target=surface`, o mesmo formato usado
por `--surface`.

Os comandos standalone `gemini`, `cursor`, `copilot`, `windsurf` e `amazonq`
foram removidos. Use `agents` e `skills` com `--targets` (os nomes de target
acima continuam válidos ali) para todos os fluxos de instalação e atualização.

`trackfw agents third-party` e `trackfw skills third-party` buscam e instalam conteúdo de
terceiros sob um gate de quarentena em duas fases — `fetch` baixa para quarentena sem instalar,
`install` consome um artefato já em quarentena e com aprovação vinculada ao checksum.

---

## `trackfw update`

Reaplica os templates atuais do trackfw a um projeto já inicializado com `trackfw init` ou
`trackfw discover --init`. É uma operação de escopo de projeto — nunca mexe no diretório home do
usuário. Atualiza o bloco de regras trackfw nos arquivos de config de agente (`CLAUDE.md`,
`GEMINI.md` etc.), `scripts/trackfw-validate.sh`, o workflow de CI, deployments de agent/skill do
Codex já existentes, slash commands históricos do Claude e Git hooks.

```bash
trackfw update
trackfw update --dry-run --json
trackfw update --targets claude,codex --install-missing
```

### Flags

| Flag | Descrição |
|------|-----------|
| `--dry-run` | Calcula e reporta os estados dos targets sem escrever nada |
| `--install-missing` | Também instala targets reportados como ausentes |
| `--targets <csv>` | Subconjunto de IDs de target (ID desconhecido é erro de uso) |
| `--json` | Emite o resultado como documento JSON, em vez do relatório em texto |

### `trackfw update harness`

Reaplica os templates atuais a todo artefato de escopo global já instalado no diretório home do
usuário (skill de compatibilidade histórica do Claude, deployments globais de agent/skill do
Codex). Nunca requer `trackfw.yaml` nem toca no repositório atual — isso é trabalho de
`trackfw update`. Aceita as mesmas flags `--dry-run`, `--json`, `--targets` e `--install-missing`.

```bash
trackfw update harness
trackfw update harness --install-missing
```

---

## `trackfw adr new`

Cria um novo Architecture Decision Record via wizard interativo.

```bash
trackfw adr new
```

### Saída esperada

```
created docs/adr/ADR-2026-06-13-titulo-da-decisao.md
```

---

## `trackfw adr list`

Lista todos os ADRs do projeto com status.

```bash
trackfw adr list
```

### Saída esperada

```
ADR-2026-06-13-usar-postgresql.md         Proposed
ADR-2026-06-10-arquitetura-monolito.md    Accepted
ADR-2026-06-01-provider-oauth.md          Draft
```

---

## `trackfw req new`

Cria um novo requisito via wizard interativo com probes contextuais.

```bash
trackfw req new
```

O wizard detecta domínios (autenticação, UI, persistência, API, deploy, eventos) com base no título e motivação e apresenta perguntas específicas por domínio. ADR Drafts são criados automaticamente quando a resposta indica decisão arquitetural pendente.

### Saída esperada

```
created docs/req/REQ-2026-06-13-login-via-oauth.md
created docs/adr/ADR-2026-06-13-provider-oauth.md (Draft)
```

---

## `trackfw req list`

Lista todos os requisitos com status.

```bash
trackfw req list
```

### Saída esperada

```
REQ-2026-06-13-login-via-oauth.md      Open
REQ-2026-06-10-exportar-relatorio.md   Closed
```

---

## `trackfw req move`

Atualiza o status de uma REQ no lugar (frontmatter e header).

```bash
trackfw req move <nome-parcial> <status>
```

### Exemplo

```bash
trackfw req move login-via-oauth Closed
```

---

## `trackfw roadmap new`

Cria um novo roadmap de implementação.

```bash
trackfw roadmap new [--title "Título"] [--req docs/req/REQ-*.md] [--from-req docs/req/REQ-*.md]
```

### Flags

| Flag | Descrição |
|------|-----------|
| `--title "Título"` | Define título sem wizard |
| `--req <path>` | Vincula REQ ao roadmap |
| `--from-req <path>` | Cria roadmap já vinculado à REQ (shorthand) |

### Exemplos

```bash
# Wizard interativo
trackfw roadmap new

# Com título e REQ definidos
trackfw roadmap new --title "Implementar OAuth" --req docs/req/REQ-2026-06-13-login-via-oauth.md

# Shorthand
trackfw roadmap new --from-req docs/req/REQ-2026-06-13-login-via-oauth.md
```

---

## `trackfw roadmap list`

Lista todos os roadmaps agrupados por estado.

```bash
trackfw roadmap list
```

### Saída esperada

```
[backlog]  ROADMAP-2026-06-13-implementar-oauth.md
[wip]      ROADMAP-2026-06-10-refactor-db.md
[done]     ROADMAP-2026-06-01-setup-ci.md
```

---

## `trackfw roadmap move`

Move um roadmap entre estados do kanban.

```bash
trackfw roadmap move <nome-parcial> <estado>
```

Estados válidos: `backlog`, `analyzing`, `wip`, `blocked`, `done`, `abandoned`

### Exemplo

```bash
trackfw roadmap move oauth wip
# ✓ moved ROADMAP-2026-06-13-implementar-oauth.md → docs/roadmaps/wip
```

A transição é registrada automaticamente em `docs/roadmaps/.trackfw-log`.
Ao mover, o CLI sincroniza a pasta, o frontmatter `status:` e o header
`| Status:`. O estado `analyzing` representa a fase de leitura, validação e
planejamento antes de iniciar execução em `wip`.

---

## `trackfw roadmap show`

Exibe o conteúdo completo de um roadmap com busca parcial por nome.

```bash
trackfw roadmap show <nome-parcial>
```

### Exemplo

```bash
trackfw roadmap show oauth
```

```
─────────────────────────────────────────
ROADMAP-2026-06-13-implementar-oauth.md — [WIP]
─────────────────────────────────────────

---
status: wip
date: 2026-06-13
req: docs/req/REQ-2026-06-13-login-via-oauth.md
squad: ""
---

# Roadmap: Implementar OAuth
...

Location: docs/roadmaps/wip/ROADMAP-2026-06-13-implementar-oauth.md
```

---

## `trackfw baseline`

Executa todas as validações e salva o resultado como baseline em `.trackfw-baseline.json`. O
`trackfw validate` subsequente reporta apenas violations **novas** em relação a esse baseline —
útil para adotar governança em projeto brownfield sem precisar zerar o passivo existente de uma vez.

```bash
trackfw baseline
```

Commite `.trackfw-baseline.json` no repositório para documentar o passivo aceito.

---

## `trackfw validate`

Valida a consistência entre ADRs, REQs e Roadmaps do projeto.

```bash
trackfw validate
```

### Regras validadas

1. Roadmaps em WIP devem ter campo REQ preenchido
2. Roadmaps em WIP devem ter critérios de aceite
3. Somente um roadmap pode estar em WIP por vez (configurável por squad)
4. Roadmaps em Blocked devem ter campo REQ preenchido
5. REQs devem ter Roadmap vinculado
6. ADRs devem ser referenciados em pelo menos uma REQ
7. REQs Open não podem estar bloqueadas por ADRs em Draft
8. Roadmaps em WIP há mais de `stale_wip_days` dias são marcados como stale
9. ADRs e REQs devem ter frontmatter YAML válido

### `stale_wip`

`stale_wip` usa a entrada mais recente do roadmap em `wip/` no
`docs/roadmaps/.trackfw-log`, incluindo transições `backlog → wip`,
`analyzing → wip` e `blocked → wip`. Em `roadmap_namespacing: by_agent`, a
identidade do roadmap inclui o prefixo do agente registrado no log, por exemplo
`zeus/ROADMAP-2026-07-27-exemplo.md`.

Se o log não existir ou não houver entrada parseável para o roadmap atual, o
fallback retrocompatível é o `mtime` do arquivo. Tempo de commit Git não faz
parte do contrato.

```yaml
stale_wip_days: 14 # default: 7
rules:
  stale_wip: warning
```

### Política de I/O do validator

Diretórios opcionais de estado ausentes, como `wip/`, `blocked/` ou `done/`,
são tratados como vazios. Falhas reais de inspeção, como permissão negada,
`ENOTDIR`, erro de walk/list, arquivo esperado ilegível ou linha inválida no
`.trackfw-log`, geram diagnóstico com regra, caminho e causa; a severidade segue
a configuração da regra (`off`, `warning` ou `error`).

### Saída esperada — sem violações

```
✓ No violations found.
```

### Saída esperada — com problemas

```
✗ 2 violation(s) found:

  [violation] ROADMAP-2026-06-13-implementar-oauth.md missing REQ field
  [violation] REQ-2026-06-13-login-via-oauth.md is blocked by Draft ADR: ADR-2026-06-13-provider-oauth.md

⚠  1 warning(s):

  [warning] ROADMAP-2026-06-10-refactor-db.md in WIP for 9 days (stale)
```

### Modo lenient (brownfield)

```
[LENIENT MODE until 2026-07-13]

⚠  1 violation treated as warning:
  [warning] ROADMAP-2026-06-13-implementar-oauth.md missing REQ field
```

---

## `trackfw status`

Exibe visão geral do estado atual do projeto.

```bash
trackfw status
```

### Saída esperada

```
trackfw — project status

📋 Backlog       2 roadmaps
🔄 WIP           1 roadmap
⚠  Stale WIP     0 roadmaps
❌ Blocked       0 roadmaps
✅ Done          3 roadmaps

📄 ADRs          4   (Proposed: 2, Accepted: 1, Draft: 1)
📝 REQs          3   (Open: 2, Closed: 1)

⏳ REQs blocked by Draft ADRs:
  REQ-2026-06-13-login-via-oauth.md → ADR-2026-06-13-provider-oauth.md
```

---

## `trackfw barrier`

Barreira determinística de liberação de wave. É **agnóstica de stack**: nunca assume ferramenta de
build, test runner ou regra de paridade — todo check executável vem do próprio roadmap (os gates
declarados na wave) ou do `trackfw validate` rodado em processo.

```bash
trackfw barrier <roadmap> --wave <n> [--json]
```

`<roadmap>` é o basename com ou sem `.md`, resolvido em `wip/` e depois em `done/` sob
`roadmap_dir` (inclusive no layout `by_agent`). `--wave` é obrigatório.

### Checks embutidos

| Check | Passa quando |
|---|---|
| `mls_complete` | A wave tem ao menos um ML e todos estão `**Status:** ✅` |
| `acceptance_evidence` | Todo ML tem um bloco `**Critérios de aceite:**` não vazio, sem nenhuma linha `- [ ]` |
| `gates` | Todo comando declarado em `**Gates da wave:**` sai com código 0 — uma wave sem esse bloco declara zero gates, e a barrier nunca inventa um |
| `validate` | `trackfw validate --json` reporta `violations: 0` |

### Exit codes

| Exit | Significado |
|---|---|
| `0` | `status: "passed"` — todos os checks passaram, a wave pode ser liberada |
| `1` | `status: "blocked"` — pelo menos um check falhou; o relatório (texto ou `--json`) diz qual |
| `2` | Erro de uso/resolução — roadmap ou wave não encontrados. **Não** é `blocked`: uma barrier que não pôde rodar é diferente de uma que rodou e reprovou |

### Fluxo de correção

```bash
$ trackfw barrier ROADMAP-example --wave 2
✗ mls_complete: ML-2C: not complete (status: 🔄)
✗ acceptance_evidence: ML-2C: 2 unmet acceptance criteria
wave 2: blocked
```

Corrija o roadmap (marque o ML como `✅`, feche os critérios pendentes) e rode o **mesmo comando** de
novo — a barrier não é uma negação permanente; a wave corrigida passa na próxima invocação:

```bash
$ trackfw barrier ROADMAP-example --wave 2
✓ mls_complete
✓ acceptance_evidence
✓ gates
✓ validate
wave 2: passed
```

### Saída JSON

```bash
trackfw barrier ROADMAP-example --wave 2 --json
```

```json
{
  "roadmap": "ROADMAP-example.md",
  "wave": 2,
  "status": "blocked",
  "started_at": "2026-07-29T10:30:00Z",
  "finished_at": "2026-07-29T10:30:04Z",
  "checks": [
    { "name": "mls_complete", "status": "passed", "evidence": ["ML-2A: ✅"], "failures": [] },
    { "name": "acceptance_evidence", "status": "blocked", "evidence": [], "failures": ["ML-2C: 2 unmet acceptance criteria"] },
    { "name": "gates", "status": "passed", "commands": ["make quality"], "evidence": ["make quality: exit 0"], "failures": [] },
    { "name": "validate", "status": "passed", "evidence": ["0 violations, 0 warnings"], "failures": [] }
  ],
  "failures": ["acceptance_evidence: ML-2C: 2 unmet acceptance criteria"]
}
```

### `trackfw barrier` vs. `/trackfw:barrier`

`trackfw barrier` é o núcleo determinístico e reproduzível — nunca invoca agentes, nunca executa
Git. O slash command `/trackfw:barrier` orquestra em torno dele: dispara as revisões de
`code-quality`/`security` quando aplicável, audita o diff contra o escopo combinado, e — apenas o
`trackfw_architect`, única autoridade Git do fluxo — comita e faz push quando tudo estiver verde.
Uma barrier de CLI verde é necessária, mas não suficiente, para liberar uma wave. Contrato completo:
`docs/cli-parity.md` → `## trackfw barrier`.

---

## `trackfw context`

Emite o contexto de governança do projeto para consumo por LLMs e agentes de IA.

```bash
trackfw context [--format=md|json]
```

### Flags

| Flag | Descrição | Padrão |
|------|-----------|--------|
| `--format` | Formato de saída: `md` ou `json` | `md` |

### Exemplo — formato JSON

```bash
trackfw context --format=json
```

```json
{
  "project": "meu-projeto",
  "governance_score": 80,
  "adrs": [
    { "file": "ADR-2026-06-13-usar-postgresql.md", "status": "Accepted" }
  ],
  "reqs": [
    { "file": "REQ-2026-06-13-login-via-oauth.md", "status": "Open" }
  ],
  "roadmaps": {
    "wip": ["ROADMAP-2026-06-13-implementar-oauth.md"]
  },
  "violations": [],
  "warnings": []
}
```

---

## `trackfw serve`

Inicia um servidor HTTP local com visualização web da cadeia ADR → REQ → ROADMAP.

```bash
trackfw serve [--port 8080]
```

### Flags

| Flag | Descrição | Padrão |
|------|-----------|--------|
| `--port` | Porta do servidor | `8080` |

Acesse `http://localhost:8080` para ver:
- **Traceability** — mapa de rastreabilidade ADR → REQ → ROADMAP
- **Timeline** — linha do tempo de transições
- **Kanban** — board visual dos roadmaps por estado

---

## `trackfw metrics`

Calcula métricas de fluxo a partir do histórico de transições (`.trackfw-log`).

```bash
trackfw metrics [--since YYYY-MM-DD] [--export relatorio.csv]
```

### Flags

| Flag | Descrição |
|------|-----------|
| `--since` | Data de início do período (ex: `2026-01-01`) |
| `--export` | Exporta métricas para CSV |

### Métricas calculadas

- **Cycle time** — tempo médio de backlog → done
- **Throughput** — roadmaps concluídos por semana
- **WIP age** — tempo médio dos roadmaps em WIP

### Saída esperada

```
Metrics (2026-01-01 → 2026-06-13)

Cycle time:    4.2 days (avg)
Throughput:    2.1 roadmaps/week
WIP age:       3 days (avg)
```

---

## `trackfw sync`

Sincroniza REQs abertas com ferramentas de issue tracking externas.

```bash
trackfw sync --to=linear
trackfw sync --to=jira
```

### Flags

| Flag | Descrição | Valores |
|------|-----------|---------|
| `--to` | Destino da sincronização | `linear`, `jira` |

### Configuração — Linear

Em `trackfw.yaml` ou via variáveis de ambiente:

```yaml
linear_api_key: "lin_api_..."
linear_team_id: "TEAM_ID"
```

Ou:
```bash
export LINEAR_API_KEY="lin_api_..."
export LINEAR_TEAM_ID="TEAM_ID"
```

### Configuração — Jira

```yaml
jira_base_url: "https://empresa.atlassian.net"
jira_email: "usuario@empresa.com"
jira_token: "ATATT..."
jira_project: "PROJ"
```

Ou:
```bash
export JIRA_BASE_URL="https://empresa.atlassian.net"
export JIRA_EMAIL="usuario@empresa.com"
export JIRA_TOKEN="ATATT..."
export JIRA_PROJECT="PROJ"
```

### Saída esperada

```
REQ-2026-06-13-login-via-oauth.md → LIN-42 (created)
REQ-2026-06-10-exportar-relatorio.md → already synced (skipped)
```

---

## `trackfw log`

Exibe o histórico de transições de estado dos roadmaps.

```bash
trackfw log [--tail N]
```

### Flags

| Flag | Descrição | Padrão |
|------|-----------|--------|
| `--tail` | Número de entradas a exibir | `20` |

### Saída esperada

```
Date                 Roadmap                                            From       To
2026-06-13 14:32     ROADMAP-2026-06-13-implementar-oauth.md           backlog  → wip
2026-06-12 09:15     ROADMAP-2026-06-10-refactor-db.md                 wip      → done
```

---

## `trackfw note new`

Cria uma nova nota de conhecimento em `vault/notes/`.

```bash
trackfw note new "<título>"
```

### Saída esperada

```
created vault/notes/2026-06-13-titulo-da-nota.md
```

---

## `trackfw changelog`

Consulta o `CHANGELOG.md` sem sair do terminal.

```bash
trackfw changelog
trackfw changelog --all
trackfw changelog --version 7.0.0
```

### Flags

| Flag | Descrição |
|------|-----------|
| `--all` | Mostra o `CHANGELOG.md` inteiro |
| `--version <x.y.z>` | Mostra apenas a seção da versão informada |

Sem flags, mostra a seção `[Unreleased]` (ou a mais recente, se não houver `[Unreleased]`).

---

## `trackfw commit`

Passo intermediário entre `git commit` puro e `trackfw ship`: comita as mudanças staged
diretamente, mas bloqueia **antes** do commit acontecer quando falta governança — em vez de
deixar o commit acontecer e só detectar o problema depois.

```bash
trackfw commit -m "fix(api): correct 401 on refresh"
trackfw commit --suggest
```

### Comportamento por branch

| Branch | Comportamento |
|---|---|
| `main`/`master` | Sempre bloqueado — commit direto na branch default nunca é permitido |
| `feat/`, `fix/`, `refactor/` | Exige roadmap com slug correspondente já em `wip/` ou `done/` (mesma lógica de `trackfw branch new` e `trackfw validate`); sem correspondência, bloqueia com a mensma orientação de governança |
| Outras (ex.: `docs/`, `chore/`) | Permitido sem exigir roadmap — um warning é registrado, mas o commit prossegue |

### Flags

| Flag | Descrição |
|------|-----------|
| `-m` / `--message` | Mensagem de commit (obrigatória, exceto com `--suggest`) |
| `--suggest` | Imprime um esqueleto heurístico de mensagem Conventional Commits a partir de `git diff --cached --name-status` e sai sem commitar (ignora `-m`; nenhuma chamada de LLM) |

Quando permitido, executa `git commit -m <message>`, propagando literalmente a saída e o exit
status do Git.

### Se bloquear

```bash
trackfw req new "title"
trackfw roadmap new "title"
trackfw roadmap move <name> wip
```

---

## `trackfw ship`

Executa a sequência de entrega governada em sete passos: valida branch, verifica governança (REQ + roadmap em wip/), detecta squash-merges pendentes, revisa staged, commita, faz push e abre PR/MR via CLI da forge detectada (ou imprime URL de fallback caso a CLI não esteja disponível).

```bash
trackfw ship -m "feat(auth): add login flow"
trackfw ship -m "fix(api): correct 401 on refresh" --dry-run
trackfw ship -m "refactor(core): simplify handler" --no-pr
trackfw ship -m "feat(ui): new dashboard" --forge gitlab
```

### Flags

| Flag | Tipo | Descrição |
|------|------|-----------|
| `-m` / `--message` | string | Mensagem de commit (formato Conventional Commits obrigatório) |
| `--dry-run` | bool | Imprime o que seria feito sem executar comandos de escrita; no passo 7, também reporta disponibilidade da CLI e imprime URL de fallback |
| `--no-pr` | bool | Pula a criação do PR/MR após o push (passos 1–6 ainda rodam) |
| `--forge` | string | Sobrepõe a detecção de forge (`github`, `gitlab`, `bitbucket`, `azure`) |

### Detecção de forge

A forge é resolvida por precedência:

1. `--forge` flag
2. Campo `forge:` no `trackfw.yaml`
3. URL do remote (ex.: `github.com`, `gitlab.com`, `bitbucket.org`, `dev.azure.com`)
4. Arquivos de CI (`.gitlab-ci.yml` → gitlab; `.github/workflows/` → github)
5. Manual — nenhuma forge detectada

A forge e sua fonte são exibidas antes do passo 7:

```
Forge:     github (source: config)
```

### Exemplo

```bash
$ trackfw ship -m "feat(auth): add OAuth login"
✓ Branch: feat/auth-oauth
✓ Governance: REQ-2026-07-20-oauth.md + roadmap in wip/
✓ No pending squash-merges detected
  staged: auth/handler.go (+120 -5)
✓ Committed: feat(auth): add OAuth login
✓ Pushed to origin/feat/auth-oauth
Forge:     github (source: remote)
✓ Pull Request opened: https://github.com/org/repo/pull/42
```

---

## `trackfw completion`

Gera o script de autocompletion do trackfw para o shell informado.

```bash
trackfw completion bash|fish|powershell|zsh
```

Consulte `trackfw completion <shell> --help` para instruções de instalação específicas do shell.

---

## `trackfw version`

Exibe a versão instalada do trackfw.

```bash
trackfw version
# trackfw v7.0.0
```
