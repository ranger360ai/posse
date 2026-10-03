//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-ws20a: the backend's OWN diagnostic lines — the
// ones no single launch owns — go to the launcher's stream, never to the
// process's stderr.
//
// THE DEFECT, and it is the half ranger-base-lcode left behind. That bead
// gave ONE launch its own writer (NewSessionOpts.Warn) and pinned the launch
// lines in launchwarnstream_qa_test.go. Every b.warn site OUTSIDE
// planLaunch/createSession stayed on HerdrBackend.Warn, which was assigned
// NOWHERE in non-test code — so it was always os.Stderr, and the one process
// the fleet's passes happen in has fd 0, 1 AND 2 on /dev/null (MEASURED
// 2026-10-03, pid 52273, lsof; docs/notes.d/ranger-base-knux2.md). The
// loop's record is a file the loop opens and tees Out and Err into
// (watchlog.go), so a line through neither is not in the record at all.
//
// The lines, all of them written on a pass's own goroutines:
//
//   - listSessions' five "session meta file(s) kept, not listed"
//     abstentions, two of which carry a repair recipe;
//   - "fold refusals spool for <name>", at killAndLand, RelaunchAgent and
//     closeRecorded;
//   - noteUnlandedOnKill's "<bead> left work unlanded in <tree> … and bd
//     could not say whether it is closed", the loudest: a kill that could
//     not note unlanded work ON the bead was saying so to nobody either.
//
// THE FIX is at the writer and not at a call site:
// Dispatcher.RouteBackendWarnings() sets HerdrBackend.Warn to the
// dispatcher's quiet err writer, and `posse dispatch` calls it once (arm 4
// is the pin that it does). So the arms below name two sites in two
// different functions — a listing and a kill — because a fix made at one
// call site passes the first and fails the second.
//
// Not a gate, like lcode: nothing here changes what a listing withholds or
// what a kill lands. Each arm asserts the pass still did its work.

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bwsRecipe plants the meta a relaunch leaves behind when its recreate fails
// (rangerhq-v52t): a session record naming NO workspace. listSessions reports
// it as a recipe and says so through b.warn, which is the cheapest of the
// five abstention lines to reach — it needs no herdr board at all.
func bwsRecipe(t *testing.T, b *HerdrBackend, name string) {
	t.Helper()
	if err := b.writeMeta(&HerdrMeta{Name: name, Agent: "ranger", Dir: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
}

// Site 1, through a real pass: the sweep reads the board (autoreap.go:108,
// d.HB.Sessions()), the listing withholds a meta and the reason lands in the
// stream the loop tees its record from — not on the stderr that loop has on
// /dev/null.
func TestQABackendListingAbstentionLandsInThePassesStreamNotOnStderr(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	errs := dispatcherErr(t, d)
	writePersona(t, b.App, "ranger", "[go]")
	idleClaude(t, fake)
	bwsRecipe(t, b, "ranger-repo-a-1")

	// b.Warn is where the backend writes when nobody has handed it a
	// writer, and in production that is os.Stderr — the /dev/null the
	// incident was. Captured BEFORE the routing below replaces it: a line
	// in here is a line the loop's record would not have.
	stderrish := warnBuf(t, b)
	d.RouteBackendWarnings()

	d.autoReapPass(afterRouting)

	if !strings.Contains(errs.String(), "recipe kept: ranger-repo-a-1") {
		t.Errorf("the listing's abstention is not in the pass's own stream, which is all the watch log tees:\n%s", errs.String())
	}
	if !strings.Contains(errs.String(), b.metaDir()) {
		t.Errorf("the line in the pass's stream does not name the dir to repair in (%s):\n%s", b.metaDir(), errs.String())
	}
	if got := stderrish.String(); strings.Contains(got, "recipe kept") {
		t.Errorf("the abstention went to the backend's own writer — os.Stderr in production, /dev/null under the loop:\n%s", got)
	}
	// Not a gate: the meta is still KEPT, which is the whole of what that
	// line is reporting.
	if _, ok := b.readMeta("ranger-repo-a-1"); !ok {
		t.Error("the recipe was deleted — the routing must change where the line goes and nothing else")
	}
}

// Site 2, a different function: killAndLand's fold refusal. A spool path
// that is a DIRECTORY is the cheapest way to make FoldRefusalsSpool fail —
// os.ReadFile answers EISDIR, which is not os.IsNotExist, so the fold
// reports rather than no-opping.
//
// Reached by killing directly rather than through the sweep, deliberately:
// autoreap.go:256 prints its own twin of this exact line through d.eprintf,
// so a sweep-driven arm would read green over a backend still writing
// stderr. This is the kill the sweep makes, with the sweep's own writer out
// of the way.
func TestQABackendKillRefusalLandsInThePassesStream(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	errs := dispatcherErr(t, d)
	writePersona(t, b.App, "ranger", "[go]")
	reapCandidate(t, b, "ranger-repo-a-1", "a-1", "closed")
	idleClaude(t, fake)
	if err := os.MkdirAll(b.App.CageSpoolPath("ranger", "ranger-repo-a-1"), 0o755); err != nil {
		t.Fatal(err)
	}

	stderrish := warnBuf(t, b)
	d.RouteBackendWarnings()

	if _, err := b.KillSessionAndLand("ranger-repo-a-1"); err != nil {
		t.Fatalf("the kill itself must still succeed — a fold failure is not a reason to fail it: %v", err)
	}

	if !strings.Contains(errs.String(), "fold refusals spool for ranger-repo-a-1") {
		t.Errorf("the kill's fold refusal is not in the pass's own stream:\n%s", errs.String())
	}
	if got := stderrish.String(); strings.Contains(got, "fold refusals spool") {
		t.Errorf("the kill's fold refusal went to the backend's own writer, which is /dev/null under the loop:\n%s", got)
	}
	// Not a gate: the kill still happened.
	if _, ok := b.readMeta("ranger-repo-a-1"); ok {
		t.Error("the session survived the kill — the routing must change where the line goes and nothing else")
	}
}

// The seam both ways. A dispatcher that has not routed the backend leaves it
// exactly where it was, which is what keeps `posse list`, `posse new` and
// `posse kill` printing to the operator's terminal — and what keeps the
// cockpit's own backend lines off its one-field status line
// (ranger-base-2vhqo).
func TestQABackendWarningsFollowTheWriterTheLauncherHandsDown(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	errs := dispatcherErr(t, d)
	writePersona(t, b.App, "ranger", "[go]")
	idleClaude(t, fake)
	bwsRecipe(t, b, "ranger-repo-a-1")
	stderrish := warnBuf(t, b)

	// No RouteBackendWarnings(): the control, and the production shape for
	// every other subcommand in the same binary.
	d.autoReapPass(afterRouting)

	if !strings.Contains(stderrish.String(), "recipe kept: ranger-repo-a-1") {
		t.Errorf("a dispatcher that routed nothing must leave the backend's own writer answering:\n%s", stderrish.String())
	}
	if strings.Contains(errs.String(), "recipe kept") {
		t.Errorf("the backend wrote the pass's stream without being handed it:\n%s", errs.String())
	}
}

// QUIET, and this is the reason the routed writer is quietErrWriter and not
// errWriter. The pulse clock calls d.HB.Sessions() every tick (pulse.go
// deliverPulse) on a goroutine of its own, so a box holding one withheld
// meta writes one of these lines per tick from a CLOCK. Stamping LastWrite
// off that would leave the watchdog's silence input refreshed forever by
// something that says nothing about whether the pass is alive —
// ranger-base-0fz98 finding 3, which is exactly why quietf exists.
func TestQABackendWarningsAreNotTheLoopsSignOfLife(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	errs := dispatcherErr(t, d)
	d.RouteBackendWarnings()

	if !d.LastWrite().IsZero() {
		t.Fatalf("premise: a dispatcher that has written nothing has no LastWrite, got %s", d.LastWrite())
	}
	b.warn("posse: 1 session meta file(s) kept, not listed\n")

	if errs.String() == "" {
		t.Fatal("the line did not reach the pass's stream, so this measures nothing")
	}
	if got := d.LastWrite(); !got.IsZero() {
		t.Errorf("a backend warning stamped LastWrite (%s): the pulse clock writes this stream, and a clock is not the loop writing (ranger-base-0fz98)", got)
	}
	// Serialized all the same — a gather writes this stream from a goroutine
	// of its own (ADR 0028 §1), which is the half quietErrWriter keeps.
	if _, ok := b.warnWriter().(dispatcherErrw); !ok {
		t.Errorf("the routed writer is %T, not the dispatcher's serialized one", b.warnWriter())
	}
}

// Arm 4: the wiring, which is the thing that was actually missing. The field
// has existed since before lcode and the defect was that nothing assigned
// it, so a behavioural pin over a dispatcher a TEST routed cannot see the
// regression — only the command can.
//
// Read as source rather than driven through the built binary: the only
// observable of a correctly wired `posse dispatch` is a line in a log that a
// pass with nothing to warn about never writes, so a behavioural pin here
// would need a fixture of the fleet. What is checked is narrow and exact:
// the `dispatch` case of main.go's switch calls it on the dispatcher it just
// built.
func TestQADispatchCommandRoutesTheBackendsWarnings(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile(filepath.Join("..", "..", "cmd", "posse", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	// The case clause, up to the next one: a call anywhere else in main.go
	// is a different command's wiring and does not answer this claim.
	body := string(src)
	at := strings.Index(body, "\tcase \"dispatch\":\n")
	if at < 0 {
		t.Fatal("no `case \"dispatch\":` in cmd/posse/main.go — this pin is reading the wrong file")
	}
	body = body[at:]
	if end := strings.Index(body[1:], "\n\tcase "); end >= 0 {
		body = body[:end+1]
	}
	if !strings.Contains(body, "d.RouteBackendWarnings()") {
		t.Errorf("`posse dispatch` does not route the backend's own warnings, so every line the arms above pin goes to a stderr the watch loop has on /dev/null (ranger-base-ws20a):\n%s", body)
	}
	// And NOT in the cockpit, which is the one other surface holding this
	// same backend (cmd/posse/cockpit.go: c.hb beside a c.disp of its own).
	// Routing it there would put a listing's two-line repair recipe and a
	// kill's refusals through Progress, a one-field status line — a surface
	// decision and ranger-base-2vhqo's to make, not a side effect of this
	// plumbing. Only the cockpit is named: another command that grows a pass
	// of its own SHOULD route it, exactly as `dispatch` does.
	ck, err := os.ReadFile(filepath.Join("..", "..", "cmd", "posse", "cockpit.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(ck), "RouteBackendWarnings()") {
		t.Error("the cockpit routes the shared backend's warnings onto its own stream; where a TUI shows a five-line listing abstention is ranger-base-2vhqo's decision")
	}
	if n := strings.Count(string(src), "RouteBackendWarnings()"); n != 1 {
		t.Errorf("main.go calls RouteBackendWarnings %d times, want exactly 1 (the `dispatch` case)", n)
	}
}

// A cheap guard on the premise the arms above rest on: HerdrBackend.Warn is
// still the ONE place the backend's non-launch diagnostics resolve through,
// so routing it reaches every site the bead named rather than the two these
// arms happen to drive. A new site that writes the process's stderr by hand
// is this defect returning under a green suite — a line nobody reads, with
// no field to point anywhere else.
//
// The exemption is by SUBSTRING and must MATCH something, like the one in
// watchhang_qa_test.go: an exemption that outlives the code it excused fails
// this test rather than quietly widening it.
func TestQABackendDiagnosticsResolveThroughOneWriter(t *testing.T) {
	t.Parallel()
	// Each entry: the substring that identifies the site, and why it may
	// name the stream directly.
	allowed := []struct{ site, why string }{
		{"return os.Stderr", "warnWriter's fallback — THE resolver, and warnWriterFor answers through it"},
	}
	src, err := os.ReadFile("herdrback.go")
	if err != nil {
		t.Fatal(err)
	}
	hit := make(map[string]bool)
	var offenders []string
	for _, line := range strings.Split(string(src), "\n") {
		if !strings.Contains(line, "os.Stderr") || strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		ok := false
		for _, a := range allowed {
			if strings.Contains(line, a.site) {
				hit[a.site], ok = true, true
			}
		}
		if !ok {
			offenders = append(offenders, strings.TrimSpace(line))
		}
	}
	if len(offenders) != 0 {
		t.Errorf("herdrback.go writes the process's stderr outside the warn resolver, which a watch loop has on /dev/null (ranger-base-ws20a):\n%s",
			strings.Join(offenders, "\n"))
	}
	for _, a := range allowed {
		if !hit[a.site] {
			t.Errorf("exemption %q (%s) matches nothing — the resolver it excused is gone, and with it the one writer this bead's routing acts on", a.site, a.why)
		}
	}
	// And the resolver is reachable: a backend nobody routed answers the
	// process's stderr, which is what keeps every interactive surface
	// printing where the operator is looking.
	b, _ := newTestBackend(t)
	b.Warn = nil
	if w := b.warnWriter(); w != io.Writer(os.Stderr) {
		t.Errorf("an unrouted backend writes %v, not the operator's terminal", w)
	}
}
