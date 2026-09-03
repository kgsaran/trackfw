---
title: Dedup lexical não vê req_dir/Backlog ≡ backlog em APFS/NTFS — e o conserto óbvio (lowercase ou inode) troca dupla contagem por SUPRESSÃO; o mecanismo certo é filtro de existência verbatim
tags: [validator, by_agent, req_dir, dedup, case-insensitive, apfs, ntfs, paridade, supressao]
date: 2026-09-03
related: [[resolvedor-de-req-era-if-else-e-a-uniao-colide-com-o-namespace-vindo-do-disco-2026-09-03]], [[lstat-nao-ve-junction-e-guarda-de-folha-nao-olha-ancestral-2026-08-31]], [[uniao-disco-agents-mascara-gate-por-presenca-2026-08-29]]
---

## 1. O defeito: a dedup que a nota anterior mandou criar fecha o caso exato e deixa aberto o equivalente

A união dos 4 layouts de `req_dir` precisa de dedup porque `agents:` é unido ao disco: um
`req_dir/backlog/` real entra na lista de agentes e o caso `<agente>/*.md` emite os **mesmos paths**
do caso `<estado>/*.md`. A dedup implementada usava `filepath.Clean` / `path.normalize` /
`os.path.normpath` — **puramente lexical**.

Em filesystem **case-insensitive** (APFS no macOS, NTFS no Windows) `req_dir/Backlog` e
`req_dir/backlog` são o **mesmo diretório** mas **strings diferentes** depois de normalizadas. O nome
`Backlog` entra na lista de agentes pelo disco; o laço de estados usa `backlog` **hardcoded em
minúscula**. O `seen` não vê a colisão.

Medido com **um** arquivo real em `docs/req/Backlog/`, projeto `by_agent`:

```
antes   go=4  node=4  py=4  violações   (context: REQs (2), o MESMO basename duas vezes)
depois  go=2  node=2  py=2  violações   (context: REQs (1))
```

**Verde no CI Linux, vermelho na máquina do dev** — em Linux o `backlog` minúsculo simplesmente não
existe, então o CI nunca pega. Foi `hades-tf` quem achou, no §4 do parecer de segurança.

## 2. 🔴 Os dois consertos óbvios trocam dupla contagem por SUPRESSÃO — que é pior

**Lowercase (case-folding).** Resolve em APFS e **suprime em Linux**: num FS case-**sensitive**
`Backlog` e `backlog` são dois diretórios reais e **legítimos**; colapsá-los apaga um arquivo real da
enumeração. Dupla contagem estraga uma AC numérica; supressão faz uma violação real desaparecer sem
ninguém notar. Não são o mesmo tamanho de erro. Também não cobre NFC/NFD.

**Identidade de inode (`os.SameFile` / `ino`/`dev` / `st_ino`/`st_dev`)** — foi o remédio sugerido
pelo parecer, e foi **rejeitado por portabilidade, não por gosto**:

- **Go não tem chave hasheável portátil de `(dev,ino)`**: `syscall.Stat_t` não existe no Windows, e
  `os.SameFile` é par-a-par → dedup O(n²). Só dá para usar com código específico de plataforma.
- **`ino` que repete ou lê `0`** (FS de rede, alguns FS de Windows) **colapsa arquivos distintos** —
  supressão outra vez, e sem sintoma visível.
- A primitiva do Python **não tem contrato medido em NTFS**, e a lei da nota
  `lstat-nao-ve-junction-…-2026-08-31` vale aqui: no Python a primitiva tem de ser **trocada**, não
  complementada — "a mesma chamada serve nos 3" já produziu o remédio errado em 2 de 3 CLIs uma vez.

## 3. O mecanismo que ficou: filtro de existência VERBATIM (mede o disco, não presume o FS)

Um candidato de subdiretório só é enumerado se o nome aparecer **literalmente** na listagem do pai
(`os.ReadDir` / `fs.readdirSync` / `os.listdir`, memoizado por pai). A **grafia do disco é a
autoridade**.

Por que fecha sem abrir nada:

- **Não pode suprimir.** Em FS case-sensitive o `readdir` devolve `Backlog` **e** `backlog`, então os
  dois passam o filtro e os dois seguem enumerados. Em FS case-insensitive existe **um** nome real,
  e é ele que passa.
- **Não presume a propriedade do filesystem** — não há `GOOS ∈ {darwin,windows}` nem probe de
  case-sensitivity. O disco é medido a cada leitura.
- **Cobre NFC/NFD de graça**: um `agents:` em outra forma Unicode é filtrado, não duplicado.
- **Fallback é join cego, NUNCA lista vazia**: pai ilegível → não filtra nada e volta ao
  comportamento anterior (dupla contagem, benigna). Devolver `[]` ali seria supressão — é o único
  ponto onde uma implementação descuidada vira para o lado ruim.
