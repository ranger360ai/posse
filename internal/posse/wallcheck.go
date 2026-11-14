package posse

// THE WALL'S LIVE PINS MEASURE AFTER EACH GREEN JUDGEMENT (ADR 0097 D1/D2/D5,
// from ranger-base-wplhv). landedcheck.go judges the landed tree; this file is
// the second check that runs behind it, in a cage that can nest.
//
// WHY IT EXISTS. ADR 0096 D3 makes every live pin that nests a bwrap abstain
// inside a wall that forbids nesting. Every seat is such a wall, the judge is
// one too, and the judge does not run the suite arms anyway (ADR 0088 D6). So
// 32 of this tree's live wall pins measured only when an operator ran `make
// test` uncaged, and nothing schedules that. A pin that runs nowhere protects
// nothing — and worse than nothing on one of them:
// `TestLiveBwrapGitRefsRefusesTheSharedGitState` PASSED on a seat because its
// bwrap never started, so the refusal it asserts read true (ADR 0086 D5's
// false pass).
//
// THE THREE PIECES, and only the middle one is new code:
//
//  1. THE VENUE (D1) is boxLandedCage's `landedCageVenue` flag — the judge's
//     own cage plus `--unshare-pid --proc /proc`, a private /var/tmp, and
//     POSSE_WALL_VENUE=1. It is rendered by the SAME function as the judge, so
//     every mask the judge grows later reaches it too. wallVenueArgv below is
//     the three additions and nothing else, and
//     TestQAWallVenueArgvIsTheJudgesPlusADR0097sThree holds that both ways.
//  2. THE STEP (D2) is `wall` here: after a judgement that answered GREEN, on
//     the same tree, in the same run dir, under ADR 0088 D4's rules. D4 comes
//     free — this runs inside once(), on the one goroutine, after the
//     coalescing that chose the tip.
//  3. THE BODY (D3) is `scripts/posse-wall-check`, which the repo owns
//     (ranger-base-lagas), and the RULE (D5) is ADR 0088 D5's ratchet and
//     D6.3's watchdog over two records of its own — parameterised, not
//     copied (landedrule.go).
//
// THREE EXITS AND NOT TWO, which is the one thing this runner does that the
// judge's does not. D3's body answers exit 3 for "no answer": the baseline did
// not nest, no member ran, or a member abstained. An abstention is never a
// pass (D5) — the ADR's own `/var/tmp` row is a fixture change that took 18
// pins dark under a green exit code — so exit 3 is read as SILENCE, counted by
// the watchdog, and never recorded as a verdict.
//
// WHAT IS DELIBERATELY NOT HERE:
//
//   - A SECOND SCHEDULER. The wall check has no Schedule of its own and no
//     goroutine of its own. It is a step inside one landed check, which is
//     what makes "one in flight, coalesced to the newest tip" true rather
//     than reimplemented.
//   - A RUN OF ITS OWN WORKTREE. It judges the worktree the judge judged,
//     byte for byte, inside once()'s defer — one `git worktree add` and one
//     removal per landing, not two.
//   - AN UNCAGED ARM. Like the judge (ADR 0088 D3), a cage that will not
//     render is the end of the run, and on a host whose wall is not bwrap the
//     step does not exist at all (ADR 0096: no darwin change).
//
// RESIDUAL, STATED. A coalesced run whose tree is ALREADY on the judge's
// record returns from once() before judging (D4's cache), so the wall check
// does not run for that tree either — there was no judgement, which is what
// D2 keys on. In practice the cached case is a landing that moved no bytes:
// the tree is the same tree, so the wall check's own record already names it
// or the next landing re-asks. What it costs is one deferred reading when a
// wall-check run was unanswered over a tree that then lands again unchanged.

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	// wallCheckScript is ADR 0097 D3's body, and a tree opts in by shipping
	// it — the same contract ADR 0088 D1 writes for the judge, asked of the
	// same tree with a different path, so a repo may ship either, both or
	// neither.
	//
	// "EXECUTABLE" IS READ AS D1 READS IT, which is a decision and not an
	// omission: the opt-in is a `cat-file -e` on the tree (landedCheckShipped)
	// and the mode is posse's at extraction (0o755, WriteExecutable). A git
	// blob's own mode is not asked for the judge and is not asked here, so
	// the two sibling checks answer "did this tree opt in" the same way. A
	// tree that ships the path non-executable still runs, which is the
	// cheaper failure: the alternative is a venue that silently does not run
	// over a mode bit, and silence is the failure mode this whole ADR is
	// about.
	wallCheckScript = "scripts/posse-wall-check"

	// wallCheckKey and wallCheckQuietKey are ADR 0097 D5's two records:
	// `posse.wallCheck = <tree> <green|red>` and
	// `posse.wallCheckQuiet = <tree> <n>`. Separate from the judge's two for
	// the reason D5 names them separately: a wall regression and a red main
	// are different stretches, and one record cannot ratchet two.
	wallCheckKey      = "posse.wallCheck"
	wallCheckQuietKey = "posse.wallCheckQuiet"

	// wallCheckFileMarkerPrefix and wallCheckHealthMarkerPrefix are the two
	// dedupe lines, and D5's one hard requirement about them is that they
	// differ from the judge's (landedrule.go's header says what a collision
	// costs). The suffix `wall-check` rather than `landed-check` is the whole
	// of the difference, and `TestQATheTwoLandedRulesShareNoRecordOrMarker`
	// is what keeps it one.
	wallCheckFileMarkerPrefix   = "wall-check: "
	wallCheckHealthMarkerPrefix = "wall-check health: "

	// wallCheckNoAnswerExit is D3's third exit. A verdict and not an error:
	// the body answers 3 when it measured nothing — the baseline would not
	// nest, the `-run` filter selected no member, or a member abstained — and
	// every one of those is "no answer" and never a red (D5, "an abstention
	// is never a pass").
	wallCheckNoAnswerExit = 3

	// wallVenueEnvVar is the variable ADR 0097 D1 adds to the venue run's
	// environment allowlist, and the ONLY thing that tells a test process it
	// is inside the venue rather than inside a seat. The two walls are
	// indistinguishable from in there on every other signal — both are a
	// bwrap under `--ro-bind / /` — which is why D1 makes the marker an
	// environment variable the renderer adds rather than something the
	// process can probe.
	//
	// SHIPPED CODE OWNS THE SPELLING, and wallvenue_test.go reads it from
	// here (it was the other way round until this bead: D1's render is what
	// writes the variable, so a test-only const was the one copy nothing
	// could render against). A second literal is the drift that makes a venue
	// run abstain on all 32 members while reporting exit 3, which D5's
	// watchdog reports as silence and not as a wall regression.
	wallVenueEnvVar = "POSSE_WALL_VENUE"

	// wallVenuePidNS and wallVenueProc are ADR 0096 arm C, which is the
	// capability the venue exists for: a pid namespace of the venue's own and
	// a procfs mounted over it. Under the judge's `--ro-bind / /` the host's
	// procfs is read-only, so nothing inside can write its own `uid_map` and
	// nothing can nest a bwrap (ADR 0096 D1, arm A). With these two, every
	// wall-watch pin reads the venue's own processes and mount namespace —
	// MEASURED: 34 of 34 pass in the venue against 15 FAIL and 17 skip on a
	// seat's /proc (ADR 0097's table).
	//
	// THEY ARE A PAIR. `--unshare-pid` without `--proc` leaves the host's
	// procfs over a namespace it does not describe, and `--proc` without
	// `--unshare-pid` is a read-write recursive bind of the host's procfs
	// (ADR 0096's arms B and D) — which is the one thing that ADR refused.
	wallVenuePidNS = "--unshare-pid"
	wallVenueProc  = "--proc"

	// wallVenueVarTmpMount is the second addition's destination, and
	// wallVenueVarTmpName is the directory inside the run dir that is bound
	// over it.
	//
	// WHY /var/tmp AT ALL, and it is the ADR's own fourth row rather than a
	// precaution: `wallUngrantedDir` makes its fixture root under /var/tmp,
	// which `--ro-bind / /` makes read-only, so without this bind 18 of the
	// 32 pins SKIP — "16 pass, 18 skip" in 18.8s, against 34 pass in ~100s
	// with it. A helper change unrelated to the wall took 18 pins dark, and
	// that is the measurement behind D5's "count the abstentions, never the
	// exit code".
	//
	// A BIND FROM THE RUN DIR AND NOT A TMPFS, which is the judge's private
	// /tmp one line over (ranger-base-7wdb7): the directory goes with the run
	// when the run is removed, and until then an operator reading a failed
	// venue run can still open what it wrote.
	wallVenueVarTmpMount = "/var/tmp"
	wallVenueVarTmpName  = "vartmp"
)

