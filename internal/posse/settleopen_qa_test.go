package posse

// QA pins for settleopen.go's tree lines, from the verify pass on
// ranger-base-nfgh (ranger-base-5gnb).
//
// ranger-base-nfgh's fix routed every sentence that renders a session
// tree's base through orDetached(), so a repo on a detached HEAD over a
// branch with no recorded base says "the branch it was cut from" instead of
// interpolating "". The sweep enumerated seven sites and missed this one,
// which is the SAME SENTENCE as the pass line's — filed as
// ranger-base-82d9.

import (
	"strings"
	"testing"
)

// detachedLegacyTree builds the bead's state and returns the session and its
// tree: a real session with a worktree, its posseBase config unset (a branch
// cut before posseBase was recorded, which baseOf documents as a supported
// shape), and the repo's HEAD detached so there is no branch to fall back
// on either. SessionTreeOf then answers Base == "" the way a pass reads it.
func detachedLegacyTree(t *testing.T) (*HerdrBackend, string, *SessionTree) {
	t.Helper()
	wtqaHome(t)
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	repo := wtRepo(t)
	write(t, b.App.ConfigPath, "")
	session := SessionForBead("ranger", repo, "a-1")
	if err := b.CreateSession(NewSessionOpts{Name: session, Dir: repo, Cmd: "true", Worktree: true}); err != nil {
		t.Fatal(err)
	}
	m, ok := b.readMeta(session)
	if !ok {
		t.Fatal("the session has no meta")
	}
	tr := SessionTreeOf(m)
	if tr == nil {
		t.Fatalf("the session got no tree: %+v", m)
	}
	mustGit(t, repo, "config", "--unset", baseKey(tr.Branch))
	mustGit(t, repo, "checkout", "-q", "--detach", "HEAD")
	tr = SessionTreeOf(m)
	if tr.Base != "" {
		t.Fatalf("the fixture is not the state under test: Base = %q", tr.Base)
	}
	return b, session, tr
}

// The escape. settleTreeLines is the body of the settle-open escalation —
// what the operator reads when deciding whether it is safe to kill a
// session, written into a bead, where the empty string outlives the pass.
func TestQASettleTreeLinesNamesTheBaseItCannotKnow(t *testing.T) {
	b, session, _ := detachedLegacyTree(t)
	got := (&Dispatcher{App: b.App, HB: b}).settleTreeLines(session)
	if strings.Contains(got, "merges to  at close") {
		t.Errorf("the settle-open body renders an empty base:\n%s", got)
	}
	// A positive witness, for noteTree's reason: an absence assertion is
	// also satisfied by the line not rendering at all.
	if !strings.Contains(got, "merges to "+orDetached("")+" at close") {
		t.Errorf("the settle-open body does not say the base is unknowable:\n%s", got)
	}
}

// The rest of the sweep, which DOES hold — kept so a later refactor cannot
// quietly undo it. Every one of these was measured on 2026-08-30 at 25503c1,
// when the pin above was the only red this state produced.
func TestQADetachedLegacyBaseIsSweptEverywhereElse(t *testing.T) {
	_, _, tr := detachedLegacyTree(t)
	said := orDetached("")

	// treeState and unaccountedFor guard on the empty base rather than
	// rendering it: "cannot say what is unmerged" is the honest answer and
	// nothing is refused on a count nobody could take.
	if s := treeState(tr); !strings.Contains(s, "detached") {
		t.Errorf("treeState does not name the detached repo: %q", s)
	}
	if s := unaccountedFor(tr, false); s != "" {
		t.Errorf("a tree whose base cannot be read is gated anyway: %q", s)
	}

	// MergeSessionWork's empty-base arm is what decides WHICH of mergeBack's
	// and landsweep's lines renders. Only the default arm is reachable, and
	// that is the one the fix routed through orDetached — so this is the
	// measurement standing behind leaving dispatch.go:3803/3817/3824 and
	// landsweep.go:133 interpolating t.Base raw.
	o, err := MergeSessionWork(tr)
	if err != nil {
		t.Fatalf("MergeSessionWork errored on an empty base: %v", err)
	}
	if o.Merged || o.Commits != 0 || len(o.Equivalent) != 0 {
		t.Errorf("an empty base reaches an arm other than the default: %+v", o)
	}
	if !strings.Contains(o.Reason, "detached HEAD") {
		t.Errorf("the outcome does not say why it cannot land: %q", o.Reason)
	}
	if !strings.Contains(said, "cut from") {
		t.Errorf("orDetached no longer names the branch: %q", said)
	}
}
