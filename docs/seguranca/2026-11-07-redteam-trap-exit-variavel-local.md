# Red Team — ML-2A: trap EXIT que cita variável local (#671)

> Hades — 2026-11-07
> REQ: `docs/req/REQ-2026-11-07-trap-exit-de-self-test-cita-variavel-local-e-mascara-a-falha-real.md`
> Wave 0: `docs/seguranca/2026-11-07-wave0-trap-exit-variavel-local.md`
> Branch: `fix/trap-exit-variavel-local`

---

## Veredito

**APROVA COM RESSALVAS**

Dois falsos negativos no gate, ambos LOW (zero instâncias no corpus atual). Os três sítios estão
corrigidos e os comportamentos medidos são limpos. Ver achados #1 e #2.

---

## Tabela de ataques

Legenda — rc: código de saída real medido com `$?`. Gate: rc=1 esperado (DETECT) ou rc=0 esperado (SKIP).
Comportamento: "CLEAN" = sem dir temporário vazado e sem `unbound variable` vindo do trap.

### Item 1 — 7 itens obrigatórios do Wave 0

| # | Item Wave 0 | Evidência medida | Resultado |
|---|-------------|-----------------|-----------|
| O1 | REQ inclui Site C (`check-orphan-gates.sh`) | `grep -n 'printf -v q'` → `check-orphan-gates.sh:142` | PASS |
| O2 | Site A: `printf -v q '%q' "$tmp"` na instalação | `check-shim-diff-applies.sh:178` tem a forma correta | PASS |
| O3 | Site A: GITHUB_OUTPUT removido do trap body | Linha 184–186 do shim: só `unset GITHUB_OUTPUT`, sem restore no trap | PASS |
| O4 | Sites B e C: mesmo padrão | `check-req-roadmap-rule-is-status-aware.sh:232` e `check-orphan-gates.sh:142` | PASS |
| O5 | Gate `check-no-trap-local-var.sh` com self-test (21 braços, vacuidade) | `bash check-no-trap-local-var.sh --self-test` → 21/21 PASS, rc=0 | PASS |
| O6 | Vetores de regressão: `false` e unbound var com `/bin/bash` e bash 5 | Comportamento medido (ver tabela abaixo) | PASS |
| O7 | Reconciliação: 21 frases `Reconciliation:` para 21 cases | `grep -c 'Reconciliation:'` → 21 | PASS |

### Item 2 — Fixtures de detecção do gate (devem ser detectadas)

| Fixture | Forma | Gate rc | Esperado | Resultado |
|---------|-------|---------|----------|-----------|
| T1 | `{` em linha separada da assinatura | rc=1 (`t1_brace_sep.sh:5:tmp`) | DETECT | PASS |
| T2 | `}` dentro de string no corpo | rc=1 (`t2_brace_in_str.sh:6:tmp`) | DETECT | PASS |
| T3 | `}` dentro de heredoc, trap após | rc=1 (`t3_brace_heredoc.sh:8:tmp`) | DETECT | PASS |
| T4 | trap dentro de `case` na função | rc=1 (`t4_trap_in_case.sh:6:tmp`) | DETECT | PASS |
| T5 | trap dentro de `if` na função | rc=1 (`t5_trap_in_if.sh:5:tmp`) | DETECT | PASS |
| T7 | body com `'\''` (aspas simples escapadas) | rc=1 (`t7_sq_escape.sh:4:tmp`) | DETECT | PASS |
| T9 | arquivo CRLF | rc=1 (`t9_crlf.sh:4:tmp`) | DETECT | PASS |
| T10 | múltiplas funções: detecta só a ruim | rc=1 (só `t10_multi.sh:10:DIR`) | DETECT | PASS |
| T13 | `local -r VAR=value` | rc=1 (`:3:tmp`) | DETECT | PASS |
| T14 | `declare -x WORK` (export, ainda local) | rc=1 (`:4:WORK`) | DETECT | PASS |
| T17 | `trap 'body' ERR EXIT` (lista com EXIT) | rc=1 (`:4:tmp`) | DETECT | PASS |
| T19 | sinal 0 não primeiro: `trap 'body' HUP 0 TERM` | rc=1 (`:4:tmp`) | DETECT | PASS |

### Item 3 — Falsos positivos (gate NÃO deve acusar)

