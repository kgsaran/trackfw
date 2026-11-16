// Package winskips implements an AST-based gate that enforces every
// Windows-conditional test exit (t.Skip*, return in a GOOS branch) is
// documented in .github/windows-skips.json.
//
// Key format: "relative/path/to/file.go:TestFuncName"
// (forward slashes, relative to the module root)
package winskips

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Entry is one record in .github/windows-skips.json.
type Entry struct {
	Test           string `json:"test"`
	Classification string `json:"classification"` // "a", "b", or "c"
	Reason         string `json:"reason"`
	Evidence       string `json:"evidence"`
}

// ViolKind classifies a gate violation.
type ViolKind string

const (
	// ViolMissing: function is marked but has no registry entry.
	ViolMissing ViolKind = "missing"
	// ViolObsolete: registry entry has no matching function.
	ViolObsolete ViolKind = "obsolete"
	// ViolNoEvidence: entry has an empty evidence field.
	ViolNoEvidence ViolKind = "no_evidence"
)

// Violation is one gate failure.
type Violation struct {
	Kind ViolKind
	Key  string
	Msg  string
}

// LoadRegistry reads .github/windows-skips.json from moduleRoot.
func LoadRegistry(moduleRoot string) ([]Entry, error) {
	p := filepath.Join(moduleRoot, ".github", "windows-skips.json")
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var entries []Entry
	return entries, json.Unmarshal(data, &entries)
}

// ModuleRoot walks up from start to find the directory containing go.mod.
func ModuleRoot(start string) (string, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	dir := abs
	for {
		if _, e := os.Stat(filepath.Join(dir, "go.mod")); e == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", start)
		}
		dir = parent
	}
}

// Check traverses the module tree at moduleRoot, finds all TestXxx functions
// that require a registry entry (marked), and validates against reg.
func Check(moduleRoot string, reg []Entry) ([]Violation, error) {
	marked, err := findMarked(moduleRoot)
	if err != nil {
		return nil, err
	}

	markedSet := make(map[string]bool, len(marked))
	for _, k := range marked {
		markedSet[k] = true
	}
	regByKey := make(map[string]Entry, len(reg))
	for _, e := range reg {
		regByKey[e.Test] = e
	}

	var v []Violation

	// 1. Every marked function must have a registry entry.
	sortedMarked := make([]string, len(marked))
	copy(sortedMarked, marked)
	sort.Strings(sortedMarked)
	for _, k := range sortedMarked {
		if _, ok := regByKey[k]; !ok {
			v = append(v, Violation{ViolMissing, k, "marked test has no registry entry"})
		}
	}

	// 2. Every entry must have non-empty evidence.
	for _, e := range reg {
		if strings.TrimSpace(e.Evidence) == "" {
			v = append(v, Violation{ViolNoEvidence, e.Test, "registry entry has empty evidence"})
		}
	}

	// 3. Every entry must correspond to a marked function (not obsolete).
	for _, e := range reg {
		if !markedSet[e.Test] {
			v = append(v, Violation{ViolObsolete, e.Test, "registry entry has no matching marked function"})
		}
	}

	return v, nil
}

// ── directories excluded from the walk ───────────────────────────────────────

var excludedDirs = map[string]bool{
	"vendor":       true,
	"testdata":     true,
	"node_modules": true,
}

// selfPkg is excluded so the gate doesn't analyse its own fixture files.
const selfPkg = "internal/winskips"

// ── findMarked ────────────────────────────────────────────────────────────────

