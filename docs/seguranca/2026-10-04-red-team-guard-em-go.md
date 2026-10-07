---
title: "Red Team — guard em Go (ADR-2026-10-04)"
date: 2026-10-06
author: hades-tf
status: concluido
roadmap: ROADMAP-2026-09-22-os-hooks-de-guard-nao-executam-no-windows-na-maioria-dos-clis-de-agente-e-o-validate-reporta-instalado.md
ml: ML-4A
---

# Red Team — guard em Go

> 2026-10-06 | Hades (Security) | ML-4A do ROADMAP-2026-09-22
>
> Binario de teste: scratchpad/hades/trackfw-test (producao) e
> scratchpad/hades/fix-c1/trackfw-fixed (overlay C1 fix),
> construidos com `go build -overlay` a partir do HEAD de
> `feat/hooks-de-guard-executam-no-windows`. Nenhum binario de `bin/` do
> repositorio foi usado. Saidas coladas verbatim.
>
> Marcadores: medido = comando executado com saida colada; documentado =
> fonte citada por arquivo + linha; [inferido] = derivado de premissa verificavel,
> nao medido ao vivo; verificado = existencia de simbolo confirmada por grep.

---

## Sumario executivo

VEREDITO: BLOQUEIA O PR

Dois achados bloqueantes:

C1 (CRITICO): D7 nao esta satisfeito para subcomandos invalidos do `guard`. O gate
ML-1A-i (`trackfw guard <folha-invalida>` -> exit 2, roadmap L162 e Wave 0 L508) e uma
falsa conclusao: o teste TestGuardCmd_PositionalArg_IsGuardError cobre arg invalido do
subcomando `git-branch`, nao subcomando invalido do pai `guard`. Sem o fix, cmd.exe
passa `guard 'git-branch;'` como argv[1] ao pai -- exit 0 silencioso.

C2 (ALTO): `&` como operador de controle nao e separador no quoteAwareSplit, e
nao foi declarado na ADR-2026-08-12 como residual. O roadmap tem AC aberto (L40):
"Medicao na VM Windows: o guard dispara e bloqueia em PowerShell, cmd.exe e Git Bash".
`echo ok & git push origin main` em cmd.exe produz exit 0. C2 bloqueia este PR porque
viola esse AC (mesma REQ, mesmo PR -- Regra Dura). O "paridade com .sh" nao e defesa
valida: o .sh nunca recebeu strings de comando de cmd.exe ou PS.

Dois achados alto (A1, A2): residuais pre-existentes reconfirmados.

---

## Achados

---

### CRITICO / BLOQUEANTE -- C1

C1 -- D7: `guard <subcomando-invalido>` sai exit 0, nao exit 2

- Arquivo: internal/commands/guard.go -- newGuardCmd() (sem Args nem RunE)
- Raiz tecnica: cobra v1.10.2 command.go:
  - L934: --help retorna flag.ErrHelp (antes de L955)
  - L955: !c.Runnable() retorna flag.ErrHelp
  - ExecuteC L1152: swallows flag.ErrHelp -> retorna cmd, nil
  - ValidateArgs (L968) nunca alcancado (L955 precede-a)
  - Exit 0: Execute() nao ve erro; root.go L135-139 nunca chama os.Exit
- Gate violado: roadmap L162 [x] e Wave 0 L508 sao falsas conclusoes. O teste
  TestGuardCmd_PositionalArg_IsGuardError cobre arg invalido de subcomando,
  nao subcomando invalido do pai.

Impacto em cmd.exe: a linha de hook PS-familia executada por cmd.exe e tokenizada como
quatro argv: `guard`, `git-branch;`, `exit`, `$LASTEXITCODE`. O pai guard recebe
`git-branch;` como argv[0]. Sem o fix, nao ha Args nem RunE no pai -- cobra swallows
flag.ErrHelp -- exit 0. O agente ve allow e o commit/push prossegue. O validate impoe
a linha exata da config (casamento por m.raw == expectedLine), mas versao skew entre
o binario instalado (sem o fix) e uma config correta deixa esta janela aberta.

