---
status: wip
date: 2026-09-22
req: "docs/req/REQ-2026-09-03-check-referential-integrity-diz-ok-e-sai-zero-sobre-arvore-vazia-sem-guarda-de-vacuidade.md"
squad: ""
---

# Roadmap: `check-referential-integrity.sh` diz `OK` e sai 0 sobre árvore vazia — quinto gate vácuo, e está dentro do `parity`

> Created: 2026-09-22 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-09-03-check-referential-integrity-diz-ok-e-sai-zero-sobre-arvore-vazia-sem-guarda-de-vacuidade.md -->
REQ: docs/req/REQ-2026-09-03-check-referential-integrity-diz-ok-e-sai-zero-sobre-arvore-vazia-sem-guarda-de-vacuidade.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ]
- [ ]

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** ✅ Concluído
**Files affected:** `scripts/check-referential-integrity.sh` (único alvo — ver seção 1)
**Analyst:** hefesto-tf — 2026-10-17

<!-- ML-0A-DONE -->

---

#### Seção 1 — Completude da enumeração

**Mecanismo alvo:** laço sobre `find … 2>/dev/null | sort` (ou equivalente glob/`git ls-files`) que,
com zero itens, termina em `OK` / `exit 0` sem contar nada.

**Varredura realizada:** todos os `scripts/check-*.sh` com `find` ou `git ls-files`. 12 candidatos
identificados por `grep -c "find\|git ls-files"`. Cada um verificado quanto à presença de guarda de
vacuidade (contador, floor-guard, `SCENARIOS_RUN`, `examined`, etc.).

| Arquivo | Linha crítica | Vácuo? | Evidência |
|---|---|---|---|
| `scripts/check-referential-integrity.sh` | 65: `find "$ROOT_DIR/$REQ_DIR" … 2>/dev/null \| sort` | **SIM** | Reproduzido: `bash check-referential-integrity.sh` com `trackfw.yaml` presente e `docs/req/` ausente → `"Referential integrity OK"`, exit 0 |
| `scripts/check-no-literal-nul-in-source.sh` | `git ls-files --eol` em `_text_files_from_repo` | não | `if [[ $examined -eq 0 ]]; then exit 1` (explicit guard) |
| `scripts/check-interpolated-path-in-python.sh` | `git ls-files … 2>/dev/null` (linha ~403) | não | `TOTAL_BODIES < MIN_BODIES` e `EXPANDING < MIN_EXPANDING` floor-guards |
| `scripts/check-parity-call-site-pins.sh` | `grep -lE …` em `find_consuming_scripts` | não | `if [[ "$CHECKED" -eq 0 ]]; then exit 1` |
| `scripts/check-ci-workflow-pin-parity.sh` | find interno | não | `if [ "$SCENARIOS_RUN" -eq 0 ]; then exit 1` |
| `scripts/check-symlink-privilege-guard.sh` | `find` interno | não | `if [[ $FILE_COUNT -eq 0 ]]; then exit 1` (linha 134) |
| `scripts/check-agent-namespace-union.sh` | `find` / fixtures internas | não | emite `$SCENARIOS scenarios passed` — count visível |
| `scripts/check-pr-closing-keyword.sh` | sem loop sobre corpus | não | classe diferente: sai `not_evaluated` se sem fonte de PR |
| `scripts/check-ref-separator-portability.sh` | `assert_count` com file checks | não | não enumera corpus por find; verifica arquivos nomeados |
| `scripts/check-req-path-literals.sh` | `find "$REPO_ROOT/scripts" … 2>/dev/null` (linha 89) | não | gate de _ausência_ de padrão: zero violações = correto por construção |
| `scripts/check-parity-contract-coverage.sh` | Python interno | não | floor guards de cobertura de seções |
| `scripts/check-agent-models-parity.sh` | fixtures internas | não | vacuity guards explícitos por cenário (`"composition/vacuity-guard"`) |

**Conclusão:** superfície fechada a **um único arquivo**:
`scripts/check-referential-integrity.sh`, linha 65.

Nenhum outro script do `make quality` exibe o mesmo mecanismo sem guarda.

**Reprodução do bug (idêntica ao cenário 12 de `check-gates-falsify.sh`):**

```bash
T=$(mktemp -d)
mkdir -p "$T/scripts"
cp scripts/check-referential-integrity.sh "$T/scripts/"
cp trackfw.yaml "$T/"               # req_dir: docs/req  — diretório NÃO criado
bash "$T/scripts/check-referential-integrity.sh"
# Saída: "Referential integrity OK"
# Exit:  0   ← deveria ser 1
```

---

#### Seção 2 — Ameaça

**Quem esvazia o gate sem quebrar regra escrita, e como:**

