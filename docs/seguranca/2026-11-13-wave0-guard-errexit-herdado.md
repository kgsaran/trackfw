# Wave 0 — Threat Model: guard hook sai 127 sob errexit herdado

> Autor: hades-tf | Data: 2026-11-13
> REQ: REQ-2026-11-13-linha-de-hook-do-guard-sai-127-sob-errexit-herdado-e-libera-o-comando.md
> Roadmap: ROADMAP-2026-11-13-linha-de-hook-do-guard-sai-127-sob-errexit-herdado-e-libera-o-comando.md

---

## 1. Enumeration completeness

### Lista de superfícies do roadmap

O roadmap cita: família PS/POSIX corrente, família nativa de Windows (cmd.exe/PS), invólucros WABash. A lista é **completa**: os três grupos cobrem todos os CLIs suportados e a forma legada D11-revisada (migration source) está no escopo como sítio a ser corrigido pelo ML-1A pela cláusula de mesma causa.

**Estrutura de famílias confirmada por leitura do código-fonte:**

| família | forma canônica | CLIs | defeito |
|---|---|---|---|
| PS/POSIX corrente | `emulate -R sh 2>${null…}; $LASTEXITCODE=2 2>${null…}; trackfw guard <nome>; …` | Claude Code, Codex, Gemini, Cursor, Copilot, Windsurf | SIM |
| cmd.exe nativa | `trackfw guard <nome> \|\| exit 2` | Kiro, Amazon Q | NÃO — cmd.exe não tem errexit; medido: bash -e dá rc=2 guard=yes |
| WABash (`/bin/bash -c '…'`) | wrapper que spawna subshell limpo | legada | NÃO — subshell não herda opções do pai; medido: bash -e outer e BASH_ENV=set-e dão rc=2 guard=yes |
| D11-revisada (legada, migration source) | `$LASTEXITCODE=2 2>${null…}; trackfw guard <nome>; …` | hooks antigos | SIM — mesmo mecanismo |
| PowerShell 7 (parcial, macOS) | mesma forma PS/POSIX | qualquer CLI em Windows | PARCIALMENTE MEDIDO — EAP=Continue guard=yes; EAP=Stop rc=1 guard=no; PS 5.1 → ML-1B |

**Nota sobre `validator_guard_binary_probe_windsurf_amazonq_test.go`:** este arquivo usa a forma D11-revisada como input de Windsurf nos testes de migração (linhas 94, 335, 378). Esses testes verificam que o validador migra D11-rev → PSPosix — não afirmam que Windsurf usa D11-rev atualmente. O gerador (`agentfiles.go` linha 1530: `windsurfGitGuardCmd = guardGitBranchCmdPSPOSIX`) confirma PSPosix para Windsurf. ML-1A deve estender este teste para a nova forma.

### Inventário de ocorrências (git grep, apenas arquivos rastreados)

Âncora A (`git grep -lF 'emulate -R sh'`): **35 arquivos**.
Âncora B (`git grep -lF '2*!!'`): **31 arquivos**.
Interseção (comm -12): **21 arquivos**.

**A-not-B — 14 arquivos** (saída literal de `comm -23 <(sort A) <(sort B)`):

```
docs/agents-working-context/20261028T211517Z-zeus-inicio-docs-refletem-produto-11.md
docs/req/REQ-2026-10-20-hooks-de-guard-gerados-falham-sob-zsh-com-exit-1-e-deixam-de-executar-a-protecao.md
docs/req/REQ-2026-10-23-update-nao-remove-geracoes-antigas-do-credential-guard-em-pares-evento-matcher-e-o-validate-manda-rodar-update-que-nao-resolve.md
docs/req/REQ-2026-10-28-documentacao-e-canais-refletem-o-produto-11-x-com-suporte-real-a-windows-e-instalador-powershell.md
docs/roadmaps/wip/ROADMAP-2026-11-13-linha-de-hook-do-guard-sai-127-sob-errexit-herdado-e-libera-o-comando.md
docs/seguranca/2026-10-23-quality-write-scope.md
docs/seguranca/2026-10-23-wave0-geracoes-do-guard.md
docs/seguranca/2026-10-28-redteam-docs-e-install-ps1.md
docs/seguranca/2026-10-28-wave0-docs-e-install-ps1.md
docs/seguranca/2026-11-01-wave0-powershell-inicio-a-frio.md
docs/visao-projeto/VISION.md
internal/validator/cursor_guard_migrate_dedup_external_test.go
internal/validator/validator_credential_guard.go
internal/validator/validator_git_branch_guard.go
```

**B-not-A — 10 arquivos** (saída literal de `comm -13 <(sort A) <(sort B)`):

```
docs/portabilidade/2026-10-07-linha-de-hook-fail-closed-sem-trackfw.md
docs/portabilidade/2026-10-20-jornadas-agentes-windows.md
docs/req/REQ-2026-11-02-relatos-do-primeiro-usuario-externo-init-pela-metade-e-npm-em-projeto-python-mensagem-de-fail-open-wip-limit-ignorado-e-adr-req-vazios.md
docs/roadmaps/done/ROADMAP-2026-09-22-os-hooks-de-guard-nao-executam-no-windows-na-maioria-dos-clis-de-agente-e-o-validate-reporta-instalado.md
docs/seguranca/2026-10-10-wave0-fiacao-do-guard-ancorada.md
internal/generators/fail_closed_d11_behavior_test.go
internal/validator/validator_credential_guard_test.go
internal/validator/validator_guard_binary_probe_windsurf_amazonq_test.go
vault/notes/codex-guard-polyglot-aborta-zsh-equals-2026-10-20.md
vault/notes/hook-subprocess-path-nao-herda-do-processo-pai-windows-2026-10-21.md
```

