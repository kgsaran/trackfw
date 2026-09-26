package generators

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// fail_closed_root_test.go — ML-9A of REQ-2026-08-31.
//
// # The defect these arms falsify
//
// Five write sites held their containment guard INSIDE
// `if root, err := projectRoot(); err == nil { … }` while the write sat OUTSIDE
// the branch. When the root could not be established the guard never ran and the
// write proceeded UNGUARDED — and the `write-containment-allowed` marker above
// each of them asserted containment that did not exist in that branch. That is a
// false marker, the exact class issue #400 measures.
//
// # Why the seam, and why at os.Getwd
//
// projectRoot() has exactly ONE error path: os.Getwd failing (an EvalSymlinks
// failure falls back to the unresolved cwd and returns nil). getwdFn
// (scaffold.go) is the seam, so the branch is exercised deterministically on
// every platform. The filesystem trick — chdir into a directory and remove it —
// does NOT work on Windows (vault/notes/
// chdir-removeall-nao-forca-getwd-falhar-no-windows-2026-09-17.md), and a
// t.Fatalf there would be reported as a new failure, not a skip.
//
// 🔴 Every arm below is PER SITE. An aggregated arm passes with four of the five
// sites fixed, which is precisely the kind of green this REQ exists to refuse.
//
// 🔴 Every negative arm carries a CONTROL arm proving the happy path still
// writes. Without it, "nothing writes any more" satisfies the negative arm — the
// fixture degeneration ML-7A already paid for once in this REQ. note.go gets TWO
// control arms because the single pinned site covers TWO writes (os.WriteFile
// when the index is absent, os.OpenFile append when it is present).
//
// # What each test affirms (Regra Dura de Reconciliação, one sentence each)
//
//   - TestGeneratePomXMLRefusesUnverifiableRoot affirms this ML's conclusion that
//     java.go:77 was fail-open and is now fail-closed AND FATAL: with projectRoot()
//     failing, GeneratePomXML returns an error carrying the injected cause and
//     writes no pom.xml, while the clean arm still writes one.
//   - TestAppendNoteToIndexRefusesUnverifiableRoot affirms this ML's conclusion
//     that BOTH writes of note.go's appendNoteToIndex (create and append) were
//     fail-open and are now fail-closed AND FATAL: neither the index creation nor
//     the append happens when the root is unverifiable, while both still happen on
//     the clean arms.
//   - TestAppendTransitionLogIsNonFatalAndRefuses affirms this ML's conclusion that
//     roadmap.go:833 refuses without writing when the root is unverifiable AND that
//     its control flow is unchanged — the refusal is audible on stderr and the void
//     wrapper still returns normally, while the extracted entry point returns the
//     error that the wrapper deliberately discards.
//   - TestAppendREQTransitionLogIsNonFatalAndRefuses affirms the same conclusion for
//     req.go:478, the deliberate mirror of the roadmap log helper.
//
// 🔴 No t.Parallel in this file: it swaps the package-level getwdFn and os.Stderr,
// and it chdirs.

// errGetwdInjected is the cause every negative arm injects. Asserting
// errors.Is(err, errGetwdInjected) is what makes each arm fail for the reason it
// NAMES — an arm that only asserts "some error" is green when the fixture is
// broken.
var errGetwdInjected = errors.New("injected: getwd unavailable")

// failProjectRoot makes projectRoot() fail for the whole test, and verifies the
// seam actually took effect before the arm runs. Without this check a typo in
// the injection would leave the happy path running and the negative arm would
// measure nothing.
func failProjectRoot(t *testing.T) {
	t.Helper()
	original := getwdFn
	getwdFn = func() (string, error) { return "", errGetwdInjected }
	t.Cleanup(func() { getwdFn = original })
	if _, err := projectRoot(); !errors.Is(err, errGetwdInjected) {
		t.Fatalf("fixture broken: projectRoot() must fail with the injected cause, got %v", err)
	}
}

