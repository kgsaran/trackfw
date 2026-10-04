---
title: "PowerShell -Command converte exit 2 em 1, e `read -t` do bash é janela fixa, não timer de ociosidade"
date: 2026-10-04
tags: [guard, windows, powershell, stdin, hooks]
---

# Três armadilhas medidas ao portar o guard para Go (REQ-2026-09-05)

## 1. `powershell -Command "<nativo>"` devolve 1 quando o nativo sai com 2

Medido na VM, PowerShell 5.1.26100, `%ERRORLEVEL%` lido em arquivo `.cmd` linha a linha:

```
powershell -NoProfile -Command "cmd /c exit 2"                      → 1
powershell -NoProfile -Command "cmd /c exit 2; exit $LASTEXITCODE"  → 2
```

Os CLIs de agente de PowerShell só bloqueiam com exit 2: sem o sufixo, o guard decide negar e o CLI
deixa passar. O sufixo funciona também em sh, bash e Git Bash (`$LASTEXITCODE` vazio → `exit` devolve
o status do último comando). **No `cmd.exe`, `;` não separa comandos**: o binário recebe
`["git-branch;","exit","$LASTEXITCODE"]`. Kiro e Amazon Q recebem a linha sem sufixo.

🔴 Com `Restricted` e `trackfw` resolvido para o shim `.ps1` do npm, o sufixo faz sair **0**: a
PSSecurityException não atualiza `$LASTEXITCODE`. O `validate` precisa denunciar essa combinação.

## 2. `%ERRORLEVEL%` na mesma linha mente

`cmd /c "a & echo %ERRORLEVEL%"` expande a variável **antes** de executar a linha: mediu 0 onde era 1.
Use um `.cmd` com uma linha por comando. E teste `Restricted` só com **cmd como pai**: um PowerShell
lançado com `-ExecutionPolicy Bypass` exporta `PSExecutionPolicyPreference`, e os filhos herdam.

## 3. `read -t 2 -d ''` é janela fixa

O guard `.sh` drena o stdin com `read -t 2 -d ''` em laço. Cada chamada é um **prazo fixo de 2 s a
partir do início da chamada**, que acumula tudo o que chegar. Se ela acumulou algo, abre outra janela;
se não acumulou nada, é truncado. Por isso `(payload; sleep 3)` libera (EOF na 2ª janela) e
`(payload; sleep 6)` nega em cerca de 4 s.

A primeira porta Go reiniciava o timer a cada `Read` com dados (timer de ociosidade) e negava o caso
`sleep 3`. O teste que dizia cobrir esse caso usava EOF imediato e passava. Correção: uma goroutine
leitora e um timer por janela que não reinicia (`internal/guard/payload.go`, `DrainStdin`).

Fontes: `docs/portabilidade/2026-10-04-trackfw-no-path-dos-shells-do-windows-por-canal.md` (ML-1D),
adendo da `docs/adr/ADR-2026-10-04-o-guard-de-hook-e-um-subcomando-go-do-trackfw-e-a-linha-de-hook-e-a-mesma-string-em-todo-shell.md`.
