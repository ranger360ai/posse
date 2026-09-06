//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-6kmkn — ADR 0052 D3, "the probe moves with the
// dispatch path".
//
// WHAT IS BEING HELD. D2 renders a per-session hooks dir and puts git's
// config-in-env form in the session's environment, so on a managed box the
// file git dispatches is NOT the one at `git rev-parse --git-path hooks`.
// The identity half of ADR 0023 has to follow it there, or the probe grades
// the employer's files — which posse did not write, will never write, and
// correctly reads as foreign — and degrades a launch whose wall is fine.
//
// THE THREE ARMS, and what each would let through if it were missing:
//
//	identity     the member and the dispatcher in the SESSION dir are both
//	             byte-for-byte this launch's render. Without it a session
//	             could edit its own wall (the dir is posse's, but it is on
//	             the box) and parity would still print "dispatch verified".
//	absolute     the dispatcher is redirectDispatcher's output for THIS
//	             managed dir, so the §9 sibling form `$d/<slot>` — which the
//	             ordinary probe accepts, and which this file proves it
//	             accepts — is refused here: `$d` is the session dir, so that
//	             chain forwards into a neighbour that does not exist and the
//	             employer's hook silently stops running.
//	completeness every executable in the managed dir has its dispatcher, or
//	             ADR 0052 M4 says this session's git SKIPS that hook. The
//	             loss there is not a posse gate — it is the employer's
//	             control, reduced by posse's own redirect, which is the one
//	             thing this ADR promises never to cause.
//
// Every pin carries its wrong arm, and two of them carry a REAL git commit:
// a claim that a dispatcher is missing is worth what the skipped hook says,
// not what the probe says about itself.

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// probe is the launcher's own call, on the dir this fixture just rendered.
func (f *hrFix) probe(wantPrePush bool) l3HookProbe {
	f.t.Helper()
	return f.a.probeL3HooksIn(f.repo, wantPrePush, &l3Redirect{Hooks: f.hooks, Managed: f.managed})
}

// corrupt rewrites one file in the session hooks dir, +x, and returns the
// path — one byte off the render is the whole fixture for the identity arm.
func (f *hrFix) corrupt(name, body string) string {
	f.t.Helper()
	p := filepath.Join(f.hooks, name)
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		f.t.Fatal(err)
	}
	return p
}

// ─── identity, at the dir git actually dispatches from ───────────────────────

// A freshly rendered session dir counts, and the probe says WHICH dir it
// read and whose hooks follow ours.
//
// The wrong arm is the same repo probed WITHOUT the redirect: git's dispatch
// path there is the employer's directory, so the ordinary probe reads two
// foreign hooks and degrades both slots. That is the whole defect D3 exists
// to close, and it must be visible in the same test — otherwise this pin
// would pass just as well against a probe that ignored its argument.
func TestQARedirectProbeGradesTheSessionDirNotTheManagedPath(t *testing.T) {
	t.Parallel()
	f := hrFixture(t)
	f.render(true)

	got := f.probe(true)
	if !got.Repo || !got.PrePush || !got.CommitGuard {
		t.Fatalf("a freshly rendered session dir must count: %+v", got)
	}
	if got.HooksDir != f.hooks || got.Managed != f.managed {
		t.Errorf("probe named %s / %s, want the session dir %s forwarding into %s", got.HooksDir, got.Managed, f.hooks, f.managed)
	}
	if len(got.Forward) != 0 {
		t.Errorf("a complete render forwards everything: %v", got.Forward)
	}
	if got.PrePushDegraded != "" || got.CommitGuardDegraded != "" {
		t.Errorf("nothing to degrade: %q / %q", got.PrePushDegraded, got.CommitGuardDegraded)
	}

	// The wrong arm.
	off := f.a.probeL3Hooks(f.repo, true)
	if off.PrePush || off.CommitGuard {
		t.Errorf("without the redirect the probe reads the EMPLOYER's dir and must degrade: %+v", off)
	}
	if off.Managed != "" || off.HooksDir != f.managed {
		t.Errorf("the ordinary probe must stay on git's dispatch path: %+v", off)
	}
}

