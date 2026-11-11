//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-qzujz's half of the join: the runtime's own
// conversation id on an occupant with no `pane get`.
//
// WHAT CAN SILENTLY UNDO IT. The reading is three steps — the block's owner,
// the pid's session row, the id — and each one has a way to be wrong that
// looks right:
//
//  1. THE OWNER. A pid taken past ADR 0076 §3's guards is a pid posse has no
//     evidence is a live runtime: a record the block's own SHELL wrote, or
//     one that outlived the process that wrote it, would hand over a pid
//     whose session row belongs to somebody else. RuntimeOwner must refuse
//     exactly where State's reading refuses to be Seen, and that agreement
//     is pinned rather than asserted, because the two spell it in two places.
//  2. THE FENCE. A session row is a file NAMED by a pid, so a reused pid is
//     a row about another conversation. claude writes the kernel's own start
//     time into it and the row is accepted only when the kernel still agrees
//     (MEASURED 2026-11-10 over six live claude processes, an exact string
//     match in every one). A fence that is deleted, or that passes on an
//     empty value, reads another seat's submits as this seat's echo — and
//     nothing else in the suite would go red.
//  3. THE STORE BEING OFF. HerdrBackend.ClaudeHistory is empty in every test
//     backend precisely so that no test reads the operator's own prompt
//     history. The sessions store is the same store, so it must be off by
//     exactly the same bit; a second resolution through the config dir would
//     have read the operator's real `sessions/` out of a backend that reads
//     no history at all.
//
// The mechanism and its measurement are in runtimesession.go.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const qzujzSession = "0226278e-a243-4f86-9200-5cd648eac89f"

// noVerbMux is an occupant that is neither herdr nor Rex. The embedded nil
// interface is never called: the door under test decides by type assertion
// alone, which is the whole of ADR 0076 §4.
type noVerbMux struct{ Mux }

