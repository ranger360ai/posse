package posse

// THE SEAT-STORE IMPORT (ranger-base-xcucy, design ranger-base-dvw0w D1, D3
// and D4 as ruled there).
//
// A walled seat may write its objects into a store of its own,
// `<own worktree git dir>/seat-objects`, instead of the shared one. Nothing
// in that store is trusted: git names an object by the hash of its bytes but
// never re-hashes on read, so a file in it may hold any bytes under any name.
// This is the one way those objects reach the shared store, and it is
// receive-pack's quarantine migration with the one step git skips put back
// in: the objects are RE-HASHED on the way in.
//
//   - Copied, never read in place. The seat's files are copied out of the
//     store into a fresh bare view this launcher makes (mktemp), through an
//     os.Root, so a symlink the seat planted cannot hand the launcher a file
//     outside the store. The store's own info/alternates is never copied:
//     inside the wall it names a side path, and outside it is seat-written
//     config.
//   - Only what the shared store lacks. A loose object whose NAME the shared
//     store already holds is never copied at all, so bytes planted under an
//     existing name cannot reach anything.
//   - Re-hashed. The rest is packed in the view (`pack-objects`) and indexed
//     into the shared store with `index-pack --stdin --strict`, which names
//     each object by hashing it. Forged bytes land under their TRUE name and
//     the name they claimed stays absent, so a later honest write of that
//     name is not skipped. MEASURED 2026-11-15, git 2.56.0, linux 7.2.15:
//     `pack-objects` carries a loose object whose bytes do not match its name
//     without complaint, and index-pack files it under the hash of its bytes.
//
// No store, no work: a seat with no store gets a no-op here. darwin's
// seatbelt seats have one since ranger-base-5blmt (seatbeltseatstore.go), so
// this is no longer a no-op there.
//
// THE CALLERS, in the order the design names them: the splice, before
// `branch -f` (spliceDetachedWork — git refuses a `branch -f` onto a commit
// the shared store lacks, so the import has to come first anyway);
// RemoveSessionTree, before `worktree remove`, refusing the removal when the
// import fails; pruneWorktrees, before `worktree prune`; and the landing
// sweep, once per pass over every store in every repo (landClosedTrees),
// which is what bounds how long an operator's `gc` fails over a seat's
// staged blobs.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// TWO ARMS, SINCE ADR 0100 D1 (ranger-base-ltmyh). The store moved out of
// the per-worktree git dir and into the seat's WALL dir, so every importer
// here asks about two places:
//
//   - the WALL arm, `<SeatWallDir>/seat-objects`. For a live tree the dir is
//     composed from the tree path, as the render composes it. For an ORPHAN
//     it cannot be — the dir's name hashes the RESOLVED tree path and a tree
//     that is gone resolves to its spelling — so the wall ROOT is swept for
//     `origin` files naming this common dir (seatstore.go), which is the
//     file the launcher writes for exactly this reason.
//   - the LEGACY arm, `<own worktree git dir>/seat-objects`, unchanged. It
//     stays until a census reads no admin dir holding one; its removal is
//     ranger-base-tbg3h and not this bead. Nothing writes a legacy store any
//     more, so after the first launch of each seat the arm costs one listing
//     and one `cat-file --batch-check` per store.
//
// THE REMOVAL ORDER IS PART OF THIS FILE'S CONTRACT (ADR 0100 D1): import
// both arms, then `git worktree remove`/`prune`, then RemoveAll the wall
// dir. A failed import refuses the removal, as it always did — the wall
// store holds committed work and the dir is the only copy.

// seatStoreName is a seat's own object store: `<SeatWallDir>/seat-objects`
// since ADR 0100 D1, and before it a directory of this same name inside the
// per-worktree git dir, which `git worktree remove` and `git worktree prune`
// delete with the rest of that dir — which is why both are still preceded by
// an import.
const seatStoreName = "seat-objects"

