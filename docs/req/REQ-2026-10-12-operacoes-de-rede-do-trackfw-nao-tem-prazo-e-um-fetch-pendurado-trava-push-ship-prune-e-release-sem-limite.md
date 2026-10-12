---
status: Open
date: 2026-10-12
author: "zeus-tf"
adr: "docs/adr/ADR-2026-10-12-operacao-de-rede-do-trackfw-tem-prazo-e-o-estouro-nunca-vira-sucesso-silencioso.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-10-12-operacoes-de-rede-do-trackfw-nao-tem-prazo-e-um-fetch-pendurado-trava-push-ship-prune-e-release-sem-limite.md"
---

# REQ: operações de rede do trackfw não têm prazo e um fetch pendurado trava push, ship, prune e release sem limite

> Date: 2026-10-12 | Status: Open
| Issue: #556

## Motivation

**Medido em 2026-10-10** (#556): um `trackfw push` ficou parado **70 min** em `git fetch origin
--prune`, porque o `ssh` neto ficou pendurado. Com o `ssh` encerrado à mão, o push concluiu
normalmente. O fetch é um passo **opcional** cujo próprio comentário diz *"non-blocking"*. Mesmo
assim, ele bloqueou o comando inteiro, sem limite.

A causa é que os executores de subprocesso de produção (`defaultGitExec`, `defaultGhExec` e
equivalentes) usam `exec.Command` sem prazo. Uma falha **rápida** é tratada; um **travamento**, não.

O mesmo mecanismo aparece em todos os pontos de rede: `push`, `ship`, `branch prune` e `release tag`,
tanto no `git` quanto no `gh`. O `release.go:310` ainda **descarta** o erro do `ls-remote`, o que
transforma "não consegui perguntar" em "a tag não existe no remoto".

## Acceptance Criteria

- [ ] **AC1** — 🔴 **Wave 0 (`hades-tf`):** parecer em `docs/seguranca/` cobrindo:
  - a população **fechada** de operações de rede, por grep dos executores e não só dos literais de
    subcomando;
  - a natureza do limite por operação (total × ocioso), **medida**;
  - se o `WaitDelay` e o grupo de processos encerram o neto `ssh` em Unix, e o que sobra no Windows;
  - o valor padrão a partir de medição;
  - os alvos de falsificação nas duas direções.
- [ ] **AC2** — Nenhuma operação de rede não interativa da população pode bloquear além de prazo +
  `WaitDelay`. A prova é um teste que roda o **executor de produção** contra um neto pendurado. Ele
  passa com a correção e **trava ou reprova** sem o `WaitDelay`. Roda no CI de Linux, macOS e Windows.
- [ ] **AC3** — No estouro, cada ponto segue o caminho de falha declarado no ADR (D1): aviso no
  `push`/`ship`; dado velho declarado no `prune` (local); falha fechada no `release tag`; degradação D2
  no forge. Há um teste por ponto.
- [ ] **AC4** — `release.go:310`: falha ou estouro do `ls-remote` → falha fechada nomeada. Nunca
  "tag ausente".
- [ ] **AC5** — A mensagem de estouro nomeia a operação, o prazo e a variável (`TRACKFW_NETWORK_TIMEOUT`
  ou `TRACKFW_PUSH_TIMEOUT`, ADR D7/A1). Valor inválido (`0`, negativo, > 24 h) gera aviso e usa o
  padrão. Estouro em escrita diz que o estado remoto é incerto.
- [ ] **AC6** — Operações interativas (`gh pr create` com stdin herdado) não ganham prazo, com teste
  que afirma isso.
- [ ] **AC7** — O `git push` do passo 4 tem o limite que o Wave 0 decidir, ou a razão escrita de não
  ter. "Fora do escopo" não é razão: a causa é a mesma.
- [ ] **AC7b** — `internal/sync/linear.go:76`: cliente HTTP com prazo (ADR D7/A10), com teste
  contra servidor que não responde.
- [ ] **AC8** — `docs/cli-parity.md` atualizado, com a anotação `trackfw-contract`.
- [ ] **AC9** — Cada teste novo declara a conclusão que afirma e reprova sem a correção.
- [ ] **AC10** — `make quality` EXIT=0 e CI verde, inclusive `windows-full-suites`.

## Negative scope

- **Chave no `trackfw.yaml` para o prazo:** fora. As variáveis de ambiente bastam (ADR D4/D7).
- **Retry automático** em operação que estourou: fora. O prazo só garante o retorno.
- **Mudar o que o `push`/`ship` fazem quando o fetch falha** (continuar com aviso): fora. Só o
  travamento vira falha.
- **Subprocessos locais** (`git rev-parse`, `git show`, `git ls-tree`, `git commit`) sem rede: fora.
- **Hooks de terceiro executados pelo git** (`pre-push`): só se o Wave 0 medir que herdam o prazo de
  forma danosa.

## Linked ADR
ADR: docs/adr/ADR-2026-10-12-operacao-de-rede-do-trackfw-tem-prazo-e-o-estouro-nunca-vira-sucesso-silencioso.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/wip/ROADMAP-2026-10-12-operacoes-de-rede-do-trackfw-nao-tem-prazo-e-um-fetch-pendurado-trava-push-ship-prune-e-release-sem-limite.md
