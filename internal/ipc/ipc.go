// Package ipc is the core side of the subprocess contract with plugin
// components: exactly one JSON object in on stdin, at most one JSON object out
// on stdout, an exit code, and free-form stderr. Entrypoints are executed
// directly — no shell, no shebang reliance.
package ipc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
)

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

// Result is the raw outcome of running an entrypoint.
type Result struct {
	Stdout   []byte
	Stderr   string
	ExitCode int
}

// Run executes entrypoint with stdinJSON on its stdin and captures the result.
// A non-zero exit or a failure to start yields a non-nil error alongside
// whatever was captured.
func Run(entrypoint string, stdinJSON []byte) (Result, error) {
	cmd := exec.Command(entrypoint)
	cmd.Stdin = bytes.NewReader(stdinJSON)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb

	runErr := cmd.Run()
	res := Result{Stdout: out.Bytes(), Stderr: errb.String()}

	if runErr == nil {
		return res, nil
	}
	if ee, ok := errors.AsType[*exec.ExitError](runErr); ok {
		res.ExitCode = ee.ExitCode()
		return res, fmt.Errorf("ipc: %s exited %d", entrypoint, res.ExitCode)
	}
	res.ExitCode = -1
	return res, fmt.Errorf("ipc: running %s: %w", entrypoint, runErr)
}

// InvokeStarter runs a Starter entrypoint and parses its response. A non-zero
// exit or unparseable stdout is an error.
func InvokeStarter(entrypoint string, in StarterInput) (*StarterResponse, error) {
	stdin, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	res, err := Run(entrypoint, stdin)
	if err != nil {
		return nil, err
	}
	var resp StarterResponse
	if err := json.Unmarshal(bytes.TrimSpace(res.Stdout), &resp); err != nil {
		return nil, fmt.Errorf("ipc: starter %s produced invalid JSON: %w", entrypoint, err)
	}
	return &resp, nil
}

// InvokeLocator runs a Locator entrypoint and parses its response.
func InvokeLocator(entrypoint string, in LocatorInput) (*LocatorResponse, error) {
	stdin, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	res, err := Run(entrypoint, stdin)
	if err != nil {
		return nil, err
	}
	var resp LocatorResponse
	if err := json.Unmarshal(bytes.TrimSpace(res.Stdout), &resp); err != nil {
		return nil, fmt.Errorf("ipc: locator %s produced invalid JSON: %w", entrypoint, err)
	}
	return &resp, nil
}
