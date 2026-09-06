//go:build posse_arm2

package posse

// CHECK 3 HAS A THIRD SUBJECT: THE COMMIT MESSAGE (ADR 0024 D2 check 3 and
// ADR 0048 D2, both as amended 2026-09-03 — product decision on
// ranger-base-1nbtn, built in ranger-base-qk8i9).
//
// The derived identity literals and this instance's config patterns are
// matched over every line of the message file the hook is already handed as
// "$1", through the same reader the data ceiling got in ranger-base-o2v6n
// (messageArm, gates.go) — one message reader for both walls, the same
// words with the fields swapped. INSIDE the visibility gate, where the rest
// of check 3 lives: this arm asks where content may GO, so a private-stamped
// repo runs none of it. That gate is the one thing separating this arm from
// the ceiling's, and the private CONTROL in PIN (m) is what measures it.
//
// WHY A MESSAGE AT ALL. It lands in the commit object and replicates with
// the branch, and it is the most operator-shaped prose in the repository.
// MEASURED over the 1136 messages then on main (ranger-base-1nbtn): the
// identity literals hit 5 commits, four of them the class's real target;
// this instance's one config pattern hit 18, every one avoidable by ADR
// 0048's own habit of writing "the pre-publication name". The price of a
// refusal is one re-issued command — git leaves the refused message in
// .git/COMMIT_EDITMSG, which PIN (r) measures rather than assumes.
//
// AND WHY CHECK 2 DOES NOT. Same census, the shipped list over the same
// 1136 messages: 29 hits, 22 of them the software's own vocabulary —
// fixture figures, blessed defaults, documented key values — and a message
// has no shape table to disposition that residue by. PIN (t) holds the
// decision so widening it later is a deliberate edit.
//
// The fixture vocabulary is instancepatternscope_qa_test.go's ("zephyr"),
// never this box's; the identity literals are whatever THIS box derives,
// read out of the wall with w.literal so nothing here is hardcoded.
//
// EVERY PIN HERE IS MUTATION-CHECKED, per alternative — a green pin over a
// wall that never had the hole measures nothing. The mutants and what each
// one reds are recorded at each pin; the runs are on ranger-base-qk8i9.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// PIN (m): the identity arm refuses a commit MESSAGE carrying a derived
// literal, under both forms that reach this hook with the message already
// in hand — `-m` and the crew's `-F -` — while the staged file and its path
// are SPOTLESS, so nothing but the message can be what refused it. The
// refusal keeps ADR 0024 D2's shape, matched text included: the box that
// reads it is the box the literal names.
//
// THE CONTROL, and it is what separates this arm from the ceiling's: the
// same message in the PRIVATE-stamped repo of the same wall commits clean
// and logs nothing. Check 3 is inside the visibility gate and this arm did
// not leave it.
//
// MUTATION-CHECKED (go test -overlay, tree untouched):
//   - the arm removed from identityGuardCheck: both forms here go red, the
//     private control stays green.
//   - the arm rendered ABOVE the gate (with the ceiling's): the private
//     CONTROL reds and the public halves stay green — two mutants, told
//     apart.
func TestQAIdentityGuardRefusesTheCommitMessage(t *testing.T) {
	w := newVisWall(t)
	username := w.literal(t, "username")
	const clean = "package posse\n\n// nothing identifying in here at all.\n"
	msg := "wire the reaper\n\nfollowed " + username + "'s note from tuesday\n"

	for i, form := range []string{"-m", "-F -"} {
		t.Run(form, func(t *testing.T) {
			rel := "internal/posse/msg" + string(rune('a'+i)) + ".go"
			out, err := qaMsgCommit(t, w, w.pub, rel, clean, form, msg, w.persona)
			if err == nil {
				t.Fatalf("an identity literal in the commit MESSAGE must be refused (%s):\n%s", form, out)
			}
			for _, want := range []string{
				"refused by posse gate: an operator identity literal in the commit MESSAGE",
				"ADR 0024 D2 check 3",
				"username:",
				"matched: " + username, // ADR 0024's shape: the box reading it is the box it names
				"matched in the commit message:",
				"rewrite the commit message",
				".git/COMMIT_EDITMSG",
				"this repo's beads db is marked: public",
				VisibilityOverrideEnv + "=" + VisibilityOverrideValue,
			} {
				if !strings.Contains(out, want) {
					t.Errorf("the message refusal must carry %q:\n%s", want, out)
				}
			}
			// The staged file and its path are clean, so neither of the other
			// two arms may be what spoke — or this pin is one of theirs
			// wearing a message.
			for _, never := range []string{
				"an operator identity literal in a staged file",
				"an operator identity literal in a staged PATH",
				"matched in the staged additions:",
				"the FILENAME, not its content",
			} {
				if strings.Contains(out, never) {
					t.Errorf("a MESSAGE hit must not be refused in another arm's words (%q):\n%s", never, out)
				}
			}
			if !strings.Contains(w.log(t), "identity literal scan [prepare-commit-msg hook] (public repo, commit message)") {
				t.Errorf("the refusal must be logged under check 3's label, naming the subject:\n%s", w.log(t))
			}
			w.unstage(t, w.pub, rel)
		})
	}

	// THE CONTROL: the same literal, the same message, the same wall — the
	// PRIVATE-stamped repo. Check 3 is gated; the ceiling is not, and there
	// is none configured here, so nothing may speak at all.
	before := w.log(t)
	for i, form := range []string{"-m", "-F -"} {
		rel := "internal/posse/ctl" + string(rune('a'+i)) + ".go"
		if out, err := qaMsgCommit(t, w, w.priv, rel, clean, form, msg, w.persona); err != nil {
			t.Errorf("control: a PRIVATE repo runs no check 3 at all — this message must commit (%s): %v\n%s", form, err, out)
		}
	}
	if w.log(t) != before {
		t.Errorf("control: nothing may be logged for a private repo:\n%s", strings.TrimPrefix(w.log(t), before))
	}
}

