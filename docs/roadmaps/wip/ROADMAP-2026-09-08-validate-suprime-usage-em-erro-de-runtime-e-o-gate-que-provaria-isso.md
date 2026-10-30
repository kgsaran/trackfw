---
status: wip
date: 2026-09-08
squad: "hades-tf, apolo-tf"
req: "docs/req/REQ-2026-09-07-validate-imprime-usage-em-erro-de-runtime-e-suja-o-stream-json-so-no-go-e-o-gate-que-provaria-isso-e-um-gap-declarado.md"
---

# Roadmap: `validate` suprime usage em erro de runtime, e o gate que provaria isso

> Criado em: 2026-09-08 | Revisado: 2026-10-30 | Status: wip

## Context

REQ: `docs/req/REQ-2026-09-07-validate-imprime-usage-em-erro-de-runtime-e-suja-o-stream-json-so-no-go-e-o-gate-que-provaria-isso-e-um-gap-declarado.md`
(seção "Revisão de escopo — 2026-10-30" é o escopo vigente) · Issue #290 (fechado; usage corrigido no #413).

Vivo: com `--json`, três comandos escrevem uma linha fora do JSON no stderr **depois** de escrever o
documento, porque devolvem `error` para sinalizar rc≠0 e o `root.go` imprime a mensagem:

| comando | sítio | stderr hoje |
|---|---|---|
| `validate --json` com violação | `internal/commands/validate.go:38` | `N violation(s) found` |
| `update --json` com alvo falho | `internal/commands/update.go:85` | `Error: trackfw update: N target(s) failed` |
| `update harness --json` com alvo falho | `internal/commands/update_harness.go:106` | `Error: trackfw update harness: N target(s) failed` |

O `barrier --json` (blocked) já sai rc=1 com stderr 0 B — usa `os.Exit(1)`. Correção: erro tipado
que carrega o código de saída, reconhecido pelo `root.go`, que sai sem imprimir (decisão na REQ).

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

---

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model do erro tipado de saída silenciosa (hades-tf)
**Status:** ✅ Concluído — auditado em 2026-10-30. 6 ameaças, 11 cenários medidos, 8 prescrições. Decisões do arquiteto: **D-1 = opção (a)**: o contrato garante que o `root.go` não escreve nada depois do documento; diagnósticos emitidos durante o cálculo (recusa do pathguard, que já está no campo `message`; aviso de `agent_models` do `update harness`) continuam no stderr. **P7 (teste que conta chamadas por grep no código-fonte) não entra**: é detector de mudança, e o red team da Wave 2 cobre o uso indevido. Achado colateral corrigido no ML-1B: `windows-probe.yml` compara `context --json` e `validate --strict`, que não existem; os dois saem rc=1 com stdout vazio e o `fc.exe` rotula BYTE_IDENTICAL (conferido no binário).
**Files affected:** `docs/seguranca/2026-10-30-wave0-json-stderr-saida-silenciosa.md`
**Actions:**
1. Fail-open: um comando que devolva o erro tipado numa falha real engole a mensagem. Prescreva como
   restringir o uso a "depois que o documento foi escrito" (nome, construtor, onde pode ser criado) e
   o teste que impede regressão.
2. Interação com `root.go`: ordem frente a `formatUnknownCommandError`, `cmd.SilenceErrors`,
   `cmd.SilenceUsage` e a regra D7 (comandos sob `guard` saem 2, ADR-2026-10-04). Erro tipado
   embrulhado (`%w`) deve ou não ser reconhecido?
3. Confirme por medição (binário compilado em fixture fora da árvore) a tabela da REQ, inclusive um
   cenário real de alvo falho em `update --json` e `update harness --json`. Use HOME descartável.
   Não use `git commit` em fixture (o guard bloqueia); `git symbolic-ref HEAD refs/heads/<b>` basta.
4. Consumidores: algum gate, teste, workflow ou doc depende das linhas no stderr ou do `Error:` do
   `update`? Feche com o comando, sem `| head`.
