---
status: Open
date: 2026-10-23
author: "zeus-tf (desenho) / KG (prioridade)"
adr: "docs/adr/ADR-2026-10-04-o-guard-de-hook-e-um-subcomando-go-do-trackfw-e-a-linha-de-hook-e-a-mesma-string-em-todo-shell.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-10-23-update-nao-remove-geracoes-antigas-do-credential-guard-em-pares-evento-matcher-e-o-validate-manda-rodar-update-que-nao-resolve.md"
---

# REQ: update não remove gerações antigas do credential guard em pares evento×matcher, e o validate manda rodar o update, que não resolve

> Date: 2026-10-23 | Status: Open
| Linear Issue:
| Jira Issue:
| GitHub Issue: #599

## Motivation

Issue #599, reproduzida num consumidor (claraea/ea-cmdb) com a 10.1.0. No `.cursor/hooks.json`, depois de `trackfw update`,
cada par `preToolUse`/`postToolUse` × `Read`/`Write` acumula 4 formas legadas do credential guard e mais a atual. O guard
roda 5 vezes por `Read`/`Write`. As formas legadas são justamente as que o `validate` descreve como falha aberta sem o
`trackfw` no PATH (#535), como passando saída ≠{0,2} no PowerShell e como abortando sob zsh. O `validate` avisa e manda
rodar o `update`, e o `update` não resolve. É a mesma família do #300.

**Causa medida pelo arquiteto (leitura do código, 2026-10-23):** em `InjectCursorHooks` (`internal/generators/agentfiles.go`),
as migrações de forma legada cobrem só `beforeShellExecution`/`afterShellExecution`. Os pares com matcher passam apenas por
`mergeCursorGuardMatcherEntry`, que acrescenta a forma atual quando não acha o par exato e não toca nas legadas. A lista de
gerações está copiada à mão em cada sítio de migração, e as cópias já divergem: o Codex migra a forma `scripts/…sh` só em
Bash/apply_patch, e o bloco do Gemini começa no D2. Vault: `vault/notes/inject-codex-gemini-d11rev-duplicata-antes-de-migracao-2026-10-21.md`
(mesma família: ordem entre migração e merge).

Decisões: ADR-2026-10-04, adendo de 2026-10-23 (D12 registro único de gerações, D13 uma entrada por par, D14 remoção
da legada de projeto com o guard global instalado).

## Acceptance Criteria

- [ ] **AC1 — Cursor, a issue.** Com a fixture das 5 gerações (as 4 da issue mais `scripts/trackfw-credential-guard.sh`)
  em `preToolUse`/`postToolUse` × `Read`/`Write`, nos dois estados (só legadas; legadas mais a atual), depois do
  `InjectCursorHooks` há **exatamente uma** entrada do guard por par, na forma atual. O validador real sobre o arquivo
  resultante dá **zero** avisos de `.cursor/hooks.json`. Contra-braço: o código de `origin/main` deixa as legadas, e o
  validador dá 4 avisos sobre a mesma fixture.
- [ ] **AC2 — Registro único (D12).** As formas legadas de cada guard (credential e git-branch) estão numa lista só,
  consumida pelo validador e por todo sítio de migração. Não sobra nenhuma cópia literal da lista: um grep pelas constantes
  `legacy*Cmd*` fora do registro e dos testes devolve só o uso por referência.
- [ ] **AC3 — Mesma causa nos outros sítios (Regra Dura).** A matriz CLI × escopo (projeto/global) × evento × matcher ×
  geração medida na Wave 0 está no roadmap, e **cada célula** em que o código de hoje deixa legada ou duplicata tem um
  teste que reprova no código de `origin/main` e passa no novo.
- [ ] **AC4 — Teste de concordância.** Um teste afirma que toda forma que o validador classifica como legada é migrada por
  todo sítio que emite aquele guard. Ele reprova quando uma geração é acrescentada ao registro sem que algum sítio a
  receba, e isso é verificado por falsificação.
- [ ] **AC5 — Hooks do usuário intactos.** Uma entrada que não está no registro (comando personalizado, inclusive um que
  só *contém* `trackfw guard credential`), no mesmo par evento×matcher, sobrevive à migração byte a byte.
- [ ] **AC6 — Guard global (D14 revista, por par).** Num par (evento, matcher) que o guard global do CLI cobre, a entrada de
  projeto reconhecida pelo registro é removida. Num par que o global não cobre (por exemplo Cursor `preToolUse`×`Read`),
  ela é migrada e continua emitida, com ou sem global. Os três braços têm teste.
- [ ] **AC7 — Idempotência.** Rodar o `update` duas vezes produz o mesmo arquivo da primeira, byte a byte, e a segunda
  rodada não reporta mudança.
- [ ] **AC8 — Fechamento.** `make quality` rc=0, CI verde, e o PR leva `Closes #599`.

## Negative scope

- **Não muda a forma atual da linha de hook** (`emulate -R sh …`). Só a migração das formas antigas.
- **Não muda o hook de atenção**, que já migra por `dedupAttentionSimple`, a não ser que a Wave 0 meça nele a mesma causa
  (aí entra como ML, pela Regra Dura).
- **Não cria comando novo**, nem flag nova no `update`.
- **Não reescreve arquivos globais do usuário** fora do que o `update harness` já escreve hoje.
- **Não regenera o `~/.cursor` do KG nem o do consumidor.** Isso fica para depois da release, com o `update`.

## Linked ADR
ADR: `docs/adr/ADR-2026-10-04-o-guard-de-hook-e-um-subcomando-go-do-trackfw-e-a-linha-de-hook-e-a-mesma-string-em-todo-shell.md` (adendo 2026-10-23, D12–D14)

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/wip/ROADMAP-2026-10-23-update-nao-remove-geracoes-antigas-do-credential-guard-em-pares-evento-matcher-e-o-validate-manda-rodar-update-que-nao-resolve.md
