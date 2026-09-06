package posse

// QA pins written verifying four folded closes under ranger-base-i9dbb
// (ranger-base-d14ie, ranger-base-8devq, ranger-base-g8e, ranger-base-9guhz).
//
// All four were closed by the ranger-base-kcnc6 groom as FOLDED INTO an open
// bead, under the convention the groom's fold comment states on each target:
// the folded bead's repro and acceptance criteria stay in its own description
// and count toward the target's DONE WHEN. In bd that convention holds — the
// pointer runs both ways on all four. In the TREE it holds only where
// something reads the folded half, and for three of the four nothing did.
//
// Each pin below is that reader. Two are parked on the OPEN bead that owes
// the fix, never on the closed one (ranger-base-6889's park, re-pointed under
// ranger-base-z84xi, is why: a park naming a closed id reads as an
// instruction to un-skip, and un-skipping reds the suite). One is live,
// because the thing it guards is a falsehood a sweep is currently instructed
// to write, not a fix somebody owes.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// i9dbbRead reads a repo file by path parts, relative to the repo root.
func i9dbbRead(t *testing.T, parts ...string) string {
	t.Helper()
	p := filepath.Join(append([]string{qibRepoRoot(t)}, parts...)...)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("the guard must read the file it judges: %v", err)
	}
	return string(b)
}

// ─── ranger-base-9guhz, carried into ranger-base-mqoid ───────────────────────

// ranger-base-9guhz was closed "folded into ranger-base-mqoid: an input
// correction for the mqoid taker, now a comment there". The comment there is
// a POINTER — one line naming 9guhz — and mqoid's own description and NOTES
// still carry the instruction 9guhz exists to retract: "0036's status line
// gains '· unbuilt: ranger-base-a0ln0'". That input was true when the ay3dr
// ruling was written and is false now. A taker who works the description and
// does not chase the pointer writes a falsehood into a shipped record.
//
// So this arm is LIVE, not parked. It is green today and reds the moment the
// sweep writes the retracted stamp — the correction enforced by the suite
// instead of by a comment somebody has to think to open.
//
// MEASURED 2026-09-02 at 13db95e: ranger-base-a0ln0 built the verb
// (internal/posse/backup.go, `case "backup"` in cmd/posse/main.go, five
// `backup_*` config keys, 13 files citing the record) and ranger-base-zv3y6
// built §4's ticker. Both are named BUILT in 0036's own status line.
func TestQAADR0036StatusLineDoesNotCarryTheRetractedUnbuiltStamp(t *testing.T) {
	t.Parallel()
	adr := i9dbbRead(t, "docs", "adr", "0036-posse-backup.md")

	// Positive witness first: without it "the stamp is absent" is equally
	// true of a read that got the wrong file (pass-count-is-not-a-coverage-
	// floor). Witnessed on the record's identity and on the presence of a
	// status line — NOT on the bead id, which a legitimate rewording of the
	// status may drop and which would then red this guard for no defect.
	if !strings.HasPrefix(adr, "# ADR 0036") {
		t.Fatalf("this guard is not reading ADR 0036 — first line %q", strings.SplitN(adr, "\n", 2)[0])
	}
	if !strings.Contains(adr, "*Status:") {
		t.Fatal("ADR 0036 has no status line — this guard has nothing to judge")
	}

	// The retracted stamp, written the way ADR 0040 §3.5 spells it. Assert
	// on the pairing, not on the word "unbuilt" alone: §4's ticker and §1's
	// cut sections are legitimately discussed as unbuilt in the prose below
	// the status, and 0036 says so deliberately.
	for _, dead := range []string{
		"unbuilt: ranger-base-a0ln0",
		"unbuilt: `ranger-base-a0ln0`",
	} {
		if strings.Contains(adr, dead) {
			t.Errorf("0036's status carries %q — ranger-base-a0ln0 BUILT the verb on 2026-09-01 (backup.go, the `backup` case in main.go, five backup_* keys). ranger-base-9guhz retracted that ruling input; mqoid's description still asks for it", dead)
		}
	}

	// And the reason, measured rather than asserted from the ADR's own
	// prose: the verb is in the tree. If this ever goes false the stamp is
	// no longer a falsehood and this whole guard should be revisited.
	if _, err := os.Stat(filepath.Join(qibRepoRoot(t), "internal", "posse", "backup.go")); err != nil {
		t.Errorf("backup.go is gone — the premise of this guard (a0ln0 built the verb) no longer holds: %v", err)
	}
	if main := i9dbbRead(t, "cmd", "posse", "main.go"); !strings.Contains(main, `case "backup":`) {
		t.Error("cmd/posse/main.go no longer dispatches a backup verb — the premise of this guard no longer holds")
	}
}

