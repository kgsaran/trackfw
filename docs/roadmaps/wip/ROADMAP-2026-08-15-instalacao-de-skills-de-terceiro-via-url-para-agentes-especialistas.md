---
status: wip
date: 2026-08-15
req: "docs/req/REQ-2026-08-15-instalacao-de-skills-de-terceiro-via-url-para-agentes-especialistas.md"
squad: "hades-tf, apolo-tf, hefesto-tf"
---

# Roadmap: instalacao de skills de terceiro via URL para agentes especialistas

> Created: 2026-08-15 | Reescrito: 2026-08-15 (Zeus) | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-08-15-instalacao-de-skills-de-terceiro-via-url-para-agentes-especialistas.md -->
REQ: `docs/req/REQ-2026-08-15-instalacao-de-skills-de-terceiro-via-url-para-agentes-especialistas.md`

Permitir que o usuário aponte uma URL de "skill" de terceiro (conhecimento de linguagem,
stack, design pattern, padrão arquitetural) e que o `trackfw` baixe esse conteúdo e o
**componha** — nunca sobrescreva — no(s) agente(s) do catálogo escolhido(s) pelo usuário.

**Restrição do usuário (2026-08-15):** o comando só roda dentro de sessão de agente e
apenas pelo orquestrador/arquiteto (`trackfw_architect`/Zeus); nunca por invocação humana
direta de terminal, nunca por um especialista. Fluxo: usuário aponta URL → Zeus invoca
`hades-tf` → só com parecer favorável a instalação prossegue.

**🔑 Emenda de escopo (2026-08-15, KG) — muda a natureza do gate.** A revisão do `hades-tf`
**não é evento único de desenho; é gate de runtime, recorrente**, disparado a **cada**
instalação de artefato de terceiro:

- **Abrange skill, agent e plugin** de terceiro — não só skill. Todo artefato de terceiro que
  vire instrução carregada por um agente entra no mesmo fluxo.
- **Dois caminhos de entrada, mesmo gate:** (i) o usuário executa o comando do `trackfw`
  (`skill add` / `agent add` / equivalente third-party); (ii) o usuário pede em linguagem
  natural na sessão ("instala essa skill pra mim"). Em ambos: **baixar → quarentena →
  `hades-tf` analisa → só com parecer favorável instala.** Nunca instalar e revisar depois.

**Consequência de design:** um comando de CLI não invoca subagente por si. Logo o comando
precisa **parar no meio do fluxo** — baixar em quarentena, emitir artefato de revisão legível
por máquina, e exigir referência ao parecer para consumar a instalação. O handshake exato em
duas fases é decisão da Wave 0 (**Q8**); a propriedade *"não existe caminho de código que
instale artefato de terceiro sem parecer prévio"* é requisito, não sugestão.

> ⚠️ Isto amplia o alvo do trabalho: o subcomando third-party precisa existir tanto para
> `skills` quanto para `agents` (`internal/commands/skills.go` **e**
> `internal/commands/agents.go`, ambos sobre `newIntegrationsLifecycleCmd` em
> `internal/commands/integrations_flags.go`), e o `trackfw plugins`
> (`internal/plugins/plugins.go` + espelhos) precisa ser avaliado na Q8 — hoje ele **baixa e
> instala binário de terceiro sem gate nenhum**.

### Mapa arquitetural apurado (2026-08-15) — base factual deste roadmap

O subsistema alvo **já existe** e é `internal/integrations/` (não `internal/generators/`):

| Peça | Go (canônico) | Node | Python |
|---|---|---|---|
| Comando `skills` | `internal/commands/skills.go` | `npm/src/commands/skills.js` | `pypi/trackfw/commands/skills.py` |
| Ciclo de vida / flags | `internal/commands/integrations_flags.go` | `npm/src/integrations/index.js` | `pypi/trackfw/integrations/command.py` |
| Catálogo | `internal/integrations/catalog.go` | `npm/src/integrations/catalog.js` | `pypi/trackfw/integrations/catalog.py` |
| Render por target | `internal/integrations/render.go` | `npm/src/integrations/render.js` | `pypi/trackfw/integrations/renderers.py` |
| Escrita atômica + posse | `internal/integrations/manager.go` · `manifest.go` | `npm/src/integrations/manager.js` | `pypi/trackfw/integrations/manager.py` |
| Assets (fonte única) | `internal/integrations/assets/` | espelho | espelho |

