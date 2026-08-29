package rhq

// ADR 0013 §4 reachability (ranger-base-hxhb, implemented on
// ranger-base-2nan). The record stage's cage half, pinned the way
// ranger-base-oyta measured it: an A/B over a RENDERED profile, executed
// under sandbox-exec, with the pre-fix profile as the control.
//
// The three tests the amendment asks for are the first three below:
//
//  1. the live rendered profile passes;
//  2. the oyta control — the same profile with the two store-of-record
//     grants deleted — fails, with a row naming the unreachable target;
//  3. the h15 pin — a trailing `(deny file-write* (subpath <store>))`
//     appended to a PASSING profile makes the check fail. This is the
//     regression the whole amendment exists for: ranger-base-h15's
//     carve-out, and every future narrowing of the writable set, reviews as
//     strictly safer and can silently re-break record. A check that read
//     SeatbeltWritable's return value would pass all three.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// rchFixture is the live shape: a session repo whose .beads holds a
// redirect, and the store of record in a second repo — the security
// persona's shape, the one ranger-base-rhw was measured on. Nothing is under TMPDIR or /tmp, so
// no blanket temp grant can make a probe pass by accident (sbRoot).
type rchFixture struct {
	a           *App
	work, store string
	ag          *AgentFile
	rt          *Runtime
}

func rchNew(t *testing.T) rchFixture {
	t.Helper()
	root := sbRoot(t)
	work := sbMkdir(t, filepath.Join(root, "work"))
	store := sbMkdir(t, filepath.Join(root, "store"))
	sbGitInit(t, work)
	sbGitInit(t, store)
	sbMkdir(t, filepath.Join(store, beadsDirName))
	sbMkdir(t, filepath.Join(work, beadsDirName))
	sbWrite(t, filepath.Join(work, beadsDirName, beadsRedirect), filepath.Join(store, beadsDirName)+"\n")

	a := NewAppAt(sbMkdir(t, filepath.Join(root, "home")))
	// The security PID shape: runtime claude, record: trusted, cage seatbelt,
	// and Edit/Write denied — the best grade §4 gives, which is the point:
	// the runtime's willingness is not the question this row asks.
	ag := &AgentFile{Name: "security", Deny: []string{"Edit", "Write"}, Cage: CageSeatbelt,
		MemoryDir: sbMkdir(t, filepath.Join(a.PersonasDir(), "security"))}
	rt, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	return rchFixture{a: a, work: work, store: store, ag: ag, rt: rt}
}

// targets is what the record stage has to be able to write for this launch.
func (f rchFixture) targets(t *testing.T) []string {
	t.Helper()
	got := recordTargets(beadsHome(f.work))
	// The fixture is the experiment: if it stopped naming the store's own
	// .beads and .git, every probe below would measure nothing and pass.
	for _, want := range []string{filepath.Join(f.store, beadsDirName), filepath.Join(f.store, ".git")} {
		found := false
		for _, g := range got {
			if g == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("fixture no longer reaches %s — the probes below would measure nothing: %v", want, got)
		}
	}
	return got
}

// profile renders the artifact this launch would really run under.
func (f rchFixture) profile(t *testing.T) string {
	t.Helper()
	p, err := f.a.RenderSeatbelt(f.ag, f.work, f.rt.StateDirs...)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// rchEdit copies a rendered profile and rewrites it — the control arm and
// the h15 arm both build their profile out of a passing one, so the only
// difference between the arms is the edit.
func rchEdit(t *testing.T, profile string, edit func(string) string) string {
	t.Helper()
	b, err := os.ReadFile(profile)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "control.sb")
	sbWrite(t, out, edit(string(b)))
	return out
}

func rchSkip(t *testing.T) {
	t.Helper()
	// Not "is sandbox-exec here" — "may this process apply one". Inside a
	// caged persona session it is here and refused, seatbeltReachRow then
	// has nothing to measure, and the control below asserts exactly the
	// refusal a blanket one produces (ranger-base-xjw9). The guard and the
	// code under test now ask the same reader (ranger-base-heur).
	sbSkipUnlessSandboxable(t)
}

