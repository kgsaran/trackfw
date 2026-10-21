# Medição Windows: candidato guard zsh (REQ-2026-10-20)

> Data: 2026-10-21 | Agente: artemis-tf | VM: powershell-vm (Windows 11 ARM64, WIN-C83B0R5MIGB)
> Branch: `fix/hooks-de-guard-gerados-falham-sob-zsh`
> Roadmap: `docs/roadmaps/wip/ROADMAP-2026-10-20-hooks-de-guard-gerados-falham-sob-zsh-com-exit-1-e-deixam-de-executar-a-protecao.md`
> Residual coberto: R1 (PS 5.1 e Git Bash não medidos na Wave 0 para o candidato)
> Script de medição: v9 (definitivo — `$argStr` correto, `bash -c "line"` direto, stdin enviado)

## Contexto

A Wave 0 (ML-0A) identificou que a linha ATUAL de guard falha em zsh com exit 1 (fail-open) e propôs o candidato com prefixo `unsetopt EQUALS NOMATCH 2>${null-/dev/null}; `. A Wave 0 mediu o candidato em sh/bash/dash/zsh no macOS, mas **não mediu em PowerShell 5.1 nem em Git Bash (Windows)** — resíduo R1 declarado. Este relatório fecha R1.

**Nota técnica**: tentativas anteriores (v2–v8) falharam por um bug na função auxiliar de medição: o parâmetro nomeado `$args` conflita com a variável automática `$args` do PowerShell, fazendo os processos filhos receberem string de argumentos vazia. Bash e PS5 sem argumentos lêem stdin como script — causando os erros `{tool:guard}: command not found` e `Token ':"guard"' inesperado` observados anteriormente. Isso explica todos os resultados inconsistentes. A v9 usa `$argStr` e reproduz o caminho de produção documentado pela ADR D11.

## Linhas medidas

**ATUAL** (151 caracteres):
```
$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard x; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE
```
SHA-256: `10119f9e15f9b5369c1b2635a65f199bdbd4983d5b6ada0dbd72f600f3ebe2b7`

**CANDIDATA** (196 caracteres):
```
unsetopt EQUALS NOMATCH 2>${null-/dev/null}; $LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard x; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE
```
SHA-256: `321d5586aadc8f1eeb382455dff8520c9579ef521382d270f39be53f5a0adf10`

Comprimentos e SHA-256 verificados por script ao início da rodada (v9).

## Ambiente de medição

| Item | Valor |
|------|-------|
| Plataforma | Windows 11 ARM64 |
| PowerShell | 5.1 (`powershell.exe`) |
| Git Bash | GNU bash 5.3.15(1)-release (x86_64-pc-cygwin) |
| pwsh (PS 7) | NÃO INSTALADO nesta VM |
| `trackfw` falso cmd | `trackfw.cmd`: escreve `invoked.txt`, captura stdin via `findstr "^"`, sai com `%FAKE_RC%` |
| `trackfw` falso bash | `trackfw` (`chmod +x`): escreve `invoked-bash.txt`, captura stdin via `cat`, sai com `$FAKE_RC` |
| PATH | WorkDir primeiro; `C:\Users\Lab\bin` (trackfw real) **excluído** |
| Verificação PATH | `where.exe trackfw` → fake confirmado; AbsentPath → NOT_FOUND confirmado |

## Método de invocação (v9 — produção)

**PS5**: linha transmitida via `powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -EncodedCommand <base64-utf16le>`. Stdin com payload `{"tool":"guard","path":"C:\\project"}` enviado e fechado. Caminho de produção: IDEs (Claude Code, etc.) invocam o guard via `-Command`; `-EncodedCommand` é equivalente funcional.

**Git Bash**: linha executada via `bash -c "export PATH='...'; export FAKE_RC='...'; . gline.sh"`. Stdin com payload enviado e fechado. Caminho de produção: IDEs (Claude Code, etc.) invocam via `bash -c "..."`.

## Tabela de resultados

Casos: **allow** = `FAKE_RC=0`, **deny** = `FAKE_RC=2`, **err** = `FAKE_RC=1`, **ausente** = sem `trackfw` no PATH.