// findMarked returns all keys for TestXxx functions that require a registry
// entry. A test is marked when it has an exit (t.Skip*/return) that occurs
// specifically when runtime.GOOS IS "windows". Keys use forward slashes and
// are relative to moduleRoot.
func findMarked(moduleRoot string) ([]string, error) {
	// Group test files by their containing directory.
	dirFiles := make(map[string][]string)

	err := filepath.WalkDir(moduleRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			rel, _ := filepath.Rel(moduleRoot, path)
			rel = filepath.ToSlash(rel)
			if excludedDirs[d.Name()] {
				return filepath.SkipDir
			}
			if rel == selfPkg || strings.HasPrefix(rel, selfPkg+"/") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), "_test.go") {
			dir := filepath.Dir(path)
			dirFiles[dir] = append(dirFiles[dir], path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	var keys []string
	for dir, files := range dirFiles {
		dk, err := analyzeDir(moduleRoot, dir, files)
		if err != nil {
			return nil, fmt.Errorf("analyzeDir %s: %w", dir, err)
		}
		keys = append(keys, dk...)
	}
	return keys, nil
}

// ── analyzeDir ────────────────────────────────────────────────────────────────

// analyzeDir analyses all test files in one directory and returns the keys
// of marked TestXxx functions.
func analyzeDir(moduleRoot, dir string, files []string) ([]string, error) {
	fset := token.NewFileSet()
	parsed := make([]*ast.File, 0, len(files))
	parsedFiles := make([]string, 0, len(files))

	for _, f := range files {
		af, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			continue // ignore files that fail to parse
		}
		parsed = append(parsed, af)
		parsedFiles = append(parsedFiles, f)
	}

	// First pass: collect skip-helpers (non-TestXxx functions with GOOS+Skip).
	// A skip-helper is any function whose name does not start with "Test" that
	// has a Windows-conditional branch containing t.Skip/Skipf/SkipNow that
	// EXECUTES WHEN GOOS IS "windows".
	skipHelpers := make(map[string]bool)
	for _, af := range parsed {
		for _, decl := range af.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name == nil || fn.Body == nil {
				continue
			}
			if isTestFunc(fn) {
				continue
			}
			if !hasTParam(fn) {
				continue
			}
			if nodeHasPlatformExit(fn.Body, nil, true /*skipOnly*/) {
				skipHelpers[fn.Name.Name] = true
			}
		}
	}

	// Transitively expand: a helper that calls another skip-helper is also a
	// skip-helper. Iterate until stable.
	for changed := true; changed; {
		changed = false
		for i, af := range parsed {
			_ = parsedFiles[i]
			for _, decl := range af.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Name == nil || fn.Body == nil {
					continue
				}
				if isTestFunc(fn) || skipHelpers[fn.Name.Name] {
					continue
				}
				if !hasTParam(fn) {
					continue
				}
				if nodeHasHelperCall(fn.Body, skipHelpers) {
					skipHelpers[fn.Name.Name] = true
					changed = true
				}
			}
		}
	}

	// Second pass: find marked TestXxx functions.
	var keys []string
	for i, af := range parsed {
		fpath := parsedFiles[i]
		rel, _ := filepath.Rel(moduleRoot, fpath)
		rel = filepath.ToSlash(rel)

		// Determine if this file is platform-specific.
		winOnly, nonWinOnly, _ := isPlatformFile(dir, filepath.Base(fpath))

		// Only mark nonWinOnly files: they do not compile on Windows, which
		// means every test in them is effectively skipped on Windows.
		// winOnly files (//go:build windows) run ONLY on Windows — they are
		// not Windows skips and must not be marked.
		if nonWinOnly {
			for _, decl := range af.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Name == nil || fn.Body == nil {
					continue
				}
				if !isTestFunc(fn) {
					continue
				}
				keys = append(keys, rel+":"+fn.Name.Name)
			}
			continue
		}
		if winOnly {
			// Windows-only file: tests run on Windows, skip on other platforms.
			// These are not Windows skips; do not mark.
			continue
		}

		// Regular file: check for GOOS platform branches that exit on Windows.
		for _, decl := range af.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name == nil || fn.Body == nil {
				continue
			}
			if !isTestFunc(fn) {
				continue
			}
			name := fn.Name.Name
			if nodeHasPlatformExit(fn.Body, skipHelpers, false) {
				keys = append(keys, rel+":"+name)
			}
		}
	}

	return keys, nil
}

// ── three-valued GOOS condition evaluation ───────────────────────────────────

// condResult is the three-valued result of evaluating a GOOS expression
// specifically when runtime.GOOS is "windows".
type condResult int8

const (
	condFalse   condResult = -1 // definitely false when GOOS is "windows"
	condUnknown condResult = 0  // value depends on other factors at runtime
	condTrue    condResult = 1  // definitely true when GOOS is "windows"
)

// hasGOOSComparison reports whether expr contains a runtime.GOOS comparison
// to the string literal "windows" anywhere in the expression tree.
// Used as a fast pre-filter: if false, the condition is unrelated to Windows
// and should not be passed to evalOnWindows.
func hasGOOSComparison(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.BinaryExpr:
		switch e.Op {
		case token.EQL, token.NEQ:
			return (isGOOSSelector(e.X) && isWindowsLit(e.Y)) ||
				(isGOOSSelector(e.Y) && isWindowsLit(e.X))
		case token.LAND, token.LOR:
			return hasGOOSComparison(e.X) || hasGOOSComparison(e.Y)
		}
	case *ast.UnaryExpr:
		if e.Op == token.NOT {
			return hasGOOSComparison(e.X)
		}
	case *ast.ParenExpr:
		return hasGOOSComparison(e.X)
	}
	return false
}

