# Cenário de falsificação quebra silenciosamente quando o código-alvo é refatorado

> Data: 2026-08-02 | Autor: Ártemis + Zeus | Domínio: gates / testes

## Sintoma

`scripts/check-gates-falsify.sh` falha com uma mensagem que **não parece** um veredito de gate:

```
[s28-python] expected exactly 1 occurrence of pattern, got 0
EXIT:1
```

Não é "o gate reprovou o código". É o **setup do cenário** que não conseguiu aplicar a corrupção.

## Causa

Cada cenário de falsificação corrompe o código-alvo substituindo um **literal de implementação**
— um trecho exato de código — e depois verifica que o defeito reaparece. A guarda
`corrupt_literal` exige exatamente 1 ocorrência do literal.

Quando alguém **refatora** o ponto corrompido, o literal deixa de existir. O cenário passa a
falhar no setup, não na verificação.

Caso real: o Cenário 28 (suporte a backtick na extração de referência) corrompia um bloco de
código em `pypi/trackfw/validator.py`. O ML-1A do ciclo de 2026-08-02 refatorou exatamente esse
bloco para `_strip_ref_delimiters` / `_REF_DELIMITERS` — correção legítima, mas que apagou o
literal que o cenário procurava.

## Por que passou despercebido por dois commits

O agente executor foi instruído a **não** rodar `make quality` (leva mais de 2 min, e a barreira
é wave posterior). E Zeus, na auditoria de cada ML, rodou `go test`, `npm test` e `pytest` —
**mas não a suíte de falsificação**. As três suítes passavam; o gate que estava vermelho era
justamente o que ninguém rodou.

**Lição de processo:** rodar as suítes de teste **não** substitui rodar o gate completo. Se o ML
mexeu em código que algum cenário de falsificação corrompe, `check-gates-falsify.sh` precisa
rodar na auditoria daquele ML — não só na barreira final.

## Como reconhecer

Mensagem de erro no formato `expected exactly N occurrences ... got 0` num cenário que **antes**
passava, logo após um refactor. Não é regressão do produto — é o cenário apontando para código
que não existe mais.

## Como corrigir

Retarget a corrupção para o novo ponto equivalente, preservando a **intenção** do cenário. No
caso do 28: passou a remover o backtick de `_REF_DELIMITERS`, que é o mesmo efeito
("suporte a backtick ausente") no código refatorado.

## Armadilha relacionada — corrupção não determinística

Ao escrever o Cenário 33, a primeira versão corrompia `sorted(os.listdir(path))` para
`os.listdir(path)` — a ordem "natural". Isso é **dependente do filesystem**: em ext4 com
`dir_index` a ordem pode sair alfabética por acaso, e o cenário ficaria **inerte** na máquina de
CI sem ninguém notar.

Trocado por `sorted(..., reverse=True)`, que é determinístico em qualquer filesystem.

**Regra:** a corrupção precisa produzir o defeito **sempre**, não "geralmente". Corrupção que
depende de ambiente é cenário vacuoso intermitente — pior que cenário ausente, porque dá
falsa confiança.

Relacionado: `vault/notes/deteccao-de-status-de-adr-divergencias-entre-clis-2026-08-01.md`.