// One byte off a member and the slot does not count — and the line names
// the FILE to look at, not the slot in general, because the two files behind
// one slot fail for different reasons.
func TestQARedirectProbeDegradesAnEditedMemberAndNamesTheFile(t *testing.T) {
	t.Parallel()
	f := hrFixture(t)
	r, _ := f.render(true)

	member := filepath.Join(f.hooks, "posse-prepare-commit-msg")
	body, err := os.ReadFile(member)
	if err != nil {
		t.Fatal(err)
	}
	// One byte, in a comment: the hook still runs and still refuses, so
	// nothing here is measuring behavior — identity is the only thing that
	// changed, which is the ADR 0023 half this bead moves.
	if err := os.WriteFile(member, append(body, '\n'), 0o755); err != nil {
		t.Fatal(err)
	}

	got := f.probe(true)
	if got.CommitGuard {
		t.Fatal("a member that is not this launch's render must not count")
	}
	if !strings.Contains(got.CommitGuardDegraded, AbbrevHome(member)) {
		t.Errorf("the degraded line must name the file: %q", got.CommitGuardDegraded)
	}
	if !strings.Contains(got.CommitGuardDegraded, "re-launch to re-render") {
		t.Errorf("the remedy for a per-session dir is the launch, not install-hooks: %q", got.CommitGuardDegraded)
	}
	if strings.Contains(got.CommitGuardDegraded, "install-hooks") || strings.Contains(got.CommitGuardDegraded, "foreign") {
		t.Errorf("posse's own per-session render is neither foreign nor reinstallable: %q", got.CommitGuardDegraded)
	}
	// Degraded flows into session meta, a flat-file format that silently
	// truncates an embedded newline (ranger-base-ujdg).
	if strings.Contains(got.CommitGuardDegraded, "\n") {
		t.Errorf("degraded lines are ONE line: %q", got.CommitGuardDegraded)
	}
	// The other slot is untouched: one bad file degrades one slot.
	if !got.PrePush {
		t.Errorf("pre-push was not edited and must still count: %q", got.PrePushDegraded)
	}
	// The wrong arm: put the render back and it counts again.
	if err := os.WriteFile(member, body, 0o755); err != nil {
		t.Fatal(err)
	}
	if !f.probe(true).CommitGuard {
		t.Error("restoring the render must restore the slot")
	}
	_ = r
}

// The absolute-neighbour arm. The INSTALL.md §9 sibling dispatcher is a
// legitimate chain in an ordinary hooks dir — the ordinary probe accepts it,
// and this pin proves that in the same breath — but in the session dir `$d`
// resolves to the session dir, so that chain forwards into a neighbour that
// is not there and the employer's hook stops running. Redirect mode takes
// the absolute form and nothing else.
func TestQARedirectProbeRefusesARelativeNeighbourDispatcher(t *testing.T) {
	t.Parallel()
	f := hrFixture(t)
	f.render(true)

	relative := chainRender("prepare-commit-msg", "prepare-commit-msg", true)
	absolute := redirectDispatcher("prepare-commit-msg", f.managed, true)
	if relative == absolute {
		t.Fatal("fixture is vacuous: the two dispatcher forms are the same bytes")
	}
	dispatcher := f.corrupt("prepare-commit-msg", relative)

	got := f.probe(true)
	if got.CommitGuard {
		t.Fatal("a relative-neighbour dispatcher must NOT be accepted in redirect mode")
	}
	if !strings.Contains(got.CommitGuardDegraded, AbbrevHome(dispatcher)) {
		t.Errorf("the degraded line must name the dispatcher: %q", got.CommitGuardDegraded)
	}

	// The control, and the reason this is a REAL difference rather than a
	// tautology: the ordinary identity check accepts exactly these bytes.
	identity, _, _ := l3Identity(f.hooks, "prepare-commit-msg", CommitGuardHook(f.visibility(), f.a.OpsPatternSet(), testIdentity(t, f.repo)...), sharedIndexMarker, legacySharedIndexMarker)
	if !identity {
		t.Fatal("fixture: the sibling chain form must be what the ORDINARY probe accepts, or this pin measures nothing")
	}

	// The wrong arm: the absolute form counts.
	f.corrupt("prepare-commit-msg", absolute)
	if !f.probe(true).CommitGuard {
		t.Error("the render's own dispatcher must count")
	}
}

// ─── forward completeness (ADR 0052 M4) ──────────────────────────────────────

