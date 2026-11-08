package treepins

// QA pin for ranger-base-5syu4 — no Go source in this tree writes a shebang
// that names bash at a fixed path.
//
// WHY. ADR 0080 moves the fleet onto an always-on Linux host, and its
// Consequences recorded "the 38 `#!/bin/bash` literals in posse's Go renders
// (a grep, not an audit)" as a portability debt: a host without /bin/bash —
// NixOS, a minimal container image — cannot run a script that names it. The
// audit this bead did says the grep was right about the COUNT and wrong about
// the place. Not one of the 38 was a render: posse's own renders were already
// `#!/bin/sh` (internal/posse/gates.go, cageinner.go, hooksredirect.go,
// adrcensus.go, runtimeprobe.go) and all 38 were test-fixture shims and
// fixture TEXT under internal/treepins. Seventeen of them were executed by a
// test through WriteExecutable, which is what made the SUITE, not the
// renders, the thing a bash-less host could not run.
//
// They are `#!/bin/sh` now, bar one: bdpin_qa_test.go's `ps` stub uses
// bash's `${@: -1}` offset expansion, which POSIX has no spelling for, and
// carries `#!/usr/bin/env bash` plus a comment saying so. So the rule this
// pin holds is the ADR's own classification, in the only direction that
// survives a tree with no /bin/bash: a POSIX body is `#!/bin/sh`, a body
// that needs bash finds it through PATH, and a fixed `/bin/bash` is refused.
//
// WHAT IT READS, and why that and not bytes. Parsed string literals under
// cmd/ and internal/, test sources included — the fixtures are the whole
// subject, so excluding them would exclude everything the grep found. Not a
// byte census, for the reason bddbflag_qa_test.go gives one directory over:
// prose about a shebang is not a shebang, and a byte census reds on the
// paragraph above that explains the rule.
//
// WHAT IT CANNOT SEE, said out loud: a shebang assembled across a
// concatenation (`"#!" + "/bin/bash"`) is invisible to a literal census, and
// deliberately not chased. Hiding this shape behind an escape takes more
// work than typing the portable spelling, and the census that WOULD catch it
// is the byte census the paragraph above refuses.
//
// Doored in $(QA_CORPUS_PINS) — `make corpus-check`. It walks two tracked
// directories, so it is tree-wide by the treepins class rule and must have
// one (see treewidedoor_qa_test.go).

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// shbFixedBash is the spelling this pin refuses, and the one the negative
// witness below plants. It is a literal in this file, which is why this file
// is the single row on shbRoster: a census that cannot name the shape it
// refuses is a census nobody can mutation-check.
const shbFixedBash = "#!/bin/bash"

// shbRoster: Go files allowed to carry shbFixedBash as a string literal,
// each with the reason. Every row is checked back against the walk below —
// a row for a file that stopped carrying it is a door held open for nothing.
var shbRoster = map[string]string{
	"internal/treepins/bashshebang_qa_test.go": "this census's own subject and its negative witness — shbFixedBash, and the fixture that plants it",
}

type shbHit struct {
	file string
	line int
	text string
}

func shbSkipDir(name string) bool {
	return name == ".git" || name == ".beads" || name == "node_modules"
}

// shbScanFile parses one .go file and returns the literals in it that carry
// shbFixedBash, plus the number carrying a shebang of ANY flavor.
func shbScanFile(fset *token.FileSet, path string) (hits []shbHit, shebangs int, err error) {
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, 0, err
	}
	rel := filepath.ToSlash(path)
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		v, err := strconv.Unquote(lit.Value)
		if err != nil || !strings.Contains(v, "#!") {
			return true
		}
		shebangs++
		if strings.Contains(v, shbFixedBash) {
			first, _, _ := strings.Cut(v, "\n")
			hits = append(hits, shbHit{file: rel, line: fset.Position(lit.Pos()).Line, text: first})
		}
		return true
	})
	return hits, shebangs, nil
}

