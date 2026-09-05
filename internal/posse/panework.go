package posse

// What a settled agent is still holding — ranger-base-htafy.
//
// THE INCIDENT (measured by the coordinating persona on 2026-09-02 at
// 08:08Z and 09:29Z, and again at 11:05Z). Two dispatched sessions finished their edits, started the repo's
// full suite behind a Monitor, and went idle to wait for the wake. Every
// store posse reads said "settled": herdr's `agent list` reports
// agent_status idle|done, `agent prompt --wait` returns the instant the
// turn ends, and neither carries a word about the work the agent is waiting
// ON. So the pass judged a settle-open, the pulse reported a `settled:`
// condition to the coordinator, and --resume typed a re-prompt into a
// session that was doing exactly what it had been told. The third
// occurrence added the other half: the re-prompt did not submit — it sat in
// the composer for four hours, and a pass that reads an idle pane as
// settled cannot tell that from an agent that stopped.
//
// WHAT THE STORES ACTUALLY CARRY (herdr 0.8.2, claude 2.1.258, measured
// 2026-09-02 over seven live panes — the bead's "measure what herdr's agent
// JSON carries and use it"):
//
//   - `herdr agent list`: agent, agent_status, cwd, pane/tab/workspace ids,
//     terminal title, revision, state_change_seq. NO task field at all.
//   - the detection manifest (claude.toml 2026.08.31.1) has a working rule
//     for `N MCP tasks still running` and one for `Waiting for N background
//     agents to finish`, and NONE for a live shell or monitor: an agent
//     idle behind its own suite run matches live_prompt_box, state idle.
//   - `herdr agent explain --json`: evaluated_rules, one entry per manifest
//     rule, each naming the SCREEN REGION that rule reads and previewing
//     what was in it — evaluated for every rule, not only the one that
//     matched. Two of those regions answer this bead:
//
//     after_last_horizontal_rule  claude's footer hint line
//     prompt_box_body             the composer: ❯ and whatever is typed
//
// Footers, verbatim from that measurement:
//
//	⏵⏵ auto mode on (shift+tab to cycle) · esc to interrupt · ← for agents
//	⏵⏵ auto mode on · 1 shell · esc to interrupt · ← for agents · ↓ to manage
//	⏵⏵ auto mode on · 1 shell, 1 monitor · esc to interrupt · ← for agents · ↓ to manage
//	⏵⏵ auto mode on · 1 monitor · esc to interrupt · ← for agents · ↓ to manage
//	⏵⏵ auto mode on (shift+tab to cycle) · ← for agents                (idle, nothing running)
//	⏵⏵ auto mode on · 1 shell · ← for agents · ↓ to manage              (IDLE, one still running)
//
// The `1 shell, 1 monitor` part is claude's own summary of its live
// background tasks. Its whole vocabulary is in the 2.1.258 binary — shells,
// monitors, teams, local agents, cloud sessions, remote and background
// dynamic workflows, Artifact comment monitors, MCP tasks, `N background
// tasks` when the kinds are mixed, and the three uncounted ones (dreaming,
// auto-mode scan, ultraplan).
//
// THAT IT SURVIVES THE SETTLE IS THE LAST LINE ABOVE, and it is a capture
// rather than an inference: a live pane at 14:38:10, `agent list` reporting
// agent_status idle, `agent explain` matching live_prompt_box — settled by
// every reading posse had — with `1 shell` still in its footer. The pane
// could not be put into that state on demand (it takes a session and a
// turn), so it was watched for; the renderer says the same thing, which is
// why it was worth watching for. The footer component (2.1.258) returns
// null if and only if the live-task list is empty, and nothing about the
// turn reaches it — while the surrounding parts ARE turn-conditional, which
// is why the idle capture keeps `auto mode on` and `← for agents` and drops
// `esc to interrupt`. If this ever stops holding the summary is simply
// absent, BackgroundWork answers "", and the settle is judged exactly as it
// was before this bead.
//
// PREVIEWS ARE TRUNCATED at 243 characters (measured: a 4949-byte
// whole_recent region previews at 243 chars). The footer measures 73-99
// bytes, so it arrives whole; a long typed prompt does not, so the question
// asked HERE is only whether the box is EMPTY. Whether the text in a
// non-empty box is a prompt somebody is about to send, or the line the pane
// last submitted showing through, is a second question and a different
// store: sentline.go and ranger-base-2hvtv. Truncation is why that
// comparison is prefix-shaped, and ComposerTruncated below is how it knows
// per reading rather than assuming.
//
// BOTH REGIONS ARE CLAUDE'S. Of the twenty detection manifests herdr ships
// (2026-09-02), `prompt_box_body` appears in claude.toml alone, and the task
// vocabulary below is claude's own — so a codex, grok or gemini pane
// previews neither, answers the empty hold, and is judged exactly as it was
// before this bead. That is a gap and not a bug: the reading is only ever
// allowed to say "this settle is not one", never to invent one.
//
// EVERY READING IS BEST-EFFORT AND FAILS TOWARDS TODAY. A herdr that cannot
// explain, a manifest whose rules no longer read those regions, a screen
// posse does not recognize: all of them answer the empty hold, and the
// settle is then judged exactly as it was before this bead. Ignorance is
// not evidence that an agent is waiting, and a hold invented out of one
// would keep a genuinely stuck bead claimed forever.

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	// footerRegion is claude's hint line — the region herdr's blocked-form
	// rules read, and the only one that carries the background-task summary.
	footerRegion = "after_last_horizontal_rule"

	// composerRegion is the prompt box: `❯` and what is typed after it.
	composerRegion = "prompt_box_body"

	// composerMark opens the prompt box body. It is REQUIRED before any of
	// this reads a composer: the same region previews a dialog's body when
	// one is drawn over the box (measured on a live pane holding an
	// AskUserQuestion form), and reading that as typed text would call every
	// blocked pane un-submitted.
	composerMark = "❯"

	// footerSep separates the footer's parts. claude joins them with a
	// middle dot; nothing else in the line uses one.
	footerSep = "·"
)

