package posse

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
// model rather than everything at Fable.
//
// That method is now the *Claude adapter's* (cost_claude.go), reached through
// the provider seam in costseam.go: price table + transcript locator + record
// decoder is the whole provider surface, and everything below []*Segment in
// this file is arithmetic that never learns a provider's name (ADR 0012 D4).
// A runtime with no adapter is reported as uncounted, never as $0. Three
// ship: claude (cost_claude.go), grok (cost_grok.go) and codex
// (cost_codex.go). The last two run on subscription seats and have no rate
// card — grok reports its own dollars per turn, codex reports none at all and
// its beads print turns and tokens with a BLANK in the $ column (Segment.Priced).

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
	"sync"
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

// PriceFor resolves a model id to list rates, falling back by family for ids
// we have not seen exactly.
//
// An id that matches no family is **unpriced, not guessed**: it returns the
// zero Price and false, and the caller must count it as uncounted rather than
// as money. The old behaviour assumed Fable — the most expensive tier — on the
// theory that an over-estimate is visible. It is not: it lands in the same
// total as real money, with nothing in the arithmetic marking which dollars
// were invented, so a mis-detected model silently inflates a budget window and
// can stop dispatch. Uncounted-never-guessed is the same rule as
// uncounted-never-zero (ADR 0012 D4, ADR 0018 §3) pointed at the other
// direction: say what is not known instead of picking a number for it.
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
	return Price{}, false
}

// grokTierByModel and codexTierByModel resolve a model id to its tier only
// for ids that ONE tier in that runtime's built-in map names — never by
// picking whichever tier a map iteration happens to hit. codex and grok
// both name the same id on strong and standard (gpt-5.6-sol, grok-4.6), so
// those ids stay ambiguous; only each runtime's fast id (gpt-5.6-luna,
// grok-4.5) is named nowhere else and resolves (ranger-base-3st5).
var (
	grokTierByModel  = uniqueModelTiers(grokModels)
	codexTierByModel = uniqueModelTiers(codexModels)
)

// uniqueModelTiers inverts a runtime's tier->model map, keeping only the
// models named by exactly one tier.
func uniqueModelTiers(models map[string]string) map[string]string {
	count := map[string]int{}
	for _, id := range models {
		count[id]++
	}
	byModel := map[string]string{}
	for tier, id := range models {
		if count[id] == 1 {
			byModel[id] = tier
		}
	}
	return byModel
}

// TierForModel maps a model id back to the ADR 0003 tier name — the model
// that actually ran, read from the transcript, never the PID or session
// meta (rangerhq-oay). "?" is the honest answer whenever the id does not
// name exactly one tier, whether because the runtime is unrecognized or
// because its built-in map names that id on more than one tier.
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
	if tier, ok := grokTierByModel[m]; ok {
		return tier
	}
	if tier, ok := codexTierByModel[m]; ok {
		return tier
	}
	return "?"
}

// Usage is one message's token counts (max over its streamed chunks).
type Usage struct {
	In, CacheW5m, CacheW1h, CacheW, CacheR, Out int
	Model                                       string
}

// Tokens is every token this message reported. Zero means it consumed no
// API at all — a locally-generated record such as Claude Code's `<synthetic>`
// notices — and such a message costs nothing whatever its model id says.
func (u Usage) Tokens() int {
	return u.In + u.CacheW + u.CacheW5m + u.CacheW1h + u.CacheR + u.Out
}

// Priced reports whether this message's model has a rate at all. False means
// its spend is unknown — which Cost renders as 0 and the report must carry as
// uncounted, never as "it was free".
func (u Usage) Priced() bool {
	_, ok := PriceFor(u.Model)
	return ok
}

// Cost prices one message against the global claude table. An unpriced model
// costs 0 here; Priced is how the caller tells that 0 apart from a message
// that genuinely cost nothing. Segment.Total prices against the message's own
// runtime adapter instead (costAt) — this method stays the claude reading.
func (u Usage) Cost() float64 {
	p, ok := PriceFor(u.Model)
	if !ok {
		return 0
	}
	return u.costAt(p)
}

