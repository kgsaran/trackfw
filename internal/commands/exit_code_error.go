package commands

// ExitCodeError signals to Execute() that the command already wrote its
// output document and only the exit code is needed. Execute() (via decideExit)
// exits with Code without printing anything to stderr. Code must be in [1,255].
//
// Restrictions:
//   - Do NOT wrap with %w: fmt.Errorf("ctx: %w", e) is NOT recognised by
//     Execute() (direct assertion, not errors.As) and will be printed as a
//     normal error message instead of silencing output.
//   - Do NOT combine with errors.Join: errors.Join(real, *ExitCodeError) is
//     likewise NOT recognised — if Execute() were ever changed to errors.As,
//     errors.Join(real, *ExitCodeError) would be silenced and real swallowed.
//   - Only use after the result document has been successfully written to
//     stdout. If the write fails, return a plain fmt.Errorf instead.
//   - Code must be in [1,255]. Use newExitCodeError to enforce this at
//     construction time; struct literals with Code outside [1,255] are treated
//     as regular errors by decideExit (noisy, rc=1).
type ExitCodeError struct {
	Code int
	Msg  string // human message, returned by Error()
}

func (e *ExitCodeError) Error() string { return e.Msg }

// newExitCodeError is the only constructor for ExitCodeError.
// It panics when code is outside [1,255] to prevent silent success on
// unintended zero codes and to prevent os.Exit(256) wrapping to rc=0 on Unix.
func newExitCodeError(code int, msg string) *ExitCodeError {
	if code < 1 || code > 255 {
		panic("ExitCodeError: code must be in [1,255]")
	}
	return &ExitCodeError{Code: code, Msg: msg}
}
