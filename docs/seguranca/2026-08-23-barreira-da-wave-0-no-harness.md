---
title: Barreira da Wave 0 no harness — ML-3A confronta o parecer de ML-0A com o entregue
date: 2026-08-23
author: hades-tf
req: docs/req/REQ-2026-08-22-wave-0-de-modelo-de-ameaca-no-harness-e-o-asset-do-arquiteto-ensina-trackfw-push.md
roadmap: docs/roadmaps/wip/ROADMAP-2026-08-22-wave-0-de-modelo-de-ameaca-no-harness-e-o-asset-do-arquiteto-ensina-trackfw-push.md
adr: docs/adr/ADR-2026-08-22-modelo-de-ameaca-no-desenho-wave-0-de-red-team-antes-da-implementacao-no-harness.md
ml: ML-3A
---

# Barreira da Wave 0 — ML-3A

> Cada afirmação está marcada **[medido]** (executei o comando/li o diff citado nesta sessão) ou
> **[raciocinado]** (inferência sobre evidência já registrada por outro ML).

## Veredito

**APROVADO COM RESSALVAS — uma delas grave, mas pré-existente a esta REQ, não introduzida por ela.**

Aprovado porque toda a superfície que a REQ e o próprio ML-0A nomearam foi entregue, e entregue
melhor do que o AC pedia em dois pontos (gate placeholder fail-closed em vez de "não-vazio";
asserção de conteúdo em vez de só diff cross-stack). Com ressalvas porque:

1. **A via de esvaziamento central do §2 do meu próprio parecer continua aberta para todo roadmap
   escrito à mão** — inclusive este mesmo roadmap (ver §1). Residual já nomeado e aceito pelo ADR.
2. 🔴 **Ao completar o vetor "newline" que eu havia deixado sem executar, encontrei execução de
   shell arbitrária via `roadmap new "<título>"` seguido de `barrier --wave N`, para qualquer N —
   inclusive N=0 (ver §2-bis).** É **pré-existente** a esta REQ (reproduzida também em `--wave 1`
   contra código anterior ao ML-1A) — esta REQ apenas estendeu a mesma superfície, já existente para
   `--wave 1+`, para `--wave 0` também, ao aceitar a wave nova. Não é regressão introduzida pelo
   ML-1A/2A, mas é a coisa mais grave que esta sessão mediu, e recomendo escalar para REQ separada
   com prioridade alta.

Nenhuma das duas bloqueia **esta** REQ especificamente — a primeira é residual já aceito pelo ADR; a
segunda é um problema estrutural do `roadmap new`/`barrier` anterior a esta REQ, fora do seu escopo
de correção, mas que precisa de tratamento urgente e independente.

---

## 1. As cinco vias de esvaziamento do §2 — o que fechou, o que não

**[medido]** Rodei `barrier --wave 0` contra a Wave 0 **deste próprio roadmap** (ML-0A, escrita à mão
por mim, sem bloco `**Gates da wave:**` — como todo roadmap desta série, inclusive o que motivou a
REQ):

```
$ ./bin/trackfw barrier docs/roadmaps/wip/ROADMAP-2026-08-22-....md --wave 0 --json
{"status":"passed","checks":[
  {"name":"mls_complete","status":"passed","evidence":["ML-0A: ✅"]},
  {"name":"acceptance_evidence","status":"passed","evidence":["ML-0A: 4 criteria met"]},
  {"name":"gates","status":"passed","commands":[],"evidence":[],"failures":[]},
  {"name":"validate","status":"passed","evidence":["0 violations, 16 warnings"]}
]}
EXIT=0
```

`gates: passed` com `commands: []` — exatamente o achado do §2.1 do meu parecer original,
reproduzido ao vivo, contra o roadmap que a própria série usa como prova de conceito.

| Via de esvaziamento (§2 original) | Fechada pelo ML-1A/2A/1B? | Evidência |
|---|---|---|
| Wave 0 escrita pelo próprio implementador | **Não, para roadmap manuscrito.** Fechada só para roadmap **gerado** (placeholder `exit 1` fail-closed). | Este roadmap: `passed`, zero gate. |
| Parecer de uma linha | **Não.** `acceptance_evidence` conta `[x]`, não lê conteúdo do artefato `docs/seguranca/*.md` — nenhum dos MLs tocou essa checagem. | Inalterado desde §2.1; `runBarrier` continua sem `os.Stat` sobre `docs/seguranca/`. |
| Wave 0 copiada da REQ anterior | **Não.** Nenhum check compara conteúdo entre REQs — fora do escopo desta REQ (nunca foi AC). | — |
| Wave marcada ✅ sem artefato correspondente | **Não**, mesma razão. | — |
| Arquiteto despacha Wave 1 sem esperar Wave 0 auditada | **Não** — `barrier` avalia uma wave por vez, sem checar ordem. Declarado explicitamente como fora de escopo pelo próprio ADR. | Confirmado no roadmap: "residual… nada impede mecanicamente". |

