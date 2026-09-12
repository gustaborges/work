// Command starter is the official reference "local-path-starter" seed
// component. It reads {"arg":"<token>"} on stdin and classifies the token
// (contracts/cli-work-start.md §Argument classification, FR-032): a
// filesystem-path-looking token emits {"repository":{"path":"<absolute>"}}
// exactly as F1 did; any other bare token emits
// {"repository":{"name":"<token>"}} so internal/locator can resolve it
// through the configured search roots. It never emits git_fetch_urls or
// query. It does not verify that a path is a git repository — the core is
// the final authority on that. See
// specs/001-first-local-work/contracts/ipc-starter.md and
// specs/004-local-clone-locator/contracts/cli-work-start.md.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "local-path-starter:", err)
		os.Exit(1)
	}
}

func run(stdin io.Reader, stdout io.Writer) error {
	data, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("reading stdin: %w", err)
	}

	var in struct {
		Arg string `json:"arg"`
	}
	if err := json.Unmarshal(data, &in); err != nil {
		return fmt.Errorf("invalid input: not a JSON object")
	}
	if strings.TrimSpace(in.Arg) == "" {
		return fmt.Errorf("no argument given")
	}

	var repository map[string]any
	if looksLikePath(in.Arg) {
		abs, err := absolutize(in.Arg)
		if err != nil {
			return fmt.Errorf("resolving %q: %w", in.Arg, err)
		}
		repository = map[string]any{"path": abs}
	} else {
		repository = map[string]any{"name": in.Arg}
	}

	out := map[string]any{"repository": repository}
	enc := json.NewEncoder(stdout)
	return enc.Encode(out)
}

// looksLikePath reports whether arg should be treated as a filesystem path
// rather than a bare logical name: it contains a path separator, starts with
// ".", "..", "~", or a drive letter, or names an existing filesystem entry
// (contracts/cli-work-start.md §Argument classification).
func looksLikePath(arg string) bool {
	if strings.ContainsRune(arg, '/') || strings.ContainsRune(arg, '\\') {
		return true
	}
	if strings.HasPrefix(arg, ".") || strings.HasPrefix(arg, "~") {
		return true
	}
	if hasDriveLetterPrefix(arg) {
		return true
	}
	if _, err := os.Stat(arg); err == nil {
		return true
	}
	return false
}

// hasDriveLetterPrefix reports whether arg starts with a Windows drive
// letter, e.g. "C:\repo" or "C:/repo".
func hasDriveLetterPrefix(arg string) bool {
	if len(arg) < 2 || arg[1] != ':' {
		return false
	}
	c := arg[0]
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// absolutize expands a leading ~ and returns the absolute form of arg.
func absolutize(arg string) (string, error) {
	expanded, err := expandHome(arg)
	if err != nil {
		return "", err
	}
	return filepath.Abs(expanded)
}

// expandHome replaces a leading ~ or ~/ with the user's home directory.
func expandHome(p string) (string, error) {
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if p == "~" {
			return home, nil
		}
		return filepath.Join(home, p[2:]), nil
	}
	return p, nil
}
