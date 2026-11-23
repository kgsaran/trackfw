# Auditoria de Onboarding — trackfw v11.8.1

**Data:** 2026-11-22  
**Auditora:** Atena (UX)  
**Binário auditado:** `/Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw/bin/trackfw` (v11.8.1)  
**Superfícies auditadas:** `README.md` (1285 linhas), `site/en/guide/getting-started.md`, `site/guide/getting-started.md` (PT-BR), `site/en/guide/commands.md`, `site/guide/commands.md`, `site/en/guide/ai-agents.md`, `site/index.md`, `site/.vitepress/config.mts`  
**Isolamento:** todos os comandos executados via `/bin/bash --noprofile --norc` em `mktemp -d` com `HOME`, `USERPROFILE`, `CODEX_HOME`, `XDG_CONFIG_HOME` apontando para subdiretório do temporário. Todos os comandos binários rodaram sem TTY (não-interativo); os caminhos de wizard TTY são inferidos do código-fonte e indicados como "não medido". **Sessão iniciada:** 2026-11-22 16:57 (data/hora do subagent .jsonl). **Verificação de home ao final:** 6 caminhos verificados sem filtro de nome (`~/.claude/agents`, `~/.claude/skills`, `~/.codex/agents`, `~/.agents/skills`, `~/.trackfw`, `~/.gemini`) com janela de 24 h. Todos os arquivos com mtime de 2026-11-21 (anterior à sessão): `~/.claude/skills/trackfw/SKILL.md` (17:53:34), `~/.trackfw/integrations-manifest.json` (20:14:13) — provável de um `trackfw init` do usuário no dia anterior; `~/.gemini/` (17:50–17:53) — atividade do Gemini CLI/Antigravity do usuário. Nenhum arquivo de 2026-11-22 encontrado. `git status --porcelain`: apenas `?? docs/qualidade/2026-11-22-auditoria-onboarding.md`. Temporários removidos ao final.

---

## A. Percurso real do usuário novo — Quick start README

Cada comando do bloco Quick start (README:21–28) executado via `/bin/bash --noprofile --norc` em ambiente isolado.

### A.1 — Sequência executada (README Quick start)

Bloco README:21–28 (5 comandos), seguido por "Quick start, continued" README:434–449 (3 comandos):

| Passo | Comando | RC | Resultado | Bate com README? |
|---|---|---|---|---|
| 1 | `trackfw init --ai-tools claude --forge github` | 0 | Estrutura criada, 30 itens em `$HOME/.claude/` e `$HOME/.trackfw/` isolado | Sim |
| 2 | `trackfw adr new "User Authentication Strategy"` | 0 | ADR criado em `docs/adr/` | Sim |
| 3 | `trackfw req new "User Authentication" --adr ADR-...` | 0 | REQ + roadmap automático em `backlog/` | Sim |
| 4 | `trackfw roadmap move ROADMAP-...-user-authentication.md wip` | 0 | Roadmap movido para `wip/` | Sim |
| 5 | `trackfw validate` | 0 | **"2 warning(s)"** — NÃO "✓ No violations found." | **NÃO** (ver A.2) |
| — | *(passo 0: `npm install -g trackfw` não testado — constraint de isolamento)* | — | — | — |
| C1 | `trackfw status` (continued) | 0 | Inventário com 1 ADR, 1 REQ, 1 roadmap | Sim |
| C2 | `trackfw ship -m "..."` (continued) | **1** | "cannot run on 'main' — use a feature branch" | **NÃO** (ver B.18) |
| C3 | `trackfw serve` (continued) | 0 | 200 em localhost:4080 | Sim |

**Todos os 5 comandos do bloco README:21–28 retornaram RC 0.** A "Quick start, continued" falha em C2 (`ship` em main).

### A.2 — Passo 5: validate não entrega a saída prometida (atrito)

README:28 promete: `# ✓ No violations found.`

Saída real:
```
⚠  adr_req_has_content: "docs/adr/ADR-2026-11-22-user-authentication-strategy.md" has mandatory
   section(s) with only model content: ## Context, ## Decision, ## Consequences,
   ## Alternatives Considered — fill in or set rules.adr_req_has_content: off
⚠  adr_req_has_content: "docs/req/REQ-2026-11-22-user-authentication.md" has mandatory
   section(s) with only model content: ## Motivation, ## Acceptance Criteria — fill in or
   set rules.adr_req_has_content: off
2 warning(s)
```

