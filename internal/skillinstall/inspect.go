package skillinstall

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	skills "github.com/gustaborges/work/ai-skills"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// fingerprint includes marker bytes, empty directories and every safe node.
func fingerprint(root string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("unsafe node")
		}
		rel, e := filepath.Rel(root, p)
		if e != nil {
			return e
		}
		kind := "f"
		if info.IsDir() {
			kind = "d"
		}
		fmt.Fprintf(h, "%d:%s:%s:", len(filepath.ToSlash(rel)), filepath.ToSlash(rel), kind)
		if !info.IsDir() {
			b, e := os.ReadFile(p)
			if e != nil {
				return e
			}
			fmt.Fprintf(h, "%d:", len(b))
			h.Write(b)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func readMarker(d Destination) (*Marker, error) {
	p := filepath.Join(d.Path, MarkerName)
	info, err := os.Lstat(p)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("unsafe marker")
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var m Marker
	if json.Unmarshal(b, &m) != nil || m.Schema != 1 || m.Skill != d.Skill || m.Algorithm != skills.Algorithm || !digestPattern.MatchString(m.Digest) {
		return nil, nil
	}
	return &m, nil
}
func inspectOnce(d Destination, embedded string) Observation {
	r := Result{Vendor: d.Vendor, Skill: d.Skill, Path: d.Path, Embedded: embedded, State: "absent"}
	fail := func(state, token string) Observation { r.State = state; r.Error = token; return Observation{Result: r} }
	if err := safePath(d); err != nil {
		if errors.Is(err, errUnsafe) {
			return fail("conflict", "skill-target-conflict")
		}
		return fail("unreadable", "skill-io-failed")
	}
	if i, err := os.Lstat(sibling(d, ".transaction.json")); err == nil {
		if !i.Mode().IsRegular() {
			return fail("conflict", "skill-target-conflict")
		}
		return fail("recovery_required", "skill-recovery-required")
	} else if !os.IsNotExist(err) {
		return fail("unreadable", "skill-io-failed")
	}
	if _, err := os.Lstat(d.Path); os.IsNotExist(err) {
		return Observation{Result: r}
	} else if err != nil {
		return fail("unreadable", "skill-io-failed")
	}
	fp, err := fingerprint(d.Path)
	if err != nil {
		if strings.Contains(err.Error(), "unsafe") {
			return fail("conflict", "skill-target-conflict")
		}
		return fail("unreadable", "skill-io-failed")
	}
	actual, err := skills.TreeDigest(os.DirFS(d.Path))
	if err != nil {
		return fail("unreadable", "skill-io-failed")
	}
	r.Actual = &actual
	m, err := readMarker(d)
	if err != nil && !os.IsNotExist(err) {
		if strings.Contains(err.Error(), "unsafe") {
			return fail("conflict", "skill-target-conflict")
		}
		return fail("unreadable", "skill-io-failed")
	}
	r.State = "foreign"
	if m != nil {
		r.Recorded = &m.Digest
		r.State = "modified"
		if actual == m.Digest {
			r.State = "outdated"
			if actual == embedded {
				r.State = "current"
			}
		}
	}
	return Observation{Result: r, Fingerprint: fp}
}

// Observe bounds coherent reads and never creates coordination artifacts.
func Observe(d Destination, embedded string) Observation {
	for range 3 {
		before := identities(d)
		a := inspectOnce(d, embedded)
		a.Identities = before
		if a.Result.Error != "" {
			return a
		}
		before = identities(d)
		b := inspectOnce(d, embedded)
		b.Identities = before
		if sameObservation(a, b) && b.Result.Error == "" {
			return b
		}
	}
	return Observation{Result: Result{Vendor: d.Vendor, Skill: d.Skill, Path: d.Path, Embedded: embedded, State: "recovery_required", Error: "skill-recovery-required"}}
}
func Inspect(d Destination, embedded string) Result { return Observe(d, embedded).Result }

func identities(d Destination) []identity {
	result := []identity{}
	for p := d.Path; ; p = filepath.Dir(p) {
		if info, err := os.Lstat(p); err == nil {
			result = append(result, identity{p, info})
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	return result
}
func sameObservation(a, b Observation) bool {
	if a.Fingerprint != b.Fingerprint || a.Result.State != b.Result.State || a.Result.Error != b.Result.Error {
		return false
	}
	for _, old := range a.Identities {
		found := false
		for _, current := range b.Identities {
			if current.Path == old.Path {
				found = true
				if !os.SameFile(old.Info, current.Info) || old.Info.Mode() != current.Info.Mode() {
					return false
				}
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func safeParents(d Destination, before Observation) error {
	if err := safePath(d); err != nil {
		return err
	}
	current := identities(d)
	for _, old := range before.Identities {
		if old.Path == d.Path {
			continue
		}
		found := false
		for _, now := range current {
			if now.Path == old.Path {
				found = true
				if !os.SameFile(old.Info, now.Info) {
					return errUnsafe
				}
				break
			}
		}
		if !found {
			return errUnsafe
		}
	}
	return nil
}
