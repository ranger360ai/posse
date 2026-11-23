package main

// QA pin for ranger-base-nqtz5 F1 (from ranger-base-56gd0's verify of
// ranger-base-p51zu): the hand-prompt `prompted:` stamp write was witnessed
// by nothing.
//
// `hb.MarkPrompted(name, time.Now())` in cmd/posse `case "prompt"` is the
// second half of ranger-base-p51zu's fix — the half its own close calls the
// measured recovery for G19, and the one G19's Detail prints as a remedy
// (`posse prompt <session>`). MEASURED 2026-11-22 (linux 7.2.15-1-omarchy,
// go1.27.3, main at 814776ed): deleting that one line left `go vet
// ./cmd/posse` clean, `go test -count=1 ./cmd/posse` ok in 103s and `make
// treepins-check` ok in 89.3s. `MarkPrompted` has exactly two call sites —
// dispatch.go:1544, pinned by TestRunRecordPersistsBeadAndPrompted, and this
// one, pinned by nothing — and the four G19 pins the same close added are all
// in internal/posse, which cannot reach package main.
//
// THE PIN IS OVER THE RECORD, NOT OVER THE CALL. What both readers need is a
// `prompted:` stamp in the run record (ADR 0011 §3) — promptedRecently, so a
// pass in another process does not type a second instruction into a seat
// mid-turn, and G19, which reads a bead-carrying seat with no stamp as a seat
// that never got its work. So this reads the record back off disk after the
// verb has run, which holds the fact whichever writer puts it there, and the
// arm in internal/posse (unpromptedpredicate_qa_test.go,
// TestMarkPromptedSatisfiesBothReadersOfTheStamp) holds that a stamp of this
// shape stands both readers down. Together: the verb writes it, and what it
// writes is what they read.
//
// The fixture is promptsubmit_qa_test.go's fake herdr, plus the one record
// state no launch leaves behind on purpose — `bead:` set, `launched:` old,
// `prompted:` absent, which is G19's own row.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ranger360ai/posse/internal/posse"
)

// handPromptLaunched is the `launched:` stamp the record carries going in:
// old enough to be past unpromptedGrace, so the fixture is a seat G19 really
// would raise and not one sitting inside its own launch patience.
const handPromptLaunched = 30 * time.Minute

