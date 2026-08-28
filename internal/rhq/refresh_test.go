package rhq

// `posse refresh` (ADR 0019 D4, ranger-base-h207) — the one credential write
// in posse, and the tests are mostly about what it REFUSES.
//
// The property under test is not "the write works". It is that the exception
// to "posse never writes a credential" cannot widen: the gate is on the verb
// and not on a branch of it, the rotating token is never written by any
// path, a metered key is refused by name and by shape, and a date posse does
// not know is reported as unknown rather than as freshness.
//
// Every test here runs on any box: the report takes its GOOS as a parameter
// and its stores from a temp $HOME, so nothing reaches the operator's own
// keychain or their live env sets from a suite run.

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// refreshApp is a posse home with an envs/ dir and nothing else live: $HOME
// is redirected too, so the meter adapter's credentials-file lookup lands in
// the fixture rather than in whoever is running the suite.
func refreshApp(t *testing.T) *App {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(EnvPersona, "") // the suite may itself be running inside a persona session
	cfg := filepath.Join(home, "config")
	if err := os.MkdirAll(filepath.Join(cfg, "envs"), 0o700); err != nil {
		t.Fatal(err)
	}
	return &App{Home: cfg, EnvsDir: filepath.Join(cfg, "envs"), StateDir: filepath.Join(cfg, "state"),
		AgentsDir: filepath.Join(cfg, "agents"), ConfigPath: filepath.Join(cfg, "config.yaml")}
}

// opts is a refresh whose four seams are wired to the test: a terminal that
// is there, a mint that records instead of execing, a paste that answers,
// and a clock that does not move.
func opts(o RefreshOpts, tok string, minted *int) RefreshOpts {
	o.goos = "linux"
	o.tty = func() bool { return true }
	o.clock = func() time.Time { return time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC) }
	o.mint = func(io.Writer, *Runtime) error {
		if minted != nil {
			*minted++
		}
		return nil
	}
	o.ask = func(io.Writer, string) (string, error) { return tok, nil }
	return o
}

func writeEnvFile(t *testing.T, a *App, set, body string, mode os.FileMode) string {
	t.Helper()
	p := filepath.Join(a.EnvsDir, set+".env")
	if err := os.WriteFile(p, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, mode); err != nil {
		t.Fatal(err)
	}
	return p
}

// ─── V4: the gate is on the verb, not on a branch of it ─────────────────────

// A persona session is refused, and it is refused for the REPORT too — the
// no-argument form writes nothing, but the deny line an operator can express
// is `Bash(posse refresh:*)`, which does not know about arguments. A gate the
// binary applies more narrowly than the rule that spells it is a gate with a
// hole in it.
func TestRefreshRefusesUnderThePersonaMarkerIncludingTheReport(t *testing.T) {
	a := refreshApp(t)
	t.Setenv(EnvPersona, "developer-3")
	for _, o := range []RefreshOpts{{}, {Runtime: "claude"}} {
		var w bytes.Buffer
		err := a.CmdRefresh(&w, opts(o, "tok", nil))
		if err == nil {
			t.Fatalf("refresh ran under %s (runtime=%q)", EnvPersona, o.Runtime)
		}
		if !strings.Contains(err.Error(), EnvPersona) || !strings.Contains(err.Error(), "developer-3") {
			t.Errorf("refusal does not name the marker that caused it: %v", err)
		}
		if w.Len() > 0 {
			t.Errorf("a refused refresh still reported: %q", w.String())
		}
	}
}

// Without a terminal there is no human to be the gate, and the browser flow
// is the whole reason this command may write at all.
func TestRefreshRefusesWithoutATTYIncludingTheReport(t *testing.T) {
	a := refreshApp(t)
	for _, o := range []RefreshOpts{{}, {Runtime: "claude", EnvSet: "container"}} {
		ro := opts(o, "tok", nil)
		ro.tty = func() bool { return false }
		var w bytes.Buffer
		err := a.CmdRefresh(&w, ro)
		if err == nil {
			t.Fatalf("refresh ran with no terminal (runtime=%q)", o.Runtime)
		}
		if !strings.Contains(err.Error(), "terminal") {
			t.Errorf("refusal does not say what was missing: %v", err)
		}
	}
	if _, err := os.Stat(filepath.Join(a.EnvsDir, "container.env")); err == nil {
		t.Error("a refused refresh created the env set anyway")
	}
}

