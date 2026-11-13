package posse

// WHICH MECHANISM REALIZES THE L2 WALL ON THIS HOST (ADR 0080 D16).
//
// `cage: seatbelt` is the name of a TIER — "the L2 path-scoped write wall",
// ADR 0002 §3 — and not of a binary. The binary behind it is a host fact:
// `sandbox-exec` on darwin, `bwrap` on Linux, the way `cage: container` is
// realized by whichever engine the home names. A PID never names the
// mechanism, which is the whole point of keeping the tier's name: the
// promoted constitution is read on both hosts (ADR 0083), so one
// `cage: seatbelt` line has to mean the same demand on each, and a per-host
// tier name would mean a per-host constitution.
//
// THREE QUESTIONS, AND THEY ARE NOT THE SAME QUESTION. WallMechanism below
// answers "which mechanism would put the wall up here"; it is pure, reads no
// host beyond GOOS, and never answers "". `AvailableCages[CageSeatbelt]`
// answers "can it", and that one is measured — on Linux by resolving `bwrap`
// and by an apply probe, because a kernel or LSM that refuses unprivileged
// user namespaces leaves the binary on PATH and refuses the mount (bwrap.go).
// They are separated deliberately: a Linux host whose kernel refuses the
// namespace prints `seatbelt (bwrap)` over a block of ✗ rows, because the
// mechanism there IS bwrap and the rows are what say it did not happen.
// Folding availability into the name would print bare `seatbelt` on that
// host, which reads as a tier with no mechanism rather than as a mechanism
// that was refused.
//
// AND THE THIRD: CAN POSSE TYPE IT (ranger-base-pl3ck). A mechanism this
// build has no render for is a wall nothing puts up, however available the
// host says it is. Before ADR 0080 D18 (ranger-base-djl3c) both of
// `wallRendered`'s call sites answered the first two questions and then
// typed `SeatbeltPrefix` — `sandbox-exec` and nothing else — so the table
// fill reads all three (seatbeltTierProvidedOn) and a Linux host with bwrap
// and a passing probe was REFUSED at parity.go's "cage %s is not available
// on this host/build" rather than handed a seatbelt row over a binary it
// does not have. D18 TYPED it: `RenderWall` below is the one place the
// mechanism is chosen, `WallPrefix` is what goes on the line, and
// wallMechanismRendered now names WallBwrap too. Three questions and not
// two, because collapsing this one into availability would make the apply
// probe answer for a render, and collapsing it into the NAME would print
// bare `seatbelt` on a Linux host whose kernel refuses the namespace — the
// exact reading the paragraph above keeps apart. The question stays asked:
// a GOOS whose mechanism posse cannot render still claims no tier, which is
// what keeps the next mechanism from shipping as a name alone.

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
)

// The mechanism names. Each is the BINARY that puts the wall up, which is
// what makes it worth printing: an operator reading `seatbelt (bwrap)` off a
// parity row has the name of the thing to go and look at.
const (
	WallSandboxExec = "sandbox-exec"
	WallBwrap       = "bwrap"
)

// WallMechanism names what realizes the seatbelt tier on this host:
// `sandbox-exec`, `bwrap`, or "" on a GOOS with neither. See the file
// comment for why this is a naming question and not an availability one.
func WallMechanism() string { return wallMechanismOn(wallMechanismHost) }

// wallMechanismHost is the GOOS WallMechanism answers for. `runtime.GOOS`,
// and a var for the reason bwrapResolves, bwrapApplyRefusal and bwrapBin are
// vars (bwrap.go): the arm it selects is unreachable on the box that builds
// it, so a pin answers it to drive the other host's render from here.
//
// IT IS THE FOURTH SUCH VAR AND THE ONE THE OTHER THREE NEEDED. Those three
// make the bwrap RENDER drivable from darwin, and `renderWallOn`'s mechanism
// parameter makes the switch drivable — but a LAUNCH asks WallMechanism(),
// so no pin could put a launch behind the other mechanism, and the launch
// paths are where the two mechanisms differ in what they REFUSE: ADR 0080
// D19 refuses a claude seat here that darwin walls without comment. That gap
// is how ranger-base-djl3c landed `make test` green on darwin and red on
// every ubuntu arm of ci.yml — seven tests, three arms, main's only gate
// (ranger-base-temhf). TestQALaunchAtTheOtherHostsWallMechanism is what the
// var is for; production never writes it, and a write in production would be
// a seat walled by one mechanism and reported under another.
var wallMechanismHost = runtime.GOOS

