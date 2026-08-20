# Parecer de Segurança — ML-4A: Gates dos Três Contratos de Maior Risco

**Data:** 2026-08-20
**Agente:** Hades (Security Reviewer)
**Roadmap:** `ROADMAP-2026-08-20-gates-para-os-tres-contratos-de-maior-risco.md`
**Branch:** `feat/gates-para-os-tres-contratos-de-maior-risco`
**Postura declarada:** ADR-2026-08-12 (detecção ancorada, não prevenção); ADR-2026-08-17 (falso-positivo é risco de primeira ordem)

---

## VEREDITO GLOBAL

**APROVADO COM RESSALVAS**

Os três contratos entregues (ML-1B: Windsurf/Amazon Q em agent-hooks-parity; ML-2A: branch_has_wip_roadmap com done/; ML-3A: credential_guard_hook_resolvable cross-CLI) provam o que declaram. Os gates detêm a regressão que se propõem a detectar. Cinco debts residuais são nomeados abaixo. Quatro são pré-existentes (A-1, B-1, B-3, D-2). Um (C-1) é introduzido por esta entrega: a expansão para done/ cria um corpus monotonicamente crescente onde slugs curtos satisfazem o gate sem correspondência semântica real. Nenhum é blocking sob as posturas ADR-2026-08-12 e ADR-2026-08-17.

---

## Convenções deste parecer

- **Medido:** evidência produzida por execução de fixture real (`make build` + `./bin/trackfw`, Node e Python via PYTHONPATH isolado). Caminhos exatos disponíveis no scratchpad de sessão.
- **Inferido:** evidência derivada de leitura de código-fonte sem fixture de execução.
- **Debt residual nomeado:** gap identificado que não bloqueia, mas deve ser registrado para fechamento em REQ separada.

---

## A. `credential_guard_hook_resolvable` cross-CLI (ML-3A)

### A.1 Prova idêntica nos 3 CLIs?

**Medido — SIM.**

Fixture exata do `check-validate-parity.sh` reproduzida manualmente:
- `cg-claude-absent`: `.claude/settings.json` com `"type":"command"` + script ausente → VIOLATION nos 3 CLIs com mensagem byte-idêntica: `".claude/settings.json (Claude Code) references trackfw-credential-guard.sh resolved to …, but the script does not exist"`
- `cg-claude-present`: mesmo hook, script presente e executável → silêncio nos 3 CLIs
- `cg-cursor-absent`: `.cursor/hooks.json` com caminho relativo, script ausente → VIOLATION nos 3 CLIs
- `cg-cursor-present`: idem, script presente → silêncio

A filtragem por `item.get("rule") == "credential_guard_hook_resolvable"` funciona corretamente nos 3 runtimes (Python usa `_enrich_items` com dicts, não strings).

### A.2 Os 4 casos cobrem o que declaram?

**Medido — SIM, com dois caminhos não exercitados pelos 4 casos.**

Os 4 casos declaram cobrir "script ausente" e "script presente". O code path "script presente mas não executável" e o code path "hook registrado sem `type:command`" produzem violações distintas nos 3 CLIs, mas nenhum dos 4 casos os exercita explicitamente.

**Medições realizadas:**

**Caminho não-executável (chmod 644, script presente):**
```
Go:   VIOLATION: "…but the script is not executable — run `trackfw update` to regenerate it"
Node: VIOLATION: "…but the script is not executable — run `trackfw update` to regenerate it"
Py:   VIOLATION: "…but the script is not executable — run `trackfw update` to regenerate it"
```
Todos 3 CLIs detectam. Sanity (chmod +x): silêncio correto nos 3.

**Caminho missing `type:command` (script ausente):**
```
Go:   VIOLATION: "…but the hook entry is missing "type":"command" (or has an invalid type) — Claude Code will silently never execute it; run `trackfw update` to regenerate it"
Node: VIOLATION: (mensagem idêntica)
Py:   VIOLATION: (mensagem idêntica)
```
Todos 3 CLIs detectam.