// The control: with both gates open the same call goes through. Without this
// the two tests above pass on any refusal at all, including one from a
// fixture that never worked.
func TestRefreshRunsWhenTheGatesAreOpen(t *testing.T) {
	a := refreshApp(t)
	var w bytes.Buffer
	if err := a.CmdRefresh(&w, opts(RefreshOpts{}, "", nil)); err != nil {
		t.Fatalf("the report refused with both gates open: %v", err)
	}
	if !strings.Contains(w.String(), "posse refresh") {
		t.Errorf("no report was printed: %q", w.String())
	}
}

// ─── V5: what the write does to the file, and to its mode ───────────────────

func TestRefreshWritesTheMintIntoTheEnvSetStampedAt600Under700(t *testing.T) {
	a := refreshApp(t)
	if err := os.Chmod(a.EnvsDir, 0o755); err != nil { // drifted, as a hand-copied dir does
		t.Fatal(err)
	}
	writeEnvFile(t, a, "container", "# hand-written\nOTHER=keep-me\n", 0o644)

	mints := 0
	var w bytes.Buffer
	o := opts(RefreshOpts{Runtime: "claude", EnvSet: "container", Expires: "2026-12-01"}, "sk-ant-oat01-fixture", &mints)
	if err := a.CmdRefresh(&w, o); err != nil {
		t.Fatal(err)
	}
	if mints != 1 {
		t.Errorf("the runtime's own mint ran %d times, want 1 — the browser flow IS the human gate", mints)
	}
	p := filepath.Join(a.EnvsDir, "container.env")
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Errorf("env set is %04o, want 0600 — it holds a plaintext secret", st.Mode().Perm())
	}
	dst, err := os.Stat(a.EnvsDir)
	if err != nil {
		t.Fatal(err)
	}
	if dst.Mode().Perm() != 0o700 {
		t.Errorf("envs/ is %04o, want 0700", dst.Mode().Perm())
	}
	body, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# hand-written", "OTHER=keep-me",
		"# minted=2026-08-28", "# expires=2026-12-01", "CLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-fixture"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("env set is missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(w.String(), "sk-ant-oat01-fixture") {
		t.Errorf("the command printed the credential it just wrote:\n%s", w.String())
	}
}

// The mode above is asserted after TightenEnvPerms has also run, and that
// pass would tighten a 0644 write back to 0600 — so the WRITER's own mode is
// pinned here, where nothing else can supply it. The window a wide write
// opens is real even when a later pass closes it: the temp file holds the
// credential for as long as it takes to rename.
func TestTheEnvSetWriterItselfNeverWidensTheFile(t *testing.T) {
	a := refreshApp(t)
	writeEnvFile(t, a, "container", "OTHER=x\n", 0o644)
	p, err := a.setEnvVarStamped("container", "CLAUDE_CODE_OAUTH_TOKEN", "tok",
		time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Errorf("the writer left %04o, want 0600 — TightenEnvPerms is a second belt, not the first", st.Mode().Perm())
	}
}

