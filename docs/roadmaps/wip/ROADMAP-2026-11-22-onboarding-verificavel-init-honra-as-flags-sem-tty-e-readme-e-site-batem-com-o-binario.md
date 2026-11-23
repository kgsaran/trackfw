---
status: wip
date: 2026-11-22
req: "docs/req/REQ-2026-11-22-onboarding-verificavel-init-honra-as-flags-sem-tty-e-readme-e-site-batem-com-o-binario.md"
squad: "hades-tf, apolo-tf, afrodite-tf, atena-tf, hefesto-tf"
---

# Roadmap: Onboarding verificável: init honra as flags sem TTY e README e site batem com o binário

> Created: 2026-11-22 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-11-22-onboarding-verificavel-init-honra-as-flags-sem-tty-e-readme-e-site-batem-com-o-binario.md -->
REQ: docs/req/REQ-2026-11-22-onboarding-verificavel-init-honra-as-flags-sem-tty-e-readme-e-site-batem-com-o-binario.md

A auditoria `docs/qualidade/2026-11-22-auditoria-onboarding.md` registrou 25 achados (B.x, A.x, C.x, D.x). O ADR-2026-11-22
fixa D1–D6. Varredura de issues abertos em 2026-11-22: nenhum aberto.

## Acceptance Criteria
- [ ] AC1: `init` honra cada flag com e sem TTY; limites do brownfield (D2) afirmados por teste
- [ ] AC2: teste do Quick start confere README e guia do site em inglês contra os comandos que executa
- [ ] AC3: site em inglês na raiz e pt-BR em `/pt/`; build do site e capturas headless verdes
- [ ] AC4: bloqueantes e atritos da auditoria corrigidos ou justificados um a um
- [ ] AC5: README com Quick start contíguo, Windows fora de *Installation*, sem âncora quebrada
- [ ] AC6: `make quality` rc=0 e CI verde no PR

## Status Legend
⬜ Pending · 🔄 In progress · ✅ Done · ❌ Blocked

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model, flag sweep and link sweep (hades-tf)
**Status:** ✅ Done — `docs/seguranca/2026-11-22-wave0-onboarding-threat-model.md`. Flags: `--brownfield` ignorado sem
TTY e `--ai-tools` ignorado com TTY; `--identity-preset`, `--forge` e `--scope` honrados nos dois caminhos. Nenhum guard
impede um agente de rodar `trackfw init` pelo shell, e projeto do `init` padrão não tem `governance_mode`: daí o D2.
Achado novo, reproduzido pelo arquiteto: `presentTopLevelKeys` (`scaffold.go:937`) reconhece chave por texto, então
`"adr_dirs":` ou `adr_dirs :` ganham um segundo `adr_dirs: docs/adr` que vence o do usuário (ADR D2a, ML-1C). Links
de D3: 22 em 6 arquivos, 15 deles em `config.mts`, que o build não confere (gate por grep no `dist`, ML-1B). Auditoria
do arquiteto: o relatório põe a implementação do D2 "em `writeTrackfwConfig` no `init.go`"; a função fica em
`scaffold.go`. O D2 vai para `init.go` (não liga `BrownfieldMode` com `trackfw.yaml` existente), para que ML-1A e
ML-1C não dividam arquivo.
**Files affected:** `docs/seguranca/2026-11-22-wave0-onboarding-threat-model.md` (novo; único arquivo do hades);
`docs/qualidade/2026-11-22-auditoria-onboarding.md` (auditoria da atena-tf que originou a REQ, commitada com a governança)
**Actions:**
1. Enumeration completeness:
   - every `trackfw init` flag (`init.go:184-188` and any other) and the path that reads it (TTY, non-TTY or both),
     with the reading line;
   - whether `--ai-tools` is ignored in a real TTY (measured with a pty, e.g. `script -q /dev/null ...`, in a temp
     dir with isolated `HOME`/`USERPROFILE`/`CODEX_HOME`/`XDG_CONFIG_HOME`);
   - every inbound link that the site locale move (ADR D3) and the README heading moves (ADR D5) can break: grep the
     repository for `github.io/trackfw`, `/en/guide`, `/guide/` and `README.md#` in `internal/`, `npm/`, `pypi/`,
     `scripts/`, `site/`, `README.md`, `.github/` and `docs/cli-parity.md`;
   - scripts and tests that pin README or site content (`check-wheel-metadata-readme.sh`, `check-channels-content.sh`,
     `check-roadmap-barrier-contract.sh`, `check-shim-diff-applies.sh`, `quickstart_e2e_test.go`, any other).
2. Threat model for ADR D2:
   - what `init --brownfield` does on a project that already has `trackfw.yaml` (creates, extends or keeps
     `lenient_until`; downgrades an existing `governance_mode`), measured on the TTY path today;
   - whether the guards stop an agent from running `trackfw init` in a governed project;
   - a recommended D2 with the measurement behind it.
3. Falsification targets in both directions: flag ignored on one path (regression of D1); flag applied where it
   should not (brownfield relaxing a project already strict).
4. Declared residual.
**Acceptance criteria:**
- [x] The four sections above answered with evidence (commands and output), not a one-line assertion
- [x] No implementation line written for this ML; no file outside the one above