// PIN (n): the instance-pattern arm refuses a MESSAGE, and does it the way
// ranger-base-8114t made every other instance refusal do it — the class and
// a hit count, never the ERE and never the text, on stdout, on stderr and
// in refusals.log. A message is read in a terminal and pasted onto beads
// exactly as a staged line's refusal is.
//
// MUTATION-CHECKED: exChecks rendered plain (opsCheckCall(..., false)) at
// the message arm reds the withholding assertions here and nothing in PIN
// (m), which is the identity source and discloses on purpose.
func TestQAInstancePatternRefusesTheCommitMessageClassOnly(t *testing.T) {
	w := qaInstanceWall(t)
	const rel = "internal/posse/wire.go"
	msg := "wire the exporter\n\nthe " + qaInstanceName + " harness shipped this in 2025\n"

	out, err := qaMsgCommit(t, w, w.pub, rel, "package posse\n", "-F -", msg, w.persona)
	if err == nil {
		t.Fatalf("an instance pattern in the commit MESSAGE must be refused:\n%s", out)
	}
	for _, want := range []string{
		"refused by posse gate: an instance-defined visibility class in the commit MESSAGE",
		"ADR 0048 D2",
		qaInstanceClass + ": 1 hit(s)",
		"matched in the commit message:",
		"rewrite the commit message",
		".git/COMMIT_EDITMSG",
		OpsPatternsConfigKey + ":",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the message refusal must carry %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "an operator identity literal") {
		t.Errorf("an instance-pattern hit must not be refused as an identity literal:\n%s", out)
	}
	if !strings.Contains(w.log(t), "instance pattern scan [prepare-commit-msg hook] (public repo, commit message)") {
		t.Errorf("the refusal must be logged under its own name, naming the subject:\n%s", w.log(t))
	}
	// The value is the whole point of the key: it may not be in the terminal
	// and it may not be in a file that outlives the terminal.
	qaNoVocabulary(t, "the terminal refusal", out)
	qaNoVocabulary(t, "refusals.log", w.log(t))
}

