package fb

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/tamnd/any-cli/kit/errs"
)

// errors.go is the typed error taxonomy the CLI turns into stable exit codes
// (spec 3004 doc 05 section 7).
//
//	2 usage        a bad flag, a bad argument shape
//	3 no results   the read worked and there was nothing there
//	4 needs auth   Facebook refused this signed out, and the message names the fix
//	5 rate limited a real 429
//	6 not found    the id does not name anything, or it is private
//	7 unsupported  nothing serves this question at any tier
//	8 network      DNS, TLS, timeout, or a body that is neither HTML nor JSON
//
// 4 against 7 is the distinction that makes this worth having. 4 means a
// credential would fix it. 7 means none would, so nobody goes looking.

// NeedAuthError marks something Facebook will not serve to a signed-out caller
// (exit code 4).
type NeedAuthError struct{ Msg string }

func (e *NeedAuthError) Error() string { return e.Msg }

// needAuth builds the exit-4 error. Every message here has to end with the
// thing that would fix it, because "needs auth" without the fix is just a
// closed door.
func needAuth(format string, args ...any) error {
	return &NeedAuthError{Msg: fmt.Sprintf(format, args...)}
}

// NotFoundError marks an id that names nothing, or names something private
// (exit code 6). Facebook does not distinguish the two to a signed-out reader
// and neither does this, because guessing which would be a guess.
type NotFoundError struct {
	What   string
	Reason string
}

func (e *NotFoundError) Error() string {
	if e.Reason == "" {
		return e.What + " was not found"
	}
	return e.What + " was not found: " + e.Reason
}

func notFound(what, reason string) error {
	return &NotFoundError{What: what, Reason: reason}
}

// NoResultsError marks a read that worked and found nothing (exit code 3),
// which is not the same as a read that could not happen.
type NoResultsError struct{ Msg string }

func (e *NoResultsError) Error() string { return e.Msg }

func noResults(format string, args ...any) error {
	return &NoResultsError{Msg: fmt.Sprintf(format, args...)}
}

// NoResults is the exit-3 error for a caller outside this package. A command
// that read something and found the list empty raises it, so the empty list and
// the failed read exit differently.
func NoResults(format string, args ...any) error { return noResults(format, args...) }

// UsageError marks a bad argument (exit code 2).
type UsageError struct{ Msg string }

func (e *UsageError) Error() string { return e.Msg }

func usage(format string, args ...any) error {
	return &UsageError{Msg: fmt.Sprintf(format, args...)}
}

// RateLimitedError marks a real 429 (exit code 5).
//
// It is deliberately hard to reach. Facebook's own refusals arrive at HTTP 200
// with the message "Rate limit exceeded" and are not rate limits at all, so
// they map to NeedAuthError instead. See graphqlRefusal.
type RateLimitedError struct {
	Msg      string
	Endpoint string
}

func (e *RateLimitedError) Error() string { return e.Msg }

// UnsupportedError marks something no surface serves at any tier (exit code 7).
type UnsupportedError struct{ Msg string }

func (e *UnsupportedError) Error() string { return e.Msg }

// unsupported builds the exit-7 error. why should say what Facebook does, not
// what fb does not.
func unsupported(what, why string) error {
	msg := "no surface serves " + what + " at any tier"
	if why != "" {
		msg += ": " + why
	}
	return &UnsupportedError{Msg: msg}
}

// NetworkError marks a transport failure (exit code 8). Nothing about the
// request was wrong, so trying again is reasonable, and that is why it is not
// lumped in with the rest.
type NetworkError struct {
	Msg string
	Err error
}

func (e *NetworkError) Error() string { return e.Msg }
func (e *NetworkError) Unwrap() error { return e.Err }

// HTTPError is a response that came back with a status fb did not want. The
// server answered, so it is not a network failure.
type HTTPError struct {
	Status int
	URL    string
	Body   string
}

func (e *HTTPError) Error() string {
	msg := fmt.Sprintf("%s answered HTTP %d", e.URL, e.Status)
	if e.Body != "" {
		body := strings.TrimSpace(e.Body)
		if len(body) > 200 {
			body = body[:200] + "..."
		}
		msg += ": " + body
	}
	return msg
}

