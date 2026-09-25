# Ponto único de contenção, e o instrumento que o prova

> ML-6A (Wave 0 da reabertura) · REQ-2026-08-31 · `hades-tf` · 2026-09-25
> Este documento **mede e desenha**. Não contém linha de implementação.

---

## 0. Refutações primeiro

Quatro. As três primeiras mudam ACs já escritos; a quarta é um achado novo e é a mais grave.

### R-1 🔴 Existe escape VIVO hoje: `trackfw adr new` escreve fora do projeto

Não é "inconsistência com risco". É o defeito do título da REQ, **reproduzido contra o binário de
hoje** (`bin/trackfw`, mtime 2026-09-25 09:30), em duas arms que diferem só na forma do `$PWD`:

```
$ D=/tmp/hades-esc-...; OUT=/tmp/hades-victim-...
$ ln -s "$OUT" "$D/docs"          # 'docs' é symlink para FORA do projeto

--- arm=resolved   (PWD desfeito -> os.Getwd() devolve /private/tmp/...)
trackfw: refusing write to /private/tmp/.../docs/adr: refusing symlink path "/private/tmp/.../docs"
Error: refusing write to ...
RC=1
files written OUTSIDE the project:        (vazio)

--- arm=pwd-unresolved   (PWD=/tmp/... — o que um shell faz sozinho no `cd`)
created docs/adr/ADR-2026-09-25-escape-probe.md
RC=0
files written OUTSIDE the project:
ADR-2026-09-25-escape-probe.md
```

E **sem nenhuma manipulação de ambiente**, num `cd` normal de login shell:

```
$ bash -lc "cd $D && trackfw adr new 'natural probe'"
PWD=/tmp/hades-nat-71457
created docs/adr/ADR-2026-09-25-natural-probe.md
RC=0
outside the project:
ADR-2026-09-25-natural-probe.md
```

**Mecanismo.** `internal/generators/adr.go:47-50`:

```go
guardRoot := absAdrDir                        // default: escopo global
if pr, prErr := projectRoot(); prErr == nil { // pr é EvalSymlinks-resolvido
    if pathguard.Beneath(pr, absAdrDir) {     // absAdrDir é filepath.Abs — NÃO resolvido
        guardRoot = pr                        // escopo de projeto
    }
}
```

`Go os.Getwd()` honra `$PWD` quando ele `stat`-bate com `.`, então devolve `/tmp/x` (medido:
`Getwd: /tmp/hades-adr-probe2-70636` · `Eval : /private/tmp/hades-adr-probe2-70636`). Logo
`pr = /private/tmp/x` e `absAdrDir = /tmp/x/docs/adr`. `Beneath` compara **namespaces diferentes**,
dá `false`, e o código **cai silenciosamente no escopo global** — que por desenho só olha symlinks
*dentro* de `adrDir` e **nunca inspeciona o ancestral `docs`**. A armadilha 3 da Decisão 2 previa
*falso positivo*; a implementação a resolveu **degradando o controle**, que é pior.

**Fronteira medida** (não over-claim — só `adr` escapa):

| comando | rc | arquivos escapados | primeira linha |
|---|---|---|---|
| `adr new` | 0 | **1** | `created docs/adr/ADR-...md` |
| `req new` | 1 | 0 | `refusing write to .../docs/req: refusing symlink path ".../docs"` |
| `roadmap new` | 1 | 0 | `req_dir "docs/req" resolves outside project root — path traversal refused` |
| `note new` | 0 | 0 | escreve em `vault/notes`, fora do `docs` isca |

🔴 **Consequência de governança:** pela Regra Dura, isto é **mesma causa, mesma REQ** — vira ML
nesta reabertura, não issue.

### R-2 O "13" é régua de identificador, tal como o "9". A régua do mecanismo dá mais

O handoff está certo em que `grep Clean(cwd)` acha 9 e perde o 13º chamado `root`. Mas
`grep 'RejectSymlinks(filepath.Clean'` **também é texto**. A régua do mecanismo é *"o primeiro
operando não deriva de fonte resolvida"*, e ela inclui sítios sem nenhum `filepath.Clean` escrito:

```
$ grep -rn 'RejectSymlinks(filepath.Clean' --include='*.go' internal/ | wc -l
13
$ grep -rn 'pathguard\.RejectSymlinks(' --include='*.go' internal/ | grep -v _test.go | wc -l
52
```

Os 39 restantes passam variáveis (`root`, `home`, `syncRoot`, `exportRoot`…). Rastreei todas —
ver §1.

### R-3 O "3 cópias byte-idênticas" do #401 é falso, e o "4" do handoff também subconta

Confirmo a refutação do handoff **e a estendo**. Byte-identidade, medida:

