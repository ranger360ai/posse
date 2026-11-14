//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-ihnnl (design ranger-base-0qpsw D1+D2): what the
// render hands `posse wall-watch`, the line that starts it, the mount-table
// diff, and D2's one-bead-per-path-per-ten-minutes record. The live half,
// a real bwrap seat losing a wall, is wallwatchlive_test.go.

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// ─── the render's walls, held from the ARGV and not from a list ─────────────
//
// WHY THERE IS A DERIVATION HERE AT ALL (ranger-base-mxbq0 finding 2,
// discovered from the verify at ranger-base-6rj8w). The pin below used to be
// a reflect.DeepEqual against a hand-written literal over a fixture that
// carried only the kinds already recorded, so it could not see a wall kind
// the FIXTURE omitted: MEASURED 2026-11-13, a render whose watch list came
// back EMPTY passed the pin named "records every wall" unchanged, because
// the fixture asked for none of the kinds that render had. That is how
// ranger-base-w6ip0 came to be found by hand instead of by this file, and a
// literal of six leaves the sixth-to-seventh step the same silent one.
//
// So the expectation is DERIVED, the shape this tree already uses for a
// class that must stay total (make corpus-check, pkgcensus, treewidedoor):
// every destination BwrapArgs mounts over is either in the watch list or in
// an opening set computed from the render's own INPUTS, and every watched
// path is a destination the argv really carries. A new mount with no watch()
// call reds; a watch() call with no mount reds; and an option the table
// below does not classify reds rather than being skipped, because a mount
// flag nobody classified is exactly the hole this replaces.
//
// THE FIXTURE IS HELD TOTAL FROM THE OTHER END TOO
// (TestWallWatchFixturesCarryEveryCarveOutField): a derivation over an argv
// says nothing about a shape the inputs never asked for, so every field of
// SeatbeltCarveOut must have a value in one of the renders here. A field
// added to that struct tomorrow reds until somebody gives it one and says
// which side of the rule its mount falls on.

// bwrapMountOpt is one option BwrapArgs can emit: how many arguments it
// takes, and whether the last of them is a mount DESTINATION.
type bwrapMountOpt struct {
	args  int
	mount bool
}

// bwrapRenderOpts is CLOSED on purpose. The pin's claim is about the render
// and not about this table, so an option the render emits and this table
// does not name is an error and never a skip — the skip is what would let a
// seventh mount kind arrive unwatched and green.
var bwrapRenderOpts = map[string]bwrapMountOpt{
	"--die-with-parent": {0, false},
	"--unshare-user":    {0, false},
	"--chdir":           {1, false},
	"--unsetenv":        {1, false},
	"--tmpfs":           {1, true},
	"--bind":            {2, true},
	"--ro-bind":         {2, true},
	"--dev-bind":        {2, true},
}

// bwrapMountDests answers every path argv mounts over, in render order.
func bwrapMountDests(t *testing.T, argv []string) []string {
	t.Helper()
	var dests []string
	for i := 0; i < len(argv); {
		o, ok := bwrapRenderOpts[argv[i]]
		if !ok {
			t.Fatalf("argv[%d] = %q is an option bwrapRenderOpts does not classify, so this file cannot say whether its destination is a wall or an opening. Add it there — and if it mounts, decide in bwrapArgsInto whether it takes a watch() call: an unclassified mount is how ranger-base-w6ip0 escaped.\nargv: %q", i, argv[i], argv)
		}
		if i+o.args >= len(argv) {
			t.Fatalf("argv[%d] = %q wants %d arguments and the argv ends: %q", i, argv[i], o.args, argv)
		}
		if o.mount {
			dests = append(dests, argv[i+o.args])
		}
		i += o.args + 1
	}
	return dests
}

// wwRender is one render this file holds to the wall/opening rule.
type wwRender struct {
	what     string
	cwd      string
	writable []string
	carve    SeatbeltCarveOut
	refKeep  []string
}

