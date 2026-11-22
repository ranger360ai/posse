//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-xab99: the Keep leaf (the session's two audit logs, re-allowed
// for write after the carve-out's deny) and the ref keep file are FILE leaves
// the render either binds in place or has the uncaged launcher touch. Both
// were decided by bwrapStat, which follows a link, and the Keep leaf was
// absResolve'd before either render saw it — so a link planted at
// <gates>/refusals.log bound its target read-write, a dangling one had the
// launcher create its target, and at L2 the target became a write-allowed
// literal. These pins hold the closed shape: the leaf keeps its declared
// spelling, is lstat'd, and a link there refuses the render.
//
// MUTATION RUNS (2026-11-21, linux, each reverted after): re-resolving the
// Keep leaf in SeatbeltCarveOut reds the shipped-render pin and the seatbelt
// pin; disabling bwrapLeafStat's symlink refusal reds the ref-keep pin (the
// Keep arm still refuses, by its plain-file check — two layers, on purpose);
// disabling that plain-file check reds the hard-link/FIFO pin; dropping
// O_EXCL from PrepTouch hung the executor pin on the FIFO, which is why that
// arm is bounded; skipping keepLeafRefusal reds the seatbelt pin.

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// klOutside is a directory inside no grant of any render below: the
// planter's chosen target.
func klOutside(t *testing.T, beside string) string {
	t.Helper()
	d := filepath.Join(filepath.Dir(beside), "outside")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	return d
}