A regra `adr_req_has_content` fica ativa por padrão e detecta seções com apenas conteúdo de template. O Quick start cria artefatos com `adr new "Título"` e `req new "Título"`, mas não instrui o usuário a preencher o conteúdo antes de rodar `validate`. Medido sem TTY (não-interativo). No caminho TTY (não medido — requer terminal real), `adr.go` prompts "Context, Decision, Consequences, Alternatives Considered" e `req.go` prompts "Motivation, Acceptance Criteria" (inferido de `internal/commands/req.go:124,142`). Se o usuário preenche todos os campos, os warnings não aparecem. O "✓ No violations found." na linha do Quick start é defensável para TTY com wizard completo; é enganoso para não-TTY e para usuários que saltam os prompts do wizard.

Nota: `internal/commands/quickstart_e2e_test.go` (linhas 127–157) **sabe que validate retorna 2 warnings** nesse fluxo — asserta explicitamente `len(warnings) == 2`. O comentário `# ✓ No violations found.` no README está desalinhado do comportamento que o próprio teste documenta.

### A.3 — Interatividade: `adr new` e `req new` sem título falham (bloqueante no caminho do site)

`trackfw adr new` sem título → RC 1: `Error: accepts 1 arg(s), received 0`  
`trackfw req new` sem título → RC 1: `Error: accepts 1 arg(s), received 0`

O README Quick start passa o título como argumento e funciona. O site (`site/en/guide/getting-started.md:110,170` e `site/index.md:57–60`) mostra os comandos sem título — esses falham com RC 1 (ver seção B.1).

Nota sobre `init`: `init </dev/null` (sem flags) retorna RC 0 com defaults silenciosos. O binário detecta ausência de TTY e pula os prompts. Com TTY real, o wizard abre. O README (linha 31): "In a terminal, `init`, `adr new` and `req new` open short wizards." — a afirmação é correta para o caso com TTY.

### A.4 — Passo 3: `req new` cria roadmap automaticamente (atrito no site)

`req new "Título" --adr file.md` cria tanto a REQ quanto um roadmap em `backlog/`. O site (passo 4) instrui o usuário a executar adicionalmente `roadmap new --from-req ...`. Resultado: `roadmap ROADMAP-... already exists — use --force to recreate`. RC 0, mas nada é criado e o usuário não sabe o que aconteceu. Ver B.8.

### A.5 — `roadmap move` com prefixo REQ falha no fluxo do site

`trackfw roadmap move ROADMAP-$(date +%F)-user-authentication.md wip` — funciona (RC 0).  
`trackfw roadmap move ROADMAP-$(date +%F)-user-authentication done` — encontra o arquivo mas falha por bloqueio de governance (ML-0A pendente). Ver B.11.  
`trackfw roadmap move REQ-$(date +%F)-user-authentication wip` — RC 1: "not found in any state directory". O site (line 246) usa o prefixo REQ, não ROADMAP — isso falha. Ver B.2.

---

## B. Divergências entre README e site

### B.1 — BLOQUEANTE: `adr new` e `req new` sem título falham no site

**Arquivo:** `site/en/guide/getting-started.md:110,170` / `site/guide/getting-started.md:112,172` / `site/index.md:57–58`

As páginas exibem:
```bash
trackfw adr new    # passo 2
trackfw req new    # passo 3
```
e `site/index.md:57–58`:
```bash
trackfw adr new       # nova decisão arquitetural
trackfw req new       # novo requisito
```

**Medido:** ambos os comandos requerem `<title>` como argumento posicional obrigatório. Executar sem título → RC 1.

```
Error: accepts 1 arg(s), received 0
```

Um usuário seguindo o guia literal não consegue passar do passo 2.  
**Classe:** bloqueante

### B.2 — BLOQUEANTE: `roadmap move` com prefixo REQ no site falha

**Arquivo:** `site/en/guide/getting-started.md:246` / `site/guide/getting-started.md:248`

Texto:
```bash
trackfw roadmap move REQ-2026-06-13-oauth-login wip
```

**Medido:** o arquivo de roadmap tem prefixo `ROADMAP-`, não `REQ-`. Usar o prefixo REQ retorna RC 1: `Error: roadmap "REQ-..." not found in any state directory`.  
**Classe:** bloqueante

### B.3 — Estrutura gerada por `init` diverge do site

**Arquivo:** `site/en/guide/getting-started.md:68–92` / `site/guide/getting-started.md:70–93`

Comparação do que o site documenta vs o que o binário realmente cria:

| Item documentado no site | Realmente criado? | Observação |
|---|---|---|
| `docs/adr/`, `docs/req/` | ✅ | |
| `docs/roadmaps/backlog/` | ✅ | |
| `docs/roadmaps/wip/` | ✅ | |
| `docs/roadmaps/blocked/` | ✅ | |
| `docs/roadmaps/done/` | ✅ | |
| `docs/roadmaps/abandoned/` | ✅ | |
| `docs/visao-projeto/` | **NÃO** | nunca criado |
| `docs/agents-working-context.md` | **NÃO** | nunca criado |
| `trackfw.yaml` | ✅ | |
| `scripts/trackfw-validate.sh` | ✅ | |
| `CLAUDE.md` *(if selected)* | criado **sempre** | mesmo com `--ai-tools codex` |
| `.claude/commands/` *(7 slash commands)* | ✅, mas são **9** | B.4 |
| `AGENTS.md` *(if codex selected)*, `.codex/agents/`, `.agents/skills/` | `.codex/agents/` e `.agents/skills/` vão para `~/`, não no projeto | default `--scope global`; `AGENTS.md` fica no projeto |

**Ausente da documentação, mas criado:**
- `docs/roadmaps/analyzing/` (documentado em commands.md:394 mas omitido da estrutura)
- `vault/notes/` e `vault/notes/index.md`
- `scripts/trackfw-attention-signal.sh`
- `scripts/trackfw-attention-cleanup.sh`
- `scripts/trackfw-credential-guard.sh`
- `scripts/trackfw-git-branch-guard.sh`
- `.claude/settings.json`

**Para `--ai-tools claude`:** 12 agents e 17 skills vão para `~/.claude/agents/` e `~/.claude/skills/` — ver B.17.  
**Classe:** atrito

### B.4 — Contagem errada de slash commands

**Arquivo:** `site/en/guide/commands.md:28` / `site/guide/commands.md:28`  
**Texto:** `— 7 slash commands for Claude Code`  
**Real:** 9 (adr, architect, barrier, implement, move, req, roadmap, status, validate) — confirmado pelo output do `init`: `✓ .claude/commands/trackfw (9 slash commands)` e por contagem direta.  
**Classe:** atrito

### B.5 — Output de `trackfw status` diverge significativamente

**Arquivo:** `site/en/guide/getting-started.md:264–274` / PT-BR equivalente

**Texto prometido no site:**
```
trackfw — project status

📋 Backlog       2 roadmaps
🔄 WIP           1 roadmap
❌ Blocked       0 roadmaps
✅ Done          3 roadmaps

📄 ADRs          4   (Proposed: 2, Accepted: 1, Draft: 1)
📝 REQs          3   (Open: 2, Closed: 1)
```

**Saída real (11.8.1):**
```
── trackfw status ──────────────────────

📊 Inventory
   ADRs        1
   REQs        1  (1 Open · 0 Done · 0 Closed)
   Roadmaps    1
     backlog 0 · analyzing 0 · wip 1
     blocked 0 · done 0 · abandoned 0

🔄 WIP (1)
   ROADMAP-2026-11-22-user-authentication.md

❌ Blocked (0)

✅ Done (last 5)

────────────────────────────────────────
```

Diferenças: cabeçalho diferente; ADRs/REQs agrupados em `📊 Inventory` (não seções separadas); Backlog não aparece como seção top-level; breakdown por status de ADR ausente.  
**Classe:** atrito

### B.6 — Formato do output de `trackfw validate` não corresponde

**Arquivo:** `site/en/guide/getting-started.md:285–296`  
**Texto:** `[violation] REQ-... is blocked by Draft ADR: ...`  
**Real (medido com roadmap sem REQ):**
```
✗ roadmap "ROADMAP-test.md" is in wip but has no linked REQ ...
Error: 3 violation(s) found
```
O prefixo `[violation]` não existe. As violações aparecem diretamente com `✗`. O texto final é `Error: N violation(s) found`, não `✗ N violation(s) found:`.  
**Classe:** cosmético

### B.7 — `req new` retorna saída diferente do prometido no site

**Arquivo:** `site/en/guide/getting-started.md:188–189`  
**Texto:** `Next step: resolve Draft ADRs before creating the roadmap.`  
**Medido:** o binário não emite essa linha. Com `--no-roadmap`: `--no-roadmap: REQ-... has no roadmap yet. Link one with: trackfw roadmap new --from-req ...`  
**Verificado:** `grep -rn "resolve Draft ADRs" internal/commands/` não retorna nenhum resultado.  
**Classe:** cosmético

### B.8 — `roadmap new` seguido de `req new` cria conflito não documentado

**Arquivo:** `site/en/guide/getting-started.md:208–210`

O site mostra como passo 4: `trackfw roadmap new --from-req docs/req/...`  
Porém o passo 3 (`req new`) já criou um roadmap automaticamente. Executar `roadmap new --from-req <mesma REQ>` retorna:
```
roadmap ROADMAP-... already exists at docs/roadmaps/wip/... — nothing overwritten; use --force to recreate
```
RC 0. O usuário não recebe feedback claro explicando por que o roadmap "already exists".  
**Classe:** atrito

