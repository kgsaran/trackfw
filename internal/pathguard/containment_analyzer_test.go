package pathguard

// containment_analyzer_test.go — the AST analyser of ML-7C / AC6 of REQ-2026-08-31.
//
// # What replaces what
//
// scripts/check-write-containment.sh accepts a write site because a human wrote
// "// write-containment-allowed: <reason>" above it. The marker is an assertion
// of authorship; the gate treats it as proof. Issue #400 audited the 157 markers
// by hand and found 22 leaf gaps and 7 false markers — 34 defects in total — with
// the gate green over every one of them.
//
// This analyser does not read the marker at all. It parses the tree, builds an
// interprocedural fact table, and answers two questions per site:
//
//	P1  is this write dominated, in the same flow, by a containment guard that
//	    covers the very path being written?
//	P2  does the root handed to that guard have provenance in an approved
//	    resolver — with filepath.Clean NOT transparent?
//
// # Why both, and why the population is defined by taint
//
// A pure reachability analyser approves pathguard.RejectAndReport(filepath.Clean(cwd), p)
// because the flow does pass through pathguard. The defect is the ARGUMENT — that
// is P2, and it is the reason a P1-only instrument would have signed off on the
// live escape of ML-7A.
//
// 🔴 The population of P1 is NOT "every write primitive in internal/". Quantifying
// over all of them inherits the bash gate's 157 sites, most of which are fixed-path
// or temp-file writes that no containment rule governs — and an exception list with
// a hundred entries IS T3 (§3.3 of the threat model) realised, not defended against.
// A write is in population when its path expression has provenance reaching a
// user-supplied root (project root, $HOME, cwd, or a config-specified directory).
// That is the same fixpoint P2 needs, so it costs nothing extra, and it yields a
// population count that is neither 0 nor 157 — the honest number to pin against
// vacuity.
//
// # Declared limits — stated as scoping, not omission
//
//  1. 🔴 T5 (threat model §3.5) is NOT covered and cannot be: projectRoot()
//     (scaffold.go) and resolveRoot() (discover.go) fall back to the UNRESOLVED
//     path when filepath.EvalSymlinks fails. Provenance is static; that degradation
//     is dynamic. This analyser marks those functions as approved producers and
//     would never see it. Closing T5 needs the resolver to fail closed plus a
//     runtime test asserting the write is refused — neither is in this ML, and
//     nothing here may be read as covering it.
//  2. This analyser is NOT a superset of scripts/check-write-containment.sh. A
//     brand-new write whose path is a literal, with no marker, is caught by the
//     bash gate and is out of this analyser's population by construction. The two
//     instruments overlap, they do not nest — which is why the bash gate stays.
//  3. No go/types, no SSA (golang.org/x/tools is not a dependency of this module).
//     Resolution is by identifier within a parsed unit set. A same-named function
//     in two packages of the same unit set is conflated — conservative for the
//     corpus arm (more findings), and measured as zero collisions on the live tree.
//  4. Dominance is block-ancestry plus source position, not a real CFG. A guard
//     reached only through a goto, or one whose covering branch is chosen at
//     runtime, is not modelled.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ─────────────────────────────────────────────────────────────────────────────
// The vocabulary the analyser keys on. Every list below is pinned by a count in
// analyzerVocabularyFloors and asserted by TestAnalyserVocabularyIsPinned: a
// silent shrink of any of them is the cheapest way to make this gate vacuous.
// ─────────────────────────────────────────────────────────────────────────────

// writePrimitives maps os.<Name> to the argument indexes that carry a path.
// Residual 4 of the threat model: this list is the model, and a primitive outside
// it is outside the analyser. It is pinned rather than trusted to be complete.
var writePrimitives = map[string][]int{
	"WriteFile":  {0},
	"Create":     {0},
	"CreateTemp": {0},
	"OpenFile":   {0},
	"Rename":     {0, 1},
	"MkdirAll":   {0},
	"Mkdir":      {0},
	"Symlink":    {1},
}

// guardFunctions are the pathguard entry points that establish containment over
// their second argument. GuardedWrite both guards and writes.
var guardFunctions = map[string]struct {
	rootArg   int
	targetArg int
	isWrite   bool
}{
	"RejectAndReport":        {0, 1, false},
	"RejectSymlinks":         {0, 1, false},
	"GuardedWrite":           {0, 1, true},
	"RefuseUnverifiableRoot": {-1, 0, false},
}

// approvedResolvers produce a root that has been through EvalSymlinks on their
// success path. 🔴 See declared limit 1: three of them fall back to the
// unresolved path on failure, and that is exactly what T5 says this analyser
// cannot see.
var approvedResolvers = map[string]bool{
	"EvalSymlinks": true, // filepath.EvalSymlinks — the primitive itself
	"projectRoot":  true,
	"scaffoldRoot": true,
	"resolveRoot":  true,
	"ResolveRoot":  true,
}

// unresolvedProducers are the sources that yield a path in the LOGICAL namespace.
// 🔴 filepath.Clean is here and is NOT transparent: Clean(x) is unresolved
// whatever x is. Treating it as a pass-through is precisely how an analyser
// approves RejectAndReport(filepath.Clean(cwd), p) — threat model §3.4.
var unresolvedProducers = map[string]bool{
	"Getwd":         true, // os.Getwd — honours $PWD, measured in R-1
	"UserHomeDir":   true,
	"Abs":           true, // filepath.Abs — inherits the logical cwd
	"Clean":         true, // filepath.Clean — NOT transparent
	"Dir":           true, // homedir.Dir
	"HomeDir":       true,
	"UserConfigDir": true,
	"UserCacheDir":  true,
}

// taintTransparent are path combinators that carry root-provenance through for
// the purpose of the P1 POPULATION (not for P2 resolution). filepath.Base is
// deliberately absent: it strips the root, so what comes out is a bare filename
// and the write it feeds is governed by a different mechanism.
var taintTransparent = map[string]bool{
	"Join": true, "Clean": true, "Dir": true, "Abs": true,
	"EvalSymlinks": true, "ToSlash": true, "FromSlash": true, "Rel": true,
}

// taintSourceCalls are the roots themselves: a path deriving from any of these
// is "derived from a user-supplied root" in the sense of the ADR.
var taintSourceCalls = map[string]bool{
	"Getwd": true, "UserHomeDir": true, "Abs": true, "EvalSymlinks": true,
	"projectRoot": true, "scaffoldRoot": true, "resolveRoot": true,
	"ResolveRoot": true, "Dir": false,
}

