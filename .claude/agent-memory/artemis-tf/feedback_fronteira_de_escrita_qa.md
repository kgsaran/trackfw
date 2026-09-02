---
name: fronteira-de-escrita-qa-sem-git
description: QA neste repo não executa git, não altera status de ML no roadmap e não toca internal//npm/src//pypi/trackfw/ — entrega não-commitada para auditoria do arquiteto
metadata:
  type: feedback
---

Nos microlotes despachados pelo `trackfw_architect` neste repo: **nenhuma operação de git**, entrega
**não-commitada**, e o **status do ML no roadmap não é alterado pelo executor** — a transição vem
depois da auditoria do orquestrador.

**Why:** o arquiteto é a única autoridade de git (cria branch, audita o diff, commita). Além disso o
`check-roadmap-barrier-contract.sh` pina um `PINNED_CORPUS_HASH` calculado sobre as **linhas de
veredito dos roadmaps** — editar o status de um ML pode quebrar esse gate por um motivo que não tem
relação com o trabalho entregue.

**How to apply:** ao terminar, `git status --short` e conferir que o conjunto alterado é exatamente o
da lista "Files affected" do ML (mais nota de vault / `index.md` / `agents-working-context.md`).
Arquivos sob `internal/`, `npm/src/`, `pypi/trackfw/` fora dessa lista significam vazamento de
escopo. Adição justificada além da lista (ex.: corrigir em `docs/cli-parity.md` uma afirmação que o
próprio microlote falsificou) é permitida, mas tem de ser **nomeada explicitamente no handoff**.
Ver [[falsificacao-nas-duas-direcoes]].
