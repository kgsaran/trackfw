---
status: wip
date: 2026-08-29
req: "docs/req/REQ-2026-08-28-barrier-so-reconhece-cabecalho-de-aceite-em-portugues-mas-os-3-geradores-de-roadmap-escrevem-em-ingles.md"
squad: "hades-tf, apolo-tf, artemis-tf"
---

# Roadmap: Dialeto canônico do roadmap e vocabulário de status do `barrier`

> Created: 2026-08-29 | Status: wip

## Context

REQ: `REQ-2026-08-28-barrier-so-reconhece-cabecalho-de-aceite-em-portugues-mas-os-3-geradores-de-roadmap-escrevem-em-ingles.md`
ADR: `ADR-2026-08-29-dialeto-canonico-do-roadmap-e-vocabulario-de-status-que-o-barrier-reconhece.md`

**Um roadmap gerado pelo `trackfw roadmap new` e preenchido exatamente como o próprio template
instrui é reprovado pelo `barrier` em dois checks.** Medido com o binário 7.3.0:

```
- ML-1A: not complete (status: done)      ← mls_complete
✗ acceptance_evidence: blocked
- ML-1A: no acceptance block              ← acceptance_evidence
```

Dois defeitos de natureza diferente: o cabeçalho é problema de **idioma** (gerador escreve
`**Acceptance criteria:**`, barrier procura `**Critérios de aceite:**`); o status é problema de
**representação** (gerador escreve `pending`, barrier exige que a linha contenha `✅`).

Nenhum gate pega porque a paridade entre os 3 CLIs está intacta — os três erram igual. O contrato
quebrado é gerador↔verificador.

## Acceptance Criteria

Consolidado — AC1 a AC12 da REQ. **AC12 é a que define a REQ:** ciclo `roadmap new` → preencher →
`barrier passed`, com CLI real, sem edição manual.

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Modelo de ameaça deste roadmap
**Status:** ⬜ Pendente
**Agente:** `hades-tf`
**Files affected:** apenas este roadmap. Nenhum arquivo de produto.
**Actions:**
1. **Completude de enumeração.** O contrato gerador↔`barrier` tem **quantos** tokens, não só os dois
   já achados? Enumere **todos** os cabeçalhos e marcadores que o `barrier` parseia
   (`internal/commands/barrier.go:160-171` e equivalentes Node/Python) e confronte, um a um, com o
   que os 3 geradores escrevem (`internal/generators/roadmap.go`, `npm/src/generators/roadmap.js`,
   `pypi/trackfw/generators/roadmap.py`). Já sabidos: `**Acceptance criteria:**` vs
   `**Critérios de aceite:**` (diverge); `**Status:**` valor `pending` vs exigência de `✅`
   (diverge); `**Gates da wave:**` (concorda). Faltam: `^## Wave <label>`, `^### ML-\S+`,
   `^- \[ \]` / `^- \[.\]`, `^\*\*` como delimitador de bloco. **Para cada um, diga se o gerador
   produz forma que o parser aceita** — e não confie em que a lista acima esteja completa.
   > A Wave 0 da REQ anterior declarou enumeração fechada sobre um padrão de busca incompleto e
   > perdeu metade da superfície. Não repita: enumere pelo **parser**, não pela memória.
2. **Modelo de ameaça.** O vocabulário de status vai **crescer** (de `✅` para `✅|done|Concluído`) e
   o mecanismo vai mudar de `contains` para primeiro-token. Quem faz um ML **não** concluído passar
   por concluído sem quebrar nenhuma regra escrita? Cubra no mínimo: `não done`,
   `pending (era done)`, `notdone`, `done-not-really`, `**Status:** ` seguido de linha vazia,
   status com marcador dentro de código inline (`` `done` ``), status com caractere invisível ou
   zero-width antes do token, `✅` em posição não inicial (`⬜ Pendente ✅`), e status multilinha.
   Lembre que este é um check que **libera wave** — falso positivo aqui é trabalho incompleto sendo
   dado como pronto.
3. **Alvos de falsificação nas duas direções.** Para cada mudança: o que quebra se regredir (volta a
   exigir só `✅`, ou só o cabeçalho PT), **e** o que quebra se regredir para o lado oposto
   (aceita qualquer status não vazio; aceita `**Status:** não done`; o cabeçalho novo passa a casar
   dentro de bloco de código ou de prosa).
4. **Residual declarado.** O que este desenho aceita não cobrir. Inclua, no mínimo: roadmaps
   históricos com status fora do vocabulário fechado (`feito`, `ok`); a dupla forma de cabeçalho
   como superfície permanente; e o fato de o `barrier` passar a conhecer dois idiomas.
**Critérios de aceite:**
- [ ] As quatro seções respondidas com evidência (comando + saída), não asserção de uma linha
- [ ] A enumeração cobre **todos** os tokens do parser, não só os dois já conhecidos
- [ ] Nenhuma linha de implementação escrita neste ML

**Gates da wave:**
```bash
# Wave 0 gate — o conjunto de regexes de parsing do barrier tem que ser o que o ML-0A enumerou.
# Superfície nova no parser sem passar pela Wave 0 reabre a wave.
set -eu
n=$(sed -n '/^var (/,/^)/p' internal/commands/barrier.go | grep -c 'regexp.MustCompile' || true)
[ "$n" -eq 9 ] || { echo "barrier.go tem $n regexes de parsing, ML-0A enumerou 9 — reabrir a Wave 0" >&2; exit 1; }
echo "Wave 0 gate OK — 9 regexes de parsing enumeradas."
```

