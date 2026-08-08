# Changelog

Todas as mudanças notáveis deste projeto são documentadas neste arquivo.

O formato segue [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
e este projeto adere a [Semantic Versioning](https://semver.org/).

> Entradas anteriores a esta versão foram reconstruídas a partir do
> histórico de commits (convenção `feat`/`fix`/`refactor`) para fins de
> backfill. A partir de `2.16.0`, este arquivo é atualizado como parte
> obrigatória do protocolo de release (ver `CLAUDE.md`).

## [6.6.0] - 2026-08-08

### Added

- **`trackfw adr new/list --scope project|global`** (#149) — novo flag nos 3 CLIs (default
  `project`, comportamento atual 100% preservado). `--scope global` escreve/lista em
  `~/.trackfw/adr/ADR-YYYY-MM-DD-<slug>.md` — mesmo diretório-base de `~/.trackfw/scripts/`
  (credential-guard) — sem exigir `trackfw.yaml`/raiz de projeto no cwd. Python ganhou
  `adr list`, que não existia antes desta feature. `--dir`/`--status` pré-existentes do
  Python (drift antigo) ficam intactos, passam a ser mutuamente exclusivos com
  `--scope global`.
- **Auto-registro de `~/.trackfw/adr` em `adr_dirs` via `trackfw update`** (#150) — o
  comando (escopo projeto, 3 CLIs) passa a registrar `~/.trackfw/adr` em `adr_dirs` do
  `trackfw.yaml` do projeto, mas somente se esse diretório existir e contiver ao menos um
  `ADR-*.md`. Escrita cirúrgica e idempotente, preserva comentários/demais chaves do arquivo
  byte a byte; nunca escreve "no escuro" contra um diretório vazio/inexistente.
- **Escolha de escopo (local/global) ao gerar ADR draft em `req new`** (#150) — no fluxo
  interativo (Go+Node.js) que detecta domínios e gera ADR drafts via probes, um único prompt
  por sessão de REQ pergunta se os ADRs são locais (default) ou globais. Sem TTY,
  comportamento inalterado. Python não tem esse fluxo de probes/ADR-draft — gap de paridade
  pré-existente, agora documentado em `docs/cli-parity.md`.

## [6.5.1] - 2026-08-08

### Fixed

- **`trackfw update harness` não gerava o script global de credential-guard** (#147) —
  o wiring de hooks `*-credential-guard` (Claude/Codex/Gemini/Cursor/Copilot/Kiro)
  apontava para `~/.trackfw/scripts/trackfw-credential-guard.sh`, mas nenhum dos 3 CLIs
  (Go/Node.js/Python) chamava a função que gera esse arquivo — hooks instalados
  apontando para um script inexistente, falhando com "No such file or directory".
  `update harness` passa a gerar o script uma vez no início do fluxo (pulado em
  `--dry-run`).
- **JSON de `trackfw update harness --json` corrompido pelo fix acima (Go)** (#147) —
  `GenerateGlobalCredentialGuardScript` imprimia um checkmark de sucesso via
  `fmt.Printf` incondicionalmente, vazando texto solto para o stdout antes do JSON.
  Corrigido para escrever silenciosamente, alinhado ao padrão já usado por
  `harnessClaudeSkillTarget`.

### Changed

- **Remoção de geradores legados órfãos** (#147) — `InstallCodex`/`InstallCopilot`/
  `InstallCursor`/`InstallGemini`/`InstallWindsurf`/`InstallAmazonQ` (Go) e o wrapper
  `installGlobalSkill()`, código pré-catálogo sem chamadores em produção, superados
  pelo sistema `internal/integrations`. Em Node.js/Python só as funções
  `installCodex`/`install_codex` foram removidas — os dicts de fixture usados pelos
  testes de reconhecimento de conteúdo legado foram preservados.

## [6.5.0] - 2026-08-07

### Added

- **Hook de guarda contra materialização de credenciais reais por subagentes** (#141) —
  `trackfw-credential-guard.sh`, novo hook gerado nos 3 stacks, detecta padrão de JWT
  (`eyJ...`) e AWS access key (`AKIA...`) em comandos Bash e os conecta aos 6 CLIs da wave
  nativa (Claude Code, Codex, Gemini CLI, GitHub Copilot, Cursor, Kiro). Modo avisador por
  padrão (`credential_guard.mode: warn`, default), bloqueio opt-in via `trackfw.yaml`
  (`mode: block`, exit 2). Novo gate de paridade estrutural
  (`scripts/check-agent-hooks-parity.sh`) protegendo os `hooks.json`/`settings.json`
  gerados por CLI contra divergência entre Go/Node.js/Python.
- **Credential-guard em escopo global via `trackfw update harness`** (#143) — 6 alvos novos
  (`<tool>-credential-guard`), opt-in puro (não muda o comportamento de `trackfw init`/`update`),
  instala o hook em `~/.claude/settings.json`/`~/.codex/hooks.json`/`~/.gemini/settings.json`/
  `~/.cursor/hooks.json`/`~/.copilot/settings.json`/`~/.kiro/hooks/`, protegendo qualquer
  projeto do usuário, com ou sem `trackfw.yaml`. Dedup por leitura: o wiring por-projeto
  detecta instalação global já existente e evita duplicar a proteção no mesmo comando.
  Novo gate `scripts/check-harness-hooks-parity.sh` cobrindo os 6 arquivos de hook globais.

### Fixed

- **Divergência de versão no fallback do pacote Python e schema legado de hooks do Cursor**
  (#142) — `pypi/trackfw/__init__.py` estava com fallback desatualizado (`6.3.1`), bloqueando
  `make quality`/`make parity` de ponta a ponta; alinhado a `6.4.1`. Wiring legado de
  attention-signal/cleanup do Cursor migrado do schema inválido (nível raiz) para o schema real
  confirmado pela documentação oficial (`hooks.preToolUse`/`hooks.postToolUse`, aninhado), com
  migração automática para projetos que já tinham o trackfw instalado.

Breaking Changes: nenhuma.

## [6.4.1] - 2026-08-05

### Fixed

- **Template canônico do agente Architect ainda instruía `git checkout -b` cru** (#139) —
  `trackfw branch new <type>/<slug>` (v6.4.0) foi criado exatamente para mover o gate
  `branch_has_wip_roadmap` para antes da criação da branch, mas o parágrafo "Git authority" do
  template — deployado como `~/.claude/agents/trackfw-architect.md` via `trackfw update harness` —
  nunca mencionava o comando. Agora instrui `trackfw branch new` como forma preferencial, com
  fallback documentado para `git checkout -b` cru quando o comando não existir (binário anterior a
  v6.4.0) ou falhar por motivo diferente do bloqueio esperado por falta de roadmap.

### Internal

- Scaffold de governança do próprio repositório (slash commands `architect`/`barrier`, workflow de
  CI `trackfw-gate.yml`, scripts de attention hooks) atualizado para os artefatos gerados pela
  v6.4.0 (#138).

Breaking Changes: nenhuma.

## [6.4.0] - 2026-08-05

### Added

- **OpenCode (opencode.ai) como 10º target de integração** (#126, #134, #135) — `agents`/`skills
  install|uninstall|update`, `trackfw init --ai-tools opencode` e o harness de `update` passam a
  suportar OpenCode nos 3 CLIs, permitindo rotear agentes trackfw para modelos open-source/locais
  configurados pelo usuário (Ollama, LM Studio, etc.). O frontmatter do agente é reconstruído do
  zero (`description` + `mode: subagent` fixo, sem `model:`/`tools:`/`memory:`) porque o schema do
  OpenCode trata `tools:` como chave reservada — reutilizar o frontmatter original derruba o
  carregamento do projeto inteiro no OpenCode real (confirmado contra o binário 1.18.13). Skills
  não precisam de tratamento especial (schema já compatível). Documentado em `docs/cli-parity.md`.
- **`trackfw branch new <tipo>/<slug>`** (#125) — bloqueia a criação de uma branch de
  feature/fix/refactor antes de existir um REQ+roadmap correspondente em `wip/`, prevenindo
  "trabalho órfão" sem rastreabilidade de governança. Complementa a regra `branch_has_wip_roadmap`
  do `trackfw validate` com um gate preventivo no momento da criação da branch.

### Fixed

- **Dispatch de subagente sem `subagent_type` explícito** (#123) — o template do agente Architect
  nomeava especialistas em prosa (`squad:`) sem instruir o harness a passar `subagent_type`
  explicitamente, fazendo alguns harnesses (ex: Windsurf) invocarem `general-purpose` em vez do
  especialista nomeado. Corrigido com uma seção de "contrato de dispatch" agnóstica de preset de
  identidade.
- **`json.MarshalIndent` do Go escapava HTML, divergindo de Node.js/Python** (#128, #130) — 3
  targets do catálogo (Kiro, Amazon Q, Antigravity legacy) recebiam `<`, `>` e `&` como
  `<`/`>`/`&` só no Go, quebrando paridade byte-a-byte. Corrigido com
  `json.Encoder.SetEscapeHTML(false)`.
- **`discover --init` não gerava os scripts de attention hooks em Go/Node.js** (#121, #124) —
  lacuna de paridade pré-existente com o Python; e os 3 scripts (Go, Node.js, Python) divergiam em
  conteúdo entre si sem nenhum gate de paridade cobrindo isso (#122, #133) — unificados e agora
  cobertos por `check-attention-scripts-parity.sh`.
- **Job `parity` do CI só rodava 4 dos 15 scripts de `make parity`** (#129, #132) — a suíte
  inteira de 101 cenários de falsificação (prova de que os gates não são vazios) nunca rodava de
  forma automatizada; o job agora roda `make parity` diretamente.

Breaking Changes: nenhuma.

## [6.3.1] - 2026-08-04

### Fixed

- **`req list`/`req move` não enxergavam `REQDir` com subpastas, e `req move` não movia o arquivo**
  (#116) — os 3 CLIs descobriam REQs só num nível de `req_dir`, ignorando layouts por-estado
  (`req_dir/<estado>/`) e by_agent (`req_dir/<agente>/<estado>/`), mesmo com `trackfw context` já
  enxergando os mesmos arquivos. `req move` também nunca movia o arquivo fisicamente, só reescrevia
  `status:` no lugar, divergindo do padrão já usado por `roadmap move`. Agora os 3 CLIs descobrem
  REQs nos 3 layouts sem flag adicional, e `req move` move fisicamente o arquivo quando ele já está
  numa subpasta de estado reconhecida — permanecendo in-place, sem migração forçada, para REQs
  soltas em `req_dir/`. Fecha também uma lacuna de paridade pré-existente: o CLI Python não tinha
  `req list`.
- **`make quality` falhava sob locale `pt_BR.UTF-8`** (#117) — o gate de falsificação pinava
  byte-a-byte a mensagem de sucesso do `validate` contra um literal em inglês hardcoded, mas os 3
  CLIs imprimem essa mensagem via i18n, dependente do locale do processo. O gate agora fixa o
  locale nas comparações, tornando-o determinístico independente da máquina onde roda.
- **`req move` no CLI Node.js despejava stack trace em vez de mensagem de erro limpa** (#118) —
  erros de `req move` (REQ não encontrada, status inválido, etc.) subiam como rejeição de Promise
  não tratada. Agora produz `Error: <mensagem>` em stderr e código de saída não-zero, como Go e
  Python já faziam.

Breaking Changes: nenhuma. REQs soltas em `req_dir/` continuam com comportamento in-place idêntico
ao anterior — nenhum projeto existente é migrado automaticamente para o layout por-estado.

## [6.3.0] - 2026-08-03

### Fixed

- **5 scanners artesanais de `trackfw.yaml` eliminados** (#109) — `update` e `sync`, nos 3 CLIs,
  liam o arquivo linha a linha com uma gramática diferente da do carregador central (mesma classe
  de defeito eliminada em #106 para `validate`, viva em outro endereço). Chave aninhada homônima
  sequestrava o valor da raiz em silêncio; valor entre aspas, comentário à direita e escalar com
  dois-pontos interno quebravam a leitura. Os 3 CLIs passam a resolver os mesmos 11 campos
  (`hooks`, `ci`, `backend`, `frontend`, `pkg_manager`, `linear_api_key`, `linear_team_id`,
  `jira_base_url`, `jira_email`, `jira_token`, `jira_project`) pelo carregador único.
- **`trackfw update` do Python não lia `hooks`/`ci`/`backend`/`frontend`/`pkg_manager`** (#109) —
  Go e Node decidiam quais git hooks e qual workflow de CI gerar com base nesses campos; o Python
  não tinha o leitor. Fechado — mesmo efeito observável nos 3 CLIs, provado por teste que demonstra
  a mudança (não apenas testes existentes permanecendo verdes).

### Changed

- **Namespaces `Update` e `Sync` no contrato de config** (#109) — `ProjectConfig` ganha os dois
  namespaces tipados; chaves no `trackfw.yaml` permanecem planas na raiz, com os nomes atuais.
  Documentado em `docs/cli-parity.md` e `README.md`.
- **3 cenários novos de proteção de falsificação** (#109) — um por CLI em
  `scripts/check-gates-falsify.sh`, provados por reintrodução temporária do scanner eliminado:
  cada cenário falha se o scanner artesanal voltar.

Breaking Changes: nenhuma. Preservação mecânica de `linear_api_key`/`jira_token` (roteamento pelo
carregador, sem mudança de tratamento) e de todos os textos de erro de `sync`/`update`.

## [6.2.0] - 2026-08-02

### Added

- **Regra `adr_accepted_when_req_done`** (#103) — ADR não aceito referenciado por REQ `Done` passa
  a ser violação (`error`). Fecha a lacuna que deixou um ADR em `Proposed` governar sete REQs
  concluídas sem nenhum gate detectar. Introduz noção canônica de "ADR não aceito" cobrindo
  `Draft` **e** `Proposed`, e com isso corrige a `blocked_by_draft_adr`, que era cega a `Proposed`
  — ou seja, só funcionava para stubs gerados por `req new`, não para ADRs criados por `adr new`.
- **Comando `status` unificado nos 3 CLIs** (#105) — Go/Node exibiam uma visão acionável e o
  Python um inventário de contagens; agora os três produzem a **mesma** saída, somando as duas
  visões. Inclui bloco `📊 Inventory` com ADRs, REQs discriminadas por status real
  (`Open`/`Done`/`Closed`) e roadmaps pelos **seis** estados.

### Fixed

- **`analyzing` omitido na contagem do Python** (#105) — o comando `status` enumerava 5 dos 6
  estados, em três pontos do código. Roadmap em `analyzing/` sumia da contagem, em silêncio.
- **Backticks tornavam a referência invisível** (#104) — ``ADR: `docs/adr/X.md` `` produzia um token
  que não terminava em `.md`, e a referência não era encontrada. 13 REQs do repositório usam essa
  forma; três ficavam inalcançáveis por qualquer regra que dependesse do extrator.
- **Python ignorava a própria chave de i18n** (#104) — `validate.ok` existia nos três locales, mas
  o CLI Python imprimia `"✓ Governance OK"` hardcoded. Os três agora imprimem a mesma mensagem.
- **Delimitador não pareado e ordenação do fallback de agentes** (#105) — `ADR: "X.md'` resolvia em
  Go/Node e não no Python; e `_list_dirs` não ordenava, deixando a ordem dos agentes dependente da
  ordem de criação no filesystem.
- **Sequência YAML em bloco não indentada descartada por Go e Node** (#105) — `agents:\n- zeus` é
  YAML válido, mas os dois tratavam linha sem indentação como top-level e **descartavam a lista em
  silêncio**. O Python lia corretamente. Afetava `adr_dirs`, `agents`, `acceptance_markers` e
  `link_fields`.
- **Lista YAML inline descartada pelos três** (#105) — `agents: [zeus, apolo]` era ignorada sem
  aviso.
- **Config malformada era descartada em silêncio** (#106) — passa a falhar com mensagem clara e
  exit não-zero, idênticos nos três CLIs. Config ausente, vazia ou só com comentários continua
  caindo nos defaults, sem erro.
- **`validate` contornava o carregador de config** (#106) — lia `trackfw.yaml` com leitores
  artesanais próprios. Com `wip_limit: "3"`, o carregador lia 3 e o `validate` reportava 1.

### Changed

- **Parser de config passa a usar biblioteca YAML** (#106) — `gopkg.in/yaml.v3` (Go, promovida de
  indirect), `yaml` 2.x (Node) e **`PyYAML` (Python — primeira dependência de runtime do pacote,
  que era zero-dep)**. Substitui ~1085 linhas de parser artesanal. Qualquer YAML válido passa a
  ser aceito, incluindo mapas inline, listas aninhadas e âncoras.

  As três bibliotecas divergem em coerção de tipo — `yes` vira booleano só no Python, `010` vira
  `8` em Go/Python e `10` no Node, datas viram tipo data em Go/Python. Por isso todo escalar é
  **normalizado para string na fronteira do parser**, lendo o nó bruto: os consumidores existentes
  não mudam e os três CLIs concordam por construção.
- **Remoção do parâmetro morto `roots`** de `referenceExists` nos 3 CLIs (#104) — era recebido e
  nunca usado, enquanto três chamadores em cada CLI o passavam de boa-fé.

### Internal

- Proteção de falsificação em CI ampliada de **24 para 92 cenários** em
  `scripts/check-gates-falsify.sh`, cobrindo contratos gerador↔validador, paridade de saída entre
  CLIs e coerção de schema YAML.
- `scripts/check-validate-parity.sh` ganhou fixture violadora e guard de vacuidade por regra —
  antes passava sem discriminar nada, porque o repositório não tinha artefato que violasse.
- CI passa a instalar as dependências Python declaradas em `pypi/pyproject.toml` nos jobs `python`
  e `parity`, e o smoke de pacote deixou de usar `--no-deps`, o que também valida a declaração de
  dependências.

## [6.1.0] - 2026-08-01

### Added

- **Dashboard: abas ADRs e REQs** (#94) — ADRs e REQs deixam de ser alcançáveis apenas como nós
  do grafo da aba Chain e ganham listas navegáveis, com busca textual (case- e acento-insensitive)
  e filtro de status derivado dinamicamente dos valores presentes na resposta. Clicar numa linha
  reusa o drawer existente. Nenhum endpoint novo: as listas derivam de `/api/chain`.

### Fixed

- **Segurança — XSS armazenado no drawer** (#95) — `openDrawer` renderizava a saída de
  `marked.parse()` diretamente em `innerHTML`, sem sanitização. Uma ADR maliciosa vinda de um PR
  executava script quando o mantenedor abria o drawer para revisar. Introduz DOMPurify 3.4.12 com
  SRI, sanitizando num ponto único, e fail-safe que degrada para texto puro quando o sanitizador
  não carrega — nunca HTML bruto.
- **`roadmap new` gerava artefato que o próprio `validate` rejeitava** (#96) — o gerador emitia
  `**Acceptance criteria:**` (negrito) enquanto o validador exige o heading `## Acceptance
  Criteria`. Todo roadmap novo falhava na primeira transição para `wip`, nos 3 CLIs. Os geradores
  passam a emitir também o heading consolidado, preservando os blocos por microlote.
- **Falso-positivo `ref_targets_exist` em `roadmap new --from-req`** (#97) — o campo `req:` do
  frontmatter recebia apenas o basename, e o validador o resolve relativo ao cwd. Passa a gravar
  o caminho relativo completo, nos 3 CLIs. Corrige junto o falso-**negativo** do caminho simples,
  em que `roadmap new --req <path>` gravava `req: ""` vazio e nenhuma violação disparava.
- **Links `.md` relativos no drawer retornavam 403** (#98) — o interceptador passava o href bruto
  para `openDrawer`. Passa a resolver o href contra o diretório do documento aberto, cobrindo
  `./X.md`, `X.md` e `../` encadeados. Link que resolva para fora dos diretórios permitidos exibe
  o caminho resolvido em mensagem explicativa, em vez de `Forbidden` cru.
- **Cadeia de suprimentos do dashboard** (#99) — `marked`, `chart.js` e `d3` ganham `integrity`
  (SRI), `crossorigin` e `referrerpolicy`. O `htmx` é **removido** por não ter nenhum uso,
  eliminando o `unpkg.com` da cadeia. O Tailwind permanece sem SRI de forma deliberada — a URL é
  não-versionada e um hash fixo quebraria o dashboard no próximo release deles; a razão está
  documentada no próprio `index.html`.

### Changed

- **Remoção do parâmetro morto `roots`** de `referenceExists` / `_reference_exists` nos 3 CLIs
  (#97). O parâmetro era recebido e nunca usado, enquanto três chamadores em cada CLI o passavam
  de boa-fé. A validação permanece estrita: um `req:` com basename continua reprovando.

### Internal

- Proteção de falsificação em CI ampliada de **24 para 42 cenários** em
  `scripts/check-gates-falsify.sh`, cobrindo o contrato gerador↔validador do heading de critérios
  de aceite e do campo `req:` do frontmatter, nos 3 CLIs e nos dois caminhos de geração.

## [6.0.0] - 2026-07-30

### Por que esta versão é major

Duas mudanças na superfície de versão do CLI quebram consumidores:

1. **O CLI Go deixa de imprimir o prefixo `v`.** `trackfw v5.0.0` passa a
   `trackfw 6.0.0`, em `version` e em `--version`. O `v` é convenção de *tag
   Git*, não de string de versão — o SemVer especifica que `v1.2.3` não é uma
   versão semântica, e `npm/package.json` e `pypi/pyproject.toml` não podem
   carregá-lo. A **tag Git permanece `v<x.y.z>`**.
2. **`trackfw -v` deixa de funcionar no CLI Go.** O atalho era aceito apenas
   pelo Go, exposto por default do cobra e não por decisão de design. `-v` e
   `--verbose` passam a ser **reservados** para um futuro modo verboso, alinhado
   à convenção de `docker`, `kubectl`, `ansible`, `ssh` e `curl`.

**Migração:**

- Scripts que parseiem a saída de `trackfw version` ou `trackfw --version` devem
  esperar `trackfw <semver>` **sem** o prefixo `v`, nos três runtimes.
- Substitua `trackfw -v` por `trackfw --version` ou `trackfw version`, que
  funcionam nos três runtimes desde a `5.0.0`.

### Changed
- **Saída de versão unificada nos três CLIs.** `version` e `--version` passam a
  imprimir **a mesma linha**, `trackfw <semver>`, byte-idêntica entre as duas
  superfícies e entre os três runtimes. Antes, o Go emitia o prefixo `v` e o
  `--version` do Node.js imprimia o número puro, sem o nome do programa —
  comportamento default do `.version()` do commander.
- **`-v` reservado para verbose.** Nenhum runtime o vincula a `--version`; os
  três o rejeitam com código de saída não-zero. A reserva é **contratual**:
  nenhum runtime o aceita como no-op, porque uma flag aceita sem efeito é
  indistinguível de uma flag quebrada.

### Fixed
- **O gate de paridade deixa de assinar divergências.** `check-cli-parity.sh`
  usava uma regex específica para o Node.js, que codificava a divergência do
  `--version` como comportamento esperado, e `^trackfw .+` para os outros dois —
  frouxa o bastante para aceitar `trackfw v5.0.0` e `trackfw 5.0.0` igualmente.
  Era por isso que o prefixo `v` sobrevivia a todas as auditorias. Os três
  passam a usar a mesma asserção literal, mais comparação byte-a-byte das seis
  saídas.

### Internal
- Seção `## Version output` em `docs/cli-parity.md` pina o formato literal, a
  equivalência entre as duas superfícies, a fonte da string por runtime, a
  asserção do gate e a reserva do `-v`.
- Registrada a fronteira do que **não** é unificado: mensagem e exit code de
  flag desconhecida seguem divergindo (cobra 1, commander 1, argparse 2), por
  serem gerados pelos frameworks e valerem para toda flag. Unificá-los exigiria
  sobrescrever o tratamento de erro dos três globalmente.
- Contagem de cenários de falsificação sobe de 21 para **24**, incluindo dois
  seams que provam **braços independentes** da asserção de versão (formato e
  comparação de bytes) e um seam com **guarda de vivacidade**, que compila o
  binário corrompido e confirma que ele exibe o defeito — não apenas que o
  arquivo mudou.

## [5.0.0] - 2026-07-30

### Por que esta versão é major

Quatro mudanças observáveis quebram consumidores que parseiam saída do CLI:

1. **Campo `wave` do documento JSON do barrier passa de número para string.**
   `{"wave": 2}` vira `{"wave": "2"}`. Necessário para suportar rótulos com
   sufixo (`2-bis`), que não são inteiros.
2. **Mensagens de erro do barrier mudam de `wave number` para `wave label`.**
   O texto é pinado literalmente em `docs/cli-parity.md` e agora nomeia o token
   rejeitado em vez de despejar a linha inteira.
3. **`## Wave 0` passa a ser rejeitada.** A gramática exige parte inteira ≥ 1.
   Roadmaps que usassem `Wave 0` deixam de ser auditáveis pelo barrier.
4. **`trackfw roadmap move` no CLI Python deixa de imprimir
   `Roadmap movido para: <caminho>`** e passa a imprimir
   `✓ moved <basename> → <diretório>`, alinhado a Go e Node.js. Era divergência
   de paridade pré-existente: idioma, forma e conteúdo diferiam dos outros dois
   runtimes.

**Migração:** consumidores de `trackfw barrier --json` devem tratar `wave` como
string. Scripts que casem mensagens de erro do barrier ou a saída de
`roadmap move` no Python precisam atualizar os padrões. Roadmaps com `Wave 0`
devem renumerar a partir de 1.

### Added
- **Rótulo de wave com sufixo no barrier**, nos três CLIs. Gramática
  `<inteiro>[-<sufixo>]` com sufixo `[a-z0-9]+`: `2`, `2-bis`, `2-hotfix`.
  Resolve o caso real de wave corretiva acrescentada **depois** que uma wave já
  foi executada e commitada, sem renumerar as waves seguintes já citadas em
  mensagens de commit. Rótulos são identidades distintas — `--wave 2` nunca casa
  com `Wave 2-bis`. Ordenação pinada: `2` < `2-bis` < `2-hotfix` < `3`.
- **`trackfw roadmap move` sincroniza a referência `roadmap:` da REQ pareada**,
  nos três CLIs. Antes, mover um roadmap deixava toda REQ que apontava para ele
  com referência inválida, e `trackfw validate` reprovava com
  `ref_targets_exist` — o comando de governança produzia um estado que o próprio
  validador rejeita. Cinco cardinalidades pinadas: zero REQs (no-op silencioso),
  uma, várias (ordenadas por basename), aponta para outro roadmap (não tocada) e
  referência já correta (nenhuma escrita, idempotente byte-a-byte).
- Novo gate de paridade `scripts/check-roadmap-move-parity.sh` com 5 cenários
  cross-runtime, todos com vacuity-guard, e cenário de falsificação que corrompe
  a implementação (nunca a asserção) com guarda contra padrão de `sed` obsoleto.
- Cenários de paridade do rótulo de wave em `scripts/check-barrier.sh`:
  heading malformada nas **duas** posições (antes e depois da wave alvo),
  identidade `2-bis` vs `2`, `Wave 0` e argumento `--wave` inválido.

### Fixed
- **`trackfw init --ai-tools <tool>` abortava o scaffold de um projeto novo**
  quando o harness global do usuário continha um artefato trackfw desatualizado.
  O preflight de `install` retornava erro para artefato `outdated` + `owned` e,
  como o lote é atômico com rollback, descartava a operação inteira. Agora o
  artefato é **pulado** com aviso em stderr, os bytes preservados e o restante do
  lote aplicado, com exit 0. Artefato `modified` continua sendo erro sem
  `--force` — bytes do usuário nunca são pulados em silêncio.
- **Heading de wave malformada abortava apenas quando posicionada antes da wave
  solicitada** no Node.js e no Python. Uma heading inválida depois da wave alvo
  não era visitada, e o barrier retornava exit 1 `blocked` em vez de exit 2 —
  fazendo um roadmap malformado ser lido como "wave reprovada", o que a decisão
  12 do ADR do barrier proíbe explicitamente. A detecção passa a ser pré-passo
  completo nos três runtimes.
- **Ordenação de REQs sincronizadas divergia nos três runtimes**, cada um por um
  motivo diferente: Go concatenava globs por agente e por estado; Node.js usava
  `readdirSync` sem `sort`; Python ordenava por caminho completo em vez de
  basename. Pinada como lexicográfica por basename.

### Internal
- Contrato de escopo de `install` documentado em `docs/cli-parity.md`, com o
  registro explícito de que as decisões D1/D4 do ADR de escopo de instalação
  permanecem em vigor: `trackfw init --ai-tools` sem TTY instala em escopo
  **global**, por decisão deliberada.
- ADR do barrier emendado com as decisões **15** (wave identificada por rótulo,
  não por inteiro) e **16** (heading fora da gramática aborta o documento
  inteiro — é feature, não defeito: ignorá-la deixaria os MLs daquela wave sem
  auditoria).
- Contagem de gates de falsificação sobe de 19 para 21 cenários, e de 12 para 14
  gates provados não-vacuosos.

## [4.0.0] - 2026-07-29

### Por que esta versão é major

Os cinco aliases de integração deprecated foram **removidos** do CLI Go:
`trackfw copilot`, `trackfw cursor`, `trackfw gemini`, `trackfw windsurf` e
`trackfw amazonq`. O fluxo canônico passa a ser exclusivamente `trackfw agents`
e `trackfw skills`.

Contexto que reduz o impacto real da quebra:

- Os aliases existiam **apenas no CLI Go**. Node.js e Python nunca os
  registraram, então usuários desses runtimes não são afetados.
- As superfícies de instalação marcadas como `legacy` no catálogo **não** foram
  removidas. Elas não são aliases de CLI e continuam listáveis e atualizáveis
  explicitamente, preservando o caminho de migração.

**Migração:** substitua `trackfw <tool>` por
`trackfw agents install --targets <tool>` ou
`trackfw skills install --targets <tool>`.

### Added
- `trackfw barrier <roadmap> --wave <n> [--json]` nos três CLIs: núcleo
  determinístico de liberação de wave, agnóstico de stack. Verifica MLs
  concluídos, evidências dos critérios de aceite, gates declarados no roadmap e
  `trackfw validate`. Retorna `passed` ou `blocked`, com exit code 2 reservado
  para erro de uso — distinto de reprovação.
- Slash command `/trackfw:barrier` com o checklist operacional completo,
  explicitando que a barrier verde do CLI é necessária mas não suficiente: as
  inspeções especializadas e a auditoria de diff não são avaliadas pelo binário.
- `trackfw update harness`: atualização do harness global em escopo próprio, sem
  exigir projeto. Quatro estados (`updated`, `skipped`, `missing`, `failed`),
  `--dry-run`, `--json`, `--targets` e `--install-missing`.
- Quatro gates de paridade cross-runtime, todos com cenário de falsificação:
  `check-barrier.sh`, `check-slash-parity.sh`, `check-rules-parity.sh` e
  `check-update-parity.sh`.

### Changed
- **Autoridade Git concentrada no orquestrador.** Os 11 agentes especialistas
  passam a declarar que não executam operações Git e que atuam somente por
  handoff autocontido. Apenas `trackfw_architect` cria branch, audita diff,
  commita e faz push.
- **`trackfw update` deixa de mutar estado global.** Antes, rodá-lo em vinte
  projetos repetia a mesma escrita global vinte vezes.
- Superfície única `trackfw help [assunto|chave]` nos três CLIs, com resolução
  determinística e sugestão em caso de assunto desconhecido. As flags nativas
  `--help` seguem preservadas.

### Fixed
- Paridade real entre os três runtimes em saída JSON, mensagens de erro, ordem de
  chaves e conjuntos de targets — divergências que as suítes por runtime não
  detectavam porque cada uma passava isoladamente.
- Bloco `Architecture Directives` estava duplicado dentro do gerador Go.
- Mapa duplicado de slash commands no Node.js: `--force` instalava 6 dos 9.
- Em projeto novo, `GEMINI.md`, `.github/copilot-instructions.md`,
  `.windsurfrules` e `.amazonq/developer/guidelines.md` voltam a ser criados de
  forma idempotente.
- `check-update-parity.sh` mutava o `CLAUDE.md` do repositório e retornava exit 0
  ao fazê-lo; agora há cenário que compara `git status --porcelain` antes e
  depois de rodar os gates.

### Internal
- Cenários de falsificação: 13 → 19. Gates provados não-vacuosos: 8 → 12.

## [3.1.0] - 2026-07-27

### Added
- `trackfw ship` com fluxo governado de commit, push e abertura de PR/MR, agnóstico de forge.
- Harness convergente para os CLIs e integrações de agentes/skills.

### Fixed
- Robustez dos gates de governança e paridade entre Go, Node.js e Python.
- Integridade referencial e ciclo de vida das REQs, incluindo estado `analyzing`.
- Convergência de templates, flags Python, parsing de valores YAML e contrato de schemas.
- `stale_wip` determinístico e configurável, diagnóstico explícito de erros de I/O e identity parity
  derivado do catálogo canônico.

### Changed
- Estrutura e frontmatter dos roadmaps canonicalizados, com documentação e artefatos sincronizados.

Nenhuma mudança breaking após a versão 3.0.0.

## [3.0.0] - 2026-07-25

### Por que esta versão é major

Até a `2.16.0`, `agents` e `skills` eram instalados **silenciosamente no
projeto atual**: `--scope` tinha default fixo `project` e nenhum dos três CLIs
perguntava onde instalar. O único prompt existente cobria apenas quais CLIs e
quais itens, e sequer disparava quando `--targets` era informado — a invocação
mais comum. Corrigir isso exigiu inverter o default, e inverter um default
muda o comportamento observável de comandos que já existiam.

São três quebras de contrato distintas. Nenhuma delas emite aviso: o comando
continua "funcionando", só que fazendo outra coisa.

1. **Destino de gravação** — `install`/`update` sem `--scope` em modo
   não-interativo passam a gravar em `~/.claude/...` em vez de `.claude/...`.
   Pipelines que instalam e depois verificam ou commitam artefatos no
   repositório param de encontrá-los.
2. **`uninstall` passa a falhar** — sem `--scope` em modo não-interativo, o
   comando retorna erro em vez de remover. É deliberado: com o novo default,
   um `uninstall` de CI apagaria os artefatos do diretório home do usuário.
   Preferimos falhar a destruir.
3. **Contrato de saída do `list`** — `list --json` sem `--scope` passa a
   reportar `"scope": "global"` e destinos `~/...`. Automações que consomem
   esse JSON para inspecionar estado leem valores diferentes para a mesma
   pergunta.

O `package-smoke` deste próprio repositório quebrou pelo item 1 durante o
desenvolvimento — foi o primeiro consumidor a sentir a mudança, e é um que
controlamos. Assumimos que existem outros que não controlamos, e é por isso
que esta é uma major e não uma minor: a atualização precisa ser deliberada.

### Migração

Pipelines de CI e scripts não-interativos devem passar `--scope`
explicitamente:

```diff
- trackfw agents install --targets claude
+ trackfw agents install --targets claude --scope project
```

Use `--scope project` para manter o comportamento anterior (artefatos no
repositório) ou `--scope global` para adotar o novo padrão. Uso interativo em
terminal não requer mudança: o CLI pergunta, com `global` pré-selecionado.

### Changed
- **BREAKING**: `agents|skills install|update` sem `--scope` em modo
  não-interativo instalam em escopo `global` (`~/.claude/...`) em vez de
  `project` (`.claude/...`).
- **BREAKING**: `agents|skills uninstall` sem `--scope` em modo não-interativo
  agora falha exigindo a flag, em vez de assumir um escopo.
- **BREAKING**: `agents|skills list` sem `--scope` reporta escopo `global` e
  os destinos correspondentes.

### Added
- Prompt interativo de escopo em `agents`, `skills` e `init` — pergunta onde
  instalar (`~/.claude` vs `.claude`), com `global` pré-selecionado, sempre
  que stdin for um TTY e `--scope` não tiver sido informado.
- Os caminhos de destino resolvidos são impressos antes da gravação, em todo
  comando mutante de `agents`/`skills` e na etapa de AI tools do `init`.

### Fixed
- `scripts/smoke-integration-packages.sh` passa `--scope project` explícito —
  primeiro consumidor a exigir a migração descrita acima.

## [2.16.0] - 2026-07-25
### Added
- Identidade personalizável de agentes nos 3 CLIs — 10 presets temáticos
  (`greek`, `norse`, `potter`, `thrones`, `chaves`, `pioneers`, `starwars`,
  `tolkien`, `turma`, `egyptian`) + modo `custom` + apelido do usuário.
  `@agent-<slug>-tf` funcional; roteamento por linguagem natural via
  `description` ([#64](https://github.com/kgsaran/trackfw/pull/64))
- `trackfw agents install` também oferece o wizard guiado de identidade
  (antes só existia em `init`), com rótulos por especialidade do catálogo e
  tela de confirmação antes de gravar
  ([#65](https://github.com/kgsaran/trackfw/pull/65))

## [2.15.1] - 2026-07-24
### Fixed
- Resolve() cross-platform no Windows (paridade Node+Go+Python) ([#62](https://github.com/kgsaran/trackfw/pull/62))

## [2.15.0] - 2026-07-20
### Added
- Slash command /trackfw:architect e diretrizes obrigatórias de arquitetura ([#58](https://github.com/kgsaran/trackfw/pull/58))
- Sinalização de atenção automática via hooks nativos dos 7 CLIs ([#57](https://github.com/kgsaran/trackfw/pull/57))
- Suporte a ADRs globais compartilhados e diretivas de IA ([#56](https://github.com/kgsaran/trackfw/pull/56))
### Fixed
- Hardening de qualidade Q1-Q8 pós-PR59 ([#60](https://github.com/kgsaran/trackfw/pull/60))
- Correções e hardening pós-auditoria dos PRs #56 e #57 ([#59](https://github.com/kgsaran/trackfw/pull/59))

## [2.14.0] - 2026-07-19
### Added
- Render Antigravity valido para o agy (tools + model tier) ([#52](https://github.com/kgsaran/trackfw/pull/52))

## [2.13.0] - 2026-07-19
### Added
- Add agents and skills lifecycle parity ([#50](https://github.com/kgsaran/trackfw/pull/50))
### Fixed
- Make npm publish step idempotent ([#48](https://github.com/kgsaran/trackfw/pull/48))

## [2.12.4] - 2026-06-24
### Fixed
- Prefer real git branch over ci env
- Ignore github ref names in temp fixtures
- Ignore GitHub branch env outside git worktrees
- Allow npm same-version publish step

## [2.12.3] - 2026-06-24
### Fixed
- Make npm publish step idempotent ([#48](https://github.com/kgsaran/trackfw/pull/48))

## [2.12.2] - 2026-06-24
### Added
- Native agent integration and v2.12.2 release prep ([#47](https://github.com/kgsaran/trackfw/pull/47))

## [2.12.1] - 2026-06-20
### Changed
- Internal maintenance release (no user-facing changes).

## [2.12.0] - 2026-06-20
### Added
- Attention hooks auto-injetados para 6 CLIs de agentes IA ([#45](https://github.com/kgsaran/trackfw/pull/45))
- Gate pré-trabalho branch_has_wip_roadmap + fallback Node.js→husky ([#44](https://github.com/kgsaran/trackfw/pull/44))

## [2.11.0] - 2026-06-19
### Changed
- Comprime SKILL.md, rules block e architect.md (~450 tokens/sessão) ([#43](https://github.com/kgsaran/trackfw/pull/43))

## [2.10.0] - 2026-06-19
### Added
- Slash command /trackfw:architect + guia de arquitetura (3 CLIs) ([#42](https://github.com/kgsaran/trackfw/pull/42))
- Estado 'Analyzing' no kanban + regras de ciclo de vida de ML ([#41](https://github.com/kgsaran/trackfw/pull/41))

## [2.9.1] - 2026-06-18
### Fixed
- Exibe próximo ML pendente no card kanban quando nenhum ML está ativo ([#40](https://github.com/kgsaran/trackfw/pull/40))

## [2.9.0] - 2026-06-18
### Added
- Kanban progress + agent rules inject + trackfw update (v2.9.0) ([#39](https://github.com/kgsaran/trackfw/pull/39))

## [2.8.0] - 2026-06-15
### Added
- --init instala hook framework automaticamente quando nenhum é detectado ([#38](https://github.com/kgsaran/trackfw/pull/38))

## [2.7.1] - 2026-06-14
### Fixed
- Corrige ordem das colunas kanban e erro 'node not found' no chain view

## [2.7.0] - 2026-06-14
### Added
- V2.7.0 — dashboard web trackfw serve (Go + Node.js + Python) ([#37](https://github.com/kgsaran/trackfw/pull/37))

## [2.6.0] - 2026-06-14
### Added
- Req_has_adr / req_has_roadmap / blocked_has_req configuráveis via applyRule ([#36](https://github.com/kgsaran/trackfw/pull/36))

## [2.5.4] - 2026-06-13
### Fixed
- FindRoadmap autodescobre agentes by_agent em vez de fallback default
- Context + validateADRsAreReferenced REQ by_agent
- Context REQ by_agent
- Context REQ by_agent

## [2.5.3] - 2026-06-13
### Fixed
- REQ indexing by_agent — resolve_req_files + _index_reqs_by_agent + salvaguarda one-sided
- REQ indexing by_agent — resolveReqFiles + salvaguarda one-sided
- REQ indexing by_agent — resolveREQFiles + traceid + salvaguarda one-sided

## [2.5.2] - 2026-06-13
### Fixed
- Suporte a roadmap_namespacing: by_agent + salvaguarda zero-entradas — ML-1A
- Suporte a roadmap_namespacing: by_agent + salvaguarda zero-entradas — ML-1C Python
- Salvaguarda zero-entradas + teste by_agent — ML-1B Node.js

## [2.5.1] - 2026-06-13
### Fixed
- Rule/file preenchidos no --json + help traceid — ML-1B Node.js
- Rule/file preenchidos no --json + help traceid — ML-1B
- Rule/file preenchidos no --json + help traceid — ML-1C

## [2.5.0] - 2026-06-13
### Added
- Trackfw discover + --init + --bootstrap-log — ML-4C
- Trackfw discover + --init + --bootstrap-log — ML-4A
- Trackfw discover + --init + --bootstrap-log — ML-4B
- Req_id bidirecional com 5 violations — ML-5B
- Namespacing by_agent — ML-3A
- Req_id bidirecional com 5 violations — ML-5A
- Req_id bidirecional com 5 violations — ML-5C
- Namespacing by_agent — ML-3C
- Namespacing by_agent — ML-3B
- Paths configuráveis adr_dirs/req_dir/roadmap_dir — ML-2A
- Paths configuráveis adr_dirs/req_dir/roadmap_dir — ML-2B
- Paths configuráveis adr_dirs/req_dir/roadmap_dir — ML-2C
### Fixed
- Flag --json output estruturado — ML-1C
- Flag --json output estruturado — ML-1B

## [2.4.1] - 2026-06-13
### Fixed
- Trim de aspas em valores YAML — ML-2C
- Trim de aspas em valores YAML — ML-2A
- Trim de aspas em valores YAML — ML-2B
- Ratchet aplica set-difference em warnings — ML-1C
- Ratchet aplica set-difference em warnings — ML-1A

## [2.4.0] - 2026-06-13
### Added
- Trackfw help e configure — ML-4C
- Trackfw help e configure — ML-4B
- Trackfw help e configure — ML-4A
- Trackfw baseline + ratchet em validate — ML-3C
- Trackfw baseline + ratchet em validate — ML-3B
- Trackfw baseline + ratchet em validate — ML-3A
- Field mapping + severity per rule — ML-2C
- Field mapping + severity per rule — ML-2B
- Field mapping + severity per rule — ML-2A
- Novos campos link_fields, acceptance_markers, rules com parser aninhado — ML-1C
- Novos campos link_fields, acceptance_markers, rules com parser aninhado — ML-1A
- Novos campos linkFields, acceptanceMarkers, rules com parser aninhado — ML-1B

## [2.3.0] - 2026-06-13
### Added
- Commands metrics/context/sync/plugins (ML-3D)
- Commands roadmap (new/move/list/show) + discover --init (ML-3C)
- Commands validate + status com breakdown by_agent (ML-3B)
- Cli.py entry point + comandos adr/req/log (ML-3A)
- Generators/req.py — geração de REQ com frontmatter (ML-2B)
- Generators/init_gen.py — scaffold flat/by_agent (ML-2D)
- Generators/roadmap.py — new + move flat/by_agent (ML-2C)
- Generators/adr.py — geração de ADR sequencial (ML-2A)
- Validator.py com wip-limit, stale-wip, req-adr (ML-1C)
- I18n com suporte pt-BR/en-US/es-ES (ML-1B)
- Config.py singleton + __main__ entry point (ML-1A)
### Fixed
- Adr_dirs recursivo, stale git log, existência de refs, pasta×status, unicidade — ML-1C Python
- Adr_dirs recursivo, stale git log, existência de refs, pasta×status, unicidade — ML-1B Node.js
- Adr_dirs recursivo, stale git log, existência de refs, pasta×status, unicidade — ML-1A Go
- Corrige workflow PyPI — remove _cli.py, atualiza __init__.py na tag

## [2.1.1] - 2026-06-13
### Added
- Site VitePress bilíngue pt-BR/en-US + GitHub Actions deploy
### Fixed
- Use trackfw.yaml config paths instead of hardcoded defaults

## [2.1.0] - 2026-06-13
### Added
- Trackfw roadmap new --from-req para geração assistida de MLs
- Trackfw context --format=md|json — Go + npm
- Frontmatter YAML em ADR/REQ/ROADMAP — Go + npm
- JSON Schema para ADR/REQ/ROADMAP + validateFrontmatterPresence — Go + npm
- Commit-msg hook com validação de REQ em feat/fix branches
- Integração PM via trackfw sync --to=linear/jira
- Registry search e resolução de nomes via kgsaran/trackfw-plugins
- WIP limit configurável por squad via trackfw.yaml
- Modo lenient de governança via --brownfield
- Cycle time, throughput e WIP age a partir do .trackfw-log
- Servidor HTTP local de visualização ADR→REQ→ROADMAP
- ADR-001 + REQ + roadmap — trackfw como trilho de governança para agentes de IA

## [2.0.0] - 2026-06-13
### Added
- Add --title/--req flags to roadmap new and non-TTY fallback to init
- Detecta HookFramework+CISystem e --init instala gates (ML-4A+4B)
- Comando trackfw discover com scan de repositório e --init / --bootstrap-log
- Suporte a roadmap_namespacing by_agent em generators e validator
- Trackfw init gera campos de paths no trackfw.yaml
- Pacote central de configuração com paths configuráveis
### Fixed
- Agent detection + REQ count recursivo corrige e2e no CMDB
### Changed
- Substituir paths hardcoded por config.Load() em todos os pacotes

## [1.1.0] - 2026-06-12
### Added
- Suporte multilingual automático pt-BR / en-US / es-ES
- Framework de backend por linguagem + scaffold pom.xml Java

## [1.0.4] - 2026-06-12
### Added
- Reescreve pacote npm como Node.js puro

## [1.0.3] - 2026-06-12
### Added
- Fat package — binários embutidos, sem postinstall
### Fixed
- Suporte a TRACKFW_BINARY_URL para mirrors corporativos
- Usa tar.gz no Windows — elimina dependência do PowerShell Expand-Archive

## [1.0.2] - 2026-06-12
### Fixed
- Busca binário recursivamente após extração + erros explícitos no Windows

## [1.0.1] - 2026-06-12
### Fixed
- Remove campos manuais linked ADR/roadmap — vínculos via probe discovery
- Substituir inputs manuais de ADR/roadmap por selects com arquivos existentes

## [1.0.0] - 2026-06-12
### Added
- Sistema de plugins com list/add/remove e dispatch automático
- Registra transições de estado e exibe histórico com trackfw log
- Adiciona subcomando show com busca parcial por nome
- Detecta roadmaps em WIP por mais de 7 dias (stale)
- Propaga README raiz para pacotes npm e PyPI
- ML-3B — seção de REQs bloqueadas por ADRs Draft
- ML-3A — verificar REQs bloqueadas por ADRs Draft
- ML-2B — wizard req new com etapa de probes contextuais
- ML-2A — REQContent com DependsOnADRs e seção Blocked by ADRs
- ML-1B — NewADRDraft para geração de ADRs Draft via wizard
- ML-1A — catálogo de probes e detecção de domínio

## [0.2.0] - 2026-06-11
### Added
- Templates Wave 1 — 55 arquivos para 5 ferramentas de IA
- Generators, CLI commands and init wizard for 5 AI tools

## [0.1.3] - 2026-06-11
### Added
- Trackfw agents command com 10 agentes especializados
- Instala SKILL.md global em ~/.claude/skills/trackfw/
- Adiciona comando 'trackfw skills' para instalar slash commands
### Changed
- Remove todos os nomes mitológicos do corpo dos agentes
- Renomeia agentes para nomes funcionais

## [0.1.2] - 2026-06-11
### Added
- Gera .claude/commands/trackfw/ com 7 slash commands no trackfw init
- Adiciona publicação automática no npm e PyPI ao release
### Fixed
- Slash commands idempotentes — não sobrescreve arquivos existentes

## [0.1.1] - 2026-06-11
### Added
- Perguntas iterativas no adr new e req new

## [0.1.0] - 2026-06-11
### Added
- Adiciona subcomando 'roadmap list' com agrupamento por estado
- Skill /trackfw:implement + CLAUDE.md com regras de conduta para agentes
- /trackfw:roadmap gera roadmap via IA nativa do Claude Code
- Geração por IA via huh.Select + Anthropic/OpenAI + fallback template
- Wizard interativo nas seções + req list
- Wizard interativo nas seções + adr list
- Wizard condicional por tipo de projeto + geração de CLAUDE.md
- Homebrew tap + 14 testes unitários Go
- Adiciona regras de acceptance criteria e wip único
- Expõe comandos trackfw como slash commands no Claude Code e Gemini CLI
- Adiciona wrapper PyPI para distribuição via pip install
- Adiciona wrapper npm para distribuição via npm install
- Adiciona pipeline GoReleaser + GitHub Actions
- Scaffold trackfw CLI — governed delivery framework
### Fixed
- Inferir nome do projeto do diretório atual
- Corrige 4 bugs no CLI trackfw
- Rastreia npm/bin/ no git e corrige .gitignore
### Changed
- Remover integração AI do binário — delegada ao slash command /trackfw:roadmap
- Renomeia comandos para namespace trackfw:
- Atualiza module path para github.com/kgsaran/trackfw
