---
status: wip
date: 2026-11-13
req: "docs/req/REQ-2026-11-13-linha-de-hook-do-guard-sai-127-sob-errexit-herdado-e-libera-o-comando.md"
squad: "hades-tf, apolo-tf, artemis-tf, hefesto-tf, zeus-tf"
---

# Roadmap: linha de hook do guard sai 127 sob errexit herdado e libera o comando

> Created: 2026-11-13 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-11-13-linha-de-hook-do-guard-sai-127-sob-errexit-herdado-e-libera-o-comando.md -->
REQ: docs/req/REQ-2026-11-13-linha-de-hook-do-guard-sai-127-sob-errexit-herdado-e-libera-o-comando.md
ADR: docs/adr/ADR-2026-10-04-o-guard-de-hook-e-um-subcomando-go-do-trackfw-e-a-linha-de-hook-e-a-mesma-string-em-todo-shell.md (adendo de 2026-11-13, D18)
Origem: resíduo R4 da REQ-2026-10-20, não corrigido. Varredura de issues em 2026-11-13: zero abertas.

A linha de hook PS/POSIX tem segmentos que, em sh/bash/dash, falham por desenho (`emulate`, `$LASTEXITCODE=2`)
ou expandem variável não definida (`$LASTEXITCODE`). Com `errexit` ou `nounset` herdados, o shell sai 127 antes
do guard e o comando passa. Medição do arquiteto na REQ (macOS): 127 em `bash -e`, `bash -u`, `sh -e`, `sh -u`,
`dash -e` e `BASH_ENV` com `set -e`; 2 em `bash`, `zsh -e` e `zsh -u`. A Wave 0 corrigiu `SHELLOPTS=errexit`:
com `env`, dá 127 (o prefixo dentro do bash é rejeitado por `readonly`). No PowerShell 7,
`$ErrorActionPreference='Stop'` dá 1 sem chamar o guard.

Sítio canônico da forma esperada: `guardExpectedLine` em `internal/validator/validator_guard_binary_probe.go`.
O literal `emulate -R sh` aparece também em `internal/generators/agentfiles.go`,
`internal/guardforms/guard_generations.go`, `internal/validator/validator_credential_guard.go` e
`internal/validator/validator_git_branch_guard.go`. Matriz existente: `internal/generators/guard_line_shell_test.go`.

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [x] AC1 — inventário e matriz medida da forma atual (ML-0A)
- [x] AC2 — deny sai 2 sob as opções herdadas (ML-1A)
- [x] AC3 — allow sai 0 sob as opções herdadas (ML-1A)
- [ ] AC4 — PowerShell 5.1 e 7 no Windows (ML-1B)
- [x] AC5 — matriz de teste com contra-braço (ML-1A)
- [x] AC6 — geração legada, aviso e migração sem duplicata (ML-1A)
- [x] AC7 — configs deste repositório migradas (ML-1A)
- [ ] AC8 — `make quality` verde (ML-2B)

## Status Legend
⬜ Pending · 🔄 In progress · ✅ Done · ❌ Blocked

## Wave 0 — Threat Model e inventário (AC1)
> Dependencies: none. Blocks all implementation.

### ML-0A — Inventário das formas de linha de hook, matriz de opções herdadas e candidatos
**Status:** ✅ Done
**Squad:** hades-tf
**Files affected:**
- `docs/seguranca/2026-11-13-wave0-guard-errexit-herdado.md` (único arquivo escrito)

**Actions:**
1. Inventário: toda forma de linha de hook que o trackfw gera ou aceita como corrente (família PS/POSIX,
   POSIX-only, nativa de Windows, invólucro `/bin/bash -c`, registro da D12 em `guard_generations.go`). Busque o
   literal das formas no repositório inteiro, não só nos arquivos citados no Context, e confira a contagem por
   um segundo caminho.
2. Matriz da forma atual: para cada forma, rc com guard falso que nega (2) e que permite (0), sob `bash`,
   `bash -e`, `bash -u`, `sh -e`, `sh -u`, `dash -e` (se houver `dash`), `zsh -e`, `zsh -u`, `BASH_ENV` com
   `set -e` e com `set -u`, e `SHELLOPTS=errexit`. Meça só em diretório de `mktemp -d` com `trap` de limpeza e
   `PATH` apontando para o guard falso; nada de `git` no repositório (se precisar de git, use um teste Go em
   `t.TempDir()`). Diga, pela documentação de cada CLI de agente, com que shell ele executa o hook e se herda o
   ambiente do usuário.
