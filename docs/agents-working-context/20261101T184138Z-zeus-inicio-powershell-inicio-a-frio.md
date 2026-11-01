# Zeus — início do #649 (PowerShell 5.1 a frio na matriz de guard)

- Medido: `git-branch/powershell/RC0-allow` é a primeira chamada ao `powershell.exe` no processo de teste.
  - Em 18 jobs distintos, levou de 2,91 a 9,86 s nos verdes e 15,12 s na falha.
  - As chamadas seguintes levaram de 0,4 a 2,2 s.
- Decisão: aquecer cada binário PS antes da matriz e distinguir a falha por timeout da falha por asserção. Só aumentar o prazo apenas move o limiar.
- REQ e roadmap em `wip/`; branch `fix/powershell-inicio-a-frio-na-matriz-de-guard`.
- Próximo passo: Wave 0 (`hades-tf`), depois o ML-1A (`artemis-tf`).
