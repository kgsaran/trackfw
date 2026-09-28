---
status: wip
date: 2026-09-28
req: "docs/req/REQ-2026-09-28-a-direcao-roadmap-para-req-nao-tem-o-tratamento-de-stale-que-a-direcao-inversa-ja-tem.md"
squad: ""
---

# Roadmap: a direcao roadmap para REQ nao tem o tratamento de stale que a direcao inversa ja tem

> Created: 2026-09-28 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-09-28-a-direcao-roadmap-para-req-nao-tem-o-tratamento-de-stale-que-a-direcao-inversa-ja-tem.md -->
REQ: docs/req/REQ-2026-09-28-a-direcao-roadmap-para-req-nao-tem-o-tratamento-de-stale-que-a-direcao-inversa-ja-tem.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ]
- [ ]

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model: medir o que NÃO precisa ser construído
> Dependências: nenhuma. **Bloqueia a implementação.**

**Gates da wave:**

```bash
n=$(git ls-files 'docs/req/*.md' | xargs -n1 basename | sort -u | wc -l | tr -d ' '); t=$(git ls-files 'docs/req/*.md' | wc -l | tr -d ' '); test "$n" = "$t" && echo "Gate W0: $t REQs, $n basenames unicos — zero colisao" || { echo "GATE FALHOU: $t REQs mas $n basenames unicos — ha colisao, o ramo de ambiguidade e alcancavel" >&2; exit 1; }
```

⚠️ Este gate **passa hoje** e é de vigilância: se algum dia houver colisão de basename, o ramo de
ambiguidade deixa de ser inalcançável e a decisão do `ML-0A` precisa ser revista.

### ML-0A — o ramo de ambiguidade é alcançável?
**Owner:** `hades-tf`
**Status:** ✅ Concluído — auditado em 2026-09-28 · 🔴 **refutou duas premissas minhas, e uma é bloqueante**
**Parecer:** `docs/seguranca/2026-09-28-wave0-req-stale-direction.md`

**Veredito resumido:**

1. **Ramo `default:` é alcançável** — `req new --agent hades "x"` + `req new --agent apolo "x"` no mesmo dia produz dois arquivos de basename idêntico sem obstáculo. Medido em projeto temporário com o binário desta branch. O ramo entra na Wave 1, condicionado ao predicado correto (ver ponto 3 abaixo).

2. **AC4 — ampliar para `backlog`**, declarar `done`/`abandoned` fora. Custo medido: 14 violações de artefatos pré-padronização em done/abandoned que ninguém vai corrigir; 0 custo adicional em backlog que não seja acionável. Opção B (backlog+analyzing, sem done/abandoned) fecha o #452 sem ruído histórico.

3. **Bloqueio crítico para Wave 1:** o predicado de entrada do mecanismo existente (`isStaleRoadmapStateRef`) usa `agentNamespaceStateNames` — e para a ref do #452 (`docs/req/hefesto/REQ-X.md`), `base_parent = "hefesto"` não está no mapa. Um mirror literal NÃO corrige o #452. O predicado correto para REQs é `filepath.Dir(ref) != "."`. O Scenario 25 é preservado com este predicado (`REQ-flag-source.md` tem dir=`.`).

4. **Wave 0 gate é cego em `by_agent`**: o glob `docs/req/*.md` não alcança `docs/req/<agente>/*.md`. Passes vacuamente em projeto by_agent puro. Corrigi-lo está fora do escopo desta REQ.

**Critérios de aceite:**
- [x] Veredito escrito sobre alcançabilidade do ramo de ambiguidade, com a medição
- [x] Decisão do AC4 (ampliar × declarar), com o custo de cada lado
- [x] Nenhuma linha de implementação neste ML

## Wave 1 — Espelhar o tratamento que a direção inversa já tem
> Dependências: **Wave 0 auditada**.

Os MLs saem **depois** do `ML-0A` — o escopo depende de o ramo de ambiguidade ser alcançável ou não.

O que já está decidido e independe disso:

- **espelhar** `resolveRoadmapRefStatus`, não escrever um segundo mecanismo (AC1);
- `links to ADR` fica **fora** — ADR não tem dimensão de estado (AC3);
- falsificação nas duas direções: REQ movida → **stale**; REQ apagada → **violação** (AC5).


---

## Auditoria da Wave 0 — 2026-09-28

### 🔴 Refutação 1, e é BLOQUEANTE: "espelhar" não corrigiria o #452

O AC1 diz *"espelhar `resolveRoadmapRefStatus`"*. **Verdade para a estrutura, falso para o
predicado.** O mecanismo existente entra no fallback por:

```go
func isStaleRoadmapStateRef(ref string) bool {
	dir := filepath.Base(filepath.Dir(filepath.ToSlash(ref)))
	return agentNamespaceStateNames[dir]      // wip, backlog, done, …
}
```

Para o ref do #452 — `docs/requisicoes/hefesto/REQ-X.md` — o pai é **`hefesto`**, que **não é
estado**. Verifiquei: `filepath.Base(filepath.Dir(...))` → `hefesto` → `false` → **o fallback nunca
roda**.

🔴 **Um implementador que copiar literalmente entrega código que compila, passa todos os testes
atuais, passa o Cenário 25 da falsificação — e não corrige nada.** É a pior forma de entrega: verde
por toda parte e inerte no sítio que motivou o trabalho.