// costAt prices one message at an already-resolved rate.
func (u Usage) costAt(p Price) float64 {
	w5, w1 := u.CacheW5m, u.CacheW1h
	if w5+w1 == 0 { // no TTL breakdown: the script's flat 1.25×
		w5 = u.CacheW
	}
	return (float64(u.In)*p.In + float64(w5)*p.In*1.25 + float64(w1)*p.In*2 + float64(u.CacheR)*p.In*0.1 + float64(u.Out)*p.Out) / 1e6
}

// Segment is the work on one bead (or an interactive stretch) in one
// transcript.
type Segment struct {
	Bead string // bead id, or "interactive"
	// Runtime is the adapter that counted this segment, filled by the scan
	// from CostProvider.Runtime() so no decoder can forget it. It is what
	// makes a mixed day readable: two beads with the same tier and persona
	// can have been paid for out of two different pools.
	Runtime string
	File    string
	Start   time.Time
	End     time.Time
	Msgs    map[string]*Usage
	Model   string  // dominant model (by cost)
	Persona string  // filled by CostReport from bead assignees, if known
	CostUSD float64 // filled by Total()

	// ProviderPriced marks a segment whose provider reported money directly
	// instead of tokens to be priced from a table. ProviderUSD is that money.
	// No price table is consulted for such a segment — the provider's own
	// number is better than any rate card we could keep current.
	//
	// Two ways a provider reports it, and picking the wrong one is a silent
	// multiple of the truth, so the seam offers exactly one call for each:
	//
	//	NotePricedTurn — a per-turn total. Sum them.
	//	NoteCumulative — a running total restated every record. Take the max,
	//	                 never sum (ADR 0012 D4): summing re-counts every
	//	                 earlier record, and the overcount grows with the
	//	                 session, so it is worst where nobody checks by hand.
	//
	// A decoder never does this arithmetic itself; it calls one of the two and
	// cannot get the rule wrong on its own.
	ProviderPriced bool
	ProviderUSD    float64

	// Unpriced counts messages whose model matched no rate (PriceFor said
	// false). Their spend is missing from CostUSD, which therefore is a floor
	// — the report says so rather than letting the gap read as $0.
	Unpriced int
}

// Priced reports whether this segment's dollars are a measurement at all.
//
// False means the two ways of learning what it cost both came up empty: every
// token-bearing message on it was on a model with no rate, and the provider
// named no money either. That is a codex segment — a subscription seat
// reports no cost and no list rate applies to one — and the report prints a
// BLANK there rather than 0.00, because a zero in a money column reads as
// "this bead was free" and it was not. Same rule as uncounted-never-$0 (ADR
// 0012 D4, ADR 0018 §3), pointed at one segment's dollars.
//
// A segment that IS partly priced stays priced: its number is a floor, which
// the report's unpriced line already says, and a floor is still a number.
func (s *Segment) Priced() bool { return s.Unpriced == 0 || s.CostUSD > 0 }

// NotePricedTurn adds one turn's cost that the provider already priced.
func (s *Segment) NotePricedTurn(usd float64) {
	s.ProviderPriced = true
	s.ProviderUSD += usd
}

// NoteCumulative records one CUMULATIVE cost snapshot — a provider restating
// its running total. Max, never sum (ADR 0012 D4).
func (s *Segment) NoteCumulative(usd float64) {
	s.ProviderPriced = true
	if usd > s.ProviderUSD {
		s.ProviderUSD = usd
	}
}

