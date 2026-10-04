//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-41zyo — the metered credential class is refused
// where it is ADMITTED, not only where it is written.
//
// The defect these hold is a check that existed in prose and in one code
// path, and was absent from the two that decide whether a session starts.
// MEASURED 2026-10-04 before the fix, on this tree: a runtime yaml carrying
// `cage_cred: ANTHROPIC_API_KEY` resolved CageCredential to that name and
// CheckCageCredential(rt, []string{"ANTHROPIC_API_KEY"}) returned nil, so a
// caged launch on it was admitted — while `posse runtime check`'s cage_cred
// row said "a METERED api key is not accepted as this credential" and
// `posse refresh` refused the same name outright. Three surfaces, one
// claim, and the two that gate a launch did not make it.
//
// Every arm here is about a NEGATIVE, because the failure is silent: an
// admitted launch on a metered key looks exactly like an admitted launch,
// and the first signal is the bill.
//
// The arms:
//
//  1. the predicate itself, and that it is a set of exact NAMES — another
//     provider's decided credential is not refused by accident, and the
//     ruling's own name is not matched by a fold or a substring.
//  2. the container precondition (CheckCageCredential) refuses a metered
//     declared credential even when that name IS in the session's env sets,
//     which is the exact shape that was admitted.
//  3. the shim precondition (CheckCredGate) refuses it with no shim in play
//     at all — the money line does not route through ADR 0042's collision.
//  4. the write path still refuses it, by name and by shape, and now reads
//     the same one definition the two above read. BOTH halves are exercised
//     since ranger-base-ghcx3 — the shape through checkSessionToken, the
//     name through refreshSession itself.
//  5. the shipped runtimes are unaffected: claude's decided credential
//     still admits, and codex/grok still refuse for being undecided, which
//     is a different refusal with a different next move.
//  6. `posse runtime check`'s cage_cred row names the refusal rather than
//     printing a metered name as this runtime's credential.
//
// MUTATION-CHECKED 2026-10-04: dropping the CheckMeteredSessionCredential
// call from cage.go reds arm 2 alone; dropping it from gates.go reds arm 3
// alone; making IsMeteredCredentialName fold case or match an `API_KEY`
// substring reds arm 1 alone; reverting the runtimecheck row reds arm 6
// alone; and, since ranger-base-ghcx3, disabling the
// IsMeteredCredentialName guard in refreshSession reds arm 4 alone — which
// before that bead reddened nothing in the tree.

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// meteredRuntime writes a template-only runtime whose cage_cred is cred and
// loads it, which is the one route a metered name has into this shop that
// `posse refresh` never sees: a line in promoted config.
func meteredRuntime(t *testing.T, a *App, name, cred string) *Runtime {
	t.Helper()
	if err := os.MkdirAll(a.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(a.RuntimesDir(), name+".yaml")
	if err := os.WriteFile(p, []byte("command: "+name+" {file}\ncage_cred: "+cred+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rt, err := a.LoadRuntime(name)
	if err != nil {
		t.Fatal(err)
	}
	return rt
}

// Arm 1 — the predicate is a set of exact names.
func TestQAMeteredCredentialNameIsAnExactSet(t *testing.T) {
	t.Parallel()
	if !IsMeteredCredentialName("ANTHROPIC_API_KEY") {
		t.Error("the name the operator's ruling refused is not in the set")
	}
	if !IsMeteredCredentialName("  ANTHROPIC_API_KEY  ") {
		t.Error("a yaml line with a trailing space is a typo, not a second credential")
	}
	// Not a fold and not a substring. Both would be a claim about env
	// naming that unix does not share, and the substring form refuses
	// another provider's decided session credential.
	for _, n := range []string{
		"anthropic_api_key",
		"Anthropic_Api_Key",
		"MY_ANTHROPIC_API_KEY_BACKUP",
		"ANTHROPIC_API_KEY_OLD",
		BobCageCred,
		"CLAUDE_CODE_OAUTH_TOKEN",
		"OWN_TOKEN",
		"",
	} {
		if IsMeteredCredentialName(n) {
			t.Errorf("%q is refused by this predicate and is not what rangerhq-kiz ruled on", n)
		}
	}
}

// Arm 2 — the container precondition refuses it WITH the name present. That
// "with" is the whole arm: presence is the question the loop asks, and a
// metered name that is present answers it yes.
func TestQAMeteredCredentialRefusedAtTheContainerPrecondition(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	rt := meteredRuntime(t, a, "metered", "ANTHROPIC_API_KEY")
	if got := CageCredential(rt); got != "ANTHROPIC_API_KEY" {
		t.Fatalf("the yaml did not take: cage_cred is %q", got)
	}
	err := CheckCageCredential(rt, []string{"BD_ACTOR", "ANTHROPIC_API_KEY"})
	if err == nil {
		t.Fatal("a metered declared credential was admitted because the name was present — the shape ranger-base-41zyo measured")
	}
	for _, want := range []string{"ANTHROPIC_API_KEY", "metered spending", "rangerhq-kiz", "runtimes/metered.yaml"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q: %v", want, err)
		}
	}
	// It must not be mistakable for the MISSING-credential refusal, whose
	// next move is to mint one into an env set — which here would be an
	// instruction to put a metered key where the launch will read it.
	if strings.Contains(err.Error(), "is not in this session's environment") {
		t.Errorf("the money-line refusal is wearing the missing-credential sentence: %v", err)
	}
}

// Arm 3 — the shim precondition refuses it with no shim in play. deny is
// empty and binDir names nothing, so CredGateCollision returns "" and the
// gate's own question does not arise.
func TestQAMeteredCredentialRefusedWithNoShimCollision(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	rt := meteredRuntime(t, a, "meteredshim", "ANTHROPIC_API_KEY")
	if rule := CredGateCollision(rt, nil, t.TempDir()); rule != "" {
		t.Fatalf("this arm needs no collision, got rule %q", rule)
	}
	err := CheckCredGate("builder", rt, nil, t.TempDir(), []string{"ANTHROPIC_API_KEY"})
	if err == nil {
		t.Fatal("no collision, so the money line was never asked")
	}
	if !strings.Contains(err.Error(), "metered spending") {
		t.Errorf("the refusal is not the money line's: %v", err)
	}
	// A runtime declaring something else, with no collision, still passes:
	// this arm adds a refusal and does not turn the gate on.
	ok := meteredRuntime(t, a, "notmetered", "OWN_TOKEN")
	if err := CheckCredGate("builder", ok, nil, t.TempDir(), nil); err != nil {
		t.Errorf("a non-metered runtime with no collision must still pass: %v", err)
	}
}

// Arm 4 — the write path reads the same definition and still refuses both
// ways. The sentences stay its own: a write refusal and an admission
// refusal have different next moves.
//
// BOTH WAYS IS TWO HALVES, and until ranger-base-ghcx3 this arm had one of
// them. `checkSessionToken` is the SHAPE — a value that looks like a metered
// key, whatever variable it was offered for — and the NAME refusal is a
// different line in a different function (refreshSession, refresh.go), which
// nothing in this tree held: MEASURED 2026-10-04, `if false &&
// IsMeteredCredentialName(key)` survived this arm, survived a
// Cage|Cred|Refresh|Metered|Launch|Wrap|Mint|Token|Session|Runtime|Gate
// filter over the whole package (61.2s) and survived the UNFILTERED package
// (257.9s). An arm whose header claims a refusal the body never reaches is
// worse than no arm: it is the reason nobody wrote the real one.
func TestQAMeteredCredentialStillRefusedAtTheWrite(t *testing.T) {
	t.Parallel()
	if err := checkSessionToken(meteredKeyPrefix+"03-"+strings.Repeat("A", 32), "CLAUDE_CODE_OAUTH_TOKEN"); err == nil {
		t.Error("a metered-shaped value was accepted for the session variable")
	} else if !strings.Contains(err.Error(), meteredKeyPrefix) {
		t.Errorf("the shape refusal does not name the prefix: %v", err)
	}
	if err := checkSessionToken("sk-ant-oat01-"+strings.Repeat("B", 32), "CLAUDE_CODE_OAUTH_TOKEN"); err != nil {
		t.Errorf("a setup-token shape must be accepted: %v", err)
	}

	// The NAME half, through the one function that would do the writing. The
	// route is the same promoted-config line arm 2 uses, because that is how
	// a metered name reaches `posse refresh` at all: the operator types
	// `posse refresh <runtime>`, and what names the variable is the runtime
	// profile, not the argument.
	a := cageApp(t)
	rt := meteredRuntime(t, a, "meteredwrite", "ANTHROPIC_API_KEY")
	var w strings.Builder
	err := a.refreshSession(&w, rt, RefreshOpts{})
	if err == nil {
		t.Fatal("refreshSession minted into an env set for a runtime whose declared credential is metered spending")
	}
	// It must be THE WRITE SENTENCE and not merely an error. An undecided
	// runtime fails two lines earlier (refresh.go, the empty cage_cred
	// branch), so an arm that asserted only `err != nil` would pass against
	// a tree that had lost this refusal entirely — the metered name would
	// simply be read as a name posse does not know.
	for _, want := range []string{"ANTHROPIC_API_KEY", "does not write it", "metered spending", "rangerhq-kiz", "ADR 0019 D4"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the write refusal does not name %q: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), "no session credential name decided") {
		t.Errorf("the refusal is the UNDECIDED one, so this arm reached the wrong branch: %v", err)
	}
	// Nothing was written on the way to the refusal: the refusal is upstream
	// of resolveRefreshSet and of the mint, so there is no env set to read
	// back and no token to leak into one.
	if s := w.String(); s != "" {
		t.Errorf("the refused write still said something to the operator: %q", s)
	}
	// The control, so this is a refusal of the NAME and not of the route: the
	// same call shape on a non-metered declared credential gets past this
	// line and fails further in, where the mint is.
	ok := meteredRuntime(t, a, "okwrite", "OWN_TOKEN")
	err = a.refreshSession(io.Discard, ok, RefreshOpts{})
	if err != nil && strings.Contains(err.Error(), "metered spending") {
		t.Errorf("a non-metered declared credential was refused by the money line: %v", err)
	}
}

// Arm 5 — the shipped runtimes are unmoved. Adding a refusal that also
// refuses what worked yesterday is not a narrowing, it is an outage.
func TestQAMeteredCredentialLeavesTheShippedRuntimesAlone(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	claude, err := a.LoadRuntime("claude")
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckMeteredSessionCredential(claude); err != nil {
		t.Errorf("claude's decided credential is not metered: %v", err)
	}
	if err := CheckCageCredential(claude, []string{"CLAUDE_CODE_OAUTH_TOKEN"}); err != nil {
		t.Errorf("claude with its mint present must still admit: %v", err)
	}
	// codex and grok are UNDECIDED, which is its own refusal with its own
	// next move (decide it), and must not be re-spelled as the money line.
	for _, n := range []string{"codex", "grok"} {
		rt, err := a.LoadRuntime(n)
		if err != nil {
			t.Fatal(err)
		}
		if err := CheckMeteredSessionCredential(rt); err != nil {
			t.Errorf("%s declares nothing, which is not metered: %v", n, err)
		}
		err = CheckCageCredential(rt, []string{"CLAUDE_CODE_OAUTH_TOKEN"})
		if err == nil || strings.Contains(err.Error(), "metered spending") {
			t.Errorf("%s's refusal must stay the undecided one: %v", n, err)
		}
	}
}

// Arm 6 — the grid says which refusal holds instead of printing a metered
// name as this runtime's credential. The row's own note has claimed the
// refusal since it was written; this is the arm that keeps the claim true.
func TestQAMeteredCredentialRowNamesTheRefusal(t *testing.T) {
	t.Parallel()
	a := cageApp(t)
	rt := meteredRuntime(t, a, "meteredrow", "ANTHROPIC_API_KEY")
	r := cageCredRow(rt)
	if !strings.Contains(r.value, "REFUSED") {
		t.Errorf("the row prints a metered name as this runtime's credential: %q", r.value)
	}
	for _, want := range []string{"metered spending", "ADR 0019 D7"} {
		if !strings.Contains(r.value, want) {
			t.Errorf("the row does not name %q: %q", want, r.value)
		}
	}
	// A decided, non-metered credential keeps the row it always had.
	ok := meteredRuntime(t, a, "okrow", "OWN_TOKEN")
	if got := cageCredRow(ok); strings.Contains(got.value, "REFUSED") {
		t.Errorf("a non-metered runtime's row reads as refused: %q", got.value)
	}
}
