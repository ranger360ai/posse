package posse

// QA pins for ranger-base-q8hbz, found as the pre-existing red under
// ranger-base-s8b4g's sweep of the verify-*.sh scripts.
//
// THE DEFECT. `scripts/verify-govern-honesty.sh` ended every run from a
// dispatched seat with
//
//	verify-govern-honesty: the lock was still held after kill -9
//
// after five PASS arms, exit 2 — which reads as the flock the whole design
// rests on not being released by process death, and was nothing of the kind.
// Its reaper was
//
//	pkill -9 -P "$LOOP_PID" >/dev/null 2>&1 || true   # the loop's CHILD
//	kill -9 "$LOOP_PID" >/dev/null 2>&1 || true       # the subshell only
//
// and every crew PID denies `Bash(pkill:*)` (operator ruling 2026-09-03,
// ranger-base-jjx19). The deny is carried as an L1 PATH shim at the head of
// every seat's PATH, so it refuses the SCRIPT's own pkill, not just a
// persona's typed one — into a `2>/dev/null`, where nothing sees it. The
// `kill -9` then took the subshell wrapper and left `env … posse dispatch
// --watch` alive, still holding the flock. Five arms passed, thirteen were
// never reached, and the one thing this script exists to measure was reported
// backwards.
//
// MEASURED 2026-09-06 at ec0beaa, before the fix: one run, five PASS, exit 2,
// `2026-09-06T13:22:39Z pkill -9 -P 88661 (deny: Bash(pkill:*))` in the seat's
// refusals.log, and two orphans on ppid 1 — the watch loop AND a `posse
// cockpit`, whose reaper had the same subshell-wrapper shape. After the fix:
// eighteen PASS, exit 0, no orphans, no refusal.
//
// `scripts/macos-install-probe.sh` had the same class twice, and those were
// PATTERN kills, which is the thing the ruling is actually about:
// `pkill -9 -f "$ROOT/"` and `pkill -9 -f "$p"` match on argv across every
// process on the box. MEASURED the same day at HEAD: one `quarantine` run,
// every arm ok, two `…/q/posse-B version` processes left behind on ppid 1 and
// both refusals in the log. After the fix: same arms, no orphans, no refusal.
//
// WHAT IS PINNED HERE, and why it is three subjects. Each is a reader plus
// the control arm that grades it, because a reader that answers "nothing
// wrong" to everything is how this defect got four months of green arms in
// the first place — so five tests for three claims.
//
// The first is the SWEEP: no shipped shell script may name `pkill` or
// `killall` outside a quote or a comment. A sweep that finds nothing proves
// nothing on its own, so its detector is graded first against a fixture table
// that carries the exact pre-fix lines it has to flag and the real corpus
// lines it must not.
//
// The second is the MECHANISM, executed rather than read, because the sweep
// only says the verb is gone — not that what replaced it reaps anything. It
// runs both reaper shapes against a live fixture with a REFUSING pkill at the
// head of PATH, the shim's own shape, and reads the kernel: the old shape has
// to leave its grandchild alive (without that arm the new one is measuring
// the absence of a defect that could never have happened here), and the new
// one has to leave nothing.
//
// The third is the SHAPE OF THE SHIPPED SCRIPTS, added by ranger-base-8v29w
// and written up where it stands, below the second. The second runs a REPLICA
// of the reaper, so between them the first two left the two scripts' own fix
// pinned by the absence of one word: `set -m` and `exec` could both come back
// out of verify-govern-honesty.sh with all three of the tests that existed
// staying green and the watch loop outliving its reaper again. MEASURED, and
// that mutant now reds.
//
// Both fixtures end their own processes by the pid they recorded when they
// started them, and a reader tempted to replace that with a tidier census
// should know it was tried: while writing this pin, a `ps | grep "sleep 30" |
// kill` sweep run to look for leaked fixtures ended a `sleep 30` belonging to
// a DIFFERENT seat's Bash call — a live pid 77992 under a gate shell, nothing
// to do with this test. A census keyed on argv is a pattern kill in other
// clothes, which is the whole of ranger-base-jjx19, demonstrated on the person
// fixing it.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// pkUnquotedKills answers every `pkill`/`killall` in one line of shell that is
// neither inside a quoted string nor after a comment `#`. Both exclusions are
// load-bearing against the real tree: `scripts/test-times.sh` carries a whole
// forensic fixture of typed pkill lines inside a `table="…"`, greps whose
// alternation lists the word, and prose about why a pattern kill is wrong,
// and `scripts/verify-pid-deny-set.sh` carries the deny literals themselves.
// None of those is a kill, and a detector that flagged them would be turned
// off within the week.
func pkUnquotedKills(line string) []string {
	var found []string
	var single, double bool
	r := []rune(line)
	for i := 0; i < len(r); i++ {
		c := r[i]
		switch {
		case single:
			if c == '\'' {
				single = false
			}
			continue
		case double:
			if c == '\\' {
				i++ // an escape inside "…" hides the next byte, quote included
			} else if c == '"' {
				double = false
			}
			continue
		case c == '\'':
			single = true
			continue
		case c == '"':
			double = true
			continue
		case c == '\\':
			i++
			continue
		case c == '#' && (i == 0 || r[i-1] == ' ' || r[i-1] == '\t'):
			return found // the rest of the line is a comment
		}
		for _, verb := range []string{"pkill", "killall"} {
			n := len([]rune(verb))
			if i+n > len(r) || string(r[i:i+n]) != verb {
				continue
			}
			// Whole word: `pkill-ish` and `dpkill` are not the verb, and
			// neither is the `killall` inside `bd daemons killall`, which is
			// a bd subcommand and reaches nothing on this box.
			if i > 0 && (isPkWordByte(r[i-1]) || r[i-1] == '-') {
				continue
			}
			// A trailing `:` is a rule name, not a command word: the deny
			// literal `Bash(pkill:*)` is the string this whole pin is about
			// and appears unquoted in the shipped PIDs. Caught by the
			// control arm below, which is what it is for.
			if i+n < len(r) && (isPkWordByte(r[i+n]) || r[i+n] == '-' || r[i+n] == ':') {
				continue
			}
			found = append(found, verb)
			i += n - 1
			break
		}
	}
	return found
}

