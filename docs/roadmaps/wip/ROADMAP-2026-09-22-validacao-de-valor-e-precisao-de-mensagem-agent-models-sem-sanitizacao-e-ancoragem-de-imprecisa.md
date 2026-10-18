---
status: wip
date: 2026-09-22
req: "docs/req/REQ-2026-08-30-validacao-de-valor-e-precisao-de-mensagem-agent-models-sem-sanitizacao-e-ancoragem-de-til-imprecisa.md"
squad: ""
---

# Roadmap: Validação de valor e precisão de mensagem — `agent_models` sem sanitização e ancoragem de `~` imprecisa

> Created: 2026-09-22 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-08-30-validacao-de-valor-e-precisao-de-mensagem-agent-models-sem-sanitizacao-e-ancoragem-de-til-imprecisa.md -->
REQ: docs/req/REQ-2026-08-30-validacao-de-valor-e-precisao-de-mensagem-agent-models-sem-sanitizacao-e-ancoragem-de-til-imprecisa.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ] **AC1** — Valor de `agent_models` validado por formato antes de interpolar; formato inválido falha com mensagem nomeando a chave e o esperado, sem gerar arquivo.
- [ ] **AC2** — Falsificação nas duas direções: valores legítimos aceitos; valores com metacaractere YAML, espaço ou controle recusados.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** ✅ Concluído
**Files read:** `internal/integrations/render.go`, `internal/integrations/models.go`, `internal/integrations/plan.go`, `internal/commands/agents_models.go`, `internal/commands/integrations_flags.go`, `internal/commands/init.go`, `internal/generators/update.go`, `internal/integrations/doctor.go`, `internal/config/config.go`, `internal/identity/identity.go`, `internal/integrations/assets/catalog.json`
**Measurement:** binary built (`make build`), `agents install --scope global --targets claude` run 8 times with `HOME` redirected to `$(mktemp -d)`, frontmatter parsed with `gopkg.in/yaml.v3`.

---

#### 1. Enumeration completeness

**Superfície primária — única onde `agentModels` chega ao artefato escrito**

O `agent_models` do `trackfw.yaml` só é interpolado num artefato gravado quando o target é `claude` com representação `subagent`. O gate está em `render.go:205`:

```go
} else if targetID == "claude" && len(agentModels) > 0 {
    if isVersionString(version) {
        modelID = composeClaudeModelID(model, version)
    } else {
        modelID = version   // escape hatch: value used literally
    }
    source, rewErr = rewriteFrontmatterModelLine(source, modelID)
```

Todos os outros 9 targets (codex, gemini, antigravity, cursor, copilot, windsurf, amazonq, opencode, kiro) ignoram `agentModels` por construção — ADR-2026-08-21 §4. Confirmado na leitura dos 12 targets do catálogo (`assets/catalog.json`) cruzada com todos os `case` de `Render`.

**Cinco call sites de escrita que passam `AgentModels` para `BuildPlans`:**

| call site | comando do usuário | scope |
|---|---|---|
| `internal/commands/init.go:427` | `trackfw init` | project ou global |
| `internal/commands/integrations_flags.go:232` | `trackfw agents install` | qualquer |
| `internal/commands/integrations_flags.go:382` | `trackfw agents update` | qualquer |
| `internal/generators/update.go:2453` | `trackfw update harness` | global |
| `internal/commands/integrations_thirdparty.go:333,522` | `trackfw agents third-party install/update` | qualquer |

`internal/generators/update.go:179,2805` passam `agentModels` para um `BuildPlans` com `Targets: []string{"codex"}` — o efeito é nulo porque a representação `custom-agent-toml` (Codex) ignora o campo por estar num `case` separado antes do `default` que contém o guard de target `claude`.

`internal/integrations/doctor.go:292` passa `agentModels` para `BuildPlans`, mas `RunDoctor` não grava arquivos — inspeciona o estado e retorna `DoctorFinding`. Fora do escopo de "gerar arquivo".

