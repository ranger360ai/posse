//go:build posse_arm2

package posse

// ADR 0088 D7/D8 at the two sites that read them: the verdict line names the
// launcher that ran the check, and a green→red window is not attributed to a
// landing when the launcher is not the one that judged the green tree
// (ranger-base-7or9p; landedlauncher.go's header has the incident and the
// measurement).
//
// WHAT BEHAVES DIFFERENTLY IF THESE PINS ARE DELETED (ADR 0006 §7). The filer
// would blame a landing for a wall that moved: it would assign a P1 to the
// persona of the newest landing in a window whose trees had nothing to do with
// the verdict, and the bead would assert in its own body that "the newest
// landing is simply the one whose tree first held them all". That is not a
// comment going stale — it is a seat dispatched against innocent work, which
// is what it cost on ranger-base-ufd7z: one P1 against eleven landings and a
// session spent establishing that no tree in the window could clear it.
//
// THE DRIFT ARMS GO THROUGH THE RUNNER, never through a call to `file` with a
// drift made up beside it, for landedfile_test.go's reason: what D8 keys on is
// a token WRITTEN by one run and READ by the next, so a test that handed the
// filer a comparison would pin the paragraph and nothing about the record it
// is read from. Each arm below reaches a recorded green with one launcher,
// lands the two halves, and judges with another.
//
// THE LAUNCHER IS THE ONE THING STUBBED, through the seam the production code
// carries for it (landedChecker.launcherFn), because the alternative is a test
// that reinstalls the binary it is running from. The real reading is pinned
// separately below, over this test binary.

import (
	"os"
	"strings"
	"testing"
	"time"
)

// ldlLauncher is a launcher that is not this test binary: a version and an
// mtime a test chooses, so two arms can differ by exactly one field.
func ldlLauncher(version string, mod time.Time) landedLauncher {
	return landedLauncher{Version: version, Path: "/opt/posse", Mod: mod, Size: 4096}
}

var (
	ldlFirst  = ldlLauncher("0.0.0+aaaaaaa", time.Unix(1_700_000_000, 0).UTC())
	ldlSecond = ldlLauncher("0.0.0+bbbbbbb", time.Unix(1_700_009_999, 0).UTC())
)

// ldlJudgeWith points a checker at one launcher for its next run.
func ldlJudgeWith(c *landedChecker, l landedLauncher) {
	c.launcherFn = func() landedLauncher { return l }
}

