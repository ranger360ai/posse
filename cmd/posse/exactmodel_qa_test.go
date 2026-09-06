package main

// ADR 0053 D1/D5 at the argv boundary (ranger-base-1oyio). The refusals are
// asserted over the BUILT BINARY rather than over parseNewFlags, because
// what the bead promises is that a bad canary line refuses "before a
// workspace or session record exists" — and "created nothing" is a claim
// about a process, not about a function (the shape TestNewHelpCreatesNothing
// established for rangerhq-qv5).
//
// RHQ_HERDR_BIN points at a path that does not exist, so any launch that got
// past the refusal would fail loudly rather than quietly reaching a herdr.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// namesTheRemovedSubstitution answers any spelling of the mechanism ADR 0003
// §3 removed. It is this package's copy of the family
// internal/posse/exactmodel_qa_test.go spells for the warn line — two
// packages, so two vars, and they must stay identical: the same rewording
// that walks out from under one walks out from under the other, which is why
// ranger-base-g6k5b's fix and ranger-base-8v29w's finding each landed on both
// surfaces at once. The reasoning is written out at the internal copy;
// TestTheTwoSubstitutionFamiliesAreOneFamily below is why "must stay
// identical" is a pin here rather than a hope.
var namesTheRemovedSubstitution = regexp.MustCompile(`(?i)(substitut|fall|\bfell\b)`)

// substitutionFamilyLiteral answers the regexp literal one file gives
// namesTheRemovedSubstitution, or "" if that file does not spell one.
func substitutionFamilyLiteral(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		t.Fatalf("read %s: %v — this pin needs both copies, and a missing one is not a match", path, err)
	}
	const decl = "var namesTheRemovedSubstitution = regexp.MustCompile(`"
	i := strings.Index(string(b), decl)
	if i < 0 {
		return ""
	}
	rest := string(b)[i+len(decl):]
	j := strings.IndexByte(rest, '`')
	if j < 0 {
		return ""
	}
	return rest[:j]
}

// TestTheTwoSubstitutionFamiliesAreOneFamily is the guard on the sentence
// above. Two operator-visible surfaces carry the same must-NOT vocabulary and
// they are in different packages, so the ban is spelled twice — and a ban
// spelled twice is a ban that gets widened once. That is not hypothetical
// here: it is the exact shape of ranger-base-8v29w's first finding, where one
// respelling walked out from under BOTH copies and each had to be widened in
// the same commit, and of ranger-base-g6k5b's fix before it.
//
// It compares the literals rather than the compiled regexps because that is
// what a reader diffs and what a reviewer waives: two patterns that match the
// same strings today but are written differently are already drifting.
func TestTheTwoSubstitutionFamiliesAreOneFamily(t *testing.T) {
	here := substitutionFamilyLiteral(t, "exactmodel_qa_test.go")
	there := substitutionFamilyLiteral(t, "../../internal/posse/exactmodel_qa_test.go")
	if here == "" || there == "" {
		t.Fatalf("the family literal was not found in both copies (cmd %q, internal %q) — "+
			"this pin is reading for a declaration that has been renamed, so its silence measures nothing", here, there)
	}
	if here != there {
		t.Errorf("the two copies of the removed-substitution family have drifted, so a respelling "+
			"caught on one operator surface is missed on the other (ranger-base-8v29w):\n\tcmd/posse      %s\n\tinternal/posse %s", here, there)
	}
}

func TestNewModelFlagRefusals(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	env := append(os.Environ(),
		"RHQ_HOME="+home,
		"RHQ_HERDR_BIN="+filepath.Join(home, "herdr-must-not-run"),
	)

	for _, c := range []struct {
		args []string
		code int
		want string
	}{
		// The flag exists and is documented where the operator looks.
		{[]string{"new", "--help"}, 0, "--model <id>"},
		// D1's companions, one refusal each.
		{[]string{"new", "s", "--model", "gpt-6-astra"}, 1, "needs --agent"},
		{[]string{"new", "s", "--agent", "architect", "--model", "gpt-6-astra"}, 1, "needs an explicit --runtime"},
		{[]string{"new", "s", "--agent", "architect", "--runtime", "codex", "--model", "gpt-6-astra"}, 1, "needs an explicit --tier"},
		// D1's id rule.
		{[]string{"new", "s", "--agent", "architect", "--runtime", "codex", "--tier", "strong", "--model", "gpt 6"}, 1, "is not one token"},
		{[]string{"new", "s", "--agent", "architect", "--runtime", "codex", "--tier", "strong", "--model", "gpt\x1b6"}, 1, "control character"},
		{[]string{"new", "s", "--agent", "architect", "--runtime", "codex", "--tier", "strong", "--model"}, 1, "flag --model needs a value"},
	} {
		out, code := runRhq(t, bin, env, c.args...)
		if code != c.code || !strings.Contains(out, c.want) {
			t.Errorf("posse %s: exit %d, output %q; want exit %d containing %q",
				strings.Join(c.args, " "), code, out, c.code, c.want)
		}
	}

	// The whole point of refusing at parse time: nothing on the host moved.
	var left []string
	filepath.WalkDir(home, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			left = append(left, p)
		}
		return nil
	})
	if len(left) > 0 {
		t.Errorf("a refused canary must leave no state behind: %q", left)
	}
}

