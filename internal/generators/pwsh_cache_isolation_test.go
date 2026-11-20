package generators

// pwsh_cache_isolation_test.go — REQ-2026-11-20 ML-1D: isolamento do cache JIT por invocação de pwsh
//
// PowerShell 7 no Linux/macOS determina o diretório do cache de inicialização do .NET
// (ProfileOptimization) a partir de XDG_CACHE_HOME:
//
//   PowerShell/src/Microsoft.PowerShell.ConsoleHost/host/msh/ConsoleHost.cs:140-313:
//     profileDir = Platform.CacheDirectory   →  $XDG_CACHE_HOME/powershell
//     ProfileOptimization.SetProfileRoot(profileDir)
//     ProfileOptimization.StartProfile("StartupProfileData-NonInteractive")  // ou -Interactive
//
//   PowerShell/src/System.Management.Automation/CoreCLR/CorePsPlatform.cs:333:
//     XDG_Type.CACHE → $XDG_CACHE_HOME/powershell  (ou $HOME/.cache/powershell se não definido)
//
// Quando os testes paralelos lançam múltiplos processos pwsh, todos eles herdam o mesmo
// HOME/XDG_CACHE_HOME do processo de teste. O .NET escreve o arquivo de perfil de inicialização
// no encerramento do processo; dois processos gravando o mesmo arquivo simultaneamente corrompem-no,
// produzindo nomes de assembly embaralhados e FileLoadException na próxima leitura:
//
//   "System.IO.FileLoadException: The given assembly name was invalid.
//    File name: 'System.Linq, Version=10.0.0.0.System.Runtime...'"
//
// Medição direta: XDG_CACHE_HOME=/tmp/x pwsh -NoProfile -Command 'exit 0' cria
// /tmp/x/powershell/StartupProfileData-NonInteractive (medido nesta sessão).
// A corrida não reproduz localmente (macOS, .NET 11); reproduz no CI (ubuntu-latest, Linux,
// run 44826888100 ML-1C): 46 exit-1 num shard do braço "after", todos com FileLoadException.
//
// Correção: dar a cada invocação de pwsh seu próprio XDG_CACHE_HOME via os.MkdirTemp.
// O diretório temporário é removido pela função de limpeza retornada, após cmd.Run() retornar.
// HOME é deixado intacto: alguns testes afirmam comportamento de perfil PS via HOME;
// alterar HOME mudaria o que esses testes medem.

import (
	"os"
	"runtime"
	"strings"
)

// newPwshCacheEnv strip XDG_CACHE_HOME de env e substitui por um diretório único criado
// com os.MkdirTemp. Retorna o env modificado e uma função de limpeza.
//
// No Windows: devolve env sem alteração e limpeza no-op. O Windows não usa XDG_CACHE_HOME
// para o cache do .NET ProfileOptimization (usa %LOCALAPPDATA%); isolamento desnecessário.
//
// No Linux/macOS: cria um diretório temporário único, filtra qualquer XDG_CACHE_HOME
// existente no env (incluindo entradas de extraEnv já anexadas), e acrescenta
// XDG_CACHE_HOME=<dir>. A limpeza remove o diretório com os.RemoveAll.
//
// HOME é deixado intencionalmente intacto: os testes de perfil PS afirmam comportamento
// que depende de HOME; substituí-lo alteraria o que esses testes medem.
func newPwshCacheEnv(env []string) ([]string, func()) {
	if runtime.GOOS == "windows" {
		return env, func() {}
	}
	cacheDir, err := os.MkdirTemp("", "pwsh-jit-*")
	if err != nil {
		// Sem diretório isolado: retorna env original sem modificação.
		// É melhor aceitar o cache compartilhado do que impedir a chamada ao pwsh.
		return env, func() {}
	}
	result := make([]string, 0, len(env)+1)
	for _, e := range env {
		if strings.HasPrefix(e, "XDG_CACHE_HOME=") {
			continue
		}
		result = append(result, e)
	}
	result = append(result, "XDG_CACHE_HOME="+cacheDir)
	return result, func() { _ = os.RemoveAll(cacheDir) }
}
