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
		next, cmd := m.Update(press(key))
		cm := next.(confirmModel)
		if !isQuit(cmd) || !cm.accepted || cm.state != listCompleted {
			t.Fatalf("%s: quit=%v accepted=%v state=%d", key, isQuit(cmd), cm.accepted, cm.state)
		}
		if cm.View().Content != "✔ Create Work confirmed\n" {
			t.Errorf("%s: accept view = %q", key, cm.View().Content)
		}
		if cm.outcome().cancelled {
			t.Errorf("%s: accept marked cancelled", key)
		}
	}

	// Enter on the (moved) Accept focus also accepts.
	m := testConfirmModel(spec)
	next, _ := m.Update(press("left")) // toggle focus from Reject to Accept
	next, cmd := next.Update(press("enter"))
	if !isQuit(cmd) || !next.(confirmModel).accepted {
		t.Errorf("enter on Accept focus did not accept")
	}
}

func TestConfirmReject(t *testing.T) {
	spec := ConfirmSpec{Title: "Create Work", Accept: "Create", Reject: "Cancel"}
	for _, key := range []string{"n", "N"} {
		m := testConfirmModel(spec)
		next, cmd := m.Update(press(key))
		cm := next.(confirmModel)
		if !isQuit(cmd) || cm.accepted {
			t.Fatalf("%s: quit=%v accepted=%v", key, isQuit(cmd), cm.accepted)
		}
		if cm.View().Content != "✘ Cancel\n" {
			t.Errorf("%s: reject view = %q", key, cm.View().Content)
		}
		if cm.outcome().cancelled {
			t.Errorf("%s: reject is not a cancellation (returns (false,nil))", key)
		}
	}
}

func TestConfirmCancel(t *testing.T) {
	for _, key := range []string{"esc", "ctrl+c"} {
		m := testConfirmModel(ConfirmSpec{Title: "T", Accept: "Go", Reject: "Stop"})
		next, cmd := m.Update(press(key))
		cm := next.(confirmModel)
		if !isQuit(cmd) || !cm.outcome().cancelled {
			t.Fatalf("%s: quit=%v cancelled=%v", key, isQuit(cmd), cm.outcome().cancelled)
		}
		if cm.View().Content != "✘ Operation cancelled\n" {
			t.Errorf("%s: cancel view = %q", key, cm.View().Content)
		}
	}
}

func TestConfirmImpactShownWhileChoosing(t *testing.T) {
	m := testConfirmModel(ConfirmSpec{Title: "Archive", Impact: "2 worktrees will be destroyed", Accept: "Archive", Reject: "Keep"})
	v := m.View().Content
	if !strings.Contains(v, "2 worktrees will be destroyed") {
		t.Errorf("impact not shown:\n%s", v)
	}
	if !strings.Contains(v, "Archive") || !strings.Contains(v, "Keep") {
		t.Errorf("choice labels not shown:\n%s", v)
	}
}
