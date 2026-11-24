# Wave 0 — Threat Model: Site Novo do trackfw

> Hades-tf · 2026-11-23 · REQ-2026-11-23 · Wave 0, ML-0A
> Binário testado: 11.9.0 · Todas as execuções com HOME isolado confirmado

---

## Seção 1 — Completude da enumeração

### Superfícies que o site novo acrescenta ou muda

#### 1A — Imagens e GIFs gerados a partir do binário

O `demo.tape` existente já registra o padrão correto de isolamento:

```
cd "$(mktemp -d)" && export HOME="$PWD/.home" && mkdir -p "$HOME" && export PS1='$ '
```

Cada comando candidato a imagem foi executado em ambiente isolado (`HOME`, `USERPROFILE`, `CODEX_HOME`, `XDG_CONFIG_HOME` apontando para tmp). Resultados medidos:

| Comando | Vazamento de path absoluto em stdout | Vazamento de identidade git | Vazamento de credencial |
|---|---|---|---|
| `trackfw init --forge github --ai-tools claude` | Não — usa notação `~` em destinos globais | Não — `author: ""` nos arquivos gerados | Não |
| `trackfw adr new "..."` | Não — só basename no stdout | Não — confirmado com canário CANARYNAME9x | Não |
| `trackfw req new "..."` | Não — caminhos relativos | Não | Não |
| `trackfw roadmap move ... wip` | Não | Não | Não |
| `trackfw validate` | Não — apenas basenames em mensagens de erro | Não | Não |
| `trackfw status` | Não — apenas basenames de roadmap | Não | Não |
| `trackfw context` | Não — basenames; global ADR exibida só com basename | Não | Não |
| `trackfw serve` (startup) | Não — "listening on http://localhost:4080" | Não | Não |

**Risco residual de PS1:** se o script de geração das novas imagens não definir `PS1='$ '` explicitamente, o prompt do shell exibirá o caminho absoluto do projeto (ex.: `kgsaran@hostname ~/projeto $`). Medição direta: o `demo.tape` já mitiga isso. O risco existe apenas se um novo script não copiar o padrão.

**Achado — `/api/chain` expõe path absoluto de ADR global (ALTO):**

Medido em ambiente isolado com `HOME=$ISOLATED_HOME`:

```
$ curl -s http://localhost:4080/api/chain | python3 -c "import sys,json; ..."
id: /private/tmp/.../scratchpad/tf-isolation-3vJpfl/home/canary-adrs/ADR-canary-global.md
```

Quando o `trackfw.yaml` de um projeto real contém uma entrada `~/global-adrs` em `adr_dirs`, o endpoint `/api/chain` retorna o path completo expandido (`/Users/kgsaran/global-adrs/ADR-meu-adr.md`) como campo `id` dos nós do grafo de dependências. Uma captura de tela do painel de cadeia (`/api/chain`) do `trackfw serve` publicaria esse path. O endpoint `/api/board` não tem esse problema — usa apenas basenames.

Isso é um defeito em `internal/serve` (não em `internal/generator`). O fix é normalizar o `id` dos nós de ADR global para basename ou usar notation relativa (`~/global-adrs/ADR-...`).

**Datas em nomes de arquivo:** O binário chama `time.Now()` diretamente (sem `SOURCE_DATE_EPOCH` nem variável de substituição). As imagens terão datas de quando o script foi executado, não datas pessoais — baixo risco, não é dado pessoal.

#### 1B — Dependências npm hoje e candidatas

**Hoje:** `vitepress@^1.6.3` (caret — minor updates permitidas, mas `npm ci` resolve pelo `package-lock.json`).

**Candidata: Mermaid 12.3.0**

`npm view mermaid@12.3.0 dependencies` lista: `d3@^7.9.0`, `elkjs`, `katex`, `dayjs`, `cytoscape`, `dompurify`, `roughjs`, `dagre-d3-es`, `chevrotain`, `cytoscape-fcose`, `@upsetjs/venn.js` — árvore larga (~50+ pacotes transitivos). Se usado como plugin VitePress (`vitepress-plugin-mermaid@2.0.17`), o processamento ocorre em build-time, não em runtime do visitante — isso elimina o risco de execução maliciosa no browser. O risco permanece na etapa de `npm ci` do CI.

**Alternativa mais segura:** pré-renderizar diagramas para SVG estático (`mermaid` em modo CLI ou `mmdc`) e embutir o SVG na página. Remove inteiramente a dependência de runtime e reduz a superfície de ataque no CI.

#### 1C — Assets externos do site estático (VitePress)

VitePress 1.6.3 tem a seguinte referência no código-fonte:

```css
/* site/node_modules/vitepress/dist/client/theme-default/styles/fonts.css */
@import url('https://fonts.googleapis.com/css2?family=Inter:...');
```

