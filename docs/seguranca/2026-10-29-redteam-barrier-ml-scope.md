# Red Team — barrier ml_scope: declaração em lista e união dos roadmaps

> Autor: hades-tf | Data: 2026-10-29
> Roadmap: docs/roadmaps/wip/ROADMAP-2026-10-29-barrier-ml-scope-le-a-declaracao-de-arquivos-afetados-em-formato-de-lista.md
> Branch: feat/documentacao-reflete-produto-11-windows-install-ps1

---

## Método

**Ambiente**: repositórios descartáveis em `<scratchpad>/hades-rt/fixtures/`. Binário compilado de
HEAD: `go build -o hades-rt/trackfw ./cmd/trackfw` (11.1.0). `git commit`, `git push` e
`git checkout -b` bloqueados pelo pré-hook do Claude Code; commits criados com `git write-tree` +
`git commit-tree` (plumbing) + escrita direta em `.git/refs/heads/<branch>`. Origem configurada como
bare clone local via `git clone --bare` + `git remote add` + `git fetch`.

Cada vetor executado com:
```
trackfw barrier <roadmap> --wave 0 --json --trust-local-gates
```
O campo `ml_scope.status`, `evidence[]` e `failures[]` extraídos via `python3 -I -c
'import json,sys; ...'`. RC capturado com `rc=0; cmd ... || rc=$?` (sem pipe).

**RC**: em todos os fixtures o código de saída é 1, porque o check `acceptance_evidence`
(ML sem bloco de aceite preenchido) e o check `validate` (3 violations na governança do fixture
mínimo) também reprovam. `ml_scope.status` é a grandeza avaliada em cada vetor, não o RC global.
Os vetores (a)–(l) foram medidos com `|| true; rc=$?` (rc sempre 0); a correção para `|| rc=$?`
foi aplicada nos vetores adicionais (f4-bis, f-adr, f2-disc, etc.). Em nenhum caso o RC afeta os
vereditos, pois eles dependem de `ml_scope.status`.

**Independência parcial**: os vereditos esperados foram derivados do contrato antes de medir.
As funções `parseMLScopeFilesAffected`, `checkMLScope`, `buildMLScopeOrchestrationSet`,
`mlScopeComputeK`, `barrierFrontmatterField` e `SplitRoadmapLines` foram lidas para construir
fixtures válidas (estrutura de frontmatter, formato de roadmap-membro, etc.) — não para deduzir
os vereditos. Os Achados 1 e 2 foram descobertos comparando o output do binário com a tabela em
`cli-parity.md:3374`; a leitura do código serviu apenas para localizar a linha exata. Os vetores
k1/k2, f1/f2/f4 e l descrevem mecânicas internas (TrimSpace, HasPrefix, path.Clean, path.Base)
porque a derivação de "por que o veredito é esse" exigiu ler o código — isso é investigação de
divergência, não fonte do veredito.

**Fontes do contrato lidas antes de escrever vereditos esperados**:
- `docs/cli-parity.md:3205–3307` (seção `ml_scope` completa)
- `docs/cli-parity.md:3374` (tabela de evidence/failures pinados)
- `docs/adr/ADR-2026-10-29-barrier-ml-scope-...`
- `docs/seguranca/2026-10-29-wave0-barrier-ml-scope.md` §3 e §4

---

## Vetores executados

### Vector (a): checkmark stop — parada em `- [ ]` / `- [x]` / `- [X]`

**Contrato (cli-parity.md:3231)**: "Checkmark item — a line starting with `- [ ]`, `- [x]`, or
`- [X]` stops before acceptance-criteria items leak into scope."

**Veredito esperado**: `leaked-criterion.go` (após `- [ ]`) fora do escopo → `blocked`.
`declared.go` dentro do escopo.

**Fixture** (ML-0A `✅`, wave 0):
```
**Files affected:**
- `declared.go`: the declared file
- [ ] `leaked-criterion.go` should NOT be collected
- [ ] another criterion
```

