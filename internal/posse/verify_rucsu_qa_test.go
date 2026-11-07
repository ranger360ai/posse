//go:build !posse_arm2 && !posse_arm3

package posse

// THE PIN FOR codexAskOver's cwd COMPARE. It began as a live-defect pin that
// shipped GREEN asserting a hole (found verifying ranger-base-fwwl8 under
// verify bead ranger-base-rucsu) and it is now the right way round:
// ranger-base-5x8xi landed the guard and INVERTED the two hole arms, which is
// the mechanism the pin was written for. The controls are untouched, and they
// are the half that says the compare landed in the right place rather than
// refusing everything.
//
// WHAT IT PINS. D7's ask sends `hooks/list {cwds:[<the seat's cwd>]}` and
// reads `result.data[]` back, one entry per asked cwd, each carrying its own
// `cwd` beside its `hooks[]`. posse asks about exactly one directory, so an
// entry naming another — or naming none — is an inventory of somebody else's
// seat, and before the guard it rendered a full trust line that a launch
// would have typed.
//
// WHY THAT IS THE SHAPE THIS FILE EXISTS FOR. codexHooksState refuses the
// sibling contradiction in codex's answer — the same hook key twice — and its
// refusal says why in words that cover this one exactly: "posse asks about
// ONE cwd, so this is not two rows about two seats". codexAskOver refuses an
// answer about no directory at all (`data[]` empty). Between those two
// guards sat an answer about one directory that is not the seat's, and
// nothing read it. What it cost is what every refusal in codextrust.go is
// written to prevent: the hooks discovered in the seat's OWN project are
// absent from the table, so they stay untrusted, codex draws the startup
// review dialog, and the seat submits nothing and reports nothing.
//
// WHY A BYTE COMPARE IS SAFE, which is the reading this pin could not take
// and a fixer had to: MEASURED 2026-11-07, codex-cli 0.153.4 on darwin
// 25.6.0, `hooks/list` echoes the asked path back VERBATIM — no
// normalization at all. A trailing slash survives, an uncollapsed `..`
// survives, a symlinked path is not resolved, and the echo is identical over
// an answer with no hooks, one with a discovered hook, and one whose
// hooks.json codex could not parse. So equality cannot refuse a launch that
// was fine. docs/notes.d/ranger-base-5x8xi.md has the six readings; the
// fixtures below still echo whatever they are handed, which is why they can
// render the shapes codex does not.

import (
	"strings"
	"testing"
)

