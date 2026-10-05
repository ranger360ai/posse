package posse

// The readings log — ADR 0066 D1, from the spike in
// docs/notes.d/ranger-base-our1e.md.
//
// THE FINDING THIS EXISTS FOR, in the spike's own words: "posse records its
// incidents by bead but not its readings by count, so the error rate of
// today's readers is UNKNOWN, and no reader — model or regex — can be shown
// better." Nine screen-reading incidents since 2026-09 are counted by bead
// (rangerhq-7ia, rangerhq-1xsj, ranger-base-0sa5a, -b96nx, -htafy, -i6t90,
// -wr624, -3j8, -3p0) and the denominator — how many readings those nine
// came out of — is recorded nowhere. Every evaluator the spike read named
// the same precondition before any second reader is worth buying: a labelled
// log of what the current reader decided, on what bytes.
//
// So this is the denominator, and it is worth having whether or not a model
// ever follows (ADR 0066 D1, operator ruling 2026-10-04 (A)).
//
// WHAT IS RECORDED. Not every reading — the consequential ones. A pass takes
// hundreds of screen readings and almost all of them are "the agent is
// working, carry on"; what the incident stream is made of is the readings
// that SPENT something: a launch refused, a claim handed back, a settle-open
// comment written, a ghost retirement that dropped a row, a hold that kept a
// seat parked. Those five are the consequences in ADR 0066 D1, and they are
// the whole of what is appended here. A reading with no consequence is a
// reading nobody has ever had to diagnose.
//
// WHAT RIDES WITH IT. The decision id (D1-D6, the spike's §3 map), the
// verdict, the rule id or code path that produced it, the herdr reading
// beside it, a timestamp, and the exact region bytes it read — footer,
// prompt box, status line, whichever the reader actually looked at. The
// bytes are the part that makes this a corpus rather than a counter:
// ranger-base-0sa5a found each of its three rule misses by replaying six
// captures, and every miss was one codepoint. A log that said "D1, idle,
// live_prompt_box" and not what was on the screen would count the incidents
// and reproduce none of them.
//
// WHERE IT LIVES, AND WHY THERE. Inside the session tree's own git dir
// (`<repo>/.git/worktrees/<session>/`), which is the one directory that is
// per session tree, invisible to `git status`, and removed by git itself
// when the tree is removed — the three properties ADR 0066 D1 asks for in
// the words "per session tree, rotated with it, never under $HOME".
//
//   - NOT in the working tree. `dirtyPaths` reads `git status --porcelain`,
//     which counts untracked files, and six callers act on that count:
//     ADR 0041's closed-dirty comment, `RemoveSessionTree`'s refusal, the
//     retire guard, the land sweep, the reap guard. A log file in the tree
//     would make every close a dirty close and every tree unretirable — a
//     diagnostic that breaks the thing it is diagnosing.
//   - NOT under $HOME's own stores. `a.StateDir` and `a.Home` outlive every
//     session, so a log there accumulates forever and is nobody's to rotate;
//     ADR 0011's rule against a fifth store is the other half of the reason.
//     ReadingsLogPath REFUSES such a path rather than falling back to one:
//     a session with no tree writes NO log, which is the only spelling of
//     "never under $HOME" that a pin can hold.
//   - It does not advance the tree's quiet clock. `lastTreeWrite` (retire.go)
//     takes the newest mtime of every file in the git dir, so an instrument
//     writing there would keep `posse worktrees --retire` from ever taking
//     the tree — the same shape this repo already paid for twice in
//     `git status`'s index refresh (ranger-base-9u5zy, -a8tqz). That walk
//     skips this file by name, for the reason those two fixes exist: a
//     quiet-tree reading that counts its own observer's notes can never go
//     quiet.
//
// REDACTED BEFORE WRITE. The region bytes are a capture of somebody's
// terminal, so they go through the data ceiling (ADR 0050) on the way in —
// `OpsPatternSet.RedactCeiling`, class-only, the ceiling and not the
// visibility list because the question a local file asks is the ceiling's
// (visibility.go's note on the split). The classes taken ride on the record,
// so a corpus case that cannot be replayed byte for byte says so instead of
// looking complete.
//
// BEST EFFORT, ALWAYS. Every writer here swallows its own failure: a
// read-only git dir, a caged seat that cannot reach another session's repo,
// a full disk. A pass that refuses to dispatch because its instrument could
// not write is an instrument that costs more than the incidents it counts.
//
// NO MODEL, NO NETWORK. ADR 0066 D1's last sentence, and the whole of this
// file's scope: nothing here calls anything, and nothing here decides
// anything. Every verdict in a record was already made and already spent by
// the time it is written down.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ReadingDecision is which of the spike's decision points took the reading
// (docs/notes.d/ranger-base-our1e.md §3). The ids are the map's own, so a
// census row and the table a reader goes back to are keyed the same.
type ReadingDecision string

