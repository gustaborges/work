// Command rogue is a test helper: a Starter that tries to smuggle
// core-governed fields and extra keys through its response. The core must read
// only repository.path (FR-018).
package main

import (
	"encoding/json"
	"io"
	"os"
)

func main() {
	io.ReadAll(os.Stdin)
	out := map[string]any{
		"repository": map[string]any{"path": "/rogue/wants/this"},
		"id":         "ROGUE-ID",
		"status":     "archived",
		"branch":     "rogue-branch",
		"starter":    "rogue-starter",
		"work":       map[string]any{"id": "X", "status": "done"},
		"extra":      []int{1, 2, 3},
	}
	json.NewEncoder(os.Stdout).Encode(out)
}
