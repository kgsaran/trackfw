---
status: done
date: 2026-07-27
req: "docs/req/REQ-2026-07-27-convergencia-dos-templates-de-artefato-do-cli-python.md"
squad: ""
---

# Roadmap: convergencia dos templates de artefato do CLI Python

> Created: 2026-07-27 | Status: done

## Context

REQ: `docs/req/REQ-2026-07-27-convergencia-dos-templates-de-artefato-do-cli-python.md`
ADR: `docs/adr/ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md`

O CLI Python gera `roadmap`, `req` e `adr` em formato próprio. O efeito não é cosmético: **duas
regras do validator passam por ausência de match** para artefatos gerados pelo Python.

| # | Regra | Por que fica cega |
|:-:|---|---|
| 1 | `req_blocked_by_draft_adr` + `sync` | procuram `"Status: Open"`; template Python usa tabela `\| Status \| Open \|` |
| 2 | `blocked_by_draft_adr` | `adrIsDraft` procura `"Status: Draft"`; template Python usa `## Status` + `status: Draft` no frontmatter |

É P2 — degradação silenciosa — no ADR de princípios de gates. Sobreviveu porque **nenhum gate jamais
executou um gerador**: `check-cli-parity.sh` compara nomes de subcomando extraídos do `--help` e
nunca lê um arquivo produzido.

### Formato canônico

Go/Node, em inglês. Já declarado em `docs/schema/*.json`; nenhum ADR declara a variante Python como
intencional. A decisão de idioma está na REQ e **não deve ser reaberta** durante a execução.

### Ordem das waves — não é arbitrária

A Wave 1 escreve os testes negativos **antes** da convergência, de propósito. Convergir primeiro faria
`Status: Open` e `Status: Draft` passarem a casar por efeito colateral, e perderíamos a evidência de
que as regras estavam cegas. O teste precisa ser visto **falhando** contra o formato Python atual.

```
Wave 1 (1A) ─ barrier ─> Wave 2 (2A) ─ barrier ─> Wave 3 (3A)
   expõe as regras cegas    converge templates     gate de saída
```

## Wave 1 — Expor as regras cegas (agente único)

> Dependências: nenhuma. **Deve ser concluída e auditada antes da Wave 2.**

### ML-1A — Testes negativos que provam a cegueira das regras

**Status:** done
**Files affected:** testes do validator nos 3 CLIs — `internal/validator/validator_test.go`,
`npm/tests/validator.test.js`, `pypi/tests/test_validator.py`

**Actions:**
1. **Teste para `blocked_by_draft_adr`**: montar um ADR **no formato Python atual** (frontmatter com
   `status: Draft`, seção `## Status` com `Draft`, **sem** a linha `> Date: … | Status: Draft`) e uma
   REQ que o referencie. A regra **deve** acusar o bloqueio. Hoje não acusa.
2. **Teste para `req_blocked_by_draft_adr` / detecção de `Open`**: montar uma REQ **no formato Python
   atual** (tabela `| Status | Open |`, sem a linha `> Date: … | Status: Open`). A regra que depende de
   status `Open` **deve** reconhecê-la.
3. **Os testes devem FALHAR** neste estado do código. Isso é o entregável: a prova de que as regras
   passavam por ausência de match. Rode-os, capture a saída da falha e registre no relatório.
4. **Marcá-los como esperando falha** de forma explícita e idiomática em cada runtime (`t.Skip` com
   motivo + referência, `test.skip`, `@pytest.mark.xfail(strict=True)`), com comentário apontando para
   esta REQ. A Wave 2 reativa. **Não deixe a suíte vermelha** — `make quality` precisa continuar verde.

**Acceptance criteria:**
- [ ] Teste de ADR-Draft-formato-Python existe nos 3 CLIs e falha contra o código atual
- [ ] Teste de REQ-Open-formato-Python existe nos 3 CLIs e falha contra o código atual
- [ ] A saída da falha está registrada no relatório do ML (é a evidência da cegueira)
- [ ] Marcados como skip/xfail com referência a esta REQ — `make quality` verde

---

## Wave 2 — Convergir os templates Python (agente único)

