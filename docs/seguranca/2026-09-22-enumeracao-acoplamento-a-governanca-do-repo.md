# Wave 0 — Enumeração e modelo de ameaça: testes e gates leem a árvore de governança do repositório onde rodam

> ML-0A · Role: hades-tf · Data: 2026-09-22

REQ: `docs/req/REQ-2026-09-22-teste-e-gate-leem-a-arvore-de-governanca-do-repositorio-onde-rodam-e-o-consumidor-nao-consegue-rodar-a-suite.md`

---

## Seção 1 — Completude da enumeração

### Critério aplicado

O critério de inclusão é: *"lê um artefato cujo caminho pertence ao domínio de governança
(`roadmap_dir/**`, `req_dir/**`, `adr_dirs/**`) do repositório onde o artefato roda"*.

Artefatos que leem **código-fonte do produto** (`.gitattributes`, `.claude/commands/`,
`scripts/`, `internal/**`) não entram, mesmo que usem `os.Getwd()` ou `../..` para chegar à
raiz — esses caminhos são invariantes em qualquer clone do source e não dependem da
configuração de governança do usuário.

### Comandos de varredura e saída

**Testes Go — mecanismo 1:** `repoRoot(t)` fora da definição

```
$ grep -rn "repoRoot(" --include='*_test.go' internal/ | grep -v "func repoRoot"
internal/roadmapdoc/roadmapdoc_test.go:250:
    doneDir := filepath.Join(repoRoot(t), "docs", "roadmaps", "done")
```

**Testes Go — mecanismo 2:** `filepath.Abs` com `../..` fora de TempDir

```
$ grep -rn 'filepath\.Abs.*\.\.' --include='*_test.go' internal/
internal/validator/validator_test.go:2245:
    repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
```

**Testes Go — fechamento da população por mecanismo:**

Os caminhos pelos quais um test binary pode alcançar a governança real são enumeráveis e
finitos. Cada mecanismo foi verificado por grep independente:

| Mecanismo | Comando de varredura | Saída (RC) |
|---|---|---|
| `repoRoot(t)` | `grep -rn "repoRoot(" --include='*_test.go' internal/ \| grep -v "func repoRoot"` | 1 sítio: `roadmapdoc_test.go:250` (RC=0) |
| `filepath.Abs` com `../..` | `grep -rn 'filepath\.Abs.*\.\.' --include='*_test.go' internal/` | 1 sítio: `validator_test.go:2246` (RC=0) |
| `runtime.Caller(0)` | `grep -rn 'runtime\.Caller' --include='*_test.go' internal/` | 5 sítios — `ship_test.go:565,932` leem `ship.go` (código-fonte do produto); `ship_test.go:1156`, `root_test.go:157`, `barrier_contract_test.go:36` são helpers de `go build` sem acesso à governança (RC=0) |
| Caminhos absolutos hardcoded | `grep -rl '"/.*docs/roadmaps\|"/.*docs/req\|"/.*docs/adr' --include='*_test.go' internal/` | 4 arquivos: `validator_traceid_test.go` (`dir := t.TempDir()`), `validator_namespacing_test.go` (`dir := t.TempDir()`), `sync_failclosed_test.go` (`chdirTempWithReset(t)` → `t.TempDir()`), `validator_test.go` (sítio 2 já declarado — é o `filepath.Abs("../..") + req/*.md`). Nenhum caminho absoluto novo fora dos sítios já declarados (RC=0) |
| `go:embed` em test files | `grep -rn 'go:embed' --include='*_test.go' internal/` | `render_test.go:574` é comentário; 0 diretivas reais (RC=0) |

Os 5 mecanismos cobrem a população completa. Os dois únicos sítios que alcançam a governança
real são os declarados acima (`roadmapdoc_test.go:250` e `validator_test.go:2246`).

**Scripts de shell:**

```
$ grep -rln 'docs/roadmaps\|docs/req\|docs/adr' scripts/*.sh
scripts/capture-barrier-baseline.sh
scripts/check-agent-models-parity.sh
scripts/check-agent-namespace-union.sh
scripts/check-barrier.sh
scripts/check-gates-falsify.sh
scripts/check-install-version-pin.sh
scripts/check-referential-integrity.sh
scripts/check-roadmap-barrier-contract.sh
scripts/check-req-path-literals.sh
scripts/check-validate-rule-pins.sh
scripts/trackfw-attention-cleanup.sh
scripts/trackfw-attention-signal.sh
scripts/trackfw-git-branch-guard.sh
scripts/trackfw-credential-guard.sh
```

