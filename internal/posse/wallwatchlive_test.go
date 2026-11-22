//go:build !posse_arm2 && !posse_arm3

package posse

// Live pin for ranger-base-ihnnl (ranger-base-0qpsw D1+D2): a real bwrap seat
// rendered by bwrapArgsWatched, watched by runWallWatch, loses a wall to a
// HOST rename — and is killed, with one bead filed. Skips off Linux, without
// bwrap, and inside a wall that forbids nesting, like the other live bwrap
// pins (bgLiveSkip's rule, ADR 0096 D3).
//
//	go test ./internal/posse -run TestLiveWallWatch -v

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

type wwLiveSeat struct {
	grant, priv, privSrc, deny, bgpid, mounts, args string
}

func wwLiveFixture(t *testing.T) wwLiveSeat {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("bwrap is the Linux mechanism of the seatbelt tier (ADR 0080 D16)")
	}
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap is not on PATH: " + err.Error())
	}
	// ADR 0096 D3: every seat below is a real bwrap, started by runWallWatch
	// from this process, so inside a caged seat there is no seat to watch.
	bwrapNestSkip(t)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := wwLiveSeat{
		grant:   filepath.Join(root, "cfg"),
		privSrc: filepath.Join(root, "seat", "shell-snapshots"),
		mounts:  filepath.Join(root, "bwrap.mounts"),
		args:    filepath.Join(root, "bwrap.args"),
	}
	s.priv = filepath.Join(s.grant, "shell-snapshots")
	s.deny = filepath.Join(s.grant, "settings.json")
	s.bgpid = filepath.Join(s.grant, "bg.pid")
	for _, d := range []string{s.grant, s.priv, s.privSrc} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(s.deny, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return s
}

// launch renders the seat, starts it under the watcher, and returns once the
// seat's command is running (it has written the pid of a background child
// it leaves behind, so the kill is judged on a process bwrap did not start).
func (s wwLiveSeat) launch(t *testing.T, st wallWatchStore, seat string, lock string) (code chan int, bwrapPid *int) {
	t.Helper()
	devnull, _ := os.Open(os.DevNull)
	t.Cleanup(func() { devnull.Close() })
	return s.launchOn(t, st, seat, lock, devnull)
}

// launchOn is launch with the watcher's stdin named — the pane's terminal,
// for a pin about what is typed at it (ranger-base-heypq).
func (s wwLiveSeat) launchOn(t *testing.T, st wallWatchStore, seat string, lock string, stdin *os.File) (code chan int, bwrapPid *int) {
	t.Helper()
	carve := SeatbeltCarveOut{
		Deny:     []string{s.deny},
		DenyFile: []string{s.deny},
		Private:  []PrivateBind{{Src: s.privSrc, SrcRoot: filepath.Dir(s.privSrc), Dst: s.priv}},
	}
	argv, prep, watch, err := bwrapArgsWatched(s.grant, []string{s.grant}, carve)
	if err != nil {
		t.Fatal(err)
	}
	// The launcher's own half, run as the launcher runs it. It used to
	// assert the render needed NO prep, which held only while the fixture
	// happened to have made everything the render wanted: ranger-base-ua625
	// gave the private bind's empty mount point an anchor seed and the
	// tripwire fired on a step a real launch makes without comment. A
	// fixture that refuses the launcher's prep is a fixture measuring a
	// wall no launch puts up.
	if err := runBwrapPrep(prep); err != nil {
		t.Fatalf("prep: %v", err)
	}
	if err := os.WriteFile(s.args, bwrapArgsBytes(argv), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.mounts, wallWatchMountsBytes(watch), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Remove(s.bgpid)
	af, err := os.Open(s.args)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { af.Close() })
	said := new(wwSaid)
	code = make(chan int, 1)
	pid := new(int)
	go func() {
		code <- runWallWatch(wallWatchRun{
			Opts: wallWatchOpts{Mounts: s.mounts, Seat: seat, Dir: s.grant,
				Bwrap: []string{"bwrap", "--args", "3", "--", "sh", "-c", "sleep 300 & echo $! > " + shellQuote(s.bgpid) + "; wait"}},
			Args: af, Stdin: stdin, Stdout: said, Stderr: said,
			Store: st, Lock: lock, PollInterval: 50 * time.Millisecond,
			OnDetach: func(d wallWatchDetach) { *pid = d.BwrapPid },
		})
	}()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if b, err := os.ReadFile(s.bgpid); err == nil && strings.TrimSpace(string(b)) != "" {
			break
		}
		select {
		case c := <-code:
			t.Fatalf("the seat ended before it ran (exit %d); watcher and bwrap said %s", c, wwOneLine(said.String()))
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("the seat never ran within 10s; watcher and bwrap said %s", wwOneLine(said.String()))
		}
		time.Sleep(20 * time.Millisecond)
	}
	return code, pid
}

