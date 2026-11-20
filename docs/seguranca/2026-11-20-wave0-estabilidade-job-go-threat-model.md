# Wave 0 — Threat Model: Estabilidade do job go

> Roadmap: `ROADMAP-2026-11-20-estabilidade-do-job-go-teto-de-timeout-por-pacote-e-etxtbsy-no-script-falso-dos-testes-de-guard.md`
> REQ: `REQ-2026-11-20-estabilidade-do-job-go-teto-de-timeout-por-pacote-e-etxtbsy-no-script-falso-dos-testes-de-guard.md`
> Autor: hades-tf | Data: 2026-11-20

---

## 1. Enumeration Completeness

### 1.1 Flags `-timeout` de `go test` — lista fechada

**Comandos executados:**
```
$ grep -rn -- '-timeout' .github/workflows/ scripts/ Makefile | wc -l
51
```
Das 51 linhas: 33 são contexto de `chunk-timeout`/`timeout-minutes` (inclui `Makefile:219`
com nome de script `check-falsify-chunk-timeout.sh`) ou `check-ci-workflow-pin-parity.sh`;
6 são linhas em `scripts/testdata/` (5 `go test -timeout` em snapshots de roadmap congelados
+ 1 ocorrência da palavra "timeout" no nome de um arquivo em `census-roadmaps.txt`); as
12 restantes são `go test -timeout` ativos (10 locais distintos). Os 6 hits de `testdata`
são corpus congelado exempto por AC2 — ML-1A não os altera. Grep de qualificação:
```
$ grep -rn -- '-timeout' .github/workflows/ scripts/ Makefile \
  | grep -v 'chunk-timeout\|timeout-minutes\|check-ci-workflow\|run-gates' \
  | grep -v testdata \
  | sort
```

**Saída (12 linhas — 10 locais distintos):**
```
Makefile:11:    TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 2m ./...
quality.yml:40:      - run: go test -timeout 2m ./...
quality.yml:361:          go test -v -p 1 -parallel 1 -timeout 2m ./... *> $goLoadOut
quality.yml:383:          go test -v -p 1 -parallel 1 -timeout 2m ./... *> $goAssertOut
quality.yml:516:          go test -v -p 1 -parallel 1 -timeout 20m ./... *> $outFile
quality.yml:1250:            -run "TestUpdateNeverWritesThrough" -v -count=1 -timeout 60s ...
quality.yml:1299:            -run "TestUpdateNeverWritesThrough" -v -count=1 -timeout 60s ...
release.yml:43:        - run: go test -timeout 2m ./...
scripts/check-serve-api-file-security.sh:117:    -run "TestFileHandler_SymlinkEscape" -count=1 -timeout 30s 2>&1 || true)
scripts/check-serve-api-file-security.sh:128:    -run "TestFileHandler_SymlinkEscape" -count=1 -timeout 30s 2>&1 || true)
scripts/check-serve-api-file-security.sh:142: go test ./internal/serve/... -run "TestFileHandler_SymlinkEscape" -count=1 -timeout 30s ...
scripts/check-serve-api-file-security.sh:154: go test ./internal/serve/... -run "TestFileHandler_SymlinkInsideRoot" -count=1 -timeout 30s ...
```
Nota: quality.yml:1248-1251 e 1297-1300 são linhas de continuação de um único
`go test ./internal/generators/... -run "TestUpdateNeverWritesThrough"` (linhas anteriores
no YAML). check-serve:117/128/142/154 são quatro linhas de dois `go test` distintos
(SymlinkEscape e SymlinkInsideRoot), com repetição do `-timeout 30s` nas linhas de
captura (`OUT=$(...)`) e nas invocações diretas.

**Total de `go test -timeout` distintos (incluindo continuações):** 10 locais (12 linhas).

Nota: `check-ci-workflow-pin-parity.sh` e `run-gates-falsify-parallel.sh` têm `-timeout` em
contexto de `timeout-minutes` de nível de job e `chunk-timeout` de script — não são
`go test -timeout` e foram excluídos pelo filtro.

**ENTRA no escopo de ML-1A** (os três sítios com `./...` sem paralelismo serializado):

| Sítio | Linha | Valor atual | Papel |
|-------|-------|-------------|-------|
| `Makefile` | 11 | `-timeout 2m ./...` | Ponto único de fix |
| `quality.yml` | 40 | `-timeout 2m ./...` | Job `go`, `ubuntu-latest` |
| `release.yml` | 43 | `-timeout 2m ./...` | Passo de teste |

**FICA FORA** (motivo explícito para cada um):

| Sítio | Linha | Valor | Motivo |
|-------|-------|-------|--------|
| `quality.yml` | 361 | `-timeout 2m ./...` | Bloco PowerShell; módulo Go sintético; `-p 1 -parallel 1`; REQ proíbe alterar |
| `quality.yml` | 383 | `-timeout 2m ./...` | Mesmo bloco, segunda execução |
| `quality.yml` | 516 | `-timeout 20m ./...` | Job `windows-full-suites`; REQ proíbe alterar |
| `quality.yml` | 1248-1251, 1297-1300 | `-timeout 60s` | Continuação de `go test ./internal/generators/... -run TestUpdateNeverWritesThrough`; teste único alvo, não `./...` |
| `scripts/check-serve-api-file-security.sh` | 117, 128 (continuações), 142, 154 | `-timeout 30s` | Dois testes únicos (`SymlinkEscape`, `SymlinkInsideRoot`); chamados por `make parity`, não `make test` |

**A lista é fechada?** Sim. O grep não-filtrado retornou 51 linhas; após excluir contexto
de `chunk-timeout`/`timeout-minutes`/`check-ci-workflow`/`run-gates` e `Makefile:219`
(nome de script), restam 12 linhas = 10 locais distintos. Os 3 sítios ENTRA são todos os
casos com `./...` sem paralelismo serializado. Os 7 locais FORA (quality.yml:361, 383, 516,
1250, 1299 + check-serve SymlinkEscape + check-serve SymlinkInsideRoot) têm motivo de
exclusão explícito derivado da REQ.

---

### 1.2 Sítios ETXTBSY: `_test.go` que gravam executável — lista fechada

#### Mecanismo (golang/go#22315)

A `syscall.ForkLock` é um `sync.RWMutex` com convenção documentada em
`$(go env GOROOT)/src/syscall/exec_unix.go` (linhas 19–65):
> "The rule for which fd-creating operations use ForkLock is as follows:
> ...the create new fd/mark close-on-exec operation is done holding ForkLock for reading,
> and the fork itself is done holding ForkLock for writing."

