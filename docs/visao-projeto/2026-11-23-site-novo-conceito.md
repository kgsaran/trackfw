# Conceito do site novo — trackfw

> ML-1A | 2026-11-23 | Atena (UX) | binário auditado: v11.9.0

---

## 1. Arquitetura de informação

### 1.1 Estado atual do site

Verificado em `site/.vitepress/config.mts` (locale raiz = inglês; PT-BR em `/pt/`):

| Rota EN | Rota PT-BR | O que é hoje |
|---|---|---|
| `/` | `/pt/` | Home com hero, 4 feature cards, bloco de instalação, "In 30 seconds" |
| `/guide/getting-started` | `/pt/guide/getting-started` | Tutorial sequencial, do `init` ao `validate` |
| `/guide/commands` | `/pt/guide/commands` | Referência de todos os comandos |
| `/guide/ai-agents` | `/pt/guide/ai-agents` | Hooks e YAML frontmatter para agentes de IA |

**O que falta:** o site lista comandos e flag syntax, mas não responde "para que serve isso?", "quem usa?", "o que acontece se eu não usar?". O visitante vindo do npm ou GitHub não vê o produto em ação; vê a referência.

### 1.2 O que mudar e onde

#### Home (`site/index.md`) — problemas reais, confirmados nesta sessão

A home tem um bloco "In 30 seconds" que diverge da sequência pinada pelo teste:

```bash
# Estado atual (site/index.md) — 3 problemas:
trackfw adr new       # (1) sem título → RC 1 "accepts 1 arg(s), received 0"
trackfw req new       # (1) sem título → RC 1
trackfw roadmap move ROADMAP-....md wip  # (2) placeholder não é nome real
trackfw ship -m "feat(x): description"  # (3) ship em main → RC 1
```

Substituir pelo bloco README:21–28, que é o único pinado por `internal/commands/quickstart_e2e_test.go` (o teste também verifica `site/guide/getting-started.md` — ver linhas 501 e 558 do arquivo de teste).

#### Home — estrutura proposta (o que entra e o que sai)

| Seção | Status | O que muda |
|---|---|---|
| Hero | ATUALIZA | Tagline alinhada com pitch do README (ver mockup em §5) |
| Problem (novo) | ENTRA | 3–4 linhas: o problema, sem bullet points |
| Flow diagram (novo) | ENTRA | Diagrama ADR → REQ → ROADMAP → waves → barrier → PR (ver §3) |
| Scenarios (novo) | ENTRA | 3 cards de cenário (ver §2) |
| Features | ATUALIZA | 4 cards reescritos com base na auditoria de onboarding |
| Installation | MANTÉM | Tab group Homebrew / npm / pip / curl — sem mudança |
| Quick start | SUBSTITUI | Bloco README:21–28 verbatim (ver mockup em §5) |

#### Guia — páginas existentes

`site/guide/getting-started.md` já está corrigida (comandos com título, prefixo `ROADMAP-`, Wave 0 documentada, 2 warnings explicados, 9 slash commands); está também pinada por `quickstart_e2e_test.go`. Não exige mudança de conteúdo para o protótipo, apenas a home.

`site/guide/commands.md` já documenta `--scope` e 9 slash commands. O CI workflow gerado por `init --forge github` é `trackfw-gate.yml` (confirmado em `internal/generators/scaffold.go:1711`).

#### Guia — nova página proposta

**"How it works"** (`/guide/how-it-works`): destino do link "Learn more" no flow diagram da home. Contém o diagrama expandido com explicação de cada elo, o modelo de arquivo-como-estado, os guards e o barrier. Sem nova dependência. Não é bloqueante para o protótipo (pode ser apontado por uma âncora em `getting-started` durante a Wave 2 — ver Q4).

---

## 2. Cenários de uso

Hipóteses a validar com o KG. Cada cenário foi conferido contra `bin/trackfw --help` e os subcomandos relevantes.

---

### Cenário 1 — Dev solo com agente de IA

**Quem é:** desenvolvedor solo usando Claude Code, Codex ou Gemini CLI para gerar código. O agente é rápido; a rastreabilidade não existe.

**A dor:** o agente abre um PR às 2 da manhã. O commit diz "fix auth". Qual decisão arquitetural autorizou isso? Qual requisito? Ninguém sabe — nem o próprio desenvolvedor na semana seguinte.

