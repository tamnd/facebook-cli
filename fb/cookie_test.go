package fb

import "testing"

func TestParseCookieFileRawHeader(t *testing.T) {
	got := parseCookieFile("c_user=123; xs=abc; datr=zzz")
	if got != "c_user=123; xs=abc; datr=zzz" {
		t.Errorf("raw header = %q", got)
	}
}

func TestParseCookieFileHeaderPrefix(t *testing.T) {
	got := parseCookieFile("Cookie: c_user=123; xs=abc")
	if got != "c_user=123; xs=abc" {
		t.Errorf("prefixed header = %q", got)
	}
}

func TestParseCookieFileJSON(t *testing.T) {
	in := `[{"name":"c_user","value":"123","domain":".facebook.com"},
	        {"name":"xs","value":"abc","domain":".facebook.com"},
	        {"name":"other","value":"x","domain":".example.com"}]`
	got := parseCookieFile(in)
	if got != "c_user=123; xs=abc" {
		t.Errorf("json cookies = %q", got)
	}
}

func TestParseCookieFileNetscape(t *testing.T) {
	in := "# Netscape HTTP Cookie File\n" +
		".facebook.com\tTRUE\t/\tTRUE\t0\tc_user\t123\n" +
		".facebook.com\tTRUE\t/\tTRUE\t0\txs\tabc\n"
	got := parseCookieFile(in)
	if got != "c_user=123; xs=abc" {
		t.Errorf("netscape cookies = %q", got)
	}
}

func TestCookieUser(t *testing.T) {
	if got := cookieUser("datr=zzz; c_user=987654321; xs=abc"); got != "987654321" {
		t.Errorf("cookieUser = %q", got)
	}
	if got := cookieUser("datr=zzz"); got != "" {
		t.Errorf("cookieUser without c_user = %q", got)
	}
}