// D7: EVERY verdict line names the launcher that ran the check — the green,
// the red, and the ⚠ that says there is no verdict at all. The last one is in
// here for the same reason D6.2's wall pin keeps it: a cage that would not
// render is a question about the launcher, so a silence that does not name one
// is the reading a reader can do least with.
//
// BOTH RULES, SIX LINES, because "every verdict line" is six and not three:
// landedcheck.go's three and wallcheck.go's three (ADR 0097 D2's own verdict
// lines, which go through this same checker, this same note and this same
// launcher record). Until ranger-base-di4bs this table was built from
// landedCheckFixture alone, so all three WALL lines could lose
// c.launcherNote() with the suite green — MEASURED over the whole arm, not a
// filter. ADR 0097 D5's abstention line is the one a reader can do least with,
// which is this comment's own argument for keeping it in.
//
// The wall check's FOURTH branch is deliberately not here: landInterrupted is
// "one ◷ line and nothing else" under ADR 0088 D6.5, and it carries no note
// by design — a launcher stopping is not a judgement about any launcher.
func TestEveryVerdictLineNamesTheLauncherThatRanTheCheck(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, want string
		arm        func(*testing.T) (*landedChecker, *strings.Builder, string)
	}{
		{"landed green", "green at", func(t *testing.T) (*landedChecker, *strings.Builder, string) {
			c, _, repo, _, _ := landedCheckFixture(t, 0)
			return c, ldlOut(c), repo
		}},
		{"landed red", "is RED at", func(t *testing.T) (*landedChecker, *strings.Builder, string) {
			c, _, repo, _, _ := landedCheckFixture(t, 1)
			return c, ldlOut(c), repo
		}},
		{"landed no answer", "NOT judged", func(t *testing.T) (*landedChecker, *strings.Builder, string) {
			c, _, repo, _, _ := landedCheckFixture(t, 0)
			c.cage = refusedCage
			return c, ldlOut(c), repo
		}},
		// The wall rule's three. Each reaches its verdict the way
		// wallcheck_test.go's own arms do — through the runner, over a tree
		// whose content decides the answer — and leaves the FINAL judgement
		// to the common body below.
		{"wall green", "wall pins green at", func(t *testing.T) (*landedChecker, *strings.Builder, string) {
			c, _, repo, _ := wckFixture(t)
			return c, ldlOut(c), repo
		}},
		{"wall red", "wall pins are RED at", func(t *testing.T) (*landedChecker, *strings.Builder, string) {
			c, _, repo, _ := wckFixture(t)
			out := ldlOut(c)
			ldlJudgeWith(c, ldlFirst)
			wckGreen(t, c, repo)
			commitIn(t, repo, wckRedFile, "red\n", ldfNewerBead+": the half that reds the pins")
			return c, out, repo
		}},
		{"wall not measured", "wall pins NOT MEASURED at", func(t *testing.T) (*landedChecker, *strings.Builder, string) {
			c, _, repo, _ := wckFixture(t)
			out := ldlOut(c)
			ldlJudgeWith(c, ldlFirst)
			wckGreen(t, c, repo)
			commitIn(t, repo, wckQuietFile, "dark\n", ldfNewerBead+": a helper change unrelated to the wall")
			return c, out, repo
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c, out, repo := tc.arm(t)
			ldlJudgeWith(c, ldlFirst)
			c.Schedule(repo, "main")
			c.Wait()
			s := out.String()
			line := ""
			for _, l := range strings.Split(s, "\n") {
				if strings.Contains(l, tc.want) {
					line = l
					break
				}
			}
			if line == "" {
				t.Fatalf("the %s arm printed no verdict line (want one naming %q):\n%s", tc.name, tc.want, s)
			}
			// The whole reading and not a field of it: the version is the
			// code that rendered the cage and the mtime is the file the
			// socket-wall step execs, and a line carrying one of the two
			// cannot tell a reinstall under a running launcher from a
			// rebuild (landedLauncher.token).
			if !strings.Contains(line, ldlFirst.String()) {
				t.Errorf("the %s verdict line does not name the launcher that ran the check, so a reader cannot tell a tree red from a judge red (ADR 0088 D7) — want %q in:\n%s\nwhole output:\n%s",
					tc.name, ldlFirst.String(), line, s)
			}
		})
	}
}

// THE TOKEN'S FIELD SET, one field at a time. This is the shape ldlLauncher
// above already promises — "so two arms can differ by exactly one field" —
// and which nothing was built out of until ranger-base-di4bs: ldlFirst and
// ldlSecond differ in BOTH the version and the mtime, and the drift table in
// TestTheLauncherReadingNamesThisBinaryAndComparesWithTheRecord compares
// whole tokens only. So the field SET was asserted by nothing.
//
// FOUND BY MUTATION: deleting the mtime from token() left the suite green,
// and the mtime is the one field this rule's own reasoning turns on — "Version
// AND mtime because they fail differently ... a reinstall under a running
// launcher moves the second only" (landedlauncher.go's header), VersionString()
// being compiled into the running process and unable to move under it. With
// the mtime gone, a reinstall that left the size unchanged reads `same`, D5
// keeps its attribution, and the filer assigns a P1 to the persona of the
// newest landing: the exact ranger-base-ufd7z cost this rule exists to stop.
func TestOneFieldMovingUnderTheLauncherReadsAsMoved(t *testing.T) {
	t.Parallel()
	base := landedLauncher{Version: "0.5.3+aaaaaaa", Path: "/opt/posse", Mod: time.Unix(1_700_000_000, 0).UTC(), Size: 4096}
	if base.token() == "" {
		t.Fatalf("the base launcher names no token, so no arm below compares anything: %+v", base)
	}
	for _, tc := range []struct {
		field, why string
		move       func(landedLauncher) landedLauncher
	}{
		{"Version", "the RUNNING code, which renders the cage and the seatbelt profile",
			func(l landedLauncher) landedLauncher { l.Version = "0.5.3+bbbbbbb"; return l }},
		{"Mod", "the mtime of the file the socket-wall step execs, which a reinstall under a running launcher moves on its own",
			func(l landedLauncher) landedLauncher { l.Mod = l.Mod.Add(time.Second); return l }},
		{"Size", "what an install that preserved the mtime still moves",
			func(l landedLauncher) landedLauncher { l.Size++; return l }},
		{"Path", "the file the step execs at all — a launcher at another path is another launcher",
			func(l landedLauncher) landedLauncher { l.Path = "/usr/local/bin/posse"; return l }},
	} {
		t.Run(tc.field, func(t *testing.T) {
			t.Parallel()
			moved := tc.move(base)
			if moved.token() == base.token() {
				t.Fatalf("a launcher whose %s moved tokenizes identically, so the record cannot see it and D8 reads `same` over a wall that moved — %s is %s\n  base:  %s\n  moved: %s",
					tc.field, tc.field, tc.why, base.token(), moved.token())
			}
			if got := landedLauncherDriftOf(moved, base.token()); got != landedLauncherMoved {
				t.Errorf("%s moved and the drift against the record reads %d, want moved (%d)", tc.field, got, landedLauncherMoved)
			}
			// The other way round, because an install can go backwards — a
			// rollback is a launcher that moved too, and a comparison that
			// only notices upgrades withholds nothing on the way down.
			if got := landedLauncherDriftOf(base, moved.token()); got != landedLauncherMoved {
				t.Errorf("%s moved back and the drift reads %d, want moved (%d)", tc.field, got, landedLauncherMoved)
			}
			// And the arm is comparing something: a launcher that tokenized
			// to nothing would satisfy both lines above as `unknown`.
			if got := landedLauncherDriftOf(moved, moved.token()); got != landedLauncherSame {
				t.Errorf("the %s arm's launcher does not compare equal with itself (%d, want same %d) — it names no token, so this arm measured an abstention", tc.field, got, landedLauncherSame)
			}
		})
	}
}

