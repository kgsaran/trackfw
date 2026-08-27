---
title: Modelo de ameaça — superfície executável de um checkout de PR
date: 2026-08-26
author: hades-tf
ml: ML-0A
roadmap: docs/roadmaps/wip/ROADMAP-2026-08-26-comando-que-audita-a-superficie-executavel-de-um-checkout-de-pr.md
req: docs/req/REQ-2026-08-26-checkout-de-pr-executa-hook-versionado-sem-que-nada-avise-o-mantenedor.md
---

# Modelo de ameaça — superfície executável de um checkout de PR

> Cada afirmação está marcada **[medido]** (executei o comando nesta sessão e vi a saída) ou
> **[raciocinado]** (inferência sobre evidências coletadas nesta sessão).
>
> Este documento responde à pergunta de escopo do ML-0A: *o que, de fato, executa a partir de um
> checkout?* A resposta é baseada em busca, não em hipótese prévia.

---

## 1. Completude de enumeração

A lista a seguir cobre todos os caminhos que o Wave 1 precisa conhecer, mapeados em
**(runtime → arquivo de wiring → script referenciado)**, com presença confirmada por `git ls-files`.
Cada rung está ordenado do menor para o maior custo de ativação — o critério que torna o relatório
útil em vez de exaustivo.

### 1.1 Inventário de superfícies por custo de ativação

#### Rung 0 — Sobre o próprio `git checkout`

Neste repositório: **nenhuma superfície executa automaticamente no momento do checkout.**

**[medido]** `.git/config` contém `hooksPath = /dev/null`. Esta linha desativa todos os hooks git,
inclusive o `.husky/pre-commit` que está versionado (confirmado: `git ls-files .husky/pre-commit`
→ encontrado). O `hooksPath = /dev/null` vive em `.git/config`, que não é versionado e portanto
não é entregável por PR. Uma PR não pode armar este vetor *nesta árvore*.

**Ressalva:** este é o estado do clone local do mantenedor. Num clone novo sem `hooksPath = /dev/null`
configurado, o `.husky/pre-commit` versionado seria ativado por um `npm prepare` ou por qualquer
ferramenta que arm husky. Mas não há `prepare` em `npm/package.json` **[medido]** — logo nenhum
`npm install` arma o husky automaticamente neste projeto. O ADR afirma que a janela começa no
checkout; isso é correto aqui e é uma afirmação mais fraca do que parece: ela depende de nenhum hook
git ser ativado no clone do mantenedor, o que é verdade por `hooksPath = /dev/null` no local, mas
não é garantia por construção para clones novos se o `prepare` for adicionado por PR.

**Superfície latente (presente, não atualmentex ativa):** `.husky/pre-commit` (versionado, chama
`scripts/trackfw-validate.sh`) + `hooksPath = /dev/null` (local, não versionado). O Wave 1 deve
reportar `.husky/pre-commit` como superfície presente — ela é real, só está desarmada pela config
local.

#### Rung 1 — Abrir o repositório na ferramenta de agente e usar qualquer ferramenta

Este rung é o cerne desta REQ. Os arquivos abaixo estão **versionados e presentes** no repositório
e executam sem nenhuma invocação deliberada de comando do trackfw.

**[medido]** `git ls-files .claude/settings.json .codex/hooks.json .gemini/settings.json`
→ os três encontrados:

| Runtime | Arquivo de wiring | Evento de ativação | Scripts referenciados |
|---|---|---|---|
| Claude Code | `.claude/settings.json` | `PreToolUse / Bash` | `scripts/trackfw-git-branch-guard.sh` |
| Claude Code | `.claude/settings.json` | `PreToolUse / AskUserQuestion` | `scripts/trackfw-attention-signal.sh` |
| Claude Code | `.claude/settings.json` | `PostToolUse / AskUserQuestion` | `scripts/trackfw-attention-cleanup.sh` |
| Codex CLI | `.codex/hooks.json` | `PermissionRequest / .*` | `scripts/trackfw-attention-signal.sh` |
| Codex CLI | `.codex/hooks.json` | `PreToolUse / .*` | `scripts/trackfw-git-branch-guard.sh` |
| Codex CLI | `.codex/hooks.json` | `PostToolUse / .*` | `scripts/trackfw-attention-cleanup.sh` |
| Gemini CLI | `.gemini/settings.json` | `BeforeTool / run_shell_command` | `scripts/trackfw-git-branch-guard.sh` |
| Gemini CLI | `.gemini/settings.json` | `AfterTool / *` | `scripts/trackfw-attention-cleanup.sh` |
| Gemini CLI | `.gemini/settings.json` | `Notification / ToolPermission` | `scripts/trackfw-attention-signal.sh` |