```
$ sed -n '86,93p' internal/generators/scaffold.go | sed 's/rejectScaffoldPath/REJ/' > a
$ sed -n '216,223p' internal/commands/discover.go | sed 's/rejectDiscoverPath/REJ/' > b
$ diff a b && echo IDENTICAL
IDENTICAL (modulo name)
```

2 cópias exatas + 2 variantes = 4 **helpers nomeados**. Mas `^func reject` é outra régua de
identificador: o par *predicado + recusa audível* está **inline** na esmagadora maioria dos sítios.

```
$ grep -rn -A2 'pathguard\.RejectSymlinks(' --include='*.go' internal/ | grep -v _test.go \
    | grep 'Fprintf(os.Stderr' | wc -l
49
```

### R-4 O AC5 está medidamente NÃO atendido — a REQ foi para `done` com ele aberto

O AC5 exige *"a recusa e a mensagem são **idênticas** em todos os sítios de escrita do Go"*.
Medido — **5 gramáticas distintas**:

```
$ grep -rn -A2 'pathguard\.RejectSymlinks(' --include='*.go' internal/ | grep -v _test.go \
    | grep 'Fprintf(os.Stderr' | sed 's/.*Fprintf(os.Stderr, //;s/,.*//' | sort | uniq -c | sort -rn
  41 "trackfw: refusing write to %s: %v\n"
   4 "trackfw: refusing symlink path %s: %v\n"
   2 "trackfw: refusing write to %s/.husky: %v\n"
   1 "trackfw roadmap move: refusing symlink path %s: %v\n"
   1 "aviso: %s — trackfw discover não escreve através de symlinks — arquivo não foi tocado\n"
```

🔴 **Régua declarada:** *literal de format-string distinto* na chamada `Fprintf(os.Stderr, …)` que
está no **ramo de falha da própria `RejectSymlinks`** (≤2 linhas). Duas objeções previsíveis, ambas
medidas e respondidas:

- *"`%s/.husky` é a mesma gramática com sufixo interpolado"* — não é: o sufixo está **dentro** do
  literal, então o chamador não pode produzi-lo variando o argumento. São formatos distintos.
- *"o `aviso:` do discover é o alerta de `Lstat` de folha, não a recusa de contenção"* — **medido e
  refutado**, é o ramo de falha da guarda:

```
internal/discover/discover.go:355:  if guardErr := pathguard.RejectSymlinks(root, filepath.Join(root, ".github", ...)); guardErr != nil {
internal/discover/discover.go:356:      fmt.Fprintf(os.Stderr, "aviso: %s — trackfw discover não escreve através de symlinks — ...
```

A única diferença é ser **não fatal** por contrato de `InstallGates` — o que é *mais* uma dimensão
de divergência, não menos. **5 gramáticas confirmadas.**

E o AC4 (*recusa audível*) tem **3 sítios silenciosos** (nenhum `os.Stderr` nas 5 linhas seguintes):

| sítio | natureza |
|---|---|
| `internal/integrations/manager.go:762` | delegate de 1 linha; os chamadores (745, 749) retornam erro sem stderr |
| `internal/generators/roadmap.go:829` | log de transição — `return` mudo |
| `internal/generators/req.go:474` | log de transição — `return` mudo |

---

## 1. Enumeração completa — pelo mecanismo

### 1.1 A tese: por que o AC foi dado por satisfeito por um instrumento que não prova nada

Esta é a pergunta do ML, e a resposta não é por sítio — é estrutural:

> 🔴 **Todo AC desta REQ delimita a sua população por IDENTIFICADOR. Um AC de população
> identificada é satisfazível sem tocar no mecanismo — basta consertar exatamente os nomes
> listados.**

O mesmo defeito, em quatro níveis, todos medidos:

| nível | população que o AC nomeia | o que sobrevive à satisfação do AC |
|---|---|---|
| o gate `check-write-containment` (#400) | "cada sítio tem marcador" | marcador copiado sem a guarda → 22 gaps + 7 marcadores falsos, gate verde |
| Wave 8 | "os **13** sítios" | os sítios de root não resolvido sem `filepath.Clean` escrito (§1.2) |
| ML-7A | "os **4** sítios passam a delegar" | as **49** pares inline com **5** gramáticas (R-3, R-4) |
| ML-7B | "o analisador de AST" | o analisador aprova `RejectSymlinks(Clean(cwd), p)`: o fluxo passa (§3.4) |

Corrigir por lista de nomes é o que fecha a REQ com o mecanismo vivo. **A correção é trocar toda
população nomeada por um predicado mecânico com piso contado.**

### 1.2 População (a) — root não derivado de fonte resolvida

🔴 **Duas colunas separadas de propósito**: *expressões de guarda* e *sítios de escrita alcançados*.
Confundir as duas é exatamente como nasceram 9, 12 e 13.

Resolvedores aprovados medidos na árvore: `projectRoot()` (`scaffold.go:21-32`, `EvalSymlinks`),
`scaffoldRoot()` (`scaffold.go:71-82`), `resolveRoot()` (`discover/discover.go:23-33`), e
`EvalSymlinks` inline (`metrics.go:212`, `validator.go:65`, `configure.go:160`, `sync.go:100`,
`commands/discover.go:37`).

| # | sítio (expressão de guarda) | root | fonte | escritas alcançadas | nas contagens 9/12/13? |
|---|---|---|---|---|---|
| a1–a9 | `agentfiles.go` 137, 241, 459, 612, 795, 928, 1107, 1357, 1473 | `filepath.Clean(cwd)` | `os.Getwd()` | 9 | **9 ✓ · 12 ✓ · 13 ✓** |
| a10–a12 | `update.go` 201, 226, 290 | `filepath.Clean(cwd)` | `os.Getwd()` | 3 | 9 ✗ · **12 ✓ · 13 ✓** |
| a13 | `update.go:2143` | `filepath.Clean(root)` | `root` = `cwd` via `update.go:110` | 1 | 9 ✗ · 12 ✗ · **13 ✓** |
| **a14** | `update.go:666` `rejectHarnessSymlink(home, …)` | `home` | `homedir.Dir()` — **nunca resolvido** | **15** (689, 728, 763, 822, 936, 1036, 1147, 1291, 1406, 1503, 1575, 1647, 1719, 1791, 1876) | ✗ ✗ ✗ |
| **a15–a16** | `manager.go:745`, `749` via `rejectSymlinks` (762) | `m.ProjectRoot` \| `m.HomeDir` | `os.Getwd()` / `homedir.Dir()` em `init.go:438`, `update.go:162`, `update.go:2300`, `integrations_flags.go:437` | 2 | ✗ ✗ ✗ |
| **a17–a20** | `adr.go` 53, 124, 255, 314 | `absAdrDir` (`filepath.Abs`) **no ramo global** | condicional — ver R-1 | 4 | ✗ ✗ ✗ |

**Totais pela régua do mecanismo: 20 expressões de guarda · 34 sítios de escrita alcançados.**

⚠️ **Estes 34 NÃO são os "34 defeitos" do #400.** Coincidência aritmética, populações
independentes: aqui são *escritas alcançadas por guarda de root não resolvido*; lá são
*22 gaps de folha + 7 marcadores falsos + 5 guardas fail-open*. Nenhum dos dois deriva do outro.

**A divergência 9/12/13 explicada pela régua:**

| contagem | régua efetivamente usada | o que ela perde |
|---|---|---|
| **9** (#402) | `grep Clean(cwd)` **em `agentfiles.go`** — arquivo + literal | os 3 de `update.go` com o mesmo literal, e todo o resto |
| **12** | `grep Clean(cwd)` em `internal/` — literal só | o `Clean(root)`: a variável mudou de nome |
| **13** | `grep RejectSymlinks(filepath.Clean` — literal da chamada | `home`, `ProjectRoot`, `absAdrDir`: **nenhum escreve `Clean`** |
| **20 / 34** | **provenância do 1º operando** | (é a régua do mecanismo; ver residual §4.2) |

🔴 **Calibragem honesta (a17–a20 são a exceção):** em a1–a16, root e alvo derivam **da mesma base
não resolvida**, ficam no mesmo namespace e **não há defeito vivo hoje** — é sub-censo e
inconsistência com risco, exatamente como o #402 diz de si mesmo. **a17–a20 é diferente**: root
resolvido contra alvo `Abs`-only, e é o escape vivo do R-1.

### 1.3 População (b) — implementações do par *predicado + recusa audível*

| forma | onde | n |
|---|---|---|
| helper nomeado, corpo byte-idêntico | `scaffold.go:86` `rejectScaffoldPath` · `commands/discover.go:216` `rejectDiscoverPath` | 2 |
| variante — 4 params, `(TargetResult, bool)` | `update.go:665` `rejectHarnessSymlink` | 1 |
| variante — delegate de 1 linha, **sem stderr** | `manager.go:762` `rejectSymlinks` | 1 |
| **par escrito inline** no sítio | 49 ocorrências `RejectSymlinks` + `Fprintf(os.Stderr,…)` ≤2 linhas | **49** |
| **par ausente** — guarda muda | `roadmap.go:829`, `req.go:474`, `manager.go:762` | 3 |

**Total pela régua do mecanismo: 53 implementações do par, em 5 gramáticas de mensagem.**

### 1.4 A enumeração está fechada? — sim, com um limite nomeado

O universo é fechado porque `RejectSymlinks` é o **único** exportado do predicado
(`pathguard.go:84`) e as 52 chamadas não-teste foram rastreadas uma a uma até o resolvedor.

🔴 **O que fecha é o universo de quem CHAMA a guarda. Não fecha o universo de quem ESCREVE sem
chamá-la** — essa é a população do #400, e nenhum grep a delimita: é precisamente o que exige AST.

---

## 2. O ponto único, e por que ele PRECEDE o analisador

### 2.1 Desenho

```
package pathguard

// RejectAndReport é o ÚNICO sítio do binário que emite a recusa de contenção.
// root DEVE vir de um resolvedor aprovado; o parâmetro é nomeado para ser
// reconhecível em AST.
func RejectAndReport(resolvedRoot, absTarget string) error

// ResolveRoot é o ÚNICO produtor aprovado de root. Falha fecha (não faz fallback
// silencioso para o caminho não resolvido — ver §3.5).
func ResolveRoot(dir string) (string, error)
```

Contrato:

1. **Uma** gramática de mensagem — `trackfw: refusing write to %s: %v` — idêntica **por
   construção**, não por coincidência textual (AC5).
2. **Zero** sítio silencioso: as 3 guardas mudas (§R-4) passam a falar (AC4).
3. As variantes de assinatura (`rejectHarnessSymlink`, `rejectSymlinks`) viram **adaptadores finos
   sobre** `RejectAndReport`, não reimplementações.
4. `ResolveRoot` **falha fechada**. Hoje `projectRoot()` (`scaffold.go:26-29`) e
   `resolveRoot()` (`discover.go:28-32`) fazem *fallback para o caminho não resolvido* quando
   `EvalSymlinks` erra — é o que transforma um erro transitório em degradação silenciosa.

### 2.2 Por que é pré-requisito, não gosto

| | nós que o analisador precisa modelar | direção do erro se for feito antes |
|---|---|---|
| hoje | **53** (4 helpers + 49 pares inline), **5** gramáticas, **3** formas de retorno | cada forma não modelada = **falso negativo silencioso** |
| pós ML-7A | **1** | — |

E o argumento é **aritmética, não amostra**: o analisador tem de provar *"todo sítio de escrita é
precedido pela guarda no mesmo fluxo"*. Com 53 formas, cada uma é um padrão a reconhecer e cada
padrão esquecido **aprova** — o analisador erra na direção insegura por omissão. Escrever o AST
sobre 53 formas é trabalho que se joga fora **e** que nasce com furos.

🔴 **Sequenciamento obrigatório:** ML-7A → ML-7B. Inverter é o caminho mais barato para um gate
verde que não prova nada.

---

## 3. Threat model DO INSTRUMENTO

> O adversário aqui é **o implementador apressado e o arquiteto otimista**, não um atacante externo.
> A pergunta: *quem faz o analisador nascer verde por vacuidade, **sem quebrar nenhuma regra
> escrita**?*

### 3.1 🔴 T1 — O corpus de falsificação não existe em lugar nenhum a que o CI chegue

**É a ameaça nº 1 e invalida o AC da Wave 7 como está escrito.**

O AC diz: *"reprova os 22 gaps e os 7 marcadores falsos reconstruídos do estado pré-Wave-4 por
`git show` ou overlay"*. Medi onde esse estado vive.

**Primeiro, o handoff/roadmap aponta o commit errado.** `7721efc6^1` (`f91ca652`) é o estado
**pré-REQ**: zero marcadores, zero guardas — não pode exibir "22 gaps + 7 marcadores falsos".
Varri os 32 commits do PR #397 pelos dois discriminantes do próprio #400:

```
$ for c in $(git rev-list --reverse 7721efc6^1..435da4e9); do ... done
...
87fe4915 scaffold_markers=33 ... lefthookPath=3 rejectScaffoldPath_in_fn=2
a8912bd6 scaffold_markers=33 ... lefthookPath=4 rejectScaffoldPath_in_fn=8
```

`87fe4915` exibe o caso exato do #400 — `lefthook.yml` escrito na raiz com o marcador, e as duas
únicas guardas do bloco cobrindo `.husky` e `.lefthook/commit-msg`:

```
13:  if err := rejectScaffoldPath(cmhRoot, filepath.Join(cmhRoot, ".husky")); err != nil {
17:  if err := rejectScaffoldPath(cmhRoot, filepath.Join(cmhRoot, ".lefthook", "commit-msg")); err != nil {
48:  lefthookPath := "lefthook.yml"
52:  // write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
53:  if err := os.WriteFile(lefthookPath, append(existing, []byte(addition)...), 0644); err != nil {
```

```
$ git log -1 --format='%h %ad %s' 87fe4915
87fe4915 Mon Sep 21 14:29:33 2026 docs(governance): Wave 3 — corretivos da barreira final (hades + hefesto)
$ git log -1 --format='%h %ad %s' a8912bd6
a8912bd6 Mon Sep 21 14:54:27 2026 fix(scaffold,gates): gap de folha em 13 sitios e um marcador que era falso (Wave 3)
```

🔴 **O commit-corpus é `87fe4915`. O commit-fix é `a8912bd6`.**

**Verificado para as QUATRO classes do #400, não só para `scaffold.go`** — um pin que só cobrisse
uma classe entregaria corpus incompleto e piso errado. Contagem de guardas por arquivo,
`87fe4915` → `435da4e9` (ponta da branch): um delta ≠ 0 prova que a correção daquela classe é
**posterior** ao pin, logo o defeito está **presente** no corpus.

| classe do #400 | arquivos | delta de guardas | corpus |
|---|---|---|---|
| `scaffold.go` — 13 gaps + 1 falso | `scaffold.go` | **+16** | íntegro |
| `update.go` — 0 gaps + 5 falsos | `update.go` | **−5** (marcadores falsos removidos) | íntegro |
| família — 5 gaps + 1 falso | `roadmap.go` +2, `req.go` +1, `note.go` +1, `adr.go` +2 = **+6** | íntegro |
| `discover` — 4 gaps + 0 | `discover/discover.go` | **+4** | íntegro |

Os deltas batem com a tabela do #400 (13+1 / 0+5 / 5+1 / 4+0). E o total de marcadores é **157** em
`87fe4915` — o mesmo número que o #400 diz ter auditado linha a linha.

```
$ for c in 87fe4915 a8912bd6 435da4e9; do ... total_markers ... done
87fe4915 total_markers=157
a8912bd6 total_markers=157      (a correção mexeu nas guardas, não na contagem de marcadores)
435da4e9 total_markers=158
```

`agentfiles.go`, `java.go` e `commands/discover.go` têm delta **0** — não contribuem com defeito ao
corpus, e o piso da §3.2 deve refletir isso em vez de presumir distribuição uniforme.

**Segundo — e é o achado grave — esse commit não é alcançável fora desta máquina:**

```
$ git for-each-ref --contains 435da4e9 --format='%(refname)'
refs/heads/fix/afirma-contencao-antes-de-escrever      ← só isto

$ git ls-remote --heads origin 'fix/afirma*'
                                                        ← vazio: apagada no remoto
```

O #397 foi **squash-merge**: os 32 commits **não** são alcançáveis a partir da `main`. Sobrevivem
num **único ref local, na máquina do KG**. Duas consequências:

1. **O CI nunca pode rodar esse braço.** Um clone novo de `main` não tem o objeto; `git show
   87fe4915:...` resolve para nada. Se o gate degradar para *skip*, o skip é **indistinguível de
   verde** — a passagem vacuosa que este ML existe para impedir.
2. **A validade do corpus expira numa única execução de prune.** `git branch -vv` mostra a branch
   como `[origin/…: gone]`, e tanto o protocolo do `CLAUDE.md` quanto `trackfw branch prune --apply`
   classificam `gone` como *seguro apagar*. Em 2026-09-12 este projeto **já apagou** uma branch com
   trabalho não integrado por esse caminho.

**Mudança de AC exigida:**

> 🔴 O corpus de falsificação é **extraído para `testdata/` versionado como saída da Wave 6**
> (pré-condição bloqueante da Wave 7), a partir de `87fe4915`, com os dois casos nomeados do #400
> (`generateCommitMsgHook` ramo lefthook; `syncREQReferences`). **Nenhum braço do gate referencia
> commit-ish.** Um gate cuja evidência mora fora da árvore versionada não é gate.

### 3.2 T2 — Vacuidade por população zero

Os 34 defeitos já estão corrigidos: o analisador roda sobre a árvore de hoje e fica verde **por não
haver nada a achar**, indistinguível de um analisador que funciona.

**Guarda de vacuidade — os três modos de obsolescência, os três reprovam (nunca *skip*):**

| modo | condição | veredito |
|---|---|---|
| (a) corpus ausente | `testdata/` do corpus não existe ou não carrega | **FAIL**, nunca skip |
| (b) população zero | sítios examinados == 0 | **FAIL** |
| (c) população divergiu | contagem ≠ piso fixado (`20` expressões / `34` escritas / `1` par) | **FAIL**, com o delta impresso |

🔴 **Piso fixado por contagem, nunca por data.** Expiry de calendário aprova em silêncio no dia
seguinte ao merge.

### 3.3 🔴 T3 — A lista de exceções nomeadas dissolve a regra (o caminho mais provável)

**É o caminho mais plausível de "esvaziar a Wave 7 sem quebrar nenhuma regra escrita".** Não é
malicioso — é o atalho natural.

`adr.go` 53/124/255/314 usa `guardRoot = filepath.Abs(adrDir)` **deliberadamente** no escopo global
(ADR, decisão 3, residual nomeado). O analisador de provenância **vai apontá-lo**. O implementador
apressado então:

1. relaxa a regra "root vem de resolvedor aprovado" para aceitar `filepath.Abs`;
2. ou adiciona uma exceção *por arquivo* (`internal/generators/adr.go`).

Qualquer uma das duas torna a regra **tautológica** — e reabre o R-1 **com o gate verde**, que é
literalmente o modo de falha que originou esta reabertura.

**Contramedida:**

- exceções são **por sítio** (arquivo + símbolo + razão), **nunca por arquivo ou por padrão**;
- a lista tem **contagem fixada** e os 3 modos de obsolescência da §3.2 valem para ela;
- 🔴 **o braço do corpus (§3.1) reprova independentemente da lista de exceções** — se uma exceção
  puder silenciar o corpus, a lista virou a porta dos fundos;
- e o R-1 diz que `adr.go` **não deveria estar na lista**: é escape vivo, não residual. A exceção
  correta é *nenhuma* — ver §3.3-bis para qual é a correção, porque a óbvia está errada.

### 3.3-bis 🔴 A correção ÓBVIA do R-1 não fecha o R-1 — e o próprio código já avisa

Autorrefutação, registrada para não ser reintroduzida por quem ler só o R-1.

A correção que salta aos olhos é *"faça `absAdrDir` resolver com `EvalSymlinks`"*. **Ela mantém o
escape.** Traçando a arm B com ela aplicada:

```
absAdrDir = /tmp/p/docs/adr  --EvalSymlinks-->  /private/tmp/victim/adr   (docs É o symlink)
Beneath(/private/tmp/p, /private/tmp/victim/adr)            -> false      (continua falso)
guardRoot = /private/tmp/victim/adr                          -> RejectSymlinks não acha nada
                                                             -> ESCREVE NA VÍTIMA
```

E `adr.go:42-45` já diz isso, em inglês, no próprio fonte:

> *"Guard adrDir **BEFORE** EvalSymlinks: if absAdrDir itself is a symlink (or has a symlink
> ancestor), EvalSymlinks would resolve it to the target outside the tree, making
> `Beneath(pr, resolved)` false and defeating the containment check."*

**O defeito não é a ausência de resolução do alvo. É o fallback permissivo** — exatamente como a
nota de vault conclui: *quando um descasamento de caminho escolhe **qual controle usar**, o ramo de
fallback tem de ser o **mais estrito***.

**O discriminante correto está no comentário de `adr.go:32-37`:**

> *"**project scope (adrDir relative** or beneath cwd): use `projectRoot()` … global scope (adrDir
> **absolute**, outside cwd): use absAdrDir as root"*