// A destination that is not a regular file is refused by name and nothing is
// created — a symlink there would aim a credential write at a path posse did
// not choose, and a directory there is a mistake worth a sentence rather than
// an errno.
func TestTheWriterRefusesADestinationThatIsNotARegularFile(t *testing.T) {
	a := refreshApp(t)
	if err := os.MkdirAll(filepath.Join(a.EnvsDir, "blocked.env"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "elsewhere"), filepath.Join(a.EnvsDir, "aimed.env")); err != nil {
		t.Fatal(err)
	}
	for _, set := range []string{"blocked", "aimed"} {
		_, err := a.setEnvVarStamped(set, "CLAUDE_CODE_OAUTH_TOKEN", "sk-ant-oat01-leftover",
			time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC), time.Time{})
		if err == nil {
			t.Fatalf("%s.env: a credential was written to a path that is not a regular file", set)
		}
		if !strings.Contains(err.Error(), "regular file") {
			t.Errorf("%s.env: refusal does not say why: %v", set, err)
		}
		if left, _ := filepath.Glob(filepath.Join(a.EnvsDir, "."+set+".env.*")); len(left) > 0 {
			t.Errorf("%s.env: a temp file was created before the refusal: %v", set, left)
		}
	}
	if b, err := os.ReadFile(filepath.Join(t.TempDir(), "elsewhere")); err == nil {
		t.Errorf("the symlink was followed and the credential landed off the envs dir: %d bytes", len(b))
	}
}

// The stamps are COMMENTS, which is why nothing else in posse had to learn
// about them. If one were ever written as a variable, `posse envs` — whose
// standing rule is key names only — would print "# minted" as a key name and
// the launch would inject it.
func TestStampsAreCommentsAndNeverBecomeKeys(t *testing.T) {
	a := refreshApp(t)
	var w bytes.Buffer
	o := opts(RefreshOpts{Runtime: "claude", EnvSet: "container", Expires: "2026-12-01"}, "tok", nil)
	if err := a.CmdRefresh(&w, o); err != nil {
		t.Fatal(err)
	}
	vars, err := a.EnvSetVars("container")
	if err != nil {
		t.Fatal(err)
	}
	if len(vars) != 1 || vars[0].Key != "CLAUDE_CODE_OAUTH_TOKEN" {
		t.Fatalf("env set parses to %+v, want exactly the one variable", envKeyNames(vars))
	}
}

func envKeyNames(vars []EnvVar) []string {
	var out []string
	for _, v := range vars {
		out = append(out, v.Key)
	}
	return out
}

// A re-mint replaces the stamps it wrote last time rather than stacking a new
// pair above the old — otherwise the file grows a history of dates and the
// report reads whichever one happens to be nearest.
func TestARemintReplacesItsOwnStampsAndKeepsTheOperatorsComments(t *testing.T) {
	a := refreshApp(t)
	writeEnvFile(t, a, "container", "# the operator's own note\n# minted=2020-01-01\n# expires=2020-02-01\nCLAUDE_CODE_OAUTH_TOKEN=old\nTAIL=still-here\n", 0o600)
	var w bytes.Buffer
	o := opts(RefreshOpts{Runtime: "claude", EnvSet: "container", Expires: "2026-12-01"}, "new", nil)
	if err := a.CmdRefresh(&w, o); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(a.EnvsDir, "container.env"))
	s := string(body)
	if strings.Count(s, "# minted=") != 1 || strings.Count(s, "# expires=") != 1 {
		t.Errorf("stamps stacked instead of being replaced:\n%s", s)
	}
	for _, want := range []string{"# the operator's own note", "TAIL=still-here", "# minted=2026-08-28", "CLAUDE_CODE_OAUTH_TOKEN=new"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "=old") {
		t.Errorf("the previous value survived:\n%s", s)
	}
}

// The stamp written by the write is the date read back by the report. Both
// halves in one test, because a stamp nothing reads is decoration.
func TestTheStampsRoundTripIntoTheReport(t *testing.T) {
	a := refreshApp(t)
	var w bytes.Buffer
	o := opts(RefreshOpts{Runtime: "claude", EnvSet: "container", Expires: "2026-12-01"}, "tok", nil)
	if err := a.CmdRefresh(&w, o); err != nil {
		t.Fatal(err)
	}
	row := sessionRowFor(t, a, opts(RefreshOpts{}, "", nil), "claude")
	if !strings.Contains(row.Source, "env set container (CLAUDE_CODE_OAUTH_TOKEN)") {
		t.Errorf("report does not name where it lives: %q", row.Source)
	}
	if !strings.Contains(row.Source, "minted 2026-08-28") {
		t.Errorf("report does not carry the minted stamp: %q", row.Source)
	}
	if !strings.HasPrefix(row.Expiry, "2026-12-01") {
		t.Errorf("expiry does not round-trip: %q", row.Expiry)
	}
	if row.Action != "nothing to do" {
		t.Errorf("a credential three months from expiry has an action: %q", row.Action)
	}
}

