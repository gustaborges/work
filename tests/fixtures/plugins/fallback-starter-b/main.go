// Command starter is the "fallback-starter-b" test fixture: a second
// fallback-layer Starter (empty pattern), distinct from fallback-starter-a,
// for tests/plugininstall's fallback-uniqueness coverage (FR-011) — installing
// this under a different alias while fallback-starter-a is already registered
// must fail plugin-fallback-conflict.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func main() {
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "fallback-starter-b:", err)
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
