---
status: Done
date: 2026-08-30
author: "trackfw_architect (Zeus)"
adr: ""
roadmap: ""
---

# REQ: `req new` grava flat, `resolveREQFiles` procura namespaced por estado — e as regras de referência ficam vácuas em `by_agent`

> Date: 2026-08-30 | Status: Done

## Motivation

Reportado por **@lourivalgarciajunior** na issue #216 (décimo achado), ao migrar um segundo
repositório consumidor de 2.12.4 para 7.3.0 e ver o `validate` sair de 0 para 18 violações — nenhuma
nova, apenas nunca olhadas.

Reproduzido por mim **na `main` de hoje**, depois do resolvedor canônico do PR #218:

```
mesma REQ, mesma referência quebrada, só mudando o layout:
flat      2 violações   ← "links to ADR ... which does not exist"
by_agent  0             ← a regra não encontra nada para checar
```

**A causa é mais grave do que o relato.** Ele suspeitou que `resolveREQFiles` não enxergasse o
subdiretório do agente. Medi, e o problema é que **o escritor e o leitor discordam sobre o layout**:

| | onde grava / procura |
|---|---|
| `req new` em `by_agent` | `req_dir/REQ-x.md` — **flat**, sem namespace |
| `resolveREQFiles` em `by_agent` | `req_dir/<agente>/<estado>/*.md` — **três níveis** |
| o repositório consumidor do relato | `docs/requisições/<agente>/*.md` — namespaced, **sem estado** |

**Nenhum dos três concorda.** REQ não é particionada por estado — `backlog`/`wip`/`done` é conceito
de roadmap. O `resolveREQFiles` procura numa árvore que REQ nenhuma usa, e por isso devolve vazio.

**Consequência:** em todo projeto `by_agent`, as regras que consomem REQ ficam **vácuas**. O relato
traz o caso concreto: quatro REQs apontando para `docs/roadmaps/claude/wip/` — diretório que não
existe mais porque os roadmaps foram para `done/` — e o `validate` reportando zero violações de link.

*Uma regra de validação que silenciosamente não faz nada num layout suportado é pior que a ausência
dela: dá confiança onde não há.*

**É a terceira vez que o mesmo padrão aparece em três dias:** gerador e verificador discordando do
contrato. Antes foi o cabeçalho de aceite (inglês contra português) e o status (`pending` contra
emoji), corrigidos no PR #217. A causa comum é não haver **teste de ciclo fechado** — criar pelo
gerador e ler pelo verificador — para cada artefato.

## Acceptance Criteria

- [ ] **AC1** — Definir e documentar o **layout canônico de REQ em `by_agent`**. Decisão de ADR: `req_dir/<agente>/*.md` é o que o relato encontra em campo e o que a namespaced-sem-estado sugere.
- [ ] **AC2** — `req new` e `resolveREQFiles` passam a concordar. Um único ponto decide o caminho; ambos o consomem.
- [ ] **AC3** — **Compatibilidade:** REQ já existente em `req_dir/*.md` num projeto `by_agent` continua encontrada. Não migrar arquivo de ninguém.
- [ ] **AC4** — A fixture do relato produz **2 violações** em `by_agent`, iguais às de `flat`.
- [ ] **AC5** — **Teste de ciclo fechado por artefato**: `req new` → `validate` enxerga; `adr new` → idem; `note new` → idem. Nos 3 CLIs, em `flat` **e** `by_agent`. É a AC que impede a quarta ocorrência.
- [ ] **AC6** — Varrer as demais regras que consomem REQ e confirmar que nenhuma fica vácua em `by_agent`.
- [ ] **AC7** — **Usabilidade, do mesmo relato:** o template do `req new` gera `ADR:` e `Roadmap:` no corpo, e o validator lê **só** o frontmatter. Ou o corpo vira comentário dizendo que é prosa, ou o validator lê os dois. Decidir e implementar.
- [ ] **AC8** — Paridade nos 3; gate falsificável nas duas direções.
- [ ] **AC9** — `make quality` exit 0 e CI verde.

## Negative Scope

- **Não** migrar `req_dir` de projeto nenhum automaticamente.
- **Não** introduzir partição por estado em REQ — o problema é o leitor supor que existe.
- **Não** reabrir o resolvedor de namespace do PR #218, que está correto; o defeito é o **layout**
  que o `resolveREQFiles` monta em cima dele.

## Linked ADR
<!-- Necessário para AC1: layout canônico de REQ em by_agent. -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap:
