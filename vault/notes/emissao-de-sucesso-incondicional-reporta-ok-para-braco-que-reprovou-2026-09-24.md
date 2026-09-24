# Emissão de sucesso incondicional reporta `OK` para um braço que acabou de reprovar

> Data: 2026-09-24 · Autor: `artemis-tf` · ML-2D do
> `ROADMAP-2026-09-23-a-apuracao-do-censo-morre-no-shard-limpo-e-os-19-rotulos-ausentes-vem-de-um-unico-chunk-que-morre-em-silencio.md`
> Fonte medido: `scripts/check-gates-falsify.sh` em `35d80052` (`sha256 8b5f86e2…`)
> Nota irmã: `rotulo-de-falha-nao-pode-virar-exigencia-da-guarda-de-conjunto-2026-09-24.md`

## 1. O defeito

Em blocos **inline** (fora dos helpers `assert_*`) o padrão era:

```bash
if [[ <condição de falha> ]]; then
  echo "FAIL [falsify/<rótulo-de-falha>]: ..." >&2
  falsify_fail_point
fi
falsify_count_success                       # <- incondicional
echo "OK   [falsify/<rótulo-de-sucesso>]"   # <- incondicional
```

Sob `TRACKFW_FALSIFY_ENUMERATE=1`, `falsify_fail_point` faz `return 0` e a execução **continua**
(`:257-263`). O `OK` do **mesmo braço** era impresso logo depois do `FAIL`.

🔴 **Por que isso é grave e não cosmético:** o censo de Windows (`.github/workflows/windows-census.yml`)
roda justamente com `TRACKFW_FALSIFY_ENUMERATE: "1"` e `continue-on-error: true` — o `rc` é
**tolerado por desenho** ("gate não, dado bruto sim") e a apuração é **por rótulo**.

⚠️ **Precisão do que foi medido, para não superar a evidência:** a linha `FAIL` **era** emitida — em
todas as rodadas com o fonte antigo o chunk imprimiu `FAIL emitidos pelo chunk: 1`. O defeito não é
"a falha some"; é que **o braço reportava as duas coisas**. Consequências medidas, e só estas:

1. **Vale para os 39 sítios:** o censo conta `^OK` e `^FAIL` (ver o comentário do trap,
   `:1539-1540`). Um braço reprovado **inflava o lado `OK`** — emitia as duas linhas —, deixando as
   duas contagens mutuamente inconsistentes. É esta a consequência universal.
2. **Vale só para o subconjunto em que o rótulo de `FAIL` difere do de `OK`** — os 6 que abriram
   este ML: ali a perna de rótulo da guarda de conjunto também **passava**
   (`OK [shard-coverage/shard_14/labels]`, com o rótulo de sucesso presente para um braço que
   reprovou), e passa a **nomear**. 🔴 Nos outros 33 a guarda é cega **antes e depois**, pelo motivo
   estrutural da §2.2 — não leia a consequência 2 como geral.

O comentário de `falsify_fail_point` (`:265-274`) já descrevia exatamente este problema — e já o
tinha resolvido **para os helpers**, com o padrão de duas linhas e `return`. Os blocos inline nunca
receberam o mesmo tratamento.

## 2. Medição (guarda de CONJUNTO, esperado do fonte pristino)

Protocolo: o esperado vem do fonte **pristino** via `gen-falsify-chunks.py`; o emitido vem do run de
uma cópia **sabotada**. Sabotagem preserva a contagem de linhas (condição → `true`), para o
particionamento não mudar — conferido em toda rodada (`chunk_pristino == chunk_sabotado`).

| braço (rótulo de sucesso) | antes | depois |
|---|---|---|
| `credential-guard-git-env-bypass/redirect-attack-is-real` | rótulo **presente**, `OK [shard-coverage/shard_14/labels]` | rótulo **ausente**, guarda **nomeia** |
| `.../config-attack-is-real` | presente, guarda cega | ausente, nomeia |
| `.../worktree-legitimate-baseline` (checagem 1) | presente, cega | ausente, nomeia |
| `.../worktree-legitimate-baseline` (checagem 2) | presente, cega | ausente, nomeia |
| `…/kiro-dedicated-file/no-double-report-and-no-regression` (checagem 1) | presente, cega | ausente, nomeia |
| idem (checagem 2) | presente, cega | ausente, nomeia |

