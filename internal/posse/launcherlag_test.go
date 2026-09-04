package posse

import (
	"path/filepath"
	"strings"
	"testing"
)

// posseRig is a checkout that FindLauncher will accept as this binary's:
// a repo on `main` whose go.mod declares the module. Returns the repo and
// the short sha of its first commit — the "stamp" a binary built there
// would carry.
func posseRig(t *testing.T, module string) (repo, first string) {
	t.Helper()
	repo = t.TempDir()
	mustGit(t, repo, "init", "-q", "-b", "main", ".")
	mustGit(t, repo, "config", "user.email", "t@example.com")
	mustGit(t, repo, "config", "user.name", "t")
	write(t, filepath.Join(repo, "go.mod"), "module "+module+"\n\ngo 1.26.5\n")
	// The repo's own path in the seed, so two rigs made in the same second
	// are two HISTORIES. Without it they are byte-identical trees with the
	// same author, message and timestamp, git gives them the same sha, and
	// a test about "this checkout does not hold the stamp" is silently
	// testing a checkout that does. Measured while writing this file.
	write(t, filepath.Join(repo, "seed.txt"), repo+"\n")
	mustGit(t, repo, "add", "go.mod", "seed.txt")
	mustGit(t, repo, "commit", "-q", "-m", "seed")
	return repo, mustGit(t, repo, "rev-parse", "--short=7", "HEAD")
}

func landCommits(t *testing.T, repo string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		commitIn(t, repo, "fix.txt", strings.Repeat("x", i+1)+"\n", "a fix nobody is getting")
	}
}

// The reading the bead is about: a binary stamped with a commit that main
// has moved past says how far past.
func TestLauncherLagCountsMainPastTheStamp(t *testing.T) {
	t.Parallel()
	repo, stamp := posseRig(t, posseModule)
	landCommits(t, repo, 5)

	l := FindLauncher("0.4.0+"+stamp, []string{repo})
	if !l.Known() {
		t.Fatalf("not counted: %s", l.Why)
	}
	if l.Behind != 5 {
		t.Fatalf("behind = %d, want 5 (%s)", l.Behind, l.Line())
	}
	if l.Base != "main" || l.Rev != stamp {
		t.Fatalf("base %q rev %q, want main/%s", l.Base, l.Rev, stamp)
	}
	if got := l.BehindLine(); !strings.Contains(got, "5 commit(s) behind main") {
		t.Fatalf("pass line does not carry the number: %q", got)
	}
	// The line has to be re-runnable by hand or the operator cannot see
	// WHICH fixes: it names the repo, both ends of the range, and the
	// command that lists them.
	for _, want := range []string{"git -C ", "log --oneline " + stamp + "..main"} {
		if !strings.Contains(l.Line(), want) {
			t.Fatalf("status line missing %q: %s", want, l.Line())
		}
	}
}

// The pass says NOTHING when the launcher is current — a loop reports
// conditions, and "the launcher is fine" is not one. `posse status` still
// answers, because an operator asked.
func TestLauncherLagCurrentIsSilentInThePassAndSpokenInStatus(t *testing.T) {
	t.Parallel()
	repo, stamp := posseRig(t, posseModule)

	l := FindLauncher("0.4.0+"+stamp, []string{repo})
	if !l.Known() || l.Behind != 0 {
		t.Fatalf("behind = %d known=%v (%s)", l.Behind, l.Known(), l.Why)
	}
	if got := l.BehindLine(); got != "" {
		t.Fatalf("a current launcher printed into the pass: %q", got)
	}
	if got := l.Line(); !strings.Contains(got, "is the tip of main") {
		t.Fatalf("status line = %q", got)
	}
}

// Count re-reads the NUMBER without re-resolving the repo. This is the
// whole per-pass contract: the resolution cannot change under a running
// loop, the number changes every time somebody lands a commit, and the
// defect is that nothing was re-reading it.
func TestLauncherLagCountReReadsWithoutResolving(t *testing.T) {
	t.Parallel()
	repo, stamp := posseRig(t, posseModule)

	l := FindLauncher("0.4.0+"+stamp, []string{repo})
	if l.Behind != 0 {
		t.Fatalf("behind = %d, want 0", l.Behind)
	}
	landCommits(t, repo, 3)
	if again := l.Count(); again.Behind != 3 {
		t.Fatalf("after 3 commits behind = %d, want 3 (%s)", again.Behind, again.Why)
	}
	// And Count is a value method: the caller's own copy is untouched
	// unless it takes the result, which is what the pass loop relies on to
	// keep one lag per loop rather than a shared mutable one.
	if l.Behind != 0 {
		t.Fatalf("Count mutated the receiver: behind = %d", l.Behind)
	}
}