Fatos que condicionam o desenho e que **não podem ser reinventados**:

1. **Assets têm fonte única.** `internal/integrations/assets/` é canônico;
   `scripts/sync-integration-assets.sh` espelha para Node/Python e
   `scripts/check-integration-assets.sh` (dentro de `make parity`) falha se divergirem.
2. **Escrita passa pelo `Manager`.** `<root>/.trackfw/integrations-manifest.json`
   (`internal/integrations/manifest.go`) registra posse + hash de conteúdo por destino e
   recusa clobber de arquivo modificado pelo usuário sem `--force`. Nada de `os.WriteFile`
   cru.
3. **Precedente de composição idempotente:** `injectOrUpdateRules` em
   `internal/generators/agentfiles.go` (marcadores `<!-- trackfw:rules:start -->` …) —
   substitui apenas o bloco entre marcadores. É o padrão a espelhar, mas vive em outro
   subsistema: a composição em arquivo **de catálogo** precisa de ponto de extensão próprio
   em `internal/integrations/render.go`.
4. **Precedente de download de terceiro:** `internal/plugins/plugins.go`
   (`httpClient` com `Timeout: 30s`, `maxRegistrySize = 1<<20`, `maxPluginSize = 50<<20` via
   `io.LimitReader`, `os.CreateTemp` + `os.Rename`). Espelhos: `npm/src/commands/plugins.js`,
   `pypi/trackfw/commands/plugins.py`. Reusar limites e escrita atômica daí.
5. **Precedente de proveniência:** `appendTransitionLog` (`internal/generators/roadmap.go:456`)
   → `.trackfw-log`, append-only, formato `"%s  %-50s  %s → %s\n"`, nunca fatal.
6. **Config:** `parse()` em `internal/config/config.go` · `npm/src/config/index.js` ·
   `pypi/trackfw/config.py`; para leitura cwd-independente seguir o padrão
   `ReadAgentConventions(cwd)` / `readAgentConventions(cwd)` / `read_agent_conventions(cwd)`.
7. **`make quality`** = `test test-node test-python lint parity` (`Makefile:50`). Os scripts
   de paridade que este trabalho toca: `scripts/check-integration-assets.sh`,
   `scripts/check-artifact-parity.sh`, `scripts/check-identity-parity.sh`,
   `scripts/check-cli-parity.sh`.

### Conflitos e tensões abertos — a Wave 0 EXISTE para resolvê-los

- 🔴 **Escopo default.** A REQ pede "local ao projeto por padrão". O
  `ADR-2026-07-25-escopo-de-instalacao-selecionavel-para-agents-e-skills` decidiu o oposto
  (**D1: default `global`**, com `resolveScope` em `integrations_flags.go:436`). Isto é
  conflito real de ADR aceito: ou a Wave 0 justifica uma exceção específica para skills de
  terceiro (superfície de ataque diferente de asset de catálogo assinado pelo projeto), ou a
  REQ cede. **Não implementar nada antes dessa decisão estar num ADR.**
- 🔴 **"Só o orquestrador executa" não é controle técnico.** O
  `ADR-2026-08-12-nao-ha-prevencao-contra-agente-induzido-com-escrita-irrestrita-a-resposta-e-deteccao-ancorada-no-git`
  é doutrina canônica do projeto: não há prevenção contra agente induzido com escrita no
  workspace; a resposta é **detecção ancorada no `HEAD` do git**. Uma env var injetada pelo
  harness é trivialmente setável num terminal humano — é *guardrail*, não controle. A Wave 0
  deve registrar isso como limitação assumida e desenhar a **detecção** (artefato versionado,
  visível em `git status`/diff/PR) como a resposta real.
