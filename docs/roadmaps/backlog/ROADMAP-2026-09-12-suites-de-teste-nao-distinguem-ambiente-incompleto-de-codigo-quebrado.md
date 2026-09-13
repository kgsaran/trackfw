---
status: backlog
date: 2026-09-12
req: "docs/req/REQ-2026-09-12-suites-de-teste-nao-distinguem-ambiente-incompleto-de-codigo-quebrado.md"
squad: ""
---

# Roadmap: suites de teste nao distinguem ambiente incompleto de codigo quebrado

> Created: 2026-09-12 | Status: ⬜ Backlog

## Contexto

REQ: docs/req/REQ-2026-09-12-suites-de-teste-nao-distinguem-ambiente-incompleto-de-codigo-quebrado.md

O defeito é o **sinal apontar para o lugar errado**. Ambiente incompleto produz exatamente o mesmo
vermelho que código quebrado. O desenvolvedor/agente não sabe se deve corrigir código ou instalar
dependência.

Medido duas vezes no mesmo dia (2026-09-12):

1. `check-serve-api-file-security.sh` acusava `FAIL Python` com `pytest` ausente — corrigido com
   guarda no gate (molde para este roadmap).
2. `npm test` num worktree sem `node_modules` produziu **912 testes vermelhos** com
   `Cannot find module 'yaml'` — indistinguível de CLI quebrado. Após `npm ci`: tudo verde.

**O molde já existe:** `scripts/check-serve-api-file-security.sh` linhas 76–93 — guarda em duas
etapas com mensagem estruturada e rc=1 sem contabilizar FAIL de produto.

### Dependência da v8

Esta REQ toca superfícies que a `REQ-2026-09-12-v8-um-binario-muitos-canais` altera:

- **AC1 (Node) e AC2 (Python):** se entrar antes da v8, valor imediato pleno. Se depois, a guarda
  migra para a suíte de smoke da casquinha e o wheel — o mecanismo é o mesmo.
- **AC3 (Go):** **não desaparece com a v8.** O `go test ./...` é a única suíte de produto após a
  v8. A inconsistência já existe hoje e é o AC mais valioso.
- **AC4 (make quality):** sobrevive integralmente — `make quality` continuará existindo.

**Decisão de ordem (antes/depois da v8) é do arquiteto.** Por isso o roadmap fica em `backlog/`.

## Acceptance Criteria (consolidado)

Cada AC com falsificação nas duas direções:

- [ ] **AC1** — `npm test` sem `node_modules`: mensagem de pré-requisito estruturada + rc≠0 antes de qualquer `✖` de teste; com `node_modules`: suíte roda normalmente.
- [ ] **AC2** — `pytest pypi/tests` sem `pytest` instalado: mensagem de diagnóstico estruturada + rc=1 sem FAIL de produto; com `pytest`: suíte roda normalmente.
- [ ] **AC3** — `go test ./...` sem `git` no PATH: testes que chamam `exec.Command("git", ...)` diretamente em `validator_git_exec_test.go` emitem `t.Skip("git not available in PATH")` em vez de `t.Fatalf`; com `git`: testes rodam e reportam resultado real.
- [ ] **AC4** — `make quality` com dependência faltando: qualifica a causa antes de propagar rc≠0 genérico; com ambiente completo: sem overhead de diagnóstico.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

---

## Wave 1 — Guarda de pré-requisito na suíte Go (independente da v8)

> Dependencies: nenhuma. Este ML sobrevive à v8 e é o mais urgente.

### ML-1A — Guardar testes Go que chamam git sem exec.LookPath

**Status:** ⬜ Pendente
**Arquivos afetados:**
- `internal/validator/validator_git_exec_test.go`

**Contexto do defeito medido:**
As funções abaixo chamam `exec.Command("git", ...)` via helper `run()` sem verificar se `git` está
disponível. Se `git` estiver ausente: `t.Fatalf` → falha indistinguível de defeito de produto.

Funções afetadas (linha/nome):
- L70: `TestCredentialGuardModeDowngrade_GitDirWorkTreeRedirecionados_ContinuaDetectando`
- L124: `TestCredentialGuardModeDowngrade_GitConfigCountMalformado_NaoVacuidade`
- L138: `TestIsGitWorktree_LinkedWorktreeLegitimo_ContinuaFuncionando`

Molde já no repositório (`branch_prune_test.go` L422, `ship_test.go` L1201):
```go
if _, err := exec.LookPath("git"); err != nil {
    t.Skip("git not available in PATH")
}
```

**Ações:**
1. Adicionar `exec.LookPath("git")` + `t.Skip` no início de cada uma das 3 funções afetadas.
2. Verificar se há outros `func Test*` no pacote `validator` que chamem `exec.Command("git", ...)`
   sem guarda equivalente (git grep no pacote).

**Critérios de aceite:**
- [ ] AC3-positivo: com `git` removido do PATH (via `t.Setenv("PATH", "")`), os 3 testes listados
  emitem SKIP, não FAIL.
- [ ] AC3-negativo: com `git` presente, os mesmos testes rodam e reportam resultado real.
- [ ] `go test ./...` verde após a mudança.
- [ ] Nenhum outro teste no pacote `validator` chama `exec.Command("git", ...)` sem guarda.

