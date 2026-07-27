---
status: wip
date: 2026-07-27
req: "docs/req/REQ-2026-07-26-robustez-dos-gates-de-governanca-e-paridade.md"
squad: ""
---

# Roadmap: robustez dos gates de governanca e paridade

> Created: 2026-07-27 | Status: wip

## Context

REQ: docs/req/REQ-2026-07-26-robustez-dos-gates-de-governanca-e-paridade.md
ADR: docs/adr/ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md

Quatro defeitos de gate apareceram em três REQs consecutivas e **nenhum foi pego pelo CI**. O trackfw
vende governança verificável: seus gates são o produto.

| # | Gate | Defeito | Situação |
|:-:|---|---|---|
| 1 | `check-integration-cli-parity.sh` | número mágico de itens do catálogo | corrigido |
| 2 | `check-cli-parity.sh` | ajuda colorida do `argparse` (Python 3.13+); validava por coincidência de texto | corrigido |
| 3 | `ship` npm/PyPI | `roadmap_dir` divergente; testes com injeção não exercitavam o caminho real | corrigido |
| 4 | `branch_has_wip_roadmap` | pune a Definition of Done que o produto prega | **aberto — ML-1A** |

### Princípios do ADR (aplicar em todo ML)

- **P1** Nenhum número mágico — derivar da fonte de verdade.
- **P2** Falha explícita, nunca degradação silenciosa.
- **P3** Independência de ambiente — runtime, cor, locale, `PATH`.
- **P4** Falsificabilidade obrigatória — provar que o gate **reprova**, não só que passa.

### Regra de paralelismo (calibrada nas REQs anteriores)

MLs só correm juntos se não compartilharem arquivo **nem saída de build**. Quem roda `make quality`
não corre em paralelo com ninguém — o gate escreve `bin/trackfw`. O orquestrador marca o status no
roadmap e **commita antes do spawn**.

### Mapa de dependências

```
Wave 1 (1A) ─ barrier ─> Wave 2 (2A ‖ 2B) ─ barrier ─> Wave 3 (3A)
```

---

## Wave 1 — Corrigir o defeito aberto (agente único)
> Dependências: nenhuma.

### ML-1A — `branch_has_wip_roadmap` aceita roadmap em `done/`
**Status:** done
**Files affected:** `internal/validator/validator.go` (~linha 1506), equivalentes em
`npm/src/validator/` e `pypi/trackfw/validator.py`, mais testes

**Actions:**
1. A regra hoje só percorre `wip/`. Passar a procurar o slug da branch também em `done/`,
   reaproveitando `normalizeBranchSlug` — **não** escrever outra normalização.
2. Reprovar apenas quando não houver roadmap correspondente em `wip/` **nem** em `done/`.
3. **Mitigação do risco de afrouxamento** (registrado no ADR): a aceitação em `done/` exige
   **casamento de slug**. Um roadmap qualquer em `done/` não serve — só o que corresponde à branch.
   Branch de feature sem nenhum roadmap correspondente continua reprovando.
4. Documentar o comportamento em `docs/cli-parity.md`.

**Acceptance criteria:**
- [ ] Roadmap em `done/` com slug da branch → **sem** violação, nos 3 CLIs
- [ ] Roadmap em `wip/` com slug da branch → sem violação (comportamento atual preservado)
- [ ] Branch de feature sem roadmap em lugar nenhum → **continua** reprovando
- [ ] Roadmap em `done/` com slug **diferente** da branch → **continua** reprovando
- [ ] Encerrar um roadmap na própria branch deixa `trackfw validate` verde — provado em repo temporário
- [ ] `make quality` verde

---

## Wave 2 — Auditoria P1–P3 (2 MLs em paralelo)
> Dependências: **barrier** — ML-1A concluído. Diretórios disjuntos: `internal/validator/` × `scripts/`.
> ⚠️ Nenhum dos dois roda `make quality`; o orquestrador roda na barrier.

### ML-2A — Auditoria das regras do validator
**Status:** pending
**Files affected:** `internal/validator/` e equivalentes; correções onde houver defeito

**Actions:**
Auditar as 17 regras (`adr_dir_exists`, `adr_orphan`, `blocked_by_draft_adr`, `blocked_has_req`,
`branch_has_wip_roadmap`, `filename_uniqueness`, `folder_status`, `note_orphan`, `ref_targets_exist`,
`req_has_adr`, `req_has_roadmap`, `stale_wip`, `wip_acceptance`, `wip_has_req`, `wip_limit`, e as de
`traceid_*`) contra P1–P3:
- **P1**: alguma regra hardcoda contagem, lista de estados ou caminho que deveria vir da config?
- **P2**: alguma regra **silencia** quando não consegue ler o que precisa (arquivo ilegível, frontmatter
  inválido, diretório ausente) em vez de reportar? Este é o padrão mais perigoso — foi o do
  `analyzing`, que era ponto cego.
- **P3**: alguma depende de locale, ordenação de sistema de arquivos, fim de linha ou fuso?

Corrigir o que encontrar. **Registrar no relatório a lista completa das 17 com o veredito de cada
uma** — inclusive as conformes. Auditoria sem inventário não é auditoria.