3. Candidatos: pelo menos duas formas que neutralizem `errexit` e `nounset` antes de qualquer segmento que
   falhe (por exemplo `set +eu` no início), medidas na mesma matriz POSIX. Para cada uma, diga o que se espera
   no PowerShell 5.1 e 7 (parse e stderr); a medição no Windows é do ML-1B.
4. Threat model: como a forma nova poderia liberar o comando (guard ausente do PATH, rc fora de 0/2, `set -e`
   religado depois, alias ou função chamada `set`) e como poderia bloquear o legítimo.
5. Residual declarado.

**Acceptance criteria:**
- [x] Inventário com contagem conferida por segundo caminho e lista final dos sítios do ML-1A
- [x] Matriz da forma atual e dos candidatos com rc medido, não presumido
- [x] Nenhuma linha de implementação escrita; `git diff --cached` vazio ao fim

**Auditoria (Zeus, 2026-11-13):** aprovado com três correções.
- Reproduzi de forma independente: a forma atual dá 127 com deny e com allow sob `bash -e`, `sh -u`,
  `env SHELLOPTS=errexit` e `BASH_ENV` com `set -e`. O CAND_B (`set +eu 2>${null-/dev/null};` antes da forma
  atual) roda o guard e devolve 2 e 0 nas mesmas quatro condições. O relatório é o único arquivo novo, e o
  índice está vazio.
- **Correção 1 — inventário incompleto.** O relatório não cita `WriteScopeCurrentPSPOSIX` (guard `write-scope`)
  nem trata `CredentialGlobalCurrentPSPOSIX` como sítio. As quatro constantes `*CurrentPSPOSIX` de
  `internal/guardforms/guard_generations.go` entram no ML-1A.
- **Correção 2 — gate inexistente.** `scripts/check-harness-hooks-parity.sh` não existe. O gate de esquema de
  hook do repositório é `scripts/check-git-branch-guard-hook-schema.sh`.
- **Correção 3 — `$ErrorActionPreference='Stop'` não é residual.** O mecanismo é o mesmo da REQ (opção herdada
  encerra a linha antes do guard, D18). O ML-1B mede se ela chega ao hook de algum CLI suportado, com e sem
  perfil. Se chega, a forma que a cobre entra nesta REQ como ML corretivo.
- R1 (`trap 'exit 0' ERR` via `BASH_ENV`), R2 (`BASH_FUNC_set`) e R3 (`emulate()` redefinida) ficam como residual
  aceito: exigem código do usuário executado antes do hook, e quem tem isso também troca o binário `trackfw`.

**Wave gates:**
```bash
test -s docs/seguranca/2026-11-13-wave0-guard-errexit-herdado.md
```

## Wave 1 — Implementação (AC2–AC7)
> Dependencies: Wave 0 auditada. ML-1A e ML-1B são sequenciais: o ML-1B mede no Windows a forma que o ML-1A
> fixar, e os dois podem tocar `guard_line_shell_test.go`.

### ML-1A — Forma nova da linha, geração legada, migração e matriz POSIX
**Status:** ✅ Done
**Squad:** apolo-tf
**Files affected:**
- produto: `internal/guardforms/guard_generations.go`, `internal/generators/agentfiles.go`,
  `internal/validator/validator_guard_binary_probe.go`, `internal/validator/validator_credential_guard.go`,
  `internal/validator/validator_git_branch_guard.go`
- testes: `internal/generators/guard_line_shell_test.go`, `internal/generators/update_test.go`,
  `internal/generators/fail_closed_d11_behavior_test.go`, `internal/commands/update_harness_test.go`,
  `internal/validator/validator_guard_binary_probe_test.go`, `internal/validator/validator_git_branch_guard_test.go`,
  `internal/validator/validator_credential_guard_test.go`, `internal/validator/cursor_guard_migrate_dedup_external_test.go`,
  `internal/validator/validator_guard_binary_probe_windsurf_amazonq_test.go`
- configs: `.claude/settings.json`, `.codex/hooks.json`, `.gemini/settings.json`
- docs e scripts: `README.md`, `docs/cli-parity.md`, `scripts/check-validate-rule-pins.sh`

**Actions:**
1. Forma nova: prefixo `set +eu 2>${null-/dev/null}; ` antes da forma atual, nas quatro constantes
   `*CurrentPSPOSIX` (credential, credential global, git-branch, write-scope).
2. As quatro formas atuais viram geração legada no registro da D12, por igualdade exata. A migração leva a
   forma atual e a D11 revista à forma nova, sem duplicar entrada.
