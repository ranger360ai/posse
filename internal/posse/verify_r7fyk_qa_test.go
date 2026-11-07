//go:build !posse_arm2 && !posse_arm3

package posse

// TWO LIVE-DEFECT PINS from verifying ranger-base-5x8xi (verify bead
// ranger-base-r7fyk, findings bead named on it).
//
// THEY SHIP GREEN AND THEY ASSERT HOLES. ranger-base-5x8xi's finding 2 fixed
// the `--ask` arm of `posse hook-report`: a bare `--ask` fell past the arm's
// own `len(args) >= 2` into the shared wrong-argv arm, whose `return 0` is
// the HOOK path's rule, and answered exit 0 over an empty stdout — "a launch
// script's green light over an empty trust line", which runCodexHooksAsk's
// own doc calls the one answer that arm must never give. The arm owns its
// arity now. The `--declare` arm beside it does not, and nothing moved there.
//
// WHAT PROCESS BEHAVES DIFFERENTLY IF THESE PINS ARE DELETED (ADR 0006 §7).
// Not a comment: `docs/adr/0076-trust-ask.probe.sh` reads the declaration at
// lines 122 and 171 as
//
//	DECL=$("$POSSE" hook-report --declare codex) || exit 1
//
// which is exactly the shape finding 2 was about — an exit 0 over an empty
// stdout leaves DECL empty and the `|| exit 1` unfired, and the script
// carries on and installs nothing. That is a shipped script in this repo,
// and it is the only reader of this arm that is not a hand run.
//
// EXIT 1 IS WHAT THE ARM ALREADY DOES WHEN IT IS ENTERED: the declare arm
// returns 1 when CodexHooksDeclaration refuses the app name (codexreport.go).
// So the arm is not undecided about refusals — it is inconsistent about the
// one argv that does not reach it.
//
// WHAT A FIXER OF FINDING 1 HITS FIRST, measured by planting the fix:
// TestHookReportNeverExitsNonZeroOnTheHookPath (codexreport_test.go:204)
// ASSERTS this hole. Its table carries `{"posse", HookReportVerb,
// HookReportDeclareFlag}` and requires exit 0, under the message "a malformed
// handler line must not be able to block a prompt" — and that justification
// is stricter than the rule it names: the rule in this file's own header is
// that codex reads exit **2** as a block, so an exit 1 on a non-hook argv
// blocks nothing. Re-pointing that row is part of finding 1's fix, not a
// separate decision, and the second pin below is coupled to it the same way:
// its `--declare` arm asserts exit 0 for the command a `--declare`
// declaration installs, which the same fix turns into exit 1.
//
// MEASURED 2026-11-07, darwin 25.6.0, go1.26.10, at main e6a8b198.

import (
	"bytes"
	"strings"
	"testing"
)

