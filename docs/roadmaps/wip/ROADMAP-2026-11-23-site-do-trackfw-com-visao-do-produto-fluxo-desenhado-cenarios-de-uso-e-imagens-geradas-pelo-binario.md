---
status: wip
date: 2026-11-23
req: "docs/req/REQ-2026-11-23-site-do-trackfw-com-visao-do-produto-fluxo-desenhado-cenarios-de-uso-e-imagens-geradas-pelo-binario.md"
squad: ""
---

# Roadmap: Site do trackfw com visão do produto: fluxo desenhado, cenários de uso e imagens geradas pelo binário

> Created: 2026-11-23 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-11-23-site-do-trackfw-com-visao-do-produto-fluxo-desenhado-cenarios-de-uso-e-imagens-geradas-pelo-binario.md -->
REQ: docs/req/REQ-2026-11-23-site-do-trackfw-com-visao-do-produto-fluxo-desenhado-cenarios-de-uso-e-imagens-geradas-pelo-binario.md

## Acceptance Criteria
- [x] AC1: conceito aprovado pelo KG
- [ ] AC2: protótipo validado visualmente pelo KG e descartado
- [x] AC3: ADR e waves de implementação escritos após a validação
- [ ] AC4: site final en + pt-BR com imagens geradas por script, sem dado pessoal
- [ ] AC5: teste do Quick start verde; build do site rc=0; capturas headless
- [ ] AC6: `make quality` rc=0 e CI verde

## Status Legend
⬜ Pending · 🔄 In progress · ✅ Done · ❌ Blocked

## Wave 0 — Threat Model
> Dependencies: none. Runs in parallel with Wave 1 (different files); blocks Wave 2 (prototype code).

### ML-0A — Threat model do site novo (hades-tf)
**Status:** ✅ Done — `docs/seguranca/2026-11-23-wave0-site-novo-threat-model.md`. Seguro, medido com HOME isolado e
canários: stdout dos 8 comandos candidatos (só caminhos relativos e `~`), identidade git (não vaza para os arquivos
gerados), tokens (não aparecem), fontes (VitePress embute a Inter), `demo.gif` (16 quadros sem dado pessoal),
`/api/board` (só basenames). Riscos: (1) `/api/chain` do `trackfw serve` devolve o caminho absoluto de ADR global
(`~/...` expandido) no `id` — captura do painel de cadeia vazaria usuário e home; as imagens desta REQ não usam esse
painel, e o defeito do serve é outra causa; (2) script de imagem sem `PS1` fixo mostraria o cwd absoluto; (3) o job
`build` do `deploy-docs.yml` herda `pages: write` e `id-token: write` (conferido pelo arquiteto: permissões no nível
do workflow); (4) o `serve` carrega CDN sem SRI (pré-existente, outra causa). Controles adotados no ML-2A e no ML-3x:
ambiente fixo de geração, scanner de vazamento com canário, permissões por job, diagrama sem plugin de runtime.
**Files affected:** `docs/seguranca/2026-11-23-wave0-site-novo-threat-model.md` (novo; único arquivo);
`docs/req/REQ-2026-11-23-trackfw-serve-expoe-caminho-absoluto-de-adr-global-no-api-chain.md` e
`docs/roadmaps/backlog/ROADMAP-2026-11-23-trackfw-serve-expoe-caminho-absoluto-de-adr-global-no-api-chain.md` (REQ
do risco (1), aberta no backlog a pedido do KG; outra causa, outra REQ); `docs/roadmaps/.trackfw-log-entries/`
**Actions:**
1. Enumeration completeness: every surface the new site adds or changes — images and GIFs generated from the binary
   (what they can capture: home paths, usernames, tokens, hostnames, git remotes, dates), new npm dependencies
   (diagram plugins, image tooling), external assets (fonts, CDNs, embeds), the deploy workflow
   (`.github/workflows/deploy-docs.yml`) and its permissions.
2. Threat model: who leaks personal data or ships a malicious dependency through the site without breaking a written
   rule, and how.
3. Falsification targets in both directions: an image that leaks; a generator that over-redacts and shows nothing.
4. Declared residual.
**Acceptance criteria:**
- [x] The four sections above answered with evidence, not a one-line assertion
- [x] No implementation line written for this ML

**Wave gates:**
```bash
test -s docs/seguranca/2026-11-23-wave0-site-novo-threat-model.md
```

## Wave 1 — Concept (Fase B1)
> Dependencies: none. No code.

