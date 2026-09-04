package posse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FIXED (ranger-base-m6szh; parked by ranger-base-vtyst verifying
// ranger-base-ymgbo). ADR 0049 D2 says the queue may hold exactly one remote
// whose fetch URL and push URL both equal the declared string.
// checkQueueRemote read `git remote get-url` and `get-url --push`, and
// git-remote(1) says of get-url: "By default, only the first URL is listed."
// A remote may carry MORE than one url — `git remote set-url --add` — and
// git-config(1) on remote.<name>.url: "the first is used for fetching, and
// all are used for pushing (assuming no remote.<name>.pushurl is defined)".
// So a remote whose first url was the sanctioned one and whose second was
// anywhere else printed the sanctioned URL on both reads, passed the check,
// and every operator push landed at both. The check now reads --all on both
// sides. The first arm (a second PUSH url) was already refused and is the
// control: it holds the fix to the escape it names.
func TestQAQueueRemoteRefusesASecondURLOnTheSanctionedRemote(t *testing.T) {
	t.Parallel()
	declared := "https://example.invalid/queue.git"
	for _, arm := range []struct {
		add     []string
		pushAll int // lines `get-url --all --push` prints: a pushurl REPLACES the url list for pushing
	}{
		{[]string{"remote", "set-url", "--add", "--push", "origin", "https://example.invalid/elsewhere.git"}, 1}, // control: refused today
		{[]string{"remote", "set-url", "--add", "origin", "https://example.invalid/elsewhere.git"}, 2},           // the finding
	} {
		queue := t.TempDir()
		mustGit(t, queue, "init", "-q", ".")
		mustGit(t, queue, "remote", "add", "origin", declared)
		mustGit(t, queue, arm.add...)
		all, _ := git(queue, "remote", "get-url", "--all", "--push", "origin")
		if n := strings.Count(all, "\n") + 1; n != arm.pushAll {
			t.Fatalf("fixture: want %d push URL(s) on origin, got %d: %q", arm.pushAll, n, all)
		}
		if err := checkQueueRemote(queue, declared); err == nil {
			t.Errorf("%v: a remote that pushes to a second URL passed as the sanctioned one (ADR 0049 D2)", arm.add[1:])
		}
	}
}

// FIXED (ranger-base-m6szh; parked by ranger-base-vtyst verifying
// ranger-base-buvq4). The launch refused a managed hooks path carrying \n or
// \r before the record was written, and writeMeta's comment said the flat
// reader's truncation "is guarded at the launch". The reader truncates more
// than a newline: yamlClean cuts at " #", strips a wrapping pair of double
// quotes, trims the value, and yamlGetLines reads "~"/"null" as unset. Each
// is a legal path, each passed the guard, and each was recorded WRONG and
// silently. This is the measurement the guard is now written against: what
// the record would read back for each of the five, and that the predicate
// the launch asks says so.
//
// It is a REFUSAL and not an escaping, because there is no encoding to
// escape to: the comment cut runs before the quotes come off, so a quoted
// "/opt/x #v2" reads back as `"/opt/x`, and "~" reads unset however it is
// spelled. Quoting rescues two of the five and the flat subset is a
// deliberate twin of the awk reader in bin/posse (NOTES.md), so the honest
// answer is to refuse the record, not to widen the dialect under it.
func TestQAManagedHooksRecordIsMangledByEveryRuleTheFlatReaderHas(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	for _, c := range []struct {
		wrote, reads string
	}{
		{"/opt/managed hooks #v2", "/opt/managed hooks"}, // yamlClean cuts at " #"
		{"\"/opt/managed-hooks\"", "/opt/managed-hooks"}, // a wrapping pair of quotes comes off
		{"/opt/managed-hooks ", "/opt/managed-hooks"},    // TrimSpace
		{"~", ""},    // yamlGetLines: unset
		{"null", ""}, // yamlGetLines: unset
		{"/opt/managed-hooks", "/opt/managed-hooks"}, // control: carried whole
	} {
		if err := b.writeMeta(&HerdrMeta{Name: "s1", HooksMode: "redirect", ManagedHooks: c.wrote}); err != nil {
			t.Fatal(err)
		}
		got, ok := b.readMeta("s1")
		if !ok {
			t.Fatal("meta unreadable")
		}
		if got.ManagedHooks != c.reads {
			t.Errorf("wrote managed_hooks %q, read %q, want %q", c.wrote, got.ManagedHooks, c.reads)
		}
		// The predicate the launch guard asks answers for the same value,
		// and answers with the same read-back — one instrument, so the
		// guard cannot fall behind the reader a second time.
		if pred, carried := flatScalarRoundTrip(c.wrote); pred != c.reads || carried != (c.wrote == c.reads) {
			t.Errorf("flatScalarRoundTrip(%q) = %q, %v; the record read back %q", c.wrote, pred, carried, c.reads)
		}
	}
}

// The refusal itself, at the launch. Of the reader's four mangling rules,
// exactly ONE is reachable by a real managed hooks dir: " #" in a directory
// name. MEASURED here, git 2.50.1 — an absolute path always starts with "/",
// so the wrapping-quote strip and the "~"/"null" rules cannot fire on one;
// and a trailing blank survives git (`core.hooksPath` is written quoted and
// `rev-parse --git-path hooks` prints the blank) but not posse, because
// gitPathRaw TrimSpaces git's output, so mh.Dir never carries one and the
// stripped path it names does not exist. The guard is written against the
// reader rather than against this list on purpose: reachability is a
// property of two other files, and it can change without either of them
// remembering this one (ranger-base-m6szh, escaped from ranger-base-buvq4).
// Refused before a workspace, a render or a record exists, exactly as the
// newline arm is. The control arm is
// TestQALaunchIntoAManagedRepoRendersTheRedirectAndCarriesItInTheEnv: the
// same launch over a path the record carries whole goes through.
func TestQALaunchRefusesAManagedHooksPathTheRecordWouldMangle(t *testing.T) {
	t.Parallel()
	b, repo := lhpFixture(t, VisibilityPrivate)
	managed := filepath.Join(t.TempDir(), "managed hooks #v2")
	if err := os.MkdirAll(managed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(managed, "pre-commit"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	qaGit(t, repo, "config", "core.hooksPath", managed)
	mhpLock(t, managed)
	// Staged, not assumed: git round-trips the path, posse classifies it
	// managed, and the record would really mangle it.
	if m := mustManaged(t, repo); m.Dir != managed {
		t.Fatalf("git did not round-trip the path: %q, want %q", m.Dir, managed)
	}
	if got, ok := flatScalarRoundTrip(managed); ok {
		t.Fatalf("fixture: the record carries %q whole (read back %q) — nothing to refuse", managed, got)
	}

	err := b.CreateSession(NewSessionOpts{Name: "s1", Dir: repo, Agent: "ranger", AllowDegraded: true})
	if err == nil || !strings.Contains(err.Error(), "cannot be recorded") {
		t.Fatalf("CreateSession over %q = %v, want a refusal naming the path", managed, err)
	}
	if !strings.Contains(err.Error(), managed) {
		t.Errorf("the refusal does not name the path it refused: %v", err)
	}
	if _, err := os.Stat(b.metaPath("s1")); !os.IsNotExist(err) {
		t.Errorf("a refused launch wrote a record (%v)", err)
	}
	if _, err := os.Stat(b.App.SessionHooksDir("s1")); !os.IsNotExist(err) {
		t.Errorf("a refused launch rendered a hooks dir (%v)", err)
	}
}