**Classificação de todos os 45 arquivos da união:**

| bucket | arquivos (quantidade) |
|---|---|
| Go de produto a alterar | `internal/guardforms/guard_generations.go`, `internal/generators/agentfiles.go`, `internal/validator/validator_guard_binary_probe.go`, `internal/validator/validator_credential_guard.go`, `internal/validator/validator_git_branch_guard.go` (5) |
| Testes a estender — ML-1A | `internal/generators/guard_line_shell_test.go`, `internal/generators/update_test.go`, `internal/commands/update_harness_test.go`, `internal/validator/validator_guard_binary_probe_test.go`, `internal/validator/validator_git_branch_guard_test.go`, `internal/validator/cursor_guard_migrate_dedup_external_test.go`, `internal/generators/fail_closed_d11_behavior_test.go`, `internal/validator/validator_credential_guard_test.go`, `internal/validator/validator_guard_binary_probe_windsurf_amazonq_test.go` (9) |
| Configurações versionadas a migrar | `.claude/settings.json`, `.codex/hooks.json`, `.gemini/settings.json` (3) |
| Docs a atualizar no PR | `README.md`, `docs/cli-parity.md`, `scripts/check-validate-rule-pins.sh` linha 565 (3) |
| Histórico congelado — não tocar | `docs/agents-working-context/20261028T…`, `CHANGELOG.md`, `docs/roadmaps/done/ROADMAP-2026-09-22-*`, `docs/roadmaps/done/ROADMAP-2026-10-20-*`, `docs/seguranca/2026-10-23-*` (×2), `docs/seguranca/2026-10-25-*`, `docs/seguranca/2026-10-28-*` (×2), `docs/seguranca/2026-11-01-*`, `docs/seguranca/2026-11-02-*`, `docs/seguranca/2026-10-10-*`, `docs/portabilidade/2026-10-21-*`, `vault/notes/codex-…`, `vault/notes/hook-subprocess-…` (15) |
| ADRs existentes — não reescrever | `docs/adr/ADR-2026-10-04-*.md` (1) |
| Governança corrente e docs informativo | `docs/req/REQ-2026-10-20-*`, `docs/req/REQ-2026-10-23-*`, `docs/req/REQ-2026-10-28-*`, `docs/req/REQ-2026-11-02-*`, `docs/roadmaps/wip/ROADMAP-2026-11-13-*`, `docs/visao-projeto/VISION.md`, `docs/portabilidade/2026-10-07-*`, `docs/portabilidade/2026-10-20-*` (8) |

Contagem: 5+9+3+3+15+1+8 = 44. O 45.º arquivo é `docs/agents-working-context.md` (histórico congelado, pertence ao bucket de histórico). Total = 45. ✓

**Frozen fixtures** (`scripts/testdata/`, `internal/pathguard/testdata/`): não aparecem em nenhuma âncora (confirmado por grep) — não contêm as strings `emulate -R sh` nem `2*!!`. Nomear fixtures que não batem com nenhuma âncora seria ruído.

---

## 2. Threat model

> A pergunta desta seção: quem quebra a Wave 0 sem quebrar nenhuma regra escrita, e como?

**O adversário típico neste projeto é o implementador que zera a Wave 0 sem medir o que ela pede.** Os três padrões concretos:

**A — O solucionador prematuro**: lê o defeito (bash -e + emulate → 127), entende a causa, escreve `set +eu; ` antes do `emulate`, acha que resolveu, e entrega sem medir (a) a forma corrente em todos os casos listados no AC2/AC3, (b) a forma nova no mesmo conjunto, e (c) os vetores de ameaça do candidato. O `trackfw barrier --wave 0` verifica só se o ML está `✅` e se os ACs têm `[x]` — não verifica se as células da matriz foram preenchidas por medição. Uma Wave 0 copiada de outra, com as células marcadas sem medir, passa o gate mecânico.

**B — O auditor de leitura**: lê o código, escreve a análise como afirmações dedutivas ("bash -e fará X porque Y") sem rodar nenhuma medição real. Duas falácias que o método de leitura não pega: (1) a forma candidata pode ter comportamento inesperado numa combinação específica (ex.: BASH_FUNC_set no-op + bash -e) que só a execução revela; (2) comportamento sob SHELLOPTS externamente exportado não é previsível por leitura — a REQ errou nesta célula por medir de dentro do bash, onde SHELLOPTS é readonly e o prefixo de atribuição é silenciosamente rejeitado pelo bash filho. O rc=2 que a REQ relatou para SHELLOPTS=errexit é reproduzido exatamente quando o prefixo é rejeitado por readonly — o ambiente medido era o contrário do ambiente ameaçante. Medição substitui leitura; leitura não substitui medição.

