package posse

// QA suite for the meta layer's prune guard (rangerhq-gvg, verifying the
// close of rangerhq-iz5): the paths the hardening's own tests do not walk.
// Pruning is the one destructive read in posse, and state/ is outside git, so
// every abstention here is worth a test.
//
// Tests marked t.Skip pin a filed bug: they encode the expected behavior
// and fail today. Remove the skip when the bead closes.

import (
	"os"
	"strings"
	"testing"
	"time"
)

// qaStaleMeta writes a meta pointing at a workspace that is not in the
// listing. `socket:` appears only when sock != "" — which is how every meta
// written before 9ac4a16 looks, the field not existing yet.
func qaStaleMeta(t *testing.T, b *HerdrBackend, name, ws, sock string) {
	t.Helper()
	meta := "name: " + name + "\nworkspace: " + ws + "\npane: " + ws + ":p1\nemoji: x\nagent: developer\n"
	if sock != "" {
		meta += "socket: " + sock + "\n"
	}
	os.MkdirAll(b.metaDir(), 0o755)
	if err := os.WriteFile(b.metaPath(name), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
}

// rangerhq-8fq: the invariant rangerhq-iz5 taught, replayed with the
// hardening in place. A meta written before `socket:` existed records no
// socket, and the different-socket arm is written `m.Socket != "" &&
// m.Socket != sock`: it cannot fire for one. A scratch server holding a
// workspace of its own is not an empty listing, so that arm cannot fire
// either. Neither arm fires — and a meta no arm can justify pruning is
// KEPT, not listed: the prune abstains rather than guesses.
func TestSocketlessMetasSurviveAScratchServer(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", "/tmp/sndprobe/herdr.sock") // the scratch server
	b, fake := newTestBackend(t)
	var warn strings.Builder
	b.Warn = &warn

	// The scratch server holds a workspace of its own: NOT an empty listing.
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w9", Label: "sndprobe-watcher"}})
	qaStaleMeta(t, b, "developer-posse", "w404", "")

	sessions, err := b.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := b.readMeta("developer-posse"); !ok {
		t.Fatal("meta pruned by a herdr server that never held its workspace (rangerhq-iz5 repro)")
	}
	for _, s := range sessions {
		if s.Name == "developer-posse" && !s.Foreign {
			t.Errorf("a session whose workspace is not on this server must not be listed: %+v", s)
		}
	}
	if !strings.Contains(warn.String(), "kept, not listed") {
		t.Errorf("refusal not reported: %q", warn.String())
	}
}

// rangerhq-y4z: herdr injects HERDR_SOCKET_PATH into panes, so a session
// created in one records the concrete default-socket path; `posse` run from a
// plain terminal has it unset. Both name the same server, and the guard
// compared them as two — so a genuinely dead workspace was never pruned and
// every listing carried a refusal that is not true. Fails safe, unlike its
// sibling, but it is the same identity bug from the other side.
//
// The fix is in SocketID(): it resolves rather than reads, so the pass from
// the plain terminal names the same path the pane was handed. The fixture
// spells the pane's injected value as SocketID() itself for exactly that
// reason — a hardcoded /Users/x/... would be measuring a string, not the
// agreement between what herdr injects and what posse resolves.
func TestDefaultSocketIsOneServerNotTwo(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", "")
	b, fake := newTestBackend(t) // gives this test its own $HOME to resolve against
	dflt := SocketID()           // what herdr injects into a pane on its default server
	if dflt == "" {
		t.Fatal("setup: the default socket must resolve to a path")
	}

	// Written from inside a pane: herdr set HERDR_SOCKET_PATH, so the meta
	// records the concrete path.
	t.Setenv("HERDR_SOCKET_PATH", dflt)
	var warn strings.Builder
	b.Warn = &warn
	qaStaleMeta(t, b, "ours", "w404", dflt)

	// Read back from a plain terminal against that same default server.
	t.Setenv("HERDR_SOCKET_PATH", "")
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: "live"}})

	if _, err := b.Sessions(); err != nil {
		t.Fatal(err)
	}
	if _, ok := b.readMeta("ours"); ok {
		t.Error("a dead workspace on this very server was kept: '' and the default socket path are one server")
	}
	if strings.Contains(warn.String(), "kept, not listed") {
		t.Errorf("refusal reported for this server's own dead workspace: %q", warn.String())
	}

	// The control: a meta from a genuinely different server on the same
	// board is still kept and still reported. Canonicalizing "" must make
	// one server one, not make every server the same one.
	warn.Reset()
	qaStaleMeta(t, b, "theirs", "w405", "/tmp/y4z/other.sock")
	if _, err := b.Sessions(); err != nil {
		t.Fatal(err)
	}
	if _, ok := b.readMeta("theirs"); !ok {
		t.Error("a meta written against another herdr was pruned by this one")
	}
	if !strings.Contains(warn.String(), "kept, not listed") {
		t.Errorf("refusal not reported for another server's meta: %q", warn.String())
	}
}

