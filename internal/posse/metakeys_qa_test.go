//go:build !posse_arm2 && !posse_arm3

package posse

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// ranger-base-1go: the herdr-upgrade runbook's post-flight meta check.
//
// The herdr-upgrade runbook (instance tree, docs/runbooks/herdr-upgrade.md;
// public until ranger-base-vbcs moved it out of NOTES.md per ADR 0024) tells the
// operator to compare `^workspace:` + `^socket:` across the handoff and says
// plainly that `gen:` is EXPECTED to be rewritten on every meta by the first
// post-handoff listing, so a byte-for-byte `diff -r` of the meta directory is
// no longer the check. Both halves of that are properties of this package, and
// the runbook went stale once already (ranger-base-zr2) because a fact about
// the binary was written down instead of pinned. This pins it:
//
//   - the comparand the runbook diffs must be byte-stable across a generation
//     change, or the post-flight falsely fails and an operator mid-window
//     diagnoses damage that does not exist;
//   - `gen:` must actually move, or the runbook is telling them to ignore a
//     real signal.
//
// If either assertion flips, that runbook's post-flight needs re-deriving.

// qaMetaKeys is the runbook's comparand, computed the way the runbook computes
// it: the identity lines of every meta, sorted. `pane:` is included here and
// deliberately NOT in the runbook — see TestMetaPaneIsAsStableAsTheComparand.
func qaMetaKeys(t *testing.T, dir string, keys ...string) string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read meta dir: %v", err)
	}
	var out []string
	for _, e := range ents {
		if !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		for _, ln := range strings.Split(string(b), "\n") {
			for _, k := range keys {
				if strings.HasPrefix(ln, k+":") {
					out = append(out, e.Name()+":"+ln)
				}
			}
		}
	}
	if len(out) == 0 {
		t.Fatalf("no %v lines under %s — the check would pass vacuously", keys, dir)
	}
	sort.Strings(out)
	return strings.Join(out, "\n")
}

func qaGenLines(t *testing.T, dir string) string {
	t.Helper()
	return qaMetaKeys(t, dir, "gen")
}

// qaHandoffBoard stands up two live sessions on a concrete socket, lists them
// once so the first generation is stamped, then recreates the socket FILE at
// the same path — new inode, same path, which is exactly what a herdr live
// handoff does to the api socket (rangerhq-6bg7) — and lists again.
func qaHandoffBoard(t *testing.T) (dir, before, beforeGen string, b *HerdrBackend) {
	t.Helper()
	b, fake := newTestBackend(t)

	sock := filepath.Join(t.TempDir(), "herdr.sock")
	if err := os.WriteFile(sock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERDR_SOCKET_PATH", sock)

	mustCreate(t, b, NewSessionOpts{Name: "alpha"})
	mustCreate(t, b, NewSessionOpts{Name: "beta"})
	saveWSTo(t, fake, []fakeWS{
		{WorkspaceID: "w1", Label: "alpha"},
		{WorkspaceID: "w2", Label: "beta"},
	})

	dir = b.metaDir()
	if _, err := b.Sessions(); err != nil { // generation 1 stamped
		t.Fatalf("Sessions (gen 1): %v", err)
	}
	before = qaMetaKeys(t, dir, "workspace", "socket")
	beforeGen = qaGenLines(t, dir)
	if !strings.Contains(beforeGen, "gen:") {
		t.Fatalf("no gen: stamped on the first listing; the fence is not armed in this fixture:\n%s", beforeGen)
	}

	// the handoff: same socket path, new inode.
	if err := os.Remove(sock); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Sessions(); err != nil { // generation 2 stamped
		t.Fatalf("Sessions (gen 2): %v", err)
	}
	return dir, before, beforeGen, b
}

// The runbook's premise, both halves at once.
func TestGenMovesAcrossAHandoffAndTheComparandDoesNot(t *testing.T) {
	dir, before, beforeGen, _ := qaHandoffBoard(t)

	if got := qaGenLines(t, dir); got == beforeGen {
		t.Errorf("gen: did not move across a new socket inode — the runbook tells the\n"+
			"operator to expect it to, and a post-flight that ignores a field which\n"+
			"never changes is ignoring a real signal:\n%s", got)
	}
	if got := qaMetaKeys(t, dir, "workspace", "socket"); got != before {
		t.Errorf("the runbook's post-flight comparand moved across a handoff.\n"+
			"The runbook tells the operator a difference here means ids moved and to run\n"+
			"the repair. Re-derive that check.\nbefore:\n%s\nafter:\n%s", before, got)
	}
}

// The premise behind "a byte-for-byte diff -r falsely fails": it must actually
// fail on a board where nothing but the generation moved. This is the defect
// ranger-base-zr2 was filed for; it is pinned so the runbook's justification
// cannot quietly stop being true.
func TestByteForByteMetaDiffFalselyFailsOnAGenOnlyRewrite(t *testing.T) {
	b, fake := newTestBackend(t)
	sock := filepath.Join(t.TempDir(), "herdr.sock")
	os.WriteFile(sock, nil, 0o600)
	t.Setenv("HERDR_SOCKET_PATH", sock)

	mustCreate(t, b, NewSessionOpts{Name: "alpha"})
	saveWSTo(t, fake, []fakeWS{{WorkspaceID: "w1", Label: "alpha"}})
	dir := b.metaDir()
	b.Sessions()

	raw, err := os.ReadFile(filepath.Join(dir, "alpha.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	os.Remove(sock)
	os.WriteFile(sock, nil, 0o600)
	b.Sessions()

	after, err := os.ReadFile(filepath.Join(dir, "alpha.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) == string(raw) {
		t.Fatalf("the meta file is byte-identical after a generation change, so\n" +
			"`diff -r` would NOT falsely fail and the runbook's reason for dropping\n" +
			"it no longer holds. Re-read the runbook's post-flight.")
	}
}

// `pane:` is the id every prompting path addresses (PaneRun, AgentTarget), and
// it is as stable across a generation change as the two fields the runbook does
// compare — so leaving it out of the comparand costs nothing and buys nothing.
// Pinned so that if some future write path starts moving pane: on a listing,
// this fails and the runbook's comparand gets revisited rather than silently
// going blind to it. Recorded on ranger-base-1go.
func TestMetaPaneIsAsStableAsTheComparand(t *testing.T) {
	dir, _, _, _ := qaHandoffBoard(t)
	// captured before the second listing would be ideal, but the first listing
	// is where a stamp could move it, so re-derive from the same board: pane
	// must still name the workspace each meta records.
	got := qaMetaKeys(t, dir, "workspace", "pane")
	for _, ln := range strings.Split(got, "\n") {
		if !strings.Contains(ln, ":pane: ") {
			continue
		}
		file := strings.SplitN(ln, ":", 2)[0]
		pane := strings.TrimPrefix(strings.SplitN(ln, ":pane: ", 2)[1], "")
		wsWant := ""
		for _, l2 := range strings.Split(got, "\n") {
			if strings.HasPrefix(l2, file+":workspace: ") {
				wsWant = strings.TrimPrefix(l2, file+":workspace: ")
			}
		}
		if wsWant == "" || !strings.HasPrefix(pane, wsWant+":") {
			t.Errorf("%s: pane %q does not name workspace %q — a listing moved pane:\n"+
				"independently of workspace:, which the runbook's post-flight comparand\n"+
				"(workspace: + socket:) cannot see. Revisit the runbook.", file, pane, wsWant)
		}
	}
}
