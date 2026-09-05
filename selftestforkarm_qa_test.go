package posse

// PARKED QA pin for ranger-base-t07yx finding 1, found verifying
// ranger-base-7hx87's close under ranger-base-kl9ui.
//
// THE DEFECT. ranger-base-7hx87 was an arm of `scripts/test-times.sh
// --self-test` reporting a line absent when what had failed was the matcher,
// and 0b5c1c4 fixed it by taking the fork out of the two `printf | grep -q`
// conditions. One line below them, the cross-check arm still reads
//
//	got_mnt=$(printf '%s' "$out" | sed -n 's/.*DISK: [0-9]* MB free on \(.*\) — .*/\1/p' | head -1)
//
// (scripts/test-times.sh:831). A `sed` that is signalled, or that cannot be
// forked under load — the condition the 2026-09-02 sighting happened in —
// yields an empty $got_mnt, and the arm then reports the DISK line as naming
// nothing. MEASURED 2026-09-05 at 51b1195, two consecutive lines of ONE run
// with a `sed` on PATH whose whole body is `kill -TERM $$`:
//
//	ok    disk: the preflight line names free MB, the filesystem and what fills it
//	FAIL  disk: the line names ''; df says $TMPDIR is on '/System/Volumes/Data'
//
// The fixed arm and the arm below it disagree about the same $out, and the
// second one is wrong. That contradiction is what this pin holds, because it
// needs no tolerance and no fixture: one run, two arms, and only the
// apparatus between them.
//
// It matters here rather than in some cold corner because `make test` runs
// verify-test-times FIRST, so this reds a suite before a single package runs,
// with a message about a DISK line — the least likely thing a reader connects
// to their diff. That is the whole reason ranger-base-7hx87 was a P2.
//
// UN-SKIPPED by ranger-base-t07yx, which took the fork out of the arm. The
// cross-check now reads the mount point out of $out with `line_after` and
// `${...}`, and asks df for the expectation in one capture that bash parses
// rather than a `df | awk` pipeline — so neither side of the comparison can be
// emptied by a matcher that never ran. MEASURED after the fix: with the same
// dying `sed` on PATH both disk arms report ok, where before the second said
// the line named ''.
//
// The arm also stopped conflating the two failures it can have. When df itself
// produces no table there is nothing to compare against, and saying "the line
// names ''" would blame the script for the box; it now says THIS ARM DID NOT
// RUN and dumps $out.
//
// The control arm below is not decoration. A probe that sabotages `sed` could
// pass by breaking the run so thoroughly that neither disk arm reports at all,
// so the same self-test has to be shown green and both arms ok with the real
// `sed` on PATH before the sabotaged run is allowed to mean anything.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const (
	// The two arm texts this pin rests on, spelled as the script prints them.
	sfPreflightOK = "ok    disk: the preflight line names free MB"
	sfCrossOK     = "ok    disk: the line names the filesystem df attributes"
	sfCrossFAIL   = "FAIL  disk: the line names ''"
)

