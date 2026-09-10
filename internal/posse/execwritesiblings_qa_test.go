//go:build !posse_arm2 && !posse_arm3

package posse

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestQASiblingsWriteExecutablesUnderTheForkLock guards ranger-base-jaqnp's
// fix: five call sites, found while scoping ranger-base-ntsvf's gates.go fix,
// wrote a script or binary with an exec bit (0o755) via plain os.WriteFile
// instead of WriteExecutable. That leaves the golang/go#22315 window open
// (execwrite.go's doc comment): a concurrent fork elsewhere in the same
// process can land between the write's open and close and inherit the write
// descriptor, and a later exec of that same file answers ETXTBSY while the
// inherited descriptor is still open. gates.go hit this for real under
// ntsvf; these five call sites are the same class, latent rather than
// observed, in cage.go, cageinner.go, hooksredirect.go (two sites) and
// runtimeprobe.go.
//
// This is a source census, not a timing race, for the same reason
// execwritegates_qa_test.go is: the race itself is covered — deeply — by
// execwrite_test.go. What a regression here needs is proof these five call
// sites still route through WriteExecutable, not a second timing rig.
//
// gates.go is deliberately NOT part of this census: ranger-base-ntsvf owns
// that file's fix on its own branch, unmerged as of this bead, and a census
// that reached into it here would red on work this bead is explicitly out
// of scope for (strict-scope: file adjacent problems, never fix them in
// passing — and this one already has its own fix in flight elsewhere).
func TestQASiblingsWriteExecutablesUnderTheForkLock(t *testing.T) {
	t.Parallel()

	type site struct {
		file string
		want string
	}
	sites := []site{
		{"cage.go", `WriteExecutable(filepath.Join(bin, "bd"), b, 0o755)`},
		{"cageinner.go", `WriteExecutable(path, []byte(script), 0o755)`},
		{"hooksredirect.go", `WriteExecutable(filepath.Join(hooks, name), []byte(members[slot]), 0o755)`},
		{"hooksredirect.go", `WriteExecutable(filepath.Join(hooks, slot), []byte(redirectDispatcher(slot, m.Dir, members[slot] != "")), 0o755)`},
		{"runtimeprobe.go", `WriteExecutable(script, []byte("#!/bin/sh\n# posse runtime probe`},
	}

	srcByFile := map[string]string{}
	for _, s := range sites {
		if _, ok := srcByFile[s.file]; !ok {
			srcByFile[s.file] = i9dbbRead(t, "internal", "posse", s.file)
		}
		if !strings.Contains(srcByFile[s.file], s.want) {
			t.Errorf("%s must write this executable through WriteExecutable, not os.WriteFile: %q not found", s.file, s.want)
		}
	}

	// The other direction: no exec-bit os.WriteFile call left behind in
	// these same files for a future edit to reintroduce, which a pure
	// substring-presence check above would miss entirely.
	for file, src := range srcByFile {
		assertNoExecBitOSWriteFile(t, file, src)
	}
}

// assertNoExecBitOSWriteFile reports every line of src that writes an
// executable with a plain os.WriteFile, naming the file it came from.
func assertNoExecBitOSWriteFile(t *testing.T, file, src string) {
	t.Helper()
	for _, line := range execBitOSWriteFileLines(src) {
		t.Errorf("%s still writes an executable with plain os.WriteFile (ETXTBSY window, golang/go#22315): %s", file, line)
	}
}

// execBitOSWriteFileLines returns the trimmed lines of src that call
// os.WriteFile with an octal literal carrying an exec bit. It parses each
// octal literal on the line and tests its exec bits by MASK rather than
// enumerating known exec modes — an enumerated list is blind to owner-only
// exec bits (0o700, 0o500, 0o711, 0o744, 0o540, ...), which is exactly how
// gates.go:5555 survived the original census (ranger-base-0phqz).
//
// Split out of the assertion above so the matcher can be driven directly by
// TestQAExecWriteCensusCanStillSayNo. The census below is an ABSENCE claim
// over ~700 files, and an absence pin whose matcher has quietly stopped
// matching is green for the same reason a clean tree is (ORDERS: "absence
// claims need the whole surface measured and a positive witness").
func execBitOSWriteFileLines(src string) []string {
	octalLit := regexp.MustCompile(`0o[0-7]{3,4}`)
	var out []string
	for _, line := range strings.Split(src, "\n") {
		if !strings.Contains(line, "os.WriteFile(") {
			continue
		}
		for _, lit := range octalLit.FindAllString(line, -1) {
			n, err := strconv.ParseInt(strings.TrimPrefix(lit, "0o"), 8, 32)
			if err == nil && n&0o111 != 0 {
				out = append(out, strings.TrimSpace(line))
				break
			}
		}
	}
	return out
}

// execwriteSweepFloors are the package directories the sweep below must
// actually reach, with a floor on each. A sweep is satisfied by sweeping
// nothing: the glob this replaced named ONE directory, and had it been
// pointed at a directory that no longer existed it would have reported a
// clean tree it never opened. The floors are well under the counts on
// 2026-09-10 (root 56, cmd/posse 50, internal/posse 584) — they are here to
// catch a walk that collapsed, not to pin a file count that moves daily.
//
// The three named here are the three that matter for provenance: root and
// cmd/posse are ranger-base-6cznr's subject, internal/posse is what the pin
// covered before it. The walk itself is not limited to them — it is the
// whole repository, so cmd/buildstamp, cmd/checkorphans and cmd/testparallel
// are swept too, and a package added tomorrow is swept the day it lands
// without anybody remembering to widen a list.
var execwriteSweepFloors = []struct {
	dir   string
	floor int
}{
	{".", 40},
	{"cmd/posse", 30},
	{"internal/posse", 400},
}