### B.9 — BLOQUEANTE (defect de produto): `--brownfield` ignorado em modo não-TTY

**Arquivo:** `site/en/guide/commands.md:19`, `site/en/guide/getting-started.md:98–102`  
**Documentado:** "Activates lenient mode for 30 days (violations become warnings)"

**Repro medido (não-TTY):**

```bash
# Em dir isolado com git init (executado em /bin/bash --noprofile --norc):
trackfw init --brownfield --ai-tools claude
# → ✓ trackfw initialized ...

grep -c 'governance_mode\|lenient_until' trackfw.yaml
# → 0  (campos ausentes; grep RC 1 = 0 matches)

# Criar roadmap inválido em wip/ e validar:
trackfw validate
# ✗ roadmap "ROADMAP-test-noreq.md" is in wip but has no linked REQ ...
# ✗ roadmap "ROADMAP-test-noreq.md" is in wip but has no acceptance criteria block
# ✗ roadmap "ROADMAP-test-noreq.md" (wip) has no ## Wave 0 heading ...
# Error: 3 violation(s) found
# RC: 1  (medido)

# Adicionar manualmente governance_mode: lenient + lenient_until: 2027-11-22:
trackfw validate
# [LENIENT MODE] Governance violations treated as warnings until 2027-11-22
# ⚠  ... (warnings, não erros)
# RC: 0  (medido)
```

**Causa:** `internal/commands/init.go:266` — o bloco não-TTY usa defaults e chama `executeScaffoldAndInstall` retornando na linha 309, antes de chegar à leitura do flag `--brownfield` em `init.go:531`. O bloco TTY lê o flag em `:531`, seta `cfg.BrownfieldMode = true` em `:550`, e `scaffold.go:1083–1084` escreve os campos ao yaml. O mecanismo de lenient mode no validate está correto; o defect é que o flag não chega ao scaffold no caminho não-TTY.

Usuários adotando brownfield em CI ou via script (o caso mais comum para projetos legados) não recebem o lenient mode prometido. O caminho TTY é provavelmente funcional, mas não foi medido nesta auditoria (requer terminal real).  
**Classe:** bloqueante para adoção não-TTY (defect de produto — routing para owner de `internal/`)

### B.10 — `branch new chore` falha sem remote

**Arquivo:** `site/en/guide/commands.md:118–121`  
**Texto:** "`chore`, `docs` — Exempt from the roadmap gate — the branch is created normally"  
**Medido:** `trackfw branch new chore/cleanup` sem remote configurado → RC 1:
```
trackfw branch new: no 'origin' remote found — use --from HEAD to create from the current HEAD, or add a remote with 'git remote add origin <url>'.
```
O site não menciona a dependência de remote.  
**Classe:** atrito

### B.11 — Wave 0 — Threat Model gerado no roadmap não está no exemplo do site

**Arquivo:** `site/en/guide/getting-started.md:223–238`  
O site mostra o roadmap gerado com `## Wave 1 — ...`. O roadmap real começa com:
```markdown
## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** ⬜ Pending
```
Essa Wave 0 tem um ML pendente que bloqueia `roadmap move ... done` até ser marcado `✅`. Um usuário tentando mover para `done` recebe `Error: 1 blocker(s): ML-0A`.  
**Classe:** atrito

### B.12 — Versão desatualizada nos dois idiomas do site

**Arquivo:** `site/en/guide/getting-started.md:44` / `site/guide/getting-started.md:44`  
**Texto:** `# trackfw 11.1.0`  
**Real:** `trackfw 11.8.1`  
**Classe:** cosmético

### B.13 — Texto final de `init` não corresponde

**Arquivo:** `site/en/guide/commands.md:46`  
**Texto:** `✓ Governance structure initialized.`  
**Real:** `✓ trackfw initialized — run 'trackfw status' to see your governance state.`  
**Classe:** cosmético

### B.14 — `--scope` não documentado na seção init do site

**Arquivo:** `site/en/guide/commands.md:16–21`  
Flags documentadas: `--brownfield`, `--ai-tools`. Flag ausente: `--scope project|global` (presente no `init --help`). Ver B.17 para o impacto concreto do default global.  
**Classe:** cosmético

### B.15 — Âncora quebrada: site linka `README#windows` que não existe

**Arquivo:** `site/en/guide/getting-started.md:304` / `site/guide/getting-started.md:304`  
**Texto:** `[trackfw README on GitHub](https://github.com/kgsaran/trackfw#windows)`  
**Real:** o README não tem heading `## Windows`. O heading é `### Windows support (partial)` (linha 94), que gera âncora `#windows-support-partial` no GitHub. O link `#windows` aponta para o topo da página.  
**Classe:** cosmético

### B.16 — Termos PT-BR em seção de EN do site (commands.md)

