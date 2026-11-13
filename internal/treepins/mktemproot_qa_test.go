package treepins

// QA pins for ranger-base-w8n6w — a `mktemp -d` root under scripts/ names a
// namespace scripts/reap-test-tmp.sh has a class for — and for
// ranger-base-u1z89, which is the same question one file type over: a
// `mktemp` with NO `-d` makes a FILE, and no walk in that script can see a
// file at all.
//
// THE DEFECT IS A CENSUS, NOT A ROOT. This is the third bead in one family to
// walk scripts/ by hand and the third to find roots the reaper cannot collect:
// ranger-base-m0j9n (prefixes with no class row), ranger-base-i78k9 (nine bare
// roots, whose own census named six of them) and ranger-base-tbukc (five
// templated roots that matched neither `posse-*` nor the bare `tmp.??????????`
// class, so NO row reached them and a leaked one was never collected at all).
// Every one of those beads renamed the roots it found and left nothing in the
// tree that asserts the property, so root 28 is found by the next leak and not
// by the suite. Fixing the roots again would be the fourth hand census.
//
// THE RULE. A live `mktemp -d` under scripts/ names the template
// `${TMPDIR:-/tmp}/posse-<what>.XXXXXX`, or is on mtExempt below with its
// reason. Both halves of that spelling are load-bearing, and for different
// reasons:
//
//   - `posse-` is the reaper's posse-scratch GLOB (reap-test-tmp.sh's class
//     table, 120m grace). That glob and only that glob reaches a shell root;
//     the other classes name Go's `$WORK`, t.TempDir's `Test*[0-9]`, and two
//     dead fixture names kept only to drain a pre-fix backlog. A root outside
//     it is matched by no row, which is tbukc's finding.
//   - `${TMPDIR:-/tmp}` is where the reaper LOOKS. Its default roots are /tmp
//     and this shell's $TMPDIR when that is somewhere else (reap-test-tmp.sh,
//     the `roots=(/tmp)` default under "Default: /tmp AND this shell's
//     $TMPDIR" — :775-778 today, and cited by its words because w8n6w's
//     ":625-632" had already rotted by the end of the day it was written),
//     so a root spelled against TMPDIR lands in a scanned root
//     whether the seat has one or not. A hardcoded `/tmp/posse-x.XXXXXX`
//     would be collected on this box and would also ignore the TMPDIR a caged
//     seat is given, which is the one case where the two differ.
//   - `.XXXXXX` is mktemp's own requirement for a template argument, and it
//     is asserted so that the pin's answer is a template and not a prefix a
//     reader guessed at.
//
// AND THE SAME QUESTION ONE FILE TYPE OVER (ranger-base-u1z89). The last
// paragraph of this comment used to say the pin does not reach a `mktemp`
// with no `-d`, "a FILE, which no class in that table globs". That was true
// and it was not a scope — it was a worse hole than the one above. EVERY
// candidate walk in reap-test-tmp.sh is `-type d`, so the class table is a
// table of DIRECTORY classes and a leaked scratch file is outside all nine
// rows: not tbukc's "no row globs this name" but "no walk could reach it
// whatever it is named", the `tmp.??????????` row that globs exactly what a
// bare `mktemp` produces included. The high-frequency producer was
// scripts/bd-argv-gate.sh, which made one per bd-ish tool call of every seat
// on this box.
//
// THE FILE RULE IS A PROHIBITION, NOT A TEMPLATE: a live `mktemp` with no
// `-d` is on mfExempt with its reason, or it reds. There is no conforming
// spelling to offer, because the house answer is not to make a scratch file
// in a swept root at all — make the dir THE RULE ABOVE describes and put a
// fixed-name file inside it, which needs no second mktemp. All ten producers
// were fixed that way (see docs/notes.d/ranger-base-u1z89.md) and what the
// census finds now is two calls, both rostered and neither in /tmp.
//
// IT WALKS plugin/ AS WELL AS scripts/, AND ONLY THE FILE ARM DOES. The root
// census's subject is the directory three beads walked by hand, and widening
// it would be a claim about roots nobody has censused; the file rule is new
// and has no legacy, so it starts at every directory in this tree that holds
// shipped shell. plugin/ holds one script, and that script was one of the ten
// producers. docs/adr/*.probe.sh is out of both on purpose: those are
// operator-run artifacts rather than this tree's machinery, and their bare
// `mktemp -d` roots do land in the reaper's `tmp.??????????` class, which
// collects them at 24h.
//
// MEASURED 2026-11-12 (linux, this box), with ranger-base-u1z89's fixes in:
// 43 grep hits for `mktemp -d` under scripts/, of which 11 are comment lines
// explaining the mechanism and 1 is prose inside verify-box.sh's quoted
// ROSTER heredoc. The reader below reports the other 31 as live roots — 28 of
// them the house spelling and three on mtExempt with their reasons. The walk
// sees 63 files under scripts/ and 65 with plugin/, and finds 2 live `mktemp`
// FILE calls, both on mfExempt: reap-test-tmp.sh's own class-table row, which
// a line reader cannot tell from a call, and gotest.sh's per-slot build temp,
// which is under $CACHE and not in /tmp. w8n6w read 36 hits / 8 comments / 27
// roots / 24 conforming over the same directory earlier the same day, and
// said 56 files. The seven extra grep hits are u1z89's four producers turning
// from files into dirs plus three comment lines naming the mechanism; the
// file count moved with whatever else landed between the two readings, which
// is the one number here nobody should read a story into. Neither count was a
// hand census.
//
// THE READER IS LINE-BASED AND LOOSE IN THE SAFE DIRECTION. Comment lines are
// dropped, because every one of this family's beads is explained in prose
// above the code it fixed and that prose creates nothing. Everything else that
// names `mktemp` with a `-d` flag counts, INCLUDING a trailing comment on a
// runnable line and a line the shell would never reach. Over-counting costs a
// roster row and a sentence; under-counting is the next bead in the family.
//
// AND A HEREDOC BODY IS CLASSIFIED RATHER THAN DROPPED (mtText). A heredoc is
// data this script hands to something else, so a `mktemp -d` in one is not a
// root this script creates — but it is not nothing either: a heredoc that
// RENDERS a script with a bare `mktemp -d` in it leaks a dir under the
// rendered script's name, which is exactly the hole with no class row. There
// are two such lines in the tree today and both are prose in the same roster
// table, so the pin holds the classification instead of guessing it: a new
// heredoc naming `mktemp` reds until someone says which of the two it is.
// Both KINDS go in that one list and the ROOT arm checks it, because mtText
// is one claim per file and splitting it would make two places answer for one
// heredoc.
//
// WHAT IT DOES NOT REACH, said once so no reader has to rediscover it: a root
// built by `mkdir -p "$TMPDIR/$$"` or any other spelling of the same idea. The
// `mktemp` with no `-d` that used to head this list is the second arm's whole
// subject now (ranger-base-u1z89).
//
// A root created by Go's os.MkdirTemp or Python's tempfile WAS on that list —
// "the tests' half of the posse-scratch class, pinned nowhere here" — and is
// temproot_qa_test.go's now (ranger-base-qbg7c, which found ten such roots
// outside the namespace plus an eleventh this file's own census could never
// have seen). The two pins are one rule over two languages and deliberately
// not one reader: this one is a line-based shell reader and that one is a
// go/parser census, so folding them would mean a shell parse or an AST over
// bash. Each says in prose which languages it owns, and `make scripts-check`
// and `make corpus-check` are the two doors.

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// mtRequired — the house template. `<what>` is lowercase alphanumeric words
// joined by hyphens, which is how all 24 conforming roots spell it.
var mtRequired = regexp.MustCompile(`^\$\{TMPDIR:-/tmp\}/posse-[a-z0-9]+(?:-[a-z0-9]+)*\.XXXXXX$`)