// evalOnWindows evaluates whether expr is true when runtime.GOOS == "windows".
// Uses Kleene K3 strong three-valued logic for compound expressions (&&, ||, !).
// Only runtime.GOOS comparisons are evaluated; os.Getenv("GOOS") is not a
// valid runtime signal (the env var is never set by go test) and is ignored.
func evalOnWindows(expr ast.Expr) condResult {
	switch e := expr.(type) {
	case *ast.BinaryExpr:
		switch e.Op {
		case token.EQL:
			if (isGOOSSelector(e.X) && isWindowsLit(e.Y)) ||
				(isGOOSSelector(e.Y) && isWindowsLit(e.X)) {
				return condTrue
			}
		case token.NEQ:
			if (isGOOSSelector(e.X) && isWindowsLit(e.Y)) ||
				(isGOOSSelector(e.Y) && isWindowsLit(e.X)) {
				return condFalse
			}
		case token.LAND:
			return kleeneAnd(evalOnWindows(e.X), evalOnWindows(e.Y))
		case token.LOR:
			return kleeneOr(evalOnWindows(e.X), evalOnWindows(e.Y))
		}
	case *ast.UnaryExpr:
		if e.Op == token.NOT {
			return kleeneNot(evalOnWindows(e.X))
		}
	case *ast.ParenExpr:
		return evalOnWindows(e.X)
	}
	return condUnknown
}

// Kleene K3 strong three-valued logic operators.

func kleeneAnd(a, b condResult) condResult {
	if a == condFalse || b == condFalse {
		return condFalse
	}
	if a == condTrue && b == condTrue {
		return condTrue
	}
	return condUnknown
}

func kleeneOr(a, b condResult) condResult {
	if a == condTrue || b == condTrue {
		return condTrue
	}
	if a == condFalse && b == condFalse {
		return condFalse
	}
	return condUnknown
}

func kleeneNot(a condResult) condResult {
	switch a {
	case condTrue:
		return condFalse
	case condFalse:
		return condTrue
	}
	return condUnknown
}

// ── AST helpers ──────────────────────────────────────────────────────────────

// isTestFunc returns true when fn looks like func TestXxx(t *testing.T).
func isTestFunc(fn *ast.FuncDecl) bool {
	if fn.Name == nil || !strings.HasPrefix(fn.Name.Name, "Test") {
		return false
	}
	if fn.Type.Params == nil || len(fn.Type.Params.List) != 1 {
		return false
	}
	p := fn.Type.Params.List[0]
	star, ok := p.Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	return ok && ident.Name == "testing" && sel.Sel.Name == "T"
}

// hasTParam returns true if fn has at least one *testing.T parameter.
func hasTParam(fn *ast.FuncDecl) bool {
	if fn.Type.Params == nil {
		return false
	}
	for _, f := range fn.Type.Params.List {
		star, ok := f.Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		sel, ok := star.X.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		ident, ok := sel.X.(*ast.Ident)
		if ok && ident.Name == "testing" && sel.Sel.Name == "T" {
			return true
		}
	}
	return false
}

