//go:build linux

package posse

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// RunWallWatch is the sidecar's entry point: it returns the exit code the
// pane should see — bwrap's own when the seat ends by itself.
func RunWallWatch(argv []string, stderr io.Writer) int {
	o, err := parseWallWatchArgv(argv)
	if err != nil {
		fmt.Fprintln(stderr, "posse:", err)
		return 2
	}
	// fd 3 is the args file the pane shell opened for bwrap.
	args := os.NewFile(3, "bwrap.args")
	var st unix.Stat_t
	if args == nil || unix.Fstat(3, &st) != nil {
		fmt.Fprintln(stderr, "posse: wall-watch: fd 3 is not open — the launch line opens the bwrap args file there (3<'…/bwrap.args')")
		return 2
	}
	return runWallWatch(wallWatchRun{
		Opts: o, Args: args,
		Stdin: os.Stdin, Stdout: os.Stdout, Stderr: stderr,
		Store: NewBd(), Lock: wallWatchLockPath(),
		Signals: true,
	})
}

type bwrapInfo struct {
	ChildPid     int    `json:"child-pid"`
	MntNamespace uint64 `json:"mnt-namespace"`
}

func runWallWatch(r wallWatchRun) int {
	say := func(format string, a ...any) { fmt.Fprintf(r.Stderr, "posse wall-watch: "+format+"\n", a...) }
	now := r.Now
	if now == nil {
		now = time.Now
	}
	if r.PollInterval == 0 {
		r.PollInterval = wallWatchInterval
	}
	b, err := os.ReadFile(r.Opts.Mounts)
	if err != nil {
		say("cannot read the walls to hold this seat to (%v) — refusing to start the seat unwatched (ranger-base-0qpsw D1)", err)
		return 2
	}
	watch, err := parseWallWatchMounts(b)
	if err != nil {
		say("%v — refusing to start the seat unwatched", err)
		return 2
	}

	infoR, infoW, err := os.Pipe()
	if err != nil {
		say("pipe: %v", err)
		return 2
	}
	blockR, blockW, err := os.Pipe()
	if err != nil {
		say("pipe: %v", err)
		return 2
	}
	bw := wallWatchBwrapArgv(r.Opts.Bwrap)
	cmd := exec.Command(bw[0], bw[1:]...)
	if r.Stdin != nil {
		cmd.Stdin = r.Stdin
	}
	cmd.Stdout, cmd.Stderr = r.Stdout, r.Stderr
	cmd.ExtraFiles = []*os.File{r.Args, infoW, blockR} // fds 3, 4, 5
	var sigs chan os.Signal
	if r.Signals {
		// The tty's own signals reach bwrap and the seat directly — they
		// share this process group — so the watcher only keeps them from
		// ending ITSELF. A handled signal, not an ignored one: SIG_IGN would
		// be inherited across bwrap's exec into the seat. Registered before
		// the start so none lands in between and ends the watcher alone.
		sigs = make(chan os.Signal, 4)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGHUP)
		defer signal.Stop(sigs)
	}
	if err := cmd.Start(); err != nil {
		say("cannot start bwrap: %v", err)
		return 2
	}
	// From here on every return hands the terminal back to the pane's shell,
	// which is OUTSIDE the cage, so whatever was typed at the seat and not
	// read by it goes with nobody (ranger-base-heypq).
	defer wallWatchDiscardTyped(r.Stdin)
	infoW.Close()
	blockR.Close()
	if sigs != nil {
		// A TERM or HUP aimed at the watcher alone (a pane teardown, an
		// operator's kill of the pane pid) is passed on to bwrap, so the seat
		// ends with its line instead of outliving the watcher's exit.
		proc := cmd.Process
		go func() {
			for s := range sigs {
				if s == syscall.SIGTERM || s == syscall.SIGHUP {
					_ = proc.Signal(s)
				}
			}
		}()
	}
	done := make(chan int, 1)
	go func() {
		err := cmd.Wait()
		code := 0
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
			if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
				code = 128 + int(ws.Signal())
			}
		} else if err != nil {
			code = 1
		}
		done <- code
	}()

	var info bwrapInfo
	derr := json.NewDecoder(infoR).Decode(&info)
	infoR.Close()
	if derr != nil || info.ChildPid <= 0 {
		// bwrap failed before it had a seat (a bad args file, a refused
		// namespace). Its own stderr already said why.
		blockW.Close()
		return <-done
	}
	d := wallWatchDetach{Seat: r.Opts.Seat, Dir: r.Opts.Dir, BwrapPid: cmd.Process.Pid, SeatPid: info.ChildPid}

	respond := func(missing []WallWatchMount, atLaunch bool) int {
		d.At, d.Missing, d.AtLaunch = now(), missing, atLaunch
		d.Killed, d.KillErr = wallWatchKill(cmd.Process.Pid, info.MntNamespace)
		blockW.Close()
		<-done
		say("KILLED seat %s: the wall over %s came off at %s (ranger-base-0qpsw D2)", d.Seat, wallWatchPaths(missing), d.At.Format(time.RFC3339Nano))
		if r.OnDetach != nil {
			r.OnDetach(d)
		}
		filed, commented, ferr := fileWallWatchDetach(r.Store, d, now(), r.Lock)
		if filed != "" {
			say("filed %s", filed)
		}
		for _, id := range commented {
			say("recorded on %s (the same path, inside ten minutes)", id)
		}
		if ferr != nil {
			say("⚠ the record is incomplete: %v — this line is the record", ferr)
		}
		return 1
	}

	// bwrap writes the info fd right after clone, BEFORE the seat's mounts
	// are up, and the child blocks on the block fd only once they are. So the
	// table is opened only when the child is parked in that read — opened
	// earlier, it would render paths against the root the child had before
	// its pivot, and its first read would be the host's table.
	if err := wallWatchSeatReady(info.ChildPid, 5); err != nil {
		say("%v — killing the seat rather than running it unwatched", err)
		_, _ = wallWatchKill(cmd.Process.Pid, info.MntNamespace)
		blockW.Close()
		<-done
		return 1
	}
	mi, err := os.Open("/proc/" + strconv.Itoa(info.ChildPid) + "/mountinfo")
	if err != nil {
		say("cannot open the seat's mount table (%v) — killing the seat rather than running it unwatched", err)
		_, _ = wallWatchKill(cmd.Process.Pid, info.MntNamespace)
		blockW.Close()
		<-done
		return 1
	}
	defer mi.Close()
	live, err := readMountinfo(mi)
	if err != nil {
		say("cannot read the seat's mount table (%v) — killing the seat rather than running it unwatched", err)
		_, _ = wallWatchKill(cmd.Process.Pid, info.MntNamespace)
		blockW.Close()
		<-done
		return 1
	}
	base, missing := wallWatchBaseline(watch, live)
	if len(missing) > 0 {
		return respond(missing, true)
	}
	// Every wall is up: let the seat's command run.
	if _, err := blockW.Write([]byte{1}); err != nil {
		say("cannot release the seat (%v)", err)
	}
	blockW.Close()

	fds := []unix.PollFd{{Fd: int32(mi.Fd()), Events: unix.POLLPRI}}
	for {
		select {
		case code := <-done:
			return code
		default:
		}
		// THE EVENT IS NOT ENOUGH, so every wake diffs — the event's and the
		// interval's alike. A detach by rename does not reliably raise
		// POLLPRI. MEASURED 2026-11-13 (ranger-base-ihnnl, Linux 7.2.15,
		// bwrap 0.13.1, the live pin with the interval diff disabled): 3 of
		// 5 host renames over a watched Deny bind did NOT wake the poll —
		// the next POLLPRI came at the seat's exit — and 2 of 5 did.
		// ranger-base-0qpsw's ~1 ms reading was the event when it fires.
		// ASSUMED cause, not read in the kernel source: a namespace's
		// pollers are woken when the box-wide mount event counter has moved
		// past the one that namespace last saw, the detach path does not
		// move it, and the 2 that woke rode another mount elsewhere on the
		// box. Either way the poll is the fast path and the interval is the
		// guarantee.
		fds[0].Revents = 0
		_, perr := unix.Poll(fds, int(r.PollInterval/time.Millisecond))
		if perr != nil && !errors.Is(perr, unix.EINTR) {
			say("poll on the seat's mount table failed (%v) — killing the seat rather than running it unwatched", perr)
			_, _ = wallWatchKill(cmd.Process.Pid, info.MntNamespace)
			return <-done
		}
		live, err := readMountinfo(mi)
		if err != nil {
			// The seat is gone (its table went with it), or going.
			return <-done
		}
		if gone := wallWatchDetached(watch, base, live); len(gone) > 0 {
			return respond(gone, false)
		}
	}
}

