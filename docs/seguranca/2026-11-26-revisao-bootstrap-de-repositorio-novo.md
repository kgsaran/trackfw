# Revisão de Qualidade — Bootstrap de Repositório Novo

> Autor: hefesto-tf | Data: 2026-11-26 | ML: ML-2A (Wave 2, REQ-2026-11-26)

## Veredito

**APROVA COM RESSALVAS** — 1 achado médio (teste de absence-lease não discrimina da rejeição por
non-fast-forward), 4 achados baixos. Nenhum bloqueante. Nenhuma regressão detectada. Build rc=0;
todos os testes do escopo passam.

```
go build ./...                                                      rc=0
go test ./internal/commands/ ./internal/guard/ ./internal/generators/ -count=1    rc=0
```

---

## Escopo revisado

Diff `git diff HEAD~1 -- internal/ docs/cli-parity.md` (commit
`d6350cae fix(init,commit,push,guard): bootstrap governado de repositório novo`).
Especificação: ADR-2026-11-26 §"Ajustes do Wave 0" (prevalece sobre o texto original).
Decisões D1–D6 auditadas: D1 (`init`), D2 (`commit`, `branch new`), D3 (push da branch padrão),
D4 (feature barrada em remoto vazio), D5 (mensagens e templates), D6 (guard).

---

## Achados

### A1 — MÉDIA: Teste de absence-lease não discrimina da rejeição por non-fast-forward

**Arquivo:** `internal/commands/push_bootstrap_test.go:330`
**Função:** `TestD3_RealGit_AbsenceLease_BlocksPushWhenMainExists`

O teste simula a janela TOCTOU: o stub de `ls-remote` retorna vazio (o check local "viu" remoto
vazio), mas o push real vai contra um bare que já tem `main` com SHA diferente do local. O teste
verifica `err != nil` e que o SHA remoto não mudou.

**Problema:** o push falharia igualmente com um plain push (sem o lease), pois o commit local não
é descendente do commit remoto — git rejeitaria com "non-fast-forward". O teste não diferencia:

- `--force-with-lease=refs/heads/main:` → push rejeitado pelo servidor (lease: ref existe)
- plain push → push rejeitado pelo servidor (non-fast-forward)
- `--force` → push aceito, SHA remoto mudaria → teste falharia

O comentário está correto ("The arm fails if absenceLease is changed to `--force`"), mas o único
cenário TOCTOU onde o absence-lease protege e um plain push não protegeria é quando o commit
remoto É ancestral do commit local (fast-forward seria aceito, mas o lease bloqueia porque a ref
existe). Esse vetor não está testado.

**Consequência:** se o absence-lease for removido acidentalmente (plain push), este teste continua
passando. A proteção TOCTOU de fast-forward permaneceria sem cobertura de teste.

**Sugestão:** adicionar um cenário onde o commit local É descendente do remoto (fast-forward),
confirmando que o absence-lease bloqueia mesmo quando o plain push passaria. Ou, como alternativa
mínima, adicionar asserção de que `err.Error()` contém texto que git emite especificamente para
lease rejection (ex: "rejected ... stale info" ou similar), distinguindo da mensagem
non-fast-forward.

---

### A2 — BAIXA: `_ = generators.PrintBootstrapSteps` em init_test.go é decorativo

**Arquivo:** `internal/commands/init_test.go:1118`
**Função:** `TestRunGitInitForBootstrap_FreshDir_PrintsBootstrapSteps`

A linha `_ = generators.PrintBootstrapSteps` tem um comentário que a descreve como "unused import
guard: confirm PrintBootstrapSteps is the actual function called (not a local copy)". Essa
descrição é incorreta: `_ = <func value>` é uma verificação de compile-time de que o símbolo existe,
não de que ele é chamado. O caminho de chamada (`runGitInitForBootstrap` → `PrintBootstrapSteps`) é
verificado indiretamente pela captura de `initBootstrapOut` e pelas asserções de conteúdo nas linhas
anteriores — que são os testes reais.

A linha decorativa não bloqueia nada e não prejudica a cobertura. Mas o comentário cria uma
expectativa falsa de que ela "prova" algo sobre o call path.

**Sugestão:** remover a linha ou corrigir o comentário para "compile-time symbol existence check
(the actual call path is verified by the initBootstrapOut assertions above)".

---

### A3 — BAIXA: Instrução D4 de 7 linhas duplicada verbatim entre push.go e ship.go

**Arquivos:** `internal/commands/push.go:305–321` e `internal/commands/ship.go:367–382`

