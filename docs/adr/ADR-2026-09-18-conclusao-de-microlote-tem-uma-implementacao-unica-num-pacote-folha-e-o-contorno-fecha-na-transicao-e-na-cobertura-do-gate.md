---
status: Accepted
date: 2026-09-18
author: "Zeus (arquiteto) / KG (decisão)"
---

# ADR: conclusão de microlote tem uma implementação única num pacote folha, e o contorno fecha na transição e na cobertura do gate

> Date: 2026-09-18 | Status: Accepted

**Issue:** #392

## Context

`trackfw roadmap new` gera um scaffold com `## Wave 0`, `### ML-0A`, `### ML-1A`, critérios vazios e
um gate que falha fechado:

```bash
# Wave 0 gate — replace this placeholder with a project-specific check before
# marking ML-0A done. Do not remove the gate; replace its command (AC13).
exit 1  # placeholder gate fails closed until ML-0A replaces it — see docs/cli-parity.md
```

Quem escreve o roadmap **prepende** o conteúdo real acima do scaffold e não apaga o resto. O arquivo
segue a vida com duas `Wave 0`, dois `ML-1A`, ou microlotes `⬜ Pendente` — e chega a `done/`.

### O corpus, medido em 2026-09-18 (192 roadmaps em `done/`)

| medição | valor |
|---|---|
| roadmaps em `done/` com ML não concluído pelo predicado canônico | **27 (14%)** |
| ...que o `ParseWaves` rejeita por rótulo com maiúscula (`3-Py`) | **4** |
| total que bloquearia a transição, com fail-safe de parse | **31** |
| roadmaps em `done/` com `## Wave 0` | **38 de 192** |
| roadmaps em `done/` com o gate placeholder `exit 1` intacto | **4** |
| interseção dos dois | 2 |

A contagem fence-aware difere da contagem crua por `grep` em 2 arquivos, ambos falsos-positivos —
ocorrência dentro de cerca de código. A diferença importa: ela é a razão de o predicado novo ter de
respeitar cercas, não de ser um `grep`.

### Por que nenhum gate pega

- **`trackfw validate`** tem **30 regras** e **uma só** olha conteúdo de roadmap: `wip_acceptance`
  (`validator.go:2299-2319`), que verifica apenas a **presença do heading** de aceite no documento
  inteiro. Não lê microlote, não lê status, não lê gate.
- **`trackfw roadmap move`** (`internal/generators/roadmap.go:524-605`) **não valida nada** para o
  destino `done`: valida o nome do estado, acha o arquivo, deriva o namespace, `os.Rename`. É a
  transição onde a informação existe, e ela não é consultada.
- **`trackfw barrier`** pega — mas só sob demanda, por wave, e quem já fechou o roadmap não roda
  barrier de novo. Nas três ocorrências ele foi executado por acaso, não por processo.
- **`trackfw doctor`** não olha roadmaps (`ADR-2026-08-27` cobre artefatos de *scaffold* por
  comparação com template; roadmap é artefato **autorado**, não coberto).

### 🔴 O achado que reordena o problema: três implementações de "ML concluído?"

O defeito não é a ausência de um check. É que **cada consumidor reimplementa a pergunta**, e as
implementações discordam:

| sítio | como decide | qualidade |
|---|---|---|
| `barrier` — `statusIsComplete` (`internal/commands/barrier.go:300`) | **primeiro token**, vocabulário fechado, VS16 stripado, máscara de cerca | correto, governado pela `ADR-2026-08-29` |
| `serve` — `parseMLProgress` (`internal/serve/api_board.go:137-168`) | `strings.Contains(trimmed, "✅")`, **sem máscara de cerca** | 🔴 é **literalmente o bug de substring que a `ADR-2026-08-29` decidiu contra** |
| `validate` — `contentHasMarker` (`validator.go:2312`) | só existência de heading | não responde a pergunta |

