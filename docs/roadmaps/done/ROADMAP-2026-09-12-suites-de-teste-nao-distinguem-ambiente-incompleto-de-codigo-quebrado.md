---
status: done
date: 2026-09-12
req: "docs/req/REQ-2026-09-12-suites-de-teste-nao-distinguem-ambiente-incompleto-de-codigo-quebrado.md"
squad: "hades-tf, artemis-tf, ares-tf"
---

# Roadmap: suites de teste nao distinguem ambiente incompleto de codigo quebrado

> Created: 2026-09-12 | Revisado: 2026-10-29 | Status: done

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
**Status:** ✅ Concluído — auditado em 2026-10-29. 10 prescrições. Três bloqueantes absorvidos abaixo: preservar GOCACHE/GOMODCACHE ao trocar HOME (5 `go build` em testes do `commands`, conferido por grep); `git --version` em vez de `LookPath` (LookPath não executa); AC3 exige `FAIL\t<pkg>`, porque "zero `--- FAIL`" passa com `os.Exit(0)`. Só `commands` precisa de isolamento de HOME (22 outros pacotes passam com HOME envenenado). O `commands` já tinha `Unsetenv CLAUDE_CONFIG_DIR`; falta o HOME.
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
- [x] As seis seções respondidas com evidência (comando e resultado)
- [x] Prescrições escritas em seção intitulada `Prescrições`
- [x] Nenhuma linha de implementação escrita

**Gates da wave:**
```bash
test -f docs/seguranca/2026-10-29-wave0-isolamento-ambiente-dos-testes.md
grep -q 'Prescrições' docs/seguranca/2026-10-29-wave0-isolamento-ambiente-dos-testes.md
```

---

## Wave 1 — Isolamento do HOME (#624) e pré-requisitos do make quality (2 MLs em paralelo)
> Dependencies: Wave 0 auditada. ML-1A e ML-1B tocam arquivos disjuntos.

### ML-1A — `internal/commands` não lê o ambiente pessoal do desenvolvedor (artemis-tf) — AC5, fecha #624
**Status:** ✅ Concluído — auditado em 2026-10-29. Medido pelo arquiteto: `env PATH="/usr/local/go/bin:/usr/bin:/bin" go test ./internal/commands/ -count=1` → ok com o HOME real (era 15 FAIL); `TestIsolatedHomeCoversBothChannels` PASS em `-v`; contra-braço do executor reprova com HOME real; nenhum HOME temporário sobra em `$TMPDIR`.
**Files affected:** `internal/commands/netexec_test.go`, `internal/commands/testenv_isolation_test.go`
**Actions:**
1. Estender o `TestMain` existente em `internal/commands/netexec_test.go` (não criar outro: o Go aceita
   um por pacote). Depois dos modos auxiliares HANG/SPAWN/SHORT_SLEEP e antes de `m.Run()`, nesta
   ordem (Wave 0, Prescrições 1 e 2): (a) capturar `GOCACHE`, `GOMODCACHE`, `GOPATH`, `GOENV` com
   `go env <VAR>`; (b) `MkdirTemp`, `Setenv HOME` e `USERPROFILE` (o `Unsetenv CLAUDE_CONFIG_DIR` já
   existe, mantenha); (c) re-`Setenv` as quatro variáveis Go capturadas; (d) após `m.Run()`, liberar
   escrita (`chmod -R u+w`) e `RemoveAll` do HOME temporário.
2. Criar `internal/commands/testenv_isolation_test.go` com o sentinela (Prescrição 3): `HOME` e
   `USERPROFILE` iguais ao HOME temporário, e `agentconfig.ClaudeConfigDir(...)` dentro dele.
3. Nenhum outro pacote: a Wave 0 mediu que só `commands` vaza.
**Acceptance criteria:**
- [x] `env PATH=<sem trackfw> go test ./internal/commands/ -count=1` rc=0 com o HOME real do desenvolvedor (que tem o guard global)
- [x] o mesmo com `CLAUDE_CONFIG_DIR` apontando para um diretório com o settings do guard
- [x] contra-braço: revertido o passo 1, o sentinela reprova
- [x] testes que fazem `t.Setenv("HOME", ...)` continuam verdes

**Gates da wave:**
```bash
go build ./...
go vet ./internal/commands/
env PATH="$(dirname "$(command -v go)"):/usr/bin:/bin" go test ./internal/commands/ -count=1
```

