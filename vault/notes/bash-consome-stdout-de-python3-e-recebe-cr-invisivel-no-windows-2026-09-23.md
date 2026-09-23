# bash consome stdout de `python3` e recebe um `\r` invisível — no Windows

> 2026-09-23 · REQ-2026-09-23 (CRLF) · PR #414 · origem: issue #353 (consumidor externo) + achado próprio

## O sintoma não é erro — é silêncio

No Windows, `python3` em modo texto traduz `\n` → `\r\n` no stdout. Todo valor que o bash colhe
dessa saída carrega um `\r` no fim, **invisível em qualquer log**. O gate roda, não encontra nada,
e reporta sucesso.

```
$ python3 -c 'print("Makefile")' | od -c
0000000   M  a  k  e  f  i  l  e  \r  \n
```

## 🔴 O padrão que denuncia o mecanismo

O **mesmo rótulo** aparece emitido e acusado ausente no mesmo run:

```
OK   [falsify/release-tag-parity/content-from-commit-false-negative]   ← emitido
AUSENTE: release-tag-parity/content-from-commit-false-negative         ← acusado
```

Porque a comparação acontece de duas formas, e só uma quebra:

| comparação | com `\r` |
|---|---|
| `grep -qxF` (linha **exata**) | **não casa** → acusa ausente |
| `grep -qF` (substring) | casa → passa |

Se você vir uma lista onde um item está nos dois lados, **pare de procurar lógica e olhe os bytes**.

## Medições

| | |
|---|---|
| #353: caminhos recebidos / terminando em `\r` | 671 / **671** |
| #353: `[[ -f ]]` como o gate lia | **0 de 671** |
| censo de Windows, antes | 8/8 shards reprovados, `TOTAL INCOMPLETO — 2/8` |
| censo, depois | rótulos ausentes **146 → 19** |
| sítios classificados | **(a)=19 defeito · (b)=0 · (c)=11 fora** — e o gate achou o 20º |

## A correção, e as três armadilhas

Ponto único: `scripts/lib-crlf-normalize.sh` → `strip_cr() { sed $'s/\r$//'; }`

1. **`sed 's/\r$//'`, nunca `tr -d '\r'`.** O `tr` destrói CR **no meio** da linha, que pode ser
   conteúdo legítimo. Prova: `printf 'a\rb\n' | strip_cr` → `61 0d 62 0a`, CR preservado.
   A grafia `$'...'` (ANSI-C quoting) é obrigatória — o BSD `sed` não interpreta `\r`.
2. **Resolver o lib por `SCRIPT_DIR` (de `BASH_SOURCE`), não por `ROOT_DIR`.** Cinco gates têm
   `--self-test` que **reaponta `ROOT_DIR`** para uma fixture; o source quebraria só nesse modo.
3. 🔴 **Normalizar dentro da função intermediária, não no call site.** 11+ sítios capturavam via
   `check_field_json`, `normalize_barrier_json`, `target_ids_json`, `doc_check_json` — invisíveis a
   qualquer gate que procure `$(python3` no ponto de uso.

## O que NÃO funciona como correção

`export PYTHONIOENCODING=utf-8` **não resolve**: controla o *codec*, não o *newline*. Medido — um
script tinha isso e foi contado por engano como "já normaliza"; a contagem real de scripts que
normalizavam era **0**, não 1.

## Por que doeu mais que um bug de gate

O sítio da segunda ocorrência era o `run-gates-falsify-shard.sh` — o **instrumento oficial de
medição de Windows**. Ele parou de produzir número. Sem número, toda triagem do cluster de Windows
vira palpite, e este projeto já estimou 14 falhas num grupo que entregou 2.

Relacionado: `crlf-frontmatter-parser-precisava-de-normalizacao-em-7-funcoes-nao-1-2026-09-05.md`
— mesma família (CRLF), e a mesma lição de alcance: a correção num sítio nunca é a população.