**Consequência para o gate:** os 4 casos de ML-3A passam corretamente porque "script presente e executável → silêncio" é o discriminante correto. Mas uma regressão futura que removesse a verificação de exec-bit ou de `type:"command"` não seria detectada pelos 4 casos existentes, porque os casos "absent" continuariam a disparar (por script ausente) e os casos "present" continuariam silentes (por script executável presente). Os dois caminhos adicionais não estão guarded no gate.

**Debt residual A-1 (cobertura de caminhos não exercitados):**
- `cg-claude-noexec`: script presente, `chmod 644` → deve produzir violation com "script is not executable"
- `cg-claude-notype`: hook sem `"type":"command"`, script ausente → deve produzir violation com "missing type"
- **Precisão sobre `type:command`:** O code path missing-`type` **já está** coberto no scope de harness (gate `gvmt`, que valida hooks gerados por `discover --init`). O gap de A-1 é especificamente no scope de projeto — `credential_guard_hook_resolvable` em `check-validate-parity.sh` — onde nenhum dos 4 casos existentes exercita este path. Quem fecha A-1 está estendendo um padrão existente, não inventando.
- Fechamento: dois casos adicionais no bloco ML-3A de `check-validate-parity.sh`, com Cenários 81 e 82 em `check-gates-falsify.sh` para sabotagem dos respectivos code paths. REQ separada.

### A.3 Caminho silenciador que os 4 casos não alcançam?

**Medido — encontrado e descrito em A.2 (caminhos não-executável e sem type).** Ambos produzem violação hoje; o gap é que o gate não detectaria regressão nesses paths.

---

## B. Windsurf e Amazon Q — check-agent-hooks-parity.sh (ML-1B)

### B.1 `deniedCommands` é comparado?

**Medido — SIM.**

Fixture exata (HOME isolado, marcadores para todos os 8 CLIs, sem `trackfw.yaml` pré-existente):

```
Go:   tools: ['*'], deniedCommands: ['^git (commit|push|checkout -b)'], preToolUse: 1 entrada
Node: tools: ['*'], deniedCommands: ['^git (commit|push|checkout -b)'], preToolUse: 1 entrada
Py:   tools: ['*'], deniedCommands: ['^git (commit|push|checkout -b)'], preToolUse: 1 entrada
```

Os três CLIs produzem arquivos Amazon Q idênticos. `compare_json` faz diff recursivo preservando ordem de array e comparando valores — `deniedCommands` estaria no diff se qualquer runtime divergisse.

**Debt residual B-1 (vacuity guard não cobre deniedCommands):**

A vacuity guard P2 do gate para `amazonq` verifica presença da string `trackfw-git-branch-guard.sh` no arquivo. Não verifica presença de `deniedCommands`. Uma regressão que removesse `deniedCommands` de forma corretamente implementada nos 3 CLIs (drop correlacionado, não divergência) não seria detectada pela vacuity guard — `compare_json` continuaria verde (ambos os lados sem a chave) e a guard P2 continuaria verde (script string presente).

Evidência: `guard_marker_for()` em `check-agent-hooks-parity.sh` linha 209 retorna apenas `"trackfw-git-branch-guard.sh"` para windsurf/amazonq. `deniedCommands` não tem vacuity guard equivalente.

Estado atual: nenhuma regressão (todos 3 escrevem `deniedCommands` identicamente). Risco: futuro drop correlacionado seria silencioso para a gate. Fechamento: vacuity guard adicional para `deniedCommands` no bloco amazonq, mais Cenário 83 em falsify. REQ separada.

### B.2 `guard_marker_for()` é guarda real (marker ausente reprova)?

**Inferido — SIM, com nit de robustez.**

`guard_marker_for()` retorna literal não-vazio em ambos os branches (`windsurf|amazonq` → literal, `*` → literal). `grep -q ""` para string não-vazia é sempre uma grep real. Marker ausente → `grep -q` falha → `fail` chamado.

Nit de robustez (não-blocking): `guard_marker_for()` não tem o arm `*) ... exit 1` que `marker_for()` e `hookfile_for()` têm. Um 9º CLI adicionado sem atualizar `guard_marker_for()` herdaria silenciosamente a string do credential-guard em vez de falhar explicitamente. Isso seria uma falha ruidosa (grep para string errada), não silenciosa, mas menos clara que um exit 1 explícito.

