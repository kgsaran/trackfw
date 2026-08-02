---
status: Done
date: 2026-08-02
author: "Zeus"
adr: "docs/adr/ADR-2026-08-02-extracao-de-referencia-tolerante-a-markdown-e-saida-do-validate-via-i18n.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-02-backticks-em-campos-de-referencia-e-mensagem-de-sucesso-do-validate-no-python.md"
---

# REQ: Backticks em campos de referencia e mensagem de sucesso do validate no Python

> Date: 2026-08-02 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation

Dois defeitos de paridade reportados como achados colaterais no PR #103, verificados por
reprodução em 2026-08-02.

**A — backticks tornam a referência invisível.** `extractRefPath` remove aspas antes de testar o
sufixo `.md`, mas **não** remove backticks. A linha

```
ADR: `docs/adr/ADR-2026-07-26-....md` (P1–P4; esta REQ é ...)
```

produz um token que não termina em `.md` → **nenhuma referência é encontrada, em silêncio**.
13 REQs do repositório usam essa forma; **3** não têm o frontmatter `adr:` populado e portanto
ficam inalcançáveis por qualquer regra que dependa do extrator, inclusive a
`adr_accepted_when_req_done` do PR #103.

Verificado que a causa é **única**: `adr: ""` no frontmatter **não** causa early-return — reduz a
string vazia, falha o teste de `.md` e o laço continua corretamente.

**B — Python ignora a própria chave de i18n.** Os 3 CLIs têm `validate.ok` =
`"✓ No violations found."` em `i18n/locales/en-US.json`. Go e Node usam.
`pypi/trackfw/commands/validate.py:104` imprime `"✓ Governance OK"` **hardcoded**.

## Acceptance Criteria

- [x] **AC1** — `extractRefPath` / equivalentes removem **backtick** além de aspas simples e
      duplas, nos 3 CLIs. Cada CLI **mantém o próprio mecanismo** — só o backtick é acrescentado
      ao conjunto.
- [x] **AC2** — **Reachability provada:** as 3 REQs hoje invisíveis
      (`roadmap-move-sincroniza-o-status-do-artefato`,
      `integridade-das-referencias-e-ciclo-de-vida-da-req`,
      `convergencia-dos-templates-de-artefato-do-cli-python`) passam a ter o ADR resolvido pelo
      extrator. Provado por teste, não por inspeção visual.
- [x] **AC3** — **Reachability discriminante:** fixture com ADR entre backticks e status
      `Proposed` **viola** `adr_accepted_when_req_done`. Antes da correção, a mesma fixture
      **não** violaria. É o que prova que a mudança tem efeito — observar `validate` verde não
      prova nada.
- [x] **AC4** — `validate` **verde neste repositório** nos 3 CLIs: as 3 REQs saem da
      invisibilidade mas apontam para ADR `Accepted`, logo reachability aumenta e violações não.
- [x] **AC5** — **Divergência medida, não presumida:** os 3 CLIs produzem saída **idêntica** para
      uma tabela compartilhada de entradas, incluindo: valor entre backticks, entre aspas duplas,
      entre aspas simples, sem delimitador, com prosa após o caminho, e com **delimitador não
      pareado** (`"x.md'`). Se algum divergir, **reportar** — não escolher sozinho.
- [x] **AC6** — Python passa a usar a chave `validate.ok` do próprio i18n; o literal
      `"✓ Governance OK"` é removido. Os 3 CLIs imprimem `✓ No violations found.`
- [x] **AC7** — `scripts/check-validate-parity.sh` e `scripts/check-artifact-parity.sh` passam.
- [x] **AC8** — Cenário de falsificação permanente em `check-gates-falsify.sh` com **fixture
      contendo backtick**: revertendo a remoção do backtick, a violação **desaparece**. Sem fixture
      com backtick o cenário seria vacuoso — lição registrada em
      `vault/notes/deteccao-de-status-de-adr-divergencias-entre-clis-2026-08-01.md`.
- [x] **AC9** — `make build`, `make test`, `make lint`, `make parity` e `make quality` verdes.

## Negative Scope (fora do escopo — NÃO fazer)

- **Não unificar os mecanismos de strip** dos 3 CLIs. Go usa `strings.Trim` (conjunto), Node usa
  regex de uma ocorrência por ponta, Python só remove **par casado**. Unificar mudaria o
  comportamento em delimitador não pareado sem caso real que o exija. A divergência é **medida**
  pelo AC5, não corrigida.
- **Não alterar o early-return de valor vazio** em `extractRefPath` — está correto.
- **Não tolerar outras decorações markdown** (`*`, `_`, colchetes de link). Só backtick, que é a
  forma efetivamente usada nos 13 casos reais.
- **Não corrigir o frontmatter das 3 REQs** para contornar — a correção é no mecanismo.
- **Não alinhar Go e Node ao `"✓ Governance OK"`** — o Python é o desviante.
- Não alterar outras mensagens de saída nem outras chaves de i18n.
- **Não mexer no comando `status`** — a divergência dele é o ponto 1 da fila, com REQ própria.
- Não alterar o status de nenhum ADR ou REQ do repositório.
- Não mexer em `pypi/build/lib/`; não adicionar dependência.

## Notas de implementação

| | Extrator | Mecanismo de strip |
|---|---|---|
| Go | `internal/validator/validator.go:1513-1533` | `strings.Trim(fields[0], "\"'")` (linha ~1526) |
| Node | `npm/src/validator/index.js:855-868` | `replace(/^["']\|["']$/g, '')` (linha ~863) |
| Python | `pypi/trackfw/validator.py:227-246` | `normalize_yaml_flat_value` (linha ~319) |

Mensagem: `pypi/trackfw/commands/validate.py:104`; chave `validate.ok` em
`pypi/trackfw/i18n/locales/en-US.json:101`.

Os 3 CLIs têm arquivos disjuntos → os MLs podem rodar **em paralelo**. Mas `make parity` só fecha
com os três prontos — a paridade é barreira de wave posterior, não critério dos MLs individuais.

`check-gates-falsify.sh` leva mais de 2 min; rodar em background.

## Linked ADR

ADR: docs/adr/ADR-2026-08-02-extracao-de-referencia-tolerante-a-markdown-e-saida-do-validate-via-i18n.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: docs/roadmaps/done/ROADMAP-2026-08-02-backticks-em-campos-de-referencia-e-mensagem-de-sucesso-do-validate-no-python.md
