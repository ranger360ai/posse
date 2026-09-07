//go:build !posse_arm2 && !posse_arm3

package posse

import (
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
	execModes := []string{"0o755", "0o750", "0o751", "0o770", "0o771", "0o775"}
	for file, src := range srcByFile {
		for _, line := range strings.Split(src, "\n") {
			if !strings.Contains(line, "os.WriteFile(") {
				continue
			}
			for _, mode := range execModes {
				if strings.Contains(line, mode) {
					t.Errorf("%s still writes an executable with plain os.WriteFile (ETXTBSY window, golang/go#22315): %s", file, strings.TrimSpace(line))
				}
			}
		}
	}
}
