---
status: wip
date: 2026-11-15
req: "docs/req/REQ-2026-11-15-cobertura-e-compatibilidade-completas-do-trackfw-no-windows.md"
squad: "hades-tf, artemis-tf"
---

# Roadmap: Cobertura e compatibilidade completas do trackfw no Windows

> Created: 2026-11-15 | Status: wip

## Context
REQ: docs/req/REQ-2026-11-15-cobertura-e-compatibilidade-completas-do-trackfw-no-windows.md

O KG pediu cobertura e compatibilidade completas no Windows, porque nem todo usuário roda o trackfw no macOS.
A medição da `main` (run 43423202733, `windows-full-suites`) deu 71 testes únicos pulados e 5 known-failures.
Pelo menos duas justificativas de skip são falsas: os 41 `TestMLScope*` e os `TestBarrierReentry_*`. Os
artefatos do consumidor (`.sh`) e os hooks de Cursor e Kiro nunca foram medidos no Windows, e npm/PyPI não têm
smoke no Windows. A REQ traz o diagnóstico completo e a definição mensurável de "completo".

Varredura de issues abertas (2026-11-15): nenhuma trata deste mecanismo. Também não há REQ aberta sobre Windows,
porque a REQ-2026-11-13 foi encerrada neste mesmo PR.

