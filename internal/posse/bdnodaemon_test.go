package posse

// ranger-base-cwu7: every bd invocation posse makes goes out `--no-daemon`.
//
// The number that earns this: ~5.3s of every ~5.6s bd call was the daemon
// dial, flat across result size, in a store where bd cannot start a daemon
// at all and says so (`Mode: direct, Connected: no`). Bd.run's doc comment
// carries the full sweep. What is pinned here is that the flag is on the
// RUNNER — one seam, every verb, reads and writes alike — because the way
// this regresses is somebody adding a method that builds its own argv, or a
// refactor that drops the prefix from a `run` nobody re-measured.
//
// Two halves, because a fake and a binary answer different questions:
//
//  1. TestBdRunCarriesNoDaemonOnEveryVerb — a recording fake, in the
//     ordinary suite. Ours to keep: the flag is there, first, ahead of the
//     verb, for every method a caller can reach.
//  2. TestLiveBdRunSkipsTheDialTheDaemonArmPays — the real binary,
//     env-gated. bd's to keep: the flag still buys the seconds and still
//     answers with the same rows. It runs both arms, so the box calibrates
//     itself — see the test for the observable that did NOT work.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// recordingBd is a bd that answers every verb with an empty JSON array and
// appends its argv to a log, one call per line, NUL-separated within a line
// so an argument containing a space cannot forge a boundary.
func recordingBd(t *testing.T) (Bd, func() [][]string) {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "argv.log")
	bin := filepath.Join(dir, "bd")
	body := "#!/bin/sh\n" +
		"printf '%s\\0' \"$@\" >> " + log + "\n" +
		"printf '\\n' >> " + log + "\n" +
		"echo '[]'\n"
	if err := os.WriteFile(bin, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return Bd{Bin: bin}, func() [][]string {
		b, err := os.ReadFile(log)
		if err != nil {
			return nil
		}
		var calls [][]string
		for _, line := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
			args := strings.Split(strings.TrimSuffix(line, "\x00"), "\x00")
			calls = append(calls, args)
		}
		return calls
	}
}

func TestBdRunCarriesNoDaemonOnEveryVerb(t *testing.T) {
	b, calls := recordingBd(t)
	dir := t.TempDir()

	// Every method that reaches run, not a sample of them: the read scans
	// the cockpit and the dispatch pass live on, and the writes, which cost
	// the same 5.3s and which a reads-only fix would have left paying it.
	// Return values are ignored on purpose — the fake's `[]` does not parse
	// as an id or an issue, and the argv is recorded before any of that.
	verbs := map[string]func(){
		"ListAll":        func() { b.ListAll(dir) },
		"Ready":          func() { b.Ready(dir, "an-actor") },
		"InProgress":     func() { b.InProgress(dir) },
		"OpenLabeledAny": func() { b.OpenLabeledAny(dir, "bug") },
		"Blocked":        func() { b.Blocked(dir) },
		"Comments":       func() { b.Comments(dir, "x-1") },
		"CommentCount":   func() { b.CommentCount(dir, "x-1") },
		"Show":           func() { b.Show(dir, "x-1") },
		"DepList":        func() { b.DepList(dir, "x-1") },
		"Dependents":     func() { b.Dependents(dir, "x-1") },
		"Flush":          func() { b.Flush(dir) },
		"Claim":          func() { b.Claim(dir, "x-1", "an-actor") },
		"Unclaim":        func() { b.Unclaim(dir, "x-1", "an-actor", false) },
		"Close":          func() { b.Close(dir, "x-1", "an-actor") },
		"Comment":        func() { b.Comment(dir, "x-1", "a note", "an-actor") },
		"DepAdd":         func() { b.DepAdd(dir, "x-1", "x-2", "an-actor") },
		"Create":         func() { b.Create(dir, BdNew{Title: "t", Actor: "an-actor"}) },
	}
	for _, run := range verbs {
		run()
	}

	got := calls()
	if len(got) < len(verbs) {
		t.Fatalf("the fake recorded %d calls for %d verbs — it is not being reached, so nothing below means anything: %v",
			len(got), len(verbs), got)
	}
	for _, args := range got {
		if len(args) == 0 || args[0] != "--no-daemon" {
			t.Errorf("bd %s: --no-daemon must lead the argv", strings.Join(args, " "))
		}
		// bd's globals go BEFORE the verb, and posse's own argv gate fences
		// the resolved verb — so a flag that drifted behind the subcommand
		// would still be a different command than the one measured.
		for i, a := range args {
			if !strings.HasPrefix(a, "-") && a != "" {
				if i == 0 {
					t.Errorf("bd %s: the verb leads the argv", strings.Join(args, " "))
				}
				break
			}
		}
	}
}

