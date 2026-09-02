package posse

// ranger-base-nlya — the ADR 0013 six-stage dispatch contract, walked end
// to end on a real runtime by the production dispatch code.
//
// Every other live pin in this package needs a human to stand up a pane
// first and point an env var at it (livesplash_test.go's header says so in
// as many words). That makes them pins on ONE stage of a session somebody
// else launched. Nothing in the repo launched a grok or a codex session,
// gave it work, and asked whether it came back — so "does a dispatched
// session on this runtime still work" was proven by nobody, and both
// silent precedents (ranger-base-z6n's narrow-pane splash rule,
// ranger-base-ocfh's blind autoUpdate sed) are of the class where the
// fixture check stayed green while the live thing was broken.
//
// This gate launches the session itself. It is opt-in because it SPENDS A
// REAL TURN on the runtime under test:
//
//	RHQ_LIVE_RUNTIME=codex go test ./internal/rhq -run TestLiveRuntimeContractWalk -v -timeout 30m
//	RHQ_LIVE_RUNTIME=grok  go test ./internal/rhq -run TestLiveRuntimeContractWalk -v -timeout 30m
//	RHQ_LIVE_RUNTIME=claude ...   # the ADR 0017 baseline, for comparison
//
// Run it before switching a lane back onto a runtime, and after any
// runtime version bump. It is never part of `make test`.
//
// What it drives is the production path, not a re-implementation:
// Dispatcher.fire → launchSession → CreateSession(Worktree, PromptFile) →
// awaitAgent → claim → Dispatcher.gather. If dispatch stops working, this
// stops working, which is the only property that makes a walk worth the
// tokens.
//
// Verdict vocabulary is ADR 0017 §2 as the architect scoped it onto the
// bead:
// MEASURED WORKING / MEASURED BROKEN / DECLARED DIFFERENCE /
// UNKNOWN(failing). The point of the last two is that they are NOT
// failures of the runtime: codex's `record: untrusted` is a declared
// difference (ranger-base-0fb measured it 3/3), and an exhausted account
// is UNKNOWN — a fact about the bill, not about the CLI. Reading one as
// the other is what cost the shop a morning on 2026-08-26.
//
// Fixture decisions, which were the bead's two open questions:
//
//   - WHICH PID. A throwaway one written into the walk's own scratch
//     RHQ_HOME. No operator PID is borrowed and none is promoted, so ADR
//     0015's promotion gate is not in the way: nothing in ~/.config/rhq is
//     read or written by the walk.
//   - SCRATCH DB OR THE LIVE QUEUE. Neither, quite: a COPY of a real
//     .beads, reached through a real `<repo>/.beads/redirect`. That is a
//     real bd store with the real redirect in the path — the thing a plain
//     scratch db would not exercise and where codex broke before — and it
//     is not the fleet's queue, so a walk cannot dirty it. The copy exists
//     because `bd init` is denied to fleet personas and a test may not
//     launder that deny by shelling out to it.
//
// What a run leaves behind, named rather than hidden: config `worktrees:`
// is pointed under ~/.posse/qa-runtimewalk/ and removed, never at the live
// ~/.posse/worktrees (ranger-base-gvrh: a test cut a real worktree there);
// and a walk on CLAUDE adds one `projects[<the fixture's temp dir>]` entry
// to the operator's ~/.claude.json, because SeedClaudeTrust is what makes
// a fresh directory promptable and diverting it to a scratch file is how
// the baseline arm turns into a trust dialog nobody answers. codex and
// grok do not reach that seed at all.
//
// Measured 2026-08-28 on this box — see the bead for the sheets.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// The four verdicts. Spelled here once so a row cannot be scored with a
// word that is not in ADR 0017 §2's vocabulary.
const (
	walkWorking  = "MEASURED WORKING"
	walkBroken   = "MEASURED BROKEN"
	walkDeclared = "DECLARED DIFFERENCE"
	walkUnknown  = "UNKNOWN(failing)"
)

// walkPing is the account probe's whole prompt: the cheapest turn the
// runtime will sell, asked before the walk spends a real one.
const walkPing = "Reply with exactly: OK"

type walkRow struct{ stage, verdict, evidence string }

// walkSheet is the parity matrix this gate produces. It is filled as the
// walk goes and printed by a cleanup, so a stage that dies still leaves
// the rows measured before it — a sheet that only prints on success would
// hide the one run anybody cares about.
type walkSheet struct {
	t    *testing.T
	name string
	mu   sync.Mutex
	rows []walkRow
}

func newWalkSheet(t *testing.T, name string) *walkSheet {
	s := &walkSheet{t: t, name: name}
	t.Cleanup(s.report)
	return s
}

func (s *walkSheet) score(stage, verdict, format string, a ...any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows = append(s.rows, walkRow{stage, verdict, fmt.Sprintf(format, a...)})
}

