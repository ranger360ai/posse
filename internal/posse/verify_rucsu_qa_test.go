//go:build !posse_arm2 && !posse_arm3

package posse

// A LIVE-DEFECT PIN from verifying ranger-base-fwwl8 (verify bead
// ranger-base-rucsu, findings bead named on it).
//
// IT SHIPS GREEN AND IT ASSERTS A HOLE. D7's ask sends `hooks/list
// {cwds:[<the seat's cwd>]}` and reads `result.data[]` back, one entry per
// asked cwd, each carrying its own `cwd` beside its `hooks[]`. posse decodes
// that `cwd` field and uses it for one thing — the log sentence
// codexAskNotes prints — and never compares it with the directory it asked
// about. So an answer about SOMEBODY ELSE'S directory, or about no directory
// at all, renders a full trust line that a launch would type.
//
// WHY THAT IS THE SHAPE THIS FILE EXISTS FOR. codexHooksState refuses the
// sibling contradiction in codex's answer — the same hook key twice — and its
// refusal says why in words that cover this one exactly: "posse asks about
// ONE cwd, so this is not two rows about two seats". codexAskOver refuses an
// answer about no directory at all (`data[]` empty). Between those two
// guards sits an answer about one directory that is not the seat's, and
// nothing reads it. What it costs is what every refusal in codextrust.go is
// written to prevent: the hooks discovered in the seat's OWN project are
// absent from the table, so they stay untrusted, codex draws the startup
// review dialog, and the seat submits nothing and reports nothing.
//
// WHEN THE GUARD LANDS THIS PIN REDS. That is deliberate and it is the whole
// mechanism: invert the two hole arms to assert the refusal, keep the two
// controls as they are, and name the bead that fixed it. Do not delete the
// arms — the controls below are the half that says a compare landed in the
// right place rather than refusing everything.
//
// WHAT A FIXER HAS TO MEASURE FIRST, because this pin cannot answer it: what
// codex actually ECHOES in that field for the path posse hands it. posse
// passes an absolute, symlink-resolved path; nothing on record says whether
// codex echoes it byte for byte, so a strict equality compare could refuse a
// launch that was fine. docs/adr/0076-trust-ask.probe.sh's `state` arm prints
// the real answer and is where that reading comes from — it is NOT measured
// here, and this pin's fixtures echo whatever they are handed.

import (
	"strings"
	"testing"
)

func TestQACodexAskDoesNotVetTheDirectoryItsAnswerIsAbout(t *testing.T) {
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

	// THE HOLE, arm 1: an answer about another seat's directory.
	t.Run("HOLE: an answer about another directory is accepted", func(t *testing.T) {
		cwd := ctCwd(t)
		bin, _ := ctFake(t, ctTalk(ctListReply("/not/the/seats/directory", ctOurs(t)), ""))
		_, err := ctAsk(bin, cwd).Flags()
		if err != nil {
			t.Fatalf("THE GUARD HAS LANDED, and this pin is now the wrong way round: asking about %s and being answered about /not/the/seats/directory is refused with %v. Invert this arm to assert that refusal — the hooks the seat's own project declares are absent from such an answer, so they stay untrusted and codex draws the startup review dialog — keep both controls, and name the bead that closed it (found verifying ranger-base-fwwl8 under ranger-base-rucsu)", cwd, err)
		}
	})

	// THE HOLE, arm 2: an answer naming no directory. `data[]` is non-empty,
	// so codexAskOver's "answered about no directory at all" refusal — which
	// reads the LENGTH of data[] — does not reach this.
	t.Run("HOLE: an answer naming no directory is accepted", func(t *testing.T) {
		cwd := ctCwd(t)
		bin, _ := ctFake(t, ctTalk(ctListReply("", ctOurs(t)), ""))
		_, err := ctAsk(bin, cwd).Flags()
		if err != nil {
			t.Fatalf("THE GUARD HAS LANDED for the empty-cwd shape: %v. Invert this arm as the one above says (ranger-base-rucsu)", err)
		}
	})

	// THE SECOND CONTROL, and it is what BOUNDS the finding: an answer that
	// repeats posse's own layer is already refused, so the gap is an answer
	// about ONE wrong directory and not every multi-directory answer.
	//
	// MEASURED which guard reaches it, because the first reading of this arm
	// named the wrong one: the DUPLICATE-KEY refusal fires, not the
	// session-flags count. The session-flags layer comes back once per asked
	// cwd with the same synthetic keys each time, so the repeat is seen
	// inside the loop and the count is never reached. Both refusals carry
	// the same reasoning ("posse asks about ONE cwd"), which is the sentence
	// this finding is about — it is written, and it is not read anywhere the
	// cwd itself is.
	t.Run("bound: an answer repeating posse's own layer is already refused", func(t *testing.T) {
		cwd := ctCwd(t)
		ours := ctOurs(t)
		two := ctSpliceInventories(ctListReply(cwd, ours), ctListReply("/not/the/seats/directory", ours))
		bin, _ := ctFake(t, ctTalk(two, ""))
		_, err := ctAsk(bin, cwd).Flags()
		if err == nil {
			t.Error("two inventories each carrying posse's eight session-flags hooks were accepted — the duplicate-key refusal is what covers any part of this class, and it is not covering this")
		} else if !strings.Contains(err.Error(), "named the key") {
			t.Errorf("the refusal is %v, and what reached this shape when it was measured was the DUPLICATE-KEY guard — if another one does now, say which here, because the bound on the finding is whatever that guard happens to cover", err)
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
