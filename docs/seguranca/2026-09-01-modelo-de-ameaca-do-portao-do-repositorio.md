---
status: draft
date: 2026-09-01
author: "hades-tf"
ml: "ML-0A"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-01-o-repositorio-do-trackfw-sob-os-cuidados-do-trackfw.md"
req: "docs/req/REQ-2026-09-01-o-repositorio-do-trackfw-nao-esta-sob-os-cuidados-do-trackfw.md"
adr: "docs/adr/ADR-2026-09-01-o-repositorio-do-trackfw-e-governado-pelo-trackfw-com-o-mesmo-rigor-que-o-produto-vende.md"
---

# Modelo de ameaça do portão do repositório

> KG — este documento é análise. Nenhuma linha de configuração, hook ou workflow foi alterada para
> produzi-lo. Toda evidência abaixo veio de leitura de código (`internal/...`) e de consultas
> read-only (`gh api`, `gh run list`, `git config --get`) contra o estado real do repositório e da
> `main` em 2026-09-01/02.

## 1. Enumeração — o que o trackfw instala em terceiros, e o que este repositório usa

Critério por item: **existe aqui? está ativo? está atualizado?** — e a distinção que decide o
roadmap: *"não usamos e deveríamos"* vs. *"não usamos e há razão"*.

### 1.1 Portão de merge da `main` (medido via `gh api repos/kgsaran/trackfw/branches/main/protection`)

| campo | valor medido | veredito |
|---|---|---|
| `required_status_checks` | **chave ausente** do JSON — não é `{}`, não existe | 🔴 **deveríamos**. Qualquer PR mergeia com CI vermelho. |
| `required_pull_request_reviews.required_approving_review_count` | `0` | ⚪ **há razão condicionada** — ver §3, este projeto tem um mantenedor; ver o argumento sobre auto-aprovação. |
| `enforce_admins.enabled` | `false` | 🔴 **deveríamos** — ver §3, argumento completo. |
| `allow_force_pushes.enabled` | `false` | ✅ já correto, nada a fazer |
| `allow_deletions.enabled` | `false` | ✅ já correto, nada a fazer |
| `required_linear_history.enabled` | `false` | ⚪ fora de escopo desta REQ — não citado no ADR/REQ, não decido aqui |

Achado positivo que o ADR não menciona: **force-push e deleção da `main` já estão bloqueados** no
nível do GitHub, independente de `required_status_checks`. Nem tudo está desligado.

### 1.2 CI existente — o material que `required_status_checks` vai referenciar

Workflows presentes: `quality.yml`, `trackfw-gate.yml`, `trackfw-validate.yml`, `windows-probe.yml`,
`deploy-docs.yml`, `release.yml`. Jobs que rodam em `pull_request` (via `quality.yml` +
`trackfw-gate.yml` + `trackfw-validate.yml`), medidos ao vivo no HEAD da `main` e no PR #241 aberto:

```
go · node · python (3.10) · python (3.12) · package-smoke · windows-integrations-resolve ·
windows-full-suites [continue-on-error: true] · windows-defect-reproduction [continue-on-error: true] ·
parity (needs: go, node, python, package-smoke, windows-integrations-resolve) ·
governance  (×3 — ver 1.3)
```

Confirmado ao vivo (`gh api .../commits/main/check-runs`, 2026-09-02): `windows-full-suites` e
`windows-defect-reproduction` estão **`failure`** no HEAD da `main` agora mesmo — não é uma hipótese
do ADR, é o estado real, e o comentário no próprio `quality.yml` (linhas 116-176) documenta por quê:
são o instrumento de medição da issue #216, `continue-on-error: true` é **temporário por desenho**, e
o job existe para **reprovar até os 11 defeitos fecharem**. Exigi-los travaria todo merge — igual ao
que o ADR previu, só que agora com o número de defeitos e a razão documentados no próprio job.

### 1.3 🔴 Achado que o ADR não tinha — colisão de nome no check `governance`

Medido ao vivo no PR #241 (`gh api .../commits/<sha>/check-runs`): **três check-runs distintos, com
IDs diferentes, todos chamados `governance`.**

Causa, encontrada lendo o código: dois mecanismos de instalação diferentes escrevem workflows com o
mesmo `job:` id `governance`:

- `internal/generators/scaffold.go:1917` (`buildGitHubActionsWorkflowContent`, escrito por
  `init`/`update`) → `.github/workflows/trackfw-gate.yml`, `on: pull_request`.
