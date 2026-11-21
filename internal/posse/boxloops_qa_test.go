//go:build !posse_arm2 && !posse_arm3

package posse

// Pins for the box-wide dispatch-loop census (boxloops.go, ADR 0092 D8,
// ranger-base-is1qp).
//
// WHAT THESE HOLD, and why each is here rather than being obvious: the census
// is the widening ranger-base-nqaa1 deliberately did not do, and the hole it
// closes was invisible for one reason — a per-instance reading that was
// SILENT about a sibling looked exactly like a reading that had found nothing
// wrong. So the pins are mostly about what the census must not be allowed to
// say: it must not confirm a process as a loop on the strength of running the
// right image (the measured false positive is a live one), it must not report
// itself, and it must not let an abstention render as an all-clear.

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// boxFixture builds a probe over a table of (pid, image, home) rows plus a
// map of each home's recorded loop pid — the shape of the MEASURED reading,
// with no /proc anywhere.
type boxFixtureProc struct {
	pid       int
	ino       string // the inode of the image it runs; the fixture's dev is always 54
	exe       string // the path its exe link names; defaults to the subject path
	home      string
	noEnviron bool
	// refused is a pid whose /proc/<pid>/exe the kernel would not let this
	// reader read, while /proc/<pid> itself is still ours — what a seat in a
	// child user namespace gets for every same-uid peer (ranger-base-rph5z).
	refused bool
	// alien is a pid of another uid: ExeOf refuses it AND it is not ours, so
	// it is excluded rather than counted as unreadable.
	alien bool
	// comm is the kernel's program name, which stays readable when exe is
	// refused. Empty means "the subject's own basename", so a `refused` row
	// is a plausible loop by default; noComm means comm is unreadable too.
	comm   string
	noComm bool
}

// boxFixturePath is the subject of every fixture reading: a path no box has,
// so a fixture can never accidentally read the installed binary.
const boxFixturePath = "/box/bin/posse"

func boxFixture(onDiskIno string, procs []boxFixtureProc, records map[string]int, self int) (string, boxProbe) {
	path := boxFixturePath
	ids := func(ino string) string { return "54:" + ino }
	probe := boxProbe{
		Self: self,
		Pids: func() ([]int, error) {
			out := make([]int, 0, len(procs))
			for _, p := range procs {
				out = append(out, p.pid)
			}
			return out, nil
		},
		ExeOf: func(pid int) (string, string, bool) {
			for _, p := range procs {
				if p.pid != pid {
					continue
				}
				if p.refused || p.alien {
					return "", "", false
				}
				exe := p.exe
				if exe == "" {
					exe = path
				}
				return ids(p.ino), exe, true
			}
			return "", "", false
		},
		Owned: func(pid int) bool {
			for _, p := range procs {
				if p.pid == pid {
					return !p.alien
				}
			}
			return false
		},
		CommOf: func(pid int) (string, bool) {
			for _, p := range procs {
				if p.pid != pid {
					continue
				}
				if p.noComm {
					return "", false
				}
				if p.comm != "" {
					return p.comm, true
				}
				return filepath.Base(path), true
			}
			return "", false
		},
		HomeOf: func(pid int) (string, bool) {
			for _, p := range procs {
				if p.pid == pid {
					if p.noEnviron || p.home == "" {
						return "", false
					}
					return p.home, true
				}
			}
			return "", false
		},
		RecPid: func(home string) (int, bool) {
			pid, ok := records[home]
			return pid, ok
		},
		Ident: func(p string) string {
			if p == path {
				return ids(onDiskIno)
			}
			return ""
		},
	}
	return path, probe
}

// theMeasuredBox is the reading MEASURED 2026-11-21 on linux
// 7.2.15-1-omarchy (boxloops.go's header), with one change that is the point
// of the fixture: the weller loop is left on the pre-install inode, which is
// the state the census exists to report and which was NOT live when the box
// was read (both loops had already restarted). So the fixture holds the
// arrangement, and the live arm below holds that the arrangement is readable.
func theMeasuredBox(self int) (string, boxProbe) {
	return boxFixture("14541392", []boxFixtureProc{
		{pid: 1185112, ino: "12591441", home: "/homes/weller"}, // the loop a rename left behind
		{pid: 1185185, ino: "14541392", home: "/homes/ranger"}, // the loop that restarted
		{pid: 3285561, ino: "14541392", home: "/homes/ranger"}, // `posse pulse wait`, a seat's child
	}, map[string]int{
		"/homes/weller": 1185112,
		"/homes/ranger": 1185185,
	}, self)
}

// TestQABoxCensusConfirmsALoopByItsOwnInstancesRecordAndNotByItsImage is the
// filter, and the measured false positive is what makes it load-bearing.
//
// Pid 3285561 was a live `posse pulse wait --after 494` child of a ranger
// seat: the same image, from the same path, under the same RHQ_HOME as the
// ranger loop. Keyed on the exe link alone — which is the only key the
// census has for finding candidates at all — it is indistinguishable from a
// loop. What separates them is the per-home record, read in the one direction
// that needs no homes root: the home names itself from the process's environ,
// and that home's dispatch-watch.pid either names the process back or does
// not.
func TestQABoxCensusConfirmsALoopByItsOwnInstancesRecordAndNotByItsImage(t *testing.T) {
	t.Parallel()
	path, probe := theMeasuredBox(9999)
	r := readBoxLoops(path, probe)
	if r.Err != nil {
		t.Fatalf("the fixture's probe answers everything, so there is nothing to abstain over: %v", r.Err)
	}
	if got := len(r.Procs); got != 3 {
		t.Fatalf("3 processes run that image, census found %d — a candidate was dropped, which is the failure this census is a correction of", got)
	}
	loops := r.Loops()
	if len(loops) != 2 {
		t.Fatalf("2 of the 3 are loops, census confirmed %d: %+v", len(loops), loops)
	}
	for _, want := range []int{1185112, 1185185} {
		found := false
		for _, l := range loops {
			if l.Pid == want {
				found = true
			}
		}
		if !found {
			t.Errorf("pid %d is its instance's recorded loop and the census did not confirm it", want)
		}
	}
	other := r.Unattributed()
	if len(other) != 1 || other[0].Pid != 3285561 {
		t.Fatalf("the `pulse wait` child (pid 3285561) is the one non-loop and must be NAMED, not dropped; got %+v", other)
	}
	if !strings.Contains(other[0].Why, "1185185") {
		t.Errorf("the reason a candidate is not a loop has to name the pid its instance's record DOES name, or a reader cannot tell a stranger from a stale record; got %q", other[0].Why)
	}
}