// wallWatchDiscardTyped throws away terminal input nobody has read yet, as
// the watcher's last act (ranger-base-heypq).
//
// THE WATCHER CAN OUTLIVE ITS SEAT. Its kill path files a bead before it
// returns (respond), which is a bd call and seconds long, and in that time
// the watcher is the pane's foreground process while the agent behind it is
// dead. On a Rex instance the watcher is also the program_status record's
// OWNER — Rex records the terminal's foreground process-group leader, and
// the launch line makes the watcher that leader (MEASURED 2026-11-13 on this
// box: a block's `foreground` and its record's owner are the pid that equals
// the tty's tpgid) — so the liveness refusal in RexMux.Prompt reads a live
// owner and lets the keystrokes through. The watcher never reads its stdin,
// so the line discipline holds them, and the next reader is the pane's
// interactive shell: OBSERVED 2026-11-13, a bead prompt typed into a seat the
// watcher had just killed was evaluated by that shell, uncaged, command
// substitutions and all.
//
// A FLUSH ALONE IS NOT ENOUGH, because a flush reaches only what has already
// ARRIVED (ranger-base-69vw5). The terminal's input queue holds 4096 bytes
// and no dispatch work prompt is that small (MEASURED 2026-11-14, Linux
// 7.2.15: the smallest is 4211 bytes, a real one ~4600), so a prompt typed
// at a dead seat is still mid-write when the watcher returns: the queue is
// full, the sender is held behind it, and a flush only makes room for the
// rest — which then arrives into an empty queue whose next reader is the
// shell. So the watcher reads and discards until the sender has stopped
// (nothing for wallWatchDrainQuiet), with ICANON off for the drain so a line
// still waiting for its Enter is as readable as a whole one, then flushes,
// restores the terminal and flushes once more. wallWatchDrainMost bounds a
// sender that never stops. Nothing typed into a dead seat was meant for the
// shell, so none of it is kept.
//
// The drain reads only from a terminal the watcher may read: one whose
// foreground group is the watcher's own, or one that is not its controlling
// terminal at all. A background read of the controlling terminal would stop
// the watcher on SIGTTIN and hand the pane to the shell with the input
// unread; there it falls back to the flush. A stdin that is not a terminal
// has nothing to discard.
//
// Residual: ranger-base-ie4mx.
func wallWatchDiscardTyped(in *os.File) {
	discardTyped(in, wallWatchDrainQuiet, wallWatchDrainMost)
}

