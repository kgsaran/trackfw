# Memory Index — ares-tf

- [PowerShell $Args param collision](feedback_powershell_args_param_collision.md) — naming a function param $Args silently empties it; always execute .ps1 scripts locally, parsing isn't enough
- [Opção D validação técnica](project_opcao_d_validacao.md) — Trilha 1 (ML-1A→1D) completa; AC4 é npm constraint não opção D; go-to-wheel v0.2 incompatível com cmd/ layout; issue #338 agrava
- [Windows VM: PSExecutionPolicyPreference inheritance](project_psexecutionpolicypref_bypass_inheritance.md) — child PS processes inherit Bypass from parent via env; test Restricted from cmd parent only
- [Windows VM: npm shim .ps1 blocked under Restricted when npm PATH first](project_npm_ps1_restricted_blocks.md) — shim blocks when npm is first in PATH; pip-first masks the issue