### B.3 "Windsurf e Amazon Q nunca terão harness scope" — afirmação correta?

**Medido/Inferido — PARCIALMENTE INCORRETA.**

A afirmação é assimétrica:

**Windsurf:** estruturalmente correto. Windsurf não tem mecanismo de hook global nativo pré-execução. O comentário em `harnessCatalogTargetOrder` (`internal/generators/update.go`) registra isso como decisão arquitetural. Sem artefato para gatear.

**Amazon Q:** ausência de implementação, não de possibilidade. `catalog.json` linha 44 registra:
```
"paths": {"agents": [{"scope": "global", "path": "~/.aws/amazonq/cli-agents/trackfw-{{id}}.json", ...}], ...}
```
Amazon Q tem caminhos globais no catálogo. O `check-harness-hooks-parity.sh` (linhas 15-23) confirma: "Amazon Q was simply never given a harness-scope pair". Isso é debt de implementação, não impossibilidade estrutural.

A anotação em `docs/cli-parity.md` linha 4258 agrupa os dois juntos ("Windsurf e Amazon Q nunca tiveram par de targets de harness") sem distinguir as razões. Uma nota de vault existente confirma essa leitura; o texto da anotação pode induzir futuros agentes a tratar a ausência de Amazon Q como permanente, quando é pendência.

**Debt residual B-3 (imprecisão na documentação):**
A anotação em `docs/cli-parity.md` deve ser emendada para distinguir:
- Windsurf: "estruturalmente impossível — sem hook global nativo"
- Amazon Q: "pendente — caminhos `~/.aws/amazonq/` existem no catálogo, harness targets não implementados"
Escopo: docs-only, não afeta controles ativos. REQ separada.

---

## C. `branch_has_wip_roadmap` com done/ (ML-2A)

### C.1 Discriminante (done/ com slug diferente recusa) é suficiente?

**Medido — SIM, com comportamento de substring esperado e documentado.**

O `BranchSlugMatchesRoadmap` em Go (`validator.go` linha 2117) usa `strings.Contains(normalizeBranchSlug(name), branchSlug)`. Isso é: "o nome do roadmap contém o slug do branch".

Testes realizados:

| Branch | done/ Roadmap | Resultado Go/Node/Py |
|--------|--------------|---------------------|
| `feat/minha-feature` | `ROADMAP-...-minha-feature-v2.md` | ACEITA (v2 contém "minha-feature") |
| `feat/minha-feature-v2` | `ROADMAP-...-minha-feature.md` | RECUSA ("minha-feature" não contém "minha-feature-v2") |
| `feat/feat-x` | `ROADMAP-...-feat-x-extra-words.md` | ACEITA (contém "feat-x") |
| `feat/minha-feature` | `ROADMAP-...-outra-coisa.md` | RECUSA ("outra-coisa" não contém "minha-feature") |

O comportamento de substring é intencional pela documentação ("inclua o branch slug no nome do roadmap"). Um roadmap `ROADMAP-...-minha-feature-v2.md` satisfaz o gate para branch `feat/minha-feature` porque o slug está contido no nome. O discriminante "outra-coisa" do ML-2A é suficiente para provar que slugs genuinamente diferentes recusam.

Os 3 CLIs concordam em todos os casos testados.

### C.3 Corpus de done/ cresce monotonicamente — risco de slug collision?

**Medido — SIM. Debt C-1 nomeado.**

Antes de ML-2A, `branch_has_wip_roadmap` escaneava apenas `wip/`, curado e ativamente podado. Após ML-2A, escaneia também `done/`, que nunca é podado e cresce com cada entrega.

Contagem medida no repo real (2026-08-20):

| Slug curto | Matches em done/ |
|-----------|-----------------|
| `guard`   | 11 |
| `serve`   | 3  |
| `gates`   | 1  |
| Total done/ | 127 |