**Verificação empírica:**
```
$ grep -n 'ForkLock' $(go env GOROOT)/src/os/file_unix.go
(nenhuma saída)
```

`os.WriteFile` chama `openFileNolog` que passa `O_CLOEXEC` ao kernel, mas **não adquire
`ForkLock`**. Esta ausência é a causa do race. O `O_CLOEXEC` faz o kernel fechar o fd no
filho quando `execve()` é chamado — mas essa fechagem ocorre *na transição* `execve()`, não
imediatamente após `fork()`. Durante a janela `fork→exec` o filho ainda possui o fd de
escrita aberto.

**Por que o race existe:**
1. Goroutine A chama `os.WriteFile(stub)`: abre fd X (escrita, sem ForkLock).
2. Goroutine B chama `exec.Command(shellB)` → `forkExec()`. `forkExec` adquire
   `ForkLock.Lock()` (escrita), faz `fork()`. O filho C-B herda fd X.
3. C-B está na janela `fork→exec` (antes de `execve()` fechar fd X via O_CLOEXEC).
4. A shell de A tenta `execve(stub)` via PATH lookup. O kernel recusa: **ETXTBSY** — o
   arquivo stub tem um fd de escrita aberto (de C-B, que herdou fd X de B).

**Por que o fix com ForkLock.RLock() funciona:**
Envolver todo o `WriteFile` com `syscall.ForkLock.RLock()` impede que qualquer `forkExec`
(que adquire `Lock()`) ocorra enquanto o fd estiver aberto. Múltiplos `RLock()` coexistem,
mas `Lock()` exclui todos os `RLock()` ativos. Resultado: enquanto goroutine A mantém
`RLock()`, nenhum fork pode ocorrer, logo nenhum filho herda fd X.

`syscall.ForkLock.Lock()` (escrita exclusiva) **também é correto** e mais restritivo:
exclui tanto forks quanto outros fd creators. Válido desde que `defer Unlock()` seja usado.

**Regra obrigatória em ambas as variantes:** usar `defer syscall.ForkLock.RUnlock()`
(ou `defer Unlock()`) imediatamente após o `RLock`/`Lock`. Se `t.Fatalf` for chamado na zona
travada sem defer, `runtime.Goexit()` não executa código posterior — apenas deferred —
e o lock nunca é liberado (ver T6 para o sintoma resultante).

**Discriminante de risco:**
- **Arquivo é execve target** — PATH lookup (`exec.Command("shell", "-c", "trackfw")`)
  ou caminho direto (`exec.Command(binaryPath)`): o kernel pode retornar ETXTBSY se outro
  processo mantiver fd de escrita aberto durante a janela fork→exec.
- **Arquivo é lido como script** — `exec.Command("bash", scriptPath)`: bash é o alvo do
  `execve`, não o script. Bash lê o script via `open()` (não `execve()`). SEGURO.
- **Concorrência** — a race requer que a goroutine escritora rode concorrentemente com outra
  goroutine que chame `exec.Command` no mesmo binário de pacote. Na fase paralela, toda goroutine
  paralela do pacote pode ser agendada concorrentemente — a condição é satisfeita quando o helper
  de escrita é chamado de qualquer goroutine em região `t.Parallel()` no mesmo pacote. Um helper
  chamado somente por funções de teste sequenciais (0 `t.Parallel()` nem no caller nem em
  nenhum `t.Run` ancestral) **não** satisfaz essa condição.

#### Enumeração completa dos sítios

**Grep 1 — `WriteFile` com permissões executáveis (incluindo variantes):**
```
$ grep -rn \
    'WriteFile.*0o\?755\|WriteFile.*0o\?700\|WriteFile.*0o\?775\|WriteFile.*0o\?711\|WriteFile.*ModePerm' \
    internal/ cmd/ --include='*_test.go' | sort
```
Retornou **48 linhas**. Padrões `0o?775|711|ModePerm` retornaram **zero linhas** adicionais.

**Census de permissões exec em test files (além de 0755/0700):**
```
$ grep -rhE -A3 '(WriteFile|OpenFile|Chmod)\(' internal/ cmd/ --include='*_test.go' \
  | grep -oE '0o?[0-7]{3,4}|ModePerm' | grep -vE '0o?644|0o?600|0o?444|0000' | sort | uniq -c | sort -rn
  70 0755    # já coberto por Grep 1
  26 0o755   # já coberto por Grep 1
   9 0o700   # já coberto por Grep 1
   4 0o500   # os.Chmod(directory, 0o500) — bit exec = busca em diretório
   3 0o666   # em comentários de identity_windows_test.go sobre NTFS Mode()
   3 0111    # em string literals (ver grep -rnE -A3 ... | grep '0111': 1 hit/arquivo)
   2 0700    # já coberto por Grep 1
   2 0500    # em t.Fatalf format strings ("chmod 0500 em %s, %v") — não perm args reais
   2 02064   # dígitos de CI run ID em contexto -A3 (38902064559), não perms
   1 0o200   # netexec_test.go:179: info.Mode()|0o200 (adiciona write bit), sem exec
   1 02610   # dígitos de data em contexto -A3 (working_context_log_test.go:108:
              #   "20261022T200000Z-symlink-entry.md"), não perm
```
`0111` não é perm arg: ocorre em string literals nos 3 arquivos `execbit_probe_test.go` —
confirmado pelo grep
`grep -rnE -A3 '(WriteFile|OpenFile|Chmod)\(' internal/ cmd/ --include='*_test.go' | grep -E '0111'`
(1 hit por arquivo, em string de mensagem). Não passado a `WriteFile`/`Chmod`.
`0o666` (0b110110110), `0o200` (0b010000000) e `02064` (0b010000110100): nenhum tem bit exec
(`& 0111 == 0` para os três — aritmética basta). `0500`, `02610` confirmados como t.Fatalf
format strings ou dígitos de data no contexto -A3.
`0o500` (4 hits reais) são todos `os.Chmod(directory, 0o500)` — bit exec de diretório
significa busca (`chdir`), nunca `execve`. Ver inspeção manual abaixo.

**Chmod +x via shell:**
```
$ grep -rn '"chmod"\|chmod +x' internal/ cmd/ --include='*_test.go'
(nenhuma saída)
```
Nenhum `chmod +x` via shell em test files.