- 🟡 **Detecção de "agent kidnapping" por marcador literal** (`## Git authority`, `## Mode
  lock`) é um teste necessário e trivialmente evadível (unicode, paráfrase, HTML comment).
  A Wave 0 define o conjunto mínimo objetivo E declara explicitamente o que **não** cobre.
- 🟡 **Onde a skill reside.** Ainda indefinido: novo item de catálogo? arquivo separado
  referenciado por link no agente? bloco entre marcadores no arquivo renderizado? Cada opção
  interage de forma diferente com o `manifest.json` e com `check-integration-assets.sh`.

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ] AC1 — Comando baixa o conteúdo e **nunca instala sem confirmação**: exibe o conteúdo
      completo (ou diff do que seria adicionado) antes de gravar; em modo não-interativo/CI
      recusa por padrão, exigindo flag explícita de confiança na fonte.
- [ ] AC2 — Instalação é recusada se o conteúdo baixado tentar redefinir fronteiras de agente
      (Git authority / governance prerequisite / mode lock), pelo critério objetivo fixado na
      Wave 0.
- [ ] AC3 — A skill **nunca substitui** o arquivo de um agente do catálogo: é sempre seção
      suplementar apensada/referenciada. O usuário confirma explicitamente a quais agentes se
      aplica; o `trackfw` sugere mas não decide sozinho.
- [ ] AC4 — Escopo de instalação: `<<TBD-D4 escopo default nomeado>>` (resolve o conflito com
      ADR-2026-07-25 D1); escopo global nunca sem confirmação extra.
- [ ] AC5 — Proveniência auditável registrada (URL, hash/checksum, data) em artefato
      versionado do projeto.
- [ ] AC6 — Comportamento idêntico nos 3 CLIs (Go · Node · Python).
- [ ] AC7 — `make quality` passa sem novas divergências de paridade.
- [ ] AC8 — Revisão do `hades-tf` documentada em parecer + ADR **antes** do primeiro ML de
      implementação.
- [ ] AC9 — Restrição de invocação (só orquestrador, dentro de sessão de agente) implementada
      via `<<TBD-D2 mecanismo nomeado>>` + detecção `<<TBD-D2 detecção nomeada>>`, com a
      limitação de "guardrail, não controle" declarada explicitamente na doc e no ADR.
- [ ] AC10 — Gate de runtime recorrente: **nenhum caminho de código instala artefato de
      terceiro (skill, agent ou plugin) sem parecer prévio do `hades-tf`**. O comando baixa
      para quarentena, para, e só consuma mediante referência ao parecer favorável
      (`<<TBD-D8 handshake nomeado>>`). Vale para os dois caminhos de entrada: comando
      explícito do `trackfw` e pedido em linguagem natural na sessão. Verificação: teste que
      tenta consumar a instalação sem referência de parecer **falha**, nos 3 CLIs.

---

## Wave 0 — Desenho de segurança e decisões arquiteturais (BARREIRA BLOQUEANTE)
> Dependências: nenhuma.
> ⛔ **Nenhuma Wave posterior pode iniciar antes de ML-0A e ML-0B estarem ✅.** As Waves 1+
> estão deliberadamente escritas como *contingentes*: seus valores exatos (nome do comando,
> nomes de flags, formato do arquivo de proveniência, local de residência da skill) são
> **saída** desta Wave, não entrada. Reescrever as Waves 1+ com os valores fixados é a última
> tarefa do ML-0B.

### ML-0A — Parecer de segurança do `hades-tf` sobre o desenho
**Status:** ✅ Concluído (2026-08-15) — parecer em `docs/seguranca/2026-08-15-skills-de-terceiro-via-url.md`
**Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Arquivos afetados (escrita):**
- `docs/seguranca/2026-08-15-skills-de-terceiro-via-url.md` (novo — único arquivo que este ML cria)

