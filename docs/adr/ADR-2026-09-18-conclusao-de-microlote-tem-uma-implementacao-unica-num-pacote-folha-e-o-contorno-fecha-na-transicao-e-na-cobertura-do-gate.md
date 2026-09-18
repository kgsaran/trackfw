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
| roadmaps em `done/` com pelo menos um ML `⬜`/`🔄` (fence-aware) | **27 (14%)** |
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

**27 dos 192 roadmaps em `done/` acenderiam de imediato.** Fechar isso exige baseline ou carve-out —
e a campanha do #387, mergeada ontem, foi inteiramente sobre o fato de que **o mecanismo de
afrouxamento é a superfície de ataque**. Construir uma regra cuja única forma de nascer verde é um
carve-out é devolver o interruptor.

Fica registrado como reconsiderável quando (e se) o corpus histórico for sanado por outra via — mas
não como dívida desta REQ.

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