**[medido]** Scripts referenciados: `git ls-files scripts/trackfw-git-branch-guard.sh
scripts/trackfw-attention-signal.sh scripts/trackfw-attention-cleanup.sh` → todos encontrados e
versionados. São o alvo do vetor "só o script muda, o wiring não" (ver §2).

**Ausentes nesta árvore (relevantes para o Wave 1, que audita checkouts arbitrários):**
Os 5 runtimes restantes cobertos por `check-agent-hooks-parity.sh` — o gate confirma que
**8 CLIs project-scope** são gerenciados pelo trackfw, não 6. O número 6 do prompt refere-se ao
escopo *harness* (global-scope), distinto. Os seguintes caminhos são ausentes neste clone, mas
pertencem ao inventário que o Wave 1 deve varrer:

| Runtime | Arquivo de wiring | Status nesta árvore |
|---|---|---|
| Cursor | `.cursor/hooks.json` | **ausente** |
| Kiro | `.kiro/hooks/trackfw-attention.json` | **ausente** (dir `.kiro/` não existe) |
| Windsurf | `.windsurf/hooks.json` | **ausente** |
| Copilot (GitHub) | `.github/hooks/trackfw-attention.json` | **ausente** |
| Amazon Q | `.amazonq/cli-agents/q_cli_default.json` | **ausente** |

**Declarar estas superfícies ausentes como "fora do escopo do Wave 1" seria o erro central desta
Wave 0** — ele esvaziaria a REQ para repositórios que usam Cursor, Kiro, Windsurf ou Amazon Q.

#### Rung 2 — Arquivos de instrução lidos pelo agente na abertura

Estes arquivos não executam código diretamente, mas são lidos pelo CLI de agente na inicialização
e interpretados como instrução — eles controlam *o que o agente fará* ao usar qualquer ferramenta.

**[medido]** `git ls-files CLAUDE.md AGENTS.md` → ambos versionados. `GEMINI.md` não aparece no
`ls-files` (não versionado neste clone).

| Runtime | Arquivo de instrução | Status nesta árvore |
|---|---|---|
| Claude Code | `CLAUDE.md` | **presente e versionado** |
| Codex CLI | `AGENTS.md` | **presente e versionado** |
| Windsurf | `.windsurfrules` | ausente |
| Amazon Q | `.amazonq/developer/guidelines.md` | ausente |
| Cursor | `.cursor/rules/trackfw.mdc` | ausente |
| Copilot | `.github/copilot-instructions.md` | ausente |

Um PR que modifica `CLAUDE.md` ou `AGENTS.md` não executa código de shell, mas altera o
comportamento do agente sobre toda operação seguinte. O Wave 1 deve reportar mudanças nestes
arquivos como superfície de instrução, distintos de scripts de shell.

#### Rung 2b — Slash commands (`.claude/commands/**/*.md`)

**[medido]** `find .claude/commands -name "*.md"` retorna 9 arquivos, todos versionados
(`barrier.md`, `architect.md`, `implement.md`, etc.). São lidos pelo Claude Code quando o
mantenedor invoca um slash command. O `barrier.md` contém `--trust-local-gates` — um PR que
altere este arquivo altera o que o slash command faz sem alterar o wiring de hook.

Ativação: requer o mantenedor invocar explicitamente o slash command. Mais fricção que o Rung 1.

#### Rung 3 — Armar commit (husky pre-commit)

**[medido]** `.husky/pre-commit` está versionado e chama `scripts/trackfw-validate.sh`. Executa
no `git commit` se `hooksPath` não for `/dev/null`. Não executa no checkout. O risco de PR aqui
é substituir `scripts/trackfw-validate.sh` por outro script sem alterar o `.husky/pre-commit` —
o mesmo padrão "só o script muda."