// priceFor resolves a model id through THIS segment's runtime adapter
// (costseam.go's third leg), falling back to the global claude table only
// when no adapter is registered for s.Runtime — the shape a Segment built
// without one (a stray test fixture) has always had. A registered adapter is
// consulted even when it is claude's own: claudeCost.PriceFor delegates to
// the same global, so the claude reading is unchanged (ranger-base-8tut).
func (s *Segment) priceFor(model string) (Price, bool) {
	if p, ok := CostProviderFor(s.Runtime); ok {
		return p.PriceFor(model)
	}
	return PriceFor(model)
}

func (s *Segment) Total() (u Usage, cost float64) {
	if s.ProviderPriced {
		// The provider already priced this. Nothing to reprice, and any
		// tokens it reported are informational — Sum still reports them so
		// the token columns are not blank, but they do not make the money.
		s.CostUSD = s.ProviderUSD
		return s.Sum(), s.CostUSD
	}
	s.Unpriced = 0
	byModel := map[string]float64{}
	for _, m := range s.Msgs {
		u.In += m.In
		u.CacheW += m.CacheW
		u.CacheW5m += m.CacheW5m
		u.CacheW1h += m.CacheW1h
		u.CacheR += m.CacheR
		u.Out += m.Out
		p, ok := s.priceFor(m.Model)
		if !ok {
			// Unknown model: its spend is unknown, not zero. Counted here so
			// the report can say the total is a floor (ADR 0012 D4).
			//
			// Unless it burned no tokens: a record with nothing in any usage
			// field consumed no API, so its cost is a known zero and no rate
			// would change it. Claude Code's `<synthetic>` notices are all of
			// this shape (measured 2026-08-27: 56 records, every field 0), and
			// counting them would print a floor warning on every report
			// forever — a false alarm is its own dishonesty.
			if m.Tokens() > 0 {
				s.Unpriced++
			}
			continue
		}
		c := m.costAt(p)
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
						cur = &Segment{Bead: m[1], Runtime: "claude", File: path, Start: ts, End: ts, Msgs: map[string]*Usage{}}
						segs = append(segs, cur)
					} else if cur == nil && txt != "" && !strings.HasPrefix(txt, "<") {
						cur = &Segment{Bead: "interactive", Runtime: "claude", File: path, Start: ts, End: ts, Msgs: map[string]*Usage{}}
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
// optionally filtered by a project-path substring. The quiet form, for
// callers that only ever display what they found.
func TranscriptFiles(project string) []string {
	files, _ := transcriptFiles(project)
	return files
}

// transcriptFiles is TranscriptFiles with the reasons it found nothing.
//
// "No transcripts here" and "cannot read where the transcripts are" are two
// different facts and this listing used to return the same empty slice for
// both — which is how an unreadable root reads as $0 spent (ADR 0018 §3).
// A root that does not exist IS no records: a machine that has never run
// the CLI has nothing to count, and calling that unreadable would park a
// fresh instance on its first blind pass. Anything else — a permission, a
// broken mount, a directory replaced by a file — is a read failure and says
// so.
//
// It walks with os.ReadDir rather than filepath.Glob because Glob discards
// every I/O error by design (path/filepath/match.go glob(): "ignore I/O
// error"). Guarding it with one os.Stat on the root caught only the arm
// stat can see — and stat on a directory needs nothing but +x on its
// PARENT, so the ADR's own example (root chmod 000) walked straight past
// the guard and came back empty. Errors are collected, not returned on the
// first one: a project dir that will not open hides an unknown spend, and
// the rest of the ledger is still the best floor available.
func transcriptFiles(project string) ([]string, []error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, []error{err}
	}
	root := filepath.Join(home, ".claude", "projects")
	projects, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // never ran the CLI: no records, not a fault
		}
		return nil, []error{err}
	}
	var out []string
	var errs []error
	for _, p := range projects {
		dir := filepath.Join(root, p.Name())
		if !p.IsDir() {
			// Glob resolved this level with os.Stat, so a symlinked project
			// dir counted; keep that. Anything else here is not a project.
			if p.Type()&os.ModeSymlink == 0 {
				continue
			}
			st, err := os.Stat(dir)
			if err != nil {
				if !os.IsNotExist(err) { // a dangling link points at no records
					errs = append(errs, err)
				}
				continue
			}
			if !st.IsDir() {
				continue
			}
		}
		ents, err := os.ReadDir(dir)
		if err != nil {
			// This project's spend is unknown, not zero — the same rule
			// ScanCosts keeps for a file it cannot open, one level up.
			if !os.IsNotExist(err) { // removed mid-walk: nothing left to count
				errs = append(errs, err)
			}
			continue
		}
		for _, e := range ents {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
				continue
			}
			f := filepath.Join(dir, e.Name())
			if project == "" || strings.Contains(f, project) {
				out = append(out, f)
			}
		}
	}
	sort.Strings(out)
	return out, errs
}

