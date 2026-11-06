# Wave 0 — Threat Model: CI vermelho sem relação com o conteúdo

> Roadmap: ROADMAP-2026-11-06-ci-vermelho-sem-relacao-com-o-conteudo-…  
> Revisor: hades-tf | Data: 2026-11-06 | ML-0A  
> Veredito: **APROVA COM RESSALVAS** — duas decisões pendentes para Zeus (A e B); itens
> obrigatórios para ML-1A ao final; Seção 1 não pode ser marcada ✅ enquanto Decisão A não for
> resolvida.

---

## Seção 1 — Enumeração de superfícies (completude)

**O roadmap elenca três superfícies.** A análise abaixo confirma que a lista está fechada e identifica
as sub-superfícies relevantes dentro de cada item.

**Estado do critério de ML-0A item 1 no roadmap:** o conjunto inerte está fechado pelo critério do
ADR D1 ("não altera o que o shim-byte-identity mede"). O critério literal de ML-0A item 1 no roadmap
diz "nenhum deles entra no build, no go:embed, no shim npm, no wheel nem em script que algum gate
execute" — essa redação conflita com o ADR (ver Decisão A ao final do documento). Este box só pode
ser marcado ✅ após Zeus decidir A.

### 1.1 Superfície D1 — conjunto inerte para o braço do shim

A decisão D1 dispensa os braços do `shim-byte-identity` apenas quando **todos** os arquivos do diff
estão no conjunto inerte. A pergunta de enumeração é: o conjunto proposto — `docs/`, `vault/`,
`.claude/agent-memory/` e os `*.md` da raiz — é fechado em relação a "não altera o que o
shim-byte-identity mede"?

**O que entra no binário Go (shim byte-identity)**

```
$ grep -rn '//go:embed' --include='*.go' . | grep -v '_test.go'
internal/integrations/catalog.go:15://go:embed assets
internal/serve/serve.go:17://go:embed static
internal/i18n/i18n.go:11://go:embed locales/en-US.json
internal/i18n/i18n.go:14://go:embed locales/pt-BR.json
internal/i18n/i18n.go:17://go:embed locales/es-ES.json
```

Cinco diretivas `//go:embed` em código de produção. Nenhum dos destinos está sob `docs/`, `vault/`,
`.claude/agent-memory/` ou na raiz como `*.md`.

**O mecanismo do shim é transparente**

`npm/bin/trackfw.js:71`:
```javascript
const result = spawnSync(binPath, process.argv.slice(2), {
  stdio: "inherit",
  env: process.env,
});
```

Com `stdio: "inherit"`, o Node repassa os descritores de arquivo herdados do processo pai
diretamente ao binário Go, sem copiar ou decodificar nenhum byte. O gate captura as saídas via
redirecionamento de shell (`check-shim-byte-identity.sh:272–273`):
```bash
(cd "$REPO_ROOT" && "$NATIVE_BIN" "${args[@]}" >"$n_out" 2>"$n_err") || n_exit=$?
(cd "$REPO_ROOT" && node "$SHIM_COPY" "${args[@]}" >"$s_out" 2>"$s_err") || s_exit=$?
```
O shell aplica as redirects antes que o Node inicie; o Go escreve diretamente nos arquivos
temporários. O shim não interpõe nenhum buffer de string.

**O que a suite do shim executa via `cwd=$REPO_ROOT`**

`check-shim-byte-identity.sh` compila e executa C2 `validate --json` com `cd "$REPO_ROOT"`. Este
comando lê dados de `docs/roadmaps/`, `docs/req/`, `docs/adr/`, `vault/notes/` e `.claude/settings.json`.
Como o shim repassa os fds sem interpretar bytes, qualquer mudança nesses arquivos alimenta
identicamente os dois lados (nativo e shim). Não existe via pela qual `docs/` ou `vault/` possam
produzir divergência nativo-vs-shim.

**Exceção para R1 (declarada em Seção 4):** se `validate --json` se tornar não-determinístico a
partir de um PR de docs, a comparação vai para NOISE → VACUOUS, e o gate não mede nada. Isso não
é divergência nativo-vs-shim — é ausência de sinal.

**`.claude/settings.json` está FORA do conjunto inerte**

