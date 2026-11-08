//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-8gh58 — the Rex codex LINE (ADR 0076 Amendment 2,
// D6, D7, D8) and D9's version pin.
//
// THE CLAIM, in four parts:
//
//  1. herdr's codex line does not move (D8). `--disable hooks` stays on it,
//     no hooks declaration appears, and RexReportLine hands the line back
//     byte-identical without starting a child.
//  2. The Rex line drops that flag and carries the pair — declaration FIRST,
//     trust after it — each quoted so that a shell hands codex back the exact
//     bytes posse asked about (measured here through a real /bin/sh, because
//     the declaration carries single quotes of its own by construction).
//  3. Anything posse cannot vet refuses the LAUNCH and never types half a
//     line: a flag that would switch the hooks off again, an ask that failed,
//     a half-rendered pair.
//  4. Admission is two declarations read before the claim (D1 and D9): the
//     bool, and the release a routed report was measured on.
//
// The arm easiest to lose is 2's ORDER. codex applies `-c` overrides in
// order and `hooks={…}` sets the whole `hooks` table, so a trust value typed
// first is wiped by the declaration that follows it — a line carrying every
// byte of the right answer, a launch that completes, and a seat on the
// startup review dialog with no record and no turn (MEASURED 2026-11-06,
// ranger-base-fwwl8).

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// The fake answer, shaped like the one codex really gives: a declaration
// holding SINGLE QUOTES around a handler path with a SPACE in it (which is
// what codex's own whitespace split requires, MEASURED under
// ranger-base-z7kle), and a trust value holding the double quotes a TOML
// inline table needs.
const (
	crxDecl  = `hooks={SessionStart=[{hooks=[{type="command",command="'/opt/po sse/posse' hook-report codex",timeout=3}]}]}`
	crxState = `hooks.state={"/<session-flags>/config.toml:SessionStart:0:0"={trusted_hash="sha256:0ba1"}}`
)

// crxAsk swaps the ask for a canned answer and records every call. It is why
// none of these pins may take t.Parallel: the seam is a package var.
func crxAsk(t *testing.T, flags CodexHooksFlags, err error, log string) *[]CodexHooksAsk {
	t.Helper()
	var asked []CodexHooksAsk
	prev := rexCodexFlags
	t.Cleanup(func() { rexCodexFlags = prev })
	rexCodexFlags = func(a CodexHooksAsk) (CodexHooksFlags, error) {
		asked = append(asked, a)
		if log != "" && a.Log != nil {
			a.Log.Write([]byte(log))
		}
		return flags, err
	}
	return &asked
}