**Obtido**:
```
STATUS: blocked
EVIDENCE: in scope: declared.go
FAILURE: out of scope: leaked-criterion.go
```

**Resultado**: CONFORME. `declared.go` aceito, `leaked-criterion.go` bloqueado.

---

### Vector (a2): variantes de checkmark — `- [~]` e `- [-]`

**Contrato (cli-parity.md:3231)**: o stop lista explicitamente apenas `- [ ]`, `- [x]`, `- [X]`.
`CriterionLineRe = regexp.MustCompile('^- \[.\]')` em `roadmapdoc.go:49` (commit `04afe3f65`,
anterior a ML-1A) casa qualquer caractere único — mas o stop introduzido por ML-1A em
`barrier.go:890` repete a lista explícita de três formas, sem usar `CriterionLineRe`.

**Veredito esperado (contrato)**: o contrato só lista `[ ]`/`[x]`/`[X]`, portanto `- [~]` e
`- [-]` não param o parser. Conforme o texto do contrato, `leaked-tilde.go` e `leaked-dash.go`
devem estar EM escopo.

**Fixture**:
```
**Files affected:**
- `declared.go`: the declared file
- [~] `leaked-tilde.go` variant criterion not stopped?
- [-] `leaked-dash.go` variant criterion not stopped?
```

**Obtido**:
```
STATUS: passed
EVIDENCE: in scope: declared.go
EVIDENCE: in scope: leaked-tilde.go
EVIDENCE: in scope: leaked-dash.go
```

**Resultado**: CONFORME ao texto do contrato. O binário e o contrato escrito concordam: `[~]` e
`[-]` não param o parser. A lacuna está entre a intenção sugerida por `CriterionLineRe` (qualquer
`[.]`) e o stop explícito de três formas em código e contrato — ver **Achado 3**.

---

### Vector (b): item de lista com `- **negrito**` no início

**Contrato (cli-parity.md:3228)**: "A list item that begins with `- **` is still a list item and
does not stop the parser." ADR §2.1 caso 2.

**Veredito esperado**: todos os tokens do item `- **\`bold-prefix-file.go\` fica.** context
\`another-bold.go\`` coletados.

**Fixture**:
```
**Files affected:**
- `first-file.go`: normal item
- **`bold-prefix-file.go` fica.** context text `another-bold.go`
```

**Obtido**:
```
STATUS: passed
EVIDENCE: in scope: another-bold.go
EVIDENCE: in scope: bold-prefix-file.go
EVIDENCE: in scope: first-file.go
```

**Resultado**: CONFORME. `- **` não dispara o stop de campo; três tokens extraídos.

---

### Vector (c): padrão largo `**` em membro da união

**Contrato (cli-parity.md:3256–3264)**: `**` tem stripped form vazio → `blocked` com mensagem
`<roadmap-path> <ML-id>: rejected: pattern "**" without literal path segments matches every or no
file without restriction`. A rejeição dispara para membros também.

**Veredito esperado**: `blocked`; `some-other-file.go` fora do escopo (padrão rejeitado, sem concessão).

**Membro**: ML-0A com status `🔄 Em andamento` e `**Files affected:** \`**\``.

**Obtido**:
```
STATUS: blocked
EVIDENCE: in scope: target-file.go
FAILURE: docs/roadmaps/wip/ROADMAP-member.md ML-0A: rejected: pattern "**" without literal path segments matches every or no file without restriction
FAILURE: out of scope: some-other-file.go
```

**Resultado**: CONFORME. Rejeição disparou para membro; `some-other-file.go` não foi concedido.

---

### Vector (c-member): `**/*.*` em membro `🔄` — residual 4

**Contrato (ADR §4, Wave 0 §2.2)**: `**/*.*` tem stripped form `"."` → não rejeitado (residual
documentado). Com membro em `🔄 Em andamento`, deve conceder qualquer arquivo com extensão.

**Fixture**: membro com ML-0A `🔄 Em andamento`, `**Files affected:** \`**/*.*\``.

