//go:build posse_arm3

package posse

// The ADR 0032 §1 probe contract, pinned where it can be pinned without a
// CLI: the four observables (evalProbe), the record, and the state machine
// that decides whether a recorded probe still describes the installed
// binary. The live half — a real pane on a real runtime — is
// runtimeprobe_live_test.go.
//
// Every observable is pinned with BOTH arms. A probe whose wrong arm passes
// measures nothing, and this is a file full of probes.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// passingReading is a reading in which all four observables hold. Each test
// below mutates exactly one field of it, so a green row can only come from
// the thing that row is about.
func passingReading(binDir string) probeReading {
	return probeReading{
		Canary:     "uname",
		BinDir:     binDir,
		Exe:        "bob",
		Where:      filepath.Join(binDir, "uname") + "\n",
		WhereFound: true,
		Refusals: "2026-08-28T00:00:00Z uname " + ProbeShapeDirect + " (deny: Bash(uname:*))\n" +
			"2026-08-28T00:00:01Z uname " + ProbeShapeShellC + " (deny: Bash(uname:*))\n" +
			"2026-08-28T00:00:02Z uname " + ProbeShapeScript + " (deny: Bash(uname:*))\n",
		Settled:   "idle",
		AgentKind: "bob",
		Detection: AgentDetection{State: "idle", VisibleIdle: true},
	}
}

func obs(t *testing.T, r probeReading, n int) ProbeObservable {
	t.Helper()
	got := evalProbe(r)
	if len(got) != ProbeObservableCount {
		t.Fatalf("evalProbe returned %d observables, want %d (the contract is four rows; a short record is not a pass)", len(got), ProbeObservableCount)
	}
	for _, o := range got {
		if o.N == n {
			return o
		}
	}
	t.Fatalf("no observable %d in %+v", n, got)
	return ProbeObservable{}
}

func TestProbeObservablesHoldOnAPassingReading(t *testing.T) {
	t.Parallel()
	bin := "/tmp/gates/bin"
	for _, o := range evalProbe(passingReading(bin)) {
		if !o.OK {
			t.Errorf("observable %d %s must hold on a passing reading: %s", o.N, o.Name, o.Detail)
		}
		if o.Detail == "" {
			t.Errorf("observable %d %s carries no detail — a row with no evidence is a claim", o.N, o.Name)
		}
	}
}

// Observable 1 is the one the whole ADR turns on: the SILENT failure (b), a
// CLI that re-execs a login shell it did not take from $SHELL, so
// path_helper puts /usr/bin in front of the gates dir and the shim never
// runs. The wrong arm has to fail, and it has to fail with the real binary's
// path in it — an operator who cannot see WHERE the lookup landed cannot
// tell this from "the gates dir is missing".
func TestProbeShimPrecedenceFailsWhenTheRealBinaryWins(t *testing.T) {
	t.Parallel()
	bin := "/tmp/gates/bin"
	r := passingReading(bin)
	r.Where = "/usr/bin/uname\n"
	o := obs(t, r, 1)
	if o.OK {
		t.Fatal("command -v landing on /usr/bin/uname means the L1 shim is BEHIND the real binary — every Bash(...) deny on this runtime is decoration, and this observable must fail")
	}
	if !strings.Contains(o.Detail, "/usr/bin/uname") || !strings.Contains(o.Detail, filepath.Join(bin, "uname")) {
		t.Errorf("the failure must name what was found and what was wanted: %q", o.Detail)
	}

	// Nothing found at all is a different diagnosis (the gates dir is not on
	// PATH), and it must not read as the demotion above.
	r.Where = "\n"
	if o := obs(t, r, 1); o.OK || !strings.Contains(o.Detail, "not on the session's PATH") {
		t.Errorf("an empty lookup is 'the gates dir is not on PATH': ok=%v %q", o.OK, o.Detail)
	}

	// The turn never ran: UNMEASURED, and it must say so rather than
	// reporting a wall that failed. Wrong either way is a wrong next move —
	// one sends the operator to their CLI's shell handling, the other to
	// their prompt.
	r = passingReading(bin)
	r.WhereFound, r.Where = false, ""
	if o := obs(t, r, 1); o.OK || !strings.Contains(o.Detail, "UNMEASURED") {
		t.Errorf("no lookup file is unmeasured, not disproved: ok=%v %q", o.OK, o.Detail)
	}
}