// seatImportTimeout bounds one import's pack/index pipe. An import moves
// objects, not refs, so it is sized like the other object-volume call
// (GitArchiveTimeout) and not like a ref read.
var seatImportTimeout = GitArchiveTimeout

// repoCommonDir is the repo's common git dir, absolute. "" when git will not
// say.
func repoCommonDir(repo string) string {
	common, err := git(repo, "rev-parse", "--git-common-dir")
	if err != nil || common == "" {
		return ""
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(repo, common)
	}
	return filepath.Clean(common)
}

// seatStoreAt is the per-worktree admin dir's store and whether there is
// one. Lstat and not Stat: a store the seat replaced with a symlink is not a
// store this launcher reads (the copy below would refuse it anyway).
func seatStoreAt(admin string) (string, bool) {
	store := filepath.Join(admin, seatStoreName)
	_, err := os.Lstat(store)
	return store, err == nil || !errors.Is(err, fs.ErrNotExist)
}

// seatWallStoreIn is the store inside one wall dir, and whether there is
// one. seatStoreAt's question at the other arm, with its Lstat for its
// reason.
func seatWallStoreIn(wall string) (string, bool) {
	if wall == "" {
		return "", false
	}
	store := filepath.Join(wall, seatStoreName)
	_, err := os.Lstat(store)
	return store, err == nil || !errors.Is(err, fs.ErrNotExist)
}

// seatWallStoreAt is the wall store of a LIVE session tree, composed from
// the tree path exactly as the render composes it. "" with no home: a
// launcher that cannot name a wall dir rendered no wall store either
// (seatStoreNoWall), so there is nothing of that arm's to import.
func seatWallStoreAt(tree string) (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", false
	}
	return seatWallStoreIn(SeatWallDir(home, tree))
}

// seatWallDir is one wall dir the sweep found, with the tree its origin file
// names.
type seatWallDir struct{ dir, tree string }

// seatWallDirsFor sweeps the wall ROOT for every wall dir whose origin names
// this common dir — orphansOnly keeping the ones whose tree is gone, which
// is the set `git worktree prune` is about to take the admin dirs of.
//
// THE SWEEP IS WHY THE origin FILE EXISTS. A wall dir is named by a hash of
// the resolved session tree path, so for a tree that is already gone there
// is no path to hash and no way to tell this repo's abandoned store from
// another repo's. An entry with no origin file is every wall dir made before
// ADR 0100's store slice landed: it holds a private /tmp and claude state
// and no store, so skipping it loses nothing.
func seatWallDirsFor(common string, orphansOnly bool) []seatWallDir {
	home, err := os.UserHomeDir()
	if err != nil || home == "" || common == "" {
		return nil
	}
	root := SeatWallRoot(home)
	ents, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	want := absResolve(common)
	var out []seatWallDir
	for _, e := range ents {
		// IsDir is the lstat kind, so a link at a wall dir's name is not
		// swept. Nothing can plant one — no render grants the root — and a
		// sweep that followed one would import another directory's files
		// under this repo's name.
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		tree, c, ok := readSeatWallOrigin(dir)
		if !ok || absResolve(c) != want {
			continue
		}
		if orphansOnly {
			if _, err := os.Stat(tree); err == nil {
				continue
			}
		}
		out = append(out, seatWallDir{dir: dir, tree: tree})
	}
	return out
}

// openSeatStore opens the store as an os.Root, from its PARENT: os.OpenRoot
// of the store's own path would follow a symlink the seat put there, where
// the parent's Root.OpenRoot refuses one that leaves the parent.
func openSeatStore(store string) (*os.Root, error) {
	parent, err := os.OpenRoot(filepath.Dir(store))
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	return parent.OpenRoot(filepath.Base(store))
}

// seatAdmin is one per-worktree admin dir under <common>/worktrees, with the
// tree its gitdir file names.
type seatAdmin struct {
	admin string
	tree  string
}

