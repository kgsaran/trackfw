---
name: github-fecha-keyword-por-balde-de-zona
description: Medido 2026-09-24 por 7 PRs sonda — GitHub IGNORA palavra-chave de fechamento em zona de CODIGO (cerca, span, indentado 4 espacos) e HONRA em zona nao-codigo (blockquote, celula de tabela, aspas retas)
metadata:
  type: project
---

`closingIssuesReferences` medido com PR rascunho, base `main`, keyword inglesa **exclusivamente** na zona:

| zona | resultado | fecha? |
|---|---|---|
| prosa (controle) | `[426]` | sim |
| cerca ``` | `[]` | **não** |
| code span | `[]` | **não** |
| indentado 4 espaços | `[]` | **não** |
| blockquote `>` | `[430]` | sim |
| célula de tabela | `[430]` | sim |
| aspas retas `"…"` | `[430]` | sim |

Sondas: issues #426/#430, PRs #427–#429 e #431–#434 (todos fechados, branches `probe/*` deletadas).
Registro completo em `docs/seguranca/2026-09-25-discriminante-do-gate-de-palavra-chave.md` §10.

**Why:** eu havia escrito no ML-0B que o gate de `check-pr-closing-keyword.sh` tinha um falso positivo
("a zona de código apaga a isenção inglesa" — forma 8). Era **erro meu**: como o GitHub não fecha dentro de
cerca, acusar está certo. E a premissa oposta ("o GitHub fecha nas cinco zonas") derrubou 2 das 5 linhas da
minha própria tabela de custo de desenho.

**How to apply:** ao desenhar mascaramento de zona neste gate, o balde decide a direção — zona de **código**
sai dos **dois** matchers (PT e EN); zona **não-código** sai **só** do matcher PT. Tratar as seis juntas erra
metade. 🔴 **Nunca extrapolar o balde por analogia** (comentário HTML, `<pre>`, aspas curvas, `~~~`, cerca com
linguagem seguem **não medidos**) — foi exatamente a analogia que produziu o erro refutado aqui.
Ver [[feedback_execute_all_named_vectors_before_verdict]] e
[[feedback_medir_decode_e_encode_separadamente]]: o braço de **controle** é o que torna um `[]` interpretável;
sem ele, resultado nulo é indistinguível de instrumento quebrado.
