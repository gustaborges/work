// Package gitx is a thin wrapper over the system git binary invoked with
// os/exec. The core never links a git library; every ref, worktree, and
// branch operation goes through the commands here so that git's own rules
// (notably check-ref-format) are authoritative.
package gitx

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// MinVersion is the lowest git version the tool supports. 2.5 introduced
// `git worktree add -b`.
var MinVersion = [2]int{2, 5}

// CommandError carries a failed git invocation's arguments and stderr.
type CommandError struct {
	Args   []string
	Stderr string
	Err    error
}

func (e *CommandError) Error() string {
	if e.Stderr != "" {
		return fmt.Sprintf("git %s: %s", strings.Join(e.Args, " "), e.Stderr)
	}
	return fmt.Sprintf("git %s: %v", strings.Join(e.Args, " "), e.Err)
}

func (e *CommandError) Unwrap() error { return e.Err }

// run executes `git <args...>` and returns trimmed stdout.
func run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return out.String(), &CommandError{Args: args, Stderr: strings.TrimSpace(errb.String()), Err: err}
	}
	return out.String(), nil
}

// isCleanNonZero reports whether err is a git command that ran but exited
// non-zero (as opposed to git failing to start).
func isCleanNonZero(err error) bool {
	_, ok := errors.AsType[*exec.ExitError](err)
	return ok
}

// exitOK reports whether `git <args...>` exited zero, distinguishing a clean
// non-zero exit (ok=false, err=nil) from an operational failure (err!=nil).
func exitOK(args ...string) (bool, error) {
	_, err := run(args...)
	if err == nil {
		return true, nil
	}
	if isCleanNonZero(err) {
		return false, nil
	}
	return false, err
}

// Preflight verifies git is on PATH and new enough. Call before any mutation.
func Preflight() error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found on PATH: %w", err)
	}
	v, err := Version()
	if err != nil {
		return err
	}
	maj, min, err := parseVersion(v)
	if err != nil {
		return err
	}
	if maj < MinVersion[0] || (maj == MinVersion[0] && min < MinVersion[1]) {
		return fmt.Errorf("git %s is too old; need >= %d.%d", v, MinVersion[0], MinVersion[1])
	}
	return nil
}

// Version returns the running git's version string, e.g. "2.43.0".
func Version() (string, error) {
	out, err := run("--version")
	if err != nil {
		return "", err
	}
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) < 3 || fields[0] != "git" || fields[1] != "version" {
		return "", fmt.Errorf("unexpected `git --version` output: %q", out)
	}
	return fields[2], nil
}

func parseVersion(v string) (major, minor int, err error) {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("cannot parse git version %q", v)
	}
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("cannot parse git version %q: %w", v, err)
	}
	// Minor may carry a suffix like "2" in "2.39.3.windows.1"; take leading digits.
	minorDigits := parts[1]
	for i, r := range minorDigits {
		if r < '0' || r > '9' {
			minorDigits = minorDigits[:i]
			break
		}
	}
	if minorDigits == "" {
		return major, 0, nil
	}
	minor, err = strconv.Atoi(minorDigits)
	if err != nil {
		return 0, 0, fmt.Errorf("cannot parse git version %q: %w", v, err)
	}
	return major, minor, nil
}

// CheckRefFormat runs `git check-ref-format <ref>`; ref must be fully qualified
// (e.g. "refs/heads/feature/x"). A nil return means the name is syntactically
// valid.
func CheckRefFormat(ref string) error {
	ok, err := exitOK("check-ref-format", ref)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("invalid ref name: %s", ref)
	}
	return nil
}

// Repo is a git repository (or working tree) rooted at Dir.
type Repo struct {
	Dir string
}

// Open binds a Repo to a directory. No validation is performed here.
func Open(dir string) Repo { return Repo{Dir: dir} }

func (r Repo) run(args ...string) (string, error) {
	return run(append([]string{"-C", r.Dir}, args...)...)
}

func (r Repo) exitOK(args ...string) (bool, error) {
	return exitOK(append([]string{"-C", r.Dir}, args...)...)
}

// Run executes an arbitrary `git -C <dir> <args...>` and returns trimmed
// stdout. Prefer the named helpers; this exists for one-off queries.
func (r Repo) Run(args ...string) (string, error) {
	out, err := r.run(args...)
	return strings.TrimSpace(out), err
}