// wallMechanismOn is that answer for an arbitrary GOOS, so a pin can read
// the Linux branch from darwin. Nothing but the OS decides it.
func wallMechanismOn(goos string) string {
	switch goos {
	case "darwin":
		return WallSandboxExec
	case "linux":
		return WallBwrap
	}
	return ""
}

// CageLabel renders a cage tier for a report: the seatbelt tier carries the
// mechanism realizing it here, every other tier is its own name.
//
// A LABEL AND NEVER A VALUE. `Parity.Cage` stays the bare tier, because it
// is compared against `CageSeatbelt`, written into session records and read
// back by `Sessions()`; this is only how a row says it to a reader. The two
// surfaces that print it are Parity.String and, through it, `posse gates`.
func CageLabel(cage string) string {
	if cage != CageSeatbelt {
		return cage
	}
	m := WallMechanism()
	if m == "" {
		return cage
	}
	return cage + " (" + m + ")"
}

// The one place the seatbelt tier's availability is filled in, for both
// mechanisms. Two inits — one per mechanism — would each be correct about
// its own host and neither would be the answer; this way a reader asking
// "what makes this tier available" has one function to read — and it is
// seatbeltTierProvidedOn, which is availability AND a render, not
// availability alone (ranger-base-pl3ck).
func init() {
	if seatbeltTierProvidedOn(runtime.GOOS) {
		AvailableCages[CageSeatbelt] = true
	}
}

// seatbeltTierProvidedOn is the whole question the table fill asks: this host
// can realize the L2 wall, AND posse has a render to type for the mechanism
// that would realize it here. Both halves, in the one function init calls, so
// a pin can drive either from the other host.
//
// Parameterized on GOOS for seatbeltTierAvailableOn's reason, and it is the
// GOOS that decides the mechanism, so the second half is pure.
func seatbeltTierProvidedOn(goos string) bool {
	return wallMechanismRendered(wallMechanismOn(goos)) && seatbeltTierAvailableOn(goos)
}

// wallMechanismRendered reports whether this build can TYPE the wall for a
// mechanism — the third question in the file comment, and the one nothing
// between the availability table and the launch line used to ask.
//
// BOTH MECHANISMS SINCE ADR 0080 D18 (ranger-base-djl3c), and the membership
// of this set is the membership of RenderWall's switch below: sandbox-exec
// renders a profile and types `sandbox-exec -f <prof>`, bwrap writes
// `bwrap.args` and types `3<args <abs bwrap> --args 3 --`. The two changed
// together in one commit, which is the invariant
// TestQAWallTableClaimsNoTierThisBuildCannotType holds — a mechanism named
// here with no arm in RenderWall is a tier posse announces and cannot put
// up, and an arm here with the name withheld is a wall posse renders and
// refuses to launch behind.
//
// IT IS STILL A SEPARATE QUESTION from availability and from the name, and
// the reason it stays one outlives bwrap: `RenderWall` refuses the mechanism
// it has no arm for, and this is what keeps that refusal from being the
// place a host first hears about it. A third mechanism — a Landlock
// ruleset, a FreeBSD jail — is a name in wallMechanismOn and an arm here,
// and until it has both the tier is simply not offered there.
func wallMechanismRendered(m string) bool {
	return m == WallSandboxExec || m == WallBwrap
}

// seatbeltTierAvailableOn reports whether this HOST can realize the L2
// write wall, by whichever mechanism this GOOS uses. A host question and not
// a process one, for the reason SeatbeltAvailable's comment gives: what it
// gates is a session posse is about to launch, whose wall binary is typed
// into a herdr pane and runs in herdr's process tree.
//
// HALF of what the table fill asks, and not all of it: a host that can is not
// a host posse can type the wall on. seatbeltTierProvidedOn above is the
// whole question, and this one is deliberately left answering only its own
// (ranger-base-pl3ck).
//
// Parameterized on GOOS so a pin can drive the Linux branch on darwin
// (ranger-base-eoxo1); the switch has already decided the OS by the time
// each arm runs, so SeatbeltAvailable's own darwin check is redundant here
// and left alone rather than split, since that answer has callers of its own.
func seatbeltTierAvailableOn(goos string) bool {
	switch wallMechanismOn(goos) {
	case WallSandboxExec:
		return SeatbeltAvailable()
	case WallBwrap:
		return bwrapAvailableOn(goos)
	}
	return false
}

