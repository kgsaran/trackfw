package serve

// symlink_helper_test.go — guarda de privilégio de symlink para este pacote.
//
// Esta é uma cópia intencional do mesmo helper definido em
// internal/generators/update_test.go, internal/discover/symlink_helper_test.go
// e internal/integrations/manager_test.go.
// Justificativa: símbolos de arquivo _test.go não são importáveis entre
// pacotes Go — não existe "import de helpers de teste" no Go. Criar um pacote
// compartilhado internal/testutil exigiria um arquivo não-_test.go (produção),
// o que está fora do escopo deste ML. Uma cópia por fronteira de pacote é o
// idioma correto aqui; a lógica é idêntica nos demais locais — qualquer
// divergência é um bug.

import (
	"errors"
	"os"
	"syscall"
	"testing"
)

// symlinkOrSkip cria um symlink em link apontando para target. Se a criação
// falhar por falta do privilégio que o Windows exige (Developer Mode ou
// processo elevado — WinError 1314, ERROR_PRIVILEGE_NOT_HELD), pula o teste
// chamador nomeando a garantia não exercitada e devolve false. Qualquer
// outro erro é um t.Fatalf.
//
// A detecção é pela CONDIÇÃO (falha de privilégio), não por runtime.GOOS:
// num Windows com Developer Mode habilitado, ou em Linux/macOS, os.Symlink
// tem sucesso e o teste executa normalmente.
func symlinkOrSkip(t *testing.T, target, link string) bool {
	t.Helper()
	err := os.Symlink(target, link)
	if err == nil {
		return true
	}
	if isSymlinkPrivilegeError(err) {
		t.Skipf(
			"guarda de symlink não exercitada: criação de symlink exige "+
				"Developer Mode (ou processo elevado) neste Windows: %v", err,
		)
		return false
	}
	t.Fatalf("os.Symlink(%q, %q): %v", target, link, err)
	return false
}

// isSymlinkPrivilegeError reporta se err é a falha "processo sem privilégio
// para criar symlink" — WinError 1314 no Windows sem Developer
// Mode/elevação, ou permission-denied genérico em qualquer plataforma. Não
// casa por GOOS/plataforma, só pelo erro subjacente.
func isSymlinkPrivilegeError(err error) bool {
	if os.IsPermission(err) {
		return true
	}
	var e syscall.Errno
	if errors.As(err, &e) && e == 1314 {
		return true
	}
	return false
}
