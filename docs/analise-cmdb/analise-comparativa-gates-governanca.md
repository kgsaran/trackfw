# Análise comparativa — `trackfw validate` × gate de governança interno do CMDB

> **Origem:** integração do trackfw v2.1.1 no monorepo CMDB.
> **Autor:** 🌩️ Zeus — arquiteto do CMDB · **Data:** 2026-06-13
> **Objetivo:** dar ao time do trackfw um diagnóstico de campo, regra a regra, contra um validador
> independente que resolve o mesmo problema — destacando lacunas e oportunidades de evolução.

---

## 1. Os dois sujeitos da comparação

| | **Gate interno do CMDB** | **trackfw validate** |
|---|---|---|
| Arquivo / fonte | `scripts/validate-kanban-gate.mjs` (Node, ~900 linhas) | `internal/validator/validator.go` (Go) |
| Especificação | ADR-036 do CMDB (governança Kanban) | README do trackfw + código |
| Escopo | `docs/roadmaps/` + `docs/requisições/` (**não** olha ADRs) | `ADR → REQ → ROADMAP` (cadeia completa) |
| Versão analisada | estado em 2026-06-13 | trackfw **2.1.1** (homebrew) |

Ambos adotam o mesmo princípio: **a pasta é a fonte de verdade do estado**
(`backlog/wip/blocked/done/abandoned`). Divergem no rigor e na superfície de verificação.

---

## 2. Matriz A — Cobertura de regras (regra a regra)

Legenda: ✅ cobre · ⚠️ cobre parcialmente/fraco · ❌ não cobre.

| Regra / verificação | Gate interno (.mjs) | trackfw validate | Observação |
|---|:---:|:---:|---|
| **Pasta × status** (status declarado = pasta) | ✅ R1 *(estrutural)* | ❌ | trackfw infere estado pela pasta mas não cobra coerência do `status:` declarado |
| **`docs/roadmap/` singular proibido** | ✅ R2 *(estrutural)* | ❌ | anti-duplicação de estrutura |
| **Unicidade de filename** entre estados (mesmo agente) | ✅ R3 *(estrutural)* | ❌ | detecta artefato "clonado" em 2 estados |
| **Unicidade lógica por `req_id`** | ✅ R10 *(estrutural)* | ❌ | trackfw não tem conceito de ID estável |
| **Pareamento REQ↔ROADMAP** (existência + reverso + estado) | ✅ R9 *(estrutural, forte)* | ⚠️ #1/#4 *(substring)* | ver Achado 1 |
| **Evidência de validação em `done`** (build/test/gate/✅) | ✅ R8 *(estrutural)* | ❌ | impede "done" sem prova |
| **Refs de arquivo quebradas** no corpo | ✅ R6 *(warning)* | ❌ | valida caminhos citados existem |
| ROADMAP em **wip tem REQ** | ✅ R9 | ✅ #1 | sobreposição |
| ROADMAP em **blocked tem REQ** | ✅ R9 *(blocked é ativo)* | ✅ #3 | sobreposição |
| **REQ → ADR** vinculado | ❌ | ✅ #2 | **exclusivo trackfw** |
| **REQ → Roadmap** vinculado | ✅ R9 (reverso) | ✅ #4 | sobreposição |
| **ADR órfão** (sem REQ que o cite) | ❌ | ✅ #5 | **exclusivo trackfw** |
| **WIP tem bloco de Critérios de Aceite** | ❌ | ✅ #6 | **exclusivo trackfw** |
| **WIP limit** (foco — por agente/squad/global) | ❌ | ✅ #7 *(warning, configurável)* | **exclusivo trackfw** |
| **Stale WIP** (envelhecido) | ✅ R7 *(git commit, 21d)* | ✅ #8 *(mtime, 7d)* | ver Achado 3 |
| **REQ Open bloqueada por ADR Draft** | ❌ | ✅ #9 | **exclusivo trackfw** (gating de ciclo de vida do ADR) |
| **Frontmatter presente** | ✅ R5 (campo `status`) | ✅ #10 (bloco `---`) | sobreposição parcial |

**Placar de exclusividade:**
- Exclusivas do gate interno (7): R1 (pasta×status), R2 (estrutura singular), R3 (unicidade filename),
  R6 (refs quebradas), R8 (evidência em done), R10 (unicidade por `req_id`), e o **rigor** do R9.
- Exclusivas do trackfw (5): REQ→ADR, ADR órfão, critérios de aceite em WIP, WIP limit, ADR-Draft gating.
- Sobreposição genuína (3): wip-tem-REQ, stale WIP, frontmatter.

---