// wwOpenings answers which of a render's destinations RE-OPEN something, as
// dst -> why, derived from the render's inputs and never from the argv.
//
// Derived, because that is what makes the pin's other half total: a new
// input list that renders a wall is caught by having no watch() call, and a
// new input list that renders an OPENING is caught here — the author has to
// come to this function and say so in as many words, which is the step the
// hand-written literal did not have.
func wwOpenings(c wwRender) map[string]string {
	o := map[string]string{
		// Step 1's base. Not a mount over a name a host can rename out from
		// under a seat: it is the floor the rest of the wall stands on, and
		// the one destination whose detaching has no meaning.
		"/": "the read-only base of step 1",
		// seatbelt's (regex #"^/dev/") grant, which every runtime's tty needs.
		"/dev": "step 1's dev bind",
	}
	for _, w := range c.writable {
		o[w] = "a write grant: a detached one leaves the seat with LESS than it was given"
		o[absResolve(w)] = "a write grant: a detached one leaves the seat with LESS than it was given"
	}
	for _, f := range c.carve.Keep {
		o[f] = "a Keep file, bound back over the deny above it"
	}
	for _, d := range c.carve.SSHDir {
		for _, f := range c.carve.sshDirAllowed(d) {
			o[f] = "an ssh allow bind: a hole deliberately left in the " + d + " tmpfs, which is the wall"
		}
	}
	return o
}

