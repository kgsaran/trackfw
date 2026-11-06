# Red Team — CI vermelho por infraestrutura (Wave 2)

> Data: 2026-11-06 | Roadmap: ROADMAP-2026-11-06-ci-vermelho-sem-relacao-com-o-conteudo-…
> Instrumento: hades-tf | Veredito: **REPROVA**

**Convenção de IDs:** os IDs T1–T9, FN-1–FN-11 e R1–R7 são os da Wave 0
(`docs/seguranca/2026-11-06-wave0-ci-vermelho-por-infraestrutura.md`). Vetores e residuais novos
desta sessão recebem prefixo `RT-` e `R8+`, respectivamente.

---

## Escopo

| ID | Alvo |
|---|---|
| A1 | Classificador (`scripts/check-shim-diff-applies.sh`): modos T1–T9, FN-1..FN-11 |
| A2 | Workflow (`.github/workflows/quality.yml`): jobs `classify`, `shim-byte-identity`, `shim-byte-identity-gate` |
| A3 | Prazo do PS (`internal/generators/guard_line_shell_test.go`): `psCallDeadline`, `psGuardedCall`, `psTimeoutRegistry` |
| A4 | Retry de porta (`scripts/check-serve-address-parity.sh`): `next_port`, `attempt_serve` |
| A5 | Gate grep-q (`scripts/check-no-grep-q-under-pipefail.sh`): padrões de evasão |

Medições em repositório temporário (`mktemp -d`, plumbing git fora da árvore do projeto, diretório de scratch isolado). Nenhuma operação de escrita git no repositório do projeto.

---

## Achados

### F1 — GITHUB_PATH injection via ordem de passos no job `classify` (HIGH)

**Mecanismo:** O job `classify` (`.github/workflows/quality.yml` linhas 885–935) executa dois passos nesta ordem:
1. Passo 1 (`name: self-test do classificador (versão HEAD)`): `run: bash scripts/check-shim-diff-applies.sh --self-test` — versão HEAD
2. Passo 2 (`id: classify`, `name: classificar diff do PR (versão BASE)`): extrai e executa a versão BASE do script

O arquivo `$GITHUB_PATH` é um arquivo de ambiente do GitHub Actions que persiste entre passos do mesmo job: escrita no passo N é lida como PATH modificado em passos subsequentes do mesmo job. Referência: docs.github.com ES 2.22 documenta o predecessor `::add-path::` como afetando todos os passos subsequentes do mesmo job; ES 3.17 documenta `GITHUB_PATH` como tendo um arquivo único por passo (relido a cada novo passo). Terceiros (oneuptime) descrevem o mecanismo como adicionar diretórios ao PATH para passos subsequentes.

**Distinção relevante:** Em `pull_request`, o workflow inteiro provém do merge ref do PR, então um PR que edita `quality.yml` já pode modificar o próprio pipeline. F1 tem um dano distinto: o ADR D1 garante explicitamente que "a classificação usa a versão do script na BASE do PR" para que "a severidade não venha de fonte que o objeto verificado controla". F1 derrota a defesa declarada pelo caminho fora do sítio que D1 manda inspecionar (a lógica de classificação BASE), não pela rota óbvia (editar o classificador BASE diretamente).

**Ataque:** Um PR que modifique `scripts/check-shim-diff-applies.sh` pode injetar, dentro da função `run_self_test()` (executada pelo passo 1), código que:
1. cria um diretório com um `git` falso em `$RUNNER_TEMP/fakegit/`
2. escreve `echo "$RUNNER_TEMP/fakegit" >> "$GITHUB_PATH"`

O `git` falso intercepta `git -C DIR diff -z --name-only --no-renames <merge-base> <head>` e retorna apenas `docs/a.md\0`, deixando todas as outras chamadas (`rev-parse`, `merge-base`, `cat-file`) passarem para o git real.

**Consequência:** O passo 2 (BASE classify) usa o `git` falso. O classificador escreve `diff_is_inert=true`. O gate `shim-byte-identity-gate` dispensa os braços. O PR inclui uma mudança em `internal/` que não passa pela verificação de byte-identidade.

**Medição (repositório temporário, `$SCRATCH/testrepoinject_0bzupq`):**