// backgroundTaskCount matches one counted entry of claude's task summary —
// the nouns are the binary's own (2.1.258), not a guess. A summary of
// several kinds is those entries joined with ", " ("1 shell, 1 monitor"),
// so the parts are checked one at a time and ALL of them must match: a
// footer part that is mostly prose does not become live work because a
// number and a noun appear somewhere inside it.
var backgroundTaskCount = regexp.MustCompile(`^[1-9]\d* (?:shells?|monitors?|teams?|local agents?|cloud sessions?|` +
	`remote dynamic workflows?|background dynamic workflows?|Artifact comment monitors?|MCP tasks?|background tasks?)$`)

// backgroundTaskWord is the rest of that vocabulary: the summaries claude
// draws with no count in front of them.
var backgroundTaskWord = map[string]bool{"dreaming": true, "auto-mode scan": true}

// backgroundTaskPrefix is the ultraplan family, whose summaries are a
// glyph, the word, and a phase ("ultraplan", "ultraplan ready",
// "ultraplan needs your input").
const backgroundTaskPrefix = "ultraplan"

// PaneHold is what a pane that herdr calls settled is still holding: work
// the agent is waiting on, and text sitting unsent in its composer. The
// zero value is "nothing, or nothing posse could see" — the two are one
// answer here on purpose (see the note above).
type PaneHold struct {
	// Work is claude's own summary of its live background tasks, verbatim
	// ("1 shell, 1 monitor"). "" = none visible.
	Work string
	// Typed is the text sitting in the prompt box. "" = the box is empty,
	// no composer was on screen to read, or the box is echoing a line this
	// pane has already submitted (Sent below).
	Typed string
	// Sent is the line the box is echoing back: text claude's own submit
	// log says was already sent in this pane's session, so it is NOT a
	// hold and Typed is empty beside it. "" = no echo was found, which is
	// also what an unreadable store answers (sentline.go).
	Sent string
}

