//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-wgzu7 finding 1: the notices that go through NO
// backend reach the launcher's stream too, never the process's stderr.
//
// THE DEFECT, and it is the third slice of one. ranger-base-lcode routed one
// LAUNCH's own lines (NewSessionOpts.Warn) and ranger-base-ws20a the
// backend's non-launch ones (HerdrBackend.Warn). Both act on a writer the
// backend resolves, and six writers in this package are not that: a blown
// git/bd/herdr deadline, LoadRuntime's dropped-key notice, the two
// credential-perm drift notices and BeadsDirs' cwd fallback. Package state
// and per-value fields on types built fresh at a dozen call sites — so
// neither fix reached them and every one still defaulted to a stderr the one
// process the fleet's passes happen in has on /dev/null (RE-MEASURED
// 2026-10-03, pid 22611, `lsof -p 22611 -a -d 0,1,2`: fd 0, 1 AND 2). The
// loop's record is a file the loop opens and tees Out and Err into
// (watch.go), so a line through neither is not in the record at all.
//
// THE FIX is one function the owning PROCESS calls once
// (processnotices.go), so the arms below name sites in five different files:
// a fix made at one call site passes the first and fails the rest. Two arms
// read source instead, for the reason ranger-base-ws20a's fourth arm gives —
// the writers have existed all along and the defect was that nothing
// assigned them, so a behavioural pin over a dispatcher a TEST routed cannot
// see the regression. Only the command can.
//
// Not a gate: nothing here changes what any of these notices SAY or what the
// call that wrote one goes on to do. Each arm asserts that too.
//
// SERIAL, every behavioural arm, and that is the price of process state: the
// routing is package-wide, so an arm that ran in parallel would steal the
// notices of every other test in this package (the same reason
// TestQAThePaneModeYamlKeyIsRetiredInertAndNamed and
// TestOverlayWarnsUnknownKeys are serial, and on three of the same vars).

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// pnSave puts all four writers back on cleanup. Every arm that routes calls
// it FIRST — the restore is what makes these arms safe to run in a package
// this size, and an arm that routed without it would leave every later test
// in the binary writing into a sink of its own that nobody reads (which is
// how TestQAUnroutedProcessNoticesStayOnTheOperatorsTerminal first failed).
func pnSave(t *testing.T) {
	t.Helper()
	gw, rw, nw, pw := gitHangw, runtimeNoticeWriter, noticeWriter, processNoticew
	t.Cleanup(func() {
		gitHangw, runtimeNoticeWriter, noticeWriter, processNoticew = gw, rw, nw, pw
	})
}

// Site 1, the one the bead calls the pin shape: a blown git deadline. Driven
// through gitHang rather than through a git that never answers, because the
// claim here is about the DESTINATION and githang_qa_test.go already owns
// the hang itself (arm 3, with a real child to reap). What this adds is that
// the line it writes is in the pass's record.
func TestQABlownGitDeadlineLandsInThePassesStreamNotOnStderr(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	errs := dispatcherErr(t, d)
	pnSave(t)
	d.RouteProcessNotices()

	hang := gitHang(t.TempDir(), []string{"merge", "--ff-only", "posse/s-1"}, time.Minute, time.Minute)

	if !strings.Contains(errs.String(), "git child hung") {
		t.Errorf("a blown git deadline is not in the pass's own stream, which is all the watch log tees:\n%s", errs.String())
	}
	// Not a gate: the finding the caller acts on is unchanged, and the half
	// of it that is about the repository is the half a discarded line cost
	// most (githang.go's GitHangError doc).
	if hang == nil || !hang.Mutation {
		t.Fatalf("the routing changed what the hang REPORTS: %+v", hang)
	}
	if !strings.Contains(errs.String(), "MAY HAVE TAKEN EFFECT") {
		t.Errorf("the line in the pass's stream dropped the sentence about the repository:\n%s", errs.String())
	}
}

// Site 2, a different file: LoadRuntime's dropped-key notice. Armed before
// anything loads the file, because the notice is said once per (path, key
// set) and a listing that got there first would leave this arm measuring a
// dedupe.
func TestQARuntimeDroppedKeyNoticeLandsInThePassesStream(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	errs := dispatcherErr(t, d)
	pnSave(t)
	d.RouteProcessNotices()

	rt := yamlRuntime(t, b, "mycli", "no_such_key: 1\n")
	if _, err := b.App.LoadRuntime(rt); err != nil {
		t.Fatalf("the runtime must still LOAD — a stray key is a notice and not a refusal: %v", err)
	}

	if !strings.Contains(errs.String(), "no_such_key:") {
		t.Errorf("the dropped-key notice is not in the pass's own stream:\n%s", errs.String())
	}
}

