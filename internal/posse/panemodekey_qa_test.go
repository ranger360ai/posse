package posse

// QA pins for ranger-base-x3hs1 — `pane_mode:` as a REGISTRY KEY on Runtime
// (ADR 0057 D1/D2), the shape `turn_outcome:` already wears.
//
// What ranger-base-vwgt shipped read a pane through a map keyed on the
// runtime's NAME plus an `if runtime == "codex"` branch. Both are ADR 0017 §3
// shadow predicates — a name standing in for a dimension — and their cost was
// exact: a fourth runtime listed every session as `mode:?` with the why
// "nobody has measured what its pane says", forever, and the only way out was
// editing Go. So the question these pins answer is Bob's, not the reader's:
// can a CLI that paints a claude-shaped footer be READ by declaring one line
// of yaml, and can a CLI measured to paint nothing DECLARE that?
//
// Four claims, and each one is a different half of the same seam:
//
//   - a yaml runtime declaring a registered reader is read TODAY, no Go;
//   - a name no reader implements refuses AT LOAD, naming what is on offer,
//     rather than promising a reading nothing performs;
//   - the built-in wall holds: which reader parses a CLI's screen is code
//     measured against that CLI's captures, so an overlay may not move it
//     (ADR 0021 D2) — while a yaml runtime's OWN file may, which is the
//     fact/mechanism split stated as a contrast rather than asserted;
//   - `none` is a DECLARATION and costs no herdr call, which is the
//     difference between "measured to render nothing" and "nobody looked".
//
// The corpus these run against is permissionmodepane_qa_test.go's verbatim
// captures — the same fixtures the shipped reader is checked on, so a green
// here is the reader the fleet runs reached through a declaration, not a
// second copy of it agreeing with itself.

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// paneReadCount is how many pane reads THIS test's fake herdr served. Absent
// log = zero, which is the reading the `none` pin needs to be able to take.
func paneReadCount(t *testing.T, fake string) int {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(fake, "pane-read-log"))
	if err != nil {
		return 0
	}
	return len(strings.Fields(string(b)))
}

