---
title: `if:` de step sem função de status carrega um `success()` implícito — e pula o passo por um motivo que não está escrito
tags: [ci, github-actions, gotcha, medicao]
date: 2026-08-30
related: [[ambiente-do-dev-e-mais-rico-que-o-do-ci-2026-08-29]]
---

## Sintoma

Job de medição em Windows com três passos de suíte, cada um com a mesma condição de entrada:

```yaml
- name: Go — suíte completa
  if: steps.precondition.outcome == 'success'
- name: Node — suíte completa
  if: steps.precondition.outcome == 'success'
- name: Python — suíte completa
  if: steps.precondition.outcome == 'success'
```

Resultado no runner:

```
12. Go — suíte completa      → failure
13. Node — suíte completa    → skipped
14. Python — suíte completa  → skipped
```

A precondição **passou**. Os três passos tinham a mesma condição. Mesmo assim, dois foram pulados —
e o motivo **não aparece em lugar nenhum** da condição escrita.

## Causa Raiz

O GitHub Actions **prepende `success()` implicitamente** a qualquer `if:` que não contenha uma função
de status (`success()`, `failure()`, `always()`, `cancelled()`). Então

```yaml
if: steps.precondition.outcome == 'success'
```

é avaliado como

```yaml
if: success() && steps.precondition.outcome == 'success'
```

Quando o passo do Go falhou, o `success()` implícito do job virou falso e derrubou os dois seguintes
— apesar de a condição escrita continuar verdadeira.

## Por que dói mais num instrumento de medição

Num pipeline comum, parar na primeira falha é o comportamento desejado. **Num job cujo propósito é
medir**, é defeito: cada execução mede só até o primeiro tropeço, e como o job foi construído para
falhar (linha de base de defeitos conhecidos), os runtimes seguintes **nunca** seriam medidos.

Medimos um de três e o relatório parecia completo.

## Correção

```yaml
if: always() && steps.precondition.outcome == 'success'
```

O `always()` neutraliza o `success()` implícito; a condição real de entrada continua sendo a
precondição — **não** virou `always()` cego, que rodaria as suítes mesmo com a precondição
reprovada e produziria resultado não confiável.

## Como detectar

Nenhum validador pega: `yaml.safe_load` valida sintaxe, `actionlint` valida schema, e a expressão é
válida nos dois. O sintoma é **passo pulado com condição verdadeira** — se você vir `skipped` num
passo cuja condição você sabe que vale, procure o `success()` implícito antes de qualquer outra
hipótese.
