//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-djl3c — ADR 0080 D18: the launch line, identical on
// herdr and Rex panes.
//
// WHAT D18 ADDED AND WHAT IT THEREFORE HAS TO PIN. D17 rendered an argv that
// nothing called. D18 writes it where a line can read it, makes on the host
// what the namespace cannot, and puts the prefix in SeatbeltPrefix's slot at
// the two call sites that prefix. So the subjects here are the LINE and the
// FILES, and three of their failure modes are silent:
//
//  1. a line the pane shell does not parse. The wall rides a redirection
//     before the command name, which is POSIX and is not something a reader
//     can verify by looking at a string. A fake bwrap that copies fd 3 and
//     then execs its own tail measures it on darwin: if the redirection, the
//     quoting of either path or the `--` separator is wrong, the seat opens
//     with no wall and nothing says so.
//  2. a bare `bwrap` on the line. The line runs in the multiplexer's process
//     tree carrying the seat's PATH, which LEADS with the persona's gates dir
//     — so a bare name is one a shim answers, and the shim would stand
//     between the seat and its own wall.
//  3. a prep step that truncates. The launcher creates the session's audit
//     logs so they can be bound back over the deny; an open with O_TRUNC
//     would empty a live seat's refusals.log at every relaunch, and the only
//     reader who would notice is whoever goes looking for a refusal.
//
// EVERY ARM RUNS ON DARWIN, for D17's reason and one more. The render is pure
// of the host except for the tree it stats, so a fixture tree is a fixture
// seat; and the three host measurements this mechanism depends on —
// bwrapResolves, bwrapApplyRefusal and bwrapBin — are package variables for
// exactly this, so the Linux arm is readable here. What is NOT readable here
// is whether the real binary accepts the argv and whether the kernel honours
// the mounts: that is probe T7 and bwrapgitrefslive_test.go, on the D1 host.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// bwlStubBin answers bwrapBin with a path of this test's choosing, so the
// bwrap arm of renderWallOn is reachable on a box with no bwrap. No
// t.Parallel in anything that calls it: it writes a package-level var
// (cmd/testparallel's own rule, the same one wmStub carries).
func bwlStubBin(t *testing.T, bin string) {
	t.Helper()
	had := bwrapBin
	bwrapBin = func() (string, error) { return bin, nil }
	t.Cleanup(func() { bwrapBin = had })
}

// bwlSeat is the fixture: a home with a state dir, a persona, a session dir,
// and a runtime that is not claude (D19's config-dir demand is its own arm
// below, not a precondition of every other one).
type bwlSeat struct {
	a   *App
	ag  *AgentFile
	rt  *Runtime
	dir string
}