(14 scripts — confirma a medição inicial do arquiteto.)

**Python scripts:** apenas `scripts/check-windows-known-failures.py` menciona `docs/adr/...`
— em comentário de docstring, não em código de execução.

**Workflows `.github/workflows/`:** referências a `docs/adr/...` apenas em comentários YAML
(`#`). Nenhum step lê governança ao vivo.

### Mecanismo de cada script dos 14

| Script | Refs de governança | Mecanismo de fixture |
|---|---|---|
| `capture-barrier-baseline.sh` | 2 | sem mktemp — lê diretamente `docs/roadmaps/{done,wip}` |
| `check-agent-models-parity.sh` | 21 | `WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-agent-models-parity.XXXXXX")` (linha 73) — grava governança em `$WORK` |
| `check-agent-namespace-union.sh` | 50 | `WORK=$(mktemp -d ...)` |
| `check-barrier.sh` | 32 | `WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-barrier.XXXXXX")` (linha 26) com cleanup trap |
| `check-gates-falsify.sh` | 131 | `WORK=$(mktemp -d ...)` |
| `check-install-version-pin.sh` | 1 | ref. em comentário apenas (`docs/roadmaps/wip/ROADMAP-...`) |
| `check-referential-integrity.sh` | 3 | sem mktemp — lê `$REQ_DIR` do projeto executante |
| `check-roadmap-barrier-contract.sh` | 22 | `WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-roadmap-barrier-contract.XXXXXX")` (linha 34); mas linha 512 lê `$ROOT_DIR/docs/roadmaps` (sem guarda de env var) |
| `check-req-path-literals.sh` | 14 | refs. no texto do pattern (alvo de grep), não lidas como arquivos |
| `check-validate-rule-pins.sh` | 25 | `TMP_DIR=$(mktemp -d ...)` |
| `trackfw-attention-cleanup.sh` | 2 | hook de runtime — usa `$ROADMAP_DIR` env do chamador |
| `trackfw-attention-signal.sh` | 2 | hook de runtime — usa `$ROADMAP_DIR` env do chamador |
| `trackfw-git-branch-guard.sh` | 1 | ref. em comentário apenas |
| `trackfw-credential-guard.sh` | 2 | hook de runtime — usa `$ROADMAP_DIR` env do chamador |

### Tabela de sítios classificados

| # | Arquivo:linha | Mecanismo | Artefato acessado | Veredito | Declaração presente? |
|---|---|---|---|---|---|
| 1 | `internal/roadmapdoc/roadmapdoc_test.go:250` | `repoRoot(t)` → `os.Getwd()/../../docs/roadmaps/done` | `roadmap_dir/done/` | **(a)** | N/A |
| 2 | `internal/validator/validator_test.go:2246` | `filepath.Abs("../..") + docs/req/<basename>.md` | 3 REQs hardcoded por nome | **(a)** | N/A |
| 3 | `scripts/check-roadmap-barrier-contract.sh:512` | `find "$ROOT_DIR/docs/roadmaps" -name "$base"` | `roadmap_dir/**` para 144 basenames | **(a)** | N/A |
| 4 | `internal/generators/scaffold_test.go:122` | `filepath.Join(orig, "../..", ".claude/commands/trackfw/roadmap.md")` | Código-fonte do produto | **(b)** | Ausente |
| 5 | `internal/generators/gitattributes_test.go:119` | `filepath.Join("..", "..", ".gitattributes")` | Código-fonte do produto | **(b)** | Ausente |
| 6 | `scripts/check-referential-integrity.sh` | lê `$REQ_DIR` do projeto executante | REQs do projeto onde roda | **(b)** | Parcial |
| 7 | `scripts/capture-barrier-baseline.sh:23-24` | lê `docs/roadmaps/{done,wip}` diretamente | `roadmap_dir/{done,wip}` | **fora de população** |  |
| 8 | `internal/roadmapdoc/compare_baseline_test.go` | `filepath.Join(wd, "testdata", "corpus")` | Fixture congelada em `testdata/` | **(c)** | |
| 9 | `internal/commands/barrier_contract_test.go` | `runtime.Caller(0)` → compilação; governança em `t.TempDir()` | Fixture própria | **(c)** | |
| 10 | `internal/commands/ship_test.go:565,932` | `runtime.Caller(0)` → lê `ship.go` (código-fonte do produto) | Código-fonte do produto (`ship.go`) | **(b)**: mesmo padrão dos sítios 4 e 5 — fora do domínio de governança | Ausente |
| 11 | `scripts/check-agent-models-parity.sh`, `check-agent-namespace-union.sh`, `check-barrier.sh`, `check-gates-falsify.sh`, `check-validate-rule-pins.sh` | `mktemp -d` + geração de fixture própria | Fixture em `$WORK` | **(c)** | |

