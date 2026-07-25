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
Wave 1 (paralelo)              Wave 2        Wave 3      Wave 4 (paralelo)   Wave 5
ML-1A identity Go ──► ML-1C ──┐
      (mesmo preset.go)       ├──► ML-2A  ──► ML-3A ──┬──► ML-4A npm  ──┐
ML-1B assets ─────────────────┘    render      CLI    └──► ML-4B pypi ──┴──► ML-5A
                                   plan        wizard                       gates
                                   manager                                  docs
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
**Status:** ABANDONADO — abordagem revertida (commit `9ef17b3` revertido)

> **Motivo da reversao.** O placeholder `{{IDENTITY_LINE}}` foi inserido nos 10
> assets e propagado aos 3 pacotes. A auditoria revelou que `Render()` tem
> **duas rotas** e o placeholder so seria removido em uma delas:
>
> - `custom-agent-toml`, `cli-agent-json`, `agent-json`, `agent-directory`
>   passam por `markdownParts()` e separam frontmatter de corpo;
> - o branch `default:` (`representation: "subagent"`) devolve
>   `normalizeMarkdown(source)` — **o source cru**.
>
> A superficie `claude` usa `subagent`. Verificacao empirica confirmou que
> `trackfw agents install` gravaria `{{IDENTITY_LINE}}` literal em
> `~/.claude/agents/trackfw-architect.md` — na superficie mais usada do
> produto, sem nenhum teste cobrindo. Apenas 2 testes Node (goldens inline)
> pegaram o problema, e nas rotas menos usadas.
>
> Manter o placeholder exigiria **strip correto em 2 rotas x 3 CLIs = 6
> implementacoes**, cada uma um ponto de vazamento silencioso. Como nenhum
> asset precisa de posicionamento diferente de "inicio do corpo", o
> placeholder comprava um ancoramento posicional que nao seria usado.
>
> **Nova abordagem (ver ML-2A):** os assets permanecem intocados e `Render()`
> **insere** a linha de identidade apos o terminador do frontmatter quando ha
> identidade configurada. O criterio de nao-regressao passa a ser verdadeiro
> **por construcao** — sem identidade, nenhum codigo novo executa — em vez de
> depender de 6 implementacoes corretas de remocao.

**Status original:** done (revertido)
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

### ML-1C — Catalogo de 5 presets tematicos + modo livre
**Status:** pending
**Agente:** trackfw-backend
**Dependencies:** ML-1A completo (mesmo arquivo `preset.go`)
**Files affected:** `internal/identity/preset.go`, `internal/identity/preset_test.go`

**Actions:**
1. Generalizar `PresetGreek()` para `Preset(name string) (Config, error)` com
   os ids `greek`, `norse`, `potter`, `thrones`, `chaves`. Manter
   `PresetNames() []string` nessa ordem.
2. Tabelas **hardcoded** (display_name / slug), conforme ADR D3-bis:

| id | greek | norse | potter | thrones | chaves |
|---|---|---|---|---|---|
| architect | Zeus / zeus | Odin / odin | Dumbledore / dumbledore | Tyrion / tyrion | Girafales / girafales |
| backend | Apolo / apolo | Thor / thor | Snape / snape | Jon / jon | Madruga / madruga |
| frontend | Afrodite / afrodite | Freya / freya | Luna / luna | Sansa / sansa | Chiquinha / chiquinha |
| qa | Ártemis / artemis | Heimdall / heimdall | Moody / moody | Arya / arya | Florinda / florinda |
| infra | Ares / ares | Tyr / tyr | Hagrid / hagrid | Brienne / brienne | Barriga / barriga |
| security | Hades / hades | Vidar / vidar | Kingsley / kingsley | Varys / varys | Quico / quico |
| dba | Poseidon / poseidon | Njord / njord | Flitwick / flitwick | Samwell / samwell | Clotilde / clotilde |
| ux | Atena / atena | Idun / idun | Tonks / tonks | Margaery / margaery | Popis / popis |
| code-quality | Hefesto / hefesto | Bragi / bragi | Hermione / hermione | Stannis / stannis | Nhonho / nhonho |
| data | Métis / metis | Mimir / mimir | Trelawney / trelawney | Bran / bran | Godinez / godinez |

3. `Preset` com nome desconhecido -> erro citando os nomes validos.
4. Modo livre (`custom`) **nao** e um preset: e coletado pelo wizard no ML-3A e
   usa `Slugify`. Nao implemente wizard aqui.