// Delete a dispatcher and the employer's hook stops running — measured by a
// REAL commit, not by the probe's own opinion — and the probe names the slot
// with the remedy that fixes it.
func TestQARedirectProbeNamesAManagedHookThisSessionWouldSkip(t *testing.T) {
	t.Parallel()
	f := hrFixture(t)
	f.render(true)

	// The arm that is being defended: with the dispatcher there, the
	// employer's pre-commit runs under the redirect.
	f.write("a", "1")
	f.git(nil, "add", "a")
	// Path-limited, because the wall this render installs is REAL and
	// refuses an unqualified commit in a shared tree (rangerhq-nyqj) — which
	// is itself the render doing its job on the way past.
	if out, err := f.git(f.redirectEnv(), "commit", "-qm", "with the forward", "--", "a"); err != nil {
		t.Fatalf("commit: %v %s", err, out)
	}
	if !containsString(f.canaries(), "managed-pre-commit") {
		t.Fatalf("fixture: the employer's pre-commit must run under a complete render: %v", f.canaries())
	}
	if gaps := f.probe(true).Forward; len(gaps) != 0 {
		t.Fatalf("a complete render has no gaps: %v", gaps)
	}

	if err := os.Remove(filepath.Join(f.hooks, "pre-commit")); err != nil {
		t.Fatal(err)
	}

	// M4, in the flesh: the slot our dir lacks is SKIPPED, not inherited.
	if err := os.Truncate(f.log, 0); err != nil {
		t.Fatal(err)
	}
	f.write("b", "1")
	f.git(nil, "add", "b")
	if out, err := f.git(f.redirectEnv(), "commit", "-qm", "without the forward", "--", "b"); err != nil {
		t.Fatalf("commit: %v %s", err, out)
	}
	if containsString(f.canaries(), "managed-pre-commit") {
		t.Fatal("fixture: the employer's pre-commit must be SKIPPED with no dispatcher — that is what the arm below reports")
	}

	got := f.probe(true)
	if len(got.Forward) != 1 || !strings.Contains(got.Forward[0], "managed hook pre-commit not forwarded") {
		t.Fatalf("the completeness arm must name the slot: %v", got.Forward)
	}
	if !strings.Contains(got.Forward[0], "re-launch to re-render") {
		t.Errorf("the remedy is the launch that re-renders: %q", got.Forward[0])
	}
	if strings.Contains(got.Forward[0], "\n") {
		t.Errorf("degraded lines are ONE line: %q", got.Forward[0])
	}
	// Posse's own slots are untouched by a missing employer forward: the
	// wall holds, and what is lost is the employer's hook. Reporting this
	// as a posse-slot failure would send the operator after the wrong file.
	if !got.CommitGuard || !got.PrePush {
		t.Errorf("a missing employer forward is not a posse-slot failure: %+v", got)
	}
}

// One problem is reported once. prepare-commit-msg is in BOTH sets — posse
// renders a member for it and the employer has one — so a naive completeness
// arm would print it beside the identity arm's line for the same file.
func TestQARedirectCompletenessDoesNotDoubleReportPosseSlots(t *testing.T) {
	t.Parallel()
	f := hrFixture(t)
	f.render(true)
	if _, err := os.Stat(filepath.Join(f.managed, "prepare-commit-msg")); err != nil {
		t.Fatalf("fixture: the employer must also own this slot, or nothing is being held: %v", err)
	}

	if err := os.Remove(filepath.Join(f.hooks, "prepare-commit-msg")); err != nil {
		t.Fatal(err)
	}
	got := f.probe(true)
	if got.CommitGuard {
		t.Fatal("a missing dispatcher is a missing wall")
	}
	if len(got.Forward) != 0 {
		t.Errorf("the identity arm already named this file; the completeness arm must not say it again: %v", got.Forward)
	}
}

// A managed dir that cannot be listed is a gap, said as one. Completeness is
// a claim about every entry in that directory, and a claim posse could not
// measure is not one it may make quietly.
func TestQARedirectCompletenessRefusesToVouchForAnUnreadableManagedDir(t *testing.T) {
	t.Parallel()
	f := hrFixture(t)
	f.render(true)
	gaps := redirectForwardGaps(f.hooks, filepath.Join(f.managed, "gone"), true)
	if len(gaps) != 1 || !strings.Contains(gaps[0], "cannot be listed") {
		t.Fatalf("an unreadable managed dir must be a gap: %v", gaps)
	}
}

// ─── parity says which dir was probed, and stays undegraded ──────────────────