**O que o trackfw faz:**
- `init --ai-tools claude` instala guard hooks que bloqueiam `git commit` / `git push` / criação de branch fora dos comandos do trackfw
- Cada mudança fica ligada a um ADR, uma REQ e um roadmap ativo
- `trackfw context --format=json` emite o estado de governança completo para o agente — sem repetir investigações já feitas
- `trackfw serve` mostra o board local com tudo que está em voo

**Comandos que mostram isso (5):**

```bash
# 1. Projeto inicializado com hooks para Claude Code
trackfw init --ai-tools claude --forge github

# 2. Registrar a decisão antes de codificar
trackfw adr new "Authentication strategy"

# 3. Requisito ligado à decisão (cria roadmap automaticamente em backlog/)
trackfw req new "User login" --adr ADR-$(date +%F)-authentication-strategy.md

# 4. Iniciar a implementação
trackfw roadmap move ROADMAP-$(date +%F)-user-login.md wip

# 5. O agente tenta um push direto — o guard bloqueia (exit 2)
echo '{"tool_name":"Bash","tool_input":{"command":"git push origin main"}}' \
  | trackfw guard git-branch
# → exit 2
```

**Validação do cenário:** todos os 5 comandos confirmados via `--help` e `demo.tape`. Guard names (`git-branch`, `write-scope`, `credential`) confirmados via `trackfw guard --help`. O exit code 2 é o declarado na help page do guard. **O cenário se sustenta.**

---

### Cenário 2 — Time com CI

**Quem é:** time de 3–10 engenheiros com GitHub Actions. O CI valida código; governança é "optional" na prática.

**A dor:** um engenheiro júnior abre um PR com 400 linhas sem ADR, sem REQ, em `feat/my-feature`. O CI diz verde. O tech lead gasta 2 horas de code review percebendo que a feature contradiz uma decisão arquitetural de 6 meses atrás.

**O que o trackfw faz:**
- `trackfw-gate.yml` (gerado por `init --forge github`) roda `trackfw validate` em cada PR como gate de CI — confirmado em `internal/generators/scaffold.go:1711`
- `trackfw barrier` verifica que cada wave do roadmap está completa antes de avançar — sem inventar gates nem assumir build tool
- `trackfw validate --json` emite resultado estruturado para dashboards e ferramentas externas
- `trackfw ship` garante commits no formato Conventional Commits, valida branch name e abre o PR — o comando recusa rodar em `main`

**Comandos que mostram isso (4):**

```bash
# CI: gate de governança em cada PR
trackfw validate
# → "✓ No violations found." ou exit 1 com lista de violações

# CI: saída estruturada para dashboard
trackfw validate --json

# Antes de mergear uma wave: o barrier confirma que tudo está completo
trackfw barrier ROADMAP-$(date +%F)-user-login.md --wave 1
# → 5 checks: wave_headings · mls_complete · acceptance_evidence · gates · validate

# Commit + push + PR aberto em um comando
trackfw ship -m "feat(auth): add login flow"
```

**Validação do cenário:** todos os 4 comandos confirmados via `--help`. **O cenário se sustenta.** Nota de framing: não é ferramenta de gestão de time — governa artefatos, não pessoas (VISION: "What trackfw is NOT").

---

### Cenário 3 — Empresa que só libera npm ou pip

**Quem é:** engenheiro em empresa com política de segurança que bloqueia downloads diretos de executáveis (curl/sh da internet) mas permite `npm install` ou `pip install`.

**A dor:** quer adotar trackfw, mas o instalador curl é bloqueado pela política corporativa.

**O que o trackfw faz:**
- `npm install -g trackfw` instala o mesmo binário Go via shim JavaScript — a resolução acontece no `require()`, não no `postinstall` (confirmado em `npm/bin/trackfw.js`). Compatível com `npm install --ignore-scripts` (inferido do design declarado no comentário do shim; não medido)
- `pip install trackfw` faz o mesmo via PyPI
- Depois da instalação, todos os comandos são idênticos aos cenários 1 e 2

**Comandos que mostram isso (3):**

```bash
# Instalação via canal corporativo permitido
npm install -g trackfw
# ou:
pip install trackfw

# Verificar que é o mesmo binário
trackfw version

# Mesmo Quick start, sem mudança de comportamento
trackfw init --ai-tools claude --forge github
```

**Validação do cenário — parcial, com ressalvas obrigatórias:**

O cenário **se sustenta** para a maioria das políticas "npm/pip mas não curl". Há dois casos em que **não se sustenta**:

1. **Política "nenhum binário compilado de qualquer forma":** o README (linhas 94–98) declara explicitamente que a partir da v8.0, o pacote npm contém o binário Go compilado. Uma política que proíba binários compilados em qualquer canal não pode usar o npm nem o pip package. O site deve declarar essa limitação sem esconder.
2. **Windows com `ExecutionPolicy Restricted`:** o shim npm usa `.ps1` em Windows; essa política bloqueia o shim. Os guard hooks (`.ps1`) também são bloqueados nesse ambiente (documentado no README). O cenário vale em Windows apenas com policy `RemoteSigned` ou mais permissiva.

**Recomendação para o site:** apresentar o cenário com a manchete "If your policy allows npm or pip but not curl" e incluir uma nota de rodapé sobre as duas limitações acima. Ver também Q3 abaixo — este pode ser um cenário de instalação dentro de outro cenário, não um cenário de uso próprio.

---

## 3. Desenho do fluxo

### 3.1 Mockup em texto

```
  ┌─────────────────────────────────────────────────────────────────┐
  │                    governance chain                             │
  │                                                                 │
  │  trackfw adr new          trackfw req new       trackfw roadmap │
  │       │                        │                move ... wip    │
  │       ▼                        ▼                     │          │
  │  ┌─────────┐             ┌─────────┐           ┌───────────┐   │
  │  │  ADR    │────────────▶│  REQ    │──────────▶│  ROADMAP  │   │
  │  │ "why"   │  linked adr │ "what"  │  auto     │  "how"    │   │
  │  └─────────┘             └─────────┘           └─────┬─────┘   │
  │                                                       │          │
  │                                                 backlog/wip/done  │
  │                                                       │          │
  │                                               ┌───────▼───────┐  │
  │                                               │  Wave N       │  │
  │                                               │  ML-NA  ML-NB │  │
  │                                               └───────┬───────┘  │
  │                                                       │          │
  │                                               trackfw barrier    │
  │                                               (5 checks before   │
  │                                                next wave)        │
  │                                                       │          │
  │                                               ┌───────▼───────┐  │
  │                                               │  trackfw ship │  │
  │                                               │  → PR / MR    │  │
  │                                               └───────────────┘  │
  │                                                                   │
  │  guards running at every tool call:                              │
  │  git-branch · write-scope · credential                          │
  └─────────────────────────────────────────────────────────────────┘
```

**Texto alternativo equivalente (para acessibilidade do diagrama ASCII):**
O diagrama mostra cinco elos em sequência: (1) `trackfw adr new` cria um ADR "why"; (2) `trackfw req new` cria uma REQ "what" ligada ao ADR; (3) `trackfw roadmap move ... wip` move o ROADMAP para wip; (4) cada Wave contém microlotes (ML-NA, ML-NB); (5) `trackfw barrier` valida a wave antes de avançar; (6) `trackfw ship` commit + push + PR. Os guards (`git-branch`, `write-scope`, `credential`) rodam em toda tool call do agente.

### 3.2 Forma de renderizar — avaliação das opções

#### Opção A — bloco de código ou `<pre>` com texto alternativo adjacente

**Prós:**
- Zero dependência nova
- VitePress renderiza nativamente
- Diffável no git: cada mudança no produto é uma linha do diagrama

**Contras:**
- Box-drawing characters em `<pre>` são anunciados um por um por screen readers — não comunicam a estrutura do diagrama. Exige um texto alternativo adjacente (lista ordenada dos elos) para satisfazer WCAG 1.3.1
- Alinhamento depende de fonte monoespaçada; pode quebrar em mobile se container for estreito

**Veredicto:** adequado para a versão "How it works" no guia, acompanhado de lista de texto. Pode ser árido demais para a home.

#### Opção B — Mermaid

**Prós:**
- Aparência profissional e familiar para devs

**Contras:**
- Renderizar diagramas Mermaid exige `vitepress-plugin-mermaid` + `mermaid` (dependência nova — verificado: ausente de `site/node_modules`; `site/package.json` lista apenas `vitepress: ^1.6.3`)
- Nova dependência entra no threat model de hades (ML-0A paralelo a este ML)

**Veredicto:** dependência nova sem aprovação do threat model. Não recomendado para esta fase.

#### Opção C — SVG estático

**Prós:** visual rico.

**Contras:** "ilustração inventada que fica desatualizada" — exatamente o que o KG vetou; nenhum subcomando `trackfw diagram` existe.

**Veredicto:** descartado.

#### Opção D — componente Vue (`.vitepress/theme/`)