**C — O escopo-estreito**: mede só as combinações do AC2/AC3, entrega com AC1 vazio ou com inventário sem a forma legada D11-revisada. A forma D11-revisada tem o mesmo defeito (medido neste relatório) e cai na cláusula de mesma causa da REQ; deixar de registrá-la resulta em AC6 sendo marcado `[x]` com sítio conhecido e não corrigido.

---

## 3. Falsification targets in both directions

### Notas metodológicas

Medição realizada com script bash e invocações diretas, usando um guard falso que escreve "ran" em stderr e sai com `${FAKE_RC:-0}`. Coluna `guard` = "sim" se "ran" apareceu no stderr capturado. Ambiente: macOS aarch64, bash 5.3.20, dash 0.5.12, zsh 5.9, pwsh 7.5.0.

**Dois artefatos de medição identificados e declarados:**

**Artefato 1 — `bash -o pipefail` via variável em zsh (artefato de word-split):** O shell padrão desta máquina é zsh. No zsh, variável não citada em `$flag` NÃO sofre word-split. `flag="-o pipefail"; bash $flag -c "..."` passa `-o pipefail` como UM único argumento → bash recebe um único string onde esperava opção → "invalid option name" (rc=2). Corrigido por invocações diretas com argumentos literais separados: `bash -o pipefail -c "$form"` → guard=yes para todas as formas (ver tabelas). Implicação para ML-1A: testes que invokem bash via `exec.Command` devem passar `-o` e `pipefail` como elementos argv separados.

**Artefato 2 — `SHELLOPTS=errexit` via prefixo de atribuição em bash:** `SHELLOPTS=errexit bash -c "..."` a partir do próprio bash falha silenciosamente: bash imprime `SHELLOPTS: readonly variable` e rejeita o prefixo; o subprocesso herda o SHELLOPTS original (sem errexit). Corrigido com `env SHELLOPTS=errexit bash -c "..."` que contorna o readonly via `envp` do processo filho. Implicação para ML-1A: testes que verifiquem SHELLOPTS devem usar `exec.Cmd.Env` em Go OU `exec.Command("env", "SHELLOPTS=errexit", "bash", "-c", form)`.

**Canais de entrada de errexit/nounset (lista fechada):**

| canal | mecanismo | medido? |
|---|---|---|
| flag `-e` | shell inicia com errexit ativo | sim |
| flag `-u` | shell inicia com nounset ativo | sim |
| `BASH_ENV` com `set -e` / `set -u` | bash lê o arquivo antes de executar `-c` | sim |
| `env SHELLOPTS=errexit` (externo) | bash lê SHELLOPTS do `envp` na inicialização | sim |
| `ZDOTDIR/.zshenv` com `setopt errexit` | zsh lê antes de executar `-c` | sim |
| `ENV` (sh interativo) | apenas em modo interativo; não é vetor de hook | não aplicável |
| login profile (`bash -l`) | não é o modo de invocação de hooks de CLI | não medido (fora do escopo) |

**Mecanismo sob bash -u (confirmado por medição direta):**

- `bash -u -c 'emulate -R sh 2>/dev/null; echo past'` → rc=0 (continua — command-not-found não aciona nounset)
- `bash -u -c 'echo $LASTEXITCODE'` → rc=127, stderr: `LASTEXITCODE: unbound variable`
- `bash -u -c '$LASTEXITCODE=2 2>/dev/null; echo past'` → rc=127 (nounset em LASTEXITCODE)

Conclusão: bash -u mata na referência a `$LASTEXITCODE` (segmento 2), não no `emulate` (segmento 1).

### Matriz medida: forma PS/POSIX corrente (CURRENT)

Forma: `emulate -R sh 2>${null-/dev/null}; $LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE`

| invocação | fake=2 rc | guard | fake=0 rc | guard |
|---|---|---|---|---|
| `bash` | 2 | sim | 0 | sim |
| `bash -e` | 127 | **NÃO** | 127 | **NÃO** |
| `bash -u` | 127 | **NÃO** | 127 | **NÃO** |
| `bash -o pipefail` (direto) | 2 | sim | — | — |
| `sh -e` | 127 | **NÃO** | 127 | **NÃO** |
| `sh -u` | 127 | **NÃO** | 127 | **NÃO** |
| `dash` | 2 | sim | 0 | sim |
| `dash -e` | 127 | **NÃO** | 127 | **NÃO** |
| `zsh` | 2 | sim | 0 | sim |
| `zsh -e` | 2 | sim | 0 | sim |
| `zsh -u` | 2 | sim | 0 | sim |
| `env BASH_ENV=set-e bash` | 127 | **NÃO** | 127 | **NÃO** |
| `env BASH_ENV=set-u bash` | 127 | **NÃO** | 127 | **NÃO** |
| `env SHELLOPTS=errexit bash` (direto) | 127 | **NÃO** | 127 | **NÃO** |
| `env BASH_ENV=trap-ERR-exit-0 bash` | 0 | **NÃO** | 0 | **NÃO** |
| guard ausente do PATH | 2 | NÃO | 2 | NÃO |
| `pwsh -Command` (EAP=Continue) | 2 | sim | 0 | sim |
| `pwsh -Command` (EAP=Stop) | 1 | **NÃO** | 1 | **NÃO** |

