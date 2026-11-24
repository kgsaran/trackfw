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
- [ ] AC1: conceito aprovado pelo KG
- [ ] AC2: protótipo validado visualmente pelo KG e descartado
- [ ] AC3: ADR e waves de implementação escritos após a validação
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
**Status:** 🔄 Primeira rodada — `generate.sh` com o ambiente fixo, tapes `quickstart`/`guard` e o demo em `.webm`;
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
- [ ] `bash scripts/site-images/generate.sh` rc=0 e `bash scripts/check-site-images-leak.sh` rc=0
- [ ] contra-braço: canário com `/Users/` reprova o scanner
- [ ] `make parity-rest` rc=0 (script novo)

### ML-2B — Protótipo da home (afrodite-tf)
**Status:** 🔄 Rodada 1 entregue ao KG — `site/prototype.md` com hero, problema, fluxo, 3 cenários, board, vídeo com
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
- [ ] `npm run build` em `site/` rc=0; capturas headless desktop e mobile conferidas pelo arquiteto
- [ ] o KG valida visualmente (rodadas curtas até a aprovação)

**Wave gates:**
```bash
bash scripts/check-site-images-leak.sh
```

## Wave 3 — Implementation (Fase C)
> Dependencies: Wave 2 validated by the KG. ADR and microbatches written after validation.