const (
	// DecisionPaneState (D1) — idle / working / blocked, off screen regions,
	// by herdr's manifest rules. rangerhq-7ia: a footer reword flipped
	// blocked to idle in silence.
	DecisionPaneState ReadingDecision = "D1"
	// DecisionDialog (D2) — is this screen a permission dialog. Same
	// regions as D1; posse branches on state plus rule id and never answers
	// one (ranger-base-0sa5a).
	DecisionDialog ReadingDecision = "D2"
	// DecisionUnknownScreen (D3) — what was herdr looking at when it
	// recognized nothing (unrecognized.go, ranger-base-3j8).
	DecisionUnknownScreen ReadingDecision = "D3"
	// DecisionComposerHold (D4) — working / typed / sent / ghost, off the
	// footer and the prompt box (panework.go, ghostbox.go, sentline.go).
	DecisionComposerHold ReadingDecision = "D4"
	// DecisionStallVerdict (D5) — rewait / keep / hand back on a stalled
	// prompt (promptstall.go, ranger-base-uauvn).
	DecisionStallVerdict ReadingDecision = "D5"
	// DecisionTurnDelivered (D6) — did this turn deliver, off the bead and
	// the runtime's own transcript (turnfailure.go, ranger-base-qcu4c).
	DecisionTurnDelivered ReadingDecision = "D6"
)

// ReadingConsequence is what the reading SPENT — the five in ADR 0066 D1,
// and the whole of what makes a reading worth recording. A reading that
// spent none of them is not logged.
type ReadingConsequence string

const (
	// ConsequenceRefusal — a launch or a prompt refused on the strength of
	// the reading. Nothing was sent.
	ConsequenceRefusal ReadingConsequence = "refusal"
	// ConsequenceHandBack — a claim was given back to the queue.
	ConsequenceHandBack ReadingConsequence = "hand-back"
	// ConsequenceSettleOpen — the settle-without-close rung wrote on the
	// bead (settleopen.go).
	ConsequenceSettleOpen ReadingConsequence = "settle-open"
	// ConsequenceGhostRetired — a claim posse was making about a composer
	// was dropped because the box was drawing claude's own suggestion
	// (ghostbox.go, ranger-base-6o7wm).
	ConsequenceGhostRetired ReadingConsequence = "ghost-retirement"
	// ConsequenceHold — a seat was left parked, or a bead not judged, on
	// the strength of what the pane was holding (panework.go,
	// ranger-base-htafy).
	ConsequenceHold ReadingConsequence = "hold"
)

// ReadingRegion is one screen region exactly as the reader read it: the
// region's name in herdr's manifest, the byte count herdr reported for it,
// and the bytes themselves after the ceiling has had them.
//
// Bytes is herdr's count of the WHOLE region and Text is what arrived, so
// the two disagree on a preview herdr truncated — which is the fact a
// replay has to know (panework.go's 243-character cap) and the reason the
// count is recorded rather than derived from the text.
type ReadingRegion struct {
	Name      string `json:"region"`
	Bytes     int    `json:"bytes"`
	Text      string `json:"text"`
	Truncated bool   `json:"truncated,omitempty"`
}

// ReadingEvidence is the herdr reading that sat beside the verdict: what
// herdr itself said, and what it was looking at when it said it.
//
// Seen is carried rather than derived because it is the one field that says
// whether any of the rest is a screen reading at all: a guess and a
// recognized screen read identically here otherwise (AgentDetection.Seen).
type ReadingEvidence struct {
	State    string          `json:"state,omitempty"`
	Rule     string          `json:"matched_rule,omitempty"`
	Fallback string          `json:"fallback_reason,omitempty"`
	Reported string          `json:"reported,omitempty"`
	Seen     bool            `json:"seen"`
	Regions  []ReadingRegion `json:"regions,omitempty"`
}

