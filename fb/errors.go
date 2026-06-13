package fb

import "fmt"

// Exit codes, mirrored in cmd/fb via CodeError.
const (
	ExitOK        = 0
	ExitGeneric   = 1
	ExitUsage     = 2
	ExitNotFound  = 3
	ExitLoginWall = 4
	ExitRateLimit = 5
	ExitNetwork   = 6
)

// CodeError carries an exit code alongside a message so main can map library
// failures to the documented exit-code table.
type CodeError struct {
	Code int
	Msg  string
	Err  error
}

func (e *CodeError) Error() string {
	if e.Err != nil {
		return e.Msg + ": " + e.Err.Error()
	}
	return e.Msg
}

func (e *CodeError) Unwrap() error { return e.Err }

func codeErr(code int, format string, args ...any) *CodeError {
	return &CodeError{Code: code, Msg: fmt.Sprintf(format, args...)}
}

// Sentinel-style constructors for the common cases.
var (
	errLoginWall = func() *CodeError {
		return codeErr(ExitLoginWall, "login wall: this content needs a session, pass --cookie or FACEBOOK_COOKIE")
	}
	errNotFound = func(what string) *CodeError {
		return codeErr(ExitNotFound, "not found: %s", what)
	}
)
