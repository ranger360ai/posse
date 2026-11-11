//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for ranger-base-ludpp: the bwrap wall hides the user runtime dir and
// takes the bus and agent addresses out of a seat's environment.
//
// THE CLASS. `--ro-bind / /` stops writes, and a read-only mount does not
// stop connect(2) on a socket or a write-open of a FIFO. The user runtime dir
// is where a desktop session keeps endpoints of exactly that kind, several of
// which act outside the cage. So the render puts an empty tmpfs over the
// whole directory (an allowlist: what a seat needs back is a write grant,
// which mounts later) and unsets the three addresses.
//
// The render arms run on every host, against fixture trees. The live arm
// skips without Linux and bwrap, and builds its own runtime dir with its own
// listening sockets, so it measures the wall and not this box's desktop.

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// bwIndex is the position of the first occurrence of the run seq in argv, or
// -1. Order is the semantics at this mechanism, so the arms below ask WHERE
// a mount is, not only whether it is.
func bwIndex(argv []string, seq ...string) int {
	for i := 0; i+len(seq) <= len(argv); i++ {
		match := true
		for j, s := range seq {
			if argv[i+j] != s {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// The tmpfs lands after the base and before every grant, so a grant inside
// the hidden dir is the one way back in. A deny or a Keep inside it that no
// grant re-opened renders NOTHING: both are binds from the host, and a
// read-only bind of a socket is a socket a seat can still connect to.
func TestBwrapArgsHidesTheRuntimeDirUnderEveryGrant(t *testing.T) {
	t.Parallel()
	root, bwAt := bwTree(t,
		[]string{"proj/agents", "run/bus.d", "run/allowed", "run/gates"},
		[]string{"run/gates/refusals.log"})
	cwd := bwAt("proj")

	argv, prep, err := BwrapArgs(cwd,
		[]string{bwAt("proj"), bwAt("run", "allowed")},
		SeatbeltCarveOut{
			Deny: []string{bwAt("proj", "agents"), bwAt("run", "gates"), bwAt("run", "allowed", "deny")},
			Keep: []string{bwAt("run", "gates", "refusals.log")},
			Hide: []string{bwAt("run"), bwAt("run"), bwAt("not-there")},
		})
	if err != nil {
		t.Fatal(err)
	}
	line := strings.ReplaceAll(bwJoin(argv), root, "R")

	tmpfs := bwIndex(argv, "--tmpfs", bwAt("run"))
	if tmpfs < 0 {
		t.Fatalf("no tmpfs over the runtime dir:\n  %s", line)
	}
	if strings.Count(line, "--tmpfs") != 1 {
		t.Fatalf("one hidden dir, given twice and once absent, must be ONE mount:\n  %s", line)
	}
	if base := bwIndex(argv, "--ro-bind", "/", "/"); base < 0 || base > tmpfs {
		t.Fatalf("the tmpfs must come after the read-only base, or the base buries it:\n  %s", line)
	}
	for _, g := range []string{bwAt("proj"), bwAt("run", "allowed")} {
		if i := bwIndex(argv, "--bind", g, g); i < tmpfs {
			t.Fatalf("grant %s at %d, tmpfs at %d: a grant must mount AFTER the tmpfs, or it is buried:\n  %s", g, i, tmpfs, line)
		}
	}
	// A deny inside a grant inside the hidden dir is still a deny: the grant
	// re-opened that subtree, so the narrower statement has to follow it.
	if bwIndex(argv, "--ro-bind", bwAt("proj", "agents"), bwAt("proj", "agents")) < 0 {
		t.Fatalf("a deny outside the hidden dir was dropped:\n  %s", line)
	}
	for _, gone := range []string{bwAt("run", "gates"), bwAt("run", "gates", "refusals.log")} {
		if strings.Contains(bwJoin(argv), " "+gone+" ") {
			t.Fatalf("%s is inside the hidden dir and no grant, so binding it brings the host's copy back in:\n  %s", gone, line)
		}
	}
	for _, s := range prep {
		if strings.HasPrefix(s.Path, bwAt("run", "gates")) {
			t.Fatalf("the launcher was asked to make %s, which the render then never binds", s)
		}
	}
	for _, k := range bwrapUnsetEnv {
		if i := bwIndex(argv, "--unsetenv", k); i < 0 {
			t.Fatalf("%s is not unset:\n  %s", k, line)
		}
	}
}

// A file where a directory is expected refuses rather than renders: a tmpfs
// cannot go over it, and a wall rendered without the mount would say
// nothing about the hole.
func TestBwrapArgsRefusesToHideAFile(t *testing.T) {
	t.Parallel()
	_, bwAt := bwTree(t, []string{"proj"}, []string{"run"})
	_, _, err := BwrapArgs(bwAt("proj"), nil, SeatbeltCarveOut{Hide: []string{bwAt("run")}})
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("err = %v, want a refusal naming the file", err)
	}
}

// The producer: both spellings of the runtime dir, resolved and deduped;
// nothing for a directory that is not there; a refusal for a value that
// would blind the seat.
func TestBwrapRuntimeDirsResolvesAndRefusesWhatWouldBlindTheSeat(t *testing.T) {
	t.Parallel()
	root, bwAt := bwTree(t, []string{"xdg", "home"}, nil)
	if err := os.Symlink(bwAt("xdg"), bwAt("link")); err != nil {
		t.Fatal(err)
	}
	const noUID = 2147483000 // no /run/user/<this> on any host

	got, err := bwrapRuntimeDirs(bwAt("link"), noUID, bwAt("home"))
	if err != nil || strings.Join(got, " ") != bwAt("xdg") {
		t.Fatalf("symlinked XDG_RUNTIME_DIR = %v, %v; want [%s]", got, err, bwAt("xdg"))
	}
	if got, err := bwrapRuntimeDirs("", noUID, bwAt("home")); err != nil || len(got) != 0 {
		t.Fatalf("no variable and no /run/user dir = %v, %v; want nothing hidden and no error", got, err)
	}
	if got, err := bwrapRuntimeDirs(bwAt("missing"), noUID, ""); err != nil || len(got) != 0 {
		t.Fatalf("an XDG_RUNTIME_DIR that is not there = %v, %v; want nothing", got, err)
	}
	for _, c := range []struct{ xdg, home, want string }{
		{"run/user/1", "", "absolute"},
		{"/", "", "top-level"},
		{"/usr", "", "top-level"},
		{root, bwAt("home"), "holds the home"},
	} {
		if _, err := bwrapRuntimeDirs(c.xdg, noUID, c.home); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("XDG_RUNTIME_DIR=%q home=%q: err = %v, want a refusal naming %q", c.xdg, c.home, err, c.want)
		}
	}
	if runtime.GOOS == "linux" {
		// The /run/user/<uid> spelling, found with no variable at all, which is
		// how a launcher started outside the desktop session sees it.
		own := filepath.Join("/run/user", strconv.Itoa(os.Getuid()))
		if fi, err := os.Stat(own); err == nil && fi.IsDir() {
			got, err := bwrapRuntimeDirs("", os.Getuid(), bwAt("home"))
			if err != nil || len(got) != 1 || got[0] != absResolve(own) {
				t.Fatalf("no variable on a host with %s = %v, %v", own, got, err)
			}
		}
	}
}

// ─── live ────────────────────────────────────────────────────────────────────

// bhLive is a runtime dir shaped like the one ranger-base-x9f3s measured:
// a session bus, the service manager's private socket, the multiplexer's
// server, an agent socket, and a launcher FIFO. Every socket LISTENS, so the
// control arm can show that the cage really reached it.
type bhLive struct {
	dir     string
	sockets []string
	fifo    string
}

func bhLiveFixture(t *testing.T) bhLive {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("bwrap is the Linux mechanism of the seatbelt tier (ADR 0080 D16)")
	}
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap is not on PATH: " + err.Error())
	}
	// Short, and not t.TempDir(): a unix socket path is limited to 108 bytes
	// and a test-named temp dir eats most of them.
	dir, err := os.MkdirTemp("", "bwh")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	f := bhLive{dir: absResolve(dir)}
	for _, rel := range []string{"bus", "systemd/private", "rex/server.sock", "gnupg/S.gpg-agent.ssh"} {
		p := filepath.Join(f.dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		l, err := net.Listen("unix", p)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = l.Close() })
		f.sockets = append(f.sockets, p)
	}
	f.fifo = filepath.Join(f.dir, "uwsm-app-daemon-in")
	if err := syscall.Mkfifo(f.fifo, 0o644); err != nil {
		t.Fatal(err)
	}
	return f
}