// 1. The artifact as it launches. The live shape, rendered by the same call
// CreateSession makes, judged by running the write the record stage needs.
func TestQARecordReachPassesOnTheProfileAsItLaunches(t *testing.T) {
	rchSkip(t)
	f := rchNew(t)
	why, unmeasured := seatbeltReachRow(f.profile(t), f.targets(t))
	if unmeasured != "" {
		t.Fatalf("rchSkip said this process may apply a profile and the probe disagreed: %s", unmeasured)
	}
	if why != "" {
		t.Fatalf("the live profile must reach the store of record: %s", why)
	}
}

// 2. The oyta control: the same profile with the two store-of-record grants
// deleted — the pre-23c4e54 shape, where `bd sync`, `bd export` and the
// path-limited commit were all denied and nothing observed it.
func TestQARecordReachFailsOnTheOytaControl(t *testing.T) {
	rchSkip(t)
	f := rchNew(t)
	pass := f.profile(t)
	dropped := 0
	// The two grants ranger-base-rhw added and nothing else: the store's
	// own .beads and its .git, by exact line. Anything broader would take
	// the carve-out's trailing deny on that repo's hook slots with it, and
	// a control that WIDENS the profile is not this control.
	grants := map[string]bool{
		"  (subpath " + sbQuote(absResolve(filepath.Join(f.store, beadsDirName))) + ")": true,
		"  (subpath " + sbQuote(absResolve(filepath.Join(f.store, ".git"))) + ")":       true,
	}
	control := rchEdit(t, pass, func(body string) string {
		var keep []string
		for _, line := range strings.Split(body, "\n") {
			if grants[line] {
				dropped++
				continue
			}
			keep = append(keep, line)
		}
		return strings.Join(keep, "\n")
	})
	// The control is only a control if it really removed the grants: an
	// edit that matched nothing would leave the passing profile in place
	// and the assertion below would be measuring the wrong file.
	if dropped != 2 {
		t.Fatalf("control must delete exactly the two store-of-record grants, dropped %d", dropped)
	}
	why, unmeasured := seatbeltReachRow(control, f.targets(t))
	if unmeasured != "" {
		t.Fatalf("nothing was measured, so this control asserts nothing: %s", unmeasured)
	}
	if why == "" {
		t.Fatal("the pre-fix profile denies the store of record; the check must say so")
	}
	if !strings.Contains(why, AbbrevHome(filepath.Join(f.store, beadsDirName))) {
		t.Errorf("the row must name the unreachable target: %s", why)
	}
}

// 3. The h15 pin, and the reason the judgement is behavioral. SBPL is
// last-match-wins: this profile's ALLOW block still names the store — a
// check that inspected SeatbeltWritable's return value, or grepped the
// profile for the grant, passes here — and the trailing deny takes it
// straight back. That is the exact shape ranger-base-h15's carve-out
// arrived in, and the shape every future narrowing will arrive in.
func TestQARecordReachFailsOnATrailingDenyNamingTheStore(t *testing.T) {
	rchSkip(t)
	f := rchNew(t)
	target := absResolve(filepath.Join(f.store, beadsDirName))
	narrowed := rchEdit(t, f.profile(t), func(body string) string {
		return body + "\n;; a future narrowing, reviewed as strictly safer\n(deny file-write*\n  (subpath " + sbQuote(target) + ")\n)\n"
	})
	// The grant is still there: what changed is which rule matches last.
	b, err := os.ReadFile(narrowed)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "(allow file-write*") || !strings.Contains(string(b), "  (subpath "+sbQuote(target)+")\n") {
		t.Fatalf("premise gone: the allow block must still name %s, or this pins the wrong thing", target)
	}
	why, unmeasured := seatbeltReachRow(narrowed, f.targets(t))
	if unmeasured != "" {
		t.Fatalf("nothing was measured, so this pin asserts nothing: %s", unmeasured)
	}
	if why == "" {
		t.Fatal("a trailing deny naming the store of record re-denies it; the check must fail")
	}
	if !strings.Contains(why, AbbrevHome(filepath.Join(f.store, beadsDirName))) {
		t.Errorf("the row must name the unreachable target: %s", why)
	}
	// The git dir the deny did NOT name stays reachable — the row is about
	// a target, not about the profile in general.
	if got, un := seatbeltReachRow(narrowed, []string{filepath.Join(f.store, ".git")}); got != "" || un != "" {
		t.Errorf("only the denied target may fail: %s%s", got, un)
	}
}

