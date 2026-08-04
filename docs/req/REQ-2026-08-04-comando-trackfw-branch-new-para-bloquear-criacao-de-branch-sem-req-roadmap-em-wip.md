---
status: Open
date: 2026-08-04
author: "kg.saran@gmail.com"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-04-comando-trackfw-branch-new-para-bloquear-criacao-de-branch-sem-req-roadmap-em-wip.md"
---

# REQ: comando trackfw branch new para bloquear criação de branch sem REQ+roadmap em wip

> Date: 2026-08-04 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation
<!-- Why is this requirement needed? What problem does it solve? -->
Hoje o `branch_has_wip_roadmap` (`internal/validator/validator.go`) só é avaliado quando alguém
lembra de rodar `trackfw validate` (ou em `trackfw ship`) — **depois** que a branch feat/fix/refactor
já foi criada e possivelmente já recebeu código. Não existe nada que impeça a criação da branch em
primeiro lugar. Isso foi observado ao vivo (REQ-2026-08-04-corrigir-dispatch-sem-subagent-type...):
o orquestrador criou `git checkout -b` antes de existir REQ+roadmap em `wip/`, e só foi pego porque um
subagente disciplinado (Prometeu) checou governança por conta própria antes de editar código — um
gate estrutural não teria dependido disso.

Motivação secundária, mas real: `git checkout -b` errado hoje custa um `git branch -m` de correção
(como aconteceu na sessão que motivou esta REQ) — evitável se o comando validar o slug antes de criar
a branch.

## Acceptance Criteria
- [ ] Novo comando `trackfw branch new <type>/<slug>` nos três CLIs (Go/Node/Python), `type` ∈
      `{feat, fix, refactor}` — mesmo vocabulário que `trackfw ship` já valida
      (`docs/cli-parity.md` § `trackfw ship`, passo 1: `feat|fix|refactor/<slug>`)
- [ ] Antes de criar a branch, reutiliza a **mesma** lógica de matching de slug já usada por
      `validateBranchHasWIPRoadmap` (`internal/validator/validator.go:1904-1964`) — normaliza o slug
      (`normalizeBranchSlug`) e verifica se algum `.md` em `wip/` ou `done/` (conforme `resolveWIPDirs`/
      `resolveDoneDirs`) contém esse slug normalizado. **Não duplicar a regra** — extrair para função
      compartilhada chamada tanto pelo validador quanto pelo novo comando, para nunca divergir do que
      `trackfw validate` aceita
- [ ] Se não houver roadmap casando o slug em `wip/` nem `done/`: comando falha, `git checkout -b`
      **não é executado**, mensagem de erro orienta o fluxo (`trackfw req new` → `trackfw roadmap new`
      → `trackfw roadmap move <nome> wip`) — mesmo texto de orientação que
      `validateBranchHasWIPRoadmap` já usa, para não ter duas mensagens diferentes para o mesmo problema
- [ ] Se houver match: executa `git checkout -b <type>/<slug>` e imprime a mesma confirmação que o Git
      já imprime (não reinventar saída)
- [ ] `--dry-run`: reporta se criaria a branch ou bloquearia, sem executar `git checkout`
- [ ] Exit code: 0 em sucesso, não-zero quando bloqueado por falta de governança, não-zero em erro de
      uso (tipo inválido, slug vazio, branch já existe — delega ao erro nativo do `git`)
- [ ] Comando documentado em `docs/cli-parity.md` (nova linha na tabela de comandos + seção própria,
      seguindo o padrão de `trackfw ship`/`trackfw barrier`)
- [ ] `trackfw help branch` funcional nos três CLIs
- [ ] Testes cobrindo: slug com match em `wip/`, slug com match em `done/`, sem match (bloqueia),
      `--dry-run` nos dois cenários, tipo inválido, branch já existente — replicados nos três runtimes
- [ ] Gate de paridade (`scripts/check-cli-parity.sh` ou equivalente) cobre o novo comando

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: N/A — extensão de um gate de validação já existente (`branch_has_wip_roadmap`) para um ponto de
execução mais cedo; não introduz novo modelo de governança.

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-04-comando-trackfw-branch-new-para-bloquear-criacao-de-branch-sem-req-roadmap-em-wip.md`
