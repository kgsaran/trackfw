---
title: "Scenario 50 (credential-guard-mode-downgrade) non-vacuity fica obsoleta pelo M4 (rules: ancorado no HEAD)"
date: 2026-08-12
author: Apolo (apolo-tf)
tags: [credential-guard, validate, falsify, baseline]
---

# Contexto

`ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-credential-guard`, ML-1A. ADR:
`docs/adr/ADR-2026-08-12-severidade-das-regras-de-credential-guard-resolvida-pela-mais-estrita-entre-head-e-disco.md`
(mecanismo M4).

## O que muda de comportamento

Antes deste ML, a severidade das 3 regras de credential-guard
(`credential_guard_hook_resolvable`, `credential_guard_script_integrity`,
`credential_guard_mode_downgrade`) era lida **só do disco** (`trackfw.yaml` no CWD), igual a
qualquer outra regra. Um `rules: <nome>: off` **não commitado** desligava a regra imediatamente.

Depois do M4: para essas 3 regras, `ruleSeverity()` (Go)/`ruleSeverity()` (Node)/`_rule_severity()`
(Python) resolve pela **mais estrita entre HEAD e disco**. Um `rules: <nome>: off` **não commitado**
**não tem mais efeito nenhum** — é exatamente o canal que o ADR fecha.

## Por que `scripts/check-gates-falsify.sh` quebra

`make quality` roda `scripts/check-gates-falsify.sh`, cujo **Scenario 50** (linhas ~4262-4296,
`falsify/credential-guard-mode-downgrade/*`) tem um braço de **não-vacuidade**
(`falsify/credential-guard-mode-downgrade/non-vacuity`) que:

1. Prepara uma fixture com `HEAD` commitando `mode: block` e disco com `mode: warn` (downgrade não
   commitado) — dispara a regra normalmente.
2. Escreve, **só em disco, sem commit**, `rules:\n  credential_guard_mode_downgrade: off\n` na
   MESMA fixture.
3. Afirma via `assert_would_now_fail` que, com a regra "desligada", o braço de detecção
   **deixaria de disparar** — prova de que a asserção de detecção realmente depende da regra estar
   ativa (não é vácua).

Isso é **exatamente o comportamento pré-ADR**. Com M4 em produção, o passo 2 não tem mais efeito —
HEAD não tem `rules:` nenhuma, então a severidade do HEAD resolve para o default (`error`, o mais
estrito), e a comparação direcional escolhe `error` mesmo com `off` em disco. O braço de detecção
**continua disparando** mesmo com a "regra desligada" (porque ela não está mais desligada — essa é
a correção). `assert_would_now_fail` falha porque a saída "ainda passaria" quando deveria ter
mudado.

## O que fazer (ML-2A, Ártemis)

O roadmap já separa isso explicitamente: **Wave 2 — ML-2A** reescreve
`scripts/check-gates-falsify.sh` para o cenário decisivo do M4 — a edição combinada
(`mode: warn` + `rules: credential_guard_mode_downgrade: off`, **ambos não commitados**) deve
continuar sendo reportada. O Scenario 50 antigo (linhas ~4262-4296) precisa ser **substituído**, não
só remendado:

- A prova de não-vacuidade original (rules: off em disco desliga a regra) **não é mais verdadeira**
  e não deveria ser — ela testava o bug que este ML corrige.
- A nova prova de não-vacuidade precisa demonstrar o oposto: que a detecção **não pode** ser
  desligada por uma edição não commitada (braço "ataque combinado, ainda dispara") E que o
  desligamento **commitado** continua funcionando (braço "legítimo, commitado, silencia" — ver ADR
  §Decision point 5 e os testes em `internal/validator/validator_credential_guard_integrity_test.go`
  para o padrão exato: HEAD precisa commitar tanto `mode: block` quanto `rules: ...: warning|off`
  juntos para a severidade ficar ancorada, e ainda assim o disco precisa divergir do `mode` do HEAD
  para a mensagem sequer ser gerada — ver os testes `*_commitado` e `*_nao_commitado_ainda_dispara`).
- Ler `vault/notes/armadilhas-ao-escrever-cenario-em-check-gates-falsify-2026-08-12.md` antes de
  escrever o novo cenário (already referenced pelo roadmap).

## Evidência

`make quality` (2026-08-12, branch
`fix/ancorar-rules-no-head-para-as-regras-de-credential-guard`): todos os alvos
(`test`, `test-node`, `test-python`, `lint`, e todos os scripts de `parity` exceto
`check-gates-falsify.sh`) passam. `check-gates-falsify.sh` falha **só** no Scenario 50
non-vacuity, com a mensagem:

```
FAIL [falsify/credential-guard-mode-downgrade/non-vacuity]: com a regra desligada (rules: ...: off),
o braço de detecção AINDA passaria (saiu 1 e contém 'current file does not resolve to block') — a
asserção de detecção não depende desta regra estar ativa
```

Essa falha é **esperada e correta** — é o próprio comportamento novo se manifestando no cenário
antigo. Não é uma regressão do ML-1A; é o motivo de existir o ML-2A.
