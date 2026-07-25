---
status: wip
date: 2026-07-25
req: "REQ-2026-07-25-identidade-humanizada-dos-agentes-trackfw.md"
squad: "trackfw"
---

# Roadmap: Identidade humanizada dos agentes

> Created: 2026-07-25 | Status: wip

## Acceptance Criteria

- [ ] Sem `~/.trackfw/identity.json`, os artefatos gerados sao **byte a byte
      identicos** aos atuais nos 3 CLIs (nao-regressao)
- [ ] Com identidade configurada, `name` = `<slug>-tf`, `description` prefixado
      pelo `display_name` e corpo cita `display_name` + apelido do usuario
- [ ] `id` canonico, path `trackfw-{{id}}` e chaves do manifest inalterados
- [ ] `agentTools` decide SET_ARCH por `item.ID == "architect"`
- [ ] Slugificacao identica nos 3 CLIs, provada por fixture compartilhada
- [ ] Colisao de `name` no destino gera aviso e exige `--force`
- [ ] Os 4 callers de `BuildPlans` (Go) e equivalentes Node/Python resolvem
      identidade — `update` nao reverte personalizacao
- [ ] `init --identity-preset` funciona e o ramo non-TTY nunca bloqueia
- [ ] Agente nao le configuracao em runtime
- [ ] `make quality` e `trackfw validate` verdes

## Context

REQ: docs/req/REQ-2026-07-25-identidade-humanizada-dos-agentes-trackfw.md
ADR: docs/adr/ADR-2026-07-25-identidade-personalizavel-de-agentes.md
squad: trackfw

Permitir que o usuario nomeie os 10 agentes (`display_name` -> `description` +
corpo; `slug`+`-tf` -> `name`) e defina um apelido pessoal (corpo apenas). A
identidade e materializada em tempo de instalacao por `Render()`; o agente
nunca le configuracao em runtime.

**Contrato compartilhado (ADR D3):** o slug e o schema do config sao contrato
entre os 3 CLIs. Por isso a Wave 1 e a implementacao de referencia em Go e as
portas Node/Python so comecam depois dela — evita que cada CLI invente sua
normalizacao e quebre `check-cli-parity.sh` no final.

### Mapa de dependencias

```
Wave 1 (paralelo)          Wave 2        Wave 3        Wave 4 (paralelo)     Wave 5
ML-1A identity Go  ──┐
                     ├──►  ML-2A  ──►  ML-3A  ──┬──►  ML-4A npm      ──┐
ML-1B assets      ───┘     render        CLI    └──►  ML-4B pypi     ──┴──► ML-5A
                           plan/manager   wizard                            gates+docs
```

---

## Wave 1 — Contrato e assets (2 MLs em paralelo)
> Dependencies: none — arquivos disjuntos

### ML-1A — Pacote `internal/identity` (referencia do contrato)
**Status:** pending
**Agente:** trackfw-backend
**Files affected:** `internal/identity/identity.go`, `internal/identity/slug.go`,
`internal/identity/preset.go`, `internal/identity/identity_test.go`,
`internal/identity/slug_test.go`

**Actions:**
1. `Config` com `schema_version` (int, valor 1), `user_nickname` (string),
   `agents` (`map[string]AgentIdentity{DisplayName, Slug string}`).
2. `Load(homeDir string) (Config, error)` lendo `~/.trackfw/identity.json`.
   Arquivo ausente -> `Config` zero **sem erro**. `schema_version` != 1 -> erro.
3. `Save(homeDir string, cfg Config) error` com escrita atomica, modo `0o600`
   (espelhar `writeManifest` em `internal/integrations/manifest.go`).
4. `Slugify(input string) (string, error)` conforme ADR D3.2: NFD +
   remocao de diacriticos (ASCII-fold), lowercase, `[ _]` -> `-`, descarte de
   caracteres fora de `[a-z0-9-]`, colapso de `-{2,}`, trim de `-`.
   Vazio pos-normalizacao **ou** > 40 chars -> erro explicito.