Direção verde: árvore limpa, shards 14 e 0 (N=60) — `shard_N.actual` **byte-idêntico** antes/depois,
`rc=0`, `OK [shard-coverage/shard_N/{rc,labels}]`, e o mesmo número de linhas `^OK ` (14 e 3).
Conjunto de rótulos exigidos **232 → 232**. `FALSIFY_SUCCESS_FLOOR` **não** foi tocado: na árvore
limpa toda checagem passa, então o ramo de sucesso executa exatamente como antes — a mudança altera
**quando** o sucesso é emitido, não **se**.

⚠️ Não confunda com a contagem de `falsify_count_success` no **fonte** (66 → 66): mover a chamada
para dentro de um `else` **não pode** mudar esse número, então ele é verdadeiro por construção e não
prova nada. A evidência de runtime é a de shard acima — e, sobretudo, a da suíte inteira:

**Direção verde definitiva** (`run-gates-falsify-parallel.sh`, 8 chunks, os dois fontes, mesma
máquina): fonte antigo **273 OK / 0 FAIL**, fonte novo **273 OK / 0 FAIL**, `rc=0` nos dois, guarda
de conjunto sem rótulo ausente nos dois. São os 273 que provam que `FALSIFY_SUCCESS_FLOOR=210` não
precisava ser tocado — não o 66.

### 2.1 Extensão real da causa — 39 sítios, não 5

Os 6 rótulos só-em-`FAIL` foram a **porta de entrada**, não o tamanho do defeito. Varredura
estrutural do arquivo inteiro (casamento `if`/`fi` por pilha, restrito à coluna 0 — `if` dentro de
string multilinha aparece indentado; conferida por **duas** variantes do parser, com a mesma lista):

| forma | o que é | sítios |
|---|---|---|
| **A** | `fi` de bloco que chama `falsify_fail_point`, seguido de `falsify_count_success` **fora** dele | **30** |
| **B** | braço de **várias** checagens: o `if` irmão anterior reprova mas **não** gateia o sucesso | **9** |

Total **39** (os 5 braços que abriram o ML estão **dentro** desses 39 — foram convertidos à mão,
com flag explícita, antes da passada mecânica; por isso a lista de B saiu com 9 e não 11).

Corrigidos **todos**: A → sucesso no ramo `else`; B → `elif [[ "$_falsify_arm_fail_<linha>" -eq 0 ]]`
com a flag setada em cada ramo de falha (🔴 **nunca** `elif` puro no lugar do irmão: em modo
enumerate os dois diagnósticos precisam sair). Varredura final: **0** sítios de A, **0** de B.

Regra Dura de Causa Raiz: mesma causa, mesma REQ, mesmo PR. Fechar o roadmap com os outros 34
sítios "medidos e não corrigidos" seria o achado A1 que este repositório já pagou uma vez.

### 2.2 🔴 Quando a guarda de conjunto discrimina — e quando NÃO

`run-gates-falsify-shard.sh` monta `shard_N.actual` com
`grep -oE '^(OK|FAIL|PROOF)[[:space:]]+\[falsify/'` — **`FAIL` entra no mesmo conjunto que `OK`**.
Logo, a perna de rótulo da guarda só discrimina quando o diagnóstico de falha usa um rótulo
**diferente** do de sucesso:

| caso | exemplo | guarda de conjunto | contagem do censo |
|---|---|---|---|
| rótulo de `FAIL` ≠ rótulo de `OK` | `attack-inert` vs `redirect-attack-is-real` | **discrimina** — `FAIL … rotulo esperado AUSENTE` | corrige |
| rótulo de `FAIL` == rótulo de `OK` | `…/no-double-report`, `credential-guard-hook-resolvable/baseline` | **cega nas duas versões** (o `FAIL` satisfaz a exigência) | corrige |

Medido, injetando falha nos dois casos:

- `credential-guard-hook-resolvable/baseline` (forma B): antes `OK=2 FAIL=1` no chunk — o braço
  emitia **as duas coisas**; depois `OK=1 FAIL=1`. Guarda: `OK [shard-coverage/shard_7/labels]` nas
  duas, como previsto.