> Dependências: **barrier** — ML-1A concluído e auditado.
> Agente único: os 3 artefatos compartilham a decisão de formato. Distribuir produziria três
> interpretações do canônico — foi a lição do ciclo do `roadmap move`.

### ML-2A — `req new`, `adr new` e `roadmap new` do Python adotam o formato canônico

**Status:** done
**Files affected:** `pypi/trackfw/generators/req.py`, `adr.py`, `roadmap.py`,
`pypi/trackfw/commands/adr.py` (nomenclatura de arquivo), e os testes que travam o formato atual

**Actions:**
1. **`req new`**: frontmatter `status: Open` · `date` · `author: ""` · `adr: ""` · `roadmap: ""`,
   nesta ordem. Header `> Date: <data> | Status: Open`. Seções `## Motivation`,
   `## Acceptance Criteria`, `## Linked ADR`, `## Blocked by ADRs`, `## Linked Roadmap`.
   Remover `name`, `title`, `linked_adr`, `created`.
2. **`adr new`**: frontmatter `status: Proposed` (draft: `Draft`) · `date` · `author: ""`.
   Header `> Date: <data> | Status: <status>`. Seções `## Context`, `## Decision`, `## Consequences`,
   `## Alternatives Considered`. H1 `# ADR: <title>`.
3. **`adr new` — nome do arquivo**: `ADR-<YYYY-MM-DD>-<slug>.md`. Remover a numeração sequencial
   (`next_adr_number`) e seus testes.
4. **`roadmap new`**: frontmatter `status: backlog` · `date` · `req: ""` · `squad: ""`, minúsculo.
   Header `> Created: <data> | Status: backlog`. Seções e labels de ML em inglês, iguais a Go/Node.
5. **Referência exata**: use `internal/generators/{req,adr,roadmap}.go` como fonte de verdade do
   formato. Onde Go e Node divergirem entre si, **siga o Node** (`npm/src/generators/`) e registre a
   divergência no relatório — as duas conhecidas estão no escopo negativo da REQ, não corrija.
6. **Reativar os testes** marcados como skip/xfail no ML-1A. Devem passar agora.
   ⚠️ **Os 3 precisam ser reativados explicitamente — o Go não avisa.** Node (`testSkip`) e Python
   (`xfail(strict=True)`) **falham se o teste passar**, forçando a reativação. O Go usa `t.Skip`, que
   nem executa o corpo: se você corrigir os templates e esquecer de remover o `t.Skip` das linhas 1477
   e 1564 de `internal/validator/validator_test.go`, o teste fica pulado para sempre e ninguém sabe.
   É degradação silenciosa (P2) dentro do mecanismo criado para expor P2 — ver Log de execução.
7. **Corrigir as asserções que travam o formato antigo**:
   `pypi/tests/test_generators_roadmap.py:70`, `test_generators_req.py:46-47,67-70`,
   `test_generators_adr.py:88,94-100,135`, e a suíte de `next_adr_number` em `:19-41`.
   **Corrigir para o novo contrato, não reverter a decisão de formato.**

**Acceptance criteria:**
- [ ] `req new`, `adr new` e `roadmap new` do Python produzem frontmatter e header canônicos
- [ ] Nome de arquivo ADR no padrão `ADR-<YYYY-MM-DD>-<slug>.md`
- [ ] Testes do ML-1A reativados e **passando**
- [ ] Asserções antigas corrigidas para o novo contrato
- [ ] `make quality` verde

### ML-2B — Eliminar as divergências Go↔Node (promovido do escopo negativo)

**Status:** done
**Files affected:** `internal/generators/req.go`, `internal/generators/roadmap.go`,
`npm/src/commands/roadmap.js`, `npm/src/generators/req.js`, `pypi/trackfw/generators/req.py`,
mais os testes afetados

**Justificativa da promoção:** era o item 5 do escopo negativo, sob a premissa de "duas linhas
cosméticas". A medição empírica após o ML-2A mostrou **quatro** divergências, duas nunca catalogadas,
e uma delas é perda silenciosa de input do usuário. Sem corrigi-las, o gate do ML-3A nasceria com
lista de exceções — o "número mágico" que P1 condena.

