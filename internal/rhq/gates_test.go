package rhq

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestParseShimRules(t *testing.T) {
	rules := ParseShimRules([]string{
		"Bash(git push:*)", "Bash(git push --force:*)", "Bash(rm -rf /)", "Bash(bd)",
		"Edit", "Write", "WebFetch", "mcp__x__y", "Bash(../evil:*)",
	})
	if len(rules) != 3 || len(rules["git"]) != 2 || len(rules["rm"]) != 1 || len(rules["bd"]) != 1 {
		t.Fatalf("grouping: %+v", rules)
	}
	if r := rules["git"][1]; strings.Join(r.Words, " ") != "push --force" || r.Exact {
		t.Errorf("prefix rule: %+v", r)
	}
	if r := rules["rm"][0]; strings.Join(r.Words, " ") != "-rf /" || !r.Exact {
		t.Errorf("exact rule: %+v", r)
	}
	if r := rules["bd"][0]; len(r.Words) != 0 || r.Exact {
		t.Errorf("whole-verb rule: %+v", r)
	}
}

// Bash(security:*) is the crew-wide keychain-CLI tripwire (ranger-base-khu).
// The PID spelling is `:*` with no extra words; that must parse as the whole
// verb (same as Bash(bd)), or the rendered shim is not an unconditional refuse.
func TestParseShimRulesSecurityStarIsWholeVerb(t *testing.T) {
	rules := ParseShimRules([]string{"Bash(git push:*)", "Bash(security:*)"})
	r, ok := rules["security"]
	if !ok || len(r) != 1 {
		t.Fatalf("security rule missing: %+v", rules)
	}
	if len(r[0].Words) != 0 || r[0].Exact || r[0].Rule != "Bash(security:*)" {
		t.Errorf("Bash(security:*) must be whole-verb: %+v", r[0])
	}
	if kind, faithful := matcherFor("security", r[0]); kind != "whole verb" || !faithful {
		t.Errorf("matcherFor: %q %v", kind, faithful)
	}
	got := L0Spellings([]string{"Bash(security:*)"})
	if strings.Join(got, " ") != "Bash(security:*)" {
		t.Errorf("L0Spellings must not duplicate an already-starred whole verb: %q", got)
	}
}

func TestRenderedSecurityShimRefusesEveryArgv(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	leaked := filepath.Join(t.TempDir(), "leaked")
	realBin := t.TempDir()
	os.WriteFile(filepath.Join(realBin, "security"), []byte("#!/bin/sh\necho LEAK >>'"+leaked+"'\n"), 0o755)
	t.Setenv("PATH", realBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	gatesDir, binDir, _, err := a.RenderGates("qa", []string{"Bash(git push:*)", "Bash(security:*)"})
	if err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(binDir, "security")
	body, _ := os.ReadFile(shim)
	if !strings.Contains(string(body), "if true; then") || !strings.Contains(string(body), "posse_refuse") {
		t.Errorf("whole-verb shim must refuse unconditionally:\n%s", body)
	}
	run := func(args ...string) (string, int) {
		cmd := exec.Command(shim, args...)
		out, err := cmd.CombinedOutput()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("run %v: %v", args, err)
		}
		return string(out), code
	}
	for _, args := range [][]string{
		{},
		{"help"},
		{"-h"},
		{"find-generic-password", "-s", "Claude Code-credentials", "-w"},
		{"dump-keychain"},
	} {
		out, code := run(args...)
		if code != 1 || !strings.Contains(out, "deny: Bash(security:*)") || !strings.Contains(out, "refused by posse gate: security") {
			t.Errorf("argv %q: code=%d out=%q", args, code, out)
		}
	}
	if _, err := os.Stat(leaked); err == nil {
		t.Error("shim exec'd the real security binary")
	}
	logb, _ := os.ReadFile(filepath.Join(gatesDir, "refusals.log"))
	if !strings.Contains(string(logb), "find-generic-password") || !strings.Contains(string(logb), "deny: Bash(security:*)") {
		t.Errorf("refusals.log: %q", logb)
	}
}

