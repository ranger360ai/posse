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
	"testing"
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
				exe := p.exe
				if exe == "" {
					exe = path
				}
				return ids(p.ino), exe, true
			}
			return "", "", false
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
	t.Logf("live census of %s (on disk %s): %d candidate(s), %d loop(s), %d stale",
		AbbrevHome(path), r.OnDisk, len(r.Procs), len(r.Loops()), len(r.StaleLoops()))
	for _, p := range r.Procs {
		t.Logf("  pid %-8d image %-14s home %-28s loop=%v %s", p.Pid, p.Image, AbbrevHome(p.Home), p.Loop, p.Why)
	}
	t.Logf("rendering:\n%s", r.Sentence())
}
