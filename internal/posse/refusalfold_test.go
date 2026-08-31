package posse

// Hermetic tests for the fold itself (ADR 0025 §4, refusalfold.go). No
// container, no engine — just the spool file, the cursor file and the
// canonical log, exercised as plain files: the live QA
// (cageinnerliveqa_test.go's TestQALiveCageEscapeAttemptsOnAWritableRepo)
// proves the same cursor logic against a real cage; this proves the logic
// itself, including the shapes a live run cannot cheaply set up (a partial
// trailing line, a same-size rewrite).

import (
	"os"
	"strings"
	"testing"
)

func TestFoldNoSpoolIsANoOpNotAnError(t *testing.T) {
	a := cageApp(t)
	if err := a.FoldRefusalsSpool("p", "s1"); err != nil {
		t.Fatalf("no spool ever written: %v", err)
	}
	if _, err := os.Stat(a.RefusalsLogPath("p")); !os.IsNotExist(err) {
		t.Errorf("a fold with nothing to fold must not create the canonical log: %v", err)
	}
}

func TestFoldMovesCompleteLinesAndTagsTheSession(t *testing.T) {
	a := cageApp(t)
	if _, err := a.EnsureCageSpool("p", "s1"); err != nil {
		t.Fatal(err)
	}
	spool := a.CageSpoolPath("p", "s1")
	if err := os.WriteFile(spool, []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.FoldRefusalsSpool("p", "s1"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(a.RefusalsLogPath("p"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if got != "session=s1 one\nsession=s1 two\n" {
		t.Errorf("folded lines must be tagged with their session, in order:\n%q", got)
	}

	// Idempotent: a second fold over an unchanged spool appends nothing
	// (ADR 0025 §4 verification 3).
	if err := a.FoldRefusalsSpool("p", "s1"); err != nil {
		t.Fatal(err)
	}
	b2, err := os.ReadFile(a.RefusalsLogPath("p"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b2) != got {
		t.Errorf("a re-fold of an unchanged spool must append zero lines:\nbefore: %q\nafter:  %q", got, b2)
	}
}

func TestFoldLeavesAPartialTrailingLineForTheNextFold(t *testing.T) {
	a := cageApp(t)
	if _, err := a.EnsureCageSpool("p", "s1"); err != nil {
		t.Fatal(err)
	}
	spool := a.CageSpoolPath("p", "s1")
	// A shim's append is mid-write: one complete line, one that never got
	// its trailing newline (yet).
	if err := os.WriteFile(spool, []byte("done\nstill-writ"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.FoldRefusalsSpool("p", "s1"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(a.RefusalsLogPath("p"))
	if got := string(b); got != "session=s1 done\n" {
		t.Errorf("only the complete line may fold; the partial one is not this fold's:\n%q", got)
	}

	// The write finishes; the next fold picks up exactly the rest.
	if err := os.WriteFile(spool, []byte("done\nstill-writing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.FoldRefusalsSpool("p", "s1"); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(a.RefusalsLogPath("p"))
	if got := string(b); got != "session=s1 done\nsession=s1 still-writing\n" {
		t.Errorf("the finished line must fold on the next pass, and only once:\n%q", got)
	}
}

func TestFoldDetectsTruncationByOffset(t *testing.T) {
	a := cageApp(t)
	if _, err := a.EnsureCageSpool("p", "s1"); err != nil {
		t.Fatal(err)
	}
	spool := a.CageSpoolPath("p", "s1")
	if err := os.WriteFile(spool, []byte("refused one\nrefused two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.FoldRefusalsSpool("p", "s1"); err != nil {
		t.Fatal(err)
	}

	// `: > refusals.log` from inside the cage — the escape ADR 0025 §4 is
	// written around.
	if err := os.WriteFile(spool, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.FoldRefusalsSpool("p", "s1"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(a.RefusalsLogPath("p"))
	if err != nil {
		t.Fatal(err)
	}
	log := string(b)
	if !strings.Contains(log, "refused one") || !strings.Contains(log, "refused two") {
		t.Errorf("the canonical log must still hold what was folded BEFORE the truncation — it was never mounted, so nothing in it can be erased from inside:\n%s", log)
	}
	if !strings.Contains(log, "refusals spool tampered [fold] session=s1") {
		t.Errorf("a spool shorter than its own cursor must fold as tampered, naming the session:\n%s", log)
	}
}

func TestFoldDetectsATruncateAndRefillToTheSameSizeByHash(t *testing.T) {
	a := cageApp(t)
	if _, err := a.EnsureCageSpool("p", "s1"); err != nil {
		t.Fatal(err)
	}
	spool := a.CageSpoolPath("p", "s1")
	original := "refused one\n"
	if err := os.WriteFile(spool, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.FoldRefusalsSpool("p", "s1"); err != nil {
		t.Fatal(err)
	}

	// Same length as the original, different bytes: the offset comparison
	// alone would read this as "nothing new to fold" and miss it entirely —
	// the hash is what has to catch it (ADR 0025 §4 verification 3).
	rewritten := "REPLACED   \n"
	if len(rewritten) != len(original) {
		t.Fatalf("test fixture bug: rewritten must be the same length as original (%d vs %d)", len(rewritten), len(original))
	}
	if err := os.WriteFile(spool, []byte(rewritten), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.FoldRefusalsSpool("p", "s1"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(a.RefusalsLogPath("p"))
	if err != nil {
		t.Fatal(err)
	}
	log := string(b)
	if !strings.Contains(log, "refusals spool tampered [fold] session=s1") {
		t.Errorf("a same-size rewrite must still fold as tampered — the hash, not the offset, is what catches it:\n%s", log)
	}
	if !strings.Contains(log, "session=s1 REPLACED") {
		t.Errorf("the re-fold from zero must still carry the rewritten content, so the tamper is legible next to what replaced it:\n%s", log)
	}
}

func TestFoldKeepsTwoSessionsOfOnePersonaSeparate(t *testing.T) {
	a := cageApp(t)
	for _, s := range []string{"s1", "s2"} {
		if _, err := a.EnsureCageSpool("p", s); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(a.CageSpoolPath("p", s), []byte("refused in "+s+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.FoldRefusalsSpool("p", "s1"); err != nil {
		t.Fatal(err)
	}
	if err := a.FoldRefusalsSpool("p", "s2"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(a.RefusalsLogPath("p"))
	if err != nil {
		t.Fatal(err)
	}
	log := string(b)
	for _, want := range []string{"session=s1 refused in s1", "session=s2 refused in s2"} {
		if !strings.Contains(log, want) {
			t.Errorf("both sessions' lines must land in the one canonical log, each tagged with its own session:\n%s\nwant: %s", log, want)
		}
	}
	// Truncating s1's spool must never be readable as tamper on s2's cursor —
	// the cursors, like the spools, are per session.
	if err := os.WriteFile(a.CageSpoolPath("p", "s1"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.FoldRefusalsSpool("p", "s2"); err != nil {
		t.Fatal(err)
	}
	b2, _ := os.ReadFile(a.RefusalsLogPath("p"))
	if strings.Contains(string(b2), "session=s2") && strings.Count(string(b2), "tampered") > 0 {
		t.Errorf("s1's truncation must not be attributed to s2's fold:\n%s", b2)
	}
}