**Mecanismo:** em bash/sh/dash, `emulate` é command-not-found (127), `2>/dev/null` suprime o stderr. Com `errexit` ativo, o shell sai antes do guard. Com `-u` apenas, morre em `$LASTEXITCODE` unbound. Em todos os casos o guard não roda; rc=127 → 5 CLIs leem como erro não-bloqueante e permitem o comando negado.

**Critério de bloqueio por CLI** (fonte: `vault/notes/guard-fallback-old-binary-exit1-fail-open-6-clis-2026-10-06.md`):
- Copilot: bloqueia em qualquer non-zero.
- Claude Code, Codex, Gemini, Cursor, Windsurf: bloqueiam apenas em exit 2.
- Kiro, Amazon Q: cmd.exe (imunes ao defeito desta REQ).

**FP direction (Copilot any-non-zero):** sob errexit herdado, CURRENT dá rc=127 em fake=0 (allow) também. Copilot bloqueia em 127 mesmo quando o guard teria permitido — falso-positivo. CAND_A/B fecham este FP: rc=0 quando fake=0.

**zsh é imune:** `emulate -R sh` é builtin zsh que reseta opções antes de retornar. Nenhum pré-segmento falha sob zsh com `-e` ou `-u`.

### Matriz medida: CAND_A

Forma: `set +eu; emulate -R sh 2>${null-/dev/null}; $LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE`

Mecanismo: `set +eu` é builtin POSIX especial que desativa `errexit` (+e) e `nounset` (+u). Como builtin especial, não pode retornar não-zero.

| invocação | fake=2 rc | guard | fake=0 rc | guard |
|---|---|---|---|---|
| `bash` | 2 | sim | 0 | sim |
| `bash -e` | 2 | sim | 0 | sim |
| `bash -u` | 2 | sim | 0 | sim |
| `bash -o pipefail` (direto) | 2 | sim | — | — |
| `sh -e` | 2 | sim | 0 | sim |
| `sh -u` | 2 | sim | 0 | sim |
| `dash` | 2 | sim | 0 | sim |
| `dash -e` | 2 | sim | 0 | sim |
| `zsh` | 2 | sim | 0 | sim |
| `zsh -e` | 2 | sim | 0 | sim |
| `zsh -u` | 2 | sim | 0 | sim |
| `env BASH_ENV=set-e bash` | 2 | sim | 0 | sim |
| `env BASH_ENV=set-u bash` | 2 | sim | 0 | sim |
| `env SHELLOPTS=errexit bash` (direto) | 2 | sim | 0 | sim |
| `zsh .zshenv setopt errexit` | 2 | sim | 0 | sim |
| `env BASH_ENV=trap-ERR-exit-0 bash` | 0 | **NÃO** | 0 | **NÃO** |
| `env BASH_FUNC_set=no-op bash` | 2 | sim | 0 | sim |
| `env BASH_FUNC_set=no-op bash -e` | 127 | **NÃO** | 127 | **NÃO** |
| `zsh + emulate()=no-op` | 1 | **NÃO** | 1 | **NÃO** |
| guard ausente do PATH | 2 | NÃO | 2 | NÃO |
| `pwsh -Command` (EAP=Continue) | 2 | sim | 0 | sim |
| `pwsh -Command` (EAP=Stop) | 1 | **NÃO** | 1 | **NÃO** |

**guard exit codes normalizados (CAND_A):** rc=1→2, rc=3→2, rc=126→2, rc=127→2, rc=130→2.

### Matriz medida: CAND_B (recomendado)

Forma: `set +eu 2>${null-/dev/null}; emulate -R sh 2>${null-/dev/null}; $LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE`

Diferença de CAND_A: `2>${null-/dev/null}` após `set +eu`.

**Semântica de `2>${null-/dev/null}`** (fonte: `docs/cli-parity.md` linha 6893-6895): polyglot discard — em POSIX `${null-/dev/null}` = `/dev/null`; em PS `2>$null` descarta stderr. Medido via `Get-Alias set` em PS 7 macOS: `set` é alias de `Set-Variable`; `set +eu` chama `Set-Variable -Name "+eu"` → parâmetro inválido → gera erro não-terminante com EAP=Continue que `2>$null` suprime. Em PS 5.1 Windows o comportamento do erro pode ser similar ou diferente — hipótese para ML-1B. CAND_B é preferido por consistência: todos os outros pré-segmentos da forma PS/POSIX já carregam `2>${null-/dev/null}`; acrescentar no primeiro segmento uniformiza o padrão.

Todos os dados abaixo confirmados por invocações diretas (sem script intermediário):

