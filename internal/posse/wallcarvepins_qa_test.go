//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ADR 0100 D2(b)/D4(a) at the CARVE-OUT's producers: THE DIRECTORY
// BETWEEN A WRITE GRANT AND A DENY OR MASK MOUNTED BELOW IT IS BOUND IN
// PLACE, so the render is pinned rather than refused (bwrapCarvePins,
// ranger-base-a59w7).
//
// WHAT THE RULE IS FOR, in one sentence that is not this bead's: a path a
// walled seat can RENAME is a mount of the seat's choosing at the next
// launch, so every component of every bind source and every mount
// destination that lies inside a write grant must itself be a mount
// (bwrapPinned, and wallpinnedsource_qa_test.go's header at length).
//
// WHAT THIS BEAD IS. Shape 1 binds the directories between nested write
// GRANTS (bwrapGrantMounts, ranger-base-dot6w), which is every loose
// component a grant's own in-place bind can have. A deny renders an in-place
// `--ro-bind` and a mask renders `/dev/null` over a name, and those have
// ancestors too: `<repo>` granted with the deny `<repo>/.git/config.lock`
// leaves `<repo>/.git` a mount of nothing inside a directory the seat may
// write, and the grant-keyed list cannot reach it because that directory
// holds no grant of its own. Step 6 then refused the WHOLE render — five of
// this package's own pins, MEASURED 2026-11-21 under a home outside the
// shared `/tmp`, each of them a seat that does not launch.
//
// THE FIXTURE HOMES ARE UNDER wallUngrantedDir or a moved TMPDIR, for the
// reason the other two headers give: `/tmp` is granted unconditionally, so a
// home under it puts everything inside a grant and the rule stands down
// there on purpose (bwrapWallRoot).
//
//	go test ./internal/posse -run 'WallCarvePins' -v
//
// The render arms here need a wall root, so they set HOME; the row table
// does not and is parallel.

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// ─── The list itself, as rows ───────────────────────────────────────────────

// Pin: what bwrapCarvePins answers, one row per shape the decision argues
// about. The render arms below drive the same function through BwrapArgs;
// this is the function's own answer, where a row costs nothing to add.
func TestQAWallCarvePinsNamesTheDirectoriesBetweenAGrantAndACarveOutMount(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name   string
		head   []string
		tail   []string
		grants []string
		want   []string
	}{
		{
			name:   "a deny ONE component inside the grant: its own mount pins it",
			tail:   []string{"--ro-bind", "/g/d", "/g/d"},
			grants: []string{"/g"},
		},
		{
			name:   "a deny TWO components inside: the directory between is pinned",
			tail:   []string{"--ro-bind", "/g/a/d", "/g/a/d"},
			grants: []string{"/g"},
			want:   []string{"/g/a"},
		},
		{
			name:   "THREE components inside: both between, shallowest first",
			tail:   []string{"--ro-bind", "/g/a/b/d", "/g/a/b/d"},
			grants: []string{"/g"},
			want:   []string{"/g/a", "/g/a/b"},
		},
		{
			// The shape TestBwrapArgsSkipsAMaskOverAWriteGrantedCredential
			// Store builds: the store mask stands down inside a grant, so
			// the /dev/null over the credential FILE is what is left and
			// the store directory is loose under it.
			name:   "a MASK: the destination's parent is loose and the /dev/null source is nobody's grant",
			tail:   []string{"--ro-bind", "/dev/null", "/g/a/auth.json"},
			grants: []string{"/g"},
			want:   []string{"/g/a"},
		},
		{
			name:   "a TMPFS, which has no source at all",
			tail:   []string{"--tmpfs", "/g/a/store"},
			grants: []string{"/g"},
			want:   []string{"/g/a"},
		},
		{
			name:   "a Keep file bound back rw two components inside",
			tail:   []string{"--bind", "/g/a/shell.log", "/g/a/shell.log"},
			grants: []string{"/g"},
			want:   []string{"/g/a"},
		},
		{
			name:   "inside no grant at all: a deny under the read-only base pins nothing",
			tail:   []string{"--ro-bind", "/h/a/d", "/h/a/d"},
			grants: []string{"/g"},
		},
		{
			name:   "a component the HEAD already mounts over — a grant pin, a private member — needs none of its own",
			head:   []string{"--bind", "/g/a", "/g/a"},
			tail:   []string{"--ro-bind", "/g/a/d", "/g/a/d"},
			grants: []string{"/g"},
		},
		{
			// Either order, because the question is whether the render
			// holds that name as a mount and not when it took it.
			name:   "a component the TAIL itself mounts over, named after the deny under it",
			tail:   []string{"--ro-bind", "/g/a/d", "/g/a/d", "--tmpfs", "/g/a"},
			grants: []string{"/g"},
		},
		{
			name:   "two denies sharing one loose directory: ONE pin",
			tail:   []string{"--ro-bind", "/g/a/d1", "/g/a/d1", "--ro-bind", "/g/a/d2", "/g/a/d2"},
			grants: []string{"/g"},
			want:   []string{"/g/a"},
		},
		{
			name:   "the loose directory is granted as well: it is a grant, shape 1 binds it, and this list is silent",
			tail:   []string{"--ro-bind", "/g/a/d", "/g/a/d"},
			head:   []string{"--bind", "/g", "/g", "--bind", "/g/a", "/g/a"},
			grants: []string{"/g", "/g/a"},
		},
		{
			// ANCESTORS ONLY. A source that differs from its destination can
			// be loose on its own account, and the remedy for that one is
			// D1's — put it under the per-seat wall dir — so this list does
			// not quietly bind it in place and step 6 still refuses it.
			name:   "a bind SOURCE loose on its own account is not pinned here",
			tail:   []string{"--bind", "/g/a", "/g/b"},
			grants: []string{"/g"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := bwrapCarvePins(c.head, c.tail, c.grants)
			if len(got) == 0 && len(c.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("the carve-out pins over head %v tail %v with grants %v are\n %v\nwant\n %v", c.head, c.tail, c.grants, got, c.want)
			}
		})
	}
}

