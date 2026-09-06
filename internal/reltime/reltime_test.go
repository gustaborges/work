package reltime

import (
	"testing"
	"time"
)

func TestFormatBuckets(t *testing.T) {
	cases := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"negative skew", -3 * time.Hour, "just now"},
		{"zero", 0, "just now"},
		{"just under 45s", 44 * time.Second, "just now"},
		{"exactly 45s", 45 * time.Second, "a minute ago"},
		{"just under 90s", 89 * time.Second, "a minute ago"},
		{"exactly 90s", 90 * time.Second, "2 minutes ago"},
		{"5 minutes", 5 * time.Minute, "5 minutes ago"},
		{"44 minutes", 44 * time.Minute, "44 minutes ago"},
		{"just under 45m rounds up", 45*time.Minute - time.Second, "45 minutes ago"},
		{"exactly 45m", 45 * time.Minute, "an hour ago"},
		{"just under 90m", 90*time.Minute - time.Second, "an hour ago"},
		{"exactly 90m", 90 * time.Minute, "2 hours ago"},
		{"5 hours", 5 * time.Hour, "5 hours ago"},
		{"just under 22h rounds up", 22*time.Hour - time.Second, "22 hours ago"},
		{"exactly 22h", 22 * time.Hour, "a day ago"},
		{"just under 36h", 36*time.Hour - time.Second, "a day ago"},
		{"exactly 36h", 36 * time.Hour, "2 days ago"},
		{"3 days", 72 * time.Hour, "3 days ago"},
		{"10 days", 240 * time.Hour, "10 days ago"},
	}
	for _, c := range cases {
		if got := Format(c.d); got != c.want {
			t.Errorf("%s: Format(%v) = %q, want %q", c.name, c.d, got, c.want)
		}
	}
}