Expectativa ADR D11 (família PS/POSIX): allow=0, deny=2, err=2, ausente=2.

| Shell | Linha | allow | deny | err | ausente | stdin capturado |
|-------|-------|-------|------|-----|---------|-----------------|
| PS 5.1 | ATUAL | **0** ✓ | **2** ✓ | **2** ✓ | **2** ✓ | Sim (allow/deny/err); N/A (ausente) |
| PS 5.1 | CANDIDATA | **0** ✓ | **2** ✓ | **2** ✓ | **2** ✓ | Sim (allow/deny/err); N/A (ausente) |
| Git Bash | ATUAL | **0** ✓ | **2** ✓ | **2** ✓ | **2** ✓ | Sim (allow/deny/err); N/A (ausente) |
| Git Bash | CANDIDATA | **0** ✓ | **2** ✓ | **2** ✓ | **2** ✓ | Sim (allow/deny/err); N/A (ausente) |
| cmd.exe | ATUAL | **1** ✗ | **1** ✗ | **1** ✗ | **1** ✗ | Não |
| cmd.exe | CANDIDATA | **1** ✗ | **1** ✗ | **1** ✗ | **1** ✗ | Não |
| pwsh | — | — | — | — | — | — |

Invocação confirmada: `invoked.txt` (PS5) e `invoked-bash.txt` (Git Bash) criados para todos os casos com fake no PATH.

## Verificação de stdin

Em todos os casos onde o fake foi invocado (allow, deny, err), o fake capturou:

```
{"tool":"guard","path":"C:\\project"}
```

PS5 e Git Bash confirmam que o payload flui do processo externo → shell → subcomando `trackfw guard x` sem redirecionamento. A linha de guard não redireciona stdin (`<`/`<<`), portanto o JSON do hook chega ao `trackfw` pelo stdin herdado.

**Ausente**: invocação não ocorre; stdin não chega ao fake (correto, não há subcomando).

**cmd.exe**: stdin não capturado — o redirect `2>${null-/dev/null}` falha antes de `trackfw` ser invocado.

## Stderr observado (casos representativos — CLIXML extraído)

**PS 5.1 — ATUAL — allow/deny/err:**
```
LASTEXITCODE=$((2*!!$?)) : O termo 'LASTEXITCODE=$((2*!!$?))' não é reconhecido como nome de cmdlet, função, arquivo de script ou programa operável. Verifique a grafia do nome...
```
Ruído esperado (ADR D11): o trecho `LASTEXITCODE=…` sem `$` prefixado é tratado como comando externo em PS. Não afeta exit codes.

**PS 5.1 — CANDIDATA — allow/deny/err:**
```
unsetopt : O termo 'unsetopt' não é reconhecido como nome de cmdlet, função, arquivo de script ou programa operável. Verifique a grafia do nome...
```
(+ mesmo ruído da ATUAL) — O prefixo adiciona um CommandNotFound para `unsetopt`. Não afeta exit codes.

**PS 5.1 — ATUAL — ausente:**
```
trackfw : O termo 'trackfw' não é reconhecido como nome de cmdlet, função, arquivo de script ou programa operável.
```
`$LASTEXITCODE` permanece no valor semeado (2); step 4: `2*!!2=2`; `exit 2`. Fail-closed correto.

**PS 5.1 — CANDIDATA — ausente:**
Mesmo que ATUAL, com adição de `unsetopt` não reconhecido. Exit 2 ✓.

**Git Bash — ATUAL/CANDIDATA — ausente:**
```
.../gline.sh: line 1: trackfw: command not found
```
`$?=127`; `LASTEXITCODE=$((2*!!127))=2`; `exit 2`. Fail-closed correto.

**cmd.exe — todos os casos:**
```
O sistema não pode encontrar o caminho especificado.
```

## Achados

### A1 — PS 5.1: CANDIDATA equivalente à ATUAL (sem regressão, sem melhoria)

Exit codes idênticos para todos os 4 casos com stdin confirmado. O prefixo `unsetopt EQUALS NOMATCH 2>${null-/dev/null}` acrescenta um CommandNotFound ao stderr, mas o mecanismo de exit code é o step 4 (`$LASTEXITCODE=2*!!$LASTEXITCODE`), não afetado. Comportamento esperado: o prefixo foi concebido para corrigir o zsh, não o PS.