// mtMktempWord matches `mktemp` as a word, so neither `posse-mktemp` nor a
// variable named MKTEMP is read as a call.
var mtMktempWord = regexp.MustCompile(`\bmktemp\b`)

// mtArgFlags — the mktemp flags that take a separate argument, so the field
// after them is not the template. None of them is used under scripts/ today
// (MEASURED 2026-11-12); they are here because getting this wrong would make
// the reader report a DIRECTORY as a template, and that is a false answer
// rather than a strict one.
var mtArgFlags = map[string]bool{"-p": true, "--tmpdir": true, "--suffix": true}

// mtRoot is one `mktemp` the reader found: where it is, the line, the
// template it names — empty when it names none, which is the bare
// `d=$(mktemp -d)` ranger-base-i78k9 found nine of — and whether the call
// asked for a directory.
type mtRoot struct {
	Script   string
	Line     int
	Code     string
	Template string
	Dir      bool
}

// At returns the `path:line` a diagnostic leads with.
func (r mtRoot) At() string { return fmt.Sprintf("%s:%d", r.Script, r.Line) }

// mtParse reads one runnable line and reports the `mktemp` on it: the
// template it names, and whether it asked for a directory. It used to answer
// ok=false for a call with no `-d` — "a file and not a root in any of the
// reaper's classes" — and that sentence was the hole ranger-base-u1z89
// found, not a scope. Both kinds are reported now and isDir is what tells
// the two arms apart.
//
// One call per line: both halves of this tree spell it `x=$(mktemp -d …)` on a
// line of its own (MEASURED 2026-11-12, all 32). A second call on the same
// line would be missed, which is the loose direction — but a line-based reader
// has no way to find it that does not amount to a shell parse, so it is said
// here rather than half-handled.
func mtParse(code string) (template string, isDir, ok bool) {
	loc := mtMktempWord.FindStringIndex(code)
	if loc == nil {
		return "", false, false
	}
	skipNext := false
	for _, f := range strings.Fields(code[loc[1]:]) {
		if skipNext {
			skipNext = false
			continue
		}
		if f == "--" {
			continue
		}
		if strings.HasPrefix(f, "-") && len(f) > 1 {
			if mtArgFlags[f] {
				skipNext = true
				continue
			}
			if strings.ContainsRune(strings.TrimLeft(f, "-"), 'd') {
				isDir = true
			}
			continue
		}
		// The first non-flag field is the template. Trim the shell syntax a
		// line-based reader cannot otherwise see past: the quotes around it,
		// and the `)` closing a `$(` plus whatever follows the call on the
		// line (`|| die …`, `; …`).
		template = strings.TrimLeft(strings.TrimRight(f, `)"';&|`), `"'`)
		break
	}
	return template, isDir, true
}

