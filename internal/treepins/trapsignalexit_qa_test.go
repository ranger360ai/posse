package treepins

// QA pin for ranger-base-kkp5g — a trap handler registered on a catchable
// DEADLY signal ends the shell; it never returns.
//
// THE DEFECT. `trap cleanup EXIT INT TERM`, with a `cleanup` that returns.
// On EXIT that is right and it is the whole of what the handler is for. On
// INT or TERM it is not: bash runs the handler and then RESUMES the script at
// the statement after the one the signal interrupted, with whatever the
// handler just removed already gone. The script carries on over a torn-down
// apparatus and reports what it finds there.
//
// MEASURED 2026-11-13 (linux 7.2.15 x86_64, bash 5.3.20, this box; the rig
// and its transcripts are in docs/notes.d/ranger-base-kkp5g.md). The shape
// under test, signalled 1s into a 5s first arm, on both signals:
//
//	trap cleanup EXIT INT TERM      kill -TERM   cleanup ran, $T gone, arm 2
//	                                             REACHED, printed its FAIL,
//	                                             exit 1
//	                                kill -INT    identical
//	trap cleanup EXIT               kill -TERM   cleanup ran once, arm 2 never
//	trap 'exit 130' INT                          reached, exit 143
//	trap 'exit 143' TERM            kill -INT    the same, exit 130
//
// and the same thing on the script the bead was found on. A `kill -TERM` of
// scripts/verify-herdr-bob-rules.sh during its seeding arms ran `cleanup`,
// which `rm -rf "$T"`-ed the scratch dir, and the run then carried straight
// on into the second arm, re-created $T by mkdir, and printed
//
//	FAIL fresh seeding: watcher read 'nothing', seeded file does NOT match …
//	FAIL override not read: watcher read 'nothing', and the file was OVERWRITTEN
//	verify-herdr-bob-rules: 2 check(s) failed
//
// with exit 1. NEITHER PROPERTY WAS FALSE. Both were unmeasured, because the
// apparatus had been torn down by this script's own handler — and that is the
// exact failure that script is built not to make: its `apparatus()` and
// `harness()` exist so a signalled or un-exec-able tool is never reported as
// a false property (ranger-base-vf3pf). A signalled RUN reporting two red
// verdicts is that sentence one level up, and the one arm that cannot say
// "that was the apparatus" is the trap.
//
// WHY A PIN AND NOT A SED. Not all members cost the same, which is the whole
// reason this is a roster with prose and not a rewrite. A script whose
// remaining statements do nothing after the handler loses only an exit
// status. And one member must NOT be changed: see tsgExempt, where
// scripts/bd-argv-gate.sh is a PreToolUse hook whose exit status IS its
// verdict, and for which every "exits on the signal" spelling is strictly
// worse than resuming. The rule had to be able to say that, which a sed
// cannot.
//
// THE RULE. In a script under scripts/, a `trap` registration that names a
// catchable deadly signal (tsgDeadly — HUP, INT, QUIT, TERM and the
// USR/ALRM/PIPE family, by name or by number, with or without `SIG`) has a
// handler that cannot RETURN. Two spellings satisfy it and both are already
// in the tree:
//
//   - an `exit`, reached in command position in the handler's body.
//     scripts/queue-cutover.sh:405-408 is the one to copy — the EXIT trap
//     keeps the bare handler and does the work, and each signal's own trap
//     only `exit`s with that signal's 128+signo, which then runs the EXIT
//     trap. The handler runs ONCE and the status names the signal.
//     scripts/reap-selftest-ambient.sh:129 is the older spelling,
//     `trap 'cleanup; exit 130' INT TERM`: also correct, at the cost of
//     running the handler twice (MEASURED: once on the signal and once on
//     the EXIT its own `exit` triggers — harmless for an idempotent handler,
//     which all six under scripts/ are) and of reporting 130 for a TERM.
//   - a reset-and-re-raise, `trap - SIG; kill -SIG $$`, which is
//     scripts/test-times.sh:1674-1675. That one is not interchangeable with
//     an `exit`: it exists so the WAIT STATUS of the wrapper is a real
//     signal death and not an ordinary exit with a signal-shaped number,
//     which is what its own reader measures.
//
// `trap '' SIG` (ignore) and `trap - SIG` (reset) run no code at all, so
// neither can tear an apparatus down and resume; the shared reader drops both
// before this rule sees them.
//
// EXIT, ERR, DEBUG AND RETURN ARE NOT IN SCOPE, and the reason is the rule
// rather than convenience: they are the shell's own pseudo-signals, where
// running the handler and carrying on IS the contract. An ERR handler that
// exited would turn every `||`-guarded probe into a fatal, and
// scripts/verify-id-recycle.sh:28 and its two siblings are exactly that
// shape — an ERR trap that prints an apparatus line and lets the script's own
// arms decide. KILL and STOP are not in scope either: bash refuses to trap
// them, so there is no handler to have an opinion about.
//
// THE CENSUS, MEASURED 2026-11-14 (ranger-base-5owgm, which gave the four
// splits that lacked it a HUP arm): 48 shell files of the 64 under scripts/,
// 53 trap registrations between them, 22 on a deadly signal, 21 of those
// ending the shell and ONE exempted — which is the whole of this rule's
// negative side, and why the roster carries prose. Of the 21: fifteen are
// ranger-base-kkp5g's five splits as this bead completed them — three each
// in clean-build.sh, release-artifacts.sh, verify-herdr-bob.sh,
// verify-herdr-bob-rules.sh and verify-gate-freshness.sh, INT/TERM/HUP — and
// six were already right (queue-cutover.sh's three,
// reap-selftest-ambient.sh's one, test-times.sh's two re-raises).
// MEASURED 2026-11-13, the same census before those four HUP arms: 49
// registrations, 18 on a deadly signal, 17 ending the shell.
//
// IT SHARES ranger-base-wzo36'S READER. trapstatepick_qa_test.go already
// lexes every script under scripts/ — logical lines, function extents,
// command position, subshell depth — to ask which shell a trap-state setter
// runs in. tsRead records every registration with its signal list, and this
// pin reads tsScript.Traps; there is no second walk of scripts/, because a
// second walk is a second answer to "what does this script trap" and the two
// could come to disagree about it.
//
// AND SHARING IT TURNED UP A HOLE IN IT, which is the third arm below. That
// reader was dropping the TAIL of four scripts — 908 physical lines,
// verify-box.sh from :271 to :898 — because a heredoc opened on a line that
// also opened a bracket frame armed the body-skip for a terminator the join
// had already eaten, and the reader then skipped to EOF looking for a second
// one. Nothing was red: the drop set no `Lost`, and every floor in
// trapstatepick's census stayed comfortably clear over the four files it
// could not see. Both halves of the repair are in tsReadLines — arm the skip
// only when the opener is on the last line the join consumed, and commit a
// heredoc's body to `Here` only when its TERMINATOR is found, so an
// unterminated one sets `Lost` and leaves its lines claimed by nobody. The
// arm that holds it is TestQATheShellReaderReadsEveryLineUnderScripts, and
// the reason it is a separate assertion rather than a floor is that no floor
// could have caught this.
//
// WHERE IT IS DELIBERATELY LOOSE, named here so nobody has to rediscover it.
// The package's rule is to be loose in the direction of a false RED rather
// than a false pass (brewguard_qa_test.go), and this reader breaks that once,
// knowingly:
//
//   - A CONDITIONAL exit is read as an exit. `[ "$x" ] && exit 1` reaches
//     `exit` in command position and this rule calls it clean, though it can
//     still return. That is a false PASS, and it is here because the honest
//     alternative is a control-flow parse of a handler body for a shape no
//     member has: MEASURED 2026-11-14 by reading all 21 of them, every
//     registration on a deadly signal under scripts/ whose handler ends the
//     shell reaches its `exit` or its re-raise unconditionally — the twelve
//     `echo …; exit N` arms of the four splits, verify-gate-freshness.sh's
//     three calls to `nothing`, queue-cutover.sh's three bare exits,
//     reap-selftest-ambient.sh's `cleanup; exit 130` and test-times.sh's two
//     re-raises. (This sentence read "all nine" until ranger-base-5owgm.
//     Nine was already stale when ranger-base-kkp5g's five splits landed —
//     the same file's census said 18 and 17 in the same breath — and nobody
//     can now recover the predicate that counted nine, so it is replaced by
//     the census's own `ending`, which is the figure the floor below reads
//     and cannot drift away from this prose unnoticed.) The repair if one
//     ever appears is to require the exit
//     as the handler's LAST statement, which is a reading of the body this
//     reader already has the lines for.
//   - A call is followed ONE level and no further. A handler whose body
//     calls a function that itself ends the shell ends the shell, because
//     that is the tree's own vocabulary and not a loophole:
//     scripts/verify-gate-freshness.sh answers "I measured nothing" through
//     `nothing()`, which exits 2, and ADR 0092 reads that 2 as `unknown` —
//     a rule blind to it would push the next author off the script's own
//     channel and onto a bare `exit 143` the consumer has no reading for.
//     Two levels is where it stops, so a conditional call to an exiting
//     helper is the same single looseness as the conditional `exit` above
//     and not a second one. No member needs a deeper follow (MEASURED
//     2026-11-13).
//   - `exec`, which also cannot return, is not accepted as an ending. It
//     appears in no handler under scripts/ and reading it as one would be a
//     spelling nobody has used.
//
// WHAT IT DOES NOT REACH is ranger-base-wzo36's list, unchanged, because it
// is the same reader: `eval` and a handler named through a variable, a
// subshell made by anything but a bracket/pipe/`&`, the first of two
// definitions of one name, and the shell that owns a trap REGISTERED inside a
// function that itself runs in a subshell — the registration is read, never
// where the registration ran. And, as there, anything outside scripts/.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// tsgDeadly — the signals whose DEFAULT disposition ends the process and
// which bash will let a script trap. Those are the ones where a handler that
// returns changes the outcome: without the trap the script would be gone, and
// with it the script runs on.
//
// Both spellings `trap` accepts, since tsTrapSignals has already upper-cased
// and dropped a `SIG` prefix. The numbers are the four that could plausibly
// be typed (HUP, INT, QUIT, TERM); no numeric spelling appears under scripts/
// today (MEASURED 2026-11-13), and one added tomorrow is read as its name.
var tsgDeadly = map[string]bool{
	"HUP": true, "INT": true, "QUIT": true, "TERM": true,
	"USR1": true, "USR2": true, "ALRM": true, "PIPE": true,
	"1": true, "2": true, "3": true, "15": true,
}