## 3. Matriz B — Design e mecânica

| Aspecto | Gate interno (.mjs) | trackfw validate |
|---|---|---|
| Escopo da cadeia | REQ ↔ ROADMAP (sem ADR) | ADR → REQ → ROADMAP (completo) |
| **Chave de pareamento** | `req_id` (estável, sobrevive a renomeação/transição) | substring textual (`REQ:`, `ADR:`, `Roadmap:`) |
| Verifica **reciprocidade** do vínculo | ✅ A→B **e** B→A + estados compatíveis | ❌ só presença de uma linha |
| Verifica **existência do alvo** referenciado | ✅ (resolve `req_id`/caminho) | ❌ (basta a substring existir) |
| Política de **legado/grandfather** | ✅ cutoff por data do ADR + isenção done/abandoned | ❌ por artefato; só `lenient`/`lenient_until` global |
| **Stale WIP — base temporal** | `git log` (commit real) | `mtime` (frágil: checkout/clone reseta) |
| **Modos** | report-only / `--strict` / `--json` | `strict` / `lenient` (+ data de expiração) |
| **Config** | env vars + paths hardcoded `docs/` | **declarativa** (`trackfw.yaml`) ✅ vantagem trackfw |
| **Namespacing por agente** | fixo `<agente>/<estado>` | configurável (by_agent / flat / by_squad) ✅ vantagem trackfw |
| Saída **JSON** para CI/automação | ✅ `--json` | ❌ (texto) |
| **i18n** | pt-BR fixo | pacote i18n multilíngue ✅ vantagem trackfw |
| **Ecossistema** além do gate | ❌ (só valida) | ✅ `status`, `metrics`, `log`, `serve`, `sync`, `context`, instaladores ✅ vantagem trackfw |

---

## 4. Achados acionáveis para o trackfw

### Achado 1 — Pareamento por substring é frágil (oportunidade: ID estável + reciprocidade)

**O que o trackfw faz hoje** (`internal/validator/validator.go`):
- `validateWIPHasREQ`: `strings.Contains(content, "REQ:")` no roadmap wip.
- `validateREQsHaveADR`: `strings.Contains(content, "ADR:")` na REQ.
- `validateREQsHaveRoadmap`: `strings.Contains(content, "Roadmap:")` na REQ.
- `validateADRsAreReferenced`: ADR órfão se o **basename** do arquivo não aparece em nenhuma REQ.

**Limitações:**
- Um vínculo que aponta para um alvo **inexistente** passa (a linha `REQ:` existe, mas o arquivo não).
- Não há **reciprocidade**: REQ→Roadmap não garante Roadmap→REQ correspondente.
- Não há **compatibilidade de estado** (REQ em `done` ↔ Roadmap em `wip` passa).
- `validateADRsAreReferenced` casa por substring de **basename** — sujeito a falso-positivo/negativo
  (basename que aparece como parte de outro texto, ou ADR citado por caminho diferente).

**Referência de design (gate interno, R9/R10):** usa um identificador estável `req_id` presente no
frontmatter de **ambos** os lados; valida (a) existência do par, (b) vínculo reverso por `req_id`,
(c) estados compatíveis, e (d) unicidade lógica (nenhum `req_id` em 2 REQs ou 2 ROADMAPs).

**Sugestão:** introduzir no trackfw um **ID de rastreabilidade opcional** (ex.: `trace_id`/`req_id` no
frontmatter) e, quando presente, validar pareamento **bidirecional + existência + estado**, mantendo o
fallback por substring para projetos sem ID. Eleva o rigor sem quebrar quem já usa as linhas textuais.

### Achado 2 — `adr_dirs` não-recursivo cega projetos com ADRs em subpastas (provável bug)

**Reprodução (ocorreu no CMDB):**
1. `trackfw.yaml` com `adr_dirs: [docs/adr/zeus]`.
2. O projeto organiza ADRs por estado: `docs/adr/zeus/done/*.md`, `docs/adr/zeus/wip/*.md`.
3. `validateADRsAreReferenced` e `validateFrontmatterPresence` usam `listDir(adrDir)` e
   `filepath.Glob(adrDir + "/*.md")` — **ambos não-recursivos**.