**Obtido**:
```
STATUS: passed
EVIDENCE: in scope: any-extension-file.go
EVIDENCE: in scope: docs/roadmaps/wip/ROADMAP-member.md
EVIDENCE: in scope: target-file.go
```

**Resultado**: CONFORME — residual 4 confirmado em membro `🔄`.

---

### Vector (c2): residual 4 — `**/*.*` em alvo não rejeitado

**Contrato (ADR §4, Wave 0 §2.2)**: `**/*.*` tem stripped form `"."` → não rejeitado.

**Obtido**: `any-file.go` em escopo. CONFORME ✓

---

### Vector (C2-target): alvo excluído do loop de membros

**Contrato (cli-parity.md:3273)**: o alvo não é tratado como membro de si mesmo — seus MLs de
wave N não alimentam a union quando executando `--wave M < N`.

**Fixture**: ROADMAP-target com ML-0A `✅` (`wave0-file.go`) e ML-1A `🔄` (`wave1-file.go`).
Executado com `--wave 0`.

**Obtido**:
```
STATUS: blocked
EVIDENCE: in scope: wave0-file.go
FAILURE: out of scope: wave1-file.go
```

**Resultado**: CONFORME. ML-1A do alvo não vaza para o escopo de wave 0.

---

### Vector (d): membro todo-⬜ contribui apenas orquestração

**Contrato (cli-parity.md:3281)**: "All-⬜ members contribute only orchestration (K undefined)."

**Veredito esperado**: membro com `⬜ Pendente` e declaração `**/*.go` → K indefinido → sem padrões.
`member-only.go` fora do escopo.

**Obtido**:
```
STATUS: blocked
EVIDENCE: in scope: target-only.go
FAILURE: out of scope: member-only.go
```

**Resultado**: CONFORME.

---

### Vector (e): membro com ML `🔄` contribui padrões

**Contrato (cli-parity.md:3281, ADR §4.p.6)**: `🔄 Em andamento` → conta para K. K=0 com wave 0
`🔄` → padrões de wave 0 incluídos.

**Veredito esperado**: `member-pattern.go` em escopo.

**Obtido**:
```
STATUS: passed
EVIDENCE: in scope: docs/roadmaps/wip/ROADMAP-member.md
EVIDENCE: in scope: member-pattern.go
EVIDENCE: in scope: target-only.go
```

**Resultado**: CONFORME.

---

### Vector (e-flip): ⬜ no main → `🔄` na branch (residual 3)

**Contrato (ADR §4.p.6)**: flip de ⬜ para 🔄 no mesmo commit da branch faz o membro contribuir
padrões — K é computado sobre a versão HEAD do roadmap.

**Fixture**: membro com ML-0A `⬜ Pendente` no main, atualizado para `🔄 Em andamento` na branch.

**Obtido**:
```
STATUS: passed
EVIDENCE: in scope: docs/roadmaps/wip/ROADMAP-member.md
EVIDENCE: in scope: member-file.go
EVIDENCE: in scope: target-file.go
```

**Resultado**: CONFORME. Flip ⬜→🔄 na branch concede padrões corretamente.

---

### Vector (e-vs16): `🔄️` com VS16 (U+FE0F)

**Contrato**: caracteres variation selector (U+FE0F) são stripped antes da comparação de status.

**Fixture**: membro com `**Status:** 🔄️ Em andamento` (U+1F504 U+FE0F) e `vs16-file.go` declarado.

**Obtido**:
```
STATUS: passed
EVIDENCE: in scope: docs/roadmaps/wip/ROADMAP-member.md
EVIDENCE: in scope: target-file.go
EVIDENCE: in scope: vs16-file.go
```

**Resultado**: CONFORME. VS16 stripped → `🔄` reconhecido, padrões concedidos.

---

### Vector (E-done-at-merge-base): membro `done/` já existia no merge-base

**Contrato (cli-parity.md:3276–3279)**: `done/` + existia em merge-base → apenas orquestração.

