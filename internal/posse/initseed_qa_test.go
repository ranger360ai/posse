package posse

// QA pins for the seed source (`posse init`, ADR 0012 D5), from the verify of
// rangerhq-hrg8. The claim itself — a binary with no repo beside it seeds a
// full instance from the embed — holds, and initembed_test.go pins it. What
// this file pins is the other half of that decision: WHICH seed the binary
// chose, and whether a wrong choice is allowed to look like success.
//
// Self-contained on purpose (own helpers, own fixtures): personas share this
// checkout, and a pin is worth nothing if the next edit to a neighbouring
// test file carries it away.

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse"
)

func seedQAHome(t *testing.T) *App {
	t.Helper()
	// The home named rather than read out of RHQ_HOME; the operator fence
	// is TestMain's now — see initTestApp (ranger-base-pj87l).
	return NewAppAt(filepath.Join(t.TempDir(), "home"))
}

// The reference PIDs a fresh public install actually receives. `posse init`
// succeeding says nothing about whether what it laid down is usable: an
// example an operator copies into agents/ is the first thing dispatch reads,
// and a contract finding in one of them is a broken persona that looked
// shipped. Measured live during the verify — a go-installed binary with no
// repo readable lays down 9 PIDs and `posse agent check --all` passes 9/9;
// this keeps that true.
//
// AMENDED by ranger-base-qajs: the nine land on the SHELF
// ($RHQ_HOME/examples/agents), not in agents/. The contract claim is
// unchanged and is checked against the shelf; what changed is that a fresh
// install ships no crew, which the companion pin below states directly.
func TestEmbeddedSeedShipsAContractValidCrew(t *testing.T) {
	t.Parallel()
	a := seedQAHome(t)
	if err := a.initFrom(io.Discard, posse.Seed, "embedded"); err != nil {
		t.Fatalf("init from the embed: %v", err)
	}
	shelf := &App{Home: a.Home, AgentsDir: a.ExampleAgentsDir()}
	names := shelf.ListAgents()
	if len(names) < 9 {
		t.Fatalf("the embedded seed laid down %d example PID(s) (%v) — a fresh install gets the reference shelf, not a stub", len(names), names)
	}
	for _, n := range names {
		findings, _, err := shelf.CheckAgent(n)
		if err != nil {
			t.Fatalf("CheckAgent(%s): %v", n, err)
		}
		if len(findings) > 0 {
			t.Errorf("example PID %s fails the PID contract: %v", n, findings)
		}
	}
}

// ranger-base-e6y. seedSource takes any directory named examples/ beside the
// binary as the seed on a bare stat, and copyDir swallows the read error for
// every root it copies — so a foreign examples/ (a project that happens to
// have bin/ and examples/, which is a common repo shape) wins over the embed
// and lays down a home with no crew, at exit 0. Re-running init repairs
// nothing: the hijack is still in place and nothing is ever overwritten.
func TestSeedOverrideThatIsNotASeedDoesNotHalfSeedSilently(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "bin")
	foreign := filepath.Join(tmp, "examples")
	for _, d := range []string{bin, foreign} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// A config.yaml and nothing else: enough to get past the first copy, so
	// the failure is silence rather than an error.
	if err := os.WriteFile(filepath.Join(foreign, "config.yaml"), []byte("default_dir: ~\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, from := seedSource(bin); from != "embedded" {
		t.Errorf("seedSource(%s) chose %s — a directory with no agents/, recipes/ or envs/ is not a seed tree, and the embed is right there", bin, from)
	}

	// Whichever arm is taken, a source that cannot supply a seed root must
	// not come back as success.
	a := seedQAHome(t)
	if err := a.initFrom(io.Discard, os.DirFS(foreign), foreign); err == nil {
		names := a.ListAgents()
		t.Errorf("init from %s returned success with %d persona(s) — a home with no crew is not an initialized instance", foreign, len(names))
	}
}

// ranger-base-n0d. The install docs must not send somebody who hit a seeding
// problem back to a repo checkout: that is the behaviour the embed removed
// (ADR 0012 D5), and the error string that row names cannot be printed by any
// build since 49e287f. README.md carried the twin of this until 83c3c10.
func TestInstallDocsDoNotPrescribeARepoCheckoutToSeed(t *testing.T) {
	t.Parallel()
	dead := []string{
		"not found next to this binary", // an error no build can emit
		"must be the repo build",        // the claim the embed falsified
	}
	for _, doc := range []string{"README.md", "INSTALL.md"} {
		b, err := os.ReadFile(filepath.Join("..", "..", doc))
		if err != nil {
			t.Fatalf("read %s: %v", doc, err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			for _, phrase := range dead {
				if strings.Contains(line, phrase) {
					t.Errorf("%s:%d says %q — `posse init` seeds from the embed with no repo present", doc, i+1, phrase)
				}
			}
		}
	}
}

// The other half of ranger-base-n0d, and the half a phrase list cannot see: a
// troubleshooting row may name no dead error at all and still send the reader
// to a checkout. Only the fix cells are read, and only inside the tables — the
// body legitimately names `./bin/posse-go init` when it explains which seed a
// dev build chooses.
func TestTroubleshootingFixesDoNotSendTheReaderToACheckout(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(filepath.Join("..", "..", "INSTALL.md"))
	if err != nil {
		t.Fatalf("read INSTALL.md: %v", err)
	}
	inTable := false
	for i, line := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(line, "|") {
			inTable = false
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) < 3 {
			continue
		}
		if !inTable {
			// The header row names the columns; everything under it until a
			// non-table line is a row whose last cell is the fix.
			inTable = strings.Contains(line, "| fix |")
			continue
		}
		fix := cells[len(cells)-1]
		for _, phrase := range []string{"bin/posse-go", "repo checkout", "the checkout"} {
			if strings.Contains(fix, phrase) {
				t.Errorf("INSTALL.md:%d prescribes %q as a fix — the embed made the checkout unnecessary (ADR 0012 D5)", i+1, phrase)
			}
		}
	}
}