func TestQARedirectParityNamesTheSessionDirAndTheManagedHooksBehindIt(t *testing.T) {
	t.Parallel()
	f := hrFixture(t)
	f.render(true)
	ag := loadTestAgent(t, "---\nname: dev\ndeny:\n  - Bash(git push:*)\n  - Bash(git commit unless --)\n---\nYou are dev.\n")
	rt, err := f.a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}

	p := f.a.checkParityIn(ag, rt, CageShims, TierStrong, f.repo, &l3Redirect{Hooks: f.hooks, Managed: f.managed})
	if len(p.Degraded) != 0 {
		t.Fatalf("a managed repo with a fresh render launches UNdegraded (ADR 0052 D3): %v", p.Degraded)
	}
	want := "render probed, dispatch verified — session hooks dir, redirected by env; managed hooks " + AbbrevHome(f.managed) + " run after ours"
	for _, gate := range ag.Deny {
		g := p.Realized[gate]
		if !strings.Contains(g.Detail, want) {
			t.Errorf("%s: %q\nmust carry: %q", gate, g.Detail, want)
		}
		// ADR 0025: env-borne or file-borne, L3 is held in-process by the
		// path the hook happens to run on. The redirect does not promote it.
		if g.Class != Cooperative {
			t.Errorf("%s is %v, want Cooperative", gate, g.Class)
		}
	}

	// The wrong arm, twice over: the same repo with no redirect in hand is
	// degraded and says nothing about a session dir.
	off := f.a.CheckParityIn(ag, rt, CageShims, TierStrong, f.repo)
	if len(off.Degraded) == 0 {
		t.Error("without the redirect the employer's hooks are foreign and the launch is degraded")
	}
	for _, gate := range ag.Deny {
		if strings.Contains(off.Realized[gate].Detail, "session hooks dir") {
			t.Errorf("%s claims a redirect nobody rendered: %q", gate, off.Realized[gate].Detail)
		}
	}
}

// A managed hook this session would skip degrades the LAUNCH, even though
// both posse slots hold — the refusal a degraded launch raises is the
// re-render that fixes it.
func TestQARedirectParityDegradesAnUnforwardedManagedHook(t *testing.T) {
	t.Parallel()
	f := hrFixture(t)
	f.render(true)
	ag := loadTestAgent(t, "---\nname: dev\ndeny:\n  - Bash(git push:*)\n  - Bash(git commit unless --)\n---\nYou are dev.\n")
	rt, err := f.a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	red := &l3Redirect{Hooks: f.hooks, Managed: f.managed}
	if p := f.a.checkParityIn(ag, rt, CageShims, TierStrong, f.repo, red); len(p.Degraded) != 0 {
		t.Fatalf("fixture must start clean: %v", p.Degraded)
	}

	if err := os.Remove(filepath.Join(f.hooks, "pre-commit")); err != nil {
		t.Fatal(err)
	}
	p := f.a.checkParityIn(ag, rt, CageShims, TierStrong, f.repo, red)
	if len(p.Degraded) != 1 || !strings.Contains(p.Degraded[0], "managed hook pre-commit not forwarded") {
		t.Fatalf("an employer hook this session would skip must degrade the launch: %v", p.Degraded)
	}
	// And it is still true that posse's own wall is intact — the degraded
	// line is about the employer's hook, and the rows must not lie about it.
	for _, gate := range ag.Deny {
		if !strings.Contains(p.Realized[gate].Detail, "dispatch verified") {
			t.Errorf("%s: posse's own slot still holds: %q", gate, p.Realized[gate].Detail)
		}
	}
}

// ─── the non-managed world is byte-identical to before ───────────────────────

// The whole existing suite is the real pin here (ADR 0052 verification 7);
// this is the statement in one place: nil redirect is the old call, field
// for field, and the parity line gains nothing.
func TestQAWithoutARedirectTheProbeIsExactlyWhatItWas(t *testing.T) {
	t.Parallel()
	b, repo := lhpFixture(t, VisibilityPrivate)
	a := b.App
	a.InstallCommitGuardHook(repo)
	InstallPrePushHook(repo)

	old := a.probeL3Hooks(repo, true)
	got := a.probeL3HooksIn(repo, true, nil)
	if !reflect.DeepEqual(old, got) {
		t.Fatalf("nil redirect must be the old call:\n%+v\n%+v", old, got)
	}
	if !got.CommitGuard || !got.PrePush {
		t.Fatalf("fixture: an ordinary install must count: %+v", got)
	}
	if got.Managed != "" || got.Forward != nil {
		t.Errorf("an ordinary repo has no managed dir and no forwards: %+v", got)
	}
	hooks, _ := hooksDir(repo)
	if got.HooksDir != hooks {
		t.Errorf("probe named %s; git dispatches from %s", got.HooksDir, hooks)
	}

	ag := loadTestAgent(t, "---\nname: dev\ndeny:\n  - Bash(git push:*)\n  - Bash(git commit unless --)\n---\nYou are dev.\n")
	rt, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	p := a.CheckParityIn(ag, rt, CageShims, TierStrong, repo)
	for _, gate := range ag.Deny {
		d := p.Realized[gate].Detail
		if !strings.Contains(d, "render probed, dispatch verified)") {
			t.Errorf("%s lost the line it always printed: %q", gate, d)
		}
		if strings.Contains(d, "session hooks dir") || strings.Contains(d, "managed hooks") {
			t.Errorf("%s says something about a redirect that is not there: %q", gate, d)
		}
	}
}

