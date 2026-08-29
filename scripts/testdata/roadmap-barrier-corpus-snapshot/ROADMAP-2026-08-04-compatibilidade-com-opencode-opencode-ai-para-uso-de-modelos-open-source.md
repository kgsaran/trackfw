---
status: done
date: 2026-08-04
req: "docs/req/REQ-2026-08-04-compatibilidade-com-opencode-opencode-ai-para-uso-de-modelos-open-source.md"
squad: "prometeu-tf"
---

# Roadmap: compatibilidade com OpenCode (opencode.ai) para uso de modelos open-source

> Created: 2026-08-04 | Status: done

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: `docs/req/REQ-2026-08-04-compatibilidade-com-opencode-opencode-ai-para-uso-de-modelos-open-source.md`

Aplica o padrão de adapter já estabelecido pelo ADR-2026-07-18 (catálogo canônico +
`internal/integrations/assets/catalog.json`) a um 10º target: OpenCode (opencode.ai), CLI/TUI de
agente de IA com suporte nativo a 75+ provedores via AI SDK, incluindo modelos open-source
self-hosted (Ollama, LM Studio, llama.cpp) — motivação de negócio deste REQ. Escopo desta primeira
fase: lifecycle `agents`/`skills` (list/install/uninstall/update) + reuso do `AGENTS.md` já existente.
MCP servers, hooks de atenção (plugin JS) e wizard de provider ficam fora (ver negative scope da REQ).

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ] Target `opencode` no catálogo canônico, agents+skills instaláveis/atualizáveis/desinstaláveis
      nos 3 CLIs via `--targets opencode`
- [x] Decisão de representação de agente documentada (reuso vs nova) antes de tocar código de produção
- [ ] `AGENTS.md` confirmado funcionando para projetos OpenCode sem mudança de detecção (ou corrigido
      se a prática divergir da leitura de código)
- [ ] Assets Go canônicos, cópias npm/PyPI byte-idênticas
- [ ] `docs/cli-parity.md` atualizado
- [ ] `make quality` verde, `trackfw validate` sem violações

## Wave 1 — Pesquisa e decisão de representação (bloqueia as waves seguintes)
> Dependencies: none

### ML-1A — Validar o formato real de agente/skill do OpenCode e decidir a representação
**Status:** ✅ Concluído
**Files affected:** nenhum arquivo de produção — só pesquisa e a decisão registrada nesta seção do
roadmap (e num ADR complementar, se necessário)
**Actions:**
1. Instalar o OpenCode real (`npm install -g opencode-ai` ou equivalente conforme a documentação
   oficial em opencode.ai/docs) e criar um projeto de teste mínimo com um agente custom em
   `.opencode/agents/teste.md` e uma skill em `.opencode/skills/teste/SKILL.md`, confirmando que o
   OpenCode de fato os reconhece (`opencode` CLI real, não só a documentação) — isso valida os
   caminhos e o frontmatter descritos no REQ contra o comportamento real, não só a doc.
2. Comparar o frontmatter de agente do OpenCode (`description`, `mode`, `model`, `permission`, etc.)
   com a representação `agent-markdown`/`subagent` já usada em `internal/integrations/render.go` —
   decidir se a função `Render` existente produz um arquivo válido para o OpenCode com pequenos
   ajustes, ou se é necessária uma nova `Representation` (ex: `"opencode-agent"`).
3. Confirmar experimentalmente (não só por leitura de código) que `AGENTS.md` já existente é lido
   pelo OpenCode real do jeito que a documentação promete (precedência, combinação com
   `~/.config/opencode/AGENTS.md`).
4. Registrar a decisão por escrito nesta seção do roadmap (atualizar este ML com o resultado) e, se a
   representação exigir mudança de esquema em `render.go` maior que "adicionar um case", abrir um ADR
   complementar antes da Wave 2.
**Acceptance criteria:**
- [x] Decisão de representação registrada com justificativa
- [x] Comportamento real do OpenCode (não só doc) confirmado para agents, skills e AGENTS.md

---

### Achados da pesquisa (2026-08-05) — ambiente real, OpenCode 1.18.13

