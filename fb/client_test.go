package fb

import "testing"

func TestIsErrorShell(t *testing.T) {
	yes := []string{
		`<html><head><title>Error Facebook</title></head><body>Sorry, something went wrong.</body></html>`,
		`<html><head><title>Lỗi</title></head><body>Trình duyệt này không hỗ trợ Facebook</body></html>`,
		`<title>Erreur</title>`,
		`<title>错误</title>`,
		`<html>The link you followed may be broken</html>`,
	}
	for _, h := range yes {
		if !isErrorShell([]byte(h)) {
			t.Errorf("isErrorShell should be true for %.40q", h)
		}
	}
	no := []string{
		`<html><head><title>NASA - Home</title></head><body>posts...</body></html>`,
		`<title>Mark Zuckerberg</title>`,
		``,
	}
	for _, h := range no {
		if isErrorShell([]byte(h)) {
			t.Errorf("isErrorShell should be false for %.40q", h)
		}
	}
}

func TestIsLoginWall(t *testing.T) {
	if !isLoginWall([]byte(`<p>You must log in to continue.</p>`)) {
		t.Error("login wall not detected")
	}
	if isLoginWall([]byte(`<title>NASA</title>`)) {
		t.Error("false positive login wall")
	}
}

func TestExtractTitle(t *testing.T) {
	if got := extractTitle(`<html><head><title>hello</title></head>`); got != "hello" {
		t.Errorf("extractTitle = %q", got)
	}
	if got := extractTitle(`<html>no title here</html>`); got != "" {
		t.Errorf("extractTitle = %q, want empty", got)
	}
}
