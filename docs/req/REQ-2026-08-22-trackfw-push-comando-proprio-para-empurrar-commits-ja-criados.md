---
status: done
date: 2026-08-22
author: "Zeus (Arquiteto)"
adr: "docs/adr/ADR-2026-08-22-comandos-de-entrega-separados-push-proprio-e-ship-como-composicao.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-22-trackfw-push-comando-proprio-para-empurrar-commits-ja-criados.md"
---

# REQ: `trackfw push` — comando próprio para empurrar commits já criados

> Date: 2026-08-22 | Status: done
| Linear Issue:
| Jira Issue:

## Motivation

Reportado por KG em uso real (2026-08-22), no encerramento da REQ do hook relativo.
**Commit já criado não tem saída sancionada:**

```
trackfw commit -m "..."   → commit criado                    ok
git push                  → bloqueado: "Use `trackfw ship`"
trackfw ship --no-pr      → "nothing is staged"
```

O `trackfw commit` se anuncia como *"the missing intermediate step between raw `git commit` and
`trackfw ship`"* — mas quem o usa fica preso. A saída que restou foi `git reset --soft HEAD~1` para
refazer pelo `ship`: **desfazer trabalho correto para caber na ferramenta**. Com o commit já
publicado, a mesma manobra desincronizaria a branch.

Agravante de ensino: a mensagem do `git-branch-guard` para `git push` bruto cita `trackfw ship` — o
comando que **não resolve** esse caso. O guard é a superfície que ensina no momento do erro, e hoje
ensina o caminho que recria o beco.

Decisão de desenho em `ADR-2026-08-22-comandos-de-entrega-separados-push-proprio-e-ship-como-composicao.md`.

## Acceptance Criteria

- [ ] **AC1** — `trackfw push` existe nos **3 CLIs** (Go, Node.js, Python) e empurra commits já
      criados sem exigir nada staged e sem `-m`.
- [ ] **AC2** — Sem upstream configurado, o push usa `-u` (`push -u origin <branch>`); com upstream,
      usa `push origin <branch>`. Reuso de `buildPushArgs`/`_build_push_args`, não reimplementação.
- [ ] **AC3** — Gates herdados, com a **mesma** semântica do `ship`: bloqueio incondicional em
      `main`/`master`; padrão `feat|fix|refactor|chore|docs/<slug>`; governança (roadmap em `wip/`
      **ou** `done/`) obrigatória para `feat`/`fix`/`refactor` e dispensada para `chore`/`docs`.
- [ ] **AC4** — `push` **nunca commita** e **nunca abre PR/MR**: não aceita `-m` e não faz nenhuma
      chamada de **escrita** ao forge. A checagem de PR aberto exigida pelo `--force-with-lease`
      (AC5) é **leitura** e é permitida.
- [ ] **AC5** — `--force-with-lease` disponível, com o **mesmo** gate do `ship` (exige PR/MR aberto
      na branch, verificado pelo CLI de forge resolvido).
- [ ] **AC6** — `--dry-run` imprime o que faria sem executar comando de escrita, nos 3 CLIs.
- [ ] **AC7** — Saídas **byte-idênticas** entre os 3 runtimes, provadas por gate de paridade novo
      (`scripts/check-push-parity.sh`), cobrindo caminho feliz, bloqueio em `main`, bloqueio de
      governança, isenção `chore`/`docs` e a ausência de upstream.
- [ ] **AC8** — REASON do ramo `push` do `git-branch-guard` cita `trackfw push`, sincronizada nos
      **5 arquivos** que a duplicam, com os 4 gates de paridade de hooks verdes. Efeito esperado e
      **declarado**: a cópia global `~/.trackfw/scripts/trackfw-git-branch-guard.sh` passa a divergir
      do template até que o usuário rode `trackfw update harness` — regra
      `git_branch_guard_script_integrity`, severidade **warning**, logo `validate` e `make quality`
      seguem exit 0 com **+1 warning** de baseline.
- [ ] **AC12** — O help do `trackfw commit` deixa de se descrever como *"the missing intermediate
      step between raw `git commit` and `trackfw ship`"* (hoje falso) e passa ao vocabulário
      composicional do ADR, **nos 3 CLIs**.
- [ ] **AC9** — Falsificação em **duas direções**: (a) suprimir um gate de `push` é detectado; (b)
      `push` que abra PR ou que commite é detectado.
- [ ] **AC10** — `docs/cli-parity.md` com seção `trackfw push`, anotação `<!-- trackfw-contract -->`
      nomeando o gate, `push` na tabela de comandos e em `scripts/check-cli-parity.sh`;
      `check-parity-contract-coverage.sh` exit 0.
- [ ] **AC11** — `ship` **não regride**: contrato de 7 passos intacto, `-m` ainda obrigatório no
      caminho normal, `pushOnly` ainda atrelado a `--force-with-lease`. `check-ship-parity.sh` e
      `check-ship-force-parity.sh` verdes.

## Negative scope — o que esta REQ NÃO faz

- **Não** relaxa nenhuma flag ou gate do `ship`.
- **Não** adiciona `--push` ao `trackfw commit`.
- **Não** cria escape hatch, allowlist ou variável de bypass no guard.
- **Não** muda a decisão de quais tipos de branch são gateados (`feat`/`fix`/`refactor` sim;
  `chore`/`docs` não) — herda a que existe.
- **Não** abre PR, nem sob flag.
- **Não** mexe em `release tag`, cuja fronteira com `ship` já está documentada.

## Linked ADR
ADR: `docs/adr/ADR-2026-08-22-comandos-de-entrega-separados-push-proprio-e-ship-como-composicao.md`

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/done/ROADMAP-2026-08-22-trackfw-push-comando-proprio-para-empurrar-commits-ja-criados.md`
