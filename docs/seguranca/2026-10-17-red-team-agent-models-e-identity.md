# Red-team Wave 1 — Validação de valor `agent_models` e YAML-safe identity

> Hades (Security) · 2026-10-17
> Branch: `fix/validacao-de-valor-e-precisao-de-mensagem-agent-models`
> REQ: docs/req/REQ-2026-08-30-validacao-de-valor-e-precisao-de-mensagem-agent-models-sem-sanitizacao-e-ancoragem-de-til-imprecisa.md

## Veredito

**BLOQUEIA** — uma falha de gate impede `make quality` de passar. O patch de produto é correto; a lacuna está no artefato de documentação.

Nenhuma regressão de segurança encontrada. Nenhum falso positivo para modelo legítimo. YAML inválido não gerado por nenhuma entrada aceita.

---

## Bloqueante

### B-1 — `make quality` falha: seção sem anotação `trackfw-contract` em `docs/cli-parity.md`

**Arquivo:** `docs/cli-parity.md`, linha 7734
**Observado:** `scripts/check-parity-contract-coverage.sh` reporta `#### Residuais declarados (REQ-2026-08-30)` como seção sem anotação. `make quality` sai com exit != 0.
**Causa:** a seção pai `### Validação de valor de agent_models...` tem a anotação (linha 7724), mas o sub-heading `####` não tem. O script exige anotação em todos os níveis 2–4.
**Reprodução:**
```bash
make quality 2>&1 | grep -A3 "seções sem anotação"
# -- seções sem anotação (reprovam — bloqueante desde o ML-3A) --
#   - #### Residuais declarados (REQ-2026-08-30) (linha 7734)
```
**Fix mínimo:** inserir imediatamente após a linha `#### Residuais declarados (REQ-2026-08-30)`:
```html
<!-- trackfw-contract: none reason=residuals documentation — esta seção declara lacunas aceitas por decisão de projeto, não um contrato comportamental com gate de teste -->
```
**Não alterar código de produto** — o fix é em `docs/cli-parity.md`.

---

## Tabela de resultados por item

| # | Vetor / comando | Deveria | Branch | main |
|---|----------------|---------|--------|------|
| 1a | `agents install` + valor inválido (`5: danger`) | exit 1, sem arquivo | exit 1, 0 arquivos | exit 0, arquivo com YAML corrompido |
| 1b | `agents update` + valor inválido | exit 1, sem arquivo | exit 1, 0 arquivos | exit 0, arquivo gravado |
| 1c | `update harness` + agentes pré-instalados + valor inválido | exit 1, agentes existentes intactos | exit 1, agentes mantêm valor anterior | exit 0, agentes atualizados com valor corrompido |
| 1d | `agents install` com vários alvos (`claude,cursor`) | claude falha, cursor não afetado | confirmado (cursor BYTE-IDENTICAL ao main) | n/a |
| 2 | Project-scope `trackfw.yaml` com valor inválido | exit 1, sem arquivo | exit 1, 0 arquivos | exit 0, arquivo gravado |
| 3a | `yamlSafeScalar("Zeus: danger")` | `"Zeus: danger"` (string YAML) | `"Zeus: danger"` | literal `Zeus: danger` (YAML PARSE ERROR) |
| 3b | `yamlSafeScalar("Apolo #tf")` | `"Apolo #tf"` (string YAML) | `"Apolo #tf"` | literal `Apolo #tf` (comment truncation) |
| 3c | `yamlSafeScalar("\\")` | `\` (plain scalar, válido) | `\` | n/a (função nova) |
| 3d | `yamlSafeScalar("Normal Name")` | `Normal Name` (sem aspas) | `Normal Name` | `Normal Name` (comportamento preservado) |
| 3e | `yamlSafeScalar("")` | `""` (string vazia cotada) | `""` | n/a |
| 3f | `yamlSafeScalar("null")` em name/description | `"null"` (string) | `"null"` | literal `null` (tipo nil) |
| 4 | `[a, b]` como valor de description | string `"[a, b]"` (cotado) | `"[a, b]"` | n/a |
| 4 | `{a: b}` como valor de description | string `"{a: b}"` (cotado) | `"{a: b}"` | n/a |
| 4 | `a, b` como valor de name | string `a, b` (plain scalar) | `a, b` | n/a |
| 5 | Cursor com `sonnet: "4.6"` — todos 12 agentes | byte-idêntico ao main | BYTE-IDENTICAL (12/12) | baseline |
| 5 | Claude com `sonnet: "4.6"` — model line | `model: "claude-sonnet-4-6"` (cotado, D-2) | `model: "claude-sonnet-4-6"` | `model: claude-sonnet-4-6` (sem aspas) |
| 6 | `agents models` com valores válidos | exit 0 (sem regressão) | exit 0 | exit 0 |
| 6 | `agents models` com valor inválido | exit 1, tabela exibe `[invalid]` | exit 1, tabela completa | exit 0, sem indicação |

**Nota B-1:** `make quality` falha — ver seção Bloqueante.

---

## Resultados por item (narrativa)

### Item 1 — Os outros call sites

**`agents install`** (5 vetores inválidos, medidos): todos exit 1, zero arquivos criados. Confirmados: `5 && rm -rf /`, `5: danger`, `{tools: Bash}`, `claude-opus-5 #x`, `a b`.

**`cl"x` como valor:** quando escrito no `trackfw.yaml` como `sonnet: "cl"x"` (YAML malformado), o config parse falha na camada de `LoadGlobalAgentModels` → silent un-pin → exit 0, agentes instalados sem pin. Este é o comportamento pré-existente T-3 (declarado como resíduo R-2 no Wave 0) — a nova validação não se aplica porque o valor nunca chega ao `isValidModelPinValue`. Quando o YAML é corretamente encodado (`sonnet: 'cl"x'` ou `sonnet: "cl\"x"`), o valor chega à validação e exit 1 é retornado sem arquivo. A medição do arquiteto usou a forma corretamente encodada.