4. Resultado: **zero ADRs** detectados → os checks de ADR (REQ→ADR #2, ADR órfão #5, ADR-Draft #9,
   frontmatter de ADR #10) tornam-se **no-ops silenciosos**, sem erro nem aviso.

**Por que é grave:** o trackfw promete "validar a cadeia até o ADR", mas falha **em silêncio** num
layout de ADR perfeitamente razoável (e que é, inclusive, simétrico ao layout de roadmaps/requisições
que o próprio trackfw suporta com namespacing por agente). Silêncio é pior que erro: passa a falsa
sensação de cobertura.

**Sugestões (qualquer uma resolve):**
- Tornar a varredura de `adr_dirs` **recursiva** (`filepath.WalkDir`), como já é feito implicitamente
  para roadmaps por agente; **ou**
- Suportar **glob** em `adr_dirs` (ex.: `docs/adr/**/*.md` ou `docs/adr/<agente>/<estado>`); **ou**
- No mínimo, **emitir um warning** quando um `adr_dir` configurado contém subpastas mas nenhum `.md`
  direto — sinalizando provável layout aninhado não coberto.

### Achado 3 — Stale WIP por `mtime` é instável; considerar `git log`

`validateStaleWIP` usa `os.Stat().ModTime()`. Em CI, clones frescos e `checkout` reescrevem `mtime`,
zerando a idade real do WIP (falso negativo) — ou, ao contrário, um `touch` mascara progresso. O gate
interno usa `git log -1 --format=%ct` (data do último commit que tocou o arquivo), que reflete
atividade real e é estável entre clones. **Sugestão:** preferir `git log` quando o projeto for um repo
git, com fallback para `mtime`.

### Achado 4 (menor) — Conflito de convenção de documento

O trackfw espera marcadores inline (`REQ:`, `ADR:`, `Roadmap:`, `Status: Open`, `## Acceptance
Criteria`/`## Critérios de Aceite`, `## Blocked by ADRs`); o CMDB usa **frontmatter** (`req_id:`,
`roadmap:`, `status:`) por força do ADR-036. Rodar `trackfw validate` no CMDB gerou warnings
("7 roadmaps wip sem REQ") que são, na verdade, **divergência de gramática**, não dívida real.
**Sugestão:** documentar claramente a convenção esperada pelo `trackfw discover`/`init` e/ou permitir
**mapear campos** (ex.: aceitar `req_id` do frontmatter como satisfazendo "tem REQ").

---

## 5. Síntese e recomendação de posicionamento

- **trackfw** brilha como **ferramenta de cadeia + ecossistema**: cobre o elo ADR, disciplina de fluxo
  (WIP limit, critérios de aceite, ADR-Draft gating), config declarativa, multilíngue, e comandos
  satélite (`status`/`metrics`/`serve`/`sync`).
- O **gate interno do CMDB** é mais forte na **integridade referencial** (ID estável, reciprocidade,
  pasta×status, evidência em done, grandfather por data) e tem **saída JSON** para CI.

As duas filosofias não competem — **convergem**. Os achados 1–3 são exatamente as arestas que tornariam
o trackfw tão rigoroso quanto o gate interno **sem perder** sua amplitude e ergonomia. Se incorporados,
o trackfw poderia **substituir** o gate `.mjs` no CMDB (em vez de coexistir), reduzindo manutenção de
dois validadores.

---

## Anexo — Inventário de regras do gate interno (referência)

| ID | Regra | Severidade |
|----|-------|-----------|
| R1 | Pasta × status | estrutural |
| R2 | `docs/roadmap/` singular proibido | estrutural |
| R3 | Unicidade de filename entre estados | estrutural |
| R4 | ROADMAP wip/backlog sem REQ | warning (endurecido por R9) |
| R5 | Frontmatter sem `status` | warning |
| R6 | Referências de arquivo quebradas | warning |
| R7 | WIP envelhecido (git, 21d) | warning |
| R8 | `done` sem evidência de validação | estrutural (pós-cutoff) |
| R9 | Pareamento bidirecional por `req_id` | estrutural |
| R10 | Unicidade lógica por `req_id` | estrutural |

## Anexo — Inventário de checks do `trackfw validate` (referência)

| # | Função | Tipo |
|---|--------|------|
| 1 | `validateWIPHasREQ` | violation |
| 2 | `validateREQsHaveADR` | violation |
| 3 | `validateBlockedHasREQ` | violation |
| 4 | `validateREQsHaveRoadmap` | violation |
| 5 | `validateADRsAreReferenced` (ADR órfão) | violation |
| 6 | `validateWIPHasAcceptanceCriteria` | violation |
| 7 | `validateWIPLimit` (por agente/squad/global) | warning |
| 8 | `validateStaleWIP` (mtime, 7d) | warning |
| 9 | `validateREQsNotBlockedByDraftADRs` | violation |
| 10 | `validateFrontmatterPresence` (ADR/REQ) | violation |
| — | Modo `lenient`/`lenient_until` → violations viram warnings | modo |
