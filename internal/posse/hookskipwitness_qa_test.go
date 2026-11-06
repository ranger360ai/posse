//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-y9i3o, from ranger-base-pr6p8's findings 1-4
// (docs/notes.d/ranger-base-pr6p8.md §1): four conjuncts of
// renderHookSkipGlobalScope (ADR 0079 D2) that carried no witness. Each could
// be deleted with the whole shipped TestQAHookSkip filter staying green —
// MEASURED 2026-11-06 at main fd2f7e24 by mutation, ten arms.
//
// The wall they belong to is what keeps a seat from pointing HOME at a
// directory of its own and committing past an employer-managed pre-commit
// scanner (ADR 0052), so a conjunct nobody tests is a wall that can be
// refactored thinner in silence. None of the four is a known hole — see the
// bead and the notes fragment for the reachability reasoning, which is
// reasoning and not measurement — and these pins claim nothing about
// reachability. What each pin says is: delete this conjunct and this test
// reds.
//
// The scratch dirs are not the tree, so none of this is a tree-wide pin and
// it owes the Makefile no door.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// hsRCStub writes a stub git that prints NOTHING and returns a chosen rc for
// each of the compare's reads, so one read can fail while every other read
// agrees. Real git cannot be made to do that for the two rc checks below:
// the have listing and the want's `--system` read read the same files, and
// the only env difference between the sides is HOME/XDG_CONFIG_HOME, so a
// value real git dies on dies on both sides (finding 1's `~nosuchuser`) or
// shows up as a listing line one side cannot produce (an `include.path` is
// itself a key the have listing prints). A stub is the instrument that
// isolates "read N failed and nothing else did"; pin 7
// (TestQAHookSkipGlobalGateExecsNothingAsRendered) renders over one too.
//
//   - `var`: exit 1, so the reference falls back to git's documented
//     global-file rule and the candidate loop renders.
//   - any verb that is not `config`: exit 0, so the commit itself passes.
func hsRCStub(t *testing.T, dir string, haveRC, nameRC, sysRC, candRC int) string {
	t.Helper()
	stub := filepath.Join(dir, "git")
	rc := func(n int) string { return strconv.Itoa(n) }
	script := "#!/bin/sh\n" +
		"case \"$1\" in\n" +
		"  var) exit 1 ;;\n" +
		"  config) ;;\n" +
		"  *) exit 0 ;;\n" +
		"esac\n" +
		"case \" $* \" in\n" +
		"  *\" --system \"*) exit " + rc(sysRC) + " ;;\n" +
		"  *\" --file \"*) exit " + rc(candRC) + " ;;\n" +
		"  *\" --name-only \"*) exit " + rc(nameRC) + " ;;\n" +
		"esac\n" +
		"exit " + rc(haveRC) + "\n"
	if err := WriteExecutable(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return stub
}

// Findings 1 and 2 (ADR 0079 pin 13; hookskip.go, the have listing's rc
// check and the want SYSTEM read's rc check), plus the sibling
// candidate-read check that
// TestQAHookSkipUnexpandableTildeFailsClosed already witnesses, so the table
// reads as one claim: EVERY read the compare makes has its rc checked, and a
// read that fails refuses instead of contributing "no lines".
//
// 1 is "no match" and must not refuse; past it git failed and the scope is
// unjudgeable. The first row is the witness that the rest say something: with
// every read agreeing, the same moved HOME is ALLOWED, so each refusal below
// is the one failing read and not the moved HOME.
func TestQAHookSkipGlobalCompareChecksEveryReadRC(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	for _, c := range []struct {
		name                      string
		have, nameOnly, sys, cand int
		refused                   bool
	}{
		// The baseline: every read agrees, both sides read nothing, equal.
		{"every read agrees: a moved HOME is allowed", 0, 0, 1, 1, false},
		// FINDING 1 — hookskip.go, the have listing: `[ $? -le 1 ] || return 1`.
		{"the have listing's git failed", 128, 0, 1, 1, true},
		// FINDING 2 — the want SYSTEM read: `[ $? -le 1 ] || exit 1`.
		{"the want SYSTEM read's git failed", 0, 0, 128, 1, true},
		// The sibling at the want CANDIDATE read, for completeness.
		{"a want CANDIDATE read's git failed", 0, 0, 1, 128, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			home := filepath.Join(dir, "home")
			if err := os.MkdirAll(home, 0o755); err != nil {
				t.Fatal(err)
			}
			write(t, filepath.Join(home, ".gitconfig"), "[user]\n\tname = t\n")
			stub := hsRCStub(t, dir, c.have, c.nameOnly, c.sys, c.cand)
			var env []string
			for _, kv := range os.Environ() {
				if strings.HasPrefix(kv, "GIT_CONFIG") || strings.HasPrefix(kv, "RHQ_GATE_") || strings.HasPrefix(kv, "HOME=") || strings.HasPrefix(kv, "XDG_CONFIG_HOME=") {
					continue
				}
				env = append(env, kv)
			}
			env = append(env, "HOME="+home)
			ref := resolveHookSkipGlobalRef(stub, env)
			if len(ref.Cands) == 0 {
				t.Fatalf("the reference must carry candidates, or the candidate read never renders: %+v", ref)
			}
			shim := filepath.Join(dir, "shim")
			body := renderShim("hg-rc", "git", stub, filepath.Join(dir, "refusals.log"), "/bin/date", nil, nil, "", filepath.Join(dir, "hooks"), ref)
			if err := WriteExecutable(shim, []byte(body), 0o755); err != nil {
				t.Fatal(err)
			}
			// A moved HOME, so the D3 gate runs the compare (pin 7).
			cmd := exec.Command(shim, "commit", "-m", "x", "--", "f")
			cmd.Env = append(append([]string{}, env...), "HOME="+dir)
			var errb strings.Builder
			cmd.Stderr = &errb
			err := cmd.Run()
			code := 0
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			} else if err != nil {
				t.Fatal(err)
			}
			errs := errb.String()
			if c.refused {
				if code == 0 || !strings.Contains(errs, "rule: hookskip:config-env,") || !strings.Contains(errs, "ADR 0079") {
					t.Errorf("code=%d, want refused by hookskip:config-env naming ADR 0079: %q", code, errs)
				}
				return
			}
			if code != 0 || strings.Contains(errs, "hookskip:") {
				t.Errorf("code=%d, want 0 and no hookskip refusal (%q)", code, errs)
			}
		})
	}
}