// Reading is one consequential reading, as one line of the log.
//
// Rule is the rule id or code path that PRODUCED the verdict, which is not
// always herdr's matched rule and is the field a census groups by when it
// asks which reader is getting things wrong: on D1-D3 it is herdr's
// manifest rule, on D4-D6 it is the posse function that decided.
type Reading struct {
	At          time.Time          `json:"at"`
	Decision    ReadingDecision    `json:"decision"`
	Verdict     string             `json:"verdict"`
	Consequence ReadingConsequence `json:"consequence"`
	Rule        string             `json:"rule"`
	Session     string             `json:"session,omitempty"`
	Bead        string             `json:"bead,omitempty"`
	Runtime     string             `json:"runtime,omitempty"`
	Herdr       ReadingEvidence    `json:"herdr"`
	// Gate is a SECOND reading, carried only where the record is about
	// keystrokes posse sent: the pane-state reading that opened the settle
	// gate and therefore TYPED (ADR 0066 D1 as amended 2026-10-04,
	// ranger-base-o1aoi; prices in docs/notes.d/ranger-base-o1aoi.md §4).
	//
	// IT IS A SEPARATE FIELD AND NOT A WIDER `Herdr`, because the two are
	// different readings by different readers at different instants and a
	// census must not add them up. Herdr above stays what it has always
	// been — on a D5 record, the stall verdict's own two halves, which read
	// no screen at all (promptstall.go). This one is herdr's D1 reading,
	// labelled as the gate's, with the screen from each side of the
	// keystrokes in its regions: PaneCaptureAtPromptRegion is what the pane
	// held the instant before the text went in, PaneCaptureRegion what it
	// held when the stall was judged. Either may be absent — both are best
	// effort — and the pair is the evidence a false IDLE is diagnosed from
	// (the note's §3: one alone says "idle" or "empty", the two together
	// say "eaten").
	//
	// A nil Gate is every record that is not about a keystroke posse typed,
	// which is every decision but D5 and, within D5, the launch-line path:
	// nothing is typed there, so there is no reading that typed.
	Gate *ReadingEvidence `json:"gate,omitempty"`
	// Redacted names the data-ceiling classes taken out of the regions
	// before this line was written (ADR 0050 D2, class only). Non-empty
	// means the bytes are not the bytes that were read, so a replay over
	// them proves nothing. BOTH blocks' regions go through the ceiling and
	// both blocks' classes land here: a capture is a capture of somebody's
	// terminal whichever reading carried it.
	Redacted []string `json:"redacted,omitempty"`
	// Posse is the version that took the reading. A corpus outlives the
	// rules it was read by, and a case whose verdict disagrees with today's
	// reader is either a fixed bug or a new one — this is how a reader
	// tells those apart without guessing from the date.
	Posse string `json:"posse,omitempty"`
}

// ReadingsLogName is the log, and it is a fixed name inside the session
// tree's git dir rather than a configured path: the location is derived from
// the tree, so there is nothing to configure and no way for a second
// instance to be pointed at the first one's log.
//
// lastTreeWrite (retire.go) skips exactly this name. Renaming it without
// renaming it there makes every tree that ever took a reading unretirable.
const ReadingsLogName = "posse-readings.jsonl"

// ReadingsLogPath is where one session's readings log goes: inside the git
// dir of the tree it is a log ABOUT.
//
// It refuses rather than falling back, in both directions:
//
//   - an empty tree path, or one git will not answer for, gets an error and
//     no log. A reading posse cannot place is a reading posse does not
//     write; the alternative is a home-rooted default, which is the one
//     thing ADR 0066 D1 rules out.
//   - a resolved path that lands under the harness's own home stores is
//     refused too, loudly, even though no real repo can produce one. That
//     check is the pin's whole purchase on "never under $HOME": it reads
//     the rule off the code rather than off this comment.
func (a *App) ReadingsLogPath(treePath string) (string, error) {
	if strings.TrimSpace(treePath) == "" {
		return "", Die("readings log: no session tree was named, and a reading is logged under the tree it is about or not at all (ADR 0066 D1)")
	}
	gd, err := git(treePath, "rev-parse", "--absolute-git-dir")
	if err != nil || gd == "" {
		return "", Die("readings log: %s is not a git work tree, so there is no per-tree place to put the log (%v)", AbbrevHome(treePath), err)
	}
	path := filepath.Join(gd, ReadingsLogName)
	if why := a.readingsLogOutOfBounds(path); why != "" {
		return "", Die("readings log: refusing %s — %s (ADR 0066 D1: per session tree, rotated with it, never under the harness's own home)", AbbrevHome(path), why)
	}
	return path, nil
}

// readingsLogOutOfBounds names the home-rooted store a resolved log path
// would land in, or "" when it lands in none.
//
// The session tree itself is under $HOME by rule (WorktreeRoot refuses a
// root outside it, because a reaped worktree under a live session destroys
// the work in it), so "never under $HOME" cannot mean the home directory —
// it means never in a store whose lifetime is the INSTANCE's rather than
// the tree's. These three are those stores: the harness home, the state dir
// under it, and the runtime's own per-user dir.
func (a *App) readingsLogOutOfBounds(path string) string {
	for _, bound := range []struct{ dir, why string }{
		{a.StateDir, "the state dir outlives every session, so nothing there is rotated with a tree"},
		{a.Home, "the harness home outlives every session, and a log there would be the fifth store ADR 0011 refuses"},
		{filepath.Join(ExpandTilde("~"), ".claude"), "the runtime's own per-user dir is not posse's to write a log into"},
	} {
		if bound.dir == "" {
			continue
		}
		if resolvedPath(path) == resolvedPath(bound.dir) || pathUnder(path, bound.dir) {
			return bound.why
		}
	}
	return ""
}