Aqui `adrDir` é **`docs/adr` — relativo**. Pelo contrato escrito ele **nunca deveria** alcançar o
ramo global; alcança porque o único teste de escopo é `Beneath`, que o descasamento de namespace
derruba. A forma proposta, para a ML avaliar e medir:

1. **`adrDir` relativo ⇒ escopo de projeto, incondicionalmente** — sem consultar `Beneath`. O ramo
   global fica alcançável **só** para `adrDir` absoluto, que é o contrato declarado.
2. **`absAdrDir` deriva de `filepath.Join(projectRoot(), adrDir)`** no ramo relativo — e **não** de
   `filepath.Abs`, que herda o `cwd` lógico. Assim root e alvo nascem no **mesmo namespace
   resolvido**, `RejectSymlinks` caminha `pr → docs` e **recusa pelo symlink**, que é o
   comportamento da arm A.
3. 🔴 **Nenhum `EvalSymlinks` sobre o alvo antes da guarda** — a ordem guarda-antes-de-resolver do
   ADR fica preservada.

Isto fecha as duas arms sem relitigar a decisão 3 do ADR e sem tocar no residual do escopo global.
**A medição das duas arms do R-1 é o critério de aceite da ML**, não o raciocínio acima.

### 3.4 T4 — O analisador certo aprovando o defeito (alcançabilidade ≠ contenção)