**Medição — build real:** o `site/.vitepress/dist/assets/style.BNLYooxM.css` não contém a URL do Google Fonts. O build substitui o `@import` por `@font-face` com fontes locais bundled em `/trackfw/assets/inter-roman-*.woff2`. A fonte Inter é servida do próprio GitHub Pages, sem chamada ao Google no runtime.

Resultado: **sem Google Fonts no site estático publicado.** Nenhum IP de visitante enviado ao Google por essa via.

#### 1D — Workflow de deploy (`.github/workflows/deploy-docs.yml`)

```yaml
permissions:
  contents: read
  pages: write
  id-token: write
```

**Problema:** as permissões são declaradas no nível do workflow, não do job. O job `build` herda `pages: write` e `id-token: write`, embora só precise de `contents: read`. Qualquer script de lifecycle npm (incluindo os de Mermaid ou Playwright se adicionados) executado no passo `npm ci` recebe um token GITHUB_TOKEN com `pages: write`.

**Actions e pinagem:**

| Action | Pin | SHA? |
|---|---|---|
| `actions/checkout@v7` | tag | não |
| `actions/setup-node@v7` | tag | não |
| `actions/upload-pages-artifact@v5` | tag | não |
| `actions/deploy-pages@v5` | tag | não |

Todas são GitHub-owned e não há actions de terceiros no workflow de deploy. `pypa/gh-action-pypi-publish@release/v1` em `release.yml` usa pin de branch — pré-existente, fora do escopo desta REQ.

**`persist-credentials` em `actions/checkout`:** o padrão de `actions/checkout@v7` é `persist-credentials: true`, o que armazena o GITHUB_TOKEN no config do git e o torna disponível para comandos subsequentes no job `build`.

---

## Seção 2 — Threat Model

**Adversário:** o implementador apressado e o arquiteto otimista — quem gera as imagens ou escreve o script de geração sem reler o `demo.tape`.

### T1 — PS1 não isolado (ALTO)

**Como:** o implementador cria um script novo para gerar screenshots das novas imagens do site sem copiar o padrão `export PS1='$ '` do `demo.tape`. O script abre um terminal com o PS1 padrão do sistema, que mostra `kgsaran@hostname ~/meu-projeto $`. A imagem capturada publica o username, o hostname e o caminho do projeto.

**Gate que deveria pegar:** AC4 da REQ ("sem dado pessoal nem caminho local nas imagens"). O gate só pega se houver uma verificação automatizada — sem ela, é só leitura visual.

**Prova de viabilidade:** o `demo.tape` linha 29 mostra que o problema era conhecido desde a criação do GIF original.

### T2 — Screenshot do painel `/api/chain` com ADR global (ALTO)

**Como:** o implementador captura uma screenshot do painel de dependências do `trackfw serve` para mostrar o fluxo ADR → REQ → ROADMAP. O projeto de demonstração tem `~/global-adrs` em `adr_dirs`. O painel mostra os nós do grafo com `id` = path absoluto. A screenshot publica `/Users/kgsaran/global-adrs/ADR-minha-decisao.md`.

**Gate que deveria pegar:** inspeção do JSON do `/api/chain` antes de capturar a screenshot. Não existe como gate automático hoje.

**Prova medida:** o endpoint `/api/chain` retorna o path absoluto expandido confirmado com canário. O fix pertence a `internal/serve` — a normalização deve usar basename ou substituir o prefix de HOME por `~`.

### T3 — `npm ci` com `pages: write` instala código malicioso (MÉDIO)

**Como:** uma nova dependência de geração (Playwright, Puppeteer, Mermaid plugin) é adicionada a `site/package.json`. Um mantenedor desse pacote publica uma versão com script `postinstall` malicioso. O CI executa `npm ci` no job `build` que tem `id-token: write` e `pages: write`. O script de lifecycle usa o GITHUB_TOKEN (herdado de `persist-credentials: true`) para publicar conteúdo arbitrário no GitHub Pages ou para exfiltrar o token.

**Gate que deveria pegar:** separação de permissões por job (fix proposto em Seção 4). Hoje não existe.

**Condição:** exige comprometimento de um maintainer de dependência npm. Probabilidade baixa, impacto alto (publicação arbitrária no Pages).

### T4 — Screenshot do `trackfw serve` com CDN indisponível (MÉDIO)

**Como:** `trackfw serve` carrega `https://cdn.tailwindcss.com` (sem versão) e quatro pacotes do `cdn.jsdelivr.net` (com versão, sem SRI). Se a CDN estiver offline no momento da captura, o board aparece sem estilo, sem gráficos e sem renderização de Markdown. A screenshot mostra "nada útil" — falso negativo de qualidade.

**Gate que deveria pegar:** verificar que o board está totalmente renderizado antes de capturar (health check de resposta dos CDNs ou uso de `--wait-for-selector`).