// PIN (o): a message REUSED. The literal is committed while no hook runs at
// all, then a NEW clean file is staged and `git commit --amend --no-edit` —
// which hands this hook HEAD's message, source "commit" — is refused. Two
// things at once: the arm scans a message it did not see typed, and it does
// not case on "$2".
//
// Bypassing the hook is how "before the wall carried it" is spelled for the
// identity source, and it is not a shortcut: unlike a config pattern, the
// literals are DERIVED from the box at every render (DeriveIdentityLiterals),
// so there is no rendered hook that lacks them to commit under. The config
// source's own version of this pin, with the key really added between the
// two commits, is TestQADataCeilingRefusesAReusedMessage.
//
// MUTATION-CHECKED: the arm rendered under `case "$2" in message)` — i.e.
// skipping source=commit — reds THIS pin alone; PIN (m) stays green because
// -m and -F - both arrive as source "message".
func TestQAIdentityGuardRefusesAReusedMessage(t *testing.T) {
	w := newVisWall(t)
	username := w.literal(t, "username")
	const first = "internal/posse/a.go"
	w.stage(t, w.pub, first, "package posse\n")
	msg := "reap the pane\n\n" + username + " asked for it\n"
	if out, err := w.git(w.pub, nil, "-c", "core.hooksPath=/dev/null", "commit", "-qm", msg, "--", first); err != nil {
		t.Fatalf("fixture premise: with no hook running this message must commit: %v\n%s", err, out)
	}

	// A NEW, clean staged file and --no-edit: nothing this commit writes
	// carries the literal. The only subject that does is the message it is
	// reusing out of HEAD.
	w.stage(t, w.pub, "internal/posse/b.go", "package posse\n\n// clean\n")
	out, err := w.git(w.pub, w.persona, "commit", "--amend", "--no-edit", "--", "internal/posse/b.go")
	if err == nil {
		t.Fatalf("a REUSED message must be scanned by check 3:\n%s", out)
	}
	if !strings.Contains(out, "an operator identity literal in the commit MESSAGE") || !strings.Contains(out, "matched: "+username) {
		t.Errorf("the amend must be refused by the MESSAGE arm:\n%s", out)
	}
	if !strings.Contains(w.log(t), "(public repo, commit message)") {
		t.Errorf("the refusal must be logged naming the subject:\n%s", w.log(t))
	}
}

// PIN (p): a '#'-leading line is scanned, because git KEEPS it. The default
// cleanup for a message given with -m or -F and no editor is "whitespace",
// which strips nothing but blank space — so a pasted markdown heading lands
// in the commit object. The CONTROL is git's own behaviour, asserted rather
// than assumed: the same message in the wall's PRIVATE repo commits, and
// `git log` shows the '#' line in HEAD. If a builder ever teaches this arm
// to strip comment lines, the refusal goes away and the control still shows
// the line landing — which is the hole, stated.
//
// MUTATION-CHECKED: the arm rendered over `grep -v '^#'` reds this pin
// alone; (m), (n), (o), (r) stay green.
func TestQAIdentityGuardScansCommentLookingLines(t *testing.T) {
	w := newVisWall(t)
	username := w.literal(t, "username")
	msg := "wire it\n\n# " + username + "\n"

	out, err := qaMsgCommit(t, w, w.pub, "internal/posse/c.go", "package posse\n", "-F -", msg, w.persona)
	if err == nil {
		t.Fatalf("a comment-looking line in the message must still be scanned:\n%s", out)
	}
	if !strings.Contains(out, "an operator identity literal in the commit MESSAGE") {
		t.Errorf("the '#' line must be refused by the MESSAGE arm:\n%s", out)
	}

	// THE CONTROL: the same message where check 3 does not run. It commits,
	// and git kept the '#' line — so the line the arm scans really lands.
	if out, err := qaMsgCommit(t, w, w.priv, "internal/posse/c.go", "package posse\n", "-F -", msg, w.persona); err != nil {
		t.Fatalf("control: a private repo must take this message: %v\n%s", err, out)
	}
	body, err := w.git(w.priv, nil, "log", "-1", "--format=%B")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "# "+username) {
		t.Errorf("control: git must KEEP the '#' line under the default cleanup — if it does not, this arm scans something that never commits:\n%q", body)
	}
}

