# Changelog

Todas as mudanças notáveis deste projeto são documentadas neste arquivo.

O formato segue [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
e este projeto adere a [Semantic Versioning](https://semver.org/).

> Entradas anteriores a esta versão foram reconstruídas a partir do
> histórico de commits (convenção `feat`/`fix`/`refactor`) para fins de
> backfill. A partir de `2.16.0`, este arquivo é atualizado como parte
> obrigatória do protocolo de release (ver `CLAUDE.md`).

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
