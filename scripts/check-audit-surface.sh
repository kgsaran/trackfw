#!/usr/bin/env bash
# check-audit-surface.sh — gate para `trackfw audit-surface` (Wave 2 / ML-2A).
#
# Este é um stub criado no ML-1A para satisfazer a verificação de existência de gate
# do check-parity-contract-coverage.sh. A implementação completa dos cenários
# FN-1..5 e FP-1..2 será feita no Wave 2 / ML-2A.
#
# Cenários previstos (a implementar no ML-2A):
#   FN-1: hook wiring alterado → deve ser reportado
#   FN-2: só o script muda (wiring inalterado) → digest diferente deve ser reportado (AC2)
#   FN-3: path de runtime ausente → novo hook de runtime adicionado é reportado (AC13)
#   FN-4: matcher alargado → mudança de matcher reportada (AC14)
#   FN-5: arquivo de instrução modificado → reportado com rótulo "instruction" (AC15)
#   FP-1: docs/cli-parity.md mencionando paths de hook → NÃO aparece no relatório (AC16)
#   FP-2: agentfiles.go mencionando paths de hook → NÃO aparece no relatório (AC16)
#
# Paridade: Go · Node.js · Python, saída byte-idêntica, --json incluído.

set -euo pipefail
export NO_COLOR=1
export TERM=dumb

echo "check-audit-surface: stub (Wave 2 / ML-2A pendente) — saindo com 0"
exit 0
