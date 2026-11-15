package posse

// THE SEAT STORE'S WALL (ranger-base-21i98; ADR 0099 D1, moved by ADR 0100
// D1).
//
// A committing seat at L3 (bwrap) and L4 (container) sees its OWN object
// store at `<common>/objects`: `<SeatWallDir>/seat-objects`, bound read-write
// over that name. The shared store is bound read-only at a side path beside
// it, `<own worktree git dir>/seat-objects-base`, and the seat store's
// `info/alternates` names that side path, so every object of history still
// reads while no object file of the shared store can be overwritten,
// unlinked or chmodded from inside the wall (EROFS). No environment variable
// is involved, so bd's commits in another repo never inherit the redirect.
//
// THE STORE LIVED AT `<own worktree git dir>/seat-objects` UNTIL ADR 0100 D1,
// and that was the costliest member of its class. bwrap resolves a bind
// SOURCE at mount time and follows links; the render's component walk judges
// the same source a moment earlier. The per-worktree git dir is in the seat's
// write set (sessionGitGrants), so a leftover of the seat could toggle
// `seat-objects` between the real directory and a link to `<common>/objects`
// in a loop and needed no timing at all: a launch that lost the race bound
// the SHARED object store read-write over `<common>/objects`, which is the
// write ADR 0099 exists to remove. Under the wall dir there is no such
// component — every one of them lies under the read-only `/` of the seat's
// namespace, and the seat reaches the store's CONTENTS through the mount at
// `<common>/objects`.
//
// The side path stays where it was, and that is not an oversight: it is a
// DESTINATION, the shared store is its source, and a destination inside a
// write grant is pinned by being a mount of this render's own (ADR 0100
// D2(b), D4(a)).
//
// What the seat writes reaches the shared store only through the re-hashing
// import (seatimport.go), at the splice, at retire/prune and on every watch
// pass. darwin's seatbelt cannot remap a path, so there is no store there and
// the import is a no-op (ADR 0099 D7).
//
// THE SIDE PATH'S MOUNT MUST NOT READ ITS SOURCE THROUGH THE STORE'S BIND.
// Both mechanisms resolve a bind's SOURCE in the host's view — bwrap through
// its old root, the engine on the host — so the order of the two mounts does
// not decide what the side path holds. The side bind is still emitted FIRST,
// so a reader of the argv does not have to know that.

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// seatStoreBaseName is the side path the shared store is bound read-only at,
// a sibling of the store inside the per-worktree git dir. A sibling and not a
// subdirectory of the store, so the import's listing of the store never meets
// it.
const seatStoreBaseName = "seat-objects-base"

// seatStoreLayout is the paths of one linked worktree's seat store, all
// resolved: the per-worktree git dir, the seat's wall dir, the store inside
// it, the LEGACY store the ADR 0099 launcher wrote inside the git dir, the
// side path, and the shared store the store stands in for. ok is false
// outside a linked worktree, where there is no shared store of anyone
// else's to protect.
//
// Store is "" — and only then — when no wall dir can be named, which is a
// launch with no home. Every producer refuses on it rather than falling back
// to the git dir: the fallback is the hole (ADR 0100 D1).
type seatStoreLayout struct {
	Own, Wall, Store, Legacy, Base, Objects string
}

func seatStoreFor(cwd string) (seatStoreLayout, bool) {
	// The error is dropped for SeatbeltWritable's reason: no home is not an
	// error here, it is a layout with no Store, and the producers below say
	// what that costs rather than composing a path out of "".
	home, _ := os.UserHomeDir()
	return seatStoreForHome(home, cwd)
}

func seatStoreForHome(home, cwd string) (seatStoreLayout, bool) {
	dirs := LinkedGitDirs(cwd)
	if len(dirs) != 2 {
		return seatStoreLayout{}, false
	}
	return seatStoreFrom(dirs[0], dirs[1], SeatWallDir(home, cwd)), true
}

