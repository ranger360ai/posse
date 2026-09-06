package posse

// Helpers lifted out of gates_test.go so every suite arm compiles them
// (ranger-base-qp1hm). A file with a build tag is absent from the arms it
// does not name, and these declarations have readers in all of them.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// claudeDenyMatch models claude's Bash rule matcher (2.1.234, prefix form
// re-measured on 2.1.241) so the table below can assert what the emitted
// rules DO, not just how they read. Three forms, in the CLI's own order:
// `<c>:*` is a prefix of the argv TOKENS — not of the command string, which
// is what the code below has always implemented and what the comment used
// to misname (ranger-base-g8e: `Bash(sed -n:*)` does not reach
// `sed -ni 1p f.txt`, and does reach `sed -n -i.bak …`) — a rule carrying
// `*` is a wildcard (`*` -> `.*`, anchored both ends, runs of whitespace
// collapsed on both sides), anything else is exact. Verified against the
// real CLI in rangerhq-3mc — the nine option spellings refused, the
// pass-through set left alone.
func claudeDenyMatch(rule, command string) bool {
	if !strings.HasPrefix(rule, "Bash(") || !strings.HasSuffix(rule, ")") {
		return false
	}
	body := strings.TrimSuffix(strings.TrimPrefix(rule, "Bash("), ")")
	flat := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	switch {
	case strings.HasSuffix(body, ":*"):
		p := flat(strings.TrimSuffix(body, ":*"))
		c := flat(command)
		return c == p || strings.HasPrefix(c, p+" ")
	case strings.Contains(body, "*"):
		var re strings.Builder
		re.WriteString("^")
		for i, lit := range strings.Split(flat(body), "*") {
			if i > 0 {
				re.WriteString(".*")
			}
			re.WriteString(regexp.QuoteMeta(lit))
		}
		re.WriteString("$")
		return regexp.MustCompile(re.String()).MatchString(flat(command))
	default:
		return body == command
	}
}

func deniedByAny(rules []string, command string) bool {
	for _, r := range rules {
		if claudeDenyMatch(r, command) {
			return true
		}
	}
	return false
}

// grokDenyMatch models grok's Bash deny matcher (1.0.5, probed live in
// rangerhq-625). It is claude's dialect in outline — a `*` really is a
// wildcard over the whole command — and diverges in three places, each of
// which was verified rather than read off the shipped docs:
//
//   - grok splits the command like a shell before matching, so what reaches
//     the matcher is a *re-rendered* segment: quotes gone, runs of
//     whitespace collapsed. `git -C <r> log --author "push me"` is refused
//     by `Bash(git -* push *)` there, and claude runs it.
//   - `:*` is a plain prefix with NO word boundary: `Bash(git push:*)`
//     refuses `git pushy --help`, which claude leaves alone.
//   - a rule with no wildcard is a *prefix*, not the exact match it is on
//     claude: `Bash(sha1sum)` refuses `sha1sum --version` unaided.
//
// `[...]` classes are supported by grok and not modelled here; nothing the
// fleet emits uses one.
func grokDenyMatch(rule, command string) bool {
	if !strings.HasPrefix(rule, "Bash(") || !strings.HasSuffix(rule, ")") {
		return false
	}
	body := strings.TrimSuffix(strings.TrimPrefix(rule, "Bash("), ")")
	c := grokSegment(command)
	switch {
	case strings.HasSuffix(body, ":*"):
		return strings.HasPrefix(c, grokSegment(strings.TrimSuffix(body, ":*")))
	case strings.ContainsAny(body, "*?"):
		var re strings.Builder
		re.WriteString("^")
		for _, ch := range body {
			switch ch {
			case '*':
				re.WriteString(".*")
			case '?':
				re.WriteString(".")
			default:
				re.WriteString(regexp.QuoteMeta(string(ch)))
			}
		}
		re.WriteString("$")
		return regexp.MustCompile(re.String()).MatchString(c)
	default:
		return strings.HasPrefix(c, body)
	}
}

// grokSegment renders a command the way grok's splitter hands it to the
// matcher: shell-parsed and re-joined, so quote characters are gone and
// runs of whitespace are one space. Quotes: a quoted `"push me"` matched
// `Bash(git -* push *)` live (rangerhq-625). Collapse: isolated in
// rangerhq-b8i with a rule that has no wildcard — `Bash(git log)` refused
// `git  log --oneline` (two spaces) live on grok 1.0.5. The earlier
// two-space-vs-wildcard probe did not isolate this (rangerhq-2uc4).
func grokSegment(s string) string {
	var toks []string
	var cur strings.Builder
	var quote rune
	open := false
	for _, ch := range s {
		switch {
		case quote != 0:
			if ch == quote {
				quote = 0
			} else {
				cur.WriteRune(ch)
			}
		case ch == '\'' || ch == '"':
			quote, open = ch, true
		case ch == ' ' || ch == '\t':
			if open {
				toks, open = append(toks, cur.String()), false
				cur.Reset()
			}
		default:
			cur.WriteRune(ch)
			open = true
		}
	}
	if open {
		toks = append(toks, cur.String())
	}
	return strings.Join(toks, " ")
}

func grokDeniedByAny(rules []string, command string) bool {
	for _, r := range rules {
		if grokDenyMatch(r, command) {
			return true
		}
	}
	return false
}

// renderGateShellFor renders a persona's gates with $SHELL pointed at a
// fake shell of the given basename, and returns the wrapper's path, the
// gates dir and the bin dir the guard prepends.
func renderGateShellFor(t *testing.T, base, body string) (wrapper, gatesDir, binDir string) {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	fakeDir := t.TempDir()
	fake := filepath.Join(fakeDir, base)
	if err := os.WriteFile(fake, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", fake)
	gatesDir, binDir, wrapper, err := a.RenderGates("developer", []string{"Bash(git push:*)"})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(wrapper) != base {
		t.Fatalf("wrapper must be named %q so a runtime picks the right dialect: %s", base, wrapper)
	}
	if b, _ := os.ReadFile(wrapper); !strings.Contains(string(b), "REAL='"+fake+"'") {
		t.Fatalf("wrapper must exec the resolved real shell:\n%s", b)
	}
	return wrapper, gatesDir, binDir
}

// installCommitGuard installs the prepare-commit-msg guard with no config
// behind it, which is the fail-closed default: unmarked repo = public beads
// db = the visibility half armed. Every test below commits files outside
// `.beads/`, so what they exercise is the shared-index half.
func installCommitGuard(dir string) (string, error) {
	p, _, _, err := (&App{}).InstallCommitGuardHook(dir)
	return p, err
}

// testIdentity is the ADR 0024 D2 check 3 literal set InstallCommitGuardHook
// / probeL3Hooks would derive for dir on THIS box, right now — for a test
// that constructs a byte-exact CommitGuardHook fixture instead of installing
// one, and needs it to agree with what an install (or a probe) would
// actually write. Fatal on a derivation error: a test fixture that cannot
// even be compared is a fixture worth stopping on, not silently trusting.
func testIdentity(t *testing.T, dir string) []IdentityLiteral {
	t.Helper()
	lits, err := DeriveIdentityLiterals(hookRepo(dir))
	if err != nil {
		t.Fatalf("DeriveIdentityLiterals(%s): %v", dir, err)
	}
	return lits
}

// firstStampLine is the one line of a rendered hook that says which way it
// was stamped — the whole hook in an error message buries it.
func firstStampLine(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "posse_beads_visibility=") {
			return line
		}
	}
	return "(no stamp line)"
}
