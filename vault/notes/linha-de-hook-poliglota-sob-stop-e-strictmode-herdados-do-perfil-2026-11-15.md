---
title: "Linha de hook poliglota sob Stop e StrictMode herdados do perfil do PowerShell"
date: 2026-11-15
tags: [guard, windows, powershell, hooks, stderr]
---

# Por que a linha de hook do guard tem `null=/dev/null` e `2>$null`

REQ-2026-11-13 (D18). A medição saiu do ML-1B (VM, PS 5.1) e dos ML-1C/1C2 (pwsh 7.6.9 no macOS).

## 1. O perfil do usuário chega ao hook e derruba a linha antes do guard

Quando o CLI roda o hook **sem** `-NoProfile`, o perfil é carregado. Um perfil com
`$ErrorActionPreference='Stop'` faz o primeiro segmento que dá erro (por exemplo, `emulate`, que não existe no
PowerShell) **terminar o script com 1, sem rodar o guard**. Os CLIs que só bloqueiam com 2 deixam passar
(fail-open).
- O Claude Code passa `-NoProfile`, conforme inferido do binário.
- Os outros CLIs não foram medidos.
- Medido no relatório `docs/portabilidade/2026-11-13-guard-errexit-windows.md`, §6.3: variantes B2b e B3.

**Remédio:** `set ErrorActionPreference Continue` logo no início.
- No PowerShell, `set` é alias de `Set-Variable`.
- No POSIX, isso só define os parâmetros posicionais `$1` e `$2`, com rc=0.

## 2. `${null-/dev/null}` é variável indefinida para o PowerShell

No PowerShell, `${null-/dev/null}` é o nome literal de uma variável que não existe.
- Com `Set-StrictMode -Version Latest`, acessar essa variável é erro, e o statement inteiro é pulado. O alvo do
  redirect é avaliado **antes** do comando.
- Assim, o preset `$LASTEXITCODE=2` e a normalização 1→2 deixam de rodar.
- Se o guard não está instalado, `exit $LASTEXITCODE` cai em variável indefinida e sai 1.
- Sem StrictMode, o PS 5.1 também não silencia nada por esse alvo: o redirect é inválido.

## 3. `Set-StrictMode -Off` sem redirect polui todo shell POSIX

A primeira correção (CAND_F) pôs `Set-StrictMode -Off;` puro. Os códigos de saída ficaram certos, mas sh, bash,
zsh e dash passaram a imprimir `Set-StrictMode: command not found` em **toda** chamada do hook. Num bloqueio, essa
linha chega ao modelo.

**Forma adotada (H):** primeiro `null=/dev/null`, depois `2>$null` em todos os redirects.
- No POSIX, a variável existe, e o redirect vai para `/dev/null`.
- No PowerShell, `$null` é automática e existe mesmo sob StrictMode.
- O custo: o PowerShell imprime 2 linhas a mais, porque `null=/dev/null` não é comando lá. O KG aceitou esse
  custo no AC4.
- O teste `TestGuardLineShellMatrix_PreambleNoStderr` afirma que o POSIX não ganha nenhuma linha de stderr.

## Armadilhas de medição desta rodada

- **`PATH=/usr/bin:/bin` esconde o `pwsh`** do Homebrew, e o resultado é 127 em todas as células. Chame
  `/opt/homebrew/bin/pwsh` pelo caminho absoluto.
- **Um stub que lê `cat -` pendura o `pwsh` indefinidamente** quando não recebe `</dev/null`. Use
  `</dev/null` e `timeout`.
- **No macOS, o `/bin/sh` imprime `echo -n` literalmente.** Em stubs de teste, use `printf '%s'`.
- **`PSExecutionPolicyPreference=Bypass` herdado falseia o teste de perfil.** Remova essa variável do ambiente
  do processo filho.
