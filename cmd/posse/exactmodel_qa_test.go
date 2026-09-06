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
	"strings"
	"testing"
)

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
// rewording. The banned entries are STEMS, matched against a lower-cased
// block: a list of whole inflected forms is evaded by any one-word respelling
// of the same meaning ("falls back" does not contain "fall back";
// "substituting" contains neither "substitute" nor "substitution"), which
// left this half green on a reworded help line (ranger-base-g6k5b). Grade it
// by rewording, never by putting the retired sentence back. The stems do not
// themselves answer a sweep for the retired phrases.
func TestModelHelpNamesTheVerdictNotTheRemovedSubstitution(t *testing.T) {
	block := modelHelpBlock(t, helpText(t))
	for _, want := range []string{"--agent", "--runtime", "--tier", "tier availability verdict"} {
		if !strings.Contains(block, want) {
			t.Errorf("the --model help does not name %q:\n%s", want, block)
		}
	}
	for _, gone := range []string{"substitut", "fall"} {
		if strings.Contains(strings.ToLower(block), gone) {
			t.Errorf("the --model help still names the removed automatic substitution (%q):\n%s", gone, block)
		}
	}
}