**Medição (feita pelo orquestrador, com os 3 binários):**

| Artefato | Go × Node | Node × Python |
|---|---|---|
| ADR | ✅ idênticos | ✅ idênticos |
| REQ | ❌ Go emite `\| Linear Issue:` e `\| Jira Issue:` | ✅ idênticos |
| ROADMAP | ❌ 3 divergências | ✅ idênticos |

**Actions:**

1. **Node `roadmap new` aceita título posicional.** Hoje só tem `--title`
   (`npm/src/commands/roadmap.js:10`), e `roadmap new "auth strategy"` silenciosamente vira
   `# Roadmap: New Roadmap`. **O próprio Node é inconsistente**: `adr new <title>` e `req new <title>`
   usam posicional obrigatório. Adicionar `.argument('[title]')` mantendo `--title` como alias.
   Este é o item mais grave — é o único que **descarta dado do usuário**.
2. **`| Linear Issue:` / `| Jira Issue:` — adicionar a Node e Python.** Só o Go emite
   (`internal/generators/req.go:58,60`) e nenhum código lê. São placeholders de rastreabilidade
   externa para o humano preencher: **funcionalidade real, ausente em dois runtimes**. Convergir
   adicionando, não removendo — não se apaga recurso para satisfazer gate. Cobrir também a variante
   com `Blocked by ADRs` (`req.go:60`).
3. **Roadmap Go — `REQ: <título>` na linha de contexto** (`internal/generators/roadmap.go`): o Go
   grava o *título* onde deveria ir o caminho da REQ (o frontmatter `req:` fica vazio). Bug claro.
4. **Roadmap Go — linha literal `squad:` no corpo e `### ML-1A — <title>` placeholder**: Node e Python
   emitem o título real e não põem `squad:` no corpo. Go converge.

**Acceptance criteria:**
- [ ] `roadmap new "titulo"` funciona posicionalmente nos 3 CLIs, sem descartar o título
- [ ] REQ gerada pelos 3 CLIs é byte a byte idêntica, incluindo as linhas de issue
- [ ] ROADMAP gerado pelos 3 CLIs é byte a byte idêntico
- [ ] ADR permanece byte a byte idêntico (já está — não regredir)
- [ ] `make quality` verde

### ML-2C — Node usa UTC onde Go e Python usam hora local (P3)

**Status:** done
**Files affected:** `npm/src/generators/req.js:76`, `adr.js:33`, `note.js:24`, e o gerador de roadmap
do Node; mais os testes afetados

**Como apareceu:** levantado pelo agente do ML-2B como observação e **confirmado empiricamente pelo
orquestrador**. Não é teórico:

```
$ TZ=Pacific/Kiritimati   (UTC+14 — local 2026-07-28, UTC 2026-07-27)
GO:   REQ-2026-07-28-tz-test.md
NODE: REQ-2026-07-27-tz-test.md     ← dia diferente, arquivo diferente
```

O Node usa `new Date().toISOString().slice(0,10)` — **UTC**. Go usa `time.Now().Format(...)` e Python
usa `date.today()` — **hora local**. A divergência afeta o nome do arquivo, o `date:` do frontmatter
e a linha de header.

**Por que é bloqueante para o ML-3A:** o gate compararia os 3 CLIs byte a byte e passaria ou falharia
**conforme o horário e o fuso da máquina**. Em UTC-3, das 21h à meia-noite, quebraria; durante o dia,
passaria. Um gate intermitente é pior que nenhum — alguém acaba desabilitando.

**A paridade que auditei hoje passou por sorte:** rodei durante o dia, quando UTC e local coincidem.
É exatamente o "verde por coincidência" que o ADR de gates existe para eliminar (foi o defeito D2,
da ajuda colorida do argparse).

**Actions:**
1. **Node converge para hora local** nos geradores de artefato — `req.js`, `adr.js`, `note.js` e o de
   roadmap. Go e Python já usam local, e é a semântica correta: uma REQ criada às 22h do dia 27 no
   Brasil é do dia 27, não do 28.