// TestLiveBdRunSkipsTheDialTheDaemonArmPays asks the real binary the two
// things the fake cannot: that the flag still buys the seconds, and that it
// does not change the answer.
//
// The rig is a `.beads/issues.jsonl` and nothing else: bd materialises a
// full database from the seed row on the first ordinary command, so this
// needs neither `bd init` (which personas' PIDs deny, and which installs
// bd's own pre-commit hook — a second daemon vector) nor a cleanup for a
// daemon. Measured 2026-08-30 on this rig, bd 0.49.1: ONE plain `bd list`
// costs 5.77s and leaves no daemon.log, daemon.pid, daemon.lock or
// daemon-error behind — the client pays a dial for a daemon that never gets
// as far as writing a file. That is why this pin is not a stat() on those
// names: written that way it passed with bdGlobalFlags emptied, measuring
// nothing at all.
//
// So the observable is the cost itself, and it is a COMPARISON rather than a
// threshold — the daemon arm runs here too and calibrates the box. Empty
// bdGlobalFlags and both arms become the same arm, the gap collapses, and
// this goes red.
func TestLiveBdRunSkipsTheDialTheDaemonArmPays(t *testing.T) {
	if os.Getenv("RHQ_LIVE_BD") == "" {
		t.Skip("set RHQ_LIVE_BD=1 (shells out to the real bd)")
	}
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("no bd on PATH")
	}
	repo := t.TempDir()
	beads := filepath.Join(repo, ".beads")
	if err := os.MkdirAll(beads, 0o755); err != nil {
		t.Fatal(err)
	}
	const seed = `{"id":"cwu7-1","title":"seed row","status":"open","priority":2,` +
		`"issue_type":"task","created_at":"2026-08-30T00:00:00Z","updated_at":"2026-08-30T00:00:00Z"}`
	if err := os.WriteFile(filepath.Join(beads, "issues.jsonl"), []byte(seed+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, repo, "init", "-q", ".")

	// The daemon arm first, so it is the one that pays for materialising the
	// database — otherwise the direct arm carries that cost and the gap is
	// measured against the wrong baseline.
	cmd := exec.Command("bd", "list", "--all", "--json", "--limit", "0")
	cmd.Dir = repo
	daemonArm := time.Now()
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("the daemon arm did not run, so there is no baseline: %v", err)
	}
	dialed := time.Since(daemonArm)
	want, err := parseBdIssues(raw)
	if err != nil {
		t.Fatalf("the daemon arm answered nothing parseable: %v\n%s", err, raw)
	}

	directArm := time.Now()
	got, err := Bd{Bin: "bd"}.ListAll(repo)
	if err != nil {
		t.Fatalf("ListAll against a jsonl-only rig: %v", err)
	}
	direct := time.Since(directArm)

	// Same store, same rows. The seconds are worth nothing if the answer
	// moved, and a rig that resolved to no rows would make the timing arm
	// vacuous too.
	if len(want) != 1 || len(got) != len(want) || got[0].ID != want[0].ID {
		t.Fatalf("direct and daemon arms disagree: direct=%+v daemon=%+v", got, want)
	}

	// A comparison, not a stopwatch: the claim is that the dial is most of
	// the call, so half the daemon arm is a floor no loaded box crosses by
	// accident and no reverted fix clears.
	if direct > dialed/2 {
		t.Errorf("the direct read cost %v against the daemon arm's %v — the dial is not being skipped", direct, dialed)
	}
	t.Logf("direct %v, daemon arm %v", direct, dialed)
}
