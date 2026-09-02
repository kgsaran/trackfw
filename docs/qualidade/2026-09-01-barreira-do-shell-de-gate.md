# Barreira final de qualidade — item 7 do #216 (PR #236)

> Autor: hefesto-tf (Code Quality) | Data: 2026-09-01
> Escopo: `git diff origin/main...HEAD` da branch
> `fix/os-3-clis-executam-gate-de-wave-com-sh-c` — ML-1A (`sh -c` nos 3 CLIs, `not_evaluated`
> para `sh` ausente) + ML-2A (gate `check-shell-posix-portability.sh` e contrato em
> `docs/cli-parity.md`). ML-0A (modelo de ameaça) é de `hades-tf`, não auditado aqui em duplicidade.

## Veredito

**APROVA.**

Os quatro itens substantivos do pedido foram provados por execução direta e independente dos três
binários. `make quality` terminou **depois** da primeira versão deste relatório (a ressalva de
"não observado até o fim" ficou registrada por alguns minutos e foi resolvida no item 7, adendo
abaixo): `MAKE_EXIT=0`, log completo sem nenhum `FAIL`/`not ok`/`Error:`/`panic:`. Nenhum achado
bloqueante. Um achado de acompanhamento (não bloqueante), descrito no item 3 — já com REQ própria
aberta por outra sessão no momento em que fechei esta auditoria
(`docs/req/REQ-2026-09-01-gate-de-shell-detecta-reversao-na-grafia-literal-mas-nao-a-semantica-endurecer-para-checagem-comportamental.md`),
confirmando que o achado era real e acionável.

---

## 1. O controle sobreviveu? (prioridade máxima do pedido)

**Sim — falsificado de forma independente, não reaproveitando as fixtures do roadmap.**

Construí um roadmap-fixture próprio (`Gates da wave: true` / `false`) e rodei `barrier --wave 1
--json --trust-local-gates` nos três binários, em `$PATH` normal:

```
GO:     "gates": {"status": "blocked", "evidence": ["true: exit 0"], "failures": ["false: exit 1"]}
NODE:   "gates": {"status": "blocked", "evidence": ["true: exit 0"], "failures": ["false: exit 1"]}
PYTHON: "gates": {"status": "blocked", "evidence": ["true: exit 0"], "failures": ["false: exit 1"]}
```

Mesmo veredito, mesma evidência, `exit code 1` do processo nos três. **A uniformização não
convergiu para "tudo passa"** — o risco central do item era exatamente esse, e não se
materializou. Nenhum caminho ficou mais permissivo do que era antes: no Go, o comportamento é
idêntico ao pré-existente (`exec.Command("sh","-c",cmd)` já era assim); em Node e Python, a
mudança troca *quem* interpreta o comando (`shell: true`/`shell=True` → `sh -c` explícito), não
*como* um exit code diferente de zero é tratado — o mapeamento `exitCode != 0 → blocked`
permanece byte-a-byte o mesmo nos dois.

## 2. `not_evaluated` × `blocked` — falsificado nas duas direções, nos 3 CLIs

**Caso 1 — `exit 127` (ferramenta interna ausente dentro do `sh`, `sh` presente):**
troquei o primeiro gate para `nosuchtool-xyz` (mantendo `$PATH` normal, com `sh`). Resultado nos
três: `status: "blocked"`, `failures: ["nosuchtool-xyz: exit 127", "false: exit 1"]`. Nunca
`not_evaluated`.

**Caso 2 — `sh` ausente do `$PATH`:** curei um `$PATH` contendo só um symlink para `git` (nenhum
`sh`), e invoquei os três binários com esse `$PATH` isolado (não o `$PATH` do meu shell — os
binários `node`/`python3` foram invocados por caminho absoluto, com `PATH=` setado só para o
subprocesso do `barrier`). Resultado nos três, byte-idêntico:

```
"status": "not_evaluated", "evidence": [],
"failures": ["gates not evaluated: sh not found in PATH — install a POSIX shell (e.g. Git Bash, WSL) to evaluate gates"]
```

**Os dois estados não colapsam em nenhum dos três CLIs.** A AC4 está sólida — confirmado por
execução real dos binários, não por leitura de código.

Isso corrobora, de forma independente, os testes já commitados:
- Go: `TestRunGateCommand_ExitCodes` (inclui `nosuchtool-xyz` → 127, `spawnFailed=false`) +
  `TestRunGateCommand_ShMissing_SpawnFailed` + `TestEvalGateCommands_ShMissing_NotEvaluated`
  (`internal/commands/barrier_test.go`).
- Node: `evalGates: a missing tool inside sh is a normal exit 127, not not_evaluated` +
  `evalGates: sh missing from $PATH reports not_evaluated with the pinned message`
  (`npm/tests/barrier.test.js`).
- Python: `test_gates_ferramenta_ausente_dentro_do_sh_e_exit_127_normal_nao_not_evaluated` +
  `test_gates_sh_ausente_do_path_reporta_not_evaluated_com_mensagem_pinada`
  (`pypi/tests/test_barrier.py`).

