//go:build !posse_arm2 && !posse_arm3

package posse

// ADR 0081 D2 (ranger-base-b87ej, from ranger-base-n6dsk). The commit-guard
// body carries `# posse-home: <abs home>`, and a reader that finds bytes
// which are not its own render asks WHOSE they are before it calls them
// stale.
//
// WHY THE DISTINCTION IS WORTH A VERDICT. Identity is byte-for-byte
// (ADR 0023), and the commit guard's bytes are a function of the rendering
// HOME: its ops patterns, its guard-value literals, its crew names, its
// `beads_visibility:`. Any home on the box can write any repo's shared
// `.git/hooks/prepare-commit-msg` — a session launch does it as a matter of
// course (herdrback planLaunch) — so two homes with different configs make
// one file that each of them reads as "ours but stale" and each of them
// re-stamps, forever. MEASURED 2026-11-06 on ranger-base-q0np6: same binary,
// same repo, only RHQ_HOME changed, and the renders were 1897 lines against
// 1137. "Stale" prescribes `posse gates install-hooks`, which is the one
// thing that keeps that loop running; "rendered by home X" prescribes
// deciding who owns the wall, which is the only thing that ends it.
//
// The pins below are the ADR's DONE WHEN, in its own terms: a body rendered
// under home X and read from home Y reports X by path, and a same-home
// render still compares equal. The second half is the control, and it is
// asserted FIRST in every arm — without it, an arm that reported X would
// pass just as well over a renderer that had stopped agreeing with itself.
//
// MUTATION-CHECKED 2026-11-07: with commitGuardHomeVerdict's refinement
// removed (the stamp read and discarded), the two arms below red at
// verdict=l3Stale, and the line the mutant prints is the defect in its own
// words — "ours but stale — run `posse gates install-hooks`; no named guard
// is missing from the installed body … so the drift is elsewhere", over a
// body whose only difference from the reader's render IS the home. The
// unstamped arm stays green under the same mutant, which is what says it is
// pinning the behaviour D3 case 4 keeps rather than the behaviour D2 adds.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// qaStampHomes is the two scratch homes the arms below compare: identical in
// every input to the render EXCEPT their path, so the only byte that can
// differ between their two bodies is the stamp. Two homes whose configs also
// differed would still reach l3OtherHome, and would prove less: the verdict
// would be reachable from the visibility stamp or an ops pattern, and the
// pin could not say which line carried it.
func qaStampHomes(t *testing.T) (x, y *App) {
	t.Helper()
	x, y = NewAppAt(t.TempDir()), NewAppAt(t.TempDir())
	if samePath(x.Home, y.Home) {
		t.Fatalf("fixture premise: the two scratch homes are one directory (%s)", x.Home)
	}
	return x, y
}

func TestQACommitGuardBodyNamesTheHomeThatRenderedIt(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	body := NewAppAt(home).CommitGuardHook(VisibilityPublic, OpsPatternSet{})
	if got := commitGuardHomeOf(body); got != home {
		t.Errorf("commitGuardHomeOf = %q, want the rendering home %q", got, home)
	}
	if n := strings.Count(body, commitGuardHomeStampPrefix); n != 1 {
		t.Errorf("the body carries %d stamp line(s), want exactly 1 — the parser takes the first and a second is a line no reader would ever see", n)
	}

	// The stamp is a COMMENT, which is what makes it safe to carry a path
	// the renderer does not control. A body whose stamp line were not one
	// would hand its home to /bin/sh on every commit in the repo.
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, home) && !strings.HasPrefix(line, "#") {
			t.Errorf("the home reached a line /bin/sh reads as a command:\n  %s", line)
		}
	}

	// An App with no Home renders no stamp at all, and that is ADR 0081 D3
	// case 4 — the one-shot migration body — rather than a home named "".
	if got := qaNoHome.CommitGuardHook(VisibilityPublic, OpsPatternSet{}); strings.Contains(got, commitGuardHomeStampPrefix) {
		t.Errorf("an App with no Home must render no stamp line; a `%s` with nothing after it parses back as unstamped and would say the bytes and their meaning disagree", commitGuardHomeStampPrefix)
	}
	if got := commitGuardHomeOf(qaNoHome.CommitGuardHook(VisibilityPublic, OpsPatternSet{})); got != "" {
		t.Errorf("commitGuardHomeOf of an unstamped body = %q, want \"\"", got)
	}
}

