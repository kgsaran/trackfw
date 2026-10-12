package commands

// netexec_scan_test.go — T4 gate: AST-based scan for unauthorized direct exec.Command /
// exec.CommandContext calls that invoke network subcommands.
//
// Regra Dura de Reconciliação: this test is the implementation of T4 declared in
// docs/seguranca/2026-10-12-wave0-prazo-das-operacoes-de-rede.md, section 2.5. It fails if
// any production Go file in internal/ contains a direct exec.Command or exec.CommandContext
// call whose first (binary) argument is the literal "git" with a network subcommand literal
// ("fetch", "ls-remote", or "push"), or whose binary is the literal "gh", "glab", or "az" —
// unless the call is inside one of the explicitly authorized executor functions listed below.
//
// Authorized callers (exempt by relative file path + function name):
//
//   - "commands/netexec.go" : defaultGitExec
//     exec.Command("git", ...) for local ops; exec.CommandContext for network ops with deadline.
//   - "commands/netexec.go" : runNetworkCmd
//     exec.CommandContext(ctx, name, args...) — the shared non-git network executor.
//   - "commands/netexec.go" : applyHTTPSIdleDetection
//     exec.Command("git", "remote", "get-url", ...) and exec.Command("git", "config", ...) — local.
//   - "commands/ship.go" : defaultExecForgeCLI
//     exec.Command(name, args...) — interactive; name is a variable, not a literal.
//
// Grep is insufficient because it cannot distinguish string literals from variable names,
// cannot determine which function a call belongs to, and cannot reliably parse multi-line
// call expressions. An AST-based scan is required (T4, Wave 0 §2.5).
//
// Conclusion: if this test passes on a given commit, no production file in internal/ contains
// an unauthorized direct exec.Command/exec.CommandContext call with a network subcommand or
// forge CLI binary literal. Future additions of such calls will fail here before reaching CI.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// networkBinaryNetSubcmds maps git network subcommand literals that trigger the rule.
var networkBinaryNetSubcmds = map[string]bool{
	"fetch":     true,
	"ls-remote": true,
	"push":      true,
}

// networkForgeCLIs lists forge CLI binary literals that always trigger the rule
// (any call to these binaries via exec.Command must go through runNetworkCmd).
var networkForgeCLIs = map[string]bool{
	"gh":   true,
	"glab": true,
	"az":   true,
}

// exemptFunctions lists the authorized executor functions that are allowed to call
// exec.Command / exec.CommandContext directly. Key: "<relpath>:<funcname>", where relpath
// is relative to the internal/ directory. The list must be kept explicit and commented.
//
// If this test flags a false positive because a new authorized executor was added, add it
// here with a comment explaining why the exemption is legitimate.
var exemptFunctions = map[string]bool{
	// defaultGitExec: the single git executor for this package; local ops use exec.Command,
	// network ops use exec.CommandContext with a context that has a deadline (ADR D2).
	"commands/netexec.go:defaultGitExec": true,

	// runNetworkCmd: the shared non-git network executor (gh, glab, az, gh api, etc.).
	// Always wraps with context+deadline (ADR D2). Args are passed as variables, not literals.
	"commands/netexec.go:runNetworkCmd": true,

	// applyHTTPSIdleDetection: uses exec.Command("git", "remote", "get-url", ...) and
	// exec.Command("git", "config", ...) — both are local operations (no socket opened).
	// Exempt so the scan does not flag local-only git calls in this helper.
	"commands/netexec.go:applyHTTPSIdleDetection": true,

	// defaultExecForgeCLI: interactive executor; inherits stdin/stdout/stderr; the binary
	// is passed as a variable (name), not a literal — but exempted defensively to avoid
	// false positives if a literal is added for a one-forge fast-path in the future.
	"commands/ship.go:defaultExecForgeCLI": true,
}

// scanViolation records one unauthorized network exec call.
type scanViolation struct {
	relFile  string // path relative to internal/
	funcName string // enclosing FuncDecl, or "<package-level>"
	line     int
	detail   string // description of the violation
}