// Observable 2 is per SHAPE, not a count. "three refusals in the log" is
// also what one shape firing three times looks like, and the shapes are the
// whole point of ADR 0009's argv table.
func TestProbeRefusalIsPinnedPerSubprocessShape(t *testing.T) {
	t.Parallel()
	bin := "/tmp/gates/bin"
	for _, tc := range []struct{ drop, want string }{
		{ProbeShapeDirect, "direct"},
		{ProbeShapeShellC, "sh -c"},
		{ProbeShapeScript, "executable script"},
	} {
		r := passingReading(bin)
		r.Refusals = strings.ReplaceAll(r.Refusals, tc.drop, "something-else")
		o := obs(t, r, 2)
		if o.OK {
			t.Errorf("dropping the %s refusal must fail observable 2 — a wall that holds for two shapes of three is not the wall ADR 0009 measured", tc.want)
		}
		if !strings.Contains(o.Detail, tc.want) {
			t.Errorf("the failure must name the shape that did not reach the log: %q", o.Detail)
		}
	}
	// Three refusals through ONE shape is the false pass a count would give.
	r := passingReading(bin)
	r.Refusals = strings.Repeat("uname "+ProbeShapeDirect+"\n", 3)
	if o := obs(t, r, 2); o.OK {
		t.Error("one shape three times is not three shapes — a count-based observable would pass here")
	}
}

// Observable 3 has two halves and needs both: a pane that settles having run
// nothing is a CLI that answered in prose, or a dialog nobody is watching.
func TestProbeUnattendedTurnNeedsBothSettleAndAction(t *testing.T) {
	t.Parallel()
	bin := "/tmp/gates/bin"
	r := passingReading(bin)
	r.Settled, r.SettleWhy = "", "herdr never saw a screen it recognizes"
	o := obs(t, r, 3)
	if o.OK || !strings.Contains(o.Detail, "herdr never saw") {
		t.Errorf("no settle is no completed turn, and the reason travels with it: ok=%v %q", o.OK, o.Detail)
	}

	r = passingReading(bin)
	r.WhereFound, r.Where = false, ""
	if o := obs(t, r, 3); o.OK {
		t.Error("a pane that settled having run none of the commands must not read as an unattended turn")
	}

	// And the passing arm has to SAY that posse sent nothing, because that
	// is the claim: ADR 0013 §2 forbids answering an interstitial, and an
	// observable that asserts unattendedness without its own precondition
	// is asserting it about a run it did not watch.
	if o := obs(t, passingReading(bin), 3); !strings.Contains(o.Detail, "0 keystrokes") {
		t.Errorf("the unattended row must state the keystroke count: %q", o.Detail)
	}
}

// Observable 4: herdr's idle FALLBACK is the trap. It answers `idle` for any
// pane holding a known agent, with no rule matched — dispatch is blind on a
// runtime that only ever produces that, because every settled state is a
// guess. Seen() is the repo's positive-evidence predicate.
func TestProbeHerdrDetectionRejectsTheIdleFallback(t *testing.T) {
	t.Parallel()
	bin := "/tmp/gates/bin"
	r := passingReading(bin)
	r.Detection = AgentDetection{State: "idle", FallbackReason: "default_known_agent_idle_fallback"}
	o := obs(t, r, 4)
	if o.OK {
		t.Fatal("a guessed idle is not detection — with only the fallback, `working` and every settled state on this runtime are guesses")
	}
	if !strings.Contains(o.Detail, "default_known_agent_idle_fallback") {
		t.Errorf("the failure must carry herdr's own reason: %q", o.Detail)
	}
	// A matched rule is evidence too, and stronger than chrome.
	r.Detection = AgentDetection{State: "idle"}
	r.Detection.Rule.ID = "live_prompt_box"
	if o := obs(t, r, 4); !o.OK || !strings.Contains(o.Detail, "live_prompt_box") {
		t.Errorf("a matched rule is positive evidence: ok=%v %q", o.OK, o.Detail)
	}

	r = passingReading(bin)
	r.AgentKind = ""
	if o := obs(t, r, 4); o.OK || !strings.Contains(o.Detail, detectionDoc) {
		t.Errorf("agent_not_found must fail and point at the manifest doc: ok=%v %q", o.OK, o.Detail)
	}

	r = passingReading(bin)
	r.AgentKind = "claude"
	if o := obs(t, r, 4); o.OK || !strings.Contains(o.Detail, "not bob") {
		t.Errorf("herdr naming another agent's kind must fail: ok=%v %q", o.OK, o.Detail)
	}
}