### ML-1A — Conceito do site novo (atena-tf)
**Status:** ✅ Done — `docs/visao-projeto/2026-11-23-site-novo-conceito.md`: home com problema, desenho do fluxo,
três cenários, cards atualizados e Quick start pinado; imagens por VHS (instalado), sem dependência npm nova.
**Decisões do KG (2026-11-23):** hero "Your AI coding agent ships fast. trackfw makes it ship traceably."; cenário 3
(npm/pip) como card; captura do board do `trackfw serve` já no protótipo; `demo.gif` vira vídeo com controles no site
(WCAG 2.2.2), e o README continua com o GIF. Decisão do arquiteto (Q4): no protótipo, o fluxo aponta para uma âncora;
a página "How it works" fica para a implementação. Correção do arquiteto ao conceito: o bloco "In 30 seconds" da home
já tem título em `adr new`/`req new` (ML-2A da REQ-2026-11-22); o problema que sobra nele é o `ship` sem branch.
**Files affected:** `docs/visao-projeto/2026-11-23-site-novo-conceito.md` (novo; único arquivo)
**Actions:** information architecture of the home and of the guide; the three usage scenarios (solo dev with an AI
agent, team with CI, company that only allows npm/pip), each with who, pain, what trackfw does, and the 3-5 commands
that show it; the drawn flow ADR → REQ → ROADMAP → waves → barrier → PR, as a text mockup; which images and GIFs, and
which binary command generates each; open questions for the KG. Benchmarks: 2-3 developer-tool sites, cited.
**Acceptance criteria:**
- [x] Conceito com mockups em texto e perguntas abertas; o KG aprova ou pede ajuste (AC1)

**Wave gates:**
```bash
test -s docs/visao-projeto/2026-11-23-site-novo-conceito.md
```