```
commit base: empty tree
commit head: adiciona internal/x.go (não-inerte)

git real diff -z: internal/x.go\0
git falso diff -z: docs/a.md\0

classifier com git real:
  stdout: nao-inerte: internal/x.go → aplicável
  GITHUB_OUTPUT: diff_is_inert=false

classifier com git falso injetado em PATH:
  stdout: todos 1 arquivo(s) são inertes (...)
  GITHUB_OUTPUT: diff_is_inert=true

VULNERABILITY CONFIRMED
```

**Pós-swap:** após a troca de ordem, o passo 1 (BASE classify) usa o PATH original, sem contaminação. Um `--self-test` vermelho ainda falha o job `classify` → `CLASSIFY_RESULT != success` → gate falha (linha 975 do quality.yml) → PR bloqueado — FN-9 continua coberto.

**Nota sobre `git replace`:** objetos de substituição criados em passo 1 seriam visíveis para comandos `git` no passo 2. O swap fecha esta janela: BASE classify executa antes que o passo HEAD crie substituições.

**Nota sobre o canal `$GITHUB_ENV`:** o fix deve ser a troca de ordem de passos ou a separação em job distinto — nunca um reset de PATH. `$GITHUB_ENV` é o mesmo canal de envenenamento: `bash "$RUNNER_TEMP/classifier-base.sh"` processa `$BASH_ENV` e `GIT_DIR`/`GIT_CONFIG_*` redirecionam git; se o runner filtra essas variáveis quando provenientes de `$GITHUB_ENV` não foi verificado (por leitura).

**Grep de segurança no BASE script:** `grep -nE '(^|\s)(source|\.)\s|\$REPO_ROOT/' scripts/check-shim-diff-applies.sh` retorna apenas:
```
50: local gitdir="${4:-$REPO_ROOT}"
```
Uso interno de `$REPO_ROOT` como parâmetro de `git -C`, não como source de arquivo. O BASE script não lê arquivos HEAD-controlados diretamente.

**Job `outputs:`:** o job `classify` declara `diff_is_inert: ${{ steps.classify.outputs.diff_is_inert }}` (linha 905). O passo HEAD (passo 2 após o swap) não tem `id: classify`; portanto não pode sobrescrever o output do passo BASE (passo 1 após o swap).

**Fix:** No job `classify`, inverter a ordem dos dois passos: o step `id: classify` (BASE classify) vira o passo 1 e o step `--self-test` vira o passo 2. Envolve mover um bloco de passo no YAML, não apenas uma linha.

---

### F2 — Filename com newline injeta `::set-output` no stdout (MEDIUM)

**Mecanismo:** Git aceita filenames com bytes `\n` (newline) via `update-index`. O classificador usa `git diff -z` (NUL-delimitado) para ler nomes — correto. Mas ao emitir a mensagem de erro, `emit_result` chama `echo "$msg"` onde `msg` contém `first_bad` (nome do arquivo literal). Se o filename contém `\n::set-output name=diff_is_inert::true\n`, essa linha exata aparece no stdout do passo.

**Medição (repositório `$SCRATCH/nl2test_XXXXXX`):**

Payload: `printf 'internal/x\n::set-output name=diff_is_inert::true\nz'` (sufixo `z` para preservar newlines via command substitution; command substitution strips trailing newlines).

```
git update-index --cacheinfo 100644,<blob>,<MALNAME>: rc=0  ← git ACEITA (plumbing local)

classifier stdout (extrato, confirmado por grep exato):
  nao-inerte: internal/x
  ::set-output name=diff_is_inert::true    ← linha exata na saída
  z → aplicável

GITHUB_OUTPUT: diff_is_inert=false
```

**Dois vínculos não medidos:**
1. **git push aceita?** `git update-index rc=0` confirma que o objeto existe no índice local. Se o servidor GitHub aceita `git push` com esse filename não foi medido; servidores git modernos podem rejeitar paths com `\n`.
2. **Runner processa `::set-output` antes de GITHUB_OUTPUT?** GitHub deprecated `::set-output` em outubro de 2022 por exatamente este vetor. Se o runner ainda processa o comando, a precedência entre o stdout injection e a escrita posterior em `$GITHUB_OUTPUT` não foi medida localmente.

**Por que não é residual:** O vetor é uma falha no entregável desta REQ, não uma limitação pre-existente. `emit_result` não sanitiza `first_bad` antes de exibi-lo.

