//go:build posse_arm3

package posse

// QA pins for ranger-base-w79sc — ADR 0082 D4: the path ADR 0081 D7's
// queue-commit refusal prescribes is spelled the way the kernel names the
// directory, however `queue_repo:` spelled it.
//
// WHAT WAS WRONG. The refusal printed `AbbrevHome(hookRepo(q))`, which is
// `queue_repo:` as the operator's config spelled it, and it printed the
// owning home from the wall's `# posse-home:` stamp — which ADR 0082 D3 had
// already made canonical. So ONE line showed two paths in two spellings, and
// the `beads_visibility:` key it told the operator to paste was a function of
// how their config happened to spell the queue repo rather than of the
// directory it names.
//
// WHY A SECOND SPELLING PIN BESIDE 8kwa5's, and it is not the same claim.
// ranger-base-8kwa5 pinned that the prescribed key is ABSOLUTE, and its arm
// asserts a PROPERTY rather than a string on purpose — it was written while
// "should this site canonicalise too?" was still open (ranger-base-wvoqe),
// and a pin demanding one exact spelling would have fenced out the better
// fix. D4 answered yes, so canonicalness is now a claim somebody has to
// make, and these arms make it: the repo the refusal names and the key it
// prescribes are both EQUAL to CanonicalAbbrevPath of the repo, and the
// spelling the config carried appears nowhere. Both pins hold; neither
// subsumes the other, and a revert of either is red in one file only.
//
// WHY A CASE ARM AND A SYMLINK ARM. They separate the two halves of
// canonicalPath (ADR 0082 D1), which is the ADR's own reason for rejecting
// its alternative (a): a render that only ran EvalSymlinks would pass the
// symlink arm and red the case arm, because EvalSymlinks keeps the caller's
// case. So the pair measures the rejection rather than restating it.
//
// WHAT THESE ARMS DO NOT TOUCH, so nobody reads them as a widening: the
// VALUE. `q` is `QueueRepo()`'s, unchanged, and every predicate this close
// asks — underSameDir, OwnsCommitWall, InstallCommitGuardHook,
// ForeignCommitWallOwner — still reads the spelling the config gave. Each
// arm asserts that premise directly (`QueueRepo()` is still the config's
// spelling) so that a future pass which "fixed" this by canonicalising the
// value reds here, carrying D4's reason, rather than passing quietly.
//
// MUTATION-CHECKED 2026-11-08, darwin 25.6.0 (go1.26.5 darwin/arm64, APFS
// case-insensitive), each mutant run over these two names alone:
//
//   - reverting queuejsonl.go's render to AbbrevHome (D4 undone) reds BOTH
//     arms, three assertions each: the repo the refusal names, the key it
//     prescribes, and the config's own spelling still being there. The key
//     it printed in the case arm was the config's `…/001/REPO` where the
//     repo on disk is `…/001/Repo`.
//   - making kernelPath return an error on darwin (D1's platform half
//     undone, so canonicalPath degrades to resolvedPath — exactly ADR
//     0082's rejected option (a)) leaves the SYMLINK arm green and reds the
//     CASE arm AT ITS PREMISE: with EvalSymlinks alone the two spellings
//     canonicalise to two strings (`…/REPO` and `…/Repo`), so there is no
//     single answer for the render to print and the arm says so rather than
//     reporting a refusal as wrong. That is the ADR's reason for rejecting
//     (a), measured here at the refusal the way canonicalwall_qa_test.go
//     measures it at the wall.
//   - canonicalising inside `QueueRepo()` instead (D4's rejected
//     alternative) reds the PREMISE of both arms and nothing else: the
//     refusals it prints are right, which is exactly why the premise is
//     asserted — the defect that alternative carries is in the three
//     comparisons downstream, where no arm here can see it.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// qrsPrescribedKey captures the `beads_visibility:` key D7 tells the reader
// to add. Read back out of the message rather than assumed, because the whole
// finding is that this one substring was the config's spelling: a test that
// only asked whether the message contains SOME canonical path would pass on
// the home stamp, which was canonical before this bead.
var qrsPrescribedKey = regexp.MustCompile("add `([^`]+): <private\\|public>`")