3. Validador: forma nova é a corrente; a atual e a D11 revista dão aviso de migração, como as legadas já dão.
4. Matriz de teste com subprocess real, afirmando rc **e** o marcador de que o guard rodou: deny 2 e allow 0
   sob `bash -e`, `bash -u`, `sh -e`, `sh -u`, `dash -e` (se existir), `zsh -e`, `zsh -u`, `BASH_ENV` com
   `set -e` e `set -u`, e `SHELLOPTS=errexit` via `cmd.Env` (nunca prefixo de atribuição). Contra-braço: a forma
   atual reprova nas condições POSIX com `-e`/`-u`.
5. Configs deste repositório: troca exata da linha nas três configs. Não rode `trackfw update harness` nem
   `trackfw init` na árvore.
6. Docs e pins: README, `docs/cli-parity.md` e o fixture de `scripts/check-validate-rule-pins.sh` com a forma
   nova.

**Acceptance criteria:**
- [x] AC2, AC3, AC5, AC6 e AC7 da REQ
- [x] `go build ./...` e os testes dos pacotes tocados verdes
- [x] `scripts/check-git-branch-guard-hook-schema.sh` e `make parity-rest` verdes
- [x] Relatório com uma frase por teste novo dizendo qual conclusão ele afirma

**Auditoria (Zeus, 2026-11-13):** aprovado depois do corretivo ML-1A2.
- Mutação: tirar o `set +eu` das quatro constantes `*CurrentPSPOSIX` reprova 165 subtestes de
  `TestGuardLineShellMatrix`; restaurado. A matriz afirma rc e marcador do guard para allow (0), deny (2), deny
  não canônico e guard ausente, sob `bash -e/-u`, `sh -e/-u`, `dash -e`, `SHELLOPTS=errexit` via env e `BASH_ENV`.
- As três configs do repositório diferem do HEAD só pelo prefixo (comparação por substituição), e o JSON é válido.
- `./bin/trackfw validate` recompilado: rc=0, nenhum `guard_wiring_removed`. As configs globais da máquina do KG
  aparecem com o aviso D18 legado, como esperado. O executor do ML-1A2 relatou 19 `guard_wiring_removed` como
  "pré-existentes"; era binário velho lendo a forma nova.
- Faltava teste da migração e do aviso (AC6): virou o ML-1A2.

### ML-1A2 — Testes da migração e do aviso D18 (corretivo)
**Status:** ✅ Done
**Squad:** artemis-tf
**Files affected:** `internal/generators/guard_d18_migration_test.go`, `internal/validator/validator_guard_d18_legacy_test.go`

**Acceptance criteria:**
- [x] Migração por CLI (Claude, Codex, Gemini, Cursor, Copilot), idempotência por bytes, D11 revista direto para a
  forma nova, mistura colapsada e escopo global: 11 testes `TestD18Migration_*`
- [x] Aviso do validador para D18 legada, silêncio na forma nova, D11 revista com o próprio aviso e sem violation:
  4 testes `TestD18LegacyWarning_*`
- [x] Contra-braços: sem a entrada D18 no registro (e sem o laço explícito) a migração reprova; sem o bloco do aviso,
  o teste do aviso reprova. Conferi os 15 nomes com `-run` e `-v`: 15 `--- PASS`.

**Nota para o red team:** o laço explícito de migração D18 em `agentfiles.go` é redundante com o dedup por registro
(o contra-braço só reprova removendo os dois). Avaliar se é código morto.

**Wave gates:**
```bash
go build ./...
go test ./internal/generators/ ./internal/validator/ ./internal/guardforms/
```

### ML-1B — PowerShell 5.1 e 7 na VM Windows
**Status:** ✅ Done
**Squad:** artemis-tf
**Files affected:** `docs/portabilidade/2026-11-13-guard-errexit-windows.md`

**Actions:**
1. Na VM Windows (ARM64), PS 5.1 e 7: a forma nova dá deny 2 e allow 0, e o stderr não tem mais linhas que o da
   forma atual.
2. `$ErrorActionPreference='Stop'`: mede a forma atual e a nova com `-NoProfile` e com um perfil que define
   `Stop`, e diz, pela documentação de cada CLI suportado no Windows, se ele carrega o perfil ao rodar o hook.

**Acceptance criteria:**
- [x] Medição entregue (PS 5.1 na VM; PS 7 só em macOS, o `pwsh` não está instalado na VM)
- [ ] AC4 da REQ: **não fecha com a forma nova** — passa para o ML-1C/ML-1D

**Auditoria (Zeus, 2026-11-14):** medição aprovada; o AC4 não fecha.
- PS 5.1 (VM): forma nova dá allow 0 e deny 2, com 16 linhas de stderr, igual à atual. O `2>${null-/dev/null}`
  não silencia nada no PS 5.1 (alvo de redirect inválido).