5. `PresetGreek()` com slugs **hardcoded** (nunca derivados):
   `architect`→(`Zeus`,`zeus`), `backend`→(`Apolo`,`apolo`),
   `frontend`→(`Afrodite`,`afrodite`), `qa`→(`Ártemis`,`artemis`),
   `infra`→(`Ares`,`ares`), `security`→(`Hades`,`hades`),
   `dba`→(`Poseidon`,`poseidon`), `ux`→(`Atena`,`atena`),
   `code-quality`→(`Hefesto`,`hefesto`), `data`→(`Métis`,`metis`).
6. `AgentName(slug string) string` -> `slug + "-tf"`. Sufixo aplicado em um
   unico ponto.
7. `Validate(cfg Config) error`: rejeita slugs duplicados entre os 10 ids e
   ids desconhecidos.
8. **Tabela de vetores de teste** exportada como fixture JSON em
   `internal/identity/testdata/slug_vectors.json`, cobrindo no minimo:
   `"Ártemis"`→`artemis`, `"Zeus"`→`zeus`, `"Meu Agente"`→`meu-agente`,
   `"Métis"`→`metis`, `"  Zeus  "`→`zeus`, `"a__b"`→`a-b`, `"a--b"`→`a-b`,
   `"🌩️"`→erro, `""`→erro, `"---"`→erro, string de 41 chars→erro.
   Essa fixture sera **copiada byte a byte** para npm e pypi nas Waves 4.

**Acceptance criteria:**
- [ ] `go build ./...` sem erros
- [ ] `go test ./internal/identity/...` verde
- [ ] `go vet ./...` limpo
- [ ] Fixture `slug_vectors.json` existe e e consumida pelo teste
- [ ] Nenhum arquivo fora de `internal/identity/` modificado

**Comandos de validação:** `go build ./... && go test ./internal/identity/... && go vet ./...`

---

### ML-1B — Placeholders de identidade nos assets dos agentes
**Status:** pending
**Agente:** trackfw-backend
**Files affected:** `internal/integrations/assets/agents/*.md` (10 arquivos),
`npm/src/integrations/assets/agents/*.md`, `pypi/trackfw/integrations/assets/agents/*.md`

**Actions:**
1. Em cada um dos 10 assets em `internal/integrations/assets/agents/`,
   inserir **no inicio do corpo** (apos o frontmatter) uma linha de identidade
   com placeholders:
   `{{IDENTITY_LINE}}`
   Nao alterar mais nada do corpo nem do frontmatter existente.
2. O placeholder e substituido por texto vazio quando nao ha identidade
   configurada — garantindo saida **byte a byte identica** a atual. Documentar
   isso em comentario no proprio ML (o consumo e feito no ML-2A).
3. Rodar `scripts/sync-integration-assets.sh` para propagar aos 3 pacotes.

**Acceptance criteria:**
- [ ] Os 3 diretorios de assets tem MD5 identico por arquivo
- [ ] `scripts/check-integration-assets.sh` passa
- [ ] Nenhum arquivo `.go`, `.js` ou `.py` modificado

**Comandos de validação:** `scripts/sync-integration-assets.sh && scripts/check-integration-assets.sh`

---

## Wave 2 — Integracao no pipeline de render (1 ML)
> Dependencies: **barrier — ML-1A e ML-1B completos**

### ML-2A — `Render`/`BuildPlans`/colisao + fim da heuristica por nome
**Status:** pending
**Agente:** trackfw-backend
**Files affected:** `internal/integrations/render.go`, `internal/integrations/plan.go`,
`internal/integrations/manager.go`, `internal/integrations/render_test.go`,
`internal/integrations/manager_test.go`

