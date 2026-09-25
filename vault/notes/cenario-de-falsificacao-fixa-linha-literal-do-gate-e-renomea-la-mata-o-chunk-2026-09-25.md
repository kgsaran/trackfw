# Cenário de falsificação fixa uma LINHA LITERAL do gate — renomeá-la mata o chunk inteiro

**Data:** 2026-09-25 | **REQ:** gate de palavra-chave de fechamento | **ML:** ML-N1

## O defeito

`check-gates-falsify.sh` sabota gates por **substituição textual de uma linha de código-fonte**. O
cenário **s182** prova que a isenção do `check-pr-closing-keyword.sh` é **por número** e não global,
assim:

```bash
# scripts/check-gates-falsify.sh:7014
sed 's/if num not in english:/if not english:/' "$S182_REAL" >"$S182_SAB"
```

🔴 **A string `if num not in english:` é um CONTRATO entre dois arquivos**, e nada no gate dizia
isso. No `ML-N1` eu reescrevi aquele laço para a forma idiomática — `if num in english: continue`,
para reduzir o aninhamento — e o `sed` deixou de casar.

## Por que custa caro, e não é só um teste vermelho

O cenário **não passou verde**: ele reprovou dizendo exatamente o que aconteceu —

```
FAIL [falsify/setup-s182]: sabotagem nao alterou nada -- a linha 'if num not in
english:' sumiu do gate (renomeada?). Este cenario parou de medir o que promete.
```

Isso é a guarda funcionando como deveria (fail-closed, e nomeando a causa). **O custo está no
efeito colateral:** sob `set -euo pipefail` o `chunk_2` **morre no meio**, e o driver reporta mais
**10 rótulos "esperado AUSENTE"** que **não têm defeito nenhum** — `ci-workflow/*`,
`pr-closing-keyword/vacuidade-*`, `pr-closing-keyword/prosa-nao-reprova`...

> 🔴 **A leitura errada aqui é cara:** parece que você quebrou 11 coisas. Quebrou **uma**, e as
> outras 10 são cobertura perdida porque o chunk parou. **Leia o primeiro `FAIL` do chunk, não a
> lista de ausentes** — é a mesma armadilha de leitura da nota
> `o-epilogo-no-meio-do-arquivo-apaga-o-ultimo-cenario-em-enumerate-e-o-contador-congela-2026-09-24`.

E o ciclo de diagnóstico é caro por si: `make quality` leva **~13 min**, então cada hipótese errada
custa um ciclo inteiro.

## Como detectar ANTES de rodar `make quality`

Antes de renomear/reescrever qualquer linha de um `scripts/check-*.sh`, pergunte se alguém a fixa:

```bash
grep -rn "sed .*<trecho-da-linha>" scripts/check-gates-falsify.sh
# ou, mais amplo, a partir do nome do gate:
grep -n "s182\|check-pr-closing-keyword" scripts/check-gates-falsify.sh
```

Para este gate, `if num not in english:` era o **único** literal fixado — verificado com busca
repo-wide por `blank_code|PT_RE|PT_KEYWORDS|PT_FILLER`, que não devolveu nada fora do próprio gate.
Refatorações **internas** (novas funções, novos regex, mascaramento em dois passes) são livres; **a
linha fixada não é**.

## A correção aplicada, e por que foi esta

Restaurei a forma original (`if num not in english:`, com o corpo aninhado) em vez de atualizar o
`sed` do cenário. Motivo: a sabotagem precisa **representar a regressão** que ela afirma testar
(isenção global em vez de por número). Trocar o literal do cenário para casar com o código novo é
fácil demais e migra a decisão para o lado errado — o cenário passa a perseguir o código em vez de
prendê-lo. E o gate agora **declara a dependência no ponto exato**:

```python
# 🔴 NAO reescrever esta condicao como `if num in english: continue`.
# O cenario s182 do check-gates-falsify.sh sabota EXATAMENTE esta linha ...
```

## Regra geral

**Sabotagem por `sed` cria acoplamento invisível entre o gate e o falsificador.** Sempre que um
cenário fixar um literal do código sob teste, esse literal precisa de um **comentário no código
sabotado** apontando o cenário — senão a próxima refatoração legítima o quebra, e quem paga é um
agente que não sabe que o contrato existe. O gate já tinha um precedente disso (o comentário sobre
`s182` em `check-pr-closing-keyword.sh`, sobre a escrita do número do PR em arquivo); faltava no
laço da isenção.

## Ver também

- `o-epilogo-no-meio-do-arquivo-apaga-o-ultimo-cenario-em-enumerate-e-o-contador-congela-2026-09-24.md`
  — mesma classe de leitura enganosa ("rótulo ausente" ≠ "cenário quebrado").
- `comentario-de-cenario-sobre-bloco-so-de-funcoes-vira-fronteira-de-corte-falsa-2026-09-07.md`
  — chunk morto por `set -euo pipefail` e a guarda de conjunto que o detecta.
