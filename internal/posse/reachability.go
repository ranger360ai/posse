package posse

// Reachability (ADR 0013 §4, amended 2026-08-27 — ranger-base-hxhb, measured
// in ranger-base-rhw/oyta). The record stage has two inputs, not one:
// `record:` grades the RUNTIME's willingness to write the store of record,
// and the CAGE decides its reachability. A session whose sandbox cannot open
// the bead store cannot do the record stage under any runtime grade —
// the security persona's `record: trusted` claude had `bd sync`, `bd export` and the
// path-limited commit all denied at the db file and at .git/index.lock, and
// nothing observed it: parity grades DENIES, so a cage that denies too much
// prints "all gates realized"; settle looks normal; and the bead — the store
// this contract nominates as truth — shows nothing, which is the one signal
// that cannot be watched for.
//
// So reachability is a LAUNCH observable. Before dispatch, the cage about to
// be used must reach beadsHome(cwd) and its git dirs (beadsGitDirs — the
// resolver SeatbeltWritable and the codex launch line already share), and a
// miss is an ordinary unrealized row: --allow-degraded waives it exactly as
// it waives every other gate, and `posse gates` prints it.
//
// The row has three answers, not two (ranger-base-heur). A probe that runs
// and is refused is a finding; a probe that runs and is granted is a pass;
// and a probe that could not be applied AT ALL measured nothing and must
// say so. The third arm is not hypothetical: a posse command run inside a
// caged persona session cannot apply a nested profile, so every probe here
// fails with the kernel's `sandbox_apply: Operation not permitted` — which
// carries the same three words as a denied write, was read as one, and
// degraded the launch on a measurement that never happened.
//
// The judgement is against the RENDERED ARTIFACT, never against the list
// that fed it. For the seatbelt that is not a preference: SBPL is
// last-match-wins (measured, ranger-base-h15/oyta), so a trailing deny
// naming the beads target re-denies what the allow block still grants, and
// any check that inspects SeatbeltWritable's return value passes exactly
// when it should fail. The class recurs by construction — every future
// narrowing of the writable set reviews as strictly safer — so the check,
// not the review, is what keeps the record stage loud at the next launch
// instead of quiet until the daemon dies.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RecordReachGate is the parity row's name. One name for all three cages:
// the question ("can this session write the store of record") is the same
// one however the wall is spelled, and only the verdict's prose differs.
// Short enough to sit in `posse gates`' gate column beside the PID's own
// rules, because a row nobody reads is a row nobody checks.
const RecordReachGate = "record: store of record"

// reachProbeFile is the probe's own name — created and removed inside each
// target, in one command, under the artifact that will run.
const reachProbeFile = ".posse-reach-probe"