Este PR também carrega o fechamento pós-merge da REQ-2026-11-13 (D18, PR #687): o roadmap vai para `done` e a
REQ para `Done`.

## Acceptance Criteria
- [ ] AC1 — inventário completo com classificação (a)/(b)/(c) por item (Wave 0)
- [ ] AC2 — zero skip de Windows com justificativa falsa
- [ ] AC3 — gate de CI contra skip de Windows sem classificação
- [ ] AC4 — artefatos e hooks por CLI executam no Windows sem Git Bash, ou são declarados pelo validate/doctor
- [ ] AC5 — smoke npm e PyPI no `windows-latest`
- [ ] AC6 — guard via perfil e sem `-NonInteractive` no PS 5.1 e no `pwsh` 7 do Windows
- [ ] AC7 — `windows-full-suites` e `make quality` verdes

## Status Legend
⬜ Pending · 🔄 In progress · ✅ Done · ❌ Blocked

## Wave 0 — Threat model e inventário (2 MLs em paralelo; arquivos disjuntos)
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model
**Status:** ✅ Done
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-11-15-wave0-windows-threat-model.md` (novo)
**Actions:**
1. Completude da enumeração: a lista de superfícies da REQ está completa? Procure no repositório outros
   emissores do mesmo artefato (`runtime.GOOS == "windows"`, `t.Skip`, geradores de `.sh`, hooks por CLI,
   workflows de release) antes de declarar a lista fechada.
2. Threat model: quem consegue esvaziar esta Wave 0 sem violar nenhuma regra escrita, e como? Por exemplo,
   reclassificando skip como (c) sem prova, ou fazendo o gate de skip aceitar comentário em vez de evidência.
3. Alvos de falsificação nas duas direções, para cada superfície.
4. Residual declarado.
**Acceptance criteria:**
- [x] as quatro seções respondidas com evidência, não com uma linha de afirmação
- [x] nenhuma linha de implementação

### ML-0B — Inventário de Windows
**Status:** ✅ Done
**Squad:** artemis-tf
**Files affected:** `docs/portabilidade/2026-11-15-inventario-windows.md` (novo)
**Actions:**
1. A partir do log do `windows-full-suites` da `main` atual (`go test -v`), listar cada skip único e cada
   known-failure, com a justificativa citada, a classificação (a)/(b)/(c) proposta e a evidência.
2. Para cada artefato gerado para o consumidor e cada hook por CLI (Claude, Codex, Gemini, Copilot, Cursor, Kiro,
   Windsurf, Amazon Q), dizer se roda no Windows sem Git Bash, com evidência (código e, se possível, CI ou VM).
3. Canais de distribuição: o que já tem smoke no Windows e o que falta (npm, PyPI, `install.ps1`, arm64).
4. Propor os MLs de implementação, agrupados por causa (Regra Dura de Causa Raiz).
**Acceptance criteria:**
- [x] cada item com fonte (linha do log, arquivo:linha ou medição); contagem conferida contra o log (após ML-0B2)
- [x] nenhuma linha de implementação

**Wave gates:**
```bash
test -s docs/seguranca/2026-11-15-wave0-windows-threat-model.md
test -s docs/portabilidade/2026-11-15-inventario-windows.md
```

**Auditoria da Wave 0 (zeus, 2026-11-15):**
- ML-0A aprovado. Conferi três afirmações:
  - o `t.Log` + `return` do braço de perfil (guard_line_shell_test.go:2315 e ~2976);
  - o `HomeFor` só trata Claude (agentconfig.go:128), o que é coerente com o D2 da ADR-2026-10-14: os CLIs
    inferidos não mudam até serem medidos;
  - o runner `windows-11-arm` é GA para repositórios públicos, e o repositório é PUBLIC.
- ML-0B: a 1ª entrega reprovou nas contagens. Dizia 74 skips únicos e 43 `TestMLScope`. Medi 71 e 41, no log do
  run 43766925360 e no código-fonte. Também afirmava que não havia runner arm64. O corretivo ML-0B2 refez os
  totais a partir de uma lista nome→classe: (b) falso 49, (b) 3, (a) 3, (c) 16, total 71. Corrigiu o arm64 e
  reconciliou cada REQ-IMP-1..9 do threat model com um ML.
- Prova de que `sh` está no PATH do `windows-latest`: `TestBarrierFragment_TransportSingleLineReadContra` (que
  chama `sh -c`) passa no log.

## Wave 1 — Skips com justificativa falsa no pacote commands (2 MLs em paralelo; arquivos disjuntos)
> Dependencies: Wave 0 auditada

### ML-1A — `TestMLScope*` no Windows (41)
**Status:** ✅ Done
**Squad:** artemis-tf
**Files affected:** `internal/commands/barrier_ml_scope_test.go`
**Actions:** remover o skip "windows path handling tested separately" das 41 ocorrências (`grep -c 't\.Skip("ml_scope'`
= 41). Trocar caminhos POSIX literais por `t.TempDir()`/`filepath.Join` onde houver. Um teste que reprove no
Windows por defeito de produto vira ML desta REQ com a causa medida; nunca volta a ser skip.
**Acceptance criteria:**
- [x] `grep -c 'windows path handling tested separately'` = 0
- [x] os 41 com `--- PASS` no `windows-full-suites` do PR (REQ-IMP-2), citando a linha do log — PR #688, job 133285325804, linhas 1537–1701: 56 `TestMLScope*` únicos com PASS, 0 SKIP, 0 FAIL; os nomes cujo skip saiu no diff estão todos no conjunto PASS (`comm -23` vazio)

### ML-1B — `TestBarrierReentry_*` no Windows
**Status:** ✅ Done
**Squad:** artemis-tf
**Files affected:** `internal/commands/barrier_reentry_test.go`
**Actions:** remover os skips falsos de T1, T2 e T3. Converter o skip incondicional de T7 em probe de capacidade.
T7e fica como (c), com a prova escrita (EvalSymlinks normaliza o case no NTFS).
**Acceptance criteria:**
- [x] T1/T2/T3/T7 com `--- PASS` no `windows-full-suites`, ou known-failure com o log de CI citado (REQ-IMP-2) — job 133285325804, linhas 1703, 1705, 1707, 1717–1719; T7e segue SKIP com prova (c)
- [x] nenhuma entrada nova em `windows-known-failures.json` sem execução no Windows (diff vazio contra origin/main)

## Wave 2 — Skips falsos de shell e o braço de perfil que se esconde (2 MLs em paralelo; arquivos disjuntos)
> Dependencies: Wave 1 (`windows-full-suites` verde)

### ML-2A — Skips "shells POSIX não disponíveis" e o perfil do PowerShell no Windows
**Status:** ✅ Done
**Squad:** artemis-tf
**Files affected:** `internal/generators/guard_line_shell_test.go`, `internal/generators/fail_closed_d11_behavior_test.go`,
`internal/commands/netexec_test.go`, `internal/commands/doctor_test.go`
**Actions:**
1. Trocar o skip por plataforma por um probe de capacidade (`exec.LookPath` do shell). `sh` e `bash` rodam; `zsh`
   e `dash` pulam pelo probe.
2. Trocar os dois `t.Log` + `return` dos braços de perfil por braços que rodam no Windows. O perfil real fica em
   `$env:USERPROFILE\Documents\{WindowsPowerShell,PowerShell}`, isolado com `USERPROFILE` apontando para
   `t.TempDir()`. Medir antes se o PowerShell respeita essa troca. Se não respeitar, usar `t.Skip` com a prova (c).
3. Cobrir PS 5.1 (`powershell.exe`) e `pwsh` 7, com a asserção `bytes recebidos == len(payload)` nos braços de
   deny e allow (REQ-IMP-4).
4. `TestD11FailClosed_PowerShell`: trocar o stub `#!/bin/sh` por `.cmd`.
**Acceptance criteria:**
- [x] nenhum `t.Log(...); return` que encerre o teste por plataforma
- [x] braço de perfil do `PSInheritedState` e do `LContraArm_ConfirmLow` com `--- PASS` no Windows, em 5.1 e 7 — job 133540617504 do PR #688: 76 PASS e 0 SKIP nos braços `profile-windows`/`via-profile-windows`, em `pwsh` e `powershell`; o USERPROFILE isolado é respeitado
- [x] mutação L→K reprova o braço de perfil no Windows (CI do PR) — contra-braço `form-K/via-profile-windows` com PASS (K entrega menos bytes) em low e medium, nos dois PowerShell

### ML-2B — Forma cmd.exe executada sob `cmd.exe` real (REQ-IMP-5/6)
**Status:** ✅ Done
**Squad:** artemis-tf
**Files affected:** `internal/generators/guard_cmdexe_exec_test.go` (novo)
**Actions:** executar a linha de hook que Kiro e Amazon Q recebem no Windows sob `cmd.exe /d /c`, com um fake
`trackfw.cmd` que grava os bytes recebidos. Cobrir deny 2, allow 0 e ausente; afirmar os bytes recebidos. Pular
fora do Windows.
**Acceptance criteria:**
- [x] `--- PASS` no `windows-full-suites`; mutação da linha (sem `|| exit 2`) reprova — job 133540617504 do PR #688: 16 PASS em `TestGuardCmdExe_*`, incluindo `ContraArm` nas 3 formas

### ML-2C — Corretivo: `TestD18ContraArm_ShAsDashSim` no Windows engole erro de execução
**Status:** ✅ Done
**Squad:** artemis-tf
**Files affected:** `internal/generators/guard_line_shell_test.go`
**Diagnóstico (PR #688, job 133335668133):** o ML-2A removeu o skip por GOOS; o `windows-latest` tem `dash`, então o
teste rodou. Os 4 casos `sh-as-dash-u` saíram com exit 0, sem `stdin.out`, e os 4 `sh-as-dash-e` passaram pelo mesmo
exit 0. O teste converte um erro que não seja `*exec.ExitError` em `code = 0`. Hipótese: o symlink `sh` sem `.exe` não
executa no Windows, e o erro some. É a mesma família do ML-2B (erro que não é ExitError precisa reprovar), só que no
arquivo do ML-2A.
**Actions:** erro que não seja ExitError → `t.Fatalf` em todo `cmd.Run()` dos 4 arquivos do ML-2A; no Windows, o link
se chama `sh.exe`; medir no CI o que o dash do Git for Windows faz com `-u`.
**Acceptance criteria:**
- [x] nenhum `cmd.Run()` nos arquivos do ML-2A converte erro que não seja ExitError em exit 0 (7 sítios corrigidos)
- [x] `TestD18ContraArm_ShAsDashSim` com `--- PASS` no `windows-full-suites`, ou (c) com o rc medido no log — job 133540617504 do PR #688: PASS nos 8 subtestes; dash -u sai 2 com `LASTEXITCODE: parameter not set`

## Wave 3 — Gate, smoke de canal, arm64 e aviso de `sh` (MLs em paralelo; arquivos disjuntos)
> Dependencies: Wave 2
> Ordem (teto de 2 agentes): leva 1 = ML-3A ∥ ML-3B; leva 2 = ML-3C (mesmo arquivo do 3B) ∥ ML-3D. ML-3D é código de
> produto: revisão do `hefesto-tf` antes do commit.

### ML-3A — Registro e gate de skips de Windows (AC3, REQ-IMP-1)
**Status:** ✅ Done
**Squad:** apolo-tf
**Files affected:** `.github/windows-skips.json` (novo), `internal/winskips/winskips_test.go` (novo; decidido no
handoff: teste Go roda em `go test ./...` sem tocar `Makefile` nem `quality.yml`, o que deixa o ML-3A disjunto do ML-3B)
**Actions:**
- O checker é AST, não grep. Ele marca toda função de teste com ramo por plataforma (`runtime.GOOS ==
  "windows"`, `testGOOS`, helpers e build tag `!windows`) que pode sair sem asserção, seja por `t.Skip` ou por
  `return`.
- Cada função marcada exige uma entrada `{test, classification, reason, evidence}` no registro.
- O registro é separado de `windows-known-failures.json`.
**Acceptance criteria:**
- [x] o gate passa na árvore com os skips restantes registrados — `TestGate_RealTree` PASS no macOS e no Windows (job 133673248259)
- [x] braços de falsificação: um skip novo não registrado reprova; um `t.Log` + `return` novo reprova; uma
  entrada sem `evidence` reprova

### ML-3A2 — Corretivo do gate: polaridade, guards mortos e skip de chmod 444
**Status:** 🔄 In progress
**Squad:** apolo-tf
**Files affected:** `internal/winskips/winskips.go`, `internal/winskips/winskips_test.go`, `.github/windows-skips.json`,
`internal/generators/scaffold_test.go`, `internal/discover/discover_test.go`, `internal/commands/json_exit_test.go`
**Diagnóstico (auditoria do ML-3A):** o gate marcava também testes que só rodam no Windows (`!= "windows"` + Skip,
`//go:build windows`): 32 das 57 entradas. Três guards `os.Getenv("GOOS") == "windows"` nunca disparam (GOOS não é
variável de runtime) e estavam registrados como (a). O skip "chmod 444 não bloqueia escrita no Windows" é
provavelmente falso (0444 em arquivo liga o atributo read-only).
**Actions:** polaridade por lógica ternária sobre `runtime.GOOS`; build tag/sufixo que exclui windows marca, tag
windows não marca; remover os 3 guards mortos e o skip de chmod 444; braços 6-8.
**Acceptance criteria:**
- [x] `TestGate_RealTree` e os 8 braços passam localmente; registro de 57 → 22 entradas, todas (c)
- [x] `TestJSONExit_UpdateJSON_TargetFailedSilentStderr` e os 3 testes de hardlink com `--- PASS` no `windows-full-suites` — job 133673248259; o skip de chmod 444 era falso
- [ ] nenhuma `evidence` com `PENDING` no registro antes da Wave 5 (resta `TestReadRegularFile_Socket_NaoTravaEAcusaTipoErrado`:
  o arquivo é `!windows` por causa do FIFO; o braço de socket não foi medido no Windows)

### ML-3B — Smoke de npm e PyPI no `windows-latest` (AC5, REQ-IMP-3)
**Status:** ✅ Done
**Squad:** ares-tf
**Files affected:** `.github/workflows/quality.yml`
**Actions:** job que instala o pacote do PR pelos dois canais: tarball local do npm e wheel local do PyPI, nunca
o registro público. No PowerShell e no cmd, roda:
- `trackfw --version`;
- `trackfw validate` num fixture mínimo;
- `echo 'git branch some-branch' | trackfw guard git-branch` → 2;
- o payload de Write para `write-scope` → 0.
**Acceptance criteria:**
- [x] job verde em 2 rodadas; braço de falsificação documentado (stdin vazio inverte os dois rc) — jobs 133591778685 e 133673248446: 4 células × 6 asserções PASS nas duas; contra-braço com stdin só CRLF

### ML-3C — arm64 executado (REQ-IMP-7)
**Status:** ✅ Done
**Squad:** ares-tf
**Files affected:** `.github/workflows/quality.yml` (job separado do ML-3B; os dois MLs são sequenciais no mesmo
arquivo: o 3C entra depois do 3B)
**Actions:** job `windows-arm64` em `windows-11-arm`: build nativo, `--version` e os dois braços de stdin.
**Acceptance criteria:**
- [x] job verde; incluído nos required checks declarados (`make check-required-checks`) — job 133682058388: GOHOSTARCH=arm64, PE 0xAA64, 6 asserções PASS; `--scope dw` verde. Proteção da `main` no GitHub: KG adiciona `windows-package-smoke` e `windows-arm64` depois do merge

### ML-3D — Aviso proativo de `sh` ausente (REQ-IMP-9) e artefatos de conveniência declarados (REQ-IMP-5)
**Status:** ✅ Done
**Squad:** apolo-tf
**Files affected:** `internal/validator/` e `internal/commands/doctor*.go` (sítios exatos no handoff)
**Actions:** quando faltar `sh` no PATH, `validate` e `doctor` avisam (warning, não erro). Os artefatos `.sh` de
conveniência (attention, trackfw-validate, runner do lefthook, husky) passam a ser declarados como dependentes
de Git Bash no Windows. Guard não entra nessa declaração.
**Acceptance criteria:**
- [x] regra nova com default `warning`, nos dois sítios do `validator.go`, com pin em `check-validate-rule-pins.sh` (`windows_sh_missing`; pins 32-33; a fiação é afirmada pelo teste Go do ML-3D2)
- [x] teste com PATH sem `sh` (aviso presente) e com `sh` (ausente)

### ML-3D2 — Corretivo da revisão do hefesto (F2, F3, F4)
**Status:** ✅ Done
**Squad:** apolo-tf
**Files affected:** os do ML-3D e, se preciso, `internal/validator/export_test.go`
**Diagnóstico:** `docs/qualidade/2026-11-16-ml-3d-windows-sh-missing-review.md`. F2: o teste do doctor monta os findings
no próprio teste e nunca chama `runDoctor`. F3: nenhum teste passa pelas duas linhas de fiação do `validator.go`. F4:
a REQ-IMP-9 pede o aviso também sem artefato `.sh`, porque o barrier falha sem `sh`. É a mesma causa, por isso entra aqui.
**Acceptance criteria:**
- [x] teste do doctor chama `runDoctor`; mutação do loop em `doctor.go` reprova
- [x] teste de fiação do `ValidateUnfiltered` e do caminho tagged; remover cada linha de fiação reprova (a do `ValidateUnfiltered` foi mutada também pelo arquiteto: reprova)
- [x] aviso também sem artefato, com a frase alinhada à `shMissingMsg` do barrier

## Wave 4 — Medição na VM por CLI (depende do KG)
> Dependencies: Wave 3. Cada CLI precisa estar instalado na VM.

### ML-4A — Flags do PowerShell e HOME por CLI no Windows (AC6, REQ-IMP-8)
**Status:** ✅ Done
**Inventário da VM (2026-11-17, `where` + `dir` via SSH):** só o `claude` está instalado (npm global). Codex, Gemini, Cursor, Copilot, Windsurf, Kiro e Amazon Q **não estão instalados → não medidos**. Git for Windows presente em `C:\Program Files\Git`, com `sh` fora do PATH interativo, que é a condição pedida pela medição do husky.
**Squad:** artemis-tf (medição), apolo-tf (correção, se houver)
**Actions:** para Codex, Gemini, Cursor, Copilot, Windsurf, Kiro e Amazon Q, medir na VM:
- se o hook roda o PowerShell com `-NonInteractive`/`-NoProfile`, usando um stub que registra `argv` e os bytes
  de stdin;
- qual HOME o CLI lê.

Pela ADR-2026-10-14 D2, o `HomeFor` só muda para um CLI medido. Se um CLI não passar `-NonInteractive`, o A1 da
forma K deixa de valer para ele, porque a forma L cobre esse caso. Registrar.
**Acceptance criteria:**
- [x] tabela CLI × (`-NonInteractive`, `-NoProfile`, HOME) medida; CLI não instalado fica declarado como não medido — medida em 2026-11-17:

  | CLI | shell do hook | `-NonInteractive` | `-NoProfile` | HOME da config global |
  |---|---|---|---|---|
  | Codex 0.169.0 | `powershell.exe` 5.1 (`-Command`), PATH herdado | **não** | **sim** | `CODEX_HOME` se definido (config na raiz); senão o perfil do SO por API (`FOLDERID_Profile`), ignorando `HOME` e `USERPROFILE` |
  | Gemini, Cursor, Copilot, Windsurf, Kiro, Amazon Q | — | não medido (não instalado na VM) | não medido | não medido |

  Como o Codex não passa `-NonInteractive`, o A1 da forma K não vale para ele, e a forma L cobre o caso. O guard que o trackfw grava
  para o Codex roda no shell certo e **não** está inerte. A hipótese `cmd.exe /C` + `env_clear()`, tirada da leitura do fonte, foi
  refutada por sonda no binário (`docs/portabilidade/2026-11-17-ml-4a3-guard-do-trackfw-no-codex-windows.md`, seção de correção).
  Evidência do HOME: `docs/portabilidade/2026-11-17-ml-4a-codex-flags-e-home-windows.md`.
- [x] hook do husky (`.husky/pre-commit`) executado na VM com `sh` fora do PATH interativo: se rodar pelo `sh` do Git for
  Windows, `.husky/*` sai da lista da regra `windows_sh_missing` (parecer do hefesto F1, 2026-11-16) — **medido em 2026-11-17: roda.**
  O Git for Windows 2.55.0 (ARM64) põe `C:\Program Files\Git\usr\bin` no PATH do hook, e os filhos nativos herdam esse PATH.
  O contra-braço `exit 1` bloqueou o commit. A medição complementar mostrou o mesmo mecanismo no lefthook `runner: sh`: um probe Go
  com `exec.LookPath("sh")` acha o sh dentro do hook e não o acha fora. Evidência: `docs/portabilidade/2026-11-17-ml-4a-husky-sem-sh-no-path.md`.

### ML-4A2 — Tirar da regra `windows_sh_missing` os artefatos que um hook do git executa
**Status:** ✅ Done
**Squad:** apolo-tf
**Por que o escopo original não previa:** o ML-3D listou todo `.sh` gerado como dependente do PATH interativo, e a medição do ML-4A
mostrou que, para um hook do git, isso é falso.
**Actions:** remover `.husky/pre-commit`, `.husky/commit-msg` e `.lefthook/commit-msg/trackfw-req-check.sh` de `shConvenienceArtifacts`
(`internal/validator/validator_windows_sh_missing_rule.go`); ajustar os testes e o `docs/cli-parity.md`.
**Acceptance criteria:**
- [x] a lista tem apenas os 3 `scripts/trackfw-*.sh`
- [x] um teste afirma que um fixture só com `.husky/*` + `.lefthook/...` produz a variante barrier-only (sem lista de scripts)
- [x] `go test ./internal/validator/ ./internal/commands/ ./internal/pathguard/` verde; `bash scripts/check-validate-rule-pins.sh` verde — auditado pelo zeus em 2026-11-17: `TestWindowsShMissing_GitHookArtifactsNotListed` PASS (braço e contra-braço), pins rc=0

### ML-4A4 — O home global do Codex segue o que o Codex lê (ADR-2026-10-14 D2)
**Status:** ✅ Done
**Squad:** apolo-tf (implementação), hefesto-tf (revisão)
**Por que o escopo original não previa:** a D2 manda mudar o `HomeFor` só para um CLI medido, e o Codex foi medido no ML-4A.
**Defeito:** `~/.codex/hooks.json` (guard global) é montado com `homedir.Dir()`, que no Windows prefere `HOME`, e nenhuma resolução
consulta `CODEX_HOME`. O Codex lê `CODEX_HOME` (config na raiz, sem `.codex`) e, sem ele, o perfil do SO. Assim, com `HOME` definido
(Git Bash) ou com `CODEX_HOME` definido, o trackfw grava e audita o guard global num arquivo que o Codex não lê.
**Actions:** espelhar `ClaudeConfigDirWithSource` (`internal/agentconfig/agentconfig.go`) para o Codex: `CODEX_HOME` absoluto vence,
em qualquer SO (relativo é erro, como no `CLAUDE_CONFIG_DIR`); senão, `HomeFor(CLICodex)` + `.codex`, com `HomeFor(CLICodex)` no
Windows igual ao do Claude. Todos os sítios que montam `<home>/.codex/...` para a config global passam a usar a função nova, na
escrita e na auditoria (doctor/validate).
**Escopo ampliado na execução (mesma causa):** o aviso D5 de cópia inerte para o Codex; as mensagens com o caminho real; os agentes do Codex
(`~/.codex/agents`) via `Manager.CodexDir`, ligado em `update`, `init` (2 sítios) e `agents/skills install|list`; e a contenção `destRoot`
em `integrations/manager.go`. As skills em `~/.agents/skills` não mudam.
**Resíduo aceito:** `USERPROFILE` sobrescrito à mão ≠ `FOLDERID_Profile`, o mesmo trade-off do Claude na ADR-2026-10-14.
**Revisão:** hefesto APROVA COM NOTAS (`docs/qualidade/2026-11-17-ml-4a4-codex-config-dir-review.md`), com 7 mutações provadas. A NOTA-1
(2 testes de D5 ausentes) vai para o ML-4A5.
**Acceptance criteria:**
- [x] `grep -rn 'Join(home, ".codex"' internal --include='*.go' | grep -v _test | grep -v internal/agentconfig/` vazio (o critério foi corrigido pelo parecer do hefesto, NOTA-2: as 2 linhas dentro do agentconfig são o próprio resolvedor e a sonda D5)
- [x] testes: `CODEX_HOME` absoluto → caminho na raiz; relativo → erro; Windows com `HOME` ≠ `USERPROFILE` → `USERPROFILE\.codex`; macOS/Linux sem `CODEX_HOME` → `$HOME/.codex` (sem mudança)
- [x] escrita e auditoria do guard global do Codex usam o mesmo caminho (um teste por sítio)
- [x] `go test ./internal/agentconfig/ ./internal/generators/ ./internal/commands/ ./internal/pathguard/` verde

### ML-4A5 — Testes do aviso D5 do Codex (NOTA-1 do hefesto no ML-4A4)
**Status:** ✅ Done
**Squad:** apolo-tf
**Acceptance criteria:**
- [x] `TestHarnessD5NoticeCodex_WindowsDivergence_RemoveItByHand`: o braço "remove it by hand" é afirmado positivamente
- [x] `TestHarnessD5NoticeCodex_GitBranch_CODEX_HOME_ConditionalMessage`: `codex-git-branch-guard` com aviso D5
- [x] contra-braço por mutação, declarado no relatório; `go test ./internal/generators/` verde
  Mutações: o texto do braço `else` foi trocado (reprova o 1º) e o `withD5NoticeCodex` foi tirado do retorno do git-branch (reprova o 2º e não o de credential). Resíduo: o caso misto CODEX_HOME + divergência no Windows não tem teste dedicado no Codex; a lógica é a mesma função já coberta.

## Wave 5 — Red team, `make quality` e CI de Windows
> Dependencies: todas as anteriores

### ML-FA — Red team da entrega
**Status:** ⬜ Pending
**Squad:** hades-tf
**Acceptance criteria:**
- [ ] parecer com cada tentativa e o rc medido

### ML-FB — `make quality` com a máquina ociosa e CI verde
**Status:** ⬜ Pending
**Squad:** zeus-tf
**Acceptance criteria:**
- [ ] `make quality` rc=0
- [ ] `windows-full-suites` verde, e as falhas, por nome, iguais à lista conhecida