// report prints the sheet and turns it into the test's verdict. A row
// scored MEASURED BROKEN fails; UNKNOWN(failing) fails too, per ADR 0017
// §2 ("the parity matrices score it as a failing cell") — but the two are
// distinguishable in the output, which is the whole presentation rule.
// DECLARED DIFFERENCE never fails.
func (s *walkSheet) report() {
	s.mu.Lock()
	defer s.mu.Unlock()
	var b strings.Builder
	fmt.Fprintf(&b, "\nADR 0013 contract walk — runtime %s (verdicts: ADR 0017 §2)\n", s.name)
	for _, r := range s.rows {
		fmt.Fprintf(&b, "  %-12s %-19s %s\n", r.stage, r.verdict, r.evidence)
	}
	s.t.Log(b.String())
	for _, r := range s.rows {
		if r.verdict == walkBroken || r.verdict == walkUnknown {
			s.t.Errorf("%s: %s — %s", r.stage, r.verdict, r.evidence)
		}
	}
}

func TestLiveRuntimeContractWalk(t *testing.T) {
	name := os.Getenv("RHQ_LIVE_RUNTIME")
	if name == "" {
		t.Skip("set RHQ_LIVE_RUNTIME=grok|codex|claude — this LAUNCHES a session and SPENDS a real turn on that runtime (see the file comment)")
	}
	sheet := newWalkSheet(t, name)

	// ── account, first ────────────────────────────────────────────────
	// Before anything is launched or spent, ask the account whether it
	// would serve a turn at all. The bead's third open question: an
	// exhausted allotment must fail LOUDLY and DISTINGUISHABLY from a
	// broken runtime. It is its own cell (the architect's scoping,
	// ranger-base-il14), it
	// is scored UNKNOWN(failing) rather than MEASURED BROKEN, and the walk
	// stops here rather than reporting six broken stages that are really
	// one unpaid bill.
	acct := probeAccount(t, name)
	switch {
	case acct.exhausted:
		sheet.score("account-live", walkUnknown, "the ACCOUNT is exhausted, not the runtime: %s", acct.line)
		t.Fatalf("UNKNOWN(failing): %s's account will not serve a turn — %s\n"+
			"  this is a fact about the allotment, NOT a runtime defect: nothing downstream was measured.", name, acct.line)
	case !acct.alive:
		sheet.score("account-live", walkUnknown, "the account probe did not come back with a turn: %s", acct.line)
		t.Fatalf("UNKNOWN(failing): %s's one-turn probe did not answer — %s\n"+
			"  neither the account nor the runtime is cleared; nothing downstream was measured.", name, acct.line)
	}
	sheet.score("account-live", walkWorking, "one headless turn served: %s", acct.line)

	// ── preflight ─────────────────────────────────────────────────────
	// The architect's first addition: run the declared grid before the walk
	// and require every interstitial probe to report silenced. A NOT SILENCED
	// probe is a KNOWN-failing launch — the operator has not clicked the
	// thing — and reporting that as a runtime finding would be this gate
	// telling the shop a lie on its first run.
	// On a throwaway App, and before the fixture: a runtime the operator
	// has not silenced should not cost a herdr server and a copied bd
	// store to refuse. The grid is a declaration plus this machine's
	// state; it needs no session.
	pre := NewAppAt(t.TempDir())
	rt, err := pre.LoadRuntime(name)
	if err != nil {
		t.Fatalf("LoadRuntime(%s): %v", name, err)
	}
	var grid strings.Builder
	clean := pre.RuntimeCheck(rt, NewHerdr(), &grid)
	t.Logf("posse runtime check %s:\n%s", name, grid.String())
	switch {
	case !clean:
		sheet.score("preflight", walkUnknown, "`posse runtime check %s` has a BLOCKING gap — this box cannot launch it", name)
		t.Fatalf("UNKNOWN(failing): preflight is not clean for %s; the grid above names the gap. Not a walk finding.", name)
	case strings.Contains(grid.String(), "NOT SILENCED"):
		sheet.score("preflight", walkUnknown, "an interstitial probe reports NOT SILENCED — a known-failing launch, not a walk finding")
		t.Fatalf("UNKNOWN(failing): %s has an unsilenced first-run screen (grid above). The OPERATOR silences it; the walk measures nothing until they do.", name)
	}
	sheet.score("preflight", walkWorking, "grid clean, every declared interstitial probe reports silenced")

	// ── the walk ──────────────────────────────────────────────────────
	daemons := walkDaemons()
	a, b, is, persona, beadsDir := walkFixture(t, name)
	out := &strings.Builder{}
	d := NewDispatcher(a, b, out)
	// The ledger is not this walk's subject and reading the operator's
	// live one makes a test red per hour (ranger-base-rp2y). Nothing here
	// spends on the claude meter.
	d.Spend = func(time.Time) *CostReport { return &CostReport{} }
	d.StartupWait = walkDuration(t, "RHQ_LIVE_WALK_STARTUP", 120*time.Second)
	d.Poll = 2 * time.Second
	// One --wait leg. The walk's whole task is a comment and a close, so a
	// leg this short is a check-in, not a deadline (gather re-waits).
	d.PromptWaitMS = int(walkDuration(t, "RHQ_LIVE_WALK_LEG", 5*time.Minute) / time.Millisecond)
	d.WaitCeiling = walkDuration(t, "RHQ_LIVE_WALK_CEILING", 15*time.Minute)

	session := SessionForBead(persona, is.Dir, is.ID)
	t.Cleanup(func() {
		if err := b.KillSession(session); err != nil {
			t.Logf("teardown: kill %s: %v", session, err)
		}
	})

	// launch + promptable, in one production call: fire assembles the ADR
	// 0005 work prompt, writes it, puts it on the launch line for a
	// `prompt: argv` runtime, waits for a herdr state it can NAME, and
	// claims the bead.
	fired := time.Now()
	p, err := d.fire(is, persona, session, name, "standard", "walk", false, nil)
	t.Logf("fire: %s\n%s", time.Since(fired).Round(time.Second), out.String())
	if err != nil {
		sheet.score("launch", walkBroken, "dispatch could not launch a %s session: %v", name, err)
		t.Fatalf("fire: %v\n%s", err, out.String())
	}
	sheet.score("launch", walkWorking, "session %s created and herdr named an agent in pane %s after %s",
		session, p.target, time.Since(fired).Round(time.Second))

	// promptable. The two delivery methods have different observables and
	// only one of them leaves a file: on `prompt: argv` the prompt rode in
	// on the launch line (WorkPromptFile is what `$(cat …)` read), and on
	// `prompt: typed` there is no file at all — the observable is that
	// awaitAgent reached a screen herdr could NAME before anything was
	// typed at it. Asserting the file on both is how this gate scored the
	// claude baseline MEASURED BROKEN on its first run for a property
	// claude never claimed.
	promptFile := a.WorkPromptFile(session)
	body, readErr := os.ReadFile(promptFile)
	switch {
	case p.unseen:
		sheet.score("promptable", walkUnknown, "prompt delivered but herdr never recognized a screen in %s within %s", session, d.StartupWait)
	case rt.PromptMode() == PromptArgv && !p.delivered:
		sheet.score("promptable", walkBroken, "%s declares prompt: argv and dispatch still had to type at a screen", name)
	case rt.PromptMode() == PromptArgv && readErr != nil:
		sheet.score("promptable", walkBroken, "no work prompt at %s: %v", promptFile, readErr)
	case rt.PromptMode() == PromptArgv && !strings.Contains(string(body), is.ID):
		sheet.score("promptable", walkBroken, "the work prompt at %s does not name the bead it was assembled for", promptFile)
	case rt.PromptMode() == PromptArgv:
		sheet.score("promptable", walkWorking, "%d-byte work prompt delivered on the launch line as argv; no screen was the delivery channel", len(body))
	case p.delivered:
		sheet.score("promptable", walkBroken, "%s declares prompt: typed and the prompt arrived some other way", name)
	default:
		sheet.score("promptable", walkWorking, "typed into pane %s after awaitAgent reached a named screen (no work-prompt file, by design)", p.target)
	}

	// work + settle. gather blocks on the same wait dispatch uses, so the
	// states herdr passes through have to be sampled beside it.
	states := newStateWatch(t, b, p.target)
	inFlight, gerr := d.gather(p)
	seen := states.stop()
	t.Logf("gather: inFlight=%v err=%v states=%v\n%s", inFlight, gerr, seen, out.String())

	switch {
	case gerr != nil:
		sheet.score("work", walkBroken, "gather failed: %v", gerr)
	case !containsString(seen, "working"):
		// Not a pass by another name: the ADR's observable for this stage
		// is herdr `working`, and a runtime whose turn is invisible to
		// herdr is a runtime dispatch cannot pace.
		sheet.score("work", walkUnknown, "herdr never showed a `working` state while the turn ran (sampled: %s)", strings.Join(seen, " "))
	case inFlight:
		sheet.score("work", walkUnknown, "the turn was still in flight when the ceiling ran out (sampled: %s)", strings.Join(seen, " "))
	default:
		sheet.score("work", walkWorking, "herdr saw `working`, then a settled state (sampled: %s)", strings.Join(seen, " "))
	}

	// record. The store of record is the bead, and this is the stage codex
	// was measured to skip. A miss on a `record: untrusted` runtime is the
	// DECLARED difference; on a `record: trusted` one it is news.
	after, showErr := d.Bd.Show(is.Dir, is.ID)
	paneTail := ""
	if showErr != nil || after.Status != "closed" {
		// A record miss is the one outcome nobody can act on from a count.
		// The pane is the evidence — what the session actually said before
		// it went idle — and it is gone the moment teardown runs.
		if tail, err := b.H.PaneRead(p.target, 80); err == nil {
			paneTail = tail
			t.Logf("pane %s, last 80 lines at the record miss:\n%s", p.target, tail)
		}
	}
	comments, _ := d.Bd.Comments(is.Dir, is.ID)
	// Printed, not just counted: "1 comment" is a number, and whether the
	// session reached the store of record at all is a sentence.
	for _, c := range comments {
		t.Logf("comment by %s: %s", c.Author, firstLines(c.Text, 4))
	}
	switch auth := walkAuthFailure(paneTail); {
	// The account cell again, and this is where it actually bites: the
	// headless probe authenticates from the shell's own environment and a
	// LAUNCHED session does not always inherit the same credentials. A
	// session that could not log in did no work, so nothing about the
	// runtime was measured — UNKNOWN, never MEASURED BROKEN.
	case auth != "":
		sheet.score("record", walkUnknown, "the SESSION could not authenticate, so no work ran: %s", auth)
	case showErr != nil:
		sheet.score("record", walkUnknown, "bd could not say what %s is: %v", is.ID, showErr)
	case after.Status == "closed" && len(comments) > 0:
		sheet.score("record", walkWorking, "%s closed with %d comment(s) through the .beads redirect at %s",
			is.ID, len(comments), beadsDir)
	case after.Status == "closed":
		sheet.score("record", walkWorking, "%s closed (no comment) through the .beads redirect", is.ID)
	case rt.RecordTrust() == RecordUntrusted:
		sheet.score("record", walkDeclared, "%s settled with %s still %q and %d comment(s) — `record: untrusted` is the declared degrade",
			name, is.ID, after.Status, len(comments))
	default:
		sheet.score("record", walkBroken, "%s is `record: trusted` and left %s %q with %d comment(s)",
			name, is.ID, after.Status, len(comments))
	}

	// settle. Not "the pane went quiet" — a MATCHED rule, which is the
	// difference between a settled agent and the idle fallback over a
	// screen herdr cannot read.
	if ex, err := b.H.AgentExplain(p.target); err != nil {
		sheet.score("settle", walkUnknown, "herdr could not explain %s after the turn: %v", p.target, err)
	} else if !ex.Seen() || !containsString([]string{"idle", "done", "blocked"}, ex.State) {
		sheet.score("settle", walkBroken, "settled state %q came from rule %q (seen=%v) — not a matched settle",
			ex.State, ex.Rule.ID, ex.Seen())
	} else {
		sheet.score("settle", walkWorking, "state %q from matched rule %q", ex.State, ex.Rule.ID)
	}

	// account. What the pass could say about the money. Not counted is a
	// DECLARED difference (ADR 0003 §4), not a break — and the cell exists
	// so that a runtime that GAINS an adapter shows up here as a change.
	// Three states since ranger-base-0lg6: priced, read-but-unpriced, and
	// nothing reading it at all.
	switch {
	case rt.CostPriced():
		sheet.score("account", walkWorking, "a cost adapter prices %s: %s", name, rt.CostReading())
	case rt.CostRead():
		sheet.score("account", walkDeclared, "UNPRICED — %s reads %s and prices none of it; uncounted_cap_%s is the only brake", rt.CostReading(), name, name)
	default:
		sheet.score("account", walkDeclared, "UNCOUNTED — no cost adapter reads %s; uncounted_cap_%s is the only brake", name, name)
	}

	// teardown. ranger-base-42mv: worktreelive_test.go leaked two bd
	// daemons on 2026-08-25. A gate that leaks its own daemons is not a
	// gate — and one that blames the fleet's daemons on itself is not one
	// either, so the check looks in two places and attributes by cwd.
	verdict, evidence := walkDaemonVerdict(t, beadsDir, daemons)
	sheet.score("teardown", verdict, "%s", evidence)
}

