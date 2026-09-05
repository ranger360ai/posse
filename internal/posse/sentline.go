package posse

// Whether a composer is holding a prompt, or echoing one already sent —
// ranger-base-2hvtv, from ranger-base-wr624.
//
// THE READING THIS ANSWERS FOR. panework.go reads claude's `prompt_box_body`
// region and calls any text there "a prompt sitting UNSENT in its box". It is
// a screen-state matcher, and ranger-base-wr624 measured what that costs: the
// pulse skipped ~586 times over ~10 hours of 2026-09-04 on lines the operator
// had already sent, because a matcher over a region that can hold text
// nobody is about to send cannot go false on its own. wr624 took the reading
// out of the pulse's delivery path and left the two callers that still act on
// it — dispatch's --resume skip and govern's G2 row — for this bead, with
// three questions to measure before building anything.
//
// MEASURED 2026-09-05 04:0x-04:3xZ, herdr 0.8.2, claude 2.1.261, on the live
// fleet. Each of the three, and what it settled:
//
//  1. "does herdr clear its composer state on submit?" is mis-framed: herdr
//     holds no composer state. `agent explain`'s region preview, `posse
//     peek` (pane read --format text) and a raw `--format ansi` read of the
//     same pane AGREE, character for character, on both boxes that carried
//     text. The reading tracks the screen, so there is nothing herdr caches
//     and nothing to report upstream.
//
//  2. `Typed == the last text POSSE sent` is not sound, because posse is not
//     who sent it. Both live boxes carried lines posse never typed — one the
//     operator's, one the coordinator's — and wr624's three episodes are all
//     the operator's own. A comparison against posse's sends can only ever
//     answer for posse's sends, and the 243-character preview truncation
//     panework.go documents makes even that prefix-shaped.
//
//  3. herdr's `agent_status` does not disagree with the detector, so it is
//     not the cheap discriminator either. The rule that produces the reading
//     — `live_prompt_box`, claude manifest 2026.08.31.1 — is keyed on
//     `^\s*❯` alone and reports idle whether the box is empty or full;
//     measured on one pane with text and one without, both idle, both
//     live_prompt_box.
//
// WHAT DOES ANSWER IT. claude keeps its own submitted-prompt log —
// `$CLAUDE_CONFIG_DIR/history.jsonl`, one JSON object per submit, carrying
// the text, the project cwd, the claude session id and a timestamp — and
// that file is the STORE OF RECORD for "what was submitted to this pane".
// The screen region is a derived copy of a fact that store already owns
// (Helland, CIDR 2005: data outside its store of record "is clearly from the
// past and not now"), and the whole defect is a derived copy being read as
// the authority. So: a box previewing the line this pane LAST SUBMITTED is
// echoing it, not holding it.
//
// It answers for every writer, which is the half `Typed == last posse sent`
// could not reach: a machine submit lands there too — the coordinator's own
// pulse prompt is in the file at 2026-09-05T00:27:06Z, typed by `herdr agent
// prompt` — and so does the operator's. Measured against wr624's own three
// episodes, by their submit times in that file:
//
//	how did the day go?     submitted 2026-09-04T16:43:21Z, skipped 13:05-16:40Z
//	how we doing?           submitted 2026-09-04T20:34:14Z, skipped ~20:0x-00:1xZ
//	ok will check in later   submitted 2026-09-04T20:43:17Z, skipped 00:2x-03:2xZ
//
// The second and third skipped for hours AFTER their submit — those are the
// ones this retires. The first skipped only BEFORE its submit: the operator
// really was sitting on unsent text for three and a half hours, and this
// leaves every one of those 222 skips standing. That is the wrong arm, and
// it is not hypothetical — both boxes on the live fleet at 04:2xZ carried
// text that is NOT in the history file, so both stay holds today.
//
// IT GOES FALSE ON ITS OWN, which is the property the screen matcher lacked:
// the moment the operator sends, claude appends a row and the box stops
// being a hold without anybody clearing anything.
//
// WHAT IT CANNOT SEE, said out loud. A human who RETYPES the line this pane
// most recently submitted, and leaves it unsent, is read as an echo. It is
// the only false negative in the mechanism and it is bounded by taking the
// LAST row only: to be mistaken the retyped line must also be the very last
// thing submitted here, so what posse then types over is a duplicate of what
// was just sent rather than a new thought. Widening the comparison to "any
// row in this session's history" would trade that bound away — the operator
// sends "how we doing?" 18 times in this store — so it is deliberately not
// widened.
//
// EVERY READING FAILS TOWARDS TODAY, the same concession panework.go makes:
// no history file, an unreadable one, rows in a shape this does not know, a
// pane herdr will not name a claude session for — all of them answer "no
// echo", the text stays a hold, and the two callers behave exactly as they
// did before this bead. Ignorance is not evidence that a prompt was sent.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// claudeHistoryFile is the log claude appends a row to on every submit,
// under its config dir. The dir is CLAUDE_CONFIG_DIR when the launch set one
// (posse's own panes inherit it) and ~/.claude otherwise; on this box the
// two are the same path.
const claudeHistoryFile = "history.jsonl"

