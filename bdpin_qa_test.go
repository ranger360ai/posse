package posse

// QA pins for ranger-base-tdwy (the detector the 08-16/08-26 bd incidents
// asked for, ranger-base-31md).
//
// Claim: the fleet's bd pin — 0.49.1 from ~/.local/bin/bd, homebrew's beads
// keg unlinked — is declared in etc/bd/version-pin.toml and asserted by
// scripts/verify-bd-pin.sh at BOTH layers. The command layer is version +
// `command -v bd` + the keg; the PROCESS layer is every live `bd daemon`
// running the pinned binary and younger than it. The 08-16 rollback passed
// every command-layer check and left the orphan running, so the process-layer
// cases below are the ones that carry this bead.
//
// Hermetic: bd, brew, ps and lsof are stubbed and HOME is a temp dir, so
// `make test` needs neither the operator's box nor a live daemon. The stubbed
// ps speaks the three forms the script uses — `-A ... pid=,args=`, `-p <pid>
// comm=`, `-p <pid> lstart=` — from a tab-separated fixture, and the stubbed
// lsof answers `-p <pid> -a -d cwd -Fn` from the same one. lsof is stubbed
// rather than left to the box precisely because it is not: a fixture pid that
// happened to collide with a live process would have the check reading a
// stranger's working directory.
//
// The CWD layer (ranger-base-42mv) is the second process-layer claim: a
// daemon whose working directory is gone, or is a throwaway one, is holding a
// database nothing can reach again. bd auto-starts a daemon on any call and
// stops none, so every bd call in a temp dir leaves one behind — ten in
// twelve days, two of them filed in one evening by this repo's own live test.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const bpLstart = "Mon Jan _2 15:04:05 2006" // what `ps -o lstart=` prints

// bpProc is one row of the stubbed process table.
type bpProc struct {
	pid     string
	comm    string // what `ps -o comm=` reports; "" is the unlinked-binary case
	started time.Time
	args    string
	cwd     string // what `lsof -d cwd` reports; "" means bpStubPS fills in a real, non-temp dir
}

func bpRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"scripts", filepath.Join("etc", "bd")} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range [][2]string{
		{"scripts/verify-bd-pin.sh", "scripts/verify-bd-pin.sh"},
		{"etc/bd/version-pin.toml", "etc/bd/version-pin.toml"},
	} {
		body, err := os.ReadFile(f[0])
		if err != nil {
			t.Fatal(err)
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(f[1], ".sh") {
			mode = 0o755
		}
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(f[1])), body, mode); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// bpHome lays down $HOME/.local/bin/bd — the pinned path the declaration
// names — reporting `version` and stamped with the given mtime, so the
// "daemon older than its own binary" case is expressible without touching a
// real binary.
func bpHome(t *testing.T, version string, mtime time.Time) string {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	bpStubBD(t, filepath.Join(dir, "bd"), version)
	if err := os.Chtimes(filepath.Join(dir, "bd"), mtime, mtime); err != nil {
		t.Fatal(err)
	}
	return home
}