**`BuildPlans` é fail-fast:** se `Render` retorna erro para qualquer agente, `BuildPlans` retorna `nil, err` e nenhum arquivo é escrito pelo chamador (os planos são acumulados antes de qualquer escrita de disco). AC1 "sem gerar arquivo" é satisfazível colocando a validação dentro do caminho de `Render`.

**AC3/AC4 confirmados corrigidos:** PR #271 adicionou `cwdDependentReason` em `internal/validator/validator_credential_guard.go` com ramos distintos para `"~/"` (aspas) e `~usuario/` — confirmado por `git show 87aded6b -- internal/validator/validator_credential_guard.go | grep "+.*tilde"`. Os MLs-1C e -1D não têm mais objeto.

**AC5 eliminado com a v8:** não há mais implementação Node.js/Python.

**Superfície secundária — mesmo mecanismo, outro canal de entrada:**

`render.go:143-144` (`agent-directory` / Antigravity) e `render.go:187` (`opencode-agent`) usam interpolação literal sem quotação:

```go
sb.WriteString("name: " + name + "\n")
sb.WriteString("description: " + description + "\n")
```

`name` e `description` derivam de `identity.Config`, lido de `~/.trackfw/identity.json`. `identity.Validate` (linha 107) só exige que `DisplayName` seja não-vazio; não valida contra newlines ou caracteres especiais YAML. Um `DisplayName` com `\n` injeta chave YAML. Esta é a mesma causa (interpolação literal em frontmatter), mas canal diferente (`identity.json`, não `agent_models`). Fora do escopo desta REQ — ver "Decisões que precisam do usuário" §3.

---

#### 2. Threat model

**O adversário é o implementador apressado e o arquiteto otimista** — o valor vem de `~/.trackfw/trackfw.yaml` do próprio usuário. Não há ator externo.

**Caminhos de esvaziamento sem violar regra escrita:**

**T-1 — Validar só no display path.** Adicionar formato check em `agents_models.go` (comando `trackfw agents models`). O WARN já existe. Nenhuma regra escrita exige que a validação seja no write path. O AC1 diz "falha sem gerar arquivo" — mas a frase não diz _onde_ falhar. Um implementador pode ler como "o comando `agents models` avisa antes que o usuário rode `agents install`". Resultado: AC1 não é satisfeito (arquivo ainda é gerado), AC2 para se os testes usarem só o comando de exibição.

**T-2 — Validar dentro de `rewriteFrontmatterModelLine`.** A função é compartilhada: `render.go:198` a chama para Cursor com `mapModelCursor` output (`claude-opus-5[effort=high]`, `composer-2.5[fast=true]`). Adicionar allowlist com `[a-zA-Z0-9._:@/+-]` rejeita `[`, `=`, `]` — quebra TODA instalação de Cursor, que o implementador não testou porque o teste só cobre `--targets claude`. A regressão aparece no CI de Cursor, que pode não existir no scope do PR.

**T-3 — Validar no config load.** `LoadGlobalAgentModels` retorna `AgentModelsSourceGlobalMalformed` quando o YAML é inválido. Adicionar validação de valor ali retornaria `AgentModelsSourceGlobalMalformed` para valores ilegais. O chamador `ResolveAgentModels` retorna `(empty_map, MalformedGlobalConfigMessage)`. O chamador `init.go:419` imprime o warn e prossegue com `agentModels = {}`. Resultado: o agente é instalado **sem nenhum pin** e exit 0 — AC1 "falha sem gerar arquivo" violado, silenciosamente.

**T-4 — Testar só o corpus do AC2 listado na REQ.** O AC2 menciona `5`, `4.6`, `4-5-sonnet` como legítimos. Um allowlist `[0-9a-z.-]` (minúsculas) bloqueia `anthropic.claude-opus-4-5-v1:0` (Bedrock, com `.`, `:`, uppercase) e `claude-opus-5@20250101` (Vertex, com `@`). Ambos são valores de escape-hatch documentados em ADR-2026-08-21 §3. O corpus de AC2 não os cobre — o gate passa, o falso positivo existe.

