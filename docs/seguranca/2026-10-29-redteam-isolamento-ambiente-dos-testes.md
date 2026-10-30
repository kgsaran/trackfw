# Red Team — ML-3A: Isolamento de Ambiente dos Testes

> Roadmap: `ROADMAP-2026-09-12-suites-de-teste-nao-distinguem-ambiente-incompleto-de-codigo-quebrado.md`
> Wave 0: `docs/seguranca/2026-10-29-wave0-isolamento-ambiente-dos-testes.md`
> Data: 2026-10-29 | ML-3A | Hades-tf

## Veredito: APROVA COM RESSALVAS

Wave 3 / ML-3A aprovada. Merge do PR condicionado aos MLs de D1/D3/D5 (Regra Dura:
mesma causa, mesmo PR deve permanecer aberto). Vetores (a) e (b direto) bloqueados;
(c) alcançado via bash versão (D5, LOW). Cinco achados LOW (D1–D5). D4 não alcança
`make quality`. D3 é pré-existente em `main`.

---

## Método

Leitura do diff completo antes de qualquer execução. Quatro vetores de ataque executados
em cópia do repositório criada com `git archive HEAD | tar -x -C <tmp>`. Sete arms de
mutação executados sequencialmente, com resultado medido. Vetores AC3 rodados no
repositório real com `go test` por pacote. Nenhum `go test ./...` inteiro.

---

## Vetores de Ataque

### (a) Fazer um teste de `commands` ler o HOME real

**Resultado: não conseguido. A proteção é sólida.**

Mutação 1 — deletar `os.Setenv("HOME", home)` no TestMain:

```
testenv_isolation_test.go:47: HOME = "/Users/kgsaran", want the synthetic home "..."
testenv_isolation_test.go:69: agentconfig.ClaudeConfigDir = "/Users/kgsaran/.claude", want path rooted under synthetic home
--- FAIL: TestIsolatedHomeCoversBothChannels
```

Mutação 2 — deletar `os.Setenv("USERPROFILE", home)`:

```
testenv_isolation_test.go:54: USERPROFILE = "", want the synthetic home "..."
--- FAIL: TestIsolatedHomeCoversBothChannels
```

Mutação 3 — deletar `os.Unsetenv("CLAUDE_CONFIG_DIR")`, rodar com
`CLAUDE_CONFIG_DIR=/Users/kgsaran/.claude`:

```
testenv_isolation_test.go:69: agentconfig.ClaudeConfigDir = "/Users/kgsaran/.claude", want path rooted under synthetic home
--- FAIL: TestIsolatedHomeCoversBothChannels
```

Nota: esta mutação só produz FAIL quando CLAUDE_CONFIG_DIR está definida no ambiente do
desenvolvedor. Quem não tem a variável não é afetado; o sentinela só captura o canal ativo.
Isso é comportamento correto: a variável controla o canal A2, e o sentinela o testa quando
o canal A2 existe.

Grep em testes de `commands` que constroem env explicitamente (`cleanEnvForReentry`,
`mlScopeGitEnv`, `barrier_fragment_test.go:273`): todos partem de `os.Environ()` que, a
essa altura, já tem `HOME=synthetic`. Nenhum subprocess lê o HOME real.

Verificado em `internal/homedir/homedir.go:32`: `os.Getenv("HOME")` retorna o HOME sintético
(não vazio) antes de qualquer fallback para `os.UserHomeDir()`. O caminho de fallback não é
alcançado.

### (b) Fazer algum dos 4 pacotes passar verde sem `git` funcional

**Resultado: não conseguido no caso direto. Bypass de cache confirmado (D4, LOW).**

AC3 end-to-end com `PATH=/usr/local/go/bin` (go presente, git ausente) nos 4 pacotes:

| Pacote | rc | `--- FAIL` | `FAIL\t<pkg>` | Diagnóstico |
|---|---|---|---|---|
| `internal/commands` | 1 | 0 | 1 | presente |
| `internal/auditsurface` | 1 | 0 | 1 | presente |
| `internal/pathguard` | 1 | 0 | 1 | presente |
| `internal/validator` | 1 | 0 | 1 | presente |