// reachRow pulls the reachability row out of a matrix: "" when the launch
// is not degraded by it.
func reachRow(p Parity) string {
	for _, line := range p.Degraded {
		if strings.HasPrefix(line, RecordReachGate+" — ") {
			return line
		}
	}
	return ""
}

// The wiring, which is the half a profile-level pin cannot reach: the row
// has to ride the parity path CreateSession already calls, or the check is
// a function nobody runs.
//
// The unreachable arm is not contrived — it is ranger-base-f5dg, live and
// open: a redirect that stays UNDER cwd is skipped by SeatbeltWritable's
// `!underDir(cwd, home)` guard as already-covered, and for a PID that
// denies Edit/Write nothing covers it (only cwd/.beads and cwd/.git are
// granted). MEASURED there with bd 0.49.1: `bd sync` and `bd export` fail
// on the database with "operation not permitted". No observable saw it
// before this row.
func TestQARecordReachRidesTheParityPath(t *testing.T) {
	rchSkip(t)
	f := rchNew(t)
	if row := reachRow(f.a.CheckParityIn(f.ag, f.rt, CageSeatbelt, TierStrong, f.work)); row != "" {
		t.Fatalf("the ordinary redirect is reachable; parity must not degrade: %s", row)
	}
	if got := f.a.CheckParityIn(f.ag, f.rt, CageSeatbelt, TierStrong, f.work).Realized[RecordReachGate]; !strings.Contains(got, "probed at launch") {
		t.Errorf("a pass must be a printed row, not a silence: %q", got)
	}

	// ranger-base-f5dg's shape, in the same tree.
	inner := sbMkdir(t, filepath.Join(f.work, "inner", beadsDirName))
	sbWrite(t, filepath.Join(f.work, beadsDirName, beadsRedirect), inner+"\n")
	row := reachRow(f.a.CheckParityIn(f.ag, f.rt, CageSeatbelt, TierStrong, f.work))
	if row == "" {
		t.Fatal("a store of record the profile does not grant must degrade the launch")
	}
	if !strings.Contains(row, AbbrevHome(inner)) {
		t.Errorf("the row must name the unreachable target: %s", row)
	}
}

// ranger-base-heur — the third answer. Inside a caged persona session the
// kernel refuses the nested sandbox_apply, so every probe above measures
// nothing; the row reported that as a store of record the profile denies —
// a finding about the grant drawn from a measurement that never happened —
// and, being a finding, it DEGRADED the launch. Both halves are pinned:
// what the row says, and that the launch survives it.
//
// The seam is the READER (sandboxApplyRefusal) and not the kernel's
// permission to read, so both arms run on a host that will apply a profile
// and inside a session that will not.
func rchFakeApplyRefusal(t *testing.T, why string) {
	t.Helper()
	old := sandboxApplyRefusal
	sandboxApplyRefusal = func() string { return why }
	t.Cleanup(func() { sandboxApplyRefusal = old })
}

const rchKernelRefusal = "sandbox-exec: sandbox_apply: Operation not permitted"

func TestQARecordReachAbstainsWhenThisProcessMayNotApplyAProfile(t *testing.T) {
	f := rchNew(t)
	rchFakeApplyRefusal(t, rchKernelRefusal)

	p := f.a.CheckParityIn(f.ag, f.rt, CageSeatbelt, TierStrong, f.work)
	got := p.Realized[RecordReachGate]
	if !strings.Contains(got, "NOT MEASURED") || !strings.Contains(got, rchKernelRefusal) {
		t.Fatalf("the row must say nothing was measured, in the kernel's own words: %q", got)
	}
	// Neither of the two things it is not. The first is the bug: a verdict
	// about the grant. The second would be worse: a pass it did not earn.
	if strings.Contains(got, "not writable under the profile") {
		t.Errorf("an unapplied probe is not a denied target: %q", got)
	}
	if strings.Contains(got, "probed at launch") {
		t.Errorf("nothing was probed: %q", got)
	}
	// The half the bead is actually about.
	if row := reachRow(p); row != "" {
		t.Fatalf("a check that did not run must not degrade the launch: %s", row)
	}
	for _, u := range p.Unrealized {
		if strings.HasPrefix(u, RecordReachGate+" — ") {
			t.Fatalf("unrealized carries the row: %s", u)
		}
	}
	// And the row itself is the same string the function returns, so the
	// abstention cannot be a denial wearing different prose one layer down.
	if why, un := seatbeltReachRow(f.profile(t), f.targets(t)); why != "" || un != rchKernelRefusal {
		t.Errorf("seatbeltReachRow: why=%q unmeasured=%q", why, un)
	}
}