**Arquivo:** `site/en/guide/commands.md:581–582`  
Tabela do `trackfw barrier` usa `**Critérios de aceite:**` e `**Gates da wave:**` em página em inglês. Esses são os campos exatos que o binário verifica no roadmap — mas sua aparição em PT em contexto EN é inconsistente.  
**Classe:** cosmético

### B.17 — `init` escreve 30 itens em `~/` por padrão — Quick start e seção de onboarding não mencionam

**Arquivos:** `README.md:21–28` / `site/en/guide/commands.md:22–32`

`trackfw init --ai-tools claude` (default `--scope global`) escreve na home:
- 12 arquivos em `~/.claude/agents/`
- 17 arquivos em `~/.claude/skills/*/SKILL.md`
- 1 arquivo `~/.trackfw/integrations-manifest.json`

Total: 30 arquivos. O `init` anuncia 29 destinos no stdout ("Destination (global): …" lista agents + skills). O `~/.trackfw/integrations-manifest.json` é escrito silenciosamente sem anúncio.

O README menciona `~/.claude/` apenas nas seções de referência (linhas 824, 840, 846) — fora do Quick start e do onboarding. O site (`site/en/guide/commands.md:22–32`, "What gets generated") não lista `~/.claude/agents/` ou `~/.claude/skills/` entre os arquivos gerados pelo `init --ai-tools claude`; lista apenas caminhos do projeto.

Atrito real: o usuário sabe dos 29 (anunciados no stdout), mas não sabe do `integrations-manifest.json` nem que `--scope project` evitaria essas escritas na home.  
**Classe:** atrito

### B.18 — `trackfw ship` falha em `main` (Quick start, continued)

**Arquivo:** `README.md:442–444`

O "Quick start, continued" (README:434–449) inclui `trackfw ship -m "..."` com a nota inline "(feat/fix/refactor branches only)" em README:442. A nota existe mas é discreta; `ship` em `main` retorna RC 1:

```
Error: trackfw ship cannot run on "main" — use a feature branch:
  trackfw branch new feat/<slug>
```

A seção não instrui o usuário a criar uma branch antes de `ship`. O erro sugere `trackfw branch new feat/<slug>` — sem `--from HEAD`. Sem `origin`, esse `branch new` falha também (B.10, medido com `chore`; `feat` não medido, inferido por mesmo mecanismo). `branch new feat/... --from HEAD` retornou RC 0 em repo sem commits (medido), mas `ship` requer push — sem `origin`, o chain não completa de qualquer forma. Com `origin`, o chain é funcional mas não foi medido.  
**Classe:** atrito

### B.19 — Âncoras auto-referenciais do README apontam para parágrafos em negrito

**Arquivo:** `README.md:195, 223, 226`

Três links internos do README usam anchors que não existem no GitHub porque os alvos são parágrafos em negrito (`**Texto**`), não headings:

| Linha | Link | Alvo real (linha) | Tipo do alvo |
|---|---|---|---|
| 195 | `#pitfall-more-than-one-trackfw-on-path` | Bold `:178` | parágrafo em negrito |
| 226 | `#pitfall-more-than-one-trackfw-on-path` | Bold `:178` | parágrafo em negrito |
| 223 | `#guard-hooks-on-windows--measured-state-since-v930` | Bold `:249` | parágrafo em negrito |

GitHub gera anchors apenas para headings (`#`, `##`, etc.). Links para bold paragraphs navegam para o topo da página.  
**Classe:** cosmético

---

## C. Estrutura e carga de leitura

### C.1 — Quick start fragmentado: 415 linhas de gap

O README tem duas seções de Quick start:
- `## Quick start` (linha 19–37): primeiro `validate` está na linha 28 — o usuário chega a ele após ~8 linhas de Quick start, se seguir o bloco.
- `## Quick start, continued` (linha 434–449): 16 linhas adicionais com `ship` e `serve`. O link "Next: Quick start, continued" em README:34 permite saltar direto; a nota ":443" instrui o usuário a usar uma branch de feature, mas `ship` em `main` falha com RC 1 (B.18).

**Gap entre os dois blocos: 415 linhas** (linhas 38–433):

| Seção | Linhas | Conteúdo |
|---|---|---|
| `## The problem` | 40–54 | 15 linhas — motivação do produto |
| `## Installation` (Homebrew, npm) | 55–93 | 39 linhas |
| `### Windows support (partial)` | 94–411 | **318 linhas** |
| `### pip` (continua Installation) | 412–433 | 22 linhas |

Um usuário macOS/Linux que role em vez de clicar no link "Next" encontra 318 linhas sobre Windows antes de chegar ao segundo bloco de onboarding.

