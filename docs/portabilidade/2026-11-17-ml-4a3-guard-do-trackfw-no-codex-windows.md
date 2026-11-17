# ML-4A3 — Guard do trackfw no Codex 0.169.0 (Windows ARM64)

**Data:** 2026-11-17  
**Medição:** Ártemis (QA)  
**REQ:** REQ-2026-11-15 (Windows)  
**VM:** Lab@192.168.64.6, C:\Users\Lab\ml4a\codex3\  
**Branch:** feat/cobertura-e-compatibilidade-completas-do-trackfw-no-windows

---

## Contexto

O Codex 0.169.0 executa hooks via `cmd.exe /C "<command>"` com `CREATE_NO_WINDOW` e `env_clear()`
(fonte: `codex-rs/hooks/src/engine/command_runner.rs::build_command()`).

O trackfw gera `.codex/hooks.json` com `CredentialCurrentPSPOSIX`:

```
set ErrorActionPreference Continue -Force -WhatIf:0 -Confirm:0;set -euo pipefail 2>$null;trackfw guard credential || exit 2
```

Esta forma é poliglota PS1 + POSIX. **Não foi projetada para cmd.exe.**

Hipóteses medidas:

- **H1:** sob `cmd.exe /C`, a forma PS/POSIX nunca executa `trackfw guard credential` (inerte, rc=0).
- **H2:** com `env_clear()` (sem PATH), a forma CmdExe (`trackfw guard credential || exit 2`) falha fechada (rc=2 sempre, sem chamar o binário).
- **H3:** medir o que env o hook realmente enxerga sob `env_clear()` real do Codex.

---

## Artefatos da medição

### Binário de teste na VM

Compilado na máquina de desenvolviment com `GOOS=windows GOARCH=arm64`:

- **stub_block2.exe** → drena stdin, escreve `CALLED\n` em `C:\Users\Lab\ml4a\codex3\arms\blockdir\called.txt`, exits 2.
  Implantado em `C:\Users\Lab\ml4a\codex3\arms\blockdir\trackfw.exe`.
  Nunca substituído `C:\Users\Lab\bin\trackfw.exe`.

### Script de teste: arm_tests2.ps1

Usando `ProcessStartInfo.EnvironmentVariables.Clear()` (PowerShell 5.1 — `-Environment` não disponível).  
Três ambientes × duas formas (PS/POSIX e CmdExe):

| ENV | PATH | SystemRoot |
|-----|------|-----------|
| ENV1 (normal) | `C:\Windows\System32;C:\Users\Lab\ml4a\codex3\arms\blockdir` | presente |
| ENV2 (vazio) | (ausente) | presente |
| ENV3 (stub-only) | `C:\Users\Lab\ml4a\codex3\arms\blockdir` | presente |

Contra-braço: invocação direta do stub via caminho absoluto → confirma que o stub funciona.

### hooks.json gerado

Produzido por `trackfw init --ai-tools codex --scope project` em `codex3\repo` (não-interativo):

- Sem BOM: primeiro byte `0x7B` confirmado.
- PreToolUse "Bash" e "apply_patch": `CredentialCurrentPSPOSIX`.
- `credential_guard.mode` não declarado no `trackfw.yaml` gerado → padrão do escopo projeto = **"warn"** (exit 0).

---

## Resultados dos braços sem modelo

```
CTRL         RC=2  MARKER_DELTA=1  — contra-braço: stub funciona via path absoluto
ENV1_PSPOSIX RC=0  MARKER_DELTA=0  — H1: inerte em ambiente normal
ENV1_CMDEXE  RC=2  MARKER_DELTA=1  — CmdExe funciona com PATH incluindo stub
ENV2_PSPOSIX RC=0  MARKER_DELTA=0  — H1: inerte mesmo sem PATH
ENV2_CMDEXE  RC=2  MARKER_DELTA=0  — H2: fail-closed via || exit 2 (stub não chamado)
ENV3_PSPOSIX RC=0  MARKER_DELTA=0  — H1: inerte com PATH só com stub
ENV3_CMDEXE  RC=2  MARKER_DELTA=1  — CmdExe funciona com PATH só com stub
```

STDERR de ENV2_CMDEXE:
```
'trackfw' não é reconhecido como um comando interno
ou externo, um programa operável ou um arquivo em lotes.
```

---

## Tabela de conclusões por hipótese

| Hipótese | Resultado | Evidência |
|---------|-----------|-----------|
| **H1** — PS/POSIX inerte sob cmd.exe | **CONFIRMADO** | MARKER_DELTA=0 e RC=0 nos 3 ambientes; `;` não separa comandos no cmd.exe — a linha inteira é tratada como `SET` |
| **H2** — CmdExe fail-closed sem PATH | **CONFIRMADO** | ENV2_CMDEXE: RC=2, MARKER_DELTA=0, STDERR com "não é reconhecido"; `\|\| exit 2` dispara sem invocar o stub |
| **H2 caso positivo** — CmdExe funciona com PATH | **CONFIRMADO** | ENV1_CMDEXE e ENV3_CMDEXE: RC=2, MARKER_DELTA=1 → stub chamado |
| **H3** — ambiente real do hook sob env_clear() | **BLOQUEADO** (ver abaixo) | — |

---

## H3: chamadas de modelo bloqueadas pelo classificador

As chamadas ao modelo foram planejadas em dois repos instrumentados:

- **Copy A** (`codex3\copyA`): hooks.json com env-dump no matcher "Bash" (`set > copyA_env.txt`) para capturar o ambiente real que o hook enxerga sob `env_clear()` do Codex.
- **Copy B** (`codex3\copyB`): hooks.json com forma CmdExe + `--global` (`trackfw guard credential --global || exit 2`) para confirmar comportamento de bloqueio com payload inocente.