// TestQAHookReportDeclareArmDoesNotOwnItsArity is finding 1.
//
// `--declare` is routed on `len(args) == 2`, so an argv one word short and an
// argv one word long both fall into the shared wrong-argv arm and answer
// exit 0 with an empty stdout. The ask arm's twin of this is now exit 1
// (ranger-base-5x8xi); this one is not.
//
// WHEN THE FIX LANDS THIS PIN REDS. That is deliberate and it is the whole
// mechanism: invert the two HOLE arms to assert exit 1 from the declare
// arm's own refusal, keep the control as it is, and name the bead that fixed
// it. Do not delete the arms — the control below is the half that says an
// arity fix landed in the right place rather than refusing the argv a launch
// line actually types.
func TestQAHookReportDeclareArmDoesNotOwnItsArity(t *testing.T) {
	t.Parallel() // argv in, a sentence out: no env, no cwd, no child

	run := func(t *testing.T, argv ...string) (int, string, string) {
		t.Helper()
		var out, errs bytes.Buffer
		q := &qaTTY{}
		code := runHookReport(append([]string{"posse", HookReportVerb}, argv...),
			strings.NewReader(`{"hook_event_name":"SessionStart"}`), &out, &errs, q.open)
		return code, out.String(), errs.String()
	}

	// THE CONTROL FIRST, so an arm below that passes cannot be passing
	// because the declare arm has stopped rendering anything at all.
	t.Run("control: the argv a launch line types renders a declaration", func(t *testing.T) {
		code, out, _ := run(t, HookReportDeclareFlag, "codex")
		if code != 0 || !strings.Contains(out, "hooks={") {
			t.Fatalf("`%s %s codex` exited %d and printed %q — the arms below measure nothing if this is not a rendered declaration",
				HookReportVerb, HookReportDeclareFlag, code, out)
		}
	})

	for _, tc := range []struct {
		name, why string
		argv      []string
	}{
		{
			name: "HOLE: one word short answers exit 0 over an empty stdout",
			why:  "an argv one word short",
			argv: []string{HookReportDeclareFlag},
		},
		{
			name: "HOLE: one word long answers exit 0 over an empty stdout",
			why:  "an argv one word long",
			argv: []string{HookReportDeclareFlag, "codex", "extra"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, out, errs := run(t, tc.argv...)
			if code != 0 || out != "" {
				t.Fatalf("THE FIX HAS LANDED, and this pin is now the wrong way round: %s (%v) answered exit %d and stdout %q. Invert this arm to assert the declare arm's own refusal — exit 1, as its sibling returns when CodexHooksDeclaration refuses an app name, and as `%s %s` has returned since ranger-base-5x8xi — keep the control, and name the bead that closed it (found verifying ranger-base-5x8xi under ranger-base-r7fyk)",
					tc.why, tc.argv, code, out, HookReportVerb, HookReportAskFlag)
			}
			// And it is the SHARED arm it fell into, not the declare arm
			// saying something of its own: that is what makes the exit 0 the
			// hook path's rule applied to a non-hook argv.
			if !strings.Contains(errs, HookReportDeclareFlag+" <app>  |") {
				t.Errorf("%s reached %q, and the shape of this finding is that it falls into the SHARED wrong-argv arm — if a declare-only usage answers now, say which here, because the bound on the finding is whatever that arm happens to cover", tc.why, errs)
			}
		})
	}

	// THE BOUND, and it is what says the finding is the ARITY and not the
	// validation: an app name the token rule refuses DOES reach the declare
	// arm and DOES exit 1. So the arm is not indifferent to bad input — it is
	// unreachable for the two argv above.
	t.Run("bound: an app name the token rule refuses already exits 1", func(t *testing.T) {
		code, out, _ := run(t, HookReportDeclareFlag, "codex:evil")
		if code != 1 || out != "" {
			t.Errorf("`%s %s codex:evil` exited %d and printed %q, want exit 1 and nothing — the declare arm's refusal is the answer the two HOLE arms above cannot reach", HookReportVerb, HookReportDeclareFlag, code, out)
		}
	})
}

