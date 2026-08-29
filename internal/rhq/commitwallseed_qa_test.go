package rhq

// ranger-base-09b7: the L1 half of the commit wall never reached the SEED.
//
// rangerhq-jgod put `Bash(git commit unless --)` in the eight PIDs the
// operator hand-edited in RHQ_HOME/agents/. It never reached
// examples/agents/, which is what //go:embed all:examples carries into every
// release binary and what `posse init` copies into a fresh instance
// (embed.go, init.go's seedSource). So the wall stood on the crew's own
// files and on no PID anyone created from what the binary ships: a new
// instance got L3 (herdrback installs the hook at every session create) and
// no L1 — and jgod's own reason for wanting both is that L1 lands on the
// TYPED LINE, on every runtime, before git runs, and reaches repos where no
// hook is installed.
//
// So this pin reads the EMBED, not `../../examples`. In a checkout the two
// are the same bytes and the existing seed pins (TestShippedPIDsDenyPromote,
// TestShippedPIDsDenyRefresh) read the directory; posse.Seed is what a
// release binary actually has, and the embed is the thing the gap was in.
//
// It names the file it is unhappy about, per PID rather than once: a
// ban-list that says "the corpus is clean" says nothing about the one file
// it never read. Delete the rule from ONE seed PID and this test names that
// PID (mutation-checked per file, ranger-base-09b7).

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse"
)

// TestSeededPIDsCarryTheL1CommitWall is the corpus pin.
func TestSeededPIDsCarryTheL1CommitWall(t *testing.T) {
	names := exampleAgentNames(posse.Seed)
	if len(names) < 9 {
		t.Fatalf("the seed ships %d example PIDs (%v) — a corpus pin over a corpus this small is measuring nothing", len(names), names)
	}
	// LoadAgent is the production parser and it reads a directory, so the
	// embed is materialized the way `posse init` materializes it. Reading
	// the frontmatter by hand here would pin this test's parser, not posse's.
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents")}
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, n := range names {
		rel := path.Join("agents", n+".md")
		b, err := fs.ReadFile(posse.Seed, rel)
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		if err := os.WriteFile(filepath.Join(a.AgentsDir, n+".md"), b, 0o644); err != nil {
			t.Fatal(err)
		}
		ag, err := a.LoadAgent(n)
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		// The exemption, and the only one: a PID that denies Bash outright
		// runs no shell verb, so there is no typed line for L1 to land on.
		// Every PID the seed ships today grants Bash, so this arm is a
		// statement about the predicate, not a way out.
		if denies(ag.Deny, "Bash") {
			continue
		}
		checked++
		if !denies(ag.Deny, "Bash(git commit unless --)") {
			t.Errorf(`examples/%s does not deny Bash(git commit unless --).

That PID ships in every release binary and is what `+"`posse init`"+` seeds a
fresh instance from, so an operator who adopts it gets the L3 hook and no L1
half: no refusal on the typed line, and nothing at all in a repo where the
hook was never installed. deny: %v`, rel, ag.Deny)
			continue
		}
		// A rule L1 cannot realize as a NEGATIVE match is the wall spelled
		// as prose — and worse than absent, because parity would report the
		// layer as present. deniesUnqualifiedCommit is the function parity
		// itself reads, so this asks production's question.
		if !deniesUnqualifiedCommit(ag.Deny) {
			t.Errorf("examples/%s: the commit rule does not parse as a negative match — L1 renders nothing for it", rel)
		}
		// And nothing else in the list may close the form the wall leaves
		// open. business-manager carried a blanket `Bash(git commit:*)`
		// until ranger-base-09b7: advisory by construction, but it also
		// forbade `git commit -- <paths>`, the one safe form.
		for _, d := range ag.Deny {
			if strings.HasPrefix(d, "Bash(git commit") && d != "Bash(git commit unless --)" {
				t.Errorf("examples/%s: %q also refuses the path-limited form — the wall leaves that one open on purpose", rel, d)
			}
		}
	}
	if checked < 9 {
		t.Errorf("only %d of %d seeded PIDs were checked — the exemption swallowed the corpus", checked, len(names))
	}
}

func denies(deny []string, rule string) bool {
	for _, d := range deny {
		if d == rule {
			return true
		}
	}
	return false
}
