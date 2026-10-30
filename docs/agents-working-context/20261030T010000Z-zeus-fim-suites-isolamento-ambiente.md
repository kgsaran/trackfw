# zeus — fim: suítes de teste e ambiente do desenvolvedor (#624 + REQ-2026-09-12) (2026-10-30)

- **Mudou:** `internal/commands` isola HOME/USERPROFILE no TestMain preservando GOCACHE/GOMODCACHE/GOPATH/GOENV (fecha #624); `auditsurface`, `commands`, `pathguard` e `validator` saem 2 com `ambiente incompleto: git não encontrado no PATH` quando `git --version` falha; `make quality` começa por `check-prereqs` (go, python3, yaml, git, bash 4+). Sentinelas: `TestIsolatedHomeCoversBothChannels` (commands e validator, com ClaudeConfigDir) e `TestGitGuardFailsFastWhenGitAbsent` (4 pacotes, com TMPDIR do filho vazio).
- **Medido:** PATH sem trackfw + HOME real: 15 FAIL → ok. PATH sem git: 233 `--- FAIL` → 0, uma linha por pacote. Barriers das Waves 0–4 passed; a da Wave 4 inclui `make quality` (rc=0).
- **Decidido:** falha rápida em vez de `t.Skip` (suíte verde sem testar é o sinal errado); AC1/AC2 retirados (sem Node/Python desde a v8). Red team: D1/D3/D5 corrigidos na Wave 4; D2/D4 residual declarado.
- **Pendente:** PR com `Closes #624`; após o merge, mover o roadmap para `done` e fechar a REQ (push exige roadmap em `wip/`). `VISION.md` ainda diz 11.1.0 (só doc).