| invocação | fake=2 rc | guard | fake=0 rc | guard |
|---|---|---|---|---|
| `bash` | 2 | sim | 0 | sim |
| `bash -e` | 2 | sim | 0 | sim |
| `bash -u` | 2 | sim | 0 | sim |
| `bash -o pipefail` (direto) | 2 | sim | — | — |
| `sh -e` | 2 | sim | 0 | sim |
| `sh -u` | 2 | sim | 0 | sim |
| `dash` | 2 | sim | 0 | sim |
| `dash -e` | 2 | sim | 0 | sim |
| `zsh` | 2 | sim | 0 | sim |
| `zsh -e` | 2 | sim | 0 | sim |
| `zsh -u` | 2 | sim | 0 | sim |
| `env BASH_ENV=set-e bash` | 2 | sim | 0 | sim |
| `env BASH_ENV=set-u bash` | 2 | sim | 0 | sim |
| `env SHELLOPTS=errexit bash` (direto) | 2 | sim | 0 | sim |
| `zsh .zshenv setopt errexit` | 2 | sim | 0 | sim |
| `env BASH_ENV=trap-ERR-exit-0 bash` | 0 | **NÃO** | 0 | **NÃO** |
| `env BASH_FUNC_set=no-op bash` | 2 | sim | 0 | sim |
| `env BASH_FUNC_set=no-op bash -e` | 127 | **NÃO** | 127 | **NÃO** |
| `zsh + emulate()=no-op` | 1 | **NÃO** | 1 | **NÃO** |
| guard ausente do PATH | 2 | NÃO | 2 | NÃO |
| `pwsh -Command` (EAP=Continue) | 2 | sim | 0 | sim |
| `pwsh -Command` (EAP=Stop) | 1 | **NÃO** | 1 | **NÃO** |

**guard exit codes normalizados (CAND_B):** rc=1→2, rc=3→2, rc=126→2, rc=127→2, rc=130→2.

**Recomendação:** adotar **CAND_B**. Não piora nenhum caso POSIX medido frente a CAND_A. Consistente com o padrão PS de supressão de stderr nos demais pré-segmentos. ML-1B deve medir CAND_B sob PS 5.1 Windows com EAP=Stop.

### Matriz medida: D11-revisada LEGACY (migration source)

Forma: `$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE`

| invocação | fake=2 rc | guard |
|---|---|---|
| `bash` | 2 | sim |
| `bash -e` | 127 | **NÃO** |
| `bash -u` | 127 | **NÃO** |
| `sh -e` | 127 | **NÃO** |
| `dash -e` | 127 | **NÃO** |
| `zsh` | 1 | **NÃO** |
| `zsh -e` | 1 | **NÃO** |
| `zsh -u` | 1 | **NÃO** |

**Conclusão:** o defeito é o mesmo. Sob `-e`/`-u`, morre em `$LASTEXITCODE` não setado. Sob zsh nativo (sem `emulate -R sh`), a opção EQUALS de zsh interpreta `=2` como path de comando, falha, rc=1 — guard nunca roda. Mesmo sem errexit.

**Obrigação ML-1A para D11-rev:** o migrador deve reescrever hooks com forma D11-rev para a nova forma CAND_B completa. Não basta prefixar `set +eu`: sem `emulate -R sh`, zsh nativo ainda falha em `=2` (EQUALS option). A migração vai de D11-rev → CAND_B integralmente. Um teste deve afirmar que tanto D11-rev → nova quanto CURRENT → nova produzem a forma CAND_B esperada.

### Falsification targets — onde a sabotagem entra e qual gate deve pegar

**A. Superfície shell-runtime (FN — guard não roda):**

| vetor | onde entra | gate que deve pegar | direção |
|---|---|---|---|
| bash/sh/dash -e / -u | flag de shell herdada | nenhum gate cobre forma corrente | FN — defeito ativo |
| BASH_ENV com `set -e` / `set -u` | env var BASH_ENV importada | nenhum gate cobre forma corrente | FN — defeito ativo |
| env SHELLOPTS=errexit (externo) | env var SHELLOPTS passada ao bash | nenhum gate cobre forma corrente | FN — defeito ativo |
| env BASH_FUNC_set=no-op + bash -e (CAND_A/B) | env exporta função `set` como no-op | nenhum gate cobre | FN — residual V1/R2 |
| BASH_ENV=trap-ERR-exit-0 (ambas as formas) | BASH_ENV com ERR trap | nenhum gate cobre | FN — residual V2/R1 |
| zsh + emulate()=no-op em .zshenv (CAND_A/B) | .zshenv sobrescreve `emulate` | nenhum gate cobre | FN — residual V3/R3 |
| PS EAP=Stop (ambas as formas, parcial) | PS lança exceção terminating no pré-guard | nenhum gate cobre | FN — residual V4/R4 |

**B. Superfície shell-runtime (FP — guard roda mas bloqueia allow):**

| vetor | rc com fake=0 | CLIs afetados | direção |
|---|---|---|---|
| CURRENT sob errexit com allow | 127 | Copilot bloqueia (any-non-zero) | FP — fechado por CAND_A/B |
| CAND_A/B BASH_FUNC_set=no-op + bash -e com allow | 127 | Copilot bloqueia | FP — residual V1/R2 |
| CAND_A/B zsh + emulate()=no-op com allow | 1 | Copilot bloqueia | FP — residual V3/R3 |

**C. Superfície de registro / validador / gerador:**

