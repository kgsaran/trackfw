# Barreira final de segurança — `fix/resolvedor-de-req-cobre-o-layout-canonico-e-ciclo-fechado-por-artefato`

> Revisor: `hades-tf` (Security). Diff auditado: `git diff origin/main...HEAD`
> (ML-1A `f7963c7` — resolvedor de REQ em união dos 4 layouts + escrita no canônico, 3 CLIs;
> ML-2A `31bcdef` — gate de ciclo fechado por artefato).
> Superfície de código: `internal/validator/validator.go`, `internal/generators/req.go`,
> `internal/generators/context.go`, `internal/validator/validator_traceid.go`,
> `npm/src/validator/index.js`, `npm/src/generators/req.js`, `npm/src/commands/context.js`,
> `pypi/trackfw/validator.py`, `pypi/trackfw/generators/req.py`, `pypi/trackfw/commands/req.py`,
> `pypi/trackfw/traceid.py`, `pypi/trackfw/commands/context.py`,
> `scripts/check-artifact-closed-cycle.sh`.
> Verificação **por execução** nos 3 runtimes (binário Go compilado da branch, `npm/bin/trackfw`,
> `python3 -m trackfw`), com braço de comparação contra `origin/main` extraído por `git archive`.
> Vault lido antes de investigar: `index.md`,
> `resolvedor-de-req-era-if-else-e-a-uniao-colide-com-o-namespace-vindo-do-disco-2026-09-03`,
> `lstat-nao-ve-junction-e-guarda-de-folha-nao-olha-ancestral-2026-08-31`,
> `update-segue-symlink-e-escreve-fora-do-projeto-2026-08-28`,
> `uniao-disco-agents-mascara-gate-por-presenca-2026-08-29`,
> `serve-validator-index-detectado-como-binario-grep-silencioso-2026-08-29` (todo `grep` em
> `npm/src/validator/index.js` neste parecer foi feito com `-a`).
> `npm/src/validator/index.js` é classificado como **dado binário** pelo `file` — `grep` sem `-a` o
> pula em silêncio; duas REQs deste repo já tiveram premissa falsa por isso.

## Veredito: **APROVA COM RESSALVAS**

**Um achado é escrita fora do projeto e é NOVO neste diff** (§1): com `roadmap_namespacing:
by_agent`, um **symlink de diretório plantado em `req_dir/default`** (ou `req_dir/<primeiro
agente>`) faz `trackfw req new` **criar e sobrescrever** arquivos fora da árvore do projeto, nos 3
CLIs. Medido nas duas direções: em `origin/main` a mesma fixture grava **dentro** de `docs/req/`; na
branch, **fora**. Ele **não bloqueia o merge**, e a razão é dura, não indulgente: o mesmo furo, com
o mesmo mecanismo e sem nenhuma guarda, **já existe em `origin/main` no `roadmap new`**
(`roadmap_dir/default` → escrita fora, medido). ML-1A não introduz uma classe nova — estende ao
escritor de REQ um padrão de escrita sem guarda de link que o projeto já carrega. Bloquear este PR
não fecharia o furo; o remédio correto é **uma REQ própria com guarda compartilhada nos 3
escritores** (req/roadmap/note), e ela é obrigatória.

**Um achado de correção é novo e quebra a AC numérica do próprio roadmap** (§4): em filesystem
case-insensitive (APFS/macOS, NTFS/Windows), um diretório `req_dir/Backlog` é enumerado **duas
vezes** e cada violação sai **em duplicata** nos 3 CLIs — a dedup por caminho normalizado é lexical
e não vê a colisão. Ressalva, não bloqueio.

**Uma divergência de paridade foi introduzida em código novo** (§5): `agents: ["", "zeus"]` manda o
Go gravar em `req_dir/default/` e Node/Python em `req_dir/zeus/`. Não é escalada — é a regra dura de
paridade violada dentro da função que este PR criou, com remédio de uma linha. Recomendo corrigir
antes do merge por ser barato e estar no mesmo arquivo; não bloqueio formalmente porque nenhum dos
dois destinos sai do projeto e ambos estão na união de leitura.

**Cinco vetores que eu ataquei e vieram FECHADOS** estão em §6, com o mecanismo que os fecha — não
inflo severidade sobre eles. Entre eles, o gate do ML-2A: **não é vácuo**, e eu reproduzi as três
sabotagens dos cenários 183/184/185 por conta própria em vez de aceitar o `OK` do script (§6.5).

---

## 0. Aparato de medição