| Fixture | Forma | Gate rc | Esperado | Resultado |
|---------|-------|---------|----------|-----------|
| N1 | trap em nível de script com var global | rc=0 | SKIP | PASS |
| N2 | body em aspas duplas (expansão na instalação) | rc=0 | SKIP | PASS |
| N3 | `declare -g GVAR` (global) | rc=0 | SKIP | PASS |
| N4 | função subshell `f() ( ... )` | rc=0 | SKIP | PASS |
| N5 | var global com mesmo nome de local de outra função | rc=0 (só `LOCAL_X` na func_b) | SKIP | PASS |
| N6 | `trap '...$E' ERR` (não EXIT/0) | rc=0 | SKIP | PASS |
| N7 | forma corrigida `printf -v q '%q'` | rc=0 | SKIP | PASS |
| T9b | arquivo CRLF com forma corrigida | rc=0 | SKIP | PASS |
| T15 | `declare -g` em func_a não contamina func_b | rc=1 (só `LOCAL_X` em func_b) | DETECT func_b | PASS |

### Item 4 — Formas em que o awk pode errar

| Fixture | Forma | Gate rc | Esperado | **Resultado** |
|---------|-------|---------|----------|--------------|
| T6 | `trap -- 'rm -rf "$tmp"' EXIT` | rc=0 (sem violação) | DETECT | **FAIL — FN** |
| T8/T16 | `f() { local x; trap 'rm -rf "$x"' EXIT; }` (one-liner) | rc=0 | DETECT | **FAIL — FN** |

### Item 5 — Vacuidade e `git ls-files` com espaço

| Teste | Resultado |
|-------|-----------|
| Corpus sem nenhum `trap` → rc=1 (vacuity guard) | PASS |
| Arquivo com espaço no nome passado como argumento direto | DETECT (rc=1, nome completo no output) |
| Arquivo com espaço via `while IFS= read -r f; do arr+=("$f"); done` | DETECT (rc=1) |
| `git ls-files '*.sh'` no repositório real (espaço não ocorre em `.sh` trackeados) | PASS (zero arquivos com espaço) |

### Item 6 — Comportamento: cópias dos três scripts, falhas injetadas

Todos os testes rodaram com `/bin/bash` (3.2 — macOS) e `/opt/homebrew/bin/bash` (5.2.37).
Critério: rc≠0, zero dirs temporários vazados, sem `unbound variable` do trap.

| Trigger | bash | rc | Dirs vazados | unbound do trap | Resultado |
|---------|------|----|-------------|-----------------|-----------|
| `false` após trap (set -e) | 3.2 | 1 | 0 | não | CLEAN |
| `false` após trap (set -e) | 5 | 1 | 0 | não | CLEAN |
| `$UNDEFINED_VAR` (set -u) | 3.2 | 1 | 0 | não¹ | CLEAN |
| `$UNDEFINED_VAR` (set -u) | 5 | 1 | 0 | não | CLEAN |
| Retorno normal + exit script | 3.2 | 0 | 0 | não | CLEAN |
| Retorno normal + exit script | 5 | 0 | 0 | não | CLEAN |
| `mktemp` com espaço no path (set -e) | 3.2 | 1 | 0 | não | CLEAN |
| `mktemp` com espaço no path (set -e) | 5 | 1 | 0 | não | CLEAN |
| `mktemp` path com aspas simples (set -e) | 3.2 | 1 | 0 | não | CLEAN |
| `mktemp` path com aspas simples (set -e) | 5 | 1 | 0 | não | CLEAN |

¹ bash 3.2: apenas `UNDEFINED_VAR: unbound variable` (a falha real); sem erro do trap.

### Item 7 — `check-validate-rule-pins.sh`: `${FENCE_TMP:-}` antes de FENCE_TMP existir

| Cenário | bash | rc | Output | Resultado |
|---------|------|----|--------|-----------|
| Script sai antes da linha 909 (`exit 0` explícito) | 3.2 | 0 | silencioso | SAFE |
| Script sai antes da linha 909 (`exit 0` explícito) | 5 | 0 | silencioso | SAFE |
| Script aborta por `false` antes da linha 909 (set -e) | 3.2 | 1 | silencioso | SAFE |
| Script aborta por `false` antes da linha 909 (set -e) | 5 | 1 | silencioso | SAFE |

