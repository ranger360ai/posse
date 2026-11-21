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
	"path/filepath"
	"reflect"
	"runtime"
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

// --- the second review finding: the home it lifts is another process's text ---

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
		t.Fatal("boxRecordedLoopPid blocked on a FIFO — this is the hang that would stop `posse upgrade` after its swap (the security review's second item)")
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
// WHAT THIS DOES NOT REACH, said here because the next reader should not have
// to find it out: neither loop line is pinned THROUGH `upgradeRunPlan`, so a
// deletion of the `upgradeBoxLine(plan)` call from the report would leave
// every pin in this file green. That gap is the per-instance line's too
// (nothing asserts "dispatch:" appears in a d4Fixture report either), so it
// is one gap over two lines and not a new one — closing it means a report
// fixture that can carry a Loop and a Box, which belongs with the D4 pins
// rather than here.
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
	if r.Err != nil {
		t.Fatalf("a linux box with an installed posse must yield a reading, not an abstention: %v", r.Err)
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
