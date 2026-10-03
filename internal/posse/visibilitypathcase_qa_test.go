package posse

// ranger-base-99gww — THE DEFECT. `beads_visibility:` is keyed by path, and
// the key was matched by STRING. resolvedPath normalized a spelling (Clean,
// Abs, EvalSymlinks) but EvalSymlinks does not fold case, so on a
// case-INsensitive volume — the APFS default, this whole box — a config key
// the operator typed as `~/src/hcn` never met the directory git spells
// `~/src/HCN`, and the repo fell through to "unmarked is public (fail
// closed)".
//
// What that cost: `posse gates install-hooks ~/src/hcn` stamped the shared
// prepare-commit-msg hook private (it was handed the config spelling), and
// the next dispatched launch — which derives the repo path from git, so
// upper case — re-stamped the SAME file public and said it had healed a
// wrong wall. Three flips measured in one morning, 2026-10-03. Nothing was
// widened (public is the closed side), but a wall that flips on every launch
// is not a wall, and on a private repo the public stamp refuses the seats'
// own ops-class beads.
//
// THE FIX: samePath — the question was never "what is the canonical string"
// but "are these two spellings the same directory", so it is answered by
// identity (os.SameFile) and not by spelling. That is right on a
// case-sensitive volume too, where `hcn` and `HCN` are two directories with
// two inodes and must NOT merge; a case fold would have merged them.
//
// MUTATION-CHECKED (2026-10-03, this box, APFS case-insensitive): samePath
// back to `resolvedPath(a) == resolvedPath(b)` reds the three case-folding
// arms. The fail-closed controls stay green under that mutation on purpose —
// they are controls, and a pin whose controls also red is measuring the
// fixture.
//
// THE TWO VOLUMES, and why each arm skips where it does. Whether two
// spellings are one directory is a property of the MOUNT, so no single
// volume can run this whole file: the folding arms need a case-insensitive
// one and TestBeadsVisibilityKeepsTwoCaseDistinctReposApart needs a
// case-sensitive one, and each skips with a line naming the volume it did
// not get rather than passing vacuously. MEASURED 2026-10-03 on both:
// `TMPDIR` on this box's APFS (case-insensitive) runs the folding arms and
// skips the distinct-repos arm; `TMPDIR` on a case-sensitive APFS disk image
// does the reverse. Green either way, and neither way green by accident.
//
// docs/notes.d/ranger-base-99gww.md has the measurements, the two candidate
// shapes that were rejected (darwin F_GETPATH, strings.EqualFold) and why.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// otherCaseDir makes a directory under parent spelled `name` and returns the
// OTHER-case spelling of it, skipping when this volume does not admit that
// spelling — on a case-sensitive volume the two spellings are two
// directories and there is no defect here to pin.
func otherCaseDir(t *testing.T, parent, name string) (onDisk, otherCase string) {
	t.Helper()
	onDisk = filepath.Join(parent, name)
	if err := os.MkdirAll(onDisk, 0o755); err != nil {
		t.Fatal(err)
	}
	otherCase = filepath.Join(parent, strings.ToLower(name))
	if strings.ToLower(name) == name {
		t.Fatalf("fixture: %q has no other case", name)
	}
	a, err := os.Stat(onDisk)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.Stat(otherCase)
	if err != nil || !os.SameFile(a, b) {
		t.Skipf("this volume is case-sensitive: %s and %s are not one directory", onDisk, otherCase)
	}
	return onDisk, otherCase
}

