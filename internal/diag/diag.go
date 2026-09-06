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
)

// All lists every category including OK, ordered by exit code. Tests assert the
// full table against this slice.
var All = []Category{
	OK, Usage, InvalidPath, UnusableRepo, NoBaseBranch, InvalidBranchName,
	BranchCollision, DestinationUnavailable, BootstrapFailed, MaterializationFailed,
	Cancelled,
}

// Error is a failure tagged with a Category. Msg is shown to the user; Err, when
// present, is an internal cause kept for wrapping and never displayed.
type Error struct {
	Category Category
	Msg      string
	Err      error
}

func (e *Error) Error() string {
	if e.Err != nil && e.Msg == "" {
		return e.Err.Error()
	}
	return e.Msg
}

func (e *Error) Unwrap() error { return e.Err }

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
