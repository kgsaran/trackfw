---
status: Open
date: 2026-09-25
author: "trackfw_architect"
adr: "docs/adr/ADR-2026-09-29-quando-uma-req-deve-ter-roadmap-e-o-casamento-req-roadmap-nao-depende-de-req-id-no-roadmap.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-29-traceid-orphan-req-reprova-estado-correto-por-duas-causas-distintas.md"
---

# REQ: regra de rastreabilidade ignora o estado da REQ e acusa backlog como orfao

> Date: 2026-09-25 | Status: Open
| Linear Issue: 
| Jira Issue: 

Origem: **issue #435**, reportada a partir de **uso real** por um projeto consumidor (CMDB, com
`roadmap_namespacing: by_agent` e `req_dir: docs/requisições`), não de leitura de código.

## Motivation

`trackfw validate` emite `traceid_orphan_req` para toda REQ cujo `req_id` não tem roadmap de mesmo
id — **inclusive quando a REQ está em `backlog/`**, onde não ter roadmap é o estado **correto**.

A autoridade normativa é o próprio protocolo que o trackfw recomenda:

```
trackfw req new  →  trackfw roadmap new  →  trackfw roadmap move <nome> wip  →  branch
```

Uma REQ em `backlog/` sem roadmap não é órfã: é **uma REQ que ainda não começou**. Exigir roadmap ali
inverte o fluxo — ou se criam roadmaps vazios para satisfazer o `validate`, ou se convive com violação
permanente.

### O custo real, medido pelo reportante

No consumidor há **14 ocorrências de `orphan_req` congeladas no `.trackfw-baseline.json`**. 🔴 **O
baseline virou o mecanismo de convivência com uma regra que dispara em estado correto** — e isso não é
inerte: **toda REQ nova criada em `backlog/` deixa o check `trackfw-chain` vermelho** até alguém
regenerar o baseline, o que apaga o passivo de **todos**, não só o novo. Foi o que aconteceu: duas REQs
legítimas criadas em `backlog/`, CI vermelho.

### Medição do arquiteto em 2026-09-25 — e o issue reportou METADE do defeito

**Sítio 1 — `traceid_orphan_req`** (`internal/validator/validator_traceid.go:262-277`): o laço percorre
`reqIndex` e **nunca consulta `e.state`**. 🔴 **O campo existe, é preenchido para REQ na linha 138, e é
usado 10 linhas abaixo** por `traceid_state_mismatch` (linha 289). O dado está na mão e é ignorado.

**Sítio 2 — `req_has_roadmap`** (`internal/validator/validator.go:798` → `validateREQsHaveRoadmap`):
itera **todas** as REQs exigindo `roadmap:` preenchido e **não tem sequer o estado disponível** — usa
`resolveREQFiles`, que devolve só caminhos. 🔴 **Uma REQ em `backlog/` dispara DUAS regras, não uma.**

Pela Regra Dura de Causa Raiz — mesma causa, mesma REQ — os dois sítios entram aqui. A enumeração
completa é entregável da Wave 0: **não presumir que são exatamente dois.**

### Por que sobreviveu tanto tempo

```
$ grep -n 'req_dir\|roadmap_namespacing' trackfw.yaml
req_dir: docs/req
roadmap_namespacing: flat
$ trackfw validate | grep -c orphan_req
0
```

As REQs **deste** repositório são **flat**, sem subpastas de estado, logo `state = ""` para todas e a
regra **nunca dispara no upstream**. 🔴 **A superfície só existe no consumidor, e o sinal de que ela
falha chega sempre de fora.** É a mesma família do **#396** (teste lia `docs/roadmaps/done` plano e
reprovava consumidor `by_agent`) — e a consequência para esta REQ é direta: **uma correção sem fixture
que construa o layout do consumidor produz um gate vacuoso**, verde por não exercitar nada.

### Por que a sugestão do issue não é o recorte certo

