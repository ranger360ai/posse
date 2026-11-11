package posse

// The cold-first-exec injector (ADR 0085). Shared, untagged, helper-only:
// the shim arm it installs is read by TestMain, which every suite arm
// compiles, and a file that carried tests of its own would run them three
// times.
//
// WHAT THE CLASS IS. A test plants a brand-new executable and execs it under
// a deadline of a few hundred ms. macOS assesses a freshly written
// executable on its FIRST exec and charges two orders of magnitude more for
// it than for the exec that follows — MEASURED on darwin 25.6.0 at a mean of
// 156-714ms and a worst of 2.17s against 6.3-10.9ms warm
// (docs/notes.d/ranger-base-aru4c.md). So a small deadline over a cold exec
// gives its verdict about process startup rather than about the code. The
// class has been found three times by hand.
//
// WHY A GATE AND NOT A MATCHER. ranger-base-aru4c censused the tree for it
// and 10 of its 14 candidate rows turned on a judgement no matcher made:
// does this duration bound an `execve` of the planted file at all? ADR 0085
// D1 makes the verdict the subject instead — "a test whose verdict changes
// when the first exec of a planted file is slow" — which is decidable by
// running the test with the first exec made slow. Both of aru4c's
// adjudications then fall out of execution rather than out of reading: a
// fixture driven through Herdr pays the injected cost inside a synchronous
// mux.Start, outside every deadline its caller sets, and `sh <path>` execs
// /bin/sh and never the plant at all.
//
// AND WHY IT IS WANTED NOW. Linux has no cold-exec cost — MEASURED
// 2026-11-10 on this host (7.2.15, x86_64, loadavg 7.56, n=40 fresh
// `#!/bin/sh` scripts): first exec mean 1.07ms, worst 1.29ms, re-exec mean
// 0.96ms. The crew writes tests here (ADR 0083) and darwin still runs the
// suite (ci.yml's macos-latest leg, and the laptop kept as the rollback
// home), so a new unwarmed fixture now passes where it is written and flakes
// only where nobody is watching.
//
// THE MECHANISM, end to end:
//
//   - `make coldexec-check` sets POSSE_COLD_EXEC_DELAY. TestMain reads it and
//     assigns coldExecRewrite (execwrite.go), the one seam, which is nil in
//     every shipped binary and applies only to a plant whose immediate
//     caller is a _test.go file.
//   - the rewrite turns `#!<interp>` into `#!<shim>` plus a second line,
//     `# posse-coldexec <nonce> <interp>`, and leaves the body alone. The
//     shim is this test binary.
//   - the kernel then execs `<shim> <path> <args…>`. The shim creates
//     `<nonce>` with O_EXCL. If it created it, this is the file's first exec
//     and it sleeps D; then it syscall.Execs <interp> <path> <args…>,
//     keeping its pid, so a pid file, a kill or a `$$` still names the
//     process it would have named without the injector.
//   - a fresh nonce per write makes a rewritten file cold again. The nonce
//     lives outside the script's directory, so a test that lists that
//     directory sees no extra file.
//
// THE EXIT HATCH is one nil variable and one argv arm. Delete both and
// nothing else changes; they hold no state outside a temporary marker dir.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

// coldExecDelayEnv is both the switch and the amount (ADR 0085 D3): set, the
// hook is installed; its value is how long the shim sleeps on a first exec.
// One name rather than two, so there is no state in which injection is on
// and the delay is unknown.
const coldExecDelayEnv = "POSSE_COLD_EXEC_DELAY"

// coldExecDefaultDelay is ADR 0085 D4's D, and it is above the measured
// darwin worst case of 2.17s: a deadline that survives 2.5s of cold exec
// survives darwin. ASSUMED that 2.17s tail is the true worst; if darwin ever
// measures worse, raise this. It is one constant.
//
// It is also the fallback the shim uses when the variable did not reach it —
// a test that scrubs its child's environment. That is correct by
// construction rather than a guess: a rewritten file exists only because
// injection was on, so the question is never whether to sleep but how long,
// and D is the answer the run was started with.
const coldExecDefaultDelay = 2500 * time.Millisecond

