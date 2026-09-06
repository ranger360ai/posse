//go:build posse_arm3

package posse

// QA pins from verify bead ranger-base-qca6i, measured against the close of
// ranger-base-lpoui (b1eb2bf). Three behaviours planstale.go's own comments
// state as deliberate and no pin holds: each one survived a mutant that
// changed it, and the shipped code is RIGHT in all three — what is missing
// is the day it stops being right going red.
//
// Method, so the next reader can re-take it: golden copy of planstale.go,
// one edit, rebuild `go test -c ./internal/posse`, run the lpoui pins, then
// restore. Control mutant (`age <= after` → `age <`) was KILLED, so the
// harness could fail. The three below were not.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A torn line ends the streak; it does not join it.
//
// planFailStreak walks the cadence log backwards and stops on any line it
// cannot parse into `<rfc3339> <caller> <outcome>`, and planLogOutcome's
// comment says why in as many words: "a file this cannot parse is not
// evidence of failure, and over-counting here would put a number in front
// of an operator that no store backs."
//
// MEASURED (verify ranger-base-qca6i): mutating that break to `outcome ==
// "ok"` alone — so an unparseable line counts as a failure and the walk
// carries on past it — leaves all eleven lpoui pins in this package GREEN.
// The fixture below is the difference: three fields short at the tail turns
// "no request has left this machine since" into "3 consecutive 429", which
// is the one sentence PlanStale.streak() has a whole comment about not
// saying by accident.
//
// Not hypothetical on this box. plan-usage.log is appended by several
// processes with O_APPEND and no lock (plancache.go logRead), and
// 2026-09-02 is the day the volume reached 133 MiB free and go builds took
// ENOSPC across the fleet (ranger-base-gcb1j). A write that runs out of
// disk mid-line leaves exactly this shape.
func TestQAPlanStaleStreakStopsAtATornLine(t *testing.T) {
	t.Parallel()
	a := staleApp(t, "")
	at := lpouiT.Add(-10 * time.Hour)
	seedReading(t, a, at, PlanUsage{{Name: "5h", Pct: 41}, {Name: "7d", Pct: 89}})
	// Oldest first, and the last line is torn: a timestamp and nothing else,
	// which is what an interrupted append leaves behind.
	body := "" +
		at.UTC().Format(time.RFC3339) + " dispatch ok\n" +
		at.Add(time.Hour).UTC().Format(time.RFC3339) + " dispatch 429 cooldown=5m\n" +
		at.Add(2*time.Hour).UTC().Format(time.RFC3339) + " dispatch 429 cooldown=5m\n" +
		at.Add(3 * time.Hour).UTC().Format(time.RFC3339)[:13] + "\n"
	if err := os.MkdirAll(a.StateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.StateDir, "plan-usage.log"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	st := a.PlanStaleness("qa", lpouiT, os.Stderr)
	if !st.Stale {
		t.Fatalf("a ten-hour-old reading under an armed guard is stale: %+v", st)
	}
	if st.Fails != 0 || st.Class != "" {
		t.Errorf("a torn tail line ends the streak: Fails=%d Class=%q, want 0 and \"\"", st.Fails, st.Class)
	}
	if !strings.HasSuffix(st.Line(), "no request has left this machine since") {
		t.Errorf("the line must not report failures no store backs:\n%s", st.Line())
	}
}

// A bracketed tail inside the SENTENCE is not a class.
//
// planLogClass reads the class out of logRead's `failed: <sentence> [401]`
// marker, and guards it with `!strings.ContainsAny(tok, " [")` so a
// sentence that happens to end in brackets cannot be mistaken for the
// marker — the same rule the whole file is built on, that the class comes
// off the type and never off the prose.
//
// MEASURED (verify ranger-base-qca6i): deleting that guard leaves every
// lpoui pin GREEN, because no fixture in them carries a bracketed sentence.
// With it deleted, the fixture below renders "3 consecutive plan window
// unavailable" — a class name invented from an error's prose, in the
// operator's face, which is exactly what PlanFailToken's type switch and
// TestQAPlanFailTokenReadsTheTypeNotTheProse exist to prevent one layer up.
func TestQAPlanStaleClassIgnoresABracketedSentence(t *testing.T) {
	t.Parallel()
	a := staleApp(t, "")
	at := lpouiT.Add(-10 * time.Hour)
	seedReading(t, a, at, PlanUsage{{Name: "5h", Pct: 41}, {Name: "7d", Pct: 89}})
	const prose = "failed: usage endpoint said [plan window unavailable]"
	seedReadLog(t, a, "ok", prose, prose, prose)

	st := a.PlanStaleness("qa", lpouiT, os.Stderr)
	if st.Fails != 3 {
		t.Fatalf("three failed reads is three: Fails=%d", st.Fails)
	}
	if st.Class != "" {
		t.Errorf("a bracketed SENTENCE is not the class marker: Class=%q, want \"\"", st.Class)
	}
	if !strings.HasSuffix(st.Line(), "3 consecutive failed reads") {
		t.Errorf("an unclassed failure is named as one:\n%s", st.Line())
	}
}

// A blind clock that reads backwards produces "0h", not "-2h".
//
// blindHours clamps a negative duration, because the key it builds is a
// pulse fingerprint the coordinator is handed (govern.go guardBlindRow) and
// `guard-blind:-2h:429` is a token nobody can act on. A clock step or a
// state dir copied off another box is how blindFor goes negative.
//
// MEASURED (verify ranger-base-qca6i): removing the clamp leaves every
// lpoui pin GREEN — TestQAGuardBlindKeyEscalatesHourlyNotPerTick's rows are
// all non-negative, and TestQAGuardBlindKeyDiffersAcrossTheHour compares two
// positive hours.
func TestQAGuardBlindKeyClampsABackwardsClock(t *testing.T) {
	t.Parallel()
	rl := &RateLimit{Status: "429 Too Many Requests"}
	for _, blind := range []time.Duration{-2 * time.Hour, -90 * time.Minute, -time.Second} {
		key, _ := guardBlindRow(blind, rl)
		// Equality, not a "no minus sign" check: the key's own name carries
		// two hyphens, so the only assertion that says what is meant is the
		// whole string.
		if key != "guard-blind:0h:429" {
			t.Errorf("blindFor=%s produced key %q, want guard-blind:0h:429 — a pulse fingerprint must never carry a negative hour", blind, key)
		}
	}
}