5. Alvos de falsificação nas duas direções para AC1, AC1-bis e AC7.
6. Prescrições numeradas para ML-1A e residual declarado.
**Acceptance criteria:**
- [x] As seis seções respondidas com evidência (comando e resultado)
- [x] Prescrições escritas em seção intitulada `Prescrições`
- [x] Nenhuma linha de implementação escrita

**Gates da wave:**
```bash
test -f docs/seguranca/2026-10-30-wave0-json-stderr-saida-silenciosa.md
grep -q 'Prescrições' docs/seguranca/2026-10-30-wave0-json-stderr-saida-silenciosa.md
```

---

## Wave 1 — Implementação (2 MLs em paralelo)
> Dependencies: Wave 0 auditada. ML-1A e ML-1B tocam arquivos disjuntos.

### ML-1A — Stream `--json` limpo depois do documento (apolo-tf) — AC1, AC1-bis, AC7, AC8
**Status:** ✅ Concluído — auditado em 2026-10-30. Medido pelo arquiteto no binário em fixture strict: `validate --json` rc=1, stdout 597 B, stderr 21 → 0 B; modo texto inalterado (`Error: 1 violation(s) found`, rc=1). Os 7 `TestJSONExit_*` PASS em `-v`; contra-braço do executor (sem o ramo do `root.go`) reprova os 3 testes de stream com 21/42/50 B. `ExitCodeError` reconhecido por type assertion direta, fora dos comandos de `guard`. Contrato em `docs/cli-parity.md`, seção `--json` stream contract (post-document). Um laço `until` órfão do executor, que esperava um marcador do `make`, foi encerrado pelo arquiteto.
**Files affected:** `internal/commands/root.go`, `internal/commands/validate.go`, `internal/commands/update.go`, `internal/commands/update_harness.go`, `internal/commands/json_exit_test.go`, `internal/commands/exit_code_error.go` (P1 da Wave 0; declarado no handoff e omitido aqui, corrigido na auditoria), `docs/cli-parity.md`
**Actions:**
1. Criar o erro tipado de saída silenciosa e reconhecê-lo no `root.go` (sair com o código, sem
   imprimir), conforme as Prescrições P1–P6 e P8 da Wave 0 (P7 não entra; D-1 = opção (a)).
2. Nos três sítios, no ramo `--json`, devolver o erro tipado em vez da mensagem. Modo texto inalterado.
3. Testes em `internal/commands/json_exit_test.go` cobrindo AC1, AC1-bis e AC7.
4. `docs/cli-parity.md`: regra de contrato do AC8, perto da seção do `validate --json`.
**Acceptance criteria:**
- [x] `validate --json` com violação: rc=1, stdout JSON válido, stderr 0 B (medido no binário em fixture)
- [x] `update --json` e `update harness --json` com alvo falho: rc≠0, stdout JSON, nenhuma linha do `root.go` no stderr (diagnósticos do cálculo podem existir, D-1)
- [x] modo texto e lenient inalterados; `guard` continua saindo 2
- [x] contra-braço: revertido o reconhecimento no `root.go`, os testes novos reprovam

**Gates da wave:**
```bash
python3 scripts/check-workflow-yaml.py
go build ./...
go vet ./internal/commands/
env PATH="$(dirname "$(command -v go)"):/usr/bin:/bin" go test ./internal/commands/ -count=1
```

### ML-1B — `windows-probe.yml` compara comandos que existem (ares-tf) — achado colateral da Wave 0
**Status:** ✅ Concluído — auditado em 2026-10-30. Lista com `version`, `validate --json`, `status`, `context --format=json` (todos rc=0 e stdout não vazio no binário); o passo reprova quando um lado sai rc≠0 com stdout vazio. YAML: `check-workflow-yaml` 10 passed. Não executado em Windows.
**Files affected:** `.github/workflows/windows-probe.yml`
**Actions:**
1. Na lista `$cmds` do passo de comparação byte a byte (~linhas 919–925): trocar `context --json` por
   `context --format=json` e remover `validate --strict` (a flag não existe).