// coldExecReportEnv names a file the run's report is APPENDED to, and it
// exists because `go test` eats the report otherwise. Without -v, go test
// shows a test binary's stderr only when the package FAILS: MEASURED
// 2026-11-10 (ranger-base-kiwvn) over six arms, the install line and the
// count report are in every red arm's log and in none of the green ones. So
// D5's "the gap is a number and not a silence" held only while the door was
// red, which is the one state nobody needs a coverage number in.
//
// NO DOOR SETS IT ANY MORE, and that is not a regression. `make
// coldexec-check` is the execve supervisor now (cmd/coldexec, ADR 0085 D2 as
// amended by ranger-base-dacil), which counts in its own process — the PARENT
// of `go test` — so nothing eats its report and no file is needed. This is
// the spelling for a BY-HAND run of the content rewrite,
// `POSSE_COLD_EXEC_DELAY=... go test ./internal/posse/...`, which is the only
// way left to reach it; ranger-base-7zkxw retires it and this with it. Unset
// — every other run — changes nothing.
const coldExecReportEnv = "POSSE_COLD_EXEC_REPORT"

// coldExecMarkerPrefix opens the injected second line. It is what the shim
// arm keys on — argv[1] naming a file whose line 2 starts with this — which
// is IsCageLaunch's pattern (cagelauncher.go) applied to a file instead of a
// flag, and needs no build step because the shim is this binary.
const coldExecMarkerPrefix = "# posse-coldexec "

// coldExecInjector holds what one run's rewrites need: the shim to point at,
// the directory the nonces live in, and the two counters. One per process,
// built by coldExecInstall; the pins build their own with their own marker
// dir, so they drive the mechanism without touching the package variable a
// parallel suite is reading.
type coldExecInjector struct {
	shim      string // this test binary, from os.Executable
	markerDir string // outside every script dir, by construction
	seq       atomic.Uint64
	// passedThrough counts the plants that could not be rewritten because
	// they carry no `#!` — a copied binary (ADR 0085 D5). The gap is a
	// number the run prints rather than a silence.
	passedThrough atomic.Int64
	// rewritten counts the plants that were, so a run that injected nothing
	// at all cannot read as a run that found nothing.
	rewritten atomic.Int64
	// reportPath is where report() appends its line, taken from the
	// environment at install and then UNSET there, so no child of this
	// process inherits it. A child that does inherit it installs an injector
	// of its own and appends a line of zeroes: MEASURED 2026-11-10, one such
	// line from arm 1's run and EIGHT from arm 3's, each from a pin that
	// re-execs this binary. The owner of the file is the process the door
	// started, and that is the one the counts are about.
	reportPath string
	// hookSlots and gateShims count the plants passed through because their
	// BYTES are a protocol some production reader decides by
	// (coldExecProtocolReason). Two counters and not one, because the two
	// shapes are read by different code and a change to either should move a
	// number of its own.
	hookSlots atomic.Int64
	gateShims atomic.Int64
}

// rewrite is the hook's body. It builds bytes and nothing else: no open, no
// stat, no fork, because WriteExecutable calls it outside underForkLock and
// the span in which no fork in this process may start is not the place for
// filesystem work (execwrite.go's doc comment).
//
// It is IDEMPOTENT, and that is load-bearing in two places. A pin plants
// bytes it rewrote itself and still writes them through WriteExecutable —
// the one door every exec-bit write in this tree goes through — so under
// `make coldexec-check` those bytes arrive here a second time; and a
// production copier handed an already-rewritten file would otherwise bury
// one shim inside another. Re-planting a file's ORIGINAL content is not this
// case and gets a fresh nonce, which is what makes a rewritten file cold
// again (ADR 0085 D2).
func (c *coldExecInjector) rewrite(path string, content []byte) []byte {
	if coldExecAlreadyRewritten(content) {
		return content
	}
	switch coldExecProtocolReason(path, content) {
	case coldExecProtocolHookSlot:
		c.hookSlots.Add(1)
		return content
	case coldExecProtocolGateShim:
		c.gateShims.Add(1)
		return content
	}
	interp, body, ok := coldExecShebang(content)
	if !ok {
		c.passedThrough.Add(1)
		return content
	}
	c.rewritten.Add(1)
	// pid as well as a sequence number: the nonce is read back by a CHILD,
	// and a test that re-execs this binary gives two processes one marker
	// dir and two counters that both start at zero.
	nonce := filepath.Join(c.markerDir, fmt.Sprintf("%d-%d", os.Getpid(), c.seq.Add(1)))
	return []byte("#!" + c.shim + "\n" + coldExecMarkerPrefix + nonce + " " + interp + "\n" + body)
}