// wwSaid is everything the watcher and its bwrap wrote, and it is a locked
// buffer rather than a strings.Builder because ONE of the two readers above
// reads it while the watcher is still writing: the deadline branch fires
// with the watcher's goroutine live, so a plain Builder shared across the
// two is a race whose value is whatever -race was not asked to look at.
// The other branch reads after a receive on `code` and would be safe alone.
type wwSaid struct {
	mu sync.Mutex
	b  strings.Builder
}

func (w *wwSaid) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

func (w *wwSaid) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.String()
}

// wwOneLine folds what the watcher said onto the FIRST line of the fatal
// that carries it, and that is ranger-base-a688i (ADR 0097 D3).
// scripts/posse-wall-check reports a failed member by its first
// `file.go:NN:` line and nothing after it (its evidence(), 300 chars), so a
// fatal reading `the seat ended before it ran (exit 1):` with the reason on
// line 2 reaches the bead as a symptom whose cause was dropped in transit —
// which is what that P1 was: one line, four possible causes, and no way to
// tell which fired. The words alone discriminate all four. A launch-time
// detach says `KILLED seat … the wall over … came off`; the watcher's own
// refusals say `refusing to start the seat unwatched` or `killing the seat
// rather than running it unwatched`; and a bwrap that died before it
// reported a child says whatever bwrap says, because cmd.Stdout and
// cmd.Stderr are this same buffer.
func wwOneLine(s string) string {
	f := strings.Fields(s)
	if len(f) == 0 {
		return "NOTHING — so no refusal of its own: bwrap ended before it reported a child, and without a word"
	}
	return strconv.Quote(strings.Join(f, " "))
}

// The fold is the behaviour, so it is pinned rather than only used: the two
// fatals above are reached by a live bwrap that this pin needs none of, and
// a fold that quietly started emitting a newline again would put the cause
// back out of the wall check's reach with every live arm still green.
// Not a member of the wall-check set and deliberately not named like one
// (`^Test(QA)?Live(Bwrap|Wall)`): it measures no wall.
func TestQAWallWatchFixtureFoldsWhatTheWatcherSaidOntoOneLine(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, in, want string
	}{
		{"a refusal's two lines become one", "posse wall-watch: cannot read the walls\nto hold this seat to\n",
			`"posse wall-watch: cannot read the walls to hold this seat to"`},
		{"bwrap's word and the watcher's are both carried", "bwrap: Can't find source path /x\nposse wall-watch: killing the seat\n",
			`"bwrap: Can't find source path /x posse wall-watch: killing the seat"`},
		{"one line is left alone but quoted, so a reader sees where it ends", "already one line",
			`"already one line"`},
	} {
		// One line, for the reason the function under test exists.
		if got := wwOneLine(c.in); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
	// Said NOTHING is its own answer and never an empty pair of quotes: a
	// bwrap that ended before it reported a child is the one cause that
	// leaves no words, so the absence has to read as a finding.
	for _, in := range []string{"", "\n", "  \n\t\n"} {
		got := wwOneLine(in)
		if strings.Contains(got, "\n") || !strings.HasPrefix(got, "NOTHING") {
			t.Errorf("wwOneLine(%q) = %q, want a one-line NOTHING finding", in, got)
		}
	}
	// And whatever it returns is ONE line, which is the whole contract.
	for _, in := range []string{"", "a\nb\nc", "a\r\nb", "\n\n\n"} {
		if got := wwOneLine(in); strings.ContainsAny(got, "\n\r") {
			t.Errorf("wwOneLine(%q) = %q — it carries a newline, so the wall check drops what is after it", in, got)
		}
	}
}

