//go:build !posse_arm2 && !posse_arm3

package posse

// Verifying ranger-base-hgzv's close (ranger-base-coxn8).
//
// hgzv swept the dead "stale leftover of a keychain login" characterization
// out of the code strings that lagged ADR 0019 as amended (ranger-base-1lza:
// on darwin ~/.claude/.credentials.json is a RECURRING UNOWNED BYPRODUCT,
// D2 store 3, not a leftover of anything). The sweep landed in 26b21af and
// the two sites the bead named are correct. Nothing pins them: the phrase
// can walk back in with the next comment somebody writes about that file,
// and a comment is exactly the kind of string no test looks at.
//
// So this is the guard the sweep did not get. It scans the shipped .go
// sources — not _test.go, which is where this very file has to spell the
// dead phrase out — and it carries a positive witness, because a scanner
// that reads nothing agrees with every claim you make about what it found.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The characterization ADR 0019's amendment retired. Written as pieces so
// the guard's own source is not a hit for the thing it forbids when some
// later scanner is pointed at a wider set than this one.
var coxnDeadFraming = "stale " + "leftover"

// The amended wording, present in credential.go since 26b21af: the witness
// that the walk below actually opened files and read their bytes. Without
// it "no file says the dead phrase" is equally true of a walk that visited
// nothing (pass-count-is-not-a-coverage-floor).
var coxnAmendedWording = "recurring unowned byproduct"

// coxnGoSources walks the repo for shipped .go files — every package, not
// just this one, because the phrase travelled: the bead named credential.go
// and cage.go, and the closer found the same framing in runtimecheck.go on
// the way past.
func coxnGoSources(t *testing.T) map[string]string {
	t.Helper()
	root := qibRepoRoot(t)
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "vendor" || d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		out[rel] = string(b)
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	return out
}

func TestQANoCodeStringCallsTheDarwinCredentialsFileAStaleLeftover(t *testing.T) {
	t.Parallel()
	srcs := coxnGoSources(t)
	if len(srcs) < 50 {
		t.Fatalf("the walk found %d shipped .go files — too few to be the tree, so the absence below measures nothing", len(srcs))
	}
	var witness bool
	for _, body := range srcs {
		if strings.Contains(body, coxnAmendedWording) {
			witness = true
			break
		}
	}
	if !witness {
		t.Fatalf("no shipped source carries %q — the walk read %d files and found none of the wording 26b21af landed, so it is not reading what it thinks it is", coxnAmendedWording, len(srcs))
	}
	for name, body := range srcs {
		if !strings.Contains(body, coxnDeadFraming) {
			continue
		}
		for i, line := range strings.Split(body, "\n") {
			if strings.Contains(line, coxnDeadFraming) {
				t.Errorf("%s:%d says %q — ADR 0019 was amended (ranger-base-1lza): on darwin the file is a recurring unowned byproduct that is not the store of record and posse never reads it, never a leftover of a keychain login:\n  %s",
					name, i+1, coxnDeadFraming, strings.TrimSpace(line))
			}
		}
	}
}

// The site the sweep missed, filed as ranger-base-d14ie: runtime.go's
// CageCred doc carried the pre-amendment claim in the container's words —
// "the on-disk credential files are stale there or unrefreshable read-only"
// — which is verbatim what 26b21af reworded one file over, at
// runtimecheck.go:557. It was parked rather than red while the fix belonged
// to the code lane; d14ie was closed as folded into ranger-base-wd4be and
// wd4be's commit reworded the site to its twin's sentence, so the park is
// lifted and this is a live guard against the framing coming back to the
// one file the "stale leftover" scan above cannot see it in — the phrase
// here is the container's, not that one.
//
// The sibling above scans every shipped source for the dead phrase; this
// one names a file, because the wording it wants is a rewording of a
// SPECIFIC doc and its absence anywhere else proves nothing.
func TestQACageCredDocDoesNotCallTheOnDiskCredentialStale(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(filepath.Join(qibRepoRoot(t), "internal", "posse", "runtime.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "stale there or unrefreshable") {
		t.Error("runtime.go's CageCred doc still characterizes the on-disk credential file as stale — the amended model (ADR 0019 D2 store 3) is runtimecheck.go's wording: posse never reads an on-disk credential file there either")
	}
}