Binário Go compilado da branch (`go build ./cmd/trackfw`); CLI Node por `node npm/bin/trackfw`; CLI
Python por `PYTHONPATH=pypi python3 -m trackfw`. Braço de comparação: `git archive origin/main npm`
e `git archive origin/main pypi` extraídos para fora da árvore, executados sobre a **mesma fixture**
— sem isso não há como separar "furo novo" de "furo herdado", que é exatamente a distinção que muda
o veredito aqui.

Projetos-fixture descartáveis fora do repositório, um por vetor. O oráculo primário é o **efeito no
filesystem** (`find`/`ls` no destino), não a linha `created …` impressa: o Go e o Node imprimem
caminho **relativo**, e um `docs/req/../../../..` impresso não diz sozinho onde o arquivo caiu.

---

## 1. 🔴 S2 — `req new` escreve através de symlink plantado em `req_dir/<agente>` (3 CLIs) — **NOVO neste diff**

**Arquivo:linha**
- `internal/validator/validator.go:1384-1397` (`REQWriteDir`) → `internal/generators/req.go:31,35`
  (`os.MkdirAll(reqDir, 0755)` + escrita)
- `npm/src/validator/index.js:370` (`reqWriteDir`) → `npm/src/generators/req.js:246-247`
- `pypi/trackfw/validator.py:611` (`req_write_dir`) → `pypi/trackfw/commands/req.py:80` →
  `pypi/trackfw/generators/req.py:56` (`os.makedirs(req_dir, exist_ok=True)`)

**Severidade: média.** Escrita/sobrescrita arbitrária de arquivo fora do projeto, **sem execução**.
O basename é forçado a `REQ-<data>-<slug>.md` — e eu **medi** isso em vez de presumir, porque o
índice do vault registra como resíduo em aberto que "`NewREQ`/`NewNote` interpolam título sem
guarda" (`roadmap-title-newline-forges-wave-section-barrier-executes-gate-2026-08-23`). O `slug` do
título **não** carrega travessia nos 3 runtimes:

```
$ trackfw req new "../../../../pwn"
GO   created docs/req/REQ-2026-09-03-pwn.md
NODE created docs/req/REQ-2026-09-03-pwn.md
PY   created …/docs/req/REQ-2026-09-03-pwn.md
$ trackfw req new "a/b/c"
GO/NODE/PY -> docs/req/REQ-2026-09-03-a-b-c.md
$ find <sandbox> -name "*.md"
proj/docs/req/REQ-2026-09-03-pwn.md
proj/docs/req/REQ-2026-09-03-a-b-c.md          # nada fora do projeto, nos 3
```

`.` e `/` colapsam no slug; o componente de **diretório** é a única variável do caminho. Sem
symlink plantado, `req new` sozinho não escreve fora, e não há caminho para `.git/hooks/*` nem para
qualquer outro arquivo executado — é por isso que o teto do dano é integridade de arquivo, não
execução.

**Precondição:** plantar um symlink **dentro de `req_dir`**, que já pressupõe quem está dentro. O qualificador
honesto que impede rebaixar isso a nada: **symlink é objeto versionável pelo git**, então
`docs/req/default -> ../../../../..` viaja num checkout/PR/template — a forma é de cadeia de
suprimentos, não de escalada de privilégio.

**Mecanismo.** ML-1A move o destino de escrita de `cfg.REQDir` (folha controlada pela config) para
`filepath.Join(reqDir, agent)` — um componente **novo** de caminho que é lido do disco no momento da
escrita. `MkdirAll`/`mkdirSync`/`makedirs` sobre um caminho que **já existe como symlink para
diretório** retornam sucesso e não são guarda nenhuma; nenhum dos 3 escritores faz `lstat` do
componente `<agente>` antes de escrever. É exatamente a **Classe 3** da nota
`lstat-nao-ve-junction-e-guarda-de-folha-nao-olha-ancestral-2026-08-31` (guarda de folha que nunca
olha ancestral) e a mesma classe de `update-segue-symlink-e-escreve-fora-do-projeto-2026-08-28`.
Quando `agents:` é vazio o componente é a constante `"default"` — **nenhuma alteração _hostil_ do
`trackfw.yaml` é necessária**: basta o `roadmap_namespacing: by_agent` legítimo, que é a
configuração que a ADR desta REQ promove. Sem `by_agent`, `REQWriteDir` devolve `reqDir` sem
componente `<agente>` e o vetor não existe. Isso torna o vetor mais barato do que o de config
hostil do §2, mas não o torna incondicional — a precisão importa porque uma descrição inflada do
dano produz o remédio errado.