// crxRex points an App at a `multiplexer: rex` instance.
func crxRex(t *testing.T, a *App) {
	t.Helper()
	if err := os.WriteFile(a.ConfigPath, []byte(MultiplexerKey+": "+MuxRex+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ─── D8: the herdr line does not move ────────────────────────────────────────

// The const itself, typed out by hand. Everything else in this file is a
// derivation; this is the one place the bytes are asserted, and the reason it
// is worth a line of its own is that D8's promise is about THESE bytes:
// "CodexFleetFlags keeps `--disable hooks` on herdr, which reads codex
// through its own detection (§6: herdr's code paths do not change)".
func TestQACodexFleetFlagsIsByteIdenticalOnHerdr(t *testing.T) {
	t.Parallel()
	const want = `--disable hooks -c allow_login_shell=false -c "projects={\"$PWD\"={trust_level=\"trusted\"}}"`
	if CodexFleetFlags != want {
		t.Errorf("CodexFleetFlags moved:\n got %s\nwant %s\n\nADR 0076 D8: the herdr codex line does not change. The Rex line is built by REMOVING a flag and APPENDING two (RexReportLine); if this const had to move for that, the removal is reaching the wrong way round.", CodexFleetFlags, want)
	}
	// And the rendered line carries it verbatim, which is what a herdr
	// launch actually types.
	a := NewAppAt(t.TempDir())
	rt := builtinRuntime(t, "codex")
	ag := &AgentFile{Name: "cx", Path: filepath.Join(a.AgentsDir, "cx.md"), MemoryDir: t.TempDir()}
	line := ag.RenderCommandFor(rt, "codex", TierStandard)
	if !strings.Contains(line, CodexFleetFlags) {
		t.Errorf("the rendered codex line does not carry CodexFleetFlags verbatim:\n%s", line)
	}
	if strings.Contains(line, "hooks={") || strings.Contains(line, "hooks.state=") {
		t.Errorf("the rendered codex line carries a hooks declaration before any instance asked for one — D6/D7 are typed per LAUNCH, on a Rex instance only:\n%s", line)
	}
}

// CodexSeatConf is what the ask is asked with, and the const is what the seat
// gets. Two spellings of one grant: this is the pin that keeps them one, and
// the COUNT is the half that catches the real drift — a third `-c` added to
// the line and not to the ask would have posse asking codex about a line the
// seat does not get, and the drift guard in codexHooksState cannot see a
// value it was never told about.
func TestQACodexSeatConfIsCodexFleetFlagsOwnValues(t *testing.T) {
	t.Parallel()
	conf := CodexSeatConf("/tmp/seat")
	if got, want := strings.Count(CodexFleetFlags, " -c "), len(conf); got != want {
		t.Errorf("CodexFleetFlags types %d `-c` value(s) and CodexSeatConf renders %d — the ask must be asked with the line the seat gets (ADR 0076 D7)\n%s\n%q", got, want, CodexFleetFlags, conf)
	}
	// Each value, in the const's own escaped spelling with $PWD where the
	// seat's cwd goes.
	for _, c := range CodexSeatConf("$PWD") {
		escaped := strings.ReplaceAll(c, `"`, `\"`)
		if !strings.Contains(CodexFleetFlags, escaped) {
			t.Errorf("CodexSeatConf renders %q, which the typed line does not carry as %q:\n%s", c, escaped, CodexFleetFlags)
		}
	}
	if !strings.Contains(strings.Join(conf, " "), "/tmp/seat") {
		t.Errorf("CodexSeatConf did not resolve the seat's cwd into the trust grant: %q", conf)
	}
}

// ─── D6/D7: the Rex line ─────────────────────────────────────────────────────

func TestQARexReportLineDropsTheDisableAndTypesTheDeclarationBeforeTheTrust(t *testing.T) {
	a := hermetic(t, NewAppAt(t.TempDir()))
	crxRex(t, a)
	asked := crxAsk(t, CodexHooksFlags{Declaration: crxDecl, State: crxState}, nil, "")
	rt := builtinRuntime(t, "codex")
	dir := t.TempDir()

	line, err := a.RexReportLine(rt, "s1", dir, "codex -a never "+CodexFleetFlags, os.Stderr)
	if err != nil {
		t.Fatalf("the Rex codex line was refused: %v", err)
	}
	if strings.Contains(line, CodexDisableHooks) {
		t.Errorf("the Rex line still switches codex's hook layer off, over the reporter posse just declared:\n%s", line)
	}
	// The QUOTED spelling, which is what is really typed: shellQuote turns
	// the declaration's own single quotes into `'\''`, so the raw value is
	// not a substring of the line and must not be asserted as one. That the
	// quoting round-trips to the exact bytes codex needs is the next pin's
	// job, through a real shell.
	di, si := strings.Index(line, shellQuote(crxDecl)), strings.Index(line, shellQuote(crxState))
	if di < 0 || si < 0 {
		t.Fatalf("the Rex line is missing half the pair (declaration at %d, trust at %d):\n%s", di, si, line)
	}
	// THE ORDER. MEASURED both ways round (ranger-base-fwwl8): trust first
	// and codex reads every hook back untrusted, because `hooks={…}` sets the
	// whole table including the `state` subtable under it.
	if di > si {
		t.Errorf("the trust is typed BEFORE the declaration, which wipes it — codex applies `-c` in order and `hooks={…}` sets the whole table (MEASURED, ADR 0076 D7):\n%s", line)
	}
	// The rest of the line is untouched: this is a removal and an append,
	// never a re-render.
	if !strings.HasPrefix(line, "codex -a never -c allow_login_shell=false") {
		t.Errorf("the Rex line rebuilt the part of the line it was handed:\n%s", line)
	}
	// And the ask was asked about THIS seat.
	if len(*asked) != 1 {
		t.Fatalf("the ask ran %d time(s), want exactly one per launch (ADR 0076 D7)", len(*asked))
	}
	q := (*asked)[0]
	if q.Cwd != resolveLinks(dir) {
		t.Errorf("the ask was asked about %q, and the seat's cwd is %q — codex discovers hooks from the cwd and keys project trust by the resolved path", q.Cwd, resolveLinks(dir))
	}
	if q.App != "codex" || q.Exe != RunningPosse().Exe || q.Bin != rt.Exe() {
		t.Errorf("the ask names app=%q exe=%q bin=%q, want codex / this posse / %q", q.App, q.Exe, q.Bin, rt.Exe())
	}
	if strings.Join(q.Conf, "|") != strings.Join(CodexSeatConf(resolveLinks(dir)), "|") {
		t.Errorf("the ask was asked with %q, and the seat's own `-c` values are %q", q.Conf, CodexSeatConf(resolveLinks(dir)))
	}
}

// The quoting, through a REAL shell, because the line is typed into one — a
// pane's, or the launch script a line over PaneLineMax spills to. The
// declaration carries single quotes by construction (codex splits a hook
// `command` on whitespace and honours them, MEASURED), so a naive `'…'`
// wrapper would end the word in the middle of the handler path and codex
// would be handed a value it could not parse: the dialog, the stall, the
// silent seat.
func TestQARexReportLineSurvivesBeingTypedIntoAShell(t *testing.T) {
	a := hermetic(t, NewAppAt(t.TempDir()))
	crxRex(t, a)
	crxAsk(t, CodexHooksFlags{Declaration: crxDecl, State: crxState}, nil, "")
	rt := builtinRuntime(t, "codex")

	// A minimal line on purpose: no `$(cat …)` anywhere near a shell this
	// test is about to run.
	line, err := a.RexReportLine(rt, "s1", t.TempDir(), "codex -a never "+CodexDisableHooks, os.Stderr)
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("/bin/sh", "-c", "set -- "+line+"; for w; do printf '%s\\n' \"$w\"; done").Output()
	if err != nil {
		t.Fatalf("/bin/sh could not even parse the line posse would type (%v):\n%s", err, line)
	}
	want := []string{"codex", "-a", "never", "-c", crxDecl, "-c", crxState}
	got := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("a shell splits the typed line into\n  %q\nand codex must receive\n  %q\n\nline: %s", got, want, line)
	}
}

// Off Rex, and off the route: no child, no edit. The first arm is D8's own
// sentence and the second is what keeps claude's native report from growing a
// hook declaration nobody measured.
func TestQARexReportLineIsSilentOffRexAndOffTheRoute(t *testing.T) {
	a := hermetic(t, NewAppAt(t.TempDir()))
	asked := crxAsk(t, CodexHooksFlags{Declaration: crxDecl, State: crxState}, nil, "")
	cx, cl := builtinRuntime(t, "codex"), builtinRuntime(t, "claude")
	const herdrLine = "codex -a never " + CodexFleetFlags

	// A herdr instance (the default — no key at all).
	got, err := a.RexReportLine(cx, "s1", t.TempDir(), herdrLine, os.Stderr)
	if err != nil || got != herdrLine {
		t.Errorf("a herdr codex line was rewritten: (%q, %v)\nwant it back byte-identical (ADR 0076 D8)", got, err)
	}
	// A Rex instance, and a runtime whose record is its OWN output.
	crxRex(t, a)
	const claudeLine = "claude --permission-mode auto"
	if got, err := a.RexReportLine(cl, "s2", t.TempDir(), claudeLine, os.Stderr); err != nil || got != claudeLine {
		t.Errorf("claude's line grew a hooks declaration: (%q, %v) — claude writes its own record (RexRoute \"\"), so posse types nothing for it", got, err)
	}
	if len(*asked) != 0 {
		t.Errorf("%d `codex app-server` ask(s) were started for launches that have no route to type — the launcher lock is held for that child (ADR 0076 D7)", len(*asked))
	}
}

// A `command:` template that spells the flag so the removal cannot see it.
// posse removes only its own spelling, so the question asked afterwards is
// "is it still here" rather than a list of spellings to keep up with — and
// the answer refuses, because the alternative is a seat that starts, runs no
// hook, writes no record and holds its bead over a turn nobody can read.
func TestQARexReportLineRefusesAFlagThatWouldSwitchTheHooksOffAgain(t *testing.T) {
	a := hermetic(t, NewAppAt(t.TempDir()))
	crxRex(t, a)
	crxAsk(t, CodexHooksFlags{Declaration: crxDecl, State: crxState}, nil, "")
	rt := builtinRuntime(t, "codex")

	_, err := a.RexReportLine(rt, "s1", t.TempDir(), "codex -a never --disable hooks,mcp", os.Stderr)
	if err == nil {
		t.Fatal("a line that still disables codex's hook layer was accepted, over the reporter posse had just declared and trusted")
	}
	for _, want := range []string{CodexDisableHooks, "no program_status record", "ADR 0076"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q:\n%v", want, err)
		}
	}
	// Half a pair is the same class of failure and refuses the same way.
	crxAsk(t, CodexHooksFlags{Declaration: crxDecl}, nil, "")
	if _, err := a.RexReportLine(rt, "s1", t.TempDir(), "codex -a never "+CodexDisableHooks, os.Stderr); err == nil {
		t.Error("a declaration with no trust beside it was typed — on its own it draws the startup review dialog (MEASURED, ADR 0076's `seed` arm)")
	}
}

// What codex said is usually the whole diagnosis, and its `errors` for a
// directory name the declarations that failed to LOAD — the one thing a
// refusal sentence cannot reconstruct, because a hook that did not load is
// absent from the answer. So the inventory is buffered and printed on the
// failing path, and on the happy path it is not printed at all.
func TestQARexReportLineRefusalPrintsWhatCodexReported(t *testing.T) {
	a := hermetic(t, NewAppAt(t.TempDir()))
	crxRex(t, a)
	crxAsk(t, CodexHooksFlags{}, Die("the ask failed after 20s"), "codex hooks/list: 1 hook(s)\n  user untrusted managed=false /x/hooks.json:Stop:0:0\n")
	rt := builtinRuntime(t, "codex")

	var warn strings.Builder
	if _, err := a.RexReportLine(rt, "s1", t.TempDir(), "codex -a never "+CodexDisableHooks, &warn); err == nil {
		t.Fatal("a failed ask did not refuse the launch (ADR 0076 D7: no answer means no trust line)")
	}
	if !strings.Contains(warn.String(), "/x/hooks.json:Stop:0:0") {
		t.Errorf("the refusal did not print the inventory codex reported:\n%s", warn.String())
	}

	// The happy path says one line about the route and never the inventory.
	crxAsk(t, CodexHooksFlags{Declaration: crxDecl, State: crxState}, nil, "codex hooks/list: 8 hook(s)\n")
	warn.Reset()
	if _, err := a.RexReportLine(rt, "s1", t.TempDir(), "codex -a never "+CodexDisableHooks, &warn); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(warn.String(), "hooks/list") {
		t.Errorf("a successful launch printed codex's whole inventory:\n%s", warn.String())
	}
	if !strings.Contains(warn.String(), "ADR 0076 D6/D7") {
		t.Errorf("a launch that typed the route said nothing about it:\n%s", warn.String())
	}
}

// ─── D9: the version pin ─────────────────────────────────────────────────────

func TestQARexVersionPinRefusesAReleaseTheRouteWasNotMeasuredOn(t *testing.T) {
	prev := skillsDriftVersion
	t.Cleanup(func() { skillsDriftVersion = prev })
	cx, cl := builtinRuntime(t, "codex"), builtinRuntime(t, "claude")

	if cx.RexVerified == "" {
		t.Fatal("the codex built-in carries no rex_verified — D9's pin is what makes a ROUTED admission re-measurable, and without it every future codex is admitted on a measurement of 0.153.4")
	}
	// The measured release: admitted, and the reading is handed back so the
	// caller can say what it saw.
	skillsDriftVersion = func(string) string { return cx.RexVerified }
	if got, ok := RexVersionDrift(cx); !ok {
		t.Errorf("the release the route WAS measured on was refused (read %q, want %q)", got, cx.RexVerified)
	}
	// Another release: refused, with the probe named as the re-measurement.
	skillsDriftVersion = func(string) string { return "codex-cli 0.154.0" }
	got, ok := RexVersionDrift(cx)
	if ok {
		t.Fatalf("a codex reporting %q was admitted on a route measured on %q (ADR 0076 D9)", got, cx.RexVerified)
	}
	err := RexVersionRefusal(cx, got)
	for _, want := range []string{"codex-cli 0.154.0", cx.RexVerified, "0076-trust-ask.probe.sh", "before the claim", "herdr"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the version refusal does not name %q:\n%v", want, err)
		}
	}
	if strings.Contains(err.Error(), "posse peek") {
		t.Errorf("a pre-claim refusal points at a session, and its whole placement is that there is none:\n%v", err)
	}
	if w := RexVersionDegraded(cx, got); !strings.Contains(w, "codex-cli 0.154.0") || !strings.Contains(w, "DEGRADED") {
		t.Errorf("the interactive arm does not carry the same facts:\n%s", w)
	}
	// A CLI that will not say: unknown is never read as no (ADR 0013 §1).
	// The pinned claim is "measured on THIS version", and a binary that
	// cannot be identified leaves it unsupported.
	skillsDriftVersion = func(string) string { return "" }
	if got, ok := RexVersionDrift(cx); ok {
		t.Errorf("a codex that would not answer `--version` was admitted (read %q)", got)
	} else if !strings.Contains(RexVersionRefusal(cx, got).Error(), "would not answer") {
		t.Errorf("the refusal for an unreadable version reads as a release called nothing:\n%v", RexVersionRefusal(cx, got))
	}
	// A NATIVE report carries no pin and is never refused over a version:
	// claude's record is claude's own output, and a regression there shows
	// up as the first-report wait missing, with nothing typed (D2).
	skillsDriftVersion = func(string) string { return "claude 9.9.9" }
	if _, ok := RexVersionDrift(cl); !ok {
		t.Error("claude was refused over a version it declares no pin on — D9's pin is for a ROUTED report, which claude's is not")
	}
	if _, ok := RexVersionDrift(nil); !ok {
		t.Error("a nil runtime was refused by the version pin")
	}
}

