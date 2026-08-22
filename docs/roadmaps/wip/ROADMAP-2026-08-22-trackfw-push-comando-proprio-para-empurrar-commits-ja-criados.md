---
status: wip
date: 2026-08-22
req: "docs/req/REQ-2026-08-22-trackfw-push-comando-proprio-para-empurrar-commits-ja-criados.md"
adr: "docs/adr/ADR-2026-08-22-comandos-de-entrega-separados-push-proprio-e-ship-como-composicao.md"
squad: "apolo-tf, hefesto-tf, hades-tf"
---

# Roadmap: `trackfw push` — comando próprio para empurrar commits já criados

> Created: 2026-08-22 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-22-trackfw-push-comando-proprio-para-empurrar-commits-ja-criados.md`
ADR: `docs/adr/ADR-2026-08-22-comandos-de-entrega-separados-push-proprio-e-ship-como-composicao.md`

Commit já criado não tem saída sancionada: `git push` é bloqueado pelo guard e o `ship` recusa com
"nothing is staged". `trackfw push` fecha o ciclo `commit → push` **reusando** os gates do `ship`,
sem gate novo e sem relaxar nada do que já existe.

## Acceptance Criteria

- [ ] AC1–AC11 da REQ, integralmente
- [ ] `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` exit 0 (invocação CI-exata)
- [ ] `./bin/trackfw validate` sem violations novas

## O que decide o desenho: `push` não inventa regra

Todo comportamento de `push` já existe dentro do `ship`. **Reuso, não reimplementação** — o mapa está
no ADR, com `arquivo:linha` por stack. Qualquer lógica reescrita à mão vira divergência entre os 3
runtimes na primeira mudança do `ship`.

## Riscos que valem para todos os MLs

1. **Paridade byte-a-byte é o critério, não "funciona nos 3"** — nove divergências reais em séries
   anteriores. Comparar as saídas dos três runtimes antes de escrever o gate.
2. **Invocação CI-exata:** `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality`. Essa variável **não**
   desliga o push — só lookups de PATH de forge/discover.
3. **Anotação `<!-- trackfw-contract -->`** obrigatória: `check-parity-contract-coverage.sh` é
   bloqueante.
4. **Não regredir o `ship`.** Ele é a fonte dos helpers reusados; mudar assinatura sem rodar
   `check-ship-parity.sh` e `check-ship-force-parity.sh` quebra o comando principal do trilho.
5. Commits, branch e PR são exclusivos do `trackfw_architect`. Entregue o trabalho **sem commitar**.

---

## Wave 1 — O comando

> Dependências: nenhuma.
> **ML único e não paralelizável de propósito:** os 3 stacks precisam sair byte-idênticos. Dividir
> por linguagem é exatamente o que produziu as divergências anteriores.

### ML-1A — `trackfw push` nos 3 CLIs
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)

**Arquivos afetados:**
- Go: `internal/commands/push.go` (novo), `internal/commands/root.go` (registro do comando),
  `internal/commands/push_test.go` (novo)
- Node.js: `npm/src/push/runner.js` (novo), `npm/src/commands/push.js` (novo), registro no
  entrypoint de comandos do `npm/src/`
- Python: `pypi/trackfw/push/runner.py` (novo), `pypi/trackfw/commands/push.py` (novo), registro no
  dispatcher do `pypi/trackfw/`
- **Proibido tocar:** `internal/commands/ship.go`, `npm/src/ship/runner.js`,
  `pypi/trackfw/ship/runner.py` — **exceto** para tornar reusável um símbolo privado (ver abaixo),
  nunca para alterar comportamento.

**Ações:**
1. Implementar `trackfw push` reusando, nesta ordem:
   - bloqueio incondicional em `main`/`master` (mesma mensagem do `ship`);
   - `isShipBranch` / `isGatedShipBranch` (Go `ship.go:729,736`; Node `runner.js:101,112`;
     Python `runner.py:99,110`);
   - `CheckShipGovernance` (Go `internal/validator/validator.go:2292`; Node `runner.js:156`;
     Python `runner.py:165`), com a isenção `chore`/`docs` intacta;
   - `detectPendingSquashMerges` (advisory) — Go `ship.go:771`, Node `runner.js:211`,
     Python `runner.py:189`;
   - `buildPushArgs` (Go `ship.go:795`, Node `runner.js:234`, Python `_build_push_args:225`);
   - gate de `--force-with-lease` (exige PR/MR aberto) — Go `ship.go:318`, Node `runner.js:516`,
     Python `runner.py:524`.
2. Flags: `--dry-run`, `--force-with-lease`. **Sem** `-m`, **sem** `--no-pr`, **sem** `--forge`.
3. **Python:** `_build_push_args`, `_detect_pending_squash_merges` são privados. Renomeie para
   público (`build_push_args`, `detect_pending_squash_merges`) mantendo alias privado onde já é
   referenciado, ou importe explicitamente. Escolha uma via e registre no parecer — é a única
   assimetria conhecida entre os stacks.
4. Testes unitários por stack cobrindo: sem upstream (`-u` presente), com upstream (`-u` ausente),
   branch `main` bloqueada, governança ausente em `feat/`, isenção em `chore/`.

**Critérios de aceite:**
- [ ] AC1, AC2, AC3, AC4, AC5, AC6 da REQ
- [ ] `make build` exit 0 · `go test ./...` verde · testes Node e Python verdes
- [ ] Saída dos 3 runtimes comparada **manualmente e byte a byte** nos 5 cenários acima, com a
      evidência colada no parecer (o gate formal é o ML-2A)
- [ ] `check-ship-parity.sh` e `check-ship-force-parity.sh` continuam verdes (AC11)
- [ ] `grep` provando que nenhum caminho de `push` chama o adaptador de forge para **abrir** PR
      (a checagem de PR aberto do `--force-with-lease` é leitura, e é permitida)

---

## Wave 2 — Gate e o que o guard ensina

> Dependências: ML-1A completo.
> **MLs sequenciais**, não paralelos: ambos recompilam `bin/trackfw` e rodam `make quality` — alvo de
> build e índice do git compartilhados. ML-2B começa só após a auditoria do ML-2A.

### ML-2A — Gate de paridade + falsificação nas duas direções
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-1A

**Arquivos afetados:**
- `scripts/check-push-parity.sh` (novo — espelhe a estrutura de `scripts/check-ship-parity.sh`)
- `scripts/check-cli-parity.sh` (adicionar `push` à enumeração de comandos, linha ~23)
- `scripts/check-gates-falsify.sh` (2 cenários novos, ao final; atualizar o total no echo final)
- `Makefile` (registrar `check-push-parity.sh` no alvo `parity`, junto aos demais)
- `docs/cli-parity.md` (linha na tabela de comandos + seção `## trackfw push` com anotação
  `<!-- trackfw-contract: gate=scripts/check-push-parity.sh -->`)