// applyRecordReach adds the reachability row to a directory-aware parity
// matrix. Cage by cage:
//
//   - shims: there is no file wall at this tier, so every path the session
//     can write, it can write. Trivially reachable, no probe.
//   - seatbelt: judged BEHAVIORALLY against the wall the launch will
//     render — a create+remove of a probe file inside each target under it.
//     The artifact that runs is the artifact judged. TWO MECHANISMS realize
//     this tier (WallMechanism, ADR 0080 D16), so there are two arms: on
//     darwin `sandbox-exec -f <the rendered .sb>`, and on Linux the
//     rendered `bwrap.args` typed exactly as the launch line types it
//     (ADR 0080 D18). Two arms and not one because the two mechanisms fail
//     differently and the ROW has to say which — a profile's trailing deny
//     against a mount the kernel locked are different things to go and fix
//     — but they judge the same way, by running the wall over the target.
//   - a self-sandboxing runtime (codex): membership of the writable roots
//     its RENDERED launch line names — the workspace plus every --add-dir
//     realizeCodex emitted. List semantics, no ordering trap, no probe.
//   - the container tier: membership of the RENDERED mount set (CageMounts)
//     — the same list semantics as codex, because a bind mount is a list
//     the engine sorts by destination depth (cageCovering), not a profile
//     with an ordering trap to fall into. Only `home` is judged, not its
//     git dirs: the inner `bd` this tier runs is always `--no-db
//     --no-daemon` (cageinner.go), so a container session appends JSONL
//     and never takes index.lock or moves a ref — unlike codex and L2,
//     where a real `bd sync` commits (ranger-base-w68m; the mount side of
//     the redirect target landed in ranger-base-yu5, this is the row that
//     judges it instead of abstaining).
func (a *App) applyRecordReach(p *Parity, ag *AgentFile, rt *Runtime, dir string) {
	if dir == "" || ag == nil || rt == nil {
		return
	}
	home := beadsHome(dir)
	targets := recordTargets(ag, dir, home)
	if len(targets) == 0 {
		// Nothing is there to reach: no store of record under this launch
		// dir. Said out loud rather than skipped in silence — a check that
		// measured nothing and a check that measured a pass must not print
		// the same line (ranger-base-fm4p).
		// Class "": nothing is there to reach, so no wall is being claimed
		// either way — this row is a "nothing to check" pass, not a gate.
		//
		// AND WHO WOULD CREATE ONE, at the mechanism where that is not the
		// same answer (ADR 0080 D23). "bd creates it on first write" is this
		// row's own sentence and the launcher used to falsify it: bwrap binds
		// only a source that exists, so prep made `<cwd>/.beads` at every
		// launch, and a store that exists is a store this row probes — a
		// LATER launch in the same tree then read a real store behind a
		// read-only sandbox and was refused for it (ranger-base-sg7a7,
		// MEASURED). Prep no longer makes it, so the sentence is true again
		// on both hosts; what the clause adds is that posse will not make one
		// for a seat that cannot, which is a fact about this host an operator
		// reading the row has no other way to learn.
		p.Realized[RecordReachGate] = RealizedGate{Detail: "no store of record at " + AbbrevHome(home) + " — nothing to reach (bd creates it on first write)" + recordReachNoStoreClause(WallMechanism(), p.Cage, a.cageAvailable(p.Cage))}
		return
	}
	switch {
	case p.Cage == CageContainer && a.cageAvailable(p.Cage):
		if why := a.containerReachRow(ag, dir, home); why != "" {
			p.unrealized(RecordReachGate, why)
			return
		}
		p.Realized[RecordReachGate] = RealizedGate{Class: Enforced, Detail: fmt.Sprintf("cage container's rendered mount set names %s read-write (CageMounts, ADR 0002 amendment)", AbbrevHome(home))}
	case rt.SelfSandbox:
		if why := codexReachRow(a.renderedLaunchLine(ag, rt, p.Tier, dir), dir, targets); why != "" {
			p.unrealized(RecordReachGate, why)
			return
		}
		p.Realized[RecordReachGate] = RealizedGate{Class: Enforced, Detail: fmt.Sprintf("%s's own sandbox names %s (rendered launch line, %d targets)", rt.Name, AbbrevHome(home), len(targets))}
	case p.Cage == CageSeatbelt && a.cageAvailable(p.Cage) && WallMechanism() == WallBwrap:
		// ADR 0080 D18's arm. Shaped like the one below, measurement for
		// measurement, and separate from it because every one of those
		// measurements has a different name at this mechanism.
		//
		// Asked before the wall is rendered, for the reason the arm below
		// gives and one of its own: rendering here WRITES — the args file,
		// and the host-side prep bwrap needs because it binds only a source
		// that exists (RenderBwrap) — so a process that cannot put the
		// namespace up should not be making those writes to find out.
		if why := bwrapApplyRefusal(); why != "" {
			// Class "": unmeasured, not enforcement — see recordReachUnmeasured.
			p.Realized[RecordReachGate] = RealizedGate{Detail: recordReachUnmeasuredOn(WallBwrap, why)}
			return
		}
		w, err := a.RenderWall(ag, rt, dir)
		if err != nil {
			p.unrealized(RecordReachGate, "the "+WallBwrap+" wall this launch would run under cannot be rendered, so the store of record cannot be judged reachable: "+err.Error())
			return
		}
		if why := wallReachRow(w, targets); why != "" {
			p.unrealized(RecordReachGate, why)
			return
		}
		p.Realized[RecordReachGate] = RealizedGate{Class: Enforced, Detail: fmt.Sprintf("L2 %s probed at launch: %d targets created and removed under %s", AbbrevHome(w.Artifact), len(targets), w.Mechanism)}
	case p.Cage == CageSeatbelt && a.cageAvailable(p.Cage):
		if !SeatbeltAvailable() {
			// The tier was pinned available on a host that cannot run the
			// artifact (fixtures do this). Nothing to judge and nothing to
			// claim: CheckParity's availability row owns that host.
			return
		}
		// Asked before the profile is rendered, not after: in the session
		// where this is true the render can fail for the same reason (the
		// .sb is a write), and a second finding drawn from the same
		// unmeasured fact is no better than the first.
		if why := sandboxApplyRefusal(); why != "" {
			// Class "": unmeasured, not enforcement — see recordReachUnmeasured.
			p.Realized[RecordReachGate] = RealizedGate{Detail: recordReachUnmeasured(why)}
			return
		}
		prof, err := a.RenderSeatbelt(ag, dir, rt.StateDirs...)
		if err != nil {
			p.unrealized(RecordReachGate, "the seatbelt profile this launch would run under cannot be rendered, so the store of record cannot be judged reachable: "+err.Error())
			return
		}
		why, unmeasured := seatbeltReachRow(prof, targets)
		if unmeasured != "" {
			p.Realized[RecordReachGate] = RealizedGate{Detail: recordReachUnmeasured(unmeasured)}
			return
		}
		if why != "" {
			p.unrealized(RecordReachGate, why)
			return
		}
		p.Realized[RecordReachGate] = RealizedGate{Class: Enforced, Detail: fmt.Sprintf("L2 %s probed at launch: %d targets created and removed under sandbox-exec", AbbrevHome(prof), len(targets))}
	default:
		// Class "": stated plainly as NOT a wall — shims has no file gate at
		// all, so this row is the honest opposite of a claim.
		p.Realized[RecordReachGate] = RealizedGate{Detail: "cage " + p.Cage + " has no file wall — every path this session can write, it can write"}
	}
}