// The drain ends after this long with nothing typed, or after the second
// whatever the sender is doing. Rex types a prompt as one `rex send` and its
// Enter as a second call tens of ms later; the quiet window spans that gap,
// so the Enter goes with the text.
const (
	wallWatchDrainQuiet = 250 * time.Millisecond
	wallWatchDrainMost  = 5 * time.Second
)

func discardTyped(in *os.File, quiet, most time.Duration) {
	if in == nil {
		return
	}
	fd := int(in.Fd())
	old, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return // not a terminal
	}
	flush := func() { _ = unix.IoctlSetInt(fd, unix.TCFLSH, unix.TCIFLUSH) }
	if pg, err := unix.IoctlGetInt(fd, unix.TIOCGPGRP); err == nil && pg != unix.Getpgrp() {
		flush()
		return
	}
	raw := *old
	raw.Lflag &^= unix.ICANON
	raw.Cc[unix.VMIN], raw.Cc[unix.VTIME] = 0, 0
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &raw); err != nil {
		flush()
		return
	}
	buf := make([]byte, 32<<10)
	fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	deadline := time.Now().Add(most)
	for {
		wait := time.Until(deadline)
		if wait <= 0 {
			break
		}
		if wait > quiet {
			wait = quiet
		}
		fds[0].Revents = 0
		n, err := unix.Poll(fds, int(wait/time.Millisecond)+1)
		if err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			break
		}
		if n == 0 {
			break // quiet: the sender has stopped
		}
		if fds[0].Revents&unix.POLLIN == 0 {
			break // hung up or invalid: nobody is typing any more
		}
		if got, err := unix.Read(fd, buf); err != nil && !errors.Is(err, unix.EINTR) && !errors.Is(err, unix.EAGAIN) {
			break
		} else if got == 0 && err == nil {
			break
		}
	}
	flush()
	_ = unix.IoctlSetTermios(fd, unix.TCSETS, old)
	flush()
}

// readMountinfo rereads the table from the top through the fd poll(2) is
// armed on: a reread is what re-arms POLLPRI.
func readMountinfo(f *os.File) (mountTable, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	return parseMountinfo(b), nil
}

