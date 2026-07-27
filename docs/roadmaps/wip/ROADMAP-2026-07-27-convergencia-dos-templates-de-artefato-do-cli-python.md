---
status: wip
date: 2026-07-27
req: "docs/req/REQ-2026-07-27-convergencia-dos-templates-de-artefato-do-cli-python.md"
squad: ""
---

# Roadmap: convergencia dos templates de artefato do CLI Python

> Created: 2026-07-27 | Status: wip

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

**Status:** in progress
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

---

## Wave 3 — Gate de paridade de saída (agente único)

> Dependências: **barrier** — ML-2A concluído e auditado.

### ML-3A — Gate que executa os geradores e compara a saída

**Status:** pending
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

## Acceptance Criteria

- [ ] As 3 waves concluídas, na ordem
- [ ] As duas regras cegas passam a detectar, com teste que provou a cegueira antes
- [ ] Os 3 CLIs geram os 3 artefatos byte a byte idênticos
- [ ] Gate impede regressão futura, com prova negativa
- [ ] `make quality` verde, sem variável auxiliar
- [ ] Escopo negativo da REQ respeitado — os 5 itens ficam registrados, não corrigidos
