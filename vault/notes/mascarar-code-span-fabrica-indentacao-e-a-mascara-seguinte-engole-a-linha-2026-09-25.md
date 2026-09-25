# Mascarar code span FABRICA indentação, e a máscara de bloco indentado engole a linha inteira

> **Data:** 2026-09-25 · **Descoberto em:** `ML-N2` da
> `ROADMAP-2026-09-10-gate-de-palavra-chave-de-fechamento-nao-reavalia-em-edited-e-le-exemplo-citado-como-diretiva`
> · **Arquivo:** `scripts/check-pr-closing-keyword.sh` (matcher embutido)
> **Classe:** cadeia de máscaras — um passe produz o sinal que o passe seguinte consome

## O sintoma, que parecia contabilidade

O `ML-N1` declarou uma **dívida de largura**: sua máscara de bloco indentado apagava **304** linhas do
corpus de 358 PRs mergeados, contra as **80** do censo do parecer de segurança. Ele mediu que **0** das
304 carregava declaração portuguesa ou keyword inglesa com `#N`, e registrou honestamente que **não
soube reconstruir a regra do censo**.

Lido como divergência de contagem, o item parece inerte. **Não é.** As 304 não eram uma regra mais
larga: eram **indentação que o gate fabricou sozinho**.

## A causa

O passe de zonas de código roda em três etapas, nesta ordem:

```
FENCE_RE  -> SPAN_RE  -> mask_indented_code
```

`_blank()` substitui o trecho casado por **espaços**, preservando as quebras de linha (para o número de
linha reportado continuar batendo). Então uma linha que **começa** com code span:

```
`trackfw validate` — Fecha #246.
```

vira, depois do `SPAN_RE.sub`:

```
                  — Fecha #246.
```

— 18 espaços à esquerda. `INDENT_RE = ^(?: {4,}|\t)` casa. Se a linha anterior estiver em branco, o
bloco **abre**, e `mask_indented_code` apaga a linha inteira, **incluindo a declaração portuguesa**.

🔴 **Medido antes da correção, pelo caminho real do gate:**

```
$ printf 'Texto.\n\n`trackfw validate` — Fecha #246.\n' > /tmp/c.md
$ PR_BODY_FILE=/tmp/c.md bash scripts/check-pr-closing-keyword.sh ; echo $?
OK   [pr-closing-keyword]: nenhuma palavra-chave de fechamento em portugues...
0
```

**Falso negativo vivo** — o gate calando sobre exatamente o defeito que ele existe para pegar. O "0 das
304 carrega declaração" do `ML-N1` era propriedade do **corpus**, não da **regra**, a mesma distinção
que a §3.4 do parecer já faz sobre o "0 falso positivo".

## A reconciliação numérica, fechada

| medida | valor | o que é |
|---|---|---|
| censo do parecer | **80** | linhas indentadas no corpo **cru**, contagem linha a linha |
| dessas, dentro de cerca | 54 | já apagadas pelo `FENCE_RE`; o `INDENT` não as vê |
| dessas, fora de cerca | 26 | — |
| dessas, **abrindo bloco** (linha anterior em branco) | **0** | logo o `INDENT` apaga **0** linhas reais neste corpus |
| medida do `ML-N1` | **304** | **100% indentação fabricada** pelo `SPAN_RE` |

As duas contagens nunca foram da mesma coisa. Não havia regra "mais larga" a descobrir.

## A correção

Decidir a indentação no texto **como o autor escreveu**, e apagar no texto já mascarado:

```python
def mask_indented_code(text, ref=None): ...   # decide por ref, apaga em text
def mask_code_zones(text):
    masked = SPAN_RE.sub(_blank, FENCE_RE.sub(_blank, text))
    return mask_indented_code(masked, ref=text)
```

As duas árvores têm o mesmo número de linhas porque `_blank` preserva as quebras; quando não tiverem,
o fallback é medir no próprio `text` (comportamento antigo), nunca `IndexError`.

Depois da correção o mesmo corpo sai **rc=1**, e o corpus de 358 PRs fica **idêntico PR a PR**
(`rc=0 353 · rc=1 4 · rc=2 1`, acusados #247/#312/#325/#330) — medido também em ablação separada, para
não confundir este efeito com o das zonas de não-código entregues no mesmo ML.

## A regra geral, que vale além deste gate

🔴 **Quando um passe de mascaramento substitui conteúdo por espaços, ele cria sinal de LAYOUT onde não
havia.** Todo passe posterior que leia **posição** (indentação, coluna, início de linha, alinhamento de
tabela) passa a ler um artefato do passe anterior, não o texto do autor.

Duas consequências práticas:

1. **Passe que lê layout tem de ler o texto ORIGINAL**, mesmo que escreva no texto derivado.
2. **Divergência de contagem entre dois censos da mesma zona é hipótese de defeito, não de
   contabilidade** — até alguém decompor os dois números. Aqui a decomposição (80 = 54 + 26 + 0)
   fechou exatamente, e foi ela que expôs o falso negativo.

A ordem alternativa (rodar o `INDENT` **antes** do `FENCE`) foi considerada e **recusada**: uma cerca
indentada por 4+ espaços teria seus marcadores ```` ``` ```` apagados pelo passe de indentação, e o
`FENCE_RE` deixaria de casar — trocaria um defeito por outro, menos visível.
