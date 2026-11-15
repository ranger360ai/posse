//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-xcucy (design ranger-base-dvw0w D1, D3, D4): a
// seat's own object store enters the shared store only by a re-hashing
// import, and a landing refuses a tip whose objects the shared store lacks.
//
// The seat store is SIMULATED, as the design's probe did: the seat's git
// runs with GIT_OBJECT_DIRECTORY=<own git dir>/seat-objects, whose
// info/alternates names the shared store. That is what a walled seat's git
// sees once its `objects` is bound over; no bwrap is needed to pin what the
// launcher does with the result.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Pin (1): forged bytes planted under a PREDICTED name never take that name
// in the shared store, and an honest write of that name afterwards is not
// skipped — it reads the honest bytes.
//
// MUTATION RUN: copying the seat's files into the shared store's objects dir
// instead of the launcher's view (no re-hash) reds this test.
func TestQASeatImportForgedObjectUnderAPredictedNameNeverTakesIt(t *testing.T) {
	t.Parallel()
	_, repo, tr, store := siTree(t)
	honest := "the honest bytes\n"
	claimed := siHashOf(t, repo, honest)
	if siHas(t, repo, claimed) {
		t.Fatal("CONTROL: the shared store already holds the predicted name, so this cannot see a plant take it")
	}
	truth := siPlant(t, tr, store, "the seat's forged bytes\n", claimed)

	if err := importSeatObjects(tr); err != nil {
		t.Fatal(err)
	}
	if siHas(t, repo, claimed) {
		t.Fatalf("the shared store now holds %s, the name the seat CLAIMED for bytes that do not hash to it", claimed[:12])
	}
	if !siHas(t, repo, truth) {
		t.Errorf("the forged bytes did not land under their true name %s — the import did not run, so this pin proved nothing", truth[:12])
	}
	// The honest write of that name, by an uncaged writer, after the import.
	got := siWriteAndRead(t, repo, honest)
	if got != honest {
		t.Fatalf("an honest write of %s after the import reads %q, not the honest bytes", claimed[:12], got)
	}
}

// Pin (2): forged bytes planted under a name the shared store ALREADY holds
// leave its bytes unchanged, and are never even read from the seat.
//
// MUTATION RUN: packing every name the store lists — both missingIn filters
// dropped, the one before the copy and the one before the pack — reds the
// "never read" half. Either filter alone holds it.
func TestQASeatImportForgedObjectUnderAnExistingNameIsNeverRead(t *testing.T) {
	t.Parallel()
	_, repo, tr, store := siTree(t)
	existing := mustGit(t, repo, "rev-parse", "HEAD:README.md")
	truth := siPlant(t, tr, store, "rewritten history\n", existing)

	if err := importSeatObjects(tr); err != nil {
		t.Fatal(err)
	}
	if got := mustGit(t, repo, "cat-file", "-p", existing); got != "seed" {
		t.Fatalf("the shared store's %s now reads %q", existing[:12], got)
	}
	if siHas(t, repo, truth) {
		t.Errorf("the forged bytes reached the shared store under their true name %s — a file under a name the store already holds was read from the seat", truth[:12])
	}
}

// Pin (3a): the splice after the import lands a seat's work whose objects
// were only in its own store.
func TestQASeatImportThenSpliceLandsTheWork(t *testing.T) {
	t.Parallel()
	a, repo, tr, store := siTree(t)
	head := siSeatCommit(t, tr, store, "fix.txt", "the work\n", "s-1: the fix")
	if siHas(t, repo, head) {
		t.Fatal("CONTROL: the seat's commit is already in the shared store, so this cannot see the import")
	}
	o, err := MergeSessionWork(a, tr)
	if err != nil {
		t.Fatal(err)
	}
	if !o.Merged || o.Commits != 1 {
		t.Fatalf("a seat's own-store work did not land: %+v", o)
	}
	if got := mustGit(t, repo, "rev-parse", "main"); got != head {
		t.Fatalf("main is at %s, not the seat's commit %s", got[:12], head[:12])
	}
	if got := mustGit(t, repo, "show", "main:fix.txt"); got != "the work" {
		t.Fatalf("main:fix.txt reads %q", got)
	}
	if missing, err := missingObjects(repo, "main", "main~1"); err != nil || len(missing) != 0 {
		t.Fatalf("the landed tip names objects the shared store lacks: %v %v", missing, err)
	}
}

