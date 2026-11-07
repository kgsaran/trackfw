# Wave 0 — Modelo de Ameaça: trap EXIT que cita variável local

> Hades — 2026-11-07
> REQ: `docs/req/REQ-2026-11-07-trap-exit-de-self-test-cita-variavel-local-e-mascara-a-falha-real.md`
> Roadmap: `docs/roadmaps/wip/ROADMAP-2026-11-07-trap-exit-de-self-test-cita-variavel-local-e-mascara-a-falha-real.md`
> ML: 0A — ações 1–4

---

## 1. Completude da enumeração

**Pergunta:** os dois sítios declarados na REQ são todos os sítios do mecanismo?

**Resposta:** não. A REQ declara dois sítios; a enumeração encontrou três.

### Protocolo

Corpus varrido com:
```
git grep -nE 'trap[[:space:]]' -- ':!*.md' ':!*_test.go' ':!*.json' ':!*.txt' ':!*.py' ':!*.ps1'
```
(o `:!*_test.go` evita o bloco do credential guard em fixtures JWT que bloquearia grep em glob.)

Resultado: 76 linhas. Nenhum arquivo com shebang bash fora de `scripts/*.sh` foi encontrado.
Arquivos Go: `grep -rn "trap " internal/ --include='*.go'` → zero ocorrências de shell (a palavra
aparece apenas em comentários de documentação). Makefile: zero traps. Workflows GitHub: zero traps.

**Discriminador aplicado:** `trap` na coluna 0 está definitivamente em escopo global por
construção. `trap` indentado requer verificação de escopo — foram verificados manualmente os 8
casos indentados.

### Tabela completa

