// Command starter is the "specific-starter" test fixture: a specific-layer
// Starter (contracts/starter-protocol.md) matching the distinctive literal
// token "demo-pr-1", used by tests/contract's Match coverage and
// tests/integration's full new-Work-from-a-plugin-Starter journey (US2/US3).
// It always resolves the same repository reference shape and offers
// contribution/fork start modes with a base branch, regardless of the
// argument it is given — pattern evaluation happens core-side before this
// binary ever runs (contracts/starter-protocol.md §Selection).
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func main() {
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "specific-starter:", err)
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

	out := map[string]any{
		"repository":  map[string]any{"name": in.Arg},
		"base_branch": "feature/source-branch",
		"start_modes": []string{"contribution", "fork"},
	}
	return json.NewEncoder(stdout).Encode(out)
}