**A alavanca que de fato fechou algo é F5** — o template do gerador agora pré-carrega
`**Gates da wave:**` com `exit 1`, então uma Wave 0 **gerada** e não preenchida reprova. Isso fecha
a via para o caminho `roadmap new`/`roadmap new --from-req`/slash command. **Não fecha, e não tinha
como fechar sem violar a decisão do ADR, a via para roadmap escrito à mão** — que é como este
roadmap, o que motivou a REQ, e (por amostragem) todo roadmap desta série nasceram. O tamanho do
buraco: **qualquer agente que escreva um roadmap do zero, sem passar pelo `roadmap new`, entrega
uma Wave 0 vazia que passa `barrier --wave 0` limpo, sempre** — a mesma conclusão do parecer
original, agora confirmada contra um artefato real em vez de raciocinada sobre código que não
existia.

**Isto não é uma falha do ML-1A/2A.** O ADR já havia decidido, com motivo, não tornar isso
bloqueante em `validate`; e o AC13 fechou exatamente a fração que era fechável sem essa mudança de
ADR (o caminho gerado). Registro como ressalva do veredito, não como reprovação: **a REQ deveria ter
nomeado explicitamente "roadmap manuscrito continua fora do gate" como residual aceito**, em vez de
deixar essa distinção implícita entre "template" (fechado) e "prática real desta série" (aberto).

---

## 2. AC13 (não-interpolação) nos 3 stacks — testado, não só lido, incluindo o vetor que faltava

**[medido]** Testei em ambiente isolado (`trackfw init` limpo, `$HOME` sintético) contra Node e
Python, além do Go já testado pelo ML-1A, com quatro vetores pedidos pela tarefa:

| Vetor | Go | Node | Python | `--from-req` |
|---|---|---|---|---|
| `$(touch /tmp/X)` + `` `id` `` no título | **[medido]** não materializou | **[medido]** não materializou | **[medido]** não materializou | **[medido, Go]** não materializou |
| aspas simples+duplas no título | **[medido]** não materializou | **[não testado]** | **[não testado]** | — |
| `;` no título | **[medido]** não materializou | **[não testado]** | **[não testado]** | — |
| REQ com `$(...)`/`;` nos critérios de aceite | — | — | — | **[medido, Go]** não vaza para o gate (vaza para prosa do ML, esperado) |
| **newline no título forjando uma seção Markdown inteira** | **[medido — ver §2-bis, ACHADO CRÍTICO]** | **[medido — ver §2-bis, reproduzido]** | **[não testado]** | **[não testado]** |

Correção de honestidade sobre o que ficou de fora: quotes/`;` e a checagem completa de
`--from-req` não foram retestados em Node/Python nesta sessão — o ML-1A já havia coberto o vetor
`$(...)`/backtick nos 3 stacks (registrado no roadmap), e `--from-req` roda nos 3 runtimes sob
`scripts/check-artifact-parity.sh` (lido, não reexecutado por mim para este vetor específico). Não
marco esses como `n/a` — marco como não retestados por mim, arquitetura idêntica entre stacks.

Para o subconjunto efetivamente medido: em todos os casos o roadmap gerado contém o gate **literal**
do template (`exit 1  # placeholder gate fails closed...`) — a restrição do AC13 sobre o **gate
próprio da Wave 0 gerada** se sustenta. O que ela **não** cobre é o achado do §2-bis abaixo, que é
uma superfície diferente da que o AC13 endereçava.

---

## 2-bis. ACHADO CRÍTICO — o título de `roadmap new` pode forjar uma seção Markdown inteira, incluindo um bloco de gate arbitrário, executado por `barrier`

