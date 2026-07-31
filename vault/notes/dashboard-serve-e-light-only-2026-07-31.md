# O dashboard do `trackfw serve` é light-only — não adicione `prefers-color-scheme`

> Data: 2026-07-31 | Autor: Zeus | Domínio: serve / frontend

## Sintoma

Componente novo adicionado ao dashboard renderiza com fundo escuro e texto branco, enquanto
nav, campo de busca, contador, drawer e cards do Board continuam claros. Só acontece em
máquinas com o **sistema operacional** em modo escuro — quem desenvolve em modo claro não vê
o defeito e aprova a mudança sem perceber.

Foi exatamente o que aconteceu com as abas ADRs/REQs: a auditoria só pegou porque a
verificação visual rodou num Chrome com `matchMedia('(prefers-color-scheme: dark)').matches
=== true`.

## Causa raiz

`internal/serve/static/style.css` **não tinha nenhuma** regra `@media (prefers-color-scheme)`
antes disso:

```bash
git show <commit-anterior>:internal/serve/static/style.css | grep -c "prefers-color-scheme"
# 0
```

O dashboard usa **Tailwind via CDN** (`cdn.tailwindcss.com`) sem `darkMode` configurado, e o
`index.html` não tem nenhuma variante `dark:`. Ou seja: a página inteira ignora a preferência
de tema do SO, por design.

Quando um componente novo traz seu próprio bloco `@media (prefers-color-scheme: dark)`, ele
vira **a única coisa na página** que responde ao tema do SO. O resultado é pior do que não ter
dark mode: é dark mode parcial e inconsistente.

## Regra

**Não adicione `@media (prefers-color-scheme: dark)` a `internal/serve/static/style.css`.**
Estilize apenas para o tema claro, alinhado aos tokens já em uso:

- Fundo de card/linha: `rgb(249, 250, 251)` (gray-50)
- Texto principal: `rgb(17, 24, 39)` (gray-900)
- Borda: `rgb(229, 231, 235)` (gray-200)

Esses são os valores computados de `.kanban-card`, que é a referência visual do dashboard.

Suporte a tema escuro é uma decisão de produto que precisa cobrir a página inteira —
nav, drawer, gráficos D3, Chart.js e cards. É REQ própria, não efeito colateral de um
componente.

## Armadilha de verificação

Gates verdes (`make build`, `make test`, `make lint`, `make quality`) **não detectam** este
defeito — é puramente visual. A verificação precisa ser feita em navegador real, e de
preferência com o SO em modo escuro, que é justamente o cenário em que a falha aparece.

Relacionado: `vault/notes/roadmap-new-gera-marcador-de-aceite-invalido-2026-07-31.md`.