// CostReport is everything posse cost prints and the cockpit reads.
type CostReport struct {
	Beads       []*Segment
	Interactive Usage
	InterCost   float64
	InterTurns  int
	// InterBlank counts interactive turns whose dollars were never measured
	// — the same gap the per-bead table prints as a blank, which here would
	// otherwise hide inside a mixed turn count and quietly deflate the
	// interactive-to-fleet ratio the line exists to show.
	InterBlank int
	Uncounted  int // persona sessions on runtimes with no cost adapter
	// UncountedRuntimes names those runtimes, sorted and deduped, so the
	// report can say which spend is missing instead of naming a fixed pair.
	UncountedRuntimes []string
	Since             time.Time

	// What the scan could NOT read (ADR 0018 §3). Beads/Interactive hold
	// what it could; these two say what is missing from them, so a caller
	// that gates on money can tell "nothing was spent" from "nothing could
	// be counted". ReadErr is the first failure, Unread how many there were.
	// Nothing here is subtracted from a total — an uncountable ledger has no
	// total, and the caller decides what that licenses.
	ReadErr error
	Unread  int

	// Unpriced counts messages on models with no rate, across every segment
	// (Segment.Unpriced summed). Non-zero means the dollar totals here are a
	// FLOOR: that spend is unknown, not zero, and is reported rather than
	// guessed at (ADR 0012 D4).
	Unpriced int

	// Dial E's caps (ADR 0003 §4), for the report footer only — filled by
	// the caller from config; 0 = unset = no cap. Nothing here enforces
	// them: `posse cost` reads, dispatch decides.
	PassCap, DayCap float64
}

// ScanCosts scans every registered provider's transcripts (or those matching
// project) since a time. Which providers those are is the registry's business
// and never this function's (ADR 0012 D4); a runtime with no adapter
// contributes no segments here and is counted as uncounted instead.
func ScanCosts(project string, since time.Time) *CostReport {
	// A scanner with no history: every file is decoded, exactly as this
	// function has always done. A caller that scans REPEATEDLY holds a
	// CostScanner of its own instead.
	return new(CostScanner).Scan(project, since)
}

// CostScanner is ScanCosts with a memory: it remembers what each transcript
// decoded to and re-reads only the files whose bytes have changed. One-shot
// callers want ScanCosts; a caller on a timer wants one of these, kept.
//
// Why it exists (ranger-base-325q). The cockpit re-scanned the whole 14-day
// transcript pile every 30 seconds. Measured on this shop 2026-08-29: 1211
// files, 786 MB, of which 1206 files / 784.6 MB had not been written in the
// last 30 seconds — 6.9s of wall and ~5.5s of CPU per tick, ~18% of a core
// spent re-decoding bytes that could not have changed, on an IDLE box. A CPU
// profile put 62% of the process's samples in the transcript decoder.
//
// The memo key is the file's bytes' identity — mtime and size, both of which
// an appended JSONL changes — plus the `since` the answer was computed
// under, because ScanTranscript drops assistant records before it and a
// different cut is a different answer. A caller that wants hits must
// therefore hold its `since` still between scans; the cockpit truncates its
// window to the hour for exactly this reason.
//
// Callers get COPIES of the cached segments. The scan itself writes
// Segment.Runtime and CostReport.AttributePersonas writes Segment.Persona:
// handing out the memo's own pointers would let one report's attribution
// rewrite the next one's, and would race a concurrent scan. The Msgs map is
// shared, not copied — nothing mutates it after the decode returns.
//
// The zero value is ready to use and safe for concurrent Scans, and so is a
// nil *CostScanner — it is a scanner with no memory, which is exactly what
// ScanCosts wants.
type CostScanner struct {
	mu   sync.Mutex
	memo map[string]scanEntry
}