// recordReachUnmeasured is the row for the third answer. It goes in
// Realized and not through unrealized() for the reason the whole bead
// exists: Unrealized feeds Degraded, and a launch must not be degraded by a
// check that did not run. It is not a pass either, and does not read like
// one — a check that measured nothing and a check that measured a pass must
// not print the same line (ranger-base-fm4p).
//
// The clause is "is not known to be able to", not "may not" (ranger-base-
// npqem): since that bead the third answer covers a probe that could not be
// RUN as well as one the kernel refused, and only the second of those is a
// may-not. `why` carries the kernel's own words when they exist, so the
// stronger fact is still in the row where it is true.
func recordReachUnmeasured(why string) string {
	return recordReachUnmeasuredOn(WallSandboxExec, why)
}

// recordReachUnmeasuredOn is that row for a named mechanism (ADR 0080 D18).
// Two whole sentences rather than one with the binary's name substituted
// into it: what the abstention IS differs — applying a profile on one host,
// unsharing a mount namespace on the other — and so does the reason the
// LAUNCH is unaffected, which is the clause an operator acts on. A template
// with a hole in it would have said "outside this process's sandbox" on a
// host where the thing outside is a namespace.
func recordReachUnmeasuredOn(mech, why string) string {
	if mech == WallBwrap {
		return "NOT MEASURED — this posse process is not known to be able to put up a bwrap mount namespace (" + why +
			") and the probe that judges the store of record IS one, so the store is neither reachable nor unreachable here. " +
			"The launch this row judges is unaffected: its bwrap is typed into a pane, outside this process's own namespace. " +
			"Re-run `posse gates` outside the cage for a verdict (ADR 0013 §4, ranger-base-heur; ADR 0080 D18)"
	}
	return "NOT MEASURED — this posse process is not known to be able to apply a seatbelt profile (" + why +
		") and the probe that judges the store of record IS one, so the store is neither reachable nor unreachable here. " +
		"The launch this row judges is unaffected: its sandbox-exec is typed into a herdr pane, outside this process's sandbox. " +
		"Re-run `posse gates` outside the cage for a verdict (ADR 0013 §4, ranger-base-heur)"
}

