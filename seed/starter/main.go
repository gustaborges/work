// Command starter is the official reference "local-path-starter" seed
// component. It reads {"arg":"<path>"} on stdin and emits
// {"repository":{"path":"<absolute path>"}} on stdout. It does not verify that
// the path is a git repository — the core is the final authority on that. See
// specs/001-first-local-work/contracts/ipc-starter.md.
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
		return fmt.Errorf("no path given")
	}

	abs, err := filepath.Abs(in.Arg)
	if err != nil {
		return fmt.Errorf("resolving %q: %w", in.Arg, err)
	}

	out := map[string]any{"repository": map[string]any{"path": abs}}
	enc := json.NewEncoder(stdout)
	return enc.Encode(out)
}
