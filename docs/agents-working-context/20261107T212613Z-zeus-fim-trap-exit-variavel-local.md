# Zeus — fim da REQ do trap EXIT de self-test que cita variável local (#671)

- **PR #674 mergeado.** Waves 0, 1 e 2 concluídas; `trackfw barrier` passou nas três; todos os checks verdes no CI, inclusive `windows-full-suites`.
- **Entregue:**
  - ML-0A (hades-tf): threat model; achou o terceiro sítio (`check-orphan-gates.sh`) e o trap sobreposto em `check-validate-rule-pins.sh` (R6);
  - ML-1A (ares-tf): trap com valor capturado na instalação (`printf -v q '%q'`) nos três self-tests; trap único em `check-validate-rule-pins.sh`; gate `scripts/check-no-trap-local-var.sh` em `parity-rest`;
  - ML-2A (hades-tf): red team aprova com ressalvas, dois falsos negativos do gate;
  - ML-2B (ares-tf): o gate acusa `trap -- '...'` e função de uma linha; self-test com 26 casos.
- **Medido:** shim com `false` após o trap no bash 5 vazava 1 diretório e imprimia "unbound" antes; depois, nada. Gate contra as versões antigas: 5 achados; corpus atual: rc=0.
- **Incidentes:**
  - a varredura inicial do arquiteto truncou o grep com `head` e perdeu `check-orphan-gates.sh`; o Wave 0 pegou;
  - o ares-tf afirmou que o gate exigia registro em workflows e pins (falso; o irmão vive só no Makefile) e fez o contra-braço em 2 de 3 sítios; o arquiteto refez os 3.
- **Pendente:** nada desta REQ. Residuais R2–R4 declarados no cabeçalho do gate.
