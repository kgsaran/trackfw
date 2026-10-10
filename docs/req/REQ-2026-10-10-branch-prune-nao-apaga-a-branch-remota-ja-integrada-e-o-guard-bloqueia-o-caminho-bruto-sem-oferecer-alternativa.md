---
status: Open
date: 2026-10-10
author: "zeus-tf"
adr: ""
roadmap: "docs/roadmaps/backlog/ROADMAP-2026-10-10-branch-prune-nao-apaga-a-branch-remota-ja-integrada-e-o-guard-bloqueia-o-caminho-bruto-sem-oferecer-alternativa.md"
---

# REQ: branch prune não apaga a branch remota já integrada e o guard bloqueia o caminho bruto sem oferecer alternativa

> Date: 2026-10-10 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

A REQ-2026-08-18 deixou a remoção de branch remota fora do escopo do `branch prune`, com a ressalva
*"se for desejado, é REQ própria"*. A REQ-2026-10-03 manteve a exclusão. Esta é essa REQ.

**Medido em 2026-10-10**, após o merge do #554:

- `trackfw branch prune --apply` apagou a única branch local integrada, como previsto.
- Sobraram duas branches **remotas** integradas: `docs/bash-consome-stdout-de-python3-sem-normalizar-crlf`
  (`2d3a7dbb`) e `fix/criterio-de-adr-por-prefixo` (`99477f44`). O conteúdo das duas foi conferido
  linha a linha na `main` (0/59 e 0/7 linhas ausentes).
- `git push origin --delete <branch>` é **bloqueado pelo guard** (*"git push bruto bloqueado. Use
  `trackfw push` …"*), e **nenhum** comando do trackfw apaga ref remota — nem `push`, nem `ship`, nem
  `branch prune`. A mensagem do guard aponta para comandos que não fazem a operação.
- A única saída restante é a interface do forge ou uma chamada de API que contorna o guard. Foi o que
  restou ao usuário.

O resultado é uma ferramenta de governança que **fecha o caminho bruto sem abrir o governado**: a
operação continua necessária e passa a ser feita fora de qualquer trilho.

**Hipótese a medir, não fato:** as duas remotas sobreviveram ao *auto-delete* do GitHub porque um
commit foi empurrado **depois** do merge, recriando a ref. Se confirmada, é o mesmo padrão dos commits
pós-merge que o #554 teve de recuperar, e o `prune` deve tratar esse caso (`keep — commits after the
merged PR`) também no remoto.

## Acceptance Criteria

- [ ] **AC1** — 🔴 **Wave 0 (`hades-tf`):** threat model do que pode fazer a remoção remota **apagar
  trabalho não integrado ou de terceiro**: branch remota de outro autor; ref remota que avançou depois
  do `fetch` (TOCTOU — o tip mudou entre a avaliação e a remoção); PR aberto de mesmo nome; branch
  protegida; remoto que não é `origin`; resposta do forge truncada; ausência de `gh`. Parecer em
  `docs/seguranca/`.
- [ ] **AC2** — ADR registrando as decisões: flag de opt-in (proposta: `--remote`, sempre exigindo
  `--apply`), qual sinal autoriza apagar no remoto (proposta: **só** `delete` pelo PR mergeado contendo
  o tip — nunca a heurística de conteúdo), e como a remoção garante que o tip apagado é o tip avaliado
  (proposta: `--force-with-lease`-equivalente, `git push --delete` condicionado ao SHA esperado).
- [ ] **AC3** — Sem `--remote`, o comportamento é idêntico ao de hoje (nenhuma chamada que escreva no
  remoto). Sem `--apply`, `--remote` só relata.
- [ ] **AC4** — A remoção remota só ocorre para branch cujo veredito é `delete` pelo sinal do forge e
  cujo tip remoto é o mesmo SHA avaliado; qualquer divergência → não apaga e relata a causa.
- [ ] **AC5** — Branches `keep`/`review`, a branch padrão e branches protegidas nunca são apagadas no
  remoto, com teste por caso.
- [ ] **AC6** — A mensagem do guard para `git push --delete` / `git push <remote> :<ref>` aponta para o
  comando que faz a operação, em vez de para `trackfw push`/`ship`.
- [ ] **AC7** — Medição de volta neste repositório: dry-run com `--remote` lista as branches remotas
  integradas e nenhuma com trabalho não integrado.
- [ ] **AC8** — `docs/cli-parity.md` atualizado com a anotação `trackfw-contract`.
- [ ] **AC9** — Cada teste novo declara a conclusão que afirma e reprova sem a correção.
- [ ] **AC10** — `make quality` EXIT=0 (máquina ociosa) e CI verde, inclusive `windows-full-suites`.

## Negative scope

- **Remotos além de `origin`:** fora; degradam com mensagem.
- **Forges além do GitHub:** degradam como no ADR-2026-10-03 (D2).
- **Apagar remoto pela heurística de conteúdo:** fora. Sem sinal do forge, não apaga no remoto.
- **Liberar `git push --delete` bruto no guard:** fora. O caminho governado substitui, não convive.
- **Mudar o padrão `--dry-run`** ou a regra de que `review` nunca é apagado sozinho.
- **Configurar o auto-delete do repositório no forge:** é configuração do repositório, não do CLI.

## Linked ADR
<!-- A escrever na análise (AC2); estende o ADR-2026-10-03 -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/backlog/ROADMAP-2026-10-10-branch-prune-nao-apaga-a-branch-remota-ja-integrada-e-o-guard-bloqueia-o-caminho-bruto-sem-oferecer-alternativa.md
