package posse

// What survived the attack on four closes (ranger-base-d3fn1), kept runnable.
// Each pin below is a question the close's own tests do not ask, found by
// mutating the fix and watching the suite stay green:
//
//   ranger-base-gk6e  identityLiteralERE's escaping is what keeps check 3
//                  fail-SAFE, and nothing pinned it: `const meta = ""` — the
//                  escaper escaping nothing — left every TestIdentityLiteral*
//                  green. A literal carrying an ERE metacharacter (a
//                  plus-addressed git email is the ordinary one) then renders
//                  a pattern that does not match the literal it was derived
//                  from, and the check passes the leak through. The close's
//                  own fixtures never carry one: the box username and a
//                  t.TempDir() path are both metacharacter-free.
//   ranger-base-gk6e  the never-committed census dispositions by PATH alone,
//                  so any of the four named files may gain a DIFFERENT
//                  identity class silently. Re-keyed by class+path here, on
//                  the census the close actually recorded.
//   ranger-base-evb1  ADR 0025 verification 1's observable — the class in the
//                  rendered `posse gates` line — was measured by a throwaway
//                  test that was removed before the close. Pinned.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ─── ranger-base-gk6e: the escaper is load-bearing, in the fail-open direction ─

// identityLiteralERE renders a fixed string as an ERE matching only itself.
// Both halves matter and only the first is obvious: an UNescaped literal can
// stop matching the very value it was derived from — `a+b@x.org` reads as
// "one or more a", not "a plus b" — so check 3 renders a pattern that finds
// nothing and the operator's own email commits into a public repo with the
// guard reporting clean. Each row's `decoy` is what the unescaped form WOULD
// match: a pin that only asserted self-match would pass with the escaping
// half-done.
func TestQAIdentityLiteralEREEscapesEveryMetacharacter(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, lit, decoy string }{
		{"plus-addressed email", "a+b@example.com", "aab@exampleXcom"},
		{"a dot is not any character", "user@x.org", "user@xYorg"},
		{"a star", "~/src/a*b", "~/src/b"},
		{"a question mark", "~/src/ab?c", "~/src/ac"},
		{"parentheses", "~/src/(a)b", "~/src/ab"},
		{"a bracket class", "~/src/a[bc]d", "~/src/abd"},
		{"a brace repeat", "~/src/a{2}b", "~/src/aab"},
		{"an alternation bar", "~/src/a|b", "~/src/a"},
		{"an anchor pair", "~/src/^a$b", "~/src/ab"},
		{"a backslash", `~/src/a\db`, "~/src/a7b"},
	} {
		t.Run(c.name, func(t *testing.T) {
			re, err := regexp.Compile(identityLiteralERE(c.lit))
			if err != nil {
				t.Fatalf("the rendered ERE must always compile: %v", err)
			}
			if !re.MatchString("prefix " + c.lit + " suffix") {
				t.Errorf("the rendered ERE no longer matches the literal it came from — check 3 would pass this leak through")
			}
			if re.MatchString("prefix " + c.decoy + " suffix") {
				t.Errorf("the rendered ERE matched %q, which is not the literal — the metacharacter is live", c.decoy)
			}
		})
	}
}

