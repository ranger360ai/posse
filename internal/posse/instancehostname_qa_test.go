//go:build !posse_arm2 && !posse_arm3

package posse

// AN INSTANCE'S HOST AND TAILNET NAMES ARE A VISIBILITY CLASS, AND THE
// ENTRY THAT WALLS THEM IS BARE (ADR 0048 D1/D2, ranger-base-qwv80, from
// ranger-base-9v2xo).
//
// WHAT HAPPENED. A docs/notes.d measurement table quoted the host's own
// name, read through /proc/<pid>/root/etc/hostname — a reading whose whole
// point is that the name came back. The prepare-commit-msg wall passed it,
// the landing push scan flagged it and did not gate the push, and the name
// reached public posse; it was fixed forward and the history keeps it.
// MEASURED 2026-11-13 over every commit on main (608 of them at that
// point): the three names in question appear in the ADDED lines of exactly
// ONE commit, and it is that one — so what was missing was a pattern, not a
// second scan.
//
// WHY THE WALL PASSED IT, and this is the whole of it: the host's name is
// not derived. Check 3 derives the box's username, every scope's git e-mail
// and the instance repo's path (DeriveIdentityLiterals) and NOT its
// hostname, and the shipped ops-pattern list is what any deployer needs, so
// a name that is confidential at ONE deployment is that deployment's own
// vocabulary — config beads_visibility_patterns:, which is where this fix
// lives. The entries themselves are never in this tree (ADR 0048 D2), so
// every fixture below is a fixture's own: `nimbusbox` and `tail9q7z2`,
// never this box's, exactly as qaInstanceName is.
//
// THE TRAP, and it is the reason this file is three pins and not one.
// Both entries this instance already carries use ADR 0048 D1's MARKER form,
// `<name>([^-]|-[^0-9a-z]|-?$)` — the exception that lets a bead id through
// — and it is the shape an operator adding a third entry would copy. It
// does not fit a host name. MEASURED 2026-11-13, the same three node
// spellings through both EREs:
//
//	subject                        marker-form   bare
//	nimbusbox                      hit           hit
//	nimbusbox-rex                  MISS          hit
//	nimbusbox.tail9q7z2.ts.net     hit           hit
//
// The MISS is the one that matters here: the always-on host's multiplexer
// registers its own tsnet node as `<host>-rex` (ADR 0080 D1, access over
// the operator's tailnet), so a marker-form entry walls the host and leaves
// the node that is actually reachable from outside the box unwalled. A host
// name has no marker form and no legitimate public spelling at all, so the
// entry is BARE — and bare is also what makes ONE entry cover the host, the
// tsnet node and the MagicDNS name, which is PIN 2.
//
// AND IT IS STILL A LINT, NOT A BOUNDARY (visibility.go,
// OpsPatternsConfigKey). Two residuals, stated because a pattern list's
// misses are its own documentation: an all-caps spelling of the host name
// is unwalled (the entry case-folds the first character only, which is the
// variation a writer produces), and Tailscale's DISPLAY name for a node can
// carry an apostrophe, which this dialect cannot express at all — a single
// quote is refused by validateOpsERE because the value is rendered into a
// single-quoted sh word.
//
// EVERY PIN HERE IS MUTATION-CHECKED, and RUN — MEASURED 2026-11-13, three
// mutants one at a time against a golden copy, which is what says these
// three pins are three and not one assertion written out three ways:
//   - M1, the pre-0048 scope: identityGuardCheck forgets `extra` after its
//     early return, so the instance entries are not rendered at all. Reds
//     PIN 1, all three rows of PIN 2 and PIN 3 — it is the arm, not this
//     subject, and this mutant reds every pin in the scope and edges files
//     too.
//   - M2, the marker form restored: give qaHostERE ADR 0048 D1's
//     marker-form exception instead of the bare spelling. Reds the `-rex`
//     row of BOTH of PIN 2's halves — the wall's and the two EREs' — and
//     leaves PIN 1, PIN 3 and PIN 2's other two rows GREEN, because the
//     host name and the MagicDNS name are hits either way. That split is
//     this file's reason to exist.
//   - M3, the tailnet entry dropped from the stamped config (with the
//     fixture premise's count dropped to match, so the premise is not what
//     reds it). Reds PIN 3 ALONE. That is what says the two classes are
//     two: the host entry does not reach a line that names no host.
//   - M4, the HOST entry dropped instead, same premise adjustment. Reds PIN
//     1 and PIN 2's host and `-rex` rows, leaves PIN 3 green — and leaves
//     PIN 2's MagicDNS row GREEN too, which is the one thing none of these
//     pins asserts on purpose and this mutant measures: `<host>.<tailnet>
//     .ts.net` is a hit of BOTH classes, so neither entry alone is the
//     whole wall and the draft is two entries for that reason as well.
//
// Each mutant's blast radius above is stated over THIS cluster, the four
// filtered runs that were measured. internal/posse is a ~950s package and
// no mutant here was run over the whole of it; M1 is the only one whose
// subject is shared code, and the scope and edges files name it themselves.

