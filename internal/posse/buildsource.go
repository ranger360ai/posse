package posse

// Where this binary was BUILT, and why the freshness reading could not find
// its own checkout without being told (ranger-base-eiwhk, found on the
// ADR 0080 host in a fresh home whose only bead repo was a project repo).
//
// THE SYMPTOM. `posse gates <persona>` and `posse status` read
//
//	wall renderer (gates) · 0.5.3+<sha> · UNKNOWN whether this binary carries
//	the wall renders and the launch-time checks that landed: no checkout of
//	github.com/ranger360ai/posse here holds <sha> (looked in <project repo>)
//
// on a box where a posse checkout holding exactly that sha was sitting in the
// account's src dir — the one `make install` had been run from minutes
// earlier. Nothing was missing and nothing was stale. The reading simply
// never looked there.
//
// WHY. The candidate list the reading scans (launcherlag.go
// launcherCandidates) was `beads:` plus the process cwd, and both of those
// answer "which repos does this instance DISPATCH" — a question with no
// reason to mention posse. On the laptop this crew was built on they coincide,
// because posse's own repo is one of the queues; on every instance whose crew
// works on something else, which is the normal case off the laptop, the posse
// checkout is on disk and in no list the reading reads. So the provenance row
// could never be green there, on any pass, forever — the same permanent
// abstention ranger-base-ctath fixed for release installs, reached by a
// different route, and with the checkout right there.
//
// TWO ANSWERS, AND NEITHER IS A NETWORK CALL OR A GUESS.
//
//	posse_repo:   the operator's answer. One path, read ahead of `beads:`,
//	              for the instance that keeps a posse checkout somewhere this
//	              reading would otherwise have to find by luck.
//	BuildSource   the BUILD's own answer, and the one that needs no
//	              configuration at all: `make build` and `make install` know
//	              the directory they are building from, so they stamp it
//	              beside the sha, and the binary carries where it came from.
//
// WHY A STAMP IS SAFE HERE, which is the whole of why this is not a guess:
// FindLauncher does not trust a candidate, it tests one. A stamped directory
// still has to be a git checkout, still has to declare the posse module, and
// still has to HOLD the commit the binary names, or the scan walks past it.
// So a stamp that has gone stale — the checkout moved, deleted, or replaced
// by an unrelated tree — cannot produce a wrong number; it produces the
// abstention that was there before. The stamp can only ever turn an UNKNOWN
// into an answer, which is also why it is the LAST candidate: appending it
// cannot change a reading that any earlier candidate already settled.
//
// AND IT IS NOT STAMPED ON A RELEASE. A release tarball is cut from a
// throwaway worktree on a machine the reader does not have (releasestamp.go),
// so the path would be dead on arrival, and a published binary should not
// carry a stranger's home directory. A release's answer is its TAG, which it
// already has. Nothing in scripts/release-artifacts.sh names the symbol below
// and internal/treepins/buildsource_qa_test.go holds that shut, both ways:
// `go build -ldflags "-X <symbol>=<value>"` on a symbol that does not exist
// is SILENT (exit 0, the variable keeps its default — cagestale_test.go
// measured it for the Build stamp), so a rename here with the build lines
// left alone would bring the defect straight back with the suite green.

import "strings"

// BuildSource is the checkout this binary was built from, stamped beside
// Build via -ldflags by the Makefile (`make build`) and by
// scripts/clean-build.sh (`make release`, and so `make install`) — and by
// nothing else in the tree.
//
// Empty for every other build, which is the honest answer rather than a
// defaulted one: a release tarball, a `go install …@latest`, a CI build and
// the cage's inner binary were none of them built in a directory the reader
// can look in, and an empty stamp adds no candidate at all.
//
// An absolute path on the machine that did the build. It is read as a
// CANDIDATE and never as a fact — see the header.
var BuildSource = ""

// BuildSourceSymbol is the linker symbol BuildSource is stamped through,
// spelled once here so the build lines can be pinned against the GO name
// instead of against a second copy of the string (ReleaseTagSymbol's reason,
// one file over: an -X at a symbol that does not exist is silent).
const BuildSourceSymbol = "github.com/ranger360ai/posse/internal/posse.BuildSource"

// PosseRepo is the checkout the operator names for this reading
// (`posse_repo:`), expanded and empty when unset.
//
// It is read AHEAD of `beads:` rather than appended after it, because it is
// the one candidate that was typed on purpose: an instance that holds two
// clones of posse and sets this key is answering the question FindLauncher's
// own header declines to answer for it ("the stamp separates unrelated
// histories, not two copies of one — for that the order is the operator's").
// A key whose value loses to whatever `beads:` happens to list first would be
// a dial with no authority.
//
// Not validated here. A path that is not a posse checkout holding this
// binary's commit is skipped by the scan like any other candidate, and the
// abstention NAMES every candidate it rejected (FindLauncher) — which is the
// half that makes a mistyped value visible instead of silently inert.
func (a *App) PosseRepo() string {
	return ExpandTilde(strings.TrimSpace(a.CfgGet("posse_repo", "")))
}