2. Cobrir `note.js` mesmo sendo o artefato cuja paridade já estava documentada
   (`docs/cli-parity.md:38-40`) — o contrato está declarado, mas a data diverge do mesmo jeito.
3. **Teste que fixa o comportamento sob fuso**: rodar o gerador com `TZ` que force divergência
   UTC↔local e afirmar que os 3 produzem a mesma data. Sem isso, a regressão volta invisível.
4. **Report-only, não corrigir:** `npm/src/generators/init.js:73,93,497` e `npm/src/commands/*.js`
   também usam `toISOString`, mas são scaffold e exibição, não artefato governado. Registrar.

**Acceptance criteria:**
- [x] Os 3 CLIs geram a mesma data sob qualquer `TZ`, incluindo UTC+14 e UTC-11
- [ ] Nome de arquivo, `date:` e header idênticos nos 3, independentemente do fuso
- [ ] Teste que prova a paridade sob `TZ` divergente
- [ ] `make quality` verde

---

## Wave 3 — Gate de paridade de saída (agente único)

> Dependências: **barrier** — ML-2A concluído e auditado.

### ML-3A — Gate que executa os geradores e compara a saída

**Status:** done
**Files affected:** `scripts/` (gate novo ou cenário em `check-gates-falsify.sh`), `Makefile`,
`docs/cli-parity.md`

**Actions:**
1. **O gate que faltava.** Executar `req new`, `adr new` e `roadmap new` nos **3 CLIs** dentro de
   `mktemp -d`, com a mesma entrada, e comparar os arquivos gerados **byte a byte**. É a auditoria que
   o orquestrador fez à mão no ciclo do `roadmap move` e que provou valer — agora automatizada.
   Normalizar apenas a data, se ela entrar no conteúdo.
2. **Prova negativa (P4)**: divergir um template propositalmente e afirmar que o gate **reprova**, com
   diagnóstico legível. Sem isso o gate é não-verificado — regra do próprio
   `docs/gate-design-principles.md`.
3. **Integrar ao `make quality`**, sem variável de ambiente auxiliar, sem resíduo no working tree.
4. **Documentar** em `docs/cli-parity.md`: o frontmatter dos 3 artefatos passa a ser contrato
   explícito, como já é o da nota de vault (`cli-parity.md:38-40`). Referenciar o gate.

**Acceptance criteria:**
- [ ] Gate executa os 3 geradores nos 3 CLIs e compara saída byte a byte
- [ ] Prova negativa: template divergente faz o gate reprovar, com diagnóstico claro
- [ ] Roda em `make quality`, sem variável auxiliar e sem resíduo
- [ ] `docs/cli-parity.md` documenta o frontmatter dos 3 artefatos como contrato

## Log de execução

**2026-07-27 — ML-1A concluído e auditado. A cegueira está provada.**

6 testes, 2 por runtime. Os fixtures não foram inventados: saíram da execução real dos geradores
Python. Saída da falha contra o código atual:

```
--- FAIL: TestADRDraftFormatoPython_regra_cega
    DEFEITO P2 confirmado: blocked_by_draft_adr não detectou ADR Draft no formato Python.
    ADR existe mas adrIsDraft() retorna false. violations: []

--- FAIL: TestREQOpenFormatoPython_regra_cega
    DEFEITO P2 confirmado: REQ usa tabela '| Status | Open |' mas validator procura
    'Status: Open' (inline). A REQ é silenciosamente ignorada. violations: []
```

`violations: []` é a assinatura exata do P2 — a regra não errou, ela não viu.

**Isolamento correto:** cada teste cruza os formatos para provar **um** defeito por vez. O teste do
ADR usa REQ canônica (que passa no guard de `Open`), então a única explicação para zero violations é
o `adrIsDraft` cego. O teste da REQ usa ADR canônico (que passa em `adrIsDraft`), isolando o guard de
`Status: Open`. Sem esse cruzamento, um teste com os dois fixtures no formato Python provaria apenas
"algo está errado".

Escopo respeitado: `git diff` em `pypi/trackfw/generators/` vazio — os templates não foram tocados.
`make quality` verde: 597 passed, 2 xfailed.

### Achado do ML-1A → entra no ML-2A

