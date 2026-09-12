// Command nameonly is a test helper: a Starter that emits a name-only
// repository reference, exercising the widened starter.Reference (research
// R8) with no path field at all.
package main

import (
	"encoding/json"
	"io"
	"os"
)

func main() {
	io.ReadAll(os.Stdin)
	out := map[string]any{"repository": map[string]any{"name": "payments"}}
	json.NewEncoder(os.Stdout).Encode(out)
}
