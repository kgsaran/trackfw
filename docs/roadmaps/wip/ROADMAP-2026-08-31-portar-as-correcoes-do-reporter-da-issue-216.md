---
status: wip
date: 2026-08-31
req: "docs/req/REQ-2026-08-31-portar-as-correcoes-dos-prs-222-225-do-reporter-da-issue-216-defeitos-1-2-3-5-e-6-de-windows.md"
squad: "hades-tf, apolo-tf, hefesto-tf"
---

# Roadmap: Portar as correções do reporter da issue #216

> Created: 2026-08-31 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-31-portar-as-correcoes-dos-prs-222-225-do-reporter-da-issue-216-defeitos-1-2-3-5-e-6-de-windows.md`

`lourivalgarciajunior` reportou o issue #216 e enviou quatro PRs com as correções. Fechados para não
colidir com nosso ciclo; **a análise concluiu que os quatro valem inteiros**
(`docs/analises/2026-08-31-aproveitamento-dos-prs-222-225.md`).

**Duas regras valem para todos os MLs:**

1. 🔴 **Atribuição:** todo commit carrega `Co-Authored-By: lourivalgarciajunior <lourival.garcia@gmail.com>`.
2. 🔴 **Porte fiel.** Estes diffs foram revisados **como estão escritos**. "Melhorar" em trânsito
   destrói a revisão que justificou aceitá-los. Se achar que algo deveria mudar, **pare e avise**.

Acesso ao conteúdo: os PRs estão `CLOSED` mas `gh pr diff 222|223|224|225` continua funcionando.
**Não faça `gh pr checkout`** — worktree compartilhado.

## Acceptance Criteria

- [ ] Defeitos 1, 2, 3, 5 e 6 corrigidos, portados fielmente, com os testes
- [ ] Atribuição em todo commit
- [ ] Contagem cai para **3 `REPRODUCED`** (itens 4, 7, 10) em runner Windows real
- [ ] Cada item que sai de `REPRODUCED` explicado no roadmap, citando o run
- [ ] `docs/cli-parity.md` ganha os dois contratos (LF na escrita, UTF-8 na saída)
- [ ] `make quality` e **CI** verdes

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model (escopo estreito, de propósito)
> Dependências: nenhuma. Bloqueia **apenas** a Wave 2.

### ML-0A — Mudança de âncora de confiança do #222 Grupo A
**Status:** ⬜ Pendente
**Agente:** `hades-tf`
**Files affected:** nenhum (documento em `docs/seguranca/`)
**Escopo estreito de propósito:** **não** remodelar os outros três PRs. `hefesto-tf` os avaliou e o
arquiteto auditou essa avaliação. A única pergunta de confiança genuinamente nova é esta.
**Actions:**
1. O #222 Grupo A muda a âncora de `validateGuardGlobalHookResolvable` de **API nativa do SO** para
   **`$HOME` env-var-first**. Env var é controlável pelo processo pai; API nativa não é. Avalie: quem
   controla `$HOME` no momento da validação, o que ganha, e se isso degrada a garantia do guard.
2. Falsificação nas duas direções: o que quebra se a âncora regride, e o que quebra se a validação
   ficar **estrita demais** e recusar ambiente legítimo (Windows sem `$HOME`, CI, contêiner).
3. Residual declarado.
**Critérios de aceite:**
- [ ] Veredito explícito sobre a troca de âncora, com vetor concreto se houver
- [ ] Nenhuma linha de implementação escrita
- [ ] Parecer em `docs/seguranca/2026-08-31-ancora-de-confianca-do-guard-global-home-first.md`

**Gates da wave:**
```bash
test -f docs/seguranca/2026-08-31-ancora-de-confianca-do-guard-global-home-first.md
! grep -qi "placeholder" docs/seguranca/2026-08-31-ancora-de-confianca-do-guard-global-home-first.md
grep -q "Residual" docs/seguranca/2026-08-31-ancora-de-confianca-do-guard-global-home-first.md
```

## Wave 1 — Bit de execução e CRLF (2 MLs em paralelo)
> Dependências: nenhuma. **Arquivos disjuntos entre os dois MLs** — verificado na análise.

### ML-1A — #222 Grupo B: bit de execução no validator (item 3)
**Status:** ⬜ Pendente
**Agente:** `apolo-tf`
**Fonte:** `gh pr diff 222`, **só o grupo do bit de execução** (não o grupo `$HOME`, que é a Wave 2).
**Atenção:** distinto da `REQ-2026-08-28`, que cobriu apenas `scaffold_doctor.go`. Este é o
**validator**. Confirme que não há sobreposição antes de editar.
**Critérios de aceite:**
- [ ] Port fiel do grupo do bit de execução, com os testes
- [ ] `Co-Authored-By: lourivalgarciajunior <lourival.garcia@gmail.com>` — o commit é meu, mas a
      autoria é dele; me devolva a mensagem sugerida
- [ ] Não toca os arquivos do Grupo A (`$HOME`)
- [ ] `make quality` verde

### ML-1B — #225: geradores Python escrevem LF, não CRLF (item 5)
**Status:** ⬜ Pendente
**Agente:** `apolo-tf`
**Fonte:** `gh pr diff 225`.
**Inclui** o gate `check-python-writes-lf.sh` que o PR introduz.
**Critérios de aceite:**
- [ ] Port fiel, com testes e com o gate
- [ ] O gate falsifica nas duas direções e tem guarda de vacuidade
- [ ] `make quality` verde

**Gates da wave:**
```bash
make quality
```

## Wave 2 — `$HOME` nos 3 runtimes (item 2)
> Dependências: **Wave 0 aprovada**. Não iniciar antes do veredito do `hades-tf`.

### ML-2A — #222 Grupo A: `$HOME` nos 3 runtimes
**Status:** ⬜ Pendente
**Agente:** `apolo-tf`
**Fonte:** `gh pr diff 222`, **só o grupo `$HOME`**.
**Critérios de aceite:**
- [ ] Port fiel, com testes, **incorporando o que a Wave 0 exigir**
- [ ] Paridade nos 3 runtimes — este defeito é dos três
- [ ] `make quality` verde

## Wave 3 — cp1252 e isatty (sequencial: diffs empilhados)
> Dependências: Wave 1 completa. **ML-3B depois de ML-3A** — o diff do #224 **contém** o do #223.

### ML-3A — #223: UTF-8 na saída do CLI Python (item 1)
**Status:** ⬜ Pendente
**Agente:** `apolo-tf`
**Fonte:** `gh pr diff 223`.
**Correção na origem:** `_force_utf8_output()` chamada no início de `main()`, reconfigurando
`sys.stdout`/`sys.stderr` **dentro do processo**, sem depender de env var.
🔴 **Isto NÃO corrige o item 4.** O item 4 é um `print()` cru em
`scripts/check-parity-contract-coverage.sh`, que nunca entra em `main()`. Não tente corrigi-lo aqui.
**Critérios de aceite:**
- [ ] Port fiel, incluindo os testes (`TestCliEmConsoleCp1252` reproduz console cp1252 em **qualquer
      SO** via `PYTHONIOENCODING=cp1252` — roda no CI Linux todo dia)
- [ ] Item 4 **não** tocado
- [ ] `make quality` verde

### ML-3B — #224: `isatty()` mente `True` para `NUL` (item 6)
**Status:** ⬜ Pendente
**Agente:** `apolo-tf`
**Fonte:** `gh pr diff 224`. **O diff dele inclui o do #223** — porte só o que for do item 6.
**Critérios de aceite:**
- [ ] Port fiel apenas da parte do item 6, sem duplicar o ML-3A
- [ ] `make quality` verde

## Wave 4 — Contratos em `docs/cli-parity.md`
> Dependências: Waves 1 e 3 completas.

### ML-4A — Escrever os dois contratos que as correções passam a impor
**Status:** ⬜ Pendente
**Agente:** `hefesto-tf`
**Files affected:** `docs/cli-parity.md`
**Diagnóstico:** o #225 introduz um gate que **impõe** um contrato que **não está escrito em lugar
nenhum**. Gate sem contrato escrito é exatamente o drift que aquele documento existe para impedir.
**Actions:**
1. *"Os 3 runtimes escrevem artefato em LF"* — imposto por `check-python-writes-lf.sh`.
2. *"Os 3 runtimes escrevem UTF-8 na saída, independente da codepage do console"* — cumprido pelo
   `_force_utf8_output()`.
**Critérios de aceite:**
- [ ] Os dois contratos escritos no formato das seções existentes, apontando o gate que os impõe
- [ ] `make quality` verde

## Verificação que só o CI fecha

A **contagem cair para 3 `REPRODUCED`** só se verifica em runner Windows real, no push. Verde local
não prova. Cada transição `REPRODUCED → ABSENT` é explicada aqui citando o run.

## Barreira final

`hefesto-tf` e `hades-tf` sobre o diff completo, auditoria do arquiteto, `barrier`. **CI verde** — e
aqui "verde" significa a camada 2 reportando **3**, com os cinco itens corrigidos explicados.