// Site 3, a third file: BeadsDirs' cwd fallback, whose own comment is "the
// operator can see which repo served it" — and under the loop nobody could.
func TestQACwdFallbackNoticeLandsInThePassesStream(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	errs := dispatcherErr(t, d)
	pnSave(t)
	d.RouteProcessNotices()

	// A config with no `beads:` key at all, which is the silent case: a
	// present-but-empty list names no repos and says nothing.
	write(t, b.App.ConfigPath, "agents_dir: "+b.App.AgentsDir+"\n")
	dirs := b.App.BeadsDirs()

	if !strings.Contains(errs.String(), "using the process cwd") {
		t.Errorf("the cwd fallback is not in the pass's own stream:\n%s", errs.String())
	}
	// Not a gate: the queue is still served out of the cwd, which is the
	// behaviour ranger-base-5b5 settled and kept.
	if len(dirs) != 1 || dirs[0] != "" {
		t.Errorf("the routing changed which repos are served: %q", dirs)
	}
}

// Site 4, the one the bead's census could not see: a writer that takes the
// stream OUTRIGHT at the call site rather than through a var. EnvSetVars is
// called from planLaunch itself — the function whose head comment
// ranger-base-lcode left reading "never to b.warn or to os.Stderr".
func TestQAEnvPermDriftNoticeLandsInThePassesStream(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	errs := dispatcherErr(t, d)
	pnSave(t)
	d.RouteProcessNotices()

	if err := os.MkdirAll(b.App.EnvsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	set := filepath.Join(b.App.EnvsDir, "harness.env")
	write(t, set, "TOKEN=x\n")
	if err := os.Chmod(set, 0o644); err != nil { // the drift `posse init` cannot prevent
		t.Fatal(err)
	}

	vars, err := b.App.EnvSetVars("harness")
	if err != nil {
		t.Fatalf("the env set must still be READ: %v", err)
	}

	if !strings.Contains(errs.String(), "tightened to 0600") {
		t.Errorf("a plaintext credential store found world-readable said so to nobody:\n%s", errs.String())
	}
	// Not a gate, twice over: the values still arrive, and the mode is
	// still fixed. The notice is the only thing this bead moved.
	if len(vars) != 1 || vars[0].Key != "TOKEN" {
		t.Errorf("the routing changed what the env set yields: %+v", vars)
	}
	st, err := os.Stat(set)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Errorf("the drift was reported and not fixed: %04o", st.Mode().Perm())
	}
}

// Sites 5 and 6 are RESOLVERS and are pinned as such: Bd and Herdr are value
// types built fresh at a dozen call sites, so nobody owns their Hangw field
// and the nil arm is the whole of production. Driving a real hang needs a
// child that never answers and belongs where that fixture lives
// (watchhang_qa_test.go); what is checked here is that the nil arm resolves
// to the routed writer and that a field somebody DID set still wins.
func TestQABdAndHerdrHangWritersResolveThroughTheRoutedStream(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	errs := dispatcherErr(t, d)
	pnSave(t)
	d.RouteProcessNotices()

	for _, c := range []struct {
		name string
		got  func() io.Writer
		set  func(io.Writer) io.Writer
	}{
		{"bd", func() io.Writer { return Bd{}.hangw() }, func(w io.Writer) io.Writer { return Bd{Hangw: w}.hangw() }},
		{"herdr", func() io.Writer { return Herdr{}.hangw() }, func(w io.Writer) io.Writer { return Herdr{Hangw: w}.hangw() }},
	} {
		if _, ok := c.got().(dispatcherErrw); !ok {
			t.Errorf("an unassigned %s.Hangw resolves to %T, not the dispatcher's serialized writer — a blown deadline there is a line the loop's record never gets", c.name, c.got())
		}
		var mine strings.Builder
		if got := c.set(&mine); got != io.Writer(&mine) {
			t.Errorf("%s.Hangw set by its caller was overridden by the routing (%T) — the field is the per-call seam the pins use", c.name, got)
		}
	}
	if errs.String() != "" {
		t.Errorf("resolving a writer wrote something: %s", errs.String())
	}
}

