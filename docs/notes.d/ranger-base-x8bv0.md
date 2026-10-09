## A page to read is a URL; material to send is an embed (ranger-base-x8bv0)

The other half of github issue #4. ranger-base-mhv7j fixed the half that is
material to SEND — the upstream filings are embedded and `posse runtime
filing <name>` hands them over — and named what it did not close: three
repo-relative doc paths a shipped binary still printed as the whole remedy,
on a box whose only posse is `bin/posse`, `README.md` and `INSTALL.md`
(MEASURED 2026-10-09, docs/notes.d/ranger-base-mhv7j.md).

### The decision

Two doors, one remedy, and it is not a second embed plus a second verb.
A filing needs the bytes on the operator's disk because they forward them
upstream; an authoring page needs nothing but a reader who can reach it, and
both pages are already on the public remote. So the citation becomes a URL.

MEASURED 2026-10-09, fetched with no credentials, `github.com/ranger360ai/posse`:

| URL | renders |
|---|---|
| `…/blob/main/etc/herdr/agent-detection/README.md` | "herdr agent-detection overrides" |
| `…/blob/main/docs/runbooks/agent-detection-manifest.md` | "Authoring an agent-detection manifest" |

Nothing is published by this. `docs/runbooks` is a genre ADR 0024 D2 admits
to the public tree (`PublicDocsGenres`, visibility.go) and `etc/` is not on
the publication exclusion list (`seedpub_qa_test.go`); both files are on
`origin/main` today. The visibility rule asks whether something moves to a
wider audience than its source had, and a link to the public copy of a
public file moves nothing.

### Why the branch name is pinned rather than avoided

`blob/main/<path>` is already the house spelling — www/index.html ships two
and `internal/treepins/quickstart_test.go` pins them — and `blob/HEAD`,
which GitHub resolves to the default branch for free, would have bought a
second spelling of one fact. What makes `main` safe is that the URL is not a
literal: `publicDoc(rel)` composes it from the repo-relative path the door
would otherwise have printed, so each page is spelled ONCE
(`detectionFilingDoc`, `detectionDocPath`) and every surface inherits it.
`detectiondoor_qa_test.go` stats that path in the tree and compares the
rendered URL to the base, so a page that is renamed or moved reds in
`make doc-check` instead of 404ing on an operator's laptop.

### The exclusion line that was the real finding

`detectiondoor_qa_test.go`'s carried-by-the-binary reading excused
`README.md` by BASENAME. That was correct about the file — it is not a
filing and must not be embedded — and it excused the defect along with it:
the one door whose citation was a dead end was the one citation the check
did not read. It is spelled by how the reader REACHES a citation now. A
citation under `publicDocBase` is a page to read and need not be carried; a
bare path must be carried; both must be in the tree. Nothing is excluded by
name, so the next page a door cites has to pick one and cannot be quiet.

Verified by restoring the defect: with one call site reverted to the bare
`detectionFilingDoc`, the pin reds on all four surfaces for claude, codex
and grok and names the URL to use. The classifier has a control arm of its
own (eight sentences, both families, both directions, plus a fork's blob URL
and a truncated base) because a classifier that called everything a URL
would excuse every bare path in the tree — the mhv7j defect, restored and
green.

### What it does not reach

A bare path written into a sentence that is not a `DetectionDoor` surface.
A census over shipped string literals naming `docs/` is not precise enough
to be a pin: MEASURED 2026-10-09, an ast walk over every non-test string
literal under `internal/` and `cmd/` naming `docs/runbooks`, `docs/adr` or
`etc/herdr` returns 32KB — hook bodies, globs and regexps, nearly all of
them legitimate and none of them read by an operator. What holds instead is
the single spelling: one path const per page, reaching every sentence
through `publicDoc`. The probe's own `agent_not_found` line is pinned where
it is rendered (`runtimeprobe_test.go`, arm 3), which is the third surface
the bead named.