// The wrong arm, and it runs in both worlds: a probe failure that is NOT an
// apply refusal is still a finding. `sandbox-exec -f <missing profile>`
// fails before it ever applies anything —
//
//	sandbox-exec: /nope/does-not-exist.sb: No such file or directory   (exit 65)
//
// MEASURED inside the cage (2026-08-29, darwin 25.4.0), which is why this
// arm discriminates where a nested apply cannot be run at all. Widen
// isSandboxApplyRefusal to "Operation not permitted", or abstain on any
// probe error, and this goes red.
func TestQARecordReachStillReportsAFailureThatIsNotAnApplyRefusal(t *testing.T) {
	f := rchNew(t)
	rchFakeApplyRefusal(t, "")

	missing := filepath.Join(t.TempDir(), "not-rendered.sb")
	why, un := seatbeltReachRow(missing, f.targets(t))
	if un != "" {
		t.Fatalf("a profile that is not there was never applied and never refused: %q", un)
	}
	if why == "" {
		t.Fatal("a probe that failed for its own reasons is a finding, not an abstention")
	}
	if !strings.Contains(why, AbbrevHome(f.targets(t)[0])) {
		t.Errorf("the row must name the target it could not write: %s", why)
	}
}

// The classifier, over strings the kernel and the shell really produced on
// this box. Both carry "Operation not permitted"; only one of them means
// nothing was measured, and reading the second as the first is the whole
// bug — so the discriminating word is pinned rather than the phrase.
func TestQAAnApplyRefusalIsNotAWriteRefusal(t *testing.T) {
	for _, tc := range []struct {
		what string
		out  string
		want bool
	}{
		{"the kernel refusing a nested apply", rchKernelRefusal, true},
		{"the sandboxed shell refused a write", "/bin/sh: /store/.beads/.posse-reach-probe.1: Operation not permitted", false},
		{"a profile that is not there", "sandbox-exec: /nope/does-not-exist.sb: No such file or directory", false},
		{"nothing at all", "", false},
	} {
		if got := isSandboxApplyRefusal(tc.out); got != tc.want {
			t.Errorf("%s: isSandboxApplyRefusal(%q) = %v, want %v", tc.what, tc.out, got, tc.want)
		}
	}
}

// The production reader must answer for the profiles the reach probe is
// about to apply, not for some easier one — and it must be what the code
// under test calls by default. Uncaged both apply; inside a caged session
// both are refused; a reader that went back to asking PATH (which is what
// SeatbeltAvailable asks, and what this bug was) says "" in that session
// while the rendered profile is refused, and this goes red.
func TestQAApplyRefusalAgreesWithTheProfileTheProbeWouldApply(t *testing.T) {
	if !SeatbeltAvailable() {
		t.Skip("no sandbox-exec on this host")
	}
	f := rchNew(t)
	err := exec.Command("sandbox-exec", "-f", f.profile(t), "/usr/bin/true").Run()
	if (err == nil) != (sandboxApplyRefusal() == "") {
		t.Fatalf("the probe says applicable=%v; the profile this launch renders says %v",
			sandboxApplyRefusal() == "", err == nil)
	}
}

// The other two cages. shims has no file wall at all, so the row is a
// statement and not a probe; a launch dir with no bead store says so in its
// own words, because "measured nothing" and "measured a pass" must not
// print the same line.
func TestRecordReachAtShimsAndWithNoStore(t *testing.T) {
	f := rchNew(t)
	got := f.a.CheckParityIn(f.ag, f.rt, CageShims, TierStrong, f.work).Realized[RecordReachGate]
	if !strings.Contains(got, "no file wall") {
		t.Errorf("cage shims: %q", got)
	}
	bare := sbMkdir(t, filepath.Join(t.TempDir(), "bare"))
	got = f.a.CheckParityIn(f.ag, f.rt, CageShims, TierStrong, bare).Realized[RecordReachGate]
	if !strings.Contains(got, "nothing to reach") {
		t.Errorf("no store of record: %q", got)
	}
}