| Arquivo | Linha | Coluna | Escopo | Variável | Local? | Status |
|---------|-------|--------|--------|----------|--------|--------|
| `check-shim-diff-applies.sh` | 178, 188 | ind. | função `run_self_test()` | `$tmp`, `$saved_github_output` | SIM | **SITE A — BUG ATIVO** |
| `check-req-roadmap-rule-is-status-aware.sh` | 232 | ind. | função `self_test()` | `$WORK` | SIM | **SITE B — RISCO ESTRUTURAL** |
| `check-orphan-gates.sh` | 142 | ind. | função `self_test()` | `$WORK` | SIM | **SITE C — RISCO ESTRUTURAL** |
| `check-adr-enumeration-single-point.sh` | 371 | ind. | bloco `if` global | `$TMPDIR_ST` | NÃO | SEGURO |
| `check-agent-models-parity.sh` | 74 | col.0 | global | `$WORK` | NÃO | SEGURO |
| `check-agent-namespace-union.sh` | 107 | col.0 | global | `$WORK` | NÃO | SEGURO |
| `check-barrier.sh` | 29 | col.0 | global | `$WORK` | NÃO | SEGURO |
| `check-channels-content.sh` | 595, 1371, 1484 | ind. | bloco `if` global (helpers) | `$CLEANUP_WORK`, `$WORK` | NÃO | SEGURO |
| `check-ci-workflow-job-id-collision.sh` | 75 | col.0 | global | `$WORK` | NÃO | SEGURO |
| `check-ci-workflow-pin-parity.sh` | 35 | col.0 | global (helper `cleanup()`) | `$WORK`, `$DUMP_TEST` | NÃO | SEGURO |
| `check-falsify-chunk-timeout.sh` | 43 | col.0 | global | `$SCRATCH` | NÃO | SEGURO |
| `check-falsify-recalibrate.sh` | 34 | col.0 | global | `$SCRATCH` | NÃO | SEGURO |
| `check-falsify-shard-coverage.sh` | 67 | col.0 | global | `$WORKDIR` | NÃO | SEGURO |
| `check-gates-falsify.sh` | 34 | col.0 | global | `$WORK` | NÃO | SEGURO |
| `check-gates-falsify.sh` | 94 | col.0 | global (sinal `ERR`) | — | NÃO | SEGURO¹ |
| `check-git-branch-guard-hook-schema.sh` | 93 | col.0 | global | `$WORK` | NÃO | SEGURO |
| `check-goreleaser-prerelease.sh` | 100 | ind. | bloco `if` global | `$WORK` | NÃO | SEGURO |
| `check-init-preserves-user-config.sh` | 209 | ind. | bloco `if` global | `$TMPDIR_ST` | NÃO | SEGURO |
| `check-install-checksum.sh` | 71 | col.0 | global | `$WORK` | NÃO | SEGURO |
| `check-install-restriction.sh` | 91 | col.0 | global | `$WORK` | NÃO | SEGURO |
| `check-install-version-pin.sh` | 40 | col.0 | global | `$WORK` | NÃO | SEGURO |
| `check-integration-assets.sh` | 7 | col.0 | global | `$TMP_ROOT` | NÃO | SEGURO |
| `check-jira-url-concat.sh` | 44 | ind. | bloco `if` global | `$TMPDIR_SELF` | NÃO | SEGURO |
| `check-no-grep-q-under-pipefail.sh` | 53 | ind. | bloco `if` global | `$tmpdir` | NÃO | SEGURO |
| `check-no-literal-nul-in-source.sh` | 73 | ind. | bloco `if` global | `$WORK` | NÃO | SEGURO |
| `check-parity-call-site-pins.sh` | 121 | col.0 | global | `$RECIPE_LINES_FILE` | NÃO | SEGURO |
| `check-platform-matrix-parity.sh` | 48 | col.0 | global (helper `cleanup()`) | — | NÃO | SEGURO |
| `check-pr-closing-keyword.sh` | 212 | col.0 | global | `$WORK` | NÃO | SEGURO |
| `check-release-tag-parity.sh` | 70 | col.0 | global | `$WORK` | NÃO | SEGURO |
| `check-roadmap-barrier-contract.sh` | 61 | col.0 | global | `$WORK` | NÃO | SEGURO |
| `check-roadmap-slug-matching.sh` | 68 | col.0 | global | `$WORK` | NÃO | SEGURO |
| `check-rules-parity.sh` | 36 | col.0 | global | `$WORK` | NÃO | SEGURO |
| `check-serve-address-parity.sh` | 74 | col.0 | global (helper) | — | NÃO | SEGURO |
| `check-serve-api-file-security.sh` | 45 | col.0 | global | `$WORK` | NÃO | SEGURO |
| `check-serve-browser-security.sh` | 26 | col.0 | global | `$WORK` | NÃO | SEGURO |
| `check-shim-byte-identity.sh` | 145 | col.0 | global | `$WORK` | NÃO | SEGURO |
| `check-slash-parity.sh` | 36 | col.0 | global | `$WORK` | NÃO | SEGURO |
| `check-static-assets.sh` | 7 | col.0 | global | `$TMP_ROOT` | NÃO | SEGURO |
| `check-symlink-privilege-guard.sh` | 46 | ind. | bloco `if` global | `$TMP` | NÃO | SEGURO |
| `check-thirdparty-parity.sh` | 33 | col.0 | global | `$WORK` | NÃO | SEGURO |
| `check-tty-detection.sh` | 48 | col.0 | global | `$WORK` | NÃO | SEGURO |
| `check-update-parity.sh` | 50 | col.0 | global | `$WORK` | NÃO | SEGURO |
| `check-usage-silencing.sh` | 95 | ind. | bloco `if` global | `$tmp` | NÃO | SEGURO |
| `check-validate-rule-pins.sh` | 35, 910 | col.0 | global | `$TMP_DIR`, `$FENCE_TMP` | NÃO | SEGURO² |
| `check-wheel-filename.sh` | 48 | col.0 | global (helper) | — | NÃO | SEGURO |
| `check-wheel-metadata-readme.sh` | 53 | col.0 | global (helper) | — | NÃO | SEGURO |
| `check-write-containment.sh` | 59 | ind. | bloco `if` global | `$TMP` | NÃO | SEGURO |
| `falsify-recalibrate.sh` | 56 | col.0 | global | `$TMP` | NÃO | SEGURO |
| `install.sh` | 193 | col.0 | global (sinal `0`) | `$TMP_DIR` | NÃO | SEGURO |
| `run-gates-falsify-parallel.sh` | 131 | col.0 | global | `$WORKDIR` | NÃO | SEGURO |
| `run-gates-falsify-shard.sh` | 84 | col.0 | global | `$WORKDIR` | NÃO | SEGURO |
| `smoke-integration-packages.sh` | 6 | col.0 | global | `$TMP_ROOT` | NÃO | SEGURO |

