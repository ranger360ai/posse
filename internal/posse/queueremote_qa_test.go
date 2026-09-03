package posse

import (
	"strings"
	"testing"
)

// PARKED (laurie, ranger-base-vtyst verifying ranger-base-ymgbo). ADR 0049 D2
// says the queue may hold exactly one remote whose fetch URL and push URL
// both equal the declared string. checkQueueRemote reads `git remote
// get-url` and `get-url --push`, and git-remote(1) says of get-url: "By
// default, only the first URL is listed." A remote may carry MORE than one
// url — `git remote set-url --add` — and git-config(1) on remote.<name>.url:
// "the first is used for fetching, and all are used for pushing (assuming no
// remote.<name>.pushurl is defined)". So a remote whose first url is the
// sanctioned one and whose second is anywhere else prints the sanctioned URL
// on both reads, passes the check, and every operator push lands at both.
// MEASURED here, git 2.50.1: get-url --push prints the declared URL;
// get-url --all --push prints two lines; checkQueueRemote returns nil.
//
// Un-skip with the fix (read `--all` on both sides and require exactly one
// line equal to declared). Un-skipped today this FAILS at the second arm; the
// first arm (a second PUSH url) is already refused, and is the control.
func TestQAQueueRemoteRefusesASecondURLOnTheSanctionedRemote(t *testing.T) {
	t.Parallel()
	t.Skip("parked: ranger-base-vtyst finding on ranger-base-ymgbo — checkQueueRemote reads only the first URL of the remote")
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

// PARKED (laurie, ranger-base-vtyst verifying ranger-base-buvq4). The launch
// refuses a managed hooks path carrying \n or \r before the record is
// written, and writeMeta's comment now says the flat reader's truncation "is
// guarded at the launch". The reader truncates more than a newline: yamlClean
// cuts at ` #`, strips a wrapping pair of double quotes, trims trailing
// blanks, and reads `~`/`null` as unset. Each is a legal absolute path (git
// round-trips them in core.hooksPath), each passes the guard, and each is
// recorded WRONG. No runtime consumer reads managed_hooks back today, so this
// is the record and nothing else. Un-skip with the fix (refuse, or escape,
// every value the reader cannot round-trip).
func TestQAManagedHooksRecordRoundTripsEveryOneLinePath(t *testing.T) {
	t.Parallel()
	t.Skip("parked: ranger-base-vtyst finding on ranger-base-buvq4 — the one-line guard is narrower than the reader's rules")
	b, _ := newTestBackend(t)
	for _, p := range []string{"/opt/managed hooks #v2", "\"/opt/managed-hooks\"", "/opt/managed-hooks ", "~", "null"} {
		if err := b.writeMeta(&HerdrMeta{Name: "s1", HooksMode: "redirect", ManagedHooks: p}); err != nil {
			t.Fatal(err)
		}
		got, ok := b.readMeta("s1")
		if !ok {
			t.Fatal("meta unreadable")
		}
		if got.ManagedHooks != p {
			t.Errorf("managed_hooks did not round-trip: wrote %q, read %q", p, got.ManagedHooks)
		}
	}
}
