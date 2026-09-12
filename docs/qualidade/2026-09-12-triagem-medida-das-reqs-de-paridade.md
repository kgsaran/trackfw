# Triagem medida das REQs de paridade — ML-1A

> Gerado por: Hefesto (Code Quality) | Data: 2026-09-12
> Roadmap: `ROADMAP-2026-09-12-triagem-medida-das-reqs-de-paridade-e-gate-de-conjunto-de-regras.md`
> Binários usados: `./bin/trackfw 7.6.0` (Go) · `node npm/bin/trackfw 7.6.0` (Node) · `PYTHONPATH=pypi python3 -m trackfw 7.6.0` (Python)
> Ferramenta de busca: `/usr/bin/grep` (nunca o `grep` do shell, que é `ugrep -I` e omite arquivos com NUL byte)

---

## Tabela de Vereditos

| REQ | Veredito | ACs pendentes |
|---|---|---|
| `note_orphan` (REQ-2026-08-20) | **PARCIAL** | AC3 |
| `thirdparty_artifact_has_provenance` (REQ-2026-09-01) | **PARCIAL** | AC4 |
| `validate --json` Python `rule: None` (REQ-2026-08-20) | **PENDENTE** | AC1–AC5 |
| CLI Python `init` sem `--ci`/`--hooks` (REQ-2026-08-28) | **PENDENTE** | AC1–AC9 |
| `status` Python conta REQs incorretamente (REQ-2026-08-30) | **PENDENTE** | AC1–AC6 |
| `check-referential-integrity` vácuo (REQ-2026-09-03) | **PENDENTE** | AC1–AC6 |

**REQs entregues: 0. Parciais: 2. Pendentes: 4.**

