package main

// ranger-base-rjfec: `posse help`'s preamble stated a rule — a subcommand
// prints its own usage for -h/--help — that only the verbs routed through
// need()/argLead kept. The filing bead probed eight and found six that did
// not keep it; the whole surface, MEASURED 2026-10-09 off this switch, was
// 17 of the 42 verb spellings outside help/version. `posse cage --help` and
// `posse recipes --help` did their real work and reported, `posse scorecard
// --help` went to bd for every persona, `posse list --help` reached for
// herdr, and the rest printed a usage and exited 1. Asking for help was an
// error, a command, or a bd scan, depending on the verb.
//
// The rule has one realization now (catalogHelp, read ahead of the switch)
// and the usage catalog is its source, so there is no second usage string to
// go stale. What that buys has to be CENSUSED rather than sampled, because
// the gap was never in the mechanism — it was in which verbs reached it.
// This file enumerates main's switch from the AST, so a verb added tomorrow
// without a catalog entry reds here instead of taking '--help' as its name.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// verbHelpExempt are the two verbs that answer with themselves: `posse help`
// IS the catalog and `posse version` is one line. Neither can print "its own
// entry" because neither has one, and neither does work an operator asking
// for help would not want done.
var verbHelpExempt = map[string]bool{
	"help": true, "-h": true, "--help": true,
	"version": true, "--version": true,
}

// mainSwitchVerbs is every spelling main's `switch cmd` accepts, read from
// the source rather than from a list in this file: a hand-kept list cannot
// red when a verb is added, which is the whole failure this pins.
func mainSwitchVerbs(t *testing.T) []string {
	t.Helper()
	sw, _ := mainVerbSwitch(t)
	var verbs []string
	for _, st := range sw.Body.List {
		cc, ok := st.(*ast.CaseClause)
		if !ok {
			continue
		}
		for _, e := range cc.List {
			lit, ok := e.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				t.Fatalf("a case in main's verb switch is not a string literal: %T", e)
			}
			verbs = append(verbs, strings.Trim(lit.Value, `"`))
		}
	}
	if len(verbs) < 30 {
		t.Fatalf("main's verb switch reads as %d verbs — this census is reading the wrong switch", len(verbs))
	}
	return verbs
}

// mainVerbSwitch returns main's `switch cmd` and its index in main's body,
// so a pin can say that the help reading comes BEFORE it.
func mainVerbSwitch(t *testing.T) (*ast.SwitchStmt, int) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "main" || fn.Recv != nil {
			continue
		}
		for i, st := range fn.Body.List {
			sw, ok := st.(*ast.SwitchStmt)
			if !ok {
				continue
			}
			if id, ok := sw.Tag.(*ast.Ident); ok && id.Name == "cmd" {
				return sw, i
			}
		}
		t.Fatal("main has no `switch cmd` — the census has nothing to read")
	}
	t.Fatal("main.go has no func main")
	return nil, 0
}

func TestEveryVerbAnswersForHelp(t *testing.T) {
	catalog := usageCatalog()
	for _, v := range mainSwitchVerbs(t) {
		if verbHelpExempt[v] {
			continue
		}
		for _, flag := range []string{"--help", "-h"} {
			block, ok := catalogHelp([]string{v, flag})
			if !ok || block == "" {
				t.Errorf("`posse %s %s` is not answered from the catalog — it falls through to the verb, which is where a bd scan, a herdr call or exit 1 came from", v, flag)
				continue
			}
			// One copy: what the verb prints is a verbatim slice of what
			// `posse help` prints, so the two cannot disagree.
			if !strings.Contains(catalog, block) {
				t.Errorf("`posse %s %s` printed text that is not a slice of the catalog:\n%s", v, flag, block)
			}
			// And it is THAT verb's entry, not a neighbour's.
			head := strings.SplitN(block, "\n", 2)[0]
			want := v
			if to, ok := catalogAlias[v]; ok {
				want = to
			}
			if lead := catalogLead(head); len(lead) == 0 || lead[0] != want {
				t.Errorf("`posse %s %s` answered with %q, whose entry leads with %q", v, flag, head, lead)
			}
		}
	}
}

// The separator and free text, which is why catalogHelp reads the argument
// AFTER the subcommand path rather than scanning argv for -h/--help.
func TestCatalogHelpReadsTheSeparatorAndFreeTextAsArgLeadDoes(t *testing.T) {
	for _, c := range []struct {
		why  string
		argv []string
	}{
		{"a literal -- ends the reading (rangerhq-qv5's other half)", []string{"kill", "--", "--help"}},
		{"a session really called --help", []string{"attach", "--", "--help"}},
		{"prompt text is text", []string{"prompt", "sess", "--help"}},
		{"a pause why is text", []string{"pause", "the box is on fire --help"}},
		{"no verb at all", []string{}},
		{"a verb and nothing else", []string{"list"}},
		{"an unknown verb still gets the unknown-command line", []string{"nosuchverb", "--help"}},
	} {
		if _, ok := catalogHelp(c.argv); ok {
			t.Errorf("catalogHelp(%q) read a help request — %s", c.argv, c.why)
		}
	}
}

