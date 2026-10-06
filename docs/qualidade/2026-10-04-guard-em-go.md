# Revisão de Qualidade — Guard de Hook em Go (ML-4B)

> Agente: hefesto-tf | Data: 2026-10-06 | Branch: `feat/hooks-de-guard-executam-no-windows`

---

## Escopo

`git diff origin/main...HEAD` — foco em:

- `internal/guard/` (novo pacote: `gitbranch.go`, `credential.go`, `payload.go`)
- `internal/commands/guard.go`
- `internal/generators/agentfiles.go` (constantes de hook line, legados, migração)
- `internal/generators/update.go`
- `internal/validator/validator_guard_binary_probe_ml2b.go` e mudanças em `validator_credential_guard.go` / `validator_git_branch_guard.go`
- Testes novos: `guard_parity_helper_test.go`, `guard_thin_wrapper_test.go`, `validator_guard_binary_probe_ml2b_test.go`, `validator_guard_binary_probe_ml2c_test.go`

---

## Cobertura medida

Comando: `go test ./internal/guard/... -coverprofile=... -count=1 && go tool cover -func=...`

**Total `internal/guard`: 81.0%**

Funções de decisão relevantes:

| Função | Arquivo | Cobertura |
|---|---|---|
| `applyRule` | `gitbranch.go:286` | 91.7% |
| `MatchSubcommand` | `gitbranch.go:192` | 81.8% |
| `RunGitBranch` | `gitbranch.go:65` | 66.0% |
| `findSubcommand` | `gitbranch.go:262` | **50.0%** |
| `stripHeredocBodies` | `gitbranch.go:468` | **55.6%** |
| `parseHeredocDelim` | `gitbranch.go:503` | **20.0%** |
| `RunCredential` | `credential.go:61` | 96.6% |
| `RunCredentialGlobal` | `credential.go:132` | 90.0% |
| `credReadMode` | `credential.go:392` | **0.0%** |
| `ExtractCommand` | `payload.go:148` | 81.5% |
| `DrainStdin` | `payload.go:42` | 82.9% |

`go vet ./internal/guard/...` — limpo, sem saída.
`go vet ./internal/validator/...` — limpo, sem saída.

---

## Achados por severidade

### MÉDIO — M1: `findSubcommand` a 50% (segurança)

**Arquivo:** `internal/guard/gitbranch.go:262`

`findSubcommand` é a função que extrai o subcomando do `git` após pular flags globais (`-C`, `-c`, `--work-tree`, `--git-dir`, `--namespace`). A 50%, o ramo não coberto é o fallback `tokens = tokens[1:]` na linha ~269 — ativado quando um desses flags aparece como último token (sem argumento seguinte). Se esse ramo estivesse errado, `git -C <EOF> push` poderia escapar ao guard.

**Corretivo proposto (a entregar ao implementador Go):**

Arquivo: `internal/guard/gitbranch_test.go`
Adicionar subteste em `TestMatchSubcommand` (ou equivalente):
```go
// flag como último token — deve cair no fallback tokens[1:] e não encontrar subcomando
{"git -C push", ""},   // -C sem valor → não encontra subcomando → allow
```
Comando de validação: `go test ./internal/guard/... -run TestMatchSubcommand -v`

---

### MÉDIO — M2: `parseHeredocDelim` a 20% e `stripHeredocBodies` a 55.6% (segurança)

**Arquivo:** `internal/guard/gitbranch.go:468` e `:503`

Heredoc stripping é segurança: sem ela, `git push <<'EOF'\ngit push\nEOF` passaria o comando pelo body e poderia bloquear indevidamente. Mais crítico: se `parseHeredocDelim` retornasse falso-negativo para `<<-` ou delimitadores com aspas, o heredoc nunca seria reconhecido e o corpo nunca seria removido.

Ramos não cobertos confirmados:
- `stripHeredocBodies`: o caminho "heredoc não fechado" (linhas 494-496) — retorna o original (lado seguro), mas não está testado.
- `parseHeredocDelim`: `<<-` (o `-` de indented heredoc), e delimitadores com aspas (`<<'EOF'`, `<<"EOF"`).

**Corretivo proposto (implementador Go, `internal/guard/gitbranch_test.go`):**

