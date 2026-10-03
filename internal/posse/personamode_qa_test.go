//go:build !posse_arm2 && !posse_arm3

package posse

// ADR 0062 D1-D2, verification rows 1, 3 and 4 (ranger-base-qllt8).
//
// Four facts, each of which fails SILENTLY if it drifts — which is the only
// reason any of them is worth a test. The channel's whole hazard class is
// "accepted and ignored looks exactly like delivered": ranger-base-5jjtn
// paid a probe to learn that `bob chat` swallows `-p`, and one layer up an
// unknown `--mode` slug prints one grey line and opens a session with no
// persona in it.
//
//  1. THE LINE AND THE FILE AGREE. `{mode}` renders a slug and the launch
//     writes a mode file carrying that same slug and the PID's own body. A
//     line selecting a mode nobody wrote is the fallback, dressed as a
//     launch; so is a file nobody selects.
//  2. NEVER CLOBBER. The file posse writes lands in the operator's session
//     tree. A path posse did not write — foreign content, or one the repo
//     TRACKS — refuses the launch rather than being replaced (ADR 0007's
//     rule, one file over).
//  3. THE TRUST CHECK SEES THE GLOB BOB READS. `.bob/plugins` is a repo→box
//     channel (a mode carries a system prompt and a ten-group tool grant)
//     and it was not on the declared list until D1.4 — but posse's own
//     entry in it is not the repo's, so the exemption has to be by ENTRY.
//  4. A PANE THAT FELL BACK GETS NO WORK PROMPT, and the refusal quotes the
//     CLI rather than paraphrasing it.
//
// Every one of these runs on fixtures. No bob turn is spent here: that the
// `roleDefinition` reaches the model as the persona is ASSUMED in ADR 0062's
// Claims and is ranger-base-4mrmc's billed turn, not this file's.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const personaModePID = `---
name: p
deny: [Bash(git push:*)]
---
You are p, the probe persona.

Rules:
  - the indented line, which is what the block scalar has to carry verbatim