**T-5 — Só quotar, sem rejeitar.** Modificar `rewriteFrontmatterModelLine` para emitir sempre `model: "value"`. Resolve os vetores `5: danger` e `{tools: Bash}` (tornam-se strings YAML válidas). Não satisfaz AC1 "formato inválido falha" — o arquivo é gerado com o valor ruim citado. O gate de AC2 passa para os testes listados; nenhum teste verifica que `agents install` retorna exit != 0 para valor inválido.

---

#### 3. Falsificação nas duas direções

**Superfície A — validação no write path (claude-branch em `Render`)**

| direção | vetor | comportamento esperado | comportamento atual (medido) |
|---|---|---|---|
| FN (não detecta defeito) | `opus: "5 && rm -rf /"` | exit 1, sem arquivo | exit 0; `model: 5 && rm -rf /` gravado; YAML parseia como string |
| FN | `opus: "5: danger"` | exit 1, sem arquivo | exit 0; `model: 5: danger` gravado; **YAML PARSE ERROR** — agente falha ao carregar |
| FN | `opus: "claude-opus-5 #x"` | exit 1, sem arquivo | exit 0; `model: claude-opus-5 #x` gravado; YAML parseia como `claude-opus-5` (truncamento silencioso) |
| FN | `opus: "{tools: Bash}"` | exit 1, sem arquivo | exit 0; `model: {tools: Bash}` gravado; YAML parseia `model` como `map[tools:Bash]` (tipo errado) |
| FP (bloqueia legítimo) | `opus: "anthropic.claude-opus-4-5-v1:0"` | exit 0; `model: anthropic.claude-opus-4-5-v1:0` gravado | exit 0; gravado corretamente — mas allowlist restrito (sem `:`) quebraria |
| FP | `opus: "claude-opus-5@20250101"` | exit 0; model com `@` gravado | exit 0; gravado corretamente — mas allowlist sem `@` quebraria |
| FP | `opus: "4-5-sonnet"` | exit 0; `model: 4-5-sonnet` gravado | exit 0; gravado corretamente — já passa |
| FP | `opus: "claude-sonnet-4-5-20250929"` | exit 0; ID literal gravado | exit 0; gravado corretamente — já passa |

**Superfície B — `LooksLikeSuspectModelValue` (warn path, `agents models`)**

| direção | vetor | comportamento esperado | comportamento atual |
|---|---|---|---|
| FN | `opus: "claude-opus-5 #x"` | WARN | sem WARN — começa com `claude-`, passa o check |
| FN | `opus: "claude-opus-5: danger"` | WARN | sem WARN — começa com `claude-`, passa o check |
| FP | `opus: "anthropic.claude-opus-v1:0"` | sem WARN | WARN — não começa com `claude-` |

**Nota sobre o valor com `"`:** `agent_models: { opus: "claude\"5" }` no YAML do config produz YAML inválido (aspas embutidas sem escape). `yaml.v3` falha o parse em `LoadGlobalAgentModels` → `AgentModelsSourceGlobalMalformed` → `agentModels = {}` → agente instalado com tier alias sem aviso visível (stderr tem MalformedGlobalConfigMessage, mas exit 0). Não é injeção — é silent un-pin. Teste com YAML single-quoted (`opus: 'claude"5'`): `model: claude"5` gravado; YAML parseia como string — seguro.

**Vetores de confusão de tipo YAML (medidos — nova classe, afeta D-2):**

| valor | config YAML | `model:` gravado | tipo YAML parseado |
|---|---|---|---|
| `null` | `opus: null` | `model: null` | `<nil>` |
| `true` | `opus: true` | `model: true` | `bool` |
| `false` | `opus: false` | `model: false` | `bool` |
| `.inf` | `opus: .inf` | `model: .inf` | `float64` |
| `1e3` | `opus: 1e3` | `model: 1e3` | `float64 (1000)` |
| `0x10` | `opus: 0x10` | `model: 0x10` | `int (16)` |
| `~` | `opus: ~` | `model: ~` | `<nil>` |
| `@x` | `opus: @x` | `model: opus` | string — **mas é silent un-pin** (`@` inicia nó YAML reservado → parse error na config → `agentModels = {}` → tier alias usado) |