Vetor de cache: rodado `pathguard` duas vezes sem `-count=1` (segunda saída: `(cached) ok`).
Terceira execução com PATH sem git: `FAIL` com rc=1 e diagnóstico. O cache é invalidado
pela mudança de PATH porque o TestMain executa `git` diretamente.

**Achado D4 — bypass de cache via wrapper git (LOW)**

Um wrapper `git` que passa normalmente mas retorna 127 quando `BREAK_GIT=1` permite
re-executar um pacote cacheado sem invalidar o cache:

```
PATH=<gitwrap>:$PATH  go test ./internal/pathguard/       → ok (primeira execução)
PATH=<gitwrap>:$PATH  go test ./internal/pathguard/       → (cached) ok
BREAK_GIT=1  PATH=<gitwrap>:$PATH  go test ./internal/pathguard/  → (cached) ok  rc=0
```

O cache key inclui PATH mas não variáveis lidas pelos subprocessos. Como o PATH não muda
(o wrapper permanece), a chave é idêntica e o resultado cacheado é servido.

Severidade LOW: requer pre-colocação deliberada de um wrapper no PATH e controle de
`BREAK_GIT=1`. Em uso normal, quando git some do PATH a chave muda e o cache é invalidado.
`-count=1` força re-execução e fecha este vetor completamente.

`make test` usa `go test -timeout 2m ./...` sem `-count=1`. O comportamento de
invalidação de cache por mudança de ambiente se aplica ao caso normal (PATH muda).
Para o vetor de wrapper, `-count=1` fecha o bypass.

`make quality` não é afetado: `check-prereqs` executa `git --version` frescamente antes
de `test`. Medido: `BREAK_GIT=1 PATH=<gitwrap>:$PATH make check-prereqs` → `ERRO: ambiente
incompleto — git não encontrado`, rc=1. O bypass limita-se a `make test` direto e `go test`.

Instância natural (hipótese, não medido): o shim `/usr/bin/git` do macOS que dispara
`xcrun` quando o Xcode Command Line Tools estão instalados mas foram removidos
posteriormente — o PATH não muda (o arquivo existe) mas `git --version` começa a falhar.
Com testes cacheados, o resultado anterior permaneceria servido. `make quality` pegaria
porque `check-prereqs` executa git diretamente.

Mutações 4 e 5 medidas em `validator` e em `commands`. Em `commands`, o sentinela
`TestGitGuardFailsFastWhenGitAbsent` strips os TRACKFW_TEST_* do env do filho antes de
relançar, então os modos HANG/SPAWN/SHORT_SLEEP não são atingidos e a re-execução chega
diretamente ao guard — mesma estrutura do `validator`. Resultados em `commands`:

M4 (`commands`):
```
testenv_isolation_test.go:125: exit code = 0, want 2
testenv_isolation_test.go:125: stdout contains PASS (tests ran when they should not)
--- FAIL: TestGitGuardFailsFastWhenGitAbsent/git_falso_127
```

M5 (`commands`):
```
testenv_isolation_test.go:100: exit code = 0, want 2
testenv_isolation_test.go:125: exit code = 0, want 2
--- FAIL: TestGitGuardFailsFastWhenGitAbsent/path_vazio
--- FAIL: TestGitGuardFailsFastWhenGitAbsent/git_falso_127
```

O pacote `auditsurface` e `pathguard` têm a mesma implementação de guard e sentinelas
equivalentes por construção; não medidos separadamente.

Mutação 4 — `exec.LookPath("git")` em vez de `exec.Command("git","--version").Run()`
no validator (braço `git_falso_127`):

```
main_test.go:152: exit code = 0, want 2
main_test.go:152: stderr does not contain "ambiente incompleto..."
main_test.go:152: stdout contains PASS (tests ran when they should not)
--- FAIL: TestGitGuardFailsFastWhenGitAbsent/git_falso_127
```

O braço `path_vazio` teria passado com LookPath (porque não há nenhum executável); o braço
`git_falso_127` captura a diferença entre "existe no PATH" e "executa com sucesso".

Mutação 5 — `os.Exit(0)` em vez de `os.Exit(2)` no validator:

```
main_test.go:134: exit code = 0, want 2
main_test.go:152: exit code = 0, want 2
--- FAIL: TestGitGuardFailsFastWhenGitAbsent/path_vazio
--- FAIL: TestGitGuardFailsFastWhenGitAbsent/git_falso_127
```

