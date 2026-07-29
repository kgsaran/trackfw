package commands

// barrier_test.go — unit tests for the string-level parser and per-check evaluation
// logic in barrier.go, additional to the cross-runtime contract fixed in
// barrier_contract_test.go (which drives the real compiled binary end-to-end).
// These tests call the unexported parsing helpers directly and do not build a binary.

import (
	"strings"
	"testing"
)

func TestParseWaves_SingleWave(t *testing.T) {
	content := "# Roadmap\n\n## Wave 1 — Foo\nbody line\n\n### ML-1A — x\n**Status:** ✅\n"
	lines := strings.Split(content, "\n")
	waves, uerr := parseWaves(lines)
	if uerr != nil {
		t.Fatalf("unexpected usage error: %v", uerr)
	}
	if len(waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(waves))
	}
	if waves[0].number != 1 {
		t.Fatalf("expected wave number 1, got %d", waves[0].number)
	}
}

func TestParseWaves_MultipleWavesEndAtNextH2(t *testing.T) {
	content := strings.Join([]string{
		"# Roadmap",
		"",
		"## Wave 1 — Foo",
		"content of wave 1",
		"",
		"## Wave 2 — Bar",
		"content of wave 2",
	}, "\n")
	lines := strings.Split(content, "\n")
	waves, uerr := parseWaves(lines)
	if uerr != nil {
		t.Fatalf("unexpected usage error: %v", uerr)
	}
	if len(waves) != 2 {
		t.Fatalf("expected 2 waves, got %d", len(waves))
	}
	if waves[0].number != 1 || waves[1].number != 2 {
		t.Fatalf("unexpected wave numbers: %+v", waves)
	}
	// Wave 1 block must end exactly where the "## Wave 2" heading starts.
	if lines[waves[0].end] != "## Wave 2 — Bar" {
		t.Fatalf("expected wave 1 to end at 'Wave 2' heading, got %q", lines[waves[0].end])
	}
}

func TestParseWaves_MalformedNumberIsUsageError(t *testing.T) {
	content := "## Wave x — Foo\nbody\n"
	lines := strings.Split(content, "\n")
	waves, uerr := parseWaves(lines)
	if uerr == nil {
		t.Fatalf("expected usage error for malformed wave number, got waves=%+v", waves)
	}
	if !strings.Contains(uerr.Error(), "line 1") {
		t.Fatalf("expected error to name line 1, got: %s", uerr.Error())
	}
}

func TestParseMLs_MultipleMLsInWave(t *testing.T) {
	content := strings.Join([]string{
		"## Wave 1 — Foo",
		"### ML-1A — First",
		"**Status:** ✅",
		"### ML-1B — Second",
		"**Status:** ⬜ Pendente",
	}, "\n")
	lines := strings.Split(content, "\n")
	mls := parseMLs(lines, 0, len(lines))
	if len(mls) != 2 {
		t.Fatalf("expected 2 MLs, got %d", len(mls))
	}
	if mls[0].id != "ML-1A" || mls[1].id != "ML-1B" {
		t.Fatalf("unexpected ML ids: %+v", mls)
	}
}

func TestMLStatusMarker_MissingLine(t *testing.T) {
	content := "### ML-1A — Foo\nno status line here\n"
	lines := strings.Split(content, "\n")
	mls := parseMLs(lines, 0, len(lines))
	if len(mls) != 1 {
		t.Fatalf("expected 1 ML, got %d", len(mls))
	}
	_, found := mlStatusMarker(lines, mls[0])
	if found {
		t.Fatal("expected found=false when no **Status:** line is present")
	}
}

func TestAcceptanceEvaluate_AllMet(t *testing.T) {
	content := strings.Join([]string{
		"### ML-1A — Foo",
		"**Status:** ✅",
		"**Critérios de aceite:**",
		"- [x] build passes",
		"- [x] tests pass",
	}, "\n")
	lines := strings.Split(content, "\n")
	mls := parseMLs(lines, 0, len(lines))
	met, unmet, hasBlock := acceptanceEvaluate(lines, mls[0])
	if !hasBlock {
		t.Fatal("expected hasBlock=true")
	}
	if unmet != 0 {
		t.Fatalf("expected 0 unmet, got %d", unmet)
	}
	if met != 2 {
		t.Fatalf("expected 2 met, got %d", met)
	}
}

