package posse

// ranger-base-82e40: the FOURTH door onto one double-seating, and the only
// one that is about the file rather than about the listing.
//
// ranger-base-5kiu4 established the rule — a session this listing cannot
// answer for keeps its seat — and ranger-base-3yqyg extended it to a listing
// that could not be read at all. Both are about herdr. This one is about the
// meta: writeMeta TRUNCATED the file and then wrote it, and every reader of
// these files is another process holding no lock, so a live session's record
// could be read while it was empty. An empty record has no workspace, and a
// meta with no workspace is a `recipe` — the one classification listSessions
// deliberately does NOT withhold, because a recipe is a session already gone.
// The seat then read free and a fresh Run seated a second bead into the live
// session.
//
// Two halves, and they are independent: the write is made atomic so the state
// cannot be produced, and the read is taught to tell "no workspace recorded"
// apart from "read nothing at all" so it is not misclassified if it is. The
// second half is what the tests below can measure end to end; the first is
// pinned on the file itself, because a race that has to be lost to be seen is
// not a test.

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tornMeta plants the state a reader used to be able to see inside
// writeMeta's truncate window: the file is there and carries nothing.
func tornMeta(t *testing.T, b *HerdrBackend, name string) {
	t.Helper()
	if err := os.MkdirAll(b.metaDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b.metaPath(name), nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

// The writer half. os.WriteFile truncates the target and writes into it, so
// the window exists for as long as the write takes; a rename gives every
// reader either the old record or the new one.
//
// Asserted on the file identity and on a reader that holds the old one open,
// which is the property itself and is deterministic — a hammer of concurrent
// readers proves nothing on the pass it does not lose.
//
// MUTATION: put os.WriteFile back → SameFile, and the held reader sees the
// SECOND record through a fd it opened on the first.
func TestWriteMetaReplacesTheMetaRatherThanTruncatingIt(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	m := &HerdrMeta{Name: "s1", Workspace: "w1", Pane: "w1:p1", Emoji: "x"}
	if err := b.writeMeta(m); err != nil {
		t.Fatal(err)
	}
	p := b.metaPath("s1")
	before, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	held, err := os.Open(p) // a reader that opened the record before the write
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()

	m.Bead = "a-2" // an ordinary pass rewriting a live session's meta
	if err := b.writeMeta(m); err != nil {
		t.Fatal(err)
	}

	after, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(before, after) {
		t.Errorf("the meta was rewritten IN PLACE: the path still names the same file, so the bytes between the truncate and the write are what a cross-process reader gets — and a record with no workspace is read as a recipe, which frees the seat (ranger-base-82e40)")
	}
	got, err := io.ReadAll(held)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "bead: a-2") {
		t.Errorf("a reader holding the old record was given the new one under it:\n%s", got)
	}
	if !strings.Contains(string(got), "workspace: w1") {
		t.Errorf("a reader holding the old record lost it: the whole point of the rename is that this fd keeps a COMPLETE record:\n%q", got)
	}
	if fi, err := os.Stat(p); err != nil || fi.Mode().Perm() != 0o644 {
		t.Errorf("mode = %v (err %v), want 0644 — the record is world-readable exactly as it was before the rename landed", fi.Mode().Perm(), err)
	}
}

// The temp file is made in the meta dir, which is also the session namespace:
// os.ReadDir over it is where every session name comes from. A temp named
// like a meta would be a session with a record nobody wrote.
//
// Two assertions, because the ordinary path hides the risk: after a rename
// there is no temp left to be seen, so a write alone cannot tell a safe
// pattern from a dangerous one (MEASURED: a `.meta-*.yaml` mutant passed the
// litter check). What is asserted instead is the litter ITSELF — the file a
// kill between the create and the rename leaves behind, planted from the
// pattern the writer uses — because that one does not clean itself up, and a
// phantom name in this dir outlives the crash that made it.
//
// MUTATION: give metaTempPattern a `.yaml` suffix → the leftover is a session
// name with no record in it, which listSessions withholds forever and whose
// seat personaActive then holds forever.
func TestWriteMetaLeavesNothingThatReadsAsASession(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	for _, n := range []string{"s1", "s2"} {
		if err := b.writeMeta(&HerdrMeta{Name: n, Workspace: "w-" + n}); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.writeMeta(&HerdrMeta{Name: "s1", Workspace: "w-s1", Bead: "a-1"}); err != nil {
		t.Fatal(err)
	}
	if got, err := b.metaNames(); err != nil || len(got) != 2 || got[0] != "s1" || got[1] != "s2" {
		t.Errorf("metaNames() = %v, %v, want [s1 s2], <nil>", got, err)
	}
	ents, err := os.ReadDir(b.metaDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 2 {
		var names []string
		for _, e := range ents {
			names = append(names, e.Name())
		}
		t.Errorf("the meta dir holds %v after two renames: the ordinary path leaves no temp behind", names)
	}

	// The crash shape: a temp with the random part filled in, exactly as
	// os.CreateTemp would have named it, and nothing to remove it.
	litter := strings.Replace(metaTempPattern, "*", "3141592653", 1)
	if litter == metaTempPattern {
		t.Fatalf("metaTempPattern %q has no `*` for os.CreateTemp to fill", metaTempPattern)
	}
	if err := os.WriteFile(filepath.Join(b.metaDir(), litter), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := b.metaNames(); err != nil || len(got) != 2 {
		t.Errorf("metaNames() = %v (err %v) after a half-written meta was left behind: a temp file must never be readable as a session name — nothing later removes it, and every name in this dir is a seat", got, err)
	}
}

// The reader half, at the bottom: a file with no record in it is not a record
// with no workspace. Both used to be a *HerdrMeta with every field empty.
//
// MUTATION: drop the `name` probe from readMeta (Unreadable always false) →
// red on the torn arm. Read it from the argument instead of the file → red
// too, because the argument is never empty.
func TestReadMetaSaysWhenTheFileCarriesNoRecord(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	// A genuine recipe: the record IS there and names no workspace, which is
	// what `posse relaunch` keeps (rangerhq-v52t). It must stay readable, or
	// the fix trades the double-seating for an unrelaunchable session.
	if err := b.writeMeta(&HerdrMeta{Name: "recipe", Emoji: "x", Agent: "developer"}); err != nil {
		t.Fatal(err)
	}
	m, ok := b.readMeta("recipe")
	if !ok || m.Unreadable {
		t.Errorf("a recipe read as unreadable: ok=%v Unreadable=%v — a record naming no workspace is still a record", ok, m.Unreadable)
	}
	if m.Workspace != "" || m.Agent != "developer" {
		t.Errorf("recipe read back wrong: workspace=%q agent=%q", m.Workspace, m.Agent)
	}

	tornMeta(t, b, "torn")
	m, ok = b.readMeta("torn")
	if !ok {
		t.Fatalf("the file is there, so readMeta's bool must stay true: it answers 'is there a meta for this name', which nameFree and mustNotOrphan read as 'this home holds no such session'")
	}
	if !m.Unreadable {
		t.Errorf("a file carrying no record read as a record: Unreadable=false, so `no workspace` is all a caller sees and a live session's seat reads free (ranger-base-82e40)")
	}

	if _, ok := b.readMeta("never-written"); ok {
		t.Errorf("readMeta invented a meta for a name with no file")
	}
}

// listSessions, end to end: the classification the whole bug turns on.
//
// The meta is one the listing would otherwise answer for in full — its
// workspace is on the board and its socket is this pass's — so nothing here
// is about herdr. Emptying the file is the only change.
//
// MUTATION: drop the Unreadable arm from listSessions → the name is reported
// as a `recipe`, is absent from both the rows and the withheld list, and the
// next assertion in TestPersonaActive... below fires.
func TestListSessionsWithholdsAMetaItCannotRead(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", "/tmp/82e40/ours.sock")
	b, fake := newTestBackend(t)
	// The label has to be the one this home would have written for s1, or
	// the meta lands on the `strangers` arm — a real abstention, and not the
	// one being measured (found while writing this).
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: b.App.WorkspaceLabel("s1")}})
	if err := b.writeMeta(&HerdrMeta{Name: "s1", Workspace: "w1", Pane: "w1:p1", Emoji: "x", Socket: SocketID(), Gen: ServerGen()}); err != nil {
		t.Fatal(err)
	}
	rows, withheld, err := b.listSessions()
	if err != nil || len(rows) != 1 || rows[0].Name != "s1" || len(withheld) != 0 {
		t.Fatalf("premise: intact, this meta is LISTED and nothing is withheld: rows=%v withheld=%v err=%v", rows, withheld, err)
	}

	tornMeta(t, b, "s1")
	w := warnBuf(t, b)
	rows, withheld, err = b.listSessions()
	if err != nil {
		t.Fatalf("a meta that cannot be read is not a listing error: err=%v", err)
	}
	for _, r := range rows {
		if r.Name == "s1" && !r.Foreign {
			t.Fatalf("premise: an unreadable meta cannot be listed as OURS — there is no workspace id in it to list the session over")
		}
	}
	if len(withheld) != 1 || withheld[0] != "s1" {
		t.Errorf("withheld = %v, want [s1]: absent from the rows and absent from the withheld list is what a caller reads as DEAD, and the session may be perfectly alive (ranger-base-82e40)", withheld)
	}
	if strings.Contains(w.String(), "relaunch") {
		t.Errorf("an unreadable meta was reported as a recipe — a positive claim that the session is gone:\n%s", w.String())
	}
	if !strings.Contains(w.String(), "carries no record") {
		t.Errorf("nothing said why the meta was withheld; an operator cannot repair what the listing will not name:\n%s", w.String())
	}
	// The workspace itself is still shown, unclaimed: the id is left
	// unclaimed by a withheld meta exactly as it is by a spared one, so the
	// pane does not vanish from the board while its record is unreadable.
	// personaActive skips a foreign row (ranger-base-p6no), which is why
	// this row is not what holds the seat below.
	foreign := false
	for _, r := range rows {
		if r.WorkspaceID == "w1" && r.Foreign {
			foreign = true
		}
	}
	if !foreign {
		t.Errorf("the workspace disappeared from the listing with its meta: rows=%v", rows)
	}
	if _, err := os.Stat(b.metaPath("s1")); err != nil {
		t.Errorf("the meta was DELETED: a file this pass could not read is the last one to prune on, and every other withheld arm keeps it (%v)", err)
	}
}

// The seat, which is the consequence the bead was filed for, in the reported
// shape: the probe below is the reporter's, with the assertion inverted to what the
// rule says it should have printed.
//
// MUTATION: drop the Unreadable arm from personaActive → the torn arm reports
// ("", ""), the answer a genuinely idle persona gives, and a fresh Run fires
// a second bead into the live session.
func TestPersonaActiveHoldsASeatWhoseMetaCannotBeRead(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", "/tmp/82e40/ours.sock")
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: "live"}})
	dir := "/src/posse"
	slot := SessionFor("developer", dir)
	session := slot + "-a-1"

	qaWithheldMeta(t, b, session, "", false)
	if name, st := d.personaActive("developer", dir); name != session || st != seatUnlisted {
		t.Fatalf("premise: intact, this seat is held: personaActive = %q %q, want %q %q", name, st, session, seatUnlisted)
	}

	tornMeta(t, b, session)
	name, st := d.personaActive("developer", dir)
	if name != session {
		t.Errorf("personaActive = %q %q, want %q held: a meta this pass cannot read is a session it cannot show idle, and reporting the seat FREE is how a second bead gets seated into a live one (ranger-base-82e40)", name, st, session)
	}
	if st != seatUnlisted {
		t.Errorf("status = %q, want %q", st, seatUnlisted)
	}

	// The control, from ranger-base-5kiu4: the abstention is per-seat. One
	// unreadable meta must not freeze the shop.
	if name, st := d.personaActive("hopper", dir); name != "" {
		t.Errorf("an unreadable meta in one seat froze another: personaActive(hopper) = %q %q, want free", name, st)
	}
}
