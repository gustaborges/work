package skillinstall

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	skills "github.com/gustaborges/work/ai-skills"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

var opPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
var fpPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func recordPaths(d Destination, r Record) (string, string) {
	p := filepath.Dir(d.Path)
	return filepath.Join(p, r.Stage), filepath.Join(p, r.Backup)
}
func loadRecord(d Destination) (Record, error) {
	var r Record
	p := sibling(d, ".transaction.json")
	i, err := os.Lstat(p)
	if err != nil {
		return r, err
	}
	if !i.Mode().IsRegular() {
		return r, fmt.Errorf("unsafe record")
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return r, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&r); err != nil {
		return r, err
	}
	if dec.Decode(new(any)) != io.EOF {
		return r, fmt.Errorf("trailing record")
	}
	stem := ".work-skill-" + d.Skill + "." + r.Operation
	if r.Schema != 1 || r.Skill != d.Skill || !opPattern.MatchString(r.Operation) || r.Stage != stem+".stage" || r.Backup != stem+".backup" || (r.Phase != "prepared" && r.Phase != "committed") || !fpPattern.MatchString(r.NewFingerprint) || !digestPattern.MatchString(r.NewDigest) || !fpPattern.MatchString(r.NewMarker) || (r.OldExists && !fpPattern.MatchString(r.OldFingerprint)) || (!r.OldExists && r.OldFingerprint != "") {
		return r, fmt.Errorf("invalid recovery record")
	}
	return r, nil
}
func treeEvidence(p string) (bool, string, error) {
	i, err := os.Lstat(p)
	if os.IsNotExist(err) {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	if !i.IsDir() || i.Mode()&os.ModeSymlink != 0 {
		return true, "", fmt.Errorf("unsafe evidence")
	}
	fp, err := fingerprint(p)
	return true, fp, err
}

// recoverTarget is called only while the persistent destination lock is held.
// Every deletion requires matching evidence. Unlisted combinations are retained.
func recoverTarget(d Destination) error { return (Engine{}).recoverTarget(d) }

func (e Engine) recoverTarget(d Destination) error {
	if err := safePath(d); err != nil {
		return err
	}
	before := Observation{Identities: identities(d)}
	r, err := loadRecord(d)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	stage, backup := recordPaths(d, r)
	te, tf, err := treeEvidence(d.Path)
	if err != nil {
		return err
	}
	se, sf, err := treeEvidence(stage)
	if err != nil {
		return err
	}
	be, bf, err := treeEvidence(backup)
	if err != nil {
		return err
	}
	validNew := func(p string) bool {
		candidate := d
		candidate.Path = p
		m, err := readMarker(candidate)
		if err != nil || m == nil || m.Digest != r.NewDigest {
			return false
		}
		actual, err := skills.TreeDigest(os.DirFS(p))
		if err != nil || actual != r.NewDigest {
			return false
		}
		b, err := os.ReadFile(filepath.Join(p, MarkerName))
		if err != nil {
			return false
		}
		h := sha256.Sum256(b)
		return hex.EncodeToString(h[:]) == r.NewMarker
	}
	if se && !validNew(stage) || te && tf == r.NewFingerprint && !validNew(d.Path) {
		return fmt.Errorf("invalid new marker evidence")
	}
	if se && sf != r.NewFingerprint || be && (!r.OldExists || bf != r.OldFingerprint) {
		return fmt.Errorf("mismatched recovery evidence")
	}
	remove := func(p, want string) error {
		if err := safeParents(d, before); err != nil {
			return err
		}
		exists, fp, err := treeEvidence(p)
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}
		if fp != want {
			return fmt.Errorf("changed recovery evidence")
		}
		if err := os.RemoveAll(p); err != nil {
			return err
		}
		return e.checkpoint("removed:" + filepath.Base(p))
	}
	if r.Phase == "committed" {
		if !te || tf != r.NewFingerprint {
			return fmt.Errorf("committed tree mismatch")
		}
		if be {
			if err = remove(backup, r.OldFingerprint); err != nil {
				return err
			}
		}
		if se {
			if err = remove(stage, r.NewFingerprint); err != nil {
				return err
			}
		}
		if err := os.Remove(sibling(d, ".transaction.json")); err != nil {
			return err
		}
		return e.checkpoint("record-removed")
	}
	if r.OldExists {
		if be {
			if te && tf != r.NewFingerprint {
				return fmt.Errorf("unexpected rollback target")
			}
			if te {
				if err = remove(d.Path, r.NewFingerprint); err != nil {
					return err
				}
			}
			if err = safeParents(d, before); err != nil {
				return err
			}
			if err = os.Rename(backup, d.Path); err != nil {
				return err
			}
			if err = e.checkpoint("old-restored"); err != nil {
				return err
			}
		} else if !te || tf != r.OldFingerprint {
			return fmt.Errorf("old tree unavailable")
		}
	} else {
		if be || te && tf != r.NewFingerprint {
			return fmt.Errorf("unexpected initial target")
		}
		if te {
			if err = remove(d.Path, r.NewFingerprint); err != nil {
				return err
			}
		}
	}
	if se {
		if err = remove(stage, r.NewFingerprint); err != nil {
			return err
		}
	}
	if err := os.Remove(sibling(d, ".transaction.json")); err != nil {
		return err
	}
	return e.checkpoint("record-removed")
}
