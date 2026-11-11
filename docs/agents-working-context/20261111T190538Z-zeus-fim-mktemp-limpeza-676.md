# Zeus: fim da REQ do #676 (mktemp sem limpeza garantida no mesmo escopo)

- **Branch:** `fix/mktemp-sem-limpeza-garantida-no-mesmo-escopo`. Waves 0, 1 e 2 entregues. Os três barriers passam (`--trust-local-gates`); `make quality` rc=0, rodado pelo arquiteto depois do último ML.
- **Wave 0 (ML-0A):** o inventário do hades bate com o relato (68 atribuídos, 11 não cobertos). A forma D3 proposta para `shim-diff:158` estava errada: `run_classify` roda em processo próprio. Corrigida na auditoria.
- **Wave 1:**
  - ML-1A: os 11 sítios corrigidos, medidos com falha forçada antes e depois.
  - ML-1B: gate `scripts/check-mktemp-cleanup.sh` no `parity-rest`. Nas versões anteriores ao ML-1A, reprova exatamente os 11 sítios.
  - ML-1C: nota de correção no AC1 da REQ-2026-11-07.
- **Wave 2:**
  - Red team (ML-2A): aprova com ressalvas. Os achados entraram como ML-2B e ML-2C nesta REQ, contra a sugestão de REQ nova (Regra Dura).
  - ML-2B: ordem do apelido checada em todo escopo; exceção de `exit` removida; self-test do marker-parity movido para uma função.
  - ML-2C: apelido de escrita única. O gate termina com 32/32 braços.
- **Achados de processo:**
  - `mktemp` sem template ignora `TMPDIR` no macOS (nota no vault).
  - Duas vezes, o executor encerrou deixando `make` em background. Antes do próximo passo, confira com `pgrep -x make`.
- **Pendente:**
  - PR com `Closes #676`, quando o KG pedir.
  - Depois do merge: REQ em Done e roadmap em `done/`.
  - O sítio `release-tag:330` só executa no Windows; quem prova é o CI.
