# Zeus — REQ-2026-11-17: `--scope global` não toca o projeto; todos os ACs (PR #697)

- **Agente:** zeus-tf (trackfw_architect)
- **Branch:** fix/scope-global-nao-escreve-no-projeto
- **PR:** #697 (draft), CI 24/24 verde em 951bed43

## O que mudou neste ciclo
- **ADR-2026-11-21, D1–D5.** Com `--scope global`, `agents install` e `skills install` não escrevem regras nem hooks sob a raiz do projeto. O comando imprime no stderr uma dica sem o cwd, e o aviso D8 do Antigravity só sai com `--scope project`.
- **Código (ML-1A, ML-2C, apolo-tf):**
  - Gate `opts.scope == "project"` em `internal/commands/integrations_flags.go`.
  - As saídas de `executeIntegrationMutation` passaram para `cmd.ErrOrStderr()`.
  - 10 testes em `integrations_scope_project_writes_test.go`. Com o gate revertido, os braços globais reprovam.
- **Revisões:** hefesto APROVA (8 NB, corrigidos no ML-2C); red team hades APROVA com ressalvas. A R1 (provenance do `third-party install`) fica fora do D1.
- **Gates:** `make quality` MQ_RC=0 com a máquina ociosa (`internal/commands` 65,1 s); barriers das Waves 0–2 verdes; CI do PR verde.

## Pendente
- O KG decide quando tirar o PR #697 de draft e quando mergear.
- Pós-merge: REQ para Done, roadmap para `done/`, e a entrada *Changed* no changelog da próxima versão.

## Lições
- O hades escreveu um `zz_*_test.go` na árvore sem rota autorizada no handoff. Removido; na Wave 2 a rota `zz_redteam_*` foi dada explicitamente.
- O primeiro `make quality` deu timeout em `internal/commands` com carga ~20 (VM UTM + Spotlight). Ocioso, passou. O teto não mudou.
