package netdeadline

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

// TestParse_Valid asserts that a well-formed duration string is parsed and returned unchanged.
// Conclusion: a valid duration value is accepted and propagated to the caller.
func TestParse_Valid(t *testing.T) {
	got, warn := Parse("90s", DefaultNetworkTimeout, NetworkTimeoutEnv)
	if got != 90*time.Second {
		t.Fatalf("want 90s, got %s", got)
	}
	if warn != "" {
		t.Fatalf("unexpected warning: %q", warn)
	}
}

// TestParse_Empty asserts that an empty string returns the default with no warning.
// Conclusion: absent configuration silently falls back to the default.
func TestParse_Empty(t *testing.T) {
	got, warn := Parse("", DefaultNetworkTimeout, NetworkTimeoutEnv)
	if got != DefaultNetworkTimeout {
		t.Fatalf("want default %s, got %s", DefaultNetworkTimeout, got)
	}
	if warn != "" {
		t.Fatalf("expected no warning for empty, got: %q", warn)
	}
}

// TestParse_Zero asserts that "0" is rejected, the default is used, and a warning is produced.
// Conclusion: zero is invalid; the deadline is never disabled (ADR A3).
func TestParse_Zero(t *testing.T) {
	got, warn := Parse("0", DefaultNetworkTimeout, NetworkTimeoutEnv)
	if got != DefaultNetworkTimeout {
		t.Fatalf("want default %s, got %s", DefaultNetworkTimeout, got)
	}
	if warn == "" {
		t.Fatal("expected warning for zero, got none")
	}
	if !strings.Contains(warn, "must be positive") {
		t.Fatalf("warning should mention 'must be positive', got: %q", warn)
	}
}

// TestParse_Negative asserts that a negative duration is rejected and the default is used.
// Conclusion: negative values are invalid; a timeout context would expire immediately (ADR A3).
func TestParse_Negative(t *testing.T) {
	got, warn := Parse("-1s", DefaultNetworkTimeout, NetworkTimeoutEnv)
	if got != DefaultNetworkTimeout {
		t.Fatalf("want default %s, got %s", DefaultNetworkTimeout, got)
	}
	if warn == "" {
		t.Fatal("expected warning for negative, got none")
	}
	if !strings.Contains(warn, "must be positive") {
		t.Fatalf("warning should mention 'must be positive', got: %q", warn)
	}
}

// TestParse_AboveMaximum asserts that a duration above 24 h is rejected.
// Conclusion: absurd values are caught; the deadline is not nominally set but effectively disabled (ADR A3).
func TestParse_AboveMaximum(t *testing.T) {
	got, warn := Parse("25h", DefaultNetworkTimeout, NetworkTimeoutEnv)
	if got != DefaultNetworkTimeout {
		t.Fatalf("want default %s, got %s", DefaultNetworkTimeout, got)
	}
	if warn == "" {
		t.Fatal("expected warning for 25h, got none")
	}
	if !strings.Contains(warn, "exceeds maximum") {
		t.Fatalf("warning should mention 'exceeds maximum', got: %q", warn)
	}
}

// TestParse_InvalidText asserts that a non-parseable string is rejected.
// Conclusion: invalid text produces a warning and falls back to the default.
func TestParse_InvalidText(t *testing.T) {
	got, warn := Parse("banana", DefaultNetworkTimeout, NetworkTimeoutEnv)
	if got != DefaultNetworkTimeout {
		t.Fatalf("want default %s, got %s", DefaultNetworkTimeout, got)
	}
	if warn == "" {
		t.Fatal("expected warning for 'banana', got none")
	}
	if !strings.Contains(warn, "is invalid") {
		t.Fatalf("warning should mention 'is invalid', got: %q", warn)
	}
}

// TestParse_ExactBoundary_24h asserts that exactly 24 h is valid (boundary case).
// Conclusion: the maximum boundary is inclusive.
func TestParse_ExactBoundary_24h(t *testing.T) {
	got, warn := Parse("24h", DefaultNetworkTimeout, NetworkTimeoutEnv)
	if got != 24*time.Hour {
		t.Fatalf("want 24h, got %s", got)
	}
	if warn != "" {
		t.Fatalf("unexpected warning at boundary: %q", warn)
	}
}

// TestNetworkTimeout_ReadEnv asserts that NetworkTimeout() reads TRACKFW_NETWORK_TIMEOUT
// and emits a warning to warnWriter on invalid values.
// Conclusion: the env-reading wrapper honours the TRACKFW_NETWORK_TIMEOUT variable.
func TestNetworkTimeout_ReadEnv(t *testing.T) {
	var buf bytes.Buffer
	old := warnWriter
	warnWriter = &buf
	t.Cleanup(func() { warnWriter = old })

	t.Setenv(NetworkTimeoutEnv, "invalid")
	got := NetworkTimeout()
	if got != DefaultNetworkTimeout {
		t.Fatalf("want default, got %s", got)
	}
	if buf.Len() == 0 {
		t.Fatal("expected warning written to warnWriter, got none")
	}
}

// TestPushTimeout_Default asserts that PushTimeout() returns the default when the env is unset.
// Conclusion: TRACKFW_PUSH_TIMEOUT absent uses the 30-minute default.
func TestPushTimeout_Default(t *testing.T) {
	t.Setenv(PushTimeoutEnv, "") // unset
	got := PushTimeout()
	if got != DefaultPushTimeout {
		t.Fatalf("want %s, got %s", DefaultPushTimeout, got)
	}
}

// TestTimeoutError_Error asserts the message format matches ADR D5.
// Format: "<op> timed out after <duration> (<var>)".
// Note: time.Duration.String() formats 60s as "1m0s", not "60s".
// Conclusion: the error message correctly identifies the operation, limit and variable.
func TestTimeoutError_Error(t *testing.T) {
	e := &TimeoutError{Op: "git fetch origin", Limit: 60 * time.Second, Var: NetworkTimeoutEnv}
	want := "git fetch origin timed out after 1m0s (TRACKFW_NETWORK_TIMEOUT)"
	if e.Error() != want {
		t.Fatalf("want %q, got %q", want, e.Error())
	}
}

// TestTimeoutError_Is asserts that errors.Is(err, ErrTimeout) returns true for *TimeoutError.
// Conclusion: callers can detect timeout without type-asserting the concrete type.
func TestTimeoutError_Is(t *testing.T) {
	e := &TimeoutError{Op: "git push", Limit: 30 * time.Minute, Var: PushTimeoutEnv}
	if !errors.Is(e, ErrTimeout) {
		t.Fatal("errors.Is(TimeoutError, ErrTimeout) should return true")
	}
	if errors.Is(e, errors.New("other")) {
		t.Fatal("errors.Is should not match unrelated errors")
	}
}