Esses valores passam por qualquer allowlist baseado em `[a-zA-Z0-9._]` mas produzem modelo de tipo errado ou silent un-pin. Implicação direta para D-2: allowlist puro (opção a) é insuficiente; quoting obrigatório na saída (opção b) elimina a confusão de tipo porque `model: "null"` parseia sempre como string.

---

#### 4. Declared residual

**R-1 — Superfície identity.json não coberta.** `agent-directory` (Antigravity) e `opencode-agent` interpolam `name` e `description` literalmente no frontmatter YAML — mesmo mecanismo que `model:`, mas lendo de `~/.trackfw/identity.json`. `identity.Validate` não checa newlines em `DisplayName`. Esta superfície tem a mesma causa (interpolação literal sem sanitização), mas origem diferente e alvo diferente (nome/descrição, não modelo). Esta REQ cobre só `model:` via `agent_models`. A superfície identity fica como resíduo declarado.

**R-2 — Escape hatch por construção.** ADR-2026-08-21 §3 deliberadamente permite qualquer string não-versão como literal. Remover essa propriedade quebraria casos de uso documentados (IDs de Bedrock, IDs com data, formatos futuros). O que esta REQ entrega é: detectar e rejeitar valores que corrompem YAML; o escape hatch sobrevive para valores bem-formados.

**R-3 — Arquivo de agente editado manualmente.** Se o usuário editar `~/.claude/agents/trackfw-architect.md` e escrever um `model:` inválido diretamente, nenhum gate do trackfw cobre isso. Fora de escopo por construção.

**R-4 — Bedrock/Vertex como alvo `claude`.** `anthropic.claude-opus-4-5-v1:0` e `claude-opus-5@20250101` são valores de escape hatch válidos? A ADR-2026-08-21 só documenta Bedrock como exemplo de "ID completo" que motivou o escape hatch. Se Claude Code aceita esses IDs no campo `model:` de um subagente, eles devem passar o allowlist; se não aceita, são falso-negativo do produto. Esta REQ não testa o comportamento de Claude Code com esses IDs — o escopo é YAML bem-formado, não compatibilidade de runtime do Claude Code.

---

**Acceptance criteria:**
- [x] The four sections above answered with evidence, not a one-line assertion
- [x] No implementation line written for this ML

**Gates da wave:**
```bash
count=$(grep -c "^#### [0-9]\." "docs/roadmaps/wip/ROADMAP-2026-09-22-validacao-de-valor-e-precisao-de-mensagem-agent-models-sem-sanitizacao-e-ancoragem-de-imprecisa.md" 2>/dev/null || echo 0); [ "$count" -eq 4 ] || { echo "GATE FAIL: esperado 4 secoes numeradas, encontrado $count"; exit 1; }
```

---

## Decisões que precisam do usuário

**D-1 — Onde colocar a validação de formato?**

Recomendação: no ramo `claude` de `Render` (`render.go:205`), imediatamente antes de `rewriteFrontmatterModelLine(source, modelID)` ser chamado. Espelhar a mesma check em `ResolveAgentModel` (`models.go`) para que `agents models` falhe com a mesma mensagem que o write path (invariante do drift gate `TestResolveAgentModelMatchesRender`).

Alternativas descartadas: dentro de `rewriteFrontmatterModelLine` (quebra Cursor — T-2 acima); em `config.Load` (silent un-pin — T-3 acima).

**D-2 — Allowlist ou quotação obrigatória?**

Medição mostra que os três vetores perigosos hoje são:
- `: ` (colon-espaço): `model: 5: danger` → YAML PARSE ERROR
- `{` `}` (flow mapping): `model: {tools: Bash}` → YAML tipo `map` em vez de `string`
- ` #` (espaço-hash): `model: claude-opus-5 #x` → YAML comment truncation silenciosa

O vetor `5 && rm -rf /` produz YAML válido (string com espaços) mas valor incorreto.