// seatAdmins lists every per-worktree admin dir in the repo. The tree is
// read from the admin dir's `gitdir` file — "" when that file is unreadable,
// which pruneWorktrees reads as "prune might take it".
func seatAdmins(repo string) []seatAdmin {
	common := repoCommonDir(repo)
	if common == "" {
		return nil
	}
	ents, err := os.ReadDir(filepath.Join(common, "worktrees"))
	if err != nil {
		return nil
	}
	var out []seatAdmin
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		admin := filepath.Join(common, "worktrees", e.Name())
		tree := ""
		if b, err := os.ReadFile(filepath.Join(admin, "gitdir")); err == nil {
			tree = filepath.Dir(strings.TrimSpace(string(b)))
		}
		out = append(out, seatAdmin{admin: admin, tree: tree})
	}
	return out
}

// importSeatObjects imports THIS session's stores — both arms — into the
// shared one. nil is "imported, or there was nothing to import".
//
// The legacy store is the one git itself would delete: a tree that exists is
// asked for its own git dir (that is the dir `worktree remove` takes), and
// only an answer inside the repo's own worktrees/ is taken. The wall store
// is the one RemoveSessionTree deletes after the removal, and it is composed
// from the tree path. A tree that is already gone has no answer to ask, so
// every store whose tree is missing is imported — exactly the set `worktree
// prune` is about to take.
func importSeatObjects(t *SessionTree) error {
	if t == nil || t.Repo == "" {
		return nil
	}
	common := repoCommonDir(t.Repo)
	if common == "" {
		return nil
	}
	if t.Path != "" {
		if _, err := os.Stat(t.Path); err == nil {
			dirs := LinkedGitDirs(t.Path)
			if len(dirs) != 2 || !pathUnder(dirs[0], filepath.Join(common, "worktrees")) {
				return nil
			}
			return importStores(t.Repo, liveSeatStores(t.Path, dirs[0]))
		}
	}
	return importOrphanedSeatStores(t.Repo)
}

// liveSeatStores is both arms for one live session tree: the wall store, and
// the legacy store in its admin dir.
func liveSeatStores(tree, admin string) []string {
	var out []string
	if store, ok := seatWallStoreAt(tree); ok {
		out = append(out, store)
	}
	if store, ok := seatStoreAt(admin); ok {
		out = append(out, store)
	}
	return out
}

// orphanedSeatStores is both arms over the stores whose tree is gone: the
// legacy stores in the admin dirs `worktree prune` would delete, and the
// wall stores the origin sweep finds for this repo.
func orphanedSeatStores(repo string) []string {
	var out []string
	for _, a := range seatAdmins(repo) {
		if a.tree != "" {
			if _, err := os.Stat(a.tree); err == nil {
				continue
			}
		}
		if store, ok := seatStoreAt(a.admin); ok {
			out = append(out, store)
		}
	}
	for _, w := range seatWallDirsFor(repoCommonDir(repo), true) {
		if store, ok := seatWallStoreIn(w.dir); ok {
			out = append(out, store)
		}
	}
	return out
}

// importOrphanedSeatStores imports every store whose tree is gone — what
// `worktree prune` would delete next, and the wall dirs that go with them.
func importOrphanedSeatStores(repo string) error {
	return importStores(repo, orphanedSeatStores(repo))
}

// importAllSeatStores is the watch pass's call: every store in the repo,
// live or orphaned, at both arms. One store's failure does not stop the
// others.
func importAllSeatStores(repo string) error {
	var stores []string
	for _, a := range seatAdmins(repo) {
		if store, ok := seatStoreAt(a.admin); ok {
			stores = append(stores, store)
		}
	}
	for _, w := range seatWallDirsFor(repoCommonDir(repo), false) {
		if store, ok := seatWallStoreIn(w.dir); ok {
			stores = append(stores, store)
		}
	}
	return importStores(repo, stores)
}

