# zeus — início: suítes de teste e ambiente do desenvolvedor (#624 + REQ-2026-09-12) (2026-10-29)

- **Começou:** KG aprovou absorver o #624 no roadmap de 2026-09-12 (mesmo sintoma: `go test` depende da máquina). Roadmap reescrito e movido para `wip/`; branch `fix/suites-de-teste-ambiente-incompleto`.
- **Medido (main `d0dba293`):** PATH sem trackfw + HOME real → 15 FAIL só em `internal/commands`; HOME temporário → 0. PATH sem `git` → 233 `--- FAIL` em 4 pacotes. `CLAUDE_CONFIG_DIR` real com HOME temporário → 0.
- **Decidido:** AC1/AC2 retirados (sem suítes Node/Python desde a v8); AC3 vira falha rápida no `TestMain` em vez de `t.Skip` em 181 testes; molde do isolamento é `internal/validator/main_test.go`.
- **Pendente:** Wave 0 (hades-tf), depois Waves 1–3.
