package posse

// ADR 0086 D5 — THE EDIT/WRITE ROWS ARE PROVED BY A WITNESS AT THE LAUNCH,
// NOT BY THE TIER BEING AVAILABLE.
//
// `wholeTreeWriteWall` (parity.go) answers a deny of writing the whole
// session tree with `L2 seatbelt` whenever the tier is available and the
// runtime does not cage itself. That is a claim about the HOST — this box has
// a mechanism and posse has a render for it — and the row reads as a claim
// about THIS WALL. ADR 0086's R0 is the two disagreeing out loud: the tier
// was available, the row said `L2 seatbelt`, and the render for that PID in a
// dispatched worktree refused outright (`bwrap cannot grant write on
// <cwd>/.git: it is not a directory`), so the only launch the row described
// was one that could not happen. A row that cannot tell those apart is the
// shape reachability.go's file comment refuses one door over: judge the
// rendered artifact, by running it.
//
// So for a PID whose Edit/Write row makes that claim, the launch TYPES the
// wall and tries the write the rule forbids — `: > <cwd>/.posse-wall-witness
// .<nonce>`, and the same under the session's git objects dir, which D3 took
// out of such a PID's writable set. Both creates must fail. One that lands is
// removed again and the rule is unrealized with the path named, which refuses
// the launch like any other gate the wall does not hold.
//
// A CREATE AND NOT `test -w` (D5). `access(2)` under a seatbelt profile is
// not measured — ASSUMED, and the ADR says so — while a create is exactly
// what the deny forbids, at both mechanisms.
//
// THE PROBE IS NEGATIVE, WHICH IS WHY IT CARRIES TWO CONTROLS. Every other
// probe in this package passes when a command SUCCEEDS, so a probe that
// cannot run at all reads as a failure and is loud. This one passes when a
// command FAILS, so anything that stops the create from being attempted —
// a target that is not there, a stub binary that swallows its argv, a
// namespace that never went up — reads as the wall holding. Three answers
// and not two, then (ranger-base-fm4p's rule, one file over):
//
//   - the BASELINE, outside the wall: posse creates and removes the same path
//     itself first. A path this process cannot write unwalled says nothing
//     about a wall, so that reading is NOT MEASURED rather than a pass.
//   - the MARKER, inside it: the walled command echoes a token before the
//     create, so a run that printed nothing never reached the shell behind
//     the prefix. Also NOT MEASURED — and it is the arm a stub `bwrap` in a
//     fixture lands on, exit 0 with no namespace and no create.
//
// Then a create that did not land, with the baseline passed and the marker
// printed, is the wall refusing the write and nothing else.
//
// WHAT A WITNESS CANNOT SEE (D5, verbatim): a write the wall allows somewhere
// it does not probe. D3's list is the complete statement of where it allows
// writes, and `posse gates` prints it.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// wallWitnessFile is the witness's own name, created under each target
// behind the typed prefix and removed again if it lands. The nonce keeps two
// posse processes in one tree off each other's file, and keeps the baseline's
// `O_EXCL` from meeting a leftover from a run that was killed between the
// create and the remove.
const wallWitnessFile = ".posse-wall-witness"

// wholeTreeSeatbeltClaim is the Detail `wholeTreeWriteWall` writes at the
// seatbelt arm, and the one row this witness upgrades, abstains on, or takes
// away. It is a const shared with that function so the two cannot drift: the
// witness proves exactly the claim parity made, and a row it cannot find is
// a row nobody claimed.
const wholeTreeSeatbeltClaim = "L2 " + CageSeatbelt

// applyWallWitness is D5 at the launch. Directory-aware, so it lives beside
// applyRecordReach and not in CheckParity: which tree the session starts in
// decides both what the wall grants and where the objects dir is, and neither
// is a fact about a persona.
//
// IT KEYS ON THE ROW AND NOT ON THE DENY LIST. `wholeTreeWriteDeny` is read
// by the producers (SeatbeltWritable, sessionGitGrantsFor, recordTargets) and
// by parity's own arm, which is what put the claim in Realized; re-reading it
// here would be a second reading of the same fact, free to disagree with the
// row it is supposed to be judging. The rows ARE the claim: a PID at another
// tier, a self-sandboxing runtime, a path-scoped subtree deny and an already
// unrealized rule all carry a different Detail or none, and every one of them
// is a wall this witness has nothing to say about.
func (a *App) applyWallWitness(p *Parity, ag *AgentFile, rt *Runtime, dir string) {
	if dir == "" || ag == nil || rt == nil {
		return
	}
	// The one predicate for "did this launch get an L2 wall" (seatbelt.go).
	// Asked before anything is rendered, because rendering WRITES.
	if !wallRendered(p.Cage, rt) {
		return
	}
	rows := wallWitnessRows(p)
	if len(rows) == 0 {
		return
	}
	// Can THIS PROCESS put the wall up at all. Asked once, before the
	// render, exactly as applyRecordReach asks it and for both of its
	// reasons: a posse running inside a caged session can type the prefix
	// and cannot apply it, and rendering the bwrap arm writes files and
	// makes host-side prep, which a process that cannot put the namespace up
	// should not be doing to find that out.
	if why := wallApplyRefusal(); why != "" {
		wallWitnessAbstain(p, rows, wallWitnessUnmeasured(WallMechanism(), why))
		return
	}
	w, err := a.RenderWall(ag, rt, dir)
	if err != nil {
		// R0's own case, and the one this row exists for: the tier is
		// available, the claim was printed, and the wall cannot be built.
		for _, gate := range rows {
			delete(p.Realized, gate)
			p.unrealized(gate, "the "+WallMechanism()+" wall this launch runs behind cannot be rendered, so this deny is realized by nothing: "+err.Error()+
				" — the row said `"+wholeTreeSeatbeltClaim+"` because the TIER is available on this host, which is a claim about the host and not about this wall (ADR 0086 D5, R0)")
		}
		return
	}
	for _, t := range wallWitnessTargets(dir) {
		why, unmeasured := wallWitnessRow(w, t)
		if unmeasured != "" {
			wallWitnessAbstain(p, rows, unmeasured)
			return
		}
		if why != "" {
			for _, gate := range rows {
				delete(p.Realized, gate)
				p.unrealized(gate, why)
			}
			return
		}
	}
	for _, gate := range rows {
		row := p.Realized[gate]
		row.Detail = wholeTreeSeatbeltClaim + " (" + w.Mechanism + "), witnessed at launch"
		p.Realized[gate] = row
	}
}

