package posse

// The box-wide suite queue, as a LAUNCH OBSERVABLE (ranger-base-r3czg).
//
// scripts/suite-lock.sh hands out POSSE_SUITE_SLOTS flocks over slot files
// in ONE box-wide directory, so at most N full `go test ./...` runs happen
// on this machine at once (ranger-base-uvzjk). Box-wide is the mechanism,
// not an implementation detail: a slot dir inside each session's workspace
// is not a queue, it is N queues of one, and the five-concurrent-suites
// incident that bought the queue comes straight back. So the dir sits
// outside every session's tree, and whether a caged session can open a slot
// file in it is a property of the CAGE, not of the script.
//
// MEASURED 2026-09-10 (ranger-base-r3czg, on a codex seat — gpt-6-astra,
// hand-launched with `posse new` into its own worktree):
// all three of `make test-arm1/2/3` failed to start with
// `suite-slot.*.lock: Operation not permitted`. codex cages itself and its
// writable roots are exactly the ones its rendered launch line NAMES
// (realizeCodex) — the slot dir was named by nothing, so the seat could
// neither take a slot nor tell that from a busy box, and it queued behind
// holders it could not read, leaving two waiting wrappers the operator had
// to end by hand. A claude seat under `cage: seatbelt` takes a slot fine:
// the L2 profile grants `~/.cache` whole, which is where the slots live. So
// the same command, on the same box, had two answers depending on the
// runtime — a PARITY gap (ADR 0017 §2 vocabulary: a gap, not a declared
// difference; nothing about codex makes it unable to queue).
//
// Two halves, because either alone rots:
//
//   - the GRANT. launchWritableRoots names the slot dir, so a
//     self-sandboxing runtime's line carries it. Narrow — the slot dir
//     itself, never its parent `~/.cache`, which would hand the session
//     every other CLI's cache on this box — and only in a tree that HAS the
//     queue, which is three files that live only in this repo
//     (ranger-base-uvzjk, and landingcopy's row on it). A session in any
//     other repo buys nothing.
//   - the ROW. A grant that goes away in silence is the failure this
//     codebase has already paid for twice on the store of record
//     (ranger-base-0fb, then ranger-base-hxhb's row). suiteQueueRow says
//     what the launch's own wall says about the slot dir, beside the record
//     row, wherever parity prints.

import (
	"fmt"
	"os"
	"path/filepath"
)

// suiteLockScriptRel is the queue, by path, relative to a launch dir. Named
// rather than pattern-matched for the reason the QA pin names it: one
// specific script carrying one specific measurement.
const suiteLockScriptRel = "scripts/suite-lock.sh"

// SuiteLockDir resolves the slot directory EXACTLY as suite_lock_dir() in
// scripts/suite-lock.sh does, including the two environment overrides. Two
// spellings of "where the slots live" is a grant that names a directory the
// script does not use — which renders as a pass and behaves as the gap. It is
// exported so the pin that holds the two together can reach both
// (internal/treepins, which runs from the repo root and can source the
// script itself).
//
// A scrubbed environment (no HOME) answers "" rather than inventing a home,
// for ExpandTilde's reason (ranger-base-a3t1): an invented home makes every
// absolute path a child of it, and a writable root derived from one would
// be a grant nobody asked for.
func SuiteLockDir() string {
	if d := os.Getenv("POSSE_SUITE_LOCK_DIR"); d != "" {
		return d
	}
	if c := os.Getenv("XDG_CACHE_HOME"); c != "" {
		return filepath.Join(c, "posse")
	}
	home := os.Getenv("HOME")
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".cache", "posse")
}

// suiteLockRoot is SuiteLockDir for ONE launch dir: the slot dir when this
// tree carries the queue, "" when it does not. The existence test is the
// script's own presence, because that is what decides whether anything in
// this tree will ever ask for a slot — a `make test` in a repo with no
// suite-lock.sh sources nothing and queues nowhere.
func suiteLockRoot(dir string) string {
	if dir == "" {
		return ""
	}
	if _, err := os.Stat(filepath.Join(dir, suiteLockScriptRel)); err != nil {
		return ""
	}
	return SuiteLockDir()
}

// SuiteQueueGate is the parity row's name, in the same shape as
// RecordReachGate and for the same reason: one name for every cage, because
// the question ("can this session take a slot in the box-wide suite queue")
// is the same one however the wall is spelled.
const SuiteQueueGate = "suite: box-wide suite queue"