// ─── the WIRE, which no pin above holds ──────────────────────────────────────

// Every pin above calls RexReportLine DIRECTLY, so all of them stay green
// over a tree whose call site in planLaunch has been deleted — and then a
// codex seat on Rex launches with no reporter, writes no record, and HOLDs
// its bead over a turn nobody can read. So this one drives a real plan and
// reads the line off it.
func TestQACodexRexLaunchLineCarriesTheHooksPairBeforeTheWorkPrompt(t *testing.T) {
	b, _ := newTestBackend(t)
	crxRex(t, b.App)
	crxAsk(t, CodexHooksFlags{Declaration: crxDecl, State: crxState}, nil, "")
	prev := skillsDriftVersion
	t.Cleanup(func() { skillsDriftVersion = prev })
	skillsDriftVersion = func(string) string { return builtinRuntime(t, "codex").RexVerified }

	write(t, filepath.Join(b.App.AgentsDir, "cx.md"), "---\nname: cx\ndescription: test\nruntime: codex\nlabels: [go]\n---\nYou are cx.\n")
	dir := wtRepo(t)
	prompt := filepath.Join(t.TempDir(), "prompt.txt")
	write(t, prompt, "work this bead\n")

	p, err := b.planLaunch(NewSessionOpts{Name: "cx1", Dir: dir, Agent: "cx", Runtime: "codex",
		PromptFile: prompt, Bead: "ranger-base-8gh58", AllowDegraded: true})
	if err != nil {
		t.Fatalf("a codex launch on a Rex instance was refused: %v", err)
	}
	if strings.Contains(p.Cmd, CodexDisableHooks) {
		t.Errorf("the planned line disables codex's hooks on a Rex instance (ADR 0076 D8):\n%s", p.Cmd)
	}
	di, si := strings.Index(p.Cmd, shellQuote(crxDecl)), strings.Index(p.Cmd, shellQuote(crxState))
	if di < 0 || si < 0 || di > si {
		t.Fatalf("the planned line does not carry the pair in order (declaration %d, trust %d):\n%s", di, si, p.Cmd)
	}
	// BEFORE the work prompt, which is positional on codex's line: a flag
	// after it would be an argument to the prompt and not to codex.
	if pi := strings.Index(p.Cmd, prompt); pi < 0 || pi < si {
		t.Errorf("the hooks pair is typed after the work prompt (prompt at %d, trust at %d):\n%s", pi, si, p.Cmd)
	}
}

