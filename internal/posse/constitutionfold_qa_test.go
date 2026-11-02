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
	"runtime"
	"sort"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// resolutionRow is one staged shape: stage it, then say what spelling git
// staged (and the refusal must print).
type resolutionRow struct {
	name   string
	staged string
	above  bool
	// constitution makes the fixture the constitution repo (the marker
	// directory present), so the promoted half of the class is live.
	constitution bool
	stage        func(t *testing.T, repo string, git func(env []string, args ...string) (string, error), env []string)
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
	// ranger-base-l717c's four MEASURED landings: one rune that APFS resolves
	// to two ASCII letters, through the FULL case fold a rune->byte table
	// could not say (fixed by ranger-base-uvcj8). Spelled with escapes rather
	// than literally, unlike the rows above: a ligature or a sharp s beside
	// the letters it opens is the hardest spelling on this page to read.
	constitutionRow(fileRow("sharp s U+00DF in posse/config.yaml", "po\u00dfe/config.yaml")),
	constitutionRow(fileRow("capital sharp s U+1E9E in posse/agents", "po\u1e9ee/agents/developer.md")),
	fileRow("fi ligature U+FB01 in .codex/config.toml", ".codex/con\ufb01g.toml"),
	fileRow("long s t ligature U+FB05 in .bob/custom_modes.yaml", ".bob/cu\ufb05om_modes.yaml"),
}

func constitutionRow(r resolutionRow) resolutionRow {
	r.constitution = true
	return r
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
			repo, git, persona := constitutionWallRepo(t, row.constitution)
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
// normalization-insensitive volume: the ASCII STRING it resolves r to, if
// any. NFC, then FULL case fold, then NFC again — the fold can produce a
// sequence that composes, and the volume compares composed names.
//
// FULL fold and not unicode.SimpleFold (ranger-base-l717c): the simple fold
// maps one rune to one rune, so it can never say that U+00DF opens "ss" or
// U+FB03 opens "ffi", and the model it replaced was blind to nine runes APFS
// resolves — the same blindness the table had, so the pin agreed with it.
//
// This is the derivation constitutionResolveRunes is the measured answer to,
// and the two agree on this box: MEASURED 2026-11-01 (ranger-base-l717c,
// Darwin 25.6, case-insensitive APFS) by opening one fixture per graphic
// ASCII character and per a-z string of length 2 and 3 under every rune
// 0x80..0x10FFFF — 13 hits, the same 13 this model derives, and U+0130 in
// neither. fold is the caller's, one per goroutine: a cases.Caser is not
// safe for concurrent use.
func constitutionResolveModel(fold cases.Caser, r rune) (string, bool) {
	s := norm.NFC.String(fold.String(norm.NFC.String(string(r))))
	if s == "" {
		return "", false
	}
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return "", false
		}
	}
	return s, true
}

// constitutionResolveDerived is constitutionResolveModel over every
// non-ASCII rune, keyed by rune.
func constitutionResolveDerived() map[rune]string {
	fold := cases.Fold()
	got := map[rune]string{}
	for r := rune(0x80); r <= unicode.MaxRune; r++ {
		if !utf8.ValidRune(r) {
			continue
		}
		if a, ok := constitutionResolveModel(fold, r); ok {
			got[r] = a
		}
	}
	return got
}