// TestQABoxCensusReadsStalenessAgainstThePathAndNamesOnlyTheLoopsLeftBehind.
// The subject of ADR 0092 D6's restart is a loop on the old inode, so the
// census has to separate the two loops by image and not merely list them.
func TestQABoxCensusReadsStalenessAgainstThePathAndNamesOnlyTheLoopsLeftBehind(t *testing.T) {
	t.Parallel()
	path, probe := theMeasuredBox(9999)
	r := readBoxLoops(path, probe)
	stale := r.StaleLoops()
	if len(stale) != 1 || stale[0].Pid != 1185112 {
		t.Fatalf("the weller loop (1185112) is the one left on the replaced inode; census says %+v", stale)
	}
	if stale[0].Home != "/homes/weller" {
		t.Errorf("a stale loop has to carry WHOSE it is — the whole point is that it is a sibling's; got %q", stale[0].Home)
	}
	s := r.Sentence()
	for _, want := range []string{"weller", "1185112", "STALE", "guardrail 3"} {
		if !strings.Contains(s, want) {
			t.Errorf("the rendering of a stale sibling must name %q; got:\n%s", want, s)
		}
	}
	// And it must not tell anyone to restart it: this is a reader.
	for _, forbidden := range []string{"restarting it for you", "restarted it"} {
		if strings.Contains(s, forbidden) {
			t.Errorf("the census is a reader — %q claims an action (ADR 0092 D6, guardrail 3)", forbidden)
		}
	}
}

// TestQABoxCensusCallsALoopStaleWhenThePathHoldsNothing, which is
// WatchImage.Stale's predicate and not a looser one. An empty on-disk
// identity means nothing is at that path — a swap in flight, or `install(1)`
// between its unlink and its write (ADR 0092 Context) — and a loop whose
// image is not the one at its path is in the condition either way. The pin is
// here because the comfortable-looking predicate (treat an absent file as
// "nothing to compare, so not stale") renders as "all already on the
// installed image" over a path with no installed image on it.
func TestQABoxCensusCallsALoopStaleWhenThePathHoldsNothing(t *testing.T) {
	t.Parallel()
	path, probe := theMeasuredBox(9999)
	probe.Ident = func(string) string { return "" } // nothing at the path
	r := readBoxLoops(path, probe)
	if r.OnDisk != "" {
		t.Fatalf("the fixture says the path holds nothing; census read %q", r.OnDisk)
	}
	if got := len(r.StaleLoops()); got != 2 {
		t.Fatalf("both loops run an image that is not at that path, so both are stale; census says %d", got)
	}
	s := r.Sentence()
	if strings.Contains(s, "all already on the installed image") {
		t.Fatal("a path holding nothing must never render as an all-clear about the installed image")
	}
	if !strings.Contains(s, "holds NOTHING") {
		t.Errorf("the rendering must say the path is empty, or a reader restarts two loops over a missing install; got:\n%s", s)
	}
	// And it must agree with G18's predicate over the same facts.
	w := WatchImage{Pid: 1185112, Image: "54:12591441", OnDisk: ""}
	if !w.Stale() {
		t.Fatal("WatchImage.Stale changed — this census was aligned to it and the two now disagree")
	}
}

// TestQABoxCensusSkipsTheReadingProcessItself. `posse upgrade` runs FROM the
// path it replaces, so the verb's own process is a candidate by the census's
// own key — and after its swap its own image is the OLD inode. Unskipped, the
// census would report the reader as a stale process on this binary on every
// single run: true, and useless, because that process is about to exit and
// renders no launch.
func TestQABoxCensusSkipsTheReadingProcessItself(t *testing.T) {
	t.Parallel()
	// The reader is pid 3285561's slot here: same image, same path, and in
	// the measured table it is not a loop — so if it were NOT skipped it
	// would show up among the unattributed and be visible.
	path, probe := theMeasuredBox(3285561)
	r := readBoxLoops(path, probe)
	for _, p := range r.Procs {
		if p.Pid == 3285561 {
			t.Fatalf("the reading process must not appear in its own census: %+v", p)
		}
	}
	if len(r.Procs) != 2 {
		t.Fatalf("skipping self should leave the two loops, got %d: %+v", len(r.Procs), r.Procs)
	}
	// Skipping self must not become skipping anything that looks like it: a
	// child of the verb running the same image IS reported.
	path, probe = theMeasuredBox(9999)
	if got := len(readBoxLoops(path, probe).Unattributed()); got != 1 {
		t.Errorf("a non-loop process that is not the reader stays in the census; got %d unattributed", got)
	}
}

// TestQABoxCensusCountsOnlyProcessesOnTheSubjectPath. The key is the path the
// install replaces, so another posse on this box — a build in a worktree, a
// copy under test — is not a subject of this rename and is not counted.
func TestQABoxCensusCountsOnlyProcessesOnTheSubjectPath(t *testing.T) {
	t.Parallel()
	path, probe := boxFixture("14541392", []boxFixtureProc{
		{pid: 10, ino: "12591441", home: "/homes/ranger"},
		{pid: 11, ino: "99999999", exe: "/tmp/worktree/posse", home: "/homes/ranger"},
	}, map[string]int{"/homes/ranger": 10}, 9999)
	r := readBoxLoops(path, probe)
	if len(r.Procs) != 1 || r.Procs[0].Pid != 10 {
		t.Fatalf("only the process whose exe link names the subject path is a candidate; got %+v", r.Procs)
	}
}

// TestQABoxCensusNamesACandidateItCannotAttributeInsteadOfDroppingIt. A
// candidate whose instance cannot be named, or whose instance keeps no
// readable record, is reported. Dropping it would be the exact shape of the
// failure this census corrects — a reading that is silent about a process it
// could not account for reads as a reading that found nothing.
func TestQABoxCensusNamesACandidateItCannotAttributeInsteadOfDroppingIt(t *testing.T) {
	t.Parallel()
	path, probe := boxFixture("14541392", []boxFixtureProc{
		{pid: 10, ino: "12591441", noEnviron: true},
		{pid: 11, ino: "12591441", home: "/homes/nowhere"},
	}, map[string]int{}, 9999)
	r := readBoxLoops(path, probe)
	if len(r.Procs) != 2 {
		t.Fatalf("both candidates must be reported; got %+v", r.Procs)
	}
	for _, p := range r.Procs {
		if p.Loop {
			t.Errorf("pid %d was confirmed a loop with nothing saying so", p.Pid)
		}
		if p.Why == "" {
			t.Errorf("pid %d is unattributed and carries no reason why", p.Pid)
		}
	}
	s := r.Sentence()
	// "2 other processes", not "2 other processs": this is the branch of the
	// rendering a reader meets least often and therefore proof-reads least,
	// which is where a helper that appends an "s" went wrong once already.
	if !strings.Contains(s, "2 other processes on this binary are not dispatch loops") {
		t.Errorf("the unattributed clause does not read as English in the plural; got:\n%s", s)
	}
	for _, want := range []string{"pid 10", "pid 11", "RHQ_HOME", "dispatch-watch.pid"} {
		if !strings.Contains(s, want) {
			t.Errorf("the rendering must name %q so a reader can chase it; got:\n%s", want, s)
		}
	}
}