const analyzerVocabularyFloors = 8 // len(writePrimitives); see TestAnalyserVocabularyIsPinned

// ─────────────────────────────────────────────────────────────────────────────
// Findings
// ─────────────────────────────────────────────────────────────────────────────

// Finding always names the artifact: file, line, and the enclosing function. A
// violation that only names the rule gives a false green — measured lesson of
// this campaign, and the reason every arm below asserts on the site, not on the
// count alone.
type Finding struct {
	File   string
	Line   int
	Func   string
	Kind   string
	Path   string
	Detail string
}

func (f Finding) String() string {
	return fmt.Sprintf("%s:%d %s() [%s] path=%s — %s", f.File, f.Line, f.Func, f.Kind, f.Path, f.Detail)
}

// siteKey identifies a site for the exception list. 🔴 It deliberately does NOT
// include the line number: a line-keyed exception silently expires when the file
// is edited, and an expired exception reads as "the rule now passes".
func (f Finding) siteKey() string {
	return fmt.Sprintf("%s|%s|%s|%s", f.File, f.Func, f.Kind, f.Path)
}

const (
	kindUnguardedWrite = "unguarded-write"    // P1
	kindGuardNotActed  = "guard-not-acted-on" // P1 — the empty `if err != nil {}` of ML-7B residual 3
	kindRawPredicate   = "raw-predicate"      // single emitter — alias-proof, ML-7B residual 2
	kindRogueEmitter   = "rogue-emitter"      // single emitter
	kindRootUnresolved = "root-unresolved"    // P2
	kindRootUnknown    = "root-unknown"       // P2
	kindRootAliased    = "root-aliased"       // P2 / Wave 8
)

// Report is the whole measurement of one unit set.
type Report struct {
	Findings      []Finding
	P2Unresolved  []Finding
	P2Unknown     []Finding
	RootAliased   []Finding
	FilesParsed   int
	WriteSites    int // every call to a write primitive, in or out of population
	InPopulation  int // write sites whose path derives from a user-supplied root
	GuardCalls    int
	GuardedWrites int
}

// ─────────────────────────────────────────────────────────────────────────────
// Unit set
// ─────────────────────────────────────────────────────────────────────────────

type unit struct {
	name string
	src  string
}

type parsedFile struct {
	name         string
	file         *ast.File
	pathguardPkg string // the local name of the pathguard import in THIS file —
	// resolving it per file is what makes `pg "…/pathguard"` visible (ML-7B residual 2)
	importsPathguard bool
	isPathguard      bool
}

type analyzer struct {
	fset      *token.FileSet
	files     []parsedFile
	funcs     map[string]*funcInfo
	funcOrder []string
}

type funcInfo struct {
	key    string
	name   string
	file   *parsedFile
	decl   *ast.FuncDecl
	params []string
	// guardsParam[i] — calling this function establishes containment over argument i.
	guardsParam map[int]bool
	// returnsTainted — some return value derives from a user-supplied root.
	returnsTainted bool
	// taintedParam[i] — some caller passes a root-derived value in position i.
	taintedParam map[int]bool
	// resolvedParam[i] — every caller passes a value with approved-resolver provenance.
	resolvedParam   map[int]resolution
	returnsResolved resolution
	// preGuardedParam[i] — EVERY call site of this function is dominated by a
	// guard covering argument i, so the parameter arrives contained.
	preGuardedParam map[int]bool
}

type resolution int

const (
	resUnknown resolution = iota
	resResolved
	resUnresolved
)

func newAnalyzer(units []unit) (*analyzer, error) {
	a := &analyzer{fset: token.NewFileSet(), funcs: map[string]*funcInfo{}}
	for _, u := range units {
		file, err := parser.ParseFile(a.fset, u.name, u.src, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", u.name, err)
		}
		pf := parsedFile{name: u.name, file: file, pathguardPkg: "pathguard"}
		pf.isPathguard = file.Name.Name == "pathguard"
		for _, imp := range file.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if strings.HasSuffix(path, "/internal/pathguard") {
				pf.importsPathguard = true
				if imp.Name != nil {
					pf.pathguardPkg = imp.Name.Name
				}
			}
		}
		a.files = append(a.files, pf)
	}
	for i := range a.files {
		pf := &a.files[i]
		for _, decl := range pf.file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			key := funcKey(fd)
			info := &funcInfo{
				key: key, name: key, file: pf, decl: fd,
				guardsParam: map[int]bool{}, taintedParam: map[int]bool{},
				resolvedParam: map[int]resolution{}, preGuardedParam: map[int]bool{},
			}
			for _, field := range fd.Type.Params.List {
				for _, ident := range field.Names {
					info.params = append(info.params, ident.Name)
				}
				if len(field.Names) == 0 {
					info.params = append(info.params, "_")
				}
			}
			if _, clash := a.funcs[key]; !clash {
				a.funcOrder = append(a.funcOrder, key)
			}
			a.funcs[key] = info
		}
	}
	return a, nil
}

func funcKey(fd *ast.FuncDecl) string {
	if fd.Recv != nil && len(fd.Recv.List) > 0 {
		return exprString(nil, fd.Recv.List[0].Type) + "." + fd.Name.Name
	}
	return fd.Name.Name
}

// exprString renders an expression to source text. Comparison of rendered text
// is how "the guard covers the path that is written" is decided; see coversPath.
func exprString(fset *token.FileSet, e ast.Expr) string {
	if e == nil {
		return ""
	}
	var sb strings.Builder
	if fset == nil {
		fset = token.NewFileSet()
	}
	if err := printer.Fprint(&sb, fset, e); err != nil {
		return "<unprintable>"
	}
	return strings.Join(strings.Fields(sb.String()), " ")
}

func (a *analyzer) render(e ast.Expr) string { return exprString(a.fset, e) }

// ─────────────────────────────────────────────────────────────────────────────
// Local environment: variable → defining expression, within one function body.
// ─────────────────────────────────────────────────────────────────────────────

type guardRecord struct {
	at     located
	target ast.Expr
	name   string
}

type writeRecord struct {
	at   located
	path ast.Expr
	name string
}

// callRecord is one call to a function of the analysed unit set, kept with the
// caller's identity and position so that "every caller guards argument i" can be
// decided.
type callRecord struct {
	caller *funcInfo
	callee string
	at     located
	stmt   ast.Stmt
	args   []ast.Expr
}

type localEnv map[string]ast.Expr

