package posse

// The codex cost adapter (ADR 0012 D4).
//
// Locator: ~/.codex/sessions/YYYY/MM/DD/rollout-<ISO8601>-<uuid>.jsonl, one
// file per session. The session's working directory — what the `project`
// filter matches — is not in the path; it is the `cwd` of the `session_meta`
// record on line 1, which is why the locator opens each file to filter.
//
// Decoder: `event_msg` records whose payload type is "token_count" carry
// `info.total_token_usage`. Segments are cut on the dispatcher's "Work beads
// issue <id>" prompt, which arrives as a `response_item` message with role
// "user", exactly as in a Claude transcript, and the model comes from the
// `turn_context` record's `model`.
//
// Price table: **none, deliberately.** codex runs on the operator's ChatGPT
// subscription: it reports no cost, and no list rate applies to a plan seat.
// PriceFor therefore always says "not priced here", every codex message
// counts as unpriced, and `posse cost` prints its turns and tokens with a
// blank in the $ column plus the legend that says why. An honest blank beats
// a number this repo would have invented — the uncounted-never-$0 rule (ADR
// 0012 D4, ADR 0018 §3) pointed at dollars rather than at sessions.
//
// # The 2× trap, and why the discipline is delta-from-cumulative
//
// The bead that ordered this work offered two readings and said to pick one:
// max the cumulative `total_token_usage`, or sum the per-turn
// `last_token_usage`. **Summing last_token_usage is wrong on this machine,
// and wrong on the CURRENT codex.** codex re-emits a token_count record with
// an identical snapshot a fraction of a second after the first — measured
// 2026-08-28 over the whole local history: 100 of 163 rollouts carry such
// duplicates, and on those files sum(last_token_usage) is ~2.00× the final
// total_token_usage (e.g. 177,412,724 summed against 88,743,731 reported).
// It is not a version to wait out: 15 of the 65 files written by codex
// 0.147.0, the version running here, have them.
//
// Maxing the cumulative snapshot is dedupe-proof but cannot attribute:
// a session that works two beads has one running total, so the second bead
// would eat the first bead's spend.
//
// So this decoder does the third thing: it reads the CUMULATIVE snapshot and
// charges each segment the **delta since the previous snapshot**. A duplicate
// record has a zero delta and contributes nothing; a real turn contributes
// exactly once; and every turn lands on the segment that was open when it
// happened. Verified against all 159 local rollouts that carry token counts:
// the deltas sum to the file's final total_token_usage on 159 of 159, with no
// snapshot ever going backwards.
//
// Two smaller facts the shape depends on, both measured on the same corpus:
// `input_tokens` INCLUDES `cached_input_tokens` (so In is the difference),
// and `reasoning_output_tokens` is a SUBSET of `output_tokens` (so adding it
// would double-count reasoning). total_tokens == input_tokens +
// output_tokens on every record.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type codexCost struct{}

func (codexCost) Runtime() string { return "codex" }

// Reads/Prices: turns, tokens and per-bead attribution, and NEVER a dollar
// — the subscription seat reports no cost and no list rate applies to one
// (PriceFor below). So codex is READ but UNPRICED: its sessions are not in
// CostReport.Uncounted, its beads print a blank rather than 0.00, and it
// keeps ADR 0013 §5's brake, because the thing that brake stands in for —
// no dollar meter on this pool — is exactly still the case.
func (codexCost) Reads() string {
	return "rollout scanner (~/.codex/sessions/**/rollout-*.jsonl, or $CODEX_HOME's, tokens only, ADR 0012 D4)"
}
func (codexCost) Prices() bool { return false }

// PriceFor never prices a codex model. The work ran on a subscription seat,
// not on the API: there is no rate that applies, and inventing one at another
// provider's list rates would put fabricated dollars in the same total as
// real ones with nothing marking which were which.
func (codexCost) PriceFor(string) (Price, bool) { return Price{}, false }

// codexTokens is one usage snapshot as codex writes it. Both
// total_token_usage and last_token_usage have this shape; only the first is
// read (see the 2× trap above).
type codexTokens struct {
	In     int `json:"input_tokens"`
	Cached int `json:"cached_input_tokens"`
	CacheW int `json:"cache_write_input_tokens"`
	Out    int `json:"output_tokens"`
	Reason int `json:"reasoning_output_tokens"`
	Total  int `json:"total_tokens"`
}

// since returns this snapshot's spend since prev. A snapshot that went
// BACKWARDS is a session whose counter restarted, and then the snapshot is
// itself the delta — clamping to zero there would silently drop everything
// after the restart. No local rollout does this (0 of 159 measured), which is
// exactly why it is handled here rather than assumed away.
func (t codexTokens) since(prev codexTokens) codexTokens {
	if t.Total < prev.Total {
		return t
	}
	d := codexTokens{
		In:     t.In - prev.In,
		Cached: t.Cached - prev.Cached,
		CacheW: t.CacheW - prev.CacheW,
		Out:    t.Out - prev.Out,
		Reason: t.Reason - prev.Reason,
		Total:  t.Total - prev.Total,
	}
	for _, f := range []*int{&d.In, &d.Cached, &d.CacheW, &d.Out, &d.Reason, &d.Total} {
		if *f < 0 {
			*f = 0
		}
	}
	return d
}

