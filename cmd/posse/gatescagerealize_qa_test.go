package main

// QA pin for ranger-base-1155n: `posse gates <persona>` prints ONE parity
// block per runtime where `cage_realize_<goos>:` collapses the two it would
// otherwise print (ADR 0083 D7).
//
// WHY THERE IS A PIN ON A PRINT. This surface prints two blocks per runtime —
// the default tier, and the seatbelt tier where this build provides it —
// because an operator reads "what would this PID get if I raised the cage"
// off the second one. Once the cage map realizes seatbelt as the tier the
// first block already reported, the two are byte-identical, and two identical
// blocks on the one surface an operator reads the wall off is a worse answer
// than one block and a line saying why. Nothing else would have caught it:
// the in-package matrix is correct either way, and both blocks are green.
//
// Run as the BINARY, like the other gates pins here, because the duplication
// is a property of the call site's two calls and not of the matrix.
//
// MEASURED BY HOST, and deliberately: the second block only ever prints where
// `AvailableCages` has the seatbelt tier, so there is a duplicate to collapse
// on darwin and nothing to collapse on a Linux host — which is the host D7 is
// written for. The arms below say which is which rather than asserting one
// shape on both.

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

// gcrHome is a home with one PID whose every rule holds at shims and whose
// `cage:` line demands seatbelt — so the map lowers it with nothing refusing,
// and the blocks below differ only in which tier they were asked about.
func gcrHome(t *testing.T, cfg string) string {
	t.Helper()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "agents", "light.md"),
		[]byte("---\nname: light\nlabels: [doc]\ncage: seatbelt\ndeny: [Bash(git push:*)]\n---\nYou are light.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return home
}

// gcrGates runs `posse gates light` against home. The herdr socket is pinned
// at a path that is not there: this reads a PID, and a reader that reached the
// operator's live socket would be red per-box.
func gcrGates(t *testing.T, bin, home string) string {
	t.Helper()
	cwd := t.TempDir()
	cmd := exec.Command(bin, "gates", "light")
	cmd.Dir = cwd
	cmd.Env = []string{
		"HOME=" + t.TempDir(),
		"RHQ_HOME=" + home,
		"RHQ_PERSONA=",
		"HERDR_SOCKET_PATH=" + filepath.Join(cwd, "no-such.sock"),
		"PATH=" + os.Getenv("PATH"),
	}
	out, _ := cmd.CombinedOutput() // a missing herdr is expected; the blocks still print
	return string(out)
}

func TestQAGatesPrintsOneBlockWhereTheCageMapCollapsesTwo(t *testing.T) {
	t.Parallel()
	if !posse.AvailableCages[posse.CageSeatbelt] {
		t.Skip("NOTHING MEASURED: this build provides no seatbelt tier, so `posse gates` prints one block whatever the map says — there is no duplicate to collapse on this host")
	}
	bin := buildRhq(t)
	const collapsed = "no separate seatbelt block"

	// The control first, so the arm below is a CHANGE and not a shape this
	// surface always had: with no map, the two blocks are two tiers.
	plain := gcrGates(t, bin, gcrHome(t, ""))
	if n := strings.Count(plain, "claude @ "); n != 2 {
		t.Fatalf("with no map this surface prints the default tier and the seatbelt tier: %d claude block(s)\n%s", n, plain)
	}
	if !strings.Contains(plain, "claude @ seatbelt/") || strings.Contains(plain, collapsed) {
		t.Errorf("with no map the seatbelt block must be printed, at the seatbelt tier:\n%s", plain)
	}

	mapped := gcrGates(t, bin, gcrHome(t, posse.CageRealizeKey(runtime.GOOS)+":\n  "+posse.CageSeatbelt+": "+posse.CageShims+"\n"))
	if n := strings.Count(mapped, "claude @ "); n != 1 {
		t.Errorf("the map realizes seatbelt as shims, so the two blocks are one: %d claude block(s)\n%s", n, mapped)
	}
	if strings.Contains(mapped, "claude @ seatbelt/") {
		t.Errorf("no block may claim the seatbelt tier on a host that realizes it as shims:\n%s", mapped)
	}
	// Said out loud, not swallowed: the absent block has a reason and the
	// reason names the key.
	if !strings.Contains(mapped, collapsed) || !strings.Contains(mapped, "cage_realize:") {
		t.Errorf("the collapsed block must say why, naming the key:\n%s", mapped)
	}
	// And the waiver itself is still on the surface — collapsing the second
	// block must not take the row that says the cage was lowered.
	if !strings.Contains(mapped, "⚖ cage_realize: "+runtime.GOOS+" seatbelt→shims") {
		t.Errorf("the ratified row must still print:\n%s", mapped)
	}
}