// ─── D18: TYPING THE WALL ───────────────────────────────────────────────────

// WallRender is one launch's rendered L2 wall: which mechanism put it up,
// the artifact posse wrote that an operator can go and read, and the prefix
// that goes between the PATH assignment and the runtime command.
//
// ONE STRUCT SO THE THREE CANNOT DISAGREE. The prefix names the artifact —
// `-f <prof>` on one mechanism, `3<<args>` on the other — so a caller that
// rendered one and reported the other would print a wall nobody is behind.
// Parity's reachability probe is exactly such a caller: its row names the
// artifact it judged (reachability.go), and it judges by RUNNING the prefix.
type WallRender struct {
	// Mechanism is WallSandboxExec or WallBwrap, never "": RenderWall
	// refuses a GOOS with no arm rather than returning an empty wall.
	Mechanism string
	// Artifact is the file the prefix reads — the rendered `seatbelt.sb`,
	// or the NUL-separated `bwrap.args`. It is what RUNS, which is why
	// bwrap's readable twin (`bwrap.txt`) is not it.
	Artifact string
	// Prefix is typed verbatim in front of the runtime command, trailing
	// space included, and is what every reader of this wall should run
	// rather than recompose.
	Prefix string
}

// ─── WHERE A RENDER THAT IS NOT A LAUNCH LANDS (ranger-base-6s5mo) ──────────

// wallWrite is what a render DOES besides rendering: which directory its
// artifact is written into, and whether the host-side prep bwrap needs is
// run. Three callers, three answers, and until ranger-base-6s5mo they all
// got the launch's.
//
// WHY A LAUNCH IS NOT THE ONLY RENDER. The wall is a FILE at both mechanisms
// — bwrap reads its argv off fd 3, sandbox-exec takes a profile path — so a
// report that names one and a probe that APPLIES one each need the bytes on
// disk somewhere. RenderWall's own header says why they come through the
// same render: a wall judged from anything but the artifact that runs is the
// class reachability.go refuses. What it did not say is where the bytes go,
// and the answer was `a.GatesDir(persona)` for all three.
//
// WHAT THAT COST, MEASURED. `posse gates <persona>` is a report (AGENTS.md
// points a seat at it to READ its wall) and it rendered the live dir of
// whatever persona was on the command line, from the environment of whoever
// typed the command. The git shim bakes the launcher's HOME in as a literal
// and refuses any git whose HOME is not it (rule hookskip:config-env, ADR
// 0079 D1), so `HOME=/elsewhere posse gates <p>` left that running seat's
// git refused by its own gate: 26 FAIL in the seat's `make test`, 24 of them
// fixtures' own `git init`, exit 0, and not one output line saying anything
// had been written (ranger-base-atq78, MEASURED 2026-11-11 on a cage:shims
// seat; the security half is breakage only — a shims seat's L1 rail is
// cooperative by declaration, so no arm was disarmed).
//
// SO: ONLY A LAUNCH NAMES THE LIVE DIR. A report and a probe render exactly
// what a launch would, element for element — the live gates dir is still
// among the render's INPUTS, so the set they print and judge is the set a
// launch would grant — and write those bytes under reportWallDir instead.
type wallWrite struct {
	// Dir is the directory the artifact is written into. "" is the
	// persona's live gates dir, which only a LAUNCH may ask for.
	Dir string
	// Prep is what this render does about the host-side prep (wallPrep
	// below). The zero value is the launch's.
	Prep wallPrep
}

// live reports whether this is the launch's own render — the one write that
// may land in the persona's gates dir.
func (w wallWrite) live() bool { return w.Dir == "" }

// ─── WHAT A READER MAY MAKE ON THE HOST (ranger-base-qd9ei) ─────────────────