// assertUnverifiableRefusal checks that an error (or a stderr line) is the
// refusal of an UNVERIFIABLE ROOT over the expected target — not a symlink
// refusal, not an I/O error that happens to be non-nil.
func assertUnverifiableRefusal(t *testing.T, what, message, target string) {
	t.Helper()
	if !strings.Contains(message, "cannot verify containment") {
		t.Errorf("%s: refused for the WRONG reason — want the unverifiable-root refusal, got: %q", what, message)
	}
	if !strings.Contains(message, target) {
		t.Errorf("%s: the refusal does not name the artifact %q — a refusal that names only the rule is a false green, got: %q", what, target, message)
	}
}

func TestGeneratePomXMLRefusesUnverifiableRoot(t *testing.T) {
	// ── control arm: the happy path still writes ──────────────────────────
	clean := t.TempDir()
	t.Chdir(clean)
	if err := GeneratePomXML(Config{ProjectType: "backend", ProjectName: "control"}); err != nil {
		t.Fatalf("control arm: GeneratePomXML must still write on a clean tree, got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(clean, "pom.xml")); err != nil {
		t.Fatalf("control arm did not reach the write (%v) — the negative arm below would prove nothing", err)
	}

	// ── negative arm: projectRoot() fails, the write must NOT happen ───────
	refused := t.TempDir()
	t.Chdir(refused)
	failProjectRoot(t)

	var err error
	output := captureStderr(t, func() {
		err = GeneratePomXML(Config{ProjectType: "backend", ProjectName: "refused"})
	})

	if err == nil {
		t.Fatalf("GeneratePomXML wrote pom.xml with NO containment guard: projectRoot() failed and the write proceeded — this is the ML-9A defect")
	}
	if !errors.Is(err, errGetwdInjected) {
		t.Errorf("the refusal does not carry the injected cause (%v) — the arm may be failing for an unrelated reason", err)
	}
	assertUnverifiableRefusal(t, "returned error", err.Error(), "pom.xml")
	if !strings.Contains(output, singleGrammar) {
		t.Errorf("the refusal is not audible in the single grammar %q, got: %q", singleGrammar, output)
	}
	if _, statErr := os.Stat(filepath.Join(refused, "pom.xml")); statErr == nil {
		t.Errorf("containment violated: pom.xml was written while the root could not be established")
	}
}

func TestAppendNoteToIndexRefusesUnverifiableRoot(t *testing.T) {
	// ── control arm A: index ABSENT → os.WriteFile creates it ─────────────
	cleanCreate := t.TempDir()
	t.Chdir(cleanCreate)
	if err := os.MkdirAll(filepath.Join(cleanCreate, vaultDir), 0o755); err != nil {
		t.Fatalf("building the clean fixture: %v", err)
	}
	if err := appendNoteToIndex("nota-a.md"); err != nil {
		t.Fatalf("control arm A: appendNoteToIndex must still create the index, got: %v", err)
	}
	createdBytes, err := os.ReadFile(filepath.Join(cleanCreate, vaultIndexFile))
	if err != nil {
		t.Fatalf("control arm A did not reach the CREATE write (%v) — the negative arm would prove nothing about it", err)
	}
	if !strings.Contains(string(createdBytes), "nota-a.md") {
		t.Fatalf("control arm A created the index without the link — got: %q", string(createdBytes))
	}

	// ── control arm B: index PRESENT → os.OpenFile appends to it ──────────
	// 🔴 Two control arms, not one: the pinned site covers TWO write primitives,
	// and arm A alone leaves the append site with no proof that it still runs.
	if err := appendNoteToIndex("nota-b.md"); err != nil {
		t.Fatalf("control arm B: appendNoteToIndex must still append, got: %v", err)
	}
	appendedBytes, err := os.ReadFile(filepath.Join(cleanCreate, vaultIndexFile))
	if err != nil {
		t.Fatalf("control arm B: reading the index: %v", err)
	}
	if len(appendedBytes) <= len(createdBytes) || !strings.Contains(string(appendedBytes), "nota-b.md") {
		t.Fatalf("control arm B did not reach the APPEND write — index unchanged at %d byte(s): %q", len(appendedBytes), string(appendedBytes))
	}

	// ── negative arm 1: index ABSENT, root unverifiable → no CREATE ───────
	refusedCreate := t.TempDir()
	t.Chdir(refusedCreate)
	if err := os.MkdirAll(filepath.Join(refusedCreate, vaultDir), 0o755); err != nil {
		t.Fatalf("building the refused fixture: %v", err)
	}
	failProjectRoot(t)

	var createErr error
	createOutput := captureStderr(t, func() {
		createErr = appendNoteToIndex("nota-refusada.md")
	})
	if createErr == nil {
		t.Fatalf("appendNoteToIndex CREATED the index with no containment guard — this is the ML-9A defect (site 1 of 2)")
	}
	if !errors.Is(createErr, errGetwdInjected) {
		t.Errorf("the refusal does not carry the injected cause (%v)", createErr)
	}
	assertUnverifiableRefusal(t, "returned error (create)", createErr.Error(), vaultIndexFile)
	if !strings.Contains(createOutput, singleGrammar) {
		t.Errorf("the refusal is not audible in the single grammar %q, got: %q", singleGrammar, createOutput)
	}
	if _, statErr := os.Stat(filepath.Join(refusedCreate, vaultIndexFile)); statErr == nil {
		t.Errorf("containment violated: index.md was CREATED while the root could not be established")
	}

	// ── negative arm 2: index PRESENT, root unverifiable → no APPEND ──────
	existing := "# Vault\n\n## Índice\n\n"
	if err := os.WriteFile(filepath.Join(refusedCreate, vaultIndexFile), []byte(existing), 0o644); err != nil {
		t.Fatalf("seeding the existing index: %v", err)
	}
	var appendErr error
	appendOutput := captureStderr(t, func() {
		appendErr = appendNoteToIndex("nota-nao-anexada.md")
	})
	if appendErr == nil {
		t.Fatalf("appendNoteToIndex APPENDED with no containment guard — this is the ML-9A defect (site 2 of 2)")
	}
	assertUnverifiableRefusal(t, "returned error (append)", appendErr.Error(), vaultIndexFile)
	if !strings.Contains(appendOutput, singleGrammar) {
		t.Errorf("the append refusal is not audible in the single grammar %q, got: %q", singleGrammar, appendOutput)
	}
	after, err := os.ReadFile(filepath.Join(refusedCreate, vaultIndexFile))
	if err != nil {
		t.Fatalf("reading the seeded index: %v", err)
	}
	if string(after) != existing {
		t.Errorf("containment violated: index.md was APPENDED to while the root could not be established — got %q", string(after))
	}
}

func TestAppendTransitionLogIsNonFatalAndRefuses(t *testing.T) {
	logRelative := logPath()

	// ── control arm: the happy path still appends ─────────────────────────
	clean := t.TempDir()
	t.Chdir(clean)
	if err := os.MkdirAll(filepath.Dir(filepath.Join(clean, logRelative)), 0o755); err != nil {
		t.Fatalf("building the clean fixture: %v", err)
	}
	controlOutput := captureStderr(t, func() {
		appendTransitionLog("ROADMAP-control.md", "backlog", "wip")
	})
	if controlOutput != "" {
		t.Fatalf("clean tree must print nothing, got: %q", controlOutput)
	}
	if _, err := os.Stat(filepath.Join(clean, logRelative)); err != nil {
		t.Fatalf("control arm did not reach the write (%v) — the negative arm below would prove nothing", err)
	}

	// ── negative arm: root unverifiable → refuses, out loud, without writing
	refused := t.TempDir()
	t.Chdir(refused)
	if err := os.MkdirAll(filepath.Dir(filepath.Join(refused, logRelative)), 0o755); err != nil {
		t.Fatalf("building the refused fixture: %v", err)
	}
	failProjectRoot(t)

	output := captureStderr(t, func() {
		appendTransitionLog("ROADMAP-refusado.md", "backlog", "wip")
	})
	// Reaching this line at all is the NON-FATAL half: appendTransitionLog is void
	// and the only ways it could abort the move are panic or os.Exit, neither of
	// which returns here.
	if output == "" {
		t.Fatalf("appendTransitionLog appended the transition line with NO containment guard, silently: projectRoot() failed and the write proceeded — this is the ML-9A defect")
	}
	if !strings.Contains(output, singleGrammar) {
		t.Errorf("the refusal is not audible in the single grammar %q, got: %q", singleGrammar, output)
	}
	assertUnverifiableRefusal(t, "stderr", output, logRelative)
	if _, statErr := os.Stat(filepath.Join(refused, logRelative)); statErr == nil {
		t.Errorf("containment violated: the transition log was written while the root could not be established")
	}

	// ── the discriminant: WHICH control flow is preserved ─────────────────
	// The extracted entry point RETURNS the refusal (so it composes like every
	// other guarded write), and the void wrapper DISCARDS it by decision. That
	// asymmetry is the "fatal vs non-fatal" claim, asserted rather than assumed.
	entryErr := appendTransitionLogEntry("ROADMAP-refusado.md", "backlog", "wip")
	if entryErr == nil {
		t.Fatalf("appendTransitionLogEntry must RETURN the refusal — a void-only refusal cannot be composed and cannot be asserted on")
	}
	if !errors.Is(entryErr, errGetwdInjected) {
		t.Errorf("the returned refusal does not carry the injected cause (%v)", entryErr)
	}
}

func TestAppendREQTransitionLogIsNonFatalAndRefuses(t *testing.T) {
	logRelative := filepath.Join(config.Load().REQDir, ".trackfw-log")

	clean := t.TempDir()
	t.Chdir(clean)
	if err := os.MkdirAll(filepath.Dir(filepath.Join(clean, logRelative)), 0o755); err != nil {
		t.Fatalf("building the clean fixture: %v", err)
	}
	controlOutput := captureStderr(t, func() {
		appendREQTransitionLog("REQ-control.md", "backlog", "wip")
	})
	if controlOutput != "" {
		t.Fatalf("clean tree must print nothing, got: %q", controlOutput)
	}
	if _, err := os.Stat(filepath.Join(clean, logRelative)); err != nil {
		t.Fatalf("control arm did not reach the write (%v) — the negative arm below would prove nothing", err)
	}

	refused := t.TempDir()
	t.Chdir(refused)
	if err := os.MkdirAll(filepath.Dir(filepath.Join(refused, logRelative)), 0o755); err != nil {
		t.Fatalf("building the refused fixture: %v", err)
	}
	failProjectRoot(t)

	output := captureStderr(t, func() {
		appendREQTransitionLog("REQ-refusada.md", "backlog", "wip")
	})
	if output == "" {
		t.Fatalf("appendREQTransitionLog appended the transition line with NO containment guard, silently — this is the ML-9A defect")
	}
	if !strings.Contains(output, singleGrammar) {
		t.Errorf("the refusal is not audible in the single grammar %q, got: %q", singleGrammar, output)
	}
	assertUnverifiableRefusal(t, "stderr", output, logRelative)
	if _, statErr := os.Stat(filepath.Join(refused, logRelative)); statErr == nil {
		t.Errorf("containment violated: the REQ transition log was written while the root could not be established")
	}

	entryErr := appendREQTransitionLogEntry("REQ-refusada.md", "backlog", "wip")
	if entryErr == nil {
		t.Fatalf("appendREQTransitionLogEntry must RETURN the refusal — the mirror of appendTransitionLogEntry, same shape")
	}
	if !errors.Is(entryErr, errGetwdInjected) {
		t.Errorf("the returned refusal does not carry the injected cause (%v)", entryErr)
	}
}
