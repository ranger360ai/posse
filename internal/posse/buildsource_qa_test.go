//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-eiwhk — the freshness reading finds the checkout
// this binary was BUILT from, on an instance whose `beads:` never names posse.
//
// THE BEAD. `posse gates <persona>` and `posse status` on a fresh home whose
// only bead repo was a project repo read "UNKNOWN whether this binary carries
// the wall renders and the launch-time checks that landed: no checkout of
// github.com/ranger360ai/posse here holds <sha> (looked in <project repo>)",
// with a posse checkout holding exactly that sha in the account's src dir —
// the one `make install` had been run from. The candidate list was `beads:`
// plus the cwd, and on any instance whose crew works on something other than
// posse neither of those can ever name a posse checkout, so the row could
// never be green: a permanent abstention, same class as ranger-base-ctath's
// release installs by a different route.
//
// WHAT THESE HOLD, and what they cannot. The composition and the ORDER are
// here, because the order is the whole safety argument (the stamp is LAST, so
// it can only turn an UNKNOWN into an answer and can never move a reading
// another candidate settled). The reading end to end is here too, through
// FindLauncher, which is the seam — every in-package reading that goes through
// VersionString() abstains before the candidate scan is reached, because a
// test binary carries no vcs stamp at all (cagestale.go's header).
//
// So two things live elsewhere, deliberately:
//   - that the SYMBOL the build lines stamp is a real one, and that the
//     release build does not stamp it: internal/treepins/buildsource_qa_test.go
//     against the build lines' text, and cmd/posse/gatesbuildsource_qa_test.go
//     which builds a stamped binary and reads `posse gates` back. An -X at a
//     symbol that does not exist is SILENT, so neither half can be dropped.
//   - that `posse gates` reads this list at all. Its reading used to compose
//     a second copy of the candidate list, which is how this bead would have
//     been fixed on `posse status` and left open on the surface it was filed
//     from; the arm that holds the two together is the built-binary one.

import (
	"strings"
	"testing"
)

// The ORDER, over all five sources at once. Every claim in this list is
// load-bearing somewhere:
//
//	extra first      — `posse promote` passes the source it is promoting FROM,
//	                   the most specific thing in hand at that moment
//	posse_repo next  — the operator typed it, so it outranks a queue list
//	                   that happens to mention a clone (PosseRepo's comment)
//	beads, then cwd  — unchanged, and the cwd still only for the two surfaces
//	                   an operator typed at (launcherlaggov_qa_test.go holds
//	                   the governance reading off it)
//	the stamp LAST   — so appending it cannot change a reading any earlier
//	                   candidate already settled, which is what makes stamping
//	                   a path into a binary safe (buildsource.go)
func TestQATheBuildStampIsTheLastLauncherCandidate(t *testing.T) {
	t.Parallel()
	got := launcherCandidateList(
		[]string{"/promote/src"}, "/cfg/posse", []string{"/beads/one", "/beads/two"},
		"/the/cwd", "/built/here")
	want := []string{"/promote/src", "/cfg/posse", "/beads/one", "/beads/two", "/the/cwd", "/built/here"}
	if len(got) != len(want) {
		t.Fatalf("candidates = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("candidate %d = %q, want %q (whole list %q)", i, got[i], want[i], got)
		}
	}
	// An absent source contributes NOTHING, not an empty candidate — the
	// unstamped build is the common case (every build but `make build` and
	// `make install`), and an empty entry in the list would put an empty
	// path in front of the reading's own "looked in" sentence.
	if got := launcherCandidateList(nil, "", []string{"/beads/one"}, "", ""); len(got) != 1 || got[0] != "/beads/one" {
		t.Fatalf("an unstamped, unconfigured, cwd-less reading has candidates %q, want just the queue", got)
	}
	// With ONE exception, kept from before this function existed: BeadsDirs'
	// own cwd fallback is a single "" entry when the key is absent, and
	// FindLauncher skips it. Dropping it here would be a behaviour change
	// smuggled in as tidying.
	if got := launcherCandidateList(nil, "", []string{""}, "", ""); len(got) != 1 || got[0] != "" {
		t.Fatalf("BeadsDirs' cwd-fallback entry was dropped: %q", got)
	}
}