**Acceptance criteria:**
- [ ] Cada preset cobre exatamente os 10 ids de `KnownAgentIDs()`
- [ ] Teste prova que nenhum preset tem slug duplicado internamente
- [ ] Teste prova que todo preset passa em `Validate`
- [ ] Teste prova que `Preset("inexistente")` retorna erro
- [ ] `go build ./... && go test ./internal/identity/... && go vet ./...` verdes
- [ ] Nenhum arquivo fora de `internal/identity/` modificado

**Comandos de validação:** `go build ./... && go test ./internal/identity/... && go vet ./...`

---

## Wave 2 — Integracao no pipeline de render (1 ML)
> Dependencies: **barrier — ML-1A, ML-1B e ML-1C completos**

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
   - corpo -> **insercao** de uma frase de identidade citando `display_name` e,
     se houver, `user_nickname`.
   **Sem entrada para o `item.ID`, nenhum codigo novo executa** — retorno
   identico ao atual por construcao (nao ha placeholder a remover).
2-bis. A insercao no corpo precisa cobrir **as duas rotas** de `Render`:
   - rotas que passam por `markdownParts()` (`custom-agent-toml`,
     `cli-agent-json`, `agent-json`, `agent-directory`): prefixar o `body`;
   - branch `default:` (`representation: "subagent"`, usado por **claude**,
     gemini, cursor, copilot, kiro-ide, windsurf): o source cru e devolvido por
     `normalizeMarkdown`. Inserir apos o terminador do frontmatter, reusando a
     mesma logica de `strings.Index(text[4:], "\n---")` ja presente em
     `markdownParts`. **Esquecer esta rota vaza texto no arquivo instalado da
     superficie mais usada do produto** — foi exatamente a falha que derrubou
     a abordagem do ML-1B.
3. **`agentTools` passa a receber `item.ID`** e decidir SET_ARCH por
   `item.ID == "architect"`. Remover `strings.HasSuffix(name, "architect")`.
4. `manager.go`: antes de escrever um agente, varrer o diretorio de destino
   por outros arquivos declarando o mesmo `name`; colisao -> erro claro
   citando o arquivo conflitante, contornavel com `force`.
5. Testes de nao-regressao com **goldens congelados**, nunca auto-referentes.
   `Render(x) == Render(x)` nao prova nada — foi por isso que a suite Go
   permaneceu verde enquanto os assets mudavam sob ela no ML-1B. Os goldens
   devem ser **strings capturadas do estado pre-mudanca**:
   - `custom-agent-toml`: literal `expected` de `npm/tests/agents-skills.test.js:271`
   - `agent-directory`: literal `expected` de `npm/tests/agents-skills.test.js:404`
   - `default:`/`subagent`: capturar de `git show 5fe5cb9:internal/integrations/assets/agents/architect.md`
   Gravar em `internal/integrations/testdata/` e comparar byte a byte.
   Isso tambem fecha a lacuna preexistente: hoje **nao existe** cobertura de
   golden para bytes renderizados em Go.

**Acceptance criteria:**
- [ ] `go test ./internal/integrations/...` verde
- [ ] Goldens congelados em `testdata/` cobrem as rotas `subagent`,
      `custom-agent-toml` e `agent-directory`
- [ ] Teste prova que a rota `subagent` (claude) recebe a linha de identidade
      quando ha identidade configurada
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
2. `init` ganha `--identity-preset` com valores
   `greek|norse|potter|thrones|chaves|neutral|none` (default `none`).
   `neutral`/`none` -> nenhuma identidade gravada. Valor invalido -> erro
   listando os aceitos.
3. Grupo `huh` novo no wizard interativo de `init`:
   - select com 7 opcoes:
     `Panteão grego (Zeus, Apolo, Afrodite...)` |
     `Mitologia nórdica (Odin, Thor, Freya...)` |
     `Harry Potter (Dumbledore, Snape, Luna...)` |
     `Game of Thrones (Tyrion, Jon, Arya...)` |
     `Chaves (Girafales, Madruga, Chiquinha...)` |
     `Personalizar um a um` | `Nomes neutros (padrão)`
   - se `Personalizar um a um`: 10 inputs de `display_name`, cada um validado
     por `identity.Slugify` no `Validate` do campo `huh` (erro inline, sem
     correcao silenciosa); slug duplicado tambem e erro inline
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