// The bead's "measured against the installed binary's own build stamp
// rather than anything a worktree can fake". A linked session worktree
// shares its refs with the checkout it hangs off, so a persona sitting on a
// branch with commits of its own must not move this number: the count is
// the CHECKOUT's branch, resolved through MainCheckout.
func TestLauncherLagCountsTheCheckoutNotTheWorktreeItWasHandedTo(t *testing.T) {
	t.Parallel()
	repo, stamp := posseRig(t, posseModule)
	landCommits(t, repo, 2)

	wt := filepath.Join(t.TempDir(), "seat")
	mustGit(t, repo, "worktree", "add", "-q", "-b", "posse/seat", wt, "main")
	// Nine commits on the seat's branch: if the reading followed the tree
	// it was handed, the number would be 9 and not 2.
	landCommits(t, wt, 9)

	l := FindLauncher("0.4.0+"+stamp, []string{wt})
	if !l.Known() {
		t.Fatalf("not counted: %s", l.Why)
	}
	if l.Base != "main" {
		t.Fatalf("counted against %q, want main — the worktree's branch was followed", l.Base)
	}
	if l.Behind != 2 {
		t.Fatalf("behind = %d, want 2 — the worktree's own commits were counted (%s)", l.Behind, l.Line())
	}
	if l.Repo != mustGit(t, repo, "rev-parse", "--show-toplevel") {
		t.Fatalf("repo = %q, want the main checkout %q", l.Repo, repo)
	}
}

// A checkout that is not this module is not this binary's repo, however
// plausible its history. Without this arm the reading would count a stamp
// against whatever repo happened to be listed first and print a confident
// wrong number, which is worse than no number at all.
func TestLauncherLagRefusesACheckoutOfAnotherModule(t *testing.T) {
	t.Parallel()
	other, stamp := posseRig(t, "github.com/example/not-posse")
	landCommits(t, other, 4)

	l := FindLauncher("0.4.0+"+stamp, []string{other})
	if l.Known() {
		t.Fatalf("counted %d against a foreign module: %s", l.Behind, l.Line())
	}
	if !strings.Contains(l.Why, posseModule) || !strings.Contains(l.Why, stamp) {
		t.Fatalf("abstention names neither the module nor the stamp: %s", l.Why)
	}
	// And it says WHERE it looked, or the operator cannot tell a
	// misconfigured `beads:` from a genuinely absent repo.
	if !strings.Contains(l.Why, AbbrevHome(other)) {
		t.Fatalf("abstention does not name where it looked: %s", l.Why)
	}
}

// The right repo is found even when it is not the first candidate — the
// STAMP picks it, not the caller's ordering.
func TestLauncherLagPicksThePosseCheckoutOutOfTheCandidates(t *testing.T) {
	t.Parallel()
	other, _ := posseRig(t, "github.com/example/not-posse")
	repo, stamp := posseRig(t, posseModule)
	landCommits(t, repo, 7)

	l := FindLauncher("0.4.0+"+stamp, []string{other, "", "/nonexistent/nowhere", repo})
	if !l.Known() {
		t.Fatalf("not counted: %s", l.Why)
	}
	if l.Behind != 7 {
		t.Fatalf("behind = %d, want 7 (%s)", l.Behind, l.Line())
	}
}

// A posse checkout that does not hold the stamp is not the one this binary
// came out of, and the scan must keep looking rather than stop at the first
// repo whose go.mod matches. A box carries more than one clone of posse —
// the checkout, an old one, a release tree — and the STAMP is what picks
// between them.
//
// This is the arm the module check and the presence check do NOT share:
// without the presence check both abstention tests below still pass (the
// count against a missing rev fails and sets Why anyway), and the only
// visible difference is that the scan stops at the wrong repo. Measured as
// a surviving mutant, 2026-09-04.
func TestLauncherLagSkipsAPosseCheckoutThatLacksTheStamp(t *testing.T) {
	t.Parallel()
	stale, _ := posseRig(t, posseModule)
	landCommits(t, stale, 2)
	repo, stamp := posseRig(t, posseModule)
	landCommits(t, repo, 11)

	l := FindLauncher("0.4.0+"+stamp, []string{stale, repo})
	if !l.Known() {
		t.Fatalf("not counted: %s", l.Why)
	}
	if l.Repo == stale {
		t.Fatalf("stopped at the checkout that does not hold %s", stamp)
	}
	if l.Behind != 11 {
		t.Fatalf("behind = %d, want 11 — counted against the wrong clone (%s)", l.Behind, l.Line())
	}
}