// AppendReading writes one record to the log for treePath. Best effort: the
// error is returned for the pins and discarded by every caller in a pass
// (LogReading below is the caller's spelling).
//
// ONE WRITE PER RECORD, O_APPEND. Several posse processes can be reading the
// same fleet at once — a watch loop, an operator's `posse status`, a
// persona's `posse pulse` — and O_APPEND plus a SINGLE `write(2)` of the
// whole record is the cheapest arrangement under which two of them cannot
// interleave a line.
//
// THE BOUND IS THE CALL, NOT A PAGE, and this comment used to say a page
// (ranger-base-76gc4). The page was never the mechanism, and since the D3
// record carries a whole pane capture (PaneCaptureRegion) it is not even
// true of the shape. MEASURED 2026-10-04 on this box (darwin 25.4.0, APFS,
// go1.26.5), six and sixteen concurrent processes each appending
// fixed-size records to one file, every line checked for length and for a
// single writer's bytes: 0 torn lines at 512, 4096, 4097, 8192, 16384,
// 65536 and 131072 bytes. The kernel holds the inode across the call, so
// what keeps a line whole is that there is one call — which is why this
// marshals first and writes once, and why a second Write here would be the
// bug however small the record.
//
// WHAT A RECORD ACTUALLY COSTS, MEASURED the same day: six truncated
// previews and no capture, 2,218 bytes; the same six plus the tallest
// fixture posse owns (grok/idle-startup-splash-wide-boxed.txt, 3,969
// bytes), 6,297; plus a 60x200 screen of box-drawing glyphs — three bytes a
// column, and the worst shape a `--source detection` read can hand us —
// 38,388. All three are one write and all three measured clean above.
//
// WHAT TEARS, since something must be named. Not concurrency, at any size
// tried: a SHORT write. `os.File.Write` does not retry — it returns
// io.ErrShortWrite when the syscall took fewer bytes than it was given — so
// a full disk or a signalled partial write leaves a fragment and returns an
// error this function hands back and every caller in a pass swallows
// (LogReading). ReadReadings then skips the unparseable line and
// `scripts/readings-census.py` counts it torn, which costs one reading and
// not the corpus. That is the same failure the old comment described; the
// trigger is the disk, never the page.
func (a *App) AppendReading(treePath string, r Reading) error {
	path, err := a.ReadingsLogPath(treePath)
	if err != nil {
		return err
	}
	if r.At.IsZero() {
		r.At = time.Now().UTC()
	} else {
		r.At = r.At.UTC()
	}
	if r.Posse == "" {
		r.Posse = VersionString()
	}
	r.Herdr.Regions, r.Redacted = a.redactRegions(r.Herdr.Regions)
	// The gate block's regions are a pane capture of somebody's terminal
	// exactly like the rest (ranger-base-dckhf), so they go through the
	// same ceiling and their classes join the same list. COPIED first: Gate
	// is a pointer, and this function takes its Reading by value — so
	// redacting through the pointer would reach back into the caller's own
	// in-flight state and leave the pendingBead holding redacted bytes.
	if r.Gate != nil {
		g := *r.Gate
		var took []string
		g.Regions, took = a.redactRegions(g.Regions)
		r.Gate = &g
		if len(took) > 0 {
			r.Redacted = dedupeStrings(append(r.Redacted, took...))
		}
	}
	line, err := json.Marshal(r)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(line, '\n'))
	return err
}

// redactRegions puts every region's bytes through the data ceiling and
// returns the classes it took, deduped and in Ceiling's order. The region
// NAMES are herdr's manifest vocabulary and are never scanned: they carry
// no content, and a class that matched one would redact a field a census
// groups by.
func (a *App) redactRegions(in []ReadingRegion) ([]ReadingRegion, []string) {
	if len(in) == 0 {
		return in, nil
	}
	set := a.OpsPatternSet()
	if len(set.Ceiling) == 0 {
		return in, nil
	}
	out := make([]ReadingRegion, 0, len(in))
	var classes []string
	seen := map[string]bool{}
	for _, reg := range in {
		text, took := set.RedactCeiling(reg.Text)
		reg.Text = text
		out = append(out, reg)
		for _, c := range took {
			if !seen[c] {
				seen[c] = true
				classes = append(classes, c)
			}
		}
	}
	return out, classes
}

// ReadReadings parses a log. A line that will not parse is SKIPPED rather
// than failing the read: the log is appended to by several processes and the
// one shape that can go wrong is a torn line, which costs one reading and
// must not cost the corpus.
func ReadReadings(path string) ([]Reading, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Reading
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var r Reading
		if json.Unmarshal([]byte(line), &r) != nil {
			continue
		}
		out = append(out, r)
	}
	return out, sc.Err()
}

// ─── building a record out of a reading posse already took ──────────────────

// ReadingEvidenceOf is the herdr half of a record, taken off the detection
// the reader already had. Every region herdr evaluated is carried, in
// herdr's own order, deduped by region name — which is `WhatHerdrSaw`'s
// grouping and for its reason: rules outnumber regions three to one and
// share their evidence.
//
// It is the WHOLE of herdr's working rather than the regions one decision
// happened to read, because the incident class this corpus exists for is a
// rule that read the wrong region (rangerhq-7ia's footer, 0sa5a's three
// misses) and a log holding only the region the rule DID read cannot show
// that.
func ReadingEvidenceOf(d AgentDetection) ReadingEvidence {
	ev := ReadingEvidence{
		State:    d.State,
		Rule:     d.Rule.ID,
		Fallback: d.FallbackReason,
		Reported: d.Reported,
		Seen:     d.Seen(),
	}
	at := map[string]bool{}
	for _, r := range d.EvaluatedRules {
		if at[r.Region] {
			continue
		}
		at[r.Region] = true
		ev.Regions = append(ev.Regions, ReadingRegion{
			Name:      r.Region,
			Bytes:     r.Evidence.RegionBytes,
			Text:      r.Evidence.RegionPreview,
			Truncated: r.Evidence.RegionBytes > len(r.Evidence.RegionPreview),
		})
	}
	return ev
}

