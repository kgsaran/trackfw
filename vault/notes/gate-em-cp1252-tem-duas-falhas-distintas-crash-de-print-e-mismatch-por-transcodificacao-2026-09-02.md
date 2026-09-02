---
title: Gate sob console cp1252 falha de DUAS formas distintas — crash de print e mismatch por transcodificação; e PYTHONUTF8 não cobre nenhuma das duas
tags: [encoding, cp1252, windows, python, gates, falsificacao, gotcha]
date: 2026-09-02
related: [[cp1252-roundtrip-mascara-o-defeito-o-discriminante-e-decode-de-stdin-2026-09-02]]
---

## Contexto

ML-1B do `ROADMAP-2026-09-02-saida-nao-ascii-declara-codificacao-em-script-gerado-e-em-gate`.
Varredura empírica dos 42 gates de `make parity` sob `PYTHONIOENCODING=cp1252`, contra o baseline
sem a variável. 42/42 verdes no baseline; **3 vermelhos** sob cp1252.

## Achado 1 — são dois modos de falha, não um

O modelo mental "o gate estoura `UnicodeEncodeError`" cobre só metade dos casos medidos.

| gate | rc | mecanismo |
|---|---|---|
| `check-parity-contract-coverage.sh` | 1 | **crash**: `print()` de `→` (U+2192) lido de `docs/cli-parity.md`; `File "<stdin>", line 332` — o heredoc do próprio gate |
| `check-barrier.sh` | 1 | **mismatch silencioso**: ninguém crasha. O `python3` do gate extrai um campo do JSON do binário e o imprime; sob cp1252 o `—` (U+2014, que **é definido** em cp1252) sai como o byte `0x97`. O bash captura esse byte e compara com um literal UTF-8 (`E2 80 94`) → `failure message mismatch` |
| `check-gates-falsify.sh` | 1 | **cascata**: `FAIL [falsify/no-repo-mutation]: scripts/check-barrier.sh saiu != 0 rodando limpo` — reprova por causa do anterior, não por defeito próprio |

O segundo modo é o perigoso: o gate **reprova com uma mensagem plausível sobre o produto**
("mensagem de falha diverge"), quando a causa é a codificação do canal entre o `python3` do gate e o
`bash` que o invoca. Um caractere que o cp1252 **representa** não crasha — ele transcodifica, e
qualquer comparação byte-a-byte a jusante quebra.

## Achado 1-bis — o `check-roadmap-barrier-contract.sh` tem os DOIS sintomas no mesmo sítio

Medido (sem editar o arquivo — PR #238 aberto sobre ele): baseline `rc=0`, `PYTHONIOENCODING=cp1252`
`rc=1`, `UnicodeEncodeError: '\u2705'` (✅) em `File "<stdin>", line 7`. Esse `<stdin>` é o heredoc
aberto na linha 516; a linha 7 dele é a linha 523 do arquivo,
`print(f"{base}\t{label}\t{c['name']}\tevidence\t{e}")`, que escreve `$CORPUS_LINES_FILE` — o
arquivo cujo sha vira `CORPUS_HASH` na linha 542.

Ou seja, **crash e hash não-determinístico são o mesmo sítio de código**, não dois defeitos. Sob
cp1252 o gate morre *antes* de o hash poder divergir; num console onde todos os caracteres do corpus
fossem representáveis, o hash divergiria em silêncio. Quem for corrigir o `CORPUS_HASH` precisa
resolver os dois: forçar a codificação mata o crash mas **não** torna o hash independente do SO.

## Achado 2 — `PYTHONUTF8=1` / `-X utf8` não cobre o stdio, e o `open()` não é simulável aqui

Medido (Python 3.14.7, macOS):

```
PYTHONIOENCODING=cp1252 python3           -> locale=UTF-8  stdout=cp1252
PYTHONIOENCODING=cp1252 python3 -X utf8   -> locale=utf-8  stdout=cp1252   <- stdio NÃO muda
PYTHONIOENCODING=utf-8  python3           -> locale=UTF-8  stdout=utf-8
```

`PYTHONUTF8` move `locale.getpreferredencoding()` (que governa `open()` sem `encoding=`) e **não**
move o stdio quando `PYTHONIOENCODING` já vem do ambiente. As duas variáveis cobrem **superfícies
diferentes**, não são alternativas.

Consequência para falsificação: a superfície do `open()` **não é verificável por execução neste
projeto**. O método de simulação adotado (`PYTHONIOENCODING=cp1252`, `TestCliEmConsoleCp1252`) só
mexe no stdio, e `LC_ALL=C` no macOS/Python≥3.7 cai no UTF-8 Mode do PEP 540 — `locale` continua
`utf-8` mesmo sob `LC_ALL=C`:

```
LC_ALL=C python3 -c "import locale;print(locale.getpreferredencoding(False))"  -> utf-8
```

Ou seja: qualquer afirmação sobre `open()` sem `encoding=` num console cp1252 real é **deduzida, não
medida**. Declare como residual em vez de alegar cobertura.

## Achado 3 — o instrumento do item 4 da issue #216 mede uma réplica, não o gate

`scripts/windows-repro/python/checks.py::cmd_cp1252_print` reproduz o item 4 rodando
`python3 -c "print('→')"` **em isolamento**, explicitamente "sem invocar o wrapper .sh". Isso
mede o mecanismo, não o artefato. O gate nomeado pela issue —
`check-parity-contract-coverage.sh` — pode virar verde sem que esse instrumento mude de veredito,
porque ele nunca executou o gate. Ao fechar a camada do item 4, verificar **o que o check mede**
antes de fixar o número.

## Achado 4 — controle byte-a-byte precisa normalizar o sandbox

`check-artifact-parity.sh` e `check-gates-falsify.sh` imprimem o caminho do `mktemp -d`, então o
hash do stdout **muda entre duas execuções da mesma árvore**. Comparar hashes crus produz dois
falsos positivos de "a saída mudou". Normalizar `/private/var/folders/...` e `/tmp/...` antes de
comparar; os outros 40 gates são byte-idênticos sem normalização nenhuma.

## Regra prática

1. Ao medir um gate sob codificação hostil, classifique a falha pelo **stderr**: traceback com
   `File "<stdin>"`/`"<string>"` = python do gate; frame em `pypi/trackfw/...` = produto; **nenhum
   traceback e mesmo assim vermelho** = transcodificação silenciosa na fronteira python→bash.
2. `PYTHONIOENCODING` e `PYTHONUTF8` não se substituem. Diga qual superfície cada um cobre.
3. Separe os caracteres em dois grupos pela tabela do charset, não pela aparência:
   **definidos em cp1252** (`—` U+2014 → `0x97`, `Á` U+00C1 → `0xC1`) **não crasham** — transcodificam
   e corrompem qualquer comparação de bytes a jusante; **indefinidos** (`→` U+2192, `✅` U+2705,
   `⬜` U+2B1C, `✓` U+2713) **crasham** no `print()`. Um cenário construído com o grupo errado prova
   a coisa errada (mesma lição de
   [[cp1252-roundtrip-mascara-o-defeito-o-discriminante-e-decode-de-stdin-2026-09-02]]).
