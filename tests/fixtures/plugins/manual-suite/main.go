// Command manual-suite supplies offline manual providers. WORK_FIXTURE_READY
// and WORK_FIXTURE_RELEASE are optional pipe paths: opening/writing READY
// announces that partial output exists, and reading RELEASE gates completion.
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
	if err := run(role, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(role string, in io.Reader, out io.Writer) error {
	data, err := io.ReadAll(in)
	if err != nil {
		return err
	}
	if path := os.Getenv("WORK_FIXTURE_LOG"); path != "" {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		var input any
		if json.Unmarshal(data, &input) != nil {
			input = string(data)
		}
		err = json.NewEncoder(f).Encode(map[string]any{"role": role, "stdin": input})
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	if role == "linker" {
		return fmt.Errorf("unexpected Linker invocation")
	}
	var input struct {
		OutputDir string `json:"output_dir"`
	}
	if err := json.Unmarshal(data, &input); err != nil || input.OutputDir == "" {
		return fmt.Errorf("output_dir missing")
	}
	put := func(rel, content string) error {
		p := filepath.Join(input.OutputDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		return os.WriteFile(p, []byte(content), 0o644)
	}
	mode := ""
	for _, tok := range strings.Split(os.Getenv("WORK_FIXTURE_MODE"), ",") {
		if m, ok := strings.CutPrefix(tok, "importer-"); ok {
			mode = m
			break
		}
	}
	switch mode {
	case "empty":
		return nil
	case "exit1":
		return fmt.Errorf("simulated importer failure")
	case "garbage":
		_, err = io.WriteString(out, "invalid JSON")
		return err
	case "overflow":
		_, err = io.WriteString(out, "{}"+strings.Repeat(" ", 2<<20))
		return err
	case "mixed":
		if err := put("notes/clean.md", "clean\n"); err != nil {
			return err
		}
		return put("work-state.json", "{}")
	case "worktree":
		return put("worktree/injected.md", "unsafe\n")
	case "snapshot":
		return put("work-state.json", "{}")
	case "symlink":
		if err := put("notes/real.md", "real\n"); err != nil {
			return err
		}
		return os.Symlink("real.md", filepath.Join(input.OutputDir, "notes", "link.md"))
	case "case-alias":
		if err := put("Notes/a.md", "a\n"); err != nil {
			return err
		}
		return put("notes/b.md", "b\n")
	case "null":
		_, err = io.WriteString(out, "null\n")
		return err
	case "hang":
		if err := put("notes/partial.md", "partial\n"); err != nil {
			return err
		}
		if path := os.Getenv("WORK_FIXTURE_READY"); path != "" {
			if err := os.WriteFile(path, []byte("ready\n"), 0o600); err != nil {
				return err
			}
		}
		if path := os.Getenv("WORK_FIXTURE_RELEASE"); path != "" {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(io.Discard, f)
			return err
		}
		// A blocked pipe, rather than a timer, keeps the child alive until killed.
		r, w, err := os.Pipe()
		if err != nil {
			return err
		}
		defer r.Close()
		defer w.Close()
		_, err = r.Read(make([]byte, 1))
		return err
	case "", "ok":
	default:
		return fmt.Errorf("unknown importer mode %q", mode)
	}
	if err := put("notes/context.md", "imported context\n"); err != nil {
		return err
	}
	return put("refs/source.txt", "saved reference\n")
}