**Escopo das greps (somente `_test.go`):**
```
$ grep -rlE 'WriteFile|OpenFile|Chmod' internal/ cmd/ --include='*.go' \
  | grep -v '_test\.go$' | wc -l
25

$ grep -rlE 'WriteFile|OpenFile|Chmod' internal/ cmd/ --include='*.go' \
  | grep -v '_test\.go$' | xargs grep -l '"testing"' 2>/dev/null
(nenhuma saída)
```
Arquivos não-test com WriteFile/OpenFile/Chmod: 25. Nenhum importa `"testing"` — o escopo está
fechado: apenas `_test.go` são relevantes para a janela ETXTBSY.

**Grep 2 — `OpenFile` com 0755 e `os.Chmod`:**
```
$ grep -rn 'OpenFile.*O_CREATE.*0o\?755\|os\.Chmod.*0o\?755\|os\.Chmod.*0o\?700' \
    internal/ cmd/ --include='*_test.go'
```
Retornou **24 linhas**. Nota: `grep -v '^\s*//'` é no-op no formato `path:line:conteúdo` (a
linha começa com `internal/`, não com `//`); o filtro foi removido. O output inclui linhas de
comentário em cabeçalhos de arquivo (`execbit_probe_test.go:9/100`,
`validator_credential_guard_test.go:57/957`, `validator_git_branch_guard_test.go:72`).

Sítios executáveis reais encontrados (além dos já no Grep 1):
- `generators/git_branch_guard_test.go:975` — `OpenFile(dst, O_WRONLY|O_CREATE|O_TRUNC, 0755)` (`copyExecutableFile`)
- `commands/ship_test.go:2085` — `OpenFile(dst, O_WRONLY|O_CREATE|O_TRUNC, 0o755)` (`placeExecutableInPath`)
- `generators/execbit_probe_test.go:60` / `commands/execbit_probe_test.go:60` / `discover/execbit_probe_test.go:60` — `os.Chmod(p, 0o755)` em arquivo vazio temporário; probe de filesystem; sem `exec.Command`
- `generators/scaffold_doctor_test.go:229` — `os.Chmod(p, 0o700)` narrowing sobre arquivo 0755 já gravado; sem `exec.Command`
- **Chmod de diretórios** (não são execve targets): `commands/doctor_test.go:648` (`hiddenDir`, restore para 0755 em Cleanup), `commands/json_exit_test.go:130` (`trackfwDir`), `integrations/manager_persistence_order_test.go:157,163,239,245,296` (cleanup/restore 0o700), `thirdparty/provenance_test.go:170,241` (cleanup/restore 0o700)

**Inspeção manual adicional** (perms não capturadas pelo Grep 2 — `0o500` e perm 0):
- `commands/doctor_test.go:643` — `os.Chmod(hiddenDir, 0)` (perm 0, sem exec); diretório
- `integrations/manager_persistence_order_test.go:154,236` — `os.Chmod(directory, 0o500)`
  onde `directory = filepath.Dir(filename)` (diretório pai de arquivo pré-existente; NÃO
  criado por MkdirAll neste teste); perm 0o500 — r-x para o dono, TEM bit exec; SEGURO
  porque diretório — x = busca, nunca `execve`
- `integrations/manager_persistence_order_test.go:300` — `os.Chmod(probe, 0o500)` onde
  `probe` é criado com `os.MkdirAll` (linha 292); diretório — x = busca, nunca `execve`
- `thirdparty/provenance_test.go:245` — `os.Chmod(probe, 0o500)` (perm 0o500 — r-x para o dono,
  TEM bit exec; SEGURO porque `probe` é criado com `os.MkdirAll` na linha 236: diretório —
  x = busca, nunca `execve`)
Todos são diretórios → SEGURO (fora do escopo ETXTBSY).

**Grep 3 — permissão em linha de continuação (blind spot de multiline):**
```
$ grep -rn -A3 -E 'WriteFile\(|OpenFile\(' internal/ cmd/ --include='*_test.go' \
  | grep -E '0o?7[0-7]{2}|ModePerm' \
  | grep -vE 'WriteFile\(|OpenFile\(|MkdirAll\('
```
Resultado: **1 linha** — `scaffold_doctor_test.go:228: // Narrow the mode to 0700 —
simulates umask 077.` — comentário explicativo do `os.Chmod` da linha 229, já capturado
no Grep 2. Grep 3 usa janela `-A3`; chamadas cujo perm aparece a mais de 3 linhas (ex.:
`validator_adr_req_has_content:473` offset +22, `validator_guard_hook_concordance:994` offset +7)
não são capturadas por Grep 3. O sweep de perm-identificador (janela 40 linhas) fecha esse gap:
todos os hits de multiline confirmados sem bit exec.

**Variável de permissão:**
`generators/scaffold_doctor_test.go:67` — `os.WriteFile(p, []byte(tc.content), tc.mode)` +
`os.Chmod(p, tc.mode)`, seguidos por `checkValidateScriptArtifact` (função Go, sem
`exec.Command`). Independente do valor de `tc.mode`, não é execve target. SEGURO.
**Grep de perm-identificador** — `WriteFile`/`OpenFile` onde o perm não é literal na mesma linha:
```
$ grep -rnE 'os\.(WriteFile|OpenFile)\(' internal/ cmd/ --include='*_test.go' \
  | grep -vE ',\s*(0o?[0-7]{3,4}|(os|fs)\.ModePerm)\)'
```
Retornou **59 linhas**. Contra-braço: `scaffold_doctor_test.go:64` (`tc.mode`) deve aparecer — aparece.
Classificação das 59 linhas (sweep com janela de 40 linhas e sufixo `,)` para evitar
falsos positivos de datas):
- **2 linhas são comentários** sem chamada real: `scaffold_literal_copy_parity_test.go:21`
  (`// CWD-relative path (os.MkdirAll("scripts")/os.WriteFile(...)`) e
  `roadmap_ml_gate_test.go:191` (`// a non-nil error.  Before ML-3A the call was _ = os.WriteFile`).
- **1 hit verdadeiro de perm-identificador**: `scaffold_doctor_test.go:64` com `tc.mode`;
  tabela confirma `mode: 0o755` em todos os 4 casos (linhas 34, 40, 46, 52) — já em Grep 1.