// THE BEAD, at the seam: the queue repo is not posse, so the reading has
// nothing to count against — until the binary says where it was built.
//
// The control is the same reading with the stamp empty, which is what every
// build but `make build`/`make install` carries and what this box read before
// the fix. Without it the arm proves only that FindLauncher can count.
func TestQAABuildSourceStampAnswersWhereBeadsNamesNoPosseCheckout(t *testing.T) {
	// Not parallel: BuildSource is a linker-stamped package var, and the
	// whole point of the arm is to set it.
	b, _ := newTestBackend(t)
	project, _ := posseRig(t, "github.com/example/a-project-this-crew-works-on")
	writeBeadsDirs(b.App, []string{project})
	repo, stamp := posseRig(t, posseModule)
	landCommits(t, repo, 3)
	version := "0.5.3+" + stamp

	// THE CONTROL — the bead's own line, including the two things an
	// operator needed and did not get: that the one directory it looked in
	// was not a posse checkout at all, and therefore that the answer was
	// never going to arrive from `beads:`.
	before := FindLauncher(version, b.App.launcherCandidates(false))
	if before.Known() {
		t.Fatalf("the unstamped reading counted %d against the project repo — this fixture cannot show the bead", before.Behind)
	}
	for _, want := range []string{posseModule, stamp, AbbrevHome(project), "not a posse checkout"} {
		if !strings.Contains(before.Why, want) {
			t.Fatalf("the abstention does not name %q, so a reader cannot tell a misconfigured queue from an absent repo: %s", want, before.Why)
		}
	}

	defer func(s string) { BuildSource = s }(BuildSource)
	BuildSource = repo

	l := FindLauncher(version, b.App.launcherCandidates(false))
	if !l.Known() {
		t.Fatalf("still not counted with the build source stamped — this IS ranger-base-eiwhk: %s", l.Why)
	}
	if l.Behind != 3 {
		t.Fatalf("behind = %d, want 3 (%s)", l.Behind, l.Line())
	}
	if l.Repo != repo {
		t.Fatalf("counted in %q, want the checkout the binary was built in (%q)", l.Repo, repo)
	}
	// The governance reading, which is the one that was silent: a condition
	// is about the fleet, and a fleet whose launcher cannot be read raises
	// nothing (GovRows abstains). A stamp that only reached the status line
	// would leave G11 permanently dark on exactly these instances.
	if rows := l.GovRows(1); len(rows) != 1 || rows[0].ID != "G11" {
		t.Fatalf("the stamped reading raises no G11 row past the threshold: %+v", rows)
	}
}

// A STALE STAMP DEGRADES, it does not lie. The checkout moved, was deleted,
// or was replaced by something else — three shapes, one requirement: the
// reading goes back to the abstention it had before the stamp existed, and
// never to a number counted somewhere else.
//
// This is the whole reason a path baked into a binary is safe to read at all,
// and it is a property of FindLauncher rather than of the stamp: a candidate
// is tested (a git checkout, declaring the module, holding the commit), never
// trusted.
func TestQAAStaleBuildSourceStampCannotProduceANumber(t *testing.T) {
	repo, stamp := posseRig(t, posseModule)
	landCommits(t, repo, 2)
	gone := repo + "-removed"
	other, _ := posseRig(t, "github.com/example/something-else")
	stale, _ := posseRig(t, posseModule) // posse, but a history without our stamp

	for _, c := range []struct {
		name, source, wantIn string
	}{
		{"the checkout is gone", gone, "not a git checkout"},
		{"something else is there now", other, "not a posse checkout"},
		{"a posse clone that never had this commit", stale, "does not hold that commit"},
	} {
		t.Run(c.name, func(t *testing.T) {
			defer func(s string) { BuildSource = s }(BuildSource)
			BuildSource = c.source
			l := FindLauncher("0.5.3+"+stamp, launcherCandidateList(nil, "", nil, "", BuildSource))
			if l.Known() {
				t.Fatalf("a stale stamp produced a number: behind=%d in %q", l.Behind, l.Repo)
			}
			// And it says which of the three it was, or the operator cannot
			// tell "your stamp is stale" from "this binary was never
			// stamped" — one is fixed by reinstalling, the other by nothing.
			if !strings.Contains(l.Why, c.wantIn) {
				t.Fatalf("the abstention does not say %q: %s", c.wantIn, l.Why)
			}
			if !strings.Contains(l.Why, AbbrevHome(c.source)) {
				t.Fatalf("the abstention does not name the stamped directory it rejected: %s", l.Why)
			}
		})
	}
}

