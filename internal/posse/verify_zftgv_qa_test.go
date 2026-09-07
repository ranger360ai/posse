//go:build posse_arm2

package posse

// QA pins from verify bead ranger-base-zftgv. Two holes measured while
// verifying four closes, neither of them a live miss today, each with the
// property that makes a pin worth more than a bead line: it goes RED on the
// day it becomes one.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ranger-base-xwepd closed the fourth bucket of hookdeps_qa_test.go's
// completeness contract by teaching the scanner a backslash and by making
// skipOver REPORT a command substitution found inside a region the scan
// steps over unread. Two such regions were given that treatment —
// skipArith's `$(( ))` and skipBraceExpansion's `${ }`. There was a THIRD,
// and it did not get it: skipRedirect walked a redirection and its target
// word and stepped over a quoted target whole, so a substitution written
// there ran a command the scan neither named nor reported.
//
// FIXED (ranger-base-i6t90): skipRedirect now gives a DOUBLE-quoted target
// skipOver's own treatment — a `$(` or a backtick found inside it is
// reported, marker `>` — and leaves a SINGLE-quoted target alone, because a
// single-quoted target runs nothing (`cat > '$(awk 1)'`) and a plain
// Contains over the raw region would report a site that is not one. The
// unquoted spelling (`cat > $(awk 1)`) is untouched: it was already SEEN,
// because skipRedirect stops the target word at `(` and the outer scanner
// picks up `awk` from there.
//
// This is now a REGRESSION guard, not a silence pin: it reds the day the
// fix above is lost, the same way TestShellCommandWordsSeesEveryCommandPrefixOrReportsIt's
// "reported" rows for this shape do (hookdeps_qa_test.go) — kept here too
// because this file is where the live census (half two) lives, and a
// regression in the report and a regression in the census are different
// failures worth telling apart.
func TestQARedirectTargetSubstitutionIsReportedAndNoHookHasOne(t *testing.T) {
	t.Parallel()

	// Half one: the report, with the controls that prove the probe can see
	// an `awk` at all and that a single-quoted target is correctly left
	// alone.
	sees := func(src string) (seen bool, blind []string) {
		w, b := shellCommandWords(src)
		for _, c := range w {
			if c.Name == "awk" {
				seen = true
			}
		}
		for _, c := range b {
			blind = append(blind, c.Name)
		}
		return seen, blind
	}
	for _, row := range []struct {
		what, src string
	}{
		{"a double-quoted redirect target", "cat > \"$(awk 1)\"\n"},
		{"an append target", "cat >> \"$(awk 1)\"\n"},
		{"an fd-numbered target", "cat 2> \"$(awk 1)\"\n"},
		{"an input redirect target", "grep x < \"$(awk 1)\"\n"},
		{"a redirect written BEFORE the command", "> \"$(awk 1)\" cat f\n"},
		{"a backtick in the target", "cat > \"`awk 1`\"\n"},
		{"a parameter expansion in the target", "cat > \"${x:-$(awk 1)}\"\n"},
	} {
		if seen, blind := sees(row.src); !seen && len(blind) == 0 {
			t.Errorf("%s: shellCommandWords is silent on it again (seen=%t blind=%v) — skipRedirect's quoted-target "+
				"substitution check (hookdeps_qa_test.go) regressed (ranger-base-zftgv, from ranger-base-xwepd)",
				row.what, seen, blind)
		}
	}
	// The single-quoted control: the fix must not report a site that runs
	// nothing.
	if seen, blind := sees("cat > '$(awk 1)'\n"); seen || len(blind) > 0 {
		t.Errorf("a SINGLE-quoted redirect target is seen or reported (seen=%t blind=%v) but it runs nothing — "+
			"skipRedirect must leave a single-quoted target unread", seen, blind)
	}
	// The plain control, with a Fatal: without it the rows above are a
	// report asserted over a probe that can see nothing.
	if seen, _ := sees("cat \"$(awk 1)\"\n"); !seen {
		t.Fatal("the control did not see `awk` in `cat \"$(awk 1)\"` either, so the rows above measured a broken probe and not a real report")
	}
	if seen, blind := sees("cat > $(awk 1)\n"); !seen {
		t.Fatal("the UNQUOTED redirect target stopped being seen — skipRedirect no longer stops its target word at `(`, so the fix above changed more than this test describes")
	} else if len(blind) > 0 {
		t.Errorf("the UNQUOTED redirect target is ALSO reported (blind=%v) — it is already seen by the outer scanner, and reporting it too is a duplicate the reader cannot act on", blind)
	}

	// Half two: the live census. Not a live miss today; this reds the day a
	// hook actually writes one — now graded off skipRedirect's own `subst`
	// return rather than a second Contains check, so a regression in the fix
	// and a regression in this census cannot silently diverge.
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo := gitTempDir(t)
	cmd := exec.Command("git", "-C", repo, "init", "-q", "-b", "main")
	cmd.Env = []string{"PATH=" + PathOutsideGates(""), "HOME=" + repo}
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, b)
	}
	hookPath, err := installCommitGuard(repo)
	if err != nil {
		t.Fatalf("install prepare-commit-msg: %v", err)
	}
	commitGuard, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatal(err)
	}
	rendered := map[string]string{
		"prepare-commit-msg": string(commitGuard),
		"pre-push":           PrePushHook,
		"chain dispatcher":   chainHookDispatcherWith("prepare-commit-msg", "theirs-prepare-commit-msg"),
	}
	regions := 0
	for _, name := range sortedKeys(rendered) {
		src := rendered[name]
		for i := 0; i < len(src); i++ {
			if src[i] != '>' && src[i] != '<' {
				continue
			}
			end, heredoc, subst := skipRedirect(src, i)
			if heredoc {
				continue // its own reported marker, `<<`
			}
			regions++
			if subst {
				t.Errorf("%s:%d writes a command substitution in a redirection target (%q) — shellCommandWords now "+
					"reports it (marker `>`), so HOOK_DEPS will not go quiet — but a hook growing this shape is worth a "+
					"look on its own merits (ranger-base-zftgv, from ranger-base-xwepd)",
					name, 1+strings.Count(src[:i], "\n"), src[i:end])
			}
			i = end - 1
		}
	}
	// A zero census with no positive control counts nothing: these hooks
	// redirect constantly, so a run that finds no region at all found no
	// hooks.
	if regions < 50 {
		t.Fatalf("only %d redirection regions over the three rendered hooks — the census above measured almost nothing (93 at the time this was written)", regions)
	}
}