- **56 hits de perm em linha de continuação**: tally do sweep (`awk -F' -> ' '{print $2}' | grep -oE '0o?[0-7]{3,4}|NONE' | sort | uniq -c`) retornou: 0644×39, 0o644×14, 0600×3, 0755×1, NONE×2 = 59 total. O hit de 0755 (offset +25 em `roadmap_ml_gate:191`) pertence a `os.MkdirAll(wip, 0755)` na janela — pertence a uma chamada diferente, não ao WriteFile comentado. `validator_guard_binary_probe:252/283`, `update_validate_guard:164`, `validator_guard_hook_concordance:994` e `validator_adr_req:473` confirmados como 0644/0600 (os dois últimos a offset 7 e 22, respectivamente, pertencentes à mesma chamada multi-linha). Nenhum exec nos 56.

**`guard_parity_helper_test.go`:** grep por WriteFile/Chmod/0755/0700 retornou **zero linhas**.

**Duas perguntas por sítio:**
1. O arquivo gravado é **execve target**? (PATH lookup ou caminho direto — risco real)
2. Pode haver **fork concorrente** no mesmo binário de teste enquanto o fd está aberto?
   (exige que o helper seja chamado de goroutine em região `t.Parallel()` que pode ser
   agendada concorrentemente com outra goroutine do mesmo binário de pacote que chame
   `exec.Command` — na fase paralela, toda goroutine paralela do pacote satisfaz essa
   condição, não apenas subtestes do mesmo nível)

**Tabela — 48 linhas do Grep 1 + sítios do Grep 2 (incluindo ship_test.go:2085 e diretórios):**

| Arquivo | Linha(s) | Arquivo gravado | Execve target? | Fork concorrente possível? | Classificação |
|---------|----------|-----------------|----------------|---------------------------|---------------|
| `generators/guard_line_shell_test.go` | **371** | `dir/trackfw` (0755) | **SIM** — `runGuardLine`: `exec.Command(shellBin, "-c", line)` com PATH injetado contendo fakeDir | **SIM** — helper chamado de subtestes paralelos (42 `t.Parallel()` em guard_line_shell_test.go) cujos irmãos chamam `runGuardLine` → `exec.Command(shellBin, ...)` | **VETOR ETXTBSY CONFIRMADO** |
| `generators/guard_thin_wrapper_test.go` | 86 | `dir/trackfw` (0755) | SIM — PATH lookup | NÃO — 0 `t.Parallel()` no arquivo; callers (L133, L226) em funções sequenciais confirmados por grep | SEGURO; defesa em profundidade se ML-1B centralizar |
| `generators/attention_hooks_test.go` | 230 | `dir/trackfw` (0755) | SIM — PATH lookup com `PATH=binDir` | NÃO — `TestAttentionShellCommandsAndFailSoftWrappers` (enclosing func confirmada por awk) não chama `t.Parallel()`; 0 `t.Parallel()` no arquivo confirmado | SEGURO; defesa em profundidade se ML-1B centralizar |
| `generators/fail_closed_d11_behavior_test.go` | 70, 81 | `dir/trackfw` (0755) | SIM — shell executa trackfw via PATH | NÃO — 0 `t.Parallel()` no arquivo; callers em funções sequenciais confirmados por grep | SEGURO; defesa em profundidade se ML-1B centralizar |
| `generators/scaffold_doctor_test.go` | 225 | `scripts/trackfw-credential-guard.sh` (0755) | NÃO — `checkScaffoldArtifact` é função Go; sem `exec.Command` sobre o arquivo | n/a | SEGURO |
| `generators/scaffold_doctor_test.go` | 229 (Grep 2) | mesmo arquivo (Chmod 0o700) | NÃO — narrowing de permissão; mesmo contexto Go | n/a | SEGURO |
| `generators/credential_guard_test.go` | 391 | script credential guard | NÃO — `exec.Command("bash", scriptPath)`: bash lê script | NÃO — 1 `t.Parallel()` em L837 (`TestSyntheticTokenFragments_NoneMatchCredentials`), função diferente; callers de `copyFrozenCredentialGuardFixture` (L203, L352, L890) estão em funções sequenciais | SEGURO |
| `generators/git_branch_guard_test.go` | 160 | script branch guard | NÃO — bash lê script | NÃO — 0 `t.Parallel()` neste arquivo | SEGURO |
| `generators/git_branch_guard_test.go` | 975 (Grep 2) | cópia de `git` (OpenFile+io.Copy, 0755) | SIM — mas **Windows only** (POSIX usa `os.Symlink`); `windows-full-suites` usa `-p 1 -parallel 1` | NÃO — sequencial; job Windows serializado | SEGURO (Windows only) |
| `generators/git_branch_guard_test.go` | 1170 | script branch guard antigo | NÃO — bash lê script | NÃO — sequencial | SEGURO |
| `generators/update_test.go` | 642, 670, 759, 787 | scripts bash/guard | NÃO — Go inspeciona conteúdo; nenhum `exec.Command` sobre os arquivos | NÃO — 0 `t.Parallel()` neste arquivo | SEGURO |
| `generators/update_test.go` | 2342 | `scripts/trackfw-attention-signal.sh` (0o755) — fixture de estado antigo | NÃO — `Update(root)` é chamada Go; sem `exec.Command` sobre o arquivo | NÃO — 0 `t.Parallel()` neste arquivo | SEGURO |
| `generators/release_tag_stub_shim_env_windows_test.go` | 75 | script `eco` bash | NÃO — `exec.Command("bash.exe", script)`: bash lê script | NÃO — 0 `t.Parallel()` | SEGURO |
| `generators/execbit_probe_test.go` | 60 (Grep 2) | arquivo vazio temporário (Chmod 0755) | NÃO — filesystem probe; sem exec | n/a | SEGURO |
| `opencodeplugin/opencodeplugin_jsruntime_test.go` | 145 | cópia do próprio binário de teste como `trackfw` (0755) | SIM — PATH lookup: `nodeEnv` prefixa `fakeTrackfwDir` ao PATH, node.js chama trackfw | NÃO — 0 `t.Parallel()` em todo o arquivo; único caller `writeFakeTrackfw` em linha 303 é sequencial | SEGURO |
| `commands/ship_test.go` | 2085 (Grep 2) | cópia de `git` (OpenFile+io.Copy, 0o755) | SIM — `git` é execve target via PATH | NÃO — sequencial; Windows only (POSIX usa symlink) | SEGURO |
| **Chmod diretórios** | `doctor_test.go:643,648`, `json_exit_test.go:130`, `integrations/...`, `thirdparty/...` | diretórios existentes | NÃO — diretórios não são execve targets; Chmod 0 ou 0755 para controle de acesso ou cleanup | n/a | SEGURO (fora de escopo) |
| `commands/barrier_reentry_test.go` | 239 | `reenter.sh` (0755) | NÃO — `sh scriptPath`: bash lê script | NÃO — 0 `t.Parallel()` | SEGURO |
| `commands/doctor_windows_sh_missing_test.go` | 33, 76 | stubs `sh`/`bash` (0755) | NÃO — `runDoctor(false)`: Go verifica existência; sem subprocess | NÃO — 0 `t.Parallel()` | SEGURO |
| `commands/netexec_test.go` | 716 | pre-push git hook (0755) | SIM — git executa o hook via `execve` como processo filho do teste | NÃO — 0 `t.Parallel()` | SEGURO |
| `commands/root_test.go` | 218 | `trackfw-vaildate` (typo intencional, 0755) | NÃO — teste confirma que binário **não** é exec'd | NÃO — 0 `t.Parallel()` | SEGURO |
| `commands/testenv_isolation_test.go` | 122 | fake `git` (0755) | SIM — PATH lookup indireto | NÃO — 0 `t.Parallel()` | SEGURO |
| `commands/execbit_probe_test.go` | 60 (Grep 2) | arquivo vazio temporário (Chmod 0755) | NÃO — filesystem probe | n/a | SEGURO |
| `auditsurface/main_test.go` | 77 | fake `git` (0755) | SIM — PATH lookup | NÃO — 0 `t.Parallel()` | SEGURO |
| `pathguard/main_test.go` | 71 | fake `git` (0755) | SIM — PATH lookup | NÃO — 0 `t.Parallel()` | SEGURO |
| `discover/execbit_probe_test.go` | 60 (Grep 2) | arquivo vazio temporário (Chmod 0755) | NÃO — filesystem probe | n/a | SEGURO |
| `validator/main_test.go` | 180 | fake `git` (0755) | SIM — PATH lookup | NÃO — 0 `t.Parallel()` neste arquivo; pacote `validator` tem t.Parallel em outros arquivos mas subtestes paralelos rodam após sequenciais no mesmo top-level function | SEGURO |
| `validator/validator_guard_binary_probe_test.go` | 214 | script guard (0755) | NÃO — `validateGitBranchGuardHookResolvableLegacyWarnings()`: função Go; sem subprocess | NÃO — 0 `t.Parallel()` | SEGURO |
| `validator/validator_guard_binary_probe_windsurf_amazonq_test.go` | 149, 255 | script guard (0755) | NÃO — mesma função Go, sem subprocess | NÃO — 0 `t.Parallel()` | SEGURO |
| `validator/validator_guard_binary_probe_gitbash_test.go` | 341 | fake `bash` (0700) | SIM — caminho direto `exec.Command(stub, ...)` | NÃO — 0 `t.Parallel()` neste arquivo; pacote `validator` tem t.Parallel em `validator_orientation_helpers_test.go` e `validator_credential_guard_integrity_test.go`, mas subtestes paralelos rodam somente depois que os sequenciais terminaram — fd já fechado | SEGURO |
| `validator/validator_credential_guard_test.go` | 164, 316, 395, 565, 611, 688, 942 | scripts credential guard (0755) | NÃO — `exec.Command("bash", scriptPath)`: bash lê script | NÃO — 0 `t.Parallel()` neste arquivo | SEGURO |
| `validator/validator_git_branch_guard_test.go` | 134, 160, 391, 467, 504, 548, 576, 612, 645, 740, 846 | scripts branch guard (0755) | NÃO — `exec.Command("bash", scriptPath)`: bash lê script | NÃO — 0 `t.Parallel()` neste arquivo | SEGURO |
| `validator/validator_windows_sh_missing_rule_test.go` | 42 | stub shell (0755) | NÃO — apenas inspeção Go de filesystem | NÃO — 0 `t.Parallel()` | SEGURO |