// PIN (q): THE RESIDUAL, pinned so it cannot change by accident. A message
// typed in the EDITOR is not scanned and cannot be: prepare-commit-msg runs
// BEFORE the editor opens and is handed git's template alone (measured, git
// 2.50.1, ranger-base-pqlxr). So this commit LANDS in the PUBLIC repo and
// the literal is in HEAD — the residual is real, not a rig artifact, which
// is why the second assertion is here. It goes red the day a commit-msg
// layer is added, so that change is deliberate rather than discovered.
func TestQAIdentityGuardResidualEditorTypedMessage(t *testing.T) {
	w := newVisWall(t)
	username := w.literal(t, "username")
	ed := filepath.Join(t.TempDir(), "editor.sh")
	write(t, ed, "#!/bin/sh\nprintf '%s\\n' 'ran what "+username+" asked for' > \"$1\"\n")
	if err := os.Chmod(ed, 0o755); err != nil {
		t.Fatal(err)
	}
	w.stage(t, w.pub, "internal/posse/d.go", "package posse\n")
	out, err := w.git(w.pub, append(append([]string(nil), w.persona...), "GIT_EDITOR="+ed),
		"commit", "--", "internal/posse/d.go")
	if err != nil {
		t.Fatalf("the EDITOR path is check 3's stated exclusion — it must still land: %v\n%s", err, out)
	}
	body, err := w.git(w.pub, nil, "log", "-1", "--format=%B")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, username) {
		t.Fatalf("fixture premise: the editor's text must actually be the commit's message, got %q", body)
	}
	if strings.Contains(w.log(t), identityScanLabel) {
		t.Errorf("nothing may be logged: the hook never saw this text:\n%s", w.log(t))
	}
}

// PIN (r): the refusal's own claims, measured. After it, .git/COMMIT_EDITMSG
// still holds the message the writer typed and HEAD is unchanged — which is
// what the remedy tells them, and a remedy that names a file that is not
// there is worse than none. Then the operator's override passes, says so,
// and logs exactly ONE line whose tail names the commit message.
//
// MUTATION-CHECKED: the override branch dropped from the message arm reds
// the second half; the OVERRIDDEN log line rendered without the subject
// tail reds the last assertion.
func TestQAIdentityGuardMessageRefusalLeavesTheMessageAndOverrides(t *testing.T) {
	w := newVisWall(t)
	username := w.literal(t, "username")
	const rel = "internal/posse/e.go"
	// A HEAD to be unchanged: the scratch repo starts with no commit at all,
	// and "HEAD is unchanged" over an empty repo asserts nothing.
	w.plant(t, w.pub, "internal/posse/base.go", "package posse\n")
	head0, err := w.git(w.pub, nil, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("fixture premise: the scratch repo must have a HEAD to be unchanged: %v %s", err, head0)
	}

	msg := "wire it\n\n" + username + " typed this one\n"
	out, err := qaMsgCommit(t, w, w.pub, rel, "package posse\n", "-F -", msg, w.persona)
	if err == nil {
		t.Fatalf("fixture premise: the message must be refused:\n%s", out)
	}
	editmsg, rerr := os.ReadFile(filepath.Join(w.pub, ".git", "COMMIT_EDITMSG"))
	if rerr != nil || !strings.Contains(string(editmsg), username) {
		t.Errorf("the remedy says the message is still in .git/COMMIT_EDITMSG — it must be (err=%v):\n%q", rerr, string(editmsg))
	}
	if head1, _ := w.git(w.pub, nil, "rev-parse", "HEAD"); head1 != head0 {
		t.Errorf("a refused commit must leave HEAD alone: %q -> %q", head0, head1)
	}

	before := strings.Count(w.log(t), identityScanLabel)
	out, err = w.gitIn(w.pub, append(append([]string(nil), w.persona...), VisibilityOverrideEnv+"="+VisibilityOverrideValue),
		msg, "commit", "-F", "-", "--", rel)
	if err != nil || !strings.Contains(out, identityScanLabel+" OVERRIDDEN") {
		t.Fatalf("the operator's override must pass, and say so: %v\n%s", err, out)
	}
	log := w.log(t)
	if after := strings.Count(log, identityScanLabel); after != before+1 {
		t.Errorf("the override must log exactly one line, got %d new:\n%s", after-before, log)
	}
	if !strings.Contains(log, identityScanLabel+" OVERRIDDEN [prepare-commit-msg hook] (commit message)") {
		t.Errorf("the OVERRIDDEN line must name the commit message:\n%s", log)
	}
}