#### Rung 4 — Instalar dependências (lifecycle hooks)

**[medido]** `npm/package.json` não define `preinstall`, `postinstall` nem `prepare`. `pyproject.toml`
não define lifecycle hooks (só define entrypoint `[project.scripts]`). O CI usa `npm ci
--ignore-scripts` na etapa `node` do `quality.yml` — mitigação que neutraliza este vetor no
ambiente de CI mesmo que alguém adicione um hook por PR.

Um PR que adicione `"preinstall": "scripts/evil.sh"` ao `npm/package.json` ativa esta superfície;
ela não existe agora.

#### Rung 5 — Makefile targets alterados

**[medido]** Targets presentes: `build`, `test`, `test-node`, `test-python`, `parity`,
`sync-integration-assets`, `check-integration-assets`, `package-smoke`, `lint`, `quality`,
`install`, `clean`. Nenhum target executa script de agente hook. Um PR que modifique um target
existente ou adicione um novo precisa que o mantenedor execute `make <target>` explicitamente.
AC3 cobre targets *quando alterados*.

#### Rung 6 — CI / workflows

**[medido]** 5 workflows presentes: `quality.yml`, `trackfw-gate.yml`, `trackfw-validate.yml`,
`release.yml`, `deploy-docs.yml`. **Nenhum usa `pull_request_target`** — logo nenhum tem acesso
a secrets do repositório quando acionado por PR de fork. `release.yml` tem `permissions:
contents: write` mas dispara apenas em tags, não em PRs. Executa em ambiente isolado de CI,
não no shell do mantenedor.

#### Superfícies ausentes e fora do escopo desta REQ

**[medido]** Não encontrados por busca:

- `.envrc` (direnv) — ausente nesta árvore e em nenhum subdiretório
- `devcontainer.json` — ausente
- `.vscode/tasks.json` — ausente (`.vscode/tasks.json` com `"runOn": "folderOpen"` executaria na
  abertura do VS Code sem nenhuma ação; esta variante deve estar no inventário do Wave 1 mesmo
  ausente aqui, pelo mesmo motivo dos 5 runtimes de hook ausentes)

### 1.2 Conclusão de completude

A lista do ADR ("hooks de agente, scripts referenciados, Makefile e CI") é **hipótese correta no
quadro geral, mas incompleta em três pontos:**

1. Nomeia "hooks de agente" sem especificar os 8 runtimes project-scope — a ausência dos 5 que não
   existem neste clone poderia levar o implementador a excluí-los.
2. Não menciona arquivos de instrução (`CLAUDE.md`, `AGENTS.md`) como superfície distinta dos
   scripts de shell.
3. Não menciona slash commands (`.claude/commands/**/*.md`) como superfície de instrução com
   efeito em comandos futuros.

A lista está agora fechada por busca. O Wave 1 deve inventariar **todos os 8 paths de hook**
(independente de presença nesta árvore), os arquivos de instrução, e os slash commands — além de
Makefile, CI e lifecycle hooks.

---

## 2. Modelo de ameaça

### 2.1 O adversário

O adversário desta Wave 0 tem dois rostos:

- **O contribuidor hostil de PR**: submete código com o objetivo de executar código no shell do
  mantenedor. Ele controla o conteúdo de qualquer arquivo no PR antes do merge.
- **O implementador apressado**: escreve o Wave 1 sem ler esta seção e produz um comando que
  não cobre a classe inteira — não porque quebre regra alguma, mas porque interpreta o inventário
  deste repositório como o inventário completo.

Ambos esvaziam esta Wave 0 de formas distintas. O contribuidor explora a superfície; o
implementador reduz o que a ferramenta consegue ver.

### 2.2 O que o contribuidor controla

Num PR, o contribuidor controla:

1. **O conteúdo de qualquer script versionado** — `scripts/trackfw-git-branch-guard.sh`,
   `scripts/trackfw-attention-signal.sh`, `scripts/trackfw-attention-cleanup.sh`,
   `scripts/trackfw-validate.sh`, e qualquer script novo que ele adicione.