// mtRead reads <script> and returns every live `mktemp -d` in it as a root,
// every live `mktemp` with no `-d` as a file, and every call of either kind
// inside a heredoc BODY as text — three lists because they are three
// different claims (see the head comment). The heredoc matcher is
// brewguard_qa_test.go's bgHeredoc: one spelling of that structure in this
// package, so a repair to either reader reaches both.
func mtRead(script string) (roots, files, text []mtRoot, err error) {
	b, err := os.ReadFile(script)
	if err != nil {
		return nil, nil, nil, err
	}
	hereTerm, hereDash := "", false
	for i, line := range strings.Split(string(b), "\n") {
		code := strings.TrimSpace(line)
		at := mtRoot{Script: script, Line: i + 1, Code: code}
		if hereTerm != "" {
			if line == hereTerm || (hereDash && code == hereTerm) {
				hereTerm = ""
				continue
			}
			if tmpl, dir, ok := mtParse(code); ok {
				at.Template, at.Dir = tmpl, dir
				text = append(text, at)
			}
			continue
		}
		if code == "" || strings.HasPrefix(code, "#") {
			continue
		}
		if m := bgHeredoc.FindStringSubmatch(code); m != nil {
			hereTerm, hereDash = m[2], m[1] == "-"
		}
		if tmpl, dir, ok := mtParse(code); ok {
			at.Template, at.Dir = tmpl, dir
			if dir {
				roots = append(roots, at)
			} else {
				files = append(files, at)
			}
		}
	}
	return roots, files, text, nil
}

