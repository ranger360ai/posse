package posse

// rangerhq-o7y4. Two halves of one rule, ADR 0012 D2 ("persona names become
// roles", no code/prose carve-out).
//
// THE LEAK: the seed shipped nine reference PIDs of which one was named after
// this instance — examples/agents/ranger.md, "You are Ranger, the terse
// operations copilot …" — so `posse init` handed every fresh deployer a
// persona carrying the development instance's own name. rangerhq-dh5g took
// the crew brand out of the identity line of all nine and left the persona
// name; this bead renames the file to agents/ops.md and drops the name.
//
// THE COST OF THE RENAME, and the reason for the second test: retirement
// (retireExamplePIDs, init.go) used to walk the names in the running binary's
// seed. Under that walk a rename is a silent leak — a home seeded by any
// earlier posse holds agents/ranger.md, and a name that has left the embed is
// a name init never asks about again, so the generic keeps taking beads in
// label routing forever. That is ranger-base-8ehw arriving by another door.
// The walk is the union with the digest table's keys now, and this pins it.
//
// Self-contained (own helpers, own fixtures): a pin an edit next door can
// carry away pinned nothing.

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ranger360ai/posse"
)

// personalNameRe is the half of identityLineRe (pidcheck.go) that a reference
// PID must NOT use: "You are <Name>, the …". The optional name is legitimate
// in an instance's own crew and is exactly what the seed may not ship.
var personalNameRe = regexp.MustCompile(`^You are [^,]+, the `)

// TestNoShippedExamplePIDCarriesAPersonalName is the leak itself. Every
// example ships the no-name form, which is the form pidcheck.go's own worked
// example uses.
func TestNoShippedExamplePIDCarriesAPersonalName(t *testing.T) {
	t.Parallel()
	names := exampleAgentNames(posse.Seed)
	if len(names) < 9 {
		t.Fatalf("the seed ships %d example PID(s) — this pin has nothing to read", len(names))
	}
	for _, n := range names {
		b, err := fs.ReadFile(posse.Seed, "agents/"+n+".md")
		if err != nil {
			t.Fatal(err)
		}
		_, body := agentFrontmatter(string(b))
		line := identityLine(body)
		if !identityLineRe.MatchString(line) {
			t.Errorf("agents/%s.md: identity line %q is not a PID identity line", n, line)
		}
		if personalNameRe.MatchString(line) {
			t.Errorf(`agents/%s.md opens %q — a shipped example may not name a person.
ADR 0012 D2: persona names become roles, and it carries no code/prose
carve-out. A deployer copies this file as-is (INSTALL.md §7), so a name here
is the development instance's name on their crew. Use the no-name form
pidcheck.go documents: "You are the <role> of the crew."`, n, line)
		}
	}
}

// TestTheSeedShipsNoPersonaNamedAfterThisInstance is the specific fix, kept
// separate because the rule above would also be satisfied by a file still
// called ranger.md whose identity line lost the name. The filename IS the
// persona name (agents.go: ag.Name is the directory entry).
func TestTheSeedShipsNoPersonaNamedAfterThisInstance(t *testing.T) {
	for _, n := range exampleAgentNames(posse.Seed) {
		if n == "ranger" {
			t.Errorf(`the seed ships agents/ranger.md again — that is the development
instance's own name, the one its org and its tracker prefixes are built from,
and rangerhq-o7y4 renamed it to agents/ops.md. ADR 0012 D2: persona names
become roles.`)
		}
	}
	if _, ok := shippedExampleDigests["agents/ranger.md"]; !ok {
		t.Error(`shippedExampleDigests lost "agents/ranger.md". Renaming an example does not
retire the old key: homes seeded by every release before rangerhq-o7y4 hold
that file, and the key is how init recognises and retires it (exampledigests.go,
and TestARetiredExampleNameIsStillRetiredOnUpgrade below).`)
	}
	if _, ok := shippedExampleDigests["agents/ops.md"]; !ok {
		t.Error(`shippedExampleDigests has no "agents/ops.md" — the renamed example is
unrecognisable, so no home ever retires it`)
	}
}

// shipAsAReleaseO7y4 enters bytes in posse's record of what posse has shipped
// under rel, the way a real release is entered — by appending. The real
// entries are never disturbed and the table is restored after.
func shipAsAReleaseO7y4(t *testing.T, rel string, b []byte) {
	t.Helper()
	was := shippedExampleDigests[rel]
	t.Cleanup(func() {
		if was == nil {
			delete(shippedExampleDigests, rel)
			return
		}
		shippedExampleDigests[rel] = was
	})
	shippedExampleDigests[rel] = append(append([]string{}, was...), sha256Bytes(b))
}

