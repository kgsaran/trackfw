---
status: wip
date: 2026-10-24
req: "docs/req/REQ-2026-10-24-update-regenera-o-trackfw-gate-com-gatilho-pull-request-e-nao-ha-como-escolher-gatilho-manual.md"
squad: "hades-tf, apolo-tf"
---

# Roadmap: gatilho configurável dos workflows gerados (ci_trigger)

> Created: 2026-10-24 | Status: wip

## Context
REQ: docs/req/REQ-2026-10-24-update-regenera-o-trackfw-gate-com-gatilho-pull-request-e-nao-ha-como-escolher-gatilho-manual.md
Issue: #607. ADR-2026-08-28, adendo de 2026-10-24.

## Acceptance Criteria
- [ ] AC1–AC5 da REQ
- [ ] `make quality` e o CI verdes

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model do gatilho configurável
**Status:** ✅ Concluído
**Agente:** hades-tf
**Files affected:** `docs/seguranca/2026-10-24-wave0-ci-trigger.md`
**Actions:**
1. **Enumeração fechada:** procure por grep todo sítio que emite, compara ou valida um workflow gerado:
   - `pull_request`, `merge_requests` e `on:` em `internal/`;
   - os três templates, o scaffold doctor e as regras do validate sobre o workflow de governança (ADR-2026-09-29);
   - os testes que fixam os templates byte a byte.

   Cruze todas as dimensões: template × produtor/consumidor × GitHub/GitLab × `init`/`update`/`discover` × doctor/validate.
2. **Ameaças:**
   - `ci_trigger: manual` desliga o gate de PR em silêncio: quem vê isso, e o `validate` precisa avisar?;
   - um valor com YAML injetado no template;
   - um valor desconhecido;
   - a regra de "um workflow por projeto" (ADR-2026-09-29) com o workflow manual;
   - `required_status_checks` apontando para um job que nunca roda.