// AsNetwork returns a *NetworkError when err is a transport failure, and nil
// when it is anything else. A cancelled context is the caller giving up rather
// than the network failing, so it is left alone.
func AsNetwork(err error) error {
	if err == nil || errors.Is(err, context.Canceled) {
		return nil
	}
	var he *HTTPError
	if errors.As(err, &he) {
		return nil
	}
	var de *net.DNSError
	var ne net.Error
	var ue *url.Error
	switch {
	case errors.As(err, &de):
		return &NetworkError{Msg: "cannot resolve " + de.Name + ": " + de.Err, Err: err}
	case errors.Is(err, context.DeadlineExceeded):
		return &NetworkError{Msg: "the request timed out: raise --timeout or try again", Err: err}
	case errors.As(err, &ne) && ne.Timeout():
		return &NetworkError{Msg: "the request timed out: raise --timeout or try again", Err: err}
	case errors.As(err, &ue):
		return &NetworkError{Msg: "cannot reach " + hostOf(ue.URL) + ": " + ue.Err.Error(), Err: err}
	}
	return nil
}

func hostOf(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Host
	}
	return raw
}

// asHTTPStatus classifies a status code fb cannot use.
func asHTTPStatus(status int, what, body string) error {
	switch {
	case status == http.StatusTooManyRequests:
		return &RateLimitedError{Msg: "facebook answered 429 for " + what + ": slow down or try again later"}
	case status == http.StatusNotFound:
		return notFound(what, "HTTP 404")
	case status >= 500:
		return &HTTPError{Status: status, URL: what, Body: body}
	}
	return nil
}

// ExitCode maps an error to the code the process exits with.
type Failure struct {
	Kind    string `json:"kind"` // always "error"
	Target  string `json:"target"`
	Code    int    `json:"code"`
	Reason  string `json:"reason"`
	Tier    int    `json:"tier"`
	Surface string `json:"surface,omitempty"`
}

// ExitCode is the code for an error, and 0 for nil.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var (
		ue *UsageError
		nr *NoResultsError
		na *NeedAuthError
		rl *RateLimitedError
		nf *NotFoundError
		un *UnsupportedError
		ne *NetworkError
	)
	switch {
	case errors.As(err, &ue):
		return 2
	case errors.As(err, &nr):
		return 3
	case errors.As(err, &na):
		return 4
	case errors.As(err, &rl):
		return 5
	case errors.As(err, &nf):
		return 6
	case errors.As(err, &un):
		return 7
	case errors.As(err, &ne):
		return 8
	}
	return 1
}

// FailureOf renders an error as a record, for the callers streaming JSONL who
// need to see what did not work without parsing stderr.
func FailureOf(err error, target string, tier int) Failure {
	f := Failure{Kind: "error", Target: target, Code: ExitCode(err), Reason: err.Error(), Tier: tier}
	var rl *RateLimitedError
	if errors.As(err, &rl) {
		f.Surface = rl.Endpoint
	}
	return f
}

// NeedAuth is the exit-4 error for a caller outside this package. The message
// has to end with the thing that would fix it, same as the internal one.
func NeedAuth(format string, args ...any) error { return needAuth(format, args...) }

// MapErr turns one of these errors into the kit error that carries the matching
// exit code. Both callers use it: the hand-written commands in cli, and the
// operations in ops.go that fill the serve and MCP surfaces.
//
// 4 and 7 are the pair that matter. 4 means a session would fix it and the
// message says so. 7 means nothing serves this at any tier, so nobody should go
// looking for a credential that would not help.
func MapErr(err error) error {
	if err == nil {
		return nil
	}
	var (
		ue *UsageError
		nr *NoResultsError
		na *NeedAuthError
		rl *RateLimitedError
		nf *NotFoundError
		un *UnsupportedError
		ne *NetworkError
	)
	switch {
	case errors.As(err, &ue):
		return errs.Usage("%s", ue.Error())
	case errors.As(err, &nr):
		return errs.NoResults("%s", nr.Error())
	case errors.As(err, &na):
		return errs.NeedAuth("%s", na.Error())
	case errors.As(err, &rl):
		return errs.RateLimited("%s", rl.Error())
	case errors.As(err, &nf):
		return errs.NotFound("%s", nf.Error())
	case errors.As(err, &un):
		return errs.Unsupported("%s", un.Error())
	case errors.As(err, &ne):
		return errs.Network("%s", ne.Error())
	}
	// A transport failure that never went through the client still gets 8.
	if n := AsNetwork(err); n != nil {
		return errs.Network("%s", n.Error())
	}
	return err
}