// wallPrep is wallWrite's second decision: what a render does about the
// host-side prep this mechanism needs (PrepStep, bwrap.go). THREE values,
// because there are three renders and the launch's answer is wrong for both
// of the others — ranger-base-6s5mo left two and this is its residual.
//
// THE PREP IS A WRITE OUTSIDE THE NAMESPACE, IN THE SEAT'S NAME, FROM
// WHOEVER'S ENVIRONMENT TYPED THE COMMAND. bwrap binds only a source that
// exists, so the render emits a step for every bind source that is NOT there
// — a `mkdir` for a write grant, a `touch` for the ref keep file and the
// session's audit log, an EMPTY FILE for a seeded config the read-only mount
// has to cover. Every one of BwrapArgs' emission sites is guarded on a stat,
// so a step exists exactly where its path is absent, and that is the fact
// prepRequire rests on: a render that asked for NO prep is a wall whose
// every source is already on this host, and applying it writes nothing.
//
// AND AN EMPTY SEEDED FILE IS NOT HARMLESS, which is why a reader abstains
// rather than skipping. `posse gates <persona>` walks parity ONCE PER
// RUNTIME in the catalog and parity's two probes RENDER AND APPLY, so a
// report typed with a HOME that is not the launcher's made that HOME's prep
// paths: MEASURED 2026-11-12 from a bwrap-caged seat into a fresh empty
// HOME — `.bob`, `.cache/posse/plan-usage`, `.cache/posse-seat/<session>`,
// `.config/go/telemetry/local`, and 24 members of `.grok`. One of those
// seeds is how ranger-base-f67jx was found: a zero-byte `~/.grok/config.toml`
// planted by a probe's prep, which posse's OWN project-config classifier
// then read as `invalid JSON: unexpected end of JSON input` and refused a
// later launch over — a P1 red in one arm of `make test`, invisible to any
// `-run` filter because one test plants it and another reads it
// (docs/notes.d/ranger-base-zibko.md has the chain).
type wallPrep int

const (
	// prepMake is the LAUNCH's answer, and a probe asked on a launch's
	// behalf takes it too: that process IS the launcher, it will write these
	// same paths itself a few hundred lines later (WallPrefix, herdrback.go),
	// and the probes run BEFORE that — so a probe with no prep on a first
	// launch into a fresh tree would measure a wall that cannot come up and
	// report the difference as this host's.
	prepMake wallPrep = iota
	// prepSkip is a REPORT's: render the artifact under reportWallDir,
	// describe it, make nothing. A command whose whole job is to describe
	// the wall has no business making it.
	prepSkip
	// prepRequire is a READER'S PROBE's: make nothing, and refuse to judge a
	// wall whose sources are not all here. It cannot simply skip the prep —
	// it APPLIES what it renders — and it may not make it either, so it
	// measures nothing and says so. That is the third answer both probes
	// already have for a wall they could not apply: not granted, not denied
	// — NOT APPLIED (ADR 0013 §4, ranger-base-heur; ADR 0086 D5).
	prepRequire
)

// parityAsk is who wants a directory-aware parity matrix, and it decides
// exactly one thing: which wallPrep the two probes that APPLY the wall
// render with (applyRecordReach, applyWallWitness). It is threaded from
// checkParityIn rather than derived, because nothing the matrix reads can
// tell a launch from a description of one — the inputs are identical and the
// difference is in who is asking.
//
// NOT the `red *l3Redirect` parameter, which is non-nil only from the
// launcher today and is a fact about git's dispatch path: a second meaning
// hung on it would make one of the two drift silently the first time a
// caller has a redirect and no launch, or a launch and no redirect.
//
// THE ZERO VALUE IS THE READER, deliberately: a caller that says nothing
// gets the answer that writes nothing outside the namespace.
type parityAsk int

const (
	// askedByReader is `posse gates <persona>` and every pin that drives the
	// door directly — a description of a launch that is not happening.
	askedByReader parityAsk = iota
	// askedByLaunch is CreateSession, about to run a session in this dir.
	askedByLaunch
)

func (k parityAsk) prep() wallPrep {
	if k == askedByLaunch {
		return prepMake
	}
	return prepRequire
}

// wallPrepAbsentError is prepRequire's refusal: this render needs a host
// path that is not there and this caller may not make it.
//
// A TYPE AND NOT A SENTENCE, because the two probes have to tell it apart
// from every other way a render fails. A render that FAILS is a finding that
// takes the row away and refuses the launch — ADR 0086 D5's R0, the whole
// reason the witness exists — and a wall whose furniture is merely absent is
// an abstention that refuses nothing. Collapsed into one error string they
// would be one branch, and the loud half is the wrong one: a `posse gates`
// in a HOME no seat has launched from would print a wall this host cannot
// realize, over a launch that would have made the path and come up fine.
type wallPrepAbsentError struct {
	// Mechanism is the wall's, for the row's own sentence.
	Mechanism string
	// Step is the FIRST step the render asked for, in BwrapArgs' emission
	// order — which is shallowest-first, so it is also the most useful one
	// to name: a grant's mkdir comes before the touch of a file inside it.
	Step PrepStep
	// N is how many steps the render asked for in all, so a row can say
	// "and 7 more" without carrying the list into a parity Detail.
	N int
}