**Linhas até o primeiro `validate` (fluxo README, seguindo o bloco Quick start):**  
README:28 — ~8 linhas de leitura efetiva. Resultado: 2 warnings em vez de zero (ver A.2 — depende de TTY). Se o usuário clica "Next", chega ao `ship` em mais ~16 linhas — onde ship falha em main (B.18; a nota está em README:442).  
**Linhas se rolando linearmente até `ship`:** ~430 linhas.

**Linhas até o primeiro `validate` verde (fluxo do site EN):**  
`site/en/guide/getting-started.md` tem 312 linhas total. O `validate` aparece na linha 279. O usuário para na linha 110 com RC 1 (B.1) — nunca alcança o `validate`.  
**Classe:** atrito

### C.2 — 318 linhas de Windows dentro de Installation interrompem macOS/Linux

O bloco `### Windows support (partial)` é subseção de `## Installation`. Interrompe a continuidade entre npm (linha 69) e pip (linha 412). Um usuário macOS/Linux procurando "pip" precisa pular 318 linhas de Windows.

**Referência:** 1285 linhas totais do README. Material de referência avançada começa na linha 505 (`## Governance chain`) e vai até 1283 — 779 linhas que não são onboarding.  
**Classe:** atrito

### C.3 — Site não tem testes automatizados; README tem

O README Quick start é pinado pelo teste `internal/commands/quickstart_e2e_test.go`. O site não tem nenhum equivalente. Achados de divergência de output (B.5, B.6, B.7, B.12, B.13) e de caminho (B.1, B.2, B.3, B.4, B.8) decorrem dessa ausência. Exceções: B.9 é um defect binário no caminho não-TTY; B.10 é uma dependência de ambiente não coberta por testes.  
**Classe:** atrito (relevante para proposta P4)

---

## D. Idioma

### D.1 — Root do site é PT-BR; produto é "inglês primeiro" desde 2026-11-02

**Arquivo:** `site/.vitepress/config.mts:16`

```typescript
locales: {
  root: { label: 'Português', lang: 'pt-BR' }
  en:   { label: 'English',   lang: 'en-US',  link: '/en/' }
}
```

A landing page padrão e o CTA "Começar agora" levam a conteúdo PT-BR. Um usuário chegando do npm, GitHub ou busca vê PT-BR. Não há detecção de idioma por `Accept-Language`.

As meta `og:*` tags no config são em PT-BR (`og:title: "trackfw — Governança para times..."`, `og:description: "CLI open-source para governança..."`), afetando previews em redes sociais para usuários EN.  
**Classe:** atrito

### D.2 — Arquivos gerados pelo binário estão em inglês; PT-BR do site mostra nomes PT

**Arquivo:** `site/guide/getting-started.md:130–152` (PT-BR)

O site PT-BR mostra seções do ADR gerado como `## Contexto`, `## Decisão`, `## Consequências`, `## Alternativas consideradas`. O arquivo real gerado pelo binário usa `## Context`, `## Decision`, `## Consequences`, `## Alternatives Considered`.

Mesmo divergência no REQ: site PT-BR mostra `## Motivação`, `## Critérios de aceite`; arquivo real tem `## Motivation`, `## Acceptance Criteria`.

Consequência prática: a regra `adr_req_has_content` menciona os nomes reais das seções (`## Context`) nas mensagens de warning. Um usuário lendo a PT-BR page esperaria nomes PT-BR.  
**Classe:** atrito (impacto direto no diagnóstico de warnings do validate)

---

## Tabela-resumo de achados