// Waiting reports whether this pane's idle is a wait rather than a settle.
func (h PaneHold) Waiting() bool { return h.Work != "" || h.Typed != "" }

// Why says what the pane is holding, in one clause, for the line that
// refuses to treat it as settled. "" when it is holding nothing.
func (h PaneHold) Why() string {
	var parts []string
	if h.Work != "" {
		parts = append(parts, h.Work+" still running")
	}
	if h.Typed != "" {
		parts = append(parts, fmt.Sprintf("a prompt sitting UNSENT in its box (%q)", ellipsis(h.Typed, 60)))
	}
	return strings.Join(parts, " and ")
}

// ellipsis trims a screen reading down to something a report line can carry
// whole.
func ellipsis(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

// regionPreview returns herdr's preview of one screen region, taken from
// whichever rule reads it. Rules are evaluated whatever matched, so this
// does not depend on the state the pane is in — only on the manifest still
// having a rule that reads the region at all.
func (d AgentDetection) regionPreview(region string) string {
	for _, r := range d.EvaluatedRules {
		if r.Region == region && r.Evidence.RegionPreview != "" {
			return r.Evidence.RegionPreview
		}
	}
	return ""
}

// ComposerTruncated reports whether herdr cut the composer preview down —
// the 243-character cap the note above measures, asked per reading rather
// than assumed, because `agent explain` reports the region's real byte count
// beside the preview it cut. A region with no rule reading it answers false:
// there is no preview, so nothing was truncated.
func (d AgentDetection) ComposerTruncated() bool {
	for _, r := range d.EvaluatedRules {
		if r.Region == composerRegion && r.Evidence.RegionPreview != "" {
			return r.Evidence.RegionBytes > len(r.Evidence.RegionPreview)
		}
	}
	return false
}

// BackgroundWork returns claude's footer summary of the background tasks it
// is running, or "" when the footer shows none (or posse cannot see one).
func (d AgentDetection) BackgroundWork() string {
	for _, part := range strings.Split(d.regionPreview(footerRegion), footerSep) {
		if s := backgroundTaskSummary(part); s != "" {
			return s
		}
	}
	return ""
}

// backgroundTaskSummary answers whether one footer part IS a task summary,
// returning it trimmed of the glyph claude prefixes some of them with.
func backgroundTaskSummary(part string) string {
	s := strings.TrimSpace(part)
	// A leading glyph (the cloud-session and ultraplan summaries carry one)
	// is chrome, not vocabulary.
	if i := strings.IndexFunc(s, isSummaryWordStart); i > 0 {
		s = strings.TrimSpace(s[i:])
	}
	if s == "" {
		return ""
	}
	if backgroundTaskWord[s] || strings.HasPrefix(s, backgroundTaskPrefix) {
		return s
	}
	for _, one := range strings.Split(s, ", ") {
		if !backgroundTaskCount.MatchString(one) {
			return ""
		}
	}
	return s
}

// isSummaryWordStart marks where a task summary's own text begins: its
// first digit or ASCII letter. Everything before that is the glyph.
func isSummaryWordStart(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// Composer returns the text typed into the pane's prompt box, or "" when
// the box is empty or no box is on screen.
func (d AgentDetection) Composer() string {
	body := strings.TrimLeft(d.regionPreview(composerRegion), " \t\n")
	if !strings.HasPrefix(body, composerMark) {
		return ""
	}
	// U+00A0 is what claude draws between the mark and the text; TrimSpace
	// takes it, being a space in Unicode's book and Go's.
	return strings.TrimSpace(strings.TrimPrefix(body, composerMark))
}

// Hold reads both of them off one detection.
func (d AgentDetection) Hold() PaneHold {
	return PaneHold{Work: d.BackgroundWork(), Typed: d.Composer()}
}

// PaneHolding asks herdr what target's pane is holding. An explain that
// fails, or a herdr with no explain at all, answers the empty hold — see
// the note at the top of this file for why that is the only safe direction.
func (b *HerdrBackend) PaneHolding(target string) PaneHold {
	det, err := b.H.AgentExplain(target)
	if err != nil {
		return PaneHold{}
	}
	hold := det.Hold()
	if hold.Typed == "" {
		return hold
	}
	// ranger-base-2hvtv. A box previewing the line this pane LAST SUBMITTED
	// is echoing it, not holding it — claude's own submit log says which
	// (sentline.go, and the measurement behind it). Asked only when there is
	// text to ask about, so an empty composer still costs one `explain` and
	// nothing else, and answered "no" by every failure on the way: a pane
	// herdr will not name a claude session for, a store that will not open,
	// a session with no row in it.
	sess, err := b.H.PaneAgentSession(target)
	if err != nil {
		return hold
	}
	sent, ok := lastSubmitted(b.ClaudeHistory, sess)
	if !ok || !submittedEcho(hold.Typed, sent, det.ComposerTruncated()) {
		return hold
	}
	hold.Sent, hold.Typed = sent, ""
	return hold
}

// sessionHolding is PaneHolding by session name, for the callers that have
// one and no pane. A session posse cannot find a pane for is not one it can
// read a screen for, and it holds nothing.
//
// The meta's own `pane:` is asked first — the root pane on record from
// session creation, which is the pane herdr's prompt and explain address
// (ranger-base-5qe6) — because it is a file read. AgentTarget is the
// fallback and costs a full `Sessions()` plus an `agent list`; a reading
// this cheap is what lets the callers ask per settled holder rather than
// once a pass.
func (b *HerdrBackend) sessionHolding(session string) PaneHold {
	if m, ok := b.readMeta(session); ok && m.Pane != "" {
		return b.PaneHolding(m.Pane)
	}
	target, err := b.AgentTarget(session)
	if err != nil {
		return PaneHold{}
	}
	return b.PaneHolding(target)
}

// promptSubmitPoll paces the read-back below, and promptSubmitWait bounds
// it. The bound is a REPORTING threshold and nothing rests on it: the box
// not having cleared inside it produces a line naming what is still in the
// box, never an action, so choosing it too short costs a warning the
// operator can see is wrong and choosing it too long costs a second of a
// hand command. Nobody has measured how long claude's composer takes to
// clear after herdr submits it; the decisions that MUST be right about an
// unsent prompt are made later and off a screen read with no race in it —
// gather's settle, the --resume skip, the pulse's G2 row.
const (
	promptSubmitPoll = 250 * time.Millisecond
	promptSubmitWait = 2 * time.Second
)

// ConfirmSubmitted reads the composer back after a prompt was submitted and
// returns the text still sitting in it, or "" when the box cleared (or when
// posse could not see one).
//
// MEASURED 2026-09-02, three occurrences: `herdr agent prompt`
// returned agent_prompted, the text was typed, and the submit never
// happened — the prompt sat in the box for four hours on the last of them
// while every store reported the agent idle. A caller that types and walks
// away has no way to know; this is the read-back the bead asked for.
// `herdr agent send-keys <pane> enter` on the stale text did NOT submit it
// (measured on the same session); a fresh `posse prompt --now` did. So this
// reports and does not attempt a recovery keystroke that has been shown not
// to work.
func (b *HerdrBackend) ConfirmSubmitted(target string) string {
	deadline := time.Now().Add(promptSubmitWait)
	for {
		det, err := b.H.AgentExplain(target)
		if err != nil {
			// The same concession every reading in this file makes: a
			// diagnostic that will not answer is not evidence of a lost
			// keystroke.
			return ""
		}
		typed := det.Composer()
		if typed == "" {
			return ""
		}
		if !time.Now().Add(promptSubmitPoll).Before(deadline) {
			return typed
		}
		time.Sleep(promptSubmitPoll)
	}
}