// AsDetection rebuilds the detection a record was taken from, as far as its
// regions allow: one synthetic evaluated rule per region, carrying the
// bytes. That is enough for every reader in panework.go, ghostbox.go and
// sentline.go, all of which read regions by name and nothing else — so a
// logged reading can be re-decided IN PROCESS by today's rules, which is
// what ADR 0066 D1's "re-running the current rules" asks for on the
// decisions posse owns.
//
// It cannot re-run D1-D3: those rules are herdr's TOML manifest, not Go, and
// a Go reimplementation of them would be a second reader to keep in sync by
// hand (the mistake the tree-wide-pin register exists to refuse). For those
// the corpus is for `herdr agent explain --file <capture> --agent <name>`,
// which reads a saved screen with no live pane, and the census script
// exports the bytes in that shape.
func (r Reading) AsDetection() AgentDetection {
	d := AgentDetection{
		State:          r.Herdr.State,
		FallbackReason: r.Herdr.Fallback,
		Reported:       r.Herdr.Reported,
	}
	d.Rule.ID = r.Herdr.Rule
	for _, reg := range r.Herdr.Regions {
		var er EvaluatedRule
		er.ID = "replay:" + reg.Name
		er.Region = reg.Name
		er.Evidence.RegionBytes = reg.Bytes
		er.Evidence.RegionPreview = reg.Text
		d.EvaluatedRules = append(d.EvaluatedRules, er)
	}
	return d
}

// ReplayHold re-runs the composer-hold reader (D4) over a record's own
// bytes and returns what today's rules say about them.
//
// SCREEN ONLY, and the record says so: `PaneHolding` asks two more stores
// after this — claude's submit log (sentline.go) and a second ANSI read
// (ghostbox.go) — and neither is in the regions. So this reproduces the half
// of the verdict the screen decided, which is the half every one of the D4
// incidents turned on, and a replay that disagrees with the logged verdict
// on `Sent` or `Ghost` is reporting the stores' absence rather than a rule
// change.
func (r Reading) ReplayHold() PaneHold {
	return r.AsDetection().Hold()
}

// ReadingRegionOf returns the recorded bytes of one region of the record's
// OWN reading, and whether it carried it. For a census and for a pin that
// has to assert the bytes a verdict was reproducible from are actually
// there.
//
// `Herdr` and not `Gate`: a record's own reading is the one its verdict came
// from, and a reader that meant the gate's asks the gate's block for it
// (RegionOf below). Two blocks reachable through one lookup would make
// "the record carries this region" an ambiguous sentence.
func (r Reading) ReadingRegionOf(name string) (ReadingRegion, bool) {
	return r.Herdr.RegionOf(name)
}

// RegionOf is the same lookup over any one evidence block, which is what
// makes the gate's captures readable by the same means as herdr's own
// regions rather than by a second loop somebody writes again.
func (e ReadingEvidence) RegionOf(name string) (ReadingRegion, bool) {
	for _, reg := range e.Regions {
		if reg.Name == name {
			return reg, true
		}
	}
	return ReadingRegion{}, false
}

// ─── the rule ids posse's own readers answer to ─────────────────────────────

// A record's Rule is the rule id or code path that produced the verdict
// (ADR 0066 D1). On D1-D3 that is herdr's manifest rule and comes off the
// detection; on D4-D6 the reader is Go, and these are its names. They are
// `file.go:func` because that is what a reader of a census row has to open,
// and they are consts so a rename moves the census with the code.
const (
	RuleComposerHold  = "panework.go:Hold"
	RuleComposerGhost = "ghostbox.go:composerIsGhost"
	RuleStallVerdict  = "promptstall.go:judgeStall"
	RuleTurnOutcome   = "turnfailure.go:TurnOutcomeReader"
	RulePromptReady   = "promptready.go:promptReady"
	RuleSettleOpen    = "settleopen.go:noteSettleOpen"
)

// ─── the log's own lifecycle ────────────────────────────────────────────────

// skipsReadingsLog is lastTreeWrite's half of the placement rule, kept here
// beside the name so the two cannot drift: the quiet-tree clock must not
// count posse's own notes about the tree.
func skipsReadingsLog(e fs.DirEntry) bool { return e.Name() == ReadingsLogName }

// ─── the callers' spelling ──────────────────────────────────────────────────

