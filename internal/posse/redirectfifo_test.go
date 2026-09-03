package posse

// ranger-base-fvfve: the THIRD reader of the class ranger-base-gs9r opened —
// identityRedirectTarget (visibility.go) read <repo>/.beads/redirect with no
// type check, and os.ReadFile on a FIFO with no writer never returns. Its
// caller DeriveIdentityLiterals is reached from InstallCommitGuardHook, from
// InstallCommitGuardHookChained and from the L3 probe, all ABOVE
// ranger-base-92n5p's own two readers on the launch path, so the launcher
// still hung on one mkfifo — just at a different path, with nothing printed
// and no deadline anywhere above it.
//
// Every arm below carries a control in the same rig, through the same call,
// before the FIFO arm, so a BLOCKED verdict is the file type and not the
// harness. The regular-redirect controls also assert the VALUE, so a guard
// that answered "" for every redirect would fail here rather than pass by
// being quiet everywhere.

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestAFifoAtTheBeadsRedirectMustNotWedgeTheLaunch(t *testing.T) {
	t.Parallel()
	// CONTROL 1: no redirect at all — nothing to derive, and it returns.
	bare := t.TempDir()
	if err := os.MkdirAll(filepath.Join(bare, beadsDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	var lits []IdentityLiteral
	if !returnsWithin(t, 10*time.Second, func() { lits, _ = DeriveIdentityLiterals(bare) }) {
		t.Fatalf("CONTROL: DeriveIdentityLiterals blocked with no redirect planted — the rig, not the code")
	}
	if got := instancePathLiterals(lits); len(got) != 0 {
		t.Errorf("CONTROL: no redirect derived instance-path literals %v", got)
	}

	// CONTROL 2: an ordinary regular redirect — the value the guard must not
	// swallow. The redirect names a .beads DIRECTORY (beadsHome's contract);
	// the instance-path literal is that directory's parent.
	reg := t.TempDir()
	if err := os.MkdirAll(filepath.Join(reg, beadsDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(reg, beadsDirName, beadsRedirect), []byte("../instance/.beads\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !returnsWithin(t, 10*time.Second, func() { lits, _ = DeriveIdentityLiterals(reg) }) {
		t.Fatalf("CONTROL: DeriveIdentityLiterals blocked on a regular redirect — the rig, not the code")
	}
	want := filepath.Join(filepath.Dir(reg), "instance")
	if err := os.MkdirAll(filepath.Join(want, beadsDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := instancePathLiterals(lits); !containsString(got, want) {
		t.Errorf("CONTROL: a regular redirect derived %v, want one of them %s", got, want)
	}

	// THE DEFECT: a named pipe there.
	fifo := t.TempDir()
	plantRedirectFifo(t, fifo)
	if !returnsWithin(t, 10*time.Second, func() { lits, _ = DeriveIdentityLiterals(fifo) }) {
		t.Fatalf("DeriveIdentityLiterals blocked forever on a FIFO at .beads/redirect")
	}
	if got := instancePathLiterals(lits); len(got) != 0 {
		t.Errorf("a FIFO redirect derived instance-path literals %v — a special file is not a redirect posse wrote", got)
	}

	// beadsHome (beadloss.go) reads the SAME path, and CheckParityIn's
	// applyRecordReach reaches it on the launch path too — measured blocking
	// planLaunch past 60s with identityRedirectTarget already guarded, which
	// is how it was found. Same three fixtures, same shape of answer: an
	// unreadable redirect means the local .beads.
	var home string
	if !returnsWithin(t, 10*time.Second, func() { home = beadsHome(bare) }) {
		t.Fatalf("CONTROL: beadsHome blocked with no redirect planted — the rig, not the code")
	}
	if got, want := home, filepath.Join(bare, beadsDirName); got != want {
		t.Errorf("CONTROL: beadsHome with no redirect = %s, want %s", got, want)
	}
	if !returnsWithin(t, 10*time.Second, func() { home = beadsHome(reg) }) {
		t.Fatalf("CONTROL: beadsHome blocked on a regular redirect — the rig, not the code")
	}
	if got, wantHome := home, filepath.Join(want, beadsDirName); got != wantHome {
		t.Errorf("CONTROL: beadsHome over a regular redirect = %s, want the redirect target %s", got, wantHome)
	}
	if !returnsWithin(t, 10*time.Second, func() { home = beadsHome(fifo) }) {
		t.Fatalf("beadsHome blocked forever on a FIFO at .beads/redirect")
	}
	if got, want := home, filepath.Join(fifo, beadsDirName); got != want {
		t.Errorf("beadsHome over a FIFO redirect = %s, want the local %s", got, want)
	}

	// End to end, the symptom the bead is named for: a launch that never
	// returns. The control launch first, over the same fixture shape with no
	// pipe planted, so a blocked launch below is the FIFO.
	b, clean := lhpFixture(t, VisibilityPrivate)
	if !returnsWithin(t, 90*time.Second, func() {
		b.planLaunch(NewSessionOpts{Name: "s0", Dir: clean, Agent: "ranger"})
	}) {
		t.Fatalf("CONTROL: planLaunch blocked over a repo with no redirect — the rig, not the code")
	}
	b2, wedged := lhpFixture(t, VisibilityPrivate)
	plantRedirectFifo(t, wedged)
	if !returnsWithin(t, 60*time.Second, func() {
		b2.planLaunch(NewSessionOpts{Name: "s1", Dir: wedged, Agent: "ranger"})
	}) {
		t.Errorf("planLaunch blocked forever on a 0644 FIFO at .beads/redirect — the launcher still wedges")
	}
}

// plantRedirectFifo makes dir/.beads and puts a 0644 named pipe at its
// redirect, with the fixture's own witness that it really is a pipe — a pin
// over a plain file would pass a broken build.
func plantRedirectFifo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, beadsDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, beadsDirName, beadsRedirect)
	_ = os.Remove(p)
	if err := syscall.Mkfifo(p, 0o644); err != nil {
		t.Fatalf("mkfifo %s: %v", p, err)
	}
	if err := os.Chmod(p, 0o644); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Lstat(p)
	if err != nil || fi.Mode()&os.ModeNamedPipe == 0 {
		t.Fatalf("fixture at %s is not a FIFO: mode %v (%v)", p, fi.Mode(), err)
	}
}

func instancePathLiterals(lits []IdentityLiteral) []string {
	var out []string
	for _, l := range lits {
		if strings.HasPrefix(l.Class, "instance-path") {
			out = append(out, l.Value)
		}
	}
	return out
}