// The operator's own terminal IS a persona pane — the `!` prefix runs in the
// current session, whose PATH leads with that persona's shim dir — so the
// crew's keychain tripwire refuses HIS credential read too, and the line used
// to name the rule and stop (ranger-base-kn99, raised on ranger-base-okbr).
// It now says where the command does run. A table and not a blanket: the git
// shim must NOT carry it, or a refusal aimed at the persona reads as the
// escape ranger-base-khu declined on purpose.
func TestRefusalNamesWhereTheCommandDoesRun(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	// Stubs, so an escape from any arm below leaks into a file instead of
	// running the real `security` or a real `git push`.
	leaked := filepath.Join(t.TempDir(), "leaked")
	realBin := t.TempDir()
	for _, c := range []string{"security", "git"} {
		os.WriteFile(filepath.Join(realBin, c), []byte("#!/bin/sh\necho LEAK "+c+" >>'"+leaked+"'\n"), 0o755)
	}
	t.Setenv("PATH", realBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, binDir, _, err := a.RenderGates("developer", []string{"Bash(security:*)", "Bash(git push:*)", "Bash(git commit unless --)"})
	if err != nil {
		t.Fatal(err)
	}
	run := func(cmd string, args ...string) string {
		out, err := exec.Command(filepath.Join(binDir, cmd), args...).CombinedOutput()
		ee, ok := err.(*exec.ExitError)
		if !ok || ee.ExitCode() != 1 {
			t.Fatalf("%s %v must be refused with exit 1: %v %s", cmd, args, err, out)
		}
		return string(out)
	}

	// The argv the operator actually typed during the outage.
	sec := run("security", "find-generic-password", "-s", "Claude Code-credentials", "-w")
	for _, want := range []string{
		"refused by posse gate: security find-generic-password",
		"deny: Bash(security:*)",
		"this shell is developer's pane",
		"gate dir leads its PATH",
		"a persona has no way past",
		"operator: run security in a terminal outside posse",
	} {
		if !strings.Contains(sec, want) {
			t.Errorf("the security refusal must say %q, got:\n%s", want, sec)
		}
	}

	// The negative control, with its own witness: git IS gated and IS
	// refused — an absence measured over a shim that refused nothing would
	// pass for the wrong reason.
	push := run("git", "push")
	if !strings.Contains(push, "deny: Bash(git push:*)") {
		t.Fatalf("git push must be refused, or the control measures nothing:\n%s", push)
	}
	commit := run("git", "commit", "-m", "x")
	if !strings.Contains(commit, "safe form: git commit … -- <operand>") {
		t.Fatalf("git commit must still name its safe form:\n%s", commit)
	}
	for _, out := range []string{push, commit} {
		if strings.Contains(out, "outside posse") || strings.Contains(out, "pane") {
			t.Errorf("the where-hint is a table, not a blanket:\n%s", out)
		}
	}
	if _, err := os.Stat(leaked); err == nil {
		b, _ := os.ReadFile(leaked)
		t.Fatalf("a shim exec'd the real binary: %s", b)
	}
}

// Both hints compose rather than one shadowing the other, safe form first:
// the form that is not refused is the answer for whoever typed it, and where
// to run it is the answer only if that was the operator.
func TestRuleHintComposesSafeFormAndWhere(t *testing.T) {
	got := ruleHint("developer", "security", shimRule{Words: []string{"unlock-keychain"}, Unless: "--"})
	lines := strings.Split(got, "\n")
	if len(lines) != 4 {
		t.Fatalf("want the safe form then the three where lines, got %d:\n%s", len(lines), got)
	}
	if !strings.HasPrefix(lines[0], "  safe form: security unlock-keychain … -- <operand>") {
		t.Errorf("safe form must lead: %q", lines[0])
	}
	if !strings.Contains(lines[1], "developer's pane") || !strings.Contains(lines[3], "outside posse") {
		t.Errorf("the where lines must follow: %q", got)
	}
	if h := ruleHint("developer", "git", shimRule{Words: []string{"push"}}); h != "" {
		t.Errorf("a positive rule on a command with no where-hint has no hint: %q", h)
	}
}

// The rendered shim refuses matching argv (message, exit 1, refusals.log)
// and execs the real binary otherwise; PATH prefix on the typed line.
func TestRenderedShimRefusesAndPasses(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	// A fake "real" git on PATH so the shim has something to exec.
	realBin := t.TempDir()
	os.WriteFile(filepath.Join(realBin, "git"), []byte("#!/bin/sh\necho \"real git $*\"\n"), 0o755)
	t.Setenv("PATH", realBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	gatesDir, binDir, _, err := a.RenderGates("security", []string{"Bash(git push:*)", "Bash(git push --force:*)", "Edit", "Bash(bd)"})
	if err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(binDir, "git")
	body, _ := os.ReadFile(shim)
	if !strings.Contains(string(body), "exec '"+filepath.Join(realBin, "git")+"'") {
		t.Errorf("shim must exec the real binary resolved at render time:\n%s", body)
	}
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
	if out, errs, code := run("push", "--force", "origin", "main"); code != 1 || !strings.Contains(errs, "refused by posse gate: git push --force origin main (deny: Bash(git push:*))") || out != "" {
		t.Errorf("git push --force: code=%d out=%q err=%q", code, out, errs)
	}
	if out, _, code := run("status", "-s"); code != 0 || strings.TrimSpace(out) != "real git status -s" {
		t.Errorf("git status must pass through: code=%d out=%q", code, out)
	}
	if out, _, code := run("pushy"); code != 0 || !strings.Contains(out, "real git pushy") {
		t.Errorf("prefix match is by word, not substring: code=%d out=%q", code, out)
	}
	logb, _ := os.ReadFile(filepath.Join(gatesDir, "refusals.log"))
	if !strings.Contains(string(logb), "git push --force origin main (deny: Bash(git push:*))") || strings.Count(string(logb), "\n") != 1 {
		t.Errorf("refusals.log: %q", logb)
	}
	// Whole-verb deny: bd refuses everything (no real bd needed for the
	// refusal path).
	if outb, err := exec.Command(filepath.Join(binDir, "bd"), "ready").CombinedOutput(); err == nil || !strings.Contains(string(outb), "refused by posse gate: bd ready (deny: Bash(bd))") {
		t.Errorf("Bash(bd) must refuse every bd: %v %q", err, outb)
	}
	// Non-Bash denies render no shim; a rule dropped from the PID is gone
	// on the next render; the log survives.
	if _, err := os.Stat(filepath.Join(binDir, "Edit")); err == nil {
		t.Error("Edit is not a shim")
	}
	if _, _, _, err := a.RenderGates("security", []string{"Edit"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(shim); err == nil {
		t.Error("stale shim survived a re-render")
	}
	if _, err := os.Stat(filepath.Join(gatesDir, "refusals.log")); err != nil {
		t.Error("refusals.log must survive re-render")
	}
	// The typed-line prefix: PATH always, SHELL/GROK_SHELL when a gate
	// shell is typed (ADR 0009 §2), neither when the runtime opts out.
	if p := GatePrefix(binDir, ""); p != "PATH='"+binDir+"':\"$PATH\" " {
		t.Errorf("prefix: %q", p)
	}
	if p := GatePrefix(binDir, "/g/zsh"); p != "PATH='"+binDir+"':\"$PATH\" SHELL='/g/zsh' GROK_SHELL='/g/zsh' " {
		t.Errorf("prefix with gate shell: %q", p)
	}
	wrapped, gd, sh, err := a.WrapWithGates("security", &Runtime{Name: "grok"}, []string{"Bash(git push:*)"}, "claude --x")
	base := filepath.Base(sh)
	if err != nil || gd != gatesDir || filepath.Dir(sh) != filepath.Join(gatesDir, "shell") ||
		(base != "zsh" && base != "bash") ||
		wrapped != "PATH='"+binDir+"':\"$PATH\" SHELL='"+sh+"' GROK_SHELL='"+sh+"' claude --x" {
		t.Errorf("wrap: %q %q %q %v", wrapped, gd, sh, err)
	}
	// The exit hatch drops the two vars but still renders the wrapper.
	wrapped, _, sh2, err := a.WrapWithGates("security", &Runtime{Name: "odd", NoGateShell: true}, []string{"Bash(git push:*)"}, "claude --x")
	if err != nil || sh2 != sh || wrapped != "PATH='"+binDir+"':\"$PATH\" claude --x" {
		t.Errorf("NoGateShell wrap: %q %q %v", wrapped, sh2, err)
	}
	if fi, err := os.Stat(sh2); err != nil || fi.Mode()&0o111 == 0 {
		t.Errorf("gate shell must be rendered and executable even for NoGateShell: %v", err)
	}
	// Running the wrapped line through sh really shadows git.
	out, _ := exec.Command("sh", "-c", GatePrefix(binDir, "")+"git push origin main; echo code=$?").CombinedOutput()
	if !strings.Contains(string(out), "refused by posse gate: git push origin main") || !strings.Contains(string(out), "code=1") {
		t.Errorf("typed-line PATH prefix must shadow git:\n%s", out)
	}
}

// rangerhq-2zm: git takes global options BEFORE the subcommand, so a
// matcher anchored at argv[1] let `git -C <repo> push` walk out of every
// repo without our pre-push hook. The shim skips the leading globals —
// consuming the values of the ones that take a separate argument — and
// matches the first non-option token.
func TestShimSkipsGlobalOptionsBeforeSubcommand(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	realBin := t.TempDir()
	os.WriteFile(filepath.Join(realBin, "git"), []byte("#!/bin/sh\necho \"real git $*\"\n"), 0o755)
	t.Setenv("PATH", realBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, binDir, _, err := a.RenderGates("developer", []string{"Bash(git push:*)"})
	if err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(binDir, "git")
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
	// Every spelling QA found in the live pane, plus the plain one.
	refused := [][]string{
		{"push", "origin", "main"},
		{"-C", ".", "push", "origin", "main"},
		{"-C", "/tmp/other", "push", "/tmp/other-bare.git", "main"},
		{"-c", "core.pager=cat", "push", "origin", "main"},
		{"--git-dir=.git", "--work-tree=.", "push", "origin", "main"},
		{"--git-dir", ".git", "--work-tree", ".", "push", "origin", "main"},
		{"-p", "push", "origin", "main"},
		{"--no-pager", "push", "origin", "main"},
		{"--literal-pathspecs", "push", "origin", "main"},
		{"--namespace", "x", "-C", ".", "--no-optional-locks", "push"},
	}
	for _, args := range refused {
		out, errs, code := run(args...)
		if code != 1 || !strings.Contains(errs, "refused by posse gate: git "+strings.Join(args, " ")+" (deny: Bash(git push:*))") || out != "" {
			t.Errorf("git %s must be refused: code=%d out=%q err=%q", strings.Join(args, " "), code, out, errs)
		}
	}
	// …and the shim still gets out of the way for everything else.
	passed := [][]string{
		{"status", "-s"},
		{"-C", "/tmp", "status", "-s"},
		{"-c", "core.pager=cat", "log", "--oneline"},
		{"--no-pager", "diff"},
		{"commit", "-m", "push"}, // 'push' as a value, not the verb
		{"-C", "push", "status"}, // 'push' as -C's value, not the verb
		{"pushy"},                // word match, not substring
	}
	for _, args := range passed {
		out, errs, code := run(args...)
		if code != 0 || strings.TrimSpace(out) != "real git "+strings.Join(args, " ") {
			t.Errorf("git %s must pass through: code=%d out=%q err=%q", strings.Join(args, " "), code, out, errs)
		}
	}
	// A trailing option that wants a value it never gets must not loop or
	// swallow the shim: `git -C` alone is git's own usage error.
	if _, _, code := run("-C"); code != 0 {
		t.Errorf("dangling -C must reach the real git: code=%d", code)
	}
	// Rules that lead with an option stay a literal argv prefix — there is
	// no subcommand grammar to be aware of.
	_, binDir2, _, err := a.RenderGates("developer", []string{"Bash(rm -rf /)"})
	if err != nil {
		t.Fatal(err)
	}
	if outb, err := exec.Command(filepath.Join(binDir2, "rm"), "-rf", "/").CombinedOutput(); err == nil || !strings.Contains(string(outb), "refused by posse gate: rm -rf /") {
		t.Errorf("literal-prefix rule must still match where it is written: %v %q", err, outb)
	}
}

// claudeDenyMatch models claude's Bash rule matcher (2.1.234) so the table
// below can assert what the emitted rules DO, not just how they read. Three
// forms, in the CLI's own order: `<c>:*` is a literal prefix of the command
// string, a rule carrying `*` is a wildcard (`*` -> `.*`, anchored both
// ends, runs of whitespace collapsed on both sides), anything else is exact.
// Verified against the real CLI in rangerhq-3mc — the nine option spellings
// refused, the pass-through set left alone.
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

// L0 politeness must refuse every spelling the L1 shim refuses: claude's
// prefix match is literal, so `git -C <repo> push` walked past the rule and
// straight into the shim's hard refusal with no polite one first
// (rangerhq-3mc, adjacent to rangerhq-2zm).
func TestL0SpellingsCoverTheOptionSpellings(t *testing.T) {
	rules := L0Spellings([]string{"Bash(git push:*)"})
	if rules[0] != "Bash(git push:*)" {
		t.Fatalf("the PID's own rule must come first and unchanged: %q", rules)
	}
	// rangerhq-2zm's list, plus the separated --git-dir/--work-tree form and
	// a stacked one — the same table TestShimSkipsGlobalOptionsBeforeSubcommand
	// asserts at L1.
	for _, cmd := range []string{
		"git push origin main",
		"git push",
		"git push --force origin main",
		"git -C /r push /bare.git main",
		"git -c core.pager=cat push /bare.git main",
		"git --git-dir=/r/.git --work-tree=/r push /bare.git main",
		"git --git-dir /r/.git --work-tree /r push /bare.git main",
		"git -p push /bare.git main",
		"git --no-pager push /bare.git main",
		"git --literal-pathspecs push /bare.git main",
		"git --namespace x -C /r --no-optional-locks push /bare.git main",
	} {
		if !deniedByAny(rules, cmd) {
			t.Errorf("L0 must refuse %q — rules %q", cmd, rules)
		}
	}
	// And must leave read-only/ordinary git alone. `push` as a value and a
	// path that merely starts with push are the cases that make the pair
	// (`git -* push` + `git -* push *`) worth the extra rule over one
	// `git -* push*`.
	for _, cmd := range []string{
		"git status -s",
		"git log --oneline",
		"git -c core.pager=cat status",
		"git -C /r status -s",
		"git --no-pager diff",
		"git log --grep=push --oneline",
		"git -c core.pager=cat log --grep=push --oneline",
		`git -c user.name=t commit --allow-empty -m "push it upstream"`,
		"git --no-pager log --oneline -- push.txt",
		"git -C /r pushy",
		"git commit -m push",
	} {
		if deniedByAny(rules, cmd) {
			t.Errorf("L0 must not refuse %q — rules %q", cmd, rules)
		}
	}
}

// The other shapes: exact rules take no trailing wildcard, a whole-verb rule
// is *exact* at L0 and needs `:*` to mean the verb, a rule leading with an
// option is a literal argv prefix in both matchers, and non-Bash rules are
// other layers' business.
func TestL0SpellingsShapes(t *testing.T) {
	for _, tc := range []struct {
		deny []string
		want []string
	}{
		{[]string{"Bash(git push)"}, []string{"Bash(git push)", "Bash(git -* push)"}},
		{[]string{"Bash(git push --force:*)"}, []string{"Bash(git push --force:*)", "Bash(git -* push --force)", "Bash(git -* push --force *)"}},
		{[]string{"Bash(bd)"}, []string{"Bash(bd)", "Bash(bd:*)"}},
		{[]string{"Bash(bd:*)"}, []string{"Bash(bd:*)"}}, // already the verb — no duplicate
		{[]string{"Bash(rm -rf /)"}, []string{"Bash(rm -rf /)"}},
		{[]string{"Edit", "Write", "mcp__x__y", "Bash(../evil:*)"}, []string{"Edit", "Write", "mcp__x__y", "Bash(../evil:*)"}},
		{nil, nil},
	} {
		got := L0Spellings(tc.deny)
		if strings.Join(got, " ") != strings.Join(tc.want, " ") {
			t.Errorf("L0Spellings(%q) = %q, want %q", tc.deny, got, tc.want)
		}
	}
	// A whole-verb rule must reach every invocation of the verb, which is
	// what the shim does with it and what `Bash(bd)` alone does not.
	if !deniedByAny(L0Spellings([]string{"Bash(bd)"}), "bd show x") {
		t.Error("whole-verb deny must refuse the verb with arguments")
	}
	if deniedByAny([]string{"Bash(bd)"}, "bd show x") {
		t.Error("test model is wrong: Bash(bd) alone is an exact match at L0")
	}
}

// grok's dialect is verified — and is why realizeGrok stays unwidened
// (rangerhq-625). The wildcard form is real there, so the option spellings
// WOULD be refused; but grok hands its matcher a dequoted segment, so the
// same pair also refuses ordinary git whose quoted argument happens to
// carry the word `push`. At L0 a false positive is a hard block the model
// cannot ask its way past — the ground rangerhq-3mc rejected a single
// `Bash(git -* push*)` on — so grok types the PID's own rules and L1 (which
// holds there since ADR 0009) stays the wall.
func TestGrokDialectIsWhyGrokIsNotWidened(t *testing.T) {
	rules := L0Spellings([]string{"Bash(git push:*)"})
	// The wildcard really does reach the option spellings on grok.
	for _, cmd := range []string{
		"git push origin main",
		"git -C /r push /bare.git main",
		"git --git-dir /r/.git --work-tree /r push /bare.git main",
		"git --namespace x -C /r --no-optional-locks push /bare.git main",
	} {
		if !grokDeniedByAny(rules, cmd) {
			t.Errorf("grok's wildcard must refuse %q — rules %q", cmd, rules)
		}
	}
	// And — the divergence — so does it reach these, which claude runs
	// because it matches the command with its quotes still on. Both were
	// refused live on grok 1.0.5 under the fleet's own --permission-mode
	// auto, which is what makes the pair unshippable there.
	for _, cmd := range []string{
		`git -c user.name=t commit --allow-empty -m "push it upstream"`,
		`git -C /r log --author "push me"`,
	} {
		if deniedByAny(rules, cmd) {
			t.Errorf("claude leaves %q alone — that is why the pair ships there", cmd)
		}
		if !grokDeniedByAny(rules, cmd) {
			t.Errorf("grok refused %q live; if it no longer does, rangerhq-625 is worth reopening", cmd)
		}
	}
	// The over-match has an edge, and it was probed in both directions
	// (rangerhq-9se, grok 1.0.5): the dequoted segment is still split into
	// tokens, so a quoted word that merely *contains* `push`, and a pathspec
	// that starts with it, both RAN on grok live under --permission-mode
	// auto. If either starts matching, this model is too broad and the
	// false-positive argument is bigger than what was measured.
	for _, cmd := range []string{
		`git -C /r log --author "pushme"`,
		"git --no-pager log -- push.txt",
	} {
		if grokDeniedByAny(rules, cmd) {
			t.Errorf("grok ran %q live — the over-match is a whole `push` token, not any occurrence", cmd)
		}
	}
	// Single quotes come off the same way double quotes do (probed live).
	if !grokDenyMatch("Bash(git -* push *)", `git -C /r log --author 'push me'`) {
		t.Error("grok strips single quotes too — `--author 'push me'` was refused live")
	}
	// The over-match is a whole `push` token after dequote, including a
	// quoted string whose *middle* word is push (rangerhq-b8i, grok 1.0.5,
	// --permission-mode auto). `"please push this"` was refused by
	// `git -* push *`; `"pushme"` above still runs.
	if !grokDeniedByAny(rules, `git -C /r log --author "please push this"`) {
		t.Error(`grok refused --author "please push this" live — the token is not required to start the quoted argument`)
	}
	// Equals-glued form does not create a ` push ` token: after dequote
	// `--author="push me"` is `--author=push me`. Ran live (rangerhq-b8i).
	if grokDeniedByAny(rules, `git -C /r log --author="push me"`) {
		t.Error(`grok ran --author="push me" live — if this starts matching, the over-match grew past a whole token`)
	}
	// The false positive needs a leading `git -` option. Plain
	// `git commit -m "push it upstream"` ran live under bypassPermissions
	// with the pair typed (rangerhq-b8i). If this starts matching, the
	// unshippable argument is larger than what was measured.
	if grokDeniedByAny(rules, `git commit --allow-empty -m "push it upstream"`) {
		t.Error(`grok ran git commit -m "push it upstream" live — the pair requires git -`)
	}
	// Whitespace collapse, isolated: a rule with NO wildcard, so `*`
	// cannot absorb the extra space (the two-space-vs-wildcard probe in
	// rangerhq-625 did not isolate this; rangerhq-2uc4). Live on grok
	// 1.0.5, --permission-mode auto: `Bash(git log)` refused
	// `git  log --oneline` (two spaces) and left `git status -s` alone.
	if !grokDenyMatch("Bash(git log)", "git  log --oneline") {
		t.Error("grok collapsed unquoted whitespace live — Bash(git log) refused `git  log --oneline`")
	}
	if grokDenyMatch("Bash(git log)", "git status -s") {
		t.Error("test model is wrong: Bash(git log) must not refuse git status")
	}
	// `?` is a single-char wildcard there too (claimed in rangerhq-625,
	// live in rangerhq-b8i under bypassPermissions).
	if !grokDenyMatch("Bash(git ?ush *)", "git push ../nope.git HEAD:refs/heads/q") {
		t.Error("grok's `?` refused `git push …` live — Bash(git ?ush *)")
	}
	// One false positive is NOT grok's: an unquoted trailing `push` word
	// ends the command, so `Bash(git -* push)` matches it in both dialects.
	// Filed rather than fixed here (rangerhq-ky3) — it is the claude pair's
	// defect and this bead is grok's. `git -C . stash push` is the same
	// shape (nested subcommand, command ends with the word); live on
	// grok 1.0.5 and claude 2.1.239 (rangerhq-bzw).
	for _, cmd := range []string{
		"git -C /r log --grep push",
		"git -C /r stash push",
	} {
		if !deniedByAny(rules, cmd) || !grokDeniedByAny(rules, cmd) {
			t.Errorf("shared false positive %q went away — close rangerhq-ky3", cmd)
		}
	}
	// The wildcard half is a different shape: any ` push ` token AFTER a
	// leading global option, not just a trailing one. Live on grok 1.0.5
	// and claude 2.1.239 (rangerhq-vr6j): `stash push -m wip` (nested
	// subcommand with args) and `git -C push status` (`push` is -C's
	// value). The control without a leading option ran on grok.
	for _, cmd := range []string{
		"git -C /r stash push -m wip",
		"git -C push status -s",
	} {
		if !deniedByAny(rules, cmd) || !grokDeniedByAny(rules, cmd) {
			t.Errorf("shared false positive %q went away — close rangerhq-vr6j", cmd)
		}
	}
	if deniedByAny(rules, "git stash push") || grokDeniedByAny(rules, "git stash push") {
		t.Error("git stash push without a leading option ran on grok live — the pair requires git -")
	}
	// So the realizer types the PID's list and nothing else.
	if got := realizeGrok(nil, []string{"Bash(git push:*)"}, ""); got.Deny != `--deny 'Bash(git push:*)'` {
		t.Errorf("realizeGrok must not widen: %q", got.Deny)
	}
	// The whole-verb half would be a no-op there in any case: a plain rule
	// is already a prefix on grok, not claude's exact match.
	if !grokDenyMatch("Bash(bd)", "bd show x") {
		t.Error("Bash(bd) is a prefix on grok — it must refuse the verb with arguments unaided")
	}
	// The PID's own rule is the broader one on grok, which is grok's call,
	// not ours to spell around.
	if !grokDenyMatch("Bash(git push:*)", "git pushy --help") {
		t.Error("grok's `:*` prefix has no word boundary — `git pushy` is refused there")
	}
}

// A command not on PATH at render time still gets a shim that refuses
// matches and searches PATH at run time (skipping its own dir).
func TestShimForMissingBinary(t *testing.T) {
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	_, binDir, _, err := a.RenderGates("p", []string{"Bash(nosuchtool danger:*)"})
	if err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(binDir, "nosuchtool")
	if outb, err := exec.Command(shim, "danger", "x").CombinedOutput(); err == nil || !strings.Contains(string(outb), "refused by posse gate: nosuchtool danger x") {
		t.Errorf("refusal without a real binary: %v %q", err, outb)
	}
	if outb, err := exec.Command(shim, "safe").CombinedOutput(); err == nil || !strings.Contains(string(outb), "real binary not found") {
		t.Errorf("pass-through without a real binary must say so: %v %q", err, outb)
	}
}

// L3: the pre-push hook refuses under a matching RHQ_TOOLS_DENY and passes
// otherwise; install-hooks never overwrites a foreign hook, replaces ours.
func TestPrePushHook(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	if PrePushHookInstalled(repo) {
		t.Fatal("fresh repo must not report our hook")
	}
	p, err := InstallPrePushHook(repo)
	if err != nil {
		t.Fatal(err)
	}
	if !PrePushHookInstalled(repo) {
		t.Error("installed hook not detected")
	}
	gates := t.TempDir()
	run := func(env ...string) (string, int) {
		cmd := exec.Command(p, "origin", "https://example.invalid/x.git")
		cmd.Env = append([]string{"PATH=" + os.Getenv("PATH"), "RHQ_GATES_DIR=" + gates, "RHQ_PERSONA=security"}, env...)
		out, err := cmd.CombinedOutput()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
		return string(out), code
	}
	if out, code := run("RHQ_TOOLS_DENY=Edit\nBash(git push:*)"); code != 1 || !strings.Contains(out, "refused by posse gate: git push (deny: Bash(git push:*))") {
		t.Errorf("must refuse under a git push deny: code=%d %q", code, out)
	}
	if out, code := run("RHQ_TOOLS_DENY=Bash(git push --force:*)"); code != 1 || !strings.Contains(out, "deny: Bash(git push --force:*)") {
		t.Errorf("git push --force rule counts as a push deny: code=%d %q", code, out)
	}
	if out, code := run("RHQ_TOOLS_DENY=Bash(git:*)"); code != 1 || !strings.Contains(out, "deny: Bash(git:*)") {
		t.Errorf("whole-git deny counts: code=%d %q", code, out)
	}
	if out, code := run("RHQ_TOOLS_DENY=Edit\nBash(git commit:*)"); code != 0 {
		t.Errorf("unrelated denies must pass: code=%d %q", code, out)
	}
	if out, code := run(); code != 0 {
		t.Errorf("no RHQ_TOOLS_DENY (interactive operator) must pass: code=%d %q", code, out)
	}
	logb, _ := os.ReadFile(filepath.Join(gates, "refusals.log"))
	if strings.Count(string(logb), "[pre-push hook]") != 3 {
		t.Errorf("refusals.log: %q", logb)
	}
	// Re-install replaces our own hook; a foreign hook is left alone.
	if _, err := InstallPrePushHook(repo); err != nil {
		t.Errorf("re-install of our hook must succeed: %v", err)
	}
	os.WriteFile(p, []byte("#!/bin/sh\necho mine\n"), 0o755)
	if _, err := InstallPrePushHook(repo); err == nil || !strings.Contains(err.Error(), "not a posse hook") {
		t.Errorf("foreign hook must not be overwritten: %v", err)
	}
	if b, _ := os.ReadFile(p); !strings.Contains(string(b), "echo mine") {
		t.Error("foreign hook was clobbered")
	}
	if _, err := InstallPrePushHook(t.TempDir()); err == nil {
		t.Error("non-repo must error")
	}
	// End to end: a real push into a bare remote is refused by git itself.
	remote := t.TempDir()
	exec.Command("git", "-C", remote, "init", "-q", "--bare").Run()
	os.WriteFile(p, []byte(PrePushHook), 0o755)
	git := func(env []string, args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append([]string{"PATH=" + os.Getenv("PATH"), "HOME=" + repo, "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}, env...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	os.WriteFile(filepath.Join(repo, "f"), []byte("x"), 0o644)
	git(nil, "add", "f")
	git(nil, "commit", "-q", "-m", "x")
	git(nil, "remote", "add", "origin", remote)
	// The full refusal line, not just "refused by posse gate": an L1 git shim
	// on PATH says the same first words, and a shim answering here is how
	// this test used to pass green while proving nothing (rangerhq-8sd).
	if out, err := git([]string{"RHQ_TOOLS_DENY=Bash(git push:*)"}, "push", "-q", "origin", "HEAD:refs/heads/main"); err == nil ||
		!strings.Contains(out, "refused by posse gate: git push (deny: Bash(git push:*)) — pre-push hook") {
		t.Errorf("real git push must be refused by the hook: %v %s", err, out)
	}
	if out, err := git(nil, "push", "-q", "origin", "HEAD:refs/heads/main"); err != nil {
		t.Errorf("push without the deny env must succeed: %v %s", err, out)
	}
}

// The refusal must be the SCRIPT's exit, not a pipeline subshell's: the
// hook used to end in `printf | while read`, whose `exit 1` left only the
// subshell, so it refused solely by being the last statement. One appended
// line — the natural reading of "chain it by hand" — made it print the
// refusal, write refusals.log, and exit 0 while git pushed (rangerhq-kk6e).
// Runs the shipped template with text appended, rather than asserting on
// its shape: the executed artifact is the wall (rangerhq-bju2).
func TestPrePushHookExitsForReal(t *testing.T) {
	gates := t.TempDir()
	// tail is appended after the whole template, exactly as chaining by
	// hand would; whatever the verdict, the template must have exited
	// first, so tail never decides anything.
	run := func(tail string, env ...string) (string, int) {
		p := filepath.Join(t.TempDir(), "pre-push")
		if err := os.WriteFile(p, []byte(PrePushHook+tail), 0o755); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(p, "origin", "https://example.invalid/x.git")
		cmd.Env = append([]string{"PATH=" + os.Getenv("PATH"), "RHQ_GATES_DIR=" + gates, "RHQ_PERSONA=probe"}, env...)
		out, err := cmd.CombinedOutput()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
		return string(out), code
	}
	deny := "RHQ_TOOLS_DENY=Edit\nBash(git push:*)"
	if out, code := run("", deny); code != 1 {
		t.Fatalf("verbatim template must refuse: code=%d %q", code, out)
	}
	// The defect: refusal printed, exit 0, push proceeds.
	if out, code := run("\nexit 0\n", deny); code != 1 || !strings.Contains(out, "refused by posse gate: git push") {
		t.Errorf("refusal must survive appended text: code=%d %q", code, out)
	}
	// Trailing text is inert on every allow path too, so a hand-chained
	// hook cannot turn a pass into a failure either.
	for _, c := range []struct {
		what string
		env  []string
	}{
		{"no deny at all", nil},
		{"unrelated denies", []string{"RHQ_TOOLS_DENY=Edit\nBash(git commit:*)"}},
	} {
		if out, code := run("\nexit 3\n", c.env...); code != 0 {
			t.Errorf("%s must pass regardless of appended text: code=%d %q", c.what, code, out)
		}
	}
	// A rule ending in ':*' must not be glob-expanded by the splitting.
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "Bash(git push:x)"), nil, 0o644)
	cmd := exec.Command("/bin/sh", "-c", "cd \"$1\" && exec \"$2\" origin x", "sh", tmp, func() string {
		p := filepath.Join(t.TempDir(), "pre-push")
		os.WriteFile(p, []byte(PrePushHook), 0o755)
		return p
	}())
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "RHQ_PERSONA=probe", deny}
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "deny: Bash(git push:*)") {
		t.Errorf("rule must not glob against the cwd: %v %q", err, out)
	}
}

// The refusal names the chain-dispatcher form rather than "chain it by
// hand", which read as "append ours to theirs" — the form that fails open
// (rangerhq-0g1c).
func TestForeignHookRefusalPrescribesTheChain(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	for _, slot := range []string{"pre-push", "prepare-commit-msg"} {
		repo := t.TempDir()
		if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
			t.Fatalf("git init: %v %s", err, out)
		}
		hooks := filepath.Join(repo, ".git", "hooks")
		os.MkdirAll(hooks, 0o755)
		os.WriteFile(filepath.Join(hooks, slot), []byte("#!/bin/sh\nexec other\n"), 0o755)
		var err error
		if slot == "pre-push" {
			_, err = InstallPrePushHook(repo)
		} else {
			_, err = installCommitGuard(repo)
		}
		if err == nil {
			t.Fatalf("%s: foreign hook must not be overwritten", slot)
		}
		msg := err.Error()
		for _, want := range []string{
			"not a posse hook",
			`"$d/posse-` + slot + `" "$@"`,
			`exec "$d/theirs-` + slot + `"`,
			"|| exit $?",
		} {
			if !strings.Contains(msg, want) {
				t.Errorf("%s refusal missing %q:\n%s", slot, want, msg)
			}
		}
		// pre-push is the slot git feeds a ref list on stdin; the other
		// hook in the chain reads it, so ours must be kept off it.
		if got := strings.Contains(msg, "</dev/null"); got != (slot == "pre-push") {
			t.Errorf("%s: </dev/null present=%v, want %v", slot, got, slot == "pre-push")
		}
		// And the probe it prescribes has to be the one that fires THIS
		// slot: the push gate keys on RHQ_TOOLS_DENY, the commit gate on
		// RHQ_PERSONA alone and only with a message-file argument.
		if got := strings.Contains(msg, "RHQ_TOOLS_DENY="); got != (slot == "pre-push") {
			t.Errorf("%s: probe names RHQ_TOOLS_DENY=%v, want %v", slot, got, slot == "pre-push")
		}
		if !strings.Contains(msg, "./"+slot) {
			t.Errorf("%s: probe must run the slot itself:\n%s", slot, msg)
		}
	}
}

// PATH minus every posse gates bin: what lets a process reach the real
// binary its own session's shims stand in front of — including this test
// binary, which runs inside a persona pane (rangerhq-8sd).
func TestPathOutsideGates(t *testing.T) {
	// TestMain ran this over the process PATH: nothing gated survives in
	// ours, so the suite meets git as itself and not through a shim.
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		if strings.Contains(p, string(filepath.Separator)+"gates"+string(filepath.Separator)) {
			t.Errorf("test process PATH still carries a gates dir: %s", p)
		}
	}
	sep := string(os.PathListSeparator)
	own := filepath.Join("opt", "shims", "bin")                  // dropped by name
	gated := filepath.Join("home", "state", "gates", "x", "bin") // dropped as a gates dir
	keep := filepath.Join("usr", "bin")
	gatesy := filepath.Join("opt", "gateskeeper", "bin") // "gates" is not a path element here
	t.Setenv("PATH", strings.Join([]string{own, "", gated, keep, gatesy}, sep))
	if got, want := PathOutsideGates(own), strings.Join([]string{keep, gatesy}, sep); got != want {
		t.Errorf("PathOutsideGates = %q, want %q", got, want)
	}
	// A caller with no shim dir of its own still sheds every gates dir.
	if got, want := PathOutsideGates(""), strings.Join([]string{own, keep, gatesy}, sep); got != want {
		t.Errorf("PathOutsideGates(\"\") = %q, want %q", got, want)
	}
}

// ─── The gate shell (ADR 0009) ───────────────────────────────────────────────

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

// The rendered wrapper walks argv the way a shell does and prepends the
// PATH guard exactly where a command string can be: the first operand
// after a -c, and — behind a `--` argv0 — the runtime's user-command slot.
// Cases are the probe's own set (docs/adr/0009-gate-shell.probe.sh), i.e.
// the shapes verified against grok 1.0.5, claude and codex 0.147.
func TestGateShellArgvWalk(t *testing.T) {
	// A fake real shell that just prints its argv, one <arg> per word.
	wrapper, _, binDir := renderGateShellFor(t, "zsh",
		"#!/bin/sh\nfor a in \"$@\"; do printf '<%s>' \"$a\"; done\nprintf '\\n'\n")
	guard := "_rgp=; _rgr=\"$PATH:\"; while [ -n \"$_rgr\" ]; do _rge=${_rgr%%:*}; _rgr=${_rgr#*:}; case \"$_rge\" in ''|*/gates/*) ;; *) _rgp=\"$_rgp:$_rge\";; esac; done; PATH=\"" + binDir + "$_rgp\"; export PATH; unset _rgp _rgr _rge; "
	// The user-command slot carries the log line ahead of the same guard.
	slot := func(cmd string) string {
		return "case \"$PATH:\" in \"" + binDir + "\":*) ;; *) echo \"$(date -u +%Y-%m-%dT%H:%M:%SZ) gates dir not first in replayed PATH; re-prepended (path_helper/rc reorder?)\" >> '" +
			filepath.Dir(binDir) + "/shell.log' 2>/dev/null;; esac; " + guard + cmd
	}
	g := func(s string) string { return guard + s }
	cases := []struct {
		name string
		argv []string
		want []string
	}{
		{"claude -c -l STR", []string{"-c", "-l", "S"}, []string{"-c", "-l", g("S")}},
		{"grok -lc STR", []string{"-lc", "S"}, []string{"-lc", g("S")}},
		{"grok zsh snapshot + user slot", []string{"-c", "S", "--", "C"}, []string{"-c", g("S"), "--", slot("C")}},
		{"grok bash snapshot + trailing arg", []string{"-O", "extglob", "-c", "S", "--", "C", "pfx"},
			[]string{"-O", "extglob", "-c", g("S"), "--", slot("C"), "pfx"}},
		{"interactive login shell", []string{"-l", "-i"}, []string{"-l", "-i"}},
		{"-c with -- before the string", []string{"-c", "--", "S"}, []string{"-c", "--", g("S")}},
		{"a script path, no -c", []string{"script", "a", "b"}, []string{"script", "a", "b"}},
		{"-o takes a value", []string{"-o", "errexit", "-c", "S", "--", "y"},
			[]string{"-o", "errexit", "-c", g("S"), "--", slot("y")}},
		{"long option before -c", []string{"--login", "-c", "S"}, []string{"--login", "-c", g("S")}},
		{"bundled -ic", []string{"-ic", "S"}, []string{"-ic", g("S")}},
	}
	for _, c := range cases {
		outb, err := exec.Command(wrapper, c.argv...).CombinedOutput()
		if err != nil {
			t.Errorf("%s: %v %q", c.name, err, outb)
			continue
		}
		want := "<" + strings.Join(c.want, "><") + ">\n"
		if string(outb) != want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, outb, want)
		}
	}
}