O validador lê `.claude/settings.json` para o credential guard check. O conjunto inerte especifica
`.claude/agent-memory/` (prefixo completo, não `.claude/`) — isso exclui corretamente `settings.json`.

**Critério 1 de ML-0A (roadmap) — Decisão A para Zeus**

O critério de ML-0A item 1 no roadmap diz: "nenhum deles entra no build, no go:embed, no shim npm,
no wheel nem em script que algum gate execute". Contudo:
- `README.md` da raiz é usado em `release.yml` para o wheel PyPI.
- `docs/cli-parity.md` e `CHANGELOG.md` são lidos por scripts em `parity-other-gates`.
- `docs/` e `vault/` são lidos por `validate --json`, que a suite do shim invoca.

Lido literalmente, o critério encolhe o conjunto inerte para apenas `.claude/agent-memory/`,
tornando D1 inútil. O critério correto é o do ADR D1: "não altera o que o shim-byte-identity mede".
Zeus deve emendar o critério de ML-0A item 1 no roadmap para corresponder ao ADR D1.

**`internal/roadmapdoc/testdata/corpus/docs/`**

Verificado: `internal/roadmapdoc/testdata/corpus/docs/algo.md` começa com `internal/` → prefixo
ancorado `[[ "$f" == docs/* ]]` não casa → `aplicavel`. SAFE.

**`npm/README.md`:** contém `/` → não casa com `[[ "$f" == *.md && "$f" != */* ]]` → `aplicavel`. SAFE.

**Conjunto fechado final (critério ADR D1)**

| Candidato | Afeta go:embed? | Produz divergência nativo-vs-shim? | Gate diferente ainda cobre? | Decisão |
|---|---|---|---|---|
| `docs/` (raiz) | Não | Não¹ | `parity-other-gates` cobre docs/cli-parity.md | **Inerto para o shim** |
| `vault/` | Não | Não¹ | Não (apenas runtime do validator) | **Inerto para o shim** |
| `.claude/agent-memory/` | Não | Não | Não | **Inerto para o shim** |
| `*.md` raiz (sem `/`) | Não | Não¹ | `parity-other-gates` cobre CHANGELOG.md | **Inerto para o shim** |

¹ Ambos os lados leem identicamente; sem divergência possível por mecanismo spawnSync+inherit.

**Enumeração: a lista está FECHADA** pelo critério do ADR D1 — condicional à Decisão A de Zeus.

---

## Seção 2 — Modelo de ameaça ✅

O adversário aqui é o implementador apressado que, ao escrever o classificador de diff (ML-1A),
abre uma das lacunas abaixo sem perceber. Cada ameaça tem um caminho de exploit e uma defesa.

### T1 — Padrão `docs/` como substring, não como prefixo ancorado

**Caminho:** o script usa `[[ "$f" == *docs/* ]]` (substring). Um PR que toca
`internal/roadmapdoc/testdata/corpus/docs/algo.md` seria classificado inerte.

**Defesa:** usar padrão prefixo ancorado: `[[ "$f" == docs/* ]]`. `git diff --name-only` retorna
caminhos relativos à raiz — `internal/…` NUNCA começa com `docs/`.

### T2 — Diff vazio, git falha, ou base ausente — classificado como "inerte"

**Caminho:** um PR com zero arquivos alterados, ou clone raso que falha no diff e devolve vazio.

**Defesa:** diff vazio → APLICÁVEL (fail-closed). `diff_is_inert=true` só é escrito após `count -gt
0` E nenhum arquivo não-inerte encontrado. O não-inerte é rastreado em variável separada. O check
de base vem ANTES do loop, com `exit 0`:

```bash
is_inert() {
  local f="$1"
  [[ "$f" == docs/*                 ]] && return 0
  [[ "$f" == vault/*                ]] && return 0
  [[ "$f" == .claude/agent-memory/* ]] && return 0
  [[ "$f" == *.md && "$f" != */*    ]] && return 0
  return 1
}

# 1) Verificar base do PR ANTES de qualquer outra coisa
if [ -z "$GITHUB_BASE_REF" ] || [ "$GITHUB_BASE_REF" != "$DEFAULT_BRANCH" ]; then
  echo "base='$GITHUB_BASE_REF' != default='$DEFAULT_BRANCH' → aplicável"
  echo "diff_is_inert=false" >> "$GITHUB_OUTPUT"
  exit 0
fi

# 2) Calcular diff — falha de git = diff indisponível → aplicável (D2)
git diff -z --name-only --no-renames "${merge_base}..HEAD" > "$tmpfile" 2>/dev/null || {
  echo "diff indisponível → aplicável (D2)"
  echo "diff_is_inert=false" >> "$GITHUB_OUTPUT"
  exit 0
}
# NOTA: o ramo "|| { … }" não foi medido nesta sessão com git(){ return 128; }

# 3) Classificar
non_inert="" count=0
while IFS= read -r -d "" f; do
  [[ -z "$f" ]] && continue
  count=$((count+1))
  if ! is_inert "$f"; then non_inert="$f"; break; fi
done < "$tmpfile"

if   [ "$count" -eq 0 ];  then
  echo "diff vazio → aplicável"; echo "diff_is_inert=false" >> "$GITHUB_OUTPUT"
elif [ -n "$non_inert" ]; then
  echo "nao-inerte: $non_inert → aplicável"; echo "diff_is_inert=false" >> "$GITHUB_OUTPUT"
else
  echo "todos $count inertes → não aplicável"; echo "diff_is_inert=true" >> "$GITHUB_OUTPUT"
fi
```

**Nota sobre `DEFAULT_BRANCH`:** `GITHUB_DEFAULT_BRANCH` não é uma variável de ambiente padrão do
GitHub Actions. `DEFAULT_BRANCH` deve ser passada via `env:` no YAML:
```yaml
env:
  DEFAULT_BRANCH: ${{ github.event.repository.default_branch }}
```

Sem isso, num PR onde `GITHUB_BASE_REF` está definido (ex: "main") mas `DEFAULT_BRANCH` está vazio,
o check produz `"main" != ""` → aplicável. Fail-closed, mas D1 nunca dispensa nenhum PR — código
morto. ML-1A deve garantir que `DEFAULT_BRANCH` seja sempre populada.

**Medição em bash 5.3.20 no scratchpad** (stub `printf '%s\0'` em lugar de `git diff`):

```
bash: 5.3.20(1)-release

V1 → aplicavel (diff vazio)
V2 → nao-aplicavel (todos 1 inertes)                    # docs/a.md
V3 → aplicavel (nao-inerte: internal/x.go)              # docs/a.md + internal/x.go
V4 → aplicavel (nao-inerte: internal/x.go)              # internal/x.go + docs/a.md
V5 → aplicavel (nao-inerte: internal/x.go)              # internal/x.go + docs/x.go
V6 → nao-aplicavel (todos 3 inertes)                    # docs/ + vault/ + README.md
V7 → aplicavel (base_ref='feature/docs' != default='main')   # base nao-padrao + docs
V8 → nao-aplicavel (todos 1 inertes)                    # base padrao + docs
V9 → aplicavel (base_ref='' != default='main')          # push (base_ref vazia)
```

V3 e V4 eram os falsos negativos do snippet anterior (que começava `aplica=true` e nunca atribuía
`false`). V7 e V9 provam que o check de base com `exit 0` antes do loop cobre o vetor T8.

### T3 — Rename silencioso via detecção automática (medido)

**Caminho:** `git diff --name-only` sem `--no-renames` detecta o rename `internal/x.go → docs/x.go`
e mostra apenas o destino `docs/x.go` — classificado inerte, braços dispensados.

**Medição em bash no scratchpad** (diretório `rename-test2/`):

```bash
$ git init -q && mkdir -p internal && echo 'x' > internal/x.go
$ git add internal/x.go && T1=$(git write-tree)
$ mkdir -p docs && git mv internal/x.go docs/x.go && T2=$(git write-tree)

$ git diff --name-only "$T1" "$T2"
docs/x.go

$ git diff --no-renames --name-only "$T1" "$T2"
docs/x.go
internal/x.go
```

Sem `--no-renames`: apenas `docs/x.go` → classificado inerte → falso negativo CONFIRMADO.
Com `--no-renames`: ambos os caminhos → `internal/x.go` não é inerte → aplicável.

**Defesa:** usar `git diff -z --name-only --no-renames`. ADR D1 especifica explicitamente `--no-renames`.

### T4 — Symlink adicionado em caminho inerte apontando para código

