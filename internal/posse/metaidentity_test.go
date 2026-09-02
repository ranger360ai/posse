package posse

// rangerhq-yt1p: a workspace id identifies a session only inside one herdr
// server process. herdr's allocator is max(live id)+1, recomputed from the
// live set at every server start — a restart and a live handoff both — so
// every id above the live high-water is handed out again on the far side,
// and that is precisely the set of ids stale metas hold (measured in
// rangerhq-6bg7, NOTES "Workspace ids recycle across a server process
// boundary").
//
// So `WorkspaceAlive(id)` answers liveness and the meta rule needs identity:
// the label says whose workspace it is, and the api socket's inode says
// which server generation issued the id. The boards below are the incident
// in miniature — a stranger holding a stale meta's id — plus the false
// negative that must not become a delete (a workspace renamed in herdr).
//
// The socket file here is an ordinary file, not a socket: only its inode is
// read, and a restart/handoff is simulated the way herdr does it — the same
// path, recreated.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// idProbeSocket points this pass at its own api socket file and returns the
// path. The path never changes afterwards, which is the whole trap: `socket:`
// is identical across a restart and a handoff, so the rangerhq-8fq guard
// cannot see a generation boundary.
func idProbeSocket(t *testing.T) string {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "herdr.sock")
	newGeneration(t, sock)
	t.Setenv("HERDR_SOCKET_PATH", sock)
	return sock
}

// newGeneration recreates the api socket file the way a herdr restart or a
// live handoff does: the same path, bound again.
//
// The BIND TIME is set here rather than left to the filesystem, and that is
// the fixture half of ranger-base-fjj. Two measured reasons. Linux hands
// back the inode of the file just unlinked, so on ext4 and overlayfs the
// path and inode are identical on both sides of a restart and cannot
// separate the generations by themselves. And file timestamps come from the
// kernel's coarse clock — 1ms on a CONFIG_HZ=1000 runner, more elsewhere —
// so a recreate this fast can land in the same tick as the file it replaced.
// A real restart is a server startup apart; sleeping to imitate that would
// buy a flaky test and nothing else, so the interval is stated instead.
func newGeneration(t *testing.T, sock string) {
	t.Helper()
	bound := genEpoch
	if st, err := os.Stat(sock); err == nil {
		bound = st.ModTime().Add(time.Minute) // the server that just died was up a while
	}
	os.Remove(sock)
	if err := os.WriteFile(sock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(sock, bound, bound); err != nil {
		t.Fatal(err)
	}
}

// genEpoch is when a test's first server generation bound its socket. Fixed,
// so a failure reads the same on two runs and on two platforms.
var genEpoch = time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC)