// paneModeRuntime writes a yaml runtime with the given pane_mode: body (or
// none at all) and returns its name. Deliberately NOT one of the built-ins:
// the whole claim is that a CLI posse ships no Go for can declare a reader.
func paneModeRuntime(t *testing.T, b *HerdrBackend, name, decl string) string {
	t.Helper()
	if err := os.MkdirAll(b.App.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "command: " + name + " --sys {file}\n" + decl
	if err := os.WriteFile(filepath.Join(b.App.RuntimesDir(), name+".yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return name
}

// The bead's first done-when, and the ADR's first consequence: a CLI whose
// pane paints a claude-shaped footer declares `pane_mode: claude-footer` and
// is READ, with no Go change. Driven end to end through `posse list`, because
// what is claimed is not that the reader can be called with a string — it is
// that an operator's own yaml reaches an operator's own screen.
//
// Both arms of claude's reader, because the covered one is the reading that
// distinguishes a declaration from a scraper that always answers: a dialog
// over the footer is `?covered` on a declared runtime exactly as it is on the
// built-in, and NOT a mode read out of the launch line in the scrollback.
func TestQAYamlRuntimeDeclaresAPaneReaderAndIsRead(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, pane, want string }{
		{"footer-auto", claudePaneAuto, "mode:auto"},
		{"footer-bypass", claudePaneBypass, "mode:bypassPermissions"},
		{"footer-covered", claudePaneDialog, "mode:?covered"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			b, fake := newTestBackend(t)
			modePersona(t, b, "dev")
			rt := paneModeRuntime(t, b, "mycli", "pane_mode: "+PaneModeClaudeFooter+"\n")
			mustCreate(t, b, NewSessionOpts{Name: "s1", Agent: "dev", Runtime: rt, Tier: TierStandard})
			paneShowing(t, b, fake, "s1", c.pane)
			if ln := modeLine(t, b, "s1"); !strings.Contains(ln, c.want) {
				t.Errorf("a yaml runtime declaring pane_mode: %s lists as:\n  %s\nwant %q — the declaration did not reach the reader",
					PaneModeClaudeFooter, ln, c.want)
			}
		})
	}
}

// The CONTROL for the test above, and the loud default the ADR calls the
// absent case: the same runtime, same pane, no `pane_mode:` line. Without
// this arm a green above would not distinguish a declaration being read from
// a listing that reads every pane with claude's reader regardless.
//
// It also pins the distinction the whole three-valued field turns on: this is
// `mode:?` with "nobody has measured", NOT `mode:—`. A CLI nobody measured
// and a CLI measured to render nothing must not render the same token.
func TestQAUndeclaredRuntimeIsLoudNotNoneAndNotRead(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	modePersona(t, b, "dev")
	rt := paneModeRuntime(t, b, "mycli", "")
	mustCreate(t, b, NewSessionOpts{Name: "s1", Agent: "dev", Runtime: rt, Tier: TierStandard})
	paneShowing(t, b, fake, "s1", claudePaneAuto)
	ln := modeLine(t, b, "s1")
	if !strings.Contains(ln, "mode:?") || strings.Contains(ln, "mode:auto") {
		t.Errorf("an undeclared runtime showing a claude footer lists as:\n  %s\nwant mode:? — a pane nobody declared a reader for is not read by claude's", ln)
	}
	if strings.Contains(ln, "mode:—") {
		t.Errorf("an UNDECLARED runtime rendered as a permanent —:\n  %s\n`none` is a measurement and absence is not; collapsing them is the defect the key exists to remove", ln)
	}
	m := PaneModeUndeclared(rt)
	if m.State != PaneModeUnread || !strings.Contains(m.Why, "nobody has measured") {
		t.Errorf("PaneModeUndeclared(%q) = %+v; want the unread state with the why that names the runtime", rt, m)
	}
	if !strings.Contains(m.Why, rt) {
		t.Errorf("the why never names the runtime it is about: %q", m.Why)
	}
}

// The bead's second done-when. A `pane_mode:` no reader implements REFUSES at
// load — the turn_outcome: arm verbatim — and the message names all three
// registered readers, because a refusal that does not say what is on offer
// sends the operator to the source.
func TestQAUnregisteredPaneModeRefusesAtLoad(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	if err := os.WriteFile(filepath.Join(a.RuntimesDir(), "typo.yaml"),
		[]byte("command: typo {file}\npane_mode: claude-footers\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := a.LoadRuntime("typo")
	if err == nil {
		t.Fatal("a pane_mode: no reader implements must refuse at load — a declaration that promises a reading nobody performs is the p84 class the ADR rejected the table shape over")
	}
	for _, want := range []string{"pane_mode:", "claude-footers", PaneModeClaudeFooter, PaneModeGrokBorder, PaneModeNone} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal = %v, want one containing %q", err, want)
		}
	}
	// And the registered names load, from a yaml, one arm each.
	for _, adapter := range PaneModeAdapters() {
		rt := writeRuntime(t, a, "ok"+strings.ReplaceAll(adapter, "-", ""), "command: x {file}\npane_mode: "+adapter+"\n")
		if rt.PaneModeAdapter != adapter {
			t.Errorf("pane_mode: %s loaded as %q", adapter, rt.PaneModeAdapter)
		}
	}
}

// The bead's third done-when and ADR 0057 D2, stated as the CONTRAST that
// makes it a fact/mechanism split rather than a list membership: the same key
// with the same registered value refuses in a BUILT-IN's overlay and loads in
// a yaml runtime's own file. Which reader parses a CLI's screen is code
// measured against that CLI's captures; a claude release that rewords its
// footer is fixed at the corpus, not by an overlay nobody measured.
//
// TestOverlayRefusesMechanismKeys drives every mechanism key from the list;
// this one is here so the pane_mode arm is legible beside its own contrast,
// and so a refusal that stopped naming the key would red where the key lives.
func TestQAPaneModeRefusesInABuiltinOverlayButLoadsFromAYaml(t *testing.T) {
	t.Parallel()
	a := checkApp(t)
	writeOverlay(t, a, "claude", "pane_mode: "+PaneModeClaudeFooter+"\n")
	_, err := a.LoadRuntime("claude")
	if err == nil {
		t.Fatal("runtimes/claude.yaml declaring pane_mode: loaded clean — an overlay moving which reader parses claude's screen is mechanism, and D2 refuses it")
	}
	for _, want := range []string{"pane_mode:", "ADR 0021 Decision 2", "measured against that CLI's captures"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal = %v, want one containing %q", err, want)
		}
	}
	// The contrast: not a forbidden key, a forbidden PLACE.
	rt := writeRuntime(t, a, "mycli", "command: mycli {file}\npane_mode: "+PaneModeClaudeFooter+"\n")
	if rt.PaneModeAdapter != PaneModeClaudeFooter {
		t.Errorf("a yaml runtime's own file must declare it: %+v", rt.PaneModeAdapter)
	}
}