func bwlFixture(t *testing.T) bwlSeat {
	t.Helper()
	root := absResolve(t.TempDir())
	// bwlAt and not `at`: cmd/testparallel resolves a called NAME against
	// every `at := func(...)` in the package, and one of those reaches the
	// plan-clock package var blindT — so every t.Parallel test reached from
	// here is reported as touching that var and goes UNCLEARED (the same
	// trap bwTree's own comment records, bwrapargs_qa_test.go). A unique
	// name is the honest fix; a parallelOK line naming blindT would clear a
	// reason these tests do not have.
	bwlAt := func(parts ...string) string { return filepath.Join(append([]string{root}, parts...)...) }
	for _, d := range []string{"proj", "home/state", "home/personas/developer", "codexstate"} {
		if err := os.MkdirAll(bwlAt(d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return bwlSeat{
		a:   &App{Home: bwlAt("home"), StateDir: bwlAt("home", "state")},
		ag:  &AgentFile{Name: "developer", MemoryDir: bwlAt("home", "personas", "developer")},
		rt:  &Runtime{Name: "codex", StateDirs: []string{bwlAt("codexstate")}},
		dir: bwlAt("proj"),
	}
}

// ─── the line ───────────────────────────────────────────────────────────────

// The prefix's shape, letter for letter. It is the one thing in D18 an
// operator reads off a pane's scrollback, and the one thing a later change
// to the render cannot alter without changing what every live seat is behind.
func TestQABwrapPrefixIsTheRedirectionLineD18Types(t *testing.T) {
	t.Parallel()
	got := BwrapPrefix("/usr/bin/bwrap", "/h/state/gates/developer/bwrap.args")
	want := "3<'/h/state/gates/developer/bwrap.args' '/usr/bin/bwrap' --args 3 -- "
	if got != want {
		t.Errorf("BwrapPrefix =\n %q\nwant %q\n(ADR 0080 D18: a redirection before the command name, fd 3 and not stdin, the binary as an absolute path, `--` closing bwrap's own options)", got, want)
	}
	// The trailing space is load-bearing: both call sites concatenate
	// `prefix + cmd`, so losing it joins the separator to the runtime's name.
	if !strings.HasSuffix(got, "-- ") {
		t.Errorf("the prefix must end in `-- ` — the call sites concatenate, and without the space the runtime command is glued to bwrap's argument separator: %q", got)
	}
	// Both paths are quoted, because both come from the box: the gates dir
	// hangs off RHQ_HOME and the binary off the host's own layout, and a
	// home with a space in it is an operator's choice posse does not get to
	// refuse.
	for _, p := range []string{"/with space/bwrap", "/with space/bwrap.args"} {
		if !strings.Contains(BwrapPrefix("/with space/bwrap", "/with space/bwrap.args"), shellQuote(p)) {
			t.Errorf("BwrapPrefix does not quote %s — a home or a prefix with a space in it would split into two words and the seat would open with no wall", p)
		}
	}
	// And `SeatbeltPrefix`'s own shape, beside it, because the two are read
	// together: one tier, two mechanisms, and a reader comparing the panes of
	// a darwin and a Linux seat is comparing these two strings.
	if got, want := SeatbeltPrefix("/h/state/gates/developer/seatbelt.sb"), "sandbox-exec -f '/h/state/gates/developer/seatbelt.sb' "; got != want {
		t.Errorf("SeatbeltPrefix = %q, want %q", got, want)
	}
}

// The line RUN, on this host, against a stand-in binary. This is the arm that
// says the redirection is real rather than well-spelled: the fake copies
// whatever arrived on fd 3 and then execs the tail after `--`, so the
// assertions are (a) the wall bytes reached the binary and (b) the runtime
// command ran as the binary's own argv.
//
// A STAND-IN AND NOT bwrap, deliberately. What is in question here is the
// SHELL's half of D18 — one redirection, two quoted paths and a separator —
// and that half is identical on both hosts, so it is measurable on the host
// that cannot run the other half. The probe's T7 row is the same measurement
// against the real binary (docs/adr/0080-bwrap-tier.probe.sh).
func TestQABwrapLaunchLineHandsTheRenderedArgsToTheBinaryOnFd3(t *testing.T) {
	t.Parallel()
	// A space in every directory, because the line's whole quoting burden is
	// paths that came from the box.
	root := filepath.Join(t.TempDir(), "with space")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	args := filepath.Join(root, "bwrap.args")
	argv := []string{"--die-with-parent", "--ro-bind", "/", "/", "--bind", filepath.Join(root, "a dir"), filepath.Join(root, "a dir")}
	if err := os.WriteFile(args, bwrapArgsBytes(argv), 0o644); err != nil {
		t.Fatal(err)
	}
	captured := filepath.Join(root, "fd3.bin")
	seen := filepath.Join(root, "argv.txt")
	bin := filepath.Join(root, "fake bwrap")
	// `cat <&3` and not a read of the path: the question is whether the
	// LINE delivered the file on that descriptor. Everything before the
	// shifts is recorded, then the tail after `--` is exec'd, which is what
	// the real bwrap does with the runtime command.
	if err := WriteExecutable(bin, []byte("#!/bin/sh\n"+
		"cat <&3 > "+shellQuote(captured)+"\n"+
		"printf '%s\\n' \"$*\" > "+shellQuote(seen)+"\n"+
		"shift 2\n"+
		"[ \"$1\" = \"--\" ] && shift\n"+
		"exec \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	line := BwrapPrefix(bin, args) + "echo RAN"
	out, err := exec.Command("/bin/sh", "-c", line).CombinedOutput()
	if err != nil {
		t.Fatalf("the rendered launch line does not run under /bin/sh: %v\n line: %s\n out: %s", err, line, out)
	}
	if got := strings.TrimSpace(string(out)); got != "RAN" {
		t.Errorf("the runtime command did not run as the wall's own argv tail: output %q, want %q\n line: %s", got, "RAN", line)
	}
	want, err := os.ReadFile(args)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(captured)
	if err != nil {
		t.Fatalf("nothing arrived on fd 3 — the redirection in the typed line did not deliver %s: %v", args, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("fd 3 delivered %d bytes, the args file holds %d — the wall the binary reads is not the wall posse wrote", len(got), len(want))
	}
	flags, _ := os.ReadFile(seen)
	if !strings.Contains(string(flags), "--args 3 --") {
		t.Errorf("the binary's own argv is %q — it must carry `--args 3 --`, which is what tells bwrap to read the wall off the descriptor rather than off the line", strings.TrimSpace(string(flags)))
	}
}

// ─── the files ──────────────────────────────────────────────────────────────

// RenderBwrap's two files: the artifact and its readable twin. The artifact
// is judged against a fresh BwrapArgs over the same lists, so this arm says
// "what was written IS the render" and not "what was written looks right".
func TestQARenderBwrapWritesTheArgsFileAndItsReadableTwin(t *testing.T) {
	// No t.Parallel: bwlStubBin writes a package-level var.
	f := bwlFixture(t)
	bwlStubBin(t, "/usr/bin/bwrap")
	w, err := f.a.renderWallOn(WallBwrap, f.ag, f.rt, f.dir)
	if err != nil {
		t.Fatalf("the bwrap wall does not render for an ordinary seat: %v", err)
	}
	gatesDir := f.a.GatesDir(f.ag.Name)
	if got, want := w.Artifact, filepath.Join(gatesDir, bwrapArgsName); got != want {
		t.Errorf("WallRender.Artifact = %s, want %s — the artifact is what the prefix READS, beside seatbelt.sb", got, want)
	}
	if got, want := w.Prefix, BwrapPrefix("/usr/bin/bwrap", w.Artifact); got != want {
		t.Errorf("WallRender.Prefix = %q, want %q", got, want)
	}
	if w.Mechanism != WallBwrap {
		t.Errorf("WallRender.Mechanism = %q, want %q", w.Mechanism, WallBwrap)
	}

	// The artifact IS the render: same lists, same function, same bytes.
	r, err := f.a.bwrapRender(f.ag, f.rt, f.dir, gatesDir)
	if err != nil {
		t.Fatal(err)
	}
	onDisk, err := os.ReadFile(w.Artifact)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(onDisk, bwrapArgsBytes(r.Argv)) {
		t.Fatalf("%s is not the argv D17 renders for these lists", w.Artifact)
	}
	// NUL-separated, every element terminated including the last — which is
	// what `bwrap --args FD` reads and what the probe's T7 row writes.
	if n := bytes.Count(onDisk, []byte{0}); n != len(r.Argv) {
		t.Errorf("%d NULs for %d argv elements — every element is terminated, the last one included", n, len(r.Argv))
	}
	if len(onDisk) == 0 || onDisk[len(onDisk)-1] != 0 {
		t.Error("the args file must end in a NUL")
	}
	// No newline separator anywhere: a mount source may hold any byte a path
	// may hold, which is why the readable copy is a second FILE and not a
	// second format for this one.
	for _, a := range r.Argv {
		if !bytes.Contains(onDisk, append([]byte(a), 0)) {
			t.Errorf("argv element %q is not in the args file NUL-terminated", a)
		}
	}

	// The readable twin: the stamp, the do-not-edit claim the tree pin
	// derives this file's class from, and the same argv one element per line.
	text, err := os.ReadFile(filepath.Join(gatesDir, bwrapTextName))
	if err != nil {
		t.Fatalf("no readable copy beside the args file: %v", err)
	}
	head, _, _ := strings.Cut(string(text), "\n")
	if !strings.Contains(head, RenderedByPosse()) {
		t.Errorf("the readable copy's header is %q and does not name its renderer — that clause is how a seat reading a wall tells \"relaunch pending\" from \"install pending\" (ranger-base-mmhhc)", head)
	}
	if !strings.Contains(strings.ToLower(string(text)), "do not edit") {
		t.Error("the readable copy must say `do not edit`: nothing reads it back, so a hand-edit would describe a wall that is not up")
	}
	lines := strings.Split(strings.TrimRight(string(text), "\n"), "\n")
	var body []string
	for _, l := range lines {
		if !strings.HasPrefix(l, "#") {
			body = append(body, l)
		}
	}
	if len(body) != len(r.Argv) {
		t.Fatalf("the readable copy has %d non-comment lines for %d argv elements", len(body), len(r.Argv))
	}
	for i := range body {
		if body[i] != r.Argv[i] {
			t.Fatalf("readable line %d is %q, the argv says %q — `tr '\\0' '\\n' < bwrap.args` must give this file's body", i, body[i], r.Argv[i])
		}
	}
	// And the gates dir holds both walls side by side on a box that renders
	// one of them: nothing here writes seatbelt.sb, so a reader finding one
	// is reading the mechanism that ran.
	if _, err := os.Stat(filepath.Join(gatesDir, "seatbelt.sb")); err == nil {
		t.Error("the bwrap render wrote a seatbelt profile too — one mechanism per launch, or `posse gates` cannot say which wall a seat is behind")
	}
}

// The prep: the writes posse makes on the HOST, in the seat's name, because
// bwrap binds only a source that exists. Two halves, and the second is the
// one with a victim — an audit log the launcher truncates is a refusal nobody
// can read afterwards.
//
// WHICH OF THE TWO DEFENCES THIS ONE HOLDS: the first. The refusals.log
// assertion below is end-to-end over the RENDER, and BwrapArgs emits a touch
// step only for a path that is NOT there — so the file this plants never
// reaches the open at all, and the assertion is true however that open is
// spelled. The FLAGS are pinned by
// TestQABwrapPrepTouchDoesNotTruncateAnExistingFile below, which hands
// runBwrapPrep the step directly (ranger-base-e7ink).
func TestQARenderBwrapPrepMakesTheMissingPathsAndNeverTruncates(t *testing.T) {
	// No t.Parallel: bwlStubBin writes a package-level var.
	f := bwlFixture(t)
	bwlStubBin(t, "/usr/bin/bwrap")
	gatesDir := f.a.GatesDir(f.ag.Name)
	if err := os.MkdirAll(gatesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A live seat's audit trail, with a refusal already in it.
	const refusal = "denied: git push\n"
	logPath := filepath.Join(gatesDir, "refusals.log")
	if err := os.WriteFile(logPath, []byte(refusal), 0o644); err != nil {
		t.Fatal(err)
	}
	// A declared state dir that is not there: at L2 that grant costs nothing
	// and here it is a mkdir the launcher owes.
	missing := filepath.Join(filepath.Dir(f.dir), "codexstate", "nested", "not-there")
	f.rt.StateDirs = append(f.rt.StateDirs, missing)

	r, err := f.a.bwrapRender(f.ag, f.rt, f.dir, gatesDir)
	if err != nil {
		t.Fatal(err)
	}
	var wantMkdir, wantTouch bool
	for _, s := range r.Prep {
		switch {
		case s.Kind == PrepMkdirAll && s.Path == absResolve(missing):
			wantMkdir = true
		case s.Kind == PrepTouch && s.Path == filepath.Join(gatesDir, "shell.log"):
			wantTouch = true
		}
		if s.Why == "" {
			t.Errorf("prep step %q carries no reason — this list is the one part of the wall that happens OUTSIDE it, and a path with no reason beside it is a line an operator cannot check", s)
		}
	}
	if !wantMkdir {
		t.Errorf("a declared state dir that is not there must be a `mkdir -p` the launcher owes; prep was %v", r.Prep)
	}
	if !wantTouch {
		t.Errorf("an audit log that is not there must be a `touch` the launcher owes — it has to exist before it can be bound back over the deny; prep was %v", r.Prep)
	}

	if _, err := f.a.renderWallOn(WallBwrap, f.ag, f.rt, f.dir); err != nil {
		t.Fatalf("the render refused after prep: %v", err)
	}
	if st, err := os.Stat(missing); err != nil || !st.IsDir() {
		t.Errorf("the launcher did not make %s a directory (%v) — bwrap would refuse the whole wall at `Can't find source path`", missing, err)
	}
	for _, name := range []string{"refusals.log", "shell.log"} {
		if _, err := os.Stat(filepath.Join(gatesDir, name)); err != nil {
			t.Errorf("the launcher did not create %s: %v", name, err)
		}
	}
	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != refusal {
		t.Errorf("refusals.log reads %q after a render, not %q — the prep must not truncate: every relaunch of a live seat would empty the one file a refusal is recorded in", got, refusal)
	}
	// Idempotent, because a relaunch renders again over the same tree.
	if _, err := f.a.renderWallOn(WallBwrap, f.ag, f.rt, f.dir); err != nil {
		t.Fatalf("a second render over the prepared tree refused: %v", err)
	}
	if got, _ := os.ReadFile(logPath); string(got) != refusal {
		t.Errorf("refusals.log reads %q after a SECOND render — prep is not idempotent", got)
	}
}

// The SECOND of the two defences the comment over runBwrapPrep names, and the
// one the pin above cannot reach. That pin plants refusals.log with a refusal
// in it and reads it back unchanged after two renders, which holds the FIRST
// defence: BwrapArgs emits a touch step only for a path that is NOT there, so
// the file it plants never reaches the open at all. Its assertion is
// therefore true however that open is spelled — `|os.O_TRUNC` added to
// bwrap.go's PrepTouch leaves it, and all 45 wall and bwrap pins, GREEN
// (MEASURED 2026-11-09 on darwin 25.6.0, ranger-base-ojmh6/ranger-base-e7ink).
//
// So this hands runBwrapPrep the step directly, which is the only way to put
// the flags under a pin. The victim is an audit log and the prep runs on the
// HOST, outside the namespace, as the seat's user: the day anything emits a
// touch for a path that is already there — a fix to BwrapArgs, a second
// caller, a re-render against a live seat — a relaunch empties the one file
// a refusal is recorded in. Not a comment going stale, an audit log going
// empty.
func TestQABwrapPrepTouchDoesNotTruncateAnExistingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "refusals.log")
	const refusal = "denied: git push\n"
	if err := os.WriteFile(p, []byte(refusal), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runBwrapPrep([]PrepStep{{Kind: PrepTouch, Path: p, Why: "an audit log that is already there"}}); err != nil {
		t.Fatalf("runBwrapPrep refused a touch of a path that already exists: %v — a relaunch renders the prep again over a live seat's gates dir, so an existing file is the ordinary case and not an error", err)
	}
	if got, _ := os.ReadFile(p); string(got) != refusal {
		t.Errorf("runBwrapPrep's touch EMPTIED %s: it reads %q, want %q — the open must stay os.O_CREATE|os.O_WRONLY with no O_TRUNC (ADR 0080 D18, and runBwrapPrep's own \"A TOUCH NEVER TRUNCATES\")", p, got, refusal)
	}
}

// ─── the one switch ─────────────────────────────────────────────────────────

// RenderWall is the only place the mechanism is chosen, which is what makes
// "the predicate and the prefix changed together" checkable: both arms render
// their own artifact and name it in their own prefix, and a mechanism with no
// arm refuses rather than returning an empty prefix — which would be a launch
// with no wall and nothing said.
func TestQARenderWallIsTheOneSwitchBetweenTheTiersTwoMechanisms(t *testing.T) {
	// No t.Parallel: bwlStubBin writes a package-level var.
	f := bwlFixture(t)
	bwlStubBin(t, "/usr/bin/bwrap")
	for _, mech := range []string{WallSandboxExec, WallBwrap} {
		w, err := f.a.renderWallOn(mech, f.ag, f.rt, f.dir)
		if err != nil {
			t.Errorf("renderWallOn(%q): %v", mech, err)
			continue
		}
		if w.Mechanism != mech {
			t.Errorf("renderWallOn(%q).Mechanism = %q", mech, w.Mechanism)
		}
		// THE INVARIANT THIS FILE IS FOR: the prefix names the artifact the
		// render wrote. A prefix pointing at another file is a seat behind a
		// wall posse is not reporting.
		if !strings.Contains(w.Prefix, shellQuote(w.Artifact)) {
			t.Errorf("renderWallOn(%q) wrote %s and types %q — the prefix must name the artifact it rendered", mech, w.Artifact, w.Prefix)
		}
		if !strings.Contains(w.Prefix, mech) {
			t.Errorf("renderWallOn(%q) types %q, which does not name the mechanism — a line that says one binary for a wall rendered by another is the window ranger-base-pl3ck closed", mech, w.Prefix)
		}
		if st, err := os.Stat(w.Artifact); err != nil || st.IsDir() {
			t.Errorf("renderWallOn(%q) named %s as its artifact and it is not a file here: %v", mech, w.Artifact, err)
		}
	}
	// A mechanism with no arm. Unreachable from a launch — the availability
	// table only names mechanisms wallMechanismRendered admits, which is this
	// switch's own membership — and it refuses out loud anyway.
	if _, err := f.a.renderWallOn("landlock", f.ag, f.rt, f.dir); err == nil {
		t.Error("renderWallOn accepted a mechanism it has no arm for: the alternative to refusing is an empty prefix, which is a seat launched with no wall and no line saying so")
	}
	if _, err := f.a.renderWallOn("", f.ag, f.rt, f.dir); err == nil {
		t.Error(`renderWallOn("") must refuse — "" is the answer for a GOOS with no mechanism at all`)
	}

	// And WallPrefix is that render's prefix and nothing of its own, on this
	// host, through the live mechanism the two call sites use.
	pre, err := f.a.WallPrefix(f.ag, f.rt, f.dir)
	if err != nil {
		t.Fatalf("WallPrefix on %s: %v", runtime.GOOS, err)
	}
	w, err := f.a.RenderWall(f.ag, f.rt, f.dir)
	if err != nil {
		t.Fatal(err)
	}
	// The launch's line is Launch where the render has one — at bwrap, the
	// same prefix with `posse wall-watch` in front of the binary
	// (ranger-base-ihnnl) — and Prefix otherwise.
	launch := w.Prefix
	if w.Launch != "" {
		launch = w.Launch
	}
	if pre != launch {
		t.Errorf("WallPrefix = %q but RenderWall says %q — one answer, or the two launch call sites can type a binary for a wall they did not render", pre, launch)
	}
	if w.Mechanism != WallMechanism() {
		t.Errorf("RenderWall on this box chose %q, and this host's mechanism is %q", w.Mechanism, WallMechanism())
	}
}

// D19's refusal reaches the LAUNCH, which is the thing ranger-base-tc41o
// could not pin: it built the refusal and nothing called it. Unset, a claude
// seat on the bwrap mechanism is refused with the one line of host procedure
// that fixes it, rather than walled behind a grant that cannot hold its
// config writes (probe B8).
func TestQAClaudeOnTheBwrapWallRefusesWithoutTheConfigDirAtTheLaunch(t *testing.T) {
	// No t.Parallel: bwlStubBin writes a package-level var, and this sets env.
	f := bwlFixture(t)
	bwlStubBin(t, "/usr/bin/bwrap")
	f.rt = &Runtime{Name: "claude", StateDirs: []string{"~/.claude", "~/.claude.json"}}
	t.Setenv(ClaudeConfigDirVar, "")
	if _, err := f.a.renderWallOn(WallBwrap, f.ag, f.rt, f.dir); err == nil {
		t.Fatal("a claude seat rendered a bwrap wall with CLAUDE_CONFIG_DIR unset — the runtime would then resolve its config to $HOME/.claude.json, a FILE outside every directory grant, and the wall would go up, report success and lose every config write (ADR 0080 D19)")
	} else if !strings.Contains(err.Error(), ClaudeConfigDirVar) {
		t.Errorf("the refusal must name the variable and the one line of host procedure that sets it: %v", err)
	}
	// Set and absolute: the directory joins the grant set and the launch
	// renders. The same arm says the substitution is reached from the LINE
	// and not only from BwrapStateDirs' own pin.
	cfg := filepath.Join(filepath.Dir(f.dir), "claudecfg")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(ClaudeConfigDirVar, cfg)
	w, err := f.a.renderWallOn(WallBwrap, f.ag, f.rt, f.dir)
	if err != nil {
		t.Fatalf("a claude seat with CLAUDE_CONFIG_DIR set and absolute must render: %v", err)
	}
	argv, err := os.ReadFile(w.Artifact)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(argv, append([]byte(absResolve(cfg)), 0)) {
		t.Errorf("the rendered wall does not bind the resolved %s (%s) — the wall, the seed and the runtime must not disagree about which directory is in play (ADR 0080 D19)", ClaudeConfigDirVar, cfg)
	}
}

// ─── both multiplexers ──────────────────────────────────────────────────────

// "Identical on herdr and Rex panes" (D18), held where it can actually break:
// posse renders ONE line and each occupant of the multiplexer slot types it
// verbatim into its seat. Neither Start may rewrite, re-quote or re-split it
// — the line carries a redirection and two single-quoted paths, and any of
// those three is a wall that silently does not go up.
func TestQABothMultiplexersTypeTheWallLineVerbatim(t *testing.T) {
	t.Parallel()
	line := BwrapPrefix("/usr/bin/bwrap", "/h/state/gates/developer/bwrap.args") + "claude --resume"
	fake := fakeDirOf(t)

	h := HerdrMux{H: Herdr{Bin: fakeBinFor(t, "herdr")}}
	if err := h.Start(Seat{Session: "w1", Pane: "w1:p1"}, line); err != nil {
		t.Fatal(err)
	}
	if log := calls(t, fake); !strings.Contains(log, "pane run w1:p1 "+line) {
		t.Errorf("herdr's pane never received the wall line verbatim.\n want: pane run w1:p1 %s\n log:\n%s", line, log)
	}

	r, _ := newTestRexMux(t)
	// The fake resolves a block target within its session, as rex does
	// (ranger-base-nfc2b), so the seat has to be on the server before a
	// verb can address it.
	rexSeedTestSeat(t)
	if err := r.Start(rexTestSeat(), line); err != nil {
		t.Fatal(err)
	}
	rexCallWith(t, "send", "-b "+rexTestBlock, line)
	rexCallWith(t, "send-key", "-b "+rexTestBlock, "enter")

	// The darwin mechanism's line goes the same way, so the claim is about
	// the SLOT and not about one mechanism's spelling.
	sb := SeatbeltPrefix("/h/state/gates/developer/seatbelt.sb") + "claude --resume"
	if err := h.Start(Seat{Session: "w1", Pane: "w1:p1"}, sb); err != nil {
		t.Fatal(err)
	}
	if log := calls(t, fake); !strings.Contains(log, "pane run w1:p1 "+sb) {
		t.Errorf("herdr's pane never received the sandbox-exec line verbatim:\n%s", log)
	}
	if err := r.Start(rexTestSeat(), sb); err != nil {
		t.Fatal(err)
	}
	rexCallWith(t, "send", "-b "+rexTestBlock, sb)
}

// ─── what `posse gates` reads ───────────────────────────────────────────────

// WallReport is where an operator reads the wall off the output, and the
// bwrap arm has to print the SAME markers over the same lists — a reader who
// has to learn a second vocabulary per host cannot compare two seats.
//
// It also has to print a head line naming a file that EXISTS, which is why
// this report renders rather than composing: SeatbeltReport writes its
// profile for the same reason, and ADR 0015 §2's structural claim is checked
// against the grant set here rather than restated.
//
// SINCE ranger-base-6s5mo THAT FILE IS A REPORT COPY, under reportWallDir,
// and the head line names it: the report renders the same wall from the same
// inputs — the live gates dir among them — and writes those bytes where they
// can break nothing. The live artifact is the launch's alone, because a
// report that wrote it rewrote a RUNNING seat's wall from whatever
// environment typed the command (ranger-base-atq78: 26 FAIL in that seat's
// suite, exit 0, nothing said). TestQAReportRenderLeavesTheLiveWallAlone
// below is that half.
func TestQABwrapReportPrintsTheWallItWroteThroughTheSameMarkers(t *testing.T) {
	// No t.Parallel: bwlStubBin writes a package-level var.
	f := bwlFixture(t)
	bwlStubBin(t, "/usr/bin/bwrap")
	var b strings.Builder
	if err := f.a.BwrapReport(f.ag, f.rt, f.dir, &b); err != nil {
		t.Fatalf("BwrapReport: %v", err)
	}
	got := b.String()
	head, _, _ := strings.Cut(got, "\n")
	live := filepath.Join(f.a.GatesDir(f.ag.Name), bwrapArgsName)
	copyDir, err := f.a.reportWallDir(f.ag.Name)
	if err != nil {
		t.Fatal(err)
	}
	args := filepath.Join(copyDir, bwrapArgsName)
	if !strings.Contains(head, AbbrevHome(args)) {
		t.Errorf("the report's head line is %q and does not name the artifact: %s", head, AbbrevHome(args))
	}
	if _, err := os.Stat(args); err != nil {
		t.Errorf("the report named %s and did not write it (%v) — a head line naming a file that is not there is worse than no report", args, err)
	}
	if strings.Contains(head, AbbrevHome(live)) {
		t.Errorf("the report's head line names the LIVE wall %s — only a launch writes that, and a report that did left a running seat's git refused by its own shim (ranger-base-6s5mo): %q", AbbrevHome(live), head)
	}
	// ...and the operator is told where the live one is, or the copy's path
	// reads as the wall.
	if !strings.Contains(got, AbbrevHome(live)) {
		t.Errorf("the report never names the wall a launch reads (%s), so the copy above reads as the wall itself:\n%s", AbbrevHome(live), got)
	}
	if !strings.Contains(head, CageSeatbelt+" ("+WallBwrap+")") {
		t.Errorf("the report must say which mechanism put this wall up: %q", head)
	}
	// The markers, each over a line that is really in the lists. `w` the
	// session dir, `x` the gates deny the carve-out closes, `!` the prep the
	// launcher ran outside the namespace.
	for _, want := range []string{
		"    w " + AbbrevHome(f.dir),
		"    x " + AbbrevHome(filepath.Join(f.a.StateDir, "gates")),
		"    ! " + string(PrepTouch) + " ",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the report is missing %q:\n%s", want, got)
		}
	}
	// Seal is the one of the carve-out's four lists that renders no mount,
	// and the report says so rather than leaving a reader to wonder whether
	// it was forgotten. Read off the SAME render the report printed, so the
	// arm cannot go vacuous the day the fixture stops producing a seal.
	r, err := f.a.bwrapRender(f.ag, f.rt, f.dir, f.a.GatesDir(f.ag.Name))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Carve.Seal) == 0 {
		t.Fatal("the fixture produces no rename seal, so the arm below would assert nothing — a seal is what SeatbeltCarveOut emits for a deny whose ancestor a grant made renamable, and this fixture's state dir is one")
	}
	if !strings.Contains(got, "rename seal — NO mount") {
		t.Errorf("a rename seal renders no mount at this mechanism (a mount is a statement about a mount POINT, probe B3/B4) and the report must say so, or a reader cannot tell an ignored input from a forgotten one:\n%s", got)
	}
	for _, p := range r.Carve.Seal {
		if !strings.Contains(got, "    · "+AbbrevHome(p)) {
			t.Errorf("the report does not name the sealed %s:\n%s", AbbrevHome(p), got)
		}
	}
	// ADR 0015 §2's claim, checked and not restated — the shared block, so
	// the two mechanisms cannot answer it differently.
	if !strings.Contains(got, "in no grant above") && !strings.Contains(got, "GRANT REACHES THE CONSTITUTION") {
		t.Errorf("the report must carry the constitution block, which is the one part of it both mechanisms share:\n%s", got)
	}
	// SBPL's own vocabulary is absent, because this mechanism cannot make
	// those statements: no `(subpath …)` grant, no sibling regex, no
	// create-only ref dir.
	for _, nope := range []string{"atomic-write siblings", "create only"} {
		if strings.Contains(got, nope) {
			t.Errorf("the bwrap report says %q, which is a statement a mount cannot make:\n%s", nope, got)
		}
	}
}

// WallReport on THIS host is SeatbeltReport, byte for byte. The dispatch must
// not have changed the darwin output: `posse gates` is read by hand on this
// box every day, and a diff in it is a diff nobody asked for.
func TestQAWallReportIsSeatbeltReportOnTheSandboxExecMechanism(t *testing.T) {
	t.Parallel()
	if WallMechanism() != WallSandboxExec {
		t.Skip("NOTHING MEASURED: this host's mechanism is not " + WallSandboxExec)
	}
	f := bwlFixture(t)
	var viaWall, viaSeatbelt strings.Builder
	if err := f.a.WallReport(f.ag, f.rt, f.dir, &viaWall); err != nil {
		t.Fatal(err)
	}
	if err := f.a.SeatbeltReport(f.ag, f.dir, &viaSeatbelt, f.rt.StateDirs...); err != nil {
		t.Fatal(err)
	}
	if viaWall.String() != viaSeatbelt.String() {
		t.Errorf("WallReport and SeatbeltReport disagree on %s:\n--- WallReport\n%s\n--- SeatbeltReport\n%s", runtime.GOOS, viaWall.String(), viaSeatbelt.String())
	}
}

// ─── D17's residual, closed (ranger-base-qoamy §4) ──────────────────────────

// The four literals `SeatbeltWritable` spelled unconditionally are per GOOS
// now, and the claim that earns that change is that DARWIN IS UNTOUCHED. It
// is checkable rather than arguable: `add` absResolves, `/tmp` IS
// `/private/tmp` on darwin, and `dedupeStrings` folded the two already — so
// the literal this bead guarded was redundant on the host it was written for.
//
// WHAT IT COST ON LINUX, and why the two halves differ. A grant that is not
// there is a `mkdir -p` the launcher owes, OUTSIDE the namespace and as the
// seat's user. For the three under the home that is three junk directories
// posse has no business making; for `/private/tmp` it is a mkdir at the
// filesystem root, which an unprivileged launcher cannot make at all — a
// refused launch for every seat on the host.
// The blanket temp grants read directly, both GOOSes from either host, the
// way homeCacheGrants' arms are read below. It is also the list sbRoot takes
// away to give its fixtures a root no grant reaches (sbNoBlanketTempGrants,
// ranger-base-u8xk4), so a member added here is a member that could satisfy
// every coverage pin in the package — which is the defect that bead was
// filed for, one spelling over.
func TestQASeatbeltTempGrantsAreTheThreeBlanketOnes(t *testing.T) {
	// No t.Parallel: seatbeltTempGrants is a package var since
	// ranger-base-u8xk4 — sbRoot stubs it for the tests that need a
	// fixture no blanket grant reaches — and this test READS it. Top-level
	// parallel tests resume only after every sequential test has run, so a
	// serial stub cannot overlap a parallel reader; staying sequential is
	// what keeps that true without anyone having to know it.
	for _, tc := range []struct {
		goos, tmpdir string
		want         []string
	}{
		{"darwin", "/var/folders/xx/T", []string{"/var/folders/xx/T", "/private/tmp", "/tmp"}},
		{"darwin", "", []string{"/private/tmp", "/tmp"}},
		{"linux", "/var/tmp/seat", []string{"/var/tmp/seat", "/tmp"}},
		// No `/private` on Linux: the grant renders a `mkdir -p` at the
		// filesystem root that an unprivileged launcher cannot make, which
		// is a REFUSED launch for every seat on the host and the whole
		// reason this is a parameter (ADR 0080 D17's residual).
		{"linux", "", []string{"/tmp"}},
	} {
		got := seatbeltTempGrants(tc.goos, tc.tmpdir)
		if strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("seatbeltTempGrants(%s, %q) = %v, want %v (order included — a grant set renders in list order into the allow block)", tc.goos, tc.tmpdir, got, tc.want)
		}
	}
}

func TestQASeatbeltWritableTmpGrantIsOneDirectoryOnDarwin(t *testing.T) {
	t.Parallel()
	f := bwlFixture(t)
	w := f.a.SeatbeltWritable(f.ag, f.dir, f.a.GatesDir(f.ag.Name))
	if runtime.GOOS == "darwin" {
		n := 0
		for _, p := range w {
			if p == "/tmp" || p == "/private/tmp" {
				n++
			}
		}
		if n != 1 {
			t.Errorf("the tmp grant is %d entries in the writable set, want 1 — on darwin /tmp resolves to /private/tmp, so both literals fold to one and guarding the second changes this profile not at all: %v", n, w)
		}
		if !containsString(w, absResolve("/tmp")) {
			t.Errorf("the writable set names no tmp directory at all: %v", w)
		}
	}
	// And the list under the home, per GOOS. The darwin ORDER is the one it
	// had before it was a function: a grant set renders in list order into
	// the allow block, so reordering it rewrites every seat's seatbelt.sb
	// for a change that means nothing.
	want := []string{"Library/Logs", ".cache/posse", "Library/Application Support/go/telemetry/local"}
	got := homeCacheGrants("darwin")
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("homeCacheGrants(darwin) = %v, want %v (order included)", got, want)
	}
	// The Linux arm, read from darwin — which is the whole reason it is a
	// parameter. No darwin path, nothing absolute, and the Go toolchain's
	// counters at their own `os.UserConfigDir()` spelling: `local` and never
	// its parent, because `telemetry/mode` beside it is what decides whether
	// the box UPLOADS them (crew guardrail 4).
	lin := homeCacheGrants("linux")
	for _, d := range lin {
		if strings.HasPrefix(d, "Library/") {
			t.Errorf("homeCacheGrants(linux) names %q — a darwin path, which on Linux is a junk directory the launcher would `mkdir -p` in the seat's home", d)
		}
		if filepath.IsAbs(d) {
			t.Errorf("homeCacheGrants(linux) names the absolute %q; every entry is joined to the home", d)
		}
	}
	if !containsString(lin, ".config/go/telemetry/local") {
		t.Errorf("homeCacheGrants(linux) = %v and does not name the toolchain's telemetry counters at their Linux spelling — 18%% of one day's denial volume before that grant existed (ranger-base-gr3ow)", lin)
	}
	for _, d := range lin {
		if d == ".config/go/telemetry" || d == ".config" {
			t.Errorf("homeCacheGrants(linux) names %q — granting the parent of `local` would let a session flip the operator's box from telemetry mode `local` to `on`, an egress the operator never chose", d)
		}
	}
	// The shared entry is shared, so a reader comparing two hosts' walls is
	// comparing the same lock dir. `.cache` and `.npm` whole are in neither
	// since ranger-base-olr0g (wallseatcache_qa_test.go).
	for _, d := range []string{".cache/posse"} {
		if !containsString(homeCacheGrants("darwin"), d) || !containsString(lin, d) {
			t.Errorf("%q must be in both GOOS lists — it is posse's own lock dir on both", d)
		}
	}
	// And XDG_DATA_HOME is in NEITHER (ranger-base-6akzw): it holds installed
	// software — mise's command wrappers lead every seat's PATH — so a grant
	// over it is a seat replacing code every other seat runs.
	for _, goos := range []string{"darwin", "linux"} {
		for _, d := range homeCacheGrants(goos) {
			if d == ".local" || d == ".local/share" || strings.HasPrefix(d, ".local/share/") {
				t.Errorf("homeCacheGrants(%s) names %q — XDG_DATA_HOME holds installed code that runs outside the wall (ranger-base-y438o)", goos, d)
			}
		}
	}
}