// armClaudeSessionRow writes one `sessions/<pid>.json` beside a history path
// the backend is pointed at, in the shape the live rows have (claude 2.1.322:
// pid, sessionId, cwd, startedAt, procStart, version, … — MEASURED
// 2026-11-10). Returns the history path, which is the only thing production
// code is told about.
func armClaudeSessionRow(t *testing.T, b *HerdrBackend, pid int, row map[string]any) string {
	t.Helper()
	dir := t.TempDir()
	history := filepath.Join(dir, "history.jsonl")
	if err := os.WriteFile(history, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, claudeSessionsDir), 0o700); err != nil {
		t.Fatal(err)
	}
	if row != nil {
		raw, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(dir, claudeSessionsDir, strconv.Itoa(pid)+".json")
		if err := os.WriteFile(p, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if b != nil {
		b.ClaudeHistory = history
	}
	return history
}

// ownProcStart is this test process's own start time as the kernel reports
// it, or a skip on a platform with no such reading — the fence is linux's and
// says so (procstart_other.go).
func ownProcStart(t *testing.T) string {
	t.Helper()
	start, err := procStartOf(os.Getpid())
	if err != nil {
		t.Skipf("no process start time on this platform, so the pid-reuse fence cannot be read: %v", err)
	}
	if start == "" {
		t.Fatal("procStartOf answered an empty start time with no error — the fence would then pass on nothing")
	}
	return start
}

// ─── the row, and the fence over it ──────────────────────────────────────────

func TestQAAClaudeSessionRowIsFencedAgainstPidReuse(t *testing.T) {
	t.Parallel()

	const pid = 4242
	for _, c := range []struct {
		name  string
		row   map[string]any
		start string
		want  string // "" = the id is answered
	}{
		{
			name:  "the live shape answers its session id",
			row:   map[string]any{"pid": pid, "sessionId": qzujzSession, "cwd": "/w", "procStart": "24545899", "version": "2.1.322", "status": "busy"},
			start: "24545899",
		},
		{
			// THE FENCE. The row is claude's, well-formed, and about a
			// process that is gone; this pid is somebody else's now, and
			// answering would read another conversation's submits as this
			// seat's echo.
			name:  "a row written for a process that held this pid before is refused",
			row:   map[string]any{"pid": pid, "sessionId": qzujzSession, "procStart": "24413296"},
			start: "24545899",
			want:  "reused",
		},
		{
			// A fence with nothing to compare against must refuse, not pass.
			// This is the arm that reds if the kernel reading is ever allowed
			// to answer "" without an error.
			name:  "a kernel start time nobody could read is refused",
			row:   map[string]any{"pid": pid, "sessionId": qzujzSession, "procStart": "24545899"},
			start: "",
			want:  "reused",
		},
		{
			name:  "a row carrying no procStart at all is refused",
			row:   map[string]any{"pid": pid, "sessionId": qzujzSession},
			start: "24545899",
			want:  "reused",
		},
		{
			// BOTH SIDES EMPTY, which is the one shape the second disjunct
			// cannot catch: a row with no procStart compares EQUAL to a
			// kernel reading nobody could take, and `start == ""` is the
			// only thing that refuses it (ranger-base-1tt2e, from
			// ranger-base-s3a9g). MEASURED: reducing the fence to
			// `if row.ProcStart != start {` left the whole package green,
			// because the two cases above each differ on the other side.
			// Defensive rather than live today — procStartOf errors where
			// there is no /proc (procstart_other.go), so claudeSessionIn is
			// not reached with an empty start from that path — and pinned
			// because the whole echo reading rests on this fence.
			name:  "no procStart and no kernel reading do not agree",
			row:   map[string]any{"pid": pid, "sessionId": qzujzSession},
			start: "",
			want:  "reused",
		},
		{
			name:  "a row about another pid is refused",
			row:   map[string]any{"pid": pid + 1, "sessionId": qzujzSession, "procStart": "24545899"},
			start: "24545899",
			want:  "names pid",
		},
		{
			name:  "a row with no session id is refused",
			row:   map[string]any{"pid": pid, "procStart": "24545899"},
			start: "24545899",
			want:  "names pid",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			history := armClaudeSessionRow(t, nil, pid, c.row)
			got, err := claudeSessionIn(claudeSessionRowPath(history, pid), pid, c.start)
			switch {
			case c.want == "" && err != nil:
				t.Fatalf("claudeSessionIn = %v, want the session id", err)
			case c.want == "" && got != qzujzSession:
				t.Fatalf("claudeSessionIn = %q, want %q", got, qzujzSession)
			case c.want != "" && err == nil:
				t.Fatalf("claudeSessionIn answered %q where it must refuse — %s", got, c.name)
			case c.want != "" && !strings.Contains(err.Error(), c.want):
				t.Errorf("refusal %q does not say %q, so a reader cannot tell which guard spoke", err, c.want)
			}
		})
	}
}

func TestQAANoSessionRowIsAnErrorAndNeverAnEmptyId(t *testing.T) {
	t.Parallel()

	// A caller must not be able to mistake "posse would not say" for "there
	// is no session": its one caller treats an error as "not an echo" and
	// leaves the composer a hold, where an empty id would be handed to
	// lastSubmitted, which refuses a "" session for its own reasons — the
	// same answer by luck rather than by rule.
	history := armClaudeSessionRow(t, nil, 4242, nil)
	for _, c := range []struct{ name, path string }{
		{"no row for this pid", claudeSessionRowPath(history, 4242)},
		{"no store at all", claudeSessionRowPath("", 4242)},
		{"a directory where a row should be", filepath.Dir(claudeSessionRowPath(history, 4242))},
	} {
		if got, err := claudeSessionIn(c.path, 4242, "1"); err == nil {
			t.Errorf("%s: claudeSessionIn answered %q with no error", c.name, got)
		}
	}
}

