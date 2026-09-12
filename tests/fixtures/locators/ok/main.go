// Command ok is a fake Repository Locator test fixture: it returns exactly
// one match, taken from LOCATOR_FIXTURE_MATCHES (a JSON array of paths; only
// the first entry is used). It also echoes what it received on stdin under
// "received"/"received_roots".
//
// When LOCATOR_FIXTURE_EXPECT_REPOSITORY and/or LOCATOR_FIXTURE_EXPECT_ROOTS
// are set (JSON), it self-checks the exact payload it received against them
// and exits 1 on any mismatch — this lets tests/contract assert the projected
// LocatorInput a real Resolve call sends over the wire (SC-004,
// contracts/repository-reference.md §Tests) without a second, test-only
// entrypoint into internal/locator's unexported projection logic.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
)

type input struct {
	Repository      map[string]any `json:"repository"`
	RepositoryRoots []string       `json:"repository_roots"`
}

func main() {
	data, _ := io.ReadAll(os.Stdin)
	var in input
	json.Unmarshal(data, &in)

	if expect := os.Getenv("LOCATOR_FIXTURE_EXPECT_REPOSITORY"); expect != "" {
		var want map[string]any
		json.Unmarshal([]byte(expect), &want)
		got, _ := json.Marshal(in.Repository)
		wantBytes, _ := json.Marshal(want)
		if string(got) != string(wantBytes) {
			fmt.Fprintf(os.Stderr, "ok: repository mismatch: got %s, want %s\n", got, wantBytes)
			os.Exit(1)
		}
	}
	if expect := os.Getenv("LOCATOR_FIXTURE_EXPECT_ROOTS"); expect != "" {
		var want []string
		json.Unmarshal([]byte(expect), &want)
		if !slices.Equal(in.RepositoryRoots, want) {
			fmt.Fprintf(os.Stderr, "ok: roots mismatch: got %v, want %v\n", in.RepositoryRoots, want)
			os.Exit(1)
		}
	}

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
