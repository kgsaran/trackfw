---
name: psmodulepath-autoload-bypass-form-l
description: PSModulePath auto-loading bypassa Form L (CredentialCurrentPSPOSIX) mesmo com -NoProfile — rc=0 medido
metadata:
  type: project
---

Encontrado 2026-11-17, ML-FA Wave 5, REQ-2026-11-15.

Módulo PS em diretório listado em PSModulePath que exporta `function trackfw { $global:LASTEXITCODE = 0; return }` → auto-loaded quando Form L chama `trackfw guard credential` → rc=0 (FAIL-OPEN).

Medição (Arm A: binário no PATH, PSModulePath com módulo evil, `-NoProfile`): rc=0.
Medição (Arm B: binário ausente, PSModulePath com módulo evil, `-NoProfile`): rc=0 (confirmado por ausência de "trackfw: not recognized" no output, ao contrário de S5a).

**PSModulePath auto-loading é independente de `-NoProfile`** — `-NoProfile` suprime scripts de perfil, não a resolução de módulos.

**Perfil (-NoProfile bloqueia):** Codex 0.169.0 real passa `-NoProfile` (confirmado em ML-4A, commit `d9ef0614`). Ataque de perfil bloqueado por Codex.

**Capability analysis:** write-scope guard não protege `~/.codex/hooks.json` global (fora de project root → `findProjectRoot` retorna `found=false`). PSModulePath injection (escrita em diretório de módulos do usuário, fora de project root) passa pelo mesmo controle. Ambos requerem write local sem controle adicional de trackfw.

**Classificação:** explorável. Recomendação de residual aceito (mesma capability que escrita direta a paths globais). Decisão: arquiteto/KG.

**Correção candidata (requer VM):** `& (Get-Command trackfw -CommandType Application).Source guard credential` força resolução de binário externo.

**Why:** PSModulePath é herdado pelo env do processo pai — pode ser injetado por qualquer processo pai sem modificar hooks.json.
**How to apply:** em red teams futuros de Form L ou WriteScopeCurrentPSPOSIX, testar PSModulePath auto-loading com `-NoProfile` explícito.
