package posse

// THE SEAT STORE'S WALL (ranger-base-21i98; ADR 0099 D1).
//
// A committing seat at L3 (bwrap) and L4 (container) sees its OWN object
// store at `<common>/objects`: `<own worktree git dir>/seat-objects`, bound
// read-write over that name. The shared store is bound read-only at a side
// path beside it, `<own worktree git dir>/seat-objects-base`, and the seat
// store's `info/alternates` names that side path, so every object of history
// still reads while no object file of the shared store can be overwritten,
// unlinked or chmodded from inside the wall (EROFS). No environment variable
// is involved, so bd's commits in another repo never inherit the redirect.
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
)

// seatStoreBaseName is the side path the shared store is bound read-only at,
// a sibling of the store inside the per-worktree git dir. A sibling and not a
// subdirectory of the store, so the import's listing of the store never meets
// it.
const seatStoreBaseName = "seat-objects-base"

// seatStoreLayout is the three paths of one linked worktree's seat store, all
// resolved: the per-worktree git dir that holds the store, the store, the side
// path, and the shared store it stands in for. ok is false outside a linked
// worktree, where there is no shared store of anyone else's to protect.
type seatStoreLayout struct {
	Own, Store, Base, Objects string
}

func seatStoreFor(cwd string) (seatStoreLayout, bool) {
	dirs := LinkedGitDirs(cwd)
	if len(dirs) != 2 {
		return seatStoreLayout{}, false
	}
	return seatStoreFrom(dirs[0], dirs[1]), true
}

func seatStoreFrom(own, common string) seatStoreLayout {
	own = absResolve(own)
	return seatStoreLayout{
		Own:     own,
		Store:   filepath.Join(own, seatStoreName),
		Base:    filepath.Join(own, seatStoreBaseName),
		Objects: absResolve(filepath.Join(common, "objects")),
	}
}

// seatStoreMechanism answers whether this launch's wall can put the store up:
// the container tier always, the seatbelt tier only where it is rendered by
// bwrap. darwin's sandbox-exec cannot bind, so the walled-but-not-caged tier
// there keeps the shared `objects` grant (ADR 0099 D7).
func seatStoreMechanism(caged bool) bool {
	return caged || WallMechanism() == WallBwrap
}

// prepareSeatStore makes the store and the side path, and (re)writes the
// store's alternates to name the side path. Every launch, because the seat
// can write its own store and may have changed that file; what it says is
// only ever the seat's own view, so a rewrite costs nothing and an old value
// costs the seat its history.
//
// THROUGH AN os.Root OPENED FROM THE PARENT, for the import's reason
// (openSeatStore): the store sits inside a dir the seat writes, so a link it
// planted at the store, at `info`, or at `info/alternates` must never hand
// this launcher a write outside the store. The file is removed and created
// exclusively rather than truncated, so a hard link the seat made there is
// replaced and not written through.
func prepareSeatStore(t *SessionTree) error {
	if t == nil || t.Path == "" {
		return nil
	}
	l, ok := seatStoreFor(t.Path)
	if !ok {
		return nil
	}
	for _, d := range []string{l.Store, l.Base} {
		fi, err := os.Lstat(d)
		switch {
		case err == nil && !fi.IsDir():
			return Die("seat store: %s is not a directory (the seat can write its parent) — remove it and relaunch (ranger-base-21i98)", AbbrevHome(d))
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
// per-worktree git dir for the store, which the seat can write below and not
// at; the common dir for the shared store, which no seat writes.
func seatStoreBinds(ag *AgentFile, cwd string) []PrivateBind {
	if ag != nil && deniesFileWrite(ag.Deny) {
		return nil
	}
	l, ok := seatStoreFor(cwd)
	if !ok {
		return nil
	}
	return []PrivateBind{
		{Src: l.Objects, Dst: l.Base, SrcRoot: filepath.Dir(l.Objects), ReadOnly: true},
		{Src: l.Store, Dst: l.Objects, SrcRoot: l.Own},
	}
}

// seatStoreCageMounts is the same wall at L4: the side path read-only, then
// the store read-write over `<common>/objects`, each at the destination the
// covering mount spells (cageCovering). A mount already landing exactly on a
// destination is REPLACED rather than doubled — two binds at one destination
// is an engine error — and the engine sorts binds by destination depth, so
// the store is the deepest mount over that name whatever covers `<common>`.
func seatStoreCageMounts(ms []CageMount, own, common string) []CageMount {
	l := seatStoreFrom(own, common)
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
	put(l.Objects, l.Base, true,
		"the shared object store, READ-ONLY at a side path the seat store's info/alternates names, so history reads and no existing object can be rewritten (ADR 0099 D1)")
	put(l.Store, l.Objects, false,
		"this seat's OWN object store, read-write over <common>/objects — what it writes reaches the shared store only through the re-hashing import (ADR 0099 D1, D3)")
	return ms
}