// probe runs, inside the wall, one line per endpoint: whether the path is
// there and — where python3 is on the host — whether a connect(2) to it
// succeeds. The connect is the claim; the path test is what still answers
// on a host without python3, and an absent path is a connect that gets
// ENOENT.
func (f bhLive) probe(t *testing.T, argv []string) map[string]string {
	t.Helper()
	script := `for p in "$@"; do
  if [ -e "$p" ]; then st=present; else st=absent; fi
  if [ -S "$p" ] && command -v python3 >/dev/null 2>&1; then
    if python3 -I -c 'import socket,sys; s=socket.socket(socket.AF_UNIX); s.connect(sys.argv[1])' "$p" 2>/dev/null; then st="$st,connected"; else st="$st,refused"; fi
  fi
  echo "$p=$st"
done
echo "DBUS=${DBUS_SESSION_BUS_ADDRESS-unset}"
echo "SSH=${SSH_AUTH_SOCK-unset}"`
	args := append(append([]string{}, argv...), "sh", "-c", script, "sh")
	args = append(args, f.sockets...)
	args = append(args, f.fifo)
	cmd := exec.Command("bwrap", args...)
	cmd.Env = append(os.Environ(),
		"DBUS_SESSION_BUS_ADDRESS=unix:path="+f.sockets[0],
		"SSH_AUTH_SOCK="+f.sockets[3])
	out, err := cmd.CombinedOutput()
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

// The fix: every endpoint is gone from inside the wall, and neither address
// reached the seat.
func TestLiveBwrapHidesTheRuntimeDirsControlEndpoints(t *testing.T) {
	t.Parallel()
	f := bhLiveFixture(t)
	hide, err := bwrapRuntimeDirs(f.dir, 2147483000, "")
	if err != nil {
		t.Fatal(err)
	}
	argv, _, err := BwrapArgs("", nil, SeatbeltCarveOut{Hide: hide})
	if err != nil {
		t.Fatal(err)
	}
	got := f.probe(t, argv)
	for _, p := range append(append([]string{}, f.sockets...), f.fifo) {
		if got[p] != "absent" {
			t.Errorf("%s inside the wall: %q, want absent", p, got[p])
		}
	}
	for _, k := range []string{"DBUS", "SSH"} {
		if got[k] != "unset" {
			t.Errorf("%s inside the wall = %q, want unset", k, got[k])
		}
	}
}

// The control: the same fixture, the same probe, no Hide. Without this arm
// the one above would pass on a probe that could reach nothing — and it is
// the reading ranger-base-x9f3s made on the live seat, reproduced here.
func TestLiveBwrapWithoutHideReachesTheRuntimeDir(t *testing.T) {
	t.Parallel()
	f := bhLiveFixture(t)
	argv, _, err := BwrapArgs("", nil, SeatbeltCarveOut{})
	if err != nil {
		t.Fatal(err)
	}
	got := f.probe(t, argv)
	for _, p := range f.sockets {
		if !strings.HasPrefix(got[p], "present") || strings.Contains(got[p], "refused") {
			t.Errorf("control: %s inside an unhidden wall = %q, want present (and connected where python3 can say)", p, got[p])
		}
	}
	if got[f.fifo] != "present" {
		t.Errorf("control: the FIFO inside an unhidden wall = %q, want present", got[f.fifo])
	}
}
