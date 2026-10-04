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
	// stores is refused, measured against a real repo made inside the state
	// dir rather than against a string. This is the arm that fails if the
	// bound check is deleted; arms 1-3 would all still pass.
	if err := os.MkdirAll(a.StateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	inState := filepath.Join(a.StateDir, "repo")
	if err := os.MkdirAll(inState, 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, inState, "init", "-q", "-b", "main", ".")
	if p, err := a.ReadingsLogPath(inState); err == nil {
		t.Errorf("ReadingsLogPath resolved %s, inside the instance's own state dir — that store outlives every session, so nothing in it is rotated with a tree", p)
	} else if !strings.Contains(err.Error(), "ADR 0066 D1") {
		t.Errorf("the refusal does not name the rule it is enforcing: %v", err)
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
	for _, r := range []Reading{
		rec("2026-10-02", DecisionComposerHold, ConsequenceHold, footerRegion, "⏵⏵ auto mode on · 1 shell"),
		rec("2026-10-02", DecisionComposerHold, ConsequenceGhostRetired, composerANSIRegion, "\x1b[2mx\x1b[0m"),
		rec("2026-10-03", DecisionDialog, ConsequenceHandBack, "whole_recent", "proceed? → 1. Yes"),
		rec("2026-10-03", DecisionStallVerdict, ConsequenceHandBack, "", ""),
		rec("2026-10-03", DecisionUnknownScreen, ConsequenceRefusal, "osc_title", ""),
	} {
		if err := a.AppendReading(tree.Path, r); err != nil {
			t.Fatalf("AppendReading: %v", err)
		}
	}
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
	}
	if err := json.Unmarshal(out, &c); err != nil {
		t.Fatalf("census --json is not json: %v\n%s", err, out)
	}
	if c.Readings != 5 {
		t.Errorf("census counted %d readings, want 5 (the torn line must be skipped, not fatal and not counted)", c.Readings)
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
	// Four of the five carry bytes; the D5 record carries none by design,
	// so "replayable" must be 4 and not 5.
	if c.Replayable != 4 {
		t.Errorf("census says %d replayable cases, want 4 — a record with no region bytes reproduces nothing and must not be counted as if it did", c.Replayable)
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
	if len(lines) != 5 {
		t.Errorf("corpus has %d cases, want 5", len(lines))
	}
	var seen int
	for _, ln := range lines {
		var one struct {
			ID      string `json:"id"`
			Verdict string `json:"verdict"`
			Regions []struct {
				Region string `json:"region"`
				Text   string `json:"text"`
			} `json:"regions"`
		}
		if err := json.Unmarshal([]byte(ln), &one); err != nil {
			t.Fatalf("corpus case is not json: %v\n%s", err, ln)
		}
		if one.ID == "" || one.Verdict == "" {
			t.Errorf("a corpus case carries no id or no verdict — the verdict is the LABEL, and a corpus without it evaluates nothing: %s", ln)
		}
		for _, reg := range one.Regions {
			seen++
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
	if seen != 4 {
		t.Errorf("the corpus exported %d region files, want 4", seen)
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
