package rhq

// ranger-base-qxvh — nothing committed persona memory, so the ORDERS backlog
// rebuilt itself every time a persona ran (203 lines uncommitted on
// 2026-08-25, 1419 the next day, 1538 by the time a human landed them by
// hand). The kill is where sessions are destroyed and so the one event worth
// hanging the commit on.
//
// These tests stand up the shape the instance actually runs rather than a
// convenient one: a constitution checkout with `rhq/agents` beside
// `rhq/personas`, and the home's `personas/` a SYMLINK into it. Both halves
// are load-bearing — the symlink is how RHQ_HOME spells it on the box this
// was measured on, and `rhq/agents` sitting one directory over is the thing
// no sweep may ever take (ADR 0015 gates the constitution behind promote).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// memoryRepo is that shape. It returns the constitution checkout; each
// persona's memory dir is repo/rhq/personas/<name>, reachable from the home
// as b.App.PersonasDir()/<name>. Call it BEFORE anything creates a session:
// the launch materializes the memory dir, and a directory already standing
// at PersonasDir is a symlink that cannot be made.
func memoryRepo(t *testing.T, b *HerdrBackend, personas ...string) string {
	t.Helper()
	repo := wtRepo(t)
	write(t, filepath.Join(repo, "rhq", "agents", "dev.md"), "the constitution\n")
	for _, p := range append([]string{"dev"}, personas...) {
		write(t, filepath.Join(repo, "rhq", "personas", p, "ORDERS.md"), "# Standing orders — "+p+"\n")
	}
	mustGit(t, repo, "add", "--", "rhq")
	mustGit(t, repo, "commit", "-q", "-m", "seed the constitution", "--", "rhq")
	if err := os.MkdirAll(b.App.Home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(repo, "rhq", "personas"), b.App.PersonasDir()); err != nil {
		t.Fatal(err)
	}
	return repo
}

// appendOrders is a persona doing what its PID tells it to at the end of a
// session: append a lesson, and commit nothing (it cannot — the auto-mode
// classifier refuses a content commit outside the session's own cwd, which
// is why the launcher does it).
func appendOrders(t *testing.T, repo, persona, text string) {
	t.Helper()
	p := filepath.Join(repo, "rhq", "personas", persona, "ORDERS.md")
	body, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(body, []byte(text)...), 0o644); err != nil {
		t.Fatal(err)
	}
}

// headFiles is what the checkout's tip commit touched, one path per line.
func headFiles(t *testing.T, repo string) string {
	t.Helper()
	return mustGit(t, repo, "show", "--name-only", "--format=", "HEAD")
}

// ─── the commit that was missing ─────────────────────────────────────────────

