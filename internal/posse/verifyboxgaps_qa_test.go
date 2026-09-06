package posse

// Three G10 claims that verifybox_qa_test.go states and does not measure
// (found verifying ranger-base-jj2ax on ranger-base-lvzm7; each line below
// was chosen because a mutant of the code it names survived the whole
// verify-box pin set).
//
// They are separate from that file rather than folded into it because each
// one is an ARM ADDED to a pin that already exists and already passes: the
// pin is not wrong, it is narrower than the sentence beside it, and keeping
// the added arms together is what makes that difference readable a year from
// now.

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// ─── the set is PARTIAL over a verdict that cannot be read ───────────────────

// verifybox.go raises no G10 row when the store is unreadable, and govern.go
// reports the read failure to ShopCheck's error return instead — "a store
// that cannot be READ is not a store that says no", so `posse status` says
// UNKNOWN and exits non-zero rather than drawing a clean board.
//
// TestAMalformedVerdictIsPartialAndNeverGreen holds the first half of that
// (no row, and the status LINE says so). This holds the second, which is the
// half that decides the exit status: MEASURED — deleting the
// `failed = append(...)` arm in ShopCheck leaves that whole file green, and a
// malformed verdict then reaches the surface as a governance set with nothing
// in it.
func TestAnUnreadableVerdictReachesTheSetAsAnError(t *testing.T) {
	b, _ := newTestBackend(t)
	appendConfig(t, b.App, "verify_box_max_age: 12h\n")
	// rc: 0 over a check that answered `finding` — the record contradicts
	// itself, which is what a half-written or hand-edited file looks like.
	write(t, VerifyBoxStatePath(b.App), fmt.Sprintf("at: %s\nrc: 0\nchecks:\n  verify-codex-pin: finding\n",
		govNow.Add(-3*time.Hour).UTC().Format(time.RFC3339)))

	set, failed := ShopCheck(govIn(t, b))
	var named bool
	for _, err := range failed {
		if strings.Contains(err.Error(), "verify-box") {
			named = true
		}
	}
	if !named {
		t.Errorf("an unreadable verify-box verdict left the set CLEAR (%d error(s): %v) — a store that cannot be read must make the set partial, or status exits 0 over a box nothing can vouch for", len(failed), failed)
	}
	if row := find(set, "G10"); row != nil {
		t.Errorf("an unreadable store was also diagnosed: %+v", row)
	}

	// The control, and it is the arm that keeps this from passing on a
	// check that always reports: the SAME instance with a record whose two
	// halves agree names no verify-box error at all.
	write(t, VerifyBoxStatePath(b.App), fmt.Sprintf("at: %s\nrc: 1\nchecks:\n  verify-codex-pin: finding\n",
		govNow.Add(-3*time.Hour).UTC().Format(time.RFC3339)))
	if _, ok := ShopCheck(govIn(t, b)); len(ok) > 0 {
		for _, err := range ok {
			if strings.Contains(err.Error(), "verify-box") {
				t.Errorf("a readable verdict is reported as unreadable: %v", err)
			}
		}
	}
}

// ─── either key arms the control ─────────────────────────────────────────────

// VerifyBoxConfigured says "either key is enough", and the reason is the
// whole ARMED design: an instance whose schedule was installed and has never
// fired has no state file, so the config is the only thing that can say this
// box expects the control to run.
//
// Every existing pin arms with `verify_box_max_age:`. MEASURED — dropping the
// `verify_box_accepted:` half of that disjunction leaves the verify-box pin
// set green, so the sentence is stated and not held. An instance that named
// only its accepted risks would then go silent on the one reading the row
// exists for.
func TestEitherConfigKeyAloneArmsTheControl(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, config string }{
		{"the freshness budget alone", "verify_box_max_age: 26h\n"},
		{"the accepted-risk list alone", "verify_box_accepted:\n  verify-codex-pin: a-bead-id\n"},
	} {
		a := NewAppAt(t.TempDir())
		write(t, a.ConfigPath, tc.config)
		r := a.VerifyBoxFreshness(vbNow(), os.Stderr)
		if !r.Armed || r.Ran {
			t.Errorf("%s: armed=%v ran=%v — a config asking for this control with no verdict on disk is armed and has not run", tc.name, r.Armed, r.Ran)
			continue
		}
		if row := vbRow(r, "verify-box-stale"); row == nil {
			t.Errorf("%s: an armed instance with no verdict raised nothing: %v", tc.name, vbKeys(r))
		}
	}

	// The control the pair rests on: a config that names NEITHER key, with
	// no verdict on disk, stays silent. Installing posse arms no schedule.
	b := NewAppAt(t.TempDir())
	write(t, b.ConfigPath, "attn_question_age: 4h\n")
	if q := b.VerifyBoxFreshness(vbNow(), os.Stderr); q.Armed || len(q.GovRows()) > 0 {
		t.Errorf("an instance that never asked for this control reports armed=%v rows=%v", q.Armed, vbKeys(q))
	}
}

// ─── the freshness budget's own edge ─────────────────────────────────────────

// `verify_box_max_age:` is the one number this row turns on, and its pins
// stand 1h either side of it (25h and 27h against a 26h default). MEASURED —
// `r.Age > r.MaxAge` mutated to `>=` survives the whole verify-box pin set,
// so the budget is measured as a region and not as a threshold.
//
// The claim the code makes is that the budget is a MAXIMUM: a verdict exactly
// that old is still inside it, and anything past it is not. Both arms, at the
// smallest step the clock has, so the boundary cannot move in either
// direction without a red.
func TestTheFreshnessBudgetIsAMaximumAtItsOwnEdge(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		age   time.Duration
		stale bool
	}{
		{"exactly the budget is inside it", 26 * time.Hour, false},
		{"one nanosecond past it is not", 26*time.Hour + 1, true},
	} {
		a := vbRig(t, "verify_box_max_age: 26h\n", vbNow().Add(-tc.age), 0, "verify-grok-pin: ok")
		r := a.VerifyBoxFreshness(vbNow(), os.Stderr)
		if r.Stale != tc.stale {
			t.Errorf("%s: age %s under a 26h max reads stale=%v, want %v", tc.name, r.Age, r.Stale, tc.stale)
		}
		if got := len(vbKeys(r)) > 0; got != tc.stale {
			t.Errorf("%s: rows %v, want a stale row = %v", tc.name, vbKeys(r), tc.stale)
		}
	}
}
