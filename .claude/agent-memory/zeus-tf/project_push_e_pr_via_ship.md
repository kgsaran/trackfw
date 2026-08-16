---
name: push-e-pr-via-ship
description: git push/commit brutos são bloqueados por hook neste repo; o caminho é trackfw commit e trackfw ship, e ship exige algo staged
metadata:
  type: project
---

Neste repositório, `git commit` e `git push` brutos são **bloqueados por hook**
(`git-branch-guard`). Os caminhos sancionados são `trackfw commit -m` e `trackfw ship`.

**Detalhes que custam tempo se descobertos na hora:**
- `trackfw ship` **exige algo staged** — se todo o trabalho já foi commitado, ele falha com
  "nothing is staged" antes de qualquer push. Não existe modo "só empurrar".
- O binário instalado em `/usr/local/bin/trackfw` costuma estar **desatualizado** em relação ao
  repo (ex.: sem o subcomando `commit`). Use `./bin/trackfw` após `make build`, ou rode
  `make install` para alinhar.
- O gate de governança do `ship` e do `branch new` aceita roadmap em `wip/` **ou** `done/` — mas
  `analyzing/` **não** satisfaz, então nenhuma branch nasce enquanto o roadmap está em análise.

**Why:** o hook existe para garantir que todo commit/push passe pelo trilho de governança, e a
autoridade de Git é exclusiva do `trackfw_architect`. Descobrir o bloqueio no meio da abertura de
um PR interrompe o fluxo e tenta a saída errada (forçar, ou criar commit vazio).

**How to apply:** para abrir PR ao final de um trabalho já commitado, registre a abertura no
`docs/agents-working-context.md` (artefato legítimo daquele momento), faça `git add` desse arquivo
e rode `trackfw ship -m "..." --no-pr` para commitar e empurrar; depois abra o PR com
`gh pr create --body-file`, que permite corpo consolidado — o corpo gerado pelo `ship` é mínimo e
não comporta um trabalho de várias waves.

Relacionado: [[gate-hades-artefatos-terceiro]].
