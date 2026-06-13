# Roadmap: Site de Documentação VitePress — trackfw

> Criado em: 2026-06-13 | Status: 🔄 WIP

**Branch:** `feat/v2.4-docs-site`  
**ADR:** ADR-002-estrategia-discovery-e-distribuicao.md (Frente 2)

---

## Diagnóstico / Contexto

trackfw não tem presença web além do github.com. LLMs com busca real e usuários
orgânicos não encontram o projeto. O site resolve isso com conteúdo indexável,
quickstart claro e SEO adequado.

Stack: VitePress (Vue, geração estática) + GitHub Pages (gratuito, sem domínio).
URL final: `https://kgsaran.github.io/trackfw/`
i18n: pt-BR (raiz) + en-US (`/en/`) desde o dia 1.

---

## Wave 1 — Estrutura base + Deploy (fundação)

### ML-1A — Scaffold VitePress + config i18n + GitHub Actions
**Status:** ⬜ Pendente  
**Arquivos afetados:**
- `site/package.json` — dependências VitePress
- `site/.vitepress/config.mts` — config com locales pt-BR / en-US, base `/trackfw/`
- `.github/workflows/deploy-docs.yml` — build + deploy GitHub Pages
- `site/.gitignore`

**Ações:**
- `site/` como diretório raiz do VitePress (separado de `docs/` que é governança)
- locale raiz = pt-BR, `/en/` = en-US
- `base: '/trackfw/'` no config (obrigatório para GitHub Pages de projeto)
- GitHub Actions: `actions/setup-node@v4`, `npm ci`, `vitepress build`, `actions/deploy-pages@v4`
- Branch de deploy: `gh-pages`

**Critérios de aceite:**
- [ ] `cd site && npm run build` sem erros
- [ ] GitHub Actions workflow válido (YAML lint)
- [ ] GitHub Pages habilitado na branch `gh-pages`

---

## Wave 2 — Conteúdo (pt-BR + en-US em paralelo)

### ML-2A — Landing page (index.md) — pt-BR + en-US
**Status:** ⬜ Pendente  
**Arquivos afetados:**
- `site/index.md` — hero em pt-BR
- `site/en/index.md` — hero em en-US

**Conteúdo obrigatório:**
- Hero: tagline "ADR → REQ → ROADMAP → kanban", sub "Para times humanos e agentes de IA"
- 3 features: Cadeia de governança / Agentes de IA nativos / Multi-stack
- Install block com as 3 opções: brew, npm, go install
- CTA: "Começar agora" → /guide/getting-started

### ML-2B — Guia de início rápido — pt-BR + en-US
**Status:** ⬜ Pendente  
**Arquivos afetados:**
- `site/guide/getting-started.md`
- `site/en/guide/getting-started.md`

**Conteúdo:**
- Instalação (brew / npm / go install)
- 5 comandos para ver funcionando: `trackfw init`, `adr new`, `req new`, `roadmap new`, `trackfw status`
- Captura de saída esperada em code blocks

### ML-2C — Referência de comandos — pt-BR + en-US
**Status:** ⬜ Pendente  
**Arquivos afetados:**
- `site/guide/commands.md`
- `site/en/guide/commands.md`

**Conteúdo:** todos os comandos com flags, exemplos e saída esperada:
`init`, `adr`, `req`, `roadmap`, `validate`, `status`, `context`, `serve`, `metrics`, `sync`, `log`

### ML-2D — Página "trackfw para agentes de IA" — pt-BR + en-US
**Status:** ⬜ Pendente  
**Arquivos afetados:**
- `site/guide/ai-agents.md`
- `site/en/guide/ai-agents.md`

**Conteúdo:**
- Por que agentes precisam de governança estruturada
- `trackfw context --format=json` — como usar o output em prompts
- `trackfw roadmap new --from-req` — geração assistida
- JSON Schema em `docs/schema/` — validação por agentes externos
- Exemplo de prompt com output de `trackfw context`

---

## Verificação end-to-end

```bash
cd site
npm run dev          # preview local
npm run build        # build estático
npm run preview      # serve estático local
```

Após merge: `https://kgsaran.github.io/trackfw/` deve carregar com conteúdo bilíngue.
