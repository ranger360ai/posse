package treepins

// QA pin for ranger-base-9mjxb — posse's own bd calls never name a store with
// a flag.
//
// THE DEFECT, as the operator measured it on the work box 2026-10-08 and
// filed as github.com/ranger360ai/posse issue 9: on bd 0.50.3, from a cwd
// inside repo A, a bd invocation carrying an explicit store-selecting global
// flag that names repo B's database auto-imports A's `.beads/issues.jsonl`
// — discovered from the cwd's git repo — into B's store. A READ verb writes
// one repository's records into another repository's database, silently and
// beyond the import line. We are pinned at 0.50.3 and will not file upstream
// (standing ruling 2026-10-08).
//
// WHY A PIN AND NOT A FIX. Nothing posse can do makes bd resolve the flag
// honestly. What posse can do is never hand it one: a bd call made with the
// working directory inside the repo that owns the store needs no flag, and
// then there is no second repository in the question at all. That is already
// how the Go runner works — internal/posse/beads.go's `Bd.runOnce` sets
// `cmd.Dir` and rebinds `BEADS_DIR` to `beadsHome(dir)`, and `bdGlobalFlags`
// is `--no-daemon` and nothing else — and since ranger-base-9mjxb it is also
// how scripts/prune-bd-relates-to.sh works, which was the one shipped call
// site that passed the flag (three writes, from whatever directory the
// operator was standing in, against a database `BEADS_DB` is documented to
// point at in another repository).
//
// This pin is what keeps the next call site from reintroducing it. The risk
// is not hypothetical on this box: it runs three stores — two repos whose
// `.beads/redirect` names a third, and a separate product repo with its own
// — so any posse code reaching for a second store is one flag away from
// writing the wrong one.
//
// THE TWO ARMS, and why the rule is spelled differently in each. A shell
// line is an invocation, so the shell arm can ask the precise question: does
// a line run `bd` as a command word AND carry the flag? Go hands bd an argv
// slice built anywhere, so no line answers it; the Go arm asks the question
// one step back — which files mention the flag as a STRING LITERAL at all —
// and holds a roster of the ones that legitimately do. Parsed literals, not
// file bytes, for the reason beads.go's own `bdStoreEnvShed` comment gives:
// prose about a flag is not a flag, and a byte census reds on the paragraph
// that explains the rule.
//
// WHAT IS OUT OF SCOPE, said out loud. Test sources are not read. They are
// full of this flag as FIXTURE INPUT — bdshim_test.go and
// bdargvgate_qa_test.go feed it to the gate's verb matcher by the dozen,
// which is the matcher's whole job — so a roster over tests would be mostly
// parser fixtures and would stop saying anything. A test that shells the real
// bd with a cross-repository flag is therefore not caught here; what catches
// that class is bddaemonleak_qa_test.go's census of real-bd call sites in the
// test corpus, which names every one of them for a different reason.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// bdfCmdWord matches `bd` in a position where a shell runs it: at the head of
// a line, or after whitespace or one of the operators that starts a new
// command word. Deliberately over-broad on `echo "  bd …"`, where the same
// bytes sit inside a string: the default has to be inclusion, because a line
// this cannot classify is a line nobody has read.
var bdfCmdWord = regexp.MustCompile(`(^|[\s;&|(])bd\s`)

// bdfStoreFlag matches bd's store-selecting global option in both spellings
// it accepts — the separate-argument form and the `=` form. The bare literal
// matches too: `$` anchors at the end of the string with no multiline flag
// set, which is what an argv element carrying nothing else looks like.
var bdfStoreFlag = regexp.MustCompile(`--db(\s|=|$)`)

// bdfGoLiteralRoster is every non-test Go file that may carry the flag as a
// string literal, with the reason. A new entry is a claim that the file
// parses somebody else's argv rather than building its own.
var bdfGoLiteralRoster = map[string]string{
	"internal/posse/gates.go": "globalValueOpts[\"bd\"] — the global options bd takes a SEPARATE value for, so the rendered gate's verb matcher skips the value instead of reading it as the verb (ranger-base-3bqn, ADR 0015 §3). It reads an argv a persona typed and builds none.",
}

// bdfScriptFile reports whether a path is a shipped file a shell or python
// interpreter runs. The root Makefile counts: its recipe lines are shell.
func bdfScriptFile(rel string) bool {
	if rel == "Makefile" {
		return true
	}
	switch filepath.Ext(rel) {
	case ".sh", ".py", ".bash", ".zsh":
		return true
	}
	return false
}

// bdfSkipDir names the directories the walk does not enter: git's own store,
// and the beads store, whose `issues.jsonl` is a projection of bead text and
// not shipped source.
func bdfSkipDir(name string) bool {
	return name == ".git" || name == ".beads" || name == "node_modules"
}

type bdfHit struct {
	file string
	line int
	text string
}

// bdfScanScripts returns every line in the tree that runs bd as a command
// word with the store-selecting flag on it, plus the two counts that say the
// walk read something: files examined and bd command-word lines seen at all.
func bdfScanScripts(t *testing.T) (hits []bdfHit, files, bdLines int) {
	t.Helper()
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if bdfSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		rel := filepath.ToSlash(path)
		if !bdfScriptFile(rel) {
			return nil
		}
		files++
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				continue
			}
			if !bdfCmdWord.MatchString(line) {
				continue
			}
			bdLines++
			if bdfStoreFlag.MatchString(line) {
				hits = append(hits, bdfHit{file: rel, line: i + 1, text: strings.TrimSpace(line)})
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree for shipped scripts: %v", err)
	}
	return hits, files, bdLines
}

