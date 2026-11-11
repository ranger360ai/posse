package posse

// A REPLAYED LANDING IS JUDGED BY THE REPO'S OWN CHECK, AND MAY NOT TURN A
// GREEN BASE RED (ADR 0087, from ranger-base-qk7w6).
//
// MergeSessionWork lands a session branch two ways. If the base has not
// moved it fast-forwards the tip the seat committed and tested. If the base
// MOVED it replays the branch onto the new base in the session's own tree
// and fast-forwards the replayed tip — and nothing compiled that tip. git
// only refuses TEXTUAL conflicts, so a semantic one lands silently.
//
// MEASURED on ranger-base-s3a9g: 4fbef672 (on 1089eb7f) and
// ranger-base-r5131's 10d046f9 each built clean; the launcher replayed the
// first onto the second as a2327230, which `go build ./...` refuses with two
// errors and `go test -c` with seven more. Every seat's `make test`,
// `tree-check` and `coldexec-check` went red, `make install` could not run,
// and `checkorphans` exited 1. ci.yml runs AFTER the landing, so it saw this
// only once main was already red. 147 of main's last 220 commits that week
// were replayed (author time ≠ committer time), so the population is most of
// them.
//
// The field's answer to the shape is a merge queue: test the combined result
// and advance main only on a pass (bors-ng, GitHub's merge queue — ADR 0087
// cites both). What differs here is that main also moves OUTSIDE the queue:
// the operator commits in the shared checkout, and the unreplayed
// fast-forward arm is not judged (ADR 0087 Alternatives says why). So the
// base can already be red when a branch arrives, and a gate that blocked on
// any red would strand every branch — including the fix for the red.
//
// SO THE RULE IS A RATCHET, and it is the whole of D3:
//
//	replay tip | base   | outcome
//	-----------+--------+----------------------------------------------------
//	green      | any    | land; record the tip's TREE as green
//	red        | green  | BLOCK: reset the branch to its pre-replay tip, and
//	           |        | file the merge-back block with the check's output
//	red        | red    | land, ⚠ "the base was already red"
//	no answer  | any    | land, ⚠; never block
//
// Only "the two sides disagree" — the a2327230 incident — is the branch's
// fault, and only that case blocks. A base somebody else broke strands
// nobody, a partial fix still lands (which matters, because the fix for a red
// main comes through this same path), and a gate that cannot answer never
// strands the fleet, because ci.yml stays the backstop.
//
// WHOSE PROGRAM RUNS, and it is the same decision generatedindex.go's header
// makes for the generator: the BASE checkout's copy, the copy a human
// reviewed, with the SESSION TREE as its cwd. A branch that edits the script
// does not change what judges it. Unlike the generator, the two copies are
// NOT required to be byte-identical — a branch whose whole subject is the
// check still has to pass the check in force, exactly as a pull request does.
//
// WHAT IT COSTS. About 6s warm and 16s cold per replay for this repo's own
// script (MEASURED 2026-11-10, ADR 0087's table), paid inside the launcher
// lock, only on the replay arm, up to ×3 on the retry loop and ×2 on a red
// replay over a base whose verdict is not on record. The exit hatch if that
// ever stops being affordable is ADR 0087's last Alternative — replay and
// judge outside the lock, and take it only for the swap.
//
// THE RESIDUALS, STATED. (1) The unreplayed arm can still land a tip the seat
// never built; that is seat discipline and ci.yml catches it. (2) A
// red-over-red landing can add errors nobody attributes. (3) A toolchain
// upgrade does not change a tree id, so a cached verdict can outlive it by
// one landing. (4) The check sees only what it compiles — tests are not in
// the lock (D6). (5) A file at the script's path WITHOUT its execute bit is
// an opt-in whose exec fails, which lands with a ⚠ rather than silently
// judging nothing; see landCheckShipped.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// landCheckScript is the ONE new contract this adds, and the only reason
// posse stays repo-agnostic about what "green" means: the repo ships an
// executable here, takes no arguments, is handed the tree to judge as its
// cwd, exits 0 for green and anything else for red, and its combined output
// is the cause. The `posse-` prefix is why a repo's own `scripts/` cannot
// match it by accident (D1); the shipped-file shape is generatorShipped's,
// so no config key is coined for it.
const landCheckScript = "scripts/posse-land-check"