**Nota sobre `capture-barrier-baseline.sh` (fora de população):** o script produz um arquivo
de saída e não emite veredito de gate (não sai com RC≠0 para sinalizar problema). O critério
da REQ é *"teste ou gate"*; este é um utilitário de captura. Não está em `make quality`.
`check-orphan-gates.sh` não o cobre (padrão `check-*.sh`, não `capture-*.sh`). Observação
retida no Residual R4.

**Nota sobre `check-referential-integrity.sh` (b, declaração parcial):** o cabeçalho
documenta o defeito prevenido (`req_dir` hardcoded) e o fallback para `docs/req`, mas não
declara explicitamente *"este gate audita a governança do projeto onde roda e passa
corretamente quando `req_dir` está vazio"* — a condição de vazio está implementada mas não
contratualizada.

**Nota sobre os sítios (b) de código-fonte (4, 5 e 10):** nenhum dos três declara que lê
código-fonte do produto. Os comentários explicam o que testam (contenção do bloco gerado,
igualdade com template canônico, invariantes de `ship.go`) mas não dizem que o artefato lido
pertence ao source do produto e não à governança do consumidor. Dívida de documentação retida
no Residual R5.

Contagem final: **(a) = 3, (b) = 4 (sítios 4, 5, 6, 10), (c) = 7 (sítios 8, 9 — 2 testes Go — mais os 5 scripts do sítio 11), fora de população = 1**.

---

## Seção 2 — O critério que distingue (a) de (b)

### Por que o critério do arquiteto está incompleto

A formulação candidata do roadmap — *"rodaria em clone sem nenhum arquivo em `docs/`?"* —
**subestima** a classe (a) e **não se aplica** a artefatos fora de `docs/`.

- Em modo `by_agent`, `docs/` tem conteúdo (`docs/roadmaps/<agente>/done/`). O teste passaria
  na formulação candidata mesmo que `docs/roadmaps/done/` (flat) não exista. O defeito persiste.
- `.gitattributes` e `.claude/commands/trackfw/roadmap.md` não ficam sob `docs/`, então a
  formulação não os cobre — e eles devem ser (b), não (a).

### Critério correto (aplicável por terceiro, sem julgamento)

**Pergunta 1 — o artefato pertence ao domínio de governança configurável?**

Um artefato pertence ao domínio de governança se seu caminho começa com `roadmap_dir`,
`req_dir`, ou um dos `adr_dirs` da configuração `trackfw.yaml`. Caminhos fora desses domínios
(`.gitattributes`, `.claude/`, `scripts/`, `internal/`, `go.mod`) são código-fonte do produto.

**Pergunta 2 — o conteúdo varia entre o mantenedor e qualquer consumidor?**

Se invariante em qualquer clone completo do source (o gerador produz exatamente ele, ou ele é
código-fonte versionado), ele audita o upstream → **(b)**. Pare.

Se varia (cada projeto tem roadmaps, REQs e ADRs diferentes), prossiga para a P3.

**Pergunta 3 — aplica-se quando P1=SIM e P2=varia: o artefato resolve o caminho via config E
tolera qualquer estado válido do domínio (incluindo vazio e layout `by_agent`)?**

Ambas as condições devem ser verdadeiras simultaneamente — não basta uma:

- "Resolve via config" = lê `trackfw.yaml` (ou variável de ambiente equivalente) para
  determinar `roadmap_dir`/`req_dir`/`adr_dirs`; não hardcoda o caminho do mantenedor.
- "Tolera qualquer estado válido" = não falha quando o domínio está vazio ou tem layout
  diferente do mantenedor (`by_agent`, `by_team`, etc.).

🔴 **Guarda contra o adversário:** quando a resposta a P3 for arguível em prosa, o árbitro
é o teste de execução da Seção 2 — copiar o source para árvore temporária com
`roadmap_namespacing: by_agent`, sem os subdiretórios planos do mantenedor, e executar o
artefato. RC=0 → P3=SIM. RC≠0 → P3=NÃO. **Prosa não substitui RC.**

Se P3=SIM → gate projetado por design para auditar a governança de quem executa. → **(b)**.

Se P3=NÃO (um dos dois critérios falha) → defeito. → **(a)**.

**Teste de falsificação prático** (executável por qualquer terceiro sem julgamento):

> Copie a árvore de source completa do trackfw para um diretório temporário. Nesse diretório,
> crie um `trackfw.yaml` com `roadmap_namespacing: by_agent`, `agents: [hades-tf]`, e
> `roadmap_dir: docs/roadmaps`. Não crie `docs/roadmaps/done/` (apenas
> `docs/roadmaps/hades-tf/done/`). Rode o artefato a partir desse diretório. Se reprovar →
> **(a)**, a menos que reprovar seja o contrato declarado do próprio artefato (e nesse caso o
> contrato deve estar escrito nele). Se passar → **(b)/(c)**.

Para distinguir (b) de (c): (b) leu pelo menos um caminho sob `roadmap_dir`/`req_dir`/`adr_dirs`
(ou um arquivo que só existe na raiz-fonte). (c) nunca saiu de `testdata/` ou `t.TempDir()`.

### Aplicação do critério a cada sítio

**Sítio 1** (`TestCorpusMeasurement_ReportOnly`): lê `roadmap_dir/done/` (domínio de
governança). Conteúdo varia por projeto. Falha empiricamente. **(a)**.

**Sítio 2** (`TestExtractRefPath_TresREQsReaisDoRepositorio`): lê `req_dir/REQ-*.md` por nome
hardcoded (domínio de governança). O nome do teste declara *"reais do repositório"*. Em modo
`by_agent`, REQs vão para `req_dir/<agente>/` — os 3 arquivos hardcoded nunca existiriam no
`req_dir/` raiz de um consumidor `by_agent`. Falha empiricamente. **(a)**.

**Sítio 3** (`check-roadmap-barrier-contract.sh:512`): `find "$ROOT_DIR/docs/roadmaps" -name
"$base"` itera 144 basenames do snapshot. `roadmap_dir` é domínio de governança. Conteúdo
varia. **(a)**.

**Sítio 4** (`scaffold_test.go`): lê `.claude/commands/trackfw/roadmap.md`. Fora do domínio
de governança. Invariante em qualquer clone. Testa que o gerador produz o template canônico.
**(b)**.

**Sítio 5** (`gitattributes_test.go`): lê `.gitattributes`. Fora do domínio de governança.
Invariante em qualquer clone. Testa contenção do bloco gerado por `init`. **(b)**.

**Sítio 6** (`check-referential-integrity.sh`): lê `$REQ_DIR` do projeto executante. P1=SIM
(domínio de governança). P2=varia (cada projeto tem REQs diferentes). P3: o script resolve
`$REQ_DIR` a partir de `trackfw.yaml` (com fallback `docs/req`) e usa
`find "$ROOT_DIR/$REQ_DIR" -maxdepth 2 -name "*.md"` — `maxdepth 2` cobre tanto o layout plano
quanto `by_agent` (onde REQs ficam em `req_dir/<agente>/*.md`). Medição:

```
# Árvore fake com roadmap_namespacing: by_agent, REQs em docs/req/hades-tf/
$ "$FAKE/scripts/check-referential-integrity.sh" > out 2> err; echo "RC=$?"
RC=0
stdout: Referential integrity OK
stderr: (vazio)
```

P3=SIM (ambas as condições verificadas por execução). **(b)**: gate projetado para auditar
a governança de quem executa, tolerante de qualquer layout válido.