Todos os três pares de teste isolam exatamente a mesma dicotomia, com a mesma técnica
(`$PATH` curado real, não mock) — nenhum é vácuo.

## 3. `scripts/check-shell-posix-portability.sh` — a exclusão de comentário e o `assert_count`

**A exclusão de linhas de comentário não abre buraco de bypass.** `assert_no_code_match` usa
`grep -vE '^\s*//'` (Node) / `grep -vE '^\s*#'` (Python) antes de procurar o padrão de código. Em
JS, uma linha que começa (após espaços) com `//` é **inteiramente** comentário de linha — não há
forma de essa linha também conter código JS executável no mesmo commit. O mesmo vale para `#` em
Python. Logo, qualquer `shell: true`/`shell=True` real (reintroduzido em código executável) **não
pode** começar a linha com `//`/`#`, e portanto **não pode** escapar da checagem por essa via — a
exclusão é de comentário genuíno, não uma brecha reaproveitável para disfarçar código. Falsifiquei
isso ao vivo: revertendo só `barrier.js` para `spawnSync(command, {shell:true,...})` (mantendo os
comentários de prosa do ML-1A que citam `shell: true` intactos), o gate reprova nomeando
`npm/src/commands/barrier.js`, sem tocar Python; o simétrico para `barrier.py` reprova só
Python. Rodei essas duas árvores (`node-regress`, `py-regress`) já deixadas em scratchpad pelo
ML-2A e confirmei os vereditos batem com o relatado no roadmap — reprodução independente, não
apenas leitura do relato.

**Achado de acompanhamento, não bloqueante:** a checagem negativa mira a sintaxe **literal**
`shell\s*:\s*true` / `shell\s*=\s*True` (o padrão de objeto-literal / parâmetro nomeado). Uma
reintrodução por **atribuição** — `opts.shell = true` seguido de `spawnSync(cmd, opts)`, ou um
objeto de opções montado por spread/computed-property — não bateria com esse regex e passaria pelo
gate sem ser pega. Isto não é uma regressão desta REQ nem um erro de implementação: é a mesma
classe de limitação inerente a todo gate baseado em `grep` de assinatura textual já presente no
repositório (`check-homedir-parity.sh`, `check-ref-separator-portability.sh`) — nenhum deles
protege contra reformulação sintática equivalente, só contra a forma literal específica. Não
bloqueia esta PR; registrar como residual conhecido do padrão de gate, não corrigir aqui.

**`assert_count` nas 10 assinaturas — a lição do gate do separador foi aplicada corretamente.**
Confirmei via `grep -c` que cada assinatura central ocorre exatamente 1x por arquivo hoje, e que
`not_evaluated` ocorre exatamente 2x por arquivo (ramo de trust do roadmap + ramo de `sh`
ausente) — `assert_count(2)`, não `assert_has`, é o número certo: um `assert_has` aprovaria
mesmo se um dos dois ramos colapsasse silenciosamente para `blocked` num commit futuro, desde que
o outro ramo ainda mencionasse `not_evaluated` uma vez. `assert_count(2)` fecha exatamente esse
buraco. Vacuidade (`ROOT` vazio/inexistente) também falsificada: as 10 checagens reprovam
individualmente, cada uma nomeando o arquivo ausente — nunca "0 encontrado, gate passa" silencioso.

## 4. Extração de `evalGateCommands` (Go) — semântica preservada

Comparação linha a linha do diff em `internal/commands/barrier.go`: os dois ramos de trust
(`trustLocalGates` explícito e `roadmapTrustForGates` fail-open) chamavam, antes da extração, o
mesmo laço `for _, gcmd := range gateCommands { runGateCommand(gcmd); ... }` duplicado
verbatim — a extração para `evalGateCommands` move esse laço para uma função só, chamada
identicamente nos dois pontos de chamada, sem alterar a ordem de execução, o critério
`exitCode == 0`, ou o mapeamento para `passed`/`blocked`. A única mudança de comportamento é a
nova saída antecipada quando `spawnFailed` — que **não existia antes** (o código antigo não
distinguia spawn-failure de exit-failure) — e essa saída antecipada é idêntica nos dois ramos que
chamam `evalGateCommands`. O ramo fail-open do `roadmapTrustForGates` (REQ aberta) não foi tocado
em sua lógica de decisão de confiança — só reusa a mesma função de avaliação de gates que o ramo
`--trust-local-gates` já usava. Nenhuma regressão de comportamento nesse ramo.

## 5. Cobertura de teste

Nenhum teste novo é vácuo — todos os 6 pares (Go/Node/Python × exit-127/sh-ausente) exercitam o
binário real com um `$PATH` genuinamente curado (não mock de `spawnSync`/`subprocess.run`), e
verificam `status`, `evidence` (vazio no caso `not_evaluated`) e `failures` (mensagem pinada
exata). O teste de controle pré-existente (`false: exit 1` → `blocked`) não foi alterado, o que é
correto — comportamento que já estava certo não precisa de teste novo.