func seatStoreFrom(own, common, wall string) seatStoreLayout {
	own = absResolve(own)
	l := seatStoreLayout{
		Own:     own,
		Wall:    wall,
		Legacy:  filepath.Join(own, seatStoreName),
		Base:    filepath.Join(own, seatStoreBaseName),
		Objects: absResolve(filepath.Join(common, "objects")),
	}
	if wall != "" {
		l.Store = filepath.Join(wall, seatStoreName)
	}
	return l
}

// seatStoreNoWall is the one refusal every producer here shares: a launch
// that cannot name a wall dir has nowhere to put a store that no seat can
// redirect, and the ADR 0099 placement it would fall back to is the hole ADR
// 0100 D1 closes. Refusing costs this seat its launch; falling back costs
// every seat in the repo its object store.
func seatStoreNoWall(cwd string) error {
	return Die("seat store: this launch has no home, so %s's per-seat wall dir cannot be named and the store would have to go back under the per-worktree git dir, where a leftover of the seat redirects its bind onto the SHARED object store (ADR 0100 D1). Set HOME and relaunch", AbbrevHome(cwd))
}

// ─── THE WALL DIR'S origin FILE (ADR 0100 D1) ───────────────────────────────

// seatWallOriginName is the file the launcher writes when it makes a wall
// dir: the session tree the dir belongs to and the git common dir of the
// repo that tree is a worktree of.
//
// IT IS WHAT MAKES AN ORPHANED WALL STORE FINDABLE. A wall dir's name is a
// hash of the RESOLVED session tree path (seatDirKey), so once the tree is
// gone the key cannot be recomposed from a path that no longer resolves —
// the importers would have no way to tell one repo's abandoned store from
// another's. Sweeping the wall root for these files answers both questions
// at once: which common dir the store belongs to, and whether its tree is
// still there.
//
// It is trustworthy for the same reason the sources beside it are: no render
// grants the wall root, so nothing but the launcher writes under it.
const seatWallOriginName = "origin"

const (
	seatWallOriginTree   = "tree "
	seatWallOriginCommon = "common "
)

// writeSeatWallOrigin writes (or rewrites) that file. Every launch, not only
// the one that creates the dir: a tree that moved, or a file lost to a
// half-finished launch, would otherwise leave the store unfindable, and the
// write costs two lines.
func writeSeatWallOrigin(wall, tree, common string) error {
	body := seatWallOriginTree + absResolve(tree) + "\n" + seatWallOriginCommon + absResolve(common) + "\n"
	p := filepath.Join(wall, seatWallOriginName)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		return Die("seat store: %s could not be written (%v) — without it an orphaned wall store cannot be found and its objects would be lost with the dir (ADR 0100 D1)", AbbrevHome(p), err)
	}
	return nil
}

// readSeatWallOrigin reads it back. ok is false for a dir that has none,
// and two kinds of dir have none: every wall dir made before this landed,
// and every wall dir of a seat that is NOT in a linked worktree — the
// launcher writes the file where it makes the store (prepareSeatStore), and
// a seat with no store has no shared object store of anyone else's to stand
// in for. Both hold a private /tmp and claude state and nothing a sweep is
// looking for, so skipping them loses nothing.
func readSeatWallOrigin(wall string) (tree, common string, ok bool) {
	b, err := os.ReadFile(filepath.Join(wall, seatWallOriginName))
	if err != nil {
		return "", "", false
	}
	for _, ln := range strings.Split(string(b), "\n") {
		switch {
		case strings.HasPrefix(ln, seatWallOriginTree):
			tree = strings.TrimSpace(strings.TrimPrefix(ln, seatWallOriginTree))
		case strings.HasPrefix(ln, seatWallOriginCommon):
			common = strings.TrimSpace(strings.TrimPrefix(ln, seatWallOriginCommon))
		}
	}
	return tree, common, tree != "" && common != ""
}

