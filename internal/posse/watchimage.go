package posse

// G18 — the dispatch loop runs an image that is no longer the installed
// binary (ranger-base-rrgnc).
//
// WHY THIS IS A ROW. `posse upgrade` swaps ~/.local/bin/posse by rename(2)
// (ADR 0092 D3.5), and rename leaves a running process on its old inode. The
// `dispatch --watch` loop is such a process, and it is what RENDERS every
// launch: the gate shims, the bwrap/landlock argv, the settings payload. So
// after an install the file on disk carries the fixes and every launch since
// still renders the walls of the binary the loop started from. MEASURED
// twice on 2026-11-20/21 on linux: a launch six minutes after the
// edfe7a79..3c0e3c5e install read "rendered by posse 0.5.3+edfe7a79", and
// after 3c0e3c5e..0e1cb6c0 a fresh seat still carried hgcmw's pre-fix
// `--add-dir` line while the binary on disk had the fix. Nothing printed
// anything: G11 reads the ON-DISK binary against the repo, so it went quiet
// the moment the install landed — exactly when this gap opened.
//
// WHAT IT READS. The loop's identity is the (dev, inode) of the image it
// executes, compared with the (dev, inode) of the path it was started from,
// stat'ed now. Different means the path was replaced under the loop.
//
//   - On linux, /proc/<pid>/exe is the truth and needs no cooperation from the
//     loop: stat follows the magic link to the image even after it is
//     unlinked, and readlink names the path with " (deleted)" appended. So
//     this reads loops started by a binary that predates this file — which
//     is every loop alive on the day it lands.
//   - Anywhere /proc/<pid>/exe cannot be read (darwin; a seat in another pid
//     namespace), the loop's own record is used: since this bead the loop
//     stamps its image identity, path and version into dispatch-watch.pid at
//     start. A loop too old to have stamped them, on such a reader, is an
//     ABSTENTION and raises nothing — the same rule G11 keeps for a build
//     that names no commit.
//
// LANE, not URGENT, on G11's rule: ADR 0029 keeps URGENT for "the shop is
// stopped", and a stale loop keeps dispatching. The remedy is a restart of a
// live dispatcher, which is the operator's or the coordinator's (guardrail
// 3), so the row SAYS it and nothing here does it.

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// fileIdentity is "dev:ino" of what path names (following links), or "" when
// it cannot be stat'ed or the platform's stat carries no inode.
func fileIdentity(path string) string {
	st, err := os.Stat(path)
	if err != nil {
		return ""
	}
	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%d:%d", sys.Dev, sys.Ino)
}

// procExe reads /proc/<pid>/exe: the identity of the image the process runs
// and the path it was executed from. ok=false when the link is unreadable —
// not linux, not our process, not our pid namespace. A var so a pin can take
// the darwin road on a linux box.
var procExe = func(pid int) (id, path string, ok bool) {
	link := "/proc/" + strconv.Itoa(pid) + "/exe"
	target, err := os.Readlink(link)
	if err != nil {
		return "", "", false
	}
	id = fileIdentity(link)
	if id == "" {
		return "", "", false
	}
	return id, strings.TrimSuffix(target, " (deleted)"), true
}

// procStarted is WHEN the process at this pid started. It is the identity
// half of a pid, and `pidAlive`'s own comment in watchpid.go is the rule it
// serves: liveness "answers liveness and not identity — a recycled pid is
// alive and is a stranger — so no caller may use it alone as proof that the
// process it recorded is the one there." The field's standard answer is to
// verify a property of the PROCESS (argv, start time) and never the pid
// number; this shop already keeps that discipline for the husk check, and
// G18 did not (ranger-base-bekle).
//
// The timestamps of /proc/<pid> ITSELF, and not field 22 of
// /proc/<pid>/stat: the directory's mtime IS the task's start time as wall
// clock, so it needs neither the boot epoch nor USER_HZ to become one.
// MEASURED 2026-11-21 (linux 7.2.15-1-omarchy, go1.27.3): for a child this
// process had just forked, the mtime read 1.77µs BEFORE the parent's own
// time.Now() taken immediately before exec.Cmd.Start, was identical across
// four reads 1.2s apart, and answered ENOENT the instant the child was
// reaped; the two dispatch loops live on this box both read
// 2026-11-21T07:11:54 and pid 1 read the boot time. /proc/<pid> is also the
// read that stays answerable where /proc/<pid>/exe is refused — which is how
// ranger-base-rph5z tells a refused pid from a dead one — so the identity
// check does not evaporate on the reader a cage gives us.
//
// A var, like procExe above, so a pin can drive it.
var procStarted = func(pid int) (time.Time, bool) {
	st, err := os.Stat("/proc/" + strconv.Itoa(pid))
	if err != nil {
		return time.Time{}, false
	}
	return st.ModTime(), true
}