// walkDaemonVerdict answers whether the walk left a bd daemon behind on
// its own fixture store, and kills one if it did.
func walkDaemonVerdict(t *testing.T, beadsDir string, before map[string]bool) (string, string) {
	t.Helper()
	// Two ways to look, because either alone is blind: the fixture store's
	// own pidfile (precise, and empty if bd wrote the pid somewhere else)
	// and the box's bd daemon set (complete, and shared — so a new pid is
	// only THIS walk's if its cwd is the fixture's own store).
	//
	// Classification by cwd, and by cwd alone, is what keeps the verdict
	// honest on a box the whole fleet is working on: another session's
	// daemon appearing mid-walk is not this walk's leak, and calling it
	// one would make the cell cry wolf until nobody read it
	// (ranger-base-42mv's own rule).
	if verdict, evidence, ours := walkClassifyDaemons(walkNewDaemons(before), walkProcCwd, beadsDir); verdict != "" {
		if ours != "" {
			_ = walkSignal(ours)
		}
		return verdict, evidence
	}
	pidFile := filepath.Join(beadsDir, "daemon.pid")
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		return walkWorking, "no bd daemon pidfile under the fixture store"
	}
	pid, convErr := strconv.Atoi(strings.TrimSpace(string(raw)))
	if convErr != nil {
		return walkWorking, fmt.Sprintf("%s holds no pid (%q)", pidFile, strings.TrimSpace(string(raw)))
	}
	proc, err := os.FindProcess(pid)
	if err != nil || proc.Signal(syscall.Signal(0)) != nil {
		return walkWorking, fmt.Sprintf("%s names pid %d, which is gone", pidFile, pid)
	}
	_ = proc.Signal(syscall.SIGTERM)
	return walkBroken, fmt.Sprintf("the walk left bd daemon %d alive on its own store %s (ranger-base-42mv) — SIGTERM sent", pid, beadsDir)
}