O template de REQ do Python **não emite a seção `## Blocked by ADRs`**. É um terceiro defeito, adjacente
aos dois da REQ: mesmo que o formato do status fosse canônico, uma REQ gerada pelo Python nunca teria
ADRs para bloquear — a regra continuaria vacuamente verde por outro caminho. A convergência do ML-2A
resolve por tabela, já que o template canônico tem a seção, mas precisa ser verificado
explicitamente.

### Assimetria encontrada na auditoria → ação obrigatória no ML-2A

Os três marcadores de "teste esperando falha" **não têm a mesma força**:

| Runtime | Mecanismo | Se o defeito for corrigido |
|---|---|---|
| Node | `testSkip` (roda o corpo, `failed++` no XPASS) | **avisa** |
| Python | `@pytest.mark.xfail(strict=True)` | **avisa** |
| Go | `t.Skip` — não executa o corpo | **cala para sempre** |

Se o ML-2A converger os templates e esquecer de remover o `t.Skip` (linhas 1477 e 1564 de
`internal/validator/validator_test.go`), o teste Go fica pulado indefinidamente e nada acusa. É
degradação silenciosa **dentro do mecanismo criado para expor degradação silenciosa**. Registrado
como ação explícita no ML-2A.

---

### ML-3B — Slug de título acentuado diverge entre CLIs (P3)

**Status:** done
**Files affected:** função de slug em `internal/generators/` e `npm/src/generators/`, o gate
`scripts/check-artifact-parity.sh`, `scripts/check-gates-falsify.sh`, `docs/cli-parity.md`, testes

**Como apareceu:** o agente do ML-3A encontrou ao sondar o gate e contornou usando título ASCII puro.
O contorno é o problema — medição do orquestrador:

```
$ trackfw req new "Autenticação e Sessão"
go    REQ-2026-07-27-autenticação-e-sessão.md
node  REQ-2026-07-27-autenticação-e-sessão.md
py    REQ-2026-07-27-autenticacao-e-sessao.md     ← sem acentos
```

O **conteúdo** é idêntico nos três; o **nome do arquivo** diverge.

