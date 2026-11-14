package posse

// The runtime's own conversation id on an occupant with no `pane get` —
// ranger-base-qzujz, from ranger-base-r5131.
//
// WHAT IT IS FOR. sentline.go tells a composer holding a prompt from one
// echoing a line already submitted by asking claude's own submit log, and
// that log is keyed on claude's SESSION ID. On herdr the id comes from the
// multiplexer: `pane get`'s `agent_session.value`, which is ADR 0076 §4 and
// answers "not available" on a `multiplexer: rex` instance. So the echo
// reading could not be asked there, every echo read as a hold, and
// ranger-base-r5131 refused to take a Typed reading on the capture route at
// all rather than invent holds (ADR 0076, Amendment 5, "The composer half
// stays §4's"). This is the join that reopens it.
//
// THE STORE OF RECORD, and it is claude's rather than Rex's. MEASURED
// 2026-11-10 on this host (linux, rex CLI 0.0.1794069975, claude 2.1.322),
// over the five live seats on the box plus one scratch seat created for the
// arm:
//
//  1. `program_status` carries NOTHING about the conversation. Its root
//     record is `state`, `app`, `title`, `msg`, `updated_at` and an `owner`
//     — a pid and a process name. No session id, no transcript path.
//  2. Rex has no send history to ask. `rex capture` is a screen and `rex
//     send` answers nothing; the block holds no record of what was typed
//     into it.
//  3. claude keeps one file per LIVE session under its own config dir:
//     `$CLAUDE_CONFIG_DIR/sessions/<pid>.json`, carrying `pid`, `sessionId`,
//     `cwd`, `procStart`, `version` and a `status`. So the join is the
//     OWNER's pid — which `program_status` does carry, and which ADR 0076 §3
//     already checks against the block's process table before it believes a
//     record at all.
//
// The chain, end to end on a live posse seat: block
// `block:01a1c83d-…-492d43a87c2e` → `program_status` owner
// `{pid:2069699,name:"claude"}` → `sessions/2069699.json` →
// `sessionId:"0226278e-a243-4f86-9200-5cd648eac89f"` → a `history.jsonl` row
// carrying that sessionId and `project` = that seat's worktree. Every link
// MEASURED in that run.
//
// PID REUSE IS THE ONE HAZARD AND IT IS FENCED. Rex says so about its own
// owner field ("Compare it as well as pid, because the system reuses process
// IDs" — RexOwner), and this read is one layer further out: a file NAMED by
// a pid. claude writes `procStart` into the row, and it is the kernel's own
// start time for that process — field 22 of `/proc/<pid>/stat`, byte for byte
// (MEASURED on all six seats above: 24545899, 24413296, 24296903, 23912539,
// 24499783, 24585257, every one an exact string match). So the row is
// accepted only when it agrees with the kernel about WHICH process that pid
// is now. A platform with no such reading answers "not available" and the
// echo reading stays off, which is the direction the whole family takes.
//
// The file is also REMOVED when the session ends — MEASURED twice in that
// run: 49 session-env entries against 5 session rows, and a seat whose
// claude exited mid-measurement (pid 3184881) had its row gone at the next
// listing, as did the scratch seat after `rex kill`. That is not relied on.
// The fence is what makes a stale row harmless; the cleanup only makes one
// rare.
//
// THE CWD IS IN THE ROW AND IS NOT READ, said out loud because it is the
// obvious second guard. `sessions/<pid>.json` carries the seat's cwd and so
// does Rex's `process` answer, so the two could be compared — but posse does
// not parse `cwd` out of `process` today (RexProc is a pid and a name), and
// the procStart fence already answers the question a cwd check would be
// approximating: is this row about the process whose keyboard we are reading.
// A second, weaker guard beside an exact one is a second thing to keep true.
//
// EVERY FAILURE ANSWERS "NO SESSION", the same concession sentline.go and
// panework.go make: no row, an unreadable one, a row in a shape this does not
// know, a pid the kernel will not answer for, a fence that disagrees, an
// owner that is not claude. All of them are an error rather than an empty
// string, so a caller cannot mistake "posse would not say" for "there is no
// session" — and its one caller treats any error as "not an echo", which
// leaves the composer a hold exactly as it was before this file.

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