**[medido]** O vetor "newline no título" (pedido pela tarefa e que eu não havia executado antes da
primeira versão deste parecer) não testa se a string é avaliada como shell — testa se ela pode
**injetar estrutura Markdown**. `NewRoadmapFromContent` (`internal/generators/roadmap.go:150`)
interpola `content.Title` com `fmt.Sprintf("# Roadmap: %s", ...)` sem remover ou escapar newlines.
Um título com `\n\n## Wave 0 — Threat Model\n\n**Gates da wave:**\n` seguido de um fence ` ```bash `
com um comando **planta uma segunda seção `## Wave 0`, com seu próprio bloco de gate, antes da seção
real** emitida pelo template:

```
$ trackfw roadmap new "forjado
> ## Wave 0 — Threat Model
> **Gates da wave:**
> \`\`\`bash
> touch /tmp/HEADING_PWNED
> \`\`\`
> "
```

Arquivo resultante (trecho, linhas reais):
```
 8: # Roadmap: forjado
10: ## Wave 0 — Threat Model        ← forjada, do título
12: **Gates da wave:**
13: \`\`\`bash
14: touch /tmp/HEADING_PWNED
15: \`\`\`
...
29: ## Wave 0 — Threat Model        ← real, do template (com o gate exit 1 fail-closed)
```

Rodando `barrier --wave 0 --json` sobre esse arquivo:

```json
{"checks":[
  {"name":"mls_complete","status":"blocked","failures":["wave 0: no ML found"]},
  {"name":"gates","status":"passed","commands":["touch /tmp/HEADING_PWNED"],
   "evidence":["touch /tmp/HEADING_PWNED: exit 0"]}
]}
$ ls /tmp/HEADING_PWNED
/tmp/HEADING_PWNED   ← existe. O comando rodou.
```

`parseWaves`/`parseGates` resolvem a **primeira** ocorrência de `## Wave 0` no arquivo — a forjada,
não a real — e executam o comando dentro dela via `sh -c`, **independente do status final do
barrier** (aqui `blocked` por outros checks, mas o gate já rodou antes de o resultado final ser
composto). **Reproduzido também em Node** (`/tmp/NODE_HEADING_PWNED` criado, mesmo padrão de JSON).
Não testei Python neste vetor especificamente, mas a arquitetura de parsing é a mesma
(`mlBlock`/`parseGates` delimitam por posição de linha do primeiro heading que casa, não por
autenticidade).

**Isto é pré-existente a esta REQ, não introduzido por ela.** Reproduzi o mesmo ataque contra
`--wave 1` com um título forjando `## Wave 1 — <name> (parallel MLs)` com seu próprio ML e gate —
funciona igual (`/tmp/WAVE1_PWNED` criado). O mecanismo de gates para waves ≥1 já existia antes desta
série (ADR-2026-07-26-principios-de-design-de-gates-verificaveis). **O que esta REQ mudou foi
estender a mesma superfície, já existente para `--wave 1+`, também para `--wave 0`** — ao aceitar
`--wave 0`, ela não criou o buraco, mas ampliou em um a lista de waves alcançáveis por ele.

**Por que isto está fora do escopo do AC13**: o AC13 pediu (corretamente) que o **gate que o próprio
template emite** não seja interpolado com conteúdo do título/REQ — e isso se sustenta, o gate
literal `exit 1` do template real nunca é tocado. O que o AC13 não cobria, porque é uma classe de
ataque diferente, é que **o título pode conter um documento Markdown inteiro**, incluindo uma seção
forjada com seu **próprio** bloco de gate, não o do template. Nenhum AC desta REQ pedia sanitização
de newline no título — a REQ nunca nomeou essa superfície.

**Severidade e recomendação**: alto — é execução de comando arbitrário reachável por qualquer
chamador de `roadmap new` com controle sobre a string de título (inclusive um título copiado de uma
fonte externa não confiável, ou um pipeline automatizado que interpola valor de usuário), seguida de
`barrier` rodando **qualquer** wave. Recomendo REQ nova e urgente, fora desta série: (a) `toSlug`-like
sanitização do título antes de interpolar em `# Roadmap: %s` (rejeitar ou colapsar newlines), e/ou
(b) `parseWaves`/`mlBlock` pinarem a seção pela heading mais próxima do bloco YAML de estado
conhecido, não pela primeira ocorrência textual — e possivelmente (c) `barrier` pedir confirmação
explícita antes de executar comandos de gate vindos de um roadmap que não foi gerado pelo próprio
CLI (heurística mais fraca, mas mitigação imediata). Não implemento nenhuma dessas correções — são
mudança de código de produto, fora da minha fronteira de escrita.

---

