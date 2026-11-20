package generators

// exec_stub_write_test.go — REQ-2026-11-20 ML-1B: helper único para gravar stubs executáveis
// sob syscall.ForkLock.RLock(), eliminando a janela de ETXTBSY em testes paralelos.
//
// Mecanismo (golang/go#22315 / Wave 0 §1.2):
//   - os.WriteFile não adquire ForkLock; o fd de escrita fica aberto durante a chamada.
//   - forkExec adquire ForkLock.Lock() e faz fork(); o filho herda todos os fds abertos,
//     incluindo o fd de escrita do arquivo que outro goroutine ainda está escrevendo.
//   - Quando esse goroutine termina a escrita e um outro processo tenta execve() do mesmo
//     arquivo, o kernel recusa: ETXTBSY (Text file busy) — há um fd de escrita aberto
//     (o herdado pelo filho do fork anterior).
//   - Envolvendo os.WriteFile em ForkLock.RLock()/defer RUnlock(), o fork nunca ocorre
//     enquanto o fd de escrita está aberto: Lock() (fork) bloqueia enquanto qualquer RLock
//     estiver ativo. Vários RLocks coexistem (paralelismo máximo preservado).

import (
	"io/fs"
	"os"
	"syscall"
)

// writeExecutableForTest grava content em path com perm, envolvendo os.WriteFile em
// syscall.ForkLock.RLock() / defer RUnlock().
//
// Regras de uso:
//  1. O t.Fatalf pertence ao chamador, fora desta função — dentro da zona travada não pode
//     haver t.Fatalf (runtime.Goexit() não executaria código depois do defer, mas o defer
//     RUnlock() seria executado; ainda assim, manter o t.Fatalf fora é mais legível e seguro).
//  2. Nada dentro desta função adquire ForkLock de forma recursiva (nenhum exec.Command).
//  3. A zona travada cobre apenas os.WriteFile — não inclui o exec.Command subsequente.
//
// Compilação: syscall.ForkLock existe em Linux, macOS e Windows (exec_unix.go /
// exec_bsd.go / exec_windows.go). Em Windows, esta proteção é no-op para ETXTBSY
// (mecanismo diferente), mas compila sem alteração de build tag.
func writeExecutableForTest(path string, content []byte, perm fs.FileMode) error {
	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()
	return os.WriteFile(path, content, perm)
}