func (a *analyzer) buildEnv(fd *ast.FuncDecl) localEnv {
	env := localEnv{}
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.AssignStmt:
			// Multi-value from one call: `reqFiles, err := scanREQFiles(cfg)`.
			// 🔴 Skipping this shape is what made syncREQReferences — the flagship
			// FALSE MARKER of #400, with zero pathguard calls in the whole function —
			// invisible to the population: its write path comes from exactly such an
			// assignment.
			if len(stmt.Lhs) > 1 && len(stmt.Rhs) == 1 {
				if _, isCall := stmt.Rhs[0].(*ast.CallExpr); isCall {
					for _, lhs := range stmt.Lhs {
						if ident, ok := lhs.(*ast.Ident); ok && ident.Name != "_" {
							if _, seen := env[ident.Name]; !seen {
								env[ident.Name] = stmt.Rhs[0]
							}
						}
					}
				}
			}
			if len(stmt.Lhs) == len(stmt.Rhs) {
				for i, lhs := range stmt.Lhs {
					if ident, ok := lhs.(*ast.Ident); ok && ident.Name != "_" {
						if _, seen := env[ident.Name]; !seen {
							env[ident.Name] = stmt.Rhs[i]
						}
					}
				}
			}
		case *ast.ValueSpec:
			for i, ident := range stmt.Names {
				if i < len(stmt.Values) && ident.Name != "_" {
					if _, seen := env[ident.Name]; !seen {
						env[ident.Name] = stmt.Values[i]
					}
				}
			}
		case *ast.RangeStmt:
			if ident, ok := stmt.Value.(*ast.Ident); ok && ident.Name != "_" {
				if _, seen := env[ident.Name]; !seen {
					env[ident.Name] = stmt.X // element of a slice of paths inherits its taint
				}
			}
		}
		return true
	})
	return env
}

// selectorParts splits pkg.Fn( … ) into ("pkg", "Fn").
func selectorParts(e ast.Expr) (string, string, bool) {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return "", "", false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", "", false
	}
	return ident.Name, sel.Sel.Name, true
}

// ─────────────────────────────────────────────────────────────────────────────
// Taint: does this expression derive from a user-supplied root?
// ─────────────────────────────────────────────────────────────────────────────

func (a *analyzer) isTainted(fn *funcInfo, env localEnv, e ast.Expr, depth int) bool {
	if e == nil || depth > 12 {
		return false
	}
	switch expr := e.(type) {
	case *ast.Ident:
		for i, p := range fn.params {
			if p == expr.Name && fn.taintedParam[i] {
				return true
			}
		}
		if def, ok := env[expr.Name]; ok && def != e {
			return a.isTainted(fn, env, def, depth+1)
		}
		return false
	case *ast.SelectorExpr:
		// cfg.ReqDir, c.RoadmapDir … — a config-specified directory IS a
		// user-supplied root by the ADR's own definition.
		if base, ok := expr.X.(*ast.Ident); ok {
			lower := strings.ToLower(base.Name)
			if strings.Contains(lower, "cfg") || strings.Contains(lower, "config") {
				return true
			}
		}
		return a.isTainted(fn, env, expr.X, depth+1)
	case *ast.CallExpr:
		if pkg, name, ok := selectorParts(expr.Fun); ok {
			if (pkg == "os" || pkg == "filepath" || pkg == "homedir" || pkg == "config") && taintSourceCalls[name] {
				return true
			}
			if pkg == "config" && name == "Load" {
				return true
			}
			if pkg == "filepath" && taintTransparent[name] {
				for _, arg := range expr.Args {
					if a.isTainted(fn, env, arg, depth+1) {
						return true
					}
				}
				return false
			}
			if pkg == "fmt" && (name == "Sprintf" || name == "Sprint") {
				for _, arg := range expr.Args {
					if a.isTainted(fn, env, arg, depth+1) {
						return true
					}
				}
				return false
			}
		}
		if ident, ok := expr.Fun.(*ast.Ident); ok {
			if taintSourceCalls[ident.Name] {
				return true
			}
			if callee, ok := a.funcs[ident.Name]; ok && callee.returnsTainted {
				return true
			}
		}
		if sel, ok := expr.Fun.(*ast.SelectorExpr); ok {
			if callee, ok := a.funcs[sel.Sel.Name]; ok && callee.returnsTainted {
				return true
			}
		}
		// Conservative transfer: a call the analyser cannot resolve (another
		// package, an interface method) that is FED a root-derived value returns a
		// root-derived value. syncREQReferences — the flagship false marker of
		// #400 — writes paths that come from validator.ResolveREQFiles(cfg), and
		// without this rule the whole function is invisible to the population.
		for _, arg := range expr.Args {
			if a.isTainted(fn, env, arg, depth+1) {
				return true
			}
		}
		return false
	case *ast.BinaryExpr:
		return a.isTainted(fn, env, expr.X, depth+1) || a.isTainted(fn, env, expr.Y, depth+1)
	case *ast.ParenExpr:
		return a.isTainted(fn, env, expr.X, depth+1)
	case *ast.IndexExpr:
		return a.isTainted(fn, env, expr.X, depth+1)
	}
	return false
}

// ─────────────────────────────────────────────────────────────────────────────
// P2: provenance of the guard's FIRST operand.
// ─────────────────────────────────────────────────────────────────────────────

func (a *analyzer) provenance(fn *funcInfo, env localEnv, e ast.Expr, depth int) resolution {
	if e == nil || depth > 12 {
		return resUnknown
	}
	switch expr := e.(type) {
	case *ast.Ident:
		for i, p := range fn.params {
			if p == expr.Name {
				if r, ok := fn.resolvedParam[i]; ok {
					return r
				}
				return resUnknown
			}
		}
		if def, ok := env[expr.Name]; ok && def != e {
			return a.provenance(fn, env, def, depth+1)
		}
		return resUnknown
	case *ast.CallExpr:
		if pkg, name, ok := selectorParts(expr.Fun); ok {
			// 🔴 Clean is checked BEFORE the transparency of Join and friends:
			// filepath.Clean(cwd) is unresolved no matter what is inside it.
			if pkg == "filepath" && name == "Clean" {
				return resUnresolved
			}
			if pkg == "filepath" && name == "EvalSymlinks" {
				return resResolved
			}
			if (pkg == "os" || pkg == "filepath" || pkg == "homedir") && unresolvedProducers[name] {
				return resUnresolved
			}
			if pkg == "filepath" && name == "Join" && len(expr.Args) > 0 {
				return a.provenance(fn, env, expr.Args[0], depth+1)
			}
			if pkg == "pathguard" && approvedResolvers[name] {
				return resResolved
			}
		}
		if ident, ok := expr.Fun.(*ast.Ident); ok {
			if approvedResolvers[ident.Name] {
				return resResolved
			}
			if callee, ok := a.funcs[ident.Name]; ok {
				return callee.returnsResolved
			}
		}
		return resUnknown
	case *ast.ParenExpr:
		return a.provenance(fn, env, expr.X, depth+1)
	case *ast.SelectorExpr:
		if base, ok := expr.X.(*ast.Ident); ok {
			lower := strings.ToLower(base.Name)
			if strings.Contains(lower, "cfg") || strings.Contains(lower, "config") {
				return resUnresolved // a config directory is a raw string, never resolved
			}
		}
		return resUnknown
	}
	return resUnknown
}