**Wave gates:**
```bash
test -s docs/seguranca/2026-11-22-wave0-onboarding-threat-model.md
```

## Wave 1 — Implementation: init and site locale (2 agents in parallel)
> Dependencies: Wave 0 audited and ADR D2/D2a written. apolo-tf runs ML-1A then ML-1C (disjoint files, same agent,
> sequential); afrodite-tf runs ML-1B in parallel (only `site/`).

### ML-1A — `init` honra as flags nos dois caminhos (apolo-tf)
**Status:** ✅ Done — pontos de injeção `initStdinIsTTY`/`initFormRunner` em `init.go`. D1: `--brownfield` lido sem
TTY; `--ai-tools` passado esconde a pergunta do wizard e vence no TTY. D2: `applyBrownfieldOrWarn`, com aviso em
`init.brownfieldYamlExists` (`internal/i18n/locales/en-US.json`, fora da lista do handoff mas exigido pelo i18n;
aceito). `guard.go:37` lista os três guards; contrato D1/D2/D2a em `docs/cli-parity.md`. Testes:
`TestInitD1_NonTTY_BrownfieldNewProject`, `TestInitD2_NonTTY_BrownfieldExistingYAML_GovernanceUnchanged`,
`TestInitD1_TTY_AIToolsFlag_SkipsWizard`. Auditoria do arquiteto, sonda no binário contra o 11.8.0: projeto novo +
`--brownfield` sem TTY grava `governance_mode` (antes 0); `trackfw.yaml` existente + `--brownfield --ai-tools claude`
mantém a governança, imprime o aviso e instala 12 agentes. O apolo não entregou o contra-braço pedido, rodou
`go test ./...` (proibido no handoff), editou o roadmap e deixou um `check-gates-falsify.sh` órfão, que encerrei.
**Files affected:** `internal/commands/init.go`; `internal/commands/init_flags_both_paths_test.go` (novo);
`internal/commands/guard.go`; `internal/i18n/locales/en-US.json`; `docs/cli-parity.md`
**Actions:**
1. ADR D1: `--brownfield` read on the non-TTY path; `--ai-tools`, when passed (`Changed`), wins on the TTY path and
   skips the AI tools question. `--identity-preset`, `--forge` and `--scope` already behave on both paths (ML-0A §1.1).
2. ADR D2 in `init.go`: with `trackfw.yaml` present, `--brownfield` does not set `BrownfieldMode` and prints the D2
   warning; rc=0; the rest of init runs. Without `trackfw.yaml`, both paths write `governance_mode: lenient` and
   `lenient_until`.
3. Tests with arm and counter-arm per path. Fixtures from ML-0A §2.1/§2.4: new project + `--brownfield` non-TTY →
   lenient fields written; existing yaml without `governance_mode` + `--brownfield --ai-tools claude` non-TTY → yaml
   governance unchanged, warning printed, agents installed, no `[LENIENT MODE]`; TTY path through an injection point
   for the wizard (create one if none exists).
4. `guard.go:37`: the no-subcommand message lists all guards (`git-branch`, `write-scope`, `credential`).
5. `docs/cli-parity.md`: `init` contract for D1/D2/D2a, and the line that cites `site/en/guide/commands.md` (ML-0A
   §1.3) updated to the post-move paths `site/guide/commands.md` and `site/pt/guide/commands.md`.
6. Existing tests and `scripts/check-gates-falsify.sh` scenarios that pin the old behavior change with it. No new
   falsify scenario is required; Go tests carry D1/D2.
**Acceptance criteria:**
- [x] AC1 da REQ (D1 e D2)
- [x] `go build ./...` e `go test ./internal/commands/` verdes

### ML-1C — Merge do `trackfw.yaml` reconhece chaves pelo parse YAML (apolo-tf)
**Status:** ✅ Done, com corretivo no ML-1D — `presentTopLevelKeys` usa parse `yaml.Node` nos dois chamadores.
Sonda do arquiteto contra o 11.8.0: `"adr_dirs":`, `adr_dirs :` e BOM deixam de ganhar um segundo `adr_dirs`
(2 → 1); YAML inválido continua abortando o `init` (rc=1, sem escrita). Achados da auditoria, no ML-1D: com YAML
inválido o detector devolve vazio, e o `lefthook.yml` inválido ganharia um `pre-commit` duplicado (antes, a detecção
por texto o achava); o teste antigo `TestPresentTopLevelKeys_P1P2P3` (comentário = ausente, prefixo `roadmap_dir`
× `roadmap_namespacing`) foi removido sem que as propriedades fossem reafirmadas; falta teste de ponta a ponta do merge
(só o detector é testado).
**Files affected:** `internal/generators/scaffold.go`; `internal/generators/trackfw_config_merge_test.go`
**Actions:**
1. ADR D2a: `presentTopLevelKeys` (`scaffold.go:937`) derives the present top-level keys from a `yaml.Node` parse,
   the same normalization the config reader uses (`internal/config/config.go`), for both call sites (`:1113`
   `trackfw.yaml`, `:2357` lefthook).
2. Unparseable existing content keeps today's safe behavior (do not overwrite the user's file); state the choice in
   the report.
3. Tests: quoted key, space before `:`, BOM on line 1, for `adr_dirs` and `governance_mode` → no duplicate key after
   init; counter-arm: a key really absent is still appended.