// recordReachNoStoreClause is the mechanism's half of the row above, "" at
// every mechanism where the launcher's behaviour and bd's are the same
// thing. Mechanism-parameterized for the reason every host-shaped answer in
// bwrap.go is: the Linux arm is unreachable on the box that builds it.
func recordReachNoStoreClause(mech, cage string, available bool) string {
	if mech != WallBwrap || cage != CageSeatbelt || !available {
		return ""
	}
	return ". At this mechanism the launcher does not create one either: bwrap binds only a source that exists and the store of record is not a path posse derives, so bd's first write lands only where the session tree itself is granted (ADR 0080 D23)"
}

// recordTargets is what the record stage writes: the .beads bd actually
// opens for this launch dir, and the git dirs a `bd sync` commit of the
// JSONL locks (index.lock in the per-worktree dir, hooks and refs in the
// common one). Only directories that EXIST are probed — a target that is
// not there yet is not a target the cage denied, and refusing a launch in a
// tree with no bead store would be a check about absence, not about a wall.
//
// A PID THAT DENIES THE WHOLE TREE IS NOT ASKED FOR A COMMIT IN ITS OWN REPO
// (ADR 0086 D3, ranger-base-8t6j1). D3 takes every git dir of the SESSION's
// own repo out of such a PID's wall, deliberately and at both mechanisms —
// so where the store of record is the session's own tree (no
// `.beads/redirect`), `beadsGitDirs(home)` names paths this launch is
// guaranteed not to have, and probing them refuses every such launch for a
// write the PID's own rule forbids. That is not a wall, it is a gate nothing
// can satisfy: the only way to pass it would be to re-grant what D3 removed.
//
// MEASURED 2026-11-10 on the D1 host, bwrap arm, a main checkout OUTSIDE
// `/tmp` (the blanket temp grant makes a fixture inside it pass for the
// wrong reason — ADR 0086's own caveat about its "move main" row): before D3
// the row read `record reach OK (2 targets)`; after D3 and before this
// narrowing it read unrealized, naming `<cwd>/.git` and `Read-only file
// system`, for a PID whose whole output is findings. The open PID reads OK
// either way, which is the control.
//
// THE SAME ANSWER THE CONTAINER ARM ALREADY GIVES one tier down
// (applyRecordReach's own list: "Only `home` is judged, not its git dirs"),
// and for the same reason — a `bd` that cannot commit does not take
// index.lock. What is NOT narrowed is the store of record in ANOTHER repo:
// D3 keeps those git dirs granted because bd commits there, so they stay
// targets and a wall that lost them is still a finding. The test is
// SeatbeltWritable's own base for this PID class (`<dir>/.beads`), so the
// one shape where a redirect stays inside cwd but outside `.beads` —
// `<cwd>/inner/.beads`, ranger-base-f5dg's REPRO 2 — is granted and probed
// rather than silently dropped.
func recordTargets(ag *AgentFile, dir, home string) []string {
	if home == "" || !isDirPath(home) {
		return nil
	}
	out := []string{home}
	if ag != nil && deniesFileWrite(ag.Deny) && underDir(filepath.Join(dir, beadsDirName), home) {
		return out
	}
	for _, g := range beadsGitDirs(home) {
		if isDirPath(g) {
			out = append(out, g)
		}
	}
	return dedupeStrings(out)
}