¹ `check-gates-falsify.sh:94` usa sinal `ERR`, não EXIT. Variável global. Documentado em ML-2E:
  "o slot de EXIT é do `rm -rf "$WORK"` e um segundo trap de EXIT o SUBSTITUIRIA, vazando o
  diretório temporário" — por isso usa ERR. Fonte: `scripts/check-gates-falsify.sh:45–61`.

² `check-validate-rule-pins.sh:910` substitui `:35` → `$TMP_DIR` vaza. Variáveis são globais.
  Mecanismo diferente desta REQ; ver Seção 4, R6.

**Site C não está declarado na REQ.** A REQ declara "dois sítios"; são três. Pelo mecanismo
(mesma causa: `local var` + `trap EXIT` dentro de função), pela Regra Dura de Causa Raiz, Site C
deve entrar na mesma REQ.

**Enumeração fechada:** todos os 51 registros do corpus foram classificados. Os três sítios
vulneráveis são os únicos com `local` + `trap EXIT` dentro de bloco de função.

---

## 2. Modelo de ameaça

O adversário aqui é o **implementador apressado** e o **arquiteto otimista**, não um atacante externo.

### Mecanismo central — três caminhos de saída

`trap 'CMD' EXIT` registra CMD para o momento em que o **processo** termina. Quando a função
`f()` está ativa e há um local `$tmp`, o comportamento depende de como o processo termina:

**Caminho 1 — `exit N` explícito de dentro de `f()`:**
O processo termina com o frame de `f()` ainda ativo na pilha. EXIT trap dispara com escopo local
acessível. CLEAN em ambas as versões.

**Caminho 2a — `set -e` causa saída do processo (comando não-zero fora de `if`/`&&`/`||`/`!`):**
O shell sai. Em bash 5.3, a pilha de chamadas é desmontada *antes* do EXIT trap disparar — `$tmp`
fica desvinculado → LEAK. Em bash 3.2, EXIT trap dispara *antes* da desmontagem da pilha →
`$tmp` acessível → CLEAN. Este é o único trigger em que bash 3.2 se comporta diferente.

**Caminho 2b — `set -u` causa saída do processo (variável não declarada é expandida):**
O shell sai diretamente, desmontando a pilha em **ambas as versões** → LEAK em ambas. Distinto
do Caminho 2a: a desmontagem de pilha antes de EXIT trap vale para set -u em ambas as versões.

**Caminho 3 — `f()` retorna normalmente, processo sai depois:**
O escopo local de `f()` é desmontado no momento do retorno da função. Quando o processo
eventualmente termina (por outro `exit` no nível superior ou pelo fim do script), EXIT trap
dispara — `$tmp` já está desvinculado em **ambas as versões** → LEAK.

### Matriz de triggers medida

Medições com `"$INTERP" script.sh` (isolando o interpretador — m5 + m6). Cada child script escreveu
`$BASH_VERSION` como primeira linha confirmando o interpretador ativo. Diretórios temporários
criados em SCRATCHPAD para rastreio de vazamento por existência de arquivo.

| Trigger | bash 5.3 | bash 3.2 | Caminho | Evidência |
|---------|----------|----------|---------|-----------|
| `false` sob `set -e` | LEAK + "unbound variable" | CLEAN | 2a | m5-T1 |
| `local undef; echo "$undef"` sob `set -u` | LEAK + 2 erros | LEAK (rc=0)† | 2b em 5.3; 3 em 3.2† | m5-T2 |
| var não declarada (`$UNDEF`) sob `set -u` | LEAK + 2 erros | LEAK + 2 erros | 2b (ambas) | m5-T3 |
| aritmética `$(( UNDEF + 1 ))` | CLEAN | CLEAN | Aritmética não abandona a função | m5-T4 |
| `y=$UNDEF` | LEAK + 2 erros | LEAK + 2 erros | 2b (ambas) | m5-T5 |
| `$()` com var não declarada | LEAK + 2 erros (rc=0) | LEAK (rc=0) | 3 (ambas)‡ | m5-T6 |
| `exit N` direto de dentro da função | CLEAN | CLEAN | 1 | m6-T10 |
| `${tmp:-}` bypass silencioso | LEAK sem stderr | CLEAN | 2a em 5.3; 1 em 3.2 | m5-T9 |

