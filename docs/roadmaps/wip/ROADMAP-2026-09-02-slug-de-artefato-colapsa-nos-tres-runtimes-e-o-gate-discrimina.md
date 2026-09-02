---
status: wip
date: 2026-09-02
req: "docs/req/REQ-2026-09-02-slugify-do-python-deleta-nao-alfanumericos-em-vez-de-colapsar-e-viola-o-contrato-de-slug.md"
---

# ROADMAP: slug de artefato colapsa nos três runtimes, e o gate discrimina

> Date: 2026-09-02 | Status: wip

REQ: docs/req/REQ-2026-09-02-slugify-do-python-deleta-nao-alfanumericos-em-vez-de-colapsar-e-viola-o-contrato-de-slug.md
ADR:

## ML-1A — Alinhar a cópia que derivou e o slug inline do Node

**Status:** ✅ Concluído

- `pypi/trackfw/generators/adr.py::slugify` passa a colapsar `[^a-z0-9]+`, igual aos outros três
  geradores do próprio Python (`note`, `req`, `roadmap`) e ao contrato em `docs/cli-parity.md`.
- `npm/src/generators/init.js::generatePomXml` passa a usar o `toSlug` compartilhado em vez da
  variante inline sem NFKD.
- Não toca em `identity/slugify`, que deleta por especificação declarada no próprio docstring.

## ML-1B — Fazer o gate discriminar

**Status:** ✅ Concluído

`scripts/check-artifact-parity.sh` usava `TITLE="Autenticação e Sessão"`. Medido: os **três**
exemplos da tabela do contrato dão 3/3 iguais **com o defeito presente** — neles deletar e colapsar
coincidem. Trocado por `TITLE="Autenticação C/C++ v1.2"`, que mantém a cobertura de NFKD e
acrescenta não-alfanumérico entre alfanuméricos, onde as duas semânticas divergem.
`SLUG` esperado ajustado para `autenticacao-c-c-v1-2`, medido nos três runtimes.

O comentário do porquê fica no próprio script — quem for mexer no título precisa saber que ele não é
decorativo.

## ML-1C — Falsificação nas duas direções

**Status:** ✅ Concluído

**(a) Direção da correção** — com as duas correções aplicadas:

```
Artifact parity checks passed (8 artifact types × 3 runtimes; roadmap flags, quoted status,
analyzing cycle flat/by_agent; CLAUDE.md ## Architect responses section)
```

**(b) Controle** — revertendo **só** `pypi/trackfw/generators/adr.py` para a versão anterior e
mantendo todo o resto, o gate reprova e nomeia o runtime:

```
created …/python/docs/req/REQ-2026-09-02-autenticacao-c-c-v1-2.md
created …/python/docs/adr/ADR-2026-09-02-autenticacao-cc-v12.md
artifact parity drift: adr (python) — arquivo ausente: docs/adr/ADR-2026-09-02-autenticacao-c-c-v1-2.md
```

O controle é a parte que importa: prova que o gate novo **pega este defeito**, e não apenas que ele
passa depois da correção. Um sinal que nunca acendeu não é um sinal verde.

**(c) Não-regressão dos exemplos documentados** — os 3 exemplos da tabela do contrato produzem o
mesmo slug de antes da correção, nos três runtimes. A correção não renomeia artefato existente.