- **Não filtra por TIPO de entrada** (de propósito): um `<estado>` que seja symlink continua sendo
  enumerado exatamente como antes. Filtrar por `IsDir()` fecharia, de carona, a porta de travessia
  por symlink do §3 do parecer — mudança de comportamento fora do microlote.

Onde: `ResolveREQFiles` (`internal/validator/validator.go`), `resolveReqFiles`
(`npm/src/validator/index.js`), `resolve_req_files` (`pypi/trackfw/validator.py`). O `seen` lexical
**continua necessário** — ele fecha o caso string-idêntica (`agents: [backlog]` + dir `backlog`).

## 4. Como falsificar isto amanhã sem depender de acreditar

Um teste que só roda em APFS **ou** só em Linux não mede nada aqui — o defeito e o anti-remédio
vivem em filesystems opostos. Dá para ter os dois no mesmo host macOS:

```bash
hdiutil create -size 60m -fs "Case-sensitive APFS" -volname CSVOL /tmp/csvol.dmg
hdiutil attach /tmp/csvol.dmg -mountpoint /tmp/csmnt -nobrowse
mkdir -p /tmp/csmnt/probe/Backlog /tmp/csmnt/probe/backlog   # coexistem: é case-sensitive
```

- **Árvore APFS normal**, 1 arquivo em `docs/req/Backlog/` → tem de dar **2** violações. Desligando o
  filtro, volta a **4** nos 3 CLIs. (Sabotar o Go com `if false {` **não compila** — variável não
  usada; sabote o `return false` final do helper para `return true`.)
- **Volume case-sensitive**, `Backlog/…-A.md` + `backlog/…-B.md` → `REQs (2)` com basenames
  **DISTINTOS**. Dois basenames iguais é o bug; um só é supressão. Este é o controle que separa
  "consertou" de "trocou de defeito".

## 4-bis. 🔴 Efeito colateral que só o `make quality` pega: o Cenário 183 casa por LITERAL

`scripts/check-gates-falsify.sh` (Cenário 183) sabota o resolvedor com `corrupt_literal` sobre a
string exata `add(ListMDFiles(filepath.Join(reqDir, agent)))`. Trocar essa chamada por
`addChild(reqDir, agent)` **não quebra nada em build, teste ou gate de ciclo fechado** — quebra o
`make quality` lá no fim, com:

```
[s183-req-canonical-case] expected exactly 1 occurrence of pattern, got 0
make: *** [parity] Error 1
```

E é **fail-closed**, o que é a propriedade certa: o cenário reprova alto em vez de virar vácuo
silencioso. Mas custa uma rodada inteira de `make quality` (~13 min) para descobrir. **Regra: ao
mexer em qualquer linha de `ResolveREQFiles`, grepe o literal em `check-gates-falsify.sh` ANTES de
rodar o quality.** O seam permanece o mesmo (o caso (3) sai do resolvedor); só a grafia muda, e o
retarget é uma linha.

## 5. Item irmão achado no mesmo lote: `agents: ["", "zeus"]` divergia entre runtimes

`REQWriteDir` do Go testava **só o índice 0** (`len(cfg.Agents) > 0 && cfg.Agents[0] != ""`) e caía em
`default`; Node e Python **filtravam os vazios** e pegavam `zeus`. Mesmo `trackfw.yaml`, dois destinos
de escrita.

O discriminante **não** foi "o parecer recomendou": o comentário do Go dizia "mesma convenção de
`roadmap new`" e isso é **verdade** — os três `roadmap new` testam o índice 0 sem filtrar
(`generators/roadmap.go:108`, `generators/roadmap.js:72`, `generators/roadmap.py:170`). O que desempata
é o **lado leitor**: `resolveAgentNamespaces` já descarta `a == ""` nos 3 runtimes. Filtrar devolve
**uma** noção de agente ao par escritor/leitor (D4). String vazia não é nome de agente — é ausência de
entrada. O índice 0 do `roadmap new` é pré-existente na `main`, idêntico nos 3, e virou resíduo
declarado.

## 6. E a lição de documento: a ESPÉCIE do resíduo importa mais que a lista

`docs/cli-parity.md` afirmava que "every rule, generator and command" chama o ponto único, e declarava
como resíduo o `traceid.js` do Node — que é **superconjunto** (lê mais que os 4 layouts, nunca vácuo,
benigno). Ficaram **fora** da lista os dois resíduos **vácuos**: `serve /api/chain` (medido: `go=1 nó`,
`node=0`, `py=0` na mesma fixture canônica) e o inventário de `status` do Python
(`status.py:50`, medido `go=1 node=1 py=0`). **Declarar o benigno e omitir o vácuo é pior que não ter
lista** — vira "coberto" aos olhos do próximo auditor. Ao enumerar delegantes, enumere **por runtime**:
o conjunto não é uniforme (o `traceid` do Node e o `status` do Python são células diferentes da mesma
tabela).
