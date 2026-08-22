---
name: push-e-pr-via-ship
description: git push/commit brutos são bloqueados por hook; use trackfw ship --no-pr -m direto (trackfw commit deixa o commit preso sem push sancionado)
metadata:
  type: project
---

Neste repositório, `git commit` e `git push` brutos são **bloqueados por hook**
(`git-branch-guard`). Os caminhos sancionados são `trackfw commit -m` e `trackfw ship`.

**Detalhes que custam tempo se descobertos na hora:**
- **Nunca use `trackfw commit` numa branch feat/fix/refactor cujo commit precisará ser empurrado.**
  Ele commita, mas deixa você **sem saída sancionada**: `git push` bruto é bloqueado pelo guard e o
  `ship` recusa com "nothing is staged" (o caminho sem `-m` exige `--force-with-lease`, que por sua
  vez exige PR aberto). Medido em 2026-08-22 — custou um `git reset --soft HEAD~1` para refazer pelo
  `ship`. O caminho certo é ir **direto** para `trackfw ship --no-pr -m "..."` com os arquivos staged.
- `trackfw ship` **exige algo staged** — se todo o trabalho já foi commitado, ele falha com
  "nothing is staged" antes de qualquer push. Não existe modo "só empurrar".
- O binário do `PATH` costuma estar **desatualizado** em relação ao repo, e **`--version` NÃO
  distingue o build**: medido em 2026-08-17, o instalado e o recém-compilado diziam `7.0.0` os dois,
  enquanto o instalado emitia aviso **falso** de divergência do script do guard. Sempre
  `make build` e usar `./bin/trackfw` antes de concluir qualquer auditoria.
- **Não rode `make install` neste ambiente:** o CLI vem do **Homebrew**
  (`/opt/homebrew/Cellar/trackfw/`), e o alvo `install` do Makefile grava em `/usr/local/bin` —
  criaria uma segunda cópia sombreada, com PATH decidindo qual vence. Para alinhar de verdade,
  atualize pelo Homebrew depois que houver tag nova.
- Scripts de guard **globais** (`~/.trackfw/scripts/*.sh`) são reescritos por
  `trackfw update harness`, mas **só fora de `--dry-run`** — o dry-run reporta `updated=0` e não
  menciona essa escrita. Rodar o harness com `./bin/trackfw` é o que atualiza o guard global.
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