// importStores is the loop every caller above shares: one store's failure
// does not stop the others, and every failure is reported.
func importStores(repo string, stores []string) error {
	var errs []error
	for _, store := range stores {
		if err := importSeatStore(repo, store); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// importLegacySeatStore drains the ADR 0099 store in the per-worktree git
// dir before a launch renders the wall store over `<common>/objects` (ADR
// 0100 D1's NO MIGRATION exception). Called by prepareSeatStore, and a
// refusal here refuses the LAUNCH: before this import the seat's own branch
// tip may be readable only out of that store, and a seat whose git cannot
// read its own HEAD is worse than a seat that did not start.
//
// It is not a move. Renaming the directory would be the uncaged launcher
// writing through the seat-writable components ADR 0100 exists to stop
// trusting; the import is the one route those objects have ever had into
// the shared store, and it re-hashes every one of them (importSeatStore).
// What is left behind is an emptied-of-meaning directory the legacy arm
// keeps meeting at one listing per launch.
func importLegacySeatStore(repo string, l seatStoreLayout) error {
	if repo == "" {
		return nil
	}
	store, ok := seatStoreAt(l.Own)
	if !ok {
		return nil
	}
	if err := importSeatStore(repo, store); err != nil {
		return Die("seat store: the legacy store %s could not be imported (%v), and this seat's own branch tip may be readable only there — the launch is refused rather than rendering a wall store over a history git cannot read (ADR 0100 D1). Fix or remove what will not import, and relaunch", AbbrevHome(store), err)
	}
	return nil
}

// pruneWorktrees is `git worktree prune` that cannot take an un-imported
// store with it, at both arms: the admin dirs the prune deletes, and the
// wall dirs of the same gone trees, which it removes AFTER the prune (ADR
// 0100 D1's retire order — import both arms, then the removal, then the
// wall dir).
//
// A store that will not import keeps EVERY stale admin dir in the repo —
// prune takes them all or none — which is best effort exactly as the bare
// prune it replaces was: a stale registration costs a refused `worktree add`
// of that one path, and a lost store costs the work. The wall dirs are
// removed best-effort for the other half of the same reason: a directory
// left behind costs disk, and no other seat is granted it.
func pruneWorktrees(repo string) error {
	walls := seatWallDirsFor(repoCommonDir(repo), true)
	if err := importOrphanedSeatStores(repo); err != nil {
		return err
	}
	if _, err := git(repo, "worktree", "prune"); err != nil {
		return err
	}
	for _, w := range walls {
		_ = os.RemoveAll(w.dir)
	}
	return nil
}

// importSeatStore is the import itself, for one store.
func importSeatStore(repo, store string) error {
	format, err := git(repo, "rev-parse", "--show-object-format")
	if err != nil || format == "" {
		format = "sha1"
	}
	hexLen := 40
	if format == "sha256" {
		hexLen = 64
	}
	loose, packs, err := listSeatStore(store, hexLen)
	if err != nil {
		return err
	}
	// The loose names the shared store already holds are dropped BEFORE a
	// byte is copied: those are the names a seat could only be planting over.
	if len(loose) > 0 {
		missing, err := missingIn(repo, loose)
		if err != nil {
			return Die("seat store %s: %v", AbbrevHome(store), err)
		}
		loose = missing
	}
	// Nothing left is the steady state the sweep meets every pass, so it
	// costs a listing and one batch read — no view is made for it.
	if len(loose) == 0 && len(packs) == 0 {
		return nil
	}
	tmp, err := os.MkdirTemp("", "posse-seat-import-")
	if err != nil {
		return Die("seat store %s: no scratch dir for the import (%v)", AbbrevHome(store), err)
	}
	defer os.RemoveAll(tmp)
	view := filepath.Join(tmp, "view.git")
	if _, err := git(tmp, "init", "-q", "--bare", "--object-format="+format, view); err != nil {
		return Die("seat store %s: the import view could not be made (%v)", AbbrevHome(store), err)
	}
	if err := copySeatStore(store, view, hexLen, loose, packs); err != nil {
		return err
	}
	all, err := git(view, "cat-file", "--batch-all-objects", "--batch-check=%(objectname)", "--unordered")
	if err != nil {
		return Die("seat store %s: its objects could not be listed (%v)", AbbrevHome(store), err)
	}
	names := strings.Fields(all)
	if len(names) == 0 {
		return nil
	}
	missing, err := missingIn(repo, names)
	if err != nil {
		return Die("seat store %s: %v", AbbrevHome(store), err)
	}
	if len(missing) == 0 {
		return nil
	}
	if err := packInto(seatImportTimeout, view, repo, missing); err != nil {
		return Die("seat store %s: %d object(s) were not imported into %s (%v) — they stay in the seat's store", AbbrevHome(store), len(missing), AbbrevHome(repo), err)
	}
	return nil
}

// listSeatStore names what is in the store ITSELF: loose objects by their
// fan-out path, and pack basenames. Inside an os.Root, so nothing it names
// is outside the store.
func listSeatStore(store string, hexLen int) (loose, packs []string, err error) {
	root, err := openSeatStore(store)
	if err != nil {
		return nil, nil, Die("seat store %s could not be opened (%v)", AbbrevHome(store), err)
	}
	defer root.Close()
	top, err := readRootDir(root, ".")
	if err != nil {
		return nil, nil, Die("seat store %s could not be read (%v)", AbbrevHome(store), err)
	}
	for _, e := range top {
		name := e.Name()
		switch {
		case len(name) == 2 && isHex(name) && e.IsDir():
			ents, err := readRootDir(root, name)
			if err != nil {
				return nil, nil, Die("seat store %s/%s could not be read (%v)", AbbrevHome(store), name, err)
			}
			for _, f := range ents {
				if n := f.Name(); len(n) == hexLen-2 && isHex(n) && f.Type().IsRegular() {
					loose = append(loose, name+n)
				}
			}
		case name == "pack" && e.IsDir():
			ents, err := readRootDir(root, "pack")
			if err != nil {
				return nil, nil, Die("seat store %s/pack could not be read (%v)", AbbrevHome(store), err)
			}
			for _, f := range ents {
				if base, ok := strings.CutSuffix(f.Name(), ".idx"); ok && f.Type().IsRegular() {
					packs = append(packs, base)
				}
			}
		}
	}
	sort.Strings(loose)
	sort.Strings(packs)
	return loose, packs, nil
}

func readRootDir(root *os.Root, name string) ([]os.DirEntry, error) {
	f, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.ReadDir(-1)
}

func isHex(s string) bool {
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return s != ""
}

// copySeatStore copies the named loose objects and every pack pair into the
// view's own objects dir. A name that is not a regular file when it is
// opened (the seat raced a symlink or a dir in) is skipped: what is not
// copied is not imported, and it stays in the store for the next pass.
func copySeatStore(store, view string, hexLen int, loose, packs []string) error {
	root, err := openSeatStore(store)
	if err != nil {
		return Die("seat store %s could not be opened (%v)", AbbrevHome(store), err)
	}
	defer root.Close()
	objects := filepath.Join(view, "objects")
	for _, name := range loose {
		rel := filepath.Join(name[:2], name[2:])
		if err := copyFromRoot(root, rel, filepath.Join(objects, rel)); err != nil {
			return Die("seat store %s: %s could not be copied (%v)", AbbrevHome(store), name, err)
		}
	}
	for _, base := range packs {
		pack, idx := filepath.Join("pack", base+".pack"), filepath.Join("pack", base+".idx")
		if err := copyFromRoot(root, pack, filepath.Join(objects, pack)); err != nil {
			return Die("seat store %s: %s could not be copied (%v)", AbbrevHome(store), pack, err)
		}
		if err := copyFromRoot(root, idx, filepath.Join(objects, idx)); err != nil {
			return Die("seat store %s: %s could not be copied (%v)", AbbrevHome(store), idx, err)
		}
	}
	return nil
}

// errNotRegular is a name in the store that is not a plain file.
var errNotRegular = errors.New("not a regular file")

func copyFromRoot(root *os.Root, rel, dst string) error {
	src, err := root.Open(rel)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil // gone since the listing: nothing to import
		}
		return err
	}
	defer src.Close()
	fi, err := src.Stat()
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() {
		return errNotRegular
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o444)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// missingIn is the subset of names the repo's store does not hold, asked in
// one `cat-file --batch-check` (which consults the repo's own alternates,
// the operator's config, and nothing of the seat's).
func missingIn(repo string, names []string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), GitTimeout)
	defer cancel()
	args := []string{"cat-file", "--batch-check=%(objectname) %(objecttype)"}
	cmd := protocolCmd(exec.CommandContext(ctx, "git", append([]string{"-C", repo}, args...)...))
	bindGitDeadline(ctx, cmd)
	var out, errb bytes.Buffer
	cmd.Stdin = strings.NewReader(strings.Join(names, "\n") + "\n")
	cmd.Stdout, cmd.Stderr = &out, &errb
	started := time.Now()
	err := cmd.Run()
	if ctx.Err() != nil {
		return nil, gitHang(repo, args, GitTimeout, time.Since(started))
	}
	if err != nil {
		return nil, Die("git cat-file --batch-check: %s", gitErrText(errb.String(), err))
	}
	var missing []string
	for _, ln := range strings.Split(out.String(), "\n") {
		if name, ok := strings.CutSuffix(strings.TrimSpace(ln), " missing"); ok {
			missing = append(missing, name)
		}
	}
	return missing, nil
}

