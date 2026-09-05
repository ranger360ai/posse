package posse

// Single writer for the refusals trail (ADR 0025 §4, bead ranger-base-l40c,
// discovered from ranger-base-6uq6 item 2: "the audit trail is writable by
// the thing it audits").
//
// The canonical gates/<persona>/refusals.log is never mounted into a cage
// (CageMounts, cage.go): a caged persona's inner L1/L3 append to a
// per-session SPOOL instead — CageSpoolPath, the one file CageMounts still
// mounts rw. Only a host process folds a spool into the canonical log, so
// the file the operator reads has host-side writers only: the fold below,
// the host's own L1 shims (gates.go), and the egress proxy (egress.go),
// none of which the caged process is.
//
// The cursor this fold keeps (spoolCursor, one file per spool, host-only,
// never mounted) is what lets four different call sites (autoreap.go's
// sweep, a session close, a relaunch, an operator's `posse cage`) call this
// on the same session without coordinating between them
// (distributed-systems skill, delivery-and-idempotency.md): the fold is
// at-least-once and idempotent on an unchanged spool — a re-run reads the
// same offset back and appends nothing. Two folds racing over the same
// spool from two processes lose the CURSOR update, never data: each reads
// the file, computes what is new against ITS OWN read of the cursor, and
// writes what it found — worst case the canonical log gains one session's
// lines twice, never a gap, because a fold only ever advances past bytes it
// just hashed and wrote.
//
// Detection reaches the FOLDED PREFIX and nothing past it. SIZE (spool
// shorter than the cursor's offset) and HASH (a same-size rewrite: the
// ADR's own worked case, where a spool truncated and refilled to the same
// size folds as tampered because the hash of bytes[0:offset] no longer
// matches, though the offset comparison alone would have missed it) both
// compare only bytes the canonical log already holds. Either shape appends
// a tamper line to the canonical log naming the session and re-folds the
// whole spool from zero. A spool cut back to exactly the cursor, to any
// length above it, or before its first fold ever ran (no cursor, so no
// comparison at all) folds as "no new lines": the un-folded refusals are
// gone with no tamper line and no marker — the residual ADR 0025 §4 states
// as amended (measured, ranger-base-j3r6z; pinned green by
// TestQAFoldDoesNotDetectATruncationBackToItsOwnCursor in
// refusalfold_qa_test.go, which goes red the day detection grows). The
// guarantee is not that an erasure leaves evidence but that the canonical
// log can only GROW from inside a cage — held by the mount that is not
// there — and that everything already folded is beyond the reach of the
// process it records.
import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// CageSpoolPath is where a caged persona's inner L1/L3 append refusals for
// ONE session's container: state/cages/<persona>/<session>/refusals.log,
// the file CageMounts binds rw. Deliberately not RefusalsLogPath's
// directory — the canonical log and a spool must never be reachable by the
// same glob or the same habitual `cat gates/<p>/refusals.log`.
func (a *App) CageSpoolPath(persona, session string) string {
	return filepath.Join(a.CageDir(persona), session, "refusals.log")
}

// EnsureCageSpool creates a session's spool file if it is not already
// there — RefusalsLog's own reason, one mount over: a bind mount of a file
// that does not exist makes a directory, and the inner shims would then
// append refusals to a path that silently eats them.
func (a *App) EnsureCageSpool(persona, session string) (string, error) {
	p := a.CageSpoolPath(persona, session)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", err
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	return p, f.Close()
}

// spoolCursorPath is the fold's own bookkeeping —
// state/gates/<persona>/spool-cursors/<session>, host-only and never
// mounted into anything. A caged persona that could read or write its own
// cursor could fold its own spool on its own terms, which is exactly the
// single-writer property this path being outside every mount exists to
// keep.
func (a *App) spoolCursorPath(persona, session string) string {
	return filepath.Join(a.GatesDir(persona), "spool-cursors", session)
}

// spoolCursor is one spool's fold state: how far the canonical log already
// has this spool's bytes (Offset), and a hash of exactly those bytes — the
// hash, not the offset, is what catches a truncate-and-refill to the same
// size (ADR 0025 §4 verification 3).
type spoolCursor struct {
	Offset int64
	Hash   string // hex sha256 of the spool's first Offset bytes
}

// readSpoolCursor reads a cursor file. ok is false only when there is none
// yet (a spool never folded before) — that is unclear, not corrupt, and the
// caller folds from zero. A cursor that IS there but does not parse is an
// error and not a silent re-fold-from-zero: the same reading countLedger
// gives a torn ledger line, because "start over" would double-count
// whatever the canonical log already holds for this spool.
func readSpoolCursor(path string) (spoolCursor, bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return spoolCursor{}, false, nil
		}
		return spoolCursor{}, false, err
	}
	fields := strings.Fields(string(b))
	if len(fields) != 2 {
		return spoolCursor{}, false, fmt.Errorf("%s: not a spool cursor (%d fields, want offset hash)", path, len(fields))
	}
	off, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return spoolCursor{}, false, fmt.Errorf("%s: offset %q: %v", path, fields[0], err)
	}
	return spoolCursor{Offset: off, Hash: fields[1]}, true, nil
}

