//go:build !posse_arm2 && !posse_arm3

package posse

// QA pin for ranger-base-nnnf1 — every docs/notes.d fragment a markdown file
// in this tree cites is a file this tree holds.
//
// THE ESCAPE. v0.5.1 (32a90cde, 2026-10-04) shipped a CHANGELOG section
// calling ADR 0062's bob claims MEASURED, and the ADR cited its evidence by
// path — "evidence in `docs/notes.d/ranger-base-4mrmc.md`" — while that
// fragment sat on the branch that wrote it and was absent from main. Nothing
// was red: the citation is prose, the fragment's absence is a missing FILE,
// and no door read the two against each other. ranger-base-aza46 landed that
// fragment, and the same gap had already let an older one through —
// docs/notes.d/ranger-base-l1rjl.md leaned on a fragment ranger-base-fm23s
// never wrote in any branch, so the rule its verification section rested on
// was unreadable. That sentence is restated in l1rjl under this bead; from
// here on a reader following a pointer finds a file.
//
// WHY NEITHER NEIGHBOUR COVERS IT. internal/treepins/mergeblockedrunbook_qa_test.go
// is the nearest pin and is narrow on purpose (its own lines 11-16): a
// tree-wide version over GO source would need an exemption register first,
// because internal/posse fixtures name fragments no tree has. Markdown has no
// such fixture problem — MEASURED 2026-10-04 on this box by the walk below:
// 320 markdown files, 160 citations, one dangling, and that one is the l1rjl
// sentence. `make notes-check` is the other near miss: it asserts
// docs/notes.d/README.md lists every fragment that EXISTS, never that a cited
// fragment exists.
//
// NAMING A FRAGMENT THAT IS NOT THERE. There is no exemption register, because
// prose does not need one: name the BEAD and not the path. The two sentences
// in this tree that have to say ranger-base-fm23s wrote no fragment say it
// that way, and a register would be the place to park the next real escape.
//
// Doored as `make doc-check` ($(QA_DOC_PINS)) — a prose pin whose subject is a
// PATH, which TestQADetectionDoorCitesAFilingThatExists already made that
// door's business — and tree-wide, living in a package nobody runs whole
// (ranger-base-rulbl, ranger-base-ik44f).

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// qncCite matches a notes-fragment citation the way these documents write one:
// bare, in backticks, or inside a link. Anchored on the directory, because
// that is the part this pin promises and the part a rename moves.
var qncCite = regexp.MustCompile(`docs/notes\.d/[A-Za-z0-9._-]+\.md`)

// qncMarkdown returns every markdown file under root, relative and slashed.
// Build output and .git are skipped the way twdSourceFiles' fallback walk
// skips them. It takes the root as an argument for the same reason
// dddMissing does: a helper that found the repo root itself would be unusable
// from the control below.
func qncMarkdown(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "bin", "dist", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// qncHit is one citation: the file and line that wrote it, and the path it
// points at.
type qncHit struct {
	path string
	num  int
	cite string
}

// qncScan returns every citation in files, and the ones that do not resolve to
// a FILE under root. A directory is not a fragment: a check that accepted one
// would pass a tree where docs/notes.d/<bead>.md had become a directory of
// that name.
func qncScan(t *testing.T, root string, files []string) (all, dangling []qncHit) {
	t.Helper()
	for _, rel := range files {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(body), "\n") {
			for _, cite := range qncCite.FindAllString(line, -1) {
				h := qncHit{path: rel, num: i + 1, cite: cite}
				all = append(all, h)
				if st, err := os.Stat(filepath.Join(root, filepath.FromSlash(cite))); err != nil || st.IsDir() {
					dangling = append(dangling, h)
				}
			}
		}
	}
	return all, dangling
}