// wallWatchKill ends the seat: bwrap first, then every process still in the
// seat's mount namespace or descended from bwrap, until none is left. bwrap's
// --die-with-parent ends only its direct child, and anything the seat
// started — a backgrounded job, a setsid'd daemon — keeps its fds and its
// view until it is killed too. Several rounds, because a process can fork
// between a scan and its kill.
func wallWatchKill(bwrapPid int, mntns uint64) (killed []int, errText string) {
	seen := map[int]bool{}
	_ = syscall.Kill(bwrapPid, syscall.SIGKILL)
	killed = append(killed, bwrapPid)
	seen[bwrapPid] = true
	for round := 0; round < 20; round++ {
		pids := wallWatchSeatPids(bwrapPid, mntns)
		left := 0
		for _, p := range pids {
			if p == bwrapPid {
				continue
			}
			left++
			if err := syscall.Kill(p, syscall.SIGKILL); err == nil && !seen[p] {
				seen[p] = true
				killed = append(killed, p)
			}
		}
		if left == 0 {
			return killed, ""
		}
		time.Sleep(25 * time.Millisecond)
	}
	return killed, fmt.Sprintf("processes in the seat's mount namespace were still alive after 20 rounds of SIGKILL: %v", wallWatchSeatPids(bwrapPid, mntns))
}

// wallWatchSeatPids is the live process table's answer to "which of these
// is the seat": in its mount namespace, or a descendant of bwrap. A zombie
// holds no fd and no namespace, so it is not counted.
func wallWatchSeatPids(bwrapPid int, mntns uint64) []int {
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	parent := map[int]int{}
	var all []int
	inNS := map[int]bool{}
	for _, e := range ents {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		stat, err := os.ReadFile(filepath.Join("/proc", e.Name(), "stat"))
		if err != nil {
			continue
		}
		state, ppid, ok := procStatStateParent(string(stat))
		if !ok || state == 'Z' || state == 'X' {
			continue
		}
		all = append(all, pid)
		parent[pid] = ppid
		if mntns != 0 {
			var st unix.Stat_t
			if unix.Stat(filepath.Join("/proc", e.Name(), "ns", "mnt"), &st) == nil && st.Ino == mntns {
				inNS[pid] = true
			}
		}
	}
	var out []int
	for _, pid := range all {
		if inNS[pid] {
			out = append(out, pid)
			continue
		}
		for p, hops := parent[pid], 0; p > 1 && hops < 4096; p, hops = parent[p], hops+1 {
			if p == bwrapPid {
				out = append(out, pid)
				break
			}
		}
	}
	return out
}

// wallWatchSeatReadyWait bounds the wait for bwrap to finish setting up.
// Setup is a few dozen mounts; seconds of it is a seat that is not coming.
const wallWatchSeatReadyWait = 10 * time.Second

// wallWatchSeatReady waits until the seat's child is parked in read(2) on
// the block fd — bwrap reads it only once every mount is up — by reading
// /proc/<pid>/syscall: the syscall number, then its first argument.
//
// The child is this process's grandchild and in a user namespace this uid
// owns, so the file is readable to it. A kernel that cannot answer at all
// (no syscall file) refuses the seat: a baseline taken before setup is the
// host's table, and a watcher holding the seat to that watches nothing.
func wallWatchSeatReady(pid, fd int) error {
	want0, want1 := strconv.Itoa(unix.SYS_READ), "0x"+strconv.FormatInt(int64(fd), 16)
	path := "/proc/" + strconv.Itoa(pid) + "/syscall"
	deadline := time.Now().Add(wallWatchSeatReadyWait)
	var last string
	for {
		b, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("cannot tell whether the seat is set up: %v", err)
		}
		f := strings.Fields(string(b))
		if len(f) >= 2 && f[0] == want0 && f[1] == want1 {
			return nil
		}
		last = strings.TrimSpace(string(b))
		if time.Now().After(deadline) {
			return fmt.Errorf("the seat did not finish its setup within %s (its last syscall: %q)", wallWatchSeatReadyWait, last)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// wallWatchInterval is the longest a detach can go unseen when POLLPRI does
// not fire for it (the loop's comment says when that is). A reread of a
// seat's mount table is a few dozen lines, so four a second per seat is
// noise; the window it leaves is the gap ranger-base-0qpsw accepts, at this
// width rather than its ~1 ms.
const wallWatchInterval = 250 * time.Millisecond
