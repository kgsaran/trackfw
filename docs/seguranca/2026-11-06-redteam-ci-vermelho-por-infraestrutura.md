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

---

## Reverificação (ML-2C)

> Data: 2026-11-06 | Instrumento: hades-tf | Plataforma: darwin arm64, bash 5.3.20, LC_ALL=unset (en_US.UTF-8)

### Escopo

B1, B2 e B3 reatacados contra o código commitado pelo ML-2B (commit `24ca1ca5`). As medições
usaram repositório temporário em `mktemp -d` isolado do projeto; nenhuma escrita git ocorreu na
árvore `/Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw`.

---

### B1 — Ordem de passos no job `classify`

**Verificação por parse do YAML:**

```
python3 -I -c "import yaml; ..."  →  quality.yml jobs.classify.steps:
  Step 1: uses="actions/checkout@v7"             id="(no id)"
  Step 2: name="classificar diff do PR (versão BASE)"  id="classify"
  Step 3: name="self-test do classificador (versão HEAD)"  id="(no id)"
```

A etapa `id: classify` (BASE) é o passo 2; o `--self-test` (HEAD) é o passo 3. Nenhum código
controlado pelo PR roda antes do passo BASE.

**Pergunta B1 do ML-2C: o `actions/checkout@v7` planta algo que o passo BASE consome?**

Testado em repositório temporário com `.gitattributes` declarando `filter=evil diff=evil` para
`scripts/check-shim-diff-applies.sh`, com e sem o driver `evil` configurado em `.git/config`:

```
git cat-file -e BASE_SHA:scripts/...  →  rc=0  (filtro não afeta existência)
git show BASE_SHA:scripts/...         →  "echo hello"  (smudge não foi aplicado)
git diff -z --name-only               →  "internal/x.go"  (somente caminho, sem transformação)
```

Resultado: `git cat-file`, `git show sha:path` e `git diff --name-only` leem diretamente do object
store; filtros de `.gitattributes` não são aplicados. Hooks não são acionados por essas operações.
`$GITHUB_ENV` não é escrito pelo `actions/checkout` em nenhuma variável que o passo BASE consuma.

**Verificação de `python3`:** o script `check-shim-diff-applies.sh` exporta `PYTHONIOENCODING=utf-8`
na linha 20, mas não invoca nenhum interpretador externo (`grep -n 'python3\|perl\|node'`
retorna vazio). O caminho `run_classify()` usa apenas built-ins bash e comandos git.

**Veredito B1: FECHADO.** O swap de ordem do ML-2B cobre o vetor completo sem superfície
residual no caminho `--classify`.

---

### B2 — Sanitização de filenames com caracteres de controle

**Plataforma:** bash 5.3.20, `[[:cntrl:]]` = locale en_US.UTF-8 (ASCII 0x00–0x1f + 0x7f).

**Aceitação de bytes por git (testado via `update-index --add --cacheinfo`):**

| Payload | Aceito pelo git local? |
|---|---|
| `internal/x` + `\r` + `z.go` | SIM (rc=0) |
| `internal/x` + ESC (`\x1b`) + `z.go` | SIM (rc=0) |
| `internal/x%0Az.go` (literal, não controle) | SIM (rc=0) |
| `internal/x` + `\n` + `z.go` | NÃO (rc=128) |

**Comportamento de `sanitize_path` (fonte verificada, linhas 57–64 do script):**

| Payload em `first_bad` | Saída de `sanitize_path` | Substituição correta? |
|---|---|---|
| `internal/x\rz.go` | `internal/x?z.go` | SIM (`\r` = cntrl) |
| `internal/x\033z.go` (ESC) | `internal/x?z.go` | SIM (ESC = cntrl) |
| `internal/x%0Az.go` | `internal/x%0Az.go` | SIM (`%0A` não é cntrl) |

**Medição end-to-end (repositório temporário, `GITHUB_OUTPUT` apontado para arquivo temp):**

Payload `\r` em caminho não-inerte:
```
stdout:  nao-inerte: internal/x?z.go → aplicável
GITHUB_OUTPUT: diff_is_inert=false
```
Nenhuma linha começa com `::`.

Payload ESC em caminho não-inerte:
```
stdout:  nao-inerte: internal/x?z.go → aplicável
GITHUB_OUTPUT: diff_is_inert=false
```
Confirmado por `od -An -tx1`: posição do ESC substituída por `3f` (`?`).

Payload `%0A` literal em caminho não-inerte:
```
stdout:  nao-inerte: internal/x%0Az.go → aplicável
GITHUB_OUTPUT: diff_is_inert=false
```
`%0A` não é caractere de controle; passa intacto, sem risco de injeção.

**Variante inerte: `docs/::error::x` (linha do inert\_list, sitio de impressão B2):**
```
stdout:
  todos 1 arquivo(s) são inertes (lista no log do job classify):
    docs/::error::x
  diff inerte → não aplicável
grep '^::' → (nenhuma linha começa com ::)
```
O prefixo `docs/` garante que `::error::x` nunca aparece na coluna 0.