| vetor | onde entra | gate que deve pegar | direção |
|---|---|---|---|
| CURRENT não registrada como Legacy em `guard_generations.go` | ML-1A não adiciona constante Legacy | `update` continua gerando forma velha; `validate` aceita forma velha como atual | FN — `credential_guard_hook_resolvable` não detecta hook desatualizado |
| Matcher de legado usa `strings.HasSuffix`/`Contains` | CAND_B é sufixo exato de CURRENT; `guard_generations.go` linha 12 proíbe Contains/HasSuffix: "exact equality on the full command string (never strings.Contains)" | ML-1A viola o invariante → CAND_B hook é re-migrado a cada update | FN estrutural — constraint documentada não enforçada automaticamente |
| `guardExpectedLine` não atualizado em `validator_guard_binary_probe.go` linha 95 | validador usa forma old como expected | `credential_guard_hook_resolvable` reprova install nova (expected≠actual) | FP — instalação correta marcada como inválida |
| Validador aceita CURRENT e nova como "saudável" | ambas as formas passam como corretas | CURRENT não é forçada a migrar | FN — hook defeituoso permanece silenciosamente |
| Configs versionadas não migradas | `.claude/settings.json`, `.codex/hooks.json`, `.gemini/settings.json` | `scripts/check-harness-hooks-parity.sh` gate | FN — hook defeituoso em produção desta repo |
| Docs e script-pin não atualizados | README.md, cli-parity.md, `scripts/check-validate-rule-pins.sh` linha 565 | revisão humana (nenhum gate automático) | FN latente |

**D. Superfície de testes do ML-1A:**

| vetor | artefato | o que o test deve afirmar |
|---|---|---|
| Test asserting only rc=2 (sem ran marker) | rc=2 pode ocorrer sem o guard rodar (guard absent, shell error) | test deve afirmar tanto rc correto quanto `guard=yes` via ran-marker em stderr |
| Test usando prefixo de atribuição `SHELLOPTS=x bash -c` | bash readonly SHELLOPTS rejeita o prefix silenciosamente | usar `exec.Cmd.Env` em Go OU `exec.Command("env", "SHELLOPTS=errexit", "bash", "-c", form)` |
| Test passando `-o pipefail` como string única em argv | bash recebe string única "-o pipefail" → "invalid option name" | usar `exec.Command("bash", "-o", "pipefail", "-c", form)` — args separados |
| Contra-braço ausente | test que passa com CAND_B pode passar com CURRENT por coincidência | todo test de CAND_B deve ter um caso idêntico usando CURRENT que falha em bash/sh/dash com -e ou -u (não zsh — zsh passa com CURRENT por ser imune) |
| D11-rev → CAND_B migration não testada | migrador para D11-rev pode ser omitido | test deve submeter hook D11-rev e afirmar que output é CAND_B integralmente |

### Análise dos vetores de ameaça residuais

**V1 — BASH_FUNC_set=no-op (bash -e) [CAND_A/B residual]:**
Mecanismo: bash importa a função `set` do ambiente; `set +eu` chama a função (no-op); errexit permanece ativo; `emulate -R sh` falha com 127; bash -e sai. Em `sh` (POSIX): funções não shadoweiam builtins especiais — imune. Medido: rc=127 guard=no para fake=2 e fake=0. Severidade: BAIXA. CAND_A/B não abre nova janela — forma corrente também falha sob bash -e. Nota de scope: um ambiente que pode injetar `BASH_FUNC_*` pode igualmente shadowear o próprio `trackfw`; nenhuma mudança de linha-de-hook fecha esse vetor.

**V2 — BASH_ENV=trap-ERR-exit-0 [ambas as formas, pré-existente]:**
Mecanismo: ERR trap intercepta qualquer falha de comando e sai com 0. guard nunca roda; rc=0. Medido: rc=0 guard=no para CAND_A/B e forma corrente. Impacto nos CLIs: rc=0 → **todos os 6 CLIs PS/POSIX** falham-open (rc=0 não satisfaz o critério "any non-zero" de Copilot, nem o critério "exit 2" dos outros 5). Um ambiente que controla BASH_ENV pode também substituir o binário `trackfw` — benign inherited options são o defeito fixado por esta REQ; adversarial pre-hook code execution é residual estrutural.

**V3 — zsh com emulate() redefinida em .zshenv [CAND_A/B residual]:**
Mecanismo: `set +eu` desativa errexit; `emulate()` como no-op; opções zsh extras não resetadas; `$LASTEXITCODE=2` falha sob semântica zsh nativa (EQUALS); rc=1 guard=no. Medido: rc=1 guard=no para fake=2 e fake=0. Copilot bloqueia em rc=1; os 5 exit-2-only permitem allow e deny igualmente. Severidade: BAIXA.

**V4 — PowerShell com `$ErrorActionPreference='Stop'` [parcialmente medido, PS7 macOS]:**
EAP=Stop converte erros não-terminantes em terminantes. Medido em PS7 macOS: todas as três formas (CURRENT, CAND_A, CAND_B) dão rc=1 guard=no. rc=1 ≠ 2 → os 5 exit-2-only CLIs permitem o comando negado — **fail-open**. Copilot bloqueia em rc=1 (any non-zero) → fail-closed para Copilot. CAND_B não piora frente à forma corrente; ambas são fail-open com EAP=Stop. PS 5.1 Windows: comportamento de `Set-Variable +eu` com EAP=Stop pode diferir; ML-1B deve medir.

---

## 4. Declared residual