Um branch `fix/guard` passaria validado hoje contra 11 roadmaps distintos em done/. O code path não mudou (substring-contains é intencional), mas o conjunto de contrapartes que satisfazem a condição cresceu por efeito da expansão para done/ — e continuará crescendo. O discriminante "slug diferente recusa" permanece correto; o risco é que slugs curtos ou genéricos se tornem cada vez mais fáceis de satisfazer sem que haja correspondência semântica real entre a branch e o roadmap.

**Debt residual C-1 (amplitude do corpus de done/):**
A expansão para done/ introduzida em ML-2A é a única mudança neste REQ que altera o comportamento de controle de forma acumulativa. Mitigações candidatas: (a) exigir match de boundary em vez de substring (slug inteiro delimitado por `-` no nome do arquivo), ou (b) aceitar done/ apenas dentro de uma janela de recência configurável. Severidade: Baixa/Média (cresce com o tempo). Fechamento: REQ separada, Apolo.

### C.2 Python `rule: null` — registro honesto ou normalização de defeito?

**Inferido — registro honesto, com consequência de CI nomeada.**

`validate_branch_has_wip_roadmap` em Python retorna `list[str]`, enquanto `_enrich_items` espera `list[dict]`. O `rule` não é aplicado às violações desta função em Python, resultando em `"rule": null` no JSON de saída.

O pin no gate (`check-validate-parity.sh`) é bidirecional: qualquer mudança em qualquer direção (Python corrigindo, ou Go/Node perdendo o tag) reprovaria a asserção. Uma REQ foi aberta para corrigir o defeito em Python.

Consequência de CI: filtros em CI que usam `--json` e filtram por `rule == "branch_has_wip_roadmap"` perdem silenciosamente esta violação no runtime Python. O gate atual documenta isso mas não o corrige. REQ aberta é o mecanismo correto.

---

## D. ML-1A-bis — Remoção de 6 campos do Amazon Q agent JSON

### D.1 `allowedTools` e `tools` têm papel de restrição?

**Medido — `tools: ['*']` está presente e idêntico nos 3 CLIs. `allowedTools` foi removido de Node/Python para alinhar ao Go canônico.**

- `tools: ['*']` (mantido): grant de todas as ferramentas no manifest do agente. Presente e idêntico nos 3 CLIs.
- `allowedTools` (removido): **MEDIDO** — Go nunca escreveu este campo. Grep em `internal/generators/agentfiles.go` retorna 2 ocorrências (linhas 1366 e 1406) — ambas em comentários de código, nenhuma em código de produção. Idêntico para `npm/src/generators/hooks.js` (linha 2130: comentário) e `pypi/trackfw/generators/hooks.py` (linhas 1263 e 1278: comentários). Nenhum dos 3 CLIs produzia `allowedTools` em código de produção; ML-1A-bis alinha a documentação de comentários ao comportamento real. A direção de viagem (escrever menos campos em schema não verificado) é a correta para reduzir risco de rejeição do arquivo pelo runtime.
- `deniedCommands` (mantido em `toolsSettings.execute_bash`): principal mecanismo de deny. Presente e idêntico nos 3 CLIs.

**D.2 Schema AWS não verificado.**

O schema do Amazon Q CLI para custom agent JSON não foi verificado formalmente. Risco: se o schema vigente rejeitar o arquivo inteiro por campo ausente ou inesperado, o hook e o `deniedCommands` falham silenciosamente (fail-open para Amazon Q). Este risco pré-data ML-1A-bis (Go já escrevia o mínimo) e ML-1A-bis corta de 9 para 3 os campos Node/Python divergindo do Go — reduz a superfície, não a amplia.

**Debt residual D-2 (schema não verificado):**
Verificação formal do schema Amazon Q CLI custom agent ou teste de aceitação end-to-end. Este é risco funcional (agente pode ser rejeitado pelo runtime) — Apolo o fecha, não Hades. Hades registra que nenhuma proteção foi removida pela mudança.

---

## Quadro-resumo de debts residuais