- `internal/generators/scaffold_doctor.go:45` (`BuildDiscoverGitHubActionsWorkflowContent`, escrito
  por `discover --init`) → `.github/workflows/trackfw-validate.yml`, **`on: [push, pull_request]`**.

Um push numa branch de PR dispara `trackfw-gate.yml` uma vez e `trackfw-validate.yml` duas vezes
(`push` + `pull_request`) — três execuções, mesmo nome de check em todas. Não verifiquei a semântica
exata do GitHub para múltiplos check-runs homônimos (se `required_status_checks` exige que **todos**
os `governance` fiquem verdes ou só o mais recente) — não afirmo isso no documento porque não medi.
O que **é** certo, independente dessa semântica: **a string `governance` em
`required_status_checks` não consegue dizer qual dos três você quis dizer**, e quem audita um
"governance failed" no PR precisa abrir os três runs para saber qual caiu. **Isto é um
pré-requisito da Wave 1, não um detalhe**: os dois geradores precisam de `job:` ids distintos
(ex.: `governance-gate` / `governance-validate`) antes de qualquer um dos dois entrar em
`required_status_checks` — e a semântica real do GitHub para nomes duplicados precisa ser medida
num PR real antes de decidir se basta renomear ou se também é preciso reduzir para um único
mecanismo.

Ambos os arquivos **existem neste repositório e batem com o que os geradores produziriam** — exceto
por uma divergência deliberada, item seguinte.

### 1.4 `trackfw-validate.yml` diverge do template — e é a divergência certa

O template (`BuildDiscoverGitHubActionsWorkflowContent`) escreve `go install
github.com/kgsaran/trackfw/cmd/trackfw@v<versão>` — instala o **release publicado**. O arquivo real
neste repositório faz `go build -o /usr/local/bin/trackfw ./cmd/trackfw` — compila **o código do
próprio branch**. `trackfw doctor` compararia os dois e reportaria `scaffold-divergent`.

⚪ **Há razão, não é lacuna.** Este é o único repositório onde validar com o binário instalado via
release seria **auto-referencialmente errado**: um PR que corrige um bug em `trackfw validate`
precisa que o CI rode a versão do PR, não a última tag publicada — senão o gate nunca testa a
mudança que está sendo revisada. Registro isto porque é exatamente o tipo de "não usamos e há razão"
que a REQ pediu para não tratar como ruído, mas também porque `trackfw doctor`, no estado atual (ver
§1.7), reportaria isto como divergência sem essa explicação — um falso positivo que precisa de uma
exceção nomeada se algum dia formos rodar `doctor` neste repositório em CI.

### 1.5 Guards — vivem em dois escopos, nenhum protege humano

```
.claude/settings.json (projeto)  → git-branch-guard.sh (PreToolUse, matcher Bash) +
                                    attention-signal/cleanup (PreToolUse/PostToolUse, matcher AskUserQuestion)
~/.claude/settings.json (global) → credential-guard.sh (PreToolUse + PostToolUse, matcher Bash)
.git/hooks/                      → vazio (só *.sample)
core.hooksPath                   → /dev/null
```

**Correção de precisão ao ADR**: o ADR diz "credential-guard e git-branch-guard vivem SÓ aqui" —
medindo, isso é impreciso quanto ao escopo. `credential-guard` está instalado **globalmente**
(`~/.claude/settings.json`, confirmado por grep direto), não no projeto — e o próprio gerador
(`internal/generators/agentfiles.go`, `globalCredentialGuardInstalledClaude()`) **deliberadamente
pula** escrever uma segunda cópia no projeto quando a global já existe, para não duplicar. Isso não é
um gap: é o dedup funcionando como projetado. `git-branch-guard` sim vive só no projeto. A conclusão
do ADR sobrevive à correção: **nenhum dos dois escopos, projeto ou global, protege um humano rodando
git puro** — ambos só disparam dentro do harness de agente (`PreToolUse`/matcher `Bash` do Claude
Code), que não existe para um terminal humano.

🔴 **A distinção mais importante desta seção, e a que decide se a Wave 1 fecha o argumento
motivador da REQ.** O AC3 do REQ pede "guards ativos para humanos" e cita como evidência seis
bloqueios reais nesta sessão: `git stash` em worktree compartilhado, `checkout --` destrutivo, `push`
bruto. Fui ler o único mecanismo de hook Git real que o produto tem hoje
(`internal/generators/scaffold.go:1985`, `generateCommitMsgHook`) para ver se ele cobre isso.
**Não cobre, e não pode:**