// QUIET, and the reason the routed writer is quietErrWriter and not
// errWriter: a blown herdr deadline is written on whatever goroutine blew
// it, and the pulse clock calls Sessions() every tick (pulse.go). Stamping
// LastWrite off a clock leaves the watchdog's silence input refreshed by
// something that says nothing about whether the pass is alive —
// ranger-base-0fz98 finding 3, which is why quietf exists.
func TestQAProcessNoticesAreNotTheLoopsSignOfLife(t *testing.T) {
	b, _ := newTestBackend(t)
	d := newTestDispatcher(t, b)
	errs := dispatcherErr(t, d)
	pnSave(t)
	d.RouteProcessNotices()

	if !d.LastWrite().IsZero() {
		t.Fatalf("premise: a dispatcher that has written nothing has no LastWrite, got %s", d.LastWrite())
	}
	gitHang(t.TempDir(), []string{"rev-list", "--count", "main..posse/s-1"}, time.Minute, time.Minute)

	if errs.String() == "" {
		t.Fatal("the line did not reach the pass's stream, so this measures nothing")
	}
	if got := d.LastWrite(); !got.IsZero() {
		t.Errorf("a process notice stamped LastWrite (%s): a clock writes this stream, and a clock is not the loop writing (ranger-base-0fz98)", got)
	}
}

// The seam both ways. A process that routed nothing keeps the operator's
// terminal, which is what `posse list`, `posse new` and `posse kill` want —
// the field's nil IS the right answer for every surface but the two that
// take the whole screen or nobody's attention. And a nil w is a no-op
// rather than a reset, because every caller hands over a writer it built.
func TestQAUnroutedProcessNoticesStayOnTheOperatorsTerminal(t *testing.T) {
	for _, c := range []struct {
		name string
		got  io.Writer
	}{
		{"processNotices()", processNotices()},
		{"gitHangw", gitHangw},
		{"runtimeNoticeWriter", runtimeNoticeWriter},
		{"noticeWriter", noticeWriter},
		{"Bd{}.hangw()", Bd{}.hangw()},
		{"Herdr{}.hangw()", Herdr{}.hangw()},
	} {
		if c.got != io.Writer(os.Stderr) {
			t.Errorf("%s answers %v in a process that routed nothing, not the operator's terminal", c.name, c.got)
		}
	}

	var mine strings.Builder
	pnSave(t)
	RouteProcessNotices(&mine)
	RouteProcessNotices(nil)
	if processNotices() != io.Writer(&mine) {
		t.Errorf("a nil writer reset the routing to %T — a nil there is a wiring bug, not a request for the default", processNotices())
	}
}