Residual apos o fix: binario totalmente sem o subcomando guard (versao anterior a
introducao do guard) responde ao comando root desconhecido com exit 1 (nao 2). Este e
o caso coberto por A1. Com o fix, subcomando invalido ao pai -> exit 2.

Medido (binario de producao):

    $ trackfw-test guard nao-existe </dev/null 2>&1; echo exit:$?
    [help do guard impressa]
    exit:0

    $ printf '{"tool_input":{"command":"git push origin main"}}' \
        | trackfw-test guard git-brnch 2>&1; echo exit:$?
    [help do guard impressa]
    exit:0

Fix verificado com overlay ao vivo (scratchpad/hades/fix-c1/trackfw-fixed).
Mudanca em internal/commands/guard.go:

    cmd.Args = func(cmd *cobra.Command, args []string) error {
        if len(args) > 0 {
            return &guardError{fmt.Sprintf(
                "trackfw guard: unknown subcommand %q", args[0])}
        }
        return nil
    }
    cmd.RunE = func(cmd *cobra.Command, _ []string) error {
        return &guardError{
            "trackfw guard: requires a subcommand (git-branch or credential)"}
    }

Medicoes com trackfw-fixed:

    $ trackfw-fixed guard nao-existe </dev/null 2>&1; echo exit:$?
    Error: trackfw guard: unknown subcommand "nao-existe"
    exit:2

    $ trackfw-fixed guard 'git-branch;' </dev/null 2>&1; echo exit:$?
    Error: trackfw guard: unknown subcommand "git-branch;"
    exit:2

    $ trackfw-fixed guard </dev/null 2>&1; echo exit:$?
    Error: trackfw guard: requires a subcommand (git-branch or credential)
    exit:2

    $ trackfw-fixed guard --help </dev/null >/dev/null 2>&1; echo exit:$?
    exit:0   (sonda guardRunProbe: inalterada)

    $ trackfw-fixed vaildate </dev/null 2>/dev/null; echo exit:$?
    exit:1   (nao-guard, inalterado)

`go test -overlay=fix-c1/overlay.json ./internal/commands/` -> ok (sem regressao).

Testes obrigatorios no ML corretivo (subprocess):
1. `trackfw guard unknown-subcmd </dev/null` -> exit 2
2. `trackfw guard </dev/null` -> exit 2
3. `trackfw guard --help </dev/null` -> exit 0

---

### ALTO / BLOQUEANTE -- C2

C2 -- `&` como operador de controle nao e separador -- fail-open em cmd.exe e PS

- Arquivo: internal/guard/gitbranch.go -- quoteAwareSplit (sem case para `&` simples)
- O operador `&` simples nao e separador (apenas `&&` esta no switch). Em cmd.exe,
  `echo ok & git push origin main` sao dois comandos. Em PS, `& git push origin main`
  usa o call operator. Ambos passam como um unico segmento sem subcomando git detectado.
- `git.exe push origin main` (forma idiomatica PS) passa porque .exe nao e stripped.
- Grep confirmado: grep -nF '&' ADR-2026-08-12-nao-ha-prevencao-... -> vazio.
  O .sh L6-8 declara evasoes de tokenizacao bash. `&` como operador de controle e
  familia diferente de `;`, `&&`, `|` -- nao declarada como residual.

Por que bloqueia este PR: roadmap AC L40 (nao marcado):
`- [ ] Medicao na VM Windows: o guard dispara e bloqueia em PowerShell, cmd.exe e Git Bash`.
`echo ok & git push` em cmd.exe produz exit 0 -- AC falha. Regra Dura: mesma REQ, mesmo PR.
O "paridade com .sh" nao e valida: o .sh nunca correu em cmd.exe nem PS.