### ML-1B — `make quality` qualifica pré-requisito ausente antes de rodar (ares-tf) — AC4
**Status:** ✅ Concluído — auditado em 2026-10-29. Pré-requisitos medidos: `go`, `python3`, módulo `yaml` (`check-workflow-yaml.py` sai 1 sem ele), `git`, `bash`. `make check-prereqs` rc=0 completo; PATH sem `go` → rc=2 (o make devolve 2; a receita sai 1) nomeando `go`. Nenhum workflow de CI roda `make quality`.
**Files affected:** `Makefile`
**Actions:**
1. Medir o que `test`, `lint`, `parity-rest` e `parity-falsify` invocam de fato (binários e módulos
   Python como `yaml`). A lista sai da medição, não da REQ antiga.
2. Criar o target `check-prereqs` com uma linha `ERRO: ambiente incompleto — <ferramenta> não encontrado`
   por ausência e rc=1; torná-lo primeira dependência de `quality`.
**Acceptance criteria:**
- [x] com ferramenta ausente (PATH reduzido), `make check-prereqs` rc=1 e nomeia a ferramenta
- [x] com ambiente completo, `make check-prereqs` rc=0 sem saída extra
- [x] não rodar `make quality` neste ML (frente paralela); o arquiteto roda no fechamento

**Gates da wave:**
```bash
make check-prereqs
```

---

## Wave 2 — Falha rápida sem git (1 ML)
> Dependencies: Wave 1 auditada (ML-2A edita o mesmo `TestMain` do ML-1A).

### ML-2A — `git` ausente produz uma linha de diagnóstico, não 181 falhas (artemis-tf) — AC3
**Status:** ✅ Concluído — auditado em 2026-10-29. Medido pelo arquiteto com PATH sem `git`: os 4 pacotes rc=1, 1 linha de diagnóstico, 0 `--- FAIL`, 0 `PASS`/`ok` (eram 233 `--- FAIL`). `TestGitGuardFailsFastWhenGitAbsent` com `path_vazio` e `git_falso_127` PASS nos 4 pacotes em `-v`. Com `git`, os 4 verdes. Contra-braço do executor: com `LookPath`, o braço do git falso reprova. Observação: os helpers do sentinela estão duplicados nos 4 pacotes (~80 linhas cada); aceito por ser código de teste sem pacote compartilhado.
**Files affected:** `internal/commands/netexec_test.go`, `internal/commands/testenv_isolation_test.go`, `internal/auditsurface/main_test.go`, `internal/pathguard/main_test.go`, `internal/validator/main_test.go`
**Actions:**
1. No `TestMain` de cada um dos 4 pacotes, como primeira instrução (no `commands`, logo depois dos
   modos auxiliares e antes do isolamento de HOME): `exec.Command("git", "--version").Run()`; se
   falhar, imprimir em stderr `ambiente incompleto: git não encontrado no PATH` e `os.Exit(2)`.
   Não usar `exec.LookPath` (Wave 0, Prescrição 7: não executa, aceita git quebrado).
2. Sentinela em cada um dos 4 pacotes, no próprio `main_test.go` (no `commands`, em
   `testenv_isolation_test.go`): re-executa `os.Args[0] -test.run=^$` com `PATH` num `t.TempDir()`
   vazio e afirma exit code 2, a linha no stderr e ausência de `PASS`. Segundo braço só fora do
   Windows: `PATH` com um `git` falso que sai 127 (Prescrições 9 e 10).
**Acceptance criteria:**
- [x] com PATH contendo todo `/usr/bin` menos `git`, cada um dos 4 pacotes: rc≠0, a linha de diagnóstico no stderr, `FAIL` + nome do pacote na saída, nenhum `PASS`/`ok` e **zero** `--- FAIL`
- [x] com `git` presente, nenhuma linha de diagnóstico; os 4 pacotes verdes
- [x] `go test ./internal/pathguard/` verde

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
**Status:** ✅ Concluído — auditado em 2026-10-29. 7 mutações; 5 capturadas pelos sentinelas. Três defeitos de mesma causa viram a Wave 4 (Regra Dura): D1 nada prova que a guarda roda antes do `MkdirTemp`; D3 o sentinela do `validator` não confere `ClaudeConfigDir`; D5 `check-prereqs` aceita bash 3.2 (o `/bin/bash` do macOS), mas 15 scripts usam `declare -A`/`mapfile` (conferido: `check-gates-falsify.sh:2350`). D2 e D4 ficam como residual declarado no parecer.
**Files affected:** `docs/seguranca/2026-10-29-redteam-isolamento-ambiente-dos-testes.md`
**Actions:**
1. Tentar fazer um teste ler o HOME real ou passar verde sem `git`, a partir da leitura do código entregue.
2. Conferir cada prescrição da Wave 0 contra o diff.
**Acceptance criteria:**
- [x] cada prescrição com veredito e evidência
- [x] residual declarado

