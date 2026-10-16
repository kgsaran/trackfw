//go:build windows

package fsperm

// White-box tests for fsperm that require access to unexported symbols.
// Run only on Windows CI (windows-full-suites).

import (
	"errors"
	"testing"
)

// TestCreateRestrictedFile_DetectorError_FailsClosed asserts that when the
// aclSupportDetector returns an error, CreateRestrictedFile returns an error and
// does NOT fall back to os.CreateTemp (R-2 / ADR D4: fail-closed, not fail-open).
//
// Reconciliation: this test affirms "CreateRestrictedFile is fail-closed when the
// ACL-support detector itself fails — no unprotected file is produced". It would
// fail without the fix because the original code used `err != nil || !aclOK` in a
// single branch that fell through to os.CreateTemp for BOTH error and no-ACL cases,
// making a detector error indistinguishable from a volume-without-ACL soft-fail.
func TestCreateRestrictedFile_DetectorError_FailsClosed(t *testing.T) {
	// Save and restore the real detector so other tests are not affected.
	orig := aclSupportDetector
	defer func() { aclSupportDetector = orig }()

	injected := errors.New("injected ACL detector failure")
	aclSupportDetector = func(string) (bool, error) {
		return false, injected
	}

	dir := t.TempDir()
	f, err := CreateRestrictedFile(dir, ".test-failclosed-*")
	if err == nil {
		if f != nil {
			f.Close()
		}
		t.Fatal("CreateRestrictedFile should return an error when the ACL detector fails, but returned nil error")
	}
	if f != nil {
		f.Close()
		t.Error("CreateRestrictedFile returned a non-nil *os.File alongside an error — must be nil on failure")
	}
	if !errors.Is(err, injected) {
		t.Errorf("expected error chain to contain injected error; got: %v", err)
	}
	t.Logf("PROBE w2b fail-closed: detector error correctly propagated: %v", err)
}
