package posse

// The REAL-line audit (ranger-base-urnj, cut from the 2026-08-27 fleet-freeze
// RCA — see ranger-base-ernt and [[bash-tool-wedged-in-posse-worktree]]).
//
// ranger-base-f0ay makes writeGateShell REFUSE to render a wrapper whose
// REAL is another gate wrapper — the chain that closed into a cycle
// (monica↔jian-yang) and wedged every Bash spawn in the fleet for ~2h. That
// is prevention, at render time, in the binary doing the rendering. This is
// detection, standing: a wrapper written by a binary older than f0ay, or one
// a future bug re-introduces, sits on disk as a live wedge waiting for its
// next spawn — and until this file existed, nothing ever looked. The
// healthy state is one grep, zero hits:
//
//	grep -l '^REAL=.*state/gates' $RHQ_HOME/state/gates/*/shell/*
//
// which the runbook (docs/runbooks/shell-wedge.md) keeps as its literal step
// 1, because a shell is exactly what a wedged session does not have. This
// file is the same check run in Go, from the one place that already reads
// every persona's gates dir each pass: the dispatcher.
//
// It is a BELT, not a fix, and it says so loudly rather than acting on it:
// the pass is never aborted, because a chained wrapper does not corrupt
// anything by sitting there — a spawn that enters it merely re-prepends
// each hop's PATH guard onto its own -c string until it cycles (the E2BIG
// wedge), and that only happens to a spawn that actually goes through the
// bad wrapper. Aborting the pass would stop work that was never going to hit
// the chain while doing nothing about the wrapper itself. Naming it the
// moment it exists — not hours into a wedge with a dead shell as the only
// diagnostic instrument — is the entire value; repair is a re-render
// (`posse gates` / the persona's next launch), same as the runbook's step 1
// says.
//
// It costs no fork, the same discipline the load guard holds itself to
// (loadguard.go): a glob and a handful of file reads, so it is exactly as
// safe to run on a saturated box as the reading that measures the box.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// realLineRe reads a rendered gate shell's REAL='<path>' line — the same
// pattern TestGateShellNeverChainsToAnotherWrapper uses to pull it out of a
// wrapper it just rendered (gates_test.go). shQuote never emits a literal
// single quote inside the quoted value (it escapes one out of the string
// instead), so a bare `.*` up to the closing quote is exact, not a
// heuristic.
var realLineRe = regexp.MustCompile(`(?m)^REAL='(.*)'$`)

// ChainedGateWrapper is one wrapper the audit found whose REAL points at
// another gate wrapper instead of a real shell.
type ChainedGateWrapper struct {
	Persona string // the gates/<persona> this wrapper belongs to
	Path    string // the wrapper file itself
	Real    string // its REAL= target — also a gate wrapper, which is the defect
}

// ChainedGateWrappers walks every rendered gate shell under
// StateDir/gates/<persona>/shell/<base> and returns the ones that fail the
// invariant writeGateShell enforces at render time: REAL must resolve
// outside every gates dir (ADR 0009 §1; ranger-base-f0ay). A wrapper is
// skipped, not failed, when it cannot be read or carries no REAL= line —
// this audit does not know the shape every future wrapper will take, only
// how to recognize the one defect that wedged the fleet, and a shape it does
// not recognize is silence, not a false alarm.
func (a *App) ChainedGateWrappers() ([]ChainedGateWrapper, error) {
	matches, err := filepath.Glob(filepath.Join(a.StateDir, "gates", "*", "shell", "*"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	var bad []ChainedGateWrapper
	for _, w := range matches {
		st, err := os.Stat(w)
		if err != nil || st.IsDir() {
			continue
		}
		b, err := os.ReadFile(w)
		if err != nil {
			continue
		}
		m := realLineRe.FindSubmatch(b)
		if m == nil {
			continue
		}
		real := string(m[1])
		if !isGateWrapper(real) {
			continue
		}
		// gates/<persona>/shell/<base> — persona is two directories up from
		// the wrapper file, the same layout GatesDir renders it into.
		persona := filepath.Base(filepath.Dir(filepath.Dir(w)))
		bad = append(bad, ChainedGateWrapper{Persona: persona, Path: w, Real: real})
	}
	return bad, nil
}

// RealAuditWitness runs the REAL-line audit and returns a loud, one-line
// report naming every chained wrapper found — or "" on a clean audit, so a
// caller can append it unconditionally. It never gates anything: see the
// file doc for why a hit is named, not acted on. A glob or read failure is
// named on errw and answered as clean — the audit must not itself be able to
// turn a healthy pass into a reported incident.
func (a *App) RealAuditWitness(errw io.Writer) string {
	bad, err := a.ChainedGateWrappers()
	if err != nil {
		fmt.Fprintf(errw, "gate audit: %v — REAL-line audit not run this pass\n", err)
		return ""
	}
	if len(bad) == 0 {
		return ""
	}
	shown := make([]string, 0, len(bad))
	for _, w := range bad {
		shown = append(shown, fmt.Sprintf("%s (%s → REAL=%s)", w.Persona, w.Path, w.Real))
	}
	return fmt.Sprintf("gate audit: %d chained gate wrapper(s) — REAL points at another gate wrapper, not a real shell (ADR 0009 §1; ranger-base-f0ay): %s — a spawn through one of these merely stalls until it cycles, nothing is corrupted; repair with a re-render (posse gates / the persona's next launch)",
		len(bad), strings.Join(shown, ", "))
}
