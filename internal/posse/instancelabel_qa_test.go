//go:build posse_arm3

package posse

// rangerhq-ouf9: `instance:` prefixes the herdr LABEL and nothing else.
//
// The bead's observable, in the shape a test can hold: two instances share
// one herdr server; the one that sets `instance:` no longer collides with
// the other over a name they both use, while every posse-internal name —
// the meta filename, what Sessions() and Resolve() answer to — stays
// exactly what it was. The two halves are one test each, because a fix that
// got only the first half would be a rename of the whole fleet.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// hermeticGen names a socket path that is not there, which makes
// ServerGen() unknown for the whole test. Two reasons, and both matter:
// the default path is the operator's own live herdr, so a test that let it
// through would read machine state and be red per-box; and an unknown
// generation is the board on which notOurWorkspace's LABEL arm is the only
// fence there is. A test run under a generation that matched would pass
// whatever the label said (the rename arm short-circuits it) and pin
// nothing.
func hermeticGen(t *testing.T) {
	t.Helper()
	t.Setenv("HERDR_SOCKET_PATH", filepath.Join(t.TempDir(), "herdr.sock"))
}

// setInstance writes just the key, which is all these tests configure.
func setInstance(t *testing.T, b *HerdrBackend, tag string) {
	t.Helper()
	if err := os.WriteFile(b.App.ConfigPath, []byte("instance: "+tag+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The prefix reaches herdr and stops there. Everything posse addresses the
// session by is the bare name, and the session created under a tag is still
// this home's own on the way back — the arm that matters most, because
// notOurWorkspace reads the label and would otherwise call every tagged
// session a stranger's the moment the key was set.
func TestInstanceTagPrefixesLabelOnlyNotSessionName(t *testing.T) {
	b, fake := newTestBackend(t)
	hermeticGen(t)
	setInstance(t, b, "work")

	dir := t.TempDir()
	mustCreate(t, b, NewSessionOpts{Name: "coordinator", Dir: dir})

	log := calls(t, fake)
	if !strings.Contains(log, "workspace create --label work/coordinator --no-focus") {
		t.Errorf("herdr was not asked for the instance-tagged label:\n%s", log)
	}

	// The ownership record is the meta dir, and its filename is the NAME.
	if _, err := os.Stat(b.metaPath("coordinator")); err != nil {
		t.Errorf("meta not written under the session name: %v", err)
	}
	if _, err := os.Stat(b.metaPath("work/coordinator")); err == nil {
		t.Error("the label reached the meta dir — names and labels must not be the same string")
	}
	m, ok := b.readMeta("coordinator")
	if !ok || m.Name != "coordinator" {
		t.Fatalf("bad meta: %+v (ok=%v)", m, ok)
	}

	sessions, err := b.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("want 1 session, got %+v", sessions)
	}
	if s := sessions[0]; s.Name != "coordinator" || s.Foreign {
		t.Errorf("a tagged session must list as this home's own, under its name: %+v", s)
	}
	if _, err := b.Resolve("coordinator"); err != nil {
		t.Errorf("Resolve(coordinator): %v", err)
	}
	if _, err := b.Resolve("work/coordinator"); err == nil {
		t.Error("the label resolved as a session name — identity is never parsed out of a label")
	}
}

// The collision the bead exists for, from the second instance's side: the
// other home's workspace is on this server under ITS tag, and this home's
// create of the same name goes through. The control below it is the guard
// this must not cost — a workspace wearing the label THIS home would write
// really is in the way, and still refuses.
//
// It used to be an UNTAGGED namesake there, asserted as designed on the
// rationale that a bare row "really might be a session somebody can address
// by that name". For a home with a tag set that rationale does not hold: a
// bare foreign row carries no meta here, this home's create is labelled
// <tag>/<name>, and afterwards Resolve prefers this home's own non-foreign
// row. It was a conservative refusal, not a necessary one, and its cost was
// that the second, TAGGED instance was the one that could not create
// (ranger-base-rcwx). The untagged home's bare namesake still refuses, and
// is pinned with the rest in instancecollide_qa_test.go.
func TestInstanceTagFreesAForeignNamesake(t *testing.T) {
	b, fake := newTestBackend(t)
	hermeticGen(t)
	setInstance(t, b, "fleet")

	// Another instance's session, as this home sees it: a workspace with no
	// meta here, labelled with a tag that is not ours.
	saveWSTo(t, fake, append(fakeLoadWSFrom(t, fake),
		fakeWS{WorkspaceID: "w9", Label: "work/coordinator"}))

	if b.HasSession("coordinator") {
		t.Fatal("another instance's tagged workspace answered for this home's name")
	}
	mustCreate(t, b, NewSessionOpts{Name: "coordinator", Dir: t.TempDir()})
	if !strings.Contains(calls(t, fake), "workspace create --label fleet/coordinator") {
		t.Error("the create did not go out under this home's tag")
	}

	// Both are listed, and the foreign row keeps its full label: a row an
	// operator cannot account for says which instance to go ask.
	sessions, err := b.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	var foreign, ours int
	for _, s := range sessions {
		switch {
		case s.Foreign && s.Name == "work/coordinator":
			foreign++
		case !s.Foreign && s.Name == "coordinator":
			ours++
		default:
			t.Errorf("unexpected row: %+v", s)
		}
	}
	if foreign != 1 || ours != 1 {
		t.Errorf("want one foreign row under its full label and one of ours, got %+v", sessions)
	}

	// The control: a row wearing the label this home WRITES is still in the
	// way. The tag frees names from other instances' labels, never from the
	// one string this home's own create would ask herdr for.
	saveWSTo(t, fake, append(fakeLoadWSFrom(t, fake),
		fakeWS{WorkspaceID: "w8", Label: "fleet/dispatch"}))
	if err := b.CreateSession(NewSessionOpts{Name: "dispatch", Dir: t.TempDir()}); err == nil {
		t.Error("a workspace already wearing this home's own label should refuse the create")
	}
}

// Unset is today's behaviour, byte for byte: the single-instance home that
// never touches the key sees no change at all.
func TestNoInstanceTagLeavesLabelsAlone(t *testing.T) {
	b, fake := newTestBackend(t)
	hermeticGen(t)
	mustCreate(t, b, NewSessionOpts{Name: "coordinator", Dir: t.TempDir()})
	if log := calls(t, fake); !strings.Contains(log, "workspace create --label coordinator --no-focus") {
		t.Errorf("an untagged home must label a workspace with the bare name:\n%s", log)
	}
	if got := b.App.WorkspaceLabel("coordinator"); got != "coordinator" {
		t.Errorf("WorkspaceLabel with no tag = %q, want %q", got, "coordinator")
	}
}

// Turning the key on does not relabel the workspaces already running, so a
// meta written before it must still read as this home's own. Without this
// arm the first pass after the edit calls the entire live fleet strangers
// and drops it out of the listing — the failure notOurWorkspace's
// positive-evidence rule exists to prevent, arriving through the config.
func TestInstanceTagKeepsPreTagSessionsOurs(t *testing.T) {
	b, fake := newTestBackend(t)
	hermeticGen(t)
	mustCreate(t, b, NewSessionOpts{Name: "coordinator", Dir: t.TempDir()})

	setInstance(t, b, "work")

	sessions, err := b.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || sessions[0].Foreign || sessions[0].Name != "coordinator" {
		t.Fatalf("a session created before the tag was set must stay ours: %+v", sessions)
	}
	if _, err := os.Stat(b.metaPath("coordinator")); err != nil {
		t.Errorf("its meta was pruned: %v", err)
	}
	_ = fake
}

// A tag posse cannot render is refused at the plan, before anything is
// created — ignoring it would put this home's sessions back under the bare
// labels the key was set to move them off, on the one server another
// instance is watching. The separator is refused with the rest: a tag
// containing it makes the split ambiguous for the only reader a label has.
func TestBadInstanceTagRefusesTheLaunch(t *testing.T) {
	for _, tag := range []string{"work/two", "two homes", "-x", "work:one"} {
		t.Run(tag, func(t *testing.T) {
			b, fake := newTestBackend(t)
			hermeticGen(t)
			setInstance(t, b, tag)

			err := b.CreateSession(NewSessionOpts{Name: "coordinator", Dir: t.TempDir()})
			if err == nil {
				t.Fatal("a malformed instance tag should refuse the launch")
			}
			if !strings.Contains(err.Error(), "instance") {
				t.Errorf("the refusal must name the key: %v", err)
			}
			if log := calls(t, fake); strings.Contains(log, "workspace create") {
				t.Errorf("a workspace was created under a tag posse refused:\n%s", log)
			}
			if _, err := os.Stat(b.metaPath("coordinator")); err == nil {
				t.Error("a refused launch left a meta behind")
			}
			if _, err := os.Stat(filepath.Join(fake, "ws.json")); err == nil {
				t.Error("a refused launch reached herdr")
			}
		})
	}
}

// Relaunch's one obstacle that lives in herdr rather than in the plan: a
// workspace already wearing the name will still be wearing it after the
// kill. "Wearing it" is this home's rendering of the name (rangerhq-ouf9) —
// another instance's row is not in the way of a label only this home
// writes, and refusing over it would block a relaunch nothing obstructs,
// with the operator sent to rename a workspace that is not theirs to touch.
func TestRelaunchIsNotBlockedByAnotherInstancesNamesake(t *testing.T) {
	b, fake := newTestBackend(t)
	hermeticGen(t)
	setInstance(t, b, "fleet")
	agentPerLaunch(t, fake)
	devSession(t, b, "s1")

	// The other instance's session, same name, its own tag.
	saveWSTo(t, fake, append(fakeLoadWSFrom(t, fake),
		fakeWS{WorkspaceID: "w9", Label: "work/s1"}))

	var out strings.Builder
	if err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"}); err != nil {
		t.Fatalf("relaunch refused over another instance's namesake: %v\n%s", err, out.String())
	}

	// The control: a row wearing the label THIS home writes really would
	// take the name, so it still refuses — and names the workspace in the
	// way, which is the whole value of the refusal.
	saveWSTo(t, fake, append(fakeLoadWSFrom(t, fake),
		fakeWS{WorkspaceID: "w8", Label: "fleet/s1"}))
	out.Reset()
	err := b.RelaunchSession(&out, RelaunchOpts{Name: "s1"})
	if err == nil {
		t.Fatal("a workspace wearing this home's own label must still refuse the relaunch")
	}
	if !strings.Contains(err.Error(), "w8") || !strings.Contains(err.Error(), "was NOT closed") {
		t.Errorf("the refusal must name the workspace in the way, before the kill: %v", err)
	}
}
