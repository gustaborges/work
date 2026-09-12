// Command dupe is a fake Repository Locator test fixture: it returns every
// path in LOCATOR_FIXTURE_MATCHES twice (as distinct raw strings — e.g. a
// relative path and a symlinked path to the same repository), exercising
// core-side dedup on the resolved, symlink-evaluated path (FR-017, SC-009).
package main

import (
	"encoding/json"
	"io"
	"os"
)

func main() {
	io.ReadAll(os.Stdin)

	var paths []string
	json.Unmarshal([]byte(os.Getenv("LOCATOR_FIXTURE_MATCHES")), &paths)

	matches := []map[string]string{}
	for _, p := range paths {
		matches = append(matches, map[string]string{"repo_path": p})
		matches = append(matches, map[string]string{"repo_path": p})
	}
	json.NewEncoder(os.Stdout).Encode(map[string]any{"matches": matches})
}