O contraste que evidencia a inconsistência do controle: o **leitor** já recusa symlink como
namespace (`resolveAgentNamespaces` usa `DirEntry.IsDir()` / `withFileTypes` / `is_dir(follow_symlinks=False)`,
comentado como AC12/AC13 em `internal/validator/validator.go:1047`,
`npm/src/validator/index.js:201`, `pypi/trackfw/config.py:55`). O **escritor** não recusa nada. Par
escritor/leitor com duas noções de confiança — a mesma assimetria que a D4 da ADR existe para
proibir, só que no eixo de segurança em vez do eixo de layout.

**Prova de execução — direção que quebra.** Fixture: `trackfw.yaml` com `roadmap_namespacing:
by_agent` e **sem** `agents:`; `ln -s <fora> docs/req/default`.

```
### MAIN node req new:    created docs/req/REQ-2026-09-03-main-symlink.md
### BRANCH node req new:  created docs/req/default/REQ-2026-09-03-branch-symlink.md
--- fora/ ---       REQ-2026-09-03-branch-symlink.md      <- escapou
--- docs/req/ ---   default -> …/lab10/fora
                    REQ-2026-09-03-main-symlink.md        <- main ficou dentro
```

Os 3 runtimes da branch escapam (fixture equivalente em `lab3`: `go`, `node` e `py` produziram os
três `REQ-*.md` dentro de `$S/lab3/fora`, com `docs/req/default` sendo o único symlink).

**Sobrescreve, não só cria** — medido, e isso é o que sobe a severidade de "criação de lixo" para
integridade:

```
$ echo "CONTEUDO ORIGINAL" > <fora>/REQ-2026-09-03-branch-symlink.md
$ node npm/bin/trackfw req new "branch symlink"   # mesmo título -> mesmo slug
created docs/req/default/REQ-2026-09-03-branch-symlink.md
$ head -3 <fora>/REQ-2026-09-03-branch-symlink.md
---
status: Open
date: 2026-09-03          <- conteúdo anterior destruído, sem aviso e sem erro
```

**Prova de execução — direção que NÃO dispara (controle).** Sem symlink, `agents: [zeus]`:

```
created docs/req/zeus/REQ-2026-09-03-controle-go.md
created docs/req/zeus/REQ-2026-09-03-controle-node.md
created …/lab8/proj/docs/req/zeus/REQ-2026-09-03-controle-py.md
```

Os três gravam **dentro** do projeto, no canônico que a ADR define. O mecanismo não dispara no caso
legítimo — ele depende do symlink, não do modo `by_agent`.

**Por que não bloqueia.** A mesma fixture contra `origin/main`, agora no `roadmap new`:

```
$ ln -s <fora3> docs/roadmaps/default
$ node <main>/npm/bin/trackfw roadmap new "main rm symlink"
✓ created docs/roadmaps/default/backlog/ROADMAP-2026-09-03-main-rm-symlink.md
$ ls -R <fora3>   ->   backlog/ROADMAP-2026-09-03-main-rm-symlink.md      <- fora, em MAIN
```

O escritor de roadmap já tinha o furo idêntico antes deste PR. Reprovar ML-1A trocaria um furo em
dois escritores por um furo em um escritor, sem fechar nada.

**Remédio (vira REQ, não patch deste PR).** Guarda **compartilhada** de escrita, aplicada nos 3
escritores (`REQWriteDir`/`reqWriteDir`/`req_write_dir` e o `agentStateDir` equivalente do roadmap),
nos 3 runtimes: antes de `MkdirAll`, percorrer **cada componente** do caminho relativo entre a raiz
do projeto e o diretório-alvo e recusar (erro alto, não silencioso) se qualquer um for link. As
primitivas **divergem por runtime** e a REQ precisa dizer isso, sob pena do remédio errado em 2 dos
3: Node `lstatSync().isSymbolicLink()` já enxerga junction; Go precisa de
`ModeSymlink|ModeIrregular`; Python **precisa trocar a primitiva** (`islink` é falso para junction —
usar sucesso de `os.readlink()` ou `FILE_ATTRIBUTE_REPARSE_POINT`). Tudo já medido na nota
`lstat-nao-ve-junction-…-2026-08-31`; não remedir.
Dono: `trackfw_architect` abre a REQ; implementação é do dono de `internal/`, `npm/src/`, `pypi/`.

---

## 2. S3 — `agents:` hostil no `trackfw.yaml` leva escrita e leitura para fora de `req_dir`

**Arquivo:linha** mesmos pontos do §1 (`validator.go:1391-1394`, `index.js:370-380`,
`validator.py:611-625`) e, do lado da leitura, `internal/validator/validator.go:1418`
(`ResolveREQFiles`), `npm/src/validator/index.js:394`, `pypi/trackfw/validator.py:630`.

