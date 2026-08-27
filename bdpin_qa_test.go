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
// Hermetic: bd, brew and ps are stubbed and HOME is a temp dir, so `make test`
// needs neither the operator's box nor a live daemon. The stubbed ps speaks
// the three forms the script uses — `-A ... pid=,args=`, `-p <pid> comm=`,
// `-p <pid> lstart=` — from a tab-separated fixture.

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
	var b strings.Builder
	for _, p := range procs {
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\n", p.pid, p.comm, p.started.Format(bpLstart), p.args)
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