**Acceptance criteria:**
- [x] AC1 da REQ (D2a)
- [x] `bash scripts/check-init-preserves-user-config.sh` rc=0

### ML-1D — Corretivo do ML-1C (apolo-tf)
**Status:** ✅ Done — com YAML inválido, `presentTopLevelKeys` cai no detector antigo
(`presentTopLevelKeysFallback`). P1–P3 reafirmadas nos dois caminhos (`TestPresentTopLevelKeys_P1P2P3`). Testes de
ponta a ponta: `TestWriteTrackfwConfig_D2a_QuotedAdrDirs`, `_SpaceBeforeColon`, `_BOM`,
`TestGenerateLefthookHook_D2a_QuotedPreCommit`, `_InvalidYAMLFallback`. Contra-braço do apolo: com o detector só de
texto, os 4 primeiros reprovam por duplicata; o último reprova na precondição (`ok`). `internal/generators` rc=0
(rodado também pelo arquiteto) e `check-init-preserves-user-config.sh` rc=0. Achado de mesma causa no ML-1E: a inserção
do lefthook compara `line == "pre-commit:"` (`scaffold.go:2359,2421`), então com `"pre-commit":` o `trackfw-validate`
não é instalado e sai um aviso falso.
**Files affected:** `internal/generators/scaffold.go`; `internal/generators/trackfw_config_merge_test.go`
**Actions:**
1. When `yaml.Node` parsing fails, `presentTopLevelKeys` falls back to the previous text detector (column-0 key before
   `:`), so invalid content keeps exactly the pre-REQ behavior; the `bool` is used by the callers only if needed.
2. Restore the P1–P3 assertions of the removed `TestPresentTopLevelKeys_P1P2P3` (commented key is absent; indented key
   is not top-level; `roadmap_dir` does not match `roadmap_namespacing`) against the new implementation.
3. End-to-end merge test: existing `trackfw.yaml` with `"adr_dirs":`, `adr_dirs :` and BOM on line 1 → after
   `writeTrackfwConfig`, exactly one `adr_dirs` and the user's value kept; `lefthook.yml` with a quoted
   `"pre-commit":` → no second `pre-commit`; invalid `lefthook.yml` with `pre-commit:` → no second block.
4. Counter-arm: with the old text-only detector, the end-to-end test fails (report the failing names).
**Acceptance criteria:**
- [x] AC1 da REQ (D2a), afirmado de ponta a ponta
- [x] `go test ./internal/generators/` e `bash scripts/check-init-preserves-user-config.sh` rc=0

### ML-1E — Inserção do lefthook reconhece a chave pelo parse YAML (apolo-tf)
**Status:** ✅ Done — helper `normalizeYAMLTopLevelKey` (BOM, aspas simples e duplas, espaço antes do `:`) nos dois
sítios. `TestGenerateLefthookHook_D2a_QuotedPreCommit` afirma também a instalação do `trackfw-validate` e a ausência
do aviso; `TestLefthookValidatePresent_D2a_QuotedKey` é novo. Contra-braço do apolo: com a comparação exata, os dois
reprovam. `internal/generators` rc=0 e `check-init-preserves-user-config.sh` rc=0.
**Files affected:** `internal/generators/scaffold.go`; `internal/generators/trackfw_config_merge_test.go`
**Actions:**
1. Same cause as D2a on the insertion side: `generateLefthookHook` and `lefthookValidatePresent` track the
   `pre-commit` section with `line == "pre-commit:"` (`scaffold.go:2359,2421`). A top-level key written as
   `"pre-commit":`, `'pre-commit':` or `pre-commit :` must be recognized as the same section, so `trackfw-validate`
   is inserted (or detected as present) and no false "no commands: section" warning is printed.
2. `TestGenerateLefthookHook_D2a_QuotedPreCommit` also asserts that `trackfw-validate` is installed.
3. Counter-arm: with the exact-string comparison, the test fails.
**Acceptance criteria:**
- [x] `trackfw-validate` instalado e detectado com a chave `pre-commit` em qualquer grafia aceita pelo YAML
- [x] `go test ./internal/generators/` rc=0

### ML-1B — Inglês na raiz do site (afrodite-tf)
**Status:** ✅ Done — inglês em `site/` e pt-BR em `site/pt/`; `site/en/` removido; `config.mts` com `root` en-US e
`pt` pt-BR; 7 links nas páginas e 15 pontos no `config.mts`. `npm ci && npm run build` rc=0. Grep do `dist`: nenhum
`/trackfw/en/`; no `dist/pt`, só os 3 links do seletor de idioma para a página equivalente em inglês. Auditoria do
arquiteto: capturas headless de `/trackfw/`, `/trackfw/pt/` e `/trackfw/guide/getting-started.html` sem asset quebrado,
com idioma certo e seletor visível (a afrodite não conseguiu gerar os PNGs: o Chromium do brew trava no
`--screenshot`; usei o `chrome-headless-shell` do Playwright). A afrodite marcou o status no roadmap, fora da lista
de arquivos dela, e deixou um Chromium órfão, que encerrei. A nav do pt mostra "IA Agents" (conteúdo, ML-2A).
**Files affected:** `site/.vitepress/config.mts`; `site/index.md`; `site/guide/getting-started.md`;
`site/guide/commands.md`; `site/guide/ai-agents.md`; `site/pt/index.md`; `site/pt/guide/getting-started.md`;
`site/pt/guide/commands.md`; `site/pt/guide/ai-agents.md`; `site/en/index.md`, `site/en/guide/getting-started.md`,
`site/en/guide/commands.md`, `site/en/guide/ai-agents.md` (removidos pela movimentação)
**Actions:**
1. Move the English pages to the root and the pt-BR pages to `site/pt/` (`site/en/*` → `site/*`,
   `site/guide/*` and `site/index.md` → `site/pt/...`). Content unchanged in this ML.
