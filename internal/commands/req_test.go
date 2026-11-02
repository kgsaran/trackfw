package commands

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/validator"
)

// TestRunReqNew_NoTTY_NoWizardNoADRDrafts — sem TTY (ambiente de teste, stdin não é terminal),
// runReqNew não abre wizard nenhum (nem o prompt de escopo de ADR) e não gera ADR drafts.
// ROADMAP-2026-08-08 ML-2A.
//
// 🔴 Renomeado no ML-4A (REQ-2026-09-09): o nome anterior era "BehaviorUnchanged", e a partir deste
// ML o comportamento MUDOU de propósito — `req new` passa a criar o roadmap no mesmo ato. O que este
// teste sempre mediu, e continua medindo, é a ausência de interação sem TTY; a asserção de roadmap
// vive em req_chain_ml4a_test.go. Este teste passa `--no-roadmap` para isolar o que ele afirma.
func TestRunReqNew_NoTTY_NoWizardNoADRDrafts(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	if err := runReqNew(nil, []string{"Estrategia de Autenticacao"}, "", true); err != nil {
		t.Fatalf("runReqNew erro inesperado (sem TTY não deveria acionar wizard): %v", err)
	}

	matches, err := filepath.Glob(filepath.Join("docs", "req", "REQ-*.md"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("esperava 1 REQ criada, encontrou %d", len(matches))
	}

	// Sem TTY não deve haver geração de ADR drafts (nenhuma probe é processada nesse caminho).
	adrMatches, _ := filepath.Glob(filepath.Join("docs", "adr", "ADR-*.md"))
	if len(adrMatches) != 0 {
		t.Errorf("não esperava ADR drafts sem TTY, encontrou %d", len(adrMatches))
	}
}

// chdirWithADR sets up a temp project directory with a single Accepted ADR file.
// It chdirs into the directory, calls config.Reset(), and registers cleanup.
// Returns the ADR basename (e.g. "ADR-2026-11-02-user-auth.md").
func chdirWithADR(t *testing.T, adrTitle string) string {
	t.Helper()
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	config.Reset()
	t.Cleanup(func() {
		_ = os.Chdir(orig)
		config.Reset()
	})

	if err := os.MkdirAll("docs/adr", 0755); err != nil {
		t.Fatalf("MkdirAll docs/adr: %v", err)
	}

	slug := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(adrTitle, " ", "-"), "_", "-"))
	date := time.Now().Format("2006-01-02")
	basename := "ADR-" + date + "-" + slug + ".md"
	content := "---\nstatus: Accepted\ndate: " + date + "\n---\n\n# ADR: " + adrTitle + "\n"
	if err := os.WriteFile("docs/adr/"+basename, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile ADR: %v", err)
	}
	return basename
}

// reqHasADRViolation calls ValidateTagged and returns true when rule req_has_adr fires for reqBase.
// Checks both violations and warnings because severity is governed separately.
func reqHasADRViolation(t *testing.T, reqBase string) bool {
	t.Helper()
	violations, warnings, err := validator.ValidateTagged()
	if err != nil {
		t.Fatalf("ValidateTagged: %v", err)
	}
	for _, m := range append(append([]validator.TaggedMsg{}, violations...), warnings...) {
		if m.Rule == "req_has_adr" && strings.Contains(m.Msg, reqBase) {
			return true
		}
	}
	return false
}

// TestReqNew_NoAdrFlag_StructureUnchanged — without --adr, the generated REQ has
// adr: "" in frontmatter and an empty ADR: line in the body (byte-identical to the
// pre-ML-1B template). Asserts: AC3 negative path does not change existing behavior.
func TestReqNew_NoAdrFlag_StructureUnchanged(t *testing.T) {
	chdirWithADR(t, "user-auth") // ADR present but not linked
	if err := runReqNewFull(nil, []string{"Scope Guard"}, "", true, ""); err != nil {
		t.Fatalf("runReqNewFull: %v", err)
	}
	matches, _ := filepath.Glob(filepath.Join("docs", "req", "REQ-*.md"))
	if len(matches) != 1 {
		t.Fatalf("expected 1 REQ, got %d", len(matches))
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	body := string(data)
	// Frontmatter must have adr: "" (empty quoted value).
	if !strings.Contains(body, `adr: ""`) {
		t.Errorf("frontmatter adr: field is not empty-quoted; got:\n%s", body)
	}
	// Body must have ADR: with nothing after it (empty line).
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "ADR:") {
			after := strings.TrimPrefix(line, "ADR:")
			if strings.TrimSpace(after) != "" {
				t.Errorf("body ADR: line should be empty without --adr, got: %q", line)
			}
			return
		}
	}
	t.Errorf("body ADR: line not found in:\n%s", body)
}