// qrsRefusal is one refused close through a `queue_repo:` spelled `spelling`,
// and the refusal it printed. Everything else is qwo*'s fixture: a repo this
// home does NOT own (D1) whose wall a second home rendered and stamped, which
// is the one state D7 refuses.
func qrsRefusal(t *testing.T, repo, spelling string) string {
	t.Helper()
	store := filepath.Join(repo, ".beads") // spelled as it is on disk
	a := qwoApp(t, repo, false)
	x, slot := qwoForeignWall(t, repo, a)
	beforeBody, beforeMtime := qwoSlot(t, slot)

	// Rewritten rather than parameterised into qwoApp: the non-owning
	// listing has to stay exactly what qwoApp wrote, or these arms would be
	// measuring ownership instead of spelling.
	cfg, err := os.ReadFile(a.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	rel := strings.Replace(string(cfg), "queue_repo: "+repo+"\n", "queue_repo: "+spelling+"\n", 1)
	// A Replace that matched nothing would leave the canonical spelling in
	// place and make every assertion below pass vacuously.
	if rel == string(cfg) {
		t.Fatalf("fixture premise: no `queue_repo: %s` line to rewrite, so this arm would measure the canonical spelling:\n%s", repo, cfg)
	}
	write(t, a.ConfigPath, rel)

	// THE PREMISE, in both directions. The value is the config's spelling
	// and is NOT canonicalised (D4: the render happens at the render point,
	// never at the value) — and that spelling genuinely differs from the
	// canonical one, or there would be nothing for the render to fix.
	if got := a.QueueRepo(); got != spelling {
		t.Errorf("QueueRepo() = %q, want the config's own spelling %q — ADR 0082 D4 keeps this a VALUE that feeds underSameDir, OwnsCommitWall and InstallCommitGuardHook, and canonicalising it moves three comparisons", got, spelling)
	}
	if canonicalPath(spelling) == spelling {
		t.Fatalf("fixture premise: %q is already its own canonical spelling, so this arm cannot tell a rendered path from an unrendered one", spelling)
	}
	if canonicalPath(spelling) != canonicalPath(repo) {
		t.Fatalf("fixture premise: %q and %q canonicalise to %q and %q — two directories, so the refusal is right either way",
			spelling, repo, canonicalPath(spelling), canonicalPath(repo))
	}

	closeBead(t, store, `{"id":"q-1","title":"closed"}`)
	c, cerr := a.CommitQueueJSONL(NewBd(), qWork(t, store), "beads: q-1 closed by developer")
	if cerr == nil {
		t.Fatalf("a close through another home's wall committed: %+v", c)
	}
	// The refusal still does everything D7's own arm asserts — a rendered
	// path must not have turned the rule into a different answer.
	if afterBody, afterMtime := qwoSlot(t, slot); afterBody != beforeBody || !afterMtime.Equal(beforeMtime) {
		t.Errorf("the refused commit wrote another home's wall: bytes equal=%v, mtime %s -> %s", afterBody == beforeBody, beforeMtime, afterMtime)
	}
	if !strings.Contains(cerr.Error(), qaStampedBy(x.Home)) {
		t.Errorf("the refusal no longer names the stamping home %s: %v", qaStampedBy(x.Home), cerr)
	}
	return cerr.Error()
}

// qrsAssert is the claim: both places the refusal prints the repo carry the
// kernel's spelling, and the config's spelling is in neither.
func qrsAssert(t *testing.T, text, repo, spelling string) {
	t.Helper()
	want := CanonicalAbbrevPath(repo)
	if !strings.Contains(text, want) {
		t.Errorf("the refusal does not name the repo as %s, which is how the identity wall in that same repo spells it:\n%s", want, text)
	}
	key := qrsPrescribedKey.FindStringSubmatch(text)
	if key == nil {
		t.Fatalf("D7's remedy no longer prescribes a `<repo>: <private|public>` key, which is the whole of what makes this refusal actionable:\n%s", text)
	}
	if key[1] != want {
		t.Errorf("the remedy prescribes the key %q, want %q — the spelling the identity wall of that repo already prints, so an operator who greps for one finds one:\n%s", key[1], want, text)
	}
	// And the config's own spelling appears NOWHERE. The two assertions
	// above would both pass on a message that printed both spellings.
	if strings.Contains(text, AbbrevHome(spelling)) {
		t.Errorf("the refusal still carries the config's spelling %s beside the canonical one:\n%s", AbbrevHome(spelling), text)
	}
}

// A case-changed `queue_repo:` — ranger-base-99gww's axis, the one
// EvalSymlinks cannot close and the one ADR 0082 D1 opens the path for
// F_GETPATH to close.
func TestQAAQueueRefusalPrescribesTheKernelsCaseNotTheConfigs(t *testing.T) {
	repo := qcaseRepo(t, "Repo")
	recasedRepo := recased(t, repo)
	qrsAssert(t, qrsRefusal(t, repo, recasedRepo), repo, recasedRepo)
}

// A symlinked `queue_repo:` — the axis every platform's half of
// canonicalPath closes, so this arm runs where the case arm skips.
func TestQAAQueueRefusalPrescribesTheRepoNotTheSymlinkItWasReachedBy(t *testing.T) {
	repo := qcaseRepo(t, "Repo")
	link := filepath.Join(gitTempDir(t), "link")
	if err := os.Symlink(filepath.Dir(repo), link); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	viaLink := filepath.Join(link, filepath.Base(repo))
	qrsAssert(t, qrsRefusal(t, repo, viaLink), repo, viaLink)
}