// walkDaemons is the set of bd daemon pids alive right now. Compared, not
// counted: the operator's own daemons are in it and are none of the walk's
// business (ranger-base-42mv says classify, never `bd daemon stop-all`).
func walkDaemons() map[string]bool {
	out, err := exec.Command("pgrep", "-f", "bd daemon").Output()
	if err != nil {
		return nil
	}
	set := map[string]bool{}
	for _, ln := range strings.Fields(string(out)) {
		set[ln] = true
	}
	return set
}

// walkClassifyDaemons decides whether any bd daemon that appeared during
// the walk is the WALK'S. It returns ("", "", "") when none is, the pid to
// SIGTERM when one is, and takes cwdOf as an argument so the three branches
// can be pinned without a daemon.
func walkClassifyDaemons(fresh []string, cwdOf func(string) string, beadsDir string) (verdict, evidence, ours string) {
	for _, pid := range fresh {
		switch cwd := cwdOf(pid); {
		case cwd == "":
			// Never "ok": a daemon nobody can attribute is the state
			// ranger-base-tdwy insists is reported, not rounded down.
			return walkUnknown, fmt.Sprintf("a bd daemon (pid %s) appeared during the walk and its cwd could not be read — whose it is is UNKNOWN, not ok", pid), ""
		case strings.HasPrefix(cwd, beadsDir):
			return walkBroken, fmt.Sprintf("the walk left bd daemon %s on its own store %s (ranger-base-42mv) — SIGTERM sent", pid, cwd), pid
		}
	}
	return "", "", ""
}