// tsgExempt — a registration on a deadly signal whose handler RETURNS, with
// the reason that is right there. Keyed by (script, line-independent handler)
// the way tsExempt and mtExempt are: by what the row is ABOUT, never by a
// line number that is stale on the next edit above it.
//
// An exemption is a claim that RESUMING is the better answer for this script,
// and that is a claim about the script, so it carries prose and not a flag.
// Two-way: a row the census no longer finds reds as stale.
var tsgExempt = []struct{ Script, Handler, Reason string }{
	{
		Script:  "scripts/bd-argv-gate.sh",
		Handler: `rm -rf -- "${errd:?}"`,
		Reason: "This is a PreToolUse hook, so its EXIT STATUS IS ITS VERDICT and " +
			"every spelling that ends the shell on a signal is worse than resuming. " +
			"Claude's contract, MEASURED against the real binary under ranger-base-3bqn " +
			"(docs/notes.d/ranger-base-3bqn.md, property 2): exit 2 blocks the tool call, " +
			"ANY OTHER nonzero shows stderr and RUNS IT — a hook that exited 1 let the " +
			"refused command through. So `exit 130`/`exit 143` here is fail-OPEN on the one " +
			"script in this census where failing open is the whole stake, and an " +
			"unconditional `exit 2` instead would refuse every signalled Bash call on the " +
			"box whether or not it names bd, which is the fleet-wide false refusal that " +
			"same note forbids. Dropping INT/TERM/HUP from the trap is worse again: bash's " +
			"default disposition then kills the gate before it can answer, which is fail open — " +
			"the whole of the reason and enough on its own. It does NOT leak $errd, which this " +
			"row used to claim: MEASURED 2026-11-13 (linux 7.2.15, bash 5.3.20; " +
			"ranger-base-5owgm, docs/notes.d/ranger-base-kbaah.md) an UNTRAPPED terminating " +
			"signal still RUNS bash's EXIT trap, exits 128+signo and does not resume, so " +
			"`rm -rf -- \"${errd:?}\"` happens anyway. " +
			"Resuming is what lets the gate still reach its own fail-closed " +
			"decision, and MEASURED 2026-11-13 it does: bash RESTORES $? across a signal " +
			"handler, so `rc=$?` at :102 still reads the parser's status and not the " +
			"handler's. What resumption costs is one line of PROSE and no verdict — " +
			"`grep -q '^bd-argv-gate:' \"$err\"` at :107 reads a file inside the removed " +
			"$errd, so a parser that refused with a reason is reported by the generic " +
			"\"parser unavailable\" line instead of its own. Still exit 2, still fail closed.",
	},
}