// Transcripts lists <codex home>/sessions/**/rollout-*.jsonl, filtered by
// project against each session's recorded working directory.
//
// The home is codexHomeIn's, so $CODEX_HOME moves this walk exactly as it
// moves the CLI's own store and posse's interstitial probe of it. Rooting
// it at ~/.codex regardless — which this did until ranger-base-z65xu — put
// the walk on an absent root under an override, and an absent root is
// "never ran codex" below: no turns, no error, no uncounted line.
//
// A missing root IS no records — a machine that has never run codex has
// nothing to count — while anything else (a permission, a broken mount, a
// directory replaced by a file) is a read failure and says so, because "no
// spend" and "cannot tell" are different facts (ADR 0018 §3). It walks with
// WalkDir and keeps walking past a failure for the same reason the Claude
// locator does: one unreadable day directory hides an unknown pile of spend,
// and the days that did read are still the best floor available.
func (codexCost) Transcripts(project string) ([]string, []error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, []error{err}
	}
	root := filepath.Join(codexHomeIn(home), "sessions")
	var out []string
	var errs []error
	filepath.WalkDir(root, func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			if !os.IsNotExist(err) { // never ran codex, or removed mid-walk
				errs = append(errs, err)
			}
			return nil
		}
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			return nil
		}
		if project != "" {
			cwd, cerr := codexSessionCwd(p)
			if cerr != nil {
				// Its spend cannot be attributed to a project either way, so
				// it is unknown rather than excluded quietly.
				errs = append(errs, cerr)
				return nil
			}
			if !strings.Contains(cwd, project) {
				return nil
			}
		}
		out = append(out, p)
		return nil
	})
	sort.Strings(out)
	return out, errs
}

// codexSessionCwd reads the working directory a session ran in from its
// `session_meta` record, which codex writes as line 1 (163 of 163 local
// rollouts). A file whose first line is not that record cannot be placed in a
// project, and says so rather than defaulting into or out of the filter.
func codexSessionCwd(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	line, err := bufio.NewReaderSize(f, 1<<20).ReadBytes('\n')
	if err != nil && len(line) == 0 {
		return "", fmt.Errorf("%s: no session_meta record: %w", path, err)
	}
	var d struct {
		Type    string `json:"type"`
		Payload struct {
			Cwd string `json:"cwd"`
		} `json:"payload"`
	}
	if json.Unmarshal(line, &d) != nil || d.Type != "session_meta" || d.Payload.Cwd == "" {
		return "", fmt.Errorf("%s: first record names no working directory", path)
	}
	return d.Payload.Cwd, nil
}

// Decode segments one codex rollout by the dispatcher's work prompts and
// charges each turn's token delta to the segment that was open for it.
func (codexCost) Decode(path string, since time.Time) ([]*Segment, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var segs []*Segment
	var cur *Segment
	var prev codexTokens
	model := ""
	r := bufio.NewReaderSize(f, 1<<20)
	for {
		line, rerr := r.ReadBytes('\n')
		if len(line) > 0 {
			var d struct {
				Timestamp string `json:"timestamp"`
				Type      string `json:"type"`
				Payload   struct {
					Type    string `json:"type"`
					Role    string `json:"role"`
					Model   string `json:"model"`
					Content []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"content"`
					Info *struct {
						Total *codexTokens `json:"total_token_usage"`
					} `json:"info"`
				} `json:"payload"`
			}
			if json.Unmarshal(line, &d) == nil {
				ts, _ := time.Parse(time.RFC3339Nano, d.Timestamp)
				p := d.Payload
				switch {
				case d.Type == "turn_context" && p.Model != "":
					model = p.Model
					if cur != nil && cur.Model == "" {
						cur.Model = model
					}
				case p.Type == "message" && p.Role == "user":
					txt := codexUserText(p.Content)
					if m := workPromptRe.FindStringSubmatch(txt); m != nil {
						cur = newCodexSegment(m[1], path, ts, model)
						segs = append(segs, cur)
					} else if cur == nil && txt != "" && !strings.HasPrefix(txt, "<") {
						cur = newCodexSegment("interactive", path, ts, model)
						segs = append(segs, cur)
					}
				case p.Type == "token_count":
					if p.Info == nil || p.Info.Total == nil {
						break
					}
					// The baseline advances on EVERY snapshot, including the
					// ones this segment will not be charged for: it is the
					// session's running total, not this segment's, and a
					// baseline that skipped a record would charge the next
					// segment for the turn that record reported.
					delta := p.Info.Total.since(prev)
					prev = *p.Info.Total
					if cur == nil || delta.Total == 0 || (!since.IsZero() && ts.Before(since)) {
						break
					}
					// One turn per non-empty delta; a duplicate snapshot has
					// an empty one and never reaches here, so Turns() counts
					// turns rather than records.
					key := "turn-" + strconv.Itoa(len(cur.Msgs))
					cur.Msgs[key] = &Usage{
						Model:  cur.Model,
						In:     delta.In - delta.Cached, // input_tokens INCLUDES cached reads
						CacheR: delta.Cached,
						CacheW: delta.CacheW,
						Out:    delta.Out, // reasoning is a subset of output
					}
					if u := cur.Msgs[key]; u.In < 0 {
						u.In = 0
					}
					if ts.After(cur.End) {
						cur.End = ts
					}
				}
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return segs, rerr
		}
	}

	var out []*Segment
	for _, s := range segs {
		if len(s.Msgs) > 0 {
			s.Total()
			out = append(out, s)
		}
	}
	return out, nil
}

// codexUserText extracts the text of a codex user message: the first
// input_text part of its content list.
func codexUserText(parts []struct {
	Type string `json:"type"`
	Text string `json:"text"`
}) string {
	for _, p := range parts {
		if p.Type == "input_text" || p.Type == "text" {
			return p.Text
		}
	}
	return ""
}

func newCodexSegment(bead, path string, ts time.Time, model string) *Segment {
	if model == "" {
		model = "codex"
	}
	return &Segment{Bead: bead, Runtime: "codex", File: path, Start: ts, End: ts, Msgs: map[string]*Usage{}, Model: model}
}

func init() { RegisterCostProvider(codexCost{}) }
