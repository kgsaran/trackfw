# Red Team — gate `check-mktemp-cleanup.sh` e correções ML-1A

> Hades (hades-tf) — 2026-11-11
> REQ: REQ-2026-11-11-mktemp-sem-limpeza-garantida-no-mesmo-escopo-vaza-temporarios-em-self-tests.md
> Branch: fix/mktemp-sem-limpeza-garantida-no-mesmo-escopo (ML-2A)
> Scratchpad: `/private/tmp/claude-501/-Users-kgsaran-Sistemas-Desenvolvimento-workspace-trackfw/a247cc38-463b-4f99-8cb2-7400f9dec171/scratchpad/hades-w2/`

---

## Veredito

**APROVA COM RESSALVAS**

O gate `check-mktemp-cleanup.sh`, o `--self-test` (22/22) e as 11 correções do ML-1A passam na
avaliação. Nenhuma regressão encontrada. `make quality` rc=0, 3182 linhas. Dois falsos negativos
foram encontrados fora dos residuais declarados, ambos no caminho de apelido DQ em escopo global.
O residual R5 foi confirmado como real, com zero instâncias problemáticas no corpus. Dois gaps
de reconciliação encontrados: a verificação de ordenamento DQ em função não tem braço de falha;
o critério de exclusão de TMPDIR não tem braço em nenhuma direção.

**Ressalvas (findings novos, não declarados):**
- **MEDIUM** — Trap DQ instalada antes do `printf -v` em escopo global: gate rc=0, V não limpo
  em runtime. Medido com runtime: script rc=0, diretório de V sobrevive.
- **LOW** — Apelido (`printf -v q`) reutilizado para segunda variável em escopo global: gate rc=0,
  V não limpo em runtime. Medido com runtime: script rc=0, diretório de V sobrevive.

**Ressalvas de reconciliação (gaps no self-test):**
- Nenhum braço `fail-alias-fn-dq-ordering` cobre a verificação de ordenamento de apelido DQ em
  função. Mutação A (ordenamento removido) dá 22/22 — conclusão sem braço.
- Nenhum braço cobre exclusão de `$TMPDIR` como container. Mutação B (exclusão removida) dá
  22/22 — conclusão sem braço.

---

## 1. Tabela de Ataques