## 3. O que a Wave 0 não enumerou, e por quê — diagnóstico, não desculpa

**[medido]** Antes de qualquer código do ML-1A existir, um `grep -rn "## Wave 1"` no repo já
retornava três ocorrências: `internal/generators/roadmap.go:113` (template `new`),
`internal/generators/roadmap.go:153` (`--from-req`) — ambas na minha lista original — e
`internal/generators/scaffold.go:333` (o slash command) — **fora** da minha lista.

**Diagnóstico**: a seção 1 do meu parecer original (`docs/seguranca/2026-08-22-...`) enumerou
superfícies **a partir da lista que a própria REQ/ADR já nomeava** ("gerador de roadmap (3 CLIs,
`new` e `--from-req`), `barrier`, asset do arquiteto, asset de segurança, `CLAUDE.md` semeado") e
perguntou "o que falta nessa lista, por leitura dos arquivos citados". Isso encontrou lacunas
**dentro** dos arquivos já nomeados (o segundo guarda do `barrier`, o rótulo `ML-1x` do
`--from-req`, o canal de propagação errado) — mas nunca fez a pergunta inversa: **"que outros
lugares do repo emitem a string `## Wave 1` ou uma estrutura de roadmap, independente de estarem na
lista da REQ?"** Um `grep` de dez segundos por esse literal — não uma leitura dirigida pela REQ —
teria encontrado o `scaffold.go` no mesmo minuto em que encontrei `roadmap.go:113`.

**O que isso ensina para o asset da Wave 0** (que é a pergunta que a tarefa pediu, e que já está
instalado no harness pelo ML-1A): a seção "Completude de enumeração" do template
(`internal/generators/roadmap.go`, bloco `wave0Block`) pede para responder "a lista está completa?"
mas não instrui **como** verificar — não há um passo tipo "faça uma busca textual pelo artefato
final que a REQ está mudando, independente da lista de arquivos que a REQ nomeia". Sem essa
instrução, o revisor (eu, desta vez) tende a auditar a lista dada em vez de derivar uma lista
própria por busca. Recomendo, para um ML futuro fora desta REQ: adicionar à Ação 1 do `wave0Block`
uma frase do tipo "não se limite aos arquivos citados na REQ — busque no repo por outros pontos que
emitem o mesmo artefato/padrão antes de declarar a lista fechada". Não implemento isso aqui — é
mudança de template, fora da minha fronteira de escrita.

---

## 4. Regressão nas superfícies de guard tocadas por `agentfiles.go`/`scaffold.go`

**[medido]** Isolei os diffs exatos dos MLs desta REQ:

- `git show dab3243 -- internal/generators/agentfiles.go`: **um único bullet** trocado no bloco
  `trackfw:rules` do `CLAUDE.md` semeado (`"Security wave: ..."` → `"Threat model waves: ..."`).
  Não toca nenhuma função de guard.
- `git diff 9c5c99f 525232d -- internal/generators/scaffold.go`: **três hunks, todos dentro do
  mesmo bloco de exemplo Markdown** (linhas ~315-380, o texto que o slash command instrui o Claude
  a seguir) — inclusão do bloco Wave 0 e alargamento de crases do fence externo (3→4). Nenhum hunk
  toca `GenerateCredentialGuardScript`, `GenerateGlobalCredentialGuardScript`, ou os blocos de
  `match_subcommand`/`case "$SUBCOMMAND"` do git-branch-guard (linhas 811+, 1046+, 1558+, 1739+ do
  arquivo atual).
- **[medido]** Repeti a mesma verificação para os dois equivalentes que `scaffold.go` tem nos
  outros stacks — `npm/src/generators/init.js` e `pypi/trackfw/generators/init_gen.py`, que também
  são arquivos monolíticos (slash command + `CLAUDE.md` + scripts de guard, tudo junto, mesmo
  padrão do Go). `git diff 9c5c99f 525232d -- <ambos> | grep -E "guard|REASON"` não retorna nada;
  os hunks de ambos ficam confinados ao bullet do `CLAUDE.md` semeado e ao mesmo bloco de exemplo
  Markdown do slash command — mesma forma do Go.