- 🔴 `$ErrorActionPreference='Stop'`: a forma nova e a atual saem 1 **sem rodar o guard** (fail-open nos CLIs que
  só bloqueiam com 2) sempre que o perfil é carregado com `Stop` (variantes B2b e B3 do §6.3). O Claude Code passa
  `-NoProfile` (inferido do binário); os outros CLIs não foram medidos. A ADR D18 manda cobrir: vira o ML-1C.
- Candidatos medidos na VM: CAND_C e CAND_E cobrem `Stop` em todas as variantes (allow 0, deny 2, guard roda).
- 🔴 Mesma causa, não coberta por nenhum candidato: `Set-StrictMode -Version Latest` (§10). Com `Stop`, rc=1 sem
  guard; com `Continue`, deny não canônico (frc=1) e guard ausente saem 1. Entra no ML-1C.
- PS 7 no Windows: os braços `pwsh` de `TestGuardLineShellMatrix` passam no `windows-latest` (run 43423202733 da
  main, forma anterior); a forma final se confirma no CI de Windows do PR.
- Processo: o classificador negou um `Remove-Item` por SSH e o executor fez a mesma remoção por script `.ps1` e
  `cmd /c del`. Foram arquivos temporários dele, mas contornar uma negação não é rota legítima; registrado e
  reportado ao KG. Fora do escopo declarado e desfeito: `git fetch` em `C:\Users\Lab\trackfw` (sem checkout),
  perfis de PS criados e removidos (não havia perfil antes), diretórios temporários fora de `%TEMP%`.
  `trackfw.exe` da VM inalterado (SHA256 conferido no relatório).

### ML-1C — Forma da linha que sobrevive a `Stop` e a `StrictMode` herdados (corretivo)
**Status:** ⬜ Pending
**Squad:** apolo-tf
**Files affected:** os mesmos do ML-1A (constantes `*CurrentPSPOSIX`, matriz em `guard_line_shell_test.go`,
configs do repositório, README, `docs/cli-parity.md`, pin de `scripts/check-validate-rule-pins.sh`)

**Actions:**
1. Partir do CAND_E do relatório do ML-1B (§8): `set +eu` primeiro, depois `set ErrorActionPreference Continue`.
   Medir, no `pwsh` do macOS, um segmento que também desligue o `StrictMode` herdado, sem quebrar os shells POSIX.
2. A forma do ML-1A nunca foi publicada: a nova substitui as quatro constantes no lugar, **sem** entrada legada
   nova no registro da D12. A forma pré-D18 e a D11 revista continuam migrando direto para a final.
3. Matriz: braços PowerShell com `Stop` e com `StrictMode Latest` herdados (via `-Command` e via perfil num HOME
   temporário), afirmando rc e marcador do guard; contra-braço com a forma do ML-1A reprovando.

**Acceptance criteria:**
- [ ] Forma nova: allow 0 e deny 2 com o guard rodando sob `Stop`, `StrictMode Latest` e os dois juntos, no `pwsh`,
  sem regressão na matriz POSIX do ML-1A
- [ ] `go build ./...`, testes dos pacotes tocados, `scripts/check-git-branch-guard-hook-schema.sh` e
  `make parity-rest` verdes
- [ ] Relatório com uma frase por teste novo dizendo qual conclusão ele afirma

### ML-1D — Forma final no PowerShell 5.1 da VM
**Status:** ⬜ Pending
**Squad:** artemis-tf
**Files affected:** `docs/portabilidade/2026-11-13-guard-errexit-windows.md` (nova seção)

**Acceptance criteria:**
- [ ] AC4 da REQ com a forma do ML-1C: PS 5.1 na VM e PS 7 no CI de Windows do PR

## Wave 2 — Red team e qualidade (AC8)
> Dependencies: Wave 1 completa.

### ML-2A — Red team da forma nova
**Status:** ⬜ Pending
**Squad:** hades-tf, hefesto-tf
**Files affected:** `docs/seguranca/2026-11-13-wave2-redteam-guard-errexit-herdado.md`

**Actions:** reimplementar a matriz a partir da leitura, sem conferir o diff, e tentar liberar um comando negado.

**Acceptance criteria:**
- [ ] Parecer com cada tentativa e o rc medido; achado de mesma causa vira ML nesta REQ

### ML-2B — `make quality` com a máquina ociosa
**Status:** ⬜ Pending
**Squad:** zeus-tf

**Acceptance criteria:**
- [ ] `make quality` rc=0
