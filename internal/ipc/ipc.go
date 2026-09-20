// Package ipc is the core side of the subprocess contract with plugin
// components: exactly one JSON object in on stdin, at most one JSON object out
// on stdout, an exit code, and free-form stderr. No shell is involved: a
// component with a declared runtime is run as "<runtime> <path>", one without
// is executed directly, and in neither case does Work rely on a shebang or an
// exec bit.
package ipc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// stderrTailBytes bounds how much of a component's stderr is kept: enough to
// show why it failed under WORK_DEBUG, never an unbounded buffer.
const stderrTailBytes = 4 << 10

// killGrace bounds how long a cancelled run waits for the killed process's
// pipes to drain, so a grandchild that inherited them cannot hold Work open.
const killGrace = 2 * time.Second

// ErrInvalidResponse marks a component that exited 0 but whose stdout is not
// the single JSON document its role allows.
var ErrInvalidResponse = errors.New("invalid response")

// Target is how to start a component: Path is the entrypoint file, Runtime the
// interpreter that runs it, empty for a native executable.
type Target struct {
	Runtime string
	Path    string
}

func (t Target) String() string {
	if t.Runtime == "" {
		return t.Path
	}
	return t.Runtime + " " + t.Path
}

func (t Target) command(ctx context.Context) *exec.Cmd {
	if t.Runtime != "" {
		return exec.CommandContext(ctx, t.Runtime, t.Path)
	}
	return exec.CommandContext(ctx, t.Path)
}

// tailBuffer keeps only the last max bytes written to it.
type tailBuffer struct {
	max int
	buf []byte
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.buf = append(b.buf, p...)
	if len(b.buf) > b.max {
		b.buf = b.buf[len(b.buf)-b.max:]
	}
	return len(p), nil
}

// RepositoryReference is the transient object a Starter produces and a Locator
// consumes (ADR-0016). F1's seed Starter populates only Path.
type RepositoryReference struct {
	Path         string   `json:"path,omitempty"`
	GitFetchURLs []string `json:"git_fetch_urls,omitempty"`
	Name         string   `json:"name,omitempty"`
	Query        string   `json:"query,omitempty"`
}

// StarterInput is the stdin payload for a Starter.
type StarterInput struct {
	Arg string `json:"arg"`
}

// StarterResponse is a Starter's stdout payload. Only these fields are read by
// the core; any other keys the subprocess emits are ignored (FR-018 trust
// boundary).
type StarterResponse struct {
	Repository RepositoryReference `json:"repository"`
	BaseBranch string              `json:"base_branch,omitempty"`
	StartModes []string            `json:"start_modes,omitempty"`
	Meta       map[string]any      `json:"meta,omitempty"`
	Links      map[string]string   `json:"links,omitempty"`
}

// LocatorInput is the stdin payload for a Repository Locator.
type LocatorInput struct {
	Repository      RepositoryReference `json:"repository"`
	RepositoryRoots []string            `json:"repository_roots"`
}

// LocatorResponse is a Locator's stdout payload.
type LocatorResponse struct {
	Matches []Match `json:"matches"`
}

// Match is one filesystem candidate a Locator found.
type Match struct {
	RepoPath string `json:"repo_path"`
}

// LinkerInput is the stdin payload for a Linker: only the declared inputs
// that resolved, keyed by bare key.
type LinkerInput struct {
	Inputs map[string]any `json:"inputs"`
}

// LinkerResponse is what a Linker discovered. Value is empty when it found
// nothing; a returned value is never empty (see InvokeLinker).
type LinkerResponse struct {
	Value string
}

// ImporterInput is the stdin payload for an Importer: its resolved inputs and
// the one directory it may write to.
type ImporterInput struct {
	Inputs    map[string]any `json:"inputs"`
	OutputDir string         `json:"output_dir"`
}

// Result is the raw outcome of running an entrypoint. ExitCode is -1 when the
// process could not be started or was killed. Exited is true when the process
// did start and ended unsuccessfully — by a non-zero status or by a signal —
// which is how a failure to run is told apart from a failure to start. Stderr
// holds at most the last 4 KiB the process wrote.
type Result struct {
	Stdout   []byte
	Stderr   string
	ExitCode int
	Exited   bool
}

// Run executes t with stdinJSON on its stdin and captures the result.
// A non-zero exit or a failure to start yields a non-nil error alongside
// whatever was captured.
func Run(t Target, stdinJSON []byte) (Result, error) {
	return RunContext(context.Background(), t, stdinJSON)
}

