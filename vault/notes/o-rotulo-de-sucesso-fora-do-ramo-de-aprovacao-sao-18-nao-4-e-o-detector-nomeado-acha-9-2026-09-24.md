# O rótulo de sucesso **fora do ramo de aprovação** são **18**, não 4 — e o detector nomeado enxerga só metade

> 2026-09-24 · ML-0B da REQ do cluster de Windows · medido em macOS 27, `bash 5.3`, sobre
> `scripts/check-gates-falsify.sh` (8146 linhas) e `scripts/check-release-tag-parity.sh`

## O que estava em aberto

A Wave 0 nomeou dois sítios (Cenários 87 e 158), o ML-3A achou mais dois, e o handoff do ML-0B
trazia **4**. O detector proposto era: *"a Forma B não chama `falsify_count_success`, logo a
contagem de `^OK` no log ≠ tally"*.

## 1. 🔴 O detector nomeado é **falso** para 9 dos 18 sítios

`5742`, `6319`, `6403`, `6484` e `6596` (numeração pré-ML-0B) chamam `falsify_count_success`
**imediatamente antes** do `echo "OK …"` incondicional. O tally e o `^OK` **andam juntos** — e
seguem andando juntos quando o cenário reprova, porque em modo de enumeração **os dois** são
executados. Quem caçasse a divergência `^OK` vs tally acharia **menos** sítios do que existem, e
concluiria que os outros estavam certos.

Corolário medido: nesses 5 sítios o cenário reprovado incrementa **os dois** contadores, o que
**infla** `falsify_success_n` contra o `FALSIFY_SUCCESS_FLOOR` — um piso de vacuidade alimentado
por reprovações.

## 2. A enumeração que funciona é **estrutural**, e o token `falsify_fail_point` não basta

A forma é *"rótulo de sucesso emitido por caminho que não é o ramo de aprovação da checagem que ele
afirma"*. Em `TRACKFW_FALSIFY_ENUMERATE=1` o ponto de reprovação **retorna** em vez de sair — é o
que torna a enumeração possível — então todo `echo "OK …"` alcançável **depois** dele imprime
veredito de sucesso sobre checagem reprovada.

O varredor (em `awk`, com heredoc ignorado, `if/else/fi` contados e `return`/`exit` desarmando)
precisa armar também nos **envelopes** que alcançam o ponto de reprovação e **retornam**:

```bash
# fecho transitivo dos envelopes — sem ele, 8 dos 18 sítios são invisíveis
awk '/^[a-zA-Z_][a-zA-Z_0-9]*\(\)[ \t]*\{/{fn=$1}
     /falsify_fail_point|falsify_count_failure/{if(fn)print fn}' \
  scripts/check-gates-falsify.sh | sort -u
# -> assert_fails_with, assert_guard_exit, assert_lacks_pattern, assert_output_contains,
#    assert_output_lacks, assert_succeeds, assert_would_now_fail, assert_writer_no_epipe,
#    build_go_or_fail, remove_roadmap_acceptance_heading, resolve_py_bin, run_go_guard_dump,
#    run_python_chain_probe, to_native_path
```

Armando só no token literal: **12 candidatos**. Com o fecho: **27 candidatos**, dos quais **18**
verdadeiros e **9** falsos positivos — estes últimos já corrigidos por uma campanha anterior com
o padrão `_falsify_arm_fail_<linha>` + `elif`, que o varredor não modela.

## 3. Duas subclasses, e a segunda é a que ninguém tinha visto

| subclasse | sítios | o que o `OK` afirma |
|---|---|---|
| **guard de setup** | 10 | "a fixture se formou / a baseline passa com o binário real" |
| **sumário de braços** | 8 | "os N braços (A…K) foram provados" — depois de uma série de `assert_*` |

A segunda inclui `call-site-pin`, `ci-workflow-self-governance`, `crlf-normalize`,
`emitting-capture`, `unguarded-rc`, `write-containment`, `roadmap-ref-stale-state/{go,python}`.
Um `assert_fails_with` que reprova em modo de enumeração **retorna 0** (deliberado: `return 1`
mataria o chunk pelo `set -e` do call site) e o sumário logo abaixo imprime *"os 3 braços
provados"*. É a forma mais enganosa das duas, porque o rótulo **afirma o conjunto**.

## 4. A correção que **não** muda o conjunto de rótulos

`gen-falsify-chunks.py` colhe os rótulos exigidos pela guarda de conjunto do driver a partir do
**texto literal** `echo "OK   [falsify/<rótulo>]"` (`ECHO_LABEL_PAT`, linhas não-comentário).
Trocar a emissão por uma chamada de helper **apagaria o rótulo do conjunto esperado** — gate verde
com cobertura perdida. Por isso a correção preserva o `echo` literal e só o move para um ramo:

```bash
_falsify_mark_sNN=$(falsify_fail_mark)      # foto do tally de reprovações
… guards / assert_* …
if falsify_failed_since "$_falsify_mark_sNN"; then
  echo "FAIL [falsify/<rótulo>]: rótulo de sucesso suprimido — …" >&2
else
  falsify_count_success
  echo "OK   [falsify/<rótulo>]"
fi
```

🔴 **O ramo suprimido emite `FAIL [falsify/<mesmo rótulo>]`**, nunca silêncio: o driver aceita a
linha `FAIL` como emissão do rótulo, e sumir com ela transformaria *"cenário reprovou"* em
*"rótulo esperado AUSENTE"* — diagnóstico errado, o mesmo raciocínio já escrito no Cenário 181.

Marca e leitura **têm** de ficar no mesmo cenário: o gerador só corta em `# Cenário N — …`
(`HDR_PAT`), então par dentro do cenário nunca é separado — conferido gerando N=4 e N=8 e
grepando as 12 marcas (`atrib=1 leit≥1` no **mesmo** `chunk_*.sh` nas duas partições).

## 5. 🔴 Achado independente: o **Cenário 200 está depois do bloco de saída do modo de enumeração**

`check-gates-falsify.sh` tem o bloco `if [[ "$TRACKFW_FALSIFY_ENUMERATE" == "1" ]]; then … exit 1`
**antes** do Cenário 200 (`:7982` vs `:8001` na numeração pré-ML-0B; `:8105` vs `:8124` depois).
Consequência medida no run sabotado: qualquer reprovação no chunk que carrega esse bloco
**aborta antes** do Cenário 200, e os **9 rótulos `interp-path/*` somem** — o driver os reporta
como *"rótulo esperado AUSENTE"*, que lê como chunk morto, não como "a execução parou aqui".

É **pré-existente** (mesma ordem relativa nas duas versões do arquivo) e **não** é Forma B: a causa
é posicional. No censo de Windows — que roda justamente em modo de enumeração e tem reprovações
por construção — significa que o último cenário do arquivo **nunca é medido** quando algo antes
dele reprova.

## Falsificação nas duas direções (o que foi medido)

| direção | comando | resultado |
|---|---|---|
| árvore íntegra passa | `GO_BIN=$PWD/bin/trackfw bash scripts/run-gates-falsify-parallel.sh` | `rc=0`, 268 rótulos `OK` distintos, 0 `FAIL` |
| mutação reprova | mesma suíte, cópia sabotada + `TRACKFW_FALSIFY_ENUMERATE=1` | `rc=1`; nos **18** sítios o `OK` **some** e o `FAIL` do mesmo rótulo aparece |
| sem cascata | diferença dos conjuntos de `OK` | **27** rótulos a menos = **18** alvos + **9** do Cenário 200 (item 5), nenhum outro |
| `assert_three_way` antes | sabotagem injetando um `fail` no Cenário 1 sobre o helper **antigo** | `FAIL …/sabotagem` na linha 1 e **`OK …/go-behavioral-pin` na linha 2** — o falso verde, reproduzido |
| `assert_three_way` depois | mesma sabotagem sobre o helper corrigido | `FAIL` + `FAIL`; **18 de 19** pins seguem `OK` — a supressão não cascateia |

⚠️ **Rodar cópia sabotada de um gate exige raiz plausível**: `ROOT_DIR=$(dirname
"${BASH_SOURCE[0]}")/..`, então a cópia em `/tmp` morre em `lib-crlf-normalize.sh: No such file`.
Para `check-release-tag-parity.sh` o caminho é uma **fazenda de symlinks** (`ln -s` de cada entrada
do repo para um `fakeroot/`, com `scripts/` real contendo symlinks + a cópia sabotada). Para
`check-gates-falsify.sh` existe saída melhor e oficial: `TRACKFW_FALSIFY_SCRIPT=<cópia>` no
`run-gates-falsify-parallel.sh`, que já foi feito para esse fim.

## Regra de bolso

Antes de confiar num detector **nomeado no handoff**, teste-o contra os sítios que você já conhece
**e** contra um sítio que ele deveria pegar por outro caminho. O detector `^OK` vs tally passava
nos 2 sítios da Wave 0 e falhava em 9 dos 18 — e a lista "4 sítios conhecidos" é a **quinta**
enumeração incompleta desta campanha.