// mtExempt — the live roots that do NOT name the house template, with the
// reason, in the two-list shape scripts/verify-box.sh's ROSTER/EXCLUDED census
// uses and brewguard's bgRoster/bgExempt already keeps next door. Matched on
// (script, template) rather than on a line number: a line number is stale on
// the next edit above it, and a template is the thing the row is about.
//
// An exemption is a claim about why the reaper does not need to reach this
// root, and the three claims here are three different reasons — which is the
// argument for a roster with prose in it rather than a wider regexp.
var mtExempt = []struct{ Script, Template, Reason string }{
	{
		"scripts/audit-silent-reverts.sh", "$SR_TMPROOT/XXXXXX",
		"nested INSIDE posse-audit-silent-reverts.* — SR_TMPROOT is itself a conforming root made at :607, and this is the per-case dir under it. The PARENT is what the reaper collects, and collecting it takes this with it; naming the parent's template again here would be a second place for it to drift.",
	},
	{
		"scripts/verify-ghost-composer.sh", "/private/tmp/posse-ghost.XXXXXX",
		"darwin-only, and the PATH is the exemption now rather than the whole template (ranger-base-qbg7c renamed the prefix into posse's namespace). /private/tmp is what darwin's /tmp resolves to, so the reaper's default root already looks there and `posse-*` reaches this root; what it cannot have is ${TMPDIR:-/tmp}, because darwin's TMPDIR is a per-user /var/folders/… path and a unix socket under it would blow the 104-byte sun_path cap this line exists to stay inside.",
	},
	{
		"scripts/verify-self-close.sh", "/private/tmp/posse-selfclose.XXXXXX",
		"darwin-only and length-bound the same way, with the same half-fix: the prefix is inside posse's namespace (ranger-base-qbg7c) and the hardcoded /private/tmp is what stays. It is ALSO pinned by name in internal/posse/selfclose_qa_test.go, so the rename was a two-file change across the arm boundary — which is why ranger-base-tbukc left it alone and why this row is the place that says the path is deliberate.",
	},
}

// mtText — a `mktemp -d` the reader finds inside a heredoc BODY, with what it
// is. Keyed by script: the claim is about the file's heredocs, and a second
// line in the same file is the same claim.
var mtText = map[string]string{
	"scripts/verify-box.sh": "prose, inside the quoted `ROSTER_EOF` heredoc that IS this script's roster table — two rows: verify-reap-test-tmp describes that check as running against a fresh `mktemp -d` root, and verify-pattern-kill-census says its fixtures sit \"in a mktemp root\", which the reader scores as a FILE call because the words -d are not in that sentence. The heredoc is read by this script's own census and never executed by anything.",
}

// mfExempt — the live `mktemp` calls with no `-d`, with the reason. The file
// rule has no conforming spelling to pass (see the head comment): the house
// answer is a dir from the rule above with a fixed-name file inside it, so
// every live file call is either a row here or a red.
//
// ONE ROW, AND IT IS ALSO THE ARM'S BLINDNESS CONTROL. A census whose live
// count is one cannot prove it is sighted by counting the way the root arm's
// `roots < 20` floor does — but this list is checked both ways, so a reader
// that stopped finding anything reds on a stale row rather than going quiet.
var mfExempt = []struct{ Script, Template, Reason string }{
	{
		"scripts/reap-test-tmp.sh", "|tmp.??????????|1440|1",
		"NOT A CALL: it is the `mktemp` row of that script's own class_table, a multi-line single-quoted string whose rows are `name|glob|grace|depth`. A line-based reader cannot see that it is inside a quoted block — the same blindness mtText answers for a heredoc — and the fix is this row rather than a narrower matcher, so the file census and the root census keep ONE reader between them. The whole template is the rest of the table row, so changing that class's grace or depth reds this row: update the string, it is two characters.",
	},
	{
		"scripts/gotest.sh", "$CACHE/.build.$sl.XXXXXX",
		"not in a root the reaper sweeps: $CACHE is $HOME/.cache/posse/testbin, the binary cache this script owns and prunes itself (--prune, KEEP). It is a temp file for an ATOMIC rename into that cache, so it has to be created on the cache's own filesystem, and a dir would be the wrong shape for a `mv` of one file.",
	},
}