1. **`req_dir` errado no `trackfw.yaml`** — typo (`docs/reqs` em vez de `docs/req`), valor
   desatualizado pós-renomeação, ou projeto com `req_dir: docs/requisições`. O script lê o yaml
   corretamente (AC5 já implementado), mas se o diretório resultante não existir, `find` retorna
   zero linhas em silêncio via `2>/dev/null`. O gate passa como se não houvesse REQs.

2. **Diretório renomeado sem atualizar o yaml** — renomear `docs/req/` para qualquer outro nome
   sem tocar no `trackfw.yaml` produz o mesmo efeito: `find` silencioso, exit 0.

3. **`2>/dev/null` engolindo erro de permissão** — se `docs/req/` existir mas for inacessível
   (`chmod 000`), `find` falha com `Permission denied`. O `2>/dev/null` engole o erro, `sort`
   retorna 0 sobre pipe vazio, o laço processa zero linhas. Gate passa.

4. **`-maxdepth 2` não alcança layouts mais profundos** — o comentário na linha 25 declara que
   depth ≤ 2 cobre `flat` e `by_agent`. Verificado: `docs/req/<agent>/REQ-*.md` = depth 2 da
   raiz de `REQ_DIR` → coberto. **Não coberto:** layout hipotético `by_agent + state subdirs`
   (`docs/req/<agent>/<state>/REQ-*.md` = depth 3). O `trackfw.yaml` deste repositório usa
   `roadmap_namespacing: flat` e nenhum layout de profundidade 3 existe hoje. Resíduo declarado
   na Seção 4.

5. **Execução de `ci`/`pre-push` em repositório recém-clonado sem `make build`** — `ROOT_DIR` é
   calculado corretamente pelo `${BASH_SOURCE[0]}`, mas se `docs/req/` não foi inicializado (ex.:
   primeiro setup), o gate passa sem verificar nada — sem aviso.

6. **REQs com `adr:` e `roadmap:` todos vazios ou `"-"`** — o gate enumera arquivos (vacuidade de
   count coberta pela guarda futura), mas o laço interno não verifica referência alguma. Exit 0
   com "OK" mesmo que zero referências tenham sido conferidas. Esta é a **segunda direção de
   vacuidade** (vacuidade de _referências verificadas_, não de _arquivos enumerados_). Coberta
   parcialmente por AC3 (contagem emitida).

---

#### Seção 3 — Alvos de falsificação nas duas direções

**Eixo 1 — arquivos enumerados:**

| Cenário | Comportamento atual | Comportamento esperado pós-fix |
|---|---|---|
| `docs/req/` ausente | `OK`, exit 0 | `FAIL: zero REQs enumeradas`, exit 1 |
| `docs/req/` com REQs íntegras | `OK`, exit 0 | `OK (N REQs verificadas)`, exit 0 |
| `docs/req/` com referência quebrada | `FAIL`, exit 1 | `FAIL: …`, exit 1 (sem regressão) |

**Eixo 2 — referências verificadas (segunda vacuidade):**

Cenário: `docs/req/` existe e contém REQs, mas todas têm `adr: ""` e `roadmap: "-"`.

- Comportamento atual: `OK`, exit 0 — nenhuma referência verificada, nenhum aviso.
- **Recomendação:** deve **passar** com exit 0, mas emitir contagem explícita de referências
  verificadas (via AC3: `"N REQs, M refs verificadas"`). Se `M = 0` e `N > 0`, a mensagem é
  informativa e rastreável, sem falso alarme — a constraint "sem refs quebradas" é genuinamente
  satisfeita. Exit 1 neste caso seria falso positivo: um projeto legítimo pode ter todas as REQs
  sem ADR/roadmap vinculados.
- **O que NÃO deve ocorrer:** `OK` sem contagem. Isso é indistinguível do bug original para
  quem lê o log.

**Sobre o cenário 12 existente em `check-gates-falsify.sh`:**

O cenário 12 (linha 1803) cobre apenas a direção "referência quebrada → reprova". Ele **não**
cobre:
- árvore sem `docs/req/` → deve reprovar (o bug desta REQ)
- árvore íntegra → deve passar (não-regressão do fix)

Ambas as direções devem ser adicionadas como ML na Wave 1 (AC2/AC4).

---

#### Seção 4 — Resíduo declarado

1. **Layout `by_agent` + state subdirs (depth 3+):** `find -maxdepth 2` não alcança
   `docs/req/<agent>/<state>/REQ-*.md`. O `trackfw.yaml` deste repositório usa `flat`; sem
   instância real no projeto, o fix não precisa cobrir depth 3. Resíduo aceito.

2. **Gates fora do `make quality`:** outros scripts do repositório não auditados aqui. A REQ
   delimita o escopo ao pipeline de paridade. O `check-orphan-gates.sh` garante que gates
   existentes sejam consumidos; o que não está no `make quality` não é responsabilidade desta REQ.