// ldlOut swaps a checker's writer for one this file can read, so an arm above
// can take the fixture's checker and still hold its own output. The checker's
// own constructor contract is a writer it may write to from the run goroutine,
// and every read here happens after Wait.
func ldlOut(c *landedChecker) *strings.Builder {
	b := &strings.Builder{}
	c.out = b
	return b
}

// D8's three answers, end to end: a green recorded by one launcher, the two
// halves landed, and the red judged by a launcher that is the same one, a
// different one, or one the record cannot name.
func TestAWindowIsAttributedOnlyWhenTheLauncherIsTheOneThatJudgedTheGreenTree(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		judgeRed   landedLauncher
		forget     bool // drop the token, as a green judged before D8 left it
		assigned   bool
		wantInBead []string
		notInBead  []string
	}{
		{
			name:     "the same launcher",
			judgeRed: ldlFirst,
			assigned: true,
			// D5's sentence, unchanged: every input to this verdict did come
			// out of the window's trees, so the claim is the one it always was.
			wantInBead: []string{"Assigned to " + ldfNewerWho, "whose tree first held them all", ldlFirst.String()},
			notInBead:  []string{"NOT ASSIGNED", "could not be checked"},
		},
		{
			name:     "a launcher that moved",
			judgeRed: ldlSecond,
			assigned: false,
			wantInBead: []string{
				"NOT ASSIGNED",
				ldlSecond.String(), // what judged this tree
				ldlFirst.token(),   // what judged the green one
				"rendered by the LAUNCHER",
			},
			// The false sentence, and the point of the whole bead: it may not
			// be in a bead whose verdict the launcher could have moved.
			notInBead: []string{"whose tree first held them all", "Assigned to " + ldfNewerWho},
		},
		{
			name:     "a launcher the record cannot name",
			judgeRed: ldlFirst,
			forget:   true,
			assigned: true,
			// Attributed as before — withholding on every unknown would
			// withhold on every repo until the first green lands — but the
			// bead says the comparison was not made.
			wantInBead: []string{"Assigned to " + ldfNewerWho, "could not be checked", ldlFirst.String()},
			notInBead:  []string{"NOT ASSIGNED"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c, out, repo, _ := ldfFixture(t)
			ldlJudgeWith(c, ldlFirst)
			ldfGreen(t, c, repo)
			if got := landedLauncherRecorded(landedJudgeRule, repo); got != ldlFirst.token() {
				t.Fatalf("%s = %q after a green, want %q — D8's token is written beside the GREEN verdict, and without it no arm here measures anything", landedJudgeRule.LauncherKey, got, ldlFirst.token())
			}
			if tc.forget {
				if _, err := git(repo, "config", "--unset", landedJudgeRule.LauncherKey); err != nil {
					t.Fatalf("the token could not be dropped: %v", err)
				}
			}
			ldfBreak(t, repo)
			ldlJudgeWith(c, tc.judgeRed)

			ldfJudge(t, c, repo)

			is := ldfFiled(t, c, repo)
			for _, want := range tc.wantInBead {
				if !strings.Contains(is.Description, want) {
					t.Errorf("the bead does not name %q (ADR 0088 D7/D8):\n%s", want, is.Description)
				}
			}
			for _, not := range tc.notInBead {
				if strings.Contains(is.Description, not) {
					t.Errorf("the bead says %q over a launcher that %s — that is the sentence a seat is dispatched on:\n%s", not, tc.name, is.Description)
				}
			}
			argv := ldfArgv(t)
			switch {
			case tc.assigned && !strings.Contains(argv, ldfCreateFields(ldfNewerWho)):
				t.Errorf("no create carried %q — the window is attributable here and D5 assigns the newest landing:\n%s", ldfCreateFields(ldfNewerWho), argv)
			case !tc.assigned && strings.Contains(argv, " -a "):
				t.Errorf("the create assigned somebody although the launcher moved under the window (ADR 0088 D8) — a dispatched P1 against a landing that did not do it:\n%s", argv)
			case !tc.assigned && !strings.Contains(out.String(), "the launcher is not the one that judged the tree on record green"):
				// The pass's own log, not only the bead: an unassigned bead
				// otherwise reads as a window that named no persona, which is
				// the other way to be unassigned and a different finding.
				t.Errorf("the pass filed an unassigned bead without saying why:\n%s", out.String())
			}
		})
	}
}

