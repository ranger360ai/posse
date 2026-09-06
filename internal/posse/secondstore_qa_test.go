//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for the second-store sweep (ranger-base-dj3k2, ADR 0012 D3, the
// September 2026 adherence audit's finding 6).
//
// The product is bytes an operator reads and a silence everywhere else, so
// what is pinned is: which fixtures report and which are silent, the exact
// sentence each of the two states takes, and that neither surface refuses,
// deletes or changes an exit code because of what it found.
//
// The silences carry the mutation load. Every arm that asserts NOTHING was
// said is a mutant killer for a check dropped out of SweepSecondStores, and
// the redirect-less arm is the one the bead names by hand.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// secondStoreFixture builds an instance whose `beads:` names one working
// copy, and returns that copy plus an instance repo a redirect can point at.
// Neither `.beads` holds anything until an arm plants it.
func secondStoreFixture(t *testing.T) (a *App, work, instance string) {
	t.Helper()
	a = hermetic(t, NewAppAt(t.TempDir()))
	work, instance = t.TempDir(), t.TempDir()
	mkdirAllOrFatal(t, filepath.Join(work, beadsDirName))
	mkdirAllOrFatal(t, filepath.Join(instance, beadsDirName))
	if err := os.WriteFile(a.ConfigPath, []byte("beads:\n  - "+work+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return a, work, instance
}

func mkdirAllOrFatal(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeBeadsFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, beadsDirName, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The whole truth table, one instance per row. The three silent rows are the
// point: this check exists to be quiet in every shape that is not the one
// D3 rejected, or it becomes a line nobody reads.
//
// MUTATION: drop the redirect requirement from SweepSecondStores (report on
// the store files alone) → "a local store and no redirect at all" and "an
// empty .beads" go red. Drop the store-file requirement → "a redirect and
// nothing beside it" goes red. Count an empty issues.jsonl → its row goes
// red.
func TestQASecondStoreReportsOnlyAStoreBesideARedirect(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		// plant runs against the working copy; "" redirect writes none.
		redirect string
		files    map[string]string
		want     []string // basenames the sweep must name; nil = silent
	}{
		{name: "a redirect and nothing beside it", redirect: "<instance>"},
		{name: "a local store and no redirect at all", files: map[string]string{"beads.db": "SQLite format 3\x00"}},
		{name: "an empty .beads"},
		{name: "an empty issues.jsonl beside a redirect", redirect: "<instance>",
			files: map[string]string{beadsJSONL: ""}},
		{name: "a database beside a redirect", redirect: "<instance>",
			files: map[string]string{"beads.db": "SQLite format 3\x00"},
			want:  []string{"beads.db"}},
		{name: "a non-empty issues.jsonl beside a redirect", redirect: "<instance>",
			files: map[string]string{beadsJSONL: `{"id":"x-1"}` + "\n"},
			want:  []string{beadsJSONL}},
		// The audit's own fixture: both files, and the shared-memory
		// sibling that is not itself a store bd would open.
		{name: "both, and a shared-memory sibling", redirect: "<instance>",
			files: map[string]string{
				"beads.db":      "SQLite format 3\x00",
				"beads.db-shm":  "",
				beadsJSONL:      `{"id":"x-1"}` + "\n",
				".gitignore":    "*\n",
				"deleted.jsonl": `{"id":"x-2"}` + "\n",
			},
			want: []string{"beads.db", beadsJSONL}},
		// A database under another name is still a database bd could open,
		// and the audit's shape is "a store nobody meant to keep".
		{name: "a differently named database beside a redirect", redirect: "<instance>",
			files: map[string]string{"queue.db": "SQLite format 3\x00"},
			want:  []string{"queue.db"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			a, work, instance := secondStoreFixture(t)
			if c.redirect != "" {
				writeBeadsFile(t, work, beadsRedirect,
					strings.ReplaceAll(c.redirect, "<instance>", filepath.Join(instance, beadsDirName))+"\n")
			}
			for name, body := range c.files {
				writeBeadsFile(t, work, name, body)
			}
			got := a.SweepSecondStores()
			if len(c.want) == 0 {
				if len(got) != 0 {
					t.Fatalf("want silence, got %d finding(s): %v", len(got), SecondStoreLines(got))
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("want exactly one finding, got %d: %v", len(got), SecondStoreLines(got))
			}
			if strings.Join(got[0].Files, ",") != strings.Join(c.want, ",") {
				t.Errorf("files named:\n got %v\nwant %v", got[0].Files, c.want)
			}
			if got[0].Home != filepath.Join(work, beadsDirName) {
				t.Errorf("home named %q, want %q", got[0].Home, filepath.Join(work, beadsDirName))
			}
			if got[0].Target != filepath.Join(instance, beadsDirName) {
				t.Errorf("target named %q, want the instance .beads", got[0].Target)
			}
			if got[0].Why != "" {
				t.Errorf("a redirect bd follows must carry no reason: %q", got[0].Why)
			}
		})
	}
}

// A redirect bd will NOT follow is the other state, and it is not the same
// finding: bd is reading the local store NOW, and "delete it" would leave bd
// with no graph at all. Every shape beadsRedirectHop can refuse to follow is
// here, so a future edit to that walk cannot quietly turn one of them into
// silence.
//
// MUTATION: report both states with one sentence → the Target/Why split and
// the "will NOT follow" assertion go red together.
func TestQASecondStoreSaysWhenBdIsAlreadyReadingIt(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, redirect string
		wantWhy        string
	}{
		{"the target is gone", "<gone>", "which is not a directory"},
		{"the target is a file", "<file>", "which is not a directory"},
		{"the redirect names no path", "\n", "names no path"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			a, work, instance := secondStoreFixture(t)
			body := c.redirect
			switch c.redirect {
			case "<gone>":
				body = filepath.Join(instance, "no-such-dir", beadsDirName) + "\n"
			case "<file>":
				p := filepath.Join(instance, "a-file")
				if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
				body = p + "\n"
			}
			writeBeadsFile(t, work, beadsRedirect, body)
			writeBeadsFile(t, work, "beads.db", "SQLite format 3\x00")

			got := a.SweepSecondStores()
			if len(got) != 1 {
				t.Fatalf("want one finding, got %d: %v", len(got), SecondStoreLines(got))
			}
			if got[0].Target != "" {
				t.Errorf("a redirect bd will not follow must name no target: %q", got[0].Target)
			}
			if !strings.Contains(got[0].Why, c.wantWhy) {
				t.Errorf("reason:\n got %q\nwant something containing %q", got[0].Why, c.wantWhy)
			}
			line := got[0].Line()
			if !strings.Contains(line, "will NOT follow") || !strings.Contains(line, "bd is reading THIS store now") {
				t.Errorf("the live state must not borrow the latent sentence: %q", line)
			}
			if strings.Contains(line, "follows the redirect today") {
				t.Errorf("the two states must not render the same: %q", line)
			}
		})
	}
}

// The bytes. Both sentences name the path, name what is in it, and name the
// fix — the three things the bead asks the line to carry.
func TestQASecondStoreLineNamesThePathAndTheFix(t *testing.T) {
	t.Parallel()
	latent := SecondStore{
		Home:   "/w/.beads",
		Target: "/i/.beads",
		Files:  []string{"beads.db", beadsJSONL},
	}
	want := "second store: /w/.beads holds beads.db, issues.jsonl beside a redirect to /i/.beads — " +
		"bd follows the redirect today and this store answers the day it is lost; delete it (ADR 0012 D3)"
	if got := latent.Line(); got != want {
		t.Errorf("latent line:\n got %q\nwant %q", got, want)
	}
	live := SecondStore{Home: "/w/.beads", Why: "it names /i/.beads, which is not a directory", Files: []string{"beads.db"}}
	want = "second store: /w/.beads holds beads.db beside a redirect bd will NOT follow " +
		"(it names /i/.beads, which is not a directory) — bd is reading THIS store now; " +
		"repair the redirect, then delete it (ADR 0012 D3)"
	if got := live.Line(); got != want {
		t.Errorf("live line:\n got %q\nwant %q", got, want)
	}
	// One line per finding, and no line at all for a clean sweep — the
	// silence every surface depends on.
	if got := SecondStoreLines(nil); got != nil {
		t.Errorf("a clean sweep renders nothing, got %v", got)
	}
}

// A redirect pointing back at the directory that holds it is one store
// reached by a pointless hop, not two: bd reads exactly those files, so
// naming them as a store to delete would be an instruction to delete the
// store of record.
//
// MUTATION: drop the self-redirect guard → red.
func TestQASecondStoreIgnoresARedirectToItself(t *testing.T) {
	t.Parallel()
	a, work, _ := secondStoreFixture(t)
	writeBeadsFile(t, work, beadsRedirect, filepath.Join(work, beadsDirName)+"\n")
	writeBeadsFile(t, work, "beads.db", "SQLite format 3\x00")
	if got := a.SweepSecondStores(); len(got) != 0 {
		t.Errorf("a self-redirect is one store, not two: %v", SecondStoreLines(got))
	}
}

// Two spellings of one checkout are one store and one finding. bd would open
// the same file twice; an operator must not be handed the same `rm` twice.
func TestQASecondStoreDeduplicatesByPath(t *testing.T) {
	t.Parallel()
	a, work, instance := secondStoreFixture(t)
	writeBeadsFile(t, work, beadsRedirect, filepath.Join(instance, beadsDirName)+"\n")
	writeBeadsFile(t, work, "beads.db", "SQLite format 3\x00")
	if err := os.WriteFile(a.ConfigPath,
		[]byte("beads:\n  - "+work+"\n  - "+filepath.Join(work, ".")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := a.SweepSecondStores(); len(got) != 1 {
		t.Errorf("two spellings of one checkout are one finding, got %d: %v", len(got), SecondStoreLines(got))
	}
}

// `posse status`'s half, through the App method the command calls: it prints
// the line, it prints nothing when there is nothing, and — the boundary the
// bead draws in capitals — it is a report. It touches no file it named.
//
// MUTATION: make ReportSecondStores remove or rename anything it found → the
// survival assertions go red.
func TestQASecondStoreReportsAndRefusesNothing(t *testing.T) {
	t.Parallel()
	a, work, instance := secondStoreFixture(t)

	var b strings.Builder
	if a.ReportSecondStores(&b) || b.String() != "" {
		t.Fatalf("a clean instance must say nothing, said %q", b.String())
	}

	writeBeadsFile(t, work, beadsRedirect, filepath.Join(instance, beadsDirName)+"\n")
	writeBeadsFile(t, work, "beads.db", "SQLite format 3\x00")
	writeBeadsFile(t, work, beadsJSONL, `{"id":"x-1"}`+"\n")
	b.Reset()
	if !a.ReportSecondStores(&b) {
		t.Fatalf("a second store must be reported: %q", b.String())
	}
	out := strings.TrimRight(b.String(), "\n")
	if strings.Contains(out, "\n") {
		t.Errorf("one finding is one line:\n%s", out)
	}
	if !strings.Contains(out, filepath.Join(work, beadsDirName)) || !strings.Contains(out, "beads.db") {
		t.Errorf("the line must name the path and what is in it: %q", out)
	}

	// Reported, and still there — twice, because a delete that only fires
	// on the second look would pass a single-call assertion.
	for i := 0; i < 2; i++ {
		a.ReportSecondStores(&b)
	}
	for _, name := range []string{beadsRedirect, "beads.db", beadsJSONL} {
		if _, err := os.Stat(filepath.Join(work, beadsDirName, name)); err != nil {
			t.Errorf("the report deleted %s: %v", name, err)
		}
	}
	// And the redirect target is untouched too: the fix the line prescribes
	// is the operator's `rm`, not this reader's.
	if _, err := os.Stat(filepath.Join(instance, beadsDirName)); err != nil {
		t.Errorf("the report touched the store of record: %v", err)
	}
}

// The line is really on the dispatch pass preamble, once, and the loop
// dispatches identically either way. A cancelled context runs one pass and
// stops, so everything in the buffer is that pass's.
//
// MUTATION: drop the SweepSecondStores call from Watch → red.
func TestQAWatchPassPreamblePrintsTheSecondStoreLine(t *testing.T) {
	t.Parallel()
	for _, planted := range []bool{false, true} {
		name := "clean"
		if planted {
			name = "a second store"
		}
		t.Run(name, func(t *testing.T) {
			b, _ := newTestBackend(t)
			d := newTestDispatcher(t, b)
			work := qaRepo(t, b.App, `[]`, "")
			instance := t.TempDir()
			mkdirAllOrFatal(t, filepath.Join(work, beadsDirName))
			mkdirAllOrFatal(t, filepath.Join(instance, beadsDirName))
			if planted {
				writeBeadsFile(t, work, beadsRedirect, filepath.Join(instance, beadsDirName)+"\n")
				writeBeadsFile(t, work, "beads.db", "SQLite format 3\x00")
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if _, err := d.Watch(ctx, "", "", 0, 10*time.Millisecond, 10*time.Millisecond); err != nil {
				t.Fatalf("a second store must not decide whether the loop runs: %v", err)
			}
			out := dispatcherOut(d)
			said := strings.Count(out, "second store: ")
			want := 0
			if planted {
				want = 1
			}
			if said != want {
				t.Fatalf("the pass preamble said the second-store line %d times, want %d:\n%s", said, want, out)
			}
			if planted && !strings.Contains(out, filepath.Join(work, beadsDirName)) {
				t.Errorf("the line must name the path:\n%s", out)
			}
		})
	}
}

// Said once per finding, not once per pass: a standing store is one line in
// a log that runs for hours, and a store that appears mid-loop is still
// said. The second half is what keeps this from being a mute.
//
// MUTATION: drop the `said != secondStoreSaid` guard in Watch → the "once"
// assertion goes red. Never reset it on a clean sweep → the reappearance
// assertion goes red.
func TestQAWatchSaysASecondStoreOncePerFinding(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	out := &syncBuf{}
	d.Out = out
	work := qaRepo(t, b.App, `[]`, "")
	instance := t.TempDir()
	mkdirAllOrFatal(t, filepath.Join(work, beadsDirName))
	mkdirAllOrFatal(t, filepath.Join(instance, beadsDirName))
	writeBeadsFile(t, work, beadsRedirect, filepath.Join(instance, beadsDirName)+"\n")
	writeBeadsFile(t, work, "beads.db", "SQLite format 3\x00")

	ctx, cancel := context.WithCancel(context.Background())
	loop := make(chan struct{})
	go func() {
		defer close(loop)
		d.Watch(ctx, "", "", 0, 10*time.Millisecond, 20*time.Millisecond)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-loop:
		case <-time.After(30 * time.Second):
			t.Errorf("the watch loop never returned after cancel:\n%s", out.String())
		}
	})

	// Three passes over an unchanged finding: one line.
	waitForCount(t, out, "── pass ", 3)
	if n := strings.Count(out.String(), "second store: "); n != 1 {
		t.Fatalf("a standing finding was said %d times over 3 passes, want 1:\n%s", n, out.String())
	}

	// The operator deletes it: silence, and the memory is cleared…
	if err := os.Remove(filepath.Join(work, beadsDirName, "beads.db")); err != nil {
		t.Fatal(err)
	}
	waitForCount(t, out, "── pass ", 6)
	if n := strings.Count(out.String(), "second store: "); n != 1 {
		t.Fatalf("a cleared finding must not be re-announced, said %d times:\n%s", n, out.String())
	}

	// …so the same store coming back is said again rather than swallowed.
	writeBeadsFile(t, work, "beads.db", "SQLite format 3\x00")
	waitForCount(t, out, "second store: ", 2)
}