// ─── what a refusal at this stage COSTS (ADR 0076 D7's own ASSUMED) ──────────

// D7 says "the worktree and the claim are released by the ordinary
// launch-failure path (ASSUMED that it covers this stage; the slice checks)".
// This is the check, and the answer is HALF of what was assumed:
//
//   - the CLAIM is handed back. createSession returns the ask's error,
//     launchWithPrompt wraps it in unclaimAfterLaunchFailure, and the bead
//     goes back to open (the generic mechanism is pinned next door in
//     TestArgvCreateFailureAfterTheClaimUnclaims; this arm is the D7 ask
//     reaching it).
//   - the WORKTREE IS NOT REMOVED. EnsureSessionTree cuts it near the top of
//     planLaunch, two hundred lines above every refusal in that function, and
//     nothing unwinds it — true of the danger, detection, parity, PID-channel
//     and version refusals alike. For THIS refusal it cannot be otherwise:
//     the ask is about the tree (codex discovers hooks from the cwd and keys
//     project trust by it), so it cannot be asked before there is one.
//
// That is a cost worth knowing and not a leak: the tree has no commits, so
// `posse worktrees --land` removes it ("had no commits — worktree and
// <branch> removed"). What it is NOT is a turn, which is the thing D7's two
// refusals exist to protect.
func TestQACodexRexAskRefusalUnclaimsTheBeadAndLeavesTheTreeForTheSweep(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	crxRex(t, b.App)
	crxAsk(t, CodexHooksFlags{}, Die("the ask failed after 20s (bound 20s), so posse does not type a trust line"), "")
	prev := skillsDriftVersion
	t.Cleanup(func() { skillsDriftVersion = prev })
	skillsDriftVersion = func(string) string { return builtinRuntime(t, "codex").RexVerified }

	write(t, filepath.Join(b.App.AgentsDir, "cx.md"), "---\nname: cx\ndescription: test\nruntime: codex\nlabels: [go]\n---\nYou are cx.\n")
	repo := wtRepo(t)
	write(t, filepath.Join(repo, "fake-ready.json"), `[{"id":"a-1","title":"t","labels":["go"]}]`)
	write(t, b.App.ConfigPath, MultiplexerKey+": "+MuxRex+"\nbeads:\n  - "+repo+"\n")

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if n != 0 {
		t.Fatalf("a refused ask dispatched %d bead(s):\n%s", n, out)
	}
	if !strings.Contains(out, "unclaimed") {
		t.Fatalf("the D7 ask refused the launch and the bead was NOT handed back:\n%s", out)
	}
	if bd := bdCalls(t, fake); !strings.Contains(bd, "--claim") || !strings.Contains(bd, "--status open") {
		t.Errorf("the claim/unclaim pair did not reach bd:\n%s", bd)
	}
	// A plain error, so the slot is benched for the pass rather than the
	// same refusal being paid once per bead routed here (ADR 0013 §2
	// property 4, RexRuntimeRefusal's own reason).
	if !strings.Contains(out, "skipped for the rest of this pass") {
		t.Errorf("a refused ask did not bench the persona/runtime slot — every bead routed here meets the same refusal:\n%s", out)
	}
	// And the tree is still there, which is the half of the ASSUMED that is
	// false. Asserted rather than wished away: a reader of this refusal has
	// to know there is a tree waiting for a sweep. The session name is read
	// off the pass's own line, because it is built from the bead and the
	// repo's basename and this fixture's repo is a t.TempDir.
	_, rest, ok := strings.Cut(out, "creating session ")
	if !ok {
		t.Fatalf("the pass printed no create line to read the session name off:\n%s", out)
	}
	session, _, _ := strings.Cut(rest, " (")
	tree, err := b.App.SessionTreePath(repo, session)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tree); err != nil {
		t.Errorf("the session tree for %s is gone after the refusal (%v) — this pin records the OPPOSITE: the tree is cut near the top of planLaunch and no refusal in that function unwinds it, which is why the refusal's cost is a tree for `posse worktrees --land` to sweep and never a turn. If this now passes, ADR 0076 D7's Realized note and codexrex.go's placement comment are both describing a behaviour that changed.", session, err)
	}
}

