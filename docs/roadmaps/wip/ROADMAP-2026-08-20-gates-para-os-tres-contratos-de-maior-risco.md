---
status: wip
date: 2026-08-20
req: "docs/req/REQ-2026-08-20-tres-contratos-afirmados-no-cli-parity-sem-gate-cross-cli.md"
adr: ""
squad: "apolo-tf, hades-tf"
---

# Roadmap: gates para os três contratos de maior risco

> Created: 2026-08-20 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-20-tres-contratos-afirmados-no-cli-parity-sem-gate-cross-cli.md`

Primeiro consumo da lista da triagem (42 `gap` + 51 `partial`). Três alvos, escolhidos por risco e
**confirmados por medição** antes de abrir a REQ.

## 🔴 Riscos que valem para todos os MLs

1. **Não afrouxar o gate para caber.** Windsurf e Amazon Q têm formato diferente dos outros seis; se
   o comparador estrutural não serve, **o comparador muda, não o critério**.
2. **Divergência real entre CLIs é achado, não conserto silencioso.** Aconteceu **cinco vezes** na
   semana passada. Registrar e abrir microlote próprio.
3. **Invocação CI-exata:** `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make parity`. Rodar o script direto
   não é a mesma coisa — três rodadas de CI se perderam por isso.
4. **Ao fechar cada um, a anotação da seção vira `gate=`.** O checker de cobertura é bloqueante desde
   o ML-3A da REQ anterior; anotação desatualizada reprova.

---

## Wave 1 — Windsurf e Amazon Q (o mais grave: alegação **falsa**)

### ML-1A — Avaliar o comparador antes de estender
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos:** nenhum de produto — **lote de investigação**, entrega um parecer curto no roadmap.

Os dois gates comparam **estrutura JSON** de 6 CLIs que compartilham forma. Windsurf usa arquivo
único `.windsurf/hooks.json` com `hooks.pre_run_command`; Amazon Q usa agente customizado em
`.amazonq/cli-agents/*.json`. **Provavelmente foi por isso que ficaram de fora.**

**Pergunta a responder com medição, não palpite:** o comparador atual estende para os dois formatos,
ou eles exigem comparador próprio? Se exigem, qual o desenho — e o que se perde em cada opção?

**Critérios de aceite:**
- [ ] Resposta com evidência: forma real dos dois arquivos gerados pelos 3 CLIs, lado a lado
- [ ] Recomendação explícita, com o trade-off
- [ ] **Nenhuma linha de gate escrita** — decidir o desenho antes de codificar é o ponto do lote

### ML-1B — Implementar a cobertura decidida
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` · **Dependência:** ML-1A
**Critérios de aceite:**
- [ ] Windsurf e Amazon Q cobertos, nos 3 CLIs, comparando saídas/artefatos reais
- [ ] Cenário P4 com baseline e detecção
- [ ] A anotação da seção deixa de afirmar cobertura inexistente
- [ ] `make quality` verde

---

## Wave 2 — `branch_has_wip_roadmap` com `done/`

### ML-2A — Cenário cross-CLI com roadmap em `done/`
**Status:** ⬜ Pendente · **Agente:** `apolo-tf`
**Arquivos:** `scripts/check-branch-new-parity.sh` e/ou `check-validate-parity.sh`,
`scripts/check-gates-falsify.sh`, `docs/cli-parity.md`.

Medido: os fixtures dizem literalmente *"wip/ and done/ deliberately left empty"*, e o gate do
`validate` tem **zero** ocorrências da regra. O comportamento que define a `REQ-2026-07-26` nunca foi
exercitado entre os 3 CLIs.

**Critérios de aceite:**
- [ ] Fixture com roadmap correspondente em `done/` e branch de slug igual → **aceita**, nos 3
- [ ] Não-regressão: sem roadmap em lugar nenhum → **recusa**, nos 3
- [ ] Discriminante: roadmap em `done/` com slug **diferente** → recusa
- [ ] Cenário P4 sabotando a aceitação de `done/` e provando gate vermelho
- [ ] `make quality` verde

---

## Wave 3 — `credential_guard_hook_resolvable` cross-CLI

### ML-3A — Estender a prova para Node e Python
**Status:** ⬜ Pendente · **Agente:** `apolo-tf`

O Cenário 47 declara no próprio comentário ser prova black-box da regra **Go**. É o controle que o
`ADR-2026-08-12` aponta como o que resta mitigando o fail-open — com prova em um terço dos runtimes.

**Critérios de aceite:**
- [ ] Regra exercitada nos 3 CLIs, com hook registrado apontando para script ausente
- [ ] Não-regressão: script presente e executável → silêncio, nos 3
- [ ] Falso-positivo dominante coberto: caminho relativo legítimo **não** acusado
- [ ] Cenário P4 com baseline e detecção
- [ ] `make quality` verde

---

## Wave 4 — Barreira

### ML-4A — `hades-tf`: os gates novos provam o que dizem provar?
**Status:** ⬜ Pendente · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-20-revisao-dos-gates-dos-tres-contratos.md`

Foco: o gate do `credential_guard_hook_resolvable` toca o controle central contra fail-open —
provar em 3 runtimes só vale se a prova for a mesma. Avaliar se o gate de Windsurf/Amazon Q compara
o que importa ou só a forma. **Veredito explícito; bloquear é saída legítima.**

---

## Notas
- **Fora de escopo, declarado:** as outras 39 `gap` e 50 `partial`. A lista é priorizável de
  propósito; fechar tudo não é meta.
- Commits e branch são exclusivos do `trackfw_architect`.
