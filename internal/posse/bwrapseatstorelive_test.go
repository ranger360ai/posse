//go:build !posse_arm2 && !posse_arm3

package posse

// Live pins for ranger-base-21i98 (ADR 0099 D1, D3-D5): a committing seat's
// `<common>/objects` is its OWN store, bound over that name by the bwrap
// render, with the shared store read-only at a side path. Run against a real
// bwrap on a real linked worktree, rendered by the shipped producers — the
// writable set from SeatbeltWritable through BwrapWritable, the store binds
// from seatStoreBinds, the store itself from PrepareSessionHead's walled arm —
// so a drift between the design and the render reds here.
//
//	go test ./internal/posse -run TestLiveBwrapSeatStore -v
//
// Gated like its neighbours in bwrapgitrefslive_test.go (bgLiveSkip): Linux,
// bwrap and git, and not from inside a seat (ADR 0096 D1).
//
// EVERY REFUSAL IS READ BACK FROM OUTSIDE. A wall row that trusts the exit
// code measures the shell; the shared store's file, its mode and what the
// operator's `cat-file` serves are what the hole was about.

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// bsLive is a walled session tree: a real posse session tree (so the landing
// half runs MergeSessionWork), prepared by the walled launch's own call, and
// the argv the render gives it — with and without the store binds.
type bsLive struct {
	a          *App
	repo       string
	tr         *SessionTree
	own        string
	common     string
	store      string
	base       string
	set        []string
	keep       []string
	argv       []string // the shipped render: the store binds in place
	argvBefore []string // the render this bead replaced: no store binds
}

