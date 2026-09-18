---
status: done
date: 2026-08-15
req: "docs/req/REQ-2026-08-15-instalacao-de-skills-de-terceiro-via-url-para-agentes-especialistas.md"
squad: "hades-tf, apolo-tf, hefesto-tf"
---

# Roadmap: instalacao de skills de terceiro via URL para agentes especialistas

> Created: 2026-08-15 | Reescrito: 2026-08-15 (Zeus) | Wave 0 ✅ | Status: done

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

> ⚠️ **O título deste roadmap, o da REQ e o slug da branch dizem só "skills" — é histórico.**
> O escopo real, a partir da emenda acima, é **artefato de terceiro: skill E agent**. Quem for
> implementar deve ler "artefato de terceiro" onde o título diz "skill"; o subcomando vale para
> `trackfw skills` e `trackfw agents` (D1). Títulos e slug não foram renomeados de propósito, para
> não quebrar o vínculo REQ↔roadmap↔branch já commitado.
>
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
- [x] AC1 — Comando baixa o conteúdo e **nunca instala sem confirmação**: exibe o conteúdo
      completo (ou diff do que seria adicionado) antes de gravar; em modo não-interativo/CI
      recusa por padrão, exigindo flag explícita de confiança na fonte.
- [x] AC2 — Instalação é recusada se o conteúdo baixado tentar redefinir fronteiras de agente
      (Git authority / governance prerequisite / mode lock), pelo critério objetivo fixado na
      Wave 0.
- [x] AC3 — A skill **nunca substitui** o arquivo de um agente do catálogo: é sempre seção
      suplementar apensada/referenciada. O usuário confirma explicitamente a quais agentes se
      aplica; o `trackfw` sugere mas não decide sozinho.
- [x] AC4 — Escopo de instalação de artefato de terceiro é **`project` por padrão** (D4 —
      exceção escopada ao `ADR-2026-07-25` D1, que segue valendo `global` para o catálogo);
      escopo `global` exige confirmação explícita adicional. Verificação: `resolveScope` retorna
      `project` para third-party sem `--scope` e `global` para `skills install` do catálogo.
- [x] AC5 — Proveniência auditável registrada (URL, hash/checksum, data) em artefato
      versionado do projeto.
- [x] AC6 — Comportamento idêntico nos 3 CLIs (Go · Node · Python).
- [x] AC7 — `make quality` passa sem novas divergências de paridade.
- [x] AC8 — Revisão do `hades-tf` documentada em parecer + ADR **antes** do primeiro ML de
      implementação.
- [x] AC9 — Restrição de invocação implementada via env var `TRACKFW_ORCHESTRATOR_SESSION`
      (**guardrail declarado**) + detecção real pela regra `thirdparty_artifact_has_provenance`
      em `trackfw validate` (D2). Verificação: a mensagem de recusa contém a palavra "guardrail"
      e o nome da regra; nenhum texto de doc/erro apresenta a env var como prevenção.
- [x] AC10 — Gate de runtime recorrente: **nenhum caminho de código instala artefato de
      terceiro (skill, agent ou plugin) sem parecer prévio do `hades-tf`**. O comando baixa
      para quarentena, para, e só consuma mediante referência ao parecer favorável — handshake
      `third-party fetch` → `.trackfw/thirdparty-quarantine/<checksum>.json` → aprovação em
      `.trackfw/thirdparty-provenance.json` vinculada por SHA-256 → `third-party install
      --checksum <sha256>` (D8). **`trackfw plugins install` fica fora desta REQ** (D8e), em REQ
      separada. Vale para os dois caminhos de entrada: comando
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
**Status:** ✅ Concluído (2026-08-15) — ADR: `docs/adr/ADR-2026-08-15-gate-de-duas-fases-para-artefatos-de-terceiro-quarentena-parecer-vinculado-por-checksum-e-deteccao-por-proveniencia-versionada.md`
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
- [ ] Nenhum placeholder de decisão restante **fora da própria seção ML-0B** (inclusive no bloco
      de AC consolidado) — a seção ML-0B cita os tokens literalmente ao descrever a tarefa, o que
      é esperado e não conta.
- [ ] Nenhum AC delega verificação ao ADR ("conforme a Wave 0") — todo AC verificável por si.
- [ ] `trackfw validate` passa.