| # | Vetor | Arquivo sintético | rc esperado | rc medido | Conclusão |
|---|-------|-------------------|-------------|-----------|-----------|
| FN-1 | Template com `"${TMPDIR:-/tmp}/…"` sem trap | `synth/fn1_no_trap.sh` | 1 | 1 | Gate correto |
| FN-2a | SQ multi-linha; var real após | `synth/fn2a_multiline_sq.sh` | 1 | 1 | Gate correto — SQ tracking preservado |
| FN-2b | ANSI-C `$'V=$(mktemp …)'` não é candidata | `synth/fn2b_ansi_c_quote.sh` | 0 | 0 | Gate correto — conteúdo `$'…'` ignorado |
| FN-3 | Função one-liner | arm `pass-function-oneliner` | 0 | 0 | Gate correto |
| FN-4 | FUNCTION+ALIAS externo | arm `pass-function-alias` | 0 | 0 | Gate correto |
| FN-5 | Trap substituído em escopo de função | arm `fail-trap-replaced-fn` | 1 | 1 | Gate correto — last-trap-wins aplicado |
| **FN-5b** | **Apelido reutilizado em escopo global** | **`synth/fn5b_alias_reassigned.sh`** | **1** | **0** | **FALSO NEGATIVO — LOW** |
| FN-5c / R5 | `exit` em `if false` dispara heurístico R5 | `synth/fn5c_exit_inside_if.sh` | 1 | 0 | Residual R5 — zero instâncias no corpus |
| FN-6 | Contenção sob diretório não coberto | arm `fail-contained-under-uncovered` | 1 | 1 | Gate correto |
| FN-7 / R3 | `exec CMD` após trap (residual R3 declarado) | `synth/fn7_exec_after_trap.sh` | 1¹ | 0 | Residual R3 declarado — FN intencional |
| FN-8a | `trap - EXIT` em função | arm `fail-trap-reset-fn` | 1 | 1 | Gate correto |
| FN-8b | `trap '' EXIT` corpo vazio | `synth/fn8b_trap_empty.sh` | 1 | 1 | Gate correto |
| FN-9 / R1 | Var em subshell `d=$(fn)` | — (ML-1A usou WORK global) | 1¹ | — | Residual R1 declarado; ML-1A mitigou o único caso |
| FN-10 / R2 | Reatribuição em loop | — | 1¹ | — | Residual R2 declarado |
| FN-11 | Sem exclusão por path | — | 0 | 0 | Gate varre todos os `.sh` sem exclusão |
| FN-12 | `mktemp` em ramo de `case` | arm `fail-fn12-case-branch` | 1 | 1 | Gate correto |
| Heredoc `<<-` + apóstrofo no corpo | `synth/heredoc_strip.sh` | 1 | 1 | Gate correto — `in_sq` não corrompido |
| Heredoc delimitador entre aspas | `synth/heredoc_quoted_delim.sh` | 1 | 1 | Gate correto |
| **DQ alias trap antes de `printf -v` (global)** | **`synth/attack_alias_after_trap_global.sh`** | **1** | **0** | **FALSO NEGATIVO — MEDIUM** |
| DQ alias trap antes de `printf -v` (função) | `synth/attack_alias_after_trap_fn.sh` | 1 | 1 | Gate correto — ordering check de fn ativo |
| Padrão `q_tf` vazio de `shim-diff:158` (ML-1A) | `synth/attack_q_tf_empty.sh` | 1 | 1 | Gate correto — ordering check de fn pega |
| Contenção + `trap - EXIT` em função | `synth/attack_contained_fn_trap_reset.sh` | 1 | 1 | Gate correto — W e sub ambos descobertos |
| `$TMPDIR` não é container válido | `synth/attack_tmpdir_container.sh` | 1 | 1 | Gate correto — TMPDIR excluído |

¹ rc esperado = 1 indica que o gate DEVERIA reprovar mas não reprova; estes são os residuais
declarados, não "comportamento correto do gate".

**Verificação de falsos positivos:** `bash scripts/check-mktemp-cleanup.sh $(git ls-files '*.sh')` → rc=0,
zero violações em todos os `.sh` do repositório.

**R5 — corpus:** gate modificado (exit-boundary neutralizado) deu rc=1 em um único arquivo:
`scripts/check-marker-corpus-parity.sh:34` (`_st_tmp`). Leitura do arquivo: `_st_tmp` é criado
dentro do bloco `if [[ "${1:-}" == "--self-test" ]]` (L33–L307), com trap DQ em L37. O exit em
L307 (`exit $?`) é o único caminho de saída desse bloco; o trap global da linha 325 (que
supersediaria L37) só é instalado no caminho principal, que nunca cria `_st_tmp`. **Separação
genuína de caminho — R5 dispara corretamente.** Zero instâncias problemáticas no corpus.

---

## 2. Tabela de Reconciliação — 22 Braços do `--self-test`

O ML-1B não entregou a frase de reconciliação por braço (Regra Dura). O cruzamento a seguir foi
feito lendo as linhas `# Affirms:` no próprio gate (`check-mktemp-cleanup.sh:734–983`).

