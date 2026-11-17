# Zeus — Windows: Waves 4 e 5 entregues; REQ-2026-11-15 com todos os ACs (PR #688)

- **Agente:** zeus-tf (trackfw_architect)
- **Branch:** feat/cobertura-e-compatibilidade-completas-do-trackfw-no-windows
- **PR:** #688 (draft)

## O que mudou neste ciclo
- Wave 4 (VM do KG):
  - **ML-4A:** Codex 0.169.0 real executa o hook com `powershell.exe -NoProfile -Command` (5.1) e herda o PATH. Não passa `-NonInteractive`. O home vem de `CODEX_HOME` ou do perfil do SO (`USERPROFILE`), nunca de `HOME`.
  - Leitura do fonte refutada: a hipótese de `cmd.exe` com ambiente limpo não se confirmou, e o veredito "guard inerte" anterior estava errado.
  - Husky e lefthook acham `sh` pelo PATH que o Git for Windows injeta nos hooks.
  - **ML-4A2:** os artefatos de hook do git saíram da regra `windows_sh_missing`.
  - **ML-4A4/4A5:** guard global e agentes do Codex vão para onde o Codex lê (`CodexConfigDir`), em init, integrations, update, validate e aviso D5.
  - Antigravity: não se aplica, pois o trackfw não gera hook para ele. O assunto está na REQ-2026-11-17 (backlog).
- **ML-3A2:** fechado. O teste de socket roda no Windows e passou; a evidência PENDING anterior era falsa.
- Wave 5:
  - **ML-FA (hades):** reprovou.
    - Resistiram: S1 (contenção do `CODEX_HOME`), S2 (escrita × auditoria) e S3.
    - Achados: 1-A (FIFO trava `Inert*GuardPath`), 4-A1..4-A5 (formas de skip invisíveis ao gate, PENDING e classe vazia aceitos) e 5-A (sombra por `PSModulePath`).
    - 5-A foi aceito como residual pelo KG.
  - **ML-FA2:**
    - Regressão do ML-4A4 no CI: os testes do Codex isolavam só `HOME`.
    - Novo pacote folha `internal/fileutil` (O_NONBLOCK + fstat).
  - **ML-FA3:**
    - O gate `winskips` agora vê as 3 formas e valida evidência e classe.
    - Braços conferidos pelo arquiteto contra o código anterior.
  - **ML-FB:**
    - `make quality` rc=0 na 2ª rodada. A 1ª reprovou em `check-symlink-privilege-guard`; a exceção dos FIFOs `!windows` foi adicionada.
    - Quality run 44172607377 todo verde; as 5 falhas do `windows-full-suites` estão na lista conhecida.

## Pendente
- Corpo do PR #688: citar a REQ-2026-11-17 (backlog) e as Waves 4–5. Sair de draft se o KG pedir.
- Pós-merge (KG):
  - `windows-package-smoke` e `windows-arm64` na proteção da `main`;
  - fechamento da REQ e `trackfw roadmap move` para done.
- Fora do commit, sem dono identificado:
  - `AGENTS.md` e `CLAUDE.md` regenerados PT→EN em 3 linhas (mtime 16:59, antes do red team);
  - arquivo `null` vazio na raiz.
- Limpeza da VM, pelo KG: apagar `C:\Users\Lab\ml4a\` e `C:\Users\Lab\$null`, e a entrada de trust do copyA em `.codex\config.toml`.
