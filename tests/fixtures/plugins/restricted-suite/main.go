// Command restricted-suite is the companion fixture to context-suite: every
// component it declares is ineligible at start:finalized and therefore must
// never be launched. Each launch is appended to WORK_FIXTURE_LOG so a test can
// assert the log holds no entry for any of them.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	role := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
	stdin, _ := io.ReadAll(os.Stdin)
	if path := os.Getenv("WORK_FIXTURE_LOG"); path != "" {
		if f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			payload := json.RawMessage(stdin)
			if !json.Valid(stdin) {
				payload, _ = json.Marshal(string(stdin))
			}
			line, _ := json.Marshal(map[string]any{"role": role, "stdin": payload})
			fmt.Fprintf(f, "%s\n", line)
			f.Close()
		}
	}
	fmt.Println("{}")
}