// walkProcCwd is one process's working directory, or "" when it cannot be
// read. `lsof -d cwd` is the only portable-enough answer on macOS.
func walkProcCwd(pid string) string {
	out, err := exec.Command("lsof", "-a", "-p", pid, "-d", "cwd", "-Fn").Output()
	if err != nil {
		return ""
	}
	for _, ln := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(ln, "n") {
			return strings.TrimPrefix(ln, "n")
		}
	}
	return ""
}

// walkSignal SIGTERMs a pid the walk has already established is its own.
func walkSignal(pid string) error {
	n, err := strconv.Atoi(pid)
	if err != nil {
		return err
	}
	proc, err := os.FindProcess(n)
	if err != nil {
		return err
	}
	return proc.Signal(syscall.SIGTERM)
}

func walkNewDaemons(before map[string]bool) []string {
	var grew []string
	for pid := range walkDaemons() {
		if !before[pid] {
			grew = append(grew, pid)
		}
	}
	return grew
}

// stateWatch samples herdr's view of one pane while something else blocks
// on it. Absence of a state proves only that it was not sampled, which is
// why the poll is short and the sampled list is printed with the verdict.
type stateWatch struct {
	done chan struct{}
	wg   sync.WaitGroup
	mu   sync.Mutex
	seen []string
}

func newStateWatch(t *testing.T, b *HerdrBackend, pane string) *stateWatch {
	w := &stateWatch{done: make(chan struct{})}
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		for {
			select {
			case <-w.done:
				return
			case <-time.After(2 * time.Second):
			}
			ex, err := b.H.AgentExplain(pane)
			if err != nil {
				continue
			}
			w.mu.Lock()
			if !containsString(w.seen, ex.State) {
				w.seen = append(w.seen, ex.State)
			}
			w.mu.Unlock()
		}
	}()
	return w
}

func (w *stateWatch) stop() []string {
	close(w.done)
	w.wg.Wait()
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.seen...)
}

// accountProbe is what the runtime's own headless one-shot said.
type accountProbe struct {
	alive     bool
	exhausted bool
	line      string
}

// walkExhausted are the words a provider uses when the answer is "you have
// no allotment left" rather than "this CLI is broken". Measured 2026-08-28:
// grok says `API error (status 402 Payment Required): Grok Build usage
// balance exhausted` on stderr and exits 1.
var walkExhausted = []string{
	"payment required", "402", "usage balance exhausted", "quota",
	"rate limit", "rate_limit", "insufficient", "out of credit",
	"usage limit", "billing", "exhausted",
}

