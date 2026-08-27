package rhq

// A bead's store of record is bd's SQLite database. `.beads/issues.jsonl` is
// a projection of it: bd's daemon exports the database to the file after
// every mutation and re-imports the file whenever it changes — both
// directions automatic, unattended, and interleaved with a `git pull`.
//
// The import direction can DELETE. `bd import --no-git-history` documents the
// behaviour it turns off: a "git history backfill for deletions" that reads
// the JSONL's git log and removes from the database whatever a commit
// dropped. The daemon's mutation log names create/status/comment/update and
// has no delete event at all, so that path leaves no line anywhere — no bd
// comment, no log entry, no sync commit. That is how rangerhq-b8i (closed,
// with committed work at 33f9645) and rangerhq-ja2 left the database on
// 2026-08-21: their rows went, and `events`/`comments`/`dependencies` went
// with them on the foreign key's ON DELETE CASCADE (rangerhq-fuom).
//
// posse never deletes a bead — no call site passes `delete` to bd, and
// verify-after only reads and creates — so this is not a guard on posse's own
// writes. It is the alarm the substrate does not ring. Every id a committed
// issues.jsonl ever carried is a git-durable census entry, and the diff is
// cheap to walk: an id the census carries that bd can no longer resolve is
// either a deliberate deletion — which must say so in `.beads/deleted.jsonl`
// — or a bead that vanished silently, which is the thing to shout about.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// beadsDirName is where bd keeps a repo's database and its projections
	// — the directory a redirect relocates. The three names below are
	// relative to it, not to a repo root, because under ADR 0012 D3-C the
	// directory bd reads is in a different repo from the one `beads:` names.
	beadsDirName = ".beads"
	// beadsJSONL is the projection bd exports and git tracks — the census.
	beadsJSONL = "issues.jsonl"
	// beadsDeleted is the deliberate-deletion ledger: the record a deletion
	// must leave. It is a sibling of issues.jsonl, tracked by git (nothing
	// in .beads/.gitignore covers it) and never read by bd, so writing it
	// cannot itself perturb the database this file is trying to protect.
	beadsDeleted = "deleted.jsonl"
	// beadsRedirect is bd's cross-repo mount point: a file naming the .beads
	// directory bd should read and write instead of this one.
	beadsRedirect = "redirect"
)

// LostBead is a bead a committed issues.jsonl once carried, that bd can no
// longer resolve, and that the deletion ledger does not account for. Record
// is its last JSONL line verbatim: the provenance the database no longer
// holds, and the only thing a restore could be built from.
type LostBead struct {
	ID       string
	Title    string
	Status   string
	Assignee string
	Commit   string    // the commit whose diff dropped it
	When     time.Time // that commit's author time
	Record   string    // its last JSONL line, verbatim
}

// DeletionRecord is one line of .beads/deleted.jsonl — a deletion somebody
// owns. Record carries the bead itself so the ledger is the restore source
// too: once the row is gone from bd, git history and this file are all that
// is left of it.
//
// Commit is the removal this record accounts for — LostBead.Commit, not a
// clock. A ledger keyed by id alone exempts the id for the life of the repo,
// so a bead restored by `bd import` and then lost again leaves as silently as
// the first time (rangerhq-6he5). Keying on the commit is what makes the
// second loss a new one: git's sha names the removal exactly, where a
// timestamp cannot — commit times are second-granular and At comes from
// whoever called RecordDeletions.
type DeletionRecord struct {
	ID     string          `json:"id"`
	Reason string          `json:"reason"`
	By     string          `json:"by"`
	At     time.Time       `json:"at"`
	Commit string          `json:"commit,omitempty"`
	Record json.RawMessage `json:"record,omitempty"`
}

// LostBeads reports the beads dir has lost without a record. Order is newest
// loss first, then by id.
//
// A repo that is not a git checkout, or whose issues.jsonl git has never
// seen, has no census and so no findings — nil, nil, not an error. This runs
// at the head of a dispatch pass on every configured repo, and a repo that
// cannot be censused must not stop the fleet or say so every pass.
func LostBeads(bd Bd, dir string) ([]LostBead, error) {
	removed, err := removedBeads(dir)
	if err != nil {
		return nil, err
	}
	if len(removed) == 0 {
		return nil, nil
	}
	live, err := bd.ListAll(dir)
	if err != nil {
		return nil, err
	}
	for _, is := range live {
		delete(removed, is.ID)
	}
	ledger, err := ReadDeletionLedger(dir)
	if err != nil {
		return nil, err
	}
	for id, rec := range ledger {
		// The record covers the removal it was written for. A different
		// commit dropped this id, so the ledger says nothing about it:
		// keep it. A record with no commit predates the field and goes on
		// exempting the id — nothing in a live ledger rides on that arm
		// (this repo's three lines were backfilled with the commits the
		// census names), but a ledger written by an older posse must not
		// start alarming about deletions it already owns.
		if lb, ok := removed[id]; ok && rec.Commit != "" && rec.Commit != lb.Commit {
			continue
		}
		delete(removed, id)
	}
	lost := make([]LostBead, 0, len(removed))
	for _, lb := range removed {
		lost = append(lost, lb)
	}
	sort.Slice(lost, func(i, j int) bool {
		if !lost[i].When.Equal(lost[j].When) {
			return lost[i].When.After(lost[j].When)
		}
		return lost[i].ID < lost[j].ID
	})
	return lost, nil
}

