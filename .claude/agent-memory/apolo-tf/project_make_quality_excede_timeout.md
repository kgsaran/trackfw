---
name: make-quality-excede-timeout-da-tool
description: make quality no trackfw leva ~13-15 min e estoura o timeout máximo de 600s da tool Bash; rodar em background e capturar o exit sem pipe
metadata:
  type: project
---

`make quality` neste repositório leva **mais de 10 minutos** (181 cenários de falsificação + suítes
dos 3 CLIs) e **estoura o timeout máximo de 600 s** da tool Bash, sendo movido para background.

**Why:** medido em 2026-09-02 (ML-1A do roadmap do `context` do CLI Node). O comando foi para
background e só terminou depois; o exit code real só ficou legível porque a saída do `echo $?` foi
para o arquivo de saída da tarefa em background.

**How to apply:** lançar `make quality > /tmp/quality.log 2>&1; echo "QUALITY_EXIT=$?"` **uma única
vez, no fim**, com `run_in_background: true`, e ler o `QUALITY_EXIT=` no arquivo de saída da tarefa —
nunca com pipe (`| tail` reporta o exit do `tail`). Para escritas só de markdown feitas **depois**,
o precedente do repo é re-executar apenas os gates que leem docs
(`scripts/check-referential-integrity.sh`, `scripts/check-parity-contract-coverage.sh`) e o
`./bin/trackfw validate`, em vez de repetir os 13 minutos.