**Severidade: baixa.** Precondição é **escrever no `trackfw.yaml`** — quem consegue isso já está
dentro e já pode apontar `req_dir` para onde quiser sem nenhum `..`. Não é escalada. Registro pelo
valor de completude e porque o vetor **relativo** funciona nos 3, o que faz dele um bom teste de
regressão para a REQ do §1.

**Prova de execução — escrita, direção que quebra.** `agents: ["../../../../../../tmp/pwn"]`:

```
GO   -> created ../../../../tmp/pwn/REQ-2026-09-03-escape-teste.md
NODE -> created ../../../../tmp/pwn/REQ-2026-09-03-escape-teste-node.md
PY   -> created …/docs/req/../../../../../../tmp/pwn/REQ-2026-09-03-escape-teste-py.md
$ find … -name "*escape*"
…/trackfw/tmp/pwn/REQ-2026-09-03-escape-teste.md
…/trackfw/tmp/pwn/REQ-2026-09-03-escape-teste-py.md
…/trackfw/tmp/pwn/REQ-2026-09-03-escape-teste-node.md      <- os 3 fora do projeto
```

**Divergência de runtime no valor ABSOLUTO — só o Python escapa.** `agents: ["/private/tmp/pwnabs"]`:

```
GO   -> created docs/req/private/tmp/pwnabs/REQ-…-abs-go.md       <- CONTIDO (filepath.Join)
NODE -> created docs/req/private/tmp/pwnabs/REQ-…-abs-node.md     <- CONTIDO (path.join)
PY   -> created /private/tmp/pwnabs/REQ-…-abs-py.md               <- ESCAPOU
$ ls /private/tmp/pwnabs/   ->   REQ-2026-09-03-abs-py.md
```

`os.path.join(a, b)` **descarta `a`** quando `b` é absoluto; `filepath.Join` e `path.join` não. Este
é o caso em que o Python diverge dos outros dois. **Pré-existente, não deste diff**: a mesma config
já leva o `roadmap new` do Python para fora em `origin/main` —
`Roadmap criado: /private/tmp/pwnabs/backlog/ROADMAP-2026-09-03-abs-roadmap-py.md`.

**Prova de execução — leitura, direção que quebra.** `agents: ["../../../secreto"]`, com
`secreto/REQ-2026-01-01-arquivo-privado.md` fora do projeto:

```
GO   ## REQs (1)  - REQ-2026-01-01-arquivo-privado.md [wip]
NODE ## REQs (1)  - REQ-2026-01-01-arquivo-privado.md [wip]
PY   ## REQs (1)  - REQ-2026-01-01-arquivo-privado.md [wip]
$ trackfw validate
✗ req "REQ-2026-01-01-arquivo-privado.md" has no linked ADR
```

**O que vaza é limitado, e isso importa para a severidade**: a saída carrega **basename + `status`
do frontmatter**, nunca o caminho absoluto nem o corpo. `context` usa `filepath.Base(full)`
(`internal/generators/context.go`, `npm/src/commands/context.js:collectReqEntries`,
`pypi/trackfw/commands/context.py:_collect_req_entries`) e as mensagens de `validate` citam só o
basename. `collectTraceIdEntriesFromFiles` (`internal/validator/validator_traceid.go:117-140`)
guarda `path: f` completo na entrada, mas nenhuma das 5 mensagens de `traceid` imprime esse campo —
verificado na saída. Logo: **enumeração**, não divulgação de conteúdo.

Com valor **absoluto**, na leitura, só o Python enumera (mesma causa de `os.path.join`); Go e Node
devolvem `## REQs (0)` porque o caminho vira `docs/req/private/tmp/…`, que não existe.

**Direção que NÃO dispara (controle).** `agents: [zeus]` com `docs/req/zeus/REQ-…md` → `## REQs (1)`
nos 3, arquivo dentro do projeto, nenhuma leitura fora.