E há um **quarto dialeto, no lado da escrita**: `internal/generators/scaffold.go:350-384` — a prosa
que ensina o agente pelo slash command `/trackfw:roadmap` — escreve `**Status:** pending`, enquanto
o gerador Go escreve `**Status:** ⬜ Pendente`. **`pending` não está no `statusVocabulary` do
barrier.** Duas fontes que ensinam o mesmo campo, uma delas ensinando um valor que o verificador
rejeita. É o defeito da `ADR-2026-07-31` e da `ADR-2026-08-29` outra vez, num terceiro sítio.

O parser bom existe, é fence-aware e endurecido — e está **preso em `package commands`**, que
importa `internal/validator`. O `validator` não pode importá-lo de volta: é ciclo. Verificado:
`grep "trackfw/internal/commands" internal/validator/*.go` → **zero**.

### 🔴 Segundo achado, medido por mim nesta análise: o rótulo duplicado é invisível

`parseWaves` e `parseMLs` retornam **slices** — duplicatas são preservadas, não colapsadas. Mas a
seleção da wave alvo (`barrier.go:877-882`) faz `break` no **primeiro** rótulo que casa.

Consequência: um roadmap com scaffold duplicado tem **duas `## Wave 0`**, e o `barrier --wave 0`
examina **apenas a primeira**. Na ocorrência #1 do #392 o arquiteto marcou ✅ na cópia errada e o
`barrier` reprovou — mas **por ordenação, não por detecção**. Se a cópia marcada fosse a primeira, o
`barrier` teria passado com um `ML-0A` pendente logo abaixo. **Nenhuma das quatro direções propostas
no issue cobre este caso.**

## Decision

### 1. Extrair o parser de roadmap para um pacote folha

Nasce `internal/roadmapdoc`, sem dependência de `commands` nem de `validator`, contendo o maquinário
que hoje vive em `barrier.go`: `splitRoadmapLines`, `fenceMask`/`detectFenceMarker`, `parseWaves`,
`parseMLs`, `mlStatusMarker`, `statusIsComplete` e o `statusVocabulary`, `acceptanceEvaluate`,
`parseGates`, e os regexes associados.

`commands` (barrier), `validator` e `serve` passam a importar esse pacote. **É a decisão estrutural
desta ADR**: enquanto a resposta a "este ML está concluído?" for reimplementada por consumidor, todo
gate novo nasce com o risco de ser o quarto dialeto.

🔴 **A extração é refactor puro, sem mudança de comportamento** — e isso tem de ser *provado*, não
afirmado. Ver AC de identidade na REQ.

### 2. O `serve` e o `scaffold` convergem para o vocabulário canônico

- `parseMLProgress` passa a usar `roadmapdoc`. O board do `serve` mostra progresso errado **hoje**:
  `Contains("✅")` casa dentro de cerca de código e casa `✅` em qualquer posição da linha.
- `scaffold.go` deixa de ensinar `**Status:** pending` e passa a ensinar o que o gerador escreve e o
  verificador aceita.

**Sequenciamento obrigatório:** a convergência do `scaffold` vem **antes ou junto** do gate da
decisão 3. Se o gate entrar primeiro, todo roadmap autorado pelo slash command nasce bloqueado na
transição, e o gate vira incômodo — que é o efeito perverso nomeado pela `ADR-2026-08-17`
(*"o usuário desliga o guard por incômodo, e guard desligado não guarda"*).

### 3. `roadmap move <nome> done` recusa quando há microlote não concluído

O gate vive na **transição**, que é onde a informação existe e onde o custo retroativo é **zero** —
os 27 roadmaps já em `done/` não são tocados.

O predicado é o de `roadmapdoc`: um ML está concluído quando o **primeiro token** do restante da
linha `**Status:**` pertence ao vocabulário de conclusão. Cercas de código são mascaradas.

**A recusa nomeia cada ML pendente**, com rótulo e linha. Guard que reprova sem dizer o quê é guard
que o usuário contorna.

### 4. O gate de wave fecha por **perda de cobertura**, não por presença de placeholder

🔴 Esta é a lição direta do ML-2B da REQ do #387, onde o discriminante `len(files) == 0` reprovava o
diretório vazio e liberava a fachada de uma linha.

`parseGates` trata **wave sem bloco `**Gates da wave:**` como zero gates, o que é legal** — o check
passa. Logo, um discriminante que detectasse "o texto do placeholder ainda está aí" entregaria
exatamente o mesmo defeito:

| tier | custo do contorno | com discriminante "placeholder presente" | com discriminante "perda de cobertura" |
|---|---|---|---|
| deixar `exit 1` intacto | 0 | reprova | reprova |
| **apagar o bloco de gates inteiro** | 1 linha | **passa** | reprova |
| 🔴 **renomear `## Wave 0 — X` para `## X`** (tier 1b, achado da Wave 0) | 1 linha | **passa** | **passa** — falha vacuosamente; fechado pela decisão 8 |

O discriminante é: **a wave perdeu o gate que o template lhe deu**. Vale em **qualquer estado**, não
só na transição.

### 5. As decisões 3 e 4 são complementares, e a razão fica escrita

A decisão 3 é um gate de **transição**, e a transição é **opcional** — `mv` ou `git mv` direto
contornam. A decisão 4 vale em qualquer estado e fecha o contorno. Uma fecha cedo; a outra fecha o
desvio. Adotar só uma delas deixa metade do caminho aberto.

### 6. Rótulo duplicado de Wave/ML é violação própria

Um roadmap não pode ter dois `## Wave 0` nem dois `### ML-1A`. Hoje o `barrier` lê a primeira e
ignora o resto silenciosamente; a ambiguidade **falha aberta**. Passa a falhar fechada e nomeada.

Medido em 2026-09-18: **4 em `done/`** e **1 em `blocked/`**, zero em `backlog/`, `analyzing/` e
`abandoned/`. Confirmado à mão num deles — `ROADMAP-2026-09-17-sync-enumera-req-por-caminho-literal`
tem o scaffold **inteiro** duplicado (linhas 13-60 e 78-103), e o `barrier --wave 0` lê apenas o
primeiro bloco.

> 🔴 **Nota de método.** A primeira medição deste número deu **0 em todos os estados**. O `awk` do
> macOS (versão 20200816) **não suporta `ENDFILE`**, e a regra que acumulava o resultado por arquivo
> nunca disparou. Unanimidade num corpus é sinal de instrumento quebrado, não de corpus limpo — foi
> o que o contra-braço revelou.

### 6-bis. O placeholder é legítimo em `backlog/` e `analyzing/`

Medição da união (placeholder + rótulo duplicado) em **todos** os estados: 16 arquivos, dos quais
**6 em `backlog/`**. Roadmap em `backlog` que ainda carrega o gate `exit 1` **não é defeito** — é um
roadmap que ninguém começou a trabalhar, e o placeholder está cumprindo exatamente o papel de falhar
fechado até ser substituído.

Portanto o discriminante da decisão 4 é **sensível ao estado**: cobra a partir de `wip`, não em
`backlog`/`analyzing`. Cobrar em `backlog` transformaria o gate em ruído sobre artefatos sadios — o
efeito perverso da `ADR-2026-08-17`.

### 6-ter. Os sítios medidos são corrigidos nesta REQ, não registrados

Em `done/`: 4 com placeholder + 4 com rótulo duplicado, com 2 em comum → **6 arquivos**.

Pela Regra Dura de Causa Raiz, defeito **medido e localizado** se corrige na REQ vigente. Seis
arquivos é factível. É a diferença qualitativa em relação aos 27 da decisão 7: aqueles são ausência
histórica de convenção; estes são **scaffold residual**, que é exatamente o defeito que esta REQ
fecha. Deixá-los seria fechar a REQ com sítios conhecidos e não corrigidos — o achado A1 da auditoria
externa de 2026-09-05, que este projeto já pagou uma vez.

### 7. 🔴 O que esta ADR NÃO decide: regra do `validate` sobre `done/`

A direção 1 do issue — regra no `validate` que reprova roadmap em `done/` com ML não concluído — é
**rejeitada nesta ADR**, com o número que a rejeita:

**27 dos 192 roadmaps em `done/` acenderiam de imediato** (31 com o fail-safe de parse). Fechar isso exige baseline ou carve-out —
e a campanha do #387, mergeada ontem, foi inteiramente sobre o fato de que **o mecanismo de
afrouxamento é a superfície de ataque**. Construir uma regra cuja única forma de nascer verde é um
carve-out é devolver o interruptor.