// catalogSubVerbs is every sub-verb path the catalog names, read from the
// catalog itself: the preamble's claim is scoped to the sub-verbs the
// catalog names, so the catalog is the enumeration that claim must be
// measured against. A hand list here would let an entry added tomorrow go
// unanswered while this file stayed green.
func catalogSubVerbs(t *testing.T) [][]string {
	t.Helper()
	seen := map[string]bool{}
	var paths [][]string
	for _, ln := range strings.Split(usageCatalog(), "\n") {
		lead := catalogLead(ln)
		if len(lead) < 2 || seen[strings.Join(lead, " ")] {
			continue
		}
		seen[strings.Join(lead, " ")] = true
		paths = append(paths, lead)
	}
	// MEASURED 2026-10-09: fourteen (agent new/edit/check, backup
	// status/verify, beads check, cage build/down, gates
	// install-hooks/managed-hooks/adr-census, runtime check/probe/filing).
	// The floor is what stops a catalog that lost its sub-verb headers from
	// making this census vacuous.
	if len(paths) < 14 {
		t.Fatalf("the catalog names %d sub-verb paths, want at least 14 — this census is reading the wrong thing", len(paths))
	}
	return paths
}

// Sub-verbs, the half need() never reached at all: `posse agent new --help`
// scaffolded a persona named '--help', and `posse backup status --help`
// exited 1 (ranger-base-cse63).
func TestSubVerbsAnswerForThemselves(t *testing.T) {
	for _, path := range catalogSubVerbs(t) {
		argv := append(append([]string{}, path...), "--help")
		block, ok := catalogHelp(argv)
		if !ok || block == "" {
			t.Errorf("`posse %s --help` is not answered from the catalog", strings.Join(path, " "))
			continue
		}
		// Its own entry and no sibling's: the parent verb answers with
		// every entry under it, a sub-verb with one.
		if n := strings.Count(block, "\n  posse "); n != 0 {
			t.Errorf("`posse %s --help` answered with %d entries, want its own:\n%s", strings.Join(path, " "), n+1, block)
		}
		if lead := catalogLead(strings.SplitN(block, "\n", 2)[0]); !leadHas(lead, path) {
			t.Errorf("`posse %s --help` answered with the entry for %q", strings.Join(path, " "), lead)
		}
	}
}

// A parent verb answers with every entry under it — the reading an operator
// typing `posse cage --help` wants, and the one that makes a sub-verb's
// grammar discoverable without `posse help`.
func TestAParentVerbAnswersWithEveryEntryUnderIt(t *testing.T) {
	// MEASURED off the catalog. `gates` is 5 rather than the 4 this bead was
	// written against: ranger-base-1jmtm gave `posse gates wrap` an entry
	// while this one was in flight (ranger-base-lw0s5 merged the two). A
	// count is the point — an entry that stops being reachable under its
	// parent is what this pin exists to catch — so it is updated by hand
	// when the catalog gains one, never derived from the thing under test.
	for verb, want := range map[string]int{
		"cage": 3, "runtime": 3, "agent": 3, "gates": 5, "backup": 3,
		"scorecard": 2, "cost": 2, "refresh": 2, "list": 1, "init": 1,
	} {
		block, ok := catalogHelp([]string{verb, "--help"})
		if !ok {
			t.Errorf("`posse %s --help` is not answered at all", verb)
			continue
		}
		if got := strings.Count(block, "\n  posse ") + 1; got != want {
			t.Errorf("`posse %s --help` answered with %d catalog entries, want %d:\n%s", verb, got, want, block)
		}
	}
}

// A verb's config keys are not its grammar. They sit in the catalog BETWEEN
// command entries, so without this boundary a block would run on into keys
// that are no verb's at all — backup's are followed immediately by
// verify_box_max_age: and verify_box_accepted:, which belong to `posse
// status`'s governance row. So catalogBlock stops, and a verb whose keys ARE
// what an operator came for widens its own answer through verbOwnHelp:
// `posse backup --help` is backupHelp(), not this slice (ranger-base-cse63,
// merged with this bead under ranger-base-lw0s5). The subject here is
// catalogBlock itself, which every other verb's answer still is.
func TestCatalogBlockStopsAtAConfigKey(t *testing.T) {
	block := catalogBlock([]string{"backup"})
	for _, no := range []string{"config backup_dir:", "config verify_box_max_age:", "verify_box_accepted:"} {
		if strings.Contains(block, no) {
			t.Errorf("`posse backup --help` carries %q — a config key is not an entry of the verb's grammar:\n%s", no, block)
		}
	}
	// The same boundary at the end of the catalog's last entry: `posse
	// cockpit` is followed by a blank line and the `environment:` section.
	if c := catalogBlock([]string{"cockpit"}); strings.Contains(c, "RHQ_HOME") {
		t.Errorf("`posse cockpit --help` ran past its entry into the environment section:\n%s", c)
	}
}