// FINDING 3 (ADR 0079 pin 14) — hookskip.go, `--includes` on the want SYSTEM
// read. `--system` names one file, so git defaults includes OFF there,
// unlike the have
// listing, which reads every file and defaults them ON. Drop the flag and the
// want side reads LESS than the have side whenever the system file carries an
// include: over-refusal, not a bypass, which is why this is debt — but it is
// a conjunct of the wall, and nothing refused when it was deleted.
//
// The witness has to be an ALLOWED call, since what the flag buys is the two
// sides agreeing: a system hooksPath reached through an include, a moved HOME
// at a byte-identical global file, and the managed hook firing. Without
// `--includes` the want system read stops at the `include.path` line, the
// sides differ, and this leg is refused.
//
// The system file cannot be planted for real (it is the box's), so the shim
// is rendered over a git wrapper that points GIT_CONFIG_SYSTEM at a scratch
// file on EVERY exec, as TestQAHookSkipSystemTildeHooksPathJudgedAgainstLauncher
// does. The include path is ABSOLUTE, so both HOMEs read the same file and
// the only thing under test is whether the want side follows it at all.
func TestQAHookSkipSystemIncludeIsFollowedOnTheWantSide(t *testing.T) {
	t.Parallel()
	fx := hsSetup(t)
	user := "[user]\n\tname = t\n\temail = t@example.com\n"
	// The hook leaves global scope entirely: it is reached through the system
	// file's include, so the compare's system read is the only side that can
	// see it.
	write(t, filepath.Join(fx.home, ".gitconfig"), user)
	inc := filepath.Join(fx.home, "system-config.inc")
	write(t, inc, "[core]\n\thooksPath = "+filepath.Join(fx.home, "managed-hooks")+"\n")
	sys := filepath.Join(fx.home, "system-config")
	write(t, sys, "[include]\n\tpath = "+inc+"\n")

	dir := t.TempDir()
	wrapper := filepath.Join(dir, "git-wrapper")
	if err := WriteExecutable(wrapper, []byte("#!/bin/sh\nGIT_CONFIG_SYSTEM="+shQuote(sys)+"; export GIT_CONFIG_SYSTEM\nexec "+shQuote(fx.realGit)+" \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ref := resolveHookSkipGlobalRef(wrapper, hsEnv(fx))
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	body := renderShim("hg-sysinc", "git", wrapper, filepath.Join(dir, "refusals.log"), "/bin/date", nil, nil, "", filepath.Join(dir, "hooks"), ref)
	if err := WriteExecutable(filepath.Join(bin, "git"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	prefix := "PATH=" + shQuote(bin) + ":\"$PATH\" "

	same := filepath.Join(fx.home, "same-home")
	if err := os.MkdirAll(same, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(same, ".gitconfig"), user)

	hsGlobalCheck(t, fx, prefix, []hsGlobalLeg{
		// The control: the managed hook IS reached through the system
		// include, so "fired" below is a statement about something.
		{"control: nothing moved, the system include names the managed hook", "", false},
		// The witness: the compare runs (HOME moved) and allows, because
		// both sides follow the include.
		{"HOME=same, byte-identical global file, system hooksPath behind an include", "HOME=" + shQuote(same) + " ", false},
	})
}

// FINDING 4 — hookskip.go, `--includes` on the want CANDIDATE read. Same
// shape in global scope (ADR 0079 pin 15), and the finding that is also a pin
// being partly wrong about what it measures: ADR 0079 pin 8,
// TestQAHookSkipMovedHomeFollowsNoTildeInclude, reads as the include pin, and
// dropping this flag leaves it green, because
// what refuses its fixture is the `~` in its include.path expanding
// differently under the two HOMEs and not the include being followed.
//
// So the fixture here carries an ABSOLUTE include.path whose target holds the
// managed hooksPath: the two sides then read the same included file, agree,
// and the call is ALLOWED with the hook firing. Drop the flag and the want
// side stops at the `include.path` line and refuses.
func TestQAHookSkipGlobalIncludeIsFollowedOnTheWantSide(t *testing.T) {
	t.Parallel()
	for _, rtName := range []string{"claude", "codex"} {
		t.Run(rtName, func(t *testing.T) {
			t.Parallel()
			fx := hsSetup(t)
			dotfile := filepath.Join(fx.home, ".gitconfig")
			managed, err := os.ReadFile(dotfile)
			if err != nil {
				t.Fatal(err)
			}
			// The hook (and the identity) move behind an include whose path
			// carries no `~`, so nothing but the include being followed can
			// make the two sides differ.
			incFile := filepath.Join(fx.home, ".gitconfig.inc")
			write(t, incFile, string(managed))
			top := "[include]\n\tpath = " + incFile + "\n"
			write(t, dotfile, top)
			wrapped := hsWrapped(t, fx, rtName)

			same := filepath.Join(fx.home, "same-home")
			if err := os.MkdirAll(same, 0o755); err != nil {
				t.Fatal(err)
			}
			write(t, filepath.Join(same, ".gitconfig"), top)

			hsGlobalCheck(t, fx, wrapped, []hsGlobalLeg{
				{"control: the included hook fires", "", false},
				{"HOME=same, byte-identical top file, hooksPath behind an absolute include", "HOME=" + shQuote(same) + " ", false},
			})
		})
	}
}

// hsSysTypePath is the want SYSTEM read's flag list as renderHookSkipGlobalScope
// writes it, and hsSysTypePathMutant is that line with `--type=path` dropped —
// arm 6 of ranger-base-fe4uq, applied to the RENDERED shim rather than to the
// source. The spelling is unique in the shim: the have listing reads
// `--show-scope --type=path` with no `--system`, and the candidate read
// `--includes --file "$posse_hs_c" --type=path`.
const (
	hsSysTypePath       = "--system --includes --show-scope --type=path"
	hsSysTypePathMutant = "--system --includes --show-scope"
)

// FINDING 1 of ranger-base-fe4uq (ADR 0079 pin 16) — hookskip.go,
// `--type=path` on the want SYSTEM read. The fifth conjunct of
// renderHookSkipGlobalScope with no witness: delete it and the whole shipped
// TestQAHookSkip filter stayed green (MEASURED 2026-11-06 at main 6125eb4e,
// md5 4b585517b267eb03b9201924c3316376, ok exit 0). It escaped from
// ranger-base-87zme, which shipped the per-read resolution, and from
// ranger-base-pr6p8, whose `--type=path` arms were "both sides raw", "the have
// side raw alone" and "the want side raw alone" — the CANDIDATE read — and
// never this read alone.
//
// Pin 11 (TestQAHookSkipSystemTildeHooksPathJudgedAgainstLauncher) plants the
// same system `core.hooksPath = ~/managed-hooks` and is NOT this conjunct's
// witness: its moved-HOME leg asserts REFUSED and the mutant refuses too, and
// its allowed leg spells the system hooksPath ABSOLUTE, where `--type=path` is
// the identity. GENERAL RULE (ranger-base-fe4uq §5): a conjunct that can only
// make a wall refuse MORE needs an ALLOWED leg as its witness, so ask of a
// candidate pin whether the mutant reaches any of ITS allowed legs.
//
// The witness is a move of XDG_CONFIG_HOME with HOME LEFT ALONE: the D3 gate
// fires (pin 2's "XDG_CONFIG_HOME at its own default" leg is the same move),
// both sides then resolve the system file's `~` against the same HOME, so they
// agree and the call is allowed with the managed hook firing. Drop the flag and
// the want side reads the raw `~/managed-hooks` while the have side reads it
// resolved: the sides differ and the call is refused.
//
// Severity is debt, by measurement and not by reasoning: both sides read the
// SAME system file, so the value text is identical on both, and `--type=path`
// can only make them differ where the resolving HOME differs — where it does,
// the compare refuses either way. Dropping the flag over-refuses and opens
// nothing. What is lost is a conjunct of a security wall that nothing notices
// going away.
//
// The mutant rides along as an in-test witness, as pin 9 does: without it an
// allowed leg is also what a compare that never RAN looks like, and this pin
// would go quiet-green under any later change that stopped the D3 gate firing
// on an XDG-only move.
//
// The system file cannot be planted for real (it is the box's), so the shim is
// rendered over a git wrapper that points GIT_CONFIG_SYSTEM at a scratch file
// on every exec, as pins 11 and 14 do.
func TestQAHookSkipSystemTildeIsTypePathResolvedOnTheWantSide(t *testing.T) {
	t.Parallel()
	fx := hsSetup(t)
	// The hook leaves global scope entirely: it is named by the system file's
	// `~`, so the compare's system read is the only side that can see it.
	write(t, filepath.Join(fx.home, ".gitconfig"), "[user]\n\tname = t\n\temail = t@example.com\n")
	sys := filepath.Join(fx.home, "system-config")
	write(t, sys, "[core]\n\thooksPath = ~/managed-hooks\n")

	dir := t.TempDir()
	wrapper := filepath.Join(dir, "git-wrapper")
	if err := WriteExecutable(wrapper, []byte("#!/bin/sh\nGIT_CONFIG_SYSTEM="+shQuote(sys)+"; export GIT_CONFIG_SYSTEM\nexec "+shQuote(fx.realGit)+" \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ref := resolveHookSkipGlobalRef(wrapper, hsEnv(fx))
	body := renderShim("hg-systype", "git", wrapper, filepath.Join(dir, "refusals.log"), "/bin/date", nil, nil, "", filepath.Join(dir, "hooks"), ref)
	shimAt := func(name, text string) string {
		bin := filepath.Join(dir, name)
		if err := os.MkdirAll(bin, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := WriteExecutable(filepath.Join(bin, "git"), []byte(text), 0o755); err != nil {
			t.Fatal(err)
		}
		return "PATH=" + shQuote(bin) + ":\"$PATH\" "
	}
	prefix := shimAt("bin", body)

	// XDG_CONFIG_HOME pointed at the dir it resolves to anyway: the gate
	// fires, the two sides read the same files under the same HOME, equal.
	xdgLeg := "XDG_CONFIG_HOME=" + shQuote(filepath.Join(fx.home, ".config")) + " "
	hsGlobalCheck(t, fx, prefix, []hsGlobalLeg{
		// The control: the managed hook IS reached through the system tilde,
		// so "fired" below is a statement about something.
		{"control: nothing moved, the system tilde names the managed hook", "", false},
		// THE PIN.
		{"XDG_CONFIG_HOME at its own default, HOME unchanged, system hooksPath = ~/managed-hooks", xdgLeg, false},
	})

	// THE WITNESS: the same leg, `--type=path` dropped from the want SYSTEM
	// read alone. The want side reads `~/managed-hooks` raw, the have side
	// reads it resolved, and the call is refused.
	// Exactly once, not merely present: a second occurrence would mean the
	// spelling is no longer unique to this read and the replace below could
	// land on another one.
	if n := strings.Count(body, hsSysTypePath); n != 1 {
		t.Fatalf("the rendered shim carries the want SYSTEM read's %q %d times, want exactly 1. Either the conjunct is GONE from renderHookSkipGlobalScope — which is what the leg above reds for, and this message is the second half of that one failure — or the flag list was respelled and this witness needs the new spelling. It is not passing either way.", hsSysTypePath, n)
	}
	mprefix := shimAt("mutbin", strings.Replace(body, hsSysTypePath, hsSysTypePathMutant, 1))
	before := hsFired(fx)
	errs, code := hsRun(t, fx, mprefix, hsLeg{env: xdgLeg, args: "commit -q -m w0 -- f"})
	if code == 0 || !strings.Contains(errs, "rule: hookskip:config-env,") || !strings.Contains(errs, "ADR 0079") {
		t.Errorf("WITNESS: with --type=path dropped from the want SYSTEM read the sides must differ and this leg must be refused by hookskip:config-env naming ADR 0079; got code=%d (%q)", code, errs)
	}
	if n := hsFired(fx) - before; n != 0 {
		t.Errorf("WITNESS: the hook ran %d times under the mutant's refusal", n)
	}
}