// An expiry posse was not told is reported as unknown — never as fresh, and
// it warns nothing (ADR 0019 D5). This is the arm that makes the test above
// discriminating: without it, "cannot tell" and a real date are both green.
func TestAnUnstampedExpiryIsCannotTellAndNotFresh(t *testing.T) {
	a := refreshApp(t)
	var w bytes.Buffer
	if err := a.CmdRefresh(&w, opts(RefreshOpts{Runtime: "claude", EnvSet: "container"}, "tok", nil)); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(w.String(), "# expires=") {
		t.Errorf("an expires stamp was invented with no date given:\n%s", w.String())
	}
	body, _ := os.ReadFile(filepath.Join(a.EnvsDir, "container.env"))
	if strings.Contains(string(body), "expires=") {
		t.Errorf("an expires stamp reached the file:\n%s", body)
	}
	row := sessionRowFor(t, a, opts(RefreshOpts{}, "", nil), "claude")
	if row.Expiry != "cannot tell" {
		t.Errorf("unstamped expiry rendered as %q", row.Expiry)
	}
	if strings.Contains(strings.ToLower(row.Action), "expired") || strings.Contains(row.Action, "re-mint within") {
		t.Errorf("an unknown expiry warned: %q", row.Action)
	}
}

// The window is a window: inside it the report says re-mint, past it the
// report says expired, and outside it says nothing. All three from the same
// fixture so the boundary is the only thing that moves.
func TestTheExpiryWindowHasThreeAnswers(t *testing.T) {
	a := refreshApp(t)
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct{ expires, want string }{
		{"2026-08-01", "expired"},
		{"2026-09-05", "re-mint within the window"},
		{"2026-12-01", "nothing to do"},
	} {
		var w bytes.Buffer
		if err := a.CmdRefresh(&w, opts(RefreshOpts{Runtime: "claude", EnvSet: "container", Expires: tc.expires}, "tok", nil)); err != nil {
			t.Fatal(err)
		}
		row := sessionRowFor(t, a, opts(RefreshOpts{}, "", nil), "claude")
		if !strings.Contains(row.Action, tc.want) {
			t.Errorf("expires=%s at %s → action %q, want it to say %q", tc.expires, now.Format(stampDate), row.Action, tc.want)
		}
	}
}

func sessionRowFor(t *testing.T, a *App, o RefreshOpts, rt string) CredRow {
	t.Helper()
	for _, r := range a.CredReport(o) {
		if r.Runtime == rt && r.Purpose == CredSession {
			return r
		}
	}
	t.Fatalf("no session row for %s in the report", rt)
	return CredRow{}
}

// ─── the money line, twice ──────────────────────────────────────────────────

// By NAME: a runtime that names ANTHROPIC_API_KEY as its session credential
// is refused rather than served. A persona is never the one who decides to
// spend, and neither is a command writing a file at 2am.
func TestRefreshWillNotWriteAMeteredCredentialByName(t *testing.T) {
	a := refreshApp(t)
	if err := os.MkdirAll(a.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.RuntimesDir(), "metered.yaml"),
		[]byte("command: metered {file}\ncage_cred: ANTHROPIC_API_KEY\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var w bytes.Buffer
	err := a.CmdRefresh(&w, opts(RefreshOpts{Runtime: "metered", EnvSet: "container"}, "tok", nil))
	if err == nil {
		t.Fatal("refresh wrote a metered credential")
	}
	if !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
		t.Errorf("refusal does not name the credential: %v", err)
	}
	if _, err := os.Stat(filepath.Join(a.EnvsDir, "container.env")); err == nil {
		t.Error("the env set was written anyway")
	}
}

