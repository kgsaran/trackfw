---
status: Accepted
date: 2026-11-23
author: "zeus-tf (conceito e protótipo aprovados pelo KG em 2026-11-23)"
---

# ADR: Site do trackfw: home com visão do produto e imagens geradas pelo binário

> Date: 2026-11-23 | Status: Accepted

## Context

O site (`site/`, VitePress, GitHub Pages em `/trackfw/`) listava comandos e não mostrava para que o trackfw serve. A
REQ-2026-11-23 seguiu o método protótipo primeiro:

- conceito da atena-tf (`docs/visao-projeto/2026-11-23-site-novo-conceito.md`), com as decisões do KG registradas no
  roadmap;
- threat model (`docs/seguranca/2026-11-23-wave0-site-novo-threat-model.md`). As saídas do binário estão limpas, mas
  há riscos: o painel de cadeia do `serve` expõe caminho absoluto (REQ própria no backlog), um script de imagem sem
  ambiente fixo mostraria o cwd, e o job de build do deploy tem permissões de deploy;
- protótipo isolado (`site/prototype.md`), aprovado pelo KG em 2026-11-23.

## Decision

**D1 — A home segue o protótipo aprovado.** Na ordem:
1. hero "Your AI coding agent ships fast. trackfw makes it ship traceably.";
2. o problema;
3. o desenho da cadeia ADR → REQ → ROADMAP → waves → barrier → PR, com os guards;
4. três cenários: dev solo com agente, time com CI, ambiente que só libera npm/pip;
5. o board do `trackfw serve`;
6. o demo em vídeo com controles;
7. os recursos;
8. o Quick start.

O Quick start usa as mesmas linhas do README, pinadas pelo teste.

**D2 — Imagens geradas pelo binário, por script versionado.** `scripts/site-images/generate.sh` roda os roteiros VHS
e a captura do board num ambiente fixo:
- HOME e projeto temporários;
- `PS1='$ '`;
- identidade git `Demo User` / `demo@example.invalid`;
- `TZ=UTC`;
- sem tokens no ambiente.

As imagens ficam versionadas em `site/public/img/`. O board usa só o painel Kanban, nunca o painel de cadeia. Nenhuma
imagem é feita à mão.

**D3 — A verificação de vazamento lê texto, não pixel.** `scripts/check-site-images-leak.sh` lê os bytes de cada
arquivo e também a transcrição em texto que o VHS grava de cada roteiro. Vale lembrar que o texto que aparece numa
imagem é pixel. O controle principal é o ambiente fixo; o scanner confirma. O autoteste usa canários que precisam
reprovar e passar.

**D4 — Diagrama estático, sem dependência nova.** A cadeia é HTML/CSS ou SVG inline no markdown do VitePress. Não
entra plugin de diagrama em tempo de execução nem dependência npm nova.

**D5 — Inglês na raiz e português espelhado.** A home e o guia em `/pt/` têm a mesma estrutura, com os textos
traduzidos; as imagens são as mesmas.

**D6 — O deploy separa permissões por job.** O job `build` do `.github/workflows/deploy-docs.yml` fica só com
`contents: read`; `pages: write` e `id-token: write` ficam só no job `deploy`.

**D7 — O demo no site é vídeo com controles.** Usa `<video controls>`, sem reprodução automática em loop (WCAG 2.2.2),
com pôster. O README continua com o `docs/demo.gif`.

**D8 — O protótipo é descartado.** `site/prototype.md` sai antes do PR final.

## Consequences

- **Positivo:** quem chega pelo site vê o problema, o fluxo e um caso parecido com o seu antes dos comandos. Toda
  imagem pode ser regenerada a partir do binário.
- **Negativo:** as imagens versionadas pesam no repositório (cerca de 1 MB) e precisam ser regeneradas quando a saída
  do CLI mudar. A regeneração é manual (`generate.sh`), porque os gates novos estão congelados.
- **Risco aceito:** o scanner não faz OCR. Uma imagem que mostre um dado vazado sem que o dado esteja no texto
  transcrito passaria. O ambiente fixo é o que impede esse dado de existir.

## Alternatives Considered

- **Mermaid ou plugin de diagrama no VitePress:** dependência nova e código em tempo de execução para um desenho que
  muda pouco. Rejeitada (D4).
- **Capturas e ilustrações feitas à mão:** desatualizam sem que ninguém perceba. Rejeitada a pedido do KG (D2).
- **Gerar as imagens no CI a cada release:** seria um job novo, com os gates congelados. Rejeitada por enquanto.
- **Tagline técnica no hero:** o KG escolheu o pitch do README (D1).