Fica registrado como reconsiderável quando (e se) o corpus histórico for sanado por outra via — mas
não como dívida desta REQ.

### 8. Wave 0 é exigida por estado, e `done/` não é reavaliado — achado da Wave 0

`waveHeadingRe` é `^## Wave (\S+) `. Renomear `## Wave 0 — Threat Model` para `## Threat Model` faz
a wave **desaparecer do parser**, e o discriminante da decisão 4 falha **vacuosamente**. É um tier de
custo idêntico ao de apagar o bloco de gates, e **eu não o havia previsto** — veio do parecer de
`hades-tf`.

🔴 A medição proíbe a solução óbvia: apenas **38 dos 192** roadmaps de `done/` têm `## Wave 0`, porque
a convenção só existe desde agosto. Exigi-la retroativamente acenderia **154** — pior que os 27 que a
decisão 7 rejeita, pelo mesmo raciocínio.

Adotado: a exigência vale em `wip`/`blocked` e na **transição** para `done`; `done/` não é
reavaliado. Custo retroativo medido: **1 arquivo** (`blocked/`). A assimetria é deliberada, e está
aqui escrita para que ninguém a "conserte" depois por parecer inconsistente.

### 9. Três categorias de status, não duas

`StatusIsComplete` responde uma pergunta binária, e o corpus tem três situações:

| categoria | valores medidos | efeito na transição para `done` |
|---|---|---|
| concluído | `✅`, `done`, `Concluído`, `CONCLUIDO` | libera |
| encerrado sem conclusão | `ABANDONADO`, `🚫 Abandonado`, `❌ Cancelado` | **libera** |
| pendente | `⬜`, `🔄`, `pending`, `PENDENTE`, `❌ Bloqueado` | **bloqueia** |

`❌ Bloqueado` bloqueia porque um ML bloqueado dentro de um roadmap `done` é exatamente a contradição
que esta ADR fecha. Abandonado libera porque encerramento explícito é decisão registrada, não
esquecimento — reprová-lo seria o falso-positivo da `ADR-2026-08-17`.

### 10. 🔴 Correção de método: eu adotei um número que não reproduzi

A versão anterior desta ADR dizia **32**. Esse número veio do parecer da Wave 0 e **eu o incorporei
sem medi-lo**, reescrevendo ADR, REQ e roadmap — e registrando no working context que eu havia
corrigido uma "contradição minha". A contradição não existia: o **27** original estava certo.

Medido por mim depois, com o pacote já extraído e contra o corpus intocado desta branch
(`git diff main...HEAD -- docs/roadmaps/done/` → **0 arquivos**, o que falsifica a explicação de
"corpus drift"): **27**. Distribuição dos tokens não-conclusão: `⬜` 101, `🔄` 5, `ABANDONADO` 1,
`❌` 1, `➡️` 1, sem linha de status 1 — os tokens "extras" caem em arquivos que **já** continham `⬜`,
e por isso não somam arquivo novo. Foi esse o passo que o parecer pulou.

A lição não é "desconfie do subagente". É que **medição de terceiro se verifica antes de virar
decisão**, e eu apliquei essa regra ao corpus e não à minha própria fonte. O agravante é que eu
escrevi uma autocrítica em cima de um erro que não havia cometido — o registro ficou duplamente
errado.

### 11. `WaveLabelRe` rejeita rótulo com maiúscula, e o `barrier` falha em 4 roadmaps reais

`WaveLabelRe` é `^\d+(?:-[a-z0-9]+)?$`. Quatro roadmaps usam `## Wave 3-Py` e são **rejeitados por
inteiro** pelo `ParseWaves`. É a mesma causa desta ADR — dialeto que o verificador não aceita — e por
isso entra nesta REQ, não numa nova (Regra Dura de Causa Raiz).

Decidido: o sufixo passa a ser aceito **insensível a caixa**, e o erro de parse deixa de ser
silencioso. 🔴 Até lá, e como postura permanente, **erro de parse conta como pendência** — falhar
aberto num gate de governança é o defeito que o #387 inteiro combateu.

