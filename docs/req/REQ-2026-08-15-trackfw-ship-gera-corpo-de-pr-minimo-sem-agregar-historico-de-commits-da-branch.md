---
status: Open
date: 2026-08-15
author: ""
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-15-trackfw-ship-gera-corpo-de-pr-minimo-sem-agregar-historico-de-commits-da-branch.md"
---

# REQ: trackfw ship gera corpo de PR minimo sem agregar historico de commits da branch

> Date: 2026-08-15 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation
<!-- Why is this requirement needed? What problem does it solve? -->
Confirmado no código: `internal/commands/ship.go` — `title := firstLine(opts.message)`
(só a primeira linha da mensagem `-m` passada NAQUELE invocação de `trackfw ship`) e
`body := buildPRBody(branch)`:

```go
func buildPRBody(branch string) string {
    return fmt.Sprintf("Branch: %s\n\nCreated by trackfw ship.", branch)
}
```

Isso ignora completamente o histórico de commits acumulado na branch antes do `ship` ser
chamado. Numa branch de trabalho real (ex: `ROADMAP-2026-08-14-bloqueio-tecnico-...`,
19 commits ao longo de várias waves/MLs), o PR aberto por `trackfw ship` teve título e
corpo praticamente vazios — descrevendo só o último commit, não o trabalho todo — e
precisou ser corrigido manualmente via `gh pr edit` depois de aberto. Achado real: nada
no `ADR-2026-07-26-trackfw-ship-agnostico-de-forge.md` documenta essa decisão de corpo
mínimo como deliberada — é lacuna de implementação, não design intencional.

O fluxo manual de abertura de PR já documentado no harness global (seção "Creating pull
requests" do CLAUDE.md do usuário) já descreve o padrão esperado: título curto (<70
caracteres), corpo com `## Summary` (bullets) + `## Test plan` (checklist), analisando
TODOS os commits desde a divergência da branch base — não só o commit mais recente.
`trackfw ship` deveria produzir algo equivalente automaticamente, sem exigir correção
manual pós-hoc.

**Achado adicional (2026-08-15, pós-merge do PR #169):** com o `git-branch-guard` (REQ
irmã `REQ-2026-08-14-bloqueio-tecnico-...`) agora bloqueando `git push` bruto em todos os
runtimes, `trackfw ship` passou a ser o **único** caminho de push — inclusive para
commits puramente doc-only (ex.: criar um novo REQ+roadmap em `backlog/`). Reproduzido ao
vivo nesta sessão tentando commitar `REQ-2026-08-15-trackfw-commit-sugere-mensagem-...`;
dois gates concretos bloqueiam, confirmados por teste direto do binário:

1. **`isShipBranch`** (`internal/commands/ship.go`) — exige `feat|fix|refactor/<slug>`
   sem exceção. Uma branch nomeada `docs/<slug>` (convenção já usada nesta sessão antes
   do guard existir, para trabalho puramente de governança) é rejeitada de cara, antes
   de qualquer checagem de conteúdo.
2. **`CheckShipGovernance`/`validateBranchHasWIPRoadmap`** (`internal/validator/validator.go`)
   — mesmo numa branch `feat/<slug>` corretamente nomeada, exige roadmap correspondente
   em `wip/`; confirmado por teste direto (`trackfw ship --dry-run`) que falha com
   "no matching roadmap in wip/ nor done/" quando nada está em `wip/`.

Nenhum dos dois tem exceção para mudanças doc-only — mesmo que o próprio CLAUDE.md (§7)
já isente explicitamente "Alteração doc-only (markdown, comentários)" da exigência de
REQ+roadmap. Resultado prático: ficou impossível empurrar um REQ+roadmap novo para
`backlog/` sem antes mover ALGUM roadmap para `wip/`.

## Acceptance Criteria
- [ ] `buildPRBody` (Go) e equivalentes (Node/Python) passam a agregar
      `git log <base>..HEAD --no-merges` (mensagens completas, não só a primeira linha)
      numa seção `## Commits` ou similar, em vez de só "Branch: %s\n\nCreated by trackfw
      ship."
- [ ] Título do PR: se houver só 1 commit não-merge na branch, mantém o comportamento
      atual (`firstLine` da mensagem do commit). Se houver mais de 1, usar um título que
      identifique o conjunto do trabalho — decisão de design a resolver no ML: pode ser
      a mensagem `-m` passada na chamada atual de `ship` (assumindo que é a mensagem
      "resumo" do PR, convenção já usada nesta sessão) ou derivar de outra fonte; **não
      travar a REQ nisso — abrir com uma opção razoável e documentar a escolha**.
- [ ] Resolução de branch base para o `git log <base>..HEAD`: usar
      `git symbolic-ref refs/remotes/origin/HEAD` com fallback para `main`, mesmo padrão
      já usado em outros pontos do código (`internal/validator/validator.go`,
      `internal/commands/commit.go`) — não hardcodar `main`.
- [ ] Comportamento idêntico nos 3 CLIs (mesmo corpo de PR gerado para o mesmo histórico
      de commits) — mensagens/formatação byte-idênticas onde aplicável.
- [ ] Não quebra o design forge-agnóstico do `ADR-2026-07-26-trackfw-ship-agnostico-de-forge.md`
      — o corpo continua sendo texto/markdown simples, agnóstico de forge; só o conteúdo
      muda, não o mecanismo de flag (`--body`/`--description` por forge já existente).
- [ ] `--dry-run` continua funcionando e mostra o corpo/título que seria usado.
- [ ] `make quality` passa sem novas divergências de paridade.
- [ ] Teste de regressão cobrindo o caso real que motivou esta REQ: branch com múltiplos
      commits não-merge, `ship` chamado no commit final — corpo do PR deve conter
      referência a todos os commits, não só o último.
- [ ] `trackfw ship` ganha exceção para commits doc-only, cobrindo os 2 gates
      identificados — coerente com a isenção já existente no CLAUDE.md §7 ("Alteração
      doc-only (markdown, comentários)" dispensa REQ+roadmap). Critério objetivo de
      detecção: se **todos** os arquivos staged (`git diff --cached --name-only`)
      estiverem sob `docs/`/`vault/` ou tiverem extensão `.md`, então:
      (a) `isShipBranch` deixa de exigir prefixo `feat/fix/refactor` — aceita qualquer
      nome de branch nesse caso (ex.: `docs/<slug>`);
      (b) `CheckShipGovernance`/`validateBranchHasWIPRoadmap` é dispensado — não exige
      roadmap correspondente em `wip/`.
      Mudança em qualquer arquivo de código (mesmo 1 arquivo fora desse critério) mantém
      os dois gates atuais, sem exceção — a detecção é "tudo staged é doc" ou nada.
- [ ] Teste de regressão cobrindo o caso real, os 2 gates: (1) branch `docs/<slug>` com
      só arquivos `docs/`/`.md`/`vault/` staged — `ship` deve permitir; (2) branch
      `feat/<slug>` sem roadmap em `wip/`, com só arquivos doc-only staged — `ship` deve
      permitir; (3) qualquer uma das duas branches acima com pelo menos 1 arquivo de
      código staged junto — `ship` deve continuar bloqueando exatamente como hoje.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-08-15-trackfw-ship-gera-corpo-de-pr-minimo-sem-agregar-historico-de-commits-da-branch.md