// mtFileSet sorts what an arm's walk found and holds it to a floor.
//
// BOTH ARMS SPELL THE WALK THEMSELVES, and that duplication is deliberate:
// treewidedoor_qa_test.go derives this class — "a QA test whose subject is
// the TREE" — from an enumeration rooted at a path spelled as a LITERAL in
// the test, and a walk moved behind a `roots ...string` parameter is a
// spelling that rule cannot see. MEASURED 2026-11-12: with the walk in a
// variadic helper, `make register-check` reported BOTH arms "no longer
// derived as tree-wide ... the pin is back to being a package-tree run
// away", which is the hole that whole register exists to close. So what is
// shared is everything that is a RULE — the reader, the floor, the rosters —
// and what is repeated is eight lines of os.DirEntry boilerplate with no
// claim in it.
//
// Each arm WALKS, for the reason ranger-base-80zqb gave one file over: an
// os.ReadDir of scripts/ reads one level, so "every root under scripts/"
// would be a claim about the top of the directory only. scripts/ is flat
// today and nothing is escaping; the walk is what makes the claim good the
// day it is not, and asserting flatness instead would be a layout ruling this
// pin has no business making.
//
// The floor is reader sanity before either arm trusts its own census: an
// assertion that every root is rostered is satisfied by a walk that found
// none, and a walk that found none is silent about it. It sits well under the
// live counts (63 files under scripts/, 65 with plugin/, MEASURED
// 2026-11-12), so an ordinary landing does not move it.
func mtFileSet(t *testing.T, roots []string, found []string) []string {
	t.Helper()
	sort.Strings(found)
	if len(found) < 40 {
		t.Fatalf("the walk of %v found only %d file(s) — the walk is broken, not the tree: %v", roots, len(found), found)
	}
	return found
}

// mtWalkInto is the body of each arm's walk, kept here so the two cannot
// disagree about what counts as a file. The WalkDir CALL stays in the arm —
// see mtFileSet on why.
func mtWalkInto(out *[]string) fs.WalkDirFunc {
	return func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		*out = append(*out, path)
		return nil
	}
}