func (e *wallPrepAbsentError) Error() string {
	return fmt.Sprintf("cage %s (%s): this wall binds %s (%s)%s, and this host does not have it — only a LAUNCH makes what the namespace cannot, so a reader has nothing here to apply (ranger-base-qd9ei)",
		CageSeatbelt, e.Mechanism, AbbrevHome(e.Step.Path), e.Step.Why, e.andMore())
}

// andMore is the count clause, "" for the single step so the common case
// reads as one path and not as "and 0 more".
func (e *wallPrepAbsentError) andMore() string {
	if e.N < 2 {
		return ""
	}
	return fmt.Sprintf(" and %d more path%s", e.N-1, plural(e.N-1, "", "s"))
}

// wallPrepAbsent is that error if `err` is one, else nil.
func wallPrepAbsent(err error) *wallPrepAbsentError {
	var e *wallPrepAbsentError
	if errors.As(err, &e) {
		return e
	}
	return nil
}

// reportWallBase is the one directory this process writes a NON-LAUNCH wall
// into: a fresh temp dir, made once and reused for the life of the process.
//
// ONE PER PROCESS AND NOT ONE PER CALL. A report renders once, but the parity
// table it prints renders twice per runtime in the catalog (applyRecordReach
// and applyWallWitness, both of which APPLY what they render), so an
// `os.MkdirTemp` per render would leave a dozen directories behind per
// command — for a command an operator types all day. Reused, the second
// render of a persona's wall overwrites the first, which is also what makes
// the path printable: the head line of `posse gates` names a file that is
// still there when the operator goes to read it.
var (
	reportWallOnce sync.Once
	reportWallBase string
	reportWallErr  error
)

// reportWallDir is where a non-launch render of this persona's wall lands.
//
// KEYED BY THE INSTANCE AS WELL AS THE PERSONA. `a.StateDir` is what makes
// two homes two walls (seatbeltinstancestate_qa_test.go builds exactly that
// pair in one process), and a hash of it rather than the path because a path
// is not a file name. 0700 on the base, because these bytes describe a
// seat's wall and nothing outside this process needs them.
func (a *App) reportWallDir(persona string) (string, error) {
	reportWallOnce.Do(func() {
		reportWallBase, reportWallErr = os.MkdirTemp("", "posse-wall-report-")
	})
	if reportWallErr != nil {
		return "", fmt.Errorf("cannot make a directory to render %s's wall into for reading (a report and a probe never write the live gates dir — ranger-base-6s5mo): %w", persona, reportWallErr)
	}
	sum := sha256.Sum256([]byte(a.StateDir))
	dir := filepath.Join(reportWallBase, hex.EncodeToString(sum[:])[:12], filepath.Base(persona))
	return dir, os.MkdirAll(dir, 0o755)
}

// wallWriteToRead is the wallWrite a reader of this persona's wall takes:
// the artifact under reportWallDir, and `prep` saying what the reader may
// make on the host (wallPrep — a report makes nothing, a probe asked on a
// launch's behalf makes what a launch makes, and a probe asked by a reader
// requires it to be there already).
func (a *App) wallWriteToRead(persona string, prep wallPrep) (wallWrite, error) {
	dir, err := a.reportWallDir(persona)
	if err != nil {
		return wallWrite{}, err
	}
	return wallWrite{Dir: dir, Prep: prep}, nil
}