const (
	// claudeSessionsDir is the directory claude keeps one row per live
	// session in, beside the submit log sentline.go reads.
	claudeSessionsDir = "sessions"

	// claudeRuntimeProcName is the process name claude's own binary carries
	// in Rex's `owner` and `process` answers (MEASURED 2026-11-10: `owner`
	// `{pid:2069699,name:"claude"}` on every seat on the box, and
	// `foreground.name` the same). Only claude's shape is answered here,
	// for the reason Herdr.PaneAgentSession answers only claude's: the store
	// this feeds is claude's.
	claudeRuntimeProcName = "claude"

	// claudeSessionRowMax bounds the row read. The live rows measure 591-672
	// bytes, so this is slack and not a budget; it is here so that a path
	// which is somehow enormous cannot be read into memory by a function
	// that runs inside govern's G2 tick and dispatch's settle judgments.
	claudeSessionRowMax = 64 << 10
)

// claudeSessionRow is the part of `sessions/<pid>.json` this reads. claude
// writes pid, sessionId, cwd, startedAt, procStart, version, peerProtocol,
// peerFeatures, kind, entrypoint, pidDomain, messagingSocketPath, name,
// nameSource, nameSince, status, updatedAt and statusUpdatedAt (2.1.322);
// nothing here depends on the others being absent.
type claudeSessionRow struct {
	PID       int    `json:"pid"`
	SessionID string `json:"sessionId"`
	// ProcStart is the kernel's start time for PID, as claude read it. It is
	// a STRING here because that is how claude writes it and because nothing
	// here does arithmetic on it: the fence is an exact comparison against
	// the same field read back from the kernel, and parsing both sides into
	// numbers would only add a way for the two to disagree about a value
	// they agree about as bytes.
	ProcStart string `json:"procStart"`
}

// claudeSessionRowPath is the row for one pid, beside the submit log at
// `history`, or "" when there is no store to read.
//
// IT IS DERIVED FROM THE ONE FIELD rather than resolved a second time
// through ClaudeConfigDirIn, and that is the point. HerdrBackend.ClaudeHistory
// is filled with the operator's real log by NewHerdrBackend and left empty by
// every test backend, precisely so that "a test must never read the
// operator's own prompt history" (herdrback.go). A second resolution here
// would have read the operator's real `sessions/` from a backend that reads
// no history at all — the two stores are one store, and a reading that is off
// must be off in both halves.
func claudeSessionRowPath(history string, pid int) string {
	if history == "" || pid <= 0 {
		return ""
	}
	return filepath.Join(filepath.Dir(history), claudeSessionsDir, strconv.Itoa(pid)+".json")
}

// claudeSessionIn reads one row and fences it against `start` — the kernel's
// own start time for that pid, passed in because it comes from a
// platform-specific file and this is the rule rather than the reading.
func claudeSessionIn(path string, pid int, start string) (string, error) {
	if path == "" {
		return "", Die("no claude session store to read for pid %d", pid)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", Die("claude session row for pid %d: %v", pid, err)
	}
	if !info.Mode().IsRegular() {
		// lastSubmitted's concession, for its reason: os.Open on a FIFO with
		// no writer blocks in-process, and this read runs inside govern's G2
		// tick and dispatch's settle judgments.
		return "", Die("claude session row for pid %d is not a regular file", pid)
	}
	f, err := os.Open(path)
	if err != nil {
		return "", Die("claude session row for pid %d: %v", pid, err)
	}
	defer f.Close()
	buf, err := io.ReadAll(io.LimitReader(f, claudeSessionRowMax))
	if err != nil {
		return "", Die("claude session row for pid %d: %v", pid, err)
	}
	var row claudeSessionRow
	if err := json.Unmarshal(buf, &row); err != nil {
		return "", Die("claude session row for pid %d is not a shape posse knows", pid)
	}
	if row.PID != pid || row.SessionID == "" {
		return "", Die("claude session row for pid %d names pid %d and session %q", pid, row.PID, row.SessionID)
	}
	if start == "" || row.ProcStart != start {
		// The fence. A row whose procStart does not match the kernel's is
		// about a DIFFERENT process that held this pid, so its session id
		// belongs to somebody else's conversation — and borrowing one would
		// read another seat's submits as this seat's echo.
		return "", Die("claude session row for pid %d was written for a process started at %q and the kernel says %q — that pid has been reused", pid, row.ProcStart, start)
	}
	return row.SessionID, nil
}