func writeSpoolCursor(path string, c spoolCursor) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(fmt.Sprintf("%d %s\n", c.Offset, c.Hash)), 0o644)
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// FoldRefusalsSpool folds one session's spool into its persona's canonical
// refusals.log (ADR 0025 §4). A no-op, not an error, when the session never
// had a container cage (no spool was ever appended to) or nothing has
// changed since the cursor's own reading — every caller (autoreap.go's
// sweep, a session close, a relaunch) is free to call this on every session
// it touches without checking first, and a second call over an unchanged
// spool appends zero lines (verification 3).
func (a *App) FoldRefusalsSpool(persona, session string) error {
	content, err := os.ReadFile(a.CageSpoolPath(persona, session))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	size := int64(len(content))

	cursorPath := a.spoolCursorPath(persona, session)
	cur, ok, err := readSpoolCursor(cursorPath)
	if err != nil {
		return err
	}

	offset := int64(0)
	tampered := false
	if ok {
		switch {
		case size < cur.Offset:
			tampered = true
		case sha256Hex(content[:cur.Offset]) != cur.Hash:
			tampered = true
		default:
			offset = cur.Offset
		}
	}

	if tampered {
		ts := time.Now().UTC().Format(time.RFC3339)
		line := fmt.Sprintf("%s refusals spool tampered [fold] session=%s\n", ts, session)
		if err := a.appendCanonicalRefusal(persona, line); err != nil {
			return err
		}
		offset = 0 // re-fold the whole spool; the tamper line above marks it suspect
	}

	newOffset := lastCompleteLine(content, offset)
	if newOffset > offset {
		if err := a.appendCanonicalRefusal(persona, foldedLines(content[offset:newOffset], session)); err != nil {
			return err
		}
	}
	return writeSpoolCursor(cursorPath, spoolCursor{Offset: newOffset, Hash: sha256Hex(content[:newOffset])})
}

// lastCompleteLine is the byte offset one past the last '\n' at or after
// from ("from" itself when there is none). The fold consumes whole lines
// only: a shim's append is a shell `>>` of one already-formed line, but a
// fold reading mid-write would otherwise see a partial one — folded here it
// would need a second, retroactive line in the canonical log to finish it,
// which is not a thing an append-only log can do. Left unconsumed, the rest
// of that line is exactly what the NEXT fold's offset starts at.
func lastCompleteLine(content []byte, from int64) int64 {
	for i := len(content) - 1; i >= int(from); i-- {
		if content[i] == '\n' {
			return int64(i) + 1
		}
	}
	return from
}

// foldedLines is what the canonical log gains for one fold: every complete
// line the spool held since the cursor, prefixed with the session it came
// from — the canonical log mixes lines from every session a persona has
// ever run caged, and this prefix is the only place that says which line is
// whose.
//
// Split by hand rather than with a bufio.Scanner, which has a token cap
// (64KiB by default) and stops at the first line longer than it. That cap
// was this trail's erasure hole (bead ranger-base-flblx, escape of
// ranger-base-l40c): the spool is the one file CageMounts binds rw, so a
// caged persona could append one overlong line and every complete line
// AFTER it was dropped — silently, since the scanner's error was ignored,
// and permanently, since the cursor advanced past the whole chunk anyway.
// There is no cap here: the chunk is already whole in memory (os.ReadFile
// in FoldRefusalsSpool), so carrying a long line verbatim costs the fold
// nothing a shorter one did not, and a line the persona wrote is exactly
// what the audit trail is for. The bytes go over unaltered — no \r
// stripping, no truncation — because a trail that edits what it records
// cannot be quoted back.
//
// chunk always ends in '\n' when this is reached: the caller slices to
// lastCompleteLine and folds nothing when that leaves the chunk empty. The
// loop tolerates a trailing partial line anyway rather than dropping it.
func foldedLines(chunk []byte, session string) string {
	var b strings.Builder
	for len(chunk) > 0 {
		line := chunk
		if i := bytes.IndexByte(chunk, '\n'); i >= 0 {
			line, chunk = chunk[:i], chunk[i+1:]
		} else {
			chunk = nil
		}
		fmt.Fprintf(&b, "session=%s %s\n", session, line)
	}
	return b.String()
}

// appendCanonicalRefusal appends to gates/<persona>/refusals.log — the same
// file the host's own L1 shims and the egress proxy append to, and the
// fold's only way of reaching it. RefusalsLog creates the file first: on a
// persona whose first-ever refusal is this fold's tamper line, there may be
// nothing there yet.
func (a *App) appendCanonicalRefusal(persona, s string) error {
	path, err := a.RefusalsLog(persona)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(s); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
