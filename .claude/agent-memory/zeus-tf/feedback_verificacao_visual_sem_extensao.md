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

**Atualização 2026-11-22:** o Google Chrome não está mais instalado. O `/opt/homebrew/bin/chromium` (Chromium.app) **trava** com `--screenshot` e não grava o PNG, tanto na afrodite quanto comigo (alarm de 30 s, rc=142). O que funcionou foi o binário do Playwright, já em cache: `~/Library/Caches/ms-playwright/chromium_headless_shell-1208/chrome-headless-shell-mac-arm64/chrome-headless-shell --disable-gpu --hide-scrollbars --user-data-dir=<scratch>/prof --window-size=1280,900 --screenshot=<scratch>/x.png <url>` (rc=0 em ~2 s). Para servir o `dist` com o `base` `/trackfw/`, use `python3 -m http.server` num diretório com um symlink `trackfw -> site/.vitepress/dist`. No handoff, passe este caminho, e não o do chromium.

