# Zeus — Windows: Waves 1 a 3 entregues e verdes no CI (REQ-2026-11-15, PR #688)

- **Agente:** zeus-tf (trackfw_architect)
- **Branch:** feat/cobertura-e-compatibilidade-completas-do-trackfw-no-windows
- **PR:** #688 (draft, todos os checks verdes)

## O que mudou neste ciclo
- Wave 1 (ML-1A, ML-1B): 41 `TestMLScope*` e `TestBarrierReentry` T1/T2/T3/T7 rodam no Windows; T3 decodifica o JSON (nota de vault sobre `\\` no JSON). REQ passou a citar a ADR pelo caminho (gate de integridade referencial).
- Wave 2 (ML-2A, 2B, 2C): skips de shell viraram probe; perfil do PowerShell isolado por USERPROFILE roda no 5.1 e no 7 (76 PASS, 0 SKIP) com contra-braço K; linha cmd.exe sob cmd.exe real (16 PASS); 7 sítios convertiam erro de execução em exit 0 (symlink `sh` sem `.exe` e DLL do MSYS fora do PATH; nota de vault).
- Wave 3: gate AST `internal/winskips` + `.github/windows-skips.json` (22 entradas, todas (c); corretivo de polaridade ML-3A2; skip de chmod 444 era falso); smoke npm/PyPI em pwsh e cmd verde em 2 rodadas; job `windows-arm64` nativo verde; regra `windows_sh_missing` no validate/doctor, revisada pelo hefesto (reprovou com F2/F3, corrigido no ML-3D2; ML-3D3 corrigiu a ordem do cleanup no Windows).
- Medido: o pacote generators leva ~119 s localmente também na origin/main (limite do make test é 120 s). Fora do escopo da REQ (tooling de contribuidor); observação para o KG.

## Pendente
- Wave 4 (ML-4A) depende do KG: VM ligada, lista de CLIs instalados e autorização para a artemis rodar a medição (`-NonInteractive`/`-NoProfile`/HOME por CLI e o hook do husky sem `sh` no PATH).
- ML-3A2: uma evidência PENDING no registro (socket em arquivo `!windows`), a fechar antes da Wave 5.
- Wave 5: red team (hades) e `make quality` + CI.
- Pós-merge (KG): incluir `windows-package-smoke` e `windows-arm64` na proteção da `main`.