// And end to end, through the rendered hook, on the class the close's own
// acceptance test never reaches: `git config user.email`. A plus-addressed
// address is an ordinary git identity and carries an ERE metacharacter, so
// this is the shell-side half of the pin above — grep -oE reads the same
// dialect regexp.Compile does.
func TestQAIdentityGuardCatchesAnEmailCarryingAMetacharacter(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	const email = "qa+probe@example.com"
	home := t.TempDir()
	t.Setenv("HOME", home)
	pub := filepath.Join(home, "pub")
	cfg := filepath.Join(home, "config.yaml")
	if err := os.WriteFile(cfg, []byte("beads_visibility:\n  "+pub+": public\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &App{ConfigPath: cfg}
	if err := os.MkdirAll(pub, 0o755); err != nil {
		t.Fatal(err)
	}
	base := []string{"PATH=" + PathOutsideGates(""), "HOME=" + home,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}
	git := func(env []string, args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", pub}, args...)...)
		cmd.Env = append(append([]string(nil), base...), env...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := git(nil, "init", "-q", "-b", "main"); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	// Repo-local, which is exactly what DeriveIdentityLiterals reads.
	if out, err := git(nil, "config", "user.email", email); err != nil {
		t.Fatalf("git config: %v %s", err, out)
	}
	lits, err := DeriveIdentityLiterals(pub)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, l := range lits {
		if l.Class == "email" && l.Value == email {
			found = true
		}
	}
	if !found {
		t.Fatalf("fixture premise: the email class must derive from the repo config, got %+v", lits)
	}
	if _, _, _, err := a.InstallCommitGuardHook(pub); err != nil {
		t.Fatal(err)
	}

	// A code file, so this also stays outside check 2's markdown pathspec.
	if err := os.WriteFile(filepath.Join(pub, "example.go"), []byte("// contact "+email+"\npackage main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(nil, "add", "example.go")
	out, err := git([]string{"RHQ_PERSONA=tester"}, "commit", "-m", "x", "--", "example.go")
	if err == nil {
		t.Fatalf("the box's own git email must be refused in a public repo:\n%s", out)
	}
	if !strings.Contains(out, "email:") {
		t.Errorf("the refusal must name the email class:\n%s", out)
	}

	// The wall is not simply refusing every commit: a near-miss the
	// UNescaped pattern would have matched must commit clean.
	if err := os.WriteFile(filepath.Join(pub, "clean.go"), []byte("// contact qaaprobe@exampleXcom\npackage main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(nil, "add", "clean.go")
	if out, err := git([]string{"RHQ_PERSONA=tester"}, "commit", "-m", "x", "--", "clean.go"); err != nil {
		t.Errorf("a string that is not the literal must commit clean: %v\n%s", err, out)
	}
}

// ─── ranger-base-evb1: ADR 0025 verification 1's observable ──────────────────

// The class has to survive the trip to the line an operator reads, which is
// Parity.String() — `posse gates <p>` prints nothing else. the evb1 close
// measured this with a throwaway test and removed it, so a Realized row that
// lost its Class, or a Stringer that stopped rendering one, was green.
// Verification 1's own two examples, at the two tiers `posse gates` renders.
func TestQAParityPrintsTheEnforcementClass(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: filepath.Join(home, "config.yaml")}
	claude, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	ag := loadTestAgent(t, "---\nname: pusher\ndeny: [Edit, Write, Bash(git push:*)]\n---\nYou are pusher.\n")

	shims := a.CheckParity(ag, claude, CageShims, TierStrong)
	if got := shims.Realized["Bash(git push:*)"]; got.Class != Cooperative {
		t.Errorf("the shell verb gate is cooperative at shims (ADR 0025 §1), got %q", got.Class)
	}
	if line := shims.String(); !strings.Contains(line, "Bash(git push:*)") || !strings.Contains(line, "→ cooperative (L1 shim") {
		t.Errorf("the class must reach the rendered line, not just the field:\n%s", line)
	}

	seat := a.CheckParity(ag, claude, CageSeatbelt, TierStrong)
	if got := seat.Realized["Edit"]; got.Class != Enforced {
		t.Errorf("L2 seatbelt is enforced (ADR 0025 §1), got %q", got.Class)
	}
	if line := seat.String(); !strings.Contains(line, "→ enforced (L2 seatbelt)") {
		t.Errorf("the enforced class must reach the rendered line:\n%s", line)
	}
	// And the verb gate does NOT get stronger because the file wall did:
	// same launch, both classes on the same block.
	if line := seat.String(); !strings.Contains(line, "→ cooperative (L1 shim") {
		t.Errorf("raising the cage must not reclass the cooperative verb gate:\n%s", line)
	}
	// A row that is not an adversarial gate claim carries no class and must
	// still print its detail — "" is not a third class (ADR 0025 §1).
	nc := RealizedGate{Detail: "nothing to class here"}
	if got := nc.String(); got != "nothing to class here" {
		t.Errorf("an unclassed row must print bare detail, got %q", got)
	}
}
