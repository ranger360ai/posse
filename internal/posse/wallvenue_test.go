//go:build !posse_arm2 && !posse_arm3

package posse

// ADR 0096 D3's nest helper, carrying ADR 0097 D4's venue contract.
//
// WHY THIS FILE IS TAGGED, and it is the D4 bullet that is enforced by the
// build rather than by a test: `//go:build !posse_arm2 && !posse_arm3` is the
// arm 1 spelling (internal/treepins/armtags_qa_test.go's armExpr), so a
// member written into arm 2 or arm 3 DOES NOT COMPILE. That is the point.
// `scripts/posse-wall-check` runs one untagged `go test -run
// '^Test(QA)?Live(Bwrap|Wall)' ./internal/posse` (ADR 0097 D3), and an
// untagged `-run` over a test behind a tag selects nothing and prints `ok` —
// the same silent hole the seed-check door hit twice (Makefile, seed-check's
// comment: two pins MOVED ARM to get in). A compile error at the moment the
// file is written is the only report that cannot be mistaken for a pass.
//
// The helper lived in wall_helpers_test.go, untagged, when ADR 0096 D3
// shipped it under ranger-base-pvkhn. It moved here under ranger-base-q749c
// for the paragraph above; the rest of that file is untagged because the
// helpers in it are read by tests in all three arms.

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// ─── ADR 0096 D3: a live pin that nests a bwrap abstains loudly ─────────────

// bwrapNestSkipPrefix is the ONE sentence D3 mandates, as a const so every
// pin prints the same words and a reader greps one string. The reason is
// named in it on purpose: a skip that does not say why reads as coverage
// (ADR 0096, Alternatives rejected).
const bwrapNestSkipPrefix = "NOTHING MEASURED: this process is inside a wall that forbids nesting (ADR 0096 D1): "

var (
	bwrapNestOnce sync.Once
	bwrapNestWhy  string
)

// liveBwrapNestRefusal asks, ONCE PER PROCESS, whether a bwrap starts from
// here: "" when one did, and bwrap's own stderr when it did not.
//
// SEPARATE FROM liveBwrapApplyRefusal ON PURPOSE (ADR 0096 D3), and that is
// the whole point of this helper rather than a convenience. That function
// answers AvailableCages' question — "does this HOST provide the seatbelt
// tier" — and "" there means posse may announce the wall. Here "" means "a
// nested namespace was MEASURED from this process". The two share a probe
// today only because one exec happens to answer both; they must never share a
// meaning, because a host that provides the tier perfectly well still refuses
// a nested bwrap to every process already inside a seat. D17's render leaves
// `--proc` out, so inside a seat `/proc` is the host's procfs under the
// read-only base, a nested user namespace cannot write its own `uid_map`, and
// every nested bwrap fails before it reaches a mount (ADR 0096's table, arm
// A). Once the Landlock layer of ranger-base-6rndn lands the kernel refuses
// the mount instead, and the two questions answer differently for good. A pin
// printing the availability sentence would then tell its reader this box has
// no bwrap, which is false.
//
// ITS OWN sync.Once and not that function's, for the re-arming reason
// liveBwrapApplyRefusal states: by the time any pin asks, whatever called
// AvailableCages first in this binary has already cached the host answer, and
// a sync.Once cannot be re-armed (ranger-base-npqem's lesson, same shape).
//
// MEASURED 2026-11-14, D1 host, Linux 7.2.15, bubblewrap 0.13.1: uncaged this
// returns "", and from a process inside D17's base plus `--unshare-user` it
// returns `bwrap: setting up uid map: Read-only file system`.
func liveBwrapNestRefusal() string {
	bwrapNestOnce.Do(func() { bwrapNestWhy = bwrapNestProbe() })
	return bwrapNestWhy
}

// bwrapNestProbe is that ask, unmemoized.
//
// bwrapApplyProbeArgv, so D17's base mount keeps ONE spelling in this
// package: `--ro-bind / /` is the mount every render of D17 begins with, so a
// process that cannot make THAT namespace cannot run any arm of any live pin
// here, whatever else the pin's own argv asks for.
//
// BARE `bwrap` AND NOT protocolCmd's gates-free resolution, which is the one
// place this differs from bwrapApplyProbe rather than twinning it. The
// question there is posse's — "which file would a launch exec" (ADR 0077 D3,
// because that exec IS one of posse's own children). The question here is
// each pin's own: "will the exec I am about to make start". Every live pin in
// this package types `exec.Command("bwrap", …)` on the ambient PATH and gates
// on `exec.LookPath("bwrap")`, so this probe has to resolve the same way or a
// green gate could precede a red exec. A shim standing in for `bwrap` makes
// this probe refuse, and refusing is the right direction: the pin's own exec
// would have met that same shim.
//
// EVERY FAILURE IS AN ABSTENTION, as at bwrapApplyProbe and for the opposite
// reason to sandboxApplyProbe's: a missing /bin/true or a bwrap too old for
// these flags must not be turned into a measurement nobody took. "Could not
// ask" is not "a namespace was made".
func bwrapNestProbe() string {
	out, err := exec.Command("bwrap", bwrapApplyProbeArgv...).CombinedOutput()
	if err == nil {
		return ""
	}
	if s := strings.TrimSpace(string(out)); s != "" {
		return s
	}
	return err.Error()
}

