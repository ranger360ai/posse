//go:build posse_arm2

package posse

// ranger-base-9fl (from the ADR 0019 posture review, ranger-base-l8o): the
// L2 writable set granted the UNION of every built-in's state_dir to every
// persona, whatever runtime the launch was on. A claude session could write
// ~/.codex and ~/.grok — and a write there is not a config inconvenience,
// it is an integrity vector: swapping ~/.grok/auth.json for a token on an
// attacker-controlled account points the NEXT grok session on the box at
// that account. No file-read deny closes it, because the planting is a
// write; ADR 0019's credential read deny (ranger-base-hw18) had already
// narrowed the READ half to "every store but your own", and this is the
// same rule on the write side.
//
// Both arms are here on purpose, and each one kills a different mutant:
//
//   - re-adding the `for _, rt := range builtinRuntimes` union reds the
//     "not another runtime's" arm;
//   - dropping the `for _, d := range stateDirs` loop reds the "its own"
//     arm, which is also what keeps the fix from being "grant nothing" —
//     a runtime whose own state dir is read-only re-runs its first-run
//     flow every launch (ADR 0012 D4, seatbelt.go).
//
// The assertions compare RESOLVED elements of the writable slice rather
// than searching the profile text: on darwin the profile also carries
// `~/.codex/auth.json` and `~/.grok/auth.json` in the file-read deny block
// for a claude launch, so a `strings.Contains(prof, "~/.codex")` is true
// under both the defect and the fix — the shape that left the witness in
// statedirlaunch_qa_test.go measuring the read deny instead of the grant.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writableHas reports whether the set grants p as a subpath entry, matching
// the way the profile renders it — exact element, so `~/.claude` is not
// answered by `~/.claude.json` sitting beside it.
func writableHas(set []string, p string) bool {
	want := absResolve(ExpandTilde(p))
	for _, w := range set {
		if w == want {
			return true
		}
	}
	return false
}

func TestQASeatbeltGrantsOnlyTheLaunchingRuntimesStateDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a := checkApp(t)
	ag := &AgentFile{Name: "developer", MemoryDir: t.TempDir()}
	work := t.TempDir()

	all := map[string][]string{}
	for _, rt := range builtinRuntimes {
		all[rt.Name] = rt.StateDirs
	}
	if len(all) != 3 {
		t.Fatalf("this pin is written against the three built-ins; got %v", all)
	}

	for name, own := range all {
		set := a.SeatbeltWritable(ag, work, t.TempDir(), own...)
		for _, d := range own {
			if !writableHas(set, d) {
				t.Errorf("%s launch: its OWN state dir %s is not writable — a caged %s "+
					"re-runs its first-run flow every launch (ADR 0012 D4):\n%s",
					name, d, name, strings.Join(set, "\n"))
			}
		}
		for other, dirs := range all {
			if other == name {
				continue
			}
			for _, d := range dirs {
				if writableHas(set, d) {
					t.Errorf("%s launch is granted %s's state dir %s — write access to "+
						"another runtime's auth store is an exfil channel that outlives "+
						"the session that plants it (ranger-base-9fl):\n%s",
						name, other, d, strings.Join(set, "\n"))
				}
			}
		}
	}
}

// The caller that knows no runtime gets no runtime state at all. This is the
// claim ranger-base-9fl reverses — the old set was "exactly what the literal
// granted" for such a caller, which is the union this bead exists to remove —
// so it is asserted rather than left implied. Fail-closed is the direction:
// the cost of being wrong is a first-run flow D4 already names and an
// operator can see, and the cost the other way is a silent cross-runtime
// grant nobody reads off `posse gates`.
func TestQANoRuntimeInHandGrantsNoRuntimeStateDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a := checkApp(t)
	ag := &AgentFile{Name: "developer", MemoryDir: t.TempDir()}
	work := t.TempDir()

	set := a.SeatbeltWritable(ag, work, t.TempDir())
	// The witness that this set is real and the absences below can fail:
	// the session's own repo is granted whatever the runtime.
	if !writableHas(set, work) {
		t.Fatalf("the writable set does not even grant cwd — measuring the wrong thing:\n%s",
			strings.Join(set, "\n"))
	}
	for _, rt := range builtinRuntimes {
		for _, d := range rt.StateDirs {
			if writableHas(set, d) {
				t.Errorf("a caller with no runtime in hand was granted %s's %s:\n%s",
					rt.Name, d, strings.Join(set, "\n"))
			}
		}
	}
}

// The wiring, not the resolver: the profile a real launch writes to disk.
// A claude persona is the whole crew today, so this is the arm that says
// what actually changed on this box.
func TestQARenderedProfileForAClaudeLaunchCarriesNoOtherRuntimesStateDir(t *testing.T) {
	seatbeltForTest(t)
	b, _ := newTestBackend(t)
	a := b.App

	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.AgentsDir, "ranger.md"),
		[]byte("---\nname: ranger\ndescription: t\nruntime: claude\ncage: seatbelt\n---\nwork\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := b.planLaunch(NewSessionOpts{Name: "s1", Dir: t.TempDir(), Agent: "ranger"}); err != nil {
		t.Fatalf("planLaunch: %v", err)
	}
	prof, err := os.ReadFile(filepath.Join(a.GatesDir("ranger"), "seatbelt.sb"))
	if err != nil {
		t.Fatalf("the launch rendered no seatbelt profile: %v", err)
	}
	// The exact SBPL line the writable set renders as — not a bare path
	// search, which the credential read-deny block would answer for
	// ~/.codex and ~/.grok under the fix as well as under the defect.
	grant := func(p string) string {
		return "  (subpath " + sbQuote(absResolve(ExpandTilde(p))) + ")"
	}
	for _, d := range []string{"~/.claude", "~/.claude.json"} {
		if !strings.Contains(string(prof), grant(d)) {
			t.Errorf("a claude launch's profile does not grant claude's own %s:\n%s", d, prof)
		}
	}
	for _, d := range []string{"~/.codex", "~/.grok"} {
		if strings.Contains(string(prof), grant(d)) {
			t.Errorf("a claude launch's profile grants %s write access — the cross-runtime "+
				"auth-store write ranger-base-9fl closes:\n%s", d, prof)
		}
	}
}