func (s wwLiveSeat) bg(t *testing.T) int {
	b, err := os.ReadFile(s.bgpid)
	if err != nil {
		t.Fatal(err)
	}
	p, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func wwWaitKilled(t *testing.T, code chan int, what string) {
	t.Helper()
	select {
	case c := <-code:
		if c != 1 {
			t.Errorf("%s: watcher exit = %d, want 1 (it killed the seat)", what, c)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("%s: the seat is still running 10s after its wall came off", what)
	}
}

func wwGone(t *testing.T, pid int, what string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for syscall.Kill(pid, 0) == nil {
		// A zombie still answers kill -0; its parent (sh, in the seat) is
		// dead, so init reaps it. Read the state instead of waiting on it.
		if b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat"); err == nil {
			if st, _, ok := procStatStateParent(string(b)); ok && st == 'Z' {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Errorf("%s: pid %d is still alive after the kill — a seat process outlived its wall", what, pid)
			syscall.Kill(pid, syscall.SIGKILL)
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// renameOver replaces `dst` on the host by rename, the way an atomic writer
// (or a purge) does.
func renameOver(t *testing.T, dst string, dir bool) {
	t.Helper()
	tmp := dst + ".new"
	if dir {
		// THE ANCHOR COMES OFF FIRST. A private bind's empty mount point
		// carries one since ranger-base-ua625, and rename(2) over a
		// directory needs the target EMPTY — so with the anchor in place
		// this rename is ENOTEMPTY, exactly as a host rmdir is. That is the
		// anchor working, and it is a wall this pin is not about: the
		// subject here is what wall-watch does once a wall IS off. So the
		// detach is made the way the one remover the anchor does not stop
		// would make it — empty the directory, then take the name — which
		// is the residual SeatbeltCarveOut.Anchored states in as many
		// words. TestQAWallAnchorKeepsClaudesSweptConfigDirsFromAHostRmdir
		// is where the refusal itself is held.
		if err := os.Remove(filepath.Join(dst, wallAnchorName)); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if err := os.Mkdir(tmp, 0o755); err != nil {
			t.Fatal(err)
		}
	} else if err := os.WriteFile(tmp, []byte("{\"replaced\":true}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// syscall.Rename and not os.Rename: os.Rename refuses an existing
	// directory as the target itself (EEXIST), which rename(2) does not.
	if err := syscall.Rename(tmp, dst); err != nil {
		ents, _ := os.ReadDir(dst)
		t.Fatalf("host rename over %s: %v (it holds %v)", dst, err, ents)
	}
}

// wwVerdict is the one durable verdict file this fixture's watchers have
// written (ranger-base-pfcci). Every seat below runs runWallWatch IN this
// process, so they all carry the same watcher pid and append to one file
// — in production that pid is one seat's watcher and the file is one
// seat's.
func wwVerdict(t *testing.T, s wwLiveSeat) string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(wallWatchRecordDir(s.mounts), "*.verdict"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("want one verdict file beside %s, got %v (%v)", s.mounts, paths, err)
	}
	b, rerr := os.ReadFile(paths[0])
	if rerr != nil {
		t.Fatal(rerr)
	}
	return string(b)
}

func TestLiveWallWatchKillsASeatWhoseWallComesOffAndFilesOneBead(t *testing.T) {
	t.Parallel()
	s := wwLiveFixture(t)
	st := &fakeWallWatchStore{}
	lock := filepath.Join(t.TempDir(), "wall-watch.lock")

	// 1. A Deny bind over a FILE, replaced on the host by rename.
	code, _ := s.launch(t, st, "seat-a", lock)
	bg := s.bg(t)
	renameOver(t, s.deny, false)
	wwWaitKilled(t, code, "deny")
	wwGone(t, bg, "deny")
	if len(st.issues) != 1 {
		t.Fatalf("deny: %d beads filed, want 1: %+v", len(st.issues), st.issues)
	}
	if d := st.issues[0].Description; !strings.Contains(d, wallWatchMarker(s.deny)) || !strings.Contains(d, "seat: seat-a") {
		t.Errorf("deny: the bead does not name the seat and the path:\n%s", d)
	}
	// And the verdict is on disk, not only in the pane this watcher just
	// closed behind it (ranger-base-pfcci).
	v := wwVerdict(t, s)
	for _, want := range []string{"seat seat-a", "KILLED seat seat-a", s.deny, "filed " + st.issues[0].ID} {
		if !strings.Contains(v, want) {
			t.Errorf("deny: the durable verdict lacks %q:\n%s", want, v)
		}
	}

	// 2. A Private bind over a DIRECTORY, replaced on the host by rename.
	// A different path, so a second bead.
	code, _ = s.launch(t, st, "seat-b", lock)
	bg = s.bg(t)
	renameOver(t, s.priv, true)
	wwWaitKilled(t, code, "private")
	wwGone(t, bg, "private")
	if len(st.issues) != 2 || !strings.Contains(st.issues[1].Description, wallWatchMarker(s.priv)) {
		t.Fatalf("private: want a second bead naming %s, got %+v", s.priv, st.issues)
	}

	// 3. The deny path again inside ten minutes: the seat is still killed,
	// and the detection is a comment on the first bead, not a third bead.
	code, _ = s.launch(t, st, "seat-c", lock)
	bg = s.bg(t)
	renameOver(t, s.deny, false)
	wwWaitKilled(t, code, "deny again")
	wwGone(t, bg, "deny again")
	if len(st.issues) != 2 {
		t.Errorf("deny again: %d beads, want still 2", len(st.issues))
	}
	if c := st.comments[st.issues[0].ID]; len(c) != 1 || !strings.Contains(c[0], "seat-c") {
		t.Errorf("deny again: comments on %s = %q, want one naming seat-c", st.issues[0].ID, c)
	}
	// Three detections, three blocks in the one file, and the third says
	// what it did instead of filing.
	v = wwVerdict(t, s)
	if n := strings.Count(v, "posse wall-watch verdict ·"); n != 3 {
		t.Errorf("the verdict file holds %d blocks, want 3:\n%s", n, v)
	}
	if !strings.Contains(v, "KILLED seat seat-c") || !strings.Contains(v, "recorded on "+st.issues[0].ID) {
		t.Errorf("the third detection is not in the file as a comment on the first bead:\n%s", v)
	}
}

// A seat whose walls stay up runs to its own end, and the watcher hands back
// its exit code.
func TestLiveWallWatchPassesTheSeatsOwnExitCode(t *testing.T) {
	t.Parallel()
	s := wwLiveFixture(t)
	carve := SeatbeltCarveOut{Deny: []string{s.deny}, DenyFile: []string{s.deny}}
	argv, _, watch, err := bwrapArgsWatched(s.grant, []string{s.grant}, carve)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(s.args, bwrapArgsBytes(argv), 0o644)
	os.WriteFile(s.mounts, wallWatchMountsBytes(watch), 0o644)
	af, _ := os.Open(s.args)
	defer af.Close()
	var out strings.Builder
	st := &fakeWallWatchStore{}
	got := runWallWatch(wallWatchRun{
		Opts: wallWatchOpts{Mounts: s.mounts, Seat: "x", Dir: s.grant,
			Bwrap: []string{"bwrap", "--args", "3", "--", "sh", "-c", "echo hi > " + shellQuote(s.deny) + " 2>/dev/null && exit 9; exit 7"}},
		Args: af, Stdout: &out, Stderr: &out, Store: st, PollInterval: 50 * time.Millisecond,
	})
	if got != 7 {
		t.Errorf("exit = %d, want 7 (the seat's own; a write to the denied file must have failed):\n%s", got, out.String())
	}
	if len(st.issues) != 0 {
		t.Errorf("a seat that kept its walls filed %d beads", len(st.issues))
	}
	if vs, _ := filepath.Glob(filepath.Join(wallWatchRecordDir(s.mounts), "*.verdict")); len(vs) != 0 {
		t.Errorf("a seat that kept its walls left a verdict behind: %v", vs)
	}
}

// A wall that is already off when the watcher first reads the table — here,
// a path in the list the render never mounted — stops the seat before its
// command runs.
func TestLiveWallWatchRefusesToReleaseASeatMissingAWallAtLaunch(t *testing.T) {
	t.Parallel()
	s := wwLiveFixture(t)
	argv, _, watch, err := bwrapArgsWatched(s.grant, []string{s.grant}, SeatbeltCarveOut{})
	if err != nil {
		t.Fatal(err)
	}
	watch = append(watch, WallWatchMount{WallWatchDeny, s.deny})
	os.WriteFile(s.args, bwrapArgsBytes(argv), 0o644)
	os.WriteFile(s.mounts, wallWatchMountsBytes(watch), 0o644)
	af, _ := os.Open(s.args)
	defer af.Close()
	ran := filepath.Join(s.grant, "ran")
	var out strings.Builder
	st := &fakeWallWatchStore{}
	got := runWallWatch(wallWatchRun{
		Opts: wallWatchOpts{Mounts: s.mounts, Seat: "x", Dir: s.grant,
			Bwrap: []string{"bwrap", "--args", "3", "--", "sh", "-c", ": > " + shellQuote(ran)}},
		Args: af, Stdout: &out, Stderr: &out, Store: st, PollInterval: 50 * time.Millisecond,
	})
	if got != 1 {
		t.Errorf("exit = %d, want 1:\n%s", got, out.String())
	}
	if _, err := os.Stat(ran); err == nil {
		t.Error("the seat's command ran with a wall missing")
	}
	if len(st.issues) != 1 || !strings.Contains(st.issues[0].Description, "at launch") {
		t.Errorf("want one bead saying the wall was missing at launch, got %+v", st.issues)
	}
	// The response here happens BEFORE the backstop record is ever
	// written, which is why the verdict path is settled ahead of the
	// baseline (ranger-base-pfcci).
	if rs, _ := filepath.Glob(filepath.Join(wallWatchRecordDir(s.mounts), "*.json")); len(rs) != 0 {
		t.Errorf("a seat that was never released left a backstop record: %v", rs)
	}
	if v := wwVerdict(t, s); !strings.Contains(v, "KILLED seat x") || !strings.Contains(v, s.deny) {
		t.Errorf("a wall missing at launch left no durable verdict:\n%s", v)
	}
}

// TestLiveWallWatchKillsASeatItsBwrapLeftRunning is ranger-base-6ec6n's
// supervisor-side kill. On an AppArmor host the exec into the seat clears the
// pdeathsig bwrap set, so a bwrap killed under its seat leaves the seat
// running, reparented and unsupervised. D1 has no AppArmor, so the fixture
// makes that seat the only honest way it can: --die-with-parent is taken off
// the line, which is exactly what the transition leaves behind.
//
// TWO ARMS, and the second is the control that keeps the kill narrow:
//   - bwrap is SIGKILLed with its seat's leader alive: the watcher must kill
//     the leader and the job it backgrounded, and say so on disk.
//   - the seat ends BY ITSELF, leaving a backgrounded job behind: bwrap exits
//     after its child as it always has, there is no leader left, and the
//     watcher kills nothing — what a seat leaves behind on its own exit is
//     not this fix's to end.
func TestLiveWallWatchKillsASeatItsBwrapLeftRunning(t *testing.T) {
	t.Parallel()
	s := wwLiveFixture(t)
	argv, _, watch, err := bwrapArgsWatched(s.grant, []string{s.grant}, SeatbeltCarveOut{Deny: []string{s.deny}, DenyFile: []string{s.deny}})
	if err != nil {
		t.Fatal(err)
	}
	var cut []string
	for _, a := range argv {
		if a != "--die-with-parent" {
			cut = append(cut, a)
		}
	}
	if len(cut) != len(argv)-1 {
		t.Fatalf("the render carries --die-with-parent %d time(s), want 1 — this fixture takes it off to model the AppArmor clear", len(argv)-len(cut))
	}
	if err := os.WriteFile(s.args, bwrapArgsBytes(cut), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.mounts, wallWatchMountsBytes(watch), 0o644); err != nil {
		t.Fatal(err)
	}
	leaderFile := filepath.Join(s.grant, "leader.pid")

	// THE PANE IS A FILE, as it is in production (os.Stdout): handed a
	// buffer, exec.Cmd copies through a pipe and its Wait then waits for
	// every holder of that pipe — the seat's backgrounded job included — so
	// a bwrap that is gone would not read as gone.
	run := func(t *testing.T, script string) (chan int, wwPaneFile, int, int) {
		os.Remove(s.bgpid)
		os.Remove(leaderFile)
		af, err := os.Open(s.args)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { af.Close() })
		pane, err := os.Create(filepath.Join(t.TempDir(), "pane"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { pane.Close() })
		said := wwPaneFile{pane}
		code := make(chan int, 1)
		go func() {
			code <- runWallWatch(wallWatchRun{
				Opts: wallWatchOpts{Mounts: s.mounts, Seat: "seat-o", Dir: s.grant,
					Bwrap: []string{"bwrap", "--args", "3", "--", "sh", "-c", script}},
				Args: af, Stdout: pane, Stderr: pane,
				Store: &fakeWallWatchStore{}, Lock: filepath.Join(t.TempDir(), "lock"), PollInterval: 50 * time.Millisecond,
			})
		}()
		read := func(p string) int {
			b, err := os.ReadFile(p)
			if err != nil {
				return 0
			}
			n, _ := strconv.Atoi(strings.TrimSpace(string(b)))
			return n
		}
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
			if l, b := read(leaderFile), read(s.bgpid); l > 0 && b > 0 {
				t.Cleanup(func() { _ = syscall.Kill(b, syscall.SIGKILL); _ = syscall.Kill(l, syscall.SIGKILL) })
				return code, said, l, b
			}
		}
		t.Fatalf("the seat never ran within 10s; watcher and bwrap said %s", wwOneLine(said.String()))
		return nil, wwPaneFile{}, 0, 0
	}

	t.Run("its bwrap is killed under a live leader", func(t *testing.T) {
		code, said, leader, bg := run(t, "echo $$ > "+shellQuote(leaderFile)+"; sleep 300 & echo $! > "+shellQuote(s.bgpid)+"; wait")
		b, err := os.ReadFile("/proc/" + strconv.Itoa(leader) + "/stat")
		if err != nil {
			t.Fatal(err)
		}
		_, bwrapPid, ok := procStatStateParent(string(b))
		if !ok || bwrapPid <= 1 {
			t.Fatalf("cannot read the seat leader's parent bwrap from %q", b)
		}
		_ = syscall.Kill(bwrapPid, syscall.SIGKILL) // the bwrap this test's watcher started
		select {
		case c := <-code:
			if c != 128+int(syscall.SIGKILL) {
				t.Errorf("watcher exit = %d, want %d (bwrap's own, killed)", c, 128+int(syscall.SIGKILL))
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("the watcher has not returned 10s after its bwrap was killed: %s", wwOneLine(said.String()))
		}
		wwGone(t, leader, "the orphaned seat's leader")
		wwGone(t, bg, "the orphaned seat's backgrounded job")
		v := wwVerdict(t, s)
		for _, want := range []string{"KILLED seat seat-o", "ranger-base-6ec6n", strconv.Itoa(leader)} {
			if !strings.Contains(v, want) {
				t.Errorf("the durable verdict lacks %q:\n%s", want, v)
			}
		}
	})

	t.Run("control: the seat ends by itself and its leftover is left alone", func(t *testing.T) {
		before, _ := filepath.Glob(filepath.Join(wallWatchRecordDir(s.mounts), "*.verdict"))
		var was string
		if len(before) == 1 {
			b, _ := os.ReadFile(before[0])
			was = string(b)
		}
		code, said, _, bg := run(t, "echo $$ > "+shellQuote(leaderFile)+"; sleep 300 & echo $! > "+shellQuote(s.bgpid)+"; sleep 0.3; exit 7")
		select {
		case c := <-code:
			if c != 7 {
				t.Errorf("watcher exit = %d, want 7 (the seat's own): %s", c, wwOneLine(said.String()))
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("the watcher has not returned 10s after its seat exited: %s", wwOneLine(said.String()))
		}
		if wuPidGone(bg) {
			t.Errorf("the seat's backgrounded job (pid %d) was killed after the seat ended by itself — the supervisor kill is for a LIVE leader whose bwrap is gone, not for what a finished seat left behind", bg)
		}
		after, _ := filepath.Glob(filepath.Join(wallWatchRecordDir(s.mounts), "*.verdict"))
		var now string
		if len(after) == 1 {
			b, _ := os.ReadFile(after[0])
			now = string(b)
		}
		if now != was {
			t.Errorf("a seat that ended by itself added to the verdict file:\n%s", strings.TrimPrefix(now, was))
		}
	})
}

// wwPaneFile is a watcher's pane as a real file, read back whole by String.
type wwPaneFile struct{ *os.File }

func (p wwPaneFile) String() string {
	b, _ := os.ReadFile(p.Name())
	return string(b)
}