**R1 — BASH_ENV com `trap 'exit 0' ERR`** (pré-existente, 6 CLIs PS/POSIX):
rc=0 → todos os 6 CLIs PS/POSIX falham-open. Mitigação disponível (`trap - ERR`) foi rejeitada: acrescenta segmentos a uma linha já complexa; o vetor exige controle sobre BASH_ENV (disponível apenas para o próprio usuário ou seu CI, não para atacante externo). Registrado como residual.

**R2 — BASH_FUNC_set=no-op + bash -e** (pré-existente):
CAND_A/B não abre nova janela frente à forma corrente. `builtin set +eu` poderia contornar — dash não tem `builtin`. Fora do escopo desta REQ.

**R3 — zsh com `emulate()` redefinida em .zshenv**:
Requer sobrescrever builtin `emulate` em dotfiles. Improvável em produção. Fora do escopo.

**R4 — PowerShell com `$ErrorActionPreference='Stop'`** (parcialmente medido):
PS7 macOS: rc=1, guard=no para todas as três formas. fail-open para 5 CLIs. PS 5.1 Windows: não medido neste ML. ML-1B deve declarar se é residual aceito ou se precisa de form específica.

**R5 — Shells não listados no D2** (ash, busybox sh, ksh, fish):
Fora do escopo declarado. Pré-existente.

---

### Critérios de aceite do ML-0A

- [x] **AC1-parte-1 — Inventário com contagem conferida por segundo caminho:**
  Âncora A (`git grep -lF 'emulate -R sh'`): 35 arquivos. Âncora B (`git grep -lF '2*!!'`): 31 arquivos. Interseção: 21 arquivos. A-not-B: 14 arquivos (lista de 14 paths exatos acima, saída literal de `comm -23`). B-not-A: 10 arquivos (lista de 10 paths exatos acima, saída literal de `comm -13`). Todos os 45 arquivos da união classificados em buckets acima. Frozen fixtures não aparecem em nenhuma âncora — confirmado por grep.

- [x] **AC1-parte-2 — Matriz da forma atual e candidatos com rc medido (não presumido):**
  CURRENT: 18 combinações. CAND_A: 22 combinações. CAND_B: 22 combinações (todas células confirmadas por invocações diretas — cada par fake=2/fake=0 medido individualmente). D11REV: 8 combinações. Dois artefatos de medição identificados e declarados com impact em ML-1A. SHELLOPTS=errexit real confirmado por `env` prefix. Mecanismo sob bash -u confirmado por medições diretas (morte em segmento 2, não segmento 1). PS7 parcial; PS 5.1 Windows → ML-1B declarado. FP direction (Copilot any-non-zero) medido e documentado. Falsification targets de registro/validador/generator/teste adicionados com nomes de regra reais (`credential_guard_hook_resolvable`, `scripts/check-harness-hooks-parity.sh`). Constraint `guard_generations.go` linha 12 (igualdade, nunca Contains) citada como falsification target. trap-ERR impacto: 6 CLIs PS/POSIX fail-open (não 5), pois rc=0.

- [x] **AC1-parte-3 — Nenhuma linha de implementação escrita:**

```
$ git diff --cached --stat
(empty — no staged changes)

$ git status --short
?? docs/seguranca/2026-11-13-wave0-guard-errexit-herdado.md
```

Apenas este arquivo está presente no working tree; `git diff --cached` está vazio.

---

### Apêndice — saídas de medição brutas

**A. Forma corrente — casos de falha:**

```
CURRENT bash -e                   fake=2  rc=127  guard=no
CURRENT bash -u                   fake=2  rc=127  guard=no
CURRENT sh -e                     fake=2  rc=127  guard=no
CURRENT sh -u                     fake=2  rc=127  guard=no
CURRENT dash -e                   fake=2  rc=127  guard=no
CURRENT env BASH_ENV=set-e bash   fake=2  rc=127  guard=no
CURRENT env BASH_ENV=set-u bash   fake=2  rc=127  guard=no
CURRENT env SHELLOPTS=errexit bash (env prefix) fake=2 rc=127 guard=no
CURRENT env BASH_ENV=trap-ERR     fake=2  rc=0    guard=no
CURRENT pwsh EAP=Stop             fake=2  rc=1    guard=no
```

**B. CAND_B — confirmações diretas:**