func hideFromTheListing(t *testing.T, fake string, ids ...string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(fake, "hidden-from-list"), []byte(strings.Join(ids, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
}

func metaOf(t *testing.T, b *HerdrBackend, name string) *HerdrMeta {
	t.Helper()
	m, ok := b.readMeta(name)
	if !ok {
		t.Fatalf("no meta for %s", name)
	}
	return m
}

// ─── the fence itself ────────────────────────────────────────────────────────

// ServerGen is the generation token herdr does not offer: `workspace get`
// carries no creation time and `api snapshot` no server pid or boot id, but
// the api socket file is recreated by both a restart and a handoff, so its
// inode fences the id space exactly.
func TestServerGenFencesOneServerProcess(t *testing.T) {
	sock := idProbeSocket(t)

	first := ServerGen()
	if first == "" {
		t.Fatal("no generation for a socket that exists")
	}
	if again := ServerGen(); again != first {
		t.Errorf("the generation moved without the socket being recreated: %q → %q", first, again)
	}

	newGeneration(t, sock) // the restart, or the handoff
	if after := ServerGen(); after == first {
		t.Errorf("the socket was recreated and the generation did not move (%q) — the fence is inert", after)
	}

	// An unreachable socket is an unknown generation, never a match: the
	// arm the prune already takes for an absent socket.
	os.Remove(sock)
	if gone := ServerGen(); gone != "" {
		t.Errorf("a socket that is not there must not name a generation, got %q", gone)
	}
}

// The linux shape of ranger-base-fjj, pinned where every platform runs it.
// A real recreate cannot express this case on APFS — the filesystem refuses
// to hand the inode back — and that is exactly why the defect shipped: the
// fence was only ever exercised where its assumption happened to hold. So
// the token is asked the question directly.
func TestGenTokenSeparatesAGenerationThatRecycledTheInode(t *testing.T) {
	t.Parallel()
	// Both sides measured in a golang:1.26 container, unlink and recreate:
	// the inode came back, and the coarse clock stamped both files alike.
	const recycled = "66:587500"
	bound := time.Unix(1787577362, 616440001)

	first := genToken(recycled, bound)
	if again := genToken(recycled, bound); again != first {
		t.Errorf("one socket named two generations: %q → %q", first, again)
	}
	if rebound := genToken(recycled, bound.Add(time.Second)); rebound == first {
		t.Errorf("a recycled inode bound again named the same generation (%q) — the fence is inert, and an inert fence is rangerhq-yt1p", rebound)
	}

	// scripts/verify-prune-guard.sh must accept N:N:N and reject N:N$.
	// A two-field $ regex is a false FAIL against a correct stamp (rangerhq-u4f7).
	three := regexp.MustCompile(`^[0-9]+:[0-9]+:[0-9]+$`)
	two := regexp.MustCompile(`^[0-9]+:[0-9]+$`)
	if !three.MatchString(first) {
		t.Errorf("genToken %q is not N:N:N — the promote gate regex is now wrong", first)
	}
	if two.MatchString(first) {
		t.Errorf("two-field $ regex matched %q — that is the false FAIL rangerhq-u4f7 caught", first)
	}
}

// rangerhq-u4f7: ranger-base-fjj changed the token to three fields and left
// the promote gate asserting two. The success path must be N:N:N.
func TestVerifyPruneGuardScriptPinsThreeFieldGen(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(filepath.Join("..", "..", "scripts", "verify-prune-guard.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `^gen: [0-9][0-9]*:[0-9][0-9]*:[0-9][0-9]*$`) {
		t.Error("verify-prune-guard.sh no longer asserts the three-field gen: token (ranger-base-fjj); a two-field $ regex is a false FAIL")
	}
}

// Without HERDR_SOCKET_PATH herdr resolves the socket itself, and a named
// session does NOT live on the default server's socket — fencing against
// that one would compare posse's ids to a server it is not talking to.
func TestServerGenResolvesTheSocketHerdrWouldUse(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("HERDR_SOCKET_PATH", "")
	t.Setenv("HERDR_SESSION", "")

	if gen := ServerGen(); gen != "" {
		t.Errorf("no socket on disk anywhere, yet a generation was named: %q", gen)
	}

	dflt := filepath.Join(home, ".config", "herdr", "herdr.sock")
	named := filepath.Join(home, ".config", "herdr", "sessions", "probe", "herdr.sock")
	for _, p := range []string{dflt, named} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	def := ServerGen()
	if def == "" {
		t.Fatal("the default server's socket exists and named no generation")
	}
	t.Setenv("HERDR_SESSION", "probe")
	if got := ServerGen(); got == "" || got == def {
		t.Errorf("a named session must fence on its own socket, not the default server's (got %q, default %q)", got, def)
	}
}

// ─── the listing ─────────────────────────────────────────────────────────────

// The incident, on the read side. A meta whose id was re-issued to somebody
// else's workspace must not be listed over it: every addressing path
// (Resolve, AgentTarget, KillSession) answers out of this listing, so the
// name would prompt into a stranger's pane and `posse kill` would close it.
// The file is kept — "not mine" is never "delete the meta".
func TestSessionsWillNotListAStrangersWorkspaceUnderOurName(t *testing.T) {
	sock := idProbeSocket(t)
	b, fake := newTestBackend(t)
	var warn strings.Builder
	b.Warn = &warn
	mustCreate(t, b, NewSessionOpts{Name: "foo", Cmd: "claude"})
	ws := metaOf(t, b, "foo").Workspace

	// herdr restarts; the allocator climbs back through foo's id and hands
	// it to a session created after it.
	newGeneration(t, sock)
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: ws, Label: "alpha"}})

	sessions, err := b.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range sessions {
		if s.Name == "foo" {
			t.Errorf("listed foo over workspace %s, which is alpha's — a prompt would land in a stranger's pane: %+v", ws, s)
		}
	}
	if _, ok := b.readMeta("foo"); !ok {
		t.Fatal("foo's meta was deleted: 'not mine' must never mean 'delete the meta'")
	}
	// The stranger's workspace is still the herd's: unclaimed, so it lists
	// under its own label.
	var sawAlpha bool
	for _, s := range sessions {
		if s.Name == "alpha" && s.WorkspaceID == ws {
			sawAlpha = true
		}
	}
	if !sawAlpha {
		t.Errorf("the workspace holding %s dropped out of the listing entirely: %+v", ws, sessions)
	}
	if _, err := b.Resolve("foo"); err == nil {
		t.Error("Resolve still hands out a session whose id belongs to somebody else")
	}
	for _, want := range []string{"foo", "alpha", ws, "rangerhq-yt1p", "repair"} {
		if !strings.Contains(warn.String(), want) {
			t.Errorf("the warning does not say %q — the operator cannot act on it:\n%s", want, warn.String())
		}
	}
}

