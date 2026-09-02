---
title: cp1252 faz round-trip de bytes e mascara o defeito — o discriminante é o DECODE de stdin, não o encode de stdout
tags: [encoding, cp1252, windows, python, falsificacao, gotcha, auditoria]
date: 2026-09-02
related: [[ambiente-do-dev-e-mais-rico-que-o-do-ci-2026-08-29]]
---

## Sintoma

Ao falsificar o fix do `attentionSignalScript` (prefixar `PYTHONIOENCODING=utf-8` nas duas chamadas
`python3 -c`), o cenário escolhido para provar a direção "antes → quebra" **não quebra**:

```
before   PYTHONIOENCODING=cp1252   entrada UTF-8 "confirmação ✓"   →  "confirmação ✓"   ← passa!
```

O mesmo cenário, com a árvore corrigida, também devolve `"confirmação ✓"`. As duas metades da
falsificação dão o mesmo resultado — ou seja, **o teste não discrimina nada**.

## Causa raiz — e por que o diagnóstico intuitivo está errado

A leitura intuitiva é *"cp1252 não representa `✓` (U+2713), logo o `print` estoura"*. Isso é
verdade **para um literal no código**:

```
$ PYTHONIOENCODING=cp1252 python3 -c "print('confirmação ✓')"
UnicodeEncodeError: 'charmap' codec can't encode character '✓'
```

Mas no script real o texto **não é literal — vem do stdin**. E aí acontece um round-trip:

```
bytes UTF-8 de "✓"      E2 9C 93
   ↓ decode como cp1252 (todos os 3 bytes são DEFINIDOS em cp1252)
str mojibake            'â' 'œ' '“'
   ↓ encode de volta como cp1252
bytes                   E2 9C 93        ← idênticos aos de entrada
```

O arquivo final recebe os bytes originais e, lido por um consumidor UTF-8, aparece **correto**. O
defeito é invisível porque a cadeia é byte-transparente.

## O discriminante real

O que quebra é o **decode do stdin**, e só quando a sequência UTF-8 contém um byte que o cp1252
**não define**: `0x81`, `0x8D`, `0x8F`, `0x90`, `0x9D`.

`Á` = `C3 81` contém `0x81`. Medido com o script realmente gerado (`trackfw init`), forçando o ramo
sem `jq`:

| variante | `PYTHONIOENCODING` | entrada | `message` gravado |
|---|---|---|---|
| antes | cp1252 | `Área crítica` (UTF-8) | **`Agent needs attention`** ← perde |
| antes | utf-8 | idem | `Área crítica` |
| depois | cp1252 | idem | `Área crítica` ← **corrige** |
| depois | utf-8 | idem | `Área crítica` (controle: não muda) |

Ou seja: **o fix é real e correto**; só a evidência escolhida para prová-lo não era discriminante.

## Regra prática

Ao falsificar um problema de codificação, escolha o caractere pelos **bytes**, não pela aparência:

- se o texto vem de um **literal no código** → o gargalo é o *encode* de stdout; qualquer caractere
  fora do charset serve (`✓` serve);
- se o texto vem do **stdin/arquivo** → o gargalo é o *decode*; só serve um caractere cuja
  codificação UTF-8 contenha um byte **indefinido** no charset alvo (`Á`, `Í`, `Ó`… — os que contêm
  `0x81`/`0x8D`/`0x8F`/`0x90`/`0x9D`).

Um cenário que passa igual antes e depois **não é evidência a favor do fix** — é evidência de que o
teste não mede o que se pensa.

## Bônus: o residual declarado era o inverso do medido

O relatório do ML declarou como residual *"agora entrada genuinamente cp1252 falha para o fallback
em vez de imprimir algo"*. Medido:

| variante | entrada realmente cp1252 (`\xC1rea cr\xEDtica`) | resultado |
|---|---|---|
| antes | — | `tr: Illegal byte sequence` → `set -euo pipefail` mata o script → **nenhum arquivo é escrito** |
| depois | — | `json.load` falha → fallback → grava `"Agent needs attention"` |

O "antes" não imprimia nada: **não havia sinal de atenção nenhum**. O "depois" grava um sinal
genérico. É uma **melhora**, não uma regressão — o oposto do que o residual afirmava. Vale a mesma
lição: um residual também precisa ser medido, não deduzido.

## Como foi descoberto

Auditoria do ML-1A (item 4 da issue #216) pelo arquiteto. O relatório do agente afirmava a direção 1
com `confirmação ✓`; a reprodução independente, com o script gerado de verdade e o ramo `jq`
desativado, devolveu `confirmação ✓` **nas duas árvores**. A contradição levou à medição dos bytes,
que explicou as duas coisas de uma vez.

O fix foi mantido — mudou a evidência, não a conclusão.
