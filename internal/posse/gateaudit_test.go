package posse

// The REAL-line audit (ranger-base-urnj) — see gateaudit.go's file doc for
// the incident this restates as a standing check. These tests write raw
// wrapper files rather than going through RenderGates/writeGateShell on
// purpose: writeGateShell already REFUSES to write the defect
// (ranger-base-f0ay), so the only way to exercise an audit that finds one is
// to plant a wrapper the way a pre-f0ay binary, or a hand edit, would have
// left it on disk.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// plantWrapper writes a minimal gate shell at StateDir/gates/<persona>/shell/<base>
// with the given REAL= target, the shape ChainedGateWrappers reads.
func plantWrapper(t *testing.T, a *App, persona, base, real string) string {
	t.Helper()
	dir := filepath.Join(a.GatesDir(persona), "shell")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, base)
	body := "#!/bin/sh\n# posse gate shell for " + persona + " — rendered at launch from the PID; do not edit (ADR 0009).\nREAL=" + shQuote(real) + "\nexec \"$REAL\" \"$@\"\n"
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestChainedGateWrappersCleanAuditIsEmpty(t *testing.T) {
	a := NewAppAt(t.TempDir())
	// No gates dir at all yet.
	bad, err := a.ChainedGateWrappers()
	if err != nil || len(bad) != 0 {
		t.Fatalf("an empty box must audit clean: %v, %v", bad, err)
	}
	// An honest wrapper, REAL pointing at a real shell outside every gates dir.
	plantWrapper(t, a, "coordinator", "zsh", "/bin/zsh")
	bad, err = a.ChainedGateWrappers()
	if err != nil || len(bad) != 0 {
		t.Fatalf("an honest REAL must not be flagged: %v, %v", bad, err)
	}
	if w := a.RealAuditWitness(os.Stderr); w != "" {
		t.Errorf("a clean audit must produce no witness, got %q", w)
	}
}

// The defect itself (ranger-base-f0ay): a wrapper whose REAL is another gate
// wrapper. The audit must name persona, path and REAL — not just say "bad".
func TestChainedGateWrappersFindsAChain(t *testing.T) {
	a := NewAppAt(t.TempDir())
	alpha := plantWrapper(t, a, "alpha", "zsh", filepath.Join(a.GatesDir("bravo"), "shell", "zsh"))
	bravo := plantWrapper(t, a, "bravo", "zsh", filepath.Join(a.GatesDir("alpha"), "shell", "zsh"))
	// A third, honest wrapper must not be swept up as a false positive.
	plantWrapper(t, a, "coordinator", "zsh", "/bin/zsh")

	bad, err := a.ChainedGateWrappers()
	if err != nil {
		t.Fatal(err)
	}
	if len(bad) != 2 {
		t.Fatalf("want both halves of the cycle flagged and nothing else, got %+v", bad)
	}
	byPersona := map[string]ChainedGateWrapper{}
	for _, w := range bad {
		byPersona[w.Persona] = w
	}
	if w, ok := byPersona["alpha"]; !ok || w.Path != alpha || w.Real != bravo {
		t.Errorf("alpha's row = %+v, want Path=%s Real=%s", w, alpha, bravo)
	}
	if w, ok := byPersona["bravo"]; !ok || w.Path != bravo || w.Real != alpha {
		t.Errorf("bravo's row = %+v, want Path=%s Real=%s", w, bravo, alpha)
	}

	witness := a.RealAuditWitness(os.Stderr)
	for _, want := range []string{"2 chained gate wrapper", "alpha", "bravo", alpha, bravo, "ranger-base-f0ay"} {
		if !strings.Contains(witness, want) {
			t.Errorf("the witness must name %q:\n%s", want, witness)
		}
	}
}

// A wrapper with no REAL= line, or one that is not a wrapper's shape at all,
// must be skipped rather than mistaken for a hit — the audit only knows the
// one defect, not every future wrapper shape.
func TestChainedGateWrappersSkipsWhatItDoesNotRecognize(t *testing.T) {
	a := NewAppAt(t.TempDir())
	dir := filepath.Join(a.GatesDir("coordinator"), "shell")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "zsh"), []byte("#!/bin/sh\necho no real line here\n"), 0o755)
	// A subdirectory under shell/ (should never happen, but Stat-and-skip
	// rather than error).
	os.MkdirAll(filepath.Join(dir, "nested"), 0o755)

	bad, err := a.ChainedGateWrappers()
	if err != nil || len(bad) != 0 {
		t.Fatalf("an unrecognized shape must not be flagged: %v, %v", bad, err)
	}
}

// The witness must never gate the pass — see dispatch.go's call site, which
// logs and continues whatever ChainedGateWrappers returns.
func TestDispatchLogsTheAuditButDoesNotDecline(t *testing.T) {
	b, fake := newTestBackend(t)
	d := newTestDispatcher(t, b)
	pauseRepo(t, b, fake)
	plantWrapper(t, b.App, "alpha", "zsh", filepath.Join(b.App.GatesDir("bravo"), "shell", "zsh"))
	plantWrapper(t, b.App, "bravo", "zsh", filepath.Join(b.App.GatesDir("alpha"), "shell", "zsh"))

	n, err := d.Run("", "", 0)
	if err != nil {
		t.Fatalf("a found chain must not fail the pass: %v", err)
	}
	if n != 1 {
		t.Fatalf("a found chain must not stop the pass from dispatching: n=%d\n%s", n, dispatcherOut(d))
	}
	out := dispatcherOut(d)
	for _, want := range []string{"gate audit", "alpha", "bravo"} {
		if !strings.Contains(out, want) {
			t.Errorf("the pass must log the audit hit loudly, want %q:\n%s", want, out)
		}
	}
}