2. Update `config.mts`: `root` locale en-US, `pt` locale pt-BR, nav, sidebar and OG tags.
3. Update the 22 inbound links ML-0A §1.3 listed inside `site/` (15 in `config.mts`). The `docs/cli-parity.md` line
   belongs to ML-1A.
**Acceptance criteria:**
- [x] `npm ci && npm run build` em `site/` rc=0 (o VitePress reprova link morto)
- [x] Captura em Chrome headless de `/trackfw/` (inglês) e `/trackfw/pt/` (pt-BR) sem asset quebrado
- [x] Nenhum `href="/trackfw/en/` no `site/.vitepress/dist`, e nenhum link do `dist/pt` para páginas da raiz em
  inglês fora do seletor de idioma (calibrar na primeira build e registrar no relatório)
- [x] Nenhum arquivo fora de `site/` (exceto o status no roadmap, registrado acima)

**Wave gates:**
```bash
go build ./...
TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 5m ./internal/commands/
TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 5m ./internal/generators/
bash scripts/check-init-preserves-user-config.sh
test -f site/guide/getting-started.md
test -f site/pt/guide/getting-started.md
test ! -e site/en
```

## Wave 2 — Content (2 MLs in parallel)
> Dependencies: Wave 1 audited. ML-2A touches only `site/`; ML-2B touches only `README.md`.

### ML-2A — Conteúdo do site (afrodite-tf)
**Status:** ✅ Done, com corretivo no ML-2C — B.1–B.8, B.10–B.17 e D.2 corrigidos nos dois idiomas; o guia em inglês
usa as 4 linhas do Quick start do README literalmente (conferido por grep: 2 ocorrências no guia, 1 no README);
saída do `validate` mostra os 2 avisos reais; brownfield D2 documentado; nav pt "Agentes de IA". Build rc=0. Captura
do guia em inglês conferida pelo arquiteto. Achados da auditoria, no ML-2C: `adr new`/`req new` sem título sobraram
na referência de comandos; a nota do `install.ps1` está desatualizada.
**Files affected:** `site/index.md`; `site/guide/getting-started.md`; `site/guide/commands.md`;
`site/guide/ai-agents.md`; `site/pt/index.md`; `site/pt/guide/getting-started.md`; `site/pt/guide/commands.md`;
`site/pt/guide/ai-agents.md`; `site/.vitepress/config.mts` (só o rótulo "IA Agents" → "Agentes de IA" da nav pt)
**Actions:** fix in both locales the audit findings B.1–B.8, B.10–B.17 and D.2, with every command run against
`bin/trackfw` in an isolated temp dir. Output blocks that change every release (`status`) become a functional
description instead of a literal snapshot. The English getting-started uses the same command lines as the README
Quick start, so ML-3A can pin both.
**Acceptance criteria:**
- [x] Cada achado listado corrigido, ou justificado no relatório
- [x] `npm ci && npm run build` em `site/` rc=0; capturas headless das duas raízes

### ML-2C — Corretivo do ML-2A (afrodite-tf)
**Status:** ✅ Done — `adr new "Decision title"` e `req new "Requirement title"` na referência dos dois idiomas, com a
frase de que o título é obrigatório; nota do `install.ps1` trocada por "publicado em toda release" (asset conferido na
v11.8.1). Grep vazio (conferido pelo arquiteto); build rc=0.
**Files affected:** `site/guide/commands.md`; `site/pt/guide/commands.md`; `site/guide/getting-started.md`;
`site/pt/guide/getting-started.md`
**Actions:**
1. B.1 left in the command reference: `trackfw adr new` and `trackfw req new` without a title (`commands.md:261,295`
   in both locales) get the positional title, as in the guide.
2. Stale note at `getting-started.md:38` (both locales): `install.ps1` is already a release asset (measured on
   v11.8.1); the note says it ships "after 11.1.0" and tells Windows users to avoid it.
**Acceptance criteria:**
- [x] `grep -nE 'trackfw (adr|req) new$' site/guide/*.md site/pt/guide/*.md` vazio
- [x] `npm run build` em `site/` rc=0