**Acceptance criteria:**
- [ ] As 17 regras auditadas, com veredito registrado individualmente
- [ ] Defeitos encontrados corrigidos, ou registrados com justificativa se fora de escopo
- [ ] Nenhuma regra degrada silenciosamente ao falhar em ler o que precisa
- [ ] `go build`, `go test` e `go vet` verdes; testes dos 3 CLIs verdes

### ML-2B — Auditoria dos scripts de gate
**Status:** pending
**Files affected:** `scripts/check-*.sh`, `scripts/smoke-integration-packages.sh`, `Makefile`

**Actions:**
Auditar contra P1–P3: `check-cli-parity.sh`, `check-identity-parity.sh`,
`check-integration-assets.sh`, `check-integration-cli-parity.sh`, `check-static-assets.sh`,
`check-validate-parity.sh`, `smoke-integration-packages.sh`.
- **P1**: números mágicos e listas hardcoded que deveriam derivar do catálogo/config.
- **P2**: `|| true`, `2>/dev/null` e `set +e` que engolem falha; comando ausente tratado como sucesso.
- **P3**: dependência de cor, locale, `PATH`, ordenação de `ls`/`find`, versão de runtime.

**Item extra herdado do ML-1A:** a mensagem de violação do `branch_has_wip_roadmap` lista **todos** os
roadmaps encontrados. Com `done/` agora incluído na busca, num projeto maduro isso vira uma parede de
texto — neste repositório já são 15 arquivos numa linha só. Truncar (ex.: 3 primeiros + contagem) ou
listar apenas os de `wip/`. Defeito de usabilidade, não de lógica, mas degrada uma mensagem que
existe para orientar.

⚠️ Atenção especial ao **P2 em shell**: `set -euo pipefail` no topo não protege comando dentro de
`$( )` nem o lado esquerdo de um pipe. Verificar caso a caso.

Registrar no relatório os 7 scripts com o veredito de cada um.

**Acceptance criteria:**
- [ ] Os 7 scripts auditados, com veredito individual registrado
- [ ] Nenhum engole falha silenciosamente
- [ ] Nenhum depende de ambiente para dar o mesmo resultado
- [ ] Cada script corrigido continua reprovando o que deveria

---

## Wave 3 — Falsificabilidade e documentação (agente único)
> Dependências: **barrier** — Wave 2 concluída.

### ML-3A — Testes de falsificação (P4) e documentação dos princípios
**Status:** pending
**Files affected:** testes nos 3 CLIs, `scripts/`, `docs/`

**Actions:**
1. **Teste de falsificação por gate.** Cada script de paridade e cada regra corrigida ganha um teste
   que **monta o cenário negativo e prova que o gate reprova**. Hoje nenhum tem — os quatro defeitos
   existiam com o CI verde.
   Usar o mecanismo de teste já existente em cada CLI. **Não criar framework.**
2. **Sem resíduo:** o cenário negativo é montado e desmontado; nada fica no repositório.
3. **Documentar os princípios P1–P4** em `docs/` (seção no `cli-parity.md` ou documento próprio), com
   os quatro defeitos reais como exemplo. Quem escrever o próximo gate precisa encontrar isso.
4. Referenciar as notas de vault existentes.

**Acceptance criteria:**
- [ ] Cada script de paridade tem teste que prova reprovação do cenário negativo
- [ ] Cada regra corrigida na Wave 2 tem teste equivalente
- [ ] Nenhum resíduo após os testes
- [ ] Princípios documentados com os casos reais
- [ ] `make quality` verde **sem** variável de ambiente auxiliar

---

## Log de execução

**2026-07-27 — ML-1A concluído e auditado.**

`make quality` verde. Reúso confirmado: o agente extraiu `resolveStateDirs(cfg, state)` e derivou
`wip` e `done` dele — uma única resolução de caminho, uma única `normalizeBranchSlug`. Era a
exigência explícita, porque duplicar resolução foi a causa raiz do `roadmap_dir` divergente na REQ
anterior.

**Verificação empírica feita no próprio repositório, nesta branch** — o cenário que falhou nas duas
REQs anteriores:

| Estado do roadmap | `trackfw validate` |
|---|---|
| em `wip/` | ✓ sem violações |
| **movido para `done/`** (DoD cumprida) | **✓ sem violações** ← antes reprovava |
| em `done/` com slug **diferente** da branch | ✗ reprova, como deve |

A terceira linha é o que prova que a regra não afrouxou: aceitar `done/` sem exigir casamento de slug
teria feito o gate nunca mais reprovar.

**Efeito colateral encontrado na prova negativa** → movido para o ML-2B: com `done/` na busca, a
mensagem passa a listar todos os roadmaps encontrados — 15 numa linha só neste repositório. Orienta
menos do que antes.

## Acceptance Criteria

- [ ] Todas as waves concluídas
- [ ] Encerrar roadmap na própria branch deixa `trackfw validate` verde
- [ ] Inventário completo das 17 regras e dos 7 scripts, com veredito individual
- [ ] Todo gate corrigido tem prova de que ainda reprova
- [ ] `make quality` verde nos 3 CLIs, sem variável auxiliar
- [ ] Escopo negativo respeitado (sem framework novo, sem dependência nova, sem rebaixar severidade)
