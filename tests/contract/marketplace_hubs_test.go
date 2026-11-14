package contract

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func readHub(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "internal", "cli", "plugin_hub.go"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Review resources must be released on every path: each review built from a
// frozen plan hands its Close to the presentation layer, and information-only
// reviews never carry Apply.
func TestManagerReviewsOwnTheirCleanup(t *testing.T) {
	src := readHub(t)
	for _, plan := range []string{"PrepareAdd(", "PrepareInstall(", "PrepareUpdates("} {
		if !strings.Contains(src, plan) {
			t.Fatalf("manager does not use %s", plan)
		}
	}
	if got := strings.Count(src, "Close: plan.Close"); got < 3 {
		t.Fatalf("expected add, install and update reviews to register Close, found %d", got)
	}
}

// Checking is pure and distinct from applying: the check review is informational
// and never reaches an Apply callback or a staging prepare.
func TestManagerCheckReviewIsPureAndInformational(t *testing.T) {
	src := readHub(t)
	start := strings.Index(src, "func prepareManagerCheck(")
	end := strings.Index(src, "func prepareManagerUpdate(")
	if start < 0 || end < start {
		t.Fatal("check review not found")
	}
	body := src[start:end]
	if strings.Contains(body, "Apply:") || strings.Contains(body, "Apply =") || strings.Contains(body, "PrepareUpdates(") {
		t.Fatal("update check must not stage or apply")
	}
	if !strings.Contains(body, "pluginadmin.Check(") {
		t.Fatal("update check must use the pure Check service")
	}
}

// Dependency authorization is a separate typed value from acceptance, and the
// manager has no compatibility or migration controls.
func TestManagerAuthorizationAndRetiredControls(t *testing.T) {
	src := readHub(t)
	if !regexp.MustCompile(`Dependencies:\s+plan\.PolicyReferences\(\)`).MatchString(src) {
		t.Fatal("removal reviews must expose policy references as separate authorization")
	}
	for _, retired := range []string{"Official", "embedded", "alias", "migrat", "link"} {
		if strings.Contains(strings.ToLower(src), strings.ToLower(retired)) {
			t.Errorf("retired control %q present in the manager", retired)
		}
	}
	if strings.Contains(src, "SafeSource(") == false {
		t.Fatal("source text must be credential-safe")
	}
}