// The stamp is ABSOLUTE (D2's own word), and unnameable homes stamp nothing.
func TestQACommitGuardHomeStampIsAbsoluteOrAbsent(t *testing.T) {
	t.Parallel()
	abs, err := filepath.Abs("rel/home")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ what, home, want string }{
		{"an absolute home is stamped as it is spelled", "/opt/posse-home", commitGuardHomeStampPrefix + "/opt/posse-home\n"},
		{"an uncleaned home is cleaned", "/opt/posse-home/../posse-home/./x", commitGuardHomeStampPrefix + "/opt/posse-home/x\n"},
		{"a relative home is made absolute", "rel/home", commitGuardHomeStampPrefix + abs + "\n"},
		{"no home, no stamp", "", ""},
		{"a whitespace home is no home", "  \t ", ""},
		// A newline in the value ends the comment and makes the rest of
		// the path a command in a file git execs on every commit. That
		// home stamps nothing, so it owns nothing — fail closed, with no
		// shell injected.
		{"a newline in the path stamps nothing", "/opt/h\nrm -rf /", ""},
		{"a carriage return too", "/opt/h\rrm -rf /", ""},
	} {
		if got := commitGuardHomeStamp(c.home); got != c.want {
			t.Errorf("%s: commitGuardHomeStamp(%q) = %q, want %q", c.what, c.home, got, c.want)
		}
	}
}

// THE DONE WHEN. A wall rendered under home X and read from home Y reports X
// by path and is not called stale; the same wall read from X compares equal.
func TestQAAWallRenderedByAnotherHomeIsNotReportedAsStale(t *testing.T) {
	t.Parallel()
	repo, _ := qaHookRepo(t)
	x, y := qaStampHomes(t)

	slot, _, _, err := x.InstallCommitGuardHook(repo)
	if err != nil {
		t.Fatalf("install from home X: %v", err)
	}

	// The control, first: X's own render of X's own install must still be
	// byte-equal, or every arm below is about a renderer that disagrees
	// with itself and the stamp proves nothing.
	if p := x.probeL3Hooks(repo, false); p.CommitGuardVerdict != l3Held || !p.CommitGuard {
		t.Fatalf("CONTROL: the rendering home must read its own wall as held, got verdict=%d commitGuard=%v (%q)",
			p.CommitGuardVerdict, p.CommitGuard, p.CommitGuardDegraded)
	}

	p := y.probeL3Hooks(repo, false)
	if p.CommitGuardVerdict != l3OtherHome {
		t.Fatalf("a foreign home's wall read verdict=%d, want l3OtherHome (%d) — line: %q",
			p.CommitGuardVerdict, l3OtherHome, p.CommitGuardDegraded)
	}
	if p.CommitGuard {
		t.Errorf("another home's render must not COUNT as this home's wall: the guards in it are that home's, and the layer this launch claims is not realized")
	}
	if !strings.Contains(p.CommitGuardDegraded, AbbrevHome(x.Home)) {
		t.Errorf("the line must name the stamping home by path (%s), which is the one fact a reader cannot get anywhere else:\n  %s", AbbrevHome(x.Home), p.CommitGuardDegraded)
	}
	if strings.Contains(p.CommitGuardDegraded, "ours but stale") {
		t.Errorf("a foreign home's current render is not ours gone stale — that line prescribes the reinstall that keeps two homes re-stamping one file:\n  %s", p.CommitGuardDegraded)
	}
	if !strings.Contains(p.CommitGuardDegraded, "ADR 0081") {
		t.Errorf("the line must cite the decision it is a verdict of:\n  %s", p.CommitGuardDegraded)
	}

	// The verdict still reports as re-stamped, and D3 (ranger-base-lh9nj)
	// did not change that: what it states is true of the write WHEN the
	// write happens, and an owning home still re-stamps a foreign-stamped
	// wall (D6). The narrowing is at the call site, which asks
	// sharedCommitWallWrite — see commitwallowner_qa_test.go. A predicate
	// narrowed HERE instead would silence the owner's own re-stamp, which
	// is the one case herdrback's warning was written for.
	if !p.CommitGuardVerdict.reStamped() {
		t.Errorf("l3OtherHome must report as re-stamped: it is what installHook does to a body carrying our marker, and the owner's launch is where that still happens")
	}

	// The parser's own answer about the file on disk, so an arm above
	// cannot pass over a verdict reached from some other difference.
	body, err := os.ReadFile(slot)
	if err != nil {
		t.Fatal(err)
	}
	if got := commitGuardHomeOf(string(body)); got != x.Home {
		t.Errorf("the installed slot is stamped %q, want home X %q", got, x.Home)
	}
}