// TestQABoxCensusAbstentionIsNeverRenderedAsAnAllClear is the pin that holds
// the lesson of the bead this one came from. A census that cannot run — no
// /proc, off linux — has an empty loop list and an empty stale list, which
// are byte-identical to a clean box. So the rendering of an abstention must
// say that the per-instance reading is the whole of the report's coverage,
// and must carry the residual, exactly as watchScopeSentence does for the
// narrower reading.
func TestQABoxCensusAbstentionIsNeverRenderedAsAnAllClear(t *testing.T) {
	t.Parallel()
	path, probe := theMeasuredBox(9999)
	probe.Pids = func() ([]int, error) { return nil, Die("no /proc here") }
	r := readBoxLoops(path, probe)
	if r.Err == nil {
		t.Fatal("a probe that cannot list pids is an abstention, not an empty box")
	}
	if len(r.Loops()) != 0 || len(r.StaleLoops()) != 0 {
		t.Fatal("an abstention has no readings, which is why its RENDERING has to do the work")
	}
	s := r.Sentence()
	for _, want := range []string{"not read", "per-instance", "ADR 0092 D6", "ranger-base-is1qp"} {
		if !strings.Contains(s, want) {
			t.Errorf("an abstention's line must carry %q; got:\n%s", want, s)
		}
	}
	for _, forbidden := range []string{"all already on the installed image", "no dispatch loop on this box"} {
		if strings.Contains(s, forbidden) {
			t.Errorf("an abstention rendered as %q is the silence this census exists to end", forbidden)
		}
	}
}

// TestQABoxCensusEveryRenderingStatesItsOwnReach is ranger-base-nqaa1's rule
// applied to the wider reading: every branch says what it looked at. Each of
// the two lines in the report states its own reach and neither claims the
// other's — the per-instance one says the subject is box-wide and it read one
// instance, this one says which processes it enumerated and how it keyed them.
func TestQABoxCensusEveryRenderingStatesItsOwnReach(t *testing.T) {
	t.Parallel()
	path, probe := theMeasuredBox(9999)
	empty := boxProbe{Self: 1, Pids: func() ([]int, error) { return []int{}, nil },
		ExeOf:  func(int) (string, string, bool) { return "", "", false },
		HomeOf: func(int) (string, bool) { return "", false },
		RecPid: func(string) (int, bool) { return 0, false },
		Ident:  func(string) string { return "54:1" }}
	fresh := theMeasuredBoxAllFresh()

	for name, r := range map[string]BoxReading{
		"stale sibling": readBoxLoops(path, probe),
		"all fresh":     readBoxLoops(path, fresh),
		"no loop":       readBoxLoops(path, empty),
	} {
		s := r.Sentence()
		if !strings.Contains(s, "/proc/<pid>/exe") {
			t.Errorf("%s: the rendering does not say what it keyed on:\n%s", name, s)
		}
		if !strings.Contains(s, "never an argv match") {
			t.Errorf("%s: the rendering does not say what it did NOT key on, which is the rule it is holding (AGENTS.md, \"Ending anything\"):\n%s", name, s)
		}
		if !strings.Contains(s, "ranger-base-is1qp") {
			t.Errorf("%s: the rendering carries no bead id to chase:\n%s", name, s)
		}
	}
}

func theMeasuredBoxAllFresh() boxProbe {
	_, probe := boxFixture("14541392", []boxFixtureProc{
		{pid: 1185112, ino: "14541392", home: "/homes/weller"},
		{pid: 1185185, ino: "14541392", home: "/homes/ranger"},
	}, map[string]int{"/homes/weller": 1185112, "/homes/ranger": 1185185}, 9999)
	return probe
}

// TestQABoxCensusReadsAnImageThroughTheSameSpellingG18Does. The per-instance
// reading and the box-wide one answer different questions over the same fact
// — which image a pid runs — and they must not grow two readings of it. One
// `procExe`, which is also the one a pin can take the darwin road through
// (watchimage.go).
func TestQABoxCensusReadsAnImageThroughTheSameSpellingG18Does(t *testing.T) {
	t.Parallel()
	live := reflect.ValueOf(liveBoxProbe().ExeOf).Pointer()
	g18 := reflect.ValueOf(procExe).Pointer()
	if live != g18 {
		t.Error("the box census reads /proc/<pid>/exe through its own function — G18 and this census would then be able to disagree about what one pid runs")
	}
}

// --- ranger-base-rph5z: a refused read is not an absence ---