// probeAccount buys the cheapest turn the runtime sells and classifies the
// answer. It runs headless — a different product surface from the
// interactive session the walk launches — so it proves the ACCOUNT, never
// the dispatch contract. That separation is the point: it is the cell that
// keeps an unpaid bill from being read as a broken runtime.
func probeAccount(t *testing.T, name string) accountProbe {
	t.Helper()
	dir := t.TempDir()
	var argv []string
	switch name {
	case "grok":
		// A scratch leader socket: the operator's shared grok leader is
		// not this probe's to wake (ranger-base-xaev).
		argv = []string{"grok", "--leader-socket", filepath.Join(dir, "leader.sock"), "-p", walkPing}
	case "codex":
		argv = []string{"codex", "exec", "--skip-git-repo-check", "--sandbox", "read-only", walkPing}
	case "claude":
		argv = []string{"claude", "-p", walkPing}
	default:
		return accountProbe{line: "no headless one-shot is known for runtime " + name}
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	// Not /dev/null by accident: codex reads a piped stdin as an extra
	// `<stdin>` block appended to the prompt.
	if f, err := os.Open(os.DevNull); err == nil {
		defer f.Close()
		cmd.Stdin = f
	}
	out, err := cmd.CombinedOutput()
	return classifyAccount(string(out), err)
}

// classifyAccount turns one headless probe into a cell.
//
// A SERVED TURN IS CHECKED FIRST, and that order is the whole function: a
// model that answers the ping and happens to say "quota" or "rate limit"
// in its own prose is a live account, and scanning for those words before
// looking at the answer would report the healthy case as an unpaid bill —
// the same confusion in the opposite direction, and this cell exists to
// end it. Only a probe that did NOT come back with an answer is read for
// exhaustion.
func classifyAccount(out string, err error) accountProbe {
	text := strings.TrimSpace(out)
	line := strings.ReplaceAll(firstLines(text, 3), "\n", " · ")
	if err == nil && strings.Contains(text, "OK") {
		return accountProbe{alive: true, line: lastLine(text)}
	}
	low := strings.ToLower(text)
	for _, w := range walkExhausted {
		if strings.Contains(low, w) {
			return accountProbe{exhausted: true, line: line}
		}
	}
	if err != nil {
		return accountProbe{line: fmt.Sprintf("%v: %s", err, line)}
	}
	return accountProbe{line: "exit 0 but no answer in the output: " + line}
}

// lastLine is the answer end of a headless run: every CLI here prints its
// own banner first and the model's words last.
func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if t := strings.TrimSpace(lines[i]); t != "" {
			return t
		}
	}
	return ""
}

// walkAuthMarkers are what a pane shows when the CLI is running but the
// account will not let it work. Measured 2026-08-28: claude prints
// `Please run /login · API Error: 401 OAuth access token has expired.`
// and then settles idle, which is indistinguishable from a finished turn
// unless the pane is read.
var walkAuthMarkers = []string{
	"oauth access token has expired", "please run /login", "api error: 401",
	"authentication_error", "invalid api key", "credit balance is too low",
	"payment required", "usage balance exhausted",
}

// walkAuthFailure returns the pane line that says the session could not
// authenticate, or "" when nothing in the pane says so.
func walkAuthFailure(pane string) string {
	for _, ln := range strings.Split(pane, "\n") {
		low := strings.ToLower(ln)
		for _, m := range walkAuthMarkers {
			if strings.Contains(low, m) {
				return strings.TrimSpace(ln)
			}
		}
	}
	return ""
}

func firstLines(s string, n int) string {
	parts := strings.Split(s, "\n")
	if len(parts) > n {
		parts = parts[:n]
	}
	return strings.Join(parts, "\n")
}

// walkFixture builds everything the walk needs and nothing it does not: a
// scratch RHQ_HOME, a throwaway PID, a git repo whose `.beads/redirect`
// points at a COPY of a real bd store, and one throwaway bead in it whose
// whole job is to be commented on and closed.
func walkFixture(t *testing.T, runtime string) (*App, *HerdrBackend, RepoIssue, string, string) {
	t.Helper()
	home := t.TempDir()
	a := NewAppAt(home)
	// Hermetic for the same reason newTestBackend is: an unconfigured
	// lister reaches no network and the preflight then launches the tier
	// exactly as asked.
	a.ModelLister = &ModelLister{}

	// Session worktrees must live under $HOME (WorktreeRoot enforces it),
	// and $HOME must stay the operator's real one — the interstitial
	// probes read ~/.grok and ~/.codex, and the launched CLI authenticates
	// from there. So the root is moved instead of the home.
	wt := filepath.Join(os.Getenv("HOME"), ".posse", "qa-runtimewalk", fmt.Sprintf("%d-%d", os.Getpid(), time.Now().Unix()))
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(wt) })
	if err := os.WriteFile(a.ConfigPath, []byte("worktrees: "+wt+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// The env set, and it is not optional on claude: a fleet PID carries
	// `envs: [default]` and that set is where `CLAUDE_CODE_OAUTH_TOKEN`
	// comes from. A fixture without it launches a session that
	// authenticates from ~/.claude alone and came up `401 OAuth access
	// token has expired` twice on 2026-08-28 — a fixture gap the sheet
	// correctly refused to blame on the runtime, and one that would leave
	// the claude baseline unmeasurable. The dir is SYMLINKED, never
	// copied: the walk has no business making a second copy of a
	// credential (ADR 0019).
	envs := ""
	if dir := walkEnvsDir(); dir != "" {
		if err := os.Symlink(dir, filepath.Join(home, "envs")); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(dir, "default.env")); err == nil {
			envs = "envs: [default]\n"
		}
	}
	persona := "walkfixture"
	pid := "---\nname: " + persona + "\ndescription: QA runtime-walk fixture\nlabels: [qa-live-walk]\n" +
		"runtime: " + runtime + "\ntier: standard\n" + envs + "---\n" +
		"You are a QA liveness fixture. Do exactly what the work prompt asks and nothing else:\n" +
		"no code, no files, no commits. Add the comment it names to the bead, close the bead, stop.\n"
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.AgentsDir, persona+".md"), []byte(pid), 0o644); err != nil {
		t.Fatal(err)
	}

	repo, beadsDir := walkRepo(t)

	b := NewHerdrBackend(a)
	walkHerdrServer(t)

	id, err := b.Bd.Create(repo, BdNew{
		Title: "QA live runtime walk (throwaway, " + runtime + ")",
		Description: "This bead exists to be closed by a dispatched session, and for no other reason.\n\n" +
			"There is NO code work here. Do exactly this and stop:\n" +
			"  1. bd comments add <this id> \"runtime walk: took the prompt, reached the store of record\"\n" +
			"  2. bd close <this id>\n\n" +
			"Do not edit any file, do not commit, do not create other beads.",
		Labels:   []string{"qa-live-walk"},
		Priority: "3",
		Actor:    "qa",
	})
	if err != nil {
		t.Fatalf("could not create the throwaway bead in the fixture store: %v", err)
	}
	issue, err := b.Bd.Show(repo, id)
	if err != nil {
		t.Fatalf("bd show %s in the fixture store: %v", id, err)
	}
	t.Logf("fixture: bead %s in %s (store %s), persona %s, RHQ_HOME %s", id, repo, beadsDir, persona, home)
	return a, b, RepoIssue{BdIssue: issue, Dir: repo}, persona, beadsDir
}

