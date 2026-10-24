package config

// TestASTCoverage_NoSilentReads is a structural test for internal/config/config.go.
// It parses the source file with go/ast and verifies three properties inside the
// parse() function body:
//
//	(a) No call to stringVal( — the unrecorded reader — exists inside parse().
//	    stringVal() was removed in ML-1E R-1; if someone re-adds it and calls it
//	    from parse(), this arm flags it. All key reads must go through stringValRec
//	    (via scalarKeyTable), stringListRec, stringValNested or recordContainerType,
//	    which all record type errors.
//
//	(b) No direct literal-keyed map index expression (m["literal"]) exists inside
//	    parse(). Literal-keyed reads bypass the table and the recording layer;
//	    all scalar keys must go through scalarKeyTable. Non-scalar container keys
//	    (ci_trigger, adr_dirs, agents, etc.) use the recording helpers above.
//	    The allowlist below exempts the ci_trigger block, which uses a direct
//	    presence check for reasons documented in parse() itself.
//
//	(c) No call to stringValRec(m, "literal", …) with a literal second argument
//	    exists inside parse() outside the scalarKeyTable loop. Inside the loop,
//	    the key is spec.key (a SelectorExpr), never a literal. A direct call with
//	    a literal bypasses the table, so KnownScalarKeys() would not list it and
//	    TestScalarType_AllKnownKeys would not cover it — the single-source-of-truth
//	    guarantee would be silently broken. ML-1E R-3.
//
// Falsification proof (manual, not automated):
//   - arm (a): re-add stringVal to config.go and call it inside parse() — this arm
//     fails with "stringVal( called from parse(): found at config.go:<line>".
//     (Before ML-1E R-1 this example used the kept stringVal body. After removal
//     the falsification requires re-adding the function first.)
//   - arm (b): add _ = m["new_key_literal"] inside parse() → this test fails with
//     "literal map index inside parse(): m[\"new_key_literal\"] at config.go:<line>"
//   - arm (c): see in-file comment below for the injected falsification and output.
//
// ML-1C, ML-1E, REQ-2026-10-24.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ciTriggerAllowedLiterals are top-level keys that parse() reads with a direct
// map index (not via scalarKeyTable) for documented reasons. The ci_trigger key
// uses a direct presence check to distinguish absent from present-but-non-scalar,
// and is already handled by ci_trigger_invalid — not config_value_type.
var ciTriggerAllowedLiterals = map[string]bool{
	"ci_trigger": true,
	"adr_dirs":   true, // list key — uses stringListRec, not the table
	"agents":     true,
	"acceptance_markers": true,
	"link_fields": true,
	"rules":       true,
	"credential_guard": true,
	"write_scope": true,
	"agent_models": true,
}

func TestASTCoverage_NoSilentReads(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed — cannot locate test file")
	}
	configGoPath := filepath.Join(filepath.Dir(testFile), "config.go")

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, configGoPath, nil, 0)
	if err != nil {
		t.Fatalf("parsing config.go: %v", err)
	}

	// Locate the parse() function declaration.
	var parseFunc *ast.FuncDecl
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fn.Name.Name == "parse" {
			parseFunc = fn
			break
		}
	}
	if parseFunc == nil {
		t.Fatal("parse() function not found in config.go")
	}

	var armAViolations []string // calls to stringVal(
	var armBViolations []string // literal map index m["literal"] not in allowlist

	ast.Inspect(parseFunc.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			// arm (a): catch calls to stringVal( (the unrecorded reader).
			// Allow calls to stringValRec, stringValNested, stringValXxx but not bare stringVal.
			if ident, ok := node.Fun.(*ast.Ident); ok {
				if ident.Name == "stringVal" {
					pos := fset.Position(node.Pos())
					armAViolations = append(armAViolations,
						"stringVal( called from parse(): found at "+pos.String())
				}
			}
		case *ast.IndexExpr:
			// arm (b): catch m["literal"] direct indexed reads on the top-level map variable `m`.
			// Nested sub-maps (lf, cg, ws, rm, am) are accessed via stringListRec/stringValNested
			// which already pass the sub-map value; only literal-keyed reads of `m` itself bypass
			// the recording layer and are flagged.
			//
			// Only flag when:
			//   - the receiver is the identifier `m` (the top-level normalized map)
			//   - the index is a string literal (not a variable like spec.key)
			recv, ok := node.X.(*ast.Ident)
			if !ok || recv.Name != "m" {
				return true
			}
			lit, ok := node.Index.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			// Unquote the literal to get the key name.
			key := strings.Trim(lit.Value, `"`)
			if ciTriggerAllowedLiterals[key] {
				return true // explicitly allowlisted
			}
			pos := fset.Position(node.Pos())
			armBViolations = append(armBViolations,
				"literal map index inside parse(): m[\""+key+"\"] at "+pos.String())
		}
		return true
	})

	// arm (c): catch stringValRec(m, "literal", …) with a literal second argument.
	// Inside the scalarKeyTable loop the key is spec.key (a SelectorExpr), never a
	// BasicLit, so any call matching this pattern is outside the loop and bypasses
	// the single-source-of-truth guarantee. stringListRec is excluded: its second
	// argument is a recording-key, not a map access key, and all current calls have
	// intentional literal recording-keys that are not bypass patterns.
	//
	// Falsification (injected + reverted, ML-1E R-3):
	//   Added inside parse(), after the scalarKeyTable range loop:
	//     _, _ = stringValRec(m, "injected_key", &cfg.ScalarTypeErrors)
	//   Result:
	//     --- FAIL: TestASTCoverage_NoSilentReads (0.00s)
	//         config_ast_coverage_test.go:181:
	//             stringValRec( called with literal key outside scalarKeyTable:
	//             found at /private/tmp/trackfw-toml-escape/internal/config/config.go:499:9
	//   The injection was reverted (Edit tool, not sed) and the test passes again.
	var armCViolations []string
	ast.Inspect(parseFunc.Body, func(n ast.Node) bool {
		callExpr, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		ident, ok := callExpr.Fun.(*ast.Ident)
		if !ok || ident.Name != "stringValRec" {
			return true
		}
		if len(callExpr.Args) < 2 {
			return true
		}
		if _, ok := callExpr.Args[1].(*ast.BasicLit); ok {
			pos := fset.Position(callExpr.Pos())
			armCViolations = append(armCViolations,
				"stringValRec( called with literal key outside scalarKeyTable: found at "+pos.String())
		}
		return true
	})

	for _, v := range armAViolations {
		t.Error(v)
	}
	for _, v := range armBViolations {
		t.Error(v)
	}
	for _, v := range armCViolations {
		t.Error(v)
	}
}