**Remédio.** Validar o valor de `agents:` **na fronteira de config** (`internal/config/config.go`,
`npm/src/config`, `pypi/trackfw/config.py`), não em cada consumidor: recusar entrada vazia após
trim, entrada contendo `/` ou `\`, entrada absoluta, e `.`/`..`. Um único ponto fecha os dois lados
(escrita do §1/§2 e leitura deste §) nos 3 runtimes e, de quebra, elimina a divergência de
`os.path.join`. Mesma REQ do §1.

---

## 3. Travessia por symlink na LEITURA — o `<estado>` é a porta, o `<agente>` não é

**Arquivo:linha** `internal/validator/validator.go:1440-1442` (laço sobre `reqLayoutStates` →
`ListMDFiles(filepath.Join(reqDir, state))`), `npm/src/validator/index.js:410`,
`pypi/trackfw/validator.py:665-666`.

**Severidade: baixa (enumeração, leitura, precondição interna).**

**Mecanismo, e a distinção que evita a conclusão alarmante.** Os nomes de **estado** são constantes
hardcoded — não passam por `resolveAgentNamespaces` e portanto **não** são filtrados por
`IsDir(follow_symlinks=false)`. `os.ReadDir`/`readdirSync`/`os.listdir` sobre
`req_dir/backlog` **seguem** o symlink porque a recusa mora na enumeração de namespaces, não na
leitura do diretório de estado. Já os nomes de **agente vindos do disco** são filtrados: um symlink
plantado em `req_dir/<qualquer-nome>` nunca vira namespace.

**Prova de execução — as duas metades na mesma fixture.** `docs/req/backlog -> <alvo>` e
`docs/req/agentelink -> <mesmo alvo>`, com um `REQ-2026-01-02-via-symlink.md` no alvo:

```
GO   ## REQs (1)     NODE ## REQs (1)     PY ## REQs (1)
```

**Um**, não dois: o caminho `docs/req/backlog/REQ-…md` entrou (symlink de estado seguido) e o
caminho `docs/req/agentelink/REQ-…md` **não** entrou. Removendo o symlink `backlog` e deixando só
`agentelink`:

```
## REQs (0)   - (none)
```

Zero — confirmação direta de que o filtro de namespace por tipo funciona e de que a porta é o nome
de estado.

**Atribuição, com a armadilha do vault explicitada.** A leitura preguiçosa seria "não é regressão
deste diff, o caso `req_dir/<estado>/*.md` já existia em `listREQFiles`". Ela está **errada**, e é
exatamente a armadilha que a nota `resolvedor-de-req-era-if-else-…-2026-09-03` nomeia: *"auditar a
função de nome parecido dá o veredito errado; conte pelos call sites das regras."* `listREQFiles`
é a função de **generators**; o que `validate`/`context` usavam era `resolveREQFiles`, e a forma
antiga dela (visível no próprio diff) era `if/else` — em `by_agent`, **só** `<agente>/<estado>/`;
fora dele, **só** `filepath.Glob(reqDir + "/*.md")`. **Nenhum** dos dois ramos lia
`req_dir/<estado>/*.md`. Logo: a superfície é pré-existente em `listREQFiles`, mas é **nova para as
regras** — o symlink de estado passa a ser seguido por `validate` e `context` nos dois modos. A
severidade não muda (leitura, enumeração de basename, precondição interna); a atribuição, sim.

**Remédio:** cobrir junto com a REQ do §1, aplicando a mesma guarda de componente **também na
leitura** dos diretórios de estado. Custo marginal, porque a guarda já terá de existir.

---

## 4. ⚠️ Dedup por caminho normalizado — **duplica** (não suprime), e a AC numérica do roadmap quebra em macOS/Windows

**Arquivo:linha** `internal/validator/validator.go:1428-1436` (`clean := filepath.Clean(p)`),
`npm/src/validator/index.js:400-407` (`path.normalize`), `pypi/trackfw/validator.py:654-661`
(`os.path.normpath`).

**A direção "suprimir arquivo legítimo" é INALCANÇÁVEL, e a razão é estrutural**, não empírica:
`Clean`/`normalize`/`normpath` são **puramente lexicais** e não consultam o filesystem. Dois
arquivos reais distintos têm strings de caminho distintas depois de normalizadas — não há como duas
entradas colapsarem numa. Controle que confirma: `req_dir/backlog/REQ-A.md` (agente legítimo
chamado `backlog`) + `req_dir/done/REQ-B.md` (agente legitimamente chamado `done`, o caso que a nota
do vault manda proteger) →

```
go=## REQs (2)   node=## REQs (2)   py=## REQs (2)
```

Dois, sem duplicata e sem sumiço. A dedup faz o que promete no caso legítimo.

**A direção alcançável é a inversa — a dedup FALHA e duplica.** Em filesystem case-insensitive
(APFS no macOS, NTFS no Windows), `req_dir/Backlog` e `req_dir/backlog` são o **mesmo** diretório mas
**strings diferentes** após normalização. O nome `Backlog` entra na lista de agentes (via disco) e
gera `req_dir/Backlog/*.md`; o laço de estados gera `req_dir/backlog/*.md`, hardcoded em minúscula.
O `seen` não vê a colisão.

**Prova de execução.** Fixture: `docs/req/Backlog/REQ-2026-01-03-caixa.md`, um único arquivo real:

```
GO   ## REQs (2)
     - REQ-2026-01-03-caixa.md [wip]
     - REQ-2026-01-03-caixa.md [wip]
NODE ## REQs (2)   (idem)

$ trackfw validate | grep -c "REQ-2026-01-03-caixa"
go=4   node=4   py=4        # 2 regras x 2 duplicatas, nos 3 runtimes
✗ req "REQ-2026-01-03-caixa.md" has no linked ADR
✗ req "REQ-2026-01-03-caixa.md" has no linked ADR
✗ req "REQ-2026-01-03-caixa.md" has no linked Roadmap
✗ req "REQ-2026-01-03-caixa.md" has no linked Roadmap
```

Este é **exatamente** o defeito que o §3 da nota
`resolvedor-de-req-era-if-else-…-2026-09-03` descreve e que a dedup foi criada para impedir
("cada violação sairia em duplicata, o que também estragaria a AC numérica"). A dedup fecha o caso
exato-igual e deixa aberto o caso equivalente-no-filesystem. **Não é segurança** (nada sai da árvore,
nada é suprimido); é correção, e é **nova neste diff**, porque só o caso `<agente>/*.md` introduzido
pelo ML-1A colide com o caso `<estado>/*.md`.

Em Linux (case-sensitive) não ocorre: `req_dir/backlog` minúsculo simplesmente não existe. Logo o
CI Linux **não pega**, e o desenvolvedor macOS pega. Vale a nota de vault.

**Remédio — direção certa, primitiva a determinar por medição.** Chavear o `seen` por identidade
**do filesystem**, não por string. Em **APFS/macOS eu medi** que a identidade discrimina nos 3
runtimes sobre as duas grafias do mesmo diretório:

```
py   ino/dev A 213071860 16777231 | B 213071860 16777231 | iguais: True
node ino/dev A 213071860 16777231 | B 213071860 16777231 | iguais: true
go   os.SameFile: true
```

🔴 **Não adote `st_ino`/`ino` como primitiva sem medir em NTFS.** O defeito vive nos dois lados
(APFS **e** NTFS) e eu só pude medir um. A lei geral deste repositório é a da nota
`lstat-nao-ve-junction-e-guarda-de-folha-nao-olha-ancestral-2026-08-31`: *no Python a primitiva
tem de ser trocada, não só complementada* — a intuição de "a mesma chamada serve nos 3" já produziu
o remédio errado em 2 dos 3 CLIs uma vez. `os.SameFile` do Go é a que tem contrato explícito de
identidade; `st_ino`/`st_dev` do Python e `ino`/`dev` do Node no NTFS precisam de medição própria
(a sonda de Windows já existe e é o instrumento certo) antes de virarem contrato. Se a medição
reprovar, o fallback aceitável é comparar também a forma case-folded quando
`GOOS ∈ {darwin, windows}` — mais barato, mas **não cobre NFC/NFD** e por isso é segunda opção, não
primeira. Em todos os casos, fallback para a string quando o `stat` falhar.
**Dono:** quem detém `internal/validator`, `npm/src/validator` e `pypi/trackfw/validator.py`; a
medição de NTFS é pré-requisito do ML, não um "verificar depois".

---

## 5. ⚠️ Paridade quebrada em código NOVO — primeiro `agents:` vazio manda Go e Node/Python para diretórios diferentes

**Arquivo:linha** `internal/validator/validator.go:1391`
(`if len(cfg.Agents) > 0 && cfg.Agents[0] != ""`) versus `npm/src/validator/index.js:375`
(`const agents = (cfg.agents || []).filter(a => a)`) e `pypi/trackfw/validator.py:624`
(`agents = [a for a in (cfg.get("agents") or []) if a]`).

**Severidade: baixa (correção/paridade, não segurança).** Go testa **só o primeiro** elemento e cai
em `"default"`; Node e Python **filtram os vazios** e pegam o primeiro não-vazio.

**Prova de execução.** `agents: ["", "zeus"]`:

```
GO   -> created docs/req/default/REQ-2026-09-03-vazio-go.md
NODE -> created docs/req/zeus/REQ-2026-09-03-vazio-node.md
PY   -> created …/docs/req/zeus/REQ-2026-09-03-vazio-py.md
```

Mesmo repositório, mesmo `trackfw.yaml`, **dois destinos de escrita**. Nenhum sai do projeto e ambos
estão na união de leitura, então nada some do `validate` — mas é a regra dura de paridade violada
dentro da função que este PR acabou de criar, e contradiz o D4 ("o par escritor/leitor não pode ter
duas noções de layout") no eixo entre runtimes. **Controle:** com `agents: [zeus]` os 3 convergem
para `docs/req/zeus/` (§1).

**Remédio (uma linha, mesmo arquivo do PR).** Em `REQWriteDir`, trocar o teste pelo filtro:
selecionar o primeiro `cfg.Agents[i] != ""` em vez de olhar só o índice 0. Recomendo aplicar agora,
pelo custo; se o `trackfw_architect` preferir agrupar com a REQ dos §1/§2, aceito — não bloqueio.
**Não sou eu quem edita** `internal/`: handoff explícito para o dono de `internal/validator`
(o mesmo do §1 e do §4), coordenado pelo `trackfw_architect`, que é a única autoridade de git.

---

## 6. Vetores que ataquei e vieram FECHADOS — com o mecanismo, não com "não achei"

**6.1 `<agente>` vindo do disco com `..`, `../..` ou separador embutido — inalcançável por
construção.** `os.ReadDir`, `readdirSync` e `os.scandir` devolvem **basenames**: `.` e `..` são
filtrados pelo wrapper de syscall e um basename não pode conter `/` em POSIX (nem `/`/`\` em
NTFS). O nome de diretório lido do disco **não tem como** carregar travessia. Isto não é uma
medição negativa — é uma propriedade do `readdir`. O vetor de travessia real é o `agents:` da
config (§2), que é outra origem.

**6.2 Metacaracteres de glob no nome de agente — fechado, e verifiquei que continua fechado.** Era o
achado 2 do meu parecer de 2026-08-30. ML-1A preserva `ListMDFiles`/`listReqMdFiles`/`_list_md_files`
(que fazem `readdir` + filtro de extensão) em vez de `filepath.Glob`/`glob.glob`. Fixture com três
agentes chamados `*`, `[a-z]` e `com espaco`, um `.md` em cada:

```
go=## REQs (3)     node=## REQs (3)
py -> resolve_req_files() = ['docs/req/*/REQ-star.md',
                             'docs/req/[a-z]/REQ-brack.md',
                             'docs/req/com espaco/REQ-space.md']