**Comandos de validação:**
```bash
trackfw validate
R=docs/roadmaps/wip/ROADMAP-2026-08-15-instalacao-de-skills-de-terceiro-via-url-para-agentes-especialistas.md
# exclui a seção ML-0B (que cita os tokens ao descrever a própria tarefa)
sed '/^### ML-0B/,/^## Wave 1/d' "$R" | grep -c "TBD-D"            # deve ser 0
sed '/^### ML-0B/,/^## Wave 1/d' "$R" | grep -ci "conforme .* Wave 0"   # deve ser 0
```

---

## Wave 1 — Núcleo Go: fetch, quarentena, proveniência e comandos de duas fases
> Dependências: Wave 0 completa (ML-0A ✅ + ML-0B ✅).
> Decisões fixadas em `docs/adr/ADR-2026-08-15-gate-de-duas-fases-para-artefatos-de-terceiro-quarentena-parecer-vinculado-por-checksum-e-deteccao-por-proveniencia-versionada.md` (D1–D8).
> ⚠️ **MLs sequenciais entre si** — os três compartilham o pacote novo `internal/thirdparty`.

### ML-1A — Pacote `internal/thirdparty`: fetch (D7), validação de marcadores (D3) e checksum (D6)
**Status:** ✅ Concluído (2026-08-15) — 20 testes verdes; auditado contra artefatos reais pelo arquiteto
**Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos afetados:**
- `internal/thirdparty/fetch.go` (novo)
- `internal/thirdparty/markers.go` (novo)
- `internal/thirdparty/fetch_test.go`, `internal/thirdparty/markers_test.go` (novos)

**Ações:**
1. `Fetch(rawURL string) ([]byte, error)` — política de rede D7, exata:
   - recusar antes do primeiro `Get` se `url.Scheme != "https"`;
   - `http.Client{Timeout: 30 * time.Second}` **próprio deste pacote** — não reusar o `httpClient`
     compartilhado de `internal/plugins/plugins.go`, para não alterar o comportamento de plugins;
   - `CheckRedirect` customizado: **máximo 3 hops**, revalidando `Scheme == "https"` a cada hop;
   - `io.LimitReader(resp.Body, maxSize+1)` com `maxSize = 2 << 20` (2 MiB), erro se `len > maxSize`;
   - recusar (não avisar) se `Content-Type` não for `text/plain`, `text/markdown` ou
     `text/x-markdown` (com ou sem `; charset=`).