// The bead's fourth done-when, and the cost half of the seam: `pane_mode:
// none` lists as a permanent `—` and spends NO pane read. It is what makes
// `none` worth being a reader at all — the answer is a constant, and paying a
// herdr call per session to re-learn it buys nothing.
//
// The control arm is the same rig declaring claude-footer, which MUST read:
// without it a zero here would be a fake herdr nobody asked anything, not a
// declaration that skipped the call (NOTES, "a rig must be shown able to
// fail"). Two sessions per arm, because the claim is per session and one of
// each would not tell a skipped read from an unfilled row.
func TestQAPaneModeNoneRendersDashAndSpendsNoPaneRead(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, decl, want string
		reads            int
	}{
		{"none", "pane_mode: " + PaneModeNone + "\n", "mode:—", 0},
		{"claude-footer", "pane_mode: " + PaneModeClaudeFooter + "\n", "mode:auto", 2},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			b, fake := newTestBackend(t)
			modePersona(t, b, "dev")
			rt := paneModeRuntime(t, b, "mycli", c.decl)
			for _, s := range []string{"s1", "s2"} {
				mustCreate(t, b, NewSessionOpts{Name: s, Agent: "dev", Runtime: rt, Tier: TierStandard})
				paneShowing(t, b, fake, s, claudePaneAuto)
			}
			for _, s := range []string{"s1", "s2"} {
				if ln := modeLine(t, b, s); !strings.Contains(ln, c.want) {
					t.Errorf("%s: session %s lists as:\n  %s\nwant %q", c.name, s, ln, c.want)
				}
			}
			// modeLine runs `posse list` once per call, so the reads are per
			// listing × per session. What is pinned is the ZERO, and that the
			// control is not also zero.
			if got := paneReadCount(t, fake); (c.reads == 0) != (got == 0) {
				t.Errorf("%s: the fake herdr served %d pane read(s), want %s — a declared `none` is a constant, and a declared reader must actually read",
					c.name, got, map[bool]string{true: "none", false: "at least one"}[c.reads == 0])
			}
		})
	}
}

// The registry is what the grid row renders from (ADR 0057 D3, shipped
// ranger-base-2p2cy), so every entry owes the row something to say: a reader, the sentence
// an absence carries, and the one-line contract. An entry with a blank field
// is a row that ships as scenery.
//
// The second half is the one that could actually be wrong: the entry's
// Absence is asserted to be WHAT THE READER RETURNS on a pane with nothing on
// it, not merely non-empty. A registry describing an absence its own reader
// does not report is the same defect one layer up from a declared table of
// footer spellings — prose beside code, agreeing by hand.
func TestQAEveryPaneReaderIsRenderableAndSaysWhatItReturns(t *testing.T) {
	t.Parallel()
	names := PaneModeAdapters()
	if !sort.StringsAreSorted(names) {
		t.Errorf("PaneModeAdapters is unsorted: %v — a refusal message must not reorder between runs", names)
	}
	if want := []string{PaneModeClaudeFooter, PaneModeGrokBorder, PaneModeNone}; strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("registered readers = %v, want %v — a reader added or removed changes what an operator may declare, so it is a decision, not a detail", names, want)
	}
	for _, name := range names {
		r, ok := PaneModeReaderFor(name)
		if !ok {
			t.Fatalf("%s: PaneModeAdapters names a reader PaneModeReaderFor does not resolve", name)
		}
		if r.Read == nil {
			t.Errorf("%s: no reader func — the key would load and read nothing", name)
			continue
		}
		if strings.TrimSpace(r.Absence) == "" || strings.TrimSpace(r.Contract) == "" {
			t.Errorf("%s: absence %q, contract %q — the grid row has nothing to render (ADR 0057 D3)", name, r.Absence, r.Contract)
		}
		// A screen with nothing on it: every reader's not-named answer, and
		// it must be the sentence its own registry row promises.
		m := r.Read(nil)
		if m.State == PaneModeNamed {
			t.Errorf("%s: an empty pane named %q — a reader that guesses a mode off no screen is the reading this field exists to replace", name, m.Mode)
			continue
		}
		if m.Why != r.Absence {
			t.Errorf("%s: reader says %q, registry row says %q — the row describes an absence the reader does not report", name, m.Why, r.Absence)
		}
		// ReadsPane is the cost claim, and it is checkable: a reader that
		// does not read a pane must answer the same thing whatever it is
		// handed, or skipping the herdr call would lose a reading.
		if !r.ReadsPane && ReadPaneMode(name, claudePaneAuto) != m {
			t.Errorf("%s: declares ReadsPane false but answers differently with a pane — the skipped read was not free", name)
		}
	}
}

// Every built-in declares a REGISTERED reader. The name-keyed map this
// replaced made that true by construction; a struct field makes it something
// a built-in can silently ship without, and the symptom would be codex-shaped
// — every session `mode:?` on a runtime posse measured years ago.
func TestQAEveryBuiltinDeclaresARegisteredPaneReader(t *testing.T) {
	t.Parallel()
	for _, rt := range builtinRuntimes {
		if rt.PaneModeAdapter == "" {
			t.Errorf("built-in %s declares no pane_mode: — posse measured all three built-ins on 2026-08-29 (permissionmodepane_qa_test.go), so an undeclared one is a measurement thrown away, not an unknown", rt.Name)
			continue
		}
		if _, ok := PaneModeReaderFor(rt.PaneModeAdapter); !ok {
			t.Errorf("built-in %s declares pane_mode: %q, which no reader implements — the load-time refusal does not run on built-ins, so this is the pin", rt.Name, rt.PaneModeAdapter)
		}
	}
}
