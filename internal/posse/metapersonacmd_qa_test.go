//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-qei5l, verifying ranger-base-k2olp's close: the guard its
// coverage argument rests on.
//
// k2olp refuses `--cmd` beside `--agent` at the argv boundary and asks the
// check NOWHERE else. The reason given — and it is the right reason — is
// that no other launch path can carry the pair meaning it: "startPlanned
// records `cmd:` only for a session with no persona (HerdrMeta.Cmd), so
// RecreateOpts hands a persona session Cmd: ''" (cmdagent.go). A census of
// every non-test NewSessionOpts construction site confirms it: the only two
// that can carry both are the recipe path (filed as ranger-base-g27p4) and
// relaunch.go's RecreateOpts, and relaunch is covered by exactly that one
// `if`.
//
// Nothing asserted it. MEASURED 2026-11-22 (this box, linux 7.2.15-1-omarchy,
// go1.27.3), before this file existed: herdrback.go's `if o.Agent == ""`
// around `meta.Cmd = o.Cmd` changed to `if true` — so a persona session's
// meta records the typed line after all — and every test matching
// Relaunch|Recreate|MetaCmd|CmdAgent|PlainSession|StartPlanned|WriteMeta was
// `ok`. A reader census says why: the only two assertions over m.Cmd are
// TestRelaunchReplaysPlainCommand, whose session is persona-LESS and whose
// claim is that the cmd IS recorded, and a kept-recipe test that compares
// m2.Cmd to m.Cmd and so reads the same value whichever it is.
//
// With the guard gone, a persona session's meta would carry a `cmd:` line,
// RecoverCommand would print `posse new … --agent … --cmd …` — a line posse
// now REFUSES — and a relaunch would discard the typed line in silence,
// which is k2olp's own defect one layer down from the flag it fixed.

import (
	"os"
	"path/filepath"
	"testing"
)

// The persona is planted here rather than through argvprompt_test.go's
// argvPersona, which lives behind `posse_arm2` — an arm-1 file reaching for
// it does not fail at the assertion, it fails `go vet` for the whole arm
// (TestQAEverySuiteArmTypeChecks caught exactly that while this pin was
// being written).
func metaPersonaPin(t *testing.T, a *App, name string) {
	t.Helper()
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\nname: " + name + "\ndescription: test\nlabels: [go]\nruntime: grok\n---\nYou are " + name + ".\n"
	if err := os.WriteFile(filepath.Join(a.AgentsDir, name+".md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Two sessions, one guard, and the control is what makes the empty read
// evidence: a persona-less session with the same typed line must still
// record it. Without that arm, a fixture that recorded no cmd for any
// session at all would pass.
func TestAPersonaSessionsMetaRecordsNoTypedCommand(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	const line = "typed-line --flag"
	metaPersonaPin(t, b.App, "ranger")

	mustCreate(t, b, NewSessionOpts{Name: "withpersona", Agent: "ranger", Cmd: line})
	if got := metaOf(t, b, "withpersona").Cmd; got != "" {
		t.Errorf("a persona session's meta recorded cmd: %q — a persona's line is RENDERED from its PID at every launch (ADR 0002 §1), so a typed one recorded here is a line relaunch would hand back in silence. This is the guard ranger-base-k2olp's coverage argument rests on (herdrback.go, startPlanned)", got)
	}

	// The control: same typed line, no persona. A plain session's --cmd
	// exists nowhere else, so it MUST be recorded — and k2olp leaves
	// `--cmd` alone when it is typed without `--agent`.
	mustCreate(t, b, NewSessionOpts{Name: "nopersona", Cmd: line})
	if got := metaOf(t, b, "nopersona").Cmd; got != line {
		t.Errorf("a persona-less session's meta recorded cmd: %q, want %q — if this arm fails the assertion above is measuring a fixture that records nothing, not the guard", got, line)
	}
}
