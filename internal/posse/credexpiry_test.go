package posse

// Expiry as a first-class answer (ADR 0019 D5/V5/V6, bead ranger-base-k6ha).
//
// The property under test is not "the date is printed". It is that a date
// posse does not know never becomes a warning, that a date it does know
// reaches the two unattended surfaces before the credential bites, and that
// neither surface can act: no park, no clock, no threshold, no gate.
//
// Every fixture is a temp home. Nothing here reads the operator's env sets,
// their keychain or their $HOME.

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

// now is the clock every case in this file is measured against.
var expiryNow = time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)

func day(n int) string { return expiryNow.AddDate(0, 0, n).Format("2006-01-02") }

// stampedSet writes an env set carrying key=value with the stamps a
// `posse refresh` write leaves. expires == "" writes no stamp at all, which
// is the unstamped mint every credential on the box is until an operator
// passes --expires.
func stampedSet(t *testing.T, a *App, set, key, value, expires string) {
	t.Helper()
	body := "# a hand-written header the writer must not eat\n\n"
	if expires != "" {
		body += expiresStamp + expires + "\n"
	}
	body += mintedStamp + day(-30) + "\n" + key + "=" + value + "\n"
	writeEnvFile(t, a, set, body, 0o600)
}

// ─── V5: the stamp round-trips through the seam's ExpiresAt ─────────────────

// ranger-base-h207 left this half open on purpose: it wrote the stamp and
// the seam still answered zero, because a package function has no home to
// find the env sets in. The seam is the one place posse acquires a
// credential, so an ExpiresAt that is structurally always zero for one of
// the two purposes is not a gap in a report — it is the seam lying about
// half of what it serves.
func TestSessionStampRoundTripsThroughTheSeam(t *testing.T) {
	a := refreshApp(t)
	claude, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	key := CageCredential(claude)
	if key == "" {
		t.Fatal("claude has no session credential name")
	}
	const mint = "sk-ant-oat01-THE-MINT-THIS-SESSION-IS-HOLDING"
	stampedSet(t, a, "container", key, mint, day(20))
	namedDefaultEnv(t, a, "container")
	// The retracted arm, present and holding something else: nothing below
	// may come from it (ADR 0039 D3d as amended).
	t.Setenv(key, envArmPoison)

	tok, meta, err := a.ReadCredential(claude, CredSession)
	if err != nil {
		t.Fatal(err)
	}
	if tok != mint {
		t.Errorf("the seam returns the value the set the launch names carries")
	}
	if got := meta.ExpiresAt.Format(stampDate); got != day(20) {
		t.Errorf("stamp did not round-trip through the seam: %q want %q", got, day(20))
	}

	// The stamp belongs to a VALUE, not to a variable name. A second set
	// holding some other mint under the same name is stamped about its own
	// credential, and lending its date to this one would be a wrong date
	// reported confidently — strictly worse than "cannot tell", which is
	// already an honest answer posse is allowed to give. Its date is the
	// SOONER of the two, so a match on the name alone would take it.
	stampedSet(t, a, "other", key, "sk-ant-oat01-SOMEBODY-ELSES-MINT", day(3))
	_, meta, err = a.ReadCredential(claude, CredSession)
	if err != nil {
		t.Fatal(err)
	}
	if got := meta.ExpiresAt.Format(stampDate); got != day(20) {
		t.Errorf("a stamp beside another value became this credential's date: %q", got)
	}

	// And an unstamped set that DOES hold this value is "cannot tell": the
	// zero time, not a guess, not today, not the minted date plus a year —
	// even with the other set's dated mint still sitting beside it.
	stampedSet(t, a, "container", key, mint, "")
	_, meta, err = a.ReadCredential(claude, CredSession)
	if err != nil {
		t.Fatal(err)
	}
	if !meta.ExpiresAt.IsZero() {
		t.Errorf("an unstamped mint got a date from somewhere: %v", meta.ExpiresAt)
	}
}

