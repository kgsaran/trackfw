---
status: Open
date: 2026-09-25
author: "trackfw_architect"
adr: ""
roadmap: "docs/roadmaps/backlog/ROADMAP-2026-09-25-regra-de-rastreabilidade-ignora-o-estado-da-req-e-acusa-backlog-como-orfao.md"
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

- [ ] **Enumeração real** das regras do validador que decidem sem consultar o estado do artefato,
      classificadas em: **(a)** decide errado por ignorar o estado → defeito · **(b)** consulta o
      estado e está correta · **(c)** o estado é irrelevante para a regra. 🔴 **Não parar nos dois
      sítios já medidos**
- [ ] Todo sítio **(a)** corrigido, com a lista de estados isentos **declarada em um só lugar** — não
      espalhada por regra
- [ ] 🔴 **Fixture que constrói o layout do consumidor** (`roadmap_namespacing: by_agent`, REQ dentro
      de pasta de estado). Sem ela o gate é vacuoso: o upstream é `flat` e não exercita a superfície
- [ ] Falsificação nas **duas** direções: REQ em `backlog/` sem roadmap **não** acusa; REQ em `wip/`
      sem roadmap **continua** acusando
- [ ] O consumidor consegue **remover as 14 entradas** de `orphan_req` do `.trackfw-baseline.json` sem
      que o `trackfw-chain` fique vermelho
- [ ] `make quality` e **CI** verdes


## 🔴 Ampliação (2026-09-25): o issue #439 entra aqui — mesmo sintoma, e a medição decide a causa

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

### Critérios de aceite acrescentados

- [ ] **Enumeração dos apontadores:** que campos referenciam artefato por caminho que **contém o
      estado**? (`roadmap:` na REQ, `req:` no roadmap, `adr:`, `Roadmap:`/`REQ:` de corpo, …) — e quais
      comandos os movem
- [ ] 🔴 **A simetria é fechada ou a assimetria é declarada.** Se `req move` não for sincronizar, ele
      **avisa** — o que não pode continuar é *"um anuncia, o outro cala"*
- [ ] Falsificação nas duas direções: mover a REQ **atualiza** quem aponta para ela; e o comando
      **não** reescreve apontador que não era dela
- [ ] O consumidor consegue remover as **78 entradas de `stale state path`** do baseline sem o
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