// The wiring, which is the thing that was actually missing — the writers
// have existed all along. Read as source for ranger-base-ws20a arm 4's
// reason: the only observable of a correctly wired command is a line in a
// log that a pass with nothing to warn about never writes.
func TestQADispatchAndCockpitRouteTheProcessNotices(t *testing.T) {
	t.Parallel()
	main, err := os.ReadFile(filepath.Join("..", "..", "cmd", "posse", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	// The `dispatch` case clause, up to the next one: a call anywhere else
	// in main.go is a different command's wiring.
	body := string(main)
	at := strings.Index(body, "\tcase \"dispatch\":\n")
	if at < 0 {
		t.Fatal("no `case \"dispatch\":` in cmd/posse/main.go — this pin is reading the wrong file")
	}
	body = body[at:]
	if end := strings.Index(body[1:], "\n\tcase "); end >= 0 {
		body = body[:end+1]
	}
	if !strings.Contains(body, "d.RouteProcessNotices()") {
		t.Errorf("`posse dispatch` does not route the package's unowned notices, so every line the arms above pin goes to a stderr that loop has on /dev/null (ranger-base-wgzu7):\n%s", body)
	}

	ck, err := os.ReadFile(filepath.Join("..", "..", "cmd", "posse", "cockpit.go"))
	if err != nil {
		t.Fatal(err)
	}
	// The cockpit's half, and it must be the OTHER call: these writers are
	// process-wide, and d.RouteProcessNotices() there would route them
	// through c.disp, whose Out is io.Discard.
	if !strings.Contains(string(ck), "posse.RouteProcessNotices(c.notices)") {
		t.Error("the cockpit does not route the package's unowned notices into its own sink: with the alt screen up a line on stderr lands ON the frame being drawn and is gone with the next redraw (ranger-base-wgzu7, ranger-base-2vhqo)")
	}
	// Code only: cockpit.go's own comment NAMES the call it must not make,
	// which is the half of the decision a reader needs.
	for i, ln := range strings.Split(string(ck), "\n") {
		if strings.HasPrefix(strings.TrimSpace(ln), "//") {
			continue
		}
		if strings.Contains(ln, "d.RouteProcessNotices()") || strings.Contains(ln, "disp.RouteProcessNotices()") {
			t.Errorf("cockpit.go:%d takes the DISPATCHER's routing for these notices: its dispatcher writes io.Discard, so that is the /dev/null again with extra steps:\n\t%s", i+1, strings.TrimSpace(ln))
		}
	}
	// And not from newCockpit, which every cmd/posse cockpit test calls:
	// routing from a constructor would leave the whole package writing into
	// the sink of a cockpit that no longer exists.
	nc := string(ck)
	if at := strings.Index(nc, "func newCockpit("); at >= 0 {
		ctor := nc[at:]
		if end := strings.Index(ctor, "\n}\n"); end >= 0 {
			ctor = ctor[:end]
		}
		if strings.Contains(ctor, "RouteProcessNotices") {
			t.Error("newCockpit routes process-wide writers — a constructor a test calls would point this package at a dead sink for every test after it")
		}
	}
}

// A cheap guard on the premise every arm above rests on: these are the ONLY
// places in this package that reach the process's stderr, so routing them
// reaches every site rather than the six these arms happen to drive. A new
// site that writes it by hand is this defect returning under a green suite —
// a line nobody reads, with no writer to point anywhere else.
//
// The exemption is by file and SUBSTRING and must MATCH something, like the
// ones in watchhang_qa_test.go and backendwarnstream_qa_test.go: an
// exemption that outlives the code it excused fails this test rather than
// quietly widening it.
func TestQAProcessNoticesResolveThroughTheDeclaredSeams(t *testing.T) {
	t.Parallel()
	// Each entry: the file, the substring that identifies the site, and why
	// that site may name the process's stderr.
	allowed := []struct{ file, site, why string }{
		{"processnotices.go", "var processNoticew io.Writer = os.Stderr", "THE resolver for the sites with no seam of their own"},
		{"processnotices.go", "return os.Stderr", "processNotices()' nil arm — the same contract HerdrBackend.Warn has"},
		{"githang.go", "var gitHangw io.Writer = os.Stderr", "a routed seam; its own pin swaps it (githang_qa_test.go)"},
		{"runtimeyaml.go", "var runtimeNoticeWriter io.Writer = os.Stderr", "a routed seam; its own pins swap it"},
		{"beads.go", "var noticeWriter io.Writer = os.Stderr", "a routed seam; the silence half is not observable otherwise"},
		{"herdrback.go", "return os.Stderr", "warnWriter's fallback — the backend's resolver (ranger-base-ws20a)"},
		{"worktree.go", "return os.Stderr", "warnw's fallback, the same contract one layer down"},
		{"dispatch.go", "return os.Stderr", "errw's fallback — the writer every other site here routes TO"},
		{"app.go", "return newApp(os.Stderr)", "the App's own writer default, set by the process that builds it"},
		{"refresh.go", "cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr", "handing a child the process's own streams, not writing a line"},
		{"cagelauncher.go", "StartCageEgress(plan, argv[2], os.Stderr)", "`posse cage-exec`, the inner process: its stderr IS the pane"},
	}
	ents, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	hit := make(map[string]bool)
	var offenders []string
	files := 0
	for _, e := range ents {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		files++
		for i, line := range strings.Split(string(src), "\n") {
			// Code only, and a TRAILING comment is a comment too:
			// dispatch.go's `Err io.Writer // ... (nil = os.Stderr)` is a
			// field doc, not a writer.
			if at := strings.Index(line, "//"); at >= 0 {
				line = line[:at]
			}
			if !strings.Contains(line, "os.Stderr") {
				continue
			}
			ok := false
			for _, a := range allowed {
				if a.file == name && strings.Contains(line, a.site) {
					hit[a.file+"\x00"+a.site], ok = true, true
				}
			}
			if !ok {
				offenders = append(offenders, fmt.Sprintf("%s:%d: %s", name, i+1, strings.TrimSpace(line)))
			}
		}
	}
	// A floor, not a count: a sweep that read nothing (wrong cwd) must fail
	// loudly instead of passing over an empty set.
	if files < 30 {
		t.Fatalf("swept %d non-test files in internal/posse; the sweep is not reading the package it thinks it is", files)
	}
	if len(offenders) != 0 {
		t.Errorf("these write the process's stderr outside the declared seams, which a watch loop has on /dev/null and a cockpit draws over (ranger-base-wgzu7) — route it through processNotices(), or add it above with its reason:\n%s",
			strings.Join(offenders, "\n"))
	}
	for _, a := range allowed {
		if !hit[a.file+"\x00"+a.site] {
			t.Errorf("exemption %s %q (%s) matched nothing — the site is gone, so the exemption must go too", a.file, a.site, a.why)
		}
	}
	t.Logf("swept %d non-test files, %d declared seams, %d flagged", files, len(allowed), len(offenders))
}
