# pwsh paralelo → corrida no cache JIT → FileLoadException com assembly names embaralhados

**Arquivo:** `internal/generators/pwsh_cache_isolation_test.go` (helper), `guard_line_shell_test.go` (5 pontos)
**Descoberto em:** ML-1D, REQ-2026-11-20, 2026-11-20

## O problema

Quando vários testes paralelos invocam `pwsh` ao mesmo tempo (via `execPSCommand`,
`execPSCommandOpts`, `execPSCommandInteractive`, `psProbe`, `psWarmup`), os processos
herdam o mesmo `HOME` e `XDG_CACHE_HOME` do processo pai. Todos gravam no mesmo arquivo
de perfil de startup do .NET:

```
$XDG_CACHE_HOME/powershell/StartupProfileData-NonInteractive
$XDG_CACHE_HOME/powershell/StartupProfileData-Interactive
```

(ou `$HOME/.cache/powershell/` se `XDG_CACHE_HOME` não estiver definido).

O .NET runtime usa esses arquivos para `ProfileOptimization` (multicore JIT warmup).
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

## Medição

- Medido localmente: `XDG_CACHE_HOME=/tmp/x pwsh -NoProfile -Command 'exit 0'` cria
  `/tmp/x/powershell/StartupProfileData-NonInteractive` (confirmado nesta sessão).
- Corrida local (macOS, .NET 11, 32 pwsh paralelos): 0 erros — .NET 11 mais resiliente.
- Corrida no CI (ubuntu-latest, run 44826888100, ML-1C): 46 `obteve -1` num shard, todos
  com FileLoadException e assembly names embaralhados.

## Correção

Função helper `newPwshCacheEnv(env []string) ([]string, func())` em
`internal/generators/pwsh_cache_isolation_test.go`:
- No-op no Windows (Windows usa `%LOCALAPPDATA%`, não `XDG_CACHE_HOME`).
- No Linux/macOS: filtra `XDG_CACHE_HOME` de `env`, cria `os.MkdirTemp("", "pwsh-jit-*")`,
  acrescenta `XDG_CACHE_HOME=<dir>`. Limpeza via `os.RemoveAll` após `cmd.Run()`.

Chamada em 5 pontos: `execPSCommand`, `execPSCommandOpts`, `execPSCommandInteractive`,
`psProbe`, `psWarmup`.

**HOME é deixado intacto**: testes de perfil PS usam HOME para localizar o perfil; alterar
HOME mudaria o que esses testes medem.

## Notas adicionais

- `psWarmup` usa `sync.OnceValues` (roda uma vez por binário), mas pode correr com
  invocações paralelas dos próprios testes; isolado por completude.
- Com caches isolados, warmup e chamadas reais operam em regime cold-start consistente
  (sem JIT pré-aquecido), tornando os deadlines derivados do warmup mais precisos.
- O deadline floor de 15s é suficiente mesmo com cold JIT start.