† bash 3.2 não aborta para `local undef; echo "$undef"` (variável local declarada-mas-não-atribuída
é tratada como vazia) → função retorna normalmente → Caminho 3 → LEAK. rc=0 porque nenhum set-u
foi disparado.

‡ set -u em `$()` cria erro no subshell, mas o subshell não propaga a saída para a função pai sob
set -u sozinho (sem set -e). A função retorna normalmente → Caminho 3.

**Revisão em relação à tabela da REQ:** as três linhas da REQ estão confirmadas. A REQ não inclui
Caminho 3 (retorno normal) como trigger — adicionado aqui.

### Site A — BUG ATIVO: `run_self_test()` em `check-shim-diff-applies.sh`

Script usa `set -euo pipefail` (linha 18). A função:

```
run_self_test() {
  local tmp                                                                    ← local
  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' EXIT                                                   ← primeira trap
  local saved_github_output="${GITHUB_OUTPUT:-}"
  unset GITHUB_OUTPUT
  trap 'rm -rf "$tmp"; [[ -n "$saved_github_output" ]] && export GITHUB_OUTPUT="$saved_github_output"' EXIT
  ...
  git -C "$gitdir" init -q                                                    ← comando bare sob set -e
  git -C "$gitdir" config user.email "test@test.test"                         ← idem
  git -C "$gitdir" write-tree                                                 ← idem
  ...
  exit 1    (linhas 680)
  exit 0    (linha 682)
}
```

A função termina com `exit 0`/`exit 1` explícitos (linhas 680 e 682, confirmados por awk).
Esses caminhos são CLEAN (Caminho 1). O bug é ativado pelos git commands bare (linhas 115+)
que podem falhar sob `set -e` antes de chegar ao `exit` explícito (Caminho 2a).

**Evidência de ocorrência real:** issue #671 registra a saída observada:
"line 1: tmp: unbound variable" — exatamente a assinatura do Caminho 2a no bash 5.3
(EXIT trap tenta acessar `$tmp` após o frame da função ter sido desmontado). A reprodução
mínima documentada no issue confirma o mecanismo.

**GITHUB_OUTPUT é duplamente inoperante** (m3, confirmado em subprocesso isolado):

```
EXIT trap: saved=UNSET
check-shim-diff-applies.sh: line 1: saved_github_output: unbound variable
parent: GITHUB_OUTPUT=UNSET
```

(a) `$saved_github_output` é `local` → desvinculado quando EXIT dispara. (b) Mesmo se rodasse:
`export GITHUB_OUTPUT=...` em um processo que está terminando não propaga para o pai (confirmado:
"parent: GITHUB_OUTPUT=UNSET"). A lógica de restauração deve ocorrer antes do retorno da função.

**RC mascarado (bash 5.3):** quando o trap body falha com "unbound variable" sob set -u, o rc
do processo pode ser substituído pelo erro da trap (m5-T7: `(exit 7)` → rc=1 ao invés de rc=7).
Esse mascaramento não ocorre uniformemente — m5-T6 mostrou rc=0 preservado. O fix do mecanismo
principal elimina o efeito secundário.

### Sites B e C — RISCO ESTRUTURAL

Ambos usam `set -uo pipefail` (sem `-e`). Seus `self_test()` terminam com `exit 0`/`exit 1`
explícitos (confirmado por awk para Site C: linhas 257 e 260 estão dentro da função; para Site
B: linhas 437 e 440 estão dentro). O Caminho 1 atual é CLEAN.