// wwEveryShape is the canonical render: every field of SeatbeltCarveOut that
// bwrapArgsInto reads a mount out of, plus the two it deliberately reads
// nothing out of (Seal, whose rename a mount point cannot be carried away
// by, and Narrow and DenyAll, which bwrapRender clears), plus a write grant and a D20
// ref keep. Both tests below drive it.
func wwEveryShape(t *testing.T) wwRender {
	t.Helper()
	root := t.TempDir()
	grant := filepath.Join(root, "cfg")
	priv := filepath.Join(grant, "shell-snapshots")
	src := filepath.Join(root, "seat", "shell-snapshots")
	deny := filepath.Join(grant, "settings.json")
	mask := filepath.Join(grant, "policy-cache.json")
	gates := filepath.Join(grant, "gates")
	hide := filepath.Join(root, "run")
	sshDir := filepath.Join(root, ".ssh")
	tsnet := filepath.Join(root, "state", "rex", "tsnet")
	cred := filepath.Join(root, "sibling", "credentials.json")
	// Absent on purpose: a Seeded file-shaped deny inside a grant (the
	// launcher writes it empty and the mount goes over it), and the ref keep
	// that renders no mount at all.
	seeded := filepath.Join(grant, "settings-seeded.json")
	for _, d := range []string{grant, priv, src, hide, sshDir, tsnet, gates, filepath.Dir(cred)} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{
		deny, mask, cred,
		filepath.Join(sshDir, "config"), filepath.Join(sshDir, "known_hosts"), filepath.Join(sshDir, "id_ed25519"),
		filepath.Join(gates, "refusals.log"),
	} {
		if err := os.WriteFile(f, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return wwRender{
		what:     "every shape",
		writable: []string{grant},
		refKeep:  []string{filepath.Join(grant, "refs-keep")},
		carve: SeatbeltCarveOut{
			Hide:        []string{hide},
			Deny:        []string{deny, mask, gates, seeded},
			DenyFile:    []string{deny, mask, seeded},
			Mask:        []string{mask},
			Seed:        []string{seeded},
			Seal:        []string{grant},
			Keep:        []string{filepath.Join(gates, "refusals.log")},
			Narrow:      []SeatbeltNarrow{{Path: priv, Allow: false}},
			DenyAll:     []string{priv},
			Private:     []PrivateBind{{Src: src, Dst: priv}},
			SSHDir:      []string{sshDir},
			DenyReadDir: []string{tsnet},
			DenyRead:    []string{cred, filepath.Join(sshDir, "id_ed25519")},
		},
	}
}

// wwPrivateTmp is its own render because carve.Tmp mounts over /tmp
// (bwrapTmpMount) and every path a t.TempDir fixture has is UNDER /tmp — so
// in one fixture with the shapes above, `reopens` would skip the denies and
// the ssh tmpfs that no write grant re-opens, and the render would be
// measuring almost nothing. Here the one shape is the point.
func wwPrivateTmp(t *testing.T) wwRender {
	t.Helper()
	root := t.TempDir()
	tmp := filepath.Join(root, "seat-tmp")
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatal(err)
	}
	return wwRender{what: "the seat's private /tmp", carve: SeatbeltCarveOut{Tmp: tmp}}
}

func wwRenders(t *testing.T) []wwRender {
	t.Helper()
	return []wwRender{wwEveryShape(t), wwPrivateTmp(t)}
}

// Every mount the render emits is either a WALL it watches or an opening
// wwOpenings names, and every wall it watches is a mount the argv carries.
// The pin ranger-base-ihnnl shipped read a hand-written list instead, and a
// wall kind the fixture omitted was invisible to it (head comment above).
func TestWallWatchEveryRenderedMountIsAWallOrANamedOpening(t *testing.T) {
	t.Parallel()
	for _, c := range wwRenders(t) {
		argv, prep, watch, err := bwrapArgsWatched(c.cwd, c.writable, c.carve, c.refKeep...)
		if err != nil {
			t.Fatalf("%s: %v", c.what, err)
		}
		open := wwOpenings(c)
		watched := map[string]bool{}
		for _, w := range watch {
			watched[w.Path] = true
		}
		dests := bwrapMountDests(t, argv)
		seen := map[string]bool{}
		for _, d := range dests {
			seen[d] = true
			switch {
			case watched[d] && open[d] != "":
				t.Errorf("%s: %s is watched AND named an opening (%s) — a destination is one or the other, and the openings are not watched because a detached one leaves the seat with less and never more", c.what, d, open[d])
			case watched[d], open[d] != "":
			default:
				t.Errorf("%s: the render mounts over %s and neither watches it nor names it an opening. If it is a WALL, give it a watch() call in bwrapArgsInto; if it re-opens something, say so in wwOpenings. An unclassified mount is a seat whose wall a host rename detaches with nothing watching (ranger-base-w6ip0).\nargv: %q", c.what, d, argv)
			}
		}
		for _, w := range watch {
			if !seen[w.Path] {
				t.Errorf("%s: %s is watched as a %s and the argv mounts nothing over it — `posse wall-watch` would then read the HOST's mount at that name as the seat's wall, and a bead filed off it names a wall that was never put up.\nargv: %q", c.what, w.Path, w.Kind, argv)
			}
		}
		// BwrapArgs's own output is unchanged by the third one.
		a1, p1, err := BwrapArgs(c.cwd, c.writable, c.carve, c.refKeep...)
		if err != nil {
			t.Fatalf("%s: %v", c.what, err)
		}
		if !reflect.DeepEqual(a1, argv) || !reflect.DeepEqual(p1, prep) {
			t.Errorf("%s: BwrapArgs and bwrapArgsWatched rendered different walls", c.what)
		}
	}
}

// The derivation above is only as total as the inputs it is driven with, so
// the inputs are held total too: every field of SeatbeltCarveOut has a value
// in one of the renders here. A field added to that struct reds this until
// somebody gives it one — at which point the test above says whether its
// mount is a wall or an opening, which is the question the hand-written
// literal let an author skip.
func TestWallWatchFixturesCarryEveryCarveOutField(t *testing.T) {
	t.Parallel()
	cs := wwRenders(t)
	rt := reflect.TypeOf(SeatbeltCarveOut{})
	for i := 0; i < rt.NumField(); i++ {
		set := false
		for _, c := range cs {
			if !reflect.ValueOf(c.carve).Field(i).IsZero() {
				set = true
			}
		}
		if !set {
			t.Errorf("SeatbeltCarveOut.%s has no value in any render here, so TestWallWatchEveryRenderedMountIsAWallOrANamedOpening says nothing about the mount it makes. Give it one in wwEveryShape (or its own render, as carve.Tmp has), and classify what it mounts: a watch() call in bwrapArgsInto, or an entry in wwOpenings", rt.Field(i).Name)
		}
	}
	// The two inputs that are parameters rather than fields.
	one := cs[0]
	if len(one.writable) == 0 || len(one.refKeep) == 0 {
		t.Errorf("%s must carry a write grant and a D20 ref keep: the grant is the opening the rule is stated against, and the keep is the one input that renders NO mount", one.what)
	}
}

// The render records its WALLS and only its walls: Hide, Private, Masked,
// Deny and step 5's read masks (ranger-base-y4cm6) are watched; a write
// grant and the ssh allow bind-backs (openings) are not. This is the KINDS
// and the ORDER, which the derivation above does not read — the derivation
// holds the set, this holds what each member is called and when it is
// mounted. Both are needed: a wall recorded under the wrong kind is a bead
// that names the wrong wall.
func TestWallWatchRenderRecordsEveryWallAndNoOpening(t *testing.T) {
	t.Parallel()
	c := wwEveryShape(t)
	grant := c.writable[0]
	root := filepath.Dir(grant)
	sshDir := c.carve.SSHDir[0]
	_, _, watch, err := bwrapArgsWatched(c.cwd, c.writable, c.carve, c.refKeep...)
	if err != nil {
		t.Fatal(err)
	}
	want := []WallWatchMount{
		{WallWatchHide, filepath.Join(root, "run")},
		{WallWatchPrivate, filepath.Join(grant, "shell-snapshots")},
		// shallowestFirst sorts the deny list, so gates, policy-cache.json,
		// settings-seeded.json, settings.json — the mask under its own kind.
		{WallWatchDeny, filepath.Join(grant, "gates")},
		{WallWatchMasked, filepath.Join(grant, "policy-cache.json")},
		{WallWatchDeny, filepath.Join(grant, "settings-seeded.json")},
		{WallWatchDeny, filepath.Join(grant, "settings.json")},
		{WallWatchReadMask, sshDir},
		{WallWatchReadMask, filepath.Join(root, "state", "rex", "tsnet")},
		{WallWatchReadMask, filepath.Join(root, "sibling", "credentials.json")},
	}
	if !reflect.DeepEqual(watch, want) {
		t.Errorf("watched walls =\n %v\nwant\n %v\n(the write grant %s is an opening and must not be watched)", watch, want, grant)
	}
	// The bind-backs under the ssh tmpfs are the holes deliberately left in
	// that wall, and the key under it renders no mount at all, so none of
	// the three is watched.
	for _, p := range []string{"config", "known_hosts", "id_ed25519"} {
		if wallWatchListed(watch, filepath.Join(sshDir, p)) {
			t.Errorf("%s is watched: it is an opening under the %s tmpfs, not a wall of its own", p, sshDir)
		}
	}
	// And the D20 ref keep, which renders no mount: watching it would be the
	// other direction of the same silence.
	if wallWatchListed(watch, c.refKeep[0]) {
		t.Errorf("%s is watched and the render mounts nothing over it", c.refKeep[0])
	}
}

func TestWallWatchMountsFileRoundTripsAPathWithANewline(t *testing.T) {
	t.Parallel()
	in := []WallWatchMount{{WallWatchDeny, "/nonexistent/a\nb"}, {WallWatchHide, "/nonexistent/c d"}}
	got, err := parseWallWatchMounts(wallWatchMountsBytes(in))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, in) {
		t.Errorf("round trip = %v, want %v", got, in)
	}
	for _, bad := range []string{"deny\x00/x", "deny\x00", "deny\x00rel\x00"} {
		if _, err := parseWallWatchMounts([]byte(bad)); err == nil {
			t.Errorf("parseWallWatchMounts(%q) accepted a file posse did not write", bad)
		}
	}
}

