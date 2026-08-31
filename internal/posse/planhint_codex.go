package posse

// The codex plan hint (ADR 0034 D1): the newest `rate_limits` reading codex
// itself already wrote to disk, typed as a fact the guard may only DISPLAY
// or use to REFUSE overflow — never a second gate.
//
// Every codex `token_count` event in
// `~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl` carries
// `payload.rate_limits`: per-window `used_percent`, `window_minutes`,
// `resets_at`, plus a `credits` block that says whether an exhausted pool
// starts billing. The same reading `planusage_anthropic.go` fetches for
// Claude — on disk, free, no credential.
//
// The type split IS the policy (ADR 0034 D1). PlanHint deliberately does
// NOT implement PlanReader, is never listed in planAdapters, and is never
// written to `plan-usage.json` — a local file needs no request-rate cache,
// and a second provider in that snapshot would be a third store of a fact
// that already has two. What makes this safe is exactly what makes it
// USELESS as a second gate: the reading is a snapshot outside its store of
// record (Helland), and the staleness is unbounded in the dangerous
// direction — the pool is account-wide, the rollouts are box-local, so
// codex on another device drains the pool without this file moving. A
// local-only argument ("any turn here refreshes it") is true and
// insufficient, so this type never starts a blind clock and never parks a
// pass. Nil is "no reading" and callers that want to gate on it (the
// overflow ladder, ADR 0034 D4) may only ever use it to REFUSE, never to
// license.
//
// Windows are named by DURATION, never by slot (ADR 0034 D2): measured on
// this box, the primary window was the rolling 5h session window
// Jan–Jun 2026 and the weekly (7d) one in Aug — a slot-named threshold
// would silently change meaning when the provider reshuffles which window
// is primary. A duration-named one at worst goes unmatched, which the
// operator is already told about (unmatchedThresholds' codex carve-out).
//
// Scan cost (the ADR's ASSUMED claim): newest-first traversal stops at the
// first file that carries a usable reading, so the common case opens
// exactly one file. Measured live on this box 2026-08-31 (173 rollouts,
// the newest one already carrying a reading): one file opened, well under a
// millisecond. The pathological case — codex on this box has NEVER once
// written a rate_limits reading — walks every rollout on every call, since
// there is no cheaper way to conclude "none of these have it"; that box has
// nothing gating on the answer (a hint that has never existed is not a
// change in urgency), so no memoization is added ahead of a measured need.

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// HintWindow is one rate window from codex's own meter: its duration-based
// name (codexWindowName) and its own reset time — unlike PlanUsage's
// Window, resets_at is per-window because codex reports it per-window.
type HintWindow struct {
	Name        string
	UsedPercent float64
	ResetsAt    time.Time
}

// PlanHintCredits is the reading's credits block: whether an exhausted pool
// keeps running by billing past the limit, or stops (a hard ceiling).
type PlanHintCredits struct {
	HasCredits bool
	Unlimited  bool
	// Balance is the endpoint's own string, kept as text and never parsed
	// into a number: nothing here does arithmetic on it, and the field is
	// absent (null) as often as it is a decimal string on this box's own
	// corpus.
	Balance string
}

// PlanHint is one reading of codex's on-disk plan meter (ADR 0034 D1). At is
// the event's OWN timestamp, never the file's mtime — a rollout keeps
// getting appended to well after its first line, so the file's mtime is
// the SESSION's freshness, not the READING's.
//
// A nil *PlanHint is "no reading anywhere in this machine's codex
// history" — codex was never used here, or every rate_limits event on disk
// is the empty shell codex writes before a session's first turn (primary,
// secondary and credits all null). Both are the same fact to a caller: cap
// only, exactly as if codex had never run.
type PlanHint struct {
	Windows []HintWindow
	At      time.Time
	Credits PlanHintCredits
	// SpendControlReached mirrors the reading's own field: true licenses
	// nothing, and ADR 0034 D4 makes it one of the two hardcoded floors
	// that refuse overflow outright.
	SpendControlReached bool
}

// ReadCodexPlanHint scans `~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl` —
// newest date directory first, newest file first within a day — for the
// newest `token_count` event that carries a usable `rate_limits` reading,
// and returns it as a *PlanHint. Nil when no rollout on this machine ever
// wrote one.
//
// No error return: a hint informs and never gates (ADR 0034 D1), so a
// read failure — a missing home directory, an unreadable file, a rollout
// that predates rate_limits — has exactly one honest answer a caller can
// act on, which is the same one "codex was never used here" gets. Callers
// that need to know WHY there is no reading have no use for the answer
// either way: nothing downstream may treat a codex read failure as
// blindness (that clock belongs to the one meter with gating duty).
func ReadCodexPlanHint() *PlanHint {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	root := filepath.Join(home, ".codex", "sessions")
	for _, f := range codexRolloutsNewestFirst(root) {
		if hint := newestPlanHintIn(f); hint != nil {
			return hint
		}
	}
	return nil
}

// codexRolloutsNewestFirst lists every rollout file under root, ordered
// newest-first: the date directories sort newest-first at each of the
// three levels (year, month, day), and within a day the filenames — which
// embed the session's own ISO-8601 start time — sort newest-first too. A
// missing root is "codex was never used on this machine", not a fault, so
// it returns an empty list rather than an error.
func codexRolloutsNewestFirst(root string) []string {
	years := sortedDirNamesDesc(root)
	var out []string
	for _, y := range years {
		yDir := filepath.Join(root, y)
		for _, m := range sortedDirNamesDesc(yDir) {
			mDir := filepath.Join(yDir, m)
			for _, d := range sortedDirNamesDesc(mDir) {
				dDir := filepath.Join(mDir, d)
				out = append(out, rolloutsInDayDesc(dDir)...)
			}
		}
	}
	return out
}

