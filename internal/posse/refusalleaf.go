package posse

// The host's own opens of the refusals trail (ranger-base-legou, discovered
// from ranger-base-xab99). Four call sites run UNCAGED over a leaf a
// planter outside every wall could have replaced: RefusalsLog and
// appendCanonicalRefusal on gates/<persona>/refusals.log, EnsureCageSpool
// and FoldRefusalsSpool on a session's spool. A plain os.OpenFile follows a
// link at the leaf — over a DANGLING one, O_CREATE makes the target — and
// the fold appends seat-authored bytes verbatim, so a canonical log linked
// to a shell rc file turns a spool line into a host command.
//
// Every one of those opens goes through openRefusalLeaf: O_NOFOLLOW on the
// leaf, then fstat of the descriptor actually opened, refusing anything but
// a regular file with exactly one link. The judgement is of the fd, never of
// an earlier lstat, so there is no window between the check and the use.
// O_NONBLOCK is there for a planted FIFO, which O_NOFOLLOW still opens: a
// write open of a FIFO with no reader would otherwise BLOCK the fold or the
// launch until one appears (ORDERS.md, ranger-base-xab99); with it the open
// fails (ENXIO) or returns at once and the fstat refuses it. The flag is
// inert on a regular file.
//
// THE PARENTS TOO (ranger-base-byqv0). O_NOFOLLOW guards the last component
// only, and every leaf above sat under directories made by os.MkdirAll and
// resolved by the kernel's ordinary walk: gates/<p>, cages/<p>/<s> and
// gates/<p>/spool-cursors. A link at any of them aimed the guarded open —
// and the cursor's unguarded os.WriteFile, which TRUNCATES — into a
// directory the planter chose. So the open is now a walk: from a ROOT the
// caller trusts (the App's StateDir, which is the operator's own and may
// itself be a link the operator made), each component below it is opened
// with openat(O_DIRECTORY|O_NOFOLLOW) relative to the descriptor of the one
// before, made with mkdirat when absent and the caller asked for a create,
// and the leaf is opened relative to the last directory's descriptor. No
// component below the root is ever resolved by name from the top again, so
// there is no window between judging a parent and using it either.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// errRefusalLeaf is what every refusal below wraps, so a caller (and a test)
// can tell "this leaf was not the file it should be" from an I/O error.
var errRefusalLeaf = errors.New("not a regular single-link file")

// errRefusalParent is the same for a directory between the root and the
// leaf: it was a link, or not a directory at all.
var errRefusalParent = errors.New("not a real directory")

func refusalLeafRefused(path, what string) error {
	return fmt.Errorf("%s: %w (%s) — the host opens the refusals trail through no link and no shared inode; remove it and retry (ranger-base-legou)", path, errRefusalLeaf, what)
}

func refusalParentRefused(path string) error {
	return fmt.Errorf("%s: %w (a symbolic link, or not a directory) — the host walks to the refusals trail through no link; remove it and retry (ranger-base-byqv0)", path, errRefusalParent)
}

// openRefusalLeaf opens path for flag (os.O_RDONLY, or os.O_WRONLY
// with or without os.O_APPEND) and returns it only if the opened descriptor
// is a regular file with one link. path must lie below root; root itself is
// opened by name and trusted, and every component between root and the
// leaf is walked through no link (see the head of this file). create makes
// a missing directory 0755 and a missing leaf 0644 — the leaf with O_EXCL,
// so a name that is already there (a dangling link included) is never
// created THROUGH; on EEXIST the open is retried without O_CREAT and judged
// like any other. Without create, a missing component answers an error
// os.IsNotExist recognises.
//
// No caller may pass O_TRUNC: it would act at the open, before the fstat
// below has judged the descriptor, and cut a hardlinked target it then
// refuses. Truncate the returned file instead (writeSpoolCursor).
func openRefusalLeaf(root, path string, flag int, create bool) (*os.File, error) {
	if flag&os.O_TRUNC != 0 {
		return nil, fmt.Errorf("%s: openRefusalLeaf refuses O_TRUNC (truncate after the fd is judged)", path)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || !filepath.IsLocal(rel) {
		return nil, fmt.Errorf("%s: not below %s", path, root)
	}
	if create {
		if err := os.MkdirAll(root, 0o755); err != nil {
			return nil, err
		}
	}
	dfd, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: root, Err: err}
	}
	parts := strings.Split(rel, string(filepath.Separator))
	walked := root
	for _, c := range parts[:len(parts)-1] {
		walked = filepath.Join(walked, c)
		next, err := openRefusalDirAt(dfd, c, create)
		unix.Close(dfd)
		if err != nil {
			if errors.Is(err, unix.ELOOP) || errors.Is(err, unix.ENOTDIR) {
				return nil, refusalParentRefused(walked)
			}
			return nil, &os.PathError{Op: "openat", Path: walked, Err: err}
		}
		dfd = next
	}
	defer unix.Close(dfd)

	leaf := parts[len(parts)-1]
	base := flag | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
	var fd int
	if create {
		fd, err = unix.Openat(dfd, leaf, base|unix.O_CREAT|unix.O_EXCL, 0o644)
		if err == unix.EEXIST {
			fd, err = unix.Openat(dfd, leaf, base, 0)
		}
	} else {
		fd, err = unix.Openat(dfd, leaf, base, 0)
	}
	if err != nil {
		switch err {
		case unix.ELOOP:
			return nil, refusalLeafRefused(path, "a symbolic link")
		case unix.ENXIO:
			return nil, refusalLeafRefused(path, "a FIFO or device with no reader")
		case unix.EISDIR:
			return nil, refusalLeafRefused(path, "a directory")
		}
		return nil, &os.PathError{Op: "openat", Path: path, Err: err}
	}
	f := os.NewFile(uintptr(fd), path)
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		f.Close()
		return nil, refusalLeafRefused(path, "mode "+fi.Mode().Type().String())
	}
	if !storeSingleEntry(fi) {
		f.Close()
		return nil, refusalLeafRefused(path, "more than one link, or a link count that cannot be read")
	}
	return f, nil
}

