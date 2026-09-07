package reltime

import (
	"fmt"
	"math"
	"time"
)

// Bucket thresholds (research R14). Each names the exclusive upper bound of the
// range that renders with the phrase in the switch below.
const (
	justNowMax = 45 * time.Second
	aMinuteMax = 90 * time.Second
	minutesMax = 45 * time.Minute
	anHourMax  = 90 * time.Minute
	hoursMax   = 22 * time.Hour
	aDayMax    = 36 * time.Hour
)

// Format renders d as a coarse, human relative phrase, always using the largest
// unit that fits: "just now", "a minute ago", "<n> minutes ago", "an hour ago",
// "<n> hours ago", "a day ago", "<n> days ago". A negative duration (the
// last-access time is in the future because of clock skew) renders as "just
// now". The result is deterministic and offline.
func Format(d time.Duration) string {
	switch {
	case d < justNowMax:
		return "just now"
	case d < aMinuteMax:
		return "a minute ago"
	case d < minutesMax:
		return fmt.Sprintf("%d minutes ago", round(d, time.Minute))
	case d < anHourMax:
		return "an hour ago"
	case d < hoursMax:
		return fmt.Sprintf("%d hours ago", round(d, time.Hour))
	case d < aDayMax:
		return "a day ago"
	default:
		return fmt.Sprintf("%d days ago", round(d, 24*time.Hour))
	}
}

// round returns d divided by unit, rounded half-up to the nearest integer.
func round(d, unit time.Duration) int {
	return int(math.Round(float64(d) / float64(unit)))
}
