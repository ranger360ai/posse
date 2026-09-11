//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-k45gn, verifying the close of ranger-base-ub2x9.
//
// ub2x9 bound every bd child to the store of the directory its caller named:
// bdStoreEnv drops an inherited `BEADS_DIR` and sets beadsHome(dir). The
// done-when was "so an inherited environment cannot redirect a query", and
// `BEADS_DIR` is not the only environment variable that redirects one.
//
// MEASURED 2026-09-10 against the pinned bd 0.50.3, production argv
// (`--no-daemon`), from a git root holding no `.beads`, each arm with a
// baseline and a control:
//
//	BEADS_DIR unset, no store           -> "Error: no beads database found"   (baseline)
//	BEADS_DB=<another repo>/beads.db    -> that repo's rows                   (REDIRECTS)
//	BEADS_DB=<a corrupt file>           -> "sqlite3: file is not a database"  (path honoured)
//	BEADS_JSONL=<another repo>/*.jsonl  -> baseline error                     (inert)
//	--db <another repo>/beads.db        -> that repo's rows                   (flag control)
//
// So an inherited `BEADS_DB` survives bdStoreEnv's rebuild and points the
// child at another repository's store, which ReadyAll then labels
// `RepoIssue{Dir: dir}` with the directory it meant to query — ub2x9's own
// masquerade, one spelling over. This tree already knows the precedence:
// bddepsafety_qa_test.go says "BEADS_DB wins over discovery, from any cwd"
// and both it and bdrelatesprune_qa_test.go strip the variable from their
// child environments for exactly this reason.
//
// The hole is DEBT rather than a live defect for one reason only: no Go
// file posse ships ever puts `BEADS_DB` into a child environment, so the
// value can only arrive from outside the process tree. THAT is what this pin
// holds. It is deliberately not a pin on bdStoreEnv's shed list — widening
// that list is the code lane's fix and would move such a pin — but on the
// reason the gap is currently harmless. The day a shipped Go file builds one
// of these into a child env, this reds and names it, and the failure message
// spells the fix.
//
// SCOPE, and why it is Go only. bdStoreEnv governs exactly one thing: the
// environment runOnce hands the bd children posse launches. A shell script
// that runs bd itself never passes through it. `scripts/prune-bd-relates-to.sh`
// and `scripts/verify-bd-dep-safety.sh` both READ `${BEADS_DB:-}` as their
// own documented operator input, which is correct and is not this claim —
// censusing them would red on the day someone documents a variable.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bdRedirectingEnvVars are the environment variables MEASURED to repoint bd
// 0.50.3's store, minus the one bdStoreEnv already sheds. A shipped file
// that sets any of them reintroduces ranger-base-ub2x9 through a spelling
// that runner does not clear.
var bdRedirectingEnvVars = []string{"BEADS_DB"}

func TestQANoShippedFileSetsAStoreVarBdStoreEnvDoesNotShed(t *testing.T) {
	t.Parallel()
	root := k45gnRepoRoot(t)
	var hits []string
	for _, dir := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			// Test files are exempt BY DESIGN: two of them strip these
			// variables on purpose, and one asserts the precedence this pin
			// is derived from. The claim is about what posse SHIPS.
			if strings.HasSuffix(p, "_test.go") {
				return nil
			}
			if filepath.Ext(p) != ".go" {
				return nil
			}
			b, rerr := os.ReadFile(p)
			if rerr != nil {
				return rerr
			}
			for _, v := range bdRedirectingEnvVars {
				// The child-env spelling and only that: a Go string literal
				// opening `"BEADS_DB=`, which is how every env row in this
				// tree is built (beads.go bdStoreEnv, cage.go's carry list).
				// A bare mention in a comment must not red this pin — the
				// one above names the variable a dozen times.
				if strings.Contains(string(b), `"`+v+`=`) {
					rel, _ := filepath.Rel(root, p)
					hits = append(hits, rel+" builds "+v+" into a child environment")
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(hits) > 0 {
		t.Errorf(`a shipped Go file now builds an environment variable that repoints bd's
store and that bdStoreEnv (beads.go) does not shed:

  %s

That is ranger-base-ub2x9 through a second spelling: bdStoreEnv drops only
BEADS_DIR, so this value is inherited by every bd child runOnce launches with
a caller-named directory, and ReadyAll labels the rows it returns with the
directory it MEANT to query. The fix is in bdStoreEnv: shed every variable in
bdRedirectingEnvVars beside BEADS_DIR. (MEASURED on bd 0.50.3: BEADS_DB
redirects; BEADS_JSONL does not.)`, strings.Join(hits, "\n  "))
	}
	// LIVENESS. A walk that read nothing is satisfied by sweeping nothing,
	// so prove the corpus was actually reached before believing the zero.
	// 122 shipped .go files at 2026-09-10; a floor well under it so an
	// ordinary deletion does not red this, but a walk that landed in the
	// package directory (0) or on the wrong tree does.
	if n := k45gnCountScanned(t, root); n < 80 {
		t.Errorf("the walk read %d shipped .go files under cmd/ and internal/, want at least 80 — it is reading the wrong tree and the census above measured nothing", n)
	}
}

func k45gnRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if fi, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !fi.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test working directory")
		}
		dir = parent
	}
}

func k45gnCountScanned(t *testing.T, root string) int {
	t.Helper()
	n := 0
	for _, dir := range []string{"cmd", "internal"} {
		if err := filepath.WalkDir(filepath.Join(root, dir), func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || strings.HasSuffix(p, "_test.go") {
				return err
			}
			if filepath.Ext(p) == ".go" {
				n++
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	return n
}