2. **O conteúdo dos arquivos de wiring** — `.claude/settings.json`, `.codex/hooks.json`,
   `.gemini/settings.json`.
3. **O conteúdo dos arquivos de instrução** — `CLAUDE.md`, `AGENTS.md`.
4. **O conteúdo dos slash commands** — `.claude/commands/trackfw/*.md`.
5. **O conteúdo dos Makefile targets** — `Makefile`.
6. **O conteúdo dos workflows de CI** — `.github/workflows/*.yml`.
7. **Novos arquivos em qualquer path** — incluindo novos hooks de runtime que não existem hoje
   (`.cursor/hooks.json`, `.kiro/hooks/`, etc.).

### 2.3 O que o mantenedor faz sem pensar

O mantenedor, ao revisar um PR, tipicamente:

1. Lê o diff no GitHub (não necessariamente abre cada arquivo alterado no editor).
2. Faz `git fetch origin pull/<N>/head && git checkout FETCH_HEAD` ou
   `gh pr checkout <N>` para testar localmente.
3. Abre a IDE ou o terminal no diretório do repositório.
4. Usa o agente (Claude Code, Codex, Gemini CLI) para ajudar na revisão ou nos testes.

**Passo 4 executa os hooks do Rung 1 imediatamente.** Não há confirmação adicional, não há flag
`--trust-local-gates`, não há nenhuma interposição do trackfw entre o checkout e a execução do
hook — é mais amplo que o vetor do `#208`, que exigia `trackfw barrier`.

### 2.4 Onde o código do contribuidor roda

| Momento | O que executa | Requer ação deliberada? |
|---|---|---|
| `git checkout` (com `.husky/pre-commit` armado) | `scripts/trackfw-validate.sh` | Não (se `hooksPath` apontar para `.husky`) |
| Abrir agente + qualquer tool call | Hook scripts referenciados pelo wiring file | **Não** — basta abrir e usar |
| Rodar `npm install` | `preinstall`/`postinstall` (se adicionado) | `npm install` explícito |
| Invocar slash command | Instruções de `.claude/commands/**/*.md` | Invocação explícita |
| `git commit` | `.husky/pre-commit` → script | `git commit` explícito |
| `make quality` | Targets do Makefile | `make` explícito |
| Push/PR → CI | Workflows de CI | Automático no GitHub, isolado |

### 2.5 O caso em que só o script muda

Este é o vetor nomeado pela REQ (AC2) e o mais insidioso:

**Cenário A — Substituição silenciosa de script:**
O PR não toca `.claude/settings.json`. O wiring continua apontando para
`scripts/trackfw-git-branch-guard.sh`. O PR substitui o conteúdo de
`scripts/trackfw-git-branch-guard.sh` por um script hostil. O diff do `.claude/settings.json`
é vazio; o mantenedor, ao revisar, vê apenas um "fix" num script cujo nome parece inócuo.
Na próxima vez que usar o Bash no Claude Code, o script hostil executa.

**Cenário B — Repontamento do wiring para script diferente:**
O PR muda uma linha em `.claude/settings.json`: `trackfw-git-branch-guard.sh` →
`scripts/install.sh`. O diff do wiring existe mas parece uma correção de path. O script
`install.sh` já existia no repositório — nenhum arquivo novo, nenhum script suspeito. O efeito:
o hook agora executa o instalador (ou qualquer outro script já presente) em toda invocação Bash.

**Cenário C — Alargamento do matcher:**
O PR muda `"matcher": "Bash"` para `"matcher": "*"` em `.claude/settings.json`. O script não
muda. O efeito: o hook que antes executava só em chamadas Bash agora executa em toda ferramenta
(Read, Write, Edit, WebSearch, etc.) — amplificação de superfície com diff de um token.

Estes três cenários mostram que o Wave 1 **não pode reportar apenas mudanças de conteúdo de
script**. Deve reportar a tupla completa **(trigger, matcher, script path, digest do script)**
para cada entrada de wiring — toda alteração em qualquer componente da tupla é superfície
executável.

