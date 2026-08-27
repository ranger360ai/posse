package rhq

// The grok cost adapter (ADR 0012 D4).
//
// Locator: ~/.grok/sessions/<url-encoded cwd>/<session-uuid>/updates.jsonl.
// The first path element is the session's working directory, percent-encoded,
// which is what the `project` filter matches against once decoded.
//
// Decoder: one record kind carries money — sessionUpdate "turn_completed",
// whose `usage.costUsdTicks` is that turn's cost in **nano-dollars**
// (1 tick = 1e-9 USD). Segments are cut on the dispatcher's "Work beads issue
// <id>" prompt, which arrives as a `user_message_chunk`, exactly as in a
// Claude transcript.
//
// Price table: **none, deliberately.** grok reports its own cost per turn, and
// a number the provider computed beats any rate card this repo could keep
// current. PriceFor therefore always says "not priced here" — the segments it
// produces are ProviderPriced and never reach a table.
//
// # The 2× trap
//
// `usage` also carries a `modelUsage` map — the same spend broken down per
// model — and each breakdown sums to exactly the aggregate beside it. A
// decoder that walks every `costUsdTicks` it can find therefore reports
// **exactly twice** the real spend. Measured over this machine's whole grok
// history (2026-08-27): 171 turn_completed records across 156 sessions,
// aggregate 219,264,190,800 ticks, breakdown 219,264,190,800 ticks,
// 171/171 records equal and 0 mismatched — a ratio of 2.0000. Read the
// aggregate and never descend into modelUsage.
//
// Note on the method this replaces: the bead that ordered this work described
// grok's records as *cumulative snapshots* needing a max-per-session. On this
// grok they are not — there is exactly one turn_completed per prompt_id
// (171 records / 171 prompts), and the values are not monotonic within a
// session, so a max would silently drop every turn but the priciest. Per-turn
// totals summed is the correct reading, and the 2× the bead attributed to
// summing is the modelUsage duplication above. Segment.NoteCumulative still
// exists for a provider that genuinely does restate a running total.

import (
	"bufio"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// grokTickUSD converts grok's costUsdTicks to dollars. Nano-dollars: the
// scale is confirmed by regressing ticks against tokens over the whole local
// history — it puts cached-read at 0.2517× input, the standard 0.25× cache
// discount, which no other scale produces.
const grokTickUSD = 1e-9

type grokCost struct{}

func (grokCost) Runtime() string { return "grok" }

// PriceFor never prices a grok model: grok reports dollars itself, so there
// is no rate card to consult and none to let go stale.
func (grokCost) PriceFor(string) (Price, bool) { return Price{}, false }

// Transcripts lists ~/.grok/sessions/*/*/updates.jsonl, filtered by project
// against each session's decoded working directory.
//
// As with the Claude locator, a missing root IS no records — a machine that
// has never run grok has nothing to count — while anything else (a
// permission, a broken mount, a directory replaced by a file) is a read
// failure and says so, because "no spend" and "cannot tell" are different
// facts (ADR 0018 §3).
func (grokCost) Transcripts(project string) ([]string, []error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, []error{err}
	}
	root := filepath.Join(home, ".grok", "sessions")
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []error{err}
	}
	files, _ := filepath.Glob(filepath.Join(root, "*", "*", "updates.jsonl"))
	var out []string
	for _, f := range files {
		if project != "" && !strings.Contains(grokSessionDir(f), project) {
			continue
		}
		out = append(out, f)
	}
	sort.Strings(out)
	return out, nil
}

// grokSessionDir decodes the working directory a session ran in from its
// path. The encoded element is two levels up from updates.jsonl. A value that
// does not decode is returned raw — a filter is better served by the literal
// text than by an empty string that matches nothing.
func grokSessionDir(path string) string {
	enc := filepath.Base(filepath.Dir(filepath.Dir(path)))
	if dec, err := url.PathUnescape(enc); err == nil {
		return dec
	}
	return enc
}

// Decode segments one grok session by the dispatcher's work prompts and sums
// each turn's provider-reported cost.
func (grokCost) Decode(path string, since time.Time) ([]*Segment, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var segs []*Segment
	var cur *Segment
	r := bufio.NewReaderSize(f, 1<<20)
	for {
		line, rerr := r.ReadBytes('\n')
		if len(line) > 0 {
			var d struct {
				Timestamp int64 `json:"timestamp"`
				Params    struct {
					Update struct {
						SessionUpdate string `json:"sessionUpdate"`
						PromptID      string `json:"prompt_id"`
						Content       struct {
							Text string `json:"text"`
						} `json:"content"`
						Meta struct {
							ModelID string `json:"modelId"`
						} `json:"_meta"`
						Usage *struct {
							In        int   `json:"inputTokens"`
							Out       int   `json:"outputTokens"`
							Reasoning int   `json:"reasoningTokens"`
							CacheR    int   `json:"cachedReadTokens"`
							CacheW    int   `json:"cacheCreationTokens"`
							Ticks     int64 `json:"costUsdTicks"`
							// modelUsage is deliberately not decoded: it
							// restates this same spend per model, and adding
							// it doubles the total exactly. See the 2× trap.
						} `json:"usage"`
					} `json:"update"`
				} `json:"params"`
			}
			if json.Unmarshal(line, &d) == nil {
				ts := time.Unix(d.Timestamp, 0)
				up := d.Params.Update
				switch up.SessionUpdate {
				case "user_message_chunk":
					txt := up.Content.Text
					if m := workPromptRe.FindStringSubmatch(txt); m != nil {
						cur = newGrokSegment(m[1], path, ts, up.Meta.ModelID)
						segs = append(segs, cur)
					} else if cur == nil && txt != "" && !strings.HasPrefix(txt, "<") {
						cur = newGrokSegment("interactive", path, ts, up.Meta.ModelID)
						segs = append(segs, cur)
					}
				case "turn_completed":
					if cur == nil || up.Usage == nil || (!since.IsZero() && ts.Before(since)) {
						break
					}
					us := up.Usage
					// One usage record per prompt, so the prompt id is the
					// turn key. Falling back to the record count keeps a
					// session with no prompt ids from collapsing to one turn.
					key := up.PromptID
					if key == "" {
						key = "turn-" + strconv.Itoa(len(cur.Msgs))
					}
					if cur.Msgs[key] == nil {
						cur.Msgs[key] = &Usage{Model: cur.Model}
					}
					u := cur.Msgs[key]
					u.In += us.In - us.CacheR // grok's inputTokens INCLUDES cached reads
					u.CacheR += us.CacheR
					u.CacheW += us.CacheW
					u.Out += us.Out + us.Reasoning
					// The money: the aggregate only, summed across turns.
					cur.NotePricedTurn(float64(us.Ticks) * grokTickUSD)
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
		if s.ProviderPriced {
			s.Total()
			out = append(out, s)
		}
	}
	return out, nil
}

func newGrokSegment(bead, path string, ts time.Time, model string) *Segment {
	if model == "" {
		model = "grok"
	}
	return &Segment{Bead: bead, File: path, Start: ts, End: ts, Msgs: map[string]*Usage{}, Model: model}
}

func init() { RegisterCostProvider(grokCost{}) }