// End to end: with the wrapper as $SHELL, a runtime that re-execs a login
// shell and replays a captured PATH snapshot still resolves a denied verb
// to the shim — and shell.log records exactly the case the ADR wants seen:
// the replay had dropped the gates dir, so only the user-command slot's
// guard saved it.
func TestGateShellGuardsTheReplayedPATH(t *testing.T) {
	// A fake real shell that behaves like one: `zsh -c STR -- CMD` runs STR
	// with $1 = CMD, which is where a snapshot replay evals the user command.
	wrapper, gatesDir, binDir := renderGateShellFor(t, "zsh", "#!/bin/sh\nexec /bin/sh \"$@\"\n")
	logPath := filepath.Join(gatesDir, "shell.log")
	// snapshot is what grok's captured login state replays before the user
	// command — including the PATH path_helper handed the login shell.
	run := func(snapshotPATH string) string {
		cmd := exec.Command(wrapper, "-c", `export PATH=`+snapshotPATH+`; eval "$1"`, "--", "command -v git; git push origin main")
		cmd.Env = []string{"PATH=" + binDir + ":/usr/bin:/bin"}
		out, _ := cmd.CombinedOutput()
		return string(out)
	}
	// The replay drops the gates dir (path_helper's doing). Only the slot
	// guard runs after it, and it is what makes git resolve to the shim.
	out := run("/usr/bin:/bin")
	if !strings.Contains(out, filepath.Join(binDir, "git")) || !strings.Contains(out, "refused by posse gate: git push origin main") {
		t.Errorf("slot guard must re-prepend the gates dir after the replay:\n%s", out)
	}
	if b, _ := os.ReadFile(logPath); strings.Count(string(b), "gates dir not first in replayed PATH") != 1 {
		t.Errorf("shell.log must record the reorder exactly once: %q", b)
	}
	// The live case (rangerhq-e43): path_helper does not DROP the gates dir
	// — the typed line put it on PATH, so path_helper re-orders it below
	// /usr/bin and keeps it. A guard that tested presence read as
	// idempotent and no-opped here, and the shim never ran.
	os.Remove(logPath)
	out = run("/usr/bin:/bin:" + binDir)
	if !strings.Contains(out, filepath.Join(binDir, "git")) || !strings.Contains(out, "refused by posse gate: git push origin main") {
		t.Errorf("a demoted gates dir must be re-prepended, not accepted as present:\n%s", out)
	}
	if b, _ := os.ReadFile(logPath); strings.Count(string(b), "gates dir not first in replayed PATH") != 1 {
		t.Errorf("shell.log must record the demotion exactly once: %q", b)
	}
	// A replay that kept the gates dir FIRST is left alone and stays out of
	// the log — a normal session ends with shell.log absent.
	os.Remove(logPath)
	if out := run(binDir + ":/usr/bin:/bin"); !strings.Contains(out, "refused by posse gate: git push origin main") {
		t.Errorf("shim must still fire:\n%s", out)
	}
	if _, err := os.Stat(logPath); err == nil {
		t.Error("shell.log must stay silent when the replayed PATH kept the gates dir first")
	}
	// And a subprocess of the replayed command hits the shim too: PATH
	// reaches children, which is why the guard is PATH and not a function.
	cmd := exec.Command(wrapper, "-c", `export PATH=/usr/bin:/bin; eval "$1"`, "--", `sh -c 'git push origin main'`)
	cmd.Env = []string{"PATH=" + binDir + ":/usr/bin:/bin"}
	if outb, _ := cmd.CombinedOutput(); !strings.Contains(string(outb), "refused by posse gate: git push origin main") {
		t.Errorf("a subprocess of the replayed command must hit the shim:\n%s", outb)
	}
}