// TestQABoxCensusNeverRendersAnAllClearWhenItWasRefusedAReadOfOurOwnProcesses
// is the pin for the defect the security review found in this census as it was
// first written, and
// it is the file's own rule turned on the file: `procExe` answers the same
// "not ok" for a pid that exited and for a link the kernel REFUSED, and the
// first version dropped both silently. MEASURED by the security persona
// 2026-11-21 from a
// seat in a child user namespace: /proc listed 460 pids and every same-uid
// peer's exe and environ were EACCES. From such a reader the census found no
// candidates and rendered "no dispatch loop on this box runs …" — a false
// all-clear about loops that may still be rendering pre-install walls, which
// is the one sentence this file says must never be printable.
func TestQABoxCensusNeverRendersAnAllClearWhenItWasRefusedAReadOfOurOwnProcesses(t *testing.T) {
	t.Parallel()
	// The caged reader: every same-uid peer refused, nothing readable.
	path, probe := boxFixture("14541392", []boxFixtureProc{
		{pid: 1185112, ino: "12591441", home: "/homes/weller", refused: true},
		{pid: 1185185, ino: "14541392", home: "/homes/ranger", refused: true},
	}, map[string]int{"/homes/weller": 1185112, "/homes/ranger": 1185185}, 9999)
	r := readBoxLoops(path, probe)
	if r.Unreadable != 2 {
		t.Fatalf("both peers were refused and both are ours, so both are unreadable candidates; census counted %d", r.Unreadable)
	}
	if r.Err == nil {
		t.Fatal("not one readable process of ours is an ABSTENTION, not an empty box")
	}
	s := r.Sentence()
	if strings.Contains(s, "no dispatch loop on this box") {
		t.Fatalf("THE false all-clear, rendered over two loops the reader was not allowed to see:\n%s", s)
	}
	if !strings.Contains(s, "dispatch (box): not read") {
		t.Errorf("a reading with nothing readable must render as an abstention; got:\n%s", s)
	}

	// The partial reader: one peer readable and fresh, one refused. This is
	// the sharper case — there IS a reading, and it must not pass itself off
	// as a reading of the box.
	path, probe = boxFixture("14541392", []boxFixtureProc{
		{pid: 1185112, ino: "12591441", home: "/homes/weller", refused: true},
		{pid: 1185185, ino: "14541392", home: "/homes/ranger"},
	}, map[string]int{"/homes/weller": 1185112, "/homes/ranger": 1185185}, 9999)
	r = readBoxLoops(path, probe)
	if r.Err != nil {
		t.Fatalf("one readable candidate is a reading: %v", r.Err)
	}
	if !r.Partial() || r.Unreadable != 1 {
		t.Fatalf("one refused peer of ours makes this partial; Partial()=%v Unreadable=%d", r.Partial(), r.Unreadable)
	}
	if len(r.StaleLoops()) != 0 {
		t.Fatal("the readable loop is on the installed image — the point is that this is not the whole box")
	}
	s = r.Sentence()
	if !boxSentenceAdmitsItDidNotCoverTheBox(s) {
		t.Errorf("a partial reading that found nothing stale must say it did not cover the box; got:\n%s", s)
	}
	if !strings.Contains(s, "not a clear box") {
		t.Errorf("and it must say so in words an operator reads as a caveat; got:\n%s", s)
	}
}

// TestQABoxCensusCountsOnlyOurOwnRefusedProcessesAndNotStrangers. Root's
// processes and non-dumpable ones are refused to everyone and are not
// subjects of this rename, so counting them would turn every reading on every
// box permanently partial — which is a caveat nobody can act on and therefore
// a caveat nobody reads.
func TestQABoxCensusCountsOnlyOurOwnRefusedProcessesAndNotStrangers(t *testing.T) {
	t.Parallel()
	path, probe := boxFixture("14541392", []boxFixtureProc{
		{pid: 1185185, ino: "14541392", home: "/homes/ranger"},
		{pid: 2, alien: true},
		{pid: 3, alien: true},
	}, map[string]int{"/homes/ranger": 1185185}, 9999)
	r := readBoxLoops(path, probe)
	if r.Unreadable != 0 {
		t.Errorf("another account's refused processes are excluded, not unreadable candidates; counted %d", r.Unreadable)
	}
	if r.Partial() {
		t.Error("a box whose only refusals are strangers' is covered")
	}
	if s := r.Sentence(); !strings.Contains(s, "all already on the installed image") {
		t.Errorf("and such a box gets the plain all-clear; got:\n%s", s)
	}
}

// TestQABoxCensusStillNamesStaleLoopsWhenTheReadingIsPartial — a partial
// reading must not swallow what it DID find. The caveat is additive.
func TestQABoxCensusStillNamesStaleLoopsWhenTheReadingIsPartial(t *testing.T) {
	t.Parallel()
	path, probe := boxFixture("14541392", []boxFixtureProc{
		{pid: 1185112, ino: "12591441", home: "/homes/weller"},
		{pid: 1185185, ino: "14541392", home: "/homes/ranger", refused: true},
	}, map[string]int{"/homes/weller": 1185112, "/homes/ranger": 1185185}, 9999)
	r := readBoxLoops(path, probe)
	if len(r.StaleLoops()) != 1 {
		t.Fatalf("the readable weller loop is stale and must still be reported; got %+v", r.StaleLoops())
	}
	s := r.Sentence()
	for _, want := range []string{"weller", "STALE", "could not read"} {
		if !strings.Contains(s, want) {
			t.Errorf("a partial reading with a stale find must carry %q; got:\n%s", want, s)
		}
	}
}

// TestQABoxCensusDoesNotCountRefusalsThatCouldNotBeALoopOnThisBinary is the
// correction of this census's own first fix, and it is here because the first
// fix made the caveat useless rather than wrong.
//
// MEASURED 2026-11-21 from a `cage shims` seat on this box: four same-uid
// processes are permanently unreadable — the user `systemd` (pid 1140),
// `(sd-pam)`, `sshd-session` and `gpg-agent`, each EACCES on its exe link
// while /proc/<pid> is owned by uid 1001, because each is non-dumpable. Count
// those and every reading on this box is partial forever, and a caveat that
// is always on is one a reader stops finishing — which would hide the false
// all-clear the caveat exists to prevent behind a sentence nobody reads.
//
// comm is what separates them, and it is used ONLY to decide whether a
// refusal is worth reporting. It never confirms a loop.
func TestQABoxCensusDoesNotCountRefusalsThatCouldNotBeALoopOnThisBinary(t *testing.T) {
	t.Parallel()
	path, probe := boxFixture("14541392", []boxFixtureProc{
		{pid: 1185185, ino: "14541392", home: "/homes/ranger"},
		// The four on this box, by name, refused and irrelevant.
		{pid: 1140, refused: true, comm: "systemd"},
		{pid: 1142, refused: true, comm: "(sd-pam)"},
		{pid: 2682725, refused: true, comm: "sshd-session"},
		{pid: 2931892, refused: true, comm: "gpg-agent"},
	}, map[string]int{"/homes/ranger": 1185185}, 9999)
	r := readBoxLoops(path, probe)
	if r.Unreadable != 0 {
		t.Errorf("a refused process whose comm names another program cannot be a loop on this binary; counted %d", r.Unreadable)
	}
	if r.Partial() {
		t.Error("so this reading covers the box, and must render as the plain all-clear")
	}

	// A refused process that COULD be one is still counted — the narrowing
	// must not swallow the case the field exists for.
	path, probe = boxFixture("14541392", []boxFixtureProc{
		{pid: 1185185, ino: "14541392", home: "/homes/ranger"},
		{pid: 1140, refused: true, comm: "systemd"},
		{pid: 1185112, refused: true}, // comm defaults to the subject's basename
	}, map[string]int{"/homes/ranger": 1185185}, 9999)
	r = readBoxLoops(path, probe)
	if r.Unreadable != 1 {
		t.Errorf("a refused process running this binary's name is exactly what the caveat is for; counted %d", r.Unreadable)
	}
	if !r.Partial() || !boxSentenceAdmitsItDidNotCoverTheBox(r.Sentence()) {
		t.Errorf("and the rendering must say so; got:\n%s", r.Sentence())
	}

	// An unreadable comm is COUNTED: knowing nothing about a process of ours
	// that we were refused is the whole content of the caveat, and the safe
	// direction is the caveat rather than the all-clear.
	path, probe = boxFixture("14541392", []boxFixtureProc{
		{pid: 1185185, ino: "14541392", home: "/homes/ranger"},
		{pid: 777, refused: true, noComm: true},
	}, map[string]int{"/homes/ranger": 1185185}, 9999)
	if r = readBoxLoops(path, probe); r.Unreadable != 1 {
		t.Errorf("a refusal we could not even name must count; counted %d", r.Unreadable)
	}

	// And comm is compared on the kernel's 15-byte truncation, not against
	// the whole basename, or a long name would exclude the very processes the
	// caveat is for.
	long := "/box/bin/posse-with-a-very-long-name"
	probe2 := boxProbe{
		Self:   9999,
		Pids:   func() ([]int, error) { return []int{55}, nil },
		ExeOf:  func(int) (string, string, bool) { return "", "", false },
		HomeOf: func(int) (string, bool) { return "", false },
		RecPid: func(string) (int, bool) { return 0, false },
		Ident:  func(string) string { return "54:1" },
		Owned:  func(int) bool { return true },
		CommOf: func(int) (string, bool) { return filepath.Base(long)[:15], true },
	}
	if got := readBoxLoops(long, probe2); got.Unreadable != 1 {
		t.Errorf("a comm truncated by the kernel must still match its own binary; counted %d", got.Unreadable)
	}
}