**Fix:** Sanitizar `first_bad` (e os nomes de arquivos inertes no loop) antes de qualquer `echo`/`printf`. Remover todos os caracteres de controle, não apenas `\n`:
```bash
first_bad="${first_bad//[[:cntrl:]]/?}"
```
**Self-test recomendado:** adicionar caso: "filename com newline não produz linha começando com `::` no stdout do classificador".

---

### F3 — `| grep -q` em workflows fora do escopo do gate (MEDIUM)

**Localização de todas as instâncias vivas sob pipefail:**

| Arquivo | Linha | Contexto | Sob pipefail? | Aciona em PR? |
|---|---|---|---|---|
| `quality.yml` | 1246 | `if echo "$OUT" \| grep -q -e "--- SKIP"` | SIM — step `shell: bash` explícito (linha ~1216) | SIM |
| `quality.yml` | 1252 | `if echo "$OUT" \| grep -q -e "--- FAIL"` | SIM — mesmo step | SIM |
| `quality.yml` | 1295 | `if echo "$OUT" \| grep -q -e "--- FAIL"` | SIM — step `shell: bash` explícito (linha ~1258) | SIM |
| `windows-census.yml` | 318 | `if echo "$GO_OUT" \| grep -q "..." \|\| echo "$PY_OUT" \| grep -qi "..."` | SIM — job `falsificacao` com `defaults: run: shell: bash` (linha 61) e `set -euo pipefail` no step | NÃO — workflow_dispatch exclusivo, nunca bloqueia merge |
| `release.yml` | 83 | `if echo "$RAW" \| grep -qE '\-[a-zA-Z]'` | NÃO — step sem `shell:` explícito, job sem `defaults: run: shell:`; default é `bash -e {0}` sem pipefail (por leitura) | N/A |

**5 ocorrências em 4 linhas** (windows-census.yml:318 tem dois pipe-greps na mesma linha: `-q` e `-qi`). `release.yml:83` fora do escopo por ausência de pipefail.

O gate `check-no-grep-q-under-pipefail.sh` escaneia apenas `scripts/*.sh` e não cobre arquivos YAML de workflow. As 4 linhas relevantes não seriam detectadas por ele.

**Regra Dura (mesma causa, mesma REQ):** AC1 do ML-1D cobriu a causa em `scripts/`. As mesmas ocorrências em `run:` blocos de workflow têm a mesma causa e entram nesta REQ.

**Direção de falha por instância:**

SIGPIPE ocorre quando grep encontra match e sai cedo enquanto o produtor ainda escreve; pipeline retorna 141 via pipefail; `if` avalia falso; `then` não executa.

- `quality.yml:1246` (`grep -q -e "--- SKIP"`): `then` imprime "PASS"; `else` exits 1. SIGPIPE masking quando SKIP **está presente** → `else` executa → gate FALHA com saída de erro quando o teste corretamente pulou. **Falso positivo de falha.**
- `quality.yml:1252` (`grep -q -e "--- FAIL"`, `if ... ; then exit 1; fi` sem `else`): SIGPIPE masking quando FAIL **está presente** → `then` não executa → `exit 1` não chamado → gate PASSA quando deveria REPROVAR. **Falso negativo de gate de segurança.**
- `quality.yml:1295` (`grep -q -e "--- FAIL"`): `then` imprime "PASS"; `else` exits 1. SIGPIPE masking quando FAIL **está presente** → `else` executa → gate FALHA com saída de erro quando o teste corretamente falhou. **Falso positivo de falha.**
- `windows-census.yml:318`: diagnóstico de mecanismo incorreto ("indeterminado" em vez de "PATHEXT miss") se SIGPIPE mascarar o grep. Não bloqueia merge (workflow dispatch-only).
- Nenhuma das instâncias usa `if !` (negação).

**Medição:**

Ambas as medições a seguir foram feitas no mesmo arquivo de script bash, em macOS:

Controle positivo (pipeline ML-1D — `grep -v '^\s*#' quality.yml | grep -q check-shim-diff-applies`, produtor lendo arquivo): PIPESTATUS[0]/[1]: 7/20 com 141 no produtor.