**Gap não coberto por teste automatizado (não bloqueante, padrão do repo):** o próprio script
`check-shell-posix-portability.sh` não tem um harness de teste (`.bats` ou similar) que exercite
as árvores `good`/`node-regress`/`py-regress`/`empty-root` de forma repetível no CI — a
falsificação relatada no roadmap (e a que refiz de forma independente nesta auditoria) foi manual,
usando fixtures em `/tmp`/scratchpad, não commitadas como teste. Isto é consistente com **todos**
os outros `scripts/check-*.sh` do repositório (nenhum deles tem teste automatizado dedicado) —
não é uma lacuna introduzida por esta PR, é a convenção já existente do projeto. Não bloqueio por
isso, mas registro como observação de arquitetura de testes que vale um ML futuro transversal
(fora do escopo desta REQ).

## 4a. O ramo não-trusted (`roadmapTrustForGates` = false) não vaza a mensagem de `sh` ausente

Todas as falsificações do item 2 usaram `--trust-local-gates`, que ignora esse ramo. Verificado à
parte, sem fixture artificial: o próprio roadmap desta REQ, no repositório real, diverge de
`origin/main` (contém as seções do ML-2A que ainda não chegaram à main — `git show
origin/main:docs/roadmaps/wip/ROADMAP-2026-09-01-....md` traz só o conteúdo do ML-1A). Rodando
`barrier --wave 1 --json` **sem** `--trust-local-gates` nos três CLIs, no diretório real:

```
"status": "not_evaluated", "commands": [], "evidence": [],
"failures": ["gates not evaluated: roadmap content differs from origin/main — pass --trust-local-gates to evaluate local gates"]
```

Byte-idêntico nos três, e é a mensagem **do ramo de trust**, não a `shMissingMsg` — os dois ramos
que agora compartilham o status `not_evaluated` continuam produzindo `failures[]` distintas e
corretas. Isso fecha a preocupação do item 4: o ramo fail-open do `roadmapTrustForGates` não foi
contaminado pela extração de `evalGateCommands`.

## 6. `docs/cli-parity.md`

A nova seção documenta o achado do ML-0A — a mudança **não é no-op em POSIX** (interpretador fixo
`/bin/sh` → resolvido por `$PATH`) — e a distinção `not_evaluated`/`exit 127`. Texto consistente
com o código auditado; nenhuma divergência entre o que o contrato promete e o que os três CLIs
fazem, confirmada pela execução real acima.

## 7. `make quality`

`make quality` foi iniciado às 20:16 (sem pipe, `MAKE_EXIT` capturado em variável) e, no momento
de fechar a primeira versão deste relatório, ainda não tinha concluído — declarei "não observado"
em vez de presumir verde, conforme pedido. A notificação de conclusão chegou logo em seguida:

```
MAKE_EXIT=0
```

Log completo (3475 linhas), **zero ocorrência** de `FAIL`/`not ok`/`Error:`/`panic:`, terminando em
`check-shell-posix-portability: OK — 10 assinaturas de execucao de gate via sh -c confirmadas em
barrier.js e barrier.py`.

**Ressalva de co-tenência mantida como observação, não como bloqueio:** durante a janela de
execução, `git status` mostrou atividade de outra(s) sessão(ões) na mesma árvore de trabalho —
`docs/seguranca/2026-09-01-barreira-do-shell-de-gate.md` (hades-tf, paralelo já avisado no pedido),
e, ao término, também `docs/cli-parity.md`, `.gitignore`, `vault/notes/index.md` modificados e
`.agents/skills/*` removidos, além de uma segunda execução de `make quality` (PID 28762, outra
sessão) concorrente com a minha. Apesar da mutação concorrente, o `MAKE_EXIT=0` observado é a
evidência real — não presumida — e o gate mais relevante para este PR
(`scripts/check-shell-posix-portability.sh`) foi, adicionalmente, verificado de forma **direta e
independente** do `make quality` (item 3 acima), o que remove a dependência de uma janela
perfeitamente estática para sustentar o veredito desta auditoria.

---

## Resumo

| # | Item | Resultado |
|---|---|---|
| 1 | Controle não regrediu para "tudo passa" | Confirmado por execução independente nos 3 CLIs |
| 2 | `not_evaluated` × `blocked` (127) | Falsificado nas duas direções, nos 3 CLIs, byte-idêntico |
| 3 | Gate `check-shell-posix-portability.sh` | Exclusão de comentário é segura; `assert_count` correto; 1 achado de acompanhamento (sintaxe por atribuição não coberta — limite genérico de gates grep, não regressão) |
| 4 | `evalGateCommands` (Go) | Semântica preservada nos dois ramos de trust; ramo não-trusted não vaza `shMissingMsg` |
| 5 | Cobertura de teste | 6 pares não vácuos; gap não bloqueante: script gate sem harness automatizado (convenção pré-existente do repo) |
| 6 | `docs/cli-parity.md` | Consistente com o código auditado |
| 7 | `make quality` | `MAKE_EXIT=0`, log completo sem `FAIL`/`not ok`/`Error:`/`panic:` (observado após conclusão, não presumido) |

**Veredito: APROVA.** **Bloqueantes: nenhum.**
