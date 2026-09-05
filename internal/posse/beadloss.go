package posse

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
	"reflect"
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
// is left of it. It also does a second job: a rebase or squash rewrites
// Commit out of the history the census walks entirely, and Record is then
// the only thing sameRemoval has left to compare against a replay of the
// same drop under a new sha (ranger-base-6mbz).
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
	home := beadsHome(dir)
	for id, recs := range ledger {
		lb, ok := removed[id]
		if !ok {
			continue
		}
		// SOME record for the id must cover the removal the census found.
		// One record owns one removal (rangerhq-6he5), so an id with two
		// removals has two records and only one of them covers this one;
		// asking "does any of them" is what keeps the answer independent
		// of the order of an append-only file that git merges, rebases
		// replay and an operator can dedupe by hand (rangerhq-fknq).
		//
		// Covering is sameRemoval, not sha equality: git attributes one
		// removal to more than one commit, so the sha the census reports
		// for a removal moves as history grows (ranger-base-ntsz).
		//
		// A record with no commit predates the field, and the whole of a
		// pre-dc2bc16 ledger looks like that: it must go on exempting the
		// id rather than alarm about deletions it already owns. But the arm
		// is compatibility, not an override — once ANY record for this id
		// names a commit, the writer that produced them keys on removals,
		// and a commit-less line among them cannot claim one those records
		// do not name. That is rangerhq-6he5 reachable through a single
		// appended line, and it is the half that goes silent; the arm's own
		// failure direction is a false alarm, which --record answers.
		var covered, modern bool
		for _, rec := range recs {
			if rec.Commit != "" {
				modern = true
				covered = covered || sameRemoval(home, id, rec.Commit, lb.Commit, rec.Record, json.RawMessage(lb.Record))
			}
		}
		if covered || !modern {
			delete(removed, id)
		}
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
// alone never prints (rangerhq-boco).
//
// That makes the commit reported here a moving answer, and rangerhq-boco's
// claim that it does not — "a removal on a branch is still attributed to that
// commit" — was wrong (ranger-base-ntsz). The walk does still visit the side
// commit, but merging the branch shows the same removal in the merge's net
// diff, and the merge is newer, so the newest-first slot moves off the side
// commit onto the merge. Nothing downstream may treat this sha as the
// removal's identity: sameRemoval is how a reader asks whether two shas are
// one removal.
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

// sameRemoval reports whether a deletion recorded against commit rec is the
// removal the census attributed to commit found — one removal seen under two
// shas, rather than two removals.
//
// git blames a removal on every commit whose diff drops the line, so the sha
// the census reports moves as history grows: merge the branch that dropped a
// bead and the merge's first-parent diff drops it again, newer than the side
// commit the ledger recorded (ranger-base-ntsz). Ancestry alone cannot tell
// that from a real second loss, because the first drop is an ancestor of the
// second one too. What separates them is that a second loss needs the line
// back first: rec and found are the same removal exactly when rec is an
// ancestor of found and nothing between them puts the id back. A restore in
// the range is rangerhq-6he5's second loss and must stay a finding.
//
// A record naming a commit that is not an ancestor at all — a record from
// another, still-unmerged line of history that never carried the id into
// found's history (ranger-base-ntsz's off-history case) — covers nothing.
// But a rebase or a squash merge ALSO make rec a non-ancestor of found, for a
// different reason: history was rewritten out from under the recorded sha
// rather than merged, so ancestry cannot speak to this pair at all
// (ranger-base-6mbz). Both replay rec's tree change onto whatever the
// branch's base had grown into by the time it landed, so the same question
// moves one commit further back — did nothing between rec's PARENT and found
// put the id back — with two more guards a plain rebase/squash needs and an
// off-history record must fail: the parent must actually have moved (a
// record whose parent equals found's parent forked from that exact point
// with nothing rewritten, which is the off-history shape, not a replay), and
// the replayed line must read back identical, because moving the ancestry
// root back one commit alone reopens exactly the false-exemption ntsz fixed.
//
// A record naming a commit that is not an ancestor by either route covers
// nothing, which is the false alarm direction: `--record` answers it, and
// silence would not.
func sameRemoval(home, id, rec, found string, recRecord, foundRecord json.RawMessage) bool {
	if rec == "" || found == "" {
		return false
	}
	if rec == found {
		return true
	}
	if exemptRange(home, id, rec, found) {
		return true
	}
	if !sameRecord(recRecord, foundRecord) {
		return false
	}
	recParent, err := gitBead(home, "rev-parse", rec+"^")
	if err != nil {
		return false
	}
	foundParent, err := gitBead(home, "rev-parse", found+"^")
	if err != nil {
		return false
	}
	if strings.TrimSpace(string(recParent)) == strings.TrimSpace(string(foundParent)) {
		return false
	}
	return exemptRange(home, id, strings.TrimSpace(string(recParent)), found)
}

// exemptRange reports whether from is an ancestor of to and nothing in
// between puts id's line back — the ancestry-and-no-readdition test
// sameRemoval runs both against a record's own commit and, when history was
// rewritten out from under that commit, against its parent instead.
func exemptRange(home, id, from, to string) bool {
	if _, err := gitBead(home, "merge-base", "--is-ancestor", from, to); err != nil {
		return false
	}
	out, err := gitBead(home, "log", "--format=", "-p", "--diff-merges=first-parent",
		"--no-renames", from+".."+to, "--", beadsJSONL)
	if err != nil {
		return false
	}
	return !readdsID(out, id)
}

// sameRecord reports whether two ledger-carried JSONL lines describe the
// same bead state. Structural, not byte, equality: the ledger re-encodes its
// embedded record through encoding/json, which escapes HTML-sensitive
// characters on the way in and so can change bytes a plain string compare
// would trip over even for the same state (a bead title using `&` is enough).
// Either side missing or unparsable answers no — the false-alarm direction,
// never the silent one.
func sameRecord(a, b json.RawMessage) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	var ia, ib BdIssue
	if json.Unmarshal(a, &ia) != nil || json.Unmarshal(b, &ib) != nil {
		return false
	}
	return reflect.DeepEqual(ia, ib)
}