**Leitura obrigatória antes de opinar:**
- `docs/req/REQ-2026-08-15-instalacao-de-skills-de-terceiro-via-url-para-agentes-especialistas.md`
- `docs/adr/ADR-2026-08-12-nao-ha-prevencao-contra-agente-induzido-com-escrita-irrestrita-a-resposta-e-deteccao-ancorada-no-git.md`
- `docs/adr/ADR-2026-07-25-escopo-de-instalacao-selecionavel-para-agents-e-skills.md`
- `internal/plugins/plugins.go` · `internal/integrations/manager.go` · `internal/integrations/manifest.go`
- `internal/generators/scaffold.go` (funções `GenerateCredentialGuardScript`, `GenerateGitBranchGuardScript`)
- `vault/notes/index.md` e, dele, obrigatoriamente as notas:
  `credential-guard-hook-resolvable-nao-detecta-script-ausente-2026-08-15`,
  `git-branch-guard-self-blocking-quote-unaware-splitter-2026-08-14`,
  `credential-guard-second-layer-cmd-extraction-json-not-raw-token-2026-08-08`

**Ações (o parecer deve responder, cada um como seção própria, com veredito explícito):**
1. **Q1 — Modelo de ameaça.** Enumerar os vetores de uma skill baixada por URL: prompt
   injection direta, agent kidnapping (auto-concessão de Git authority / desligamento do gate
   de governança), exfiltração de segredos, TOCTOU (URL serve conteúdo A na revisão e B na
   instalação), redirect/DNS rebinding, conteúdo não-markdown, zip-bomb/tamanho.
2. **Q2 — Invocação restrita ao orquestrador.** Avaliar a proposta da REQ (env var injetada
   pelo harness). Declarar se é controle ou guardrail à luz do ADR-2026-08-12 e **propor a
   detecção correspondente** ancorada no git (qual artefato versionado registra a instalação,
   como `trackfw validate` a verifica).
3. **Q3 — Critério objetivo de recusa (AC2).** Definir a lista literal mínima de marcadores/
   padrões que causam recusa (ex.: headings `## Git authority`, `## Mode lock`,
   `## Dispatch contract`, `## Git authority` com variações de nível `#`/`###`), a política de
   normalização antes do match (case, unicode NFKC, strip de HTML comments) e, obrigatoriamente,
   uma seção **"O que este critério NÃO cobre"**.
4. **Q4 — Escopo default.** Emitir recomendação fundamentada sobre o conflito REQ (project) ×
   ADR-2026-07-25 D1 (global), considerando que o ADR-2026-08-12 argumenta que artefato dentro
   do repositório é *mais* auditável.
5. **Q5 — Residência e composição.** Recomendar entre: (a) bloco entre marcadores no arquivo
   renderizado do agente, (b) arquivo separado sob `.claude/skills/` referenciado por link,
   (c) novo item de catálogo. Avaliar cada opção contra o `manifest.json` (posse/hash) e contra
   `scripts/check-integration-assets.sh`.
6. **Q6 — Proveniência.** Recomendar formato e local (append-only estilo `.trackfw-log` vs JSON
   estruturado), incluindo algoritmo de hash e o que fazer quando o hash da URL mudar depois.
7. **Q7 — Rede.** Política de fetch: esquemas permitidos (só `https`), timeout, limite de
   tamanho, política de redirect, verificação de content-type — ancorada nos limites já usados
   em `internal/plugins/plugins.go`.