// seatStoreMechanism answers whether this launch's wall can put the store up:
// the container tier always, the seatbelt tier only where it is rendered by
// bwrap. darwin's sandbox-exec cannot bind, so the walled-but-not-caged tier
// there keeps the shared `objects` grant (ADR 0099 D7).
func seatStoreMechanism(caged bool) bool {
	return caged || WallMechanism() == WallBwrap
}

// prepareSeatStore makes the wall dir, its origin file, the store and the
// side path, and (re)writes the store's alternates to name the side path.
// Every launch, because the seat can write its own store and may have
// changed that file; what it says is only ever the seat's own view, so a
// rewrite costs nothing and an old value costs the seat its history.
//
// THE LEGACY STORE IS IMPORTED FIRST, and that is ADR 0100 D1's one
// exception to its own NO MIGRATION rule: the old store holds COMMITTED
// WORK, and after the move nothing binds it, so a seat whose branch tip is
// readable only there would find git unable to read its own HEAD. The store
// is not MOVED — a rename of it would be the uncaged launcher writing
// through the seat-writable components this ADR exists to stop using — it is
// drained through the re-hashing import, which is the one route its objects
// have ever had into the shared store.
//
// THE STORE AND THE SIDE PATH ARE MADE THROUGH AN os.Root OPENED FROM THE
// PARENT, for the import's reason (openSeatStore): the side path sits inside
// a dir the seat writes, so a link it planted at the path, at `info`, or at
// `info/alternates` must never hand this launcher a write outside the store.
// The file is removed and created exclusively rather than truncated, so a
// hard link the seat made there is replaced and not written through.
func prepareSeatStore(t *SessionTree) error {
	if t == nil || t.Path == "" {
		return nil
	}
	l, ok := seatStoreFor(t.Path)
	if !ok {
		return nil
	}
	if l.Store == "" {
		return seatStoreNoWall(t.Path)
	}
	if err := importLegacySeatStore(t.Repo, l); err != nil {
		return err
	}
	if err := os.MkdirAll(l.Wall, 0o755); err != nil {
		return Die("seat store: the seat's wall dir %s could not be made (%v)", AbbrevHome(l.Wall), err)
	}
	if err := writeSeatWallOrigin(l.Wall, t.Path, filepath.Dir(l.Objects)); err != nil {
		return err
	}
	for _, d := range []string{l.Store, l.Base} {
		fi, err := os.Lstat(d)
		switch {
		case err == nil && !fi.IsDir():
			return Die("seat store: %s is not a directory — remove it and relaunch (ranger-base-21i98, ADR 0100 D1)", AbbrevHome(d))
		case errors.Is(err, fs.ErrNotExist):
			if err := os.Mkdir(d, 0o755); err != nil {
				return Die("seat store: %s could not be made (%v)", AbbrevHome(d), err)
			}
		case err != nil:
			return Die("seat store: %s could not be read (%v)", AbbrevHome(d), err)
		}
	}
	root, err := openSeatStore(l.Store)
	if err != nil {
		return Die("seat store: %s could not be opened (%v)", AbbrevHome(l.Store), err)
	}
	defer root.Close()
	if fi, err := root.Lstat("info"); err == nil && !fi.IsDir() {
		return Die("seat store: %s/info is not a directory — remove it and relaunch (ranger-base-21i98)", AbbrevHome(l.Store))
	} else if errors.Is(err, fs.ErrNotExist) {
		if err := root.Mkdir("info", 0o755); err != nil {
			return Die("seat store: %s/info could not be made (%v)", AbbrevHome(l.Store), err)
		}
	} else if err != nil {
		return Die("seat store: %s/info could not be read (%v)", AbbrevHome(l.Store), err)
	}
	alt := filepath.Join("info", "alternates")
	if err := root.Remove(alt); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Die("seat store: %s/%s could not be replaced (%v)", AbbrevHome(l.Store), alt, err)
	}
	f, err := root.OpenFile(alt, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return Die("seat store: %s/%s could not be written (%v)", AbbrevHome(l.Store), alt, err)
	}
	if _, err := f.WriteString(l.Base + "\n"); err != nil {
		f.Close()
		return Die("seat store: %s/%s could not be written (%v)", AbbrevHome(l.Store), alt, err)
	}
	if err := f.Close(); err != nil {
		return Die("seat store: %s/%s could not be written (%v)", AbbrevHome(l.Store), alt, err)
	}
	return nil
}

