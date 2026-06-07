package cli

// ExitError is an error that maps to an explicit exit code.
type ExitError struct {
	code int
	msg  string
}

func (e *ExitError) Error() string { return e.msg }
func (e *ExitError) Code() int     { return e.code }