8. **Q8 — Handshake de duas fases (gate de runtime recorrente).** ⭐ Pergunta mais importante
   desta Wave, decorrente da emenda de escopo de KG. Um comando de CLI não invoca subagente.
   Desenhar o handshake que torna verdadeira a propriedade *"não existe caminho de código que
   instale artefato de terceiro sem parecer prévio do `hades-tf`"*. Responder especificamente:
   - **(a) Quarentena:** onde o conteúdo baixado repousa antes da aprovação, e por que esse
     local não é carregável por nenhum agente enquanto estiver lá.
   - **(b) Artefato de revisão:** o que a fase 1 emite para o `hades-tf` consumir (formato,
     caminho, campos — no mínimo URL, checksum, conteúdo integral, resultado da validação
     automática de Q3).
   - **(c) Prova de aprovação:** como a fase 2 verifica que o parecer é favorável **e que é
     daquele conteúdo** — o vínculo tem de ser pelo checksum, senão aprova-se um conteúdo e
     instala-se outro (TOCTOU de Q1). Quem pode emitir a prova e por que ela não é forjável
     trivialmente por quem já tem escrita no workspace (à luz do ADR-2026-08-12: se não for,
     dizer isso e ancorar em detecção).
   - **(d) Caminho de linguagem natural:** como o gate também vale quando o usuário pede
     "instala essa skill" em vez de rodar o comando — o que impede o agente de pular a fase 1.
   - **(e) Cobertura:** decidir se `trackfw plugins install` (`internal/plugins/plugins.go`,
     hoje **sem gate nenhum**, baixa e `chmod 0755` binário de terceiro) entra no escopo desta
     REQ, entra como REQ separada, ou fica declaradamente fora — com justificativa.
   - **(f) Falha aberta ou fechada:** se o parecer não puder ser lido/validado, o comportamento
     é recusar (fail-closed). Confirmar e dizer o que acontece em CI.

**Critérios de aceite:**
- [ ] Arquivo `docs/seguranca/2026-08-15-skills-de-terceiro-via-url.md` existe e responde Q1–Q8,
      cada uma com veredito explícito (não "depende").
- [ ] Q8 responde (a) a (f) individualmente, e a resposta de (c) vincula aprovação a checksum.
- [ ] Q3 contém a seção "O que este critério NÃO cobre".
- [ ] Q2 declara explicitamente se a restrição de invocação é controle ou guardrail e cita o
      ADR-2026-08-12.
- [ ] Nenhum arquivo fora de `docs/seguranca/` foi modificado (nenhum código de produto).

**Comandos de validação:** `git status --porcelain` deve listar exclusivamente
`docs/seguranca/2026-08-15-skills-de-terceiro-via-url.md`.

---

### ML-0B — ADR consolidando as decisões + reescrita das Waves 1+ com valores fixados
**Status:** ⬜ Pendente
**Agente:** Zeus (`trackfw_architect`) — não delegável: é decisão arquitetural e resolve conflito
entre ADRs aceitos.
**Dependência:** ML-0A ✅.
**Arquivos afetados:**
- `docs/adr/ADR-2026-08-15-<slug>.md` (novo, via `trackfw adr new`)
- `docs/req/REQ-2026-08-15-instalacao-de-skills-de-terceiro-via-url-para-agentes-especialistas.md`
  (preencher o campo `adr:` do frontmatter, hoje `""`)
- este roadmap (reescrita das Waves 1+ com nome de comando, flags, caminhos e formatos exatos)

**Ações:**
1. Ler o parecer do ML-0A na íntegra.
2. Escrever o ADR com decisões numeradas **D1…D8**, uma por pergunta Q1–Q8, cada uma
   acionável (nome de comando, nomes de flags, caminhos, formatos, limites numéricos).
3. Se o ADR divergir do `ADR-2026-07-25` D1 (escopo default), declarar a relação
   explicitamente — emenda ou exceção escopada a skills de terceiro — e o porquê. Não deixar
   dois ADRs aceitos em contradição silenciosa.
4. Reescrever as Waves 1 a 4 deste roadmap substituindo cada `<<TBD-Dn>>` pelo valor decidido.
5. **Guarda de escopo:** se a resposta de Q8(e) trouxer `trackfw plugins` (download de
   **binário** de terceiro + `chmod 0755`) para dentro do escopo, **abrir REQ separada** — não
   expandir esta REQ nem renomear a branch. Gate de binário é superfície de ameaça
   materialmente distinta de gate de composição de markdown e arrastaria a Wave 2 de lado.
   Registrar a decisão no ADR de qualquer forma (dentro, fora, ou REQ nova).
