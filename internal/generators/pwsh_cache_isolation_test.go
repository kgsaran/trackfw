package generators

// pwsh_cache_isolation_test.go — REQ-2026-11-20 ML-1D/ML-1D2:
// isolamento do cache JIT por invocação de pwsh (ML-1D) com cache pré-aquecido
// compartilhado por cópia (ML-1D2).
//
// Contexto ML-1D (corrida no StartupProfileData-*):
//
//   PowerShell/src/Microsoft.PowerShell.ConsoleHost/host/msh/ConsoleHost.cs:140-313:
//     profileDir = Platform.CacheDirectory   →  $XDG_CACHE_HOME/powershell
//     ProfileOptimization.SetProfileRoot(profileDir)
//     ProfileOptimization.StartProfile("StartupProfileData-NonInteractive")  // ou -Interactive
//
//   PowerShell/src/System.Management.Automation/CoreCLR/CorePsPlatform.cs:333:
//     XDG_Type.CACHE → $XDG_CACHE_HOME/powershell  (ou $HOME/.cache/powershell se não definido)
//
// Quando os testes paralelos lançam múltiplos processos pwsh, todos herdam o mesmo
// HOME/XDG_CACHE_HOME do processo de teste. O .NET escreve StartupProfileData-* na saída do
// processo; dois processos gravando o mesmo arquivo simultaneamente corrompem-no, produzindo
// nomes de assembly embaralhados e FileLoadException na próxima leitura.
//
// Correção ML-1D: dar um XDG_CACHE_HOME único a cada invocação via os.MkdirTemp. Eliminou
// FileLoadException. Mas cada pwsh passou a arrancar a frio, elevando o tempo de
// inicialização de 0.12 s para ~1.48 s no ubuntu-latest, o que fez chamadas excederem
// o prazo de 15 s e a carga estourar 20 m (run 44833943635).
//
// Correção ML-1D2 (este arquivo): cache pré-aquecido por cópia.
//
//   • psWarmup (sync.OnceValues por binário) cria um diretório template e executa
//     pwsh -NoProfile -NonInteractive sequencialmente, gerando
//     StartupProfileData-NonInteractive no template.
//   • newPwshCacheEnv, quando existe template para o binário, copia o template para um
//     diretório novo (writável) em vez de criar um diretório vazio.
//   • Cada processo pwsh recebe sua própria cópia: leitura pré-aquecida + escrita isolada,
//     sem corrida, sem partida a frio.
//
// Medições no macOS (pwsh 7.7.0, run 2026-11-20):
//   Medições em um único script bash (TIMEFORMAT='%3R'), 10 runs cada:
//   - Partida fria (dir vazio):   230 ms médio
//   - Partida quente + cópia:     171 ms médio
//   - Partida quente + read-only: 172 ms médio  (equivalente a cópia — sem vantagem de speed)
//   No CI (ubuntu-latest, run 44833943635): 1.48 s frio; estimativa quente pendente do rerun.
//   Escolha copy vs read-only: velocidade equivalente; copy não requer chmod nem restore de
//   permissões para os.RemoveAll, e funciona em contêineres root (onde chmod não bloqueia).
//   - Measurement (b): pwsh exits 0, sem stderr e arquivo inalterado em cache read-only.
//   - Measurement (c): 32 paralelos read-only: 0 falhas; contra-braço (writable compartilhado):
//     sha mudou — contra-braço não-vacuidade confirmado.
//   - As 3 formas de invocação (-NoProfile -NonInteractive, -NoProfile, sem flags) com stdin
//     como pipe criam apenas StartupProfileData-NonInteractive; InteractiveProfile não aparece.
//     Fato medido; causa (stdin não-TTY) não verificada.
//
// Limpeza do template: cleanupPwshWarmCaches() está disponível para TestMain.
// Pendente: adicionar cleanupPwshWarmCaches() em guard_parity_helper_test.go:TestMain (linha
// após `cleanupGuardBinary()`, linha 182) — fora do escopo de escrita desta QA.
// Os templates ficam em $TMPDIR/pwsh-warm-* até serem removidos pelo SO (no CI, o runner
// descarta o contêiner; no Mac, /tmp é limpo periodicamente).
//
// HOME é deixado intencionalmente intacto: os testes de perfil PS afirmam comportamento
// que depende de HOME; substituí-lo alteraria o que esses testes medem.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// --- Template de cache por binário -------------------------------------------

// pwshWarmCacheMu guarda pwshWarmCacheByBin para acesso concorrente seguro.
var pwshWarmCacheMu sync.Mutex

// pwshWarmCacheByBin mapeia psBin → diretório template pré-aquecido.
// Escrito uma vez (por psWarmup, que usa sync.OnceValues), lido por múltiplas goroutines.
// Template permanece gravável para que os.RemoveAll funcione sem restaurar permissões.
var pwshWarmCacheByBin = make(map[string]string)

// getPwshWarmCache retorna o diretório template para psBin, ou "" se não existir.
// Seguro para leituras concorrentes via mutex.
func getPwshWarmCache(psBin string) string {
	pwshWarmCacheMu.Lock()
	defer pwshWarmCacheMu.Unlock()
	return pwshWarmCacheByBin[psBin]
}

// setPwshWarmCache registra o diretório template para psBin.
// Chamado apenas de dentro do sync.OnceValues de psWarmup (sequencial).
func setPwshWarmCache(psBin, dir string) {
	pwshWarmCacheMu.Lock()
	pwshWarmCacheByBin[psBin] = dir
	pwshWarmCacheMu.Unlock()
}

