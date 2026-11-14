package skillinstall

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	skills "github.com/gustaborges/work/ai-skills"
)

// Resolve performs selection and path validation without creating directories.
func Resolve(o Options) ([]Destination, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return resolveAt(o, cwd)
}

func resolveAt(o Options, cwd string) ([]Destination, error) {
	if o.Scope != "local" && o.Scope != "user" {
		return nil, fmt.Errorf("--scope must be local or user")
	}
	if o.Vendor != "claude" && o.Vendor != "codex" {
		return nil, fmt.Errorf("--vendor must select exactly one of claude or codex")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	anchor := cwd
	suffix := filepath.Join(".claude", "skills")
	if o.Vendor == "codex" {
		suffix = filepath.Join(".agents", "skills")
	}
	if o.Scope == "local" {
		info, err := os.Stat(cwd)
		if err != nil || !info.IsDir() {
			return nil, fmt.Errorf("current directory must exist and be a directory")
		}
	} else if o.Vendor == "claude" {
		anchor = os.Getenv("CLAUDE_CONFIG_DIR")
		if anchor == "" {
			anchor = filepath.Join(home, ".claude")
		}
		if !filepath.IsAbs(anchor) {
			anchor = filepath.Join(cwd, anchor)
		}
		suffix = "skills"
	} else {
		anchor = filepath.Join(home, ".agents", "skills")
		suffix = ""
	}
	anchor, err = canonicalPrefix(anchor)
	if err != nil {
		return nil, err
	}
	ds := []Destination{}
	for _, name := range skills.Names() {
		ds = append(ds, Destination{o.Vendor, name, o.Scope, anchor, filepath.Join(anchor, suffix, name)})
	}
	sort.Slice(ds, func(i, j int) bool {
		if ds[i].Vendor != ds[j].Vendor {
			return ds[i].Vendor < ds[j].Vendor
		}
		return ds[i].Skill < ds[j].Skill
	})
	for i, a := range ds {
		for _, b := range ds[i+1:] {
			x, y := strings.ToLower(a.Path), strings.ToLower(b.Path)
			if x == y && a.Path != b.Path || strings.HasPrefix(x, y+string(filepath.Separator)) || strings.HasPrefix(y, x+string(filepath.Separator)) {
				return nil, fmt.Errorf("overlapping or ambiguous skill targets")
			}
		}
	}
	return ds, nil
}
func canonicalPrefix(p string) (string, error) {
	p, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	tail := []string{}
	prefix := p
	for {
		_, err = os.Lstat(prefix)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(prefix)
		if parent == prefix {
			return "", err
		}
		tail = append(tail, filepath.Base(prefix))
		prefix = parent
	}
	prefix, err = filepath.EvalSymlinks(prefix)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(prefix)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("anchor is not a directory")
	}
	for i := len(tail) - 1; i >= 0; i-- {
		prefix = filepath.Join(prefix, tail[i])
	}
	return prefix, nil
}

var errUnsafe = errors.New("unsafe skill target")

func safePath(d Destination) error {
	rel, err := filepath.Rel(d.Anchor, d.Path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errUnsafe
	}
	// The selected anchor may have been canonicalized once, but substitutions
	// after selection must never be followed, including above a missing anchor.
	p := filepath.VolumeName(d.Path) + string(filepath.Separator)
	for _, part := range strings.Split(strings.TrimPrefix(d.Path, p), string(filepath.Separator)) {
		p = filepath.Join(p, part)
		info, e := os.Lstat(p)
		if os.IsNotExist(e) {
			return nil
		}
		if e != nil {
			return e
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return errUnsafe
		}
	}
	return nil
}
func sibling(d Destination, suffix string) string {
	return filepath.Join(filepath.Dir(d.Path), ".work-skill-"+d.Skill+suffix)
}