func TestQAATheSessionStoreIsOffExactlyWhenTheHistoryStoreIs(t *testing.T) {
	t.Parallel()

	// herdrback.go's discipline: a backend built without ClaudeHistory reads
	// NO store, so that a test can never read the operator's own prompt
	// history. The session rows live beside that log and are the same store,
	// so they must be off by the same bit — a second resolution through
	// ClaudeConfigDirIn would have reached the operator's real `sessions/`
	// out of a backend that reads no history at all.
	if got := claudeSessionRowPath("", 4242); got != "" {
		t.Errorf("claudeSessionRowPath with no history store = %q, want \"\" — that path is the operator's own session store", got)
	}
	if got := claudeSessionRowPath("/x/history.jsonl", 0); got != "" {
		t.Errorf("claudeSessionRowPath with no pid = %q, want \"\"", got)
	}
	want := filepath.Join("/x", claudeSessionsDir, "7.json")
	if got := claudeSessionRowPath("/x/history.jsonl", 7); got != want {
		t.Errorf("claudeSessionRowPath = %q, want %q — the rows are beside the log, not resolved a second way", got, want)
	}
}

// ─── the owner, and §3's guards over it ──────────────────────────────────────

// rexSeedOwnedBy puts rexTestSeat on the fake server with one root record
// owned by `owner`, and a process table in which `live` names the processes
// the block actually holds.
//
// The owner is a POINTER because nil is one of the three §3 causes, and a
// cause this helper could not say was a cause RuntimeOwner's agreement pin
// could not carry a row for (ranger-base-1tt2e): Rex leaves `owner` out when
// it could not tell the block's foreground process, which is a record that
// says nothing about a runtime having written it.
func rexSeedOwnedBy(t *testing.T, owner *RexOwner, proc RexProcess) RexMux {
	t.Helper()
	m, _ := newTestRexMux(t)
	rexSeedTestSeat(t)
	rexSeedProcess(t, rexTestBlock, proc)
	rexSeedStatus(t, rexTestBlock, []RexStatusRecord{{State: "idle", App: "claude-code", Owner: owner}})
	return m
}

// claudeProcTable is a block running claude under the owner pid given: the
// shell is `child`, claude is `foreground`, the shell is between them
// (RexProcess's measured shape).
func claudeProcTable(pid int) RexProcess {
	return RexProcess{
		Child:      &RexProc{PID: 10, Name: "zsh"},
		Foreground: &RexProc{PID: pid, Name: claudeRuntimeProcName},
		Ancestors:  []RexProc{{PID: 10, Name: "zsh"}},
	}
}

func TestQAARuntimeOwnerAnswersExactlyWhenTheReadingIsSeen(t *testing.T) {
	t.Parallel()

	// THE AGREEMENT PIN. §3's three guards decide both whether a record is a
	// reading at all (rexReading, which sets Seen) and whether its pid may be
	// joined on (RuntimeOwner). They are spelled in two places, so this is
	// the arm that reds when one of them moves and the other does not —
	// which is the shape of bug that would hand a session row's id to a seat
	// posse has no evidence a runtime is in.
	for _, c := range []struct {
		name  string
		owner *RexOwner
		proc  RexProcess
	}{
		{"a live runtime owns the record", &RexOwner{PID: 20, Name: claudeRuntimeProcName}, claudeProcTable(20)},
		{"the block's own shell wrote it", &RexOwner{PID: 10, Name: "zsh"}, claudeProcTable(20)},
		{"its owner is gone", &RexOwner{PID: 999, Name: claudeRuntimeProcName}, claudeProcTable(20)},
		{"the owner's name no longer matches that pid", &RexOwner{PID: 20, Name: "python3"}, claudeProcTable(20)},
		// THE THIRD CAUSE THE DOC COMMENT ENUMERATES, which had no row
		// (ranger-base-1tt2e, from ranger-base-s3a9g): all four rows above
		// carry a non-nil Owner, so "a record with no owner" was unpinned
		// and `return RexOwner{}, nil` in its place left the whole package
		// green. A zero owner is pid 0, and the caller is about to read a
		// file NAMED by that pid — which is the whole of why muxrex.go's
		// comment says an error is the only other answer.
		{"its program_status record names no owner", nil, claudeProcTable(20)},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := rexSeedOwnedBy(t, c.owner, c.proc)
			// Two record sets, because each verb consumes one from the queue.
			rexSeedStatus(t, rexTestBlock, []RexStatusRecord{{State: "idle", App: "claude-code", Owner: c.owner}})
			read, err := m.State(rexTestSeat())
			if err != nil {
				t.Fatalf("State: %v", err)
			}
			_, ownerErr := m.RuntimeOwner(rexTestSeat())
			if read.Seen != (ownerErr == nil) {
				t.Fatalf("State().Seen = %v but RuntimeOwner err = %v — the two read ADR 0076 §3's guards apart", read.Seen, ownerErr)
			}
		})
	}
}