**Por que estes sítios são RISCO ESTRUTURAL, não apenas "se set -e for adicionado":**

1. Caminho 2b (var não declarada sob set -u — m5-T3, T5): LEAK em **ambas as versões**, já hoje,
   sem precisar de set -e. Qualquer uso de variável não declarada dentro de `self_test()` ativa
   o bug.
2. Caminho 2a: a adição de set -e (padrão do projeto em todos os novos scripts) ativa Caminho 2a
   especificamente em bash 5.3.

O fix deve ser aplicado em todos os três sítios.

### Correção: `printf %q`

m2c-v2 mediu as formas candidatas com **ambas** as versões do bash, usando `"$INTERP" child.sh`
(cada child script imprimiu `$BASH_VERSION` para confirmar o interpretador ativo):

| Forma | bash | Espaço | Aspas simples | `$` | Resultado |
|-------|------|--------|--------------|-----|-----------|
| `printf -v q '%q' "$tmp"; trap "rm -rf $q" EXIT` | 5.3 | CLEAN | CLEAN | CLEAN | Recomendada |
| `printf -v q '%q' "$tmp"; trap "rm -rf $q" EXIT` | 3.2 | CLEAN | CLEAN | CLEAN | Recomendada |
| `trap "rm -rf '$tmp'" EXIT` (double-quote+sq-wrap) | 5.3 | CLEAN | LEAK (syntax, rc=2) | não medido | Insegura |
| `trap "rm -rf '$tmp'" EXIT` (double-quote+sq-wrap) | 3.2 | CLEAN | LEAK (syntax, rc=2) | não medido | Insegura |
| `trap "rm -rf $tmp" EXIT` (sem quoting) | 5.3 | LEAK (word split) | n/a | n/a | Insegura |
| `trap "rm -rf $tmp" EXIT` (sem quoting) | 3.2 | LEAK (word split) | n/a | n/a | Insegura |

A forma `printf %q` é a única que cobre todos os casos em ambas as versões.

**Bypass silencioso via `${tmp:-}` (m5-T9):** em bash 5.3, `${tmp:-}` expande para string vazia →
`rm -rf ""` = no-op; LEAK sem stderr. Em bash 3.2: Caminho 1 preservado → CLEAN. Este bypass é
uma ameaça real de degradação discreta.

---

## 3. Alvos de falsificação em ambas as direções

### Sites A, B, C — direção falso negativo (gate não detecta)

Resultados medidos (bash 5.3, `"$INTERP" script.sh`, dirs criados em SCRATCHPAD):

| Variante | Comportamento | Gate deve flagrar | Evidência |
|----------|--------------|-------------------|-----------|
| FN-1: `${VAR}` (com chaves) | LEAK confirmado (dir vivo pós-run) | SIM | m4-FN1-v2 |
| FN-2: `local -r VAR` | LEAK confirmado | SIM | m4-FN2-v2 |
| FN-3: `declare VAR` dentro de função | LEAK confirmado | SIM | m4-FN3-v2 |
| FN-4: `typeset VAR` dentro de função | LEAK confirmado | SIM | m4-FN4-v2 |
| FN-5: `function f { }` estilo | LEAK confirmado | SIM | m4-FN5-v2 |
| FN-6: `local` declarado APÓS `trap` | LEAK confirmado | SIM (escopo vale para toda a função) | m4-FN6-v2 |
| FN-7: trap com múltiplas vars locais | LEAK confirmado | SIM (cada var) | m4-FN7-v2 |

Gate FN não cobertos (declarados como residuais em Seção 4):
- Trap body é uma chamada de função helper que usa local via dynamic scope (m6-T15, LEAK em 5.3)
- `trap '...' 0` (sinal numérico, equivalente a EXIT) se gate só verifica o token `EXIT`
- `${VAR:-}` (bypass silencioso) se gate só verifica `\$VAR` e `\$\{VAR\}`

### Sites A, B, C — direção falso positivo (gate bloqueia código seguro)