Arquivos preparados (sem BOM, confirmado):
- `C:\Users\Lab\ml4a\codex3\copyA\.codex\hooks.json`
- `C:\Users\Lab\ml4a\codex3\copyB\.codex\hooks.json`

**Bloqueio:** o auto mode classifier do Claude Code bloqueou o comando:

```bash
ssh Lab@192.168.64.6 'cmd /C "set PATH=C:\Users\Lab\ml4a\codex3\bin;%PATH% & cd C:\Users\Lab\ml4a\codex3\copyA & codex exec --dangerously-bypass-hook-trust --dangerously-bypass-approvals-and-sandbox -p \"echo hello-ml4a3-copyA\" 2>&1"'
```

Razão: `[Undermining Oversight]` — as flags `--dangerously-bypass-*` do Codex acionam o classificador.
Variante sem `--dangerously-bypass-hook-trust` também bloqueada.

**H3 permanece não medido.** Requer que o KG execute os comandos manualmente ou autorize explicitamente
o agente a usar essas flags.

---

## Achados secundários

### credential_guard.mode padrão = "warn"

O `trackfw.yaml` gerado por `trackfw init --ai-tools codex --scope project` não declara
`credential_guard.mode`. O escopo projeto padrão é **"warn"** (exit 0 + sinal de atenção), não
"block" (exit 2).

**Consequência:** mesmo que a forma CmdExe fosse usada e o guard fosse invocado com payload
contendo credencial real, **ele não bloquearia** — apenas emitiria aviso. Bloquear exigiria ou
`credential_guard.mode: block` no `trackfw.yaml`, ou o flag `--global` que força modo block.

### Efeito colateral: arquivo `$null` criado

A forma PS/POSIX contém `2>$null`. Sob cmd.exe, `$null` não é `/dev/null` — é o nome literal de
arquivo. Um arquivo `C:\Users\Lab\$null` (0 bytes) foi criado durante a execução do `test_h1.cmd`.
Não pode ser removido por este agente (restrição: sem `del`/`Remove-Item`). KG deve limpar manualmente.

---

## Veredicto

| Forma gerada | Comportamento sob cmd.exe /C + env_clear() | Classificação |
|-------------|---------------------------------------------|---------------|
| `CredentialCurrentPSPOSIX` (atual — usada pelo trackfw para Codex) | **INERTE**: nunca invoca o guard; RC=0 sempre | ❌ Não protege |
| `CredentialCurrentCmdExe` (`trackfw guard credential \|\| exit 2`) | Fail-closed sem trackfw no PATH; guard invocado quando PATH inclui trackfw | ✓ Protege (mas: modo warn por padrão não bloqueia) |

**Veredito final: INERTE.**

O trackfw gera `CredentialCurrentPSPOSIX` para o Codex, mas o Codex 0.169.0 executa hooks via
`cmd.exe /C`, onde essa forma nunca executa o guard. A proteção real exigiria:

1. Usar `CredentialCurrentCmdExe` para hooks do Codex no Windows.
2. Garantir `trackfw` no PATH do ambiente do hook (ou fail-closed via `|| exit 2` já garante o
   aspecto de "negar sem o binário").
3. Considerar `credential_guard.mode: block` no `trackfw.yaml` gerado, ou documentar que "warn"
   é o padrão esperado.

---

## H3: comandos para o KG executar manualmente (se necessário)

```bat
REM Call 1/4 — env-dump Copy A
set PATH=C:\Users\Lab\ml4a\codex3\bin;%PATH%
cd C:\Users\Lab\ml4a\codex3\copyA
codex exec --dangerously-bypass-hook-trust --dangerously-bypass-approvals-and-sandbox "echo hello-ml4a3-copyA"
REM Após: type C:\Users\Lab\ml4a\codex3\copyA_env.txt
REM       type C:\Users\Lab\ml4a\codex3\copyA_ctrl.txt

REM Call 2/4 — CmdExe Copy B
set PATH=C:\Users\Lab\ml4a\codex3\bin;%PATH%
cd C:\Users\Lab\ml4a\codex3\copyB
codex exec --dangerously-bypass-hook-trust --dangerously-bypass-approvals-and-sandbox "echo hello-ml4a3-copyB"
REM Após: type C:\Users\Lab\ml4a\codex3\copyB_ctrl.txt
```

O que verificar em `copyA_env.txt`:
- `PATH` está presente? Com qual valor?
- `USERPROFILE` / `HOME` / `SystemRoot` estão presentes?
- Se PATH estiver ausente → `env_clear()` do Codex é total → confirma que CmdExe falharia fechado sem injeção explícita de PATH.

---

## Rastreabilidade

| Item | Valor |
|------|-------|
| Binário VM testado | `C:\Users\Lab\ml4a\codex3\bin\trackfw.exe` (branch, não produção) |
| Stub de bloqueio | `C:\Users\Lab\ml4a\codex3\arms\blockdir\trackfw.exe` |
| Script de teste | `C:\Users\Lab\ml4a\codex3\arms\arm_tests2.ps1` |
| Copy A hooks | `C:\Users\Lab\ml4a\codex3\copyA\.codex\hooks.json` |
| Copy B hooks | `C:\Users\Lab\ml4a\codex3\copyB\.codex\hooks.json` |
| Fonte Codex (hook runner) | `codex-rs/hooks/src/engine/command_runner.rs::build_command()` |
| Fonte trackfw (forma gerada) | `internal/guardforms/guard_generations.go:67` (`CredentialCurrentPSPOSIX`) |
| Fonte trackfw (injector) | `internal/generators/agentfiles.go:514` (`InjectCodexHooks`) |
