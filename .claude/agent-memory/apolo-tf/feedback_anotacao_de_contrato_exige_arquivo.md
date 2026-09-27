---
name: anotacao-de-contrato-exige-arquivo
description: No cli-parity.md, gate=<caminho> tem de nomear ARQUIVO existente — diretório de fixture reprova o check-parity-contract-coverage
metadata:
  type: feedback
---

Na anotação `<!-- trackfw-contract: gate=… -->` do `docs/cli-parity.md`, cada caminho depois de
`gate=` precisa ser **arquivo** no disco. Nomear o **diretório** da fixture
(`scripts/testdata/<corpus>`) reprova com `gate nomeado não existe no disco`, mesmo com o diretório
existindo.

**Why:** o `check-parity-contract-coverage.sh` testa existência de arquivo, não de caminho. Medido em
2026-09-26 no ML-3C: a anotação apontava para o diretório do corpus e o gate reprovou; nomear
`…/census-verdicts.tsv` e `…/manifest.txt` resolveu.

**How to apply:** ao criar seção nova (`##`/`###`/`####`) no `cli-parity.md`, a anotação é
**obrigatória** na primeira linha não-vazia (seção sem anotação reprova desde o ML-3A), e todo
`gate=` cita arquivos. Rode `scripts/check-parity-contract-coverage.sh` **lendo o rc em linha
separada** antes do `make quality` — ver [[metrica-por-artefato]].
