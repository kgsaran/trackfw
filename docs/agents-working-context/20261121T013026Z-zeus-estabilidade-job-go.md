# Zeus — REQ-2026-11-20: estabilidade do job go (#690), todos os ACs (PR #694)

- **Agente:** zeus-tf (trackfw_architect)
- **Branch:** fix/estabilidade-do-job-go-timeout-e-etxtbsy
- **PR:** #694 (draft), CI 24/24 verde em 32d11c35

## O que mudou neste ciclo
- **ML-1A (ares):** o `make test` define o teto por pacote, 5m para `internal/generators` e 2m para os demais (31 = 30 + 1 pacotes). O job `go` do `quality.yml` e o `release.yml` chamam `make test`.
- **ML-1B (artemis):** `writeExecutableForTest` grava os stubs sob `syscall.ForkLock.RLock()`, porque o fork toma `Lock`.
- **ML-1C (ares):** job temporário de estresse no `ubuntu-latest`, 3 rodadas, removido antes do merge.
  - `before`: `Text file busy` em 3 de 12 shards e exit -1 em 2.
  - `after` final: tudo zero.
- **ML-1D/1D2 (artemis):** o exit -1 era o `pwsh` abortando com `FileLoadException`. A causa é a escrita concorrente no `StartupProfileData-*`, em `$XDG_CACHE_HOME/powershell`.
  - O cache vazio por chamada zerou o crash, mas a partida a frio estourou o prazo.
  - A correção final aquece um modelo uma vez e entrega uma cópia por chamada.
- **ML-2A (hefesto):** a revisão aprovou. F2 e F3 foram corrigidos e F1 ficou registrado.

## Pendente
- O KG decide quando tirar o PR #694 de draft e quando mergear.
- Pós-merge: fechar a REQ e mover o roadmap para `done/`. O #690 fecha pelo `Closes #690`.

## Lições
- Uma correção de flake pode criar outro: o cache isolado tirou o crash e trouxe o timeout. Só o estresse com os dois braços mostrou isso.
- Docker Desktop com 1 CPU não reproduz corrida; a medição vai para o CI.
- Teste paralelo que lança `pwsh` precisa isolar o `XDG_CACHE_HOME`.