// bdfScanGo returns the non-test Go files under cmd/ and internal/ that carry
// the flag as a parsed string literal, and the number of files parsed.
func bdfScanGo(t *testing.T) (map[string][]int, int) {
	t.Helper()
	found := map[string][]int{}
	parsed := 0
	fset := token.NewFileSet()
	for _, root := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if bdfSkipDir(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			rel := filepath.ToSlash(path)
			if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
				return nil
			}
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			parsed++
			ast.Inspect(f, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				v, err := strconv.Unquote(lit.Value)
				if err != nil {
					return true
				}
				if bdfStoreFlag.MatchString(v) {
					found[rel] = append(found[rel], fset.Position(lit.Pos()).Line)
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", root, err)
		}
	}
	return found, parsed
}

func TestQANoShippedPosseCallHandsBdAStoreSelectingFlag(t *testing.T) {
	t.Parallel()

	// ── arm 1: shell, python and the Makefile ──
	hits, files, bdLines := bdfScanScripts(t)
	// The positive witness. An assertion of pure absence is satisfied by a
	// walk that read nothing — a renamed directory, a suffix list that stopped
	// matching — and "no shipped script hands bd a store flag" is exactly what
	// a walk over zero files says. These floors sit well under the live counts
	// (MEASURED 2026-10-09 at this bead's commit: 61 shipped script files, 106
	// lines running bd, 0 of them carrying the flag) so an ordinary landing
	// does not move them; they fail a walk that collapsed.
	if files < 40 {
		t.Fatalf("the script walk read %d files — the corpus this arm censuses is not there", files)
	}
	if bdLines < 60 {
		t.Fatalf("the script walk found %d lines running bd at all, over %d files — a census that cannot see this repo's bd calls cannot say they carry no store flag", bdLines, files)
	}
	for _, h := range hits {
		t.Errorf("%s:%d hands bd a store-selecting flag: %q\n"+
			"  On bd 0.50.3 that auto-imports THIS repo's .beads/issues.jsonl into the named store (github issue 9, ranger-base-9mjxb).\n"+
			"  Name the repo that owns the store, chdir into it, and drop the flag — scripts/prune-bd-relates-to.sh's store_owner is the worked example, and it refuses rather than guessing when the owning repo cannot be named.",
			h.file, h.line, h.text)
	}

	// ── arm 2: non-test Go, as parsed string literals ──
	// 144 non-test Go files under the two roots, MEASURED the same day.
	found, parsed := bdfScanGo(t)
	if parsed < 100 {
		t.Fatalf("parsed %d non-test Go files under cmd/ and internal/ — the corpus this arm censuses is not there", parsed)
	}
	// Sorted, so a red names the same files in the same order on every box.
	var carriers []string
	for rel := range found {
		carriers = append(carriers, rel)
	}
	sort.Strings(carriers)
	for _, rel := range carriers {
		lines := found[rel]
		if _, ok := bdfGoLiteralRoster[rel]; ok {
			continue
		}
		t.Errorf("%s names bd's store-selecting flag as a string literal (line(s) %v) and is not on bdfGoLiteralRoster.\n"+
			"  If it BUILDS a bd argv: don't — Bd.runOnce binds the store with cmd.Dir and BEADS_DIR, which needs no flag (internal/posse/beads.go).\n"+
			"  If it only PARSES someone else's argv, add it to the roster with that reason.",
			rel, lines)
	}
	// And the roster's own rows have to be real. A roster entry for a file
	// that stopped carrying the literal is a row holding the door open for
	// nothing, and a roster read by a walk that found nothing would pass this
	// arm silently.
	var excused []string
	for rel := range bdfGoLiteralRoster {
		excused = append(excused, rel)
	}
	sort.Strings(excused)
	for _, rel := range excused {
		why := bdfGoLiteralRoster[rel]
		if len(found[rel]) == 0 {
			t.Errorf("bdfGoLiteralRoster excuses %s (%s) and no such literal is there any more — drop the row", rel, why)
		}
	}
	if len(found) == 0 {
		t.Error("no non-test Go file names the flag at all, and the roster says one should — the literal scan is reading nothing")
	}

	// ── the Go runner's own global flags, by name ──
	// The roster above cannot see a flag that arrives as a variable, and
	// bdGlobalFlags is the one slice every posse bd call is built from. It
	// lives in internal/posse, so this reads the source rather than the value.
	src, err := os.ReadFile(filepath.Join("internal", "posse", "beads.go"))
	if err != nil {
		t.Fatal(err)
	}
	decl := "var bdGlobalFlags = []string{\"--no-daemon\"}"
	if !strings.Contains(string(src), decl) {
		t.Errorf("internal/posse/beads.go no longer declares %q.\n"+
			"  Every posse bd call prepends bdGlobalFlags, so a store-selecting flag added there is one on every call in the process, against whatever store the flag names rather than the directory the caller asked for. If the slice legitimately changed, re-state this arm over what it is now.",
			decl)
	}
}