| ID | Classe | Superfície | Arquivo:Linha | Evidência |
|---|---|---|---|---|
| B.1 | **bloqueante** | Site EN+PT, home | site/en/guide/getting-started.md:110,170 / site/index.md:57–58 | `adr new` e `req new` sem título → RC 1 |
| B.2 | **bloqueante** | Site EN+PT | site/en/guide/getting-started.md:246 | `roadmap move REQ-...` → RC 1 "not found" |
| B.9 | **bloqueante** (defect de produto) | Site EN+PT | site/en/guide/commands.md:19 / internal/commands/init.go:266,531 | `--brownfield` ignorado em não-TTY; yaml sem governance_mode; validate RC 1 — fix em `internal/` |
| A.2 | atrito | README | README.md:28 | validate retorna "2 warning(s)" em vez de "✓ No violations found." |
| B.3 | atrito | Site EN+PT | site/en/guide/getting-started.md:68–92 | `docs/visao-projeto/` e `agents-working-context.md` não criados; 7 itens reais ausentes |
| B.4 | atrito | Site EN+PT | site/en/guide/commands.md:28 | "7 slash commands" vs 9 reais |
| B.5 | atrito | Site EN+PT | site/en/guide/getting-started.md:264–274 | Output de `status` difere (header, grouping, ADR breakdown) |
| B.8 | atrito | Site EN+PT | site/en/guide/getting-started.md:208–210 | `roadmap new --from-req` após `req new` já existente → "already exists" |
| B.10 | atrito | Site EN | site/en/guide/commands.md:118–121 | `branch new chore` → RC 1 sem remote |
| B.11 | atrito | Site EN+PT | site/en/guide/getting-started.md:223–238 | Wave 0 gerada no roadmap; bloqueia `roadmap move ... done` |
| B.17 | atrito | README + Site EN | README.md:21–28 / site/en/guide/commands.md:22–32 | 30 itens em `~/` por padrão; Quick start e onboarding não mencionam; integrations-manifest não anunciado no stdout |
| B.18 | atrito | README | README.md:442–444 | `trackfw ship` em `main` → RC 1; nota inline discreta em :442; branch não criada antes |
| C.1 | atrito | README | README.md:19,434 | Quick start e continuação separados por 415 linhas; site: usuário para na linha 110 (RC 1) |
| C.2 | atrito | README | README.md:94–411 | 318 linhas de Windows support dentro de Installation |
| C.3 | atrito | site/ | site/ (todos) | Site não tem testes equivalentes ao quickstart_e2e_test.go |
| D.1 | atrito | Site config | site/.vitepress/config.mts:16 | Root locale PT-BR; produto "inglês primeiro" desde 2026-11-02; OG tags em PT-BR |
| D.2 | atrito | Site PT-BR | site/guide/getting-started.md:130–152 | Seções de ADR/REQ em PT-BR no site vs inglês no arquivo real; impacto no diagnóstico de warnings |
| B.6 | cosmético | Site EN+PT | site/en/guide/getting-started.md:285–296 | Formato `[violation]` não existe no output real |
| B.7 | cosmético | Site EN+PT | site/en/guide/getting-started.md:188–189 | "Next step: resolve Draft ADRs" não emitida |
| B.12 | cosmético | Site EN+PT | site/en/guide/getting-started.md:44 | Versão 11.1.0 vs 11.8.1 |
| B.13 | cosmético | Site EN | site/en/guide/commands.md:46 | Texto final de `init` difere |
| B.14 | cosmético | Site EN | site/en/guide/commands.md:16–21 | `--scope` não documentado em init |
| B.15 | cosmético | Site EN+PT | site/en/guide/getting-started.md:304 | Link `#windows` → âncora inexistente no README |
| B.16 | cosmético | Site EN | site/en/guide/commands.md:581–582 | `Critérios de aceite`, `Gates da wave` em PT-BR em página EN |
| B.19 | cosmético | README | README.md:195, 223, 226 | 3 links para anchors de bold paragraphs — não existem no GitHub |

**Contagem por classe: 3 bloqueante · 14 atrito · 8 cosmético (total: 25 achados)**

---

## Proposta de reestruturação (10 itens priorizados)

As propostas abaixo são recomendações; nenhuma mudança foi implementada.

### P1 — Corrigir comandos bloqueantes no site (prioridade: crítica)

`site/en/guide/getting-started.md:110,170` e PT-BR equivalente: adicionar `<title>` como argumento posicional nos exemplos de `adr new` e `req new` — a ausência do título causa RC 1 antes mesmo de qualquer wizard (cobra rejeita a chamada). Correção paralela em `site/index.md:57–58` ("Em 30 segundos").

### P2 — Corrigir `roadmap move` no site (prioridade: crítica)

`site/en/guide/getting-started.md:246` e PT-BR: mudar `REQ-<date>-oauth-login` para `ROADMAP-<date>-oauth-login.md` (ou equivalente com `.md`). Verificar que o comando de exemplo funciona com o roadmap gerado no passo 3.

### P3 — Corrigir `--brownfield` no modo não-TTY (prioridade: crítica — defect de produto)

**Routing:** `internal/commands/init.go` — owner de `internal/`. Não é fix de documentação.

O bloco não-TTY (`init.go:266`) deve ler `--brownfield` via `cmd.Flags().GetBool("brownfield")` (atualmente só em `:531`, no bloco TTY) e passar `cfg.BrownfieldMode = true` para `executeScaffoldAndInstall` antes de retornar em `:309`. O mecanismo de lenient mode no validate está correto (medido: RC 0 com `[LENIENT MODE]` quando os campos existem no yaml); o defeito é que o flag não persiste no yaml quando o `init` roda sem TTY.

**Referência de test:** `grep -rln -i brownfield internal/ --include='*_test.go'` para confirmar cobertura atual.

### P4 — Pinar o site por testes (prioridade: alta)