// TestQAHookReportRendersADeclarationItsOwnReporterRefuses is finding 2.
//
// A leading dash is a legal plainStatusToken (`-` is in its character set,
// codexreport.go), so `-c`, `-`, `--declare` and `--ask` are app names both
// the declare arm and D7's ask arm accept — and CodexHooksDeclaration renders
// `posse hook-report <app>` as the hook command for all eight events. But
// runHookReport's shared wrong-argv arm is `len(args) != 1 || HasPrefix(args[0], "-")`,
// so when codex then RUNS that command the reporter never reaches its own
// report path: it opens no tty and writes nothing, at exit 0, on every event,
// forever. The declaration and the reporter disagree about which app names
// are usable, and the token rule is checked on only one side of that.
//
// `--ask` is the sharper half and it is NEW: before ranger-base-5x8xi the ask
// arm was routed on `len(args) >= 2`, so a hook running `posse hook-report
// --ask` landed in the shared arm at exit 0 like the others. The arm owns its
// arity now, so that argv reaches runCodexHooksAsk's usage and exits 1 — and
// this file's own rule, pinned by TestHookReportNeverExitsNonZeroOnTheHookPath,
// is that no arm on the hook path returns anything but 0.
//
// WHAT PROCESS BEHAVES DIFFERENTLY IF THIS PIN IS DELETED (ADR 0006 §7): a
// seat whose runtime is named with a leading dash launches with codex trusting
// a reporter that reports nothing, so Rex reads no record for that seat, the
// settle is never seen and dispatch judges the bead on a status probe alone.
// Reachable today by a hand run, a hand-written launch script, or a
// `runtimes/<name>.yaml` whose name starts with a dash — which is the cost
// class ranger-base-5x8xi accepted for its own finding 2.
//
// WHEN THE FIX LANDS THIS PIN REDS, as the one above: invert each HOLE arm to
// assert the refusal, keep the control.
func TestQAHookReportRendersADeclarationItsOwnReporterRefuses(t *testing.T) {
	t.Parallel()

	// The CONTROL: a real app name renders, and the rendered command reports.
	t.Run("control: a real app name renders and the reporter reports", func(t *testing.T) {
		if !plainStatusToken("codex") {
			t.Fatal("fixture: plainStatusToken refuses \"codex\", so nothing below measures what it claims")
		}
		decl, err := CodexHooksDeclaration("/bin/posse", "codex")
		if err != nil || !strings.Contains(decl, HookReportVerb+" codex") {
			t.Fatalf("fixture: the declaration for codex is (%v) %.80q", err, decl)
		}
		var out, errs bytes.Buffer
		q := &qaTTY{}
		code := runHookReport([]string{"posse", HookReportVerb, "codex"},
			strings.NewReader(`{"hook_event_name":"SessionStart"}`), &out, &errs, q.open)
		if code != 0 || q.opens != 1 || q.buf.Len() == 0 {
			t.Fatalf("fixture: the rendered command for codex exited %d, opened the tty %d time(s) and wrote %d bytes — the arms below measure nothing if the reporter is not working",
				code, q.opens, q.buf.Len())
		}
	})

	for _, app := range []string{"-c", "-", HookReportDeclareFlag, HookReportAskFlag} {
		t.Run("HOLE: a declaration for "+app+" installs a reporter that never reports", func(t *testing.T) {
			if !plainStatusToken(app) {
				t.Fatalf("THE FIX MAY HAVE LANDED IN plainStatusToken: it now refuses %q. If a leading dash is no longer a legal app name, invert this arm to assert THAT refusal and say so (ranger-base-r7fyk)", app)
			}
			decl, derr := CodexHooksDeclaration("/bin/posse", app)
			if derr != nil {
				t.Fatalf("THE FIX HAS LANDED in the declaration: CodexHooksDeclaration refuses %q with %v. Invert this arm to assert that refusal, keep the control, and name the bead (found verifying ranger-base-5x8xi under ranger-base-r7fyk)", app, derr)
			}
			if !strings.Contains(decl, HookReportVerb+" "+app) {
				t.Fatalf("the declaration for %q does not name it as the reporter's argument, so this arm is aimed at the wrong thing: %.120q", app, decl)
			}

			// Now run the command that declaration installs, as codex would.
			var out, errs bytes.Buffer
			q := &qaTTY{}
			code := runHookReport([]string{"posse", HookReportVerb, app},
				strings.NewReader(`{"hook_event_name":"SessionStart"}`), &out, &errs, q.open)
			if q.opens != 0 || q.buf.Len() != 0 {
				t.Fatalf("THE FIX HAS LANDED in the reporter: the command declared for %q opened the tty %d time(s) and wrote %q. Invert this arm — a declaration and the reporter it installs now agree about this app name (ranger-base-r7fyk)",
					app, q.opens, q.buf.String())
			}
			if app == HookReportAskFlag {
				// The half ranger-base-5x8xi's arity change created. Exit 1
				// from a hook argv is against this file's own rule.
				if code != 1 {
					t.Fatalf("THE FIX HAS LANDED: the command declared for %q exits %d, not 1. This arm exists because ranger-base-5x8xi's arity change routed this argv to runCodexHooksAsk's usage, and TestHookReportNeverExitsNonZeroOnTheHookPath's rule is that no arm here returns anything but 0 — invert it to assert exit 0 and name the bead (ranger-base-r7fyk)", app, code)
				}
				return
			}
			if code != 0 {
				t.Fatalf("THE FIX HAS LANDED: the command declared for %q exits %d rather than reporting at exit 0. Invert this arm and name the bead (ranger-base-r7fyk)", app, code)
			}
		})
	}
}