// ─── D6 under a cage: can the hook open /dev/tty? (the amendment's ASSUMED) ──

// ADR 0076 Amendment 2's Consequences: "On darwin, the hook runs as codex's
// child under posse's seatbelt when the PID is caged. That the hook may open
// /dev/tty there is ASSUMED, and the slice tests it."
//
// WHAT THE SLICE FOUND, and it is the premise rather than the conclusion:
// posse renders NO seatbelt for codex AT ALL. codex declares SelfSandbox, and
// wallRendered is false for a SelfSandbox runtime whatever the tier
// asked for — macOS refuses to nest sandbox-exec, so posse wraps nothing and
// says so as a DeclaredDifference instead (ADR 0017 §2). So at `cage:
// seatbelt` there is no posse profile in front of a codex seat for the hook
// to run under, and the ASSUMED is moot at that tier.
//
// The COUNTERFACTUAL is still worth measuring, because it is what the
// amendment was worried about and because a profile does reach the hook by
// another door: Rex itself runs outside this seat's cage, but a posse that
// ever did wrap codex, or a future runtime on this route that is not
// SelfSandbox, would hand the hook exactly the profile below. The second arm
// renders a real one and asks the kernel.
//
// ALREADY MEASURED, and not repeated here: in ranger-base-z7kle's and
// ranger-base-fwwl8's probe arms every codex ran inside `sandbox-exec
// (version 1)(allow default)(deny network*)` and under codex's OWN
// `-s workspace-write`, and the hook opened /dev/tty and wrote records that
// were read back through Rex. So neither codex's own sandbox nor an outer
// seatbelt stops the open; what this arm adds is posse's own profile, which
// is `(deny file-write*)` with an allow list, and is therefore the one shape
// that COULD have stopped it.
// NO t.Parallel: this reads the package var AvailableCages, which other
// tests write (cmd/testparallel refuses the clearance and is right to —
// the arm below is skipped or run on its value).
func TestQATheHookTTYUnderACagedCodexPID(t *testing.T) {
	cx := builtinRuntime(t, "codex")

	// Arm 1: the premise. No posse seatbelt is rendered for codex at any
	// tier, so the amendment's sentence describes a wall that is not there.
	if !cx.SelfSandbox {
		t.Error("the codex built-in stopped declaring SelfSandbox — if posse now wraps codex in its own seatbelt, the hook's /dev/tty open runs under the profile arm 2 measures, and ADR 0076 Amendment 2's Consequences bullet is live again rather than moot")
	}
	if wallRendered(CageSeatbelt, cx) {
		t.Error("posse rendered a seatbelt profile for a SelfSandbox runtime — macOS refuses to nest sandbox-exec, so this is a wall that cannot apply (parity.go's DeclaredDifference)")
	}

	// Arm 2: the counterfactual, live. Skipped wherever it cannot be
	// measured honestly — not darwin, no sandbox-exec, or no controlling
	// terminal (an unattended suite run has none, and `/dev/tty` is the
	// controlling terminal by definition).
	if runtime.GOOS != "darwin" || !AvailableCages[CageSeatbelt] {
		t.Skip("not a darwin box with sandbox-exec — the profile under test is SBPL")
	}
	// The gate every test whose measurement IS a sandbox-exec goes behind
	// (ranger-base-xjw9): a session that may not APPLY a profile can neither
	// pass nor fail this, and it is exactly the shape a caged crew seat is in
	// — nested seatbelt is refused inside one (MEASURED, this ADR's reporter
	// probe). The Rex block the `tty` probe arm runs this in is outside that
	// cage, which is the other half of why the arm exists.
	sbSkipUnlessSandboxable(t)
	f, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		t.Skipf("this run has no controlling terminal (%v), so there is nothing for the sandboxed open to be compared against", err)
	}
	f.Close()

	a := hermetic(t, NewAppAt(t.TempDir()))
	dir := t.TempDir()
	ag := &AgentFile{Name: "cx", Path: filepath.Join(a.AgentsDir, "cx.md"), MemoryDir: t.TempDir()}
	prof, err := a.RenderSeatbelt(ag, dir, cx.StateDirs...)
	if err != nil {
		t.Fatal(err)
	}
	// THE OPEN AND NOT THE WRITE, deliberately: `openTTY` in codexreport.go
	// is an O_WRONLY open of /dev/tty, which is the syscall `(deny
	// file-write*)` would refuse, and opening without writing emits no OSC
	// bytes into whatever terminal is running this suite. A record written
	// onto the operator's own Rex block by a test is a side effect nobody
	// asked for.
	out, err := exec.Command("sandbox-exec", "-f", prof, "/bin/sh", "-c", "exec 3>/dev/tty").CombinedOutput()
	if err != nil {
		t.Errorf("posse's own seatbelt profile REFUSES an O_WRONLY open of /dev/tty (%v): %s\n\nthat is the open codexreport.go's reporter makes, so a codex seat running under this profile would report nothing and read downstream as a runtime that went silent (ADR 0076 D6). The profile is at %s. Note the reporter answers 0 on a failed open by design — a hook must not break codex's turn — so the failure is SILENT, which is why this is measured rather than assumed.", err, out, prof)
	}
	// And the reporter itself runs under the profile at all — an unknown
	// event, so it opens nothing and writes nothing, which isolates "the
	// binary execs and exits 0 in there" from the open measured above.
	bin := filepath.Join(t.TempDir(), "posse")
	if out, err := exec.Command("go", "build", "-o", bin, "../../cmd/posse").CombinedOutput(); err != nil {
		t.Skipf("could not build posse for the sandboxed run (%v): %s", err, out)
	}
	cmd := exec.Command("sandbox-exec", "-f", prof, bin, HookReportVerb, "codex")
	cmd.Stdin = strings.NewReader(`{"hook_event_name":"NotAnEventCodexHas"}`)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("posse's reporter could not even run under posse's own seatbelt profile (%v): %s", err, out)
	}
}