// The two reasons coldExecProtocolReason gives. They are the message a
// report prints, so they read as a reason and not as an enum.
const (
	coldExecProtocolHookSlot = "a git hook slot"
	coldExecProtocolGateShim = "a gate shim header"
)

// coldExecProtocolReason reports why these bytes must reach the disk exactly
// as the test wrote them, and "" when nothing does. A plant it names is
// passed through and counted, like ADR 0085 D5's non-scripts.
//
// WHY IT EXISTS, MEASURED (ranger-base-kiwvn, the first full run under
// injection: 28 red rows over 26 tests, against three green baseline arms;
// the second run, with the path half of this rule alone, left 4 of them and
// each named one of the two keys the first spelling could not see — a
// relative plant and a per-session hooks dir).
// Every one of those reds is one shape, and the shape is not a deadline —
// the plant's own BYTES are the subject. 23 plant a git hook and let posse's
// probe decide ours/foreign/stale/chained by comparing them to the render it
// would write; 2 plant a gate shim, which gateShimTarget identifies by
// gateShimMarker inside the FIRST TWO LINES (possebinary.go) — and the
// injected line is line 2. Rewriting either makes production answer a
// question the test did not ask, and the fatal then names a hook as foreign
// or a shim as a shadow: ADR 0085's `$0`-and-line-numbers artifact class,
// arriving as a family rather than as a tail.
//
// IT IS NOT A NEW RULE. execwrite.go's callerIsTestFile already keeps a shim
// PRODUCTION code writes byte-exact, in its own words "so the content
// read-back pins over rendered gates, hooks and probes cannot red for the
// injector's reason rather than their own". A FIXTURE that plants the same
// protocol file is the same claim about the same reader, and the writer is
// the only thing that differs.
//
// THE COST IS A BLIND SPOT AND IT IS COUNTED, exactly for D5's reason: a
// test that plants a hook or a shim and then times its first exec is not
// reached by this gate, and the run prints how many plants that was rather
// than leaving a silence.
// coldExecHookSlots are the three slots posse installs into and identifies
// by bytes (gates.go, hooksredirect.go). A plant whose basename is one of
// them, or a chain spelling of one, is a git hook whatever directory it is
// written into — which is the half of the rule that cannot be read off the
// path, because `make coldexec-check`'s own first run found a fixture that
// plants `pre-push` RELATIVE, from a cwd inside the hooks dir
// (chainrelpath_qa_test.go, whose subject is surviving its own cd).
var coldExecHookSlots = []string{"pre-commit", "pre-push", "prepare-commit-msg"}

func coldExecProtocolReason(path string, content []byte) string {
	// THE NAME first, because it is the only signal a relative plant has.
	// The `-` suffix form is the chain: posse-pre-push, theirs-pre-push,
	// bd-prepare-commit-msg are all the same slot moved aside, and posse
	// certifies each of them against a render of its own.
	base := filepath.Base(path)
	for _, slot := range coldExecHookSlots {
		if base == slot || strings.HasSuffix(base, "-"+slot) {
			return coldExecProtocolHookSlot
		}
	}
	// THEN THE DIRECTORY, for a slot whose name is not one of the three —
	// git has a score of hook names and an employer may use any of them.
	// The parent and the grandparent, and no further: the measured shapes
	// are `.git/hooks/<slot>`, `managed-hooks/<slot>` and the rendered
	// per-session dir `state/hooks/<session>/<slot>`, while an unbounded
	// walk would hand the whole gate to any tree that happened to live
	// under a directory called hooks.
	dir := filepath.Dir(path)
	for _, d := range []string{dir, filepath.Dir(dir)} {
		switch filepath.Base(d) {
		case "hooks", "managed-hooks":
			return coldExecProtocolHookSlot
		}
	}
	// The same window gateShimTarget reads (possebinary.go: `head -2`, 512
	// bytes), so this answers yes exactly when that reader would call the
	// file a shim.
	lines := strings.SplitN(string(content), "\n", 3)
	if len(lines) > 2 {
		lines = lines[:2]
	}
	if strings.Contains(strings.Join(lines, "\n"), gateShimMarker) {
		return coldExecProtocolGateShim
	}
	return ""
}

// coldExecAlreadyRewritten reports whether these bytes carry the injected
// second line already.
func coldExecAlreadyRewritten(content []byte) bool {
	lines := strings.SplitN(string(content), "\n", 3)
	return len(lines) >= 2 && strings.HasPrefix(lines[1], coldExecMarkerPrefix)
}

