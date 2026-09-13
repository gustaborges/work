// Command starter is the "colliding-starter" test fixture: a specific-layer
// Starter whose pattern ("^demo-pr-\d+$") overlaps specific-starter's exact
// match on "demo-pr-1", for tests/contract and tests/integration's Starter
// collision coverage (US4). Its repository name is prefixed so a test can
// tell it apart from specific-starter's response when the collision is
// resolved in its favour.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func main() {
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "colliding-starter:", err)
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
		"repository":  map[string]any{"name": "colliding:" + in.Arg},
		"base_branch": "feature/source-branch",
		"start_modes": []string{"contribution", "fork"},
	}
	return json.NewEncoder(stdout).Encode(out)
}
