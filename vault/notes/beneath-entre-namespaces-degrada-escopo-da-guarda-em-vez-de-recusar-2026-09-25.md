# `Beneath` entre namespaces diferentes DEGRADA o escopo da guarda em vez de recusar

> 2026-09-25 · REQ-2026-08-31 (contenção de escrita) · ML-6A, reabertura · issues #400/#401/#402

## O sintoma

`trackfw adr new` escreve **fora do projeto** através de um `docs` symlinkado, `rc=0`, sem nenhuma
mensagem. `trackfw req new` e `trackfw roadmap new`, no mesmo projeto, com a mesma isca, **recusam
corretamente**. É o defeito do título da própria REQ, vivo depois de ela ter ido para `done`.

## Reprodução (2 arms, macOS, contra `bin/trackfw`)

```bash
D=/tmp/p; OUT=/tmp/victim
mkdir -p "$OUT/adr" "$D"
printf 'project: p\nadr_dir: docs/adr\nreq_dir: docs/req\nroadmap_dir: docs/roadmaps\n' > "$D/trackfw.yaml"
ln -s "$OUT" "$D/docs"            # 'docs' aponta para FORA

# arm A — PWD desfeito: os.Getwd() devolve o caminho físico
(cd "$D" && unset PWD; trackfw adr new 'x')
# trackfw: refusing write to /private/tmp/p/docs/adr: refusing symlink path "/private/tmp/p/docs"
# rc=1 — CORRETO

# arm B — cd normal de login shell (PWD=/tmp/p)
bash -lc "cd $D && trackfw adr new 'x'"
# created docs/adr/ADR-....md
# rc=0 — ESCAPOU: o arquivo está em /tmp/victim/adr
```

## A causa raiz — e o passo não óbvio é o terceiro

### 1. `os.Getwd()` do Go honra `$PWD`

Se `$PWD` `stat`-bate com `.`, o Go devolve **o caminho lógico**, não o físico. Um `cd` de shell
comum já produz isso. Medido:

```
Getwd: /tmp/hades-adr-probe2-70636
Eval : /private/tmp/hades-adr-probe2-70636
```

Por isso o problema **não** aparece num `cd` feito dentro de um script que desfaz `PWD`, e aparece
para o usuário. Um teste que rode com `PWD` físico **não reproduz**.

### 2. Um lado resolvido, o outro não

`internal/generators/adr.go:47-50`:

```go
guardRoot := absAdrDir                        // filepath.Abs — NÃO resolvido
if pr, prErr := projectRoot(); prErr == nil { // EvalSymlinks — resolvido
    if pathguard.Beneath(pr, absAdrDir) {
        guardRoot = pr                        // escopo de projeto
    }
}
```

`pr = /private/tmp/p`, `absAdrDir = /tmp/p/docs/adr`. `Beneath` compara **namespaces diferentes** e
dá `false`.

### 3. 🔴 O `false` não vira recusa — vira **rebaixamento silencioso de escopo**

É isto que torna o defeito difícil de ver, e é a diferença desta nota para
`resolve-symlinks-primitivas-divergem-nos-3-runtimes-folha-inexistente-2026-08-31`, que descreve o
mesmo descasamento de namespace mas na direção do **falso positivo**.

Aqui o `else` implícito é o **escopo global**, que por desenho só inspeciona symlinks *dentro* de
`adrDir` e **nunca olha o ancestral `docs`** (residual nomeado na decisão 3 do ADR). Ou seja: o
descasamento de namespace **não recusa e não avisa — ele troca o controle forte pelo fraco.**

> Regra geral: quando um descasamento de caminho seleciona **qual controle usar**, o ramo de
> fallback precisa ser o **mais estrito**, nunca o mais permissivo. Fallback permissivo transforma
> um falso positivo (barulhento, alguém conserta) num escape (mudo, ninguém vê).

## Fronteira medida — só `adr` escapa

| comando | rc | arquivos fora do projeto |
|---|---|---|
| `adr new` | 0 | **1** |
| `req new` | 1 | 0 |
| `roadmap new` | 1 | 0 |
| `note new` | 0 | 0 (escreve em `vault/notes`, fora da isca) |

`req`/`roadmap` derivam root **e** alvo de `projectRoot()` — mesmo namespace, `Beneath` acerta.

## Consequência para instrumentos

Um analisador de AST de **alcançabilidade** (*"a escrita passou por `pathguard`?"*) **aprova** este
sítio: o fluxo passa. O defeito é o **argumento**. Qualquer gate desta classe precisa de um segundo
predicado — **provenância do 1º operando** — ou não pega a reintrodução.