### A2 — Git Bash: CANDIDATA equivalente à ATUAL (sem regressão)

Exit codes idênticos para todos os 4 casos com stdin confirmado. `unsetopt` não é um builtin do bash; `2>${null-/dev/null}` suprime o erro. Sem diferença de comportamento observável. Esperado por construção.

### A3 — cmd.exe: ambas as linhas saem 1 para todos os casos (gap pré-existente)

O redirect `2>${null-/dev/null}` falha em cmd.exe (`${null-/dev/null}` não é um caminho válido — cmd não expande `${...}`). Resultado: rc=1 para **todos** os casos — allow, deny, err e ausente. Isso inclui deny/err/ausente (deveria ser 2) e allow (deveria ser 0): **todas as direções são fail-open em cmd**.

Este gap é **pré-existente e igual para ATUAL e CANDIDATA**. A ADR D7/D11 exclui cmd.exe da família PS/POSIX: "A linha de hook para a família cmd.exe (Kiro, Amazon Q) usa `trackfw guard <nome> || exit 2`, não a linha polyglot." A medição no `docs/portabilidade/2026-10-07-linha-de-hook-fail-closed-sem-trackfw.md` registrou Cpoly (linha polyglot) em cmd = 1/1/1 para allow/deny/err — corrobora este achado. A CANDIDATA não altera o comportamento em cmd.

### A4 — pwsh (PS 7) ausente nesta VM

R1 declarou PS 5.1 como medição mínima obrigatória. pwsh não está instalado em powershell-vm ARM64. A ADR D11 não distingue PS 5 de PS 7 nos mecanismos usados (`$LASTEXITCODE`, CommandNotFound, atribuição). Risco estimado baixo, mas medição direta ausente.

### A5 — Bug `$args` nos scripts v2–v8 (retrospectiva)

Em PS, `$args` é uma variável automática que recebe os argumentos posicionais não vinculados da função. Usar `param([string]$args, ...)` **não declara** um parâmetro chamado `$args`; a variável automática tem precedência. O `ProcessStartInfo` filho recebia string de argumentos vazia → shell sem argumentos lê stdin como script → erros `{tool:guard}: command not found` e `Token ':"guard"' inesperado`. A v9 usa `$argStr`, corrigindo o problema. **Este bug mascarou os resultados de todas as iterações anteriores.**

## Veredito

**CANDIDATA APROVADA para PS 5.1 e Git Bash** (Windows ARM64, invocação de produção confirmada, stdin provado).

| Shell | Veredito | Base |
|-------|----------|------|
| PS 5.1 | ✅ APROVADA | allow=0, deny=2, err=2, ausente=2 — idêntico à ATUAL; stdin capturado |
| Git Bash | ✅ APROVADA | allow=0, deny=2, err=2, ausente=2 — idêntico à ATUAL; stdin capturado |
| cmd.exe | ⚠️ Gap pré-existente | Linha PS/POSIX não é para cmd (ADR D7/D11); exit 1 para todos os casos idêntico entre ATUAL e CANDIDATA |
| pwsh | ⚠️ NÃO MEDIDO | pwsh não instalado na VM |

A CANDIDATA não introduz nenhuma regressão em PS 5.1 ou Git Bash. O prefixo `unsetopt EQUALS NOMATCH 2>${null-/dev/null}` é neutro nesses shells. O único efeito observável é em zsh (objetivo da correção, medido no macOS pela Wave 0).

## Resíduos após esta medição

- **R1 fechado**: PS 5.1 e Git Bash medidos com resultados corretos, invocação de produção, stdin provado.
- **R_pwsh**: pwsh ausente na VM powershell-vm; se ML-1A precisar de garantia para PS 7, requer VM diferente ou instalação de pwsh. Mecanismo idêntico ao PS 5, risco baixo.
- **R_cmd**: gap pré-existente; fora do escopo desta correção.

## Cleanup

Todos os arquivos temporários da VM confirmados removidos pela v9:

```
Gone: C:\Users\Lab\tf-guard-zsh-m8-measure8.ps1
Gone: C:\Users\Lab\diag7.ps1
Gone: C:\Users\Lab\tf-guard-zsh-20261021-measure7.ps1
Gone: C:\Users\Lab\tf-guard-zsh-20261021-script.ps1
Gone: C:\Users\Lab\AppData\Local\Temp\guard-results.txt
WorkDir C:\Users\Lab\tf-guard-zsh-m9: removed OK
```

---

## Rodada 2 — forma emulate -R sh (2026-10-21)

> Agente: artemis-tf | VM: powershell-vm | Script: measure-r2-v2.ps1 (adaptado de v9)
> Pergunta: `emulate -R sh 2>${null-/dev/null}` funciona como substituto do prefixo `unsetopt EQUALS NOMATCH`?
> Escopo: PS 5.1 e Git Bash (cmd.exe excluído por ADR D11).

### Linha medida

```
emulate -R sh 2>${null-/dev/null}; $LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard x; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE
```

**Comprimento:** 186 bytes (UTF-8, sem BOM)
**SHA-256:** `4787c8ac9a2c6074b18877f575e352e8cdbd2bcf31171cdb8982269a3b9a8626`

Verificado no host macOS via `python3 hashlib.sha256`. A VM computou o mesmo hash via `[System.Security.Cryptography.SHA256]::Create()` sobre os bytes UTF-16LE da string PS — fonte de verdade é o host (hash direto sobre UTF-8). A linha enviada ao shell PS5 chega por construção (a `-EncodedCommand` codifica o mesmo valor de `$R2`); não foi verificada decodificando o base64 na VM.

**Linha de diagnóstico** (sem o `2>` no prefixo emulate, para captura de stderr bruto):

```
emulate -R sh; $LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard x; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE
```

Comprimento: 166 bytes | SHA-256: `c2ef485765eff7e25dab54d69b94e2cc14addfe89ba86a33fcd1ebd366cf28e3`

### Ambiente

Idêntico a Rodada 1 (tabela do relatório acima). Workdir novo: `C:\Users\Lab\tf-guard-emulate-20261021\`. `C:\Users\Lab\bin` excluído do PATH dos subprocessos, confirmado com `where.exe trackfw` retornando exclusivamente o fake.

Verificação Git Bash:
```
with WorkDir:  trackfw is /c/Users/Lab/tf-guard-emulate-20261021/trackfw
               /usr/bin/bash: line 1: type: emulate: not found
