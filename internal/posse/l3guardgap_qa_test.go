//go:build !posse_arm2 && !posse_arm3

package posse

// ranger-base-e78c4: the stale prepare-commit-msg line used to name four
// guards as unrealized for every verdict, including the one verdict where
// posse is holding both bodies. MEASURED cost, 2026-10-03
// (docs/notes.d/ranger-base-knux2.md): a config edit adding this instance's
// FIRST data_ceiling_patterns: block staled three repos' hooks at once; the
// render grew by 324 lines and lost none, so the beads visibility,
// constitution-path and shared-index guards were present and byte-identical
// in all three stale bodies. One name of four was true.
//
// The arm-1 tag is deliberate: this is a reporting pin on the line an
// operator reads at every launch, so it belongs in the default build (and
// so `make test-arm1` and a `./...` run both carry it).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// THE TABLE AND THE RENDER MUST AGREE. l3GuardGap decides a guard is absent
// from the installed body by looking for its banner, so a banner that no
// longer appears in the render would report its guard as lost on every
// stale slot on the box — the same defect, louder. Exactly once, too: a
// banner that appeared twice would be a sentinel that cannot say which arm
// it found.
func TestL3CommitGuardBannersAreInTheRender(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	cfg := filepath.Join(home, "config.yaml")
	if err := os.WriteFile(cfg, []byte(qaCeilingCfg), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: cfg}
	set := a.OpsPatternSet()
	if len(set.Ceiling) == 0 {
		t.Fatalf("fixture premise: the ceiling patterns must be accepted, got %+v/%v", set.Ceiling, set.CeilingRejected)
	}
	render := CommitGuardHook(VisibilityPublic, set)
	for _, g := range l3CommitGuards {
		if n := strings.Count(render, g.banner); n != 1 {
			t.Errorf("the %s guard's banner %q appears %d time(s) in the render, want 1 — l3GuardGap would misreport it", g.name, g.banner, n)
		}
	}

	// AND THE IN-FORCE HALF: the ceiling arm renders nothing at all for an
	// instance that configured no patterns (dataCeilingCheck's empty-list
	// return), which is why l3GuardGap names a guard on neither side when
	// this render does not carry it. Every other banner is unconditional.
	bare := CommitGuardHook(VisibilityPublic, (&App{ConfigPath: filepath.Join(home, "absent.yaml")}).OpsPatternSet())
	for _, g := range l3CommitGuards {
		got, want := strings.Contains(bare, g.banner), g.name != "data ceiling"
		if got != want {
			t.Errorf("with no ceiling configured, render carries the %s banner = %v, want %v", g.name, got, want)
		}
	}
}

// THE INCIDENT, reproduced: install the hook under a config with no data
// ceiling, add the instance's first ceiling block, probe. The slot is stale,
// and the line must name the one guard the installed body does not carry —
// not the three it does.
func TestStaleL3LineNamesOnlyTheGuardTheBodyLacks(t *testing.T) {
	t.Parallel()
	repo, _ := qaHookRepo(t)
	home := t.TempDir()
	cfg := filepath.Join(home, "config.yaml")
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents"), ConfigPath: cfg}
	if _, _, _, err := a.InstallCommitGuardHook(repo); err != nil {
		t.Fatal(err)
	}

	// The 2026-10-02 23:39:07 edit, in one line: the instance's first
	// data_ceiling_patterns: block. Nothing else about the render moves.
	if err := os.WriteFile(cfg, []byte(qaCeilingCfg), 0o644); err != nil {
		t.Fatal(err)
	}

	p := a.probeL3Hooks(repo, false)
	if p.CommitGuardVerdict != l3Stale {
		t.Fatalf("a hook rendered before the ceiling block is l3Stale, got verdict=%d line=%q", p.CommitGuardVerdict, p.CommitGuardDegraded)
	}
	line := p.CommitGuardDegraded
	if strings.Contains(line, "\n") {
		t.Errorf("Degraded is ONE LINE (yamlflat.go, ranger-base-ujdg):\n%s", line)
	}
	if !strings.Contains(line, "the data ceiling guard is not realized") {
		t.Errorf("the stale line does not name the guard the body actually lacks:\n%s", line)
	}
	for _, kept := range []string{"beads visibility", "constitution-path", "shared-index", "NOTES.md"} {
		if !strings.Contains(line, kept) {
			t.Errorf("the stale line says nothing about the %s guard, which the stale body carries:\n%s", kept, line)
		}
	}
	if !strings.Contains(line, "present in the installed body") {
		t.Errorf("the stale line does not say the other guards are present:\n%s", line)
	}
	// The whole finding: the old line's four names, in one breath, on a body
	// that had lost one of them.
	if strings.Contains(line, commitGuardWorstCase) {
		t.Errorf("the stale line still prints the static worst case:\n%s", line)
	}
	// Belt: the three present guards must not be reported as lost. The
	// clause order is "missing ... not realized; present ...", so anything
	// present appearing before the semicolon is the old over-claim.
	lost, _, _ := strings.Cut(line, "; the ")
	for _, kept := range []string{"beads visibility", "constitution-path", "shared-index", "NOTES.md"} {
		if strings.Contains(lost, kept) {
			t.Errorf("the %s guard is named as lost by a line that is holding the body that carries it:\n%s", kept, line)
		}
	}
	t.Logf("derived line: %s", line)
}

