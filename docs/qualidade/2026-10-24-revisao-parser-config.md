# Revisão de qualidade — reescrita do `parse()` em `internal/config/config.go`

> ML-1D · Agente: hefesto-tf · Data: 2026-10-24
> REQ: REQ-2026-10-24 · Roadmap: ROADMAP-2026-10-24-update-regenera-o-…

## Veredito

**APROVADO com recomendações (0 B, 3 R).**
Nenhuma mudança de comportamento foi encontrada para as chaves existentes em `origin/main`.
A reescrita preserva funcionalmente todos os campos da `ProjectConfig` para entradas válidas e
para entradas inválidas (tipo errado): o valor de configuração fica idêntico ao antigo (ausente /
padrão), e a diferença intencional é o registro do erro em `ScalarTypeErrors`.

---

## Como a revisão foi feita

1. Diff completo lido (`git diff origin/main...HEAD -- internal/config/config.go`).
2. Função `parse()` do `origin/main` extraída e comparada chave a chave com a nova.
3. Paths de código para cada categoria de chave analisados: escalares via `scalarKeyTable`,
   listas via `stringListRec`, escalares aninhados via `stringValNested`, containers via
   `recordContainerType`, e `ci_trigger` com verificação direta de presença.
4. Suite de testes completa rodada: `go test ./internal/config/ -count=1` → **PASS** (0.082 s).
5. Testes discriminantes relevantes rodados: `TestScalarType_*`, `TestASTCoverage_NoSilentReads`,
   `TestLoad_CITrigger_*`, `TestConfigLinkFields`, `TestConfigAcceptanceMarkers`.

---

## Tabela de equivalência por chave

Legenda: `=` comportamento idêntico · `+` novo registro de erro (não afeta valor de config)

| Chave | Campo em `ProjectConfig` | Ausente | String válida | Tipo errado (era) | Tipo errado (novo) |
|---|---|---|---|---|---|
| `req_dir` | `REQDir` | `=` padrão | `=` | silêncio | `+` ScalarTypeError |
| `roadmap_dir` | `RoadmapDir` | `=` | `=` | silêncio | `+` |
| `roadmap_namespacing` | `RoadmapNamespacing` | `=` | `=` | silêncio | `+` |
| `governance_mode` | `GovernanceMode` | `=` | `=` | silêncio | `+` |
| `lenient_until` | `LenientUntil` | `=` | `=` | silêncio | `+` |
| `wip_limit` | `WipLimit` | `=` padrão=1 | `=` | silêncio | `+` |
| `wip_by_squad` | `WipBySquad` | `=` | `=` | silêncio | `+` |
| `stale_wip_days` | `StaleWIPDays` | `=` padrão=7 | `=` | silêncio | `+` |
| `require_req_in_commit` | `RequireReqInCommit` | `=` | `=` | silêncio | `+` |
| `trace_id_field` | `TraceIdField` | `=` | `=` | silêncio | `+` |
| `strict_ci_paths` | `StrictCIPaths` | `=` | `=` | silêncio | `+` |
| `forge` | `Forge` | `=` | `=` | silêncio | `+` |
| `hooks` | `Update.Hooks` | `=` | `=` | silêncio | `+` |
| `ci` | `Update.CI` | `=` | `=` | silêncio | `+` |
| `backend` | `Update.Backend` | `=` | `=` | silêncio | `+` |
| `frontend` | `Update.Frontend` | `=` | `=` | silêncio | `+` |
| `pkg_manager` | `Update.PkgManager` | `=` | `=` | silêncio | `+` |
| `agent_conventions` | `Update.AgentConventions` | `=` | `=` | silêncio | `+` |
| `linear_api_key` | `Sync.LinearAPIKey` | `=` | `=` | silêncio | `+` |
| `linear_team_id` | `Sync.LinearTeamID` | `=` | `=` | silêncio | `+` |
| `jira_base_url` | `Sync.JiraBaseURL` | `=` | `=` | silêncio | `+` |
| `jira_email` | `Sync.JiraEmail` | `=` | `=` | silêncio | `+` |
| `jira_token` | `Sync.JiraToken` | `=` | `=` | silêncio | `+` |
| `jira_project` | `Sync.JiraProject` | `=` | `=` | silêncio | `+` |
| `adr_dirs` | `ADRDirs` | `=` | `=` | silêncio | `+` |
| `agents` | `Agents` | `=` | `=` | silêncio | `+` |
| `acceptance_markers` | `AcceptanceMarkers` | `=` | `=` | silêncio | `+` |
| `link_fields` (container) | — | `=` | `=` | silêncio | `+` |
| `link_fields.req/adr/roadmap` | `LinkFields*` | `=` | `=` | silêncio | `+` |
| `rules` (container) | — | `=` | `=` | silêncio | `+` |
| `rules.<rule>` | `Rules[k]` | `=` | `=` silêncio | silêncio | `+` |
| `credential_guard` (container) | — | `=` | `=` | silêncio | `+` |
| `credential_guard.mode` | `CredentialGuard.Mode` | `=` padrão="warn" | `=` | silêncio | `+` |
| `write_scope` (container) | — | `=` | `=` | silêncio | `+` |
| `write_scope.frozen_paths` | `WriteScope.FrozenPaths` | `=` | `=` | silêncio | `+` |
| `write_scope.frozen_paths_subagent` | `WriteScope.FrozenPathsSubagent` | `=` | `=` | silêncio | `+` |
| `agent_models` (container) | — | `=` | `=` | silêncio | `+` |
| `agent_models.<tier>` | `AgentModels[k]` | `=` | `=` | silêncio | `+` |
| `ci_trigger` | `Update.CITrigger*` | n/a (nova) | n/a (nova) | n/a | n/a |

