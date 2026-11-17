---
name: sonda-no-binario-nao-leitura-de-fonte
description: Comportamento de CLI de terceiro (shell do hook, env) se mede por sonda no binário real; leitura de fonte e braço que escolhe o shell por conta própria dão veredito falso
metadata:
  type: feedback
---

Veredito sobre como um CLI de terceiro roda hooks (shell, flags, ambiente) só vale medido **dentro do próprio CLI**, com sonda.

**Why:** em 2026-11-17 (ML-4A3, Codex 0.169) a artemis leu `command_runner.rs` (`cmd.exe /C` + `env_clear()`), testou a linha do guard num `cmd.exe` escolhido por ela e declarou o guard de credencial INERTE. Três sondas como hook no Codex real, rodadas pelo KG, mostraram `powershell.exe -NoProfile -Command` com PATH herdado. O guard estava certo. Antes disso, a medição do HOME recomendou `os.UserHomeDir()` (USERPROFILE), contradizendo o próprio braço que mostrou USERPROFILE ignorado.

**How to apply:** a sonda que distingue os shells numa rodada é `echo shell=%CD% > f`, que grava texto literal no PowerShell e expandido no cmd, mais `(Get-CimInstance Win32_Process -Filter "ProcessId=$PID").CommandLine` e a do processo pai. Um "hook não dispara" só vale com um contra-braço que prove que o arquivo de hooks foi carregado (SessionStart/PreToolUse `.*`). O BOM do PowerShell invalida o JSON em silêncio. Pasta nova precisa de `--trust`. O `codex exec` com bypass é barrado pelo classificador para subagentes, então o KG roda e eu leio as sondas por SSH. [[vm-investiga-ci-mede]]