// PIN (s): the render. Both sources carry each posse_check call THREE times
// — content, path, message — the arm sorts after the path arm and inside
// the gate, the ceiling's message arm sorts BEFORE the gate line, and an
// empty identity with an empty pattern list renders no arm at all.
//
// MUTATION-CHECKED: the arm removed reds the counts and the order; the arm
// rendered above the gate reds the gate assertion (and PIN (m)'s private
// control); an arm rendered for an empty check 3 reds the last block.
func TestQACheckThreeMessageArmRenders(t *testing.T) {
	t.Parallel() // renders and reads strings; no repo, no env (ranger-base-pj87l)
	extra := []OpsPattern{{Class: qaInstanceClass, ERE: qaInstanceERE}}
	lit := IdentityLiteral{Class: "username", Value: "qa-fixture-operator"}
	set := OpsPatternSet{Extra: extra, Ceiling: []OpsPattern{{Class: qaCeilingClass, ERE: qaCeilingERE}}}
	hook := CommitGuardHook(VisibilityPublic, set, lit)

	gate := strings.Index(hook, `if [ "$posse_beads_visibility" = `+shQuote(VisibilityPublic)+` ]; then`)
	// The banner names check 3's sources and gained a third when the crew
	// names did (ranger-base-cdxpf); the landmark is its stable head.
	content := strings.Index(hook, "─── check 3: identity literals")
	path := strings.Index(hook, "check 3, second arm: the same patterns over ADDED staged PATHS")
	message := strings.Index(hook, "check 3, third arm: the commit MESSAGE")
	ceilingMsg := strings.Index(hook, "the data ceiling, third arm: the commit MESSAGE")
	if gate < 0 || content < 0 || path < 0 || message < 0 || ceilingMsg < 0 {
		t.Fatalf("a landmark is missing (gate=%d content=%d path=%d message=%d ceilingMsg=%d)", gate, content, path, message, ceilingMsg)
	}
	// The ceiling's message arm runs above the gate and check 3's inside it,
	// so a message that trips both is refused with the ceiling's stricter
	// remedy (ADR 0050 D2, "first in order").
	if !(ceilingMsg < gate && gate < content && content < path && path < message) {
		t.Errorf("want ceiling message < gate < check 3 content < path < message, got %d %d %d %d %d", ceilingMsg, gate, content, path, message)
	}
	// One message reader, not two mechanisms: check 3's arm reads the same
	// `posse_msg` the ceiling's does, rendered at check 3's indent. The
	// file itself is read once per wall into that variable, above the
	// cleanup-mode branch, because the scissors cut applies to every mode
	// (ranger-base-xfgcn).
	if n := strings.Count(hook, `posse_msg=$(cat "$1" 2>/dev/null)`); n != 2 {
		t.Errorf("want exactly two message file reads — one per wall — got %d", n)
	}
	if n := strings.Count(hook, `posse_added=$posse_msg`); n != 2 {
		t.Errorf("want exactly two whole-message reads — one per wall — got %d", n)
	}
	// Counted from check 3's own banner to the end of the file: an instance
	// pattern is rendered into check 0 as well (the beads jsonl), and a
	// count over the whole hook would be measuring both checks at once.
	block := hook[content:]
	for _, call := range []string{
		"posse_check 'username' 'qa-fixture-operator'",
		"posse_check '" + qaInstanceClass + "' '" + qaInstanceERE + "' " + opsClassOnlyArg,
	} {
		if n := strings.Count(block, call); n != 3 {
			t.Errorf("check 3 must render %q at all three arms, got %d", call, n)
		}
	}
	for _, want := range []string{
		"an operator identity literal in the commit MESSAGE",
		"an instance-defined visibility class in the commit MESSAGE",
		"(public repo, commit message)",
		"OVERRIDDEN [prepare-commit-msg hook] (commit message)",
	} {
		if !strings.Contains(hook, want) {
			t.Errorf("the rendered arm must carry %q:\n%s", want, hook)
		}
	}

	// Nothing derived and nothing configured is still nothing — the message
	// arm included: a box with neither pays for no read of the message file.
	bare := CommitGuardHook(VisibilityPublic, OpsPatternSet{})
	if strings.Contains(bare, "check 3, third arm") || strings.Contains(bare, `posse_msg=$(cat "$1"`) {
		t.Error("an empty check 3 and an empty ceiling must render no message arm at all")
	}
}

