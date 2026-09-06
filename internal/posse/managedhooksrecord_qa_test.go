//go:build posse_arm2

package posse

// QA pins for the managed-hooks RECORD (ranger-base-m6szh, escaped from
// ranger-base-buvq4). This file was internal/posse/queueremote_qa_test.go
// until ranger-base-gjbdl: it carried m6szh's two findings, and the first
// of them — checkQueueRemote reading only the first URL of the sanctioned
// remote — went out with the source-remote check itself when ADR 0049 was
// simplified. The pin went with the code it held; the second finding, which
// was never about backup at all, stayed and gave the file its name.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