// newPwshWarmTemplate cria um diretório temporário a ser usado como template de cache
// pré-aquecido para psBin. Retorna o caminho e erro.
//
// No Windows: retorna ("", nil) sem criar nada. No Windows o cache do .NET usa
// %LOCALAPPDATA%, não XDG_CACHE_HOME; o template nunca seria lido (newPwshCacheEnv
// retorna cedo no Windows), e criar o dir seria uma perda desnecessária.
//
// No Linux/macOS: cria os.MkdirTemp("", "pwsh-warm-*") e retorna o caminho.
func newPwshWarmTemplate() (string, error) {
	if runtime.GOOS == "windows" {
		return "", nil
	}
	return os.MkdirTemp("", "pwsh-warm-*")
}

// cleanupPwshWarmCaches remove os diretórios template por binário.
// Destinado para TestMain: chamar após m.Run().
//
// Pendente: adicionar em guard_parity_helper_test.go:TestMain, linha após
// cleanupGuardBinary() (linha 182):
//
//	cleanupPwshWarmCaches()
func cleanupPwshWarmCaches() {
	pwshWarmCacheMu.Lock()
	dirs := make([]string, 0, len(pwshWarmCacheByBin))
	for _, dir := range pwshWarmCacheByBin {
		dirs = append(dirs, dir)
	}
	pwshWarmCacheByBin = make(map[string]string)
	pwshWarmCacheMu.Unlock()
	for _, dir := range dirs {
		_ = os.RemoveAll(dir)
	}
}

// --- Helpers de ambiente -----------------------------------------------------

// applyXDGCacheHome remove qualquer entrada XDG_CACHE_HOME de env e acrescenta a nova.
// Retorna slice novo; env não é modificado.
func applyXDGCacheHome(env []string, dir string) []string {
	result := make([]string, 0, len(env)+1)
	for _, e := range env {
		if !strings.HasPrefix(e, "XDG_CACHE_HOME=") {
			result = append(result, e)
		}
	}
	return append(result, "XDG_CACHE_HOME="+dir)
}

// copyPwshWarmDir copia os arquivos de StartupProfileData-* (e telemetry.uuid) do
// diretório template warmDir para um novo diretório temporário gravável.
//
// Estrutura copiada: $warmDir/powershell/* → $newDir/powershell/*
//
// Erros de cópia individual são ignorados (melhor entregar um dir parcialmente
// populado do que falhar a invocação). Retorna o caminho do novo dir e o erro de
// os.MkdirTemp. Retorna ("", err) somente se não foi possível criar o dir raiz.
func copyPwshWarmDir(warmDir string) (string, error) {
	dst, err := os.MkdirTemp("", "pwsh-jit-*")
	if err != nil {
		return "", err
	}
	psSrc := filepath.Join(warmDir, "powershell")
	psDst := filepath.Join(dst, "powershell")
	if mkErr := os.Mkdir(psDst, 0755); mkErr != nil {
		// Não conseguiu criar o subdir; retornar o dir raiz vazio (partida fria).
		return dst, nil
	}
	entries, rdErr := os.ReadDir(psSrc)
	if rdErr != nil {
		// Sem subdir no template (aquecimento não gerou arquivos); dir raiz vazio.
		return dst, nil
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, rdFileErr := os.ReadFile(filepath.Join(psSrc, e.Name()))
		if rdFileErr != nil {
			continue
		}
		_ = os.WriteFile(filepath.Join(psDst, e.Name()), data, 0644)
	}
	return dst, nil
}

// --- Isolamento de cache por invocação ---------------------------------------

// newPwshCacheEnv retorna env com um XDG_CACHE_HOME isolado e uma função de limpeza.
//
// No Windows: retorna env sem alteração e limpeza no-op. O Windows usa %LOCALAPPDATA%,
// não XDG_CACHE_HOME; isolamento desnecessário.
//
// No Linux/macOS, em dois modos:
//
//  1. Cache pré-aquecido (ML-1D2): se psBin != "" e existe template para esse binário,
//     copia o template (copyPwshWarmDir) para um dir novo e gravável. O pwsh lê o perfil
//     JIT pré-compilado (partida quente) e escreve na cópia isolada (sem corrida).
//     A limpeza remove a cópia via os.RemoveAll.
//
//  2. Fallback por invocação (ML-1D): se psBin == "" ou não há template, cria um dir
//     vazio novo (os.MkdirTemp), isolando a corrida mas forçando partida fria.
//     Usado durante o próprio aquecimento em psWarmup (que ainda não registrou o
//     template), ou se a criação do template tiver falhado.
//
// HOME é deixado intencionalmente intacto.
func newPwshCacheEnv(psBin string, env []string) ([]string, func()) {
	if runtime.GOOS == "windows" {
		return env, func() {}
	}

	// Modo 1: copiar template pré-aquecido.
	if psBin != "" {
		if warmDir := getPwshWarmCache(psBin); warmDir != "" {
			if copied, cpErr := copyPwshWarmDir(warmDir); cpErr == nil {
				return applyXDGCacheHome(env, copied), func() { _ = os.RemoveAll(copied) }
			}
		}
	}

	// Modo 2: dir vazio (fallback ML-1D).
	cacheDir, err := os.MkdirTemp("", "pwsh-jit-*")
	if err != nil {
		// Sem diretório isolado: retorna env original sem modificação.
		// É melhor aceitar o cache compartilhado do que impedir a chamada ao pwsh.
		return env, func() {}
	}
	return applyXDGCacheHome(env, cacheDir), func() { _ = os.RemoveAll(cacheDir) }
}