// ─── The same claim through BwrapArgs ───────────────────────────────────────

// Pin: a render whose DENY lies two components inside a write grant binds
// the directory between in place, in the right place in the argv — and the
// same render with that one bind taken out is refused by the loose
// directory's name, which is what the bind buys.
//
// THE CONTROL IS THE SAME DENY WITH NO GRANT ABOVE IT: no grant, no loose
// component, no pin. That is what "it widens nothing" means here — a pin
// inside no grant would be handing out a write nobody asked for.
//
// MUTATION RUN 2026-11-21: dropping BwrapArgs' step 5d reds this and
// TestQAWallCarvePinsAMasksLooseParentIsBoundInPlace; splicing the pins at
// step 5d's own position instead of 2d's mark reds the order arm here.
func TestQAWallCarvePinsADenysLooseParentIsBoundInPlace(t *testing.T) {
	// No t.Parallel: this sets HOME, which the wall root is composed from.
	root := wallUngrantedDir(t)
	home, repo := filepath.Join(root, "home"), filepath.Join(root, "repo")
	gitDir := filepath.Join(repo, ".git")
	lock := filepath.Join(gitDir, "config.lock")
	for _, d := range []string{home, gitDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(lock, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	if bwrapWallRoot() == "" {
		t.Skip("NOTHING MEASURED: ADR 0100's rules stand down for a home under " + bwrapTmpMount + " (bwrapWallRoot)")
	}

	// CONTROL: the same deny with the cwd elsewhere and no grant over the
	// repo. Nothing is loose, so nothing is pinned.
	other := filepath.Join(root, "elsewhere")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	argv, _, err := BwrapArgs(other, []string{other}, SeatbeltCarveOut{Deny: []string{lock}, DenyFile: []string{lock}})
	if err != nil {
		t.Fatalf("CONTROL: a deny inside no write grant refused the render: %v", err)
	}
	if containsString(bwrapMountDsts(bwrapMounts(argv)), gitDir) {
		t.Errorf("CONTROL: the render binds %s with no grant above it — a pin inside no grant hands out a write nobody asked for\n %s", gitDir, bwJoin(argv))
	}

	// The shipped render: the deny is two components inside the grant, so
	// the directory between is bound in place.
	argv, _, err = BwrapArgs(repo, []string{repo}, SeatbeltCarveOut{Deny: []string{lock}, DenyFile: []string{lock}})
	if err != nil {
		t.Fatalf("a render denying %s inside the grant %s is refused (%v) — that is a seat that does not launch, and the fix ADR 0100 prescribes for a loose component is an in-place bind of it, never an exemption (ranger-base-a59w7)", lock, repo, err)
	}
	line := bwJoin(argv)
	if wcpMountAt(argv, "--bind", gitDir, gitDir) < 0 {
		t.Fatalf("the render denies %s two components inside %s and binds nothing over %s, so a leftover of this seat renames %s and bwrap resolves the deny's own path to whatever the plant points at, at the next launch (ADR 0100 D2(b))\n %s", lock, repo, gitDir, gitDir, line)
	}

	// AND IN THE RIGHT PLACE: after the grant that contains it, before the
	// deny it pins. A pin after the deny would cover the `/dev/null` or the
	// read-only bind with the host's own directory again and take the wall
	// off (BwrapArgs step 2d).
	grant := wcpMountAt(argv, "--bind", repo, repo)
	pin := wcpMountAt(argv, "--bind", gitDir, gitDir)
	deny := wcpMountAt(argv, "--ro-bind", lock, lock)
	if !(grant >= 0 && deny >= 0 && grant < pin && pin < deny) {
		t.Errorf("the mounts are not grant (%d), then pin (%d), then deny (%d): a pin under its own deny takes the wall off, and a pin above its grant is a mount the grant then buries\n %s", grant, pin, deny, line)
	}

	// The same argv with that one bind taken out: refused, by the loose
	// directory's name and with the fix in the message. The refusal is not
	// dead code — the landed-check judge and the wall venue add mounts
	// after BwrapArgs and are judged again there (boxLandedCage).
	without := bwDropInPlaceBind(t, argv, gitDir)
	err = bwrapRefuseUnpinnedSource(without, []string{repo})
	switch {
	case err == nil:
		t.Errorf("a render denying %s inside %s with nothing bound over %s is not refused — that is the whole of what the in-place bind above buys (ADR 0100 D2(b))", lock, repo, gitDir)
	case !strings.Contains(err.Error(), gitDir):
		t.Errorf("the refusal does not name the loose component %s, which is the only thing the operator can act on: %v", gitDir, err)
	case !strings.Contains(err.Error(), "in place"):
		t.Errorf("the refusal does not name the fix (bind the loose component in place): %v", err)
	}
}

// Pin: A LINK AT A PIN'S OWN NAME REFUSES THE RENDER, which is the one
// thing this producer cannot inherit from shape 1's, whose own same question
// is open under the bead id ADR 0100's D2(b) amendment keeps.
//
// The path a pin binds is by construction a
// directory the seat may write — that is why it needs pinning — so a link at
// that name is one a seat or its leftover planted, and bwrap follows a link
// at a bind's source and at its destination alike. Binding it in place would
// resolve both ends to the target and mount that target read-write at its
// own name: the very attack D2(b) refuses, let through by the mount meant to
// refuse it.
//
// BOTH DIRECTIONS, because a refusal that fires on everything is not a
// refusal: the directory arm above is the control, and here the same render
// with a link in place of that directory is refused by name.
func TestQAWallCarvePinsRefuseALinkAtThePinsOwnName(t *testing.T) {
	// No t.Parallel: this sets HOME, which the wall root is composed from.
	root := wallUngrantedDir(t)
	home, repo := filepath.Join(root, "home"), filepath.Join(root, "repo")
	real := filepath.Join(repo, "real-git")
	gitDir := filepath.Join(repo, ".git")
	lock := filepath.Join(gitDir, "config.lock")
	for _, d := range []string{home, real} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(real, "config.lock"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	if bwrapWallRoot() == "" {
		t.Skip("NOTHING MEASURED: ADR 0100's rules stand down for a home under " + bwrapTmpMount + " (bwrapWallRoot)")
	}
	carve := SeatbeltCarveOut{Deny: []string{lock}, DenyFile: []string{lock}}

	// CONTROL: `<repo>/.git` a real directory holding the same deny. The pin
	// is emitted and the render stands, or the refusal below is a refusal of
	// the shape and not of the link.
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lock, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	argv, _, err := BwrapArgs(repo, []string{repo}, carve)
	if err != nil {
		t.Fatalf("CONTROL: the same render over a real directory at %s is refused: %v", gitDir, err)
	}
	if wcpMountAt(argv, "--bind", gitDir, gitDir) < 0 {
		t.Fatalf("CONTROL: the render emitted no pin over %s, so the arm below is not about the link\n %s", gitDir, bwJoin(argv))
	}

	// THE PLANT: the same name, a link to a directory inside the same grant,
	// holding the same deny — so every stat that FOLLOWS answers "a
	// directory, and the deny is there", and the deny itself resolves inside
	// the grant and renders. Where it points is not the question: a seat can
	// repoint it between this render and the mount, which is the
	// check-then-use this ADR rejects.
	if err := os.RemoveAll(gitDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, gitDir); err != nil {
		t.Fatal(err)
	}
	argv, prep, err := BwrapArgs(repo, []string{repo}, carve)
	switch {
	case err == nil:
		t.Fatalf("a link at %s was bound in place as a pin, so bwrap resolves both ends of that bind to whatever it points at and mounts that read-write at a name inside the grant (ADR 0100 D1/D2(b))\n %s", gitDir, bwJoin(argv))
	case !strings.Contains(err.Error(), gitDir):
		t.Errorf("the refusal does not name %s, which is the plant the operator has to remove: %v", gitDir, err)
	case !strings.Contains(err.Error(), "a symlink"):
		t.Errorf("the refusal does not say what is at that name in words, so a reader cannot tell it from a missing directory (fileTypeName): %v", err)
	}
	if argv != nil || prep != nil {
		t.Errorf("a refused render returned argv %v and prep %v", argv, bwPrep(prep))
	}
}

// Pin: the same at the DESTINATION arm, which is the other half of this
// class and a different attack (ADR 0100 D4(a), MEASURED: a namespace
// holding no mount on the parent renames it mid-session and plants its own
// directory at that name, while the victim's mount rides the renamed one).
// A read mask is `/dev/null` over a NAME, so its loose ancestor is the
// directory above it — here a credential store inside a write grant, which
// is the shape TestBwrapArgsSkipsAMaskOverAWriteGrantedCredentialStore's
// "the grant is above the store" row renders: the store's own tmpfs stands
// down inside a grant, so the file mask is what is left and nothing else
// names the store.
//
// AND THE PIN LANDS BEFORE 2c's PRIVATE MEMBERS as well, which the entry
// here is for: those are walls over paths inside the same grants, and a pin
// above one emitted after it would bury it.
func TestQAWallCarvePinsAMasksLooseParentIsBoundInPlace(t *testing.T) {
	// No t.Parallel: this sets HOME, which the wall root is composed from.
	root := wallUngrantedDir(t)
	home, proj := filepath.Join(root, "home"), filepath.Join(root, "proj")
	cfg := filepath.Join(root, "cfg")
	store := filepath.Join(cfg, "codex")
	cred := filepath.Join(store, "auth.json")
	priv := filepath.Join(store, "shell-snapshots")
	for _, d := range []string{home, proj, store} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(cred, []byte("token"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	if bwrapWallRoot() == "" {
		t.Skip("NOTHING MEASURED: ADR 0100's rules stand down for a home under " + bwrapTmpMount + " (bwrapWallRoot)")
	}

	argv, _, err := BwrapArgs(proj, []string{proj, cfg}, SeatbeltCarveOut{
		DenyRead:    []string{cred},
		ReadMaskDir: []string{store},
		Private:     []PrivateBind{{Dst: priv}},
	})
	if err != nil {
		t.Fatalf("a render masking %s inside the grant %s is refused (%v) — a seat that does not launch, and the remedy is the in-place bind of the directory between (ranger-base-a59w7)", cred, cfg, err)
	}
	line := bwJoin(argv)
	if wcpMountAt(argv, "--bind", store, store) < 0 {
		t.Fatalf("the render mounts /dev/null over %s and nothing over its parent %s, so a leftover of this seat renames the parent and plants the name while this seat's mask rides the renamed one (ADR 0100 D4(a))\n %s", cred, store, line)
	}
	pin := wcpMountAt(argv, "--bind", store, store)
	mask := wcpMountAt(argv, "--ro-bind", "/dev/null", cred)
	private := wcpMountAt(argv, "--tmpfs", priv)
	if !(mask >= 0 && pin < mask) {
		t.Errorf("the pin (%d) lands after the mask (%d), so the host's own directory covers the name the /dev/null was mounted over\n %s", pin, mask, line)
	}
	if !(private >= 0 && pin < private) {
		t.Errorf("the pin (%d) lands after 2c's private member (%d), so it buries a wall the seat's siblings are held out by\n %s", pin, private, line)
	}
}

// ─── The census ─────────────────────────────────────────────────────────────

// CENSUS (the one ranger-base-a59w7 was told to run): how many carve-out
// pins each of the five argvs this package renders needs, which is the other
// half of the two censuses in wallpinnedsource_qa_test.go and
// wallpinneddst_qa_test.go — those say every in-place bind and every
// differing destination is PINNED, this says what step 5d had to emit for
// them to be.
//
// MEASURED 2026-11-21, read at 65d90a5e plus this bead's fix: 0 over all
// five (claude, codex, grok, the judge, the venue). No shipped render has a
// deny or mask two components inside a write grant, so the list is inert
// until one does — the shapes that have one are a `<repo>` grant with a
// `.git/config*` deny (a main-checkout seat, which ADR 0095 D1 refuses one
// line earlier for a reason of its own) and a grant above a CLI home with
// that home's credential store masked.
//
// IT IS A LOG AND NOT AN EXPECTED NUMBER. A render that grows a pin is
// CORRECT — that is what the list is for — so a count asserted at 0 would
// red the day somebody fixed a wall. What this holds is that the number can
// be read: the two censuses hold the property, and a pin that fails to be
// emitted reds there.
func TestQAWallCarvePinsCensusOverEveryRender(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	f := wpsSeatTree(t)
	if err := os.MkdirAll(ClaudeConfigDirIn(f.home), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"claude", "codex", "grok"} {
		r, err := f.a.bwrapRender(f.ag, wrcRuntime(name), f.dir, f.gates)
		if err != nil {
			t.Fatalf("%s: the render is refused (%v) — if that is this rule's refusal, the render needs an in-place bind of the loose component and not an exemption", name, err)
		}
		t.Logf("CENSUS: the %s render needs %d carve-out pin(s): %v", name, len(wcpPins(r.Argv, r.Set)), wcpPins(r.Argv, r.Set))
	}
	g := newLandedCageFixture(t)
	if WallMechanism() != WallBwrap {
		t.Skip("the judge and venue arms want this host's bwrap render; its mechanism is " + WallMechanism())
	}
	writable := []string{g.run}
	for _, kind := range []landedCageKind{landedCageJudge, landedCageVenue} {
		cage, err := boxLandedCage(&App{}, kind, filepath.Join(g.run, "judge"), g.tree, writable, landedRunEnv(g.run))
		if err != nil {
			t.Fatalf("%s: the render is refused (%v) — same reading as the seat arms above", kind, err)
		}
		t.Logf("CENSUS: the %s render needs %d carve-out pin(s): %v", kind, len(wcpPins(cage.Argv, writable)), wcpPins(cage.Argv, writable))
	}
}

// wcpMountAt is the argv INDEX of a mount spelled as consecutive words, or
// -1. Read off the ARGV and not off bwJoin's line, because a string search
// for `--bind p p` needs a trailing space to tell `p` from `p/x` — and a
// mount that lands LAST in the argv has no trailing space, so an order arm
// driven by strings reads "absent" for a pin spliced at the wrong END and
// then reports the wrong thing. MEASURED while this file was written: the
// splice-late mutant red this test's presence arm rather than its order arm.
func wcpMountAt(argv []string, words ...string) int {
	for i := 0; i+len(words) <= len(argv); i++ {
		hit := true
		for j, w := range words {
			if argv[i+j] != w {
				hit = false
				break
			}
		}
		if hit {
			return i
		}
	}
	return -1
}

// wcpPins is the census over one FINISHED argv: the pins such a render would
// need if nothing in it were bound in place yet. It reads the whole argv as
// both halves, which is the one honest reading of a render whose pins are
// already spliced in — a path step 5d pinned is a mount destination now, so
// it answers for itself and the count is what remains.
func wcpPins(argv, grants []string) []string {
	return bwrapCarvePins(nil, argv, shallowestFirst(grants))
}

// bwDropInPlaceBind is one argv with the in-place bind of `p` removed, for
// the arms that assert what the pin buys by taking it out. It fails the test
// when there was none to drop, so an arm cannot measure nothing.
func bwDropInPlaceBind(t *testing.T, argv []string, p string) []string {
	t.Helper()
	var out []string
	for i := 0; i < len(argv); i++ {
		if argv[i] == "--bind" && i+2 < len(argv) && argv[i+1] == p && argv[i+2] == p {
			i += 2
			continue
		}
		out = append(out, argv[i])
	}
	if len(out) == len(argv) {
		t.Fatalf("CONTROL: there was no in-place bind of %s to drop, so the arm below measures nothing", p)
	}
	return out
}
