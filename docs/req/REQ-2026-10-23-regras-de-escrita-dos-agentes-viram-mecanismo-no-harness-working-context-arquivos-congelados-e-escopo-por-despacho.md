---
status: Done
date: 2026-10-23
author: "zeus-tf (desenho) / KG (decisão)"
adr: "docs/adr/ADR-2026-10-23-regras-de-escrita-dos-agentes-sao-aplicadas-pelo-harness-e-nao-pelo-texto-do-handoff.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-10-23-regras-de-escrita-dos-agentes-viram-mecanismo-no-harness-working-context-arquivos-congelados-e-escopo-por-despacho.md"
---

# REQ: as regras de escrita dos agentes viram mecanismo no harness (working context, arquivos congelados e escopo por despacho)

> Date: 2026-10-23 | Status: Done
| Linear Issue:
| Jira Issue:

## Motivation

Os subagentes desrespeitam regras de escrita que só existem como texto. Os casos medidos estão na ADR: 5 de 9 em
2026-10-12, a escrita no checkout principal em 2026-10-21 e a escrita no `.md` congelado em 2026-10-23. O KG relata o
mesmo em outros projetos. A instrução do próprio arquivo de agente (`## Working context`, "automatically, without asking")
vence a proibição do handoff, e nenhum controle mecânico segura a escrita.

Decisão do KG (2026-10-23): uma REQ só, na ordem D1 → D2 → D3 → D4 da ADR.

## Acceptance Criteria

- [x] **AC1 — D1, a instrução.** Nos 11 assets de agente que não são o arquiteto, a seção `## Working context` passa a
  mandar reportar no relatório e proíbe escrever no working context. O asset do arquiteto mantém a escrita. Os goldens
  mudam junto, e um teste afirma as duas formas por papel.
- [x] **AC2 — D2, o barrier.** O `trackfw barrier <roadmap> --wave N` tem a checagem `ml_scope`. Um arquivo tocado na wave
  que não está em nenhum `Files affected` dos MLs dela, nem no conjunto de orquestração, reprova com o caminho e o ML.
  Há braços positivo e negativo testados num repositório em `t.TempDir()`. Roadmap sem `Files affected` segue a regra que
  a Wave 0 definir, com teste.
- [x] **AC3 — D3, os congelados.** O `trackfw guard write-scope` nega (exit 2) uma escrita em `docs/agents-working-context.md`
  pelo payload real de `Edit`/`Write` do Claude Code, e libera uma escrita em `docs/agents-working-context/<arquivo>.md`. A
  lista é extensível pelo `trackfw.yaml`. O `init` e o `update` cabeiam o hook nos CLIs que a Wave 0 listar, e o `validate`
  detecta a fiação ausente.
- [x] **AC4 — D4, o escopo por despacho.** Com uma permissão para `agent_type=apolo-tf` autorizando o worktree W, uma escrita
  do apolo-tf fora de W é negada e uma dentro de W é liberada. Sem permissão, vale só a D3. A permissão em si é congelada
  para todo agente que não é o arquiteto. Os payloads de teste seguem o formato documentado do Claude Code.
- [x] **AC5 — Falha do guard.** Os casos de payload ilegível, caminho relativo, symlink, `..`, caminho do Windows e worktree
  dentro de `/private/tmp` versus `/tmp` têm o comportamento definido pela Wave 0 e testado, sem fail-open por acidente.
- [x] **AC6 — Fechamento.** `make quality` rc=0, CI verde e `docs/cli-parity.md` atualizado com o contrato do `guard write-scope`.

## Negative scope

- **Não** intercepta escrita por Bash nas D3 e D4. O resíduo fica declarado e só a D2 pega, salvo se a Wave 0 medir que o
  parser do credential guard cobre os casos comuns.
- **Não** muda a autoridade git, que continua só com o arquiteto.
- **Não** cria comando para editar a permissão: o arquiteto grava o arquivo, num formato documentado.
- **Não** reescreve os arquivos globais de agente do usuário (`~/.claude/agents`). Isso fica com o `trackfw agents update`
  depois da release.
- **Não** muda o comportamento da sessão principal fora dos caminhos congelados.

## Linked ADR
ADR: `docs/adr/ADR-2026-10-23-regras-de-escrita-dos-agentes-sao-aplicadas-pelo-harness-e-nao-pelo-texto-do-handoff.md`

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/done/ROADMAP-2026-10-23-regras-de-escrita-dos-agentes-viram-mecanismo-no-harness-working-context-arquivos-congelados-e-escopo-por-despacho.md

## Encerramento (2026-10-24)

Mergeado no #606 (7bddcd34). O AC3 (fiação) está entregue no gerador (`InjectClaudeHooks`) e no `validate` (`write_scope_not_wired`). A fiação do `.claude/settings.json` **deste repositório** fica para depois da release e da instalação do binário, porque o 10.1.0 trata `guard write-scope` como subcomando desconhecido e bloquearia toda escrita (contrato do ML-3A, item 8 revisto).