import (
	"strings"
	"testing"
)

// The host-name and tailnet-name classes in the shape ranger-base-qwv80
// drafted them for this instance's config: a class name, and an ERE that is
// BARE — no marker-form exception, because a host name has no legitimate
// public spelling to make an exception for.
//
// Fixture values throughout. `nimbusbox` and `tail9q7z2` are absent from
// this tree (MEASURED 2026-11-13, `git grep -i` over 1,903 tracked files:
// zero hits each), which is what lets this file name them in its own source
// — a fixture that collided with the live entries would refuse its own
// commit.
const (
	qaHostClass    = "instance-host-name"
	qaHostName     = "nimbusbox"
	qaHostERE      = "[nN]imbusbox"
	qaTailnetClass = "tailnet-name"
	qaTailnetID    = "tail9q7z2"
	qaTailnetERE   = "[tT]ail9q7z2"

	// The multiplexer's tsnet node, the way ADR 0080 D1 names it, and the
	// MagicDNS name the tailnet gives the host.
	qaHostRexNode = qaHostName + "-rex"
	qaHostDNSName = qaHostName + "." + qaTailnetID + ".ts.net"

	qaHostCfg = OpsPatternsConfigKey + ":\n" +
		"  " + qaHostClass + ": " + qaHostERE + "\n" +
		"  " + qaTailnetClass + ": " + qaTailnetERE + "\n"
)

// qa9v2xoLine is the staged line this bead exists for, restated with the
// fixture's host name: the measurement row whose third cell IS the reading.
// Byte-for-byte the shape of the row that landed, because a pin over a
// paraphrase measures the paraphrase.
const qa9v2xoLine = "| uncaged, same uid, live target | `/proc/<pid>/root/etc/hostname` | reads `" + qaHostName + "` |\n"

// qa9v2xoFragment is the whole staged file: a docs/notes.d fragment with the
// table around that row, which is the genre the line actually travelled in.
const qa9v2xoFragment = "# ranger-base-9v2xo — what a caged seat can read of an uncaged one\n\n" +
	"| reader | target | result |\n|---|---|---|\n" + qa9v2xoLine

// qaHostWall is the visibility wall with this bead's two entries stamped,
// and it asserts the same fixture premise qaInstanceWall does: a pattern the
// stamp REFUSED guards nothing, and a pin over one is green against any wall
// at all.
func qaHostWall(t *testing.T) *visWall {
	t.Helper()
	w := newVisWallCfg(t, "instance", qaHostCfg)
	set := (&App{ConfigPath: w.home + "/config.yaml"}).OpsPatternSet()
	if len(set.Rejected) > 0 || len(set.Extra) != 2 {
		t.Fatalf("fixture premise: both entries must be accepted at stamp time, got extra=%+v rejected=%v", set.Extra, set.Rejected)
	}
	for i, want := range []string{qaHostClass, qaTailnetClass} {
		if set.Extra[i].Class != want {
			t.Fatalf("fixture premise: entry %d must be %q, got %q", i, want, set.Extra[i].Class)
		}
	}
	return w
}