**Caminho:** PR adiciona `docs/magic-link.go` como symlink (modo 120000) para código de produto.
`git diff --name-only` mostra `docs/magic-link.go` → classificado inerte.

**Estado atual:** nenhum `//go:embed` aponta para `docs/`. Nenhum symlink existe no repositório.
O vetor é atualmente inócuo. Ver R2 em Seção 4.

### T5 — Nome de arquivo com caracteres não-ASCII ou controle

**Caminho:** `git diff --name-only` sem `-z` cita (backslash-escapes) nomes com bytes acima de 0x7F.
O caminho citado começa com `"` e não casa com nenhum padrão inerte → APLICÁVEL → fail-closed benigno.

**Estado atual:** nenhum nome não-ASCII em `docs/`, `vault/` ou `.claude/`. Ver R4 em Seção 4.

### T6 — O próprio classificador controlado pelo PR

**Caminho:** um PR muda `scripts/check-shim-diff-applies.sh` para sempre emitir `diff_is_inert=true`.
O classificador lê o arquivo novo (HEAD) → braços dispensados.

**Defesa:** o classificador usa a versão BASE SHA. O `--self-test` roda no job `classify` ANTES
da etapa de classificação, usando a versão HEAD. Se o self-test falhar, o job `classify` falha →
gate rejeita `CLASSIFY_RESULT != success` (ver T9) → PR bloqueado.

**Bootstrap deadlock (FN-8 — risco de ML-1A):** quando ML-1A abre o PR, o script
`scripts/check-shim-diff-applies.sh` não existe em BASE. A etapa de classificação de BASE não pode
ser um `git show BASE:… | bash` que falha com `-eo pipefail`. Caso contrário, o próprio PR de ML-1A
fica vermelho por construção, e o implementador apressado "corrige" removendo o check
`CLASSIFY_RESULT != success`. Solução: a etapa de classificação de BASE trata "script ausente em
BASE" como D2 — job `classify` tem saída com `diff_is_inert=false`, status `success`. O job
`classify` não tem `if:` condicional; roda em todo evento. Ver mandatory item 10.

**Residual:** o `quality.yml` é executado na versão HEAD do PR. Um PR que modifica o próprio workflow
pode alterar como o classificador é invocado — residual pré-existente a qualquer gate de CI.
Ver R3.

**Split de responsabilidades:**
- `--self-test`: roda na versão HEAD (testa a lógica que o PR introduz; falha → job falha → PR bloqueado).
- Classificação: usa a versão BASE (sem que o PR controle sua própria cobertura — ADR-2026-09-17).

### T7 — `pull_request_target`, `workflow_dispatch`, `merge_group`

**Estado atual (medido):** `quality.yml` lines 3–15 — only triggers `pull_request` (no `types:`)
and `push: branches: [main]`. Sem `pull_request_target`, `workflow_dispatch`, `merge_group` ou
`schedule`. Se `merge_group` for adicionado, o diff não estará disponível → D2 → gate exige success.

### T8 — Retargetamento de base do PR (hipótese; ML-2A verifica)

**A defesa age na abertura do PR, não após o retargetamento.**

Cenário: branch B = `main` + commits em `internal/`. Branch H = B + um commit em `docs/`. PR aberto
com base = B: `GITHUB_BASE_REF = B`, que não é `DEFAULT_BRANCH = main`. O check de base da abertura
detecta `"B" != "main"` → `diff_is_inert=false` → gate exige braços → braços rodam com base B →
green em H é ganho por braços reais.

Mais tarde, PR é retargetado para `main` sem novo push (head SHA H não muda). Se nenhum novo run
for disparado — hipótese, pois `edited` pode não ser subscrito (padrão do evento `pull_request`
cobre `opened`, `synchronize`, `reopened`; a nota inline do `quality.yml` confirma que `edited`
está ausente) — o check verde em H permanece. O que branch protection vê é o check em H contra a
abertura original. O log daquela run mostraria "aplicável (base_ref=B)", não "não aplicável" — esse
é o sinal discriminante que ML-2A deve verificar.