// visibility is what the render and the probe must BOTH resolve — a test
// that spelled it itself could pass while the two disagreed.
func (f *hrFix) visibility() string {
	f.t.Helper()
	v, _ := f.a.BeadsVisibility(hookRepo(f.repo))
	return v
}

// ─── the record (ADR 0052 D3: session meta) ──────────────────────────────────

// A launch into a managed repo records HOW L3 was realized and whose hooks
// it forwards into. The rendered dir is removed with the session, so after
// the kill nothing else on the box can answer either question.
func TestQAManagedLaunchRecordsHooksModeAndTheManagedDir(t *testing.T) {
	t.Parallel()
	b, repo := lhpFixture(t, VisibilityPrivate)
	managed := filepath.Join(t.TempDir(), "managed-hooks")
	if err := os.MkdirAll(managed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(managed, "pre-commit"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	qaGit(t, repo, "config", "core.hooksPath", managed)
	mhpLock(t, managed)

	warnBuf(t, b)
	// No AllowDegraded, and that is half the pin: ADR 0052's whole point is
	// that a managed repo launches with L3 realized, so criterion 5 runs
	// without the standing waiver criterion 5 forbids.
	mustCreate(t, b, NewSessionOpts{Name: "s1", Dir: repo, Agent: "ranger"})

	m, ok := b.readMeta("s1")
	if !ok {
		t.Fatal("no meta")
	}
	if m.HooksMode != "redirect" || m.ManagedHooks != managed {
		t.Fatalf("meta: hooks_mode=%q managed_hooks=%q, want redirect / %s", m.HooksMode, m.ManagedHooks, managed)
	}
	if m.Degraded != "" {
		t.Errorf("a managed repo with a fresh render is not a degraded launch: %q", m.Degraded)
	}
	// yamlflat: one line each, no embedded newline to be silently truncated
	// on read-back (ranger-base-ujdg).
	body, err := os.ReadFile(b.metaPath("s1"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"hooks_mode: redirect", "managed_hooks: " + managed} {
		if n := strings.Count(string(body), want+"\n"); n != 1 {
			t.Errorf("meta carries %q %d times, want exactly one line:\n%s", want, n, body)
		}
	}

	// The wrong arm: an ordinary repo records neither, so a reader can tell
	// the two realizations apart by the absence.
	b2, plain := lhpFixture(t, VisibilityPrivate)
	mustCreate(t, b2, NewSessionOpts{Name: "s2", Dir: plain, Agent: "ranger"})
	m2, _ := b2.readMeta("s2")
	if m2.HooksMode != "" || m2.ManagedHooks != "" {
		t.Errorf("an ordinary launch recorded a redirect: %q / %q", m2.HooksMode, m2.ManagedHooks)
	}
}

// A dispatcher that is PRESENT but is not the forward posse rendered is a
// gap too. Completeness measured by presence alone would call this session
// complete while the employer's hook never ran — and the file doing the
// skipping would be one in posse's own directory, put there under posse's
// name.
func TestQARedirectCompletenessIsIdentityNotPresence(t *testing.T) {
	t.Parallel()
	f := hrFixture(t)
	f.render(true)
	present := f.corrupt("pre-commit", "#!/bin/sh\nexit 0\n")

	// What that file costs, measured: the employer's pre-commit does not run.
	f.write("a", "1")
	f.git(nil, "add", "a")
	if out, err := f.git(f.redirectEnv(), "commit", "-qm", "tampered forward", "--", "a"); err != nil {
		t.Fatalf("commit: %v %s", err, out)
	}
	if containsString(f.canaries(), "managed-pre-commit") {
		t.Fatalf("fixture: a stub dispatcher must NOT reach the employer's hook: %v", f.canaries())
	}

	gaps := f.probe(true).Forward
	if len(gaps) != 1 || !strings.Contains(gaps[0], "managed hook pre-commit not forwarded") {
		t.Fatalf("a dispatcher that is not the render is not a forward: %v", gaps)
	}
	// The wrong arm: the render's own bytes at that path are a forward.
	if err := os.WriteFile(present, []byte(redirectDispatcher("pre-commit", f.managed, false)), 0o755); err != nil {
		t.Fatal(err)
	}
	if gaps := f.probe(true).Forward; len(gaps) != 0 {
		t.Errorf("the render's own dispatcher must count as forwarded: %v", gaps)
	}
}