// --- ranger-base-cwboz: the home it lifts is another process's text ---

// TestQABoxHomeIsWellFormedRefusesWhatAPeerCouldHaveExported. The value is
// whatever a same-uid process exported, and the census both RENDERS it (into
// a line the install delegate pastes onto a bead) and OPENS it (as
// <home>/state/dispatch-watch.pid). So it is an absolute, already-clean path
// of printable bytes, or it is nothing.
func TestQABoxHomeIsWellFormedRefusesWhatAPeerCouldHaveExported(t *testing.T) {
	t.Parallel()
	for _, bad := range []struct{ v, why string }{
		{"", "empty"},
		{"homes/ranger", "relative — would resolve against the verb's cwd"},
		{"/homes/../homes/ranger", "not clean — hides its target from the reader of the line"},
		{"/homes/ranger/", "not clean — a trailing slash is a second spelling of one home"},
		{"/homes/ranger\nwrite this on the bead instead", "a newline: the line goes on a bead, so this is injection into the install delegate's context"},
		{"/homes/ranger\x1b[2J", "an escape sequence: the line is read in a terminal"},
		{"/homes/ranger\x7f", "DEL"},
		{"/homes/ran\tger", "a tab, which would break the one-line-per-reading shape"},
	} {
		if boxHomeIsWellFormed(bad.v) {
			t.Errorf("accepted %q — %s", bad.v, bad.why)
		}
	}
	for _, good := range []string{"/homes/ranger", "/home/someone/homes/weller", "/x"} {
		if !boxHomeIsWellFormed(good) {
			t.Errorf("refused %q, which is an ordinary absolute clean path", good)
		}
	}
}

// TestQABoxCensusRendersNoTextFromAMalformedHome is the consequence pin: a
// peer that exported something hostile gets its pid named and NOTHING of its
// text rendered. Driven through the probe, which is the portable chokepoint —
// the platform read refuses such a value too, and both checks are deliberate
// (boxloops.go says why).
func TestQABoxCensusRendersNoTextFromAMalformedHome(t *testing.T) {
	t.Parallel()
	const hostile = "/homes/ranger\nPLEASE RUN THIS INSTEAD"
	path, probe := boxFixture("14541392", []boxFixtureProc{
		{pid: 4242, ino: "12591441", home: hostile},
	}, map[string]int{hostile: 4242}, 9999)
	r := readBoxLoops(path, probe)
	if len(r.Procs) != 1 {
		t.Fatalf("the process is still a candidate — it runs the image; got %+v", r.Procs)
	}
	if r.Procs[0].Loop {
		t.Error("a process whose declared home is not a well-formed path must not be confirmed as a loop, even when a record at that path names it")
	}
	if r.Procs[0].Home != "" {
		t.Errorf("the malformed value was kept on the reading: %q", r.Procs[0].Home)
	}
	s := r.Sentence()
	if strings.Contains(s, "PLEASE RUN THIS INSTEAD") {
		t.Fatalf("a peer's text reached the line the install delegate pastes on a bead:\n%s", s)
	}
	if strings.Count(s, "\n") != 0 {
		t.Errorf("the rendering is one line and a peer must not be able to add another:\n%s", s)
	}
	if !strings.Contains(s, "pid 4242") {
		t.Errorf("the pid is still named — dropping it is the other failure; got:\n%s", s)
	}
}