// ─────────────────────────────────────────────────────────────────────────────
// Call-site inspection helpers
// ─────────────────────────────────────────────────────────────────────────────

// guardCallAt reports whether the call is a pathguard entry point, resolving the
// package qualifier through the FILE's import alias. 🔴 This is what closes
// ML-7B residual 2: `pg "…/pathguard"` is recognised, where a literal scanner for
// "pathguard.RejectSymlinks(" sees nothing.
func (a *analyzer) guardCallAt(pf *parsedFile, call *ast.CallExpr) (string, bool) {
	pkg, name, ok := selectorParts(call.Fun)
	if !ok {
		// Inside package pathguard itself the calls are unqualified.
		if ident, isIdent := call.Fun.(*ast.Ident); isIdent && pf.isPathguard {
			if _, known := guardFunctions[ident.Name]; known {
				return ident.Name, true
			}
		}
		return "", false
	}
	if pkg != pf.pathguardPkg {
		return "", false
	}
	if _, known := guardFunctions[name]; !known {
		return "", false
	}
	return name, true
}

func (a *analyzer) writeCallAt(call *ast.CallExpr) (string, []int, bool) {
	pkg, name, ok := selectorParts(call.Fun)
	if !ok || pkg != "os" {
		return "", nil, false
	}
	indexes, known := writePrimitives[name]
	if !known {
		return "", nil, false
	}
	return name, indexes, true
}

// ─────────────────────────────────────────────────────────────────────────────
// Dominance by block ancestry
// ─────────────────────────────────────────────────────────────────────────────
//
// 🔴 Source order alone would approve a guard nested in an `if` that the write
// does not go through — a false negative, which is the unsafe direction. A guard
// dominates a write only when the guard's block chain is a PREFIX of the write's
// (i.e. the guard's block encloses the write) and the guard comes first.

type located struct {
	blockPath []ast.Node
	pos       token.Pos
}

func (g located) dominates(w located) bool {
	if len(g.blockPath) > len(w.blockPath) {
		return false
	}
	for i, node := range g.blockPath {
		if w.blockPath[i] != node {
			return false
		}
	}
	return g.pos < w.pos
}

// walkWithBlocks visits every call expression in the body, handing the visitor
// the chain of enclosing block-like nodes.
func walkWithBlocks(body *ast.BlockStmt, visit func(call *ast.CallExpr, at located, stmt ast.Stmt)) {
	var walkStmts func(stmts []ast.Stmt, path []ast.Node)
	var walkStmt func(stmt ast.Stmt, path []ast.Node)

	collect := func(node ast.Node, path []ast.Node, stmt ast.Stmt) {
		ast.Inspect(node, func(n ast.Node) bool {
			switch inner := n.(type) {
			case *ast.BlockStmt:
				return false // nested blocks are visited by walkStmt with their own path
			case *ast.FuncLit:
				return false
			case *ast.CallExpr:
				visit(inner, located{blockPath: path, pos: inner.Pos()}, stmt)
			}
			return true
		})
	}

	walkStmt = func(stmt ast.Stmt, path []ast.Node) {
		switch s := stmt.(type) {
		case *ast.BlockStmt:
			walkStmts(s.List, append(append([]ast.Node{}, path...), s))
		case *ast.IfStmt:
			if s.Init != nil {
				collect(s.Init, path, stmt)
			}
			collect(s.Cond, path, stmt)
			walkStmt(s.Body, path)
			if s.Else != nil {
				walkStmt(s.Else, path)
			}
		case *ast.ForStmt:
			if s.Init != nil {
				collect(s.Init, path, stmt)
			}
			if s.Cond != nil {
				collect(s.Cond, path, stmt)
			}
			walkStmt(s.Body, path)
		case *ast.RangeStmt:
			collect(s.X, path, stmt)
			walkStmt(s.Body, path)
		case *ast.SwitchStmt:
			if s.Init != nil {
				collect(s.Init, path, stmt)
			}
			if s.Tag != nil {
				collect(s.Tag, path, stmt)
			}
			for _, clause := range s.Body.List {
				if cc, ok := clause.(*ast.CaseClause); ok {
					walkStmts(cc.Body, append(append([]ast.Node{}, path...), cc))
				}
			}
		case *ast.TypeSwitchStmt:
			for _, clause := range s.Body.List {
				if cc, ok := clause.(*ast.CaseClause); ok {
					walkStmts(cc.Body, append(append([]ast.Node{}, path...), cc))
				}
			}
		case *ast.SelectStmt:
			for _, clause := range s.Body.List {
				if cc, ok := clause.(*ast.CommClause); ok {
					walkStmts(cc.Body, append(append([]ast.Node{}, path...), cc))
				}
			}
		case *ast.LabeledStmt:
			walkStmt(s.Stmt, path)
		default:
			collect(stmt, path, stmt)
		}
	}

	walkStmts = func(stmts []ast.Stmt, path []ast.Node) {
		for _, stmt := range stmts {
			walkStmt(stmt, path)
		}
	}
	walkStmts(body.List, []ast.Node{body})
}

// ─────────────────────────────────────────────────────────────────────────────
// Coverage: does a guard on G cover a write to W?
// ─────────────────────────────────────────────────────────────────────────────
//
// RejectSymlinks walks W and every ancestor up to the root, so a guard on G
// covers W when W IS G, or when W is an ancestor of G — writing the directory
// that the guarded leaf lives in is inside the walked chain.
//
// 🔴 The converse is deliberately NOT accepted: a guard on a DIRECTORY does not
// cover a write to a child of it. That asymmetry is the leaf gap of #400 — the
// lefthook branch guarded ".husky" and ".lefthook/commit-msg" and wrote
// "lefthook.yml" — and accepting it here would make the analyser approve exactly
// the 22 defects it exists to catch.

