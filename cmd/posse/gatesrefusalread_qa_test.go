package main

// QA, ranger-base-jielm (finding 2 of the ranger-base-q2peg verify) — the
// host's own READ of the canonical refusals leaf, EXECUTED through the built
// binary, because the finding was filed on a reading of the source and not
// on a run:
//
//	plant a symlink at <gates>/<persona>/refusals.log pointing at any
//	readable file, run the `posse gates` path that reaches it, and the
//	link target's last ten lines are printed.
//
// That leaf is the one ranger-base-legou guarded at its three WRITES —
// RefusalsLog, appendCanonicalRefusal and the fold's canonical append —
// plus the fold's spool READ, which was brought in because it copies a link
// target's bytes somewhere. This one copies them into the operator's
// terminal: the same leaf, the same planter outside every wall, the same
// following open, and a much narrower prize, which is why it survived the
// census (that census's subject was the writers, and it was true as
// stated).
//
// Driven through the binary beside this command's other `gates` pins
// (gatesreportwrites_qa_test.go, whose grwHome/grwRun this reuses), because
// the bytes a planter would win are only readable as the command's output.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// grrLeaf is the persona's canonical trail under an RHQ_HOME grwHome made.
func grrLeaf(rhq string) string {
	return filepath.Join(rhq, "state", "gates", "builder", "refusals.log")
}

// The acceptance pin: the planter's file is NOT printed, and the report says
// so out loud instead of calling the trail empty.
//
// The secret is the operator's ssh key by SHAPE and not by path — nothing is
// read from the real home — so that what the arm asserts about is a file the
// planter picked and this command has no business opening.
func TestQAGatesReportReadsTheRefusalsLeafThroughNoLink(t *testing.T) {
	bin := buildRhq(t)
	rhq := grwHome(t)
	launchHome := t.TempDir()

	// One run first, so the wall and the gates dir are on disk and the
	// second run takes the READ path rather than the render (ranger-base-6s5mo).
	if _, code := grwRun(t, bin, rhq, launchHome); code != 0 {
		t.Fatalf("the first report exited %d, so this fixture has no wall to report on", code)
	}

	const secret = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI-NOT-YOURS-TO-PRINT\n"
	elsewhere := filepath.Join(t.TempDir(), "id_ed25519.pub")
	if err := os.WriteFile(elsewhere, []byte(secret), 0o600); err != nil {
		t.Fatal(err)
	}

	// THE CONTROL FIRST, and it is not optional: a report that prints no
	// refusals at all would pass the arm below over nothing. So the same
	// bytes go in as a REGULAR file and must be printed.
	leaf := grrLeaf(rhq)
	if err := os.WriteFile(leaf, []byte(secret), 0o644); err != nil {
		t.Fatal(err)
	}
	ctl, _ := grwRun(t, bin, rhq, launchHome)
	if !strings.Contains(ctl, strings.TrimSpace(secret)) {
		t.Fatalf("the report does not print a REGULAR refusals.log's lines, so the arm below measures nothing:\n%s", ctl)
	}

	// Now the plant, at the leaf.
	if err := os.Remove(leaf); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, leaf); err != nil {
		t.Fatal(err)
	}
	outp, _ := grwRun(t, bin, rhq, launchHome)
	if strings.Contains(outp, strings.TrimSpace(secret)) {
		t.Errorf("`posse gates` printed the bytes of %s, which a planter named at the refusals leaf — the host's read follows a link (ranger-base-jielm):\n%s", elsewhere, outp)
	}
	// And it does not report a tampered trail as a clean one: this
	// command's exit and output are read as "cleared to dispatch", so a
	// refused leaf that printed "empty" would be the lie the unshimmed-gate
	// arm of the same report exists to refuse.
	if strings.Contains(outp, "refusals.log: empty") {
		t.Errorf("`posse gates` called a PLANTED refusals leaf empty; it has to say it could not read it:\n%s", outp)
	}
	if !strings.Contains(outp, "refusals.log: ⚠️  NOT READ") {
		t.Errorf("`posse gates` says nothing about a refusals leaf it refused to read:\n%s", outp)
	}
}

// The other half of the leaf's guard, at the same call site: a DANGLING link
// is refused too, and is the shape an absent trail is most easily confused
// with — `os.ReadFile` of one answers an error os.IsNotExist recognises, so
// a caller that only asks "is it there" reports it as empty and the plant
// survives to be followed by whatever creates the leaf next.
func TestQAGatesReportDoesNotCallADanglingPlantAnEmptyTrail(t *testing.T) {
	bin := buildRhq(t)
	rhq := grwHome(t)
	launchHome := t.TempDir()
	if _, code := grwRun(t, bin, rhq, launchHome); code != 0 {
		t.Fatalf("the first report exited %d, so this fixture has no wall to report on", code)
	}
	leaf := grrLeaf(rhq)
	os.Remove(leaf)
	target := filepath.Join(t.TempDir(), "not-here-yet")
	if err := os.Symlink(target, leaf); err != nil {
		t.Fatal(err)
	}
	outp, _ := grwRun(t, bin, rhq, launchHome)
	if strings.Contains(outp, "refusals.log: empty") {
		t.Errorf("`posse gates` called a DANGLING planted leaf empty, which is what an absent trail prints:\n%s", outp)
	}
	if !strings.Contains(outp, "refusals.log: ⚠️  NOT READ") {
		t.Errorf("`posse gates` says nothing about a dangling planted leaf:\n%s", outp)
	}
	// The report still describes the wall it was asked about: a refused
	// read is not a reason to die before the rest of the surface.
	if !strings.Contains(outp, "wall-watch:") {
		t.Errorf("the report stopped at the refused leaf instead of finishing:\n%s", outp)
	}
}
