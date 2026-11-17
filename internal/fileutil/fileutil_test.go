package fileutil_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kgsaran/trackfw/internal/fileutil"
)

// readWithDeadline runs ReadRegularFile in a goroutine and reports whether it
// hung past the deadline. If it hangs, the goroutine leaks for the remainder of
// the test process — acceptable because os.Exit tears down OS threads regardless
// of the syscall they are blocked in; the point is to turn a hang into a fast,
// deterministic failure rather than a go test timeout.
func readWithDeadline(t *testing.T, d time.Duration, path string) (content []byte, err error, hung bool) {
	t.Helper()
	type result struct {
		content []byte
		err     error
	}
	ch := make(chan result, 1)
	go func() {
		c, e := fileutil.ReadRegularFile(path)
		ch <- result{c, e}
	}()
	select {
	case r := <-ch:
		return r.content, r.err, false
	case <-time.After(d):
		return nil, nil, true
	}
}

// TestReadRegularFile_RegularFile_ReturnsContent asserts the base case:
// ReadRegularFile on a regular file returns its content without error.
//
// Conclusion (ML-FA2): ReadRegularFile does not break the normal read path;
// this test FAILS if the function incorrectly rejects regular files.
func TestReadRegularFile_RegularFile_ReturnsContent(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "test.json")
	want := []byte(`{"hooks":{}}`)
	if err := os.WriteFile(p, want, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err, hung := readWithDeadline(t, 3*time.Second, p)
	if hung {
		t.Fatal("ReadRegularFile blocked on a regular file")
	}
	if err != nil {
		t.Fatalf("unexpected error reading regular file: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("content mismatch: got %q, want %q", got, want)
	}
}

// TestReadRegularFile_Absent_ReturnsNotExist asserts that an absent path returns
// an error satisfying os.IsNotExist.
//
// Conclusion (ML-FA2): absence remains distinguishable from other errors; this
// test FAILS if ErrNotRegularFile is returned for absent paths.
func TestReadRegularFile_Absent_ReturnsNotExist(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "does-not-exist.json")

	_, err, hung := readWithDeadline(t, 3*time.Second, p)
	if hung {
		t.Fatal("ReadRegularFile blocked on an absent path")
	}
	if !os.IsNotExist(err) {
		t.Errorf("expected os.IsNotExist(err) for absent path, got: %v", err)
	}
}

// TestReadRegularFile_Directory_ReturnsErrNotRegularFile asserts that a directory
// path returns ErrNotRegularFile (not a "not exist" error, not nil).
//
// Conclusion (ML-FA2): directories are rejected by the allowlist check; this test
// FAILS if ReadRegularFile reads from a directory or misclassifies the error.
func TestReadRegularFile_Directory_ReturnsErrNotRegularFile(t *testing.T) {
	dir := t.TempDir()

	_, err, hung := readWithDeadline(t, 3*time.Second, dir)
	if hung {
		t.Fatal("ReadRegularFile blocked on a directory")
	}
	if err == nil {
		t.Fatal("expected error for directory, got nil")
	}
	if os.IsNotExist(err) {
		t.Errorf("directory should not be reported as absent: %v", err)
	}
	if !errors.Is(err, fileutil.ErrNotRegularFile) {
		t.Errorf("expected ErrNotRegularFile for directory, got: %v", err)
	}
}