// selfImage is what a starting loop stamps about itself: the identity of the
// image it runs and the path it came from. /proc/self/exe first, because the
// path can be swapped between the exec and this read and the magic link
// cannot; os.Executable's path stat'ed otherwise, which is right unless an
// install lands inside that window.
func selfImage() (id, path string) {
	if id, path, ok := procExe(os.Getpid()); ok {
		return id, path
	}
	exe, err := os.Executable()
	if err != nil {
		return "", ""
	}
	return fileIdentity(exe), exe
}

// WatchImage is one reading of the running loop's image against its path.
type WatchImage struct {
	Pid     int
	Started time.Time
	Path    string // the path the loop was started from
	Image   string // dev:ino of the image the loop runs
	OnDisk  string // dev:ino of Path now; "" when nothing is there
	Version string // the loop's own build stamp, "" when it did not record one
	// Now is what Path holds now, by version, when the reader can say: the
	// reading process IS that file, so its compiled-in version is the
	// file's. "" otherwise — no exec of the target just to name it.
	Now string
	// Via is where Image came from: "proc" or "record".
	Via string
	// Home is WHOSE loop this is: the RHQ_HOME whose watch lock and
	// dispatch-watch.pid the reading came from (ranger-base-nqaa1). It is
	// here because the reading's scope is narrower than its subject's — see
	// watchScopeSentence — and every rendering has to say so. "" when the
	// reader could not name the instance, which reads as "this instance".
	Home string
	// Why is why there is NO reading, for the one abstention this reader can
	// explain rather than merely report: the lock says a loop of this home is
	// running and the record names a pid the record cannot be matched to
	// (ranger-base-bekle). "" on a reading, and on the abstentions that have
	// nothing to explain — no loop, no record.
	//
	// It is here because "no loop of this home could be read" is the wrong
	// sentence for this case and sends an operator looking for a loop that is
	// running: what could not be read is the record's claim about it.
	Why string
}

// watchScopeSentence is the one thing every rendering of a loop reading has
// to say about its own reach (ranger-base-nqaa1, finding 1).
//
// THE ASYMMETRY. `posse upgrade` renames ONE file, ~/.local/bin/posse, and
// that file is box-wide: every RHQ_HOME's `dispatch --watch` loop on this box
// execs it. A reading here is the opposite shape — it takes one `*App`, asks
// THAT home's watch lock whether a loop is running and reads THAT home's
// dispatch-watch.pid for its pid. So a box with two instances has one binary
// and two loops, and this reads one of them.
//
// MEASURED 2026-11-21, this box, linux 7.2.15-1-omarchy: two loops, pid
// 2066610 under ~/homes/ranger and pid 1991667 under ~/homes/weller, both
// exec'ing /home/posse/.local/bin/posse at dev:ino 54:12591441 — each with
// its own dispatch-watch.pid naming its own pid, so both are legitimate and
// the per-home lock invariant holds. Neither was stale at the time, so G18
// was correctly silent for both: this is COVERAGE and was not a live
// exposure. What made it worth a sentence is that nothing else would have
// said it — ShopCheck's G18 arm is per-instance (pulse.go, `posse status`,
// the cockpit), the weller instance has no pulse state at all so no tick
// there ever evaluates G18, and `posse upgrade` runs in the delegate's
// instance only. After an install, no reader on this box says a sibling
// loop is on the old inode.
//
// WHAT THIS IS NOT. It is not the fix for that coverage hole; widening the
// reading means coining a notion of "the instances on this box", which exists
// nowhere in this tree (nothing enumerates RHQ_HOMEs), and that is a design
// call with its own bead (ranger-base-is1qp). It is the fix for the READING
// OF THE REMEDY: ADR
// 0092 D6 says "restart the dispatch loops" in the plural, the verb's line
// and the G18 row name one, and without this clause a reader finishes either
// one believing the plural was satisfied by the singular.
func watchScopeSentence(home string) string {
	where := "this instance"
	if home != "" {
		where = AbbrevHome(home)
	}
	return "scope: one instance's loop — " + where + "'s, the pid its own state/dispatch-watch.pid names — while the binary an install renames is box-wide, so ADR 0092 D6's \"restart the dispatch loops\" reaches sibling instances this reading did not look at (ranger-base-nqaa1)"
}