// LogReading writes one consequential reading to the log of the session it
// is about, and swallows every way that can fail.
//
// IT TAKES A SESSION AND RESOLVES THE TREE, rather than taking a path: every
// consequence site in a pass has a session name and most have no tree in
// hand, and the meta is a file read. A session with no meta, or a meta with
// no dir, writes nothing — there is no tree to put the log under and ADR
// 0066 D1 has no second place to put it.
//
// SILENT, AND THAT IS THE DESIGN. The failures here are a read-only git dir,
// a caged seat that cannot reach another session's repo, a full disk — none
// of which is news to the operator reading a pass, and all of which would
// otherwise print a line beside every refusal and hand-back in the pass.
// Where a reader asks whether the log is being written is
// `scripts/readings-census.py`, which answers 0 readings in 0 logs as
// plainly as it answers a count — one command, and no new CLI surface for
// an instrument that is read once a week.
func (b *HerdrBackend) LogReading(session string, r Reading) {
	if b == nil || b.App == nil || session == "" {
		return
	}
	m, ok := b.readMeta(session)
	if !ok || m.Dir == "" {
		return
	}
	if r.Session == "" {
		r.Session = session
	}
	_ = b.App.AppendReading(m.Dir, r)
}

// logReading is the pass's spelling, and the one thing it adds is the
// --dry-run refusal: a pass that acted on nothing must not leave state a
// later pass counts (seatidle.go's rule, and settleopen.go applies the same
// one to the bead). A dry run prints its skips and its refusals exactly as a
// live pass does, so a census over a log that counted them would price a
// decision nobody made.
func (d *Dispatcher) logReading(session string, r Reading) {
	if d.DryRun {
		return
	}
	if r.At.IsZero() {
		r.At = d.now()
	}
	d.HB.LogReading(session, r)
}

// d3Evidence is a dispatch D3 record's herdr half: what herdr saw, plus the
// pane capture where there is one to take (withPaneCapture).
//
// IT CARRIES logReading's --dry-run RULE ONE STEP EARLIER, and that is the
// whole reason it exists rather than two inline calls. The capture is a
// herdr CALL, and a Go argument is evaluated before the function that would
// have discarded it — so wrapping the evidence at the call site would make
// every dry-run hold and refusal fork a `pane read` whose only consumer is
// a record the dry run then refuses to write.
//
// The HAND path (AwaitPromptable, logUnrecognized) has no dry run to honour
// and takes its capture unconditionally: `posse prompt` either refused or
// it did not.
func (d *Dispatcher) d3Evidence(target string, det AgentDetection) ReadingEvidence {
	ev := ReadingEvidenceOf(det)
	if d.DryRun {
		return ev
	}
	return d.HB.withPaneCapture(target, ev)
}

// gateReading is the evidence a typed prompt carries into its own judgment:
// the settle gate's D1 reading, plus the screen as it was the instant before
// the keystrokes (ADR 0066 D1 as amended, ranger-base-o1aoi §3).
//
// IT IS BUILT AT TYPE TIME AND WRITTEN LATER, OR NEVER. fire holds the
// result on the pendingBead and logStall writes it if — and only if — the
// prompt stalls; a turn that starts drops it. That is the whole shape of the
// amendment: the reading posse wants a record of has no consequence of its
// own to key on (it types, and typing is not one of ADR 0066 D1's five), so
// the record is keyed on the consequence it PRODUCES, which is the D5 stall
// verdict the log already counts. Nothing new is written per prompt and no
// sixth consequence is coined.
//
// ALWAYS NON-NIL on the typed path, even when herdr said nothing worth
// carrying and the capture came back empty: the block then says the gate
// opened on a reading with no rule and no bytes, which is itself the finding
// for a reword that dropped herdr to its idle fallback (rangerhq-7ia). An
// absent block would be indistinguishable from the launch-line path, which
// is a different fact.
//
// It is a Dispatcher method for the backend, not for a dry-run guard: fire
// is unreachable on a dry pass, and that is argued where the call is.
func (d *Dispatcher) gateReading(det AgentDetection, target string) *ReadingEvidence {
	ev := d.HB.withPaneCaptureNamed(target, PaneCaptureAtPromptRegion, ReadingEvidenceOf(det))
	return &ev
}

// TurnOutcomeRegion is what the readings log calls D6's evidence: the
// refusal message the runtime's own record carried (turnfailure.go). It is
// not a screen region and is named apart from herdr's manifest vocabulary
// for that reason — but it is the bytes the verdict was taken from, which is
// what the log records (ADR 0066 D1), and the one reader in the map whose
// rule is three literal phrasings of a vendor's wording.
const TurnOutcomeRegion = "turn_outcome_message"

// turnReading is the D6 reading as a record's evidence: which of the three
// states the turn-outcome reader was in, and the message if there was one.
//
// The THREE states are kept apart exactly as turnOutcomeLines keeps them,
// because they are the difference between "this settle is news" and "this
// settle is what posse cannot see" (ranger-base-02zr, ranger-base-1mei):
//
//	blind      no reader for this runtime at all
//	unobserved a reader looked and the record was not readable
//	answered   a reader read the turn; Message is "" or the refusal
//
// A census that collapsed them would price the readings posse took against
// the settles it never read, which is the arithmetic ADR 0066 D1 exists to
// make possible.
func turnReading(find TurnOutcomeReader, observed bool, o TurnOutcome) ReadingEvidence {
	ev := ReadingEvidence{Seen: observed}
	switch {
	case find == nil:
		ev.State = "blind"
	case !observed:
		ev.State = "unobserved"
	default:
		ev.State = "answered"
	}
	if o.Message != "" {
		ev.Regions = []ReadingRegion{{
			Name:  TurnOutcomeRegion,
			Bytes: len(o.Message),
			Text:  o.Message,
		}}
	}
	return ev
}