**Actions:**
1. `PlanRequest` ganha campo `Identity identity.Config`.
2. `Render` ganha o parametro de identidade e aplica, **quando houver entrada
   para o `item.ID`**:
   - `name` -> `identity.AgentName(slug)` (ex: `zeus-tf`)
   - `description` -> `"<DisplayName> — " + description` original
   - `{{IDENTITY_LINE}}` -> frase de identidade citando `display_name` e, se
     houver, `user_nickname`.
   Sem entrada: `name`/`description` inalterados e `{{IDENTITY_LINE}}` -> `""`,
   **sem deixar linha em branco extra**.
3. **`agentTools` passa a receber `item.ID`** e decidir SET_ARCH por
   `item.ID == "architect"`. Remover `strings.HasSuffix(name, "architect")`.
4. `manager.go`: antes de escrever um agente, varrer o diretorio de destino
   por outros arquivos declarando o mesmo `name`; colisao -> erro claro
   citando o arquivo conflitante, contornavel com `force`.
5. Testes de **nao-regressao**: com `identity.Config` zero, a saida de `Render`
   e byte a byte igual a atual, para as 5 representacoes.

**Acceptance criteria:**
- [ ] `go test ./internal/integrations/...` verde
- [ ] Teste explicito prova saida identica sem identidade (5 representacoes)
- [ ] Teste prova `SET_ARCH` mantido com `name` customizado (`zeus-tf`)
- [ ] Teste prova erro de colisao e bypass com `force`
- [ ] `go build ./... && go vet ./...` limpos

**Comandos de validação:** `go build ./... && go test ./internal/integrations/... && go vet ./...`

---

## Wave 3 — CLI e wizard Go (1 ML)
> Dependencies: **barrier — ML-2A completo**

### ML-3A — Wizard `init`, flag e wiring dos 4 callers
**Status:** pending
**Agente:** trackfw-backend
**Files affected:** `internal/commands/init.go`,
`internal/commands/integrations_flags.go`, `internal/generators/update.go`,
`internal/i18n/locales/{pt-BR,en-US,es-ES}.json`, testes correspondentes

**Actions:**
1. Resolver `identity.Load(home)` e injetar em `PlanRequest.Identity` nos
   **4 callers**: `integrations_flags.go:136` (mutation),
   `integrations_flags.go:178` (list), `init.go:274`, `generators/update.go:144`.
2. `init` ganha `--identity-preset` com valores `greek|neutral|none`
   (default `none`). `neutral`/`none` -> nenhuma identidade gravada.
3. Grupo `huh` novo no wizard interativo de `init`:
   - select: `Preset: Panteão grego` | `Nomes neutros (padrão)` |
     `Personalizar um a um` | `Pular`
   - se `Personalizar um a um`: 10 inputs de `display_name`
   - input opcional: apelido do usuario
   Persistir via `identity.Save`.
4. **Ramo `!IsTerminal` nao pode exibir prompt** — respeita apenas a flag.
5. Re-executar `init` com identidade ja persistida **reutiliza** o config e
   nao re-pergunta (a nao ser que a flag seja passada explicitamente).
6. Chaves i18n novas nos 3 locales.

**Acceptance criteria:**
- [ ] `go build ./... && go test ./... && go vet ./...` verdes
- [ ] Teste prova que os 4 callers repassam a identidade
- [ ] Teste prova que `init` nao-TTY nao bloqueia e respeita a flag
- [ ] Teste prova que `init` re-executado nao sobrescreve identidade existente
- [ ] As 3 locales tem exatamente o mesmo conjunto de chaves

**Comandos de validação:** `go build ./... && go test ./... && go vet ./... && make lint`

---

## Wave 4 — Paridade Node.js e Python (2 MLs em paralelo)
> Dependencies: **barrier — ML-3A completo**. Diretorios disjuntos entre si.

### ML-4A — Porta Node.js
**Status:** pending
**Agente:** trackfw-frontend
**Files affected:** `npm/src/identity/*.js`, `npm/src/integrations/render.js`,
`npm/src/integrations/index.js`, `npm/src/commands/update.js`,
`npm/src/commands/init.js`, `npm/src/i18n/locales/*.json`, testes npm,
`npm/test/fixtures/slug_vectors.json`