Ambiente: `opencode --version` → `1.18.13` (`/opt/homebrew/bin/opencode`), já instalado na máquina,
com `~/.config/opencode/opencode.json` configurando um provider `lmstudio` local (confirma a
motivação de negócio do REQ — o usuário já usa OpenCode com LM Studio). Projeto de teste isolado
criado em `/tmp` (fora do repo, removido ao final) com `git init`, `.opencode/agents/teste.md`,
`.opencode/skills/teste/SKILL.md` e `AGENTS.md` na raiz.

#### 1. Agents — reconhecimento real confirmado

Comando: `opencode agent list` (lista todos os agentes carregados, com suas regras de permissão) e
`opencode debug agent teste` / `opencode debug config` (mostram a configuração resolvida de um
agente específico ou de todos).

`opencode agent list` no projeto de teste retornou, entre os agentes nativos (`build`, `plan`,
`explore`, `general`, ...) e os agentes globais do usuário (`zeus`, `apolo`, ...):
```
teste (subagent)
```
E `opencode debug agent teste` (após subir `opencode serve --port 4128` e consultar `GET /agent`)
devolveu o JSON completo do agente carregado a partir de `.opencode/agents/teste.md`:
```json
{
  "name": "teste",
  "description": "Agente de teste para validar reconhecimento do OpenCode",
  "mode": "subagent",
  "native": false,
  "model": { "providerID": "anthropic", "modelID": "claude-sonnet-4-5" },
  "prompt": "Você é um agente de teste. Responda sempre em português."
}
```
Confirma: `description`, `mode` e `model` (no formato `provider/model-id`) do frontmatter foram
parseados corretamente; o corpo markdown virou o campo `prompt`. **Caminho de projeto confirmado**:
`.opencode/agents/<nome>.md` (plural). **Caminho global confirmado por evidência de filesystem**: o
usuário já tinha `~/.config/opencode/agents/zeus.md` publicado manualmente (formato OpenCode nativo,
não Claude) e esse agente apareceu em `opencode agent list` como `zeus (primary)` — confirma que
`~/.config/opencode/agents/<nome>.md` é o caminho global real, não só documentado.

A skill de documentação embutida no próprio binário (`customize-opencode`, obtida via
`opencode debug skill`, fonte primária mais confiável que a doc web pois reflete exatamente a versão
instalada) confirma a tabela de caminhos:
```
Project agents  | .opencode/agent/<name>.md  ou  .opencode/agents/<name>.md
Global agents   | ~/.config/opencode/agent(s)/<name>.md
Project skills  | .opencode/skill(s)/<name>/SKILL.md
Global skills   | ~/.config/opencode/skill(s)/<name>/SKILL.md
```

#### 2. Skills — reconhecimento real confirmado, formato já compatível

`opencode debug skill` (lista todas as skills carregadas, nativas e customizadas) retornou a skill de
teste com o caminho exato:
```json
{"name": "teste", "location": ".../.opencode/skills/teste/SKILL.md"}
```
O frontmatter de skill do OpenCode (`name` + `description`, únicos campos obrigatórios; opcionais:
`license`, `compatibility`, `metadata`) é **idêntico** ao já usado nos assets canônicos do trackfw —
conferido em `internal/integrations/assets/skills/code-quality.md` (só `name:`/`description:`, sem
campos extras). `Render()` já trata `KindSkills` de forma independente da `Representation`
(`if kind == KindSkills { return normalizeMarkdown(source), nil }` — render.go linha 38), então
**nenhuma mudança é necessária para skills**: a saída atual já é um `SKILL.md` válido para o OpenCode.

Achado colateral (não acionável nesta fase, só para registro): o OpenCode já faz auto-scan de
`~/.claude/skills/<nome>/SKILL.md` e `~/.agents/skills/<nome>/SKILL.md` como "external skills"
(compatibilidade nativa com Claude Code), com aviso de `"duplicate skill name"` nos logs quando o
mesmo nome existe em mais de um caminho. Isso é irrelevante para o catálogo do trackfw (que vai
publicar no caminho canônico `.opencode/skills/`), mas explica por que máquinas com Claude Code já
instalado podem ver skills do trackfw "aparecerem" no OpenCode mesmo antes deste REQ.

