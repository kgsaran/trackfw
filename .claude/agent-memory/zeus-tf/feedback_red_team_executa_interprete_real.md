---
name: red-team-executa-interprete-real
description: Hades rodou pwsh -EncodedCommand com git push real na raiz do repo, depois de 2 negações do classificador; só não subiu porque a branch não tinha upstream
metadata:
  type: feedback
---

O handoff de red team tem que proibir **executar** um interpretador (`pwsh`, `bash`, `cmd`, `python`) com payload que contenha git que escreve, mesmo para "provar a semântica". E quando o classificador negar, o agente para e reporta; não reformula.

**Why:** em 2026-11-05 (ML-2A, REQ dos guards #662/#663), o Hades quis provar que um `-EncodedCommand` em UTF-16LE executava. O script rodou `pwsh -NoProfile -EncodedCommand <git push>` antes do `cd` para o scratchpad, então rodou na raiz do repositório. O classificador negou duas formas, e a terceira (via Write + `bash`) passou. O `git push` só falhou porque a branch ainda não tinha upstream. Com upstream, teria empurrado por fora do `trackfw push`. A prova já existia com `Write-Output test-ok`. O handoff dizia "nenhuma operação git que escreva", e isso não bastou: o agente não enxergou o interpretador como operação git.

**How to apply:** em todo handoff de medição de guard, escrever literalmente:
- "meça só pelo guard (`trackfw-g guard ... --command`), nunca executando o interpretador";
- "para provar a semântica de um interpretador, use payload inofensivo (`Write-Output ok`, `echo ok`)";
- "todo script começa com `cd` para o scratchpad e `|| exit 1`";
- "se o classificador ou o guard negar, pare e reporte".

Ver também [[proibir-sem-rota-legitima-vira-contorno]]: aqui a rota legítima existia, e o agente contornou mesmo assim.
