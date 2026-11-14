//go:build posse_arm2

package posse

// LIVE pin for ranger-base-wplhv — ADR 0097 D1/D2/D3 run against the box's
// REAL venue, over this repo's own tree and this repo's own
// `scripts/posse-wall-check`.
//
//	RHQ_LIVE_WALLCHECK=$HOME/.cache/posse go test -tags posse_arm2 \
//	  ./internal/posse -run TestLiveTheWallCheckMeasuresInTheRealVenue \
//	  -v -timeout 30m
//
// The variable takes a PATH — a directory the run dir may live under, outside
// /tmp and /var/tmp — and wallLiveApp says why it has to.
//
// IT IS THE WHOLE PATH AND NOT A PIECE OF IT: one `Schedule` of this repo's
// branch, the production judge behind ADR 0088 D3's cage, and then — if that
// answered green — the production wall check behind ADR 0097 D1's venue,
// rendered by `boxLandedCage`'s flag and nothing else. What it measures is the
// question the stubbed arms cannot ask: do D1's three additions actually buy
// the nesting the 32 live pins need, on this kernel, through this bwrap, with
// the judge's masks over them.
//
// SO THIS IS A MEASUREMENT AND NOT A GATE, exactly as landedchecklive_test.go
// is: off by default, a prerequisite of nothing, and `make test` does not run
// it. Every reading it has produced is in docs/notes.d/ranger-base-wplhv.md,
// which is the one place they are said.
//
// IT IS IN ARM 2 ON PURPOSE, and that is not where its subject lives. The 32
// members are arm 1 files (ADR 0097 D4), and this test's NAME would be
// selected by D3's own `-run` filter — so in arm 1 it would be a test that a
// venue run starts inside the venue it is measuring. A tag the venue's
// untagged run does not build is the cheapest way for that to be impossible
// rather than merely unlikely.
//
// WHY IT BUILDS ITS OWN HOME UNDER THE CACHE DIR, which is the one thing this
// fixture does differently from its sibling's: the two private mounts stand
// down when the run dir is under /tmp (landedCheckTmp, and ADR 0097 D1's
// /var/tmp bind with it), and `wtApp`'s home is a `t.TempDir()`. Under that
// home the venue would render with the host's /var/tmp read-only, 18 of the
// 32 members would go dark, and the body would answer D3's exit 3 — a
// correct answer to a question nobody asked, and 100s to find out. Production
// puts its run dir under the state dir (`~/.posse/landed-check`), so this
// fixture does the same kind of thing one directory over.
//
// IT WRITES TO THE SHARED STORE for landedchecklive_test.go's reason — `git
// worktree add` registers under the repo's COMMON `.git/worktrees/` — and it
// puts both of ADR 0097 D5's records back on the way out.