// claudePrivateSessionRowPath is the row for one pid in the seat-private
// sessions/ a caged claude seat registers into (ranger-base-lx5jk): the
// SeatCacheDir of the directory the KERNEL says that pid runs in, under the
// home the private bind was rendered against. "" when there is no home — every
// test backend, by the same discipline as ClaudeHistory — or no cwd reading.
func claudePrivateSessionRowPath(home, cwd string, pid int) string {
	seat := SeatCacheDir(home, cwd)
	if seat == "" || pid <= 0 {
		return ""
	}
	return filepath.Join(seat, claudePrivateSessionStateRel, claudeSessionsDir, strconv.Itoa(pid)+".json")
}

// claudeSessionOfPID is claudeSessionIn with the fence taken, over the
// seat's PRIVATE row first and the shared one second (ranger-base-lx5jk).
//
// WHY TWO ROWS. A caged claude seat's sessions/ is a private bind
// (claudeSessionStateDirNames), so its row is no longer in the shared dir at
// all; without this read every caged seat's echo reading would go off. The
// private dir is found through /proc/<pid>/cwd — the kernel's reading, the
// same file family as the fence — and never through the row's own `cwd`,
// which would be the thing being located saying where it is.
//
// WHY THIS ORDER IS SAFE. After the bind, only that seat (or the host) can
// write its private row, and only the host can write the shared one. So
// either source is honest about the pid it names, and a seat that lies in its
// own private row misleads only its own echo reading. The fence is taken on
// EACH row, and a private row that is absent or refused falls through to the
// shared one: an uncaged seat has no private dir, and that is its normal case.
func claudeSessionOfPID(history, home string, pid int) (string, error) {
	start, err := procStartOf(pid)
	if err != nil {
		return "", err
	}
	if home != "" && history != "" {
		if cwd, err := procCwdOf(pid); err == nil {
			if id, err := claudeSessionIn(claudePrivateSessionRowPath(home, cwd, pid), pid, start); err == nil {
				return id, nil
			}
		}
	}
	return claudeSessionIn(claudeSessionRowPath(history, pid), pid, start)
}

// rexAgentSession is paneAgentSession's Rex half: the runtime's own
// conversation id for a block, through the block's own owner.
//
// THE OWNER IS ASKED OF THE OCCUPANT and not of the process table directly,
// because ADR 0076 §3's guards are what make a pid worth joining on: a record
// with no owner, a record whose owner is the block's own SHELL, and a record
// that outlived the process that wrote it all have to fail here, and all
// three are already decided in muxrex.go. A pid taken past them would be a
// pid posse has no evidence is a live runtime.
func (b *HerdrBackend) rexAgentSession(rm RexMux, target Seat) (string, error) {
	_, sess, err := b.rexClaudeOwnerSession(rm, target)
	return sess, err
}

// rexClaudeOwnerSession is rexAgentSession with the owner's pid kept: the
// submit-log read needs it to find a caged seat's private log
// (lastSubmittedOfPID, ranger-base-a544g), and asking the occupant twice
// could answer two different owners.
func (b *HerdrBackend) rexClaudeOwnerSession(rm RexMux, target Seat) (int, string, error) {
	owner, err := rm.RuntimeOwner(target)
	if err != nil {
		return 0, "", err
	}
	if owner.Name != claudeRuntimeProcName {
		return 0, "", Die("rex block %s is held by %s(%d) and not claude — the submit log this id is for is claude's own", target.Pane, owner.Name, owner.PID)
	}
	sess, err := claudeSessionOfPID(b.ClaudeHistory, b.ClaudeSeatHome, owner.PID)
	return owner.PID, sess, err
}

// paneLastSubmitted is sentline's echo question for one pane: the last line
// the pane's claude session submitted, and whether any store answered. Both
// echo callers (panework.go, capturedcomposer.go) ask it, so the order below
// is spelled once.
//
// On rex the owner pid is known, so the seat's private log is read before the
// shared one (lastSubmittedOfPID). On herdr there is no owner pid, so the read
// stays shared-only and a caged herdr seat reads as a hold, the safe direction
// (ranger-base-tgc4b D2). Every failure on the way answers "no".
func (b *HerdrBackend) paneLastSubmitted(target Seat) (string, bool) {
	if hm, ok := b.herdrMux(); ok {
		sess, err := hm.PaneAgentSession(target)
		if err != nil {
			return "", false
		}
		return lastSubmitted(b.ClaudeHistory, sess)
	}
	if rm, ok := b.rexMux(); ok {
		pid, sess, err := b.rexClaudeOwnerSession(rm, target)
		if err != nil {
			return "", false
		}
		return lastSubmittedOfPID(b.ClaudeHistory, b.ClaudeSeatHome, pid, sess)
	}
	return "", false
}