func TestQACodexAskVetsTheDirectoryItsAnswerIsAbout(t *testing.T) {
	t.Setenv(CodexHomeVar, "")

	// THE CONTROL FIRST, so an arm below that passes cannot be passing
	// because the fixture never worked: the same fake, answering about the
	// directory it was asked about, renders a trust line.
	t.Run("control: the answer is about the seat's own cwd", func(t *testing.T) {
		cwd := ctCwd(t)
		bin, _ := ctFake(t, ctTalk(ctListReply(cwd, ctOurs(t)), ""))
		flags, err := ctAsk(bin, cwd).Flags()
		if err != nil {
			t.Fatalf("the fixture cannot render a trust line at all, so the arms below measure nothing: %v", err)
		}
		if !strings.HasPrefix(flags.State, "hooks.state={") {
			t.Fatalf("the control rendered %q, which is not a trust value", flags.State)
		}
	})

	// ARM 1: an answer about another seat's directory refuses.
	t.Run("an answer about another directory refuses", func(t *testing.T) {
		cwd := ctCwd(t)
		bin, _ := ctFake(t, ctTalk(ctListReply("/not/the/seats/directory", ctOurs(t)), ""))
		flags, err := ctAsk(bin, cwd).Flags()
		if err == nil {
			t.Fatalf("asking about %s and being answered about /not/the/seats/directory rendered %q. That is an inventory of a DIFFERENT seat: the hooks this seat's own project declares are not in it, so they would stay untrusted, codex would draw the startup review dialog, and the seat would submit nothing (found verifying ranger-base-fwwl8 under ranger-base-rucsu; guarded by ranger-base-5x8xi)", cwd, flags.State)
		}
		if !strings.Contains(err.Error(), "/not/the/seats/directory") {
			t.Errorf("the refusal is %v, and it has to NAME the directory codex answered about — that sentence is the whole diagnosis for a launch that otherwise looks like nothing at all", err)
		}
	})

	// ARM 2: an answer naming no directory refuses. `data[]` is non-empty,
	// so codexAskOver's "answered about no directory at all" refusal — which
	// reads the LENGTH of data[] — is not what reaches this; the compare is.
	t.Run("an answer naming no directory refuses", func(t *testing.T) {
		cwd := ctCwd(t)
		bin, _ := ctFake(t, ctTalk(ctListReply("", ctOurs(t)), ""))
		if flags, err := ctAsk(bin, cwd).Flags(); err == nil {
			t.Fatalf("an entry naming no directory at all rendered %q (ranger-base-5x8xi)", flags.State)
		}
	})

	// THE SECOND CONTROL: the compare did not take over the refusal that was
	// already covering the neighbouring shape. An answer with TWO entries
	// both about the seat's own cwd passes the compare on both — so what
	// refuses it is still the duplicate-key guard, whose own sentence is the
	// one this guard was written from ("posse asks about ONE cwd").
	//
	// MEASURED which guard reaches it, because the first reading of this arm
	// named the wrong one: the DUPLICATE-KEY refusal fires, not the
	// session-flags count. The session-flags layer comes back once per asked
	// cwd with the same synthetic keys each time, so the repeat is seen
	// inside the loop and the count is never reached.
	t.Run("control: a repeat of the seat's OWN cwd still refuses on the key", func(t *testing.T) {
		cwd := ctCwd(t)
		ours := ctOurs(t)
		two := ctSpliceInventories(ctListReply(cwd, ours), ctListReply(cwd, ours))
		bin, _ := ctFake(t, ctTalk(two, ""))
		_, err := ctAsk(bin, cwd).Flags()
		if err == nil {
			t.Error("two inventories each carrying posse's eight session-flags hooks were accepted — the duplicate-key refusal is what covers this shape, and it is not covering it")
		} else if !strings.Contains(err.Error(), "named the key") {
			t.Errorf("the refusal is %v, and this shape's guard is the DUPLICATE-KEY one — both entries name the seat's own cwd, so the cwd compare has nothing to say about it. If the compare is what answers now it has widened past what it was scoped to (ranger-base-5x8xi)", err)
		}
	})

	// AND THE MIXED SHAPE, which is what the finding was about: two entries,
	// one of them the seat's. Before the compare this was refused only
	// incidentally — by the duplicate key the second entry's session-flags
	// layer happened to repeat — so an answer whose extra entry carried no
	// posse hooks at all went through. The compare answers it on the
	// DIRECTORY now, whatever the extra entry holds.
	t.Run("two entries, one of them another directory, refuses on the directory", func(t *testing.T) {
		cwd := ctCwd(t)
		two := ctSpliceInventories(ctListReply(cwd, ctOurs(t)), ctListReply("/not/the/seats/directory", nil))
		bin, _ := ctFake(t, ctTalk(two, ""))
		_, err := ctAsk(bin, cwd).Flags()
		if err == nil {
			t.Fatal("an answer carrying an entry about /not/the/seats/directory was accepted (ranger-base-5x8xi)")
		}
		if !strings.Contains(err.Error(), "/not/the/seats/directory") {
			t.Errorf("the refusal is %v, and this shape has to be refused on the DIRECTORY: the extra entry declares no hook, so no duplicate key stands behind it", err)
		}
	})
}

// ctSpliceInventories joins two one-entry `hooks/list` answers into one
// answer carrying both entries, which is a shape ctListReply cannot render
// and codex really can (one entry per asked cwd).
func ctSpliceInventories(a, b string) string {
	const head = `{"id":2,"result":{"data":[`
	const tail = `]}}`
	return strings.TrimSuffix(a, tail) + "," + strings.TrimPrefix(b, head)
}