// handPromptRig is promptSubmitHerdr's home with the record put into G19's
// state: a bead pointer, an old launch stamp and no prompt stamp. Written
// straight onto the meta rather than driven through a launch, for the reason
// govLaunchStamps gives one package over — the state's whole subject is a
// record no launch leaves behind.
func handPromptRig(t *testing.T, box string) (home, herdr string, launched time.Time) {
	t.Helper()
	home, herdr = promptSubmitHerdr(t, box)
	launched = time.Now().Add(-handPromptLaunched).UTC().Truncate(time.Second)
	meta := filepath.Join(home, "state", "herdr", "fresh.yaml")
	if err := os.WriteFile(meta, []byte(""+
		"name: fresh\n"+
		"workspace: w1\n"+
		"pane: p1\n"+
		"agent: developer\n"+
		"bead: ranger-base-72z36\n"+
		"launched: "+launched.Format(time.RFC3339)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return home, herdr, launched
}

// handPromptStamp reads the `prompted:` field back off the record, as the
// FILE — which is what every cross-process reader of it does (promptedRecently
// reads the meta as a file on purpose, dispatch.go) and what a test in this
// package can reach. The empty string means the line is absent, which is the
// state the deleted write leaves and is not the same claim as an unparseable
// one.
func handPromptStamp(t *testing.T, home string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(home, "state", "herdr", "fresh.yaml"))
	if err != nil {
		t.Fatalf("no record to read back: %v", err)
	}
	for _, line := range strings.Split(string(body), "\n") {
		if v, ok := strings.CutPrefix(line, "prompted:"); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// The fix, on the entry point G19's own remedy names. A hand `posse prompt`
// leaves the stamp in the record, so the next reader in any process can see
// that this seat got its work.
func TestQAHandPromptStampsThePromptedFieldOnTheRecord(t *testing.T) {
	home, herdr, launched := handPromptRig(t, "❯\\n")
	before := time.Now().UTC().Truncate(time.Second)
	out := runPrompt(t, home, herdr)
	after := time.Now().UTC()

	// The apparatus first: a prompt that never reached herdr would leave the
	// stamp absent for a second reason, and the assertions below could not
	// tell the two apart.
	if !strings.Contains(out, "agent_prompted") {
		t.Fatalf("the prompt did not reach herdr — this fixture cannot say anything about the stamp:\n%s", out)
	}

	stamp := handPromptStamp(t, home)
	if stamp == "" {
		t.Fatalf("`posse prompt` left the record with no prompted: stamp — the hand path is the measured "+
			"recovery for G19 (unprompted.go) and the persisted half of PromptGrace (ADR 0011 §3), so "+
			"without it the remedy cannot clear its own condition and a pass in another process cannot "+
			"see that this seat was prompted at all (ranger-base-p51zu, ranger-base-nqtz5 F1). The "+
			"writer is hb.MarkPrompted in cmd/posse `case \"prompt\"`:\n%s", out)
	}
	at, err := time.Parse(time.RFC3339, stamp)
	if err != nil {
		t.Fatalf("prompted: %q does not parse as RFC3339 — parseLaunched reads an unparseable stamp as "+
			"the zero time, which is exactly the absence this pin is about (herdrback.go): %v", stamp, err)
	}
	// The same relation TestRunRecordPersistsBeadAndPrompted holds over the
	// dispatch call site: a stamp before the launch it belongs to is not this
	// run's prompt.
	if at.Before(launched) {
		t.Errorf("prompted: %s is before launched: %s", at.Format(time.RFC3339), launched.Format(time.RFC3339))
	}
	// And it is THIS prompt's clock rather than some value carried in from
	// the fixture: a second's truncation either side, because the record
	// holds RFC3339 seconds.
	if at.Before(before.Add(-time.Second)) || at.After(after.Add(time.Second)) {
		t.Errorf("prompted: %s is outside the window this command ran in (%s..%s) — the stamp is not "+
			"this prompt's", at.Format(time.RFC3339), before.Format(time.RFC3339), after.Format(time.RFC3339))
	}
	// The bead pointer is untouched. Both halves of the record are read as
	// one pair by G19 — `bead:` set with no `prompted:` under it — so a write
	// that landed the stamp by clearing the pointer would stand the condition
	// down for the wrong reason.
	body, err := os.ReadFile(filepath.Join(home, "state", "herdr", "fresh.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "bead: ranger-base-72z36") {
		t.Errorf("the prompt lost the record's bead pointer:\n%s", body)
	}
}

// THE WRONG ARM, and the reason the stamp is written where it is: after the
// prompt took. A failed prompt is not a prompt, and a record saying one was
// delivered is worse than one saying nothing — promptedRecently would hold
// the next pass off re-prompting a seat that never got its work, which is the
// repair G19 is counting on (unprompted.go, "THE PASS THAT HEALS IT").
//
// The herdr here refuses `agent prompt`, so the verb dies before the marks.
// Moving MarkPrompted above the Prompt call greens the arm above and reds
// this one.
func TestQAAPromptThatFailedStampsNothing(t *testing.T) {
	home, _, _ := handPromptRig(t, "❯\\n")
	herdr := filepath.Join(t.TempDir(), "herdr")
	// Everything the verb asks before the prompt is answered, and the prompt
	// itself is refused — so what this arm varies against the one above is
	// one herdr verb and nothing else.
	script := `#!/bin/sh
case "$1 $2" in
"workspace list")
  printf '%s\n' '{"result":{"workspaces":[{"workspace_id":"w1","label":"fresh","agent_status":"idle"}]}}'
  exit 0;;
"agent list")
  printf '%s\n' '{"result":{"agents":[{"agent":"claude","agent_status":"idle","pane_id":"p1","workspace_id":"w1"}]}}'
  exit 0;;
"agent explain")
  printf '%s\n' '{"state":"idle","matched_rule":{"id":"live_prompt_box"},"visible_idle":true,"fallback_reason":null,"evaluated_rules":[{"id":"live_prompt_box","matched":true,"region":"prompt_box_body","state":"idle","evidence":{"region_bytes":4,"region_preview":"❯\n"}}]}'
  exit 0;;
"agent prompt")
  printf '%s\n' '{"error":{"code":"no","message":"pane is gone"}}'
  exit 1;;
esac
printf '%s\n' '{"error":{"code":"no","message":"unexpected"}}'
exit 1
`
	if err := posse.WriteExecutable(herdr, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(buildRhq(t), "prompt", "fresh", promptText)
	cmd.Env = append(os.Environ(),
		"RHQ_HOME="+home,
		"RHQ_HERDR_BIN="+herdr,
		"HERDR_SOCKET_PATH="+filepath.Join(home, "no-such.sock"))
	raw, err := cmd.CombinedOutput()
	out := string(raw)
	if err == nil {
		t.Fatalf("a refused prompt exited 0:\n%s", out)
	}
	if stamp := handPromptStamp(t, home); stamp != "" {
		t.Errorf("a prompt herdr refused left prompted: %s on the record — the next pass then reads this "+
			"seat as prompted and declines to deliver the work it never got (promptedRecently, "+
			"dispatch.go), and G19 stands down over a seat still sitting at its banner:\n%s", stamp, out)
	}
}