func (a *analyzer) coversPath(fn *funcInfo, env localEnv, guardTarget, writePath ast.Expr) coverage {
	guard := a.canonical(fn, env, guardTarget, 0)
	write := a.canonical(fn, env, writePath, 0)
	if guard == "" || write == "" {
		return coverNone
	}
	if guard == write {
		return coverExact
	}
	// The write is the PARENT of the guarded leaf: RejectSymlinks walked that
	// parent on its way up, so it is contained. (The converse — guard on the
	// parent, write the child — is the leaf gap and stays refused.)
	if write == "filepath.Dir("+guard+")" {
		return coverExact
	}
	guardChain, writeChain := components(guard), components(write)
	if coversComponents(guardChain, writeChain) {
		return coverExact
	}
	// Same relative chain under a DIFFERENT root spelling: the site guards
	// root/.husky/pre-commit and writes rootDir/.husky/pre-commit. The flow does
	// pass a guard covering this chain — what differs is the ARGUMENT, which is
	// P2's question and Wave 8's population, not a missing guard. Reporting it as
	// "unguarded" would be wrong; swallowing it would be worse.
	if len(guardChain) > 1 && len(writeChain) > 1 &&
		coversComponents(guardChain[1:], writeChain[1:]) {
		return coverRootAliased
	}
	return coverNone
}

type coverage int

const (
	coverNone coverage = iota
	coverExact
	coverRootAliased
)

// components flattens a canonical path expression into its path components,
// expanding nested filepath.Join, and shortening filepath.Dir(x) to x's chain
// minus its last component — Dir IS "the parent of", which is what containment
// reasons about.
func components(canonical string) []string {
	if inner, ok := strings.CutPrefix(canonical, "filepath.Dir("); ok && strings.HasSuffix(inner, ")") {
		parent := components(strings.TrimSuffix(inner, ")"))
		if len(parent) > 1 {
			return parent[:len(parent)-1]
		}
		return []string{canonical}
	}
	// filepath.Base(x) IS the last component of x — the shape the leaf guards of
	// Wave 4 use: RejectAndReport(root, filepath.Join(root, filepath.Base(target))).
	if inner, ok := strings.CutPrefix(canonical, "filepath.Base("); ok && strings.HasSuffix(inner, ")") {
		chain := components(strings.TrimSuffix(inner, ")"))
		return chain[len(chain)-1:]
	}
	// A string literal with separators inside is a multi-component path:
	// os.MkdirAll(".github/workflows") is the same chain as
	// filepath.Join(root, ".github", "workflows").
	if strings.HasPrefix(canonical, `"`) && strings.HasSuffix(canonical, `"`) {
		raw := strings.Trim(canonical, `"`)
		if strings.Contains(raw, "/") {
			var out []string
			for _, part := range strings.Split(raw, "/") {
				if part != "" {
					out = append(out, `"`+part+`"`)
				}
			}
			return out
		}
		return []string{canonical}
	}
	const prefix = "filepath.Join("
	if !strings.HasPrefix(canonical, prefix) {
		return []string{canonical}
	}
	args := splitTopLevel(strings.TrimSuffix(strings.TrimPrefix(canonical, prefix), ")"))
	var out []string
	for _, arg := range args {
		out = append(out, components(arg)...)
	}
	return out
}

