package fb

import "testing"

func TestCanonURLKeepsWhatIdentifies(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://web.facebook.com/zuck?_rdc=1&_rdr", "https://www.facebook.com/zuck"},
		{"https://m.facebook.com/zuck/posts/123?__tn__=-R", "https://www.facebook.com/zuck/posts/123"},
		{"https://www.facebook.com/photo.php?fbid=1&set=pb.4&type=3", "https://www.facebook.com/photo.php?fbid=1&set=pb.4&type=3"},
		{"https://www.facebook.com/zuck#footer", "https://www.facebook.com/zuck"},
		{"https://scontent.fhan17-1.fna.fbcdn.net/v/t39.jpg?oh=1", "https://scontent.fhan17-1.fna.fbcdn.net/v/t39.jpg?oh=1"},
		{"", ""},
	}
	for _, c := range cases {
		if got := canonURL(c.in); got != c.want {
			t.Errorf("canonURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMirrorOfEndsTheBounce(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://www.facebook.com/zuck", "https://web.facebook.com/zuck?_rdc=1&_rdr"},
		{"https://www.facebook.com/profile.php?id=4", "https://web.facebook.com/profile.php?_rdc=1&id=4&_rdr"},
		// Already on the mirror, so there is nowhere left to fall back to.
		{"https://web.facebook.com/zuck", ""},
		// Not facebook, so not ours to rewrite.
		{"https://scontent.fhan17-1.fna.fbcdn.net/v/t39.jpg", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := mirrorOf(c.in); got != c.want {
			t.Errorf("mirrorOf(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
