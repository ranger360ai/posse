package rhq

// posse cost — API-equivalent dollars per bead, from Claude Code transcripts
// (ADR 0003 §4 accounting; the business manager's bead-cost.py is the
// reference method).
//
// Method: each ~/.claude/projects/*/*.jsonl is segmented by the "Work beads
// issue <id>" prompts the dispatcher sends; assistant records in a segment
// are deduped by message id (streamed chunks repeat the same id — take the
// max per usage field), and priced at list rates for the model the record
// names. Divergences from the script, both stated: cache writes are priced
// by TTL when the breakdown is present (5m = 1.25× input, 1h = 2× input;
// the script used 1.25× flat), and each message is priced at its own
// model rather than everything at Fable. codex/grok sessions leave no
// transcript here — they are reported as uncounted, never as $0.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Price per MTok. Cache write = 1.25× (5m) / 2× (1h) input; cache read =
// 0.1× input — the standard multipliers.
type Price struct{ In, Out float64 }

// PriceTable: list rates (USD per MTok) by model id, then by family for ids
// we have not seen. Sonnet 5 at list ($3/$15; the intro $2/$10 through
// 2026-08-31 is ignored — this is API-equivalent, not the invoice).
var PriceTable = map[string]Price{
	"claude-fable-5":   {10, 50},
	"claude-mythos-5":  {10, 50},
	"claude-opus-5":    {5, 25},
	"claude-opus-4-8":  {5, 25},
	"claude-opus-4-7":  {5, 25},
	"claude-sonnet-5":  {3, 15},
	"claude-haiku-4-5": {1, 5},
}

// PriceFor resolves a model id to list rates; unknown ids fall back by
// family, then to Fable (the expensive assumption — visible in the output).
func PriceFor(model string) (Price, bool) {
	if p, ok := PriceTable[model]; ok {
		return p, true
	}
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "fable"), strings.Contains(m, "mythos"):
		return Price{10, 50}, true
	case strings.Contains(m, "opus"):
		return Price{5, 25}, true
	case strings.Contains(m, "sonnet"):
		return Price{3, 15}, true
	case strings.Contains(m, "haiku"):
		return Price{1, 5}, true
	}
	return Price{10, 50}, false
}

// TierForModel maps a claude model id back to the ADR 0003 tier name.
func TierForModel(model string) string {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "fable"), strings.Contains(m, "mythos"):
		return TierStrong
	case strings.Contains(m, "opus"):
		return TierStandard
	case strings.Contains(m, "sonnet"), strings.Contains(m, "haiku"):
		return TierFast
	}
	return "?"
}

// Usage is one message's token counts (max over its streamed chunks).
type Usage struct {
	In, CacheW5m, CacheW1h, CacheW, CacheR, Out int
	Model                                       string
}

// Cost prices one message.
func (u Usage) Cost() float64 {
	p, _ := PriceFor(u.Model)
	w5, w1 := u.CacheW5m, u.CacheW1h
	if w5+w1 == 0 { // no TTL breakdown: the script's flat 1.25×
		w5 = u.CacheW
	}
	return (float64(u.In)*p.In + float64(w5)*p.In*1.25 + float64(w1)*p.In*2 + float64(u.CacheR)*p.In*0.1 + float64(u.Out)*p.Out) / 1e6
}

// Segment is the work on one bead (or an interactive stretch) in one
// transcript.
type Segment struct {
	Bead    string // bead id, or "interactive"
	File    string
	Start   time.Time
	End     time.Time
	Msgs    map[string]*Usage
	Model   string  // dominant model (by cost)
	Persona string  // filled by CostReport from bead assignees, if known
	CostUSD float64 // filled by Total()
}

func (s *Segment) Total() (u Usage, cost float64) {
	byModel := map[string]float64{}
	for _, m := range s.Msgs {
		u.In += m.In
		u.CacheW += m.CacheW
		u.CacheW5m += m.CacheW5m
		u.CacheW1h += m.CacheW1h
		u.CacheR += m.CacheR
		u.Out += m.Out
		c := m.Cost()
		cost += c
		byModel[m.Model] += c
	}
	best := 0.0
	for m, c := range byModel {
		if c > best {
			best, s.Model = c, m
		}
	}
	s.CostUSD = cost
	return u, cost
}