**O predicado para REQ é outro:** `filepath.Dir(ref) != "."` — deixa `docs/req/hefesto/REQ-X.md`
entrar no fallback e bloqueia `REQ-flag-source.md` (sem diretório), preservando o Cenário 25.

### 🔴 Refutação 2: o ramo de ambiguidade É alcançável

Eu escrevi no AC2 que ele provavelmente seria inalcançável, com base em **0 colisões em 233 REQs**.
A medição derruba: o executor produziu a colisão com **dois comandos normais**.

```
$ trackfw req new "same title test" --agent hades   → docs/req/hades/REQ-2026-09-28-same-title-test.md
$ trackfw req new "same title test" --agent apolo   → docs/req/apolo/REQ-2026-09-28-same-title-test.md
```

`req new` **não verifica colisão entre namespaces de agente**. Não é estado fabricado — são dois
comandos que qualquer consumidor `by_agent` roda.

⚠️ **O erro de método meu:** o corpus que medi é deste repositório, que é **`flat`**. Confundi
*ausência no meu corpus* com *impossibilidade no produto*. O ramo entra.

### ✅ E uma refutação DELE que a medição derruba

Ele afirmou que o gate desta Wave 0 é *"cego em `by_agent`"*, porque `docs/req/*.md` não alcançaria
subpastas. **Medi o contrário:**

```
cenário by_agent com colisão real (hades/REQ-X.md + apolo/REQ-X.md)
  git ls-files 'docs/req/*.md'    → 2 arquivos
  o gate                          → GATE FALHOU: 2 REQs mas 1 basenames unicos
```

🔴 **`git ls-files` trata o padrão como *pathspec*, que casa recursivamente** — diferente do glob de
shell, onde `*` para no separador. A intuição dele vem do shell e é razoável; o instrumento é que se
comporta diferente. **O gate pega a colisão.**

### Decisões aceitas, com o custo medido

**AC4 — ampliar para `backlog` + `analyzing`, declarar `done`/`abandoned` fora.** Medido nos 206
roadmaps em `done`/`abandoned`:

| categoria | n |
|---|---|
| literal resolve | 149 |
| resolveria por basename (viraria *stale*) | 20 |
| não existe em lugar nenhum | **14** |
| sem campo `req:` | 23 |

As **14** são refs legados, de antes da padronização de caminho. **Ninguém vai corrigir um roadmap
`done` para atualizar um campo legado** — emiti-los seria ruído permanente no CI. `backlog` é o
oposto: trabalho futuro, acionável, e é **onde o #452 se manifesta**.

**Achado de mesma causa — `FindRoadmapLinkingREQ` cruza namespaces.** Ao criar a REQ do `apolo`, o
`chainRoadmapForREQ` encontrou o roadmap do **`hades`** por basename e vinculou os dois. Mesma causa,
mesmo mecanismo → **ML desta REQ**, não REQ nova.

---

## Wave 1 — o predicado correto, não o espelho literal
> Dependências: **Wave 0 auditada** ✅

**Gates da wave:**

```bash
grep -qE 'filepath\.Dir\(ref\) != "\."|dirOfRefIsNotDot' internal/validator/validator.go && echo "Gate W1: o predicado de REQ nao e o de roadmap" || { echo "GATE FALHOU: o predicado de REQ ainda nao existe — copiar isStaleRoadmapStateRef NAO corrige o #452" >&2; exit 1; }
```

⚠️ Reprova hoje, de propósito.

### ML-1A — a direção Roadmap → REQ classifica, com o predicado CERTO
**Owner:** `apolo-tf`
**Status:** ⬜ Pendente
**Arquivos:** `internal/validator/validator.go`

🔴 **NÃO copie `isStaleRoadmapStateRef`.** Ela testa se o pai é um **estado**; o caso do #452 tem o
pai igual ao **agente**. O predicado é `filepath.Dir(ref) != "."`.

**Critérios de aceite:**
- [ ] `docs/req/<agente>/REQ-X.md` movido → **stale**, não violação
- [ ] REQ **apagada** → **violação**, como hoje
- [ ] 🔴 **O teste exercita `docs/req/hefesto/REQ-X.md`**, não só `docs/req/wip/REQ-X.md` — o segundo
      passaria com o predicado errado e daria falso verde
- [ ] Cenário 25 do `check-gates-falsify.sh` **continua passando** (ref sem diretório)
- [ ] Ramo de ambiguidade implementado — **é alcançável**, medido na Wave 0
- [ ] A frase da Regra Dura de Reconciliação, por teste novo

### ML-1B — **AC4** — `ref_targets_exist` varre `backlog` e `analyzing`
**Owner:** `apolo-tf`
**Status:** ⬜ Pendente
**Critérios de aceite:**
- [ ] `backlog` e `analyzing` entram; `done`/`abandoned` **declarados fora** no contrato, com as 14
      entradas legadas como razão medida
- [ ] Contagem de warnings antes/depois, escrita

### ML-1C — `FindRoadmapLinkingREQ` não cruza namespace de agente
**Owner:** `apolo-tf`
**Status:** ⬜ Pendente
**Critérios de aceite:**
- [ ] REQ de `apolo` **não** vincula ao roadmap de `hades` de mesmo basename
- [ ] 🔴 Contra-braço: em `flat`, o vínculo por basename **continua funcionando**