## Wave 2 — Prototype (Fase B2)
> Dependencies: Wave 0 audited and Wave 1 approved by the KG. One agent, sequential (ML-2B uses ML-2A's images).

### ML-2A — Gerador de imagens e scanner de vazamento (afrodite-tf)
**Status:** ✅ Done (rodada 1; pendências no ML-3A) — `generate.sh` com o ambiente fixo, tapes `quickstart`/`guard` e o demo em `.webm`;
`board.png` só do painel Kanban; scanner com 5 canários (`5 passed`) e varredura limpa (rodada também pelo arquiteto,
rc=0); `make parity-rest` rc=0. Pendências da auditoria: (1) o `quickstart.gif` termina com "1 violation" (um Enter
às cegas no wizard do `req new` cria um ADR draft); (2) o scanner lê os bytes com `strings`, e o texto que aparece
numa imagem é pixel: ele pega metadado, não o que o terminal mostra. O controle real é o ambiente fixo; a
verificação tem de ler a saída em texto do VHS (`Output *.txt`) de cada tape.
**Files affected:** `scripts/site-images/generate.sh` (novo); `scripts/site-images/*.tape` (novos);
`scripts/check-site-images-leak.sh` (novo); `site/public/img/` (gerados)
**Actions:**
1. `generate.sh` runs every VHS tape in a fixed environment: `HOME` and project in `mktemp -d`, `PS1='$ '`,
   `GIT_AUTHOR_NAME/EMAIL` and `GIT_COMMITTER_NAME/EMAIL` set to `Demo User`/`demo@example.invalid`, `TZ=UTC`,
   `GH_TOKEN`/`GITHUB_TOKEN` unset, binary taken from `bin/trackfw`. Outputs land in `site/public/img/`.
2. Images: terminal sessions for the scenarios and the Quick start; the `trackfw serve` board (Kanban panel only,
   never the chain panel); the demo as `.webm`/`.mp4` with the same script as `docs/demo.gif`.
3. `check-site-images-leak.sh` scans every generated file with `strings` for `/Users/`, `/home/`, `C:\Users\`,
   e-mail addresses (allowlist `example.invalid`), `ghp_`/`gho_`/`github_pat_`, `Bearer `; allowlist
   `github.com/kgsaran/trackfw`. Self-test with a canary that must fail and one that must pass.
**Acceptance criteria:**
- [x] `bash scripts/site-images/generate.sh` rc=0 e `bash scripts/check-site-images-leak.sh` rc=0
- [x] contra-braço: canário com `/Users/` reprova o scanner
- [x] `make parity-rest` rc=0 (script novo)

### ML-2B — Protótipo da home (afrodite-tf)
**Status:** ✅ Done — **aprovado pelo KG em 2026-11-23** na rodada 1 ("pode seguir, aprovado"). Ajustes do arquiteto no ML-3B. `site/prototype.md` com hero, problema, fluxo, 3 cenários, board, vídeo com
controles, features e Quick start (linhas idênticas ao README). Build rc=0; `TestQuickStart` rc=0. Notas do arquiteto
na captura desktop: o fluxo quebra o texto em nós estreitos; o vídeo e o GIF do Quick start mostram o primeiro quadro
vazio (falta pôster); o board aparece pequeno e com colunas quase vazias; o card do cenário 1 tem o GIF do guard
sem conteúdo visível.
**Files affected:** `site/prototype.md` (descartável; removido antes do PR final); `site/public/img/board.png`;
`site/public/img/demo.webm`; `site/public/img/guard.gif`; `site/public/img/quickstart.gif`
**Actions:** an isolated page, outside the nav and with `noindex`, implementing the concept's home in English with
the KG's decisions: hero, problem, drawn flow (static, no runtime plugin), three scenario cards, board screenshot,
demo video with controls, updated feature cards and the pinned Quick start. Build and headless screenshots at 1280 px
and 390 px.
**Acceptance criteria:**
- [x] `npm run build` em `site/` rc=0; capturas headless desktop e mobile conferidas pelo arquiteto
- [x] o KG valida visualmente (rodadas curtas até a aprovação)

**Wave gates:**
```bash
bash scripts/check-site-images-leak.sh
```

## Wave 3 — Implementation (Fase C) (2 agents in parallel)
> Dependencies: Wave 2 approved by the KG; ADR-2026-11-23 (site) D1–D8. afrodite-tf runs ML-3A then ML-3B
> (ML-3B uses ML-3A's images); ares-tf runs ML-3C in parallel (only `.github/workflows/deploy-docs.yml`).

### ML-3A — Imagens finais e scanner que lê texto (afrodite-tf)
**Status:** ✅ Done, com corretivo no ML-3D — transcrições `.txt` por tape; scanner lê transcrições e bytes, com
autoteste de 7 canários (F: transcrição com `/Users/x` reprova; G: mídia sem transcrição reprova); Quick start termina
com 0 violations e 2 warnings (conferido na transcrição); pôsteres do demo, do Quick start e do guard; board com as
6 colunas preenchidas a 2x. Gates do agente rc=0. Auditoria do arquiteto: a animação do Quick start mostra os comandos
com `< /dev/null` (e o `init` com `--identity-preset neutral < /dev/null > /dev/null 2>&1`), diferentes das linhas do
README que a página pina. ML-3D.
**Files affected:** `scripts/site-images/generate.sh`; `scripts/site-images/quickstart.tape`;
`scripts/site-images/guard.tape`; `scripts/check-site-images-leak.sh`; `site/public/img/board.png`;
`site/public/img/demo.webm`; `site/public/img/guard.gif`; `site/public/img/quickstart.gif`; `site/public/img/*.png`
(pôsteres, novos); `site/public/img/*.txt` (transcrições, novas)
**Actions:**
1. ADR D3: each tape also writes a text transcript (VHS `Output <name>.txt`); the scanner reads the transcripts and
   the bytes; self-test canaries cover a transcript with `/Users/x` (must fail).
2. `quickstart.tape` ends with the promised `validate` output (0 violations, 2 warnings): no blind Enter that picks a
   wizard option; drive the wizard deterministically or use non-TTY input.
3. Poster images (first meaningful frame) for the demo video and the Quick start animation.
4. Board fixture with roadmaps in every column, so the captured Kanban is readable; capture at a size that stays
   legible at the site width.
5. Guard animation shows the block message clearly at card size.
**Acceptance criteria:**
- [x] `generate.sh` e `check-site-images-leak.sh` rc=0; autoteste com canário de transcrição reprovando
- [x] transcrição do Quick start termina com 0 violations e 2 warnings
- [x] `make parity-rest` rc=0

### ML-3B — Home e guia em inglês e português; protótipo removido (afrodite-tf)
**Status:** ✅ Done, com corretivo no ML-3D — `site/index.md` e `site/pt/index.md` com a estrutura aprovada (layout
`doc`), fluxo em grade 2×3 no desktop e coluna no celular, vídeo com pôster e controles, animação do Quick start atrás
de `<details>`, Quick start literal; `site/prototype.md` removido. Build e `TestQuickStart` rc=0. Capturas do
arquiteto (inglês e português a 1280 px, inglês a 390 px): estrutura e traduções certas; o card do cenário 1 mostra o
primeiro quadro vazio do `guard.gif` (o pôster existe e não é usado ali). ML-3D.
**Files affected:** `site/index.md`; `site/pt/index.md`; `site/prototype.md` (removido);
`site/.vitepress/config.mts` (só se o layout da home exigir)
**Actions:**
1. ADR D1/D5: port the approved prototype into `site/index.md` (English) and `site/pt/index.md` (Portuguese, same
   structure). The "In 30 seconds" block is replaced by the pinned Quick start.
2. Fixes from the KG round: flow nodes wide enough for their text on desktop and stacked on mobile; posters on video
   and Quick start animation; board readable.
3. ADR D7: `<video controls>` with poster, no autoplay loop; transcript below.
4. Delete `site/prototype.md` (ADR D8).
5. `site/guide/getting-started.md` does not change (pinned by `TestQuickStartDocsPinned`).
**Acceptance criteria:**
- [x] `npm run build` rc=0; capturas headless de `/trackfw/` e `/trackfw/pt/` a 1280 e 390 px conferidas
- [x] `TestQuickStart*` rc=0; `site/prototype.md` não existe

### ML-3D — Corretivo do ML-3A/3B (afrodite-tf)
**Status:** ✅ Done — função `trackfw` no setup escondido do tape (stdin sem TTY; saída do `init` descartada); as
linhas digitadas são as do README (diff vazio, conferido pelo arquiteto), e a transcrição termina com 2 warnings. Card
do cenário 1 com `guard-poster.png` e a animação atrás de `<details>` (captura conferida). O gerador regenerou também
`guard.gif`, `demo.webm` e as transcrições (variação de data). Scanner: 10 arquivos limpos.
**Files affected:** `scripts/site-images/quickstart.tape`; `scripts/site-images/generate.sh`; `site/index.md`;
`site/pt/index.md`; `site/public/img/quickstart.gif`; `site/public/img/quickstart.txt`;
`site/public/img/quickstart-poster.png`
**Actions:**
1. The Quick start animation types exactly the README lines. Non-TTY input and the identity preset go into the hidden
   setup (for example a shell function `trackfw() { command trackfw "$@" </dev/null; }` and the preset in the fixture),
   not into the visible commands. The transcript still ends with 0 violations and 2 warnings.
2. Scenario 1 card shows `guard-poster.png`, with the animation behind `<details>`, as in the Quick start.
**Acceptance criteria:**
- [x] cada linha de comando da transcrição do Quick start é igual à do README; transcrição termina com 2 warnings
- [x] scanner e build rc=0; captura do card do cenário 1 com a mensagem de bloqueio visível

### ML-3C — Permissões por job no deploy do site (ares-tf)
**Status:** ✅ Done — bloco `permissions` do nível do workflow removido; `build`: `contents: read`; `deploy`:
`pages: write`, `id-token: write` (sem checkout, não precisa de `contents`). `actionlint` rc=0 (rodado também pelo
arquiteto); `check-required-status-checks.py --scope dw` rc=0. O efeito real só se confirma no deploy depois do merge.
**Files affected:** `.github/workflows/deploy-docs.yml`
**Actions:** ADR D6: move permissions from workflow level to job level — `build`: `contents: read`; `deploy`:
`pages: write`, `id-token: write`. Keep actions pinned as they are today. `actionlint` if available.
**Acceptance criteria:**
- [x] nenhuma permissão no nível do workflow; `build` só com `contents: read`
- [x] `python3 scripts/check-required-status-checks.py --scope dw` rc=0 (o nome dos jobs não muda)

**Wave gates:**
```bash
bash scripts/check-site-images-leak.sh
test ! -e site/prototype.md
TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 5m -run 'TestQuickStart' ./internal/commands/
```

## Wave 4 — Review and red team (2 MLs in parallel)
> Dependencies: Wave 3 audited.

### ML-4A — Revisão de qualidade (hefesto-tf)
**Status:** ✅ Done — APROVA com ressalvas, 0 bloqueante. (1) `generate.sh:38` tem o caminho do Chrome com
`/Users/kgsaran/...` no código versionado, o que expõe o usuário no repositório e só funciona nesta máquina; (2) o
canário G do scanner é vazio (não exercita o laço principal); (3) ícones decorativos sem `aria-hidden` (8 ocorrências).
Paridade en/pt, D6, D7 e D8 conferidos; imagens ~1,5 MB. ML-4C.
**Files affected:** `docs/qualidade/2026-11-23-site-novo-review.md`
**Acceptance criteria:**
- [x] Parecer escrito; achados bloqueantes corrigidos por ML corretivo

### ML-4B — Red team (hades-tf)
**Status:** ✅ Done — APROVA com ressalvas, 0 bloqueante. Nenhum dado pessoal nas imagens versionadas (bytes,
transcrições, 4 pôsteres e 19 quadros do `guard.gif` extraídos com ffmpeg); D6 e D7 conferidos. Para corrigir: canário G
vazio (teste de mutação confirma), pre-flight do `generate.sh` antes do isolamento (linha 74), `USER`/`LOGNAME` não
fixados, caminho do Chrome com o usuário, `persist-credentials` no checkout do deploy. Residuais: transcrição e GIF
são vistas diferentes; scanner sem hostname e sem `sk-`/`AKIA`/`xoxb-`; `board.png` sem transcrição; ações sem pin
por SHA. O hades não conseguiu rodar o VHS com canários no ambiente (3 tentativas); a ausência foi inferida por grep
e inspeção visual, com o usuário real como canário natural. ML-4C e ML-4D.
**Files affected:** `docs/seguranca/2026-11-23-red-team-site-novo.md`
**Acceptance criteria:**
- [x] Falsifica D2, D3 e D6 a partir do contrato (vazamento em imagem, scanner, permissões do deploy); residual declarado

### ML-4C — Corretivo da Wave 4 nos scripts e na home (afrodite-tf)
**Status:** ✅ Done — o Chrome é descoberto em tempo de execução (cache do Playwright no `$HOME`, antes do isolamento;
depois o PATH); nenhum `kgsaran` fora de `github.com/kgsaran` nos scripts (conferido pelo arquiteto). Pre-flight depois
do isolamento; `USER`/`LOGNAME=demo`. Scanner: `scan_dir` extraído, canário G chama a varredura real (mutação do laço
principal → rc=1), padrões `sk-`, `AKIA`, `xox[abp]-`, hostname e usuário da máquina (`id -un`). `guard.tape` com
altura 560 (transcrição com `exit=`). `aria-hidden` nos 8 ícones. Gates rc=0. Notas do arquiteto: o
`guard-poster.png` veio de uma rodada de teste da afrodite, porque o `Screenshot` do VHS não sobrescreveu o arquivo
no `generate.sh` completo (residual: o pôster não é reproduzível por esse caminho); o VHS deixou 6 processos do Chrome
vivos, encerrados pelo arquiteto.
**Files affected:** `scripts/site-images/generate.sh`; `scripts/site-images/guard.tape`;
`scripts/check-site-images-leak.sh`; `site/index.md`; `site/pt/index.md`
**Actions:**
1. Chrome: no personal path in the versioned script. `CHROME` comes from the environment; otherwise discover the
   Playwright headless shell under `$HOME/Library/Caches/ms-playwright/chromium_headless_shell-*/` (newest) or a
   `chrome-headless-shell`/`chromium` on PATH; otherwise warn and skip the board, as today.
2. Isolation before anything runs: the pre-flight (line ~74) moves after the isolated environment; `USER`, `LOGNAME`
   fixed to `demo` inside the isolated block.
3. Scanner: canary G runs the real main scan over a temp dir with a media file and no transcript and expects rc=1;
   patterns also for `sk-`, `AKIA[0-9A-Z]{16}`, `xox[abp]-`, and the current `hostname` and `$USER` of the machine
   running the scan (dynamic, skipped when they equal the demo values); a mutation of the main loop must make the
   self-test fail.
4. `guard.tape`: height enough for the transcript to include `exit=`.
5. Decorative icons with `aria-hidden="true"` in both homes.
6. Do not regenerate the versioned images unless needed for item 4; if `guard.*` is regenerated, run the scanner.
**Acceptance criteria:**
- [x] `grep -rn "/Users/" scripts/site-images scripts/check-site-images-leak.sh` só em canários do autoteste
- [x] autoteste do scanner reprova com o laço principal removido (mutação medida); `make parity-rest` rc=0
- [x] build do site rc=0

### ML-4D — Checkout do deploy sem credencial persistida (ares-tf)
**Status:** ✅ Done — `persist-credentials: false` no `actions/checkout` do `build`; nenhum step posterior usa git
autenticado. `actionlint` rc=0 (rodado também pelo arquiteto); check de required checks rc=0.
**Files affected:** `.github/workflows/deploy-docs.yml`
**Actions:** `persist-credentials: false` on the `actions/checkout` step of `build`; nothing else changes.
**Acceptance criteria:**
- [x] `actionlint` rc=0; `python3 scripts/check-required-status-checks.py --scope dw` rc=0

**Wave gates:**
```bash
test -s docs/qualidade/2026-11-23-site-novo-review.md
test -s docs/seguranca/2026-11-23-red-team-site-novo.md
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