// landCheckKey is D4's whole record: one repo-level git config entry holding
// `<tree-id> <green|red>` — the last tree this gate judged, and its verdict.
//
// KEYED ON THE TREE AND NOT THE COMMIT because the check reads BYTES. Two
// commits with one tree are one question, which is what makes the common case
// free: a green landing records the tip's tree, that tree is the new base's
// tree, and the next replay's "is the base green?" is a lookup. The script
// itself lives in the tree, so changing it changes the key.
//
// It also means a record written for a tip whose fast-forward then LOST the
// race is still true — "this tree was judged green" is a fact about the tree
// and not about what landed — which is why the record is written before the
// swap and never unwound.
//
// One key and not one per tree: this is a cache with one useful entry (the
// base's), and a config section that grew an entry per landing would be a
// store nothing ever prunes.
const landCheckKey = "posse.landCheck"

// landCheckTailBytes bounds what is KEPT of the check's output while it runs,
// and landCheckTailLines how many of those lines are quoted as the cause. The
// check is `go build ./...` and friends for this repo, so the output is
// bounded in practice; the cap is here because nothing in the contract says
// it has to be, and an unbounded child's stdout accumulating in the launcher
// under the fleet lock is not a failure mode worth discovering in the field.
//
// The LAST lines and not the first, which is D1's wording ("the last lines of
// combined output are the cause") — then flattened to one line and bounded
// the way gitSaid bounds git's complaint, because this sentence is embedded
// verbatim in a merge-back bead somebody reads days later.
const (
	landCheckTailBytes = 8 << 10
	landCheckTailLines = 3
)

// landVerdict is the check's answer, and the third value is the point: "could
// not answer" is not "red". A timeout and a child that would not exec say
// nothing whatever about the tree, and D3 makes both of them LAND — a gate
// that reads its own silence as a verdict is the failure question 3 of
// ranger-base-qk7w6 was afraid of.
type landVerdict int

const (
	landUnanswered landVerdict = iota
	landGreen
	landRed
)

// String is the word the record holds, and the one a human reads back out of
// `git config --get posse.landCheck`.
func (v landVerdict) String() string {
	switch v {
	case landGreen:
		return "green"
	case landRed:
		return "red"
	default:
		return "unanswered"
	}
}

// landCheckShipped reports whether the base's checkout opts in at all — a
// REGULAR file at the contract's path. Absent is today's behaviour exactly,
// which is every repo that has not shipped one.
//
// NON-REGULAR IS NOT SHIPPED, for ranger-base-gs9r's reason one function
// over: an executable FIFO passes a directory-and-exec-bit check and then
// blocks the reader forever. Here it would block the launcher for
// generatorTimeout while holding the fleet lock, and answer nothing.
//
// THE EXECUTE BIT IS NOT ASKED, which is this file's one divergence from the
// ADR's literal wording ("a repo opts in by shipping an EXECUTABLE at …"), and
// it is a divergence in the loud direction. A file sitting at a path with
// posse's own prefix in it is an opt-in whatever its mode; reading a missing
// mode bit as "this repo has no check" would mean the one failure that silently
// restores the a2327230 hole — judged nothing, said nothing. So the mode is
// left to exec, whose refusal is a "could not answer" that lands with a ⚠
// naming the file (D3's bottom row). The opt-in is the file; the bit is what
// makes it work.
func landCheckShipped(repo string) bool {
	fi, err := os.Stat(filepath.Join(repo, landCheckScript))
	return err == nil && fi.Mode().IsRegular()
}

