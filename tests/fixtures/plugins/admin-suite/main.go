// Command admin-suite is an offline management fixture. Synchronization uses
// stdin/stdout pipes, so every role can block without polling files.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	role := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
	reader := bufio.NewReader(os.Stdin)
	decoder := json.NewDecoder(reader)
	var data json.RawMessage
	err := decoder.Decode(&data)
	if err != nil {
		panic(err)
	}
	if log := os.Getenv("WORK_ADMIN_LOG"); log != "" {
		f, err := os.OpenFile(log, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			panic(err)
		}
		_ = json.NewEncoder(f).Encode(map[string]any{"role": role, "input": string(data)})
		_ = f.Close()
	}
	if os.Getenv("WORK_ADMIN_BLOCK") == "pipe" {
		_, _ = fmt.Fprintln(os.Stdout, "ready:"+role)
		var b [1]byte
		remaining := io.MultiReader(decoder.Buffered(), reader)
		for b[0] != 1 {
			if _, err := io.ReadFull(remaining, b[:]); err != nil {
				panic(err)
			}
		}
	}

	if os.Getenv("WORK_ADMIN_FAIL") == role {
		os.Exit(1)
	}
	var out any
	switch role {
	case "locator":
		paths := []string{}
		_ = json.Unmarshal([]byte(os.Getenv("WORK_ADMIN_MATCHES")), &paths)
		matches := []map[string]string{}
		for _, p := range paths {
			matches = append(matches, map[string]string{"repo_path": p})
		}
		out = map[string]any{"matches": matches}
	case "starter":
		out = map[string]any{"repository": map[string]string{"name": os.Getenv("WORK_ADMIN_NAME")}}
	case "linker":
		out = map[string]string{"value": "https://example.test/admin"}
	case "importer":
		var in struct {
			OutputDir string `json:"output_dir"`
		}
		_ = json.Unmarshal(data, &in)
		if err := os.WriteFile(filepath.Join(in.OutputDir, "admin.txt"), []byte("admin\n"), 0600); err != nil {
			panic(err)
		}
		out = map[string]any{}
	default:
		panic("unknown fixture role")
	}
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		panic(err)
	}
}
