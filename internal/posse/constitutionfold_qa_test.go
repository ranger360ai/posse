//go:build posse_arm3

package posse

// The commit wall's class matched by RESOLUTION-equivalence rather than by
// name (ranger-base-g55ah, fixing ranger-base-y52v6). Both readers — the
// rendered sh arm and the land belt (ConstitutionTouched) — used to match a
// staged name exactly or as a prefix, and the security review MEASURED four shapes landing
// past both on APFS case-insensitive (security review, 2026-10-31, Darwin 25.6): a symlink at
// `.claude`, `.Claude/settings.json`, and `.claude/ſettings.json` (U+017F),
// each readable at the canonical path afterwards. The rows below are those
// shapes plus grok's and the KELVIN SIGN, each through a real persona commit
// AND through the land belt, with negative controls that must still land.

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode"
)

// resolutionRow is one staged shape: stage it, then say what spelling git
// staged (and the refusal must print).
type resolutionRow struct {
	name   string
	staged string
	above  bool
	stage  func(t *testing.T, repo string, git func(env []string, args ...string) (string, error), env []string)
}

func symlinkRow(name, at, target string) resolutionRow {
	return resolutionRow{name: name, staged: at, above: true, stage: func(t *testing.T, repo string, git func(env []string, args ...string) (string, error), env []string) {
		t.Helper()
		// The target holds what the runtime would read — never staged, so
		// the ONLY staged path is the link itself.
		if err := os.MkdirAll(filepath.Join(repo, target, "hooks"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, target, "settings.json"), []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(repo, at)); err != nil {
			t.Fatal(err)
		}
		if out, err := git(env, "add", "--", at); err != nil {
			t.Fatalf("git add %s: %v %s", at, err, out)
		}
	}}
}

func fileRow(name, rel string) resolutionRow {
	return resolutionRow{name: name, staged: rel, stage: func(t *testing.T, repo string, git func(env []string, args ...string) (string, error), env []string) {
		t.Helper()
		stageAt(t, repo, git, env, rel, "{\"hooks\":{}}\n")
	}}
}

var resolutionRows = []resolutionRow{
	symlinkRow("symlink at .claude", ".claude", "vendorcfg"),
	symlinkRow("symlink at .grok (member .grok/hooks is a dir)", ".grok", "vendorgrok"),
	fileRow("upper-case directory", ".Claude/settings.json"),
	fileRow("upper-case file", ".claude/Settings.json"),
	fileRow("long s U+017F", ".claude/ſettings.json"),
	fileRow("KELVIN SIGN U+212A", ".groK/hooks/x"),
}

// TestQAConstitutionWallRefusesResolutionEquivalentNames is the fix measured
// through both readers. The sh arm through a real path-limited persona
// commit; the land belt through ConstitutionTouched over the very spelling
// git staged, since the belt is the one that runs before main moves.
func TestQAConstitutionWallRefusesResolutionEquivalentNames(t *testing.T) {
	t.Parallel()
	for _, row := range resolutionRows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			repo, git, persona := constitutionWallRepo(t, false)
			row.stage(t, repo, git, persona)
			staged, err := git(nil, "diff", "--cached", "--name-only", "-z", "HEAD")
			if err != nil {
				t.Fatalf("git diff --cached: %v %s", err, staged)
			}
			if got := strings.TrimRight(staged, "\x00"); got != row.staged {
				t.Fatalf("fixture staged %q, want exactly %q — the row is not measuring its shape", got, row.staged)
			}
			out, err := git(persona, "commit", "-m", "land a channel past the name check", "--", row.staged)
			// The refusal prints the ORIGINAL staged spelling, not the fold.
			assertConstitutionRefusal(t, out, err, row.staged, gatesDirOf(persona))
			if row.above && !strings.Contains(out, "a path above class member") {
				t.Errorf("a refusal of %q must say it is a path above a class member, got:\n%s", row.staged, out)
			}
			hit := ConstitutionTouched(ConstitutionClassIn(repo), []string{row.staged})
			if len(hit) == 0 {
				t.Fatalf("the land belt lets %q through — the hook refused it, and the belt behind it must too", row.staged)
			}
			if !strings.HasPrefix(hit[0], row.staged+" (") {
				t.Errorf("the belt must print the staged spelling, got %q", hit[0])
			}
			if row.above != strings.Contains(hit[0], "a path above class member") {
				t.Errorf("belt rendering %q: above=%v", hit[0], row.above)
			}
		})
	}
}

// TestQAConstitutionWallLandsTheNearMisses is the CONTROL that makes the rows
// above mean anything: a wall that refuses every path starting ".claude" or
// every symlink would pass them all. These must land through both readers.
func TestQAConstitutionWallLandsTheNearMisses(t *testing.T) {
	t.Parallel()
	for _, rel := range []string{"claude-notes.md", ".claudex/settings.json", "vendorcfg/settings.json", ".grokk/hooks/x"} {
		t.Run(rel, func(t *testing.T) {
			t.Parallel()
			repo, git, persona := constitutionWallRepo(t, false)
			stageAt(t, repo, git, persona, rel, "{}\n")
			if out, err := git(persona, "commit", "-m", "an ordinary persona commit", "--", rel); err != nil {
				t.Fatalf("%s is not in the class and must land: %v\n%s", rel, err, out)
			}
			if hit := ConstitutionTouched(ConstitutionClassIn(repo), []string{rel}); len(hit) != 0 {
				t.Errorf("the land belt refuses %s, which is not in the class: %v", rel, hit)
			}
		})
	}
}