Opção recomendada **(b): allowlist + saída sempre em double-quote para o pin claude.**
- Allowlist para o ramo de escape hatch: `[a-zA-Z0-9._:@/+-]` (cobre IDs Claude, IDs Bedrock com `.` e `:0`, IDs Vertex com `@`, separador de provedor `/`, hífens e pontos ubíquos). Rejeitar se contiver espaço, `{`, `}`, `#`, `"`, `'`, `,`, `[`, `]`.
- Emissão com double-quote obrigatório para o valor composto E para o escape-hatch: `model: "claude-opus-5"`, `model: "anthropic.claude-opus-v1:0"`. Isso torna o colon-space seguro como fallback mesmo que o allowlist seja relaxado.
- Quoting escopo: APENAS no ramo claude de `Render` (linhas 217-227), não dentro de `rewriteFrontmatterModelLine`.

Opção (a) allowlist sem quotação: suficiente para os vetores medidos, mas sem defesa em profundidade para formatos futuros de modelo.

Opção (c) só quotar sem rejeitar: não satisfaz AC1 ("formato inválido falha") — arquivo é gerado com valor citado mas potencialmente inútil (`model: "5 && rm -rf /"`).

Decisão do usuário necessária: o charset `[a-zA-Z0-9._:@/+-]` bloqueia valores com espaço (como `"5 && rm -rf /"`). Isso é intenção do AC2 ("espaço recusado")? Se sim, opção (b) cobre. Se o usuário quiser aceitar espaços para nomes de modelo com espaço (nenhum formato known tem), seria (c).

**D-3 — Superfície identity.json (resíduo R-1): abre REQ nova ou fica como resíduo declarado?**

Mesmo mecanismo: interpolação literal em frontmatter YAML sem sanitização, agora em `name` e `description`. Canal diferente: `~/.trackfw/identity.json`, campo `DisplayName`, sem check de newlines. Para fechar a causa completa (Regra Dura), um ML adicional nesta REQ ou REQ nova. Recomendação: declarar como resíduo por ora (o campo `DisplayName` vem de um wizard interativo, não de config YAML sem validação de UI), mas documentar e acompanhar.

**D-4 — O que fazer com `LooksLikeSuspectModelValue` após o fix?**

Opção (i): manter o warn como está, adicionar os novos vetores ao check (`claude-xxx: danger` — começa com `claude-` mas tem `: `). Opção (ii): remover o warn, deixar só a rejeição no write path. Recomendação: manter o warn e estender para cobrir os FNs medidos (Superfície B, tabela acima) — o warn em `agents models` é mais cedo que o erro em `agents install`.

---

## Wave 1 — Implementation (derived from REQ criteria)
> Dependencies: Wave 0 completa

### ML-1A — **AC1** — Validação de formato antes de interpolar; formato inválido falha sem gerar arquivo
**Status:** ⬜ Pendente
**Files affected:** `internal/integrations/render.go` (ramo claude, ~linha 214), `internal/integrations/models.go` (`ResolveAgentModel`)
**Actions:**
1. Adicionar função `isValidModelPinValue(v string) bool` em `render.go` (ou `models.go`, compartilhada): retorna true se `v` corresponde a `[a-zA-Z0-9._:@/+-]+` com zero espaço, `{`, `}`, `#`, `"`, `'`, `,`, `[`, `]`.
2. No ramo claude de `Render` (~linha 214), antes de chamar `rewriteFrontmatterModelLine`, checar `isValidModelPinValue(version)`. Se falso, retornar `nil, fmt.Errorf(...)` nomeando a chave e o esperado.
3. Espelhar a check em `ResolveAgentModel` para o ramo `targetID == "claude"`: se `isValidModelPinValue` falso, retornar `("", false)` ou um error sentinel (decisão de implementação — avaliar impacto em `agents models`).
4. Para o output, emitir double-quote obrigatório para o pin claude: `model: "claude-opus-5"` em vez de `model: claude-opus-5`.
**Acceptance criteria:**
- [ ] `agents install --scope global --targets claude` com `opus: "5: danger"` retorna exit != 0 e não cria nenhum arquivo de agente
- [ ] `agents install --scope global --targets claude` com `opus: "{tools: Bash}"` retorna exit != 0 e não cria nenhum arquivo
- [ ] `agents install --scope global --targets claude` com `opus: "5 && rm -rf /"` retorna exit != 0 (espaço rejeitado)
- [ ] `update harness` com valor inválido retorna exit != 0 para o target claude
- [ ] build passes · tests green

