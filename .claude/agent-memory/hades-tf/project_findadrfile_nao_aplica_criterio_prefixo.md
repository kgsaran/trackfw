---
name: project_findadrfile_nao_aplica_criterio_prefixo
description: findADRFile não aplica critério de prefixo ADR- — fail-closed para blocked_by_draft_adr e adr_accepted_when_req_done
metadata:
  type: project
---

`findADRFile` (validator.go:3137) busca ADR por basename exato, sem filtro de prefixo.

Wave 0 do ROADMAP-2026-10-02 decidiu que ele NÃO deve aplicar o critério ADR-, divergindo do ADR D2.

**Why:** se aplicar o critério e um ADR legado (sem prefixo, com `| Status: Draft` no cabeçalho sem frontmatter) for listado como bloqueador, `findADRFile` retorna `""` → `adrStatusForRule` retorna `("", true)` → `blocked_by_draft_adr` não dispara. D4 não detecta (verifica apenas frontmatter, não cabeçalho). Bypass silencioso de governança — fail-open na direção errada.

**How to apply:** ao auditar regras `blocked_by_draft_adr` e `adr_accepted_when_req_done`, verificar que o `findADRFile` não está filtrado por prefixo. Teste de AC necessário: REQ com `adr: arquivo-sem-prefixo.md` (Draft) deve continuar acionando `blocked_by_draft_adr`. Ajuste pendente: D4 deve usar `resolveAdrStatus` (frontmatter + cabeçalho) não só frontmatter.

[[project_dir_symlink_adr_evil_contado]]
