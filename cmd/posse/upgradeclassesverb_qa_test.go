package main

// QA, ranger-base-e6yct — THE BINARY MUST ANSWER `posse upgrade --classes`,
// and that is a pin rather than a style note because of WHO CALLS IT.
//
// ADR 0092 D4 rules 4-6 (ranger-base-v2xj9) have `posse upgrade` ask the
// NEW binary for its own upgrade partition after the swap, so that a status
// row added inside the installed range is classified by the binary that
// prints it instead of by the table that predates it. The process doing the
// asking is the OLD one. So this flag is part of the interface every FUTURE
// binary owes the one installed today: drop it, and the answer reads as "not
// asked", every gained row the old table cannot name trips, and the install
// rolls back — correctly, by its own rule, and forever, because each
// rollback leaves the old binary in place to ask again. That is the shape
// MEASURED at 0e1cb6c0..65d90a5e (ranger-base-i9fq8), one LANE G18 row
// rolling back an install whose own target classed it install-subject.
//
// MEASURED 2026-11-21, this box (linux), at eb3f34fc — the commit before the
// flag existed: `posse upgrade --classes` prints NOTHING on stdout, prints
// `posse: unknown flag: --classes — usage: posse upgrade [<sha>]` on stderr,
// and exits 1. Identical under RHQ_PERSONA unset and under a persona no
// `install_delegate:` names, because the flag ladder refuses before D2's
// fence reads anything. Exit 1 is also what the OTHER cross-binary verb's
// absence answers (RenderProbeMark's header), which is why both readings are
// taken off a MARK on stdout and neither off an exit code — and why this pin
// asserts the mark on every line.
//
// It runs the REAL binary rather than reading main.go for the string: an arm
// that greps the source would stay green over a flag that is spelled in the
// ladder and prints nothing, which is precisely the answer that trips.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranger360ai/posse/internal/posse"
)

func TestQAUpgradeClassesFlagIsInTheBinary(t *testing.T) {
	bin := buildRhq(t)

	home := t.TempDir()
	cache := filepath.Join(t.TempDir(), "cache")

	// A persona no `install_delegate:` names, under a HOME with no config at
	// all — which is the arm D2's fence refuses hardest (an absent key
	// refuses every persona, upgradeFence's header). Rule 4 says this flag
	// needs no delegate, so the answer must come back anyway: a fence here
	// would be read as "did not answer" by the very install that put this
	// binary in place.
	cmd := exec.Command(bin, "upgrade", "--classes")
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"XDG_CACHE_HOME="+cache,
		"RHQ_HOME=", "POSSE_HOME=",
		"RHQ_PERSONA=nobody-this-shop-declares",
	)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("posse upgrade --classes: %v\n%s", err, stderr.String())
	}
	if code != 0 {
		t.Fatalf("`posse upgrade --classes` exited %d, want 0:\n%s\n%s", code, stdout, stderr.String())
	}

	// The whole answer, compared against the one renderer both sides read
	// (posse.UpgradeClassLines) rather than against a list spelled here: a
	// table this pin copied would disagree with the verb the first time a
	// G-row lands, and the direction it disagrees in is a red that names the
	// wrong subject.
	want := posse.UpgradeClassLines()
	if len(want) < 12 {
		t.Fatalf("UpgradeClassLines has only %d line(s) — the partition is empty or nearly so, and every claim below would be vacuous: %v", len(want), want)
	}
	got := strings.Split(strings.TrimRight(string(stdout), "\n"), "\n")
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("`posse upgrade --classes` answered %d line(s), want the %d of this binary's own table:\n got: %s\nwant: %s", len(got), len(want), strings.Join(got, "\n      "), strings.Join(want, "\n      "))
	}

	// Every line marked, asserted over the output and not over the slice
	// above: the mark is what the reader uses to tell this verb's answer
	// from an old binary's refusal, so a line that reached stdout without it
	// is a line that reader would skip.
	for _, ln := range got {
		if !strings.HasPrefix(ln, posse.UpgradeClassMark) {
			t.Errorf("a line of the answer does not open with %q, so `posse upgrade` would not read it as this verb's at all (ADR 0092 D4 rule 4): %q", posse.UpgradeClassMark, ln)
		}
	}

	// `and does nothing else` (D4 rule 4), in the two ways a subprocess can
	// show it: nothing on stderr, and no box-wide upgrade lock left behind.
	// The lock is the one side effect that would matter — the asker already
	// holds it, and flock(2) is per open file description, so a nested claim
	// is a deadlock and not a queue (lockUpgrade's header).
	if s := strings.TrimSpace(stderr.String()); s != "" {
		t.Errorf("`posse upgrade --classes` wrote to stderr, and a reader of this verb reads stdout only: %q", s)
	}
	lock := filepath.Join(cache, "posse", "upgrade.lock")
	if _, err := os.Stat(lock); err == nil {
		t.Errorf("`posse upgrade --classes` created %s — rule 4's answer takes no lock", lock)
	}

	// And the flag is named where someone would look for it: the verb's own
	// usage line, and the catalog `posse help` prints.
	for _, probe := range []struct {
		what string
		argv []string
	}{
		{"posse upgrade --help", []string{"upgrade", "--help"}},
		{"posse help", []string{"help"}},
	} {
		u := exec.Command(bin, probe.argv...)
		u.Env = append(os.Environ(), "HOME="+home, "XDG_CACHE_HOME="+cache)
		uout, _ := u.CombinedOutput()
		if !strings.Contains(string(uout), "--classes") {
			t.Errorf("`%s` does not name --classes:\n%s", probe.what, uout)
		}
	}
}
