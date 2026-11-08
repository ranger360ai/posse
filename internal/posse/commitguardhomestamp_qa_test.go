//go:build !posse_arm2 && !posse_arm3

package posse

// ADR 0081 D2 (ranger-base-b87ej, from ranger-base-n6dsk). The commit-guard
// body carries `# posse-home: <abs home>`, and a reader that finds bytes
// which are not its own render asks WHOSE they are before it calls them
// stale.
//
// SINCE ADR 0082 D3 (ranger-base-8gwu1) that path is spelled
// `canonicalPath(a.Home)` — the way the kernel names the directory, the same
// spelling check 3's instance-path literals take one line further down the
// same body. D2's "absolute" survives; the sentence that said absolute and
// NOT resolved, "because the operator acts on the spelling they configured",
// is withdrawn. So every expected stamp in this file is built with
// qaStampedHome / qaStampedBy (gates_helpers_test.go) and never from an
// App's `Home` field, which on darwin is the one spelling the stamp is
// guaranteed not to carry. The READERS are unchanged, and the two arms at
// the foot of the file are the split that keeps them apart: one home spelled
// two ways is now one wall at the RENDER, and samePath is what still makes a
// body an earlier binary stamped non-canonically this home's to re-render.
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
	"runtime"
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
	// canonicalPath, since ADR 0082 D3: the stamp is spelled the way the
	// kernel names the directory, and a temp root on darwin is reached
	// through /var -> /private/var, so the raw t.TempDir() string is the
	// one spelling the body is guaranteed NOT to carry. ADR 0082's
	// Consequences says this in as many words — a fixture that builds an
	// expected literal from a temp dir passes it through canonicalPath.
	want := canonicalPath(home)
	body := NewAppAt(home).CommitGuardHook(VisibilityPublic, OpsPatternSet{})
	if got := commitGuardHomeOf(body); got != want {
		t.Errorf("commitGuardHomeOf = %q, want the rendering home's canonical spelling %q", got, want)
	}
	// And it still names the same DIRECTORY the App was built on, which is
	// the claim every reader of the stamp makes. Asserted beside the string
	// so a canonicalPath that answered some other real path could not pass
	// the line above by agreeing with itself.
	if got := commitGuardHomeOf(body); !samePath(got, home) {
		t.Errorf("commitGuardHomeOf = %q, which is not the directory %q the App renders from", got, home)
	}
	if n := strings.Count(body, commitGuardHomeStampPrefix); n != 1 {
		t.Errorf("the body carries %d stamp line(s), want exactly 1 — the parser takes the first and a second is a line no reader would ever see", n)
	}

	// The stamp is a COMMENT, which is what makes it safe to carry a path
	// the renderer does not control. A body whose stamp line were not one
	// would hand its home to /bin/sh on every commit in the repo. Both
	// spellings are swept: the one the stamp carries, and the one the
	// caller handed in, which a future renderer could start emitting.
	for _, line := range strings.Split(body, "\n") {
		if (strings.Contains(line, want) || strings.Contains(line, home)) && !strings.HasPrefix(line, "#") {
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
//
// Every home in the TABLE is a path that does NOT exist, so each row
// measures canonicalPath's deterministic fallback (ADR 0082 D1's third
// bullet) and the syntax rules around it. The canonical FOLD — a home that
// exists and is reachable by more than one spelling — is the subject of
// TestQAOneHomeSpelledTwoWaysRendersOneCommitWall, because it needs a real
// directory and a real symlink to have one.
//
// The arm after the table is the newline guard over the RENDERED answer,
// which ADR 0082 D3 made necessary and no row can reach.
// MUTATION-CHECKED 2026-11-08: with that second guard removed the arm reds,
// and the line it prints is the defect in full —
// `# posse-home: <tmp>/home\nrm -rf ` written into a file git execs on
// every commit in the repo.
func TestQACommitGuardHomeStampIsAbsoluteOrAbsent(t *testing.T) {
	t.Parallel()
	abs, err := filepath.Abs("rel/home")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ what, home, want string }{
		{"an absolute home with nothing to fold is stamped as it is spelled", "/opt/posse-home", commitGuardHomeStampPrefix + "/opt/posse-home\n"},
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

	// And the guard over the RENDERED answer, which is the one the table
	// above cannot reach: since ADR 0082 D3 the value written is a string
	// the KERNEL chose, so a home whose own path is comment-safe can
	// resolve onto one that is not. The home here carries no newline and
	// the directory it names does, which is a file git execs on every
	// commit handing the tail of a path to /bin/sh.
	root := t.TempDir()
	unsafe := filepath.Join(root, "home\nrm -rf /")
	if err := os.MkdirAll(unsafe, 0o755); err != nil {
		t.Skipf("no newline in a filename here: %v", err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(unsafe, link); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	if strings.ContainsAny(link, "\n\r") {
		t.Fatalf("fixture premise: the home handed in already carries a newline (%q), so the argument guard answers and the rendered one is not measured", link)
	}
	if !strings.ContainsAny(canonicalPath(link), "\n\r") {
		t.Skipf("canonicalPath(%q) = %q carries no newline on this box, so there is nothing for the second guard to catch", link, canonicalPath(link))
	}
	if got := commitGuardHomeStamp(link); got != "" {
		t.Errorf("a home that RESOLVES onto a newline stamped %q, want nothing — that home is unnameable in a comment and therefore owns nothing (D3 case 4), which is the fail-closed answer with no shell injected", got)
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
	if !strings.Contains(p.CommitGuardDegraded, qaStampedBy(x.Home)) {
		t.Errorf("the line must name the stamping home by path (%s), which is the one fact a reader cannot get anywhere else:\n  %s", qaStampedBy(x.Home), p.CommitGuardDegraded)
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
	if got, want := commitGuardHomeOf(string(body)), canonicalPath(x.Home); got != want {
		t.Errorf("the installed slot is stamped %q, want home X's canonical spelling %q (ADR 0082 D3)", got, want)
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
	if !strings.Contains(out, "rendered by home") || !strings.Contains(out, qaStampedBy(x.Home)) {
		t.Errorf("the sweep must name the stamping home %s rather than report freshness:\n%s", qaStampedBy(x.Home), out)
	}
	if strings.Contains(out, "ours but stale") {
		t.Errorf("the sweep called another home's current render stale:\n%s", out)
	}
}

// ─── one home spelled two ways is one home: at the RENDER, then at the reader ─

// qaStampHomeSpellings is one directory and the spellings that reach it: the
// on-disk path, a path through a symlink to its parent, and — darwin on a
// case-folding volume — the same name in another case. Every spelling is
// asserted to be a different STRING for the same DIRECTORY, because without
// the first the pin measures nothing and without the second it is measuring
// a real foreign home.
//
// The home is a NAMED subdirectory and not the temp root itself: the case
// axis needs letters to change the case of, and a t.TempDir() basename is
// digits.
func qaStampHomeSpellings(t *testing.T) (home string, others []string) {
	t.Helper()
	root := t.TempDir()
	home = filepath.Join(root, "Home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root, filepath.Join(root, "link")); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	cand := []string{filepath.Join(root, "link", "Home")}
	// darwin only, and only where the volume folds case: this is the axis
	// EvalSymlinks cannot close and F_GETPATH can (ADR 0082 D1). On a
	// case-SENSITIVE volume `<root>/HOME` is a directory that does not
	// exist, and keeping it apart is correct rather than a miss.
	if cased := filepath.Join(root, "HOME"); runtime.GOOS == "darwin" && samePath(home, cased) {
		cand = append(cand, cased)
	} else {
		t.Logf("case spelling skipped on this box: %s", cased)
	}
	for _, o := range cand {
		if o == home {
			t.Fatalf("fixture premise: %q and the on-disk home are one string", o)
		}
		if !samePath(o, home) {
			t.Fatalf("fixture premise: %q and %q are not one directory, so a difference below would be a real foreign home", o, home)
		}
		others = append(others, o)
	}
	return home, others
}

// THE PIN ADR 0082 D3 IS FOR. Two Apps whose Home is one directory under two
// spellings render byte-identical commit-guard bodies, so the L3 probe's
// byte compare (ADR 0023) reads one home's wall as HELD however that home
// was spelled when it was written.
//
// Before D3 the stamp was `filepath.Abs` + `Clean` of the home as
// configured, so an RHQ_HOME typed through a symlink — or a `/tmp` spelling
// of a `/private/tmp` home, which is what every test rig on this box has —
// rendered a body differing in that one line from the body the next launch
// renders. Identity then failed, the verdict said "ours but stale", and
// `posse gates install-hooks` re-rendered a file that was already correct,
// on every launch into that repo, forever. That is the same loop
// ranger-base-q0np6 measured between two REAL homes, reached through a path
// spelling instead: ADR 0082's class, over the home rather than the repo.
//
// Byte equality of the whole body is the assertion, not a grep for the
// stamp line, for the reason canonicalwall_qa_test.go gives about the repo
// half: one equality holds the claim for every line the render grows later.
//
// MUTATION-CHECKED 2026-11-08 (darwin 25.6.0, APFS case-insensitive), the
// same two mutants the repo half is checked with, and they red DIFFERENT
// spellings — which is what says this pin measures D3 and D1's platform
// half separately rather than one equality twice:
//
//   - commitGuardHomeStamp reverted to filepath.Abs + Clean (D3 undone, the
//     pre-0082 stamp) reds BOTH spellings here and reds
//     TestQAAWallInstalledUnderAnotherSpellingOfThisHomeReadsAsHeld at the
//     installed slot's stamp. The READER arm below stays GREEN under it,
//     which is what says that one is pinning samePath and not the render.
//   - kernelPath returning an error on darwin (D1's platform half undone, so
//     canonicalPath degrades to resolvedPath — exactly ADR 0082's rejected
//     option (a)) leaves the SYMLINK spelling green and reds the CASE
//     spelling, here and in TestQACanonicalPathIsTheKernelsSpelling. That is
//     the ADR's reason for rejecting (a), measured rather than asserted.
func TestQAOneHomeSpelledTwoWaysRendersOneCommitWall(t *testing.T) {
	t.Parallel()
	home, others := qaStampHomeSpellings(t)
	want := canonicalPath(home)

	ref := NewAppAt(home).CommitGuardHook(VisibilityPublic, OpsPatternSet{})
	// The premise, or every equality below is two unstamped bodies agreeing
	// about nothing: the reference really does carry a stamp, and it carries
	// the CANONICAL spelling rather than the one handed in.
	if got := commitGuardHomeOf(ref); got == "" {
		t.Fatalf("fixture premise: the reference body carries no %q line", commitGuardHomeStampPrefix)
	} else if got != want {
		t.Fatalf("fixture premise: the reference is stamped %q, want canonicalPath's %q", got, want)
	}

	for _, o := range others {
		body := NewAppAt(o).CommitGuardHook(VisibilityPublic, OpsPatternSet{})
		if got := commitGuardHomeOf(body); got != want {
			t.Errorf("a home spelled %q stamps %q, want the kernel's spelling %q (ADR 0082 D3)", o, got, want)
		}
		if body != ref {
			t.Errorf("a home spelled %q rendered a different wall than %q:\n%s", o, home, qcwDiffLines(body, ref))
		}
		// And the renderer on its own, so a failure names the stamp rather
		// than "two bodies of N lines are unequal".
		if got, ref := commitGuardHomeStamp(o), commitGuardHomeStamp(home); got != ref {
			t.Errorf("commitGuardHomeStamp(%q) = %q, want %q", o, got, ref)
		}
	}
}

// And the end of the loop, through the probe that pays for it: a wall
// INSTALLED under one spelling of this home reads as HELD from another. This
// is the arm that would have caught ranger-base-k9x8n's defect on the home
// half, and the one the ADR's Consequences is written in terms of.
func TestQAAWallInstalledUnderAnotherSpellingOfThisHomeReadsAsHeld(t *testing.T) {
	t.Parallel()
	repo, _ := qaHookRepo(t)
	home, others := qaStampHomeSpellings(t)

	slot, _, _, err := NewAppAt(others[0]).InstallCommitGuardHook(repo)
	if err != nil {
		t.Fatalf("install from the %s spelling: %v", others[0], err)
	}
	if got, want := commitGuardHomeOf(qaReadFile(t, slot)), canonicalPath(home); got != want {
		t.Fatalf("the installed slot is stamped %q, want the canonical %q", got, want)
	}

	a := NewAppAt(home)
	p := a.probeL3Hooks(repo, false)
	if p.CommitGuardVerdict != l3Held || !p.CommitGuard {
		t.Errorf("one home spelled two ways read verdict=%d commitGuard=%v, want l3Held (%d) and true — the bytes are this home's own render and a reinstall would change nothing: %q",
			p.CommitGuardVerdict, p.CommitGuard, l3Held, p.CommitGuardDegraded)
	}
	// The write rule, which is the half that costs something: a wall held is
	// a wall this home may refresh.
	if write, stamp := a.sharedCommitWallWrite(repo, p); !write || stamp != "" {
		t.Errorf("ADR 0081 D3 refused this home its OWN wall over a spelling: write=%v stamp=%q", write, stamp)
	}
}

// THE READER'S samePath, which ADR 0082 D3 keeps and which now has exactly
// one source of a non-canonical stamp: a body an EARLIER BINARY wrote. D3
// says such a body is still ours — it re-renders once through the
// stale->reinstall path that already exists, and no migration case is added
// for it — so the arm plants the pre-0082 spelling the way the shell
// reader's sibling does (TestQAHookFreshnessDoesNotCryWolfOverASymlinked
// HomeSpelling, internal/treepins): one line of the CURRENT render changed,
// so the only thing identity can be failing over is the home.
//
// Until ranger-base-hgq2k the Go reader had no arm for this at all: MEASURED
// 2026-11-07 on fcb61cb6, `samePath(stamp, a.Home)` replaced by
// `stamp == a.Home` left all 28 L3 arms of this package and all 10 of
// cmd/posse's install-hooks arms GREEN. The Go side is where it costs more
// than a sentence, because ADR 0081 D3's write rule is asked of this verdict
// (sharedCommitWallWrite): a wall read as another home's over a spelling
// would stop this home refreshing its OWN wall, take the per-session
// redirect instead, and send the operator to settle an ownership question
// that is already settled — a finding about path spelling, printed as a
// finding about ownership. Both halves are asserted here: the verdict, and
// the write it decides.
func TestQAOneHomeSpelledTwoWaysIsOneHomeToTheCommitWallReader(t *testing.T) {
	t.Parallel()
	repo, hooks := qaHookRepo(t)
	home, others := qaStampHomeSpellings(t)
	old := others[0]

	a := NewAppAt(home)
	cur := a.CommitGuardHook(VisibilityPublic, a.OpsPatternSet(), testIdentity(t, repo)...)
	// Taken OUT OF the render and never recomputed here, for the reason the
	// shell sibling's hfHomeOf gives: this arm is about the READER, so what
	// the renderer currently spells its own home as is none of its business
	// — and a value computed here would make the arm red on a renderer
	// change it is not pinning.
	mine := commitGuardHomeOf(cur)
	if mine == "" {
		t.Fatalf("fixture premise: this home's render carries no %q line to rewrite", commitGuardHomeStampPrefix)
	}
	if mine == old {
		t.Fatalf("fixture premise: this home already renders the %q spelling, so the planted body would be its own render", old)
	}
	// The body an earlier binary left: current in every other line, stamped
	// with the spelling that binary was configured with.
	pre := strings.Replace(cur, commitGuardHomeStampPrefix+mine, commitGuardHomeStampPrefix+old, 1)
	if got := commitGuardHomeOf(pre); got != old {
		t.Fatalf("fixture premise: the planted body is stamped %q, want the pre-0082 spelling %q", got, old)
	}
	if err := WriteExecutable(filepath.Join(hooks, "prepare-commit-msg"), []byte(pre), 0o755); err != nil {
		t.Fatal(err)
	}

	p := a.probeL3Hooks(repo, false)
	if p.CommitGuardVerdict == l3Held {
		t.Fatalf("CONTROL: the planted body is byte-equal to this home's render, so nothing below is about the stamp at all")
	}
	if p.CommitGuardVerdict != l3Stale {
		t.Errorf("one home spelled two ways read verdict=%d, want l3Stale (%d): this is this home's own render gone stale and `posse gates install-hooks` is the remedy — line: %q",
			p.CommitGuardVerdict, l3Stale, p.CommitGuardDegraded)
	}
	if p.CommitGuardHome != "" {
		t.Errorf("CommitGuardHome = %q, want nothing: the probe names a home only when the stamp is ANOTHER home's", p.CommitGuardHome)
	}
	if strings.Contains(p.CommitGuardDegraded, "rendered by home") {
		t.Errorf("a finding about ownership over a path spelling sends the operator to decide a question that is already settled:\n  %s", p.CommitGuardDegraded)
	}
	if !strings.Contains(p.CommitGuardDegraded, "ours but stale") {
		t.Errorf("this home's own wall must keep the line that prescribes the re-render:\n  %s", p.CommitGuardDegraded)
	}

	// And the consequence, which is why this arm is worth more than wording:
	// D3's write rule must still let this home write the wall it already owns
	// the bytes of, so the one re-render ADR 0082 D3 promises can happen. A
	// string compare here answers write=false and names the home as foreign.
	if write, stamp := a.sharedCommitWallWrite(repo, p); !write || stamp != "" {
		t.Errorf("ADR 0081 D3 refused this home its OWN wall over a spelling: write=%v stamp=%q", write, stamp)
	}
}