func bpStubBD(t *testing.T, path, version string) {
	t.Helper()
	body := "#!/bin/bash\n" +
		"[ \"$1\" = \"--no-daemon\" ] && shift\n" +
		"if [ \"$1\" = \"version\" ]; then echo \"bd version " + version + " (deadbeef: HEAD@deadbeef)\"; exit 0; fi\n" +
		"echo \"stub bd: $*\" >&2; exit 99\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// bpStubBrew answers `brew info --json=v2 <formula>` with one canned formula
// record. state is "unlinked", "pinned", "LINKED" or "absent".
func bpStubBrew(t *testing.T, dir, state string) {
	t.Helper()
	var json string
	switch state {
	case "absent":
		json = `{"formulae":[{"name":"beads","installed":[],"pinned":false,"linked_keg":null}]}`
	case "pinned":
		json = `{"formulae":[{"name":"beads","installed":[{"version":"1.2.2"}],"pinned":true,"linked_keg":null}]}`
	case "LINKED":
		json = `{"formulae":[{"name":"beads","installed":[{"version":"1.2.2"}],"pinned":false,"linked_keg":"1.2.2"}]}`
	default:
		json = `{"formulae":[{"name":"beads","installed":[{"version":"1.2.2"}],"pinned":false,"linked_keg":null}]}`
	}
	body := "#!/bin/bash\ncat <<'JSON'\n" + json + "\nJSON\n"
	if err := os.WriteFile(filepath.Join(dir, "brew"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// bpStubPS writes the fixture plus a `ps` that serves the three forms the
// script asks for. Real ps is never consulted, so the suite cannot be
// perturbed by whatever daemons the box happens to be running.
func bpStubPS(t *testing.T, dir string, procs []bpProc) {
	t.Helper()
	// An unset cwd stands for "an ordinary daemon in a real repo", and it has
	// to be a directory that EXISTS and is not under a temp root, or the new
	// cwd layer would flag every fixture in the file. The test binary's own
	// working directory is the repo checkout, which is exactly that.
	real, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, p := range procs {
		cwd := p.cwd
		if cwd == "" {
			cwd = real
		}
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%s\n", p.pid, p.comm, p.started.Format(bpLstart), p.args, cwd)
	}
	fixture := filepath.Join(dir, "ps.fixture")
	if err := os.WriteFile(fixture, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	body := `#!/bin/bash
F="$PS_FIXTURE"
case "$*" in
*-A*) awk -F'\t' '{ printf "  %s %s\n", $1, $4 }' "$F"; exit 0;;
esac
pid=${@: -1}
case "$*" in
*comm=*)   awk -F'\t' -v p="$pid" '$1==p{print $2}' "$F";;
*lstart=*) awk -F'\t' -v p="$pid" '$1==p{print $3}' "$F";;
esac
exit 0
`
	if err := os.WriteFile(filepath.Join(dir, "ps"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	// `lsof -p <pid> -a -d cwd -Fn`: one `n<path>` line, and nothing at all
	// for a pid the fixture does not know — which is the honest "could not
	// read it" case rather than a wrong answer.
	lsof := `#!/bin/bash
F="$PS_FIXTURE"
pid=""
while [ $# -gt 0 ]; do
	case "$1" in -p) pid=$2; shift;; esac
	shift
done
awk -F'\t' -v p="$pid" '$1==p && $5!=""{ print "p" $1; print "n" $5 }' "$F"
exit 0
`
	if err := os.WriteFile(filepath.Join(dir, "lsof"), []byte(lsof), 0o755); err != nil {
		t.Fatal(err)
	}
}

// bpRun runs the copied script with a PATH that puts the pinned binary first,
// exactly as the fleet PATH is supposed to resolve.
func bpRun(t *testing.T, root, home, stubs string, shadow string) (string, int) {
	t.Helper()
	cmd := exec.Command(filepath.Join(root, "scripts", "verify-bd-pin.sh"))
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "PATH=") || strings.HasPrefix(kv, "HOME=") || strings.HasPrefix(kv, "PS_FIXTURE=") {
			continue
		}
		env = append(env, kv)
	}
	parts := []string{}
	if shadow != "" { // something linked in FRONT of the pin
		parts = append(parts, shadow)
	}
	if home != "" {
		parts = append(parts, filepath.Join(home, ".local", "bin"))
	}
	if stubs != "" {
		parts = append(parts, stubs)
	}
	parts = append(parts, "/usr/bin", "/bin")
	cmd.Env = append(env,
		"PATH="+strings.Join(parts, string(os.PathListSeparator)),
		"HOME="+home,
		"PS_FIXTURE="+filepath.Join(stubs, "ps.fixture"),
	)
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("verify-bd-pin.sh: %v\n%s", err, out)
		}
	}
	return string(out), code
}

// bpFixture is the ordinary case: pinned binary written a week ago, one
// daemon started yesterday from it.
func bpFixture(t *testing.T, brew string, procs []bpProc) (root, home, stubs string, binMtime time.Time) {
	t.Helper()
	root = bpRoot(t)
	binMtime = time.Now().Add(-7 * 24 * time.Hour)
	home = bpHome(t, "0.49.1", binMtime)
	stubs = t.TempDir()
	bpStubBrew(t, stubs, brew)
	bpStubPS(t, stubs, procs)
	return
}

// bpStubStat puts a `stat` on the stub PATH that behaves like the named
// platform's, so BOTH platforms' stat is exercised on EITHER host — the bug
// this pins was reachable only from linux and so only `make test-linux` ever
// saw it (ranger-base-tssy).
//
//	gnu     `-c FMT` is the format flag. `-f` is DISPLAY FILESYSTEM STATUS and
//	        takes NO format, so `stat -f %m FILE` reads two FILE operands:
//	        it prints FILE's filesystem block on STDOUT and only then exits
//	        non-zero on the missing `%m` — which is why a `-f`-first `||`
//	        chain got the blob AND the fallback's epoch appended to it.
//	bsd     `-f FMT` is the format flag; `-c` is not an option at all and is
//	        rejected with no stdout, which is what makes GNU-first safe.
//	broken  answers both forms with non-numeric junk and exit 0 — the third
//	        stat nobody has met. The verdict must degrade to "age unverified",
//	        never to `ok`.
//
// Only the pinned binary is a known operand; anything else fails as it would
// on the real thing.
func bpStubStat(t *testing.T, dir, flavor, target string, mtime time.Time) {
	t.Helper()
	epoch := fmt.Sprintf("%d", mtime.Unix())
	var body string
	switch flavor {
	case "gnu":
		body = `#!/bin/bash
if [ "$1" = -c ]; then
  [ "$2" = %Y ] || { echo "stat: invalid directive" >&2; exit 1; }
  shift 2
  [ "$1" = "$TARGET" ] || { echo "stat: cannot statx '$1'" >&2; exit 1; }
  echo "$EPOCH"; exit 0
fi
if [ "$1" = -f ]; then
  shift; rc=0                       # -f takes no format: every word is a FILE
  for f in "$@"; do
    if [ "$f" = "$TARGET" ]; then
      printf '  File: "%s"\n    ID: 5225eaf229dfbec4 Namelen: 255     Type: overlayfs\nBlock size: 4096       Fundamental block size: 4096\n' "$f"
    else
      echo "stat: cannot read file system information for '$f'" >&2; rc=1
    fi
  done
  exit $rc
fi
echo "stat: unsupported: $*" >&2; exit 1
`
	case "bsd":
		body = `#!/bin/bash
if [ "$1" = -f ]; then
  [ "$2" = %m ] || { echo "stat: bad format" >&2; exit 1; }
  shift 2
  [ "$1" = "$TARGET" ] || { echo "stat: $1: No such file or directory" >&2; exit 1; }
  echo "$EPOCH"; exit 0
fi
echo "stat: illegal option -- ${1#-}" >&2; exit 1
`
	case "broken":
		body = `#!/bin/bash
echo "mtime: whenever"; exit 0
`
	default:
		t.Fatalf("bpStubStat: unknown flavor %q", flavor)
	}
	body = "#!/bin/bash\nEPOCH=" + epoch + "\nTARGET=" + target + "\n" + strings.TrimPrefix(body, "#!/bin/bash\n")
	if err := os.WriteFile(filepath.Join(dir, "stat"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// argv0 is what `ps -A ... args=` reports and comm is what `ps -o comm=`
// reports. They are the same path for a healthy daemon and they diverge
// exactly in the orphan case: argv[0] is fixed at exec and survives, while
// macOS empties comm once the executable is unlinked (MEASURED 2026-08-27).
func bpDaemon(pid, argv0, comm string, started time.Time) bpProc {
	return bpProc{pid: pid, comm: comm, started: started, args: argv0 + " daemon start"}
}

func TestQABdPinDeclarationAndWiring(t *testing.T) {
	pin, err := os.ReadFile("etc/bd/version-pin.toml")
	if err != nil {
		t.Fatal(err)
	}
	body := string(pin)
	for _, want := range []string{
		`posse_pinned_version = "0.49.1"`,
		`pinned_binary = "~/.local/bin/bd"`,
		`formula = "beads"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("etc/bd/version-pin.toml missing %q", want)
		}
	}

	mk, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mk), "verify-bd-pin:\n\tscripts/verify-bd-pin.sh\n") {
		t.Error("Makefile lost the verify-bd-pin target")
	}
	if !strings.Contains(string(mk), ".PHONY:") || !strings.Contains(string(mk), "verify-bd-pin ") {
		t.Error("verify-bd-pin is not in .PHONY")
	}

	info, err := os.Stat("scripts/verify-bd-pin.sh")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0111 == 0 {
		t.Fatal("scripts/verify-bd-pin.sh is not executable")
	}

	// The check is wired where the pin can silently move: `git worktree add`
	// in clean-build.sh fires the bd post-checkout hook, which is how the
	// storm broke `make install`.
	cb, err := os.ReadFile("scripts/clean-build.sh")
	if err != nil {
		t.Fatal(err)
	}
	cbs := string(cb)
	if !strings.Contains(cbs, "verify-bd-pin.sh") {
		t.Error("clean-build.sh does not pre-flight the bd pin before `git worktree add`")
	}
	// Regression pin, MEASURED while building this: clean-build.sh's own
	// output PATH is `out`. Capturing the pin report into `out` silently
	// replaced the built binary's path with the report text — `make release`
	// still exited 0 and wrote its binary somewhere nobody asked for.
	if strings.Contains(cbs, `out=$("$pincheck"`) {
		t.Error("clean-build.sh captures the pin report into `out`, which is its output path")
	}
	if i, j := strings.Index(cbs, "verify-bd-pin.sh"), strings.Index(cbs, `worktree add --detach`); i < 0 || j < 0 || i > j {
		t.Error("the pin pre-flight must run BEFORE `git worktree add` fires the bd hook")
	}
}

func TestQABdPinHappyPathNoDaemons(t *testing.T) {
	root, home, stubs, _ := bpFixture(t, "unlinked", nil)
	out, code := bpRun(t, root, home, stubs, "")
	if code != 0 {
		t.Fatalf("exit %d, want 0\n%s", code, out)
	}
	for _, want := range []string{"pin intact at 0.49.1", "none running", "unlinked 1.2.2"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

func TestQABdPinHappyPathDaemonOnPinnedBinary(t *testing.T) {
	root, home, stubs, mtime := bpFixture(t, "unlinked", nil)
	pinned := filepath.Join(home, ".local", "bin", "bd")
	bpStubPS(t, stubs, []bpProc{bpDaemon("4548", pinned, pinned, mtime.Add(time.Hour))})
	out, code := bpRun(t, root, home, stubs, "")
	if code != 0 {
		t.Fatalf("exit %d, want 0\n%s", code, out)
	}
	if !strings.Contains(out, "pid 4548") || !strings.Contains(out, "pin intact") {
		t.Errorf("a daemon on the pinned binary, younger than it, must pass:\n%s", out)
	}
}

// The bead. The 08-16 rollback checked `bd version`, `bd ready` and
// `dispatch --dry-run` — every command-layer row below is green — and left a
// daemon from the reverted artifact running for 12d21h.
func TestQABdPinCatchesTheOrphanTheCommandLayerMissed(t *testing.T) {
	root, home, stubs, mtime := bpFixture(t, "unlinked", nil)
	// comm empty: macOS reports no path once the executable is unlinked
	// (MEASURED 2026-08-27). A stale path that no longer exists is the same
	// verdict, covered below.
	bpStubPS(t, stubs, []bpProc{bpDaemon("31368", "/opt/homebrew/bin/bd", "", mtime.Add(time.Hour))})
	out, code := bpRun(t, root, home, stubs, "")
	if code != 1 {
		t.Fatalf("exit %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "ORPHAN") {
		t.Errorf("an unlinked-binary daemon must be named ORPHAN:\n%s", out)
	}
	if !strings.Contains(out, "bd version") || strings.Contains(out, "bd version               ?") {
		t.Errorf("the command layer must still read green — that is the point:\n%s", out)
	}
}

func TestQABdPinCatchesDeletedBinaryPath(t *testing.T) {
	root, home, stubs, mtime := bpFixture(t, "unlinked", nil)
	bpStubPS(t, stubs, []bpProc{bpDaemon("4548", "/opt/homebrew/bin/bd", "/opt/homebrew/bin/bd", mtime.Add(time.Hour))})
	out, code := bpRun(t, root, home, stubs, "")
	if code != 1 {
		t.Fatalf("exit %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "ORPHAN") {
		t.Errorf("a daemon whose binary path is gone must be ORPHAN:\n%s", out)
	}
}

func TestQABdPinCatchesForeignBinary(t *testing.T) {
	root, home, stubs, mtime := bpFixture(t, "unlinked", nil)
	foreign := filepath.Join(stubs, "bd")
	bpStubBD(t, foreign, "1.2.2")
	bpStubPS(t, stubs, []bpProc{bpDaemon("4548", foreign, foreign, mtime.Add(time.Hour))})
	out, code := bpRun(t, root, home, stubs, "")
	if code != 1 {
		t.Fatalf("exit %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "FOREIGN") {
		t.Errorf("a daemon on a different, existing binary must be FOREIGN:\n%s", out)
	}
}

// ranger-base-zk8v. `ps -o comm=` reports the path AS INVOKED, so a daemon
// started with a relative argv0 — `cd <dir> && ./bd daemon start`, which is
// how a hand-built or vendored bd gets run — reports a RELATIVE path. The
// script has already `cd`'d to the repo root by then, so testing it with `-e`
// asked whether the REPO contains a file called `./bd`. It does not, and a
// binary that is sitting on disk was called ORPHAN — the wrong verdict and
// therefore the wrong runbook (ORPHAN says the artifact is gone, so the
// operator is sent to reap and restart; what is actually running is a foreign
// build that is still there to be identified).
//
// Fail-safe both ways — the alarm fired and the exit code was 1 either way,
// because a relative path can never equal the absolute $want_bin — so only
// the verdict text was ever wrong. It survived a green suite because every
// other fixture in this file passes an ABSOLUTE comm.
//
// The cwd line reads EPHEMERAL here and that is not what this arm is about:
// the daemon's directory has to be one the test can write a binary into, and
// every directory a test owns is under a temp root. The claim is the BINARY
// line, which is exactly why the two layers are reported separately.
func TestQABdPinResolvesARelativeCommAgainstTheProcessCwdNotTheRepo(t *testing.T) {
	root, home, stubs, mtime := bpFixture(t, "unlinked", nil)
	dir := filepath.Join(t.TempDir(), "vendor")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	bpStubBD(t, filepath.Join(dir, "bd"), "1.2.2") // present, and NOT the pin
	bpStubPS(t, stubs, []bpProc{bpDaemonAt("4548", "./bd", "./bd", mtime.Add(time.Hour), dir)})
	out, code := bpRun(t, root, home, stubs, "")
	if code != 1 {
		t.Fatalf("exit %d, want 1 — a daemon on an unpinned binary is a failure\n%s", code, out)
	}
	if strings.Contains(out, "ORPHAN") {
		t.Errorf("the binary is on disk in the daemon's own directory — ORPHAN is the repo root answering a question about another directory:\n%s", out)
	}
	if !strings.Contains(out, "FOREIGN") {
		t.Errorf("a relative comm pointing at a present, unpinned binary must be FOREIGN:\n%s", out)
	}
}

// The other side of the same fix: resolving against the cwd must not lose the
// ORPHAN arm for a daemon whose binary really has been deleted out from under
// it — the ranger-base-9x1 shape, only invoked relatively. `vendor/` exists,
// `vendor/bd` does not.
func TestQABdPinStillCallsARelativeCommOrphanWhenItsBinaryIsGone(t *testing.T) {
	root, home, stubs, mtime := bpFixture(t, "unlinked", nil)
	dir := filepath.Join(t.TempDir(), "vendor")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	bpStubPS(t, stubs, []bpProc{bpDaemonAt("4549", "./bd", "./bd", mtime.Add(time.Hour), dir)})
	out, code := bpRun(t, root, home, stubs, "")
	if code != 1 {
		t.Fatalf("exit %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "ORPHAN") {
		t.Errorf("a relative comm with nothing behind it in its own directory is still an orphan:\n%s", out)
	}
}

// And the arm a blanket "every relative comm is FOREIGN" fix would get wrong:
// a daemon started from inside ~/.local/bin as `./bd` IS the pinned binary.
// The binary layer must read ok — the cwd line is EPHEMERAL only because the
// pinned binary's directory is under $HOME and $HOME is a t.TempDir here.
func TestQABdPinAcceptsThePinnedBinaryInvokedByARelativePath(t *testing.T) {
	root, home, stubs, mtime := bpFixture(t, "unlinked", nil)
	bpStubPS(t, stubs, []bpProc{bpDaemonAt("4550", "./bd", "./bd", mtime.Add(time.Hour), filepath.Join(home, ".local", "bin"))})
	out, code := bpRun(t, root, home, stubs, "")
	if !strings.Contains(out, "binary ok") {
		t.Errorf("exit %d: the pinned binary invoked from its own directory is still the pinned binary:\n%s", code, out)
	}
}

// Relative comm and no cwd to resolve it against: there is nothing to test
// for existence, so the honest answer is the one claim that still holds — a
// relative path is not the absolute pinned binary. Never ok, and never a
// confident ORPHAN.
func TestQABdPinRelativeCommWithNoCwdIsNotOkAndNotOrphan(t *testing.T) {
	root, home, stubs, mtime := bpFixture(t, "unlinked", nil)
	bpStubPS(t, stubs, []bpProc{bpDaemonAt("4551", "./bd", "./bd", mtime.Add(time.Hour), "")})
	fixture := filepath.Join(stubs, "ps.fixture")
	b, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	cols := strings.SplitN(strings.TrimRight(string(b), "\n"), "\t", 5)
	if err := os.WriteFile(fixture, []byte(strings.Join(cols[:4], "\t")+"\t\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := bpRun(t, root, home, stubs, "")
	if code != 1 {
		t.Fatalf("exit %d, want 1 — a daemon that is provably not on the pin is a failure\n%s", code, out)
	}
	if strings.Contains(out, "binary ok") || strings.Contains(out, "ORPHAN") {
		t.Errorf("with no cwd the binary's existence is unknowable — neither ok nor ORPHAN:\n%s", out)
	}
}

// The 08-16 shape exactly: the orphan (started 08-13) predates the binary the
// rollback wrote (08-16 23:54). Path checks alone say ok; the age check does
// not.
func TestQABdPinCatchesDaemonOlderThanItsOwnBinary(t *testing.T) {
	root, home, stubs, mtime := bpFixture(t, "unlinked", nil)
	pinned := filepath.Join(home, ".local", "bin", "bd")
	bpStubPS(t, stubs, []bpProc{bpDaemon("31368", pinned, pinned, mtime.Add(-3*24*time.Hour))})
	out, code := bpRun(t, root, home, stubs, "")
	if code != 1 {
		t.Fatalf("exit %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "STALE") {
		t.Errorf("a daemon predating the pinned binary must be STALE:\n%s", out)
	}
}

// ranger-base-tssy. The mtime probe read BSD `stat -f %m` first and fell back
// on exit status, but `-f` does not FAIL on GNU — it prints a filesystem blob
// and then errors, so the fallback ran too and bin_mtime came out as blob +
// epoch. `-lt` errored, the STALE arm went false, and the "age unverified"
// arm went false as well because bin_mtime was non-empty: linux printed `ok`
// for a daemon it had never checked. That is the 08-16 command-layer-only
// verdict (TestQABdPinCatchesTheOrphanTheCommandLayerMissed) reintroduced on
// the other platform and reported as green.
//
// Stubbing stat is what makes this a pin on BOTH hosts. The live bug was
// visible only under `make test-linux`; here darwin runs the GNU case too.
func TestQABdPinReadsBinaryMtimeWhicheverStatIsInstalled(t *testing.T) {
	for _, flavor := range []string{"gnu", "bsd"} {
		t.Run(flavor+"/stale", func(t *testing.T) {
			root, home, stubs, mtime := bpFixture(t, "unlinked", nil)
			pinned := filepath.Join(home, ".local", "bin", "bd")
			bpStubStat(t, stubs, flavor, pinned, mtime)
			bpStubPS(t, stubs, []bpProc{bpDaemon("31368", pinned, pinned, mtime.Add(-3*24*time.Hour))})
			out, code := bpRun(t, root, home, stubs, "")
			if code != 1 {
				t.Fatalf("exit %d, want 1\n%s", code, out)
			}
			if !strings.Contains(out, "STALE") {
				t.Errorf("a daemon predating the pinned binary must be STALE under %s stat:\n%s", flavor, out)
			}
			// The failure mode was silent: a shell error on stderr and a
			// filesystem block pasted into the run, with the verdict still ok.
			for _, leak := range []string{"Namelen", "integer expression expected"} {
				if strings.Contains(out, leak) {
					t.Errorf("%s stat output leaked into the run (%q):\n%s", flavor, leak, out)
				}
			}
		})

		t.Run(flavor+"/young", func(t *testing.T) {
			root, home, stubs, mtime := bpFixture(t, "unlinked", nil)
			pinned := filepath.Join(home, ".local", "bin", "bd")
			bpStubStat(t, stubs, flavor, pinned, mtime)
			bpStubPS(t, stubs, []bpProc{bpDaemon("4548", pinned, pinned, mtime.Add(time.Hour))})
			out, code := bpRun(t, root, home, stubs, "")
			if code != 0 {
				t.Fatalf("exit %d, want 0\n%s", code, out)
			}
			// `ok` here and not "age unverified": the mtime was actually read
			// under this stat, not quietly discarded. A fix that always
			// returned empty would pass the stale case above and fail here.
			if strings.Contains(out, "age unverified") {
				t.Errorf("%s stat: the binary mtime must be readable, not unverified:\n%s", flavor, out)
			}
		})
	}
}

// The belt. A stat this script has never met — one that answers with
// something that is not an epoch and exits 0 — must route the verdict to the
// honest "age unverified" arm. `ok` for an unchecked daemon is the thing
// ranger-base-tdwy exists to prevent, and it must not be reachable by a probe
// coming back wrong.
func TestQABdPinCallsAnUnreadableMtimeUnverifiedNotOk(t *testing.T) {
	root, home, stubs, mtime := bpFixture(t, "unlinked", nil)
	pinned := filepath.Join(home, ".local", "bin", "bd")
	bpStubStat(t, stubs, "broken", pinned, mtime)
	bpStubPS(t, stubs, []bpProc{bpDaemon("31368", pinned, pinned, mtime.Add(-3*24*time.Hour))})
	out, code := bpRun(t, root, home, stubs, "")
	if code != 0 {
		t.Fatalf("exit %d, want 0 — unreadable is not a failure, it is unverified\n%s", code, out)
	}
	if !strings.Contains(out, "age unverified") {
		t.Errorf("an unparseable binary mtime must read as unverified:\n%s", out)
	}
}

// Not a daemon: `bd list` and anything whose argv[1] is not `daemon` must not
// be flagged, or the check cries wolf on every bd call the fleet makes.
func TestQABdPinIgnoresNonDaemonBdProcesses(t *testing.T) {
	root, home, stubs, mtime := bpFixture(t, "unlinked", nil)
	pinned := filepath.Join(home, ".local", "bin", "bd")
	bpStubPS(t, stubs, []bpProc{
		{pid: "42894", comm: pinned, started: mtime.Add(-3 * 24 * time.Hour), args: pinned + " list --all --json"},
		{pid: "42917", comm: "/bin/zsh", started: mtime, args: "/bin/zsh -c grep 'bd daemon' log"},
	})
	out, code := bpRun(t, root, home, stubs, "")
	if code != 0 {
		t.Fatalf("exit %d, want 0 — neither row is a daemon\n%s", code, out)
	}
	if !strings.Contains(out, "none running") {
		t.Errorf("`bd list` and a grep for the phrase must not count as daemons:\n%s", out)
	}
}

// ─── the cwd layer (ranger-base-42mv) ───────────────────────────────────────
//
// bdDaemonAt is bpDaemon with a working directory. A daemon's cwd is the
// `.beads` directory beside the database it is serving, which is what makes
// the classification safe: the canonical queue's is inside a real repo and is
// never named, whatever else the box is running.
func bpDaemonAt(pid, argv0, comm string, started time.Time, cwd string) bpProc {
	p := bpDaemon(pid, argv0, comm, started)
	p.cwd = cwd
	return p
}

// The nine monica reaped by hand on 2026-08-26: session scratchpads and test
// fixtures that were deleted with the daemon still holding them. The binary
// is fine, the version is fine, and every command-layer row is green — the
// directory is what is gone.
func TestQABdPinFlagsDaemonWhoseWorkingDirectoryIsGone(t *testing.T) {
	root, home, stubs, mtime := bpFixture(t, "unlinked", nil)
	pinned := filepath.Join(home, ".local", "bin", "bd")
	gone := filepath.Join(t.TempDir(), "deleted-session", ".beads") // never created
	bpStubPS(t, stubs, []bpProc{bpDaemonAt("21232", pinned, pinned, mtime.Add(time.Hour), gone)})
	out, code := bpRun(t, root, home, stubs, "")
	if code != 1 {
		t.Fatalf("exit %d, want 1 — a daemon holding a deleted directory is a failure\n%s", code, out)
	}
	if !strings.Contains(out, "LEAKED") {
		t.Errorf("a daemon whose working directory is gone must be named LEAKED:\n%s", out)
	}
	if !strings.Contains(out, "binary ok") {
		t.Errorf("the binary layer must still read ok — folding the two verdicts together is how this process reads `ok` today:\n%s", out)
	}
	if !strings.Contains(out, "kill -TERM 21232") {
		t.Errorf("the report must hand the operator the exact reap for this pid:\n%s", out)
	}
}

// The two this repo filed on 2026-08-25, and the six claude scratchpads
// before them: the directory is still there for now, but it is a throwaway,
// so the database behind it is already unreachable in every sense that
// matters. Flag it while the reap is still cheap.
func TestQABdPinFlagsDaemonInAThrowawayDirectory(t *testing.T) {
	root, home, stubs, mtime := bpFixture(t, "unlinked", nil)
	pinned := filepath.Join(home, ".local", "bin", "bd")
	tmp := filepath.Join(t.TempDir(), ".beads") // t.TempDir is /tmp or /var/folders
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatal(err)
	}
	bpStubPS(t, stubs, []bpProc{bpDaemonAt("85079", pinned, pinned, mtime.Add(time.Hour), tmp)})
	out, code := bpRun(t, root, home, stubs, "")
	if code != 1 {
		t.Fatalf("exit %d, want 1 — a daemon in a temp dir is a failure\n%s", code, out)
	}
	if !strings.Contains(out, "EPHEMERAL") {
		t.Errorf("a daemon whose cwd exists but is under a temp root must be named EPHEMERAL:\n%s", out)
	}
}

// The blast-radius rule from ranger-base-nsm, and the reason the whole layer
// classifies by cwd: the canonical queue's daemon must come through a run
// that flags two others, and the reap the report prints must name those two
// and only those two. `bd daemon stop-all` would take all three.
func TestQABdPinSparesTheCanonicalDaemonAndNamesOnlyTheLeaked(t *testing.T) {
	root, home, stubs, mtime := bpFixture(t, "unlinked", nil)
	pinned := filepath.Join(home, ".local", "bin", "bd")
	young := mtime.Add(time.Hour)
	tmp := filepath.Join(t.TempDir(), ".beads")
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatal(err)
	}
	bpStubPS(t, stubs, []bpProc{
		bpDaemon("92822", pinned, pinned, young), // the live canonical queue
		bpDaemonAt("85079", pinned, pinned, young, tmp),
		bpDaemonAt("21232", pinned, pinned, young, filepath.Join(t.TempDir(), "gone", ".beads")),
	})
	out, code := bpRun(t, root, home, stubs, "")
	if code != 1 {
		t.Fatalf("exit %d, want 1\n%s", code, out)
	}
	var reap string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "kill -TERM") && strings.Contains(line, "85079") {
			reap = line
		}
	}
	if reap == "" {
		t.Fatalf("no reap line naming the leaked daemons:\n%s", out)
	}
	if !strings.Contains(reap, "21232") {
		t.Errorf("the reap must name BOTH leaked daemons, got %q", reap)
	}
	if strings.Contains(reap, "92822") {
		t.Errorf("the reap named the canonical queue's daemon, which is the one thing it must never do: %q", reap)
	}
	if !strings.Contains(out, "NEVER `bd daemon stop-all`") {
		t.Errorf("the report must say why a global stop is not the remedy:\n%s", out)
	}
}

// The honest arm. A cwd the probe cannot read is UNVERIFIED, never ok and
// never LEAKED: `ok` for a process nobody looked at is the 08-16
// command-layer verdict all over again, and a reap prescribed off a failed
// read is worse.
func TestQABdPinCwdUnreadableIsUnverifiedNotOk(t *testing.T) {
	root, home, stubs, mtime := bpFixture(t, "unlinked", nil)
	pinned := filepath.Join(home, ".local", "bin", "bd")
	bpStubPS(t, stubs, []bpProc{bpDaemonAt("41000", pinned, pinned, mtime.Add(time.Hour), "")})
	// bpStubPS fills an empty cwd with a real directory, so blank the column
	// the way an lsof that answered nothing would.
	fixture := filepath.Join(stubs, "ps.fixture")
	b, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	cols := strings.SplitN(strings.TrimRight(string(b), "\n"), "\t", 5)
	if err := os.WriteFile(fixture, []byte(strings.Join(cols[:4], "\t")+"\t\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := bpRun(t, root, home, stubs, "")
	if code != 0 {
		t.Fatalf("exit %d, want 0 — unreadable is not a failure, it is unverified\n%s", code, out)
	}
	if !strings.Contains(out, "working directory unverified") {
		t.Errorf("an unreadable cwd must read as unverified:\n%s", out)
	}
	for _, bad := range []string{"LEAKED", "EPHEMERAL", "kill -TERM 41000"} {
		if strings.Contains(out, bad) {
			t.Errorf("an unread cwd must not be prescribed a reap (%q):\n%s", bad, out)
		}
	}
}

func TestQABdPinFailsOnVersionDrift(t *testing.T) {
	root := bpRoot(t)
	mtime := time.Now().Add(-7 * 24 * time.Hour)
	home := bpHome(t, "1.2.2", mtime)
	stubs := t.TempDir()
	bpStubBrew(t, stubs, "unlinked")
	bpStubPS(t, stubs, nil)
	out, code := bpRun(t, root, home, stubs, "")
	if code != 1 {
		t.Fatalf("exit %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "bd version") || !strings.Contains(out, "1.2.2") || !strings.Contains(out, "FAIL") {
		t.Errorf("must FAIL the version row at 1.2.2:\n%s", out)
	}
}

// /opt/homebrew/bin precedes ~/.local/bin on the fleet PATH, so "a bd is on
// PATH and reports 0.49.1" is not the claim — "THE pinned bd is what resolves"
// is.
func TestQABdPinFailsWhenSomethingIsLinkedInFrontOfThePin(t *testing.T) {
	root, home, stubs, _ := bpFixture(t, "unlinked", nil)
	shadow := t.TempDir()
	bpStubBD(t, filepath.Join(shadow, "bd"), "0.49.1")
	out, code := bpRun(t, root, home, stubs, shadow)
	if code != 1 {
		t.Fatalf("exit %d, want 1 — a shadowing bd of the SAME version still breaks the pin\n%s", code, out)
	}
	if !strings.Contains(out, "command -v bd") || !strings.Contains(out, "FAIL") {
		t.Errorf("must FAIL the resolution row:\n%s", out)
	}
}

// bpStubGateShim writes a posse gate shim (internal/rhq/gates.go renderShim)
// that execs target — the shape every persona session actually has on PATH
// ahead of ~/.local/bin, distinct from an arbitrary shadowing binary.
func bpStubGateShim(t *testing.T, dir, target string) {
	t.Helper()
	body := "#!/bin/sh\n" +
		"# posse gate for testpersona — rendered from the PID's deny: at launch; do not edit (rangerhq-9ha)\n" +
		"exec '" + target + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "bd"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// ranger-base-43v1: row 2 compared raw paths, so a posse gate shim — which
// every persona session has on PATH ahead of ~/.local/bin by design — never
// string-equalled the pinned path and FAILed in every session on a box whose
// pin was intact. A shim whose own exec line targets the pinned binary must
// read GATED, not FAIL, and must not fail the run.
func TestQABdPinTreatsAGateShimExecingThePinnedBinaryAsGated(t *testing.T) {
	root, home, stubs, _ := bpFixture(t, "unlinked", nil)
	shadow := t.TempDir()
	bpStubGateShim(t, shadow, filepath.Join(home, ".local", "bin", "bd"))
	out, code := bpRun(t, root, home, stubs, shadow)
	if code != 0 {
		t.Fatalf("exit %d, want 0 — the gate shim execs the pinned binary\n%s", code, out)
	}
	if !strings.Contains(out, "command -v bd") || !strings.Contains(out, "GATED") {
		t.Errorf("must report the resolution row GATED:\n%s", out)
	}
	if strings.Contains(out, "command -v bd") && strings.Contains(out[strings.Index(out, "command -v bd"):], "FAIL") {
		t.Errorf("gated row must not read FAIL:\n%s", out)
	}
}

// The shim's OWN exec target is what is asserted, not merely that a gate
// shim is present: a shim left stale by a render that predates a pin bump
// still points at the wrong binary, and that must still FAIL.
func TestQABdPinFailsWhenAGateShimExecsSomethingElse(t *testing.T) {
	root, home, stubs, _ := bpFixture(t, "unlinked", nil)
	shadow := t.TempDir()
	wrong := filepath.Join(t.TempDir(), "bd")
	bpStubGateShim(t, shadow, wrong)
	out, code := bpRun(t, root, home, stubs, shadow)
	if code != 1 {
		t.Fatalf("exit %d, want 1 — the gate shim execs the wrong binary\n%s", code, out)
	}
	if !strings.Contains(out, "command -v bd") || !strings.Contains(out, "FAIL") || !strings.Contains(out, wrong) {
		t.Errorf("must FAIL the resolution row and name the shim's real target:\n%s", out)
	}
}

func TestQABdPinFailsWhenTheKegIsLinked(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3: the keg row degrades to a non-failing note by design")
	}
	root, home, stubs, _ := bpFixture(t, "LINKED", nil)
	out, code := bpRun(t, root, home, stubs, "")
	if code != 1 {
		t.Fatalf("exit %d, want 1\n%s", code, out)
	}
	if !strings.Contains(out, "LINKED") || !strings.Contains(out, "FAIL") {
		t.Errorf("a linked keg is the 08-16 outage re-armed and must FAIL:\n%s", out)
	}
}

// brew pin is the belt and is the operator's hand, not this lane's: pinned
// passes, and unlinked-but-unpinned passes while saying so.
func TestQABdPinAcceptsPinnedAndAdvisesWhenOnlyUnlinked(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("no python3")
	}
	root, home, stubs, _ := bpFixture(t, "pinned", nil)
	out, code := bpRun(t, root, home, stubs, "")
	if code != 0 {
		t.Fatalf("brew-pinned: exit %d, want 0\n%s", code, out)
	}
	if strings.Contains(out, "NOT BELT-AND-BRACES") {
		t.Errorf("a brew-pinned keg needs no belt advisory:\n%s", out)
	}

	root, home, stubs, _ = bpFixture(t, "unlinked", nil)
	out, code = bpRun(t, root, home, stubs, "")
	if code != 0 {
		t.Fatalf("unlinked: exit %d, want 0\n%s", code, out)
	}
	for _, want := range []string{"NOT BELT-AND-BRACES", "brew pin beads"} {
		if !strings.Contains(out, want) {
			t.Errorf("unlinked-but-unpinned must say so and name the operator's command, missing %q:\n%s", want, out)
		}
	}
}

// The check reports; it never acts. `Bash(bd daemon:*)` is denied fleet-wide,
// so a failing run must hand the operator the exact next step instead of
// implying the script took one.
func TestQABdPinNeverActsAndSaysSo(t *testing.T) {
	root, home, stubs, mtime := bpFixture(t, "unlinked", nil)
	bpStubPS(t, stubs, []bpProc{bpDaemon("31368", "/opt/homebrew/bin/bd", "", mtime)})
	out, code := bpRun(t, root, home, stubs, "")
	if code != 1 {
		t.Fatalf("exit %d, want 1\n%s", code, out)
	}
	for _, want := range []string{"REMEDIATION IS THE OPERATOR'S", "kill -TERM"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}

	script, err := os.ReadFile("scripts/verify-bd-pin.sh")
	if err != nil {
		t.Fatal(err)
	}
	// Scan the CODE only: comments and here-doc bodies are the operator-facing
	// report, and naming `kill -TERM` / `brew pin` there is the whole point.
	var codeLines []string
	term := ""
	for _, line := range strings.Split(string(script), "\n") {
		s := strings.TrimSpace(line)
		if term != "" {
			if s == term {
				term = ""
			}
			continue
		}
		if i := strings.Index(line, "<<"); i >= 0 {
			w := strings.TrimLeft(line[i+2:], "-")
			w = strings.Trim(strings.Fields(w + " ")[0], "'\"")
			if w != "" {
				term = w
			}
		}
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		codeLines = append(codeLines, s)
	}
	for _, forbidden := range []string{"kill -", "bd daemon stop", "bd daemon start", "brew pin ", "brew link", "brew upgrade", "rm ", "install "} {
		for _, s := range codeLines {
			if strings.Contains(s, forbidden) {
				t.Errorf("verify-bd-pin.sh must not invoke %q — it is read-only: %s", forbidden, s)
			}
		}
	}
}

func TestQABdPinExits2WhenItCannotCheck(t *testing.T) {
	root := bpRoot(t)
	stubs := t.TempDir()
	bpStubBrew(t, stubs, "unlinked")
	bpStubPS(t, stubs, nil)
	out, code := bpRun(t, root, t.TempDir(), stubs, "") // HOME with no bd anywhere
	if code != 2 {
		t.Fatalf("no bd on PATH: exit %d, want 2\n%s", code, out)
	}
	if !strings.Contains(out, "bd not on PATH") {
		t.Errorf("want the bd-missing diagnostic:\n%s", out)
	}

	// Declaration gone: cannot check, must not claim the pin is intact.
	root2, home, stubs2, _ := bpFixture(t, "unlinked", nil)
	if err := os.Remove(filepath.Join(root2, "etc", "bd", "version-pin.toml")); err != nil {
		t.Fatal(err)
	}
	out, code = bpRun(t, root2, home, stubs2, "")
	if code != 2 {
		t.Fatalf("missing declaration: exit %d, want 2\n%s", code, out)
	}
	if !strings.Contains(out, "missing") {
		t.Errorf("want the missing-declaration diagnostic:\n%s", out)
	}
}
