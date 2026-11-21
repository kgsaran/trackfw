# pwsh paralelo → corrida no cache JIT → FileLoadException com assembly names embaralhados

**Arquivo:** `internal/generators/pwsh_cache_isolation_test.go` (helper), `guard_line_shell_test.go` (5 pontos)
**Descoberto em:** ML-1D, REQ-2026-11-20, 2026-11-20
**Atualizado em:** ML-1D2, 2026-11-20

## O problema

Quando vários testes paralelos invocam `pwsh` ao mesmo tempo (via `execPSCommand`,
`execPSCommandOpts`, `execPSCommandInteractive`, `psProbe`, `psWarmup`), os processos
herdam o mesmo `HOME` e `XDG_CACHE_HOME` do processo pai. Todos gravam no mesmo arquivo
de perfil de startup do .NET:

```
$XDG_CACHE_HOME/powershell/StartupProfileData-NonInteractive
```

(ou `$HOME/.cache/powershell/` se `XDG_CACHE_HOME` não estiver definido).

O .NET runtime usa esse arquivo para `ProfileOptimization` (multicore JIT warmup).
Quando dois processos escrevem o mesmo arquivo simultaneamente, os assembly names ficam
embaralhados, e qualquer processo que leia o arquivo corrompido falha com:

```
Unhandled exception. System.IO.FileLoadException: The given assembly name was invalid.
File name: 'System.Linq, Version=10.0.0.0.System.Runtime.Serialization.Formatters, Version=8.'
   at System.Reflection.AssemblyNameParser.Parse(ReadOnlySpan`1 name)
   at System.Reflection.AssemblyName.ParseAsAssemblySpec(Char* pAssemblyName, Void*)
```

O pwsh sai com exit -1 (SIGABRT / unhandled exception), e o stdin.out não é gerado.

## Fonte (PowerShell source)

**`src/Microsoft.PowerShell.ConsoleHost/host/msh/ConsoleHost.cs:140-313`:**
```csharp
string profileDir = Platform.CacheDirectory;   // $XDG_CACHE_HOME/powershell
ProfileOptimization.SetProfileRoot(profileDir);
ProfileOptimization.StartProfile("StartupProfileData-NonInteractive");
// ou: StartProfile("StartupProfileData-Interactive");
```

**`src/System.Management.Automation/CoreCLR/CorePsPlatform.cs:333`:**
```csharp
string xdgcachehome = Environment.GetEnvironmentVariable("XDG_CACHE_HOME");
// XDG_Type.CACHE:
return string.IsNullOrEmpty(xdgcachehome)
    ? Path.Combine(envHome, ".cache", "powershell")
    : Path.Combine(xdgcachehome, "powershell");
