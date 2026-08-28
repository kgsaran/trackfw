# Changelog

Todas as mudanças notáveis deste projeto são documentadas neste arquivo.

O formato segue [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
e este projeto adere a [Semantic Versioning](https://semver.org/).

> Entradas anteriores a esta versão foram reconstruídas a partir do
> histórico de commits (convenção `feat`/`fix`/`refactor`) para fins de
> backfill. A partir de `2.16.0`, este arquivo é atualizado como parte
> obrigatória do protocolo de release (ver `CLAUDE.md`).

## [7.3.0] - 2026-08-28

Um comando novo de auditoria, a revisão de segurança movida para **antes** da
implementação, e cinco correções em mecanismos que davam sinal verde enquanto o
controle estava inerte. **Sem breaking changes** — atualização direta.

### Added

- **`trackfw audit-surface <ref>`** — responde *"o que neste PR roda na minha
  máquina?"* **sem checkout**, lendo hook wiring e arquivos de instrução direto do
  object database. Um checkout de PR hostil executa hook na máquina do mantenedor
  **sem exigir comando nenhum do trackfw** — basta abrir o repositório e usar a
  ferramenta. O comando fecha a janela entre o checkout e o primeiro uso.

  A unidade reportada é a tupla **(trigger, matcher, caminho, digest)**, porque as
  três variantes de ataque produzem diff de wiring **limpo**: só o script muda
  (diff do `settings.json` é zero), o wiring reaponta para outro script existente,
  ou o matcher alarga de `"Bash"` para `"*"`. Varre os **8 runtimes** de escopo de
  projeto por padrão de path — ausência é informação, não exclusão. Arquivos de
  instrução (`CLAUDE.md`, `AGENTS.md`, slash commands) têm rótulo próprio: não
  executam, **instruem**.

- **Wave 0 de modelo de ameaça no harness** — o gerador de roadmap passa a emitir
  uma wave de red team **antes** da implementação, com quatro seções verificáveis:
  completude de enumeração, modelo de ameaça, alvos de falsificação nas duas
  direções e residual declarado. O gate da wave é **fail-closed**: uma Wave 0
  gerada e não preenchida **reprova** no `trackfw barrier`.

  `trackfw barrier` passa a aceitar `--wave 0`, e o asset do arquiteto exige a wave
  antes de despachar implementação.

- **`doctor` cobre os artefatos de scaffold** — 9 slash commands, scripts de
  attention, `trackfw-validate.sh` e workflows de CI passam a ser comparados com o
  template, com propriedade dada pelo **caminho**. Antes, um projeto podia ficar
  com o slash command defasado e **nada acusava**: só o `update` revelava, e ele
  corrige no mesmo passo, então o usuário nunca sabia.

- **Estado `scaffold-wrong-mode`** — artefato com conteúdo correto e **bit de
  execução ausente** passa a ser reportado, verificado por `mode & 0o100`.

### Fixed

- **`barrier` executava o gate de roadmap não confiável.** Um roadmap chegado por
  PR de terceiro fazia o mantenedor executar shell que ele nunca aceitou — e
  *"bloqueado" não significava "não executou"*: os gates rodavam **antes** de o
  veredito ser composto. Agora o conteúdo é comparado com `origin/main` e o gate de
  roadmap não confiável sai como **`not_evaluated`**, com o consentimento do fluxo
  normal vindo do slash command, não de flag digitada.

- **`roadmap new` aceitava newline no título**, permitindo forjar uma seção
  Markdown inteira com bloco de gate próprio. Rejeitado nos dois caminhos (`new` e
  `--from-req`), antes de escrever qualquer arquivo.

- **O pin de modelo dos agentes dependia do diretório de invocação.** `agent_models`
  de escopo global passa a ser resolvido **exclusivamente** de
  `~/.trackfw/trackfw.yaml`, em **19 call sites** dos 3 CLIs. Antes, rodar
  `agents update` de outro diretório revertia o pin **em silêncio** — e dois
  caminhos escritos (`.windsurf/hooks.json`, `.amazonq/cli-agents/…`) nem sequer
  eram declarados na saída do comando.

  Config global malformada **não é fatal**: reusar a política do carregador de
  projeto faria um arquivo global quebrado derrubar todo comando do trackfw, em
  todo diretório.

- **`update --dry-run` abortava em symlink pendurado.** O sandbox copiava a árvore
  inteira do projeto — um `.venv` com interpretador removido derrubava a operação.
  Agora copia **apenas os destinos declarados**; o que está fora do conjunto deixa
  de existir como problema.

- **`update` não restaurava o bit de execução.** `os.WriteFile` e
  `fs.writeFileSync` aplicam `perm` **apenas** na criação do arquivo; em arquivo
  existente o conteúdo é reescrito e o modo não é tocado. O `doctor` acusaria e o
  remédio não remediaria — em loop.

### Internal

- Suíte de falsificação vai a **181 cenários** e **23 gates**. Novos:
  `check-push-force-parity.sh`, `check-audit-surface.sh`, e cobertura de
  `not_evaluated`, resolução por escopo, sandbox por inclusão e bit de execução.
- `check-artifact-parity.sh` passa a comparar contra **conteúdo esperado**, não só
  entre os 3 runtimes: uma regressão **sincronizada** passava em silêncio.

### Nota de atualização

Rode **`trackfw update`** e **`trackfw agents update --force`** depois de atualizar.
Se você usa `agent_models`, mova a chave do `trackfw.yaml` do projeto para
**`~/.trackfw/trackfw.yaml`** — o comando avisa qual é o caso quando não encontra a
configuração no lugar certo.

## [7.2.0] - 2026-08-22

Um comando novo, contratos de paridade transformados em gates executáveis e três
correções de segurança em regras que decidem se um controle está **ativo**.
**Sem breaking changes** — atualização direta.

### Added

- **`trackfw push`** — empurra commits já criados, sem commitar e sem abrir PR.
  Fecha o beco sem saída em que `trackfw commit` deixava o usuário: com o commit
  feito, `git push` é bloqueado pelo guard e `trackfw ship` recusa com
  `nothing is staged`. O vocabulário de entrega passa a ser composicional:

  ```
  trackfw commit -m "..."   commita
  trackfw push              empurra
  trackfw ship -m "..."     commit + push + PR (composição)
  ```

  `push` **reusa** os gates do `ship` em vez de reimplementá-los — bloqueio em
  `main`/`master`, padrão de nome de branch, governança (`wip/` ou `done/`, com a
  isenção `chore`/`docs`), aviso de squash pendente e o gate de
  `--force-with-lease`, que só executa com PR/MR aberto verificado via CLI de
  forge. Flags: `--dry-run` e `--force-with-lease`. **Nunca** aceita `-m`.

- **`trackfw agents models`** e versão de modelo por tier no `trackfw.yaml`, com
  composição por alvo no render de agentes para Codex (TOML) e Cursor
  (frontmatter).

- **Gates cross-CLI para os três contratos de maior risco** e mecanismo que
  transforma contrato pinado em `cli-parity.md` em **gate nomeado**, com checker
  de cobertura bloqueante — um contrato afirmado sem gate deixa de ser aceito.

- **Caminho governado para push forçado e tag de release**: `trackfw ship
  --force-with-lease` (exige PR aberto) e `trackfw release tag`, que publica a tag
  anotada via API do forge preservando a anotação. O guard passa a bloquear a
  classe destrutiva de comandos de working tree.

- **Regra de verbosidade do arquiteto** no asset do agente e no `CLAUDE.md`
  semeado.

### Fixed

- **`validate` era cego ao hook de guard na forma relativa antiga.** Um
  `.claude/settings.json` apontando `scripts/trackfw-credential-guard.sh` resolve
  a partir da raiz e **falha em silêncio** fora dela — o guard não executava, com
  status não-bloqueante, e o único sinal era ruído no terminal. A regra modelava
  resolvibilidade como propriedade do **caminho**, quando é propriedade do par
  **(caminho, cwd)**. Cursor, Copilot e Kiro, para os quais o caminho relativo é a
  forma correta, continuam limpos.

- **`validate` era cego a `$PWD`, que falha do mesmo jeito.** Corrigido pela
  classificação por **semântica de ancoragem** — não por casamento com o que o
  gerador emite. Três classes: ancorado (silêncio; inclui caminho absoluto e `~/`
  não aspeado), dependente do cwd (acusa; inclui `$PWD/`, `${PWD}/`, `"$PWD/"`,
  `./`, `../`) e indecidível (silêncio declarado). A mensagem explica **por que** a
  forma não ancora, em vez de dizer apenas que é inválida.

- **`release tag` confiava em conteúdo local** para versão e mensagem da tag; agora
  ancora ambas no commit do forge, com `--no-replace-objects` para fechar o desvio
  por `refs/replace/`.

- **A mensagem do guard para `git push` bruto** passa a ensinar `trackfw push`
  como caminho primário. A mensagem do `git reset --hard` continua indicando
  `trackfw ship -m`, que é o comando correto ali — depois de `reset --soft` o
  trabalho está *staged*, não commitado.

- **Panic com `agent-models` configurado** — `nil map` na construção de
  `ProjectConfig`.

### Internal

- Higiene de estado dos artefatos de governança e abertura do contrato pinado.
- Suíte de falsificação vai a **165 cenários** e **23 gates**; gates novos:
  `check-push-parity.sh` e `check-push-force-parity.sh`.

### Nota de atualização

Rode **`trackfw update harness`** depois de atualizar: a mensagem do guard que
ensina `trackfw push` só chega ao seu ambiente por ela. Até lá o guard continua
bloqueando normalmente — apenas indicando o comando antigo.

## [7.1.0] - 2026-08-19

Dois comandos novos e uma série de correções de segurança e de higiene acumuladas
desde a `7.0.0`. **Sem breaking changes** — atualização direta.

### Added

- **`trackfw doctor`** — diagnostica divergências entre o disco e o manifesto de
  integrações, em escopo de projeto e global, e distingue **três** classes com
  remédios diferentes, que nunca são fundidas:
  - `unregistered-write` — os bytes são do trackfw e batem com o template do
    catálogo; só falta o registro no manifesto. Adotar é seguro.
  - `hand-modified` — o manifesto é dono do destino, mas o arquivo foi editado
    depois. Adotar **perde a edição**, e a saída avisa disso.
  - `unknown-content` — o conteúdo não bate com o template nem tem entrada no
    manifesto. É o estado que faz o `install` recusar com `unmanaged artifact`,
    e o remédio **nomeia essa recusa** e declara as duas causas possíveis
    (arquivo de terceiro, ou artefato do trackfw que derivou) em vez de acusar
    adulteração.

  O comando **nunca escreve nada** — só imprime o comando de correção.
- **`trackfw branch prune`** — remove branches locais já integradas, com
  **dry-run por padrão**. Detecta corretamente **squash-merge**, que não deixa
  ancestralidade e por isso engana o `git branch -d`. Nunca apaga a branch
  atual, a branch padrão, nem branch presa em worktree.

### Fixed

- 🔒 **`trackfw serve` amarra em loopback por padrão.** Antes escutava em todas
  as interfaces, expondo a cadeia de governança na rede local sem autenticação.
  A exposição agora exige opt-in explícito, com aviso.
- 🔒 **Guard global de branch cabeado e verificado.** O script era escrito em
  `~/.trackfw/scripts/` e **nada jamais o invocava** — e a regra de integridade
  só avaliava configs que o referenciassem, então nunca rodava para ele.
  Consequência real: o script global ficou 3 versões atrasado com `validate`
  verde o tempo todo. Agora é cabeado nos mesmos CLIs do credential-guard, com
  **no-op fora de projeto trackfw**, e a verificação de integridade dispara por
  **existência do artefato**, não por fiação.
- 🔒 **Handler global de erro no CLI Node.** Erro não tratado vazava stack
  trace, caminhos absolutos de instalação e versão do runtime.
- **Ordem de persistência invertida: manifesto antes dos artefatos.** Uma
  interrupção no meio da gravação deixava o disco à frente do manifesto, um
  estado que exigia decisão humana (`unmanaged artifact`). A direção invertida é
  auto-reparável por um `install`/`update` seguinte.
- **Sete débitos acumulados** da entrega de plugins e da release `7.0.0`,
  incluindo brechas de contorno do guard de branch (`git switch -c`, prefixos
  `env`/`command`, `git worktree add -b`, flags fora da primeira posição).
- **`trackfw ship` e `trackfw branch new` aceitam branches `chore` e `docs`**
  sem exigir roadmap correspondente.

### Internal

- Novo gate `check-doctor-parity.sh` comparando as **três saídas reais** do
  `doctor` (texto e `--json`), não apenas testes por stack. Ele encontrou dois
  defeitos reais de paridade que nenhum teste por runtime pegaria.
- Suíte de falsificação cresceu para **133 cenários**: todo gate tem braço de
  baseline e braço de detecção, com prova de não-vacuidade.

## [7.0.0] - 2026-08-16

> ⚠️ **Versão com Breaking Change.** O subsistema de plugins foi **removido**. Leia a seção
> **Removed** antes de atualizar.

### Removed

- 🔴 **BREAKING — subsistema de plugins removido por completo: download, gestão e execução.**
  Saem `trackfw plugins add`, `plugins search`, `plugins list` e `plugins remove`, o pacote de
  download, o registry externo e a execução de binários de terceiro pelo trackfw.

  **Motivo: o trackfw era, ele próprio, uma superfície de cadeia de suprimento (supply chain).**
  Não houve incidente — o que havia era o caminho aberto, e ele era completo:

  1. o `plugins add` baixava um **binário** de terceiro do GitHub Releases;
  2. **sem verificação de assinatura**, **sem checksum publicado pelo autor** e **sem pinagem de
     release** — nada provava que aquele artefato era o que o autor publicou;
  3. a partir de um **registry apontando para a branch `main` de um repositório externo**, ou seja,
     um alvo **mutável** que podia mudar de conteúdo entre uma execução e outra;
  4. gravava o binário e o tornava **executável (`chmod 0755`)**;
  5. e, por fim, **qualquer argumento desconhecido** passado ao `trackfw` executava um binário
     `trackfw-<argumento>` encontrado no `PATH` — inclusive por **erro de digitação**.

  Comprometer o repositório do plugin, ou o registry, significava executar código arbitrário na
  máquina de quem usasse o trackfw — **sob a marca do trackfw**.

  **Por que remover em vez de proteger.** Um gate de duas fases (quarentena → revisão de segurança →
  instalação) chegou a ser projetado e avaliado. A análise mostrou que ele entregaria bem menos do
  que aparentava:

  - **seria gate de revisão, não de supply chain.** Sem verificação de origem, o checksum prova que
    o binário instalado é *o que foi revisado* — e **não** que o autor publicou aquilo;
  - **o revisor não consegue ler um binário** como lê um texto: o parecer certificaria proveniência
    aceita, nunca ausência de malícia;
  - **a detecção de instalação clandestina era estruturalmente impossível**, porque os plugins vivem
    num diretório por-máquina compartilhado entre todos os projetos;
  - e a permissão de execução tardia seria **redução de janela, não controle**.

  **Removemos a superfície em vez de manter uma mitigação parcial que passaria falsa sensação de
  proteção.** Um gate que parece proteger e não protege é pior que a ausência declarada dele.

  **O que muda para você.** Instalar e executar ferramentas `trackfw-*` passa a ser **inteiramente
  responsabilidade sua**, invocando o binário direto no shell, sem intermediação do trackfw. Não há
  substituto embutido: se um sistema de extensão fizer sentido no futuro, nascerá com o gate
  desenhado desde o início, e não acrescentado depois.

  Detalhe completo em
  `docs/adr/ADR-2026-08-15-remocao-do-subsistema-de-plugins-em-vez-de-gate-de-binario-de-terceiro.md`
  e no parecer de segurança `docs/seguranca/2026-08-15-gate-de-plugins-binario.md`.

### Added

- **Gate de duas fases para instalação de skill/agent de terceiro via URL** — `trackfw <skills|agents>
  third-party fetch <url>` baixa para **quarentena** e **nunca instala**; a instalação só é consumada
  por `third-party install --checksum <sha256>`, mediante aprovação **vinculada por checksum** (o que
  fecha a janela entre revisar um conteúdo e instalar outro). Proveniência versionada em
  `.trackfw/thirdparty-provenance.json`, nova regra `thirdparty_artifact_has_provenance` no
  `trackfw validate`, e escopo `project` por padrão para que o artefato seja auditável no repositório.
  **Limite declarado:** a checagem de conteúdo é um *tripwire* para o caso descuidado, **não** um
  filtro contra adversário competente — paráfrase, indireção e homóglifo de outro alfabeto passam,
  por decisão registrada.
- **Bloqueio técnico de `git commit`/`git push`/criação de branch brutos por subagente** — novo
  comando `trackfw commit -m "<mensagem>"` (3 CLIs), que recusa commit direto em branch protegida e
  commit em `feat/fix/refactor` sem roadmap correspondente em `wip/`. Guard ligado por hook técnico
  nos 7 runtimes suportados: Claude Code, Codex CLI, Gemini CLI, GitHub Copilot, Cursor, Windsurf e
  Amazon Q Developer.
- **`trackfw changelog`** — consulta o `CHANGELOG.md` pelo próprio CLI.
- **`trackfw commit --suggest`** — imprime um esqueleto de mensagem em Conventional Commits derivado
  do diff staged (heurística estrutural, sem chamada de modelo).
- **Contexto de convenções do projeto para os agentes especialistas** — chave `agent_conventions` no
  `trackfw.yaml`, composta nos arquivos de regras dos agentes. `trackfw discover` sugere o framework
  de teste detectado, mas **nunca** grava a chave automaticamente.
- **`trackfw validate` detecta scripts de hook ausentes ou desatualizados** (`git-branch-guard`,
  escopo de projeto e global).

### Changed

- **Comando desconhecido agora é erro, com sugestão** — e a saída é **byte-idêntica nos 3 CLIs**,
  em `stderr`, com exit 1:

  ```
  Error: unknown command "vaildate" for "trackfw"
  Did you mean "validate"?
  Run 'trackfw --help' for usage.
  ```

  Substitui o comportamento anterior, em que um argumento desconhecido executava um binário do
  `PATH`. Um erro de digitação agora **sugere o comando certo** em vez de executar algo.

### Fixed

- **`trackfw branch new` e `trackfw ship` aceitam branches `chore` e `docs`**, sem exigir REQ +
  roadmap — alinhando os dois ao que o `trackfw commit` já fazia para branches de housekeeping. Em
  ambos, **vocabulário** e **gate** passaram a ser coisas separadas: o conjunto de tipos aceitos
  cresceu, mas o gate de governança **continua valendo integralmente** para `feat`, `fix` e
  `refactor`.

  Sem isso, o próprio **protocolo de release do projeto ficava inexecutável** pelos caminhos
  sancionados: a criação de branch de release era recusada por um comando e a publicação pelo outro,
  enquanto a via crua está bloqueada pelo guard de git. Efeito colateral não previsto do bloqueio
  técnico de git bruto, que só apareceu na primeira release publicada depois dele.

  Ganho colateral: `scripts/check-ship-parity.sh`, o primeiro contrato de paridade **comportamental**
  do `trackfw ship` — antes só se verificava que o nome do comando aparecia no `--help`.

### Notas de atualização

- **O bloqueio técnico de git bruto só passa a valer após `trackfw update`** (projetos existentes) ou
  `trackfw init` (projetos novos). Não é retroativo.
- Deny é global em todos os 7 runtimes, inclusive para o agente arquiteto/orquestrador — isolamento
  por subagente fica como débito técnico documentado.
- **Se você usava `trackfw plugins add`:** não há caminho de migração automático. Instale a ferramenta
  por conta própria e invoque o binário diretamente.

## [6.10.0] - 2026-08-14

### Added

- **Roteamento de model tier para Codex CLI e Cursor** (#167) — o catálogo canônico de
  agentes já declarava um tier de custo por agente (`model: opus` para `architect`,
  `model: sonnet` para os demais 9 especialistas), mas esse tiering só era efetivo para
  Claude Code e Antigravity CLI. Agora:
  - **Codex CLI**: `.codex/agents/trackfw-*.toml` passa a emitir `model = "gpt-5.4"`
    (tier `opus`) ou `model = "gpt-5.4-mini"` (tier `sonnet`) — antes o campo nunca era
    emitido, e o agente customizado sempre rodava no modelo default da sessão.
  - **Cursor**: `.cursor/agents/trackfw-*.md` passa a emitir `model: claude-opus-5[effort=high]`
    (tier `opus`) ou `model: composer-2.5[fast=true]` (tier `sonnet`) — antes emitia
    `opus`/`sonnet` verbatim, sintaxe não documentada como aceita pela Cursor.
  - `gemini` e `kiro` (mesma representação de agente compartilhada com o Cursor)
    permanecem byte-a-byte inalterados — comportamento coberto por teste de regressão
    dedicado nos 3 CLIs.

### Notas de atualização

- **A mudança só chega aos seus agentes depois de `trackfw agents update --targets codex,cursor`.**
- Sem Cursor CLI/Codex CLI instalados no ambiente de desenvolvimento, o fechamento desta
  REQ (ver `docs/req/REQ-2026-08-14-...md`) se apoiou em confirmação documental contra a
  documentação oficial de cada ferramenta, não em teste end-to-end ao vivo — risco
  residual caso a sintaxe aceita mude após 2026-08-14.

### Breaking Changes

Nenhum. Mudança aditiva: agentes já instalados só são afetados após `trackfw agents update`.

## [6.9.1] - 2026-08-13

### Fixed

- **Definições dos agentes auditores eram internamente contraditórias** (#165) — `code-quality`
  (Hefesto), `security` (Hades) e `ux` (Atena) declaravam *"You do not modify code"* enquanto o mesmo
  arquivo **ordenava** que eles acrescentassem entrada em `docs/agents-working-context.md`, e o
  `tools:` **não concedia** `Write`/`Edit`. Outras duas frases (*"Do not **edit code** without a
  requirement…"* e *"refuse to **implement** anything without…"*) pressupunham que o papel edita e
  implementa. **O arquivo exigia escritas que não concedia.**

  Efeito prático observado: sob a mesma redação, um auditor escrevia seus pareceres normalmente
  enquanto outro **recusava** microlotes de documentação equivalentes — roteamento imprevisível,
  dependente de como o pedido era redigido.

  Agora: `tools:` concede `Write, Edit`; a proibição é explicitamente de **código de produto**
  (`internal/`, `npm/src/`, `pypi/trackfw/` e testes); e o que o papel **pode** escrever está
  **afirmado** — relatório, working context e documentação designada pelo orquestrador — com a
  ressalva de que recusar isso é erro de escopo na direção oposta.

### Notas de atualização

- **A correção só chega aos seus agentes depois de `trackfw agents update`.** Sem `--force` é
  suficiente, desde que você não tenha editado os arquivos em `~/.claude/agents/` à mão.
- Agentes que **implementam** código (`backend`, `qa`, `dba`, `frontend`, `infra`, `iac`, `data`,
  `tooling`, `architect`) **não** foram alterados.

### Breaking Changes

Nenhum. Não há mudança de comportamento do CLI: apenas o conteúdo dos arquivos de definição de
agente gerados por `trackfw agents install`/`update`.

## [6.9.0] - 2026-08-13

Ciclo de trabalho sobre o **credential guard**, iniciado a partir de um bug de produção e conduzido
por medição: o que se descobriu foi que o guard **falhava aberto** — quando o hook não conseguia
rodar, a ferramenta prosseguia.

### Added

- **`credential_guard_hook_resolvable`** (#160) — o `validate` passa a detectar hook de
  credential-guard registrado cujo script **não existe** ou **não é executável**. Cobre a classe do
  incidente que abriu este ciclo.
- **`credential_guard_script_integrity`** (#162) — detecta **sobrescrita** do script, comparando o
  conteúdo em disco com o template desta versão do binário. Severidade **`warning`**: o script não
  carrega marcador de versão, então a regra **não consegue** distinguir *drift* legítimo (não rodou
  `trackfw update` após um bump) de adulteração — a mensagem é causalmente neutra por isso.
- **`credential_guard_mode_downgrade`** (#162) — detecta rebaixamento de `credential_guard.mode`
  comparando com o **último commit**, de forma direcional (`block` no `HEAD` → não-`block` no disco).
- **Gate de paridade byte-a-byte** do script do credential-guard entre os 3 CLIs (#162) — não existia;
  o teste anterior reconstruía Node/Python por regex do texto-fonte, **sem executar os runtimes**, e
  era cego a deriva de ordem de composição.
- Documentação de **usuário final** no `README.md` (#162, #163) sobre o que estas verificações
  **não** cobrem.

### Changed

- **Hooks de attention e wiring de caminho** (#156, na v6.8.0) — contexto do ciclo.
- **Severidade das 3 regras de credential-guard é resolvida pela mais estrita entre o `HEAD` e o
  disco** (#163). As outras ~38 regras seguem **inalteradas**, com teste de zero-delta.
- **Invocações de `git` do validador** passam a rodar com o ambiente **sem nenhuma variável `GIT_*`**
  e ancoradas em `git -C <root>` (#163).

### Fixed

- **Auto-silenciamento das regras de credential-guard** (#163) — elas podiam ser desligadas pela
  **mesma edição não commitada** que deveriam denunciar, via `rules:` no `trackfw.yaml`. Sem commit,
  **sem rastro**.
- **Bypass por variáveis de ambiente** (#163) — `GIT_DIR`/`GIT_WORK_TREE`, e qualquer `GIT_*` capaz de
  fazer o `git` falhar (ex.: `GIT_CONFIG_COUNT` malformado), derrotavam a ancoragem **em silêncio**.
- **Guard de vacuidade `credential-guard-present`** ganha prova negativa dedicada (#158) — o gate que
  existia para impedir falso verde era, ele próprio, não provado.

### ⚠️ Breaking Changes — leia antes de atualizar

**Violações das 3 regras de credential-guard não são mais suprimíveis via `.trackfw-baseline.json`.**

Se o seu projeto **tolera** hoje uma dessas violações pelo baseline, ela passa a ser **reportada**.
É intencional: um controle que pode ser silenciado por um arquivo não versionado não é controle.

Duas saídas legítimas, ambas deixando rastro:

```yaml
# trackfw.yaml — commite esta mudança
rules:
  credential_guard_hook_resolvable: off
```

ou corrija a causa (normalmente `trackfw update`, que regenera o script e o wiring).

### Notas

- **Isto é detecção, não prevenção.** Foi **medido** que não há prevenção técnica possível, no escopo
  do trackfw, contra um agente com escrita irrestrita ao workspace: em 4 dos 6 CLIs de agente, um
  hook que falha simplesmente deixa a chamada prosseguir.
- **`governance_mode: lenient` continua convertendo tudo em warning**, inclusive estas regras. O
  problema está **reduzido, não resolvido** — tratado separadamente.
- Sem `HEAD` (repositório sem commits, `trackfw.yaml` não versionado) não há âncora, e a resolução
  cai no disco.

## [6.8.0] - 2026-08-12

### Added

- **Migração in-place dos comandos de hook estendida a Codex e Gemini** (#156) — o helper que
  reescreve entradas antigas de `settings.json`/`hooks.json` existia apenas para o Claude Code
  (`migrateClaudeHookCommand`). Generalizado para `migrateHookCommand`/`_migrate_hook_command`
  (Go/Node/Python) e ligado aos injectors de Codex e Gemini, que também são *merge-based*. Sem isso,
  qualquer mudança futura nas strings desses CLIs faria `trackfw update` **acrescentar** a entrada
  nova ao lado da antiga quebrada, em vez de corrigi-la.
- **Documentação do mecanismo de resolução de caminho por CLI** (#156) — nova seção em
  `docs/cli-parity.md` registrando os 4 mecanismos distintos, por que a heterogeneidade é
  intencional, e as pré-condições do fix do Codex que **não constam da documentação do fornecedor**.

### Fixed

- **Hooks de attention do Claude Code falhavam com "No such file or directory" após `cd`** (#156) —
  mesma classe de bug corrigida em 6.7.1 para o `credential-guard`, que aquele release deixou
  explicitamente fora de escopo. `trackfw-attention-signal.sh` e `trackfw-attention-cleanup.sh`
  passam a usar `$CLAUDE_PROJECT_DIR/scripts/...` (Go/Node/Python). Frequência de disparo é menor
  que a do credential-guard porque os hooks de attention casam apenas o matcher `AskUserQuestion`.
- **Hooks do Codex CLI não resolviam a partir de subdiretório** (#156) — o Codex não expõe env var
  de raiz de projeto para hooks de repositório e executa os comandos com o `cwd` **da sessão**, que
  não é necessariamente a raiz. Os 6 comandos passam a ser emitidos como
  `"$(git rev-parse --show-toplevel)/scripts/..."`, forma recomendada pela própria documentação do
  fornecedor. Verificado empiricamente com `codex-cli` real, incluindo controle negativo (o caminho
  relativo antigo falha a partir de subdiretório; o novo funciona).
- **Hooks do Gemini CLI passam a resolver contra a raiz do projeto** (#156) — os 8 comandos passam a
  usar `$GEMINI_PROJECT_DIR/scripts/...`, forma usada em 100% dos exemplos oficiais. Mudança segura
  por construção: a variável resolve para a raiz independentemente de o `cwd` derivar ou não.

### Changed

- **Nada muda para Cursor, GitHub Copilot CLI e Kiro** (#156) — verificação em documentação primária
  mostrou que Cursor executa hooks de projeto a partir da raiz por design, e que o wiring do Copilot
  **já estava correto** por usar o campo nativo `"cwd": "."`. Kiro ficou como `INDETERMINADO`: a
  documentação oficial não descreve o diretório de trabalho da *Shell Command action*, e o padrão
  adotado é não alterar o que não se pode verificar. Registrado em `docs/cli-parity.md`.

### Notas de atualização

- Projetos com `settings.json`/`hooks.json` gerados por versões anteriores precisam rodar
  `trackfw update` para que a migração in-place reescreva as entradas antigas.
- O fix do Codex só produz efeito em projeto marcado como `trusted` em `~/.codex/config.toml` —
  fora disso o Codex ignora hooks de projeto silenciosamente. Comportamento do fornecedor, não do
  trackfw.

### Breaking Changes

Nenhum. As entradas antigas são migradas in-place; nenhuma ação manual é necessária além de rodar
`trackfw update`.

## [6.7.1] - 2026-08-09

### Fixed

- **`credential-guard` no Claude Code falhava com "No such file or directory" após `cd` para
  subdiretório** (#154) — o comando registrado em `.claude/settings.json` era um caminho relativo
  puro, resolvido contra o cwd *dinâmico* do hook (que rastreia `cd`s do agente durante a sessão),
  não a raiz do projeto. Passa a usar `$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh`
  (Go/Node/Python), env var que o Claude Code garante fixa na raiz do projeto. `settings.json` já
  gerados por versões antigas são migrados in-place ao rodar `trackfw update`/`init` de novo, em
  vez de acumular uma segunda entrada quebrada ao lado da corrigida. Escopo: só o wiring do Claude
  Code, CLI onde o bug foi reportado e reproduzido.
- **`pypi/trackfw/__init__.py` com fallback de versão desatualizado** (#154) — literal `6.6.0`
  esquecido no bump de release para `6.7.0`, quebrando `scripts/check-cli-parity.sh`.

## [6.7.0] - 2026-08-09

### Added

- **Cobertura de `Read`/`Write`/`Edit` no `credential-guard`** (#152) — o wiring gerado por
  `update harness`/`init`/`update` (Go/Node.js/Python, 6 CLIs nativos) passa a registrar o hook
  também para os tools de leitura/escrita de arquivo equivalentes a `Read`/`Write`/`Edit`, além do
  `Bash`/shell já coberto: Claude (`Read`/`Write|Edit`), Gemini
  (`read_file|read_many_files`/`write_file|replace`), Kiro (`read`/`write`), GitHub Copilot
  (`view`/`create|edit`), Cursor (`Read`/`Write` via eventos genéricos `preToolUse`/`postToolUse`).
  Codex documentado como limitação explícita — não expõe tool de leitura interceptável por hook;
  escrita/edição coberta via `apply_patch`.
- **Segunda camada de detecção do `credential-guard` — conteúdo de arquivo referenciado** (#152) —
  além de escanear o payload cru do tool call, o script agora resolve e escaneia (teto de 1MB) o
  conteúdo de alvos de redirect não-efêmeros e de argumentos de arquivo existente quando o comando
  é `cat`/`head`/`tail`/`jq`/`grep` — cobre o padrão `head -c 50 arquivo-com-segredo` sem exigir um
  resolvedor de dataflow completo.

### Changed

- **`credential-guard` em escopo global: modo default `warn` → `block`** (#152) — sem exigir novo
  arquivo de config: reusa a leitura de `credential_guard.mode` de `trackfw.yaml` já existente no
  escopo de projeto quando presente no cwd; sem essa config explícita, o fallback passa a bloquear
  em vez de só avisar. Quem já define `credential_guard: mode: warn` explicitamente no próprio
  `trackfw.yaml` não tem nenhuma mudança de comportamento.

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