**`agents update`:** exit 1, zero arquivos. Passa pelo mesmo `BuildPlans` → `Render`. FECHA.

**`update harness`:** exit 1. Agentes pré-instalados mantêm o valor anterior (`model: "claude-sonnet-4-6"`). O harness falha o sub-componente `claude-agents` e reporta o erro; scripts e credential guard são atualizados (não dependem de `agent_models`). AC1 "sem gerar arquivo" interpretado como "sem gravar valor inválido em arquivo de agente": FECHA.

**`agents third-party install/update`:** call sites em `integrations_thirdparty.go:333,522` passam `AgentModels` para `BuildPlans` → mesma validação. Não testável de ponta a ponta sem artefato quarentenado; FECHA POR CONSTRUÇÃO.

**Múltiplos alvos:** `agents install --targets claude,cursor` com valor inválido para `sonnet` — claude falha, cursor continua e instala todos os 12 agentes normalmente (Cursor não usa `agentModels`). Nenhum arquivo claude é gravado. FECHA.

### Item 2 — Fontes do valor

Project-scope `trackfw.yaml` com `agent_models.sonnet: "5: danger"`: exit 1, zero arquivos. Passa pelo mesmo caminho de resolução. FECHA.

### Item 3 — Escape de YAML

`yamlEscapeDoubleQuoted` escapa `"` → `\"` e `\` → `\\`. Não escapa U+2028/U+2029 explicitamente — esses caracteres, quando presentes, são detectados por `containsControlChar` (que os checa em `needsYAMLQuoting`), causando quoting. U+2028/U+2029 em double-quoted YAML strings são válidos em yaml.v3 (medido). Não chegam ao path porque `identity.Load` os rejeita via `containsControlCharIdent`. Defesa em profundidade adequada.

Todos os vetores obrigatórios produzem YAML válido e tipo string. Nomes comuns são byte-idênticos ao original (sem aspas desnecessárias).

### Item 4 — A remoção da vírgula

`a, b` como plain scalar YAML (`name: a, b`) parseia como string "a, b" — correto, vírgula não é metacaractere em plain scalars YAML. `[a, b]` e `{a: b}` contêm `[` e `{` respectivamente — ambos na switch do `needsYAMLQuoting` — e são emitidos cotados. FECHA.

### Item 5 — Cursor e os demais alvos

12/12 arquivos Cursor: BYTE-IDENTICAL entre branch e main.

Claude: única diferença é `model: "claude-sonnet-4-6"` vs `model: claude-sonnet-4-6` — mudança intencional D-2, documentada no CHANGELOG. Semanticamente equivalente (yaml.v3 parseia ambos como string).

### Item 6 — `agents models`

Comportamento novo documentado no CHANGELOG: exit != 0 quando há tier inválido. Comportamento com valores válidos: exit 0 (igual ao main). Nenhum script que consume `agents models` e verifica exit code seria afetado por valores válidos.

### Item 7 — Os testes novos

| Teste | O que afirma | Reprovaria sem a correção? |
|-------|-------------|--------------------------|
| `TestValidateAgentModelValue` | O allowlist rejeita os vetores da Seção 3 do Wave 0 e aceita o corpus de escape-hatch | Não compilaria em main (`ValidateAgentModelValue` inexistente) |
| `TestRenderRejectsInvalidModelValueAndWritesNoFile` | `Render` retorna erro e não grava arquivo para valor com espaço | Reprovaria em main (`Render` retornava nil error) |
| `TestLoadRejectsControlCharInDisplayName` | `Load` rejeita `\n` em `display_name` e `user_nickname` | Reprovaria em main (check inexistente) |
| `TestContainsControlCharIdent` | `containsControlCharIdent` detecta tab, newline, null — não detecta colon-space ou hash (intenção D-3) | Não compilaria em main (função inexistente) |

Todos os testes passam na branch. Todos os testes do repositório: `go test ./...` verde (medido).

---

## Residuais declarados neste review

| ID | Descrição | Classificação |
|----|-----------|--------------|
| R-1 | `cl"x` em YAML malformado (`sonnet: "cl"x"`) → silent un-pin. Pré-existente (T-3/R-2 do Wave 0). O valor correto (`'cl"x'`) é rejeitado. | Resíduo pré-existente, declarado |
| R-2 | `0755` como display name puro produziria `name: 0755` (integer YAML). Na prática impossível: `name` usa o slug, `description` é sempre `DisplayName + " — " + template`. | Resíduo teórico, não atingível pelo fluxo atual |
| R-3 | `yamlEscapeDoubleQuoted` não escapa U+2028/U+2029 explicitamente. Defesa upstream em `identity.Load` impede que cheguem ao path. | Resíduo com mitigação upstream |
| R-4 | `needsYAMLQuoting` não cobre valores puramente numéricos que YAML 1.1 parseia como octal (`0755`) ou float (`1e3`). Não atingível como `name:` (slug) nem como `description:` isolado. | Resíduo teórico |

---

## Fix requerido antes do PR

**Apenas um fix de documentação:**

Em `docs/cli-parity.md`, linha 7734, inserir após `#### Residuais declarados (REQ-2026-08-30)`:

```html
<!-- trackfw-contract: none reason=residuals documentation — esta seção declara lacunas aceitas por decisão de projeto, não um contrato comportamental com gate de teste -->
```

Após o fix: `make quality` deve sair com exit 0.