E o teste da direção "não recusa operação legítima" tem de afirmar **três** coisas: rc=0, arquivo
**dentro** da árvore, **e que o escopo de guarda usado continuou sendo o de projeto**. Sem a
terceira, o teste passa com o controle desligado.

## Correção (ML-7A, 2026-09-25) — e a medição que endurece a autorrefutação

**Forma aplicada** (`adrGuardPaths`, ponto único de decisão de escopo em `internal/generators/adr.go`):

- `adrDir` **relativo** ⇒ escopo de projeto **incondicional**: `Beneath` não é consultado,
  `root = projectRoot()` e `alvo = filepath.Join(projectRoot(), adrDir)`. Os dois nascem no mesmo
  namespace resolvido, então `RejectSymlinks` caminha `root → docs` e recusa pelo symlink.
  **Falha fechada** se `projectRoot()` erra — sem fallback para `filepath.Abs`.
- `adrDir` **absoluto** ⇒ escopo global (`root = adrDir`), o residual nomeado da decisão 3.
  🔴 `Beneath` sobrevive **só aqui**, e só numa posição onde pode **AUMENTAR** a estritura: caminho
  absoluto genuinamente dentro do projeto é **promovido** a escopo de projeto. Nunca rebaixa.
  Trocar isso por um `filepath.IsAbs` puro seria um **enfraquecimento** — hoje um `adr_dirs[0]`
  absoluto e interno pega escopo de projeto, e perderia a checagem dos ancestrais.

### 🔴 A correção ÓBVIA é PIOR que o defeito — medido, não deduzido

A §3.3-bis do relatório do ML-6A já dizia que `EvalSymlinks(absAdrDir)` **mantém** o escape. A
medição por mutação do ML-7A mostra mais que isso: ela **abre a arm que hoje recusa**.

| mutante | arm `$PWD` resolvido | arm `$PWD` não resolvido |
|---|---|---|
| A — forma pré-fix (`main` de hoje) | recusa (correto) | **escreve na vítima, rc=0** |
| B — a correção "óbvia" (`EvalSymlinks` no alvo) | **escreve na vítima, rc=0** | **escreve na vítima, rc=0** |
| ML-7A | recusa | recusa |

⚠️ **E a falsificação do mutante B depende de um detalhe de fixture:** `EvalSymlinks` exige que o
caminho **exista**. Se `<vítima>/adr` ainda não existe, a resolução **falha**, o código mantém o
caminho não resolvido e o mutante B degenera no mutante A — falsificado, mas **pela razão errada**,
e a arm resolvida passa dando a impressão de que a correção óbvia "quase funciona". A fixture
**pré-cria `<vítima>/adr`** (o estado que qualquer escape anterior já deixou) para exercitar o traço
da §3.3-bis de verdade.

### Como o teste prova, e o que ele exige para não ser vácuo

- **Duas arms deterministas em qualquer plataforma:** em vez de depender de `/tmp` ser symlink,
  a fixture cria `alias -> proj` e faz `chdir(alias)` + `PWD=alias`. `os.Getwd()` do Go devolve
  `$PWD` quando ele `stat`-bate com `.`; `projectRoot()` resolve. Divergência garantida.
- 🔴 **No Windows `os.Getwd()` ignora `$PWD`** (devolve `syscall.Getwd()` direto): as duas arms
  colapsariam numa só e a suíte ficaria verde tendo medido metade. A arm não resolvida **assere a
  pré-condição** (`Getwd != projectRoot()`) e **`t.Skip` nomeando o que não foi exercitado**.
- **Braço de controle no mesmo teste:** `NewREQ` continua recusando — sem ele, "tudo recusa porque
  quebrei tudo" passaria como conserto.
- **A asserção é `refusing symlink path` + vítima vazia**, não `err != nil`: erro de config ou de
  ambiguidade de agente satisfaria `err != nil` sem provar contenção nenhuma.

## Ver também

- `docs/seguranca/2026-09-25-ponto-unico-de-contencao-e-o-instrumento-que-o-prova.md` (ML-6A)
- `vault/notes/resolve-symlinks-primitivas-divergem-nos-3-runtimes-folha-inexistente-2026-08-31.md`
- `internal/generators/adr_scope_guard_test.go` (ML-7A — as duas arms, o controle e a falsificação)
- `vault/notes/marcador-de-contencao-pode-ser-falso-guarda-de-dir-nao-cobre-a-folha-2026-09-21.md`
