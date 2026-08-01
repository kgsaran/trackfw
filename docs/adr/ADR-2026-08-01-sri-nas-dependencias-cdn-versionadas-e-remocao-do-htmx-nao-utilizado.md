---
status: Accepted
date: 2026-08-01
author: "Zeus"
---

# ADR: SRI nas dependencias CDN versionadas e remocao do htmx nao utilizado

> Date: 2026-08-01 | Status: Accepted

## Context

O dashboard do `trackfw serve` carrega cinco dependências por CDN. Quando o DOMPurify foi
adicionado no PR #95, ele recebeu `integrity` (SRI) — mas as cinco anteriores seguem sem, e
estender o SRI ficou registrado como follow-up.

Estado verificado em 2026-08-01 (`internal/serve/static/index.html`):

| Linha | Dependência | URL | Versionada? | SRI |
|---|---|---|---|---|
| 8 | Tailwind | `https://cdn.tailwindcss.com` | **não** | não |
| 10 | htmx 2.0.4 | `unpkg.com/htmx.org@2.0.4/...` | sim | não |
| 12 | marked 12.0.0 | `cdn.jsdelivr.net/npm/marked@12.0.0/...` | sim | não |
| 14 | chart.js 4.4.4 | `cdn.jsdelivr.net/npm/chart.js@4.4.4/...` | sim | não |
| 16 | d3 7.9.0 | `cdn.jsdelivr.net/npm/d3@7.9.0/...` | sim | não |
| 320 | DOMPurify 3.4.12 | `cdn.jsdelivr.net/npm/dompurify@3.4.12/...` | sim | **sim** |

Dois fatos mudaram o que parecia ser "adicionar SRI em cinco tags":

**1. O htmx não é usado.** Varredura em `internal/serve/`, `npm/src/serve/` e
`pypi/trackfw/serve/`: **zero** atributos `hx-*` no HTML e **zero** referências a `htmx` no
`app.js`. É dependência morta, baixada em toda visita.

**2. O Tailwind não pode receber SRI com segurança.** A URL é não-versionada e responde
`HTTP/2 302` com `cache-control: max-age=14400`. O conteúdo servido pode mudar a qualquer
release. Um `integrity` fixo ali quebraria o dashboard inteiro — sem estilo algum — no momento em
que a Tailwind publicasse uma atualização, e de forma silenciosa para quem não olhasse o console.

## Decision

**SRI onde é seguro; remoção onde a dependência é morta; exceção documentada onde SRI é inviável.**

1. **htmx é removido.** Não recebe SRI — a tag sai. Eliminar o vetor é estritamente melhor do que
   mitigá-lo, e ainda tira o `unpkg.com` inteiro da cadeia de suprimentos do dashboard.
2. **marked, chart.js e d3 recebem `integrity` + `crossorigin="anonymous"` +
   `referrerpolicy="no-referrer"`**, no mesmo padrão já aplicado ao DOMPurify. Hashes calculados
   em 2026-08-01 e **conferidos em dois downloads independentes cada**:

   | Dependência | SRI |
   |---|---|
   | marked 12.0.0 | `sha384-NNQgBjjuhtXzPmmy4gurS5X7P4uTt1DThyevz4Ua0IVK5+kazYQI1W27JHjbbxQz` |
   | chart.js 4.4.4 | `sha384-NrKB+u6Ts6AtkIhwPixiKTzgSKNblyhlk0Sohlgar9UHUBzai/sgnNNWWd291xqt` |
   | d3 7.9.0 | `sha384-CjloA8y00+1SDAUkjs099PVfnY2KmDC2BZnws9kh8D/lX1s46w6EPhpXdqMfjK6i` |

3. **O Tailwind fica sem SRI**, com a razão registrada aqui e em comentário no próprio
   `index.html`, para que ninguém "conserte" a inconsistência sem entender o motivo.
4. Mudança **frontend pura**, restrita a `index.html`. `internal/serve/static/` é canônico;
   npm e pypi são espelhos byte-a-byte.

## Consequences

**Positivas**

- Quatro das cinco dependências restantes ficam protegidas contra CDN comprometido (três por SRI,
  uma por remoção). Somando o DOMPurify, o dashboard passa de 1/6 para 5/6 tags tratadas.
- O `unpkg.com` sai da cadeia — menos um terceiro em quem confiar.
- Uma requisição a menos e ~50 KB a menos por visita.

**Negativas / aceitas**

- **O Tailwind, que é a maior dependência do dashboard, segue desprotegido.** É o buraco real que
  esta decisão não fecha. Aceito porque a alternativa — trocar a Play CDN por URL versionada —
  substitui um compilador JIT em runtime por um artefato estático, o que exige auditoria visual
  de todo o dashboard e traz risco concreto de regressão de layout. Fica como REQ própria caso
  a proteção passe a ser exigida.
- Se alguém pretendia usar htmx, precisará readicioná-lo — agora com SRI. Nada no repositório
  indica essa intenção.
- Hashes fixos exigem atualização manual junto com qualquer bump de versão. É o custo inerente ao
  SRI e o motivo pelo qual só se aplica a URL versionada.

## Alternatives Considered

**Adicionar SRI ao htmx e mantê-lo** — cobriria as cinco tags de maneira uniforme.
**Rejeitado:** protege algo que não deveria estar carregado. SRI numa dependência morta é
cerimônia; a resposta correta a código não usado é removê-lo.

**Fixar o Tailwind numa URL versionada e aplicar SRI** — cobertura completa.
**Rejeitado por decisão do usuário:** a Play CDN compila utilitários em runtime; migrar para o
build estático é mudança de comportamento, não de configuração, e pediria verificação visual de
todas as views. Desproporcional para um ciclo cujo objetivo é endurecer a cadeia de suprimentos.

**Baixar as dependências e servi-las localmente via `go:embed`** — eliminaria a cadeia externa
inteira e resolveria também o Tailwind. **Rejeitado:** contraria a decisão de distribuição já
vigente (o dashboard carrega tudo por CDN), aumentaria muito o tamanho do binário e dos pacotes
npm/pypi, e teria de ser espelhado nos três CLIs. Merece ADR próprio se um dia for considerado.