// tailBuf keeps the LAST landCheckTailBytes of what is written to it and
// forgets the rest, so a check that prints without end costs one buffer.
type tailBuf struct{ b []byte }

func (w *tailBuf) Write(p []byte) (int, error) {
	n := len(p)
	if len(p) > landCheckTailBytes {
		p = p[len(p)-landCheckTailBytes:]
	}
	w.b = append(w.b, p...)
	if len(w.b) > landCheckTailBytes {
		w.b = append(w.b[:0], w.b[len(w.b)-landCheckTailBytes:]...)
	}
	return n, nil
}

// landCheckSaid is gitSaid's shape over a child that is not git: the last few
// non-empty lines of combined output, flattened to one line, joined the way
// gitSaid joins and bounded the same.
func landCheckSaid(out string) string {
	var keep []string
	for _, ln := range strings.Split(out, "\n") {
		if ln = strings.TrimSpace(oneLine(ln)); ln != "" {
			keep = append(keep, ln)
		}
	}
	if len(keep) > landCheckTailLines {
		keep = keep[len(keep)-landCheckTailLines:]
	}
	said := strings.Join(keep, "; ")
	const most = 400
	if r := []rune(said); len(r) > most {
		said = "…" + strings.TrimSpace(string(r[len(r)-most:]))
	}
	return said
}

// runLandCheck runs the BASE's check over the tree at `tree` and says what it
// answered. The second return is the cause to quote: the output tail for a
// red, and why there is no answer for landUnanswered.
//
// Bounded exactly like the one other non-git child this path starts
// (runNotesIndex): generatorTimeout for the child's life and
// generatorKillGrace as cmd.WaitDelay, because os/exec's Wait does not return
// until the goroutines copying the child's output are done and a GRANDCHILD
// that inherited the pipe keeps it open after the child is killed. This check
// definitely has grandchildren — this repo's is a shell script running `go`
// — so the grace is load-bearing here rather than held on principle.
//
// THE THREE ANSWERS COME FROM THE ERROR'S TYPE AND NOT ITS TEXT, which is
// credential.go's lesson at line 1045: an *exec.ExitError is the only error
// that means the program RAN and the system answered. Everything else — no
// such file, not executable, ETXTBSY, any transient fork/exec failure under
// load — is this process failing to ask. And a child that was SIGNALLED is
// the third case rather than a red (ranger-base-zfza8's doctrine, one child
// over): the deadline is a statement about posse's silence, never a verdict
// about the tree.
func runLandCheck(repo, tree string) (landVerdict, string) {
	return runLandCheckWithin(repo, tree, generatorTimeout)
}

// runLandCheckWithin is the same call with the bound in hand, and the bound is
// a parameter for ONE reason, said out loud so nobody reads it as
// configuration: the deadline arm above is a real outcome with a sentence of
// its own, and the only other way to reach it in a test is to spend
// generatorTimeout — two minutes of suite wall, inside a package that is
// already ~950s. Production has exactly one caller and it passes the const.
func runLandCheckWithin(repo, tree string, limit time.Duration) (landVerdict, string) {
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	bin := filepath.Join(repo, landCheckScript)
	cmd := protocolCmd(exec.CommandContext(ctx, bin))
	cmd.Dir = tree
	cmd.WaitDelay = generatorKillGrace
	var out tailBuf
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	said := landCheckSaid(string(out.b))
	switch {
	case ctx.Err() != nil:
		return landUnanswered, fmt.Sprintf("no answer in %s, signalled (%s)", limit, orElse(said, "no output"))
	case errors.Is(err, exec.ErrWaitDelay):
		// The check exited and a DESCENDANT it started outlived it holding the
		// output pipe, so the grace closed the pipe on the way out: what was
		// read is short by an unknown amount. generatorKillGrace's own header
		// is why that grace is here at all, and this is the arm that proves it
		// is not held on principle — this repo's check is a shell script
		// running `go`, so it has grandchildren by construction. Unanswered
		// rather than the exit status it also has: a check that leaks a child
		// into the launcher is not a repo whose verdict this should act on.
		return landUnanswered, fmt.Sprintf("it exited and a descendant of it outlived the %s reaping grace, so its answer could not be read whole (%s)", generatorKillGrace, orElse(said, "no output"))
	case err == nil:
		return landGreen, said
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		return landUnanswered, fmt.Sprintf("it would not run: %s", oneLine(err.Error()))
	}
	if ee.ProcessState != nil && ee.ProcessState.ExitCode() < 0 {
		// Signalled by something that is not this deadline — the operator, an
		// OOM killer. A child that did not choose its exit status has not
		// judged anything (ranger-base-zfza8's doctrine, one child over).
		return landUnanswered, fmt.Sprintf("it was signalled (%s): %s", ee.ProcessState, orElse(said, "no output"))
	}
	return landRed, orElse(said, fmt.Sprintf("exit %d with no output", ee.ExitCode()))
}