// IsWorkTree reports whether Dir is inside a git working tree.
func (r Repo) IsWorkTree() (bool, error) {
	out, err := r.run("rev-parse", "--is-inside-work-tree")
	if err != nil {
		if isCleanNonZero(err) {
			return false, nil
		}
		return false, err
	}
	return strings.TrimSpace(out) == "true", nil
}

// IsBare reports whether Dir is a bare repository.
func (r Repo) IsBare() (bool, error) {
	out, err := r.run("rev-parse", "--is-bare-repository")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "true", nil
}

// HasCommit reports whether HEAD resolves to a commit (false for an unborn
// branch).
func (r Repo) HasCommit() (bool, error) {
	return r.exitOK("rev-parse", "--verify", "--quiet", "HEAD")
}

// Ref is one row of ForEachRef.
type Ref struct {
	Refname     string // e.g. refs/heads/main
	ObjectShort string // short object name
	Short       string // e.g. main, origin/main
	Upstream    string // upstream short name, may be empty
}

// ForEachRef lists refs under the given patterns. refs/remotes/*/HEAD is
// dropped.
func (r Repo) ForEachRef(patterns ...string) ([]Ref, error) {
	args := append([]string{
		"for-each-ref",
		"--format=%(refname)%09%(objectname:short)%09%(refname:short)%09%(upstream:short)",
	}, patterns...)
	out, err := r.run(args...)
	if err != nil {
		return nil, err
	}
	var refs []Ref
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		cols := strings.Split(line, "\t")
		for len(cols) < 4 {
			cols = append(cols, "")
		}
		if strings.HasSuffix(cols[0], "/HEAD") && strings.HasPrefix(cols[0], "refs/remotes/") {
			continue
		}
		refs = append(refs, Ref{
			Refname:     cols[0],
			ObjectShort: cols[1],
			Short:       cols[2],
			Upstream:    cols[3],
		})
	}
	return refs, sc.Err()
}

// ShowRefVerify reports whether a fully-qualified ref exists locally.
func (r Repo) ShowRefVerify(ref string) (bool, error) {
	return r.exitOK("show-ref", "--verify", "--quiet", ref)
}

// Worktree is one entry of `git worktree list --porcelain`.
type Worktree struct {
	Path     string
	Head     string
	Branch   string // fully-qualified, e.g. refs/heads/main; empty if detached
	Bare     bool
	Detached bool
}

// WorktreeList parses `git worktree list --porcelain`.
func (r Repo) WorktreeList() ([]Worktree, error) {
	out, err := r.run("worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var (
		list []Worktree
		cur  Worktree
		have bool
	)
	flush := func() {
		if have {
			list = append(list, cur)
		}
		cur = Worktree{}
		have = false
	}
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			flush()
		case strings.HasPrefix(line, "worktree "):
			have = true
			cur.Path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "HEAD "):
			cur.Head = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			cur.Branch = strings.TrimPrefix(line, "branch ")
		case line == "bare":
			cur.Bare = true
		case line == "detached":
			cur.Detached = true
		}
	}
	flush()
	return list, sc.Err()
}

// WorktreeAdd creates branch from base and checks it out at dir in one step.
func (r Repo) WorktreeAdd(dir, branch, base string) error {
	_, err := r.run("worktree", "add", "-b", branch, dir, base)
	return err
}

// WorktreeRemove force-removes a linked worktree.
func (r Repo) WorktreeRemove(dir string) error {
	_, err := r.run("worktree", "remove", "--force", dir)
	return err
}

// BranchDelete force-deletes a local branch.
func (r Repo) BranchDelete(name string) error {
	_, err := r.run("branch", "-D", name)
	return err
}

// RevParse resolves a revision to its full object name.
func (r Repo) RevParse(rev string) (string, error) {
	out, err := r.run("rev-parse", rev)
	return strings.TrimSpace(out), err
}

// CurrentBranch returns the checked-out branch's short name, or "HEAD" when
// detached.
func (r Repo) CurrentBranch() (string, error) {
	out, err := r.run("rev-parse", "--abbrev-ref", "HEAD")
	return strings.TrimSpace(out), err
}

// MergeBase returns the best common ancestor of two revisions.
func (r Repo) MergeBase(a, b string) (string, error) {
	out, err := r.run("merge-base", a, b)
	return strings.TrimSpace(out), err
}