6. **Reescrever também AC4 e AC9 do bloco `## Acceptance Criteria` consolidado** com os valores
   nomeados (escopo default nomeado; mecanismo de invocação e detecção nomeados). Nenhum AC pode
   restar referenciando "conforme a Wave 0" — todo AC precisa ser verificável sem abrir o ADR.

**Critérios de aceite:**
- [ ] ADR criado com status `Accepted` e decisões D1–D8 cobrindo Q1–Q8 (D8 = handshake de duas fases).
- [ ] Relação com `ADR-2026-07-25` declarada explicitamente (emenda, exceção ou reafirmação).
- [ ] Campo `adr:` da REQ preenchido.
- [ ] Nenhum `<<TBD-Dn>>` restante neste arquivo (inclusive no bloco de AC consolidado).
- [ ] Nenhum AC contém a expressão "conforme … Wave 0".
- [ ] `trackfw validate` passa.

**Comandos de validação:**
```bash
trackfw validate
R=docs/roadmaps/wip/ROADMAP-2026-08-15-instalacao-de-skills-de-terceiro-via-url-para-agentes-especialistas.md
grep -c "TBD-D" "$R"          # deve ser 0
grep -ci "conforme .* Wave 0" "$R"   # deve ser 0
```

---

## Wave 1 — Núcleo Go: fetch, validação e proveniência (contingente à Wave 0)
> Dependências: Wave 0 completa (ML-0A + ML-0B ✅).
> ⚠️ MLs sequenciais entre si: ML-1A e ML-1B compartilham o mesmo pacote novo.

### ML-1A — Fetch seguro + validação de conteúdo (pacote Go novo)
**Status:** ⬜ Pendente
**Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos afetados:**
- `internal/skillsrc/fetch.go` (novo — nome de pacote confirmado em `<<TBD-D5>>`)
- `internal/skillsrc/validate.go` (novo)
- `internal/skillsrc/fetch_test.go`, `internal/skillsrc/validate_test.go` (novos)

**Ações:**
1. Implementar fetch HTTPS-only espelhando os limites de `internal/plugins/plugins.go`:
   `http.Client{Timeout: <<TBD-D7>>}`, `io.LimitReader` com `<<TBD-D7 tamanho máx>>`, política de
   redirect `<<TBD-D7>>`, recusa de esquema não-`https`.
2. Implementar `Validate(content []byte) error` aplicando os marcadores de recusa de
   `<<TBD-D3>>` após a normalização de `<<TBD-D3>>`. Erro deve nomear o marcador encontrado.
3. Calcular checksum `<<TBD-D6 algoritmo>>` do conteúdo bruto baixado.
4. **Não** escrever em disco neste ML — só fetch, validate, hash.

**Critérios de aceite:**
- [ ] `go build ./...` sem erros; `go vet ./...` limpo.
- [ ] Testes cobrem: URL `http://` recusada; conteúdo acima do limite truncado/recusado;
      cada marcador de `<<TBD-D3>>` recusado; conteúdo benigno aceito; hash estável.
- [ ] Nenhuma chamada de rede real nos testes (`httptest.Server`), respeitando
      `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1`.

**Comandos de validação:** `go build ./... && go vet ./... && TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test ./internal/skillsrc/...`

### ML-1B — Comando Go + composição + proveniência + gate de invocação
**Status:** ⬜ Pendente
**Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Dependência:** ML-1A ✅ (mesmo pacote).
**Arquivos afetados:**
- `internal/commands/skills.go` (novo subcomando `<<TBD-D1 nome>>`)
- `internal/commands/skills_thirdparty_test.go` (novo)
- `internal/integrations/render.go` (ponto de extensão de composição — **apensar, nunca
  sobrescrever**)
- `internal/integrations/manifest.go` (se `<<TBD-D5>>` exigir nova claim)
- `internal/skillsrc/provenance.go` (novo — writer append-only estilo
  `appendTransitionLog`, `internal/generators/roadmap.go:456`)

