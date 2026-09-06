# `credentialGuardScriptMarker`/`gitBranchGuardScriptMarker` são const compartilhada entre 3 consumidores — sabotar a const, não o call site, quebra cenários que miram só um deles

**Data:** 2026-09-06
**Onde:** `internal/validator/validator_credential_guard.go` (declaração das const),
`internal/validator/validator_git_branch_guard.go` (2 dos 3 consumidores),
`scripts/check-gates-falsify.sh` Cenário 80, `scripts/check-validate-parity.sh`
**Achado por:** apolo-tf, ML-1D (ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio)

## Sintoma

`make quality` reportava 1 FAIL isolado depois dos MLs 1A/1B/1C desta REQ:

```
FAIL [falsify/validate-parity/credential-guard-hook-resolvable-not-detected]:
  saiu com 1 mas falta diagnóstico
  'credential_guard_hook_resolvable parity (claude-absent/go): expected violation'
```

O Cenário 80 do `check-gates-falsify.sh` já reprovava (`exit 1`) — a falha era só de
diagnóstico ausente, não de exit code. Isso é o sintoma clássico de "o gate morreu antes
de chegar no assert que prova a coisa certa", não de vacuidade.

## Causa raiz medida (não inferida)

`credentialGuardScriptMarker` (`const … = "trackfw-credential-guard.sh"`) é usada em **3
call sites**, não 1:

1. `validateCredentialGuardHookResolvable` (escopo PROJETO, `validator_credential_guard.go:554`)
2. `validateGuardGlobalHookResolvable` (escopo GLOBAL, `validator_git_branch_guard.go:353`)
3. `validateGuardGlobalScriptIntegrity` (escopo GLOBAL, `validator_git_branch_guard.go:357`)

O Cenário 80 original sabotava a **declaração da const** (`sed 's/const
credentialGuardScriptMarker = "trackfw-credential-guard.sh"/…-DISABLED.sh"/'`), pensada
quando só o consumidor 1 existia no `check-validate-parity.sh`. O ML-1C desta REQ
acrescentou um bloco cross-CLI para `credential_guard_script_integrity` em escopo GLOBAL
(consumidor 3) **posicionado ANTES**, no arquivo, do bloco `credential_guard_hook_resolvable`
que o Cenário 80 mira (posicionado depois).

Com a const inteira sabotada, `validateGuardGlobalScriptIntegrity` passou a procurar
`$HOME/.trackfw/scripts/trackfw-credential-guard-DISABLED.sh` — que não existe no fixture
(que só cria o arquivo com o nome real) — e `os.IsNotExist` faz a função devolver `nil, nil`
(sem violação) SÓ para Go. O bloco do ML-1C então morre com:

```
validate parity (script_integrity unreadable, GLOBAL): …/siu-global-go.json produced ZERO
credential_guard_script_integrity warnings — fixture is vacuous, or this CLI regressed to
silencing on an unreadable GLOBAL script
```

`check-validate-parity.sh` roda sob `set -euo pipefail` e esse `raise SystemExit` acontece
num heredoc Python que o shell aborta na hora — o script morre ali, o bloco do Cenário 80
(`credential_guard_hook_resolvable`, mais adiante) nunca executa, e o `assert_fails_with`
do falsify vê `exit 1` (correto) mas com o diagnóstico ERRADO (o do ML-1C, não o do Cenário
80).

Reproduzido isoladamente com `GO_BIN` apontando para binário sabotado fora do gate completo
— confirma que é ordem, não fixture quebrado nem const errada.

## Decisão e por quê

Das 3 opções levantadas (fazer o gate reportar todos os blocos que falham / aceitar
qualquer diagnóstico do primeiro bloco que falhar / tornar a sabotagem cirúrgica), a
escolhida foi **tornar a sabotagem cirúrgica**: sabotar o **call site 1** (linha 554,
`validateGuardHookResolvable("credential_guard_hook_resolvable",
credentialGuardScriptMarker)`) trocando o segundo argumento por um literal
`"trackfw-credential-guard-DISABLED.sh"` NAQUELE call site apenas — deixando a const e os
outros 2 consumidores intactos.

- Preserva exatamente o que o Cenário 80 afirma provar: que uma regressão específica na
  detecção do escopo de PROJETO do `credential_guard_hook_resolvable` é pega pelo parity
  gate. Sabotar a const inteira nunca provou isso com precisão — provava um efeito colateral
  em cascata sobre 3 regras diferentes ao mesmo tempo, e dependia da ordem dos blocos no
  script para não ser mascarado por outro bloco morrendo primeiro.
- Não muda o comportamento de `check-validate-parity.sh` (rejeitada a opção de fazer o gate
  reportar todas as falhas em vez de morrer na primeira — mudança de escopo maior, e nenhum
  outro consumidor do script depende de comportamento "reporta tudo").
- Não afrouxa o assert do Cenário 80 para aceitar qualquer diagnóstico (rejeitada essa
  opção também) — continuaria pinado exatamente na mensagem
  `credential_guard_hook_resolvable parity (claude-absent/go): expected violation`.

## Duas direções verificadas

- Árvore sabotada (call site) → `GO_BIN` aponta pro binário com o call site corrompido →
  `check-validate-parity.sh` reproduz **exatamente** a mensagem
  `credential_guard_hook_resolvable parity (claude-absent/go): expected violation from rule
  'credential_guard_hook_resolvable', none reported (exit=0)`, e todos os blocos anteriores
  (incluindo os do ML-1C, GLOBAL script_integrity, FIFO) passam limpos.
- Árvore íntegra (`bin/trackfw` real) → `check-validate-parity.sh` passa `exit 0` de ponta a
  ponta, incluindo o bloco `credential_guard_hook_resolvable cross-CLI` com as 22 fixtures
  (`claude-absent` … `claude-utf16`).

## Outros cenários com a mesma fragilidade — verificado, nenhum outro

`gitBranchGuardScriptMarker` tem o MESMO padrão de 3 consumidores compartilhando 1 const
(`validator_credential_guard.go:21` declaração, usada em `validator_git_branch_guard.go:361`
e `:365`, e em `validator_credential_guard.go:560`). Buscado no `check-gates-falsify.sh` por
`const gitBranchGuardScriptMarker` / `gitBranchGuardScriptMarker = ` — nenhum cenário sabota
a declaração dessa const. Só o Cenário 80 tinha esse padrão, e só ele foi retargetado.

## Ver também

- [[falsify-cenario-pina-linha-de-fonte-por-sed-guard-de-plataforma-quebra-2026-08-31]] —
  mesma família de problema (retarget de âncora sed em `check-gates-falsify.sh`), causa
  diferente (guard de plataforma prefixado vs. const compartilhada entre consumidores)
- `docs/roadmaps/wip/ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio.md` — ML-1D