#### 3. Agents — INCOMPATIBILIDADE CONFIRMADA no formato de frontmatter (bloqueia reuso direto)

Testei o frontmatter real usado hoje pelos assets canônicos (`internal/integrations/assets/agents/architect.md`):
```yaml
---
name: trackfw-architect
description: Principal software architect for system design, ADRs and governed multi-agent coordination.
model: opus
memory: project
tools: Agent, Read, Edit, Write, Bash, Grep, Glob, WebSearch, WebFetch, AskUserQuestion, EnterPlanMode, ExitPlanMode, TaskCreate, TaskGet, TaskList, TaskUpdate, TaskStop, TaskOutput
---
```
Colocado verbatim em `.opencode/agents/realformat.md`, o OpenCode **recusa a configuração inteira**:
```
Error: Configuration is invalid at .../.opencode/agents/realformat.md
↳ Expected object | undefined, got "Agent, Read, Edit, Write, Bash, Grep, Glob, ..." tools
```
Causa raiz: `tools:` é uma **chave reservada** no schema de agente do OpenCode (não um campo livre)
— ele espera um objeto de overrides por-ferramenta (ex.: `tools: { bash: false }`), não uma lista/
string de nomes de ferramentas ao estilo Claude Code. Por documentação embutida do próprio binário:
*"opencode hard-fails on invalid config"* — ou seja, isso não seria um campo ignorado silenciosamente,
seria a falha de **todo o carregamento do OpenCode no projeto** (não só daquele agente), reproduzida
experimentalmente. Confirmado também que campos desconhecidos não-reservados (ex.: `memory:`) são
absorvidos silenciosamente em `options` — o problema é especificamente `tools:` colidir com uma chave
já reservada pelo schema.

Segundo achado: sem `mode:` explícito no frontmatter, o agente carrega com `mode: "all"` (seletável
tanto como agente primário/interativo quanto como subagente) — não `"subagent"`. Testado com um agente
mínimo sem `mode:` (`.opencode/agents/nomode2.md`, com `name: trackfw-architect2`): apareceu em
`opencode agent list` como `trackfw-architect2 (all)`. Isso diverge do comportamento pretendido (os
agentes trackfw devem ser subagentes puros, nunca selecionáveis como persona principal de chat, para
paridade com o comportamento em Claude Code/Cursor/Gemini) — exige `mode: subagent` explícito.

Terceiro achado: `model:` no catálogo usa aliases curtos do Claude Code (`opus`, `sonnet`), mas o
OpenCode espera `provider/model-id` (confirmado no achado #1: `anthropic/claude-sonnet-4-5` funcionou
corretamente). Testei `model: opus` (sem prefixo) — o OpenCode **aceita no load** (não falha), mas
resolve para `{"providerID": "opus", "modelID": ""}` — uma referência de provider/modelo inválida que
falharia em runtime (fallback silencioso ruim, pior que omitir o campo). Isso precisa de mapeamento
(análogo ao `mapModel()` já existente para o Antigravity em render.go) ou omissão deliberada do campo
`model:` para deixar o OpenCode usar o modelo default configurado globalmente pelo usuário — o que
aliás se alinha melhor com a motivação de negócio do REQ (permitir que o usuário roteie os agentes
trackfw para o modelo local que ele já configurou em `opencode.json`, em vez de fixar Anthropic por
agente). **Esta é uma decisão de produto, não só técnica — sinalizo para confirmação do orquestrador
antes da Wave 2**, mas tecnicamente ambas as opções (mapear ou omitir) são igualmente simples de
implementar em `render.go`.

#### 4. AGENTS.md — parcialmente confirmado (limite do ambiente, não do OpenCode)

Confirmado: o OpenCode rodou sem erro em um projeto com `AGENTS.md` presente na raiz (nenhuma rejeição
ou ignorância observável do arquivo — todos os comandos `opencode agent list`, `opencode debug ...`
executaram normalmente com o arquivo presente). A skill embutida `customize-opencode` (fonte primária,
lida diretamente do binário instalado) documenta `"instructions": ["AGENTS.md", "docs/style.md"]`
como campo de config de primeira classe, com `AGENTS.md` como o exemplo canônico — reforça que é o
arquivo de instruções padrão do OpenCode (aliás `AGENTS.md` é o padrão aberto que o próprio time do
OpenCode ajudou a popularizar entre múltiplas CLIs).