## Wave 1 — Parser do `barrier` nos 3 CLIs (ML único)
> Dependências: Wave 0 aprovada. **ML único e sequencial**: os 3 runtimes implementam a mesma regra
> de casamento, e três agentes em paralelo produziram divergência de comportamento na REQ anterior
> (ML-2C acrescentou uma linha; ML-3D deixou o Node mudo). Um agente, os 3 arquivos.

### ML-1A — Cabeçalho bilíngue e status por primeiro token
**Status:** ⬜ Pendente
**Agente:** `apolo-tf`
**Files affected:** `internal/commands/barrier.go`, `npm/src/commands/barrier.js`,
`pypi/trackfw/commands/barrier.py` e os testes correspondentes de cada runtime.
**Actions:**
1. `criteriaHeaderRe` (e equivalentes) passa a aceitar `**Acceptance criteria:**` **e**
   `**Critérios de aceite:**`. AC1, AC2, AC3.
2. A detecção de conclusão deixa de ser `contains(marker, "✅")` e passa a ser **primeiro token**:
   concluído quando o primeiro token do restante da linha é `✅`, `done` ou `Concluído` — insensível
   a caixa e a acento. AC8.
   > Os 3 CLIs hoje fazem substring: `barrier.go:554`, `barrier.js:134`, `barrier.py:207`.
   > Ampliar o vocabulário **sem** trocar o mecanismo faz `**Status:** não done` passar. Ver
   > `vault/notes/adr-status-substring-livre-falso-positivo-2026-08-01.md`.
3. Sufixos continuam válidos: `✅ Concluído · **Agente:** \`apolo-tf\`` e
   `✅ concluído (auditado 2026-08-02)` seguem sendo concluídos — são 48 ocorrências no corpus.
4. Paridade exata nos 3: mesmas formas aceitas, mesmas rejeitadas, mesma saída.
**Critérios de aceite:**
- [ ] AC1, AC2, AC3, AC8
- [ ] **AC9 provado por teste**, com os 6 casos negativos nomeados na REQ
- [ ] `go build ./...` → 0 · `go test ./...` → 0 · `npm test --prefix npm` → 0 ·
      `PYTHONPATH=pypi python3 -m pytest pypi/tests` → 0
- [ ] `./bin/trackfw barrier` sobre este próprio roadmap continua `passed`

## Wave 2 — Template e legenda (ML único)
> Dependências: Wave 1 concluída. Toca os 3 geradores; ML único pela mesma razão da Wave 1.

### ML-2A — `roadmap new` escreve a forma canônica e ensina a legenda
**Status:** ⬜ Pendente
**Agente:** `apolo-tf`
**Files affected:** `internal/generators/roadmap.go`, `npm/src/generators/roadmap.js`,
`pypi/trackfw/generators/roadmap.py` e testes.
**Actions:**
1. O template passa a escrever a forma canônica de status e a incluir a **legenda dos quatro
   estados** (⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado). AC11.
2. **Byte-identidade entre os 3 geradores** para o mesmo input.
3. `**Acceptance criteria:**` **permanece** — é a forma canônica pelo ADR. Não traduzir.
4. `**Gates da wave:**` **não muda**. Está no escopo negativo da REQ.
**Critérios de aceite:**
- [ ] AC11
- [ ] Template gerado byte-idêntico nos 3, provado por `diff`
- [ ] Testes dos 3 runtimes verdes

## Wave 3 — Gate de ciclo fechado e contrato
> Dependências: Waves 1 e 2 concluídas.

### ML-3A — Gate falsificável do contrato gerador↔`barrier`
**Status:** ⬜ Pendente
**Agente:** `artemis-tf`
**Files affected:** `scripts/check-roadmap-barrier-contract.sh` (novo), `docs/cli-parity.md`,
`Makefile`.
**Actions:**
1. Gate que executa o **ciclo fechado** com CLI real, nos 3 runtimes: `roadmap new` em sandbox →
   preencher status e critérios **seguindo apenas o que o template diz** → `roadmap move wip` →
   `barrier --wave N` → exigir `passed`. **AC12.** Nada de chamada de função interna: foi assim que
   o ML-2G da REQ anterior escapou da auditoria.
2. **AC10 — não reclassificação:** rodar o parser novo sobre os 143 roadmaps de `docs/roadmaps/**` e
   comparar ML a ML com o veredito atual. Emitir a tabela do antes/depois. A única diferença
   permitida é ML que dizia `done`/`Concluído` e passa a ser reconhecido.
3. Falsificação nas duas direções, com `assert_fails_with` mirando a razão que o **próprio gate**
   emite: cabeçalho PT deixa de ser aceito → reprova; `**Status:** não done` passa a ser aceito →
   reprova; template deixa de trazer a legenda → reprova.
4. Guarda de vacuidade obrigatória; contagem de cenários no fim.
5. Seção em `docs/cli-parity.md` documentando o contrato gerador↔`barrier`, anotada com `gate=`.
6. Registrar no `Makefile`.
**Critérios de aceite:**
- [ ] AC10, AC12, AC6 da REQ
- [ ] `bash scripts/check-roadmap-barrier-contract.sh` → exit 0 com contagem
- [ ] Guarda de vacuidade provada empiricamente
- [ ] `bash scripts/check-parity-contract-coverage.sh` → exit 0
- [ ] AC7: `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` → exit 0

## Barreira final
Revisão `hefesto-tf` (qualidade) e `hades-tf` (segurança — o `barrier` é um check que **libera
wave**: falso positivo aqui é trabalho incompleto dado como pronto). Auditoria de diff pelo
arquiteto e `trackfw barrier --wave 3`.