// ─── ADR 0097 D4: the venue contract ───────────────────────────────────────

// wallVenueMarker is the MEMBERSHIP MARKER, verbatim as ADR 0097 D4 writes
// it. `scripts/posse-wall-check` reads this line out of `go test -json` and
// counts one member per test that printed it (D3: "Membership is read from
// the run and never inferred from the name"). Every byte of it is part of the
// contract — the script greps the line, so a reworded marker is a venue run
// that measures nothing and answers exit 3, which is the shape D5 files as
// "wall check silent" rather than as a red.
//
// A CONST AND NOT A VAR, here and for every other name in this block, so no
// parallel reader of it needs a cmd/testparallel clearance: filter 2 taints a
// test naming a WRITTEN package-level var, and a `var x = regexp.MustCompile`
// is written by its own initializer. The regexp is therefore a func
// (wallVenueMemberRe), the way instancePathSpellings is one for the same
// reason. It is compiled once per gated pin and there are 32 of them.
const wallVenueMarker = "wall venue: measuring (ADR 0097)"

// wallVenueEnvVar is the variable ADR 0097 D1 adds to the venue run's
// environment allowlist, and the ONLY thing that tells a test process it is
// inside the venue rather than inside a seat. The two walls are
// indistinguishable from in here on every other signal — both are a bwrap
// under `--ro-bind / /` — which is why D1 makes the marker an environment
// variable the renderer adds rather than something the process can probe.
//
// THE SPELLING IS SHARED WITH NOTHING YET. D1's venue is rendered by
// `boxLandedCage` through a flag, and that work is its own bead; until it
// lands, this const is the one place the name is written in this tree. The
// bead that teaches boxLandedCage the flag must read it from here or lift it
// into shipped code — a second literal is the drift that makes a venue run
// abstain on all 32 members while reporting exit 3, which D5's watchdog
// reports as silence and not as a wall regression.
const wallVenueEnvVar = "POSSE_WALL_VENUE"

// wallVenueMemberPattern is ADR 0097 D3's `-run` filter, which is also D4's
// rule about a member's NAME. One literal for both, because the drift D4
// exists to prevent is precisely the two coming apart: the script selects the
// run with this filter, so a member outside it is a pin the venue never runs
// and nobody reds, and a test inside it that is not a member is counted by
// nothing (it prints no marker) and costs only its own wall time.
//
// ^ AND NOT A BARE PREFIX TEST: `-run` anchors each path element, so the
// filter the script types matches at the start of the top-level name and this
// must read the same way.
const wallVenueMemberPattern = `^Test(QA)?Live(Bwrap|Wall)`

// wallVenueMemberRe compiles that pattern. A func, not a var — see
// wallVenueMarker's second paragraph.
func wallVenueMemberRe() *regexp.Regexp { return regexp.MustCompile(wallVenueMemberPattern) }

// wallVenueTopLevel is the top-level test name inside a `t.Name()`, which for
// a subtest is `TestFoo/sub/sub`. D4's rule is about the name the `-run`
// filter selects and the name `go test -json` reports a member under, and
// both of those are the top-level one — so a member that calls the gate from
// inside a `t.Run` must be judged by its parent's name and not by the subtest
// label, which no filter ever sees.
func wallVenueTopLevel(name string) string {
	if i := strings.IndexByte(name, '/'); i >= 0 {
		return name[:i]
	}
	return name
}

// wallVenueActive answers whether this process is inside ADR 0097 D1's venue.
//
// EXACTLY `1`, and ANY OTHER SET VALUE IS A REFUSAL rather than a quiet no.
// D1 writes `POSSE_WALL_VENUE=1` and nothing else, so a value this function
// does not recognise can only be a hand-typed or mis-rendered venue — and the
// cost of reading it as "not the venue" is that all 32 members abstain, the
// body answers exit 3, and D5's watchdog files "wall check silent at <repo>"
// instead of the wall regression the run was there to find. An abstention is
// never a pass (D5), and a venue that cannot be recognised is the one place
// that rule can be lost silently, so the helper says so in the member's own
// failure instead.
//
// Unset is the ordinary case and the only quiet one: every seat and every
// operator `make test` runs with no such variable, and D4 says behaviour
// there is exactly ADR 0096 D3's.
func wallVenueActive(t *testing.T) bool {
	t.Helper()
	switch v := os.Getenv(wallVenueEnvVar); v {
	case "":
		return false
	case "1":
		return true
	default:
		t.Fatalf("%s is set to %q, and ADR 0097 D1 renders the venue with exactly %s=1. Read as 'not the venue' this would abstain every member of the wall check, which answers exit 3 and reaches a reader as D5's \"wall check silent\" rather than as a wall regression — so it is refused here instead. Set it to 1, or unset it.",
			wallVenueEnvVar, v, wallVenueEnvVar)
		return false
	}
}

// nestAction is what the gate does to its caller: the three answers ADR 0097
// D4 allows, and nothing in between.
type nestAction int