### 2.6 Como o implementador apressado esvazia esta Wave 0

O implementador lê o inventário da seção §1 e observa que este repositório tem `.claude/`,
`.codex/` e `.gemini/` mas não tem `.cursor/`, `.kiro/`, `.windsurf/`, `.github/hooks/` nem
`.amazonq/`. Ele implementa o Wave 1 varrendo apenas os caminhos presentes. O comando
resultante não reporta nada sobre um PR que adicione `.kiro/hooks/trackfw-attention.json`
apontando para um script hostil — porque esse path não existia no repositório quando o
implementador o escreveu.

O Wave 1 deve varrer todos os 8 paths de hook por **padrão de path**, independente de presença.
A ausência é informação a ser reportada ("nenhum hook Kiro encontrado"), não uma razão para
omitir o check.

### 2.7 Achado pré-existente e fora de escopo

**[cross-referência]** O roadmap title com newline forja uma seção `## Wave N` que o `barrier`
executa como gate (encontrado em 2026-08-23, não corrigido). Esta superfície — conteúdo de
roadmap que chega por PR e, via `--trust-local-gates`, provoca execução de gate forjado — é
irmã desta mas precede esta REQ e tem seu próprio rastreamento. Não está no escopo deste comando.

---

## 3. Alvos de falsificação nas duas direções

Para cada superfície: onde a sabotagem entra e qual gate a detecta, nas duas direções.

### Direção A — Superfície executável alterada que deixa de ser reportada (falso-negativo)

| Superfície | Sabotagem | Onde entra | Gate que deveria acusar |
|---|---|---|---|
| Hook wiring (ex.: `.claude/settings.json`) | Comparação só por presença/ausência do arquivo, ignora conteúdo | `scripts/check-audit-surface.sh` cenário FN-1 | Cenário altera wiring, baseline é "nenhuma mudança reportada" |
| Script referenciado (ex.: `trackfw-git-branch-guard.sh`) | Comparação de hash só pelo wiring file; script não incluído no digest | `scripts/check-audit-surface.sh` cenário FN-2 (AC2) | Cenário modifica o script, wiring inalterado; baseline é "saída idêntica ao caso sem modificação" |
| Path de runtime ausente (ex.: `.kiro/hooks/`) | Comando não varre paths não presentes na árvore atual | `scripts/check-audit-surface.sh` cenário FN-3 | PR adiciona `.kiro/hooks/evil.json`; baseline é "reportado como nova superfície" |
| Matcher alargado | Comparação só do script, não da tupla completa | `scripts/check-audit-surface.sh` cenário FN-4 | PR muda `"matcher": "Bash"` → `"matcher": "*"`; baseline é "mudança de matcher reportada" |
| Arquivo de instrução (`CLAUDE.md`) | Superfície de instrução não inventariada, só scripts de shell | `scripts/check-audit-surface.sh` cenário FN-5 | PR modifica `CLAUDE.md`; baseline é "arquivo de instrução reportado" |

### Direção B — Arquivo inócuo reportado como superfície executável (falso-positivo)

Este é o vetor 🔴 da REQ (AC5, AC6). O mais traiçoeiro: torna o relatório inútil por ruído.

**Fixture gratuita nesta árvore:** o arquivo que você está lendo agora
(`docs/seguranca/2026-08-26-modelo-de-ameaca-da-superficie-executavel-de-checkout.md`) contém a
string literal `.claude/settings.json`. `internal/generators/agentfiles.go` também contém todos os
8 paths de hook como string literal. Um grep ingênuo por path de hook reportaria ambos como
superfície executável.

O mesmo ocorre com:
- `docs/cli-parity.md` — documenta todos os paths de hook com nomes de arquivo literais
- `scripts/check-agent-hooks-parity.sh` — define `CLIS` e itera sobre paths literais
- Qualquer test file que cite os paths

| Sabotagem | Onde entra | Gate que deveria acusar |
|---|---|---|
| Grep de path literal sem discriminar se o arquivo *é* o wiring ou apenas *menciona* o path | Implementação de Wave 1 | `scripts/check-audit-surface.sh` cenário FP-1: PR modifica `docs/cli-parity.md` com novo path de hook; baseline é "não reportado como superfície executável" |
| Grep de script name sem verificar se a referência é wiring ativo | Implementação de Wave 1 | `scripts/check-audit-surface.sh` cenário FP-2: PR adiciona `docs/*.md` citando nome de script; baseline é "não reportado" |