// nodeHasPlatformExit reports whether body contains a GOOS platform branch
// whose exit (t.Skip* or return) occurs specifically when GOOS IS "windows".
// It descends into FuncLit nodes to find branches inside t.Run closures.
// When skipOnly is true, only t.Skip* counts; return does not.
//
// Polarity rules (Kleene three-valued logic on the condition):
//   - condTrue  → exit in then-branch executes on Windows → marked.
//   - condFalse → exit in then-branch executes on non-Windows → not marked;
//     exit in else-branch executes on Windows → marked.
//   - condUnknown → both branches may execute on Windows → both checked.
//
// Switch statements: case "windows" exits are always checked; the default
// case is checked only when no case "windows" exists in the switch.
func nodeHasPlatformExit(body ast.Node, skipHelpers map[string]bool, skipOnly bool) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found || n == nil {
			return false
		}
		switch stmt := n.(type) {
		case *ast.IfStmt:
			// Only process conditions that actually involve a runtime.GOOS
			// comparison to "windows". Conditions like `err != nil` evaluate to
			// condUnknown but are not GOOS-related and must be ignored.
			if !hasGOOSComparison(stmt.Cond) {
				break
			}
			r := evalOnWindows(stmt.Cond)
			// Then-branch executes when condition is true.
			// Check it unless condition is definitely false on Windows.
			if r != condFalse && branchHasExit(stmt.Body, skipOnly) {
				found = true
				return false
			}
			// Else-branch executes when condition is false.
			// Check it unless condition is definitely true on Windows.
			if r != condTrue && stmt.Else != nil && branchHasExit(stmt.Else, skipOnly) {
				found = true
				return false
			}
		case *ast.SwitchStmt:
			// switch runtime.GOOS { case "windows": ... }
			if stmt.Tag == nil || !isGOOSSelector(stmt.Tag) {
				break
			}
			hasWindowsCase := false
			for _, cl := range stmt.Body.List {
				cc, ok := cl.(*ast.CaseClause)
				if !ok {
					continue
				}
				for _, v := range cc.List {
					if isWindowsLit(v) {
						hasWindowsCase = true
						block := &ast.BlockStmt{List: cc.Body}
						if branchHasExit(block, skipOnly) {
							found = true
							return false
						}
					}
				}
			}
			// If there is no case "windows", the default case executes on
			// Windows. Check it.
			if !hasWindowsCase {
				for _, cl := range stmt.Body.List {
					cc, ok := cl.(*ast.CaseClause)
					if !ok || len(cc.List) != 0 { // len == 0 means default
						continue
					}
					block := &ast.BlockStmt{List: cc.Body}
					if branchHasExit(block, skipOnly) {
						found = true
						return false
					}
				}
			}
		case *ast.CallExpr:
			// Any call to a skip-helper marks the enclosing test.
			if skipHelpers != nil {
				if ident, ok := stmt.Fun.(*ast.Ident); ok {
					if skipHelpers[ident.Name] {
						found = true
						return false
					}
				}
			}
		}
		return true
	})
	return found
}

// nodeHasHelperCall reports whether body contains a call to any name in calls.
func nodeHasHelperCall(body ast.Node, calls map[string]bool) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found || n == nil {
			return false
		}
		if call, ok := n.(*ast.CallExpr); ok {
			if ident, ok := call.Fun.(*ast.Ident); ok {
				if calls[ident.Name] {
					found = true
				}
			}
		}
		return !found
	})
	return found
}

// branchHasExit reports whether a branch body (IfStmt.Body, Else, or a
// CaseClause block) contains a qualifying exit WITHOUT crossing FuncLit
// boundaries. When skipOnly is true, only t.Skip* counts; return does not.
func branchHasExit(node ast.Node, skipOnly bool) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if found || n == nil {
			return false
		}
		// Stop at nested function literals (closures), but not the starting node.
		if fl, ok := n.(*ast.FuncLit); ok && fl != node {
			return false
		}
		switch stmt := n.(type) {
		case *ast.ReturnStmt:
			if !skipOnly {
				found = true
			}
		case *ast.ExprStmt:
			if isSkipCall(stmt.X) {
				found = true
			}
		}
		return !found
	})
	return found
}

// isSkipCall returns true if expr is a call to t.Skip, t.Skipf, or t.SkipNow.
func isSkipCall(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	switch sel.Sel.Name {
	case "Skip", "Skipf", "SkipNow":
		return true
	}
	return false
}

// isGOOSSelector returns true if expr is exactly runtime.GOOS.
func isGOOSSelector(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	return ok && ident.Name == "runtime" && sel.Sel.Name == "GOOS"
}

// isWindowsLit returns true if expr is the string literal "windows".
func isWindowsLit(expr ast.Expr) bool {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return false
	}
	return strings.Trim(lit.Value, `"`+"`") == "windows"
}

// ── isPlatformFile ────────────────────────────────────────────────────────────

// isPlatformFile returns whether file name in dir is included for windows but
// not linux/darwin (winOnly) or for linux/darwin but not windows (nonWinOnly).
// It uses go/build.Context.MatchFile, which respects both //go:build tags and
// filename suffixes (_windows_test.go, _linux_test.go, etc.).
func isPlatformFile(dir, name string) (winOnly, nonWinOnly bool, err error) {
	check := func(goos string) (bool, error) {
		ctx := build.Default
		ctx.GOOS = goos
		return ctx.MatchFile(dir, name)
	}
	matchWin, err := check("windows")
	if err != nil {
		return
	}
	matchLin, err := check("linux")
	if err != nil {
		return
	}
	matchDar, err := check("darwin")
	if err != nil {
		return
	}
	winOnly = matchWin && !matchLin && !matchDar
	nonWinOnly = !matchWin && (matchLin || matchDar)
	return
}