Medido:

    $ echo '{"command":"echo ok & git push origin main"}' \
        | trackfw-test guard git-branch; echo exit:$?
    exit:0

    $ echo '{"command":"& git push origin main"}' \
        | trackfw-test guard git-branch; echo exit:$?
    exit:0

    $ echo '{"command":"git.exe push origin main"}' \
        | trackfw-test guard git-branch; echo exit:$?
    exit:0

Fix proposto (conteudo, nao execucao -- escopo deste red team):
- quoteAwareSplit: ao encontrar `&` fora de aspas com lookahead 1 char: se proximo e `&`
  -> ja coberto como `&&`; se nao -> emitir separador para `&` simples.
  Excecoes (FP a cobrir nos testes de aceite): `2>&1`, `>&2`, `&>file`, `|&` (nao dividir
  quando `&` e precedido ou seguido por `>` ou `<`); `&` dentro de aspas (ja opaco);
  `^&` em cmd.exe (escape de `&`).
- applyRule: strip leading `&` token (PS call operator).
- applyRule: strip trailing .exe em nome de binario (case-insensitive).
- A divergencia com o fixture .sh e autorizada -- o .sh nunca cobriu Windows nativamente.

---

### ALTO (residuais pre-existentes -- reconfirmados)

A1 -- R5: Git Bash PATH diverge do PATH do validate -- fail-open em 6/8 CLIs

- Arquivo: internal/validator/validator_guard_binary_probe_ml2b.go
- exec.LookPath("trackfw") usa PATH do processo. Git Bash login-shell prefixa ~/bin,
  que pode conter binario antigo (ex.: 8.0.0-rc2) sem guard. Validate passa; hook usa
  o binario antigo, que retorna exit 1.

Documentado: portabilidade ML-3A L515:
"Exit 1 = hook falhou por erro, mas segue = fail-open em 6/8 CLIs."
L578 (cenario 4b): "login, resolve 8.0.0-rc2 de ~/bin; validate nao ve este binario |
Git Bash | 1 | 1 | fail-open 6/8 CLIs".

`grep -r "min_version\|version_too_old" ./internal/validator/` -> vazio (regra de versao
minima ausente).

---

A2 -- R1+R2: PS + `; exit $LASTEXITCODE` + Restricted -> exit 0 (fail-open)

- Arquivo: internal/generators/agentfiles.go
- Sob ExecutionPolicy Restricted com .ps1 (npm), PSSecurityException nao atualiza
  $LASTEXITCODE. O sufixo executa com $LASTEXITCODE = 0 -> exit 0 -> fail-open.

Documentado: portabilidade ML-3A L312:
"RESTRICTED_EXIT: 0 - FALHA ABERTA. $LASTEXITCODE permanece 0 do estado anterior;
`exit $LASTEXITCODE` retorna 0."

---

### BAIXO

B1 -- credWriteAttention imprime em os.Stderr real

pathguard.RejectAndReport usa os.Stderr real, nao o writer injetado. Sem impacto
na decisao de permit/deny. Testes que capturam o writer injetado nao veriam a mensagem
de recusa de symlink. Verificado em internal/guard/credential.go.

B2 -- guardRunProbe executa binario de PATH

exec.LookPath recusa ./trackfw (ErrDot, Go 1.19+). Ambiente pos-comprometido
necessario. Anotado #nosec G204 -- verificado por grep.

B3 -- TestRunCredential_OutsideProjectNoOp e vacuo (Regra de Reconciliacao)

Mut-4 (check cwd removido) -> ok (todos os testes passam). O teste usa payload limpo
(sem JWT/AWS); exit 0 nao discrimina no-op de allow.
Afirmacao do teste que falha a reconciliacao: "credential e inativo fora do projeto".
Com check removido, credential aceita tudo fora do projeto e o teste nao detecta isso.
Fix: adicionar payload JWT fora de projeto, assertar stderr vazio e ausencia de arquivo
de atencao.

Higiene -- .trackfw-credential-guard.json nao rastreado