As 7 linhas do bloco de orientação D4 são idênticas, exceto na primeira linha ("trackfw push" vs
"trackfw ship") e na última ("come back and push" vs "come back and ship"). As 5 linhas do meio
(passos 1–6 do bootstrap) são cópia literal. Se um passo de bootstrap mudar (ex: `trackfw push`
mudar para `trackfw publish`), precisa ser corrigido em dois lugares.

**Sugestão:** extrair as linhas comuns para uma constante ou função helper interna
`bootstrapStepsHint()`. Não é bloqueante — a duplicação está correta hoje.

---

### A5 — BAIXA: Comentário de ordenação incorreto em `init.go` (duas ocorrências)

**Arquivo:** `internal/commands/init.go:534` e `internal/commands/init.go:820`

Ambas as ocorrências dizem "git init is called inside executeScaffoldAndInstall, after the AI-tool
destination pre-check and **before the scaffold write**". A ordem real em `executeScaffoldAndInstall`
é: (1) `preCheckAIToolDestinations`, (2) rc=0/false guard, (3) `generators.Scaffold`, (4)
`installAITools`, (5) `runGitInitForBootstrap` — ou seja, git init roda **depois** de Scaffold e
installAITools, não antes.

A motivação de design está correta (pré-check garante diretório limpo se scaffold falhar), mas a
descrição do comentário inverte a sequência. O comentário correto seria: "git init is called after
scaffold and AI-tool install so that a scaffold pre-check failure leaves no `.git/` behind".

**Consequência:** nenhuma — o comportamento é correto. O comentário pode confundir um mantenedor
que precise rastrear a causa de um estado parcial (`.git/` presente sem scaffold).

**Sugestão:** corrigir as duas ocorrências.

---

### A4 — BAIXA: Comentário incorreto em `parseLSRemoteHeads`

**Arquivo:** `internal/commands/push.go:405–407`

O comentário diz "Other refs (tags, HEAD, etc.) from --heads are unexpected but not treated as
unrecognized". Com `--heads`, `git ls-remote` só retorna `refs/heads/*` — tags e HEAD nunca
aparecem nessa saída. O comentário descreve um caso impossível.

**Consequência:** nenhuma — o comportamento é correto. O comentário pode induzir o mantenedor a
acreditar que a função precisaria de lógica adicional para "outros refs".