// The version pin on the WIRE, and it is the arm a direct call cannot hold:
// MEASURED by mutation (ranger-base-8gh58) — with dispatch.go's version arm
// deleted, every pin that calls RexVersionDrift directly stayed green, and a
// codex the route was never measured on would have been claimed, given a
// worktree and handed a prompt on its launch line.
//
// WHAT IT ASSERTS IS THE PLACEMENT, not the message: bd is never asked to
// CLAIM at all. That is D9's own sentence ("before the claim") and the whole
// reason the reading is a declaration rather than a launch-then-wait.
func TestQACodexRexVersionRefusalLandsAboveTheClaim(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	crxRex(t, b.App)
	asked := crxAsk(t, CodexHooksFlags{Declaration: crxDecl, State: crxState}, nil, "")
	prev := skillsDriftVersion
	t.Cleanup(func() { skillsDriftVersion = prev })
	skillsDriftVersion = func(string) string { return "codex-cli 0.154.0" }

	write(t, filepath.Join(b.App.AgentsDir, "cx.md"), "---\nname: cx\ndescription: test\nruntime: codex\nlabels: [go]\n---\nYou are cx.\n")
	repo := wtRepo(t)
	write(t, filepath.Join(repo, "fake-ready.json"), `[{"id":"a-1","title":"t","labels":["go"]}]`)
	write(t, b.App.ConfigPath, MultiplexerKey+": "+MuxRex+"\nbeads:\n  - "+repo+"\n")

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if n != 0 {
		t.Fatalf("a codex the Rex route was not measured on dispatched %d bead(s):\n%s", n, out)
	}
	if bd := bdCalls(t, fake); strings.Contains(bd, "--claim") {
		t.Errorf("the bead was CLAIMED before the version pin refused — D9's whole placement is that it is read off a declaration before anything is spent:\n%s", bd)
	}
	if !strings.Contains(out, "0.154.0") || !strings.Contains(out, "0076-trust-ask.probe.sh") {
		t.Errorf("the pass does not say which release it saw, or where the re-measurement is:\n%s", out)
	}
	if len(*asked) != 0 {
		t.Errorf("%d `codex app-server` ask(s) were started for a launch the version pin had already refused", len(*asked))
	}
	if !strings.Contains(out, "skipped for the rest of this pass") {
		t.Errorf("the version refusal did not bench the slot — every bead routed here meets the same unmeasured CLI:\n%s", out)
	}
}