// renderWallToJudge is RenderWall for a caller that will APPLY the wall to
// measure this host rather than launch behind it — parity's reachability row
// and the Edit/Write witness (reachability.go, wallwitness.go). The render
// and the prefix are the launch's, byte for byte; what differs is that the
// artifact is written under reportWallDir, so a probe run from `posse gates`
// cannot replace a live seat's wall with one keyed to the caller's
// environment (ranger-base-6s5mo) — and that the host-side prep is made only
// where this probe is a LAUNCH's (`ask`, ranger-base-qd9ei). A reader's
// probe over a wall whose sources are not all here fails with
// wallPrepAbsentError, which both callers read as their third answer.
//
// UNEXPORTED since it took `ask`, and that is not cosmetic: the parameter's
// type is the package's own, so an exported door would be one no caller
// outside this package could call — and the two callers that matter are the
// two probes beside it. Whoever wants a judged wall from outside wants a
// parity matrix, which is CheckParityIn.
func (a *App) renderWallToJudge(ag *AgentFile, rt *Runtime, cwd string, ask parityAsk) (WallRender, error) {
	w, err := a.wallWriteToRead(ag.Name, ask.prep())
	if err != nil {
		return WallRender{}, err
	}
	return a.renderWallInto(WallMechanism(), w, ag, rt, cwd)
}

// RenderWall renders this launch's L2 wall and returns what posse wrote and
// what it will type. THE ONE PLACE THE MECHANISM IS CHOSEN (ADR 0080 D18):
// every caller that used to render a profile and type `SeatbeltPrefix`
// itself asks this instead, so there is a single switch between the tier's
// two mechanisms and no call site that can answer it differently.
//
// It is the third of D16's three questions made concrete — see the file
// comment. `wallRendered` (seatbelt.go) decides WHETHER a launch gets a wall
// and this decides WHAT it gets; the two are kept apart because the first is
// a predicate about the tier and the runtime, and this one writes files and
// can fail.
//
// IT WRITES, AND SO DID WHAT IT REPLACES. `RenderSeatbelt` has written the
// `.sb` at this point in a launch since the tier existed; the bwrap arm
// writes two files and also makes the host-side prep the namespace cannot
// (RenderBwrap, bwrap.go). Callers that only want to READ the wall — a
// report, a probe — still come through here, because a wall judged from
// anything but the artifact that runs is the class reachability.go's own
// file comment refuses.
//
// A MECHANISM WITH NO ARM REFUSES, naming it and the host. Unreachable from
// a launch — `wallRendered` requires AvailableCages[CageSeatbelt], which
// seatbeltTierProvidedOn only fills for a mechanism wallMechanismRendered
// admits, which is this switch's own membership — and spelled anyway,
// because the alternative return is an empty prefix, which is a launch with
// no wall and nothing said.
func (a *App) RenderWall(ag *AgentFile, rt *Runtime, cwd string) (WallRender, error) {
	return a.renderWallOn(WallMechanism(), ag, rt, cwd)
}

// renderWallOn is that render for a NAMED mechanism, so a pin can drive the
// arm this host does not have — the rule every host-shaped decision in ADR
// 0080 follows (wallMechanismOn, seatbeltTierAvailableOn,
// credentialReadDenyLiterals). It is the whole reason the bwrap arm is
// readable from darwin at all: the render is pure of the host except for the
// tree it stats, so a fixture tree is a fixture seat.
//
// The mechanism is a PARAMETER and never a preference. Production has
// exactly one caller and it passes WallMechanism(); a second production
// caller choosing its own would be a seat walled by one mechanism and
// reported under another.
func (a *App) renderWallOn(mech string, ag *AgentFile, rt *Runtime, cwd string) (WallRender, error) {
	return a.renderWallInto(mech, wallWrite{}, ag, rt, cwd)
}