// coldExecShebang splits a plant into its interpreter line and the rest.
// interp is what followed `#!`, with trailing whitespace trimmed as the
// kernel trims it; body is every line after the first, newline included, so
// that interp + body is the original content minus its shebang.
//
// ok is false for content that cannot be rewritten: no `#!` at all (a copied
// binary — ADR 0085 D5's counted gap) or a `#!` with nothing after it, which
// no kernel would exec either.
func coldExecShebang(content []byte) (interp, body string, ok bool) {
	rest, found := strings.CutPrefix(string(content), "#!")
	if !found {
		return "", "", false
	}
	// strings.Cut drops the newline it split on, which is what the rewrite
	// wants: it supplies its own. A plant with no newline at all has no
	// body, and Cut answers that with "" rather than needing a branch.
	line, body, _ := strings.Cut(rest, "\n")
	interp = strings.TrimRight(line, " \t\r")
	if interp == "" {
		return "", "", false
	}
	return interp, body, true
}

// coldExecInterpArgv turns a stored interpreter line back into the argv the
// kernel would have built from it, which is NOT the same on the two
// platforms this suite runs on: Linux hands the interpreter at most one
// argument — everything after the first word arrives as a single argv entry
// — and darwin splits the line into words. Reproducing each platform's own
// rule is what keeps `#!/usr/bin/env python3 -u` arriving at the
// interpreter the way it would have without the injector.
func coldExecInterpArgv(interp string) []string {
	s := strings.TrimLeft(interp, " \t")
	fields := strings.Fields(s)
	if len(fields) <= 1 || runtime.GOOS != "linux" {
		return fields
	}
	return []string{fields[0], strings.TrimSpace(strings.TrimPrefix(s, fields[0]))}
}

// coldExecShimArgs reads the argv the kernel hands a shebang shim and
// reports whether this process is one. argv[1] is the script the kernel was
// asked to run, and its line 2 is the marker: that is the detection, and it
// is a FILE read rather than a flag because the kernel chooses the argv, not
// us.
//
// It is cheap on the path that matters. Under `go test` argv[1] is a
// `-test.*` flag, under the fake substrates it is a verb, and os.Open fails
// on both.
func coldExecShimArgs(argv []string) (script, nonce, interp string, ok bool) {
	if len(argv) < 2 {
		return "", "", "", false
	}
	f, err := os.Open(argv[1])
	if err != nil {
		return "", "", "", false
	}
	defer f.Close()
	// A bounded head, not the file: only the first two lines can carry the
	// marker and a plant may be any size.
	buf := make([]byte, 4096)
	n, _ := f.Read(buf)
	lines := strings.SplitN(string(buf[:n]), "\n", 3)
	if len(lines) < 2 {
		return "", "", "", false
	}
	rest, found := strings.CutPrefix(lines[1], coldExecMarkerPrefix)
	if !found {
		return "", "", "", false
	}
	nonce, interp, found = strings.Cut(rest, " ")
	if !found || nonce == "" || strings.TrimSpace(interp) == "" {
		return "", "", "", false
	}
	return argv[1], nonce, interp, true
}

// coldExecRunShim is the shim. It never returns: either syscall.Exec
// replaces this process with the real interpreter, or it exits.
//
// syscall.Exec and not exec.Command, so the interpreter KEEPS THIS PID
// (ADR 0085 D2). A test that writes a pid file, kills the child or echoes
// `$$` names the process it would have named with no injector in the way;
// an extra process between the two would have made the injector visible to
// every such test at once.
func coldExecRunShim(script, nonce, interp string, rest []string) {
	switch f, err := os.OpenFile(nonce, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644); {
	case err == nil:
		f.Close()
		time.Sleep(coldExecDelay())
	case errors.Is(err, fs.ErrExist):
		// A warm re-exec of the same bytes. The cost is paid; charge nothing.
	default:
		// LOUD, never silent. A marker this process cannot create is an
		// injector that has stopped injecting, and a run in that state must
		// not read as a suite that measured the class and found it clean.
		fmt.Fprintf(os.Stderr, "posse coldexec shim: %s: %v — THIS EXEC WAS NOT DELAYED\n", nonce, err)
	}
	argv := append(coldExecInterpArgv(interp), script)
	argv = append(argv, rest...)
	bin, err := exec.LookPath(argv[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "posse coldexec shim: %s: %v\n", argv[0], err)
		os.Exit(127)
	}
	err = syscall.Exec(bin, argv, os.Environ())
	fmt.Fprintf(os.Stderr, "posse coldexec shim: exec %s: %v\n", bin, err)
	os.Exit(126)
}