**Ações:**
1. Subcomando `<<TBD-D1>>` com flags `<<TBD-D1 flags de confirmação>>`; sem TTY e sem a flag
   explícita → recusa (AC1).
2. Gate de invocação `<<TBD-D2>>`; mensagem de erro deve declarar que é guardrail e apontar a
   detecção correspondente.
3. Exibir conteúdo/diff antes de gravar (AC1) e exigir confirmação de a quais agentes aplicar
   (AC3) — sugestão permitida, decisão silenciosa proibida.
4. Gravar via `Manager` (nunca `os.WriteFile` cru), respeitando o manifest.
5. Registrar proveniência em `<<TBD-D6 caminho>>`: URL, checksum, data. Falha de escrita da
   proveniência **é fatal** (diferente do `.trackfw-log`, que é best-effort) — sem registro não
   há instalação.
6. Escopo default conforme `<<TBD-D4>>`, via `resolveScope` (`integrations_flags.go:436`), sem
   quebrar o comportamento existente de `skills install`/`agents install`.

**Critérios de aceite:**
- [ ] Instalação sem confirmação em modo não-interativo é recusada.
- [ ] Conteúdo com marcador proibido é recusado antes de qualquer escrita em disco.
- [ ] Arquivo de agente pré-existente permanece íntegro (teste compara o conteúdo anterior byte
      a byte, exceto o bloco apensado).
- [ ] Registro de proveniência presente após instalação bem-sucedida e ausente após recusa.
- [ ] `trackfw skills install`/`agents install` mantêm comportamento atual
      (`internal/commands/agents_skills_test.go` verde, sem edição dos casos existentes).

**Comandos de validação:** `go build ./... && TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test ./... && go vet ./...`

---

## Wave 2 — Portes Node e Python (paralelizáveis)
> Dependências: Wave 1 completa e auditada (o Go é a referência byte-a-byte).
> ML-2A e ML-2B tocam árvores disjuntas (`npm/` × `pypi/`) → **executam em paralelo**.
> ⛔ Nenhum dos dois toca `scripts/` nem `docs/cli-parity.md` — são da Wave 3, para não colidir.

### ML-2A — Porte Node.js 1:1
**Status:** ⬜ Pendente
**Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos afetados:** `npm/src/commands/skills.js`, `npm/src/integrations/render.js`,
`npm/src/integrations/manager.js`, `npm/src/skillsrc/` (novo, espelho de `internal/skillsrc/`),
`npm/tests/skills-thirdparty.test.js` (novo)
**Ações:** porte literal do Go da Wave 1 — mesmas mensagens, mesmos códigos de saída, mesmos
limites numéricos, mesmo formato de proveniência. Rede via `fetch` seguindo o padrão de
`npm/src/commands/plugins.js`.
**Critérios de aceite:**
- [ ] `cd npm && npm test` verde, sem regressão nos testes pré-existentes.
- [ ] Saída byte-idêntica ao Go nos cenários: recusa por marcador, recusa não-interativa,
      instalação bem-sucedida, conteúdo do registro de proveniência.
**Comandos de validação:** `cd npm && npm test`

### ML-2B — Porte Python 1:1
**Status:** ⬜ Pendente
**Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos afetados:** `pypi/trackfw/commands/skills.py`, `pypi/trackfw/integrations/renderers.py`,
`pypi/trackfw/integrations/manager.py`, `pypi/trackfw/skillsrc/` (novo),
`pypi/tests/test_skills_thirdparty.py` (novo)
**Ações:** idem ML-2A, em Python puro; rede seguindo o padrão de `pypi/trackfw/commands/plugins.py`.
**Critérios de aceite:**
- [ ] `python3 -m pytest pypi/tests -q` verde, sem regressão.
- [ ] Mesma paridade byte-a-byte de saída exigida em ML-2A.
**Comandos de validação:** `python3 -m pytest pypi/tests -q`

