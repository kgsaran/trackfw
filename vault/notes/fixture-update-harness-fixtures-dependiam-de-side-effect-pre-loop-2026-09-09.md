---
name: fixture-update-harness-fixtures-dependiam-de-side-effect-pre-loop
description: Fixtures dos cenários 68/69 do falsify dependiam do side-effect pre-loop do update harness para criar .trackfw/scripts/; ao remover o side-effect (ML-1A), 3 fixtures falharam
metadata:
  type: project
---

# Fixtures de falsify que dependiam do side-effect pre-loop do `update harness`

**Cenário:** ML-1A removeu o bloco pre-loop de `UpdateHarness`/`run`/`_run` que escrevia
`trackfw-git-branch-guard.sh` e `trackfw-credential-guard.sh` incondicionalmente antes do loop de
alvos. Os scripts passaram a ser escritos SOMENTE quando os alvos `git-branch-guard-script` /
`credential-guard-script` são explicitamente incluídos.

**Por que:** Os cenários 68 e 69 do `check-gates-falsify.sh` tinham chamadas do tipo:
```bash
update harness --targets claude-git-branch-guard,codex-git-branch-guard --install-missing
```
…e depois tentavam usar (ou tamper) os scripts em `.trackfw/scripts/`. Antes do ML-1A, o side-effect
escrevia os scripts mesmo com targets filtrados; após o ML-1A, os scripts só existem se listados.

**Sintoma:** `printf '# tampered\n' >> "$SCRIPT_PATH"` falhava com
`No such file or directory` porque o DIRETÓRIO `.trackfw/scripts/` não havia sido criado. Bash
com `set -e` encerrava o chunk; `falsify_fail_point` nunca era chamado; 0 FAIL mas vários GUARDA
labels ausentes em cascata.

**Fixtures afetadas (3):**
1. `s68-dup-home-gbg`: `update harness --targets claude-git-branch-guard,codex-git-branch-guard` → faltava `git-branch-guard-script`
2. `s68-dup-home-cg`: `update harness --targets claude-credential-guard,codex-credential-guard` → faltava `credential-guard-script`
3. `s69`: `update harness --targets kiro-git-branch-guard,kiro-credential-guard` → faltavam ambos

**Correção:** adicionar os alvos faltantes às chamadas de `update harness` dentro das fixtures.

**Regra geral:** quando uma fixture chama `update harness --targets X` e depois usa arquivos que
SÃO escritos por target Y (não X), essa dependência estava implícita no side-effect. Após qualquer
refactoring que torna targets explícitos, as fixtures devem declarar todos os targets que consomem.

**Why:** Fixtures de falsify testam invariantes de produção; se a dependência de um side-effect não
está declarada nos `--targets` da chamada de setup, o teste depende de comportamento que não pode
ser garantido por contrato.

**How to apply:** ao remover side-effects de `update harness` (qualquer runtime), varrer
`check-gates-falsify.sh` por chamadas de `update harness --targets` e verificar se os targets
declarados cobrem todos os arquivos que o cenário usa. Buscar por: `$SCRIPT_PATH`, `$T??_GBG_SCRIPT`,
`$T??_DUP_SCRIPT`, `>> "$T??_`...