func isDirPath(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// seatbeltReachRow judges a rendered SBPL profile by running the write the
// record stage needs: create a probe file inside each target and remove it,
// in one `sandbox-exec -f <profile>`. It answers three ways:
//
//	"",  ""          every target answered — the profile grants the store
//	why, ""          a target was refused — the finding, naming the first
//	"",  unmeasured  the kernel refused the APPLY: nothing was measured
//
// The third is checked once, before the loop, because it is a fact about
// this process and not about a target: sandboxApplyRefusal is cached for
// the process's lifetime (seatbelt.go), so if this process could not apply
// at all the caller (applyRecordReach) already abstained before rendering
// the profile, and if it reached here it can apply. A per-probe error can
// therefore only be the target's own write refused, never a fresh apply
// refusal — reclassifying it by matching the PROBE'S OWN OUTPUT against the
// same "sandbox_apply" token read a target path that happens to contain
// that literal (e.g. a store under a dir named sandbox_apply) as an apply
// refusal instead of the denial it is, and the launch went unmeasured
// instead of degraded (ranger-base-2w9l).
//
// Behavior and not inspection, because inspection cannot answer the
// question: the profile's allow block may name the target and a trailing
// deny below it may take it straight back (SBPL is last-match-wins), and
// the deny is exactly the shape a future narrowing arrives in.
func seatbeltReachRow(profile string, targets []string) (why, unmeasured string) {
	if r := sandboxApplyRefusal(); r != "" {
		return "", r
	}
	for _, t := range targets {
		out, err := seatbeltReachProbe(profile, t)
		if err == nil {
			continue
		}
		reason := reachProbeReason(out, err)
		return fmt.Sprintf("%s is not writable under the profile this launch runs (%s): %s — a caged session cannot `bd sync`, `bd export` or commit the JSONL there, so it claims, comments and closes nothing however good the runtime's record: grade is (ADR 0013 §4 reachability, ranger-base-hxhb; measured in ranger-base-rhw/oyta). SBPL is last-match-wins: a trailing deny naming this target beats every grant above it",
			AbbrevHome(t), AbbrevHome(profile), reason), ""
	}
	return "", ""
}

func seatbeltReachProbe(profile, target string) (string, error) {
	f := shellQuote(filepath.Join(target, fmt.Sprintf("%s.%d", reachProbeFile, os.Getpid())))
	out, err := protocolCmd(exec.Command("sandbox-exec", "-f", profile, "/bin/sh", "-c",
		": > "+f+" && rm -f "+f)).CombinedOutput()
	return string(out), err
}

// wallReachRow is seatbeltReachRow's twin for the bwrap mechanism, and it
// judges the wall BY TYPING IT (ADR 0080 D18): the prefix RenderWall
// returned, in front of the same create-and-remove the other arm runs, under
// one `/bin/sh -c`.
//
// THE PREFIX AND NOT A HAND-BUILT ARGV, which is the whole difference from
// the probe script's own T7 row. What this mechanism's wall is spelled as on
// a launch line is a redirection before the command name —
// `3<args bwrap --args 3 --` — and a probe that exec'd bwrap directly with
// the args file on an fd would measure the MOUNTS and say nothing about the
// line posse types. So the shell is the point: if the redirection, the
// quoting of the args path or the resolved binary is wrong, this row is
// where it shows, at a launch rather than on a seat that opened with no wall
// and no complaint.
//
// There is no apply-refusal arm inside this function, unlike its twin. The
// caller (applyRecordReach) asks bwrapApplyRefusal before it renders, because
// rendering this wall WRITES; seatbeltReachRow re-asks because its caller's
// ask is memoized and free. So a per-probe failure here can only be the
// target's own write refused — which is also why the reason is read off the
// output exactly as the other arm reads it.
func wallReachRow(w WallRender, targets []string) string {
	for _, t := range targets {
		out, err := wallReachProbe(w, t)
		if err == nil {
			continue
		}
		return fmt.Sprintf("%s is not writable under the %s wall this launch runs (%s): %s — a caged session cannot `bd sync`, `bd export` or commit the JSONL there, so it claims, comments and closes nothing however good the runtime's record: grade is (ADR 0013 §4 reachability, ranger-base-hxhb). At this mechanism the narrow statement is the LATER mount: a read-only bind rendered over this target beats the write grant below it, and MNT_LOCKED means the session cannot take it off (ADR 0080 D17, probe B6/B6b)",
			AbbrevHome(t), w.Mechanism, AbbrevHome(w.Artifact), reachProbeReason(out, err))
	}
	return ""
}

// wallReachProbe runs one target's create-and-remove behind the typed
// prefix. The inner command is quoted as one argument so that the wall's own
// shell — the `sh` bwrap execs inside the namespace — receives exactly the
// bytes the launch's runtime command would be, and the outer `/bin/sh` is
// only there to perform the redirection.
func wallReachProbe(w WallRender, target string) (string, error) {
	f := shellQuote(filepath.Join(target, fmt.Sprintf("%s.%d", reachProbeFile, os.Getpid())))
	line := w.Prefix + "/bin/sh -c " + shellQuote(": > "+f+" && rm -f "+f)
	out, err := protocolCmd(exec.Command("/bin/sh", "-c", line)).CombinedOutput()
	return string(out), err
}

// reachProbeReason names the probe's failure. It reads from the END of the
// child's combined output, not the start: a dyld/libmalloc line the child
// inherited into its own environment prints to the same stream before
// sandbox-exec's own diagnostic does, so the first line can be loader noise
// while the last is always the diagnostic this row exists to report
// (ranger-base-mtnzv) — the same lines-from-the-end reader errEnvelope
// (herdr.go) already uses against a comparable ordering.
//
// A NIL ERROR IS A CASE, since ADR 0086 D5's witness (wallwitness.go). The
// two rows above call this only after a probe FAILED, so the fallback could
// dereference `err` safely; the witness is a probe that must fail, so it also
// reads the reason off a run that exited 0 and said nothing — which is a
// prefix that swallowed its command, the shape a stub binary in a fixture
// has. That used to be a nil dereference on the launch path, so the fallback
// says it instead of crashing a parity read over a successful exec.
func reachProbeReason(out string, err error) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if s := strings.TrimSpace(lines[i]); s != "" {
			return s
		}
	}
	if err == nil {
		return "it printed nothing and exited 0"
	}
	return err.Error()
}

