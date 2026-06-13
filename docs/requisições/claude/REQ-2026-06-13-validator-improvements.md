---
name: REQ-2026-06-13-validator-improvements
title: "Melhorias no validador — adr_dirs recursivo, stale por git log, existência de refs, coerência pasta×status, unicidade de filename"
status: Open
linked_adr: —
linked_roadmap: docs/roadmaps/claude/wip/v2.3-validator-improvements-2026-06-13.md
created: 2026-06-13
author: zeus
---

# REQ: Melhorias no validador trackfw validate

| Campo | Valor |
|---|---|
| Status | Open |
| Criado | 2026-06-13 |
| Roadmap | [v2.3-validator-improvements-2026-06-13](../../../roadmaps/claude/wip/v2.3-validator-improvements-2026-06-13.md) |

---

## Origem

Análise comparativa entre `trackfw validate` e o gate de governança do projeto CMDB (`scripts/validate-kanban-gate.mjs`), realizada em 2026-06-13. Documento: `docs/analise-cmdb/analise-comparativa-gates-governanca.md`.

---

## Problemas identificados

### Bug 1 — `adr_dirs` não-recursivo (Achado 2 da análise)

`validateADRsAreReferenced`, `validateFrontmatterPresence` e `adrIsDraft` usam `listDir` / `filepath.Glob(adrDir + "/*.md")` — **não-recursivos**. Quando ADRs ficam em subpastas (`docs/adr/zeus/done/*.md`), os checks 2, 5, 9 e 10 viram no-ops silenciosos. O validate passa verde mentindo.

### Bug 2 — Stale WIP por `mtime` instável (Achado 3 da análise)

`validateStaleWIP` usa `os.Stat().ModTime()` / `fs.statSync().mtimeMs`. Em CI, clones e checkouts reescrevem `mtime`, zerando a idade real. O gate interno usa `git log -1 --format=%ct`. A solução correta é `git log` com fallback para `mtime` quando não for repo git.

### Melhoria 3 — Referências a arquivos inexistentes passam em silêncio (Achado 1 parcial)

`validateWIPHasREQ`, `validateREQsHaveADR`, `validateREQsHaveRoadmap` e `validateADRsAreReferenced` verificam apenas presença de substring (`REQ:`, `ADR:`, `Roadmap:`). Se o valor da referência for um caminho `.md`, o arquivo pode não existir e nenhum erro é emitido.

### Melhoria 4 — Sem coerência pasta × status declarado (R1 do gate interno)

O `status:` declarado no frontmatter pode divergir da pasta onde o arquivo está. Drift silencioso em edições manuais.

### Melhoria 5 — Mesmo filename em dois estados passa em silêncio (R3 do gate interno)

Copiar em vez de mover um roadmap resulta em dois arquivos com o mesmo nome em pastas diferentes — o validate não detecta.

---

## Critérios de Aceite

### Bug 1 — adr_dirs recursivo
- [ ] `validateADRsAreReferenced` detecta ADRs em `docs/adr/zeus/done/`, `docs/adr/zeus/wip/`, etc.
- [ ] `validateFrontmatterPresence` inspeciona ADRs em subpastas
- [ ] `adrIsDraft` resolve ADRs em subpastas
- [ ] Quando `adrDir` configurado existe mas não contém `.md` direto (só subpastas), **emite warning** informando layout aninhado detectado
- [ ] Paridade nos 3 CLIs

### Bug 2 — Stale por git log
- [ ] `validateStaleWIP` usa `git log -1 --format=%ct -- <arquivo>` para obter data real do último commit
- [ ] Fallback para `mtime` quando `git log` falha (dir não é repo git, git não instalado)
- [ ] Paridade nos 3 CLIs

### Melhoria 3 — Existência de referências
- [ ] Quando o valor após `REQ:`, `ADR:` ou `Roadmap:` termina em `.md`, verificar se o arquivo existe
- [ ] Arquivo inexistente → warning (não violation, para não quebrar projetos com referências parciais)
- [ ] Paridade nos 3 CLIs

### Melhoria 4 — Coerência pasta × status
- [ ] Para cada arquivo em `wip/`, `backlog/`, `blocked/`, `done/`, `abandoned/`: verificar que `status:` no frontmatter bate com a pasta
- [ ] Divergência → warning
- [ ] Se arquivo não tem frontmatter `status:`, não emite (coberto pelo check #10 de frontmatter)
- [ ] Paridade nos 3 CLIs

### Melhoria 5 — Unicidade de filename entre estados
- [ ] Para roadmaps: detectar mesmo filename em dois ou mais estados diferentes
- [ ] Duplicata → violation
- [ ] Suporte a flat e by_agent
- [ ] Paridade nos 3 CLIs

### Qualidade
- [ ] Todos os testes existentes continuam verdes
- [ ] Novos testes para cada uma das 5 mudanças nos 3 CLIs
- [ ] `go test ./...` verde
- [ ] `node npm/tests/validator.test.js` verde (ou equivalente)
- [ ] `python -m unittest discover -s pypi/tests` verde

---

## Fora de Escopo
- `trace_id` / pareamento bidirecional completo (Achado 1 completo — feature maior, backlog)
- R8 (evidência em done) — polêmico, backlog
- `--json` output — backlog
- R2 (`docs/roadmap/` singular) — específico do CMDB