**Prós:**
- Zero dependência nova — Vue já vem com VitePress
- Elementos HTML reais (`<ol>`, `<a>`, `<button>`) no DOM: screen-readers leem a estrutura, não caracteres de box-drawing — WCAG 1.3.1 sem texto alternativo adicional
- Pode usar CSS do tema (cores, tipografia, dark mode) sem override manual
- Interativo se necessário (hover para tooltips, links para seções do guia)

**Contras:**
- Requer escrita de um componente Vue (~50 linhas) — mais complexo que ASCII para o protótipo

**Veredicto:** melhor escolha para a home. Proposta: um componente `<GovernanceChain />` com 5 nós ligados por setas, cada nó com link para a seção correspondente do guia. Para o protótipo (Wave 2), o agente frontend escreve o componente; para este conceito (Wave 1), o mockup ASCII acima é suficiente.

**Recomendação final:**
- Home (implementação): Opção D (Vue component)
- Home (protótipo Wave 2): Opção A (ASCII com lista de texto adjacente)
- Guia "How it works": Opção A — funcional, sem nova dependência

---

## 4. Imagens e GIFs — inventário

### 4.1 `docs/demo.gif` (existente)

**Como foi gerado:** `vhs demo.tape` (confirmado por `git log --follow docs/demo.gif`: commit `9308be94`). O tape script está em `/demo.tape` na raiz do repositório. Regenerar:

```bash
go build -o "$TMPDIR/trackfw-demo-bin/trackfw" ./cmd/trackfw
PATH="$TMPDIR/trackfw-demo-bin:$PATH" vhs demo.tape
```

VHS suporta `Output file.gif`, `Output file.webm` e `Output file.mp4` (confirmado via `vhs man`).

**O que mostra:** sequência README Quick start completa — `init`, `adr new` (wizard respondido), `req new` (wizard respondido), `roadmap move ... wip`, `validate`, `guard git-branch` bloqueando push, `status`.

**Problema de WCAG 2.2.2 (Pause, Stop, Hide):** o GIF autoplaya infinitamente, sem controle de pausa. Usuários com sensibilidade a movimento não têm como parar. A solução é emitir `.webm` ou `.mp4` via VHS e renderizar como `<video controls poster="...">` com `prefers-reduced-motion: reduce` desativando autoplay. Ver Q5.

**Problema de path:** `docs/demo.gif` está fora de `site/`. O README o referencia via `raw.githubusercontent.com/kgsaran/trackfw/main/docs/demo.gif` — URL externa que rastreia `main`, não uma release. Se o GIF for movido para `site/public/`, o `README.md:17` também precisa ser atualizado. Incluir essa mudança no escopo da Wave 3 ou manter `docs/demo.gif` no lugar atual.

### 4.2 Inventário de novas imagens propostas

Todos os comandos confirmados via `--help` ou `demo.tape`.

| ID | Descrição | Onde entra | Como gerar | Formato | Nota |
|---|---|---|---|---|---|
| IMG-01 | Quick start: `adr new` → `validate` verde | Home, abaixo do Quick start | `vhs site/assets/tapes/quickstart.tape` | `.webm` + `.gif` fallback | Baseado no `demo.tape` existente |
| IMG-02 | Guard bloqueando `git push origin main` enviado pelo agente | Home, cenário 1; guia ai-agents | `vhs` (trecho isolável de `demo.tape:98–105`) | `.webm` + `.gif` fallback | Trecho já existe |
| IMG-03 | `trackfw serve` — board Kanban local | Home, features; cenário 2 | `vhs Screenshot` command com servidor em `mktemp -d` isolado (HOME, USERPROFILE, CODEX_HOME, XDG_CONFIG_HOME redirecionados), `--host 127.0.0.1`. Sem Playwright — nenhum método headless existente confirmado em `scripts/` ou `Makefile`. | `.png` | Único ativo que requer captura de UI; run local por script, não em CI (REQ: "Não cria job novo de CI") |
| IMG-04 | `trackfw validate --json` — saída JSON para CI | Guia commands, cenário 2 | Bloco de código (texto), não imagem | texto | WCAG 1.4.5: output de terminal → `<code>`, não PNG |
| IMG-05 | `trackfw barrier` — relatório de wave completa | Guia "How it works" | Bloco de código (texto) | texto | Mesmo critério |
| IMG-06 | Estrutura de pastas após `init` | Guia getting-started | Bloco de código (texto gerado pelo binário) | texto | Atualizar a estrutura documentada para incluir `vault/notes/`, scripts de attention/credential/branch e `.claude/settings.json` |