// qaNoHostVocabulary is the withholding assertion for this cluster's own
// fixture values (ADR 0048 D2, ranger-base-8114t): an instance entry's
// refusal names the class and a hit count, never the value. It is
// qaNoVocabulary's claim over a different name, and a separate function
// rather than a parameter on that one because the pins that call it name
// their own allowed subjects and nothing else changes hands.
func qaNoHostVocabulary(t *testing.T, what, text string) {
	t.Helper()
	for _, ere := range []string{qaHostERE, qaTailnetERE} {
		if strings.Contains(text, ere) {
			t.Errorf("%s carried an entry's ERE — that IS the value the key exists to keep out:\n%s", what, text)
		}
	}
	for _, v := range []string{qaHostName, qaTailnetID} {
		if strings.Contains(text, v) {
			t.Errorf("%s carried the instance's own %q — a refusal is read in a terminal and pasted onto beads:\n%s", what, v, text)
		}
	}
}

// PIN 1: THE REGRESSION. The 9v2xo row, in the genre it travelled in, in a
// public-stamped repo with this bead's entries stamped — refused, by the
// host class, naming neither the name nor the ERE. And the CONTROL, which
// is what says this pin measured the entry and not the line: the same
// bytes, the same public stamp, a wall stamped from a config with no
// patterns key at all commit CLEAN. That control is not a hypothetical —
// it is the wall the row actually landed through.
//
// MUTATION-CHECKED: M1 and M4 (the host entry dropped) each red the refusal
// half; M3, the tailnet entry dropped, leaves this pin GREEN, because the
// row names a host and no tailnet. And a wall that refused this markdown for
// some OTHER reason (check 1's docs-genre allowlist, check 2's shipped
// markdown list) is caught by the control half going red instead.
func TestQAInstanceHostNameRefusesThe9v2xoLine(t *testing.T) {
	w := qaHostWall(t)
	const rel = "docs/notes.d/ranger-base-9v2xo.md"

	w.stage(t, w.pub, rel, qa9v2xoFragment)
	out, err := w.git(w.pub, w.persona, "commit", "-m", "the notes", "--", rel)
	if err == nil {
		t.Fatalf("the row that reached public posse must be refused:\n%s", out)
	}
	for _, want := range []string{
		"refused by posse gate: an instance-defined visibility class in a staged file",
		"ADR 0048 D2",
		qaHostClass + ":",
		"matched in the staged additions:",
		OpsPatternsConfigKey + ":",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the refusal must carry %q:\n%s", want, out)
		}
	}
	// Not check 3's identity arm wearing a different hat: the hostname is
	// exactly the box value DeriveIdentityLiterals does NOT derive, which is
	// why this is a config entry at all.
	if strings.Contains(out, "an operator identity literal") {
		t.Errorf("a host-name hit is an instance class, not a derived identity literal:\n%s", out)
	}
	qaNoHostVocabulary(t, "the refusal", out)
	if !strings.Contains(w.log(t), "instance pattern scan [prepare-commit-msg hook] (public repo)") {
		t.Errorf("the refusal must be logged under its own name:\n%s", w.log(t))
	}
	qaNoHostVocabulary(t, "refusals.log", w.log(t))

	// THE CONTROL: the shipped list alone takes it, which is the measured
	// history of this row.
	plain := newVisWall(t)
	plain.stage(t, plain.pub, rel, qa9v2xoFragment)
	if out, err := plain.git(plain.pub, plain.persona, "commit", "-m", "the notes", "--", rel); err != nil {
		t.Errorf("fixture premise: without the config entry this row is clean to the shipped wall — that is how it landed: %v\n%s", err, out)
	}

	// And the reading itself is not lost: these are VISIBILITY entries, so
	// the private tree still takes the row verbatim. That is the remedy the
	// refusal names (write it in the instance tree, restate and cite), and
	// it is the reason these two classes belong under
	// beads_visibility_patterns: and not under data_ceiling_patterns:,
	// which would refuse the row in every repo on the box and leave the
	// measurement nowhere to live.
	w.stage(t, w.priv, rel, qa9v2xoFragment)
	if out, err := w.git(w.priv, w.persona, "commit", "-m", "the notes", "--", rel); err != nil {
		t.Errorf("the private tree must still take the reading: %v\n%s", err, out)
	}
}