**Contra-braço para sítio S1** — única `WriteFile(..., 0755)` em `guard_line_shell_test.go`:
```
$ grep -n 'WriteFile' internal/generators/guard_line_shell_test.go
344: os.WriteFile(..., 0600)   # preamble fixture — não executável
351: os.WriteFile(..., 0600)   # preamble fixture — não executável
371: os.WriteFile(..., 0755)   # ← único 0755: sítio ETXTBSY confirmado
471: os.WriteFile(..., 0644)   # trackfw.cmd Windows — 0644
2361,2463,3138,3255: WriteFile(..., 0644) # perfis PowerShell — não executáveis
```

**Precisão sobre paralelismo dos call sites de `makeFakeTrackfw*`:** nem todos os ~36 call
sites estão em subtestes paralelos. `TestGuardLineShellMatrix_DefeitoAntigo` (~L1001) e
`TestGuardLineShellMatrix_HContraArm` (~L2621) não chamam `t.Parallel()` — call sites em
~L1050, L1156, L1302, L2661, L2756 são sequenciais. A proteção via ForkLock é aplicada no
helper único e cobre todos os sítios uniformemente.

**`guard_cmdexe_exec_test.go`** — 4 `t.Parallel()` no arquivo. Cada função de teste de
nível superior contém `if runtime.GOOS != "windows" { t.Skip(...) }` no início (L190, L221,
L253, L278): em Linux a função retorna via Skip antes de entrar no loop de subtestes que chama
`t.Parallel()`. O `exec.Command` usa `cmdExe` (`cmd.exe`), não os helpers `makeFakeTrackfw*`.
Sem risco de ETXTBSY em Linux.

**`agent_conventions_test.go`** — 2 `t.Parallel()`, nenhum `WriteFile` com bits exec;
sem risco.

**A lista é fechada para escrita feita em `_test.go`?** Sim. O Grep 1 (WriteFile com bits exec)
retornou 48 linhas; o Grep 2 adicionou ship_test.go:2085, git_branch_guard:975, 3 probes de
filesystem, scaffold_doctor:229 e Chmod de diretórios; inspeção manual adicionou doctor:643
(perm 0), integrations:154,236,300 e provenance:245 (todos 0o500 em diretórios — x = busca,
não execve); Grep 3 retornou 1 linha (comentário, executável já em Grep 2);
guard_parity_helper_test.go confirmado sem escrita de executável; sweep de perm-identificador
(59 linhas, 2 comentários + 1 identifier [tc.mode=0o755, já em Grep 1] + 56 multiline sem exec)
fecha o gap de janela -A3 do Grep 3; os 7 OpenFile de test (confirmados via
`grep -rnE 'os\.OpenFile\(' ... --include='*_test.go'`) têm perm 0644×5 ou já estão no
Grep 2 (ship:2085, git_branch_guard:975). Todos os sítios classificados na tabela. Único vetor
com risco CONFIRMADO por evidência de CI: S1. Sítios S2–S4 são SEGUROS (nenhum `t.Parallel()`
nos callers nem em nenhum `t.Run` ancestral — confirmado por grep); defesa em profundidade se
ML-1B centralizar. Todos os demais são SEGUROS com evidência explícita.