**Sítio 10** (`ship_test.go:565,932`): usa `runtime.Caller(0)` para localizar `ship.go`
(código-fonte do produto no mesmo pacote) e lê seu conteúdo. P1=NÃO — `ship.go` não pertence
ao domínio de governança (`roadmap_dir`/`req_dir`/`adr_dirs`). **(b)**: mesmo padrão dos
sítios 4 e 5.

---

## Seção 3 — Modelo de ameaça

### Adversário nomeado

O adversário é o **implementador apressado e o arquiteto otimista** — não um atacante externo.

O caminho óbvio para esvaziar esta Wave 0 sem quebrar nenhuma regra escrita:

**Reclassificar tudo como (b)** com o argumento *"é gate do upstream, é legítimo"*. A Wave 0
fica verde, os 3 sítios permanecem, e o consumidor continua não conseguindo rodar a suíte.
Este foi o padrão medido 59 vezes em 30 roadmaps (REQ-2026-09-03).

**Como o adversário procede:**
1. Argui que `TestCorpusMeasurement_ReportOnly` é (b) porque *"mede o corpus do mantenedor"*.
2. Argui que `TestExtractRefPath` é (b) porque *"os arquivos estão commitados no source"*.
3. Argui que `check-roadmap-barrier-contract.sh:512` é (b) porque *"a tripwire protege o
   corpus congelado de divergir do disco"*.

**Por que o argumento falha em todos os três:**

Para o sítio 1: o corpus que o teste mede está no `roadmap_dir` — não é código-fonte. A
afirmação *"mede o corpus do mantenedor"* é precisamente a definição de (a): o consumidor não
tem esse corpus.

Para o sítio 2: o critério não é *"os arquivos estão presentes em algum clone"* mas *"o
caminho pertence ao domínio de governança e varia entre projetos"*. Em modo `by_agent`, REQs
vão para `req_dir/<agente>/`, não para `req_dir/` direto. O teste usa `t.Fatalf`, sem skip.
Empiricamente reprovado com RC=1.

Para o sítio 3: o loop em `check-roadmap-barrier-contract.sh:508-523` é incondicional — não
há guarda de env var. O próprio script documenta o experimento de apagar 1 arquivo (comentário
~519-527): *"apagando UM roadmap do disco, o gate emitia 6 falhas"*. Para 144 arquivos ausentes
(consumidor sem nenhum roadmap do mantenedor), o gate emite 144 falhas de
`corpus/basename-missing-from-disk`.

### O teste que o adversário não pode passar

O teste está descrito na Seção 2. Execuções reais:

**Sítio 1 — RC=1 medido:**
```
$ go test -c -o /tmp/roadmapdoc.test ./internal/roadmapdoc/
$ mkdir -p /tmp/fake/internal/roadmapdoc
$ mkdir -p /tmp/fake/docs/roadmaps/hades-tf/done   # layout by_agent
# sem docs/roadmaps/done/ (flat)
$ cd /tmp/fake/internal/roadmapdoc
$ /tmp/roadmapdoc.test -test.run TestCorpusMeasurement_ReportOnly -test.v
--- FAIL: TestCorpusMeasurement_ReportOnly (0.00s)
    roadmapdoc_test.go:253: ReadDir .../fake/docs/roadmaps/done:
        open .../fake/docs/roadmaps/done: no such file or directory
FAIL
RC=1
```

**Sítio 2 — RC=1 medido:**
```
$ go test -c -o /tmp/validator.test ./internal/validator/
$ mkdir -p /tmp/fake2/internal/validator /tmp/fake2/docs/req
$ echo "# Consumer own REQ" > /tmp/fake2/docs/req/REQ-consumer.md
$ cd /tmp/fake2/internal/validator
$ /tmp/validator.test -test.run TestExtractRefPath_TresREQsReaisDoRepositorio -test.v
--- FAIL: TestExtractRefPath_TresREQsReaisDoRepositorio (0.00s)
    validator_test.go:2260: erro ao ler REQ real "docs/req/REQ-2026-07-27-...":
        open .../fake2/docs/req/REQ-2026-07-27-...: no such file or directory
FAIL
RC=1
```