**Notas:**
- YAML null (`~` / bare key) tratado de forma idêntica ao antigo: `"" string` → valor ignorado.
  - Escalares: `stringValRec("") = ("", true)` → setter chamado com ""; `parseInt("", def) = def`
    → WipLimit e StaleWIPDays preservam padrão; booleanos ficam `false`. Idêntico ao antigo.
  - Listas: `stringListRec(nil)` e `stringListRec("")` → `(nil, false)`. Idêntico ao antigo.
  - Containers: `recordContainerType("") = noop`. Idêntico ao antigo.
- Iteração determinística sobre `rules` e `agent_models` (novo `sort.Strings`): o map Go
  continua não-ordenado, mas a ordem de escrita é agora deterministicamente alfabética.
  **Não afeta conteúdo do map** — nenhum consumidor depende de ordem de escrita.
- `ci_trigger` é nova, sem equivalente em `origin/main`. Não há base de comparação.

---

## Chaves aninhadas — preservação confirmada

### `credential_guard.mode`
- Old: `stringVal(cg, "mode"); ok && (mode == "warn" || mode == "block")`
- New: `stringValNested(cg, "mode", "credential_guard.mode", errs); ok` + mesmo enum check
- Paths: string/ausente/não-string/enum-fora → todos produzem o mesmo `CredentialGuard.Mode`.
- String fora do enum (ex: `"deny"`): ambos retornam `ok=true`; ambos reprovam no enum; Mode
  fica em "warn" (padrão). Idêntico.

### `rules.<rule>`
- Old: iteração direta `for k, rv := range rm`; não-strings silenciadas.
- New: iteração ordenada; não-strings registradas. Mapa resultante: idêntico para entradas
  válidas.

### `link_fields.*`
- Old: sem verificação do container; sub-listas via `stringList`.
- New: `recordContainerType` para o container (sem efeito no config); sub-listas via
  `stringListRec`. Resultado: idêntico para entradas válidas.

---

## Análise do `scalarKeyTable` e do teste AST