// The other half of ranger-base-9guhz: ADR 0040's disposition row for 0036
// reads "nothing live: no `backup` symbol, no age dependency, no config key;
// no build bead found". All four clauses are false as of 2026-09-01, and the
// row is mqoid's to correct (0040 §1 row 0036).
//
// Parked, because unlike the stamp above this is a fix somebody owes rather
// than a falsehood about to be written. Un-skip when ranger-base-mqoid lands.
//
// Shown able to fail: with the skip lifted this FAILS today at 13db95e.
func TestQAADR0040Row0036DoesNotCallTheBackupVerbUnbuilt(t *testing.T) {
	t.Parallel()
	t.Skip("ranger-base-mqoid (carries closed ranger-base-9guhz): ADR 0040's 0036 row still says no backup symbol, no config key and no build bead — four clauses, all false since 2026-09-01")
	doc := i9dbbRead(t, "docs", "adr", "0040-adr-consolidation.md")
	var row string
	for _, line := range strings.Split(doc, "\n") {
		if strings.HasPrefix(line, "| 0036 ") {
			row = line
			break
		}
	}
	if row == "" {
		t.Fatal("ADR 0040 §1 no longer has a row for 0036 — this guard is not reading what it thinks it is")
	}
	for _, dead := range []string{"no build bead found", "no config key", "no `backup` symbol"} {
		if strings.Contains(row, dead) {
			t.Errorf("ADR 0040's 0036 row still says %q:\n  %s", dead, row)
		}
	}
}

// ─── ranger-base-g8e, carried into rangerhq-vr6j ─────────────────────────────

// ranger-base-g8e was closed "folded into rangerhq-vr6j: the L0 model comment
// and the L0Spellings wildcard half are one block of gates.go". The fold is
// honest — they are one block — but vr6j's own pin reaches only the wildcard
// half, and it reaches it with an instruction: gates_test.go's shared
// false-positive loop errors with "close rangerhq-vr6j" the moment the
// wildcard shape is fixed. Follow that literally and vr6j closes with g8e's
// half — the three-way model comment — still saying the retired thing.
// g8e's id appears in no file in the tree.
//
// What g8e measured (claude 2.1.241, non-interactive, --settings '{}'): the
// `:*` form is not a character prefix of the command string, it is a prefix
// of the argv TOKENS. Under `Bash(sed -n:*)`, `sed -ni '1p' f.txt` is DENIED
// though the string starts with the rule's text, and `sed -n -i.bak …` is
// ALLOWED. No behaviour change follows — the widening L0Spellings does is
// still needed and still correct under token matching — so this is the
// comment being precise about WHY, and the pin is the reader that notices
// when it stops being wrong.
//
// Was parked on the open bead; rangerhq-vr6j landed and the skip is lifted.
// All three sites now name the token semantics: gates.go's three-way table,
// claudeDenyMatch's twin of it in gates_test.go (which had always
// IMPLEMENTED token matching — `c == p || strings.HasPrefix(c, p+" ")` — and
// only described it wrong), and NOTES.md.
//
// Shown able to fail: it FAILED on all three sites at 13db95e with the skip
// lifted, and it fails again the moment any of them says the retired phrase.
func TestQAL0ModelDoesNotCallThePrefixFormALiteralStringPrefix(t *testing.T) {
	t.Parallel()

	// Assembled from pieces so this file is not itself a hit for the phrase
	// it forbids, the way coxnDeadFraming is.
	dead := "literal " + "prefix of the command string"
	// The witness that the reads landed on the right text: each file must
	// still describe the three-way matcher at all.
	sites := []struct {
		parts   []string
		witness string
	}{
		{[]string{"internal", "posse", "gates.go"}, "L0Spellings widens"},
		{[]string{"internal", "posse", "gates_test.go"}, "claudeDenyMatch models"},
		{[]string{"NOTES.md"}, "L0Spellings"},
	}
	for _, s := range sites {
		name := filepath.Join(s.parts...)
		body := i9dbbRead(t, s.parts...)
		if !strings.Contains(body, s.witness) {
			t.Fatalf("%s no longer contains %q — this guard is not reading what it thinks it is", name, s.witness)
		}
		// The verdict is taken on a FLATTENED body, not line by line: at
		// gates_test.go:608 the phrase wraps across a comment continuation
		// ("… of the command\n// string"), and a per-line scan reported the
		// other two sites and silently passed over that one — the exact
		// shape where a partial fix goes green.
		if !strings.Contains(i9dbbFlatten(body), dead) {
			continue
		}
		// Best effort at a location: the head of the phrase is on one line
		// at all three sites today. Name the file either way.
		where := name
		for i, line := range strings.Split(body, "\n") {
			if strings.Contains(line, "literal "+"prefix") {
				where = fmt.Sprintf("%s:%d", name, i+1)
				break
			}
		}
		t.Errorf("%s still models the `:*` form as a character prefix — claude 2.1.241 matches per argv token (ranger-base-g8e)", where)
	}
}