// planLaunch's own backstop for the same reading (the cockpit's `d` on a
// session it must create, a recipe, a recreate), with the interactive
// asymmetry the readings around it all carry: a launch nobody is watching
// refuses, and the operator's own is told what it has.
func TestQACodexRexVersionBackstopRefusesDispatchAndWarnsTheKeyboard(t *testing.T) {
	b, _ := newTestBackend(t)
	crxRex(t, b.App)
	crxAsk(t, CodexHooksFlags{Declaration: crxDecl, State: crxState}, nil, "")
	prev := skillsDriftVersion
	t.Cleanup(func() { skillsDriftVersion = prev })
	skillsDriftVersion = func(string) string { return "codex-cli 0.154.0" }

	write(t, filepath.Join(b.App.AgentsDir, "cx.md"), "---\nname: cx\ndescription: test\nruntime: codex\nlabels: [go]\n---\nYou are cx.\n")
	dir := wtRepo(t)

	_, err := b.planLaunch(NewSessionOpts{Name: "cx1", Dir: dir, Agent: "cx", Runtime: "codex", Bead: "ranger-base-8gh58", AllowDegraded: true})
	if err == nil {
		t.Fatal("a bead-carrying launch was planned on a codex the Rex route was not measured on")
	}
	if !strings.Contains(err.Error(), "0.154.0") {
		t.Errorf("the backstop refusal does not name what it read:\n%v", err)
	}
	// Interactive: no bead, so it proceeds and says so. The operator is at
	// the keyboard, can read the screen posse cannot, and launching is how
	// the next measurement gets taken (ADR 0013 §1 rule 5).
	p, err := b.planLaunch(NewSessionOpts{Name: "cx2", Dir: dir, Agent: "cx", Runtime: "codex", AllowDegraded: true})
	if err != nil {
		t.Fatalf("an interactive launch was refused by the version pin: %v", err)
	}
	if w := warnBuf(t, b).String(); !strings.Contains(w, "DEGRADED") || !strings.Contains(w, "0.154.0") {
		t.Errorf("the interactive launch said nothing about the version it is opening on:\n%s", w)
	}
	// And it still typed the route: the pin is about which codex this is,
	// not about whether the reporter goes on the line.
	if !strings.Contains(p.Cmd, shellQuote(crxDecl)) {
		t.Errorf("the degraded launch dropped the reporter as well:\n%s", p.Cmd)
	}
}

