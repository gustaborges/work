// Command context-suite is the test fixture behind F5's automatic-context
// scenarios. One binary plays every role of the package (starter, linker,
// linker2, importer, importer2), picked from its own executable name, so a
// single go build per entrypoint yields the whole plugin.
//
// WORK_FIXTURE_MODE is a comma-separated list of "<entrypoint>-<mode>"
// tokens; each entrypoint reads only the tokens carrying its own name and
// falls back to plain success when it finds none. WORK_FIXTURE_LOG, when set,
// receives one {role, stdin} JSON line per launch so tests can prove which
// components started and exactly what they were sent.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	role := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
	if err := run(role, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", role, err)
		os.Exit(1)
	}
}

func run(role string, stdin io.Reader, stdout io.Writer) error {
	data, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("reading stdin: %w", err)
	}
	logLaunch(role, data)
	mode := modeFor(role)

	switch role {
	case "starter":
		return runStarter(mode, data, stdout)
	case "linker", "linker2":
		return runLinker(role, mode, stdout)
	case "importer", "importer2":
		return runImporter(role, mode, data, stdout)
	}
	return fmt.Errorf("unknown role %q", role)
}

// modeFor returns the mode suffix of the first WORK_FIXTURE_MODE token that
// starts with "<role>-", or "" when none does.
func modeFor(role string) string {
	for _, tok := range strings.Split(os.Getenv("WORK_FIXTURE_MODE"), ",") {
		if m, ok := strings.CutPrefix(strings.TrimSpace(tok), role+"-"); ok {
			return m
		}
	}
	return ""
}

func logLaunch(role string, stdin []byte) {
	path := os.Getenv("WORK_FIXTURE_LOG")
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	line, _ := json.Marshal(map[string]any{"role": role, "stdin": json.RawMessage(validJSONOr(stdin))})
	fmt.Fprintf(f, "%s\n", line)
}

func validJSONOr(b []byte) []byte {
	if json.Valid(b) {
		return b
	}
	quoted, _ := json.Marshal(string(b))
	return quoted
}

func runStarter(mode string, stdin []byte, stdout io.Writer) error {
	var in struct {
		Arg string `json:"arg"`
	}
	if err := json.Unmarshal(stdin, &in); err != nil {
		return fmt.Errorf("invalid input: not a JSON object")
	}
	out := map[string]any{
		"repository": map[string]any{"name": in.Arg},
	}
	switch mode {
	case "context":
		out["meta"] = map[string]any{"github.pull_request.number": 212}
		out["links"] = map[string]any{"github.pull_request": "https://example.test/pr/212"}
	case "bad-key":
		out["links"] = map[string]any{"GitHub.PR": "x"}
	case "foreign-private":
		out["meta"] = map[string]any{"plugin.someone-else.x": "y"}
	}
	return json.NewEncoder(stdout).Encode(out)
}

func runLinker(role, mode string, stdout io.Writer) error {
	value := func(v any) error { return json.NewEncoder(stdout).Encode(map[string]any{"value": v}) }
	switch mode {
	case "value":
		if role == "linker2" {
			return value("https://example.test/pr/212-from-linker2")
		}
		return value("https://example.test/pr/212-from-linker")
	case "none":
		_, err := io.WriteString(stdout, "{}\n")
		return err
	case "empty-value":
		return value("")
	case "exit1":
		fmt.Fprintln(os.Stderr, "linker: simulated failure")
		os.Exit(1)
	case "garbage":
		_, err := io.WriteString(stdout, "this is not json")
		return err
	case "hang":
		time.Sleep(time.Hour)
		return nil
	case "":
		// Plain success: the first Linker finds a value, the second stays silent
		// so single-Linker scenarios are not perturbed by its presence.
		if role == "linker" {
			return value("https://example.test/pr/212-from-linker")
		}
		_, err := io.WriteString(stdout, "{}\n")
		return err
	}
	return fmt.Errorf("unknown mode %q", mode)
}

func runImporter(role, mode string, stdin []byte, stdout io.Writer) error {
	var in struct {
		OutputDir string `json:"output_dir"`
	}
	if err := json.Unmarshal(stdin, &in); err != nil || in.OutputDir == "" {
		return fmt.Errorf("invalid input: output_dir missing")
	}
	put := func(rel, content string) error {
		p := filepath.Join(in.OutputDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		return os.WriteFile(p, []byte(content), 0o644)
	}

	switch mode {
	case "empty":
		return nil
	case "exit1":
		fmt.Fprintln(os.Stderr, "importer: simulated failure")
		os.Exit(1)
	case "garbage":
		_, err := io.WriteString(stdout, "this is not json")
		return err
	case "hang":
		// Leave partial output behind, then never finish: an interrupt must
		// discard it and still remove the stage.
		if err := put("notes/partial.md", "partial\n"); err != nil {
			return err
		}
		time.Sleep(time.Hour)
		return nil
	case "mixed":
		if err := put("notes/clean.md", "clean\n"); err != nil {
			return err
		}
		return put("work-state.json", "{}\n")
	case "symlink":
		if err := put("notes/real.md", "real\n"); err != nil {
			return err
		}
		return os.Symlink("real.md", filepath.Join(in.OutputDir, "notes", "link.md"))
	case "into-worktree":
		return put("worktree/injected.md", "injected\n")
	case "over-state":
		return put("work-state.json", "{}\n")
	case "collide":
		return put("notes/context.md", "from importer2\n")
	case "ok", "":
		if role == "importer2" {
			return put("notes/importer2.md", "importer2\n")
		}
		return put("notes/context.md", "imported context\n")
	}
	return fmt.Errorf("unknown mode %q", mode)
}