// realShell's contract, in its own words: "$SHELL when it is a bash or zsh
// that is really there, else the first of zsh/bash on PATH, else /bin/sh".
//
// This asserted macOS's filesystem layout instead — REAL='/bin/zsh' — and so
// failed on Linux in both configurations, whether the image ships zsh
// (/usr/bin/zsh) or does not ship it at all (ranger-base-gaf). The product
// code was right and is unchanged.
//
// The PATH search is the part that most needed pinning and had no test at
// all: the same renderer runs INSIDE a cage image, where $SHELL is unset and
// /bin/zsh does not exist (rangerhq-6so), and a wrapper whose REAL cannot be
// exec'd is a dead gate shell. A literal path cannot express that case. So
// the search gets a PATH this test owns, and every arm of the contract has
// one answer on every platform.
func TestGateShellRealShellResolution(t *testing.T) {
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}

	pathDir := t.TempDir()
	t.Setenv("PATH", pathDir)
	install := func(name string) string {
		t.Helper()
		p := filepath.Join(pathDir, name)
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	var gatesDir string
	render := func(shell string) string {
		t.Helper()
		t.Setenv("SHELL", shell)
		d, _, sh, err := a.RenderGates("developer", nil)
		if err != nil {
			t.Fatalf("$SHELL=%q: %v", shell, err)
		}
		gatesDir = d
		return sh
	}
	// The two halves of one promise: the wrapper execs that shell, and is
	// installed under its name — a runtime that infers its snapshot dialect
	// from the shell's name (grok does) reads the name through the wrapper.
	execs := func(wrapper, real string) {
		t.Helper()
		if got, want := filepath.Base(wrapper), filepath.Base(real); got != want {
			t.Errorf("wrapper is named %q, must carry the resolved shell's own name %q", got, want)
		}
		b, err := os.ReadFile(wrapper)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), "REAL="+shQuote(real)) {
			t.Errorf("wrapper must exec %s:\n%s", real, b)
		}
	}

	// $SHELL wins when it names a bash or zsh that is really there — even one
	// nothing on PATH would have found.
	own := filepath.Join(t.TempDir(), "bash")
	if err := os.WriteFile(own, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	fromShell := render(own)
	execs(fromShell, own)

	// A $SHELL that is neither, and an unset one, fall through to the PATH
	// search — which prefers zsh to bash, in that order.
	zsh, bash := install("zsh"), install("bash")
	for _, s := range []string{filepath.Join(t.TempDir(), "fish"), ""} {
		execs(render(s), zsh)
		if _, err := os.Stat(fromShell); err == nil {
			t.Error("a wrapper from a previous $SHELL must not survive a re-render")
		}
	}

	// zsh gone, bash still there: the second choice, not the last resort.
	if err := os.Remove(zsh); err != nil {
		t.Fatal(err)
	}
	execs(render(""), bash)

	// Neither on PATH: the cage image this search exists for. Every image has
	// /bin/sh and the wrapper script is POSIX sh, so this is a working gate
	// shell rather than a placeholder — it costs only the dialect a runtime
	// might infer from the name, which is why it is last.
	if err := os.Remove(bash); err != nil {
		t.Fatal(err)
	}
	execs(render(""), "/bin/sh")

	if _, err := os.Stat(filepath.Join(gatesDir, "shell")); err != nil {
		t.Errorf("shell dir: %v", err)
	}
}

