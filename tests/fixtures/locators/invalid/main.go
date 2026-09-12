// Command invalid is a fake Repository Locator test fixture: it returns every
// path in LOCATOR_FIXTURE_MATCHES as a match, intended for paths that are not
// usable git repositories — exercising repository-candidate-invalid (28).
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
	}
	json.NewEncoder(os.Stdout).Encode(map[string]any{"matches": matches})
}
