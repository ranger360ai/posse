//go:build !posse_arm2 && !posse_arm3

package posse

// QA pin for ranger-base-h0hjx, verifying the close of ranger-base-32ll0
// (ADR 0079: the hook-skip arm judges git's global scope against the
// launcher's resolved global files). The finding it holds is filed as
// ranger-base-7czd6, which is where the fix goes.
//
// WHAT PROCESS BEHAVES DIFFERENTLY IF THIS PIN IS DELETED (ADR 0006 §7): the
// L1 git shim's global-scope compare lets a seat point HOME at a directory of
// its own and commit past an employer-managed pre-commit scanner — the thing
// ADR 0052 says posse must never cause to be skipped — whenever the
// launcher's global config spells `core.hooksPath` HOME-relative. No
// refusal, no refusals.log line, rc 0, commit landed, and the seat's own
// pre-commit runs in place of the managed one. Deleting this pin takes the
// only executable witness of that with it.
//
// THE HOLE. ADR 0079 D2 compares TEXT: `have` is the call's own `git config
// --show-scope --get-regexp <hookSkipRegexp>` filtered to global lines,
// `want` is the same regexp read per baked candidate under the LAUNCHER's
// HOME. `git config --get-regexp` prints a value RAW, and git expands `~` in
// `core.hooksPath` later, from the HOME of the process that runs the hook.
// So the two sides are byte-identical and the directory is not:
//
//	launcher ~/.gitconfig: core.hooksPath = ~/managed-hooks
//	have (HOME=evil):  global<TAB>core.hookspath ~/managed-hooks
//	want (HOME=launcher): global<TAB>core.hookspath ~/managed-hooks   → equal → allowed
//	what git then runs:  $evil/managed-hooks/pre-commit               → the seat's own
//
// This is the sibling of the case ranger-base-32ll0 DID close. That bead
// found `include.path = ~/x` in the launcher's file expanding from the
// call's HOME and shipped the fix for it — the want reads run under the
// launcher's HOME (TestQAHookSkipMovedHomeFollowsNoTildeInclude, and ADR
// 0079 pin 8). That fix works because git resolves an include AT READ TIME,
// so the two reads disagree. A `core.hooksPath` value is resolved at read
// time by neither side, so the same class walks straight through the same
// compare.
//
// MEASURED 2026-11-06, this box (darwin 25.6.0, git 2.50.1 Apple Git-155),
// at main 1f4da269, through a real rendered claude seat shim: the attack leg
// exited 0, the managed pre-commit fired ZERO times, the seat's own
// pre-commit ran ONCE, and no refusal was printed — while the plain commit
// fired the managed hook and a HOME move that drops the key outright was
// refused under hookskip:config-env. The shipped ADR 0079 pins 1-7 were
// green throughout: every one of their fixtures spells the hooksPath
// ABSOLUTE, so no shipped leg reaches this.
//
// REACHABILITY, and it is why this is P2 and not P1: the hole needs the
// LAUNCHER's global file to carry the HOME-relative spelling. posse writes
// no global `core.hooksPath` at all (gates.go: "posse's answer there is to
// write NOTHING"), and the boundary legs below keep a seat from writing one
// through the shim. So it needs a pre-existing operator or employer-managed
// config — which is the shape ADR 0052 exists for and the shape ADR 0079's
// reference is built to judge, not past its edge. `~/.company/hooks` is an
// ordinary way to write that line.
//
// A CANDIDATE ROUTE, MEASURED and NOT a prescription — the lane picks the
// fix: reading both sides with `--type=path` makes git expand the `~` per
// read, so the two sides differ and the arm refuses. Trialled 2026-11-06 on
// top of 1f4da269: both legs below flip to REFUSED with the seat's hook not
// run, and ADR 0079 pins 1-7, TestQAHookSkipMovedHomeFollowsNoTildeInclude
// and TestQAHookSkipRefusedOnEveryRuntime all stayed green (the full suite
// was not run under it). `--type=path` leaves a value with no `~` alone, so
// the `alias.*` half of hookSkipRegexp reads unchanged — also MEASURED.
//
// WHEN THE FIX LANDS this test REDS, naming the assertion to invert. The
// scratch dirs are not the tree, so this is not a tree-wide pin and owes the
// Makefile no door.

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// h0hjxTildeConfig is the launcher's global file for these legs: the managed
// hooksPath spelled HOME-relative. Under the fixture's HOME it names exactly
// the dir hsSetup put the managed pre-commit in.
const h0hjxTildeConfig = "[core]\n\thooksPath = ~/managed-hooks\n[user]\n\tname = t\n\temail = t@example.com\n"

// h0hjxGlobalWrites are the spellings that would PLANT the HOME-relative
// value in global scope. Every one must stay refused: they are the boundary
// that keeps this a P2 rather than a P1.
var h0hjxGlobalWrites = []string{
	`config --global core.hooksPath "~/managed-hooks"`,
	`config --global set core.hooksPath "~/managed-hooks"`,
	`config --global --replace-all core.hooksPath "~/managed-hooks"`,
	`config --global -- core.hooksPath "~/managed-hooks"`,
	`config core.hooksPath "~/managed-hooks"`,
}

