// Command publisher is a test helper: a Starter that publishes meta and links
// next to a name-only repository reference.
package main

import (
	"encoding/json"
	"io"
	"os"
)

func main() {
	io.ReadAll(os.Stdin)
	json.NewEncoder(os.Stdout).Encode(map[string]any{
		"repository": map[string]any{"name": "acme"},
		"meta":       map[string]any{"github.pull_request.number": 212},
		"links":      map[string]any{"github.pull_request": "https://example.test/pr/212"},
		"rogue":      "ignored",
	})
}