### ML-2B — README contíguo (atena-tf)
**Status:** ✅ Done, com corretivo no ML-2D — Quick start e continuação contíguos (linhas 19 e 40); Windows virou
`## Windows support (partial)`, com a âncora mantida; B.19 corrigido (2 parágrafos em negrito viraram títulos, 0
âncora quebrada); A.2: o comentário do `validate` diz "0 violations, 2 warning(s)", medido; B.18: `branch new` antes
do `ship`. Diff ordenado: só movimentação e as linhas listadas; +4 linhas. Gates: wheel-metadata, `guardforms` e
`TestQuickStart` rc=0. Auditoria do arquiteto: o bloco de Windows (320 linhas) ficou **antes** de *The problem* e
*Installation*, empurrando o pitch e a instalação para a linha 379. Foi seguido o texto do ADR D5 ("depois deles"),
que eu escrevi mal; a intenção era tirar o Windows do caminho de leitura de macOS/Linux. ML-2D o move para depois de
*Installation*.
**Files affected:** `README.md`
**Actions:**
1. ADR D5: `## Quick start, continued` right after `## Quick start`; `### Windows support (partial)` leaves
   *Installation* for its own top-level section after them. No content removed.
2. Update every internal anchor affected and fix B.19 (anchors that point to bold paragraphs).
3. Fix A.2: the `validate` line promises the output the Quick start really produces.
4. Fix B.18: `trackfw branch new feat/<slug>` before `ship` in *Quick start, continued*.
**Acceptance criteria:**
- [x] AC5 da REQ; lista das âncoras antes/depois no relatório
- [x] `bash scripts/check-wheel-metadata-readme.sh` rc=0

### ML-2D — Windows depois de *Installation* (atena-tf)
**Status:** ✅ Done — ordem de topo: Quick start (19), continued (40), The problem (59), Installation (74), Windows
support (partial) (136), Commands (456). Diff ordenado vazio: só reordenação. wheel-metadata e `guardforms` rc=0. A
atena relatou como quebrada a âncora `#guard-hooks-on-windows--measured-state-since-v930`, que no ML-2B tinha dado
como OK. Pelo algoritmo do GitHub (github-slugger: minúsculas, remove pontuação, cada espaço vira hífen), o título
`Guard hooks on Windows — measured state (since v9.3.0)` gera exatamente esse id; o checker do ML-2D é que diverge.
Âncora válida.
**Files affected:** `README.md`
**Actions:** move the whole `## Windows support (partial)` section (with its subsections) to right after
`## Installation`, before `## Commands`. Nothing else changes; anchors keep working.
**Acceptance criteria:**
- [x] Ordem de topo: Quick start, Quick start continued, The problem, Installation, Windows support (partial), Commands
- [x] `bash scripts/check-wheel-metadata-readme.sh` e `go test ./internal/guardforms/` rc=0

**Wave gates:**
```bash
test -f site/guide/getting-started.md
test -f site/guide/commands.md
bash scripts/check-wheel-metadata-readme.sh
```

## Wave 3 — Docs pinned by test
> Dependencies: Wave 2 audited (the test pins the final text).

### ML-3A — Teste do Quick start confere README e site (apolo-tf)
**Status:** ✅ Done — lista única `quickStartLines`; `TestQuickStartEndToEnd` executa a partir dela;
`TestQuickStartDocsPinned` exige as 5 linhas como bloco contíguo no README e em `site/guide/getting-started.md`,
`2 warning(s)` nos dois e nenhum `✓ No violations found.` na linha do `validate` do README. Contra-braços permanentes
(`_CounterArm_README`, `_CounterArm_SiteGuide`) com cópia em `t.TempDir()`; o bloco contíguo evita falso verde pela
linha `init` repetida no site. Contra-braço do arquiteto no arquivo real: trocar a linha `roadmap move` do site pela
forma antiga (`REQ-... wip`) → rc=1; restaurado → rc=0. `internal/commands` rc=0 (62 s).
**Files affected:** `internal/commands/quickstart_e2e_test.go`
**Actions:** ADR D4: the test keeps the list of commands it runs and asserts that each one appears literally in
`README.md` and `site/guide/getting-started.md`, and that the promised `validate` output in both documents is what
the run produces. No markdown parsing: a line-presence check. Counter-arm: removing or editing one command line
in either document makes the test fail.
**Acceptance criteria:**
- [x] AC2 da REQ
- [x] contra-braço medido: editar uma linha do Quick start em cada documento reprova o teste

**Wave gates:**
```bash
TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 5m -run 'TestQuickStart' ./internal/commands/
```

## Wave 4 — Review and red team (2 MLs in parallel)
> Dependencies: Wave 3 audited.

### ML-4A — Revisão de qualidade (hefesto-tf)
**Status:** ✅ Done — APROVA com ressalvas; 0 bloqueante. R1 (`presentTopLevelKeysFallback` guarda a chave com aspas:
`lefthook.yml` inválido com `"pre-commit":` duplica o bloco) vai para o ML-4C. O1 (rodar `go test
./internal/generators/` alteraria o `trackfw.yaml` do repositório) **não reproduzido** pelo arquiteto: `-count=1` no
pacote inteiro, rc=0, md5 do `trackfw.yaml` idêntico antes e depois.
**Files affected:** `docs/qualidade/2026-11-22-onboarding-review.md`
**Acceptance criteria:**
- [x] Parecer escrito; achados bloqueantes corrigidos por ML corretivo