// shbScanGo walks the tree's Go sources and returns the files carrying
// shbFixedBash, the number of files parsed, and the number of literals
// carrying a shebang of any flavor. That last count is the positive witness:
// "no Go source names /bin/bash" is also what a walk that read nothing says,
// and a shebang count of zero means this census stopped being able to see the
// class it censuses.
//
// The two roots are spelled as literals in the range, not composed and not
// taken as a parameter, because that is the spelling the tree-wide
// derivation can read (treewidedoor_qa_test.go's twdRelTreePath is
// literal-only — a root behind a variable makes this pin undoored while the
// door stays green).
func shbScanGo(t *testing.T) (hits []shbHit, parsed, shebangs int) {
	t.Helper()
	fset := token.NewFileSet()
	for _, root := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if shbSkipDir(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			fileHits, n, err := shbScanFile(fset, path)
			if err != nil {
				return err
			}
			parsed++
			shebangs += n
			hits = append(hits, fileHits...)
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", root, err)
		}
	}
	return hits, parsed, shebangs
}

func TestQANoGoSourceWritesABinBashShebang(t *testing.T) {
	t.Parallel()

	hits, parsed, shebangs := shbScanGo(t)

	// The floors. MEASURED at this bead's commit, 2026-11-08: 906 .go files
	// parsed under the two roots and 352 string literals carrying a shebang
	// of some flavor. Set well under the live numbers so an ordinary landing
	// does not move them; they fail a walk that collapsed — a renamed root, a
	// suffix list that stopped matching, a parser that returned on the first
	// file.
	if parsed < 400 {
		t.Fatalf("the walk parsed %d .go files under cmd/ and internal/ — the corpus this pin censuses is not there", parsed)
	}
	if shebangs < 150 {
		t.Fatalf("the walk found %d string literals carrying a shebang at all, over %d files — a census that cannot see this tree's shebangs cannot say none of them names /bin/bash", shebangs, parsed)
	}

	byFile := map[string][]shbHit{}
	for _, h := range hits {
		byFile[h.file] = append(byFile[h.file], h)
	}
	var carriers []string
	for rel := range byFile {
		carriers = append(carriers, rel)
	}
	sort.Strings(carriers) // so a red names the same files in the same order on every box
	for _, rel := range carriers {
		if _, ok := shbRoster[rel]; ok {
			continue
		}
		for _, h := range byFile[rel] {
			t.Errorf("%s:%d writes a shebang naming bash at a fixed path: %q\n"+
				"  A host without /bin/bash cannot run it (ADR 0080; NixOS and minimal images have none).\n"+
				"  If the body is POSIX, say `#!/bin/sh`. If it needs bash, say `#!/usr/bin/env bash` and comment WHICH bashism — bdpin_qa_test.go's `ps` stub is the worked example.\n"+
				"  A shim that must not trust PATH keeps an absolute path and says why, on a roster row here.",
				h.file, h.line, h.text)
		}
	}

	// The roster's own rows have to be real.
	var excused []string
	for rel := range shbRoster {
		excused = append(excused, rel)
	}
	sort.Strings(excused)
	for _, rel := range excused {
		if len(byFile[rel]) == 0 {
			t.Errorf("shbRoster excuses %s (%s) and no such literal is there any more — drop the row", rel, shbRoster[rel])
		}
	}

	// ── the negative witness ──
	// Every roster row above is an exemption, so the live walk passing says
	// nothing about whether this census can still FAIL. Two planted files,
	// one per literal spelling, read by the same per-file scanner the walk
	// above reads every file with.
	for _, w := range []struct {
		name string
		src  string
	}{
		{"interpreted string", "package p\n\nvar body = \"" + shbFixedBash + "\\nexit 0\\n\"\n"},
		{"raw string", "package p\n\nvar body = `" + shbFixedBash + "\nexit 0\n`\n"},
	} {
		t.Run(w.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "planted.go")
			if err := os.WriteFile(p, []byte(w.src), 0o644); err != nil {
				t.Fatal(err)
			}
			planted, shebangs, err := shbScanFile(token.NewFileSet(), p)
			if err != nil {
				t.Fatal(err)
			}
			if shebangs != 1 {
				t.Fatalf("the witness read %d shebang literals, want 1 — the apparatus, not the tree", shebangs)
			}
			if len(planted) != 1 {
				t.Fatalf("a Go file whose literal is %q was not caught: %v — this census can no longer say no, and every green run above means nothing", shbFixedBash, planted)
			}
		})
	}
}