// A nil *App is the seam's degenerate caller (a reader constructed without a
// home). It must ANSWER, not panic — acquiring a new way to crash inside the
// credential seam is not a trade this design makes.
//
// What it answers changed when the environment arm was retracted (ADR 0039
// D3d as amended): the session store IS the files under the home, so a
// caller with no home has no store to open and the honest answer is the
// refusal, not a value. It says so in the sentence that names the sets it
// opened — none — and it invents no date. The environment holding the name
// does not make it an answer; that is the whole retraction.
func TestSeamWithNoHomeRefusesRatherThanPanicking(t *testing.T) {
	a := refreshApp(t)
	claude, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(CageCredential(claude), envArmPoison)
	var none *App
	tok, meta, err := none.ReadCredential(claude, CredSession)
	if err == nil {
		t.Fatalf("a seam with no home has no session store to read")
	}
	if tok != "" {
		t.Errorf("a seam with no home returned a value")
	}
	if !meta.ExpiresAt.IsZero() {
		t.Errorf("a seam with no home invented a date: %v", meta.ExpiresAt)
	}
	if !strings.Contains(err.Error(), "claude setup-token") {
		t.Errorf("the refusal still says how the operator mints one: %v", err)
	}
	// The nil receiver reaches the set list too, which is the other half of
	// the same panic question.
	if got := none.LaunchEnvSets(nil, nil); len(got) != 0 {
		t.Errorf("a nil App has no home to find a default_env under: %v", got)
	}
}

// ─── V6: what warns, what does not, and what a zero renders as ─────────────

func TestOnlyADatedCredentialInsideTheWindowWarns(t *testing.T) {
	a := refreshApp(t)
	key := CageCredential(mustRuntime(t, a, "claude"))

	cases := []struct {
		name    string
		expires string
		warns   bool
	}{
		// The whole "cannot tell" rule in one row: no stamp, no warning,
		// ever. Every credential on the box is this until an operator
		// passes --expires, so a design that warned about unknowns would
		// warn about all of them on day one.
		{"unstamped", "", false},
		{"far outside the window", day(90), false},
		// One day outside and one day inside: the boundary is a real edge
		// and a fortnight is the number ADR 0019 D5 names.
		{"just outside the window", day(15), false},
		{"just inside the window", day(13), true},
		{"expired", day(-2), true},
		// A stamp posse cannot parse is not a date. Same answer as no
		// stamp: silence, never a warning about a string.
		{"unparseable stamp", "next tuesday", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stampedSet(t, a, "container", key, "sk-ant-oat01-MINT", c.expires)
			ex := a.ExpiringCredentials(expiryNow)
			if got := len(ex) > 0; got != c.warns {
				t.Fatalf("warns=%v want %v (%+v)", got, c.warns, ex)
			}
			if !c.warns {
				return
			}
			if ex[0].Runtime != "claude" || ex[0].Purpose != CredSession ||
				ex[0].Set != "container" || ex[0].Key != key {
				t.Errorf("the warning must name what to go fix: %+v", ex[0])
			}
			if want := c.expires == day(-2); ex[0].Expired(expiryNow) != want {
				t.Errorf("Expired=%v want %v", ex[0].Expired(expiryNow), want)
			}
		})
	}
}

// Expired renders DISTINCTLY from approaching (ADR 0019 V6). "in -3d" is
// the failure this pins: an operator reading a negative countdown as a
// countdown is an operator who thinks they have time.
func TestExpiredRendersDistinctly(t *testing.T) {
	soon := CredExpiry{Runtime: "claude", Purpose: CredSession, Set: "container",
		Key: "CLAUDE_CODE_OAUTH_TOKEN", At: expiryNow.Add(72 * time.Hour)}
	dead := soon
	dead.At = expiryNow.Add(-72 * time.Hour)

	if b := soon.Brief(expiryNow); b != "in 3d" {
		t.Errorf("approaching brief: %q", b)
	}
	if b := dead.Brief(expiryNow); b != "EXPIRED" {
		t.Errorf("expired brief: %q", b)
	}
	if w := dead.Warning(expiryNow); !strings.Contains(w, "EXPIRED") || strings.Contains(w, "in -") {
		t.Errorf("expired warning reads as a countdown: %q", w)
	}
	// Hours below two days, days above: a mint with 40 hours left is a
	// today problem, and "in 1d" reads like a tomorrow one.
	near := soon
	near.At = expiryNow.Add(40 * time.Hour)
	if b := near.Brief(expiryNow); b != "in 40h" {
		t.Errorf("under two days is told in hours: %q", b)
	}
}