// logPromptHandBack writes the record for a claim handed back because herdr
// refused the submission (ADR 0066 D1). This is the consequence the spike's
// D1 and D2 rows both end in — "false blocked hands a claim back", and a
// phantom dialog is the measured way that happens (ranger-base-0sa5a's
// false-block on the permissions picker, one codepoint).
//
// WHICH DECISION IT WAS is read off herdr's own code, because the two are
// different readings with the same consequence (promptstall.go's taxonomy):
//
//	agent_blocked    herdr read a dialog on the screen and refused   → D2
//	agent_not_ready  herdr will not address the pane at all          → D1
//	anything else    the refusal is not a screen reading             → D1
//
// IT PAYS ONE `agent explain` FOR THE BYTES, and only here. A refused
// submission's envelope carries no regions — herdr says `agent_blocked` and
// stops — so without a second read the record would name a verdict and
// reproduce nothing, which is the whole failure the spike priced (a regex's
// miss is replayable at the codepoint, and that is the property the corpus
// has to keep). A hand-back is rare and already costs an unclaim and a bead
// comment; one more herdr call on that path buys the only bytes there are.
// The explain is a SECOND INSTANT and the record says so by carrying herdr's
// state beside the code: a screen that changed between the refusal and the
// read is a case a reader must be able to spot rather than one this function
// should hide.
func (d *Dispatcher) logPromptHandBack(session, bead, runtime, target string, promptErr error) {
	decision := DecisionPaneState
	if IsHerdrCode(promptErr, "agent_blocked") {
		decision = DecisionDialog
	}
	ev := ReadingEvidence{}
	if target != "" {
		if det, err := d.HB.H.AgentExplain(target); err == nil {
			ev = ReadingEvidenceOf(det)
		}
	}
	d.logReading(session, Reading{
		Decision:    decision,
		Verdict:     fmt.Sprintf("hand back: %v", promptErr),
		Consequence: ConsequenceHandBack,
		Rule:        herdrRefusalRule(promptErr, ev),
		Bead:        bead,
		Runtime:     runtime,
		Herdr:       ev,
	})
}

// herdrRefusalRule is what produced a refusal, in the one field a census
// groups by: herdr's matched rule where the second read found one, and
// herdr's own code where it did not. A rule id is what a reader takes to the
// manifest; the code alone is what there is when the screen moved on.
func herdrRefusalRule(promptErr error, ev ReadingEvidence) string {
	if ev.Rule != "" {
		return ev.Rule
	}
	return "herdr:" + HerdrCodeOf(promptErr)
}

// PaneCaptureRegion is the one region on a D3 record that is not herdr's.
// It is named apart from herdr's manifest vocabulary for TurnOutcomeRegion's
// reason — nothing in `etc/herdr/agent-detection/*.toml` answers to it, and
// a census grouping by region name has to be able to tell posse's own read
// from a rule's.
//
// WHAT IT IS, AND WHY A D3 RECORD NEEDS IT (ADR 0066 D3 as amended by
// ranger-base-qk9tr, measured in docs/notes.d/ranger-base-qk9tr.md §5).
// `agent explain` emits a PREVIEW of each region, capped at 243 characters,
// and ReadingEvidenceOf carries exactly that. Every top-anchored region
// starts at the top of the screen, and on codex the top of the screen is
// 15-16 rows of ASCII logo — so for 9 of the 15 labelled screens posse owns
// a capture of, the screen's OWN HEADING ("Sign in with ChatGPT", "Select
// Model and Effort", "Help improve Grok") sits below every preview in the
// record. A reader of any kind handed that record is handed logo art: over
// the full capture a heading reader names 11 of 11 residue cases on the
// three measured incident classes, over the previews 5 of 11. The input was
// the bottleneck, not the reader.
//
// So the record carries the screen once, whole, in the shape a fixture is.
const PaneCaptureRegion = "pane_capture"

// PaneCaptureAtPromptRegion is the OTHER instant, and only a record with a
// Gate block carries it: the screen as it was immediately before posse
// pressed the keys (ADR 0066 D1 as amended, ranger-base-o1aoi §3).
//
// WHY A SECOND NAME RATHER THAN A SECOND REGION CALLED pane_capture. The
// pair is the evidence, and a reader of the record has to be able to tell
// which side of the keystrokes each half is from: for rangerhq-37c — a
// transient splash read as a seen screen — the type-time capture is the
// splash with its banner and the stall-time one is a composer with the text
// gone, and it is exactly that ORDER that says "eaten" where either alone
// says "idle" or "empty". Two regions under one name would also collide in
// the census's corpus export, which writes one file per region name
// (scripts/readings-census.py).
//
// Named apart from herdr's manifest vocabulary for PaneCaptureRegion's own
// reason: nothing in `etc/herdr/agent-detection/*.toml` answers to it.
const PaneCaptureAtPromptRegion = "pane_capture_at_prompt"