// scanEntry is one file's decode and the bytes it was taken from. err is
// remembered with the rest: the same bytes decode to the same failure, and
// the report must go on counting it as unread every scan (ADR 0018 §3).
type scanEntry struct {
	mtime time.Time
	size  int64
	since time.Time
	segs  []*Segment
	err   error
}

// Scan is ScanCosts over this scanner's memory.
func (cs *CostScanner) Scan(project string, since time.Time) *CostReport {
	rep := &CostReport{Since: since}
	seen := map[string]bool{}
	for _, p := range CostProviders() {
		rep.scanProvider(cs, p, project, since, seen)
	}
	cs.forget(seen)
	sort.Slice(rep.Beads, func(i, j int) bool { return rep.Beads[i].Start.Before(rep.Beads[j].Start) })
	return rep
}

// memoKey namespaces a path by its adapter: two providers may legitimately
// locate the same file and decode it differently.
func memoKey(p CostProvider, path string) string { return p.Runtime() + "\x00" + path }

// decode is p.Decode through the memo. A file whose bytes cannot be
// identified is decoded and never remembered — a cache entry nothing can
// invalidate is worse than no cache.
//
// The stat is taken BEFORE the decode on purpose. A file appended in between
// leaves the entry stamped with the OLDER mtime, so the next scan re-decodes
// it: the wasted read is the safe direction. Stamping after the decode would
// record bytes newer than the answer and serve a stale segment forever.
func (cs *CostScanner) decode(p CostProvider, path string, since time.Time) ([]*Segment, error) {
	if cs == nil {
		return p.Decode(path, since)
	}
	st, err := os.Stat(path)
	if err != nil {
		return p.Decode(path, since)
	}
	key := memoKey(p, path)
	cs.mu.Lock()
	e, ok := cs.memo[key]
	cs.mu.Unlock()
	if ok && e.size == st.Size() && e.mtime.Equal(st.ModTime()) && e.since.Equal(since) {
		return copySegments(e.segs), e.err
	}
	segs, derr := p.Decode(path, since)
	cs.mu.Lock()
	if cs.memo == nil {
		cs.memo = map[string]scanEntry{}
	}
	cs.memo[key] = scanEntry{mtime: st.ModTime(), size: st.Size(), since: since, segs: segs, err: derr}
	cs.mu.Unlock()
	return copySegments(segs), derr
}

// Remembered is how many files this scanner is holding a decode for. It
// exists for the tests that have to say WHICH scanner did a read — a memo is
// invisible from its answers, which are identical either way.
func (cs *CostScanner) Remembered() int {
	if cs == nil {
		return 0
	}
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return len(cs.memo)
}

// forget drops the memo of every file this scan did not list. Without it a
// cockpit open for a week would remember every transcript ever rotated away.
func (cs *CostScanner) forget(seen map[string]bool) {
	if cs == nil {
		return
	}
	cs.mu.Lock()
	defer cs.mu.Unlock()
	for k := range cs.memo {
		if !seen[k] {
			delete(cs.memo, k)
		}
	}
}

// copySegments shallow-copies a decode so the caller can write Runtime and
// Persona onto its own segments without touching the memo's.
func copySegments(in []*Segment) []*Segment {
	if in == nil {
		return nil
	}
	out := make([]*Segment, len(in))
	for i, s := range in {
		c := *s
		out[i] = &c
	}
	return out
}

