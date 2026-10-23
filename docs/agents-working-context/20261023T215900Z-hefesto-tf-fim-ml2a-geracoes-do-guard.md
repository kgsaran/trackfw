## 2026-10-23 — hefesto-tf — FIM: ML-2A revisão de qualidade (REQ-2026-10-23, issue #599)

- Veredito: REPROVADO — 1 bloqueante (B1), 5 recomendações (R1–R5).
- B1: D12 violada em `internal/generators/agentfiles.go` linhas 808–819 — 6 chamadas a `migrateHookCommand` com literal cru `"scripts/trackfw-credential-guard.sh"` em `InjectGeminiHooks`, em vez do alias `legacyBareCredGuardCmd`. AC2 não captura porque testa padrão `legacy*Cmd*` (nome de variável), não literais. As chamadas também são redundantes: `migrateDedupGuardInNestedArray` nas linhas 847–850 já cobre o mesmo literal via `guardforms.CredentialLegacyPSPOSIX`.
- Findings em: `docs/qualidade/2026-10-23-revisao-geracoes-do-guard.md`.
- Build e testes principais: rc=0.