// TestReqNew_AdrFlag_LinksADR — valid --adr value: REQ frontmatter has adr: "<path>",
// body has ADR: <path>, and req_has_adr does NOT fire after roadmap is in wip.
// Asserts: AC3 positive path links the ADR and satisfies req_has_adr.
func TestReqNew_AdrFlag_LinksADR(t *testing.T) {
	basename := chdirWithADR(t, "user-auth-strategy")

	if err := runReqNewFull(nil, []string{"My REQ"}, "", false, basename); err != nil {
		t.Fatalf("runReqNewFull --adr: %v", err)
	}

	matches, _ := filepath.Glob(filepath.Join("docs", "req", "REQ-*.md"))
	if len(matches) != 1 {
		t.Fatalf("expected 1 REQ, got %d", len(matches))
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	body := string(data)

	// Frontmatter adr: field must be a non-empty quoted path ending in .md.
	if !strings.Contains(body, `adr: "docs/adr/`+basename+`"`) {
		t.Errorf("frontmatter adr: field not linked; got body:\n%s", body)
	}
	// Body ADR: line must contain the same path.
	found := false
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "ADR:") {
			after := strings.TrimSpace(strings.TrimPrefix(line, "ADR:"))
			if after == "docs/adr/"+basename {
				found = true
			} else {
				t.Errorf("body ADR: line mismatch; got %q, want %q", after, "docs/adr/"+basename)
			}
			break
		}
	}
	if !found {
		t.Errorf("body ADR: line not found or empty in:\n%s", body)
	}

	// Move roadmap to wip so req_has_adr fires in the full validate context.
	roadmaps, _ := filepath.Glob(filepath.Join("docs", "roadmaps", "backlog", "ROADMAP-*.md"))
	if len(roadmaps) != 1 {
		t.Fatalf("expected 1 backlog roadmap, got %d: %v", len(roadmaps), roadmaps)
	}
	root := newRootCmd()
	root.SilenceErrors = true
	root.SetArgs([]string{"roadmap", "move", filepath.Base(roadmaps[0]), "wip"})
	if err := root.Execute(); err != nil {
		t.Fatalf("roadmap move wip: %v", err)
	}

	// Counter-arm: req_has_adr must NOT fire when ADR link is present.
	reqBase := filepath.Base(matches[0])
	if reqHasADRViolation(t, reqBase) {
		data2, _ := os.ReadFile(matches[0])
		t.Errorf("req_has_adr fired even though ADR link is present:\n%s", data2)
	}
}