### ML-4B — Red team (hades-tf)
**Status:** ✅ Done, REPROVA corrigido no ML-4C — D1 com pty real fechado: `--ai-tools claude` instala 12 agentes sem
perguntar; `--brownfield` grava lenient pelo caminho TTY (residuais 9 e 10 da Wave 0). D2: symlink, symlink quebrado e
diretório abortam com rc=1 (falha fechada). F-9 (bloqueante, mesma causa do D2a): `presentTopLevelKeys` não expande
merge key `<<: *anchor`, então o `init` acrescenta o padrão de chaves herdadas por âncora, e o leitor fica com o
explícito (`require_req_in_commit: true` → `false`; `wip_limit: 3` → `1`). Pré-existente no detector antigo, mas é a
causa do D2a: ML-4C. F-10 e R1 (mesmo detector, lado texto) e F-8 (conferência de `2 warning(s)` no site aceita prosa
fora do bloco de saída) também no ML-4C. F-4 (`trackfw.yml` ignorado; o leitor também o ignora) e F-6 (BOM no meio do
arquivo; o leitor também falha) ficam como residuais: não há rebaixamento. F-5 (write-scope re-enraizado por
`trackfw.yaml` aninhado, `internal/guard/writescope.go:504`) é outra causa, pré-existente: REQ própria. **Incidente:**
o hades rodou um `init --brownfield` com o HOME real, contra o handoff. O arquiteto conferiu: nenhum arquivo de
`~/.claude`, `~/.codex`, `~/.agents`, `~/.gemini` ou `~/.config/opencode` com mtime de hoje (fora o sync do app em
`~/.claude/skills/synced`); `~/.trackfw/integrations-manifest.json` tem mtime 20:32, anterior ao red team.
**Files affected:** `docs/seguranca/2026-11-22-red-team-onboarding.md`
**Acceptance criteria:**
- [x] Reimplementa a falsificação de D1, D2 e D4 a partir do contrato, não do diff; residual declarado

