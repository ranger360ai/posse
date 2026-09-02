package posse

// QA, ranger-base-q32o — the chain prescription re-read at a slot posse has
// ALREADY chained. Everything here runs the hook files the way git does and
// builds its fixtures out of the prescription posse actually prints, never a
// retyped copy of it: a pin whose fixture is a copy of the thing under test
// measures the copy (the lesson gateschain_qa_test.go's header records).
//
// Two defects are pinned, both reached by following posse's own instructions:
//
//  1. the prescription's first step was a bare `mv <slot> theirs-<slot>`. At a
//     chained slot that name is the third party's hook, and `mv` destroys it
//     without a word — leaving `theirs-<slot>` holding the dispatcher, whose
//     last line `exec`s into `theirs-<slot>`. Itself. The loop sits PAST the
//     gate's refusal, so a refused push still exits 1 and the wall looks
//     intact; only a PERMITTED push reaches the loop, and then it spins with
//     no output and no exit.
//
//  2. the operator arrives at that prescription because `install-hooks`
//     refuses a slot holding posse's own dispatcher when `posse-<slot>` is
//     gone — which is what the marker line inside `posse-<slot>` tells the
//     operator to remove. Re-chaining was never the repair; restoring the
//     gate is.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

const q32oTheirHook = "#!/bin/sh\necho \"their pre-push ran\" >&2\nexit 0\n"

// q32oPrescription is the chain block install-hooks prints when it refuses
// the pre-push slot. It is the fixture for everything below.
func q32oPrescription(t *testing.T, repo string) string {
	t.Helper()
	_, err := InstallPrePushHook(repo)
	if err == nil {
		t.Fatal("install-hooks must refuse a foreign pre-push and print the chain")
	}
	text := err.Error()
	if !strings.Contains(text, "Chain it —") {
		t.Fatalf("refusal carries no chain prescription: %q", text)
	}
	return text
}

var q32oMove = regexp.MustCompile(`(?m)^mv (\S+) (\S+)$`)

