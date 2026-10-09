//go:build !posse_arm2 && !posse_arm3

package posse

// QA pin for ranger-base-qbbzz, closing a coverage hole ranger-base-x8bv0's
// close left.
//
// THE HOLE. ranger-base-x8bv0 named three citation sites and fixed all three:
// DetectionDoor's built-in no-filing arm (detection.go:447), its
// DECLARED-runtime arm (detection.go:453) and the probe's agent_not_found
// observable (runtimeprobe.go:561). Two of the three got a pin. The third did
// not, and the pin that was supposed to hold it says it does:
// detectiondoor_qa_test.go's "Every surface that prints the door, so none of
// them can carry a path the others do not" sits inside `for i := range
// builtinRuntimes`, so every sentence it classifies comes from the BUILT-IN
// arm. Its "WHAT THIS PIN DOES NOT REACH" header names only a bare path
// written outside a DetectionDoor surface — not the declared half of
// DetectionDoor itself.
//
// MEASURED 2026-10-09: reverting detection.go:453 to the bare
// `detectionDocPath` and running ./internal/posse under a
// `(Detection|Door|Probe|Filing|Runtime|Preflight)` filter answers `ok` on all
// three arms (53.5s / 38.4s / 42.3s). The one test that renders the declared
// arm — detection_qa_test.go's
// TestQAUndetectableDoorIsTheFilingForABuiltinAndTheRunbookForADeclaredOne —
// asserts the line CONTAINS `"docs/runbooks/" + detectionDoc`, which the
// composed URL and the bare path both satisfy, so it was green before the fix
// and is green after it: a true assertion about WHICH door, blind to how the
// reader reaches it.
//
// This closes that half, and it is the declared arm's whole exposure: the
// page's existence in the tree is already held, once, by the const-level
// reading at the head of TestQADetectionDoorCitesAFilingThatExists, which
// stats `detectionFilingDoc` and `detectionDocPath` before any sentence is
// rendered. What was missing was the classification — URL or bare path — of
// the sentences the DECLARED arm prints.
//
// Deliberately NOT tree-wide: it renders real functions over a declared
// runtime written into a scratch App and reads nothing from the repo, so it
// needs no Makefile door (ranger-base-rulbl's class is a reading whose
// subject is the tree). The three couplings that DO need a tree-reading pin
// and a door are filed separately.

import (
	"strings"
	"testing"
)

// The declared-runtime half of x8bv0's fix, over every surface that prints
// the door. A declared runtime's author is told to write a manifest or alias
// their CLI onto one that exists, and that sentence is read wherever posse
// was installed from — a brew keg carries `posse`, README.md and INSTALL.md
// and no `docs/` at all, so a repo-relative path there is the dead end
// github issue #4 was filed for.
func TestQADetectionDoorForADeclaredRuntimeCitesAPageAsAURL(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	rt := writeRuntime(t, a, "mycli", "command: mycli {file}\n")
	// CONTROL: a DECLARED runtime, or this is the built-in arm the other pin
	// already holds and the hole stays open underneath a green test.
	if rt.Builtin {
		t.Fatalf("CONTROL: mycli loaded as a built-in, so this pin is reading the wrong arm of DetectionDoor")
	}

	r := ManifestReading{Argv0: rt.Exe(), State: ManifestUnknownAgent}
	for surface, line := range map[string]string{
		"DetectionDoor":          DetectionDoor(rt),
		"DetectionRefusal":       DetectionRefusal(rt, r).Error(),
		"DetectionRetypeRefusal": DetectionRetypeRefusal(rt, r, "s1").Error(),
		"DetectionGapLine":       DetectionGapLine(rt, r),
		"DetectionDegraded":      DetectionDegraded(rt, r),
	} {
		urls, bare := dddSplitCites(line)
		// The positive control FIRST, in the same arm: an absence asserted
		// over a sentence the classifier never read is true of nothing, and
		// a door that stopped citing the page at all would otherwise pass
		// here by citing nothing (ORDERS, "a check whose only signal is
		// output passes by producing none").
		if len(urls) != 1 || urls[0] != detectionDocPath {
			t.Errorf("%s on a declared runtime must cite %s exactly once, as a URL under publicDocBase — got urls=%q bare=%q:\n%s",
				surface, detectionDocPath, urls, bare, line)
			continue
		}
		if len(bare) != 0 {
			t.Errorf("%s on a declared runtime hands the author %q as a bare repo-relative path, and this sentence is read on whatever box posse was installed onto — a release carries no docs/ at all, so cite it as %s (ranger-base-x8bv0, github issue #4):\n%s",
				surface, bare, publicDoc(detectionDocPath), line)
		}
		if !strings.Contains(line, publicDoc(detectionDocPath)) {
			t.Errorf("%s on a declared runtime does not carry %q verbatim — the classifier read a citation under the base and the sentence does not spell it, so the two readings are of different strings:\n%s",
				surface, publicDoc(detectionDocPath), line)
		}
	}
}
