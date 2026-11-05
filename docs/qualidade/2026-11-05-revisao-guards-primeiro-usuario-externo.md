# Parecer de Qualidade: Guards do Primeiro Usuário Externo

> Hefesto (hefesto-tf) · 2026-11-05
> Branch: `fix/guards-do-primeiro-usuario-externo`, HEAD `f6509c91`
> Código revisado: diffs `1df288e2→8404de52` (ML-1A) e `8404de52→b73e13a3` (ML-1B)
> Roadmap: `ROADMAP-2026-11-05-guards-do-primeiro-usuario-externo-mode-block-ignorado-com-crlf-padroes-de-credencial-e-involucro-de-shell-no-guard-de-git.md`
> ML: 2B — paralelo ao red team do hades-tf

---

## 1. Metodologia

**Documentação de referência lida:**
- ADR `ADR-2026-11-05-guards-cumprem-o-que-o-readme-promete-...`
- Roadmap com auditoria do arquiteto nos MLs 1A e 1B
- Parecer Wave 0 §5.1 e §5.2
- REQ `REQ-2026-11-05-guards-do-primeiro-usuario-externo-...` (critérios AC1–AC5)

**Compilação do binário:**
```
go build -o .../scratchpad/hefesto/trackfw-g ./cmd/trackfw
# rc=0
```

**Projeto de medição:**
```
.../scratchpad/hefesto/proj/trackfw.yaml   (mínimo, credential_guard.mode: block)
```

**Regras de medição:**
- Todos os vetores git-guard: scripts `.sh` no scratchpad (não inline), para evitar interceptação pelo hook de sessão.
- Testes: apenas `env PATH="..." go test -count=1 ./internal/guard/`
- Contra-braço de profundidade: backup em scratchpad, restauração com `cp`, verificação com `cmp`. Declarado neste relatório.
- Scripts Python rodados com `python3 -I`.

**Git status confirmado antes do handoff:**
```
git status --porcelain    → ?? docs/qualidade/2026-11-05-revisao-guards-...
git diff --stat HEAD      → (vazio — nenhum arquivo de produto modificado)
```

---

## 2. Dois tokenizadores: posição e medição

### 2.1 Duplicação estrutural

Medida por `difflib.SequenceMatcher` (stripped lines):

| Par | Linhas orig | Linhas POSIX | Similaridade | Linhas iguais |
|---|---|---|---|---|
| `quoteAwareSplit` / `quoteAwareSplitPOSIX` | 141 | 134 | 78% | 107 |
| `shellTokenize` / `shellTokenizePOSIX` | 37 | 85 | 46% | 28 |
| `stripShellSuffix` / `stripShellSuffixPOSIX` | 108 | 106 | 86% | 92 |

### 2.2 Divergências comportamentais

**1. Backslash fora de aspas (divergência primária, dentro do escopo da mudança)**
- Original: `\git` → token literal `\git` (não reconhece como `git`)
- POSIX: `\git` → token `git` (§2.2.1); bloco L1615-1629 de `quoteAwareSplitPOSIX` não existe no original

**2. Backslash dentro de `"..."` (segunda divergência de backslash)**
- Original (`quoteAwareSplit`): decodifica inline — `\"→"`, `\\→\`
- POSIX (`quoteAwareSplitPOSIX`): emite `\"` verbatim → `shellTokenizePOSIX` faz o unescape conforme §2.2.3

**3. Design de dois estágios (divergência estrutural, não de backslash)**
- O pipeline POSIX usa arquitetura de dois estágios: `quoteAwareSplitPOSIX` coleta verbatim → `shellTokenizePOSIX` faz unescaping (incluindo `\$→$`, `` \`→` ``, `\"→"` dentro de `"..."`). O pipeline original faz tudo inline em `quoteAwareSplit`.