// coversComponents decides containment between two component chains.
//
// The guarded chain may carry a root prefix the written chain does not spell out
// (the site writes a project-root-relative path and guards its absolute form —
// the dominant shape of the ML-4B leaf-gap fix). So the written chain may start
// at index 0 or 1 of the guarded chain. From there it must be a PREFIX:
//
//	guard [root, docs, req, X.md]   write [docs, req]        → covered (ancestor)
//	guard [root, docs, req, X.md]   write [docs, req, X.md]  → covered (same path)
//
// 🔴 The converse is refused by construction: a written chain LONGER than the
// guarded one is the leaf gap of #400 — guarding ".husky" and writing
// ".husky/pre-commit", or guarding a directory and writing a file inside it.
// RejectSymlinks walks ancestors, never descendants, so approving that direction
// would make the analyser bless the 22 defects it exists to catch.
func coversComponents(guard, write []string) bool {
	if len(guard) == 0 || len(write) == 0 {
		return false
	}
	for _, offset := range []int{0, 1} {
		if offset+len(write) > len(guard) {
			continue
		}
		matched := true
		for i := range write {
			if guard[offset+i] != write[i] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

// canonical renders an expression with local variables substituted by their
// defining expressions, so that `absPath` and `filepath.Join(root, name)` compare
// equal when they are the same value.
func (a *analyzer) canonical(fn *funcInfo, env localEnv, e ast.Expr, depth int) string {
	if e == nil || depth > 8 {
		return ""
	}
	switch expr := e.(type) {
	case *ast.Ident:
		if def, ok := env[expr.Name]; ok && def != e {
			if sub := a.canonical(fn, env, def, depth+1); sub != "" {
				return sub
			}
		}
		return expr.Name
	case *ast.CallExpr:
		if pkg, name, ok := selectorParts(expr.Fun); ok && pkg == "filepath" {
			parts := make([]string, 0, len(expr.Args))
			for _, arg := range expr.Args {
				parts = append(parts, a.canonical(fn, env, arg, depth+1))
			}
			return "filepath." + name + "(" + strings.Join(parts, ", ") + ")"
		}
		return a.render(e)
	case *ast.ParenExpr:
		return a.canonical(fn, env, expr.X, depth+1)
	}
	return a.render(e)
}

func splitTopLevel(s string) []string {
	var parts []string
	depth, start := 0, 0
	for i, r := range s {
		switch r {
		case '(', '[':
			depth++
		case ')', ']':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	if strings.TrimSpace(s[start:]) != "" {
		parts = append(parts, strings.TrimSpace(s[start:]))
	}
	return parts
}

// ─────────────────────────────────────────────────────────────────────────────
// Acting on the guard's error
// ─────────────────────────────────────────────────────────────────────────────
//
// 🔴 ML-7B residual 3: the textual scanner proves the site DELEGATES, not that it
// ACTS. `if err := guard(…); err != nil { }` passes a scanner and writes anyway.
//
// The predicate is NOT "the branch returns the error" — discover's advisory
// legitimately prints and `return nil`s (skip the file, do not write). It is:
// the failure branch must not fall through to the write.

func failureBranchIsTerminating(stmt ast.Stmt) (ok bool, detail string) {
	ifStmt, isIf := stmt.(*ast.IfStmt)
	if !isIf {
		switch s := stmt.(type) {
		case *ast.ReturnStmt:
			return true, "" // return guard(…) — the error propagates, nothing follows
		case *ast.AssignStmt:
			// err := guard(…) — handled by the caller, which looks for a later
			// `if err != nil` on the same identifier.
			_ = s
			return false, "guard result assigned but no failure branch found in the same block"
		case *ast.ExprStmt:
			return false, "guard called as a bare statement — its error is discarded"
		}
		return false, "guard result is not consumed by a failure branch"
	}
	if ifStmt.Body == nil || len(ifStmt.Body.List) == 0 {
		return false, "failure branch is EMPTY — the guard refuses and the flow falls through to the write"
	}
	if blockTerminates(ifStmt.Body) {
		return true, ""
	}
	return false, "failure branch does not stop the flow — it falls through to the write"
}

func blockTerminates(block *ast.BlockStmt) bool {
	if block == nil || len(block.List) == 0 {
		return false
	}
	for _, stmt := range block.List {
		switch s := stmt.(type) {
		case *ast.ReturnStmt:
			return true
		case *ast.BranchStmt:
			return true // continue / break / goto — leaves the write behind
		case *ast.ExprStmt:
			if call, ok := s.X.(*ast.CallExpr); ok {
				if pkg, name, ok := selectorParts(call.Fun); ok {
					if (pkg == "os" && name == "Exit") || (pkg == "log" && (name == "Fatal" || name == "Fatalf")) {
						return true
					}
				}
				if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "panic" {
					return true
				}
			}
		case *ast.IfStmt:
			if blockTerminates(s.Body) && s.Else != nil {
				if elseBlock, ok := s.Else.(*ast.BlockStmt); ok && blockTerminates(elseBlock) {
					return true
				}
			}
		}
	}
	return false
}

// ─────────────────────────────────────────────────────────────────────────────
// The fixpoint
// ─────────────────────────────────────────────────────────────────────────────

func (a *analyzer) computeFacts() {
	for round := 0; round < 6; round++ {
		changed := false
		for _, key := range a.funcOrder {
			fn := a.funcs[key]
			env := a.buildEnv(fn.decl)

			// (1) does this function guard one of its parameters?
			walkWithBlocks(fn.decl.Body, func(call *ast.CallExpr, _ located, _ ast.Stmt) {
				targetArg := -1
				if name, ok := a.guardCallAt(fn.file, call); ok {
					targetArg = guardFunctions[name].targetArg
				} else if ident, ok := call.Fun.(*ast.Ident); ok {
					if callee, known := a.funcs[ident.Name]; known {
						for idx, guards := range callee.guardsParam {
							if guards && idx < len(call.Args) {
								if p := a.paramIndexOf(fn, env, call.Args[idx]); p >= 0 && !fn.guardsParam[p] {
									fn.guardsParam[p] = true
									changed = true
								}
							}
						}
					}
				}
				if targetArg >= 0 && targetArg < len(call.Args) {
					if p := a.paramIndexOf(fn, env, call.Args[targetArg]); p >= 0 && !fn.guardsParam[p] {
						fn.guardsParam[p] = true
						changed = true
					}
				}
			})

			// (2) does this function return a root-derived path?
			if !fn.returnsTainted {
				ast.Inspect(fn.decl.Body, func(n ast.Node) bool {
					ret, ok := n.(*ast.ReturnStmt)
					if !ok {
						return true
					}
					for _, result := range ret.Results {
						if a.isTainted(fn, env, result, 0) {
							fn.returnsTainted = true
							changed = true
						}
					}
					return true
				})
			}

			// (3) provenance of what this function returns (first string result).
			if fn.returnsResolved == resUnknown {
				ast.Inspect(fn.decl.Body, func(n ast.Node) bool {
					ret, ok := n.(*ast.ReturnStmt)
					if !ok || len(ret.Results) == 0 {
						return true
					}
					if r := a.provenance(fn, env, ret.Results[0], 0); r == resResolved && fn.returnsResolved != resUnresolved {
						fn.returnsResolved = resResolved
						changed = true
					}
					return true
				})
			}

			// (4) propagate taint and provenance into the parameters of callees.
			ast.Inspect(fn.decl.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				var callee *funcInfo
				if ident, isIdent := call.Fun.(*ast.Ident); isIdent {
					callee = a.funcs[ident.Name]
				} else if sel, isSel := call.Fun.(*ast.SelectorExpr); isSel {
					callee = a.funcs[sel.Sel.Name]
				}
				if callee == nil {
					return true
				}
				for i, arg := range call.Args {
					if i >= len(callee.params) {
						break
					}
					if a.isTainted(fn, env, arg, 0) && !callee.taintedParam[i] {
						callee.taintedParam[i] = true
						changed = true
					}
					r := a.provenance(fn, env, arg, 0)
					if r != resUnknown {
						prev, seen := callee.resolvedParam[i]
						merged := r
						if seen && prev != r {
							merged = resUnresolved // any unresolved caller taints the parameter
						}
						if !seen || prev != merged {
							callee.resolvedParam[i] = merged
							changed = true
						}
					}
				}
				return true
			})
		}
		if !changed {
			break
		}
	}
}

// collectGuardsAndCalls gathers, for one function, the containment guards that
// appear in it (direct, through a guarding helper, or inherited from a
// pre-guarded parameter) and every call it makes to a function of the unit set.
func (a *analyzer) collectGuardsAndCalls(pf *parsedFile, fn *funcInfo, env localEnv) ([]guardRecord, []callRecord) {
	var guards []guardRecord
	var calls []callRecord
	for index, guarded := range fn.preGuardedParam {
		if guarded && index < len(fn.params) {
			guards = append(guards, guardRecord{
				at:     located{blockPath: []ast.Node{fn.decl.Body}, pos: fn.decl.Body.Pos()},
				target: &ast.Ident{Name: fn.params[index]},
				name:   "«every caller guards this parameter»",
			})
		}
	}
	walkWithBlocks(fn.decl.Body, func(call *ast.CallExpr, at located, stmt ast.Stmt) {
		if name, ok := a.guardCallAt(pf, call); ok {
			spec := guardFunctions[name]
			if spec.targetArg < len(call.Args) {
				guards = append(guards, guardRecord{at, call.Args[spec.targetArg], name})
			}
			return
		}
		ident, isIdent := call.Fun.(*ast.Ident)
		if !isIdent {
			return
		}
		callee, known := a.funcs[ident.Name]
		if !known {
			return
		}
		calls = append(calls, callRecord{caller: fn, callee: callee.key, at: at, stmt: stmt, args: call.Args})
		for index, doesGuard := range callee.guardsParam {
			if doesGuard && index < len(call.Args) {
				guards = append(guards, guardRecord{at, call.Args[index], ident.Name})
			}
		}
	})
	return guards, calls
}

// computePreGuardedParams decides, for every function of the unit set, which of
// its parameters arrive already contained because EVERY call site guards them.
//
// 🔴 "Every", not "some": one unguarded caller is enough for the parameter to be
// unsafe inside the callee, and a rule that accepted "some" would let a single
// guarded caller launder the whole function. A function with NO call site in the
// unit set gets nothing — absence of callers is not evidence of containment.
func (a *analyzer) computePreGuardedParams() {
	for round := 0; round < 4; round++ {
		guardedArg := map[string]map[int]int{} // callee → arg index → guarded call count
		totalCalls := map[string]int{}

		for i := range a.files {
			pf := &a.files[i]
			for _, decl := range pf.file.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				fn := a.funcs[funcKey(fd)]
				if fn == nil || fn.decl != fd {
					continue
				}
				env := a.buildEnv(fd)
				guards, calls := a.collectGuardsAndCalls(pf, fn, env)
				for _, record := range calls {
					totalCalls[record.callee]++
					if guardedArg[record.callee] == nil {
						guardedArg[record.callee] = map[int]int{}
					}
					for index, arg := range record.args {
						for _, g := range guards {
							if g.at.dominates(record.at) && a.coversPath(fn, env, g.target, arg) == coverExact {
								guardedArg[record.callee][index]++
								break
							}
						}
					}
				}
			}
		}

		changed := false
		for key, total := range totalCalls {
			callee := a.funcs[key]
			if callee == nil || total == 0 {
				continue
			}
			for index, guardedCount := range guardedArg[key] {
				if guardedCount == total && !callee.preGuardedParam[index] {
					callee.preGuardedParam[index] = true
					changed = true
				}
			}
		}
		if !changed {
			return
		}
	}
}

func (a *analyzer) paramIndexOf(fn *funcInfo, env localEnv, e ast.Expr) int {
	ident, ok := e.(*ast.Ident)
	if !ok {
		return -1
	}
	for i, p := range fn.params {
		if p == ident.Name {
			return i
		}
	}
	_ = env
	return -1
}

// ─────────────────────────────────────────────────────────────────────────────
// The analysis proper
// ─────────────────────────────────────────────────────────────────────────────

func (a *analyzer) analyze() Report {
	a.computeFacts()
	a.computePreGuardedParams()
	report := Report{FilesParsed: len(a.files)}

	for i := range a.files {
		pf := &a.files[i]
		for _, decl := range pf.file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			fn := a.funcs[funcKey(fd)]
			if fn == nil {
				continue
			}
			env := a.buildEnv(fd)

			var guards []guardRecord
			var writes []writeRecord

			// 🔴 Interprocedural P1: a parameter EVERY caller guards before passing
			// is contained inside the callee too. Without this, the class-(c)
			// reference implementation (integrations.atomicWrite) reads as a defect,
			// and the threat model's own measurement says a purely intraprocedural
			// analyser loses 17 of the 34 writes.
			for index, guarded := range fn.preGuardedParam {
				if !guarded || index >= len(fn.params) {
					continue
				}
				guards = append(guards, guardRecord{
					at:     located{blockPath: []ast.Node{fd.Body}, pos: fd.Body.Pos()},
					target: &ast.Ident{Name: fn.params[index]},
					name:   "«every caller guards this parameter»",
				})
			}

			walkWithBlocks(fd.Body, func(call *ast.CallExpr, at located, stmt ast.Stmt) {
				// ── containment guard ────────────────────────────────────────
				if name, ok := a.guardCallAt(pf, call); ok {
					spec := guardFunctions[name]
					report.GuardCalls++

					// Single-emitter, alias-proof: the raw predicate outside pathguard.
					if name == "RejectSymlinks" && !pf.isPathguard {
						report.Findings = append(report.Findings, a.finding(pf, call.Pos(), fn.name, kindRawPredicate,
							a.render(call.Args[spec.targetArg]),
							"raw predicate called outside package pathguard — the refusal is emitted by the caller, not by the single emitter"))
					}

					// P2 — provenance of the FIRST operand.
					if spec.rootArg >= 0 && spec.rootArg < len(call.Args) {
						rootExpr := call.Args[spec.rootArg]
						switch a.provenance(fn, env, rootExpr, 0) {
						case resUnresolved:
							report.P2Unresolved = append(report.P2Unresolved, a.finding(pf, rootExpr.Pos(), fn.name, kindRootUnresolved,
								a.render(rootExpr),
								"the guard root is in the logical namespace — filepath.Clean/Abs/Getwd is not a resolver"))
						case resUnknown:
							report.P2Unknown = append(report.P2Unknown, a.finding(pf, rootExpr.Pos(), fn.name, kindRootUnknown,
								a.render(rootExpr),
								"the guard root has no provenance the analyser can trace to a resolver"))
						}
					}

					// Does the site ACT on the refusal? Package pathguard is exempt:
					// it IS the emitter — RejectAndReport consumes the predicate's
					// error to build the refusal, so "the branch must stop the flow"
					// does not apply to the emitter itself. Every arm that exercises
					// this predicate below lives in another package, so the exemption
					// cannot hide a caller.
					if acted, detail := failureBranchIsTerminating(stmt); !acted && !pf.isPathguard {
						if !a.errorHandledLater(fd, stmt, at) {
							report.Findings = append(report.Findings, a.finding(pf, call.Pos(), fn.name, kindGuardNotActed,
								a.render(call.Args[spec.targetArg]), detail))
						} else {
							guards = append(guards, guardRecord{at, call.Args[spec.targetArg], name})
						}
					} else {
						guards = append(guards, guardRecord{at, call.Args[spec.targetArg], name})
					}
					if spec.isWrite {
						report.GuardedWrites++
					}
					return
				}

				// ── a named helper that guards one of its arguments ──────────
				// 🔴 Without this the lefthook branch of generateCommitMsgHook is
				// invisible: it guards through rejectScaffoldPath, not through a
				// direct pathguard call, and #400's flagship case lives there.
				if ident, isIdent := call.Fun.(*ast.Ident); isIdent {
					if callee, known := a.funcs[ident.Name]; known && len(callee.guardsParam) > 0 {
						for index, doesGuard := range callee.guardsParam {
							if !doesGuard || index >= len(call.Args) {
								continue
							}
							if acted, detail := failureBranchIsTerminating(stmt); !acted && !a.errorHandledLater(fd, stmt, at) {
								report.Findings = append(report.Findings, a.finding(pf, call.Pos(), fn.name, kindGuardNotActed,
									a.render(call.Args[index]), detail))
								continue
							}
							guards = append(guards, guardRecord{at, call.Args[index], ident.Name})
						}
						return
					}
				}

				// ── write primitive ──────────────────────────────────────────
				if name, indexes, ok := a.writeCallAt(call); ok {
					report.WriteSites++
					for _, index := range indexes {
						if index >= len(call.Args) {
							continue
						}
						writes = append(writes, writeRecord{at, call.Args[index], name})
					}
				}
			})

			// 🔴 Second population rule, and it is the one that catches the flagship
			// case of #400. `generateCommitMsgHook`'s lefthook branch writes the
			// literal "lefthook.yml" — a relative literal, so taint alone puts it
			// OUT of population — in a function that guards ".husky" and
			// ".lefthook/commit-msg" two lines above. A function that guards at all
			// has declared that it operates on user-root paths; every write in it is
			// therefore in population, whatever the spelling of its path.
			functionGuards := len(guards) > 0
			for _, w := range writes {
				if pf.isPathguard {
					// Self-exemption, the same one scripts/check-write-containment.sh
					// and single_emitter_test.go grant: GuardedWrite IMPLEMENTS the
					// guarded write (MkdirAll/CreateTemp/Rename after RejectAndReport),
					// so measuring it against itself says nothing. Every arm that
					// exercises P1 below lives in another package.
					continue
				}
				if !a.isTainted(fn, env, w.path, 0) && !functionGuards {
					continue // out of population: neither root-derived nor in a guarding function
				}
				report.InPopulation++
				best := coverNone
				for _, g := range guards {
					if !g.at.dominates(w.at) {
						continue
					}
					if c := a.coversPath(fn, env, g.target, w.path); c > best {
						best = c
					}
				}
				switch best {
				case coverExact:
					// contained
				case coverRootAliased:
					report.RootAliased = append(report.RootAliased, a.finding(pf, w.path.Pos(), fn.name, kindRootAliased,
						a.render(w.path),
						fmt.Sprintf("os.%s is dominated by a guard over the same relative chain under a DIFFERENT root spelling — the flow passes the guard, the argument does not match (Wave 8 population)", w.name)))
				default:
					report.Findings = append(report.Findings, a.finding(pf, w.path.Pos(), fn.name, kindUnguardedWrite,
						a.render(w.path),
						fmt.Sprintf("os.%s writes a root-derived path with no containment guard dominating it in this flow", w.name)))
				}
			}

			// ── rogue containment emitter (AST, so an alias cannot hide it) ──
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || pf.isPathguard {
					return true
				}
				pkg, name, ok := selectorParts(call.Fun)
				if !ok || pkg != "fmt" || (name != "Fprintf" && name != "Fprintln") {
					return true
				}
				if len(call.Args) < 2 {
					return true
				}
				if a.render(call.Args[0]) != "os.Stderr" {
					return true
				}
				lit, ok := call.Args[1].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				if lineIsRefusal(lit.Value) {
					report.Findings = append(report.Findings, a.finding(pf, call.Pos(), fn.name, kindRogueEmitter,
						lit.Value, "a containment refusal is printed outside package pathguard"))
				}
				return true
			})
		}
	}

	sortFindings(report.Findings)
	sortFindings(report.P2Unresolved)
	sortFindings(report.P2Unknown)
	sortFindings(report.RootAliased)
	sortFindings(report.RootAliased)
	return report
}