// TestQATreeGoFilesWriteExecutablesUnderTheForkLock widens the census above
// to every .go file in the repository.
//
// It started (ranger-base-o6oj4) as a sweep of internal/posse/*_test.go: the
// sibling census above deliberately excluded test files, and a repo-wide
// grep of that one directory turned up ~150 exec-bit os.WriteFile call sites
// across ~50 files — same golang/go#22315 window, fixed there by routing
// them through WriteExecutable and guarded going forward by this sweep.
//
// ranger-base-c2er3 then converted the 46 sites OUTSIDE internal/posse — the
// root package (through a package-local WriteExecutable in execwrite_test.go,
// because internal/posse's init.go imports the root package and Go refuses
// the cycle back) and cmd/posse (through posse.WriteExecutable, no cycle
// there). That left the tree at zero everywhere and this door still reading
// one directory, so a new exec-bit os.WriteFile in either place regrew the
// class undetected. ranger-base-6cznr is that gap: the walk is now the whole
// repository, .git and vendor aside, and it covers PRODUCTION files as well
// as test files — the sibling census above pins five named production sites
// positively, and nothing guarded the rest.
//
// A test file's sheer count of sites rules out the per-site positive check
// the sibling census uses (enumerating a want string per site does not scale
// to 150 of them, and a file adding one more site tomorrow would not be in
// the list to enumerate anyway). The negative sweep alone is the right shape
// here: any .go file, present or future, that writes an octal literal with
// an exec bit through plain os.WriteFile fails this test.
//
// What it does NOT see, deliberately, is a mode that is not a literal —
// execwrite.go's own WriteExecutable ends in a plain os.WriteFile with a
// `perm` parameter, and treewidedoor_qa_test.go copies a tree with
// `info.Mode().Perm()`. Both are correct, neither carries an octal literal,
// and a rule that flagged them would have to understand where the value came
// from. That is a different pin, and the ETXTBSY window this one is about
// arrives as a hand-typed 0o755.
func TestQATreeGoFilesWriteExecutablesUnderTheForkLock(t *testing.T) {
	t.Parallel()

	root := qibRepoRoot(t)
	perDir := map[string]int{}
	total := 0
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		perDir[path.Dir(rel)]++
		total++
		// The rel path, not the base name: the tree carries two
		// configdirfence_test.go (root and cmd/posse) and a base-name
		// message would send a seat to the wrong one.
		assertNoExecBitOSWriteFile(t, rel, string(b))
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}

	if total < 500 {
		t.Fatalf("the walk found %d .go files under %s — too few to be this tree, so the absence above measures nothing", total, root)
	}
	for _, want := range execwriteSweepFloors {
		if perDir[want.dir] < want.floor {
			t.Errorf("the walk read %d .go files in %s, want at least %d — that package is not being swept, and an exec-bit os.WriteFile landing there would pass in silence", perDir[want.dir], want.dir, want.floor)
		}
	}
}

// TestQAExecWriteCensusCanStillSayNo drives the matcher the census above
// rests on. The census is an absence claim, and the cheapest way for an
// absence claim to go green is for its matcher to stop matching — so this
// asks the matcher, on synthetic lines, for both answers.
//
// The offending call is ASSEMBLED rather than spelled, for the same reason
// treewidedoor_qa_test.go assembles its drift probes: this file is inside
// the walk above, and a fixture spelled in full would make the census a hit
// on its own source.
//
// It reads no file, so it is not a tree-wide pin and needs no Makefile door
// (treewidedoor_qa_test.go arm 2 reds on a door variable naming a test that
// does not read the tree).
func TestQAExecWriteCensusCanStillSayNo(t *testing.T) {
	t.Parallel()

	call := "os.Write" + "File("
	cases := []struct {
		line string
		want bool
	}{
		{"\tif err := " + call + "p, b, 0o755); err != nil {", true},
		// The owner-only shape gates.go:5555 hid behind: an enumerated list
		// of "known exec modes" misses it, the mask does not.
		{"\tif err := " + call + "p, b, 0o700); err != nil {", true},
		{"\tif err := " + call + "p, b, 0o544); err != nil {", true},
		// Data, which os.WriteFile is still the right call for.
		{"\tif err := " + call + "p, b, 0o644); err != nil {", false},
		{"\tif err := " + call + "p, b, 0o600); err != nil {", false},
		// The fix this pin exists to hold people to.
		{"\tif err := WriteExecutable(p, b, 0o755); err != nil {", false},
		// A non-literal mode: out of scope on purpose, see the census above.
		{"\treturn " + call + "path, content, perm)", false},
		// An exec-bit literal with no os.WriteFile on the line at all.
		{"\tif err := os.Chmod(p, 0o755); err != nil {", false},
	}
	for _, c := range cases {
		got := len(execBitOSWriteFileLines(c.line)) == 1
		if got != c.want {
			t.Errorf("execBitOSWriteFileLines(%q) matched=%v, want %v", c.line, got, c.want)
		}
	}

	// And the positive witness the census itself cannot carry: the matcher
	// finds a planted site in a body the size of a real file, not only on a
	// line handed to it alone.
	body := "package posse\n\nfunc plant() {\n" + cases[0].line + "\n}\n"
	if hits := execBitOSWriteFileLines(body); len(hits) != 1 {
		t.Errorf("the matcher found %d hits in a planted source file, want 1: %v", len(hits), hits)
	}
}
