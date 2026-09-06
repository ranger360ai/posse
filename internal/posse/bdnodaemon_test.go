//go:build posse_arm3

package posse

// ranger-base-cwu7: every bd invocation posse makes goes out `--no-daemon`.
//
// The number that earns this: ~5.3s of every ~5.6s bd call was the daemon
// dial, flat across result size, in a store where bd cannot start a daemon
// at all and says so (`Mode: direct, Connected: no`). Bd.run's doc comment
// carries the full sweep. That number is bd 0.49.1's: on the pinned 0.50.3
// the daemon class is gone and the flag is deprecated, so it buys nothing
// and half 2 below no longer times anything — ranger-base-a67nu holds what
// that means for the flag and for the doc comment. What is pinned here is
// unchanged by any of it, because it was never the seconds: the flag is on the
// RUNNER — one seam, every verb, reads and writes alike — because the way
// this regresses is somebody adding a method that builds its own argv, or a
// refactor that drops the prefix from a `run` nobody re-measured.
//
// Two halves, because a fake and a binary answer different questions:
//
//  1. TestBdRunCarriesNoDaemonOnEveryVerb — a recording fake, in the
//     ordinary suite. Ours to keep: the flag is there, first, ahead of the
//     verb, for every method a caller can reach.
//  2. TestLiveBdRunAcceptsNoDaemonAndAnswersTheSameRows — the real binary,
//     env-gated. bd's to keep: the shipped bd still accepts the flag and
//     still answers with the same rows either way. It ran both arms for the
//     seconds too until bd 0.50.x retired the daemon class and the gap it
//     was measuring went with it — see the test for that measurement, and
//     for the two observables before it that did NOT work.

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
	t.Parallel()
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

// TestLiveBdRunAcceptsNoDaemonAndAnswersTheSameRows asks the real binary the
// thing the fake cannot: that the flag Bd.run puts in front of every verb is
// still one the shipped bd accepts, and that carrying it does not change the
// answer.
//
// The rig is a no-db (JSONL-only) store asked for BY NAME — a
// `.beads/config.yaml` holding `no-db: true` beside the seed row — and the
// class bd actually built is CHECKED below rather than trusted to the config.
// It used to be a `.beads/issues.jsonl` and nothing else, on the strength of
// bd 0.49.1 materialising a full database from the seed row on the first
// ordinary command. Measured 2026-09-04 on bd 0.50.3 (ranger-base-c201c):
// that same bare rig now builds a SQLite store — beads.db and metadata.json
// appear — which answers `[]`, so both arms came back empty and every
// assertion below it was vacuous. Naming the class keeps what the bare rig
// was standing in for: still no `bd init` (which personas' PIDs deny, and
// which installs bd's own pre-commit hook), and still no database for the
// seed row to fall out of sync with.
//
// WHAT THIS PIN NO LONGER CLAIMS, and why. It used to time both arms and
// require the direct one to cost under half the daemon arm — the dial being
// ~5.3s of a ~5.6s call, measured 2026-08-30 on bd 0.49.1 (ranger-base-cwu7;
// bdGlobalFlags' doc comment carries the sweep). That gap is gone with the
// daemon class, which went in 0.50.x (posse 291523c), and `bd --help` on
// 0.50.3 documents the flag as "(deprecated) All operations use direct mode".
// Measured 2026-09-04 on 0.50.3, five runs per arm, `bd list --all --json`
// with and without the flag: in this no-db rig 0.46-0.58s against
// 0.42-0.58s, and against the fleet's own SQLite store 0.45-0.66s against
// 0.48-0.59s. The same arm twice, in either store class. So the two arms are
// logged and NOT compared — a threshold either way would be measuring the
// box. Whether posse keeps carrying the flag, and the doc comment that still
// argues from the 0.49.1 seconds, are ranger-base-a67nu.
//
// What is left can still fail, and ranger-base-c201c is the witness that it
// does: a bd that stops accepting the deprecated flag fatals in the direct
// arm, a bd whose default store class drifts again is named at the class
// check before either answer is read, and a bd that answers the two arms
// differently fails the comparison that was always the point. Mutation-
// checked 2026-09-04 on 0.50.3: dropping the config.yaml fatals at the class
// check, emptying the seed fatals at the row comparison.
//
// And one thing this half no longer catches, said out loud rather than left
// to be discovered: emptying bdGlobalFlags now SURVIVES here (checked, same
// pass). With the flag a no-op both arms are the same command either way, so
// half 1's recording fake — which does kill that mutant — is the only thing
// holding the flag on the runner. Do not read a green here as covering it.
func TestLiveBdRunAcceptsNoDaemonAndAnswersTheSameRows(t *testing.T) {
	t.Parallel()
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
	// The store class, spelled out. The issue prefix stays bd's to infer from
	// the seed row — the config names the class and nothing else.
	if err := os.WriteFile(filepath.Join(beads, "config.yaml"), []byte("no-db: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	const seed = `{"id":"cwu7-1","title":"seed row","status":"open","priority":2,` +
		`"issue_type":"task","created_at":"2026-08-30T00:00:00Z","updated_at":"2026-08-30T00:00:00Z"}`
	if err := os.WriteFile(filepath.Join(beads, "issues.jsonl"), []byte(seed+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, repo, "init", "-q", ".")

	// The daemon arm first and the direct arm second, in the order the header
	// describes them. Neither materialises anything now, so the order buys
	// nothing and neither reading is a baseline for the other.
	cmd := exec.Command("bd", "list", "--all", "--json", "--limit", "0")
	cmd.Dir = repo
	daemonArm := time.Now()
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("the daemon arm did not run, so there is nothing to compare against: %v", err)
	}
	plain := time.Since(daemonArm)

	// The class bd built, read after the first command that could have built
	// one and before either answer is trusted: an empty arm below is then a
	// disagreement about rows, never a rig that quietly became a different
	// store. Without the config.yaml above, this is exactly what fires.
	if _, err := os.Stat(filepath.Join(beads, "beads.db")); err == nil {
		t.Fatalf("the rig asked for a no-db store and bd built a sqlite one (%s exists): the class this pin was measured on is gone, and both arms would be reading a database the seed row never reached",
			filepath.Join(beads, "beads.db"))
	}

	want, err := parseBdIssues(raw)
	if err != nil {
		t.Fatalf("the daemon arm answered nothing parseable: %v\n%s", err, raw)
	}

	directArm := time.Now()
	got, err := Bd{Bin: "bd"}.ListAll(repo)
	if err != nil {
		t.Fatalf("ListAll against a no-db rig — bdGlobalFlags carries %v, and a bd that stopped accepting one of them fails here: %v",
			bdGlobalFlags, err)
	}
	direct := time.Since(directArm)

	// Same store, same rows, with the flag and without it. This is the whole
	// live claim now, and the seeded row is named so a rig that resolves to
	// nothing fails here rather than passing on two empty answers.
	if len(want) != 1 || len(got) != len(want) || got[0].ID != want[0].ID {
		t.Fatalf("direct and daemon arms disagree: direct=%+v daemon=%+v", got, want)
	}

	t.Logf("direct %v, plain %v — logged, not compared: the daemon class is gone on bd 0.50.x (see the header)", direct, plain)
}
