package fb

import "testing"

// TestLoginBounceIsSpottedByWhereItLanded pins the one signal there is.
//
// The bounce is a 302 to a 200, so every other check in the client says the
// request succeeded. Getting this predicate wrong in either direction is
// expensive: too loose and every read costs three requests, too tight and a
// public group reads as walled about two thirds of the time.
func TestLoginBounceIsSpottedByWhereItLanded(t *testing.T) {
	cases := []struct {
		name       string
		raw, final string
		want       bool
	}{
		{
			"the group page bounced",
			"https://www.facebook.com/groups/1443890352589739",
			"https://web.facebook.com/login/?next=https%3A%2F%2Fwww.facebook.com%2Fgroups%2F1443890352589739&_rdc=1&_rdr",
			true,
		},
		{
			"the mirror served the group",
			"https://www.facebook.com/groups/1443890352589739",
			"https://web.facebook.com/groups/1443890352589739?_rdc=1&_rdr",
			false,
		},
		{
			"nothing redirected",
			"https://www.facebook.com/zuck",
			"https://www.facebook.com/zuck",
			false,
		},
		{
			"no final URL to judge",
			"https://www.facebook.com/zuck",
			"",
			false,
		},
		{
			// Asking for the log-in page and getting it is the answer, not a
			// bounce, and retrying it would be three requests for nothing.
			"the log-in page was what was asked for",
			"https://www.facebook.com/login/",
			"https://web.facebook.com/login/?_rdc=1&_rdr",
			false,
		},
		{
			// A profile whose handle happens to start with the word is not a
			// redirect to /login, and the substring match has to know that.
			"a handle that reads like the wall",
			"https://www.facebook.com/loginradius",
			"https://web.facebook.com/loginradius?_rdc=1&_rdr",
			false,
		},
	}
	for _, c := range cases {
		if got := loginBounce(c.raw, c.final); got != c.want {
			t.Errorf("%s: loginBounce(%q, %q) = %v, want %v", c.name, c.raw, c.final, got, c.want)
		}
	}
}
