// Command mixedref is a test helper: a Starter that emits a repository
// reference carrying name, query, and git_fetch_urls together with no path,
// exercising the widened starter.Reference (research R8).
package main

import (
	"encoding/json"
	"io"
	"os"
)

func main() {
	io.ReadAll(os.Stdin)
	out := map[string]any{"repository": map[string]any{
		"name":           "acme",
		"query":          "billing",
		"git_fetch_urls": []string{"https://example.com/acme.git"},
	}}
	json.NewEncoder(os.Stdout).Encode(out)
}