import (
	"bytes"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLiveTheWallCheckMeasuresInTheRealVenue(t *testing.T) {
	if os.Getenv("RHQ_LIVE_WALLCHECK") == "" {
		t.Skip("set RHQ_LIVE_WALLCHECK=<a directory outside /tmp, e.g. $HOME/.cache/posse> to measure this tree's wall pins behind the box's real venue")
	}
	if m := WallMechanism(); m != WallBwrap {
		t.Skipf("NOTHING MEASURED: this host realizes its wall with %s, and ADR 0097 D1's venue is a bwrap (ADR 0096: no darwin change)", m)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	top, err := git(cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		t.Fatal(err)
	}
	branch := repoBranch(top)
	if branch == "" {
		t.Skipf("%s has a detached HEAD, so there is no branch for a landing to have moved", AbbrevHome(top))
	}
	// This suite's own fake-bd markers, shed for the run: the judge runs
	// `make`, whose doors run `go test ./internal/posse`, and the marker
	// would reach the inner test binary (landedchecklive_test.go's comment
	// has the measurement).
	for _, k := range []string{"RHQ_FAKE_HERDR", "RHQ_FAKE_GATED_BD_STORE", "RHQ_FAKE_POSSE"} {
		t.Setenv(k, "")
	}

	a := hermetic(t, wallLiveApp(t))
	out := &bytes.Buffer{}
	c := newLandedChecker(a, out)
	c.bd = testBd(t) // nothing is filed into the real store by a measurement

	for _, r := range landedRules() {
		wallLiveRestoreRecord(t, r.Key, top)
		wallLiveRestoreRecord(t, r.QuietKey, top)
	}

	start := time.Now()
	c.Schedule(top, branch)
	c.Wait()
	took := time.Since(start)

	s := out.String()
	t.Logf("MEASURED %s for the pair:\n%s", took.Round(time.Second), strings.TrimSpace(s))
	if s == "" {
		t.Fatalf("nothing was said about %s: either it ships neither check or its tree is already on record (%s / %s)", AbbrevHome(top), landedCheckKey, wallCheckKey)
	}
	if strings.Contains(s, "wall check not run") {
		// The judge is the gate on this step (ADR 0097 D2), so a judge that
		// did not answer green is not this pin's finding — and it is not a
		// measurement either.
		t.Skipf("NOTHING MEASURED: the judge did not answer green, so the wall check never ran:\n%s", s)
	}
	if strings.Contains(s, "wall pins NOT MEASURED") {
		// D3's exit 3, which is the one answer this pin exists to refuse:
		// the venue rendered and the pins did not measure in it. The tail
		// names which — a baseline that would not nest, zero members, or a
		// member that abstained.
		t.Fatalf("the real venue could not measure this tree's wall pins in %s — ADR 0097 D5 counts this as silence, and the tail says which of D3's three it was:\n%s", took.Round(time.Second), s)
	}
	tree, v := landedRecorded(wallCheckRule, top)
	if v != landGreen {
		t.Fatalf("%s reads %q %s after a live venue run — a RED here is a finding about this tree's pins and not about the venue, and the verdict line above has the members it names:\n%s", wallCheckKey, tree, v, s)
	}
	// The count is the point of D3's green line: a green exit over zero
	// measured pins is the hole the third exit closes, and ADR 0097 counted
	// 32 members on 2026-11-14.
	if !strings.Contains(s, "wall check: green,") {
		t.Errorf("the verdict carries no `wall check: green, <n> pins measured` line, so this run proves a green exit and not a measurement:\n%s", s)
	}
	t.Logf("MEASURED the venue's own argv and wall are on the verdict line above (ADR 0097 D2); the pair took %s, of which the wall check is the part after the judge's ✓",
		landedCheckWall(took))
}

// wallLiveApp is an App whose home — and so whose landed-check run dir — is
// NOT under /tmp or /var/tmp. See the file header: a run dir under either
// stands ADR 0097 D1's private mounts down, and the venue then measures 16 of
// 32 pins and answers D3's exit 3.
//
// AND IT RUNS UNDER THE BOX'S OWN HOME, which is the other half of the same
// problem and was MEASURED the hard way (docs/notes.d/ranger-base-wplhv.md,
// reading 1). This package's TestMain moves HOME to a temp dir under /tmp for
// every test in the binary (ranger-base-6vvgs' reason). Two things then go
// wrong at once: the private mounts stand down because the HOME is under
// /tmp, and — with the mounts forced on — two members red because `codex` on
// this box is a mise command-wrapper that resolves its installs under HOME,
// finds none under a fixture home, and exits 1 with no network to install
// from. Neither is a fact about the tree. The launcher's own HOME is the
// operator's, so the fixture takes the home out of the PASSWD database, which
// is the one source `$HOME` cannot have moved.
//
// THE ROOT COMES FROM THE GATE VARIABLE, which is why that variable takes a
// PATH: a caller who does not want a run dir under their own cache dir names
// another, and nothing here writes beside the repo, where a stray directory
// is something `posse worktrees` reports.
func wallLiveApp(t *testing.T) *App {
	t.Helper()
	home := wallLivePasswdHome(t)
	// Process-wide for the run, which is what the env gate makes safe: this
	// test is run alone and nothing else in the binary is looking at HOME.
	t.Setenv("HOME", home)
	root := os.Getenv("RHQ_LIVE_WALLCHECK")
	if !filepath.IsAbs(root) {
		root = filepath.Join(home, ".cache", "posse")
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Skipf("NOTHING MEASURED: %s cannot be made (%v) — pass RHQ_LIVE_WALLCHECK=<an absolute path outside /tmp> instead", root, err)
		}
	}
	for _, bad := range []string{bwrapTmpMount, wallVenueVarTmpMount} {
		if underDir(bad, root) {
			t.Skipf("NOTHING MEASURED: the run root %s is under %s, so ADR 0097 D1's private mounts would stand down and the pins would go dark for a reason that is this fixture's. Pass RHQ_LIVE_WALLCHECK=<an absolute path outside %s> (this binary moves HOME under /tmp for every test, so the default cache dir is there too).", root, bad, bad)
		}
	}
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		t.Skipf("NOTHING MEASURED: the run root %s is not a directory (%v)", root, err)
	}
	// THE ROOT IS THE STATE DIR, WITH NO PER-TEST DIRECTORY INSIDE IT, and
	// the reason is a byte budget that was MEASURED as two reds
	// (docs/notes.d/ranger-base-wplhv.md, readings 2 and 3). Two members bind
	// a unix socket under the run's TMPDIR and `sun_path` is 108 bytes; the
	// longest such path this tree builds is
	//
	//	<state>/landed-check/run-<10 digits>/tmp/posse-bwh<10 digits>/gnupg/S.gpg-agent.ssh
	//
	// which is 74 bytes after the state dir. Production's state dir is
	// `~/.posse` (17 here) and leaves 17 bytes of headroom; a per-test
	// `wv-<10 digits>` under `~/.cache/posse` spent 35 and both members
	// answered `bind: invalid argument` — a red that is the fixture's path
	// and not the tree's wall. Uniqueness is the RUN dir's, which posse
	// already makes with MkdirTemp, so there is nothing for a second level to
	// buy.
	if over := len(root) + wallLiveSocketBudget - wallLiveSunPath; over > 0 {
		t.Skipf("NOTHING MEASURED: the run root %s is %d byte(s) too long — the longest unix socket this tree binds under a run's TMPDIR would be %d bytes and sun_path is %d, so two members would answer `bind: invalid argument` for a reason that is this path's. Pass RHQ_LIVE_WALLCHECK=<a shorter absolute path outside /tmp>.",
			root, over, len(root)+wallLiveSocketBudget, wallLiveSunPath)
	}
	t.Cleanup(func() {
		// Non-recursive on purpose: the runner removes its own run dir, and
		// what is left is one empty directory. A RemoveAll here would be a
		// test deleting a path an operator may have given it.
		_ = os.Remove(filepath.Join(root, "landed-check"))
	})
	return &App{
		Home: root, ConfigPath: filepath.Join(root, "config.yaml"),
		RecipesDir: filepath.Join(root, "recipes"), EnvsDir: filepath.Join(root, "envs"),
		StateDir: root, AgentsDir: filepath.Join(root, "agents"),
	}
}

