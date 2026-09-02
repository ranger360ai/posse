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
import (
	"context"
	"os/exec"
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
		if len(f) != 2 {
			t.Errorf("%s: cannot probe %q — expected `<flag> <value>`", rt.Name, rt.Unattended)
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