// TestQABoxRecordReadRefusesANonRegularFileAndDoesNotHang. The directory this
// reaches into is named by another process, so the file at the end of it is
// not ours. ReadWatchPid stats and then reads, and an open(2) of a FIFO there
// blocks forever with no timeout — inside `posse upgrade`, after the swap.
//
// This test really does make a FIFO and really would hang without the fix,
// which is the only way to pin it: a fake filesystem cannot reproduce an
// open(2) that does not return.
func TestQABoxRecordReadRefusesANonRegularFileAndDoesNotHang(t *testing.T) {
	t.Parallel()
	// NOT named `home`: internal/treepins' temp-root census resolves an
	// identifier across the whole package, and a seventh binding of `home`
	// here made app_test.go:209's `os.MkdirTemp(home, …)` unreadable to it —
	// a red in a file this bead never touched. The census is right to work
	// that way (a prefix nobody can read is a prefix nobody can check); the
	// cheap half of keeping it readable is not reusing its names.
	stateHome := t.TempDir()
	state := filepath.Join(stateHome, "state")
	if err := os.MkdirAll(state, 0o755); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(state, "dispatch-watch.pid")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Skipf("cannot create a FIFO here (%v) — the fence is still Lstat+IsRegular", err)
	}
	done := make(chan bool, 1)
	go func() {
		_, ok := boxRecordedLoopPid(stateHome)
		done <- ok
	}()
	select {
	case ok := <-done:
		if ok {
			t.Error("a FIFO was read as an instance's loop record")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("boxRecordedLoopPid blocked on a FIFO — this is the hang that would stop `posse upgrade` after its swap (ranger-base-cwboz)")
	}

	// And a real record is still read, so the fence is not a refusal of
	// everything.
	good := filepath.Join(t.TempDir())
	gs := filepath.Join(good, "state")
	if err := os.MkdirAll(gs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteWatchPid(filepath.Join(gs, "dispatch-watch.pid"), WatchPid{Pid: 4242}); err != nil {
		t.Fatal(err)
	}
	if pid, ok := boxRecordedLoopPid(good); !ok || pid != 4242 {
		t.Errorf("an ordinary record must still read: pid=%d ok=%v", pid, ok)
	}
}

// TestQATheVerbsBoxLineIsAbsentWithoutAReaderAndCarriesItWithOne pins the
// verb's side of D8 the way watchimage_qa_test.go pins the sibling line: at
// `upgradeBoxLine`, over a plan.
//
// WHAT THIS DOES NOT REACH, AND WHERE THAT NOW IS. This pin stops at
// `upgradeBoxLine`, so a deletion of the `upgradeBoxLine(plan)` call from the
// report — or of the assignment that gives the plan a reader at all — would
// leave every pin in THIS file green. That gap was the per-instance line's
// too (nothing asserted "dispatch:" appeared in a d4Fixture report either),
// so it was one gap over two lines; it is closed in
// upgradereportloops_qa_test.go (ranger-base-r1zdr), where a d4Fixture report
// carries a Loop and a Box and the wiring goes in through
// `upgradeLoopReaders`. Four mutants that were green over 98 tests before
// those pins existed are in that file's header.
func TestQATheVerbsBoxLineIsAbsentWithoutAReaderAndCarriesItWithOne(t *testing.T) {
	t.Parallel()
	if got := upgradeBoxLine(upgradePlan{}); got != "" {
		t.Errorf("a plan with no box reader must print nothing, not an empty-looking reading: %q", got)
	}
	path, probe := theMeasuredBox(9999)
	got := upgradeBoxLine(upgradePlan{Box: func() BoxReading { return readBoxLoops(path, probe) }})
	for _, want := range []string{"dispatch (box):", "weller", "STALE", "ranger-base-is1qp"} {
		if !strings.Contains(got, want) {
			t.Errorf("the verb's box line does not carry %q; got:\n%s", want, got)
		}
	}
	// An abstention still prints: a report that silently drops this line is
	// a report whose per-instance reading looks like the whole box.
	abstain := upgradeBoxLine(upgradePlan{Box: func() BoxReading {
		return BoxReading{Path: path, Err: Die("no /proc here")}
	}})
	if !strings.Contains(abstain, "dispatch (box): not read") {
		t.Errorf("an abstention must still print its line; got %q", abstain)
	}
}

// TestLiveBoxCensusReadsThisBox is the arm that holds the fixtures honest: on
// a box with a real installed posse, the census must RUN — not abstain — and
// everything it confirms must be confirmable a second way.
//
// It asserts no particular number of loops: this box had two when the
// mechanism was written and a CI runner has none. What it asserts is that a
// reading was taken, that the reader is not in it, and that every confirmed
// loop's instance really does name that pid. It logs the table, so a reader
// of a suite run gets the measurement rather than having to take this file's
// word for the one in boxloops.go's header.
func TestLiveBoxCensusReadsThisBox(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "linux" {
		t.Skipf("no /proc on %s — boxloops_other.go's refusal is the behaviour there, pinned by its own build", runtime.GOOS)
	}
	// operatorHome, NOT $HOME and not os.UserHomeDir: this package's
	// TestMain replaces HOME with a per-tree temp home (herdr_test.go), so
	// the obvious spelling skips on the one box the arm exists for — which
	// it did, silently, on the first run of this file. The same reason
	// ciwatch_live_test.go reaches for it.
	if operatorHome == "" {
		t.Skip("the home this binary started with is not recorded, so no installed posse can be found")
	}
	path := filepath.Join(operatorHome, ".local", "bin", "posse")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("no installed posse at %s — nothing for a live census to read", AbbrevHome(path))
	}
	r := ReadBoxLoops(path)
	// AN ABSTENTION IS THE RIGHT ANSWER FROM ONE REAL SEAT CLASS, and until
	// ranger-base-zm03j this line was a t.Fatalf that said so was impossible.
	// MEASURED 2026-11-21 from a child user namespace on this box (`unshare -U
	// -r`, uid_map `0 1001 1`, both live loops EACCES on exe and readable on
	// comm): the census abstained — correctly, which is the whole of what
	// ranger-base-rph5z fixed — and this arm FAILED on it, "a linux box with an
	// installed posse must yield a reading, not an abstention". That is the
	// venue the security review's finding was measured in and the one the fix
	// is for, so the arm cannot call it a failure; and the comment below about
	// asserting the implication in both directions was unreachable there,
	// because the Fatalf above it ran first. What a reading from such a seat
	// owes is the shape of the abstention, so that is what is asserted.
	if r.Err != nil {
		if r.Unreadable == 0 {
			t.Fatalf("the census abstained with nothing of ours refused, so this is not the caged-reader case and /proc answered: %v", r.Err)
		}
		s := r.Sentence()
		if strings.Contains(s, "no dispatch loop on this box runs") {
			t.Errorf("an abstention rendered the all-clear — the one sentence boxloops.go says must never be printable:\n%s", s)
		}
		if !boxSentenceAdmitsItDidNotCoverTheBox(s) {
			t.Errorf("an abstention's rendering does not admit it did not cover the box:\n%s", s)
		}
		t.Logf("live census of %s: ABSTAINED with %d process(es) of this account refused — the child-userns reader class (ranger-base-rph5z). Err: %v",
			AbbrevHome(path), r.Unreadable, r.Err)
		t.Logf("rendering:\n%s", s)
		return
	}
	if r.OnDisk == "" {
		t.Errorf("the census could not stat %s, so nothing it found can be called stale or fresh", AbbrevHome(path))
	}
	for _, p := range r.Procs {
		if p.Pid == os.Getpid() {
			t.Errorf("the census reported the reading process (pid %d)", p.Pid)
		}
		if p.Image == "" {
			t.Errorf("pid %d was admitted as a candidate with no image identity", p.Pid)
		}
		if !p.Loop {
			continue
		}
		rec, ok := boxRecordedLoopPid(p.Home)
		if !ok || rec != p.Pid {
			t.Errorf("pid %d was confirmed as %s's loop, but a second read of that instance's record says %d (ok=%v)",
				p.Pid, AbbrevHome(p.Home), rec, ok)
		}
	}
	// The partial/complete verdict is the thing the security review's finding
	// turned on,
	// so the live arm states it rather than leaving it to be inferred: from a
	// shims seat it must be complete, and from a child-userns seat it must be
	// partial. The arm cannot choose which seat it runs on, so it asserts the
	// IMPLICATION in both directions instead of a fixed answer.
	if r.Partial() && len(r.Procs) == 0 {
		t.Error("nothing readable and something refused is an abstention — Err must be set, and it was not")
	}
	if !r.Partial() && r.Unreadable != 0 {
		t.Errorf("Unreadable=%d but Partial() is false — the caveat would not be rendered", r.Unreadable)
	}
	t.Logf("live census of %s (on disk %s): %d candidate(s), %d loop(s), %d stale, %d unreadable (partial=%v)",
		AbbrevHome(path), r.OnDisk, len(r.Procs), len(r.Loops()), len(r.StaleLoops()), r.Unreadable, r.Partial())
	for _, p := range r.Procs {
		t.Logf("  pid %-8d image %-14s home %-28s loop=%v %s", p.Pid, p.Image, AbbrevHome(p.Home), p.Loop, p.Why)
	}
	t.Logf("rendering:\n%s", r.Sentence())
}