// The false negative, and it is the one that must never become a delete: a
// workspace renamed in herdr fails the label check while still being ours.
// Inside one server generation an id is never re-issued, so the fence says
// "rename" and the session keeps its name.
func TestSessionsKeepListingAWorkspaceRenamedInHerdr(t *testing.T) {
	idProbeSocket(t)
	b, fake := newTestBackend(t)
	mustCreate(t, b, NewSessionOpts{Name: "foo", Cmd: "claude"})
	ws := metaOf(t, b, "foo").Workspace

	// Same server, same generation — the operator renamed the workspace.
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: ws, Label: "renamed-by-hand"}})

	sessions, err := b.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, s := range sessions {
		if s.Name == "foo" && s.WorkspaceID == ws {
			found = true
		}
	}
	if !found {
		t.Errorf("a renamed workspace is still ours — ids are not re-issued inside one server process: %+v", sessions)
	}
}

// gen: is stamped on positive identity only. The workspace this server holds
// under the meta's id wears the meta's own name, so it is proof; anything
// weaker would forge the fence the next generation reads.
func TestTheGenerationIsBackfilledOnlyOnPositiveIdentity(t *testing.T) {
	sock := idProbeSocket(t)
	b, fake := newTestBackend(t)
	gen := ServerGen()

	// Two metas written before gen: existed (rangerhq-8fq's shape, one field
	// later): one whose workspace still wears its name — proof — and one
	// whose workspace carries no label at all, which is not evidence either
	// way. Both are listed; only the first may stamp.
	for _, m := range []*HerdrMeta{
		{Name: "legacy", Workspace: "wz1", Pane: "wz1:p1", Socket: sock},
		{Name: "unlabelled", Workspace: "wz2", Pane: "wz2:p1", Socket: sock},
	} {
		if err := b.writeMeta(m); err != nil {
			t.Fatal(err)
		}
	}
	saveWSTo(t, fake, []fakeWS{
		{WorkspaceID: "wz1", Label: "legacy"},
		{WorkspaceID: "wz2", Label: ""},
	})

	sessions, err := b.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 2 {
		t.Fatalf("both metas hold live workspaces and must be listed: %+v", sessions)
	}
	if got := metaOf(t, b, "legacy").Gen; got != gen {
		t.Errorf("holding the workspace under our own label is the proof the file never carried: gen %q, want %q", got, gen)
	}
	if got := metaOf(t, b, "unlabelled").Gen; got != "" {
		t.Errorf("stamped a generation on no identity evidence (%q) — a forged fence is worse than an absent one, it makes the next generation trust a recycled id", got)
	}
}

// ─── the prune ───────────────────────────────────────────────────────────────

// Alive keeps the file whether the workspace is ours or a stranger's, so the
// decision does not move here — what moves is what the operator is told.
// "Alive" reads as "your session is fine, elsewhere"; a squatted id needs a
// repair, and a warning that cannot tell them apart hides it forever.
func TestPruneNamesAStrangerHoldingTheId(t *testing.T) {
	sock := idProbeSocket(t)
	b, fake := newTestBackend(t)
	var warn strings.Builder
	b.Warn = &warn
	mustCreate(t, b, NewSessionOpts{Name: "mine", Cmd: "claude"})  // keeps the listing non-empty
	mustCreate(t, b, NewSessionOpts{Name: "ghost", Cmd: "claude"}) // the stale meta
	mine, ghost := metaOf(t, b, "mine").Workspace, metaOf(t, b, "ghost").Workspace

	m := metaOf(t, b, "ghost")
	m.Launched = time.Now().Add(-2 * PruneGrace) // past the grace: only the id can save it
	if err := b.writeMeta(m); err != nil {
		t.Fatal(err)
	}

	// The restart re-issued ghost's id, and this pass's listing does not
	// carry it — so the prune reaches its per-id query.
	newGeneration(t, sock)
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: mine, Label: "mine"}, {WorkspaceID: ghost, Label: "alpha"}})
	hideFromTheListing(t, fake, ghost)

	if _, err := b.Sessions(); err != nil {
		t.Fatal(err)
	}
	if _, ok := b.readMeta("ghost"); !ok {
		t.Fatal("pruned a meta on an id somebody else holds — the answer was about a stranger, not about this session")
	}
	if !strings.Contains(warn.String(), "alpha") {
		t.Errorf("the operator is told the meta was kept but not that its id was re-issued:\n%s", warn.String())
	}
}