2. `CheckMarkers(content []byte) (matched []string)` — critério D3, na ordem exata:
   (1) remover comentários HTML `<!-- ... -->`; (2) **remover blocos cercados** (``` e ~~~) —
   emenda de D3, linhas dentro de fence não são headings; (3) NFKC; (4) casefold;
   (5) colapsar espaços internos + strip; (6) casar só linhas `^#{1,6}\s+` contra a lista:
   `Git authority`, `Mode lock`, `Governance prerequisite`, `Reporting boundary`,
   `Scope boundary`, `Dispatch contract`. Retorna os marcadores encontrados (nomeados no erro).
3. `Checksum(raw []byte) string` — SHA-256 hex dos **bytes brutos**, antes de qualquer
   normalização. Reusar `contentHash` de `internal/integrations/manager.go` se a assinatura
   permitir; se não, replicar o algoritmo e citar a origem em comentário.
4. **Não escrever em disco neste ML** — só fetch, check, hash.

**Critérios de aceite:**
- [ ] `go build ./...` sem erros; `go vet ./...` limpo.
- [ ] Testes cobrem: `http://` recusado; esquema não-http(s) recusado; downgrade para `http` em
      redirect recusado; 4º redirect recusado; conteúdo > 2 MiB recusado; `Content-Type: text/html`
      recusado; **cada um dos 6 marcadores** recusado como heading em `#` e em `######`;
      marcador dentro de bloco cercado **aceito** (emenda de D3); marcador com caractere de
      **largura total / compatibility-equivalent** (ex. `＃＃ Ｇit authority`) recusado — é o que
      NFKC dobra. **Homoglifo de outro alfabeto (ex. `а` cirílico em "Git аuthority") NÃO é
      coberto: o comportamento esperado é PASSAR**, conforme a seção "o que NÃO cobre" de D3 —
      escreva o teste afirmando que passa, não tente fazê-lo falhar nem invente normalização que
      o ADR não especifica. Além disso: conteúdo benigno aceito; checksum estável e igual ao
      `sha256sum` do arquivo.
- [ ] Nenhuma chamada de rede real (`httptest.Server`), compatível com
      `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1`.

**Comandos de validação:**
`go build ./... && go vet ./... && TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test ./internal/thirdparty/...`

---

### ML-1B — Quarentena (D8a/b) e proveniência (D6) — persistência fatal-on-failure
**Status:** ✅ Concluído (2026-08-15) — 34 testes verdes; auditado por falsificação do arquiteto
**Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Dependência:** ML-1A ✅ (mesmo pacote).
**Arquivos afetados:**
- `internal/thirdparty/quarantine.go` (novo)
- `internal/thirdparty/provenance.go` (novo)
- `internal/thirdparty/quarantine_test.go`, `internal/thirdparty/provenance_test.go` (novos)

**Ações:**
1. **Quarentena (D8a/b):** escrever `.trackfw/thirdparty-quarantine/<checksum>.json` com os campos
   exatos: `schema_version: 1`, `url`, `checksum_sha256`, `fetched_at` (RFC3339 UTC),
   `content_base64` (conteúdo **integral embutido** — nunca caminho para outro arquivo),
   `marker_check: {result: "pass"|"fail", matched_markers: [...]}`, `kind` (`skill`|`agent`),
   `requested_targets: [...]`. Escrita atômica (`os.CreateTemp` + `os.Rename`, padrão de
   `internal/integrations/manager.go`).
2. **Proveniência (D6):** ler/escrever `.trackfw/thirdparty-provenance.json` com
   `schema_version: 1` e `entries` **chaveado por destino**, cada entrada com `url`,
   `checksum_sha256`, `installed_at`, `approved_by`, `review_reference`, `scope`,
   `marker_override`.
3. **Falha de escrita da proveniência é FATAL** — retorna erro que aborta a instalação. Isto é o
   oposto de `appendTransitionLog` (`internal/generators/roadmap.go:456`), que é best-effort;
   registrar essa divergência em comentário no código para quem for espelhar o padrão errado.
4. **Fail-closed na leitura (D8f):** arquivo ausente, JSON inválido, ou `schema_version`
   incompatível → erro, **nunca** degradar para "proveniência vazia" silenciosamente. Espelhar o
   rigor de `loadManifest` em `internal/integrations/manifest.go`.
5. `VerifyApproval(root, checksum, dest string) error` — a prova de D8c: só passa se existir
   entrada com **aquele checksum exato** e `approved_by` não-vazio. Booleano "aprovado" solto não
   é aceito por construção.
   > 📌 **Assinatura final entregue (portar assim na Wave 2):** todas as funções do pacote levam
   > `root` como **primeiro parâmetro** — `QuarantinePath`, `WriteQuarantine`, `ReadQuarantine`,
   > `ProvenancePath`, `LoadProvenance`, `WriteProvenance`, `UpsertProvenanceEntry`,
   > `VerifyApproval`. O roadmap original omitia `root`; sem ele não há como localizar
   > `.trackfw/thirdparty-*` sem depender de cwd implícito. Segue a convenção de
   > `internal/integrations/manager.go` (`manifestPath(root)`).
   > Conveniências adicionais entregues e usadas pelo ML-1C: `NewQuarantineEntry(...)` e
   > `QuarantineEntry.DecodeContent()`.
   > **Divisão do fail-closed (D8f), verificada por auditoria:** `LoadProvenance` com arquivo
   > ausente retorna vazio (espelha `loadManifest`); quem impõe a recusa é `VerifyApproval`, que
   > falha sem entrada para o destino. `ReadQuarantine` trata ausência **sempre** como erro. Ambas
   > são rígidas contra JSON malformado e `schema_version` incompatível. Não é inconsistência —
   > portar exatamente assim.

**Critérios de aceite:**
- [ ] Round-trip de quarentena e de proveniência preserva todos os campos do schema.
- [ ] `content_base64` decodificado é byte-idêntico ao original.
- [ ] Erro de escrita de proveniência (diretório read-only no teste) **aborta** e retorna erro.
- [ ] `schema_version: 2` na leitura → erro, não fallback.
- [ ] `VerifyApproval` recusa: checksum ausente, checksum diferente, `approved_by` vazio.
- [ ] `go vet ./...` limpo.

**Comandos de validação:**
`go build ./... && go vet ./... && TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test ./internal/thirdparty/...`

---

### ML-1C — Subcomandos `third-party fetch` / `third-party install` (D1) + composição (D5) + guardrail (D2)
**Status:** ✅ Concluído (2026-08-15) — 10 testes verdes; ver relatório de entrega para desvio de escopo (plan.go) e imprecisões de ADR encontradas
**Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Dependência:** ML-1B ✅.
**Arquivos afetados:**
- `internal/commands/integrations_flags.go` (registrar o subcomando `third-party` no
  `newIntegrationsLifecycleCmd`, valendo para `skills` **e** `agents` — D1)
- `internal/commands/integrations_thirdparty.go` (novo — implementação das duas fases)
- `internal/commands/integrations_thirdparty_test.go` (novo)
- `internal/integrations/render.go` (ponto de extensão D5: linha de referência entre os marcadores
  `<!-- trackfw:thirdparty-skills:start -->` / `<!-- trackfw:thirdparty-skills:end -->`, padrão
  idempotente de `injectOrUpdateRules` em `internal/generators/agentfiles.go`)
- `internal/integrations/manifest.go` (nova claim para o destino de terceiro — hash isolado)

**Ações:**
1. **Fase 1 — `trackfw <skills|agents> third-party fetch <url>`:** `Fetch` → `CheckMarkers` →
   `Checksum` → grava quarentena. **Nunca instala.** Imprime o caminho do artefato de revisão.
   Recusa por marcador é o **default**; `--force-thirdparty-markers` sobrescreve e grava
   `marker_override: true` na proveniência.
2. **Fase 2 — `trackfw <skills|agents> third-party install --checksum <sha256>`:** lê a quarentena,
   recalcula o SHA-256, chama `VerifyApproval` (D8c) e só então grava. Exibe conteúdo e destino
   resolvido antes de gravar (AC1); sem TTY, recusa salvo `--yes-i-trust-this-source`.
3. **Alvos (AC3):** o comando pode **sugerir** agentes por palavra-chave, mas exige confirmação
   explícita da lista. Decisão silenciosa é proibida.
4. **Destino (D5):** `<target>/skills/thirdparty/<slug>.md`, resolvido pelo `Manager` por target —
   **nunca** hardcodar `.claude/`. O arquivo do agente do catálogo recebe **só** a linha de
   referência entre os marcadores novos; jamais o conteúdo apensado.
5. **Escopo (D4):** default `project` para third-party, via `resolveScope`
   (`integrations_flags.go:436`), usando *flag-set* (`cmd.Flags().Changed("scope")`) e **sem
   alterar** o default `global` de `skills install`/`agents install` do catálogo.
6. **Guardrail (D2):** checar `TRACKFW_ORCHESTRATOR_SESSION`; a mensagem de recusa **deve dizer que
   é guardrail, não controle**, e apontar a regra `thirdparty_artifact_has_provenance` como a
   detecção real. Proibido qualquer texto que a apresente como prevenção.
7. Toda gravação passa pelo `Manager` — nunca `os.WriteFile` cru.

**Critérios de aceite:**
- [ ] `third-party fetch` **nunca** cria arquivo em `.claude/`/`<target>/` — só em
      `.trackfw/thirdparty-quarantine/`.
- [ ] `third-party install` sem entrada de proveniência aprovada **falha** (AC10).
- [ ] Conteúdo trocado após aprovação (checksum diferente) **falha** — teste explícito de TOCTOU.
- [ ] Arquivo do agente do catálogo permanece **byte-idêntico** exceto pelas linhas entre os
      marcadores `trackfw:thirdparty-skills` — comparação byte a byte no teste.
- [ ] `trackfw agents update` executado após a instalação **não** reporta `StateModified` no
      artefato do catálogo (regressão que D5 existe para evitar).
- [ ] Escopo default de third-party é `project`; `skills install`/`agents install` seguem `global`
      (`internal/commands/agents_skills_test.go` verde, **sem editar** os casos existentes).
- [ ] Mensagem do guardrail contém a palavra "guardrail" e o nome da regra de detecção.

**Comandos de validação:**
`go build ./... && TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test ./... && go vet ./...`

---

## Wave 2 — Portes Node e Python (paralelizáveis)
> Dependências: Wave 1 completa e auditada (o Go é a referência byte-a-byte).
> ML-2A e ML-2B tocam árvores disjuntas (`npm/` × `pypi/`) → **executam em paralelo**.
> ⛔ Nenhum dos dois toca `scripts/` nem `docs/cli-parity.md` — são da Wave 3, para não colidir.

### ML-2A — Porte Node.js 1:1
**Status:** ✅ Concluído (2026-08-15) — 566/566 testes npm verdes
**Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos afetados:** `npm/src/integrations/index.js`, `npm/src/integrations/render.js`,
`npm/src/integrations/manager.js`, `npm/src/thirdparty/` (novo — espelho de
`internal/thirdparty/`: `fetch.js`, `markers.js`, `quarantine.js`, `provenance.js`),
`npm/src/integrations/index.js` — `buildPlans` (:45), campo `projectRoot` + reaplicação de referências (D9),
`npm/src/commands/thirdparty.js` (novo), `npm/tests/thirdparty.test.js` (novo)
**Ações:** porte literal do Go da Wave 1 — mesmas mensagens, mesmos códigos de saída, mesmos
limites numéricos (30s, 2 MiB, 3 redirects), mesmo schema JSON de quarentena e de proveniência,
mesma ordem de normalização de D3 (inclusive a remoção de blocos cercados).
> 🔴 **Risco de paridade descoberto no ML-1A — leia antes de codar.** O Go implementou a remoção
> de blocos cercados como **line-scanner com estado explícito** (regras CommonMark: mesmo caractere
> delimitador, fechamento com repetição ≥ abertura), porque o `regexp` do Go (RE2) **não suporta
> backreference** e o padrão sugerido pelo ADR daria panic em runtime. Node e Python **suportam**
> backreference — e é exatamente por isso que a tentação de usar um regex de uma linha aqui é uma
> armadilha: o comportamento divergiria do Go em casos de borda (fence não fechado, fechamento mais
> curto que a abertura, fence indentado). **Porte o algoritmo line-scanner, não um regex.**
> Detalhe em `vault/notes/go-regexp-re2-sem-backreference-fenced-block-removal-2026-08-15.md`.
>
> 🔴 **São TRÊS schemas a portar, não dois (D9).** Além de `thirdparty-quarantine/<checksum>.json`
> e `thirdparty-provenance.json`, existe **`.trackfw/thirdparty-references.json`** — registro de
> quais agentes referenciam qual skill, lido no caminho de render. Sem ele, o bloco de referência
> some no próximo `agents update` e a anexação vira drift (`StateModified`). O porte precisa
> replicar também: campo `ProjectRoot`/`projectRoot`/`project_root` em `PlanRequest`, e a
> reaplicação **depois** do `Render`, opt-in (root vazio → saída byte-idêntica à anterior).
> Ver **D9** e **D10** no ADR — D10 lista as imprecisões já resolvidas (regra de mesmo escopo do
> `--apply-to`; quem escreve a entrada de proveniência; ambiguidade de `requested_targets`). Rede via `fetch`
seguindo o padrão de `npm/src/commands/plugins.js`.
**Critérios de aceite:**
- [ ] `cd npm && npm test` verde, sem regressão nos testes pré-existentes.
- [ ] Saída byte-idêntica ao Go nos cenários: recusa por marcador, recusa não-interativa,
      instalação bem-sucedida, conteúdo do registro de proveniência.
**Comandos de validação:** `cd npm && npm test`

### ML-2B — Porte Python 1:1
**Status:** ✅ Concluído (2026-08-15) — 1207 testes pytest verdes
**Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos afetados:** `pypi/trackfw/integrations/command.py`,
`pypi/trackfw/integrations/renderers.py`, `pypi/trackfw/integrations/manager.py`,
`pypi/trackfw/thirdparty/` (novo — espelho: `fetch.py`, `markers.py`, `quarantine.py`,
`provenance.py`), `pypi/trackfw/integrations/catalog.py` — `plan_deployments` (:55), parâmetro
`project_root` + reaplicação de referências (D9), `pypi/tests/test_thirdparty.py` (novo)
**Ações:** idem ML-2A, em Python puro; rede seguindo o padrão de
`pypi/trackfw/commands/plugins.py`. Atenção à normalização NFKC (`unicodedata.normalize`) para
bater byte a byte com o Go.
**Critérios de aceite:**
- [ ] `python3 -m pytest pypi/tests -q` verde, sem regressão.
- [ ] Mesma paridade byte-a-byte de saída exigida em ML-2A.
**Comandos de validação:** `python3 -m pytest pypi/tests -q`

---

## Wave 3 — Paridade, gates e documentação
> Dependências: Wave 2 completa. **Sequencial** — toca `scripts/` e docs compartilhados.

### ML-3A — Contrato de paridade + `trackfw validate` + docs
**Status:** ✅ Concluído
**Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos afetados:** `scripts/check-cli-parity.sh`, `scripts/check-artifact-parity.sh`,
`docs/cli-parity.md`, `internal/validator/validator_thirdparty_provenance.go` (novo) + espelhos
Node/Python, `scripts/check-thirdparty-parity.sh` (novo, adicionado ao alvo `parity` do
`Makefile`, cobrindo os **três** schemas de D9), `CLAUDE.md` (seção do comando)
**Ações:**
0-bis. Renomear o teste Go `TestFetch_RefusesFourthRedirect` → `TestFetch_RefusesThirdRedirect`
   (débito de D7-bis: o nome descreve mal o comportamento, que recusa o 3º hop, não o 4º).
0. **Decisão JÁ TOMADA pelo arquiteto — ver D11 no ADR. Não reabrir.** A `Claim` de
   `internal/integrations/manifest.go` ganha `Origin string \`json:"origin,omitempty"\`` —
   `""` = catálogo (retrocompatível, sem migração), `"thirdparty"` = artefato de terceiro. Aplicar
   igual nos 3 CLIs. Usar a proveniência como índice foi **rejeitado por ser circular**: o ramo (i)
   detecta destino de terceiro *sem* entrada de proveniência, então a proveniência não pode ser o
   que define "é de terceiro".
1. Estender o contrato de paridade para cobrir o novo subcomando nos 3 CLIs.
2. Implementar em `trackfw validate` a regra **`thirdparty_artifact_has_provenance`** (D2), nos 3
   CLIs, **bidirecional**: (i) destino gerenciado com origem de terceiro sem entrada de
   proveniência → violação `error`; (ii) entrada de proveniência cujo `checksum_sha256` não bate
   com o SHA-256 do conteúdo instalado → violação `error`. A regra **nunca faz fetch de rede**
   (D6) — compara só contra o conteúdo local.
3. Documentar em `docs/cli-parity.md`: o comando, o schema dos **três** JSONs (quarentena,
   proveniência e referências — D9), e **a exceção de escopo de D4** (third-party é `project`; catálogo segue `global` por `ADR-2026-07-25` D1).
   A doc do comando **deve** conter a seção "o que o critério de marcadores NÃO cobre" (D3) —
   qualquer texto que sugira que o critério filtra adversário competente é bug.
4. Rodar `scripts/sync-integration-assets.sh` se algum asset canônico mudou.
**Critérios de aceite:**
- [ ] `make quality` passa integralmente.
- [ ] `scripts/check-integration-assets.sh` verde (árvores de assets idênticas).
- [ ] `trackfw validate` sinaliza artefato instalado sem proveniência **e** proveniência com
      checksum divergente, nos 3 CLIs, com saída byte-idêntica.
- [ ] `docs/cli-parity.md` documenta a exceção de escopo de D4, a limitação de D3 e os três
      schemas de D9.
- [ ] UX de D10.1: **decidido por KG em 2026-08-15 — a recusa fica.** Garantir apenas que a
      mensagem traga o comando exato de remediação, e que o comportamento seja idêntico nos 3 CLIs.
**Comandos de validação:** `make quality`

---

---

### ML-3B — Microlote corretivo: `installed_sha256` na proveniência (D2-bis)
**Status:** ✅ Concluído (apolo-tf, 2026-08-15) — aguardando auditoria/commit do arquiteto
**Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Dependência:** ML-3A ✅ (commitado).
**Origem:** auditoria do arquiteto sobre o ML-3A — ver **D2-bis** no ADR.
**Arquivos afetados (3 stacks):**
- `internal/thirdparty/provenance.go` + testes · `internal/validator/validator_thirdparty_provenance.go` + testes
- `npm/src/thirdparty/provenance.js` · `npm/src/validator/index.js` · `npm/tests/`
- `pypi/trackfw/thirdparty/provenance.py` · `pypi/trackfw/validator.py` · `pypi/tests/`
- `scripts/check-thirdparty-parity.sh` · `docs/cli-parity.md`

**Ações:**
1. Adicionar `installed_sha256` à entrada de proveniência (SHA-256 dos bytes **normalizados**,
   calculado na instalação pelo mesmo código que grava o arquivo). `checksum_sha256` **não muda**.
2. Bump de `schema_version` da proveniência para `2`. Sem migração: a feature é inédita.
3. Reescrever o ramo (ii) da regra `thirdparty_artifact_has_provenance` para
   `sha256(arquivo instalado) == entry.installed_sha256`. **Remover a dependência do arquivo de
   quarentena** neste ramo.
4. A quarentena **continua** sendo escrita e commitada; sua ausência **deixa de ser erro** desta
   regra (hoje é fail-closed — mudar).
5. Atualizar o script de paridade e a doc.

**Critérios de aceite:**
- [ ] Ramo (ii) detecta adulteração pós-instalação **sem** consultar a quarentena — teste explícito
      que apaga `.trackfw/thirdparty-quarantine/` e mostra que (a) a regra segue detectando
      adulteração e (b) uma instalação íntegra **não** vira violação.
- [ ] Instalação legítima com conteúdo bruto **não-canônico** (markdown com linha em branco final)
      **não** gera violação — é o falso-positivo que D2-bis existe para matar.
- [ ] `checksum_sha256` intocado e o vínculo de aprovação de D8c preservado.
- [ ] Comportamento e saída idênticos nos 3 CLIs; `make quality` verde.

**Comando de validação:** `make quality`

## Wave 4 — Barreira de revisão final
> Dependências: Wave 3 completa.

### ML-4A — Revisão de segurança final (`hades-tf`)
**Status:** ✅ Concluído (2026-08-15) — libera para merge; achados foram para o ML-4C
**Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Arquivos afetados (escrita):** `docs/seguranca/2026-08-15-skills-de-terceiro-via-url.md`
(seção "Verificação pós-implementação" apensada)
**Ações:** verificar o código entregue contra cada decisão D1–D8 do ADR; tentar falsificar o
critério de recusa com payloads de evasão (unicode, HTML comment, paráfrase, fence); confirmar que a
proveniência é fatal-on-failure e versionada, e que o TOCTOU está fechado por checksum (D8c).
**Critérios de aceite:**
- [ ] Cada decisão D1–D8 marcada como implementada ou desviada (com o desvio nomeado).
- [ ] Payloads de evasão testados e resultado registrado.
- [ ] Nenhum código de produto modificado por este ML.

### ML-4B — Revisão de qualidade (`hefesto-tf`)
**Status:** ✅ Concluído (2026-08-15) — 0 bloqueante; achados foram para o ML-4C
**Agente:** `hefesto-tf` (`subagent_type: hefesto-tf`)
**Arquivos afetados (escrita):** `docs/qualidade/2026-08-15-skills-de-terceiro-via-url.md` (novo)
**Ações:** avaliar duplicação entre os 3 portes, tratamento de erro, cobertura dos caminhos de
falha de rede, aderência ao padrão do subsistema `internal/integrations/`.
**Critérios de aceite:**
- [ ] Relatório emitido com achados classificados por severidade.
- [ ] Nenhum código de produto modificado por este ML.

---

---

### ML-4C — Microlote corretivo da barreira (D3-ter, D4-bis, D6-bis)
**Status:** ✅ Concluído (2026-08-15) — `make quality` verde, `apolo-tf` devolveu não commitado
**Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Dependência:** ML-4A ✅ e ML-4B ✅ (barreira executada).
**Origem:** achados da Wave 4 verificados pelo arquiteto + decisões de KG (2026-08-15).
**Arquivos afetados (3 stacks):**
- Go: `internal/thirdparty/markers.go`, `fetch.go`, `quarantine.go`, `provenance.go`,
  `internal/commands/integrations_thirdparty.go`, `internal/integrations/render.go` (+ testes)
- Node: `npm/src/thirdparty/{markers,fetch,quarantine,provenance}.js`,
  `npm/src/commands/thirdparty.js`, `npm/src/integrations/render.js` (+ `npm/tests/`)
- Python: `pypi/trackfw/thirdparty/{markers,fetch,quarantine,provenance}.py`,
  `pypi/trackfw/commands/thirdparty.py`, `pypi/trackfw/integrations/renderers.py` (+ `pypi/tests/`)
- `scripts/check-thirdparty-parity.sh`, `docs/cli-parity.md`

**Ações:**
1. **D3-ter(a) — fence não fechado deixa de conceder imunidade.** Fence **sem** fechamento não é
   fence: as linhas seguintes voltam a ser escaneadas. Fence **fechado** continua ignorado.
   ⚠️ O teste `TestCheckMarkers_UnclosedFenceDropsRestOfDocument` (e espelhos) assertava o
   comportamento que sai — **substituir**, não afrouxar.
2. **D3-ter(b) — comentário HTML neutralizado, não apagado.** O conteúdo dentro de `<!-- -->`
   continua sendo escaneado; só os delimitadores saem.
3. **D3-ter(c) — unificar o casefold** nos 3 CLIs (hoje Go/Node usam lowercase simples e Python usa
   `str.casefold()`) e cobrir por teste de paridade.
4. **D4-bis — `--scope global` com aviso próprio.** Continua permitido, mas emite aviso explícito
   nomeando que aquela instalação **nunca será verificada por `trackfw validate`**, e exige
   confirmação **própria**, distinta de `--yes-i-trust-this-source`.
5. **D6-bis — redigir a query string** antes de persistir em quarentena e proveniência: gravar
   `esquema://host/caminho` com query (e `userinfo`, se houver) substituídos por `[redacted]`.
   A URL redigida é a que vai para os dois arquivos; a URL completa é usada só em memória, para o
   fetch.
6. **Menores, do `hefesto-tf`:** guarda de `end < start` ao localizar o marcador de fechamento em
   `ApplyThirdPartyReferences`; teste de resposta HTTP não-200 no `Fetch` de Go e Node (hoje só
   existe em Python).
7. Atualizar a lista de lacunas declaradas de D3 em `docs/cli-parity.md` — remover fence-aberto e
   comentário-HTML de "não coberto", já que passam a ser cobertos.

**Critérios de aceite:**
- [ ] Arquivo iniciado por ``` sem fechamento **e** contendo marcador → **recusado**, nos 3 CLIs.
      (Hoje retorna `[]` — reproduzido pelo arquiteto.)
- [ ] Marcador dentro de `<!-- -->` → **recusado**, nos 3 CLIs. (Hoje retorna `[]`.)
- [ ] Fence **fechado** contendo marcador → **continua aceito**; o parecer
      `docs/seguranca/2026-08-15-skills-de-terceiro-via-url.md` **não** pode passar a ser recusado
      (regressão da emenda original de D3 — testar contra o arquivo real).
- [ ] `--scope global` imprime aviso contendo "não será verificada"/`trackfw validate` e exige a
      confirmação própria; mensagem idêntica nos 3 CLIs.
- [ ] URL com query (`?token=abc`) → persistida como `[redacted]` em **ambos** os arquivos;
      teste que faz `grep` do token nos JSONs gravados e exige **zero** ocorrências.
- [ ] Casefold idêntico nos 3 CLIs, coberto pelo script de paridade.
- [ ] `make quality` verde; nenhum teste pré-existente afrouxado (os que asseguravam a semântica
      removida devem ser **substituídos** e citados no relatório).

**Comando de validação:** `make quality`

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

---

## Evidência de fechamento (auditoria do arquiteto, 2026-08-15)

| AC | Evidência |
|---|---|
| AC1 | Fase 1 nunca escreve fora da quarentena (teste); sem TTY recusa salvo flag explícita |
| AC2 | Marcadores recusados antes de qualquer escrita; evasões de fence aberto e comentário HTML fechadas no ML-4C |
| AC3 | D5 — arquivo separado + linha entre marcadores; agente do catálogo byte-idêntico fora do bloco; `agents update` segue `StateCurrent` |
| AC4 | Escopo default `project` (D4); `global` permitido com aviso próprio e flag dedicada (D4-bis, decisão de KG) |
| AC5 | `.trackfw/thirdparty-provenance.json` versionado, com `checksum_sha256` + `installed_sha256`; URL redigida (D6-bis) |
| AC6 | Paridade três-vias verificada pelo arquiteto em 13 fixtures adversariais + `scripts/check-thirdparty-parity.sh` no alvo `parity` |
| AC7 | `make quality` exit 0, 229 checagens OK |
| AC8 | Parecer `hades-tf` (ML-0A) antes de qualquer implementação; verificação pós-implementação (ML-4A) libera para merge |
| AC9 | Guardrail declarado como guardrail na mensagem, com a regra de detecção nomeada; nenhum texto o apresenta como prevenção |
| AC10 | `VerifyApproval` vinculado por checksum; TOCTOU testado e fechado; sem aprovação, install falha nos 3 CLIs |

**Débito aberto e rastreado:** `trackfw plugins install` segue sem gate (D8e) — REQ própria já aberta
em `docs/req/REQ-2026-08-15-gate-de-seguranca-para-trackfw-plugins-install-...md`.

**Limites declarados, não resolvidos por desenho:** o critério de marcadores é tripwire, não filtro
(paráfrase, indireção e homoglifo de outro alfabeto passam — D3); a prova de aprovação é
git-detectável, não inforjável (D8c); a detecção só alcança o que foi commitado (D2/D11).
