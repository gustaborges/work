package branchname

import (
	"strings"

	"github.com/gustaborges/work/internal/diag"
)

// ValidateSlug enforces the prompt-level slug rules before a branch name is
// derived from it. Git's check-ref-format (via Validate) remains the
// authoritative check on the derived branch name; these rules only reject the
// obvious cases early with a precise message. A failure carries
// diag.InvalidBranchName so the exit code matches git's own rejection of the
// derived name (exit 13) and an interactive field can show it and recover.
func ValidateSlug(s string) error {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return diag.New(diag.InvalidBranchName, "the slug is empty")
	case strings.ContainsAny(s, " \t\n\r"):
		return diag.Newf(diag.InvalidBranchName, "the slug %q must not contain whitespace", s)
	case strings.Contains(s, ".."):
		return diag.Newf(diag.InvalidBranchName, "the slug %q must not contain '..'", s)
	case strings.HasPrefix(s, "-"):
		return diag.Newf(diag.InvalidBranchName, "the slug %q must not start with '-'", s)
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return diag.Newf(diag.InvalidBranchName, "the slug %q must not contain control characters", s)
		}
	}
	return nil
}
