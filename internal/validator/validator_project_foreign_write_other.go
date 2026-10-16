//go:build !windows

package validator

// validateProjectForeignWriteWindows is a no-op outside Windows.
// The project_foreign_write_windows rule exists only on Windows:
// on POSIX, mode bits (0o700/0o600) provide equivalent protection.
func validateProjectForeignWriteWindows() []string { return nil }

// checkProjectForeignWrite is the injectable inner function for tests.
// Outside Windows it always returns nil (the rule does not apply on POSIX).
func checkProjectForeignWrite(_ string) []string { return nil }
