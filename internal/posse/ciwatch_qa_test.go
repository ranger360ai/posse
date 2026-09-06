//go:build posse_arm3

package posse

// ci-watch driven through the REAL dispatch pass rather than through
// App.CIWatch in isolation (ranger-base-x9e34, ciwatch.go), plus the one
// place it reaches into another mechanism: verify-after's exemption for its
// own close.
//
// The pass-level pins exist because the call SITE is the half a unit test
// cannot see. Delete the block in dispatch.go and every test in
// ciwatch_test.go still passes — the mechanism works perfectly and is never
// invoked, which is the same failure the bead is about: a thing that is
// correct and unread.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The pass files it. A red gate reaches the queue without anyone typing
// `gh run list`, which is the bead's whole DONE WHEN.
func TestDispatchPassFilesTheCiRedBead(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	a := b.App
	repo := cwRepo(t, a)
	os.WriteFile(filepath.Join(repo, "fake-ready.json"), []byte("[]"), 0o644)
	a.CIRead = func(CIQuery) CIState { return redState(191) }

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if out := dispatcherOut(d); !strings.Contains(out, "ci red ·") {
		t.Errorf("the pass did not run ci-watch:\n%s", out)
	}
	if calls := bdCalls(t, fake); !strings.Contains(calls, "create ci is red on main") {
		t.Errorf("no ci-red bead filed by the pass:\n%s", calls)
	}
}

// And a second pass over the same standing red files nothing and says
// nothing — the invariant, asserted where it actually has to hold.
func TestDispatchPassFilesOneCiRedBeadNotOnePerPass(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	repo := cwRepo(t, a)
	os.WriteFile(filepath.Join(repo, "fake-ready.json"), []byte("[]"), 0o644)
	a.CIRead = func(CIQuery) CIState { return redState(1) }

	for i := 0; i < 3; i++ {
		d := newTestDispatcher(t, b)
		if _, err := d.Run("", "", 0); err != nil {
			t.Fatal(err)
		}
	}
	if n := strings.Count(bdCalls(t, fake), "create ci is red on main"); n != 1 {
		t.Errorf("%d ci-red beads across three passes, want 1", n)
	}
}

// --dry-run shows routing without acting, and filing a bead is acting.
func TestDispatchDryRunFilesNoCiRedBead(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.DryRun = true
	a := b.App
	repo := cwRepo(t, a)
	os.WriteFile(filepath.Join(repo, "fake-ready.json"), []byte("[]"), 0o644)
	read := 0
	a.CIRead = func(CIQuery) CIState { read++; return redState(1) }

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	if calls := bdCalls(t, fake); strings.Contains(calls, "create ci is red on main") {
		t.Errorf("--dry-run filed a ci-red bead:\n%s", calls)
	}
	if read != 0 {
		t.Errorf("--dry-run took %d readings", read)
	}
}

// ─── the verify-after exemption ──────────────────────────────────────────────

// cwCommit makes dir a git repo (if it is not one) and lands one commit with
// the given subject, so `git log --grep <id>` in it answers for real.
func cwCommit(t *testing.T, dir, subject string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		if _, err := git(dir, args...); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}
	if _, err := git(dir, "rev-parse", "--git-dir"); err != nil {
		run("init", "-b", "main")
		run("config", "user.email", "t@example.com")
		run("config", "user.name", "t")
	}
	name := "f" + strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' {
			return r
		}
		return -1
	}, subject) + ".txt"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(subject+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "--", name)
	run("commit", "-m", subject, "--", name)
}

// A ci-red close that no commit names builds nothing: the gate cleared, a
// persona read the "close this" comment and closed, and a QA session sent to
// verify it has no artefact to look at. Without this, the 7 red episodes
// measured over 6.6 days each cost a second seat on top of the close.
func TestVerifyAfterExemptsACiRedCloseNoCommitNames(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	repo := vaRepo(t, a, closedList("a-1", `["`+CIRedLabel+`","`+CIRedLane+`"]`, "2026-08-18T09:20:06-04:00"))
	writeVerifyWatermark(a.verifyWatermarkPath(repo), time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	// A real repo holding a real commit that does NOT name the bead. The
	// second signal must be "git answered and found nothing", never "git
	// could not answer" — doubt files the bead, which is the arm the
	// no-repo spelling of this fixture was silently testing instead.
	cwCommit(t, repo, "something else entirely")

	n, out, _ := vaRun(t, a, testBd(t))
	if n != 0 {
		t.Errorf("filed %d verify beads for ci-watch's own close, want 0", n)
	}
	if !strings.Contains(out, "no verify bead: a ci-red close no commit names") {
		t.Errorf("the exemption was not named on stdout:\n%s", out)
	}
	if calls := bdCalls(t, fake); strings.Contains(calls, "create verify:") {
		t.Errorf("a verify bead was filed anyway:\n%s", calls)
	}
}

// The other arm, and the one that makes the exemption safe: a persona who
// actually FIXED ci under this bead leaves commits naming it, and that close
// earns its verify bead like any other. The label alone must never suppress
// a control.
func TestVerifyAfterDoesNotExemptACiRedBeadWithCommits(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	a := b.App
	repo := vaRepo(t, a, closedList("a-1", `["`+CIRedLabel+`","`+CIRedLane+`"]`, "2026-08-18T09:20:06-04:00"))
	writeVerifyWatermark(a.verifyWatermarkPath(repo), time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	// A real commit NAMING the bead, in the repo verify-after greps.
	cwCommit(t, repo, "unwedge the linux job (a-1)")

	n, out, _ := vaRun(t, a, testBd(t))
	if n != 1 {
		t.Errorf("filed %d verify beads for a ci-red close that shipped commits, want 1", n)
	}
	if !strings.Contains(out, "ci-red, not exempt: 1 commit(s) name it") {
		t.Errorf("stdout did not say why it was not exempt:\n%s", out)
	}
	if calls := bdCalls(t, fake); !strings.Contains(calls, "create verify:") {
		t.Errorf("no verify bead filed:\n%s", calls)
	}
}

// A close carrying neither label is untouched by any of this — the exemption
// must not widen past the beads ci-watch itself files.
func TestVerifyAfterUnaffectedByTheCiRedExemption(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	a := b.App
	repo := vaRepo(t, a, closedList("a-1", `["code"]`, "2026-08-18T09:20:06-04:00"))
	writeVerifyWatermark(a.verifyWatermarkPath(repo), time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))

	n, out, _ := vaRun(t, a, testBd(t))
	if n != 1 {
		t.Errorf("filed %d, want 1", n)
	}
	if strings.Contains(out, "ci-red") {
		t.Errorf("an ordinary close was judged by the ci-red exemption:\n%s", out)
	}
}