// TestQAConstitutionResolveRunesAreTheClosedSet pins both arms' fold. It
// re-derives constitutionResolveModel over EVERY rune and asserts the
// non-ASCII runes that resolve to an ASCII string, and the strings, are
// exactly constitutionResolveRunes. When a Go table bump adds one — Unicode's CASE
// fold or its NORMALIZATION tables, and this is what the predecessor pin
// could not see (ranger-base-qu8wn: it enumerated unicode.SimpleFold alone
// and so was blind to the two runes that reach ASCII through NFC, which both
// arms then left unfolded) — this reds. The fix is a row in
// constitutionResolveRunes, which renders into the sh fold's sed line
// (gates.go, posse_cls_fold) and is read by the Go belt's constitutionFold.
func TestQAConstitutionResolveRunesAreTheClosedSet(t *testing.T) {
	t.Parallel()
	got := constitutionResolveDerived()
	want := map[rune]string{}
	for _, f := range constitutionResolveRunes {
		if _, dup := want[f.R]; dup {
			t.Errorf("U+%04X is in constitutionResolveRunes twice", f.R)
		}
		want[f.R] = f.ASCII
		if f.ASCII == "" {
			t.Errorf("U+%04X resolves to the empty string — a row must name what the volume opens", f.R)
		}
		for i := 0; i < len(f.ASCII); i++ {
			c := f.ASCII[i]
			if c >= 0x80 {
				t.Errorf("U+%04X resolves to %q, which is not ASCII — the table's whole point is an ASCII target", f.R, f.ASCII)
			}
			// What printf hands sed lands inside the s///, read as a
			// replacement: "&" is the matched text, a backslash escapes,
			// "/" is the delimiter and a newline ends the expression. The
			// WHOLE string is checked, not one byte (ranger-base-uvcj8: a
			// replacement is several bytes now). No row has one today.
			if strings.IndexByte("&\\/\n", c) >= 0 {
				t.Errorf("U+%04X resolves to %q, which carries %q — a sed replacement metacharacter or delimiter; constitutionFoldSed needs an escape before this row lands", f.R, f.ASCII, c)
			}
		}
	}
	if len(got) != len(want) {
		t.Errorf("non-ASCII runes resolving to ASCII: got %d %q, constitutionResolveRunes has %d — add the new row so BOTH arms fold it (gates.go posse_cls_fold and promote.go constitutionFold)", len(got), got, len(want))
	}
	for r, a := range got {
		if w, ok := want[r]; !ok || w != a {
			t.Errorf("U+%04X resolves to %q and is not in constitutionResolveRunes as such — neither the sh sed line nor constitutionFold folds it", r, a)
		}
	}
	for r, w := range want {
		if _, ok := got[r]; !ok {
			t.Errorf("constitutionResolveRunes has U+%04X -> %q, which the derivation does not produce", r, w)
		}
	}
	render := CommitGuardHook(VisibilityPublic, OpsPatternSet{})
	if !strings.Contains(render, "LC_ALL=C sed"+constitutionFoldSed()+" | LC_ALL=C tr 'A-Z' 'a-z'") {
		t.Errorf("the rendered hook does not carry the fold sed line built from constitutionResolveRunes")
	}
}