| ID | Severidade | Descrição | Fechamento |
|----|-----------|-----------|-----------|
| A-1 | Média | Gate ML-3A não exercita "script não-executável" nem "missing type:command" no scope de projeto como casos explícitos (harness scope já cobre via gvmt) | 2 novos fixtures + 2 cenários falsify (REQ separada) |
| B-1 | Baixa | Vacuity guard amazonq não cobre `deniedCommands` — drop correlacionado nos 3 CLIs não seria detectado | Vacuity guard adicional + Cenário 83 (REQ separada) |
| B-3 | Baixa | Documentação agrupa Windsurf/Amazon Q sem distinguir razões (impossibilidade vs. pendência) | Emenda em `docs/cli-parity.md` (docs-only, REQ separada) |
| C-1 | Baixa/Média | done/ cresce monotonicamente (127 files hoje; `guard` já bate 11 roadmaps) — ML-2A é a mudança que introduz essa amplitude; slug curto passa o gate sem correspondência semântica real | Boundary match ou janela de recência configurável (REQ separada, Apolo) |
| D-2 | Baixa | Schema Amazon Q CLI custom agent não verificado formalmente | Verificação end-to-end por Apolo (REQ separada) |

A-1, B-1, B-3 e D-2 preexistiam ao REQ sob revisão. **C-1 é o único debt que esta entrega introduz** (resultado direto da expansão para done/ em ML-2A). Nenhum enfraquece um controle ativo. Nenhum é blocking sob as posturas ADR-2026-08-12 e ADR-2026-08-17.

---

## Controles confirmados como ativos

1. **`credential_guard_hook_resolvable`** detecta: (a) script ausente, (b) script não-executável, (c) hook sem `"type":"command"` — todos nos 3 CLIs, mensagens byte-idênticas nos casos cross-CLI.
2. **`compare_json` em check-agent-hooks-parity.sh** é diff recursivo completo — `deniedCommands` e `tools` estão no diff se qualquer runtime divergir.
3. **`guard_marker_for()` em check-agent-hooks-parity.sh** é guarda real — marker ausente reprova.
4. **`branch_has_wip_roadmap` done/** aceita corretamente roadmaps cujo nome contém o branch slug; recusa slugs genuinamente diferentes; 3 CLIs concordam.
5. **Python `rule: null`** é pinado bidirecionalmente — drift em qualquer direção reprova.
6. **Amazon Q `deniedCommands`** e **`tools: ['*']`** estão presentes e idênticos nos 3 CLIs.

---

## Arquivos examinados

- `scripts/check-agent-hooks-parity.sh` (leitura completa, 311 linhas)
- `scripts/check-validate-parity.sh` (leitura completa, 725 linhas)
- `scripts/check-gates-falsify.sh` (cenários 78–80, linhas 7437–7600)
- `docs/cli-parity.md` (anotações relevantes, linhas 4258–4279)
- `internal/validator/validator.go` (linhas 2104–2124: `BranchSlugMatchesRoadmap`)
- `internal/generators/agentfiles.go` (grep: `deniedCommands`)
- `npm/src/generators/hooks.js` (grep: `deniedCommands`)
- `pypi/trackfw/generators/hooks.py` (grep: `deniedCommands`)
- `internal/integrations/assets/catalog.json` (linha 44: Amazon Q global paths)
- `scripts/check-harness-hooks-parity.sh` (linhas 15–23: harness scope justificativas)

## Fixtures medidos

Todos os fixtures foram criados em diretórios isolados no scratchpad de sessão. Nenhum arquivo de produto foi modificado.

- **A1-exact**: `.claude/settings.json` com `type:command`, script `chmod 644` → violação nos 3 CLIs ("script is not executable")
- **A2-notype**: `.claude/settings.json` sem `type:command`, script ausente → violação nos 3 CLIs ("missing type")
- **A2-exact**: `.claude/settings.json` com `type:command`, script ausente → violação nos 3 CLIs ("does not exist")
- **B1-exact**: HOME isolado, todos os 8 marcadores de CLI, `discover --init` nos 3 CLIs → arquivos Amazon Q idênticos com `deniedCommands`
- **C1**: `feat/minha-feature` com `done/ROADMAP-...-minha-feature-v2.md` → aceita nos 3 CLIs (substring por design)
- **C1b**: `feat/minha-feature-v2` com `done/ROADMAP-...-minha-feature.md` → recusa nos 3 CLIs