// packInto packs names out of the view and indexes the pack into repo's
// store: `pack-objects --stdout` piped into `index-pack --stdin --strict`.
// index-pack is the re-hash; --strict is the fsck of every object and link.
func packInto(limit time.Duration, view, repo string, names []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	pr, pw, err := os.Pipe()
	if err != nil {
		return err
	}
	defer pr.Close()
	defer pw.Close()
	packArgs := []string{"pack-objects", "--stdout", "-q"}
	indexArgs := []string{"index-pack", "--stdin", "--strict"}
	packCmd := protocolCmd(exec.CommandContext(ctx, "git", append([]string{"-C", view}, packArgs...)...))
	indexCmd := protocolCmd(exec.CommandContext(ctx, "git", append([]string{"-C", repo}, indexArgs...)...))
	bindGitDeadline(ctx, packCmd)
	bindGitDeadline(ctx, indexCmd)
	var packErr, indexErr, indexOut strings.Builder
	packCmd.Stdin = strings.NewReader(strings.Join(names, "\n") + "\n")
	packCmd.Stdout, packCmd.Stderr = pw, &packErr
	indexCmd.Stdin, indexCmd.Stdout, indexCmd.Stderr = pr, &indexOut, &indexErr
	started := time.Now()
	if err := indexCmd.Start(); err != nil {
		return err
	}
	pr.Close()
	if err := packCmd.Start(); err != nil {
		pw.Close()
		_ = indexCmd.Wait()
		return err
	}
	pw.Close()
	packWait := packCmd.Wait()
	indexWait := indexCmd.Wait()
	if ctx.Err() != nil {
		return gitHang(repo, indexArgs, limit, time.Since(started))
	}
	if packWait != nil {
		return fmt.Errorf("git pack-objects: %s", gitErrText(packErr.String(), packWait))
	}
	if indexWait != nil {
		return fmt.Errorf("git index-pack --strict: %s", gitErrText(indexErr.String(), indexWait))
	}
	return nil
}

// missingObjects is the landing gate (design ranger-base-dvw0w D1/D5): every
// object tip reaches that base does not, which the shared store does NOT
// hold. Empty is "landable"; an error is "cannot say", and both callers
// refuse on it.
func missingObjects(repo, tip, base string) ([]string, error) {
	out, err := git(repo, "rev-list", "--objects", "--missing=print", tip, "--not", base)
	if err != nil {
		return nil, err
	}
	var missing []string
	for _, ln := range strings.Split(out, "\n") {
		if name, ok := strings.CutPrefix(strings.TrimSpace(ln), "?"); ok {
			missing = append(missing, name)
		}
	}
	return missing, nil
}
