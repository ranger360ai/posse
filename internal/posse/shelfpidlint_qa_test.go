//go:build !posse_arm2 && !posse_arm3

package posse

// TestShelfPIDsLintCleanAsASet, moved out of pidcheck_test.go by
// ranger-base-nt0pi. IT IS HERE FOR ITS ARM. The pin's subject is the
// SHELF — examples/agents, and the examples/skills the seeded home needs —
// so a bead that adds, retires or edits a reference PID or a shipped skill
// reds it, and no seat's `-run` filter has ever named it. That makes it a
// tree-wide pin, which needs a Makefile door ($(QA_SEED_PINS),
// `make seed-check`), and that door's `-run` runs with NO `-tags`
// (TestQAEveryMakefileDoorPinIsReachableInItsDoorsArm). In pidcheck_test.go
// it was behind `//go:build posse_arm2`, where a door could not reach it at
// all: the filter would have matched nothing and `go test -run` exits 0 over
// nothing. So the pin moves to arm 1 — the default build — rather than
// seed-check learning to pass a tag.
//
// A TAGGED DOOR EXISTS NOW (ranger-base-na4rm), and this file still stands.
// The mechanism arrived for a different reason than this pin had: fifteen
// one-package censuses were admitted to the class at once and seven of them
// carry an arm tag, so the alternative there was moving seven files to suit
// their doors. Here it was one file, and the door it belongs behind —
// seed-check, whose subject is the published seed surface, which is this
// pin's subject — runs the default build for the other seven pins in it.
// Moving the file was the smaller change then and is the smaller change now;
// teaching seed-check a tag would split its membership across two arms to
// save one `git mv`.
//
// Nothing else in pidcheck_test.go moved: the rest of that file is the
// checker's own unit tests, which read a planted home and are nobody's
// tree-wide anything.
//
// Its root comes from qibRepoRoot for the reason extdiff_qa_test.go's does:
// the class is derived from that one helper, and the `../../examples/skills`
// climb this test used to spell reached the same files invisibly.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The checker must stay in step with the fleet it ships with (rangerhq-h44:
// `agent check` reported `missing section ## Role` against PIDs that say
// `## Who you are`, and `metrics: unknown id` for a persona's own ids —
// both since ruled on by the ADR 0001 amendment, neither pinned as a fact
// about the *shelf*). So: every examples/agents PID, in one home posse init
// has seeded, lints clean. A one-file pin (TestCheckAgent, architect.md
// alone) cannot see a finding computed across personas — the derived
// catalog's near-duplicate check is one — and a headings-only pin
// (TestExampleAgentsArePIDs) cannot see anything the linter checks that is
// not a heading.
func TestShelfPIDsLintCleanAsASet(t *testing.T) {
	t.Parallel()
	shelf := shelfPIDs(t, qibRepoRoot(t))
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range shelf {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(a.AgentsDir, filepath.Base(p)), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// The shelf declares skills: [distributed-systems]; `posse init` seeds
	// examples/skills into the home, so "clean" is judged in a seeded home.
	seeded, _ := filepath.Glob(filepath.Join(qibRepoRoot(t), "examples", "skills", "*"))
	for _, p := range seeded {
		mkSkill(t, a.SkillsDir(), filepath.Base(p))
	}

	// Positive witness: a green loop over an empty set proves nothing.
	names := a.ListAgents()
	if len(names) != len(shelf) {
		t.Fatalf("seeded %d shelf PIDs, ListAgents sees %d: %v", len(shelf), len(names), names)
	}
	for _, n := range names {
		fs, _, err := a.CheckAgent(n)
		if err != nil {
			t.Errorf("%s: %v", n, err)
			continue
		}
		if len(fs) != 0 {
			t.Errorf("%s: shelf PID has findings: %v", n, fs)
		}
	}

	// Control: put one shelf PID back on the ADR's first-draft heading and
	// the lint must name it. Without this the loop above is green whether
	// CheckAgent looked at the bodies or not.
	raw, err := os.ReadFile(filepath.Join(a.AgentsDir, "developer.md"))
	if err != nil {
		t.Fatal(err)
	}
	drifted := strings.Replace(string(raw), "\n## Who you are\n", "\n## Role\n", 1)
	if drifted == string(raw) {
		t.Fatal("control did not plant: developer.md has no ## Who you are heading")
	}
	if err := os.WriteFile(filepath.Join(a.AgentsDir, "developer.md"), []byte(drifted), 0o644); err != nil {
		t.Fatal(err)
	}
	fs, _, err := a.CheckAgent("developer")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(fs, "\n"), "missing section ## Who you are") {
		t.Errorf("control: a drifted identity heading must be a finding, got %v", fs)
	}
}