**Sugestão:** corrigir para: "Other refs under refs/heads/ that do not indicate a branch (none
expected from --heads, which limits output to refs/heads/*) are skipped without setting
unrecognized."

---

## Conformidade com D1–D6

| Decisão | Arquivo principal | Verificação |
|---|---|---|
| D1 — `init` roda `git init` (4 estados) | `init.go:142–203` | Implementação correta; fallback para git antigo; rc=0/false aborta antes de qualquer escrita |
| D2 — `commit` aceita só 1º commit na branch padrão; `branch new` recusa HEAD não-nascida | `commit.go:355–366`, `branch.go:212–231` | `isBootstrapFirstCommit` usa conjunção correta (rev-parse --verify -q HEAD + rev-list --all --count); `branch new` usa só condição 1 conforme ADR |
| D3 — push bootstrap fail-closed | `push.go:142–201` | Consulta todas as pushurls, refusa se qualquer URL tem branch ou falha, 1 commit, absence-lease |
| D4 — feature barrada em remoto vazio fail-open | `push.go:305–321`, `ship.go:367–382` | Fail-open confirmado (d4Err == nil && d4Empty); helper `checkRemoteEmpty` compartilhado |
| D5 — mensagens e templates | `barrier.go`, `validator_credential_guard_integrity.go`, `agentfiles.go`, `nextsteps.go`, `architect.md`, `cli-parity.md` | Nenhum emissor com `git add .` ou `-A`; instrução genérica usa "git add <each path listed>" |
| D6 — guard | `gitbranch.go` | `gh repo create --push`, `git switch --orphan`, `git worktree add --orphan`, `git symbolic-ref` write/delete — todos bloqueados com contra-braço |

---

## Verificações adicionais

**Regressões em testes existentes:** `git diff HEAD~1` para os arquivos de teste dos comandos
afetados (`push_test.go`, `ship_test.go`, `commit_test.go`, `branch_test.go`) não mostrou nenhuma
linha removida de asserção ou de teste existente. As proteções de main/master existentes foram
mantidas e o D3 foi acrescentado acima delas.

**D2 bypass via `--from HEAD`:** `branch.go:140` define `branchNewDeps.gitExec = defaultGitExec`
incondicionalmente no `RunE`, portanto o check de HEAD não-nascido em `runBranchNew` dispara em
todos os caminhos de produção — sem bypass.

**rc=0/false duplicado em `init.go`:** O erro "refusing to run inside a git directory" aparece em
duas linhas (`init.go:164` em `runGitInitForBootstrap` e `init.go:258` em
`executeScaffoldAndInstall`). Não é código morto: `executeScaffoldAndInstall` faz o guard cedo
(antes de qualquer escrita de scaffold); `runGitInitForBootstrap` repete o estado em seu switch. Os
dois caminhos são exercidos por testes distintos.

**POSIX pass para `gh`:** `matchSubcommandPOSIX` em `gitbranch.go:450` delega para `MatchSubcommand`
após strip de wrappers de shell; a ramificação `base == "gh"` em `MatchSubcommand:422` está nesse
caminho — `sh -c 'gh repo create x --push'` é bloqueado pelo segundo passo POSIX. Não há teste
explícito para esse caso; os testes `TestMatchSubcommand_GhRepoCreate_Push_Blocked` e
`TestMatchSubcommand_D6_Supplemental` cobrem o direct call.

**ML-1F cobertura de caminho real:** `TestRunInit_RealGit_ClaudeScope_PrintsActualPaths`
(`init_test.go:1162`) chama `executeScaffoldAndInstall` com `--ai-tools claude --scope project` e
verifica que a saída de bootstrap contém `.claude` e `CLAUDE.md` com caminhos reais do filesystem.

---

## Observações de portabilidade

- `t.Chdir` em `push_bootstrap_test.go`: Go 1.25.2, compatível.
- `git init -b main` nos testes: consistente com testes já existentes no pacote; nenhum Windows-skip
  novo necessário (os testes existentes usam a mesma construção).
- `LC_ALL=C` em `defaultInitGitCheck`: correto para garantir mensagem "not a git repository" em
  inglês em todas as plataformas.
- Ausência-lease `--force-with-lease=refs/heads/<branch>:` validada com git real em
  `TestD3_RealGit_AbsenceLease_BlocksPushWhenMainExists`.

---

## Testes confirmados por grep

Todos os testes citados abaixo existem e passam (confirmado por grep + `go test -count=1`):

```
TestCheckRemoteEmpty_RealGit_FreshBareRepo           push_bootstrap_test.go:55
TestCheckRemoteEmpty_RealGit_BareRepoWithBranch      push_bootstrap_test.go:74
TestD3_RealGit_FreshRemote_OneCommit_D3Pushes        push_bootstrap_test.go:98
TestD3_RealGit_TwoCommits_Blocked                    push_bootstrap_test.go:126
TestD3_RealGit_RemoteAlreadyHasMain_Blocked          push_bootstrap_test.go:154
TestD4_RealGit_FeatureBranch_EmptyRemote_Blocked     push_bootstrap_test.go:176
TestD4_RealGit_FeatureBranch_RemoteHasMain_Passes    push_bootstrap_test.go:201
TestD3_RealGit_PushURL_ExistingMain_Blocked          push_bootstrap_test.go:259
TestD3_RealGit_TwoPushURLs_SecondHasMain_Blocked     push_bootstrap_test.go:292
TestD3_RealGit_AbsenceLease_BlocksPushWhenMainExists push_bootstrap_test.go:330
TestD4_RealGit_RemoteHasDevelopOnly_Passes           push_bootstrap_test.go:389
TestMatchSubcommand_SwitchOrphan_Blocked             gitbranch_test.go:134
TestMatchSubcommand_SwitchOrphan_CounterArm          gitbranch_test.go:149
TestMatchSubcommand_WorktreeAddOrphanBlocked         gitbranch_test.go:268
TestMatchSubcommand_SymbolicRefWrite_Blocked         gitbranch_test.go:404
TestMatchSubcommand_SymbolicRefDelete_Blocked        gitbranch_test.go:421
TestMatchSubcommand_SymbolicRefRead_Allowed          gitbranch_test.go:437
TestMatchSubcommand_GhRepoCreate_Push_Blocked        gitbranch_test.go:455
TestMatchSubcommand_GhRepoCreate_NoPush_Allowed      gitbranch_test.go:472
TestMatchSubcommand_D6_Supplemental                  gitbranch_test.go:503
TestPush_D3_Main_ForceWithLease_Refused              push_test.go:446
TestPush_D3_Main_RemoteUnreachable_FailClosed        push_test.go:468
TestShip_D4_FeatureBranch_EmptyRemote_Blocked        ship_test.go:2085
TestShip_D4_FeatureBranch_EmptyRemote_CounterArm     ship_test.go:2108
TestShip_D4_FeatureBranch_EmptyRemote_FailOpen       ship_test.go:2131
TestIsBootstrapFirstCommit_RealGit_SwitchOrphan      commit_unborn_branch_test.go:136
TestAgentProtocol_NoGitAddDotOrAddAll                agentfiles_test.go:1698
TestRunInit_RealGit_ClaudeScope_PrintsActualPaths    init_test.go:1162
```
