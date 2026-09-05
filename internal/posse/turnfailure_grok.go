package posse

// grok's half of the turn_outcome: registry — the reader ADR 0013 §1's
// promotion rule licenses only after the refusal artifact in that runtime's
// OWN session store has been captured and pinned as a fixture
// (docs/adr/0013-turn-outcome-refusal-probe.md, ranger-base-e123).
//
// claude writes an account refusal as a synthetic ASSISTANT message inside
// its own transcript, so FindClaudeTurnOutcome reads prose and has to
// pattern-match it. grok does not, and that is the whole reason this is a
// separate reader rather than a second regexp: a refused turn leaves grok's
// `chat_history.jsonl` silent — no assistant record at all, so a transcript
// scan sees `observed=false`, not a signal — and the failure is recorded as
// typed JSON one file over. MEASURED on the real artifact:
//
//	$GROK_HOME/sessions/<url-encoded cwd>/<id>/updates.jsonl
//	refused: {"sessionUpdate":"turn_completed","stop_reason":"error",
//	          "agent_result":"API error (status 402 Payment Required): …"}
//	served:  {"sessionUpdate":"turn_completed","stop_reason":"end_turn",
//	          "usage":{…}}
//
// # What this keys on, and what it deliberately does not
//
// `stop_reason` plus `agent_result`, and nothing else, for WHETHER the turn
// was refused. The probe listed three discriminators; the other two are not
// load-bearing for that question and one of them is measurably wrong:
//
//   - "a served turn always carries `usage`; a refused one never does" is
//     FALSE off the two-session sample. Censused over this machine's whole
//     grok history on 2026-09-05 — 192 turn_completed records — `stop_reason`
//     is `end_turn` 180×, `error` 7×, `cancelled` 5×, and one of the seven
//     errors DOES carry a `usage` object. A reader keyed on usage-absence
//     would have called that refusal a healthy turn.
//
//     That object is not read as *whether* this turn was refused — it is read
//     as HOW MUCH of it had already run when the refusal landed
//     (ranger-base-qcu4c), which is a different question and the one the
//     settle line was answering wrongly. The same census is what licenses
//     reading its absence as "nothing ran": all 186 usage objects on this box
//     are nonzero in every field (min modelCalls 1, min outputTokens 25), so
//     grok writes one for every turn that served anything and none for a turn
//     it refused before serving. Nonzero rather than merely present is what
//     TurnOutcome.Worked() keys on, so `usage:{}` — a shape this box has
//     never written — reads as nothing ran rather than as work to go looking
//     for.
//   - the preceding `retry_state`/`type:"failed"` update is real (7/7 of the
//     errors have one) but redundant: it says the same thing about the same
//     turn, one record earlier, and reading two records to learn one fact
//     just doubles the ways the read can go wrong.
//
// `cancelled` is the third value the two-arm probe never saw, and it is not a
// refusal: all 5 carry a `usage` object and none carries an `agent_result` —
// a turn that ran and was stopped, not an account that would not serve one.
// It reads as an outcome with no message and observed=true, the same as
// `end_turn`: an outcome WAS read, and it was not a refusal.
//
// The refusal message is NOT narrowed to a payment/quota phrase the way
// claudeAllotmentLimit narrows claude's. It does not need to be and must not
// be: claude's synthetic message shares a channel with arbitrary model prose,
// so it has to prove it is a limit; grok's `stop_reason:"error"` is a
// purpose-built field on a purpose-built record, and every string that
// arrives in `agent_result` beside it is the provider's own account of a turn
// that did not settle. Pattern-matching it for "402" would go silent on the day
// the account fails some other way — the exact blindness this seam exists to
// retire.