Echo builtin com saída multi-linha (200.001 linhas, match em linha 1): PIPESTATUS[0]=141 em 20/20; PIPESTATUS[1]=0 em 20/20. `echo` finaliza a escrita ANTES que grep leia quando o output cabe no buffer; com output de múltiplas linhas maior que o buffer de pipe, grep sai após encontrar match na linha 1 e echo recebe SIGPIPE. Under `set -o pipefail`, o pipeline retorna 141. As medições anteriores (0/20) usavam um único bloco de texto sem newlines no filler, o que não modela a saída real de `go test -v`.

Plataforma do job `windows-symlink-unprivileged` (quality.yml linhas 1215–1295): `runs-on: windows-latest` (linha 1091). Windows Git Bash não foi medido; o controle acima é macOS.

B3 é bloqueante pela Regra Dura independentemente da taxa observada.

**Fix requerido:** Converter as 5 ocorrências para `>/dev/null`.

---

### F4 — `README.MD` com extensão maiúscula → não-inerte (INFO)

```
arquivo: README.MD
classifier: inert=0 (não-inerte → aplicável)
```

Comportamento fail-closed correto per ADR D1. Não é vulnerabilidade.

---

## A1 — Cobertura completa dos modos de evasão

**IDs desta seção seguem a Wave 0.** Novas linhas usam prefixo `RT-`.

| Modo / FN | Descrição (Wave 0) | Resultado medido | Veredicto |
|---|---|---|---|
| FN-1 / T1 | Arquivos não-inertes — vetor exato da Wave 0: `internal/roadmapdoc/testdata/corpus/docs/algo.md` | `nao-inerte: internal/roadmapdoc/testdata/corpus/docs/algo.md → aplicável`; `diff_is_inert=false` (medido) | BLOQUEADO |
| FN-2 | Arquivo novo desconhecido (`mystery.bin`) | `diff_is_inert=false` (self-test 23/23) | BLOQUEADO |
| FN-3 / T3 | Rename `docs/x.md` → `src/y.go` (`--no-renames` lista ambos) | diff: `docs/x.md\0src/y.go\0`; `src/y.go` não-inerte → `false` (medido) | BLOQUEADO |
| FN-4 / T3 | Rename `internal/x.go` → `docs/x.go` (`--no-renames` lista ambos) | diff: `docs/x.go\0internal/x.go\0`; `internal/x.go` não-inerte → `false` (medido) | BLOQUEADO |
| FN-5 / T4 | Symlink em caminho inerte (R2) | medido — `docs/magic-link.go` mode 120000 → `diff_is_inert=true`; confirma mecanismo de R2: symlink em `docs/` é tratado como inerte; inócuo enquanto nenhum `go:embed docs/` existir | RESIDUAL R2 |
| FN-6 / T2 | Diff vazio (BASE_SHA == HEAD_SHA → empty diff) | `diff vazio → aplicável (fail-closed)`; `diff_is_inert=false` (medido) | BLOQUEADO |
| FN-7 / T2 | Base SHA indisponível — clone raso via `file://` | `fatal: Not a valid commit name` → D2; `diff_is_inert=false` (medido) | BLOQUEADO |
| FN-7a / T2 | BASE_SHA inexistente | merge-base falha → D2; `false` (medido) | BLOQUEADO |
| FN-7b / T2 | BASE_SHA vazio | merge-base falha → D2; `false` (medido) | BLOQUEADO |
| FN-7c / T2 | HEAD_SHA vazio | merge-base falha → D2; `false` (medido) | BLOQUEADO |
| FN-8 / T6 | Script ausente na BASE (ML-1A PR) | medido — `git cat-file -e "$BASE:scripts/check-shim-diff-applies.sh"` rc=128 → else branch → `echo "classificador ausente na BASE ou BASE_SHA vazio → aplicável (D2)"` + `diff_is_inert=false` no GITHUB_OUTPUT temp file | BLOQUEADO |
| FN-9 / T6 | PR modifica classificador, `--self-test` FALHA | por leitura (workflow-only, sem PR real) — quality.yml linha 975: `if [[ "$CLASSIFY_RESULT" != "success" ]]; then exit 1; fi` → gate falha; matriz `shim-byte-identity` também roda (`!cancelled() && !(classify.result==success && inert==true)` = true) | BLOQUEADO (gate) |
| FN-9 dois-PR | PR X auto-testa vermelho → não mergeia; PR Y subsequente dispensado indevidamente? | por leitura — BASE não muda (PR X não mergeou); BASE classify usa o script antigo correto; PR Y não é dispensado indevidamente. Risco residual R7 (Wave 0): bug que self-test não detecta; ver R7. | BLOQUEADO (PR Y seguro) |
| FN-9b | PR modifica classificador, `--self-test` PASSA (via GITHUB_PATH) | **F1: PATH envenenado antes de BASE classificar** | **VULNERÁVEL** |
| FN-10 | (não listado no doc Wave 0 — numeração pula de FN-9 para FN-11) | — | N/A |
| FN-11 / T8 | Base não é a branch padrão (retarget) | `base='feature/...' != default='main' → aplicável`; `false` (medido) | BLOQUEADO |
| T7 | `pull_request_target`, `workflow_dispatch` | quality.yml só dispara em `pull_request` e `push: branches: [main]`; outros eventos → D2; `false` (medido) | BLOQUEADO |
| T9 | Arquitetura do gate (`shim-byte-identity-gate`) | por leitura — `if: always()`; `CLASSIFY_RESULT != success → exit 1`; dispensa apenas `event==pull_request && base==default && inert==true`; caso contrário exige `SHIM_RESULT==success` | BLOQUEADO |
| RT-1 | Execução fora do repo (--self-test/--classify fora de git tree) | `git rev-parse` falha → D2; coberto pelo `--self-test` (casos T10a/T10b) | BLOQUEADO |
| RT-2 | Filename com newline injeta `::set-output` no stdout | ver F2 | VER F2 |