// rangerhq-5on: YamlGet maps null / ~ / quoted-empty onto the same "" the
// missing-field arm reads. A scratch server with a workspace of its own
// must not prune any of those spellings — they name no server either.
func TestSocketlessYamlSpellingsSurviveAScratchServer(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", "/tmp/sndprobe/herdr.sock")
	b, fake := newTestBackend(t)
	var warn strings.Builder
	b.Warn = &warn
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w9", Label: "sndprobe-watcher"}})

	spellings := map[string]string{
		"nullv":  "socket: null\n",
		"tilde":  "socket: ~\n",
		"emptyq": "socket: \"\"\n",
	}
	os.MkdirAll(b.metaDir(), 0o755)
	for name, line := range spellings {
		meta := "name: " + name + "\nworkspace: w404\npane: w404:p1\nemoji: x\nagent: developer\n" + line
		if err := os.WriteFile(b.metaPath(name), []byte(meta), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := b.Sessions(); err != nil {
		t.Fatal(err)
	}
	for name := range spellings {
		if _, ok := b.readMeta(name); !ok {
			t.Errorf("%s spelling pruned; YamlGet must treat it as unrecorded (rangerhq-8fq)", name)
		}
	}
	if !strings.Contains(warn.String(), "kept, not listed") {
		t.Errorf("refusal not reported: %q", warn.String())
	}
}

// rangerhq-5on: backfill is a full writeMeta. Known fields — launched
// included — have to survive the stamp, or the cure rewrites session
// identity to fill in a socket.
func TestBackfillPreservesLaunchedAndKnownFields(t *testing.T) {
	const sock = "/tmp/this/herdr.sock"
	t.Setenv("HERDR_SOCKET_PATH", sock)
	b, fake := newTestBackend(t)
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: "old"}})

	const launched = "2026-08-18T23:02:18-04:00"
	meta := "name: old\nworkspace: w1\npane: w1:p1\nemoji: L\nagent: qa\nruntime: claude\ntier: standard\nenvs: qa\nlaunched: " + launched + "\n"
	os.MkdirAll(b.metaDir(), 0o755)
	if err := os.WriteFile(b.metaPath("old"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := b.Sessions(); err != nil {
		t.Fatal(err)
	}
	m, ok := b.readMeta("old")
	if !ok {
		t.Fatal("live meta disappeared during backfill")
	}
	if m.Socket != sock {
		t.Fatalf("socket not backfilled: %+v", m)
	}
	want, err := time.Parse(time.RFC3339, launched)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Launched.Equal(want) {
		t.Errorf("launched not preserved: got %v want %v", m.Launched, want)
	}
	if m.Agent != "qa" || m.Runtime != "claude" || m.Tier != "standard" || m.Envs != "qa" || m.Emoji != "L" {
		t.Errorf("known fields lost: %+v", m)
	}
}

// mustRefuseWrites makes path unwritable and then PROVES it, because 0444 is
// a promise about a uid and not about a file. Root carries CAP_DAC_OVERRIDE
// and rewrites a 0444 file anyway. Measured 2026-08-29, alpine:3 on
// overlayfs, as uid 0 and as uid 65534:
//
//	                                       uid 0    uid 65534
//	chmod 0444 f; printf y >> f            wrote    refused
//	chmod 0555 d; : > d/new                wrote    refused
//	chmod 0555 d; printf y >> d/existing   wrote    WROTE
//
// The third row is why a read-only PARENT directory is not the fix here: the
// capability bypasses a directory's write bit too, and even unprivileged it
// never guarded an existing file — which is the only kind writeMeta targets.
// So a caller that only chmods is asserting the environment its author had.
// Under a hand-rolled `docker run --rm -v "$PWD":/w -w /w golang:1.26 go
// test ./...` the suite runs as uid 0, the backfill SUCCEEDS, and the arm
// below reports a product defect that is not there — which is how
// ranger-base-c00 was found, while rehearsing a release. The repo's own gate
// is not that command: `make test-linux` runs the container as the invoking
// user (scripts/test-linux.sh), CI and darwin are non-root too, and the arm
// is honestly green in all three. It is the by-hand run that lies, and that
// is the run a releaser reaches for.
//
// Nothing in userspace makes a file readable and root-unwritable portably —
// it takes an immutable flag or a read-only mount, neither of which a `go
// test` may assume — and readable is not optional here: the meta has to
// survive readMeta for the backfill to be attempted at all. When the
// environment cannot pose the question, the honest report is that it did not
// ask, not a red that names the wrong culprit. crew_test.go's unwritable-meta
// arm is this same probe written inline, and costscan_root_qa_test.go guards
// its unreadable arms on Geteuid; unify them deliberately, not by accident.
func mustRefuseWrites(t *testing.T, path string) {
	t.Helper()
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(path, 0o644) })
	// O_WRONLY alone: the same permission check os.WriteFile's
	// O_WRONLY|O_CREATE|O_TRUNC makes, without the truncation, so a probe
	// that finds the file writable has not damaged it.
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err == nil {
		f.Close()
		t.Skipf("uid %d can write %s at mode 0444 — root with CAP_DAC_OVERRIDE, "+
			"or a filesystem that ignores the bit. A backfill write that fails "+
			"cannot be staged here, so the arm below would measure the "+
			"environment and not the promise (ranger-base-c00). Run the suite "+
			"as an unprivileged uid to exercise it.", os.Getuid(), path)
	}
}

// rangerhq-5on: backfill is best-effort. A listing must still return the
// live session when the stamp cannot be written.
func TestBackfillDoesNotFailTheListing(t *testing.T) {
	const sock = "/tmp/this/herdr.sock"
	t.Setenv("HERDR_SOCKET_PATH", sock)
	b, fake := newTestBackend(t)
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: "ro"}})

	os.MkdirAll(b.metaDir(), 0o755)
	body := []byte("name: ro\nworkspace: w1\npane: w1:p1\nemoji: R\nagent: qa\n")
	if err := os.WriteFile(b.metaPath("ro"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	mustRefuseWrites(t, b.metaPath("ro"))

	sessions, err := b.Sessions()
	if err != nil {
		t.Fatalf("listing failed over a backfill write: %v", err)
	}
	raw, err := os.ReadFile(b.metaPath("ro"))
	if err != nil {
		t.Fatal("read-only meta vanished")
	}
	if strings.Contains(string(raw), "socket:") {
		t.Error("read-only meta was rewritten")
	}
	found := false
	for _, s := range sessions {
		if s.Name == "ro" && !s.Foreign {
			found = true
		}
	}
	if !found {
		t.Error("live session missing from listing after a failed backfill")
	}
}
