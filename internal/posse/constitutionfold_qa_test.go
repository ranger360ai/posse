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
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
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

// constitutionResolveModel is the rune-level model of a case- AND
// normalization-insensitive volume: the ASCII byte it resolves r to, if any.
// Normalization first (APFS compares NFC-normalized names), then case, which
// is the order that makes U+212A reach "k" by either road.
//
// This is the derivation constitutionResolveRunes is the measured answer to,
// and the two agree on this box: MEASURED 2026-11-01 (ranger-base-qu8wn,
// Darwin 25.6, case-insensitive APFS) by opening one file per ASCII graphic
// character under every rune 0x80..0x10FFFF — four hits, the same four this
// model derives, and U+0130 in neither.
func constitutionResolveModel(r rune) (byte, bool) {
	rs := []rune(norm.NFC.String(string(r)))
	if len(rs) != 1 {
		return 0, false
	}
	c := rs[0]
	if c < 0x80 {
		return byte(unicode.ToLower(c)), true
	}
	for f := unicode.SimpleFold(c); f != c; f = unicode.SimpleFold(f) {
		if f < 0x80 {
			return byte(unicode.ToLower(f)), true
		}
	}
	return 0, false
}

// TestQAConstitutionResolveRunesAreTheClosedSet pins both arms' fold. It
// re-derives constitutionResolveModel over EVERY rune and asserts the
// non-ASCII runes that resolve to an ASCII byte are exactly
// constitutionResolveRunes. When a Go table bump adds one — Unicode's CASE
// fold or its NORMALIZATION tables, and this is what the predecessor pin
// could not see (ranger-base-qu8wn: it enumerated unicode.SimpleFold alone
// and so was blind to the two runes that reach ASCII through NFC, which both
// arms then left unfolded) — this reds. The fix is a row in
// constitutionResolveRunes, which renders into the sh fold's sed line
// (gates.go, posse_cls_fold) and is read by the Go belt's constitutionFold.
func TestQAConstitutionResolveRunesAreTheClosedSet(t *testing.T) {
	t.Parallel()
	got := map[rune]byte{}
	for r := rune(0x80); r <= unicode.MaxRune; r++ {
		if !utf8.ValidRune(r) {
			continue
		}
		if a, ok := constitutionResolveModel(r); ok {
			got[r] = a
		}
	}
	want := map[rune]byte{}
	for _, f := range constitutionResolveRunes {
		want[f.R] = f.ASCII
		if f.ASCII >= 0x80 {
			t.Errorf("U+%04X resolves to a non-ASCII byte %#x — the table's whole point is an ASCII target", f.R, f.ASCII)
		}
		// "&" is sed's one replacement metacharacter; the hook renders the
		// replacement through printf octal, so a literal & would be the one
		// byte that still misbehaved. No row has one today.
		if f.ASCII == '&' {
			t.Errorf("U+%04X resolves to %q, which is sed's replacement metacharacter — constitutionFoldSed needs an escape before this row lands", f.R, f.ASCII)
		}
	}
	if len(got) != len(want) {
		t.Errorf("non-ASCII runes resolving to ASCII: got %d %v, constitutionResolveRunes has %d — add the new row so BOTH arms fold it (gates.go posse_cls_fold and promote.go constitutionFold)", len(got), got, len(want))
	}
	for r, a := range got {
		if want[r] != a {
			t.Errorf("U+%04X resolves to %q and is not in constitutionResolveRunes as such — neither the sh sed line nor constitutionFold folds it", r, a)
		}
	}
	render := CommitGuardHook(VisibilityPublic, OpsPatternSet{})
	if !strings.Contains(render, "LC_ALL=C sed"+constitutionFoldSed()+" | LC_ALL=C tr 'A-Z' 'a-z'") {
		t.Errorf("the rendered hook does not carry the fold sed line built from constitutionResolveRunes")
	}
}

// TestQAConstitutionResolveIsSingleRuneSufficient closes the question the
// table's SHAPE begs: it maps one rune to one ASCII byte, so a SEQUENCE
// equivalent to a single ASCII character would be past both arms with no row
// able to express it. None exists, and the reason is checkable — no ASCII
// character has a canonical decomposition, so its NFD is itself and nothing
// longer can be equivalent to it. If a future Unicode gives an ASCII
// character a decomposition, this reds and the table's shape is wrong.
func TestQAConstitutionResolveIsSingleRuneSufficient(t *testing.T) {
	t.Parallel()
	for c := rune(0); c < 0x80; c++ {
		if d := norm.NFD.String(string(c)); d != string(c) {
			t.Errorf("ASCII %q has canonical decomposition %q — a MULTI-rune spelling now resolves to it, which a rune→byte table cannot fold", string(c), d)
		}
	}
}