2. Fazer o laço reprovar quando qualquer dos dois lados sair com rc≠0 e stdout vazio, para que
   dois vazios não virem BYTE_IDENTICAL.
**Acceptance criteria:**
- [x] nenhum comando da lista falha com flag desconhecida no binário atual (medido localmente)
- [x] YAML válido (`python3 scripts/check-workflow-yaml.py` rc=0)

---

## Wave 2 — Red team e fechamento
> Dependencies: Wave 1 auditada.

### ML-2A — Red team do erro tipado (hades-tf)
**Status:** ✅ Concluído — auditado em 2026-10-30. Comportamento de produção correto nos três comandos. Três lacunas de teste viram a Wave 3 (Regra Dura): F1 `newExitCodeError(256, …)` sai rc=0 no Unix (o construtor só barra 0); F2 remover `!isCommandUnderGuard` passa a suíte; F3 `TestJSONExit_WrappedExitCodeError_NotSilenced` testa a semântica do Go, não o `root.go` — trocar `err.(*ExitCodeError)` por `errors.As` também passa a suíte (conferido pelo arquiteto na leitura do teste; o arquiteto aprovou esse teste no ML-1A sem notar). F5 docstring descreve `errors.As`. Erratum da Wave 0: o §3.9 e o residual 6 supõem que `1>&-` provoca falha de escrita; no Go do macOS o fd fechado é reaberto como `/dev/null` (F4, limite documentado, sem correção). F6 aceito como residual.
**Files affected:** `docs/seguranca/2026-10-30-redteam-json-stderr-saida-silenciosa.md`
**Actions:**
1. A partir da leitura do diff, tentar fazer uma falha real sair silenciosa, ou o stderr sujar de novo.
2. Veredito por prescrição da Wave 0, com evidência.
**Acceptance criteria:**
- [x] cada prescrição com veredito e evidência
- [x] residual declarado

**Gates da wave:**
```bash
test -f docs/seguranca/2026-10-30-redteam-json-stderr-saida-silenciosa.md
```

---

## Wave 3 — Corretivos do red team (1 ML)
> Dependencies: Wave 2 auditada.

### ML-3A — Decisão de saída testável no `root.go` (apolo-tf) — F1, F2, F3, F5
**Status:** ⬜ Pendente
**Files affected:** `internal/commands/root.go`, `internal/commands/exit_code_error.go`, `internal/commands/json_exit_test.go`
**Actions:**
1. Extrair do `Execute()` uma função pura que recebe o comando e o erro e decide código de saída e
   se imprime (inclusive o ramo do `ExitCodeError`, a regra D7 e o caso padrão). `Execute()` só a chama.
2. F1: código fora de 1..255 não é silenciado (cai no caminho normal, rc 1); o construtor recusa o mesmo intervalo.
3. Testes de tabela em processo contra a função: `ExitCodeError{Code:1}` silencia; valor zero
   `&ExitCodeError{}`, `Code:256` e `fmt.Errorf("%w", …)` não silenciam; comando sob `guard` sai 2.
   Substituir `TestJSONExit_WrappedExitCodeError_NotSilenced` por esses casos.
4. F5: docstring do `ExitCodeError` descreve a type assertion direta.
**Acceptance criteria:**
- [ ] mutação B (`errors.As` no lugar da type assertion) reprova os testes novos
- [ ] mutação C (sem `!isCommandUnderGuard`) reprova os testes novos
- [ ] `Code:256` não sai silencioso (teste)
- [ ] `TestJSONExit_*` de stream continuam verdes

**Gates da wave:**
```bash
go build ./...
go vet ./internal/commands/
env PATH="$(dirname "$(command -v go)"):/usr/bin:/bin" go test ./internal/commands/ -count=1
make quality
```

---

## Triagem pós-v8 — 2026-10-17 (REQ-2026-10-17)

**Mudou de forma**: usage e gate corrigidos (#413); vivo só o AC1 — com violações, `validate --json` escreve "N violation(s) found" no stderr (`root.go:123`).

Evidência e tabela completa: `docs/governanca/2026-10-17-triagem-backlog-pre-v8.md`.