Ambos os braços falham. Exit(0) satisfaz "zero `--- FAIL`" (nenhum teste roda), mas não
satisfaz o AC3 revisado que exige rc≠0 e `FAIL\t<pkg>`.

### (c) Fazer `check-prereqs` passar com ferramenta ausente ou quebrada

**Resultado: conseguido para bash versão (D5, LOW). Não conseguido para os casos diretos.**

Ferramenta `python3` presente mas sem módulo `yaml`:

```
ERRO: ambiente incompleto — yaml (módulo Python) não encontrado
make: *** [check-prereqs] Error 1
```

Ferramenta `git` substituída por fake que imprime versão e sai 0: `check-prereqs` passa.
Este é o residual declarado — `--version` verifica presença funcional, não operabilidade
plena. O mesmo limite existe no TestMain.

`SKIP_PREREQS` ou qualquer bypass por variável de ambiente: não existe no Makefile.
Prescrição 6 atendida.

`make test` contorna `check-prereqs` (executa `go test` diretamente sem o prereq).
Comportamento esperado: `quality` é o gate; `test` é o runner cru.

Enumeração de ferramentas verificada por leitura dos guards, não por execução com PATH
curado: `parity-rest` chama 46 scripts; `make -n quality` listou todos. Ferramentas com
chamadas reais verificadas:

- `node` / `npm` — `check-shim-byte-identity.sh`: `command -v node` + SKIP nomeado se
  ausente (CI: FAIL). `check-install-restriction.sh`: `command -v node` + `command -v npm`
  + SKIP nomeado (CI: FAIL). `check-tty-detection.sh`: npm aparece apenas em comentário
  histórico; não invocado. `check-channels-content.sh --self-test`: modo `--self-test`
  cria stubs npm em `$SCRATCH` e não requer node real; o guard `command -v node` pertence
  ao modo `--local` (branch diferente), não ao mode `--self-test` que `make quality` usa.
- `curl` — `check-install-checksum.sh`, `check-install-version-pin.sh`,
  `check-channels-content.sh --self-test`: criam stubs em `$STUB_BIN/curl`; nunca atingem
  curl real nos modos usados por `make quality`.
- `gh` — `check-ci-workflow-pin-parity.sh`: sem invocação real (gh aparece só em string
  `sed`). `check-falsify-recalibrate.sh`: usa gh falso criado em `$SCRATCH`.
  `check-roadmap-slug-matching.sh`: nenhuma invocação real identificada.
- `jq` — `check-emitting-capture-fallback.sh` e `check-unguarded-capture-rc.sh`:
  referenciado apenas em comentários e como padrão de string no parser da segunda; nunca
  invocado como binário.

- `check-gates-falsify.sh` — chamado por `run-gates-falsify-parallel.sh` (em `parity-falsify`
  que é parte de `parity` que é parte de `quality`): jq aparece apenas como string dentro
  de fixtures de teste (o script analisa padrões de captura, não executa jq); gh não invocado.
- `check-tty-detection.sh` — usa `GO_BIN` (Go binary), não npm/node; npm aparece só em
  comentário histórico.
- `pip` / `packaging` — `check-wheel-filename.sh`: `import packaging` em inline Python;
  `ImportError` resulta em SKIP em modo local, FAIL em CI. `build_wheel.py` usa apenas
  stdlib (confirmado: sem import packaging). `check-git-branch-guard-hook-schema.sh` e
  `check-interpolated-path-in-python.sh`: referências a pip apenas em comentários; sem
  invocação real do binário pip.
- Bare `python` — aparece apenas em scripts Windows (`windows-repro/`) e em strings de
  fixture, nunca como chamada direta em scripts chamados por `make quality`.

**Achado D5 — bash versão não verificada em check-prereqs (LOW, AC4)**

Todos os 5 scripts usam `#!/usr/bin/env bash` (shebang verificado), logo resolvem o bash
do PATH. `mapfile`, `declare -A` e `readarray` (bash 4.0+) são usados em
`check-crlf-normalize-capture.sh`, `check-serve-address-parity.sh`,
`check-write-containment.sh`, `check-unguarded-capture-rc.sh` e `check-orphan-gates.sh`.
Todos chamados por `parity-rest` (em `quality`). macOS inclui bash 3.2 em `/bin/bash`.
`check-prereqs` verifica apenas `bash --version >/dev/null 2>&1` — bash 3.2 passa.
Os scripts falham com syntax error; `make quality` falharia em `parity-rest` (inferido:
`make quality` não foi rodado diretamente nesta sessão).