func bsLiveFixture(t *testing.T) bsLive {
	t.Helper()
	bgLiveSkip(t)
	f := bsLive{a: wtApp(t), repo: wtRepo(t)}
	tr, err := f.a.EnsureSessionTree(f.repo, "s-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	f.tr = tr
	if err := PrepareSessionHead(tr, true, false, io.Discard); err != nil {
		t.Fatalf("the walled prepare refused the fixture's tree: %v", err)
	}
	dirs := LinkedGitDirs(tr.Path)
	if len(dirs) != 2 {
		t.Fatalf("no linked worktree: %v", dirs)
	}
	f.own, f.common = absResolve(dirs[0]), absResolve(dirs[1])
	l := seatStoreFrom(dirs[0], dirs[1])
	f.store, f.base = l.Store, l.Base
	if b, err := os.ReadFile(filepath.Join(f.store, "info", "alternates")); err != nil || strings.TrimSpace(string(b)) != f.base {
		t.Fatalf("the walled prepare did not leave a store whose alternates name the side path %s: %q %v", f.base, b, err)
	}
	ag := &AgentFile{Name: "bs-live"}
	f.set, f.keep = BwrapWritable(ag, bgLiveSessionGrants(t, f.repo, tr.Path, ag), tr.Path)
	if !wantsPath(f.set, filepath.Join(f.common, "objects")) {
		t.Fatalf("CONTROL: the writable set does not grant <common>/objects, so the store bind has nothing to cover: %v", f.set)
	}
	f.argv = f.render(t, f.set, seatStoreBinds(ag, tr.Path))
	if len(seatStoreBinds(ag, tr.Path)) != 2 {
		t.Fatal("CONTROL: seatStoreBinds rendered no store for a committing PID in a linked worktree")
	}
	f.argvBefore = f.render(t, f.set, nil)
	return f
}

// render is BwrapArgs over the set with these private binds, plus the
// launcher's half of the prep, run in the list's order.
func (f bsLive) render(t *testing.T, set []string, private []PrivateBind) []string {
	t.Helper()
	argv, prep, err := BwrapArgs(f.tr.Path, set, SeatbeltCarveOut{Private: private}, f.keep...)
	if err != nil {
		t.Fatalf("BwrapArgs refused the seat store render: %v", err)
	}
	for _, s := range prep {
		switch s.Kind {
		case PrepMkdirAll:
			if err := os.MkdirAll(s.Path, 0o755); err != nil {
				t.Fatalf("prep %s: %v", s, err)
			}
		case PrepTouch:
			fh, err := os.OpenFile(s.Path, os.O_CREATE|os.O_WRONLY, 0o644)
			if err != nil {
				t.Fatalf("prep %s: %v", s, err)
			}
			fh.Close()
		default:
			t.Fatalf("the render asked for a prep step this test cannot run: %s", s)
		}
	}
	return argv
}

func (f bsLive) in(t *testing.T, argv []string, stdin string, arg ...string) (bool, string) {
	t.Helper()
	cmd := exec.Command("bwrap", append(append([]string{}, argv...), arg...)...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	out, err := cmd.CombinedOutput()
	return err == nil, strings.TrimSpace(string(out))
}

func (f bsLive) git(arg ...string) []string {
	return append([]string{"git", "-c", "user.name=p", "-c", "user.email=p@example.com", "-C", f.tr.Path}, arg...)
}

// sharedObject writes an object into the SHARED store from outside, as
// history already holds it, and returns its name and its file.
func (f bsLive) sharedObject(t *testing.T, body string) (string, string) {
	t.Helper()
	cmd := exec.Command("git", "-C", f.repo, "hash-object", "-w", "--stdin")
	cmd.Stdin = strings.NewReader(body)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	sha := strings.TrimSpace(string(out))
	p := filepath.Join(f.common, "objects", sha[:2], sha[2:])
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("CONTROL: %s is not a loose object in the shared store: %v", sha, err)
	}
	return sha, p
}

// Pin (1), the design's live row and the sign-off's (1): under the shipped render a
// committing seat creates new objects and commits, and every way of changing
// an EXISTING shared object's file — unlink, chmod, rename over it, rewrite
// it in place — is refused, by its real name and by the side path. The same
// holds when another grant covers `<common>` WHOLE (a store of record inside
// the same repo), which is what says the store's bind is the deepest mount.
//
// The control is the render this bead replaced, against a second object: the
// same unlink succeeds there, so the arms measure the binds and not the file
// mode (a loose object is 0444, which stops an in-place write and nothing
// else).
//
// MUTATION RUN: dropping the store bind from seatStoreBinds reds every arm
// (the seat writes the shared store again); dropping the side bind reds the
// read-through arm and the commit (the alternates name an empty dir).
func TestLiveBwrapSeatStoreRefusesRewritingASharedObject(t *testing.T) {
	t.Parallel()
	f := bsLiveFixture(t)
	honest := "the honest history\n"
	sha, file := f.sharedObject(t, honest)
	ctlSha, ctlFile := f.sharedObject(t, "the control's object\n")

	if ok, out := f.in(t, f.argvBefore, "", "rm", "-f", ctlFile); !ok {
		t.Fatalf("CONTROL: the pre-store render refused unlinking %s, so the arms below cannot be told from the file's own mode:\n%s", ctlSha[:12], out)
	}
	if _, err := os.Stat(ctlFile); err == nil {
		t.Fatal("CONTROL: the pre-store render's unlink exited 0 and the shared object is still there")
	}

	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	fi0, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	side := filepath.Join(f.base, sha[:2], sha[2:])

	for _, shape := range []struct {
		name string
		argv []string
	}{
		{"as rendered", f.argv},
		{"with <common> granted whole", f.render(t, append(append([]string{}, f.set...), f.common), seatStoreBinds(&AgentFile{Name: "bs-live"}, f.tr.Path))},
	} {
		for _, p := range []string{file, side} {
			for _, c := range []struct {
				name string
				arg  []string
			}{
				{"unlink", []string{"rm", "--", p}},
				{"chmod", []string{"chmod", "0644", p}},
				{"rename over", []string{"sh", "-c", "printf evil > " + p + ".new && mv -f " + p + ".new " + p}},
				{"rewrite in place", []string{"sh", "-c", "chmod u+w " + p + " && printf evil > " + p}},
			} {
				if ok, out := f.in(t, shape.argv, "", c.arg...); ok {
					t.Errorf("%s: a committing seat could %s %s:\n%s", shape.name, c.name, p, out)
				}
			}
		}
		if ok, out := f.in(t, shape.argv, "", f.git("cat-file", "-p", sha)...); !ok || out+"\n" != honest {
			t.Errorf("%s: the seat cannot read history through its store's alternates (ok=%v): %q", shape.name, ok, out)
		}
		ok, out := f.in(t, shape.argv, "a new object "+shape.name+"\n", f.git("hash-object", "-w", "--stdin")...)
		if !ok {
			t.Fatalf("%s: a committing seat cannot write a NEW object:\n%s", shape.name, out)
		}
		if _, err := os.Stat(filepath.Join(f.store, out[:2], out[2:])); err != nil {
			t.Errorf("%s: the new object %s is not in the seat's own store: %v", shape.name, out[:12], err)
		}
		if _, err := os.Stat(filepath.Join(f.common, "objects", out[:2], out[2:])); err == nil {
			t.Errorf("%s: the new object %s was written into the SHARED store", shape.name, out[:12])
		}
		if ok, out := f.in(t, shape.argv, "", f.git("commit", "-q", "--allow-empty", "-m", "21i98 "+shape.name)...); !ok {
			t.Errorf("%s: a committing seat cannot commit under its store:\n%s", shape.name, out)
		}
	}

	after, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("the shared object %s is gone: %v", sha[:12], err)
	}
	if string(after) != string(before) {
		t.Errorf("the shared object %s's file changed under the wall", sha[:12])
	}
	if fi, err := os.Stat(file); err != nil || fi.Mode() != fi0.Mode() {
		t.Errorf("the shared object %s's mode changed under the wall: %v -> %v (%v)", sha[:12], fi0.Mode(), fi.Mode(), err)
	}
	if got := mustGit(t, f.repo, "cat-file", "-p", sha); got+"\n" != honest {
		t.Errorf("the operator's cat-file serves %q for %s", got, sha[:12])
	}
}