// coldExecDelay is D, read from the environment with
// coldExecDefaultDelay's fallback. A value that is not a duration is said
// out loud and then ignored, for the reason _suite_lock_slots gives about
// POSSE_SUITE_SLOTS: refusing would make a typo in a diagnostic variable
// into a suite that cannot run.
func coldExecDelay() time.Duration {
	v := os.Getenv(coldExecDelayEnv)
	if v == "" {
		return coldExecDefaultDelay
	}
	d, err := time.ParseDuration(v)
	if err != nil || d < 0 {
		fmt.Fprintf(os.Stderr, "posse coldexec: %s=%q is not a duration — using %s\n", coldExecDelayEnv, v, coldExecDefaultDelay)
		return coldExecDefaultDelay
	}
	return d
}

// coldExecInstall assigns the hook when POSSE_COLD_EXEC_DELAY is set, and
// returns the injector it installed or nil. TestMain calls it once, before
// m.Run, and nothing else in the tree assigns coldExecRewrite.
//
// Every failure here is fatal rather than a warning: the caller asked for an
// injected run, and a run that silently did not inject is a green suite that
// measured nothing — the one outcome this gate exists to prevent.
func coldExecInstall() *coldExecInjector {
	if os.Getenv(coldExecDelayEnv) == "" {
		return nil
	}
	shim, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "posse coldexec: os.Executable: %v\n", err)
		os.Exit(1)
	}
	dir, err := os.MkdirTemp("", "posse-coldexec-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "posse coldexec: marker dir: %v\n", err)
		os.Exit(1)
	}
	// The nonce is the first of two space-separated fields on the injected
	// line, so a marker dir with a space in it would be read back short.
	// Refused here, where the message can name it, rather than at the
	// first exec of a plant.
	if strings.ContainsAny(dir, " \t") {
		fmt.Fprintf(os.Stderr, "posse coldexec: marker dir %q contains whitespace — set TMPDIR to a path without any\n", dir)
		os.Exit(1)
	}
	inj := &coldExecInjector{shim: shim, markerDir: dir, reportPath: os.Getenv(coldExecReportEnv)}
	// Consumed, not merely read: see reportPath. The DELAY variable stays
	// set, because the shim in every child reads it to size its sleep.
	os.Unsetenv(coldExecReportEnv)
	coldExecRewrite = inj.rewrite
	fmt.Fprintf(os.Stderr, "posse coldexec: injecting %s at the first exec of every plant written from a _test.go file (ADR 0085)\n", coldExecDelay())
	return inj
}

// report prints what the run did and takes the marker dir away. Nil-safe:
// an ordinary suite installs no injector and this is a no-op.
//
// The pass-through count is ADR 0085 D5. Content with no `#!` cannot be
// rewritten, so those plants keep their cold first exec and the gate says
// nothing about them — and the number is printed so that the gap is a
// number somebody can act on rather than a silence.
func (c *coldExecInjector) report() {
	if c == nil {
		return
	}
	// How many of the rewrites the gate actually REACHED. A marker exists if
	// and only if a shim took its first-exec branch, so counting them is the
	// one honest answer to "how much of this run was measured": a plant that
	// is written and never exec'd was rewritten for nothing, and under this
	// package's suite most of them are.
	execd := -1
	if ents, err := os.ReadDir(c.markerDir); err == nil {
		execd = len(ents)
	}
	line := fmt.Sprintf("posse coldexec: %d plants rewritten, %d of them exec'd at least once; passed through unchanged: %d with no #! (ADR 0085 D5), %d %s, %d %s (ranger-base-kiwvn)\n",
		c.rewritten.Load(), execd, c.passedThrough.Load(),
		c.hookSlots.Load(), coldExecProtocolHookSlot,
		c.gateShims.Load(), coldExecProtocolGateShim)
	fmt.Fprint(os.Stderr, line)
	// APPEND, and never truncate: the door points all three arms at one file
	// and prints it once, so three lines are three arms and not the last one
	// standing. A failure to write is said out loud rather than swallowed —
	// an unreadable report is the silence this variable exists to end.
	if path := c.reportPath; path != "" {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err == nil {
			_, err = f.WriteString(line)
			f.Close()
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "posse coldexec: %s=%q: %v\n", coldExecReportEnv, path, err)
		}
	}
	os.RemoveAll(c.markerDir)
}