// ─── boxloops_linux.go's own three reads, against real pids ──────────────────
//
// WHY THIS EXISTS, and it is a family rather than a tail. Every pin above
// drives `readBoxLoops` through `boxProbe`, which substitutes all four
// platform reads, so the PLATFORM half of ranger-base-rph5z's and
// ranger-base-cwboz's fix had exactly one reader —
// TestLiveBoxCensusReadsThisBox — and that arm asserts implications which
// hold whatever those functions answer. MEASURED 2026-11-21 under
// ranger-base-zm03j, each mutant applied alone with the subject untouched,
// graded against the whole `^TestQABox|^TestLiveBox|^TestQATheVerbsBoxLine`
// filter: three of them SURVIVED green.
//
//   - `boxHomeOf`'s own fence deleted, the portable chokepoint in
//     `readBoxLoops` left in place — which is precisely the "two checks on
//     purpose" pair ranger-base-cwboz (a) landed, so the pair's second half
//     was a sentence no run tested.
//   - `boxPidOwned` returning true without asking the uid.
//   - `boxCommOf` answering `("", false)` always, which puts back the
//     always-on caveat that ranger-base-rph5z's second round existed to
//     remove, with every pin in the package green.
//
// So each is pinned here against the only fixture these three have: real pids
// on this box, plus one child of this test for the one value a peer controls.
func TestQABoxPlatformReadsAnswerForRealPidsOnThisBox(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "linux" {
		t.Skipf("no /proc on %s — boxloops_other.go's refusal is the behaviour there", runtime.GOOS)
	}

	// boxPidOwned: the difference between "refused" and "gone", which is what
	// lets a refusal be counted at all.
	if !boxPidOwned(os.Getpid()) {
		t.Error("boxPidOwned says this very process is not ours, so every refusal on this box would be dropped as a dead pid")
	}
	if boxPidOwned(1 << 30) {
		t.Error("boxPidOwned says a pid with no /proc entry is ours — a process that exited would then be counted against the reading's completeness forever")
	}
	switch st, err := os.Stat("/proc/1"); {
	case err != nil:
		t.Logf("no /proc/1 to read, so the another-account arm has no fixture here: %v", err)
	case st.Sys().(*syscall.Stat_t).Uid == uint32(os.Getuid()):
		t.Log("pid 1 belongs to this uid on this box, so it cannot serve as the another-account arm")
	default:
		if boxPidOwned(1) {
			t.Error("boxPidOwned claims pid 1, which another account owns — a stranger's refusal would make every reading on every box permanently partial")
		}
	}

	// boxCommOf AND the narrowing it feeds, in BOTH directions over ONE pid.
	// A refused process of ours whose comm names another program must not be
	// counted (the always-on caveat), and the SAME pid read against a target
	// named after its own program must be (the false all-clear). One pid, two
	// target paths, opposite answers: a mutant that stops reading comm, or one
	// that stops narrowing, reds one of the two whichever way it falls.
	//
	// THE FIXTURE IS FOUND WITHOUT THE FUNCTION UNDER TEST, which is the whole
	// reason the search reads /proc/<pid>/comm itself instead of calling
	// boxCommOf: with boxCommOf mutated to answer ("", false) the first
	// spelling of this arm found NO fixture and logged its way to green,
	// MEASURED under ranger-base-zm03j — a rig that builds its fixture from
	// the list under test makes every mutant of that list equivalent.
	const decoy = "qa-decoy-progra" // boxCommLen bytes, so no truncation moves it
	refusedPid, refusedComm := 0, ""
	pids, err := boxPids()
	if err != nil {
		t.Fatalf("/proc would not list, so nothing below has a fixture: %v", err)
	}
	for _, pid := range pids {
		if pid == os.Getpid() {
			continue
		}
		if _, _, ok := procExe(pid); ok {
			continue
		}
		if !boxPidOwned(pid) {
			continue
		}
		raw, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/comm")
		if err != nil {
			continue
		}
		c := strings.TrimRight(string(raw), "\n")
		if c == "" || c == decoy {
			continue
		}
		refusedPid, refusedComm = pid, c
		break
	}
	if refusedPid == 0 {
		t.Log("no process of this account on this box has a refused exe link and a readable comm — this box is not a fixture for the narrowing arms (a bare CI runner is one such box)")
	} else {
		if got, ok := boxCommOf(refusedPid); !ok || got != refusedComm {
			t.Errorf("boxCommOf(%d) = %q ok=%v where /proc/%d/comm reads %q — comm is world-readable with no ptrace check, and it is the only thing that tells a refusal worth reporting from the four non-dumpable processes that are permanently refused on this box (ranger-base-rph5z round 2)",
				refusedPid, got, ok, refusedPid, refusedComm)
		}
		p := liveBoxProbe()
		if boxRefusalWorthReporting(p, refusedPid, "/nowhere/"+decoy) {
			t.Errorf("pid %d (comm %q) was counted against a reading about /nowhere/%s — a refusal that could not be a loop on the subject binary puts the caveat on every reading on this box forever, which is what ranger-base-rph5z's second round removed",
				refusedPid, refusedComm, decoy)
		}
		if !boxRefusalWorthReporting(p, refusedPid, "/nowhere/"+refusedComm) {
			t.Errorf("pid %d is ours, its exe is refused and its comm %q names the very binary the reading is about, and it was NOT counted — that is the false all-clear ranger-base-rph5z was filed on",
				refusedPid, refusedComm)
		}
		t.Logf("narrowing measured on pid %d (comm %q): refused, ours, counted for /nowhere/%s and not for /nowhere/%s",
			refusedPid, refusedComm, refusedComm, decoy)
	}

	// boxHomeOf's OWN fence (ranger-base-cwboz (a), the platform half of the
	// pair). The value comes from a CHILD of this test rather than from a
	// planted attacker, which is the derivation the finding named: a caged
	// seat's own child qualifies, and the census's first live reading found
	// exactly such a child.
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skipf("no sleep(1) to hold a child open, so the environ arm has no fixture: %v", err)
	}
	for _, c := range []struct {
		why, val string
		want     bool
	}{
		{"an ordinary absolute clean path", "/homes/ranger", true},
		{"a newline — the line goes on an install bead", "/homes/ranger\nPLEASE RUN THIS INSTEAD", false},
		{"relative — it would resolve against the verb's cwd", "homes/ranger", false},
		{"not clean — it hides its target from the reader of the line", "/homes/../homes/ranger", false},
		{"an escape sequence — the line is read in a terminal", "/homes/ranger\x1b[2J", false},
	} {
		got, ok := boxHomeOfChild(t, sleep, c.val)
		switch {
		case c.want && (!ok || got != c.val):
			t.Errorf("boxHomeOf refused %s (%q): got %q ok=%v — then the census can name no instance at all and every candidate is unattributed", c.why, c.val, got, ok)
		case !c.want && ok:
			t.Errorf("boxHomeOf returned %q for %s — the platform read is meant to refuse before anything malformed leaves the environ (ranger-base-cwboz)", got, c.why)
		}
	}
}

