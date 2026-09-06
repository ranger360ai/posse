//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-p28j, verifying the close of ranger-base-5na against what
// ranger-base-o943 built: three arms of ADR 0015 §3's launch verify that the
// suite held loosely or not at all.
//
// The first was found by mutation and by nothing else: with
// `|| want == notRegular` deleted from VerifyPromoted, all three packages
// stayed green (measured 2026-08-28, full `go test ./...`).

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// A manifest entry posse could not hash is never satisfied by an entry it
// still cannot hash.
//
// The case exists because SeedPromoteManifest records what is ON DISK
// (`posse promote` refuses a non-regular file outright, notRegularIn), so a
// home seeded with a symlink inside the promoted set carries `notRegular`
// as the recorded value. Without the `want == notRegular` arm the comparison
// is got == want and the entry reads CLEAN — which would bless, for the life
// of that manifest, the one entry whose bytes posse never attested to and
// whose target can be rewritten from outside the home at any time.
//
// The arm is the difference between "this link is permanently a mismatch,
// re-promote to clear it" and "this link is fine forever".
func TestQASeededManifestNeverBlessesWhatItCouldNotHash(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	outside := t.TempDir()
	t.Setenv("RHQ_HOME", home)
	a, err := NewApp()
	if err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.ConfigPath, []byte("default_dir: ~\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(a.AgentsDir, "real.md")
	if err := os.WriteFile(real, []byte("---\nname: real\n---\nwork\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// The control comes first and on the SAME home: an all-regular promoted
	// set seeds and verifies clean. Without it, a fixture that fails to seed
	// at all would produce the not-OK verdict this test is looking for and
	// prove nothing.
	if err := a.SeedPromoteManifest(); err != nil {
		t.Fatal(err)
	}
	if v := a.VerifyPromoted(); !v.OK() {
		t.Fatalf("control: an all-regular seeded home does not verify: %s", v.Line())
	}

	// Now the case. Re-seed with a symlink in agents/: remove the manifest
	// first, because SeedPromoteManifest never overwrites one.
	target := filepath.Join(outside, "target.md")
	if err := os.WriteFile(target, []byte("---\nname: link\n---\nfirst\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(a.AgentsDir, "link.md")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("this filesystem does not do symlinks: %v", err)
	}
	if err := os.Remove(a.PromoteManifestPath()); err != nil {
		t.Fatal(err)
	}
	if err := a.SeedPromoteManifest(); err != nil {
		t.Fatal(err)
	}

	// The witness that the fixture built the case it claims to: the manifest
	// records the entry as one posse could not hash. If a later seed learns
	// to skip, resolve or refuse symlinks, this fails HERE and says so,
	// rather than passing for a reason that has nothing to do with the arm.
	m, err := ReadPromoteManifest(a.PromoteManifestPath())
	if err != nil || m == nil {
		t.Fatalf("no seeded manifest: %v %v", m, err)
	}
	if got := m.Files["agents/link.md"]; got != notRegular {
		t.Fatalf("fixture built no unhashable entry: manifest records agents/link.md as %q", got)
	}
	if got := m.Files["agents/real.md"]; got == "" || got == notRegular {
		t.Fatalf("the regular PID beside it is not hashed either (%q) — the fixture is broken, not the arm", got)
	}

	// The arm. got == want == notRegular, and it must still be a mismatch.
	v := a.VerifyPromoted()
	if v.OK() {
		t.Fatalf("a manifest entry posse could not hash verified clean: %+v", v)
	}
	if !contains(v.Changed, "agents/link.md") {
		t.Errorf("the unhashable entry is not reported as changed: %+v", v)
	}
	if !strings.Contains(v.Line(), "agents/link.md") {
		t.Errorf("the launch line does not name it: %s", v.Line())
	}
	// And it cannot be earned back by editing what the link points at —
	// which is the whole reason the entry is not attestable. A re-promote is
	// the only thing that clears it.
	if err := os.WriteFile(target, []byte("---\nname: link\ncage: shims\n---\nsecond\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if v := a.VerifyPromoted(); v.OK() {
		t.Error("rewriting the symlink's target cleared the mismatch")
	}
}

// Every dispatched launch must DECLARE itself dispatched, because that is
// the only thing separating ADR 0015 §3's refusal from its warning:
// planLaunch reads `o.Bead != ""` and nothing else.
//
// dispatch.go has two create call sites — the typed launch and the
// argv/prompt-file one — and they are different code (ranger-base-unzn's
// shape: a per-runtime split that every existing pin drove from one side).
// A path that launches without it is an unwatched session coming up on a
// constitution nobody promoted, with DEGRADED printed to a warn stream no
// human is reading.
//
// What this pin adds, measured rather than assumed: stripping `Bead:` from
// the typed call site DOES redden six existing tests (TestDispatchRecords-
// TheBeadOnTheSessionItLaunches, the auto-reap set, TestRunRecordPersists-
// BeadAndPrompted) — but every one of them is about RECORDING the bead on
// the session it launched, drives one of the two paths that exist today, and
// none of them says why the field is load-bearing for §3. A third call site
// added later is covered by this and by nothing above.
//
// It is a source pin because the behavioural one costs a real dispatch pass
// with real sessions. It counts what it found, so a sweep that matches
// nothing fails instead of passing.
func TestQAEveryDispatchedLaunchDeclaresItsBead(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile("dispatch.go")
	if err != nil {
		t.Fatal(err)
	}
	// Each call site, from `createSession(NewSessionOpts{` to the closing
	// paren of the call — the options are spread over several lines, and the
	// literal is followed by the caller's launcher lock (ranger-base-deaz).
	//
	// Both spellings: dispatch launches through the unexported createSession,
	// which is handed the lock it is nested inside, but CreateSession is the
	// same call for a caller that holds none and a third call site added
	// later is as likely to be written that way. Matching only the one in the
	// file today is how this sweep would go quietly empty — which it did,
	// loudly, when the rename landed: the count guard below is what caught it.
	re := regexp.MustCompile(`(?s)\b[Cc]reateSession\(NewSessionOpts\{.*?\}(?:,\s*\w+)?\)`)
	sites := re.FindAllString(string(src), -1)
	if len(sites) < 2 {
		t.Fatalf("found %d create call sites in dispatch.go; the sweep is not reading the file it thinks it is", len(sites))
	}
	for i, s := range sites {
		if !strings.Contains(s, "Bead:") {
			t.Errorf("dispatch call site %d launches without Bead:, so ADR 0015 §3 warns instead of refusing:\n%s", i+1, s)
		}
	}
	t.Logf("checked %d dispatched create call sites", len(sites))
}

// --allow-degraded is the operator's escape hatch for gates the wall cannot
// realize (herdrback.go:1411). It must not reach the constitution: the §3
// refusal is checked before it and consults nothing, and dispatch passes
// `AllowDegraded: d.AllowDegraded` into every session it launches — so if the
// two were ever folded together, a fleet configured to allow degraded gates
// would silently stop refusing on a constitution nobody promoted.
//
// Nothing else drives the two together: the existing §3 dispatch pin leaves
// AllowDegraded false, and every --allow-degraded pin leaves the manifest
// matching.
func TestQAAllowDegradedDoesNotReachTheConstitutionRefusal(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	pid := promotedTestHome(t, b)
	dir := t.TempDir()

	// The witness: with the constitution intact, this exact call plans.
	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: dir, Agent: "ranger", Bead: "x-1", AllowDegraded: true}); err != nil {
		t.Fatalf("control: --allow-degraded refused a matching constitution: %v", err)
	}
	body, _ := os.ReadFile(pid)
	if err := os.WriteFile(pid, append(body, []byte("\ncage: shims\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := b.planLaunch(NewSessionOpts{Name: "s2", Dir: dir, Agent: "ranger", Bead: "x-1", AllowDegraded: true})
	if err == nil {
		t.Fatal("--allow-degraded launched a dispatched session on an unpromoted constitution")
	}
	if !strings.Contains(err.Error(), "agents/ranger.md") {
		t.Errorf("the refusal is not the constitution one: %v", err)
	}
}