// By SHAPE: the same key pasted into the session variable spends the same
// money under a name that looks right. The refusal names the prefix it saw
// and never the value.
func TestRefreshWillNotWriteAMeteredCredentialByShape(t *testing.T) {
	a := refreshApp(t)
	var w bytes.Buffer
	err := a.CmdRefresh(&w, opts(RefreshOpts{Runtime: "claude", EnvSet: "container"}, "sk-ant-api03-secretsecret", nil))
	if err == nil {
		t.Fatal("refresh wrote a metered API key into the session variable")
	}
	if !strings.Contains(err.Error(), meteredKeyPrefix) {
		t.Errorf("refusal does not name the shape it refused: %v", err)
	}
	if strings.Contains(err.Error(), "secretsecret") {
		t.Errorf("the refusal quoted the credential: %v", err)
	}
	if _, err := os.Stat(filepath.Join(a.EnvsDir, "container.env")); err == nil {
		t.Error("the env set was written anyway")
	}
}

// A value that cannot BE an env set line is refused before it corrupts one:
// a newline writes a second line the parser reads as another variable.
func TestATokenWithALineBreakIsRefused(t *testing.T) {
	a := refreshApp(t)
	var w bytes.Buffer
	err := a.CmdRefresh(&w, opts(RefreshOpts{Runtime: "claude", EnvSet: "container"}, "tok\nEXTRA=1", nil))
	if err == nil || !strings.Contains(err.Error(), "line break") {
		t.Fatalf("a multi-line token was accepted: %v", err)
	}
}

// ─── the meter half: nothing is written, ever ───────────────────────────────

func TestRefreshMeterWritesNothingAndNamesTheStoreOfRecord(t *testing.T) {
	a := refreshApp(t)
	writeEnvFile(t, a, "container", "CLAUDE_CODE_OAUTH_TOKEN=untouched\n", 0o600)
	before, _ := os.ReadFile(filepath.Join(a.EnvsDir, "container.env"))
	tree := treeOf(t, os.Getenv("HOME"))
	mints := 0
	var w bytes.Buffer
	o := opts(RefreshOpts{Runtime: "claude", Purpose: CredMeter, EnvSet: "container"}, "tok", &mints)
	if err := a.CmdRefresh(&w, o); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(a.EnvsDir, "container.env"))
	if string(before) != string(after) {
		t.Errorf("the meter branch wrote to an env set:\n%s", after)
	}
	if now := treeOf(t, os.Getenv("HOME")); now != tree {
		t.Errorf("the meter branch wrote somewhere under the home:\nbefore %s\nafter  %s", tree, now)
	}
	if mints != 0 {
		t.Errorf("the meter branch ran a mint %d times", mints)
	}
	out := w.String()
	if !strings.Contains(out, ".credentials.json") {
		t.Errorf("the store of record is not named: %q", out)
	}
	if !strings.Contains(out, "writes nothing") {
		t.Errorf("the instruction does not say posse writes nothing: %q", out)
	}
	if strings.Contains(out, "keychain") {
		t.Errorf("a non-darwin answer said keychain (ADR 0019 V2): %q", out)
	}
}

// ─── the report over structural absence ─────────────────────────────────────