Medido diretamente: `check-orphan-gates.sh:331` imprime `declare: -A: invalid option` em
bash 3.2 desta máquina. CI não inspecionado nesta sessão.

Mitigação parcial: em CI (linux/apt/nix) e em máquinas com Homebrew bash instalado, `PATH`
resolve bash 5.x. O defeito se manifesta em stock macOS quando somente go e python3 estão
no Homebrew mas não bash. Severidade LOW (não silencia, falha com syntax error legível).
Recomenda-se adicionar verificação de versão mínima ao `check-prereqs` como ML adicional nesta REQ:
```bash
bash -c '[[ ${BASH_VERSINFO[0]} -ge 4 ]]' >/dev/null 2>&1 \
  || { printf 'ERRO: ambiente incompleto — bash 4.0+ necessário\n'; _fail=1; }
```

Conclusão: as 5 ferramentas verificadas por `check-prereqs` cobrem os hard-fail reais para
ambientes com bash 4.0+. Scripts que requerem node/npm ou packaging Python SKIP
graciosamente em modo local (FAIL em CI onde a ferramenta é declarada no workflow).
Gap documentado (D5): bash 3.2 passa `check-prereqs` mas faz `parity-rest` falhar com
syntax error em stock macOS sem Homebrew bash.

### (d) Efeitos colaterais do isolamento

**Achado D1 — Ordenação do guard não tem sentinela (LOW)**

Mutação 6 — mover o guard de git para depois de `MkdirTemp` em `commands`:
O binário de teste compilado com a mutação, ao ser re-executado pelo sentinela com PATH vazio,
produziu **2 diretórios temporários vazados** em `$TMPDIR`:

```
trackfw-commands-test-home-831098378
trackfw-commands-test-home-1888291282
```

O sentinela `TestGitGuardFailsFastWhenGitAbsent` passou **verde** (rc=2, diagnóstico presente,
sem PASS). Ele não verifica se um `trackfw-*-test-home-*` foi criado e não removido.

A implementação real tem a ordem correta (guard linha 90, MkdirTemp linha 121, comentário
"Runs BEFORE MkdirTemp so no temporary HOME leaks on incomplete environments"). Mas não há
loop-breaker para essa propriedade de ordenação.

Nota sobre os 2 diretórios vazados acima: são artefatos da mutação 6 executada em sessão
anterior. A deleção foi negada pela sandbox de execução; os diretórios estão vazios e
inócuos. O repositório real não os cria — confirmado por set-diff antes/após `go test
-count=1 ./internal/commands/`: contagem antes=2, após=2 (os mesmos dois).

**Limpeza vs. chmod walk:**
`internal/commands/netexec_test.go` faz chmod walk antes de `RemoveAll`; `internal/validator/main_test.go`
não. Medido no macOS real: `go test -count=1 ./internal/validator/` → 0 diretórios
`trackfw-validator-test-home-*` em `$TMPDIR` após conclusão. O `RemoveAll` sem chmod walk
é suficiente quando os arquivos sob o HOME sintético são criados pelo próprio processo de
teste (permissões de escrita garantidas). Diferença de implementação documentada, não defeito.

**Achado D2 — P2 (GOCACHE re-injeção) não tem sentinela (LOW)**

Mutação 7 — remover re-injeção de GOCACHE/GOMODCACHE em `commands`. Com `GOPROXY=off`:

```
internal/commands/adr.go:8:2: module lookup disabled by GOPROXY=off
--- FAIL: TestUnknownCommand_NeverExecutesExternalBinary
```

A falha aparece como product FAIL (o go build computa GOMODCACHE a partir do HOME sintético
e não encontra os módulos). Nenhum sentinela nomeado captura isso — o desenvolvedor veria
falhas de produto, não de ambiente. A implementação real tem a re-injeção (linhas 103-151).

Contra-braço (M7): `GOPROXY=off go test -count=1 -run TestUnknownCommand_NeverExecutesExternalBinary ./internal/commands/` no repositório real (com re-injeção presente) → `ok` em 1.2s. A implementação actual não produz este defeito.