### Legibilidade da tabela (`internal/config/config.go:716-741`)
A tabela é plana, de um campo por linha, com setters de uma linha. Clara e extensível.
Ponto de atenção: `wip_limit` usa `c.WipLimit` (valor atual do `cfg`) como padrão de
`parseInt` — o que é correto, mas depende de `cfg` ter sido inicializado com `defaults()`.
Callers que constroem `ProjectConfig{}` zero-value (como vários testes) recebem `WipLimit = 0`
como padrão. Isso é comportamento existente antes desta mudança (o setter é idêntico ao
`if v, ok := ...; ok { cfg.WipLimit = parseInt(v, cfg.WipLimit) }` do original).

### Teste `TestASTCoverage_NoSilentReads`

**Arm (a) — proíbe `stringVal(` dentro de `parse()`:** funciona. Falsificação documentada
no próprio arquivo de teste.

**Arm (b) — proíbe `m["literal"]` não-autorizado:** funciona para leituras diretas por
índice. Falsificação documentada.

**Gap identificado (R-3):** o par de braços não captura uma chamada direta
`stringValRec(m, "nova_chave_literal", errs)` dentro de `parse()` fora do loop da tabela.
Arm (a) só verifica `stringVal(`, não `stringValRec(`; arm (b) só verifica `*ast.IndexExpr`
(acesso por índice), não `*ast.CallExpr` com argumento literal. Um developer que adicionasse
tal chamada direta: (a) não apareceria em `KnownScalarKeys()`, portanto não seria coberta
por `TestScalarType_AllKnownKeys`; (b) não seria pega por nenhum dos dois braços do AST.
O mecanismo de ponto único ficaria silenciosamente furado.

---

## Achados

### B — Bloqueantes
Nenhum.

### R — Recomendações

**R-1** (`internal/config/config.go:693-701`) — `stringVal` mantida como função exportável
interna sem marcação de obsolescência.
- O comentário diz que é mantida "para que qualquer reintrodução compile." Mas o compilador
  não impede que outro package (ex. `internal/validator`) a importe no futuro (é exportada
  dentro do package — `config.stringVal` seria não-exportada para fora do package, portanto
  ok). Dentro do package, qualquer arquivo de teste poderia chamá-la diretamente. Sem risco
  imediato; mas um comentário `// Deprecated: use stringValRec.` reduziria a chance de
  reintrodução acidental.

**R-2** (`internal/config/config.go:574-587`) — `ci_trigger: ""` (string vazia explícita ou
`ci_trigger: ~`) marca `CITriggerInvalid = true`.
- `normalizeNode` retorna `""` para qualquer ScalarNode null. A verificação de presença
  `if raw, present := m["ci_trigger"]; present` é `true` para `ci_trigger:` bare (chave
  existe no mapa com valor `""`). `raw.(string) = ""`, `s != "pull_request" && s != "manual"`,
  → `CITriggerInvalid = true`. Usuário que escrevesse `ci_trigger:` como placeholder receberia
  `ci_trigger_invalid` do validate. Comportamento é defensivo e provavelmente correto (ausente
  = pull_request; presente-mas-vazio = inválido), mas sem teste que afirme isso explicitamente.
  Risco: falso bloqueio em projeto que usa `ci_trigger:` bare sem intenção.

**R-3** (`internal/config/config_ast_coverage_test.go:86-133`) — gap no braço (b) do teste AST.
- Como descrito na seção "Análise do teste AST" acima: `stringValRec(m, "literal", errs)`
  chamado diretamente dentro de `parse()` fora do loop da `scalarKeyTable` não seria capturado
  por nenhum dos dois braços. A garantia de "ponto único" ficaria silenciosamente furada. O gap
  pode ser fechado com um terceiro braço que inspeciona `*ast.CallExpr` onde a função é
  `stringValRec` e o segundo argumento é um `*ast.BasicLit` de tipo `token.STRING`.

---

## Contagem de achados
- **B (bloqueantes):** 0
- **R (recomendações):** 3

---

## `git status --short`

```
(clean — sem arquivos modificados no worktree antes desta revisão)
```

Após esta revisão, apenas este arquivo foi criado:
```
?? docs/qualidade/2026-10-24-revisao-parser-config.md
```