without WorkDir (absent): /usr/bin/bash: line 1: type: trackfw: not found
```

`emulate` é confirmado como **não encontrado** em Git Bash (não é builtin nem externo no ambiente da VM). Fake de PS5 (`trackfw.cmd`) e fake de Git Bash (`trackfw`) confirmados como resolução exclusiva no PATH de teste.

### Método de invocação

Idêntico ao v9:
- **PS5**: `powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -EncodedCommand <base64-utf16le>`. PATH e FAKE_RC injetados via `ProcessStartInfo.EnvironmentVariables` (não embutidos no `-EncodedCommand`). Linha codificada é exatamente a linha acima.
- **Git Bash**: `bash -c "export PATH='...'; export FAKE_RC='N'; . <workdir>/gline.sh"`. Linha escrita em `gline.sh` sem BOM, sem `\n` final adicionado.
- Stdin: `{"tool":"guard","path":"C:\\project"}` enviado e fechado. Validação de conteúdo: leitura do arquivo capturado e comparação byte a byte com o payload esperado (`stdinOK=True`).

### Tabela de resultados

Casos: **allow** = `FAKE_RC=0`, **deny** = `FAKE_RC=2`, **err** = `FAKE_RC=1`, **ausente** = sem fake no PATH.

Expectativa: allow=0, deny=2, err=2, ausente=2.

| Shell | Caso | RC | Invocado | stdin OK | Conteúdo stdin | Veredito |
|-------|------|----|----------|----------|----------------|----------|
| PS 5.1 | allow | **0** | Sim | **Sim** | `{"tool":"guard","path":"C:\\project"}` | ✓ |
| PS 5.1 | deny | **2** | Sim | **Sim** | `{"tool":"guard","path":"C:\\project"}` | ✓ |
| PS 5.1 | err | **2** | Sim | **Sim** | `{"tool":"guard","path":"C:\\project"}` | ✓ |
| PS 5.1 | ausente | **2** | Não | N/A | (not captured) | ✓ herdado¹ |
| Git Bash | allow | **0** | Sim | **Sim** | `{"tool":"guard","path":"C:\\project"}` | ✓ |
| Git Bash | deny | **2** | Sim | **Sim** | `{"tool":"guard","path":"C:\\project"}` | ✓ |
| Git Bash | err | **2** | Sim | **Sim** | `{"tool":"guard","path":"C:\\project"}` | ✓ |
| Git Bash | ausente | **2** | Não | N/A | (not captured — stderr: `trackfw: command not found`) | ✓ |

¹ PS5 ausente: rc=2 e invoked=False confirmados. A prova completa (nenhum `trackfw` alternativo no CleanPath) é herdada da Rodada 1, onde a mesma VM e o mesmo CleanPath produziram `trackfw : O termo 'trackfw' não é reconhecido…` no stderr. O stderr desta rodada foi truncado a 300 chars antes de alcançar a mensagem de trackfw (emulate CNF ocupou a primeira parte).

stdin verificado por comparação de string após `.Trim()` (não comparação byte a byte — conteúdo ASCII simples, sem BOM no payload).

Resultado 0/2/2/2 nos dois shells. Base de comparação: ATUAL e CANDIDATA da Rodada 1 (mesma VM, mesmo dia).

### Stderr observado

**PS 5.1 — todos os casos (extraído do CLIXML via strip, truncado a 300 chars):**
```
emulate : O termo 'emulate' não é reconhecido como nome de cmdlet, função, arquivo de script ou programa operável.
Verifique a grafia do nome ou, se um caminho tiver sido incluído, veja se o caminho está correto e tente novamente.
No linha:1 caractere:1; + emulate -R sh 2>${null-/d...
```

**Observação — redirect inerte no PS5:** O `2>${null-/dev/null}` no prefixo `emulate` aparece no CLIXML **com e sem o redirect** (braço principal R2 vs braço de diagnóstico R2_DIAG). O registro de CNF aparece nos dois casos. O redirect é **inerte em PS5** para esse tipo de erro: não suprime nem restringe a emissão do CLIXML. A execução continua porque `CommandNotFoundException` é **não-terminante por padrão em PS5** (não aborta o pipeline), e não porque o redirect engole o erro. Comportamento idêntico ao de `unsetopt EQUALS NOMATCH 2>${null-/dev/null}` (Rodada 1).

**Git Bash — ausente:**
```
/c/Users/Lab/tf-guard-emulate-20261021/gline.sh: line 1: trackfw: command not found
```
`$?=127`; `LASTEXITCODE=$((2*!!127))=2`; `exit 2`. Fail-closed correto.

**Git Bash — allow/deny/err:** sem stderr (emulate não encontrado → `$?=127`, suprimido pelo `2>/dev/null`; resto da linha executa normalmente).

### Stderr bruto do diagnóstico (PS 5.1, emulate sem redirect, caso allow)

```xml
#< CLIXML
<Objs ...>
  <S S="Error">emulate : O termo 'emulate' não é reconhecido como nome de cmdlet, função,
arquivo de script ou programa operável. </S>
  <S S="Error">Verifique a grafia do nome ou, se um caminho tiver sido incluído, veja se o
caminho está correto e tente novamente.</S>
  <S S="Error">No linha:1 caractere:1</S>
  <S S="Error">+ emulate -R sh; $LASTEXITCODE=2 2&gt;${null-/dev/null}; trackfw guard x; ...</S>
  <S S="Error">+ ~~~~~~~</S>
  <S S="Error">    + CategoryInfo : ObjectNotFound: (emulate:String) [], CommandNotFoundException</S>
  <S S="Error">    + FullyQualifiedErrorId : CommandNotFoundException</S>
  <S S="Error">LASTEXITCODE=$((2*!!$?)) : O termo 'LASTEXITCODE=$((2*!!$?))' não é reconhecido...</S>
  <S S="Error">    + FullyQualifiedErrorId : CommandNotFoundException</S>
</Objs>
```

**RC do diagnóstico: 0** (allow). Invocado: Sim. stdin OK: Sim.

Decodificado: `emulate -R sh` gera `CommandNotFoundException` com `FullyQualifiedErrorId = CommandNotFoundException`. Execução continua para o próximo `;`. `$LASTEXITCODE` não é alterado por `CommandNotFoundException` (só exit code de comando externo o modifica). O fake `trackfw guard x` roda e define `$LASTEXITCODE=0` (allow). Saída final: 0. Demonstra que o prefixo `emulate -R sh` é engolido sem efeito no exit code final.

### Achados

#### A6 — PS 5.1: `emulate -R sh` é CommandNotFound não-terminante; redirect é inerte

Confirmado pelo braço de diagnóstico (R2_DIAG, sem `2>` no prefixo): `FullyQualifiedErrorId = CommandNotFoundException`. Execução continua para `trackfw guard x`. `$LASTEXITCODE` final = exit code do fake (0 no braço allow, confirmado). Portanto:

- **`emulate -R sh` é CommandNotFound?** Sim.
- **O redirect `2>${null-/dev/null}` suprime algo no PS5?** Não. O registro CNF aparece no CLIXML com e sem o redirect. O redirect é **inerte** para `CommandNotFoundException` no PS5 — o mecanismo que permite a execução continuar é que CNF é **não-terminante** em PS5, independente de qualquer redirect.
- **`$LASTEXITCODE` é alterado pelo CNF?** Não. `CommandNotFoundException` não é um comando externo e não altera `$LASTEXITCODE`. Apenas exit codes de processos externos fazem isso.
- **Diferença vs ATUAL:** um registro CNF extra no CLIXML stderr. Nenhuma diferença de exit code. Mesma classe que `unsetopt EQUALS NOMATCH 2>${null-/dev/null}` da Rodada 1.

#### A7 — Git Bash: `emulate` não encontrado como externo, suprimido pelo `2>/dev/null` efetivo

Em Git Bash, `${null-/dev/null}` = `/dev/null` (bash shell parameter expansion: `$null` indefinido → usa o default `/dev/null`). Portanto `2>${null-/dev/null}` = `2>/dev/null`. O comando `emulate -R sh 2>/dev/null` tenta executar `emulate` como externo, falha com rc=127, stderr suprimido. `$?=127` não afeta `LASTEXITCODE` (variável bash independente do `$?` do emulate). A linha continua normalmente.

### Veredito

**APROVADA para PS 5.1 e Git Bash** (0/2/2/2 nos dois shells; stdin verificado por comparação de string com `.Trim()`, payload ASCII sem BOM; invocação do fake confirmada nos casos allow/deny/err).

| Shell | Veredito | Base |
|-------|----------|------|
| PS 5.1 | ✅ APROVADA | allow=0, deny=2, err=2, ausente=2; stdin payload verificado; sem regressão vs ATUAL/CANDIDATA |
| Git Bash | ✅ APROVADA | allow=0, deny=2, err=2, ausente=2; stdin payload verificado; sem regressão vs ATUAL/CANDIDATA |
| cmd.exe | ⚠️ Fora de escopo | ADR D7/D11 exclui cmd da linha polyglot; não medido nesta rodada |

A linha `emulate -R sh 2>${null-/dev/null}; ...` é funcionalmente equivalente a `unsetopt EQUALS NOMATCH 2>${null-/dev/null}; ...` (Rodada 1) em PS 5.1 e Git Bash. O prefixo `emulate -R sh` é neutro nesses dois shells — gera CommandNotFound suprimido, sem alterar exit code.

### Cleanup

Verificação pós-medição via `EncodedCommand` (evita ambiguidade de quoting do shell externo):

```
WorkDir exists=False                          ← C:\Users\Lab\tf-guard-emulate-20261021 removido
measure-r2*.ps1 before=0 after=0             ← nenhum script de medição remanescente
Leftover procs (not self): (none)            ← nenhum powershell/bash órfão
```