**Não foi possível** confirmar o round-trip completo (o conteúdo do `AGENTS.md` de fato influenciando
uma resposta observável do modelo), porque a máquina não tem nenhum provider de LLM ativo no momento
do teste: `curl http://127.0.0.1:1234/v1/models` (LM Studio) não respondeu, e `ollama` não está
instalado (`command not found`). Um teste com `opencode run "oi" --agent teste --model
lmstudio/mistralai/devstral-small-2-2512 --print-logs --log-level DEBUG` chegou a montar a sessão e
tentar a chamada real ao modelo, mas falhou em `AI_APICallError: Cannot connect to API` — evidência de
que o fluxo de request foi montado e disparado (não é um erro de parsing/config), mas sem servidor de
modelo local rodando não há como inspecionar o payload final enviado. **Não é um problema do OpenCode
nem motivo para bloquear a Wave 2** — é uma limitação do ambiente de teste (sem LM Studio/Ollama
rodendo), e o critério de aceite do ML-1A ("confirmar que o arquivo não é ignorado/rejeitado") foi
atendido. Recomendo repetir o teste de round-trip completo quando houver um provider local ativo,
como validação leve na Wave 2/3, mas sem bloquear o início delas.

#### Decisão final

**É necessária uma nova `Representation`** (proposta: `"opencode-agent"`) — o reuso direto de
`Render()` (Rota B / default branch, hoje usada por `subagent`) **quebraria o carregamento do
OpenCode** por causa do campo `tools:` (chave reservada com schema incompatível), e produziria agentes
com `mode` e `model` incorretos mesmo se `tools:` fosse removido manualmente. Isso não é uma mudança
cosmética — é uma incompatibilidade de schema comprovada experimentalmente (erro de "Configuration is
invalid", processo inteiro recusa iniciar).

**Escopo da mudança avaliado como "adicionar um case"** (não exige ADR complementar): o padrão a
seguir já existe em `render.go` no case `"agent-directory"` (usado pelo Antigravity), que resolve
exatamente esse tipo de situação — reconstrói o frontmatter do zero a partir de `name`/`description`/
`model`/`body` (já extraídos por `markdownParts`), mapeia o campo `model` via uma função dedicada, e
omite campos não suportados. Para `"opencode-agent"` bastaria: (a) reconstruir frontmatter só com
`description` + `mode: subagent` fixo + `model` (mapeado ou omitido — decisão de produto pendente,
ver achado #3) e (b) manter o body como prompt. Nenhuma skill precisa de mudança (achado #2).

**Decisão do orquestrador (trackfw_architect) sobre `model:` — omitir o campo, não mapear.**
Justificativa: a motivação de negócio deste REQ inteiro é permitir que os agentes trackfw rodem sob
o modelo open-source/local que o usuário já configurou no `opencode.json` (o achado #1 confirma que
a própria máquina de teste já tem um provider `lmstudio` configurado). Mapear `model:` para um valor
fixo (ex: sempre Anthropic) contradiria essa motivação — forçaria o mesmo modelo de nuvem em todo
agente trackfw, ignorando exatamente a escolha de provider que o usuário já fez. Omitir o campo
deixa o OpenCode resolver o modelo pelo default configurado (global ou por-agente, à escolha do
usuário), que é o comportamento correto tanto para quem usa Anthropic quanto para quem usa
Ollama/LM Studio. Simetria de esforço de implementação confirmada pela pesquisa (achado #3): as duas
opções são igualmente simples em `render.go`, então a decisão é puramente de produto, não técnica.

## Wave 2 — Go: catálogo + adapter (referência comportamental)
> Dependencies: Wave 1 completa

### ML-2A — Adicionar target `opencode` a `internal/integrations/assets/catalog.json`
**Status:** ✅ Concluído
**Files affected:**
- `internal/integrations/assets/catalog.json` — novo target `opencode`, surface `cli`, escopos
  `global`+`project`, `agents.representation: "opencode-agent"`, `skills.representation: "skill"`
- `internal/integrations/render.go` — novo case `"opencode-agent"` em `Render()`, reconstruindo o
  frontmatter do zero (mesmo padrão de `"agent-directory"`): `description` mantida, `mode: subagent`
  sempre fixo, `model:`/`tools:`/`memory:` omitidos (decisão de produto do orquestrador registrada na
  Wave 1 — omitir em vez de mapear)
- `internal/integrations/render_test.go` — `TestRenderOpenCodeAgent`
- `internal/integrations/catalog_test.go` — `TestLoadCatalogHasCanonicalInventory` atualizado para 10
  targets (inclui `opencode`)
**Actions:** Definido o target `opencode` seguindo exatamente o schema dos 9 targets existentes (surface
`cli`, escopos `global`+`project`, paths para agents/skills conforme decidido na Wave 1).
**Acceptance criteria:**
- [x] `go build ./...`, `go test ./internal/integrations/...` verdes
- [x] `trackfw agents list --json` mostra o target `opencode`

### ML-2B — Lifecycle Go completo (install/uninstall/update) + AGENTS.md
**Status:** ✅ Concluído
**Files affected:**
- `internal/commands/agents_skills_test.go` — `TestOpenCodeAgentsLifecycleEndToEnd` (install → list →
  update → uninstall com `--targets opencode`)
- `internal/generators/agentfiles.go` — nenhuma mudança necessária, confirmado manualmente (ver Actions)
**Actions:** Confirmado que o lifecycle genérico já cobre o novo target sem código extra (ponto central
do ADR-2026-07-18). Validação contra o OpenCode real (1.18.13, `/opt/homebrew/bin/opencode`):
1. `trackfw agents install --targets opencode --scope project --items architect,backend` gerou
   `.opencode/agents/trackfw-architect.md` e `.opencode/agents/trackfw-backend.md` com frontmatter
   correto (`description`, `mode: subagent`, sem `model:`/`tools:`/`memory:`).
2. `opencode agent list` num projeto de teste isolado (`git init`, fora do repo) carregou ambos como
   `trackfw-architect (subagent)` e `trackfw-backend (subagent)` sem nenhum erro de configuração —
   confirma que o bug de `tools:` (achado #3 da Wave 1) está corrigido pela nova representação.
3. `opencode serve` + `GET /agent` confirmou via JSON resolvido: `mode: "subagent"` correto, chave
   `model` **ausente** do objeto (não apenas null — omitida de fato, comportamento pretendido), `prompt`
   preservando o corpo completo (incluindo saudação/assinatura de identidade quando configurada).
4. `opencode debug skill` confirmou a skill de projeto reconhecida (colidiu por nome com uma skill
   global pré-existente do Claude Code na máquina de teste — comportamento de dedupe já documentado
   como achado colateral não-acionável na Wave 1; conteúdo idêntico em ambos os caminhos).
5. `trackfw agents list/update/uninstall --targets opencode` e `trackfw skills install/uninstall
   --targets opencode` testados manualmente end-to-end, todos corretos.
6. `AGENTS.md`: confirmado com `trackfw discover --init` num projeto de teste com `AGENTS.md`
   pré-existente — o bloco de regras trackfw foi injetado corretamente, sem nenhuma mudança de código
   (a detecção em `agentfiles.go` já é por path, independente da ferramenta que criou o arquivo).
**Acceptance criteria:**
- [x] Testes end-to-end de install/uninstall/update com `--targets opencode` verdes
- [x] `go test ./internal/...` completo verde

> Auditoria manual (trackfw_architect): revalidei tudo de forma independente — `go build`,
> `go test ./...` completo (todos os pacotes), `trackfw agents list --json` confirmando o target.
> Instalei os agentes num projeto de teste real (`/tmp`, fora do repo) e rodei o binário OpenCode
> de verdade: `opencode agent list` mostrou `trackfw-architect (subagent)` e
> `trackfw-backend (subagent)` sem nenhum erro de configuração — confirma que o bug crítico da
> Wave 1 (`tools:` derrubando o carregamento inteiro) está corrigido. Subi `opencode serve` e
> consultei `GET /agent` via curl: `grep '"model"'` no JSON completo não encontrou **nenhuma**
> ocorrência — confirma que o campo está de fato ausente (decisão de produto respeitada), não só
> null. Diretório de teste removido ao final.

## Wave 3 — Node.js + Python (paralelo entre si)
> Dependencies: Wave 2 completa

### ML-3A — Sincronizar assets + confirmar lifecycle Node.js
**Status:** ✅ Concluído
**Files affected:** `npm/src/integrations/render.js`, `npm/tests/render_opencode.test.js`,
`npm/src/integrations/assets/catalog.json` (via `scripts/sync-integration-assets.sh`, rodado pelo
orquestrador após Wave 3A+3B prontas)
**Acceptance criteria:**
- [x] `npm test` verde com os novos cenários (375 passed)
- [x] `bash scripts/check-integration-assets.sh` verde

### ML-3B — Sincronizar assets + confirmar lifecycle Python
**Status:** ✅ Concluído
**Files affected:** `pypi/trackfw/integrations/renderers.py`,
`pypi/tests/test_integrations_identity.py`, `pypi/tests/test_agents_skills.py` (fixture de 9→10
targets atualizada, ordem `..., amazonq, opencode, kiro`),
`pypi/trackfw/integrations/assets/catalog.json` (via sync script)
**Acceptance criteria:**
- [x] `python3 -m pytest` verde com os novos cenários (892 passed)
- [x] `bash scripts/check-integration-assets.sh` verde

### Achado extra (auditoria pós-Wave 3): gap de paridade pré-existente no harness de `update`
`internal/generators/update.go` (`harnessCatalogTargetOrder`) e
`pypi/trackfw/commands/update_harness.py` (`_CATALOG_TARGET_ORDER`) hardcodam uma lista de targets
**separada** de `catalog.json` (Node.js deriva a lista dinamicamente do catálogo, por isso já estava
correto). A Wave 2 (Go) não atualizou essa lista fixa ao adicionar `opencode` — `check-update-parity.sh`
pegou a divergência (Go emitia 19 ids, sem `opencode-agents`/`opencode-skills`; Python tinha o mesmo
gap). Corrigido: `opencode` inserido entre `amazonq` e `kiro` nas duas listas fixas (ordem real do
`catalog.json`), contagem 19→21 ids atualizada nos comentários e em `docs/cli-parity.md` ("Declared
harness targets — pinned list").

### Achado extra (ML-4A): mais duas listas hardcoded de targets sem `opencode` (mesma classe da Wave 3)

A varredura pedida na ML-4A (`grep` por `amazonq`/`kiro` fora de `catalog.json`) encontrou, além dos
dois hits já corrigidos na Wave 3 (`internal/generators/update.go`,
`pypi/trackfw/commands/update_harness.py`), mais duas listas hardcoded no fluxo de `trackfw init`
que não tinham sido atualizadas quando `opencode` entrou no catálogo (Wave 2), uma delas um bug
funcional real, não só cosmético:

1. **`npm/src/commands/init.js:61` (bug funcional).** O `Set` `supported`, usado para validar
   `--ai-tools` no modo não-interativo, não incluía `opencode` — `trackfw init --ai-tools opencode`
   (sem TTY) lançava `Unsupported AI tool: opencode` no CLI Node.js, enquanto Go e Python aceitavam
   normalmente (ambos validam via o catálogo, sem lista própria). Divergência real entre os 3
   runtimes para o mesmo input — corrigido acrescentando `opencode` ao Set.
2. **Wizards interativos inutilizáveis para o novo target.** Nem o wizard `huh` de
   `internal/commands/init.go` (Go) nem o `checkbox` de `npm/src/commands/init.js` (Node.js) listavam
   OpenCode como opção selecionável — o target ficava inacessível via `trackfw init` interativo em
   ambos, apesar de já funcionar via `trackfw agents/skills install --targets opencode`. Python não
   tem lista hardcoded equivalente em `pypi/trackfw/commands/init.py` (repassa `--ai-tools` direto
   para `plan_deployments`, validado pelo catálogo), logo não tinha esse gap. Corrigido adicionando a
   opção `OpenCode`/`opencode` nos dois wizards e nas duas help strings de `--ai-tools`. Verificado
   antes de editar que `InjectRulesForTool` (Go) / `injectRulesForTool` (Node.js) fazem no-op seguro
   para `opencode` (ausente dos mapas `agentFiles`/`AGENT_FILES`), então a opção nova não abre um
   caminho de erro.
3. **`internal/commands/agents_skills_test.go:284`
   (`TestInitAIToolsHelpIncludesEveryCatalogTarget`)** tinha a mesma lista desatualizada — o teste
   existe exatamente para pegar esse tipo de gap, mas estava passando vacuamente por nunca checar
   `opencode`. Lista do teste atualizada para incluir o novo target.
4. **Confirmado sem gap**: `pypi/trackfw/commands/init.py` (sem lista hardcoded, já genérico) e as
   suítes `npm/tests/` / `pypi/tests/` (nenhum teste de `init`/`ai-tools` hardcoda a lista de 9/10
   alvos, então nenhum teste precisou de atualização além do Go acima).

**Lição para o vault/lições futuras**: essas listas (`update` harness targets, `init` wizard
choices/help, Sets de validação não-interativa) são **intencionalmente estáticas por contrato**
(`docs/cli-parity.md` pina a lista de harness targets como "not derived at runtime") — cada alvo novo
no catálogo exige sincronizar manualmente todas essas listas nos 3 runtimes; não há um único ponto
central que as force a crescer junto com `catalog.json`. Uma varredura por `grep` de "todo alvo já
existente" (ex: `amazonq.*kiro`) é hoje o único jeito prático de achar as que ficaram para trás.

## Wave 4 — Documentação e gate de paridade
> Dependencies: Wave 3 completa

### ML-4A — Documentar e validar o gate de paridade de identidade
**Status:** ✅ Concluído
**Files affected:**
- `docs/cli-parity.md` — OpenCode adicionado à frase de targets suportados da seção "AI integration
  lifecycle"; nova subseção `### OpenCode agent representation (opencode-agent)` documentando por
  que o frontmatter é reconstruído do zero, por que `mode: subagent` é sempre fixo, e por que
  `model:`/`tools:`/`memory:` são omitidos (com a evidência do `tools:` hard-fail no OpenCode
  1.18.13 e o comportamento observado de `GET /agent`); cross-link explícito confirmando que a
  tabela "Declared harness targets — pinned list" (já atualizada na Wave 3) não foi duplicada.
- `internal/commands/init.go` — `--ai-tools` help string e opção `huh.NewOption("OpenCode",
  "opencode")` no wizard interativo de `trackfw init` (achado extra, ver abaixo).
- `internal/commands/agents_skills_test.go` — `TestInitAIToolsHelpIncludesEveryCatalogTarget` agora
  cobre `opencode` (achado extra).
- `npm/src/commands/init.js` — `--ai-tools` help string, `supported` Set (validação não-interativa)
  e opção de checkbox `{ name: 'OpenCode', value: 'opencode' }` no wizard interativo (achado extra).
**Actions:**
1. OpenCode adicionado à lista de CLIs suportados por `agents`/`skills` em `docs/cli-parity.md`,
   com a decisão de representação documentada (ver Files affected).
2. Confirmado que `scripts/check-identity-parity.sh` **já cobre o novo target automaticamente** —
   `load_catalog_targets()` deriva a lista de `target/surface` a partir de
   `internal/integrations/assets/catalog.json` (`support_level != "unsupported"`), sem lista manual;
   `opencode` entra no gate sem nenhuma edição. **Não é um bug — nenhuma correção necessária aqui.**
3. Varredura ampla (`grep -rn` por `amazonq`/`kiro` em `scripts/*.sh`, `internal/`, `npm/src/`,
   `pypi/trackfw/`) encontrou mais duas ocorrências reais da mesma classe de defeito da Wave 3 (lista
   de targets hardcoded que não cresce junto com o catálogo) — corrigidas nesta ML, ver "Achado
   extra" abaixo.
**Acceptance criteria:**
- [x] `make quality` verde
- [x] `trackfw validate` sem violações
