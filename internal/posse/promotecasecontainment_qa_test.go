//go:build posse_arm3

package posse

// QA pins for ranger-base-v603x — `posse promote` refuses to promote onto
// the constitution it is promoting FROM by IDENTITY, not by string.
//
// THE DEFECT. CmdPromote (promote.go) asks "are the home and the source the
// same tree?" before it copies, and it asked with underDir, which compares
// the two resolved spellings using filepath.Rel. The two sides are
// independent: `src` comes from resolvePromoteSource (the `--source` flag,
// or the previous manifest's recorded source), `a.Home` is `RHQ_HOME`
// carried verbatim through newApp/NewAppAt, and nothing canonicalises
// either. It FAILS OPEN on a false — "not inside" means the guard declines
// and the promote proceeds — and the refusal's own comment states the cost:
// "promoting there would copy a tree onto itself and then 'remove what the
// source no longer has' from the source".
//
// MEASURED 2026-11-08, darwin 25.6.0, go1.26.10 darwin/arm64, APFS
// case-insensitive, through promoteFixture with the home spelled in another
// case of the source (`…/constitution/posse` against `…/constitution/POSSE`):
//
//	samePath(src, cased)             = true   (one directory)
//	underDir(home=cased, src)        = false  [want true]
//	underDir(src, home=cased)        = false  [want true]
//	underSameDir(src, cased)         = true   [the fixed predicate]
//	CmdPromote(...)                  = nil    — THE GUARD DID NOT FIRE
//	…and `promoted.json` was written INTO the constitution source tree,
//	where `git status` then reports it untracked (`?? posse/promoted.json`).
//
// THE LIMIT, STATED. The second half of the comment's cost — "remove what
// the source no longer has" — is NOT reachable today, and this file does not
// pin it. promoteRemovals is bounded to PromotedPaths, and promoteCleanGate
// refuses on any dirty or ignored path inside exactly those pathspecs
// (`--ignored=matching`), so a file the recorded commit lacks cannot be
// present inside a promoted path AND pass the gate. MEASURED the same day:
// an uncommitted `agents/wip.md` in the source is refused by the clean gate
// before the copy, so the prune removed nothing. What this guard protects
// against today is therefore the self-copy and the manifest written into the
// constitution — the operator's `posse promote` reporting success while the
// promoted set and the manifest land in the repo instead of the home. The
// prune is defence in depth behind the clean gate; this refusal is the one
// that names the condition.
//
// THE FIX is underSameDir (visibility.go) — the string answer first,
// unchanged, then a climb up the ancestors asking samePath, which stats. ADR
// 0082 D4's closing paragraph prescribes exactly that for a caller whose
// question is only ever about the directory, and rules out the alternatives:
// underDir's own sides must not canonicalise, because ~25 of its callers
// decide sandbox grants, cage mounts, PID `writable:` reach and store
// placement, and for those the spelling another program matches IS the
// subject. This is the third call site to move (CommitQueueJSONL,
// ranger-base-oczuq; initFrom's operator fence, ranger-base-2emry).
//
// WHAT EACH PIN IS FOR. The four refusal arms are the bead's DONE WHEN in
// the shapes the case axis takes: the home as another case of the source
// itself, a home NESTED inside the source reached through a recased
// spelling, a recased ANCESTOR (ADR 0082's own `~/src` against `~/SRC` pair,
// which is what makes this ordinary rather than a typo), and the source
// inside a recased home. The last two of those are each the ONLY arm
// constraining one half of the guard's `||`, which the mutation record below
// measured rather than assumed. The separate stranger pin holds the other
// direction — a genuine stranger home still promotes — so a predicate that
// answered true unconditionally reds there rather than passing quietly. Every arm asserts the refusal's OWN text ("the same tree"), not
// merely that an error came back, so an arm cannot pass because some
// unrelated gate refused first.
//
// MUTATION-CHECKED 2026-11-08, each mutant run over these two names alone:
//
//   - reverting promote.go's call to underDir reds all FOUR refusal arms of
//     TestQAPromoteRefusesTheConstitutionAsItsOwnHomeByIdentity and leaves
//     the stranger pin green — so these arms measure the call site and not
//     the predicate.
//   - making underSameDir return true unconditionally reds
//     TestQAPromoteStillPromotesToAStrangerHome alone.
//   - dropping either direction of the `||` reds the one arm that needs it,
//     and only that arm: keeping `underSameDir(a.Home, src)` alone reds
//     "nested inside the source under another case", and keeping
//     `underSameDir(src, a.Home)` alone reds "the source inside a recased
//     home". The same-directory and recased-ancestor arms are answered by
//     EITHER direction, so neither of them constrains the `||` — which is
//     why the fourth arm exists. Measured, not assumed: with only the three
//     arms this file was first written with, dropping the first direction
//     reded nothing at all.
//
// THE SKIP. Every arm needs a case-INsensitive volume, and whether a mount
// is one is a per-mount question neither statfs nor a filesystem name
// answers (samePath's note). `recased` asks the filesystem directly and
// skips when the two spellings are genuinely two directories; on a
// case-sensitive volume there is no defect to measure here.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestQAPromoteRefusesTheConstitutionAsItsOwnHomeByIdentity is the bead's
// DONE WHEN: the refusal fires through a spelling of the home that is not
// the spelling of the source, in the three shapes the case axis takes.
func TestQAPromoteRefusesTheConstitutionAsItsOwnHomeByIdentity(t *testing.T) {
	t.Parallel()

	// Each case names the home, given the fixture's source, and runs in its
	// own fixture — a refused promote writes nothing, but the guard is
	// reached after the clean gate and the arms must not share a tree.
	for _, tc := range []struct {
		name string
		home func(t *testing.T, src string) string
	}{
		{
			// The source itself, under another case: one directory, two
			// spellings, which is what underDir's Rel cannot see.
			name: "the source under another case",
			home: func(t *testing.T, src string) string { return recased(t, src) },
		},
		{
			// A home NESTED inside the constitution, reached through the
			// recased spelling of the source. This is the direction
			// underSameDir(src, a.Home) answers.
			name: "nested inside the source under another case",
			home: func(t *testing.T, src string) string {
				return filepath.Join(recased(t, src), "home")
			},
		},
		{
			// The recased component is an ANCESTOR rather than the leaf —
			// ADR 0082's own `~/src` against `~/SRC` pair. The firing
			// configuration is any RHQ_HOME whose path differs from the
			// source in the case of ANY component.
			name: "a recased ancestor of the source",
			home: func(t *testing.T, src string) string {
				return filepath.Join(recased(t, filepath.Dir(src)), filepath.Base(src))
			},
		},
		{
			// The home CONTAINS the source: the constitution repo root,
			// spelled in another case, with `posse/` inside it. This is the
			// direction underSameDir(a.Home, src) answers and the other
			// three do not — without it the first half of the guard's `||`
			// is unconstrained, which this file's mutation record measured.
			// It is also the live shape of the pre-cutover hazard: promoting
			// into the repo root copies the promoted set beside its own
			// source and prunes there.
			name: "the source inside a recased home",
			home: func(t *testing.T, src string) string {
				return recased(t, filepath.Dir(src))
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, src, _ := promoteFixture(t)
			home := tc.home(t, src)

			// The premise, asserted rather than assumed: the two spellings
			// are one directory, and the STRING answer misses it. Without
			// this the arms could go green on a tree where there was
			// nothing to measure.
			// Either direction counts: two of these arms nest the home
			// inside the source and one nests the source inside the home.
			if !underSameDir(src, home) && !underSameDir(home, src) {
				t.Fatalf("fixture premise broken: %q and %q are neither the same directory nor nested either way", src, home)
			}
			if underDir(home, src) || underDir(src, home) {
				t.Skipf("the string answer already contains these (%q, %q), so this arm measures nothing", home, src)
			}

			a.Home = home
			var b bytes.Buffer
			err := a.CmdPromote(&b, PromoteOpts{Source: src})
			if err == nil {
				t.Fatalf("promote ran with the home spelled as the constitution it promotes from\n"+
					"  source %s\n  home   %s\n  output:\n%s", src, home, b.String())
			}
			// The refusal's own words, so this arm cannot pass because some
			// other gate refused first.
			if !strings.Contains(err.Error(), "the same tree") {
				t.Fatalf("promote refused, but not as the same-tree guard: %v", err)
			}
			// And the self-copy did not happen: no manifest landed in the
			// constitution source, which is what the escape measured.
			if _, e := os.Stat(filepath.Join(src, filepath.Base(a.PromoteManifestPath()))); e == nil {
				t.Errorf("a manifest was written into the constitution source %s", src)
			}
		})
	}
}

// TestQAPromoteStillPromotesToAStrangerHome holds the other direction. An
// identity walk that answered "contained" for every pair would satisfy the
// arms above and brick the command; this is the pin that reds for it.
func TestQAPromoteStillPromotesToAStrangerHome(t *testing.T) {
	t.Parallel()
	a, src, _ := promoteFixture(t)
	// The fixture's own home: a sibling of the constitution repo, which is
	// the ordinary shape and must keep working.
	if underSameDir(a.Home, src) || underSameDir(src, a.Home) {
		t.Fatalf("fixture premise broken: the home %q and the source %q are one tree", a.Home, src)
	}
	out := promote(t, a, PromoteOpts{Source: src})
	if !strings.Contains(out, "promoted") {
		t.Fatalf("a stranger home did not promote:\n%s", out)
	}
}