// historyTailBytes bounds the read. The live file is 2.7MB / 11818 rows
// after months of use and only the LAST row for a session is wanted, so
// this reads the end and walks backwards. A session whose last submit is
// further back than this window answers "no echo" — the safe direction, and
// a session that quiet is not one a settled-holder check is about to
// re-prompt on the strength of its composer.
const historyTailBytes = 512 << 10

// submittedRow is the part of a history row this reads. claude writes
// display, pastedContents, project, sessionId and timestamp (2.1.261);
// nothing here depends on the others being absent.
type submittedRow struct {
	Display   string `json:"display"`
	SessionID string `json:"sessionId"`
}

// claudeHistoryPath is the store of record's path, or "" when there is no
// home to hang it off.
func claudeHistoryPath() string {
	if d := strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR")); d != "" {
		return filepath.Join(d, claudeHistoryFile)
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".claude", claudeHistoryFile)
}

// lastSubmitted returns the text of the most recent prompt submitted in the
// named claude session, and whether the store answered at all. A "" session
// is never matched: an unknown pane must not borrow another pane's sends.
func lastSubmitted(path, session string) (string, bool) {
	if path == "" || session == "" {
		return "", false
	}
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", false
	}
	off, whole := int64(0), true
	if info.Size() > historyTailBytes {
		off, whole = info.Size()-historyTailBytes, false
	}
	if _, err := f.Seek(off, io.SeekStart); err != nil {
		return "", false
	}
	buf, err := io.ReadAll(io.LimitReader(f, historyTailBytes+1))
	if err != nil {
		return "", false
	}
	if !whole {
		// The window opens mid-row; that row belongs to the previous read.
		if i := bytes.IndexByte(buf, '\n'); i >= 0 {
			buf = buf[i+1:]
		} else {
			return "", false
		}
	}
	last, found := "", false
	sc := bufio.NewScanner(bytes.NewReader(buf))
	sc.Buffer(make([]byte, 0, 64<<10), 4<<20)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var row submittedRow
		if err := json.Unmarshal(line, &row); err != nil {
			// One unreadable row is not an unreadable store: claude may add
			// a shape tomorrow, and the rows around it still answer.
			continue
		}
		if row.SessionID != session {
			continue
		}
		last, found = row.Display, true
	}
	return last, found
}

// submittedEcho reports whether the composer preview IS the line the pane
// last submitted.
//
// Equality is the whole test when the preview arrived whole. A prefix is
// accepted only when herdr TRUNCATED the region — measured at 243 characters
// (panework.go), and detectable per reading because `agent explain` reports
// the region's real byte count beside the preview it cut down. Without that
// guard a short composer would match every long submit that happens to start
// with it, which is the containment mistake a prefix comparison invites.
func submittedEcho(typed, sent string, truncated bool) bool {
	typed, sent = strings.TrimSpace(typed), strings.TrimSpace(sent)
	if typed == "" || sent == "" {
		return false
	}
	if typed == sent {
		return true
	}
	return truncated && len(typed) < len(sent) && strings.HasPrefix(sent, typed)
}
