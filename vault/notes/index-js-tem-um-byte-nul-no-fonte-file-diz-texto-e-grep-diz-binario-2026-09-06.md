# `npm/src/validator/index.js` tem UM byte NUL no fonte — `file` diz texto, `grep` diz binário

> 2026-09-06 · `trackfw_architect` (Zeus) · medido, não herdado

## A armadilha

`grep` **sem `-a`** pula `npm/src/validator/index.js` **em silêncio** — sai 0, imprime nada, e quem
buscou conclui "não existe". **Duas REQs deste repositório nasceram com premissa falsa por isso**, e
a armadilha reapareceu em 2026-09-05 dentro de uma triagem que existia para limpar o backlog.

## A causa, localizada em bytes

```
offset 83123 de 178225 · linha 1855
const seenKey = `${m.raw}<NUL>${m.typeIsCommand}`
```

**Um único byte NUL literal no código-fonte**, usado como separador de chave num `Set` de dedup.
Um byte em cento e setenta e oito mil.

## 🔴 Por que a armadilha sobrevive: as ferramentas discordam

```
file npm/src/validator/index.js   →  "Unicode text, UTF-8 text"
grep sem -a                        →  pula, silenciosamente
grep com -a                        →  encontra
tr -d '\000' | wc -c               →  178224   (vs 178225 do arquivo)
```

**`file` diz que é texto. `grep` trata como binário.** Quem tentar "verificar se o alerta procede"
usando `file` conclui que é folclore — e cai na armadilha na busca seguinte.

## 🔴 A ironia que quase me pegou

Tentei medir o NUL com `grep -c $'\x00' npm/src/validator/index.js`. Resultado: **nada**.

Porque **o `grep` pulou o arquivo por causa exatamente do byte que eu procurava**. A ferramenta de
medição foi derrotada pelo objeto medido. Só a leitura em bytes (Python) achou.

Se eu tivesse parado no `grep`, teria "falsificado" a premissa e escrito que ela era falsa.

## O que fazer

- **Sempre `grep -a`** neste arquivo. Não é superstição: é o byte 83123.
- Para verificar a premissa, **não use `grep` nem `file`** — use leitura em bytes.
- Antes de declarar que algo não existe no Node, confirme com `-a`. **Busca vazia não é ausência.**

## Nota de método

Eu repeti "o index.js é classificado binário" em **todos** os handoffs de 2026-09-05 sem nunca ter
medido. Estava certo por herança, não por evidência — e fui verificar só quando um agente afirmou a
causa. A premissa era verdadeira; o meu conhecimento dela, não.