// wallCheckRule is ADR 0097 D5's rule: ADR 0088 D5's ratchet and D6.3's
// watchdog, keyed on this check's own two records and dedupeing on its own
// two markers. Everything else — the transition table, the three answers a
// store can give, the gating on a record that could not be written, the
// window and its attribution — is the judge's code, unchanged.
var wallCheckRule = landedRule{
	Name:               "wall check",
	Agent:              "wall check",
	Script:             wallCheckScript,
	Extract:            "wall-check",
	Key:                wallCheckKey,
	QuietKey:           wallCheckQuietKey,
	FileMarkerPrefix:   wallCheckFileMarkerPrefix,
	HealthMarkerPrefix: wallCheckHealthMarkerPrefix,
	// The subject is the PINS and not the branch: a wall regression is not
	// "main is red" — every seat's `make test` is green over these pins by
	// design (ADR 0096 D3) — so a bead titled "main red at <tree>" would
	// send its reader to a suite that cannot show them the finding. This is
	// also the title ADR 0097 D5 writes.
	RedSubject:    "wall pins",
	RedWhen:       "in ADR 0097 D1's venue after a green judgement, outside the lock (ADR 0097 D2)",
	RedAdvice:     "What to do: reproduce it in the venue, which is the only wall on this box where these pins measure — `bash docs/adr/0097-wall-venue.probe.sh venue+check` runs `%s` in it — or run it uncaged, which is still the control (ADR 0097 D6). A caged seat ABSTAINS on %s (ADR 0096 D3), so a seat's green `make test` is not a reproduction and never was. Fix it on a branch and land it as usual. ",
	CeilingTitle:  "its ceiling",
	SilentAdvice:  "What to do: run `%s` by hand on a fresh checkout of %s, inside the venue (`bash docs/adr/0097-wall-venue.probe.sh venue+check`), and time it. Exit 3 names what it could not measure: a baseline that would not nest is the venue or the kernel, zero members is the `-run` filter or the marker, and an abstaining member is the tree (ADR 0097 D3's table). If it answers there, the silence is the cage this launcher rendered; if it does not, %s bounded a run that was already going nowhere. ",
	CeilingAdvice: "What to do: price the run (`%s` is one filtered `go test` over ./internal/posse, measured at 94-104s cold in the venue) and decide whether the price is worth paying. ADR 0097's own Consequences accept about 100s per coalesced green judgement; a reading well past that is either the box or a member that has started to wait for something, and the per-member walls are in the run's own output. ",
	SilentCost:    "no landing's wall pins are measured anywhere — a caged seat abstains on them by design (ADR 0096 D3) — nothing is recorded, and ADR 0097 D5's ratchet files nothing",
	RunADR:        "ADR 0097 D2",
	FileADR:       "ADR 0097 D5",
	HealthADR:     "ADR 0097 D5",
	PriceADR:      "ADR 0097 D5",
	BoundsADR:     "ADR 0097 D5",
	CeilingADR:    "ADR 0097 D5",
}