| Variante | bash | Comportamento real | Gate deve flagrar? | Evidência |
|----------|------|-------------------|-------------------|-----------|
| FP-1: trap em escopo global, var global | ambas | CLEAN | NÃO | m4-FP1 |
| FP-2: `f() ( ... )` subshell | 5.3 | CLEAN (EXIT dispara dentro do subshell) | NÃO | FP-2-v2 |
| FP-2: `f() ( ... )` subshell | 3.2 | CLEAN | NÃO | FP-2-v2 |
| FP-3: trap em função, var NÃO local (global) | ambas | CLEAN | NÃO | m4-FP3 |
| FP-4: `trap "..." EXIT` double-quote externo | 5.3 | CLEAN | NÃO (expansão no momento da instalação) | m4-FP4 |
| FP-4: `trap "..." EXIT` double-quote externo | 3.2 | CLEAN | NÃO | FP-4-v2 |
| FP-5: `declare -g VAR` | 5.3 | CLEAN (global — trap acessa sem erro) | NÃO | dg-test53 |
| FP-5: `declare -g VAR` | 3.2 | rc=2 (opção inválida em bash < 4.2) | NÃO (gate deve ignorar `-g`) | dg-test-32 |

### Vetores de regressão para ML-1A

ML-1A deve incluir ao menos estes vetores, invocando o interpretador explicitamente
(`/bin/bash` e `/opt/homebrew/bin/bash`), não pelo PATH:

| Vetor | Versão(ões) | Comportamento esperado pós-fix |
|-------|-------------|-------------------------------|
| `false` sob set -e (Caminho 2a) | 5.3 | CLEAN |
| var não declarada sob set -u (Caminho 2b) | 3.2 + 5.3 | CLEAN |
| Retorno normal da função seguido de exit (Caminho 3) | 3.2 + 5.3 | CLEAN |
| `exit 1` explícito dentro da função (já seguro) | ambas | CLEAN (regressão) |
| Path com espaço | 5.3 | CLEAN |
| Path com aspas simples | 5.3 | CLEAN |

---

## 4. Residual declarado

**R1 — bash 3.2 é SAFE para Caminho 2a, mas não para Caminhos 2b e 3**

bash 3.2 (padrão no macOS) preserva o escopo local no Caminho 2a (set -e causa saída do
processo) → m5-T1 CLEAN. MAS: Caminho 2b (set -u com var não declarada) LEAKA em bash 3.2
(m5-T3, T5 LEAK). E Caminho 3 (retorno normal) LEAKA em ambas as versões. O bug não é
exclusivo do CI — é reprodutível no macOS via Caminhos 2b e 3. A correção com `printf %q`
é neutra em ambas as versões.

**R2 — Trap helper via dynamic scope não é detectado pelo gate simples**

```bash
cleanup() { rm -rf "$tmp"; }   # acessa $tmp do chamador via dynamic scope
f() {
  local tmp=$(mktemp -d)
  trap 'cleanup' EXIT           # body é só o nome da função, sem $tmp
  false
}
```

LEAK em bash 5.3 (m6-T15 confirmado). Gate baseado em texto não detecta esta forma (a trap
body não contém `$tmp`). Aceitável: padrão não ocorre em nenhum dos três sítios do corpus atual.

**R3 — `${VAR:-}` não coberto pelo gate simples**

m5-T9 confirmou: `${tmp:-}` produz LEAK sem stderr em bash 5.3. Gate que só verifica `\$VAR`
e `\$\{VAR\}` não detecta esta forma. O gate para ML-1A deve incluir um vetor explícito para
`${VAR:-}`, ou declará-la como FN nomeado na AC com texto de aviso para revisores.

**R4 — `trap RETURN` tem mesmo comportamento de leak**

m6-T12 mediu `trap '...$local...' RETURN` — LEAK em ambas as versões. Padrão não ocorre no
corpus atual. Residual aceitável.

**R5 — RC mascarado (bash 5.3, caminhos específicos)**

Quando o trap body falha com "unbound variable", rc pode ser substituído pelo erro da trap.
Medido em m5-T7 (rc=7 → rc=1). Não uniforme — m5-T6 mostrou rc=0 preservado. A correção do
mecanismo principal elimina o efeito secundário.