// boxHomeOfChild starts one child exporting RHQ_HOME=val and asks boxHomeOf
// what it declares.
//
// IT WAITS FOR THE EXEC TO SETTLE, and that is not caution for its own sake:
// exec.Cmd.Start returns once the child's exec is REPORTED, and /proc/<pid>'s
// mm is not reliably the new image's when the parent is next scheduled —
// watchimage_qa_test.go's three fixtures read 2 failures in 20 runs that way
// (ranger-base-rph5z's third defect), and the environ of an unsettled child is
// this process's own, which carries whatever RHQ_HOME this session was
// launched with. So the poll is on a sentinel only the child's env holds.
func boxHomeOfChild(t *testing.T, sleep, val string) (string, bool) {
	t.Helper()
	cmd := exec.Command(sleep, "600")
	cmd.Env = []string{"POSSE_BOX_PROBE=settled", "RHQ_HOME=" + val}
	if err := cmd.Start(); err != nil {
		t.Fatalf("could not start a child to carry the environ: %v", err)
	}
	// Our own pid, so this is the kill AGENTS.md prescribes: one pid, the one
	// this call launched, and never a pattern.
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})
	settled := false
	for i := 0; i < 400; i++ {
		raw, err := os.ReadFile("/proc/" + strconv.Itoa(cmd.Process.Pid) + "/environ")
		if err == nil && strings.Contains(string(raw), "POSSE_BOX_PROBE=settled") {
			settled = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !settled {
		t.Fatalf("the child's environ never carried this probe's sentinel, so a read of it would be this process's own environment and not the child's (pid %d)", cmd.Process.Pid)
	}
	return boxHomeOf(cmd.Process.Pid)
}

// TestQABoxCensusSaysUnknownWhenItSawNoLoopAndWasRefusedOne is `Sentence`'s
// first partial arm, and until ranger-base-zm03j no pin in this file reached
// it — the one arm standing directly in front of the false all-clear.
//
// THE SHAPE IT NEEDS, which is why no other fixture here produces it: a
// reader that CAN read one candidate, that candidate is NOT a loop, and the
// loops themselves are REFUSED. Then `len(loops) == 0` with a reading in hand
// and `Sentence` must say UNKNOWN rather than "no dispatch loop on this box".
// Every other refusal fixture above either has no readable candidate at all,
// so the abstention arm answers, or has a confirmed loop among the readable
// ones, so a later partial arm does. It is not a contrived arrangement: the
// unattributed candidate is the measured box's own third row — a seat's
// `posse pulse wait` child running the same image under the ranger home — and
// the refusals are what a child-userns seat gets for the loops beside it.
//
// MEASURED under ranger-base-zm03j by deleting the arm: the whole
// `^TestQABox|^TestLiveBox` filter stayed GREEN and the next arm rendered the
// plain all-clear over two loops the reader had been refused.
func TestQABoxCensusSaysUnknownWhenItSawNoLoopAndWasRefusedOne(t *testing.T) {
	t.Parallel()
	path, probe := boxFixture("14541392", []boxFixtureProc{
		// Readable, runs the image, and its own instance's record names
		// another pid — a candidate, never a loop.
		{pid: 3285561, ino: "14541392", home: "/homes/ranger"},
		// The two loops, refused. comm defaults to the subject's basename,
		// so both are refusals worth reporting.
		{pid: 1185112, refused: true},
		{pid: 1185185, refused: true},
	}, map[string]int{"/homes/ranger": 1185185, "/homes/weller": 1185112}, 9999)
	r := readBoxLoops(path, probe)
	if r.Err != nil {
		t.Fatalf("one readable candidate is a reading, not an abstention: %v", r.Err)
	}
	if len(r.Loops()) != 0 || !r.Partial() || r.Unreadable != 2 {
		t.Fatalf("CONTROL: this fixture must give no loop, a partial reading and two refusals, or it is not this arm: loops=%d partial=%v unreadable=%d",
			len(r.Loops()), r.Partial(), r.Unreadable)
	}
	s := r.Sentence()
	if strings.Contains(s, "no dispatch loop on this box runs") {
		t.Fatalf("THE false all-clear, over two loops this reader was refused — the one sentence boxloops.go says must never be printable:\n%s", s)
	}
	if !boxSentenceAdmitsItDidNotCoverTheBox(s) {
		t.Errorf("the reading does not admit it did not cover the box:\n%s", s)
	}
	for _, want := range []string{"COULD read", "not a clear box", "3285561"} {
		if !strings.Contains(s, want) {
			t.Errorf("the rendering does not carry %q — it owes its reader what it DID see as well as what it could not:\n%s", want, s)
		}
	}
}
