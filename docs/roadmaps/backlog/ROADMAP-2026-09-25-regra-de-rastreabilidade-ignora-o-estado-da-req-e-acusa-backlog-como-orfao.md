---
status: backlog
date: 2026-09-25
req: "REQ-2026-09-25-regra-de-rastreabilidade-ignora-o-estado-da-req-e-acusa-backlog-como-orfao"
squad: ""
---

# Roadmap: regra de rastreabilidade ignora o estado da REQ e acusa backlog como orfao

> Created: 2026-09-25 | Status: backlog

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: 

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ]
- [ ]

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).
2. Threat model — who empties this Wave 0 without breaking any written rule, and how?
3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?
4. Declared residual — what this design accepts not covering.
**Acceptance criteria:**
- [ ] The four sections above answered with evidence, not a one-line assertion
- [ ] No implementation line written for this ML

**Gates da wave:**
```bash
# Wave 0 gate — replace this placeholder with a project-specific check before
# marking ML-0A done. Do not remove the gate; replace its command (AC13).
exit 1  # placeholder gate fails closed until ML-0A replaces it — see docs/cli-parity.md
```

## Wave 1 — <name> (parallel MLs)
> Dependencies: none

### ML-1A — regra de rastreabilidade ignora o estado da REQ e acusa backlog como orfao
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] build passes
- [ ] tests green
- [ ] validate passes

REQ: `docs/req/REQ-2026-09-25-regra-de-rastreabilidade-ignora-o-estado-da-req-e-acusa-backlog-como-orfao.md`

## Diagnóstico

Ver a REQ. Em uma frase: **duas regras do validador decidem sobre rastreabilidade sem consultar o
estado do artefato**, e acusam como órfã uma REQ em `backlog/`, onde não ter roadmap é o estado
correto pelo protocolo que o próprio trackfw recomenda.

🔴 **A superfície não existe no upstream** (`req_dir` flat → `state = ""` → 0 ocorrências aqui). Toda
correção precisa de fixture que construa o layout do consumidor, senão o gate nasce vacuoso.

## Wave 0 — Threat Model
> Dependências: nenhuma. **Bloqueia toda implementação.**

### ML-0A — Enumeração e threat model
**Owner:** `hades-tf`
**Status:** ⬜ Pendente

**Ações:**
1. **Enumeração pela forma**, não pelo nome, nas **duas** superfícies:
   **(a)** quais regras do validador **decidem** sem consultar o estado do artefato?
   **(b)** quais **comandos** movem artefato sem propagar a mudança aos apontadores que o referenciam
   por caminho contendo o estado? (medido: `MoveRoadmap` sincroniza e anuncia; `MoveREQ` não faz nem
   diz — 115 linhas, zero chamadas de sync) 🔴 **Não parar nos dois sítios já medidos** (`traceid_orphan_req`,
   `req_has_roadmap`) — varrer as **32 regras** declaradas via `applyRule`/`applyRuleTagged`.
2. Classificar cada uma em **(a)** decide errado por ignorar o estado · **(b)** consulta e está
   correta · **(c)** o estado é irrelevante.
3. Decidir, com medição, **quais estados isentam**: `backlog` é certo; `abandoned` provavelmente;
   🔴 **`analyzing` é a pergunta aberta** — é o estado em que a REQ já está sendo estudada, e pode ou
   não exigir roadmap.
4. 🔴 **Decidir, com medição, se (a) e (b) são a MESMA causa ou duas.** Elas entraram juntas por
   **mesmo sintoma** — *"o estado mora na pasta e o resto do sistema não acompanha"*. O teste literal
   sugere que são duas (corrigir `MoveREQ` não fecha o #435, e vice-versa), **mas o ônus é de quem
   quer separar**: a separação só vale com a medição escrita.
5. **Threat model:** quem esvazia esta Wave 0 sem quebrar regra escrita? Em particular: uma isenção
   larga demais transforma `orphan_req` num gate que nunca acusa, o que é pior que o falso positivo
   de hoje — o falso positivo ao menos é **visível**.
5. **Falsificação nas duas direções** para cada sítio: o que quebra quando o produto regride, e o que
   quebra quando regride ao contrário.
6. **Residual declarado.**

**Critérios de aceite:**
- [ ] As 6 seções respondidas **com evidência medida**, não asserção de uma linha
- [ ] A enumeração busca **ativamente** além dos 2 sítios conhecidos, e diz onde procurou
- [ ] O veredito sobre `analyzing/` é **medido ou declarado como decisão**, nunca presumido
- [ ] 🔴 Nenhuma linha de implementação escrita neste ML

**Gates da wave:**
```bash
test -f docs/seguranca/2026-09-25-estado-ignorado-em-regras-de-rastreabilidade.md
git diff --quiet "$(git merge-base origin/main HEAD)" HEAD -- internal/validator/
```

## Wave 1 — Implementação
> Dependências: Wave 0 **auditada**. Os MLs serão detalhados a partir da enumeração — escrevê-los
> agora seria particionar um conjunto que ainda não foi medido, que é o erro que esta casa já pagou.
