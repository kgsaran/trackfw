package pathguard_test

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/kgsaran/trackfw/internal/pathguard"
)

// TestBeneath_ContainedPath — affirms ML-1A extraction conclusion (i): Beneath has the same
// semantics as the private beneath in internal/integrations/manager.go; a path strictly inside
// root is reported as contained.
func TestBeneath_ContainedPath(t *testing.T) {
	root := "/project"
	cases := []string{
		"/project/file.txt",
		"/project/sub/dir/file.txt",
		"/project/a",
	}
	for _, c := range cases {
		if !pathguard.Beneath(root, c) {
			t.Errorf("Beneath(%q, %q) = false, want true", root, c)
		}
	}
}

// TestBeneath_NotContained — affirms ML-1A extraction conclusion (i): Beneath correctly rejects
// paths that are equal to root, escape via "..", or are absolute siblings — matching the
// production behaviour already exercised by internal/integrations tests.
func TestBeneath_NotContained(t *testing.T) {
	root := "/project"
	cases := []struct {
		filename string
		desc     string
	}{
		{"/project", "path equal to root"},
		{"/project/../other", "path escaping via .."},
		{"/other/file.txt", "absolute sibling"},
		{"/", "filesystem root"},
	}
	for _, c := range cases {
		if pathguard.Beneath(root, filepath.Clean(c.filename)) {
			t.Errorf("Beneath(%q, %q) = true, want false (%s)", root, c.filename, c.desc)
		}
	}
}

// TestRejectSymlinks_CleanPath — affirms ML-1A extraction conclusion (i): a plain path with no
// symlinks anywhere on the walk returns nil, preserving existing behaviour that legitimate writes
// are not refused (the "braço (b)" of the ADR, falsification direction 2).
func TestRejectSymlinks_CleanPath(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "sub", "new-file.md")
	if err := pathguard.RejectSymlinks(root, file); err != nil {
		t.Errorf("RejectSymlinks clean path: unexpected error %v", err)
	}
}

// TestRejectSymlinks_SymlinkInLeaf — affirms ML-1A extraction conclusion (i): a symlink at the
// leaf of the path is detected and rejected, reproducing the pre-existing class (c) protection
// in internal/integrations/manager.go for symlinks in catalog destination files.
func TestRejectSymlinks_SymlinkInLeaf(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "victim.md")
	if err := os.WriteFile(target, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	leaf := filepath.Join(root, "ROADMAP-decoy.md")
	if !symlinkOrSkip(t, target, leaf) {
		return
	}
	err := pathguard.RejectSymlinks(root, leaf)
	if err == nil {
		t.Error("RejectSymlinks with symlink leaf: expected error, got nil")
	}
}

// TestRejectSymlinks_SymlinkInAncestor — affirms ML-1A extraction conclusion (ii): the predicate
// walks ALL ancestors up to root, not just the leaf. This is the exact vulnerability reproduced
// against the main-branch binary on 2026-09-18 (mkdir -p /fora/out && ln -s /fora/out scripts &&
// trackfw discover --init wrote 6 files outside the tree). The leaf does not exist yet; the
// symlink is on an ancestor directory.
func TestRejectSymlinks_SymlinkInAncestor(t *testing.T) {
	root := t.TempDir()
	// Create a real directory outside root to be the symlink target.
	outside := t.TempDir()
	// Place the symlink on an intermediate directory inside root.
	ancestor := filepath.Join(root, "scripts")
	if !symlinkOrSkip(t, outside, ancestor) {
		return
	}
	// The leaf does not exist — simulates creating a new file.
	leaf := filepath.Join(ancestor, "trackfw-validate.sh")
	err := pathguard.RejectSymlinks(root, leaf)
	if err == nil {
		t.Error("RejectSymlinks with symlink ancestor: expected error, got nil (ancestor symlink not detected)")
	}
}

// TestRejectSymlinks_ExistingFile — affirms ML-1A extraction conclusion (i): an existing regular
// file with no symlinks anywhere on its path is accepted (braço (b), operation continues working).
func TestRejectSymlinks_ExistingFile(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "roadmaps")
	if err := os.MkdirAll(dir, 0750); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "ROADMAP-real.md")
	if err := os.WriteFile(file, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := pathguard.RejectSymlinks(root, file); err != nil {
		t.Errorf("RejectSymlinks existing regular file: unexpected error %v", err)
	}
}

// TestRejectSymlinks_PathEscapesRoot — affirms ML-1A extraction conclusion (i): a path that
// resolves outside root is rejected as "escapes root", matching the escape-via-.. protection
// already in production in class (c).
func TestRejectSymlinks_PathEscapesRoot(t *testing.T) {
	root := t.TempDir()
	// Construct a path that starts inside root but uses filepath.Dir() enough times
	// to climb out. We do this by passing an absolute path that is a sibling of root.
	sibling := filepath.Join(filepath.Dir(root), "sibling", "file.txt")
	err := pathguard.RejectSymlinks(root, sibling)
	if err == nil {
		t.Error("RejectSymlinks with path outside root: expected error, got nil")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// symlink_helper — guarda de privilégio de symlink para este pacote.
//
// Copiado de internal/validator/symlink_helper_test.go (versão referência).
// Justificativa: símbolos de arquivo _test.go não são importáveis entre
// pacotes Go — não existe "import de helpers de teste" no Go. Criar um pacote
// compartilhado internal/testutil exigiria um arquivo não-_test.go (produção),
// o que está fora do escopo deste ML. Uma cópia por fronteira de pacote é o
// idioma correto; a lógica deve ser idêntica — qualquer divergência é um bug.
// ──────────────────────────────────────────────────────────────────────────────

// symlinkOrSkip cria um symlink em link apontando para target. Se a criação
// falhar por falta do privilégio que o Windows exige (Developer Mode ou
// processo elevado — WinError 1314, ERROR_PRIVILEGE_NOT_HELD), pula o teste
// chamador nomeando a garantia não exercitada e devolve false. Qualquer
// outro erro é um t.Fatalf.
//
// A detecção é pela CONDIÇÃO (falha de privilégio), não por runtime.GOOS:
// num Windows com Developer Mode habilitado, ou em Linux/macOS, os.Symlink
// tem sucesso e o teste executa normalmente. Isso corrige o antipadrão
// runtime.GOOS == "windows" → t.Skip, que abandona cobertura mesmo em
// Windows com Developer Mode.
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