Concordo integralmente com o ponto que a Wave 8 levanta. Um analisador de **alcançabilidade** —
*"o caminho escrito passou por `pathguard` neste fluxo?"* — **aprova**
`RejectSymlinks(filepath.Clean(cwd), path)`, porque o fluxo passa. O defeito é o **argumento**.

**Como se expressa em AST/SSA** — provenância (taint) sobre o **1º operando**:

- **fato propagado:** uma função é *produtora de root resolvido* se todo `return` do caminho de
  sucesso deriva de `filepath.EvalSymlinks` — marca `projectRoot`, `scaffoldRoot`,
  `discover.resolveRoot`, `pathguard.ResolveRoot`;
- **checagem no sítio:** a cadeia de definições SSA do 1º operando de `RejectAndReport` termina numa
  produtora marcada;
- **reprova** quando a cadeia termina em `os.Getwd()`, `homedir.Dir()`, `filepath.Abs`,
  `filepath.Clean` ou parâmetro não anotado — **e `filepath.Clean` não é transparente**: envolver
  não sanitiza (é o `Clean(cwd)` inteiro);
- **interprocedural:** o parâmetro `home` de `rejectHarnessSymlink` (a14, 15 escritas) só se resolve
  olhando os chamadores. Um analisador intraprocedural **perde a14, a15–a16** — 17 das 34 escritas.
  Isso é metade do universo, e é por isso que o ponto único (§2) precede.