// PIN (s), continued: the three render sites see ONE arm (ADR 0023). The
// launcher's L3 probe re-renders the hook and compares byte-for-byte, so an
// arm the install stamps and the probe does not would make every launch
// into a hooked repo read "ours but stale" forever.
//
// MUTATION-CHECKED: the probe rendering check 3 from a set copy with Extra
// cleared reds this pin and nothing else in this file.
func TestQACheckThreeMessageArmIsSeenIdenticallyByTheL3Probe(t *testing.T) {
	w := qaInstanceWall(t)
	a := &App{ConfigPath: w.home + "/config.yaml"}
	if p := a.probeL3Hooks(w.pub, false); !p.CommitGuard {
		t.Errorf("L3 must vouch for the hook it just stamped: %s", p.CommitGuardDegraded)
	}
	if !strings.Contains(qaHookFile(t, w.pub), "check 3, third arm: the commit MESSAGE") {
		t.Error("the stamped hook does not carry check 3's commit-message arm")
	}
	vis, _ := a.BeadsVisibility(w.pub)
	if want := CommitGuardHook(vis, a.OpsPatternSet(), testIdentity(t, w.pub)...); qaHookFile(t, w.pub) != want {
		t.Error("the public repo's stamped hook is not byte-identical to CommitGuardHook over the same set")
	}
}

// PIN (t): the SHIPPED list does NOT scan the message, and that is a
// decision with a census behind it (ranger-base-1nbtn), not an omission. A
// shipped cost-class figure in a commit message LANDS and nothing is
// logged; the CONTROL is the same figure on an ADDED line of a staged .md,
// which check 2 still refuses — so this pin says the message is out of
// check 2's scope rather than that check 2 is asleep.
//
// Widening check 2 to the message is what reds this pin, which is the
// point: over the 1136 messages then on main the shipped list hit 29, 22 of
// them the software's own vocabulary, and a message has no shape table to
// disposition them by. The trigger for re-opening it is a live cost or
// guard figure found in a message at verify time, filed -l product with the
// sha.
func TestQAShippedPatternsDoNotScanTheCommitMessage(t *testing.T) {
	w := newVisWall(t)
	// A shipped 'cost' hit and nothing else: no instance pattern is
	// configured on this wall, and the identity literals are not in it.
	const opsLine = "last window's spend was $715/wk"

	if out, err := qaMsgCommit(t, w, w.pub, "internal/posse/budget.go", "package posse\n", "-F -",
		"tune the budget\n\n"+opsLine+"\n", w.persona); err != nil {
		t.Errorf("the shipped list is not scanned over the commit MESSAGE (ADR 0024, ranger-base-1nbtn): %v\n%s", err, out)
	}
	if l := w.log(t); strings.Contains(l, "markdown ops-content scan") || strings.Contains(l, identityScanLabel) {
		t.Errorf("nothing may be logged for a message check 2 does not read:\n%s", l)
	}

	// THE CONTROL: check 2 is alive, on the subject it does have.
	w.stage(t, w.pub, "docs/notes.d/budget.md", "# notes\n\n// "+opsLine+"\n")
	out, err := w.git(w.pub, w.persona, "commit", "-m", "x", "--", "docs/notes.d/budget.md")
	if err == nil {
		t.Fatalf("control: the same figure on an ADDED markdown line must still be refused by check 2:\n%s", out)
	}
	for _, want := range []string{
		"refused by posse gate: ops-class content in staged markdown in a public repo",
		"cost:", "$715/wk",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("control: check 2's refusal must carry %q:\n%s", want, out)
		}
	}
}