// Sum totals the tokens without repricing (CostUSD/Model come from Total).
func (s *Segment) Sum() (u Usage) {
	for _, m := range s.Msgs {
		u.In += m.In
		u.CacheW += m.CacheW
		u.CacheW5m += m.CacheW5m
		u.CacheW1h += m.CacheW1h
		u.CacheR += m.CacheR
		u.Out += m.Out
	}
	return u
}

func (s *Segment) Turns() int { return len(s.Msgs) }

var workPromptRe = regexp.MustCompile(`^Work beads issue (\S+?)[: (]`)

// ScanTranscript segments one Claude Code transcript.
func ScanTranscript(path string, since time.Time) ([]*Segment, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var segs []*Segment
	var cur *Segment
	r := bufio.NewReaderSize(f, 1<<20)
	for {
		line, err := r.ReadBytes('\n')
		if len(line) > 0 {
			var d struct {
				Type      string `json:"type"`
				Timestamp string `json:"timestamp"`
				Message   struct {
					ID      string          `json:"id"`
					Model   string          `json:"model"`
					Content json.RawMessage `json:"content"`
					Usage   *struct {
						In     int `json:"input_tokens"`
						CacheW int `json:"cache_creation_input_tokens"`
						CacheR int `json:"cache_read_input_tokens"`
						Out    int `json:"output_tokens"`
						CC     *struct {
							M5 int `json:"ephemeral_5m_input_tokens"`
							H1 int `json:"ephemeral_1h_input_tokens"`
						} `json:"cache_creation"`
					} `json:"usage"`
				} `json:"message"`
			}
			if json.Unmarshal(line, &d) == nil {
				ts, _ := time.Parse(time.RFC3339Nano, d.Timestamp)
				switch d.Type {
				case "user":
					txt := userText(d.Message.Content)
					if m := workPromptRe.FindStringSubmatch(txt); m != nil {
						cur = &Segment{Bead: m[1], File: path, Start: ts, End: ts, Msgs: map[string]*Usage{}}
						segs = append(segs, cur)
					} else if cur == nil && txt != "" && !strings.HasPrefix(txt, "<") {
						cur = &Segment{Bead: "interactive", File: path, Start: ts, End: ts, Msgs: map[string]*Usage{}}
						segs = append(segs, cur)
					}
				case "assistant":
					if cur == nil || d.Message.Usage == nil || (!since.IsZero() && ts.Before(since)) {
						break
					}
					u := cur.Msgs[d.Message.ID]
					if u == nil {
						u = &Usage{Model: d.Message.Model}
						cur.Msgs[d.Message.ID] = u
					}
					if u.Model == "" {
						u.Model = d.Message.Model
					}
					us := d.Message.Usage
					u.In = max(u.In, us.In)
					u.CacheW = max(u.CacheW, us.CacheW)
					u.CacheR = max(u.CacheR, us.CacheR)
					u.Out = max(u.Out, us.Out)
					if us.CC != nil {
						u.CacheW5m = max(u.CacheW5m, us.CC.M5)
						u.CacheW1h = max(u.CacheW1h, us.CC.H1)
					}
					if ts.After(cur.End) {
						cur.End = ts
					}
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return segs, err
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

// userText extracts the text of a user record: a plain string, or the first
// text part of a content list.
func userText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) == nil {
		for _, p := range parts {
			if p.Type == "text" {
				return p.Text
			}
		}
	}
	return ""
}

// TranscriptFiles lists Claude Code transcripts under ~/.claude/projects,
// optionally filtered by a project-path substring.
func TranscriptFiles(project string) []string {
	home, _ := os.UserHomeDir()
	files, _ := filepath.Glob(filepath.Join(home, ".claude", "projects", "*", "*.jsonl"))
	var out []string
	for _, f := range files {
		if project == "" || strings.Contains(f, project) {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}

// CostReport is everything posse cost prints and the cockpit reads.
type CostReport struct {
	Beads       []*Segment
	Interactive Usage
	InterCost   float64
	InterTurns  int
	Uncounted   int // persona sessions on runtimes with no transcript here (codex/grok)
	Since       time.Time

	// Dial E's caps (ADR 0003 §4), for the report footer only — filled by
	// the caller from config; 0 = unset = no cap. Nothing here enforces
	// them: `posse cost` reads, dispatch decides.
	PassCap, DayCap float64
}

// ScanCosts scans all transcripts (or those matching project) since a time.
func ScanCosts(project string, since time.Time) *CostReport {
	rep := &CostReport{Since: since}
	for _, f := range TranscriptFiles(project) {
		// A file untouched since `since` holds no record after it, and
		// ScanTranscript would drop every segment it built from one. Skipping
		// it on mtime alone is what makes dispatch's per-launch budget check
		// (ADR 0003 Dial E) affordable as the transcript pile grows.
		if !since.IsZero() {
			if st, err := os.Stat(f); err == nil && st.ModTime().Before(since) {
				continue
			}
		}
		segs, _ := ScanTranscript(f, since)
		for _, s := range segs {
			if s.Bead == "interactive" {
				u, c := s.Sum(), s.CostUSD
				rep.Interactive.In += u.In
				rep.Interactive.CacheW += u.CacheW
				rep.Interactive.CacheR += u.CacheR
				rep.Interactive.Out += u.Out
				rep.InterCost += c
				rep.InterTurns += s.Turns()
				continue
			}
			rep.Beads = append(rep.Beads, s)
		}
	}
	sort.Slice(rep.Beads, func(i, j int) bool { return rep.Beads[i].Start.Before(rep.Beads[j].Start) })
	return rep
}

// AttributePersonas fills Segment.Persona from bead assignees across the
// configured repos (bd list --all). Best effort; unknown stays "".
func (r *CostReport) AttributePersonas(a *App, bd Bd) {
	owner := map[string]string{}
	for _, dir := range a.BeadsDirs() {
		issues, err := bd.ListAll(dir)
		if err != nil {
			continue
		}
		for _, is := range issues {
			if is.Assignee != "" {
				owner[is.ID] = is.Assignee
			}
		}
	}
	for _, s := range r.Beads {
		s.Persona = owner[s.Bead]
	}
}

// CountUncounted counts live persona sessions whose runtime writes no
// transcript this scanner reads (anything but claude).
func (r *CostReport) CountUncounted(hb *HerdrBackend) {
	ss, err := hb.Sessions()
	if err != nil {
		return
	}
	for _, s := range ss {
		if s.Agent != "" && s.Runtime != "" && s.Runtime != "claude" {
			r.Uncounted++
		}
	}
}

// ByBead returns cost per bead id (summed over its segments).
func (r *CostReport) ByBead() map[string]float64 {
	out := map[string]float64{}
	for _, s := range r.Beads {
		out[s.Bead] += s.CostUSD
	}
	return out
}

// TodayTotal sums beads (and interactive when includeInteractive) that
// started on the local calendar day of now.
func (r *CostReport) DayTotal(day time.Time) float64 {
	y, m, d := day.Date()
	total := 0.0
	for _, s := range r.Beads {
		yy, mm, dd := s.Start.Local().Date()
		if yy == y && mm == m && dd == d {
			total += s.CostUSD
		}
	}
	return total
}

// PassTotal sums the beads whose work began at or after a dispatch pass
// started — Dial E's `pass` window (ADR 0003 §4). A zero passStart means
// there is no pass in flight (a one-off launch), and the window is empty.
func (r *CostReport) PassTotal(passStart time.Time) float64 {
	if passStart.IsZero() {
		return 0
	}
	total := 0.0
	for _, s := range r.Beads {
		if !s.Start.Before(passStart) {
			total += s.CostUSD
		}
	}
	return total
}

type costGroup struct {
	Key   string
	N     int
	Cost  float64
	costs []float64
}

func groupBy(segs []*Segment, key func(*Segment) string) []costGroup {
	m := map[string]*costGroup{}
	for _, s := range segs {
		k := key(s)
		g := m[k]
		if g == nil {
			g = &costGroup{Key: k}
			m[k] = g
		}
		g.N++
		g.Cost += s.CostUSD
		g.costs = append(g.costs, s.CostUSD)
	}
	var out []costGroup
	for _, g := range m {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func median(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	c := append([]float64(nil), v...)
	sort.Float64s(c)
	return c[len(c)/2]
}

// Print renders the report: per bead, then by tier / persona / day, then
// interactive and the uncounted note.
func (r *CostReport) Print(w io.Writer) {
	fmt.Fprintf(w, "%-16s %-16s %-8s %-8s %5s %8s %9s %10s %7s\n", "bead", "start", "persona", "tier", "turns", "out", "cache_w", "cache_r", "api$")
	var all []float64
	for _, s := range r.Beads {
		u, c := s.Sum(), s.CostUSD
		all = append(all, c)
		p := s.Persona
		if p == "" {
			p = "?"
		}
		fmt.Fprintf(w, "%-16s %-16s %-8s %-8s %5d %8d %9d %10d %7.2f\n", s.Bead, s.Start.Local().Format("2006-01-02 15:04"), p, TierForModel(s.Model), s.Turns(), u.Out, u.CacheW, u.CacheR, c)
	}
	if len(all) > 0 {
		sum := 0.0
		for _, c := range all {
			sum += c
		}
		sorted := append([]float64(nil), all...)
		sort.Float64s(sorted)
		fmt.Fprintf(w, "\n%d bead segments: median $%.2f  mean $%.2f  p90 $%.2f  max $%.2f  sum $%.2f\n",
			len(all), median(all), sum/float64(len(all)), sorted[int(float64(len(sorted))*0.9)], sorted[len(sorted)-1], sum)
	}
	section := func(title string, gs []costGroup) {
		if len(gs) == 0 {
			return
		}
		fmt.Fprintf(w, "\nby %s:\n", title)
		for _, g := range gs {
			fmt.Fprintf(w, "  %-14s %3d bead(s)  sum $%7.2f  median $%.2f  per-bead $%.2f\n", g.Key, g.N, g.Cost, median(g.costs), g.Cost/float64(g.N))
		}
	}
	section("tier (from the model that did the work)", groupBy(r.Beads, func(s *Segment) string { return TierForModel(s.Model) }))
	section("persona (bead assignee)", groupBy(r.Beads, func(s *Segment) string {
		if s.Persona == "" {
			return "?"
		}
		return s.Persona
	}))
	section("day", groupBy(r.Beads, func(s *Segment) string { return s.Start.Local().Format("2006-01-02") }))
	fmt.Fprintf(w, "\ninteractive: %d turns, api-equiv $%.2f (not gated — shown so the ratio is visible)\n", r.InterTurns, r.InterCost)
	if r.Uncounted > 0 {
		fmt.Fprintf(w, "uncounted: %d live persona session(s) on codex/grok — no transcript this scanner reads; their spend is not in these numbers\n", r.Uncounted)
	} else {
		fmt.Fprintln(w, "codex/grok: uncounted (no live sessions now; when there are, they will not appear here)")
	}
	fmt.Fprintln(w, "per pass: measured live by `posse dispatch` from the moment a pass starts (Dial E's pass window); transcripts carry no pass id, so it cannot be reconstructed after the fact")
	if r.PassCap > 0 || r.DayCap > 0 {
		st := BudgetState{PassCap: r.PassCap, DayCap: r.DayCap, DaySpend: r.DayTotal(time.Now())}
		st.resolve()
		caps := []string{}
		if r.PassCap > 0 {
			caps = append(caps, fmt.Sprintf("budget_pass $%.2f", r.PassCap))
		}
		if r.DayCap > 0 {
			caps = append(caps, fmt.Sprintf("budget_day $%.2f (spent $%.2f today)", r.DayCap, st.DaySpend))
		}
		fmt.Fprintf(w, "budget (ADR 0003 Dial E): %s — at %.0f%% of a window standard steps down to fast, at %.0f%% dispatch stops\n",
			strings.Join(caps, ", "), BudgetStepDownPct, BudgetStopPct)
	} else {
		fmt.Fprintln(w, "budget (ADR 0003 Dial E): no caps set (budget_pass:/budget_day: unset) — dormant, nothing steps down and nothing stops")
	}
}
