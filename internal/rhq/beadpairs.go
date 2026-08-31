package rhq

// posse beads check's second alarm (ranger-base-z3s3). beadloss.go's census
// answers "every id git ever carried still resolves" from git history alone
// — it never opens beads.db. This answers a different question, read from
// the live database bd itself keeps: does the dependency graph contain a
// symmetric same-type pair — the bd 0.49.1 `dep add` landmine (NOTES.md,
// ranger-base-pkqn). A `relates-to` edge is always symmetric by design, and
// either `bd dep relate` (one call, both rows) or two `bd dep add -t
// relates-to` calls in opposite directions (ranger-base-uw8g, one row each,
// bd accepts the second unconditionally) plant one. Any `bd dep add` /
// `bd create --deps` whose target can reach such a pair never returns: bd's
// cycle check is a recursive CTE with no visited set, so a 2-cycle bounces
// the walk until its depth-100 cap.
//
// scripts/verify-bd-dep-safety.sh --gate already answers "does one exist" in
// one read-only sqlite pass; it just has no caller. This is that caller,
// ported to Go — cycleNodesSQL below is the same query, kept byte-identical
// on purpose — rather than shelled out to, so the command's dependency on
// sqlite3 is explicit rather than borrowed from a script sitting beside it.
//
// Two different alarms under one verb, on purpose: the census and this are
// independent questions over independent stores (git history vs. the live
// db), so PairCheck never folds into LostBeads, and a caller wanting both
// runs both and reports them under their own labels.

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

// SymmetricPair is one node caught in a same-type dependency 2-cycle: some
// other node depends on it and it depends back, with the identical type.
type SymmetricPair struct {
	ID string
}

// cycleNodesSQL mirrors verify-bd-dep-safety.sh's CYCLE_NODES_SQL exactly —
// the two must never drift, since they answer the same question over the
// same schema.
const cycleNodesSQL = `SELECT DISTINCT a.issue_id
  FROM dependencies a
  JOIN dependencies b
    ON a.issue_id = b.depends_on_id
   AND a.depends_on_id = b.issue_id
   AND a.type = b.type
  ORDER BY 1;`

// PairCheckUnavailableError means the live graph could not be read at all —
// no sqlite3 on PATH, or a WAL-mode db with no live writer and no -shm that
// even a read-only open refuses (CANTOPEN(14), the state bd leaves after a
// write with nobody holding the store). The contract this exists to keep,
// the same one the gate script keeps with its own exit 2: could-not-check
// must never come back as clean.
type PairCheckUnavailableError struct {
	Dir string
	Err error
}

func (e PairCheckUnavailableError) Error() string {
	return DirLabel(e.Dir) + ": bd pair check: " + e.Err.Error()
}
func (e PairCheckUnavailableError) Unwrap() error { return e.Err }

// PairCheck reports the nodes sitting in a symmetric same-type dependency
// pair in dir's beads database. A dir with no database yet is not a finding
// — there is nothing to protect against — and returns nil, nil, the same
// shape LostBeads uses for "no census here". Anything else that keeps this
// from answering comes back as PairCheckUnavailableError, never a silent
// empty result.
func PairCheck(dir string) ([]SymmetricPair, error) {
	db := filepath.Join(beadsHome(dir), "beads.db")
	if _, err := os.Stat(db); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, PairCheckUnavailableError{Dir: dir, Err: err}
	}
	read, cleanup, err := pairCheckReader(db)
	if err != nil {
		return nil, PairCheckUnavailableError{Dir: dir, Err: err}
	}
	defer cleanup()
	out, err := exec.Command("sqlite3", read, cycleNodesSQL).Output()
	if err != nil {
		return nil, PairCheckUnavailableError{Dir: dir, Err: runErr(err)}
	}
	var pairs []SymmetricPair
	for _, line := range bytes.Split(bytes.TrimSpace(out), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		pairs = append(pairs, SymmetricPair{ID: string(line)})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].ID < pairs[j].ID })
	return pairs, nil
}

// runErr recovers sqlite3's stderr, where its own error text lives, rather
// than handing the caller a bare "exit status 1".
func runErr(err error) error {
	if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
		return Die("%s", bytes.TrimSpace(ee.Stderr))
	}
	return err
}

// pairCheckReader picks a readable sqlite source for db — the same fallback
// verify-bd-dep-safety.sh uses. A WAL-mode db with no live writer and no
// -shm cannot be opened `mode=ro` at all (sqlite returns CANTOPEN(14)), so a
// copy is a faithful, safe-to-read snapshot of exactly that state. cleanup
// removes the copy, if one was made; it is always safe to call.
func pairCheckReader(db string) (source string, cleanup func(), err error) {
	noop := func() {}
	probe := exec.Command("sqlite3", "file:"+db+"?mode=ro", "SELECT 1;")
	probe.Stderr = io.Discard
	if probe.Run() == nil {
		return "file:" + db + "?mode=ro", noop, nil
	}
	tmp, err := os.MkdirTemp("", "posse-pair-check-*")
	if err != nil {
		return "", noop, err
	}
	cleanup = func() { os.RemoveAll(tmp) }
	dst := filepath.Join(tmp, "beads.db")
	if err := copyFile(db, dst); err != nil {
		cleanup()
		return "", noop, err
	}
	if wal := db + "-wal"; fileExists(wal) {
		if err := copyFile(wal, dst+"-wal"); err != nil {
			cleanup()
			return "", noop, err
		}
	}
	return dst, cleanup, nil
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o600)
}
