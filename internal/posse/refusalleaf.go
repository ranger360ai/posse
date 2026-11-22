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

import (
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
)

// errRefusalLeaf is what every refusal below wraps, so a caller (and a test)
// can tell "this leaf was not the file it should be" from an I/O error.
var errRefusalLeaf = errors.New("not a regular single-link file")

func refusalLeafRefused(path, what string) error {
	return fmt.Errorf("%s: %w (%s) — the host opens the refusals trail through no link and no shared inode; remove it and retry (ranger-base-legou)", path, errRefusalLeaf, what)
}

// openRefusalLeaf opens path for flag (os.O_RDONLY, or os.O_WRONLY|
// os.O_APPEND) without following a link at the leaf and returns it only if
// the opened descriptor is a regular file with one link. create makes the
// file 0644 when it is absent — with O_EXCL, so a name that is already
// there (a dangling link included) is never created THROUGH; on EEXIST the
// open is retried without O_CREATE and judged like any other.
func openRefusalLeaf(path string, flag int, create bool) (*os.File, error) {
	base := flag | syscall.O_NOFOLLOW | syscall.O_NONBLOCK
	var f *os.File
	var err error
	if create {
		f, err = os.OpenFile(path, base|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, os.ErrExist) {
			f, err = os.OpenFile(path, base, 0)
		}
	} else {
		f, err = os.OpenFile(path, base, 0)
	}
	if err != nil {
		switch {
		case errors.Is(err, syscall.ELOOP):
			return nil, refusalLeafRefused(path, "a symbolic link")
		case errors.Is(err, syscall.ENXIO):
			return nil, refusalLeafRefused(path, "a FIFO or device with no reader")
		case errors.Is(err, syscall.EISDIR):
			return nil, refusalLeafRefused(path, "a directory")
		}
		return nil, err
	}
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

// readRefusalLeaf is os.ReadFile through openRefusalLeaf: the spool's bytes
// are copied into the canonical log, so a spool linked elsewhere would copy
// the link target's bytes there.
func readRefusalLeaf(path string) ([]byte, error) {
	f, err := openRefusalLeaf(path, os.O_RDONLY, false)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}