Achados adicionais da varredura (sem REQ associada): **2** (issues #310 e #298).

---

## REQ 1 — `note_orphan` existe em Go e Python e está ausente do CLI Node

**REQ:** `REQ-2026-08-20-note-orphan-existe-em-go-e-python-e-esta-ausente-do-cli-node.md`
**Veredito: PARCIAL**

### Medição de presença — `/usr/bin/grep`

A evidência original da REQ usou o `grep` do ambiente (`ugrep -I`) que omite silenciosamente
`npm/src/validator/index.js` (190 KB, NUL bytes como separador de chave). O resultado foi "0
ocorrências no Node", que era falso.

```
/usr/bin/grep -rn "note_orphan" internal/validator/validator.go
  → 4 ocorrências (linhas 170, 495, 795, 2995)

/usr/bin/grep -rn "note_orphan" npm/src/validator/index.js
  → 3 ocorrências (linhas 1621, 3469, 3758)

/usr/bin/grep -rn "note_orphan" pypi/trackfw/validator.py
  → 5 ocorrências (linhas 309, 2100, 2123, 2144, 4272)
```

**Divergência fonte vs evidência original:** a evidência original dizia "ausente no Node"; a medição
com `/usr/bin/grep` confirma **presente nos 3 runtimes**. A REQ foi aberta com base em falso negativo
do ugrep-I.

### Medição de comportamento — 3 binários

Fixture: diretório com `vault/notes/index.md` (vazio) e `vault/notes/nota-orfa-2026-09-12.md` (não
linkada no index).

```bash
# Go
./bin/trackfw validate --json
  → rule: "note_orphan" - note "nota-orfa-2026-09-12.md" is not referenced in vault/notes/index.md

# Node
node npm/bin/trackfw validate --json
  → rule: "note_orphan" - note "nota-orfa-2026-09-12.md" is not referenced in vault/notes/index.md

# Python (PYTHONPATH=pypi, cwd=projeto)
python3 -m trackfw validate --json
  → rule: "note_orphan" - note "nota-orfa-2026-09-12.md" is not referenced in vault/notes/index.md
```

Todos os 3 runtimes detectam e nomeam corretamente.

### Por AC

| AC | Status | Evidência |
|---|---|---|
| AC1 — note_orphan no Node | **Entregue** | `npm/src/validator/index.js:1621,3469,3758`; binário detecta a nota órfã |
| AC2 — Go e Python concordam antes de codificar | N/A post-facto | os 3 concordam; processo não é mais verificável |
| AC3 — Gate comparando **3 saídas reais** | **Pendente** | `check-artifact-closed-cycle.sh` testa por runtime independentemente; `cli-parity.md:148` documenta explicitamente `partial=check-artifact-closed-cycle.sh`, com exclusão de "severidade / `rules: note_orphan: error` / projeto sem vault continuam sem comparação cross-CLI" |
| AC4 — Cenário P4, baseline + detecção | **Entregue** | `check-artifact-closed-cycle.sh`: `note_orphan-silent-for-indexed` (baseline) + `note_orphan-fires-for-unindexed` (detecção) para os 3 runtimes; falsificável via `check-gates-falsify.sh` |
| AC5 — `cli-parity.md` atualizado, gate nomeado | **Parcial** | gate nomeado (`check-artifact-closed-cycle.sh`) mas documentado como `partial=` |
| AC6 — `make quality` verde | Presumido entregue | gate integrado em `parity-rest` (Makefile:48) |

**AC pendente: AC3.** A REQ pediu "gate comparando as três saídas reais — teste por stack não fecha".
O que existe testa cada runtime independentemente; não há comparação byte a byte entre runtimes para
esta regra.

---

## REQ 2 — `thirdparty_artifact_has_provenance` existe em Go e Python, mas não no validator do Node

**REQ:** `REQ-2026-09-01-regra-thirdparty-artifact-has-provenance-existe-em-go-e-python-mas-nao-no-validator-do-node.md`
**Veredito: PARCIAL**

### Medição de presença — `/usr/bin/grep`

A evidência original (Zeus, 2026-09-01) também usou o grep do ambiente e obteve "0 ocorrências no
Node" para a mesma causa (ugrep-I + NUL bytes).

```
/usr/bin/grep -rn "thirdparty_artifact_has_provenance" internal/validator/validator_thirdparty_provenance.go
  → 9 ocorrências (incluindo linha 24, 113, 135, 166, 176, 187, 196+)

/usr/bin/grep -rn "thirdparty_artifact_has_provenance" npm/src/validator/index.js
  → 8 ocorrências (linhas 3614, 3654, 3669, 3700, 3712, 3728, 3770)
  npm/src/commands/thirdparty.js
  → 1 ocorrência (linha 39: THIRD_PARTY_PROVENANCE_RULE)

/usr/bin/grep -rn "thirdparty_artifact_has_provenance" pypi/trackfw/validator.py
  → 12+ ocorrências (linhas 127, 4112, 4118, 4119, 4160, 4174, 4204, 4219, 4229, 4310, 4311)
```

**Divergência fonte vs evidência original:** a evidência original dizia "ausente no Node"; a medição
com `/usr/bin/grep` confirma **presente nos 3 runtimes**. Mesma causa que note_orphan.

### Por AC

| AC | Status | Evidência |
|---|---|---|
| AC1 — regra no Node com mesma mensagem e severidade | **Entregue** | `npm/src/validator/index.js:3614`; `check-thirdparty-parity.sh` parte C (`Makefile:86`) verifica byte-parity da mensagem de violação nos 3 CLIs |
| AC2 — chave montada com `/`, não separador nativo | **Entregue** | `npm/src/lib/pathfmt.js:39-43`: `normalizeRefSeparator` substitui `\` por `/` incondicionalmente; usada na linha `3696` para `provenanceKey` |
| AC3 — Falsificação nos 3 runtimes | **Entregue** | `check-thirdparty-parity.sh` parte C (linha 421-422): `diff -u "$WORK/go-branch-i.msg" "$WORK/node-branch-i.msg"` e `diff -u "$WORK/go-branch-i.msg" "$WORK/python-branch-i.msg"`; D2-bis (linha 384): positivo — install legítimo produz 0 violações em 3 CLIs |
| AC4 — Gate reprova se regra existe em N e falta em outro | **Pendente** | Gate de conjunto de regras não existe; é o que ML-2A deste roadmap deve criar |

**AC pendente: AC4.** "Esta lacuna sobreviveu porque nenhum gate compara o conjunto de regras entre
os 3 validators" (texto da própria REQ). O gate `check-thirdparty-parity.sh` é específico para esta
regra; a AC4 pede o gate geral que detectaria automaticamente a próxima regra que nascer só em 2
runtimes.

---

## REQ 3 — `validate --json` do Python não rotula a regra `branch_has_wip_roadmap`

**REQ:** `REQ-2026-08-20-validate-json-do-python-nao-rotula-a-regra-branch-has-wip-roadmap.md`
**Veredito: PENDENTE**

### Medição de comportamento — 3 binários

```bash
# Fixture: projeto com TRACKFW_BRANCH=feat/testnotexist (sem roadmap em wip)
TRACKFW_BRANCH=feat/testnotexist ./bin/trackfw validate --json
  → rule: "branch_has_wip_roadmap"    (Go — correto)

TRACKFW_BRANCH=feat/testnotexist node npm/bin/trackfw validate --json
  → rule: "branch_has_wip_roadmap"    (Node — correto)

TRACKFW_BRANCH=feat/testnotexist python3 -m trackfw validate --json
  → rule: None                        (Python — BUG CONFIRMADO)
```

O gate `check-validate-parity.sh` linha 807 **pina a divergência como comportamento esperado**:
```python
expected_rules = ([None] if rt == "py" else ["branch_has_wip_roadmap"])
```
Comentário do gate: "Go and Node.js both tag `rule`: `branch_has_wip_roadmap`; Python tagueia
`rule`: null para esta regra especificamente. Qualquer mudança nesse conjunto é uma regressão real."

A REQ quer que o gate passe a fixar a **convergência** (rule não-None no Python), não a divergência.

### Por AC

| AC | Status |
|---|---|
| AC1 — Python rotula `branch_has_wip_roadmap` | **Pendente** — binário produz `rule: None` |
| AC2 — varredura das demais regras do Python | Pendente |
| AC3 — gate deixa de fixar divergência | Pendente — hoje o gate fixa `rule: None` como esperado |
| AC4 — Cenário P4 | Pendente |
| AC5 — `make quality` verde | Pendente |

---

## REQ 4 — CLI Python não oferece `--ci` e `--hooks` no `init`

**REQ:** `REQ-2026-08-28-cli-python-nao-oferece-superficie-de-ci-e-git-hooks-no-init-e-nao-declara-git-hooks-como-alvo-do-update.md`
**Veredito: PENDENTE**

### Medição de comportamento — binário Python

```bash
python3 -m trackfw init --help
  usage: trackfw init [-h] [--project-name PROJECT_NAME]
                      [--namespacing {flat,by_agent}] [--agents AGENTS]
                      [--wip-limit WIP_LIMIT] [--ai-tools AI_TOOLS]
                      [--identity-preset IDENTITY_PRESET] [--forge FORGE]
```

**Sem `--ci` e sem `--hooks`** — confirma o defeito reportado. Go e Node aceitam `--forge` e produzem
scripts de git hooks e CI; Python não aceita as flags correspondentes.

Todos os ACs (AC1–AC9) pendentes.

---

## REQ 5 — `status` do Python conta REQs incorretamente (`consumidores-by-agent`)

**REQ:** `REQ-2026-08-30-consumidores-que-nao-conhecem-by-agent-status-do-python-conta-reqs-flat-e-walkmd-do-node-indexa-sem-filtro.md`
**Veredito: PENDENTE**

### Medição de comportamento — 3 binários

Fixture: projeto `by_agent` com `roadmap_namespacing: by_agent`, `agents: [hefesto, apolo]` e uma
REQ em `docs/req/backlog/REQ-2026-09-12-test.md`.

```bash
./bin/trackfw status
  → REQs  1  (1 Open · 0 Done · 0 Closed)    (Go — correto)

node npm/bin/trackfw status
  → REQs  1  (1 Open · 0 Done · 0 Closed)    (Node — correto)

python3 -m trackfw status
  → REQs  0  (0 Open · 0 Done · 0 Closed)    (Python — BUG CONFIRMADO)
```

### Nota de escopo (issue #268)

O issue #268 (lourivalgarciajunior, medição em `4f0ad33`) refina o AC1:

> "O defeito não é de `by_agent`. O layout `req_dir/<estado>/*.md` é válido em **todos** os modos.
> Reproduzido num projeto **flat**, com `roadmap_namespacing` ausente do `trackfw.yaml`:
> python status → REQs 0, enquanto req list e context → 4."

O discriminante é o arquivo: `pypi/trackfw/commands/status.py:57` usa `_list_files(req_dir)` (flat)
enquanto `status.py:133` (`_blocked_reqs`) já usa `resolve_req_files`. O contador é a única
enumeração que ignora o layout.

AC1-bis (saída se contradiz): o mesmo comando Python pode mostrar "REQs 0" e abaixo listar a REQ
como "blocked by not-accepted ADRs" — confirmado por issue #268 com fixture adequada.

Todos os ACs (AC1–AC6) pendentes.

---

## REQ 6 — `check-referential-integrity.sh` diz OK sobre árvore vazia

**REQ:** `REQ-2026-09-03-check-referential-integrity-diz-ok-e-sai-zero-sobre-arvore-vazia-sem-guarda-de-vacuidade.md`
**Veredito: PENDENTE**

### Medição de comportamento — execução do script

```bash
# Árvore vazia (docs/req/ e docs/adr/ criados mas sem arquivos)
bash scripts/check-referential-integrity.sh
  → Referential integrity OK
  → RC=0
```

O script tem o corpo:
```bash
for req in docs/req/*.md; do [[ -f "$req" ]] || continue; ...
```

Quando o glob `docs/req/*.md` não casa (pasta vazia), o `for` não itera, o script imprime `OK` e sai
0 sem ter verificado nada. Confirma o defeito descrito na REQ.

Todos os ACs (AC1–AC6) pendentes.

---

## Achados adicionais da varredura — issues sem REQ associada

### Achado A — Issue #310: 2 divergências de `init` confirmadas em 7.6.0

**Critério de inclusão:** evidência de ausência/divergência num runtime confirmada por execução dos 3
binários.

Medição executada neste ML com os 3 binários 7.6.0:

```bash
# init com flags comuns --ai-tools claude --forge github
Go:     19 arquivos criados
Node:   19 arquivos criados
Python: 20 arquivos criados

# Arquivo extra no Python:
docs/adr/ADR-001-inicio-do-projeto.md

# Shebang de scripts/trackfw-validate.sh:
Go:     #!/usr/bin/env sh
Node:   #!/usr/bin/env sh
Python: #!/usr/bin/env bash   ← DIVERGE
```

**Achado 1** confirmado (Python cria ADR-001, Go/Node não criam).
**Achado 2** confirmado (shebang diverge: sh vs bash).

Não sei qual lado é o certo para o shebang — se `trackfw-validate.sh` usa construção `bash`-only, o
sh do Go e do Node é que está errado, e a correção vai na direção oposta à óbvia. A medição aponta a
divergência; a decisão de qual lado convergir é do arquiteto.

**Sem REQ associada.** Reportando ao arquiteto para decisão.

---

### Achado B — Issue #298: `check-cli-parity.sh` só compara primeiro nível

**Critério de inclusão:** mesma família do AC4 (gate de conjunto ausente), mas na superfície de
comandos em vez de regras. Confirmado por leitura do script.

```bash
# check-cli-parity.sh (Makefile:37) executa:
trackfw --help   → compara o bloco de "Available Commands:" (primeiro nível)
roadmap new --help   → compara flags específicas do roadmap new

# Não executa:
trackfw adr --help   (subcomandos: list, new)
trackfw req --help   (subcomandos: list, move, new)
trackfw roadmap --help   (subcomandos: list, move, new, show)
```

Um subcomando que desaparecer de um runtime não é detectado por nenhum gate de CI.

O issue #298 traz um script `check-subcommand-parity.sh` (fork do consumidor externo) que desceu um
nível a partir do `--help` de cada comando com subcomando e compara conjuntos nos dois sentidos
(faltando e sobrando). O PR com o script não foi aberto ainda.

Zeus instrui: "Não expanda o ML-2A para cobri-los sem me falar. Se a medição mostrar que é o mesmo
mecanismo do gate de conjunto de regras, a Regra Dura de Causa Raiz manda tratar na mesma REQ — mas
quem decide isso sou eu, com a sua medição na mão."

**Medição:** confirmado por leitura do script. Gate de subcomandos ausente.
**Sem REQ associada.** Reportando ao arquiteto para decisão.

---

## Fonte vs Binário — divergências detectadas

| REQ | Fonte (ugrep-I original) | Fonte (/usr/bin/grep) | Binário | Divergência |
|---|---|---|---|---|
| note_orphan | Node: 0 ocorrências | Node: 3 (index.js:1621,3469,3758) | Node detecta órfã | SIM — evidência original era falso negativo |
| thirdparty_artifact | Node: 0 ocorrências | Node: 8 (index.js:3614+) + 1 (thirdparty.js) | check-thirdparty-parity.sh verde | SIM — evidência original era falso negativo |
| validate --json Python | N/A (fonte correto, bug é de retorno) | Python tem o código da regra | Python emite `rule: None` | NÃO (fonte e binário concordam: bug é no retorno, não na ausência de código) |
| init Python | N/A | Python não tem --ci/--hooks no argparse | help confirma ausência | NÃO (concordam) |
| status Python | N/A | Python usa _list_files, não resolve_req_files | Python conta 0 REQs | NÃO (concordam) |
| referential-integrity | N/A (é um script shell) | glob `docs/req/*.md` sem guarda | RC=0 com tree vazia | NÃO (concordam) |

---

## Resposta à pergunta desta REQ

**Quantas REQs estavam entregues: 0.**

As duas que Zeus havia medido como "provavelmente parciais" (note_orphan e thirdparty_artifact)
confirmaram-se parciais — não entregues. A evidência de ausência no Node era falsa (ugrep-I), mas
o requisito de gate cross-CLI (AC3 de note_orphan; AC4 de thirdparty) ainda não foi satisfeito.

As 4 restantes são pendentes com comportamento defeituoso confirmado por execução dos 3 binários.

---

## Instrução para ML-1B

| REQ | Ação |
|---|---|
| note_orphan | Não fechar. Marcar no artefato: AC1 entregue; AC3 pendente (gate cross-CLI ausente) |
| thirdparty_artifact | Não fechar. Marcar: AC1-AC3 entregues; AC4 pendente (gate de conjunto) |
| validate --json Python | Não tocar (pendente) |
| cli-python-init | Não tocar (pendente) |
| consumidores-by-agent | Não tocar. Anotar: escopo do AC1 corrigido pelo issue #268 |
| check-referential-integrity | Não tocar (pendente) |

---

*Hefesto — Code Quality Specialist — 2026-09-12*