// wallWitnessRows is every gate whose Detail is the availability claim, in a
// fixed order. Sorted because the source is a Go map and two readings of the
// same launch that order their findings differently cannot be diffed
// (rangerhq-epes, the same reason Parity.String sorts).
func wallWitnessRows(p *Parity) []string {
	var out []string
	for gate, row := range p.Realized {
		if row.Detail == wholeTreeSeatbeltClaim {
			out = append(out, gate)
		}
	}
	sort.Strings(out)
	return out
}

// wallWitnessAbstain is the third answer. The row STAYS realized and the
// verdict is untouched — a launch must not be degraded by a check that did
// not run (ADR 0013 §4's own rule, ranger-base-heur) — but it does not read
// like a pass either: the Detail says the wall was not witnessed and why, so
// `L2 seatbelt (bwrap), witnessed at launch` and an abstention are never the
// same line.
func wallWitnessAbstain(p *Parity, rows []string, why string) {
	for _, gate := range rows {
		row := p.Realized[gate]
		row.Detail = wholeTreeSeatbeltClaim + " (" + WallMechanism() + ") — " + why
		p.Realized[gate] = row
	}
}

// wallWitnessTargets is where the witness creates: the session tree itself,
// and the git objects dir of the session's own repo.
//
// TWO AND NOT ONE because D3 and D5 are halves of the same rule. The tree is
// what the PID's deny names; `objects` is what a seat that cannot edit a file
// used to mint a commit in anyway (W14/W15), and D3 took it out of this PID's
// writable set at both mechanisms. A witness over cwd alone would pass over
// exactly the capability D3 removed.
//
// Only a target that EXISTS is probed, for recordTargets' reason and with a
// sharper edge here: a create into a directory that is not there fails with
// ENOENT, and this probe reads a failed create as the wall holding. A repo
// with no objects dir yet is not a wall that refused anything.
func wallWitnessTargets(dir string) []string {
	out := []string{dir}
	if o := sessionGitObjectsDir(dir); o != "" && isDirPath(o) {
		out = append(out, o)
	}
	return dedupeStrings(out)
}

// sessionGitObjectsDir names the object store the session's own repo writes
// into — `<git-common-dir>/objects` — in a linked worktree and in a main
// checkout alike, and "" where dir is no repo.
//
// It asks `rev-parse --git-common-dir` rather than reusing one of the two
// doors beside it, because each answers a NARROWER question on purpose:
// `LinkedGitDirs` is empty in a main checkout (where the objects dir is
// inside cwd, which is exactly where this witness must still probe), and
// `gitCommonDirOutside` is "" for that same shape by design. Joined against
// dir the way both of them join the same flag's answer, since git rewrites a
// relative common dir against the cwd it was asked from.
func sessionGitObjectsDir(dir string) string {
	common, err := git(dir, "rev-parse", "--git-common-dir")
	if err != nil || common == "" {
		return ""
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(dir, common)
	}
	return filepath.Join(filepath.Clean(common), "objects")
}