```

Três em cada, sem cross-match e sem queda de contagem. Nenhum nome foi interpretado como padrão.

**6.3 Symlink como namespace de agente — fechado.** §3 acima: `## REQs (0)` com o symlink
`agentelink` sozinho em `req_dir`. O filtro por tipo (`IsDir(follow_symlinks=false)` e equivalentes)
faz o trabalho.

**6.4 Dedup suprimindo arquivo legítimo — inalcançável.** §4: normalização lexical não colapsa
arquivos reais distintos; controle com agente chamado `done` devolve 2, não 1.

**6.5 O gate do ML-2A NÃO é vácuo — falsifiquei as 3 fronteiras por conta própria.** Revisar o
script por injeção (§6.6) responde outra pergunta. A que importa aqui é *"ele reprova a regressão
que existe para pegar?"* — e é a pergunta que eu já deixei sem resposta uma vez (memória
`feedback_execute_all_named_vectors_before_verdict`, ML-3A Wave-0). Rodei o gate íntegro e depois
reproduzi as 3 sabotagens dos cenários 183/184/185 em cópias fora do repositório, cada uma na sua
fronteira:

```
# baseline — árvore íntegra
$ scripts/check-artifact-closed-cycle.sh
Closed-cycle checks passed (3 artefatos x 3 CLIs x 2 layouts = 18 combinacoes)   EXIT=0

# 183 — REQ, verificador (Go): remove `add(ListMDFiles(filepath.Join(reqDir, agent)))`, rebuild
OK   [closed-cycle/req/go/flat/req_has_adr-names-generated]                 <- flat continua passando
closed cycle broken: req/go/by_agent/req_has_adr-names-generated           <- DETECTA
closed cycle broken: adr/go/by_agent/status-literal-read-back              <- colateral, ver abaixo
closed cycle broken: adr/go/by_agent/adr_orphan-clears-after-link          <- colateral

# 184 — NOTE, gerador (Node): link do index.md vira `(notes/<arquivo>.md)`
closed cycle broken: note/node/flat/note_orphan-silent-for-indexed         <- DETECTA
OK   [closed-cycle/note/node/flat/note_orphan-fires-for-unindexed]         <- a outra metade intacta

# 185 — ADR, gerador (Python): default do argparse vira "Rascunho" (+ choices, senão aborta antes)
closed cycle broken: adr/python/flat/status-literal-read-back              <- DETECTA
OK   [closed-cycle/adr/python/flat/adr_orphan-clears-after-link]
```

