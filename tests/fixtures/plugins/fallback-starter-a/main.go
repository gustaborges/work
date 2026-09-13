// Command starter is the "fallback-starter-a" test fixture: a fallback-layer
// Starter (empty pattern) for tests/plugininstall's fallback-uniqueness
// coverage (FR-011). It behaves like the seed's local-path-starter: it
// echoes the argument back as a repository name, with no start_modes.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func main() {
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "fallback-starter-a:", err)
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
	out := map[string]any{"repository": map[string]any{"name": in.Arg}}
	return json.NewEncoder(stdout).Encode(out)
}
