# 2026-10-22 — apolo-tf — INÍCIO: translog.WriteEntry ganha guard interno

- Worktree: `/private/tmp/trackfw-toml-escape`
- Branch: `fix/trackfw-log-conflita-em-toda-branch-paralela`
- Escopo: adicionar guard `pathguard.RejectAndReport` dentro de `translog.WriteEntry`, consolidar callers (req.go, roadmap.go, discover.go), corrigir gates `TestContainmentAnalyserOverTheLiveTree` e `TestML9ASitesStayClosed`
- Arquivos em escopo: `internal/translog/translog.go`, `internal/translog/translog_test.go`, `internal/generators/req.go`, `internal/generators/roadmap.go`, `internal/commands/discover.go`, `internal/pathguard/containment_live_test.go`