// The relaunch path, which is the OTHER place a persona line is rendered —
// and a seat it revives without a reporter is a seat posse can no longer
// read at all. The ask runs again rather than its answer being remembered:
// trust binds to the hash of the declaration asked about, and this re-render
// can name a posse that was upgraded since the launch.
//
// The slot here is the herdr fake and the KEY says rex, which is the one
// artificial thing in this fixture: RexReportLine reads the key, and what is
// under test is the line this path types, not Rex's own protocol (the
// first-report wait has its own fifteen pins next door in muxslot_qa_test.go).
func TestQACodexRexRelaunchRetypesTheReporterOntoTheLine(t *testing.T) {
	b, fake := newTestBackend(t)
	crxRex(t, b.App)
	asked := crxAsk(t, CodexHooksFlags{Declaration: crxDecl, State: crxState}, nil, "")
	prev := skillsDriftVersion
	t.Cleanup(func() { skillsDriftVersion = prev })
	skillsDriftVersion = func(string) string { return builtinRuntime(t, "codex").RexVerified }

	write(t, filepath.Join(b.App.AgentsDir, "cx.md"), "---\nname: cx\ndescription: test\nruntime: codex\nlabels: [go]\n---\nYou are cx.\n")
	mustCreate(t, b, NewSessionOpts{Name: "cx1", Agent: "cx", Runtime: "codex", Dir: t.TempDir(), AllowDegraded: true})

	// The state a relaunch needs: the launch is past the grace window and
	// herdr sees no agent in the workspace any more.
	m, ok := b.readMeta("cx1")
	if !ok {
		t.Fatal("no meta for cx1")
	}
	m.Launched = m.Launched.Add(-time.Hour)
	if err := b.writeMeta(m); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(fake, "agents.json"))
	if ok, err := b.RelaunchAgent("cx1", time.Second); err != nil || !ok {
		t.Fatalf("relaunch: %v %v", ok, err)
	}

	lines := paneRunLines(t, fake)
	if len(lines) != 2 {
		t.Fatalf("expected the launch and the relaunch, got %d: %v", len(lines), lines)
	}
	line := resolvedTypedLine(t, lines[1])
	if !strings.Contains(line, "-s workspace-write") {
		t.Fatalf("the relaunch line is not a codex sandbox line:\n%s", line)
	}
	if strings.Contains(line, CodexDisableHooks) {
		t.Errorf("the revived seat switches codex's hooks off again:\n%s", line)
	}
	di, si := strings.Index(line, shellQuote(crxDecl)), strings.Index(line, shellQuote(crxState))
	if di < 0 || si < 0 || di > si {
		t.Errorf("the revived codex seat carries no reporter, or carries it after its trust (declaration %d, trust %d) — it would report nothing and read as a runtime that went silent:\n%s", di, si, line)
	}
	if len(*asked) != 2 {
		t.Errorf("the ask ran %d time(s) across a launch and a relaunch, want 2 — a remembered answer is trust for a declaration this render may no longer be typing", len(*asked))
	}
}