// walkRepo is the fixture checkout: a real git repo with one commit, whose
// `.beads/redirect` names a copy of a real bd store.
//
// The copy, rather than `bd init`: init is denied to fleet personas, and a
// test that shelled out to it would be laundering that deny. The seed is
// whatever store this checkout itself resolves to, so the walk runs
// against the same bd schema the fleet is on rather than a hand-built one.
// RHQ_LIVE_WALK_SEED overrides it.
// walkEnvsDir is the operator's env-set directory — the one a dispatched
// session's PID names. Overridable, and "" when there is none to find, in
// which case the walk launches without one and says so through whatever
// the session then fails at.
func walkEnvsDir() string {
	if d := os.Getenv("RHQ_LIVE_WALK_ENVS"); d != "" {
		return d
	}
	for _, base := range []string{os.Getenv("RHQ_HOME"), filepath.Join(os.Getenv("HOME"), ".config", "rhq")} {
		if base == "" || base == "envs" {
			continue
		}
		d := filepath.Join(base, "envs")
		if st, err := os.Stat(d); err == nil && st.IsDir() {
			return d
		}
	}
	return ""
}

func walkRepo(t *testing.T) (string, string) {
	t.Helper()
	seed := os.Getenv("RHQ_LIVE_WALK_SEED")
	if seed == "" {
		seed = walkSeedStore(t)
	}
	root := t.TempDir()
	repo, store := filepath.Join(root, "repo"), filepath.Join(root, "beads")
	if err := os.MkdirAll(filepath.Join(repo, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatal(err)
	}
	// Copied by name, never wholesale: a socket cannot be copied and a
	// daemon pidfile copied out of a LIVE store would point the fixture at
	// the operator's daemon.
	for _, f := range []string{"beads.db", "config.yaml", "issues.jsonl", "metadata.json", "deleted.jsonl", "interactions.jsonl", ".gitignore"} {
		src, err := os.ReadFile(filepath.Join(seed, f))
		if err != nil {
			continue
		}
		if err := os.WriteFile(filepath.Join(store, f), src, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(store, "beads.db")); err != nil {
		t.Fatalf("the seed store %s has no beads.db — set RHQ_LIVE_WALK_SEED to a .beads directory", seed)
	}
	if err := os.WriteFile(filepath.Join(repo, ".beads", "redirect"), []byte(store), 0o600); err != nil {
		t.Fatal(err)
	}
	seedFile := filepath.Join(repo, "README.walk")
	if err := os.WriteFile(seedFile, []byte("qa runtime walk fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	walkGit(t, repo, "init", "-q", ".")
	walkGit(t, repo, "add", "README.walk")
	// `--` and an operand: the gate shim on a QA persona's PATH refuses
	// `git commit` without one (ADR 0022 / rangerhq-ojnw), and a fixture
	// that only builds for personas without that deny is a fixture that
	// measures nothing on the box it was written on.
	walkGit(t, repo, "commit", "-q", "-m", "fixture", "--", "README.walk")
	return repo, store
}

// walkSeedStore finds the bd store this checkout resolves to, following
// one `.beads/redirect` the way bd does.
func walkSeedStore(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		cand := filepath.Join(dir, ".beads")
		if st, err := os.Stat(cand); err == nil && st.IsDir() {
			if r, err := os.ReadFile(filepath.Join(cand, "redirect")); err == nil {
				return strings.TrimSpace(string(r))
			}
			return cand
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no .beads store above the working directory — set RHQ_LIVE_WALK_SEED")
		}
		dir = parent
	}
}

func walkGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
}

// walkHerdrServer starts the walk its OWN herdr server and points this
// process at it. The fleet server is not the place to launch a fixture
// session: a walk must not appear in the operator's board, and a teardown
// that reached the wrong server would close a real workspace.
func walkHerdrServer(t *testing.T) {
	t.Helper()
	sess := fmt.Sprintf("qa-walk-%d-%d", os.Getpid(), time.Now().Unix())
	cmd := exec.Command("herdr", "--session", sess, "server")
	// A pane's own herdr env disables nesting; a server started with it
	// inherited is a server that never comes up.
	cmd.Env = walkCleanEnv()
	// Started, never waited on, and never through CombinedOutput — both
	// cost this gate a run before it launched anything. `herdr … server`
	// is a FOREGROUND process that lives as long as the session, so Run()
	// blocks forever, and CombinedOutput blocks on a pipe it will never
	// see EOF on. The socket appearing is the readiness signal.
	log, err := os.Create(filepath.Join(t.TempDir(), "herdr-server.log"))
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		t.Fatalf("herdr --session %s server: %v", sess, err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		log.Close()
	})
	sock := filepath.Join(os.Getenv("HOME"), ".config", "herdr", "sessions", sess, "herdr.sock")
	deadline := time.Now().Add(20 * time.Second)
	for {
		if _, err := os.Stat(sock); err == nil {
			break
		}
		if time.Now().After(deadline) {
			out, _ := os.ReadFile(log.Name())
			t.Fatalf("herdr session %s never wrote %s\n%s", sess, sock, out)
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Setenv("HERDR_SOCKET_PATH", sock)
	// The rest of the pane's own herdr identity has to go for this process
	// too, not just for the server: herdr refuses to nest a workspace
	// created from inside a pane, and `go test` inherits those names when
	// the suite is run from one.
	for _, k := range []string{"HERDR_ENV", "HERDR_PANE_ID", "HERDR_WORKSPACE_ID", "HERDR_TAB_ID"} {
		if old, ok := os.LookupEnv(k); ok {
			k := k
			os.Unsetenv(k)
			t.Cleanup(func() { os.Setenv(k, old) })
		}
	}
	t.Cleanup(func() {
		for _, verb := range []string{"stop", "delete"} {
			c := exec.Command("herdr", "session", verb, sess)
			c.Env = walkCleanEnv()
			if out, err := c.CombinedOutput(); err != nil {
				t.Logf("teardown: herdr session %s %s: %v\n%s", verb, sess, err, out)
			}
		}
	})
	t.Logf("fixture: scratch herdr session %s at %s", sess, sock)
}

// walkPollution is what a session launched by this gate must NOT inherit
// from the shell that ran it. The scratch herdr server is the seam: every
// pane it opens inherits its environment, so a name left here reaches the
// runtime under test.
//
// Measured 2026-08-28, and each entry cost a run:
//
//   - HERDR_* — herdr refuses to nest a server or a workspace under a
//     pane's own identity.
//   - CLAUDECODE / CLAUDE_CODE_* — a claude launched inside another
//     claude's session env came up `401 OAuth access token has expired ·
//     Please run /login` and settled idle having done nothing. The walk
//     scored that MEASURED BROKEN and it was the harness's own env.
//   - RHQ_PERSONA / RHQ_GATES_DIR / RHQ_TOOLS_DENY / RHQ_SKILLS_DIR /
//     RHQ_HOME and kin — the LAUNCHING persona's gates, deny list, skills
//     and home. Inherited, the fixture session wears somebody else's
//     enforcement and the walk measures a runtime nobody configured.
var walkPollution = []string{
	"HERDR_", "CLAUDECODE", "CLAUDE_CODE_",
	"RHQ_HOME=", "RHQ_PERSONA", "RHQ_GATES_DIR=", "RHQ_TOOLS_DENY=",
	"RHQ_SKILLS_DIR=", "RHQ_RUNTIME=", "RHQ_TIER=", "RHQ_CAGE=",
}

func walkCleanEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		if walkPolluted(kv) {
			continue
		}
		if k, v, ok := strings.Cut(kv, "="); ok && k == "PATH" {
			kv = k + "=" + walkCleanPath(v)
		}
		env = append(env, kv)
	}
	return env
}

func walkPolluted(kv string) bool {
	for _, p := range walkPollution {
		if strings.HasPrefix(kv, p) {
			return true
		}
	}
	return false
}

// walkCleanPath drops the launching persona's gate shims. They are on PATH
// by construction inside a persona pane, and a fixture session that
// inherited them would be refused by another persona's deny rules — a
// refusal the sheet would report as the runtime failing to do its work.
func walkCleanPath(path string) string {
	var keep []string
	for _, e := range strings.Split(path, ":") {
		if e == "" || strings.Contains(e, "/state/gates/") {
			continue
		}
		keep = append(keep, e)
	}
	return strings.Join(keep, ":")
}

// walkDuration reads a duration override, so an operator on a slow account
// can widen one leg without editing the gate.
func walkDuration(t *testing.T, key string, def time.Duration) time.Duration {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		t.Fatalf("%s=%q is not a duration: %v", key, v, err)
	}
	return d
}
