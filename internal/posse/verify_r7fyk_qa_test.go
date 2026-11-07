//go:build !posse_arm2 && !posse_arm3

package posse

// THE TWO LIVE-DEFECT PINS from verifying ranger-base-5x8xi (verify bead
// ranger-base-r7fyk), INVERTED: both holes are closed by ranger-base-pjui5
// and these arms now assert the refusals.
//
// THEY SHIPPED GREEN ASSERTING THE HOLES, which is the mechanism working as
// written — each arm's failure message named its own inversion, and
// MEASURED 2026-11-07 against the fix all six HOLE arms reddened with
// exactly those messages while every control stayed green. What they held
// was ranger-base-5x8xi's finding 2 one arm over: that fix gave the `--ask`
// arm of `posse hook-report` its own arity, because a bare `--ask` fell past
// `len(args) >= 2` into the shared wrong-argv arm, whose `return 0` is the
// HOOK path's rule, and answered exit 0 over an empty stdout — "a launch
// script's green light over an empty trust line", the one answer
// runCodexHooksAsk's doc says that arm must never give. The `--declare` arm
// beside it was routed on `len(args) == 2` and did the same; and a leading
// dash was a legal app name on the declaration side while the reporter side
// refused it.
//
// WHAT PROCESS BEHAVES DIFFERENTLY IF THESE PINS ARE DELETED (ADR 0006 §7).
// Not a comment: `docs/adr/0076-trust-ask.probe.sh` reads the declaration at
// lines 122 and 171 as
//
//	DECL=$("$POSSE" hook-report --declare codex) || exit 1
//
// which is the shape finding 1 was about — an exit 0 over an empty stdout
// leaves DECL empty and the `|| exit 1` unfired, and the script carries on
// and installs nothing. That is a shipped script in this repo and the only
// reader of this arm that is not a hand run. For finding 2 it is a seat: a
// runtime whose name starts with a dash launched with codex trusting a
// reporter that reported nothing, so Rex read no record for it, the settle
// was never seen and dispatch judged the bead on a status probe alone.
//
// WHY THERE ARE TWO COPIES OF FINDING 1's TABLE, on purpose and not by
// drift: TestHookReportDeclareArmOwnsItsArity (codexreport_test.go) is the
// REGRESSION pin, beside the code it holds and reachable by the `-run` filter
// anyone editing that code would type. This file is the PROVENANCE pin, which
// is where a reader following ranger-base-r7fyk lands. Neither is stale; if
// they ever disagree, the one beside the code is the one to believe.
//
// MEASURED 2026-11-07, darwin 25.6.0, go1.26.10, over the fix on main
// e6a8b198. Fixed under ranger-base-pjui5; found under ranger-base-r7fyk;
// the hole was built by ranger-base-z7kle and its twin fixed by
// ranger-base-5x8xi.

import (
	"bytes"
	"strings"
	"testing"
)

// TestQAHookReportDeclareArmOwnsItsArity is finding 1, closed.
//
// `--declare` is routed on `len(args) >= 1` now and refuses any arity but 2
// from the arm itself, so neither an argv one word short nor one word long
// can reach the shared wrong-argv arm and borrow the hook path's `return 0`.
//
// EXIT 1 IS WHAT THE ARM ALREADY DID WHEN IT WAS ENTERED — it returns 1 when
// CodexHooksDeclaration refuses the app name — which is what said the finding
// was the ARITY and not the validation, and it is why the bound below is
// still here. And exit 1 holds no prompt: codex reads **2** from a hook as
// "block this", so an exit 1 on an argv no declaration installs blocks
// nothing. Re-pointing TestHookReportNeverExitsNonZeroOnTheHookPath's
// `{"posse", HookReportVerb, HookReportDeclareFlag}` row, which asserted this
// hole under a justification stricter than the rule it named, was part of
// this fix rather than a separate decision.
func TestQAHookReportDeclareArmOwnsItsArity(t *testing.T) {
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

	usage := "usage: posse " + HookReportVerb + " " + HookReportDeclareFlag + " <app>"
	for _, tc := range []struct {
		name, why string
		argv      []string
	}{
		{
			name: "one word short is refused by the arm itself",
			why:  "an argv one word short",
			argv: []string{HookReportDeclareFlag},
		},
		{
			name: "one word long is refused by the arm itself",
			why:  "an argv one word long",
			argv: []string{HookReportDeclareFlag, "codex", "extra"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, out, errs := run(t, tc.argv...)
			if code != 1 || out != "" {
				t.Fatalf("THE HOLE IS BACK: %s (%v) answered exit %d and stdout %q, want exit 1 and nothing. Exit 0 over an empty stdout is what docs/adr/0076-trust-ask.probe.sh reads as a rendered declaration — `DECL=$(… --declare codex) || exit 1` leaves DECL empty with the `|| exit 1` unfired (ranger-base-pjui5, found verifying ranger-base-5x8xi under ranger-base-r7fyk)",
					tc.why, tc.argv, code, out)
			}
			// And it is the DECLARE ARM's own usage, not the shared
			// wrong-argv arm's: falling into that arm is the whole shape of
			// the finding, so a refusal that borrowed its sentence would be
			// the same bug with a better exit code.
			if !strings.Contains(errs, usage) {
				t.Errorf("%s was refused without naming its own arm: %q, want a usage carrying %q. The finding was this argv falling into the SHARED wrong-argv arm, so the arm that answers is half of what is pinned here", tc.why, errs, usage)
			}
		})
	}

	// THE BOUND, and it is what said the finding was the ARITY and not the
	// validation: an app name the token rule refuses always DID reach the
	// declare arm and exit 1. The arm was never indifferent to bad input — it
	// was unreachable for the two argv above.
	t.Run("bound: an app name the token rule refuses exits 1", func(t *testing.T) {
		code, out, _ := run(t, HookReportDeclareFlag, "codex:evil")
		if code != 1 || out != "" {
			t.Errorf("`%s %s codex:evil` exited %d and printed %q, want exit 1 and nothing — the declare arm's refusal, which the two arms above could not reach before ranger-base-pjui5", HookReportVerb, HookReportDeclareFlag, code, out)
		}
	})
}