1. **O único gerador de hook Git existente é `commit-msg`, e só escreve para `husky` ou
   `lefthook`** (`cfg.Hooks` só aceita esses dois valores mais `"none"` —
   `internal/commands/init.go:225-227`; `generateCommitMsgHook` faz `switch cfg.Hooks { case
   "husky": ...; case "lefthook": ... }`, sem `default`). **Este repositório é Go puro, sem
   husky nem lefthook** — não existe hoje nenhum caminho no produto para gerar um hook Git real
   para um projeto como o próprio trackfw. AC3 não é "ligar algo que já existe": é construir uma
   capacidade nova (um hook nativo via `core.hooksPath`, independente de gerenciador de pacote
   Node), porque a única já existente é inaplicável aqui.
2. **Mesmo com essa capacidade nova construída, o hook `commit-msg` não é a mesma classe de
   proteção que bloqueou o agente seis vezes.** `commit-msg` verifica o *conteúdo da mensagem*
   de um commit que já vai acontecer — é controle de metadado de governança (referência a REQ).
   Os três incidentes citados como motivação (`stash`, `checkout --`, `push` bruto) são
   **comandos destrutivos**, não commits malformados. O git-branch-guard do agente funciona
   porque intercepta *qualquer* comando de shell antes de rodar (`PreToolUse`, matcher `Bash`) —
   um gancho que só existe no harness do Claude Code, não no git. **O git não tem um hook
   equivalente**: os hooks nativos do git disparam em eventos de ciclo de vida específicos
   (`pre-commit`, `commit-msg`, `pre-push`, `pre-rebase`, `post-checkout`...), nunca "antes de
   qualquer subcomando". `git stash` e `checkout --` **não têm hook nativo correspondente** —
   não há forma honesta de replicar essa proteção via `core.hooksPath` para um humano no
   terminal. `pre-push` é o único hook nativo que mapeia para um dos três incidentes citados
   (bloquear push direto para `main`, redundante com `allow_force_pushes`/branch protection já
   ativos no lado do servidor).

**Consequência para o roadmap**: se a Wave 1 entregar "AC3 concluído" instalando só um hook
`commit-msg` novo, isso fecha a letra do AC3 mas **não fecha o argumento que a REQ usou para
justificá-lo** — dois dos três incidentes citados continuam sem nenhum equivalente humano possível
via git hooks, e isso deve ser dito explicitamente na Wave 1, não descoberto depois.

### 1.6 Cadeia exigida (`CONTRIBUTING.md`, template de PR)

Confirmado ausentes os dois (`find . -maxdepth 1 -iname CONTRIBUTING*` e
`.github/*PULL_REQUEST*` vazios). ⚪ **Não é lacuna desta REQ** — o Negative Scope da própria REQ
já nomeia isto (`REQ-2026-09-01-projeto-nao-publica-a-exigencia-de-governanca-...`, pausada,
volta depois desta, com a razão explícita: publicar a regra antes de ter o portão repetiria o erro
de contrato sem gate). Listo aqui só para fechar a enumeração pedida, não como item a resolver.

### 1.7 `agents install` / `skills install` / manifesto do catálogo

`.claude/agents/`, `.claude/skills/` e `.trackfw/manifest.json` **não existem neste repositório**
(escopo de projeto). Os agentes nomeados no `squad:` do roadmap (`hades-tf`, `ares-tf`, `apolo-tf`)
correspondem a definições que **existem em `~/.claude/agents/`** (escopo global — confirmado por
`ls`, arquivos como `trackfw-security.md`, `trackfw-architect.md`), não em `.claude/agents/` do
projeto. ⚪ **Há razão, condicionada**: definição de agente pessoal do mantenedor, coerente com o
`CLAUDE.md` global do usuário (que descreve `~/.claude/agents/trackfw-architect.md` como o ponto de
orquestração). Não verifico se isso deveria ser diferente — é uma escolha de escopo de quem mantém a
máquina, fora do que este repositório pode ou deve impor. Sinalizo como item a **confirmar
explicitamente com você**, não como veredito fechado: se a intenção for permitir que qualquer
contribuidor externo rode os mesmos agentes, `.claude/agents/` do projeto teria que existir; se a
intenção é só o seu fluxo pessoal, está correto como está.