// A box with no login has no meter credential and that is a STRUCTURE, not
// an outage: the report says what would arm it, and never in the words of a
// platform it is not on.
func TestTheReportRendersStructuralAbsenceAsSomethingToDo(t *testing.T) {
	a := refreshApp(t)
	rows := a.CredReport(opts(RefreshOpts{}, "", nil))
	var meter CredRow
	for _, r := range rows {
		if r.Runtime == "claude" && r.Purpose == CredMeter {
			meter = r
		}
	}
	if meter.Source == "" {
		t.Fatal("no claude meter row in the report")
	}
	if !strings.Contains(meter.Source, ".credentials.json") {
		t.Errorf("source is not the store this platform would use: %q", meter.Source)
	}
	if strings.Contains(meter.Source+meter.Action, "keychain") {
		t.Errorf("a linux row said keychain (ADR 0019 V2): %+v", meter)
	}
	if !strings.Contains(meter.Action, "`claude`") {
		t.Errorf("the arm does not say what would arm it: %q", meter.Action)
	}
	if meter.Expiry != "—" && meter.Expiry != "cannot tell" {
		t.Errorf("an absent store reported an expiry: %q", meter.Expiry)
	}
}

// Every runtime posse can launch appears with both purposes, because the
// report's job is to be complete rather than to be short: a credential
// nobody listed is one nobody renews.
func TestTheReportCoversEveryRuntimeAndBothPurposes(t *testing.T) {
	a := refreshApp(t)
	seen := map[string]bool{}
	for _, r := range a.CredReport(opts(RefreshOpts{}, "", nil)) {
		seen[r.Runtime+"/"+string(r.Purpose)] = true
	}
	for _, rt := range a.ListRuntimes() {
		for _, p := range []CredPurpose{CredSession, CredMeter} {
			if !seen[rt+"/"+string(p)] {
				t.Errorf("the report has no %s row for %s", p, rt)
			}
		}
	}
	if len(seen) < 6 {
		t.Errorf("only %d (runtime, purpose) pairs reported: %v", len(seen), seen)
	}
}

// A runtime whose session credential nobody has decided says exactly that,
// and offers the decision rather than a mint it cannot perform.
func TestAnUndecidedSessionCredentialSaysSoRatherThanNothing(t *testing.T) {
	a := refreshApp(t)
	row := sessionRowFor(t, a, opts(RefreshOpts{}, "", nil), "grok")
	if !strings.Contains(row.Source, "undecided") {
		t.Errorf("grok's session row: %q", row.Source)
	}
	var w bytes.Buffer
	err := a.CmdRefresh(&w, opts(RefreshOpts{Runtime: "grok", EnvSet: "container"}, "tok", nil))
	if err == nil || !strings.Contains(err.Error(), "cage_cred") {
		t.Fatalf("refreshing an undecided credential: %v", err)
	}
}

// ─── which env set gets written ─────────────────────────────────────────────

// With no --env-set, the set that already holds the variable is the answer
// when there is exactly one, and ambiguity is refused BY NAME: a token
// written into the set no launch reads is a failure that looks like success.
func TestTheEnvSetIsResolvedOrRefusedByName(t *testing.T) {
	a := refreshApp(t)
	if _, err := a.resolveRefreshSet("", "CLAUDE_CODE_OAUTH_TOKEN"); err == nil {
		t.Error("an empty envs/ resolved to a set")
	}
	writeEnvFile(t, a, "container", "CLAUDE_CODE_OAUTH_TOKEN=a\n", 0o600)
	got, err := a.resolveRefreshSet("", "CLAUDE_CODE_OAUTH_TOKEN")
	if err != nil || got != "container" {
		t.Fatalf("one holder resolved to (%q, %v)", got, err)
	}
	writeEnvFile(t, a, "spare", "CLAUDE_CODE_OAUTH_TOKEN=b\n", 0o600)
	_, err = a.resolveRefreshSet("", "CLAUDE_CODE_OAUTH_TOKEN")
	if err == nil {
		t.Fatal("two holders resolved silently to one of them")
	}
	for _, want := range []string{"container", "spare"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %s: %v", want, err)
		}
	}
}

