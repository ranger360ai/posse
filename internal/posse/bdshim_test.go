//go:build posse_arm2

package posse

// The bd half of the L1 shim's option-aware verb match (ranger-base-3bqn).
//
// A `Bash(bd <verb>:*)` permission rule matches a token prefix of the typed
// line, so `bd --no-daemon daemon --help` walks past it (MEASURED on az93).
// The shim is the layer that does not have that hole — but only where
// globalValueOpts knows which of the command's global options eat the next
// word. bd has four. Without them `bd --db /tmp/x daemon stop` resolves to
// the verb `/tmp/x`.
//
// The two tables below pin the entry from both sides, so deleting it fails
// this test twice: `--db /x daemon stop` stops being refused, and `--actor
// daemon show x` — whose OPTION VALUE is the word `daemon` — starts being.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBdShimResolvesTheVerbBehindGlobalOptions(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	realBin := t.TempDir()
	if err := os.WriteFile(filepath.Join(realBin, "bd"), []byte("#!/bin/sh\necho \"real bd $*\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", realBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, binDir, _, err := a.RenderGates("devops", []string{"Bash(bd daemon:*)"})
	if err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(binDir, "bd")
	run := func(args ...string) (string, string, int) {
		cmd := exec.Command(shim, args...)
		var out, errb strings.Builder
		cmd.Stdout, cmd.Stderr = &out, &errb
		err := cmd.Run()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
		return out.String(), errb.String(), code
	}

	// Every spelling that reaches `bd daemon`, including the one the fleet
	// is taught to type and the four options that take a separate value.
	refused := [][]string{
		{"daemon", "stop"},
		{"--no-daemon", "daemon", "--help"},
		{"--json", "--no-daemon", "daemon", "stop"},
		{"--db", "/tmp/x", "daemon", "stop"},
		{"--db=/tmp/x", "daemon", "stop"},
		{"--actor", "someone", "daemon", "stop"},
		{"--lock-timeout", "5s", "daemon", "stop"},
		{"--dolt-auto-commit", "on", "daemon", "stop"},
		{"--allow-stale", "--no-auto-flush", "--sandbox", "daemon"},
		{"-q", "-v", "daemon", "killall"},
	}
	for _, args := range refused {
		out, errs, code := run(args...)
		want := "refused by posse gate: bd " + strings.Join(args, " ") + " (deny: Bash(bd daemon:*))"
		if code != 1 || !strings.Contains(errs, want) || out != "" {
			t.Errorf("bd %s must be refused: code=%d out=%q err=%q", strings.Join(args, " "), code, out, errs)
		}
	}

	// …and the shim stays out of the way otherwise. The middle rows are the
	// other half of the entry: `daemon` as an option's VALUE is not the verb.
	passed := [][]string{
		{"show", "ranger-base-3bqn"},
		{"--no-daemon", "ready", "--json"},
		{"--actor", "daemon", "show", "x"},
		{"--db", "daemon", "list"},
		{"--lock-timeout", "daemon", "status"},
		{"create", "daemon", "-t", "task"},
		{"daemons", "list"}, // a different word: the shim matches tokens
	}
	for _, args := range passed {
		out, errs, code := run(args...)
		if code != 0 || strings.TrimSpace(out) != "real bd "+strings.Join(args, " ") {
			t.Errorf("bd %s must pass through: code=%d out=%q err=%q", strings.Join(args, " "), code, out, errs)
		}
	}

	// A dangling value option is bd's own usage error, not the shim's.
	if _, _, code := run("--db"); code != 0 {
		t.Errorf("dangling --db must reach the real bd: code=%d", code)
	}
}

// And what parity is allowed to say about it: option-aware, so the rule is
// matched faithfully — which is what the entry buys. `daemons` is a second
// rule, not something the first one covers, and the hidden-verb class needs
// the allow-list gate (scripts/bd-argv-gate.py), not this.
func TestBdRuleIsOptionAwareForParity(t *testing.T) {
	t.Parallel()
	rules := ParseShimRules([]string{"Bash(bd daemon:*)"})["bd"]
	if len(rules) != 1 {
		t.Fatalf("want one bd rule, got %+v", rules)
	}
	kind, faithful := matcherFor("bd", rules[0])
	if kind != "subcommand, option-aware" || !faithful {
		t.Errorf("bd rules must be option-aware for parity, got %q faithful=%v", kind, faithful)
	}
}

// The flag half of the same miss (ranger-base-vct2). `Bash(bd sync
// --full:*)` rendered as `$1 = sync && $2 = --full`, and cobra gives a flag
// no position: `bd sync --push --full` and `bd sync --dry-run --full` both
// RAN past the wall (MEASURED on the rendered shim). `bd sync --full` is
// the one spelling of sync that commits AND pushes, and on grok and codex
// the shim is the only bd fence there is.
//
// Pinned from both sides, so a membership scan that forgets verbValueOpts
// fails this test just as loudly as a return to the positional match: the
// refuse arms below are the flag wherever it sits, and the pass arms are
// the words that only LOOK like it — an option's value, an operand after
// `--`, another verb.
func TestBdSyncFullIsRefusedWhereverTheFlagSits(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	realBin := t.TempDir()
	if err := os.WriteFile(filepath.Join(realBin, "bd"), []byte("#!/bin/sh\necho \"real bd $*\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", realBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, binDir, _, err := a.RenderGates("devops", []string{"Bash(bd sync --full:*)"})
	if err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(binDir, "bd")
	run := func(args ...string) (string, string, int) {
		cmd := exec.Command(shim, args...)
		var out, errb strings.Builder
		cmd.Stdout, cmd.Stderr = &out, &errb
		err := cmd.Run()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
		return out.String(), errb.String(), code
	}

	// Every spelling that reaches a full sync. Rows 2-6 are flags real to
	// `bd sync` (MEASURED, `bd sync --help`, bd 0.49.1) sitting in front of
	// the denied one; row 7 puts the denied one first with more behind it.
	refused := [][]string{
		{"sync", "--full"},
		{"sync", "--dry-run", "--full"},
		{"sync", "--no-push", "--full"},
		{"sync", "--force", "--check", "--full"},
		{"sync", "--squash", "--full"},
		{"sync", "--full", "--dry-run"},
		{"--no-daemon", "sync", "--dry-run", "--full"},
		{"--db", "/tmp/x", "sync", "--check", "--full"},
		// A value-taking option whose value is NOT the flag: consumed as a
		// pair, and the flag behind it is still found.
		{"sync", "-m", "msg", "--full"},
		{"sync", "--message=msg", "--full"},
		{"sync", "--set-mode", "realtime", "--full"},
		// `--full=true` is the same flag to pflag (MEASURED: `bd list
		// --limit 1 --json=true` prints the JSON `--json` does), and the
		// positional matcher missed this one too.
		{"sync", "--full=true"},
		// The stated cost of matching `--flag=*`: `--full=false` is not a
		// full sync and is refused anyway. One respelling away, which is the
		// cheap error of this class.
		{"sync", "--full=false"},
	}
	for _, args := range refused {
		out, errs, code := run(args...)
		want := "refused by posse gate: bd " + strings.Join(args, " ") + " (deny: Bash(bd sync --full:*))"
		if code != 1 || !strings.Contains(errs, want) || out != "" {
			t.Errorf("bd %s must be refused: code=%d out=%q err=%q", strings.Join(args, " "), code, out, errs)
		}
	}

	// …and the words that only look like the flag. The three -m/--message/
	// --set-mode rows are verbValueOpts pinned from its own side: drop the
	// entry and these start being refused.
	passed := [][]string{
		{"sync"},
		{"sync", "--dry-run"},
		{"sync", "--status"},
		{"-m", "--full"},         // not even the verb: bd's own usage error
		{"sync", "-m", "--full"}, // the commit message is the word `--full`
		{"sync", "--message", "--full"},
		{"sync", "--set-mode", "--full"},
		{"sync", "--", "--full"},         // pflag stops at `--`: an operand, not a flag
		{"sync", "--message"},            // dangling value option: bd's usage error, not ours
		{"list", "--full"},               // a different verb
		{"create", "x", "--full"},        // ditto
		{"--actor", "sync", "show", "x"}, // `sync` as a global option's value
	}
	for _, args := range passed {
		out, errs, code := run(args...)
		if code != 0 || strings.TrimSpace(out) != "real bd "+strings.Join(args, " ") {
			t.Errorf("bd %s must pass through: code=%d out=%q err=%q", strings.Join(args, " "), code, out, errs)
		}
	}
}

// Denying a flag that itself takes a value. The membership scan consumes
// the subcommand's value-taking options in pairs, so a rule naming one of
// THOSE would have its own flag shifted past as somebody's value and refuse
// nothing at all — verbValueOptsFor drops the denied option from the
// pairing. No rule in ADR 0015 §3 has this shape today; without a pin the
// branch is a comment.
func TestBdRuleOnAValueTakingFlagStillRefuses(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	realBin := t.TempDir()
	if err := os.WriteFile(filepath.Join(realBin, "bd"), []byte("#!/bin/sh\necho \"real bd $*\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", realBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, binDir, _, err := a.RenderGates("devops", []string{"Bash(bd sync --set-mode:*)"})
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"sync", "--set-mode", "realtime"},
		{"sync", "--dry-run", "--set-mode", "realtime"},
		{"sync", "--set-mode=realtime"},
	} {
		cmd := exec.Command(filepath.Join(binDir, "bd"), args...)
		var out, errb strings.Builder
		cmd.Stdout, cmd.Stderr = &out, &errb
		if err := cmd.Run(); err == nil {
			t.Errorf("bd %s must be refused: out=%q err=%q", strings.Join(args, " "), out.String(), errb.String())
		}
	}
	// The other verb's value option is still paired, so this is not it.
	cmd := exec.Command(filepath.Join(binDir, "bd"), "sync", "-m", "--set-mode")
	if err := cmd.Run(); err != nil {
		t.Errorf("`bd sync -m --set-mode` is a commit message and must pass: %v", err)
	}
}

// What parity may claim about a flag rule, and what it may not. The `bd
// sync --full` shape is realized: the flag is refused wherever it sits. A
// rule with a SHORT flag is not: `-f` clusters into `-qf`, which the shim
// walks past, so it stays best-effort rather than being claimed — with the
// reason that names the cluster, not the canned global-option one, which
// would send the reader to the wrong table (shimRule.Lead, matcherWhy).
func TestBdFlagRuleParityClaim(t *testing.T) {
	t.Parallel()
	long := ParseShimRules([]string{"Bash(bd sync --full:*)"})["bd"][0]
	kind, faithful := matcherFor("bd", long)
	if kind != "subcommand, option-aware, flag anywhere in the segment" || !faithful {
		t.Errorf("bd sync --full must be claimed flag-anywhere, got %q faithful=%v", kind, faithful)
	}
	short := ParseShimRules([]string{"Bash(bd sync -f:*)"})["bd"][0]
	if kind, faithful := matcherFor("bd", short); faithful {
		t.Errorf("a short-flag rule must not be claimed faithful, got %q", kind)
	} else if why := matcherWhy("bd", short); !strings.Contains(why, "clusters") {
		t.Errorf("a short-flag rule must be explained by the cluster, got %q", why)
	}
	// And a rule with no flag at all keeps the matcher it had.
	verb := ParseShimRules([]string{"Bash(bd daemon:*)"})["bd"][0]
	if kind, faithful := matcherFor("bd", verb); kind != "subcommand, option-aware" || !faithful {
		t.Errorf("a plain verb rule must be unchanged, got %q faithful=%v", kind, faithful)
	}
}