// scanProvider folds one adapter's transcripts into the report. A provider
// whose records cannot be looked for is a read failure like any other (ADR
// 0018 §3): the rest of the scan continues — a partial ledger is still the
// best floor available — and the report remembers that it is a floor.
func (r *CostReport) scanProvider(cs *CostScanner, p CostProvider, project string, since time.Time, seen map[string]bool) {
	files, errs := p.Transcripts(project)
	for _, err := range errs {
		// A locator failure hides an unknown number of transcripts, so what
		// follows is a floor even if every file it did find reads cleanly.
		// One per failure, not one per provider: a walk that could not open
		// three project dirs lost three unknown piles of spend (ADR 0018 §3).
		// Report that rather than an empty scan that reads as a quiet day.
		r.noteUnread(err)
	}
	for _, f := range files {
		// Listed is enough to keep the memo of it: a file skipped below is
		// still a file that exists, and forgetting it here would re-decode
		// it the moment it is written again.
		seen[memoKey(p, f)] = true
		// A file untouched since `since` holds no record after it, and the
		// decoder would drop every segment it built from one. Skipping
		// it on mtime alone is what makes dispatch's per-launch budget check
		// (ADR 0003 Dial E) affordable as the transcript pile grows.
		if !since.IsZero() {
			if st, err := os.Stat(f); err == nil && st.ModTime().Before(since) {
				continue
			}
		}
		segs, err := cs.decode(p, f, since)
		if err != nil {
			// This file's spend is unknown, not zero. Keep scanning — a
			// partial ledger is still the best floor available — but
			// remember that it is a floor.
			r.noteUnread(err)
			continue
		}
		for _, s := range segs {
			s.Runtime = p.Runtime()
			if s.Bead == "interactive" {
				u, c := s.Sum(), s.CostUSD
				r.Interactive.In += u.In
				r.Interactive.CacheW += u.CacheW
				r.Interactive.CacheR += u.CacheR
				r.Interactive.Out += u.Out
				r.InterCost += c
				r.InterTurns += s.Turns()
				if !s.Priced() {
					r.InterBlank += s.Turns()
				}
				continue
			}
			r.Beads = append(r.Beads, s)
		}
	}
}