// errorHandledLater covers `err := guard(…)` followed by `if err != nil { … }`
// in the same block — a legitimate shape the single-statement check cannot see.
func (a *analyzer) errorHandledLater(fd *ast.FuncDecl, stmt ast.Stmt, at located) bool {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || len(assign.Lhs) == 0 {
		return false
	}
	names := map[string]bool{}
	for _, lhs := range assign.Lhs {
		if ident, ok := lhs.(*ast.Ident); ok {
			names[ident.Name] = true
		}
	}
	handled := false
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		ifStmt, ok := n.(*ast.IfStmt)
		if !ok || ifStmt.Pos() < at.pos {
			return true
		}
		binary, ok := ifStmt.Cond.(*ast.BinaryExpr)
		if !ok || binary.Op != token.NEQ {
			return true
		}
		ident, ok := binary.X.(*ast.Ident)
		if !ok || !names[ident.Name] {
			return true
		}
		if blockTerminates(ifStmt.Body) {
			handled = true
		}
		return true
	})
	return handled
}

func (a *analyzer) finding(pf *parsedFile, pos token.Pos, fnName, kind, path, detail string) Finding {
	position := a.fset.Position(pos)
	return Finding{File: pf.name, Line: position.Line, Func: fnName, Kind: kind, Path: path, Detail: detail}
}

func sortFindings(findings []Finding) {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		return findings[i].Line < findings[j].Line
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Unit-set builders
// ─────────────────────────────────────────────────────────────────────────────

// liveUnits reads every production Go file under the given roots of the working
// tree. Test files are excluded: they are not in the binary.
func liveUnits(repoRoot string, roots ...string) ([]unit, error) {
	var units []unit
	for _, root := range roots {
		err := filepath.Walk(filepath.Join(repoRoot, root), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				base := info.Name()
				if base == "testdata" || base == "vendor" || base == ".git" || base == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			relative, relErr := filepath.Rel(repoRoot, path)
			if relErr != nil {
				relative = path
			}
			units = append(units, unit{name: filepath.ToSlash(relative), src: string(data)})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(units, func(i, j int) bool { return units[i].name < units[j].name })
	return units, nil
}

// analyzeUnits is the single entry point every arm below goes through, so that
// no arm can accidentally analyse a different thing than the others.
func analyzeUnits(units []unit) (Report, error) {
	a, err := newAnalyzer(units)
	if err != nil {
		return Report{}, err
	}
	return a.analyze(), nil
}