func TestQAEveryNotesFragmentCitationResolves(t *testing.T) {
	t.Parallel()
	root := qibRepoRoot(t)
	files := qncMarkdown(t, root)
	// A pin over absence is satisfied by reading nothing. Both floors are
	// well under what the tree holds (320 markdown files, 160 citations on
	// 2026-10-04), so they survive ordinary growth and fail a walk or a
	// scanner that has stopped matching.
	if len(files) < 100 {
		t.Fatalf("only %d markdown files found under %s — the walk this pin reads found nothing", len(files), root)
	}
	all, dangling := qncScan(t, root, files)
	if len(all) < 50 {
		t.Fatalf("only %d docs/notes.d citations read across %d markdown files — the scanner has stopped matching the way these documents write a citation", len(all), len(files))
	}
	t.Logf("read %d docs/notes.d citations across %d markdown files", len(all), len(files))
	for _, h := range dangling {
		t.Errorf("%s:%d cites %s, which is not a file in this tree — the evidence pointer resolves to nothing, which is how v0.5.1 was cut over an ADR citing a fragment stranded on a branch (ranger-base-nnnf1). Land the fragment, or name the bead rather than the path.", h.path, h.num, h.cite)
	}
}

// The control: the scanner and the existence check must both be able to say
// no. Without it this is a green function nobody has seen refuse anything — a
// citation regexp that matches nothing, or a stat that never fails, reads
// exactly like a tree whose every pointer resolves.
//
// It takes its own scratch root rather than the repo's, so it is not a
// tree-wide pin and needs no door of its own.
func TestQANotesCitationCheckCanFail(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const (
		here = "docs/notes.d/ranger-base-control-here.md"
		gone = "docs/notes.d/ranger-base-control-gone.md"
	)

	write := func(rel, body string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// The scanner, on the shapes these documents use and on a sentence that
	// names a bead rather than a path — which is the way out this pin leaves
	// open, so it has to stay a non-match.
	if got := qncCite.FindAllString("evidence in `"+gone+"` and again in "+here+".", -1); len(got) != 2 || got[0] != gone || got[1] != here {
		t.Errorf("the scanner does not read citations out of a sentence that writes two: %q", got)
	}
	if got := qncCite.FindAllString("ranger-base-fm23s is a merge-back bead that wrote no fragment", -1); len(got) != 0 {
		t.Errorf("the scanner invents a citation out of a bead id: %q", got)
	}

	// A tree where one pointer resolves and one does not, and the line
	// numbers are the ones a reader needs.
	write(here, "# present\n")
	write("docs/cites.md", "first line\nevidence in `"+here+"`\nand `"+gone+"` is not here\n")
	files := qncMarkdown(t, root)
	if len(files) != 2 {
		t.Fatalf("the walk found %v, want the two markdown files just written", files)
	}
	all, dangling := qncScan(t, root, files)
	if len(all) != 2 {
		t.Fatalf("the scan read %d citations, want 2: %v", len(all), all)
	}
	if len(dangling) != 1 || dangling[0].cite != gone {
		t.Fatalf("a fragment that is not in the tree reads as present: %v", dangling)
	}
	if got := dangling[0]; got.path != "docs/cites.md" || got.num != 3 {
		t.Errorf("the dangling citation is reported at %s:%d, want docs/cites.md:3 — a hit a reader cannot find is a hit they have to grep for", got.path, got.num)
	}

	// Land it, and the same scan goes quiet: the check is not simply always
	// red.
	write(gone, "# landed\n")
	if _, dangling = qncScan(t, root, qncMarkdown(t, root)); len(dangling) != 0 {
		t.Errorf("a fragment that IS in the tree reads as missing: %v", dangling)
	}

	// And a directory of that name is not a fragment.
	full := filepath.Join(root, filepath.FromSlash(gone))
	if err := os.Remove(full); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(full, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, dangling = qncScan(t, root, []string{"docs/cites.md"}); len(dangling) != 1 || dangling[0].cite != gone {
		t.Errorf("a directory reads as a fragment: %v", dangling)
	}
}