// And the record is left ALONE by a red, which is what makes the comparison
// above readable from inside the red run: D8's token answers "who judged the
// tree on record green", so a red that overwrote it would compare this
// launcher with itself and read `same` forever.
func TestARedVerdictLeavesTheLauncherOnRecordAlone(t *testing.T) {
	t.Parallel()
	c, _, repo, _ := ldfFixture(t)
	ldlJudgeWith(c, ldlFirst)
	ldfGreen(t, c, repo)
	ldfBreak(t, repo)
	ldlJudgeWith(c, ldlSecond)

	ldfJudge(t, c, repo)

	if _, v := landedRecorded(landedJudgeRule, repo); v != landRed {
		t.Fatalf("the fixture never reached a red, so this arm measured nothing: %s", v)
	}
	if got := landedLauncherRecorded(landedJudgeRule, repo); got != ldlFirst.token() {
		t.Errorf("%s = %q after a red judged by another launcher, want the green tree's %q (ADR 0088 D8)", landedJudgeRule.LauncherKey, got, ldlFirst.token())
	}
}

// The real reading, over this test binary, because every arm above stubs it:
// a launcher that resolves names a version, a path and an mtime, and its token
// compares equal to itself and unequal to another's.
func TestTheLauncherReadingNamesThisBinaryAndComparesWithTheRecord(t *testing.T) {
	t.Parallel()
	l := landedLauncherNow()
	self, err := os.Executable()
	if err != nil {
		t.Skipf("this binary cannot name itself here (%v), which is the Why arm and not this one", err)
	}
	if l.Path != self || l.Mod.IsZero() || l.Size == 0 || l.Version == "" {
		t.Fatalf("landedLauncherNow read %+v, want this binary's path (%s), its mtime, its size and a version — every field is one a reinstall can move on its own", l, self)
	}
	if l.token() == "" {
		t.Fatalf("a launcher that resolved names no token, so D8 would read `unknown` on every repo: %+v", l)
	}
	for _, tc := range []struct {
		name string
		was  string
		want landedLauncherDrift
	}{
		{"its own token", l.token(), landedLauncherSame},
		{"the same token with trailing space", l.token() + "\n", landedLauncherSame},
		{"another launcher's", ldlSecond.token(), landedLauncherMoved},
		{"no record at all", "", landedLauncherUnknown},
		{"a blank record", "   ", landedLauncherUnknown},
	} {
		if got := landedLauncherDriftOf(l, tc.was); got != tc.want {
			t.Errorf("against %s the drift reads %d, want %d", tc.name, got, tc.want)
		}
	}
	// A launcher that cannot name itself is UNKNOWN against any record and
	// never a difference: "the wall may have moved" is not "the wall moved",
	// and only one of the two withholds an attribution.
	blind := landedLauncher{Version: l.Version, Why: "it does not resolve"}
	if blind.token() != "" {
		t.Errorf("a launcher with no path tokenized as %q, which would be compared as an identity", blind.token())
	}
	if got := landedLauncherDriftOf(blind, l.token()); got != landedLauncherUnknown {
		t.Errorf("a launcher that could not read its own binary reads %d against a real token, want unknown (%d)", got, landedLauncherUnknown)
	}
	if !strings.Contains(blind.String(), "could not be read") {
		t.Errorf("a blind launcher renders as %q, which does not say that it is one — this string is what a verdict line and a bead carry", blind.String())
	}
}
