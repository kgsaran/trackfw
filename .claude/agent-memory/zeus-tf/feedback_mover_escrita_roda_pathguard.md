---
name: feedback-mover-escrita-roda-pathguard
description: ML que move uma escrita de disco para outra função ou pacote precisa rodar go test ./internal/pathguard/, que analisa a árvore inteira
metadata:
  type: feedback
---

Quando um ML move uma escrita (`os.WriteFile`, `MkdirAll`, `Rename`, `CreateTemp`) para outra função ou pacote, a auditoria roda também `go test ./internal/pathguard/`, e não só os pacotes tocados.

**Why:** em 2026-10-22 (REQ-2026-09-02), a escrita do log saiu do `roadmap.go`/`req.go` para o `translog.WriteEntry`, e a guarda ficou nos chamadores. Os testes dos 7 pacotes tocados, o `parity-rest` e o `check-write-containment` passaram. Só o `make quality` pegou: `TestContainmentAnalyserOverTheLiveTree` exige que a guarda domine a escrita **na mesma função**, e `TestML9ASitesStayClosed` fixa a contagem de escritas por sítio. O analisador vive em `internal/pathguard/` e varre a árvore inteira, então nenhum teste "do pacote tocado" o aciona.

**How to apply:** em todo handoff que crie um pacote que escreve em disco ou mova uma escrita, ponha `go test ./internal/pathguard/` na validação e exija que a guarda esteja dentro da função que escreve. Ligado a [[feedback-paralelismo-tem-custo-de-cpu]] (teste do pacote tocado) e a [[feedback-mudar-contrato-auditar-quem-afirma-o-antigo]].