// removedBeads walks the JSONL's diff history newest-first and keeps, per id,
// the most recent commit that removed its line. An id removed and later put
// back is still in here; the caller drops it when bd resolves it, which is
// the authority — the census only says "git saw this id leave".
//
// A repo that is not a git checkout, or whose issues.jsonl git has never
// seen, has no census and so no findings — nil, nil, not an error (gitBead
// failing is that case, not a scan failure). --diff-merges=first-parent asks
// git for a merge commit's net diff against its first parent, which `-p`
// alone never prints; the walk still visits side-branch commits on their own
// entries, so a removal that happened on a branch is still attributed to
// that commit and a merge only adds its own net effect.
func removedBeads(dir string) (map[string]LostBead, error) {
	out, err := gitBead(beadsHome(dir), "log", "--format=%x00%H %at", "-p", "--diff-merges=first-parent", "--no-renames", "--", beadsJSONL)
	if err != nil {
		return nil, nil
	}
	removed := map[string]LostBead{}
	sha, when := "", time.Time{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	// A bead line is its whole description on one line; bufio's 64K default
	// truncates the long ones into unparseable halves.
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "\x00") {
			sha, when = parseCommitStamp(line[1:])
			continue
		}
		// One JSON object per line, so a removed bead is exactly "-{…}".
		if !strings.HasPrefix(line, "-{") {
			continue
		}
		var is BdIssue
		if json.Unmarshal([]byte(line[1:]), &is) != nil || is.ID == "" {
			continue
		}
		if _, seen := removed[is.ID]; seen {
			continue // newest-first: the first removal we meet is the last one
		}
		removed[is.ID] = LostBead{
			ID: is.ID, Title: is.Title, Status: is.Status, Assignee: is.Assignee,
			Commit: sha, When: when, Record: line[1:],
		}
	}
	// A line over the scanner's cap ends the walk mid-history with no error
	// from Scan itself — sc.Err() is the only thing that tells the caller
	// this census is partial rather than complete, and a mechanism whose
	// whole contract is "a loss cannot stay quiet" must not go quiet here.
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scanning %s history: %w", beadsJSONL, err)
	}
	return removed, nil
}

// parseCommitStamp splits "<sha> <unix-seconds>".
func parseCommitStamp(s string) (string, time.Time) {
	sha, ts, _ := strings.Cut(strings.TrimSpace(s), " ")
	secs, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return sha, time.Time{}
	}
	return sha, time.Unix(secs, 0)
}

// gitBead runs git from inside a .beads directory, so a pathspec naming a
// file in it needs no repo root: git -C finds whichever checkout the
// directory belongs to, and pathspecs resolve against the cwd. That is what
// lets the census walk a redirect target's history without first working out
// where that repo starts.
func gitBead(home string, args ...string) ([]byte, error) {
	return exec.Command("git", append([]string{"-C", home}, args...)...).Output()
}