```

## Medição (ML-1D)

- Medido: `env XDG_CACHE_HOME=/tmp/x pwsh -NoProfile -Command 'exit 0'` cria
  `/tmp/x/powershell/StartupProfileData-NonInteractive` (confirmado nesta sessão).
- Corrida local (macOS, pwsh 7.7.0, 32 pwsh paralelos): arquivo SHA mudou no braço
  writable-compartilhado (contra-braço positivo — corrida detectável localmente).
  As 0 falhas de exit code em 32 paralelos locais refletem .NET 11 mais resiliente,
  não ausência da corrida.
- Corrida no CI (ubuntu-latest, run 44826888100, ML-1C): 46 `obteve -1` num shard, todos
  com FileLoadException e assembly names embaralhados.

## Correção ML-1D (dir vazio por invocação)

Função `newPwshCacheEnv(psBin string, env []string) ([]string, func())` em
`internal/generators/pwsh_cache_isolation_test.go`:
- No-op no Windows (Windows usa `%LOCALAPPDATA%`, não `XDG_CACHE_HOME`).
- No Linux/macOS: filtra `XDG_CACHE_HOME` de `env`, cria `os.MkdirTemp("", "pwsh-jit-*")`,
  acrescenta `XDG_CACHE_HOME=<dir>`. Limpeza via `os.RemoveAll` após `cmd.Run()`.

Chamada em 5 pontos: `execPSCommand`, `execPSCommandOpts`, `execPSCommandInteractive`,
`psProbe`, `psWarmup`.

**Resultado do ML-1D (run 44833943635):** `obteve -1` = 0 ✓, mas cada pwsh arranca a
frio: 0.12 s → 1.48 s de aquecimento no ubuntu-latest. Sob 4 shards com dezenas de
paralelos, o prazo de 15 s por chamada foi excedido e a carga estourou 20 m.

**Lição:** `O piso de 15 s é insuficiente quando o cache está vazio e o runner tem muitos
paralelos.` A nota anterior "piso de 15s suficiente mesmo com cold JIT start" foi
falsificada pelo run 44833943635 — não mantê-la.

## Correção ML-1D2 (cache pré-aquecido por cópia)

`psWarmup` (que já usa `sync.OnceValues`, rodando em sequência antes dos paralelos) cria
um diretório template com `os.MkdirTemp("", "pwsh-warm-*")`, executa
`pwsh -NoProfile -NonInteractive -Command 'exit 0'`, e registra o template via
`setPwshWarmCache(psBin, warmDir)`.

`newPwshCacheEnv(psBin, env)`: se existe template, chama `copyPwshWarmDir(warmDir)`
(copia `powershell/StartupProfileData-NonInteractive` e `telemetry.uuid`, ~64 KB total),
retorna dir copiado (writável) com limpeza normal. Fallback: dir vazio (ML-1D).

Cada processo pwsh recebe sua própria cópia do cache: leitura pré-aquecida + escrita
isolada, sem corrida, sem partida a frio.

## Medições ML-1D2 (macOS, pwsh 7.7.0, run 2026-11-20)

Todas as medições usam `bash TIMEFORMAT` no mesmo script para evitar ruído cross-run.

| modo              | média (10 runs) |
|-------------------|-----------------|
| cold (dir vazio)  | 230 ms          |
| warm+copy         | 171 ms          |
| warm+readonly     | 172 ms          |

Warm+copy e warm+readonly são equivalentes em velocidade (dentro do ruído de medição).
A vantagem de copy sobre read-only não é velocidade — são:
  1. Sem `chmod` → sem restore de permissões antes de `os.RemoveAll` (funciona sem TestMain).
  2. Funciona em contêineres root onde `chmod 0555` não bloqueia escrita.
  3. Cada processo pwsh tem sua cópia writável → sem corrida mesmo que o .NET atualize
     o perfil (isolamento por construção, não por bloqueio do SO).

- Measurement (b): `pwsh` exits 0, sem stderr e arquivo inalterado em cache read-only.
- Measurement (c): 32 paralelos read-only: 0 falhas; contra-braço (writable compartilhado):
  sha mudou — contra-braço não-vacuidade confirmado.
- As 3 formas de invocação (-NoProfile -NonInteractive, -NoProfile, sem flags) com stdin
  como pipe criam apenas StartupProfileData-NonInteractive. Fato medido; causa (stdin
  não-TTY) não verificada.

## Limpeza do template

`cleanupPwshWarmCaches()` está disponível em `pwsh_cache_isolation_test.go`.
**Pendente:** adicionar em `guard_parity_helper_test.go:TestMain` linha 182
(após `cleanupGuardBinary()`):
```go
cleanupPwshWarmCaches()
```
Enquanto não adicionada, templates ficam em `$TMPDIR/pwsh-warm-*` e são removidos pelo
OS (CI: contêiner descartado; Mac: /tmp limpo periodicamente). Os templates são writáveis,
então `os.RemoveAll` funciona sem restore de permissões.

## Notas adicionais

- HOME é deixado intacto: testes de perfil PS usam HOME; alterar HOME mudaria o que medem.
- `psWarmup` usa `sync.OnceValues` — o template é criado uma vez por binário antes de
  qualquer chamada paralela. Se criar o dir falhar, `newPwshCacheEnv` cai de volta no
  fallback ML-1D (dir vazio por invocação).
- O piso de 15s com cache pré-aquecido é confortável (171 ms por chamada no Mac). O tempo
  quente no CI ainda não foi medido — depende do rerun do estresse (ML-1D2 terceira checkbox,
  pendente). Nota: `aquecimento de pwsh` no CI continuará mostrando ~1.48 s (a primeira
  execução é fria por construção e define o prazo conservadoramente).