O issue sugere rebaixar a regra a `warning`. Medido: `applyRule` (`validator.go:504`) já suporta
`off · warning · error` por config — logo **isso já é possível hoje**, e é **convivência, não
correção**. Rebaixar perde o caso legítimo: REQ em `wip` **sem** roadmap é defeito real, e é
exatamente o que `branch_has_wip_roadmap` cobre do outro lado.

O recorte proposto é **ignorar `backlog/` e `abandoned/`**, mantendo `error` de `wip` em diante — a
confirmar pela Wave 0, que decide também sobre `analyzing/`.

## Acceptance Criteria

- [x] ~~**Enumeração real** das regras do validador que decidem sem consultar o estado do artefato,~~ — **SUBSTITUÍDO** pelos ACs consolidados (2026-09-29)
      classificadas em: **(a)** decide errado por ignorar o estado → defeito · **(b)** consulta o
      estado e está correta · **(c)** o estado é irrelevante para a regra. 🔴 **Não parar nos dois
      sítios já medidos**
- [x] ~~Todo sítio **(a)** corrigido, com a lista de estados isentos **declarada em um só lugar** — não~~ — **SUBSTITUÍDO** pelos ACs consolidados (2026-09-29)
      espalhada por regra
- [x] ~~🔴 **Fixture que constrói o layout do consumidor** (`roadmap_namespacing: by_agent`, REQ dentro~~ — **SUBSTITUÍDO** pelos ACs consolidados (2026-09-29)
      de pasta de estado). Sem ela o gate é vacuoso: o upstream é `flat` e não exercita a superfície
- [x] ~~Falsificação nas **duas** direções: REQ em `backlog/` sem roadmap **não** acusa; REQ em `wip/`~~ — **SUBSTITUÍDO** pelos ACs consolidados (2026-09-29)
      sem roadmap **continua** acusando
- [x] ~~O consumidor consegue **remover as 14 entradas** de `orphan_req` do `.trackfw-baseline.json` sem~~ — **SUBSTITUÍDO** pelos ACs consolidados (2026-09-29)
      que o `trackfw-chain` fique vermelho
- [x] ~~`make quality` e **CI** verdes~~ — **SUBSTITUÍDO** pelos ACs consolidados (2026-09-29)


## ❌ RETIRADA (2026-09-26): o #439 saiu daqui — a causa é outra, e o erro de atribuição foi meu

🔴 **Correção minha, declarada em vez de silenciosa.** Absorvi o #439 nesta REQ em 2026-09-25 por
**mesmo sintoma** (passivo congelado no baseline do consumidor: 78 `stale state path` + 14
`orphan_req`). A medição do `ML-1B` da `REQ-2026-09-09-req-nasce-orfa-…` mostrou que a **causa** é
outra, e é de lá:

| | causa |
|---|---|
| **esta REQ (#435)** | regra de **leitura** decide sem consultar o **estado da pasta** |
| **#439** | **o comando conhece o vínculo e não o escreve** — e `syncREQReferences`, o sincronizador da REQ-2026-09-09, é **literalmente a função que falta do outro lado** |

**O teste literal separa nos dois sentidos:** isentar `backlog/` na regra não faz `req move`
sincronizar; e fazer `req move` sincronizar não impede a regra de acusar REQ em `backlog/`.

⚠️ **Eu havia escrito aqui que "o custo de juntar é zero porque a REQ ainda não começou".** Era
verdade sobre o custo, e **irrelevante sobre a causa** — agrupar por sintoma quando a causa é
conhecida é o erro que a Regra Dura nomeia. O #439 está agora na `REQ-2026-09-09`, cuja Wave 1 já
entregou o sincronizador de um dos lados.

**O que fica desta ampliação:** nada do escopo. A medição abaixo foi movida para a REQ correta e é
mantida aqui **apenas como registro do erro**, riscada.

<details><summary>Texto original da ampliação (retirado — mantido para auditoria)</summary>

### A assimetria, medida por mim (2026-09-25)

**Origem:** issue **#439**, do mesmo consumidor, no mesmo dia. `trackfw req move` **não atualiza o
campo `req:`** dos roadmaps que apontam para a REQ movida.

### A assimetria, medida por mim

```
$ grep -n 'syncREQReferences' internal/generators/roadmap.go
760:  if syncErr := syncREQReferences(filepath.Base(src), portableDst); syncErr != nil {
1117: func syncREQReferences(roadmapBasename, newRoadmapPath string) error {

$ awk '/^func MoveREQ/,/^}/' internal/generators/req.go | grep -c 'sync'
0          ← 115 linhas, nenhuma chamada de sincronização
```

`roadmap move` sincroniza **e anuncia** (`✓ synced REQ-….md → …`). `req move` **não faz e não diz**.

🔴 **A assimetria é pior que a falta.** Quem segue a ordem que o próprio protocolo recomenda
(`roadmap move` → `req move`) termina com o repositório **inconsistente** e só descobre no `validate`
seguinte — normalmente no CI, **já dentro do PR**. No consumidor há **78 violações de `stale state
path` congeladas no baseline**; somadas às 14 de `orphan_req`, são **92 entradas de passivo** que
existem para conviver com dois defeitos.

⚠️ **Nota irônica e útil:** `syncREQReferences` é a **mesma função** que o analisador de contenção
nomeou, no corpus pré-fix da REQ-2026-08-31, como *"the false marker in its purest form"*. Ela já é
o ponto de sincronização do projeto; o que falta é a **simetria**.

### Por que entra nesta REQ, e não em REQ própria

**Pelo teste literal, as causas são distintas:** corrigir `MoveREQ` **não** fecha o #435, e isentar
`backlog/` na regra **não** fecha o #439. Se o critério fosse só esse, seriam duas REQs.

🔴 **Mas o sintoma é o mesmo** — *"o estado mora na pasta e o resto do sistema não acompanha"* — e a
Regra Dura é explícita: **mesmo sintoma investiga junto; só se separa com a medição escrita.** O ônus
é de quem quer dividir. Como esta REQ **ainda está em `backlog/`, sem uma linha de trabalho feita**, o
custo de juntar é **zero** e o ganho é a Wave 0 medir as duas superfícies com a mesma régua:

| | superfície | direção |
|---|---|---|
| **#435** | regra de **leitura** decide sem consultar o estado | passiva |
| **#439** | comando de **escrita** não propaga a mudança de estado aos apontadores | ativa |

**A Wave 0 decide se são uma causa ou duas, e a decisão fica escrita.** Se forem duas, a separação
acontece **com a medição**, não por presunção — que é exatamente o que a regra exige.

</details>

### ~~Critérios de aceite acrescentados~~ — RETIRADOS com o #439

- [x] ~~**Enumeração dos apontadores:** que campos referenciam artefato por caminho que **contém o~~ — **RETIRADO** desta REQ (#439 saiu; causa é outra)
      estado**? (`roadmap:` na REQ, `req:` no roadmap, `adr:`, `Roadmap:`/`REQ:` de corpo, …) — e quais
      comandos os movem
- [x] ~~🔴 **A simetria é fechada ou a assimetria é declarada.** Se `req move` não for sincronizar, ele~~ — **RETIRADO** desta REQ (#439 saiu; causa é outra)
      **avisa** — o que não pode continuar é *"um anuncia, o outro cala"*
- [x] ~~Falsificação nas duas direções: mover a REQ **atualiza** quem aponta para ela; e o comando~~ — **RETIRADO** desta REQ (#439 saiu; causa é outra)
      **não** reescreve apontador que não era dela
- [x] ~~O consumidor consegue remover as **78 entradas de `stale state path`** do baseline sem o~~ — **RETIRADO** desta REQ (#439 saiu; causa é outra)
      `trackfw-chain` ficar vermelho

## Negative scope — o que esta REQ NÃO faz

- **Não** desliga nem rebaixa `traceid_orphan_req`. Rebaixar é convivência disponível hoje por config,
  e perderia o caso legítimo de REQ em `wip` sem roadmap.
- **Não** corrige **#273** (`branch_has_wip_roadmap` erra nas duas direções). Mesmo **sintoma** —
  regra de rastreabilidade que erra —, **causa medida diferente**: lá é **casamento textual** entre
  nome de branch e de roadmap; aqui é **estado ignorado num índice que o possui**. A distinção fica
  escrita, como a Regra Dura exige de quem quer separar.
- **Não** migra layout de ninguém nem altera `req_dir`/`roadmap_namespacing` deste repositório.
- **Não** regenera o `.trackfw-baseline.json` do consumidor — isso é dele, e só faz sentido depois.
- **Não** muda o formato do campo `req:`/`roadmap:` nem introduz referência por id em vez de caminho.
  Seria outra decisão, com outro custo, e não é necessária para fechar estas causas.

## Linked ADR
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/backlog/ROADMAP-2026-09-25-regra-de-rastreabilidade-ignora-o-estado-da-req-e-acusa-backlog-como-orfao.md`


---

## 🔴 CONSOLIDAÇÃO (2026-09-29) — esta REQ absorve a duplicata que eu criei por falha de varredura

**Erro meu, registrado porque é o que evita repetição.** Em 2026-09-29 criei a
`REQ-2026-09-29-traceid-orphan-req-reprova-estado-correto-por-duas-causas-distintas` para o **mesmo
#435** — sem ver que **esta REQ já existia, aberta, escrita por mim quatro dias antes**.

Varri os **issues** abertos, como a regra manda. **Não varri as REQs abertas** — e a Regra Dura é
explícita: *"REQs abertas com o mesmo mecanismo têm o mesmo peso; a regra manda absorvê-las em vez de
abrir trabalho paralelo"*. Citei essa regra três vezes no mesmo dia para decidir escopo, e a violei.

**A duplicata foi marcada `Abandoned`.** Esta REQ — a vigente — recebe a ADR, o roadmap em `wip` e
tudo que a duplicata produziu.

### O que a consolidação traz de novo

**C2 — a SEGUNDA causa, que esta REQ não continha.** Reproduzida na v9.1.0: um par REQ↔roadmap
**válido e pareado** dispara `traceid_orphan_req` porque `roadmap new` **não escreve `req_id:`** no
frontmatter do roadmap (medido: `grep -c '^req_id:'` no roadmap gerado = **0**). O casamento é por
id, e o par fica invisível.

🔴 **As duas causas se parecem no log e diferem no mecanismo.** Quem corrigir só uma vê a regra
continuar reprovando e conclui que a correção falhou.

**ADR própria** (`ADR-2026-09-29-quando-uma-req-deve-ter-roadmap-e-o-casamento-req-roadmap-nao-depende-de-req-id-no-roadmap.md`), com o desenho decidido: recorte semântico por `status:`, casamento por
vínculo real (`req:`), e as **quatro** regras vizinhas alinhadas.

**Wave 0 auditada**, que refutou duas afirmações minhas e mediu o alcance real de cada decisão.

### O que ESTA REQ já tinha e a duplicata não

A medição dos sítios, que é melhor que a que refiz hoje:

- **Sítio 1** — `validator_traceid.go:262-277`: o laço percorre `reqIndex` e **nunca consulta
  `e.state`**. 🔴 O campo **existe**, é preenchido, e é usado **10 linhas abaixo** por
  `traceid_state_mismatch`.
- **Sítio 2** — `req_has_roadmap`: itera todas as REQs e **não tem o estado disponível**.

⚠️ E esse achado **refinou a ADR**: eu ia recusar `e.state` só por invocar a ADR-2026-09-03 D1.
A razão real, medida, é melhor — **`e.state` é vazio em layout plano** (`validator_traceid.go:77`),
e a regra ficaria inerte no layout deste repositório. Ver **D1-bis**.

## Acceptance Criteria (consolidados — substituem os originais acima)

- [x] **Enumeração real** das regras que decidem "esta REQ deveria ter roadmap?" → **4, não 2**
      (`req_has_roadmap`, `traceid_orphan_req`, `ref_targets_exist`, `req_roadmap_lifecycle`)
- [x] 🔴 **C1 fechada por recorte SEMÂNTICO** (`status:`), **nunca pela pasta da REQ** — teste que
      → medido: REQ `Open` com `req_id` sem roadmap **silencia**; e a Wave 2 provou o recorte semântico colocando uma REQ `Done` **fisicamente** em `req_dir/backlog/` — ela **dispara**, logo manda o `status:`, não a pasta
      falhe se alguém reintroduzir decisão por diretório
- [x] 🔴 **C2 fechada por casamento via vínculo real** (`req:` do roadmap, com
      → medido: par com roadmap tendo `req:` e **sem** `req_id` silencia, em **flat e by_agent**; `req:` com `\` casa igual (`normalizeRefSeparator`)
      `normalizeRefSeparator`), e **não** só fazendo o gerador escrever `req_id:`
- [x] **Braço do passivo:** par já existente, sem `req_id` no roadmap, deixa de disparar **sem
      → medido: o par deixa de disparar **sem alterar arquivo nenhum**
      alterar arquivo nenhum**
- [x] **Contra-braço:** REQ **`Done`** sem roadmap **ainda** dispara — senão a correção virou remoção
      → medido em fixture minha: REQ `Done` sem vínculo em direção nenhuma → **as duas regras disparam**
- [x] `Superseded` e `Closed` **não** disparam (ADR **D2-bis**)
      → medidos: silenciam. ⚠️ Resíduo registrado (A2): `status: "Done "` com aspas **e** espaço final bypassa as duas — `EqualFold` não normaliza. Pré-existente, consistente, fora desta causa
- [x] 🔴 **As regras sob D4 aplicam o mesmo critério** — **SATISFEITO no ML-1E** (2026-09-29):
      `req_has_roadmap` passou a aceitar o vínculo reverso pelo mesmo critério do D3. Auditado em
      fixture própria: braço do achado → as duas silenciam; contra-braço (sem vínculo nenhum) → as
      duas disparam. Histórico do gap: medido na Wave 2, sobre
      o mesmo par (REQ `Done` sem campo `roadmap:`, roadmap apontando para ela), `traceid_orphan_req`
      silencia e `req_has_roadmap` dispara. É o sintoma original sobrevivendo dentro da REQ. **ML-1E.**
      ⚠️ **Correção de escopo:** eram "4 regras" na redação anterior; `ref_targets_exist` **saiu** —
      ela mede integridade referencial, não obrigação de vínculo (ADR, correção da Wave 2)
- [x] **Delta medido** de violações no corpus, antes/depois, com a razão de cada uma que sair
      → `req_has_roadmap`: **173 → 163** warnings, grandfathering **13 → 3** (10 `Superseded` silenciam, D2-bis) e o cutoff **preservado**. `traceid_orphan_req` é **inerte** neste repo (sem `trace_id_field`) — medido em fixture
- [x] `make quality` e **CI** verdes
      → `make quality` **RC=0** verificado pelo arquiteto: 8 chunks, **347 OK, 0 FAIL**. CI: ver PR

## Negative scope (consolidado)

- **Não** remove o cutoff de `req_has_roadmap` — 🔴 a Wave 0 **refutou** que ele ficaria redundante:
  **3** dos 13 grandfathered são `Done` e voltariam a violar sem ele.
- **Não** fecha C2 para os **58 roadmaps** fora do alcance de D3 (32 `req:` stale · 8 vazio/null ·
  18 sem campo). Limite **medido e declarado** na ADR, não omitido.
- **Não** trata o **#273** — **CLOSED**, e a medição separa: lá é algoritmo de match de slug em
  `branch_has_wip_roadmap`, que **já é** consciente de estado. Falsificação nas duas direções:
  corrigir C1/C2 não afeta slug matching, e vice-versa.
- **Não** regenera `.trackfw-baseline.json` de ninguém.