// PIN 2: ONE BARE ENTRY, THREE NODE SPELLINGS — and the marker form's MISS,
// measured here rather than asserted in prose. The host name, the
// multiplexer's `<host>-rex` tsnet node (ADR 0080 D1) and the MagicDNS name
// are all hits of the ONE host class; under ADR 0048 D1's marker-form
// exception, which is the shape both of this instance's existing entries
// use, the tsnet node is not.
//
// MUTATION-CHECKED: M2 (qaHostERE given the marker-form exception) reds the
// `-rex` row of the first half and the `<host>-rex` row of the second, and
// nothing else in this cluster — PIN 1, PIN 3 and this pin's other two rows
// stay green under it, which is the split that makes `-rex` a pin of its
// own rather than a third spelling in a list.
func TestQAInstanceHostNameEntryIsBare(t *testing.T) {
	// Half one: the wall itself, over each spelling, as a staged line.
	for _, subject := range []string{qaHostName, qaHostRexNode, qaHostDNSName} {
		t.Run("refuses "+subject, func(t *testing.T) {
			w := qaHostWall(t)
			const rel = "docs/notes.d/ranger-base-qwv80.md"
			w.stage(t, w.pub, rel, "the reading came back from "+subject+" over the tailnet\n")
			out, err := w.git(w.pub, w.persona, "commit", "-m", "x", "--", rel)
			if err == nil {
				t.Fatalf("%q must be refused — one bare entry covers every node spelling:\n%s", subject, out)
			}
			if !strings.Contains(out, "an instance-defined visibility class in a staged file") {
				t.Errorf("the refusal must come from the instance arm:\n%s", out)
			}
			qaNoHostVocabulary(t, "the refusal", out)
		})
	}

	// Half two: WHY the entry is bare, as a property of the two EREs rather
	// than of the wall — the row that would have been missed, named. A pin
	// over the wall alone cannot say this: the marker form is a config value
	// nobody has, so there is no wall to stamp from it that is not this
	// comparison written out.
	marker := qaHostName + "([^-]|-[^0-9a-z]|-?$)"
	bare := qaHostERE
	for _, row := range []struct {
		subject    string
		wantMarker bool
	}{
		{qaHostName, true},
		{qaHostRexNode, false}, // the MISS this pin exists for
		{qaHostDNSName, true},
	} {
		m, err := NewOpsPattern(qaHostClass, marker)
		if err != nil {
			t.Fatalf("the marker form must be a valid entry — it is the one in force today: %v", err)
		}
		b, err := NewOpsPattern(qaHostClass, bare)
		if err != nil {
			t.Fatalf("the bare form must be a valid entry: %v", err)
		}
		if !b.Match(row.subject) {
			t.Errorf("the BARE entry must match %q — that is the whole of its claim", row.subject)
		}
		if got := m.Match(row.subject); got != row.wantMarker {
			t.Errorf("the marker form over %q: want match=%v, got %v — if this row flipped, ADR 0048 D1's exception changed and the host entry's shape has to be re-decided", row.subject, row.wantMarker, got)
		}
	}
}

// PIN 3: THE TAILNET SUFFIX IS ITS OWN ENTRY. The host entry does not reach
// a line that carries the tailnet and not the host — a bare MagicDNS suffix,
// which is what a `rex server start --tailscale` invocation, a node URL or a
// `tailscale status` paste carries — so the draft is two entries and the
// refusal names which one it was.
//
// MUTATION-CHECKED: M1 reds it; dropping the tailnet entry from qaHostCfg
// reds it and leaves PIN 1 and PIN 2 green, which is what says the two
// classes are two.
func TestQAInstanceTailnetNameIsItsOwnClass(t *testing.T) {
	w := qaHostWall(t)
	const rel = "docs/adr/0080-the-always-on-host.md"
	w.stage(t, w.pub, rel, "the listener registers under "+qaTailnetID+".ts.net\n")
	out, err := w.git(w.pub, w.persona, "commit", "-m", "x", "--", rel)
	if err == nil {
		t.Fatalf("the tailnet's MagicDNS suffix must be refused:\n%s", out)
	}
	if !strings.Contains(out, qaTailnetClass+":") {
		t.Errorf("the refusal must name the tailnet class, which is how the writer knows which name they wrote:\n%s", out)
	}
	if strings.Contains(out, qaHostClass+":") {
		t.Errorf("the host entry must NOT reach a line that names no host — if it does, these are one class and the draft should say so:\n%s", out)
	}
	qaNoHostVocabulary(t, "the refusal", out)
}