**ML-1B DEVE** corrigir `guard_line_shell_test.go:371` (`makeFakeTrackfw`) — único sítio
com risco CONFIRMADO por evidência de CI.

**ML-1B PODE** centralizar S2–S4 (`guard_thin_wrapper_test.go:86`,
`attention_hooks_test.go:230`, `fail_closed_d11_behavior_test.go:70,81`) como defesa em
profundidade; esses sítios são SEGUROS hoje (0 `t.Parallel()` nos callers confirmado). Se um
caller paralelo for adicionado sem a proteção, o race voltará (R2).

Código-de-produto (`scaffold.go`, `update.go`) grava scripts guard (0755) mas é invocado de
testes sequenciais via bash. Risco atual BAIXO; declarado como residual R1.

---

## 2. Threat Model

Adversário: o implementador apressado e o arquiteto otimista — quem esvazia um AC sem quebrar
uma regra escrita.

### T1 — Teto alto que esconde regressão real (AC1)

Implementador sobe `internal/generators` para 10m em vez de 5m. AC1 diz "5m" explicitamente
— 10m viola AC1 diretamente.

**Gate:** verificar que o teto gravado no Makefile é exatamente `-timeout 5m`.

---

### T2 — `make test` testa menos pacotes que `./...` (AC1)

Implementador exclui um pacote acidentalmente (typo em filtro) ou inclui `internal/generators`
em ambos os buckets.

**Medição preventiva:**
```
$ go list ./internal/generators/...
github.com/kgsaran/trackfw/internal/generators
```
O pacote `internal/generators` não tem subpacotes. Logo `./internal/generators/` não deixa
nenhum subpacote cair em nenhum bucket.

**Gate de AC1 (ML-1A deve verificar — não medido nesta Wave 0):**

ML-1A deve observar o que `make test` realmente passa, não recalcular por grep de texto:
```bash
# contar linhas ok/FAIL/? na saída de make test:
make test 2>&1 | grep -cE '^(ok|FAIL|\?)[[:space:]]'
# deve ser igual a:
go list ./... | wc -l  # = 31
```

---

### T3 — Proteção ETXTBSY via remoção de paralelismo (AC3)

Implementador remove `t.Parallel()` de todos os subtestes problemáticos. ETXTBSY some;
`-count=N` passa. AC3 diz "nenhum teste removido, pulado ou desparalelizado".

**Gate de ML-2A:**
```bash
git diff $(git merge-base HEAD origin/main) -- '*_test.go' | grep -c '^\-.*t\.Parallel'
# deve ser 0
```

---

### T4 — `t.Skip` condicional mascara falha (AC3)

Implementador adiciona `t.Skip("flaky")` em subtestes que falham com ETXTBSY.

**Gate:** contagem de SKIPs em `go test -v` antes e depois do fix deve ser igual.

---

### T5 — `release.yml` continua com `go test` hardcoded (AC2)

Implementador atualiza `quality.yml:40` para `make test` mas esquece `release.yml:43`.

**Gate de AC2:**
```bash
grep -n 'go test -timeout 2m \./\.\.\.' \
    .github/workflows/quality.yml \
    .github/workflows/release.yml
# deve retornar vazio após o fix
```
Nota: o grep não interpreta YAML; match em comentário YAML (`#`) seria falso positivo. O
revisor deve confirmar que qualquer match remanescente está em linha comentada.

---

### T6 — ForkLock deadlock por `t.Fatalf` dentro da zona travada (AC3, risco de impl)

```go
// ERRADO — sem defer:
syscall.ForkLock.RLock()
if err := os.WriteFile(stub, ...); err != nil {
    t.Fatalf(...)  // runtime.Goexit() sem defer RUnlock → lock jamais liberado
}
syscall.ForkLock.RUnlock()
```

Sintoma: todos os `exec.Command` subsequentes no binário travam indefinidamente; pacote
atinge o teto (5m) → `panic: test timed out` — **mesmo sintoma da causa original**, mas
no "depois" do fix, sem mensagem `Text file busy`.

**Gate de ML-2A:**
1. O helper usa obrigatoriamente `defer syscall.ForkLock.RUnlock()` (ou `defer Unlock()`)
   imediatamente após o `RLock`/`Lock`. Com `defer`, `t.Fatalf` dentro da zona é correto
   (Goexit executa deferred calls). Sem `defer`, qualquer retorno antecipado vaza o lock.
2. Nada dentro da zona travada pode adquirir ForkLock de forma recursiva. Especificamente:
   nenhum `exec.Command` (que chama `forkExec`, que adquire ForkLock.Lock() no mesmo
   goroutine → deadlock). A zona cobre SOMENTE o `os.WriteFile`.

---

### T7 — Variante Lock vs RLock (AC3, risco de classificação)

Ambas as variantes são corretas para prevenir ETXTBSY:
- `RLock()`: exclui `Lock()` (fork); múltiplos `RLock()` coexistem → paralelismo máximo
- `Lock()`: exclui tanto forks quanto outros fd creators → serializa mais mas é válido

**RLock é a variante preferida** por dois motivos:
1. É exatamente o "fd creator" pattern documentado em exec_unix.go (linhas 55-65).
2. `acquireForkLock` em `forkpipe2.go:52-73` tem um starvation escape: se ForkLock está
   travado para escrita e há readers esperando, o fork temporariamente adquire `RLock` para
   dar passagem — confirmando que o runtime explicitamente acomoda holders de `RLock` durante
   a janela de fork. Um `Lock()` externo criaria serialização extra não prevista nesse modelo.

O roadmap de ML-1B usa `Lock()` como ponto de partida; `RLock()` também é aceitável. Nenhuma
variante deve ser rejeitada por ser "errada" — a presença de `defer` é o requisito crítico.

**Gate de ML-2A:** verificar presença de `defer RUnlock()` ou `defer Unlock()`. Ambas as
variantes são aceitáveis.

---

