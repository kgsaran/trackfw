---
status: wip
date: 2026-09-12
req: "docs/req/REQ-2026-09-12-suites-de-teste-nao-distinguem-ambiente-incompleto-de-codigo-quebrado.md"
squad: "hades-tf, artemis-tf, ares-tf"
---

# Roadmap: suites de teste nao distinguem ambiente incompleto de codigo quebrado

> Created: 2026-09-12 | Revisado: 2026-10-29 | Status: wip

## Contexto

REQ: docs/req/REQ-2026-09-12-suites-de-teste-nao-distinguem-ambiente-incompleto-de-codigo-quebrado.md
(seção "Revisão de escopo — 2026-10-29" é o escopo vigente).

Fecha o issue **#624**, absorvido por mesmo sintoma: o resultado de `go test` depende da máquina.

Três defeitos, um sintoma (o sinal aponta para o lugar errado):

| AC | defeito | medido em `main` `d0dba293` |
|---|---|---|
| AC5 (#624) | testes de `internal/commands` leem o `~/.claude/settings.json` real | 15 FAIL com HOME real e PATH sem trackfw; 0 com HOME temporário |
| AC3 | `git` ausente vira 181 falhas de produto | 233 `--- FAIL` em `auditsurface`, `commands`, `pathguard`, `validator` |
| AC4 | `make quality` não qualifica pré-requisito ausente | `quality: test lint parity`; `parity-rest` chama `python3`, scripts `bash` |

AC1 e AC2 (Node/Python) foram retirados: as suítes não existem desde a v8.

**Moldes no repositório:** `internal/validator/main_test.go` (isolamento de HOME/USERPROFILE/
CLAUDE_CONFIG_DIR + sentinela `TestIsolatedHomeCoversBothChannels`); `scripts/check-serve-api-file-security.sh`
(diagnóstico de pré-requisito com rc≠0).

### Por que Wave 1 e Wave 2 são sequenciais em `internal/commands/netexec_test.go`

ML-1A (isolamento) e ML-2A (falha rápida sem git) editam o mesmo `TestMain`. Não podem rodar juntos.
ML-1B (Makefile) é disjunto e roda em paralelo com ML-1A.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

---

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model do isolamento de ambiente dos testes (hades-tf)
**Status:** ⬜ Pendente
**Files affected:** `docs/seguranca/2026-10-29-wave0-isolamento-ambiente-dos-testes.md`
**Actions:**
1. Inventário: liste todo pacote em `internal/` cujos testes (ou o código que eles exercitam) leem
   `HOME`, `USERPROFILE`, `CLAUDE_CONFIG_DIR` ou chamam `homedir.Dir`/`os.UserHomeDir`, e diga quais
   têm isolamento no `TestMain`. Feche com o comando usado, sem `| head`.
2. Escrita no HOME real: algum teste hoje **escreve** no `~/.claude` (ou `~/.codex`, `~/.gemini`) de quem
   roda? Meça com um HOME descartável e `find <home> -newer <marca>` após `go test ./...`. Rode o
   `go test` com `PATH` sem trackfw.
3. Herança: filhos re-executados (`os.Args[0]` no `TestMain` do `commands`) e subprocessos `git`
   herdam o HOME temporário. O que muda para o `git` (config global, identidade de commit) e se algum
   teste depende do `~/.gitconfig` do desenvolvedor.
4. Falha rápida sem `git` (AC3): ameaça de a guarda virar skip silencioso ou ser contornada; como
   impedir que o CI fique verde sem ter rodado (o CI tem `git`; prescreva o que o teste-sentinela afirma).
5. Alvos de falsificação nas duas direções para AC5 e AC3: o que quebra se o isolamento regredir, e o
   que quebra se ele for longe demais (ex.: teste que define o próprio HOME via `t.Setenv` passa a
   ser sobrescrito).
6. Prescrições numeradas para ML-1A, ML-1B e ML-2A, e residual declarado.
**Acceptance criteria:**
- [ ] As seis seções respondidas com evidência (comando e resultado)
- [ ] Prescrições escritas em seção intitulada `Prescrições`
- [ ] Nenhuma linha de implementação escrita

**Gates da wave:**
```bash
test -f docs/seguranca/2026-10-29-wave0-isolamento-ambiente-dos-testes.md
grep -q 'Prescrições' docs/seguranca/2026-10-29-wave0-isolamento-ambiente-dos-testes.md
```

---

## Wave 1 — Isolamento do HOME (#624) e pré-requisitos do make quality (2 MLs em paralelo)
> Dependencies: Wave 0 auditada. ML-1A e ML-1B tocam arquivos disjuntos.

### ML-1A — `internal/commands` não lê o ambiente pessoal do desenvolvedor (artemis-tf) — AC5, fecha #624
**Status:** ⬜ Pendente
**Files affected:** `internal/commands/netexec_test.go`, `internal/commands/testenv_isolation_test.go`
**Actions:**
1. Estender o `TestMain` existente em `internal/commands/netexec_test.go` (não criar outro: o Go aceita
   um por pacote). Depois dos modos auxiliares HANG/SPAWN/SHORT_SLEEP e antes de `m.Run()`: criar HOME
   temporário, `Setenv HOME` e `USERPROFILE`, `Unsetenv CLAUDE_CONFIG_DIR`, remover o diretório após
   `m.Run()`. Copiar o molde de `internal/validator/main_test.go`.
2. Criar `internal/commands/testenv_isolation_test.go` com o teste-sentinela equivalente a
   `TestIsolatedHomeCoversBothChannels`.
3. Aplicar as prescrições da Wave 0 sobre outros pacotes **somente** se a Wave 0 os apontar; nesse
   caso o arquiteto amplia `Files affected` antes.
**Acceptance criteria:**
- [ ] `env PATH=<sem trackfw> go test ./internal/commands/ -count=1` rc=0 com o HOME real do desenvolvedor (que tem o guard global)
- [ ] o mesmo com `CLAUDE_CONFIG_DIR` apontando para um diretório com o settings do guard
- [ ] contra-braço: revertido o passo 1, o sentinela reprova
- [ ] testes que fazem `t.Setenv("HOME", ...)` continuam verdes

**Gates da wave:**
```bash
go build ./...
go vet ./internal/commands/
env PATH="$(dirname "$(command -v go)"):/usr/bin:/bin" go test ./internal/commands/ -count=1
```

### ML-1B — `make quality` qualifica pré-requisito ausente antes de rodar (ares-tf) — AC4
**Status:** ⬜ Pendente
**Files affected:** `Makefile`
**Actions:**
1. Medir o que `test`, `lint`, `parity-rest` e `parity-falsify` invocam de fato (binários e módulos
   Python como `yaml`). A lista sai da medição, não da REQ antiga.
2. Criar o target `check-prereqs` com uma linha `ERRO: ambiente incompleto — <ferramenta> não encontrado`
   por ausência e rc=1; torná-lo primeira dependência de `quality`.
**Acceptance criteria:**
- [ ] com ferramenta ausente (PATH reduzido), `make check-prereqs` rc=1 e nomeia a ferramenta
- [ ] com ambiente completo, `make check-prereqs` rc=0 sem saída extra
- [ ] não rodar `make quality` neste ML (frente paralela); o arquiteto roda no fechamento

**Gates da wave:**
```bash
make check-prereqs
```

---

## Wave 2 — Falha rápida sem git (1 ML)
> Dependencies: Wave 1 auditada (ML-2A edita o mesmo `TestMain` do ML-1A).

### ML-2A — `git` ausente produz uma linha de diagnóstico, não 181 falhas (artemis-tf) — AC3
**Status:** ⬜ Pendente
**Files affected:** `internal/commands/netexec_test.go`, `internal/auditsurface/main_test.go`, `internal/pathguard/main_test.go`, `internal/validator/main_test.go`
**Actions:**
1. No `TestMain` de cada um dos 4 pacotes, antes de `m.Run()` (e no `commands`, depois dos modos
   auxiliares): `exec.LookPath("git")`; se falhar, imprimir em stderr
   `ambiente incompleto: git não encontrado no PATH` e `os.Exit(2)`.
2. Aplicar as prescrições da Wave 0 sobre o sentinela que impede o CI de ficar verde sem rodar.
**Acceptance criteria:**
- [ ] com PATH contendo todo `/usr/bin` menos `git`, os 4 pacotes emitem a linha e **zero** `--- FAIL`
- [ ] com `git` presente, nenhuma linha de diagnóstico; os 4 pacotes verdes
- [ ] `go test ./internal/pathguard/` verde

**Gates da wave:**
```bash
go build ./...
go vet ./internal/commands/ ./internal/auditsurface/ ./internal/pathguard/ ./internal/validator/
go test ./internal/auditsurface/ ./internal/pathguard/ ./internal/validator/ -count=1
env PATH="$(dirname "$(command -v go)"):/usr/bin:/bin" go test ./internal/commands/ -count=1
```

---

## Wave 3 — Red team e fechamento
> Dependencies: Wave 2 auditada.

### ML-3A — Red team do isolamento e da falha rápida (hades-tf)
**Status:** ⬜ Pendente
**Files affected:** `docs/seguranca/2026-10-29-redteam-isolamento-ambiente-dos-testes.md`
**Actions:**
1. Tentar fazer um teste ler o HOME real ou passar verde sem `git`, a partir da leitura do código entregue.
2. Conferir cada prescrição da Wave 0 contra o diff.
**Acceptance criteria:**
- [ ] cada prescrição com veredito e evidência
- [ ] residual declarado

**Gates da wave:**
```bash
test -f docs/seguranca/2026-10-29-redteam-isolamento-ambiente-dos-testes.md
make quality
```

---

## Triagem pós-v8 — 2026-10-17 (REQ-2026-10-17)

**Mudou de forma**: Node/Python sumiram; vivos AC3 (3 testes de `validator_git_exec_test.go` sem `LookPath`+`Skip`) e AC4 (`make quality` sem checagem de pré-requisitos).

Evidência e tabela completa: `docs/governanca/2026-10-17-triagem-backlog-pre-v8.md`.