**Achado D3 — Validator sentinel ausente check (c) (LOW)**

`internal/validator/main_test.go::TestIsolatedHomeCoversBothChannels` verifica HOME e
USERPROFILE mas não `agentconfig.ClaudeConfigDir(agentconfig.DefaultResolver)`. A versão em
`commands/testenv_isolation_test.go` tem os 3 checks (Prescrição 3). O isolamento funciona
(HOME→sintético→ClaudeConfigDir→sintético), mas o sentinela não o afirma.

Medição do gap: mutante com `os.Unsetenv("CLAUDE_CONFIG_DIR")` removido do validator
TestMain, rodado com `CLAUDE_CONFIG_DIR=/Users/kgsaran/.claude`:

```
--- PASS: TestIsolatedHomeCoversBothChannels (0.00s)
PASS
```

O sentinela passa **verde** mesmo com `ClaudeConfigDir` resolvendo para o diretório real do
desenvolvedor. Confirma que a ausência do check (c) no validator é um gap mensurável.

O `validator_git_branch_guard.go` chama ClaudeConfigDir nas linhas 210, 434, 494, 561, 637.
Um refactor que trocar `homedir.Dir()` por `ClaudeConfigDir` numa superfície sem isolamento
não seria capturado pelo sentinela do validator.

**Observação W1 — Windows PATH: dedup confirma que é seguro**

`gitGuardChildEnv` adiciona `PATH=<emptyDir>` ao final do slice de env sem filtrar `Path=`
(case-insensitive Windows). Verificado em `$(go env GOROOT)/src/os/exec/exec.go:1244`:
`dedupEnvCase(caseInsensitive=true)` processa o slice em reverso e mantém a **última**
ocorrência de cada chave. Como `PATH=<emptyDir>` é adicionado ao final, ele vence `Path=`.
Não é defeito — é correto por construção.

---

## Vereditos por Prescrição

| # | Prescrição | Veredito | Evidência |
|---|---|---|---|
| 1 | Ordem do TestMain: guard antes de MkdirTemp | **Atendida** | commands:90 guard, commands:121 MkdirTemp; comentário explícito. Gap: ordering não tem sentinela (D1) |
| 2 | Preservar GOCACHE/GOMODCACHE | **Atendida** (commands) | commands:103-151 captura+re-injeção; mutation 7 rc≠0 sem ela. Validator não precisa: sem `go build` em testes |
| 3 | Sentinela com canal de produto (ClaudeConfigDir) | **Atendida** | ML-1A action 3 (roadmap linha 93): "Nenhum outro pacote: a Wave 0 mediu que só `commands` vaza." Prescrição 3 scoped a `commands`. `commands/testenv_isolation_test.go:63` atende. Validator sentinel pré-existente (em `main` antes desta REQ), sem ClaudeConfigDir — gap D3 é pré-existente. |
| 4 | PATH no gate do ML-1A documentado | **Atendida** | Roadmap ML-1A linha 106: `env PATH="$(dirname "$(command -v go)"):/usr/bin:/bin" go test ./internal/commands/ -count=1`. Gate documentado e marcado `[x]` na linha de AC5. |
| 5 | `git --version` em `check-prereqs` | **Atendida** | Makefile linha 287: `git --version >/dev/null 2>&1` |
| 6 | Sem bypass via variável de ambiente | **Atendida** | Nenhuma variável SKIP_PREREQS ou similar no Makefile |
| 7 | `git --version` não LookPath nos 4 pacotes | **Atendida** | Todos os 4 TestMain usam `exec.Command("git","--version").Run()`. M4 (LookPath) reprova em `validator` e `commands` |
| 8 | AC3 exige rc≠0 e `FAIL\t<pkg>` | **Atendida** | Roadmap ML-2A AC3: "rc≠0, a linha de diagnóstico no stderr, `FAIL` + nome do pacote na saída, nenhum `PASS`/`ok` e zero `--- FAIL`". AC medido e marcado `[x]` |
| 9 | Sentinela por pacote | **Atendida** | `TestGitGuardFailsFastWhenGitAbsent` nos 4 pacotes (auditsurface:59, pathguard:53, validator:123, commands:85) |
| 10 | Arm POSIX com fake git 127 | **Atendida** | Todos os 4 pacotes têm `git_falso_127` subtest. M4 medido em `validator` e `commands`; M5 medido nos mesmos dois |