// Stale is the condition: the path no longer names the image the loop runs.
func (w WatchImage) Stale() bool { return w.Image != "" && w.Image != w.OnDisk }

// Old names the loop's image for a human.
func (w WatchImage) Old() string {
	if w.Version != "" {
		return "posse " + w.Version
	}
	return "an image no longer at that path (inode " + w.Image + ")"
}

// Sentence is the one line both renderings — the G18 row and the upgrade
// verb's report — print, so the two cannot disagree.
func (w WatchImage) Sentence() string {
	now := "has been replaced since the loop started"
	if w.OnDisk == "" {
		now = "holds nothing"
	} else if w.Now != "" {
		now = "is posse " + w.Now
	}
	return fmt.Sprintf("dispatcher pid %d runs %s; %s %s; launches render the old walls until this loop restarts — restart the dispatch loop, then relaunch seats (ADR 0092 D6; restarting a live dispatcher is the operator's or the coordinator's, guardrail 3). %s",
		w.Pid, w.Old(), AbbrevHome(w.Path), now, watchScopeSentence(w.Home))
}

// GovRows is G18, the governance rendering of this reading.
func (w WatchImage) GovRows() []GovCondition {
	if !w.Stale() {
		return nil
	}
	return []GovCondition{{
		ID:     "G18",
		Class:  GovLane,
		Key:    "dispatch-image:" + strconv.Itoa(w.Pid) + ":" + w.Image,
		Detail: w.Sentence(),
	}}
}

// ReadWatchImage reads this home's loop — ONE instance's, which is what
// WatchImage.Home carries and watchScopeSentence says out loud
// (ranger-base-nqaa1). ok=false when no loop is running, its record names no
// pid, neither /proc nor the record can say what the loop runs, or the
// process holding the recorded pid is not the process the record describes
// (ranger-base-bekle — WatchImage.Why carries which) — every one an
// abstention, never an all-clear claimed and never a verdict about a pid
// this reader could not attribute.
//
// An abstention still names its instance: the caller that prints "no loop of
// this home could be read" is the one place where saying WHICH home matters
// most, so Home is set on every return and not only on the readings.
func ReadWatchImage(a *App) (WatchImage, bool) {
	home := watchInstance(a)
	running, err := WatchLoopRunning(a)
	if err != nil || !running {
		return WatchImage{Home: home}, false
	}
	rec, ok := ReadWatchPid(WatchPidPath(a))
	if !ok {
		return WatchImage{Home: home}, false
	}
	w := readWatchImage(rec, procExe, procStarted)
	w.Home = home
	if w.Via == "" {
		// The sentence above, kept: an abstention is never returned as a
		// reading. It used to return true here with an empty WatchImage,
		// which Stale() reads as "not stale" — so no caller claimed an
		// all-clear from it, but every caller had to know that to be safe,
		// and one of them (upgradeWatchLine) renders the two cases
		// differently. ok=false is what this reader documents.
		return w, false
	}
	return w, true
}

// watchInstance names the instance a reading came from. The RHQ_HOME, which
// is what an operator reading the line knows the instance by; the state dir
// when there is no home — a fixture, and the dir the pid record it wrote
// actually lives in.
func watchInstance(a *App) string {
	if a == nil {
		return ""
	}
	if a.Home != "" {
		return a.Home
	}
	return a.StateDir
}