// The self-sandboxing cage (codex): membership of the writable roots the
// RENDERED launch line names. No probe — the flag is a list, a later entry
// cancels no earlier one — but it is the line, not the argument list that
// fed it, so a PID's own `command:` that drops {deny} is judged too.
func TestRecordReachOnASelfSandboxingRuntime(t *testing.T) {
	f := rchNew(t)
	codex, err := f.a.LoadRuntime("codex")
	if err != nil {
		t.Fatal(err)
	}
	open := &AgentFile{Name: "dev", MemoryDir: f.ag.MemoryDir}

	// No redirect: the store is under the workspace codex writes anyway.
	plain := sbMkdir(t, filepath.Join(t.TempDir(), "plain"))
	sbGitInit(t, plain)
	sbMkdir(t, filepath.Join(plain, beadsDirName))
	if row := reachRow(f.a.CheckParityIn(open, codex, CageShims, TierStrong, plain)); row != "" {
		t.Errorf("a store under the workspace needs no --add-dir: %s", row)
	}

	// -s read-only: the sandbox makes NOTHING writable, so the best
	// record: grade in the world still records nothing. A file gate bought
	// at the cost of the record stage is a trade, and the row is what puts
	// it in front of the operator instead of in the dark.
	ro := f.a.CheckParityIn(f.ag, codex, CageShims, TierStrong, plain)
	if row := reachRow(ro); !strings.Contains(row, "read-only") {
		t.Errorf("codex -s read-only reaches nothing: %q", row)
	}

	// The redirect shape — the fleet's own, and the arm this test was
	// written waiting for. The line used to name beadsHome(dir) and the
	// SESSION's git dirs, never the STORE's, so `bd sync`'s commit of the
	// JSONL was denied at index.lock exactly as it was under the pre-fix
	// seatbelt; launchWritableRoots names all three now (ranger-base-xqwr)
	// and this is the pass.
	if row := reachRow(f.a.CheckParityIn(open, codex, CageShims, TierStrong, f.work)); row != "" {
		t.Errorf("the redirect shape must be reachable: %s", row)
	}
	// The pass is not vacuous. Strip the store's git dir back off the
	// rendered line — the exact pre-fix line — and the row must come back
	// naming it. The strip is asserted to have changed the line, so an arm
	// that measures nothing fails rather than passes (ranger-base-fm4p).
	gitDir := filepath.Join(f.store, ".git")
	line := f.a.renderedLaunchLine(open, codex, TierStrong, f.work)
	// The line names the RESOLVED root since ranger-base-c02a (codex refuses
	// a root with a symlink component, and t.TempDir() is behind /var ->
	// /private/var here); the row below still judges the literal target,
	// because underDir resolves both sides.
	pre := strings.ReplaceAll(line, " --add-dir "+shellQuote(codexWritableRoot(gitDir)), "")
	if pre == line {
		t.Fatalf("the rendered line never named %s, so the control below measures nothing:\n%s", gitDir, line)
	}
	row := codexReachRow(pre, f.work, recordTargets(beadsHome(f.work)))
	if !strings.Contains(row, AbbrevHome(gitDir)) {
		t.Errorf("without the store's git dir the row must name it: %q", row)
	}
	// …and beadsHome itself is named on both lines, which is what
	// ranger-base-0fb fixed: this row is about the git dir, not the store dir.
	if strings.Contains(row, AbbrevHome(filepath.Join(f.store, beadsDirName))+" is in no") {
		t.Errorf("the .beads grant is on the line; the row must not claim otherwise: %q", row)
	}
}

// shellTokens reads flag VALUES, so it has to survive what shellQuote
// writes — a path with a space, and an apostrophe in a persona's own name.
func TestShellTokensUnquotesLikeTheShellThatTypesIt(t *testing.T) {
	line := "codex -s workspace-write --add-dir " + shellQuote("/a b/.beads") + " --add-dir " + shellQuote("/o'brien/.git")
	got := shellTokens(line)
	want := []string{"codex", "-s", "workspace-write", "--add-dir", "/a b/.beads", "--add-dir", "/o'brien/.git"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q want %q", got, want)
	}
	if row := codexReachRow(line, "/elsewhere", []string{"/a b/.beads"}); row != "" {
		t.Errorf("a quoted --add-dir is a grant: %s", row)
	}
	if row := codexReachRow(line, "/elsewhere", []string{"/a b/other"}); row == "" {
		t.Error("a path outside every root must not read as granted")
	}
}