**Sítio 3 — trecho do script (verificado no código-fonte; sem env guard):**

```bash
# check-roadmap-barrier-contract.sh:508-527 (trecho literal)
elif [[ -n "${CORPUS_FILELIST:-}" ]]; then
  MISSING_FROM_DISK=""
  while IFS= read -r snapshot_path; do
    base=$(basename "$snapshot_path")
    # Basename ausente do disco (docs/roadmaps/**) reprova: o corpus congelado referencia um
    # arquivo que já não existe na árvore de trabalho em NENHUM estado (wip/done/backlog/...).
    on_disk=$(find "$ROOT_DIR/docs/roadmaps" -type f -name "$base" | head -n1)
    if [[ -z "$on_disk" ]]; then
      MISSING_FROM_DISK="${MISSING_FROM_DISK}${MISSING_FROM_DISK:+, }$base"
      # ...
    fi
    CORPUS_FILES=$((CORPUS_FILES + 1))
    cp "$snapshot_path" "$CORPUS_SANDBOX/docs/roadmaps/wip/$base"
```

O loop itera `$CORPUS_FILELIST` (144 basenames do snapshot em `scripts/testdata/`). Não há
nenhuma guarda de env var antes do bloco `elif` que inicia o loop. Para consumidor sem nenhum
dos 144 roadmaps do mantenedor: `on_disk` vazio para cada iteração → `MISSING_FROM_DISK`
acumula todos os 144 nomes → `fail "corpus/basename-missing-from-disk"` para cada um.
O próprio comentário no código documenta o efeito: *"apagando UM roadmap do disco, o gate
emitia 6 falhas"* — para 144 ausentes, o gate emite 144 falhas.

---

## Seção 4 — Falsificação nas duas direções

Para cada sítio (a), o que se perde quando o produto **regride** (falso negativo / falha
silenciosa) e o que se perde quando a **correção vai longe demais** (falso positivo = regressão
silenciosa no upstream).

### Sítio 1: `TestCorpusMeasurement_ReportOnly`

**Direção 1 — regressão (t.Fatalf volta):**
O teste falha em qualquer consumer `by_agent`. Gate que captura: o próprio teste, executado
num clone `by_agent`. Ausência desse exercício na CI do mantenedor é o motivo pelo qual a
falha viveu até o #396.

**Direção 2 — sobre-correção (skip incondicional):**
Se transformado em `t.Skip(...)` incondicional, o log
`done/ corpus: total=194, unfinished(...)=25` desaparece da CI do mantenedor. O arquiteto
perde visibilidade sobre roadmaps em `done/` com MLs genuinamente inacabados.

Discriminante para a correção correta *(input para ML-1A, não a decisão final)*: a saída do
teste no upstream deve continuar imprimindo a contagem. O skip só deve ocorrer quando o
diretório não existe — `os.Stat(doneDir)` antes do `ReadDir`; se `IsNotExist`, `t.Skip` com
mensagem. Se o diretório existe mas `ReadDir` falha, `t.Fatal` permanece (erro real). O braço
que conta no upstream não muda.

### Sítio 2: `TestExtractRefPath_TresREQsReaisDoRepositorio`

**Direção 1 — regressão:**
Qualquer dos 3 REQ files volta a ser lido com `t.Fatalf` sem guarda. Consumer sem esses
arquivos falha. Sinal: RC=1 no package `internal/validator`.

**Direção 2 — sobre-correção (fixture vazia ou nenhum teste):**
A propriedade que o teste afirma é: *"REQs com ADR referenciado somente por backtick no corpo
(não em frontmatter) são resolvidas pelo `extractRefPath`"*. Essa é uma propriedade de parsing
do produto. Se os 3 REQs forem removidos sem substituição, o comportamento de parsing deixa
de ter cobertura de regressão.

Discriminante para a correção correta *(input para ML-1A)*: substituir as 3 leituras de disco
por fixtures inline ou arquivos em `testdata/` com conteúdo idêntico (byte-a-byte) ao dos 3
REQs reais no momento da correção. O upstream deve manter uma asserção separada de que ao
menos 1 REQ real em `req_dir/` ainda satisfaz a propriedade de backtick-only ADR ref (controle
de não-regressão por corpus, distinto do teste unitário).

