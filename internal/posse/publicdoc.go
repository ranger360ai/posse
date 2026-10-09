package posse

// Where a reader with NO CHECKOUT reads this repo's docs (ranger-base-x8bv0,
// github issue #4's second half).
//
// A door that hands the operator a repo-relative path is true of the
// repository and false of every artifact the repository publishes: the
// release tarball and the Homebrew keg carry `posse` plus README.md and
// INSTALL.md, nothing else (MEASURED 2026-10-09,
// docs/notes.d/ranger-base-mhv7j.md). ranger-base-mhv7j fixed that for
// MATERIAL TO SEND — the upstream filings are embedded and `posse runtime
// filing <name>` hands them over, because the operator has to forward those
// bytes. It left the PAGES TO READ, which need no bytes: the reader has to
// get to the page, and the page is already published.
//
// So the remedy here is a URL and not a second embed plus verb. Both pages
// were MEASURED 2026-10-09 to render at this base on a fetch with no
// credentials — `etc/herdr/agent-detection/README.md` ("herdr
// agent-detection overrides") and `docs/runbooks/agent-detection-manifest.md`
// ("Authoring an agent-detection manifest") — so nothing moves to a wider
// audience than it already has: this points at the public copy of a file
// that is already on the public remote, which is the visibility rule's
// whole question (NOTES.md "Privacy model", ADR 0024 D2 — `runbooks` is a
// published genre and `etc/` is not excluded).
//
// THE BRANCH NAME IS PINNED, not avoided. `blob/main/<path>` is the house
// spelling already — www/index.html ships two of them and
// internal/treepins/quickstart_test.go pins them — and matching it is worth
// more than `blob/HEAD`, which would resolve the default branch for free
// and leave two spellings of one fact in the tree. What makes it safe is
// that the URL is NOT a literal: it is composed from the repo-relative path
// the door would otherwise have printed, so one pin
// (detectiondoor_qa_test.go) can stat that path in the tree and compare the
// rendered URL against the base. A page that is renamed or moved reds here
// before it 404s on an operator's laptop.
//
// A citation under this base is therefore a page to READ. It is deliberately
// NOT a filing: `detectiondoor_qa_test.go`'s carried-by-the-binary reading
// used to excuse `README.md` by its BASENAME, pending this bead; it now
// excuses any citation spelled as one of these URLs, and requires the rest
// to be carried. Which is the same line of reasoning in both directions —
// a path a reader cannot reach is a dead end whichever half of it is wrong.
const publicDocBase = "https://github.com/ranger360ai/posse/blob/main/"

// publicDoc renders the URL for a repo-relative doc path. A function rather
// than a const per page so that the path stays spelled ONCE, as the thing
// the pin stats.
func publicDoc(rel string) string { return publicDocBase + rel }
