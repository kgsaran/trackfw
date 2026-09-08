// Package pathanchor classifies whether a string DENOTES an anchored (root-independent)
// filesystem location, using a definition that is invariant across host operating systems.
//
// It answers a narrower question than "is this an absolute path for the OS this process runs
// on" (that question is `path/filepath`'s job, via filepath.IsAbs — see the boundary note
// below). It answers: "does this string, on its face, claim to be anchored to a filesystem
// root — POSIX '/', a Windows drive letter, or a Windows UNC share — regardless of which OS
// is asking?"
//
// # Why filepath.IsAbs is the wrong tool for this question
//
// `filepath.IsAbs("/opt/foo")` is `false` on Windows — Go's definition of "absolute" for that
// GOOS requires a drive letter or UNC prefix. That is correct for filepath's own job (deciding
// how a *this-host* syscall will resolve a path), but it is the wrong verdict whenever the
// question being asked is "should this string bypass a root-relative join", because the answer
// to that question must not silently flip depending on which OS runs the check. Two sites in
// this codebase need exactly that invariant answer:
//
//  1. internal/validator classifies a hook-command path read from an agent CLI's config and
//     interpreted by bash (or the agent CLI itself), never by the Go process's own filesystem
//     calls — see ADR-2026-09-04-caminho-posix-ancorado-num-config-lido-por-cli-de-agente-e-
//     absoluto-independente-do-so-host, decision D1.
//  2. internal/integrations.Manager.resolve classifies a PlannedArtifact.Destination string to
//     decide whether it should be accepted verbatim (then Clean()-ed and checked against the
//     scope root) or force-joined under the scope root as a relative fragment. That decision is
//     itself a security boundary — accepting a destination that LOOKS anchored under one OS's
//     rules but treating it as a safe relative fragment under another's is precisely how a
//     POSIX-absolute destination ("/tmp/x") slipped past the scope-root guard on Windows
//     (measured on Windows ARM64, ROADMAP-2026-09-03 Wave reaberta 2026-09-08, ML-R1 — see
//     TestManagerRejectsTraversalAbsoluteMismatchAndNUL in internal/integrations/manager_test.go
//     and TestManagerRejectsAnchoredDestinationHostMismatch there for the regression test).
//
// # The boundary this package does NOT cross
//
// This package never governs an actual filesystem traversal, syscall, or path-building step —
// `filepath.Clean`, `filepath.Join`, `filepath.Rel`, `os.Stat`/`os.Lstat`, and every other call
// that ends up asking the OS to resolve or open a path keep using `path/filepath` exactly as
// before. Once a caller has decided (via IsAnchored, possibly combined with filepath.IsAbs — see
// internal/integrations/manager.go's resolve()) that a string is anchored on THIS host too, all
// further path manipulation is `path/filepath`'s job, not this package's. Swapping this
// predicate into a real traversal site would risk breaking Windows path resolution
// intermittently — see ADR-2026-09-04 D2, which draws this same line (its enumeration of
// examples is corrected by this package's own doc comment at the two call sites above; D2's
// principle — classification vs. traversal — is what carries forward, not its stale example
// list).
package pathanchor

import "strings"

// IsAnchored reports whether raw is a string that denotes an anchored (root-independent)
// filesystem location, using a definition that is the SAME regardless of which OS the calling
// process runs on:
//
//   - POSIX: a leading "/" — anchored on any host, including a Go process running on Windows.
//   - Windows drive letter: "C:\..." or "C:/..." — anchored on any host, including a Go process
//     running on Linux or macOS.
//   - Windows UNC: "\\server\share\..." with a non-empty SERVER segment (not "." or "..") and a
//     non-empty SHARE segment that does not itself start with another backslash — anchored on
//     any host. "\\", "\\x" (no share segment), "\\.\x" / "\\..\evil" (server "." or "..") are
//     NOT valid UNC and fall through as NOT anchored.
//
// 🔴 ZERO calls into path/filepath here, and none dependent on GOOS/runtime.GOOS. That is not an
// implementation detail — it is the entire reason this function exists (see the package doc).
// Any call into filepath.IsAbs, filepath.Separator, or any other filepath predicate that varies
// by build/runtime target reintroduces the exact host-dependence this predicate exists to
// remove, and is verifiable by grep: this file must never import "path/filepath".
func IsAnchored(raw string) bool {
	if raw == "" {
		return false
	}
	if raw[0] == '/' {
		return true
	}
	// UNC: \\servidor\share\... — exige um segmento de SERVIDOR não vazio e diferente de "." ou
	// ".." (não são hostname válido), seguido de um separador, seguido de um segmento de SHARE não
	// vazio que não comece com outra barra invertida (evita componente vazio quando há barra dupla
	// no meio). "\\" e "\\x" sozinhos (sem separador de share), "\\.\x" / "\\..\evil" (server "."
	// ou ".."), e "\\..\\evil" (barra dupla no meio produz share vazio) NÃO são UNC válido — são
	// POSIX cwd-dependent (barra invertida não é separador em POSIX). A forma POSIX equivalente,
	// "//servidor/share", já é coberta pelo braço raw[0]=='/' acima — este braço cobre só a forma
	// com barra invertida.
	if len(raw) >= 2 && raw[0] == '\\' && raw[1] == '\\' {
		server, share, found := strings.Cut(raw[2:], `\`)
		if found && server != "" && server != "." && server != ".." && share != "" && share[0] != '\\' {
			return true
		}
	}
	// Letra de unidade: C:\... ou C:/...
	if len(raw) >= 3 && isASCIIDriveLetter(raw[0]) && raw[1] == ':' && (raw[2] == '\\' || raw[2] == '/') {
		return true
	}
	return false
}

// isASCIIDriveLetter reporta se b é uma letra ASCII (a-z, A-Z) — reconhecimento de letra de
// unidade do Windows em IsAnchored. Não usa unicode.IsLetter de propósito: uma letra de unidade
// do Windows é sempre ASCII, e a checagem de byte único evita qualquer dependência de locale.
func isASCIIDriveLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