// ADR 0053 D5: the override is a crew-session flag and nothing else. Dispatch
// is the one other launcher with its own flag table, and a pass-wide model
// experiment is a different risk boundary — so `--model` is not a flag there
// and must not become one by being ignored.
func TestDispatchHasNoModelFlag(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	env := append(os.Environ(),
		"RHQ_HOME="+home,
		"RHQ_HERDR_BIN="+filepath.Join(home, "herdr-must-not-run"),
	)
	out, code := runRhq(t, bin, env, "dispatch", "--model", "gpt-6-astra")
	if code == 0 || !strings.Contains(out, "unknown flag: --model") {
		t.Errorf("posse dispatch --model: exit %d, output %q; want a refusal naming the flag", code, out)
	}
}

func runRhq(t *testing.T, bin string, env []string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("posse %s: %v", strings.Join(args, " "), err)
	}
	return string(out), code
}

// modelHelpBlock is the usage catalog's `--model <id>` entry: its own line
// and every continuation indented under it, up to the next flag.
func modelHelpBlock(t *testing.T, out string) string {
	t.Helper()
	// A flag line is the catalog's own six-space indent; its continuations
	// are indented far past that, and one of them opens with `--runtime`, so
	// a trimmed prefix test would end the block at its second line.
	var b strings.Builder
	in := false
	for _, ln := range strings.Split(out, "\n") {
		if strings.HasPrefix(ln, "      --") {
			if in {
				break
			}
			in = strings.HasPrefix(ln, "      --model <id>")
		}
		if in {
			b.WriteString(ln + "\n")
		}
	}
	if b.Len() == 0 {
		t.Fatal("the usage catalog no longer has a `--model <id>` entry — this pin is reading nothing")
	}
	return b.String()
}

// ranger-base-nxf11, finding 1: the operator-visible half of the same defect
// TestExactModelSkipsTierVerdict pins on the shipped line.
//
// The retired sentence told the operator that `--model` skipped a
// substitution the tier availability step used to perform, so a provider
// refusal was the answer rather than a quiet one. (It is not quoted word
// for word on purpose — that would put the scrubbed bytes back into the
// tree the scrub is about, and `git show` on this pin's own commit is where
// a reader gets them.) Every word of it stayed true-looking after ADR 0003
// §3 removed the mechanism (ranger-base-hv2zr), and became a lie by
// contrast: a clause selling the ABSENCE of a mechanism tells the reader
// the mechanism is there for everyone else. Nothing in the tree substitutes
// now, so the sentence promised an ordinary launch a quiet landing it does
// not have.
//
// Both halves, for that reason: what the block must say, and the vocabulary
// of the removed walk, which must not come back into this block by any
// rewording. That vocabulary is namesTheRemovedSubstitution, a FAMILY and not
// a list of forms: a list of whole inflected forms is evaded by any one-word
// respelling of the same meaning ("falls back" does not contain "fall back";
// "substituting" contains neither "substitute" nor "substitution"), which
// left this half green on a reworded help line (ranger-base-g6k5b) — and the
// stems that answered THAT were themselves evaded by the irregular past,
// which contains no "fall" (ranger-base-8v29w; " — an ordinary launch fell
// back, this one does not — " appended to this very entry MEASURED green).
// Grade it by rewording, never by putting the retired sentence back. The
// family does not itself answer a sweep for the retired phrases.
func TestModelHelpNamesTheVerdictNotTheRemovedSubstitution(t *testing.T) {
	block := modelHelpBlock(t, helpText(t))
	for _, want := range []string{"--agent", "--runtime", "--tier", "tier availability verdict"} {
		if !strings.Contains(block, want) {
			t.Errorf("the --model help does not name %q:\n%s", want, block)
		}
	}
	if m := namesTheRemovedSubstitution.FindString(block); m != "" {
		t.Errorf("the --model help still names the removed automatic substitution (%q):\n%s", m, block)
	}
}
