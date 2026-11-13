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

// The render records its WALLS and only its walls: Hide, Private, Masked,
// Deny and step 5's read masks (ranger-base-y4cm6) are watched; a write
// grant and the ssh allow bind-backs (openings) are not.
func TestWallWatchRenderRecordsEveryWallAndNoOpening(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	grant := filepath.Join(root, "cfg")
	priv := filepath.Join(grant, "shell-snapshots")
	src := filepath.Join(root, "seat", "shell-snapshots")
	deny := filepath.Join(grant, "settings.json")
	mask := filepath.Join(grant, "policy-cache.json")
	hide := filepath.Join(root, "run")
	sshDir := filepath.Join(root, ".ssh")
	sshCfg := filepath.Join(sshDir, "config")
	sshHosts := filepath.Join(sshDir, "known_hosts")
	sshKey := filepath.Join(sshDir, "id_ed25519")
	cred := filepath.Join(root, "sibling", "credentials.json")
	for _, d := range []string{grant, priv, src, hide, sshDir, filepath.Dir(cred)} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{deny, mask, sshCfg, sshHosts, sshKey, cred} {
		if err := os.WriteFile(f, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	carve := SeatbeltCarveOut{
		Hide:     []string{hide},
		Deny:     []string{deny, mask},
		DenyFile: []string{deny, mask},
		Mask:     []string{mask},
		Private:  []PrivateBind{{Src: src, Dst: priv}},
		SSHDir:   []string{sshDir},
		DenyRead: []string{cred, sshKey},
	}
	_, _, watch, err := bwrapArgsWatched("", []string{grant}, carve)
	if err != nil {
		t.Fatal(err)
	}
	want := []WallWatchMount{
		{WallWatchHide, hide},
		{WallWatchPrivate, priv},
		{WallWatchMasked, mask},
		{WallWatchDeny, deny},
		{WallWatchReadMask, sshDir},
		{WallWatchReadMask, cred},
	}
	if !reflect.DeepEqual(watch, want) {
		t.Errorf("watched walls =\n %v\nwant\n %v\n(the write grant %s is an opening and must not be watched)", watch, want, grant)
	}
	// The bind-backs under the ssh tmpfs are the holes deliberately left in
	// that wall, and the key under it renders no mount at all, so none of
	// the three is watched.
	for _, p := range []string{sshCfg, sshHosts, sshKey} {
		if wallWatchListed(watch, p) {
			t.Errorf("%s is watched: it is an opening under the %s tmpfs, not a wall of its own", p, sshDir)
		}
	}
	// BwrapArgs's own output is unchanged by the third one.
	a1, p1, _ := BwrapArgs("", []string{grant}, carve)
	a2, p2, _, _ := bwrapArgsWatched("", []string{grant}, carve)
	if !reflect.DeepEqual(a1, a2) || !reflect.DeepEqual(p1, p2) {
		t.Error("BwrapArgs and bwrapArgsWatched rendered different walls")
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