// sfSelfTest runs the self-test with dir prepended to PATH (empty for none)
// and answers its combined output. The exit status is deliberately not the
// test: a sabotaged run is expected to be non-zero, and what this pin reads
// is which arms reported what.
func sfSelfTest(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("bash", "scripts/test-times.sh", "--self-test")
	if dir != "" {
		cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	out, err := cmd.CombinedOutput()
	if _, ok := err.(*exec.ExitError); err != nil && !ok {
		t.Fatalf("bash scripts/test-times.sh --self-test: %v\n%s", err, out)
	}
	return string(out)
}

// sfDyingBin writes a `name` on PATH whose whole body is a TERM to itself.
// A fork failure under load (rc 127) has the same shape and is silent but for
// a stderr line, so this stands in for the load condition without needing one.
func sfDyingBin(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/bash\nkill -TERM $$\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestQATheSelfTestsDiskArmsDoNotContradictEachOtherWhenAForkFails(t *testing.T) {
	// CONTROL. With the real `sed` the self-test is green and both disk arms
	// report ok, so the sabotaged run below is measuring the apparatus and not
	// a script that was broken to begin with.
	clean := sfSelfTest(t, "")
	for _, want := range []string{sfPreflightOK, sfCrossOK} {
		if !strings.Contains(clean, want) {
			t.Fatalf("control: the unsabotaged self-test does not report %q, so this pin measures nothing:\n%s", want, clean)
		}
	}

	// And the run under a `sed` that dies of TERM. The DISK line is produced
	// by df and awk, so it is still there and still well formed — the arm
	// above says so in this very output.
	got := sfSelfTest(t, sfDyingBin(t, "sed"))
	if !strings.Contains(got, sfPreflightOK) {
		t.Fatalf("the preflight arm did not report ok under a sabotaged sed, so the contradiction this pin is about cannot arise here:\n%s", got)
	}
	if strings.Contains(got, sfCrossFAIL) {
		t.Errorf("one self-test run says the DISK line names free MB, the filesystem and what fills it, and then says it names '' — the second arm reported a failed fork as a missing filesystem (scripts/test-times.sh:831, ranger-base-t07yx). make test runs this gate before any package, so it reds a suite with a message about a disk:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// THE SCAN. ranger-base-t07yx asked for a pin over the SHAPE rather than a QA
// allowlist of the sites known today, because the sites known today were
// already wrong twice: ranger-base-7hx87 fixed two `printf | grep -q`
// conditions and left the `sed -n | head -1` on the next line; the verify bead
// that caught that listed three more files and missed scripts/suite-lock.sh,
// which `make test` gates on through verify-suite-lock.
//
// THE INVARIANT: in an assertion arm the MATCHER MUST NOT FORK. Run the thing
// under test — that fork IS the measurement — capture its output, and decide
// with `case`, `[[ ]]` and `${...}`. A matcher that is signalled (143/137),
// that cannot be exec'd under load (127, silent but for a stderr line), or
// that takes EPIPE past the 64 KB pipe buffer (141) otherwise reports the
// property false when the apparatus is what failed.
//
// Two roles are allowed and both are decided mechanically from the line, not
// from a list of places:
//
//	MESSAGE — the fork sits inside an argument to `bad`/`fail`/`note`/`die`/
//	          `say`. The verdict has already been decided; the worst a dead
//	          matcher does is blank the explanation.
//	FIXTURE — `cat` writing a file or opening a heredoc. A dead `cat` there
//	          does not misreport one arm, it fails the whole rig loudly, and
//	          spelling every rig script as `printf` would cost more than it
//	          buys.
//
// Heredoc BODIES are skipped: they are data the script writes out, not its own
// assertion path.
//
// Both of those roles are bounded, because ranger-base-u2etb measured three
// ways they were not (all three green over a planted violation at 3851edc):
//
//	A MESSAGE ends where the helper's command does. `bad "x"; if printf …
//	| grep -q foo` is a verdict with a helper in front of it, not a message,
//	and so is the same shape on a `bad … \` continuation line. The extent is
//	decided by sfMessageReaches, which walks quoting and `$( … )` nesting.
//	A HEREDOC that never closes is not a heredoc. `<<EOF` in a comment, or
//	`$((1<<shift))`, used to open a body that swallowed every remaining line
//	of the file with the pin still green — one planted line took the scan
//	from 123 occurrences to 118, nowhere near the floor. Those two spellings
//	are blinded, and because no enumeration of them is ever complete, a
//	heredoc still open at EOF is itself reported as a failure.

var (
	sfRegionStart = regexp.MustCompile(`^[a-zA-Z_]*[sS]elf_?[tT]est[a-zA-Z_]*\(\)\s*\{`)
	sfMatcher     = regexp.MustCompile(`(^|[|&;(]|\$\(|` + "`" + `|\b(?:if|elif|while|until)\s+|!\s+)\s*(grep|sed|awk|head|tail|cut|tr|sort|uniq|wc|cat|jq|paste|xargs)\b`)
	sfHeredoc     = regexp.MustCompile(`<<-?[ \t]*(['"]?)([A-Za-z_][A-Za-z0-9_]*)['"]?`)
	sfArith       = regexp.MustCompile(`\$?\(\([^()]*\)\)`)
	sfAssertHelp  = regexp.MustCompile(`\b(bad|fail|note|die|say)\b[ \t]+["'$]`)
)

// sfUnswept — scripts whose assertion arms still decide through an exec'd
// matcher. This is a LEDGER of outstanding work, not a definition of the
// shape: a NEW violation in any script that is not on this list reds without
// anybody editing anything, which is the property a site allowlist could not
// have. Sweeping a file means deleting its row.
var sfUnswept = map[string]string{
	"verify-bd-dep-safety.sh":  "ranger-base-s8b4g",
	"verify-bd-pin.sh":         "ranger-base-s8b4g",
	"verify-codex-pin.sh":      "ranger-base-s8b4g",
	"verify-detection.sh":      "ranger-base-s8b4g",
	"verify-ghost-composer.sh": "ranger-base-s8b4g",
	"verify-govern-honesty.sh": "ranger-base-s8b4g",
	"verify-grok-pin.sh":       "ranger-base-s8b4g",
	"verify-hook-freshness.sh": "ranger-base-s8b4g",
	"verify-id-recycle.sh":     "ranger-base-s8b4g",
	"verify-orphan-report.sh":  "ranger-base-s8b4g",
	"verify-pid-deny-set.sh":   "ranger-base-s8b4g",
	"verify-prune-guard.sh":    "ranger-base-s8b4g",
	"verify-self-close.sh":     "ranger-base-s8b4g",
}

type sfHit struct {
	file, text string
	line       int
	role       string // "verdict" (a violation), "message", "fixture", "heredoc"
}

// sfMessageReaches reports whether a matcher whose match ends at `to` is still
// inside the argument list of the helper call that starts at `from`. A message
// role is earned only while the helper's own command is still being written:
// an unquoted, top-level `;`, `|` or `&` ends it, and a matcher past that
// decides a verdict of its own. Separators nested inside a `$( … )` or inside
// a quoted string end nothing — that nesting is the shape every legitimate
// message fork in the tree already has.
//
// Before ranger-base-u2etb the role test was `startsMessage[0] < loc[1]`
// alone, so `bad "checking"; if printf … | grep -q foo; then` read as a
// message for all five helpers, and the same hole was inherited by every
// `bad … \` continuation line.
func sfMessageReaches(l string, from, to int) bool {
	if to > len(l) {
		to = len(l)
	}
	depth := 0
	quote := byte(0)
	var stack []byte // quote context to restore when a `$( … )` closes
	for i := from; i < to; i++ {
		c := l[i]
		switch {
		case quote == '\'':
			if c == '\'' {
				quote = 0
			}
		case c == '\\':
			i++ // escapes one byte, in `"` and unquoted alike
		case quote == '"':
			switch {
			case c == '"':
				quote = 0
			case c == '$' && i+1 < to && l[i+1] == '(':
				stack, quote, depth = append(stack, quote), 0, depth+1
				i++
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '$' && i+1 < to && l[i+1] == '(':
			stack, quote, depth = append(stack, quote), 0, depth+1
			i++
		case c == '(':
			stack, depth = append(stack, quote), depth+1
		case c == ')':
			if depth > 0 {
				depth--
				quote, stack = stack[len(stack)-1], stack[:len(stack)-1]
			}
		case depth == 0 && (c == ';' || c == '|' || c == '&'):
			return false
		}
	}
	return true
}

// sfScanLines classifies every matcher occurrence on an assertion path.
// Exported as a function over lines so the controls below can drive it with
// synthetic input and prove it separates the three roles.
//
// It answers, second, the heredoc opener it was still holding at EOF. A
// heredoc that never closes is not a heredoc — it is a line the scan misread,
// and it silently swallowed every line after it. The caller must red on that
// rather than report the clean tail it produces, because no enumeration of
// phantom spellings is ever complete (ranger-base-u2etb).
func sfScanLines(file string, lines []string, wholeFile bool) ([]sfHit, *sfHit) {
	var hits []sfHit
	var opener *sfHit
	in := wholeFile
	heredoc := ""
	inMessage := false
	for i, raw := range lines {
		// A heredoc body is data the script writes, not its assertion path.
		if heredoc != "" {
			if strings.TrimSpace(raw) == heredoc {
				heredoc, opener = "", nil
			}
			continue
		}
		l := strings.TrimLeft(raw, " \t")
		// A comment carries no assertion AND opens no heredoc. This test
		// runs BEFORE the opener probe: a comment that mentions `<<EOF`
		// used to open a body that never closed, and the rest of the file
		// went unscanned with the pin green (ranger-base-u2etb 1a).
		if strings.HasPrefix(l, "#") {
			continue
		}
		// `<<<` is a here-STRING and opens no body, and `$((1<<shift))` is
		// a left shift by a variable; blind both rather than trying to
		// express "not those" in a Go regexp.
		probe := sfArith.ReplaceAllString(strings.ReplaceAll(raw, "<<<", "\x00\x00\x00"), "")
		if m := sfHeredoc.FindStringSubmatch(probe); m != nil {
			heredoc = m[2]
			opener = &sfHit{file: file, line: i + 1, text: l, role: "heredoc"}
		}
		if !wholeFile {
			if !in {
				if sfRegionStart.MatchString(l) {
					in = true
				}
				continue
			}
			if strings.HasPrefix(raw, "}") {
				in = false
				continue
			}
		}
		// A `bad …` / `fail …` call continued with a trailing backslash keeps
		// its message role on the next line — suite-lock.sh writes most of
		// its failure messages that way.
		startsMessage := sfAssertHelp.FindStringIndex(l)
		continues := strings.HasSuffix(strings.TrimRight(raw, " \t"), `\`)
		// Read the carried flag BEFORE updating it: this line is part of the
		// message when the PREVIOUS line opened one, and clearing first made
		// every `bad … \` continuation classify as a verdict.
		carried := inMessage
		inMessage = (carried || startsMessage != nil) && continues

		loc := sfMatcher.FindStringSubmatchIndex(l)
		if loc == nil {
			continue
		}
		tool := l[loc[4]:loc[5]]
		role := "verdict"
		switch {
		case carried && sfMessageReaches(l, 0, loc[1]):
			role = "message"
		case startsMessage != nil && startsMessage[0] < loc[1] && sfMessageReaches(l, startsMessage[0], loc[1]):
			role = "message"
		case tool == "cat" && (strings.Contains(l[loc[5]:], ">") || strings.Contains(l, "<<")):
			role = "fixture"
		}
		hits = append(hits, sfHit{file: file, line: i + 1, text: l, role: role})
	}
	return hits, opener
}

func sfScanScripts(t *testing.T) []sfHit {
	t.Helper()
	files, err := filepath.Glob("scripts/*.sh")
	if err != nil || len(files) < 20 {
		t.Fatalf("globbed %d scripts (%v) — the scan is reading almost nothing, so a clean result is not evidence", len(files), err)
	}
	var all []sfHit
	for _, f := range files {
		base := filepath.Base(f)
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		// A verify-*.sh is assertions from top to bottom; cleanroom.sh is a
		// preflight prober with the same shape and no --self-test to scope to.
		whole := strings.HasPrefix(base, "verify-") || base == "cleanroom.sh"
		hits, open := sfScanLines(base, strings.Split(string(b), "\n"), whole)
		if open != nil {
			t.Errorf("scripts/%s:%d opens a heredoc that never closes, so the scan skipped every line after it and a clean result below is not evidence — either the line is not a redirection and the opener probe must be blinded to it, or the script is malformed:\n  %s", base, open.line, open.text)
		}
		all = append(all, hits...)
	}
	return all
}

func TestQANoAssertionArmDecidesThroughAForkedMatcher(t *testing.T) {
	hits := sfScanScripts(t)

	// THE FLOOR, and it is the positive witness this whole test rests on: a
	// regexp that stopped matching, or a region finder that stopped finding
	// self-test bodies, would leave `hits` empty and this test green over a
	// scan that reads nothing at all. There were 123 occurrences across 19
	// files when this was re-measured (2026-09-05, ranger-base-u2etb; the
	// 126 written here before is not a count this tree produces at 3851edc
	// or at HEAD, and the two scanners agree on 123 hit for hit).
	if len(hits) < 80 {
		t.Fatalf("the scan found only %d matcher occurrences across scripts/ — it was 123 when last measured, so the shape or the region finder has gone blind and a clean result here means nothing", len(hits))
	}
	var roles = map[string]int{}
	for _, h := range hits {
		roles[h.role]++
	}
	if roles["message"] == 0 || roles["fixture"] == 0 {
		t.Fatalf("the classifier put nothing in message (%d) or fixture (%d) — it is no longer separating the roles, so every allowed fork would read as a violation and every violation as allowed", roles["message"], roles["fixture"])
	}

	var bad []string
	stillDirty := map[string]bool{}
	for _, h := range hits {
		if h.role != "verdict" {
			continue
		}
		if _, known := sfUnswept[h.file]; known {
			stillDirty[h.file] = true
			continue
		}
		text := h.text
		if len(text) > 100 {
			text = text[:100] + "…"
		}
		bad = append(bad, fmt.Sprintf("scripts/%s:%d: %s", h.file, h.line, text))
	}
	if len(bad) > 0 {
		t.Errorf("%d assertion arm(s) decide through a forked matcher:\n  %s\n\n"+
			"In an assertion arm the MATCHER MUST NOT FORK (ranger-base-t07yx,\n"+
			"ranger-base-7hx87). A grep/sed/awk/head that is signalled, that cannot be\n"+
			"exec'd under load, or that takes EPIPE past the 64 KB pipe buffer makes the\n"+
			"arm report the property false when the apparatus is what failed — and\n"+
			"`make test` gates on verify-test-times and verify-suite-lock before a single\n"+
			"package runs.\n\n"+
			"Run the thing under test, capture its output, and decide with `case`,\n"+
			"`[[ ]]` and `${...}`. scripts/test-times.sh's `line_after`,\n"+
			"scripts/suite-lock.sh's `log_has`/`marker_field` and\n"+
			"scripts/audit-silent-reverts.sh's `sr_has`/`sr_count`/`sr_field2` are the\n"+
			"shapes already in the tree. Do not add a row to sfUnswept for new work.",
			len(bad), strings.Join(bad, "\n  "))
	}

	// A row that has been swept must not sit here claiming debt that is paid:
	// a stale ledger is how the next reader learns to ignore it.
	for f := range sfUnswept {
		if !stillDirty[f] {
			t.Errorf("scripts/%s is on sfUnswept but has no forked-matcher verdict left — delete its row (the sweep is ranger-base-s8b4g)", f)
		}
	}
}

// The controls. A scan is worth exactly what it can be shown to catch, and
// this one has three ways to be quietly wrong: miss a violation, call an
// allowed fork a violation, or classify from a line it never actually read.
func TestQATheForkedMatcherScanSeparatesTheThreeRoles(t *testing.T) {
	cases := []struct {
		want, line string
	}{
		// Verdicts — every spelling this class has actually appeared in.
		{"verdict", `if printf '%s' "$out" | grep -q 'TestBeta'; then`},
		{"verdict", `if grep -q 'waiting for suite lock held by .*/' "$tmp/m3.log" 2>/dev/null; then`},
		{"verdict", `got_mnt=$(printf '%s' "$out" | sed -n 's/.*on \(.*\) —.*/\1/p' | head -1)`},
		{"verdict", `want_mnt=$(df -kP "${TMPDIR:-/tmp}" 2>/dev/null | awk 'NR==2 { print $NF }')`},
		{"verdict", `n=$(printf '%s\n' "$out" | grep -c 'path(s) went backwards')`},
		{"verdict", `if [ "$(tr '\n' ' ' < "$tmp/argv")" = "test -timeout 25m ./... " ]; then`},
		{"verdict", `out=$(cat "$tmp/out")`},
		{"verdict", `	! grep -q 'not a positive integer' "$tmp/m14.log" 2>/dev/null; then`},
		{"verdict", `cachedcount() { find "$T/cache" -name '*.test' | wc -l | tr -d ' '; }`},
		// Messages — the fork runs after the verdict is decided.
		{"message", `*) bad "budget not read: $(printf '%s' "$out" | grep 'package times')" ;;`},
		{"message", `bad "$arm14" "the holder never acquired: $(tr '\n' '|' <"$tmp/m18.log")"`},
		// Fixtures — writing the rig, not reading a verdict.
		{"fixture", `cat >"$tmp/holder.sh" <<'HOLDER'`},
		{"fixture", `cat <<'EOF'`},
		{"fixture", `	cat >>"$pkg/a_test.go" <<-'EOF'`},
	}
	for _, c := range cases {
		got, _ := sfScanLines("probe.sh", []string{c.line}, true)
		if len(got) != 1 {
			t.Errorf("classifier saw %d hits, want 1, on: %s", len(got), c.line)
			continue
		}
		if got[0].role != c.want {
			t.Errorf("classified as %q, want %q: %s", got[0].role, c.want, c.line)
		}
	}

	// Lines that must produce NO hit at all: the fixed shapes. If the scan
	// flagged these, the fix it demands would be impossible to write.
	for _, l := range []string{
		`case $out in *TestBeta*) say "ran" ;; esac`,
		`if [[ $before == *'slot 1: HELD'* ]]; then`,
		`got_mnt=${rest%%' — '*}`,
		`argv_seen=$(<"$tmp/argv")`,
		`suspects=$(signal_suspects 505 <<<"$table")`,
		`sr_has() { case $1 in *"$2"*) return 0 ;; esac; return 1; }`,
	} {
		if got, _ := sfScanLines("probe.sh", []string{l}, true); len(got) != 0 {
			t.Errorf("the scan flags a FIXED line as %q — the invariant it demands cannot be satisfied: %s", got[0].role, l)
		}
	}

	// A heredoc body is skipped, and the here-STRING that looks like one is
	// not: `<<<` opens no body, so blinding on it would swallow the rest of
	// the file and turn every later violation invisible.
	body := []string{`cat >"$t/rig.sh" <<'RIG'`, `  grep -q x "$f" || exit 1`, `RIG`, `if grep -q 'y' "$out"; then`}
	got, _ := sfScanLines("probe.sh", body, true)
	if len(got) != 2 || got[0].role != "fixture" || got[1].role != "verdict" || got[1].line != 4 {
		t.Errorf("heredoc handling is wrong: %+v — want the opener as fixture and line 4 as a verdict, with the body skipped", got)
	}
	hs := []string{`suspects=$(signal_suspects 505 <<<"$table")`, `if grep -q 'y' "$out"; then`}
	if got, _ := sfScanLines("probe.sh", hs, true); len(got) != 1 || got[0].line != 2 {
		t.Errorf("a here-string was read as opening a heredoc body, so everything after it goes unscanned: %+v", got)
	}
}

// The three blind spots ranger-base-u2etb measured, each shown GREEN over a
// planted violation in scripts/test-times.sh before the fix. They share one
// failure mode — the scan reports a clean file it never finished reading, or
// never finished classifying — which the 80-floor cannot see, because dropping
// five lines out of 123 occurrences is not a floor breach.
func TestQATheForkedMatcherScanSeesPastAPhantomHeredocAndAnEarlierMessage(t *testing.T) {
	// 1a. PHANTOM HEREDOC. `<<WORD` in a comment and `$((1<<n))` are not
	// redirections; before the fix either one opened a body that never
	// closed and the violation on the next line was invisible.
	for _, phantom := range []string{
		"# the rig is written with a <<EOF body",
		"  sh=$((1<<shift))",
		"  n=$(( 1 << shift ))",
	} {
		lines := []string{phantom, `if printf '%s' "$out" | grep -q x; then`}
		got, open := sfScanLines("probe.sh", lines, true)
		if open != nil {
			t.Errorf("%q was read as opening a heredoc body: %+v", phantom, *open)
		}
		if len(got) != 1 || got[0].role != "verdict" || got[0].line != 2 {
			t.Errorf("a violation after %q is invisible to the scan: %+v", phantom, got)
		}
	}

	// …and the backstop, which is what makes the two spellings above
	// examples rather than the definition: a heredoc left open at EOF is
	// reported, so the NEXT unenumerated phantom reds instead of silently
	// swallowing the tail of a file.
	if _, open := sfScanLines("probe.sh", []string{`cat >"$t/rig.sh" <<'RIG'`, `  grep -q x "$f" || exit 1`}, true); open == nil {
		t.Errorf("an unterminated heredoc is not reported, so a misread opener still swallows the rest of a file in silence")
	}
	if _, open := sfScanLines("probe.sh", []string{`cat >"$t/rig.sh" <<'RIG'`, `  grep -q x "$f" || exit 1`, `RIG`}, true); open != nil {
		t.Errorf("a heredoc that DOES close is reported as unterminated (%+v) — every rig script in the tree would red", *open)
	}

	// 1b. A message helper EARLIER on the line does not excuse a matcher the
	// shell has already separated from it. Measured for all five helpers.
	for _, h := range []string{"bad", "fail", "note", "die", "say"} {
		l := h + ` "checking"; if printf '%s' "$out" | grep -q foo; then :; fi`
		got, _ := sfScanLines("probe.sh", []string{l}, true)
		if len(got) != 1 || got[0].role != "verdict" {
			t.Errorf("`%s …;` earlier on the line makes a real verdict fork read as %+v", h, got)
		}
	}

	// 1c. The same hole on a `\` continuation line: the carried message role
	// reaches exactly as far as the helper's own command does.
	cont := []string{`bad "$arm" \`, `  "detail"; if printf '%s' "$out" | grep -q foo; then :; fi`}
	if got, _ := sfScanLines("probe.sh", cont, true); len(got) != 1 || got[0].role != "verdict" {
		t.Errorf("a verdict fork past the end of a carried message reads as %+v", got)
	}

	// The carry must still WORK: suite-lock.sh writes most of its failure
	// detail on a continuation line, and those forks are messages. Without
	// this the fix above would turn a dozen honest lines into violations.
	carry := []string{`bad "$arm14" \`, `  "the holder never acquired: $(tr '\n' '|' <"$tmp/m18.log")"`}
	if got, _ := sfScanLines("probe.sh", carry, true); len(got) != 1 || got[0].role != "message" {
		t.Errorf("a real continued message now reads as %+v — the 1c fix broke suite-lock.sh's failure detail", got)
	}
	// And the separators that are NOT separators: nested inside `$( … )` or
	// inside the quoted message itself.
	for _, l := range []string{
		`bad "budget not read: $(printf '%s' "$out" | grep 'package times')"`,
		`bad "queued=$q; alive: $(printf '%s' "$before" | tr '\n' '|'); dead: none"`,
	} {
		if got, _ := sfScanLines("probe.sh", []string{l}, true); len(got) != 1 || got[0].role != "message" {
			t.Errorf("a nested separator ended the message role early, so an allowed fork reads as %+v: %s", got, l)
		}
	}
}
