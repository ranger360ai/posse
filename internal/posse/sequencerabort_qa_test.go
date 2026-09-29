//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-71g2f — the git shim's sequencer-audit arm.
//
// Claim: a seat cannot get a ZERO exit out of `git cherry-pick|revert|rebase`
// while the operation is still in progress. See renderSequencerAudit for the
// live measurement this stands on; what this file owns is that the rendered
// shell really behaves that way, and that the arm stays as narrow as the
// measurement says it must be.
//
// WHY THE PIN IS ABOUT SILENCE. The defect it guards is not a crash, it is a
// zero exit — the working tree is restored, HEAD is right, and since ADR 0059
// granted the lock there is not even a stderr line to read: the two routes
// that still strand a blocker (a STRAY lock someone else left in the shared
// git dir, and L4, where nothing binds one) each end in git retrying for
// `core.packedRefsTimeout` and then exiting 0, silently. If this arm stops
// firing, nothing goes red anywhere: the next seat to cherry-pick simply
// loses its commit to `fatal: cannot do a partial commit during a
// cherry-pick` and reads that as its own PID refusing it.
//
// Hermetic, and deliberately so: the real failure needs a stray lock sitting
// in another repo's common dir, or an L4 cage, neither of which a test can
// arrange. The fake git below reproduces the only thing the shim can actually
// see — a verb that exits 0 having left a pseudo-ref behind — so these arms
// test the wall, not the cage. It reads no repo root, so it is not a
// tree-wide pin and owes the Makefile no door (AGENTS.md, "A -run filter
// cannot reach a tree-wide pin").
//
// MUTATION-CHECKED (2026-09-11), each mutation applied to gates.go alone and
// the six arms below run against it:
//
//	drop AUTO_MERGE from sequencerLeftovers   reds the recipe arm alone
//	add AUTO_MERGE to sequencerBlockers       reds the AUTO_MERGE arm alone
//	add `merge` to sequencerVerbs             reds the leaves-alone arm alone
//	drop the globalValueOpts scan             reds the `-C` arm alone
//	`[ $posse_src -eq 0 ]` -> `-ne 0`         reds FOUR arms
//	add packed-refs.lock to sequencerLeftovers  reds the common-dir arm alone
//	`--absolute-git-dir` -> `--git-common-dir`  reds the common-dir arm alone
//	a recipe line that rm's the stray lock too  reds the common-dir arm alone
//
// Re-run in full 2026-09-20 (ranger-base-u18bo) with three more, each applied
// to renderSequencerAudit alone — the first is the escape that bead found:
//
//	an UNQUOTED `rm` of <common>/index            reds :431 and :457
//	an unquoted `rm` of /etc/<x>, outside common  reds :431
//	<common>/worktrees/other in PROSE, no `rm`    reds :457
//
// The last two are there to keep the two halves of the common-dir check
// honest about what each one sees: a path outside the common dir entirely is
// the rm-line scan's alone, a shared path named in prose is the whole-refusal
// scan's alone, and the escape reds both.
//
// The `-ne 0` one reds four and that is the honest reading, not a leak:
// inverted, the audit fires on every failing run and on no succeeding one, so
// the three arms that expect an alarm go quiet and the one that expects quiet
// alarms. A mutation that breaks the invariant itself is supposed to be
// visible everywhere the invariant is asserted.
//
// Rows 6-8 are ranger-base-o0dr4's (ADR 0059 D3), and the common-dir
// arm is the one that can go vacuous with nothing else noticing: no other arm
// in this file looks at WHICH dir the recipe points into. They are deliberately
// the two directions the arm can be wrong in — the recipe reaching a shared
// path by NAME (row 6), by asking git for the wrong dir (row 7), and by
// helpfully tidying the stray lock it can see (row 8, written as a second
// `rm -f` line built with dirname, so it needs no --git-common-dir and tests
// the executed half on its own). Row 8 reds three separate assertions.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fakeGitForSequencer writes a git stand-in that honours the leading global
// options the way real git does, answers `rev-parse --absolute-git-dir` and
// `--git-common-dir` (differently, for a worktree-shaped git dir), and leaves
// behind whatever $FAKE_LEAK names — which is the whole of what a ref delete
// that could not take the lock looks like from outside.
func fakeGitForSequencer(t *testing.T, dir string) {
	t.Helper()
	body := `#!/bin/sh
gd=$FAKE_GITDIR
while [ $# -gt 0 ]; do
  case "$1" in
    -C) gd="$2/.git"; shift 2 ;;
    -c|--git-dir|--work-tree|--namespace|--super-prefix|--config-env|--attr-source) shift 2 ;;
    -*) shift ;;
    *) break ;;
  esac
done
if [ "$1" = rev-parse ]; then
  # A linked worktree's own git dir is <common>/worktrees/<name>, so these two
  # answers differ there and nowhere else, which is the whole of what the
  # common-dir arm varies. A fake that answered both the same could not refuse.
  case "$2" in
    --git-common-dir) case "$gd" in */worktrees/*) echo "${gd%/worktrees/*}" ;; *) echo "$gd" ;; esac ;;
    *) echo "$gd" ;;
  esac
  exit 0
fi
echo "real git $*"
if [ -n "$FAKE_LEAK" ]; then
  mkdir -p "$gd"
  for m in $FAKE_LEAK; do
    case "$m" in sequencer) mkdir -p "$gd/$m" ;; *) : > "$gd/$m" ;; esac
  done
fi
exit ${FAKE_RC:-0}
`
	if err := WriteExecutable(filepath.Join(dir, "git"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// sequencerShim renders a crew-shaped git shim in front of the fake git and
// returns the shim's path, the gates dir and the git dir the fake reports.
func sequencerShim(t *testing.T) (shim, gatesDir, gitDir string) {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	home := t.TempDir()
	realBin := t.TempDir()
	fakeGitForSequencer(t, realBin)
	t.Setenv("PATH", realBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	a := &App{Home: home, StateDir: filepath.Join(home, "state")}
	// The deny list every crew PID carries — the shim this arm rides on.
	gatesDir, binDir, _, err := a.RenderGates("security", []string{"Bash(git push:*)", "Bash(git commit unless --)"})
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(binDir, "git"), gatesDir, filepath.Join(t.TempDir(), "gitdir")
}

// runSequencerShim runs the shim with the fake's knobs set, and returns
// stdout, stderr and the exit code.
func runSequencerShim(t *testing.T, shim, gitDir, leak, rc string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(shim, args...)
	cmd.Env = append(os.Environ(), "FAKE_GITDIR="+gitDir, "FAKE_LEAK="+leak, "FAKE_RC="+rc)
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return out.String(), errb.String(), code
}

// The bead's own case, and the wider one the measurement found: a sequencer
// verb that exits 0 with a blocker still in place is refused, loudly, with
// the recipe that ends the operation.
func TestQASequencerVerbExitingZeroWithoutEndingIsRefused(t *testing.T) {
	shim, gatesDir, gitDir := sequencerShim(t)

	for _, c := range []struct {
		args     []string
		leak     string
		blockers string
	}{
		{[]string{"cherry-pick", "--abort"}, "CHERRY_PICK_HEAD AUTO_MERGE", "CHERRY_PICK_HEAD"},
		{[]string{"cherry-pick", "--quit"}, "CHERRY_PICK_HEAD", "CHERRY_PICK_HEAD"},
		{[]string{"cherry-pick", "--continue"}, "CHERRY_PICK_HEAD", "CHERRY_PICK_HEAD"},
		// The case the bead did not name: a CLEAN cherry-pick leaks too, so
		// the arm must key on the verb and not on --abort.
		{[]string{"cherry-pick", "deadbeef"}, "CHERRY_PICK_HEAD", "CHERRY_PICK_HEAD"},
		{[]string{"revert", "--abort"}, "REVERT_HEAD", "REVERT_HEAD"},
		{[]string{"rebase", "--abort"}, "CHERRY_PICK_HEAD", "CHERRY_PICK_HEAD"},
		{[]string{"rebase", "--continue"}, "CHERRY_PICK_HEAD MERGE_HEAD", "CHERRY_PICK_HEAD MERGE_HEAD"},
	} {
		os.RemoveAll(gitDir)
		out, errs, code := runSequencerShim(t, shim, gitDir, c.leak, "0", c.args...)
		line := "git " + strings.Join(c.args, " ")
		if code != 1 {
			t.Errorf("%s: code=%d, want 1 (err=%q)", line, code, errs)
		}
		if !strings.Contains(out, "real git "+c.args[0]) {
			t.Errorf("%s: the real git must still run: out=%q", line, out)
		}
		for _, m := range strings.Fields(c.blockers) {
			if !strings.Contains(errs, m) {
				t.Errorf("%s: refusal must name the surviving %s: %q", line, m, errs)
			}
		}
		for _, want := range []string{
			"STILL IN PROGRESS",
			"ranger-base-71g2f",
			"cannot do a partial commit during a cherry-pick",
			"that is THIS, not your PID",
			"rm -rf --",
			filepath.Join(gitDir, strings.Fields(c.blockers)[0]),
		} {
			if !strings.Contains(errs, want) {
				t.Errorf("%s: refusal must carry %q: %q", line, want, errs)
			}
		}
	}

	// Every alarm is on the record, in the same log L1's refusals land in.
	logb, err := os.ReadFile(filepath.Join(gatesDir, "refusals.log"))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(logb), "(alarm: ranger-base-71g2f)"); n != 7 {
		t.Errorf("refusals.log must carry one alarm line per firing, got %d:\n%s", n, logb)
	}
	if !strings.Contains(string(logb), "git cherry-pick --abort exited 0 leaving CHERRY_PICK_HEAD") {
		t.Errorf("alarm line must name the verb and what survived:\n%s", logb)
	}
}

// The recipe removes the STALE state as well as the blocker, so one line ends
// the operation instead of leaving the seat to find AUTO_MERGE and the
// sequencer dir on its next commit.
//
// The wanted set is written out LITERALLY rather than read off
// sequencerLeftovers. Ranging over the variable under test made this pin
// agree with every edit to it — dropping AUTO_MERGE from the list dropped it
// from the assertion too, and the arm stayed green over a recipe that no
// longer ends the operation (caught in this bead's own mutation check, which
// is the only reason it is not still written that way).
func TestQASequencerRecipeNamesEveryLeftoverNotJustTheBlocker(t *testing.T) {
	shim, _, gitDir := sequencerShim(t)
	_, errs, code := runSequencerShim(t, shim, gitDir,
		"CHERRY_PICK_HEAD AUTO_MERGE MERGE_MSG sequencer", "0", "cherry-pick", "--abort")
	if code != 1 {
		t.Fatalf("code=%d err=%q", code, errs)
	}
	for _, m := range []string{"CHERRY_PICK_HEAD", "AUTO_MERGE", "MERGE_MSG", "sequencer"} {
		if p := filepath.Join(gitDir, m); !strings.Contains(errs, "'"+p+"'") {
			t.Errorf("recipe must name %s, quoted: %q", p, errs)
		}
	}
	// And only what is actually there: a path the operation did not leave
	// behind has no business in an `rm -rf` a seat is told to paste.
	for _, m := range []string{"REVERT_HEAD", "MERGE_HEAD"} {
		if p := filepath.Join(gitDir, m); strings.Contains(errs, p) {
			t.Errorf("recipe must not name the absent %s: %q", m, errs)
		}
	}
}

// AUTO_MERGE is not a blocker. A tree holding only AUTO_MERGE commits
// path-limited exactly as it always did (measured), so alarming on it would
// fire where nothing is wrong — which on a verb that leaves it behind on
// EVERY run would be an alarm nobody could act on.
func TestQASequencerAuditIgnoresAutoMergeAlone(t *testing.T) {
	shim, gatesDir, gitDir := sequencerShim(t)
	out, errs, code := runSequencerShim(t, shim, gitDir, "AUTO_MERGE", "0", "cherry-pick", "--abort")
	if code != 0 || strings.Contains(errs, "ranger-base-71g2f") {
		t.Errorf("AUTO_MERGE alone must not alarm: code=%d out=%q err=%q", code, out, errs)
	}
	if _, err := os.Stat(filepath.Join(gatesDir, "refusals.log")); err == nil {
		t.Error("a quiet run must write no alarm line")
	}
}

// git's own failure is git's to report. The arm fires only on a ZERO exit —
// a sequencer verb that STOPS (a conflict, a rebase pausing at `edit`) exits
// nonzero with a blocker legitimately in place, and that is not this defect.
func TestQASequencerAuditPassesAFailureThroughUntouched(t *testing.T) {
	shim, _, gitDir := sequencerShim(t)
	for _, rc := range []string{"1", "128"} {
		os.RemoveAll(gitDir)
		_, errs, code := runSequencerShim(t, shim, gitDir, "CHERRY_PICK_HEAD", rc, "cherry-pick", "deadbeef")
		if code != map[string]int{"1": 1, "128": 128}[rc] {
			t.Errorf("rc %s: git's own exit code must survive, got %d", rc, code)
		}
		if strings.Contains(errs, "ranger-base-71g2f") {
			t.Errorf("rc %s: a nonzero exit is not this defect: %q", rc, errs)
		}
	}
}

// The audit asks rev-parse about the repo the VERB ran against, so a leading
// global option that takes a separate value cannot point it at the wrong git
// dir — the same table posse_verb_match consumes (globalValueOpts).
func TestQASequencerAuditFollowsTheGlobalOptionsToTheRepo(t *testing.T) {
	shim, _, gitDir := sequencerShim(t)
	elsewhere := t.TempDir()
	for _, args := range [][]string{
		{"-C", elsewhere, "cherry-pick", "--abort"},
		{"-c", "core.editor=true", "-C", elsewhere, "cherry-pick", "--abort"},
		{"--no-pager", "-C", elsewhere, "cherry-pick", "--abort"},
	} {
		os.RemoveAll(gitDir)
		_, errs, code := runSequencerShim(t, shim, gitDir, "CHERRY_PICK_HEAD", "0", args...)
		want := filepath.Join(elsewhere, ".git")
		if code != 1 || !strings.Contains(errs, want) {
			t.Errorf("git %s: audit must read %s: code=%d err=%q",
				strings.Join(args, " "), want, code, errs)
		}
	}
	// And a global option consuming its value cannot be mistaken for the verb.
	os.RemoveAll(gitDir)
	if _, errs, code := runSequencerShim(t, shim, gitDir, "", "0", "-C", "cherry-pick", "status"); code != 0 || errs != "" {
		t.Errorf("`git -C cherry-pick status` is a status: code=%d err=%q", code, errs)
	}
}

// Narrow by construction: only the three verbs that end by deleting a
// pseudo-ref, and only the `git` shim.
func TestQASequencerAuditLeavesEverythingElseAlone(t *testing.T) {
	shim, _, gitDir := sequencerShim(t)
	// merge and am were MEASURED clean — merge unlinks MERGE_HEAD out of
	// `reset --merge` and am ends by removing a directory, neither through
	// the ref transaction that needs packed-refs.lock.
	// `commit` is absent on purpose: the PID's own deny refuses the
	// unqualified form before the audit is reached, which the last arm here
	// pins directly.
	for _, verb := range []string{"merge", "am", "status", "log", "stash"} {
		os.RemoveAll(gitDir)
		out, errs, code := runSequencerShim(t, shim, gitDir, "CHERRY_PICK_HEAD MERGE_HEAD", "0", verb, "--abort")
		if code != 0 || strings.Contains(errs, "ranger-base-71g2f") {
			t.Errorf("git %s must pass through: code=%d err=%q", verb, code, errs)
		}
		if !strings.Contains(out, "real git "+verb) {
			t.Errorf("git %s must reach the real binary: %q", verb, out)
		}
	}
	// A non-git shim carries no arm at all, and neither does a git shim with
	// no real binary to ask (the runtime PATH search has no name to run).
	if s := renderSequencerAudit("bd", "/usr/local/bin/bd", "/bin/date"); s != "" {
		t.Errorf("the arm is git's alone: %q", s)
	}
	if s := renderSequencerAudit("git", "", "/bin/date"); s != "" {
		t.Errorf("no real binary, no arm: %q", s)
	}
	// The deny rules the arm rides on still refuse first: the audit is
	// appended AFTER posse_verb_match, so a refused push never reaches it.
	if _, errs, code := runSequencerShim(t, shim, gitDir, "", "0", "push", "origin", "main"); code != 1 ||
		!strings.Contains(errs, "refused by posse gate: git push origin main") {
		t.Errorf("the deny arm must still fire first: code=%d err=%q", code, errs)
	}
}

// trimPathPunct strips the sentence punctuation the refusal's PROSE leaves on
// the end of a path — `... survives in <dir>.` — without eating a trailing `.`
// or `..` PATH element. `<own>/..` is the shared `worktrees` dir; trimming it
// to `<own>` would hand the two scans below the blind spot ranger-base-f6pt2
// found, in the one spelling filepath.Clean cannot then undo.
func trimPathPunct(s string) string {
	for len(s) > 0 {
		c := s[len(s)-1]
		if c != ',' && c != ';' && c != ':' && c != '.' {
			break
		}
		if c == '.' && (strings.HasSuffix(s, "/.") || strings.HasSuffix(s, "/..")) {
			break
		}
		s = s[:len(s)-1]
	}
	return s
}

// pathSpanAt returns the path span that begins at s[i], where s is ONE line of
// the refusal. Where a path ENDS depends on how it is quoted, and that is the
// whole of what ranger-base-1h6d6 corrected: both scans below used to end a
// span at the first space — one by `strings.Fields`, the other by
// `strings.IndexAny(..., "'\" \n")` — and strip the quoting only afterwards,
// so the quoting they acknowledged was not the quoting they read. The recipe
// single-quotes every path precisely because a path may contain a space, so
// `'<own>/a b/../../../index'` was read as `'<own>/a` — absolute, under `own`,
// passing — plus `b/../../../index'`, which has no leading `/` and was skipped
// unexamined; the path the line actually names is the SHARED `<common>/index`,
// and both scans stayed green over a recipe deleting the operator's index.
//
// So: a span opened by a quote runs to its CLOSING quote, spaces included, and
// any other span ends at the first whitespace or quote. The test is the
// character in FRONT of the span rather than a quote-state machine over the
// line, because the refusal's prose carries apostrophes — `git's delete of
// that pseudo-ref`, `the SHARED repo's`, `the operator's to remove` — and a
// state machine reads those as opening quotes and welds prose onto whatever
// path follows. An unterminated quote runs to the end of the line, which is
// the conservative reading: the tail is then one span and every `..` element
// in it is resolved rather than dropped.
//
// The second return, and the refusal that hangs off it, is ranger-base-yplnv,
// and it is meant to end a series rather than add one more spelling to it.
// u18bo taught these scans about a bare token, f6pt2 about a traversal, 1h6d6
// about a quoted space; each was right about the spelling in front of it, and
// each left the next one open, because deciding where a path ends from the
// character in front of it alone is a shell parser written one escape at a
// time. Two spellings were still open after 1h6d6, both naming the SHARED
// `<common>/index`:
//
//	<own>/a\ b/../../../index      the space is BACKSLASH-escaped, so the
//	                               unquoted span ends at it — backslash is not
//	                               a quote — and the tail `b/../../../index`
//	                               has no `/` at a word boundary for either
//	                               scan to examine
//	'<own>/a'\''b/../../../index'  the shell's own splice for an apostrophe:
//	                               the closing-quote search ends the span at
//	                               the splice's first quote, and the same tail
//	                               goes unread
//
// So the scans stop guessing where such a span ends and REPORT it instead. A
// span is readable only when it carries no backslash at all and the shell word
// ends where the span does; an unreadable one is reported as it was written,
// not resolved, because its resolved form is a fiction. This refuses more than
// it has to — `'<own>/a\ b/MERGE_MSG'` is a literal path inside `own`, since a
// backslash inside single quotes is not an escape — and that is the trade:
// telling those two apart is exactly the shell parser this is not. The
// rendered recipe has never held either spelling (every path is
// `'$posse_sg/$posse_sm'`, a git pseudo-ref with no space, no backslash and no
// `..`), so what the refusal costs is that a recipe which grows one gets read
// by a person, which is what this pin is for. It fails LOUD, in the same
// direction as the unquoted prose path the ADR already records.
func pathSpanAt(s string, i int) (string, bool) {
	if i > 0 && (s[i-1] == '\'' || s[i-1] == '"') {
		if k := strings.IndexByte(s[i:], s[i-1]); k >= 0 {
			// +1 for the closing quote: the word ends after it, not at it.
			return s[i : i+k], spanIsWholePath(s[i:i+k], s, i+k+1)
		}
		return s[i:], spanIsWholePath(s[i:], s, len(s))
	}
	if k := strings.IndexAny(s[i:], "'\" \t"); k >= 0 {
		return s[i : i+k], spanIsWholePath(s[i:i+k], s, i+k)
	}
	return s[i:], spanIsWholePath(s[i:], s, len(s))
}

// spanIsWholePath reports whether span, which ends at s[end], is the whole
// path the shell would read there: no backslash in it, and the shell WORD ends
// where the span does. A span that ends at a quote or any other non-blank is a
// fragment of a longer word — the apostrophe splice above, and a bare path
// with a quoted part spliced onto its end — and what follows it is part of the
// same path.
func spanIsWholePath(span, s string, end int) bool {
	if strings.ContainsRune(span, '\\') {
		return false
	}
	return end >= len(s) || s[end] == ' ' || s[end] == '\t'
}

// insideOwn reports whether the RESOLVED path q is the session's own git dir
// or something under it. `own` is `<common>/worktrees/<name>`, so every
// correct recipe names paths that are textually under the common dir; what D3
// forbids is the rest of it.
func insideOwn(q, own string) bool {
	return q == own || strings.HasPrefix(q, own+string(os.PathSeparator))
}

// sequencerRecipeWords is the closed list of NON-PATH, non-option words an
// `rm` line of the refusal may hold. The rendered recipe holds exactly one —
// `echo "    rm -rf --$posse_srm"`, whose every operand is a quoted
// `'$posse_sg/$posse_sm'`. A word added here is a word the scan below stops
// reading, so the reason belongs beside it.
var sequencerRecipeWords = map[string]bool{"rm": true}

// isRecipeRemovalLine reports whether this line of the refusal is a removal
// instruction, and it is the gate in FRONT of the word walk below — the
// decision about which lines get read at all.
//
// By FIELDS, not by the substring `"rm "`. The substring is a literal `rm`
// followed by a SPACE, so `rm\t-rf\t--\t../../index` holds no such
// substring and the walk below never ran on it: the one class of operand for
// which the removal scan is the ONLY reader — relative, so it carries the
// common dir nowhere for refusalSpansOutside to find — escaped on the
// separator alone (ranger-base-yhih9). The walk itself has always skipped
// ' ' and '\t' alike; only this gate was space-bound. Five beads before it
// asked what a path LOOKS like; this one is whether the line holding it is
// read.
//
// strings.Fields, so every blank run separates and the test is membership in
// the same closed vocabulary the walk uses. Over-selection is the safe
// direction: a line selected in error is WALKED, and a walk of a line whose
// every word resolves inside `own` is silent. `"rm "` over-selected too, and
// worse — it matches inside `confirm `.
func isRecipeRemovalLine(line string) bool {
	for _, w := range strings.Fields(line) {
		if sequencerRecipeWords[w] {
			return true
		}
	}
	return false
}

// recipeRemovalsOutside returns every word of an `rm` line of the refusal that
// this reader cannot account for as a path inside `own` — resolved where the
// word is an absolute path it can read, and quoted as written where it is not.
//
// The vocabulary is a WHITELIST, and that is the whole difference from the four
// beads in front of it. ranger-base-u18bo, -f6pt2, -1h6d6 and -yplnv each asked
// "where does this path END", and each was one spelling whose answer came back
// wrong — a bare path, a traversal wearing `own` as a prefix, a space inside
// quotes, a backslash and an apostrophe splice. A word that does not begin with
// `/` asks a different question — "is this a path at all" — and a scan whose
// spans start at a `/` never looks at it. So `rm -rf -- ../../index`, from a
// worktree's own git dir, IS the shared `<common>/index` and was reported by
// NEITHER scan: silence over a line that names shared state, which is the one
// direction the other four never failed in (ranger-base-rg19l). Every spelling
// they added fails loud; this one did not fail at all.
//
// Enumerating spellings is what kept losing, so this no longer does. An `rm`
// line may hold an option word, a word in sequencerRecipeWords, or an absolute
// path this reader can read and resolve under `own`. Anything else is reported
// as written, unread. `cd` and `&&` are not on the list, so a recipe that grows
// a subshell prefix is read by a person too — that is the trade, and it is the
// same trade ranger-base-yplnv made one axis over: refuse rather than resolve,
// on a recipe that has never held either spelling.
//
// Words, not `/`-spans, so a quoted operand is one word: a word runs to the
// next blank, unless it OPENS with a quote, in which case it runs to the
// closing one. pathSpanAt reads the span inside it and says whether that span
// is the whole word; an unreadable word ends the line, because what follows it
// belongs to a path this reader cannot read. This replaces a `strings.Fields`
// walk that read the odd fields between `'`s (no check at all for a recipe that
// prints a BARE path — the split yields nothing and every token goes unexamined,
// ranger-base-u18bo) and then, once tokenising on whitespace, could not read a
// quoted path with a space in it at all (ranger-base-1h6d6).
func recipeRemovalsOutside(errs, own string) []string {
	var out []string
	for _, line := range strings.Split(errs, "\n") {
		if !isRecipeRemovalLine(line) {
			continue
		}
		for i := 0; i < len(line); {
			if line[i] == ' ' || line[i] == '\t' {
				i++
				continue
			}
			// The span starts just INSIDE an opening quote, which is the
			// index pathSpanAt reads a quoted path at.
			start := i
			if line[i] == '\'' || line[i] == '"' {
				start++
			}
			span, whole := pathSpanAt(line, start)
			next := start + len(span)
			if start > i && next < len(line) {
				next++ // the closing quote; the word ends after it, not at it
			}
			word := line[i:next]
			i = next
			// Unreadable is reported, not resolved: where the span is a
			// fragment of a longer word the resolved form names a path the
			// line does not (ranger-base-yplnv). The REST of the line is that
			// same unread path, so the reading of this line stops here.
			if !whole {
				out = append(out, span)
				break
			}
			if strings.HasPrefix(word, "-") || sequencerRecipeWords[word] {
				continue
			}
			// Not an absolute path, so not a path this reader can resolve at
			// all: `../../index` and `sequencer/../../../index` both name the
			// shared part from a worktree's own git dir, and neither can be
			// told from a shell word by anything short of a parser. Reported
			// as the WHOLE word, quotes included — the seat has to find it in
			// the refusal, and an empty operand (`''`) has no span to print.
			if !strings.HasPrefix(span, "/") {
				out = append(out, word)
				continue
			}
			// Resolved, not compared raw: `<own>/../../index` wears `own` as
			// a prefix and IS `<common>/index` — the shared index — so a
			// string prefix test reads the traversal as inside the private
			// subtree and passes a recipe that deletes shared state
			// (ranger-base-f6pt2).
			if q := filepath.Clean(trimPathPunct(span)); !insideOwn(q, own) {
				out = append(out, q)
			}
		}
	}
	return out
}

// refusalSpansOutside returns every span of the refusal — recipe or prose,
// quoted or bare — that names the fixture's common dir and does not resolve
// under `own`. Stated over the common dir itself rather than as a list of
// names under it: everything else beneath it is by definition the shared part.
func refusalSpansOutside(errs, own, common string) []string {
	var out []string
	for _, line := range strings.Split(errs, "\n") {
		for off := 0; off < len(line); {
			j := strings.Index(line[off:], common)
			if j < 0 {
				break
			}
			j += off
			// Take the whole path-looking span and RESOLVE it before
			// deciding, rather than skipping past a bare `own` prefix: the
			// skip walked over the `own` in `<own>/../../index` and read the
			// `../..` that followed as the next thing to look for `common`
			// in, which it is not (ranger-base-f6pt2).
			named, whole := pathSpanAt(line, j)
			off = j + len(named)
			if named == "" {
				off++
				continue
			}
			if !whole {
				out = append(out, named)
				continue
			}
			if q := filepath.Clean(trimPathPunct(named)); insideOwn(q, own) {
				continue
			}
			out = append(out, named)
		}
	}
	return out
}

// ADR 0059 D3: the recipe a seat is told to paste names the pseudo-refs in
// the session's OWN git dir and nothing in the shared part of the common dir
// — above all not the `packed-refs.lock` that is now the only thing standing
// between the seat and a finished cherry-pick. Git's lock files carry no
// holder identity and no expiry, so a session cannot tell a stranded lock
// from the one a live `gc` is holding; an `rm` on that guess is the
// destructive act the wall exists to prevent, and removing it is the
// operator's.
//
// "Under the common dir" cannot be a substring test, and that is why this arm
// is shaped the way it is. A linked worktree's own git dir IS
// `<common>/worktrees/<name>` (MEASURED on this box, git 2.50.1: `rev-parse
// --absolute-git-dir` answers it and `--git-common-dir` answers the dir two
// levels up), so every CORRECT recipe names paths under the common dir
// textually. What D3 forbids is the SHARED part — the common dir outside that
// private subtree — and the fixture below is worktree-shaped so the two
// answers differ and the distinction is measurable at all.
func TestQASequencerRecipeStaysOutOfTheCommonDir(t *testing.T) {
	shim, _, _ := sequencerShim(t)

	// The two structural halves run FIRST, so a change that stops the arm
	// firing at all still reports WHY here rather than only that it went
	// quiet. The first is structural because outside a worktree-shaped git dir
	// the two rev-parse answers are the same string, so no execution can tell
	// them apart.
	rendered := renderSequencerAudit("git", "/usr/bin/git", "/bin/date")
	if !strings.Contains(rendered, "rev-parse --absolute-git-dir") {
		t.Error("the audit no longer asks rev-parse for the session's own git dir")
	}
	if strings.Contains(rendered, "--git-common-dir") {
		t.Error("the audit asks git for the COMMON dir, the one place ADR 0059 D3 says its recipe may not point")
	}
	// The lists the recipe is built from hold this session's leftovers only.
	for _, m := range append(append([]string{}, sequencerLeftovers...), sequencerBlockers...) {
		if strings.Contains(m, "packed-refs") {
			t.Errorf("%q is shared state, not a leftover of this session's operation", m)
		}
	}

	common := filepath.Join(t.TempDir(), ".git")
	own := filepath.Join(common, "worktrees", "w1")
	stray := filepath.Join(common, "packed-refs.lock")
	if err := os.MkdirAll(common, 0o755); err != nil {
		t.Fatal(err)
	}
	// Case 1's cause, sitting in plain sight for a shim that decided to tidy.
	if err := os.WriteFile(stray, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	// The SHARED index, which every real repo has and this fixture did not.
	// It is here so the traversal ranger-base-f6pt2 found is reachable by
	// EXECUTION and not only by reading: the recipe names what it finds, so a
	// `../../index` added to sequencerLeftovers prints only if the file it
	// resolves to exists.
	shared := filepath.Join(common, "index")
	if err := os.WriteFile(shared, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	// A lock in the session's OWN git dir as well, which real git never puts
	// there — deliberately, so the mutation this arm exists to catch is
	// REACHABLE by execution and not only by reading the list. The recipe
	// names what it finds, so a `packed-refs.lock` added to sequencerLeftovers
	// prints only if there is one to find.
	_, errs, code := runSequencerShim(t, shim, own,
		"CHERRY_PICK_HEAD AUTO_MERGE MERGE_MSG sequencer packed-refs.lock", "0", "cherry-pick", "--abort")
	if code != 1 {
		t.Fatalf("the arm did not fire, so this pin is reading nothing: code=%d err=%q", code, errs)
	}
	// The positive half first: it does still end the operation, in the dir
	// that is the session's own. An arm that only asserts absence is happy
	// with a recipe that names nothing at all.
	if want := "'" + filepath.Join(own, "CHERRY_PICK_HEAD") + "'"; !strings.Contains(errs, want) {
		t.Fatalf("recipe does not name the blocker in the session's own git dir (%s): %q", want, errs)
	}
	// Not by path alone: a `packed-refs.lock` the recipe reached by any
	// spelling is the removal D3 forbids.
	for _, line := range strings.Split(errs, "\n") {
		if isRecipeRemovalLine(line) && strings.Contains(line, "packed-refs.lock") {
			t.Errorf("the recipe tells a seat to remove a packed-refs.lock (ADR 0059 D3): %q", line)
		}
	}
	// Every WORD of every removal line is an option, a word in
	// sequencerRecipeWords, or a path inside that dir — quoted or not, and
	// with a space in it or without. Stated over the whole word rather than
	// over the paths in it because a word that is not a path AT ALL is what
	// escaped the bead before it: a relative operand names no `/` for a span to
	// start at, so it went unread by both scans while naming shared state
	// (ranger-base-rg19l). Where the scan cannot tell WHERE a path ends it
	// reports the span rather than resolving it, so a recipe that grows shell
	// quoting this reader does not do lands here too (ranger-base-yplnv);
	// pathSpanAt says which spellings those are.
	//
	// "Every removal line" is isRecipeRemovalLine's, and that gate is the
	// sixth axis, one step in front of the vocabulary: while the line was
	// selected by the substring `"rm "` this sentence was false for any line
	// whose argv is TAB-separated, which the walk below would have read
	// correctly had it been handed one (ranger-base-yhih9).
	for _, q := range recipeRemovalsOutside(errs, own) {
		t.Errorf("the recipe tells a seat to remove %q, outside its own git dir %s (ADR 0059 D3): %q", q, own, errs)
	}
	// And the shared part of the common dir is not named anywhere in the
	// refusal, quoted or not, recipe or prose — the stray lock included.
	//
	// Stated over the common dir itself rather than as a list of names under
	// it: every occurrence of `common` in the refusal has to RESOLVE to a path
	// under `own`, and anything else is by definition the shared part. The fixed
	// five-name list this replaces (the stray lock, `packed-refs`,
	// `packed-refs.new`, `refs`, `HEAD`) saw none of `index`, `logs/`,
	// `config`, `objects/` or a sibling `worktrees/<other>` — ranger-base-u18bo.
	if named := refusalSpansOutside(errs, own, common); len(named) > 0 {
		t.Errorf("the refusal names %s, in the common dir but outside this session's own git dir %s — the operator's to touch and not this session's (ADR 0059 D3): %q", named[0], own, errs)
	}
	// It audits, it does not repair: the stray lock and the shared index are
	// both still there.
	if _, err := os.Stat(stray); err != nil {
		t.Errorf("the stray lock is gone — the audit removed shared state on a guess: %v", err)
	}
	if _, err := os.Stat(shared); err != nil {
		t.Errorf("the shared index is gone — the audit removed shared state on a guess: %v", err)
	}
}

// The escapes ranger-base-1h6d6 and ranger-base-yplnv found, pinned on the two
// scans themselves. The arm above can only read what renderSequencerAudit
// prints TODAY, and the recipe on main is correct — every name a git pseudo-ref
// with no space, no backslash and no `..`. What failed was the READING, for
// spellings the recipe does not have yet: `'<own>/a b/../../../index'` is the
// shared `<common>/index`, and both scans passed it because both ended the path
// at the first space (1h6d6); then `<own>/a\ b/../../../index` and the
// apostrophe splice passed for the same reason one layer down, because a
// backslash is not a quote and a spliced quote is not the end of a word
// (yplnv). So the cases below are lines the refusal could grow on the next
// change to it, handed straight to the scans.
//
// `out` is what the refusal must be REPORTED as, and after yplnv it has three
// grounds: the line names something outside `own` once resolved, the scans
// cannot read where its paths end and refuse it unread, or an `rm` line holds
// a word the removal scan cannot account for as a path inside `own` at all
// (ranger-base-rg19l). The distinction is in pathSpanAt's and
// recipeRemovalsOutside's comments and the case names say which one each row
// is; what matters here is that no such line goes unreported.
//
// `out` was BOTH scans until ranger-base-rg19l, and rmScanOnly is where that
// stopped being true. refusalSpansOutside is defined over occurrences of the
// COMMON DIR in the refusal, prose included, and a purely relative operand
// carries the common dir nowhere — so nothing about widening it would let it
// see one. Widening it to read relative spans in PROSE is the opposite of a
// fix: the two prose rows at the foot of this table each hold a `../../index`
// that resolves out of `own` and is an innocent sentence, so a scan that read
// them would red the refusal for saying what a seat may not do. The removal
// scan is therefore the only reader of that class, and it is the one whose
// line is an instruction to delete rather than a sentence.
func TestQASequencerScansEndAPathAtItsQuoteNotAtASpace(t *testing.T) {
	t.Parallel()

	common := filepath.Join("/tmp", "fixture", ".git")
	own := filepath.Join(common, "worktrees", "w1")

	for _, c := range []struct {
		name string
		line string
		// out is what the line must be reported as: true when it names
		// something outside `own` once resolved, is refused unread, or holds a
		// word the removal scan cannot read as a path inside `own`; false when
		// it is the session's own and readable.
		out bool
		// rmScanOnly: the offending word is RELATIVE, so the line names the
		// common dir nowhere and only recipeRemovalsOutside can see it.
		rmScanOnly bool
	}{
		{"the recipe on main", "    rm -rf -- '" + own + "/CHERRY_PICK_HEAD' '" + own + "/sequencer'", false, false},
		{"a quoted path with a space, still inside", "    rm -rf -- '" + own + "/a b/MERGE_MSG'", false, false},
		{"THE ESCAPE: a quoted path with a space that traverses out", "    rm -rf -- '" + own + "/a b/../../../index'", true, false},
		{"the same, quote never closed", "    rm -rf -- '" + own + "/a b/../../../index", true, false},
		{"a quoted traversal with no space (ranger-base-f6pt2)", "    rm -rf -- '" + own + "/../../index'", true, false},
		{"a bare traversal", "    rm -rf -- " + own + "/../../HEAD", true, false},
		{"a bare path, inside", "    rm -rf -- " + own + "/AUTO_MERGE", false, false},
		{"a trailing `..`, which is the shared worktrees dir", "    rm -rf -- '" + own + "/..'", true, false},
		{"a space and a traversal clean out of the repo", "    rm -rf -- '" + own + "/a b/../../../../etc/hosts'", true, false},
		// ranger-base-yplnv: the two spellings that end a path somewhere the
		// character in FRONT of the span cannot see. Each names the SHARED
		// `<common>/index`, and each was read as a path inside `own` plus a
		// tail with no `/` at a word boundary for anything to examine.
		{"THE ESCAPE: a backslash-escaped space that traverses out", "    rm -rf -- " + own + "/a\\ b/../../../index", true, false},
		{"THE ESCAPE: the shell's own splice for an apostrophe", "    rm -rf -- '" + own + "/a'\\''b/../../../index'", true, false},
		// And the two neighbouring spellings that were already caught, here
		// so the fix for the two above cannot be a widening that loses them.
		{"a backslash-escaped space inside quotes, traversing out", "    rm -rf -- '" + own + "/a\\ b/../../../index'", true, false},
		{"a double-quoted path holding an apostrophe, traversing out", "    rm -rf -- \"" + own + "/a'b/../../../index\"", true, false},
		// A backslash that resolves INSIDE own is refused all the same: a
		// scan that decided this one would have to know that a backslash is
		// literal inside single quotes and an escape outside them, which is a
		// shell parser. The rendered recipe has never carried a backslash, so
		// refusing is the cheap end and it fails loud.
		{"a backslash inside own is refused unread, not resolved", "    rm -rf -- '" + own + "/a\\ b/MERGE_MSG'", true, false},
		// Prose, where the apostrophes live. A quote-STATE machine reads
		// `operator's` as an opening quote and welds the rest of the sentence
		// onto the path that follows — `../../index` in the prose after it
		// would then resolve out of `own` and red an innocent line. Reading
		// the character in front of the span instead ends this one at the
		// space, which is the correct reading of an unquoted path.
		{"prose: an apostrophe before the path does not open a quote", "  the operator's " + own + "/sequencer, not ../../index — say so on your bead.", false, false},
		{"prose: a path ending a sentence keeps its dot out of the path", "  CHERRY_PICK_HEAD survives in " + own + ".", false, false},
		// ranger-base-rg19l: the axis none of the four before it touched — the
		// operand is not an absolute path, so there is no span for a scan that
		// starts at a `/` to read, and the line was reported by NEITHER scan.
		// Each of these three resolves to the SHARED index from `own`.
		{"a relative traversal out of own", "    rm -rf -- ../../index", true, true},
		{"a relative traversal with cd in front", "    cd '" + own + "' && rm -rf -- ../../index", true, true},
		{"a relative traversal through a leftover name", "    rm -rf -- sequencer/../../../index", true, true},
		// ranger-base-yhih9: the sixth axis, which is not a spelling of a path
		// but the gate that decides whether the line is read. While that gate
		// was the substring `"rm "` — a literal `rm` and a SPACE — every row
		// below reported false, because the walk that reads them was never
		// reached. The tab is doing all the work: each of these is a row above
		// with its blanks respelled.
		{"THE ESCAPE: a tab-separated relative traversal", "    rm\t-rf\t--\t../../index", true, true},
		{"the same, no leading indent", "rm\t-rf\t--\t../../index", true, true},
		{"one tab after the verb, the rest spaces", "    rm\t-rf -- ../../index", true, true},
		// And the two neighbouring tab spellings, so a fix for the three above
		// cannot be a widening that loses them. An ABSOLUTE operand still
		// carries the common dir, so the span scan sees it whatever the
		// separator — the gap was exactly tab AND relative.
		{"a tab-separated absolute traversal (both scans)", "    rm\t-rf\t--\t'" + own + "/../../index'", true, false},
		{"a tab-separated path inside own", "    rm\t-rf\t--\t'" + own + "/MERGE_MSG'", false, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			rm := recipeRemovalsOutside(c.line, own)
			named := refusalSpansOutside(c.line, own, common)
			// UNGATED, deliberately. This read `if
			// strings.Contains(c.line, "rm ") && ...`, which is the scan's
			// own line filter — so a row the scan wrongly declined to read
			// was a row this assertion also declined to make, and the table
			// went green over exactly the class it was added to catch. A
			// gate shared with the function under test makes a row inert
			// precisely when the function is wrong (ranger-base-yhih9).
			// Nothing needs one: every out=false row is either a removal
			// line whose words all resolve inside `own` or prose holding no
			// verb, and the scan reports nothing for both.
			if (len(rm) > 0) != c.out {
				t.Errorf("recipeRemovalsOutside(%q) = %q, want outside=%v", c.line, rm, c.out)
			}
			if want := c.out && !c.rmScanOnly; (len(named) > 0) != want {
				t.Errorf("refusalSpansOutside(%q) = %q, want outside=%v", c.line, named, want)
			}
		})
	}

	// And a relative word is reported AS WRITTEN rather than resolved, which
	// is the reason recipeRemovalsOutside has an arm for it instead of letting
	// one fall through to filepath.Clean. Clean answers `../../index` for the
	// word below — a path the line does not name, from a base the scan does
	// not know — and printing it would be the same fiction ranger-base-yplnv
	// refused to print for an unreadable span. The seat reading the refusal
	// has to find the word in it.
	t.Run("a relative word is reported as written", func(t *testing.T) {
		t.Parallel()
		const word = "sequencer/../../../index"
		got := recipeRemovalsOutside("    rm -rf -- "+word, own)
		if len(got) != 1 || got[0] != word {
			t.Errorf("recipeRemovalsOutside reported %q, want exactly [%q]", got, word)
		}
	})
}