// A stamp belongs to the variable it sits directly above. One read off the
// wrong variable is a date reported about a credential it is not true of.
func TestAStampBelongsToTheVariableItSitsAbove(t *testing.T) {
	body := "# minted=2020-01-01\n# expires=2020-02-01\nOTHER=x\n\nCLAUDE_CODE_OAUTH_TOKEN=y\n"
	st, ok := readStamps(body, "CLAUDE_CODE_OAUTH_TOKEN")
	if !ok {
		t.Fatal("the variable was not found")
	}
	if !st.Minted.IsZero() || !st.Expires.IsZero() {
		t.Errorf("stamps from another variable were read as this one's: %+v", st)
	}
	tight := "# minted=2020-01-01\nOTHER=x\nCLAUDE_CODE_OAUTH_TOKEN=y\n"
	if st, ok := readStamps(tight, "CLAUDE_CODE_OAUTH_TOKEN"); !ok || !st.Minted.IsZero() {
		t.Errorf("a stamp carried across the variable it belongs to: %+v", st)
	}
	st, ok = readStamps(body, "OTHER")
	if !ok || st.Expires.Format(stampDate) != "2020-02-01" {
		t.Errorf("the stamps did not attach to the variable below them: %+v", st)
	}
}

// --paste is the headless box: the mint is not run, and the token still
// lands stamped. Without this the mint count in the write test proves only
// that a mint happened, not that --paste is what suppresses it.
func TestPasteSkipsTheMintAndStillWrites(t *testing.T) {
	a := refreshApp(t)
	mints := 0
	var w bytes.Buffer
	o := opts(RefreshOpts{Runtime: "claude", EnvSet: "container", Paste: true}, "carried-over", &mints)
	if err := a.CmdRefresh(&w, o); err != nil {
		t.Fatal(err)
	}
	if mints != 0 {
		t.Errorf("--paste ran the mint %d times", mints)
	}
	body, _ := os.ReadFile(filepath.Join(a.EnvsDir, "container.env"))
	if !strings.Contains(string(body), "CLAUDE_CODE_OAUTH_TOKEN=carried-over") {
		t.Errorf("--paste wrote nothing:\n%s", body)
	}
}

// A bad --expires is refused before anything is minted or written: a date
// posse cannot read must not become a file it half-wrote.
func TestABadExpiresIsRefusedBeforeTheMint(t *testing.T) {
	a := refreshApp(t)
	mints := 0
	var w bytes.Buffer
	err := a.CmdRefresh(&w, opts(RefreshOpts{Runtime: "claude", EnvSet: "container", Expires: "next tuesday"}, "tok", &mints))
	if err == nil || !strings.Contains(err.Error(), "YYYY-MM-DD") {
		t.Fatalf("a bad date was accepted: %v", err)
	}
	if mints != 0 {
		t.Errorf("the mint ran %d times before the date was checked", mints)
	}
}

// Nothing this command produces carries a credential — the same rule the
// seam's errors keep, checked over the write path's own output.
func TestNoRefreshOutputEverCarriesTheCredential(t *testing.T) {
	a := refreshApp(t)
	const secret = "sk-ant-oat01-do-not-print-me"
	writeEnvFile(t, a, "container", "CLAUDE_CODE_OAUTH_TOKEN="+secret+"\n", 0o600)
	var w bytes.Buffer
	if err := a.CmdRefresh(&w, opts(RefreshOpts{}, "", nil)); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(w.String(), secret) {
		t.Fatalf("the report printed a credential:\n%s", w.String())
	}
	w.Reset()
	if err := a.CmdRefresh(&w, opts(RefreshOpts{Runtime: "claude", EnvSet: "container"}, secret+"-new", nil)); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(w.String(), secret) {
		t.Fatalf("the write printed the credential:\n%s", w.String())
	}
}

// treeOf is every path under root with the size of each file, which is what
// "wrote nothing" has to be measured against: a claim of pure absence is
// satisfied by measuring nothing, so this measures the whole tree twice and
// compares the readings.
func treeOf(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		fmt.Fprintf(&b, "%s %d %04o\n", p, fi.Size(), fi.Mode().Perm())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if b.Len() == 0 {
		t.Fatalf("nothing under %s to compare — the fixture is empty", root)
	}
	return b.String()
}
