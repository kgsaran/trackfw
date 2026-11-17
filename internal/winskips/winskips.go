// Package winskips implements an AST-based gate that enforces every
// Windows-conditional test exit (t.Skip*, return in a GOOS branch) is
// documented in .github/windows-skips.json.
//
// Key format: "relative/path/to/file.go:TestFuncName"
// (forward slashes, relative to the module root)
//
// Recognised forms of Windows-conditional skip:
//
//	if runtime.GOOS == "windows" { t.Skip() }         (direct selector)
//	goos := runtime.GOOS; if goos == "windows" { … }  (local alias — 4-A2)
//	switch runtime.GOOS { case "windows": t.Skip() }  (tagged switch)
//	switch { case runtime.GOOS == "windows": t.Skip() } (tagless switch — 4-A1)
//	for { if runtime.GOOS == "windows" { continue } } (continue in loop — 4-A3)
//
// Limits of alias detection (4-A2): only direct assignments within the same
// function body are tracked (`x := runtime.GOOS`, `x = runtime.GOOS`,
// `var x = runtime.GOOS`). Shadowing, reassignment, and package-level
// variables are not tracked. os.Getenv("GOOS") is intentionally NOT
// recognised (ADR-2026-10-14 / ML-3A2 decision: that signal is dead).
//
// Limits of BranchStmt detection (4-A3): `continue` and `break` are counted
// when they appear in a Windows branch and !skipOnly. `goto` is not counted
// (would require CFG analysis). An unlabeled `break` whose target is an
// inner for/switch/select nested inside the Windows branch is a known
// false-positive risk; no such pattern exists in the tree as of 2026-11-17.
// `fallthrough` is never counted.
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
	"regexp"
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
	// ViolNoEvidence: entry has an empty or placeholder evidence field.
	ViolNoEvidence ViolKind = "no_evidence"
	// ViolBadClassification: entry has an invalid classification (not "a", "b", or "c").
	ViolBadClassification ViolKind = "bad_classification"
)

// Violation is one gate failure.
type Violation struct {
	Kind ViolKind
	Key  string
	Msg  string
}