// The two numbers the budget above is made of: AF_UNIX's `sun_path`, and the
// longest socket path this tree builds under a run's TMPDIR, counted from the
// state dir.
const (
	wallLiveSunPath      = 108
	wallLiveSocketBudget = len("/landed-check/run-0123456789/tmp/posse-bwh0123456789/gnupg/S.gpg-agent.ssh")
)

// wallLivePasswdHome is this uid's home as the passwd database has it —
// `credential.go`'s own fallback, used here as the primary for the reason
// wallLiveApp's comment gives: `$HOME` in this binary is a temp directory
// TestMain made, and what the launcher hands the venue is the operator's.
func wallLivePasswdHome(t *testing.T) string {
	t.Helper()
	u, err := user.Current()
	if err != nil || u.HomeDir == "" {
		t.Skipf("NOTHING MEASURED: the passwd database names no home for this uid (%v), and $HOME in this binary is a fixture's", err)
	}
	for _, bad := range []string{bwrapTmpMount, wallVenueVarTmpMount} {
		if underDir(bad, u.HomeDir) {
			t.Skipf("NOTHING MEASURED: this uid's home %s is under %s, so ADR 0097 D1's private mounts would stand down", u.HomeDir, bad)
		}
	}
	return u.HomeDir
}

// wallLiveRestoreRecord reads one of the real repo's records and puts it back
// on the way out: a live run that left `posse.wallCheck` naming this tree
// would hand the next landing a cached verdict this test measured rather than
// the launcher did.
func wallLiveRestoreRecord(t *testing.T, key, repo string) {
	t.Helper()
	had, err := git(repo, "config", "--get", key)
	t.Cleanup(func() {
		if err != nil {
			_, _ = git(repo, "config", "--unset", key)
			return
		}
		_, _ = git(repo, "config", key, had)
	})
}
