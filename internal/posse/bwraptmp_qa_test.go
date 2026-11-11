//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-7wdb7: the bwrap wall binds the seat's own directory
// over /tmp, so no walled seat writes, or reads, the host's shared /tmp.
//
// The render arms run on every host, against fixture trees. The live arm
// skips without Linux and bwrap, and carries a control that reaches the
// host's /tmp without the mount, so it measures the wall and not the probe.

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The private /tmp mounts after the base and the hides and before every
// grant; a grant of /tmp itself is what it replaces, so it is not rendered;
// and a deny under /tmp that no grant re-opened renders nothing, because a
// bind from the host is what the mount keeps out.
func TestBwrapArgsBindsTheSeatTmpOverTheHosts(t *testing.T) {
	t.Parallel()
	root, bwAt := bwTree(t, []string{"proj", "run"}, nil)
	seatTmp := bwAt("seat", "tmp")
	underTmp := filepath.Join(bwrapTmpMount, "posse-7wdb7-never-there", "deny")

	argv, prep, err := BwrapArgs(bwAt("proj"),
		[]string{bwAt("proj"), bwrapTmpMount},
		SeatbeltCarveOut{
			Deny: []string{underTmp},
			Hide: []string{bwAt("run")},
			Tmp:  seatTmp,
		})
	if err != nil {
		t.Fatal(err)
	}
	line := strings.ReplaceAll(bwJoin(argv), root, "R")

	mount := bwIndex(argv, "--bind", seatTmp, bwrapTmpMount)
	if mount < 0 {
		t.Fatalf("no bind of the seat's tmp over %s:\n  %s", bwrapTmpMount, line)
	}
	if hide := bwIndex(argv, "--tmpfs", bwAt("run")); hide < 0 || hide > mount {
		t.Fatalf("the private tmp must follow the base and the hides:\n  %s", line)
	}
	if g := bwIndex(argv, "--bind", bwAt("proj"), bwAt("proj")); g < mount {
		t.Fatalf("a grant must mount AFTER the private tmp, or a grant under /tmp is buried:\n  %s", line)
	}
	if bwIndex(argv, "--bind", bwrapTmpMount, bwrapTmpMount) >= 0 {
		t.Fatalf("the host's /tmp is still granted on top of the seat's:\n  %s", line)
	}
	if strings.Contains(bwJoin(argv), underTmp) {
		t.Fatalf("a deny under /tmp binds the host's path back into the seat's tmp:\n  %s", line)
	}
	if got := bwPrep(prep); len(got) != 1 || !strings.Contains(got[0], seatTmp) {
		t.Fatalf("prep = %v, want exactly the mkdir of the seat's tmp %s", got, seatTmp)
	}
}

func TestBwrapArgsRefusesASeatTmpThatIsAFile(t *testing.T) {
	t.Parallel()
	_, bwAt := bwTree(t, []string{"proj"}, []string{"seat-tmp"})
	_, _, err := BwrapArgs(bwAt("proj"), nil, SeatbeltCarveOut{Tmp: bwAt("seat-tmp")})
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("err = %v, want a refusal naming the file", err)
	}
}

// The producer: one dir per session dir, under its seat cache (so the
// worktree remove that takes the cache takes it), and nothing for a home or
// session dir that is itself under /tmp.
func TestBwrapSeatTmpIsTheSeatsOwnAndStandsDownUnderTmp(t *testing.T) {
	t.Parallel()
	home, cwd := "/home/someone", "/home/someone/src/repo"
	got := bwrapSeatTmp(home, cwd)
	if want := filepath.Join(SeatCacheDir(home, cwd), "tmp"); got != want {
		t.Fatalf("bwrapSeatTmp = %q, want %q", got, want)
	}
	if other := bwrapSeatTmp(home, "/home/someone/src/other"); other == got {
		t.Fatalf("two session dirs share one private tmp %q", got)
	}
	for _, c := range [][2]string{
		{"", cwd}, {home, ""},
		{"/tmp/h", "/tmp/h/src/repo"},
		{home, "/tmp/scratch/repo"},
	} {
		if g := bwrapSeatTmp(c[0], c[1]); g != "" {
			t.Errorf("bwrapSeatTmp(%q, %q) = %q, want none", c[0], c[1], g)
		}
	}
}