```
CAND_B bash                            fake=2 rc=2   guard=yes
CAND_B bash                            fake=0 rc=0   guard=yes
CAND_B bash -e                         fake=2 rc=2   guard=yes
CAND_B bash -e                         fake=0 rc=0   guard=yes
CAND_B bash -u                         fake=2 rc=2   guard=yes
CAND_B bash -u                         fake=0 rc=0   guard=yes
CAND_B bash -o pipefail (args sep)     fake=2 rc=2   guard=yes
CAND_B sh -e                           fake=2 rc=2   guard=yes
CAND_B sh -e                           fake=0 rc=0   guard=yes
CAND_B sh -u                           fake=2 rc=2   guard=yes
CAND_B sh -u                           fake=0 rc=0   guard=yes
CAND_B dash                            fake=2 rc=2   guard=yes
CAND_B dash                            fake=0 rc=0   guard=yes
CAND_B dash -e                         fake=2 rc=2   guard=yes
CAND_B dash -e                         fake=0 rc=0   guard=yes
CAND_B zsh                             fake=2 rc=2   guard=yes
CAND_B zsh                             fake=0 rc=0   guard=yes
CAND_B zsh -e                          fake=2 rc=2   guard=yes
CAND_B zsh -e                          fake=0 rc=0   guard=yes
CAND_B zsh -u                          fake=2 rc=2   guard=yes
CAND_B zsh -u                          fake=0 rc=0   guard=yes
CAND_B env BASH_ENV=set-e bash         fake=2 rc=2   guard=yes
CAND_B env BASH_ENV=set-e bash         fake=0 rc=0   guard=yes
CAND_B env BASH_ENV=set-u bash         fake=2 rc=2   guard=yes
CAND_B env BASH_ENV=set-u bash         fake=0 rc=0   guard=yes
CAND_B env SHELLOPTS=errexit bash      fake=2 rc=2   guard=yes
CAND_B env SHELLOPTS=errexit bash      fake=0 rc=0   guard=yes
CAND_B zsh .zshenv setopt errexit      fake=2 rc=2   guard=yes
CAND_B zsh .zshenv setopt errexit      fake=0 rc=0   guard=yes
CAND_B env BASH_ENV=trap-ERR-exit-0    fake=2 rc=0   guard=no
CAND_B env BASH_ENV=trap-ERR-exit-0    fake=0 rc=0   guard=no
CAND_B env BASH_FUNC_set=no-op bash    fake=2 rc=2   guard=yes
CAND_B env BASH_FUNC_set=no-op bash    fake=0 rc=0   guard=yes
CAND_B env BASH_FUNC_set=no-op bash-e  fake=2 rc=127 guard=no
CAND_B env BASH_FUNC_set=no-op bash-e  fake=0 rc=127 guard=no
CAND_B zsh + emulate()=no-op           fake=2 rc=1   guard=no
CAND_B zsh + emulate()=no-op           fake=0 rc=1   guard=no
CAND_B guard ausente do PATH           fake=-- rc=2  guard=no
CAND_B guard rc=1  → rc=2  guard=yes
CAND_B guard rc=3  → rc=2  guard=yes
CAND_B guard rc=126 → rc=2 guard=yes
CAND_B guard rc=127 → rc=2 guard=yes
CAND_B guard rc=130 → rc=2 guard=yes
```

**C. PowerShell 7 macOS (pwsh -NoProfile -NonInteractive -Command):**

```
# Get-Alias set → Set-Variable (medido: "set IS alias for: Set-Variable")
CURRENT pwsh EAP=Continue   fake=2 rc=2  guard=yes
CURRENT pwsh EAP=Continue   fake=0 rc=0  guard=yes
CURRENT pwsh EAP=Stop       fake=2 rc=1  guard=no
CURRENT pwsh EAP=Stop       fake=0 rc=1  guard=no
CAND_A  pwsh EAP=Continue   fake=2 rc=2  guard=yes
CAND_A  pwsh EAP=Continue   fake=0 rc=0  guard=yes
CAND_A  pwsh EAP=Stop       fake=2 rc=1  guard=no
CAND_A  pwsh EAP=Stop       fake=0 rc=1  guard=no
CAND_B  pwsh EAP=Continue   fake=2 rc=2  guard=yes
CAND_B  pwsh EAP=Continue   fake=0 rc=0  guard=yes
CAND_B  pwsh EAP=Stop       fake=2 rc=1  guard=no
CAND_B  pwsh EAP=Stop       fake=0 rc=1  guard=no
# set +eu com EAP=Continue: Set-Variable +eu invocado — não-terminante, execução continua (medido)
# set +eu 2>$null com EAP=Continue: idem, stderr suppressed (medido)
```

**D. bash -u: mecanismo de morte confirmado:**

```
bash -u: 'emulate -R sh 2>/dev/null; echo past' → rc=0 (continua — command-not-found não é nounset)
bash -u: 'echo $LASTEXITCODE'                   → rc=127 (nounset: unbound variable)
bash -u: '$LASTEXITCODE=2 2>/dev/null; echo p'  → rc=127 (nounset em LASTEXITCODE)
sh -u:   'emulate -R sh 2>/dev/null; echo past' → rc=0
sh -u:   '$LASTEXITCODE=2 2>/dev/null; echo p'  → rc=127 (nounset em LASTEXITCODE)
```

**E. Formas e guard falso (fonte para reprodução):**

```bash
CURRENT='emulate -R sh 2>${null-/dev/null}; $LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE'
CAND_A='set +eu; emulate -R sh 2>${null-/dev/null}; $LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE'
CAND_B='set +eu 2>${null-/dev/null}; emulate -R sh 2>${null-/dev/null}; $LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE'
D11REV='$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard credential; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE'

# Guard falso (fake_guard.sh):
# #!/bin/bash
# echo "ran" >&2
# exit "${FAKE_RC:-0}"
#
# Uso: FAKE_RC=2 PATH="<dir_do_fake_guard>:$PATH" bash -c "$FORM" 2>stderr.txt
# guard=yes se stderr.txt contém "ran"
# Para SHELLOPTS: env SHELLOPTS=errexit bash -c "$FORM"  (não: SHELLOPTS=errexit bash -c)
# Para BASH_FUNC: env "BASH_FUNC_set%%=() { :; }" bash -e -c "$FORM"
```