🔴 **Então o gate tem DOIS predicados, não um:** (P1) todo sítio de escrita é precedido pela guarda
no mesmo fluxo (#400); (P2) o root da guarda tem provenância resolvida (#402 / R-1). Um analisador
que só faz P1 é o instrumento que aprova o escape do R-1 — verde, e errado.

### 3.5 T5 — O fallback silencioso do resolvedor

`projectRoot()` devolve o `cwd` **não resolvido** quando `EvalSymlinks` falha (`scaffold.go:26-29`),
e `resolveRoot()` idem (`discover.go:28-32`). Um analisador de provenância marca essas funções como
*produtoras aprovadas* e **nunca vê** que elas emitem um root não resolvido em runtime.

**Isto é uma verdade que o AST não alcança** — a provenância é estática, a degradação é dinâmica.
Contramedida: `ResolveRoot` **falha fechada** (§2.1 item 4), e um teste em runtime afirma que a
falha de resolução **recusa a escrita** em vez de escrever com root fraco.

### 3.6 Resumo dos braços

| ameaça | braço que DISCRIMINA "funciona" de "não há o que achar" |
|---|---|
| T1 corpus inalcançável | corpus em `testdata/` versionado (de `87fe4915`); ausência = **FAIL**; zero referência a commit-ish |
| T2 população zero | contagem examinada ≥ piso fixado; 0 = FAIL; divergência = FAIL com delta |
| T3 exceções dissolvem a regra | exceção por sítio + contagem fixada + corpus reprova **independentemente** da lista |
| T4 alcançabilidade ≠ contenção | P2 (provenância do 1º operando) com arm negativa: `RejectAndReport(filepath.Clean(cwd), p)` no corpus **deve** reprovar |
| T5 fallback silencioso | `ResolveRoot` falha fechada + teste de runtime, não de AST |

---

## 4. Falsificação nas duas direções, e residual

### 4.1 As duas direções, por população

**População (a) — root não resolvido**

| direção | o que quebra | braço |
|---|---|---|
| **falso negativo** (produto regride) | um sítio novo volta a passar `Clean(cwd)`/`homedir.Dir()` | P2 reprova o sítio; corpus contém `RejectAndReport(filepath.Clean(cwd), p)` e **exige** reprovação |
| **falso positivo** (guarda larga demais) | root resolvido × alvo não resolvido recusa escrita legítima — armadilha 3 da Decisão 2 | `trackfw adr new` / `req new` / `roadmap new` num projeto sob `/tmp` com `PWD` lógico **devem terminar rc=0 e escrever DENTRO** |

🔴 **O braço do falso positivo é onde o R-1 mora, e o produto hoje o "resolve" da pior maneira: em
vez de recusar (FP) ou aceitar corretamente, ele degrada o escopo e escreve fora.** Logo o teste da
direção 2 tem de afirmar **três** coisas, não duas: (i) rc=0; (ii) o arquivo está **dentro**;
(iii) **e o escopo de guarda usado continuou sendo o de projeto** — senão o teste passa com o
controle desligado. É a forma exata do "marcador que afirma sem provar", uma camada acima.

**População (b) — o par predicado+recusa**

| direção | o que quebra | braço |
|---|---|---|
| **falso negativo** | 5ª gramática de mensagem reaparece, ou guarda muda | `Fprintf(os.Stderr, …refus…)` **fora** de `pathguard` == 0 (mecânico, sem lista de nomes) |
| **falso positivo** | o ponto único recusa escrita legítima em massa | suíte de smoke dos 34 sítios: `init`, `discover --init`, `update`, `adr/req/roadmap/note new` rc=0 numa árvore limpa |

### 4.2 Residual declarado

1. **TOCTOU** entre guarda e escrita — já declarado na Wave 0 original. CLI local mono-usuário.
   Inalterado.
2. **Junctions do Windows** — REQ irmã, causa diferente (detecção de forma). Nenhum braço aqui.
3. **`Beneath` sensível a casing em APFS** — produz falso-rejeição, nunca escrita fora. Mantido.
4. 🔴 **P1 (o analisador do #400) não prova ausência de sítio de escrita sem guarda alguma.** Ele
   prova que os primitivos de escrita *que ele modela* estão precedidos. Um primitivo novo
   (`os.OpenFile`, `os.Symlink`, `io/fs` novo, um `exec` que escreve) sai do modelo. **Mitigação:
   lista de primitivos com contagem fixada**, não confiança na completude.
5. 🔴 **O universo enumerado em §1 fecha sobre quem CHAMA a guarda, não sobre quem escreve.** Isto é
   assumido explicitamente; é a razão de o AST existir.
6. **A provenância é estática** — T5 não é coberto por AST em hipótese nenhuma. Coberto por
   fail-closed + teste de runtime.
7. **Exceções nomeadas continuam sendo confiança.** Contagem fixada limita o crescimento; não impede
   que uma exceção individual esteja errada.

---

## 5. O que muda nos ACs das Waves 7 e 8

| AC como está | por que não serve | forma medida |
|---|---|---|
| W7: *"reconstruídos do estado pré-fix por `git show`"* | objeto só num ref local; CI nunca alcança; um prune destrói (§3.1) | corpus em `testdata/` versionado, extraído de **`87fe4915`**, como saída bloqueante da Wave 6 |
| W7: *"os 4 sítios passam a delegar"* | população por identificador; deixa 49 pares inline e 5 gramáticas (R-3, R-4) | **1** sítio emissor de recusa no binário; `Fprintf(…refus…)` fora de `pathguard` == 0 |
| W8: *"os **13** sítios"* | régua de identificador (R-2) | **20** expressões / **34** escritas, por provenância do 1º operando, com piso fixado |
| W8: *"a armadilha 3 é falsificada por teste"* | o teste como descrito passa com o controle degradado | as 3 afirmações da §4.1, incluindo *"o escopo de guarda continuou o de projeto"* |
| — (não existe) | o escape vivo do R-1 não tem AC | **ML novo nesta REQ**: escopo de projeto incondicional para `adrDir` relativo + `Join(projectRoot(), adrDir)` (§3.3-bis — 🔴 **não** "resolver `absAdrDir`", que mantém o escape); as duas arms do R-1 viram teste |

---

## 6. Reconciliação — o que cada medição afirma

| medição | conclusão deste ML que ela afirma |
|---|---|
| duas arms de `adr new` (R-1) | o defeito do título da REQ está vivo; a armadilha 3 foi "resolvida" degradando o controle |
| tabela de 4 comandos (R-1) | o escape é de `adr new` apenas — a fronteira é medida, não presumida |
| `13` vs `52` chamadas (R-2) | o 13 é régua de identificador; a régua do mecanismo dá 20/34 |
| `diff a b → IDENTICAL` (R-3) | 2 cópias exatas; o "3 byte-idênticas" do #401 é falso |
| `49` pares inline / `5` gramáticas / `3` mudos (R-3, R-4) | AC5 e AC4 estão não atendidos; a REQ foi para `done` com eles abertos |
| varredura dos 32 commits → `87fe4915` | o commit-corpus do AC da W7 estava errado; este é o certo |
| `for-each-ref` + `ls-remote` vazio (§3.1) | o corpus é inalcançável pelo CI e expira num prune → o AC muda |
| deltas de guarda por classe em `87fe4915` (§3.1) | o pin cobre as **4** classes do #400, não só `scaffold.go`; e 3 arquivos têm delta 0, logo o piso não é uniforme |
| traço da arm B com `EvalSymlinks` aplicado (§3.3-bis) | a correção óbvia do R-1 **mantém** o escape; o defeito é o fallback permissivo, não a resolução ausente |
| `adr.go:355-356` no ramo de falha da guarda (R-3) | o `aviso:` do discover **é** recusa de contenção → as 5 gramáticas se sustentam |
| `Getwd:/tmp` vs `Eval:/private/tmp` (R-1) | a divergência de namespace é real nesta máquina, não teórica |