func TestAcceptanceEvaluate_EmptyBlockIsNotVacuouslyPassed(t *testing.T) {
	content := strings.Join([]string{
		"### ML-1A — Foo",
		"**Status:** ✅",
		"**Critérios de aceite:**",
		"**Files affected:**",
	}, "\n")
	lines := strings.Split(content, "\n")
	mls := parseMLs(lines, 0, len(lines))
	_, _, hasBlock := acceptanceEvaluate(lines, mls[0])
	if hasBlock {
		t.Fatal("expected hasBlock=false for an empty acceptance block (anti-vacuity)")
	}
}

func TestAcceptanceEvaluate_NoHeaderAtAll(t *testing.T) {
	content := strings.Join([]string{
		"### ML-1A — Foo",
		"**Status:** ✅",
	}, "\n")
	lines := strings.Split(content, "\n")
	mls := parseMLs(lines, 0, len(lines))
	_, _, hasBlock := acceptanceEvaluate(lines, mls[0])
	if hasBlock {
		t.Fatal("expected hasBlock=false when no **Critérios de aceite:** header exists")
	}
}

func TestParseGates_NoBlockYieldsEmptyNonNilSlice(t *testing.T) {
	content := "## Wave 1 — Foo\nno gates here\n"
	lines := strings.Split(content, "\n")
	cmds, uerr := parseGates(lines, 0, len(lines))
	if uerr != nil {
		t.Fatalf("unexpected usage error: %v", uerr)
	}
	if cmds == nil {
		t.Fatal("expected non-nil empty slice for a wave with no gates block")
	}
	if len(cmds) != 0 {
		t.Fatalf("expected 0 commands, got %v", cmds)
	}
}

func TestParseGates_ParsesCommandsIgnoringBlankAndComment(t *testing.T) {
	content := strings.Join([]string{
		"## Wave 1 — Foo",
		"**Gates da wave:**",
		"```bash",
		"go build ./...",
		"",
		"# a comment",
		"go test ./...",
		"```",
	}, "\n")
	lines := strings.Split(content, "\n")
	cmds, uerr := parseGates(lines, 0, len(lines))
	if uerr != nil {
		t.Fatalf("unexpected usage error: %v", uerr)
	}
	want := []string{"go build ./...", "go test ./..."}
	if len(cmds) != len(want) {
		t.Fatalf("expected %v, got %v", want, cmds)
	}
	for i := range want {
		if cmds[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, cmds)
		}
	}
}

func TestParseGates_UnterminatedFenceIsUsageError(t *testing.T) {
	content := strings.Join([]string{
		"## Wave 1 — Foo",
		"**Gates da wave:**",
		"```bash",
		"go build ./...",
	}, "\n")
	lines := strings.Split(content, "\n")
	_, uerr := parseGates(lines, 0, len(lines))
	if uerr == nil {
		t.Fatal("expected usage error for unterminated fence")
	}
}

func TestParseGates_MissingFenceRightAfterHeaderIsUsageError(t *testing.T) {
	content := strings.Join([]string{
		"## Wave 1 — Foo",
		"**Gates da wave:**",
		"go build ./...",
	}, "\n")
	lines := strings.Split(content, "\n")
	_, uerr := parseGates(lines, 0, len(lines))
	if uerr == nil {
		t.Fatal("expected usage error when no ```bash fence immediately follows the gates header")
	}
}

func TestRunGateCommand_ExitCodes(t *testing.T) {
	if code := runGateCommand("true"); code != 0 {
		t.Fatalf("expected exit 0 for 'true', got %d", code)
	}
	if code := runGateCommand("false"); code != 1 {
		t.Fatalf("expected exit 1 for 'false', got %d", code)
	}
	if code := runGateCommand("exit 7"); code != 7 {
		t.Fatalf("expected exit 7 for 'exit 7', got %d", code)
	}
}
