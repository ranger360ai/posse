package posse

// ADR 0013 §2 — the promptable stage, dispatch half (ranger-base-dg5).
//
// What these pin is an ORDER and a SPLIT, not a rendering. The order is
// claim → write → create → await, and every one of the four steps has a
// failure whose cleanup is different from its neighbour's. The split is the
// busy key: a pane that did not take the launch is not the persona being
// unavailable (dispatch_qa_test.go holds that pair).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// argvPersona writes a PID on a runtime that declares `prompt: argv`.
func argvPersona(t *testing.T, a *App, name, labels string) {
	t.Helper()
	os.MkdirAll(a.AgentsDir, 0o755)
	md := "---\nname: " + name + "\ndescription: test\nlabels: " + labels + "\nruntime: grok\n---\nYou are " + name + ".\n"
	if err := os.WriteFile(filepath.Join(a.AgentsDir, name+".md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The whole point of the ADR: on an argv runtime the work prompt is an
// argument to the CLI, so the screen is never the delivery channel. Nothing
// is typed at the pane, and herdr is never asked to recognize a composer.
func TestArgvDeliversTheWorkPromptOnTheLaunchLine(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	argvPersona(t, b.App, "ranger", "[go]")
	repo := qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, `[{"id":"a-1","title":"t","status":"closed"}]`)
	idleClaude(t, fake)

	n, err := d.Run("", "", 0)
	if err != nil || n != 1 {
		t.Fatalf("dispatched %d, err=%v:\n%s", n, err, dispatcherOut(d))
	}
	session := SessionForBead("ranger", repo, "a-1")
	file := b.App.WorkPromptFile(session)
	body, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("no work prompt written for %s: %v", session, err)
	}
	if !strings.HasPrefix(string(body), "Work beads issue a-1") {
		t.Errorf("the file must hold the assembled ADR 0005 prompt, got:\n%.120s", body)
	}
	log := calls(t, fake)
	if !strings.Contains(log, ArgvPromptSuffix(file)) {
		t.Errorf("the launch line must end in %s:\n%s", ArgvPromptSuffix(file), log)
	}
	// The failure this replaces: a prompt typed into a screen herdr guessed
	// was idle. Not typed at all now.
	if strings.Contains(log, "agent prompt") {
		t.Errorf("argv delivery must type nothing at the pane:\n%s", log)
	}
	if !strings.Contains(dispatcherOut(d), "prompt on the launch line") {
		t.Errorf("the pass must say how the prompt got there:\n%s", dispatcherOut(d))
	}
}

// Step 1 is the fence. A lost claim creates NOTHING — no workspace, no
// prompt file, no persona sitting in a repo working someone else's bead.
// Asserted by absence rather than by ordering two logs: if the create ran
// first there would be a workspace to find.
func TestArgvClaimsBeforeCreatingTheSession(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	argvPersona(t, b.App, "ranger", "[go]")
	repo := qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`,
		`[{"id":"a-1","title":"t","status":"in_progress","assignee":"devops"}]`)
	os.WriteFile(filepath.Join(repo, "fake-claim-lost"), []byte("devops"), 0o644)
	idleClaude(t, fake)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if n != 0 || !strings.Contains(out, "claim lost") {
		t.Fatalf("want a clean claim loss, got n=%d:\n%s", n, out)
	}
	if log := calls(t, fake); strings.Contains(log, "workspace create") {
		t.Errorf("a lost claim created a session anyway — the claim is not the fence:\n%s", log)
	}
	if _, err := os.Stat(b.App.WorkPromptFile(SessionForBead("ranger", repo, "a-1"))); !os.IsNotExist(err) {
		t.Errorf("a lost claim wrote the work prompt anyway (%v)", err)
	}
	// A lost claim is the bead's failure, not the pane's: nothing benched.
	if strings.Contains(out, "skipped for the rest of this pass") || strings.Contains(out, "keeps its slot") {
		t.Errorf("a claim race is neither a session nor a persona failure:\n%s", out)
	}
}

// Step 5: create-fails-after-claim unclaims. The claim goes first so a race
// loses cleanly, and the price of that is handing the bead back when the
// session it was claimed for never came into being.
func TestArgvCreateFailureAfterTheClaimUnclaims(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	argvPersona(t, b.App, "ranger", "[go]")
	qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
	idleClaude(t, fake)
	os.WriteFile(filepath.Join(fake, "create-error"), []byte("internal|fake herdr: workspace create refused"), 0o644)

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if n != 0 || !strings.Contains(out, "unclaimed") {
		t.Fatalf("a create that failed after the claim must unclaim, got n=%d:\n%s", n, out)
	}
	bd := bdCalls(t, fake)
	if !strings.Contains(bd, "--claim") {
		t.Fatalf("the claim must have been attempted first:\n%s", bd)
	}
	if !strings.Contains(bd, "--status open") {
		t.Errorf("the bead stays claimed by a persona that has no session:\n%s", bd)
	}
}

// Step 4, and the one outcome the typed path has no equivalent for. The
// prompt is with the CLI the moment the line runs, so a screen herdr never
// recognized is missing EVIDENCE, not missing work: the claim is kept, the
// bead is not judged, and no settle-wait is started over herdr's idle guess
// (which would return instantly and read a session that never worked as one
// that settled).
func TestArgvUnrecognizedScreenKeepsTheClaimAndJudgesNothing(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.StartupWait = 150 * time.Millisecond
	argvPersona(t, b.App, "ranger", "[go]")
	qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, "")
	idleClaude(t, fake)
	os.WriteFile(filepath.Join(fake, "explain-fallback"), nil, 0o644) // guess forever

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	out := dispatcherOut(d)
	if !strings.Contains(out, "herdr never recognized a screen") {
		t.Errorf("the pass must say what it could not see:\n%s", out)
	}
	if !strings.Contains(out, "claim kept") {
		t.Errorf("a delivered prompt whose screen is unreadable keeps its claim:\n%s", out)
	}
	if bd := bdCalls(t, fake); strings.Contains(bd, "--status open") {
		t.Errorf("the bead was handed back although its prompt was delivered:\n%s", bd)
	}
	if log := calls(t, fake); strings.Contains(log, "agent wait") {
		t.Errorf("a settle-wait started over herdr's idle guess:\n%s", log)
	}
	// And it is not the persona's failure either — the next bead launches.
	if strings.Contains(out, "skipped for the rest of this pass") {
		t.Errorf("an unreadable screen must not bench the persona:\n%s", out)
	}
}

// "Resume into a live session stays agent prompt" (ADR 0013 §2). The launch
// line has already been typed; the only way into a running CLI is its
// composer, argv runtime or not.
func TestArgvResumeIntoALiveSessionStaysTyped(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	d.Resume = true
	argvPersona(t, b.App, "ranger", "[go]")
	repo := qaRepo(t, b.App,
		`[{"id":"a-1","title":"t","labels":["go"],"status":"in_progress","assignee":"ranger"}]`,
		`[{"id":"a-1","title":"t","status":"closed"}]`)
	session := SessionForBead("ranger", repo, "a-1")
	mustCreate(t, b, NewSessionOpts{Name: session, Dir: repo, Agent: "ranger"})
	idleClaude(t, fake)
	os.Remove(filepath.Join(fake, "calls.log"))

	if _, err := d.Run("", "", 0); err != nil {
		t.Fatal(err)
	}
	log := calls(t, fake)
	if !strings.Contains(log, "agent prompt") {
		t.Errorf("a resume must type into the live composer:\n%s\n%s", log, dispatcherOut(d))
	}
	if strings.Contains(log, "workspace create") {
		t.Errorf("a resume must not build a second session beside the holder:\n%s", log)
	}
	if _, err := os.Stat(b.App.WorkPromptFile(session)); !os.IsNotExist(err) {
		t.Errorf("a resume wrote an argv prompt file it can never deliver (%v)", err)
	}
}

// claude is untouched: it declares `prompt: typed`, and a typed dispatch
// still creates, waits for a screen it can see, claims, and types.
func TestTypedRuntimeIsUnchangedByArgvDelivery(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	writePersona(t, b.App, "ranger", "[go]")
	repo := qaRepo(t, b.App, `[{"id":"a-1","title":"t","labels":["go"]}]`, `[{"id":"a-1","title":"t","status":"closed"}]`)
	idleClaude(t, fake)

	n, err := d.Run("", "", 0)
	if err != nil || n != 1 {
		t.Fatalf("dispatched %d, err=%v:\n%s", n, err, dispatcherOut(d))
	}
	if log := calls(t, fake); !strings.Contains(log, "agent prompt w1:p1 Work beads issue a-1") {
		t.Errorf("the typed path must still type the prompt:\n%s", log)
	}
	if _, err := os.Stat(b.App.WorkPromptFile(SessionForBead("ranger", repo, "a-1"))); !os.IsNotExist(err) {
		t.Errorf("a typed launch wrote an argv prompt file (%v)", err)
	}
	// The claim still happens after the session is promptable there: on the
	// typed path the risky step is the prompt, and moving the claim ahead of
	// a wait that can take 45s would hold beads open for nothing.
	if !strings.Contains(dispatcherOut(d), "prompted,") {
		t.Errorf("the typed launch line must not claim to be argv:\n%s", dispatcherOut(d))
	}
}

// A `prompt: argv` file cannot be handed to a runtime that does not declare
// it: the CLI would take the page of text as a subcommand or a path. The
// refusal is at the render, where the pair is known.
func TestPromptFileRefusedOnATypedRuntime(t *testing.T) {
	b, _ := newTestBackend(t)
	writePersona(t, b.App, "ranger", "[go]")
	err := b.CreateSession(NewSessionOpts{Name: "s1", Dir: t.TempDir(), Agent: "ranger", PromptFile: "/tmp/nope.txt"})
	if err == nil || !strings.Contains(err.Error(), "prompt: typed") {
		t.Errorf("claude must refuse a launch-line prompt, got %v", err)
	}
}

// A positional argument that starts with a dash is parsed as a flag, not as
// text — codex died on exactly that when a PID's `---` frontmatter was
// passed as its prompt (rangerhq-5oi). Nothing assembles such a prompt
// today; this is what keeps that true.
func TestWorkPromptStartingWithADashIsRefused(t *testing.T) {
	b, _ := newTestBackend(t)
	if _, err := b.App.WriteWorkPrompt("s1", "--help\nrest of it\n"); err == nil {
		t.Error("a prompt starting with a dash must refuse, not be written")
	}
	if _, err := b.App.WriteWorkPrompt("s1", "Work beads issue a-1\n"); err != nil {
		t.Errorf("an ordinary work prompt must write: %v", err)
	}
}