// seatStoreBinds is the store's wall at L3, as two private binds the bwrap
// render places after every write grant (BwrapArgs step 2c), so the store's
// bind is the deepest mount over `<common>/objects` even when another grant
// covers `<common>` whole. nil for a PID that denies the whole tree: it is
// granted no `objects` at all (ADR 0086 D3), and the side path's grant goes
// with it.
//
// SrcRoot is the deepest directory the render trusts for each source: the
// seat's WALL dir for the store, which nothing but the launcher writes (ADR
// 0100 D1), and the common dir for the shared store, which no seat writes.
//
// IT REFUSES RATHER THAN RENDERING A STORE UNDER THE GIT DIR. Before ADR
// 0100 the store's SrcRoot was that git dir — "which the seat can write
// below and not at", which was true of the directory and false of the store
// inside it.
func seatStoreBinds(ag *AgentFile, home, cwd string) ([]PrivateBind, error) {
	if ag != nil && deniesFileWrite(ag.Deny) {
		return nil, nil
	}
	l, ok := seatStoreForHome(home, cwd)
	if !ok {
		return nil, nil
	}
	if l.Store == "" {
		return nil, seatStoreNoWall(cwd)
	}
	return []PrivateBind{
		{Src: l.Objects, Dst: l.Base, SrcRoot: filepath.Dir(l.Objects), ReadOnly: true},
		{Src: l.Store, Dst: l.Objects, SrcRoot: l.Wall},
	}, nil
}

// seatStoreCageMounts is the same wall at L4: the side path read-only, then
// the store read-write over `<common>/objects`, each at the destination the
// covering mount spells (cageCovering). A mount already landing exactly on a
// destination is REPLACED rather than doubled — two binds at one destination
// is an engine error — and the engine sorts binds by destination depth, so
// the store is the deepest mount over that name whatever covers `<common>`.
//
// WITH NO WALL DIR TO NAME IT, `<common>/objects` IS MOUNTED READ-ONLY OVER
// ITSELF and the seat gets no store (ADR 0100 D1). This tier cannot refuse —
// it renders a mount list and returns no error — so the fail-closed answer
// is the one mount that is never the hole: the seat reads every object of
// history and commits nothing, which is loud at the first commit and in the
// mount list the report prints. The launch does not in fact get this far
// (prepareSeatStore refuses first, with the sentence that says what to do);
// this is what the renderer answers if it ever does.
func seatStoreCageMounts(ms []CageMount, own, common, wall string) []CageMount {
	l := seatStoreFrom(own, common, wall)
	put := func(src, dst string, ro bool, why string) {
		i, over := cageCovering(ms, absResolve(dst))
		if i < 0 {
			return
		}
		m := CageMount{Src: src, Dst: over.Dst, RO: ro, Why: why}
		if ms[i].Dst == over.Dst {
			ms[i] = m
			return
		}
		ms = append(ms, m)
	}
	if l.Store == "" {
		put(l.Objects, l.Objects, true,
			"the shared object store, READ-ONLY over its own name: this launch named no home, so there is no per-seat wall dir to hold a store of the seat's own, and a read-write mount here would be the shared store (ADR 0099 D1, ADR 0100 D1)")
		return ms
	}
	put(l.Objects, l.Base, true,
		"the shared object store, READ-ONLY at a side path the seat store's info/alternates names, so history reads and no existing object can be rewritten (ADR 0099 D1)")
	put(l.Store, l.Objects, false,
		"this seat's OWN object store, under the per-seat wall dir and read-write over <common>/objects — what it writes reaches the shared store only through the re-hashing import (ADR 0099 D1, D3; ADR 0100 D1)")
	return ms
}
