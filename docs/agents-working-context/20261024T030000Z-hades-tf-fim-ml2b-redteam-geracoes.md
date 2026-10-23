# hades-tf — ML-2B Red-team da remoção (FIM)

**Timestamp:** 2026-10-24T03:00:00Z
**Agente:** hades-tf
**ML:** ML-2B (red-team da remoção)
**Roadmap:** ROADMAP-2026-10-23-update-nao-remove-geracoes-antigas-do-credential-guard-em-pares-evento-matcher-e-o-validate-manda-rodar-update-que-nao-resolve.md
**Branch:** fix/update-nao-remove-geracoes-antigas-do-credential-guard
**Dir:** /private/tmp/trackfw-toml-escape

## Resultado

APROVA COM BLOQUEANTE.

- 14 vetores PASS, 1 FAIL (A10).
- Entregável: docs/seguranca/2026-10-23-redteam-geracoes-do-guard.md
- git status --short internal/: vazio (teste descartável criado e removido).

## Achados

**B1 (BLOQUEANTE):** agentfiles.go:1149 — `if !globalCredentialGuardInstalledCopilot()` envolve todos os três pares de Copilot (bash, view, create|edit). Global cobre apenas bash. Com global instalado, view e create|edit ficam sem guard. Violação da D14 revisada / AC6. Requer correção pelo apolo-tf.

**R1–R4:** residuais documentados ou pré-existentes — aceitos.

## Próximo passo

Apolo-tf deve corrigir agentfiles.go:1149 (mover pares view e create|edit para fora do if).