// `posse_repo:` is the operator's answer, for the install the stamp cannot
// name — a binary copied in from another box, a release tarball beside a
// checkout that should be counted — and it is read AHEAD of `beads:`.
func TestQAPosseRepoIsReadForTheReadingAndAheadOfTheQueues(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	project, _ := posseRig(t, "github.com/example/a-project-this-crew-works-on")
	writeBeadsDirs(b.App, []string{project})
	repo, stamp := posseRig(t, posseModule)
	landCommits(t, repo, 5)
	appendConfig(t, b.App, "posse_repo: "+repo+"\n")

	if got := b.App.PosseRepo(); got != repo {
		t.Fatalf("PosseRepo() = %q, want %q", got, repo)
	}
	c := b.App.launcherCandidates(false)
	if len(c) == 0 || c[0] != repo {
		t.Fatalf("candidates = %q, want the configured repo first — a dial that loses to whatever `beads:` lists first has no authority", c)
	}
	l := FindLauncher("0.5.3+"+stamp, c)
	if !l.Known() || l.Behind != 5 || l.Repo != repo {
		t.Fatalf("the configured checkout was not counted: behind=%d repo=%q why=%q", l.Behind, l.Repo, l.Why)
	}
	// Unset is unchanged, which is what keeps every instance that needs
	// nothing from having to say so.
	b2, _ := newTestBackend(t)
	writeBeadsDirs(b2.App, []string{project})
	if got := b2.App.PosseRepo(); got != "" {
		t.Fatalf("PosseRepo() on an instance that never set it = %q, want empty", got)
	}
	if c := b2.App.launcherCandidates(false); len(c) != 1 || c[0] != project {
		t.Fatalf("an unconfigured instance's candidates = %q, want just its queue", c)
	}
}

// ONE LIST, which is the half that would otherwise have been fixed twice and
// half-landed. `(*App).WallRenderer` composed its own copy of the candidates,
// and `posse gates` — the surface the bead was filed from — is that copy.
// Here: the caller's `extra` still leads, and the list it gets is the same
// function the status line reads.
func TestQATheWallRendererReadsTheSameCandidateList(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)
	project, _ := posseRig(t, "github.com/example/a-project-this-crew-works-on")
	writeBeadsDirs(b.App, []string{project})

	withExtra := b.App.launcherCandidates(true, "/promote/source")
	if len(withExtra) < 2 || withExtra[0] != "/promote/source" {
		t.Fatalf("candidates = %q, want the caller's directory first: `posse promote` counts against the tree it is promoting FROM", withExtra)
	}
	plain := b.App.launcherCandidates(true)
	if len(withExtra) != len(plain)+1 {
		t.Fatalf("extra added %d candidates, want exactly one (%q vs %q)", len(withExtra)-len(plain), withExtra, plain)
	}
	for i := range plain {
		if plain[i] != withExtra[i+1] {
			t.Fatalf("the two lists differ past the extra: %q vs %q", plain, withExtra)
		}
	}
}