**Receita exata para ML-2A:**
1. Criar branch B = `main` + commits em `internal/`.
2. Criar branch H = B + um commit em `docs/`.
3. Abrir PR com base = B, head = H. Aguardar o run. Confirmar no log do gate: "aplicável (base_ref=B)".
4. Sem push novo, retargetar a base para `main`.
5. Verificar se novo run foi disparado (hipótese: não). Se sim, confirmar que o check de base
   produz "aplicável (base_ref=main == main)". Se não: registrar que o green em H foi ganho pelo
   run original que corretamente exigiu os braços.

### T9 — Arquitetura do gate: job separado, condição do matrix job e condição do gate

**Matrix job `shim-byte-identity`:** um job-level `if:` é uma condição de RUN. `if: diff_is_inert
== 'true'` RODA os braços apenas em PRs inertes (invertido — erra). A condição correta é a negação:

```yaml
# Dispensa os braços SOMENTE quando classify passou E diff é inerte
shim-byte-identity:
  needs: [classify]
  if: >
    always() &&
    !(needs.classify.result == 'success' &&
      needs.classify.outputs.diff_is_inert == 'true')
```

**Output do job classify:** a saída `diff_is_inert` deve ser declarada em `outputs:` no job `classify`:
```yaml
classify:
  runs-on: ubuntu-latest
  outputs:
    diff_is_inert: ${{ steps.classify.outputs.diff_is_inert }}
```

**Gate `shim-byte-identity-gate`:** quando os braços são pulados, `needs.shim-byte-identity.result`
= `skipped`. O gate rejeita `skipped`. O gate verifica o classificador para distinguir "inerte,
dispensa intencional" de "pulado inesperadamente". **O gate SEMPRE falha se `classify` falhou** —
isso garante que um `--self-test` vermelho bloqueie o merge:

```yaml
shim-byte-identity-gate:
  needs: [classify, shim-byte-identity]
  if: always()
  runs-on: ubuntu-latest
  steps:
    - name: Evaluate gate
      env:
        EVENT_NAME:       ${{ github.event_name }}
        BASE_REF:         ${{ github.base_ref }}
        DEFAULT_BRANCH:   ${{ github.event.repository.default_branch }}
        CLASSIFY_RESULT:  ${{ needs.classify.result }}
        DIFF_IS_INERT:    ${{ needs.classify.outputs.diff_is_inert }}
        SHIM_RESULT:      ${{ needs.shim-byte-identity.result }}
      run: |
        # 1) classify DEVE ter sucedido — sem isso, gate sempre falha
        if [[ "$CLASSIFY_RESULT" != "success" ]]; then
          echo "FAIL: classify result=$CLASSIFY_RESULT"
          exit 1
        fi
        # 2) Condição de dispensa: PR + base padrão + diff inerte
        if [[ "$EVENT_NAME" == "pull_request" &&
              -n "$BASE_REF" &&
              "$BASE_REF" == "$DEFAULT_BRANCH" &&
              "$DIFF_IS_INERT" == "true" ]]; then
          echo "não aplicável: diff só toca caminhos inertes"
          exit 0
        fi
        # 3) Em todo o resto: exige braços com sucesso
        if [[ "$SHIM_RESULT" == "success" ]]; then exit 0; fi
        echo "FAIL: shim-byte-identity=$SHIM_RESULT inert=$DIFF_IS_INERT"
        exit 1
```

Consequência de (1): um `--self-test` vermelho no job `classify` torna o required check vermelho,
bloqueando o merge. Essa é a defesa contra o vetor da Seção 2 T6.

---

## Seção 3 — Alvos de falsificação (dois sentidos) ✅

### ML-1A — Gate do shim com classificação de diff

**Falso negativo (gate dispensa quando não deveria):**

| Vetor | Arquivo(s) no diff | Resultado esperado | Defesa |
|---|---|---|---|
| FN-1 | `internal/main.go`, `npm/x.js`, `cmd/y.go` | `diff_is_inert=false` | prefixo ancorado |
| FN-2 | arquivo novo desconhecido (`mystery.bin`) | `diff_is_inert=false` | fail-closed para desconhecidos |
| FN-3 | rename `docs/x.md` → `src/y.go` (--no-renames lista ambos) | `diff_is_inert=false` (`src/y.go` não é inerte) | `--no-renames` (medido) |
| FN-4 | rename `internal/x.go` → `docs/x.go` (--no-renames lista ambos) | `diff_is_inert=false` (`internal/x.go` aparece) | `--no-renames` (medido) |
| FN-6 | diff vazio (zero linhas) | `diff_is_inert=false` | fail-closed explícito |
| FN-7 | base SHA indisponível (clone raso) | falha de git → `diff_is_inert=false` | D2 / tratar erro |
| FN-8 | script ausente na BASE (ML-1A PR) | classify job sucede com `diff_is_inert=false` | D2 para script ausente (item 10) |
| FN-11 | base não é a branch padrão (retarget) | `diff_is_inert=false` | check `base_ref == DEFAULT_BRANCH` (exit antes do loop) |