### 1.8 `trackfw update harness` e `integrations install` — os dois alvos que faltavam na primeira varredura

- **`update harness`** (`internal/commands/update_harness.go:1-6`, comentário do próprio arquivo):
  "the global-scope counterpart to `trackfw update`... it never requires trackfw.yaml or a project
  working directory, and it never touches anything outside the user's home directory." **Não tem
  artefato voltado a este repositório por desenho** — não é um "não usamos", é "não se aplica". O
  `.claude/settings.json` deste repositório (git-branch-guard) foi escrito pelo caminho de projeto
  (`init`/`update`, via `InjectHooksDetected`/`InjectClaudeHooks`), não por `update harness`.
- **`integrations install`** (`internal/commands/integrations_flags.go`,
  `internal/commands/integrations_thirdparty.go`): mecanismo de instalar skills/agentes/integrações
  de terceiros com quarentena e provenance (`docs/cli-parity.md`, seção já documentada no
  `CLAUDE.md` do projeto). Sem uso neste repositório — coerente com §1.7: os artefatos que
  existiriam aqui são exatamente `.claude/agents/`/`.claude/skills/`, já tratados.

## 2. Veredito — quais checks exigir em `required_status_checks`, e o custo de cada escolha

**Exigir** (todos correspondem a jobs sem `continue-on-error`, determinísticos, e que hoje terminam
`success` de forma estável no HEAD medido):

- `go`, `node`, `python (3.10)`, `python (3.12)`, `package-smoke` — build e teste dos 3 CLIs; recusar
  isto é recusar a Regra Dura de Paridade do próprio `CLAUDE.md` do projeto.
- `windows-integrations-resolve` — **não** é `continue-on-error`; o próprio comentário no
  `quality.yml` o chama de "o único guard honesto" para o bug de path Windows que é invisível em
  `ubuntu-latest`. Custo de exigir: nenhum além do já pago hoje — já roda em todo PR e passa.
- `parity` — depende (`needs:`) de `go, node, python, package-smoke,
  windows-integrations-resolve`; exigi-lo sozinho já implica os cinco anteriores em prática, mas
  listar os cinco explicitamente evita que uma falha de infraestrutura no `parity` (ex.: timeout)
  esconda qual dos cinco realmente quebrou.
- `governance` — **só depois** de resolvido o item 1.3 (nomes únicos por mecanismo). Exigir a
  string ambígua hoje significa que o portão vai travar ou destravar sem que o log deixe claro qual
  dos dois `trackfw validate` (via `trackfw-gate.yml` ou via `trackfw-validate.yml`) decidiu.

**Não exigir, deliberadamente** (confirmado `continue-on-error: true` no `quality.yml`, e
confirmado `failure` ao vivo no HEAD da `main` agora):

- `windows-full-suites`, `windows-defect-reproduction` — nascem vermelhos por desenho, são o
  instrumento de medição da issue #216. Exigi-los trava **todo** merge até os 11 defeitos fecharem —
  o oposto do que o instrumento existe para permitir, e o próprio job documenta isso na cabeça do
  arquivo. **Custo de não exigir**: PRs podem mergear sem que estes dois jobs jamais tenham passado —
  aceito, porque é exatamente o comportamento desenhado, não uma omissão.

**Custo agregado de ligar o portão com esta lista**: nenhum job hoje "born red" seria bloqueado; os
jobs que hoje passam continuam passando. O único atrito real é a colisão de nome em `governance`
(1.3), que precisa ser resolvida **antes**, não depois, de configurar `required_status_checks` — do
contrário o primeiro PR legítimo pode travar por ambiguidade de nome, não por falha real, e o
primeiro instinto será desligar o portão de novo.

## 3. Veredito — `enforce_admins`

**O argumento, não só a conclusão.**

Primeiro, por que `required_approving_review_count` fica em `0` independente de qualquer decisão
sobre `enforce_admins`: este é um repositório de um mantenedor. O GitHub não permite auto-aprovação
do próprio PR nas regras padrão de review; exigir `required_approving_review_count > 0` sem um
segundo revisor humano **trava todo merge para sempre**, não apenas os do admin. Isso não é um
argumento sobre `enforce_admins` — é o motivo pelo qual a variável de revisão sai da mesa antes de
chegar lá.

