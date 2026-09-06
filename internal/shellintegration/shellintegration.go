// Package shellintegration implements the terminal-repositioning contract
// (FR-022, FR-023): the per-shell wrapper snippets emitted by `work shell-init`,
// the WORK_CD_FILE hand-off the core writes on a successful create, and the
// honest notice printed when no integration is active.
package shellintegration

import (
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gustaborges/work/internal/diag"
)

//go:embed snippets/*
var snippets embed.FS

// Supported lists the shells `work shell-init` can emit a snippet for.
var Supported = []string{"bash", "zsh", "fish", "powershell"}

var snippetFile = map[string]string{
	"bash":       "snippets/bash.sh",
	"zsh":        "snippets/zsh.sh",
	"fish":       "snippets/fish.fish",
	"powershell": "snippets/powershell.ps1",
}

// Snippet returns the embedded wrapper snippet for shell. An unknown shell is a
// usage error listing the supported shells.
func Snippet(shell string) (string, error) {
	name, ok := snippetFile[strings.ToLower(strings.TrimSpace(shell))]
	if !ok {
		return "", diag.Newf(diag.Usage,
			"unsupported shell %q; supported shells are: %s", shell, strings.Join(Supported, ", "))
	}
	data, err := snippets.ReadFile(name)
	if err != nil {
		return "", diag.Wrapf(diag.Usage, err, "no snippet for shell %q", shell)
	}
	return string(data), nil
}

// Active reports whether the shell wrapper is present for this process (it sets
// WORK_CD_FILE to a writable path).
func Active() bool {
	return strings.TrimSpace(os.Getenv("WORK_CD_FILE")) != ""
}

// WriteTargetPath writes absWorktree to $WORK_CD_FILE when the wrapper is
// active. It is a no-op (nil) when no integration is present — the caller then
// takes the ReportNoIntegration path.
func WriteTargetPath(absWorktree string) error {
	target := strings.TrimSpace(os.Getenv("WORK_CD_FILE"))
	if target == "" {
		return nil
	}
	if err := os.WriteFile(target, []byte(absWorktree), 0o600); err != nil {
		return diag.Wrapf(diag.MaterializationFailed, err, "cannot hand the new path to the shell")
	}
	return nil
}

// ReportNoIntegration prints the FR-023 notice to w (the core passes stderr):
// it never claims a directory change happened and prints the real worktree
// path on its own line.
func ReportNoIntegration(w io.Writer, absWorktree, detectedShell string) {
	shell := detectedShell
	if _, ok := snippetFile[shell]; !ok {
		shell = "bash"
	}
	fmt.Fprintln(w, "note: this shell session was not moved into the new worktree.")
	fmt.Fprintln(w, "note: worktree path:")
	fmt.Fprintln(w, absWorktree)
	fmt.Fprintln(w, "note: to move automatically next time, add this to your shell startup file:")
	fmt.Fprintf(w, "      eval \"$(work shell-init %s)\"\n", shell)
	if detectedShell == "" || shell != detectedShell {
		fmt.Fprintf(w, "      (supported shells: %s)\n", strings.Join(Supported, ", "))
	}
}

// DetectShell infers the user's shell from the environment. It returns "" when
// it cannot tell.
func DetectShell() string {
	if os.Getenv("FISH_VERSION") != "" {
		return "fish"
	}
	if os.Getenv("ZSH_VERSION") != "" {
		return "zsh"
	}
	if os.Getenv("BASH_VERSION") != "" {
		return "bash"
	}
	if os.Getenv("PSModulePath") != "" && os.Getenv("SHELL") == "" {
		return "powershell"
	}
	if sh := os.Getenv("SHELL"); sh != "" {
		base := strings.ToLower(filepath.Base(sh))
		for _, s := range Supported {
			if base == s {
				return s
			}
		}
	}
	return ""
}
