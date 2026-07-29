package fb

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// session.go stores the two cookies Tier 1 needs.
//
// Two, and not the twelve a browser holds: c_user names the account and xs is
// the session. Facebook accepts that pair on its own, so that is all fb asks
// for and all it keeps. It never asks for a password, never automates a login
// and never touches a checkpoint.
//
// The file is 0600 in the data directory. The cookie values go into a request
// header and nowhere else: not into the cache, not into the store, not into a
// sources list, and there is a test that greps every output format for them.

// Session is an imported browser session.
type Session struct {
	CUser      string    `json:"c_user"`
	XS         string    `json:"xs"`
	ImportedAt time.Time `json:"imported_at,omitzero"`
}

// Status is what `fb auth status` prints. It is a separate type because the
// obvious way to render a Session is to marshal it, and marshalling a Session
// prints the session cookie.
type Status struct {
	Account    string    `json:"account,omitempty"`
	ImportedAt time.Time `json:"imported_at,omitzero"`
	Path       string    `json:"path"`
	Present    bool      `json:"present"`
}

// Empty reports whether there is no usable session here.
func (s Session) Empty() bool { return s.CUser == "" || s.XS == "" }

// Cookie is the header value a request carries.
func (s Session) Cookie() string {
	if s.Empty() {
		return ""
	}
	return "c_user=" + s.CUser + "; xs=" + s.XS
}

// SessionPath is where the cookies live for a data directory.
func SessionPath(dataDir string) string {
	if dataDir == "" {
		dataDir = DefaultDataDir()
	}
	return filepath.Join(dataDir, "session.json")
}

// LoadSession reads the stored session. No file is not an error: running
// signed out is the normal case, and Tier 0 is most of what fb does.
func LoadSession(dataDir string) (Session, error) {
	b, err := os.ReadFile(SessionPath(dataDir))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Session{}, nil
		}
		return Session{}, err
	}
	var s Session
	if err := json.Unmarshal(b, &s); err != nil {
		return Session{}, usage("the session file at %s is not readable: %v", SessionPath(dataDir), err)
	}
	return s, nil
}

// SaveSession writes the session 0600.
func SaveSession(dataDir string, s Session) error {
	if s.Empty() {
		return usage("both cookies are needed: pass --c-user and --xs")
	}
	path := SessionPath(dataDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o600)
}

// ClearSession forgets the session. Forgetting one that is not there is not an
// error, because the state the caller asked for is the state they end up in.
func ClearSession(dataDir string) error {
	err := os.Remove(SessionPath(dataDir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// SessionStatus describes the stored session without printing it.
func SessionStatus(dataDir string) (Status, error) {
	st := Status{Path: SessionPath(dataDir)}
	s, err := LoadSession(dataDir)
	if err != nil {
		return st, err
	}
	if s.Empty() {
		return st, nil
	}
	st.Present, st.Account, st.ImportedAt = true, s.CUser, s.ImportedAt
	return st, nil
}