// The headline. A session is killed; the lesson its persona appended is in
// git afterwards, and the memory dir is clean — which before this bead it
// never was, because nothing on any path committed it.
func TestKillCommitsThePersonaMemoryNothingElseWould(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	repo := memoryRepo(t, b)
	devSession(t, b, "s1")
	appendOrders(t, repo, "dev", "- gitRaw, not git, when the leading space is data.\n")

	landing, err := b.KillSessionAndLandOpts("s1", KillOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if dirty := b.App.MemoryDirtyPaths("dev"); len(dirty) != 0 {
		t.Fatalf("the kill left the persona's memory uncommitted: %v", dirty)
	}
	if got := headFiles(t, repo); strings.TrimSpace(got) != "rhq/personas/dev/ORDERS.md" {
		t.Fatalf("the commit took the wrong paths:\n%s", got)
	}
	if body := mustGit(t, repo, "show", "HEAD:rhq/personas/dev/ORDERS.md"); !strings.Contains(body, "leading space is data") {
		t.Errorf("the lesson is not in the commit:\n%s", body)
	}
	// The commit has to be traceable to why it happened: it is the launcher
	// writing on a persona's behalf, at the moment the persona is gone.
	if msg := mustGit(t, repo, "log", "-1", "--format=%B"); !strings.Contains(msg, "posse kill s1") || !strings.Contains(msg, "dev") {
		t.Errorf("the commit message does not say who or why:\n%s", msg)
	}
	if line := landing.Memory.Line(); !strings.Contains(line, "dev memory committed") {
		t.Errorf("the kill said nothing about it: %q", line)
	}
	if lines := landing.Lines(); len(lines) != 1 || lines[0] != landing.Memory.Line() {
		t.Errorf("the memory line must reach the caller: %v", lines)
	}
}

// The constraint the parent bead states in capitals: never `rhq/agents`.
// That is the constitution — the PIDs every future session runs under — and
// ADR 0015 puts it behind `posse promote` on purpose. It sits one directory
// from the memory this commits, which is exactly why a path-limited commit
// and not a repo-wide one.
func TestKillNeverCommitsTheConstitutionBesideTheMemory(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	repo := memoryRepo(t, b)
	devSession(t, b, "s1")
	appendOrders(t, repo, "dev", "- a lesson.\n")
	// A persona rewriting its own PID, and an untracked file beside it:
	// both are the class the sweep must walk past.
	write(t, filepath.Join(repo, "rhq", "agents", "dev.md"), "deny: []\n")
	write(t, filepath.Join(repo, "rhq", "agents", "new-persona.md"), "a persona nobody ratified\n")

	if _, err := b.KillSessionAndLandOpts("s1", KillOpts{}); err != nil {
		t.Fatal(err)
	}
	if got := headFiles(t, repo); strings.Contains(got, "rhq/agents") {
		t.Fatalf("the kill committed the constitution:\n%s", got)
	}
	// And left it exactly as it found it, still there for the operator.
	if body, _ := os.ReadFile(filepath.Join(repo, "rhq", "agents", "dev.md")); !strings.Contains(string(body), "deny: []") {
		t.Errorf("the constitution was not left alone: %q", body)
	}
	if st := mustGit(t, repo, "status", "--porcelain", "--", "rhq/agents"); !strings.Contains(st, "rhq/agents") {
		t.Errorf("the constitution's own changes must still be uncommitted: %q", st)
	}
}

// One kill lands ONE persona's memory. This is the objection that sank the
// periodic-sweep design: a commit that takes another persona's memory from
// an unrelated session's context detaches the write from the only actor that
// could explain it, and is the shared-index shape ADR 0022 is about.
func TestKillCommitsOnlyTheKilledSessionsPersona(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	repo := memoryRepo(t, b)
	write(t, filepath.Join(repo, "rhq", "personas", "other", "ORDERS.md"), "# Standing orders — other\n")
	devSession(t, b, "s1")
	appendOrders(t, repo, "dev", "- dev's lesson.\n")

	if _, err := b.KillSessionAndLandOpts("s1", KillOpts{}); err != nil {
		t.Fatal(err)
	}
	if got := headFiles(t, repo); strings.Contains(got, "other") {
		t.Fatalf("dev's kill committed another persona's memory:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(repo, "rhq", "personas", "other", "ORDERS.md")); err != nil {
		t.Errorf("the other persona's memory was disturbed: %v", err)
	}
}

// ─── the credential scan ─────────────────────────────────────────────────────

// 1538 lines of agent-authored prose is exactly where a credential gets
// quoted by accident. The human who landed that batch grepped it first, and
// that check is part of the mechanism or it is not a check at all.
func TestKillHoldsMemoryThatLooksLikeACredential(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	repo := memoryRepo(t, b)
	devSession(t, b, "s1")
	const leaked = "sk-ant-api03-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	appendOrders(t, repo, "dev", "- the key that worked: "+leaked+"\n")
	// The seed commit already touched rhq/personas, so what proves nothing
	// was committed is that the tip did not move at all.
	before := mustGit(t, repo, "rev-parse", "HEAD")

	landing, err := b.KillSessionAndLandOpts("s1", KillOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if after := mustGit(t, repo, "rev-parse", "HEAD"); after != before {
		t.Fatalf("a credential shape was committed as %s:\n%s", after, headFiles(t, repo))
	}
	if dirty := b.App.MemoryDirtyPaths("dev"); len(dirty) != 1 {
		t.Errorf("the held file must be left exactly where it was: %v", dirty)
	}
	// Nothing staged either: the scan runs before the add, so a refusal
	// leaves the shared index as untouched as the file.
	if st := mustGit(t, repo, "diff", "--cached", "--name-only"); strings.TrimSpace(st) != "" {
		t.Errorf("a held commit left paths staged: %q", st)
	}
	line := landing.Memory.Line()
	if !strings.Contains(line, "ORDERS.md:2") || !strings.Contains(line, "an Anthropic key") {
		t.Errorf("the refusal must name the file, the line and the shape: %q", line)
	}
	// It must never print what it matched. Saying it out loud publishes the
	// thing the refusal exists to keep out of git — into a terminal, a log,
	// and this process's own output.
	if strings.Contains(line, leaked) {
		t.Errorf("the refusal echoed the credential: %q", line)
	}
	// And the kill still happened: a commit that cannot be made is no
	// reason to keep a session the operator asked to close.
	if _, ok := b.readMeta("s1"); ok {
		t.Error("a held memory commit must not stop the kill")
	}
}

// The load-bearing half of the scan: it reads what the commit ADDS, not the
// whole file. Persona memory legitimately quotes credential shapes — the
// fleet's security persona keeps a leak canary spelled out in its own notes
// — and those lines are already committed. Scanning whole files would hold
// that persona's every future commit forever on prose git has held for
// weeks, which is this bead's own defect wearing a safety label.
func TestTheCredentialScanReadsOnlyWhatTheCommitAdds(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	repo := memoryRepo(t, b)
	devSession(t, b, "s1")
	// Already in git, and staying there.
	appendOrders(t, repo, "dev", "- control pid ran `-H Authorization: Bearer CANARY_SETUP_TOKEN_0n6_DO_NOT_LEAK`\n")
	mustGit(t, repo, "commit", "-q", "-m", "the canary, landed by hand", "--", "rhq/personas/dev/ORDERS.md")
	if strings.TrimSpace(mustGit(t, repo, "status", "--porcelain")) != "" {
		t.Fatal("the fixture must start clean")
	}
	appendOrders(t, repo, "dev", "- and an ordinary lesson.\n")

	if _, err := b.KillSessionAndLandOpts("s1", KillOpts{}); err != nil {
		t.Fatal(err)
	}
	if dirty := b.App.MemoryDirtyPaths("dev"); len(dirty) != 0 {
		t.Fatalf("a credential shape already IN git held a later commit: %v", dirty)
	}
	if body := mustGit(t, repo, "show", "HEAD:rhq/personas/dev/ORDERS.md"); !strings.Contains(body, "ordinary lesson") {
		t.Errorf("the new lesson did not land:\n%s", body)
	}
}

// A persona's memory dir is not only ORDERS.md, and a file git has never
// seen has no HEAD side to diff against — so the scan reads it whole. Its
// first line is line 1, which is what the refusal must say.
func TestTheCredentialScanReadsWholeUntrackedFiles(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	repo := memoryRepo(t, b)
	devSession(t, b, "s1")
	write(t, filepath.Join(repo, "rhq", "personas", "dev", "notes", "probe.md"),
		"curl -H 'Authorization: Bearer ya29.A0ARrdaM9xxxxxxxxxxxxxxxxxxxxxx' https://x\n")

	landing, err := b.KillSessionAndLandOpts("s1", KillOpts{})
	if err != nil {
		t.Fatal(err)
	}
	line := landing.Memory.Line()
	if !strings.Contains(line, "notes/probe.md:1") || !strings.Contains(line, "a bearer token") {
		t.Errorf("an untracked file must be scanned whole and named: %q", line)
	}
	if got := headFiles(t, repo); strings.Contains(got, "probe.md") {
		t.Fatalf("the untracked credential was committed:\n%s", got)
	}
}

// The other direction, and the one that decides whether this feature helps
// or hurts: prose ABOUT credentials is not a credential. Persona memory is
// thousands of lines of exactly this, so a scan keyed on the WORDS would
// hold every commit and rebuild the backlog it was added to prevent.
func TestTheCredentialScanDoesNotFireOnProseAboutCredentials(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	repo := memoryRepo(t, b)
	devSession(t, b, "s1")
	appendOrders(t, repo, "dev", strings.Join([]string{
		"- the setup-token prefix is sk-ant-oat01-… and the metered one is sk-ant-api.",
		"- the probe sends an Authorization: Bearer header, never an argv.",
		"- `password:` in the recipe means the operator is asked, not stored.",
		"- accessToken is the field credShapes reads out of the keychain envelope.",
		"- -----BEGIN is how you know somebody pasted a PEM into a bead.",
		"- the path is /Users/x/.config/posse/state/plan-usage-cache.json, no secret in it.",
		"",
	}, "\n"))

	if _, err := b.KillSessionAndLandOpts("s1", KillOpts{}); err != nil {
		t.Fatal(err)
	}
	if dirty := b.App.MemoryDirtyPaths("dev"); len(dirty) != 0 {
		t.Fatalf("prose about credentials held the commit: %v", dirty)
	}
}

// ─── the landing turn ────────────────────────────────────────────────────────

// The turn is gated on the persona HAVING memory no commit holds. Without
// that gate every reap costs a real turn — the ~30-session sweeps this
// instance does in a day included — for a session with nothing to say.
func TestKillSpendsNoTurnOnAPersonaWithNothingToLand(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	memoryRepo(t, b)
	devSession(t, b, "s1")

	var out strings.Builder
	if _, err := b.KillSessionAndLandOpts("s1", KillOpts{Land: true, Out: &out}); err != nil {
		t.Fatal(err)
	}
	if log := calls(t, fake); strings.Contains(log, "agent prompt") {
		t.Errorf("a clean memory must cost no turn:\n%s", log)
	}
}

// And when there IS something, the turn happens — with the prompt that says
// this session is ENDING, and that the persona must not try to commit its
// own orders (in its own worktree it cannot; the launcher does it).
func TestKillLandsThePlaneWhenThereIsMemoryToLand(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	repo := memoryRepo(t, b)
	devSession(t, b, "s1")
	m, _ := b.readMeta("s1")
	appendOrders(t, repo, "dev", "- a lesson.\n")

	var out strings.Builder
	if _, err := b.KillSessionAndLandOpts("s1", KillOpts{Land: true, Out: &out}); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	log := calls(t, fake)
	iWait, iPrompt, iClose := strings.Index(log, "agent wait"), strings.Index(log, "agent prompt"), strings.Index(log, "workspace close "+m.Workspace)
	if iWait < 0 || iPrompt < iWait || iClose < iPrompt {
		t.Fatalf("expected wait → prompt → close, got %d/%d/%d:\n%s", iWait, iPrompt, iClose, log)
	}
	for _, want := range []string{
		"about to be CLOSED",
		"nothing takes over",
		"ORDERS.md",
		"Do NOT commit that file",
		"Push only what your own guardrails permit",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("kill landing prompt missing %q:\n%s", want, log)
		}
	}
	if !strings.Contains(out.String(), "landing s1") {
		t.Errorf("a kill that has become slow must say why while it is being slow: %q", out.String())
	}
	// The commit is taken AFTER the workspace closes, so the last writer to
	// that file is gone before it is read.
	if iCommit := strings.Index(log, "workspace close"); iCommit < 0 {
		t.Fatal("no close in the log")
	}
	if dirty := b.App.MemoryDirtyPaths("dev"); len(dirty) != 0 {
		t.Errorf("the turn ran but the memory did not land: %v", dirty)
	}
}

// A turn that never settles stops the kill — the same answer relaunch gives,
// for the same reason: closing a workspace whose agent may be mid-commit is
// the loss this path exists to prevent. And it says so loudly, because a
// kill that silently became a no-op is its own surprise.
func TestKillRefusesWhileTheLandingTurnIsUnsettled(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	repo := memoryRepo(t, b)
	devSession(t, b, "s1")
	m, _ := b.readMeta("s1")
	appendOrders(t, repo, "dev", "- a lesson.\n")
	os.WriteFile(filepath.Join(fake, "wait-status"), []byte("working"), 0o644)

	var out strings.Builder
	_, err := b.KillSessionAndLandOpts("s1", KillOpts{Land: true, Out: &out})
	if err == nil || !strings.Contains(err.Error(), "NOT killed") || !strings.Contains(err.Error(), "--no-land") {
		t.Fatalf("a working session must be refused, naming the way through: %v", err)
	}
	if strings.Contains(calls(t, fake), "workspace close") {
		t.Error("a refused kill must not close the workspace")
	}
	if m2, ok := b.readMeta("s1"); !ok || m2.Workspace != m.Workspace {
		t.Errorf("a refused kill must leave the session alone: %+v", m2)
	}

	// --no-land is the way through, and it declines the TURN only: the
	// commit is what makes the memory durable and it still happens. A
	// wedged session's memory is not less worth keeping than a settled
	// one's — it is more.
	out.Reset()
	if _, err := b.KillSessionAndLandOpts("s1", KillOpts{Out: &out}); err != nil {
		t.Fatalf("--no-land: %v\n%s", err, out.String())
	}
	if strings.Contains(calls(t, fake), "agent prompt") {
		t.Errorf("--no-land must not prompt:\n%s", calls(t, fake))
	}
	if dirty := b.App.MemoryDirtyPaths("dev"); len(dirty) != 0 {
		t.Fatalf("--no-land dropped the memory instead of committing it: %v", dirty)
	}
	if got := headFiles(t, repo); strings.TrimSpace(got) != "rhq/personas/dev/ORDERS.md" {
		t.Errorf("--no-land committed the wrong thing:\n%s", got)
	}
}

// ─── the homes that have nothing to land ─────────────────────────────────────

// The default install keeps `personas/` outside git, and posse must not
// require the operator to have made one a checkout. A kill there is exactly
// the kill that shipped before this bead: no commit, no line, no error.
func TestKillSaysNothingWhenTheHomeKeepsMemoryOutsideGit(t *testing.T) {
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	devSession(t, b, "s1")
	write(t, filepath.Join(b.App.PersonasDir(), "dev", "ORDERS.md"), "# Standing orders\n")

	landing, err := b.KillSessionAndLandOpts("s1", KillOpts{Land: true})
	if err != nil {
		t.Fatal(err)
	}
	if landing.Memory != nil || len(landing.Lines()) != 0 {
		t.Errorf("a non-git memory dir must be silent, got %+v", landing.Memory)
	}
	if log := calls(t, fake); strings.Contains(log, "agent prompt") {
		t.Errorf("nothing landable must cost no turn:\n%s", log)
	}
}

// ─── the porcelain read ──────────────────────────────────────────────────────

// `-z` spends a SECOND field on a rename's source path — it drops the
// ` -> ` spelling the human format uses. Left in place that field is read as
// a record whose first four bytes are a path's, and the scan is then aimed
// at a filename made of somebody's directory name.
func TestPorcelainZKeepsRenamesAndOddPathsWhole(t *testing.T) {
	in := []byte("R  rhq/personas/dev/NEW.md\x00rhq/personas/dev/OLD.md\x00 M rhq/personas/dev/two words.md\x00?? rhq/personas/dev/plain.md\x00")
	got := porcelainZChanges(in)
	want := []memoryChange{
		{Path: "rhq/personas/dev/NEW.md"},
		{Path: "rhq/personas/dev/two words.md"},
		{Path: "rhq/personas/dev/plain.md", Untracked: true},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("record %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// ─── the path that actually reaps ────────────────────────────────────────────

// The end-of-pass auto-reaper is where sessions are destroyed at scale —
// ~30 in a day on the instance this bead was filed from — so it is the path
// where memory was actually being lost. It commits, and it spends no landing
// turn doing it: the bead is closed and the agent has settled, so the
// persona already had its wrap-up, and N bounded turns in a row would stall
// the pass this sweep is an epilogue to.
func TestAutoReapCommitsThePersonaMemoryAndSpendsNoTurn(t *testing.T) {
	wtqaHome(t)
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	exe, _ := os.Executable()
	b.Bd = Bd{Bin: exe}
	con := memoryRepo(t, b, "ranger")
	repo := wtqaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, `[{"id":"a-1","status":"closed"}]`)
	idleClaude(t, fake)

	session := SessionForBead("ranger", repo, "a-1")
	tr, err := b.App.EnsureSessionTree(repo, session, nil)
	if err != nil {
		t.Fatal(err)
	}
	fakeBdInTree(t, repo, tr.Path, `[{"id":"a-1","status":"closed"}]`)
	commitIn(t, tr.Path, "fix.txt", "the persona's work\n", "a-1: the fix")
	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	// The lesson the persona appended while it worked, which nothing but
	// this sweep will ever commit.
	appendOrders(t, con, "ranger", "- a lesson the reap must not lose.\n")

	write(t, filepath.Join(repo, "fake-ready.json"), `[]`)
	agePrompt(t, b, session, d.PromptGrace+time.Minute)
	d2 := newTestDispatcher(t, b)
	if _, err := d2.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d2)

	if _, ok := b.readMeta(session); ok {
		t.Fatalf("the sweep did not reap the session:\n%s", out)
	}
	if dirty := b.App.MemoryDirtyPaths("ranger"); len(dirty) != 0 {
		t.Fatalf("the reap left the persona's memory uncommitted: %v\n%s", dirty, out)
	}
	if body := mustGit(t, con, "show", "HEAD:rhq/personas/ranger/ORDERS.md"); !strings.Contains(body, "must not lose") {
		t.Errorf("the lesson is not in the commit:\n%s", body)
	}
	if !strings.Contains(out, "ranger memory committed") {
		t.Errorf("the sweep must say what it did with the memory:\n%s", out)
	}
	// Not "no agent prompt" — the pass sent this session its WORK prompt,
	// and that is in the same log. What must be absent is the landing turn,
	// which is the only thing that says this.
	if log := calls(t, fake); strings.Contains(log, "about to be CLOSED") {
		t.Errorf("the sweep must spend no landing turn:\n%s", log)
	}
}