| # | Nome do braço | rc | Referência (ADR D2 / Wave 0) | Conclusão afirmada |
|---|---------------|----|------------------------------|--------------------|
| 1 | `pass-direct` | 0 | ADR D2 DIRECT | Trap SQ que cita `$V` diretamente cobre V |
| 2 | `pass-direct-brace-default` | 0 | ADR D2 DIRECT + FP-1 | `${V:-}` no corpo também é citação válida |
| 3 | `pass-alias-printf-v` | 0 | ADR D2 ALIAS + FP-2 | `printf -v A '%q' "$V"` + trap DQ citando `$A` cobre V |
| 4 | `pass-alias-assign-direct` | 0 | ADR D2 ALIAS | Atribuição `A=$V`; trap SQ que cita `$A` cobre V |
| 5 | `pass-function-alias` | 0 | ADR D2 FUNCTION+ALIAS | Indireção de função + apelido por atribuição cobre V |
| 6 | `pass-function` | 0 | ADR D2 FUNCTION | Indireção de um nível: trap chama função que cita V |
| 7 | `pass-function-oneliner` | 0 | ADR D2 FUNCTION + FN-3 | Corpo de função one-liner é coletado corretamente |
| 8 | `pass-contained` | 0 | ADR D2 CONTAINED + FP-3/5/6 | `mktemp` sob diretório coberto herda cobertura |
| 9 | `pass-local-fn-alias` | 0 | ADR D2 ALIAS (escopo fn) | Apelido em escopo de função satisfaz last-trap-rule |
| 10 | `pass-sq-fixture-skip` | 0 | FP-4b + FN-2 (B1/B2 fixes) | SQ tracking impede string fixture de virar candidata |
| 11 | `pass-no-dollar-paren-not-candidate` | 0 | FP-4a | Linha sem `$(` nunca é candidata a `mktemp` |
| 12 | `fail-no-trap` | 1 | ADR D2 baseline | Ausência de trap EXIT = ausência de garantia de limpeza |
| 13 | `fail-trap-no-citation` | 1 | ADR D2 (T1 threat model) | Trap que cita var diferente não cobre V |
| 14 | `fail-rm-normal-path-only` | 1 | ADR D1 | `rm` só no caminho normal não é limpeza garantida |
| 15 | `fail-trap-replaced-fn` | 1 | Last-trap-wins (função) + FN-5 | Trap de cobertura substituído em função deixa V descoberto |
| 16 | `fail-trap-reset-fn` | 1 | Last-trap-wins + FN-8 | `trap - EXIT` remove cobertura em escopo de função |
| 17 | `fail-contained-under-uncovered` | 1 | ADR D2 CONTAINED + FN-6 | Contenção só cobre V quando o container W está coberto |
| 18 | `fail-fn1-template-args` | 1 | FN-1 | Args de `mktemp` com `"` ainda são detectados como sítio |
| 19 | `fail-fn12-case-branch` | 1 | FN-12 | `mktemp` em ramo de `case` é detectado |
| 20 | `fail-trap-replaced-global` | 1 | Last-trap-wins (global) | Trap global substituído sem exit entre eles deixa V descoberto |
| 21 | `fail-heredoc-apostrophe` | 1 | B1/B2 bugs + heredoc | Conteúdo de heredoc não corrompe `in_sq`; V após heredoc detectado |
| 22 | `vac-no-candidates` | rc≠0 | Guarda de vacuidade | Gate falha em corpus sem candidatos (piso = 1) |

**Cobertura da tabela — mecanismos ADR D2:**

| Mecanismo | Braço positivo | Braço negativo |
|-----------|---------------|---------------|
| DIRECT | 1, 2 | 12, 13, 14 |
| ALIAS | 3, 4, 9 | — ⚠️ |
| FUNCTION | 5, 6, 7 | — ⚠️ |
| CONTAINED | 8 | 17 |

**Gaps de reconciliação encontrados por mutação:**

- **Verificação de ordenamento DQ em função (alias)** — Mutação A (linha 557 de `trap_covers`
  removida): gate ainda dá 22/22 no self-test. Os braços 3 e 9 afirmam que o ordering correto
  passa, mas não há braço `fail-alias-fn-dq-ordering` que confirme que o ordering errado reprova.
  Medido: `attack_alias_after_trap_fn.sh` (DQ trap antes do printf-v em função) dá rc=1 com o gate
  real, mas rc=0 com Mutação A — confirmando que a lógica existe mas não tem braço de falha.

- **Exclusão de `$TMPDIR` como container** — Mutação B (exclusão removida): gate dá 22/22 no
  self-test. `attack_tmpdir_container.sh` ainda dá rc=1 com Mutação B (porque TMPDIR também não é
  variável coberta no script), mas o critério de exclusão explícita não tem braço que o afirme.

