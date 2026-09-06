// Package reltime formats a time.Duration as a coarse, human relative phrase
// ("just now", "a minute ago", "3 hours ago", "2 days ago") for the resume and
// archive lists. It is deterministic and offline; a negative duration (clock
// skew) formats as "just now".
package reltime