func isPkWordByte(c rune) bool {
	return c == '_' || c == '.' || c == '/' ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// TestThePatternKillDetectorFlagsTheLinesItIsFor is the control arm. Without
// it the sweep below is a green light over a detector that answers no to
// everything — which is how the defect it guards got four months of green
// arms in the first place.
func TestThePatternKillDetectorFlagsTheLinesItIsFor(t *testing.T) {
	// The four must-flag lines are verbatim from the pre-fix scripts, at
	// ec0beaa: two from verify-govern-honesty.sh's reaper and two from
	// macos-install-probe.sh.
	mustFlag := []string{
		"\tpkill -9 -P \"$LOOP_PID\" >/dev/null 2>&1 || true",
		"\tpkill -9 -f \"$ROOT/\" 2>/dev/null",
		"\t\tpkill -9 -f \"$p\" 2>/dev/null",
		"killall yes",
		"cd /x && pkill -9 -f foo",
		"kill -9 $pid; pkill -9 -P $pid",
	}
	for _, line := range mustFlag {
		if got := pkUnquotedKills(line); len(got) == 0 {
			t.Errorf("the detector did not flag a real pattern kill, so a zero sweep measures nothing:\n\t%s", line)
		}
	}

	// The must-pass lines are the real corpus: every one of them is in the
	// tracked tree today and none of them kills anything.
	mustPass := []string{
		"# pkill/killall: a pattern kill matches every seat's byte-identical argv",
		"\t- Bash(pkill:*)",
		"  'Bash(pkill:*)' 'Bash(killall:*)'",
		"  grep -E 'pkill|kill -|kill [0-9]' \\",
		"    | grep -vF 'pkill|kill -|kill [0-9]' \\",
		"  table=\"  501 /bin/zsh -c eval 'pgrep -f t.sh; pkill -f t.sh; pkill -f \\\"go test\\\"'\"",
		"    *'never with a `pkill -f` pattern'*) ok 'run line: says why a pattern kill is wrong here' ;;",
		"printf 'test-times: stop it with `kill %s`, never with a `pkill -f` pattern\\n' \"$$\"",
		"\tkill -9 -- \"-$pid\" >/dev/null 2>&1 || kill -9 \"$pid\" >/dev/null 2>&1 || true",
		"echo hi # pkill is denied here",
		"\t\"cd /tmp && bd --no-daemon daemons killall\": \"daemons\",",
	}
	for _, line := range mustPass {
		if got := pkUnquotedKills(line); len(got) != 0 {
			t.Errorf("the detector flagged %v on a line that kills nothing — a detector this noisy gets waived:\n\t%s", got, line)
		}
	}
}

// TestNoShippedShellScriptCallsAPatternKill sweeps what posse ships. The
// ruling is crew-wide and its realization is a PATH shim, so a `pkill` left in
// a script is not a style question: it is a call that will be refused, into a
// `2>/dev/null`, with the arm above it reporting whatever the un-reaped
// process makes true.
func TestNoShippedShellScriptCallsAPatternKill(t *testing.T) {
	out, err := exec.Command("git", "ls-files", "-z").Output()
	if err != nil {
		t.Fatalf("git ls-files: %v — this pin needs the checkout to name what posse ships", err)
	}
	tracked := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	if len(tracked) < 100 {
		t.Fatalf("git tracks %d files — the listing failed, so a clean result here is not evidence", len(tracked))
	}

	var scanned int
	var hits []string
	for _, rel := range tracked {
		if rel == "" {
			continue
		}
		if !strings.HasSuffix(rel, ".sh") && filepath.Base(rel) != "Makefile" {
			continue
		}
		b, err := os.ReadFile(filepath.FromSlash(rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		scanned++
		for i, line := range strings.Split(string(b), "\n") {
			if verbs := pkUnquotedKills(line); len(verbs) > 0 {
				hits = append(hits, fmt.Sprintf("%s:%d: %s → %s", rel, i+1, strings.TrimSpace(line), verbs))
			}
		}
	}
	// The floor is the positive witness: a glob that matched nothing would
	// leave hits empty and this test green over a sweep of no files at all.
	// 38 tracked .sh files plus the Makefile at the time of writing. A floor
	// this close to the census also catches the sweep silently NARROWING —
	// a suffix test or a path filter that quietly stops seeing most of the
	// tree, which reads exactly like a clean result.
	if scanned < 30 {
		t.Fatalf("only %d shell files swept in the whole tracked tree — the sweep is not "+
			"finding them, so a clean result measures nothing", scanned)
	}
	if len(hits) > 0 {
		t.Fatalf("a shipped script calls a pattern kill, which every crew seat refuses "+
			"(Bash(pkill:*)/Bash(killall:*), ranger-base-jjx19) — kill the pid you launched, or\n"+
			"`kill -- -$$` for your own process group; `kill`, `kill -0` and `pgrep` still run:\n\t%s",
			strings.Join(hits, "\n\t"))
	}
}

// pkShimDir writes the L1 gate's own shape — refuse on stderr, exit 1 — and
// answers a dir to put at the head of PATH.
func pkShimDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	shim := "#!/bin/sh\n" +
		"echo \"refused by posse gate: $(basename \"$0\") $* (deny: Bash($(basename \"$0\"):*))\" >&2\n" +
		"exit 1\n"
	for _, verb := range []string{"pkill", "killall"} {
		if err := os.WriteFile(filepath.Join(dir, verb), []byte(shim), 0o755); err != nil {
			t.Fatalf("write %s shim: %v", verb, err)
		}
	}
	return dir
}

// pkRunReaper runs one reaper shape against a live fixture and answers the
// grandchild's pid and whether it was still alive after the reap. The fixture
// is a `sleep` under an `env` under a subshell — the shape start_loop had.
func pkRunReaper(t *testing.T, shimDir, body string) (pid int, alive bool) {
	t.Helper()
	script := "#!/usr/bin/env bash\nset -uo pipefail\n" + body
	f := filepath.Join(t.TempDir(), "reaper.sh")
	if err := os.WriteFile(f, []byte(script), 0o755); err != nil {
		t.Fatalf("write reaper fixture: %v", err)
	}
	cmd := exec.Command("bash", f)
	cmd.Env = append(os.Environ(), "PATH="+shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("run reaper fixture: %v\n%s", err, out)
	}
	fields := strings.Fields(string(out))
	if len(fields) != 2 {
		t.Fatalf("reaper fixture printed %q, wanted `<pid> <alive|gone>`", out)
	}
	pid, err = strconv.Atoi(fields[0])
	if err != nil {
		t.Fatalf("reaper fixture printed a non-pid %q", fields[0])
	}
	return pid, fields[1] == "alive"
}

// TestTheGroupReaperEndsWhatThePatternKillCouldNot executes both shapes with a
// refusing pkill on PATH. The old arm is not decoration: it is what makes the
// new arm's "gone" a measurement rather than a fixture that was never going to
// survive anything.
func TestTheGroupReaperEndsWhatThePatternKillCouldNot(t *testing.T) {
	shimDir := pkShimDir(t)

	// The pre-fix shape, verbatim in its two reaping lines.
	oldPID, oldAlive := pkRunReaper(t, shimDir, `
(cd /tmp && env PKPROBE=1 sleep 30 >/dev/null 2>&1) &
LOOP_PID=$!
sleep 0.4
KID=$(pgrep -P "$LOOP_PID" | head -1)
pkill -9 -P "$LOOP_PID" >/dev/null 2>&1 || true
kill -9 "$LOOP_PID" >/dev/null 2>&1 || true
wait "$LOOP_PID" 2>/dev/null || true
sleep 0.4
if kill -0 "$KID" 2>/dev/null; then echo "$KID alive"; else echo "$KID gone"; fi
# Read first, then end it — by the pid this fixture started. The 30-second
# sleep is the backstop if this line is never reached.
kill -9 "$KID" 2>/dev/null || true
`)
	if !oldAlive {
		t.Fatalf("the pre-fix reaper ended its grandchild (pid %d) even with pkill refused — "+
			"then this fixture is not the shape the defect lived in, and the arm below "+
			"measures nothing", oldPID)
	}

	// The fix: `set -m` + `exec` make the job a process-GROUP leader that IS
	// the binary, and the group is reapable by the pid we were handed.
	newPID, newAlive := pkRunReaper(t, shimDir, `
set -m
(cd /tmp && exec env PKPROBE=1 sleep 30 >/dev/null 2>&1) &
LOOP_PID=$!
set +m
disown "$LOOP_PID" 2>/dev/null || true
sleep 0.4
kill -9 -- "-$LOOP_PID" >/dev/null 2>&1 || kill -9 "$LOOP_PID" >/dev/null 2>&1 || true
n=0
while [ "$n" -lt 40 ] && { kill -0 "$LOOP_PID" 2>/dev/null || [ -n "$(pgrep -g "$LOOP_PID" 2>/dev/null)" ]; }; do
	n=$((n + 1)); sleep 0.1
done
if kill -0 "$LOOP_PID" 2>/dev/null || [ -n "$(pgrep -g "$LOOP_PID" 2>/dev/null)" ]; then
	echo "$LOOP_PID alive"
else
	echo "$LOOP_PID gone"
fi
`)
	if newAlive {
		t.Errorf("the group reaper left pid %d alive with pkill refused — the fix for "+
			"ranger-base-q8hbz does not reap, and the arms after it are unmeasured", newPID)
	}
}

// ─── the shipped scripts' own shape, read rather than replicated ─────────────
//
// TestTheGroupReaperEndsWhatThePatternKillCouldNot above runs a REPLICA of the
// reaper shape, not either script, so what the two shipped scripts actually do
// was pinned only by the absence of the word `pkill` — and the sweep is blind
// to every other half of the fix. MEASURED (ranger-base-8v29w, the verify of
// ranger-base-q8hbz): drop the `set -m` and the `exec` out of
// verify-govern-honesty.sh's start_loop, reintroducing NO pkill so the sweep
// still passes, and all three tests above stay green while the watch loop
// outlives its reaper exactly as it did pre-fix.
//
// So this arm reads the scripts. Three rules, each of them a half of the fix
// q8hbz landed, and each stated as what the shipped file must show:
//
//	A  a background launch whose pid is CAPTURED (`x=$!`) runs under `set -m`,
//	   so the pid it hands out is also a process-GROUP id, and `kill -SIG --
//	   "-$pid"` reaches the work rather than the wrapper around it.
//	D  a launch that is a SINGLE-COMMAND subshell `( … ) &` execs that command,
//	   so the group leader pid IS the binary under test — which makes both the
//	   `|| kill -SIG "$pid"` fallback and every `kill -0 "$pid"` speak about
//	   that binary rather than about a wrapper that may already be gone.
//	   run_bounded in macos-install-probe.sh is exempt from D and honestly so:
//	   its subshell carries a second command (`echo $? >"$p.rc"`) that must
//	   outlive the first, so it cannot exec.
//
//	   WHY BOTH, measured on this box rather than argued: the two halves reap
//	   by different routes and either one alone is enough in this shape. With
//	   `set -m` and no `exec`, the group kill reaps the grandchild (the
//	   wrapper is the group leader and the work is in its group). With `exec`
//	   and no `set -m`, the group kill finds no group and the pid-only
//	   fallback takes the work, because the pid IS the work. With NEITHER the
//	   grandchild survives — which is the pre-fix shape, and why the mutant
//	   ranger-base-8v29w measured had to drop both to restore q8hbz. The rules
//	   are separate because the two scripts' reasons for each are separate,
//	   and because a fix that keeps only one half is one edit from keeping
//	   neither.
//	B  a kill carrying an explicit terminating signal tries the GROUP first —
//	   `kill -SIG -- "-$pid"` — whatever it falls to afterwards on the same
//	   line. `kill -0` is a liveness probe and is not a kill; a signal-less
//	   `kill "$pid"` is exempt, because the dozen of those on
//	   macos-install-probe.sh's early-return paths are courtesy TERMs whose
//	   backstop is cleanup's `reap`, which IS a group kill and is checked by
//	   this rule.
//
// A launch that captures no `$!` is exempt from A and D. That exemption is
// not described here and counted nowhere — it is asserted by NAME in the arm
// below, against the launches the reader actually exempted, because it is the
// one way a launch leaves the ruleset while the 5/4 floor stays satisfied
// (ranger-base-wenqb). One launch is named today: verify-govern-honesty.sh's
// scratch herdr server, whose pid is never taken because it is ended by
// `herdr session stop` and by nothing else.
//
// "Captures" is read FORWARD to the next launch, not at the single next
// statement. `$!` survives intervening foreground commands in bash, so a
// capture one ordinary statement further down is still a capture — and
// reading only the next statement is what let one inserted `printf` exempt
// start_loop from both launch rules with the pre-fix shape restored
// underneath and nothing red (ranger-base-wenqb, measured).
//
// This is a static read, and it says so: it does not run either script, and
// it is not a substitute for doing so. A seat cannot in any case — REPORTED
// on ranger-base-8v29w and not re-measured here: a QA seat's scratch herdr
// will not start under the seatbelt cage ("Error: Os { code: 1, kind:
// PermissionDenied }"), so verify-govern-honesty.sh exits 2 at "scratch herdr
// did not come up" before its first arm. What the static read buys is the
// half a live run would not give either: the shape is asserted on every suite
// run, on any box, in under a fifth of a second, including the boxes and the
// runs where neither script is executed at all.

// pkShellCode answers one line of shell with its comment tail removed, and a
// mask saying which of the returned runes were inside a quoted string. The
// quoting rules are pkUnquotedKills's, for the reasons written there: this
// tree's shell carries pkill lines, kill lines and `&` inside strings and
// comments that are not the thing being looked for.
func pkShellCode(line string) (string, []bool) {
	var out []rune
	var mask []bool
	var single, double bool
	r := []rune(line)
	for i := 0; i < len(r); i++ {
		c := r[i]
		switch {
		case single:
			out, mask = append(out, c), append(mask, true)
			if c == '\'' {
				single = false
			}
			continue
		case double:
			out, mask = append(out, c), append(mask, true)
			if c == '\\' && i+1 < len(r) {
				i++
				out, mask = append(out, r[i]), append(mask, true)
			} else if c == '"' {
				double = false
			}
			continue
		case c == '\'':
			single = true
		case c == '"':
			double = true
		case c == '\\' && i+1 < len(r):
			out, mask = append(out, c), append(mask, false)
			i++
			c = r[i]
		case c == '#' && (i == 0 || r[i-1] == ' ' || r[i-1] == '\t'):
			return string(out), mask // the rest of the line is a comment
		}
		out, mask = append(out, c), append(mask, false)
	}
	return string(out), mask
}

// pkHasUnquoted answers whether b appears outside a quoted string in a line
// already run through pkShellCode.
func pkHasUnquoted(code string, mask []bool, b rune) bool {
	for i, c := range []rune(code) {
		if c == b && i < len(mask) && !mask[i] {
			return true
		}
	}
	return false
}

var (
	// pkGroupKill matches the group-first reaper anywhere in a line: an
	// explicit signal, then the `--` that keeps the negative pid off the
	// option table (ADR-less but load-bearing: without it `-9` and `-$pid`
	// are both read as options), then a `-$…` target.
	pkGroupKill = regexp.MustCompile(`kill\s+-[A-Za-z0-9]+\s+--\s+"?-\$`)
	// pkGroupKillHere is the same shape ANCHORED, and the rule uses this one:
	// the group form has to be the FIRST killing verb on the line, not merely
	// present somewhere on it. `kill -9 "$a" || kill -9 -- "-$b"` reaps $a by
	// pid and carries a group kill for a different job, and an unanchored
	// read calls that clean.
	pkGroupKillHere = regexp.MustCompile(`^kill\s+-[A-Za-z0-9]+\s+--\s+"?-\$`)
	// pkSignalKill matches any kill carrying an explicit signal, group form
	// or not. `-0` is excluded by the rule below, not here, so the count of
	// what was examined stays visible to the caller.
	pkSignalKill = regexp.MustCompile(`(^|[^-\w])kill\s+-([A-Za-z0-9]+)\b`)
	// pkExec answers the `exec` that makes a subshell's group leader the
	// binary itself rather than a wrapper around it.
	pkExec = regexp.MustCompile(`(^|[^-\w])exec\s`)
)

// pkAudit reads one script and answers its complaints, how many background
// launches it saw, how many explicit-signal kills it examined, and which
// launches it EXEMPTED from the launch rules for handing out no pid. The two
// counts are the positive witness: a parse that recognized nothing produces no
// complaints either, which reads exactly like a clean script. The exempt list
// is the second half of that witness, and it is returned rather than described
// because the exemption is the one way a launch leaves rules A and D while
// `launches++` still fires — so the caller pins WHICH launches took it
// (ranger-base-wenqb), and a new one shows up as a named failure.
func pkAudit(name, text string) (complaints []string, launches, kills int, exempt []string) {
	lines := strings.Split(text, "\n")
	code := make([]string, len(lines))
	mask := make([][]bool, len(lines))
	for i, l := range lines {
		code[i], mask[i] = pkShellCode(l)
	}
	stmt := func(i int) bool { return strings.TrimSpace(code[i]) != "" }
	// prev/next answer the nearest statement line either way, stepping over
	// blank lines and the comment blocks these scripts are mostly made of.
	prev := func(i int) int {
		for j := i - 1; j >= 0; j-- {
			if stmt(j) {
				return j
			}
		}
		return -1
	}
	next := func(i int) int {
		for j := i + 1; j < len(lines); j++ {
			if stmt(j) {
				return j
			}
		}
		return -1
	}
	// isLaunch answers whether line j is a background launch: its last
	// unquoted rune is a lone `&`. `&&` at the end of a line is a
	// continuation and `>&2` is a redirection.
	isLaunch := func(j int) bool {
		c := strings.TrimSpace(code[j])
		if !strings.HasSuffix(c, "&") || strings.HasSuffix(c, "&&") || strings.HasSuffix(c, ">&") {
			return false
		}
		idx := strings.LastIndex(code[j], "&")
		return idx >= 0 && !mask[j][len([]rune(code[j][:idx]))]
	}
	// captures answers whether the launch at line i has its pid taken, and it
	// reads FORWARD to the next launch rather than at the single next
	// statement. `$!` in bash names the most recent background job and
	// survives any number of intervening foreground commands, so every
	// statement up to the next launch — the line where `$!` starts naming
	// something else — can still be the capture. MEASURED (ranger-base-wenqb):
	// reading only the immediately-next statement let one `printf` between
	// the launch and its `LOOP_PID=$!` exempt start_loop from BOTH launch
	// rules, with `launches++` still firing, so the pre-fix shape restored
	// under that one line of re-layout was green on every reader of either
	// script. Erring the other way is safe and loud: a `$!` that belongs to
	// some later block applies MORE rules to this launch, and says so.
	captures := func(i int) bool {
		for j := next(i); j >= 0; j = next(j) {
			if strings.Contains(code[j], "$!") {
				return true
			}
			if isLaunch(j) {
				return false
			}
		}
		return false
	}

	for i := range lines {
		c := strings.TrimSpace(code[i])
		at := fmt.Sprintf("%s:%d", name, i+1)

		// ── the launch rules.
		if isLaunch(i) {
			launches++
			if !captures(i) {
				exempt = append(exempt, at+": "+strings.TrimSpace(lines[i]))
			} else {
				if p := prev(i); p < 0 || strings.TrimSpace(code[p]) != "set -m" {
					complaints = append(complaints, at+": a background launch whose pid is captured "+
						"is not under `set -m`, so that pid is not a process-group id and "+
						"`kill -SIG -- \"-$pid\"` reaps the wrapper only (ranger-base-q8hbz): "+strings.TrimSpace(lines[i]))
				}
				if strings.HasPrefix(c, "(") &&
					!pkHasUnquoted(code[i], mask[i], ';') &&
					!pkExec.MatchString(code[i]) {
					complaints = append(complaints, at+": a single-command subshell launch does not `exec`, "+
						"so the group leader is a wrapper and `kill -0` speaks about the wrong "+
						"process (ranger-base-q8hbz): "+strings.TrimSpace(lines[i]))
				}
			}
		}

		// ── the reaper rule, read at the FIRST killing verb on the line.
		for _, loc := range pkSignalKill.FindAllStringSubmatchIndex(code[i], -1) {
			if code[i][loc[4]:loc[5]] == "0" { // `kill -0` lists, it does not kill
				continue
			}
			verb := loc[3] // past the (^|[^-\w]) guard: the `kill` itself
			if mask[i][len([]rune(code[i][:verb]))] {
				continue // inside a string: an echo about a kill is not one
			}
			kills++
			if !pkGroupKillHere.MatchString(code[i][verb:]) {
				complaints = append(complaints, at+": a kill with an explicit signal does not try the "+
					"process GROUP first (`kill -SIG -- \"-$pid\"`), so a job whose work is a CHILD "+
					"of the pid outlives its reaper (ranger-base-q8hbz): "+strings.TrimSpace(lines[i]))
			}
			break // one complaint per line, whatever it chains to
		}
	}
	return complaints, launches, kills, exempt
}

// TestThePkAuditFlagsTheShapesItIsFor is pkAudit's control arm, and it is the
// same demand TestThePatternKillDetectorFlagsTheLinesItIsFor makes of the
// sweep: a reader that answers "no complaint" to everything is a green light,
// not a measurement. The must-flag text is the PRE-FIX shape of
// verify-govern-honesty.sh's start_loop and kill_loop, spelled the way the
// finding that filed this arm measured it.
func TestThePkAuditFlagsTheShapesItIsFor(t *testing.T) {
	// One complaint per rule, each one the real defect.
	for _, c := range []struct {
		name string
		want string
		text string
	}{{
		name: "no set -m",
		want: "not under `set -m`",
		text: "start_loop() {\n\t(cd \"$W\" && exec env \"$P\" dispatch --watch 5m >\"$L\" 2>&1) &\n\tLOOP_PID=$!\n}\n",
	}, {
		name: "no exec",
		want: "does not `exec`",
		text: "start_loop() {\n\tset -m\n\t(cd \"$W\" && env \"$P\" dispatch --watch 5m >\"$L\" 2>&1) &\n\tLOOP_PID=$!\n\tset +m\n}\n",
	}, {
		name: "the pre-fix pid-only reaper",
		want: "does not try the process GROUP first",
		text: "kill_loop() {\n\tkill -9 \"$pid\" >/dev/null 2>&1 || true\n}\n",
	}, {
		name: "the pre-fix cockpit reaper",
		want: "does not try the process GROUP first",
		text: "\tkill -INT \"$cp\" >/dev/null 2>&1 || true\n",
	}, {
		// The case only the FORWARD capture read catches, and the escape
		// ranger-base-wenqb measured: the pre-fix shape restored under one
		// line of re-layout. `$!` survives an intervening foreground
		// command, so this launch's pid IS handed out and rules A and D
		// apply — a reader that looks only at the immediately-next
		// statement calls it uncaptured and waives both, silently, with
		// `launches++` still firing.
		name: "a capture one statement below the launch",
		want: "not under `set -m`",
		text: "start_loop() {\n\t:\n\t(cd \"$W\" && env \"$P\" dispatch --watch 5m >\"$L\" 2>&1) &\n\tprintf 'launched\\n' >&2\n\tLOOP_PID=$!\n}\n",
	}, {
		// The case only the ANCHORED read catches: a group kill is present
		// on the line, and it is for a different job than the pid-only kill
		// that runs first. An unanchored `does this line contain a group
		// form` call this clean, which is why the rule is anchored at the
		// first killing verb.
		name: "a pid-only kill chained before a group kill",
		want: "does not try the process GROUP first",
		text: "\tkill -9 \"$a\" 2>/dev/null || kill -9 -- \"-$b\" 2>/dev/null\n",
	}} {
		got, _, _, _ := pkAudit("fixture.sh", c.text)
		if len(got) == 0 {
			t.Errorf("%s: pkAudit flagged nothing, so a clean sweep of the real scripts measures nothing:\n%s", c.name, c.text)
			continue
		}
		if !strings.Contains(strings.Join(got, "\n"), c.want) {
			t.Errorf("%s: pkAudit complained, but not about %q:\n\t%s", c.name, c.want, strings.Join(got, "\n\t"))
		}
	}

	// And the must-pass text: every exemption this reader grants, spelled as
	// the shipped line that needs it. A reader this noisy gets waived.
	for _, c := range []struct {
		name string
		text string
	}{{
		name: "the fixed start_loop",
		text: "\tset -m\n\t(cd \"$W\" && exec env \"$P\" dispatch --watch 5m >\"$L\" 2>&1) &\n\tLOOP_PID=$!\n\tset +m\n",
	}, {
		// The same shape with the re-layout: reading the capture forward
		// must not turn a correct launch into a complaint either.
		name: "the fixed start_loop with a statement before its capture",
		text: "\tset -m\n\t(cd \"$W\" && exec env \"$P\" dispatch --watch 5m >\"$L\" 2>&1) &\n\tprintf 'launched\\n' >&2\n\tLOOP_PID=$!\n\tset +m\n",
	}, {
		name: "the group-first reaper and its fallback",
		text: "\tkill -9 -- \"-$pid\" >/dev/null 2>&1 || kill -9 \"$pid\" >/dev/null 2>&1 || true\n",
	}, {
		name: "kill -0 is a liveness probe",
		text: "\tkill -0 \"$1\" 2>/dev/null && return 0\n",
	}, {
		name: "a signal-less courtesy TERM whose backstop is cleanup",
		text: "\t\tkill \"$server\" 2>/dev/null\n",
	}, {
		name: "a launch whose pid is never taken",
		text: "unset_herdr\nenv RHQ_HOME=\"$H\" \"$HERDR\" --session \"$S\" server >/dev/null 2>&1 &\nwait_server || exit 2\n",
	}, {
		name: "a subshell that must outlive its command cannot exec",
		text: "\tset -m\n\t( RHQ_HOME=$R/never \"$p\" version >\"$p.out\" 2>\"$p.err\"; echo $? >\"$p.rc\" ) &\n\tlocal child=$!\n\tset +m\n",
	}, {
		name: "a kill named in a comment or an echo",
		text: "# what stood here was `kill -9 $LOOP_PID`, which took the wrapper\n\techo \"the loop ($pid) survived kill -9 — the reaper failed\"\n",
	}, {
		name: "&& at the end of a line is a continuation",
		text: "\t( cd \"$t\" && git init -q . &&\n\t\tgit add F ) >\"$log\" 2>&1\n",
	}} {
		if got, _, _, _ := pkAudit("fixture.sh", c.text); len(got) != 0 {
			t.Errorf("%s: pkAudit complained about a line that is correct as it stands:\n\t%s", c.name, strings.Join(got, "\n\t"))
		}
	}

	// And the exempt list, graded here rather than only where it is used. It
	// is what the shipped-script arm pins by name, so a reader that exempts
	// every launch — or none — has to fail in this control rather than pass
	// there for the wrong reason. Each case gives the launches it contains
	// and how many of them hand out no pid.
	for _, c := range []struct {
		name              string
		launches, exempts int
		text              string
	}{{
		name: "a launch whose pid is never taken is exempt",
		// The shipped scratch herdr server: ended by `herdr session stop`.
		launches: 1, exempts: 1,
		text: "unset_herdr\nenv RHQ_HOME=\"$H\" \"$HERDR\" --session \"$S\" server >/dev/null 2>&1 &\nwait_server || exit 2\n",
	}, {
		name:     "a capture one statement below the launch is NOT exempt",
		launches: 1, exempts: 0,
		text: "\tset -m\n\t(exec \"$P\" run) &\n\tprintf 'launched\\n' >&2\n\tLOOP_PID=$!\n\tset +m\n",
	}, {
		// The bound on the forward read, and the reason it is the next
		// LAUNCH rather than the end of the block: after a second launch,
		// `$!` names that one. The first job's pid is genuinely never taken.
		name:     "a `$!` below a SECOND launch names that one, so the first is exempt",
		launches: 2, exempts: 1,
		text: "\tset -m\n\t(exec \"$A\") &\n\tset -m\n\t(exec \"$B\") &\n\tB_PID=$!\n\tset +m\n",
	}} {
		got, launches, _, exempt := pkAudit("fixture.sh", c.text)
		if launches != c.launches {
			t.Errorf("%s: pkAudit saw %d background launches, want %d — the exempt count below "+
				"says nothing if the launches were not recognized:\n%s", c.name, launches, c.launches, c.text)
			continue
		}
		if len(exempt) != c.exempts {
			t.Errorf("%s: pkAudit exempted %d of %d launches from the `set -m` and `exec` rules, want %d:\n\t%s",
				c.name, len(exempt), launches, c.exempts, strings.Join(exempt, "\n\t"))
		}
		if len(got) != 0 {
			t.Errorf("%s: pkAudit complained about a fixture that is correct as it stands:\n\t%s",
				c.name, strings.Join(got, "\n\t"))
		}
	}
}

// TestTheReapingScriptsKeepTheirGroupShape reads what posse ships. It is the
// half TestTheGroupReaperEndsWhatThePatternKillCouldNot cannot reach: that arm
// proves the SHAPE reaps, this one proves the two scripts still have it.
func TestTheReapingScriptsKeepTheirGroupShape(t *testing.T) {
	// Named rather than globbed. Every script that launches a job it later
	// reaps by pid is one of these two, and that is asserted below rather
	// than assumed: a third one appearing is a change this pin should be
	// told about, in a failure that names it.
	want := []string{"scripts/verify-govern-honesty.sh", "scripts/macos-install-probe.sh"}
	var totalLaunches, totalKills int
	var totalExempt []string
	for _, rel := range want {
		b, err := os.ReadFile(filepath.FromSlash(rel))
		if err != nil {
			t.Fatalf("read %s: %v — this pin needs the shipped script, and a missing one "+
				"is not a clean result", rel, err)
		}
		complaints, launches, kills, exempt := pkAudit(rel, string(b))
		totalLaunches += launches
		totalKills += kills
		totalExempt = append(totalExempt, exempt...)
		if len(complaints) > 0 {
			t.Errorf("%s no longer carries the shape ranger-base-q8hbz landed:\n\t%s",
				rel, strings.Join(complaints, "\n\t"))
		}
	}
	// The floor is the positive witness, and it is the census at the time of
	// writing: five background launches (two in verify-govern-honesty.sh's
	// start_loop and cockpit_frame, its scratch herdr server, and
	// macos-install-probe.sh's run_bounded and loopback http.server) and four
	// explicit-signal kills (kill_loop, cockpit_frame's INT and its -9, and
	// macos-install-probe.sh's reap). A parse that quietly stops recognizing
	// them reads exactly like two clean scripts.
	if totalLaunches < 5 || totalKills < 4 {
		t.Fatalf("the reader found %d background launches and %d explicit-signal kills in %v — "+
			"it is not recognizing them, so a clean result measures nothing", totalLaunches, totalKills, want)
	}

	// The floor cannot see the OTHER way a launch stops being measured: a
	// launch that hands out no pid is exempt from rules A and D, and
	// `launches++` fires for it just the same, so an exemption that grows
	// leaves the floor satisfied and the shape unread. MEASURED
	// (ranger-base-wenqb) before the forward capture read landed above: one
	// `printf` between start_loop's launch and its `LOOP_PID=$!` took it out
	// of both rules, and the pre-fix shape restored underneath was green on
	// every reader of either script. So the exemption is pinned by NAME. A
	// new one is either a genuine fire-and-forget job — add it here, with why
	// its pid is never taken — or a launch that has quietly left the ruleset.
	// Keyed on the launch itself rather than on `file:line`, so an edit
	// anywhere above it does not red this pin for a reason that has nothing
	// to do with the shape — which is what a line number would do here, and
	// did while this was being written.
	wantExempt := []struct{ file, launch, why string }{{
		file: "scripts/verify-govern-honesty.sh", launch: `"$HERDR" --session`,
		why: "the scratch herdr server, ended by `herdr session stop` and by nothing else",
	}}
	matched := make([]int, len(wantExempt))
	for _, e := range totalExempt {
		at, _, _ := strings.Cut(e, ": ")
		file, _, _ := strings.Cut(at, ":")
		hit := false
		for i, w := range wantExempt {
			if file == w.file && strings.Contains(e, w.launch) {
				matched[i]++
				hit = true
			}
		}
		if !hit {
			t.Errorf("a background launch is exempt from the `set -m` and `exec` rules because its pid is "+
				"never captured, and it is not one of the %d that is meant to be — either it is a new "+
				"fire-and-forget job (name it in `wantExempt` with why its pid is never taken), or it "+
				"captures a pid this reader stopped seeing and it has left the ruleset silently "+
				"(ranger-base-wenqb):\n\t%s", len(wantExempt), e)
		}
	}
	for i, w := range wantExempt {
		if matched[i] != 1 {
			t.Errorf("%s: %q matched %d exempt launches, want exactly 1 (%s) — a named exemption that no "+
				"longer happens is fine to delete here, but not to leave: this is what says the reader "+
				"still recognizes the launch at all", w.file, w.launch, matched[i], w.why)
		}
	}

	// And the two named files are the whole class, asserted by the property
	// that defines it rather than by a memory of the tree: a script that
	// reaps a process GROUP is a script with a job whose work is a CHILD of
	// the pid it holds, which is what this reader is for. A third one
	// appearing is a change this pin must be told about, in a failure that
	// names it.
	//
	// THE LIMIT, said out loud: this does NOT find a new script that grows
	// the pre-fix shape from scratch — a wrapper launch reaped by pid alone
	// looks, statically, exactly like scripts/suite-lock.sh's arms, where the
	// captured pid IS the work, every job is `wait`ed, and `kill -9 "$h2"` is
	// the measurement itself (release is process death, ranger-base-2fgu4).
	// Telling those apart needs a dataflow read of shell that is not worth
	// building for it. What covers the new script is
	// TestNoShippedShellScriptCallsAPatternKill above, which catches the
	// spelling the defect actually arrives in.
	out, err := exec.Command("git", "ls-files", "-z").Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	named := map[string]bool{want[0]: true, want[1]: true}
	var strays []string
	for _, rel := range strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00") {
		if rel == "" || !strings.HasSuffix(rel, ".sh") || named[rel] {
			continue
		}
		b, err := os.ReadFile(filepath.FromSlash(rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			c, _ := pkShellCode(line)
			if pkGroupKill.MatchString(c) {
				strays = append(strays, fmt.Sprintf("%s:%d: %s", rel, i+1, strings.TrimSpace(line)))
			}
		}
	}
	if len(strays) > 0 {
		t.Errorf("a shipped script outside %v reaps a process group, so it holds a job whose work is a "+
			"CHILD of its pid and it has a launch shape this pin does not read — add it to `want` above:\n\t%s",
			want, strings.Join(strays, "\n\t"))
	}
}