**Por que não é caso de borda:** este é um projeto em PT-BR — título acentuado é o caso comum, não a
exceção. Um gate que só testa `"parity gate test"` é vacuoso para o uso real, e vacuidade por escolha
de fixture é exatamente o defeito D2 desta linhagem (o argparse colorido, que "validava por
coincidência de texto").

**Dois defeitos reais além da paridade:**
1. **Portabilidade**: macOS normaliza nomes para NFD, Linux não. O mesmo nome lógico tem bytes
   diferentes entre plataformas — P3, e o repo tem job `windows-integrations-resolve` no CI.
2. **Quebra o `branch_has_wip_roadmap`**: `normalizeBranchSlug` descarta tudo fora de `[a-z0-9]`, então
   `...autenticação-e-sessão.md` normaliza para `autentica-o-e-sess-o`. Uma branch
   `fix/autenticacao-e-sessao` **nunca casaria** e a regra reprovaria trabalho legítimo. O gate que me
   pegou dois ciclos atrás quebra sozinho com título acentuado.

**Canônico: a normalização do Python** (NFKD + remoção de diacríticos). É a única das três que produz
nome de arquivo portável.

**Actions:**
1. Go e Node adotam NFKD + remoção de diacríticos antes do lowercase/hifenização já existente.
2. Aplicar em **todos** os geradores — `req`, `adr`, `roadmap`, `note` compartilham o caminho de slug.
3. **O gate passa a usar título acentuado** (ex.: `"Autenticação e Sessão"`). Remover o comentário de
   limitação do script e a seção correspondente de `docs/cli-parity.md` — vira contrato cumprido, não
   limitação documentada.
4. **Segundo cenário negativo**: o gate compara conteúdo **e** nome de arquivo, mas só o caminho de
   conteúdo tem prova (`sed` em `status: Open`). Adicionar cenário que diverge o **nome** num runtime e
   afirma reprovação com diagnóstico distinto. Dois caminhos de comparação exigem duas provas.
5. Teste dedicado nos 3 CLIs: título acentuado → mesmo slug nos três.

**Escopo negativo deste ML:** não renomear artefatos existentes em `docs/`. A mudança é para frente;
arquivos com acento continuam funcionando. `docs/requisições/` é diretório de config, não slug gerado.

**Acceptance criteria:**
- [ ] Título acentuado gera o mesmo nome de arquivo nos 3 CLIs
- [ ] Gate usa título acentuado — sem contorno ASCII
- [ ] Prova negativa para divergência de **nome de arquivo**, além da de conteúdo
- [ ] Teste dedicado de slug acentuado nos 3 CLIs
- [ ] `make quality` verde, `git status` limpo

---

## Log de execução — fechamento

**2026-07-27 — 6 MLs, 3 promovidos do escopo negativo por medição.**

O ciclo começou com 3 MLs planejados e terminou com 6. As três promoções (2B, 2C, 3B) não foram
escopo fugindo: **cada uma só ficou visível porque a wave anterior a tornou mensurável**, e todas
eram pré-requisito do mesmo gate.

| ML | Origem | O que se descobriu ao medir |
|---|---|---|
| 1A | planejado | as duas regras cegas, com `violations: []` como assinatura |
| 2A | planejado | Python convergiu para o Node |
| **2B** | promovido | não eram 2 divergências Go↔Node, eram **4** — uma delas o Node descartando o título do usuário |
| **2C** | promovido | Node em UTC, Go/Python em local — o gate seria intermitente por fuso |
| 3A | planejado | o gate; e ao sondá-lo, o slug acentuado |
| **3B** | promovido | acento no nome de arquivo: paridade + portabilidade NFD/NFC + quebra do `branch_has_wip_roadmap` |

### O padrão que se repetiu três vezes

Cada defeito promovido tinha a **mesma assinatura**: passava despercebido porque a verificação
existente não exercitava o caso real.

- O gate de paridade comparava nomes de comando, nunca a saída → 3 formatos de artefato divergiram.
- Minha auditoria de fuso passou **por sorte** — rodei de dia, quando UTC == local no Brasil.
- O gate do ML-3A contornou o slug acentuado usando `"parity gate test"` → num projeto PT-BR, o caso
  comum ficava fora da prova.

É o defeito D2 da linhagem (o `argparse` colorido que "validava por coincidência de texto") aparecendo
em três disfarces novos. **Verde por coincidência não é verde.**

### Prova final (orquestrador, com título acentuado)

`trackfw {req,adr,roadmap,note} new "Autenticação e Sessão"` nos 3 binários:

| Artefato | Nome gerado | Go×Node | Go×Python |
|---|---|:-:|:-:|
| REQ | `REQ-2026-07-27-autenticacao-e-sessao.md` | ✅ | ✅ |
| ADR | `ADR-2026-07-27-autenticacao-e-sessao.md` | ✅ | ✅ |
| ROADMAP | `ROADMAP-2026-07-27-autenticacao-e-sessao.md` | ✅ | ✅ |
| NOTE | `autenticacao-e-sessao-2026-07-27.md` | ✅ | ✅ |

Nome **e** conteúdo idênticos. `make quality`: 604 passed, **8 cenários de falsificação, 7 gates**
provados não-vacuosos — eram 6 no início do ciclo.

### O que muda daqui pra frente

`scripts/check-artifact-parity.sh` executa os geradores dos 3 runtimes e compara a saída real, com
título acentuado e prova negativa nos dois caminhos (conteúdo e nome de arquivo). O contrato de
frontmatter dos 4 artefatos está em `docs/cli-parity.md`. A próxima divergência de template quebra o
CI em vez de sobreviver anos.

---

## Acceptance Criteria

- [x] As 3 waves concluídas, na ordem
- [x] As duas regras cegas passam a detectar, com teste que provou a cegueira antes
- [x] Os 3 CLIs geram os 3 artefatos byte a byte idênticos
- [x] Gate impede regressão futura, com prova negativa
- [x] `make quality` verde, sem variável auxiliar
- [x] Escopo negativo da REQ respeitado — os 5 itens ficam registrados, não corrigidos