---

## Wave 3 — Paridade, gates e documentação
> Dependências: Wave 2 completa. **Sequencial** — toca `scripts/` e docs compartilhados.

### ML-3A — Contrato de paridade + `trackfw validate` + docs
**Status:** ⬜ Pendente
**Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos afetados:** `scripts/check-cli-parity.sh`, `scripts/check-artifact-parity.sh`,
`docs/cli-parity.md`, `internal/validator/` (+ espelhos Node/Python) se `<<TBD-D2>>` exigir
regra nova de detecção, `CLAUDE.md` (seção do comando, se aplicável)
**Ações:**
1. Estender o contrato de paridade para cobrir o novo subcomando nos 3 CLIs.
2. Implementar em `trackfw validate` a detecção de `<<TBD-D2>>` (skill instalada sem registro
   de proveniência correspondente → violação), nos 3 CLIs.
3. Documentar o comando e suas exceções em `docs/cli-parity.md`.
4. Rodar `scripts/sync-integration-assets.sh` se algum asset canônico mudou.
**Critérios de aceite:**
- [ ] `make quality` passa integralmente.
- [ ] `scripts/check-integration-assets.sh` verde (árvores de assets idênticas).
- [ ] `trackfw validate` sinaliza skill instalada sem proveniência, nos 3 CLIs.
**Comandos de validação:** `make quality`

---

## Wave 4 — Barreira de revisão final
> Dependências: Wave 3 completa.

### ML-4A — Revisão de segurança final (`hades-tf`)
**Status:** ⬜ Pendente
**Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Arquivos afetados (escrita):** `docs/seguranca/2026-08-15-skills-de-terceiro-via-url.md`
(seção "Verificação pós-implementação" apensada)
**Ações:** verificar o código entregue contra cada decisão D1–D8 do ADR; tentar falsificar o
critério de recusa com payloads de evasão (unicode, HTML comment, paráfrase); confirmar que a
proveniência é fatal-on-failure e versionada.
**Critérios de aceite:**
- [ ] Cada decisão D1–D8 marcada como implementada ou desviada (com o desvio nomeado).
- [ ] Payloads de evasão testados e resultado registrado.
- [ ] Nenhum código de produto modificado por este ML.

### ML-4B — Revisão de qualidade (`hefesto-tf`)
**Status:** ⬜ Pendente
**Agente:** `hefesto-tf` (`subagent_type: hefesto-tf`)
**Arquivos afetados (escrita):** `docs/qualidade/2026-08-15-skills-de-terceiro-via-url.md` (novo)
**Ações:** avaliar duplicação entre os 3 portes, tratamento de erro, cobertura dos caminhos de
falha de rede, aderência ao padrão do subsistema `internal/integrations/`.
**Critérios de aceite:**
- [ ] Relatório emitido com achados classificados por severidade.
- [ ] Nenhum código de produto modificado por este ML.

---

## Notas de sequenciamento e autoridade

- **Git / transição para `wip`:** nenhuma branch é criada enquanto este roadmap estiver em
  `analyzing/` — `trackfw branch new` só aceita roadmap em `wip/` ou `done/`
  (`internal/commands/branch.go:127-145`).
  ⚠️ **Verificado em `~/.claude/agents/trackfw-security.md`:** o `hades-tf` tem pré-requisito de
  governança explícito — *"Do not produce deliverables without a requirement and a roadmap already
  in the `wip` state"*. Portanto o **ML-0A já exige o roadmap em `wip/`**. Sequência correta
  antes de despachar o ML-0A:
  `trackfw roadmap move <nome> wip` → `trackfw branch new feat/<slug>` → commit de governança
  (Zeus) → dispatch do ML-0A. A branch **não** nasce no ML-0B.
- **Commits:** exclusivos do `trackfw_architect`. Todo especialista devolve o trabalho não
  commitado.
- **Auditores não editam código de produto:** `hades-tf` e `hefesto-tf` escrevem apenas os
  documentos designados em seus MLs.