---

## 3. Revisão do ML-1A — Regressão

### `check-usage-silencing.sh` — unificação do trap do self-test

O executor removeu o trap secundário do self-test interno (que supersedia o trap global WORK) e
reescreveu a criação de `tmp` como `mktemp -d "$WORK/selftest.XXXXXX"` (contenção). O padrão
antigo foi confirmado como detectado pelo gate: `attack_selftest_trap_supersedes.sh` rc=1
(`scripts/check-usage-silencing.sh:3: variable WORK created by mktemp without guaranteed cleanup`).
O padrão novo: gate rc=0. Nenhuma regressão.

### `check-gates-falsify.sh:1476` — `bare_origin` sob `$WORK`

`bare_origin` foi movido de `mktemp -d` livre para `mktemp -d "$WORK/bare-origin.XXXXXX"`. WORK
global tem trap em L34. `s5x_commit_fixture_with_origin` não tem trap próprio, herda o global. O
gate marca `bare_origin` como CONTAINED. `attack_contained_fn_trap_reset.sh` confirma que `trap -
EXIT` em função exporia esse padrão — mas nenhum `trap - EXIT` existe na função em questão.
Nenhuma regressão.

### Demais sítios

Lidos no diff (commit `81f6be5e`), todos na forma prescrita pela ADR D3:
- `check-shim-diff-applies.sh:158` — forma canônica isolada. `attack_q_tf_empty.sh` rc=1 confirma
  que ordering check de função captura o padrão oposto. ✓
- `check-shim-diff-applies.sh:490,491` — contenção sob `$tmp`. ✓
- `check-no-literal-nul-in-source.sh:89,133,148,165` — trap SQ combinado com `${VAR:-}`. ✓
- `check-parity-call-site-pins.sh:222` — trap SQ combinado com `${local_dryrun:-}`. ✓
- `check-release-tag-parity.sh:330` — contenção sob `$WORK`. ✓

Nenhuma regressão em nenhum dos 11 sítios.

---

## 4. `make quality`

```
rc=0
3182 linhas
```

Confirmado no log (`quality.log`): `check-mktemp-cleanup.sh` aparece nas linhas 2201–2229 (self-test
rc=0, scan rc=0). Linhas com `FAIL` no log são exclusivamente braços de self-test de outros gates
(expected failures). Nenhum `FAIL` real no corpus do produto.

A diferença de ~31 linhas em relação à estimativa de ~3151 é explicada por saídas verbosas do
self-test do gate novo (22 braços com log individual).

---

## 5. Achados

### MEDIUM — Trap DQ instalada antes de `printf -v` em escopo global

**Arquivo:** `synth/attack_alias_after_trap_global.sh`

```bash
#!/usr/bin/env bash
set -euo pipefail
q=""
V=$(mktemp -d)
trap "rm -rf -- $q" EXIT   # DQ: bash expande $q na instalação → $q="" → body é "rm -rf -- "
printf -v q '%q' "$V"      # alias definido DEPOIS do trap
```

**Gate:** rc=0 (V marcado como coberto via apelido `q`).
**Runtime medido:** `V=/tmp/tmp.qitrJmDr1Q` criado; ao sair, trap body é `rm -rf -- ` (argumento
vazio); diretório de V sobrevive. Script rc=0.

**Causa:** `trap_covers` em AWK aplica o ordering check apenas para escopos não-globais:
```awk
# scripts/check-mktemp-cleanup.sh:557
if (alias_scope_name != "global" && alias_line_no[i] > trap_line_no[ti]) continue
```
A condição `alias_scope_name != "global"` exclui o escopo global da verificação. `trap_is_dq[ti]`
existe no array (L523, L667).