// TestQAConstitutionResolveIsSingleRuneSufficient closes the two questions
// the table's SHAPE begs, one per direction.
//
// A SEQUENCE resolving to one ASCII character would be past both arms with no
// row able to express it. None exists, and the reason is checkable — no
// ASCII character has a canonical decomposition, so its NFD is itself and
// nothing longer can be equivalent to it. If a future Unicode gives an ASCII
// character a decomposition, this reds and the table's shape is wrong.
//
// ONE rune resolving to SEVERAL ASCII characters is the other direction, and
// the one the rune->byte table missed (ranger-base-l717c: U+00DF opens "ss",
// U+FB01 "fi", U+FB03 "ffi"). The table carries a string now; this half
// holds that every multi-character resolution the derivation finds is a row
// with exactly that string, and that there ARE some — a model that could
// only answer one byte would pass the first check over an empty set.
func TestQAConstitutionResolveIsSingleRuneSufficient(t *testing.T) {
	t.Parallel()
	for c := rune(0); c < 0x80; c++ {
		if d := norm.NFD.String(string(c)); d != string(c) {
			t.Errorf("ASCII %q has canonical decomposition %q — a MULTI-rune spelling now resolves to it, which a rune-keyed table cannot fold", string(c), d)
		}
	}
	table := map[rune]string{}
	for _, f := range constitutionResolveRunes {
		table[f.R] = f.ASCII
	}
	multi := 0
	for r, a := range constitutionResolveDerived() {
		if len(a) < 2 {
			continue
		}
		multi++
		if table[r] != a {
			t.Errorf("U+%04X resolves to the %d-character %q, and constitutionResolveRunes has %q — both arms would compare it against the wrong name", r, len(a), a, table[r])
		}
	}
	if multi == 0 {
		t.Errorf("the derivation found no rune resolving to a multi-character ASCII string — ranger-base-l717c MEASURED nine; the model has gone blind to the full fold")
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
		want.WriteString(".claude/" + f.ASCII + "ettings\n")
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

// constitutionResolveWiderCandidates is the derivation constitutionResolveRunes
// is NOT: every non-ASCII rune that reaches an ASCII string through
// COMPATIBILITY equivalence (NFKC) and the full case fold, minus the rows the
// table already carries.
//
// WHY A SECOND DERIVATION, and why this one. The table's own derivation is
// APFS's equivalence as measured — canonical normalization plus the full case
// fold — so a pin built on it can only ever ask whether Go agrees with Go.
// The question the volume alone can answer is whether its equivalence is
// WIDER than that, and Unicode's next-wider standard relation is
// compatibility equivalence. That is where every lookalike spelling of an
// ASCII name lives: the fullwidth forms (U+FF53 for "s"), the circled and
// parenthesized letters, the superscripts, the roman numerals, the squared
// abbreviations whose compatibility form is several ASCII characters. A
// volume that folded any of them would open a class member under a spelling
// no row in the table can say, which is the shape of every bead in this
// chain.
func constitutionResolveWiderCandidates() map[rune]string {
	fold := cases.Fold()
	// The shipped table, never constitutionResolveDerived(): subtracting the
	// DERIVATION would put every rune the derivation produces outside this
	// set, so a row deleted from constitutionResolveRunes would leave the
	// boundary half green over a spelling this volume really does open.
	// MUTATION-CHECKED both ways (ranger-base-3pm6b): dropping the U+212A row
	// survives the derivation-keyed version of this line and reds the
	// table-keyed one, naming the KELVIN SIGN as a live escape.
	table := map[rune]bool{}
	for _, f := range constitutionResolveRunes {
		table[f.R] = true
	}
	got := map[rune]string{}
	for r := rune(0x80); r <= unicode.MaxRune; r++ {
		if !utf8.ValidRune(r) {
			continue
		}
		if table[r] {
			continue
		}
		s := norm.NFC.String(fold.String(norm.NFKC.String(string(r))))
		if s == "" {
			continue
		}
		ascii := true
		for i := 0; i < len(s); i++ {
			if s[i] >= 0x80 {
				ascii = false
				break
			}
		}
		if ascii {
			got[r] = s
		}
	}
	return got
}

// constitutionFoldVolume returns a directory to probe, or skips. Two
// conditions, each naming the volume it did not get rather than passing
// vacuously (ranger-base-99gww's rule for a pin whose subject is the MOUNT):
// the premise is APFS's, and on a case-SENSITIVE volume no name resolves to
// another, so there is nothing there for the fold to be a model of.
func constitutionFoldVolume(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skipf("the premise under test is APFS's name equivalence and this is %s", runtime.GOOS)
	}
	dir := t.TempDir()
	at := filepath.Join(dir, "constitutionvolumeprobe")
	if err := os.WriteFile(at, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	lower, err := os.Stat(at)
	if err != nil {
		t.Fatal(err)
	}
	upper, err := os.Stat(filepath.Join(dir, "CONSTITUTIONVOLUMEPROBE"))
	if err != nil || !os.SameFile(lower, upper) {
		t.Skipf("%s is on a case-SENSITIVE volume: no spelling resolves to another one here, so the fold models nothing measurable", dir)
	}
	return dir
}

// constitutionFoldFixture makes a file this volume holds under the ASCII name
// ascii, and returns what names it. The name is the MIDDLE of "x<ascii>y":
// several compatibility targets are not legal names on their own — "." and
// "/" both occur among them — and a wrapper gives every target one spelling
// that is.
func constitutionFoldFixture(dir, ascii string) (os.FileInfo, error) {
	at := filepath.Join(dir, filepath.FromSlash("x"+ascii+"y"))
	if err := os.MkdirAll(filepath.Dir(at), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(at, []byte("the fold's subject\n"), 0o644); err != nil {
		return nil, err
	}
	return os.Stat(at)
}

// TestQAConstitutionResolveTableIsThisVolumesEquivalence EXECUTES the premise
// every other arm of the fold rests on, against the subject rather than
// against a model of it.
//
// THE GAP (ranger-base-3pm6b, from ranger-base-2g820's verify of
// ranger-base-l717c). constitutionResolveRunes is DERIVED, and every pin over
// it re-derives it the same way — NFC, cases.Fold, NFC — so the sh sed
// render, the Go land belt, the closed-set pin and the sufficiency pin all
// agree with the table whenever the table agrees with GO, whatever the
// filesystem does. Three beads in this chain were one defect:
// ranger-base-y52v6, ranger-base-qu8wn and ranger-base-l717c each found a
// model of the volume's name equivalence narrower than the equivalence, each
// fix widened the model, and none of them executed the premise — so each
// reading found it again. The premise was carried as prose, correct and
// dated, and prose does not red.
//
// BOTH DIRECTIONS, and a failure means opposite things.
//
//   - A ROW THAT DOES NOT RESOLVE says the table is wider than this volume.
//     The wall stays shut — a fold wider than the equivalence refuses
//     spellings nobody could have used — but the table's claim about the
//     volume is then false, and the next bead reasoning from it inherits a
//     wrong measurement.
//   - A CANDIDATE THAT RESOLVES says the wall is OPEN: a spelling reaches a
//     class member's name and no row in the table folds it, which is exactly
//     the landing ranger-base-l717c measured.
//
// WHAT THIS DOES NOT COVER. The candidate set is still derived, from a
// strictly wider model, so this arm reds the day the volume's equivalence
// widens as far as COMPATIBILITY equivalence and not the day it widens past
// that. The total reading — every rune 0x80..0x10FFFF against one fixture per
// graphic ASCII character and per a-z string of length 2 and 3 — is
// ranger-base-l717c's, dated, and ADR 0015 section 3 carries it as the
// obligation an OS bump re-pays. What is no longer prose is everything inside
// compatibility equivalence, which is where a lookalike spelling of a class
// member would be found.
//
// MEASURED 2026-11-02 (ranger-base-3pm6b, Darwin 25.6, case-insensitive APFS,
// go1.26.10, golang.org/x/text v0.43.0): 1,392 candidates, ZERO of them
// resolving; all 13 rows resolving; U+0130 opening nothing. 0.41s for the
// test, both derivations over 1.1M runes included.
//
// MUTATION-CHECKED, and docs/notes.d/ranger-base-3pm6b.md has the four
// mutants. One of them SURVIVED the first draft and is why the candidate set
// subtracts the shipped table rather than the derivation.
func TestQAConstitutionResolveTableIsThisVolumesEquivalence(t *testing.T) {
	t.Parallel()
	dir := constitutionFoldVolume(t)
	cands := constitutionResolveWiderCandidates()
	// Every fixture BEFORE any probe, so each probe is asked against the
	// whole name set: a candidate resolving to some OTHER row's target is
	// still a spelling that opens an ASCII name, and must be caught.
	fixture := map[string]os.FileInfo{}
	unnameable := map[string]error{}
	for _, want := range cands {
		if _, done := fixture[want]; done {
			continue
		}
		fi, err := constitutionFoldFixture(dir, want)
		if err != nil {
			// A target this volume cannot hold is a target no class member
			// can be named through, so it is not a hazard — counted, not
			// fatal, and the floor below keeps the count honest.
			unnameable[want] = err
			continue
		}
		fixture[want] = fi
	}
	for _, f := range constitutionResolveRunes {
		if _, done := fixture[f.ASCII]; done {
			continue
		}
		fi, err := constitutionFoldFixture(dir, f.ASCII)
		if err != nil {
			t.Fatalf("this volume cannot hold a file named through %q, which constitutionResolveRunes says U+%04X opens: %v", f.ASCII, f.R, err)
		}
		fixture[f.ASCII] = fi
	}

	// FORWARD: the table says the volume resolves these 13. Identity, never
	// existence — the table names WHICH string, so the question is whether
	// the two names are one file.
	for _, f := range constitutionResolveRunes {
		probe, err := os.Stat(filepath.Join(dir, "x"+string(f.R)+"y"))
		if err != nil {
			t.Errorf("constitutionResolveRunes says U+%04X opens %q, and on this volume it opens nothing (%v) — the table is WIDER than this volume's equivalence and its measurement is stale, though the wall stays shut", f.R, f.ASCII, err)
			continue
		}
		if !os.SameFile(fixture[f.ASCII], probe) {
			t.Errorf("U+%04X opens a file on this volume that is not the one named through %q — the table names the wrong target", f.R, f.ASCII)
		}
	}

	// BOUNDARY: nothing outside the table may open an ASCII name.
	probed := 0
	for r, want := range cands {
		if _, ok := fixture[want]; !ok {
			continue
		}
		probed++
		if _, err := os.Stat(filepath.Join(dir, "x"+string(r)+"y")); err == nil {
			t.Errorf("U+%04X opens an ASCII-named file on this volume (its compatibility form is %q) and constitutionResolveRunes has no row for it — both arms of the wall leave this spelling unfolded, so it reaches a class member under a name neither reader compares", r, want)
		}
	}
	if probed < 1000 {
		t.Errorf("only %d of %d compatibility candidates were probed (1,392 probed on 2026-11-02) — the boundary half has gone vacuous; unnameable targets: %v", probed, len(cands), unnameable)
	}

	// The rig can say NO over a rune that is not a candidate at all: U+0130
	// folds to "i" plus a combining dot and is the negative control every
	// measurement in this chain has carried.
	if _, err := constitutionFoldFixture(dir, "i"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "xİy")); err == nil {
		t.Errorf("U+0130 opens an ASCII-named file on this volume — the negative control every bead in this chain measured has flipped, and the whole derivation is now a guess")
	}
}