---

## Defeitos por Severidade

### D1 — Ordenação do guard não tem loop-breaker (LOW)

**Evidência:** Mutação 6 (guard após MkdirTemp) → sentinela PASS, 2 diretórios temporários
vazados em `$TMPDIR`. O sentinela verifica saída (rc=2, diagnóstico, sem PASS) mas não
verifica ausência de artefatos vazados.

**Correção mínima:** redirecionar o `TMPDIR`/`TMP`/`TEMP` do processo filho para um
`t.TempDir()` isolado antes de lançá-lo, depois afirmar que o diretório está vazio após
o filho sair.

`filepath.Glob(os.TempDir() + "/trackfw-commands-test-home-*")` não funciona por
construção: `netexec_test.go:121` chama `MkdirTemp("", "trackfw-commands-test-home-*")`
antes de `m.Run()`, portanto o dir do TestMain pai está vivo em `$TMPDIR` durante toda
a execução de `m.Run()` — inclusive quando `TestGitGuardFailsFastWhenGitAbsent` roda.
O Glob encontraria ≥ 1 match em código correto e falharia com falso positivo a cada execução.

Prova em ambas as direções (rt-mutant):

- M6 (guard após MkdirTemp) + TMPDIR-redirect no sentinela:
  ```
  testenv_isolation_test.go:106: guard after MkdirTemp leak: [trackfw-commands-test-home-2724063704] dirs in childTmp
  --- FAIL: TestGitGuardFailsFastWhenGitAbsent/path_vazio
  ```
- Ordem correta + TMPDIR-redirect:
  ```
  --- PASS: TestGitGuardFailsFastWhenGitAbsent/path_vazio
  ```

Exemplo de verificação completa para implementador:
```go
childTmp := t.TempDir()
env := gitGuardChildEnv(emptyDir, []string{"TRACKFW_TEST_"})
env = append(env, "TMPDIR="+childTmp, "TMP="+childTmp, "TEMP="+childTmp)
cmd.Env = env
// ... executa o filho ...
entries, _ := os.ReadDir(childTmp)
if len(entries) > 0 {
    names := make([]string, len(entries))
    for i, e := range entries { names[i] = e.Name() }
    t.Errorf("guard after MkdirTemp leak: %v dirs in childTmp", names)
}
```

A implementação real tem a ordem correta. Recomenda-se como ML adicional nesta REQ
(Regra Dura: mesma causa, mesmo PR) — consulte o arquiteto.

### D2 — GOCACHE re-injeção sem sentinela (LOW)

**Evidência:** Mutação 7 (re-injeção removida) + `GOPROXY=off` → product FAILs em
`TestUnknownCommand_NeverExecutesExternalBinary`. Sem diagnóstico de ambiente incompleto.

**Diferença em relação a D1/D3/D5:** D2 não tem corrective mínima, porque o failure mode
é visível (product FAILs, não silêncio). D1/D3/D5 têm sentinelas ausentes que deixam um
problema real passar verde — esse é o defeito que esta REQ existe para fechar.
D2 é documentado para futuros refactors do TestMain.

### D3 — Validator sentinel sem check (c) (LOW, pré-existente)

**Evidência:** `internal/validator/main_test.go:205-229` tem HOME+USERPROFILE mas não
`agentconfig.ClaudeConfigDir`. A versão de `commands` tem o terceiro check. Confirmado via
`git diff main -- internal/validator/main_test.go`: `TestIsolatedHomeCoversBothChannels` não
mudou nesta branch — o gap existia em `main` antes desta REQ.

**Correção mínima:** adicionar a `TestIsolatedHomeCoversBothChannels` em
`internal/validator/main_test.go` (ou em arquivo separado):
```go
dir, err := agentconfig.ClaudeConfigDir(agentconfig.DefaultResolver)
if err != nil {
    t.Fatalf("agentconfig.ClaudeConfigDir error: %v", err)
}
if !strings.HasPrefix(dir, isolatedTestHome+string(filepath.Separator)) {
    t.Errorf("agentconfig.ClaudeConfigDir = %q, want rooted under %q", dir, isolatedTestHome)
}
```