// applySuiteQueueReach adds that row to a directory-aware parity matrix.
//
// IT NEVER DEGRADES A LAUNCH, and that is deliberate. The queue's own header
// rules that it never makes the suite unrunnable: no lock tool, an
// uncreatable dir, and now an unopenable slot file all end the same way — one
// line and an unserialized run. An ungranted slot dir therefore costs the BOX
// speed, not the session its suite, and a launch refused over it (or forced
// through `--allow-degraded`) would be the ranger-base-d17a mistake: a row
// that renders like a missing wall when no wall is missing. So every arm here
// writes into Realized with class "" — a statement about a grant, never a
// claim about enforcement — and Degraded is untouched.
func (a *App) applySuiteQueueReach(p *Parity, ag *AgentFile, rt *Runtime, dir string) {
	if dir == "" || ag == nil || rt == nil {
		return
	}
	slots := suiteLockRoot(dir)
	if slots == "" {
		// No queue in this tree (or no home to resolve one). Silence, not a
		// row: a line about a file that is not there is noise, and the
		// record row's "nothing to reach" case earns its line only because
		// bd is going to create the thing it names.
		return
	}
	switch {
	case rt.SelfSandbox:
		// Membership of the roots the RENDERED line names — the same
		// artifact, the same reader and the same list semantics the record
		// row uses (codexReachRow). Judged against the line and not against
		// launchWritableRoots' return value, because a PID's own `command:`
		// that drops {deny} renders no roots at all and would otherwise
		// read as a pass.
		roots, readOnly := lineWritableRoots(a.renderedLaunchLine(ag, rt, p.Tier, dir), dir)
		switch {
		case readOnly:
			p.Realized[SuiteQueueGate] = RealizedGate{Detail: fmt.Sprintf(
				"the rendered launch line is `-s read-only`, so nothing is writable and no slot can be taken: suites from this session run UNSERIALIZED against every other suite on this box (scripts/suite-lock.sh degrades, it never refuses). A session that cannot write also cannot run this repo's suite for larger reasons — see the record row")}
		case !anyUnderDir(roots, slots):
			p.Realized[SuiteQueueGate] = RealizedGate{Detail: fmt.Sprintf(
				"%s is in NO writable root the rendered %s line names, so a slot file there answers `Operation not permitted` and this session's suites run UNSERIALIZED against every other suite on this box (ranger-base-r3czg, measured 2026-09-10; the seat that found it sat in the queue instead, behind holders it could not read). launchWritableRoots names this dir in a tree carrying %s — a miss here means a PID `command:` that drops {deny}, or a slot dir moved by POSSE_SUITE_LOCK_DIR/XDG_CACHE_HOME after the launch line was rendered",
				AbbrevHome(slots), rt.Name, suiteLockScriptRel)}
		default:
			p.Realized[SuiteQueueGate] = RealizedGate{Detail: fmt.Sprintf(
				"%s's own sandbox names %s (rendered launch line), so a full suite here takes a box-wide slot like any other seat", rt.Name, AbbrevHome(slots))}
		}
	case p.Cage == CageSeatbelt && a.cageAvailable(p.Cage):
		// Read from the profile's cache grant rather than probed, and said
		// so: the L2 grant is a literal (`~/.cache`, seatbelt.go) and SBPL
		// is last-match-wins, so a trailing deny naming this dir would beat
		// it and only a probe could see that (ADR 0013 §4). This row is a
		// grant statement and never a wall claim — which is exactly why it
		// is allowed to be cheaper than the record row's probe.
		home := os.Getenv("HOME")
		if home != "" && underDir(filepath.Join(home, ".cache"), slots) {
			p.Realized[SuiteQueueGate] = RealizedGate{Detail: fmt.Sprintf(
				"L2 grants ~/.cache whole, which covers %s — read from the profile's cache literal, not probed", AbbrevHome(slots))}
			return
		}
		p.Realized[SuiteQueueGate] = RealizedGate{Detail: fmt.Sprintf(
			"%s is OUTSIDE ~/.cache, which is the only cache grant the L2 profile writes (a literal, not a resolution of POSSE_SUITE_LOCK_DIR/XDG_CACHE_HOME) — so a slot file there is denied and this session's suites run UNSERIALIZED against every other suite on this box (scripts/suite-lock.sh degrades, it never refuses)",
			AbbrevHome(slots))}
	default:
		// shims has no file wall, and the container tier runs the suite
		// inside its own filesystem view, where what queues against what is
		// a question about the mount set nobody has measured. Neither is
		// this bead's gap and neither is asserted here.
		p.Realized[SuiteQueueGate] = RealizedGate{Detail: fmt.Sprintf(
			"not judged at cage %s — %s is reached by whatever this tier's file view says, and the queue degrades to an unserialized run either way", p.Cage, AbbrevHome(slots))}
	}
}