// The warning and the report an operator checks it against must print one
// date for one stamp. They share renderExpiry for exactly this reason, and
// this is the pin that keeps a second rounding from being introduced into
// either.
func TestTheWarningAndTheReportAgreeOnTheDate(t *testing.T) {
	a := refreshApp(t)
	key := CageCredential(mustRuntime(t, a, "claude"))
	stampedSet(t, a, "container", key, "sk-ant-oat01-MINT", day(9))

	ex := a.ExpiringCredentials(expiryNow)
	if len(ex) != 1 {
		t.Fatalf("want one warning, got %+v", ex)
	}
	o := opts(RefreshOpts{}, "", nil)
	o.clock = func() time.Time { return expiryNow }
	var reported string
	for _, r := range a.CredReport(o) {
		if r.Runtime == "claude" && r.Purpose == CredSession {
			reported = r.Expiry
		}
	}
	if reported == "" {
		t.Fatal("the report has no session row for claude")
	}
	if !strings.Contains(ex[0].Warning(expiryNow), reported) {
		t.Errorf("warning %q does not carry the report's own date %q", ex[0].Warning(expiryNow), reported)
	}
}

// Two credentials, one line. The surfaces ADR 0019 D5 asks for are "one
// stderr line per dispatch pass", and the soonest is the one that gets the
// verb — sorted, not whichever set the directory listing happened to yield.
func TestTheSoonestExpiryIsTheOneWithTheVerb(t *testing.T) {
	a := refreshApp(t)
	key := CageCredential(mustRuntime(t, a, "claude"))
	// The soonest is the alphabetically LAST set on purpose: a directory
	// listing is already sorted by name, so a fixture where the two agree
	// is green with the sort deleted.
	stampedSet(t, a, "alpha", key, "sk-ant-oat01-A", day(10))
	stampedSet(t, a, "zulu", key, "sk-ant-oat01-B", day(2))

	ex := a.ExpiringCredentials(expiryNow)
	if len(ex) != 2 {
		t.Fatalf("both sets hold a dated mint: %+v", ex)
	}
	if ex[0].Set != "zulu" {
		t.Errorf("soonest first: %+v", ex)
	}
	if fix := ex[0].Fix(); fix != "posse refresh claude --env-set zulu" {
		t.Errorf("the verb must name the set it is about: %q", fix)
	}
}

// ─── V6-as-amended (ranger-base-swqk): the meter is report-only ────────────