// withPaneCapture adds that capture to a D3 record's evidence: one
// `pane read --source detection` of the pane the refusal was about.
//
// ONE CALL, ON THE REFUSAL AND HOLD PATHS ONLY — the same bargain
// logPromptHandBack already strikes one paragraph down. D3 fires when herdr
// recognized nothing, which is rare and already costs a refused launch or a
// parked seat; one more herdr read there buys a record that IS a candidate
// `etc/herdr/agent-detection/testdata/<agent>/<state>-<what>.txt`, and
// capture → fixture → rule is how every real D3 so far was closed
// (rangerhq-7ia, rangerhq-9py0, ranger-base-n6s2u, ranger-base-3j8).
// `scripts/readings-census.py --export-corpus` writes every region to
// `regions/<id>/<region>.txt` already, so the capture lands on disk as a
// file `herdr agent explain --file` can be pointed at with no new surface.
//
// IT IS NOT A SECOND READER. Nothing decides on these bytes; they are
// written down. The redaction is the ceiling's, like every other region
// (AppendReading), and the classes taken ride on the record.
//
// THREE WAYS IT ADDS NOTHING, each silent:
//
//   - the REPORTED route (ADR 0061 D3.3, Reported != ""). There is no screen
//     herdr failed to recognize there — what there is, is another authority's
//     word about a pane herdr does not address — so a capture would be posse
//     reading a screen its own detection has declared out of scope. Skipped,
//     as today.
//   - no target. awaitTarget can fail before any pane is known.
//   - a read that errors or comes back empty. The log is best effort, and a
//     record that says what herdr saw and nothing else is the record there
//     has always been.
func (b *HerdrBackend) withPaneCapture(target string, ev ReadingEvidence) ReadingEvidence {
	return b.withPaneCaptureNamed(target, PaneCaptureRegion, ev)
}

// withPaneCaptureNamed is the same read under a region name the caller
// picks, and it is the whole of the mechanism — withPaneCapture above is
// this at PaneCaptureRegion.
//
// ONE IMPLEMENTATION, because the second caller (the settle gate's capture
// at type time, dispatch.go's gateReading) differs from the D3 one in the
// region name and in NOTHING else: the same herdr verb, the same three
// silent ways of adding nothing, the same ceiling on the way into the log.
// A second copy would be the same bargain struck twice and kept in sync by
// hand.
//
// The three silences are the ones the comment above names, and the REPORTED
// one carries over unchanged for the same reason it was written: a pane
// herdr does not address is a pane posse may not read a screen off, and
// that is as true the instant before a keystroke as it is after a refusal.
// A reported-route gate therefore carries its evidence with no capture on
// either side — which the record says by holding neither region.
func (b *HerdrBackend) withPaneCaptureNamed(target, region string, ev ReadingEvidence) ReadingEvidence {
	if b == nil || target == "" || ev.Reported != "" {
		return ev
	}
	text, err := b.H.PaneReadDetection(target)
	if err != nil || text == "" {
		return ev
	}
	// Appended, so herdr's own regions keep herdr's own order, and
	// Truncated is false because this one is not a preview of anything.
	ev.Regions = append(ev.Regions, ReadingRegion{
		Name:  region,
		Bytes: len(text),
		Text:  text,
	})
	return ev
}

// logUnrecognized writes the D3 record: a readiness gate that refused
// because herdr recognized nothing on the screen (ADR 0066 D1).
//
// IT IS THE CHEAPEST RECORD IN THE FILE and the richest, because the gate
// already has the whole of herdr's working in hand — every rule it tried and
// what each one's region held. That is exactly the block `WhatHerdrSaw`
// prints into the failure line (ranger-base-3j8), and this writes the same
// evidence somewhere a census can count it: three distinct screens produced
// one identical failure line on that bead, two of them needing opposite
// fixes, and the only reason anybody could tell them apart was a hand-launch
// and a `posse peek`.
//
// IT TAKES THE PANE AS WELL AS THE SESSION, for withPaneCapture above: the
// 243-character previews herdr's working is made of are the one thing in
// this record that cannot reproduce the screen (ranger-base-76gc4).
//
// Both callers refuse with NOTHING TYPED, so the consequence is a refusal in
// ADR 0066 D1's sense and not a hold: the launch did not happen.
func (b *HerdrBackend) logUnrecognized(session, target string, d AgentDetection, verdict string) {
	b.LogReading(session, Reading{
		Decision:    DecisionUnknownScreen,
		Verdict:     verdict,
		Consequence: ConsequenceRefusal,
		Rule:        RulePromptReady,
		Herdr:       b.withPaneCapture(target, ReadingEvidenceOf(d)),
	})
}
