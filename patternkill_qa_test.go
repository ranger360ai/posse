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
// WHAT IS PINNED HERE, and why it is two tests. The first is the sweep: no
// shipped shell script may name `pkill` or `killall` outside a quote or a
// comment. A sweep that finds nothing proves nothing on its own, so its
// detector is graded first against a fixture table that carries the exact
// pre-fix lines it has to flag and the real corpus lines it must not.
//
// The second is the mechanism, executed rather than read, because the sweep
// only says the verb is gone — not that what replaced it reaps anything. It
// runs both reaper shapes against a live fixture with a REFUSING pkill at the
// head of PATH, the shim's own shape, and reads the kernel: the old shape has
// to leave its grandchild alive (without that arm the new one is measuring
// the absence of a defect that could never have happened here), and the new
// one has to leave nothing.
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
	if scanned < 12 {
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
