// Command ok is a fake Repository Locator test fixture: it returns exactly
// one match, taken from LOCATOR_FIXTURE_MATCHES (a JSON array of paths; only
// the first entry is used). It also echoes what it received on stdin under
// "received"/"received_roots", so tests/contract can assert the projected
// LocatorInput payload (contracts/repository-reference.md).
package main

import (
	"encoding/json"
	"io"
	"os"
)

type input struct {
	Repository      map[string]any `json:"repository"`
	RepositoryRoots []string       `json:"repository_roots"`
}

func main() {
	data, _ := io.ReadAll(os.Stdin)
	var in input
	json.Unmarshal(data, &in)

	var paths []string
	json.Unmarshal([]byte(os.Getenv("LOCATOR_FIXTURE_MATCHES")), &paths)

	matches := []map[string]string{}
	if len(paths) > 0 {
		matches = append(matches, map[string]string{"repo_path": paths[0]})
	}

	json.NewEncoder(os.Stdout).Encode(map[string]any{
		"matches":        matches,
		"received":       in.Repository,
		"received_roots": in.RepositoryRoots,
	})
}