// TestQAHookReportCannotRenderADeclarationItsOwnReporterRefuses is finding 2,
// closed.
//
// A leading dash used to be a legal plainStatusToken (`-` is in its character
// set), so `-c`, `-`, `--declare` and `--ask` were app names BOTH the declare
// arm and D7's ask arm accepted — and CodexHooksDeclaration rendered `posse
// hook-report <app>` as the hook command for all eight events. But
// runHookReport's shared wrong-argv arm is
// `len(args) != 1 || HasPrefix(args[0], "-")`, so when codex then RAN that
// command the reporter never reached its own report path: it opened no tty
// and wrote nothing, at exit 0, on every event, forever. MEASURED 2026-11-07
// before the fix: the declaration rendered for all four and the command each
// installed wrote 0 bytes, against 29 bytes for the `codex` control.
//
// plainStatusToken REFUSES A LEADING DASH NOW, which is the cheapest spelling
// of "the two sides read one rule" — the dash is otherwise legal here
// (`grok-build`), and what makes a leading one different is that the app name
// is also the last word of an argv codex splits and runs. runHookReport's own
// dash guard stays where it is: it is reached before any name rule, and its
// subject is the exit code codex sees rather than what a name may be.
//
// THE `--ask` ROW is the half ranger-base-5x8xi created: before its arity
// change that argv landed in the shared arm at exit 0 like the others, and
// after it reaches runCodexHooksAsk's usage and exits 1 — against this file's
// rule, pinned by TestHookReportNeverExitsNonZeroOnTheHookPath, that no arm
// on the hook path returns anything but 0. It stays exit 1 and that is now
// sound rather than tolerated, for the reason the arity pin above gives: 1 is
// not codex's block, and since the name rule refuses the name, no declaration
// can install that argv for codex to run. The arm below asserts both halves.
func TestQAHookReportCannotRenderADeclarationItsOwnReporterRefuses(t *testing.T) {
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

	// A DASH IS STILL A NAME BYTE anywhere but the front, which is what keeps
	// the arms below about the position rather than the character.
	t.Run("bound: a dash inside a name is still a name", func(t *testing.T) {
		if !plainStatusToken("grok-build") {
			t.Fatal("plainStatusToken refuses \"grok-build\" — ranger-base-pjui5 refused a LEADING dash, and a rule that took the byte everywhere would refuse a runtime name posse really reports")
		}
		if _, err := CodexHooksDeclaration("/bin/posse", "grok-build"); err != nil {
			t.Errorf("the declaration refuses \"grok-build\": %v", err)
		}
	})

	for _, app := range []string{"-c", "-", HookReportDeclareFlag, HookReportAskFlag} {
		t.Run("a declaration for "+app+" is refused before a seat is launched", func(t *testing.T) {
			if plainStatusToken(app) {
				t.Fatalf("THE HOLE IS BACK in plainStatusToken: %q is a legal app name again. A name that looks like a flag is the last word of the argv codex runs, and runHookReport reads a leading dash there as `not a hook argv` before it has any opinion about names (ranger-base-pjui5)", app)
			}
			if decl, err := CodexHooksDeclaration("/bin/posse", app); err == nil {
				t.Fatalf("THE HOLE IS BACK in the declaration: %q rendered %.120q. The command that installs is one this binary's own reporter refuses to run, so the seat gets a reporter silent on every event forever while codex trusts it (ranger-base-pjui5)", app, decl)
			}

			// AND THE REPORTER SIDE IS UNCHANGED, which is what says the fix
			// closed the disagreement rather than moved it: this argv still
			// writes no record. It is the EXIT that differs between a plain
			// dash name and this verb's own flag spellings, and neither is
			// codex's block.
			var out, errs bytes.Buffer
			q := &qaTTY{}
			code := runHookReport([]string{"posse", HookReportVerb, app},
				strings.NewReader(`{"hook_event_name":"SessionStart"}`), &out, &errs, q.open)
			if q.opens != 0 || q.buf.Len() != 0 {
				t.Errorf("the reporter wrote a record for %q (%d opens, %q) — a name the declaration refuses is not a name the reporter should report under", app, q.opens, q.buf.String())
			}
			want := 0
			if app == HookReportDeclareFlag || app == HookReportAskFlag {
				want = 1
			}
			if code != want {
				t.Errorf("`%s %s` exited %d, want %d", HookReportVerb, app, code, want)
			}
			if code == 2 {
				t.Errorf("`%s %s` exited 2, which is the one status codex reads from a hook as `block this prompt`", HookReportVerb, app)
			}
		})
	}
}
