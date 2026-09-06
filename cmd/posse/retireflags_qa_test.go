package main

// ranger-base-iz8fx: ADR 0058 D3's flag grammar, measured where an operator
// meets it — the built binary's argv, not the parser's source.
//
// `posse worktrees --retire` runs the landing sweep's own destroy predicate
// over every tree on the board, and the one thing it must never grow is a
// `--force`. force is RemoveSessionTree's existing override: it stands down
// the single refusal that exists to say no while something would be lost,
// and D3 leaves it where it is — the two-command hand recipe those refusals
// print, typed at ONE tree by a human who has just read why. A flag that
// waives it over 70 trees in one keystroke is the shape this whole record
// was written to avoid.
//
// So it is refused, and refused as an UNKNOWN FLAG rather than accepted and
// quietly ignored, which is the difference this file pins. An ignored
// `--force` exits 0 and retires whatever the unforced predicate allows — the
// operator's word for "override the guard" answered by a run that looked
// like it obeyed. The refusal is the same line an unrecognized flag gets,
// because under `[--land [--force] | --retire]` that is exactly what it is.
//
// The CONTROL arms are what make the refusal mean anything: `--land --force`
// is the same two flags in the combination that IS legal, and a bare
// `--retire` is the verb by itself. A parser that refused everything would
// pass the refusal arm alone.
//
// HOW TO MUTATION-CHECK THIS FILE, because the obvious way measures nothing.
// `go test -overlay` swaps files at COMPILE time for the package under test,
// and this pin does not compile the parser — it runs `go build` (buildRhq)
// and executes the binary, which reads main.go off the real tree and never
// sees the overlay. MEASURED 2026-09-06: the mutant that deletes the
// `retire && (force || land)` refusal outright passes this file green under
// -overlay and reds it in 1.5s when main.go is edited in the worktree
// instead. Mutate the file, run, restore.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runWorktrees runs `posse worktrees ...` against an empty scratch instance
// — no beads dirs, so nothing is listed, nothing is locked and nothing can
// be destroyed. What is under test is the argv gate, which runs before any
// of that.
func runWorktrees(t *testing.T, bin, home string, args ...string) (int, string) {
	t.Helper()
	cmd := exec.Command(bin, append([]string{"worktrees"}, args...)...)
	cmd.Env = append(os.Environ(),
		"RHQ_HOME="+home,
		"RHQ_HERDR_BIN="+filepath.Join(home, "herdr-must-not-run"),
	)
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("posse worktrees %s: %v", strings.Join(args, " "), err)
	}
	return code, string(out)
}

func TestRetireTakesNoForceAndSaysSoLikeAnUnknownFlag(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte("beads: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// The refusal, in both orders: a gate that only read the flags left to
	// right would take `--force --retire` and refuse its mirror.
	for _, args := range [][]string{
		{"--retire", "--force"},
		{"--force", "--retire"},
		{"--retire", "--land"},
		{"--retire", "--land", "--force"},
	} {
		code, out := runWorktrees(t, bin, home, args...)
		if code == 0 {
			t.Errorf("posse worktrees %s exited 0 — the flag was accepted:\n%s", strings.Join(args, " "), out)
		}
		if !strings.Contains(out, "posse worktrees [--dir <repo>] [--land [--force] | --retire]") {
			t.Errorf("posse worktrees %s did not answer with the usage line an unknown flag gets:\n%s",
				strings.Join(args, " "), out)
		}
	}

	// THE CONTROLS. Without them the arms above are satisfied by a command
	// that refuses everything, which is not what D3 asked for.
	for _, args := range [][]string{
		{},
		{"--retire"},
		{"--land"},
		{"--land", "--force"},
	} {
		code, out := runWorktrees(t, bin, home, args...)
		if code != 0 {
			t.Errorf("posse worktrees %s exited %d, want 0:\n%s", strings.Join(args, " "), code, out)
		}
		if !strings.Contains(out, "no session worktrees") {
			t.Errorf("posse worktrees %s did not run over the (empty) board:\n%s", strings.Join(args, " "), out)
		}
	}

	// A genuinely unknown flag gets the same line, which is what "refused as
	// unknown" means: the two are indistinguishable to the operator because
	// they are the same refusal.
	code, out := runWorktrees(t, bin, home, "--nonsense")
	if code == 0 || !strings.Contains(out, "posse worktrees [--dir <repo>] [--land [--force] | --retire]") {
		t.Errorf("an unknown flag does not get the usage line the paired flags get (exit %d):\n%s", code, out)
	}
}

// The catalog is where an operator finds the verb at all, and a flag no help
// text names is a flag nobody runs — which is the exact failure ADR 0058 was
// written about ("a human can retire the tree", printed for two weeks at
// nobody).
func TestHelpNamesRetireUnderWorktrees(t *testing.T) {
	out := helpText(t)
	// commandOf reports the whole catalog header, which for this verb IS
	// the grammar — `posse worktrees` carries its alternatives on the header
	// line rather than in a second column, so the assertion is that the
	// `--retire` paragraph hangs under that header and no other.
	const header = "posse worktrees [--dir <repo>] [--land [--force] | --retire]"
	if got := commandOf(t, out, "--retire "); got != header {
		t.Errorf("--retire is documented under %q, want %q", got, header)
	}
	if !strings.Contains(out, header) {
		t.Errorf("the catalog's worktrees header does not carry the grammar the parser enforces:\n%s", out)
	}
}