**R6 — Sobreposição de trap em `check-validate-rule-pins.sh:35→910`**

Mesmo sintoma (diretório temporário vaza): linha 910 substitui linha 35, `$TMP_DIR` vaza.
Causa distinta: sobreposição de trap em escopo global — sem variável local envolvida.
Este repositório já foi afetado por exatamente este padrão: `check-gates-falsify.sh:45–61`
documenta o ML-2E onde um trap dentro de um bloco de cenário era substituído por um segundo
trap EXIT, causando vazamento de `$WORK`. A decisão de abrir REQ separada para a sobreposição
global pertence ao arquiteto; o presente Wave 0 apenas registra o caso como residual com o
mesmo sintoma mas mecanismo distinto.

---

## Veredito

**APROVA COM RESSALVAS**

ML-1A pode prosseguir. Os itens abaixo são obrigatórios.

### Itens obrigatórios para o ML-1A

1. **[PRÉ-CONDIÇÃO — Regra Dura de Causa Raiz]** Widening da REQ: adicionar Site C
   (`check-orphan-gates.sh:140-142`) à lista de sítios antes do primeiro commit de ML-1A.
   A REQ atualmente declara "dois sítios"; são três.

2. **Site A — fix do trap body em `check-shim-diff-applies.sh`**: substituir o trap atual
   (linha 188) por:
   ```bash
   local q; printf -v q '%q' "$tmp"
   trap "rm -rf $q" EXIT
   ```
   A expansão de `$tmp` ocorre no momento da instalação (double-quote). `printf %q` trata
   espaços, aspas simples e `$` corretamente em bash 5.3 e 3.2 (m2c-v2 confirmado com
   interpreter explícito em ambas as versões).

3. **Site A — GITHUB_OUTPUT**: remover completamente a lógica de restauração de
   `GITHUB_OUTPUT` do trap body. Ela é inoperante por dois motivos independentes (m3): (a)
   `$saved_github_output` é `local`, desvinculado quando EXIT dispara; (b) `export` em processo
   terminando não propaga ao pai. Se a restauração for necessária, deve ocorrer antes do
   retorno da função.

4. **Sites B e C — fix do mesmo mecanismo**: aplicar o mesmo padrão:
   ```bash
   local q; printf -v q '%q' "$WORK"
   trap "rm -rf $q" EXIT
   ```

5. **Gate `check-trap-local-var.sh`** deve:
   - Flagrar: `trap '...$VAR...' EXIT`, `trap '...$VAR...' 0`, `trap '...$VAR...' EXIT HUP INT TERM`
     quando `VAR` foi declarado com `local`/`local -r`/`declare`/`typeset` (sem `-g`) na mesma
     função (`f() { }` ou `function f { }`)
   - Flagrar também: forma `${VAR}` com chaves e `${VAR:-}` (bypass silencioso — se não detectado,
     declarar como FN nomeado com nota de alerta)
   - NÃO flagrar: (a) trap em escopo global, (b) trap com expansão double-quote, (c)
     `declare -g`/`typeset -g` (bash 5.3: global, CLEAN; bash 3.2: opção inválida — de qualquer
     forma, não é vulnerabilidade), (d) função subshell `f() ( )`

6. **Vetores de regressão**: cada vetor da tabela na Seção 3 deve ter um teste correspondente no
   gate ou nos testes de integração. Invocar o interpretador **explicitamente** com `/bin/bash` e
   com `$(command -v bash)` (ou caminho homebrew) — nunca via PATH. Invocar pelo PATH usa o
   interpretador do ambiente de execução, que pode mascarar regressões versão-específicas
   (a mesma armadilha que invalidou os primeiros rascunhos deste Wave 0: `bash x.sh` ignora o
   shebang e usa o interpretador do chamador).

7. **Regra de reconciliação**: para cada teste novo entregue no ML-1A, o relatório deve declarar
   qual conclusão deste Wave 0 o teste afirma, em uma frase.

---

## Verificação de estado do repositório

```
$ git status --short
?? docs/seguranca/2026-11-07-wave0-trap-exit-variavel-local.md

$ git diff --cached --name-only
(vazio)
```
