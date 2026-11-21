//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-rrgnc — G18, the dispatch loop that runs an image
// the install replaced. The acceptance asked for "a pin that starts a watch
// loop, swaps the binary by rename and asserts the condition fires": the loop
// here is a real process exec'd from a real file, holding this home's real
// watch lock and record, and the swap is the rename `posse upgrade` does.

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// rrgncLoopLife is how long the stand-in loop lives, and it is this long for
// a reason that cost a red suite (ranger-base-rph5z's run, 2026-11-21).
//
// It was "60". These tests start a stand-in dispatch loop, record its pid,
// and then read that pid back — and `WatchLoopRunning` plus `ReadWatchPid`
// answer LIVENESS and not identity, which watchpid.go's own `pidAlive`
// comment says in so many words: "a recycled pid is alive and is a
// stranger". internal/posse is one parallel test binary that ran 262.7s on
// the failing pass, and a parallel test can be descheduled for longer than a
// minute on a loaded box. So the loop EXITED mid-test, its pid was reused by
// another test's re-exec of the test binary, and the reading came back
// pointing at `posse.test` in the go-build cache with `Image == OnDisk` —
// rendering as "this loop is not stale" and failing an assertion about a
// rename that had worked perfectly.
//
// 1500 is the per-package `-timeout` the Makefile passes, so the child cannot
// outlive the budget that would kill the suite anyway, and cannot exit inside
// it. The loop blocks in a read that never completes, so it burns no CPU and
// a leaked one is not what the load guard's kill arm is for (AGENTS.md,
// "Something you MEANT to leave running"); each test kills its own child in a
// t.Cleanup regardless, and rrgncStartLoop's pipe bounds an orphan at the
// death of this process rather than at this timer.
const rrgncLoopLife = "1500"

// rrgncDonor is the binary a stand-in loop runs, and it is bash for one
// reason: it is the only candidate whose SHAPE this fixture cannot be wrong
// about.
//
// It was `sleep`, and on 2026-11-21 that reddened every ubuntu arm-1 CI job
// (ranger-base-f177g). ubuntu-latest is Ubuntu 26.04 now, whose `coreutils`
// is a 2.7 kB Architecture:all metapackage over `coreutils-from-uutils`:
// /usr/bin/sleep is a SYMLINK into one multi-call binary, and that binary
// decides which utility to BE from the path the kernel exec'd it through,
// taking that path's file STEM and ignoring argv[0] on purpose (uutils
// src/common/validation.rs: "prevent `env -a` from bypassing AppArmor,
// SELinux policies"). The subject of these tests is an install renaming over
// the file a loop runs, so the donor's bytes live at `<root>/posse` by
// construction — which asked that donor to be a utility called `posse`. It
// answered `coreutils: unknown program 'posse'` and exited 1 inside the test,
// a few milliseconds in, and both reds landed on the read-time guard after
// the install rather than before it. ranger-base-x78s5 hit the same class in
// scripts/reap-test-tmp.sh with /usr/bin/tee, where no argv[0] spelling could
// fix it either; docs/notes.d/ranger-base-x0ses.md carries the .deb readings
// and docs/notes.d/ranger-base-f177g.md this fixture's.
//
// So the rule the donor is chosen by: it must run the same whatever its
// basename is, and it must not be picked for its NAME. Picking a system
// utility for its name is how this bug was written twice, and the next
// distro's build of the next utility is the next nine days. bash dispatches
// on nothing, it is the shell this suite already runs under, and with `-c` it
// HONORS its argv, which is where rrgncLoopLife goes. There is no fallback to
// another utility on purpose: a box with no bash skips, and says so.
//
// A `#!` wrapper is REFUSED and not worked around. A shebang exec rebuilds
// argv and leaves /proc/<pid>/exe reading the INTERPRETER, and that link is
// the one fact every assertion in this file is taken from.
func rrgncDonor(t *testing.T) string {
	t.Helper()
	donor, err := exec.LookPath("bash")
	if err != nil {
		t.Skipf("no bash(1) to stand in for a loop's image, and this fixture takes no other donor (see rrgncDonor): %v", err)
	}
	magic, err := rrgncMagic(donor)
	if err != nil {
		t.Skipf("cannot read the donor %s to tell a binary from a stub: %v", donor, err)
	}
	if magic == "#!" {
		t.Skipf("the donor %s is a %q stub and not a binary: a shebang exec leaves /proc/<pid>/exe reading the interpreter, which is the fact every reading here is taken from (ranger-base-x78s5)", donor, magic)
	}
	return donor
}