// RunContext is Run with cancellation: when ctx ends the process is killed and
// the error wraps ctx.Err(), so callers can tell an interrupt from a failure.
func RunContext(ctx context.Context, t Target, stdinJSON []byte) (Result, error) {
	cmd := t.command(ctx)
	cmd.Stdin = bytes.NewReader(stdinJSON)
	var out bytes.Buffer
	errTail := &tailBuffer{max: stderrTailBytes}
	cmd.Stdout = &out
	cmd.Stderr = errTail
	cmd.WaitDelay = killGrace

	runErr := cmd.Run()
	res := Result{Stdout: out.Bytes(), Stderr: string(errTail.buf)}

	if runErr == nil {
		return res, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		res.ExitCode = -1
		return res, fmt.Errorf("ipc: %s interrupted: %w", t, ctxErr)
	}
	if ee, ok := errors.AsType[*exec.ExitError](runErr); ok {
		res.ExitCode = ee.ExitCode()
		res.Exited = true
		return res, fmt.Errorf("ipc: %s exited %d", t, res.ExitCode)
	}
	res.ExitCode = -1
	return res, fmt.Errorf("ipc: running %s: %w", t, runErr)
}

// InvokeStarter runs a Starter and parses its response. A non-zero exit or
// unparseable stdout is an error.
func InvokeStarter(t Target, in StarterInput) (*StarterResponse, error) {
	stdin, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	res, err := Run(t, stdin)
	if err != nil {
		return nil, err
	}
	var resp StarterResponse
	if err := json.Unmarshal(bytes.TrimSpace(res.Stdout), &resp); err != nil {
		return nil, fmt.Errorf("ipc: starter %s produced invalid JSON: %w", t, err)
	}
	return &resp, nil
}

// InvokeLocator runs a Locator and parses its response.
func InvokeLocator(t Target, in LocatorInput) (*LocatorResponse, error) {
	stdin, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	res, err := Run(t, stdin)
	if err != nil {
		return nil, err
	}
	var resp LocatorResponse
	if err := json.Unmarshal(bytes.TrimSpace(res.Stdout), &resp); err != nil {
		return nil, fmt.Errorf("ipc: locator %s produced invalid JSON: %w", t, err)
	}
	return &resp, nil
}

// InvokeLinker runs a Linker and interprets its answer. Empty stdout, {} and a
// null or absent "value" all mean "nothing found" (empty LinkerResponse);
// a value that is "" or not a string, or stdout that is not a single JSON
// object, wraps ErrInvalidResponse. The Result is returned in every case so
// the caller can report the exit status and stderr tail.
func InvokeLinker(ctx context.Context, t Target, in LinkerInput) (LinkerResponse, Result, error) {
	stdin, err := json.Marshal(in)
	if err != nil {
		return LinkerResponse{}, Result{}, err
	}
	res, err := RunContext(ctx, t, stdin)
	if err != nil {
		return LinkerResponse{}, res, err
	}
	obj, err := decodeSingleObject(res.Stdout)
	if err != nil {
		return LinkerResponse{}, res, fmt.Errorf("ipc: linker %s: %w", t, err)
	}
	raw, present := obj["value"]
	if !present || string(bytes.TrimSpace(raw)) == "null" {
		return LinkerResponse{}, res, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || value == "" {
		return LinkerResponse{}, res, fmt.Errorf("ipc: linker %s: %w: value must be a non-empty string", t, ErrInvalidResponse)
	}
	return LinkerResponse{Value: value}, res, nil
}

// InvokeImporter runs an Importer. Its stdout must be empty or one JSON
// object, whose content is ignored: everything an Importer produces travels
// through the files it writes under OutputDir.
func InvokeImporter(ctx context.Context, t Target, in ImporterInput) (Result, error) {
	stdin, err := json.Marshal(in)
	if err != nil {
		return Result{}, err
	}
	res, err := RunContext(ctx, t, stdin)
	if err != nil {
		return res, err
	}
	if _, err := decodeSingleObject(res.Stdout); err != nil {
		return res, fmt.Errorf("ipc: importer %s: %w", t, err)
	}
	return res, nil
}

// decodeSingleObject parses stdout as at most one JSON object. Empty output is
// an empty object; a second document, a non-object or malformed JSON wraps
// ErrInvalidResponse.
func decodeSingleObject(stdout []byte) (map[string]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(stdout)
	if len(trimmed) == 0 {
		return nil, nil
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	var obj map[string]json.RawMessage
	if err := dec.Decode(&obj); err != nil || obj == nil {
		return nil, fmt.Errorf("%w: stdout is not a JSON object", ErrInvalidResponse)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: stdout holds more than one JSON document", ErrInvalidResponse)
	}
	return obj, nil
}