Criar teste que valide pelo menos: (a) saída de `status` não diverge da amostra documentada; (b) estrutura de diretórios de `init` corresponde ao que `getting-started.md` lista; (c) contagem de slash commands corresponde; (d) roadmap gerado inclui Wave 0. Estender `quickstart_e2e_test.go` ou criar `site_parity_test.go` equivalente. Isso cobre achados de divergência de output e caminho (B.1–B.8, B.11–B.13); não cobre defects binários (B.9) nem dependências de ambiente (B.10).

### P5 — Atualizar estrutura gerada por `init` no site (prioridade: alta)

`site/en/guide/getting-started.md:68–92` e PT-BR: remover `docs/visao-projeto/` e `docs/agents-working-context.md`; adicionar `docs/roadmaps/analyzing/`, `vault/notes/`, os 4 scripts adicionais, `.claude/settings.json`. Acrescentar nota sobre scope global (agents/skills instalados em `~/`, não no projeto).

### P6 — Corrigir README:28 e atualizar snapshots do site (prioridade: média)

**A.2 / README:28:** adicionar nota após o comentário `# ✓ No violations found.` explicando que o resultado depende de preencher os wizards com TTY — ou remover o comentário e substituir por `# (2 warnings se wizard não preenchido)`. Prioridade: evita a primeira surpresa do usuário pós-onboarding.

**B.5–B.7 / `site/en/guide/getting-started.md:264–296`:** substituir pelo output real do 11.8.1. Se o formato muda frequentemente, substituir o bloco de código por uma descrição funcional ("mostra ADRs, REQs e roadmaps em cada estado") em vez de snapshot literal — reduz custo de manutenção futura.

### P7 — Mover o bloco Windows para fora de Installation (prioridade: média)

README.md: extrair `### Windows support (partial)` (318 linhas, 94–411) para uma seção top-level `## Windows` após `## Quick start, continued`. Isso permite que macOS/Linux leitores leiam Installation (npm/pip/curl) sem desvio.

### P8 — Unir ou reordenar as seções Quick start (prioridade: média)

README.md: mover `## Quick start, continued` para imediatamente após `## Quick start` (antes de `## The problem`). Alternativa: unir os dois blocos em um único fluxo de 8 comandos. Elimina o gap de 415 linhas.

### P9 — Corrigir fluxo de `roadmap new` no guia e adicionar passo de branch antes de `ship` (prioridade: média)

`site/en/guide/getting-started.md:208–210`: substituir o exemplo `roadmap new --from-req` pelo `roadmap move ... wip` (como o README faz). Adicionar nota: "O `req new` cria o roadmap automaticamente. `roadmap new` só é necessário se você usou `req new --no-roadmap`."

`README.md:434–449` ("Quick start, continued"): adicionar `trackfw branch new feat/<slug>` antes de `ship` (o comando que `ship` mesmo sugere no erro). `--from HEAD` só é necessário sem `origin`; a maioria dos usuários tem `origin` configurado.

### P10 — Alinhar site ao "inglês primeiro" (prioridade: baixa)

**D.1 / `site/.vitepress/config.mts`:** tornar `en` o locale raiz e mover PT-BR para `/pt/`. Alinha o site com a decisão de produto de 2026-11-02 e garante que links diretos do npm/GitHub levem ao conteúdo em inglês. Requer atualização das URLs internas.

**D.2 / `site/guide/getting-started.md:130–152`:** mostrar o output real do binário em inglês (`## Context`, `## Motivation`) com nota de que o binário é "inglês primeiro".

---

## Notas de execução

- Todos os 5 comandos do bloco README:21–28 retornaram RC 0 com o v11.8.1. A "Quick start, continued" falha em C2 (`trackfw ship` em main, RC 1).
- Os comandos do site (B.1, B.2) falham com RC 1 quando seguidos literalmente.
- Todos os comandos foram executados em modo não-TTY (sem terminal real); os caminhos de wizard TTY são inferidos do código-fonte e indicados como "não medido" no relatório.
- Isolamento confirmado: 6 caminhos verificados (`.claude/agents`, `.claude/skills`, `.codex/agents`, `.agents/skills`, `.trackfw`, `.gemini`) com janela de 24 h. Todos os hits de 2026-11-21: `integrations-manifest.json` (20:14, `trackfw init` anterior do KG), `SKILL.md` (17:53), `~/.gemini/` (17:50–17:53, atividade Gemini/Antigravity do KG). Nenhum de 2026-11-22. `git status --porcelain`: apenas `?? docs/qualidade/2026-11-22-auditoria-onboarding.md`.
- Shell usado: `/bin/bash --noprofile --norc` para evitar aliases de zsh (como `ls --color=never`).
- Temporários removidos ao final da sessão.
