package posse

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// QA pins for rangerhq-xjyl (verified under rangerhq-vx6j).
//
// Claim: every PID opens with `---` (ADR 0001 frontmatter), so a clap-based
// CLI reads it as an option and refuses to start. The PID must therefore ride
// in a flag's VALUE, never as a bare positional. Measured on the live CLIs:
//
//	grok 1.0.5    --permission-mode auto "$(cat p.md)"           -> rc=2 unexpected argument '---
//	grok 1.0.5    --permission-mode auto --rules "$(cat p.md)"   -> rc=2 unexpected argument '---
//	grok 1.0.5    --permission-mode auto --rules="$(cat p.md)"   -> rc=0 help prints
//	codex 0.147.0 -a never "$(cat p.md)"                         -> rc=2 unexpected argument '---
//	codex 0.147.0 -a never -c developer_instructions="$(cat p.md)" -> rc=0
//
// The pins below are the two halves that can regress silently: a built-in
// template edited into the positional form, and §8 losing the rule a cold
// installer reads before writing a template profile of their own.
//
// NOT pinned here: §8's probe recipe ("Help text means the parser bound the
// PID") is a FALSE PASS on any CLI whose --help short-circuits parsing —
// claude 2.1.250 prints help for a misspelled flag. That is ranger-base-1fad;
// its control-probe pin belongs with the fix, not born red here.

// pidInFlagValue reports whether the PID placeholder in a command template
// sits inside a flag's value. Two shapes qualify: glued (`--rules=…`,
// `-c developer_instructions=…`), where the text runs up to an `=`, and
// separated (`--append-system-prompt …`), where the preceding word is a flag.
// A bare positional — the preceding word is not a flag — does not.
func pidInFlagValue(cmd string) bool {
	i := strings.Index(cmd, "{file}")
	if i < 0 {
		return false
	}
	// Everything left of the PID, minus the `"$(cat ` that reads it.
	head := catWrapper.ReplaceAllString(cmd[:i], "")
	if strings.HasSuffix(head, "=") {
		return true
	}
	if !strings.HasSuffix(head, " ") {
		return false
	}
	fields := strings.Fields(head)
	return len(fields) > 0 && strings.HasPrefix(fields[len(fields)-1], "-")
}

var catWrapper = regexp.MustCompile(`["']?\$\(\s*cat\s+$`)

func TestPIDPlaceholderChecksDiscriminate(t *testing.T) {
	// Without this the pin below could pass on any string at all.
	ok := []string{
		`claude --append-system-prompt "$(cat {file})" --add-dir {memory}`,
		`grok --permission-mode auto --rules="$(cat {file})"`,
		`codex -a never -c developer_instructions="$(cat {file})"`,
		`x --rules='$(cat {file})'`,
	}
	bad := []string{
		`grok --permission-mode auto "$(cat {file})"`, // the bead's REPRO
		`codex -a never $(cat {file})`,
		`claude {model} --add-dir {memory}`, // no PID at all
	}
	for _, c := range ok {
		if !pidInFlagValue(c) {
			t.Errorf("flag form rejected: %s", c)
		}
	}
	for _, c := range bad {
		if pidInFlagValue(c) {
			t.Errorf("positional form accepted: %s", c)
		}
	}
}

var cmdTemplate = regexp.MustCompile("(?m)^\\s*(?:Command|const DefaultAgentCommand =)\\s*:?\\s*(`.*)$")

func TestBuiltinTemplatesPassThePIDInAFlagValue(t *testing.T) {
	for _, path := range []string{"internal/rhq/runtime.go", "internal/rhq/agents.go"} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		found := 0
		for _, m := range cmdTemplate.FindAllStringSubmatch(string(b), -1) {
			line := m[1]
			if !strings.Contains(line, "{file}") {
				continue // e.g. the template-only loader's own Command field
			}
			found++
			if !pidInFlagValue(line) {
				t.Errorf("%s: PID is a bare positional — every PID opens with `---` and a clap CLI refuses it (rangerhq-xjyl):\n  %s", path, line)
			}
		}
		if found == 0 {
			t.Errorf("%s: no command template carrying {file} found — the pin has stopped reading its subject", path)
		}
	}
}

func TestInstallSection8CarriesThePIDFlagRule(t *testing.T) {
	b, err := os.ReadFile("INSTALL.md")
	if err != nil {
		t.Fatalf("read INSTALL.md: %v", err)
	}
	doc := string(b)
	i := strings.Index(doc, "## 8. A launch profile of your own")
	j := strings.Index(doc, "## 9. ")
	if i < 0 || j < 0 || j < i {
		t.Fatal("INSTALL.md: §8 not found — the pin has stopped reading its subject")
	}
	sec := doc[i:j]

	// The rule itself, and why it exists.
	for _, want := range []string{
		"never as a positional",
		"Every\n   PID opens with `---`",
		"unexpected argument",
	} {
		if !strings.Contains(sec, want) {
			t.Errorf("INSTALL.md §8 no longer states the PID-in-a-flag rule: missing %q (rangerhq-xjyl)", want)
		}
	}

	// Each built-in dialect a reader copies from, in the flag form that works.
	for _, dialect := range []string{
		`--append-system-prompt "$(cat {file})"`,
		`-c developer_instructions="$(cat {file})"`,
		`--rules="$(cat {file})"`,
	} {
		if !strings.Contains(sec, dialect) {
			t.Errorf("INSTALL.md §8 no longer shows the working dialect %s", dialect)
		}
	}

	// A cold installer meets {file} in the template comment before the
	// caveats; that is where the cross-reference has to be.
	if !strings.Contains(sec, "never a bare\n#             positional") {
		t.Error("INSTALL.md §8: the {file} placeholder comment no longer points at the flag rule")
	}
}