As três reprovam **com o label exato** que `assert_fails_with` fixa nos cenários, e o braço
`go/flat` continua verde sob a sabotagem do 183 — isto é o que separa "detectou" de "quebrou o
binário e reprovou tudo".

**O colateral do 183 é evidência a favor do desenho, não ruído.** Tirar o caso canônico de REQ
também derruba `adr/go/by_agent/status-literal-read-back` e
`adr/go/by_agent/adr_orphan-clears-after-link` — porque em `by_agent` as fixtures de ADR dependem
da REQ ser enumerada. Mas derruba **só no braço `go/by_agent`**: `note_orphan` fica intacto em
todos os braços (a nota nunca toca `req_dir`) e Node/Python passam inteiros. Ou seja, uma sabotagem
única cobre menos de um terço da matriz — os cenários 184 e 185 precisam das suas próprias
sabotagens, em runtimes e lados da fronteira diferentes. O desenho do ML-2A está certo pelo motivo
que ele declara.

**6.6 `scripts/check-artifact-closed-cycle.sh` — sem superfície de injeção que eu tenha encontrado.**
Varri por `eval`, substituição de comando em contexto não citado, `chmod`, `curl`, `shell=True`. Não
há `eval`. `WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-closed-cycle.XXXXXX")` com
`trap 'rm -rf "$WORK"' EXIT` (linhas 54-55). O único `rm -rf "$dir"` (linha 195) recebe
`"$WORK/$runtime-$layout"` (linha 224), com `$runtime` e `$layout` literais de laço — não há
caminho de dado externo até ele. `HOME` isolado em `"$WORK/home"` (linha 64), `set -euo pipefail`,
expansões citadas. Uma ressalva **de medição, não de segurança**, herdada e já registrada no vault:
sob `$TMPDIR` com componente simbólico (macOS), `roadmapTrustForGates` cai em fail-open
(`barrier-trust-check-fail-open-em-tmpdir-simbolico-2026-08-29`) — não afeta este gate, que executa
o CLI diretamente.