**Comandos de validação:**
```bash
go test ./internal/validator/... -v -run "TestCredentialGuardModeDowngrade_GitDirWork|TestCredentialGuardModeDowngrade_GitConfigCountMalformado_NaoVacuidade|TestIsGitWorktree"
go test -timeout 2m ./...
```

---

## Wave 2 — Guarda de pré-requisito nas suítes Node e Python (antes ou depois da v8)

> Dependencies: Wave 1 concluída (independência de risco; Node e Python podem ser paralelos entre si).

### ML-2A — Guarda de pré-requisito no npm test

**Status:** ⬜ Pendente
**Arquivos afetados:**
- `npm/package.json` (script `test`)
- ou: script wrapper `npm/bin/test-with-prereq-check.js` (alternativa sem reescrever o entry)

**Contexto:**
`"test": "node --test tests/*.test.js"` — sem verificação de `node_modules`. Sem `commander` (ou
`yaml`, ou qualquer dep), o runner emite `✖` por arquivo como se fosse falha de produto.

**Ações:**
1. Adicionar verificação de pré-requisito antes de rodar os testes. Opções:
   a. Preflight inline no script `test`: checar se `node_modules` existe e tem tamanho não-zero.
   b. Script wrapper que verifica e delega.
2. Mensagem estruturada de diagnóstico quando `node_modules` ausente, rc≠0 distinto (sugere rc=2
   para distinguir de falha de teste rc=1).
3. Sem overhead quando `node_modules` presente.

**Critérios de aceite:**
- [ ] AC1-positivo: sem `node_modules`, `npm test` emite mensagem de pré-requisito + rc≠1 (sem `✖`).
- [ ] AC1-negativo: com `node_modules`, `npm test` roda normalmente.

**Ressalva v8:** se a v8 for implementada antes deste ML, o alvo passa a ser a suíte de smoke da
casquinha (`npm/tests/smoke/` ou equivalente). A guarda em si é idêntica — só a superfície muda.

---

### ML-2B — Guarda de pré-requisito no pytest

**Status:** ⬜ Pendente
**Arquivos afetados:**
- `Makefile` (target `test-python`)
- e/ou: `pypi/tests/conftest.py` (session-scoped autouse fixture de pré-requisito)

**Contexto:**
`python3 -m pytest pypi/tests -q` — sem verificação de `pytest` instalado. Se ausente:
`python3: No module named pytest` (rc=1) sem diagnóstico estruturado.

O molde para a guarda está em `scripts/check-serve-api-file-security.sh` linhas 76–93.

**Ações:**
1. Opção A (Makefile): adicionar verificação antes de `python3 -m pytest`:
   ```bash
   @python3 -m pytest --version >/dev/null 2>&1 || (printf 'ERRO: ambiente incompleto — pytest nao encontrado\n'; exit 1)
   ```
2. Opção B (conftest): `conftest.py` session fixture que verifica e skipa tudo com mensagem clara se
   pytest não detectar a si mesmo (edge case: conftest só roda se pytest já iniciou — prefer Opção A).

**Critérios de aceite:**
- [ ] AC2-positivo: sem `pytest`, `make test-python` emite mensagem de diagnóstico + rc=1 sem FAIL de produto.
- [ ] AC2-negativo: com `pytest`, `make test-python` roda normalmente.

**Ressalva v8:** análogo ao ML-2A — a guarda migra para o runner do wheel. Mecanismo idêntico.

---

## Wave 3 — Guarda no make quality

> Dependencies: Waves 1 e 2 concluídas (qualifica o que já foi corrigido nas suítes).

### ML-3A — Preflight de pré-requisitos no make quality

**Status:** ⬜ Pendente
**Arquivos afetados:**
- `Makefile` (target `quality` e/ou target `check-prereqs` novo)

**Ações:**
1. Adicionar target `check-prereqs` que verifica: `node`, `npm`, `python3`, `pytest`, `git`, `go`.
2. Chamar `check-prereqs` como primeira dependência de `quality`.
3. Mensagem estruturada por ferramenta ausente. rc=1 com diagnóstico, sem propagar rc genérico.

**Critérios de aceite:**
- [ ] AC4-positivo: com qualquer ferramenta ausente, `make quality` para em `check-prereqs` com
  mensagem identificando o que está faltando, antes de rodar qualquer suíte.
- [ ] AC4-negativo: com ambiente completo, `make quality` roda normalmente sem overhead perceptível.
- [ ] `make quality` verde após a mudança.

**Comandos de validação:**
```bash
make check-prereqs   # deve passar
PATH=/usr/bin make check-prereqs   # deve falhar com mensagem identificando tools ausentes
make quality
```

---

## Gate de fechamento do roadmap

```bash
# Verifica que os 3 testes afetados no Go têm exec.LookPath guard:
/usr/bin/grep -c "exec.LookPath" internal/validator/validator_git_exec_test.go
# Esperado: >= 3

# Verifica que make quality tem check-prereqs como dependência:
/usr/bin/grep "check-prereqs" Makefile
```