func TestProbeRecordPassedNeedsEveryObservable(t *testing.T) {
	t.Parallel()
	full := evalProbe(passingReading("/tmp/gates/bin"))
	if r := (&ProbeRecord{Observables: full}); !r.Passed() {
		t.Error("four green observables is a pass")
	}
	// The one that matters: a SHORT record — written by an older posse, or
	// by a probe that stopped early — has no failures either, and must not
	// be read as a pass.
	if r := (&ProbeRecord{Observables: full[:3]}); r.Passed() {
		t.Error("a record carrying three of four observables is not a pass: the fourth was never measured")
	}
	if r := (&ProbeRecord{}); r.Passed() {
		t.Error("a record with no observables at all is not a pass")
	}
	var nilRec *ProbeRecord
	if nilRec.Passed() {
		t.Error("no record is not a pass")
	}
	bad := append([]ProbeObservable(nil), full...)
	bad[2].OK, bad[2].Detail = false, "the turn did not complete"
	r := &ProbeRecord{Observables: bad}
	if r.Passed() {
		t.Error("one red observable fails the record")
	}
	if f := r.Failures(); len(f) != 1 || !strings.Contains(f[0], "the turn did not complete") {
		t.Errorf("Failures must name the row and its detail: %v", f)
	}
}

// probeApp is an App whose state dir is a temp dir, so nothing here reads or
// writes the operator's live RHQ_HOME (the class in ranger-base-gvrh).
func probeApp(t *testing.T) *App {
	t.Helper()
	home := t.TempDir()
	return &App{Home: home, StateDir: filepath.Join(home, "state"), AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
}

func TestProbeRecordRoundTrips(t *testing.T) {
	t.Parallel()
	a := probeApp(t)
	want := &ProbeRecord{
		Runtime: "bob", CLIPath: "/opt/homebrew/bin/bob", LauncherPath: "/usr/local/bin/bob",
		Version: "bob 1.2.3",
		Date:    time.Now().UTC().Truncate(time.Second), PosseVersion: Version, Canary: "uname",
		Observables: evalProbe(passingReading("/tmp/gates/bin")),
	}
	if err := a.WriteProbeRecord(want); err != nil {
		t.Fatal(err)
	}
	got, err := a.ReadProbeRecord("bob")
	if err != nil || got == nil {
		t.Fatalf("read back: %v %v", got, err)
	}
	if !got.Passed() || got.Version != want.Version || got.CLIPath != want.CLIPath || !got.Date.Equal(want.Date) {
		t.Errorf("round trip lost something: %+v", got)
	}
	// Both paths, separately. They are two answers to "which binary" and a
	// round trip that folded them into one would put the record back where
	// ranger-base-385x found it.
	if got.LauncherPath != want.LauncherPath || got.LauncherPath == got.CLIPath {
		t.Errorf("launcher_cli_path must survive the round trip beside cli_path: %+v", got)
	}
	// A record that cannot be parsed is an ERROR, not an absence: absence is
	// the state the operator is told to fix by probing, so treating a
	// corrupt file as absent would let the next pass overwrite it having
	// never noticed.
	if err := os.WriteFile(a.ProbeRecordPath("bob"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rec, err := a.ReadProbeRecord("bob"); err == nil || rec != nil {
		t.Errorf("a corrupt record must be an error, not (nil, nil): %v %v", rec, err)
	}
	// And a missing one is (nil, nil) — "nobody probed" is an answer.
	if rec, err := a.ReadProbeRecord("nobody"); err != nil || rec != nil {
		t.Errorf("an unprobed runtime is (nil, nil): %v %v", rec, err)
	}
}

// The drift machine of ADR 0032 §1 rule 1: a record is CURRENT only while it
// describes the binary that is installed now. Every arm is driven through
// the injected reader seam, never through the real PATH — the seam is the
// READER, not the permission to read (ranger-base-02zr).
func TestProbeStateDriftAndCurrency(t *testing.T) {
	t.Parallel()
	a := probeApp(t)
	rt := &Runtime{Name: "bob", Command: "bob --pid {file}"}
	at := func(path, version string) (func(string) string, func(string) string) {
		return func(string) string { return path }, func(string) string { return version }
	}
	resolve, version := at("/usr/local/bin/bob", "bob 1.2.3")

	// 1. Nothing recorded — loud, and it names the command that fixes it.
	st := a.probeStateWith(rt, resolve, version)
	if st.Current || st.Drift || !strings.Contains(st.Why, "posse runtime probe bob") {
		t.Errorf("unprobed: %+v", st)
	}

	// 2. A recorded FAILURE is not the same fact as no record, and the
	// remedy differs: fix the runtime, then re-probe.
	failed := evalProbe(passingReading("/tmp/gates/bin"))
	failed[0] = ProbeObservable{1, "shim-precedence", false, "command -v uname → /usr/bin/uname"}
	rec := &ProbeRecord{Runtime: "bob", CLIPath: "/usr/local/bin/bob", LauncherPath: "/usr/local/bin/bob", Version: "bob 1.2.3", Date: time.Now().UTC(), Observables: failed}
	if err := a.WriteProbeRecord(rec); err != nil {
		t.Fatal(err)
	}
	st = a.probeStateWith(rt, resolve, version)
	if st.Current || !strings.Contains(st.Why, "FAILED") || !strings.Contains(st.Why, "/usr/bin/uname") {
		t.Errorf("a recorded failure must say so, and carry the observable that failed: %+v", st)
	}

	// 3. A passing record on the installed binary is CURRENT — the arm that
	// unlocks the parity claim.
	rec.Observables = evalProbe(passingReading("/tmp/gates/bin"))
	if err := a.WriteProbeRecord(rec); err != nil {
		t.Fatal(err)
	}
	if st := a.probeStateWith(rt, resolve, version); !st.Current || st.Drift {
		t.Errorf("a passing record on the same binary at the same version is current: %+v", st)
	}

	// 4. Version drift: same path, the CLI moved under the record.
	_, newer := at("", "bob 1.3.0")
	st = a.probeStateWith(rt, resolve, newer)
	if st.Current || !st.Drift || !strings.Contains(st.Why, "bob 1.3.0") || !strings.Contains(st.Why, "bob 1.2.3") {
		t.Errorf("version drift must un-current the record and name both versions: %+v", st)
	}

	// 5. Path drift: a DIFFERENT binary answers to the same name now. Not
	// covered by the version check — two builds can print the same string.
	elsewhere, _ := at("/opt/homebrew/bin/bob", "")
	st = a.probeStateWith(rt, elsewhere, version)
	if st.Current || !st.Drift || !strings.Contains(st.Why, "/opt/homebrew/bin/bob") {
		t.Errorf("path drift must un-current the record: %+v", st)
	}

	// 6. A CLI that prints no version at all: the record still counts (a
	// live measurement happened) but the surface must SAY drift is
	// undetectable rather than quietly reading unknown as unchanged.
	rec.Version = ""
	if err := a.WriteProbeRecord(rec); err != nil {
		t.Fatal(err)
	}
	st = a.probeStateWith(rt, resolve, func(string) string { return "bob 9.9.9" })
	if !st.Current || !strings.Contains(st.Why, "UNKNOWN") {
		t.Errorf("an unreadable version keeps the record but must be loud about the drift check it is not doing: %+v", st)
	}

	// 7. And the version reader answering "" now (the CLI stopped talking)
	// must not read as drift — an unknown is not a difference.
	rec.Version = "bob 1.2.3"
	if err := a.WriteProbeRecord(rec); err != nil {
		t.Fatal(err)
	}
	if st := a.probeStateWith(rt, resolve, func(string) string { return "" }); !st.Current || st.Drift {
		t.Errorf("an unreadable installed version is unknown, not drift: %+v", st)
	}
}

// The canary must exist BOTH in the gates dir and in a system dir, or
// observable 1 is a tautology: a command that lives only under gates/bin
// resolves there whatever PATH says, which is exactly the runtime the probe
// exists to catch.
func TestProbeCanaryResolvesOutsideTheGates(t *testing.T) {
	t.Parallel()
	name, path := probeCanary()
	if name == "" {
		t.Skip("no probe canary on this host — the probe would refuse here too")
	}
	if !strings.HasSuffix(path, string(filepath.Separator)+name) {
		t.Errorf("canary %q resolved to %q", name, path)
	}
	if strings.Contains(path, string(filepath.Separator)+"gates"+string(filepath.Separator)) {
		t.Errorf("canary %q resolved INSIDE a gates dir (%s) — observable 1 would be a tautology", name, path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("canary path %s does not exist: %v", path, err)
	}
}

// The prompt is the delivery vehicle for all three shapes and for the lookup
// that observable 1 reads. A prompt that dropped one would make its
// observable fail on a runtime that is fine.
func TestProbePromptCarriesEveryShapeAndTheLookup(t *testing.T) {
	t.Parallel()
	p := probePrompt("uname", "/tmp/w/where.txt", "/tmp/w/probe.sh")
	for _, want := range []string{
		"command -v uname > /tmp/w/where.txt",
		"uname " + ProbeShapeDirect,
		"sh -c 'uname " + ProbeShapeShellC + "'",
		"/tmp/w/probe.sh",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("probe prompt is missing %q:\n%s", want, p)
		}
	}
	// And it has to tell the model the refusals are expected, or a CLI that
	// stops at the first one measures one shape of three.
	if !strings.Contains(p, "EXPECTED to be refused") {
		t.Errorf("the prompt must say the refusals are the measurement:\n%s", p)
	}
}

func TestReadFromReturnsOnlyTheDelta(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "refusals.log")
	if err := os.WriteFile(p, []byte("old line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	at := fileSize(p)
	if err := os.WriteFile(p, []byte("old line\nnew line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The delta is the whole point: refusals.log is kept across renders, so
	// a stale line from an earlier probe would answer for this one.
	if got := readFrom(p, at); got != "new line\n" {
		t.Errorf("readFrom must return only what was appended: %q", got)
	}
	// A log that SHRANK (rotated, or deleted and recreated) reads from the
	// start rather than returning nothing.
	if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readFrom(p, at); got != "x\n" {
		t.Errorf("a shrunken log reads whole: %q", got)
	}
	if got := readFrom(filepath.Join(t.TempDir(), "nope"), 0); got != "" {
		t.Errorf("a missing log is empty: %q", got)
	}
}

// ─── which binary the record names (ranger-base-385x) ────────────────────────

// fakeProbeHerdr is a herdr whose `pane run` really runs the line, in an
// environment the caller controls — the daemon's, not this process's. body
// is the script's whole behaviour, so an arm can also make the pane answer
// nothing at all.
func fakeProbeHerdr(t *testing.T, body string) Herdr {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "herdr")
	// WriteExecutable, not os.WriteFile: this file is exec'd within
	// microseconds of being written, by a test that runs beside hundreds of
	// others that fork. os.WriteFile leaves a window for one of those forks
	// to inherit its write descriptor, and Linux answers an execve inside
	// that window with ETXTBSY — which is what red ubuntu-latest in ci.yml
	// run 34002511879, 1 of the 6 runs in that streak
	// (execwrite.go, ranger-base-d26ak).
	if err := WriteExecutable(bin, []byte("#!/bin/sh\n"+body+"\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return Herdr{Bin: bin}
}

// script writes an executable that prints one line, and returns its path.
func probeFakeCLI(t *testing.T, dir, name, says string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	// Same window, same reason as fakeProbeHerdr above: the probe resolves
	// this file and the pane execs it immediately.
	if err := WriteExecutable(p, []byte("#!/bin/sh\necho \""+says+"\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// The defect this bead is: the pane resolves the CLI in the herdr DAEMON's
// PATH and posse resolved it in its own, so the record certified a binary
// nobody probed. The witness has to come from the pane, and it has to be the
// pane's answer even when this process would have said something else.
func TestProbeSessionExeIsThePanesAnswerAndNotTheLaunchers(t *testing.T) {
	dir := t.TempDir()
	srv := filepath.Join(dir, "srvbin") // only the "daemon" has this
	cli := filepath.Join(dir, "clibin") // only posse has this
	bin := filepath.Join(dir, "gatesbin")
	srvExe := probeFakeCLI(t, srv, "bob", "server copy")
	probeFakeCLI(t, cli, "bob", "launcher copy")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", cli+string(os.PathListSeparator)+os.Getenv("PATH"))

	h := fakeProbeHerdr(t, `[ "$1" = pane ] && [ "$2" = run ] || exit 0
PATH="`+srv+`:$PATH" /bin/sh -c "$4"`)
	got, err := probeSessionExe(h, "%1", bin, "bob", filepath.Join(dir, "cli.txt"), 10*time.Second)
	if err != nil {
		t.Fatalf("the pane would not answer: %v", err)
	}
	if got != srvExe {
		t.Errorf("cli_path must be the SESSION's answer: got %q, want %q", got, srvExe)
	}
	// Two-way, or the arm passes on a rig where both sides happen to agree:
	// posse's own lookup names the other file, and that is the answer the
	// record used to carry.
	if out := resolveOutside("bob", ""); out == got {
		t.Fatalf("the rig proves nothing: posse's own PATH resolves bob to %q too", out)
	} else if out != filepath.Join(cli, "bob") {
		t.Fatalf("the rig is not set up: posse resolves bob to %q", out)
	}
}

// The lookup is typed with the launch line's own PATH prefix, so a shim in
// the gates bin dir that shadowed the CLI is what the session would run —
// and the record has to say that, not what the CLI would have been without
// the wall in front of it.
func TestProbeSessionExeSeesThroughTheGatePrefix(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	srv := filepath.Join(dir, "srvbin")
	bin := filepath.Join(dir, "gatesbin")
	probeFakeCLI(t, srv, "bob", "server copy")
	shim := probeFakeCLI(t, bin, "bob", "the shim")
	h := fakeProbeHerdr(t, `[ "$1" = pane ] && [ "$2" = run ] || exit 0
PATH="`+srv+`:$PATH" /bin/sh -c "$4"`)
	got, err := probeSessionExe(h, "%1", bin, "bob", filepath.Join(dir, "cli.txt"), 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got != shim {
		t.Errorf("the gates bin dir leads the launch line's PATH, so it leads this lookup too: got %q, want %q", got, shim)
	}
}

// A pane that cannot say what it would launch gets no record at all. Each
// arm is a different silence, and each has to be told apart in the message:
// the operator's next move differs.
func TestProbeSessionExeRefusesWhatItCannotName(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body, want string
	}{
		// Nothing named bob exists on any PATH the pane has, so the
		// lookup writes an empty answer — which the `.part` rename tells
		// apart from a redirect this read got to before the shell did.
		{"resolves nothing",
			`[ "$1" = pane ] && [ "$2" = run ] || exit 0
/bin/sh -c "$4"`,
			"herdr daemon's PATH"},
		// An alias or a shell function in the pane: `command -v` answers,
		// and its answer is not a file anyone can record.
		{"answers something that is not a path",
			`[ "$1" = pane ] && [ "$2" = run ] || exit 0
/bin/sh -c "$(printf '%s' "$4" | sed 's|command -v [^ ]*|echo alias-for-bob|')"`,
			"not a path to a binary"},
		{"never runs the lookup", `exit 0`, "did not say which binary"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			h := fakeProbeHerdr(t, tc.body)
			got, err := probeSessionExe(h, "%1", filepath.Join(dir, "bin"), "bob", filepath.Join(dir, "cli.txt"), 300*time.Millisecond)
			if err == nil {
				t.Fatalf("a probe that cannot name its binary must refuse; got %q", got)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("the refusal must say which silence this was: %v", err)
			}
		})
	}
}

// probeLaunchExe is the word the pane's shell resolves. Exe() is the name
// the drift check re-resolves later, and the probe refuses when they are not
// the same command — two paths in one record that answer about two different
// names would make "the two sides agreed" meaningless.
func TestProbeLaunchExeIsTheFirstWordOfTheRenderedLine(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ line, want string }{
		{"codex exec --pid '/tmp/x.md'", "codex"},
		{"/opt/homebrew/bin/codex exec", "/opt/homebrew/bin/codex"},
		{"  claude  --pid x", "claude"},
		{"", ""},
	} {
		if got := probeLaunchExe(tc.line); got != tc.want {
			t.Errorf("probeLaunchExe(%q) = %q, want %q", tc.line, got, tc.want)
		}
	}
}

// The version in the record is the measured binary's own. Reading it by NAME
// re-resolves on posse's PATH, which is the side that named the wrong binary
// in the first place.
func TestReadCLIVersionAtDoesNotReResolveTheName(t *testing.T) {
	dir := t.TempDir()
	onPath := filepath.Join(dir, "onpath")
	elsewhere := filepath.Join(dir, "elsewhere")
	probeFakeCLI(t, onPath, "bobv", "bobv 1.0.0-on-path")
	measured := probeFakeCLI(t, elsewhere, "bobv", "bobv 2.0.0-measured")
	t.Setenv("PATH", onPath+string(os.PathListSeparator)+os.Getenv("PATH"))
	probeVersions.Delete("bobv")

	if got := readCLIVersionAt(measured); got != "bobv 2.0.0-measured" {
		t.Errorf("readCLIVersionAt must run the path it was given: %q", got)
	}
	// The two-way half: reading by NAME really does answer differently here,
	// so the arm above is not passing on a box where both are the same file.
	if got := readCLIVersion("bobv"); got != "bobv 1.0.0-on-path" {
		t.Errorf("the rig proves nothing unless the name resolves elsewhere: %q", got)
	}
	if readCLIVersionAt("") != "" {
		t.Error("no path is no version")
	}
}

// A record written before the probe read the session's own resolution
// carries a cli_path from the launcher's side, over observables measured on
// the pane's. It cannot be told which of the two it holds, so it is not
// current — the ranger-base-385x state is re-probed, never trusted.
func TestProbeStateRefusesARecordThatCannotNameTheSessionsBinary(t *testing.T) {
	t.Parallel()
	a := probeApp(t)
	rt := &Runtime{Name: "bob", Command: "bob --pid {file}"}
	resolve := func(string) string { return "/usr/local/bin/bob" }
	version := func(string) string { return "bob 1.2.3" }

	for _, tc := range []struct{ name, cli, launcher string }{
		{"written before the probe asked the pane", "/usr/local/bin/bob", ""},
		{"the pane never answered", "", "/usr/local/bin/bob"},
	} {
		rec := &ProbeRecord{
			Runtime: "bob", CLIPath: tc.cli, LauncherPath: tc.launcher, Version: "bob 1.2.3",
			Date: time.Now().UTC(), Observables: evalProbe(passingReading("/tmp/gates/bin")),
		}
		if err := a.WriteProbeRecord(rec); err != nil {
			t.Fatal(err)
		}
		st := a.probeStateWith(rt, resolve, version)
		if st.Current || !strings.Contains(st.Why, "posse runtime probe bob") {
			t.Errorf("%s: a record that cannot name its binary is not current: %+v", tc.name, st)
		}
	}
}

// Drift is a like-for-like comparison. The reader resolves on POSSE's PATH,
// so it is compared against launcher_cli_path; comparing it against the
// SESSION's path would report drift forever on exactly the boxes where the
// two PATHs differ, which is the divergence the record exists to record.
func TestProbeStateComparesTheLauncherSideAgainstTheLauncherSide(t *testing.T) {
	t.Parallel()
	a := probeApp(t)
	rt := &Runtime{Name: "bob", Command: "bob --pid {file}"}
	rec := &ProbeRecord{
		Runtime: "bob", CLIPath: "/opt/daemon/bin/bob", LauncherPath: "/usr/local/bin/bob",
		Version: "bob 1.2.3", Date: time.Now().UTC(),
		Observables: evalProbe(passingReading("/tmp/gates/bin")),
	}
	if err := a.WriteProbeRecord(rec); err != nil {
		t.Fatal(err)
	}
	unmoved := func(string) string { return "/usr/local/bin/bob" }

	// Nothing has moved on posse's side, so nothing has drifted — even
	// though the version reader, which can only reach posse's binary,
	// answers something else entirely. That version belongs to another file.
	st := a.probeStateWith(rt, unmoved, func(string) string { return "bob 9.9.9" })
	if st.Drift {
		t.Errorf("two PATHs naming two binaries is not drift, or a divergent box re-probes forever: %+v", st)
	}
	if !st.Current || !strings.Contains(st.Why, "cannot be checked") || !strings.Contains(st.Why, "/opt/daemon/bin/bob") {
		t.Errorf("it must name the measured binary and say the version check it is NOT doing: %+v", st)
	}
	// And when posse's own side really does move, that IS drift, named from
	// the recorded launcher path and not from the measured one.
	st = a.probeStateWith(rt, func(string) string { return "/opt/homebrew/bin/bob" }, unmoved)
	if !st.Drift || !strings.Contains(st.Why, "/usr/local/bin/bob") || !strings.Contains(st.Why, "/opt/homebrew/bin/bob") {
		t.Errorf("a moved launcher binary is drift, and the line names both sides: %+v", st)
	}
	// A record whose launcher side resolved nothing at probe time keeps the
	// sentinel out of the comparison: "" now and "(none…)" then is not a
	// move, it is the same absence.
	rec.LauncherPath = ProbeExeUnresolved
	if err := a.WriteProbeRecord(rec); err != nil {
		t.Fatal(err)
	}
	if st := a.probeStateWith(rt, func(string) string { return "" }, unmoved); st.Drift || !st.Current {
		t.Errorf("an exe posse never had is not a binary that moved: %+v", st)
	}
	// Nor is one APPEARING on posse's PATH. The record's subject is the
	// binary the session resolved, which this says nothing about — and the
	// sentinel must not be compared as if it were a path, or that appearance
	// reads as a move from "(none…)" to a real file.
	if st := a.probeStateWith(rt, unmoved, unmoved); st.Drift || !st.Current {
		t.Errorf("a binary appearing on posse's PATH is not drift on the session's: %+v", st)
	}
}

// cli_path and launcher_cli_path have to be answers about the SAME command
// name, or "the two sides agreed" — the gate on the whole drift check — is
// comparing apples to a different fruit. It holds by construction, and this
// is the reading of it: the probe's own PID carries no command: of its own,
// so the rendered line starts the runtime template's first word and Exe() is
// that word's basename.
func TestProbeLaunchExeAnswersTheNameTheDriftCheckReResolves(t *testing.T) {
	t.Parallel()
	a := probeApp(t)
	for _, cmd := range []string{
		"bob --pid {file}",
		"/opt/homebrew/bin/bob exec --pid {file} --memory {memory}",
		"bob",
	} {
		rt := &Runtime{Name: "bob", Command: cmd}
		ag, err := a.writeProbePID(probeAgentName(rt.Name), rt, t.TempDir(), "uname")
		if err != nil {
			t.Fatal(err)
		}
		inner := ag.RenderCommandFor(rt, rt.Name, DefaultTier)
		if got := filepath.Base(probeLaunchExe(inner)); got != rt.Exe() {
			t.Errorf("command: %q renders a line starting %q (base %q); Exe() re-resolves %q", cmd, probeLaunchExe(inner), got, rt.Exe())
		}
	}
}

// The whole path, end to end: the record RuntimeProbe writes names the
// binary the pane resolved and keeps posse's own answer beside it. Driven
// through a herdr that really runs what is typed into its pane, in an
// environment this process does not have — which is the one thing about the
// probe that no pure function can be asked (ranger-base-385x).
func TestRuntimeProbeRecordsTheBinaryTheSessionResolved(t *testing.T) {
	a, rt := probeParityApp(t)
	dir := t.TempDir()
	srv := filepath.Join(dir, "srvbin") // the herdr daemon's PATH
	cli := filepath.Join(dir, "clibin") // posse's own
	srvExe := probeFakeCLI(t, srv, "bob", "bob 2.0-daemon-copy")
	launcherExe := probeFakeCLI(t, cli, "bob", "bob 1.0-launcher-copy")
	t.Setenv("PATH", cli+string(os.PathListSeparator)+os.Getenv("PATH"))

	h := fakeProbeHerdr(t, `case "$1 $2" in
"workspace create") echo '{"id":"f","result":{"workspace":{"workspace_id":"w1"},"root_pane":{"pane_id":"w1:p1"}}}' ;;
"pane run") PATH="`+srv+`:$PATH" /bin/sh -c "$4" >/dev/null 2>&1
	echo '{"id":"f","result":{"type":"pane_run"}}' ;;
"agent list") echo '{"id":"f","result":{"agents":[{"agent":"bob","agent_status":"idle","pane_id":"w1:p1","workspace_id":"w1"}]}}' ;;
"agent wait") echo '{"id":"f","result":{"agent":{"agent_status":"idle"}}}' ;;
"agent explain") echo '{"id":"f","result":{"state":"idle","matched_rule":{"id":"live_prompt_box"},"visible_idle":true}}' ;;
"pane read") echo "fake pane" ;;
*) echo '{"id":"f","result":{}}' ;;
esac`)

	rec, err := a.RuntimeProbe(rt, h, ProbeOpts{Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("the probe could not run: %v", err)
	}
	// It FAILS — nothing here answers the prompt — and that is beside the
	// point: a failed probe writes a record too, and the record's provenance
	// is what this measures.
	if rec.CLIPath != srvExe {
		t.Errorf("cli_path must be the binary the SESSION resolved: got %q, want %q", rec.CLIPath, srvExe)
	}
	if rec.LauncherPath != launcherExe {
		t.Errorf("launcher_cli_path must be posse's own answer: got %q, want %q", rec.LauncherPath, launcherExe)
	}
	// And the version is read off the measured binary, not off the name.
	if rec.Version != "bob 2.0-daemon-copy" {
		t.Errorf("the version must come from the binary the session resolved: %q", rec.Version)
	}
	// Read back from disk, because the record on disk is what every later
	// surface reads — and it is not current, precisely because the two sides
	// disagree here.
	back, err := a.ReadProbeRecord("bob")
	if err != nil || back == nil {
		t.Fatalf("record: %v %v", back, err)
	}
	if back.CLIPath != srvExe || back.LauncherPath != launcherExe {
		t.Errorf("the two paths must survive the write: %+v", back)
	}
}

// posse's own PATH need not have the CLI at all — the pane's is the herdr
// daemon's, and that is the one the session launches from. The record spells
// that absence rather than leaving the field empty, because an empty
// launcher_cli_path is how a record written before ranger-base-385x is told
// apart from one written after it.
func TestProbeLauncherPathSpellsAnAbsenceRatherThanLeavingItEmpty(t *testing.T) {
	t.Parallel()
	absent := &Runtime{Name: "bob", Command: "definitely-not-installed-anywhere-385x --pid {file}"}
	if got := probeLauncherPath(absent); got != ProbeExeUnresolved {
		t.Errorf("an exe posse cannot resolve must be spelled, not blank: %q", got)
	}
	name, path := probeCanary()
	if name == "" {
		t.Skip("no resolvable command on this host to use as the present arm")
	}
	present := &Runtime{Name: "bob", Command: name + " --pid {file}"}
	if got := probeLauncherPath(present); got != path {
		t.Errorf("a resolvable exe is recorded as its path: got %q, want %q", got, path)
	}
}
