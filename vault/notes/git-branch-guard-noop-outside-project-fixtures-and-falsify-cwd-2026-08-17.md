# git-branch-guard: no-op fora de projeto trackfw — fixtures que quebram e o cwd ambiente do falsify gate

> 2026-08-17 · ML-1A, ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-integridade-independente-de-fiacao.md

## O que mudou

`scripts/trackfw-git-branch-guard.sh` (e as 7 cópias derivadas do gerador: constantes Go/Node/Python
em generators + validator + o arquivo checked-in) agora sobem diretórios a partir do cwd físico
(`pwd -P`) até achar `trackfw.yaml`. Sem `trackfw.yaml` em nenhum ancestral, o script sai com `0`
antes de qualquer parsing de comando — decisão em
`docs/adr/ADR-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-trackfw.md`, pré-requisito
para cabear o guard em escopo global (Wave 2 do mesmo roadmap) sem bloquear `git commit`/`git push`
em toda a máquina.

## Armadilha 1 — todo fixture de teste existente parou de bloquear

Antes desta ML, o guard bloqueava incondicionalmente — os testes de "block" (Go
`setupGitBranchGuardFixture`, Node `setupFixture`, Python `setUp` de 3 classes distintas) criavam o
script num `t.TempDir()`/`tempfile.mkdtemp()` puro, sem `trackfw.yaml`. Depois desta ML, todo esse
fixture vira **no-op silencioso** — os testes de bloqueio passariam a falhar (esperando exit 2,
recebendo exit 0) a menos que o fixture escreva `trackfw.yaml` na raiz E o subprocesso rode com esse
diretório como cwd.

Fix aplicado nos 3 stacks: fixture de "bloqueio" escreve `trackfw.yaml`; um fixture PAR sem
`trackfw.yaml` foi criado para os testes de no-op (`setupGitBranchGuardFixtureWithoutTrackfwYAML` /
`setupFixtureWithoutTrackfwYAML` / `TestGitBranchGuardNoOpOutsideProject`).

**Achado extra em Python**: `pypi/tests/test_git_branch_guard.py` nunca passava `cwd=` para
`subprocess.run` — os testes rodavam com o cwd AMBIENTE do processo pytest, não com o `tmpdir` do
fixture. Isso não quebrava antes (guard incondicional), mas ficaria silenciosamente dependente de
onde `pytest` é invocado depois desta ML. Corrigido adicionando `cwd=self.tmpdir` explícito em toda
chamada — nunca confiar em cwd ambiente para comportamento de guard.

## Armadilha 2 — `check-gates-falsify.sh` Cenários 60-63 dependiam do cwd ambiente

`assert_guard_exit()` (helper do gate) sempre invocou `bash "$script"` sem `cd` explícito — herdando
o cwd de quem chama `check-gates-falsify.sh`. Antes desta ML isso não importava. Depois dela, os ~30
call-sites dos Cenários 60-63 (que testam bloqueio, esperam exit 2) ficariam corretos **hoje** (porque
`make quality` roda da raiz do repo, que TEM `trackfw.yaml`) mas silenciosamente quebrados se o gate
for chamado de outro diretório — e pior: os braços de DETECÇÃO (que também esperam exit 2 na versão
corrompida) continuariam "passando" mesmo se o matcher real quebrasse, porque o no-op mascararia
tanto o baseline quanto a detecção da mesma forma.

Fix: em vez de mudar a assinatura de `assert_guard_exit` (usada só pelos Cenários 60-64, nenhum call
site fora desse bloco), o próprio script faz `cd` para um diretório de fixture com `trackfw.yaml`
logo antes do Cenário 60 e restaura o cwd original logo depois do Cenário 63 — todo `assert_guard_exit`
do bloco herda o cwd correto sem qualquer mudança nos ~30 call sites.

## Armadilha 3 (evitada, registrada para não repetir)

`git rev-parse --show-toplevel` foi cogitado como alternativa à caminhada por `test -f`, mas:
medido, custa ~16ms/chamada (fork+exec) contra ~0.77ms/chamada da caminhada por builtins — a
diferença importa porque o guard roda em toda chamada de ferramenta do agente. Além do custo, `git
rev-parse` sai 128 fora de um repositório git (precisa tratamento extra) e resolve a raiz do
repositório GIT, não a raiz do projeto trackfw — resposta errada dentro de um submódulo ou
repositório aninhado onde `trackfw.yaml` não coincide com o topo do git.

## Onde olhar

- `internal/generators/scaffold.go` (`gitBranchGuardScript`, bloco "--- 0. No-op...")
- `internal/generators/git_branch_guard_test.go` (`setupGitBranchGuardFixture*`)
- `npm/tests/git_branch_guard.test.js` (`setupFixture*`)
- `pypi/tests/test_git_branch_guard.py` (`TestGitBranchGuardNoOpOutsideProject` + `cwd=` nas 3 classes)
- `scripts/check-gates-falsify.sh` (Cenário 64 + bloco de `cd` em torno dos Cenários 60-63)

## Ver também

- [[git-branch-guard-branch-create-heuristic-e-env-var-assignment-stripping-2026-08-17]]
- [[armadilhas-ao-escrever-cenario-em-check-gates-falsify-2026-08-12]]
