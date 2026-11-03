---
status: Open
date: 2026-11-02
author: "Zeus (trackfw_architect)"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-11-02-relatos-do-primeiro-usuario-externo-init-pela-metade-e-npm-em-projeto-python-mensagem-de-fail-open-wip-limit-ignorado-e-adr-req-vazios.md"
---

# REQ: Relatos do primeiro usuário externo: init pela metade e npm em projeto Python, mensagem de fail open, wip_limit ignorado e ADR/REQ vazios

> Date: 2026-11-02 | Status: Open
| Linear Issue: 
| Jira Issue: 
| GitHub Issues: #657, #658, #659, #660, #661

## Motivation

Os cinco issues foram abertos em 2026-11-02 por `bruno-castiglioni`, o primeiro usuário externo em meses,
logo depois do primeiro post da série no LinkedIn. O ambiente é trackfw 11.3.0 (npm, win32-x64) no
Windows 11, com Claude Code e Git Bash. Todos trazem medição e contra-braço.

**Uma REQ por objetivo, com uma causa por ML.** As causas são diferentes e estão escritas abaixo,
uma por AC. Elas ficam numa REQ só porque o `trackfw barrier` compara o diff da branch inteira com os
arquivos de um roadmap: com um roadmap por causa na mesma branch, cada barrier acusaria os arquivos dos
outros (medido no PR #655 com os arquivos do bump de versão).

### Causas, conferidas pelo arquiteto no código

- **#658 — mensagem invertida.** `internal/validator/validator_guard_binary_probe.go:308` (binário ausente)
  e `:321` (binário sem `guard`) dizem "the hook will fail open". A linha de hook gerada normaliza
  qualquer saída diferente de zero para 2 (`LASTEXITCODE=$((2*!!$?))`), então nesses dois casos o hook
  **falha fechado**: bloqueia todo comando, até `git status`. O Bruno mediu exit 2 no Git Bash e no
  PowerShell 5.1, e exit 1 no cmd.exe (limitação já documentada). As mensagens de política Restricted
  (`:428` e `:435`) descrevem um fail-open real e ficam como estão.
- **#660 — severidade configurada ignorada.** `validateWIPLimit` (`internal/validator/validator.go`, por
  volta das linhas 689 e 707) põe o limite estourado em `warnings` de forma fixa. O `applyRule("wip_limit", ...)`
  só recebe a outra lista, então `rules.wip_limit: error` não tem efeito nesse caso. O padrão em
  `internal/config/config.go:402` é `error` e o README (linha ~480) diz "one at a time". Reproduzido pelo
  arquiteto com a 11.3.0: o segundo `roadmap move ... wip` sai 0 e o `validate` emite só
  `⚠ 2 roadmaps in wip/ (limit: 1)`.
- **#661 — npm fixo sem TTY.** Sem TTY, o `trackfw init` monta a config com `PkgManager: "npm"` fixo
  (`internal/commands/init.go:130`). A detecção de stack em `internal/discover` não é usada.
- **#657 — init pela metade.** O `init` escreve os arquivos do projeto e só depois instala as AI tools no
  escopo global (`installAITools(aiTools, cwd, "global")`, `init.go:144`). Quando `~/.claude` é uma
  junction, a escrita global é recusada (proteção de reparse point, que fica) e o projeto fica pela metade.
  Sem TTY também não há como escolher o escopo do projeto: o `--scope` existe no `agents install`, mas não
  no `init`.
- **#659 — ADR e REQ vazios passam.** Nenhuma regra olha o conteúdo; só o vínculo é verificado. Decisão do
  KG em 2026-11-02: entra agora, como **aviso**. A direção de 2026-10-28 congelou gates novos "até existir
  um segundo consumidor", e esse consumidor existe e pediu.

## Acceptance Criteria

- [ ] **AC1 (#658).** As mensagens de binário ausente e de binário sem `guard` (`validator_guard_binary_probe.go`,
  linhas ~308 e ~321) e as duas equivalentes da sonda do Git Bash (~511 e ~543) deixam de afirmar só
  "fail open" e passam a dizer, **sempre**, as duas verdades medidas:
  - na forma atual PowerShell/POSIX, o hook **bloqueia todo comando** (exit 2);
  - nas formas legadas e nos hooks de família cmd.exe, ele pode deixar o comando passar (fail open).

  Em seguida vem a correção (`install trackfw` / `trackfw update`). Teste afirma as duas cláusulas.
  - *Emenda do arquiteto em 2026-11-02, na auditoria da Wave 0.* A Wave 0 mostrou que o "fail open" está
    certo para o D2-legado (o bash sai 127) e não foi medido nos hooks cmd.exe (Kiro e Amazon Q). Ela
    propôs uma mensagem condicional, com dois booleanos novos no loop de detecção e o cache da sonda
    ampliado. A mensagem incondicional com as duas cláusulas é exata em qualquer configuração e só muda
    texto. As mensagens de política Restricted não mudam.
- [ ] **AC2 (#660).** O limite estourado passa por `applyRule("wip_limit", ...)` e respeita a severidade
  configurada. O padrão muda para `warning`, por decisão do KG, para não passar a bloquear quem já tem mais
  de um roadmap em `wip/`. Com `rules.wip_limit: error`, o `validate` reprova. O README deixa de dizer
  "one at a time" sem ressalva e explica o limite e a severidade.
- [ ] **AC3 (#661).** Sem TTY, o `init` não grava `pkg_manager: npm` num projeto sem `package.json`. Ele usa
  a detecção existente, ou deixa vazio quando não há sinal. Um projeto com `package.json` continua npm.
- [ ] **AC4 (#657).**
  - Antes de escrever qualquer arquivo do projeto, o `init` confere se o destino das AI tools aceita a
    escrita. Se não aceitar, aborta sem alterar o projeto e diz como seguir (`--scope project`, ou
    `trackfw agents install --scope project` depois).
  - O `init` ganha `--scope project|global`, com padrão `global` como hoje.
  - A recusa de reparse point continua.
- [ ] **AC5 (#659).** Regra nova, com nome a definir na Wave 0 e severidade padrão `warning`: ADR ou REQ
  cujas seções obrigatórias só têm o comentário do modelo (`<!-- ... -->`) ou estão vazias geram aviso.
  ADR e REQ preenchidos não geram aviso. A regra é configurável como as outras e não vira exigência sobre
  seções opcionais.
- [ ] **AC6.** Cada issue recebe comentário com o que mudou e a versão. O PR fecha os cinco com `Closes`.

## Negative scope

- Não muda a semântica dos guards nem a linha de hook gerada.
- Não remove a recusa de reparse point (#657): só a antecipa e oferece alternativa.
- Não transforma a regra do AC5 em violação por padrão.
- Não reescreve a detecção de stack do `internal/discover`: só a usa no `init`.
- Não trata o cmd.exe saindo 1 (#658), que é limitação já documentada.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-11-02-relatos-do-primeiro-usuario-externo-init-pela-metade-e-npm-em-projeto-python-mensagem-de-fail-open-wip-limit-ignorado-e-adr-req-vazios.md