### T8 — Test-level retry loop mascarando ETXTBSY (AC3)

**Esta ameaça** (nomeada explicitamente no roadmap como "retry que mascara falha real"):
implementador adiciona loop de retry no helper ou em `runGuardLine` que silencia `Text file
busy`. O ETXTBSY some nos logs; `-count=N` passa; `t.Parallel()` está presente.

```go
// ERRADO — loop de retry mascarando ETXTBSY:
for i := 0; i < 5; i++ {
    err := exec.Command(stub).Run()
    if err == nil { break }
    // silencia "text file busy" sem reportar
}
```

**Gate de ML-2A (quatro partes):**

a) Nenhuma linha não-comentário adicionada menciona ETXTBSY ou "text file busy" como
branch que silencia, pula ou retenta. Cada match deve ser uma assertion que FALHA o teste:
```bash
git diff $(git merge-base HEAD origin/main) -- '*_test.go' \
  | grep '^+' | grep -v '^\+\+\+' | grep -v '^\+\s*//' \
  | grep -iE 'ETXTBSY|text file busy'
# revisar cada match: deve ser t.Fatalf / t.Error — nunca continue/break/skip/sleep
```

b) Nenhum hunk toca `runGuardLine`, `runInteractive` ou `buildShellPath` (o fix pertence
ao lado da escrita, não ao lado do exec):
```bash
git diff $(git merge-base HEAD origin/main) -- '*_test.go' \
  | grep '^@@\|^+' | grep -E 'runGuardLine|runInteractive|buildShellPath'
# deve retornar vazio; qualquer alteração a essas funções exige justificativa explícita
```

c) Nenhum `time.Sleep` adicionado. `t.Skip` é gatilho de revisão — permitido somente como
guarda de plataforma no início de um teste novo (`if runtime.GOOS != "linux" { t.Skip(...) }`);
proibido adicionado a teste existente ou a subtest:
```bash
git diff $(git merge-base HEAD origin/main) -- '*_test.go' \
  | grep '^+' | grep -v '^\+\+\+' | grep -v '^\+\s*//' \
  | grep -E 'time\.Sleep|t\.Skip'
# time.Sleep deve retornar vazio; t.Skip: revisar cada match — aceito só como
# platform guard (if runtime.GOOS ...) no topo de função nova, nunca em teste existente
```

d) O helper que adquire ForkLock não contém `for`, `exec.Command` nem `time.Sleep`
dentro da zona travada (qualquer deles = recursão ou retenção prolongada do lock):
```bash
# inspeção manual: a zona entre RLock/Lock e RUnlock/Unlock deve conter
# somente os.WriteFile + defer + retorno de erro
```

---

### T9 — Bash retry interno mascara "antes" da medição (AC3, medição)

**Hipótese** (não verificada nesta Wave 0, medição em ML-1B): bash implementa loop de retry
para ETXTBSY. Se a medição de "antes" usar dash como shellBin, dash sai imediatamente (sem
retry) e exit code não é -1. O exit -1 observado nas runs de CI pode ser de bash que ficou
em loop de retry por > 10s. Medição de "antes" deve usar o shellBin que a CI usa.

**Gate:** o "antes" da medição ML-1B deve mostrar `Text file busy` ou exit -1 em pelo menos
1 de N tentativas com `-count=N -parallel P` (P ≥ 4).

---

### T10 — Mutex de pacote que preserva t.Parallel mas serializa execução (AC3)

Implementador usa `var writeMu sync.Mutex` e envolve write + exec no mesmo lock. ETXTBSY
some; `t.Parallel()` presente; mas subtestes correm um por vez.

**Gate de ML-2A:** o lock deve cobrir SOMENTE `os.WriteFile`, não o `exec.Command`
subsequente. O tempo de parede de `TestGuardLineShellMatrix` antes/depois deve ser comparável.

---

### T11 — Makefile com env var em apenas um dos dois `go test` (AC1)

ML-1A deve garantir que as duas invocações de `go test` satisfaçam:
1. Ambas carregam `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1`
2. Falha de qualquer uma reprova o alvo `test`
3. Os dois conjuntos de pacotes formam partição de `go list ./...`

Padrão errado (env var apenas no primeiro com `&&`):
```
TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 5m ./internal/generators/ && \
go test -timeout 2m ...          # ← sem env var
```

Padrão errado (separador `;` não aborta em falha):
```
TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 5m ./internal/generators/ ; \
TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 2m ...
# ← falha do primeiro não aborta
```

ML-1A deve satisfazer as propriedades acima independentemente da sintaxe escolhida.

**Gate adicional:** `make test` com um teste que falha em `internal/generators` deve reprovar
o alvo inteiro (rc ≠ 0).

---

## 3. Falsification Targets — duas direções para AC1–AC3 e AC4

### AC1 — Teto por pacote (duas direções)

**Falso negativo (AC1 passa, regressão passa despercebida):**
Teste em `internal/generators` que toma 4m58s. Passa o teto de 5m. Monitoramento contínuo
fora do escopo desta REQ (residual R5).

**Falso positivo (AC1 reprova quando deveria passar):**
Build tags fazem `go list ./...` retornar contagem diferente em macOS vs Linux. Mitigação:
medir no mesmo runner (`ubuntu-latest`). Contra-braço: `go list ./... | wc -l` = 31.

**Gates de falsificação do ML-1A:**
1. `go list ./... | wc -l` antes e depois = 31 no runner Linux
2. Soma dos dois conjuntos = 31; pacote `internal/generators` aparece em exatamente um dos dois alvos
3. Teste que dorme 150s num pacote não-generators reprova em 2m; mesmo sono em generators passa
4. `make test` com falha em generators falha inteiro (ver T11)

---

### AC2 — Workflows chamam `make test` (duas direções)

**Falso negativo (sítio antigo persiste):**
```bash
grep -n 'go test -timeout 2m \./\.\.\.' \
    .github/workflows/quality.yml \
    .github/workflows/release.yml
# deve retornar vazio após o fix
```

**Falso positivo (grep encontra string em comentário YAML):**
O grep não interpreta YAML. O revisor deve confirmar que qualquer match remanescente está em
linha YAML iniciada por `#` ou em bloco que nunca é executado.

---

### AC3 — Sem ETXTBSY sob paralelismo (duas direções)

**Falso negativo (ETXTBSY persiste após o fix):**

a) Fix em `makeFakeTrackfw` mas não em `makeFakeTrackfwForOS`: verificar que `ForOS` chama o
   helper corrigido (não reimplementa a escrita independentemente).