// Pin (3b): the gate refuses a tip with a missing object and names it. Every
// commit and tree is in the shared store and one blob is not — the shape of
// a store whose import did not happen — so every guard above the gate (the
// count, the branch read) answers and only the gate can say no.
//
// The missing blob is in an INTERMEDIATE commit and deleted by the tip, and
// that is the shape the gate is for: `merge --ff-only` reads only the tip's
// tree to update the checkout, so it refuses a tip whose own tree has a hole
// by itself — and fast-forwards over this one without complaint, putting
// history on main nobody can read (MEASURED: the mutation below lands it).
//
// MUTATION RUN: removing the landingGateReason call from MergeSessionWork
// reds this test (the fast-forward lands an unreadable history).
func TestQASeatImportGateRefusesATipWithAMissingObject(t *testing.T) {
	t.Parallel()
	a, repo, tr, blob := siGateFixture(t)
	before := mustGit(t, repo, "rev-parse", "main")

	o, err := MergeSessionWork(a, tr)
	if err != nil {
		t.Fatal(err)
	}
	if o.Merged {
		t.Fatalf("a tip naming a blob the shared store lacks was landed: %+v", o)
	}
	if !strings.Contains(o.Reason, abbrevSHA(blob)) {
		t.Errorf("the refusal does not name the missing object %s: %s", abbrevSHA(blob), o.Reason)
	}
	if got := mustGit(t, repo, "rev-parse", "main"); got != before {
		t.Fatalf("main moved to %s over a refused landing", got[:12])
	}
}

// Pin (4): every census reader, asked about an UN-imported tip, says nothing
// that would drop the work: never "landed", never "nothing to land", never
// "safe to remove".
//
// MUTATION RUN: restoring treeState's `err == nil &&` collapse (an
// unreadable count read as zero) reds the treeState arm.
func TestQASeatImportReadersOfAnUnimportedTipKeepTheWork(t *testing.T) {
	t.Parallel()
	_, repo, tr, store := siTree(t)
	head := siSeatCommit(t, tr, store, "fix.txt", "the work\n", "s-1: the fix")
	if siHas(t, repo, head) {
		t.Fatal("CONTROL: the commit is in the shared store")
	}

	// SHA-only reads work before the import (design D6).
	if got, ok := workHead(tr); !ok || got != head {
		t.Fatalf("workHead = %q %v, want the seat's tip", got, ok)
	}
	if got, ok := treeDetachedHead(tr.Path); !ok || got != head {
		t.Fatalf("treeDetachedHead = %q %v", got, ok)
	}
	// Object reads degrade toward keeping the work.
	if s := treeState(tr); strings.Contains(s, "nothing unlanded") {
		t.Errorf("treeState says %q over a tip it cannot read", s)
	}
	if nothingToLand(tr) {
		t.Error("nothingToLand is true over a tip it cannot read")
	}
	if h := treeHolds(tr); h == "" {
		t.Error("treeHolds says a removal takes nothing")
	}
	if o := landed(MergeOutcome{Branch: tr.Branch, Base: tr.Base}, tr); o.Merged {
		t.Errorf("landed() reports Merged over an unreadable tip: %+v", o)
	}
	if _, ok := workHeadTime(tr); ok {
		t.Error("workHeadTime claims to read the commit time of a commit the shared store lacks")
	}
	if _, ok := commitsAhead(tr); ok {
		t.Error("commitsAhead claims a count")
	}
	if _, ok := unlandedAhead(tr, head); ok {
		t.Error("unlandedAhead claims a count")
	}
	if reaches(repo, tr.Base, head) {
		t.Error("reaches says the base holds an unreadable tip")
	}
	// The gate itself fails closed on a tip it cannot read.
	if m, err := missingObjects(repo, head, tr.Base); err == nil && len(m) == 0 {
		t.Error("the landing gate passed a tip the shared store does not have")
	}
	// And the destroy refuses unforced on the same tree — before the import
	// would make it answerable, the guard must not read it as empty.
	tips := removalTips(tr)
	var sawHead bool
	for _, tip := range tips {
		if tip.ref == head {
			sawHead = true
			if r, err := heldByTip(tr, tip); err == nil || r {
				t.Errorf("heldByTip(%s) = %v, %v — not a refusal", abbrevSHA(head), r, err)
			}
		}
	}
	if !sawHead {
		t.Errorf("removalTips does not name the seat's tip: %+v", tips)
	}
}

// Pin (5): a retire whose import fails keeps the tree — forced, too — and
// one whose import succeeds removes it with the objects in the shared store.
//
// MUTATION RUN: dropping the importSeatObjects call from RemoveSessionTree
// reds the second half (the commit is gone with the store).
func TestQASeatImportRetireKeepsTheTreeWhenTheImportFails(t *testing.T) {
	t.Parallel()
	_, repo, tr, store := siTree(t)
	siSeatCommit(t, tr, store, "fix.txt", "the work\n", "s-1: the fix")
	// A loose object that is not zlib at all, under a name nobody has.
	bad := strings.Repeat("ab", 20)
	write(t, filepath.Join(store, bad[:2], bad[2:]), "not an object")

	if err := RemoveSessionTree(tr, true); err == nil {
		t.Fatal("a forced retire removed a tree whose store would not import")
	}
	if _, err := os.Stat(tr.Path); err != nil {
		t.Fatalf("the tree is gone: %v", err)
	}
	if _, err := os.Stat(store); err != nil {
		t.Fatalf("the store is gone: %v", err)
	}

	// The same tree with the bad file gone: the import runs, and the forced
	// removal then takes nothing the shared store does not hold.
	if err := os.Remove(filepath.Join(store, bad[:2], bad[2:])); err != nil {
		t.Fatal(err)
	}
	head, _ := workHead(tr)
	if err := RemoveSessionTree(tr, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tr.Path); err == nil {
		t.Fatal("the tree was not removed")
	}
	if !siHas(t, repo, head) {
		t.Fatalf("the removal took the seat's commit %s with its store", abbrevSHA(head))
	}
}