`${FENCE_TMP:-}` expande para string vazia quando a variável não existe (set -u não dispara
para expansão com `:-`). `rm -rf "$TMP_DIR" ""` em macOS: `rm -rf ""` retorna rc=0 sem remover
nada (confirmado — dir corrente intacto). Sem risco.

---

## Achados

### Achado 1 — `trap -- 'body $local' EXIT` não detectado pelo gate (FN) — LOW

**Sítio:** `scripts/check-no-trap-local-var.sh`, função `store_trap` no AWK.

**Mecanismo:** `store_trap` procura o body do trap exigindo que o primeiro caractere após
`trap[[:space:]]+` seja uma aspas simples:
```awk
if (substr(rest, 1, 1) != sq) return
```
Para `trap -- 'rm -rf "$tmp"' EXIT`, `rest` começa com `--`, não com `'`. O body é ignorado.

**Medido:** fixture `t6_trap_dd_confirm.sh` com `local tmp` e `trap -- 'rm -rf "$tmp"' EXIT`
→ gate rc=0 (sem violação). Esperado: rc=1.

**Severidade:** LOW. `trap --` é válido em bash como separador de fim de opções antes do
body, mas **zero instâncias** desta forma existem nos scripts trackeados:
`git ls-files '*.sh' | xargs grep 'trap -- '` → vazio (exceto comentários irrelevantes).
Os scripts corrigidos usam `trap "rm -rf -- $q" EXIT` onde `--` está dentro do body
(flag de `rm`), não antes do body.

**Correção sugerida (para o arquiteto avaliar):** em `store_trap`, após `if (substr(rest,1,1) != sq)`,
adicionar tratamento para `--`:
```awk
if (substr(rest, 1, 2) == "--") {
    sub(/^--[[:space:]]+/, "", rest)
}
if (substr(rest, 1, 1) != sq) return
```

---

### Achado 2 — Função de uma linha não detectada pelo gate (FN) — LOW

**Sítio:** `scripts/check-no-trap-local-var.sh`, lógica de detecção de função no AWK.

**Mecanismo:** quando a assinatura e o corpo inteiro estão na mesma linha —
`f() { local x; x=$(mktemp -d); trap 'rm -rf "$x"' EXIT; }` —, o fluxo é:
1. `is_func=1`, `{` encontrado na linha → `in_func=1`, `brace_depth=0`, coleções limpas.
2. Bloco "within function": `brace_depth += brace_net(line)`. O `brace_net` da linha inteira
   retorna 0 (uma `{` abre, uma `}` fecha). `brace_depth = 0 → ≤ 0`.
3. `flush_traps` é chamado imediatamente com `n_traps=0` e `lvars` vazio — nada a checar.
4. `collect_lvars` e `store_trap` **não são chamados** para essa linha.

**Medido:** fixture `t16_oneliner.sh` → rc=0 (sem violação). Esperado: rc=1.

**Severidade:** LOW. **Zero instâncias** de função de uma linha com trap no corpus trackeado.
Os helpers de uma linha existentes nos scripts (`st_pass()`, `st_fail()`) não contêm trap.

**Correção sugerida (para o arquiteto avaliar):** quando `brace_depth` cai a 0 no mesmo
processamento em que a função foi aberta (ou seja, foi uma one-liner), processar
`collect_lvars` e `store_trap` antes de `flush_traps`. Alternativa mais simples: detectar
one-liners separadamente e processar o conteúdo entre `{` e `}` antes de fechar.

---

### Achado 3 — CRLF: detecção correta, sem falso positivo (informativo)

Arquivo com `\r\n` em todas as linhas: gate detecta a violação corretamente (T9).
Arquivo CRLF com a forma corrigida: gate não acusa (T9b, rc=0).
O `\r` no final da linha não confunde o delimitador de heredoc nem o `brace_net`.
Nenhuma ação necessária — registrado para confirmar robustez.

---

### Achado 4 — `rm -rf ""` não apaga o diretório corrente em macOS (informativo)

`${FENCE_TMP:-}` expande para `""` quando `FENCE_TMP` não está definida. `rm -rf ""` em macOS:
rc=0, nenhum arquivo ou diretório removido (medido com dir criado como CWD). Sem risco de
remoção acidental de dados.

---

## Estado do repositório ao final da auditoria

```
$ git status --short
(vazio)

$ git diff --cached --name-only
(vazio)
```