func TestWallWatchMountsFileNamesTheResolvedPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	real := filepath.Join(root, "real")
	if err := os.MkdirAll(filepath.Join(real, "d"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	got, _ := parseWallWatchMounts(wallWatchMountsBytes([]WallWatchMount{{WallWatchDeny, filepath.Join(link, "d")}}))
	want, _ := filepath.EvalSymlinks(filepath.Join(real, "d"))
	if len(got) != 1 || got[0].Path != want {
		t.Errorf("mounts file names %v, want the path the kernel spells: %s", got, want)
	}
}

func TestWallWatchPrefixStartsTheWatcherInFrontOfBwrap(t *testing.T) {
	t.Parallel()
	got := WallWatchPrefix("/opt/posse", "/h/gates/dev/bwrap.mounts", "dev", "/w/s", "/usr/bin/bwrap", "/h/gates/dev/bwrap.args")
	want := "3<'/h/gates/dev/bwrap.args' '/opt/posse' wall-watch --mounts '/h/gates/dev/bwrap.mounts' --seat 'dev' --dir '/w/s' -- '/usr/bin/bwrap' --args 3 -- "
	if got != want {
		t.Errorf("WallWatchPrefix =\n %q\nwant\n %q", got, want)
	}
	o, err := parseWallWatchArgv([]string{"posse", WallWatchVerb, "--mounts", "/m", "--seat", "dev", "--dir", "/w", "--", "/usr/bin/bwrap", "--args", "3", "--", "claude"})
	if err != nil {
		t.Fatal(err)
	}
	if o.Mounts != "/m" || o.Seat != "dev" || o.Dir != "/w" || !reflect.DeepEqual(o.Bwrap, []string{"/usr/bin/bwrap", "--args", "3", "--", "claude"}) {
		t.Errorf("parsed %+v", o)
	}
	if got := wallWatchBwrapArgv(o.Bwrap); !reflect.DeepEqual(got, []string{"/usr/bin/bwrap", "--info-fd", "4", "--block-fd", "5", "--args", "3", "--", "claude"}) {
		t.Errorf("bwrap argv = %q: the info and block fds go before the rendered options", got)
	}
	for _, bad := range [][]string{
		{"posse", WallWatchVerb},
		{"posse", WallWatchVerb, "--mounts", "/m"},
		{"posse", WallWatchVerb, "--", "/usr/bin/bwrap"},
		{"posse", WallWatchVerb, "--bogus", "x", "--mounts", "/m", "--", "b"},
	} {
		if _, err := parseWallWatchArgv(bad); err == nil {
			t.Errorf("parseWallWatchArgv(%q) accepted it", bad)
		}
	}
}

// A bwrap render writes the mounts file beside the args, and its Launch line
// is the watcher in front of the same binary and args; Prefix (what a probe
// types) is unchanged.
func TestWallWatchBwrapRenderWritesTheMountsFileAndTheLaunchLine(t *testing.T) {
	// No t.Parallel: bwlStubBin and wallWatchBin are package-level vars.
	f := bwlFixture(t)
	bwlStubBin(t, "/usr/bin/bwrap")
	old := wallWatchBin
	wallWatchBin = func() (string, error) { return "/opt/posse", nil }
	t.Cleanup(func() { wallWatchBin = old })
	w, err := f.a.renderWallOn(WallBwrap, f.ag, f.rt, f.dir)
	if err != nil {
		t.Fatal(err)
	}
	mounts := filepath.Join(filepath.Dir(w.Artifact), bwrapMountsName)
	if want := WallWatchPrefix("/opt/posse", mounts, f.ag.Name, f.dir, "/usr/bin/bwrap", w.Artifact); w.Launch != want {
		t.Errorf("WallRender.Launch =\n %q\nwant\n %q", w.Launch, want)
	}
	if w.Prefix != BwrapPrefix("/usr/bin/bwrap", w.Artifact) {
		t.Errorf("WallRender.Prefix changed: %q — probes type it, and they run no watcher", w.Prefix)
	}
	r, err := f.a.bwrapRender(f.ag, f.rt, f.dir, f.a.GatesDir(f.ag.Name))
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(mounts)
	if err != nil {
		t.Fatalf("the render wrote no mounts file: %v", err)
	}
	if !reflect.DeepEqual(b, wallWatchMountsBytes(r.Watch)) {
		t.Errorf("%s is not the render's own wall list", mounts)
	}
	if len(r.Watch) == 0 {
		t.Error("an ordinary claude seat renders no wall to watch — its Deny binds alone should be there")
	}
}

func TestWallWatchMountinfoDiff(t *testing.T) {
	t.Parallel()
	mi := "36 35 98:0 / / ro,relatime master:1 - ext4 /dev/root rw\n" +
		"40 36 0:33 / /h/cfg rw - ext4 /dev/root rw\n" +
		"41 40 0:33 / /h/cfg/settings.json ro - ext4 /dev/root rw\n" +
		"42 40 0:5 / /h/with\\040space ro - tmpfs tmpfs rw\n" +
		"43 40 0:5 / /h/back\\134slash ro - tmpfs tmpfs rw\n"
	live := parseMountinfo([]byte(mi))
	for _, p := range []string{"/", "/h/cfg", "/h/cfg/settings.json", "/h/with space", `/h/back\slash`} {
		if len(live[p]) == 0 {
			t.Errorf("mountinfo parse lost %q (got %v)", p, live)
		}
	}
	watch := []WallWatchMount{
		{WallWatchDeny, "/h/cfg/settings.json"},
		{WallWatchPrivate, "/h/cfg/shell-snapshots"},
		{WallWatchHide, "/h/with space"},
		{WallWatchMasked, "/h/cfg/shell-snapshots"},
	}
	base, missing := wallWatchBaseline(watch, live)
	if !reflect.DeepEqual(missing, []WallWatchMount{{WallWatchPrivate, "/h/cfg/shell-snapshots"}}) {
		t.Errorf("missing at launch = %v, want the one path that is not in the table, once", missing)
	}
	if got := wallWatchDetached(watch, base, live); len(got) != 0 {
		t.Errorf("an unchanged table reads as detached: %v", got)
	}
	// The deny's mount leaves.
	gone := parseMountinfo([]byte(strings.Replace(mi, "41 40 0:33 / /h/cfg/settings.json ro - ext4 /dev/root rw\n", "", 1)))
	if got := wallWatchDetached(watch, base, gone); !reflect.DeepEqual(got, []WallWatchMount{{WallWatchDeny, "/h/cfg/settings.json"}}) {
		t.Errorf("detached = %v, want the deny", got)
	}
}

// A wall stacked over another mount at the same point is watched by its own
// id: when it leaves, the lower mount still at that point does not hide it.
func TestWallWatchStackedWallIsWatchedByItsId(t *testing.T) {
	t.Parallel()
	both := "20 1 0:1 / /tmp rw - tmpfs tmpfs rw\n50 20 0:9 / /tmp rw - ext4 /dev/root rw\n"
	watch := []WallWatchMount{{WallWatchHide, "/tmp"}}
	base, missing := wallWatchBaseline(watch, parseMountinfo([]byte(both)))
	if len(missing) != 0 {
		t.Fatal(missing)
	}
	lower := parseMountinfo([]byte("20 1 0:1 / /tmp rw - tmpfs tmpfs rw\n"))
	if got := wallWatchDetached(watch, base, lower); len(got) != 1 {
		t.Errorf("the top mount at /tmp left and the diff saw nothing: a point-only compare reads the lower mount as the wall")
	}
}

// fakeWallWatchStore is an in-memory bead store for D2's dedupe.
type fakeWallWatchStore struct {
	issues   []BdIssue
	comments map[string][]string
	readErr  error
	n        int
	now      func() time.Time
}

func (s *fakeWallWatchStore) OpenLabeledAny(dir string, labels ...string) ([]BdIssue, error) {
	if s.readErr != nil {
		return nil, s.readErr
	}
	return append([]BdIssue(nil), s.issues...), nil
}

func (s *fakeWallWatchStore) Create(dir string, n BdNew) (string, error) {
	s.n++
	id := "t-" + string(rune('a'+s.n-1))
	created := time.Now()
	if s.now != nil {
		created = s.now()
	}
	pri := 0
	if n.Priority == "1" {
		pri = 1
	}
	s.issues = append(s.issues, BdIssue{ID: id, Title: n.Title, Description: n.Description, Labels: n.Labels, Priority: pri, IssueType: n.Type, Created: created, Status: "open"})
	return id, nil
}

func (s *fakeWallWatchStore) Comment(dir, id, text, actor string) error {
	if s.comments == nil {
		s.comments = map[string][]string{}
	}
	s.comments[id] = append(s.comments[id], text)
	return nil
}

func TestWallWatchFilesOneP1PerPathAndCommentsInsideTenMinutes(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	clock := t0
	st := &fakeWallWatchStore{now: func() time.Time { return clock }}
	settings := WallWatchMount{WallWatchDeny, "/h/.claude/settings.json"}
	snaps := WallWatchMount{WallWatchPrivate, "/h/.claude/shell-snapshots"}
	det := func(seat string, ms ...WallWatchMount) wallWatchDetach {
		return wallWatchDetach{Seat: seat, Dir: "/w/" + seat, BwrapPid: 10, SeatPid: 11, At: clock, Missing: ms, Killed: []int{10, 11}}
	}

	filed, commented, err := fileWallWatchDetach(st, det("a", settings), clock, "")
	if err != nil || filed == "" || len(commented) != 0 {
		t.Fatalf("first detection: filed=%q commented=%v err=%v", filed, commented, err)
	}
	is := st.issues[0]
	if is.Priority != 1 || is.IssueType != "bug" || !reflect.DeepEqual(is.Labels, []string{"security"}) {
		t.Errorf("bead is P%d %s %v, want a P1 security bug", is.Priority, is.IssueType, is.Labels)
	}
	for _, want := range []string{"seat: a", settings.Path, "detached at: 2026-11-13T10:00:00Z", "killed: pids 10 11", wallWatchMarker(settings.Path)} {
		if !strings.Contains(is.Description, want) {
			t.Errorf("description lacks %q:\n%s", want, is.Description)
		}
	}

	// Another seat, same path, five minutes on: a comment, no bead.
	clock = t0.Add(5 * time.Minute)
	filed, commented, err = fileWallWatchDetach(st, det("b", settings), clock, "")
	if err != nil || filed != "" || !reflect.DeepEqual(commented, []string{is.ID}) || len(st.issues) != 1 {
		t.Fatalf("same path inside ten minutes: filed=%q commented=%v err=%v beads=%d", filed, commented, err, len(st.issues))
	}
	if c := st.comments[is.ID]; len(c) != 1 || !strings.Contains(c[0], "seat b") {
		t.Errorf("comment = %q, want one naming seat b", c)
	}

	// Both paths at once: a comment for the filed one, a bead for the new one.
	clock = t0.Add(6 * time.Minute)
	filed, commented, err = fileWallWatchDetach(st, det("c", settings, snaps), clock, "")
	if err != nil || filed == "" || len(commented) != 1 || len(st.issues) != 2 {
		t.Fatalf("mixed: filed=%q commented=%v err=%v beads=%d", filed, commented, err, len(st.issues))
	}
	if d := st.issues[1].Description; strings.Contains(d, wallWatchMarker(settings.Path)) || !strings.Contains(d, wallWatchMarker(snaps.Path)) {
		t.Errorf("the new bead must carry only the new path's marker:\n%s", d)
	}

	// Past the window: a new bead for the old path.
	clock = t0.Add(11 * time.Minute)
	filed, _, err = fileWallWatchDetach(st, det("d", settings), clock, "")
	if err != nil || filed == "" || len(st.issues) != 3 {
		t.Fatalf("past ten minutes: filed=%q err=%v beads=%d", filed, err, len(st.issues))
	}
}

// A store that cannot be read still gets the bead, and the error says the
// dedupe was skipped.
func TestWallWatchUnreadableStoreStillFiles(t *testing.T) {
	t.Parallel()
	st := &fakeWallWatchStore{readErr: errors.New("locked")}
	filed, _, err := fileWallWatchDetach(st, wallWatchDetach{Seat: "a", Missing: []WallWatchMount{{WallWatchDeny, "/x"}}}, time.Now(), filepath.Join(t.TempDir(), "l", "wall-watch.lock"))
	if filed == "" || len(st.issues) != 1 {
		t.Fatal("an unreadable store filed nothing — a lost wall with no record")
	}
	if err == nil || !strings.Contains(err.Error(), "not deduped") {
		t.Errorf("err = %v, want it to say the dedupe was skipped", err)
	}
}