// wallVenueArgv is ADR 0097 D1's three additions to a rendered judge argv,
// and the prep the launcher owes the second of them. It takes the judge's
// argv and returns the venue's, which is the shape that makes "plus exactly
// three things" something a pin can read: the only difference between the two
// is what this function appends.
//
// APPENDED, AND THAT IS THE SEMANTICS (BwrapArgs' "the order is the
// semantics"): bwrap applies mounts in argv order and a later mount covers an
// earlier one, so a procfs added here lands over the base `--ro-bind / /` and
// the /var/tmp bind lands over whatever the base put there. Neither covers a
// mask or a hide — the judge's layers are all elsewhere in the tree — so the
// venue is never weaker than the judge, which is D1's first bullet.
//
// `tmp` is carve.Tmp's presence, and it gates the /var/tmp bind alone: see
// the call site for why they stand down together.
func wallVenueArgv(argv []string, run string, tmp bool) ([]string, []PrepStep) {
	out := append(argv, wallVenuePidNS, wallVenueProc, wallVenueProcMount)
	if !tmp {
		return out, nil
	}
	src := filepath.Join(run, wallVenueVarTmpName)
	return append(out, "--bind", src, wallVenueVarTmpMount),
		[]PrepStep{{PrepMkdirAll, src, "ADR 0097 D1's private /var/tmp for the wall venue; bwrap cannot bind a source that does not exist, and 18 live pins put their fixture root there"}}
}