// TestT4_NoDirectNetworkExecCommand scans every production Go file in internal/ for
// unauthorized exec.Command / exec.CommandContext calls with network binary/subcommand literals.
//
// Conclusion: the test passes only when all direct exec.Command/exec.CommandContext calls with
// network subcommands or forge CLI literals in internal/ route through the authorized executor
// functions. A future developer who bypasses defaultGitExec or runNetworkCmd will see this test
// fail with the exact file, line and function that needs to be routed through the executor.
func TestT4_NoDirectNetworkExecCommand(t *testing.T) {
	// Locate the internal/ directory relative to this test file.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed: cannot locate source tree")
	}
	// This file is internal/commands/netexec_scan_test.go; internal/ is two levels up.
	internalDir := filepath.Join(filepath.Dir(thisFile), "..")
	internalDir, err := filepath.Abs(internalDir)
	if err != nil {
		t.Fatalf("cannot abs internal dir: %v", err)
	}

	fset := token.NewFileSet()
	var violations []scanViolation
	var totalFiles, totalCalls int

	err = filepath.Walk(internalDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip testdata directories.
			if info.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil // skip test files
		}

		// Compute path relative to internal/ for display and exemption lookup.
		relPath, relErr := filepath.Rel(internalDir, path)
		if relErr != nil {
			return relErr
		}
		// Normalize to forward slashes for consistent exemption keys.
		relPath = filepath.ToSlash(relPath)

		f, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			t.Errorf("parse error in %s: %v", relPath, parseErr)
			return nil
		}
		totalFiles++

		// Build a map from ast.Node pointer → enclosing FuncDecl name.
		// We walk each top-level FuncDecl and map every node within it.
		// Package-level var/init that contain calls map to "<package-level>".
		nodeFunc := buildNodeFuncMap(f)

		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			// Check whether this is an exec.Command or exec.CommandContext call.
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			if pkg.Name != "exec" {
				return true
			}

			var binaryArgIdx int
			switch sel.Sel.Name {
			case "Command":
				binaryArgIdx = 0
			case "CommandContext":
				binaryArgIdx = 1 // Args[0] is the context; Args[1] is the binary name.
			default:
				return true
			}

			totalCalls++

			if len(call.Args) <= binaryArgIdx {
				return true // not enough args; skip
			}

			binaryLit, ok := stringLiteral(call.Args[binaryArgIdx])
			if !ok {
				return true // binary is a variable, not a literal; no rule to check
			}

			funcName := nodeFunc[n]
			if funcName == "" {
				funcName = "<package-level>"
			}
			exemptKey := relPath + ":" + funcName

			isViolation := false
			detail := ""

			if binaryLit == "git" {
				// Check for network subcommand literals in remaining args.
				for _, arg := range call.Args[binaryArgIdx+1:] {
					subcmd, isLit := stringLiteral(arg)
					if !isLit {
						continue
					}
					if networkBinaryNetSubcmds[subcmd] {
						isViolation = true
						detail = `exec.Command*("git", ..., "` + subcmd + `", ...) — network subcommand`
						break
					}
				}
			} else if networkForgeCLIs[binaryLit] {
				isViolation = true
				detail = `exec.Command*("` + binaryLit + `", ...) — forge CLI binary`
			}

			if !isViolation {
				return true
			}
			if exemptFunctions[exemptKey] {
				return true // authorized executor
			}

			pos := fset.Position(call.Pos())
			violations = append(violations, scanViolation{
				relFile:  relPath,
				funcName: funcName,
				line:     pos.Line,
				detail:   detail,
			})

			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk internal/: %v", err)
	}

	// Green-with-zero guard: the scan must have seen files and calls.
	if totalFiles == 0 {
		t.Fatal("T4 scan: no Go files found in internal/ — check the path")
	}
	if totalCalls == 0 {
		t.Fatal("T4 scan: no exec.Command/exec.CommandContext calls found — authorized executors themselves must have been removed or the scan is broken")
	}

	if len(violations) == 0 {
		t.Logf("T4 scan: %d files, %d exec.Command* calls; no violations", totalFiles, totalCalls)
		return
	}

	t.Errorf("T4: %d unauthorized direct exec.Command/CommandContext call(s) with network binary/subcommand found:", len(violations))
	for _, v := range violations {
		t.Errorf("  %s:%d [func %s]: %s", v.relFile, v.line, v.funcName, v.detail)
	}
	t.Error("Route these calls through defaultGitExec (git) or runNetworkCmd (gh/glab/az) so they receive a network deadline (ADR D2, T4).")
}

// buildNodeFuncMap returns a map from *ast.CallExpr (as ast.Node) to the name of the
// enclosing top-level FuncDecl. Nodes outside any FuncDecl map to "".
// We walk each FuncDecl's body and record every call node within it.
func buildNodeFuncMap(f *ast.File) map[ast.Node]string {
	m := make(map[ast.Node]string)
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		name := fd.Name.Name
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			if n == nil {
				return false
			}
			if _, isCall := n.(*ast.CallExpr); isCall {
				m[n] = name
			}
			return true
		})
	}
	return m
}

// stringLiteral returns the unquoted string value of a basic string literal node,
// and ok=true. Returns ("", false) for any other expression type.
func stringLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return s, true
}