**Actions:**
1. Portar `identity` (schema, `Load`/`Save`, `Slugify`, preset grego hardcoded,
   `AgentName`, `Validate`) com **comportamento identico** ao Go.
2. Copiar `internal/identity/testdata/slug_vectors.json` **byte a byte** e
   consumi-lo na suite npm.
3. Aplicar identidade em `render.js` e propagar em `buildPlans`
   (`npm/src/integrations/index.js:41`) e nos callers
   (`index.js:92`, `commands/update.js:80`).
4. Mesma decisao de `agentTools` por `item.id`, mesma deteccao de colisao.
5. Wizard/flag `--identity-preset` no `init` do npm, sem bloquear em non-TTY.
6. Chaves i18n nos 3 locales npm.

**Acceptance criteria:**
- [ ] `npm test` verde no workspace npm
- [ ] Vetores de slug produzem resultado identico ao Go (fixture compartilhada)
- [ ] Teste de nao-regressao: sem identidade, saida byte a byte igual a atual
- [ ] Nenhum arquivo fora de `npm/` modificado

**Comandos de validação:** `cd npm && npm test`

---

### ML-4B — Porta Python
**Status:** pending
**Agente:** trackfw-data
**Files affected:** `pypi/trackfw/identity/*.py`,
`pypi/trackfw/integrations/renderers.py`, `pypi/trackfw/integrations/catalog.py`,
`pypi/trackfw/integrations/command.py`, `pypi/trackfw/i18n/locales/*.json`,
testes pypi, `pypi/tests/fixtures/slug_vectors.json`

**Actions:**
1. Portar `identity` com comportamento identico ao Go (mesmo schema, mesma
   slugificacao via `unicodedata.normalize("NFD", ...)`, preset hardcoded).
2. Copiar a fixture `slug_vectors.json` **byte a byte** e consumi-la.
3. Aplicar identidade em `renderers.py` e propagar no construtor de planos e
   em todos os callers equivalentes aos 4 pontos do Go.
4. Mesma decisao de `agentTools` por `item.id`, mesma deteccao de colisao.
5. Wizard/flag `--identity-preset` no `init`, sem bloquear em non-TTY.
6. Chaves i18n nos 3 locales pypi.

**Acceptance criteria:**
- [ ] Suite Python verde
- [ ] Vetores de slug identicos ao Go (fixture compartilhada)
- [ ] Teste de nao-regressao: sem identidade, saida byte a byte igual a atual
- [ ] Nenhum arquivo fora de `pypi/` modificado

**Comandos de validação:** `make test-python`

---

## Wave 5 — Gates de paridade e documentacao (1 ML)
> Dependencies: **barrier — ML-4A e ML-4B completos**

### ML-5A — Gates, teste de paridade cross-CLI e docs
**Status:** pending
**Agente:** trackfw-qa
**Files affected:** `scripts/check-cli-parity.sh`, `docs/cli-parity.md`,
`README.md`, `npm/README.md`, `pypi/README.md`, `docs/agents-working-context.md`

**Actions:**
1. Estender `scripts/check-cli-parity.sh` para provar que os 3 CLIs geram o
   **mesmo artefato** para a mesma `identity.json` (comparacao de hash).
2. Verificar que as 3 fixtures `slug_vectors.json` sao byte-identicas.
3. Documentar a feature em `docs/cli-parity.md` (secao de identidade) e nos
   3 READMEs: config, flag, preset, sufixo `-tf`, e o fato de que o agente
   **nao le config em runtime**.
4. Atualizar `docs/agents-working-context.md`.

**Acceptance criteria:**
- [ ] `make quality` verde (Go + Node + Python + paridade)
- [ ] `trackfw validate` sem violations
- [ ] Gate falha propositalmente se um CLI divergir (verificado manualmente)
- [ ] Documentacao dos 3 pacotes cita `~/.trackfw/identity.json`

**Comandos de validação:** `make quality && trackfw validate`

---

## Legenda de status
- pending / in_progress / done / blocked
