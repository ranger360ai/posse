// Package posse is the module root, and exists for one reason: go:embed
// cannot reach out of its own directory, and the seed tree it must carry —
// examples/ — belongs at the repo root where deployers read it (ADR 0012 D1).
//
// Seed is what `posse init` copies into a fresh RHQ_HOME. Embedding it is what
// lets a release binary seed an instance with no repo beside it (ADR 0012 D5:
// "public repo + release binary with embedded examples"). The on-disk tree
// still wins when the binary is run out of a checkout — see internal/rhq's
// seedSource.
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
