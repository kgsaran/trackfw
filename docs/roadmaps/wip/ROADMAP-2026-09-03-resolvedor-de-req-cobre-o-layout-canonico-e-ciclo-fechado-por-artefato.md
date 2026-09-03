---
status: wip
date: 2026-09-03
squad: apolo-tf
req: "docs/req/REQ-2026-08-30-req-new-grava-flat-mas-resolvereqfiles-procura-namespaced-por-estado-e-as-regras-de-referencia-ficam-vacuas-em-by-agent.md"
adr: "docs/adr/ADR-2026-09-03-layout-canonico-de-req-em-by-agent-e-o-invariante-de-que-req-nao-tem-dimensao-de-estado.md"
---

# Roadmap: Resolvedor de REQ cobre o layout canônico, e ciclo fechado por artefato

> Criado em: 2026-09-03 | Status: wip

## Context

REQ: docs/req/REQ-2026-08-30-req-new-grava-flat-mas-resolvereqfiles-procura-namespaced-por-estado-e-as-regras-de-referencia-ficam-vacuas-em-by-agent.md

ADR: docs/adr/ADR-2026-09-03-layout-canonico-de-req-em-by-agent-e-o-invariante-de-que-req-nao-tem-dimensao-de-estado.md

## Diagnóstico

Em todo projeto `by_agent`, as regras que consomem REQ ficam **vácuas** — passam sempre, sem olhar
nada. Medido: mesma REQ, mesma referência quebrada, `flat` → **2 violações**, `by_agent` → **0**.

O escritor grava flat, o leitor procura `<agente>/<estado>/`, e o campo usa `<agente>/`. **Nenhum dos
três concorda.**

🔴 **Correção de custo, feita na ADR:** a leitura de backlog afirmou que "a lógica correta já existe
— é fiação, não lógica nova", apontando `listREQFiles` (`internal/generators/req.go:119-152`).
**Verificado e parcialmente falso:** ele cobre flat, por-estado e `<agente>/<estado>`, e **não cobre
`<agente>/*.md`** — justamente o canônico da ADR. **Falta um caso.**

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Acceptance Criteria

- [ ] O resolvedor de leitura cobre os 4 layouts, em união, nos 3 CLIs
- [ ] `req new` grava no canônico da ADR (`req_dir/<agente>/*.md` em `by_agent`)
- [ ] 🔴 Zero regras enxergando zero REQs em `by_agent` — forma mensurável, não impressão
- [ ] 🔴 Compatibilidade: REQ em qualquer layout continua encontrada; nenhum arquivo migrado
- [ ] Ciclo fechado por artefato nos 3 CLIs, em `flat` **e** `by_agent`
- [ ] `make quality` verde e CI verde

## Wave 1 — O resolvedor
> Dependências: nenhuma.

### ML-1A — Resolvedor de REQ em união, e `req new` no canônico
**Status:** ⬜ Pendente
**Agente:** `apolo-tf`
**Files affected — os 3 stacks, regra dura de paridade sem exceção:**
`internal/validator/validator.go` (`resolveREQFiles`), `internal/generators/req.go` (`listREQFiles`,
`NewREQ`), `npm/src/validator/index.js`, `npm/src/generators/req.js`,
`pypi/trackfw/validator.py`, `pypi/trackfw/generators/req.py`, `docs/cli-parity.md`

⚠️ **`npm/src/validator/index.js` é classificado como BINÁRIO pelo `file`** — `grep` sem `-a` o pula
**em silêncio**. Duas REQs deste repositório têm premissa falsa por causa disso. Use sempre `grep -a`.

**Ações:**
1. Leitura em **união** dos 4 layouts (ADR D3). O caso ausente é `req_dir/<agente>/*.md`.
2. `req new` grava no canônico (ADR D2). Em `flat`, nada muda.
3. 🔴 **Um único ponto decide o caminho, consumido pelos dois lados** (ADR D4). Se hoje há duas
   noções de layout no mesmo runtime, unificar é parte do ML — é a causa das três ocorrências.

🔴 **`ResolveAgentNamespaces` já lê nome de agente do disco** e há nota registrando que
metacaracteres de glob no nome corrompiam contagem em silêncio (`ListMDFiles` em vez de `Glob`).
Ao acrescentar o 4º caso, **não reintroduza `Glob` sobre nome vindo do disco.**

**Critérios de aceite:**
- [ ] A fixture do relato produz **2 violações** em `by_agent`, iguais às de `flat`
- [ ] 🔴 **Forma mensurável do "não fica mais vácuo":** contar, num projeto `by_agent`, quantas
      regras enxergam **zero** REQs. Tem de ser **zero regras**. Lista a confirmar, não rederivar:
      `ref_targets_exist`, `req_has_adr`, `req_has_roadmap`, `blocked_by_draft_adr`,
      `adr_accepted_when_req_done`, traceid
- [ ] 🔴 **Compatibilidade falsificada:** REQ em `req_dir/*.md` num projeto `by_agent` **continua
      encontrada**. Nenhum arquivo movido — `git status` limpo quanto a renomeações
- [ ] 🔴 **Falsificação na direção oposta:** removendo o 4º caso do resolvedor, a fixture volta a
      dar **0 violações**. Um teste que passa nas duas árvores não mede nada
- [ ] Paridade: os 3 CLIs dão o **mesmo** número de violações sobre a mesma fixture
- [ ] `make quality` verde

**Comandos de validação:** `make quality`, e a fixture do relato executada nos 3 CLIs.

## Wave 2 — A rede que impede a quarta ocorrência
> Dependências: Wave 1.

### ML-2A — Teste de ciclo fechado por artefato
**Status:** ⬜ Pendente
**Agente:** `artemis-tf`

**Por que é microlote próprio, com barreira própria:** é a AC que impede a **quarta** ocorrência do
padrão *gerador e verificador discordando do contrato* — antes foram o cabeçalho de aceite e o
vocabulário de status. Como caixinha dentro do ML de correção, seria a primeira coisa cortada sob
pressão de escopo.

**Ações:** para cada artefato — `req new`, `adr new`, `note new` — criar pelo **gerador** e provar
que o **verificador enxerga**. Nos **3 CLIs**, em `flat` **e** `by_agent`. Mínimo 6 combinações por
artefato.

**Critérios de aceite:**
- [ ] Ciclo fechado verde para os 3 artefatos × 3 CLIs × 2 layouts
- [ ] 🔴 **Falsificação:** sabotando o resolvedor, o ciclo fechado **reprova**. Prove as duas direções
- [ ] 🔴 O teste roda o **CLI**, não o módulo com mock — o defeito do `context` do Node sobreviveu
      desde a origem exatamente por o teste não executar o binário
- [ ] Cenário em `scripts/check-gates-falsify.sh` se virar gate
- [ ] `make quality` verde

## Verificação que só o CI fecha

CI verde nos 3 runtimes. E a prova de campo é o repositório consumidor do relato voltar a reportar as
18 violações que existiam e nunca foram olhadas.

## Barreira final

`hefesto-tf` e `hades-tf` — há mudança de resolução de caminho a partir de nome de diretório vindo do
disco, então o Hades entra. Auditoria do arquiteto e `barrier`.