const (
	nestMeasure nestAction = iota // log the marker and let the pin run
	nestAbstain                   // t.Skip with ADR 0096 D3's sentence
	nestFail                      // t.Fatal: the tree did this, not the wall
)

// bwrapNestDecision is the gate's whole rule, as a function of three facts
// and nothing else: the caller's `t.Name()`, whether a bwrap starts from this
// process (liveBwrapNestRefusal), and whether this process is the venue. It
// returns the action and the exact words.
//
// PURE, AND THAT IS WHY IT IS SPLIT OUT. The gate itself can only be
// exercised by being a member of the class it gates — a test that called it
// with a bad name would fail, which is not an assertion — so the rule is
// read here by a unit pin (wallvenue_qa_test.go) that drives all nine
// combinations without a bwrap, a wall, or an environment write. The tree's
// own callers then exercise one row of it each.
//
// THE NAME IS CHECKED FIRST, before the wall. A caller outside D3's filter is
// a defect in the TREE and is true on every host: in the venue, uncaged, and
// inside a caged seat where the wall's own refusal would otherwise hide it
// behind a skip. That is the drift D4 is for — a member the `-run` filter
// does not select is a pin the venue never runs, and the only places that
// would ever notice are this check and a hand audit. It costs a caged seat's
// `make test` a red, which ADR 0096 D3 otherwise forbids; the difference is
// that this red is a line a bead wrote and not the wall working.
func bwrapNestDecision(name, refusal string, venue bool) (nestAction, string) {
	top := wallVenueTopLevel(name)
	if !wallVenueMemberRe().MatchString(top) {
		return nestFail, top + " calls the nest gate, so it is one of ADR 0097 D3's members — and a member's top-level name must match `" +
			wallVenueMemberPattern + "`, which this one does not. `scripts/posse-wall-check` SELECTS the venue's run with that filter and then reads membership off the marker line, so a member outside the filter is a pin the venue never runs and no ratchet ever reds (ADR 0097 D4). Rename the test into the filter, or do not call this gate."
	}
	if refusal == "" {
		return nestMeasure, wallVenueMarker
	}
	if venue {
		return nestFail, bwrapNestSkipPrefix + refusal + "\n" +
			wallVenueEnvVar + "=1 says this process IS ADR 0097 D1's venue, and the body's baseline `bwrap --ro-bind / / /bin/true` has already proved that a bwrap starts in it — so this refusal is something the TREE did and not the wall working, and D4 makes it a red rather than an abstention. Outside the venue this same answer is a skip."
	}
	return nestAbstain, bwrapNestSkipPrefix + refusal
}

// bwrapNestSkip is THE GATE: every live pin in this package that execs a
// bwrap calls it, after its own GOOS and `LookPath("bwrap")` gates — which is
// what lets this one skip the fork off Linux — and before it renders
// anything.
//
// IT NEVER FAILS FOR THE WALL, outside the venue. On this fleet every
// dispatched seat is caged, so inside one the answer is no for a reason that
// is the wall WORKING, and a red that is not a regression teaches people to
// rerun until green (ADR 0096, Alternatives rejected). Its two fail arms are
// both the tree's doing and neither is reachable on a seat with a correctly
// named caller: a caller outside D3's filter (bwrapNestDecision's first
// paragraph), and a refusal inside the venue, where the baseline has already
// proved the wall nests (ADR 0097 D4).
//
// t.Fatal AND t.Log, NOT THE `f` SPELLINGS: bwrap's stderr is pasted into the
// abstention and the venue red, and a `%` anywhere in it would be read as a
// verb by a format call with no arguments.
//
// WHERE THE MEMBERSHIP MARKER GOES, and it is deliberately not at the end:
// the gate logs it the moment it lets the pin proceed, so a pin that then
// skips for its OWN reason is still counted a member and still turns the
// body's verdict into exit 3. That is ADR 0097's `/var/tmp` row — a helper
// change unrelated to the wall took 18 pins dark, and the run's exit code
// said nothing — and it is why D3 counts abstentions instead of reading the
// exit code.
//
// THE SENTENCE IS PINNED BY NOTHING, deliberately, and this is the one place
// that says so. A pin would have to read ADR 0096 D3's own prose to be worth
// more than a second copy of the literal, and a test in THIS package that
// reads the tree joins the class that needs a Makefile door of its own
// (AGENTS.md, "A `-run` filter cannot reach a tree-wide pin"). What stands in
// its place is a measurement on the bead and in
// docs/notes.d/ranger-base-pvkhn.md: the same binary run uncaged and inside
// D17's base, with every skip line quoted. The MARKER is different — it is
// read by a shipped script rather than by a person, so wallvenue_qa_test.go
// holds it to the ADR's literal.
func bwrapNestSkip(t *testing.T) {
	t.Helper()
	venue := wallVenueActive(t)
	switch act, msg := bwrapNestDecision(t.Name(), liveBwrapNestRefusal(), venue); act {
	case nestMeasure:
		t.Log(msg)
	case nestAbstain:
		t.Skip(msg)
	default:
		t.Fatal(msg)
	}
}