**4. Recursão de invólucro de shell (divergência estrutural, não de backslash)**
- Apenas o segundo passo (`matchSubcommandPOSIX`) chama `handleShellWrapper` para analisar `bash -c "git push"`. O primeiro passo (`MatchSubcommand`) não faz recursão em invólucros — detecta apenas git direto nos segmentos.

### 2.3 Posição sobre unificar vs. manter

**Manter separados.** Os dois pipelines servem gramáticas fundamentalmente diferentes: o original preserva caminhos Windows (`C:\Program Files\Git`) porque `filepath.Base` no Windows divide em `\`, portanto o token sem conversão é correto; o POSIX trata `\` como escape conforme §2.2.1. Mais ainda: a análise de invólucros de shell existe apenas no segundo passo por design — unificar em um único parser introduziria acoplamento entre o tratamento de git direto e o tratamento recursivo de wrappers.

A sobreposição de 78–86% nas funções `quoteAwareSplit` e `stripShellSuffix` justifica extração de helpers compartilhados por par. Isso é uma refatoração futura válida, não urgente.

---

## 3. Achados

### F1-A — BLOQUEANTE: `extractPowerShellCommandString` falha para flags com argumento antes de `-Command`

**Arquivo:** `internal/guard/gitbranch.go` L1987–2003

**Comportamento observado (via scripts no scratchpad):**
```
powershell -NoProfile -Command "git push"                           → rc=2 (correto)
powershell -ExecutionPolicy Bypass -Command "git push"              → rc=0 (INCORRETO)
powershell -ExecutionPolicy RemoteSigned -Command "git push"        → rc=0 (INCORRETO)
powershell -WorkingDirectory C:/foo -Command "git push"             → rc=0 (INCORRETO)
powershell -WindowStyle Hidden -Command "git push"                  → rc=0 (INCORRETO)
powershell -w hidden -Command "git push"                            → rc=0 (INCORRETO)
powershell -ep Bypass -Command "git push"                           → rc=0 (INCORRETO)
powershell.exe -ExecutionPolicy Bypass -Command "git push"          → rc=0 (INCORRETO)
```

**Causa raiz:** `extractPowerShellCommandString` (L1997–2000) percorre os tokens e, ao encontrar qualquer token sem prefixo `-`, executa `break` antes de encontrar `-Command`. Flags como `-ExecutionPolicy`, `-WorkingDirectory` e `-WindowStyle` consomem um argumento posicional (`Bypass`, `C:/foo`, `Hidden`) que não começa com `-`, causando o break prematuro.

A função lida corretamente apenas com flags que não consomem argumento (`-NoProfile`, `-NonInteractive`).

**Zero cobertura de teste** para qualquer vetor com flag-argumento antes de `-Command`. O contrato em `cli-parity.md` (L6762) promete reconhecimento de `powershell` com `-Command`; `-ExecutionPolicy Bypass` como prefixo quebra essa promessa silenciosamente.

**Nota:** `-ExecutionPolicy Bypass` é a forma padrão como agentes invocam PowerShell no Windows (target primário desta REQ). Esta é a lacuna de maior impacto operacional.

---

### F1-B — BLOQUEANTE: `credential_mode_test.go` não testa convergência guard→validator (AC1)

**Arquivo:** `internal/guard/credential_mode_test.go`

**AC1 da REQ diz:**
> "Um teste de tabela única afirma que o guard e o `extractCredentialGuardMode` do validator chegam ao mesmo `mode` para cada entrada"

**O que o teste faz:** `TestReadCredentialGuardMode_UnitVectors` (L135) chama exclusivamente `guard.ReadCredentialGuardMode(v.content)` e verifica o resultado esperado. O arquivo importa apenas `internal/guard` — nunca `internal/validator`.

**O que o teste NÃO faz:** não chama `validator.extractCredentialGuardMode` em nenhum ponto. O cabeçalho do arquivo (L2) diz "to allow importing internal/validator" mas a importação nunca acontece.

**Consequência:** se o `extractCredentialGuardMode` do validator fosse revertido para uma implementação independente (removendo o delegate `guard.ReadCredentialGuardMode`), todos os testes de `credential_mode_test.go` continuariam passando. A divergência seria indetectável.

A frase da Regra Dura: *"a test that does not assert any conclusion of the ML shouldn't exist"*. Este teste pode honestamente dizer que testa o guard — mas não pode dizer que afirma convergência guard/validator, porque o lado do validator nunca é chamado.

**Nota:** a implementação atual está correta por construção (`extractCredentialGuardMode` é um delegate puro). O problema é que nenhum teste impede um revert futuro. AC1 exige o teste explicitamente.

---

### F2 — CORRIGIR NO PR: `credJWTRe` e `credAWSRe` são dead code com comentário enganoso

**Arquivo:** `internal/guard/credential.go` L94–99 · Introduzido em ML-1A

Antes de ML-1A (`1df288e2`), `credJWTRe` e `credAWSRe` eram usados nas funções `credScanInput` e `credDeepScan`. O ML-1A substituiu a detecção pelos 12 padrões em `credPatterns`, mas não removeu as variáveis nem atualizou o comentário.

```go
// credJWTRe and credAWSRe are kept as package-level vars for the parts of the
// code (credScanFile, credDeepScan) that still reference them directly.
credJWTRe = credPatterns[0].re
credAWSRe  = credPatterns[1].re
```

Grep confirma: nenhum uso das variáveis além da declaração. `credScanFile` chama `credScanInput` (todos os 12 padrões); `credDeepScan` chama `credSecondLayer`. O comentário cria leitura falsa de que apenas JWT e AWS são detectados.

---

### F3 — CORRIGIR NO PR: Contagem de linhas do script em `credential.go` L2 está desatualizada

**Arquivo:** `internal/guard/credential.go` L2 · Introduzido em ML-1A

O arquivo diz `Faithful Go port of scripts/trackfw-credential-guard.sh (152 lines)`.

O script `scripts/trackfw-credential-guard.sh` tem atualmente 14 linhas (`wc -l` = 14). Era 152 linhas antes do ML-1A (confirmado em `git show 1df288e2`). O ML-1A transformou o Go na implementação completa e reduziu o script a um shim, mas não atualizou o comentário. O número 152 é agora o inverso da realidade.

---

### F4 — CORRIGIR NO PR: Comentário em `MatchSubcommand` L407–409 é ambíguo sobre o que D4 bloqueia

**Arquivo:** `internal/guard/gitbranch.go` L405–409 · Introduzido em ML-1B

```go
// Block if the POSIX pipeline also blocks (additive by construction: nothing
// allowed today becomes blocked on any OS because we deny only when the POSIX
// path independently agrees). D4, Wave 1 ML-1B (REQ-2026-11-05).
```

**Dois problemas:**

1. **"nothing allowed today becomes blocked"** é ambíguo: pode ser lido como "D4 não bloqueia nada que não fosse já bloqueado" — falso. `\git push` era permitido antes deste ML e agora é bloqueado. A intenção real é "o resultado não varia por sistema operacional", não "nenhuma coisa nova é bloqueada".

2. **"independently agrees"** é enfraquecido pelo fato de que ML-1B também modificou `stripEnvCommandPrefix` (confirmado por `git diff 8404de52 b73e13a3`), que é função compartilhada entre o primeiro e o segundo passo. O segundo passo não é completamente independente do primeiro: ambos chamam `stripEnvCommandPrefix`. O comentário exagera o grau de independência.

---

### F5 — CORRIGIR NO PR: Dois guards de profundidade usam operadores assimétricos sem explicação inline

**Arquivo:** `internal/guard/gitbranch.go`
- L2059: `if depth >= 2 { return "wrapper-not-analyzed" }` — em `handleShellWrapper`
- L2076: `if depth > 2 { return "wrapper-not-analyzed" }` — em `matchSubcommandPOSIX`

A assimetria (`>=` vs `>`) é intencional: `handleShellWrapper` é o teto efetivo (dispara antes de qualquer chamada recursiva); `matchSubcommandPOSIX` é belt-and-suspenders contra callers diretos com `depth > 2`, situação que não ocorre hoje mas pode ocorrer futuramente.

O segundo guard é logicamente alcançável apenas por um caller direto com `depth >= 3` (que hoje não existe). O segundo guard é redundante nos callers atuais mas não é dead code — ele protege futuros callers.

**Contra-braço executado (backup/restore com cmp):**
```
sha256(gitbranch.go antes) = 729cb81b5b38fd85288326e60544c4c1e02915dc68ea400de2b0a0e3c387cb44
Patch: depth >= 2 → depth >= 3  AND  depth > 2 → depth > 3
Test: TestMatchSubcommand_AC4_Depth3_WrapperNotAnalyzed → FAIL (esperado)
Restauração: cp backup → repo; cmp → "RESTORED: identical"
Test após restauração: PASS
```

Patching apenas `matchSubcommandPOSIX` resultaria em falso verde (`handleShellWrapper` dispararia primeiro para profundidade 3). Ambos os guards devem ser patchados para obter o contra-braço correto.

**Recomendação:** sem mudança de lógica. Acrescentar constante `maxWrapperDepth = 2` e comentário inline explicando a assimetria dos operadores.

---

### F7 — CORRIGIR NO PR: Quatro interpretadores prometidos e dois atributos prometidos sem vetor na tabela Blocked

**Arquivo:** `internal/guard/gitbranch_wrapper_test.go`

`cli-parity.md` L6762 promete explicitamente: `zsh`, `dash`, `ksh` com `-c`; `cmd` com `/k`; case-insensitive; sem `.exe`. Todos funcionam corretamente (rc=2, confirmado via script). Nenhum tem vetor em `TestMatchSubcommand_AC4_Wrapper_Blocked`:

```
zsh -c "git push"              → rc=2  (correto, sem vetor de teste)
dash -c "git push"             → rc=2  (correto, sem vetor de teste)
ksh -c "git push"              → rc=2  (correto, sem vetor de teste)
cmd /k "git push"              → rc=2  (correto, sem vetor de teste)
BASH -c "git push"             → rc=2  (correto, sem vetor de teste — case-insensitive)
GIT push                       → rc=2  (correto, sem vetor de teste — case-insensitive)
git.exe push                   → rc=2  (correto, sem vetor de teste — .exe stripped)
```

A ausência não indica defeito hoje, mas quebra a correspondência entre promessa documentada e evidência executável. Regressão nessas áreas não seria detectada pelo CI.

---

## 4. Observações (não urgentes)

### O4.1 — `sudo` documentado em cli-parity.md L6718 (git-add), não na seção D4 (git-branch)

`sudo` está implementado, testado (`sudo-bash-c-git-push` na tabela Blocked) e documentado em cli-parity.md L6718 no contexto do `git add`. A seção D4/wrapper (L6782) enumera os prefixos para o git-branch guard e não menciona `sudo`. É uma inconsistência de seção, não uma omissão total. Pre-existente a esta diff (nenhuma mudança no `sudo` neste diff).

### O4.2 — Teste de convergência em `credential_mode_test.go` tem cabeçalho desatualizado

**Arquivo:** `internal/guard/credential_mode_test.go` L2–3

O cabeçalho diz "to allow importing internal/validator without creating an import cycle" mas o arquivo nunca importa `internal/validator`. Isso está relacionado a F1-B: a intenção declarada no cabeçalho é o estado desejado, e atingir F1-B tornaria o cabeçalho correto.

### O4.3 — Vetor `"tab indented mode"` aceita desvio do spec Wave 0 sem nota no teste

**Arquivo:** `internal/guard/credential_mode_test.go` L89

O vetor espera `wantMode: "block"` para tab-indent. Wave 0 §5.1 especifica indentação com 2 espaços. A implementação aceita tabs (documentado no roadmap como desvio aceito), mas o teste não tem comentário mencionando isso.

### O4.4 — Stale comments em `gitbranch.go` referem ao bash original (pre-existente)

**Arquivo:** `internal/guard/gitbranch.go` L350, L377

L350: "faithful Go port of the bash match_subcommand() function"
L377: "mirrors bash while loop lines 430–452"

Presentes desde antes de `1df288e2`. O script `scripts/trackfw-git-branch-guard.sh` ainda existe, então a referência não é totalmente falsa, mas o Go é a fonte de verdade (v8+). Pre-existente, fora do escopo desta diff.

### O4.5 — Windows path: macOS não pode confirmar, Windows CI pode

**Arquivo:** `internal/guard/gitbranch.go` / `internal/guard/gitbranch_wrapper_test.go`

Medição macOS:
```
C:\Git\bin\git.exe push                              → rc=0  (macOS: filepath.Base não divide em \)
bash -c "\"C:\\Git\\bin\\git.exe\" push"             → rc=0  (macOS)
```

No Windows nativo, `filepath.Base('C:\Git\bin\git.exe')` → `git.exe` → stripped → `git` → seria bloqueado. `TestShellTokenizePOSIX_WindowsPathInsideDoubleQuote` (L330) confirma que o tokenizador POSIX preserva `C:\Program Files\Git\bin\git.exe` como token inteiro dentro de `"..."` (backslashes não-especiais preservados por §2.2.3). Se o `filepath.Base` Windows dividir corretamente, o guard bloqueará. Requer CI Windows para confirmar.

---

## 5. Métricas e suíte de testes

```bash
env PATH="..." go test -count=1 ./internal/guard/
# ok  github.com/kgsaran/trackfw/internal/guard  2.893s  rc=0
```

**Contract markers verificados (`cli-parity.md` L6752):** todos os 6 nomes de teste existem:
- `TestMatchSubcommand_AC4_Wrapper_Blocked` ✓
- `TestMatchSubcommand_AC4_Wrapper_Allowed` ✓
- `TestRunGitBranch_AC4_WrapperBlocked` ✓
- `TestRunGitBranch_AC4_WrapperAllowed` ✓
- `TestMatchSubcommand_AC4_Depth3_WrapperNotAnalyzed` ✓
- `TestMatchSubcommand_AC5_LanguageRuntime_Allowed` ✓

**Tabela Blocked:** 35 vetores (o número 47 é de verificação ad-hoc do arquiteto na auditoria ML-1B, não do tamanho da tabela de testes).

---

## 6. Veredito

**Dois achados bloqueantes:**

**F1-A** — `powershell -ExecutionPolicy Bypass -Command "git push"` → rc=0. Esta é a forma padrão de invocação de PowerShell por agentes no Windows (target primário desta REQ). Zero cobertura de teste.

**F1-B** — AC1 da REQ exige "um teste de tabela única afirma que o guard e o `extractCredentialGuardMode` do validator chegam ao mesmo `mode` para cada entrada". O teste atual (`credential_mode_test.go`) nunca chama o validator — importa apenas `internal/guard`. Se o delegate for revertido, nenhum teste falhará. AC1 não está mecanicamente coberto.

**F2–F5, F7 são corrigíveis no PR** sem bloquear o merge, mas devem entrar neste PR por serem causas relacionadas.

**Os dois pipelines devem permanecer separados.** A sobreposição de 78–86% justifica extração de helpers, não unificação. As gramáticas são fundamentalmente diferentes.

---

*Parecer escrito por Hefesto (hefesto-tf), Code Quality Specialist.*
*Arquivos de produto não foram modificados. Único arquivo escrito nesta sessão: este parecer.*
