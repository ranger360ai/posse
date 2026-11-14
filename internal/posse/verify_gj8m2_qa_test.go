//go:build !posse_arm2 && !posse_arm3

package posse

// QA pin from the verify of ranger-base-y4cm6 (verify bead ranger-base-gj8m2).
//
// WHAT WAS UNPINNED. y4cm6 gave the credential read masks their own
// wall-watch kind, and the reason it is a kind of its own rather than Deny's
// is stated in the bead in as many words: the D2 bead must tell its reader
// that a CREDENTIAL name came off, so that credential is judged for ROTATION
// and not only for the writes after the detach time. wallWatchDescription
// does that with its ROTATE: line, gated on wallWatchHasKind. The bead's own
// Pins clause named only the render pin, and the render pin's subject is the
// MOUNT LIST — so the line that is the whole point of the kind had no reader
// anywhere in the tree.
//
// MEASURED 2026-11-13 at main 2bb926f3: the string ROTATE appeared exactly
// once in the tracked tree, at its producer. Two mutants of the gate — never
// print it, and print it on every detach of every kind — both left
// `go test -run 'TestWallWatch|TestUpgrade|TestFakePosse'` GREEN over 77
// tests. This file is the reader those mutants should have met.
//
// BOTH DIRECTIONS, and the negative one is not decoration. A ROTATE: line on
// a detach that lost no read mask is the same loss as no line at all: it asks
// for a credential rotation that nothing here says is warranted, on every
// bead, which is how a reviewer learns to skip the line. So the absence arm
// carries its own positive control — the same description must still carry
// the REVIEW: line every detach gets — because an assertion that a string is
// missing from a description that was never built is true of nothing.
//
// Through fileWallWatchDetach rather than wallWatchDescription directly: the
// bead's claim is about what the D2 BEAD says, and the filing path is what
// puts the description there.

import (
	"strings"
	"testing"
	"time"
)

func TestWallWatchBeadCallsForRotationExactlyWhenAReadMaskCameOff(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	cred := "/h/sibling/credentials.json"
	sshDir := "/h/.ssh"
	settings := "/h/cfg/settings.json"

	// filedBead runs one detach into a store of its own — a fresh store per
	// arm, because D2's ten-minute dedupe would turn the second detach of a
	// path into a comment and there would be no description to read.
	filedBead := func(t *testing.T, ms ...WallWatchMount) BdIssue {
		t.Helper()
		st := &fakeWallWatchStore{now: func() time.Time { return at }}
		d := wallWatchDetach{Seat: "a", Dir: "/w/a", BwrapPid: 10, SeatPid: 11, At: at, Missing: ms, Killed: []int{10, 11}}
		filed, _, err := fileWallWatchDetach(st, d, at, "")
		if err != nil || filed == "" || len(st.issues) != 1 {
			t.Fatalf("fileWallWatchDetach over %v: filed=%q err=%v beads=%d", ms, filed, err, len(st.issues))
		}
		return st.issues[0]
	}

	const rotate = "ROTATE:"
	const review = "REVIEW:"

	// 1. A read mask came off: the bead asks for the rotation, and it names
	//    the path whose credential has to be judged.
	for _, m := range []WallWatchMount{{WallWatchReadMask, cred}, {WallWatchReadMask, sshDir}} {
		is := filedBead(t, m)
		if !strings.Contains(is.Description, rotate) {
			t.Errorf("a %s wall at %s came off and the bead does not ask for a rotation — the kind's whole reason over %s is that a read mask handed the seat a CREDENTIAL in cleartext (ranger-base-y4cm6):\n%s",
				m.Kind, m.Path, WallWatchDeny, is.Description)
		}
		// The path WITH ITS KIND, which is the walls-that-came-off block's
		// own shape and is not the dedupe marker's: the marker's parenthesis
		// holds the design bead's id, so a bare Contains of the path is
		// satisfied by the marker at the end of the description and would
		// pass over a description that listed no walls at all (MEASURED —
		// that mutant survived the first cut of this pin). ROTATE: says
		// "judge every credential under those paths", so the paths and
		// which of them is the read mask are what make it actionable.
		if want := m.Path + " (" + m.Kind + ")"; !strings.Contains(is.Description, want) {
			t.Errorf("the bead asks for a rotation and never says %q, so the reviewer has no path to judge and no way to tell which wall was the read mask:\n%s", want, is.Description)
		}
	}

	// 2. No read mask came off: no rotation is asked for. The REVIEW: line
	//    is the positive control — it proves this description was built, so
	//    the missing ROTATE: is a decision and not an empty string.
	for _, m := range []WallWatchMount{{WallWatchDeny, settings}, {WallWatchPrivate, "/h/cfg/shell-snapshots"}, {WallWatchMasked, "/h/cfg/policy-cache.json"}, {WallWatchHide, "/h/run"}} {
		is := filedBead(t, m)
		if !strings.Contains(is.Description, review) {
			t.Fatalf("control: a %s detach filed a bead carrying no %s line, so this arm's absence test measures nothing:\n%s", m.Kind, review, is.Description)
		}
		if strings.Contains(is.Description, rotate) {
			t.Errorf("a %s wall at %s came off and the bead asks for a CREDENTIAL rotation anyway — a rotation asked for on every detach is one nobody reads:\n%s",
				m.Kind, m.Path, is.Description)
		}
	}

	// 3. Mixed. wallWatchHasKind is an ANY and must stay one: a detach that
	//    took a read mask AND an ordinary wall still asks for the rotation,
	//    whichever order the render listed them in.
	for _, ms := range [][]WallWatchMount{
		{{WallWatchDeny, settings}, {WallWatchReadMask, cred}},
		{{WallWatchReadMask, cred}, {WallWatchDeny, settings}},
	} {
		is := filedBead(t, ms...)
		if !strings.Contains(is.Description, rotate) {
			t.Errorf("a detach carrying both %s and %s walls does not ask for the rotation (first kind listed: %s):\n%s",
				WallWatchReadMask, WallWatchDeny, ms[0].Kind, is.Description)
		}
	}
}