// beadsHome is the .beads directory bd actually reads for dir, following
// .beads/redirect when one is there. Everything the census touches — the git
// history of the JSONL, the deletion ledger — hangs off this one answer, so
// reader, writer and git can never disagree about which repo they are in.
//
// Under ADR 0012 D3-C the single `beads:` entry is a working copy whose
// .beads/ holds a redirect and nothing else: the database, its committed
// jsonl and therefore the whole census live in the instance repo the
// redirect names. Without this hop the census walks an empty history and
// LostBeads reports nothing forever — the alarm disarmed without a word,
// which is the exact failure class it exists to shout about (rangerhq-fuom).
//
// The path may be absolute — what the cut-over runbook writes, and what an
// instance fact should be — or relative to the repo root, which is what
// `bd worktree create` writes. A redirect naming something that is not a
// directory falls back to the local .beads: bd warns and reads locally in
// that case, and the census must census what bd is actually reading.
func beadsHome(dir string) string {
	if dir == "" {
		dir = "."
	}
	home := filepath.Join(dir, beadsDirName)
	// A worktree of a redirected repo chains one redirect onto another, and
	// a cycle must not hang a dispatch pass: follow a bounded number of hops.
	for hop := 0; hop < 8; hop++ {
		b, err := os.ReadFile(filepath.Join(home, beadsRedirect))
		if err != nil {
			return home
		}
		// firstLine (cagelauncher.go): bd writes one path and nothing else.
		target := strings.TrimSpace(firstLine(string(b)))
		if target == "" {
			return home
		}
		if !filepath.IsAbs(target) {
			// bd writes the relative form against the repo root, not
			// against .beads/ — one ".." off and bd falls back too.
			target = filepath.Join(filepath.Dir(home), target)
		}
		if st, err := os.Stat(target); err != nil || !st.IsDir() {
			return home
		}
		home = target
	}
	return home
}

// ReadDeletionLedger returns the accounted-for deletions by id. A missing
// ledger is the normal case — no deletions have been owned yet — and a line
// that will not parse is skipped rather than failing the read: the ledger's
// job is to silence what it explains, never to break the check.
func ReadDeletionLedger(dir string) (map[string]DeletionRecord, error) {
	b, err := os.ReadFile(beadsPath(dir, beadsDeleted))
	if os.IsNotExist(err) {
		return map[string]DeletionRecord{}, nil
	}
	if err != nil {
		return nil, err
	}
	recs := map[string]DeletionRecord{}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var r DeletionRecord
		if json.Unmarshal([]byte(line), &r) == nil && r.ID != "" {
			recs[r.ID] = r
		}
	}
	return recs, nil
}

// RecordDeletions appends the lost beads to dir's deletion ledger, each with
// the reason somebody is giving for it, the commit that dropped it, and the
// bead's last JSONL line. This is the record the substrate did not leave;
// committing the ledger is what makes it durable.
func RecordDeletions(dir, reason, by string, lost []LostBead, now time.Time) error {
	if len(lost) == 0 {
		return nil
	}
	var buf bytes.Buffer
	for _, lb := range lost {
		r := DeletionRecord{ID: lb.ID, Reason: reason, By: by, At: now, Commit: lb.Commit}
		if json.Valid([]byte(lb.Record)) {
			r.Record = json.RawMessage(lb.Record)
		}
		line, err := json.Marshal(r)
		if err != nil {
			return Die("recording %s: %v", lb.ID, err)
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	p := beadsPath(dir, beadsDeleted)
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return Die("opening %s: %v", p, err)
	}
	defer f.Close()
	// One write, so a concurrent recorder interleaves whole lines or none.
	if _, err := f.Write(buf.Bytes()); err != nil {
		return Die("writing %s: %v", p, err)
	}
	return nil
}

// beadsPath locates a file in dir's beads directory — the redirect target's
// when there is one, so the ledger is written and read where the census that
// needs explaining lives, and where git tracks it.
func beadsPath(dir, name string) string {
	return filepath.Join(beadsHome(dir), name)
}

// DeletionLedgerPath names the ledger a caller has just written to, which is
// not under dir when dir redirects — the operator has to commit it in the
// repo it landed in.
func DeletionLedgerPath(dir string) string {
	return beadsPath(dir, beadsDeleted)
}

// WarnLostBeads is the dispatch pass's alarm: one line per bead the census
// says is gone and nothing accounts for. It never blocks the pass — a lost
// bead is already lost, and refusing to dispatch would not bring it back —
// and every failure is one line, the same contract verify-after runs under.
// Returns the number of lost beads named.
func (a *App) WarnLostBeads(bd Bd, dirs []string, errw io.Writer) int {
	n := 0
	for _, dir := range dirs {
		lost, err := LostBeads(bd, dir)
		if err != nil {
			fmt.Fprintf(errw, "bead-loss check: %v\n", err)
			continue
		}
		for _, lb := range lost {
			fmt.Fprintf(errw, "bead-loss: %s (%s) is in git but not in bd — dropped by %s; `posse beads check%s` to see it, --record to own it\n",
				lb.ID, lb.Status, shortSHA(lb.Commit), dirFlag(dir))
			n++
		}
	}
	return n
}

func shortSHA(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	if s == "" {
		return "an unknown commit"
	}
	return s
}

func dirFlag(dir string) string {
	if dir == "" {
		return ""
	}
	return " --dir " + dir
}
