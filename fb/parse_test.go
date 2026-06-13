package fb

import (
	"testing"
	"time"
)

func TestParseCount(t *testing.T) {
	cases := map[string]int64{
		"":            0,
		"0":           0,
		"42":          42,
		"1,234":       1234,
		"1.2K":        1200,
		"1.2k likes":  1200,
		"3.4M":        3400000,
		"2B":          2000000000,
		"12 comments": 12,
		"no digits":   0,
	}
	for in, want := range cases {
		if got := parseCount(in); got != want {
			t.Errorf("parseCount(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestParseTimeRelative(t *testing.T) {
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		in   string
		want time.Time
	}{
		{"just now", now},
		{"3h", now.Add(-3 * time.Hour)},
		{"2 days ago", now.AddDate(0, 0, -2)},
		{"yesterday at 4:00 PM", now.AddDate(0, 0, -1)},
		{"5m", now.Add(-5 * time.Minute)},
		{"1w", now.AddDate(0, 0, -7)},
	}
	for _, c := range cases {
		if got := parseTime(c.in, now); !got.Equal(c.want) {
			t.Errorf("parseTime(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseTimeEmpty(t *testing.T) {
	if !parseTime("", time.Now()).IsZero() {
		t.Error("empty string should parse to zero time")
	}
}

func TestCleanText(t *testing.T) {
	if got := cleanText("  hello   world \n"); got != "hello world" {
		t.Errorf("cleanText = %q", got)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello world", 5); got != "hello" && got != "he..." {
		t.Errorf("truncate did not shorten: %q", got)
	}
	if got := truncate("hi", 10); got != "hi" {
		t.Errorf("truncate over-shortened: %q", got)
	}
}