// ranger-base-te3ib put ADR 0042 D2's credential precondition in front of
// planLaunch and, for ranger-base-az23f, RelaunchAgent. Its fold comment
// offered the alternative of hoisting the check into WrapWithGates "so a
// third renderer cannot forget it". There is a third: runtimeprobe.go's
// `posse runtime probe` renders gates from a scratch PID's own Deny at the
// shims tier, creates a workspace and runs the runtime CLI in it, and calls
// neither CredGateWarning (which is gone) nor CheckCredGate.
//
// It is not a live gap, for one reason and one reason only: the probe's
// deny is a single `Bash(<canary>:*)` drawn from ProbeCanaryCandidates, and
// no candidate is any runtime's CredBin — so CredGateCollision is empty
// there by construction. Nothing said so, and nothing would notice: add a
// credential binary to that list and the probe launches a session that
// cannot authenticate, then reports the runtime as one that never settles —
// exactly the misdiagnosis ADR 0042 D1 was written about.
//
// This is that guard. It costs one launch of the probe nothing and it is
// the cheapest way to keep the third renderer honest without hoisting.
func TestQANoProbeCanaryIsARuntimesCredentialBinary(t *testing.T) {
	t.Parallel()
	a := NewAppAt(t.TempDir())
	names := a.ListRuntimes()
	if len(names) == 0 {
		t.Fatal("no runtimes to grade the canary list against — this test measured nothing")
	}
	creds := map[string]string{} // CredBin -> the runtime that declares it
	for _, n := range names {
		rt, err := a.LoadRuntime(n)
		if err != nil {
			t.Fatalf("load runtime %s: %v", n, err)
		}
		if rt.CredBin != "" {
			creds[rt.CredBin] = n
		}
	}
	if len(creds) == 0 {
		t.Fatal("no runtime declares a CredBin, so this test cannot fail and grades nothing (claude declared `security` when it was written)")
	}
	for _, c := range ProbeCanaryCandidates {
		if rt, ok := creds[c]; ok {
			t.Errorf("ProbeCanaryCandidates names %q, which is runtime %s's own credential binary (Runtime.CredBin). "+
				"`posse runtime probe` renders that deny into a real launch and is the ONE renderer of a persona line that does not "+
				"call CheckCredGate (herdrback.go planLaunch and RelaunchAgent both do), so the probe would open a session that cannot "+
				"authenticate and report the runtime as never settling — ADR 0042 D1's misdiagnosis with a new sentence. Either drop the "+
				"canary, or give runtimeprobe.go the precondition too (ranger-base-zftgv, from ranger-base-te3ib / ranger-base-az23f)",
				c, rt)
		}
	}
	// The wrong arm, so a list that grades nothing cannot pass by being
	// empty: the check must be able to fire.
	var wall []string
	for c := range creds {
		wall = append(wall, c)
	}
	hit := false
	for _, c := range append(append([]string{}, ProbeCanaryCandidates...), wall...) {
		if _, ok := creds[c]; ok {
			hit = true
		}
	}
	if !hit {
		t.Fatalf("the check cannot fire even with a CredBin spliced into the list it reads — it is grading nothing (canaries %v, credential binaries %v)", ProbeCanaryCandidates, wall)
	}
	_ = filepath.Join
}
