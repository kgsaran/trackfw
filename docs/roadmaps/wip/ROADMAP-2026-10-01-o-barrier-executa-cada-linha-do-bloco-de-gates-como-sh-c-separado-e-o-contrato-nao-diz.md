---
status: wip
date: 2026-10-01
req: "docs/req/REQ-2026-10-01-o-barrier-executa-cada-linha-do-bloco-de-gates-como-sh-c-separado-e-o-contrato-nao-diz.md"
squad: "hades-tf"
---

# Roadmap: o barrier executa cada linha do bloco de gates como sh -c separado e o contrato não diz a consequência

> Created: 2026-10-01 | Status: wip

## Context
REQ: docs/req/REQ-2026-10-01-o-barrier-executa-cada-linha-do-bloco-de-gates-como-sh-c-separado-e-o-contrato-nao-diz.md
Issue: #491 · ADR: `ADR-2026-09-01` (gate é contrato POSIX shell) · Notas de vault:
`parsegates-per-line-isolation-fuse-same-line-2026-09-30.md`, `barrier-gate-auto-referencial-vira-fork-bomb-2026-09-30.md`

Varredura (2026-10-01): nenhuma issue ou REQ aberta com este mecanismo. **Premissa do #491
corrigida** (comentário no issue): a execução por linha **está** na regra 5 do `cli-parity.md`. O que
falta é a consequência, o aviso na superfície de autoria e a distinção entre "bloco malformado" e
"gate reprovou".

## Acceptance Criteria
- [ ] **AC1** — decisão (a) documentar · (b) detectar fragmento · (c) bloco como script, com medição do acervo
- [ ] **AC2** — regra 5 escreve a consequência, com exemplo
- [ ] **AC3** — template e assets de autoria avisam no ponto em que o gate é escrito
- [ ] **AC4** — comportamento do `barrier` conforme o AC1
- [ ] **AC5** — 🔴 todo bloco do acervo que passa hoje continua passando
- [ ] **AC6** — `make quality` e CI verdes

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependências: nenhuma. 🔴 **BLOQUEIA toda implementação.**

### ML-0A — medir o acervo e decidir entre (a), (b) e (c)
**Owner:** `hades-tf`
**Status:** ⬜ Pendente
**Arquivos de leitura:** `internal/roadmapdoc/roadmapdoc.go` (`ParseGates`) · `internal/commands/barrier.go`
(`runGateCommand`, `evalGateCommands`) · `docs/cli-parity.md` (regra 5, ~:2619, e § *Wave gates are a
portable POSIX-shell contract*) · ADR-2026-09-01 · o template de Wave 0 em `internal/generators/` ·
as duas notas de vault acima
**Entregável:** `docs/seguranca/2026-10-01-wave0-gate-por-linha.md`
**Método:** medir. 🔴 **Não escrever implementação.**

**Perguntas:**
1. **O acervo.** Extraia **todos** os blocos de gate do acervo (`docs/roadmaps/**`, as fixtures
   em `scripts/testdata/` e `internal/roadmapdoc/testdata/`) com a mesma regra do `ParseGates`. Quantos
   blocos: (i) têm alguma linha que **não é comando completo** sozinha (aspas desbalanceadas, `$(`/`(`/`{`
   sem fechar, `\` no fim, heredoc)? (ii) **dependem** de estado de uma linha anterior (`export`, `cd`,
   atribuição usada depois)? (iii) dependem de **todas** as linhas rodarem mesmo quando uma no meio
   falha? É isso que separa (c) de (b).
2. **Candidatos.** Para (a), (b) e (c), o que cada um quebra e o que deixa passar, com o contra-braço
   medido. Em (c), diga como fica a evidência por comando (`<cmd>: exit N`), que o JSON do barrier e o
   corpus do contrato (`scripts/check-roadmap-barrier-contract.sh`) já fixam, e o que acontece sem
   `set -e`: um script cujo **último** comando passa mascara uma falha anterior. Em (b), como detectar
   "comando incompleto" **sem** reimplementar um parser de shell (por exemplo `sh -n` por linha), qual o
   custo e quais os falsos positivos.
3. **Superfície de autoria.** Enumere todo lugar em que um autor aprende a escrever um gate: o template
   gerado, os assets de agente e skill gerados, o `cli-parity.md` e o que mais existir.
4. **Ameaça.** Alguma das opções abre um caminho novo para conteúdo do roadmap chegar ao `sh`
   (diferente de hoje)? Exemplo: em (c), o bloco inteiro passa a ser **um** argumento. Isso muda a
   relação com o trust check?

**Ataque as minhas premissas:**
- Afirmo que nenhum gate do acervo depende de estado entre linhas, porque isso nunca funcionou.
  **Meça**: pode haver gate que "funciona por acaso" (a linha 2 só lê arquivo, a variável não importa).
- Afirmo que (b) é o menor corte que protege o autor. Se (c) for mais simples e não quebrar o acervo
  nem a evidência, **diga**.

**Critérios de aceite:**
- [ ] Contagens (i), (ii) e (iii) sobre o acervo, com o comando usado
- [ ] (a), (b) e (c), cada um com o contra-braço medido e o veredito
- [ ] Superfície de autoria enumerada com `arquivo:linha`
- [ ] Recomendação, com o resíduo declarado
- [ ] Nenhuma linha de implementação

**Gates da wave:**

> 🔴 Cada linha deste bloco é um comando separado: essa é a semântica que este roadmap documenta.

```bash
test -f docs/seguranca/2026-10-01-wave0-gate-por-linha.md || { echo 'GATE FALHOU: parecer da Wave 0 ausente' >&2; exit 1; }
grep -qi 'recomenda' docs/seguranca/2026-10-01-wave0-gate-por-linha.md || { echo 'GATE FALHOU: parecer sem recomendacao' >&2; exit 1; }
```

## Wave 1 — Implementação
> Dependências: Wave 0 auditada e, se o parecer recomendar mudar a semântica (c), **decisão do KG**.
> MLs escritos depois, a partir do parecer.