// retiredNameHome is a home as an older posse left it: seeded, plus one
// generic live in agents/ under a name THIS seed no longer ships. gonePID is
// that file's bytes, entered as a release.
func retiredNameHome(t *testing.T, name string, gonePID []byte) *App {
	t.Helper()
	// Hermetic against the operator fence (ADR 0031 §2): see initTestApp.
	t.Setenv(EnvPersona, "")
	a := NewAppAt(filepath.Join(t.TempDir(), "home"))
	if err := a.initFrom(io.Discard, posse.Seed, "embedded"); err != nil {
		t.Fatalf("seeding: %v", err)
	}
	if live := a.ListAgents(); len(live) != 0 {
		t.Fatalf("fixture: a fresh init left %v in agents/, so this home is not the shape under test", live)
	}
	shipAsAReleaseO7y4(t, "agents/"+name+".md", gonePID)
	if err := os.WriteFile(filepath.Join(a.AgentsDir, name+".md"), gonePID, 0o644); err != nil {
		t.Fatal(err)
	}
	// The fixture is only a fixture if the seed really has stopped shipping
	// the name — otherwise this measures the ordinary path.
	for _, n := range exampleAgentNames(posse.Seed) {
		if n == name {
			t.Fatalf("fixture: the seed still ships agents/%s.md — pick a name it does not", name)
		}
	}
	return a
}

// TestARetiredExampleNameIsStillRetiredOnUpgrade: the rename cost, pinned.
// Collapse retirableExampleNames back to exampleAgentNames and this fails
// with the generic still in agents/.
func TestARetiredExampleNameIsStillRetiredOnUpgrade(t *testing.T) {
	gone := []byte("---\nname: gone\ndescription: a generic an older posse shipped\n---\nYou are the gone engineer of the crew.\n")
	a := retiredNameHome(t, "gone", gone)

	var out strings.Builder
	if err := a.initFrom(&out, posse.Seed, "embedded"); err != nil {
		t.Fatalf("upgrade init: %v", err)
	}
	if live := a.ListAgents(); len(live) != 0 {
		t.Fatalf(`a generic posse shipped under a name it no longer ships is still routing
after an upgrade: %v. retireExamplePIDs must walk every name posse has
shipped an example for (retirableExampleNames), not the embed alone — a
rename otherwise strands the old file on every home that has it.
init said:
%s`, live, out.String())
	}
	if !strings.Contains(out.String(), "retired") || !strings.Contains(out.String(), "gone") {
		t.Errorf("init said %q — a retirement the operator cannot see is a file that vanished", out.String())
	}
	// A move, not a delete. These bytes exist nowhere else in the home and
	// nowhere in the embed, so the shelf is the only place they can survive.
	shelf, err := os.ReadFile(filepath.Join(a.ExampleAgentsDir(), "gone.md"))
	if err != nil {
		t.Fatalf("gone.md left the home entirely: %v", err)
	}
	if string(shelf) != string(gone) {
		t.Errorf("the shelf copy is not what left agents/:\n got %q\nwant %q", shelf, gone)
	}
}

// TestARetiredExampleNameDoesNotOverwriteTheOperatorsShelfCopy is the
// negative control, and it needs the positive witness above to mean anything:
// same fixture, same run, one thing different — the shelf slot is the
// operator's. Nothing moves, and their file is byte-unchanged.
func TestARetiredExampleNameDoesNotOverwriteTheOperatorsShelfCopy(t *testing.T) {
	gone := []byte("---\nname: gone\ndescription: a generic an older posse shipped\n---\nYou are the gone engineer of the crew.\n")
	mine := []byte("---\nname: gone\ndescription: mine now\n---\nYou are the gone engineer of the Vantage crew.\n")
	a := retiredNameHome(t, "gone", gone)
	shelf := filepath.Join(a.ExampleAgentsDir(), "gone.md")
	if err := os.WriteFile(shelf, mine, 0o644); err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	if err := a.initFrom(&out, posse.Seed, "embedded"); err != nil {
		t.Fatalf("upgrade init: %v", err)
	}
	if b, err := os.ReadFile(shelf); err != nil || string(b) != string(mine) {
		t.Fatalf("the operator's shelf copy was overwritten: %q, %v", b, err)
	}
	if live := a.ListAgents(); len(live) != 1 || live[0] != "gone" {
		t.Errorf("agents/ = %v, want gone.md kept — nothing may move onto a file init did not write", live)
	}
	if !strings.Contains(out.String(), "the shelf copy differs") {
		t.Errorf("init said %q — it must say why it kept the file", out.String())
	}
}