**O discriminante correto:** o arquivo é um arquivo de wiring executável se e somente se (a) está
no path esperado para o runtime (ex.: `.claude/settings.json`, não `docs/algo.md`) E (b) tem
estrutura de wiring (campo `hooks` ou equivalente), não apenas menciona o path como string.

### 3.1 Superfícies de gate

As superfícies de falsificação desta REQ devem ser verificadas por:
- `scripts/check-audit-surface.sh` (a criar no Wave 2) — cenários FN-1..5 e FP-1..2
- `scripts/check-gates-falsify.sh` — cenários a serem adicionados ao total existente (172+)

O gate existente `trackfw validate` não cobre nenhuma destas superfícies — **[medido]** com
fixture da REQ: `trackfw validate` retorna 1 warning (dir ADR inexistente), silêncio sobre hook.
Esta ausência é o caso de uso que justifica o comando novo.

---

## 4. Residual declarado

### 4.1 O comando depende de o mantenedor rodar

**Este é o limite estrutural da decisão e está nos olhos abertos:** o trackfw não está no caminho
de execução do hook. Qualquer solução é antes (informar) ou depois (constatar), nunca durante. O
ADR o nomeia explicitamente e é aceito.

**O que estreita a janela sem violar o Negative scope:** o Wave 1 pode aceitar um ref como
argumento e auditar o conteúdo **diretamente do object database** (`git show <ref>:<path>`) sem
fazer checkout na worktree. Isso permite ao mantenedor rodar `trackfw audit-surface <pr-ref>` antes
de fazer `git checkout FETCH_HEAD` — a janela cai de "checkout → primeiro uso da ferramenta" para
zero. Esta variante não bloqueia nada, não julga conteúdo, e não requer worktree. Recomendação
para o Wave 1.

### 4.2 O comando não protege quem já abriu o repositório

Se o hook já executou, o comando constata, não previne. Declarado no ADR. Não é coberto por esta
REQ nem violável por nenhuma implementação dentro do Negative scope.

### 4.3 Não classifica conteúdo como hostil

O comando nomeia *o que executa*, não decide se é malicioso. Esta fronteira é intencional (AC5)
e é a razão pela qual a REQ é estruturalmente viável: heurística de conteúdo é a fuga conhecida de
toda allowlist de shell. O residual é: um mantenedor que não leia o relatório não obtém proteção.

### 4.4 Superfícies de instrução têm semântica diferente de scripts de shell

`CLAUDE.md` e `AGENTS.md` versionados não executam código de shell; eles instrui o agente. Um PR
hostil que os modifique produz execução indireta (o agente age sob instrução hostil). Esta
superfície está no inventário mas o Wave 1 deve reportá-la com rótulo distinto ("arquivo de
instrução" vs "script executado por hook") para que o mantenedor saiba a que tipo de risco está
exposto.

### 4.5 Runtime global não é escopo desta REQ

Hooks globais (`~/.claude/settings.json`, `~/.codex/hooks.json`, etc.) não são entregáveis por
PR — estão fora do escopo por construção. O comando audita apenas o ref comparado à base.

### 4.6 `.claude/settings.local.json` não é versionado

**[medido]** `git ls-files .claude/settings.local.json` → não encontrado. A lista de permissões
(`permissions.allow`) que ela contém não é entregável por PR nesta árvore. Se outro projeto
versionar este arquivo, a largura da permissão seria superfície — o Wave 1 deve inventariá-la
quando presente.

### 4.7 Pre-existing finding fora de escopo

O vetor roadmap-title-newline → forged `## Wave N` → execução de gate pelo `barrier` (encontrado
em 2026-08-23) não é coberto por esta REQ. É superfície executável por PR (o roadmap chega via
PR) que usa caminho diferente do que o Wave 1 audita (gate do `barrier`, não hook de agente).
Precisa de REQ própria.
