---
status: wip
date: 2026-08-23
req: "docs/req/REQ-2026-08-23-agents-update-de-escopo-global-perde-o-pin-de-modelo-porque-le-agent-models-do-cwd.md"
adr: "docs/adr/ADR-2026-08-23-config-global-do-trackfw-como-fonte-do-pin-de-modelo-para-instalacao-de-escopo-global.md"
squad: "hades-tf, apolo-tf"
---

# Roadmap: `agents update` de escopo global resolve o pin de modelo da config global

> Created: 2026-08-23 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-23-agents-update-de-escopo-global-perde-o-pin-de-modelo-porque-le-agent-models-do-cwd.md`
ADR: `docs/adr/ADR-2026-08-23-config-global-do-trackfw-como-fonte-do-pin-de-modelo-para-instalacao-de-escopo-global.md`

Artefato de **escopo global** é renderizado com a config do **diretório de invocação**. Rodar
`agents update` de outro lugar reverte o pin de modelo de todos os agentes, em silêncio. Impacto
diário de cota, reportado por KG.

**Este é o primeiro roadmap gerado com Wave 0 pelo próprio harness** (PR #206).

## Acceptance Criteria

- [ ] AC1–AC10 da REQ, integralmente
- [ ] `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` exit 0 (exit code medido)
- [ ] `./bin/trackfw validate` sem violations novas

## O que já está medido — não repita este trabalho

```
config.Load()  internal/config/config.go:125   le "trackfw.yaml" do CWD, sem fallback
~/.trackfw/    identity.json, integrations-manifest.json, scripts/   SEM trackfw.yaml
render         internal/integrations/render.go:205   compoe corretamente quando recebe agentModels
plano          internal/integrations/plan.go:69      Render(..., request.AgentModels)
chamadores     internal/commands/integrations_flags.go:228 e :339   AgentModels: config.Load().AgentModels

agents update --force --scope global --targets claude, rodado DE DENTRO deste repo:
  antes:  model: opus / model: sonnet
  depois: model: claude-opus-5 / model: claude-sonnet-4-6
```

**O código funciona. O defeito é de onde a config vem.**

---

## Wave 0 — Modelo de ameaça

> Dependências: nenhuma. **Bloqueia toda a implementação.**

### ML-0A — Modelo de ameaça da config global de modelo
**Status:** ⬜ Pendente · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-23-modelo-de-ameaca-da-config-global-de-modelo.md`

**Ações:**

1. **Completude de enumeração** — a lista de superfícies está completa? Ela é: `config.Load()`, os
   chamadores que montam `PlanRequest`, `agents models`, e os equivalentes Node/Python. **Não se
   limite a essa lista** — busque no repositório por outros pontos que leiam `trackfw.yaml` ou que
   montem `AgentModels`, e por outros comandos que escrevem em escopo global (`update harness`,
   `skills`, `integrations`). Cada um sofre do mesmo defeito?
2. **Modelo de ameaça** — o adversário aqui é o **ambiente**, não um atacante: cwd errado, config nos
   dois lugares com valores diferentes, config global corrompida, `HOME` não definido, `~/.trackfw/`
   sem permissão de leitura. Para cada um: o que o usuário vê?
3. **Alvos de falsificação nas duas direções** — (a) global voltando a ler do cwd; (b) projeto
   passando a ler do global. Onde cada sabotagem entra, e qual gate acusa.
4. **Residual declarado** — em especial: **a decisão de precedência do AC2 ainda não está tomada**.
   Recomende uma, com o motivo, e diga o que ela deixa passar. Considere que este repositório tem
   `agent_models` no `trackfw.yaml` de projeto **hoje**.

**Critérios de aceite:**
- [ ] As quatro seções com evidência medida, não asserção de uma linha
- [ ] Recomendação explícita de precedência para o AC2, com o que ela custa
- [ ] Nenhuma linha de implementação escrita

**Gates da wave:**
```bash
test -f docs/seguranca/2026-08-23-modelo-de-ameaca-da-config-global-de-modelo.md
grep -q "Completude de enumera" docs/seguranca/2026-08-23-modelo-de-ameaca-da-config-global-de-modelo.md
grep -q "Residual declarado" docs/seguranca/2026-08-23-modelo-de-ameaca-da-config-global-de-modelo.md
```

---

## Wave 1 — Resolução por escopo

> Dependências: ML-0A auditado. **ML único:** os 3 stacks precisam sair byte-idênticos.

### ML-1A — Config global como fonte para escopo global, nos 3 CLIs
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-0A

**Escopo:** carregar `agent_models` de `~/.trackfw/trackfw.yaml` quando o alvo é escopo global;
manter projeto lendo do projeto; aviso visível quando global renderiza sem pin resolvido; `agents
models` mostrando a origem. Precedência conforme a decisão do ML-0A.

**Critérios de aceite:** AC1–AC7 da REQ · `make quality` exit 0 medido

---

## Wave 2 — Gate

> Dependências: ML-1A auditado.

### ML-2A — Paridade e falsificação nas duas direções
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-1A

**Critérios de aceite:** AC8, AC9, AC10 da REQ

---

## Wave 3 — Barreira

### ML-3A — Reverificação
**Status:** ⬜ Pendente · **Agente:** `hades-tf` (`subagent_type: hades-tf`)

Quem escreveu a Wave 0 verifica se a implementação honra o que ela enumerou. **Veredito explícito.**

---

## Notas
- **Fora de escopo:** tudo listado no *Negative scope* da REQ.
- Commits, branch e PR são exclusivos do `trackfw_architect`.