**Vetores de borda medidos nesta sessão:**

| Arquivo | inert | Observação |
|---|---|---|
| `docs/my file.md` (espaço) | 1 | NUL-delimitado lida corretamente com espaço |
| `docs/été.md` (non-ASCII) / T5 | 1 | NUL-delimitado lida corretamente com UTF-8; sem `-z`, `git diff` citaria o nome (R4) |
| `docs` (root, sem slash) | 0 | fail-closed: prefixo requer `/` |
| `vault` (root, sem slash) | 0 | fail-closed |
| `.claude/agent-memory` (arquivo, sem slash final) | 0 | fail-closed: prefixo `.claude/agent-memory/` requer `/` |
| `.claude/agent-memory/subfile` | 1 | correto |
| `docs/submodule` (gitlink mode 160000) | 1 | gitlink em `docs/` tratado como inerte (correto: `diff -z` lista como caminho) |
| `README.MD` (uppercase) | 0 | fail-closed: `*.md` case-sensitive |
| `docs/sub/deep/file.go` | 1 | qualquer extensão em `docs/` é inerte |

### AC2: T8 retarget — resposta pelo trigger list (`.github/workflows/quality.yml` linhas 6–13)

```
# SEM `types:` — de proposito. O default do `on: pull_request` e
# `opened · synchronize · reopened`, e acrescentar `edited` aqui dispararia
# o workflow INTEIRO a cada edicao de descricao de PR: medido em 2026-09-25,
# TODOS os 13 jobs deste arquivo (11 sem `if:` + 2 com `if: always()`, nenhum
# com `if:` que exclua `edited`) passariam a rodar, incluindo as tres suites
# de windows-latest. Escrever esse `if:` job a job deixaria o proximo job novo
# sem ele — contrato que se quebra por omissao. O gate que PRECISA de
# `edited` e so o de palavra-chave de fechamento, e por isso ele mora em
# .github/workflows/pr-closing-keyword.yml, com gatilho proprio.
```

`edited` está ausente por design; o workflow é `quality.yml`, não um workflow próprio de retarget. Se `edited` fosse necessário para o `classify` job, o remédio seria um workflow próprio (como o de palavra-chave), não adicionar `edited` a `quality.yml` — a nota inline explica exatamente por quê.

**Direção relevante para o threat model:** um PR classificado como "inerte" contra `main` retargetado para uma branch mais antiga pode apresentar um diff não-inerte contra o novo merge-base, e o classify não re-executa. O `shim-byte-identity-gate` leria o resultado antigo `diff_is_inert=true`. Porém: (a) `main` é a única branch protegida (`gh api 'repos/kgsaran/trackfw/branches?protected=true'` retorna apenas `main`); (b) mergear em uma branch de feature não dispensa o check na main; (c) este cenário exigiria que `shim-byte-identity-gate` fosse required numa branch de feature destino — atualmente não é o caso.

