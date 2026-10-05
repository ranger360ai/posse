package posse

// QA pins for ranger-base-3xt9y — the readings log (ADR 0066 D1).
//
// The bead's own measure-before-closing list is arm 4: replay this weekend's
// incidents from the corpus shape and show each would have been captured
// with enough bytes to reproduce. The other arms pin the two rules the bead
// names outright — the redaction, and never-under-the-home — plus the two
// properties the log would otherwise break on its way in: a dirty session
// tree, and a quiet-tree clock that never goes quiet.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// rlTree is a repo with one session worktree cut off it, and the App that
// owns both. The log's placement is a fact about a real git worktree — its
// `.git` is a FILE pointing at an admin dir inside the main checkout — so
// nothing here is faked.
func rlTree(t *testing.T) (*App, string, *SessionTree) {
	t.Helper()
	a := hermetic(t, NewAppAt(gitTempDir(t)))
	repo := wtRepo(t)
	tree, err := a.EnsureSessionTree(repo, "ranger-posse-rl-1", nil)
	if err != nil || tree == nil {
		t.Fatalf("EnsureSessionTree: %v (tree=%v)", err, tree)
	}
	return a, repo, tree
}

// appendRawReading writes one record to the log the way AppendReading
// would, bypassing the writer's own decisions about it. The one record a
// pin cannot otherwise stage is a REDACTED one: `Redacted` is set by
// AppendReading from the instance's data ceiling and never by its caller,
// so an App with no ceiling configured cannot be made to write one. The
// line is still json.Marshal of the real type, so nothing here invents a
// shape the writer could not produce.
func appendRawReading(t *testing.T, a *App, treePath string, r Reading) {
	t.Helper()
	path, err := a.ReadingsLogPath(treePath)
	if err != nil {
		t.Fatal(err)
	}
	line, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		t.Fatal(err)
	}
}