// The set: /tmp goes, a TMPDIR under it goes (it would bind a shared host
// dir back on top), and a named grant under /tmp that is not the TMPDIR
// stays — it is a PID's deliberate `writable:` extra.
func TestBwrapDropTmpGrantsTakesTmpAndATmpdirUnderIt(t *testing.T) {
	t.Parallel()
	set := []string{"/home/h/src/repo", "/tmp", "/tmp/launcher-tmp", "/tmp/named-extra"}
	got := strings.Join(bwrapDropTmpGrants(set, "/tmp/launcher-tmp"), " ")
	if got != "/home/h/src/repo /tmp/named-extra" {
		t.Fatalf("bwrapDropTmpGrants = %q", got)
	}
	got = strings.Join(bwrapDropTmpGrants(set, "/home/h/tmp"), " ")
	if got != "/home/h/src/repo /tmp/launcher-tmp /tmp/named-extra" {
		t.Fatalf("a TMPDIR outside /tmp drops only /tmp itself: %q", got)
	}
}

// ─── live ────────────────────────────────────────────────────────────────────

// btLive plants a file in the HOST's /tmp — the shape of an uncaged build's
// WORK dir — and returns a seat tmp dir of its own.
func btLive(t *testing.T) (plant, seatTmp string) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("bwrap is the Linux mechanism of the seatbelt tier (ADR 0080 D16)")
	}
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap is not on PATH: " + err.Error())
	}
	work, err := os.MkdirTemp(bwrapTmpMount, "bwt7wdb7-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(work) })
	plant = filepath.Join(work, "b001.a")
	if err := os.WriteFile(plant, []byte("object"), 0o600); err != nil {
		t.Fatal(err)
	}
	return plant, filepath.Join(t.TempDir(), "seat-tmp")
}

// btProbe runs, inside the wall, a read and an overwrite of the plant and a
// create of a fresh name under /tmp, and prints what each did.
func btProbe(t *testing.T, argv []string, plant, fresh string) map[string]string {
	t.Helper()
	script := `if [ -e "$1" ]; then echo plant=present; else echo plant=absent; fi
if echo evil > "$1" 2>/dev/null; then echo overwrite=ok; else echo overwrite=refused; fi
if echo x > "$2" 2>/dev/null; then echo create=ok; else echo create=refused; fi
echo "tmpdir=${TMPDIR-unset}"`
	args := append(append([]string{}, argv...), "sh", "-c", script, "sh", plant, fresh)
	out, err := exec.Command("bwrap", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("bwrap %v: %v\n%s", argv, err, out)
	}
	res := map[string]string{}
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if k, v, ok := strings.Cut(l, "="); ok {
			res[k] = v
		}
	}
	return res
}

// The fix: the host's /tmp is not in the seat's view at all, so the plant
// can be neither read nor replaced; the seat's own writes to /tmp land in
// its own dir and not on the host.
func TestLiveBwrapSeatTmpKeepsTheSeatOutOfTheHostsTmp(t *testing.T) {
	t.Parallel()
	plant, seatTmp := btLive(t)
	fresh := filepath.Join(bwrapTmpMount, "bwt7wdb7-fresh-"+filepath.Base(filepath.Dir(plant)))
	t.Cleanup(func() { _ = os.Remove(fresh) })

	argv, prep, err := BwrapArgs("", []string{bwrapTmpMount}, SeatbeltCarveOut{Tmp: seatTmp})
	if err != nil {
		t.Fatal(err)
	}
	if err := runBwrapPrep(prep); err != nil {
		t.Fatal(err)
	}
	got := btProbe(t, argv, plant, fresh)
	if got["plant"] != "absent" || got["overwrite"] != "refused" {
		t.Errorf("the host's /tmp plant inside the wall: %v, want absent and refused", got)
	}
	if got["create"] != "ok" {
		t.Errorf("the seat cannot write its own /tmp: %v", got)
	}
	if b, _ := os.ReadFile(plant); string(b) != "object" {
		t.Errorf("the plant on the host reads %q after the seat ran", b)
	}
	if _, err := os.Stat(fresh); !os.IsNotExist(err) {
		t.Errorf("the seat's /tmp write reached the host's /tmp at %s: %v", fresh, err)
	}
	if _, err := os.Stat(filepath.Join(seatTmp, filepath.Base(fresh))); err != nil {
		t.Errorf("the seat's /tmp write is not in its own dir: %v", err)
	}
}

// The control: the same plant and probe with /tmp granted as it was before
// this bead. Without this arm the one above passes on a probe that reaches
// nothing.
func TestLiveBwrapWithoutSeatTmpReachesTheHostsTmp(t *testing.T) {
	t.Parallel()
	plant, _ := btLive(t)
	fresh := filepath.Join(filepath.Dir(plant), "fresh")
	argv, _, err := BwrapArgs("", []string{bwrapTmpMount}, SeatbeltCarveOut{})
	if err != nil {
		t.Fatal(err)
	}
	got := btProbe(t, argv, plant, fresh)
	if got["plant"] != "present" || got["overwrite"] != "ok" {
		t.Errorf("control: the host's /tmp plant inside the old wall: %v, want present and overwritten", got)
	}
}