- **Proibido tocar:** qualquer arquivo de implementação do ML-1A.

**Ações:**
1. `check-push-parity.sh` com, no mínimo, os cenários: caminho feliz sem upstream; caminho feliz com
   upstream; `main` bloqueada; `feat/` sem roadmap (bloqueio de governança); `chore/` sem roadmap
   (isenção). Comparação byte-a-byte entre Go/Node/Python, com **guard de vacuidade** em cada
   cenário (fixture que não dispara nada é indistinguível de produto que não detecta nada).
2. Cenários de falsificação, **duas direções**:
   - direção A — remover o gate de governança do `push` num clone isolado ⇒ o gate deve falhar;
   - direção B — fazer o `push` abrir PR (ou commitar) num clone isolado ⇒ o gate deve falhar.
   Cada um com braço de baseline + braço de detecção, no padrão dos cenários 159/160.
3. `docs/cli-parity.md`: descrever a sequência de passos do `push`, a tabela de flags e a **fronteira
   explícita com `ship` e `commit`** (quem faz o quê).

**Critérios de aceite:**
- [ ] AC7, AC9, AC10 da REQ
- [ ] `bash scripts/check-push-parity.sh` verde · `check-cli-parity.sh` verde
- [ ] Os 2 cenários novos de falsificação com baseline verde e detecção vermelha, evidência colada
- [ ] `bash scripts/check-parity-contract-coverage.sh` exit 0
- [ ] `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` exit 0

### ML-2B — REASON do guard cita `trackfw push`
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-2A

**Arquivos afetados (os 5 que duplicam a string, em sincronia obrigatória):**
- `scripts/trackfw-git-branch-guard.sh:512`
- `internal/validator/validator_git_branch_guard_reference.go:554`
- `internal/generators/scaffold.go:1742`
- `npm/src/generators/hooks.js:1001`
- `pypi/trackfw/generators/init_gen.py:1634`
- **Proibido tocar:** `scripts/check-gates-falsify.sh` (a sincronia destes 5 já é cobrada pelos gates
  de hooks; não acrescente cenário lá neste ML) e qualquer arquivo do ML-1A/ML-2A.

**Ações:**
1. Alterar **apenas** a REASON do ramo `push` para apontar `trackfw push` como caminho primário,
   mantendo `trackfw release tag` onde já é citado. O ramo `commit` continua citando o comando dele.
2. Aplicar a **mesma string** nos 5 arquivos — divergência de um caractere quebra os gates.
3. Rodar os 4 gates que cobram a sincronia: `check-agent-hooks-parity.sh`,
   `check-harness-hooks-parity.sh`, `check-artifact-parity.sh`, `check-gates-falsify.sh`.
4. **Não** rodar `trackfw update` nem `update harness` no ambiente do usuário — a atualização do
   harness instalado é decisão dele.

**Critérios de aceite:**
- [ ] AC8 da REQ
- [ ] Os 5 arquivos com a string **byte-idêntica** (evidência: `grep -n` nos cinco, colado)
- [ ] Os 4 gates de hooks verdes
- [ ] `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` exit 0

---

## Wave 3 — Barreira

> Dependências: Wave 2 completa e auditada. **MLs paralelos** — escrevem documentos distintos e não
> alteram código.

### ML-3A — Revisão de qualidade
**Status:** ⬜ Pendente · **Agente:** `hefesto-tf` (`subagent_type: hefesto-tf`)
**Escreve:** `docs/qualidade/2026-08-22-revisao-do-comando-push.md`

Avaliar **duplicação real vs. reuso declarado**: o ADR manda reusar os helpers do `ship`; verificar
se foi isso que aconteceu nos 3 stacks ou se houve cópia. Apontar onde uma mudança futura no `ship`
deixaria de propagar para o `push`. **Veredito explícito.** Não altera código de produto.

### ML-3B — Revisão de segurança
**Status:** ⬜ Pendente · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-22-revisao-do-comando-push.md`

O `push` é caminho de escrita para o remoto. Avaliar: (a) se algum gate do `ship` foi perdido no
caminho; (b) se `--force-with-lease` continua exigindo PR aberto e não é alcançável por outra via;
(c) se a mudança da REASON do guard abre alguma leitura que ensine um caminho não governado.
**Veredito explícito.** Não altera código de produto.

---

## Notas
- **Fora de escopo:** tudo listado na seção *Negative scope* da REQ.
- Commits, branch e PR são exclusivos do `trackfw_architect`.