Com a revisão fixada em `0`, `enforce_admins` decide exatamente **uma coisa**: se a exigência de
`required_status_checks` (uma vez configurada, §2) **vale também para o admin**, ou só para quem não
é admin.

**A favor de `enforce_admins: true`** — o argumento empírico, e ele é direto: a REQ lista quatro
incidentes reais nesta sessão (roadmap deixado em `wip` com trabalho mergeado, cinco vezes; branch
paralela violando regra própria; resíduo de PoC commitado duas vezes; troca de branch com agente
vivo movendo arquivos para a branch errada) e nomeia quem cometeu os quatro: o próprio mantenedor,
que é o admin do repositório. **Nenhum desses apareceu no CI verde** — não porque o CI não teria
detectado nada relacionado, mas porque, com `enforce_admins: false`, o admin nunca precisou esperar
o CI para mergear. Se `required_status_checks` for ligado (§2) e `enforce_admins` continuar `false`,
o portão passa a valer para todo contribuidor externo e **continua opcional exatamente para a
pessoa cujos erros motivaram a REQ**. Isso não é uma lacuna residual — é a REQ inteira sendo
resolvida para todo mundo, menos para quem a escreveu.

**Contra `enforce_admins: true`** — a escotilha de emergência tem valor real e específico: se um
check obrigatório quebrar por razão de infraestrutura (outage do GitHub Actions, uma dependência de
CI fora do ar, ou — cenário mais concreto para este repositório — um bug no próprio
`trackfw validate` que faz o job `governance` reprovar PRs legítimos), um mantenedor único não tem
a quem pedir bypass. Sem `enforce_admins`, ele fica bloqueado de corrigir o próprio bloqueio.

**O que decide**: o GitHub oferece um caminho para o segundo cenário que não exige deixar
`enforce_admins` permanentemente `false` — alternar `enforce_admins` via `gh api` (ou a UI) no
momento da emergência, resolver, e religar. Essa alternância **é ela própria uma ação de admin,
auditável no log de auditoria da organização/repositório**, e sem revisão de terceiros — o que é o
resíduo real desta escolha (nomeado explicitamente na §5), não uma mitigação que o torna
equivalente a manter a chave sempre aberta. A diferença entre os dois estados não é "nunca há
bypass" vs. "sempre há bypass" — é **bypass registrado e excepcional** vs. **bypass permanente e
silencioso**, e é essa diferença que a REQ pede (AC5: "decisão registrada, não omissão").

**Recomendação que eu defenderia**: `enforce_admins: true`, com o procedimento de flip-temporário
documentado (não implementado por mim) como o mecanismo de emergência nomeado. É a única escolha
consistente com a evidência empírica que a própria REQ cita como motivação — manter `false` resolve
o problema para terceiros e deixa aberto exatamente o padrão de falha que gerou a REQ.

## 4. O que não pode ser bloqueado — falsificação simétrica

Cada controle abaixo precisa passar nas duas direções antes de entrar em produção: **bloqueia o
destrutivo/inválido** E **deixa passar o legítimo**. Nomeio o que não pode quebrar:

- **`git commit` comum, fora de `feat/*`/`fix/*`.** O script de `commit-msg` já se auto-escopa
  (`case "$BRANCH" in feat/*|fix/*) ... esac`) — commits em `main`, `chore/*`, `docs/*`, ou
  qualquer branch fora do padrão passam sem checar REQ. Isso precisa continuar assim; expandir o
  escopo do hook para todo commit quebraria a exceção de trivialidade do §7 do `CLAUDE.md` global
  (typo, doc-only, revert) que **não exige REQ por desenho**.
- **Commits em branches `feat/*`/`fix/*` sem `REQ:` na mensagem, mas que já têm REQ referenciada no
  roadmap/PR** (ex.: quem esquece a linha na mensagem mas a rastreabilidade já existe no
  frontmatter). Se o hook virar realidade, ele precisa de uma via de saída documentada
  (`--no-verify` continua existindo no git; o hook não deve ser o único lugar onde a exigência é
  verificada — `trackfw validate`/`governance` já checa isso de forma auditável no PR, o hook é
  conveniência local, não a fonte de verdade).
- **Push para branches não-`main`** — `required_status_checks` e `enforce_admins` só têm efeito em
  merges para a branch protegida. Pushes para branches de trabalho, scratch, ou de agentes em
  worktree não podem ficar bloqueados por checks que ainda não terminaram — isso já é o
  comportamento padrão do GitHub, mas vale registrar como invariante a não regredir se algum dia se
  cogitar proteger outras branches.