### T5 — Mermaid com SVG contendo XSS via diagrama controlado por dev (BAIXO)

**Como:** se Mermaid for usado em modo runtime (não em build-time), um diagrama com título ou rótulo contendo `<script>` ou `javascript:` seria injetado no DOM. DOMPurify mitiga isso, mas versões antigas de Mermaid já tiveram bypasses.

**Gate:** AC5 (build rc=0) não pega isso. Mitigação: usar pré-renderização SVG em build-time.

---

## Seção 3 — Alvos de falsificação nas duas direções

### 3A — Imagem que vaza (falso negativo do gate)

**Superficie:** screenshot de terminal sem `PS1='$ '`, mostrando `kgsaran@hostname ~/trackfw $`.

**Por que o gate falha:** AC4 diz "sem dado pessoal", mas não há verificação automatizada que abra a imagem e procure padrões. Uma leitura visual pode passar batido se a string aparecer no prompt, fora do foco narrativo da imagem.

**Por que `grep` não resolve:**
```bash
grep -c "kgsaran" sample_0016.png   # resultado: 0
```
O texto está rasterizado em pixels. `grep` é cego a texto em imagens.

**Ferramenta de detecção correta:** `strings` nos arquivos de imagem detecta texto em metadados e comentários embutidos, mas não texto rasterizado. Tesseract (OCR) detectaria o texto rasterizado — não está disponível no ambiente atual.

**Controle proposto:** o script de geração valida sua própria saída antes de commitar: executa OCR (ou `strings`) sobre cada imagem gerada e aplica um regex ao resultado:

```bash
strings "$IMAGE" | grep -iE '(/Users/|/home/|C:\\Users\\|@[a-z0-9]+\.(com|br|io)|ghp_[A-Za-z0-9]+|token|password)'
```

E confirma que PS1 está definido antes de qualquer captura:
```bash
[[ "$PS1" == '$ ' ]] || { echo "ERRO: PS1 não isolado"; exit 1; }
```

**Canário para provar que o controle funciona:** antes de cada sessão de geração, o script planta uma imagem de teste com o texto `LEAK-TEST-kgsaran` visível, verifica que o scanner a rejeita, e só então prossegue com as imagens reais.

### 3B — Gerador que redige demais (falso positivo do gate)

**O que é:** um sanitizer que procura `kgsaran` em toda saída (incluindo URLs do projeto) bloquearia também:
- `github.com/kgsaran/trackfw` — URL legítima do projeto
- `npm install -g trackfw` seguido de `kgsaran` no context de um exemplo de roadmap

**Por que isso seria um problema:** o site exibirá a URL do repositório em vários lugares; um scanner que proíba `kgsaran` cegamente tornaria o site impossível de publicar.

**Regra correta:** o scanner deve procurar padrões de *caminho pessoal*, não o string `kgsaran` como tal:

```
/Users/<qualquer>
/home/<qualquer>
C:\Users\<qualquer>
endereços de email (.*@.*)
prefixos de token (ghp_, ghs_, ghr_, sk-, Bearer )
```

E deve ter um allowlist explícito:
```
github.com/kgsaran/trackfw    # URL pública do projeto — permitida
npmjs.com/package/trackfw     # idem
```

**Prova de que o scanner funciona de verdade:** deve **falhar** num fixture de teste que contém `/Users/kgsaran/meu-projeto` e **passar** num fixture que contém apenas `github.com/kgsaran/trackfw`.

---

## Seção 4 — Residual declarado e controles propostos

### Residuais aceitos (não corrigidos nesta REQ)

| Item | Justificativa |
|---|---|
| **`/api/chain` expõe path absoluto de ADR global** | Defeito em `internal/serve`. Fora do escopo de código de produto desta REQ. Deve abrir REQ própria com fix em `internal/serve`. |
| **Actions sem SHA pin** (`@v7`, `@v5`) | Pre-existente em todos os workflows. `actions/*` são GitHub-owned; risco de supply chain é baixo sem terceiros. Recomendado como housekeeping, não bloqueante. |
| **CDN sem SRI em `trackfw serve`** | Pre-existente em `internal/serve`. Afeta apenas usuários que rodam `trackfw serve` localmente; não afeta visitantes do site estático. Deve abrir REQ própria. |
| **`tailwindcss` CDN sem versão** | Idem — `internal/serve`. |
| **`pypa/gh-action-pypi-publish@release/v1` (branch pin)** | Pre-existente em `release.yml`, fora do escopo desta REQ. |
| **Datas em nomes de arquivo gerados** | Não é dado pessoal — apenas indica quando o script rodou. Pode ser estabilizado com `TZ=UTC` e um commit de data fixa nos dados de demonstração, mas não é requisito de privacidade. |

### Controles propostos para a implementação (Waves 2–3)

