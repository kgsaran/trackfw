---
status: Done
date: 2026-09-28
author: ""
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-09-28-a-direcao-roadmap-para-req-nao-tem-o-tratamento-de-stale-que-a-direcao-inversa-ja-tem.md"
---

# REQ: a direcao roadmap para REQ nao tem o tratamento de stale que a direcao inversa ja tem

> Date: 2026-09-28 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation

Origem: **issue #452**, de consumidor externo. 🔴 **Eu respondi que não era defeito e me retratei**
depois de reproduzir — a retratação está no issue.

### A armadilha de dois lados, reproduzida

Em `roadmap_namespacing: by_agent`, o `req new --agent` grava a REQ na **raiz** do namespace, e o
roadmap pareado **congela esse caminho** no campo `req:`:

```bash
$ trackfw req new "…" --agent hefesto
created docs/requisicoes/hefesto/REQ-….md
✓ created docs/roadmaps/hefesto/backlog/ROADMAP-….md

$ grep '^req:' …/ROADMAP-….md
req: "docs/requisicoes/hefesto/REQ-….md"

$ mv docs/requisicoes/hefesto/REQ-….md docs/requisicoes/hefesto/backlog/
$ trackfw validate
✗ roadmap "ROADMAP-….md" links to REQ "docs/requisicoes/hefesto/REQ-….md" which does not exist
```

**Não há estado em que as duas coisas estejam certas:** deixar a REQ onde o `req new` a pôs viola a
`ADR-036` do consumidor (*"pasta = fonte de verdade do estado"*); movê-la quebra o `validate`.

⚠️ **A minha primeira sonda NÃO reproduziu** — a regra `ref_targets_exist` só varre `wip` e `blocked`
(`validator.go:3163`), e o roadmap da sonda estava em `backlog/`. Se eu tivesse parado ali, teria
confirmado o veredito errado **com uma sonda como prova**. Isso é um segundo achado: **roadmap em
`backlog/` com `req:` quebrado passa silencioso**, e essa cobertura parcial não está declarada.

### 🔴 O achado que define o escopo: é ASSIMETRIA, não ausência

O tratamento correto **já existe no produto** — na direção **REQ → Roadmap**:

```go
// validator.go:3197
resolved, stale := resolveRoadmapRefStatus(cfg, ref)
  case 0            → "does not exist"                                   (violação real)
  case 1 && stale   → "but the file is now in <estado>/ (stale state path)"
  case 1            → silêncio
  default           → "is ambiguous: found in multiple states"
```

E `resolveRoadmapRefStatus` faz exatamente o que falta: tenta o **caminho literal**, cai para
**fallback por basename**, e **classifica** o resultado.

A direção **Roadmap → REQ** não recebeu nada disso:

```go
// validator.go:3172
if !referenceExists(ref) { "links to REQ %q which does not exist" }
```

`referenceExists` é `os.Stat` no caminho literal. Ponto.

🔴 **O comentário no próprio código já previu o dano da outra direção:** *"o serve casa `edge.To` pelo
literal e desenharia aresta órfã para o mesmo vínculo se calássemos aqui"*. Alguém sabia que
silenciar sem classificar produz grafo mentiroso — e aplicou **numa direção só**.

### Por que classificar, e não só resolver por basename

A regra hoje confunde **dois estados diferentes**:

| estado | hoje | correto |
|---|---|---|
| a REQ **não existe** em lugar nenhum | violação | **violação** — link roto |
| a REQ existe, em **outro caminho** | violação | **stale** — vínculo desatualizado, não roto |

Resolver tudo por basename tiraria a armadilha **e jogaria fora** a detecção do link genuinamente
roto. Classificar preserva as duas.

### Medição do arquiteto — 2026-09-28

```
REQs no corpus          233   ·  basenames únicos  233   ·  colisões  0
roadmaps no corpus      230   ·  basenames únicos  230   ·  colisões  0
```

O produto **já resolve por basename** em dois sítios (`roadmap.go:804` e `:1202`) — o fallback não é
invenção, é o padrão da casa.

## Acceptance Criteria

- [ ] **AC1** — a direção **Roadmap → REQ** classifica como a inversa já faz: *inexistente* ·
      *stale* · *(ambíguo, se alcançável)*. **Espelhar `resolveRoadmapRefStatus`**, não escrever um
      segundo mecanismo
- [x] **AC2 — medido, e a minha premissa CAIU.** Eu supus que o ramo fosse inalcançável a partir de
      *0 colisões em 233 REQs*. A Wave 0 produziu a colisão com **dois comandos normais**
      (`req new --agent hades` e `--agent apolo`, mesmo título): `req new` **não verifica colisão
      entre namespaces**. 🔴 Confundi *ausência no meu corpus* — que é `flat` — com *impossibilidade
      no produto*. **O ramo entra.**

- [ ] **AC7 (2026-09-28) — 🔴 o predicado NÃO é o do espelho.** `isStaleRoadmapStateRef` testa se o
      pai do ref é um **estado**; no caso do #452 o pai é o **agente** (`hefesto`), e o fallback
      nunca roda. Copiar `resolveRoadmapRefStatus` entrega código que compila, passa todos os testes
      e **não corrige nada**. O predicado para REQ é `filepath.Dir(ref) != "."`

- [ ] **AC8 (2026-09-28)** — `FindRoadmapLinkingREQ` **cruza namespace de agente**: a REQ do `apolo`
      foi vinculada ao roadmap do `hades` de mesmo basename. Mesma causa, mesmo mecanismo → entra
      aqui, não vira REQ nova
- [ ] **AC3** — `links to ADR` (`validator.go:3192`) fica **fora**: ADR não tem dimensão de estado,
      logo não sofre deste defeito. Declarado, não esquecido
- [ ] **AC4 — a cobertura parcial de `ref_targets_exist` é decidida:** ela só varre `wip` e
      `blocked`. Roadmap em `backlog/` com `req:` roto **passa silencioso**. Ampliar ou **declarar**
- [ ] **AC5 — falsificação nas duas direções:** REQ movida → **stale**, não violação; REQ apagada →
      **violação**, como hoje
- [ ] **AC6** — `make quality` e **CI** verdes

## Negative scope — o que esta REQ NÃO faz

- **Não** muda onde o `req new` grava a REQ. A `ADR-2026-09-03` D1 decide que **REQ não tem dimensão
  de estado**, e isso continua valendo — o conflito com a `ADR-036` do consumidor é dele para
  resolver, e está respondido no issue.
- **Não** cria `validate --fix`. Conserto sob demanda não ajuda quem vê vermelho no CI sem saber que
  é falso: **a regra tem de classificar certo antes** de qualquer comando de conserto existir.
- **Não** toca o `serve` nem o desenho do grafo.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/done/ROADMAP-2026-09-28-a-direcao-roadmap-para-req-nao-tem-o-tratamento-de-stale-que-a-direcao-inversa-ja-tem.md
