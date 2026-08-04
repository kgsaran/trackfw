---
status: Open
date: 2026-08-04
author: "Zeus"
adr: "docs/adr/ADR-2026-08-04-make-quality-forca-locale-fixo-no-gate-de-falsificacao-em-vez-de-pin-em-ingles.md"
roadmap: "docs/roadmaps/backlog/ROADMAP-2026-08-04-make-quality-locale-fixo-no-falsify.md"
---

# REQ: make quality falha sob locale pt_BR — teste fixa literal em inglês No violations found

> Date: 2026-08-04 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

Achado durante a auditoria de conformidade da Wave 2 do roadmap
`ROADMAP-2026-08-04-req-move-list-subpastas-e-move-fisico.md` (agente ML-2A): `make quality`, rodado na
máquina do desenvolvedor sob locale padrão `pt_BR.UTF-8`, falha no Cenário 29 de
`scripts/check-gates-falsify.sh`. Reproduzido numa árvore limpa (stash de todas as mudanças da Wave 1/2),
confirmando que é pré-existente e não relacionado a essa correção.

Causa raiz: o Cenário 29 (linha ~1947 de `scripts/check-gates-falsify.sh`) pina byte-a-byte a mensagem de
sucesso do `validate` sem violações contra o literal em inglês hardcoded
`S29_EXPECTED=$'\xe2\x9c\x93 No violations found.\n'` ("✓ No violations found."). Os 3 CLIs, porém,
imprimem essa mensagem via `i18n_t("validate.ok")` (`internal/i18n/locales/en-US.json:49`), que resolve
para a tradução do locale ativo do processo — sob `pt_BR.UTF-8`, a mensagem real impressa é a tradução
em português, não o literal em inglês pinado. O gate reprova comparando a saída real (localizada) contra
uma expectativa fixa em um único idioma.

O próprio comentário do Cenário 29 explica sua motivação original: "Nada em CI garantia isto até agora —
foi exatamente por não haver gate que o Python ficou meses imprimindo o literal hardcoded '✓ Governance
OK' em vez da chave `validate.ok` do i18n... Um diff três-a-três puro (sem pin) passaria mesmo se os 3
imprimissem a mesma coisa errada... por isso o baseline também compara contra o literal esperado pinado".
O pin é necessário para a prova de regressão que o cenário existe para fazer — mas hoje ele está pinado a
um locale específico (inglês), quando deveria pinar a paridade entre os 3 CLIs **dentro do locale ativo**,
ou forçar explicitamente `LANG=en_US.UTF-8`/`LC_ALL=en_US.UTF-8` antes de rodar os 3 binários no próprio
script — hoje quem herda o locale do ambiente do desenvolvedor/CI, e diverge silenciosamente conforme a
máquina.

### Por que importa

- `make quality` é o gate de qualidade oficial do projeto (`CLAUDE.md`: "Build obrigatório após qualquer
  alteração"). Falhar de forma dependente de locale do desenvolvedor é uma fonte de falso-negativo/
  falso-positivo que mina a confiança no gate — quem roda numa máquina `pt_BR` (comum no time) vê falha
  sem ter introduzido regressão real.
- CI provavelmente roda com locale `C`/`en_US` por padrão em muitos runners, o que mascara o problema lá
  e faz com que só apareça localmente — dificultando diagnóstico ("passa no CI, falha na minha máquina").

## Acceptance Criteria
- [ ] AC1 — `scripts/check-gates-falsify.sh` Cenário 29 passa de forma determinística independente do
      locale do ambiente onde é executado (testar explicitamente sob `pt_BR.UTF-8` e `en_US.UTF-8`).
- [ ] AC2 — A correção preserva o propósito original do cenário: detectar regressão de um CLI voltando a
      imprimir uma mensagem hardcoded divergente dos outros dois (a prova de detecção do Python reintroduzindo
      `"✓ Governance OK"` continua reprovando corretamente).
- [ ] AC3 — Decisão de design explícita (documentar no commit ou ADR leve, se a mudança não for trivial):
      (a) o script força `LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8` nos 3 subprocessos que compara, ou
      (b) o script lê a mensagem esperada dinamicamente de `internal/i18n/locales/en-US.json`/equivalente
      no locale ativo em vez de hardcode, ou (c) outra abordagem equivalente — qualquer uma que elimine a
      dependência do locale do processo pai.
- [ ] AC4 — `make quality` roda verde em uma máquina com `LANG=pt_BR.UTF-8` sem exigir que o
      desenvolvedor troque o locale manualmente antes de rodar o comando.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: `docs/adr/ADR-2026-08-04-make-quality-forca-locale-fixo-no-gate-de-falsificacao-em-vez-de-pin-em-ingles.md`

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: `docs/roadmaps/backlog/ROADMAP-2026-08-04-make-quality-locale-fixo-no-falsify.md`