// Pin (3): end to end through the real wall and the real landing. An honest
// seat's commit lands on the base unchanged; a seat that rewrites one of ITS
// OWN objects after committing (which its store lets it) lands nothing: the
// name it claimed stays absent from the shared store, main does not move, and
// the tree is kept.
//
// MUTATION RUN: the import copying the seat's loose files straight into the
// shared store (no re-hash) reds the tampered arm; dropping the store bind
// reds the honest arm's control (the commit is in the shared store at once).
func TestLiveBwrapSeatStoreLandsHonestWorkAndNeverATamperedObject(t *testing.T) {
	t.Parallel()
	t.Run("honest", func(t *testing.T) {
		t.Parallel()
		f := bsLiveFixture(t)
		ok, out := f.in(t, f.argv, "", "sh", "-c",
			"printf 'the work\\n' > fix.txt && "+strings.Join(f.git("add", "fix.txt"), " ")+" && "+strings.Join(f.git("commit", "-q", "-m", "s-1:fix"), " "))
		if !ok {
			t.Fatalf("the seat could not commit under its store:\n%s", out)
		}
		head := mustGit(t, f.tr.Path, "rev-parse", "HEAD")
		if siHas(t, f.repo, head) {
			t.Fatal("CONTROL: the seat's commit is in the shared store before any import, so the store bind did not take")
		}
		o, err := MergeSessionWork(f.a, f.tr)
		if err != nil {
			t.Fatal(err)
		}
		if !o.Merged {
			t.Fatalf("an honest walled seat's work did not land: %+v", o)
		}
		if got := mustGit(t, f.repo, "rev-parse", "main"); got != head {
			t.Fatalf("main is at %s, not the seat's commit %s", got[:12], head[:12])
		}
		if got := mustGit(t, f.repo, "show", "main:fix.txt"); got != "the work" {
			t.Fatalf("main:fix.txt reads %q", got)
		}
	})
	t.Run("tampered", func(t *testing.T) {
		t.Parallel()
		f := bsLiveFixture(t)
		ok, out := f.in(t, f.argv, "", "sh", "-c",
			"printf 'the claimed bytes\\n' > fix.txt && "+strings.Join(f.git("add", "fix.txt"), " ")+" && "+strings.Join(f.git("commit", "-q", "-m", "s-1:fix"), " "))
		if !ok {
			t.Fatalf("the seat could not commit under its store:\n%s", out)
		}
		// Read inside the wall: outside, before the import, the tip's
		// objects are not there to read (ADR 0099 D6).
		ok, claimed := f.in(t, f.argv, "", f.git("rev-parse", "HEAD:fix.txt")...)
		if !ok {
			t.Fatalf("the seat cannot read its own commit:\n%s", claimed)
		}
		ok, forged := f.in(t, f.argv, "the forged bytes\n", f.git("hash-object", "-w", "--stdin")...)
		if !ok {
			t.Fatalf("the seat could not write the forged object:\n%s", forged)
		}
		from := filepath.Join(f.store, forged[:2], forged[2:])
		to := filepath.Join(f.store, claimed[:2], claimed[2:])
		if ok, out := f.in(t, f.argv, "", "mv", "-f", from, to); !ok {
			t.Fatalf("CONTROL: the seat could not rewrite an object in its OWN store, so this is not the tamper the landing is for:\n%s", out)
		}
		before := mustGit(t, f.repo, "rev-parse", "main")
		o, err := MergeSessionWork(f.a, f.tr)
		if err != nil {
			t.Fatal(err)
		}
		if o.Merged {
			t.Fatalf("a tip naming a tampered object landed: %+v", o)
		}
		if got := mustGit(t, f.repo, "rev-parse", "main"); got != before {
			t.Fatalf("main moved to %s over a refused landing", got[:12])
		}
		// MEASURED 2026-11-15 (git 2.56.0, linux 7.2.15): the tree that
		// names the claimed blob travels in the same pack, so `index-pack
		// --strict` refuses the WHOLE pack ("did not receive expected
		// object") and nothing of the store lands — not the forged bytes
		// under their true name either. xcucy's predicted-name pin is the
		// shape where no tree names the plant and the true name does land.
		if !strings.Contains(o.Reason, "could not be imported") {
			t.Errorf("the refusal is not the import's: %s", o.Reason)
		}
		for name, sha := range map[string]string{"the name the seat claimed": claimed, "the forged bytes' true name": forged} {
			if siHas(t, f.repo, sha) {
				t.Errorf("the shared store holds %s, %s, from a store whose import was refused", sha[:12], name)
			}
		}
	})
}

// Pin (4): an operator `gc` succeeds right after a watch-pass import while a
// walled seat holds a staged blob only its store has — and fails before it,
// which is the control that says the blob really was in the seat's store.
func TestLiveBwrapSeatStoreOperatorGcAfterTheWatchPassImport(t *testing.T) {
	t.Parallel()
	f := bsLiveFixture(t)
	ok, out := f.in(t, f.argv, "", "sh", "-c",
		"printf 'staged, never committed\\n' > staged.txt && "+strings.Join(f.git("add", "staged.txt"), " "))
	if !ok {
		t.Fatalf("the seat could not stage under its store:\n%s", out)
	}
	if _, err := git(f.repo, "gc", "-q"); err == nil {
		t.Fatal("CONTROL: gc succeeded with a staged blob only the seat store holds, so the import has nothing to fix")
	}
	if err := importAllSeatStores(f.repo); err != nil {
		t.Fatal(err)
	}
	if _, err := git(f.repo, "gc", "-q"); err != nil {
		t.Fatalf("the operator's gc still fails after the watch-pass import: %v", err)
	}
}
