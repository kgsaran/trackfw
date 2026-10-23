# hades-tf — ML-2B Red-team da remoção (INÍCIO)

**Timestamp:** 2026-10-24T02:00:00Z
**Agente:** hades-tf
**ML:** ML-2B (red-team da remoção)
**Roadmap:** ROADMAP-2026-10-23-update-nao-remove-geracoes-antigas-do-credential-guard-em-pares-evento-matcher-e-o-validate-manda-rodar-update-que-nao-resolve.md
**Branch:** fix/update-nao-remove-geracoes-antigas-do-credential-guard
**Dir:** /private/tmp/trackfw-toml-escape

## Objetivo

Barreira independente: reimplementar a regra de remoção a partir do ADR/REQ (não do diff), atacar o código real com casos que o modelo diz que devem sobreviver ou sumir, e emitir o parecer.

## Estado inicial

- Wave 0 (ML-0A + ML-0B) aprovada com auditoria do zeus-tf
- ML-1A a ML-1E implementadas pelo apolo-tf
- ML-2A (hefesto-tf) concluída antes deste ML
- Este é o ML-2B — barreira independente de segurança

## Artefatos lidos

- REQ-2026-10-23-...599.md
- ADR-2026-10-04 adendos D12/D13/D14 revisada (2026-10-23)
- Wave 0: docs/seguranca/2026-10-23-wave0-geracoes-do-guard.md (T1–T6 + ML-0B)
- internal/guardforms/guard_generations.go
- internal/generators/guard_generations.go
- internal/generators/agentfiles.go (seções InjectClaudeHooks, InjectCodexHooks, InjectGeminiHooks, InjectCursorHooks, InjectWindsurfHooks, InjectAmazonQHooks, InjectCopilotHooks, globalCredential*)