FN-5 (symlink em caminho inerte): atualmente inócuo (nenhum `go:embed` em `docs/`). Coberto por R2;
não entra no `--self-test`.

FN-9 (PR modifica classificador, `--self-test` falha → gate bloqueia) é mecânica de workflow —
verificada apenas quando o CI roda. Movido para ML-2A.

**Vetor ML-2A adicional:** sequência de dois PRs — PR X introduz bug no classificador (self-test
falha → gate vermelho → não mergeia); confirmar que PR Y subsequente não é dispensado indevidamente.

**Vetor ML-2A — sentido FP obrigatório:** um PR de docs-only real deve logar "não aplicável" no
gate. Sem isso, D1 pode estar presente no código mas nunca disparar — código morto que não protege.

**`--self-test` cobre:** FN-1, FN-2, FN-3, FN-4, FN-6, FN-7, FN-11, FP-1, FP-2, FP-3, FP-4.
FN-3/FN-4 exigem `git diff` real via fixture em CI (não apenas lista de strings).

**Falso positivo (gate exige success quando não precisaria — fail-closed, benigno):**

| Vetor | Arquivo(s) no diff | Resultado esperado | Notas |
|---|---|---|---|
| FP-1 | `docs/CONTRIBUTING.md` | `diff_is_inert=true` | todos os braços dispensados |
| FP-2 | `vault/notes/index.md` | `diff_is_inert=true` | — |
| FP-3 | `README.md` (raiz, sem `/`) | `diff_is_inert=true` | — |
| FP-4 | `CHANGELOG.md` (raiz, sem `/`) | `diff_is_inert=true` | parity-other-gates ainda cobre |
| FP-5 | `docs/résum\303\251.md` (não-ASCII, sem -z) | `diff_is_inert=false` (quoted, não casa) | FP benigno; -z resolve |
| FP-6 | rename `vault/x.md` → `vault/y.md` | `diff_is_inert=true` | ambos no conjunto inerte |

### ML-1B — Prazo do PowerShell derivado do aquecimento

**Contagem de chamadas afirmadas (PowerShell):**
```
guardShellMatrix:  4 guards (git-branch, credential, write-scope, credential-global)
psEntries:         2 binários (pwsh, powershell)
cenários por guard:4 (RC0-allow, RC2-deny, RC1-deny-normalizado, absent)
Total:             4 × 2 × 4 = 32 chamadas afirmadas via psAssertedTimeout
TestRunGuardLinePS_TimeoutDiscriminado: 2 chamadas adicionais (pwsh)
```

**D3 (`clamp(4 × warmup, 15s, 60s)`) — atribuição de erro**

**Sem D3** (prazo fixo 15 s): `32 × 15 s = 480 s < 1200 s` (limite de `-timeout 20m` por pacote).
Sem risco de panic; a atribuição é ruim (mensagem genérica de estouro, sem mencionar qual guard).

**Com D3** (cap 60 s): `32 × 60 s = 1920 s > 1200 s`. O cap de 60 s introduz o risco de panic no
pacote `internal/generators`. O que ADR D3 chama de "o estouro só muda de lugar" é exatamente isso:
D3 move o panic de "estouro do prazo fixo" para "estouro do `-timeout 20m`" quando muitas chamadas
pendem ao cap. Um run onde todas as chamadas pendem é uma regressão e deve ser vermelho; o problema
é a qualidade do erro (panic genérico vs. mensagem que cita warmup e guard).

O único ponto medido do aquecimento é o do issue #665 (powershell 5.1: 8,00 s com estouro; 1,91 s
no re-run). O aquecimento de `pwsh` não foi medido por hades nesta sessão.