// renderedLaunchLine is the line a self-sandboxing runtime will really be
// launched with — the same call CreateSession makes, with the same writable
// roots, so what is judged is what runs. A PID's own `command:` that drops
// {deny} lands here too: it names no roots, and that is a finding.
func (a *App) renderedLaunchLine(ag *AgentFile, rt *Runtime, tier, dir string) string {
	return ag.RenderCommandFor(rt, a.ResolveRuntime("", ag), tier, launchWritableRoots(dir)...)
}

// codexReachRow judges the rendered launch line of a runtime that cages
// itself: the roots it makes writable are the workspace it starts in plus
// every --add-dir on the line, and `-s read-only` makes that set empty.
// Membership, not order — the flag is a list and a later entry never
// cancels an earlier one, which is why this arm needs no probe.
func codexReachRow(cmd, dir string, targets []string) string {
	// Before membership, validity: a root the runtime REFUSES makes every
	// answer below moot, because codex validates its writable roots before
	// it applies a sandbox and one bad root refuses the whole set — the
	// session writes the store of record in the sense that it writes
	// nothing at all (ranger-base-k62e, measured on codex-cli 0.150.1).
	// Without this the row reads the refused root as an ordinary grant,
	// covers the target with it, and prints "reachable" over a line that
	// runs no command: the false pass this bead's whole class is about. The
	// launch itself refuses on the same fact (writableRootRefusal);
	// this is what `posse gates` says when nobody is launching.
	if root, comp := refusedWritableRoot(cmd); root != "" {
		return fmt.Sprintf("the rendered launch line names writable root %s, whose component %s is a SYMLINK — a self-sandboxing runtime refuses a writable root with a symlink component before it applies its sandbox, so the session runs no command at all and %s is unreachable along with everything else (ranger-base-k62e; ranger-base-c02a is the same refusal arriving at command-run time, silently). Make %s resolve and the root renders real by itself",
			AbbrevHome(root), AbbrevHome(comp), AbbrevHome(targets[0]), AbbrevHome(comp))
	}
	roots, readOnly := lineWritableRoots(cmd, dir)
	if readOnly {
		return fmt.Sprintf("the rendered launch line is `-s read-only`, so the sandbox makes NOTHING writable — %s included, which is where every `bd claim`, `bd comments add` and `bd close` this session owes lands (ADR 0013 §4 reachability, ranger-base-hxhb). A PID that denies Edit and Write on a self-sandboxing runtime buys the file gate at the cost of the record stage: raise it to cage: seatbelt, or accept the trade with --allow-degraded",
			AbbrevHome(targets[0]))
	}
	for _, t := range targets {
		if !anyUnderDir(roots, t) {
			return fmt.Sprintf("%s is in no writable root the rendered launch line names — the workspace is %s and the line's --add-dir set is [%s], so the sandbox denies the store of record and the session records nothing (ADR 0013 §4 reachability, ranger-base-hxhb; the same shape left five dispatched codex sessions silent in ranger-base-0fb). launchWritableRoots names the store, its git dirs and the session tree's own; a target outside all of them means a PID `command:` that drops {deny}, or a store this resolver cannot reach",
				AbbrevHome(t), AbbrevHome(dir), strings.Join(abbrevAll(roots[1:]), " "))
		}
	}
	return ""
}