// Every `trap` on a catchable deadly signal under scripts/ has a handler that
// ends the shell, or is on tsgExempt with its reason. Two-way, like
// tsExempt's and brewguard's and boxcheck's: a row the census no longer finds
// reds as stale, so neither the rule nor the roster can go quiet while
// staying green.
//
// The walk root is a LITERAL at the walk, because treewidedoor_qa_test.go
// derives the tree-wide class from an enumeration rooted at a relative path
// spelled in the test. A walk moved behind a parameter is a spelling that
// rule cannot see, and this pin would silently lose its door
// (mktemproot_qa_test.go's mtFileSet carries the measurement).
func TestQAEveryTrapOnADeadlySignalEndsTheShell(t *testing.T) {
	t.Parallel()
	walkRoots := []string{"scripts"}
	var walked []string
	for _, root := range []string{"scripts"} {
		if err := filepath.WalkDir(root, mtWalkInto(&walked)); err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	paths := mtFileSet(t, walkRoots, walked)

	type key struct{ script, handler string }
	exempt := map[key]string{}
	for _, e := range tsgExempt {
		exempt[key{e.Script, e.Handler}] = e.Reason
	}
	seen := map[key]bool{}
	shell, registrations, deadly, ending := 0, 0, 0, 0
	for _, path := range paths {
		s, err := tsRead(path)
		if err != nil {
			t.Errorf("read %s: %v", path, err)
			continue
		}
		if !s.Shell {
			continue
		}
		shell++
		if s.Lost != "" {
			// trapstatepick's arm reports this too and for the same reason;
			// the duplicate is deliberate, because each arm has to be able
			// to say on its own that it could not read a file rather than
			// report it clean.
			t.Errorf("%s: the reader lost the lexing at %s — it cannot say what this file traps, and a census that cannot read a file must not report it clean.", path, s.Lost)
		}
		registrations += len(s.Traps)
		for _, tr := range s.Traps {
			var hit []string
			for _, sig := range tr.Signals {
				if tsgDeadly[sig] {
					hit = append(hit, sig)
				}
			}
			if len(hit) == 0 {
				continue
			}
			deadly++
			k := key{path, tr.Handler}
			how, ends := tsgEndsShell(s, tr.Body)
			if reason, ok := exempt[k]; ok {
				seen[k] = true
				if ends {
					t.Errorf("tsgExempt names the %s handler in %s and the census finds it ends the shell (%s) — the row is an exemption over nothing, and it is also how this census would go blind without saying so", tsgShow(tr.Handler), path, how)
				}
				if len(strings.TrimSpace(reason)) < 120 {
					t.Errorf("%s: the %s handler is exempted from the signal rule with no reason worth reading (%q) — an exemption nobody can check is the rule being dropped quietly", tr.At(path), tsgShow(tr.Handler), reason)
				}
				continue
			}
			if ends {
				ending++
				continue
			}
			sort.Strings(hit)
			t.Errorf("%s: this trap is registered on %s and its handler RETURNS:\n"+
				"  %s\n"+
				"  %s\n"+
				"On EXIT a handler that returns is right. On a deadly signal it is not: bash runs the handler and then RESUMES this script at the statement after the one the signal interrupted, with whatever the handler just gave back already gone — so the run carries on over a torn-down apparatus and reports what it finds there. MEASURED 2026-11-13 on scripts/verify-herdr-bob-rules.sh: a `kill -TERM` ran cleanup, `rm -rf \"$T\"` took the scratch dir, and the script printed two FAILs for properties that were never false and never measured.\n"+
				"Split it — EXIT keeps the bare handler and the signals get a form that ends the shell:\n"+
				"  trap %s EXIT\n"+
				"  trap 'exit 130' INT\n"+
				"  trap 'exit 143' TERM\n"+
				"which is scripts/queue-cutover.sh:405-408 (the `exit` runs the EXIT trap, so the handler runs once and the status names the signal). Or, where the WAIT STATUS has to be a real signal death, reset and re-raise as scripts/test-times.sh:1674 does. Or put a row in tsgExempt (%s) saying why resuming is the better answer here — scripts/bd-argv-gate.sh's row is what one looks like.\n"+
				"This is ranger-base-kkp5g.",
				tr.At(path), strings.Join(hit, "/"),
				tr.Code,
				tsgWhy(tr),
				tsgHandlerWord(tr),
				"internal/treepins/trapsignalexit_qa_test.go")
		}
	}
	// Floors, so a reader that stopped reading cannot pass quietly. An
	// assertion that every handler ends the shell is satisfied by a census
	// that found no handlers, and a census that found none says nothing
	// about it. MEASURED 2026-11-14 (ranger-base-5owgm's four HUP arms): 48
	// shell files of the 64 under scripts/, 53 trap registrations between
	// them, 22 on a deadly signal and 21 of those ending the shell — the
	// twenty-second is bd-argv-gate.sh's exempted row, which is the whole
	// population of this rule's negative side. Each floor sits under its
	// reading, so an ordinary landing does not move it — these still sit
	// under the 2026-11-13 reading one bead below (49/18/17), on purpose.
	if shell < 40 || registrations < 42 || deadly < 15 || ending < 14 {
		t.Fatalf("the census found %d shell file(s), %d trap registration(s), %d on a deadly signal and %d of those ending the shell — the reader is broken, not the tree (MEASURED 2026-11-14: see the head comment)", shell, registrations, deadly, ending)
	}
	for _, e := range tsgExempt {
		if !seen[key{e.Script, e.Handler}] {
			t.Errorf("tsgExempt names the %s handler in %s and the census finds no such trap on a deadly signal there — a stale roster row is an exemption over nothing, and over a population this small it is also the only way this census can say it has gone blind", tsgShow(e.Handler), e.Script)
		}
	}
}

// THE READER READS EVERY LINE, which is the arm whose absence let the defect
// above hide (ranger-base-kkp5g). Both pins in this family decide from
// tsRead, both report a file they could not lex through `Lost`, and NEITHER
// noticed that four files under scripts/ were being read to a point and then
// abandoned — 908 physical lines, verify-box.sh from :271 to :898 — because
// the abandonment set no `Lost` and left every floor comfortably clear.
//
// The invariant is total and has no floor in it: a physical line carrying
// code is inside some logical line's extent, or it was consumed as a heredoc
// BODY. There is no third place for one to be, so a line in neither is a line
// the reader dropped, and a census that dropped a line must not report the
// file it was in as clean.
//
// It lives here rather than in trapstatepick_qa_test.go only because this is
// the bead that measured it; it is about tsRead and serves both pins. It
// enumerates scripts/ at a relative literal for the reason the arm above
// does, so it carries its own membership in the tree-wide class.
func TestQATheShellReaderReadsEveryLineUnderScripts(t *testing.T) {
	t.Parallel()
	walkRoots := []string{"scripts"}
	var walked []string
	for _, root := range []string{"scripts"} {
		if err := filepath.WalkDir(root, mtWalkInto(&walked)); err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	paths := mtFileSet(t, walkRoots, walked)

	shell, code := 0, 0
	for _, path := range paths {
		s, err := tsRead(path)
		if err != nil {
			t.Errorf("read %s: %v", path, err)
			continue
		}
		if !s.Shell {
			continue
		}
		shell++
		covered := map[int]bool{}
		for _, l := range s.Logical {
			for n, end := l.Line, l.At(len(l.Text)); n <= end; n++ {
				covered[n] = true
			}
		}
		var dropped []int
		for i, line := range s.Lines {
			n := i + 1
			if c := strings.TrimSpace(line); c == "" || strings.HasPrefix(c, "#") {
				continue
			}
			code++
			if covered[n] || s.Here[n] {
				continue
			}
			dropped = append(dropped, n)
		}
		if len(dropped) == 0 {
			continue
		}
		show := dropped
		if len(show) > 6 {
			show = show[:6]
		}
		var where []string
		for _, n := range show {
			where = append(where, fmt.Sprintf("    %d: %s", n, s.CodeAt(n)))
		}
		t.Errorf("%s: the reader read neither %d line(s) of this file nor any TERMINATED heredoc they could belong to — it stopped at line %d of %d (it says Lost=%q; an empty one there means it dropped them without noticing).\n%s\n"+
			"A line carrying code is inside a logical line's extent or it is a heredoc BODY; there is no third place, so a line in neither was DROPPED, and both pins in this family then report this file clean over code they never saw. That is how ranger-base-kkp5g's 908 lines across four files hid: the drop set no `Lost` and every floor stayed clear. Fix the reader, not this arm.",
			path, len(dropped), func() int {
				last := 0
				for n := range covered {
					if n > last {
						last = n
					}
				}
				return last
			}(), len(s.Lines), s.Lost, strings.Join(where, "\n"))
	}
	// One floor, and it is about THIS arm's own reach rather than the rule:
	// the invariant above is vacuous over a walk that found no shell, or a
	// reader that reported no lines. MEASURED 2026-11-13: 48 shell files
	// under scripts/ carrying 11,313 lines of code between them.
	if shell < 40 || code < 9000 {
		t.Fatalf("the coverage census read %d shell file(s) and %d line(s) of code — the reader is broken, not the tree (MEASURED 2026-11-13: 48 files, 11313 lines)", shell, code)
	}
}

// tsgEndsShell reports whether this handler body cannot RETURN to the
// statement the signal interrupted, and which spelling says so.
//
// It follows a call ONE level: a handler whose body calls a function that
// itself ends the shell ends the shell. That is not generosity, it is the
// tree's own vocabulary — scripts/verify-gate-freshness.sh answers "I
// measured nothing" through `nothing()`, which exits 2, and ADR 0092 reads
// that 2 as `unknown`. A rule that could not see it would push the next
// author off the script's own channel and onto a bare `exit 143` the
// consumer has no reading for. The follow stops at one level because a
// handler that reaches an `exit` only two calls deep is a handler whose
// signal behaviour no reader can see at a glance.
//
// Loose in one direction on purpose, and the head comment says which: a
// CONDITIONAL exit — or a conditional call to an exiting helper — is read as
// an ending, though either can still return.
func tsgEndsShell(s *tsScript, body string) (string, bool) {
	if how, ok := tsgSelfEnds(body); ok {
		return how, ok
	}
	if s == nil {
		return "", false
	}
	for _, name := range s.Order {
		f := s.Funcs[name]
		word := regexpQuoteWord(name)
		for _, loc := range word.FindAllStringIndex(body, -1) {
			if !tsCommandWord(body, loc[0]) {
				continue
			}
			if how, ok := tsgSelfEnds(f.Body); ok {
				return fmt.Sprintf("%s via %s(), which reaches %s", how, name, how), true
			}
		}
	}
	return "", false
}

// tsgSelfEnds is the direct test: does THIS text end the shell on its own.
func tsgSelfEnds(body string) (string, bool) {
	for off := 0; off+4 <= len(body); off++ {
		if !strings.HasPrefix(body[off:], "exit") {
			continue
		}
		if off+4 < len(body) && isTsNameByte(body[off+4]) {
			continue
		}
		if tsCommandWord(body, off) {
			return "an `exit` in command position", true
		}
	}
	// Reset-and-re-raise: `trap - TERM; kill -TERM $$`. All three parts, so
	// a handler that merely mentions `kill` is not read as one — the point of
	// the shape is that the signal's own default disposition is what ends the
	// shell, which needs the reset AND the re-raise at this pid.
	if strings.Contains(body, "trap -") && strings.Contains(body, "kill ") && strings.Contains(body, "$$") {
		return "a reset-and-re-raise (`trap - SIG; kill -SIG $$`)", true
	}
	return "", false
}

// regexpQuoteWord is the whole-word matcher tsRead's own call-site pass uses,
// spelled once here so the two readings of "this name is called" cannot come
// to disagree about what a word boundary is.
func regexpQuoteWord(name string) *regexp.Regexp {
	return regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
}

// tsgWhy renders the second line of a diagnostic: WHERE the code that will
// not end the shell is, which is different prose for a named handler than for
// an inline one.
func tsgWhy(tr tsTrap) string {
	if tr.IsFunc {
		return fmt.Sprintf("%s() reaches no `exit` and no re-raise, so it returns and the script goes on", tr.Handler)
	}
	return "the handler is this text, and it reaches no `exit` and no re-raise, so it returns and the script goes on"
}

// tsgHandlerWord renders the handler the way the suggested `trap … EXIT`
// would be typed: a function by name, an inline body back inside its quotes.
func tsgHandlerWord(tr tsTrap) string {
	if tr.IsFunc {
		return tr.Handler
	}
	return "'" + tr.Handler + "'"
}

// tsgShow renders a handler for a roster diagnostic, short enough to read.
func tsgShow(handler string) string {
	if len(handler) > 48 {
		handler = handler[:45] + "…"
	}
	return "`" + handler + "`"
}

// The reader and the rule over planted scripts, because the one thing this
// pin was asked for is that it reds on `trap cleanup EXIT INT TERM` — and a
// rule that has only ever run over a tree that passes has not been shown to
// say no. The fixtures are absolute paths under t.TempDir(), so this arm
// enumerates nothing and is not in the tree-wide class (trapstatepick's
// control is not, for the same reason).
func TestQATheSignalTrapReaderCanStillSayNo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name, body string) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	// badOf returns the registrations on a deadly signal whose handler
	// returns — the thing the tree-wide arm turns into an error.
	badOf := func(t *testing.T, path string) []tsTrap {
		t.Helper()
		s, err := tsRead(path)
		if err != nil {
			t.Fatal(err)
		}
		if s.Lost != "" {
			t.Fatalf("%s: the reader lost the lexing at %s", path, s.Lost)
		}
		var bad []tsTrap
		for _, tr := range s.Traps {
			deadly := false
			for _, sig := range tr.Signals {
				if tsgDeadly[sig] {
					deadly = true
				}
			}
			if _, ends := tsgEndsShell(s, tr.Body); deadly && !ends {
				bad = append(bad, tr)
			}
		}
		return bad
	}

	for _, c := range []struct {
		name string
		body string
		want int // registrations the rule refuses
		why  string
	}{
		{
			name: "returns.sh",
			body: "#!/usr/bin/env bash\nT=$(mktemp -d)\ncleanup() {\n  rm -rf \"$T\"\n}\ntrap cleanup EXIT INT TERM\nsleep 5\necho FAIL\n",
			want: 1,
			why:  "the shape the bead was filed on, and the one this pin exists to red",
		},
		{
			name: "split.sh",
			body: "#!/usr/bin/env bash\nT=$(mktemp -d)\ncleanup() {\n  rm -rf \"$T\"\n}\ntrap cleanup EXIT\ntrap 'exit 130' INT\ntrap 'exit 143' TERM\nsleep 5\n",
			want: 0,
			why:  "queue-cutover.sh's spelling: the EXIT handler may return, the signal handlers exit",
		},
		{
			name: "inline-exit.sh",
			body: "#!/usr/bin/env bash\nT=$(mktemp -d)\ncleanup() {\n  rm -rf \"$T\"\n}\ntrap 'cleanup; exit 130' INT TERM\ntrap cleanup EXIT\n",
			want: 0,
			why:  "reap-selftest-ambient.sh:129's spelling, the handler reached through an inline body",
		},
		{
			name: "reraise.sh",
			body: "#!/usr/bin/env bash\nnote() { echo signalled >&2; }\ntrap 'note; trap - TERM; kill -TERM $$' TERM\nsleep 5\n",
			want: 0,
			why:  "test-times.sh:1674's spelling, where the wait status has to be a real signal death",
		},
		{
			name: "exit-in-the-function.sh",
			body: "#!/usr/bin/env bash\nT=$(mktemp -d)\nbail() {\n  rm -rf \"$T\"\n  exit 143\n}\ntrap bail INT TERM\ntrap 'rm -rf \"$T\"' EXIT\n",
			want: 0,
			why:  "the handler is a FUNCTION and the exit is inside its body, which is the resolution tsRead does",
		},
		{
			name: "exit-in-a-helper.sh",
			body: "#!/usr/bin/env bash\nnothing() { echo \"nothing measured\"; exit 2; }\ntrap 'nothing \"interrupted by SIGINT\"' INT\n",
			want: 0,
			why:  "verify-gate-freshness.sh's shape: the handler answers through the script's own `nothing()`, which exits 2 — the one-level follow is what lets the rule accept the tree's own vocabulary",
		},
		{
			name: "helper-that-does-not-exit.sh",
			body: "#!/usr/bin/env bash\nnote() { echo signalled >&2; }\nbail() {\n  note \"x\"\n}\ntrap bail INT TERM\n",
			want: 1,
			why:  "the follow is not a free pass: a handler calling a helper that merely prints still RETURNS, and is still a finding",
		},
		{
			name: "exit-two-calls-deep.sh",
			body: "#!/usr/bin/env bash\nharness() { echo apparatus >&2; exit 1; }\nnote() { harness \"x\"; }\nbail() {\n  note \"y\"\n}\ntrap bail INT TERM\n",
			want: 1,
			why:  "the follow stops at ONE level — the head comment's second loose edge, held here so it cannot widen or close in silence",
		},
		{
			name: "exit-is-a-word-not-a-command.sh",
			body: "#!/usr/bin/env bash\nnote() {\n  echo \"exit status recorded\" >&2\n  touch /tmp/exit.log\n}\ntrap note INT TERM\n",
			want: 1,
			why:  "`exit` inside a string and as part of a path is not an exit; the command-position test is what makes this a red",
		},
		{
			name: "ignored.sh",
			body: "#!/usr/bin/env bash\ntrap '' INT TERM\nsleep 5\n",
			want: 0,
			why:  "`trap '' SIG` runs no code, so there is nothing that could tear an apparatus down and resume",
		},
		{
			name: "reset.sh",
			body: "#!/usr/bin/env bash\ntrap - INT TERM\nsleep 5\n",
			want: 0,
			why:  "`trap - SIG` restores the default disposition, which ends the shell by itself",
		},
		{
			name: "exit-trap-only.sh",
			body: "#!/usr/bin/env bash\nT=$(mktemp -d)\ncleanup() {\n  rm -rf \"$T\"\n}\ntrap cleanup EXIT\n",
			want: 0,
			why:  "EXIT alone is the shape this rule has no opinion about — a handler that returns there is the contract",
		},
		{
			name: "err-trap.sh",
			body: "#!/usr/bin/env bash\ntrap '[ \"$?\" -eq 2 ] && echo \"apparatus failure\" >&2' ERR\nfalse || true\n",
			want: 0,
			why:  "verify-id-recycle.sh:28's shape: ERR is a pseudo-signal where running the handler and carrying on IS the contract",
		},
		{
			name: "sigterm-spelling.sh",
			body: "#!/usr/bin/env bash\nT=$(mktemp -d)\ncleanup() {\n  rm -rf \"$T\"\n}\ntrap cleanup EXIT SIGINT SIGTERM\n",
			want: 1,
			why:  "`SIGTERM` and `sigterm` are the same signal as `TERM`, so the rule cannot be stepped around by spelling it longer",
		},
		{
			name: "numeric-spelling.sh",
			body: "#!/usr/bin/env bash\nT=$(mktemp -d)\ncleanup() {\n  rm -rf \"$T\"\n}\ntrap cleanup 0 2 15\n",
			want: 1,
			why:  "nor by spelling it as a number — 2 and 15 are INT and TERM, and 0 is EXIT",
		},
		{
			name: "two-registrations.sh",
			body: "#!/usr/bin/env bash\nT=$(mktemp -d)\ncleanup() {\n  rm -rf \"$T\"\n}\ntrap cleanup EXIT INT TERM\ntrap cleanup HUP\n",
			want: 2,
			why:  "each REGISTRATION is judged, not each handler: the same returning cleanup on two trap lines is two findings",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			bad := badOf(t, write(c.name, c.body))
			if len(bad) != c.want {
				var got []string
				for _, tr := range bad {
					got = append(got, fmt.Sprintf("%d:%v:%s", tr.Line, tr.Signals, tr.Handler))
				}
				t.Fatalf("the rule refuses %d registration(s) in %s, want %d — %s\n  refused: %v\n  body:\n%s",
					len(bad), c.name, c.want, c.why, got, c.body)
			}
		})
	}
}