**Nota sobre IMG-03 — isolamento obrigatório:** o board do `trackfw serve` reflete os roadmaps do projeto em que o servidor roda. Capturar em `mktemp -d` com `HOME` isolado e dados de fixture estáticos garante conformidade com AC4 ("sem dado pessoal nem caminho local nas imagens").

**Regra geral (WCAG 1.4.5 Images of Text):** output de terminal em texto deve entrar como bloco `<code>`, não como PNG ou GIF. Imagens são reservadas para:
1. Movimento que demonstra interatividade (wizard, guard bloqueando)
2. UI visual que não tem representação em texto (board do `trackfw serve`)

### 4.3 Transcripts de acessibilidade

Cada `<video>` ou `<img>` de terminal deve ter:
- `alt` descritivo (ex: `alt="trackfw guard blocking a raw git push — exits with code 2"`)
- Bloco `<details><summary>Transcript</summary>...</details>` com o texto exato do terminal abaixo do vídeo

---

## 5. Mockups em texto — home

Os textos de tela abaixo são em inglês (decisão do KG de 2026-11-23).

---

### Hero

```
trackfw

Your AI coding agent ships fast.
trackfw makes it ship traceably.

[Get started →]   [View on GitHub]
```

A tagline atual ("Software delivery governance for human teams and AI agents.") é precisa mas passiva. A proposta acima é o pitch do README (linha 11), testado por leitura. A discussão entre as duas está em Q1.

---

### Problem (nova seção, logo abaixo do hero)

```
Most teams accumulate technical debt not because they lack tools,
but because they lack governance traceability.

Decisions are made in Slack. Requirements live in someone's head.
Roadmaps drift from what was actually shipped.

trackfw closes the loop — the connective tissue between why, what, and when.
```

---

### Flow diagram (nova seção)

Título proposto: `"The governance chain"`. Renderizado como componente Vue ou bloco ASCII (ver §3).

```
ADR         REQ              ROADMAP           Waves           PR
"why"  ───▶ "what"  ──────▶  backlog           ML-1A ◀── barrier check
              │               → wip      ───▶  ML-1B
              │               → done            │
              │                                 ▼
              └────────────────────────────── trackfw ship
```

Link ao lado: `Learn how the chain works →` → `/guide/how-it-works`

---

### Scenarios (nova seção)

Título proposto: `"Fits how you work"`. Três cards lado a lado:

```
┌─────────────────────────┐  ┌─────────────────────────┐  ┌─────────────────────────┐
│  Solo dev + AI agent    │  │  Team with CI           │  │  Policy-restricted env  │
│                         │  │                         │  │                         │
│  You use Claude Code or │  │  trackfw validate runs  │  │  Can't curl executables │
│  Codex. Your agent      │  │  as a CI gate on every  │  │  to your machines?      │
│  ships fast — trackfw   │  │  PR. No ADR? No         │  │  trackfw ships via npm  │
│  makes it traceable.    │  │  merge.                 │  │  and pip too.           │
│                         │  │                         │  │                         │
│  [See the commands]     │  │  [See the commands]     │  │  [Read the caveats]     │
└─────────────────────────┘  └─────────────────────────┘  └─────────────────────────┘
```

---

### Features (atualização dos 4 cards existentes)

```
🔗 Governance chain
   ADR → REQ → ROADMAP → backlog/wip/done.
   Every line of code traces back to a decision.

🤖 AI-agent native
   Guard hooks block raw git operations in every major AI CLI.
   `trackfw context --format=json` feeds the full governance
   state to your agent at session start.

🌐 Multi-stack, single binary
   Go, Java, Node, Python. React/Vue/Angular.
   husky, lefthook, GitHub Actions, GitLab CI.
   One Go binary, delivered via npm, pip, Homebrew or curl.

📦 No lock-in
   Markdown files in your own repo. No server, no database,
   no mandatory account. `trackfw serve` gives you a local
   Kanban board — no cloud required.
```

---

### Quick start (substituição do bloco atual)

O bloco atual mostra `trackfw ship` em `main` (RC 1) e `adr new` sem título (RC 1). O bloco abaixo é o pinado por `internal/commands/quickstart_e2e_test.go` (o teste também verifica `site/guide/getting-started.md`, linhas 501 e 558):

```bash
npm install -g trackfw            # or: pip install trackfw · brew install kgsaran/trackfw/trackfw

trackfw init --ai-tools claude --forge github
trackfw adr new "User Authentication Strategy"
trackfw req new "User Authentication" --adr ADR-$(date +%F)-user-authentication-strategy.md
trackfw roadmap move ROADMAP-$(date +%F)-user-authentication.md wip
trackfw validate                  # 0 violations, 2 warning(s) until you fill in the ADR and REQ sections
```

