package posse

// ADR 0016's fourth done-when row, for ranger-base-4dxpo: no event
// subscription actor or state remains, and the watch loop's early wake is
// the LOCAL one.
//
// A removal needs a pin for the same reason a fix does. The mechanism was
// removed because it was measured not to work on this shop — over one live
// 10-minute window, a single un-redialled connection saw 23 settles while
// the production adapter delivered 0, because `pane.agent_detected` arrives
// faster than a dial completes and every one of them is a planned reconnect
// (599 of them in that window). Nothing in a green suite says that, so
// nothing in a green suite would stop it being rebuilt; this is what does.
//
// It reads THIS directory and cockpit.go's own package reads its own, rather
// than walking from the repo root: a tree-wide pin is nobody's `-run`
// subject and needs a Makefile door (treewidedoor_qa_test.go), and the two
// packages that held the mechanism are the two that can grow it back.
//
// MUTATION, each restored: name any banned symbol in a non-test file here
// and the census arm reds naming path, line and symbol; delete the
// `case <-d.settled()` arm from Watch's timer select and the wake arm reds
// (and TestQACarriedSettleWakesTheNextPassAtOnce reds behaviourally, which
// is the pair this pin is the cheap half of).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// hintVocabulary is the removed mechanism, spelled as the identifiers and
// wire strings that only it ever had. Each entry is a substring; a name that
// is a substring of a surviving one is not listed, so a hit is a hit.
var hintVocabulary = []string{
	"HerdrHint",
	"HerdrSettleHints",
	"HerdrAllHints",
	"herdrHints(",
	"streamHerdrHints",
	"herdrSubscriptions",
	"herdrEventEnvelope",
	"HerdrLifecycleSubscriptions",
	"HerdrPaneSubscription",
	"herdrRedialFloor",
	"herdrHintRetry",
	"AgentPanes",
	"events.subscribe",
	"pane.agent_status_changed",
	"pane.agent_detected",
	"workspace.created",
}

func TestQANoEventSubscriptionRemainsInInternalPosse(t *testing.T) {
	t.Parallel()
	assertNoHintVocabulary(t, ".")
}

// assertNoHintVocabulary sweeps one directory's non-test .go files. Test
// files are excluded because THIS file names every banned symbol, and a
// census that could not name its own subject would have to spell it in
// pieces — which is how a census stops being readable and starts being
// wrong.
func assertNoHintVocabulary(t *testing.T, dir string) {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, e := range ents {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			for _, bad := range hintVocabulary {
				if strings.Contains(line, bad) {
					found++
					t.Errorf("%s:%d names %q — ADR 0016 removed the herdr event subscription, and a "+
						"replacement notification seam is a decision for the architect, not a diff:\n\t%s",
						filepath.Join(dir, name), i+1, bad, strings.TrimSpace(line))
				}
			}
		}
	}
	if found == 0 && len(hintVocabulary) == 0 {
		t.Fatal("the vocabulary is empty, so this census can only ever pass")
	}
}

// Watch's early wake is the carry's local completion and nothing else. The
// behavioural pin is TestQACarriedSettleWakesTheNextPassAtOnce; this one owns
// the other half of ADR 0016's sentence — that the wake has no SECOND source
// that could go down separately from the loop.
func TestQAWatchTimerSelectWakesOnlyOnTheLocalCarry(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("watch.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{"case <-d.settled():", "case <-timer.C:", "case <-ctx.Done():"} {
		if !strings.Contains(src, want) {
			t.Errorf("watch.go no longer has `%s` in its timer select — the loop lost either its early "+
				"wake, its backstop tick or its cancellation, and ADR 0016 keeps all three", want)
		}
	}
	// Backoff, cadence floor and cancellation, the three ADR 0016 keeps by
	// name where it removed the socket beside them.
	for _, want := range []string{"NextInterval(wait, base, maxInterval,", "d.GatherWindow = base"} {
		if !strings.Contains(src, want) {
			t.Errorf("watch.go no longer carries %q — ADR 0016 forbids compensating for the removed "+
				"optimization by changing the reconciliation cadence", want)
		}
	}
}
