package guard

import (
	"io"
	"strings"
	"testing"
	"time"
)

// testTimingWindow is the window used for timing-sensitive DrainStdin tests and
// for TestRunGitBranch_EarlyEOFAllow. It is larger than testIdleTimeout (50ms) to
// give sufficient margin under -race (≥100ms between sleep boundaries).
const testTimingWindow = 200 * time.Millisecond

// timedEOFReader sends data on the first Read call, then sleeps eofDelay before
// returning EOF on every subsequent call. It simulates a writer that closes its
// end of the pipe after eofDelay (e.g. "(payload; sleep 3) |").
type timedEOFReader struct {
	data     []byte
	sent     bool
	eofDelay time.Duration
}

func (r *timedEOFReader) Read(p []byte) (int, error) {
	if !r.sent {
		r.sent = true
		n := copy(p, r.data)
		return n, nil
	}
	time.Sleep(r.eofDelay)
	return 0, io.EOF
}

// chunkedReader delivers data as discrete chunks with a constant inter-chunk
// delay (starting from the second chunk). After all chunks it returns EOF.
type chunkedReader struct {
	chunks [][]byte
	idx    int
	delay  time.Duration
}

func (r *chunkedReader) Read(p []byte) (int, error) {
	if r.idx >= len(r.chunks) {
		return 0, io.EOF
	}
	if r.idx > 0 {
		time.Sleep(r.delay)
	}
	n := copy(p, r.chunks[r.idx])
	r.idx++
	return n, nil
}

// delayReader is a test helper that sends data on first Read, then blocks
// until eofCh is closed (returning EOF) or doneCh is closed (also EOF).
// This avoids real time.Sleep while simulating the late-EOF cases.
type delayReader struct {
	data  []byte
	sent  bool
	eofCh chan struct{}
}

func (r *delayReader) Read(p []byte) (int, error) {
	if !r.sent {
		r.sent = true
		n := copy(p, r.data)
		return n, nil
	}
	<-r.eofCh
	return 0, io.EOF
}

// newDelayReader creates a delayReader. Call close(eofCh) to signal EOF.
func newDelayReader(data []byte) (*delayReader, chan struct{}) {
	ch := make(chan struct{})
	return &delayReader{data: data, eofCh: ch}, ch
}

// TestDrainStdin_EOF confirms that DrainStdin returns not-truncated when EOF
// arrives within the first idle window (eofCh closed immediately after payload).
// Assertion: payload + immediate EOF → not truncated (EOF in window 1, no idle window fires).
func TestDrainStdin_EOF(t *testing.T) {
	payload := []byte(`{"tool_input":{"command":"echo ok"}}`)
	r, eofCh := newDelayReader(payload)
	close(eofCh) // EOF available immediately after first read

	got, truncated := DrainStdin(r, 50*time.Millisecond)
	if truncated {
		t.Fatal("expected not-truncated when EOF arrives quickly")
	}
	if string(got) != string(payload) {
		t.Fatalf("data mismatch: got %q want %q", got, payload)
	}
}

// TestDrainStdin_Truncated confirms that DrainStdin returns truncated=true when
// a second idle window fires with no bytes, even though EOF eventually arrives
// (simulating "(payload; sleep 6) | guard" — EOF at 3×window, deny at 2×window).
// Assertion: (payload; sleep 6) | guard → deny because window 2 expires empty
// before the late EOF arrives. elapsed bounds prove: (a) the guard waits for
// window 1 to collect data and starts window 2, and (b) it does NOT wait for EOF.
// Old rolling-timer code fires after only 1 window; this test would fail there
// because elapsed < 1.5W (no second window started).
func TestDrainStdin_Truncated(t *testing.T) {
	payload := []byte(`{"tool_input":{"command":"echo ok"}}`)
	// timedEOFReader: payload at t=0, EOF at 3W — well after the expected truncation at 2W.
	r := &timedEOFReader{
		data:     payload,
		eofDelay: testTimingWindow * 3, // EOF at 3W; guard should truncate at 2W
	}

	start := time.Now()
	got, truncated := DrainStdin(r, testTimingWindow)
	elapsed := time.Since(start)

	if !truncated {
		t.Fatal("expected truncated: EOF at 3W but window 2 should fire empty at 2W")
	}
	if string(got) != string(payload) {
		t.Fatalf("data before truncation mismatch: got %q want %q", got, payload)
	}
	// elapsed ∈ [1.5W, 2.75W]: ≥1.5W proves window 2 started; <2.75W proves we didn't wait for EOF.
	minE, maxE := testTimingWindow*3/2, testTimingWindow*11/4
	if elapsed < minE {
		t.Fatalf("elapsed %v < %v: fixed-window semantics require window 2 to fire (got data in window 1)", elapsed, minE)
	}
	if elapsed > maxE {
		t.Fatalf("elapsed %v > %v: guard waited past expected truncation point (EOF at 3W not yet due)", elapsed, maxE)
	}
}