// TestReqNew_AdrFlag_NotFound — --adr with a name that matches no ADR file returns an
// error mentioning "not found" and creates no REQ file.
// Asserts: AC3 error path (ADR absent) is fatal and leaves no partial artifact.
func TestReqNew_AdrFlag_NotFound(t *testing.T) {
	chdirWithADR(t, "other-adr") // creates a different ADR — no match for "ghost-adr"

	err := runReqNewFull(nil, []string{"My REQ"}, "", false, "ghost-adr")
	if err == nil {
		t.Fatal("expected error for non-existent ADR, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}

	// No REQ file must have been created.
	matches, _ := filepath.Glob(filepath.Join("docs", "req", "REQ-*.md"))
	if len(matches) != 0 {
		t.Errorf("expected 0 REQs created on error, got %d: %v", len(matches), matches)
	}
}

// TestReqNew_AdrFlag_Ambiguous — --adr matching two ADR files returns an error
// mentioning "ambiguous" and creates no REQ file.
// Asserts: AC3 error path (ambiguous) is fatal and leaves no partial artifact.
func TestReqNew_AdrFlag_Ambiguous(t *testing.T) {
	chdirWithADR(t, "user-auth") // creates docs/adr/ADR-<date>-user-auth.md

	// Create a second ADR in a subdirectory so basename matches the same bare name.
	if err := os.MkdirAll("docs/adr/done", 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	date := time.Now().Format("2006-01-02")
	dup := "docs/adr/done/ADR-" + date + "-user-auth.md"
	if err := os.WriteFile(dup, []byte("---\nstatus: Accepted\n---\n# ADR: dup\n"), 0644); err != nil {
		t.Fatalf("WriteFile dup: %v", err)
	}

	// But reachable via adr_dirs only if the default includes done/ — check by adding
	// a trackfw.yaml that lists both adr_dirs entries.
	if err := os.WriteFile("trackfw.yaml", []byte("adr_dirs:\n  - docs/adr\n  - docs/adr/done\n"), 0644); err != nil {
		t.Fatalf("WriteFile trackfw.yaml: %v", err)
	}
	config.Reset()

	err := runReqNewFull(nil, []string{"My REQ"}, "", false, "ADR-"+date+"-user-auth")
	if err == nil {
		t.Fatal("expected error for ambiguous ADR, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "ambiguous") {
		t.Errorf("error should mention 'ambiguous', got: %v", err)
	}
	matches, _ := filepath.Glob(filepath.Join("docs", "req", "REQ-*.md"))
	if len(matches) != 0 {
		t.Errorf("expected 0 REQs on ambiguous error, got %d: %v", len(matches), matches)
	}
}

// TestReqNew_AdrFlag_WithSlash — slash in --adr value is rejected before any file is created.
// Asserts: AC3 traversal guard (P5) fires on "/".
func TestReqNew_AdrFlag_WithSlash(t *testing.T) {
	chdirWithADR(t, "user-auth")

	err := runReqNewFull(nil, []string{"My REQ"}, "", false, "docs/adr/ADR-any.md")
	if err == nil {
		t.Fatal("expected error for slash in --adr, got nil")
	}
	if !strings.Contains(err.Error(), "invalid character") && !strings.Contains(err.Error(), "not allowed") {
		t.Errorf("error should mention invalid character, got: %v", err)
	}
	matches, _ := filepath.Glob(filepath.Join("docs", "req", "REQ-*.md"))
	if len(matches) != 0 {
		t.Errorf("expected 0 REQs on slash error, got %d", len(matches))
	}
}

// TestReqNew_AdrFlag_WithDotDot — exact ".." in --adr value is rejected as path traversal.
// Asserts: AC3 traversal guard (P5) fires on "..".
func TestReqNew_AdrFlag_WithDotDot(t *testing.T) {
	chdirWithADR(t, "user-auth")

	err := runReqNewFull(nil, []string{"My REQ"}, "", false, "..")
	if err == nil {
		t.Fatal("expected error for '..' in --adr, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "traversal") {
		t.Errorf("error should mention 'traversal', got: %v", err)
	}
	matches, _ := filepath.Glob(filepath.Join("docs", "req", "REQ-*.md"))
	if len(matches) != 0 {
		t.Errorf("expected 0 REQs on traversal error, got %d", len(matches))
	}
}

// TestReqNew_AdrFlag_WithNewline — newline in --adr value is rejected before any file is created.
// Asserts: AC3 traversal guard (P5) fires on control characters.
func TestReqNew_AdrFlag_WithNewline(t *testing.T) {
	chdirWithADR(t, "user-auth")

	err := runReqNewFull(nil, []string{"My REQ"}, "", false, "ADR-valid\nmalicious")
	if err == nil {
		t.Fatal("expected error for newline in --adr, got nil")
	}
	if !strings.Contains(err.Error(), "invalid character") && !strings.Contains(err.Error(), "not allowed") {
		t.Errorf("error should mention invalid character, got: %v", err)
	}
	matches, _ := filepath.Glob(filepath.Join("docs", "req", "REQ-*.md"))
	if len(matches) != 0 {
		t.Errorf("expected 0 REQs on newline error, got %d", len(matches))
	}
}

// TestReqNew_AdrFlag_CaseMismatch — lowercase --adr arg against an uppercase ADR filename
// returns "not found" (P6: string-exact comparison, not case-folded).
// Asserts: AC3 exact-match semantics are enforced, not case-insensitive.
func TestReqNew_AdrFlag_CaseMismatch(t *testing.T) {
	// chdirWithADR creates e.g. "ADR-2026-11-02-user-auth.md" (lowercase slug).
	basename := chdirWithADR(t, "user-auth")
	lower := strings.ToLower(basename)
	// If the basename is already lowercase (typical), inject an uppercase variant.
	if lower == basename {
		// Rename to uppercase so the original is now gone and only UPPERCASE exists.
		upper := strings.ToUpper(basename[:3]) + basename[3:]
		if err := os.Rename("docs/adr/"+basename, "docs/adr/"+upper); err != nil {
			t.Fatalf("rename to uppercase: %v", err)
		}
		// Requesting the lowercase name should now return not-found.
		err := runReqNewFull(nil, []string{"My REQ"}, "", false, lower)
		if err == nil {
			t.Fatal("expected not-found for lowercase arg against uppercase ADR, got nil")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "not found") {
			t.Errorf("expected 'not found', got: %v", err)
		}
		return
	}
	// baseline: if the basename itself had uppercase letters, requesting all-lowercase is the mismatch.
	err := runReqNewFull(nil, []string{"My REQ"}, "", false, lower)
	if err == nil {
		t.Fatal("expected not-found for case-folded arg, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "not found") {
		t.Errorf("expected 'not found', got: %v", err)
	}
}

// captureStderr redirects os.Stderr to a pipe, calls fn, and returns everything written.
// The pipe is closed before draining, so the copy never blocks.
func captureStderr(fn func()) string {
	r, w, err := os.Pipe()
	if err != nil {
		return ""
	}
	oldStderr := os.Stderr
	os.Stderr = w
	fn()
	w.Close()
	os.Stderr = oldStderr
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

// TestReqNew_AC3b_ExistingREQKeptIntactAndRelinked — AC3b contract after architect decision:
// when req new runs again with the same title, the REQ is kept byte-for-byte intact (not rewritten),
// the roadmap link is repaired idempotently, and the command exits 0 with a stderr warning.
//
// Arm 1 (plain rerun — relink is a no-op): REQ already has roadmap linked; relink writes nothing;
//   file stays byte-identical to what was there before the second run, including the MARCA sentinel.
//
// Arm 2 (--no-roadmap then default — relink writes): REQ has roadmap: ""; rerun links it;
//   any content appended to the REQ after the first run (the MARCA sentinel) must survive the write.
func TestReqNew_AC3b_ExistingREQKeptIntactAndRelinked(t *testing.T) {
	t.Run("arm1: plain-rerun is no-op — file stays byte-identical including MARCA", func(t *testing.T) {
		chdirWithADR(t, "user-auth")

		// First run creates REQ + roadmap (roadmap: field backlinked).
		if err := runReqNewFull(nil, []string{"Keep Guard REQ"}, "", false, ""); err != nil {
			t.Fatalf("1st run: %v", err)
		}
		reqMatches, _ := filepath.Glob(filepath.Join("docs", "req", "REQ-*.md"))
		if len(reqMatches) != 1 {
			t.Fatalf("expected 1 REQ after 1st run, got %d", len(reqMatches))
		}
		reqPath := reqMatches[0]

		// Append a distinctive marker.
		const marca = "\n<!-- AC3b MARCA sentinel -->"
		f, err := os.OpenFile(reqPath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			t.Fatalf("OpenFile: %v", err)
		}
		_, _ = f.WriteString(marca)
		_ = f.Close()

		before, err := os.ReadFile(reqPath)
		if err != nil {
			t.Fatalf("ReadFile before 2nd run: %v", err)
		}

		// Second run must exit 0 and print a stderr warning.
		var afterErr error
		stderr := captureStderr(func() {
			afterErr = runReqNewFull(nil, []string{"Keep Guard REQ"}, "", false, "")
		})
		if afterErr != nil {
			t.Fatalf("2nd run should exit 0 (AC3b), got err: %v", afterErr)
		}
		if !strings.Contains(stderr, "already exists") {
			t.Errorf("expected stderr warning containing 'already exists', got: %q", stderr)
		}

		// File must be byte-identical — no write happened (relink is a no-op when already linked).
		after, err := os.ReadFile(reqPath)
		if err != nil {
			t.Fatalf("ReadFile after 2nd run: %v", err)
		}
		if !bytes.Equal(before, after) {
			t.Errorf("REQ was modified by 2nd run (AC3b: must stay byte-identical):\nbefore=%q\nafter=%q",
				before, after)
		}

		// Roadmap count must remain 1 (no duplicate).
		roadmaps, _ := filepath.Glob(filepath.Join("docs", "roadmaps", "backlog", "ROADMAP-*.md"))
		if len(roadmaps) != 1 {
			t.Errorf("expected 1 roadmap after 2nd run, got %d: %v", len(roadmaps), roadmaps)
		}
	})

	t.Run("arm2: --no-roadmap then default — relink writes but MARCA survives", func(t *testing.T) {
		chdirWithADR(t, "user-auth")

		// First run with --no-roadmap: REQ created with roadmap: "" (no backlink yet).
		if err := runReqNewFull(nil, []string{"Keep Guard REQ Relink"}, "", true, ""); err != nil {
			t.Fatalf("1st run --no-roadmap: %v", err)
		}
		reqMatches, _ := filepath.Glob(filepath.Join("docs", "req", "REQ-*.md"))
		if len(reqMatches) != 1 {
			t.Fatalf("expected 1 REQ after 1st run, got %d", len(reqMatches))
		}
		reqPath := reqMatches[0]

		// Append MARCA while roadmap: "" is still empty.
		const marca = "\n<!-- AC3b MARCA relink sentinel -->"
		f, err := os.OpenFile(reqPath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			t.Fatalf("OpenFile: %v", err)
		}
		_, _ = f.WriteString(marca)
		_ = f.Close()

		// Second run without --no-roadmap: REQ is kept; chainRoadmapForREQ creates the roadmap
		// and relinks (writes roadmap: path into the existing REQ file).
		var afterErr error
		stderr := captureStderr(func() {
			afterErr = runReqNewFull(nil, []string{"Keep Guard REQ Relink"}, "", false, "")
		})
		if afterErr != nil {
			t.Fatalf("2nd run should exit 0 (AC3b), got err: %v", afterErr)
		}
		if !strings.Contains(stderr, "already exists") {
			t.Errorf("expected stderr warning containing 'already exists', got: %q", stderr)
		}

		after, err := os.ReadFile(reqPath)
		if err != nil {
			t.Fatalf("ReadFile after relink: %v", err)
		}
		// The MARCA must survive the surgical roadmap: write.
		if !strings.Contains(string(after), marca) {
			t.Errorf("MARCA was destroyed by relink (AC3b: relink must preserve non-marker content):\n%s",
				string(after))
		}
		// The REQ must now be linked (not orphaned).
		if reqHasRoadmapNames(t, filepath.Base(reqPath)) {
			data, _ := os.ReadFile(reqPath)
			t.Errorf("REQ still orphaned after relink:\n%s", data)
		}
	})
}

// TestReqNew_AC3b_ExistingREQ_ConflictingADR_Refuses — AC3b contract item (b):
// when req new --adr is called on a REQ that already exists with a different (or absent) ADR link,
// the command refuses with a non-zero exit and leaves the REQ byte-identical.
// Asserts: AC3b item (b) — --adr on existing REQ with mismatched adr: is fatal and leaves no change.
func TestReqNew_AC3b_ExistingREQ_ConflictingADR_Refuses(t *testing.T) {
	t.Run("existing REQ with no ADR + --adr supplied", func(t *testing.T) {
		basename := chdirWithADR(t, "user-auth-strategy")

		// First run without --adr: REQ created with adr: "".
		if err := runReqNewFull(nil, []string{"ADR Conflict REQ"}, "", false, ""); err != nil {
			t.Fatalf("1st run: %v", err)
		}
		reqMatches, _ := filepath.Glob(filepath.Join("docs", "req", "REQ-*.md"))
		if len(reqMatches) != 1 {
			t.Fatalf("expected 1 REQ, got %d", len(reqMatches))
		}
		before, err := os.ReadFile(reqMatches[0])
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}

		// Second run with --adr must refuse (existing adr: "" != requested ADR path).
		err = runReqNewFull(nil, []string{"ADR Conflict REQ"}, "", false, basename)
		if err == nil {
			t.Fatal("expected error when --adr conflicts with existing REQ adr:, got nil")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("error should mention 'already exists', got: %v", err)
		}

		after, err := os.ReadFile(reqMatches[0])
		if err != nil {
			t.Fatalf("ReadFile after refused run: %v", err)
		}
		if !bytes.Equal(before, after) {
			t.Errorf("REQ was modified by refused run (must be byte-identical)")
		}
	})

	t.Run("existing REQ with ADR-A + --adr ADR-B supplied", func(t *testing.T) {
		basename := chdirWithADR(t, "user-auth-strategy")

		// Create a second ADR.
		date := time.Now().Format("2006-01-02")
		secondBasename := "ADR-" + date + "-second-decision.md"
		if err := os.WriteFile("docs/adr/"+secondBasename,
			[]byte("---\nstatus: Accepted\ndate: "+date+"\n---\n\n# ADR: Second Decision\n"), 0644); err != nil {
			t.Fatalf("write second ADR: %v", err)
		}

		// First run with --adr basename (first ADR), --no-roadmap to keep fixture simple.
		if err := runReqNewFull(nil, []string{"ADR Swap REQ"}, "", true, basename); err != nil {
			t.Fatalf("1st run: %v", err)
		}
		reqMatches, _ := filepath.Glob(filepath.Join("docs", "req", "REQ-*.md"))
		before, _ := os.ReadFile(reqMatches[0])

		// Second run with --adr secondBasename (different ADR) must refuse.
		err := runReqNewFull(nil, []string{"ADR Swap REQ"}, "", true, secondBasename)
		if err == nil {
			t.Fatal("expected error when --adr differs from existing, got nil")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("error should mention 'already exists', got: %v", err)
		}
		after, _ := os.ReadFile(reqMatches[0])
		if !bytes.Equal(before, after) {
			t.Errorf("REQ was modified by refused run (must be byte-identical)")
		}
	})
}

// TestReqNew_AC3b_ExistingREQ_SameADR_Succeeds — AC3b contract item (a):
// when a REQ already has adr: <X> and req new --adr <X> is called again, the command exits 0
// and the REQ stays byte-identical (same-ADR rerun is treated as a no-op on the file).
// Asserts: AC3b item (a) — --adr matching existing link is idempotent.
func TestReqNew_AC3b_ExistingREQ_SameADR_Succeeds(t *testing.T) {
	basename := chdirWithADR(t, "user-auth-strategy")

	// First run with --adr basename.
	if err := runReqNewFull(nil, []string{"Same ADR REQ"}, "", false, basename); err != nil {
		t.Fatalf("1st run: %v", err)
	}
	reqMatches, _ := filepath.Glob(filepath.Join("docs", "req", "REQ-*.md"))
	if len(reqMatches) != 1 {
		t.Fatalf("expected 1 REQ, got %d", len(reqMatches))
	}
	before, err := os.ReadFile(reqMatches[0])
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// Second run with the same --adr must exit 0 and print a warning.
	var afterErr error
	stderr := captureStderr(func() {
		afterErr = runReqNewFull(nil, []string{"Same ADR REQ"}, "", false, basename)
	})
	if afterErr != nil {
		t.Fatalf("2nd run with same --adr should exit 0, got: %v", afterErr)
	}
	if !strings.Contains(stderr, "already exists") {
		t.Errorf("expected stderr warning containing 'already exists', got: %q", stderr)
	}

	after, err := os.ReadFile(reqMatches[0])
	if err != nil {
		t.Fatalf("ReadFile after 2nd run: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("REQ was modified by 2nd run with same --adr (must be byte-identical):\nbefore=%q\nafter=%q",
			before, after)
	}
}
