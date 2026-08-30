package posse

// ranger-base-zgek (QA, verifying rangerhq-fl0h's close).
//
// TestNoShippedScriptAcquiresACredential walks an ALLOW-LIST of directories.
// That is the right shape — a walk of the whole tree would read build output
// and scratch files — but a list is a thing that goes stale silently, and it
// had already gone stale when this was written: `docs/adr/*.probe.sh` are
// shipped, executable, carry a shebang, and sat outside the walk. None of
// them touches a credential today, so nothing leaked; the pin simply could
// not have seen it if one did, and NOTES.md's D3 sentence says "any code
// path", not "any code path under scripts/".
//
// So this is the pin on the pin: every runnable file posse ships is inside
// the scan. When the answer changes the fix is one of two lines — move the
// file, or add its directory to cgScannedDirs — and either way somebody
// looks, which is the whole point.

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEveryShippedRunnableFileIsInsideTheCredentialScan(t *testing.T) {
	// The shipped tree is what git tracks, not what the working directory
	// happens to hold: a scratch script a session left behind is not
	// something posse ships, and must not red this pin.
	out, err := exec.Command("git", "ls-files", "-z").Output()
	if err != nil {
		t.Fatalf("git ls-files: %v — this pin needs the checkout to name what posse ships", err)
	}
	tracked := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	if len(tracked) < 100 {
		t.Fatalf("git tracks %d files — the listing failed, so a clean result here is not evidence", len(tracked))
	}

	covered := func(rel string) bool {
		if rel == "Makefile" {
			return true // scanned on its own, beside the dirs
		}
		for _, d := range cgScannedDirs {
			if rel == d || strings.HasPrefix(rel, d+"/") {
				return true
			}
		}
		return false
	}

	var runnable, outside int
	var missed []string
	for _, rel := range tracked {
		if rel == "" || !cgRunnable(filepath.FromSlash(rel)) {
			continue
		}
		runnable++
		if !covered(rel) {
			outside++
			missed = append(missed, rel)
		}
	}
	// The floor is the positive witness: cgRunnable answering "no" to
	// everything would leave `missed` empty and this test green over a scan
	// that covers nothing.
	if runnable < 18 {
		t.Fatalf("only %d runnable files in the whole tracked tree — cgRunnable is not "+
			"recognising them, so the credential scan is measuring far less than it looks", runnable)
	}
	if outside > 0 {
		t.Errorf("%d shipped runnable file(s) are outside the credential scan:\n  %s\n\n"+
			"TestNoShippedScriptAcquiresACredential can never see a credential read in\n"+
			"these, and NOTES.md \"Env sets and secrets\" tells the operator that every\n"+
			"code path reading secrets/, envs/ or the keychain ships through `make\n"+
			"install` (ADR 0019 D3). Either move the file under a scanned directory or\n"+
			"add its directory to cgScannedDirs — do not delete this test.",
			outside, strings.Join(missed, "\n  "))
	}
}