// renderWallInto is that render with `w` saying where the artifact lands and
// whether the prep runs — the launch's `wallWrite{}` for renderWallOn above,
// and a reader's for renderWallToJudge. Split out rather than parameterizing
// renderWallOn, so that the production mechanism switch keeps one caller
// passing WallMechanism() and the destination is a parameter nobody has to
// choose twice.
func (a *App) renderWallInto(mech string, w wallWrite, ag *AgentFile, rt *Runtime, cwd string) (WallRender, error) {
	switch m := mech; m {
	case WallSandboxExec:
		// SeatbeltStateDirs and not rt.StateDirs: with CLAUDE_CONFIG_DIR on
		// the line the home config file is no seat's to write (ADR 0090 D2).
		prof, err := a.renderSeatbeltInto(w, ag, cwd, SeatbeltStateDirs(rt)...)
		if err != nil {
			return WallRender{}, err
		}
		// SBPL has no env of its own, so the seat's cache env (ranger-base-
		// olr0g) rides in front of sandbox-exec as `/usr/bin/env K=V …`: an
		// executable, so it reads the same after an `exec` or an assignment
		// list as sandbox-exec itself did. bwrap's arm sets it in the argv.
		home, _ := os.UserHomeDir()
		return WallRender{Mechanism: m, Artifact: prof, Prefix: seatCacheEnvPrefix(seatCacheEnv(SeatCacheDir(home, cwd))) + SeatbeltPrefix(prof)}, nil
	case WallBwrap:
		args, _, err := a.renderBwrapWall(ag, rt, cwd, w)
		if err != nil {
			return WallRender{}, err
		}
		bin, err := bwrapBin()
		if err != nil {
			return WallRender{}, err
		}
		return WallRender{Mechanism: m, Artifact: args, Prefix: BwrapPrefix(bin, args)}, nil
	}
	return WallRender{}, Die("cage %s: posse has no render for wall mechanism %q (this host is %s, and posse renders %s and %s and nothing else), so there is no L2 write wall to type here (ADR 0080 D16/D18)",
		CageSeatbelt, mech, runtime.GOOS, WallSandboxExec, WallBwrap)
}

// WallPrefix is what the two launch call sites want out of RenderWall: the
// prefix, with the wall rendered as a side effect. It replaced the
// `RenderSeatbelt` + `SeatbeltPrefix` pair that both of wallRendered's call
// sites spelled inline (herdrback.go, the create and the relaunch path).
//
// ONE FUNCTION FOR THE SAME REASON `wallRendered` IS ONE PREDICATE: spelled
// twice, the second spelling is a copy that can drift, and the drift is
// silent in the direction that matters — a launch line typing a binary for
// the wall it did not render.
func (a *App) WallPrefix(ag *AgentFile, rt *Runtime, cwd string) (string, error) {
	w, err := a.RenderWall(ag, rt, cwd)
	if err != nil {
		return "", err
	}
	return w.Prefix, nil
}

// WallReport prints the wall `posse gates <persona>` reads, for whichever
// mechanism realizes the tier here. On darwin it IS SeatbeltReport, byte for
// byte; on Linux it is BwrapReport, which prints the same markers over the
// same lists (ADR 0080 D18).
//
// The runtime and not its state dirs, because the bwrap arm needs the
// runtime itself: claude's state grant at that mechanism is the directory
// its CLAUDE_CONFIG_DIR names and not its declared `state_dir:`
// (BwrapStateDirs, D19), and that substitution is keyed on rt.Name.
func (a *App) WallReport(ag *AgentFile, rt *Runtime, cwd string, out io.Writer) error {
	if WallMechanism() == WallBwrap {
		return a.BwrapReport(ag, rt, cwd, out)
	}
	return a.SeatbeltReport(ag, cwd, out, SeatbeltStateDirs(rt)...)
}

// wallApplyRefusal is "can THIS PROCESS put up the wall it is about to type",
// for whichever mechanism realizes the tier here: "" when it was measured
// able to, else the kernel's own words. The third of D16's three questions
// asked about the PROCESS rather than about the host — a posse running inside
// a caged session can type the prefix and cannot apply it, and a probe that
// judges a wall by running it has to know which of those it just measured
// (ADR 0013 §4, ranger-base-heur).
//
// Mechanism-keyed through renderWallOn's own parameter shape rather than
// through GOOS, so the arm this host does not have is drivable from the other
// one (wallMechanismOn, seatbeltTierAvailableOn, credentialReadDenyLiterals).
// Both halves are memoized for the process's lifetime by their owners
// (sandboxApplyRefusal in seatbelt.go, bwrapApplyRefusal in bwrap.go), so
// asking is free after the first time.
func wallApplyRefusal() string { return wallApplyRefusalOn(WallMechanism()) }

func wallApplyRefusalOn(mech string) string {
	switch mech {
	case WallSandboxExec:
		return sandboxApplyRefusal()
	case WallBwrap:
		return bwrapApplyRefusal()
	}
	// The same refusal RenderWall gives, as a sentence a row can print: a
	// mechanism with no arm is a wall nothing puts up, so nothing can be
	// measured about it either.
	return "this build renders no wall for mechanism " + strconv.Quote(mech) + " (" + runtime.GOOS + "), so there is nothing to apply (ADR 0080 D16/D18)"
}