// ─── the create ──────────────────────────────────────────────────────────────

// The write side of the same fact (rangerhq-jeu2: one predicate, both
// halves). A stranger answering for the id proves nothing about this
// session, and on the destructive write "proves nothing" is a refusal —
// proceeding would overwrite the only record of a session state/ keeps
// outside git.
func TestCreateRefusesWhenAStrangerHoldsTheRecordedId(t *testing.T) {
	sock := idProbeSocket(t)
	b, fake := newTestBackend(t)
	mustCreate(t, b, NewSessionOpts{Name: "foo", Cmd: "claude"})
	ws := metaOf(t, b, "foo").Workspace

	newGeneration(t, sock)
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: ws, Label: "alpha"}})
	before, err := os.ReadFile(b.metaPath("foo"))
	if err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(fake, "calls.log"))

	err = b.CreateSession(NewSessionOpts{Name: "foo", Cmd: "claude", Dir: t.TempDir()})
	if err == nil {
		t.Fatal("created over the only record of foo on an id that says nothing about foo")
	}
	if !strings.Contains(err.Error(), ws) || !strings.Contains(err.Error(), "attach foo") {
		t.Errorf("the refusal names neither the workspace nor the way out: %v", err)
	}
	after, rerr := os.ReadFile(b.metaPath("foo"))
	if rerr != nil || string(after) != string(before) {
		t.Errorf("foo's meta was rewritten by a refused create:\n%s", after)
	}
	if log := calls(t, fake); strings.Contains(log, "workspace create") {
		t.Errorf("a workspace was created before the refusal:\n%s", log)
	}
}

// The control, and it is the one that keeps the fence from becoming a
// blanket refusal: a generation boundary does not make every name
// un-creatable. A workspace never changes its id while it exists, so
// workspace_not_found is death in this generation and any other.
func TestCreateStillReusesTheNameOfAnIdNobodyHolds(t *testing.T) {
	sock := idProbeSocket(t)
	b, fake := newTestBackend(t)
	mustCreate(t, b, NewSessionOpts{Name: "foo", Cmd: "claude"})
	ws := metaOf(t, b, "foo").Workspace

	// Restart, and foo's workspace did not come back with it. Something
	// else is running, so the listing is not empty.
	newGeneration(t, sock)
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: "wz9", Label: "somebody-else"}})

	if err := b.CreateSession(NewSessionOpts{Name: "foo", Cmd: "claude", Dir: t.TempDir()}); err != nil {
		t.Fatalf("a name whose id this server confirms nobody holds must be reusable: %v", err)
	}
	m := metaOf(t, b, "foo")
	if m.Workspace == ws || m.Workspace == "" {
		t.Errorf("the replacement workspace was not recorded: %+v", m)
	}
	if m.Gen != ServerGen() || m.Gen == "" {
		t.Errorf("a create records the generation that issued its id: gen %q, want %q", m.Gen, ServerGen())
	}
	if log := calls(t, fake); !strings.Contains(log, "workspace get "+ws) {
		t.Errorf("overwrote a meta without asking herdr about its workspace:\n%s", log)
	}
}

// The typing path, one caller further out. RelaunchAgent re-types a persona
// command into the pane its meta records, and a pane id is only as good as
// the workspace id it hangs off — so a meta the listing refuses to claim
// must not be typed into either (rangerhq-i2g9's rule: walk every caller
// that reaches the same line).
func TestRelaunchAgentWillNotTypeIntoAStrangersPane(t *testing.T) {
	sock := idProbeSocket(t)
	b, fake := newTestBackend(t)
	writePersona(t, b.App, "ranger", "[go]")
	mustCreate(t, b, NewSessionOpts{Name: "foo", Agent: "ranger"})
	m := metaOf(t, b, "foo")
	m.Launched = time.Now().Add(-time.Hour) // past the startup grace
	if err := b.writeMeta(m); err != nil {
		t.Fatal(err)
	}

	// The restart re-issued foo's id; herdr sees no agent in there (the
	// stranger's workspace has none), which is exactly the board that makes
	// RelaunchAgent re-type the persona command.
	newGeneration(t, sock)
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: m.Workspace, Label: "alpha"}})
	os.Remove(filepath.Join(fake, "agents.json"))
	os.Remove(filepath.Join(fake, "calls.log"))

	relaunched, err := b.RelaunchAgent("foo", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if relaunched {
		t.Error("relaunched a persona into a workspace that is not foo's")
	}
	if log := calls(t, fake); strings.Contains(log, "pane run "+m.Pane) {
		t.Errorf("typed a persona command into a stranger's pane:\n%s", log)
	}
}