// TestDrainStdin_IdleNoData confirms that an idle stdin (no bytes at all) is
// truncated within one idle window.
// Assertion: no bytes in window 1 → truncated after exactly 1 window. elapsed ∈ [W, 2W).
// This test does not bite the old rolling-timer code (both implementations truncate
// after 1 window when no bytes arrive); it guards against regressions where the
// implementation waits longer than one window or returns immediately.
func TestDrainStdin_IdleNoData(t *testing.T) {
	r, _ := newDelayReader([]byte{}) // no data, never closes
	start := time.Now()
	_, truncated := DrainStdin(r, testTimingWindow)
	elapsed := time.Since(start)
	if !truncated {
		t.Fatal("expected truncated on idle stdin (no bytes in window)")
	}
	if elapsed < testTimingWindow {
		t.Fatalf("elapsed %v < %v: must wait at least one window before declaring truncated", elapsed, testTimingWindow)
	}
	if elapsed > 2*testTimingWindow {
		t.Fatalf("elapsed %v > %v: should truncate after exactly 1 window for idle stdin", elapsed, 2*testTimingWindow)
	}
}

// TestDrainStdin_EOFInSecondWindow confirms that not-truncated is returned when
// EOF arrives during the second fixed window (the sleep-3 analog).
// Assertion: (payload; sleep 3) | guard → allow: payload fills window 1, EOF
// arrives at 1.5×window which is inside window 2 (before it expires).
func TestDrainStdin_EOFInSecondWindow(t *testing.T) {
	payload := []byte(`{"tool_input":{"command":"echo ok"}}`)
	r := &timedEOFReader{
		data:     payload,
		eofDelay: testTimingWindow * 3 / 2, // EOF arrives mid second window
	}

	got, truncated := DrainStdin(r, testTimingWindow)
	if truncated {
		t.Fatal("expected not-truncated when EOF arrives in second window (sleep-3 analog)")
	}
	if string(got) != string(payload) {
		t.Fatalf("data mismatch: got %q want %q", got, payload)
	}
}

// TestDrainStdin_ChunkedDataNotTruncated confirms that a stream delivering chunks
// every 0.5 windows across ≥5 windows is not truncated, and that all bytes are
// received in order.
// Assertion: accumulation across window boundaries: each window sees new data, so
// no idle window fires. This test does not bite the old rolling-timer code (0.5W
// gaps never idle a rolling timer either); it guards against boundary-drain bugs
// that could drop or reorder chunks arriving at window edges.
func TestDrainStdin_ChunkedDataNotTruncated(t *testing.T) {
	// 11 single-byte chunks "abcdefghijk" at 0.5W intervals → EOF at ≈5W.
	// Each window receives 2 chunks, so gotData is always true at window expiry.
	const expected = "abcdefghijk"
	chunks := make([][]byte, len(expected))
	for i, c := range expected {
		chunks[i] = []byte{byte(c)}
	}
	r := &chunkedReader{
		chunks: chunks,
		delay:  testTimingWindow / 2,
	}

	got, truncated := DrainStdin(r, testTimingWindow)
	if truncated {
		t.Fatal("expected not-truncated for chunked stream with no idle window")
	}
	if string(got) != expected {
		t.Fatalf("data mismatch: got %q want %q", got, expected)
	}
}

