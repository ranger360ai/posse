package posse

// Pins for the transcript ceiling — ADR 0050's third channel as a per-repo
// switch on the launch line (ranger-base-ac22n, operator ruling 2026-11-08).
//
// The pin that matters is the one that is easy to lose: the resolution of a
// SESSION DIRECTORY to the repo the operator marked. A dispatched seat's dir
// is its own linked worktree under ~/.posse/worktrees, SessionTranscripts
// compares by path identity, and a worktree path matches no config key — so a
// WrapWithGates that handed `dir` straight to SessionTranscripts would read
// `on` for every dispatched seat in the marked repo and `off` only for a crew
// session in the shared checkout. That is a wall that is closed where nobody
// needed it and open where the ruling was, and every other signal — a green
// unit test over SessionTranscripts, a launch that completes, a config line
// that says `off` — looks right while it is wrong. arm 3 is that pin and it
// is built on a REAL linked worktree for that reason.
//
// MUTATIONS RUN (each reds the arm named):
//   - `hookRepo(dir)` → `dir` in WrapWithGates: arm 3.
//   - TranscriptPin's `off` arm returns "" : arms 1, 3.
//   - TranscriptPin's `on` arm returns "" (decline instead of unset): arm 2.
//   - ClaudeTranscriptOffValue "1" → "0": arm 5.
//   - SessionTranscripts' default case returns TranscriptsOff: arm 4.
//   - drop TranscriptOffVar from the claude builtin: arms 1, 3, 6.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// tcHome is a posse home whose config.yaml carries a `session_transcripts:`
// block over the dirs named, plus the agent the arms launch as. Returns the
// App.
func tcHome(t *testing.T, persona string, marks map[string]string) *App {
	t.Helper()
	home := t.TempDir()
	for _, d := range []string{"agents", "state", "personas"} {
		if err := os.MkdirAll(filepath.Join(home, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	body := "session_transcripts:\n"
	for dir, v := range marks {
		body += "  " + dir + ": " + v + "\n"
	}
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	pid := "---\nname: " + persona + "\ndescription: qa\n---\nqa persona\n"
	if err := os.WriteFile(filepath.Join(home, "agents", persona+".md"), []byte(pid), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RHQ_HOME", home)
	t.Setenv("POSSE_HOME", home)
	a, err := NewApp()
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// tcClaude is the claude runtime as the launch resolves it, so the arms below
// read the BUILTIN's TranscriptOffVar rather than a literal spelled here — a
// field dropped from the table has to red these, not pass them.
func tcClaude(t *testing.T, a *App) *Runtime {
	t.Helper()
	rt, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	if rt.TranscriptOffVar == "" {
		t.Fatal("fixture: the claude builtin declares no TranscriptOffVar, so every arm here would measure the empty-variable path")
	}
	return rt
}

// ─── arm 1: a marked repo's launch line SETS the variable ────────────────────

func TestQATranscriptCeilingSetsTheVariableForAMarkedRepo(t *testing.T) {
	repo := t.TempDir()
	a := tcHome(t, "qa-tc-off", map[string]string{repo: "off"})
	rt := tcClaude(t, a)

	mode, source := a.SessionTranscripts(repo)
	if mode != TranscriptsOff {
		t.Fatalf("SessionTranscripts(%s) = %q (%s), want %q", repo, mode, source, TranscriptsOff)
	}
	wrapped, _, _, err := a.WrapWithGates("qa-tc-off", rt, nil, repo, "claude --x")
	if err != nil {
		t.Fatal(err)
	}
	want := rt.TranscriptOffVar + "=" + shQuote(ClaudeTranscriptOffValue) + " "
	if !strings.Contains(wrapped, want) {
		t.Errorf("a launch into a %s: %s repo must carry %q on the line:\n%s", TranscriptsConfigKey, TranscriptsOff, want, wrapped)
	}
	// And it must not ALSO unset it: both fragments on one line is a line
	// whose answer depends on which `env` the shell reaches last.
	if strings.Contains(wrapped, "-u "+rt.TranscriptOffVar) {
		t.Errorf("the line both sets and unsets %s:\n%s", rt.TranscriptOffVar, wrapped)
	}
	// The sentence, because a channel closed in silence is indistinguishable
	// from one nobody closed.
	line := TranscriptCeilingLine(rt, mode, source, false)
	if line == "" || !strings.Contains(line, TranscriptsConfigKey) {
		t.Errorf("a marked launch says nothing about it: %q", line)
	}
	// And the CONTAINER tier, where the pin is not rendered at all: the line
	// is the only thing standing in for the wall, so it must say the ceiling
	// is NOT in force rather than repeat the reassurance above.
	caged := TranscriptCeilingLine(rt, mode, source, true)
	if !strings.Contains(caged, "NOT IN FORCE") {
		t.Errorf("a marked launch at the container tier must say the ceiling is not in force — WrapInCage carries no pin, so the channel is open and nothing else reports it: %q", caged)
	}
	if caged == line {
		t.Errorf("the container tier reads the same as the shims tier: %q", caged)
	}
}

// ─── arm 2: an UNMARKED repo's launch line UNSETS it ─────────────────────────

// The arm that makes the pin a guarantee rather than a default. A seat is a
// child of the multiplexer SERVER, whose environment posse cannot read; a
// CLAUDE_CODE_SKIP_PROMPT_HISTORY that reached it by any route would silence
// every seat's transcript on the box, and the loss arrives as no error — the
// cost scan stops counting and the turn-outcome reader stops answering.
func TestQATranscriptCeilingUnsetsTheVariableEverywhereElse(t *testing.T) {
	marked, plain := t.TempDir(), t.TempDir()
	a := tcHome(t, "qa-tc-on", map[string]string{marked: "off"})
	rt := tcClaude(t, a)

	if mode, src := a.SessionTranscripts(plain); mode != TranscriptsOn || src != TranscriptsUnmarkedSource {
		t.Fatalf("an unmarked repo = %q (%s), want %q / the unmarked source", mode, src, TranscriptsOn)
	}
	wrapped, _, _, err := a.WrapWithGates("qa-tc-on", rt, nil, plain, "claude --x")
	if err != nil {
		t.Fatal(err)
	}
	want := "/usr/bin/env -u " + rt.TranscriptOffVar + " "
	if !strings.Contains(wrapped, want) {
		t.Errorf("an unmarked launch must UNSET %s (a leak into the pane's environment silences the box silently), want %q:\n%s", rt.TranscriptOffVar, want, wrapped)
	}
	if strings.Contains(wrapped, rt.TranscriptOffVar+"=") {
		t.Errorf("an unmarked launch set the variable:\n%s", wrapped)
	}
	if line := TranscriptCeilingLine(rt, TranscriptsOn, TranscriptsUnmarkedSource, false); line != "" {
		t.Errorf("an unmarked launch must say nothing — a line per launch saying the default is still the default teaches a reader to skip the stream; got %q", line)
	}
	// `/usr/bin/env` ABSOLUTE: the gates dir leads the PATH this very prefix
	// just set, so a bare `env` would resolve to whichever shim the
	// persona's deny list rendered there (ADR 0077 D3).
	if strings.Contains(wrapped, " env -u ") {
		t.Errorf("the unset arm must spell /usr/bin/env absolute:\n%s", wrapped)
	}
}

// ─── arm 3: a dispatched seat's WORKTREE resolves to the marked repo ─────────

// The defect this whole file exists for. Built on a real `git worktree add`,
// because the resolution is `--git-common-dir` and nothing cheaper reproduces
// it: a plain sibling directory would pass a `hookRepo` that had been mutated
// away.
func TestQATranscriptCeilingReachesASeatInItsOwnWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git on PATH")
	}
	// gitTempDir and not t.TempDir: this arm runs a real `git init` and a
	// `git worktree add`, and a temp root holding a git repo needs the
	// tolerant removal (removeAllTolerant) a concurrent write can otherwise
	// defeat at cleanup.
	root := gitTempDir(t)
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=qa", "GIT_AUTHOR_EMAIL=qa@example.invalid",
			"GIT_COMMITTER_NAME=qa", "GIT_COMMITTER_EMAIL=qa@example.invalid", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run(repo, "init", "-q", "-b", "main", ".")
	if err := os.WriteFile(filepath.Join(repo, "f"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(repo, "add", "f")
	run(repo, "commit", "-qm", "seed", "--", "f")
	tree := filepath.Join(root, "wt")
	run(repo, "worktree", "add", "-q", "-b", "qa-tc", tree)

	// The repo is marked; the WORKTREE is not, and never would be — it is
	// cut fresh per session and the operator has never seen its path.
	a := tcHome(t, "qa-tc-wt", map[string]string{repo: "off"})
	rt := tcClaude(t, a)
	if mode, _ := a.SessionTranscripts(tree); mode != TranscriptsOn {
		t.Fatalf("fixture: the worktree path matched a config key directly (%q) — this arm then measures nothing about the resolution", mode)
	}
	wrapped, _, _, err := a.WrapWithGates("qa-tc-wt", rt, nil, tree, "claude --x")
	if err != nil {
		t.Fatal(err)
	}
	want := rt.TranscriptOffVar + "=" + shQuote(ClaudeTranscriptOffValue) + " "
	if !strings.Contains(wrapped, want) {
		t.Errorf("a seat in a linked worktree of a %s: %s repo got NO ceiling — the dir must resolve through its git common dir, or the wall closes only for a crew session in the shared checkout:\n%s",
			TranscriptsConfigKey, TranscriptsOff, wrapped)
	}
}

// ─── arm 4: every way of being unsure keeps the transcript ───────────────────

func TestQATranscriptCeilingFailsTowardTheRecord(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct{ name, mark string }{
		{"neither word", "maybe"},
		{"empty value", ""},
		{"the on word", "on"},
	} {
		a := tcHome(t, "qa-tc-fc", map[string]string{dir: tc.mark})
		if mode, src := a.SessionTranscripts(dir); mode != TranscriptsOn {
			t.Errorf("%s (%q) = %q (%s), want %q — only an explicit %q closes the channel, because closing it costs the fleet its turn-outcome reader and this seat's share of the cost scan",
				tc.name, tc.mark, mode, src, TranscriptsOn, TranscriptsOff)
		}
	}
	// A home with no config file at all: unreadable is on, same as unmarked.
	bare := t.TempDir()
	t.Setenv("RHQ_HOME", bare)
	t.Setenv("POSSE_HOME", bare)
	ba, err := NewApp()
	if err != nil {
		t.Fatal(err)
	}
	if mode, _ := ba.SessionTranscripts(dir); mode != TranscriptsOn {
		t.Errorf("an unreadable config = %q, want %q", mode, TranscriptsOn)
	}
	// Case folds on the VALUE, and the KEY is compared by identity — which is
	// what makes a config key typed in another case still match on the APFS
	// default (samePath, os.SameFile; the class ranger-base-99gww paid for).
	a := tcHome(t, "qa-tc-case", map[string]string{dir: "OFF"})
	if mode, _ := a.SessionTranscripts(dir); mode != TranscriptsOff {
		t.Errorf("`OFF` = %q, want %q — the value is folded", mode, TranscriptsOff)
	}
}

// ─── arm 5: the VALUE is 1, and 0 is the trap ────────────────────────────────

// The runtime reads this one name under two rules: the transcript resolver
// tests plain JS truthiness, under which "0" is TRUE, and the prompt-history
// writer parses it as a boolean, under which "0" is FALSE. So `0` suppresses
// the transcript and keeps writing prompt history — a half-closed channel
// that reads, from the config line, exactly like a closed one.
func TestQATranscriptCeilingValueIsOneAndNeverZero(t *testing.T) {
	if ClaudeTranscriptOffValue != "1" {
		t.Errorf("ClaudeTranscriptOffValue = %q, want \"1\": the two readers of this name disagree about every other spelling, and \"0\" closes the transcript while leaving prompt history on",
			ClaudeTranscriptOffValue)
	}
	// Nothing takes the value from config: the line says WHETHER, posse says
	// WITH WHAT. A rendered pin must not echo the config value.
	dir := t.TempDir()
	a := tcHome(t, "qa-tc-val", map[string]string{dir: "off"})
	rt := tcClaude(t, a)
	mode, _ := a.SessionTranscripts(dir)
	if got := TranscriptPin(rt, mode); got != rt.TranscriptOffVar+"='1' " {
		t.Errorf("TranscriptPin(off) = %q, want the variable set to '1'", got)
	}
}

// ─── arm 6: a runtime with no measured surface renders NOTHING ───────────────

func TestQATranscriptPinRendersNothingWithoutAMeasuredVariable(t *testing.T) {
	for _, mode := range []string{TranscriptsOff, TranscriptsOn} {
		if got := TranscriptPin(nil, mode); got != "" {
			t.Errorf("TranscriptPin(nil, %s) = %q, want \"\" — WrapWithGates documents rt as nillable", mode, got)
		}
		if got := TranscriptPin(&Runtime{Name: "qa-no-surface"}, mode); got != "" {
			t.Errorf("TranscriptPin(no TranscriptOffVar, %s) = %q, want \"\" — typing a name nothing consumes is a lever that reads as connected and is not", mode, got)
		}
		if line := TranscriptCeilingLine(&Runtime{Name: "qa-no-surface"}, mode, "config", false); line != "" {
			t.Errorf("a runtime with no surface announced a ceiling it does not have: %q", line)
		}
	}
	// And the one runtime that HAS it is claude, which is the whole of the
	// declaration today — every other built-in is a gap, named as such in
	// transcript.go, not a decision about that CLI.
	a := tcHome(t, "qa-tc-only", nil)
	for _, name := range []string{"codex", "grok"} {
		rt, err := a.LoadRuntime(name)
		if err != nil {
			continue
		}
		if rt.TranscriptOffVar != "" {
			t.Errorf("%s declares TranscriptOffVar %q — if a suppression surface was MEASURED for it, say so where transcript.go names the gap", name, rt.TranscriptOffVar)
		}
	}
}

// ─── arm 7: the pin composes with the runtime-home pin, all four ways ────────

// Two of the four combinations put a prefix assignment AFTER an `env -u`,
// which is `env` taking a NAME=VALUE operand, and two put an `env -u` after an
// assignment. All four are run through a real /bin/sh, because "it should
// compose" is the kind of sentence a launch line proves or does not.
func TestQATranscriptPinComposesWithTheRuntimeHomePin(t *testing.T) {
	rt := &Runtime{Name: "qa-compose", HomeVar: "QA_TC_HOME", TranscriptOffVar: "QA_TC_SKIP"}
	for _, home := range []struct{ name, val string }{{"home in force", t.TempDir()}, {"home unset", ""}} {
		for _, mode := range []string{TranscriptsOff, TranscriptsOn} {
			t.Setenv("QA_TC_HOME", home.val)
			t.Setenv("QA_TC_SKIP", "leaked-from-the-pane")
			// The reader is a CHILD shell, inside single quotes, so the
			// outer shell cannot expand the two names before the prefix
			// assignments are in force — which is what a seat is: a
			// process started under this line, reading its OWN environment.
			line := RuntimeHomePin(rt) + TranscriptPin(rt, mode) +
				`/bin/sh -c 'printf "%s|%s" "${QA_TC_HOME-UNSET}" "${QA_TC_SKIP-UNSET}"'`
			out, err := exec.Command("/bin/sh", "-c", line).CombinedOutput()
			if err != nil {
				t.Fatalf("%s / %s: %v\n%s\nline: %s", home.name, mode, err, out, line)
			}
			parts := strings.SplitN(string(out), "|", 2)
			if len(parts) != 2 {
				t.Fatalf("%s / %s: unreadable output %q from %s", home.name, mode, out, line)
			}
			wantHome := home.val
			if wantHome == "" {
				wantHome = "UNSET"
			}
			if parts[0] != wantHome {
				t.Errorf("%s / %s: home var = %q, want %q (line: %s)", home.name, mode, parts[0], wantHome, line)
			}
			wantSkip := "UNSET"
			if mode == TranscriptsOff {
				wantSkip = ClaudeTranscriptOffValue
			}
			if parts[1] != wantSkip {
				t.Errorf("%s / %s: transcript var = %q, want %q — the pane's leaked value survived (line: %s)", home.name, mode, parts[1], wantSkip, line)
			}
		}
	}
}

// ─── arm 8: no launch ever renders cleanupPeriodDays ─────────────────────────

// The lever the opening bead was written around, and the one that must never
// appear: the shipped validator types the key positive, so a payload carrying
// `0` is REJECTED — and a single rejected row makes the runtime discard the
// whole `--settings` object in silence (fieldpin.go, MEASURED 2026-09-05),
// which would take the credential-dir pin, every inlet row and the fleet's
// permission mode off the line while it was busy failing to stop a
// transcript. A source-text sweep because the risk is a future edit, not
// today's code.
func TestQANoLaunchPayloadNamesCleanupPeriodDays(t *testing.T) {
	t.Parallel()
	const key = "cleanupPeriodDays"
	for _, got := range []string{
		ClaudeFleetSettings,
		ClaudeFleetSettingsJSON(nil),
		credentialDirPinJSON(),
	} {
		if strings.Contains(got, key) {
			t.Errorf("a rendered settings payload names %s:\n%s\nThe validator types it positive, so 0 is rejected — and a rejected row discards the WHOLE payload in silence. The transcript ceiling is an environment variable (transcript.go), not a retention number.", key, got)
		}
	}
	for _, f := range fieldPin() {
		if f.Key == key {
			t.Errorf("fieldPin carries a %s row", key)
		}
	}
	for _, v := range settingsPin() {
		if strings.Contains(v.Key, key) {
			t.Errorf("settingsPin carries a %s row", key)
		}
	}
}

// ─── arm 9: the REAL launch line, and the relaunch that re-types it ──────────

// Arms 1-3 read WrapWithGates, which is where the pin is rendered and where a
// mutation of it reds. This arm reads the line posse actually typed, through
// the raw call log and the spill file, because `calls()` collapses this
// fragment into the `GATES ` marker (herdr_test.go, gatePrefixRe) — so no
// other launch-level assertion in this package can see it, and "all three
// sites carry it" would otherwise be a property of one function call nobody
// checked from the outside. The relaunch half is the one that has drifted
// before (ranger-base-az23f, verifyesa0j): it renders its own line and
// trusts nothing in the meta.
func TestQATranscriptCeilingIsOnTheLineTheLaunchTyped(t *testing.T) {
	b, fake := newTestBackend(t)
	a := b.App
	repo := t.TempDir()
	if err := os.WriteFile(a.ConfigPath, []byte(TranscriptsConfigKey+":\n  "+repo+": "+TranscriptsOff+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if mode, _ := a.SessionTranscripts(repo); mode != TranscriptsOff {
		t.Fatalf("fixture: the config did not take (%q) — the arms below would measure an unmarked launch", mode)
	}
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.AgentsDir, "qa-tc-line.md"),
		[]byte("---\nname: qa-tc-line\ndeny: [Bash(git push:*)]\n---\nYou are a qa persona.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustCreate(t, b, NewSessionOpts{Name: "tcl", Agent: "qa-tc-line", Dir: repo})

	rt := tcClaude(t, a)
	want := rt.TranscriptOffVar + "=" + shQuote(ClaudeTranscriptOffValue) + " "
	launched := rawCalls(t, fake) + spilled(t, a, "tcl")
	if !strings.Contains(launched, want) {
		t.Errorf("the launch into a %s: %s repo must carry %q on the line it typed:\n%s", TranscriptsConfigKey, TranscriptsOff, want, launched)
	}
	// And the sentence, on the launch's own warn stream — the only report
	// that the fleet just gave up its turn-outcome reader and this seat's
	// share of the cost scan.
	if w := warnBuf(t, b).String(); !strings.Contains(w, "transcript ceiling") {
		t.Errorf("the launch said nothing about the ceiling:\n%s", w)
	}

	// The relaunch path: it renders its own line, so it has to re-type the
	// pin or a CLI that died comes back writing transcripts into a repo the
	// operator marked off.
	// Best-effort, as runtimehome_qa_test.go's own relaunch arm does it: the
	// file is the fake's agent roster and a fixture that never wrote one is
	// already in the state this removal is arranging — the CLI is not there.
	_ = os.Remove(filepath.Join(fake, "agents.json"))
	m, _ := b.readMeta("tcl")
	m.Launched = m.Launched.Add(-time.Hour)
	if err := b.writeMeta(m); err != nil {
		t.Fatal(err)
	}
	if ok, err := b.RelaunchAgent("tcl", time.Second); err != nil || !ok {
		t.Fatalf("relaunch: %v %v", ok, err)
	}
	if retyped := rawCalls(t, fake) + spilled(t, a, "tcl"); !strings.Contains(retyped, want) {
		t.Errorf("the relaunch must re-type the transcript pin verbatim (%q):\n%s", want, retyped)
	}
}