**Direção T8 B → main (Wave 0's receita exata):** branch B = `main` + commits em `internal/`; branch H = B + commit em `docs/`. PR aberto com base = B: `base_ref=B != default=main` → classify resulta em `aplicável`; braços rodam com base B; resultado é ganho pelos braços reais. Sem `edited` trigger, o resultado desta run persiste após retarget para `main`. O resultado registrado foi `aplicável` (não `diff_is_inert=true`), portanto o estado é conservador e seguro: o gate exige braços e os braços já passaram com base B.

A primeira medição real da dispensa de PR (inerte contra a main) só ocorrerá após o merge desta REQ, quando o primeiro PR só de documentação for aberto contra main.

---

## A3 — Prazo do PowerShell: psCallDeadline, psGuardedCall, psTimeoutRegistry

`psCallDeadline`: `clamp(4 × warmup, 15s, 60s)` — seis casos de fronteira testados:
- warmup < 3,75 s → piso 15 s; 3,75 s ≤ warmup ≤ 15 s → 4 × warmup; warmup > 15 s → teto 60 s

`psTimeoutRegistry`: global `psTimeouts` usado só em `runGuardLinePS` (linha 484). `TestRunGuardLinePS_TimeoutDiscriminado` chama `execPSCommand` diretamente (linhas 865 e 878) sem consultar o registry global — registry não envenenado pelo teste discriminado.

`TestRunGuardLinePS_FailFastPorBinario`: usa `reg := &psTimeoutRegistry{...}` isolado, não global `psTimeouts`. Independência confirmada.

**Resultado: sem achados no A3.**

---

## A4 — Retry de porta: next_port, attempt_serve

`next_port()`: usa python3 para ligar ao porto 0, lê `getsockname`, fecha — porta livre fornecida pelo OS.

`attempt_serve()`: até 3 tentativas, apenas com `rc==1 && "address already in use"` em stderr. Qualquer outro erro (timeout rc=2, permission denied) reprova imediatamente.

`SERVE_PARITY_FIRST_PORT`: presente no script para contra-braços, ausente dos workflows de CI. Sem risco de colisão previsível.

**Resultado: sem achados no A4.**

---

## A5 — Gate grep-q: padrões de evasão

| Padrão | Detectado pelo gate | Instâncias vivas em scripts/ |
|---|---|---|
| `\| grep -q PAT` (flag -q antes do padrão posicional) | SIM | 0 |
| `\| grep PAT -q` (flag -q depois do padrão posicional) | NÃO | 0 |
| `\| grep -Eq` (-q não-inicial no grupo de flags) | SIM | 0 |
| `command grep -q` (prefixo `command`) | NÃO | 0 |
| `\grep -q` (barra escape) | NÃO | 0 |
| `\| grep -E -q` (grupos separados) | SIM | 0 |
| `\| egrep -q` | SIM | 0 |
| `\|<TAB>grep -q` (tab em vez de espaço) | NÃO | 0 |
| `echo "$OUT" \| grep -q` em YAML | NÃO coberto (fora de scripts/) | 4 (ver F3) |

As lacunas de deteção (flags após posicional, `command`, barra-escape, TAB) não têm instâncias vivas em `scripts/`. O self-test 8/8 cobre os padrões do gate, não as formas de evasão — portanto o self-test verde não garante ausência de evasão futura.

---

## Veredito

**REPROVA** com três itens bloqueantes:

**[B1] F1 (HIGH):** `--self-test` (HEAD) precede BASE classify no mesmo job. PATH-envenenamento via `$GITHUB_PATH` confirmado por medição: classifier retorna `diff_is_inert=true` para commit não-inerte quando git falso está em PATH. Derrota ADR D1 "quem classifica" pelo caminho fora do sítio que D1 manda inspecionar.

**Fix B1:** No job `classify`, inverter a ordem dos dois passos: o step `id: classify` (BASE classify) vira o passo 1 e o step `--self-test` vira o passo 2 (mover bloco de passo no YAML).

**[B2] F2 (MEDIUM):** Filename com newline injeta a linha exata `::set-output name=diff_is_inert::true` no stdout do passo. Git aceita o filename localmente (rc=0). Linha exata confirmada por medição. Dois vínculos não medidos: (1) se o servidor GitHub aceita push com filename com `\n`; (2) se o runner ainda processa `::set-output` antes de ler o GITHUB_OUTPUT file.

**Fix B2:** Sanitizar `first_bad` e nomes no loop com `${var//[[:cntrl:]]/?}` antes de qualquer `echo`/`printf`. Acrescentar caso de self-test: "filename com newline não produz linha começando com `::` no stdout".

**[B3] F3 (MEDIUM):** 5 ocorrências de `| grep -q` em workflows sob pipefail; a mesma causa coberta por ML-1D AC1 em `scripts/`. Regra Dura pull estas ocorrências para esta REQ. `quality.yml:1252` tem falso negativo de gate de segurança confirmado por análise estática (FAIL presente → SIGPIPE → gate passa). Controle positivo: ML-1D pipeline (produtor lendo arquivo): 10/20 com 141 em bash/macOS. Echo com 1MB: 0/20 (macOS buffer absorve; Linux não medido). B3 é bloqueante pela Regra Dura independentemente da taxa.

**Fix B3:** Converter as 5 ocorrências para `>/dev/null`.

**F4** não bloqueia merge.

---

## Declaração de residuais

Residuais R1–R7 pertencem à Wave 0 e são aqui repetidos por referência.

- **R1 (Wave 0):** NOISE pode ocultar o gate de shim em PRs de docs.
- **R2 (Wave 0):** Symlink em docs/ com `go:embed` futuro.
- **R3 (Wave 0):** Workflow YAML executado na versão HEAD do PR.
- **R4 (Wave 0):** Nomes com não-ASCII nos caminhos inertes (FP futuro).
- **R5 (Wave 0):** Regressão de desempenho próxima ao cap de 60s pode passar.
- **R6 (Wave 0):** Probe TOCTOU no retry de porta (D4).
- **R7 (Wave 0):** Classificador quebrado bloqueia o required check.

Residuais novos desta sessão:

- **R8:** Retarget para branch sem `shim-byte-identity-gate` como required (T8). Após um retarget de main→outra-branch, uma dispensa "não aplicável" computada com base=main persiste num PR cuja base não é mais a branch padrão. Isso torna a emenda D1 do ADR — "a dispensa só é possível com base igual à branch padrão" — falsa como escrita. É inócuo hoje porque `main` é a única branch protegida (`gh api`: `main` como único resultado).
- **R9:** `README.MD` (uppercase) aciona verificação de shim desnecessariamente. Fail-closed correto; FP benigno.
- **R10:** Lacunas de deteção do gate grep-q (flags após posicional, `command`, barra-escape, TAB) existem sem instâncias vivas em `scripts/`. Aceito com atenção em revisões futuras.

---

## Fontes

- docs.github.com Enterprise Server 2.22, `::add-path::` — predecessor deprecado de `GITHUB_PATH`, documenta a semântica de persistência do PATH para passos subsequentes do mesmo job: [https://docs.github.com/en/enterprise-server@2.22/articles/development-tools-for-github-actions](https://docs.github.com/en/enterprise-server@2.22/articles/development-tools-for-github-actions)
- docs.github.com Enterprise Server 3.17, `GITHUB_PATH` — documenta que cada passo tem seu próprio arquivo de caminho único (implicando releitura por passo): [https://docs.github.com/en/enterprise-server@3.17/actions/reference/workflows-and-actions/variables](https://docs.github.com/en/enterprise-server@3.17/actions/reference/workflows-and-actions/variables)
- oneuptime.com (terceiros), tutorial sobre workflow commands — descreve `GITHUB_PATH` como adicionando diretórios ao PATH para passos subsequentes: [https://oneuptime.com/blog/post/2026-01-30-github-actions-workflow-commands/view](https://oneuptime.com/blog/post/2026-01-30-github-actions-workflow-commands/view)
- Deprecação de `::set-output` pelo vetor de injeção (outubro 2022): [https://blog.amnuts.com/tag/github/](https://blog.amnuts.com/tag/github/) (referência encontrada na busca; URL canônica do GitHub Changelog não foi recuperada nesta sessão)