import (
	"bufio"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// TurnOutcomeGrokSessionStore is the registry key grok's built-in declares.
// It names the STORE and not the file, for the same reason `claude-transcript`
// does: which record inside it carries the fact is this code's business, and
// a runtime whose CLI writes the same store shape declares this and is read.
const TurnOutcomeGrokSessionStore = "grok-session-store"

// FindGrokTurnOutcome finds the outcome of the turn this dispatch prompt
// started, in grok's own session store. It reads only sessions written since
// this dispatch launched, only a `turn_completed` record after the matching
// bead prompt, and only that record's two typed fields — so bead text quoting
// a provider error cannot become a false positive any more than it can in
// claude's reader.
//
// observed distinguishes a first turn that settled normally (no message,
// true) from a store this pass could not read an outcome out of (no message,
// false) — the third state turnOutcomeClause prints, and the only honest
// answer when grok's store says a turn errored but carries no message with
// it. A refusal also carries how much of the turn had already run when it
// landed (TurnOutcome, ranger-base-qcu4c).
func FindGrokTurnOutcome(dir, bead string, since time.Time) (out TurnOutcome, observed bool) {
	// No window is not every window. claude's reader is bounded by the
	// project directory whatever `since` says; this one is bounded by
	// `since` alone once the cwd stops filtering, so a zero floor would let
	// a months-old refusal on this bead answer for today's turn. Dispatch
	// never passes one — pendingBead.launched is stamped before the launch —
	// and if something ever does, "cannot tell" is the right answer.
	if since.IsZero() {
		return TurnOutcome{}, false
	}
	for _, path := range grokUpdateFiles(dir, since) {
		if out, observed := scanGrokTurnOutcome(path, bead, since); observed {
			return out, observed
		}
	}
	return TurnOutcome{}, false
}

// grokUpdateFiles is every updates.jsonl a turn started at `since` could have
// been written to, freshest-relevant first: the sessions grok ran in `dir`
// before the sessions it ran anywhere else.
//
// `dir` ORDERS the candidates; it does not filter them, and that is measured
// rather than cautious. grok keys its store on the CLI's real working
// directory, and a dispatched session's working directory is its WORKTREE
// (planLaunch: `Worktree: true` on both dispatch launch sites, so `dir`
// becomes the session tree's path) while the `dir` this reader is handed is
// the repo the bead lives in. On this box today those are
// ~/.posse/worktrees/<repo>/<session> and ~/src/<repo> — no substring of each
// other — so a cwd-equality filter would have made this reader answer
// "nothing readable" for every worktree dispatch there is, which is a
// declaration that promises a reading nothing performs. The bead id in the
// work prompt plus `since` is what actually identifies the turn; the cwd is
// a good guess about where to look first and no more than that.
//
// A root that will not open is no candidates, which is observed=false —
// "cannot read" and "read a healthy turn" stay different facts (ADR 0018 §3)
// because only the second one is a message-less `observed`.
func grokUpdateFiles(dir string, since time.Time) []string {
	root := filepath.Join(grokHome(), "sessions")
	cwds, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	want := filepath.Clean(dir)
	var own, rest []string
	for _, c := range cwds {
		if !c.IsDir() {
			continue
		}
		sessions, err := os.ReadDir(filepath.Join(root, c.Name()))
		if err != nil {
			continue
		}
		mine := grokCwd(c.Name()) == want
		for _, s := range sessions {
			if !s.IsDir() {
				continue
			}
			p := filepath.Join(root, c.Name(), s.Name(), "updates.jsonl")
			// Same staleness gate FindClaudeTurnOutcome puts on a transcript,
			// and the same second of slack: a session grok has not written to
			// since this dispatch launched cannot hold this dispatch's turn.
			// It is a COST guard — a store with 196 sessions is not worth
			// parsing whole on every settle — and the slack is what keeps it
			// no stricter than the record gate below, whose own floor is
			// `since` truncated to at worst a whole second.
			st, err := os.Stat(p)
			if err != nil || st.ModTime().Before(since.Add(-time.Second)) {
				continue
			}
			if mine {
				own = append(own, p)
			} else {
				rest = append(rest, p)
			}
		}
	}
	sort.Strings(own)
	sort.Strings(rest)
	return append(own, rest...)
}

// grokCwd decodes the working directory grok percent-encoded into a session
// directory NAME — the same decode cost_grok.go's grokSessionDir does from a
// path inside it, and the reason a value that does not decode is returned raw
// is the same: the literal text is a better answer than an empty string.
func grokCwd(enc string) string {
	if dec, err := url.PathUnescape(enc); err == nil {
		return dec
	}
	return enc
}

// scanGrokTurnOutcome reads one session's updates.jsonl for the outcome of
// the turn `bead`'s work prompt started at or after `since`.
//
// The work prompt arrives as a `user_message_chunk` carrying the dispatcher's
// own "Work beads issue <id>" first line — the same record cost_grok.go cuts
// its cost segments on — and the turn's `turn_completed` follows it in FILE
// order. That ordering is the one thing this scan assumes about the store, so
// it was censused rather than assumed: over all 178 of this machine's
// updates.jsonl files that carry either record, every `turn_completed` is
// preceded by a `user_message_chunk` (2026-09-05). The file is NOT written in
// timestamp order — a refused turn's `retry_state` lands ahead of the
// user_message_chunk whose own stamp is earlier — which is why this reads
// position and not the clock.
func scanGrokTurnOutcome(path, bead string, since time.Time) (out TurnOutcome, observed bool) {
	f, err := os.Open(path)
	if err != nil {
		return TurnOutcome{}, false
	}
	defer f.Close()

	inTurn := false
	r := bufio.NewReaderSize(f, 1<<20)
	for {
		line, readErr := r.ReadBytes('\n')
		if len(line) > 0 {
			var d struct {
				Timestamp int64 `json:"timestamp"`
				Params    struct {
					Update struct {
						SessionUpdate string `json:"sessionUpdate"`
						StopReason    string `json:"stop_reason"`
						AgentResult   string `json:"agent_result"`
						Content       struct {
							Text string `json:"text"`
						} `json:"content"`
						// Not the whole object — the two fields that say
						// whether the model was reached before the account
						// went out from under the turn.
						Usage struct {
							OutputTokens int `json:"outputTokens"`
							ModelCalls   int `json:"modelCalls"`
						} `json:"usage"`
					} `json:"update"`
					Meta struct {
						AgentTimestampMs int64 `json:"agentTimestampMs"`
					} `json:"_meta"`
				} `json:"params"`
			}
			if json.Unmarshal(line, &d) == nil {
				up := d.Params.Update
				switch up.SessionUpdate {
				case "user_message_chunk":
					m := workPromptRe.FindStringSubmatch(up.Content.Text)
					at, res := grokUpdateTime(d.Timestamp, d.Params.Meta.AgentTimestampMs)
					// `since` is a time.Now() and carries nanoseconds; the
					// record carries whatever grok wrote it with. Comparing
					// them at the floor's resolution rather than the
					// record's drops a prompt stamped inside the same
					// millisecond as the launch — MEASURED, that is both
					// arms of the edge pin, not a hypothetical.
					inTurn = m != nil && m[1] == bead && !at.Before(since.Truncate(res))
				case "turn_completed":
					if !inTurn {
						break
					}
					if up.StopReason != "error" {
						// end_turn, cancelled, or anything else grok settles a
						// turn with that is not its error path: an outcome was
						// read and it was not a refusal, which is the positive
						// evidence a stale failure marker may be cleared on.
						return TurnOutcome{}, true
					}
					if up.AgentResult == "" {
						// The error path with nothing to quote. Reporting no
						// message here would say "healthy turn" about a turn
						// grok itself called an error, so this is the
						// not-readable rung instead: the settle line tells the
						// operator to peek rather than guessing either way.
						return TurnOutcome{}, false
					}
					return TurnOutcome{
						Message: strings.Join(strings.Fields(up.AgentResult), " "),
						// Zero unless this refusal carried a usage object —
						// which 1 of the 7 on this box did, from a turn six
						// model calls deep when the 402 landed.
						ModelCalls:   up.Usage.ModelCalls,
						OutputTokens: up.Usage.OutputTokens,
					}, true
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return TurnOutcome{}, false
		}
	}
	return TurnOutcome{}, false
}

// grokUpdateTime is when grok recorded an update, and — the half that
// matters — how precisely. The top-level `timestamp` is whole SECONDS, so a
// turn prompted at .900 records as .000; the `_meta.agentTimestampMs` beside
// it carries milliseconds and is present on 389/389 of this machine's
// user_message_chunk and turn_completed records (censused 2026-09-05). Read
// the precise one, fall back to the coarse one, and say which, because a
// stamp is only comparable to a floor at its own resolution: the caller
// truncates `since` to it rather than testing a rounded-down record against
// an unrounded clock, which is a gate that drops the very record it was
// written to keep.
//
// Reading the millisecond stamp is precision and not correctness — the
// seconds fallback is right too, just a whole second looser at the floor —
// and no pin demands the tightness, because nothing in the fleet starts two
// turns on one bead inside one second. It is here so the window means the
// launch instant rather than the launch second, which is the difference that
// would matter the day something does.
func grokUpdateTime(sec, ms int64) (at time.Time, resolution time.Duration) {
	if ms > 0 {
		return time.UnixMilli(ms), time.Millisecond
	}
	return time.Unix(sec, 0), time.Second
}