3. **Vacuidade de referências verificadas (M = 0, N > 0):** AC3 (contagem emitida) torna o caso
   visível no log, mas o gate ainda sai com exit 0. Aceito pelo AC da REQ: a constraint é "sem
   quebras", não "ao menos uma ref verificada".

4. **`2>/dev/null` em `find`:** a supressão de erro de permissão permanece após o fix da guarda
   de vacuidade (o fix exige `find` retornando zero, não `find` falhando). O risco de silenciar
   erros de permissão real é resíduo aceito; mitigação seria separar `find` de `sort` e verificar
   o exit code, mas isso está fora do escopo desta REQ.

---

**Acceptance criteria:**
- [x] The four sections above answered with evidence, not a one-line assertion
- [x] No implementation line written for this ML

**Gates da wave:**
```bash
# Wave 0 gate — verifica que o ML-0A foi concluído (marcador ML-0A-DONE presente no roadmap).
grep -q "ML-0A-DONE" docs/roadmaps/wip/ROADMAP-2026-09-22-check-referential-integrity-sh-diz-ok-e-sai-0-sobre-arvore-vazia-quinto-gate-vacuo-e-esta-dentro-do-parity.md
```

## Wave 1 — Implementation (derived from REQ criteria)
> Dependencies: none

### ML-1A — **AC1** — Guarda de vacuidade: varredura que enumere **zero** itens **falha** ou reporta
**Status:** ✅ Concluído (2026-10-17)
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC1** — Guarda de vacuidade: varredura que enumere **zero** itens **falha** ou reporta
- [ ] build passes
- [ ] tests green

### ML-1B — **AC2** — 🔴 **Falsificação nas duas direções:** árvore vazia → **reprova**; árvore íntegra →
**Status:** ✅ Concluído (2026-10-17)
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC2** — 🔴 **Falsificação nas duas direções:** árvore vazia → **reprova**; árvore íntegra →
- [ ] build passes
- [ ] tests green

### ML-1C — **AC3** — O gate emite **contagem** do que verificou. Os demais gates ≤0,11 s têm guarda ou
**Status:** ✅ Concluído (2026-10-17)
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC3** — O gate emite **contagem** do que verificou. Os demais gates ≤0,11 s têm guarda ou
- [ ] build passes
- [ ] tests green

### ML-1D — **AC4** — 🔴 **Cenário em scripts/check-gates-falsify.sh.** Esta é a correção da **classe**,
**Status:** ✅ Concluído (2026-10-17)
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC4** — 🔴 **Cenário em scripts/check-gates-falsify.sh.** Esta é a correção da **classe**,
- [ ] build passes
- [ ] tests green

### ML-1E — **AC5** — O gate resolve req_dir/adr_dirs/roadmap_dir do trackfw.yaml em vez de
**Status:** ✅ Concluído (2026-10-17)
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC5** — O gate resolve req_dir/adr_dirs/roadmap_dir do trackfw.yaml em vez de
- [ ] build passes
- [ ] tests green

### ML-1F — **AC6** — make quality verde e trackfw validate exit 0.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC6** — make quality verde e trackfw validate exit 0.
- [ ] build passes
- [ ] tests green

---

## Triagem pós-v8 — 2026-10-17 (REQ-2026-10-17)

🔴 **Vivo** — prioridade 2: árvore sem REQ → "Referential integrity OK", exit 0 (reconferido). `req_dir` do yaml já é lido (AC5 feito).

Evidência e tabela completa: `docs/governanca/2026-10-17-triagem-backlog-pre-v8.md`.

### Ajuste do arquiteto à Wave 0 (2026-10-17)
O resíduo 4 (`2>/dev/null` no `find` engolindo erro de permissão) **não fica como resíduo**: é o
mesmo mecanismo desta REQ — o gate termina em OK sem ter conseguido olhar. Entra no ML-1A: `req_dir`
inexistente ou ilegível reprova com mensagem nomeando o caminho, e erro do `find` não é engolido.

## Entrega — 2026-10-17
`scripts/check-referential-integrity.sh`: `req_dir` inexistente → reprova nomeando o caminho e a
origem; zero REQs → reprova; erro do `find` não é mais engolido; OK com contagem
("Referential integrity OK: 257 REQs, 327 referencias verificadas" neste repositório).
Cenário 12 do falsify ganhou 7 braços, incluindo o script pré-correção embarcado (heredoc) que passa
na árvore vazia — prova de que o cenário mede. Não usa `git show origin/main:`, que após o merge
apontaria para a versão corrigida (desvio do handoff, correto). Reconferido pelo arquiteto: sem
diretório, vazio, referência quebrada → exit 1; íntegro → exit 0 com contagem.
