package commands

// ExitCodeError signals to Execute() that the command already wrote its
// output document and only the exit code is needed. Execute() exits with
// Code without printing anything to stderr. Code must not be 0.
//
// Restrictions:
//   - Do NOT combine with errors.Join: errors.Join(real, *ExitCodeError) is
//     recognised by Execute() as a silent exit and real is swallowed.
//   - Do NOT wrap with %w: fmt.Errorf("ctx: %w", e) is not recognised by
//     Execute() (direct assertion, not errors.As) and will be printed as a
//     normal error message instead of silencing output.
//   - Only use after the result document has been successfully written to
//     stdout. If the write fails, return a plain fmt.Errorf instead.
type ExitCodeError struct {
	Code int
	Msg  string // human message, returned by Error()
}

func (e *ExitCodeError) Error() string { return e.Msg }

// newExitCodeError is the only constructor for ExitCodeError.
// It panics when code == 0 to prevent silent success on unintended zero codes.
func newExitCodeError(code int, msg string) *ExitCodeError {
	if code == 0 {
		panic("ExitCodeError: code must not be 0")
	}
	return &ExitCodeError{Code: code, Msg: msg}
}