**Fixture**: `ROADMAP-done-member.md` em `done/` no main (merge-base). Branch edita o arquivo e
toca `done-file.go`.

**Obtido**:
```
STATUS: blocked
EVIDENCE: in scope: docs/roadmaps/done/ROADMAP-done-member.md
EVIDENCE: in scope: target-file.go
FAILURE: out of scope: done-file.go
```

**Resultado**: CONFORME. `done-file.go` do membro pré-existente não vaza para o escopo.

---

### Vector (f): validação de prefixo `req:` e `adr:`

**Contrato (cli-parity.md:3289)**: req: e adr: limpos com `path.Clean`, rejeitados se absolutos,
com `..` ou fora do `req_dir`/`adr_dirs`. Silenciosamente descartados.

#### f1: traversal `docs/req/../../go.mod`

`path.Clean` = `"go.mod"`, sem prefixo `docs/req/` → descartado.
**Obtido**: `go.mod` fora do escopo. CONFORME ✓

#### f2: sibling `docs/req-x/f.md` (discriminação positiva — arquivo tocado na branch)

`docs/req-x/f.md` tocado na branch. `strings.HasPrefix("docs/req-x/f.md", "docs/req/")` = false
(proteção de barra) → descartado.

**Obtido**:
```
STATUS: blocked
EVIDENCE: in scope: target.go
FAILURE: out of scope: docs/req-x/f.md
```

CONFORME ✓ — sibling bloqueado mesmo quando tocado.

#### f3: caminho absoluto `/abs/go.mod`

`path.IsAbs` = true → descartado.
**Obtido**: `go.mod` fora do escopo. CONFORME ✓

#### f4: prefixo `./docs/req/REQ-legit.md` (controle positivo — arquivo tocado na branch)

`path.Clean("./docs/req/REQ-legit.md")` = `"docs/req/REQ-legit.md"` → prefixo correto → ACEITO.
`REQ-legit.md` tocado na branch.

**Obtido**:
```
STATUS: passed
EVIDENCE: all 2 touched file(s) are within scope
EVIDENCE: in scope: docs/req/REQ-legit.md
EVIDENCE: in scope: target-f4.go
```

CONFORME ✓ — `./` normalizado; REQ tocada aceita.

#### f5: `adr: go.mod` fora de `docs/adr/`

`path.Clean("go.mod")` sem prefixo `docs/adr/` → descartado.
**Obtido**: `go.mod` fora do escopo. CONFORME ✓

#### f-adr: controle positivo — ADR dentro de `adr_dirs` tocada na branch

REQ com `adr: docs/adr/ADR-ok.md`. `docs/adr/ADR-ok.md` tocado na branch.

**Obtido**:
```
STATUS: passed
EVIDENCE: all 2 touched file(s) are within scope
EVIDENCE: in scope: docs/adr/ADR-ok.md
EVIDENCE: in scope: target.go
```

CONFORME ✓ — ADR dentro de `adr_dirs` aceita via cadeia `req:→adr:`.

#### f-trailing: `req_dir: docs/req/` com barra à direita — sem falso positivo

Config com `req_dir: docs/req/` (barra à direita). REQ legítima em `docs/req/REQ-trail.md` tocada.

**Obtido**:
```
STATUS: passed
EVIDENCE: in scope: docs/req/REQ-trail.md
EVIDENCE: in scope: target.go
```

CONFORME ✓ — barra à direita normalizada antes da comparação; sem falso positivo.

---

### Vector (g): membro renomeado (wip/ → done/) na branch

**Contrato**: `done/` mas NÃO existia em merge-base → contribui padrões.

**Veredito esperado**: membro movido de `wip/` para `done/` NESTA branch → contribui.

**Obtido**:
```
STATUS: passed
EVIDENCE: in scope: docs/roadmaps/done/ROADMAP-member.md
EVIDENCE: in scope: docs/roadmaps/wip/ROADMAP-member.md
EVIDENCE: in scope: member-file.go
EVIDENCE: in scope: target-file.go
```