Pré-existente em `main`; não introduzido por esta REQ. Recomenda-se como ML adicional
nesta REQ (Regra Dura: mesma causa — gap de sentinela em pacote tocado por esta REQ).

### D4 — Bypass de cache via wrapper git (LOW)

**Evidência:** Wrapper git em PATH que passa normalmente mas retorna 127 com `BREAK_GIT=1`
permite re-executar um pacote cacheado sem invalidar o cache (PATH não muda → chave idêntica
→ resultado cacheado servido). Medido em `internal/pathguard/`:

```
PATH=<gitwrap>:$PATH  go test ./internal/pathguard/       → ok
PATH=<gitwrap>:$PATH  go test ./internal/pathguard/       → (cached) ok
BREAK_GIT=1  PATH=<gitwrap>:$PATH  go test ./internal/pathguard/  → (cached) ok  rc=0
```

**Limite:** requer pre-colocação deliberada do wrapper e controle de variável de ambiente.
`make quality` não é afetado: `check-prereqs` executa `git --version` frescamente. O
bypass limita-se a `make test` e `go test` diretos. `-count=1` fecha o bypass completamente.

**Correção mínima:** nenhuma obrigatória. Observação operacional: usar `-count=1` em
runs de CI suspeitos ou quando o ambiente mudou.

### D5 — bash versão não verificada em check-prereqs (LOW, AC4)

**Evidência:** `mapfile` e `declare -A` (bash 4.0+) usados em 5 scripts de `parity-rest`;
shebangs verificados como `#!/usr/bin/env bash` (resolvem do PATH). `check-prereqs`
verifica apenas presença (`bash --version >/dev/null`); bash 3.2 (macOS stock) passa.
Medido diretamente: `check-orphan-gates.sh:331` emite `declare: -A: invalid option` em
bash 3.2 desta máquina. Consequência em `make quality`: inferida (parity-rest falha);
`make quality` não foi executado diretamente nesta sessão. CI não inspecionado.

Vetor (c) alcançado: `check-prereqs` retorna 0 com bash 3.2, mas `parity-rest` falharia.

**Correção mínima:** adicionar ao `check-prereqs`:
```bash
bash -c '[[ ${BASH_VERSINFO[0]} -ge 4 ]]' >/dev/null 2>&1 \
  || { printf 'ERRO: ambiente incompleto — bash 4.0+ necessário\n'; _fail=1; }
```

Mitigação parcial: CI (linux/apt/nix) e Homebrew bash incluem bash 5.x. Stock macOS sem
Homebrew bash apresenta o defeito. Recomenda-se como ML adicional nesta REQ
(Regra Dura: mesma causa — gap de verificação de pré-requisito).

---

## Residual

1. Fake tool que sai 0 (git, go, python3) passa `check-prereqs` e o TestMain. Aceito —
   qualquer verificação de versão tem este limite.
2. `make test` contorna `check-prereqs`. Por design — `quality` é o gate.
3. Sentinela de ordenação (D1) sem loop-breaker de artefato. LOW. Correção: TMPDIR redirect
   no sentinela (demonstrado em mutant). ML adicional nesta REQ.
4. P2 sem sentinela dedicado (D2). LOW — failure mode é product FAILs, não silêncio.
5. Validator sentinel sem check (c) (D3, pré-existente). LOW — gap em `main` antes desta
   REQ; não introduzido aqui. ML adicional nesta REQ.
6. Cache bypass via wrapper git (D4). LOW — afeta `make test`/`go test` diretos; `make
   quality` fechado via `check-prereqs`. `-count=1` fecha o bypass completamente.
   Hipótese natural: shim xcrun do macOS (não medido).
7. bash versão não verificada (D5). LOW — bash 3.2 passa `check-prereqs`, parity-rest
   falha com syntax error. Realista em stock macOS sem Homebrew bash. ML adicional nesta REQ.
8. Pacotes sem TestMain na closure (`guard`, `serve`, `sync`, `agentconfig`, `integrations`,
   `config`, `discover`): passam com HOME sintético hoje porque nenhum teste percorre o
   caminho que chama `homedir.Dir()`. Um teste novo sem `t.Setenv` vaza silenciosamente.
   Aceito como risco de revisão de código (herdado da Wave 0).