// `worktree prune` takes the admin dir of a tree that was deleted by hand,
// and the store in it: pruneWorktrees imports it first, and keeps it when it
// cannot.
func TestQASeatImportPruneImportsAnOrphanedStoreFirst(t *testing.T) {
	t.Parallel()
	_, repo, tr, store := siTree(t)
	head := siSeatCommit(t, tr, store, "fix.txt", "the work\n", "s-1: the fix")
	bad := strings.Repeat("cd", 20)
	write(t, filepath.Join(store, bad[:2], bad[2:]), "not an object")
	if err := os.RemoveAll(tr.Path); err != nil {
		t.Fatal(err)
	}
	if err := pruneWorktrees(repo); err == nil {
		t.Fatal("prune ran over an orphaned store that would not import")
	}
	if _, err := os.Stat(store); err != nil {
		t.Fatalf("the store went with the prune: %v", err)
	}
	if err := os.Remove(filepath.Join(store, bad[:2], bad[2:])); err != nil {
		t.Fatal(err)
	}
	if err := pruneWorktrees(repo); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(store); err == nil {
		t.Error("CONTROL: the prune did not take the orphaned admin dir")
	}
	if !siHas(t, repo, head) {
		t.Fatalf("the prune took the seat's commit %s with its store", abbrevSHA(head))
	}
}

// The watch pass's call (design D4): a STAGED blob in a seat's index — no
// commit at all — fails an operator's `gc` in the shared store until it is
// imported, and the import clears it.
func TestQASeatImportWatchPassClearsGcOverAStagedBlob(t *testing.T) {
	t.Parallel()
	_, repo, tr, store := siTree(t)
	write(t, filepath.Join(tr.Path, "staged.txt"), "staged, never committed\n")
	siSeatGit(t, tr.Path, store, "", "add", "--", "staged.txt")
	if _, err := git(repo, "gc", "-q"); err == nil {
		t.Fatal("CONTROL: gc succeeded with a staged blob only the seat store holds, so this cannot see the import fix it")
	}
	if err := importAllSeatStores(repo); err != nil {
		t.Fatal(err)
	}
	if _, err := git(repo, "gc", "-q"); err != nil {
		t.Fatalf("gc still fails after the watch-pass import: %v", err)
	}
}

// A symlink the seat plants in its store is never followed: neither a loose
// object linked to another repo's file, nor a fan-out dir linked to another
// repo's, nor the store itself.
func TestQASeatImportFollowsNoSymlinkOutOfTheStore(t *testing.T) {
	t.Parallel()
	_, repo, tr, store := siTree(t)
	secret := wtRepo(t)
	write(t, filepath.Join(secret, "key.txt"), "a secret the seat cannot read\n")
	mustGit(t, secret, "add", "key.txt")
	mustGit(t, secret, "commit", "-q", "-m", "secret", "--", "key.txt")
	blob := mustGit(t, secret, "rev-parse", "HEAD:key.txt")
	src := filepath.Join(secret, ".git", "objects", blob[:2], blob[2:])
	if err := os.MkdirAll(filepath.Join(store, blob[:2]), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(src, filepath.Join(store, blob[:2], blob[2:])); err != nil {
		t.Fatal(err)
	}
	commit := mustGit(t, secret, "rev-parse", "HEAD")
	if err := os.Symlink(filepath.Join(secret, ".git", "objects", commit[:2]), filepath.Join(store, commit[:2])); err != nil && commit[:2] != blob[:2] {
		t.Fatal(err)
	}
	if err := importSeatObjects(tr); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{blob, commit} {
		if siHas(t, repo, name) {
			t.Errorf("%s from another repo reached the shared store through a symlink in the seat's store", abbrevSHA(name))
		}
	}

	// The store itself replaced by a link to another repo's objects.
	if err := os.RemoveAll(store); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(secret, ".git", "objects"), store); err != nil {
		t.Fatal(err)
	}
	_ = importSeatObjects(tr)
	if siHas(t, repo, blob) {
		t.Error("a store that is a symlink to another repo's objects was imported")
	}
}