// readdsID reports whether any diff in out puts id's line back. A read it
// cannot finish answers yes: an unreadable range is no proof the bead stayed
// gone, and this mechanism's failure direction is a noisy alarm, never a
// quiet one.
func readdsID(out []byte, id string) bool {
	sc := bufio.NewScanner(bytes.NewReader(out))
	// The same monster-line cap removedBeads walks under.
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "+{") {
			continue
		}
		var is BdIssue
		if json.Unmarshal([]byte(line[1:]), &is) == nil && is.ID == id {
			return true
		}
	}
	return sc.Err() != nil
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
// .beads/redirect ONE hop when one is there. Everything the census touches —
// the git history of the JSONL, the deletion ledger — hangs off this one
// answer, so reader, writer and git can never disagree about which repo they
// are in. Which is why the hop count is bd's and not ours.
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
//
// A target that itself holds a redirect ends the walk, because that is where
// bd ends it. Measured on bd 0.49.1 (ranger-base-7kw), work → mid → store:
// bd prints "redirect chains not allowed, ignoring redirect in <mid>/.beads"
// and then treats mid as an ordinary beads dir — it reads mid's issues.jsonl
// and creates mid/beads.db, and store is never touched. Following to store
// would census a repo bd is not using, which is rangerhq-fuom's blindness by
// another route; and where mid holds no database bd errors outright, so the
// census of store was computed and then thrown away on the ListAll error.
// One hop also means a redirect cycle cannot loop here at all.
func beadsHome(dir string) string {
	home, target, _ := beadsRedirectHop(dir)
	if target != "" {
		return target
	}
	return home
}

// beadsRedirectHop is that one hop with the REASON kept, and it is the only
// reader of a redirect file in this package: beadsHome is a projection of it,
// and so is the second-store sweep (secondstore.go), so the census and the
// guard can never disagree about which directory bd is reading — the same
// rule beadsHome's comment above states for reader, writer and git.
//
// home is always <dir>/.beads. target is the directory bd will read INSTEAD,
// and "" when bd reads home itself; why then says which fallback that was,
// and is "" only when there is no redirect file there at all. So the three
// states a caller can be in are: no redirect (target and why both ""), a
// redirect bd follows (target set), and a redirect bd will not follow (why
// set) — which beadsHome collapses to two because it only needs the
// directory, and which the second-store sweep must keep apart because they
// take different sentences.
func beadsRedirectHop(dir string) (home, target, why string) {
	if dir == "" {
		dir = "."
	}
	home = filepath.Join(dir, beadsDirName)
	redirect := filepath.Join(home, beadsRedirect)
	// Lstat first, and never a read: absence is the overwhelmingly common
	// case and it is not a fallback anyone should be told about — a repo
	// with no redirect has not failed to follow one.
	if _, err := os.Lstat(redirect); err != nil {
		return home, "", ""
	}
	// isRegularFile (gates.go) before the open, the same guard the launch
	// path's other readers carry (ranger-base-gs9r, ranger-base-92n5p,
	// ranger-base-fvfve): os.ReadFile on a FIFO with no writer never returns,
	// and this read is on the launch path too — CheckParityIn's
	// applyRecordReach reaches it, measured blocking planLaunch past 60s. A
	// special file is never a redirect bd wrote and cannot be one, so it gets
	// the answer every other unreadable redirect already gets: the local
	// .beads, reached without the open.
	if !isRegularFile(redirect) {
		return home, "", "the redirect file is not a regular file"
	}
	b, err := os.ReadFile(redirect)
	if err != nil {
		return home, "", fmt.Sprintf("the redirect file cannot be read: %v", err)
	}
	// firstLine (cagelauncher.go): bd writes one path and nothing else.
	target = strings.TrimSpace(firstLine(string(b)))
	if target == "" {
		return home, "", "the redirect file names no path"
	}
	if !filepath.IsAbs(target) {
		// bd writes the relative form against the repo root, not against
		// .beads/ — one ".." off and bd falls back too.
		target = filepath.Join(filepath.Dir(home), target)
	}
	if st, err := os.Stat(target); err != nil || !st.IsDir() {
		return home, "", "it names " + AbbrevHome(target) + ", which is not a directory"
	}
	return home, target, ""
}

// ReadDeletionLedger returns the accounted-for deletions, ALL of them, keyed
// by id and in file order. Since rangerhq-6he5 a record owns one removal
// rather than an id, so an id legitimately carries one record per removal and
// a reader that keeps a single record per id keeps whichever line happened to
// land last — handing the verdict to the line order of an append-only trail
// that git merges and rebases replay (rangerhq-fknq). The caller asks whether
// ANY of an id's records covers the removal in hand; the slice is what lets
// it.
//
// A missing ledger is the normal case — no deletions have been owned yet —
// and a line that will not parse is skipped rather than failing the read: the
// ledger's job is to silence what it explains, never to break the check.
func ReadDeletionLedger(dir string) (map[string][]DeletionRecord, error) {
	b, err := os.ReadFile(beadsPath(dir, beadsDeleted))
	if os.IsNotExist(err) {
		return map[string][]DeletionRecord{}, nil
	}
	if err != nil {
		return nil, err
	}
	recs := map[string][]DeletionRecord{}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var r DeletionRecord
		if json.Unmarshal([]byte(line), &r) == nil && r.ID != "" {
			recs[r.ID] = append(recs[r.ID], r)
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