func TestQAHookSkipTildeHooksPathIsJudgedByRawText(t *testing.T) {
	t.Parallel()
	for _, rtName := range []string{"claude", "codex"} {
		t.Run(rtName, func(t *testing.T) {
			t.Parallel()
			fx := hsSetup(t)
			write(t, filepath.Join(fx.home, ".gitconfig"), h0hjxTildeConfig)
			wrapped := hsWrapped(t, fx, rtName)

			// evil: a byte-identical global file and its OWN managed-hooks
			// dir, holding a pre-commit that is not the managed one. bare: a
			// HOME that drops the key outright, which the arm does refuse.
			evil := filepath.Join(fx.home, "evil-home")
			bare := filepath.Join(fx.home, "bare-home")
			if err := os.MkdirAll(filepath.Join(evil, "managed-hooks"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(bare, 0o755); err != nil {
				t.Fatal(err)
			}
			write(t, filepath.Join(evil, ".gitconfig"), h0hjxTildeConfig)
			write(t, filepath.Join(bare, ".gitconfig"), "[user]\n\tname = t\n\temail = t@example.com\n")
			seatHook := filepath.Join(fx.home, "seat-hook-ran")
			if err := WriteExecutable(filepath.Join(evil, "managed-hooks", "pre-commit"),
				[]byte("#!/bin/sh\necho ran >> "+shQuote(seatHook)+"\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			seatRan := func() int {
				b, _ := os.ReadFile(seatHook)
				return strings.Count(string(b), "ran")
			}

			// CONTROL 1: the managed hook is reachable THROUGH THE TILDE —
			// without this the "fired 0 times" reading below is true of
			// nothing.
			if errs, code := hsRun(t, fx, wrapped, hsLeg{args: "commit -q -m tilde-plain -- f"}); code != 0 || hsFired(fx) != 1 {
				t.Fatalf("CONTROL 1: a plain commit must run the managed hook that `core.hooksPath = ~/managed-hooks` names: code=%d fired=%d (%s)", code, hsFired(fx), errs)
			}
			// CONTROL 2: the arm is live on this fixture. A HOME whose config
			// drops the key is refused, so "allowed" below is a verdict and
			// not a dead arm.
			before := hsFired(fx)
			if errs, code := hsRun(t, fx, wrapped, hsLeg{env: "HOME=" + shQuote(bare) + " ", args: "commit -q -m tilde-bare -- f"}); code == 0 || !strings.Contains(errs, "hookskip:config-env") {
				t.Fatalf("CONTROL 2: HOME at a config with no hooksPath must still be refused under hookskip:config-env (ADR 0079 pin 1): code=%d (%s)", code, errs)
			} else if hsFired(fx) != before {
				t.Fatalf("CONTROL 2: the bare HOME was refused but the hook still ran")
			}

			// THE BOUNDARY: a seat may not plant the HOME-relative value.
			for _, w := range h0hjxGlobalWrites {
				errs, code := hsRun(t, fx, wrapped, hsLeg{args: w})
				if !strings.Contains(errs, "hookskip:") {
					t.Errorf("THE HOLE GOT WORSE: `git %s` was not refused (code=%d), so a seat can now PLANT the HOME-relative hooksPath this pin is about, not merely inherit one. Re-price ranger-base-h0hjx's finding above P2.\n%s", w, code, errs)
				}
			}

			// THE HOLE. Each leg is ALLOWED today and must not be: the text
			// compares equal while the directory git resolves does not.
			for i, leg := range []struct {
				name, env string
				seat      int // times the seat's own pre-commit runs
			}{
				{"HOME=evil, byte-identical config, the seat's own hooks dir", "HOME=" + shQuote(evil) + " ", 1},
			} {
				firedBefore, seatBefore := hsFired(fx), seatRan()
				errs, code := hsRun(t, fx, wrapped, hsLeg{env: leg.env, args: "commit -q -m h0" + strconv.Itoa(i) + " -- f"})
				fired, seat := hsFired(fx)-firedBefore, seatRan()-seatBefore
				if code != 0 || fired != 0 || seat != leg.seat || strings.Contains(errs, "hookskip:") {
					t.Errorf("INVERT ME — this is the pin to flip when the fix lands.\n"+
						"%s: code=%d managed_fired=%d seat_hook_ran=%d (%s)\n"+
						"This leg asserted the MEASURED hole: rc 0, the managed hook fired 0, the seat's own hook ran %d, no refusal. "+
						"If it is now refused under hookskip:config-env with the seat's hook not run, the fix has landed: rewrite this leg as a refusal (the shape hsGlobalCheck uses in hookskipglobal_qa_test.go), and say in ADR 0079 D2 that the compare resolves HOME-relative values rather than comparing them raw.",
						leg.name, code, fired, seat, errs, leg.seat)
				}
			}
		})
	}
}