**Variante crítica: `::set-output name=diff_is_inert::true.md` (root \*.md, inerte):**
```
stdout:
  todos 1 arquivo(s) são inertes (lista no log do job classify):
    ::set-output name=diff_is_inert::true.md
  diff inerte → não aplicável
GITHUB_OUTPUT: diff_is_inert=true
grep '^::' → (nenhuma linha começa com :: na coluna 0)
grep '^\s+::' → "  ::set-output name=diff_is_inert::true.md"  (2 espaços antes)
```

Análise de risco: (a) o runner do GitHub Actions processa `::set-output` apenas em linhas onde `::` está
na **coluna 0**; `  ::set-output` (2 espaços) não é reconhecido; (b) `::set-output` foi
deprecado/removido do runner em outubro de 2022; (c) mesmo que processado, o valor seria
`true.md` (≠ `true`), e o `$GITHUB_OUTPUT` explícito sobrescreve na mesma etapa; (d) o
veredito permanece `diff_is_inert=true` mesmo sem o stdout injection, porque o classificador
já decidiu que todos os arquivos são inertes.

**Self-test Part 5 (27/27 passaram, medido e via `parity-rest`):** os casos T\_newline-a/b e
T\_newline\_in-a/b cobrem filenames com `\n` nos dois sítios de impressão, verificando `^::`.
O self-test afirma `^::` (coluna 0), que é a mesma condição que o runner checa — coerente.

**Declaração de residual (R11):** `::set-output name=diff_is_inert::true.md` em root `*.md` gera
`  ::set-output` com 2 espaços no inert\_list. Runner moderno (pós-out 2022) não processa; mesmo
runner antigo não processaria pela indentação. Aceito como residual LOW.

**Veredito B2: FECHADO.** A sanitização cobre `\r`, ESC e `\n` (os únicos bytes de controle que
o git local aceita em nomes de arquivo). O sítio de impressão inerte (`  <nome>`) nunca coloca
`::` na coluna 0. R11 declarado.

---

### B3 — `| grep -q` em workflows

**Gate executado diretamente:**

```
bash scripts/check-no-grep-q-under-pipefail.sh
→ check-no-grep-q-under-pipefail.sh: OK — no pipe-grep-q under pipefail found
```

**Self-test (10/10 passaram):**

```
bash scripts/check-no-grep-q-under-pipefail.sh --self-test
→ Results: 10 passed, 0 failed
  (inclui ty1-yml-grep-q e ty2-yml-devnull — os dois casos YAML de B3)
```

**Verificação direta dos 5 sítios do ML-2A:**

| Arquivo | Linha antes | Linha depois | Sob pipefail? |
|---|---|---|---|
| `quality.yml` | `echo "$OUT" \| grep -q -e "--- SKIP"` | `echo "$OUT" \| grep -e "--- SKIP" >/dev/null` | SIM (shell: bash explícito) |
| `quality.yml` | `echo "$OUT" \| grep -q -e "--- FAIL"` | `echo "$OUT" \| grep -e "--- FAIL" >/dev/null` | SIM |
| `quality.yml` | `echo "$OUT" \| grep -q -e "--- FAIL"` | `echo "$OUT" \| grep -e "--- FAIL" >/dev/null` | SIM |
| `windows-census.yml` | `echo "$GO_OUT" \| grep -q ...\|\| echo "$PY_OUT" \| grep -qi ...` | `echo "$GO_OUT" \| grep ... >/dev/null \|\| echo "$PY_OUT" \| grep -i ... >/dev/null` | SIM (defaults: run: shell: bash + set -euo pipefail) |
| `release.yml:83` | `echo "$RAW" \| grep -qE '...'` | `echo "$RAW" \| grep -E '...' >/dev/null` | default bash -e {0} sem pipefail |

**Escaneamento de escape patterns (nenhuma instância viva encontrada):**

| Padrão | Detectado pelo gate? | Instâncias vivas em scripts/ ou workflows? |
|---|---|---|
| `\| grep PAT -q` (flag após posicional) | NÃO | 0 (apenas em comentários do gate) |
| `\| command grep -q` | NÃO | 0 |
| `\| \grep -q` | NÃO | 0 |
| `\|&` | NÃO | 0 (ocorrência em comentário de `check-unguarded-capture-rc.sh`) |
| `\| xargs grep -q` | NÃO | 0 |
| `\| grep -e PAT -q` | NÃO | 0 |
| `\|` continuação de linha + `grep -q` na próxima | NÃO | 0 |
| Folded scalar `run: >` | N/A | 0 |
| `\| grep -vqE` em workflows | NÃO | 0 |