**C1 — Script de geração de imagens deve herdar o padrão do `demo.tape`:**

```bash
# Obrigatório no início de qualquer script de captura
export HOME="$(mktemp -d)"
export USERPROFILE="$HOME"
export CODEX_HOME="$HOME/.x"
export XDG_CONFIG_HOME="$HOME/.c"
export PS1='$ '
export GIT_AUTHOR_NAME="Demo User"
export GIT_AUTHOR_EMAIL="demo@example.invalid"
export GIT_COMMITTER_NAME="Demo User"
export GIT_COMMITTER_EMAIL="demo@example.invalid"
export TZ=UTC
# GH_TOKEN e GITHUB_TOKEN: ausentes (não exportar)
```

**C2 — Verificação de path e credencial pós-geração:**

Executar sobre cada imagem/GIF gerado antes de commitar:

```bash
strings "$f" | grep -qiE '(/Users/[a-z]|/home/[a-z]|C:\\Users\\|@.*\.(com|io|br)|ghp_|ghs_|ghr_|Bearer |password|token)' \
  && echo "VAZAMENTO em $f" && exit 1
```

Allowlist: `github.com/kgsaran/trackfw` não ativa o filtro (o padrão `@.*\.(com|io|br)` não casa com URLs sem `@`).

**C3 — Não usar painel `/api/chain` em screenshots enquanto `internal/serve` não normalizar os IDs:**

Enquanto o bug T2 não for corrigido, as screenshots do `trackfw serve` devem usar apenas o painel Kanban (`/api/board`). Se a chain view for necessária para o site, o projeto deve ter `adr_dirs` com apenas caminhos relativos (sem `~`).

**C4 — Separação de permissões por job no deploy workflow:**

```yaml
jobs:
  build:
    permissions:
      contents: read      # suficiente para checkout e build
    steps: ...

  deploy:
    permissions:
      pages: write
      id-token: write
    needs: build
    steps: ...
```

Isso impede que scripts de lifecycle npm de dependências de build acessem o token de deploy.

**C5 — Mermaid: usar pré-renderização em build-time:**

Se diagramas Mermaid forem necessários, usar `mmdc` (CLI do Mermaid) para gerar SVG em build-time e embutir o resultado estático. Não adicionar `vitepress-plugin-mermaid` ao `site/package.json` sem avaliar a árvore de dependências.

---

## Evidências colhidas

**HOME real nunca usado:** o primeiro comando desta sessão, `which trackfw && trackfw version`, foi bloqueado pelo auto-mode antes de executar (negado pelo classificador). Nenhum outro comando executou sem o prefixo de isolamento. Confirmado: `HOME=/Users/kgsaran` aparece apenas no log de verificação inicial (read-only), nunca em execução do binário.

**Comandos de git executados no fixture isolado:** `git init`, `git config user.*`, `git remote add/set-url` (todos em `$ISOLATED/project` ou `$ISOLATED/canary-identity`). Tentativa de `git add -A && git commit` foi bloqueada pelo guard antes de executar — sem staging.

**init não escreveu no HOME real:** `~/.claude/agents/trackfw-architect.md` tem data de modificação 2026-11-01 (pré-existente). Agents do teste foram escritos em `$ISOLATED_HOME/.claude/agents/`.

**Canário de identidade git:** `CANARYNAME9x`/`CANARYEMAIL9x@example.invalid` não apareceu em nenhum arquivo gerado pelo `adr new` — `author: ""` confirmado.

**Canário de ADR global:** `/api/chain` expôs `$ISOLATED_HOME/canary-adrs/ADR-canary-global.md` — confirmação do achado T2.

**Canário de credencial:** `CANARYTOK1` (remote URL), `CANARYTOK2` (`GH_TOKEN`), `CANARYTOK3` (`GITHUB_TOKEN`) — nenhum apareceu em stdout de `validate`, `context`, `/api/chain` ou `/api/board`.

**demo.gif (49s, 16 frames amostrados):** exibe apenas texto de comando genérico (`trackfw init --ai-tools claude --forge github`, `trackfw adr new "User Authentication Strategy"`, etc.). Nenhum path pessoal, username, hostname ou token nos frames visuais. `strings docs/demo.gif` não retornou matches para `/Users`, `home`, `@`, `token`, `kgsaran`.

**Grep em PNG rasterizado:** `grep -c "Inventory" sample_0016.png` retorna `0`, mesmo com "Inventory" visível na imagem — confirmado que o controle de detecção C2 requer `strings` ou OCR, não `grep`.

**Google Fonts:** o build VitePress bundla a fonte Inter localmente; `googleapis` não aparece em `site/.vitepress/dist/`. Confirmado que visitantes não enviam requests ao Google ao carregar o site.

---

*Relatório produzido por hades-tf · ML-0A concluído · Nenhuma linha de implementação escrita neste ML.*
