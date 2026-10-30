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
**Files affected:** `internal/commands/root.go`, `internal/commands/validate.go`, `internal/commands/update.go`, `internal/commands/update_harness.go`, `internal/commands/json_exit_test.go`, `docs/cli-parity.md`
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
**Status:** ⬜ Pendente
**Files affected:** `docs/seguranca/2026-10-30-redteam-json-stderr-saida-silenciosa.md`
**Actions:**
1. A partir da leitura do diff, tentar fazer uma falha real sair silenciosa, ou o stderr sujar de novo.
2. Veredito por prescrição da Wave 0, com evidência.
**Acceptance criteria:**
- [ ] cada prescrição com veredito e evidência
- [ ] residual declarado

**Gates da wave:**
```bash
test -f docs/seguranca/2026-10-30-redteam-json-stderr-saida-silenciosa.md
make quality
```

---

## Triagem pós-v8 — 2026-10-17 (REQ-2026-10-17)

**Mudou de forma**: usage e gate corrigidos (#413); vivo só o AC1 — com violações, `validate --json` escreve "N violation(s) found" no stderr (`root.go:123`).

Evidência e tabela completa: `docs/governanca/2026-10-17-triagem-backlog-pre-v8.md`.
