---
name: trackfw-dashboard-light-only
description: O dashboard do `trackfw serve` é light-only por design — nada de prefers-color-scheme em style.css
metadata:
  type: project
---

O dashboard servido por `trackfw serve` não suporta tema escuro e isso é deliberado: usa Tailwind
via CDN sem `darkMode` configurado e sem nenhuma variante `dark:` no `index.html`.

**Why:** um componente que traz seu próprio `@media (prefers-color-scheme: dark)` vira a única
coisa da página que responde ao tema do SO — dark mode parcial é pior que nenhum. Aconteceu nas
abas ADRs/REQs em 2026-07-31 e exigiu um ML corretivo.

**How to apply:** ao planejar mudanças de UI no dashboard, estilizar só para o tema claro, usando
os tokens de `.kanban-card` como referência (fundo `rgb(249,250,251)`, texto `rgb(17,24,39)`,
borda `rgb(229,231,235)`). Suporte a tema escuro, se algum dia for pedido, é REQ própria cobrindo
a página inteira — nav, drawer, D3 e Chart.js.

Detalhe operacional relacionado: os assets do Go são `go:embed`, então mudança em
`internal/serve/static/` só aparece após `make build`; npm e pypi servem do disco. E
`internal/serve/static/` é a **fonte canônica** — `scripts/check-static-assets.sh` exige espelho
byte-a-byte em npm e pypi, o que faz a paridade dos 3 CLIs virar cópia mecânica.

Relacionado: [[verificacao-visual-obrigatoria]]
