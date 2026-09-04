package main

// QA pin for ranger-base-w2uoe's close, written verifying it under
// ranger-base-aqu29.
//
// The close is sound and its four pins hold: eight mutants against the frame
// envelope, the dedupe and paint's reset were each killed by at least one of
// them, re-run here. This is the ninth, and it survives all four —
//
//	footerLines' modeConfirm arm, hand-rolled instead of handed to the col:
//	    return []string{"", "", aRed + truncCells(ask, w)}
//
// — because TestCockpitNoLineEndsWithAnOpenColour sweeps modeNormal, modePeek
// and modePrompt, and there are FOUR cockpitModes. Nothing renders
// modeConfirm, so nothing measures the coupling that bead named as the one
// its own fix creates: ESC[K erases with the CURRENT attributes, so a line
// ending on an open colour paints the rest of its row in that colour, a bar
// to the right edge that redraws on every frame. ESC[2J never showed it,
// because it ran before any of the frame's own SGR.
//
// modeConfirm is clean at HEAD — every column goes through paint, which
// appends aRst — so this is the pin's reach and not a live defect. It is
// still the arm worth having: it is the one a destructive key (x kill, u
// unclaim) puts on the glass, the one coloured aRed rather than the aDim and
// aBold of the swept modes, and the mutant above is what a future edit to it
// looks like.
//
// The second test is why this file does not rot: modesDeclared reads
// cockpit.go's own cockpitMode block, so a fifth mode reds here until a sweep
// names it. Its own failing arm is below it — a source fixture carrying a
// mode no sweep visits — because a source-reading pin cannot be mutation-
// checked through `go test -overlay`: the overlay reaches the compiler, not
// os.ReadFile.

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

var confirmSGR = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// Every coloured line the confirm footer puts on the glass closes what it
// opened, at every width where the ask truncates, on both arms of the mode.
func TestCockpitConfirmModeLineNeverEndsOnAnOpenColour(t *testing.T) {
	for _, arm := range []struct {
		name   string
		kind   confirmKind
		target confirmTarget
	}{
		{"kill", confirmKill, confirmTarget{name: "developer-ranger-base-w2uoe"}},
		{"unclaim", confirmUnclaim, confirmTarget{dir: "/w", id: "ranger-base-w2uoe"}},
	} {
		t.Run(arm.name, func(t *testing.T) {
			c := qaFixture()
			c.mode = modeConfirm
			c.confirm = arm.kind
			c.target = arm.target
			coloured := 0
			for w := 12; w <= 160; w++ {
				for i, ln := range c.renderLines(w, 24) {
					sgr := confirmSGR.FindAllString(ln, -1)
					if len(sgr) == 0 {
						continue
					}
					coloured++
					if last := sgr[len(sgr)-1]; last != aRst {
						t.Fatalf("w=%d line %d ends on %q, so ESC[K paints its tail in that colour: %q",
							w, i, last, ln)
					}
				}
			}
			if coloured < 100 {
				t.Fatalf("only %d coloured lines swept — the fixture stopped colouring anything and this pin measures nothing", coloured)
			}
		})
	}
}

// modesDeclared is the cockpitMode identifiers a source declares, read from
// the const block rather than listed here, so the list cannot drift from it.
func modesDeclared(t *testing.T, src string) []string {
	t.Helper()
	i := strings.Index(src, "type cockpitMode int")
	if i < 0 {
		t.Fatal("the source no longer declares cockpitMode — re-aim this pin")
	}
	block := src[i:]
	open, end := strings.Index(block, "const ("), strings.Index(block, "\n)")
	if open < 0 || end < 0 || open > end {
		t.Fatal("cockpitMode's const block is not where this pin looks — re-aim it")
	}
	var out []string
	for _, m := range regexp.MustCompile(`(?m)^\s*(mode[A-Za-z]+)`).FindAllStringSubmatch(block[open:end], -1) {
		out = append(out, m[1])
	}
	return out
}

// unswept is every declared mode that no sweep source renders.
func unswept(modes []string, sweeps string) []string {
	var out []string
	for _, m := range modes {
		if !strings.Contains(sweeps, m) {
			out = append(out, m)
		}
	}
	return out
}

// The open-colour sweep must name every mode the cockpit can render. Add a
// fifth cockpitMode and this goes red until a sweep visits it.
func TestCockpitOpenColourSweepCoversEveryMode(t *testing.T) {
	src, err := os.ReadFile("cockpit.go")
	if err != nil {
		t.Fatalf("read cockpit.go: %v", err)
	}
	modes := modesDeclared(t, string(src))
	if len(modes) < 4 {
		t.Fatalf("found %d cockpitMode constants (%v), want at least the four that exist — the pin is not reading the block", len(modes), modes)
	}
	var sweeps strings.Builder
	for _, f := range []string{"cockpitflicker_test.go", "cockpitconfirmcolour_qa_test.go"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		sweeps.Write(b)
	}
	if miss := unswept(modes, sweeps.String()); len(miss) > 0 {
		t.Errorf("%v: a cockpitMode no open-colour sweep renders — a line of that mode ending "+
			"on an open colour would paint its row's tail in that colour and no pin would say "+
			"so (ranger-base-w2uoe)", miss)
	}
}

// ...and the arm that keeps the test above from being a mute. The overlay a
// mutation check would use reaches the compiler, not os.ReadFile, so the
// failing case is fed in as source here instead.
func TestCockpitModeSweepReaderSeesAnUnsweptMode(t *testing.T) {
	const src = `type cockpitMode int

const (
	modeNormal  cockpitMode = iota
	modePrompt
	modeConfirm
	modePeek
	modeGraph // a fifth mode, added without a sweep
)
`
	modes := modesDeclared(t, src)
	if len(modes) != 5 {
		t.Fatalf("read %v from the fixture, want five — the reader is not seeing the block", modes)
	}
	sweeps := "modeNormal modePrompt modeConfirm modePeek"
	miss := unswept(modes, sweeps)
	if len(miss) != 1 || miss[0] != "modeGraph" {
		t.Fatalf("unswept(%v) = %v, want [modeGraph] — the check cannot report the mode it exists to report", modes, miss)
	}
}
