---
title: cenário de `check-gates-falsify.sh` pina linha de fonte via sed — guard de plataforma novo na mesma linha quebra o cenário, não o código
tags: [gates, falsify, validator, windows, gotcha]
date: 2026-08-31
related: [[job-de-windows-largo-so-reproduz-2-dos-11-defeitos-2026-08-30]]
---

## O sintoma

Ao portar o #222 Grupo B (bit de execução no validator, `ROADMAP-2026-08-31-portar-as-correcoes-do-
reporter-da-issue-216`, ML-1A) — trocar `case info.Mode()&0111 == 0:` por
`case CurrentGOOS != "windows" && info.Mode()&0111 == 0:` em
`internal/validator/validator_credential_guard.go` — `make quality` reprovou em `parity` com:

```
FAIL [falsify/setup-s81]: sed não alterou validator_credential_guard.go — padrão não encontrado; prova P4 inválida
```

Nenhum teste Go quebrou. `go build`, `go test ./internal/validator/...` e a suíte inteira do Python e
Node ficaram verdes. Só `check-gates-falsify.sh` reprovou.

## A causa

O Cenário 81 de `scripts/check-gates-falsify.sh` prova que `check-validate-parity.sh` **detectaria**
uma regressão na checagem de bit de execução se ela quebrasse — falsificação P4, não teste de
comportamento. Para simular a regressão, ele faz `sed` sobre o **código-fonte real do repo**,
substituindo a cláusula `case info.Mode()&0111 == 0:` inteira por uma versão sempre-falsa, compila um
binário sabotado num diretório temporário, e prova que `check-validate-parity.sh` reprova com esse
binário.

O `sed` casava a **cláusula inteira**, `case info\.Mode()&0111 == 0:`, âncora que inclui o `case `.
Ao prefixar a condição com `CurrentGOOS != "windows" &&`, a linha deixou de casar
**byte a byte** com esse padrão — o `sed` não altera nada, `cmp -s` não vê diferença, e o cenário
recusa prosseguir (`padrão não encontrado`) em vez de silenciosamente falsificar vácuo.

**Isto não é um defeito do #222 nem do port** — é o cenário de falsificação corretamente recusando
prosseguir sem confirmar que a sabotagem teve efeito. O comportamento é o desejado; a âncora textual
é que ficou desatualizada pela mudança de linha.

## O precedente que já existia no próprio arquivo

O Cenário 179 (`ROADMAP-2026-08-28-doctor-compara-o-bit-de-execucao-dos-artefatos-de-scaffold`,
ML-2A) já resolveu exatamente este problema para `scaffold_doctor.go`: em vez de mirar a cláusula
`if execBit && CurrentGOOS != "windows" && !execBitPresent` inteira, mira só o substring `execBit &&`
— sobrevive a qualquer prefixo/sufixo que se adicione à condição depois, desde que o substring
sobreviva.

## O remédio aplicado

Retargetar o `sed` do Cenário 81 do mesmo jeito: da cláusula inteira (`case info\.Mode()&0111 == 0:`)
para o substring da checagem de modo (`info\.Mode()&0111 == 0:`), preservando a intenção de
falsificação (inverter especificamente a checagem de modo, não o guard de plataforma que a
acompanha):

```diff
-sed 's/case info\.Mode()&0111 == 0:/case false \&\& info.Mode()\&0111 == 0: \/\/ [falsified]/' \
+sed 's/info\.Mode()&0111 == 0:/false \&\& info.Mode()\&0111 == 0: \/\/ [falsified]/' \
```

Resultado após o sed: `case CurrentGOOS != "windows" && false && info.Mode()&0111 == 0: // [falsified]`
— Go continua com a checagem de modo inertizada (mesmo efeito de antes), independente de qual guard
de plataforma precede a cláusula.

## Por que isto vai se repetir

Qualquer cenário de `check-gates-falsify.sh` que faça `sed`/`corrupt_literal` ancorado numa cláusula
`case`/`if` inteira (em vez de um substring interno) quebra assim que alguém adiciona um guard de
plataforma (ou qualquer outro prefixo condicional) à mesma linha. **Antes de adicionar um guard a uma
condição já coberta por falsificação, grepar `scripts/check-gates-falsify.sh` pela expressão exata
que está sendo alterada** — não só pelo nome do arquivo. A Wave 2 (`ML-2A`, #222 Grupo A, `$HOME`)
toca os mesmos três arquivos de validator e deve repetir esta checagem antes de fechar.

## Como não cair nisto de novo

```bash
# antes de editar uma condição já falsificada, confirmar o padrão exato usado no sed/corrupt_literal
grep -n "<trecho da condição atual>" scripts/check-gates-falsify.sh
```

Se o padrão ancora a cláusula inteira, prefira mirar o **substring mínimo que não muda** — é o que
`corrupt_literal` do Cenário 179 já faz por convenção.
