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

## Segunda ocorrência — `set -euo pipefail` esconde o SEGUNDO cenário quebrado atrás do primeiro

No ML-2A do ROADMAP-2026-08-02-substituir-os-parsers-artesanais-de-config-por-
biblioteca-yaml-nos-tres-clis (a própria substituição por `yaml.v3`/`yaml`
2.x/PyYAML descrita no título deste roadmap), os Cenários 34 E 35 tinham o MESMO
sintoma: ambos corrompiam literais de um scanner linha-a-linha
(`continuesOpenList` no 34, `splitTopLevelCommas` no 35) que a própria REQ
eliminou por inteiro nos 3 CLIs — Wave 1 substituiu TODO o parsing manual por
biblioteca real, então nenhum dos dois pontos de código sobreviveu.

Rodar os 82 cenários herdados ANTES de editar (protocolo obrigatório) mostrou só
UM erro: `[s34-go] expected exactly 1 occurrence... got 0` — porque
`set -euo pipefail` faz o script abortar no PRIMEIRO `exit 1`, antes de chegar ao
Cenário 35. Depois de consertar e reverificar o 34 isoladamente (script mínimo em
scratch, sem rodar os 82 inteiros de novo), rodar a suíte COMPLETA revelou o
34 verde mas o 35 quebrado pelo MESMO motivo.

**Lição adicional**: depois de corrigir um cenário quebrado por refactor-do-alvo,
rodar a suíte INTEIRA de novo antes de declarar "os herdados estão verdes" — não
basta validar o cenário corrigido isoladamente. Um `set -euo pipefail` que aborta
no primeiro erro pode estar escondendo mais de um cenário obsoleto atrás do
primeiro; corrigir um e nunca rodar a suíte completa de novo deixa o segundo
silenciosamente quebrado até a próxima vez que alguém rodar tudo do zero.

## Terceira nuance — quando o mecanismo inteiro (não só um branch) foi eliminado, o braço de detecção perde seletividade

Nos Cenários 28/28-como-descrito-acima, o retarget preservou a MESMA
seletividade (corrompendo o ponto novo equivalente, ainda especializado no
mesmo defeito). No 34/35 isso não foi possível: `continuesOpenList` (indentação)
e `splitTopLevelCommas` (vírgula dentro de aspas) eram branches ESPECÍFICOS de
um scanner artesanal — depois que a biblioteca YAML assumiu o parsing por
inteiro, não existe mais NENHUM branch especializado nesses dois casos (a
biblioteca trata ambos de forma genérica, sem código próprio para nenhum dos
dois). O retarget, nesses dois casos, teve que subir um nível de abstração — de
"corromper o branch que trata o caso específico" para "corromper a atribuição
final do valor já parseado" (`cfg.Agents = items` / `cfg.agents = items` /
`cfg["agents"] = items`). Isso preserva a INTENÇÃO operacional do cenário ("a
fixture ainda usada continua sendo lida corretamente"), mas o braço de detecção
deixa de provar a coisa especializada original — passa a provar só "a chave é
lida", que é mais fraco, mas ainda genuíno (não vira `assert True`). Sinal para
reconhecer esse caso: o retarget do Cenário 28 trocou uma função por outra
equivalente; aqui não havia mais NENHUMA função equivalente para apontar —
sintoma de que a mudança de wave anterior era estrutural (trocar o mecanismo
inteiro), não um refactor local.

Relacionado: `vault/notes/deteccao-de-status-de-adr-divergencias-entre-clis-2026-08-01.md`.