// Every live `mktemp -d` under scripts/ names a namespace the reaper has a
// class for, or is rostered with its reason. Two-way, like brewguard's and
// boxcheck's: a row no longer found by the census reds as stale, so neither
// list can go quiet while staying green.
func TestQAEveryMktempDirRootUnderScriptsIsReapedOrRostered(t *testing.T) {
	t.Parallel()
	walkRoots := []string{"scripts"}
	var walked []string
	for _, root := range []string{"scripts"} {
		if err := filepath.WalkDir(root, mtWalkInto(&walked)); err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	paths := mtFileSet(t, walkRoots, walked)

	type key struct{ script, template string }
	exempt := map[key]string{}
	for _, e := range mtExempt {
		exempt[key{e.Script, e.Template}] = e.Reason
	}
	seenExempt := map[key]bool{}
	seenText := map[string]bool{}
	roots := 0
	for _, path := range paths {
		found, _, text, err := mtRead(path)
		if err != nil {
			t.Errorf("read %s: %v", path, err)
			continue
		}
		roots += len(found)
		for _, r := range found {
			k := key{r.Script, r.Template}
			if reason, ok := exempt[k]; ok {
				seenExempt[k] = true
				if len(strings.TrimSpace(reason)) < 40 {
					t.Errorf("%s is exempted from the reaper-namespace rule with no reason worth reading (%q) — an exemption nobody can check is the rule being dropped quietly", r.At(), reason)
				}
				if mtRequired.MatchString(r.Template) {
					t.Errorf("%s names the house template %q AND is on mtExempt — one place, one answer, and the roster row is the one to delete", r.At(), r.Template)
				}
				continue
			}
			if mtRequired.MatchString(r.Template) {
				continue
			}
			t.Errorf("%s makes a scratch root as %s\n"+
				"  template: %q\n"+
				"  want:     ${TMPDIR:-/tmp}/posse-<what>.XXXXXX, or a row in mtExempt (%s) with the reason\n"+
				"Only scripts/reap-test-tmp.sh's posse-scratch class (glob `posse-*`, 120m grace) reaches a shell root, and it looks in /tmp and $TMPDIR — so a root spelled any other way is collected by no class at all, and a leaked one stays until an operator finds it. This is the fourth time that hole has been found by hand (ranger-base-m0j9n, -i78k9, -tbukc, -w8n6w).",
				r.At(), r.Code, r.Template, "internal/treepins/mktemproot_qa_test.go")
		}
		for _, r := range text {
			if reason, ok := mtText[r.Script]; ok {
				seenText[r.Script] = true
				if len(strings.TrimSpace(reason)) < 40 {
					t.Errorf("%s is classified as heredoc text with no reason worth reading (%q)", r.At(), reason)
				}
				continue
			}
			t.Errorf("%s names `mktemp -d` inside a heredoc BODY and is in neither mtText nor the live census:\n  %s\n"+
				"Say which it is in mtText (%s). If the heredoc is prose or a roster row, the reason is one sentence. If it RENDERS a script that will run that line, the rendered root leaks under its own name and no class in the reaper's table globs it — give it the house template instead.",
				r.At(), r.Code, "internal/treepins/mktemproot_qa_test.go")
		}
	}
	if roots < 20 {
		t.Fatalf("the census found only %d live `mktemp -d` root(s) under scripts/ — the reader is broken, not the tree (31, MEASURED 2026-11-12)", roots)
	}
	for _, e := range mtExempt {
		if !seenExempt[key{e.Script, e.Template}] {
			t.Errorf("mtExempt names %s with template %q and the census finds no such root — a stale roster row is an exemption over nothing, and it is also how this census would go blind without saying so", e.Script, e.Template)
		}
	}
	for p := range mtText {
		if !seenText[p] {
			t.Errorf("mtText names %s and the census finds no `mktemp -d` in a heredoc body there — delete the row rather than leave a classification nobody will check again", p)
		}
	}
}

// And no live `mktemp` with no `-d` anywhere in the shipped shell, unless it
// is rostered on mfExempt with its reason (ranger-base-u1z89). The rule is a
// prohibition rather than a template: a scratch FILE in a root the reaper
// sweeps is reached by none of its nine classes, because every candidate walk
// there is `-type d`, and the fix is to make the dir the rule above describes
// and put a fixed-name file inside it — which needs no second mktemp at all.
//
// Two-way, like the root arm: a row that the census no longer finds reds as
// stale, which is the only liveness signal an arm with one live member has.
func TestQAEveryMktempFileInTheShippedShellIsRosteredOrExempt(t *testing.T) {
	t.Parallel()
	// The list is spelled twice on purpose: the one the `range` reads has to
	// be a LITERAL where the walk is, or treewidedoor_qa_test.go stops
	// deriving this arm as tree-wide (mtFileSet's comment has the
	// measurement). The copy in walkRoots is only what a diagnostic prints.
	walkRoots := []string{"scripts", "plugin"}
	var walked []string
	for _, root := range []string{"scripts", "plugin"} {
		if err := filepath.WalkDir(root, mtWalkInto(&walked)); err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	paths := mtFileSet(t, walkRoots, walked)

	type key struct{ script, template string }
	exempt := map[key]string{}
	for _, e := range mfExempt {
		exempt[key{e.Script, e.Template}] = e.Reason
	}
	seen := map[key]bool{}
	for _, path := range paths {
		_, found, _, err := mtRead(path)
		if err != nil {
			t.Errorf("read %s: %v", path, err)
			continue
		}
		for _, r := range found {
			k := key{r.Script, r.Template}
			if reason, ok := exempt[k]; ok {
				seen[k] = true
				if len(strings.TrimSpace(reason)) < 40 {
					t.Errorf("%s is exempted from the no-scratch-file rule with no reason worth reading (%q) — an exemption nobody can check is the rule being dropped quietly", r.At(), reason)
				}
				continue
			}
			t.Errorf("%s makes a scratch FILE as %s\n"+
				"  template: %q\n"+
				"  want:     a `mktemp -d \"${TMPDIR:-/tmp}/posse-<what>.XXXXXX\"` and a fixed-name file inside it, or a row in mfExempt (%s) with the reason\n"+
				"Every candidate walk in scripts/reap-test-tmp.sh is `-type d`, so a leaked scratch FILE is outside all nine of its classes — not a 24h quarantine but no walk that could reach it whatever it is named, the `tmp.??????????` row that globs a bare mktemp's own name included. /tmp here is a 14G tmpfs with one box-wide inode budget for uid posse, and the near-outage that script exists for was inode exhaustion at 29%% of bytes. The reaper's own refusal of a `-type f` arm, and the three reasons for it, are in that script's \"WHAT IT WILL NOT TAKE\" header.",
				r.At(), r.Code, r.Template, "internal/treepins/mktemproot_qa_test.go")
		}
	}
	for _, e := range mfExempt {
		if !seen[key{e.Script, e.Template}] {
			t.Errorf("mfExempt names %s with template %q and the census finds no such call — a stale roster row is an exemption over nothing, and over a live population this small it is also the only way this census can say it has gone blind", e.Script, e.Template)
		}
	}
}

// The reader and the rule, over planted scripts, because an assertion that
// only ever ran over a tree that passes is a census nobody has shown can say
// no — and "it reds on a root renamed back" is this bead's own done-when.
//
// It walks nothing: the fixtures are absolute paths under t.TempDir(), so this
// arm is not in the tree-wide class and needs no door of its own.
func TestQATheMktempRootReaderCanStillSayNo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name, body string) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	// What the reader must SEE, and what it must leave alone.
	script := write("probe.sh", strings.Join([]string{
		"#!/usr/bin/env bash",
		"# a bare mktemp -d in a comment is prose, not a root",
		`good=$(mktemp -d "${TMPDIR:-/tmp}/posse-probe.XXXXXX")`,
		"bare=$(mktemp -d)",
		`hard=$(mktemp -d /tmp/posse-probe.XXXXXX) || exit 1`,
		`nested=$(mktemp -d "$PARENT/XXXXXX")`,
		`file=$(mktemp "${TMPDIR:-/tmp}/posse-probe.XXXXXX")`,
		"barefile=$(mktemp)",
		"cat <<'DATA'",
		"rendered=$(mktemp -d)",
		"DATA",
		`last=$(mktemp -d "${TMPDIR:-/tmp}/posse-probe-last.XXXXXX") # trailing comment`,
		"",
	}, "\n"))
	roots, fileCalls, text, err := mtRead(script)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range roots {
		got = append(got, fmt.Sprintf("%d=%s", r.Line, r.Template))
	}
	want := []string{
		"3=${TMPDIR:-/tmp}/posse-probe.XXXXXX",
		"4=",
		"5=/tmp/posse-probe.XXXXXX",
		"6=$PARENT/XXXXXX",
		"12=${TMPDIR:-/tmp}/posse-probe-last.XXXXXX",
	}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("live roots = %v, want %v — the comment and the heredoc body are the two the census must not read as roots, the `mktemp` with no -d is a FILE and belongs in the other list, and the trailing-comment line is the one it must read", got, want)
	}
	// The same line the root list must NOT carry has to turn up here, or the
	// file rule is reading nothing (ranger-base-u1z89).
	var gotFiles []string
	for _, r := range fileCalls {
		gotFiles = append(gotFiles, fmt.Sprintf("%d=%s", r.Line, r.Template))
	}
	wantFiles := []string{"7=${TMPDIR:-/tmp}/posse-probe.XXXXXX", "8="}
	if strings.Join(gotFiles, " ") != strings.Join(wantFiles, " ") {
		t.Errorf("live file calls = %v, want %v — a `mktemp` with no -d is the subject of the file arm, templated or bare, and a reader that drops either is the hole ranger-base-u1z89 closed", gotFiles, wantFiles)
	}
	if len(text) != 1 || text[0].Line != 10 || !text[0].Dir {
		t.Errorf("heredoc-body calls = %v, want the one `mktemp -d` at line 10 — a call in a rendered script is a classification this pin holds, not a line it drops, and which KIND it is is half of that classification", text)
	}

	// The rule itself, over the near misses. Each of these is a root the
	// reaper's posse-scratch glob, or its scan roots, would not reach.
	for _, tc := range []struct {
		template string
		want     bool
	}{
		{"${TMPDIR:-/tmp}/posse-probe.XXXXXX", true},
		{"${TMPDIR:-/tmp}/posse-two-words-here.XXXXXX", true},
		{"", false},                                       // the bare mktemp -d: i78k9's nine
		{"/tmp/posse-probe.XXXXXX", false},                // right glob, ignores the seat's TMPDIR
		{"${TMPDIR:-/tmp}/probe.XXXXXX", false},           // no class row globs it: tbukc's five
		{"${TMPDIR:-/tmp}/posse-probe.XXXXX", false},      // five X: not a template mktemp takes
		{"${TMPDIR}/posse-probe.XXXXXX", false},           // no default: empty TMPDIR makes /posse-…
		{"$TMPDIR/posse-probe.XXXXXX", false},             // ditto, and unbraced
		{"${TMPDIR:-/tmp}/posse-probe.XXXXXX.bak", false}, // the X's are not the tail
		{"${TMPDIR:-/tmp}/sub/posse-probe.XXXXXX", false}, // a level down: the glob is one deep
		{"${TMPDIR:-/tmp}/posse-Probe.XXXXXX", false},     // the house spelling is lowercase
		{"${TMPDIR:-/tmp}/posse-probe.XXXXXXX", false},    // seven X
		{"${TMPDIR:-/tmp}/posse-.XXXXXX", false},          // names no `what`
		{"x${TMPDIR:-/tmp}/posse-probe.XXXXXX", false},    // anchored at the start
		{"${TMPDIR:-/tmp}/posse-probe.XXXXXX ", false},    // and at the end
	} {
		if got := mtRequired.MatchString(tc.template); got != tc.want {
			t.Errorf("mtRequired.MatchString(%q) = %v, want %v", tc.template, got, tc.want)
		}
	}

	// And the flag reading, which is the one place a wrong answer would be
	// permissive rather than strict: a `-p DIR` read as a template would
	// report a directory that happens to be spelled right.
	for _, tc := range []struct {
		code     string
		template string
		isDir    bool
		ok       bool
	}{
		{`d=$(mktemp -d "${TMPDIR:-/tmp}/posse-x.XXXXXX")`, "${TMPDIR:-/tmp}/posse-x.XXXXXX", true, true},
		{`d=$(mktemp --directory "${TMPDIR:-/tmp}/posse-x.XXXXXX")`, "${TMPDIR:-/tmp}/posse-x.XXXXXX", true, true},
		{`d=$(mktemp -d)`, "", true, true},
		{`d=$(mktemp -d -p "${TMPDIR:-/tmp}/posse-x.XXXXXX")`, "", true, true},
		{`d=$(mktemp -u -d "${TMPDIR:-/tmp}/posse-x.XXXXXX")`, "${TMPDIR:-/tmp}/posse-x.XXXXXX", true, true},
		// A FILE is reported now, where it used to be dropped as "not a root
		// in any of the reaper's classes" — which was true and was the hole
		// (ranger-base-u1z89). The line after it is the one that pays for
		// that looseness: a runnable line that merely NAMES mktemp without a
		// `-d` reads as a file call with the next word as its template, and
		// the answer to that is a row in mfExempt saying it is prose, the
		// way mtText already answers it for a heredoc. Over-counting costs a
		// roster row and a sentence; under-counting is the next bead in this
		// family.
		{`f=$(mktemp "${TMPDIR:-/tmp}/posse-x.XXXXXX")`, "${TMPDIR:-/tmp}/posse-x.XXXXXX", false, true},
		{`f=$(mktemp)`, "", false, true},
		{`echo "no mktemp call here"`, "call", false, true},
		{`echo "nothing to see"`, "", false, false},
	} {
		tmpl, isDir, ok := mtParse(tc.code)
		if ok != tc.ok || isDir != tc.isDir || tmpl != tc.template {
			t.Errorf("mtParse(%q) = (%q, %v, %v), want (%q, %v, %v)", tc.code, tmpl, isDir, ok, tc.template, tc.isDir, tc.ok)
		}
	}
}