// TestQAReadingsLogLivesUnderTheSessionTreeAndNeverTheHome is ADR 0066 D1's
// placement rule, in the only spelling a pin can hold.
//
// "Never under $HOME" cannot mean the home DIRECTORY: WorktreeRoot refuses a
// worktree root outside $HOME on purpose, because a reaped worktree under a
// live session destroys the work in it, so every session tree there will
// ever be is under the home. What the rule means is that the log's lifetime
// is the TREE's and not the INSTANCE's — so the arms are: it lands in the
// tree's own git dir, a path in one of the instance's own stores is refused
// outright, and a session with no tree gets no log rather than a fallback.
func TestQAReadingsLogLivesUnderTheSessionTreeAndNeverTheHome(t *testing.T) {
	t.Parallel()
	a, repo, tree := rlTree(t)

	// ARM 1 — the tree's own git dir, which git removes with the tree.
	got, err := a.ReadingsLogPath(tree.Path)
	if err != nil {
		t.Fatalf("ReadingsLogPath(%s): %v", tree.Path, err)
	}
	want := filepath.Join(repo, ".git", "worktrees", filepath.Base(tree.Path), ReadingsLogName)
	// samePath and not a string compare: `want` does not exist yet (nothing
	// has been appended), and EvalSymlinks answers differently for a path
	// that is there and one that is not — /var against /private/var on this
	// box, which is a fixture artefact and not the rule being pinned.
	if !samePath(filepath.Dir(got), filepath.Dir(want)) || filepath.Base(got) != filepath.Base(want) {
		t.Errorf("readings log at %s, want %s — the log is per session tree and rotated with it (ADR 0066 D1)", got, want)
	}

	// ARM 2 — a reading written there leaves the WORKING tree clean. Six
	// callers act on `git status --porcelain`, untracked files included:
	// ADR 0041's closed-dirty comment, RemoveSessionTree's refusal, the
	// retire guard, the land sweep, the reap guard. A log in the tree would
	// make every close a dirty close.
	if err := a.AppendReading(tree.Path, Reading{Decision: DecisionPaneState, Consequence: ConsequenceHold}); err != nil {
		t.Fatalf("AppendReading: %v", err)
	}
	if d := dirtyPaths(tree.Path); len(d) > 0 {
		t.Errorf("a logged reading made the session tree dirty (%v) — every close in it would now be a closed-dirty close (ADR 0041)", d)
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("nothing was written to %s: %v", got, err)
	}

	// ARM 3 — no tree, no log. The refusal is the rule: a fallback path is
	// the one thing ADR 0066 D1 rules out, and a writer that invented one
	// would put a session's readings somewhere nothing rotates.
	if _, err := a.ReadingsLogPath(""); err == nil {
		t.Error("ReadingsLogPath(\"\") resolved a path — a reading posse cannot place is a reading posse does not write")
	}
	if _, err := a.ReadingsLogPath(gitTempDir(t)); err == nil {
		t.Error("ReadingsLogPath resolved a path for a directory that is not a git work tree")
	}

	// ARM 4 — and a git dir that DOES land in one of the instance's own
	// stores is refused, measured against a real repo made inside each one
	// rather than against a string. These are the arms that fail if the
	// bound check is deleted; arms 1-3 would all still pass.
	//
	// ALL THREE BOUNDS, one repo each (ranger-base-r5546). readingsLogOutOfBounds
	// carries three and this arm used to make a repo in one of them — and
	// because the state dir is UNDER the harness home, that one repo was
	// refused by the first row whichever of the other two was deleted:
	// dropping the `a.Home` row and dropping the `~/.claude` row were both
	// SURVIVORS across all six readings pins (MEASURED 2026-10-04, M10 and
	// M11 on ranger-base-qr0as). The home arm's shape is a session tree
	// that is a standalone repo rather than a linked worktree; the
	// `~/.claude` one is the runtime's own per-user dir, which is not
	// posse's to write a log into at all.
	// The third one is made with MkdirTemp: `~/.claude` is the one bound
	// that is not per-App — it is this test BINARY's temp home, shared with
	// every other test in the package — so a fixed name would be two live
	// tests' under `-count=2`, which is how this shop measures a flake rate.
	claudeDir := filepath.Join(ExpandTilde("~"), ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	inClaude, err := os.MkdirTemp(claudeDir, "rl-repo-")
	if err != nil {
		t.Fatal(err)
	}
	// Tolerantly, and by hand: this root is not gitTempDir's, because the
	// bound under test IS `~/.claude` and no helper allocates there. A git
	// repo's own index can still be open for a beat after the command that
	// wrote it returned (commitwall_qa_test.go).
	t.Cleanup(func() { removeAllTolerant(t, inClaude) })
	// EACH ARM ASKS FOR ITS OWN ROW'S REASON, not just for a refusal. The
	// state dir is UNDER the harness home, so its row is unreachable by
	// outcome alone — delete it and the home row refuses the same path,
	// green. What the row carries that the home row does not is why: a
	// store that outlives every session is a different fact about the same
	// path from a fifth store ADR 0011 refuses, and the operator reading
	// the refusal is owed the one that applies.
	for _, bound := range []struct{ what, dir, why string }{
		{"the instance's own state dir", filepath.Join(a.StateDir, "repo"), "the state dir outlives every session"},
		{"the harness home, outside the state dir", filepath.Join(a.Home, "repo"), "ADR 0011 refuses"},
		{"the runtime's own per-user dir", inClaude, "not posse's to write a log into"},
	} {
		if err := os.MkdirAll(bound.dir, 0o755); err != nil {
			t.Fatal(err)
		}
		mustGit(t, bound.dir, "init", "-q", "-b", "main", ".")
		p, err := a.ReadingsLogPath(bound.dir)
		if err == nil {
			t.Errorf("ReadingsLogPath resolved %s, inside %s — that store outlives every session, so nothing in it is rotated with a tree", p, bound.what)
			continue
		}
		if !strings.Contains(err.Error(), "ADR 0066 D1") {
			t.Errorf("the refusal for %s does not name the rule it is enforcing: %v", bound.what, err)
		}
		if !strings.Contains(err.Error(), bound.why) {
			t.Errorf("a log under %s was refused for the wrong reason: %v\n  wanted the refusal to say %q — %s is a bound of its own, and a path it catches must be told why by ITS row and not by whichever row happens to contain it", bound.what, err, bound.why, bound.what)
		}
	}
}

// TestQAReadingsLogRedactsCeilingContentBeforeWrite is the bead's other
// named rule. The region bytes are a capture of somebody's terminal, so
// they go through the data ceiling on the way in (ADR 0050): class-only,
// never the text, and the record says which classes it lost so a corpus
// case that cannot be replayed byte for byte does not look complete.
func TestQAReadingsLogRedactsCeilingContentBeforeWrite(t *testing.T) {
	// No t.Parallel: qaCeilingWall sets the environment (newVisWallCfg's
	// t.Setenv), which Go refuses to let a parallel test do.
	w := qaCeilingWall(t, "")
	a := hermetic(t, &App{ConfigPath: filepath.Join(w.home, "config.yaml"), Home: w.home, StateDir: filepath.Join(w.home, "state")})
	repo := wtRepo(t)
	tree, err := a.EnsureSessionTree(repo, "ranger-posse-rl-2", nil)
	if err != nil || tree == nil {
		t.Fatalf("EnsureSessionTree: %v", err)
	}

	// The hit sits INSIDE an ordinary footer, which is the real shape: a
	// region is a screen line and the class is something that happened to
	// be drawn on it.
	footer := "⏵⏵ auto mode on · 1 shell · " + qaCeilingHit + " · ← for agents"
	clean := "❯ keep waiting, then commit and close"
	if err := a.AppendReading(tree.Path, Reading{
		Decision:    DecisionComposerHold,
		Verdict:     "1 shell still running",
		Consequence: ConsequenceHold,
		Rule:        RuleComposerHold,
		Herdr: ReadingEvidence{State: "idle", Regions: []ReadingRegion{
			{Name: footerRegion, Bytes: len(footer), Text: footer},
			{Name: composerRegion, Bytes: len(clean), Text: clean},
		}},
	}); err != nil {
		t.Fatalf("AppendReading: %v", err)
	}

	path, err := a.ReadingsLogPath(tree.Path)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// THE WHOLE FILE, not the parsed record: a redaction that put the text
	// back through a second field would be green against a parsed read of
	// the first one.
	if strings.Contains(string(raw), qaCeilingHit) {
		t.Errorf("the readings log carries data-ceiling content verbatim — this content may not exist in a local file at all (ADR 0050):\n%s", raw)
	}
	if !strings.Contains(string(raw), "["+RedactedMark+":"+qaCeilingClass+"]") {
		t.Errorf("the ceiling took the text and left no marker naming the class, so a reader of the corpus cannot tell a hole from a short region:\n%s", raw)
	}

	rs, err := ReadReadings(path)
	if err != nil || len(rs) != 1 {
		t.Fatalf("ReadReadings: %v (%d records)", err, len(rs))
	}
	if got := rs[0].Redacted; len(got) != 1 || got[0] != qaCeilingClass {
		t.Errorf("record names redacted classes %v, want exactly [%s] — a case whose bytes are not the bytes read has to say so", got, qaCeilingClass)
	}
	// The CONTROL, and it is the half a redactor that redacted everything
	// would fail: a region with no ceiling class in it arrives untouched,
	// or the corpus is worthless for the readings that matter.
	reg, ok := rs[0].ReadingRegionOf(composerRegion)
	if !ok || reg.Text != clean {
		t.Errorf("the composer region carried no ceiling class and came back %q, want %q", reg.Text, clean)
	}

	// And a log with NO ceiling configured leaves the same footer alone —
	// the mirror arm, so "redacted" cannot be a thing this does to every
	// reading on every instance.
	bare, _, bareTree := rlTree(t)
	if err := bare.AppendReading(bareTree.Path, Reading{
		Decision: DecisionComposerHold, Consequence: ConsequenceHold,
		Herdr: ReadingEvidence{Regions: []ReadingRegion{{Name: footerRegion, Text: footer}}},
	}); err != nil {
		t.Fatal(err)
	}
	bp, _ := bare.ReadingsLogPath(bareTree.Path)
	brs, err := ReadReadings(bp)
	if err != nil || len(brs) != 1 {
		t.Fatalf("ReadReadings (no ceiling): %v (%d)", err, len(brs))
	}
	if r, _ := brs[0].ReadingRegionOf(footerRegion); r.Text != footer || len(brs[0].Redacted) != 0 {
		t.Errorf("an instance with no data_ceiling_patterns: redacted anyway (%q, classes %v) — the ceiling is an instance's own vocabulary and an empty list is not a wall", r.Text, brs[0].Redacted)
	}
}

// TestQAReadingsLogDoesNotHoldTheQuietTreeClockOpen is the property that
// decided where the log goes.
//
// lastTreeWrite (retire.go) takes the newest mtime of every file in the
// session's git dir, and a reading of a settled seat is appended on every
// pass that holds it — which is exactly the tree a retire is for. Counting
// it would make any tree that ever took a reading unretirable: the same
// shape as `git status`'s index refresh holding the clock open, twice paid
// (ranger-base-9u5zy, -a8tqz).
//
// Measured by mtime rather than by waiting: the log is stamped in the
// FUTURE, so a walk that reads it cannot avoid saying so.
func TestQAReadingsLogDoesNotHoldTheQuietTreeClockOpen(t *testing.T) {
	t.Parallel()
	a, _, tree := rlTree(t)
	if err := a.AppendReading(tree.Path, Reading{Decision: DecisionComposerHold, Consequence: ConsequenceHold}); err != nil {
		t.Fatalf("AppendReading: %v", err)
	}
	log, err := a.ReadingsLogPath(tree.Path)
	if err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(72 * time.Hour)
	if err := os.Chtimes(log, future, future); err != nil {
		t.Fatal(err)
	}
	quiet, ok := lastTreeWrite(tree)
	if !ok {
		t.Fatal("lastTreeWrite could not read the tree at all")
	}
	if !quiet.Before(future.Add(-time.Hour)) {
		t.Errorf("lastTreeWrite read the readings log's own mtime (%s) — a tree that ever took a reading could never be retired", quiet)
	}

	// THE CONTROL. The same future mtime on any OTHER file in the same
	// directory IS read, so the arm above is measuring the skip and not a
	// walk that never reaches that directory.
	gd := filepath.Dir(log)
	other := filepath.Join(gd, "posse-readings-control")
	if err := os.WriteFile(other, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(other, future, future); err != nil {
		t.Fatal(err)
	}
	if loud, ok := lastTreeWrite(tree); !ok || loud.Before(future.Add(-time.Hour)) {
		t.Errorf("a future-stamped file beside the log read as %s (ok=%v) — the walk does not reach this directory, so the arm above proves nothing", loud, ok)
	}
}

// TestQAReadingsLogReplaysThisWeekendsIncidents is the bead's own
// measure-before-closing arm: each incident in its list, built in the corpus
// shape, and the claim checked rather than asserted — that the record
// carries enough bytes to reproduce the verdict.
//
// WHAT "REPRODUCE" MEANS IS DIFFERENT FOR THE TWO READER FAMILIES, and the
// arms say which they are:
//
//	D4  posse's own Go rules (panework.go, ghostbox.go). Re-decided IN
//	    PROCESS off the logged bytes, so the arm is an equality against
//	    today's rules and reds the day a rule changes meaning.
//	D1/D2/D3  herdr's TOML manifest. posse has no second implementation of
//	    those rules and must not grow one, so the claim checkable here is
//	    that the BYTES round-trip exactly — every codepoint, since each of
//	    ranger-base-0sa5a's three misses was one — and the corpus export
//	    puts them in a file `herdr agent explain --file` can be pointed at.
//	D5  reads no screen at all, so there are no bytes and the arm pins that
//	    the record says so rather than carrying somebody else's.
func TestQAReadingsLogReplaysThisWeekendsIncidents(t *testing.T) {
	t.Parallel()
	a, _, tree := rlTree(t)

	// ranger-base-htafy / rangerhq-7ia: the footer. htafy's incident is a
	// seat idle behind its own suite run; 7ia's is a footer REWORD that
	// flipped a reading in silence. Both turn on the footer's bytes, and
	// posse's own footer reader is the one this arm can re-run.
	footer := "⏵⏵ auto mode on · 1 shell, 1 monitor · esc to interrupt · ← for agents · ↓ to manage"
	// ranger-base-6o7wm: the box drawing claude's own suggestion FAINT.
	// SGR 2 is the whole discriminator and it lives in the ansi line, not
	// in herdr's stripped preview — so a record that kept only the preview
	// could not reproduce this verdict at all.
	ghostText := "ping me when it's green"
	ghostANSI := composerMark + " \x1b[0m\x1b[2m" + ghostText + "\x1b[0m"
	// ranger-base-0sa5a: each of the three rule misses was ONE codepoint —
	// a `→`, a hyphen — so this is the region that proves the log is a
	// corpus and not a counter.
	blocked := "│ Do you want to proceed? → 1. Yes  2. No, and tell Claude what to do differently │"

	// The records, in the order the arms below read them back. The
	// reproduction claim is checked over the record as it came OUT of the
	// log and not over the struct that went in: the question is whether the
	// LOG reproduces the verdict, not whether the struct does.
	for _, rec := range []Reading{
		// 0 — ranger-base-htafy
		{
			Decision: DecisionComposerHold, Consequence: ConsequenceHold,
			Verdict: "1 shell, 1 monitor still running", Rule: RuleComposerHold,
			Herdr: ReadingEvidence{State: "idle", Rule: "live_prompt_box", Seen: true,
				Regions: []ReadingRegion{
					{Name: footerRegion, Bytes: len(footer), Text: footer},
					{Name: composerRegion, Bytes: 1, Text: composerMark},
				}},
		},
		// 1 — ranger-base-6o7wm
		{
			Decision: DecisionComposerHold, Consequence: ConsequenceGhostRetired,
			Verdict: "ghost: " + ghostText, Rule: RuleComposerGhost,
			Herdr: ReadingEvidence{State: "idle", Rule: "live_prompt_box", Seen: true,
				Regions: []ReadingRegion{
					{Name: composerRegion, Bytes: len(composerMark + " " + ghostText), Text: composerMark + " " + ghostText},
					{Name: composerANSIRegion, Bytes: len(ghostANSI), Text: ghostANSI},
				}},
		},
		// 2 — ranger-base-0sa5a
		{
			Decision: DecisionDialog, Consequence: ConsequenceHandBack,
			Verdict: "hand back: herdr: agent is blocked (agent_blocked)", Rule: "blocked_confirm_arrow",
			Herdr: ReadingEvidence{State: "blocked", Rule: "blocked_confirm_arrow", Seen: true,
				Regions: []ReadingRegion{{Name: "whole_recent", Bytes: len(blocked), Text: blocked}}},
		},
		// 3 — ranger-base-uauvn
		{
			Decision: DecisionStallVerdict, Consequence: ConsequenceHandBack,
			Verdict: "hand back: no turn started in 45s either, and no commit on the branch",
			Rule:    RuleStallVerdict,
			Herdr:   ReadingEvidence{State: stallNoTurn, Seen: true},
		},
		// 4 — ranger-base-qcu4c
		{
			Decision: DecisionTurnDelivered, Consequence: ConsequenceRefusal,
			Verdict: "refused", Rule: RuleTurnOutcome,
			Herdr: ReadingEvidence{State: "answered", Seen: true,
				Regions: []ReadingRegion{{
					Name: TurnOutcomeRegion,
					Text: "You've reached your usage limit. Upgrade at /usage-credits or switch models with /model.",
				}}},
		},
	} {
		if err := a.AppendReading(tree.Path, rec); err != nil {
			t.Fatalf("AppendReading: %v", err)
		}
	}

	path, err := a.ReadingsLogPath(tree.Path)
	if err != nil {
		t.Fatal(err)
	}
	rs, err := ReadReadings(path)
	if err != nil {
		t.Fatalf("ReadReadings: %v", err)
	}
	if len(rs) != 5 {
		t.Fatalf("%d records in the log, want 5", len(rs))
	}
	for n, r := range rs {
		switch n {
		case 0:
			hold := r.ReplayHold()
			if hold.Work != "1 shell, 1 monitor" || !hold.Waiting() {
				t.Errorf("htafy: replaying the logged footer gives %+v — the record does not reproduce its own verdict", hold)
			}
		case 1:
			ansi, ok := r.ReadingRegionOf(composerANSIRegion)
			if !ok {
				t.Fatal("6o7wm: no ansi composer line in the record — the dim run is the whole discriminator (ranger-base-6o7wm)")
			}
			if ghost, read := composerGhostRead(ansi.Text, ghostText); !ghost {
				t.Errorf("6o7wm: replaying the logged ansi line does not reproduce the ghost verdict (read %q)", read)
			}
			if undim, _ := composerGhostRead(strings.ReplaceAll(ansi.Text, "\x1b[2m", ""), ghostText); undim {
				t.Error("6o7wm: the same line with no SGR 2 still reads as a ghost — the replay is not reading the dim run")
			}
		case 2:
			reg, ok := r.ReadingRegionOf("whole_recent")
			if !ok {
				t.Fatal("0sa5a: no screen region in the record — a D2 hand-back with no bytes reproduces nothing")
			}
			if reg.Text != blocked {
				t.Errorf("0sa5a: the screen region did not round-trip byte for byte:\n got %q\nwant %q", reg.Text, blocked)
			}
			if !strings.Contains(reg.Text, "→") {
				t.Error("0sa5a: the `→` each of its misses turned on did not survive the log")
			}
		case 3:
			if len(r.Herdr.Regions) != 0 {
				t.Errorf("uauvn: the D5 record carries %d region(s) — the stall verdict reads no screen", len(r.Herdr.Regions))
			}
			for _, want := range []string{"no turn started", "no commit"} {
				if !strings.Contains(r.Verdict, want) {
					t.Errorf("uauvn: the D5 verdict %q does not name %q — only herdr's answer AND git's together reach an unclaim", r.Verdict, want)
				}
			}
		case 4:
			reg, ok := r.ReadingRegionOf(TurnOutcomeRegion)
			if !ok {
				t.Fatal("qcu4c: no message in the D6 record — the phrasing IS the evidence")
			}
			if !claudeAllotmentLimit(reg.Text) {
				t.Errorf("qcu4c: today's reader does not recognize the logged refusal %q", reg.Text)
			}
		}
		// Every record, whatever its decision: the three fields a census
		// groups by, and the version that took it.
		if r.Decision == "" || r.Consequence == "" || r.Rule == "" {
			t.Errorf("record %d is missing a census key: decision=%q consequence=%q rule=%q", n, r.Decision, r.Consequence, r.Rule)
		}
		if r.At.IsZero() {
			t.Errorf("record %d carries no timestamp, so no readings-per-day can be taken from it", n)
		}
		if r.Posse == "" {
			t.Errorf("record %d does not say which posse took it — a corpus outlives the rules it was read by", n)
		}
	}
}

// TestQAReadingEvidenceMarksARegionHerdrCutDown is the writer's half of the
// census's quality column (ranger-base-r5546).
//
// herdr hands over a PREVIEW and the region's real size beside it, and the
// two disagree exactly when the preview was cut — the 243-character cap in
// panework.go. ReadingEvidenceOf takes that disagreement down as
// `truncated`, and until this pin the field was written by one line and
// read by nothing: replacing that line with `Truncated: false` left every
// readings pin green (MEASURED 2026-10-04, M5 on ranger-base-qr0as).
//
// It is the WRITER's statement and not a thing a reader can derive later:
// the data ceiling rewrites the text after this runs, so by the time a
// record is on disk its region's length is no longer the length the flag
// was taken from.
func TestQAReadingEvidenceMarksARegionHerdrCutDown(t *testing.T) {
	t.Parallel()
	whole := "⏵⏵ auto mode on · 1 shell"
	preview := strings.Repeat("x", 243)
	var det AgentDetection
	det.State = "idle"
	det.EvaluatedRules = []EvaluatedRule{
		{ID: "live_prompt_box", Region: footerRegion},
		{ID: "live_prompt_box", Region: "whole_recent"},
	}
	det.EvaluatedRules[0].Evidence.RegionPreview = whole
	det.EvaluatedRules[0].Evidence.RegionBytes = len(whole)
	det.EvaluatedRules[1].Evidence.RegionPreview = preview
	det.EvaluatedRules[1].Evidence.RegionBytes = 4949

	ev := ReadingEvidenceOf(det)
	if len(ev.Regions) != 2 {
		t.Fatalf("ReadingEvidenceOf carried %d regions, want 2 — the evidence is every region herdr evaluated", len(ev.Regions))
	}
	for _, reg := range ev.Regions {
		switch reg.Name {
		case footerRegion:
			if reg.Truncated {
				t.Errorf("a region herdr handed over whole (%d bytes, %d of text) was recorded as cut down", reg.Bytes, len(reg.Text))
			}
		case "whole_recent":
			if !reg.Truncated {
				t.Errorf("a 243-character preview of a %d-byte region was recorded as complete — a census reading this record counts it as reproducible and it carries 5%% of the bytes", reg.Bytes)
			}
			if reg.Bytes != 4949 {
				t.Errorf("the region's real size was recorded as %d, want 4949 — the count is herdr's and is what the preview is short OF", reg.Bytes)
			}
		default:
			t.Errorf("unexpected region %q", reg.Name)
		}
	}
}

// TestQAReadingsCensusCountsByDayAndExportsTheCorpus runs the real
// instrument over a real log: the script ADR 0066 D1 names, the two things
// it has to produce (readings per day by decision and verdict, and a replay
// corpus of the bytes plus the verdict), and the one failure mode a log
// several processes append to actually has.
func TestQAReadingsCensusCountsByDayAndExportsTheCorpus(t *testing.T) {
	t.Parallel()
	script := filepath.Join("..", "..", "scripts", "readings-census.py")
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("%s: %v — ADR 0066 D1 names a census script and this pin is the only thing that runs it", script, err)
	}
	if _, err := exec.LookPath("python3"); err != nil {
		// Not a skip: the script IS python3, so a box without it cannot be
		// the box this is measured on.
		t.Fatalf("no python3 on PATH, and %s is python3: %v", script, err)
	}

	a, _, tree := rlTree(t)
	rec := func(d string, dec ReadingDecision, cons ReadingConsequence, region, text string) Reading {
		at, err := time.Parse(time.RFC3339, d+"T12:00:00Z")
		if err != nil {
			t.Fatal(err)
		}
		r := Reading{At: at, Decision: dec, Consequence: cons, Verdict: "v", Rule: "r"}
		if region != "" {
			r.Herdr.Regions = []ReadingRegion{{Name: region, Bytes: len(text), Text: text}}
		}
		return r
	}
	// THE TWO LOSSY CASES go on their own day so the per-day arms below
	// keep measuring what they were written to measure. Each carries
	// bytes, so neither is excluded by the clause the other one tests —
	// which is the whole point of having both (ranger-base-r5546): with
	// only one of them in the fixture, `replayable` answers correctly
	// with the other clause deleted.
	//
	// The truncated one is the shape herdr hands over all day: a region
	// whose `bytes` is the WHOLE region and whose text is the 243
	// characters that arrived (panework.go).
	cut := rec("2026-10-04", DecisionDialog, ConsequenceHandBack, "whole_recent", strings.Repeat("x", 243))
	cut.Herdr.Regions[0].Bytes = 4949
	cut.Herdr.Regions[0].Truncated = true
	for _, r := range []Reading{
		rec("2026-10-02", DecisionComposerHold, ConsequenceHold, footerRegion, "⏵⏵ auto mode on · 1 shell"),
		rec("2026-10-02", DecisionComposerHold, ConsequenceGhostRetired, composerANSIRegion, "\x1b[2mx\x1b[0m"),
		rec("2026-10-03", DecisionDialog, ConsequenceHandBack, "whole_recent", "proceed? → 1. Yes"),
		rec("2026-10-03", DecisionStallVerdict, ConsequenceHandBack, "", ""),
		rec("2026-10-03", DecisionUnknownScreen, ConsequenceRefusal, "osc_title", ""),
		cut,
	} {
		if err := a.AppendReading(tree.Path, r); err != nil {
			t.Fatalf("AppendReading: %v", err)
		}
	}
	// And the redacted one, marshalled from the same type but appended by
	// hand: AppendReading decides `Redacted` itself from the instance's
	// ceiling, and this App has none configured. The redactor is pinned
	// where it belongs, by TestQAReadingsLogRedactsCeilingContentBeforeWrite;
	// what this fixture needs is a record of that SHAPE for the census to
	// read.
	red := rec("2026-10-04", DecisionDialog, ConsequenceHandBack, "whole_recent", "proceed? → ["+RedactedMark+":token]")
	red.Posse = VersionString()
	red.Redacted = []string{"token"}
	appendRawReading(t, a, tree.Path, red)
	log, err := a.ReadingsLogPath(tree.Path)
	if err != nil {
		t.Fatal(err)
	}
	// THE TORN LINE, which is the one failure a log several posse processes
	// append to really has. A census that died on it would be an instrument
	// that stops working exactly when the fleet is busiest.
	f, err := os.OpenFile(log, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"at":"2026-10-03T12:00:00Z","decis`); err != nil {
		t.Fatal(err)
	}
	f.Close()

	out, err := exec.Command("python3", script, "--log", log, "--json").CombinedOutput()
	if err != nil {
		t.Fatalf("%s --json: %v\n%s", script, err, out)
	}
	var c struct {
		Readings    int                       `json:"readings"`
		PerDay      map[string]map[string]int `json:"per_day"`
		PerDecision map[string]map[string]int `json:"per_decision"`
		Verdicts    map[string]map[string]int `json:"verdicts"`
		Regions     map[string]int            `json:"regions"`
		Replayable  int                       `json:"replayable"`
		Truncated   int                       `json:"truncated"`
		Redacted    int                       `json:"redacted"`
	}
	if err := json.Unmarshal(out, &c); err != nil {
		t.Fatalf("census --json is not json: %v\n%s", err, out)
	}
	if c.Readings != 7 {
		t.Errorf("census counted %d readings, want 7 (the torn line must be skipped, not fatal and not counted)", c.Readings)
	}
	if got := c.PerDay["2026-10-02"]["total"]; got != 2 {
		t.Errorf("2026-10-02 counted %d, want 2 — readings PER DAY is the denominator ADR 0066 D1 exists for", got)
	}
	if got := c.PerDay["2026-10-03"]["hand-back"]; got != 2 {
		t.Errorf("2026-10-03 counted %d hand-backs, want 2 — the per-day table must break down by consequence", got)
	}
	if got := c.PerDecision["D4"]["total"]; got != 2 {
		t.Errorf("D4 counted %d, want 2 — the table must group by decision", got)
	}
	if got := c.PerDecision["D4"]["ghost-retirement"]; got != 1 {
		t.Errorf("D4 counted %d ghost retirements, want 1", got)
	}
	if len(c.Verdicts) == 0 {
		t.Error("the census reports no verdicts at all — ADR 0066 D1 asks for readings per day BY DECISION AND VERDICT")
	}
	if c.Regions[footerRegion] != 1 || c.Regions[composerANSIRegion] != 1 {
		t.Errorf("the census does not count which regions were read: %v", c.Regions)
	}
	// THE QUALITY COLUMN, and it has three clauses that must each be able
	// to fail on their own (ranger-base-r5546). Six of the seven carry
	// bytes — the D5 record carries none by design — and two of those six
	// carry bytes that are not the bytes that were read: one region herdr
	// cut down, one record the ceiling took a class out of. So four.
	if c.Replayable != 4 {
		t.Errorf("census says %d replayable cases, want 4 — a case reproduces a verdict byte for byte only if it carries region bytes, all of them, and the ones that were read", c.Replayable)
	}
	if c.Truncated != 1 {
		t.Errorf("census says %d truncated cases, want 1 — herdr previews a region at 243 characters, so a census that cannot see truncation over-counts its own quality column by construction", c.Truncated)
	}
	if c.Redacted != 1 {
		t.Errorf("census says %d redacted cases, want 1", c.Redacted)
	}

	// THE CORPUS. The bytes and the verdict, in both shapes: the jsonl a
	// reader scores itself against, and one file per region so a reader
	// that takes a file can be pointed at it.
	dir := t.TempDir()
	out, err = exec.Command("python3", script, "--log", log, "--export-corpus", dir).CombinedOutput()
	if err != nil {
		t.Fatalf("%s --export-corpus: %v\n%s", script, err, out)
	}
	body, err := os.ReadFile(filepath.Join(dir, "corpus.jsonl"))
	if err != nil {
		t.Fatalf("no corpus.jsonl: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) != 7 {
		t.Errorf("corpus has %d cases, want 7", len(lines))
	}
	var seen, cutCases int
	for _, ln := range lines {
		var one struct {
			ID        string   `json:"id"`
			Verdict   string   `json:"verdict"`
			Truncated []string `json:"truncated"`
			Redacted  []string `json:"redacted"`
			Regions   []struct {
				Region    string `json:"region"`
				Text      string `json:"text"`
				Truncated bool   `json:"truncated"`
			} `json:"regions"`
		}
		if err := json.Unmarshal([]byte(ln), &one); err != nil {
			t.Fatalf("corpus case is not json: %v\n%s", err, ln)
		}
		if one.ID == "" || one.Verdict == "" {
			t.Errorf("a corpus case carries no id or no verdict — the verdict is the LABEL, and a corpus without it evaluates nothing: %s", ln)
		}
		if len(one.Truncated) > 0 {
			cutCases++
		}
		for _, reg := range one.Regions {
			seen++
			// The case's quality fields are what an offline evaluation
			// reads to know whether a miss is the reader's fault or the
			// capture's, so a cut region has to be named at the CASE and
			// not only inside the region it happened to.
			if reg.Truncated && !slices.Contains(one.Truncated, reg.Region) {
				t.Errorf("corpus case %s carries a region herdr cut down (%s) and its `truncated` field does not name it: %s", one.ID, reg.Region, ln)
			}
			p := filepath.Join(dir, "regions", one.ID, reg.Region+".txt")
			got, err := os.ReadFile(p)
			if err != nil {
				t.Errorf("no region file at %s: %v — a reader that takes a file (herdr agent explain --file) cannot be pointed at this case", p, err)
				continue
			}
			if string(got) != reg.Text {
				t.Errorf("%s holds %q, the case says %q — the two shapes disagree about the same bytes", p, got, reg.Text)
			}
		}
	}
	if seen != 6 {
		t.Errorf("the corpus exported %d region files, want 6", seen)
	}
	if cutCases != 1 {
		t.Errorf("%d corpus cases name a truncated region, want 1 — a case whose bytes are a 243-character preview of a 4949-byte region must say so, or an evaluation scores a reader against a capture it never had", cutCases)
	}
}

// TestQAReadingsLogIsNotWrittenByADryRun. A pass that acted on nothing must
// not leave state a later pass counts — seatidle.go's rule, which
// settleopen.go already applies to the bead. A census over a log that
// counted dry-run readings would price decisions nobody made.
func TestQAReadingsLogIsNotWrittenByADryRun(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	repo := wtqaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
	session := SessionForBead("ranger", repo, "a-1")
	tree, err := b.App.EnsureSessionTree(repo, session, nil)
	if err != nil || tree == nil {
		t.Fatalf("EnsureSessionTree: %v", err)
	}
	// The meta is what LogReading resolves the tree through, so it is
	// written rather than a session created: the pin is about the writer's
	// --dry-run rule, not about a launch.
	if err := b.writeMeta(&HerdrMeta{Name: session, Agent: "ranger", Dir: tree.Path, Repo: repo, Branch: tree.Branch}); err != nil {
		t.Fatalf("writeMeta: %v", err)
	}
	rec := Reading{Decision: DecisionComposerHold, Consequence: ConsequenceHold, Verdict: "v", Rule: RuleComposerHold}

	d.DryRun = true
	d.logReading(session, rec)
	log, err := b.App.ReadingsLogPath(tree.Path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(log); err == nil {
		t.Error("a --dry-run pass wrote a reading — a pass that acted on nothing must not leave state a later pass counts")
	}

	// THE CONTROL, and it is what makes the arm above a measurement: the
	// same call on a live pass writes. Without it, a logReading that was
	// broken outright would pass the arm above.
	d.DryRun = false
	d.logReading(session, rec)
	rs, err := ReadReadings(log)
	if err != nil || len(rs) != 1 {
		t.Fatalf("a live pass logged %d readings (%v), want 1 — the dry-run arm above measures nothing if this does not write", len(rs), err)
	}
	if rs[0].Session != session {
		t.Errorf("the record names session %q, want %q — a census grouped by session reads this field", rs[0].Session, session)
	}
}
