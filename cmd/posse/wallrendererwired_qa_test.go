package main

// QA pin for ranger-base-vso72 — `posse gates <persona>` names the binary
// whose renders its parity matrix is describing.
//
// WHY THIS COMMAND. Every row of that matrix is a claim about a wall THIS
// binary rendered moments earlier, at the top of the same branch (the gates
// dir is re-rendered as a side effect of being asked to report it —
// gatesreadonly_qa_test.go is the pin on that). So a matrix read off a binary
// that predates the gate fix it is being checked for is a wall of green about
// a wall nobody has: ranger-base-mmhhc's twelve caged launches rendered a
// superseded credential deny while every surface agreed the deny was in
// force.
//
// Driven through the built binary, like the other pins on this command: the
// call lives in the `gates` case in main.go, and a unit test of the reading
// cannot tell whether anything calls it.

import (
	"strings"
	"testing"
)

const wrwPrefix = "wall renderer (gates) · "

func TestQAGatesNamesTheBinaryWhoseRendersItReports(t *testing.T) {
	bin := buildRhq(t)
	out, code := roGatesRun(t, bin, roGatesHome(t))
	if code != 0 {
		t.Fatalf("posse gates builder exited %d:\n%s", code, out)
	}
	at := strings.Index(out, wrwPrefix)
	if at < 0 {
		t.Fatalf("`posse gates <persona>` does not name the binary whose renders the matrix below describes — the control fires nowhere:\n%s", out)
	}
	// Above the matrix, because the matrix is what it qualifies. A reading
	// printed after several screens of green rows is a reading nobody reaches.
	matrix := strings.Index(out, "parity (ADR 0002 §4")
	if matrix < 0 {
		t.Fatalf("no parity matrix in the output, so this pin is measuring the wrong command:\n%s", out)
	}
	if at > matrix {
		t.Fatalf("the renderer line came after the matrix it qualifies:\n%s", out)
	}
	// It moves no exit code, whatever it found: this is a reading, and
	// installing over a binary that is dispatching a live fleet is the
	// operator's (guardrail 3). The rig's binary is built without the
	// Makefile's stamp, so the reading here is the UNKNOWN arm — which is the
	// one that must not fail a command either.
	line := out[at:]
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	if strings.TrimSpace(strings.TrimPrefix(line, wrwPrefix)) == "" {
		t.Fatalf("the renderer line says nothing after its prefix: %q", line)
	}
}
