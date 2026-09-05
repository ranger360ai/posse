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

var (
	sfRegionStart = regexp.MustCompile(`^[a-zA-Z_]*[sS]elf_?[tT]est[a-zA-Z_]*\(\)\s*\{`)
	sfMatcher     = regexp.MustCompile(`(^|[|&;(]|\$\(|` + "`" + `|\b(?:if|elif|while|until)\s+|!\s+)\s*(grep|sed|awk|head|tail|cut|tr|sort|uniq|wc|cat|jq|paste|xargs)\b`)
	sfHeredoc     = regexp.MustCompile(`<<-?[ \t]*(['"]?)([A-Za-z_][A-Za-z0-9_]*)['"]?`)
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
	role       string // "verdict" (a violation), "message", "fixture"
}

// sfScanLines classifies every matcher occurrence on an assertion path.
// Exported as a function over lines so the controls below can drive it with
// synthetic input and prove it separates the three roles.
func sfScanLines(file string, lines []string, wholeFile bool) []sfHit {
	var hits []sfHit
	in := wholeFile
	heredoc := ""
	inMessage := false
	for i, raw := range lines {
		// A heredoc body is data the script writes, not its assertion path.
		if heredoc != "" {
			if strings.TrimSpace(raw) == heredoc {
				heredoc = ""
			}
			continue
		}
		l := strings.TrimLeft(raw, " \t")
		// `<<<` is a here-STRING and opens no body; blind the heredoc match
		// to it rather than trying to express "not <<<" in a Go regexp.
		probe := strings.ReplaceAll(raw, "<<<", "\x00\x00\x00")
		if m := sfHeredoc.FindStringSubmatch(probe); m != nil {
			heredoc = m[2]
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
		if strings.HasPrefix(l, "#") {
			continue
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
		case carried || (startsMessage != nil && startsMessage[0] < loc[1]):
			role = "message"
		case tool == "cat" && (strings.Contains(l[loc[5]:], ">") || strings.Contains(l, "<<")):
			role = "fixture"
		}
		hits = append(hits, sfHit{file: file, line: i + 1, text: l, role: role})
	}
	return hits
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
		for _, h := range sfScanLines(base, strings.Split(string(b), "\n"), whole) {
			all = append(all, h)
		}
	}
	return all
}

func TestQANoAssertionArmDecidesThroughAForkedMatcher(t *testing.T) {
	hits := sfScanScripts(t)

	// THE FLOOR, and it is the positive witness this whole test rests on: a
	// regexp that stopped matching, or a region finder that stopped finding
	// self-test bodies, would leave `hits` empty and this test green over a
	// scan that reads nothing at all. There were 126 occurrences across 19
	// files when this was measured (2026-09-05).
	if len(hits) < 80 {
		t.Fatalf("the scan found only %d matcher occurrences across scripts/ — it was 126 when written, so the shape or the region finder has gone blind and a clean result here means nothing", len(hits))
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
		got := sfScanLines("probe.sh", []string{c.line}, true)
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
		if got := sfScanLines("probe.sh", []string{l}, true); len(got) != 0 {
			t.Errorf("the scan flags a FIXED line as %q — the invariant it demands cannot be satisfied: %s", got[0].role, l)
		}
	}

	// A heredoc body is skipped, and the here-STRING that looks like one is
	// not: `<<<` opens no body, so blinding on it would swallow the rest of
	// the file and turn every later violation invisible.
	body := []string{`cat >"$t/rig.sh" <<'RIG'`, `  grep -q x "$f" || exit 1`, `RIG`, `if grep -q 'y' "$out"; then`}
	got := sfScanLines("probe.sh", body, true)
	if len(got) != 2 || got[0].role != "fixture" || got[1].role != "verdict" || got[1].line != 4 {
		t.Errorf("heredoc handling is wrong: %+v — want the opener as fixture and line 4 as a verdict, with the body skipped", got)
	}
	hs := []string{`suspects=$(signal_suspects 505 <<<"$table")`, `if grep -q 'y' "$out"; then`}
	if got := sfScanLines("probe.sh", hs, true); len(got) != 1 || got[0].line != 2 {
		t.Errorf("a here-string was read as opening a heredoc body, so everything after it goes unscanned: %+v", got)
	}
}