// WHERE POSSE CANNOT TELL, THE WORST CASE STAYS. A foreign hook, an empty
// slot and a session dir that is not this launch's are all bodies posse has
// no render of its own to compare — and a behavior failure under held
// identity is not about the body at all.
func TestL3ConsequenceKeepsTheWorstCaseWhereItCannotTell(t *testing.T) {
	t.Parallel()
	render := CommitGuardHook(VisibilityPublic, OpsPatternSet{})
	for _, v := range []l3Verdict{l3Held, l3Uninstalled, l3Foreign, l3RedirectMismatch} {
		if got := commitGuardConsequence(filepath.Join(t.TempDir(), "absent"), render, v); got != commitGuardWorstCase {
			t.Errorf("verdict %d must keep the static worst case, got %q", v, got)
		}
	}

	// l3MemberGone IS derived, with an empty installed body: the dispatcher
	// holds the slot and the member file does not exist, so every guard the
	// render carries is genuinely absent. What the derivation buys there is
	// the name it does NOT print — this instance renders no ceiling arm, so
	// the ceiling is not among the guards it lost.
	got := commitGuardConsequence(filepath.Join(t.TempDir(), "absent"), render, l3MemberGone)
	if strings.Contains(got, "data ceiling") {
		t.Errorf("a member-gone slot on an instance with no ceiling configured must not mourn one: %q", got)
	}
	for _, want := range []string{"beads visibility", "constitution-path", "shared-index", "NOTES.md", "not realized"} {
		if !strings.Contains(got, want) {
			t.Errorf("a member-gone slot loses every guard the render carries, and %q is missing from %q", want, got)
		}
	}
}

// A body that carries every banner and still fails identity by bytes: the
// line must say the drift is elsewhere rather than claim a guard was lost,
// and must not claim the present ones are CURRENT — identity failed, so a
// guard that is there may be an older render of itself.
func TestL3GuardGapSaysWhenNoNamedGuardIsMissing(t *testing.T) {
	t.Parallel()
	render := CommitGuardHook(VisibilityPublic, OpsPatternSet{})
	got := l3GuardGap(render+"\n# one line of drift\n", render)
	for _, want := range []string{"no named guard is missing", "this render's or an older one", "the drift is elsewhere"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q missing from the no-gap clause: %q", want, got)
		}
	}
	if strings.Contains(got, "not realized") {
		t.Errorf("a body carrying every banner has lost no named guard: %q", got)
	}

	// A marker-bearing pass-through — posse's by marker, nothing of the wall
	// in it — is the other end: every in-force guard, named as lost, which
	// is what the static line always said and the only body it fitted.
	if got := l3GuardGap("#!/bin/sh\n"+sharedIndexMarker+"\nexit 0\n", render); !strings.HasSuffix(got, "not realized") || strings.Contains(got, "present in the installed body") {
		t.Errorf("a body with no guard in it loses all of them: %q", got)
	}
}