```go
// <<- (heredoc com indentação) — deve detectar o delimitador
{"git push <<-EOF\n\tgit push\nEOF", ""}  // body removido → allow

// delimitador com aspas simples
{"git push <<'EOF'\ngit push\nEOF", ""}   // body removido → allow

// heredoc nunca fechado — retorna original (safe-side)
{"git push <<EOF\ngit push", "push"}      // nunca fechado → original contém "push" → block
```

Comando de validação: `go test ./internal/guard/... -run TestStrip -v`

---

### BAIXO — B1: `credReadMode` com 0% de cobertura — código morto

**Arquivo:** `internal/guard/credential.go:392-394`

```go
func credReadMode(yamlPath string) string {
    return credReadModeWithDefault(yamlPath, "warn")
}
```

A função existe, tem doc comment ("project scope wrapper"), mas nunca é chamada: `RunCredential` em linha 106 chama `credReadModeWithDefault` diretamente. Nenhum teste a cobre. Não é bug de segurança (retornaria o valor correto se chamada), mas cria superfície de API enganosa.

**Corretivo proposto (implementador Go):**

Opção A: remover `credReadMode` de `credential.go` (a chamada direta em `RunCredential` já é clara).
Opção B: fazer `RunCredential` chamar `credReadMode(...)` em vez de `credReadModeWithDefault(..., "warn")` — eliminando a repetição do default "warn" na call-site.

---

### BAIXO — B2: nome de arquivo de produção com sufixo de ML

**Arquivo:** `internal/validator/validator_guard_binary_probe_ml2b.go`

O sufixo `-ml2b` no nome do arquivo de produção encoda o roadmap ML que o criou. Isso polui a leitura do pacote (o próximo leitor precisa saber que ML-2B é o "binary probe"), e a já existente `validator_guard_binary_probe_ml2c_test.go` para o arquivo de teste de ML-2C cria inconsistência. O ML-2B também mudou a função `guardExpectedLine` que é usada por outros arquivos do pacote.

**Corretivo proposto (implementador Go / arquiteto):**

Renomear `validator_guard_binary_probe_ml2b.go` → `validator_guard_binary_probe.go`. O histórico do ML vive no `git log`. A decisão é do arquiteto, não do implementador sozinho (requer `git mv` na branch ativa).

---

### INFORMACIONAL — I1: dois sítios para a string de linha de hook

**Arquivos:** `internal/generators/agentfiles.go:448-454` (constantes) e `internal/validator/validator_guard_binary_probe_ml2b.go:47-52` (função `guardExpectedLine`)

O gerador define as strings via constantes:
```go
guardGitBranchCmdPSPOSIX = "trackfw guard git-branch; exit $LASTEXITCODE"
guardGitBranchCmdCmdExe  = "trackfw guard git-branch"
```

O validator as constrói via `guardExpectedLine(subcmdName, fam)`:
```go
func guardExpectedLine(subcmdName string, fam guardShellFamily) string {
    if fam == guardShellFamilyCmdExe { return "trackfw guard " + subcmdName }
    return "trackfw guard " + subcmdName + "; exit $LASTEXITCODE"
}
```

Os valores concordam hoje. Os testes externos (`TestCredentialGuardScriptReference_MatchesGenerator`, `TestGitBranchGuardScriptReference_MatchesGenerator`) pinnam as referências do validator contra o gerador, mas **não existe teste que afirme diretamente `guardExpectedLine("git-branch", PSPosix) == guardGitBranchCmdPSPOSIX`**. Um drift futuro (ex.: adicionar `&&` em vez de `;` no gerador) passaria nos testes de scripts mas quebraria silenciosamente a validação.

**Proposta** (não bloqueante): adicionar um teste em `validator_guard_binary_probe_ml2b_test.go` ou em um arquivo externo que imports ambos os pacotes:
```go
// Afirma que guardExpectedLine e as constantes do gerador concordam.
got := guardExpectedLine("git-branch", guardShellFamilyPSPosix)
want := generators.GuardGitBranchCmdPSPOSIX // precisa exportar a constante
```
Se exportar a constante for indesejável, o teste pode reconstruir o valor esperado com a mesma lógica e comparar.

---

## Duplicação: intencional vs. dívida

### Intencional (fixtures congeladas)