// ExpiringCredentials is the one reader both timer surfaces (the cockpit
// header, credentialExpiry's stderr line) use — credexpiry.go's own doc
// comment says so, and dispatch.go and cmd/posse/cockpit.go both call
// nothing else to build theirs. So pinning what THIS returns pins both
// surfaces at once; there is no third path either one could take instead.
//
// The session mint dated inside the same window is the positive witness
// (probe-needs-a-failing-wrong-arm): without it, "no meter in the list"
// cannot be told apart from "the reader never ran" or "nothing on this box
// is expiring at all". Mutation-checked by temporarily appending a
// CredMeter CredExpiry built from meterRow's own reading into
// ExpiringCredentials' loop — that turns this pin red.
func TestMeterEnvelopeReachesReportButNeitherTimerSurface(t *testing.T) {
	a := refreshApp(t)
	key := CageCredential(mustRuntime(t, a, "claude"))

	// The positive witness: a session mint inside the window.
	stampedSet(t, a, "container", key, "sk-ant-oat01-SESSION-MINT", day(5))

	// The meter envelope, also inside the window — read through the file
	// adapter (goos="linux" below), never a live keychain (ranger-base-ouf9:
	// a test must not read the box). credentialsHome and envelope are
	// credseam_test.go's fixtures for exactly this seam.
	meterExpires := expiryNow.AddDate(0, 0, 5)
	credentialsHome(t, envelope("sk-ant-oat01-METER-TOKEN", meterExpires.UnixMilli()))

	// The seam both timer surfaces read: the meter must not be in it, and
	// the session mint — the positive witness — must be.
	ex := a.ExpiringCredentials(expiryNow)
	if len(ex) != 1 || ex[0].Purpose != CredSession || ex[0].Set != "container" {
		t.Fatalf("want exactly the session mint (the positive witness), got %+v", ex)
	}
	for _, e := range ex {
		if e.Purpose == CredMeter {
			t.Fatalf("a meter envelope reached the timer surfaces' own reader: %+v", ex)
		}
	}

	// The report carries both purposes: the meter row is there and dated.
	o := opts(RefreshOpts{}, "", nil)
	o.clock = func() time.Time { return expiryNow }
	var sawMeterRow bool
	for _, r := range a.CredReport(o) {
		if r.Runtime != "claude" || r.Purpose != CredMeter {
			continue
		}
		sawMeterRow = true
		if !strings.Contains(r.Expiry, "in 5d") {
			t.Errorf("the meter's near-expiry date did not reach the report: %+v", r)
		}
	}
	if !sawMeterRow {
		t.Fatal("no meter row for claude in the report — the report is supposed to carry both purposes")
	}

	// The dispatch pass's stderr line — one of the two timer surfaces —
	// names the session mint and never the meter.
	d, _, errb := expiryDispatcher(t, a)
	d.credentialExpiry()
	line := errb.String()
	if !strings.Contains(line, "claude session") {
		t.Errorf("the pass line did not name the session mint: %q", line)
	}
	if strings.Contains(line, "meter") {
		t.Errorf("the pass line named the meter, which V6-as-amended gives no timer surface: %q", line)
	}
}

// ─── the dispatch pass: one line, and nothing else ─────────────────────────

func expiryDispatcher(t *testing.T, a *App) (*Dispatcher, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errb bytes.Buffer
	d := NewDispatcher(a, nil, nil)
	d.Out, d.Err = &out, &errb
	d.Now = func() time.Time { return expiryNow }
	return d, &out, &errb
}

func TestDispatchPassWarnsOncePerPassAndParksNothing(t *testing.T) {
	a := refreshApp(t)
	key := CageCredential(mustRuntime(t, a, "claude"))
	stampedSet(t, a, "container", key, "sk-ant-oat01-MINT", day(6))

	d, out, errb := expiryDispatcher(t, a)
	d.credentialExpiry()
	line := errb.String()
	if n := strings.Count(strings.TrimRight(line, "\n"), "\n"); n != 0 {
		t.Fatalf("one line per pass, got %d extra:\n%s", n, line)
	}
	// day(6) is a DATE — midnight — and the clock is noon, so 5d12h left
	// renders "in 5d". Truncating is the direction a warning may err in,
	// and it is the report's own arithmetic (expiryIn).
	for _, want := range []string{"credential expiry", "claude", "container", key, "in 5d",
		"posse refresh claude --env-set container", "only actuator"} {
		if !strings.Contains(line, want) {
			t.Errorf("the pass line does not carry %q:\n%s", want, line)
		}
	}
	// The value is never in it. This file's fixtures use an obviously fake
	// mint; the rule it stands for is the one the whole seam is under.
	if strings.Contains(line, "sk-ant-oat01-MINT") {
		t.Errorf("the warning quoted the credential:\n%s", line)
	}
	if out.Len() != 0 {
		t.Errorf("a warning is not a pass event: %q", out.String())
	}
	// It warns. It does not act: no trip, no blind clock, no park.
	if d.planTrip != "" || d.planBlind != "" || !d.blindSince.IsZero() || d.blindFailed {
		t.Errorf("expiry moved the guard: trip=%q blind=%q since=%v failed=%v",
			d.planTrip, d.planBlind, d.blindSince, d.blindFailed)
	}
}