// wallVenueProcMount is where that procfs goes. The literal, named, because it is
// also what `--proc` takes as its argument and a reader of the argv should
// find one spelling of it.
const wallVenueProcMount = "/proc"

// wall is ADR 0097 D2: the step after a judgement.
//
// IT IS CALLED ON EVERY JUDGED RUN, with the judge's verdict, and the first
// three questions it asks are in this order and for these reasons:
//
//  1. DID THE TREE OPT IN. A repo that ships no `scripts/posse-wall-check`
//     gets nothing — not a line, not a record — which is every repo until it
//     ships one. Asked first so the other two never print at a repo that
//     never asked for this step.
//  2. IS THIS HOST'S WALL A BWRAP. If it is not, the step DOES NOT EXIST (D2,
//     ADR 0096: no darwin change). It says so and counts nothing: a host that
//     was never asked to answer has not gone silent, and the watchdog must
//     not file for it.
//  3. DID THE JUDGE ANSWER GREEN. Red or silent, this records nothing and
//     says "not run (judge <verdict>)" — which is neither a green nor a
//     silence of its own, so ONE broken tree is filed ONCE (D2).
func (c *landedChecker) wall(repo, base, tip, tree, run string, judged landVerdict) {
	r := wallCheckRule
	if !landedCheckShipped(r, repo, tree) {
		return
	}
	if m := WallMechanism(); m != WallBwrap {
		c.say("· %s: wall check not run — this host realizes cage %s with %s, and ADR 0097 D1's venue is a bwrap (ADR 0096: no darwin change), so the step does not exist here", AbbrevHome(repo), CageSeatbelt, m)
		return
	}
	if judged != landGreen {
		c.say("· %s: wall check not run (judge %s) — it runs on a tree the judge answered GREEN on, and this one is not recorded either way (ADR 0097 D2)", AbbrevHome(repo), judged)
		return
	}
	// D5's left-hand column, read BEFORE the run for the reason once() reads
	// the judge's: the record as it stands now is the transition's left half,
	// and — when it is green — the far edge of the window a filed bead names.
	had, prev := landedRecorded(r, repo)
	start := time.Now()
	v, said, argv := c.measure(repo, run, tree)
	took := time.Since(start)
	// D2's verdict line carries the venue's argv and the wall, on every one
	// of the three answers: the argv because a venue that rendered the wrong
	// wall is the first thing to suspect when 32 pins abstain, and the wall
	// because ADR 0097's price is read off these lines (its Consequences
	// accept about 100s a judgement). An unanswered run may have no argv at
	// all — a cage that would not render is one of the ways to be unanswered
	// — and the line says that rather than printing an empty field.
	argv = orElse(argv, "none rendered")
	switch v {
	case landUnanswered:
		c.warn("%s wall pins NOT MEASURED at %s after %s — %s — nothing recorded (ADR 0097 D5: an abstention is never a pass)\n  venue: %s", AbbrevHome(repo), abbrevSHA(tip), landedCheckWall(took), said, argv)
	case landGreen:
		// THE GREEN LINE CARRIES THE BODY'S LAST LINE, which the judge's does
		// not, and the difference is D3's third exit: that line is `wall
		// check: green, <n> pins measured`, and `n` is the only thing in the
		// run that says the venue measured anything at all. A green exit over
		// zero measured pins is the hole the third exit closes (D5, "an
		// abstention is never a pass"), so the count belongs where a human
		// reads the verdict rather than only in the exit code.
		c.say("✓ %s wall pins green at %s in %s (%s, ADR 0097 D2) — %s\n  venue: %s", AbbrevHome(repo), abbrevSHA(tip), landedCheckWall(took), r.Script, orElse(oneLine(said), "it said nothing"), argv)
	default:
		c.warn("%s wall pins are RED at %s in %s — %s — %s (ADR 0097 D2)\n  venue: %s", AbbrevHome(repo), abbrevSHA(tip), landedCheckWall(took), r.Script, said, argv)
	}
	if v == landUnanswered {
		// Silence records nothing and files nothing under the ratchet, so the
		// watchdog is the only thing that can say these pins have stopped
		// measuring. Same shape as once()'s unanswered arm, and the same
		// reason.
		c.health(r, repo, tree, v, took, said)
		return
	}
	recorded := true
	if err := recordLandedCheck(r, repo, tree, v); err != nil {
		recorded = false
		c.warn("%s: the wall check judged %s %s and the verdict could not be recorded (%v)", AbbrevHome(repo), abbrevSHA(tree), v, err)
	}
	c.file(r, repo, base, tip, tree, had, prev, v, said, recorded)
	c.health(r, repo, tree, v, took, said)
}