// TestQAConstitutionShFoldResolvesEveryTableRune runs the rendered sh fold.
// The table pin above says which runes must be folded and the render pin says
// the hook carries the line; neither executes it, and the fold's hazard is
// the SHELL rather than sed — these are s/// expressions inside double
// quotes, and U+1FEF's ASCII byte is a backtick (ranger-base-qu8wn).
//
// Its loudest shape is covered elsewhere: rendered as a literal, the
// backtick opens a command substitution that swallows the rest of the hook,
// and TestQAInstalledCommitHookParses already reds on it (MEASURED — `sh -n`
// exit 2). What this adds is the quiet shape, a line that parses and folds
// the wrong thing or nothing. No real class member carries a ";" or a
// backtick, so no fixture on this page reaches those two rows through a real
// commit; running the line is how they are measured at all.
func TestQAConstitutionShFoldResolvesEveryTableRune(t *testing.T) {
	t.Parallel()
	line := "LC_ALL=C sed" + constitutionFoldSed() + " | LC_ALL=C tr 'A-Z' 'a-z'"
	if !strings.Contains(CommitGuardHook(VisibilityPublic, OpsPatternSet{}), line) {
		t.Fatalf("the rendered hook does not carry this fold line; the run below would not be measuring the shipped arm:\n%s", line)
	}
	var probe, want strings.Builder
	for _, f := range constitutionResolveRunes {
		probe.WriteString(".claude/" + string(f.R) + "ettings\n")
		want.WriteString(".claude/" + string(f.ASCII) + "ettings\n")
	}
	// Upper case through the same pipe: the sed runs before the tr, so a row
	// whose ASCII byte is a letter has to survive being lower-cased after.
	probe.WriteString(".CLAUDE/SETTINGS.JSON\n")
	want.WriteString(".claude/settings.json\n")
	script := "posse_cls_fold() {\n  " + line + "\n}\nprintf '%s' \"$1\" | posse_cls_fold\n"
	cmd := exec.Command("sh", "-c", script, "posse_cls_fold_probe", probe.String())
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("running the rendered fold: %v\nscript:\n%s", err, script)
	}
	if string(out) != want.String() {
		t.Errorf("the rendered sh fold does not resolve its own table:\n got %q\nwant %q\nscript:\n%s", out, want.String(), script)
	}
}

// TestQAConstitutionClassMembersAreTheirOwnFold holds the two facts the fold
// rests on. ASCII: so only constitutionResolveRunes can reach a member's
// byte, which is what makes that set sufficient for both arms. Lower case:
// the sh arm loops over the FOLDED class and prints that member in its
// refusal, which is only the true spelling when the member is its own fold.
//
// Sufficiency is now the TABLE's property and no longer the class's
// (ranger-base-qu8wn). While the table held two runes and missed ";" and the
// backtick, it was true only because no member happened to carry either —
// an accident of the live 22 that nothing pinned, and that the first member
// with a ";" in its name would have ended silently.
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
		// The character before "/hooks" is U+212A KELVIN SIGN, not an ASCII
		// "K": this file spells its runes literally (the row above it is
		// U+017F), and this is the one that cannot be told from its ASCII
		// relative by eye. Read the bytes, not the glyph.
		{".groK/hooks", ".grok/hooks", false},
		// U+0130 is the negative control: a rune this volume was MEASURED
		// not to resolve to its ASCII relative, so the fold is not simply
		// "any non-ASCII rune matches" (ranger-base-qu8wn).
		{".claude/İettings.json", "", false},
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

// TestQAConstitutionClassMatchFoldsNormalizationRunes is the row
// ranger-base-qu8wn asked the fix to carry: a class whose member's name
// contains the two ASCII bytes that only a NORMALIZATION rune reaches — ";"
// (U+037E) and "`" (U+1FEF) — matched through those spellings.
//
// The class is SYNTHETIC on purpose, and that is the finding restated: every
// one of the live 22 members is ASCII lower case with neither byte in it, so
// nothing a real member can be named exercises this. While the table held
// two runes the wall was shut by that accident alone, and the first member
// named with a ";" in it would have reopened it in silence.
//
// The Go belt is measured here, through the matcher and through
// ConstitutionTouched's rendering. The sh arm's own fold of the same four
// runes is measured by TestQAConstitutionShFoldResolvesEveryTableRune, which
// runs the rendered line — its class is rendered from the real members, so a
// synthetic one cannot reach it.
//
// Every row spells its runes LITERALLY, as the rest of this file does, so
// the paths below are indistinguishable from the member by eye. The row
// NAME is what says which rune it carries; read the bytes.
func TestQAConstitutionClassMatchFoldsNormalizationRunes(t *testing.T) {
	t.Parallel()
	const member = "we;rd/ho`ks"
	class := []string{member}
	for _, c := range []struct {
		name   string
		path   string
		member string
		above  bool
	}{
		{"as spelled", member, member, false},
		{"U+037E for the semicolon", "we;rd/ho`ks", member, false},
		{"U+1FEF for the backtick", "we;rd/ho`ks", member, false},
		{"both, and under the member", "we;rd/ho`ks/x", member, false},
		{"both, with ASCII case too", "WE;RD/HO`KS", member, false},
		{"both, above the member", "we;rd", member, true},
		// Controls: a rune that does NOT resolve, and the plain near miss.
		{"U+0130 does not resolve", "we;rd/ho`kİ", "", false},
		{"U+0387 does not resolve to the semicolon", "we·rd/ho`ks", "", false},
		{"near miss", "wexrd/ho`ks", "", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m, above := constitutionClassMatch(class, c.path)
			if m != c.member || above != c.above {
				t.Errorf("constitutionClassMatch(%q) = (%q, %v), want (%q, %v)", c.path, m, above, c.member, c.above)
			}
			hit := ConstitutionTouched(class, []string{c.path})
			if (len(hit) != 0) != (c.member != "") {
				t.Fatalf("the land belt's answer for %q is %v, want a hit=%v", c.path, hit, c.member != "")
			}
			if len(hit) == 0 {
				return
			}
			// The refusal prints the STAGED spelling, never the fold.
			if !strings.HasPrefix(hit[0], c.path+" (") {
				t.Errorf("the belt must print the staged spelling, got %q", hit[0])
			}
			if c.above != strings.Contains(hit[0], "a path above class member") {
				t.Errorf("belt rendering %q: above=%v", hit[0], c.above)
			}
		})
	}
}
