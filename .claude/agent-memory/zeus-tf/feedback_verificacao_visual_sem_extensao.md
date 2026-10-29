---
name: verificacao-visual-sem-extensao
description: Sem as ferramentas mcp__claude-in-chrome na sessão, a verificação visual obrigatória sai do Chrome headless com --screenshot, lido pelo Read
metadata:
  type: feedback
---

Quando as ferramentas de navegador (`mcp__claude-in-chrome__*`) não estão disponíveis, a verificação visual não é dispensada. Ela sai do Chrome headless:

```bash
"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" --headless=new --disable-gpu \
  --hide-scrollbars --window-size=1280,4200 --screenshot=<scratchpad>/shot.png "<url>"
```

O PNG gerado se lê com `Read`.

**Why:** em 2026-10-29 (ML-3B, site VitePress), o `ToolSearch` não existia na sessão. O screenshot headless mostrou o que build, gate e curl não mostravam: um logo quebrado no cabeçalho de todas as páginas do site publicado desde a criação (`/trackfw/logo.svg` dava 404 e nunca existiu). Ver [[verificacao-visual-obrigatoria]].

**How to apply:** em todo ML de UI ou site, suba o preview (`npx vitepress preview`, `trackfw serve`) e tire um screenshot por idioma ou página afetada, mais um recorte do cabeçalho depois de cada corretivo. Desligue o preview antes do `make quality`.