**Gates da wave:**
```bash
test -f docs/seguranca/2026-10-29-redteam-isolamento-ambiente-dos-testes.md
```

---

## Wave 4 — Corretivos do red team (2 MLs em paralelo)
> Dependencies: Wave 3 auditada. ML-4A e ML-4B tocam arquivos disjuntos.

### ML-4A — Sentinelas de ordem da guarda e do ClaudeConfigDir no validator (artemis-tf) — D1, D3
**Status:** ✅ Concluído — auditado em 2026-10-29. `path_vazio` de `commands` e `validator` agora afirma TMPDIR do filho vazio; contra-braço do executor (guarda depois do `MkdirTemp`) reprova nomeando o diretório nos dois pacotes. `TestIsolatedHomeCoversBothChannels` do validator confere `ClaudeConfigDir`; contra-braço por `CLAUDE_CONFIG_DIR` falso reprova só a verificação nova. Nomes conferidos pelo arquiteto em `-v`.
**Files affected:** `internal/commands/testenv_isolation_test.go`, `internal/validator/main_test.go`
**Actions:**
1. D1: no braço `path_vazio` do `TestGitGuardFailsFastWhenGitAbsent` do `commands` e do `validator` (os dois que criam HOME temporário), apontar `TMPDIR`, `TMP` e `TEMP` do filho para um `t.TempDir()` próprio e afirmar que ele está vazio depois que o filho sai.
2. D3: acrescentar ao `TestIsolatedHomeCoversBothChannels` do `validator` a verificação de `agentconfig.ClaudeConfigDir(agentconfig.DefaultResolver)` dentro de `isolatedTestHome`, como no `commands`.
**Acceptance criteria:**
- [x] contra-braço D1: com a guarda movida para depois do `MkdirTemp`, o braço reprova nomeando o diretório; na ordem certa, passa
- [x] contra-braço D3: sem o `Setenv HOME` do validator, a verificação nova reprova
- [x] `go test ./internal/validator/ ./internal/commands/` verdes

### ML-4B — `check-prereqs` exige bash 4+ (ares-tf) — D5
**Status:** ✅ Concluído — auditado em 2026-10-29. Medido pelo arquiteto: PATH com `/bin/bash` 3.2 → rc=2 nomeando só bash 4+; ambiente atual (bash 5.3) → rc=0 sem saída.
**Files affected:** `Makefile`
**Actions:**
1. Trocar a checagem de `bash` por uma que exija `BASH_VERSINFO[0] >= 4` (escapando `$` para o make), com a mensagem `ERRO: ambiente incompleto — bash 4+ não encontrado (os scripts usam declare -A e mapfile)`.
**Acceptance criteria:**
- [x] com PATH cujo primeiro `bash` é `/bin/bash` 3.2, `make check-prereqs` reprova nomeando bash 4+
- [x] no ambiente atual (bash 5.3), rc=0 sem saída

**Gates da wave:**
```bash
go build ./...
go test ./internal/validator/ ./internal/auditsurface/ ./internal/pathguard/ -count=1
env PATH="$(dirname "$(command -v go)"):/usr/bin:/bin" go test ./internal/commands/ -count=1
make quality
```

---

## Triagem pós-v8 — 2026-10-17 (REQ-2026-10-17)

**Mudou de forma**: Node/Python sumiram; vivos AC3 (3 testes de `validator_git_exec_test.go` sem `LookPath`+`Skip`) e AC4 (`make quality` sem checagem de pré-requisitos).

Evidência e tabela completa: `docs/governanca/2026-10-17-triagem-backlog-pre-v8.md`.