// ranger-base-f0ay — the fleet freeze of 2026-08-27. A rendered wrapper is
// installed as gates/<persona>/shell/zsh: shell basename, ordinary
// executable file. Every test realShell applied to $SHELL was true of one,
// so a render running while $SHELL was another persona's wrapper captured
// that wrapper as REAL. Wrappers chained persona-to-persona; on 08-27 the
// chain closed into a two-node cycle (two personas each other's REAL) and
// every spawn entering it exec-looped, growing the -c string ~320 B/hop
// until E2BIG ~40 minutes later. The symptom was every Bash call in every
// session hanging with zero bytes.
//
// The three renders below are that incident, in order: an honest one, the
// chain link, then the render that closed the cycle. No platform path is
// asserted (ranger-base-gaf/2cv: a hardcoded /bin/zsh here fails on Linux
// runners either way) — the contract is "not a wrapper, and really there".
func TestGateShellNeverChainsToAnotherWrapper(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}

	// A zsh that is really a shell, on PATH and outside every gates dir —
	// the answer the search must fall through to on any platform.
	pathDir := t.TempDir()
	honest := filepath.Join(pathDir, "zsh")
	if err := os.WriteFile(honest, []byte("#!/bin/sh\nexec /bin/sh \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", pathDir)

	realOf := func(t *testing.T, wrapper string) string {
		t.Helper()
		b, err := os.ReadFile(wrapper)
		if err != nil {
			t.Fatal(err)
		}
		m := regexp.MustCompile(`(?m)^REAL='(.*)'$`).FindStringSubmatch(string(b))
		if m == nil {
			t.Fatalf("no REAL= line in %s:\n%s", wrapper, b)
		}
		return m[1]
	}
	render := func(t *testing.T, persona, shell string) string {
		t.Helper()
		t.Setenv("SHELL", shell)
		_, _, w, err := a.RenderGates(persona, []string{"Bash(git push:*)"})
		if err != nil {
			t.Fatalf("render %s under $SHELL=%s: %v", persona, shell, err)
		}
		real := realOf(t, w)
		if isGateWrapper(real) {
			t.Fatalf("%s's REAL is a gate wrapper (%s) — this is the chain that wedged the fleet", persona, real)
		}
		if _, err := os.Stat(real); err != nil {
			t.Errorf("%s's REAL must be a shell that is really there: %v", persona, err)
		}
		return w
	}

	// 1. An honest render: $SHELL is a shell, and it wins.
	coordinator := render(t, "coordinator", honest)
	if got := realOf(t, coordinator); got != honest {
		t.Errorf("an honest $SHELL still wins: REAL=%s, want %s", got, honest)
	}
	// 2. The chain link: a render whose $SHELL is another persona's
	//    wrapper. It must refuse it and fall through to the search.
	developer3 := render(t, "developer-3", coordinator)
	if got := realOf(t, developer3); got != honest {
		t.Errorf("a wrapper as $SHELL must fall through to the PATH search: REAL=%s, want %s", got, honest)
	}
	// 3. The render that closed the cycle on 08-27 — coordinator again, this
	//    time under developer-3's wrapper, with coordinator already developer-3's
	//    REAL in the buggy world.
	render(t, "coordinator", developer3)

	// The spawn canary the RCA asks for, run where a cycle would now be:
	// under the bug, coordinator and developer-3 name each other and this hangs
	// until E2BIG. Time-bounded, so a regression fails the suite instead
	// of wedging it — which is exactly how the incident presented.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, coordinator, "-c", "echo canary").CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("spawn through the gate shell did not terminate — the wrappers are chained: %q", out)
	}
	if err != nil || !strings.Contains(string(out), "canary") {
		t.Errorf("gate shell must run its command: %v %q", err, out)
	}
}

