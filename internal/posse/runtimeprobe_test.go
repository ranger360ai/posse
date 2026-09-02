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
		Runtime: "bob", CLIPath: "/usr/local/bin/bob", Version: "bob 1.2.3",
		Date: time.Now().UTC().Truncate(time.Second), PosseVersion: Version, Canary: "uname",
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
	rec := &ProbeRecord{Runtime: "bob", CLIPath: "/usr/local/bin/bob", Version: "bob 1.2.3", Date: time.Now().UTC(), Observables: failed}
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
