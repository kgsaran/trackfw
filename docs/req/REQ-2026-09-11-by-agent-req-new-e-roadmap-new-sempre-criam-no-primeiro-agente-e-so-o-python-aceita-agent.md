---
status: Superseded
date: 2026-09-11
author: ""
adr: ""
roadmap: "docs/roadmaps/abandoned/ROADMAP-2026-09-11-by-agent-req-new-e-roadmap-new-sempre-criam-no-primeiro-agente-e-so-o-python-aceita-agent.md"
---

# REQ: by_agent: req new e roadmap new sempre criam no primeiro agente, e so o Python aceita --agent

> Date: 2026-09-11 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation
<!-- Why is this requirement needed? What problem does it solve? -->

## Acceptance Criteria
- [ ]
- [ ]

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: `docs/roadmaps/abandoned/ROADMAP-2026-09-11-by-agent-req-new-e-roadmap-new-sempre-criam-no-primeiro-agente-e-so-o-python-aceita-agent.md`
## Motivation

Issue **#320** do consumidor externo, medido nos 3 CLIs (v7.5.1) num projeto descartável com
`agents: [alpha, beta]`:

```
                                        Go      Node    Python
req new "x"                             alpha/  alpha/  alpha/
roadmap new --req .../beta/REQ-….md     alpha/  alpha/  alpha/   <- ignora o agente da REQ
req new … --agent beta                  erro    erro    erro
roadmap new … --agent beta              erro    erro    OK       <- so o Python tem a flag
```

**São duas coisas, e o relator já as separou.**

### 1. A lacuna, nos 3 runtimes

Sem flag, o artefato cai em `agents[0]`. E o `roadmap new --req` **não usa o agente da REQ que
recebe**: com a REQ em `beta/`, o roadmap nasce em `alpha/`.

Sitios derivados por ele:

```
Go       internal/generators/roadmap.go:110 (fallback) e :141
Node     npm/src/generators/roadmap.js:74
Python   pypi/trackfw/generators/roadmap.py:185  ·  req.py:207
```

O sitio do `req new` do Go ele **nao localizou** — declarou isso em vez de presumir. **Derivar.**

### 2. A divergencia de paridade

O Python define `--agent` no `roadmap new` (`pypi/trackfw/commands/roadmap.py:211`); Go e Node
recusam. **O mesmo comando funciona num runtime e falha nos outros dois.**

Nota de merito do relator: **a recusa e barulhenta nos tres** — nenhum aceita a flag e a ignora em
silencio, que seria o pior caso.

## 🔴 A correcao tem precedente interno — nao inventar desenho

> *"O codigo **ja sabe** derivar o agente de um caminho: o `roadmap move` faz isso
> (`internal/generators/roadmap.go:442`, `npm/src/generators/roadmap.js:267-268`).
> **So o `new` nao usa.**"*

E o mesmo padrao do `serve`, onde os ramos Darwin/Linux do Python ja estavam corretos e serviram de
referencia. **Trazer o `new` para a forma que o `move` ja tem**, em vez de escrever uma terceira.

## Como apareceu — e por que nao tinhamos visto

No fork dele, com 3 agentes: `req new` criou em `apolo/`, ele moveu para `claude/`, e o
`roadmap new --req` pos o roadmap em `apolo/`.

🔴 **Nos temos ZERO testes com 2+ agentes** — medido em 2026-09-11. E um projeto com dois agentes e
configuracao **trivial**, nao caso exotico. O job `consumer-smoke-by-agent` (PR #326) existe
exatamente por isso, e **nasce vermelho detectando este defeito**.

## Acceptance Criteria

- [ ] **AC1** — sem flag, o artefato nasce no agente **certo**, nao em `agents[0]`. O que e "certo"
      quando nao ha contexto **e decisao a registrar**: primeiro da lista, agente do branch, ou erro
      pedindo a flag. **Escolher e justificar.**
- [ ] **AC2** — `roadmap new --req <caminho>` **herda o agente da REQ**, usando o mesmo mecanismo do
      `roadmap move`. 🔴 Nao escrever derivacao nova.
- [ ] **AC3** — `--agent` existe e funciona nos **3 CLIs**, em `req new` e `roadmap new`. A
      divergencia atual (so Python, so `roadmap`) some.
- [ ] **AC4** — sitio do `req new` do Go **derivado**, nao presumido. O relator declarou que nao o
      localizou; nos localizamos.
- [ ] **AC5** — falsificacao nas duas direcoes, com `agents: [alpha, beta]`: artefato vai para o
      agente pedido; **e** vai para o correto quando nenhum e pedido. 🔴 Contra-braco obrigatorio.
- [ ] **AC6** — 🔴 **o `consumer-smoke-by-agent` passa a VERDE**, e o `continue-on-error: true` dele
      **e removido** no mesmo PR. Estava declarado como temporario no PR #326; remover e parte de
      fechar esta REQ, nao "alguem lembra depois".
- [ ] **AC7** — `flat` continua funcionando. A correcao nao pode quebrar quem nao usa `by_agent`.

## Fora desta REQ

Mudar o layout padrao ou o `roadmap_namespacing`. Migrar artefatos ja existentes.

---

## 🔴 SUPERSEDED no mesmo dia — 2026-09-11

**Esta REQ nao deveria ter sido aberta.** O defeito ja tinha REQ, desde **2026-08-29**:

```
docs/req/REQ-2026-08-29-agents-install-nao-registra-o-agente-na-governanca-e-roadmap-new-em-by-agent-escreve-sempre-no-primeiro-da-lista.md
```

Ela ja nomeava a causa (`cfg.Agents[0]` em silencio), **ja tinha o mecanismo decidido pelo KG** — um
namespace usa, varios sem `--agent` falham nomeando — e a triagem de `2026-09-05` (linha 7) a marcava
**AINDA VALIDA (verificado)**, citando **"AC5 nao implementado"**.

Todo o conteudo do #320 foi movido para la como **AC10-AC15**, pela `Regra Dura de Causa Raiz`:
**mesma causa ⇒ mesma REQ ⇒ mesmo PR.**

### Como o engano aconteceu — e o que o evitou

Li o issue, medi o defeito, e parti para REQ nova sem procurar REQ existente com a mesma causa.
🔴 **E exatamente o padrao que a regra existe para impedir** — e ele produz backlog que cresce por
construcao, com o defeito vivo por tras da aparencia tranquilizadora de estar "registrado".

O que pegou foi a varredura que precedeu o desenho: ao procurar se `agents[0]` estava **documentado**
como default, apareceu `docs/portabilidade/2026-09-05-triagem-das-reqs-abertas.md` linha 7. **A REQ
duplicada durou minutos, e nao um ciclo, porque a medicao veio antes da decisao.**