// TestQAConstitutionWallTrackedFoldFailsClosed is the security review's git-ignorecase
// control: over a TRACKED .claude/settings.json, a fold spelling must not
// carry a change onto the branch. On a case-insensitive fs git stages nothing
// ("pathspec did not match", MEASURED); on a case-sensitive fs the spelling is
// a distinct new file, and then the wall must refuse it. Either is closed.
func TestQAConstitutionWallTrackedFoldFailsClosed(t *testing.T) {
	t.Parallel()
	repo, git, persona := constitutionWallRepo(t, false)
	stageAt(t, repo, git, nil, ClaudeProjectConfig, "{}\n")
	if out, err := git(nil, "commit", "-qm", "operator settings", "--", ClaudeProjectConfig); err != nil {
		t.Fatalf("operator commit: %v %s", err, out)
	}
	rel := ".claude/ſettings.json"
	if err := os.WriteFile(filepath.Join(repo, filepath.FromSlash(rel)), []byte("{\"hooks\":{}}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(persona, "add", "--", rel) // may refuse; that is the closed answer
	staged, _ := git(nil, "diff", "--cached", "--name-only", "HEAD")
	if strings.TrimSpace(staged) == "" {
		return
	}
	out, err := git(persona, "commit", "-m", "edit settings through a fold", "--", strings.TrimSpace(staged))
	if err == nil {
		t.Fatalf("a fold spelling over a tracked class member landed (staged %q):\n%s", staged, out)
	}
}

// TestQAConstitutionFoldRunesAreTheClosedSet pins the sh arm's fold. It
// enumerates unicode.SimpleFold over EVERY rune and asserts the non-ASCII
// runes whose fold orbit reaches ASCII are exactly constitutionFoldRunes.
// When a Go Unicode table bump adds one, this reds: the fix is a row in
// constitutionFoldRunes, which renders into the sh fold's sed line
// (gates.go, posse_cls_fold) — without it the sh arm would quietly reopen
// while the Go belt, which uses strings.EqualFold, stayed shut.
func TestQAConstitutionFoldRunesAreTheClosedSet(t *testing.T) {
	t.Parallel()
	got := map[rune]byte{}
	for r := rune(0x80); r <= unicode.MaxRune; r++ {
		for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
			if f < 0x80 {
				got[r] = byte(unicode.ToLower(f))
				break
			}
		}
	}
	want := map[rune]byte{}
	for _, f := range constitutionFoldRunes {
		want[f.R] = f.ASCII
	}
	if len(got) != len(want) {
		t.Errorf("non-ASCII runes folding to ASCII: got %d %v, constitutionFoldRunes has %d — add the new row so the sh sed line in gates.go posse_cls_fold folds it", len(got), got, len(want))
	}
	for r, a := range got {
		if want[r] != a {
			t.Errorf("U+%04X folds to %q and is not in constitutionFoldRunes as such — the sh sed line in gates.go posse_cls_fold does not fold it", r, a)
		}
	}
	render := CommitGuardHook(VisibilityPublic, OpsPatternSet{})
	if !strings.Contains(render, "LC_ALL=C sed"+constitutionFoldSed()+" | LC_ALL=C tr 'A-Z' 'a-z'") {
		t.Errorf("the rendered hook does not carry the fold sed line built from constitutionFoldRunes")
	}
}

// TestQAConstitutionClassMembersAreTheirOwnFold holds the two facts the fold
// rests on. ASCII: so only constitutionFoldRunes can EqualFold a member's
// byte, which is what makes that set sufficient for the sh arm. Lower case:
// the sh arm loops over the FOLDED class and prints that member in its
// refusal, which is only the true spelling when the member is its own fold.
func TestQAConstitutionClassMembersAreTheirOwnFold(t *testing.T) {
	t.Parallel()
	members := ConstitutionClassIn(constitutionRepoTop(t))
	for i := range builtinRuntimes {
		members = append(members, builtinRuntimes[i].ProjectConfig...)
	}
	sort.Strings(members)
	for _, m := range members {
		for _, r := range m {
			if r >= 0x80 {
				t.Errorf("class member %q is not ASCII — the sh fold only covers runes reaching ASCII", m)
				break
			}
		}
		if strings.ToLower(m) != m {
			t.Errorf("class member %q is not its own fold — the sh refusal would print it lower-cased", m)
		}
	}
}

// TestQAConstitutionClassMatchIsResolutionEquivalent is the Go matcher as a
// table, both rules and both directions, so a byte-length prefix or a
// ToLower regression reds here without a git fixture.
func TestQAConstitutionClassMatchIsResolutionEquivalent(t *testing.T) {
	t.Parallel()
	class := []string{ClaudeProjectConfig, ".grok/hooks"}
	for _, c := range []struct {
		path   string
		member string
		above  bool
	}{
		{".claude", ClaudeProjectConfig, true},
		{".CLAUDE", ClaudeProjectConfig, true},
		{".grok", ".grok/hooks", true},
		{".groK", ".grok/hooks", true},
		{".claude/settings.json", ClaudeProjectConfig, false},
		{".claude/ſettings.json", ClaudeProjectConfig, false},
		{".claude/SETTINGS.JSON/x", ClaudeProjectConfig, false},
		{".grok/HOOKS/a/b", ".grok/hooks", false},
		{".claude/other.json", "", false},
		{".claudex", "", false},
		{".clau", "", false},
		{"claude", "", false},
		{".claude/settings.jso", "", false},
		{".claude/settings.json.bak", "", false},
		{"", "", false},
	} {
		m, above := constitutionClassMatch(class, c.path)
		if m != c.member || above != c.above {
			t.Errorf("constitutionClassMatch(%q) = (%q, %v), want (%q, %v)", c.path, m, above, c.member, c.above)
		}
	}
}