func TestQAARexAgentSessionJoinsTheBlocksOwnerToClaudesSessionRow(t *testing.T) {
	t.Parallel()

	start := ownProcStart(t)
	pid := os.Getpid()
	b, _ := newTestBackend(t)
	b.Slot = rexSeedOwnedBy(t, &RexOwner{PID: pid, Name: claudeRuntimeProcName}, claudeProcTable(pid))
	armClaudeSessionRow(t, b, pid, map[string]any{
		"pid": pid, "sessionId": qzujzSession, "cwd": "/w", "procStart": start, "version": "2.1.322",
	})

	// Through the §4 DOOR, not through the Rex half directly: the point of
	// Amendment 6 is that the fact stops being herdr-only without its caller
	// learning which multiplexer answered.
	got, err := b.paneAgentSession(rexTestSeat())
	if err != nil {
		t.Fatalf("paneAgentSession on a Rex slot: %v — this is the reading ranger-base-r5131 had to refuse the composer half over", err)
	}
	if got != qzujzSession {
		t.Fatalf("paneAgentSession = %q, want %q", got, qzujzSession)
	}
}

func TestQAARexAgentSessionRefusesARuntimeThatIsNotClaude(t *testing.T) {
	t.Parallel()

	// The store this id is for is claude's own submit log, which is why
	// Herdr.PaneAgentSession answers only claude's shape too. A codex or a
	// grok seat has no row in it, and borrowing the join would read a
	// `sessions/<pid>.json` that is not about that seat at all.
	// THE ROW IS ARMED WITH THE KERNEL'S OWN START TIME (ranger-base-1tt2e,
	// from ranger-base-s3a9g). With `"procStart": "1"` against the live value
	// for os.Getpid() the pid-reuse fence refused first, and its message
	// carries the word "claude" too — so both assertions below passed with
	// the name guard NEUTERED, MEASURED over the whole package. Armed as the
	// sibling arm above arms it, only the name guard can refuse.
	start := ownProcStart(t)
	pid := os.Getpid()
	b, _ := newTestBackend(t)
	b.Slot = rexSeedOwnedBy(t, &RexOwner{PID: pid, Name: "codex"},
		RexProcess{Child: &RexProc{PID: 10, Name: "zsh"}, Foreground: &RexProc{PID: pid, Name: "codex"},
			Ancestors: []RexProc{{PID: 10, Name: "zsh"}}})
	armClaudeSessionRow(t, b, pid, map[string]any{"pid": pid, "sessionId": qzujzSession, "procStart": start})

	if got, err := b.paneAgentSession(rexTestSeat()); err == nil {
		t.Fatalf("paneAgentSession answered %q for a block held by codex", got)
	} else if !strings.Contains(err.Error(), "claude") {
		t.Errorf("refusal %q does not name claude, so a reader cannot tell why the join was declined", err)
	}
}

func TestQAAnOccupantWithNeitherRouteStillAnswersSection4(t *testing.T) {
	t.Parallel()

	// A slot that is neither herdr nor Rex has no route to the runtime's own
	// id, and §4's answer is "not available" — a typed silence, so that the
	// capture route can tell it from a verb that was asked and failed
	// (muxVerbMissing, ranger-base-l32dl).
	b, _ := newTestBackend(t)
	b.Slot = noVerbMux{}
	_, err := b.paneAgentSession(rexTestSeat())
	if err == nil {
		t.Fatal("paneAgentSession answered on an occupant with no such verb")
	}
	if verb, missing := muxVerbMissing(err); !missing {
		t.Fatalf("paneAgentSession refused with %v, which is not §4's typed silence", err)
	} else if !strings.Contains(verb, "agent_session") {
		t.Errorf("the §4 refusal names the verb %q, want the runtime-session read", verb)
	}
}