// Two expiring credentials: still one line, and the ones it cannot name are
// counted rather than dropped — a count is what sends an operator to the
// report, and a silent drop is what lets the second one surprise them.
func TestASecondExpiryIsCountedNotPrinted(t *testing.T) {
	a := refreshApp(t)
	key := CageCredential(mustRuntime(t, a, "claude"))
	stampedSet(t, a, "alpha", key, "sk-ant-oat01-A", day(9))
	stampedSet(t, a, "zulu", key, "sk-ant-oat01-B", day(1))

	d, _, errb := expiryDispatcher(t, a)
	d.credentialExpiry()
	line := errb.String()
	if !strings.Contains(line, "+1 more") || !strings.Contains(line, "zulu") {
		t.Errorf("want the soonest named and the rest counted:\n%s", line)
	}
	if strings.Contains(line, "alpha") {
		t.Errorf("the second one was printed, not counted:\n%s", line)
	}
}

// Nothing to say is SILENCE, not a reassurance. A pass that prints
// "credentials fine" every minute is a log nobody reads, and the state it
// would be reporting is ambiguous anyway — nothing expiring, or nothing
// dated. `posse refresh` is where that ambiguity is resolved, per credential.
func TestAnUndatedOrHealthyBoxSaysNothingOnThePass(t *testing.T) {
	a := refreshApp(t)
	key := CageCredential(mustRuntime(t, a, "claude"))
	for _, expires := range []string{"", day(90)} {
		stampedSet(t, a, "container", key, "sk-ant-oat01-MINT", expires)
		d, out, errb := expiryDispatcher(t, a)
		d.credentialExpiry()
		if errb.Len() != 0 || out.Len() != 0 {
			t.Errorf("expires=%q warned: %q / %q", expires, errb.String(), out.String())
		}
	}
}

// ─── the wiring, not the function ──────────────────────────────────────────

// A pass that never calls the warning is a warning that does not exist, and
// every test above it would still be green: they call credentialExpiry
// directly. This one runs a real Run() and reads its stderr.
//
// It also pins WHERE in the pass: the plan guard is unarmed here (no
// `plan_guard_<window>:` in the config at all), because a credential expires
// on a box whose operator never armed a meter guard, and a warning nested
// inside that guard would be silent on exactly that box.
func TestARealPassPrintsTheCredentialWarning(t *testing.T) {
	b, _ := newTestBackend(t) // its own temp $HOME
	a := b.App
	if err := os.MkdirAll(a.EnvsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	key := CageCredential(mustRuntime(t, a, "claude"))
	stampedSet(t, a, "container", key, "sk-ant-oat01-MINT", day(4))

	d := newTestDispatcher(t, b)
	d.DryRun = true
	var errb bytes.Buffer
	d.Err = &errb
	d.Now = func() time.Time { return expiryNow }
	writePersona(t, a, "ranger", "[go]")
	repo := qaRepo(t, a, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
	budgetConfig(t, a, repo, "")

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errb.String(), "credential expiry: claude session in env set container") {
		t.Errorf("a real pass printed no credential warning:\n%s", errb.String())
	}
	// And it warned without stopping anything: the pass still routed.
	if out := dispatcherOut(d); !strings.Contains(out, "a-1") {
		t.Errorf("the warning cost the pass its routing:\n%s", out)
	}
}

func mustRuntime(t *testing.T, a *App, name string) *Runtime {
	t.Helper()
	rt, err := a.LoadRuntime(name)
	if err != nil {
		t.Fatal(err)
	}
	return rt
}
