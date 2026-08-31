package posse

// PAUSE — the write half of state/pause.yaml, the authority allowed to
// write it, and the line every declining pass prints.
//
// Design: ADR 0029 §3 (docs/adr/0029-governance-surface.md, restating the
// archive's governance-surface ADR; bead rangerhq-a2g6, from archive bead
// rangerhq-e37c). govern.go owns the READ half (PausePath, Pause,
// ReadPause) because a pause is G8, a governance row; this file is what
// puts the row there, and dispatch.go is where it stops a pass.
//
// **Three verbs, kept distinct** (§3's own words):
//
//   SKIP   this pass only, keep polling — the plan guard, the blind window,
//          Dial E, the busy key, the load guard. Automatic, self-healing,
//          pure mechanism. NO CONDITION MAY AUTO-PAUSE: latching a transient
//          reading into a durable stop that needs a human to clear trades a
//          self-healing skip for a flapping meter parking the shop
//          overnight. Nothing in this file is called from a predicate.
//   PAUSE  stop dispatching until told otherwise. A human speech act, and
//          the one legitimately NEW store the design adds: pause intent is a
//          new fact with a single writer, not a copy of another store's.
//   STOP   the loop killed, autostart disarmed, `make uninstall`. The
//          operator's promotion key wearing its off switch, and not this
//          file's business.
//
// **Pause stops spend, not oversight.** The gate sits at the fire loop's
// entry (Dispatcher.Run) and on the cockpit's single-bead launcher
// (LaunchBead) — the two places a bead becomes a running agent. The pulse
// goroutine is deliberately untouched: a paused shop still escalates
// blocked sessions and aging questions, which is what distinguishes a pause
// from killing the loop and why the coordinator reaches for it instead of
// `kill`. By the same line, a hand-run `posse new`/`relaunch`/`recipe` is
// the operator's own hand on a session, not dispatch spending, and is not
// gated here.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PauseOperator is what `by:` says when the pause came from a shell with no
// RHQ_PERSONA in it. Same reading MarkCrewOnOperatorPrompt makes: a
// persona's session carries the variable, a person's does not.
const PauseOperator = "operator"

// PauseActor answers who this process may pause AS, or refuses.
//
// §3: "Who may pause: the operator, and the coordinator — strictly gentler
// than the authority the coordinator's PID already grants." Everyone else
// gets a refusal naming the two, because a stop the rest of the fleet can
// write is a stop any one of them can sit in.
func PauseActor(a *App) (string, error) {
	persona := os.Getenv(EnvPersona)
	if persona == "" {
		return PauseOperator, nil
	}
	coord := a.Coordinator()
	if coord != "" && persona == coord {
		return persona, nil
	}
	if coord == "" {
		return "", Die("refused: %s may not pause the shop — with no coordinator: in %s, pausing is the operator's alone (ADR 0029 §3)",
			persona, AbbrevHome(a.ConfigPath))
	}
	return "", Die("refused: %s may not pause the shop — that is the operator's and %s's (ADR 0029 §3)", persona, coord)
}

// WritePause records the stop. The why is MANDATORY — the file shape is
// what makes "pauses with a recorded why: 100%" a metric rather than a
// hope — and `at` is handed in rather than read from a clock here, so every
// caller (and every test) says which now it means.
//
// It returns the pause AS STORED, read back through the same flat-YAML
// reader every pass uses. That round trip is the point: the reader takes a
// line, and a why carrying " #" (a comment, to it) or a newline would come
// back shorter than what was typed. Rather than invent an escaping dialect
// for one field, the writer flattens what it can (whitespace, which also
// closes the "why: ...\nby: someone-else" injection) and SAYS SO when the
// stored form still differs from the typed one. A stop must never fail over
// its own formatting, so this is a warning and never a refusal.
func WritePause(a *App, by, why string, at time.Time, errw io.Writer) (Pause, error) {
	by, why = flatScalar(by), flatScalar(why)
	if why == "" {
		return Pause{}, Die(`posse pause needs a reason: posse pause "<why>" — every declining pass prints it`)
	}
	path := PausePath(a)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Pause{}, err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "by: %s\n", by)
	fmt.Fprintf(&b, "at: %s\n", at.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "why: %s\n", why)
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return Pause{}, err
	}
	p := ReadPause(path)
	if p.Why != why && errw != nil {
		fmt.Fprintf(errw, "warning: the reason is stored as %q — that is what %s gives back and what every declining pass will print\n",
			p.Why, AbbrevHome(path))
	}
	return p, nil
}

// ClearPause lifts the stop and returns what it lifted, so `posse resume`
// can name the pause it ended. A file that is not there is not an error:
// resume is idempotent, the way a stop's off switch has to be.
func ClearPause(a *App) (Pause, error) {
	path := PausePath(a)
	p := ReadPause(path)
	if !p.Present {
		return p, nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return p, err
	}
	return p, nil
}

// PauseLine is the one line a declining pass prints — "paused — by X, at Y,
// why: Z". Same words G8 renders in `posse status` and the cockpit, off the
// same clause builder, because a pause named two ways is a pause somebody
// has to correlate.
func PauseLine(p Pause) string { return "paused" + PauseClause(p) }

// flatScalar renders a typed string as something the flat-YAML reader gives
// back on one line: every whitespace run (newlines included) becomes one
// space, and the ends are trimmed. What it cannot fix — " #", which the
// reader takes as a comment wherever it appears — WritePause reports.
func flatScalar(s string) string { return strings.Join(strings.Fields(s), " ") }
