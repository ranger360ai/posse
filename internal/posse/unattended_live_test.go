//go:build posse_arm2

package posse

// The residual risk rangerhq-qs5r could not close, made loud (rangerhq-beby).
//
// qs5r fixed the mode by TYPING it, because the CLI's own default moved
// under the fleet once already. But a typed value is only as good as the
// CLI's vocabulary, and that moves too: claude has already RETIRED
// `default` — it is gone from --help, yet still accepted, silently mapping
// to `manual`. Measured on claude 2.1.240: `--permission-mode default`
// starts a session whose footer reads "⏸ manual mode on". Nothing errors.
//
// So the day `auto` is retired the way `default` was, every fleet session
// lands back in manual with no error, no failing string test, and a pane
// footer that cannot tell the difference — the footer reads "auto mode on"
// for a session launched with NO flag at all, because the CLI's default is
// auto today. An operator-driven pane is the standing proof of that.
//
// This is the check that discriminates: ask each runtime's real CLI what it
// will accept, and fail if the value posse types is not on the list. It costs
// no tokens — an invalid value is rejected during argument parsing, before
// any session starts — and it skips where the CLI is not installed.
//
// TWO SHAPES OF UNATTENDED FLAG, and the second arrived with bob
// (ranger-base-ymmiv). `--permission-mode auto` and `-a never` carry a
// VALUE, so the question is whether the vocabulary still holds it and the
// rejection above is how you ask. `--auto-approve` is a BOOLEAN: there is no
// value to retire, and the failure one layer over is that the FLAG goes —
// which on bob is worse than on the others, because bobshell is built with
// allowUnknownOption() and swallows a retired flag in silence rather than
// refusing the launch. So the boolean arm asks the CLI's own help whether it
// still names the flag. Same cost (help only, no session), same skip when
// the CLI is absent, same failure sentence: posse is typing something this
// CLI no longer does anything with.
import (
	"context"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"
)

// notInVocabulary reports whether want is absent from out as a whole token.
// Substring matching would let a renamed `autoAccept` vouch for a retired
// `auto` — the exact failure this test exists to catch.
func notInVocabulary(out, want string) bool {
	for _, tok := range strings.FieldsFunc(out, func(r rune) bool {
		return !(r == '-' || r == '_' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'))
	}) {
		if tok == want {
			return false
		}
	}
	return true
}

func TestCLIStillKnowsTheMode(t *testing.T) {
	t.Parallel()
	for _, rt := range builtinRuntimes {
		if rt.Unattended == "" {
			continue // TestEveryBuiltinTemplateIsUnattended owns that failure
		}
		f := strings.Fields(rt.Unattended)
		switch len(f) {
		case 1:
			booleanFlagStillOffered(t, rt, f[0])
			continue
		case 2:
		default:
			t.Errorf("%s: cannot probe %q — expected `<flag>` or `<flag> <value>`", rt.Name, rt.Unattended)
			continue
		}
		flag, want := f[0], f[1]
		t.Run(rt.Name, func(t *testing.T) {
			exe := rt.Exe()
			if _, err := exec.LookPath(exe); err != nil {
				t.Skipf("no %s on this host — the vocabulary can only be asked of the real CLI", exe)
			}
			// A value no CLI will ever adopt, so the answer is always the
			// rejection that lists what it does accept.
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, exe, flag, "posse-not-a-mode")
			cmd.Stdin = strings.NewReader("")
			out, _ := cmd.CombinedOutput()
			got := string(out)
			if ctx.Err() != nil {
				t.Fatalf("%s did not reject %s posse-not-a-mode and had to be killed — it may not validate "+
					"this flag at all, so nothing here vouches for %q:\n%s", exe, flag, rt.Unattended, got)
			}
			if !strings.Contains(strings.ToLower(got), "invalid") {
				t.Fatalf("%s %s posse-not-a-mode was not rejected, so this probe proves nothing "+
					"about %q:\n%s", exe, flag, rt.Unattended, got)
			}
			if notInVocabulary(got, want) {
				t.Errorf("%s no longer offers %s %s — every %s session posse launches is now taking "+
					"whatever that value degrades to, silently (see the file comment; claude retired "+
					"`default` exactly this way). The CLI accepts:\n%s",
					exe, flag, want, rt.Name, got)
			}
		})
	}
}

// plainWord matches a bare subcommand token in a launch template: no flag,
// no placeholder, no shell. It is how the boolean probe finds `chat` in
// bob's line without parsing the template's flag grammar, which is the part
// that differs per CLI and would rot.
var plainWord = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// booleanFlagStillOffered is the boolean arm: the CLI's own help must still
// NAME the flag posse types.
//
// It asks the help of the command the TEMPLATE runs, not just of the
// program, because a CLI with a subcommand keeps its flags on the
// subcommand — `bob chat --help` lists --auto-approve and `bob --help` does
// not. The candidates are the template's plain words rather than a per-CLI
// constant, so this needs no edit when a fifth built-in arrives with a
// different subcommand; a template with no subcommand has no plain words and
// the program's own help is the whole walk.
//
// A candidate whose help cannot be read is simply not an answer — the walk
// moves on — so the only way to fail is for NO help on the CLI to name the
// flag. That direction is the safe one: a false red sends a maintainer to
// read a --help, and a false green would be posse typing into the void.
func booleanFlagStillOffered(t *testing.T, rt Runtime, flag string) {
	t.Helper()
	t.Run(rt.Name, func(t *testing.T) {
		exe := rt.Exe()
		if _, err := exec.LookPath(exe); err != nil {
			t.Skipf("no %s on this host — the vocabulary can only be asked of the real CLI", exe)
		}
		// THE CONTROL, and it runs first: a flag no CLI will ever offer must
		// come back absent. Without it a help that could not be read at all
		// — a program that printed nothing, a --help that changed shape —
		// would pass this test for every flag, which is the green-over-
		// nothing this file exists to refuse one layer up.
		if helpMentions(t, exe, "--posse-not-a-flag") {
			t.Fatalf("%s --help names a flag that does not exist, so nothing here vouches for %s", exe, flag)
		}
		tried := []string{exe + " --help"}
		if helpMentions(t, exe, flag) {
			return
		}
		for _, w := range strings.Fields(rt.Command) {
			if w == exe || !plainWord.MatchString(w) {
				continue
			}
			tried = append(tried, exe+" "+w+" --help")
			if helpMentions(t, exe, w, flag) {
				return
			}
		}
		t.Errorf("%s no longer names %s in any help posse can reach (tried: %s) — every %s session "+
			"posse launches is now typing a flag the CLI does nothing with, and on a program built "+
			"with allowUnknownOption() that is SILENT: the pane comes up asking for approvals with "+
			"nobody watching. Re-read the CLI's help and move Unattended, or drop it and let "+
			"TestEveryBuiltinTemplateIsUnattended say the runtime has none",
			exe, flag, strings.Join(tried, ", "), rt.Name)
	})
}

// helpMentions runs one `--help` and reports whether flag appears in it as a
// whole token. Whole-token, for notInVocabulary's reason: a substring match
// would let `--auto-approve-edits` vouch for a retired `--auto-approve`.
func helpMentions(t *testing.T, exe string, argv ...string) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	want := argv[len(argv)-1]
	cmd := exec.CommandContext(ctx, exe, append(append([]string{}, argv[:len(argv)-1]...), "--help")...)
	cmd.Stdin = strings.NewReader("")
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Logf("%s %v --help had to be killed; not an answer either way", exe, argv[:len(argv)-1])
		return false
	}
	if err != nil && len(out) == 0 {
		return false
	}
	return !notInVocabulary(string(out), want)
}