// wallWitnessRow judges one target by typing the wall over it. Three answers,
// shaped like seatbeltReachRow's:
//
//	"",  ""          the create was REFUSED — the wall holds here
//	why, ""          the create LANDED — the finding, with the path named
//	"",  unmeasured  nothing was measured: see the file comment's two controls
//
// The verdict is the FILE and not the exit code. A prefix that runs nothing
// exits 0 with no create, and reading that as "the write was allowed" would
// degrade a launch over a probe that never ran; reading it as "refused" is
// the false pass this whole file is about. So the file decides, and the
// marker decides whether anything ran at all.
func wallWitnessRow(w WallRender, target string) (why, unmeasured string) {
	nonce := wallWitnessNonce()
	path := filepath.Join(target, wallWitnessFile+"."+nonce)
	marker := "POSSE-WALL-WITNESS-RAN-" + nonce

	// Control one: the same create, by this process, outside the wall.
	if err := createAndRemove(path); err != nil {
		return "", "NOT WITNESSED — posse cannot create " + AbbrevHome(path) + " itself, outside any wall (" + err.Error() +
			"), so a create refused INSIDE the wall would say nothing about the wall. The witness is a create that must fail (ADR 0086 D5), and a target this process cannot write makes it fail for a reason that is not this PID's rule"
	}

	out, err := wallWitnessProbe(w, path, marker)
	landed := false
	removeErr := ""
	if _, serr := os.Stat(path); serr == nil {
		landed = true
		if rerr := os.Remove(path); rerr != nil {
			removeErr = " THE WITNESS FILE IS STILL THERE: " + rerr.Error() + "."
		}
	}
	switch {
	case landed:
		return fmt.Sprintf("%s is WRITABLE under the %s wall this launch runs (%s): the witness created %s behind the rendered prefix.%s This deny is realized by nothing — the row said `%s` because the TIER is available on this host, which is a claim about the host and not about this wall (ADR 0086 D5). A whole-tree write deny drops `cwd` from the writable set and every git dir of this session with it (D3), so a create that LANDS means the set, the carve-out's order or the mount list put one of them back",
			AbbrevHome(target), w.Mechanism, AbbrevHome(w.Artifact), AbbrevHome(path), removeErr, wholeTreeSeatbeltClaim), ""
	case !strings.Contains(out, marker):
		return "", "NOT WITNESSED — the command behind the rendered " + w.Mechanism + " prefix (" + AbbrevHome(w.Artifact) +
			") never printed its marker, so the create was not attempted and no wall was measured: " + reachProbeReason(out, err) +
			". A prefix that runs nothing refuses every write there is, which is why this reads as an abstention and not as a pass (ADR 0086 D5)"
	}
	return "", ""
}

// wallWitnessProbe types the wall over one create. THE PREFIX AND NOT A
// HAND-BUILT ARGV, for wallReachProbe's reason — at the bwrap mechanism the
// wall is a redirection before the command name (`3<args bwrap --args 3 --`),
// so the outer `/bin/sh` is what performs it and a probe that exec'd the
// binary itself would measure the mounts and say nothing about the line posse
// types. Mechanism-neutral because `WallRender.Prefix` is (wall.go): the
// sandbox-exec arm has nothing to redirect and runs through the same shell
// unchanged.
//
// The marker is echoed BEFORE the create and is not the path: a refusal
// prints the path back in the kernel's own diagnostic, so a marker spelled
// out of the file name would be "found" in the message that says the create
// failed.
func wallWitnessProbe(w WallRender, path, marker string) (string, error) {
	inner := "echo " + marker + "; : > " + shellQuote(path)
	line := w.Prefix + "/bin/sh -c " + shellQuote(inner)
	out, err := protocolCmd(exec.Command("/bin/sh", "-c", line)).CombinedOutput()
	return string(out), err
}

// wallWitnessNonce is the per-probe suffix. The pid alone is what
// reachProbeFile uses, and it is not enough here: this probe's baseline
// opens with `O_EXCL`, so one leftover file from a run killed between the
// create and the remove would make every later witness in that tree abstain.
func wallWitnessNonce() string {
	return fmt.Sprintf("%d.%d", os.Getpid(), time.Now().UnixNano())
}

// createAndRemove is the baseline: create the path exclusively, close it, and
// remove it again. `O_EXCL` so a file that is already there is an error
// rather than a silent pass over somebody else's file, and the remove is
// reported because a baseline that leaves its own file behind in the session
// tree is the one side effect this probe must not have.
func createAndRemove(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Remove(path)
}

// wallWitnessUnmeasured is the abstention for a process that cannot put this
// host's wall up at all. Two whole sentences and not one with the binary
// substituted into it, for recordReachUnmeasuredOn's reason: what the
// abstention IS differs — applying a profile on one host, unsharing a mount
// namespace on the other — and so does the clause that says the launch is
// unaffected, which is the part an operator acts on.
func wallWitnessUnmeasured(mech, why string) string {
	if mech == WallBwrap {
		return "NOT WITNESSED: this posse process is not known to be able to put up a bwrap mount namespace (" + why +
			") and the witness that proves this deny IS one, so the wall is neither held nor broken here. " +
			"The launch this row judges is unaffected: its bwrap is typed into a pane, outside this process's own namespace. " +
			"Re-run `posse gates` outside the cage for a verdict (ADR 0086 D5; ADR 0013 §4, ranger-base-heur)"
	}
	return "NOT WITNESSED: this posse process is not known to be able to apply a seatbelt profile (" + why +
		") and the witness that proves this deny IS one, so the wall is neither held nor broken here. " +
		"The launch this row judges is unaffected: its sandbox-exec is typed into a herdr pane, outside this process's sandbox. " +
		"Re-run `posse gates` outside the cage for a verdict (ADR 0086 D5; ADR 0013 §4, ranger-base-heur)"
}
