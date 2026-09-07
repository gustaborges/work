package present

import (
	"strings"
	"testing"
)

func testConfirmModel(spec ConfirmSpec) confirmModel {
	m := newConfirmModel(IO{}, spec)
	m.th = offTheme()
	return m
}

func TestConfirmAccept(t *testing.T) {
	spec := ConfirmSpec{Title: "Create Work", Impact: "repo: x\nbranch: y", Accept: "Create", Reject: "Cancel"}
	for _, key := range []string{"y", "Y"} {
		m := testConfirmModel(spec)
		next, _ := m.Update(press(key))
		cm := next.(confirmModel)
		if !cm.status().done || !cm.accepted || cm.state != listCompleted {
			t.Fatalf("%s: done=%v accepted=%v state=%d", key, cm.status().done, cm.accepted, cm.state)
		}
		if cm.status().receipt != "✔ Create Work confirmed\n" {
			t.Errorf("%s: accept receipt = %q", key, cm.status().receipt)
		}
		if cm.status().cancelled {
			t.Errorf("%s: accept marked cancelled", key)
		}
		if cm.status().answer != true {
			t.Errorf("%s: answer = %v, want true", key, cm.status().answer)
		}
	}

	// Enter on the (moved) Accept focus also accepts.
	m := testConfirmModel(spec)
	next, _ := m.Update(press("left")) // toggle focus from Reject to Accept
	next, _ = next.Update(press("enter"))
	if !next.(confirmModel).accepted {
		t.Errorf("enter on Accept focus did not accept")
	}
}

func TestConfirmReject(t *testing.T) {
	spec := ConfirmSpec{Title: "Create Work", Accept: "Create", Reject: "Cancel"}
	for _, key := range []string{"n", "N"} {
		m := testConfirmModel(spec)
		next, _ := m.Update(press(key))
		cm := next.(confirmModel)
		if !cm.status().done || cm.accepted {
			t.Fatalf("%s: done=%v accepted=%v", key, cm.status().done, cm.accepted)
		}
		if cm.status().receipt != "✘ Cancel\n" {
			t.Errorf("%s: reject receipt = %q", key, cm.status().receipt)
		}
		if cm.status().cancelled {
			t.Errorf("%s: reject is not a cancellation (returns (false,nil))", key)
		}
	}
}

func TestConfirmCancel(t *testing.T) {
	for _, key := range []string{"esc", "ctrl+c"} {
		m := testConfirmModel(ConfirmSpec{Title: "T", Accept: "Go", Reject: "Stop"})
		next, _ := m.Update(press(key))
		cm := next.(confirmModel)
		if !cm.status().cancelled {
			t.Fatalf("%s: cancelled=%v", key, cm.status().cancelled)
		}
		// A cancelled confirmation leaves no receipt; the border prints the notice.
		if cm.status().receipt != "" {
			t.Errorf("%s: cancel receipt = %q, want empty", key, cm.status().receipt)
		}
	}
}

func TestConfirmImpactShownWhileChoosing(t *testing.T) {
	m := testConfirmModel(ConfirmSpec{Title: "Archive", Impact: "2 worktrees will be destroyed", Accept: "Archive", Reject: "Keep"})
	v := stepBody(m)
	if !strings.Contains(v, "2 worktrees will be destroyed") {
		t.Errorf("impact not shown:\n%s", v)
	}
	if !strings.Contains(v, "Archive") || !strings.Contains(v, "Keep") {
		t.Errorf("choice labels not shown:\n%s", v)
	}
}
