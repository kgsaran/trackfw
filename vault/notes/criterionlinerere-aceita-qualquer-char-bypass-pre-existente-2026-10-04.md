---
title: CriterionLineRe aceita qualquer char em [.] — bypass pré-existente mais barato que Caducou:
date: 2026-10-04
tags: [barrier, acceptance_evidence, bypass, security]
---

# `CriterionLineRe = ^- \[.\]` aceita qualquer caractere único — bypass sem justificativa

## Medição (binário main, 2026-10-04)

```bash
[tilde-bracket-[~]]: result: passed    # [~] → met=1, passed
[dash-bracket-[-]]:  result: passed    # [-] → met=1, passed
[question-mark-[?]]: result: passed    # [?] → met=1, passed
```

Qualquer critério `- [~] texto` conta como atendido, sem exigir justificativa nem `Caducou:`.
Isso é mais barato que `Caducou:` (que exige texto não-vazio) e não deixa rastro visível.

## Por que importa para a REQ-2026-10-04

A ADR menciona `[~]` como token rejeitado porque "os parsers que não o aprendessem contariam
como neutro". A medição mostra que **já conta como met**, na direção errada.

Se o objetivo de segurança é impedir bypass silencioso, `CriterionLineRe` precisa ser
restrito a `^- \[[ xX]\]`. Sem isso, a D3 não melhora a postura: `[~]` continuará passando
sem texto algum.

## Ação recomendada

Abrir issue separada. O ajuste é de uma linha em `roadmapdoc.go:46` mas tem surface de
regressão alta (qualquer teste que usa `- [~]` ou similar).

## Referência

REQ-2026-10-04, ML-0A, achado T3 (`docs/seguranca/2026-10-04-wave0-criterio-caducado-e-cortes.md`).