// Pin: through the SHIPPED render, for every runtime, a real audit log is
// bound in place (control), a link at its name to a host file refuses the
// render and never names the target (plant), and a dangling link there
// refuses the render and asks the launcher to touch nothing (dangling).
func TestQAKeepLeafShippedRenderRefusesALinkAtTheAuditLog(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	f := wpsSeatTree(t)
	if err := os.MkdirAll(ClaudeConfigDirIn(f.home), 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(f.gates, "refusals.log")
	outside := klOutside(t, f.dir)

	for _, name := range []string{"claude", "codex", "grok"} {
		_ = os.Remove(keep)
		r, err := f.a.bwrapRender(f.ag, wrcRuntime(name), f.dir, f.gates)
		if err != nil {
			t.Fatalf("%s CONTROL: the untouched render refused: %v", name, err)
		}
		if !strings.Contains(bwJoin(r.Argv), "--bind "+keep+" "+keep) {
			t.Fatalf("%s CONTROL: no in-place bind of %s, so nothing below measures the Keep leaf:\n%s", name, keep, bwJoin(r.Argv))
		}

		tgt := filepath.Join(outside, "stolen-"+name)
		if err := os.WriteFile(tgt, []byte("host content\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(tgt, keep); err != nil {
			t.Fatal(err)
		}
		r2, err := f.a.bwrapRender(f.ag, wrcRuntime(name), f.dir, f.gates)
		if err == nil {
			t.Errorf("%s PLANT: a link at %s -> %s rendered; argv names the target: %v", name, keep, tgt, strings.Contains(bwJoin(r2.Argv), tgt))
		} else if !strings.Contains(err.Error(), "ranger-base-xab99") {
			t.Errorf("%s PLANT: refused, but not by the Keep leaf's check: %v", name, err)
		}

		_ = os.Remove(keep)
		dangling := filepath.Join(outside, "not-there-"+name)
		if err := os.Symlink(dangling, keep); err != nil {
			t.Fatal(err)
		}
		r3, err := f.a.bwrapRender(f.ag, wrcRuntime(name), f.dir, f.gates)
		if err == nil {
			t.Errorf("%s DANGLING: a dangling link at %s rendered, with prep %v", name, keep, bwPrep(r3.Prep))
		}
		if _, serr := os.Lstat(dangling); !os.IsNotExist(serr) {
			t.Errorf("%s DANGLING: the link's target %s exists after the render", name, dangling)
		}
		_ = os.Remove(keep)
	}
}

// Pin: the same leaf through BwrapArgs over a hand-built carve, for the
// shapes the shipped census does not reach — a hard link and a FIFO at the
// name refuse, a symlink to a file INSIDE a grant still refuses (the rule is
// the link, not where it points), and an absent leaf is touched.
func TestQAKeepLeafRefusesEveryNonPlainFileAtTheName(t *testing.T) {
	// No t.Parallel: sets HOME.
	root := wallUngrantedDir(t)
	home, proj := filepath.Join(root, "home"), filepath.Join(root, "proj")
	gates := filepath.Join(root, "state", "gates", "dev")
	outside := filepath.Join(root, "outside")
	for _, d := range []string{home, proj, gates, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	if bwrapWallRoot() == "" {
		t.Skip("NOTHING MEASURED: ADR 0100's rules stand down for a home under " + bwrapTmpMount)
	}
	keep := filepath.Join(gates, "refusals.log")
	grants := []string{proj, filepath.Join(root, "state"), gates}
	carve := SeatbeltCarveOut{Deny: []string{filepath.Dir(gates)}, Keep: []string{keep}}

	// CONTROL, absent: a touch, then an in-place bind.
	argv, prep, err := BwrapArgs(proj, grants, carve)
	if err != nil {
		t.Fatalf("CONTROL (absent) refused: %v", err)
	}
	if !containsString(bwPrep(prep), "touch "+keep) || wcpMountAt(argv, "--bind", keep, keep) < 0 {
		t.Fatalf("CONTROL (absent): want a touch and an in-place bind of %s; prep %v", keep, bwPrep(prep))
	}
	// CONTROL, a plain file.
	if err := os.WriteFile(keep, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if argv, _, err = BwrapArgs(proj, grants, carve); err != nil || wcpMountAt(argv, "--bind", keep, keep) < 0 {
		t.Fatalf("CONTROL (file): want an in-place bind of %s, got err %v", keep, err)
	}

	plant := func(what string, mk func() error) {
		t.Helper()
		_ = os.Remove(keep)
		if err := mk(); err != nil {
			t.Fatalf("%s: plant: %v", what, err)
		}
		if _, _, err := BwrapArgs(proj, grants, carve); err == nil || !strings.Contains(err.Error(), "ranger-base-xab99") {
			t.Errorf("%s at %s: want the Keep leaf's refusal, got %v", what, keep, err)
		}
	}
	inside := filepath.Join(proj, "granted-file")
	if err := os.WriteFile(inside, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	plant("a symlink to a file inside a grant", func() error { return os.Symlink(inside, keep) })
	secret := filepath.Join(outside, "secret")
	if err := os.WriteFile(secret, []byte("s"), 0o600); err != nil {
		t.Fatal(err)
	}
	plant("a hard link to a file inside no grant", func() error { return os.Link(secret, keep) })
	plant("a FIFO", func() error { return syscall.Mkfifo(keep, 0o644) })
	_ = os.Remove(keep)
}

// Pin: the ref keep file (step 2b). Its directory is the seat's own rw ref
// grant, so the seat can leave a dangling link at its name; the render must
// refuse that rather than read it as absent and emit an uncaged touch.
func TestQARefKeepLeafRefusesALinkAtTheKeepFile(t *testing.T) {
	// No t.Parallel: sets HOME.
	root := wallUngrantedDir(t)
	home, proj := filepath.Join(root, "home"), filepath.Join(root, "proj")
	refDir := filepath.Join(root, "common", "refs", "heads", "posse")
	outside := filepath.Join(root, "outside")
	for _, d := range []string{home, proj, refDir, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	if bwrapWallRoot() == "" {
		t.Skip("NOTHING MEASURED: ADR 0100's rules stand down for a home under " + bwrapTmpMount)
	}
	keep := filepath.Join(refDir, bwrapRefKeepName)
	grants := []string{proj, refDir}

	_, prep, err := BwrapArgs(proj, grants, SeatbeltCarveOut{}, keep)
	if err != nil {
		t.Fatalf("CONTROL (absent) refused: %v", err)
	}
	if !containsString(bwPrep(prep), "touch "+keep) {
		t.Fatalf("CONTROL (absent): want a touch of %s, got %v", keep, bwPrep(prep))
	}

	dangling := filepath.Join(outside, "packed-refs.lock")
	if err := os.Symlink(dangling, keep); err != nil {
		t.Fatal(err)
	}
	_, prep, err = BwrapArgs(proj, grants, SeatbeltCarveOut{}, keep)
	if err == nil || !strings.Contains(err.Error(), "ranger-base-xab99") {
		t.Errorf("DANGLING: a dangling link at %s rendered (err %v) with prep %v", keep, err, bwPrep(prep))
	}
	_ = os.Remove(keep)
	live := filepath.Join(outside, "live")
	if err := os.WriteFile(live, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(live, keep); err != nil {
		t.Fatal(err)
	}
	if _, _, err = BwrapArgs(proj, grants, SeatbeltCarveOut{}, keep); err == nil || !strings.Contains(err.Error(), "ranger-base-xab99") {
		t.Errorf("PLANT: a link at %s to a file rendered (err %v)", keep, err)
	}
}

// Pin: the executor. PrepTouch runs UNCAGED, so it must never create through
// a link: a dangling link at the name fails the prep and its target stays
// absent; a hard link at the name fails it too (ranger-base-jum4x); an absent
// name is created; a regular file already there is left.
func TestQAPrepTouchNeverCreatesThroughALink(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	tgt := filepath.Join(d, "target")
	link := filepath.Join(d, "link")
	if err := os.Symlink(tgt, link); err != nil {
		t.Fatal(err)
	}
	if err := runBwrapPrep([]PrepStep{{PrepTouch, link, "pin"}}); err == nil {
		t.Errorf("a PrepTouch over a dangling link succeeded")
	}
	if _, err := os.Lstat(tgt); !os.IsNotExist(err) {
		t.Errorf("a PrepTouch over a dangling link created its target %s (lstat err %v)", tgt, err)
	}

	fifo := filepath.Join(d, "fifo")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Fatal(err)
	}
	// Bounded: an open of a FIFO for write blocks until a reader arrives, so
	// a regression here HANGS rather than fails (MEASURED in the mutation run
	// below: 600s to the package timeout). On timeout the pin opens the read
	// end itself to release the blocked open, then reds.
	done := make(chan error, 1)
	go func() { done <- runBwrapPrep([]PrepStep{{PrepTouch, fifo, "pin"}}) }()
	select {
	case err := <-done:
		if err == nil {
			t.Errorf("a PrepTouch over a FIFO succeeded")
		}
	case <-time.After(5 * time.Second):
		if r, err := os.OpenFile(fifo, os.O_RDONLY, 0); err == nil {
			<-done
			r.Close()
		}
		t.Errorf("a PrepTouch over a FIFO blocked opening it: the launcher opens what is planted at the name")
	}

	// A HARD LINK is lstat-regular, so it reaches the EEXIST fallback's
	// accept rather than its refusal; the fallback must ask the render's
	// nlink question too, or a link to a host file planted after the render
	// is bound read-write (ranger-base-jum4x). Neither arm above reaches it.
	host := filepath.Join(d, "host-secret")
	if err := os.WriteFile(host, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	hard := filepath.Join(d, "hard")
	if err := os.Link(host, hard); err != nil {
		t.Fatal(err)
	}
	if err := runBwrapPrep([]PrepStep{{PrepTouch, hard, "pin"}}); err == nil {
		t.Errorf("a PrepTouch over a hard link (nlink 2) succeeded: the launcher accepts a leaf the render refuses")
	} else if !strings.Contains(err.Error(), "ranger-base-jum4x") {
		t.Errorf("a PrepTouch over a hard link refused for the wrong reason: %v", err)
	}

	fresh := filepath.Join(d, "fresh")
	if err := runBwrapPrep([]PrepStep{{PrepTouch, fresh, "pin"}}); err != nil {
		t.Fatalf("CONTROL: a PrepTouch of an absent name failed: %v", err)
	}
	if fi, err := os.Lstat(fresh); err != nil || !fi.Mode().IsRegular() {
		t.Fatalf("CONTROL: %s is not a regular file after the touch (err %v)", fresh, err)
	}
	if err := os.WriteFile(fresh, []byte("kept"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runBwrapPrep([]PrepStep{{PrepTouch, fresh, "pin"}}); err != nil {
		t.Errorf("CONTROL: a PrepTouch over an existing regular file failed: %v", err)
	}
	if b, _ := os.ReadFile(fresh); string(b) != "kept" {
		t.Errorf("CONTROL: a PrepTouch truncated an existing file to %q", b)
	}
}

// Pin: the L2 half. The carve-out's Keep list carries the leaf's DECLARED
// spelling, never a planted link's target, so the SBPL write literal never
// names a path outside the gates dir — and the seatbelt render refuses the
// plant outright rather than emitting a literal a seat could have staged.
//
// EVERY ARM OF keepLeafRefusal, EACH BY ITS OWN MESSAGE (ranger-base-ul49h).
// It has two refusing arms and they OVERLAP: a symlink is not a regular
// file, so either arm alone refuses the symlink this pin used to be the
// whole of — and MEASURED 2026-11-21, linux, over the five xab99 pins, each
// of three narrower mutants left all five green: dropping the symlink arm,
// dropping the non-regular/hard-link arm, and dropping just the
// `|| !storeSingleEntry(fi)` half of it. The combined mutant (skipping
// keepLeafRefusal outright) was the only one that red, so the L2 side was
// graded as one arm while the L1 side next door is graded per arm by
// TestQAKeepLeafRefusesEveryNonPlainFileAtTheName. The hard link and the
// FIFO below witness the non-regular arm's DECISION; asserting the symlink
// refusal's own wording witnesses the symlink arm, whose only product is
// that message, since nothing it refuses the other arm would pass.
func TestQAKeepLeafSeatbeltNeverAllowsAPlantedTarget(t *testing.T) {
	// No t.Parallel: the fixture sets HOME and the CLI home variables.
	f := wpsSeatTree(t)
	if err := os.MkdirAll(f.gates, 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(f.gates, "refusals.log")
	outside := klOutside(t, f.dir)
	tgt := filepath.Join(outside, "stolen")
	if err := os.WriteFile(tgt, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	// CONTROL: the untouched carve names the leaf, and the render goes through.
	_ = os.Remove(keep)
	if _, err := f.a.RenderSeatbelt(f.ag, f.dir); err != nil {
		t.Fatalf("CONTROL: the untouched seatbelt render refused: %v", err)
	}

	if err := os.Symlink(tgt, keep); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(keep) })
	writable, carve := f.a.seatbeltSets(f.ag, f.dir, f.gates)
	if !containsString(carve.Keep, keep) {
		t.Errorf("the carve-out's Keep list lost the declared leaf %s: %v", keep, carve.Keep)
	}
	if containsString(carve.Keep, tgt) {
		t.Errorf("the carve-out's Keep list carries the planted target %s: %v", tgt, carve.Keep)
	}
	if prof := SeatbeltProfile(f.ag.Name, writable, nil, carve); strings.Contains(prof, tgt) {
		t.Errorf("the SBPL profile names the planted target %s", tgt)
	}
	if _, err := f.a.RenderSeatbelt(f.ag, f.dir); err == nil || !strings.Contains(err.Error(), "ranger-base-xab99") {
		t.Errorf("a link at %s rendered a seatbelt profile (err %v)", keep, err)
	} else if !strings.Contains(err.Error(), "it is a symlink") {
		// The symlink arm's own wording: the non-regular arm below would
		// refuse this same plant, so the message is the only thing that says
		// which arm answered.
		t.Errorf("the symlink at %s was refused by the wrong arm: %v", keep, err)
	}

	// The non-regular arm's DECISION, over the two shapes the arm above
	// cannot reach: a hard link hands over another inode under the log's
	// name exactly as a link's target was handed over, and a FIFO is nothing
	// an audit log appends to.
	for _, c := range []struct {
		what string
		mk   func() error
	}{
		{"a hard link to a file inside no grant", func() error { return os.Link(tgt, keep) }},
		{"a FIFO", func() error { return syscall.Mkfifo(keep, 0o644) }},
	} {
		_ = os.Remove(keep)
		if err := c.mk(); err != nil {
			t.Fatalf("%s: plant: %v", c.what, err)
		}
		_, err := f.a.RenderSeatbelt(f.ag, f.dir)
		switch {
		case err == nil:
			t.Errorf("%s at %s rendered a seatbelt profile", c.what, keep)
		case !strings.Contains(err.Error(), "ranger-base-xab99"),
			!strings.Contains(err.Error(), "not a regular file with one name"):
			t.Errorf("%s at %s: want the Keep leaf's plain-file refusal, got %v", c.what, keep, err)
		}
	}
	_ = os.Remove(keep)
}
