// Package diag defines the typed failure categories the CLI reports. Each
// category has a fixed process exit code and a stable machine-readable token so
// that scripts can branch on outcomes; the human message is carried per-error
// and never interpolates repository contents, remote URLs, or environment
// secrets.
package diag

import (
	"errors"
	"fmt"
)

// Category classifies a failure. The zero Category is not valid; use one of the
// package-level values.
type Category struct {
	// Token is the stable, lowercase, hyphenated identifier emitted on stderr
	// and used by callers to recognise the outcome.
	Token string
	// Code is the process exit code for this category.
	Code int
}

// Categories, ordered by exit code. OK is the success sentinel and is never
// carried by an Error.
var (
	OK                     = Category{Token: "ok", Code: 0}
	Usage                  = Category{Token: "usage", Code: 2}
	InvalidPath            = Category{Token: "invalid-path", Code: 10}
	UnusableRepo           = Category{Token: "unusable-repo", Code: 11}
	NoBaseBranch           = Category{Token: "no-base-branch", Code: 12}
	InvalidBranchName      = Category{Token: "invalid-branch-name", Code: 13}
	BranchCollision        = Category{Token: "branch-collision", Code: 14}
	DestinationUnavailable = Category{Token: "destination-unavailable", Code: 15}
	BootstrapFailed        = Category{Token: "bootstrap-failed", Code: 16}
	MaterializationFailed  = Category{Token: "materialization-failed", Code: 17}
	Cancelled              = Category{Token: "cancelled", Code: 20}

	// F2 (daily cycle) categories.
	TargetNotFound     = Category{Token: "target-not-found", Code: 21}
	TargetArchived     = Category{Token: "target-archived", Code: 22}
	DirtyWorktree      = Category{Token: "dirty-worktree", Code: 23}
	ArchiveFailed      = Category{Token: "archive-failed", Code: 24}
	SnapshotUnreadable = Category{Token: "snapshot-unreadable", Code: 25}

	// F3 (local clone locator) categories. See internal/locator for the
	// resolution outcomes that produce 26-29; repository-ambiguous (30) is
	// constructed by the CLI when a resolved ambiguity cannot be prompted.
	NoRepositoryFound          = Category{Token: "no-repository-found", Code: 26}
	NoEligibleLocator          = Category{Token: "no-eligible-locator", Code: 27}
	RepositoryCandidateInvalid = Category{Token: "repository-candidate-invalid", Code: 28}
	LocatorFailed              = Category{Token: "locator-failed", Code: 29}
	RepositoryAmbiguous        = Category{Token: "repository-ambiguous", Code: 30}
)

// All lists every category including OK, ordered by exit code. Tests assert the
// full table against this slice.
var All = []Category{
	OK, Usage, InvalidPath, UnusableRepo, NoBaseBranch, InvalidBranchName,
	BranchCollision, DestinationUnavailable, BootstrapFailed, MaterializationFailed,
	Cancelled, TargetNotFound, TargetArchived, DirtyWorktree, ArchiveFailed,
	SnapshotUnreadable, NoRepositoryFound, NoEligibleLocator, RepositoryCandidateInvalid,
	LocatorFailed, RepositoryAmbiguous,
}

// Error is a failure tagged with a Category. Msg is shown to the user; Err, when
// present, is an internal cause kept for wrapping and never displayed.
//
// Summary and Hint are optional and drive only the interactive diagnostic
// renderer (contracts/diagnostics.md): Summary is a user-vocabulary statement of
// what failed, Hint the known next action. They never affect the non-interactive
// "error: <token>: <Msg>" line, the exit code, or the token.
type Error struct {
	Category Category
	Msg      string
	Err      error
	Summary  string
	Hint     string
}

func (e *Error) Error() string {
	if e.Err != nil && e.Msg == "" {
		return e.Err.Error()
	}
	return e.Msg
}

func (e *Error) Unwrap() error { return e.Err }

// Cause is an alias for Unwrap: the retained internal cause, or nil. It is
// surfaced to the user only through the WORK_DEBUG affordance, never in normal
// output (FR-016).
func (e *Error) Cause() error { return e.Err }

// WithSummary returns e with its interactive Summary set. It mutates and returns
// the receiver for fluent construction: diag.New(cat, msg).WithSummary(...).
func (e *Error) WithSummary(summary string) *Error {
	e.Summary = summary
	return e
}

// WithHint returns e with its interactive next-action Hint set.
func (e *Error) WithHint(hint string) *Error {
	e.Hint = hint
	return e
}

// New builds an Error with a literal message.
func New(c Category, msg string) *Error {
	return &Error{Category: c, Msg: msg}
}

// Newf builds an Error with a formatted message.
func Newf(c Category, format string, args ...any) *Error {
	return &Error{Category: c, Msg: fmt.Sprintf(format, args...)}
}

// Wrap builds an Error that carries an internal cause alongside the user message.
func Wrap(c Category, cause error, msg string) *Error {
	return &Error{Category: c, Msg: msg, Err: cause}
}

// Wrapf is Wrap with a formatted message.
func Wrapf(c Category, cause error, format string, args ...any) *Error {
	return &Error{Category: c, Msg: fmt.Sprintf(format, args...), Err: cause}
}

// ExitCode returns the process exit code for err: 0 when nil, the carried
// category's code for a *Error anywhere in the chain, otherwise 1.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if d, ok := errors.AsType[*Error](err); ok {
		return d.Category.Code
	}
	return 1
}

// Token returns the stable token for err, or "error" when err carries no
// category.
func Token(err error) string {
	if d, ok := errors.AsType[*Error](err); ok {
		return d.Category.Token
	}
	return "error"
}

// Format renders the single stderr line "error: <token>: <message>".
func Format(err error) string {
	if err == nil {
		return ""
	}
	if d, ok := errors.AsType[*Error](err); ok {
		return fmt.Sprintf("error: %s: %s", d.Category.Token, d.Error())
	}
	return fmt.Sprintf("error: %s", err)
}