// q32oPaste performs the printed prescription, step for step, and holds every
// step to the one rule the defect broke: a `mv` in pasted text must never name
// a file that is already there. `posse gates install-hooks` is the installer
// call itself rather than a shelled-out binary; the two mv's and the
// dispatcher body come out of the printed text.
func q32oPaste(t *testing.T, repo, hooks, text string) {
	t.Helper()
	moves := q32oMove.FindAllStringSubmatch(text, -1)
	if len(moves) != 2 {
		t.Fatalf("prescription no longer has exactly two mv steps: %q", text)
	}
	open := "cat > pre-push <<'EOF'\n"
	i := strings.Index(text, open)
	if i < 0 {
		t.Fatalf("prescription no longer writes pre-push with a heredoc: %q", text)
	}
	rest := text[i+len(open):]
	j := strings.Index(rest, "\nEOF\n")
	if j < 0 {
		t.Fatalf("prescription's heredoc has no terminator: %q", text)
	}
	body := rest[:j+1]

	// A `mv` in pasted text may land on an existing file only when it cannot
	// destroy anything: the prescription re-writes posse-<slot> with the gate
	// it just installed, byte for byte. Any other collision loses a file, and
	// mv says nothing about it.
	mv := func(m []string) {
		from, to := filepath.Join(hooks, m[1]), filepath.Join(hooks, m[2])
		src, err := os.ReadFile(from)
		if err != nil {
			t.Fatal(err)
		}
		if dst, err := os.ReadFile(to); err == nil && string(dst) != string(src) {
			t.Fatalf("prescription pastes `mv %s %s` over a different file — mv destroys it silently", m[1], m[2])
		}
		if err := os.Rename(from, to); err != nil {
			t.Fatal(err)
		}
	}
	mv(moves[0])
	if _, err := InstallPrePushHook(repo); err != nil {
		t.Fatalf("the prescription's own install step failed: %v", err)
	}
	mv(moves[1])
	if err := os.WriteFile(filepath.Join(hooks, "pre-push"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// q32oChainedRepo is a repo carrying a foreign pre-push, chained by pasting
// the prescription posse printed at it: pre-push = the dispatcher,
// posse-pre-push = the gate, theirs-pre-push = the foreign hook.
func q32oChainedRepo(t *testing.T) (repo, hooks string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo = t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	hooks = filepath.Join(repo, ".git", "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "pre-push"), []byte(q32oTheirHook), 0o755); err != nil {
		t.Fatal(err)
	}
	q32oPaste(t, repo, hooks, q32oPrescription(t, repo))
	if !PrePushHookInstalled(repo) {
		t.Fatal("the pasted chain is not recognized as installed")
	}
	return repo, hooks
}

// q32oRun runs the slot the way git does, on a BUDGET: the defect's whole
// signature is a run that never returns, so a pin without a deadline would
// hang the suite instead of failing it.
func q32oRun(t *testing.T, repo, slot string, env ...string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(repo, ".git", "hooks", slot), "origin", "https://example.invalid/x.git")
	cmd.Dir = repo
	cmd.Stdin = strings.NewReader("refs/heads/main a refs/heads/main b\n")
	cmd.Env = append([]string{"PATH=" + PathOutsideGates("")}, env...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("%s never returned (self-exec loop): %q", slot, out)
	}
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("%s: %v", slot, err)
	}
	return string(out), code
}

// q32oNoSelfExec fails if any hook file in the dir ends up exec'ing into
// itself — the state the clobbering mv left behind, stated directly.
func q32oNoSelfExec(t *testing.T, hooks string) {
	t.Helper()
	ents, err := os.ReadDir(hooks)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		b, err := os.ReadFile(filepath.Join(hooks, e.Name()))
		if err != nil {
			continue
		}
		if strings.Contains(string(b), "exec \"$d/"+e.Name()+"\" \"$@\"") {
			t.Errorf("%s execs into itself — every permitted run of it hangs", e.Name())
		}
	}
}