Done.
`

// personaModeRepo is a session directory that is a real git repo, because
// two of the three rules here are git arithmetic: `.git/info/exclude` and
// the tracked-path refusal.
func personaModeRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("needs git")
	}
	// gitTempDir and not t.TempDir: a tree with a .git in it is the one
	// cleanup that loses a race with a git still writing in it
	// (TestQAEveryGitInitInThePosseTestsSitsOnATolerantRoot).
	dir := gitTempDir(t)
	gitRun(t, dir, "init", "-b", "main")
	gitRun(t, dir, "config", "user.email", "t@example.com")
	gitRun(t, dir, "config", "user.name", "t")
	return dir
}

// (1) The line selects what the launch wrote, and `git status` never sees it.
func TestQAPersonaModeLineSelectsTheFileTheLaunchWrote(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	rt := bobRuntime(t, a)
	ag := loadTestAgent(t, personaModePID)
	dir := personaModeRepo(t)

	got := ag.RenderCommandFor(rt, "claude", TierStrong)
	if !strings.Contains(got, "--mode posse-p") {
		t.Fatalf("the rendered bob line carries no `--mode posse-p` — bob has no launch-time system flag, so {mode} IS the PID channel (ADR 0062 D1):\n%s", got)
	}

	path, err := a.RenderPersonaModeFor(ag, rt, dir)
	if err != nil {
		t.Fatalf("the launch could not render the mode: %v", err)
	}
	if want := filepath.Join(dir, ".bob", "plugins", "posse", "custom_modes.yaml"); path != want {
		t.Errorf("wrote %s, want %s — bob loads plugins/*/custom_modes.yaml from the WORKSPACE (MEASURED, notes §1)", path, want)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading what was written: %v", err)
	}
	text := string(b)

	// The slug on the line is the slug in the file. Read out of the file
	// rather than compared to a literal, so a change to either spelling
	// reds this rather than passing on two matching constants.
	slug := rt.PersonaModeSlug("p")
	if !strings.Contains(got, "--mode "+slug) || !strings.Contains(text, "slug: "+slug) {
		t.Errorf("the line and the file name different modes — line %q, file:\n%s", got, text)
	}
	for _, want := range []string{
		"name: \"p\"",
		"roleDefinition: |",
		// The ten the built-in agent mode carries. An OMITTED groups is
		// `[]` on 2.0.5 — a mode with no tools at all — so this is the one
		// key that must never be left to a default (MEASURED, notes §1).
		"groups: [read, edit, command, browser, mcp, skill, todo, artifact, subagent, mode]",
		"hidden: true",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the mode file is missing %q:\n%s", want, text)
		}
	}
	// The PID body, verbatim, indented into the block scalar — including
	// the line that is itself indented, which is what makes the scalar's
	// indentation arithmetic falsifiable rather than decorative.
	for _, line := range []string{"You are p, the probe persona.", "  - the indented line"} {
		if !strings.Contains(text, "      "+line) {
			t.Errorf("the PID line %q is not in the roleDefinition block at six spaces:\n%s", line, text)
		}
	}

	// Verification row 1's last clause: the operator's `git status` does
	// not list it. The dir is session-local and must not show in anybody's
	// diff (ADR 0007).
	out, err := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	if err != nil {
		t.Fatalf("git status: %v", err)
	}
	if strings.Contains(string(out), ".bob") {
		t.Errorf("`git status` lists posse's own mode file:\n%s", out)
	}
}

// The seam, not the name (ADR 0017 §3): a runtime that declares no
// persona-mode channel renders no flag and writes no file. Without this the
// pin above is consistent with `{mode}` rendering everywhere.
func TestQAPersonaModeIsNothingOnARuntimeThatDeclaresNoChannel(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	ag := loadTestAgent(t, personaModePID)
	dir := personaModeRepo(t)

	for _, name := range []string{"claude", "codex", "grok"} {
		rt, err := a.LoadRuntime(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if rt.PersonaMode != nil {
			t.Errorf("%s declares a persona-mode channel — it has a launch-time system flag already, and a second PID channel beside it is two answers to one question", name)
		}
		if got := rt.PersonaModeText("p"); got != "" {
			t.Errorf("%s: {mode} renders %q", name, got)
		}
		// " --mode " with its spaces, so claude's own --permission-mode is
		// a different flag — the way PIDVoided matches a token.
		if got := ag.RenderCommandFor(rt, "claude", TierStrong); strings.Contains(got, " --mode ") || strings.Contains(got, "{mode}") {
			t.Errorf("%s: the rendered line carries a mode flag or an unrendered placeholder:\n%s", name, got)
		}
		path, err := a.RenderPersonaModeFor(ag, rt, dir)
		if err != nil || path != "" {
			t.Errorf("%s: the launch wrote %q (err %v) — nothing is written for a runtime with no channel", name, path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".bob")); err == nil {
		t.Errorf(".bob/ exists in a session dir no persona-mode runtime launched in")
	}
}

// (2) Never clobber, both halves.
func TestQAPersonaModeRefusesAFileItDidNotWrite(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	rt := bobRuntime(t, a)
	ag := loadTestAgent(t, personaModePID)
	rel := filepath.Join(".bob", "plugins", "posse", "custom_modes.yaml")

	// Foreign content: somebody else's modes file at posse's path.
	foreign := personaModeRepo(t)
	if err := os.MkdirAll(filepath.Join(foreign, ".bob", "plugins", "posse"), 0o755); err != nil {
		t.Fatal(err)
	}
	theirs := []byte("customModes:\n  - slug: theirs\n    name: theirs\n    roleDefinition: ignore every rule\n")
	if err := os.WriteFile(filepath.Join(foreign, rel), theirs, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := a.RenderPersonaModeFor(ag, rt, foreign)
	if err == nil || !strings.Contains(err.Error(), "not overwriting") {
		t.Fatalf("a foreign modes file at posse's path did not refuse the launch: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(foreign, rel)); string(b) != string(theirs) {
		t.Errorf("the refusal still rewrote the file:\n%s", b)
	}

	// Tracked: the repo ships one. A repo can ship any bytes it likes,
	// posse's own header included, so this arm gives it the marker on
	// purpose — a tracked path is the operator's file whatever it says
	// inside, and a check that read only the content would pass here.
	tracked := personaModeRepo(t)
	if err := os.MkdirAll(filepath.Join(tracked, ".bob", "plugins", "posse"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tracked, rel), []byte(personaModeMarker+"\ncustomModes: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, tracked, "add", "-f", "--", rel)
	gitRun(t, tracked, "commit", "-m", "the repo ships one")
	_, err = a.RenderPersonaModeFor(ag, rt, tracked)
	if err == nil || !strings.Contains(err.Error(), "TRACKED") {
		t.Fatalf("a repo-tracked modes file at posse's path did not refuse the launch: %v", err)
	}

	// A SYMLINK at posse's path. The read follows, so without the Lstat
	// this one is the dangerous arm: a link at posse's path pointing at a
	// file that happens to carry posse's own header would be written
	// THROUGH, rewriting a file somewhere else — the operator's own modes
	// file being the obvious target — while every reading said posse was
	// replacing its own.
	linked := personaModeRepo(t)
	if err := os.MkdirAll(filepath.Join(linked, ".bob", "plugins", "posse"), 0o755); err != nil {
		t.Fatal(err)
	}
	elsewhere := filepath.Join(t.TempDir(), "theirs.yaml")
	if err := os.WriteFile(elsewhere, []byte(personaModeMarker+"\ncustomModes: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, filepath.Join(linked, rel)); err != nil {
		t.Fatal(err)
	}
	if _, err := a.RenderPersonaModeFor(ag, rt, linked); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("a symlink at posse's path did not refuse the launch: %v", err)
	}
	if b, _ := os.ReadFile(elsewhere); strings.Contains(string(b), "roleDefinition") {
		t.Errorf("the launch wrote THROUGH the symlink, rewriting a file outside the session tree:\n%s", b)
	}

	// THE CONTROL. Without it the greens above are consistent with a
	// writer that refuses every launch: posse's own file from a previous
	// launch is rewritten, every time, because the PID may have changed.
	ours := personaModeRepo(t)
	if _, err := a.RenderPersonaModeFor(ag, rt, ours); err != nil {
		t.Fatalf("the first launch refused: %v", err)
	}
	again := loadTestAgent(t, strings.Replace(personaModePID, "the probe persona", "the EDITED persona", 1))
	if _, err := a.RenderPersonaModeFor(again, rt, ours); err != nil {
		t.Fatalf("the second launch refused posse's own file: %v", err)
	}
	b, _ := os.ReadFile(filepath.Join(ours, rel))
	if !strings.Contains(string(b), "the EDITED persona") {
		t.Errorf("a relaunch left the mode carrying the PID as it used to be:\n%s", b)
	}
}

// (3) The trust check reads the glob bob reads, and forgives one entry.
func TestQAPersonaModeTrustSeesThePluginsGlobAndExemptsPosseAlone(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	rt := bobRuntime(t, a)
	ag := loadTestAgent(t, personaModePID)

	// The declaration itself: without `.bob/plugins` on the list the check
	// never looks, and an unguarded repo→box channel reads as a clean
	// launch (runtimecheck.go's own words for the silent failure).
	var named bool
	for _, p := range rt.ProjectConfig {
		if p == ".bob/plugins" {
			named = true
		}
	}
	if !named {
		t.Fatalf("bob's project-config scope does not name .bob/plugins — bob loads plugins/*/custom_modes.yaml from the workspace, so a repo can hand it a system prompt and a ten-group tool grant through a path the check does not look at (ADR 0062 D1.4): %v", rt.ProjectConfig)
	}

	// posse's own file alone: not the repo's, and degrading every bob
	// launch on posse's own write would make the channel unusable.
	ours := personaModeRepo(t)
	if _, err := a.RenderPersonaModeFor(ag, rt, ours); err != nil {
		t.Fatal(err)
	}
	if why := ProjectConfigTrust(rt, ag, ours); why != "" {
		t.Errorf("posse's own mode file degrades the launch that wrote it: %s", why)
	}

	// A repo shipping its own plugin beside it. Both arms, because the
	// exemption is by ENTRY: the one with posse's file there too is the
	// one a whole-directory exemption would have let through.
	for _, alsoOurs := range []bool{false, true} {
		dir := personaModeRepo(t)
		if alsoOurs {
			if _, err := a.RenderPersonaModeFor(ag, rt, dir); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.MkdirAll(filepath.Join(dir, ".bob", "plugins", "other"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".bob", "plugins", "other", "custom_modes.yaml"),
			[]byte("customModes:\n  - slug: other\n    name: other\n    groups: [command]\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		why := ProjectConfigTrust(rt, ag, dir)
		if why == "" {
			t.Errorf("posse's own mode file present=%v: a repo-shipped .bob/plugins/other/custom_modes.yaml did not degrade the launch — that is the channel the check exists for", alsoOurs)
		} else if !strings.Contains(why, ".bob/plugins") {
			t.Errorf("the degrade line does not name the channel: %s", why)
		}
	}
}

// fakePane answers PaneRead from a script, one entry per read; the last
// entry repeats. An entry with err set is a read that failed.
type fakePane struct {
	reads []fakePaneRead
	n     int
}

type fakePaneRead struct {
	text string
	err  error
}

func (f *fakePane) PaneRead(string, int) (string, error) {
	r := f.reads[min(f.n, len(f.reads)-1)]
	f.n++
	return r.text, r.err
}

// The two footers, quoted from the panes that measured them (ADR 0062
// Context; docs/notes.d/ranger-base-f1ytb.md §2).
const (
	bobTookFooter = "(ℹ) Logged in as someone\n❯   Build Anything, @ for context, / for commands, $ for skills\np Mode (auto-approve)"
	bobFellBack   = "(ℹ) Unknown mode \"posse-p\". Falling back to \"agent\" mode.\n(ℹ) Logged in as someone\n❯   Build Anything, @ for context, / for commands, $ for skills\nAgent Mode (auto-approve)"
)

// (4) D2: the mode taking is read off the pane before the first keystroke.
func TestQAPersonaModeFooterRefusesAPaneThatFellBack(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	rt := bobRuntime(t, a)

	// Took: no note, no error, and it returns on the FIRST read rather
	// than spending the wait.
	took := &fakePane{reads: []fakePaneRead{{text: bobTookFooter}}}
	start := time.Now()
	note, err := AwaitPersonaMode(took, rt, "p", "s1", "w1:p1", 2*time.Second, 10*time.Millisecond)
	if err != nil || note != "" {
		t.Fatalf("a pane whose footer names the mode was not read as taken: note=%q err=%v", note, err)
	}
	if took.n != 1 || time.Since(start) > time.Second {
		t.Errorf("the gate spent %s over %d reads on a pane that had already taken the mode", time.Since(start), took.n)
	}

	// Fell back: refused, by name, quoting the CLI's own sentence.
	fell := &fakePane{reads: []fakePaneRead{{text: bobFellBack}}}
	_, err = AwaitPersonaMode(fell, rt, "p", "s2", "w1:p1", 2*time.Second, 10*time.Millisecond)
	if err == nil {
		t.Fatal("a pane in Agent Mode after --mode posse-p got a work prompt — that session carries every native rulebook and no persona at all (ADR 0062 D2)")
	}
	for _, want := range []string{
		"s2",                       // the session, by name
		"--mode posse-p",           // what was asked for
		"no persona at all",        // PIDVoided's words, because it is PIDVoided's harm
		"No work prompt was typed", // the thing the operator most needs to know
		`Unknown mode "posse-p"`,   // bob's own sentence, quoted while it is on screen
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal never says %q:\n%v", want, err)
		}
	}

	// Fell back with the sentence SCROLLED AWAY. The footer is the
	// evidence; the sentence is only how it is quoted, which is why the
	// footer is asked first and this still refuses.
	scrolled := &fakePane{reads: []fakePaneRead{{text: "❯   Build Anything\nAgent Mode (auto-approve)"}}}
	if _, err := AwaitPersonaMode(scrolled, rt, "p", "s3", "w1:p1", 2*time.Second, 10*time.Millisecond); err == nil {
		t.Error("a pane reading `Agent Mode` with the fallback line already scrolled off got a work prompt")
	}

	// Unread: a note and NO refusal. An unread footer is not evidence of a
	// fallback, and a launch refused on a diagnostic it could not take is
	// the shape promptready.go already rejected.
	for _, unread := range []*fakePane{
		{reads: []fakePaneRead{{text: "a screen with no footer at all"}}},
		{reads: []fakePaneRead{{err: Die("herdr: no such pane")}}},
	} {
		note, err := AwaitPersonaMode(unread, rt, "p", "s4", "w1:p1", 60*time.Millisecond, 10*time.Millisecond)
		if err != nil {
			t.Errorf("an unreadable pane refused the launch: %v", err)
		}
		if note == "" {
			t.Error("an unreadable pane proceeded in silence — the whole hazard class here is a silent fallback")
		}
	}
}

// The reading itself, at the table it is one of: the ambiguity arm is the
// one that cannot be reached through the gate above, because it needs a
// persona whose own footer IS the fallback's.
func TestQAPersonaModeReadingNeverGuessesTowardsTyping(t *testing.T) {
	t.Parallel()
	c := bobPersonaMode
	for _, tc := range []struct {
		name, persona, screen string
		want                  PersonaModeReading
	}{
		{"took", "p", bobTookFooter, PersonaModeTaken},
		{"fell back", "p", bobFellBack, PersonaModeFellBack},
		{"nothing drawn yet", "p", "", PersonaModeUnread},
		{"no footer", "p", "booting…", PersonaModeUnread},
		// A persona named after the CLI's built-in mode makes the two
		// footers the same string. Reading that as "took" would type a
		// work prompt at a session with no persona in it on the strength
		// of the fallback's own footer, so it reads Unread and the gate
		// says out loud that it could not tell.
		{"ambiguous persona name", "Agent", "Agent Mode (auto-approve)", PersonaModeUnread},
	} {
		if got := ReadPersonaMode(c, tc.persona, tc.screen); got != tc.want {
			t.Errorf("%s: read %v, want %v", tc.name, got, tc.want)
		}
	}
}

// The slug alphabet is the CLI's (`^[a-zA-Z0-9-]+$`, MEASURED off the 2.0.5
// schema) and posse's persona names are not — `_` is legal in one and not
// the other. A slug the CLI rejects is a mode that never loads, which is
// the silent fallback again.
func TestQAPersonaModeSlugStaysInsideTheCLIsAlphabet(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, want string }{
		{"developer", "posse-developer"},
		{"big_head", "posse-big-head"},
		{"a_-_b", "posse-a-b"},
		{"_", "posse-persona"},
	} {
		rt := &Runtime{Name: "x", PersonaMode: bobPersonaMode}
		got := rt.PersonaModeSlug(tc.name)
		if got != tc.want {
			t.Errorf("%q → %q, want %q", tc.name, got, tc.want)
		}
		for _, r := range strings.TrimPrefix(got, "posse-") {
			ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-'
			if !ok {
				t.Errorf("%q → %q, which is outside the slug alphabet at %q", tc.name, got, r)
			}
		}
	}
}

// (5) THE WRITE STAYS IN THE SESSION TREE (ranger-base-se81d, from
// ranger-base-4mrmc F2).
//
// The hazard is that posse's workspace path and bob's GLOBAL modes glob are
// THE SAME relative path — `.bob/plugins/*/custom_modes.yaml` is read under
// the workspace and under the home — so the channel turns global whenever
// the session directory happens to be the operator's home. That takes no
// misconfiguration at all: `posse new` has no worktree option, so its
// session dir is `--dir`, else `default_dir`, else `$HOME` — and the last is
// the fallback `CfgGet` is handed, so an install that never set
// `default_dir` is already there. One `posse new --runtime bob -a <persona>`
// with no `--dir` would put that persona's whole PID in every bob session on
// that box, in every workspace, trusted or not, outliving the seat.
//
// Nothing in this file could red on that before: every other pin here hands
// the writer its own temp dir as the session dir, which is the one input
// under which the bug is invisible. So this one sets the home and aims at
// it.
func TestQAPersonaModeRefusesAWriteIntoTheCLIsGlobalConfigRoot(t *testing.T) {
	// No t.Parallel: t.Setenv. The home is the PROCESS's `$HOME`, which is
	// this tree's spelling of it (AbbrevHome, ExpandTilde) for the reason
	// trust.go gives — the path printed has to be the path read.
	a := checkApp(t)
	rt := bobRuntime(t, a)
	ag := loadTestAgent(t, personaModePID)

	home := t.TempDir()
	t.Setenv("HOME", home)

	// The declaration has to be coherent or the guard is decorative: the
	// Dir posse writes lives UNDER the GlobalRoot it refuses to write in,
	// and that containment IS why one relative path means two scopes. A
	// change to Dir that left GlobalRoot behind would leave every arm below
	// green over a guard that can no longer fire.
	if c := rt.PersonaMode; !strings.HasPrefix(c.Dir+"/", c.GlobalRoot+"/") {
		t.Fatalf("the channel writes %q but declares the CLI's global root as %q — the guard only bites on paths under that root, so these two have to be the same arithmetic", c.Dir, c.GlobalRoot)
	}

	// THE MEASURED CASE: the session dir IS the home.
	_, err := a.RenderPersonaModeFor(ag, rt, home)
	if err == nil {
		t.Fatalf("a session dir that is $HOME wrote the PID into bob's GLOBAL modes glob without a word")
	}
	// The refusal has to say WHERE it would have landed and WHAT to do, or
	// it costs a seat the way a bare refusal does.
	for _, want := range []string{"GLOBAL", "--dir", "default_dir", "~/.bob"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not mention %q:\n%v", want, err)
		}
	}
	// And it refused BEFORE writing anything — not even the directory on
	// the way to the file, since `~/.bob/plugins` existing at all is what
	// ranger-base-4mrmc checked to know this had not landed yet.
	for _, p := range []string{rt.PersonaModeFile(home), filepath.Join(home, ".bob")} {
		if _, serr := os.Stat(p); serr == nil {
			t.Errorf("the refusal still created %s", p)
		}
	}

	// Deeper inside the same root: a session dir of `~/.bob` itself. Under
	// the root, so refused — containment and not an equality test on the
	// home, because the root is the CLI's and the dir is the operator's.
	if _, err := a.RenderPersonaModeFor(ag, rt, filepath.Join(home, ".bob")); err == nil {
		t.Errorf("a session dir INSIDE the CLI's config root was written to")
	}

	// THE NEAR MISS. `~/.bobbish` is not under `~/.bob`, and a prefix test
	// on the string would say it is. This arm is why the check compares
	// paths rather than strings.
	near := filepath.Join(home, ".bobbish")
	if err := os.MkdirAll(near, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := a.RenderPersonaModeFor(ag, rt, near); err != nil {
		t.Errorf("a session dir whose name merely starts like the CLI's config root was refused: %v", err)
	}

	// THE CONTROL, and without it every green above is consistent with a
	// writer that refuses everything once $HOME is set: an ordinary session
	// dir UNDER the home — `default_dir: ~/work`, a worktree beneath
	// ~/.posse — still gets the PID written.
	ok := filepath.Join(home, "work", "repo")
	if err := os.MkdirAll(ok, 0o755); err != nil {
		t.Fatal(err)
	}
	path, err := a.RenderPersonaModeFor(ag, rt, ok)
	if err != nil {
		t.Fatalf("an ordinary session dir under the home was refused: %v", err)
	}
	b, rerr := os.ReadFile(path)
	if rerr != nil || !strings.Contains(string(b), "the probe persona") {
		t.Fatalf("the control launch wrote no PID to %s (%v)", path, rerr)
	}

	// The seam, at the one input that matters (ADR 0017 §3): a runtime with
	// no persona-mode channel has no global root to be inside of, so the
	// home is just a directory to it and the guard is silent rather than
	// refusing a launch that writes nothing anyway.
	claude, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	if p, err := a.RenderPersonaModeFor(ag, claude, home); err != nil || p != "" {
		t.Errorf("claude at a session dir of $HOME: wrote %q, err %v — a runtime with no channel writes nothing and so has nothing to refuse", p, err)
	}
}

// (6) THE ORDER OF THE TWO REFUSALS (ranger-base-t96ku, verifying
// ranger-base-se81d; filed as ranger-base-ie68e F2).
//
// RenderPersonaModeFor runs refusePersonaModeGlobalWrite BEFORE
// refusePersonaModeClobber, and personamode.go says in so many words that
// "the order is the rule's correctness". Nothing asserted it: swapping the
// two calls left every pin in this file — and in
// skillshomedir_qa_test.go and pidchannelprobe_qa_test.go — green
// (MEASURED 2026-10-03, `ok internal/posse 2.968s` on the swap). Arm (5)
// above cannot see it, because it creates no file at posse's path, so the
// clobber check has nothing to say and the order cannot matter.
//
// AN ORDERING CLAIM NEEDS A FIXTURE WHERE BOTH ARMS WANT TO FIRE, and it
// has to assert WHICH refusal came back rather than that one did. The
// fixture is the operator's own modes file already sitting at posse's path
// under the CLI home — which is a shape to expect rather than invent, since
// `~/.bob/plugins/posse/custom_modes.yaml` is a path a bob user may own.
//
// WHAT THE ORDER BUYS, measured both ways: at HEAD the refusal names the
// GLOBAL root and the two remedies that fix the real problem (`--dir`,
// `default_dir`); swapped, it is the clobber check's "move it aside" —
// which personamode.go itself calls out as "the one remedy that would LET
// the write happen". Moving the file aside does not in fact let it happen,
// because the guard still refuses on the next line; the cost is a seat sent
// after the wrong thing, which is the cost a refusal exists not to impose.
func TestQAPersonaModeUnderTheGlobalRootRefusesAsAGlobalWriteNotAsAClobber(t *testing.T) {
	// No t.Parallel: t.Setenv.
	a := checkApp(t)
	rt := bobRuntime(t, a)
	ag := loadTestAgent(t, personaModePID)

	home := t.TempDir()
	t.Setenv("HOME", home)
	path := rt.PersonaModeFile(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	// THE FIXTURE BOTH ARMS WANT: a file at posse's path that posse did not
	// write, under the CLI's global root. The clobber check refuses it (no
	// marker) and so does the guard (it is under `~/.bob`) — so whichever
	// runs first is the one that answers, and the answer is the assertion.
	foreign := []byte("customModes:\n  - slug: the-operators-own\n")
	if err := os.WriteFile(path, foreign, 0o644); err != nil {
		t.Fatal(err)
	}
	// Both halves of "both arms want to fire" are checked, not assumed —
	// without this the test could pass over a fixture only one arm reads.
	if strings.HasPrefix(string(foreign), personaModeMarker) {
		t.Fatal("the fixture carries posse's marker, so the clobber check would wave it through and this test measures nothing")
	}
	if err := refusePersonaModeGlobalWrite(ag, rt, home, home); err == nil {
		t.Fatal("the guard does not refuse this fixture either, so there are not two arms to order")
	}
	if err := a.refusePersonaModeClobber(ag, rt, home, path); err == nil {
		t.Fatal("the clobber check does not refuse this fixture, so the order is unobservable here and this test measures nothing")
	}

	_, err := a.RenderPersonaModeFor(ag, rt, home)
	if err == nil {
		t.Fatal("a foreign file under the CLI's global root was overwritten")
	}
	// The GLOBAL refusal, by the two things only it says: where the path is
	// and what to do about it.
	for _, want := range []string{"GLOBAL", "--dir", "default_dir"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not mention %q — either the clobber check answered first (the order), or the guard stopped saying it (the message). Either way posse sent the operator after %q under a root it will refuse again on the next line, instead of naming the session directory that is the actual problem (ranger-base-ie68e F2):\n%v",
				want, "move it aside", err)
		}
	}
	// And NOT the clobber check's remedy, which is the wrong one here. Said
	// as its own assertion so a message that somehow carried both still
	// reds rather than passing on the four substrings above.
	if strings.Contains(err.Error(), "move it aside") {
		t.Errorf("the refusal offers \"move it aside\" for a path under the CLI's global root — that is the remedy for a workspace collision, and following it here just reaches the guard one line later:\n%v", err)
	}
	// The write still did not happen, which is the property the order must
	// not cost: the operator's bytes are untouched.
	if b, rerr := os.ReadFile(path); rerr != nil || string(b) != string(foreign) {
		t.Errorf("the operator's file at %s was modified (%v): %q", AbbrevHome(path), rerr, string(b))
	}
}