- `…/no-double-report` (forma A): antes `OK=5 FAIL=1` (3 ocorrências do rótulo); depois
  `OK=4 FAIL=1` (2 ocorrências).

🔴 **Conclusão que precisa ficar escrita:** para a maioria dos sítios o ganho desta correção está na
**consistência da apuração do censo** (`^OK` vs `^FAIL`), **não** na guarda de conjunto. Quem quiser
que a guarda discrimine esses casos precisa mudar **outra** coisa — separar `FAIL` de `OK` na
montagem do `.actual`, o que é decisão de contrato, não deste ML. Não confunda os dois ganhos.

## 3. Braço com DUAS checagens: use flag, nunca `elif`

`worktree-*` e o braço do Kiro têm **duas** checagens e **um** rótulo de sucesso. `elif` engoliria o
diagnóstico da segunda quando a primeira reprovasse — e em modo enumerate os dois diagnósticos
precisam sair. O padrão correto:

```bash
bad=0
if <checagem 1 falhou>; then echo "FAIL ..."; falsify_fail_point; bad=1; fi
if <checagem 2 falhou>; then echo "FAIL ..."; falsify_fail_point; bad=1; fi
if [[ $bad -eq 0 ]]; then falsify_count_success; echo "OK   [falsify/...]"; fi
```

## 4. 🔴 `vacuity-guard` NÃO pode receber rótulo de sucesso — medido, não argumentado

O epílogo (`:7490`) está dentro de `if ! declare -f __falsify_timing_mark`. Essa função é injetada
pelo `gen-falsify-chunks.py` em **todo** chunk — logo, **em chunk o bloco nunca executa**.

Experimento: acrescentei `echo "OK [falsify/vacuity-guard/floor-met]"` dentro do bloco, numa cópia.
O rótulo **entra no manifesto** (`chunk=43 label=vacuity-guard/floor-met`, N=60) e **nunca é
emitido** → guarda de conjunto `FAIL ... rotulo esperado AUSENTE`. **Gate permanentemente vermelho.**

É a mesma armadilha da nota irmã, por outro caminho: rótulo que a árvore correta **não emite** não
pode virar exigência. `vacuity-guard` fica **declarado sem ponto de prova**, não maquiado.

## 5. Família `setup*` está fora — confirmado num caso concreto

`…/kiro-dedicated-file/setup` **não aparece no manifesto** (contagem 0). Quebrei o `update harness`
do Cenário 69 (`--targets nao-existe-este-alvo`): o rótulo `setup` continua não sendo exigido, mas a
guarda **nomeia o cenário assim mesmo**, por
`…/kiro-dedicated-file/no-double-report-and-no-regression` ausente. Setup quebrado se manifesta pela
ausência dos rótulos **do cenário**, não por um rótulo próprio.

## 6. 🔴 Dois achados laterais, mesma família de sintoma (para o arquiteto)

**(a) O `CHUNK_ABORT` do ML-2A cobre só o chunk que recebeu o Cenário 18.** O `trap ERR` é instalado
em `:1547`, *dentro* do bloco do Cenário 18, e o próprio comentário declara o alcance ("do ponto de
instalação até o fim do processo"). Num particionamento de N chunks, **N-1 chunks não têm o trap**.
Medido: chunk_0 (N=60) morreu por `set -e` **sem uma linha de `CHUNK_ABORT`** — exatamente o sintoma
que o ML-2A existe para eliminar. No censo de 8 shards, 7 dos 8 ficam sem diagnóstico.

**(b) O sítio da morte silenciosa medida acima:**

```bash
s69bad_gbg_count=$(grep -oF 'trackfw-git-branch-guard.json' <<<"$s69bad_out" | wc -l | tr -d ' ')
```

Sob `set -euo pipefail`, `grep -oF` **sem casar nada** sai 1, o `pipefail` propaga e o `set -e`
**mata o chunk** ali. Não é hipótese: é o ponto onde o log para, no run com o setup quebrado.
Não corrigi nenhum dos dois — `check-gates-falsify.sh` na região do 18 é entrega do ML-2A, já
auditada, e a decisão de escopo é do arquiteto (Regra Dura: mesmo sintoma, mesmo roadmap).