**Conflito de arquivo entre ML-1B e ML-1A — Decisão B para Zeus:**

Opção (a): ML-1A bump do `-timeout 20m` em `quality.yml`. ML-1B fica disjunto. O job
`windows-full-suites` tem `timeout-minutes: 60` (quality.yml:147), compartilhado por todos os
packages de `go test -p 1 ./...` e as outras suites. Um bump do `-timeout` por pacote aumenta o
budget de `internal/generators`; outros packages continuam com 20 m cada. Esta opção está alinhada
com o ADR D3 literal ("o `-timeout` do `go test` no `windows-full-suites` comporte o novo teto ×
número de chamadas").

Opção (b): ML-1B implementa early-exit no loop de guards após a primeira chamada que expira o prazo,
emitindo a mensagem que cita o warmup e o prazo. Sem tocar `quality.yml`. Pior caso de um guard
problemático: `1 chamada × 60 s = 60 s` por execução (não 32 × 60 s). Esta opção requer que Zeus
**emende** ADR D3 para trocar "comporta o teto × número de chamadas" por "comporta o pior caso que
o teste pode produzir" — e ML-1B deve registrar essa emenda no relatório.

Recomendação de hades: opção (b) — early-exit é semanticamente correto (uma chamada que expira é
uma falha; continuar testando outros guards não agrega). Mas exige emenda ao ADR D3 e ao critério 3
de ML-1B; Zeus deve aprovar antes de ML-1B iniciar.

**Falso negativo:** guard regride e cada chamada leva 25s a mais que antes, numa máquina com warmup
8s (assertedTimeout=32s). 25s < 32s → o regresso passa. Esta é a troca aceita pelo ADR D3.

**Falso positivo:** máquina com warmup 15s → assertedTimeout=60s. Chamada normal em 2s → passa com
folga. A fórmula `4 × warmup` absorve a máquina lenta sem exigir regressos.

### ML-1C — Porta livre no teste de endereço do serve

**Falso negativo (retry mascara defeito real):**

| Vetor | O que acontece | Deve reprovar |
|---|---|---|
| FN-C1 | Serve inicia e bind à porta errada (`127.0.0.1` em vez de `0.0.0.0`) | Sim — lsof detecta |
| FN-C2 | Erro de bind por outro motivo que não `address already in use` (permissão) | Sim — sem retry |
| FN-C3 | Porta "livre" mas serve demora além do wait_timeout para imprimir URL | Não é EADDRINUSE — sem retry, reprova como timeout |

**Restrição crítica:** retry SOMENTE quando o serve tenta o bind, falha com `address already in use`
em stderr, e sai antes de imprimir a URL.

**Falso positivo (retry dispara quando deveria):**

| Vetor | O que acontece | Deve passar |
|---|---|---|
| FP-C1 | Porta ocupada (como PR #666) | Sim — retry escolhe nova porta |
| FP-C2 | Terceira tentativa bem-sucedida após duas colisões | Sim |
| FP-C3 | Deterministic self-collision (3 tentativas todas falham) | Não — reprova na terceira |

---

## Seção 4 — Residual declarado ✅

**R1 — NOISE pode ocultar o gate de shim em PRs de docs**

Se `validate --json` se tornar não-determinístico a partir de um PR de docs, a comparação vai para
NOISE → VACUOUS_EMPTY. O gate não detecta "shim passou bytes errados" porque não há divergência
nativo-vs-shim. Aceitável: o shim não mudou, não há o que detectar. A cobertura volta no push
para a `main`.

**R2 — Symlink em docs/ com `go:embed` futuro**

Se no futuro um `//go:embed docs/algo` for adicionado ao código, e um PR anterior tiver adicionado
um symlink em `docs/` apontando para código de produto, o classificador classificaria o symlink como
inerte. Mitigação: a adição de qualquer nova diretiva `//go:embed` que inclua um caminho inerte exige
auditoria e atualização do conjunto inerte.

**R3 — Workflow YAML executado na versão HEAD do PR**

O `quality.yml` é executado na versão HEAD do PR (padrão do evento `pull_request`). Um PR que
modifica o workflow pode alterar como o classificador é invocado. Isso é um residual pré-existente
a qualquer gate de CI — não piorado por esta REQ.

**R4 — Nomes com não-ASCII nos caminhos inertes (FP futuro)**

Atualmente não há nomes não-ASCII em `docs/`, `vault/` ou `.claude/agent-memory/`. Se adicionados
e o classificador não usar `git diff -z --name-only`, o caminho citado começa com `"` e é
classificado como aplicável (fail-closed, benigno). Mitigação: usar `git diff -z --name-only
--no-renames` com `read -r -d ""`.

**R5 — Regressão de desempenho próxima ao cap de 60s pode passar**

A asserção afirma exit code e stdin; não mede latência. Regressões de desempenho até o cap de 60s
não são detectadas. O warmup é uma amostra de uma única máquina no issue #665; o fator 4 é palpite
documentado no ADR D3.

**R6 — Probe TOCTOU no retry de porta (D4)**

A verificação "porta livre" antes do bind é sujeita a condição de corrida. O retry é a defesa real,
não a probe. Três colisões consecutivas reprovam corretamente (runner problemático).

**R7 — Classificador quebrado bloqueia o required check**

A partir desta REQ, um job `classify` vermelho (ex: ambiente ubuntu indisponível) torna o required
check `shim-byte-identity-gate` vermelho por construção. Essa é a troca correta: `--self-test` pega
bugs acidentais de classificador. Não pega co-edição deliberada do conjunto inerte e do `--self-test`
no mesmo PR — esse caso só é coberto por revisão do diff.

---

**Itens obrigatórios para ML-1A (bloqueiam implementação):**

1. Classificador de diff lido da BASE SHA, não do HEAD.
2. `--self-test` roda no job `classify` ANTES da classificação de BASE — falha do self-test → job
   falha → gate falha → PR bloqueado.
3. Check `[ -z "$GITHUB_BASE_REF" ] || [ "$GITHUB_BASE_REF" != "$DEFAULT_BRANCH" ]` com `exit 0`
   e `diff_is_inert=false` ANTES do loop.
4. `DEFAULT_BRANCH` passada via `env: DEFAULT_BRANCH: ${{ github.event.repository.default_branch }}`.
5. Diff vazio → `diff_is_inert=false` (fail-closed explícito, `count == 0`).
6. `fetch-depth: 0` no job de classificação (ou fallback explícito: `git diff … || { exit 0 com
   inert=false; }` — ramo não medido nesta sessão).
7. Padrão inerte como prefixo ancorado, não substring.
8. `git diff -z --name-only --no-renames` com `read -r -d ""` (rename FN-4 medido; NUL-delimitado
   para R4).
9. Condição do gate: sempre falha se `CLASSIFY_RESULT != success`; passa como "não aplicável" apenas
   quando `event_name==pull_request && base_ref==DEFAULT_BRANCH && classify.result==success &&
   diff_is_inert==true`; caso contrário, exige `shim-byte-identity.result==success`.
10. A etapa de classificação de BASE trata "script ausente em BASE / BASE SHA indisponível" como D2:
    `diff_is_inert=false` com saída `success` do job `classify` (não falha). Sem isso, o PR de
    ML-1A fica vermelho por construção e o implementador remove o check do item 9.
11. Job `classify` sem `if:` condicional (roda em todo evento, inclusive push). Outputs declarados:
    `diff_is_inert: ${{ steps.<classify-step-id>.outputs.diff_is_inert }}`.

**Decisões para Zeus:**

A. Critério de ML-0A item 1 no roadmap vs. critério do ADR D1: o critério do roadmap diz "não entra
   no wheel nem em script de gate" — o que encolheria o conjunto inerte para apenas
   `.claude/agent-memory/`. O ADR D1 diz "não altera o que o shim-byte-identity mede". Zeus deve
   emendar o critério do roadmap para corresponder ao ADR, ou restringir o conjunto. Esta seção
   não pode ser marcada ✅ enquanto A não for resolvida.

B. Conflito ML-1A/ML-1B sobre `-timeout` em `quality.yml`: opção (a) bump via ML-1A, alinhado com
   ADR D3 literal; opção (b) early-exit em ML-1B, sem tocar `quality.yml`, mas requer emenda ao
   ADR D3 e ao critério 3 de ML-1B. Recomendação: opção (b), mas Zeus deve aprovar antes de
   ML-1B iniciar.