// Where the reading sits is the fix. `posse cage --help` and `posse recipes
// --help` did their work and reported; `posse scorecard --help` scanned bd
// for every persona. All three are verbs whose own body is the first thing
// that would see '--help', so a reading moved below the switch answers
// nothing for them.
func TestHelpReadingSitsAheadOfTheVerbSwitch(t *testing.T) {
	_, at := mainVerbSwitch(t)
	fn := mainFuncBody(t)
	found := -1
	for i, st := range fn[:at] {
		if strings.Contains(stmtText(st), "catalogHelp") {
			found = i
		}
	}
	if found < 0 {
		t.Fatalf("main reads -h/--help nowhere before `switch cmd` (statement %d) — every verb whose body does work before its flag loop answers help by doing that work", at)
	}
}

// mainFuncBody returns main's top-level statements.
func mainFuncBody(t *testing.T) []ast.Stmt {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "main" && fn.Recv == nil {
			return fn.Body.List
		}
	}
	t.Fatal("main.go has no func main")
	return nil
}

// stmtText renders a statement's identifiers, which is all this file needs
// to say "the call is in here".
func stmtText(st ast.Stmt) string {
	var b strings.Builder
	ast.Inspect(st, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			b.WriteString(id.Name)
			b.WriteString(" ")
		}
		return true
	})
	return b.String()
}

// The alias table is the one hand-kept part of the surface, because the
// catalog documents an alias in prose ("(alias: focus)") instead of giving
// it a header. TestEveryVerbAnswersForHelp is what makes it total; this is
// what keeps it from going stale the other way.
func TestCatalogAliasNamesLiveVerbsAndLiveEntries(t *testing.T) {
	verbs := map[string]bool{}
	for _, v := range mainSwitchVerbs(t) {
		verbs[v] = true
	}
	for from, to := range catalogAlias {
		if !verbs[from] {
			t.Errorf("catalogAlias maps %q, which main's switch no longer accepts — drop it", from)
		}
		if catalogBlock([]string{from}) != "" {
			t.Errorf("catalogAlias maps %q, which the catalog now documents itself — drop the alias", from)
		}
		if catalogBlock([]string{to}) == "" {
			t.Errorf("catalogAlias sends %q to %q, which has no catalog entry", from, to)
		}
	}
}

// The preamble is the sentence this bead was filed over: it scoped the rule
// to "a subcommand that takes a <name>", which was the honest reading while
// need() was the only realization and a false one afterwards.
func TestPreambleStatesTheRuleForEverySubcommand(t *testing.T) {
	catalog := usageCatalog()
	head := catalog
	if i := strings.Index(catalog, "sessions (herdr workspaces):"); i > 0 {
		head = catalog[:i]
	}
	if strings.Contains(head, "A subcommand that takes a <name>") {
		t.Error("the preamble still scopes the -h/--help rule to the verbs that take a <name> — every verb keeps it now")
	}
	for _, want := range []string{"Every subcommand", "-h/--help", "every sub-verb this catalog names", "posse kill --\n-oddly-named"} {
		if !strings.Contains(head, want) {
			t.Errorf("the preamble no longer carries %q:\n%s", want, head)
		}
	}
}

// verbOwnHelp is the one seam where a verb's answer is not a verbatim slice
// of the catalog, so it is also the one place a stale or invented path could
// hide. Both halves are checked: the path has to be a real catalog path, and
// the wider text has to still BE the catalog's — contained in it verbatim,
// and a superset of the slice it replaces. TestBackupHelpIsTheCatalogBlockItself
// makes the same verbatim claim for backup from the other side; this one holds
// for whatever is added here next.
func TestVerbOwnHelpWidensARealPath(t *testing.T) {
	if len(verbOwnHelp) == 0 {
		t.Skip("no verb widens its own help — nothing to check")
	}
	catalog := usageCatalog()
	for path, own := range verbOwnHelp {
		words := strings.Fields(path)
		slice := catalogBlock(words)
		if slice == "" {
			t.Errorf("verbOwnHelp names %q, which is not a catalog path", path)
			continue
		}
		text := own()
		if !strings.Contains(catalog, text) {
			t.Errorf("verbOwnHelp[%q] is not a verbatim slice of the catalog — the verb's help and the catalog have become two copies:\n%s", path, text)
		}
		if !strings.Contains(text, slice) {
			t.Errorf("verbOwnHelp[%q] does not contain the block it replaces, so widening it LOST grammar:\n--- own ---\n%s\n--- block ---\n%s", path, text, slice)
		}
		if text == slice {
			t.Errorf("verbOwnHelp[%q] is exactly catalogBlock(%q) — it widens nothing and is an entry that cannot drift out of step, so remove it", path, path)
		}
		// And it is reached: the verb's own -h answers with the wide text.
		if block, ok := catalogHelp(append(append([]string{}, words...), "--help")); !ok || block != text {
			t.Errorf("`posse %s --help` did not answer with verbOwnHelp[%q]", path, path)
		}
	}
}