3. **Alvos de falsificação** nos dois sentidos e **resíduo declarado**.
**Acceptance criteria:**
- [x] A matriz com as fontes (arquivo:linha)
- [x] As ameaças respondidas com evidência
- [x] Nenhuma linha de produto
**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-24-wave0-ci-trigger.md
git diff --quiet origin/main -- internal/
```

## Wave 1 — Implementação
> Dependencies: Wave 0 auditada. Detalhada depois da Wave 0.

### ML-1A — `ci_trigger` nos três templates, no doctor e no validate
**Status:** ✅ Concluído
**Agente:** apolo-tf
**Files affected:** `internal/config/config.go`, `internal/config/config_ci_trigger_test.go`, `internal/generators/scaffold.go`, `internal/generators/scaffold_doctor.go`, `internal/generators/update.go`, `internal/generators/claudemd.go`, `internal/generators/ci_workflow_version_pin_test.go`, `internal/generators/discover_workflow_trigger_test.go`, `internal/generators/ci_trigger_test.go`, `internal/generators/testdata/ci-trigger/`, `internal/discover/discover.go`, `internal/discover/discover_test.go`, `internal/commands/init.go`, `internal/validator/validator.go`, `internal/validator/validator_ci_trigger_test.go`, `docs/cli-parity.md`
**Contrato (arquiteto, a partir da Wave 0, `docs/seguranca/2026-10-24-wave0-ci-trigger.md` §Z):**
1. **Chave:** a `ci_trigger` fica no mesmo bloco de `ci`, como `cfg.Update.CITrigger`. Os valores aceitos são `pull_request`,
   que é o padrão quando a chave está ausente, e `manual`.
2. **Templates sem interpolação (T2):** cada função faz um `switch` sobre o valor e devolve templates completos,
   pré-escritos. O valor nunca é concatenado no YAML.
   - **GitHub** (gate e discover): `manual` vira `on:\n  workflow_dispatch:`, sem inputs.
   - **GitLab:** `manual` troca `only: - merge_requests` por `rules:\n    - if: '$CI_PIPELINE_SOURCE == "web"'`, que é o
     equivalente do `workflow_dispatch`: pipeline disparado à mão pela interface.
   - Fora o gatilho, todo o resto do workflow fica byte a byte igual: job id, passos, pin e fetch do produtor.
3. **Os três builders** recebem o gatilho. `BuildDiscoverGitHubActionsWorkflowContent` (T4) também, e os chamadores dele
   passam a ler a config: `discover.go`, o refresh em `update.go` e o doctor.
4. **Doctor (AC3):** a comparação continua byte a byte contra **o** template do gatilho configurado, com
   `checkScaffoldArtifact`. Não pode haver comparação por pertença a um conjunto, porque isso aceitaria um gatilho
   divergente do configurado.
5. **Valor inválido (AC4):** o `validate` emite violation com o valor e os aceitos. O `update` e o `init` recusam gerar o
   workflow com rc≠0 e deixam o arquivo existente intocado.
6. **Aviso (T3):** com `manual`, o `validate` emite um aviso informativo dizendo que o gate de PR não roda automaticamente
   e que `required_status_checks` que dependem dele bloqueiam o PR (T5).
7. **`init`:** honra o `ci_trigger` de um `trackfw.yaml` já existente. Não ganha flag nem pergunta nova no wizard, e o
   template do `init` não emite a chave.
8. **AC2:** antes de mudar os templates, capture em `internal/generators/testdata/ci-trigger/` a saída atual das três
   funções, nas variantes de produtor e de consumidor, a partir de `origin/main`. Um teste afirma igualdade byte a byte com
   o padrão. Não basta comparar a função contra ela mesma.
**Acceptance criteria:**
- [x] AC1–AC4 da REQ, cada um com teste e contra-braço
- [x] Injeção: `ci_trigger: "manual\non: push"` é inválido e nada é gerado
- [x] Os goldens do padrão foram capturados de `origin/main` e batem
**Gates da wave:**
```bash
go build ./...
go test ./internal/config/ ./internal/generators/ ./internal/discover/ ./internal/validator/ ./internal/commands/ ./internal/pathguard/ -count=1
```

**Auditoria do arquiteto (2026-10-24):** aprovado. Os gates estão verdes e o `parity-rest` deu rc=0. Os goldens do padrão
foram capturados de `origin/main`. Medição de ponta a ponta com o binário do worktree num projeto temporário:
- `ci_trigger: manual` → `trackfw update` gera `on:\n  workflow_dispatch:` e o `validate` avisa `ci_trigger_manual`;
- `ci_trigger: [manual]` → o `update` sai com rc=1 e o `trackfw-gate.yml` fica byte a byte intocado (`cmp`); o `validate`
  sai com rc=1 com `ci_trigger_invalid`.

Falta um ponto, que passa para o ML-1B: o aviso `ci_trigger_manual` não menciona `required_status_checks` (item 6 do
contrato e T5).

### ML-1B — Valor não escalar em chave escalar some em silêncio (mesma causa do T1)
**Status:** ✅ Concluído
**Agente:** apolo-tf
**Dependencies:** ML-1A, porque os dois mexem em `internal/config/config.go`.
**Files affected:** `internal/config/config.go`, `internal/config/config_scalar_type_test.go`, `internal/validator/validator.go`, `internal/validator/validator_config_scalar_test.go`, `docs/cli-parity.md`
**Por que existe:** a Wave 0 mediu que `stringVal` (`config.go`) descarta sequência e mapa (`ci: [github-actions]` vira
"ausente"). Isso vale para toda chave escalar, e não só para a `ci_trigger`. Corrigir só a `ci_trigger` deixaria a mesma
causa viva nas outras chaves.
**Acceptance criteria:**
- [x] O parser registra, num ponto único, toda chave escalar conhecida que recebeu sequência ou mapa, com o nome e o tipo
- [x] O `validate` emite violation `config_scalar_type` para cada uma; braços positivo (escalar) e negativo (sequência e mapa) testados
- [x] Contra-braço no HEAD
- [x] O aviso `ci_trigger_manual` também diz que `required_status_checks` que dependem do job do gate bloqueiam o PR, porque ele não roda (herdado do ML-1A)
**Gates da wave:**
```bash
go build ./...
go test ./internal/config/ ./internal/validator/ -count=1
```

**Auditoria do arquiteto (2026-10-24):** aprovado como entrega, mas incompleto como causa raiz. Corretivo no ML-1C.
- **Contra-braço medido:** o binário do HEAD sai com rc=0 e não diz nada sobre `ci: [x]` nem sobre `hooks: {a: b}`. O do
  worktree sai com rc=1 e emite duas violations.
- **O ponto não é único:** o `stringVal` sem registro continua existindo, e a `knownScalarKeys` é uma lista à parte. Uma
  chave nova, lida pelo `stringVal` antigo ou deixada fora da lista, escapa sem que nenhum teste perceba.
- **Mesma causa em outros sítios:** o agente declarou três (lista recebendo escalar; valores de `rules:` e
  `agent_models`; `credential_guard.mode`). Pela Regra Dura e por "defeito conhecido se corrige agora", entram no ML-1C.

### ML-1C — Ponto único de verdade para o tipo das chaves do `trackfw.yaml`
**Status:** ✅ Concluído
**Agente:** apolo-tf
**Files affected:** `internal/config/`, `internal/validator/validator.go`, `internal/validator/validator_config_scalar_test.go`, `docs/cli-parity.md`
**Acceptance criteria:**
- [x] Não sobra leitura de chave do `trackfw.yaml` que descarte tipo errado em silêncio. O `stringVal` sem registro deixa
  de existir ou passa a registrar, e um teste por AST ou grep em `internal/config/config.go` afirma que toda leitura
  passa pelo leitor que registra
- [x] A lista de chaves vem das próprias leituras, sem cópia à mão. Um teste falha se uma leitura nova não estiver coberta
- [x] Os três sítios declarados registram o erro (lista recebendo escalar ou mapa; valor não string em `rules:` e em
  `agent_models`; `credential_guard.mode` não escalar, com o nome qualificado `credential_guard.mode`)
- [x] Cada um tem violation no `validate` com o nome qualificado, e o braço negativo testado
- [x] Contra-braço no HEAD; gates do ML-1B verdes; `make parity-rest` rc=0

**Auditoria do arquiteto (2026-10-24):** aprovado.
- **Tabela de leitura:** `scalarKeyTable` substitui a lista mantida à mão.
- **Teste por AST:** `TestASTCoverage_NoSilentReads` tem dois braços, e os dois foram falsificados com saída colada.
- **Contra-braço por binário:** `agents: x` e `rules.wip_limit: [error]` passavam em silêncio no HEAD e agora emitem violation.
- **Renomeação:** a regra passou de `config_scalar_type` para `config_value_type`. Só sobraram comentários e nomes de
  arquivo com o nome antigo.
- **Gate:** o arquiteto rodou o `make parity-rest`, que o agente não tinha rodado. Deu rc=0.

### ML-1D — Revisão de qualidade da reescrita do parser
**Status:** ✅ Concluído
**Agente:** hefesto-tf
**Files affected:** `docs/qualidade/2026-10-24-revisao-parser-config.md`
**Por que existe:** o ML-1C reescreveu o `parse()` de `internal/config/config.go` em formato de tabela. Mudança desse tamanho
no leitor de toda configuração pede uma revisão independente de preservação de comportamento.
**Acceptance criteria:**
- [x] Parecer B/R sobre a preservação de comportamento para toda chave existente (valor, padrão, ausência, chave aninhada) — veredito APROVADO, 0 B e 3 R, em `docs/qualidade/2026-10-24-revisao-parser-config.md`.

### ML-1E — Corretivo: pin do cenário 39 e as R do Hefesto
**Status:** ⬜ Pendente
**Agente:** apolo-tf
**Files affected:** `scripts/check-gates-falsify.sh`, `internal/config/`
**Por que existe:** o `make quality` deu rc=2. O chunk 4 da falsificação morreu em `[s39-go] expected exactly 1 occurrence
of pattern, got 0`. O `GO_S39_OLD` (`scripts/check-gates-falsify.sh:3872`) fixa o corpo de `loadUpdateConfig`, que ganhou os
campos `CITrigger*` no ML-1A. É o sintoma documentado em
`vault/notes/cenarios-de-falsificacao-quebram-em-refactor-do-alvo-2026-08-02.md`. Revisões do Hefesto que entram junto:
R-1 (o `stringVal` vira obsoleto ou sai), R-2 (`ci_trigger:` vazio ou null vale como ausente, igual às outras chaves
escalares) e R-3 (o teste por AST pega também `stringValRec(m, "literal", …)` fora da tabela).
**Acceptance criteria:**
- [ ] `GO_S39_OLD` casando com o corpo atual. O `GO_S39_NEW` continua sabotando o mesmo comportamento (scanner artesanal)
- [ ] R-1, R-2 (com teste) e R-3 (terceiro braço do AST, falsificado)
- [ ] `go test ./internal/config/ ./internal/validator/ ./internal/generators/ -count=1` verde e `make quality` rc=0 (rodado pelo arquiteto)

## Wave 2 — Fechamento
> Dependencies: Wave 1 auditada.

### ML-2A — make quality e CI
**Status:** ⬜ Pendente
**Agente:** zeus-tf
**Files affected:** `docs/cli-parity.md`
**Acceptance criteria:**
- [ ] `make quality` rc=0
- [ ] CI do PR verde, com `Closes #607`