**Emenda ML-1D (2026-09-18, REQ #392):** Dois arquivos reais usam `## Wave 1b` (sufixo sem hífen)
e continuavam invisíveis após ML-1C. `WaveLabelRe` passa de `^\d+(?:-[a-zA-Z0-9]+)?$` para
`^\d+(?:-?[a-zA-Z0-9]+)?$` — o hífen antes do sufixo torna-se **opcional**. `SplitWaveLabel("1b")`
devolve `(1, "b")`, idêntico a `SplitWaveLabel("1-b")`, portanto
`CompareWaveLabels("1b", "1-b") == 0`. Rótulos puramente alfabéticos (`reaberta`) e rótulos de
letra única sem dígito (`X`) continuam inválidos — o prefixo inteiro (`^\d+`) é obrigatório.
Quatro arquivos de corpus com `1b` ou `reaberta`: os dois com `1b` agora passam na gramática (este
ML); os dois com `reaberta` permanecem inválidos e são corrigidos no ML-4A.

### 12. Cascade isolation: heading malformada não aborta o documento inteiro

**Emenda ADR-2026-07-29 decisão 16** (princípio preservado, remédio corrigido — ML-1D/ML-1E,
REQ #392).

A decisão 16 foi tomada com base na premissa de que isolar o erro tornaria os MLs da wave
malformada "invisíveis". A emenda ML-1D mediu que essa premissa é parcialmente falsa para as
waves válidas; a emenda ML-1E corrigiu o remédio para que o princípio da decisão 16 seja
integralmente preservado:

**O que o ML-1D fez (mantido):**

- `## Wave X` ainda é um `## ` heading — o parser de fim-de-bloco
  (`strings.HasPrefix(lines[j], "## ")`) detecta-o corretamente e termina a wave anterior.
  As waves válidas antes e depois não são corrompidas.
- `ParseWaves` passa a devolver `([]WaveBlock, []MalformedWave)` em vez de `([]WaveBlock, error)`.
  Cada heading inválida gera um `MalformedWave{Line, Token}` e o parse continua.
- `--wave X` ainda é rejeitado em exit 2 na validação de flag, muito antes de chegar ao parser.

**O que o ML-1E adicionou (corretivo obrigatório):**

O ML-1D imprimia cada `MalformedWave.Error()` em `stderr` **mas não entrava em nenhum check**.
O veredito dependia só de `mls_complete`/`acceptance_evidence`/`gates`/`validate`. Resultado:
`barrier --wave 1` podia emitir `status: "passed"` mesmo que o documento contivesse `## Wave X`
com MLs `⬜ Pendente` — exatamente o cenário que a decisão 16 proibia.

O ML-1E adiciona um **check `wave_headings`** (primeiro na lista de checks) que:
- É `"blocked"` quando há headings malformadas, nomeando linha e token no campo `failures`.
- É `"passed"` quando o documento não tem headings malformadas.
- Entra no veredito final: `status: "blocked"` enquanto houver heading malformada.
- As waves válidas **continuam sendo avaliadas** — a cegueira não volta.

**Propriedade preservada da decisão 16:** o `barrier` nunca emite `status: "passed"` enquanto
houver heading malformada no documento. O princípio ("reprovar alto") é inviolável. O remédio
mudou de "abortar o documento (exit 2)" para "check que bloqueia (exit 1)".

**Razão da mudança de remédio:** o remédio original cegava o `barrier` em 2 roadmaps reais
(`convergencia-do-harness` e `serve-amarra`) com `## Wave 1b`. O documento inteiro era
descartado por exit 2, e nenhuma wave — nem as válidas — era avaliada. Trabalho não auditado,
por razão oposta: não por ignorar o malformado, mas por descartar tudo com ele.

## Consequences

**Positivas**

- Uma implementação de "ML concluído?", num pacote sem ciclo, disponível aos três consumidores. O
  quarto dialeto deixa de ser possível por construção.
- O board do `serve` passa a mostrar progresso correto — defeito que existe hoje e ninguém reportou.
- Custo retroativo **zero** para a decisão 3; **4 arquivos** para a decisão 4.
- O contorno por `mv` direto fica coberto, porque a decisão 4 não depende da transição.

**Negativas / aceitas**

- A extração toca o `barrier`, que é gate de release. Mitigado pelo AC de identidade byte-a-byte da
  saída `--json` sobre todo o corpus, capturado **antes** de qualquer edição.
- Roadmaps já em `done/` com ML pendente continuam lá. Aceito e nomeado: sanar 27 arquivos
  retroativamente é risco desproporcional, e a decisão 7 explica por que a alternativa é pior.
- A decisão 3 recusa transições que hoje passam. É o ponto — mas depende da decisão 2 ter convergido
  o scaffold antes, sob pena de virar incômodo.

## Alternativas consideradas

- **Direção 3 do issue (`roadmap new` deixa de emitir scaffold pré-preenchido).** Barrada por ADR: a
  `ADR-2026-07-31` decidiu **explicitamente** que a seção consolidada de aceite é placeholder a
  preencher, e não agregação automática, para não criar duas fontes de verdade. "Placeholder existe"
  não pode ser, sozinho, o discriminante de violação.
- **Chamar o `barrier` a partir do `validator`.** Impossível sem ciclo de import — daí a extração.
- **Duplicar o parser no `validator`.** Seria o quarto dialeto, criando o defeito que esta ADR fecha.

## Fora de escopo, com a diferença de mecanismo escrita

Pela Regra Dura de Causa Raiz, o que fica de fora precisa ter a **causa diferente demonstrada**, não
apenas o escopo declarado estreito:

- **Seção `## Linked Roadmap` perdida na reescrita manual da REQ do #387.** Discriminante: a regra
  `req_roadmap_sync` **já detecta** esse caso — ela emitiu o warning, e foi assim que eu achei. O
  default dela é `warning` (`validator.go:169-195`). É lacuna de **severidade e atenção**, não de
  detecção. O #392 é o oposto: não-detectado por nenhuma regra. Causas distintas.
- **`_ = os.WriteFile` em `roadmap.go:581-585`**, que engole falha da sincronização de `status:`.
  Está **dentro** da função que a decisão 3 modifica. Entra nesta REQ por proximidade e por ser
  falha-aberta na mesma transição — decidido explicitamente, não por omissão.

## Breaking Change — ML-4B/ML-4C: `roadmap_wave0_required` promovido a `error`

**O que mudou (ML-4B, REQ #392):** a regra `roadmap_wave0_required` passou de ausente dos
`ruleDefaults` (tratada como `warning` implícito) para `error` explícito. Roadmaps em `wip/` **sem
`## Wave 0`** agora fazem `trackfw validate` falhar com exit 1.

**Efeito em cascata sobre `trackfw barrier`:** o `barrier` executa `validate` como último check
(após `wave_headings`, `mls_complete`, `acceptance_evidence`, `gates`). Com `roadmap_wave0_required`
em `error`, qualquer roadmap em `wip/` sem `## Wave 0` — ainda que Wave 1 esteja 100% verde — faz
o `barrier` sair com exit 1 (check `validate` blocked).

**Remédio:** adicionar `## Wave 0 — Threat Model` ao roadmap, com um gate real (não `exit 1`
placeholder). O gate `exit 0` é válido para fixtures de teste; projetos reais devem substituí-lo
por um check de threat model concreto.

**O que NÃO se deve fazer:** relaxar a regra (editar `ruleDefaults`, `trackfw.yaml` ou a
severidade). O template `roadmap new` já emite `## Wave 0` com gate `exit 1` (placeholder
falha-fechado); a ruptura afeta apenas roadmaps criados antes do ML-4B ou que omitiram Wave 0
propositalmente.

**Corretivo de fixtures (ML-4C):** todos os fixtures de `scripts/check-barrier.sh` que criavam
roadmaps em `wip/` sem `## Wave 0` foram atualizados para incluí-la com gate `exit 0`. A fixture S9
(malformed heading AFTER target) teve `WANT9` atualizado de `"line 15"` para `"line 26"` porque a
inserção de Wave 0 desloca o heading malformado 11 linhas abaixo.
