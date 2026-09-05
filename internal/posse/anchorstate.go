package posse

// The anchor-state line (ADR 0015 §3, ranger-base-zio33 → ranger-base-xevp7).
//
// promoted.json is a trust anchor stored inside the tree it attests, and an
// ABSENT manifest is by design not a mismatch: ReadPromoteManifest reads a
// missing file as (nil, nil) and VerifyPromoted reads that as OK, because
// every pre-0015 home and every RHQ_HOME test rig must keep launching
// (pinned, TestVerifyPromotedIsSilentWithNoManifest). The consequence zio33
// weighed and accepted: until this file, an anchor that had been deleted was
// invisible on every surface, forever — the manifest's only readers are
// promote.go, init.go, seatbelt.go and the launch verify, and none of them
// says anything when there is nothing to say.
//
// This is the whole of what zio33 bought instead of a second anchor: one
// read-only line, printed once at the watch preamble beside ReportHookWall,
// naming which of the three states the home is in.
//
// What it does NOT buy, stated here so a later edit has to argue with it:
//
//   - Nothing against a deliberate session. A session that re-stamps the
//     manifest with its own tampered file's sha256 (zio33 case D, MEASURED)
//     leaves a home that reads `promoted` here and verifies clean at launch.
//     Case D subsumes case C — a session that would delete the anchor can
//     re-stamp it instead — so no presence rule, this line included, catches
//     a session that means it. Adversary-proof anchoring requires leaving the
//     uid, which ADR 0015's Alternatives rejects.
//   - No launch behavior. This reads the manifest and prints; it refuses
//     nothing, degrades nothing, and never fires on absence. The (nil, nil)
//     branch stays exactly as load-bearing as it was.
//
// What it does buy: an ACCIDENTAL deletion — a cleanup script, a botched
// restore, an `rm -rf` one directory up — is visible at the operator's next
// touch point instead of never.

import (
	"fmt"
	"io"
)

// AnchorStateLine renders the home's anchor state from what
// ReadPromoteManifest returned. Split from the read so all four states pin
// without a home on disk to build them.
//
// A manifest is `seeded` on Seeded alone, never on the absence of a SHA: a
// seeded manifest is a real anchor with no commit behind it (SeedPromoteManifest
// writes no SHA and never will), while a promote that could not name its
// commit is still a promotion and must not be reported as a fresh install's
// stamp.
func AnchorStateLine(m *PromoteManifest, err error) string {
	switch {
	case err != nil:
		// An unreadable manifest already degrades where it matters — the
		// launch verify carries the error into its verdict, and promote says
		// it is promoting without a baseline. Here it is one line, because a
		// preamble that printed nothing for it would report the one state
		// that is neither of the other two as if it were fine.
		return fmt.Sprintf("constitution: %v", err)
	case m == nil:
		return "constitution: never promoted — no " + PromoteManifestFile
	case m.Seeded:
		return "constitution: seeded " + anchorWhen(m)
	case m.SHA != "":
		return fmt.Sprintf("constitution: promoted %s %s", short(m.SHA), anchorWhen(m))
	default:
		return fmt.Sprintf("constitution: promoted %s — the manifest records no commit", anchorWhen(m))
	}
}

// anchorWhen is the manifest's own timestamp, verbatim (RFC3339, as both
// writers stamp it) — the same value init quotes when it refuses a promoted
// home, so one grep finds both. A manifest written without one says so
// rather than rendering a blank where a date belongs.
func anchorWhen(m *PromoteManifest) string {
	if m.PromotedAt == "" {
		return "(no date recorded)"
	}
	return m.PromotedAt
}

// ReportAnchorState prints the line for this home. Read-only: it opens the
// manifest and nothing else, and returns nothing for a caller to branch on —
// there is deliberately no verdict here to act upon.
func (a *App) ReportAnchorState(w io.Writer) {
	m, err := ReadPromoteManifest(a.PromoteManifestPath())
	fmt.Fprintln(w, AnchorStateLine(m, err))
}