`internal/generators/testdata/guard-sh-reference/` contém cópias congeladas dos scripts originais completos do ML-1C. O `TestGuardShReferenceFixtures_Sha256` pina o sha256 dessas fixtures. Essa duplicação é **intencional e documentada**: após o ML-2A os scripts ao vivo viraram invólucros finos, mas as fixtures do braço bash da paridade ficaram no estado original. A comparação continua significativa porque compara o binário Go contra o comportamento do script completo, não contra um invólucro.

### Intencional (referências do validator)

As constantes `credentialGuardScriptReference`, `credentialGuardGlobalScriptReference` e `gitBranchGuardScriptReference` em `internal/validator/validator_*_reference.go` são cópias do gerador. A duplicação existe porque `internal/validator` não pode importar `internal/generators` (import cycle). Os testes externos (pacote `validator_test`) importam os dois e verificam byte-equality. **Intencional, documentado e coberto por testes.**

### Dívida

- `credReadMode` duplica a semântica de `credReadModeWithDefault(..., "warn")` sem ser usada — ver B1 acima.
- Nenhuma outra dívida de duplicação identificada no escopo.

---

## Falsificação de três testes novos

### T1 — `TestGitBranchGuardWrapper_NoTrackfwInPath_ExitsTwo` (`guard_thin_wrapper_test.go:109`)

**Afirma:** o invólucro fino `scripts/trackfw-git-branch-guard.sh` falha fechado (rc=2, "not found in PATH" em stderr) quando `trackfw` não está no PATH.

**Por que não é vacuoso:** o invólucro contém:
```bash
if ! command -v trackfw >/dev/null 2>&1; then
  echo "trackfw-git-branch-guard: trackfw not found in PATH …" >&2; exit 2; fi
```
Se esse bloco fosse removido e o script apenas chamasse `exec trackfw guard git-branch`, `bash` retornaria rc=127 com "command not found" no stderr — o teste falharia em duas asserções (`rc != 2` e `stderr não contém "not found in PATH"`).

### T2 — `TestValidateTrackfwBinaryInProjectRoot_DiretorioNomeadoExe_Ok` (`validator_guard_binary_probe_ml2b_test.go:532`)

**Afirma:** um diretório chamado `trackfw.exe` NÃO gera violation na regra `trackfw_binary_in_project_root`.

**Por que não é vacuoso:** a implementação em `validateTrackfwBinaryInProjectRoot` tem:
```go
if e.IsDir() { continue }
```
Removendo essa guarda, `os.ReadDir` retornaria a entrada do diretório `trackfw.exe` e a comparação `strings.ToLower(e.Name()) == "trackfw.exe"` passaria — gerando 1 violation. O teste falharia na asserção `len(msgs) != 0`.

### T3 — `TestGitBranchGuardWrapper_WithRealBinary_BlocksGitPush` (`guard_thin_wrapper_test.go:151`)

**Afirma:** o invólucro fino com o binário real no PATH bloqueia `git push` (rc=2).

**Por que não é vacuoso:** o teste usa `makeDirWithTrackfwYAML` para criar um diretório com `trackfw.yaml` e injeta o binário compilado pelo `TestMain` no PATH via `injectGuardBinaryPath`. Se o binário não implementasse o bloqueio de `git push` (ex.: `RunGitBranch` sempre retornasse 0), o invólucro retornaria rc=0 e o teste falharia. Se o diretório não tivesse `trackfw.yaml`, o guard retornaria rc=0 (no-op fora de projeto) e o teste também falharia.

---

## Veredito

**LIBERA O PR.**

Nenhum dos achados é bug de correção nos caminhos de decisão primários. A arquitetura é sólida: `applyRule` a 91.7% cobre as regras de bloqueio; `RunCredential`/`RunCredentialGlobal` acima de 90%; o único caminho de decisão com gap preocupante é `findSubcommand` a 50% (M1).

**Rastreamento obrigatório pós-merge:**

| ID | Severidade | O que fazer |
|---|---|---|
| M1 | Médio | Adicionar testes para `findSubcommand` com flag como último token |
| M2 | Médio | Adicionar testes para `parseHeredocDelim` (<<-, aspas) e `stripHeredocBodies` (não fechado) |
| B1 | Baixo | Remover `credReadMode` ou fazer `RunCredential` chamá-la |
| B2 | Baixo | Renomear `validator_guard_binary_probe_ml2b.go` → `validator_guard_binary_probe.go` |
| I1 | Info | Adicionar teste de concordância `guardExpectedLine` ↔ constantes do gerador |