// measure extracts D3's body and runs it in D1's venue over the judge's own
// worktree. The three returns are the verdict, the cause, and the venue argv
// the verdict line prints.
//
// THE LAUNCHER RUNS ONLY GIT HERE, as the judge's does (ADR 0088 D3): the
// `cat-file` that reads the body out is the whole of what this process
// executes outside the cage.
func (c *landedChecker) measure(repo, run, tree string) (landVerdict, string, string) {
	r := wallCheckRule
	path := filepath.Join(run, landedTreeName)
	check, from, err := c.extract(r, repo, run, tree)
	if err != nil {
		return landUnanswered, oneLine(err.Error()), ""
	}
	// The judge made the worktree, the TMPDIR and the build cache before this
	// ran (ADR 0097 D2: after a green judgement), and this run gets the same
	// three. The same GOCACHE deliberately: the judge built this tree a moment
	// ago, and a second cold cache would pay for the same build twice inside
	// one run dir that is about to be removed.
	cage, err := c.cageFn()(c.a, landedCageVenue, check, path, landedCheckWritable(run, path), landedRunEnv(run))
	if err != nil {
		// A venue that will not render is no answer, exactly as a judge's
		// cage that will not render is (ADR 0088 D3) — and D5 says so in as
		// many words: "a venue that will not render" counts as unanswered.
		// There is no uncaged arm to fall through to.
		return landUnanswered, oneLine(err.Error()), ""
	}
	c.sayOwnCopy(r, repo, from, tree)
	ctx, cancel := context.WithTimeout(context.Background(), landedCheckLimit)
	defer cancel()
	cmd := protocolCmd(exec.CommandContext(ctx, cage.Argv[0], cage.Argv[1:]...))
	cmd.Dir = path
	cmd.Env = landedJudgeEnv(cage.Env)
	// THE JUDGE'S OWN VERDICT RULE, through the judge's own binder: the
	// deadline read off the context, the reaping grace for a child with
	// grandchildren, a signal that is not this deadline, the tail. Called
	// directly rather than through a wrapper of this file's, because
	// `landChildVerdict` is where `cmd.WaitDelay` is set and a wrapper that
	// did not set it reads — correctly — as a child under a deadline with no
	// grace (internal/treepins/execwaitdelay_qa_test.go, ranger-base-2cttm).
	v, said := landChildVerdict(ctx, cmd, landedCheckLimit)
	// The one difference, and it is ADR 0097 D3's: exit 3 is an ANSWER of
	// "no answer". -1 for a child that never got a status, which is every
	// arm landChildVerdict already answered unanswered for.
	exit := -1
	if cmd.ProcessState != nil {
		exit = cmd.ProcessState.ExitCode()
	}
	v, said = wallThirdExit(v, said, exit)
	return v, said, strings.Join(cage.Argv, " ")
}

// wallThirdExit is ADR 0097 D3's third exit, read over landChildVerdict's
// answer: an exit of 3 is NO ANSWER and never a red.
//
// WHY IT MATTERS THAT THIS IS NOT A RED (D5): the three things that produce
// exit 3 are a baseline that would not nest, a `-run` filter that selected
// nothing, and a member that abstained. None of them is a finding about the
// tree, and all three are a measurement that did not happen — so reading one
// as red would file a P1 against the newest landing for a wall that was never
// asked a question.
//
// IT NARROWS AND NEVER WIDENS: only a RED is reconsidered. A green is a green
// whatever the exit (there is only one), and an answer that was already
// unanswered — a deadline, a signal, a child that would not start — keeps the
// sentence landChildVerdict built for it, which says which of those it was.
//
// A function of three values and not a reader of the Cmd, so the rule is
// readable without a process to run (TestQAWallThirdExitReadsOnlyARedThree).
func wallThirdExit(v landVerdict, said string, exit int) (landVerdict, string) {
	if v != landRed || exit != wallCheckNoAnswerExit {
		return v, said
	}
	return landUnanswered, fmt.Sprintf("exit %d, no answer: %s", wallCheckNoAnswerExit, orElse(said, "it said nothing"))
}