// rrgncMagic is a file's first two bytes, which is what tells a binary from a
// `#!` stub — in the refusal above and in the postmortem below.
func rrgncMagic(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	var head [2]byte
	if _, err := io.ReadFull(f, head[:]); err != nil {
		return "", err
	}
	return string(head[:]), nil
}

// rrgncLoop is a started stand-in loop, the donor it came from, and the file
// its own output lands in. The output is here because the one line that
// settles a donor surprise is the donor's OWN complaint, and the guard that
// reports these failures would otherwise have no handle on it: the two CI
// reds this shape was written for arrived in rrgncChildStillOurs, and the
// line that explained them was printed by nothing (ranger-base-f177g).
type rrgncLoop struct {
	cmd   *exec.Cmd
	donor string // the file on PATH the bytes came from, per rrgncDonor
	bin   string // the copy of them this loop was exec'd from
	say   string // the file holding the loop's own stdout and stderr
}

// Pid is the loop's pid — the thing every reading in this file is about.
func (l *rrgncLoop) Pid() int { return l.cmd.Process.Pid }

// rrgncStartLoop starts a stand-in dispatch loop from bin and does not return
// until /proc says that pid really runs it.
//
// THE WAIT IS THE POINT, and it is the second half of what cost a red suite.
// `exec.Cmd.Start` returns once the child's exec has been reported, but
// `/proc/<pid>/exe` is not reliably the new image by the time the PARENT is
// next scheduled: MEASURED 2026-11-21, 2 failures in 20 runs of this file's
// own fixtures, each reading the forked-but-not-yet-settled child as the TEST
// BINARY (`/tmp/go-build…/posse.test`) a fifth of a second into the test. The
// same window is why `cmd/checkorphans` refuses to judge a process that is
// not "old enough not to be a fork/exec teardown window" (AGENTS.md) — this
// package had simply never waited for it.
//
// A fixture may re-establish its own precondition, and that is all this does:
// the product code under test is unchanged and is not being accommodated.
// What it must never do is proceed on an unsettled reading, because then the
// assertion below it is about a stranger's image and the failure arrives as a
// verdict about staleness.
func rrgncStartLoop(t *testing.T, donor, bin string) *rrgncLoop {
	t.Helper()
	// The LIFE is argv and the BLOCK is a pipe. `read -t <n>` with nothing to
	// read waits in the kernel, burning no CPU, and returns either when the
	// timer fires or the moment the write end below goes away — so an orphan
	// is bounded by THIS PROCESS's death and not only by rrgncLoopLife, which
	// is strictly tighter than the `sleep <n>` this replaced. MEASURED
	// 2026-11-21, and in both directions: docs/notes.d/ranger-base-f177g.md.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	loop := exec.Command(bin, "-c", "read -t "+rrgncLoopLife+" -r _")
	loop.Stdin = r
	// Set and not inherited: BASH_ENV would make this child a shell that
	// sources a file, and nothing about this fixture wants that.
	loop.Env = []string{}
	say, err := os.CreateTemp(t.TempDir(), "loop-*.out")
	if err != nil {
		t.Fatal(err)
	}
	loop.Stdout, loop.Stderr = say, say
	l := &rrgncLoop{cmd: loop, donor: donor, bin: bin, say: say.Name()}
	if err := loop.Start(); err != nil {
		t.Fatalf("the donor at %s would not start: %v", bin, err)
	}
	r.Close()
	// The Cleanup is also what keeps `w` REACHABLE. An *os.File that nothing
	// references can be closed by its finalizer, and closing this one hands
	// the child EOF — a stand-in loop that exits mid-test, which is the exact
	// failure rrgncChildStillOurs exists to report.
	t.Cleanup(func() {
		loop.Process.Kill()
		loop.Wait()
		w.Close()
		say.Close()
	})
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, path, ok := procExe(l.Pid()); ok && path == bin {
			return l
		}
		if time.Now().After(deadline) {
			_, path, ok := procExe(l.Pid())
			t.Fatalf("pid %d never settled onto %s (last reading: %q ok=%v) — a fixture that proceeds here asserts against a stranger's image%s", l.Pid(), bin, path, ok, rrgncPostmortem(l))
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// rrgncChildStillOurs fails loudly when the pid a test recorded is no longer
// the child it started. It is the READ-TIME half: rrgncStartLoop settles the
// child before it is recorded, and this says so again before each reading, so
// a child that dies mid-test (or a pid reused after it) arrives as "your
// fixture died" and never as a verdict about staleness.
func rrgncChildStillOurs(t *testing.T, l *rrgncLoop) {
	t.Helper()
	_, path, ok := procExe(l.Pid())
	if !ok {
		t.Fatalf("pid %d: this fixture's stand-in loop is gone before the reading — the assertions below would be about a stranger%s", l.Pid(), rrgncPostmortem(l))
	}
	if path != l.bin {
		t.Fatalf("pid %d now runs %s, not this fixture's %s — so every reading below is a stranger's (see rrgncStartLoop and rrgncLoopLife)%s", l.Pid(), path, l.bin, rrgncPostmortem(l))
	}
}

// rrgncPostmortem prints OBSERVATIONS about a loop that did not behave, and
// never the fixture's intentions about it. That is the whole lesson of the
// family this fixture's donor came from: two CI runs could not say whether an
// exec had even taken, because the rig reprinted the variable it MEANT to
// pass (ranger-base-x0ses). The donor's own stderr is the line that names the
// mechanism — `coreutils: unknown program 'posse'` is what nine days of this
// family were spent not reading.
func rrgncPostmortem(l *rrgncLoop) string {
	var b strings.Builder
	b.WriteString("\n    postmortem, as observed:")
	fmt.Fprintf(&b, "\n      donor   %s%s (chosen by rrgncDonor)", l.donor, rrgncShape(l.donor))
	fmt.Fprintf(&b, "\n      argv    %q", l.cmd.Args)
	if id, path, ok := procExe(l.Pid()); ok {
		fmt.Fprintf(&b, "\n      /proc/%d/exe -> %s (%s)", l.Pid(), path, id)
	} else {
		fmt.Fprintf(&b, "\n      /proc/%d/exe unreadable — the loop is gone or is not ours", l.Pid())
	}
	if st, err := os.Stat(l.bin); err == nil {
		fmt.Fprintf(&b, "\n      the copy it ran, %s%s: %d bytes, mode %v, %s", l.bin, rrgncShape(l.bin), st.Size(), st.Mode(), fileIdentity(l.bin))
	} else {
		fmt.Fprintf(&b, "\n      the copy it ran, %s: %v", l.bin, err)
	}
	out, err := os.ReadFile(l.say)
	if err != nil {
		fmt.Fprintf(&b, "\n      its own output: unreadable: %v", err)
	} else {
		fmt.Fprintf(&b, "\n      its own output: %q", strings.TrimSpace(string(out)))
	}
	b.WriteString("\n      a donor that answers \"unknown program\" there dispatches on its own path's stem and cannot be this fixture's loop — see rrgncDonor")
	return b.String()
}

// rrgncShape says whether a file is a binary or a `#!` stub, for the
// postmortem's two file lines. It is a reading and not a claim: a path that
// cannot be read says so rather than reading as either one.
func rrgncShape(path string) string {
	magic, err := rrgncMagic(path)
	switch {
	case err != nil:
		return fmt.Sprintf(" (first two bytes unreadable: %v)", err)
	case magic == "#!":
		return fmt.Sprintf(" (first two bytes %q, so a '#!' stub — rrgncDonor must refuse one)", magic)
	default:
		return fmt.Sprintf(" (first two bytes %q, so a binary)", magic)
	}
}

// rrgncCopy writes src's bytes to dst, executable, through a temp file and a
// rename — the swap ADR 0092 D3.5 does.
func rrgncCopy(t *testing.T, src, dst string) {
	t.Helper()
	body, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	// WriteExecutable and not a bare write: a sibling's fork inside the write
	// window would make the exec below answer ETXTBSY (ranger-base-d26ak).
	tmp := dst + ".new"
	if err := WriteExecutable(tmp, body, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		t.Fatal(err)
	}
}

func TestQAG18FiresWhenTheInstallRenamesOverALiveLoop(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the /proc road; the record road is pinned below on every OS")
	}
	t.Parallel()
	donor := rrgncDonor(t)
	root := t.TempDir()
	bin := filepath.Join(root, "posse")
	rrgncCopy(t, donor, bin)

	// The loop: a process running the installed file. It stamps nothing —
	// like every loop alive on the day this lands, started by a binary that
	// predates the record fields — so /proc is all G18 has.
	loop := rrgncStartLoop(t, donor, bin)

	a := &App{StateDir: filepath.Join(root, "state")}
	lock, held, err := lockWatch(a)
	if err != nil || held {
		t.Fatalf("lock: held=%v err=%v", held, err)
	}
	defer lock.Release()
	if err := WriteWatchPid(WatchPidPath(a), WatchPid{Pid: loop.Pid(), Started: time.Now()}); err != nil {
		t.Fatal(err)
	}

	rrgncChildStillOurs(t, loop)
	img, ok := ReadWatchImage(a)
	if !ok || img.Via != "proc" || img.Path != bin {
		t.Fatalf("before the swap the reading must come from /proc and name %s: ok=%v %+v", bin, ok, img)
	}
	if img.Stale() || len(img.GovRows()) != 0 {
		t.Fatalf("a loop running the installed file is not stale: %+v", img)
	}

	// The install.
	rrgncCopy(t, donor, bin)

	rrgncChildStillOurs(t, loop)
	img, ok = ReadWatchImage(a)
	if !ok || !img.Stale() {
		t.Fatalf("after a rename over the loop's path G18 must fire: ok=%v %+v", ok, img)
	}
	rows := img.GovRows()
	if len(rows) != 1 || rows[0].ID != "G18" || rows[0].Class != GovLane {
		t.Fatalf("want one LANE G18 row, got %+v", rows)
	}
	for _, want := range []string{"dispatcher pid", "restart the dispatch loop, then relaunch seats", "guardrail 3"} {
		if !strings.Contains(rows[0].Detail, want) {
			t.Errorf("G18 detail is missing %q: %s", want, rows[0].Detail)
		}
	}

	// And the governance surface carries it, through the seam the row is
	// wired with — not just the reading.
	// govIn's hermetic shop, with an all-clear control: the fixture CAN print
	// "nothing needs a human", and does until the reading is stale.
	b, _ := newTestBackend(t)
	clearIn, staleIn := govIn(t, b), govIn(t, b)
	cur := img
	cur.OnDisk = cur.Image
	clearIn.Image = func() (WatchImage, bool) { return cur, true }
	staleIn.Image = func() (WatchImage, bool) { return img, true }
	var clear, stale bytes.Buffer
	GovReport(&clear, shopSet(t, clearIn), nil)
	GovReport(&stale, shopSet(t, staleIn), nil)
	if !strings.Contains(clear.String(), "nothing needs a human") {
		t.Fatalf("the control arm raised something else — this fixture cannot state the claim:\n%s", clear.String())
	}
	if got := stale.String(); strings.Contains(got, "nothing needs a human") || !strings.Contains(got, "G18") {
		t.Errorf("a loop on a replaced image printed no G18 row — this IS the bead:\n%s", got)
	}

	// The negative control on the same loop: a restart onto the new file
	// clears it. (Here: the record names a pid whose image IS the new file.)
	fresh := rrgncStartLoop(t, donor, bin)
	if err := WriteWatchPid(WatchPidPath(a), WatchPid{Pid: fresh.Pid()}); err != nil {
		t.Fatal(err)
	}
	rrgncChildStillOurs(t, fresh)
	if img, ok := ReadWatchImage(a); !ok || img.Stale() {
		t.Errorf("a loop restarted onto the installed file must clear G18: ok=%v %+v", ok, img)
	}
}

// Where /proc cannot answer (darwin, another pid namespace), the loop's own
// stamp is the reading — and a loop too old to have stamped is an abstention.
func TestQAG18ReadsTheLoopsOwnStampWhereProcCannotAnswer(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	bin := filepath.Join(root, "posse")
	if err := os.WriteFile(bin, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	noProc := func(int) (string, string, bool) { return "", "", false }
	rec := &WatchPid{Pid: 4242, Exe: bin, Image: fileIdentity(bin), Version: "0.5.3+aaaaaaaa"}
	if rec.Image == "" {
		t.Skip("this platform's stat carries no inode")
	}

	if img := readWatchImage(rec, noProc); img.Via != "record" || img.Stale() {
		t.Fatalf("the stamped image is still installed: %+v", img)
	}
	tmp := bin + ".new"
	if err := os.WriteFile(tmp, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, bin); err != nil {
		t.Fatal(err)
	}
	img := readWatchImage(rec, noProc)
	if !img.Stale() || !strings.Contains(img.Sentence(), "posse 0.5.3+aaaaaaaa") {
		t.Fatalf("after the rename the stamp must read stale and name its version: %+v / %s", img, img.Sentence())
	}

	old := &WatchPid{Pid: 4242}
	if img := readWatchImage(old, noProc); img.Stale() || len(img.GovRows()) != 0 {
		t.Errorf("an unstamped loop with no /proc is an abstention, not a row: %+v", img)
	}
}

// The record round-trips the three stamp fields, and the stamp a starting
// loop writes names this process's own image.
func TestQAWatchPidCarriesTheLoopsImage(t *testing.T) {
	t.Parallel()
	a := &App{StateDir: filepath.Join(t.TempDir(), "state")}
	d := &Dispatcher{App: a, Out: io.Discard, Err: io.Discard}
	d.stampWatchPid()
	rec, ok := ReadWatchPid(WatchPidPath(a))
	if !ok {
		t.Fatal("no record")
	}
	id, path := selfImage()
	if rec.Image != id || rec.Exe != path || rec.Version != VersionString() {
		t.Errorf("stamp = exe %q image %q version %q; want %q %q %q", rec.Exe, rec.Image, rec.Version, path, id, VersionString())
	}
	if id == "" {
		t.Errorf("this process could not name its own image")
	}
}

// D6's line before the last one: it names the stale loop, and it is absent
// when the plan carries no reader (every pin that does not ask).
func TestQAUpgradeNamesTheLoopTheRenameLeftBehind(t *testing.T) {
	t.Parallel()
	if got := upgradeWatchLine(upgradePlan{}); got != "" {
		t.Errorf("no reader must print nothing, got %q", got)
	}
	stale := WatchImage{Pid: 77, Path: "/x/posse", Image: "1:2", OnDisk: "1:3", Version: "0.5.3+old"}
	got := upgradeWatchLine(upgradePlan{Loop: func() (WatchImage, bool) { return stale, true }})
	for _, want := range []string{"dispatch: dispatcher pid 77 runs posse 0.5.3+old", "restart the dispatch loop, then relaunch seats"} {
		if !strings.Contains(got, want) {
			t.Errorf("line is missing %q: %s", want, got)
		}
	}
	cur := WatchImage{Pid: 77, Path: "/x/posse", Image: "1:3", OnDisk: "1:3"}
	if got := upgradeWatchLine(upgradePlan{Loop: func() (WatchImage, bool) { return cur, true }}); !strings.Contains(got, "already runs") {
		t.Errorf("a current loop must say so: %s", got)
	}
}

// ─── the reading's scope (ranger-base-nqaa1, finding 1) ──────────────────────

// EVERY rendering of a loop reading says it read one instance's loop, and
// names which. The verdict branches are not the interesting ones: a stale
// loop is already being told to restart, so the clause only sharpens it.
// What this pin is for is the other two — "already runs" and "could not be
// read" — because those are the lines an operator finishes believing the BOX
// is covered, and the binary the rename replaced is the box's.
func TestQAEveryLoopReadingSaysWhichInstanceItRead(t *testing.T) {
	t.Parallel()
	const home = "/home/posse/homes/ranger"
	// The clause has to name the instance and has to say the subject is
	// wider than the reading. "loops" plural is ADR 0092 D6's own word for
	// the remedy, and the whole finding is that the plural was being read as
	// satisfied by a singular reading.
	says := func(t *testing.T, what, line string) {
		t.Helper()
		for _, want := range []string{"scope: one instance's loop", home, "box-wide", "restart the dispatch loops", "sibling instances this reading did not look at"} {
			if !strings.Contains(line, want) {
				t.Errorf("%s does not say %q:\n  %s", what, want, line)
			}
		}
	}

	stale := WatchImage{Pid: 77, Path: "/x/posse", Image: "1:2", OnDisk: "1:3", Version: "0.5.3+old", Home: home}
	says(t, "the G18 sentence", stale.Sentence())
	rows := stale.GovRows()
	if len(rows) != 1 {
		t.Fatalf("want one G18 row, got %+v", rows)
	}
	says(t, "the G18 row's detail", rows[0].Detail)
	says(t, "the verb's line over a stale loop", upgradeWatchLine(upgradePlan{Loop: func() (WatchImage, bool) { return stale, true }}))

	cur := stale
	cur.OnDisk = cur.Image
	says(t, `the verb's "already runs" line`, upgradeWatchLine(upgradePlan{Loop: func() (WatchImage, bool) { return cur, true }}))

	// The abstention: ok=false, and the reading still names its instance —
	// which is the one line where "this home" was all the reader had.
	abstain := WatchImage{Home: home}
	says(t, "the verb's abstention line", upgradeWatchLine(upgradePlan{Loop: func() (WatchImage, bool) { return abstain, false }}))

	// A reader that could not name the instance says so instead of naming
	// nothing: the clause is never silently absent.
	anon := upgradeWatchLine(upgradePlan{Loop: func() (WatchImage, bool) { return WatchImage{}, false }})
	if !strings.Contains(anon, "scope: one instance's loop — this instance's") {
		t.Errorf("a reading with no instance name still has to carry the clause:\n  %s", anon)
	}
}

// THE READER'S OWN ABSTENTIONS, which is the half the pin above cannot see:
// every arm of it hand-builds the WatchImage it hands upgradeWatchLine, so
// none of them observes what ReadWatchImage puts there. ReadWatchImage's own
// comment says Home is "set on every return and not only on the readings",
// and until this pin nothing held it: the whole tree carried exactly one
// img.Home assertion and it was on the ok=true path.
//
// FOUND BY MUTATION (ranger-base-di4bs, finding 6 of ranger-base-zzhen's
// verify pass): `return WatchImage{Home: home}, false` → `return
// WatchImage{}, false` at EITHER of watchimage.go's two abstention returns
// left the suite green. What a reader is handed then is `posse upgrade`
// saying "scope: one instance's loop — this instance's" over a box with two
// of them — the line watchimage.go calls "the one place where saying WHICH
// home matters most", and the line an operator finishes believing the box was
// covered.
//
// ONE ARM PER RETURN, because they are two statements and a mutant can drop
// either. NOT the two-home /proc fixture below, which the fix shape on the
// bead named: killing a loop there leaves the watch lock held by the test and
// the pid record readable, so the reader reaches `readWatchImage` and answers
// ok=TRUE — neither abstention is on that road. These two arms are the road,
// and they need no /proc, so they run on every OS.
func TestQAAnAbstainingLoopReadingStillNamesTheInstanceItRead(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, tc := range []struct {
		name  string
		dir   string
		setUp func(*testing.T, *App)
	}{
		{
			// Nothing holds this home's watch lock: the reading every
			// `posse upgrade` on a box that is not dispatching takes.
			name:  "no loop is running",
			dir:   "quiet",
			setUp: func(*testing.T, *App) {},
		},
		{
			// A loop IS running and the record cannot name its pid — the
			// second return, and a different sentence to whoever reads it.
			name: "a running loop whose record names no pid",
			dir:  "unrecorded",
			setUp: func(t *testing.T, a *App) {
				lock, held, err := lockWatch(a)
				if err != nil || held {
					t.Fatalf("this home's watch lock could not be taken: held=%v err=%v", held, err)
				}
				t.Cleanup(lock.Release)
				if running, err := WatchLoopRunning(a); err != nil || !running {
					t.Fatalf("the fixture reads no running loop (running=%v err=%v), so this arm would measure the OTHER abstention", running, err)
				}
				if _, ok := ReadWatchPid(WatchPidPath(a)); ok {
					t.Fatalf("%s already names a pid, so this arm would not abstain at all", WatchPidPath(a))
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			home := filepath.Join(root, tc.dir)
			a := &App{Home: home, StateDir: filepath.Join(home, "state")}
			tc.setUp(t, a)

			img, ok := ReadWatchImage(a)

			if ok {
				t.Fatalf("the reading answered ok with %s, so this arm measured a reading and not an abstention: %+v", tc.name, img)
			}
			if img.Home != home {
				t.Errorf("the abstention names its instance as %q, want %q — this field is the scope of what was NOT read, and a blank one reads as the box (ranger-base-nqaa1)", img.Home, home)
			}
			// And the sentence the caller prints, because that is where the
			// field is spent rather than where it is set.
			// AbbrevHome because that is the spelling the sentence carries;
			// a temp home under $HOME would otherwise read as `~/...` here
			// and nowhere else.
			line := upgradeWatchLine(upgradePlan{Loop: func() (WatchImage, bool) { return img, ok }})
			if !strings.Contains(line, "scope: one instance's loop — "+AbbrevHome(home)) {
				t.Errorf("the verb's abstention line does not name the home the reading came from:\n  %s", line)
			}
		})
	}
}

// TWO INSTANCES, TWO LOOPS, ONE BINARY — the box this finding was measured
// on, as a fixture. MEASURED 2026-11-21: pid 2066610 under ~/homes/ranger and
// pid 1991667 under ~/homes/weller, both exec'ing ~/.local/bin/posse at
// dev:ino 54:12591441.
//
// What it pins is the SHAPE and not a widening: each instance's reading names
// its own loop and nothing of the other's, so an install's report and that
// instance's G18 row cover one of the two loops the rename left behind. The
// clause above is what carries that to the reader. Widening the reading needs
// a notion of "the instances on this box", which exists nowhere in this tree.
func TestQAALoopReadingCoversItsOwnInstanceAndNotItsSiblings(t *testing.T) {
	// FIRST, before the GOOS skip, which is the order `make verify-parallel`
	// reads: it takes the body's first statement and nothing else, and this
	// test is env-clean and writes no package var, so it is eligible.
	t.Parallel()
	if runtime.GOOS != "linux" {
		t.Skip("the /proc road; the record road is pinned above on every OS")
	}
	donor := rrgncDonor(t)
	root := t.TempDir()
	// ONE binary, as the box has one: both loops exec this file and the
	// rename below replaces it under both of them.
	bin := filepath.Join(root, "bin", "posse")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	rrgncCopy(t, donor, bin)

	type instance struct {
		app  *App
		loop *rrgncLoop
		pid  int
		home string
	}
	var insts []instance
	for _, name := range []string{"ranger", "weller"} {
		home := filepath.Join(root, "homes", name)
		loop := rrgncStartLoop(t, donor, bin)

		a := &App{Home: home, StateDir: filepath.Join(home, "state")}
		lock, held, err := lockWatch(a)
		if err != nil || held {
			t.Fatalf("%s: lock held=%v err=%v — each instance holds its OWN watch lock", name, held, err)
		}
		defer lock.Release()
		if err := WriteWatchPid(WatchPidPath(a), WatchPid{Pid: loop.Pid(), Started: time.Now()}); err != nil {
			t.Fatal(err)
		}
		insts = append(insts, instance{app: a, loop: loop, pid: loop.Pid(), home: home})
	}

	// The install: one rename, both loops left on the old inode.
	rrgncCopy(t, donor, bin)

	for i, in := range insts {
		other := insts[(i+1)%len(insts)]
		rrgncChildStillOurs(t, in.loop)
		img, ok := ReadWatchImage(in.app)
		if !ok || !img.Stale() {
			t.Fatalf("%s: after the rename this instance's loop is stale: ok=%v %+v", in.home, ok, img)
		}
		if img.Pid != in.pid {
			t.Errorf("%s: the reading names pid %d, want this instance's %d", in.home, img.Pid, in.pid)
		}
		if img.Home != in.home {
			t.Errorf("%s: the reading says its instance is %q", in.home, img.Home)
		}
		rows := img.GovRows()
		if len(rows) != 1 || rows[0].ID != "G18" {
			t.Fatalf("%s: want one G18 row, got %+v", in.home, rows)
		}
		// The claim, both halves: the row is about THIS loop, and it says
		// so rather than leaving the reader to assume the box.
		if strings.Contains(rows[0].Detail, strconv.Itoa(other.pid)) || strings.Contains(rows[0].Detail, other.home) {
			t.Errorf("%s: this instance's G18 row names the sibling's loop — the reading is per-instance and the row must not claim otherwise:\n  %s", in.home, rows[0].Detail)
		}
		if !strings.Contains(rows[0].Detail, "scope: one instance's loop — "+in.home) {
			t.Errorf("%s: the row does not say whose loop it read:\n  %s", in.home, rows[0].Detail)
		}
	}

	// And the sibling's staleness is real, not an artifact of reading it
	// through the first instance's App: the box genuinely has two stale
	// loops after one rename, which is the whole finding.
	a, b := insts[0], insts[1]
	ia, _ := ReadWatchImage(a.app)
	ib, _ := ReadWatchImage(b.app)
	if ia.Pid == ib.Pid {
		t.Fatalf("the fixture read the same loop twice: %d", ia.Pid)
	}
	if !ia.Stale() || !ib.Stale() {
		t.Errorf("one rename left both loops stale; got %+v and %+v", ia, ib)
	}
}