// containerReachRow judges the container tier's own artifact — the
// rendered mount set CageMounts would produce — for membership: the
// deepest bind covering `home` decides its mode inside the cage
// (cageCovering, the same rule the mount renderer itself resolves
// overlays by), so this asks the identical question CageMounts answers
// rather than a second, looser one. No probe, because a bind mount is a
// list the engine sorts by destination depth and not by the order this
// list was built in.
//
// The engine loads without error here: a.cageAvailable(p.Cage) already
// proved LoadEngine + the binary's presence on PATH before this is
// called. The session name is synthetic ("reach-probe") and unused by
// anything CageMounts reads from disk — it only spells two paths from it
// (the refusals spool, a socket mount), neither a fact this question
// depends on.
func (a *App) containerReachRow(ag *AgentFile, dir, home string) string {
	e, err := a.LoadEngine(a.ResolveEngine())
	if err != nil {
		return "cage container: engine " + a.ResolveEngine() + " could not be loaded to judge the mount set: " + err.Error()
	}
	ms := a.CageMounts(ag, e, dir, "reach-probe")
	if i, _ := cageCovering(ms, absResolve(home)); i < 0 || ms[i].RO {
		return fmt.Sprintf("%s is not writable under the mount set this launch would render — no read-write bind covers it, so a caged session cannot `bd comments add` or `bd close` there (ADR 0013 §4 reachability, ranger-base-w68m)",
			AbbrevHome(home))
	}
	return ""
}

// lineWritableRoots is the writable set a self-sandboxing runtime's RENDERED
// launch line names: the workspace it starts in, plus every --add-dir on the
// line. readOnly is `-s read-only`, where the set is EMPTY rather than "the
// workspace" — the distinction both rows that read this line turn on
// (rangerhq-5oi: under read-only codex exits on an --add-dir, so the flag is
// not even emitted).
//
// One reader, because two rows ask this of the same line now — the store of
// record (codexReachRow) and the box-wide suite queue
// (applySuiteQueueReach, ranger-base-r3czg) — and two spellings of "what
// this line makes writable" is how one of them passes a line the other
// fails.
func lineWritableRoots(cmd, dir string) (roots []string, readOnly bool) {
	toks := shellTokens(cmd)
	for i := 0; i+1 < len(toks); i++ {
		if toks[i] == "-s" && toks[i+1] == "read-only" {
			return nil, true
		}
	}
	roots = []string{dir}
	for i := 0; i+1 < len(toks); i++ {
		if toks[i] == "--add-dir" {
			roots = append(roots, toks[i+1])
		}
	}
	return roots, false
}

func anyUnderDir(dirs []string, p string) bool {
	for _, d := range dirs {
		if d != "" && underDir(d, p) {
			return true
		}
	}
	return false
}

func abbrevAll(ps []string) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = AbbrevHome(p)
	}
	return out
}

// shellTokens splits a rendered launch line the way the shell that types it
// will: on whitespace outside single quotes, unquoting as it goes.
// strings.Fields is what PIDVoided uses and it is enough for a flag NAME,
// but this reads flag VALUES — paths, which on macOS routinely carry a
// space ("Library/Application Support"), and shellQuote wraps every one of
// them in single quotes.
func shellTokens(s string) []string {
	var out []string
	var cur strings.Builder
	inQuote, has := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inQuote && c == '\'':
			inQuote = false
		case inQuote:
			cur.WriteByte(c)
		case c == '\'':
			inQuote, has = true, true
		case c == '\\' && i+1 < len(s):
			i++
			cur.WriteByte(s[i])
			has = true
		case c == ' ' || c == '\t' || c == '\n':
			if has || cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
				has = false
			}
		default:
			cur.WriteByte(c)
			has = true
		}
	}
	if has || cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}