docs/roadmaps/.trackfw-credential-guard.json esta untracked (git status confirmado).
Gerado por esta sessao de red team ao executar o guard com payloads de teste JWT. Nao e
artefato desta sessao (escopo restrito a este arquivo); nao foi staged. O arquiteto deve
remover ou colocar em .gitignore antes de qualquer `git add`.

---

## 1. Riscos da Wave 0 -- reconfirmacao

(a) npm + PS Restricted
Reconfirmado. guardCheckPS1RestrictedPolicy detecta .ps1 + Restricted. Ver A2.

(b-1) Exit code PS
Reconfirmado. guardGitBranchCmdPSPOSIX = "trackfw guard git-branch; exit $LASTEXITCODE",
guardGitBranchCmdCmdExe = "trackfw guard git-branch". Validate verifica linha exata.

(b-2) Binario sem subcomando guard
Reconfirmado. TestGuardBinaryProbeOnce_BinarioSemGuard_Violation -- sonda
`guard --help -> 0`. Gap no pai (C1) nao detectado pela sonda (--help cobra L934 precede
L955; nao afetado pelo fix).

(c) Sequestro de PATH
Reconfirmado. TestValidateTrackfwBinaryInProjectRoot_ExePresente_Violation e
TestValidateTrackfwBinaryInProjectRoot_CmdPresente_Violation cobrem a regra.

(d) trackfw ausente do PATH
Reconfirmado. TestGuardBinaryProbeOnce_BinarioAusente_Violation. Verificado.

(e) cwd -- walk-up vs. cwd-only
Reconfirmado. Medido:

    $ echo '{"command":"cat output.txt"}' | (cd internal && trackfw-test guard credential)
    exit:0

Ordem stdin-antes-de-check correta (previne EPIPE).

(f) Parser JSON em Go

| Sub-risco | Resultado |
|---|---|
| F1 case-sensitive | DENY -- TestExtractCommand_CaseSensitive (verificado) |
| F2 last-wins | ALLOW -- design; TestExtractCommand_DuplicateKeyLastWins (verificado) |
| F3 NUL | DENY -- TestExtractCommand_NUL, TestRunGitBranch_NULDeny (verificados) |
| F4 BOM | BOM stripped; benign -> exit 0; TestExtractCommand_BOM (verificado) |
| F5 late-EOF / truncado | gate (iii) medido abaixo |
| Dois objetos JSON | DENY em ambas as ordens -- medido |

Gate (iii) -- late-EOF medido:

    $ time ( (printf '{"command":"echo ok"}'; sleep 3) | trackfw-test guard git-branch )
    exit:0  elapsed:3s

    $ time ( (printf '{"command":"echo ok"}'; sleep 6) | trackfw-test guard git-branch )
    exit:2  elapsed:6s