// TestExtractCommand_ToolInputCommand confirms priority 1 extraction.
// Assertion: tool_input.command is preferred over root-level command.
func TestExtractCommand_ToolInputCommand(t *testing.T) {
	input := `{"tool_input":{"command":"git push origin main"}}`
	cmd, err := ExtractCommand([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "git push origin main" {
		t.Fatalf("got %q want %q", cmd, "git push origin main")
	}
}

// TestExtractCommand_RootCommand confirms priority 2 extraction.
// Assertion: root "command" is used when tool_input.command is absent.
func TestExtractCommand_RootCommand(t *testing.T) {
	input := `{"command":"git push origin main"}`
	cmd, err := ExtractCommand([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "git push origin main" {
		t.Fatalf("got %q want %q", cmd, "git push origin main")
	}
}

// TestExtractCommand_ToolInfoCommandLine confirms priority 3 extraction.
// Assertion: tool_info.command_line is used when higher priorities absent.
func TestExtractCommand_ToolInfoCommandLine(t *testing.T) {
	input := `{"tool_info":{"command_line":"git push origin main"}}`
	cmd, err := ExtractCommand([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "git push origin main" {
		t.Fatalf("got %q want %q", cmd, "git push origin main")
	}
}

// TestExtractCommand_HookInputCommand confirms priority 4 extraction.
// Assertion: hook_input.command is used as last priority.
func TestExtractCommand_HookInputCommand(t *testing.T) {
	input := `{"hook_input":{"command":"git push origin main"}}`
	cmd, err := ExtractCommand([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "git push origin main" {
		t.Fatalf("got %q want %q", cmd, "git push origin main")
	}
}

// TestExtractCommand_CaseSensitive is gate (ii) from Wave 0 / ML-1A acceptance criteria.
// The struct-based decoder would case-fold "Command" onto "command" and return
// "echo ok" (allow). With map[string]json.RawMessage, "command" and "Command"
// are distinct keys; tool_input.command → "git push origin main" → deny.
// Assertion: {"tool_input":{"command":"git push origin main","Command":"echo ok"}} → deny payload.
func TestExtractCommand_CaseSensitive(t *testing.T) {
	input := `{"tool_input":{"command":"git push origin main","Command":"echo ok"}}`
	cmd, err := ExtractCommand([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "git push origin main" {
		t.Fatalf("case-sensitive check failed: got %q (should be 'git push origin main')", cmd)
	}
}

// TestExtractCommand_DuplicateKeyLastWins confirms ADR-2026-10-02 D2-ter.
// Assertion: {"command":"git push","command":"echo ok"} → "echo ok" (last wins).
func TestExtractCommand_DuplicateKeyLastWins(t *testing.T) {
	// encoding/json decodes duplicate keys with last-wins semantics for maps.
	input := `{"command":"git push origin main","command":"echo ok"}`
	cmd, err := ExtractCommand([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "echo ok" {
		t.Fatalf("duplicate key last-wins failed: got %q want %q", cmd, "echo ok")
	}
}

// TestExtractCommand_NUL confirms ADR-2026-10-02 D2-bis (fail-closed on NUL).
// Assertion: command containing \u0000 → ErrNULInCommand.
func TestExtractCommand_NUL(t *testing.T) {
	input := `{"command":"git push\u0000origin main"}`
	_, err := ExtractCommand([]byte(input))
	if err != ErrNULInCommand {
		t.Fatalf("expected ErrNULInCommand, got %v", err)
	}
}

// TestExtractCommand_Indecodeable confirms ADR-2026-10-02 D2 fail-closed.
// Assertion: command key present with non-string value → ErrIndecodeable.
func TestExtractCommand_Indecodeable(t *testing.T) {
	input := `{"command":12345}`
	_, err := ExtractCommand([]byte(input))
	if err == nil {
		t.Fatal("expected error for non-string command value, got nil")
	}
}

// TestExtractCommand_NullSkipped confirms null values are skipped (try next priority).
// Assertion: {"tool_input":{"command":null},"command":"echo ok"} → "echo ok".
func TestExtractCommand_NullSkipped(t *testing.T) {
	input := `{"tool_input":{"command":null},"command":"echo ok"}`
	cmd, err := ExtractCommand([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "echo ok" {
		t.Fatalf("null skip failed: got %q want %q", cmd, "echo ok")
	}
}

// TestExtractCommand_NoCommand confirms that missing command key returns empty string.
// Assertion: guard is a no-op when no known command key exists.
func TestExtractCommand_NoCommand(t *testing.T) {
	input := `{"some_other_field":"value"}`
	cmd, err := ExtractCommand([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != "" {
		t.Fatalf("expected empty command, got %q", cmd)
	}
}

// TestExtractCommand_NonJSON confirms that non-JSON input is returned as raw command.
// Assertion: bash `case *) CMD_RAW="$INPUT"` path.
func TestExtractCommand_NonJSON(t *testing.T) {
	input := "git push origin main"
	cmd, err := ExtractCommand([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != input {
		t.Fatalf("non-JSON path: got %q want %q", cmd, input)
	}
}

// TestExtractCommand_BOM confirms that a UTF-8 BOM is stripped before parsing.
// Assertion: PowerShell may emit BOM; guard must not fail on it.
func TestExtractCommand_BOM(t *testing.T) {
	bom := []byte{0xEF, 0xBB, 0xBF}
	input := append(bom, []byte(`{"command":"git push origin main"}`)...)
	cmd, err := ExtractCommand(input)
	if err != nil {
		t.Fatalf("BOM strip failed: %v", err)
	}
	if cmd != "git push origin main" {
		t.Fatalf("BOM strip: got %q want %q", cmd, "git push origin main")
	}
}

// TestExtractCommand_TrackFWGitCommandViaEnv is tested indirectly through
// RunGitBranch; ExtractCommand itself does not read env vars.

// --- quoteAwareSplit / stripHeredocBodies unit tests ---

// TestQuoteAwareSplit_BasicSemicolon confirms ';' splits segments outside quotes.
func TestQuoteAwareSplit_BasicSemicolon(t *testing.T) {
	parts := quoteAwareSplit("echo ok; git push")
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d: %v", len(parts), parts)
	}
	if strings.TrimSpace(parts[1]) != "git push" {
		t.Fatalf("unexpected second part: %q", parts[1])
	}
}

// TestQuoteAwareSplit_SemicolonInsideQuotes confirms ';' inside double quotes is not a split.
func TestQuoteAwareSplit_SemicolonInsideQuotes(t *testing.T) {
	parts := quoteAwareSplit(`git commit -m "msg; with semi"`)
	if len(parts) != 1 {
		t.Fatalf("expected 1 part (semicolon inside quotes), got %d: %v", len(parts), parts)
	}
}

// TestQuoteAwareSplit_AND splits on '&&' outside quotes.
func TestQuoteAwareSplit_AND(t *testing.T) {
	parts := quoteAwareSplit("echo ok && git push")
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts on &&, got %d", len(parts))
	}
}
