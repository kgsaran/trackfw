# Zeus — D18: forma L, CI verde, PS 5.1 medido (REQ-2026-11-13, PR #687)

- **Agente:** zeus-tf (trackfw_architect)
- **Branch:** fix/linha-de-hook-do-guard-sai-127-sob-errexit-herdado
- **PR:** #687 (draft, todos os checks verdes)

## O que mudou neste ciclo
- ML-2D: pacote generators de ~182 s para ~83 s (matriz completa só em git-branch, t.Parallel com semáforo de 4 pwsh); literal PT do aviso de write-scope em inglês.
- ML-2E/2F: o contra-braço D18 reprovava no CI de Linux havia 4 runs. Lá `sh` é dash, e `dash -u` sai 2 sem rodar o guard (fail-closed). O braço agora afirma "guard não rodou" e o exit esperado por shell resolvido. A simulação sh→dash usa `symlinkOrSkip`.
- ML-2H (hades): reprovou a forma K. Sem -NonInteractive e com ConfirmPreference Low/Medium, o `set +eu` abria prompt que drenava o stdin do guard (0 bytes), e git-branch/credential liberavam.
- ML-2I (apolo): forma L = K + `set ConfirmPreference None -Force -WhatIf:0 -Confirm:0;` antes do `set +eu`. K foi substituída no lugar (nunca publicada); braço sem -NonInteractive com contra-braço K.
- ML-2G: forma L no PS 5.1 da VM, 56/56 células esperadas (via -Command e via $PROFILE real). O KG executou por ssh do próprio terminal, porque o classificador do modo automático negou a execução aos agentes duas vezes.
- ML-2B: make quality rc=0. AC4 e AC8 fechados no roadmap.
- REQ corrigida (sh -u no Linux); nota do vault estendida (formas K e L, dash -u).

## Pendente
- Tirar o PR #687 de draft é decisão do KG; o merge também é dele.
- Pós-merge: marcar os ACs da REQ, `trackfw roadmap move` para done.
- Depois do merge: abrir a REQ, o roadmap e a branch de cobertura completa no Windows a partir do rascunho em scratchpad/rascunho-req-windows.md (Wave 0 hades + artemis).
- Remover o worktree scratchpad/wt-main.
- Residual: A1 (prompt drenando stdin) não medido no PS 5.1 nem nos outros CLIs (só Claude Code foi inferido com -NonInteractive).