---

## 7. Observação fora do escopo — `trackfw context` do Python aborta quando há violação de string

`pypi/trackfw/commands/context.py:160`:

```python
violations = [v["message"] for v in result.get("violations", [])]
```

`validate()` mistura `dict` e `str` na lista de violações; a violação
`agent namespace "X" exists in req_dir but is not declared in agents:` sai como **string**, e o
comando morre com `trackfw context: string indices must be integers, not 'str'`. Reproduzido na
fixture do §4.

**Pré-existente, não deste diff** — verificado contra `origin/main` na **mesma fixture**:

```
### lab4 com pypi de MAIN    -> trackfw context: string indices must be integers, not 'str'
### lab4 com pypi da BRANCH  -> trackfw context: string indices must be integers, not 'str'
```

Idêntico nos dois lados. Não bloqueia, não é da REQ desta branch, e é REQ própria: `context` do
Python fica indisponível em qualquer projeto com namespace de agente não declarado — que é
justamente a configuração que a REQ-2026-08-29 tornou comum.

---

## 8. Resumo de decisão

**Bloqueia o merge:** nada.

**Recomendo corrigir dentro deste PR (barato, mesmo arquivo, sem risco):**
- §5 — `REQWriteDir` do Go filtrar entradas vazias de `agents:` como Node/Python já fazem.

**Vira REQ própria (nesta ordem de prioridade):**
1. §1 + §2 + §3 — **guarda de escrita e leitura contra link em componente de caminho**, compartilhada
   pelos escritores de REQ **e** de roadmap nos 3 runtimes, **mais** validação de `agents:` na
   fronteira de config. As primitivas divergem por runtime; usar a tabela já medida em
   `lstat-nao-ve-junction-e-guarda-de-folha-nao-olha-ancestral-2026-08-31` em vez de remedir.
2. §4 — dedup de `ResolveREQFiles` por identidade de filesystem (inode/dev), não por string
   normalizada. Quebra a AC numérica do próprio roadmap em macOS/Windows e o CI Linux não pega.
   **AC obrigatória desta REQ** (não recomendação solta): nota de vault escrita e linkada no
   `index.md` no mesmo ML, ligada a `resolvedor-de-req-era-if-else-…-2026-09-03`, registrando que a
   dedup lexical falha em FS case-insensitive e que o sintoma é verde no CI Linux / vermelho na
   máquina do dev. Sem AC explícita, a nota não é escrita e o próximo revisor remede isto do zero.
3. §7 — `context.py:160` tolerar violação em forma de string (pré-existente).

**Nota de vault:** transformada em **AC do item 2** acima, com dono e ML definidos. Recomendação a
parte não nomeada é como nota não é escrita — e o §4 custa mais de dez minutos a quem topar com ele
amanhã. Eu não a escrevo aqui porque esta barreira tem fronteira de um único arquivo.

> Nenhuma operação de git executada. Nenhum arquivo de produto modificado. Único arquivo escrito por
> esta barreira: este parecer, mais a entrada de início/fim em `docs/agents-working-context.md`.
