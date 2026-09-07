//go:build !posse_arm2 && !posse_arm3

package posse

import (
	"os"
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

// assertNoExecBitOSWriteFile flags any os.WriteFile call on a line carrying
// an octal literal with an exec bit set. It parses each octal literal on the
// line and tests its exec bits by mask rather than enumerating known exec
// modes — an enumerated list is blind to owner-only exec bits (0o700, 0o500,
// 0o711, 0o744, 0o540, ...), which is exactly how gates.go:5555 survived the
// original census (ranger-base-0phqz).
func assertNoExecBitOSWriteFile(t *testing.T, file, src string) {
	t.Helper()
	octalLit := regexp.MustCompile(`0o[0-7]{3,4}`)
	for _, line := range strings.Split(src, "\n") {
		if !strings.Contains(line, "os.WriteFile(") {
			continue
		}
		for _, lit := range octalLit.FindAllString(line, -1) {
			n, err := strconv.ParseInt(strings.TrimPrefix(lit, "0o"), 8, 32)
			if err == nil && n&0o111 != 0 {
				t.Errorf("%s still writes an executable with plain os.WriteFile (ETXTBSY window, golang/go#22315): %s", file, strings.TrimSpace(line))
			}
		}
	}
}

// TestQATestFilesWriteExecutablesUnderTheForkLock widens the census above to
// every *_test.go file in this package (ranger-base-o6oj4). The sibling
// census above deliberately excluded test files (ranger-base-jaqnp's scope
// was non-test siblings found alongside gates.go); a repo-wide grep of
// internal/posse/*_test.go turned up ~150 more os.WriteFile(...,0o<exec-bit>)
// call sites across ~50 files — same golang/go#22315 window, fixed here by
// routing them through WriteExecutable, and guarded going forward by the
// sweep below.
//
// A test file's sheer count of sites rules out the per-site positive check
// the sibling census above uses (enumerating a want string per site does not
// scale to 150 of them, and a new test file adding one more exec-write site
// tomorrow would not be in the list to enumerate anyway). The negative sweep
// alone is the right shape here: any *_test.go file, present or future,
// that writes an octal literal with an exec bit through plain os.WriteFile
// fails this test.
func TestQATestFilesWriteExecutablesUnderTheForkLock(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(qibRepoRoot(t), "internal", "posse")
	matches, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("no *_test.go files found — glob or repo root is wrong")
	}
	for _, path := range matches {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		assertNoExecBitOSWriteFile(t, filepath.Base(path), string(b))
	}
}