// watchPidStranger is the identity half: whether the live process holding
// rec.Pid is the process rec describes. It answers only on POSITIVE evidence
// — why != "" means something read here says "not that process" — and
// checked says whether anything could be compared at all, which is the
// difference between "this is the loop" and "nobody can tell".
//
// Both comparisons are one-sided on purpose, because only one direction is
// evidence:
//
//   - A stamped image that is not the image /proc names. The image a process
//     runs cannot change without an exec, and rename(2) — the whole subject
//     of this file — does not touch it. So a mismatch is never this loop
//     having moved; it is a different process.
//   - A process that started AFTER the record was written. A process cannot
//     predate the record that describes it, so this is exactly pid recycling
//     and nothing else. The converse is not evidence: a loop's start precedes
//     its stamp by however long its preamble took, which on a loaded box has
//     no bound worth asserting.
//
// image is the dev:ino /proc names for that pid, "" where /proc could not say.
func watchPidStranger(rec *WatchPid, image string, started func(int) (time.Time, bool)) (why string, checked bool) {
	if rec.Image != "" && image != "" {
		if rec.Image != image {
			return fmt.Sprintf("this home's dispatch-watch.pid stamped image %s for pid %d and /proc says that pid runs %s — the loop that wrote the record is gone and its pid is somebody else's now", rec.Image, rec.Pid, image), true
		}
		checked = true
	}
	if !rec.Started.IsZero() {
		if t, ok := started(rec.Pid); ok {
			// TRUNCATED, and that is not a slack: WriteWatchPid writes
			// `started:` as RFC3339, which carries no sub-second part, so
			// the stamp a genuine loop wrote reads up to a second BEFORE its
			// own start time. Comparing untruncated makes this a coin flip
			// on whether the process's start and its stamp straddled a
			// second boundary, which would red an honest loop at a rate
			// nothing in a solo run would show — the shape this bead's
			// fixture half already cost a suite for.
			if t.Truncate(time.Second).After(rec.Started) {
				return fmt.Sprintf("pid %d started at %s, after this home's dispatch-watch.pid was written at %s — a process cannot predate the record that describes it, so the recorded loop is gone and this pid has been recycled", rec.Pid, t.UTC().Format(time.RFC3339), rec.Started.UTC().Format(time.RFC3339)), true
			}
			checked = true
		}
	}
	return "", checked
}

func readWatchImage(rec *WatchPid, procExe func(int) (string, string, bool), procStarted func(int) (time.Time, bool)) WatchImage {
	w := WatchImage{Pid: rec.Pid, Started: rec.Started, Version: rec.Version}
	abstain := func(why string) WatchImage {
		return WatchImage{Pid: rec.Pid, Started: rec.Started, Why: why}
	}
	if id, path, ok := procExe(rec.Pid); ok {
		// THE PID IS THE WHOLE SOURCE on this road — the image, the path and
		// therefore the verdict all come from /proc/<that pid> — so it is
		// read only once the record can be matched to the process holding
		// it. A reading that cannot be attributed is no reading
		// (ranger-base-bekle): what G18 would otherwise print is a
		// stranger's image rendered as "dispatcher pid N runs an image no
		// longer at that path", internally consistent and about the wrong
		// process. It used to drop the record's VERSION on a stamped-image
		// mismatch and keep that stranger's path — half of this check, drawn
		// from the same fact, acted on only where it changed a word.
		why, checked := watchPidStranger(rec, id, procStarted)
		if why != "" {
			return abstain(why)
		}
		if !checked {
			return abstain(fmt.Sprintf("this home's dispatch-watch.pid names pid %d and carries neither a start time nor an image stamp, so nothing in it can be matched against the process holding that pid now", rec.Pid))
		}
		w.Image, w.Path, w.Via = id, path, "proc"
	} else if rec.Image != "" && rec.Exe != "" {
		// Here the RECORD is its own witness — it says which image it ran
		// and where from — and the pid is a label on it, which is what makes
		// this road work at all where /proc cannot be read (darwin, another
		// pid namespace). So an unanswerable start time is not held against
		// it. But where /proc can still TIME the pid and a cage only refused
		// the exe link (ranger-base-rph5z's reader), a process that started
		// after the record was written is a recycled pid here too.
		if why, _ := watchPidStranger(rec, "", procStarted); why != "" {
			return abstain(why)
		}
		w.Image, w.Path, w.Via = rec.Image, rec.Exe, "record"
	} else {
		return w
	}
	w.OnDisk = fileIdentity(w.Path)
	if w.OnDisk != "" {
		if self, _ := selfImage(); self == w.OnDisk {
			w.Now = VersionString()
		}
	}
	return w
}
