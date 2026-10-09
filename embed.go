// Package posse is the module root, and exists for one reason: go:embed
// cannot reach out of its own directory, and the trees it must carry —
// examples/ and etc/herdr/agent-detection/ — belong at the repo root where
// deployers read them (ADR 0012 D1).
//
// Seed is what `posse init` copies into a fresh RHQ_HOME. Embedding it is what
// lets a release binary seed an instance with no repo beside it (ADR 0012 D5:
// "public repo + release binary with embedded examples"). The on-disk tree
// still wins when the binary is run out of a checkout — see internal/posse's
// seedSource.
//
// Filings is the other half of the same rule, arrived at the same way: a
// release binary whose own refusal names a repo file is a refusal with no
// door on a `brew install` (ranger-base-mhv7j). Its directives are below.
package posse

import (
	"embed"
	"io/fs"
)

//go:embed all:examples
var examplesFS embed.FS

// Seed is examples/, rooted at its own directory (no "examples/" prefix on
// the paths inside it), so a caller can treat it and os.DirFS(<checkout>/
// examples) as the same shape.
var Seed fs.FS

func init() {
	sub, err := fs.Sub(examplesFS, "examples")
	if err != nil {
		// The embed directive above guarantees the subtree: a failure here
		// is a broken build, not a runtime condition worth handling.
		panic(err)
	}
	Seed = sub
}

// ─── the upstream detection filings (ranger-base-mhv7j) ─────────────────────

// filingsFS is etc/herdr/agent-detection/'s FILING PACKAGES — the drafts
// posse's own refusals tell the operator to send upstream, plus the pane
// snapshots that go with them (ADR 0060 D2).
//
// It is embedded for the reason Seed is, and the defect was the same shape
// one surface over. Every door a detection refusal prints named
// `etc/herdr/agent-detection/upstream-bob.md` as a REPO-RELATIVE path —
// "posse ships the filing at <path> and the operator sends it" — and
// scripts/release-artifacts.sh tars the binary and two docs, so on a
// `brew install` (and on a `go install`, which fetches a binary and no repo
// files) the file that sentence named did not exist. The operator could not
// send what he did not have, a dispatched bob seat stayed agent_not_found,
// and the one route out of the refusal was the one route the release did not
// carry — issue #4, measured on the work box 2026-10-08.
//
// Two patterns rather than `all:etc/herdr/agent-detection`, deliberately:
// the directory also holds the local overrides `make install-detection`
// globs and the testdata/ fixtures `make verify-detection` replays, and
// neither is anything a release binary can use. What ships is what a filing
// IS — the covering note and the package beside it.
//
//go:embed etc/herdr/agent-detection/upstream-*.md
//go:embed all:etc/herdr/agent-detection/upstream
var filingsFS embed.FS

// Filings is etc/herdr/agent-detection/, rooted at its own directory (no
// `etc/herdr/agent-detection/` prefix on the paths inside it), so a caller
// holding a DetectionFiling — which is repo-relative, because that is what
// it means in a checkout — reaches it by trimming that prefix and nothing
// else. Same shape contract as Seed.
var Filings fs.FS

func init() {
	sub, err := fs.Sub(filingsFS, "etc/herdr/agent-detection")
	if err != nil {
		// As above: the directives guarantee the subtree.
		panic(err)
	}
	Filings = sub
}