// placeholderEvidence matches evidence values that are placeholder tokens.
// Matching is case-insensitive (caller uppercases before matching) and anchored:
// a string that merely CONTAINS one of these words as a substring is accepted.
var placeholderEvidence = regexp.MustCompile(`^(PENDING|TODO|TBD|FIXME)$`)

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

	// 2. Every entry must have non-empty, non-placeholder evidence.
	for _, e := range reg {
		trimmed := strings.TrimSpace(e.Evidence)
		if trimmed == "" {
			v = append(v, Violation{ViolNoEvidence, e.Test, "registry entry has empty evidence"})
			continue
		}
		// Reject placeholder tokens regardless of case. Anchored match so that
		// "PENDING até job 134276469418" (a real evidence with PENDING as prefix)
		// is NOT rejected.
		if placeholderEvidence.MatchString(strings.ToUpper(trimmed)) {
			v = append(v, Violation{ViolNoEvidence, e.Test, "registry entry has placeholder evidence: " + trimmed})
		}
	}

	// 3. Every entry must have a valid classification ("a", "b", or "c").
	for _, e := range reg {
		switch e.Classification {
		case "a", "b", "c":
			// valid
		default:
			v = append(v, Violation{ViolBadClassification, e.Test,
				fmt.Sprintf("registry entry has invalid classification %q (must be a, b, or c)", e.Classification)})
		}
	}

	// 4. Every entry must correspond to a marked function (not obsolete).
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
// entry. A test is marked when it has an exit (t.Skip*/return/continue) that
// occurs specifically when runtime.GOOS IS "windows". Keys use forward slashes
// and are relative to moduleRoot.
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
			goosVars := collectGOOSVars(fn.Body)
			if nodeHasPlatformExit(fn.Body, goosVars, nil, true /*skipOnly*/) {
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
			goosVars := collectGOOSVars(fn.Body)
			if nodeHasPlatformExit(fn.Body, goosVars, skipHelpers, false) {
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
// (or a local alias in goosVars) to the string literal "windows" anywhere in
// the expression tree.
// Used as a fast pre-filter: if false, the condition is unrelated to Windows
// and should not be passed to evalOnWindows.
func hasGOOSComparison(expr ast.Expr, goosVars map[string]bool) bool {
	switch e := expr.(type) {
	case *ast.BinaryExpr:
		switch e.Op {
		case token.EQL, token.NEQ:
			return (isGOOSExpr(e.X, goosVars) && isWindowsLit(e.Y)) ||
				(isGOOSExpr(e.Y, goosVars) && isWindowsLit(e.X))
		case token.LAND, token.LOR:
			return hasGOOSComparison(e.X, goosVars) || hasGOOSComparison(e.Y, goosVars)
		}
	case *ast.UnaryExpr:
		if e.Op == token.NOT {
			return hasGOOSComparison(e.X, goosVars)
		}
	case *ast.ParenExpr:
		return hasGOOSComparison(e.X, goosVars)
	}
	return false
}

// evalOnWindows evaluates whether expr is true when runtime.GOOS == "windows".
// Uses Kleene K3 strong three-valued logic for compound expressions (&&, ||, !).
// Only runtime.GOOS comparisons (and local aliases in goosVars) are evaluated;
// os.Getenv("GOOS") is not a valid runtime signal (the env var is never set by
// go test) and is ignored.
func evalOnWindows(expr ast.Expr, goosVars map[string]bool) condResult {
	switch e := expr.(type) {
	case *ast.BinaryExpr:
		switch e.Op {
		case token.EQL:
			if (isGOOSExpr(e.X, goosVars) && isWindowsLit(e.Y)) ||
				(isGOOSExpr(e.Y, goosVars) && isWindowsLit(e.X)) {
				return condTrue
			}
		case token.NEQ:
			if (isGOOSExpr(e.X, goosVars) && isWindowsLit(e.Y)) ||
				(isGOOSExpr(e.Y, goosVars) && isWindowsLit(e.X)) {
				return condFalse
			}
		case token.LAND:
			return kleeneAnd(evalOnWindows(e.X, goosVars), evalOnWindows(e.Y, goosVars))
		case token.LOR:
			return kleeneOr(evalOnWindows(e.X, goosVars), evalOnWindows(e.Y, goosVars))
		}
	case *ast.UnaryExpr:
		if e.Op == token.NOT {
			return kleeneNot(evalOnWindows(e.X, goosVars))
		}
	case *ast.ParenExpr:
		return evalOnWindows(e.X, goosVars)
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
// whose exit (t.Skip*, return, or loop control) occurs specifically when GOOS
// IS "windows". It descends into FuncLit nodes to find branches inside t.Run
// closures. When skipOnly is true, only t.Skip* counts; return and continue do
// not.
//
// goosVars is the set of local variable names assigned from runtime.GOOS in
// the enclosing function body (see collectGOOSVars). Pass nil if none.
//
// Polarity rules (Kleene three-valued logic on the condition):
//   - condTrue  → exit in then-branch executes on Windows → marked.
//   - condFalse → exit in then-branch executes on non-Windows → not marked;
//     exit in else-branch executes on Windows → marked.
//   - condUnknown → both branches may execute on Windows → both checked.
//
// Switch statements:
//   - Tagged switch (switch runtime.GOOS { … }): case "windows" exits are
//     always checked; the default case is checked only when no case "windows"
//     exists.
//   - Tagless switch (switch { case runtime.GOOS == "windows": … }): each
//     case expression is evaluated by evalOnWindows; a condTrue case is checked.
//     Default is checked only if no case was condTrue.
func nodeHasPlatformExit(body ast.Node, goosVars map[string]bool, skipHelpers map[string]bool, skipOnly bool) bool {
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
			if !hasGOOSComparison(stmt.Cond, goosVars) {
				break
			}
			r := evalOnWindows(stmt.Cond, goosVars)
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
			if stmt.Tag == nil {
				// Tagless switch: switch { case expr: … }
				// Each case item is a full boolean expression.
				// Recognised form: switch { case runtime.GOOS == "windows": t.Skip() }
				//
				// Walk cases top-to-bottom. A case whose expression evaluates to
				// condTrue on Windows is the case that executes on Windows; check it
				// for a qualifying exit. The default case is checked only when at
				// least one non-default case had a GOOS comparison AND no condTrue
				// case was found (same logic as the tagged-switch default).
				//
				// Guard: if NO case in the switch contains a GOOS comparison at all,
				// the switch is completely unrelated to Windows — do not check the
				// default. Without this guard, any tagless switch whose default
				// contains `return` or `continue` would be a false positive.
				hasCondTrueCase := false
				hasAnyGOOSCase := false
				for _, cl := range stmt.Body.List {
					cc, ok := cl.(*ast.CaseClause)
					if !ok {
						continue
					}
					if len(cc.List) == 0 {
						// default: handled below
						continue
					}
					// Combine multiple expressions in one case clause with kleeneOr
					// (e.g., `case a == "windows", b == "windows":`).
					var combinedResult condResult = condFalse
					hasWindowsExpr := false
					for _, caseExpr := range cc.List {
						if !hasGOOSComparison(caseExpr, goosVars) {
							continue
						}
						hasWindowsExpr = true
						combinedResult = kleeneOr(combinedResult, evalOnWindows(caseExpr, goosVars))
					}
					if !hasWindowsExpr {
						continue
					}
					hasAnyGOOSCase = true
					if combinedResult == condTrue {
						hasCondTrueCase = true
						block := &ast.BlockStmt{List: cc.Body}
						if branchHasExit(block, skipOnly) {
							found = true
							return false
						}
					}
				}
				// Check default only if the switch is GOOS-related AND no case
				// was condTrue on Windows.
				if hasAnyGOOSCase && !hasCondTrueCase {
					for _, cl := range stmt.Body.List {
						cc, ok := cl.(*ast.CaseClause)
						if !ok || len(cc.List) != 0 {
							continue
						}
						block := &ast.BlockStmt{List: cc.Body}
						if branchHasExit(block, skipOnly) {
							found = true
							return false
						}
					}
				}
				break
			}
			// Tagged switch: switch runtime.GOOS { case "windows": … }
			// Also recognises switch goos { … } when goos is a local alias.
			if !isGOOSExpr(stmt.Tag, goosVars) {
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
// boundaries. When skipOnly is true, only t.Skip* counts; return, continue and
// break do not.
//
// Qualifying exits:
//   - t.Skip, t.Skipf, t.SkipNow (always, independent of skipOnly)
//   - return (when !skipOnly)
//   - continue, break (when !skipOnly — these skip loop iterations or exit
//     the nearest enclosing for/switch/select, effectively bypassing assertions)
//
// Known limit: an unlabeled break whose target is an inner loop or switch
// nested inside the Windows branch is counted as an exit even though it only
// exits the inner construct, not the test. No such pattern exists in the tree
// as of 2026-11-17; a false positive from this would be surfaced by RealTree.
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
		case *ast.BranchStmt:
			// continue skips the remaining loop body on Windows.
			// break exits the nearest enclosing for/switch/select on Windows.
			// fallthrough is never counted (it falls through to the next case,
			// not out of the Windows branch).
			// goto is not counted (would require CFG analysis).
			if !skipOnly && (stmt.Tok == token.CONTINUE || stmt.Tok == token.BREAK) {
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

// isGOOSExpr returns true if expr is runtime.GOOS or a local variable that
// was assigned from runtime.GOOS (see collectGOOSVars).
func isGOOSExpr(expr ast.Expr, goosVars map[string]bool) bool {
	if isGOOSSelector(expr) {
		return true
	}
	if goosVars != nil {
		if ident, ok := expr.(*ast.Ident); ok {
			return goosVars[ident.Name]
		}
	}
	return false
}

// collectGOOSVars walks body and returns the set of local variable names that
// are directly assigned from runtime.GOOS. Covers short-variable declarations
// (`:=`), regular assignments (`=`), and var declarations (`var x = …`).
//
// Limits: matching is by name only, not by object identity. Shadowing and
// reassignment are not tracked. Package-level variables and imports are not
// covered. os.Getenv("GOOS") is intentionally excluded.
func collectGOOSVars(body *ast.BlockStmt) map[string]bool {
	vars := make(map[string]bool)
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		switch stmt := n.(type) {
		case *ast.AssignStmt:
			// x := runtime.GOOS  or  x = runtime.GOOS
			if stmt.Tok != token.DEFINE && stmt.Tok != token.ASSIGN {
				break
			}
			for i, rhs := range stmt.Rhs {
				if isGOOSSelector(rhs) && i < len(stmt.Lhs) {
					if ident, ok := stmt.Lhs[i].(*ast.Ident); ok && ident.Name != "_" {
						vars[ident.Name] = true
					}
				}
			}
		case *ast.DeclStmt:
			// var x = runtime.GOOS
			decl, ok := stmt.Decl.(*ast.GenDecl)
			if !ok {
				break
			}
			for _, spec := range decl.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, val := range vs.Values {
					if isGOOSSelector(val) && i < len(vs.Names) {
						if vs.Names[i].Name != "_" {
							vars[vs.Names[i].Name] = true
						}
					}
				}
			}
		}
		return true
	})
	return vars
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