b) Janela residual no lock: o `RLock()` deve envolver a chamada `os.WriteFile` completa,
   incluindo o `open()` interno. Medição: "depois" com `-count=20 -parallel 8` não deve
   produzir nenhum ETXTBSY.

c) Sítios S2–S4 são sequenciais hoje (SEGURO). Se ML-1B centralizar: verificar que a escrita
   nesses sítios vai pelo helper corrigido e não permanece como implementação independente
   sem proteção. Se não forem centralizados, confirmar que continuam sem callers paralelos.

**Falso positivo (medição de "antes" não reproduz):**
Runner leve (CI sorteou máquina com 1 core efetivo). Mitigação: usar `-count=N` com N ≥ 10
e `-parallel P` com P ≥ 4.

---

### AC4 — Causa de exit -1 / stdin vazio medida e escrita

**Código relevante** (`runGuardLine`, `guard_line_shell_test.go:393–446`):
- `exec.CommandContext(ctx, shellBin, shellFlags..., "-c", line)` com deadline de 10s
- PATH injetado: `buildShellPath(shellBin, fakeDir)` — fakeDir contém o stub trackfw
- stdin: `strings.NewReader(stdinPayload)` — sempre alimentado
- Sem `WaitDelay`
- exit -1 = `exitErr.ExitCode()` após SIGKILL (contexto expirado a 10s)
- `stdin.out`: gravado pelo stub via `cat > "$(dirname "$0")/stdin.out"` — ausente se stub não executou

**Linhas 3077–3110** (`TestGuardLineShellMatrix_LContraArm_ConfirmLow`):
Essas linhas mostram `runInteractive(t, line, dir, fc.frc, nil, true)` — caminho PowerShell, não
`runGuardLine`. A falha com exit -1 registrada nas runs de CI é do caminho POSIX shell
(`runGuardLine`), não do PowerShell.

**Hipótese para exit -1 + stdin vazio** (não verificada nesta Wave 0, medição em ML-1B):
bash implementa loop de retry para ETXTBSY; se o loop durar > 10s → SIGKILL → exit -1.
Dash sai imediatamente em ETXTBSY (exit não-(-1)). Candidato alternativo: starvation de CPU
no runner (10s de timeout sem ETXTBSY). ML-1B deve discriminar os dois candidatos.

**Falsificação de AC4:**

Falso negativo: a causa escrita é "bash retry" (hipótese); ML-1B deve identificar shellBin
da run que produziu exit -1. `readlink /bin/sh` no ubuntu-latest → `/usr/bin/dash`; bash
disponível em `/usr/bin/bash`.

Falso positivo: após o fix ETXTBSY, o exit -1 some se a causa era bash retry; persiste se a
causa era starvation. ML-1B deve confirmar: "depois" com exit -1 sem `Text file busy` aponta
para causa independente.

---

## 4. Declared Residual

**R1 — Product-code writers fora de `_test.go`.**
`scaffold.go` (8 sítios, 0755) e `update.go` (2 sítios, 0755) gravam scripts guard. Testes
que exercem essas funções são sequenciais; os scripts são exec'd por `bash scriptPath`. Risco
atual BAIXO. ML-1B é escopo `_test.go` apenas. Enumeração desta Wave 0 cobre somente escritas
em `_test.go`. Não coberto: executável gravado por código de produto (`scaffold.go`, `update.go`,
etc.) chamado in-process a partir de um teste paralelo e depois exec'd — mesma race, Greps 1–3
e o sweep não enxergam. AC3 diz "helper de teste", então está fora do escopo desta REQ. Se o
job `go` continuar flaky após ML-1B, este é o próximo sítio a medir.

**R2 — Sítios S2–S4 (thin wrapper, attention_hooks, fail_closed).**
PATH lookup no pacote `generators`; callers atuais são todos sequenciais (0 `t.Parallel()`
nos arquivos que os chamam — confirmado por grep). Nenhum desses sítios chama
`makeFakeTrackfw`; cada um tem sua própria escrita independente. ML-1B DEVE corrigir S1.
ML-1B PODE centralizar S2–S4 num helper unificado como defesa em profundidade. Se qualquer
caller paralelo for adicionado a S2–S4 sem a proteção, o race voltará.

**R3 — Windows ETXTBSY equivalent.**
Sharing violation no Windows é mecanismo diferente; `windows-full-suites` serializa com `-p 1
-parallel 1`. Fora do escopo desta REQ.

**R4 — Grandchild pipe como causa independente de exit -1.**
Se a causa de exit -1 for grandchild mantendo pipe (não bash retry), o fix ETXTBSY não
resolve o hang. `runGuardLine` não tem `WaitDelay`. Medição em ML-1B.

**R5 — Regressão de desempenho abaixo do teto.**
O teto de 5m é 2,5× o cluster lento medido (119s). Drift entre 119s e 5m não é detectável
por teto.

**R6 — Hipótese de bash retry não verificada nesta Wave 0.**
AC4 exige "causa medida e escrita" — esta Wave 0 estabelece o quadro analítico; a medição
fica com ML-1B.

---

**Veredito de Wave 0:**
A lista de tetos é fechada (51 hits brutos → 12 linhas qualificadas = 10 locais distintos;
3 entram, 7 locais ficam fora com motivo). A lista de ETXTBSY é fechada para `_test.go` (48 linhas do Grep 1 + 24 linhas do Grep 2
incluindo ship_test:2085 e diretórios + inspeção manual de doctor:643, integrations:154,236,300
e provenance:245 (todos 0o500 em diretórios); Grep 3 retornou 1 comentário; sweep de
perm-identificador (59 linhas: 2 comentários + 1 identifier [0o755, em Grep 1] + 56 multiline
sem exec) fecha o gap de janela -A3 do Grep 3; os 7 OpenFile de test classificados; Não coberto:
escrita por código de produto em teste paralelo — declarado R1); guard_parity_helper confirmado
sem escrita executável; guard_cmdexe Windows-only; todos os sítios na tabela;
1 vetor confirmado — S1; S2–S4 SEGUROS por evidência de grep; restante SEGURO com evidência
explícita). As ameaças T1–T11 estão ordenadas por executabilidade. Os residuais
R1–R6 são declarados explicitamente. ML-1B DEVE corrigir S1; PODE centralizar S2–S4.

**Wave 1 pode começar**, sujeito à auditoria do arquiteto sobre o conteúdo desta análise.