**Correção sugerida:** em L557, substituir a condição de forma a aplicar o ordering check para DQ
traps independentemente do escopo:
```awk
# antes:
if (alias_scope_name != "global" && alias_line_no[i] > trap_line_no[ti]) continue
# depois:
if (trap_is_dq[ti] && alias_line_no[i] > trap_line_no[ti]) continue
```
Verificação pós-patch necessária: self-test 22/22 e corpus rc=0 (a correção do shim-diff usa
printf-v DEPOIS do mktemp e ANTES do trap — o ordering seria respeitado e o braço `pass-alias-printf-v`
continuaria verde).

**Corpus:** nenhum dos scripts em `git ls-files '*.sh'` usa DQ alias trap em escopo global.
Risco futuro, não defeito presente.

---

### LOW — Apelido reutilizado para segunda variável em escopo global

**Arquivo:** `synth/fn5b_alias_reassigned.sh`

```bash
#!/usr/bin/env bash
set -euo pipefail
V=$(mktemp -d)
printf -v q '%q' "$V"      # q → V (L4)
W=$(mktemp -d)
printf -v q '%q' "$W"      # q → W (sobrescreve, L6)
trap "rm -rf -- $q" EXIT   # DQ: bash expande $q na instalação → q = W's path → só W limpo
```

**Gate:** rc=0 (V marcado como coberto porque apelido `q` aponta para V em algum ponto no escopo).
**Runtime medido:** trap body é `rm -rf -- <W_path>`. Diretório de V sobrevive ao script.

**Causa:** o gate não rastreia "último valor do apelido" ao resolver uma trap DQ. Qualquer
`printf -v q '%q' "$V"` antes do trap faz o gate associar `q → V`, mesmo que uma reatribuição
posterior aponte `q → W`. A correção do achado MEDIUM não cobre este caso: ambas as atribuições
(L4 e L6) estão antes do trap (L7), então o filtro de ordering não elimina a L4.

**Correção sugerida:** para traps DQ, o gate deve associar o apelido ao `printf -v` de maior
número de linha anterior ao trap, não ao primeiro. Exige rastrear o índice de alias mais recente
para cada nome de apelido antes do trap.

**Corpus:** nenhum dos scripts usa reuso de apelido para duas variáveis no mesmo escopo.

---

### Residual R5 — descrição subespecificada (não bloqueia)

**Arquivo:** `synth/fn5c_exit_inside_if.sh`

```bash
V=$(mktemp -d)
trap 'rm -rf "$V"' EXIT
if false; then
  exit 0          # ^[[:space:]]*exit casa em escopo global mesmo dentro de if/fi
fi
W=$(mktemp -d)
trap 'rm -rf "$W"' EXIT   # supersede o trap de V
```

**Gate:** rc=0 (exit em L6 detectado como fronteira de caminho pela regex `^[[:space:]]*exit`).
**Runtime:** `if false; then exit 0; fi` nunca executa; ambos os traps são instalados; segundo
supersede o primeiro; apenas W é limpo; V vaza.

**Avaliação:** o gate acerta na separação genuína de caminho (medido em `check-marker-corpus-parity.sh`
— ver §1). O problema é que a regex `^[[:space:]]*exit` captura exits condicionalmente não-alcançáveis
(inside `if/fi` com `brace_depth=0`), não apenas exits em ramos de `case`. A declaração de R5 no
cabeçalho do gate diz "case-branch exit" mas o mecanismo real é mais amplo. **Zero instâncias
problemáticas no corpus.** Recomenda-se ampliar a descrição de R5 para incluir qualquer bloco
condicional; nenhuma ação de código nesta REQ.

---

## 6. Status dos Critérios de Aceite (ML-2A)

| Critério | Status | Evidência |
|----------|--------|-----------|
| Parecer com veredito | ✅ | Veredito: APROVA COM RESSALVAS |
| Falsos negativos FN-1..12 atacados com sítios sintéticos | ✅ | Tabela §1 |
| FP check: gate rc=0 no corpus real | ✅ | rc=0 medido |
| Reconciliação 22 braços | ✅ | Tabela §2 (gaps identificados) |
| `make quality` rc=0 (AC5) | ✅ | rc=0, 3182 linhas |