// The lookup itself: the configured visibility is what both spellings get.
func TestBeadsVisibilityReadsAConfigKeySpelledInAnotherCase(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	onDisk, lower := otherCaseDir(t, home, "HCN")
	cfg := filepath.Join(home, "config.yaml")
	// The operator's spelling is the lower-case one; git's is on disk.
	if err := os.WriteFile(cfg, []byte("beads_visibility:\n  "+lower+": private\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &App{ConfigPath: cfg}

	for _, dir := range []string{onDisk, lower, onDisk + "/"} {
		got, src := a.BeadsVisibility(dir)
		if got != VisibilityPrivate || !strings.Contains(src, "config beads_visibility:") {
			t.Errorf("%s: got %s (%s), want private from config — a repo whose on-disk spelling differs in case from the key is the SAME repo on this volume",
				dir, got, src)
		}
	}
}

// And the other direction, which is the spelling the operator was told to
// type as the instance-side workaround: a config key in git's case, looked
// up by the lower-case path a script or a stale note carries.
func TestBeadsVisibilityReadsALowerCaseLookupOfAnUpperCaseKey(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	onDisk, lower := otherCaseDir(t, home, "HCN")
	cfg := filepath.Join(home, "config.yaml")
	if err := os.WriteFile(cfg, []byte("beads_visibility:\n  "+onDisk+": private\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, src := (&App{ConfigPath: cfg}).BeadsVisibility(lower); got != VisibilityPrivate {
		t.Errorf("%s: got %s (%s), want private", lower, got, src)
	}
}

// Fail-closed is not weakened by the fix: an unlisted repo beside a listed
// one is still public, and a sibling whose name merely shares a prefix is
// not the listed repo.
func TestBeadsVisibilityStillFailsClosedAcrossCase(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	_, lower := otherCaseDir(t, home, "HCN")
	other := filepath.Join(home, "hcn-instance")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(home, "config.yaml")
	if err := os.WriteFile(cfg, []byte("beads_visibility:\n  "+lower+": private\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &App{ConfigPath: cfg}
	for _, dir := range []string{other, filepath.Join(home, "nope")} {
		if got, src := a.BeadsVisibility(dir); got != VisibilityPublic {
			t.Errorf("%s: got %s (%s) — only the directory the key NAMES is marked", dir, got, src)
		}
	}
}

// The stamp, which is what flipped: install-hooks under either spelling of
// one repo renders the same wall.
func TestCommitGuardStampIsTheSameUnderEitherSpellingOfTheRepo(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	home := gitTempDir(t)
	onDisk, lower := otherCaseDir(t, home, "HCN")
	qaGit(t, onDisk, "init", "-q")
	cfg := filepath.Join(home, "config.yaml")
	if err := os.WriteFile(cfg, []byte("beads_visibility:\n  "+lower+": private\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &App{ConfigPath: cfg}

	// The operator's install, then the launch's pre-heal: the launch derives
	// the repo path from git, so it arrives in the on-disk case. Both must
	// stamp private, and the file both wrote is one file.
	var renders []string
	for _, dir := range []string{lower, onDisk} {
		path, vis, src, err := a.InstallCommitGuardHook(dir)
		if err != nil {
			t.Fatalf("install under %s: %v", dir, err)
		}
		if vis != VisibilityPrivate {
			t.Errorf("install-hooks %s stamped %s (%s) — the two spellings are one repo and one wall", dir, vis, src)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), "posse_beads_visibility='private'") {
			t.Errorf("the hook installed under %s does not carry the private stamp", dir)
		}
		renders = append(renders, string(b))
	}
	if renders[0] != renders[1] {
		t.Error("the two spellings render different hooks — the L3 probe reads 'ours but stale' and re-stamps on every launch, which is the flapping wall ranger-base-99gww names")
	}
}

// twoCaseDistinctDirs makes two directories under parent whose names differ
// only in case, skipping when this volume folds them into one — there, they
// are not two repos and there is nothing to keep apart.
func twoCaseDistinctDirs(t *testing.T, parent, name string) (upper, lower string) {
	t.Helper()
	upper, lower = filepath.Join(parent, name), filepath.Join(parent, strings.ToLower(name))
	for _, d := range []string{upper, lower} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	a, err := os.Stat(upper)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.Stat(lower)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(a, b) {
		t.Skipf("this volume is case-insensitive: %s and %s are one directory", upper, lower)
	}
	return upper, lower
}

// The other volume, and the reason this is identity and not a case fold: on
// a case-sensitive filesystem `hcn` and `HCN` are two repos with two inodes,
// and a private mark on one must not mark the other. A fold would pass every
// arm above and silently hand a public repo the private exemption here.
//
// MUTATION-CHECKED (2026-10-03, case-sensitive APFS disk image): samePath to
// `strings.EqualFold(resolvedPath(a), resolvedPath(b))` reds THIS test and
// nothing else — on that volume the other arms skip, and on a
// case-insensitive one they all pass. That is why the fold had to be
// measured on a second volume to be refused at all.
func TestBeadsVisibilityKeepsTwoCaseDistinctReposApart(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	upper, lower := twoCaseDistinctDirs(t, home, "HCN")
	cfg := filepath.Join(home, "config.yaml")
	if err := os.WriteFile(cfg, []byte("beads_visibility:\n  "+lower+": private\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &App{ConfigPath: cfg}
	if got, src := a.BeadsVisibility(lower); got != VisibilityPrivate {
		t.Errorf("fixture: the marked repo %s reads %s (%s)", lower, got, src)
	}
	if got, src := a.BeadsVisibility(upper); got != VisibilityPublic {
		t.Errorf("%s is a DIFFERENT directory from %s on this volume and is unmarked, but reads %s (%s) — a case fold would hand it the exemption",
			upper, lower, got, src)
	}
}