// i9dbbFlatten drops comment markers and collapses every run of whitespace to
// one space, so a phrase that wraps across two comment lines still reads as
// one phrase. Nothing here needs the original offsets — the location is
// recovered separately, and the verdict is about the words.
func i9dbbFlatten(body string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(body, "//", " ")), " ")
}

// ─── ranger-base-8devq, carried into ranger-base-b22vq ───────────────────────

// ranger-base-8devq was closed "folded into ranger-base-b22vq: same sweep,
// strings and a parked pin that still describe the old promoted set". b22vq's
// four sites are production strings that enumerate the promoted PATHS;
// 8devq's two are about what a promoted home is TOLD. Disjoint files, and a
// commit that lands all four of b22vq's enumerations leaves both of 8devq's
// sites untouched and every test green.
//
// installseedrow_test.go already reads INSTALL.md §14's seeding row — and
// asserts the row names `posse promote` and `promoted.json`, which the STALE
// sentence does. So the existing reader passes over the defect. This is the
// arm that does not.
//
// THE DEFECT (8devq, measured at be5077c): ranger-base-pith gave initFrom a
// third sentence on a promoted home — VerifyPromoted's line plus "every
// dispatched launch will refuse until you run `posse promote`"
// (init.go:356-357, promote.go:361). INSTALL.md §14's row still ends by
// teaching the reader to infer the promoted case from an ABSENCE: "If
// neither sentence appears, this home was promoted, not seeded". The absence
// is gone. A reader following the row meets a sentence the row lists no arm
// for.
//
// Was parked on the open bead; ranger-base-b22vq landed and the skip is
// lifted. HALF of what this pin asked for arrived before that, from a
// different bead: ranger-base-39jnl made init REFUSE a promoted home, and
// the row was rewritten around the refusal, so "If neither sentence appears"
// was already gone. The second arm was still owed and still red — §14's row
// listed no arm for the mismatch sentence at all — and b22vq's edit added
// it.
//
// The other site 8devq named — installseedrow_test.go's header comment, its
// `promoted.json` assertion and the t.Skip at its line 117 — is gone too,
// swept by the same ranger-base-39jnl rewrite: the file now pins init's
// refusal, holds no t.Skip, and asserts nothing about the manifest filename.
// Checked at 1674846b, not assumed.
//
// Shown able to fail: with the skip lifted it FAILED at 1674846b on the
// second arm, and it fails again the moment the row stops naming the
// sentence.
func TestQAInstallSeedingRowNamesTheManifestMismatchSentence(t *testing.T) {
	t.Parallel()

	// The witness that the sentence really is printed: the row is stale only
	// because the code changed under it.
	if init := i9dbbRead(t, "internal", "posse", "init.go"); !strings.Contains(init, "every dispatched launch will refuse until you run") {
		t.Fatal("init.go no longer prints the third sentence — the premise of this guard no longer holds")
	}
	if promote := i9dbbRead(t, "internal", "posse", "promote.go"); !strings.Contains(promote, "constitution does not match its manifest") {
		t.Fatal("promote.go no longer builds the mismatch line — the premise of this guard no longer holds")
	}

	row := seedingRow(t, i9dbbRead(t, "INSTALL.md"))
	if strings.Contains(row, "If *neither* sentence appears") || strings.Contains(row, "If neither sentence appears") {
		t.Errorf("INSTALL.md §14's seeding row still sends the reader looking for an absence that ranger-base-pith removed:\n  %s", row)
	}
	if !strings.Contains(row, "does not match its manifest") {
		t.Errorf("INSTALL.md §14's seeding row does not name the third sentence initFrom now prints on a promoted home:\n  %s", row)
	}
}