Mecanismo lido do codigo (payload.go L18, L42, L71, L117):
DrainStdin usa loop com timer de 2s por janela (L71: window := time.NewTimer(idleTimeout)).
Janela 1 (t=0-2s): payload chega; janela nao e ociosa; nova janela comeca. Para sleep 3:
EOF chega em t=3s, dentro da janela 2 (t=2-4s) -> not truncated -> allow. Para sleep 6:
janela 2 nao recebe bytes nem EOF (pipe mantido pelo sleep 6) -> timer expira (L117:
return result.Bytes(), true // idle window -> truncated) -> deny. Pipeline total 6s porque
sleep 6 mantem o pipe aberto mesmo apos o guard ter retornado. OK

---

## 2. Alvos de falsificacao -- ao vivo e por mutacao

Nomes verificados por `grep -rE "func <nome>\b" internal/` -- todos 30 nomes: OK.
"Mut: MORDE" = mutacao executada e teste falhou; "PASSA" = nao detectada;
"existe, nao mutado" = verificado mas mutacao nao executada nesta sessao.

### Guard Go -- internal/guard/

| Superficie | Teste | Mut |
|---|---|---|
| Case-insensitive | TestExtractCommand_CaseSensitive | existe, nao mutado |
| NUL sem verificacao | TestExtractCommand_NUL, TestRunGitBranch_NULDeny | Mut-1 MORDE |
| BOM-FP | TestExtractCommand_BOM | existe, nao mutado |
| Duplicate key first-wins | TestExtractCommand_DuplicateKeyLastWins | existe, nao mutado |
| check cwd removido (credential) | nenhum | Mut-4 PASSA (ver B3) |
| guard subcomando-invalido -> exit 0 (C1) | nenhum subprocess | Mut-3 confirma ausencia |
| D7 exit code subprocess 2 vs 1 | nenhum subprocess para guard | Mut-3 PASSA |
| Exact-match vs Contains no validate | TestGuardHookResolvable_SufixoExtra_Violation | Mut-2 MORDE |
| argv_overrides_stdin | TestRunGitBranch_CommandFlag | existe, nao mutado |
| Stdin ocioso sem dados | TestDrainStdin_IdleNoData, TestRunGitBranch_TruncatedDeny | existe, nao mutado |
| Pipe fecha apos janela de 2s | TestDrainStdin_Truncated + medido ao vivo | existe, nao mutado |
| Prioridade de chaves | TestExtractCommand_ToolInputCommand | existe, nao mutado |
| Dois objetos JSON | TestExtractCommand_NonJSON | medido: ambas as ordens negam |
| TRACKFW_GIT_COMMAND env var | TestRunGitBranch_EnvFallback | existe, nao mutado |
| Non-string em command | TestExtractCommand_NonStringAbsent | medido: 4 vetores |
| & como separador (C2) | nenhum | medido: exit 0; nao declarado na ADR |

### Involucro .sh

| Superficie | Teste | Status |
|---|---|---|
| Binario antigo -> exit 1 -> fail-open | TestGuardBinaryProbeOnce_BinarioSemGuard_Violation | existe, nao mutado; residual A1/R5 |
| Script adulterado | TestCredentialGuardScriptIntegrity_ScriptIlegivel_ViolationSemAbortar | existe, nao mutado |

### Emissao por CLI -- internal/generators/

| Superficie | Teste | Status |
|---|---|---|
| Sufixo PSPosix emitido | TestGuardHookResolvable_ExatoPSPosix_Ok, TestGuardHookResolvable_FamiliaCmdExeEmPSPosix_Violation | existe, nao mutado |
| Copilot sem command (D4) | TestInjectCopilotHooks, TestInjectCopilotHooks_StructuralParityAcrossStacks | existe, nao mutado |
| migrateHookCommand legado | TestInjectClaudeHooks_MigratesLegacyRelativeCredentialGuardCommand | existe, nao mutado |
| Windsurf exato PS/POSIX | TestGuardHookResolvable_Windsurf_ExatoPSPosix_Ok | existe, nao mutado |
| Amazon Q exato cmd.exe | TestGuardHookResolvable_AmazonQ_ExatoCmdExe_Ok | existe, nao mutado |

### Validate -- internal/validator/

| Superficie | Teste | Status |
|---|---|---|
| collectCommandsWithMarker exact-match | TestGuardHookResolvable_SufixoExtra_Violation | Mut-2 MORDE |
| Sonda guard --help -> 0 | TestGuardBinaryProbeOnce_BinarioComGuard_Ok | existe, nao mutado |
| PS1 + Restricted detectado | TestGuardBinaryProbeOnce_WindowsPS1Restricted_Violation | existe, nao mutado |
| Versao minima ausente | nenhum | gap em A1 |

---

### Falsificacoes ao vivo

F1 -- Case-insensitive:

    $ echo '{"tool_input":{"command":"git push origin main","Command":"echo ok"}}' \
        | trackfw-test guard git-branch; echo exit:$?
    exit:2    DENY correto.

F2 -- Duplicate key last-wins:

    $ echo '{"command":"git push origin main","command":"echo ok"}' \
        | trackfw-test guard git-branch; echo exit:$?
    exit:0    ALLOW -- design declarado.

F3 -- NUL:

    $ printf '{"command":"git push\x00origin main"}' \
        | trackfw-test guard git-branch; echo exit:$?
    exit:2    DENY correto.

Idle stdin:

    $ sleep 3 | trackfw-test guard git-branch; echo exit:$?
    exit:2    (nega em ~2s)

D7 GAP (C1):

    $ trackfw-test guard nao-existe </dev/null 2>&1; echo exit:$?
    [help]
    exit:0    FAIL-OPEN. Bloqueante (C1).

TRACKFW_GIT_COMMAND:

    $ printf '{}' | TRACKFW_GIT_COMMAND="git push origin main" \
        trackfw-test guard git-branch; echo exit:$?
    exit:2    DENY correto.

Dois objetos JSON:

    $ printf '{"command":"echo ok"}{"command":"git push origin main"}' \
        | trackfw-test guard git-branch; echo exit:$?
    exit:2    DENY -- Go fail-closed para JSON invalido.

---

### Mutacoes

Mut-1 -- checkNUL killed (`if false && strings.ContainsRune`):

    --- FAIL: TestExtractCommand_NUL
    FAIL    github.com/kgsaran/trackfw/internal/guard

Gate morde. OK

Mut-2 -- exact-match -> Contains:

    --- FAIL: TestGuardHookResolvable_SufixoExtra_Violation
    FAIL    github.com/kgsaran/trackfw/internal/validator

Gate morde. OK

Mut-3 -- exitCode 2 -> 1 em root.go L135:

    ok    github.com/kgsaran/trackfw/internal/commands

Nenhum teste subprocess verifica exit code de guard. Mutacao nao detectada. Gap que o
ML corretivo de C1 fecha.

Mut-4 -- check cwd removido em credential.go:

    ok    github.com/kgsaran/trackfw/internal/guard

TestRunCredential_OutsideProjectNoOp passa com check removido (payload limpo; exit 0
nao discrimina no-op de allow). Ver B3.

---

## 3. Questoes de arquitetura (itens 3a-3f)

### 3a -- Non-string em command: divergencia Go vs .sh

Jq `a // b` retorna `b` so se `a` e false ou null. Para numero/array/objeto, jq retorna
a representacao string (com -r) e nao faz fallthrough para a proxima chave. Go trata
qualquer nao-string como ausente e cai para a proxima prioridade na cadeia.

Medido -- quatro vetores:

| Payload | Go | .sh com jq | .sh sem jq (awk) |
|---|---|---|---|
| {"tool_input":{"command":["git","push","origin","main"]}} | ALLOW | ALLOW | ALLOW |
| {"tool_input":{"command":{"x":"git push..."}}} | ALLOW | ALLOW | ALLOW |
| "git push origin main" (nao-objeto no topo) | ALLOW | ALLOW | ALLOW |
| {"tool_input":{"command":42},"command":"git push origin main"} | DENY (exit 2) | ALLOW | ALLOW |

Para numero 42: Go cai do nao-string para `command` de topo -> "git push" -> DENY.
.sh com jq: CMD_RAW="42" (jq -r converte int para string; sem fallthrough) -> nao
detecta git -> ALLOW. .sh sem jq (awk): awk extractor procura valores delimitados por
aspas; `42` sem aspas nao e extraido -> CMD_RAW="" -> `[ -z "$CMD_RAW" ] || exit 0`
-> ALLOW. Go e mais estrito neste vetor. A direcao encontrada e a oposta a "Go libera
onde .sh bloquearia".

### 3b -- quoteAwareSplit: separadores especiais

| Caractere | .sh | Go | Status |
|---|---|---|---|
| \n fora de aspas | sep | sep | paridade |
| && | sep | sep | paridade |
| || | sep | sep | paridade |
| ; | sep | sep | paridade |
| pipe | sep | sep | paridade |
| \r via --command arg | ALLOW | ALLOW | paridade; [inferido] PS/cmd pode tratar CR como terminador -- nao medido na VM |
| \r nao-escapado em JSON | invalido | invalido -> DENY | Go fail-closed |
| & simples | opaco -- ALLOW | opaco -- ALLOW | paridade; nao declarado na ADR-2026-08-12 (C2, bloqueia) |
| echo ok & git push | ALLOW | ALLOW | paridade; residual C2 |
| & git push (PS call op) | ALLOW | ALLOW | paridade; residual C2 |
| git.exe push | nao medido no .sh | ALLOW | medido Go; residual C2 |
| backtick | ALLOW | ALLOW | paridade; residual declarado |
| $(git push) | ALLOW | ALLOW | paridade; residual declarado |

Residuais IFS/braces declarados no .sh L6-8. `&` e operador de controle, familia diferente
-- nao coberto (C2).

### 3c -- credWriteAttention: impacto em CLI?

Sem impacto na decisao de permit/deny. Ver B1.

### 3d -- Git Bash PATH -- fail-OPEN em 6/8 CLIs (nao fail-closed)

O binario antigo retorna exit 1. Exit 1 = fail-open em Claude Code, Codex, Gemini,
Cursor, Windsurf e Amazon Q. Kiro e Copilot bloqueiam exit != 0 (deny-all FP ate
atualizacao). Fail-open, nao fail-closed. Fonte: portabilidade ML-3A L515 e L578.

### 3e -- Sonda do validate executa binario de PATH

exec.LookPath recusa ./trackfw (ErrDot, Go 1.19+). Ambiente pos-comprometido necessario.
Ver B2.

### 3f -- `[ -f trackfw.yaml ] || exit 0`

Refere-se ao involucro .sh. O .sh le stdin (L5) ANTES de checar projeto (L8) -- previne
EPIPE. Go replica a ordem. Guard de projeto e no-op fora do projeto (D9). Guard global
cobre subdiretorios.

---

## 4. Reconciliacao com os criterios de aceite

| AC | Status | Evidencia |
|---|---|---|
| Riscos (a)-(f) reconfirmados | OK | Secao 1 |
| Tabela Secao 3 com teste que morde ou "nenhum" | OK | ~24 linhas; "nenhum" documentados; todos 30 nomes verificados |
| >=3 falsificacoes ao vivo | OK | F1, F2, F3, idle-stdin, C1 gap, TRACKFW_GIT_COMMAND, dois-objetos, gate(iii) -- 8 medidas |
| >=3 mutacoes com resultado documentado | OK | Mut-1 morde, Mut-2 morde, Mut-3 passa, Mut-4 vacuo |
| 3a: Go libera onde .sh bloquearia? | OK | 4 vetores; so direcao inversa; .sh sem jq (awk) medido |
| 3b: separadores Go vs .sh | OK | 13 vetores; ADR-2026-08-12 citado; & nao declarado -> C2 bloqueia |
| 3c: credWriteAttention | OK | Sem impacto na decisao |
| 3d: fail-OPEN em 6/8 CLIs | OK | ML-3A L515/L578 |
| 3e: sonda e risco do binario | OK | BAIXO; ErrDot; pos-comprometimento |
| 3f: ordem stdin-antes-de-check | OK | Fiel ao .sh; cwd-only (D9) |
| Veredito com achados por severidade | OK | C1 CRITICO (bloqueia); C2 ALTO (bloqueia); A1/A2 ALTO; B1/B2/B3 BAIXO |

---

Documento produzido por hades-tf (Hades, Security Reviewer) em 2026-10-06.
Binario de teste: construido a partir de HEAD de feat/hooks-de-guard-executam-no-windows.
Nenhum codigo de produto foi alterado. Nenhum artefato de governanca foi alterado.
`git status --porcelain` confirmado: zero arquivos staged; zero arquivos de hades
fora de docs/seguranca/2026-10-04-red-team-guard-em-go.md.
Entradas omitidas por restricao do handoff ("SOMENTE este arquivo"):
  - docs/agents-working-context.md
  - vault notes pendentes (cobra ErrHelp C1; & C2; false-green closure ML-1A gate (i))
  - .claude/agent-memory/hades-tf/