**Cobertura do gate:**
- `scripts/*.sh` com `pipefail`: anti-vacuidade ativa (guard `${#SCRIPTS[@]} -eq 0 → exit 2`).
- `.github/workflows/*.yml`: 9 arquivos escaneados (confirmado por `ls -1 | wc`); **sem
  anti-vacuidade** para o braço YAML — se o diretório estiver vazio, o scan passa silencioso.
  Aceito como residual R12 (LOW): hoje há 9 arquivos, e o build quebraria muito antes por outras
  razões se os workflows sumissem.
- `scripts/` subdirectórios: nenhum existe (maxdepth 1 é adequado).
- Makefile: `grep -q` aparece apenas em comentários e em chamadas ao gate, nunca em target
  com `SHELL += -o pipefail`.
- `.github/actions/`: diretório não existe.

**Medição de estabilidade (20 iterações, forma >/dev/null):**

```
Pipeline: grep -v '^\s*#' quality.yml | grep check-shim-diff-applies >/dev/null
20 execuções: 0/20 retornaram 141
```

**Veredito B3: FECHADO.** Os 5 sítios foram corrigidos; nenhum escape pattern detectável tem
instância viva; a ausência de anti-vacuidade no braço YAML é declarada como R12 LOW.

---

### Regressão

**`bash scripts/check-shim-diff-applies.sh --self-test`:** 27/27 passaram (incluindo
T\_newline-a/b e T\_newline\_in-a/b da Parte 5 — cobertura B2).

**`make parity-rest` (rc=0, 2126 linhas de saída):**
- `check-workflow-yaml.py`: 10 passed, 0 failed
- `check-barrier.sh`: todos os cenários passaram
- `check-shim-byte-identity.sh`: 10 passed, 0 failed (shim byte-idêntico ao nativo em 5/5 comparações substantivas)
- `check-no-grep-q-under-pipefail.sh --self-test`: 10 passed, 0 failed
- `check-no-grep-q-under-pipefail.sh`: OK — no pipe-grep-q under pipefail found
- Nenhum `FAIL` inesperado em toda a saída.

---

### Veredito final (ML-2C)

**APROVA COM RESSALVAS**

Os três bloqueantes do ML-2A foram fechados pelo ML-2B e confirmados por medição independente:

- **B1 FECHADO:** ordem BASE classify → HEAD self-test verificada por parse YAML; nenhum passo
  HEAD roda antes do passo BASE; `actions/checkout` não planta nada que o passo BASE consuma
  (filtros `.gitattributes` não afetam `git show sha:path`, git não aciona hooks em
  `cat-file`/`show`/`diff --name-only`; nenhum interpretador externo invocado no caminho
  `run_classify()`).

- **B2 FECHADO:** `sanitize_path` substitui `\r` e ESC por `?` (medido end-to-end com
  `GITHUB_OUTPUT` apontado para arquivo temp). Git rejeita `\n` em nomes localmente (rc=128);
  `%0A` literal não é caractere de controle. Sítio do inert\_list imprime `  <nome>` com 2
  espaços — `::` nunca aparece em coluna 0. R11 declarado (LOW).

- **B3 FECHADO:** gate `check-no-grep-q-under-pipefail.sh` passa (OK) e cobre workflows YAML;
  os 5 sítios foram convertidos para `>/dev/null`; nenhum escape pattern com instância viva
  encontrado; 20/20 iterações do pipeline fixado retornam rc=0. R12 declarado (LOW).

**Residuais novos declarados nesta sessão:**

- **R11 (LOW):** filename `::set-output name=diff_is_inert::true.md` em root `*.md` gera linha
  `  ::set-output...` no inert\_list com 2 espaços de prefixo. Runner pós-2022 não processa;
  coluna 0 não atingida; valor seria `true.md` ≠ `true` mesmo se processado; `$GITHUB_OUTPUT`
  sobrescreve. Sem caminho de escalada para mudar `diff_is_inert`.

- **R12 (LOW):** braço YAML do gate `check-no-grep-q-under-pipefail.sh` não tem anti-vacuidade;
  se `.github/workflows/` estiver vazio, o scan passa silencioso. Mitigado pelo fato de que a
  ausência dos workflows quebraria o CI muito antes.

---

### Estado do repositório (pós-medição)

```
git -C /Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw status --short
(saída vazia — árvore limpa)

git -C /Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw diff --cached --name-only
(saída vazia — nada staged)
```

Verificado em três pontos: pré-harness, pós-B2, pós-parity-rest. Nenhuma entrada estranha
no índice do repositório real em nenhum ponto.

---

### Temporários criados

- `/private/tmp/claude-501/-Users-kgsaran-Sistemas-Desenvolvimento-workspace-trackfw/a247cc38-463b-4f99-8cb2-7400f9dec171/scratchpad/hades3/` — diretório raiz dos temporários desta sessão
- `/private/tmp/claude-501/-Users-kgsaran-Sistemas-Desenvolvimento-workspace-trackfw/a247cc38-463b-4f99-8cb2-7400f9dec171/scratchpad/hades3/repo.5nlTWl/` — repositório temporário para medições B1/B2 (pode ser removido)
