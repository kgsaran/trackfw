# Python's buffered sys.stdout reorders against an inherited-stdio subprocess when not a TTY

**Data:** 2026-08-14
**Onde:** `pypi/trackfw/commands/commit.py:run_commit` (case c — branch não-governada)
**Achado por:** `scripts/check-commit-parity.sh` (ML-4A, `ROADMAP-2026-08-14-bloqueio-tecnico-de-comandos-git-brutos-por-subagente-via-deny-hooks-nos-7-runtimes-suportados.md`)

## Sintoma

`trackfw commit` numa branch fora do padrão `feat/fix/refactor` deveria imprimir um aviso
("does not follow feat/fix/refactor — committing without a roadmap check.") e DEPOIS rodar o
`git commit` real, deixando a saída do próprio git aparecer em seguida. Go e Node.js
produziam essa ordem. O Python invertia: a saída do `git commit` (via
`subprocess.run(["git", "commit", "-m", message])`, sem `capture_output`, herdando stdio real)
aparecia ANTES do aviso do trackfw, quando o stdout do processo Python era redirecionado para
um arquivo/pipe (não um TTY) — exatamente o caso de `check-commit-parity.sh`, que captura
`>"$out_file"`.

## Causa raiz

`sys.stdout` do Python é **line-buffered quando é um TTY** e **block-buffered quando não é**
(pipe/arquivo). `out.write(msg + "\n")` enfileirava o aviso no buffer interno do
`io.TextIOWrapper`, sem `flush()`. O `subprocess.run` seguinte herda o **file descriptor**
real de stdout (não o buffer Python) e escreve direto nele, sem passar pelo buffer do
processo pai. Resultado: a escrita do subprocesso chega ao arquivo/pipe antes do `flush()`
implícito do processo Python (que só ocorre na saída normal do interpretador). Invisível
rodando no terminal (TTY é line-buffered, o `\n` já dispara o flush) — só reproduz quando a
saída é redirecionada, que é justamente como todo gate de paridade captura stdout.

## Por que isso não apareceu nos testes unitários

Os testes unitários de `run_commit` injetam `exec_git_commit` como fake — nunca tocam um
`subprocess.run` real com stdio herdado. A divergência só é observável end-to-end, com o
`git commit` real rodando via `execGitCommit`/`_default_git_commit` de produção. Mesma classe
de achado que `[[branch-new-exit-code-leak-vs-propagation-2026-08-04]]`: um bug que só existe
no caminho de produção (wrapper de processo real), invisível em qualquer suíte que injete
fakes — só um gate shell que invoca o binário/CLI de verdade pega isso.

## Fix

`out.flush()` logo após o `out.write()` do aviso, antes de chamar `exec_git_commit`. Go
(`fmt.Fprintln` para `os.Stdout`) e Node (`process.stdout.write`) já são efetivamente
unbuffered/imediatos para este padrão de uso — não precisaram de mudança.

## Generalização — onde procurar de novo

Qualquer lugar em `pypi/trackfw/**` que faça `out.write(...)`/`print(...)` e IMEDIATAMENTE
depois invoque um `subprocess.run(...)` com stdio herdado (sem `capture_output=True`) tem o
mesmo risco. Vale auditar `ship.py`/`branch.py` se algum dia adicionarem uma escrita própria
antes de um `git` real com stdio herdado. Regra prática: toda escrita em `sys.stdout` que
precede um subprocess com stdio herdado precisa de `flush()` explícito — não depender do
buffering automático, que só funciona em TTY.

## Ver também

- `vault/notes/branch-new-exit-code-leak-vs-propagation-2026-08-04.md` — mesma classe de
  achado (bug só visível no wrapper de processo real, não nos fakes injetados).
- `scripts/check-commit-parity.sh` — gate que prova esta divergência e a mantém fechada.
