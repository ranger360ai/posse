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
// §8's probe recipe used to read "Help text means the parser bound the PID",
// which is a FALSE PASS on any CLI that answers --help before it parses
// (ranger-base-1fad). The recipe now runs as a probe/control PAIR and names
// the escape; TestInstallSection8ProbeRecipeCarriesAControl pins that.
// Measured 2026-08-27 with /tmp/p.md = "---\nname: x\n---\nhello\n":
//
//	grok 1.0.5     --permission-mode auto --nosuchflag=zzz --help  -> rc=2 unexpected argument
//	codex 0.147.0  -a never --nosuchflag=zzz --help                -> rc=2 unexpected argument
//	claude 2.1.250 --permission-mode auto --nosuchflag=zzz --help  -> rc=0 HELP PRINTS
//	claude 2.1.250 --append-system-promt "$(cat p.md)" --help      -> rc=0 HELP PRINTS
//
// A subcommand has to finish parsing before it can dispatch, so it restores
// the discrimination commander's --help destroys:
//
//	claude --append-system-prompt="$(cat p.md)" mcp list  -> rc=0 "No MCP servers configured."
//	claude --append-system-prompt "$(cat p.md)" mcp list  -> rc=0 "No MCP servers configured."
//	claude --append-system-promt="$(cat p.md)"  mcp list  -> rc=1 unknown option
//	claude --nosuchflag=zzz                     mcp list  -> rc=1 unknown option
//
// Those last four also retire the NOTE on ranger-base-1fad: claude really does
// bind BOTH the glued and the separated form, and now that is evidenced by a
// probe whose wrong arm fails rather than by help text that always prints.
// §8's "glue it unless you have proved the separated form works" still stands
// as advice for a CLI nobody has probed this way. The worked pair itself is
// now glued on BOTH arms (ranger-base-axpt): the shipped pair varied spelling
// AND gluing at once, so a reader could credit the control's failure to the
// separation instead of the typo.
//
// A passing pair still only proves the parser ACCEPTED the value, not that
// the CLI treats it as instructions (ranger-base-axpt) — a real flag with an
// optional argument passes the same probe:
//
//	claude --debug="$(cat p.md)" mcp list  -> rc=0 "No MCP servers configured."
//
// --debug is a logging flag, not the unattended-instructions one; §8 now
// says to check the flag's name against --help before trusting a green pair.

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

// installSection8 returns §8 of INSTALL.md, or fails the test if the section
// has moved — a pin that reads an empty string pins nothing.
func installSection8(t *testing.T) string {
	t.Helper()
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
	return doc[i:j]
}

func TestInstallSection8CarriesThePIDFlagRule(t *testing.T) {
	sec := installSection8(t)

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

// The probe recipe is the one thing in §8 a cold installer *runs*, so it is
// the one thing that can hand them a green light on a flag that binds
// nothing. Pin that it still ships a control arm and still names the escape.
func TestInstallSection8ProbeRecipeCarriesAControl(t *testing.T) {
	sec := installSection8(t)

	probe := strings.Index(sec, "Probe your CLI before trusting it")
	if probe < 0 {
		t.Fatal("INSTALL.md §8: the probe recipe is gone — the pin has stopped reading its subject")
	}
	// Prose reflows; the claims must not. Match on collapsed whitespace so a
	// rewrap cannot break the pin and a deletion still can.
	recipe := strings.Join(strings.Fields(sec[probe:]), " ")

	for _, want := range []struct{ text, why string }{
		{"--nosuchflag=zzz --help", "the control arm of the probe pair"},
		{"the control", "the probe is still presented as a pair"},
		{"a probe only discriminates if its wrong arm fails", "why the control is there at all"},
		{"the probe has proved nothing about", "what a passing control means"},
		{"subcommand", "the repair for a CLI that short-circuits --help"},
		{"--append-system-promt", "the measured commander false pass"},
		{"proves the parser accepted the value, not that the CLI treats it as instructions", "the real-but-wrong-flag caveat"},
		{"--debug=", "the measured optional-argument false pass"},
	} {
		if !strings.Contains(recipe, want.text) {
			t.Errorf("INSTALL.md §8 probe recipe no longer states %s: missing %q (ranger-base-1fad)", want.why, want.text)
		}
	}

	// The old recipe's verdict sentence stood alone: help text, therefore
	// bound. If it ever comes back unconditioned, the false pass is back.
	if strings.Contains(recipe, "Help text means the parser bound the PID") {
		t.Error("INSTALL.md §8: the unconditioned verdict \"Help text means the parser bound the PID\" is back — it is a false pass on claude 2.1.250 (ranger-base-1fad)")
	}

	// The prose above says "repair the probe with a subcommand"; the block
	// below it is the only thing a cold installer can COPY. Verified
	// 2026-08-27 (ranger-base-yuhs) that the assertions above all pass with
	// that block deleted — the words "subcommand" and "--append-system-promt"
	// both survive in the surrounding prose, so the six checks above went on
	// green over a §8 with no runnable repair left in it. A recipe nobody can
	// run is the state ranger-base-1fad was filed about.
	//
	// Both arms, because a repair shown only passing is the undiscriminating
	// probe this very section warns against.
	for _, want := range []struct{ text, why string }{
		{`--append-system-prompt="$(cat /tmp/p.md)" mcp list`, "the repair's PROBE arm, runnable as written"},
		{`--append-system-promt="$(cat /tmp/p.md)" mcp list`, "the repair's CONTROL arm, runnable as written"},
		{"No MCP servers configured", "what the probe arm prints when the flag bound"},
		// "unknown option" alone survives both the glued and the separated
		// misspelling (ranger-base-21n5), so it cannot notice the arm moving
		// back and forth. Pin the glued arm's actual failing token instead:
		// commander echoes the WHOLE value, so gluing with `=` means the PID
		// itself — "name: x", the closing "hello'" — spills into the error.
		{"unknown option '--append-system-promt=---", "the control arm is glued, so its failing token carries the `=` plus the PID's opening line, not just the flag name"},
		{"hello'", "the PID's last line reaching the closing quote — a stale single-line error here means the shown output no longer matches the glued arm"},
	} {
		if !strings.Contains(recipe, want.text) {
			t.Errorf("INSTALL.md §8: the subcommand repair is no longer runnable as written — missing %s: %q (ranger-base-1fad, pinned by ranger-base-yuhs)", want.why, want.text)
		}
	}
}