// noteUnread records a read failure: the count for how bad, the first
// error for what to print.
func (r *CostReport) noteUnread(err error) {
	r.Unread++
	if r.ReadErr == nil {
		r.ReadErr = err
	}
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

// CountUncounted counts live persona sessions on runtimes with no cost
// adapter registered.
//
// What makes a runtime countable is an adapter, not its name (ADR 0012 D4):
// a runtime gains a price table, a locator and a decoder and drops out of
// this count on the same commit, with nothing here to edit. The test used to
// be `!= "claude"`, which would have gone on calling a counted runtime
// uncounted forever.
func (r *CostReport) CountUncounted(hb *HerdrBackend) {
	ss, err := hb.Sessions()
	if err != nil {
		return
	}
	seen := map[string]bool{}
	for _, s := range ss {
		if s.Agent == "" || s.Runtime == "" {
			continue
		}
		if _, ok := CostProviderFor(s.Runtime); ok {
			continue
		}
		r.Uncounted++
		if !seen[s.Runtime] {
			seen[s.Runtime] = true
			r.UncountedRuntimes = append(r.UncountedRuntimes, s.Runtime)
		}
	}
	sort.Strings(r.UncountedRuntimes)
}

// CountUnpriced sums the unpriced-message count over every segment, and
// caches it on the report. Non-zero means the totals are a floor.
func (r *CostReport) CountUnpriced() int {
	n := 0
	for _, s := range r.Beads {
		n += s.Unpriced
	}
	r.Unpriced = n
	return n
}

// BlankBeads names the beads whose dollars were never measured — every
// segment of theirs unpriced (Segment.Priced false). ByBead reports 0.00 for
// exactly these, and a caller that renders that 0 as money says a codex bead
// was free. It is a separate lookup rather than a flag on the ByBead value
// because a bead worked on BOTH a priced and an unpriced runtime — the same
// id appearing on claude and on codex, which the live ledger does contain —
// has a real number that is a floor, and belongs in neither category's
// simple reading.
func (r *CostReport) BlankBeads() map[string]bool {
	measured, blank := map[string]bool{}, map[string]bool{}
	for _, s := range r.Beads {
		if s.Priced() {
			measured[s.Bead] = true
			continue
		}
		blank[s.Bead] = true
	}
	for id := range measured {
		delete(blank, id)
	}
	return blank
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

// costBlank is what the api$ column prints when a segment's dollars were
// never measured — width-matched to the "%7.2f" a priced row prints, so the
// column stays a column. It is a marker and not an empty string on purpose: a
// truly blank cell is indistinguishable from a rendering bug, and the legend
// has something to name.
const costBlank = "      —"

type costGroup struct {
	Key  string
	N    int
	Cost float64
	// Blank counts the segments in this group whose dollars are unknown
	// (Segment.Priced false). Cost is the sum of the rest, so Blank == N
	// means the group has no money in it at all — which prints as a blank,
	// not as $0.00 — and 0 < Blank < N means Cost is a floor.
	Blank int
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
		if !s.Priced() {
			g.Blank++
			continue
		}
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

// Print renders the report: per bead, then by runtime / tier / persona / day,
// then interactive and the uncounted note.
//
// Two kinds of row share the money column and must never be confused. A
// PRICED segment prints a figure; an unpriced one prints costBlank and is
// kept out of every statistic — it is not averaged in as a zero, not summed,
// and not counted in a median. A number computed over blanks would be a
// number about a pool that has no dollars, presented beside one that does.
func (r *CostReport) Print(w io.Writer) {
	fmt.Fprintf(w, "%-16s %-16s %-8s %-8s %-8s %5s %8s %9s %10s %7s\n", "bead", "start", "persona", "runtime", "tier", "turns", "out", "cache_w", "cache_r", "api$")
	var all []float64
	blankRuntimes := map[string]bool{}
	for _, s := range r.Beads {
		u := s.Sum()
		p, rt := s.Persona, s.Runtime
		if p == "" {
			p = "?"
		}
		if rt == "" {
			rt = "?"
		}
		// The $ column: a number when this segment's dollars were measured,
		// the blank marker when they were not. Only measured dollars join
		// the summary line below — averaging a blank as zero would drag
		// every statistic under it toward a number nobody counted.
		money := costBlank
		if s.Priced() {
			money = fmt.Sprintf("%7.2f", s.CostUSD)
			all = append(all, s.CostUSD)
		} else {
			blankRuntimes[rt] = true
		}
		fmt.Fprintf(w, "%-16s %-16s %-8s %-8s %-8s %5d %8d %9d %10d %7s\n", s.Bead, s.Start.Local().Format("2006-01-02 15:04"), p, rt, TierForModel(s.Model), s.Turns(), u.Out, u.CacheW, u.CacheR, money)
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
			if g.Blank == g.N { // nothing in this group had a price at all
				fmt.Fprintf(w, "  %-14s %3d bead(s)  sum %8s  (no bead here has a rate — turns and tokens above are the measurement)\n", g.Key, g.N, costBlank)
				continue
			}
			priced := g.N - g.Blank
			floor := ""
			if g.Blank > 0 {
				floor = fmt.Sprintf("  (%d of %d unpriced — sum is a floor, median and per-bead are over the %d priced)", g.Blank, g.N, priced)
			}
			fmt.Fprintf(w, "  %-14s %3d bead(s)  sum $%7.2f  median $%.2f  per-bead $%.2f%s\n", g.Key, g.N, g.Cost, median(g.costs), g.Cost/float64(priced), floor)
		}
	}
	// Runtime first of the groupings: on a mixed day it is the one that says
	// which numbers are dollars and which are blanks, and the tier and
	// persona rows below read differently once you know.
	section("runtime (the pool the work was paid out of)", groupBy(r.Beads, func(s *Segment) string {
		if s.Runtime == "" {
			return "?"
		}
		return s.Runtime
	}))
	section("tier (from the model that did the work)", groupBy(r.Beads, func(s *Segment) string { return TierForModel(s.Model) }))
	section("persona (bead assignee)", groupBy(r.Beads, func(s *Segment) string {
		if s.Persona == "" {
			return "?"
		}
		return s.Persona
	}))
	section("day", groupBy(r.Beads, func(s *Segment) string { return s.Start.Local().Format("2006-01-02") }))
	if r.ReadErr != nil {
		// ADR 0018 §3: a transcript the scan could not read is spend that
		// is missing from every number above, not spend that did not
		// happen. Same shape dispatch's stderr witness uses, so a receipt
		// and a pass log name one condition the same way.
		fmt.Fprintf(w, "\nunreadable: %d transcript(s) unreadable (%v) — the ledger counts less than was spent; every total above is a floor\n", r.Unread, r.ReadErr)
	}
	// The other way the totals are a floor: a file that read fine but whose
	// model has no rate. Both lines can print; they are different gaps.
	if n := r.CountUnpriced(); n > 0 {
		fmt.Fprintf(w, "\nunpriced: %d message(s) on models with no rate — their spend is unknown, not zero, so every $ above is a floor\n", n)
	}
	if len(blankRuntimes) > 0 {
		which := make([]string, 0, len(blankRuntimes))
		for rt := range blankRuntimes {
			which = append(which, rt)
		}
		sort.Strings(which)
		fmt.Fprintf(w, "legend: %q in api$ means the runtime reported no cost and no rate card here applies (%s) — the turns and tokens on those rows ARE the measurement, and a blank beats a number this report would have invented\n",
			strings.TrimSpace(costBlank), strings.Join(which, "/"))
	}
	blank := ""
	if r.InterBlank > 0 {
		blank = fmt.Sprintf(", %d of them unpriced so the $ is a floor", r.InterBlank)
	}
	fmt.Fprintf(w, "\ninteractive: %d turns%s, api-equiv $%.2f (not gated — shown so the ratio is visible)\n", r.InterTurns, blank, r.InterCost)
	if r.Uncounted > 0 {
		which := strings.Join(r.UncountedRuntimes, "/")
		if which == "" {
			which = "runtimes with no adapter"
		}
		fmt.Fprintf(w, "uncounted: %d live persona session(s) on %s — no cost adapter for that runtime; their spend is not in these numbers, and is not zero\n",
			r.Uncounted, which)
	} else {
		fmt.Fprintf(w, "counted runtimes: %s — a session on any other runtime is uncounted, never $0 (none live now)\n",
			strings.Join(CountedRuntimes(), "/"))
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
			// "at least" when the scan could not read everything: the day
			// spend is what Dial E measures against, and an unreadable
			// transcript makes it a floor rather than the total.
			spent := fmt.Sprintf("spent $%.2f today", st.DaySpend)
			if r.ReadErr != nil {
				spent = fmt.Sprintf("spent at least $%.2f today", st.DaySpend)
			}
			caps = append(caps, fmt.Sprintf("budget_day $%.2f (%s)", r.DayCap, spent))
		}
		fmt.Fprintf(w, "budget (ADR 0003 Dial E): %s — at %.0f%% of a window standard steps down to fast, at %.0f%% dispatch stops\n",
			strings.Join(caps, ", "), BudgetStepDownPct, BudgetStopPct)
	} else {
		fmt.Fprintln(w, "budget (ADR 0003 Dial E): no caps set (budget_pass:/budget_day: unset) — dormant, nothing steps down and nothing stops")
	}
}