- **PR que corrige o próprio `governance`.** Se um bug em `trackfw validate` fizer o check
  obrigatório reprovar PRs legítimos, o PR que conserta esse bug roda a versão **antiga** (já
  commitada) do `trackfw validate` contra o próprio código novo — o padrão auto-referencial já visto
  em `trackfw-validate.yml` (§1.4). Isto não é resolvido por `enforce_admins` sozinho; é o cenário
  concreto para o qual a escotilha de emergência de §3 existe.
- **`windows-full-suites`/`windows-defect-reproduction` continuam nunca bloqueando** — já coberto em
  §2, repito aqui porque é a metade simétrica mais fácil de esquecer ao configurar
  `required_status_checks` na UI do GitHub (a lista de checks disponíveis na tela de configuração
  não distingue visualmente `continue-on-error` de obrigatório-elegível).
- **Deleção/force-push da `main` já protegidos** (§1.1) — não regredir isso ao tocar
  `required_status_checks`; são configurações independentes na mesma API.

## 5. Residual declarado

Mesmo com Wave 1 completa (portão ligado com a lista do §2, `enforce_admins: true`, guard novo de
`commit-msg` para humanos), o seguinte **permanece sem controle** depois desta REQ:

1. **Dois dos três incidentes que motivaram AC3 continuam sem proteção alguma** (§1.5,
   item 2) — `git stash` e `checkout --` destrutivos não têm hook nativo do git correspondente.
   Um humano rodando esses comandos em `main` ou num worktree compartilhado segue tão exposto
   quanto hoje. Fechar isso exigiria uma capa adicional (wrapper de shell, alias, ou política
   documentada) fora do que `core.hooksPath` pode oferecer — não é escopo natural desta REQ,
   registro para não ser descoberto como surpresa na Wave 1.
2. **`required_approving_review_count` permanece `0` indefinidamente** — ponto único de falha
   aceito conscientemente (§3): nenhum PR, nem os de contribuidores externos, tem revisão humana
   obrigatória além do autor. Se o projeto ganhar um segundo mantenedor, isto precisa ser
   revisitado — não é uma decisão que envelhece bem sozinha.
3. **O flip temporário de `enforce_admins` para emergências é, ele mesmo, uma ação de admin sem
   revisão de terceiros** (§3) — resolve o "nunca há controle" mas não resolve "controle
   auto-aplicado". É o trade-off aceito, não uma lacuna a fechar.
4. **A colisão de nome em `governance` (§1.3) não tem a semântica do GitHub para múltiplos
   check-runs homônimos verificada.** Antes de configurar `required_status_checks`, isto precisa
   ser medido num PR real (não presumido), e os dois geradores precisam de `job:` ids distintos.
5. **`trackfw doctor` não tem nenhuma visão sobre o que este documento mediu** — nem branch
   protection (API do GitHub, requer rede + token), nem `core.hooksPath`/hooks reais do git. Hoje
   `doctor` (`internal/commands/doctor.go`, `runDoctor`) só compara **manifesto do catálogo** e
   **templates de scaffold no disco** — as três lacunas originais do ADR (portão, guards para
   humanos, cadeia exigida) são **invisíveis** para o `doctor` atual, e as duas primeiras não são
   nem arquivo: uma vive na API do GitHub, a outra em configuração de `git config`. **AC6 não é
   "adicionar mais um check ao doctor que já existe" — é dar ao `doctor` uma segunda modalidade
   de verificação (rede + autenticação) que ele nunca teve**, uma mudança de superfície maior do
   que o texto da AC sugere. Nomeio isto explicitamente porque é o gap que a Wave 2 mais
   provavelmente vai subestimar de partida.
6. **`.claude/agents/`/`.claude/skills/` de projeto continuam ausentes** (§1.7) — decisão que
   depende de confirmação sua (pessoal vs. compartilhável), não fechada por este documento.
7. **`trackfw-validate.yml` deste repositório segue divergente do template `discover` por
   desenho** (§1.4) — se algum dia `doctor` rodar contra este repositório (item 5), essa
   divergência precisa de uma exceção nomeada, ou vai gerar um falso positivo permanente.
8. **Não há retroatividade** — commits e merges já existentes na `main` não são reauditados; o
   portão vale só a partir de quando for ligado (fora de escopo por decisão explícita do REQ).