// openRefusalDirAt opens name under dfd as a directory through no link,
// making it first when create is set and it is absent. A mkdirat that
// loses a race to another creator (EEXIST) just opens what is there, and
// that is judged exactly like a directory found on the first try.
func openRefusalDirAt(dfd int, name string, create bool) (int, error) {
	const dirFlags = unix.O_RDONLY | unix.O_DIRECTORY | unix.O_NOFOLLOW | unix.O_CLOEXEC
	fd, err := unix.Openat(dfd, name, dirFlags, 0)
	if err == unix.ENOENT && create {
		if merr := unix.Mkdirat(dfd, name, 0o755); merr != nil && merr != unix.EEXIST {
			return -1, merr
		}
		fd, err = unix.Openat(dfd, name, dirFlags, 0)
	}
	return fd, err
}

// readRefusalLeaf is os.ReadFile through openRefusalLeaf: the
// spool's bytes are copied into the canonical log, so a spool linked
// elsewhere — at the leaf or at any directory above it below root — would
// copy the link target's bytes there.
func readRefusalLeaf(root, path string) ([]byte, error) {
	f, err := openRefusalLeaf(root, path, os.O_RDONLY, false)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// ReadRefusalsLog is the canonical trail's bytes for a reader outside this
// package: `posse gates <persona>` prints its last ten lines into the
// operator's terminal (cmd/posse/main.go). That was the one uncaged ACCESS
// of this leaf left on a plain os.ReadFile — the same leaf, the same
// planter class and the same following open as the three WRITES above,
// which is all legou's census claimed, since its subject was the writers
// (ranger-base-jielm, from the ranger-base-q2peg verify). What a link
// bought there is narrower than what it bought the fold — a chosen file's
// last ten lines in the operator's own output, not seat-authored bytes
// appended to a shell rc — but the open was identical, and the parents are
// walked here for ranger-base-byqv0's reason too.
//
// An ABSENT trail answers an error os.IsNotExist recognises, which a
// reporting caller may read as "empty"; a REFUSED one answers
// errRefusalLeaf or errRefusalParent, which it must not.
func (a *App) ReadRefusalsLog(persona string) ([]byte, error) {
	return readRefusalLeaf(a.StateDir, a.RefusalsLogPath(persona))
}

// refusalLeafSize and refusalDeltaAfter are the DELTA read of the canonical
// trail: `runtime probe` notes where the log stood, launches a real seat,
// and grades observable 2 — "the canary deny refused and landed in
// refusals.log through all three shapes" — by searching what the file
// GAINED for three fixed markers (runtimeprobe.go).
//
// FOUND WHILE FIXING THE READ IN `posse gates` (ranger-base-jielm): that
// pair was an os.Stat and an os.ReadFile, so it was the same following open
// at a second host-side call site, and it buys a planter strictly more. The
// `posse gates` read spends the link target's last ten lines on the
// operator's terminal; this one spends the probe's VERDICT — a file linked
// here that happens to carry the three markers reports a wall that refused
// nothing as a wall that held, on the command INSTALL.md §7 sells as the
// thing you read before your first dispatch.
//
// Both fail CLOSED on a refusal, which is the only direction that is safe
// here: a size of 0 reads from the start, and a delta of "" leaves every
// marker missing, so observable 2 reports "no refusal reached refusals.log"
// — true of a leaf the host would not read, and the opposite of what the
// unguarded pair could be made to say.
func refusalLeafSize(root, path string) int64 {
	f, err := openRefusalLeaf(root, path, os.O_RDONLY, false)
	if err != nil {
		return 0
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return 0
	}
	return st.Size()
}

// refusalDeltaAfter returns what the trail gained after offset. A leaf that
// SHRANK (rotated, deleted and recreated) reads from the start rather than
// returning nothing — readFrom's own reason, kept: the alternative is a
// probe that silently measures an empty log.
func refusalDeltaAfter(root, path string, offset int64) string {
	b, err := readRefusalLeaf(root, path)
	if err != nil {
		return ""
	}
	if offset < 0 || offset > int64(len(b)) {
		return string(b)
	}
	return string(b[offset:])
}