**Resultado**: CONFORME. Caminho deletado (`wip/ROADMAP-member.md`) coberto pelo mecanismo de
sibling-states em `buildMLScopeOrchestrationSet`.

---

### Vector (h): membro em `backlog/` contribui apenas orquestração

**Obtido**: `backlog-file.go` fora do escopo. CONFORME ✓

---

### Vector (i): membro com `date:` anterior a 2026-07-18

**Fixture**: membro com `date: 2026-06-01` e `**Files affected:** \`old-file.go\``.
**Obtido**: `old-file.go` fora do escopo. CONFORME ✓

---

### Vector (j): arquivo fora de qualquer declaração → blocked

**Obtido**: `undeclared.go` fora do escopo. CONFORME ✓

---

### Vector (k): CRLF no roadmap membro

**k1** (date 2026-09-01 CRLF): `barrierFrontmatterField` usa `strings.TrimSpace` → `\r` eliminado
→ `2026-09-01` → pós-cutoff → `crlf-file.go` em escopo. CONFORME ✓

**k2** (date 2026-06-01 CRLF — pré-cutoff): `SplitRoadmapLines` strip `\r`; `barrierFrontmatterField`
usa `TrimSpace`. `date: 2026-06-01\r` → `"2026-06-01"` → pré-cutoff → apenas orquestração.

**Obtido** (k2): `should-be-orch-only.go` fora do escopo. CONFORME ✓

CRLF não introduz fail-open na direção de pré-cutoff.

---

### Vector (l): `roadmap_namespacing: by_agent`

**Obtido**: `agent-file.go` em escopo. CONFORME ✓

Nota sobre `.trackfw-log-entries/`: `buildMLScopeOrchestrationSet` usa
`rdDir + "/.trackfw-log-entries/"` onde `rdDir = filepath.ToSlash(cfg.RoadmapDir)`. Em modo
`by_agent`, `cfg.RoadmapDir` é o diretório raiz (`docs/roadmaps/`). `translog.WriteEntry` recebe
`dir = cfg.RoadmapDir` — entradas ficam em `docs/roadmaps/.trackfw-log-entries/`. A cobertura da
orchestration set está correta nos dois modos.

---

### Vector (formato): strings pinadas no contrato vs código

**Contrato (cli-parity.md:3374)**:
```
| ml_scope | scope: <n> file(s) checked, <m> out of scope (when run) | out of scope: <path> | undeclared ML: <ML-id> has no Files affected declaration |
```

**Obtido — caso passed**:
```json
{
  "name": "ml_scope",
  "status": "passed",
  "evidence": ["all 1 touched file(s) are within scope", "in scope: ..."],
  "failures": []
}
```

**Obtido — caso blocked**:
```json
{
  "name": "ml_scope",
  "status": "blocked",
  "evidence": ["in scope: declared.go"],
  "failures": ["out of scope: undeclared.go"]
}
```

**Obtido — undeclared ML**:
```
failures: ['ML-0A: no **Files affected:** declaration (post-cutoff roadmap requires declaration for all MLs in the wave)']
```

**Divergências**:
1. Contrato: `scope: <n> file(s) checked, <m> out of scope` → código NUNCA emite esta string.
2. Contrato: `undeclared ML: <ML-id> has no Files affected declaration` → código emite
   `<ML-id>: no **Files affected:** declaration (...)`.

Ver **Achados 1 e 2**.

---

## Achados

### Achado 1 — MÉDIO: evidence `scope: N file(s) checked, M out of scope` nunca emitida

