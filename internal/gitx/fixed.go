package gitx

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// ErrUnavailable marks a source, branch or fixed revision that cannot be
// reached or does not exist. Its message is safe to show: it never contains
// the source location, arguments or Git's own diagnostics.
var ErrUnavailable = errors.New("git source, branch or revision is unavailable")

// ErrUnsafeContent marks committed content that cannot be materialized safely
// (submodules, unsafe paths or special entries).
var ErrUnsafeContent = errors.New("committed content cannot be materialized safely")

var commitName = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

// ValidCommit reports whether s is a full lowercase object name.
func ValidCommit(s string) bool { return commitName.MatchString(s) }

// runCtx executes git without a terminal prompt so a private source fails
// fast instead of blocking a persistent terminal session. It returns stdout.
func runCtx(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return out.Bytes(), &CommandError{Args: args, Stderr: strings.TrimSpace(errb.String()), Err: err}
	}
	return out.Bytes(), nil
}

// unavailable converts any git failure that is not cancellation into the safe
// sentinel, keeping the detailed cause only as an unexported wrap target.
func unavailable(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return fmt.Errorf("%w", ErrUnavailable)
}

// DefaultBranch discovers the branch the remote's HEAD points at. It is used
// only at initial registration; later default-branch changes never switch a
// recorded branch.
func DefaultBranch(ctx context.Context, source string) (string, error) {
	out, err := runCtx(ctx, "", "ls-remote", "--symref", "--", source, "HEAD")
	if err != nil {
		return "", unavailable(ctx, err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if rest, ok := strings.CutPrefix(line, "ref: refs/heads/"); ok {
			if name, _, ok := strings.Cut(rest, "\t"); ok && name != "" {
				return name, nil
			}
		}
	}
	return "", fmt.Errorf("%w", ErrUnavailable)
}

// ResolveBranch returns the exact commit a branch points at right now.
func ResolveBranch(ctx context.Context, source, branch string) (string, error) {
	if err := CheckRefFormat("refs/heads/" + branch); err != nil {
		return "", fmt.Errorf("%w", ErrUnavailable)
	}
	out, err := runCtx(ctx, "", "ls-remote", "--", source, "refs/heads/"+branch)
	if err != nil {
		return "", unavailable(ctx, err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		sha, ref, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if ok && ref == "refs/heads/"+branch && ValidCommit(sha) {
			return sha, nil
		}
	}
	return "", fmt.Errorf("%w", ErrUnavailable)
}

// Fetched is a scratch repository holding one exact commit.
type Fetched struct {
	Dir    string
	Commit string
}

// FetchCommit materializes exactly commit from source into a new bare scratch
// repository under parent, trying the recorded branch tip when the server will
// not serve an arbitrary object. It never falls forward to a different commit.
func FetchCommit(ctx context.Context, source, branch, commit, parent string) (*Fetched, error) {
	if !ValidCommit(commit) {
		return nil, fmt.Errorf("%w", ErrUnavailable)
	}
	dir, err := os.MkdirTemp(parent, "work-git-*")
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*Fetched, error) {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	if _, err := runCtx(ctx, dir, "init", "--bare", "--quiet"); err != nil {
		return fail(err)
	}
	if _, err := runCtx(ctx, dir, "fetch", "--quiet", "--no-tags", "--depth", "1", "--", source, commit); err != nil {
		if ctx.Err() != nil {
			return fail(ctx.Err())
		}
		// Servers that refuse a bare object name still serve the branch; the
		// object must then be the fetched tip or reachable from it.
		if _, err := runCtx(ctx, dir, "fetch", "--quiet", "--no-tags", "--", source, "refs/heads/"+branch+":refs/work/fetched"); err != nil {
			return fail(unavailable(ctx, err))
		}
	}
	if _, err := runCtx(ctx, dir, "cat-file", "-e", commit+"^{commit}"); err != nil {
		return fail(unavailable(ctx, err))
	}
	return &Fetched{Dir: dir, Commit: commit}, nil
}

// Close removes the scratch repository.
func (f *Fetched) Close() { _ = os.RemoveAll(f.Dir) }

// Materialize writes the committed tree of f.Commit to dest, rejecting
// submodules and unsafe entries. It returns the executable bit recorded for
// every regular file (slash-separated relative path), since the host
// filesystem may not be able to represent it.
func (f *Fetched) Materialize(ctx context.Context, dest string) (map[string]bool, error) {
	listing, err := runCtx(ctx, f.Dir, "ls-tree", "-r", "-z", f.Commit)
	if err != nil {
		return nil, unavailable(ctx, err)
	}
	for _, rec := range strings.Split(string(listing), "\x00") {
		if strings.HasPrefix(rec, "160000 ") {
			return nil, fmt.Errorf("%w: submodule", ErrUnsafeContent)
		}
	}
	cmd := exec.CommandContext(ctx, "git", "archive", "--format=tar", f.Commit)
	cmd.Dir = f.Dir
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, unavailable(ctx, err)
	}
	execBits := map[string]bool{}
	extractErr := extractTar(tar.NewReader(stdout), dest, execBits)
	_, _ = io.Copy(io.Discard, stdout)
	if err := cmd.Wait(); err != nil && extractErr == nil {
		extractErr = unavailable(ctx, err)
	}
	if extractErr != nil {
		return nil, extractErr
	}
	return execBits, nil
}

func extractTar(tr *tar.Reader, dest string, execBits map[string]bool) error {
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name := path.Clean(strings.TrimSuffix(h.Name, "/"))
		if h.Typeflag == tar.TypeXGlobalHeader || name == "." {
			continue
		}
		if !filepath.IsLocal(filepath.FromSlash(name)) || strings.ContainsAny(name, "\\\x00") {
			return fmt.Errorf("%w: %q", ErrUnsafeContent, name)
		}
		to := filepath.Join(dest, filepath.FromSlash(name))
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(to, 0o755); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(h.Linkname, to); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
				return err
			}
			mode := os.FileMode(0o644)
			if h.Mode&0o111 != 0 {
				mode = 0o755
				execBits[name] = true
			}
			f, err := os.OpenFile(to, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				_ = f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%w: %q", ErrUnsafeContent, name)
		}
	}
}