// rangerhq-v553: a persona launched from ANOTHER persona's pane inherits
// that pane's PATH, whose head is the launching persona's shim dir. The
// wrapper only ever prepended its own, so both dirs were live and a verb
// that only the launching PID denies had no shim of ours in front of it:
// the launched session was refused by a rule it does not carry, and ADR
// 0002 §3 says the PID is the source of truth for a session's wall.
//
// The wrapper's REAL no longer chains (ranger-base-f0ay), which removed the
// other half of the same leak; this is the half that survives it, because
// PATH arrives through the environment and not through the exec.
func TestGateShellDropsAnotherPersonasGatesBin(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	// A zsh that is really a shell, so REAL is honest on every platform.
	pathDir := t.TempDir()
	honest := filepath.Join(pathDir, "zsh")
	if err := os.WriteFile(honest, []byte("#!/bin/sh\nexec /bin/sh \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", honest)

	// Two PIDs that deny different verbs — the asymmetry is the whole test:
	// alpha has no shim to shadow beta's, so beta's is what runs.
	_, alphaBin, alpha, err := a.RenderGates("alpha", []string{"Bash(ls:*)"})
	if err != nil {
		t.Fatal(err)
	}
	_, betaBin, _, err := a.RenderGates("beta", []string{"Bash(date:*)"})
	if err != nil {
		t.Fatal(err)
	}

	// alpha's session, started from inside beta's: beta's shim dir is on
	// the inherited PATH, first, exactly as the launching pane left it.
	run := func(args ...string) string {
		cmd := exec.Command(alpha, args...)
		cmd.Env = []string{"PATH=" + betaBin + ":" + PathOutsideGates(""), "RHQ_GATES_DIR=" + filepath.Dir(alphaBin)}
		out, _ := cmd.CombinedOutput()
		return string(out)
	}
	if out := run("-c", "date +%Y"); strings.Contains(out, "refused by posse gate") {
		t.Errorf("alpha's PID does not deny date — a rule from beta's PID must not reach it:\n%s", out)
	}
	if out := run("-c", "echo $PATH"); strings.Contains(out, betaBin) {
		t.Errorf("beta's shim dir must be off alpha's PATH:\n%s", out)
	}
	// The guard still does its first job: alpha's own dir, first, and its
	// own deny lands.
	if out := run("-c", "echo $PATH"); !strings.HasPrefix(strings.TrimSpace(out), alphaBin+string(os.PathListSeparator)) {
		t.Errorf("alpha's own shim dir must lead its PATH:\n%s", out)
	}
	if out := run("-c", "ls /"); !strings.Contains(out, "refused by posse gate: ls /") {
		t.Errorf("alpha's own deny must still bite:\n%s", out)
	}
	// And with no -c string to prefix — a script, an interactive login —
	// the wrapper's own exported PATH is already clean, because exec hands
	// it to REAL and nothing else would.
	script := filepath.Join(t.TempDir(), "p.sh")
	if err := os.WriteFile(script, []byte("echo \"$PATH\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out := run(script); strings.Contains(out, betaBin) {
		t.Errorf("the wrapper's own env must not carry beta's shim dir into REAL:\n%s", out)
	}
}

// The marker is what makes a wrapper recognizable when the path cannot say
// so — a second RHQ_HOME, or the cage render at CageStateRoot. It lives in
// the script, so it can drift out of it; this is the pin. The content test
// is exercised on a copy OUTSIDE any gates dir, since inside one the path
// test would answer first and prove nothing.
func TestGateShellScriptCarriesItsMarker(t *testing.T) {
	if !strings.Contains(gateShellScript, gateShellMarker) {
		t.Fatalf("gateShellScript must carry %q — isGateWrapper reads it to tell a wrapper from a shell", gateShellMarker)
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	t.Setenv("SHELL", "")
	_, _, wrapper, err := a.RenderGates("developer", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !isGateWrapper(wrapper) {
		t.Errorf("a rendered wrapper must be recognized in place: %s", wrapper)
	}
	b, err := os.ReadFile(wrapper)
	if err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(t.TempDir(), "zsh") // no `gates` path element
	if err := os.WriteFile(moved, b, 0o755); err != nil {
		t.Fatal(err)
	}
	if !isGateWrapper(moved) {
		t.Errorf("a wrapper must be recognized by content wherever it sits: %s", moved)
	}
	if isGateWrapper(filepath.Join(t.TempDir(), "zsh")) {
		t.Error("a missing file is not a wrapper")
	}
	plain := filepath.Join(t.TempDir(), "zsh")
	if err := os.WriteFile(plain, []byte("#!/bin/sh\nexec /bin/sh \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if isGateWrapper(plain) {
		t.Errorf("an ordinary shell is not a wrapper: %s", plain)
	}
}

// The render-time assertion: whatever resolved REAL, a wrapper is never
// written naming another one. realShell refuses first, so this is the belt
// — it is what would have turned the 08-27 chain into a failed launch
// instead of a silent link, and it holds even if some later resolution
// path learns a new way to hand back a wrapper.
func TestGateShellRenderRefusesAWrapperAsReal(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	t.Setenv("SHELL", "")
	gatesDir, binDir, wrapper, err := a.RenderGates("developer", nil)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(wrapper)
	if err != nil {
		t.Fatal(err)
	}
	other := a.GatesDir("qa")
	_, err = writeGateShell("qa", other, filepath.Join(other, "bin"), wrapper, "zsh")
	if err == nil {
		t.Fatal("a render whose REAL is a gate wrapper must be refused, not written")
	}
	for _, want := range []string{"qa", wrapper, "ranger-base-f0ay"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must name %q so an operator can act on it: %v", want, err)
		}
	}
	if _, err := os.Stat(filepath.Join(other, "shell")); err == nil {
		t.Error("a refused render must write nothing")
	}
	// Asserted before the dir is cleared: refusing a render must not also
	// cost the persona the working wrapper it already had.
	if after, err := os.ReadFile(wrapper); err != nil || string(after) != string(before) {
		t.Errorf("a refused render must leave the existing wrapper alone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(gatesDir, "bin")); err != nil {
		t.Errorf("bin dir: %v", err)
	}
	_ = binDir
}

// rangerhq-lmq9. The L1 half of the shared-index wall: a NEGATIVE rule.
// `Bash(git commit unless --)` refuses `git commit` UNLESS argv carries
// `--` with at least one operand. The three argv shapes rangerhq-nyqj
// measured are the three cases here, plus the empty pathspec — `git commit
// --` reaches git with the shared index, so the bare token is not enough.
//
// AND UNLESS THE OPERAND IS A LIE (rangerhq-ojnw): `-i`/`--include` carries
// a pathspec and commits the shared index on top of it, so the qualifier is
// satisfied by a form that does the very thing the rule refuses. The
// qualifier is only a proxy for "path-limited"; its false negatives are
// spelled out in qualifierSpoilers and refused by name.
func TestShimNegativeMatchUnless(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	if r := ParseShimRules([]string{"Bash(git commit unless --)"})["git"][0]; r.Unless != "--" ||
		strings.Join(r.Words, " ") != "commit" || r.Exact {
		t.Fatalf("parse: %+v", r)
	}
	// `unless` needs a word in front of it to be the grammar; otherwise it
	// is an ordinary subcommand token.
	if r := ParseShimRules([]string{"Bash(git unless --)"})["git"][0]; r.Unless != "" ||
		strings.Join(r.Words, " ") != "unless --" {
		t.Errorf("bare `unless` is not the grammar: %+v", r)
	}
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	realBin := t.TempDir()
	os.WriteFile(filepath.Join(realBin, "git"), []byte("#!/bin/sh\necho \"real git $*\"\n"), 0o755)
	t.Setenv("PATH", realBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	gatesDir, binDir, _, err := a.RenderGates("developer", []string{"Bash(git push:*)", "Bash(git commit unless --)"})
	if err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) (string, string, int) {
		cmd := exec.Command(filepath.Join(binDir, "git"), args...)
		var out, errb strings.Builder
		cmd.Stdout, cmd.Stderr = &out, &errb
		err := cmd.Run()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
		return out.String(), errb.String(), code
	}
	refused := []struct {
		what string
		argv []string
	}{
		{"bare commit (git add … && git commit)", []string{"commit"}},
		{"commit with a message", []string{"commit", "-F", "-"}},
		{"commit -a", []string{"commit", "-am", "x"}},
		{"commit --amend", []string{"commit", "--amend", "--no-edit"}},
		{"empty pathspec", []string{"commit", "-m", "x", "--"}},
		{"behind git's global options", []string{"-C", "/tmp", "commit", "-m", "x"}},
		{"the qualifier consumed as a message", []string{"commit", "-m", "--"}},
		{"--include, qualifier and all", []string{"commit", "-i", "-m", "x", "--", "a.go"}},
		{"--include spelled long", []string{"commit", "--include", "-m", "x", "--", "a.go"}},
		{"-i bundled into a cluster", []string{"commit", "-im", "x", "--", "a.go"}},
	}
	for _, c := range refused {
		out, errs, code := run(c.argv...)
		if code != 1 || out != "" || !strings.Contains(errs, "(deny: Bash(git commit unless --))") {
			t.Errorf("%s must be refused: code=%d out=%q err=%q", c.what, code, out, errs)
		}
		if !strings.Contains(errs, "safe form: git commit … -- <operand> [<operand>…], and without -i/--include") {
			t.Errorf("%s: refusal must name the safe form, got %q", c.what, errs)
		}
	}
	passed := [][]string{
		{"commit", "-F", "-", "--", "a.go"},
		{"commit", "-m", "x", "--", "a.go", "b.go"},
		{"-C", "/tmp", "commit", "-m", "x", "--", "a.go"},
		{"commit", "--amend", "--no-edit", "--", "a.go"},
		{"status", "-s"},
		{"log", "--", "commit"}, // not the commit verb at all
		// A long option that merely contains the spoiler's letter is not
		// the spoiler, and past `--` a word spelled like one is a path.
		{"commit", "--signoff", "-m", "x", "--", "a.go"},
		{"commit", "--fixup=HEAD", "--", "a.go"},
		{"commit", "-m", "x", "--", "-i"},
	}
	for _, argv := range passed {
		if out, errs, code := run(argv...); code != 0 || !strings.HasPrefix(out, "real git ") {
			t.Errorf("git %s must pass: code=%d out=%q err=%q", strings.Join(argv, " "), code, out, errs)
		}
	}
	// A message that merely contains the token is not the qualifier: the
	// shim compares whole argv words.
	if _, _, code := run("commit", "-m", "see -- here"); code != 1 {
		t.Errorf("a message containing `--` is not a pathspec: code=%d", code)
	}
	logb, _ := os.ReadFile(filepath.Join(gatesDir, "refusals.log"))
	if got := strings.Count(string(logb), "Bash(git commit unless --)"); got != len(refused)+1 {
		t.Errorf("refusals.log lines: %d, want %d:\n%s", got, len(refused)+1, logb)
	}
	// Parity may claim it, and names both layers.
	if kind, faithful := matcherFor("git", ParseShimRules([]string{"Bash(git commit unless --)"})["git"][0]); !faithful ||
		kind != "subcommand, option-aware, negative match" {
		t.Errorf("matcherFor: %q %v", kind, faithful)
	}
	if !deniesUnqualifiedCommit([]string{"Bash(git commit unless --)"}) ||
		deniesUnqualifiedCommit([]string{"Bash(git push:*)", "Bash(git commit:*)"}) {
		t.Error("deniesUnqualifiedCommit")
	}
	// L0 (claude --disallowedTools) has no negation, so the widening is the
	// two EXACT shapes that are unsafe whatever follows — never a prefix or
	// trailing wildcard, which would refuse the safe form too.
	got := strings.Join(L0Spellings([]string{"Bash(git commit unless --)"}), " ")
	if got != "Bash(git commit) Bash(git -* commit)" {
		t.Errorf("L0 spellings for a negative rule: %q", got)
	}
}

// installCommitGuard installs the prepare-commit-msg guard with no config
// behind it, which is the fail-closed default: unmarked repo = public beads
// db = the visibility half armed. Every test below commits files outside
// `.beads/`, so what they exercise is the shared-index half.
func installCommitGuard(dir string) (string, error) {
	p, _, _, err := (&App{}).InstallCommitGuardHook(dir)
	return p, err
}

// ranger-base-i5f4: the §9 chain leaves the slot foreign on purpose, with
// posse's marker-owned hook behind it. Session launch must refresh that member
// too; otherwise a hook installed before linked-worktree support stays frozen
// forever and refuses ordinary commits in each session's private index.
//
// ranger-base-r5ba: run it under BOTH names the docs hand the operator. The
// printed prescription says `theirs-<slot>`; INSTALL.md §9 walks the same
// arrangement with bd's hook moved to `bd-<slot>`, and that is what the repo
// this bug was found in actually has on disk. Matching one spelling refreshed
// neither in practice — the chain is a shape, not a filename.
func TestInstallCommitGuardRefreshesItsChainedHook(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	for _, neighborName := range []string{"theirs-prepare-commit-msg", "bd-prepare-commit-msg"} {
		t.Run(neighborName, func(t *testing.T) {
			repo := t.TempDir()
			gitEnv := []string{"PATH=" + PathOutsideGates(""), "HOME=" + repo,
				"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}
			git := func(dir string, extra []string, args ...string) (string, error) {
				cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
				cmd.Env = append(append([]string(nil), gitEnv...), extra...)
				out, err := cmd.CombinedOutput()
				return string(out), err
			}
			if out, err := git(repo, nil, "init", "-q", "-b", "main"); err != nil {
				t.Fatalf("git init: %v %s", err, out)
			}
			if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if out, err := git(repo, nil, "add", "base.txt"); err != nil {
				t.Fatalf("git add base: %v %s", err, out)
			}
			if out, err := git(repo, nil, "commit", "-qm", "base"); err != nil {
				t.Fatalf("base commit: %v %s", err, out)
			}
			wt := filepath.Join(t.TempDir(), "session")
			if out, err := git(repo, nil, "worktree", "add", "-q", "-b", "posse/session", wt); err != nil {
				t.Fatalf("git worktree add: %v %s", err, out)
			}

			hooks := filepath.Join(repo, ".git", "hooks")
			slot := filepath.Join(hooks, "prepare-commit-msg")
			ours := filepath.Join(hooks, "posse-prepare-commit-msg")
			theirs := filepath.Join(hooks, neighborName)
			dispatcher := chainHookDispatcherWith("prepare-commit-msg", neighborName)
			stale := `#!/bin/sh
` + sharedIndexMarker + ` — stale hook from before linked worktrees
[ -n "$RHQ_PERSONA" ] || exit 0
echo "refused by posse gate: stale shared-index guard" >&2
exit 1
`
			neighbor := "#!/bin/sh\nexit 0\n"
			for path, body := range map[string]string{slot: dispatcher, ours: stale, theirs: neighbor} {
				if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
					t.Fatal(err)
				}
			}

			if err := os.WriteFile(filepath.Join(wt, "mine.txt"), []byte("mine\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if out, err := git(wt, nil, "add", "mine.txt"); err != nil {
				t.Fatalf("git add mine: %v %s", err, out)
			}
			persona := []string{"RHQ_PERSONA=developer", "RHQ_GATES_DIR="}
			if out, err := git(wt, persona, "commit", "-m", "mine"); err == nil || !strings.Contains(out, "stale shared-index guard") {
				t.Fatalf("premise: the stale chained hook must refuse in a linked worktree: %v %s", err, out)
			}

			got, _, _, err := (&App{}).InstallCommitGuardHook(wt)
			if err != nil {
				t.Fatalf("refresh chained hook: %v", err)
			}
			if resolveExisting(got) != resolveExisting(ours) {
				t.Errorf("refreshed path = %q, want marker-owned chain member %q", got, ours)
			}
			if body, _ := os.ReadFile(slot); string(body) != dispatcher {
				t.Error("refresh overwrote the foreign dispatcher")
			}
			if body, _ := os.ReadFile(theirs); string(body) != neighbor {
				t.Error("refresh overwrote the neighboring foreign hook")
			}
			if body, _ := os.ReadFile(ours); !strings.Contains(string(body), "git rev-parse --git-common-dir") {
				t.Fatal("the chained posse hook was not refreshed with the linked-worktree stand-down")
			}
			if out, err := git(wt, persona, "commit", "-m", "mine"); err != nil {
				t.Fatalf("ordinary commit in the linked worktree must pass after refresh: %v %s", err, out)
			}
		})
	}
}

// ranger-base-r5ba: the dispatcher is recognized by shape so the neighbour's
// name may be anything the operator called it — but only the name varies. A
// file that does not run `posse-<slot>` first and check its status is not the
// prescribed chain, and refreshing a sibling behind it would claim a wall that
// nothing calls.
func TestIsChainHookDispatcher(t *testing.T) {
	for _, c := range []struct {
		name string
		body string
		want bool
	}{
		{"printed prescription", chainHookDispatcher("prepare-commit-msg"), true},
		{"INSTALL.md §9 naming", chainHookDispatcherWith("prepare-commit-msg", "bd-prepare-commit-msg"), true},
		{"pre-push keeps its stdin redirect", chainHookDispatcherWith("pre-push", "bd-pre-push"), true},
		{"wrong slot", chainHookDispatcherWith("pre-push", "bd-pre-push"), false},
		{"a plain foreign hook", "#!/bin/sh\necho theirs\n", false},
		{"ours dropped from the chain", "#!/bin/sh\nd=$(dirname \"$0\")\nexec \"$d/bd-prepare-commit-msg\" \"$@\"\n", false},
		{"exit status discarded", strings.Replace(chainHookDispatcher("prepare-commit-msg"), " || exit $?", "", 1), false},
		{"a path, not a sibling", chainHookDispatcherWith("prepare-commit-msg", "../../../bin/sh"), false},
		{"a command, not a name", chainHookDispatcherWith("prepare-commit-msg", "x\"; rm -rf /; :\""), false},
		{"no neighbour at all", chainHookDispatcherWith("prepare-commit-msg", ""), false},
		{"trailing junk", chainHookDispatcher("prepare-commit-msg") + "echo more\n", false},
		// rangerhq-xo65: the unguarded form every chain built before the fix
		// still carries. Its gate runs and its status is checked, so it is
		// still the prescribed chain — recognizing it is what lets install
		// upgrade it rather than declare those repos foreign.
		{"pre-xo65 chain, no guard", legacyChainHookDispatcherWith("prepare-commit-msg", "bd-prepare-commit-msg"), true},
		{"pre-xo65 printed prescription", legacyChainHookDispatcherWith("prepare-commit-msg", "theirs-prepare-commit-msg"), true},
		{"guard names a different neighbour", strings.Replace(chainHookDispatcherWith("prepare-commit-msg", "bd-prepare-commit-msg"), "[ -x \"$d/bd-prepare-commit-msg\" ]", "[ -x \"$d/other\" ]", 1), false},
		{"guard exits non-zero", strings.Replace(chainHookDispatcherWith("prepare-commit-msg", "bd-prepare-commit-msg"), "|| exit 0", "|| exit 1", 1), false},
	} {
		slot := "prepare-commit-msg"
		if c.name == "pre-push keeps its stdin redirect" {
			slot = "pre-push"
		}
		if got := isChainHookDispatcher(c.body, slot); got != c.want {
			t.Errorf("%s: isChainHookDispatcher(_, %q) = %v, want %v", c.name, slot, got, c.want)
		}
	}
}

// rangerhq-lmq9 / rangerhq-nyqj: the L3 half. Driven by real git, because
// the whole guard turns on what git puts in GIT_INDEX_FILE per commit form
// and no fixture can assert that honestly.
func TestSharedIndexCommitHook(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo := t.TempDir()
	gates := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	if CommitGuardHookInstalled(repo) {
		t.Fatal("fresh repo must not report our hook")
	}
	if _, err := installCommitGuard(repo); err != nil {
		t.Fatal(err)
	}
	if !CommitGuardHookInstalled(repo) {
		t.Error("installed hook not detected")
	}
	base := []string{"PATH=" + PathOutsideGates(""), "HOME=" + repo,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}
	git := func(env []string, args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(append([]string(nil), base...), env...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	persona := []string{"RHQ_PERSONA=developer", "RHQ_GATES_DIR=" + gates}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.txt", "a")
	write("b.txt", "b")
	git(nil, "add", "a.txt", "b.txt")
	if out, err := git(nil, "commit", "-qm", "init"); err != nil {
		t.Fatalf("operator's own commit must pass: %v %s", err, out)
	}

	// Shape 1: git add + git commit — the form that swept d15c55a.
	write("a.txt", "a1")
	git(nil, "add", "a.txt")
	out, err := git(persona, "commit", "-m", "sweep")
	if err == nil || !strings.Contains(out, "refused by posse gate: an unqualified git commit") ||
		!strings.Contains(out, "safe form: git commit -F - -- <paths>") {
		t.Errorf("unqualified commit must be refused, naming the safe form: %v %s", err, out)
	}
	// Shape 2: -a, the worst form — and it gets a temp index too, so the
	// message must say which form it was.
	write("b.txt", "b1")
	if out, err := git(persona, "commit", "-am", "sweep-all"); err == nil ||
		!strings.Contains(out, "refused by posse gate: git commit -a") {
		t.Errorf("git commit -a must be refused as -a: %v %s", err, out)
	}
	// --no-verify skips pre-commit but NOT prepare-commit-msg: this slot is
	// why the wall holds.
	if out, err := git(persona, "commit", "--no-verify", "-m", "sneak"); err == nil ||
		!strings.Contains(out, "refused by posse gate") {
		t.Errorf("--no-verify must not walk past the guard: %v %s", err, out)
	}
	// Shape 3: the path-limited form is the way through, and it leaves the
	// other persona's staged entry in the shared index untouched.
	if out, err := git(persona, "commit", "-m", "safe", "--", "b.txt"); err != nil {
		t.Fatalf("path-limited commit must pass: %v %s", err, out)
	}
	if out, _ := git(nil, "diff", "--cached", "--name-only"); strings.TrimSpace(out) != "a.txt" {
		t.Errorf("the other persona's staged entry must survive, got %q", out)
	}
	// An empty pathspec is not a pathspec: git hands the hook .git/index.
	if out, err := git(persona, "commit", "-m", "empty", "--"); err == nil ||
		!strings.Contains(out, "refused by posse gate: an unqualified git commit") {
		t.Errorf("`git commit --` must be refused: %v %s", err, out)
	}
	// The operator, in the same tree, is untouched — this is the same argv
	// that was just refused.
	if out, err := git(nil, "commit", "-m", "operator"); err != nil {
		t.Errorf("operator's unqualified commit must pass: %v %s", err, out)
	}
	// Commits git drives itself cannot take a pathspec, so they pass.
	git(nil, "checkout", "-q", "-b", "side", "HEAD~1")
	write("s.txt", "s")
	git(nil, "add", "s.txt")
	git(nil, "commit", "-qm", "side", "--", "s.txt")
	git(nil, "checkout", "-q", "main")
	if out, err := git(persona, "merge", "--no-edit", "-q", "side"); err != nil {
		t.Errorf("a merge commit must pass (git forbids a pathspec there): %v %s", err, out)
	}
	git(nil, "checkout", "-q", "side")
	write("s.txt", "s2")
	git(nil, "commit", "-qm", "side2", "--", "s.txt")
	git(nil, "checkout", "-q", "main")
	if out, err := git(persona, "cherry-pick", "side"); err != nil || !strings.Contains(out, "side2") {
		t.Errorf("a cherry-pick must pass (CHERRY_PICK_HEAD, no pathspec possible): %v %s", err, out)
	}
	logb, _ := os.ReadFile(filepath.Join(gates, "refusals.log"))
	if n := strings.Count(string(logb), "[prepare-commit-msg hook]"); n != 4 {
		t.Errorf("refusals.log: %d lines, want 4:\n%s", n, logb)
	}
	// Re-install replaces ours; a foreign prepare-commit-msg is left alone.
	p, err := installCommitGuard(repo)
	if err != nil {
		t.Fatalf("re-install of our hook must succeed: %v", err)
	}
	os.WriteFile(p, []byte("#!/bin/sh\necho mine\n"), 0o755)
	if _, err := installCommitGuard(repo); err == nil || !strings.Contains(err.Error(), "not a posse hook") {
		t.Errorf("foreign hook must not be overwritten: %v", err)
	}
	if b, _ := os.ReadFile(p); !strings.Contains(string(b), "echo mine") {
		t.Error("foreign hook was clobbered")
	}
	if _, err := installCommitGuard(t.TempDir()); err == nil {
		t.Error("non-repo must error")
	}
}

// TestLegacyMarkedHooksAreOursToReplace pins the transition arm of the
// posse rename (rangerhq-tyay). Every repo hooked before the rename carries
// the OLD marker; ownership is a question about a file an earlier binary
// wrote, so it has to be asked in both vocabularies or the rename converts
// every already-hooked repo into a repo we refuse to touch — install
// refusing it as a stranger's, and parity reporting the L3 wall missing on
// a repo that has it.
func TestLegacyMarkedHooksAreOursToReplace(t *testing.T) {
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	hooks := filepath.Join(repo, ".git", "hooks")
	os.MkdirAll(hooks, 0o755)

	for _, c := range []struct {
		slot      string
		legacy    string
		installed func(string) bool
		install   func(string) (string, error)
	}{
		{"pre-push", legacyPrePushMarker, PrePushHookInstalled, InstallPrePushHook},
		{"prepare-commit-msg", legacySharedIndexMarker, CommitGuardHookInstalled, installCommitGuard},
	} {
		p := filepath.Join(hooks, c.slot)
		// What the previous binary left behind: the old marker, old wording.
		old := "#!/bin/sh\n" + c.legacy + " — installed by rhq gates install-hooks\nexit 0\n"
		if err := os.WriteFile(p, []byte(old), 0o755); err != nil {
			t.Fatal(err)
		}
		if !c.installed(repo) {
			t.Errorf("%s: a hook written before the rename must still read as installed", c.slot)
		}
		if _, err := c.install(repo); err != nil {
			t.Errorf("%s: re-install over the legacy marker must replace, not refuse: %v", c.slot, err)
		}
		// Replaced in place, and the file written back wears the new marker:
		// the door only swings one way, so the legacy arm cannot go stale.
		b, _ := os.ReadFile(p)
		if strings.Contains(string(b), "exit 0\n") && !strings.Contains(string(b), prePushMarker) && !strings.Contains(string(b), sharedIndexMarker) {
			t.Errorf("%s: legacy hook was not replaced: %q", c.slot, b)
		}
		if !strings.Contains(string(b), "posse-gate") {
			t.Errorf("%s: replacement does not carry the new marker: %q", c.slot, b)
		}
		// And a genuinely foreign hook is still nobody's to overwrite.
		os.WriteFile(p, []byte("#!/bin/sh\necho theirs\n"), 0o755)
		if _, err := c.install(repo); err == nil || !strings.Contains(err.Error(), "not a posse hook") {
			t.Errorf("%s: foreign hook must still be refused: %v", c.slot, err)
		}
	}
}

// rangerhq-cqq1: the exemption used to be a glob on a FILENAME the caller
// picks, so the private-index recipe that reproduces rangerhq-8rtf — commit
// from a private GIT_INDEX_FILE, leave the shared index stale, let the next
// unqualified commit revert it — was refused as `<tmp>/index` and waved
// through as `<tmp>/next-index-mine`. Same recipe, one filename over. The
// wall now asks where the index IS, so every hand-rolled spelling is refused
// and git's own temp index still passes — including in a linked worktree,
// where it lives in the per-worktree git dir, not the common one.
func TestSharedIndexCommitHookRefusesHandRolledNextIndex(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	if _, err := installCommitGuard(repo); err != nil {
		t.Fatal(err)
	}
	base := []string{"PATH=" + PathOutsideGates(""), "HOME=" + repo,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"}
	git := func(dir string, env []string, args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(append([]string(nil), base...), env...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	write := func(dir, name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(repo, "fix.go", "v1")
	write(repo, "other.txt", "o")
	git(repo, nil, "add", "-A")
	if out, err := git(repo, nil, "commit", "-qm", "base"); err != nil {
		t.Fatalf("base commit: %v %s", err, out)
	}
	head, _ := git(repo, nil, "rev-parse", "HEAD")
	head = strings.TrimSpace(head)

	// Every spelling of a hand-rolled private index, run as the full recipe
	// (read-tree + add + commit) so a pass would really land the commit.
	// "" means "inside the repo's own git dir" — the location is right, the
	// name is not pid-shaped, and that is the half the digits rule holds.
	for _, name := range []string{
		"index",           // the spelling that was already refused
		"next-index-mine", // the bypass: a name, not a location
		"next-index-1234", // pid-shaped, still not git's
		"next-index-x/index",
		"", // .git/next-index-mine
	} {
		dir := t.TempDir()
		idx := filepath.Join(dir, name)
		if name == "" {
			name = ".git/next-index-mine"
			idx = filepath.Join(repo, ".git", "next-index-mine")
		}
		if err := os.MkdirAll(filepath.Dir(idx), 0o755); err != nil {
			t.Fatal(err)
		}
		env := []string{"RHQ_PERSONA=qa", "GIT_INDEX_FILE=" + idx}
		write(repo, "fix.go", "v2-THE-FIX")
		if out, err := git(repo, env, "read-tree", "HEAD"); err != nil {
			t.Fatalf("read-tree with GIT_INDEX_FILE=%s: %v %s", name, err, out)
		}
		if out, err := git(repo, env, "add", "--", "fix.go"); err != nil {
			t.Fatalf("add with GIT_INDEX_FILE=%s: %v %s", name, err, out)
		}
		out, err := git(repo, env, "commit", "-m", "the fix")
		if err == nil {
			t.Errorf("GIT_INDEX_FILE=<tmp>/%s must be refused, it landed: %s", name, out)
		} else if !strings.Contains(out, "refused by posse gate: a commit from a private GIT_INDEX_FILE") {
			t.Errorf("GIT_INDEX_FILE=<tmp>/%s refused as the wrong form: %s", name, out)
		}
		if now, _ := git(repo, nil, "rev-parse", "HEAD"); strings.TrimSpace(now) != head {
			t.Fatalf("GIT_INDEX_FILE=<tmp>/%s moved HEAD: %s", name, now)
		}
	}
	git(repo, nil, "checkout", "-q", "--", "fix.go")

	// The way through is untouched, in the main tree…
	write(repo, "fix.go", "v2-THE-FIX")
	if out, err := git(repo, []string{"RHQ_PERSONA=qa"}, "commit", "-m", "safe", "--", "fix.go"); err != nil {
		t.Fatalf("path-limited commit must still pass: %v %s", err, out)
	}
	// …and in a linked worktree, whose next-index-<pid> lives in
	// .git/worktrees/<name>, not in the common git dir. A location check
	// written against the common dir would refuse this.
	wt := filepath.Join(t.TempDir(), "wt")
	if out, err := git(repo, nil, "worktree", "add", "-q", wt, "-b", "side"); err != nil {
		t.Fatalf("worktree add: %v %s", err, out)
	}
	write(wt, "fix.go", "v3-IN-THE-WORKTREE")
	if out, err := git(wt, []string{"RHQ_PERSONA=qa"}, "commit", "-m", "wt", "--", "fix.go"); err != nil {
		t.Errorf("path-limited commit in a linked worktree must pass: %v %s", err, out)
	}
}

// ranger-base-3c3 + ADR 0023: a marker never decides whether a slot works —
// that was always true, and stays true. What changed is what DOES decide:
// not the raw behavior of whatever bytes sit at the dispatch path (a
// planted hook can lie about that — ranger-base-vqvl), but full-byte
// identity of the dispatched file against our render, paired with the
// behavior of OUR OWN render exec'd from a private temp file. A legitimate
// chain dispatcher has no marker at the slot and still counts, because
// posse-<slot> behind it is byte-exact our render. A marker-bearing script
// whose bytes have been altered does not count even though the marker
// survives. And a foreign body with no marker that happens to refuse
// everything — indistinguishable, to a black-box probe, from a hook that
// refuses only the probe — does not count either: the launcher never runs
// it to find out.
func TestL3HookProbeIdentityNotMarkersOrForeignBehavior(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	hooks, err := hooksDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	write := func(slot, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(hooks, slot), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	a := &App{}

	// A legitimate chain dispatcher: no marker at the slot itself, posse-<slot>
	// behind it byte-exact our render. Must count, by identity.
	write("pre-push", chainHookDispatcherWith("pre-push", "theirs-pre-push"))
	write("theirs-pre-push", "#!/bin/sh\nexit 1\n")
	write("posse-pre-push", PrePushHook)
	write("prepare-commit-msg", chainHookDispatcherWith("prepare-commit-msg", "theirs-prepare-commit-msg"))
	write("theirs-prepare-commit-msg", "#!/bin/sh\nexit 1\n")
	write("posse-prepare-commit-msg", CommitGuardHook(VisibilityPublic))
	if got := a.probeL3Hooks(repo, true); !got.Repo || !got.PrePush || !got.CommitGuard {
		t.Errorf("byte-exact chain must count by identity: %+v", got)
	}

	// The inverse of the old defect: both ownership markers survive, but the
	// bytes around them differ from our render (neutralized). Stale, not
	// realized — the marker never decides.
	write("pre-push", "#!/bin/sh\n"+prePushMarker+"\nexit 0\n")
	write("prepare-commit-msg", "#!/bin/sh\n"+sharedIndexMarker+"\nexit 0\n")
	if !PrePushHookInstalled(repo) || !CommitGuardHookInstalled(repo) {
		t.Fatal("fixture must carry both ownership markers")
	}
	if got := a.probeL3Hooks(repo, true); got.PrePush || got.CommitGuard {
		t.Errorf("marker survives but bytes differ from our render — must not count: %+v", got)
	}

	// A foreign body with no marker that behaviorally refuses everything.
	// Must not count: the launcher never execs it to ask.
	write("pre-push", "#!/bin/sh\nexit 1\n")
	write("prepare-commit-msg", "#!/bin/sh\nexit 1\n")
	if got := a.probeL3Hooks(repo, true); got.PrePush || got.CommitGuard {
		t.Errorf("a foreign refuser must not count — identity, not behavior of foreign bytes, is the evidence: %+v", got)
	}

	// The pre-push arm is conditional on the PID; prepare-commit-msg is not,
	// because its visibility and shared-index guards apply to every persona.
	// wantPrePush=false is vacuous for the push arm regardless of what sits
	// there; the commit-guard arm still requires identity.
	os.Remove(filepath.Join(hooks, "pre-push"))
	write("prepare-commit-msg", CommitGuardHook(VisibilityPublic))
	if got := a.probeL3Hooks(repo, false); !got.PrePush || !got.CommitGuard {
		t.Errorf("commit-only probe: %+v", got)
	}
}

// ranger-base-hr5x: `date` is the one real binary the refusal path itself
// runs, and it ran it by BARE NAME. The shim dir leads the session's PATH by
// construction (ADR 0009 §1), so under a PID carrying Bash(date:*) the
// refusal called this persona's own date shim — which refused, logged, and
// called `date` again: an unbounded fork chain, the shape that cost the
// fleet a day in ranger-base-f0ay.
//
// The pin is behavioral and BOUNDED. A decoy `date` sits at the head of the
// PATH the shim runs under, in FRONT of the shim dir, so the lookup this
// test asks about lands on a script that does not recurse: the wrong arm
// costs one extra process instead of a fork storm, which is what makes the
// deny-date arm safe to run on a live box. The decoy is planted in the
// CHILD's env only — on the process PATH it would be what render time
// resolves, and the test would be measuring itself.
func TestShimRefusalNeverLooksUpDateOnThePath(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	if _, err := exec.LookPath("date"); err != nil {
		t.Skip("no date")
	}
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	// A git stub, so an escape from either arm leaks into a file instead of
	// running a real `git push`.
	realBin := t.TempDir()
	leaked := filepath.Join(t.TempDir(), "leaked")
	if err := os.WriteFile(filepath.Join(realBin, "git"), []byte("#!/bin/sh\necho LEAK >>'"+leaked+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", realBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	gatesDir, binDir, _, err := a.RenderGates("developer", []string{"Bash(git push:*)", "Bash(date:*)"})
	if err != nil {
		t.Fatal(err)
	}
	decoyDir := t.TempDir()
	witness := filepath.Join(decoyDir, "witness")
	if err := os.WriteFile(filepath.Join(decoyDir, "date"),
		[]byte("#!/bin/sh\necho DECOY >>'"+witness+"'\necho DECOY-TIME\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	sep := string(os.PathListSeparator)
	childPath := decoyDir + sep + binDir + sep + PathOutsideGates("")

	// The control, and it is not decoration: "the decoy was never called" is
	// also what a PATH that could never reach it says. Measure the
	// instrument first (the fm4p lesson).
	probe := exec.Command("/bin/sh", "-c", "date +%s")
	probe.Env = []string{"PATH=" + childPath}
	if out, err := probe.CombinedOutput(); err != nil || !strings.Contains(string(out), "DECOY-TIME") {
		t.Fatalf("control: a bare `date` on the child's PATH must reach the decoy: %v %q", err, out)
	}
	// That call is itself the decoy's first mark: the assertion below is a
	// no-growth test, so the mark proves the witness file records calls.
	calls := func() int {
		b, err := os.ReadFile(witness)
		if err != nil {
			return 0
		}
		return len(strings.Fields(string(b)))
	}
	if n := calls(); n != 1 {
		t.Fatalf("control: the decoy must record its own call, got %d marks", n)
	}

	run := func(shim string, args ...string) (string, int) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, filepath.Join(binDir, shim), args...)
		cmd.Env = []string{"PATH=" + childPath, "RHQ_GATES_DIR=" + gatesDir}
		out, err := cmd.CombinedOutput()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("%s %v: %v", shim, args, err)
		}
		return string(out), code
	}
	for _, tc := range []struct {
		shim string
		args []string
	}{
		{"git", []string{"push", "origin", "main"}},
		// The verb the refusal itself uses: the recursive arm.
		{"date", []string{"-u", "+%Y"}},
	} {
		out, code := run(tc.shim, tc.args...)
		if code != 1 || !strings.Contains(out, "refused by posse gate: "+tc.shim) {
			t.Errorf("%s %v: code=%d out=%q", tc.shim, tc.args, code, out)
		}
	}
	if n := calls(); n != 1 {
		t.Errorf("the refusal path looked `date` up on PATH (%d decoy calls, 1 is the control's) — in a session the shim dir LEADS that PATH, so this is the fork chain", n)
	}
	if _, err := os.Stat(leaked); err == nil {
		t.Error("shim exec'd the real git")
	}
	logb, _ := os.ReadFile(filepath.Join(gatesDir, "refusals.log"))
	lines := strings.Split(strings.TrimSpace(string(logb)), "\n")
	if len(lines) != 2 {
		t.Fatalf("one line per refusal, not a chain of them: %q", logb)
	}
	stamp := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z `)
	for _, l := range lines {
		if !stamp.MatchString(l) {
			t.Errorf("a refusal line must carry a real UTC timestamp, not a decoy's and not an empty substitution: %q", l)
		}
	}
	// And the rendered script says so on its face, so the bare form is not
	// reintroduced by someone copying the line.
	body, err := os.ReadFile(filepath.Join(binDir, "date"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "$(date ") {
		t.Errorf("posse_refuse must not spell `date` bare:\n%s", body)
	}
	// The escape hatch is the other way the loop could reopen: when `date`
	// is nowhere outside the gates, the line loses its time rather than its
	// bound.
	if strings.Contains(refusalTimestamp(""), "date") {
		t.Errorf("the unresolvable fallback must not spell `date` either: %q", refusalTimestamp(""))
	}
}