// ADR 0081 D3 case 4, which is every body on the box the day this lands: no
// stamp is NOT another home, it is the migration case a launch re-stamps.
func TestQAAnUnstampedWallIsStillStale(t *testing.T) {
	t.Parallel()
	repo, hooks := qaHookRepo(t)
	y := NewAppAt(t.TempDir())
	id := testIdentity(t, repo)

	// y's own render but for the stamp line: the sharpest unstamped body
	// there is, and the one an earlier binary left.
	pre := qaNoHome.CommitGuardHook(VisibilityPublic, y.OpsPatternSet(), id...)
	if commitGuardHomeOf(pre) != "" {
		t.Fatal("fixture premise: the planted body must be unstamped")
	}
	if err := WriteExecutable(filepath.Join(hooks, "prepare-commit-msg"), []byte(pre), 0o755); err != nil {
		t.Fatal(err)
	}
	p := y.probeL3Hooks(repo, false)
	if p.CommitGuardVerdict != l3Stale {
		t.Fatalf("an unstamped body read verdict=%d, want l3Stale (%d) — a body nobody stamped is this home's to re-stamp, not another home's to leave alone: %q",
			p.CommitGuardVerdict, l3Stale, p.CommitGuardDegraded)
	}
	if !strings.Contains(p.CommitGuardDegraded, "ours but stale") {
		t.Errorf("the migration body must keep the line that prescribes the re-render:\n  %s", p.CommitGuardDegraded)
	}
}

// The sweep is the second reader D2 names (ADR 0029 G12 reads it, and
// `posse promote` and the watch preamble print it). It keeps the probe's
// line byte for byte, so what this pin holds is that the line reaching an
// operator from a REPO THEY DECLARED says whose wall it is.
func TestQAHookWallSweepNamesAForeignStampedWall(t *testing.T) {
	a, dirs := hwsFixture(t, map[string]string{"priv": VisibilityPrivate}, "priv")
	repo := dirs["priv"]

	// A second home that declares the same repo the same way: every input
	// to the render but the home is equal, so the stamp is the only line
	// that can differ.
	x := NewAppAt(t.TempDir())
	if err := os.WriteFile(x.ConfigPath, []byte("beads_visibility:\n  "+repo+": "+VisibilityPrivate+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if v, _ := x.BeadsVisibility(repo); v != VisibilityPrivate {
		t.Fatalf("fixture: home X stamps %q, so a difference below need not be the home", v)
	}

	if out, found := hwsReport(t, a, "pin"); found {
		t.Fatalf("CONTROL: the sweep's own install must read clean before X writes over it:\n%s", out)
	}
	if _, _, _, err := x.InstallCommitGuardHook(repo); err != nil {
		t.Fatal(err)
	}
	out, found := hwsReport(t, a, "pin")
	if !found {
		t.Fatalf("a wall another home rewrote passed the sweep:\n%s", out)
	}
	if !strings.Contains(out, "rendered by home") || !strings.Contains(out, AbbrevHome(x.Home)) {
		t.Errorf("the sweep must name the stamping home %s rather than report freshness:\n%s", AbbrevHome(x.Home), out)
	}
	if strings.Contains(out, "ours but stale") {
		t.Errorf("the sweep called another home's current render stale:\n%s", out)
	}
}