// sortedDirNamesDesc lists dir's subdirectories, newest (lexically
// greatest) name first — the date components sort this way because they
// are always zero-padded (YYYY, MM, DD).
func sortedDirNamesDesc(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	return names
}

// rolloutsInDayDesc lists one day directory's rollout-*.jsonl files, newest
// first — the filename embeds the session's start time
// (`rollout-<ISO8601>-<uuid>.jsonl`), so a lexical reverse sort is a
// chronological one.
func rolloutsInDayDesc(dayDir string) []string {
	entries, err := os.ReadDir(dayDir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "rollout-") && strings.HasSuffix(e.Name(), ".jsonl") {
			names = append(names, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = filepath.Join(dayDir, n)
	}
	return out
}

// codexRateLimitsWindow is one window exactly as codex writes it.
type codexRateLimitsWindow struct {
	UsedPercent float64 `json:"used_percent"`
	WindowMin   int     `json:"window_minutes"`
	ResetsAt    int64   `json:"resets_at"`
}

// codexRateLimitsBody is the `payload.rate_limits` object. Every field is a
// pointer (or, for the two bools inside Credits, the whole block is) so a
// key that is present-but-null and a key that is absent read the same way —
// which is how this box's own corpus writes the empty shell codex logs
// before a session's first turn: `{"primary":null,"secondary":null,
// "credits":null,"plan_type":null}`.
type codexRateLimitsBody struct {
	Primary   *codexRateLimitsWindow `json:"primary"`
	Secondary *codexRateLimitsWindow `json:"secondary"`
	Credits   *struct {
		HasCredits bool    `json:"has_credits"`
		Unlimited  bool    `json:"unlimited"`
		Balance    *string `json:"balance"`
	} `json:"credits"`
	SpendControlReached *bool `json:"spend_control_reached"`
}

// codexTokenCountLine is the one event line this reader looks at: a
// `token_count` event_msg record. Every other record type in a rollout
// (session_meta, turn_context, response_item, …) decodes into the zero
// value here and is skipped — RateLimits stays nil.
type codexTokenCountLine struct {
	Timestamp string `json:"timestamp"`
	Payload   struct {
		RateLimits *codexRateLimitsBody `json:"rate_limits"`
	} `json:"payload"`
}

// newestPlanHintIn scans one rollout file top to bottom and returns the
// LAST usable rate_limits reading it carries, or nil if it carries none. A
// reading is usable when at least one of its two windows is present — the
// empty shell (both null) is not a reading at all, and is treated exactly
// like a line with no rate_limits key.
//
// Top-to-bottom because a rollout's own events are already time-ordered
// (codex appends), so the last MATCHING line is the newest one — no need to
// buffer the whole file to find it.
func newestPlanHintIn(path string) *PlanHint {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var latest *PlanHint
	r := bufio.NewReaderSize(f, 1<<20)
	for {
		line, rerr := r.ReadBytes('\n')
		if len(line) > 0 {
			if hint := decodeCodexPlanHintLine(line); hint != nil {
				latest = hint
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			break
		}
	}
	return latest
}

// decodeCodexPlanHintLine decodes one rollout line into a *PlanHint, or nil
// if the line is not a usable rate_limits reading (wrong record type,
// malformed JSON, or the empty shell with both windows null).
func decodeCodexPlanHintLine(line []byte) *PlanHint {
	var d codexTokenCountLine
	if json.Unmarshal(line, &d) != nil {
		return nil
	}
	rl := d.Payload.RateLimits
	if rl == nil || (rl.Primary == nil && rl.Secondary == nil) {
		return nil
	}
	at, err := time.Parse(time.RFC3339Nano, d.Timestamp)
	if err != nil {
		return nil
	}
	hint := &PlanHint{At: at}
	for _, w := range []*codexRateLimitsWindow{rl.Primary, rl.Secondary} {
		if w == nil {
			continue
		}
		hint.Windows = append(hint.Windows, HintWindow{
			Name:        codexWindowName(w.WindowMin),
			UsedPercent: w.UsedPercent,
			ResetsAt:    time.Unix(w.ResetsAt, 0),
		})
	}
	if rl.Credits != nil {
		hint.Credits.HasCredits = rl.Credits.HasCredits
		hint.Credits.Unlimited = rl.Credits.Unlimited
		if rl.Credits.Balance != nil {
			hint.Credits.Balance = *rl.Credits.Balance
		}
	}
	if rl.SpendControlReached != nil {
		hint.SpendControlReached = *rl.SpendControlReached
	}
	return hint
}

// codexWindowName names a window by its duration, never by which slot
// codex called it (ADR 0034 D2): `codex_5h` for the 300-minute window,
// `codex_7d` for the 10080-minute one — the two measured on this box — and
// `codex_<N>m` for anything else, so a window this reader has never seen
// still gets an honest name instead of being dropped.
func codexWindowName(minutes int) string {
	switch minutes {
	case 300:
		return "codex_5h"
	case 10080:
		return "codex_7d"
	default:
		if minutes <= 0 {
			return "codex_0m"
		}
		return "codex_" + strconv.Itoa(minutes) + "m"
	}
}
