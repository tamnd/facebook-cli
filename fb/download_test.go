package fb

import (
	"testing"
	"time"
)

// TestExtOfBelievesStpOverThePath is the bug this file exists for. Every URL
// here is real, taken off a photo record on 2026-07-29, and the first one is the
// one that caught it: the path says .png, the CDN answers Content-Type
// image/jpeg, and the only thing in the URL that tells the truth is stp.
func TestExtOfBelievesStpOverThePath(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want string
	}{
		{
			"a png path that stp says is a jpeg",
			"https://scontent.fhan17-1.fna.fbcdn.net/v/t39.99422-6/759547557_n.png?stp=dst-jpg_tt6&cstp=mx1638x2048&oe=6A6F87BE",
			".jpg",
		},
		{
			"a jpg path and stp agreeing",
			"https://scontent.fhan17-1.fna.fbcdn.net/v/t39.30808-6/756229023_n.jpg?stp=dst-jpg_tt6&oe=6A6F896A",
			".jpg",
		},
		{
			"stp naming webp, sizing after the underscore",
			"https://scontent.fhan17-1.fna.fbcdn.net/v/t1.6435-9/123_n.jpg?stp=dst-webp_s600x600",
			".webp",
		},
		{
			"no stp, so the path is all there is",
			"https://scontent.fhan17-1.fna.fbcdn.net/v/t1.6435-9/123_n.png?_nc_cat=1",
			".png",
		},
		{
			"an stp that names no destination format",
			"https://scontent.fhan17-1.fna.fbcdn.net/v/t1.6435-9/123_n.gif?stp=c0.5000x0.5000",
			".gif",
		},
		{
			"a video URL, where the query is long and the path is plain",
			"https://video.fhan17-1.fna.fbcdn.net/v/t42.1790-2/123_n.mp4?efg=eyJ2ZW5jb2RlIjoiaHZjMSJ9&_nc_ht=video.fhan17-1.fna",
			".mp4",
		},
		{
			"nothing to read either way",
			"https://scontent.fhan17-1.fna.fbcdn.net/safe_image.php?d=AQ&url=x",
			".jpg",
		},
		{
			"an extension nobody asked for stays off the filename",
			"https://scontent.fhan17-1.fna.fbcdn.net/v/t1.6435-9/123_n.php?stp=dst-exe",
			".jpg",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := extOf(c.url, ".jpg"); got != c.want {
				t.Errorf("extOf(%s) = %s, want %s", c.url, got, c.want)
			}
		})
	}
}

// TestFreshRefusesARecordWhoseSignatureHasHadTimeToExpire checks the guard that
// turns an unexplained 403 into a sentence saying what to do about it.
func TestFreshRefusesARecordWhoseSignatureHasHadTimeToExpire(t *testing.T) {
	if err := fresh(Envelope{}, "photo"); err != nil {
		t.Errorf("a record with no fetch time is not something we can call stale: %v", err)
	}
	if err := fresh(Envelope{FetchedAt: time.Now()}, "photo"); err != nil {
		t.Errorf("a record read just now is fresh: %v", err)
	}
	err := fresh(Envelope{FetchedAt: time.Now().Add(-2 * time.Hour)}, "photo")
	if err == nil {
		t.Fatal("a record read two hours ago has signed URLs that have expired, and fresh let it through")
	}
	if ExitCode(err) != 2 {
		t.Errorf("stale record exited %d, want 2: it is the caller asking for the wrong thing", ExitCode(err))
	}
}
