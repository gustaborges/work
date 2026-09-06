package tui

import (
	"errors"
	"strings"

	huh "charm.land/huh/v2"

	"github.com/gustaborges/work/internal/diag"
)

// abort maps a huh abort into a diag cancelled error; other errors pass through.
func abort(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, huh.ErrUserAborted) {
		return diag.New(diag.Cancelled, "cancelled")
	}
	return err
}

func run(field huh.Field) error {
	return abort(huh.NewForm(huh.NewGroup(field)).Run())
}

// SelectBaseBranch shows grouped base-branch options and returns the chosen
// label's index into options. groupTags parallels options ("local" /
// "remote-tracking") only for display.
func SelectBaseBranch(options []string) (int, error) {
	if len(options) == 0 {
		return 0, diag.New(diag.NoBaseBranch, "the repository has no selectable base branch")
	}
	var choice int
	opts := make([]huh.Option[int], len(options))
	for i, label := range options {
		opts[i] = huh.NewOption(label, i)
	}
	err := run(huh.NewSelect[int]().
		Title("Base branch").
		Options(opts...).
		Value(&choice))
	return choice, err
}

// SelectPrefix shows the convention's prefixes and returns the chosen one.
func SelectPrefix(prefixes []string) (string, error) {
	if len(prefixes) == 1 {
		return prefixes[0], nil
	}
	var choice string
	err := run(huh.NewSelect[string]().
		Title("Branch prefix").
		Options(huh.NewOptions(prefixes...)...).
		Value(&choice))
	return choice, err
}

// InputSlug prompts for the slug with inline validation.
func InputSlug() (string, error) {
	var slug string
	err := run(huh.NewInput().
		Title("Slug").
		Description("a short identifier for this Work").
		Validate(ValidateSlug).
		Value(&slug))
	return strings.TrimSpace(slug), err
}

// InputPath prompts for a local repository path.
func InputPath() (string, error) {
	var path string
	err := run(huh.NewInput().
		Title("Local repository path").
		Validate(func(s string) error {
			if strings.TrimSpace(s) == "" {
				return errors.New("a path is required")
			}
			return nil
		}).
		Value(&path))
	return strings.TrimSpace(path), err
}

// EditWorkspaceRoot prompts for the workspace root, pre-filled with suggested.
func EditWorkspaceRoot(suggested string) (string, error) {
	value := suggested
	err := run(huh.NewInput().
		Title("Workspace root").
		Description("where materialized Works are kept").
		Validate(func(s string) error {
			if strings.TrimSpace(s) == "" {
				return errors.New("a path is required")
			}
			return nil
		}).
		Value(&value))
	return strings.TrimSpace(value), err
}

// ConfirmCreate shows summary and asks for a yes/no confirmation.
func ConfirmCreate(summary string) (bool, error) {
	var ok bool
	err := run(huh.NewConfirm().
		Title(summary).
		Affirmative("Create").
		Negative("Cancel").
		Value(&ok))
	if err != nil {
		return false, err
	}
	return ok, nil
}

// ValidateSlug enforces the prompt-level slug rules. Git remains the
// authoritative check on the derived branch name.
func ValidateSlug(s string) error {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return errors.New("the slug is empty")
	case strings.ContainsAny(s, " \t\n\r"):
		return errors.New("the slug must not contain whitespace")
	case strings.Contains(s, ".."):
		return errors.New("the slug must not contain '..'")
	case strings.HasPrefix(s, "-"):
		return errors.New("the slug must not start with '-'")
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return errors.New("the slug must not contain control characters")
		}
	}
	return nil
}