---

## 6. Referências

### R1 — mise.jdx.dev

**URL:** https://mise.jdx.dev/

**Por que citar:** VitePress, open-source, CLI em Rust/Go entregue por múltiplos canais (npm, brew, pip), mesma audiência. A home tem tagline direta ("Dev tools, env vars, and tasks in one CLI"), nav enxuta, e o guia tem uma seção "Getting Started" com exatamente os comandos do caminho feliz — sem poluição de referência. O CLI reference é gerado a partir do fonte (`mise run render:usage`).

**O que vale copiar:** a separação clara entre "Getting Started" (narrativa de onboarding) e "Reference" (gerado do fonte). O trackfw hoje mistura as duas coisas em `getting-started.md`.

**O que não vale copiar:** mise tem sidebar com 30+ entradas — funciona porque o produto tem escopo maior. Para trackfw (5–6 páginas), a sidebar enxuta atual é melhor.

### R2 — charmbracelet (charm.sh)

**URL:** https://charm.sh / https://github.com/charmbracelet

**Por que citar:** Charm é a empresa que faz o VHS (a ferramenta que gera o `demo.gif` do trackfw) e o huh (o wizard do trackfw). O estilo dos READMEs deles — demo GIF no topo, tagline curta, Quick start de 3 comandos, sem seção de Windows na primeira dobra — é o benchmark mais próximo para a primeira dobra da home.

**O que vale copiar:** demo GIF posicionado logo abaixo do hero (não enterrado em 300 linhas). Tagline que mostra o benefício, não a categoria. Terminal como evidência visual primária.

**O que não vale copiar:** o Charm tem identidade visual própria construída sobre Lip Gloss (fontes, cores, gradientes). Para o trackfw, o tema padrão do VitePress já é suficiente para esta fase.

### R3 — evilmartians.github.io/lefthook

**URL:** https://evilmartians.github.io/lefthook/ (domínio principal: https://lefthook.dev/, gerador não confirmado nesta sessão)

**Por que citar:** ferramenta CLI de governança Git, open-source, mesma audiência do trackfw. A página de uso organiza o conteúdo na estrutura "problema → solução → comandos" antes de mostrar referência — essa estrutura narrativa é o que falta na home atual do trackfw.

**O que vale copiar:** a unidade narrativa "problema → solução → comandos" como estrutura dos cenários de uso.

**O que não vale copiar:** a docs do lefthook são muito centradas em configuração YAML. Para o trackfw, o MVP da home não deve entrar em `trackfw.yaml` — isso é material de referência avançada.

---

## 7. Perguntas abertas para o KG

**Q1 — Hero: pitch ativo ou tagline técnica?**
O mockup propõe "Your AI coding agent ships fast. trackfw makes it ship traceably." (pitch do README:11). A tagline atual é "Software delivery governance for human teams and AI agents." A segunda é mais precisa; a primeira é mais ativa. Qual prevalece na home?

**Q2 — Board (`trackfw serve`) no GIF/vídeo do site?**
Uma das imagens propostas (IMG-03) requer captura do `trackfw serve` via VHS Screenshot ou método equivalente. Essa imagem deve entrar já no protótipo (Wave 2) ou pode ficar para a implementação final (Wave 3)?

**Q3 — Cenário 3: canal ou cenário?**
O cenário npm/pip descreve como instalar o trackfw, mas o que o produto faz depois é idêntico aos cenários 1 e 2. A pergunta é: isso deve ser um card de cenário na home, ou uma nota dentro da seção Installation? O "caveats" (políticas "no binaries" e Windows Restricted) se sustentam melhor como nota do que como card de cenário.

**Q4 — Página "How it works": no protótipo ou depois?**
O flow diagram na home pode ser um link "Learn more" → `/guide/how-it-works`. Essa página precisa existir no protótipo (Wave 2) ou o link pode apontar para uma âncora em `getting-started` durante o protótipo?

**Q5 — demo.gif: manter `.gif` ou migrar para `<video controls>`?**
VHS suporta `.webm` e `.mp4` além de `.gif`. Um `<video controls>` resolve WCAG 2.2.2 (autoplay infinito sem controle de pausa). O custo é uma mudança no `demo.tape`, no README:17 e na referência do site. Faz parte do escopo desta REQ ou é item separado?
