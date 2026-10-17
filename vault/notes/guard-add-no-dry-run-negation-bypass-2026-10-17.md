---
name: guard-add-no-dry-run-negation-bypass-2026-10-17
description: git add -An --no-dry-run passa o guard mas estageia whole-tree; git usa last-flag-wins, --no-dry-run cancela -n; guard não modela negação
metadata:
  type: project
---

# Guard de staging: --no-dry-run / --no-dry cancela a detecção de dry-run

**Encontrado em:** Rodada 2 do red-team REQ-2026-09-05, 2026-10-17.
**Status:** NÃO CORRIGIDO — B-3 bloqueante no parecer.

## Mecanismo

O B-1 fix (commit `ccccedc8`) adicionou decomposição de combined flags: o char `n` em `-An`
seta `dryRun=true`, e o guard passa o comando. Correto para `-An` genuíno.

Mas git usa **last-flag-wins** via parse-options. `--no-dry-run` e `--no-dry` (abreviação) são
flags válidas que cancelam `-n`/`--dry-run`. O guard não reconhece esses negativos:

```
git add -An --no-dry-run   → guard vê dryRun=true → PASSA; git estageia tudo (--no-dry-run cancela -n)
git add -An --no-dry       → idem (abreviação aceita)
```

**Medido em /tmp/tgit:**
- `git add -An`: dry-run, nada staged (correto)
- `git add -An --no-dry-run`: `other.txt`, `test.txt` staged (whole-tree)

## Escopo do bypass

- Bypass de staging whole-tree com combined flag: `git add -An --no-dry-run` (B-1 fix introduziu)
- Bypass pré-existente (era miss na round 1 também): `git add -n --no-dry-run .`, `git add -p --no-patch .`, `git add --dry-run --no-dry-run .`

## Fix

Tratar `--no-dry-run`, `--no-dry`, `--no-patch`, `--no-interactive` como cancelamento dos
booleans correspondentes — last-wins, não OR acumulativo. Implementar como sobrescrita
sequencial em vez de `dryRun = dryRun || condition`.

Atenção: `--no-dry-run -n .` deve bloquear (last-wins → dry-run ativo); implementar com
last-wins correto.

**Why:** [[feedback_execute_all_named_vectors_before_verdict]]
