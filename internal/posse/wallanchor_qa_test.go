//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-v84cn: the deny directories a host process keeps
// removing while they are empty get an anchor file before their bind, so a
// host rmdir is refused and cannot take the wall off a live seat.
// SeatbeltCarveOut.Anchored and claudeSweptConfigDirNames have the
// measurement.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// The render: an anchored deny DIRECTORY gets a seed of the anchor file
// after its own mkdir (absent) or alone (present). An anchored name that is
// a FILE gets nothing, and so does an unanchored dir.
//
// MUTATION-CHECKED: dropping the anchor step from BwrapArgs reds the first
// two rows; dropping the `!exists || dir` guard reds the file row.
func TestBwrapArgsAnchorsAnAnchoredDenyDirectoryAndNothingElse(t *testing.T) {
	t.Parallel()
	_, bwAt := bwTree(t, []string{"wt", "cfg/present", "cfg/plain"}, []string{"cfg/afile"})
	grant := bwAt("cfg")
	present, absent, afile, plain := bwAt("cfg", "present"), bwAt("cfg", "absent"), bwAt("cfg", "afile"), bwAt("cfg", "plain")

	argv, prep, err := BwrapArgs(bwAt("wt"), []string{bwAt("wt"), grant}, SeatbeltCarveOut{
		Deny:   []string{present, absent, afile, plain},
		Anchor: []string{present, absent, afile},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(bwPrep(prep), "\n")
	for _, want := range []string{
		"seed " + filepath.Join(present, wallAnchorName),
		"mkdir -p " + absent + "\nseed " + filepath.Join(absent, wallAnchorName),
	} {
		if !strings.Contains(got, want) {
			t.Errorf("prep has no %q — an anchored deny dir left empty is one host rmdir away from losing its mount:\n%s", want, got)
		}
	}
	for _, no := range []string{filepath.Join(afile, wallAnchorName), filepath.Join(plain, wallAnchorName)} {
		if strings.Contains(got, no) {
			t.Errorf("prep anchors %s, which is a file or not anchored:\n%s", no, got)
		}
	}
	line := bwJoin(argv)
	for _, d := range []string{present, absent, afile, plain} {
		if !strings.Contains(line, "--ro-bind "+d+" "+d) {
			t.Errorf("the anchor changed the wall: %s is no longer bound read-only:\n%s", d, line)
		}
	}
}

// End to end over claude's config dir: the carve anchors exactly
// claudeSweptConfigDirNames, the launcher's prep plants the anchor in each,
// a host rmdir of each is then refused, and a second prep (the next launch)
// leaves the anchor as it is. A dir that loads code gets no anchor.
func TestQAWallAnchorKeepsClaudesSweptConfigDirsFromAHostRmdir(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	f := weFixtureIn(t, wallUngrantedDir(t))
	stateDirs, err := BwrapStateDirs(f.rt)
	if err != nil {
		t.Fatal(err)
	}
	cfg := absResolve(f.cfg)
	w := f.a.SeatbeltWritable(f.ag, f.dir, f.gates, stateDirs...)
	c := f.a.SeatbeltCarveOut(f.ag, f.dir, f.gates, w, stateDirs...)

	// Hand-typed, so a name dropped from the table reds here.
	swept := []string{"access-audit", "logs", "mcp-discovery-cache", "mcp-skill-archives", "shares", "statsig", "todos"}
	var want []string
	for _, n := range swept {
		want = append(want, filepath.Join(cfg, n))
	}
	if strings.Join(c.Anchor, "\n") != strings.Join(want, "\n") {
		t.Fatalf("carve anchors %v, want %v", c.Anchor, want)
	}

	r, err := f.a.bwrapRender(f.ag, f.rt, f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ { // the second run is the next launch
		if err := runBwrapPrep(r.Prep); err != nil {
			t.Fatalf("prep run %d: %v", i+1, err)
		}
	}
	for _, d := range want {
		if fi, err := os.Lstat(filepath.Join(d, wallAnchorName)); err != nil || !fi.Mode().IsRegular() || fi.Size() != 0 {
			t.Errorf("%s holds no empty anchor file after the prep: %v", d, err)
			continue
		}
		if err := syscall.Rmdir(d); !errors.Is(err, syscall.ENOTEMPTY) && !errors.Is(err, syscall.EEXIST) {
			t.Errorf("host rmdir of %s = %v, want ENOTEMPTY — an rmdir that succeeds takes the seat's mount off", d, err)
		}
		// AND A RENAME OVER IT, which is the other way a host takes a
		// directory's name (ranger-base-ua625). rename(2) over a directory
		// needs the target EMPTY, so the same anchor refuses both — not a
		// second mechanism, a second caller of the one the anchor already
		// beats. MEASURED here rather than claimed: the live detach pin in
		// wallwatchlive_test.go has to remove the anchor before its rename
		// reaches a detach at all.
		over := d + ".rename-probe"
		if err := os.Mkdir(over, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := syscall.Rename(over, d); !errors.Is(err, syscall.ENOTEMPTY) && !errors.Is(err, syscall.EEXIST) {
			t.Errorf("host rename over %s = %v, want ENOTEMPTY — a rename that succeeds takes the seat's mount off exactly as an rmdir does", d, err)
		}
		if err := os.Remove(over); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	for _, n := range []string{"daemon", "jobs", "skills", "hooks", "state"} {
		if _, err := os.Lstat(filepath.Join(cfg, n, wallAnchorName)); err == nil {
			t.Errorf("%s/ got an anchor; only claudeSweptConfigDirNames are anchored, and a file in a dir claude acts on is not measured inert", n)
		}
	}
}

// ─── the class: every empty host mount point, not a list of names ────────────
//
// Pins for ranger-base-ua625 finding 1, from the verify at
// ranger-base-pxprm. The anchor was filed as seven names and the property
// that made those names dangerous is a SHAPE — a mount point this render
// leaves empty on the host — which two other arms were in and neither was
// anchored: the credential-store tmpfs (step 5c) and the private binds over
// claude's shared state (step 2).
//
// SINCE ranger-base-bncbk THE SHAPE HAS NO EXCEPTION. ua625 drew the class's
// edge at "this launcher", which excluded steps 5a (~/.ssh) and 5b (a
// read-masked state dir) because both SKIPPED an absent directory. That skip
// was itself the hole bncbk closed — an absent one got no mount and no watch,
// so a credential the host wrote during the seat's life was readable and
// nothing detached — and both arms make their directory now. So the switch
// below has one read-mask arm where it used to have two, and the floors at
// its foot are per producer, because one total was how the pin passed while
// grading neither of them.

// An anchored deny whose name is a LINK gets no anchor, and the one it does
// not get is the one that used to land in the link's TARGET
// (ranger-base-ua625 finding 1, second half). The arm read `dir` out of
// bwrapStat, which is os.Stat and follows.
//
// MUTATION-CHECKED: putting bwrapAnchorStep back on os.Stat reds this.
func TestBwrapArgsAnchorsNoLinkAndSeedsNothingInItsTarget(t *testing.T) {
	t.Parallel()
	_, bwAt := bwTree(t, []string{"wt", "cfg/target"}, nil)
	grant, target, link := bwAt("cfg"), bwAt("cfg", "target"), bwAt("cfg", "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	_, prep, err := BwrapArgs(bwAt("wt"), []string{bwAt("wt"), grant}, SeatbeltCarveOut{
		Deny:   []string{link},
		Anchor: []string{link},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(bwPrep(prep), "\n")
	for _, no := range []string{
		filepath.Join(target, wallAnchorName), // the one that used to land
		filepath.Join(link, wallAnchorName),
	} {
		if strings.Contains(got, no) {
			t.Errorf("prep seeds %s — a link at an anchored name is not a directory this render may write inside:\n%s", no, got)
		}
	}
}

// The two arms the list never reached. A masked credential store and a
// private bind are both mounts over a host directory this render leaves
// EMPTY, so both carry an anchor; a private FILE bind carries none, because
// nothing rmdirs a file.
//
// MUTATION-CHECKED: dropping either new anchor call reds its own rows.
func TestBwrapArgsAnchorsTheCredentialStoreMaskAndEveryPrivateDirBind(t *testing.T) {
	t.Parallel()
	_, bwAt := bwTree(t, []string{"wt", "home/.bob", "home/.claude/sessions", "home/seat/claude/sessions"}, []string{"home/.claude/history.jsonl"})
	store := bwAt("home", ".bob")
	absent := bwAt("home", ".codex")                  // made empty by the prep, so in the shape by construction
	sessions := bwAt("home", ".claude", "sessions")   // a private dir copy
	ephem := bwAt("home", ".claude", "tmp")           // the empty-tmpfs arm
	inplace := bwAt("home", ".claude", "projects")    // the bound-in-place arm
	histf := bwAt("home", ".claude", "history.jsonl") // a private FILE bind

	// The grant is claude's config dir and NOT the home: a write grant that
	// reaches a credential store renders no mask at all (step 5c's own skip),
	// so granting the home would make every store row read "no anchor" for a
	// reason that is not this pin's.
	_, prep, err := BwrapArgs(bwAt("wt"), []string{bwAt("wt"), bwAt("home", ".claude")}, SeatbeltCarveOut{
		ReadMaskDir: []string{store, absent},
		Private: []PrivateBind{
			{Src: bwAt("home", "seat", "claude", "sessions"), Dst: sessions, SrcRoot: bwAt("home", "seat")},
			{Src: "", Dst: ephem},
			{Src: inplace, Dst: inplace},
			{Src: bwAt("home", "seat", "claude", "history.jsonl"), Dst: histf, SrcRoot: bwAt("home", "seat"), File: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(bwPrep(prep), "\n")
	for _, want := range []string{store, absent, sessions, ephem} {
		if !strings.Contains(got, "seed "+filepath.Join(want, wallAnchorName)) {
			t.Errorf("prep has no anchor in %s — an empty host mount point is one rmdir from detaching whatever is over it:\n%s", want, got)
		}
	}
	for _, no := range []struct{ path, why string }{
		{histf, "a private FILE bind got an anchor; nothing rmdirs a file"},
		{inplace, "a dir bound over ITSELF got an anchor; the seat writes through to that name, so the launcher neither empties it nor keeps it empty, and the file would be posse's in a dir the operator's own claude enumerates"},
	} {
		if strings.Contains(got, filepath.Join(no.path, wallAnchorName)) {
			t.Errorf("%s:\n%s", no.why, got)
		}
	}
}

// THE CLASS, DERIVED FROM THE RENDER AND NOT FROM A TABLE. Every mount this
// render records as a WALL over a host DIRECTORY carries an anchor, and the
// two kinds that do not are priced here by kind rather than by name — so a
// NEW arm that mounts a directory and watches it reds this pin instead of
// joining the class silently, which is what finding 1 is about.
//
// It is the anchor half of what TestWallWatchEveryRenderedMountIsAWallOrA
// NamedOpening does for the watch list, and it holds both ends the same way:
// the argv says what was mounted, the carve-out says what each path is.
func TestQAWallAnchorCoversEveryWatchedDirectoryMountOrPricesIt(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and CLAUDE_CONFIG_DIR.
	f := weFixtureIn(t, wallUngrantedDir(t))
	r, err := f.a.bwrapRender(f.ag, f.rt, f.dir, f.gates)
	if err != nil {
		t.Fatal(err)
	}
	anchored := map[string]bool{}
	for _, s := range r.Prep {
		if s.Kind == PrepSeed && filepath.Base(s.Path) == wallAnchorName {
			anchored[filepath.Dir(s.Path)] = true
		}
	}
	// DIRECTORY-OR-FILE IS READ OFF THE INPUTS AND NEVER OFF THE DISK. The
	// first draft lstat'd each bind's SOURCE, and a private copy's source is
	// a path the PREP makes — which has not run — so all seven copy binds
	// lstat'd ENOENT, were scored as file mounts, and the arm this pin exists
	// for graded nothing while reading green. The `graded` floor at the foot
	// is what caught it. The carve-out already says what every path is, which
	// is the half this pin holds that the render cannot.
	graded := 0               // private mounts this pin REQUIRED an anchor of
	masks := map[string]int{} // read-mask tmpfs mounts it required, BY PRODUCER
	seen := map[string]bool{}
	for _, w := range r.Watch {
		seen[w.Kind] = true
		switch w.Kind {
		case WallWatchPrivate:
			pb, ok := wallPrivateAt(r.Carve.Private, w.Path)
			switch {
			case !ok:
				t.Errorf("a watched private mount over %s is in no PrivateBind of the carve-out, so this pin cannot say what it mounts", w.Path)
			case pb.File:
				if anchored[w.Path] {
					t.Errorf("the private FILE bind at %s got an anchor; nothing rmdirs a file", w.Path)
				}
			case pb.Src == pb.Dst:
				// PRICED, and the only private arm that is: a dir bound
				// over ITSELF is the host's live directory, written
				// through by the seat, so the launcher neither empties it
				// nor keeps it empty. claude's `projects/` is the live
				// member, and a live pin caught the first draft anchoring
				// it — as "the real claude changed projects/", the change
				// being posse's.
				if anchored[w.Path] {
					t.Errorf("%s is bound in place and got an anchor; posse neither empties that directory nor keeps it empty", w.Path)
				}
			default:
				graded++
				if !anchored[w.Path] {
					t.Errorf("the private copy or tmpfs over the host dir %s carries no anchor — the launcher leaves it empty for the seat's whole life, and one host rmdir detaches it (ranger-base-ua625)", w.Path)
				}
			}
		case WallWatchReadMask:
			// THREE PRODUCERS SHARE THIS KIND AND THEY ARE NOW ONE CLASS,
			// which is ranger-base-bncbk undrawing the edge this switch used
			// to carry. It used to read: 5c MAKES its directory empty so a
			// login after launch lands inside the mask, so it is anchored;
			// 5a (~/.ssh) and 5b (DenyReadDir) mount over a directory the
			// HOST made and SKIP an absent one, so posse neither creates nor
			// empties them and a seed there would be posse writing into the
			// operator's ~/.ssh. The skip was the hole — an absent ~/.ssh or
			// state dir got no mount and no watch, so a credential written
			// during the seat's life was readable with nothing to detach —
			// and all three arms make their directory now. So all three are
			// REQUIRED to anchor, and the only question left is whether the
			// directory mount has a tmpfs over it at all.
			//
			// COUNTED BY PRODUCER and not in one total, because that is the
			// shape of the mistake the floor at the foot is for: with one
			// number, a fixture whose three credential stores all render
			// satisfies the floor while 5a and 5b grade nothing, which is
			// exactly the state this bead found the pin in.
			switch {
			case wallMember(r.Carve.ReadMaskDir, w.Path),
				wallMember(r.Carve.SSHDir, w.Path),
				wallMember(r.Carve.DenyReadDir, w.Path):
				masks[wallReadMaskProducer(r.Carve, w.Path)]++
				if !anchored[w.Path] {
					t.Errorf("the read-mask tmpfs over %s (%s) carries no anchor — this launcher leaves that directory empty on purpose, and one host rmdir then detaches the mask and shows the seat whatever the host writes at that name after launch (ranger-base-ua625, ranger-base-bncbk)", w.Path, wallReadMaskProducer(r.Carve, w.Path))
				}
			case wallMember(r.Carve.DenyRead, w.Path):
				if anchored[w.Path] {
					t.Errorf("the read deny at %s got an anchor; it is a file", w.Path)
				}
			default:
				t.Errorf("read-mask mount over %s comes from no producer this pin knows — say whether its directory is one this launcher leaves empty (ranger-base-ua625)", w.Path)
			}
		case WallWatchDeny:
			if r.Carve.FileShaped(w.Path) {
				if anchored[w.Path] {
					t.Errorf("the file-shaped deny at %s got an anchor; nothing rmdirs a file", w.Path)
				}
				continue
			}
			// PRICED: a deny dir is anchored only when its producer says so.
			// MEASURED 2026-11-14 across a sweep that took all seven swept
			// dirs: the fourteen empty members of claudeExecConfigDirNames
			// were untouched, and they are dirs claude LOADS CODE from,
			// where an empty dotfile is not measured inert.
			if anchored[w.Path] != r.Carve.Anchored(w.Path) {
				t.Errorf("deny dir %s: anchored=%v, carve says %v — the Anchor field is what decides, and the carve-out's doc prices the exclusion", w.Path, anchored[w.Path], r.Carve.Anchored(w.Path))
			}
		case WallWatchMasked:
			if anchored[w.Path] {
				t.Errorf("the /dev/null mask at %s got an anchor; it is a file", w.Path)
			}
		case WallWatchHide:
			// PRICED: the user runtime dir and the host's /tmp. Neither is a
			// directory posse creates or leaves empty, and a seed in either
			// is a file in something else's area.
			if anchored[w.Path] {
				t.Errorf("hide mount %s got an anchor; posse neither makes nor empties that directory", w.Path)
			}
		default:
			t.Errorf("mount kind %q over %s is in no priced class — say in this switch whether an empty mount point of this kind is anchored (ranger-base-ua625)", w.Kind, w.Path)
		}
	}
	// The fixture must exercise the arms, or the rows above are vacuous.
	for _, k := range []string{WallWatchDeny, WallWatchPrivate, WallWatchReadMask} {
		if !seen[k] {
			t.Errorf("the fixture rendered no watched %s mount, so this pin graded none of them", k)
		}
	}
	// And floors on what was REQUIRED to carry an anchor, not merely priced
	// out: a fixture whose every private bind were in place satisfies the
	// loop above and grades nothing. Eight private copies here, MEASURED
	// 2026-11-21 (seven before ranger-base-bncbk); the floor is under that
	// so an ordinary landing does not move it.
	if graded < 4 {
		t.Errorf("the pin required an anchor of %d private copy/tmpfs mount(s), want at least 4 — a reader that stopped recognising that arm passes every row above", graded)
	}
	// EVERY READ-MASK PRODUCER, one floor each and not one total
	// (ranger-base-bncbk). Three stores, one ~/.ssh and one native rex state
	// dir in this fixture, MEASURED 2026-11-21 — five mounts, and under the
	// single floor of 2 this pin used to carry, a reader that stopped
	// recognising 5a and 5b passed on the stores alone. Which is not
	// hypothetical: that is the state the pin was in, pricing 5a and 5b OUT
	// of the class, while an absent ~/.ssh went unmasked and unwatched for a
	// seat's whole life.
	for _, p := range []string{"5a ~/.ssh", "5b read-masked state dir", "5c credential store"} {
		if masks[p] == 0 {
			t.Errorf("the pin required an anchor of no %s mount (the five it graded: %v) — a fixture that renders none of one producer's masks says nothing about that producer, and an unanchored tmpfs is one host rmdir from showing the seat a credential written after launch", p, masks)
		}
	}
}

// wallReadMaskProducer names which of step 5's three directory masks put this
// path in the carve-out, so a row can say what it graded. All three are one
// class since ranger-base-bncbk — made when absent, anchored, watched — and
// the name is for the floors at the foot and for the failure text.
func wallReadMaskProducer(c SeatbeltCarveOut, p string) string {
	switch {
	case wallMember(c.SSHDir, p):
		return "5a ~/.ssh"
	case wallMember(c.DenyReadDir, p):
		return "5b read-masked state dir"
	case wallMember(c.ReadMaskDir, p):
		return "5c credential store"
	}
	return ""
}

// wallMember reports whether a carve-out list names this exact path. The
// lists hold absResolved paths and so does the argv, so it is equality and
// never a prefix: a prefix test would let a mount INSIDE one of these
// directories borrow its producer's pricing.
func wallMember(list []string, p string) bool {
	for _, d := range list {
		if d == p {
			return true
		}
	}
	return false
}

// wallPrivateAt returns the private bind this render mounted at dst, so the
// pin can ask the carve-out what the mount IS rather than asking a disk the
// prep has not touched yet.
func wallPrivateAt(ps []PrivateBind, dst string) (PrivateBind, bool) {
	for _, pb := range ps {
		if pb.Dst == dst {
			return pb, true
		}
	}
	return PrivateBind{}, false
}