// And the roster's own two-way half over a planted tree, because the arm
// above cannot reach it: the tree-wide arm's stale-row check only fires when
// a real script stops matching, and that is the one state nobody can plant
// in scripts/ without breaking the tree.
func TestQATheSignalTrapRosterIsTwoWay(t *testing.T) {
	t.Parallel()
	// Every live row names a script that exists and a handler the census
	// finds there on a deadly signal, returning. That is what the tree-wide
	// arm asserts; this asserts the ROW is well formed on its own terms, so
	// a typo in a path or a handler reads as a stale row and not as a rule
	// that stopped applying.
	for _, e := range tsgExempt {
		if _, err := os.Stat(e.Script); err != nil {
			t.Errorf("tsgExempt names %s, which this tree does not hold: %v", e.Script, err)
			continue
		}
		s, err := tsRead(e.Script)
		if err != nil {
			t.Errorf("read %s: %v", e.Script, err)
			continue
		}
		found := false
		var had []string
		for _, tr := range s.Traps {
			had = append(had, tr.Handler)
			if tr.Handler != e.Handler {
				continue
			}
			found = true
			if _, ends := tsgEndsShell(s, tr.Body); ends {
				t.Errorf("tsgExempt's row for %s names a handler that ENDS the shell — the row is over nothing", e.Script)
			}
		}
		if !found {
			t.Errorf("tsgExempt's row for %s names the handler %q, and %s registers none spelled that way — the handler these rows are keyed by is the registration's own text, and this one has drifted from it.\n  it traps: %q", e.Script, e.Handler, e.Script, had)
		}
	}
}