// The reachable path, end to end. `posse-<slot>` is removed — which is what
// the marker line inside it says to do to uninstall — and install-hooks is
// re-run, the operator's natural repair. It must restore the gate the
// dispatcher already runs, not refuse and print a prescription that destroys
// the neighbour.
func TestQARemovedGateBehindAChainIsRestoredNotReChained(t *testing.T) {
	t.Parallel()
	repo, hooks := q32oChainedRepo(t)
	dispatcher, err := os.ReadFile(filepath.Join(hooks, "pre-push"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(hooks, "posse-pre-push")); err != nil {
		t.Fatal(err)
	}

	p, err := InstallPrePushHook(repo)
	if err != nil {
		t.Fatalf("install-hooks must restore the gate behind an intact dispatcher: %v", err)
	}
	if want := filepath.Join(hooks, "posse-pre-push"); p != want {
		t.Errorf("restored %s, want %s", p, want)
	}
	if !PrePushHookInstalled(repo) {
		t.Error("the restored chain must read as installed")
	}
	if got, err := os.ReadFile(filepath.Join(hooks, "theirs-pre-push")); err != nil || string(got) != q32oTheirHook {
		t.Errorf("their hook must be untouched: %v %q", err, got)
	}
	if got, err := os.ReadFile(filepath.Join(hooks, "pre-push")); err != nil || string(got) != string(dispatcher) {
		t.Errorf("the dispatcher must be untouched: %v %q", err, got)
	}
	q32oNoSelfExec(t, hooks)

	// The wall still refuses what it refused.
	if out, code := q32oRun(t, repo, "pre-push", "RHQ_PERSONA=qa", "RHQ_TOOLS_DENY=Bash(git push:*)", "RHQ_GATES_DIR="+t.TempDir()); code != 1 ||
		!strings.Contains(out, "refused by posse gate: git push") {
		t.Errorf("denied push must refuse with exit 1: code=%d %q", code, out)
	}
	// And the permitted push — the only path that reaches the exec, and so
	// the only one the loop was ever visible on — completes and runs theirs.
	out, code := q32oRun(t, repo, "pre-push")
	if code != 0 {
		t.Errorf("permitted push must pass: code=%d %q", code, out)
	}
	if !strings.Contains(out, "their pre-push ran") {
		t.Errorf("permitted push must reach their hook: %q", out)
	}
}

// The other half: whatever else is wrong, the prescription posse prints must
// never tell the operator to `mv` a hook onto another hook. Here a third-party
// tool has retaken the slot of an already-chained repo, so `theirs-pre-push`
// is occupied and the printed block is still the way through.
func TestQAPrescriptionAtAChainedSlotMovesNothingOntoAnotherHook(t *testing.T) {
	t.Parallel()
	repo, hooks := q32oChainedRepo(t)
	const retaken = "#!/bin/sh\necho \"the new tool ran\" >&2\nexit 0\n"
	if err := os.WriteFile(filepath.Join(hooks, "pre-push"), []byte(retaken), 0o755); err != nil {
		t.Fatal(err)
	}

	text := q32oPrescription(t, repo)
	if strings.Contains(text, "mv pre-push theirs-pre-push\n") {
		t.Error("prescription still moves the slot onto the existing theirs-pre-push")
	}
	if !strings.Contains(text, "theirs-pre-push is already taken") {
		t.Errorf("prescription must say why the name differs: %q", text)
	}
	// q32oPaste fails the test itself if any step moves onto an existing file.
	q32oPaste(t, repo, hooks, text)

	if got, err := os.ReadFile(filepath.Join(hooks, "theirs-pre-push")); err != nil || string(got) != q32oTheirHook {
		t.Errorf("the first tool's hook must survive the paste: %v %q", err, got)
	}
	q32oNoSelfExec(t, hooks)
	if !PrePushHookInstalled(repo) {
		t.Error("the re-pasted chain must read as installed")
	}
	if out, code := q32oRun(t, repo, "pre-push", "RHQ_PERSONA=qa", "RHQ_TOOLS_DENY=Bash(git push:*)", "RHQ_GATES_DIR="+t.TempDir()); code != 1 ||
		!strings.Contains(out, "refused by posse gate: git push") {
		t.Errorf("denied push must refuse with exit 1: code=%d %q", code, out)
	}
	out, code := q32oRun(t, repo, "pre-push")
	if code != 0 || !strings.Contains(out, "the new tool ran") {
		t.Errorf("permitted push must pass and reach the retaken hook: code=%d %q", code, out)
	}
}

// A dispatcher over a `posse-<slot>` that is NOT ours is a third situation
// again: nothing there is safe to overwrite, and re-chaining would bury it.
// Refuse, name the file, and do not print a prescription that moves anything.
func TestQAChainOverAForeignPosseSlotRefusesWithoutPrescribingARechain(t *testing.T) {
	t.Parallel()
	repo, hooks := q32oChainedRepo(t)
	const foreign = "#!/bin/sh\necho not-ours\nexit 0\n"
	posse := filepath.Join(hooks, "posse-pre-push")
	if err := os.WriteFile(posse, []byte(foreign), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := InstallPrePushHook(repo)
	if err == nil {
		t.Fatal("install-hooks must not overwrite a foreign posse-pre-push")
	}
	if !strings.Contains(err.Error(), "posse-pre-push") {
		t.Errorf("the refusal must name the file it will not overwrite: %q", err)
	}
	if strings.Contains(err.Error(), "Chain it —") {
		t.Errorf("re-chaining is not the repair here: %q", err)
	}
	if got, readErr := os.ReadFile(posse); readErr != nil || string(got) != foreign {
		t.Errorf("posse-pre-push must be untouched: %v %q", readErr, got)
	}
}