### ML-4C — Corretivo da Wave 4 (apolo-tf)
**Status:** ✅ Done, com sítio de mesma causa no ML-4D — `presentTopLevelKeys` expande merge key pela tag `!!merge`
(`<<: *a`, `<<: [*a, *b]`, aninhado); `normalizeYAMLTopLevelKey` faz parse YAML da linha (escape decodificado), com
texto como fallback, e `presentTopLevelKeysFallback` o usa (R1); a conferência do site exige `2 warning(s)` dentro do
bloco de saída (F-8). Testes: `TestPresentTopLevelKeys_D2a_MergeKey`, `TestWriteTrackfwConfig_D2a_MergeKey`,
`TestNormalizeYAMLTopLevelKey_EscapeSequence`, `TestPresentTopLevelKeysFallback_QuotedKey`,
`TestQuickStartDocsPinned_CounterArm_SiteGuideOutputFence`; contra-braço por item reportado. `internal/generators` e
`internal/commands` rc=0. **Achado do apolo, medido pelo arquiteto no binário:** o leitor (`config.normalizeMapping`)
também não expande merge key — com `<<: *d` e `wip_limit: 3` na âncora, o `validate` avisa "2 roadmaps in wip/
(limit: 1)", igual ao `wip_limit: 1` explícito. Então o rebaixamento do F-9 não acontecia na leitura: o leitor sempre
ignorou âncoras. Depois do ML-4C o escritor reconhece a chave herdada e o leitor não, o que contraria o D2a ("como o
leitor reconhece"). ML-4D corrige o leitor.
**Files affected:** `internal/generators/scaffold.go`; `internal/generators/trackfw_config_merge_test.go`;
`internal/commands/quickstart_e2e_test.go`
**Actions:**
1. F-9: `presentTopLevelKeys` expands merge keys (`<<: *a` and `<<: [*a, *b]`) and adds every inherited key.
2. One key normalizer: `normalizeYAMLTopLevelKey` derives the key of a column-0 line through the YAML parser (so
   escapes such as `"pre\x2dcommit"` decode, F-10), with the text rules only as fallback;
   `presentTopLevelKeysFallback` uses it (R1).
3. F-8: the site check asserts `2 warning(s)` inside the output block that follows the Quick start run, not anywhere
   in the page.
4. Tests with counter-arm for each item; reconciliation sentence per test.
**Acceptance criteria:**
- [x] `trackfw.yaml` com chave de governança herdada por âncora mantém o valor do usuário após `init`
- [x] `go test ./internal/generators/` e `go test ./internal/commands/` rc=0

### ML-4D — Leitor de config expande merge key (apolo-tf)
**Status:** ✅ Done, com sítio de mesma família no ML-4E — `normalizeMapping` em duas passadas (explícitas, depois
merge preenchendo lacunas), `applyMergeNode`/`applyMergeMapping` com sequência e aninhamento. 6 testes em
`config_merge_key_test.go`; contra-braço do apolo: 5 reprovam sem a mudança (`TestMergeKey_QuotedKeyIsNotMerge` passa
nos dois, por desenho). Ponta a ponta: com âncora `wip_limit: 3`, o aviso "limit: 1" some. `internal/config`,
`internal/validator` e `internal/generators` rc=0. Residual do apolo, medido pelo arquiteto: âncora auto-referente
(`d: &d` / `<<: *d`) derruba o binário com estouro de pilha (rc=2) **já no 11.8.0** (`normalizeMapping` ↔
`normalizeNode`), e continua derrubando depois do ML-4D (`applyMergeNode` ↔ `applyMergeMapping`). ML-4E.
**Files affected:** `internal/config/config.go`; `internal/config/config_merge_key_test.go` (novo)
**Actions:**
1. `normalizeMapping` honors YAML merge keys (tag `!!merge`, `<<: *a` and `<<: [*a, *b]`, nested) with YAML
   semantics: explicit keys of the mapping win over merged ones regardless of order; in a sequence, earlier sources
   win over later ones. Applies to nested mappings too (same function).
2. Tests: `wip_limit` and `require_req_in_commit` from an anchor are read; explicit key overrides the merged one;
   sequence precedence; nested merge. Counter-arm: without the change, the tests fail.
3. End-to-end: the anchored `trackfw.yaml` makes `validate` respect `wip_limit: 3`.
**Acceptance criteria:**
- [x] o leitor e o escritor reconhecem as mesmas chaves num `trackfw.yaml` com âncora (ADR D2a)
- [x] `go test ./internal/config/` e `go test ./internal/validator/` rc=0

### ML-4E — Âncora cíclica não derruba leitor nem escritor (apolo-tf)
**Status:** ✅ Done — conjunto `visiting` por caminho (entra na chegada, sai com `defer`) nas 4 funções do leitor e
nas 2 do detector do escritor; elemento cíclico de valor vira `nil` (descartado nas listas). O ciclo mútuo com
referência adiante (`a: &a {<<: *b}`) não é YAML válido para o yaml.v3 ("unknown anchor"); o apolo usou a forma
aninhada. Testes: `TestCyclicAnchor_SelfRefMapping`, `_SelfRefSequenceValue`, `_MutualCycle`,
`_SameAliasTwiceNoCycle`; `TestPresentTopLevelKeys_CyclicSelfRefMapping`, `_CyclicSequence`,
`_CyclicMutualNested`, `_SameAliasTwice`; `TestWriteTrackfwConfig_CyclicMergeKey`. Contra-braço do apolo por binário
do HEAD anterior: estouro de pilha, rc=2. Auditoria do arquiteto no binário novo: os três `trackfw.yaml` cíclicos
(auto-referente, sequência, mútuo aninhado) com `validate` e `init` rc=0, sem estouro; a âncora `wip_limit: 3`
continua respeitada. Varredura do apolo: nenhum outro caminhador de alias em `internal/`.
**Files affected:** `internal/config/config.go`; `internal/config/config_merge_key_test.go`;
`internal/generators/scaffold.go`; `internal/generators/trackfw_config_merge_test.go`
**Actions:**
1. Cycle guard in the config reader (`normalizeNode`, `normalizeMapping`, `applyMergeNode`, `applyMergeMapping`) and in
   the writer's key detector (`presentTopLevelKeys`, `expandTopLevelKeys`, `expandMergeValue`): a node already on the
   current expansion path is not expanded again.
2. Contract for a cyclic `trackfw.yaml`: no crash; the reader keeps every key it can read without following the cycle;
   the writer treats the cyclic key as present (no append).
3. Tests: `d: &d` / `<<: *d`; `x: &x [*x]`; mutual cycle `a: &a {<<: *b}` / `b: &b {<<: *a}`; counter-arm: without the
   guard the test process overflows (run the counter-arm as a subprocess or assert by timeout, never crashing the
   package run).
4. End-to-end: `bin/trackfw validate` and `init` on a cyclic `trackfw.yaml` return without a stack overflow.
**Acceptance criteria:**
- [x] nenhum `trackfw.yaml` derruba o binário por ciclo de âncora (medido no binário)
- [x] `go test ./internal/config/` e `go test ./internal/generators/` rc=0

### ML-4F — Red team dos corretivos ML-4C a ML-4E (hades-tf)
**Status:** ✅ Done, ressalva R-A no ML-4G — APROVA com ressalvas. Nenhuma divergência entre leitor e escritor em 10
grafias (escalar e sequência em merge, `!!merge` explícito, `<<` duplicado, merge aninhado, escape, multi-documento,
chave número, chave complexa). Âncora de `governance_mode: lenient` agora vale (o 11.8.0 ignorava); chave explícita
`strict` vence a herdada. Ciclos via sequência protegidos. Guards: write-scope falha fechado (faz um primeiro
`yaml.Unmarshal` em `interface{}`, que tem limitador de alias), credential e git-branch não leem o YAML como nó.
R-A: bomba de alias (30 níveis) trava `validate` e `init` por mais de 30 s **já no 11.8.0**; decodificar para
`yaml.Node` ignora o limitador do yaml.v3, e depois do ML-4C o escritor também percorre o merge. R-B (`credential_guard`
`mode:` via âncora não detectado) é residual já declarado no ADR D9. HOME isolado em todos os comandos. O hades
escreveu a nota de vault `vault/notes/yaml-node-alias-bomb-bypass-interface-limit-2026-11-22.md` e a entrada no
`vault/notes/index.md`, fora da lista do handoff; aceitas, porque a regra de vault pede a nota.
**Files affected:** `docs/seguranca/2026-11-22-red-team-onboarding-corretivos.md`;
`vault/notes/yaml-node-alias-bomb-bypass-interface-limit-2026-11-22.md`; `vault/notes/index.md`
**Acceptance criteria:**
- [x] Falsifica D2a no leitor e no escritor depois de ML-4C/4D/4E (merge key, ciclo, normalizador único) a partir do
  contrato; residual declarado

### ML-4G — Limite de alias no leitor e no escritor (apolo-tf)
**Status:** ✅ Done, incompleto: ML-4H fecha — primeira passada em `interface{}` em `Load()` e em
`presentTopLevelKeys`; só o erro "excessive aliasing" desvia (leitor: YAML malformado, rc=1; escritor: detector de
texto). Ponta a ponta medido pelo apolo: bomba de 30 níveis, `validate` e `init` de 30 s (timeout) para 0,01 s, rc=1.
6 testes (`TestAliasBomb_*`). Auditoria do arquiteto: a proteção depende do texto da mensagem do yaml.v3 e o próprio
apolo mostrou dois desvios — um ciclo ou um `<<` duplicado **antes** da bomba muda o erro da primeira passada e a
bomba volta a travar — além de `ParseRulesFromContent`, `ParseDirsFromContent`, `ReadNamespacingConfig` e
`ReadAgentConventions`, que chamam `parse()` sem passar por `Load()`. Mesma causa: o caminhador não tem limite.
**Files affected:** `internal/config/config.go`; `internal/config/config_merge_key_test.go`;
`internal/generators/scaffold.go`; `internal/generators/trackfw_config_merge_test.go`
**Actions:**
1. Before walking the `yaml.Node` tree, both the reader (`config` parse path) and the writer (`presentTopLevelKeys`)
   run the same first pass the write-scope guard uses: `yaml.Unmarshal` into `interface{}`, which carries yaml.v3's
   aliasing limiter. On "excessive aliasing" the reader handles the file as invalid YAML (same path and message style
   as any other parse error) and the writer falls back to the text detector (no append on keys it sees).
2. Tests: a 30-level sequence alias bomb returns within 2 s on both sides; counter-arm: without the first pass, the
   same fixture exceeds the timeout (run as subprocess or with a short deadline, never hanging the package run).
3. End-to-end: `validate` and `init` with the bomb return in under 2 s with a clear error.
**Acceptance criteria:**
- [x] bomba de alias de 30 níveis: `validate` e `init` terminam em menos de 2 s (medido no binário)
- [x] `go test ./internal/config/` e `go test ./internal/generators/` rc=0

### ML-4H — Orçamento de visitas no caminhador (apolo-tf)
**Status:** ✅ Done — `walkCtx`/`scaffoldCtx` com `visiting` e contador; orçamento de 10 000 visitas (o `trackfw.yaml` do
repositório faz ~15); `parse()` passa a devolver erro: `Load()` sai como YAML malformado, os demais chamadores seguem
com o que foi lido; o escritor cai no detector de texto. A primeira passada do ML-4G foi removida (o orçamento é o
controle). 9 testes novos (`TestAliasBomb_Budget_*`, `TestAliasBomb_WriterBudget_*`), contra-braços com limite 1<<30.
Auditoria do arquiteto no binário, contra o 11.8.0: bomba sozinha, ciclo antes e `<<` duplicado antes — `validate` e
`init` com rc=1 e "malformed YAML" em até 0,41 s (no 11.8.0: timeout de 30 s ou estouro de pilha, rc=2); o
`trackfw.yaml` do repositório segue com rc=0. O relatório do apolo dizia `EXIT:0` na ponta a ponta; o rc medido é 1.
**Files affected:** `internal/config/config.go`; `internal/config/config_merge_key_test.go`;
`internal/generators/scaffold.go`; `internal/generators/trackfw_config_merge_test.go`
**Actions:**
1. A node-visit budget inside the walkers themselves (`parse()` → `normalizeMapping`/`normalizeNode`/`applyMerge*`, and
   `presentTopLevelKeys` → `expandTopLevelKeys`/`expandMergeValue`), shared per top-level call. When exceeded, the
   reader returns an error that every caller of `parse()` handles as malformed YAML, and the writer falls back to the
   text detector. Budget value stated and justified in the report (well above any real `trackfw.yaml`).
2. The ML-4G string match on "excessive aliasing" stays only if still useful as an early exit; the budget is the
   control.
3. Tests: bomb alone; cycle-before-bomb; duplicate `<<`-before-bomb; bomb through each direct caller of `parse()`
   (`ParseRulesFromContent`, `ParseDirsFromContent`, `ReadNamespacingConfig`, `ReadAgentConventions`); all return
   within 2 s. Legitimate alias use keeps working.
**Acceptance criteria:**
- [x] nenhuma das 4 formas (bomba, ciclo antes, `<<` duplicado antes, chamador direto de `parse()`) passa de 2 s
- [x] `go test ./internal/config/` e `go test ./internal/generators/` rc=0

**Wave gates:**
```bash
test -s docs/qualidade/2026-11-22-onboarding-review.md
test -s docs/seguranca/2026-11-22-red-team-onboarding.md
test -s docs/seguranca/2026-11-22-red-team-onboarding-corretivos.md
TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 5m ./internal/generators/
TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 5m ./internal/commands/
TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 5m ./internal/config/
```

## Wave 5 — Fechamento
> Dependencies: Wave 4 audited.

### ML-FB — make quality e CI (zeus-tf)
**Status:** ⬜ Pending
**Acceptance criteria:**
- [ ] AC6: `make quality` rc=0 com a máquina ociosa; CI verde no PR

**Wave gates:**
```bash
go build ./...
```