// orElse is the "and it said nothing" fallback these sentences need in three
// places: a check that fails with no output at all must still produce a cause
// a bead can carry.
func orElse(said, fallback string) string {
	if said == "" {
		return fallback
	}
	return said
}

// revTreeID is the key D4 records against: the tree id a commit names, "" when
// it does not resolve.
func revTreeID(repo, rev string) string {
	if rev == "" {
		return ""
	}
	out, err := git(repo, "rev-parse", "--verify", "--quiet", rev+"^{tree}")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// recordLandCheck writes the verdict for one tree. Best effort by contract,
// for recordProbedBase's reason: a config write that fails leaves the next
// replay probing the base for real, which is the behaviour before any record
// existed — the old cost is the failure mode, so it owes nobody a sentence.
func recordLandCheck(repo, tree string, v landVerdict) error {
	if tree == "" || v == landUnanswered {
		return nil
	}
	_, err := git(repo, "config", landCheckKey, tree+" "+v.String())
	return err
}

// landCheckRecorded reads it back. A record in any other shape is no record:
// this is one line of git config a human can edit, and a half-parsed verdict
// is the one thing that must not be read as green.
func landCheckRecorded(repo string) (string, landVerdict) {
	out, err := git(repo, "config", "--get", landCheckKey)
	if err != nil {
		return "", landUnanswered
	}
	f := strings.Fields(out)
	if len(f) != 2 {
		return "", landUnanswered
	}
	switch f[1] {
	case landGreen.String():
		return f[0], landGreen
	case landRed.String():
		return f[0], landRed
	}
	return "", landUnanswered
}

// baseLandCheck is "was the base green?" — D4's lookup, and the one run that
// pays for it when the lookup misses.
//
// THE LOOKUP IS THE COMMON CASE AND COSTS NOTHING: a base that came through
// this gate green had its tree recorded by the landing that put it there. Only
// a RED replay over a base nobody judged pays the second run, which is why
// this is not asked unless the first run came back red.
//
// THE RUN DETACHES THE SESSION TREE, which is the only tree the launcher may
// write: `wasAt` checked out detached, the base's check over it, then back to
// the branch. Not the operator's checkout — that is the one store in this path
// the launcher lock does not govern (ADR 0011 §1) — and not a scratch clone,
// which would be a second copy of the repo per landing for a case this rare.
//
// It is also the one arm here that writes the session tree for a reason the
// grace clock reads (ADR 0058 fact 4). It is bounded by what reaches it: a red
// replay, over a base whose tree is not on record, and the pass that follows
// skips the whole probe on the record standingMergeBlock reads.
//
// EVERY ARM THAT CANNOT FINISH ANSWERS landUnanswered, because the caller's
// only use for a non-green base is to land — so an unreadable answer costs a
// ⚠ and never a block.
func baseLandCheck(t *SessionTree, wasAt string) (landVerdict, string) {
	baseTree := revTreeID(t.Repo, wasAt)
	if baseTree == "" {
		return landUnanswered, fmt.Sprintf("git could not read %s's own tree", abbrevSHA(wasAt))
	}
	if tree, v := landCheckRecorded(t.Repo); tree == baseTree && v != landUnanswered {
		return v, "on record from an earlier landing, not re-run"
	}
	if cur, err := git(t.Path, "symbolic-ref", "--short", "HEAD"); err != nil || cur != t.Branch {
		// Nothing detaches a tree whose HEAD is not where this function would
		// put it back. A container-tier session works detached on purpose
		// (PrepareSessionHead) and spliceDetachedWork has already run by now;
		// either way, restoring is the half that must be guaranteed.
		return landUnanswered, fmt.Sprintf("%s is not on %s, so %s could not be checked out there to judge it", AbbrevHome(t.Path), t.Branch, abbrevSHA(wasAt))
	}
	if _, err := git(t.Path, "checkout", "--detach", wasAt); err != nil {
		return landUnanswered, fmt.Sprintf("%s could not be checked out in %s (%s)", abbrevSHA(wasAt), AbbrevHome(t.Path), gitSaid(err))
	}
	v, said := runLandCheck(t.Repo, t.Path)
	// `--force`, and the tree it discards is not the persona's: the dirt check
	// ran before the replay, so anything in the way was written by the check
	// that just ran. A restore that fails anyway leaves the tree detached, and
	// the caller's sentence says so and says the command that fixes it — there
	// is nothing to be gained by a second attempt at the same command.
	if _, err := git(t.Path, "checkout", "--force", t.Branch); err != nil {
		return landUnanswered, fmt.Sprintf("%s was judged, and %s is now DETACHED at %s because git would not put it back on %s (%s) — `git -C %s checkout %s` restores it",
			abbrevSHA(wasAt), AbbrevHome(t.Path), abbrevSHA(wasAt), t.Branch, gitSaid(err), t.Path, t.Branch)
	}
	_ = recordLandCheck(t.Repo, baseTree, v)
	return v, said
}

// landCheckGate is ADR 0087 at the one site it runs: immediately before the
// replay arm's `merge --ff-only`, on the exact tip that swap will take.
//
// blocked is the one outcome that refuses the landing, and by then the branch
// has already been reset to `pre`; `said` is the check's output, which the
// CALLER turns into the refusal sentence (worktree.go's landCheckRefusal).
// That split is not tidiness: every merge refusal in this package is produced
// in worktree.go, because mergereasonfloor_qa_test.go's census derives the
// class of refusals by parsing that one file and fails closed on a sentence
// written anywhere else — a pin that cost eight undoored pins elsewhere
// (ranger-base-sx2dq) when a derivation was keyed on one place while the tree
// carried a second.
//
// warning != "" is a landing that was not judged, which the pass prints and
// never files. All three zero is the ordinary answer: green, or no check
// shipped.
//
// WHY THIS SITE IS CORRECT BY CONSTRUCTION AND NOT MERELY NARROW (D2).
// `merge --ff-only` is a compare-and-swap on the base's ref. If it succeeds
// the base is still the `wasAt` the replay was built on, so the tree that
// lands is the tree that was judged. If the base moved during the check the
// swap fails, and the existing bounded loop replays and judges again.
//
// WHAT IS JUDGED IS THE WORKING TREE, and it is the tip's tree because the
// session tree is clean by the time anything reaches here — the dirt check ran
// before the replay, and the replay and the generated-index step both COMMIT
// what they write. A check that writes into the tree it is judging breaks that
// equality and is out of contract (D1: it is handed the tree to judge, not a
// scratch directory); what it would cost is a record keyed on a tree id that
// is not quite the bytes that were read.
func landCheckGate(t *SessionTree, pre, wasAt string) (blocked bool, said, warning string) {
	if !landCheckShipped(t.Repo) {
		return false, "", ""
	}
	v, said := runLandCheck(t.Repo, t.Path)
	tip := revTreeID(t.Repo, t.Branch)
	switch v {
	case landGreen:
		_ = recordLandCheck(t.Repo, tip, landGreen)
		return false, "", ""
	case landUnanswered:
		return false, "", fmt.Sprintf("%s could not judge the replay of %s onto %s — %s — so it landed unjudged (ADR 0087 D3: a check that cannot answer never blocks; ci.yml is the backstop)",
			landCheckScript, t.Branch, t.Base, said)
	}
	// Red. The only question left is whose fault it is, and the only answer
	// that blocks is "this branch's": a base that was already red strands
	// nobody, and the fix for a red base comes through this same path.
	bv, bsaid := baseLandCheck(t, wasAt)
	if bv != landGreen {
		_ = recordLandCheck(t.Repo, tip, landRed)
		if bv == landRed {
			return false, "", fmt.Sprintf("%s was already red before this landing — %s fails on %s too (%s) — so %s landed unjudged, and the errors it adds are nobody's to attribute (ADR 0087 D3)",
				t.Base, landCheckScript, abbrevSHA(wasAt), bsaid, t.Branch)
		}
		return false, "", fmt.Sprintf("the replay of %s onto %s fails %s (%s), and whether %s was already failing it could not be established (%s) — landed unjudged, because only a green base this landing would turn red is worth blocking (ADR 0087 D3)",
			t.Branch, t.Base, landCheckScript, said, abbrevSHA(wasAt), bsaid)
	}
	// Red over green: the a2327230 shape, and the one case that blocks.
	if why := resetReplay(t, pre); why != "" {
		// A BLOCK THAT CANNOT BE REALIZED MUST NOT BE REPORTED AS ONE. The
		// branch is on the replayed tip, so the next pass would take it by
		// plain fast-forward — the arm this gate does not judge — and land the
		// same red tip with nothing said at all. Landing it HERE lands the
		// identical tree and says so on the pass that did it.
		_ = recordLandCheck(t.Repo, tip, landRed)
		return false, "", fmt.Sprintf("the replay of %s onto %s fails %s (%s) where %s passes it, and the replay could not be undone (%s) — so it landed unjudged rather than leaving a replayed tip for the unjudged fast-forward to take (ADR 0087 D5)",
			t.Branch, t.Base, landCheckScript, said, t.Base, why)
	}
	return true, said, ""
}

// resetReplay is D5: a block leaves the branch where the seat left it.
// `rebase --abort` already gives that guarantee for a conflict, and a replay
// that SUCCEEDED has rewritten the branch, so the undo is an explicit reset to
// the pre-replay tip. "" is "the branch is back"; anything else is why it is
// not, which the caller lands with rather than reporting a block it did not
// make.
//
// The tree is clean (the dirt check ran before the replay), it is not the
// shared checkout, and what the reset discards is the launcher's own work: the
// replayed commits and any generated-index commit made on the far side of them
// (refreshGeneratedIndex). That is why AGENTS.md's "never reset" is about the
// shared checkout and not about this call.
//
// THE HEAD IS ASKED FIRST, and it is not a formality: `reset --hard` moves
// whatever HEAD names, so over a DETACHED session tree it would move HEAD and
// leave the branch on the replayed tip — the undo silently not happening, which
// is the one outcome worse than not attempting it.
func resetReplay(t *SessionTree, pre string) string {
	if pre == "" {
		return "the tip before the replay was never read"
	}
	if cur, err := git(t.Path, "symbolic-ref", "--short", "HEAD"); err != nil || cur != t.Branch {
		return fmt.Sprintf("%s is not on %s, so a reset would not move the branch", AbbrevHome(t.Path), t.Branch)
	}
	if _, err := git(t.Path, "reset", "--hard", pre); err != nil {
		return gitSaid(err)
	}
	if head, err := git(t.Path, "rev-parse", "HEAD"); err != nil || head != pre {
		return fmt.Sprintf("%s is at %s after the reset, not %s", AbbrevHome(t.Path), abbrevSHA(head), abbrevSHA(pre))
	}
	return ""
}