### ML-1B — **AC2** — Falsificação nas duas direções (corpus completo)
**Status:** ⬜ Pendente
**Files affected:** `internal/integrations/render_test.go` ou `internal/integrations/models_test.go`
**Actions:**
1. Adicionar testes que cobrem os dois lados: aceitos (`5`, `4.6`, `4-5-sonnet`, `claude-sonnet-4-5-20250929`, `anthropic.claude-opus-4-5-v1:0`, `claude-opus-5@20250101`) e rejeitados (`5: danger`, `{tools: Bash}`, `claude-opus-5 #x`, `5 && rm -rf /`).
2. Para cada teste "aceito": confirmar que o YAML do artefato resultante parseia como `string` (não `map` nem erro).
3. Para cada teste "rejeitado": confirmar exit != 0 e ausência do arquivo de agente.
**Reconciliação (Regra Dura de Causa Raiz):** cada teste novo afirma qual conclusão da Seção 3 (tabela de falsificação) ele verifica — escrever em comentário de teste.
**Acceptance criteria:**
- [ ] Corpus de aceitos: todos passam sem arquivo corrompido (YAML parseia como string)
- [ ] Corpus de rejeitados: todos falham com exit != 0 e sem arquivo gerado
- [ ] Cursor install não regride (representação `agent-markdown`, targetID cursor)
- [ ] build passes · tests green

### ML-1C — **AC6** — `make quality` exit 0 e CI verde
**Status:** ⬜ Pendente
**Files affected:** scripts de gate (se houver gate específico de `agent_models`)
**Actions:**
1. Se existir `scripts/check-agent-models-parity.sh` ou similar, atualizar com os novos vetores de rejeição.
2. `make quality` exit 0.
3. CI verde.
**Acceptance criteria:**
- [ ] `make quality` exit 0
- [ ] CI verde

---

## Triagem pós-v8 — 2026-10-17 (REQ-2026-10-17)

**Vivo (AC1–AC2)** e **absorve o resíduo de REQ-2026-08-21**: `render.go:587` só rejeita caractere de controle; `"5 && rm -rf /"` vira artefato com WARN e exit 0; `:`/`"` geram YAML inválido. AC3–AC4 corrigidos (#271); AC5 sumiu com a v8.

Evidência e tabela completa: `docs/governanca/2026-10-17-triagem-backlog-pre-v8.md`.

## Decisões do usuário — 2026-10-17

- **D-1 (aceita como recomendada):** validar no ramo `claude` de `Render`, espelhado em
  `ResolveAgentModel`; nem em `rewriteFrontmatterModelLine` (quebra o Cursor, T-2) nem só no load
  (silent un-pin, T-3). Falha antes de gravar qualquer arquivo, nos 5 call sites.
- **D-2 (usuário): lista `[a-zA-Z0-9._:@/+-]` + `model:` sempre entre aspas.** Fora da lista →
  erro nomeando a chave e o esperado, exit ≠ 0, nenhum arquivo. Aceito → emitido quotado e escapado
  (fecha a troca de tipo `null`/`true`/`0x10`). Mudança visível: o `model:` dos agentes do Claude passa
  a sair entre aspas — vai para o CHANGELOG.
- **D-3 (usuário): `identity.json` entra nesta REQ** (mesma causa). ML novo: `name`/`description`
  emitidos de forma segura em YAML (aspas quando necessário) e caractere de controle recusado ao ler.
- **D-4 (aceita):** `LooksLikeSuspectModelValue` cobre os falsos negativos medidos
  (`claude-x: danger`, `claude-x #x`) e deixa de avisar sobre ids Bedrock legítimos.
