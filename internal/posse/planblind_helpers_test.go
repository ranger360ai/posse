package posse

// Helpers lifted out of planblind_test.go so every suite arm compiles them
// (ranger-base-qp1hm). A file with a build tag is absent from the arms it
// does not name, and these declarations have readers in all of them.

import (
	"strings"
	"testing"
	"time"
)

// blindT is the fixed instant every test's clock starts from.
var blindT = time.Date(2026, 8, 19, 20, 53, 0, 0, time.UTC)

// deadURL is an address nothing is listening on, so Read fails the way it
// fails in the wild: a real transport error, real "usage endpoint
// unreachable" text, no faked error strings.
func deadURL(t *testing.T) string {
	t.Helper()
	return "http://127.0.0.1:1"
}

// blindRig: a dispatcher with a ready bead, a clock the test drives, and a
// plan reader that can be blinded and restored between passes. Returns the
// dispatcher, its stderr, and the two knobs (clock, blind switch).
type blindRig struct {
	d     *Dispatcher
	errb  *strings.Builder
	fake  string
	repo  string
	ps    *planServer // the working endpoint, for its request count
	live  string      // its URL
	dead  string      // the one that refuses connections
	clock time.Time
}

func newBlindRig(t *testing.T, cfg string) *blindRig {
	t.Helper()
	b, fake := newTestBackend(t)
	ps := newPlanServer(t, 12, 40) // well under any threshold: only the blind window gates here
	d, errb := planDispatcher(t, b, ps)
	writePersona(t, b.App, "ranger", "[go]")
	repo := planRepo(t, `[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","title":"t","status":"closed"}]`)
	planConfig(t, b.App, repo, cfg)
	idleClaude(t, fake)

	r := &blindRig{d: d, errb: errb, fake: fake, repo: repo, ps: ps, live: ps.URL, dead: deadURL(t), clock: blindT}
	d.Now = func() time.Time { return r.clock }
	d.blindSince = blindT
	return r
}

func (r *blindRig) blind() { planReaderOf(r.d).URL = r.dead }

func (r *blindRig) sighted() { planReaderOf(r.d).URL = r.live }

func (r *blindRig) at(d time.Duration) { r.clock = blindT.Add(d) }

func (r *blindRig) out() string { return dispatcherOut(r.d) }

func (r *blindRig) err() string { return r.errb.String() }

func (r *blindRig) run(t *testing.T) int { t.Helper(); n, _ := r.d.Run("", "", 0); return n }

const guardOn = "plan_guard_5h: 70\nplan_guard_7d: 85"