// A posse checkout that has never fetched the commit this binary was built
// from cannot say how far behind it is, and must not answer as though it
// could.
func TestLauncherLagAbstainsWhenNoCheckoutHoldsTheStamp(t *testing.T) {
	t.Parallel()
	repo, _ := posseRig(t, posseModule)
	landCommits(t, repo, 2)

	l := FindLauncher("0.4.0+deadbee", []string{repo})
	if l.Known() {
		t.Fatalf("counted %d against a commit the repo does not hold: %s", l.Behind, l.Line())
	}
	if l.BehindLine() != "" {
		t.Fatalf("an abstention printed into the pass: %q", l.BehindLine())
	}
	if !strings.Contains(l.Line(), "not counted") {
		t.Fatalf("status line hides the abstention: %s", l.Line())
	}
}

// "+dev" is the one shape that names no commit — a plain `go build` from a
// linked worktree, which carries no vcs stamp at all. It abstains before it
// looks at any repo, and it says so rather than reading as an all-clear.
func TestLauncherLagAbstainsOnADevBuild(t *testing.T) {
	t.Parallel()
	// A good checkout, present so the abstention cannot be the repo's fault.
	repo, _ := posseRig(t, posseModule)

	l := FindLauncher("0.4.0+dev", []string{repo})
	if l.Known() {
		t.Fatalf("counted a build that names no commit: %s", l.Line())
	}
	if l.Rev != "" {
		t.Fatalf("rev = %q, want empty", l.Rev)
	}
	if !strings.Contains(l.Why, "names no commit") {
		t.Fatalf("why = %q", l.Why)
	}
}

// The brew-keg shape (ranger-base-39jnl): an install of the TAG carries no
// "+sha", and versionString collapses it to the bare version. A stale keg
// ahead of ~/.local/bin on PATH is a real way this box has run three-day-old
// code, so the tag is counted rather than abstained on.
func TestLauncherLagCountsATagBuild(t *testing.T) {
	t.Parallel()
	repo, _ := posseRig(t, posseModule)
	mustGit(t, repo, "tag", "v0.4.0")
	landCommits(t, repo, 6)

	l := FindLauncher("0.4.0", []string{repo})
	if !l.Known() {
		t.Fatalf("not counted: %s", l.Why)
	}
	if l.Rev != "v0.4.0" || l.Behind != 6 {
		t.Fatalf("rev %q behind %d, want v0.4.0/6 (%s)", l.Rev, l.Behind, l.Line())
	}
}

// A dirty stamp is still a real commit, so it still counts — and the line
// says the count is a FLOOR, because whatever was uncommitted in that tree
// is in no commit on the base either.
func TestLauncherLagDirtyStampCountsAndSaysItIsAFloor(t *testing.T) {
	t.Parallel()
	repo, stamp := posseRig(t, posseModule)
	landCommits(t, repo, 3)

	l := FindLauncher("0.4.0+"+stamp+"-dirty", []string{repo})
	if !l.Known() || l.Behind != 3 {
		t.Fatalf("behind = %d known=%v (%s)", l.Behind, l.Known(), l.Why)
	}
	if !l.Dirty {
		t.Fatal("the -dirty suffix was not read")
	}
	if !strings.Contains(l.Line(), "floor") {
		t.Fatalf("line does not say the count is a floor: %s", l.Line())
	}
}

// The cadence. Doubling, and the first nonzero always speaks — a loop
// STARTED behind must not wait for the number to double before it says so.
func TestLagDrumbeatDoubles(t *testing.T) {
	t.Parallel()
	var d lagDrumbeat
	var said []int
	// One commit landing at a time, the way main actually moves.
	for n := 0; n <= 40; n++ {
		if d.say(n) {
			said = append(said, n)
		}
	}
	want := []int{1, 2, 4, 8, 16, 32}
	if len(said) != len(want) {
		t.Fatalf("said at %v, want %v", said, want)
	}
	for i := range want {
		if said[i] != want[i] {
			t.Fatalf("said at %v, want %v", said, want)
		}
	}
}

func TestLagDrumbeatSaysAStartingGapImmediately(t *testing.T) {
	t.Parallel()
	var d lagDrumbeat
	if !d.say(29) {
		t.Fatal("a loop that starts 29 behind said nothing on its first pass")
	}
	if d.say(30) {
		t.Fatal("said again one commit later — the cadence is doubling, not every pass")
	}
	if !d.say(58) {
		t.Fatal("did not say again at twice the number")
	}
}

// Zero never speaks, however many passes ask.
func TestLagDrumbeatIsSilentAtZero(t *testing.T) {
	t.Parallel()
	var d lagDrumbeat
	for i := 0; i < 100; i++ {
		if d.say(0) {
			t.Fatal("a current launcher printed into the pass")
		}
	}
	if !d.say(1) {
		t.Fatal("silent at zero left the drumbeat unable to speak at one")
	}
}