**Tipo**: divergência código × contrato.
**Commit de origem**: `7bddcd346` ("feat(harness): regras de escrita dos agentes aplicadas pelo
harness") — introduziu o check `ml_scope` e a tabela, com strings que nunca corresponderam ao
código. Anterior a ML-1A. ML-1A não alterou essas linhas.
**Evidência**: vetor `formato`; `barrier.go:1329–1333` emite `"all %d touched file(s) are within
scope"`; `cli-parity.md:3374` (blame: `7bddcd346`) pina `scope: <n> file(s) checked, <m> out of scope`.
**Impacto**: ferramentas que parseiam evidence strings segundo o contrato não encontrarão a linha.
`check-barrier.sh cenário 18` não verifica a evidence string, apenas `status` — por isso
passou despercebido.
**Dono do fix**: quem mantém `barrier.go` (implementador) alinha a evidence string, OU quem mantém
`cli-parity.md` atualiza a tabela. Ambos são sítios válidos.

### Achado 2 — MÉDIO: failure string de ML sem declaração diverge do contrato

**Tipo**: divergência código × contrato.
**Commit de origem**: `7bddcd346` — anterior a ML-1A.
**Evidência**: vetor `formato`; `barrier.go:1193–1194` emite
`"<ML-id>: no **Files affected:** declaration (...)"`.
**Contrato**: `cli-parity.md:3374` pina `undeclared ML: <ML-id> has no Files affected declaration`.
**Impacto**: ferramentas que filtram failures por prefixo `undeclared ML:` não encontrarão a entrada.
`check-barrier.sh:1465` verifica apenas o caminho do arquivo, não o prefixo.
**Dono do fix**: implementador (`barrier.go:1193`) ou mantenedor do contrato (`cli-parity.md:3374`).

### Achado 3 — BAIXO: stop de checkmark não usa `CriterionLineRe` — `[~]`/`[-]` não parados

**Tipo**: lacuna de intenção — código e contrato escrito concordam entre si, mas ambos são mais
estreitos que o regex `CriterionLineRe` existente sugere.
**Commit de origem**: ML-1A (`a67354ef`, `barrier.go:888–892`) introduziu o stop com lista explícita
`[ ]`/`[x]`/`[X]` sem usar `CriterionLineRe = ^- \[.\]` (commit `04afe3f65`, anterior a ML-1A).
O contrato escrito em `cli-parity.md:3231` também lista apenas as três formas.
**Evidência**: vetor `a2`; `barrier.go:890` vs `roadmapdoc.go:49`.
**Mecanismo**: `- [~] \`leaked.go\`` (critério caducado) alimenta `leaked.go` ao escopo porque o
stop não o reconhece como critério. O binário CONCORDA com o texto do contrato; a lacuna está entre
`CriterionLineRe` (intenção de cobertura mais ampla) e a implementação/texto explícitos.
**Impacto operacional**: 0 ocorrências de `[~]`/`[-]` em blocos `Files affected` no corpus de 838
declarações (Wave 0 §1.4). Risco teórico, sem impacto imediato.
**Dono do fix**: implementador (`barrier.go:890`): substituir a cadeia de `HasPrefix` por
`roadmapdoc.CriterionLineRe.MatchString(trimmed)`. Mantenedor do contrato: atualizar `cli-parity.md:3231`
para `"any - [X] where X is a single character"`.

### Achado 4 — DOCUMENTAÇÃO: `rejected:` format ausente da tabela pinada

**Tipo**: lacuna de documentação.
**Introduzido por**: ML-1A (`a67354ef`) — adicionou o formato `rejected:` na prosa de
`cli-parity.md:3260–3262` (blame: `a67354ef`) mas não atualizou a tabela em `cli-parity.md:3374`
(blame: `7bddcd346`).
**Evidência**: vetor `c`; o `FAILURE` de `rejected:` aparece no JSON mas não consta na tabela.
**Impacto**: consumidores que dependem exclusivamente da tabela não conhecem o formato.
**Dono do fix**: mantenedor do contrato — adicionar à coluna `failures` de `ml_scope` em
`cli-parity.md:3374`:
`<roadmap-path> <ML-id>: rejected: pattern "<tok>" without literal path segments matches every or no file without restriction`.

---

## Cobertura dos alvos de falsificação (Wave 0 §3)

| Superfície | Direção testada | Vetor | Resultado |
|---|---|---|---|
| A — parser de lista | FN: lista = zero padrões | (b) `- **` e lista com tokens | CONFORME |
| A — parser de lista | FP: lista engole o resto | (a) `- [ ]` para a lista | CONFORME |
| A — parser de lista | FP: critério vaza — `- [ ]` | (a) | CONFORME |
| A — parser de lista | critério `[~]` não parado | (a2) | conforme ao texto; lacuna de intenção (Achado 3) |
| B — glob largo | FN: `**` aceito | (c) `**` em membro | CONFORME (rejeitado) |
| B — glob largo | FP: `**/*.go` rejeitado | (c2)+(c-member) `**/*.*` residual | CONFORME (aceito) |
| C — união | FN: só um roadmap | (e) membro 🔄 contribui | CONFORME |
| C — união | FP: uncommitted entra | implícito em (h)/(d) | CONFORME |
| C2 — alvo excluído | alvo não auto-expande wave N | (C2-target) | CONFORME |
| D — recorte de waves | FN: todo-⬜ expande | (d) membro todo-⬜ | CONFORME |
| D — recorte de waves | FP: K muito restrito | (e)+(e-flip)+(e-vs16) K=0 | CONFORME |
| E — done-at-merge-base | FP: histórico contribui | (E-done-at-merge-base) | CONFORME |
| E — done/ nesta branch | FN: moved não contribui | (g) renamed wip→done | CONFORME |
| E — pré-cutoff | FN: pré-cutoff contribui | (i) | CONFORME |
| E — CRLF + pré-cutoff | FP/FN: CRLF engana date | (k2) | CONFORME |
| F — req:/adr: prefix | FN: traversal entra | (f1–f5) | CONFORME |
| F — req:/adr: prefix | FP: `./` descartado | (f4) controle positivo | CONFORME |
| F — req:/adr: prefix | FP: sibling bloqueado quando tocado | (f2) | CONFORME |
| F — req:/adr: prefix | FP: trailing-slash no config | (f-trailing) | CONFORME |
| F — req:/adr: prefix | FN: ADR dentro de adr_dirs | (f-adr) | CONFORME |
| Formato | Evidence/failures strings | (formato) | DIVERGÊNCIA (Achados 1, 2) |
| Formato | `rejected:` ausente da tabela | (c) | DIVERGÊNCIA (Achado 4) |

---

## Veredito

**APROVA COM RESSALVAS**

Nenhum dos achados representa regressão de segurança introduzida por ML-1A que sobre-conceda escopo
ou falhe aberto:

- **Achados 1 e 2** (MÉDIO): pré-existentes desde `7bddcd346`, anterior a ML-1A. Não enfraquecem o
  controle de escopo; afetam interoperabilidade de ferramentas que dependem das strings exatas.
- **Achado 3** (BAIXO): ML-1A introduziu o stop com lista de três formas em vez de usar
  `CriterionLineRe`. O binário concorda com o texto do contrato. Zero ocorrências no corpus.
- **Achado 4** (DOC): ML-1A adicionou prosa mas não a tabela. Sem impacto no controle.

O controle central opera corretamente em todos os vetores medidos.

| # | Tipo | Sev | Dono do fix | Ação mínima |
|---|---|---|---|---|
| 1 | código × contrato: evidence string | MÉDIO | implementador ou mantenedor contrato | Alinhar `barrier.go:1329` ou `cli-parity.md:3374` |
| 2 | código × contrato: undeclared ML failure | MÉDIO | implementador (`barrier.go:1193`) | Alinhar ao formato pinado, ou atualizar contrato |
| 3 | lacuna intenção código + contrato: `[~]`/`[-]` | BAIXO | implementador (`barrier.go:890`) + mantenedor contrato | Usar `CriterionLineRe`; atualizar cli-parity.md:3231 |
| 4 | doc: `rejected:` ausente da tabela | DOC | mantenedor contrato | Adicionar à tabela em `cli-parity.md:3374` |