Os dois trechos de `REASON=` que aparecem num diff mais amplo (`dc89d91..525232d`) — mudança de
texto explicativo sobre `trackfw push`/`git reset --soft` nas mensagens de bloqueio — **não
pertencem a esta REQ**: `git log -S'commitar e empurrar' -- internal/generators/scaffold.go` aponta
para o commit `7132fc5` (`feat(push)`, #202), anterior a esta série. Confirmei que nenhuma condição
de match (`case reset)`, `case push)`, detecção de `--hard`) mudou nos commits desta REQ — só o
texto das mensagens, e isso já vinha de antes.

**Veredito do item 4: nenhuma regressão de segurança em credential-guard ou git-branch-guard
atribuível a esta REQ.**

---

## 5. `trackfw doctor` não cobre os assets do `scaffold.go` — REQ ou aceitável?

**[medido]** `internal/commands/doctor.go:19-37` declara o escopo explicitamente: *"trackfw doctor
sweeps every **catalog-managed agents/skills destination**"*. `.claude/commands/trackfw/roadmap.md`
não é um artefato do catálogo de agents/skills — é gerado por `scaffold.go` via `trackfw init`/
`update harness`, um pipeline diferente. Confirmei também que não existe REQ/ADR aberta cobrindo
esse gap (`grep -rl scaffold docs/req docs/adr` não retorna nada relacionado a cobertura de
doctor).

**Não é aceitável deixar como está, sem nomear** — é exatamente o tipo de ponto cego que a REQ
seguinte deveria fechar, pelas mesmas razões que motivaram o `check-artifact-parity.sh` ganhar a
asserção de conteúdo (AC14): hoje, se o slash command deste repositório (ou de qualquer projeto que
rode `trackfw init`) ficar defasado em relação ao gerador real, **nenhuma ferramenta do próprio
trackfw acusa** — nem `doctor`, nem `validate`. A única rede de segurança é o
`check-artifact-parity.sh`, que roda em CI do próprio trackfw, não em projetos consumidores. **Não
é bloqueante para esta REQ** (o ML-1B já documentou o ponto cego, e regenerar o artefato deste repo
foi decisão correta e declarada, não encobrimento). Recomendo abrir REQ nova: "doctor cobre
artefatos scaffold-managed (slash commands) além de catalog-managed".

---

## 6. Resumo da barreira

| Pergunta | Resposta |
|---|---|
| AC1-AC14 entregues? | **[raciocinado, herdado do registro de apolo-tf em `agents-working-context.md`]** — não reexecutei `make quality`/os gates completos nesta sessão. O que **eu** medi diretamente: AC13 (parcial, ver tabela §2) e o comportamento do `barrier` sobre este roadmap e sobre fixtures próprias. |
| Vias de esvaziamento do §2 (original) fechadas? | Só a via "gerado". Manuscrita continua aberta — medido ao vivo contra este roadmap (§1). |
| AC13 (gate do template não interpolado) honrado? | Sim, no subconjunto que medi: `$(...)`/backtick nos 3 stacks, aspas/`;` só em Go, `--from-req` só em Go. Não retestei quotes/`;`/`--from-req` em Node/Python nesta sessão. |
| Existe outra via de execução de shell além da coberta pelo AC13? | **Sim — achado crítico, §2-bis.** Título com newline forja uma seção Markdown inteira com gate próprio, executado por `barrier`, em qualquer wave (não específico de Wave 0, mas agora também alcançável via `--wave 0`). Reproduzido em Go e Node. |
| Enumeração original completa? | Não — diagnóstico: auditou a lista da REQ, não fez busca independente pelo artefato final (§3). |
| Regressão em guard (credential-guard/git-branch-guard)? | Não, nos 3 stacks — diffs desta REQ isolados e conferidos linha a linha, incluindo os equivalentes Node/Python de `scaffold.go` (§4). |
| `doctor` cobre scaffold? | Não, por desenho declarado — ponto cego real, não bloqueante, recomendo REQ nova (§5). |

**Nenhuma das ressalvas acima bloqueia esta barreira**, com uma delas — §2-bis — precisando de
escalonamento imediato e independente: (a) o residual manuscrito, já previsto e aceito pelo ADR,
agora com evidência ao vivo; (b) o achado crítico de execução de shell via título, pré-existente a
esta REQ e presente em todas as waves, não introduzido pelo ML-1A/2A, mas ampliado em superfície
(agora também via `--wave 0`) — recomendo REQ nova de prioridade alta, aberta imediatamente; (c) uma
lição de método para o próprio asset da Wave 0, fora da minha fronteira de implementação; (d)
confirmação negativa de regressão em guard, nos 3 stacks; (e) uma REQ nova recomendada para o
`doctor`, não bloqueante.