### Sítio 3: `check-roadmap-barrier-contract.sh:512` (tripwire de disco)

**Direção 1 — regressão:**
A `on_disk` check desaparece e um basename do corpus congelado é apagado do disco sem que
ninguém perceba. A integridade declarada do corpus (144 arquivos existem como arquivos vivos
no working tree) não é mais verificada.

**Direção 2 — sobre-correção (tripwire morta):**
A proposta do #277 — mover a tripwire atrás de `TRACKFW_SELF_GOVERNED=1` — introduz um
controle que nunca é exercido, pois nenhum job do CI seta essa variável:

```
$ grep -rn "TRACKFW_SELF_GOVERNED" .github/ Makefile scripts/
(nenhuma saída — RC=1)
```

Um controle morto é mais perigoso que a ausência de controle porque parece estar protegido.

**Avaliação da proposta #277 — o que generaliza e o que não generaliza:**

A metade da proposta que generaliza: *"corpus vira fixture em `scripts/testdata/`"* — já
aconteceu (o snapshot está em `scripts/testdata/roadmap-barrier-corpus-snapshot/`). O gate já
lê bytes do snapshot, não do disco vivo. O hash AC10 já é imune ao crescimento do corpus.

A metade que requer atenção: a **tripwire de disco** (verificar que o snapshot não divergiu
do working tree). Ela é genuinamente uma verificação sobre a governança do upstream — *"o
corpus não foi truncado"*. Duas alternativas seguras:

1. **Makefile target separado** (`make parity-upstream`) chamado pelo CI via step dedicado com
   nome explícito. A separação é estrutural, não depende de env var não documentada.
2. **Env var com gate de vacuidade**: setar `TRACKFW_SELF_GOVERNED=1` num step do CI, e
   adicionar um gate que reprova se a variável estiver vazia durante `parity-rest`. Essa
   opção fecha o falso negativo que a proposta original deixa aberto.

A escolha entre as duas pertence ao ML-1B — aqui fica o discriminante: *qualquer solução
adotada deve ter evidência de que a tripwire continua sendo executada no CI do mantenedor*.

---

## Seção 5 — Residual declarado

**R1 — Forks que deletam governance artifacts versionados.**
Se um fork do trackfw apaga os 3 REQs de 2026-07-27 do `docs/req/`, o sítio 2 continua sendo
(a) mesmo após a correção (que usa fixture inline). Esse é o comportamento correto — a fixture
captura os bytes dos REQs reais no momento da correção, e o fork não tem os originais. A REQ
não manda proteger forks que deletam artefatos de origem.

**R2 — Novos sítios criados em waves futuras.**
Esta enumeração é fechada para a população de 2026-09-22. Um ML futuro pode criar novos sítios
(a). O critério da Seção 2 é suficiente para identificá-los. A CI do consumidor `by_agent`
(AC de fechamento do roadmap) é o detector natural de regressão.

**R3 — CI do mantenedor não exercita o consumer scenario.**
O CI atual roda em `roadmap_namespacing: flat`. Mesmo após corrigir os 3 sítios, nenhum job do
CI prova que um consumidor `by_agent` consegue rodar a suíte. O critério de aceite do roadmap
exige essa prova explicitamente — ela fica fora do escopo desta Wave 0 e é entregável da Wave 1
(barreira final).

**R4 — `capture-barrier-baseline.sh` sem consumidor automático.**
O script lê `docs/roadmaps/{done,wip}` e não está em `make quality`. `check-orphan-gates.sh`
não o cobre (padrão `check-*.sh`, não `capture-*.sh`). Nenhuma regra atual exige que
utilitários manuais tenham consumidor. Aceito como residual.

**R5 — Sítios (b) sem declaração.**
Os sítios 4, 5 e 10 (`scaffold_test.go`, `gitattributes_test.go`, `ship_test.go:565,932`) leem
código-fonte do produto mas não declaram isso. O risco é que um revisor futuro os reclassifique
como (a) por desconhecer o critério. Não é um defeito de produto — é dívida de documentação.
Pode ser resolvido com um comentário de uma linha em cada teste: *"lê arquivo de source
versionado do produto, não governança do consumidor"*.

---

*Hades, Security Reviewer — hades-tf*
