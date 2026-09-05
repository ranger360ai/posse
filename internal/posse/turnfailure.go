package posse

// A runtime process being alive is not proof that a turn ran. Claude writes
// subscription/allotment refusals as synthetic assistant messages: herdr
// correctly sees the CLI settle back to idle, but no model ever handled the
// prompt. The transcript is the runtime-owned fact that distinguishes that
// terminal outcome from an ordinary settled turn.

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// TurnOutcome is what a runtime's own store says one settled turn did: the
// message a refusal carried (empty = the turn was answered normally), and how
// much of that turn had already RUN when the refusal landed.
//
// The second half is why this is a struct and not a string
// (ranger-base-qcu4c). An account does not only refuse before it serves: one
// of the seven refusals in this box's grok history landed 190,817 tokens and
// six model calls into a turn that had been running for a minute and a half,
// and a session that far in may well have edited files and commented on the
// bead. "no work ran" is exactly wrong about that turn, and the runtime's own
// record is the only thing that can tell the two apart.
type TurnOutcome struct {
	// Message is the refusal the runtime recorded, folded to one line.
	Message string
	// ModelCalls and OutputTokens are what the runtime's record says this
	// turn spent BEFORE the refusal — the units an operator decides in, not
	// the store's whole usage object, because the question the settle line
	// asks is only "is there work on the other side of this".
	//
	// They are read only beside a Message, and stay zero on an outcome that
	// carries none: nothing asks what a turn that was never refused spent,
	// and what it spent is already `posse cost`'s to read off the same
	// record. A reader filling them in on a healthy turn would be answering
	// a question this type is not the one to answer.
	ModelCalls   int
	OutputTokens int
}

// Worked reports that this turn had already run when it was refused, so a
// relaunch lands on top of work that may exist — a worktree with edits, a
// bead with comments.
//
// False carries a claim, so a reader owes it one, and BOTH registered readers
// now owe it off a measurement rather than off a shape: grok reads the `usage`
// object its store writes on every turn that ran anything (186/186, censused
// 2026-09-05), claude sums the `usage` objects on the assistant records ahead
// of the refusal in the same turn. Claude's used to be "by construction" —
// the reader stopped at the first answer, so a refusal it reported could have
// nothing behind it — and that construction was the ranger-base-4ldma defect,
// not a licence: 11 of this box's 13 in-turn refusals land after real work.
// A third adapter that cannot tell must say so in its own docstring, because
// the settle line reads a zero here as "nothing ran".
func (o TurnOutcome) Worked() bool { return o.ModelCalls > 0 || o.OutputTokens > 0 }

// TurnOutcomeReader reads the runtime-owned outcome of one settled turn: what
// the turn did (TurnOutcome) and whether an outcome was READ AT ALL. The two
// are separate because "nothing to report" and "nothing readable" are
// different facts, and only the first one may clear a failure marker.
//
// cwd is where the session's CLI actually ran, not where the bead lives:
// both readers built so far are looking for a per-session store a runtime
// keyed on its own working directory, and on a worktree dispatch that is the
// session's tree (Dispatcher.sessionCwd, ranger-base-f09bw).
type TurnOutcomeReader func(cwd, bead string, since time.Time) (out TurnOutcome, observed bool)

// TurnOutcomeClaudeTranscript reads claude's own JSONL transcript under the
// config dir's projects/ — `$CLAUDE_CONFIG_DIR`'s, else `~/.claude`'s, the
// locator being TranscriptFiles' (ranger-base-yqdov), so an operator who
// moves the config home does not silently make every turn unobserved.
// A runtime whose CLI writes that same shape declares
// `turn_outcome: claude-transcript` and is read by it; anything else needs a
// reader here first (ADR 0012 D4's adapter seam). grok's store is the second
// one built — TurnOutcomeGrokSessionStore, turnfailure_grok.go.
const TurnOutcomeClaudeTranscript = "claude-transcript"

// turnOutcomeReaders maps a runtime's declared turn_outcome: adapter to the
// code that implements it. The map is the whole registry: a name that is
// not a key here is refused at load, so a declaration can never promise a
// reading nothing performs.
//
// grok's store is read since ranger-base-fc8go: its refusal artifact was
// captured first and pinned as a fixture, which is the order ADR 0013 §1's
// promotion rule sets (docs/adr/0013-turn-outcome-refusal-probe.md).
//
// codex writes ~/.codex/sessions/*.jsonl (MEASURED, ranger-base-xaev) and is
// reachable in principle, but its refusal artifact has NOT been captured —
// its account was alive when the probe ran and the probe was told not to
// spend to force one — so it has no reader yet, and a reader built over a
// guessed shape is exactly what the promotion rule refuses. That is a
// declared blindness on codex, printed on the settle line, not a silence.
var turnOutcomeReaders = map[string]TurnOutcomeReader{
	TurnOutcomeClaudeTranscript: FindClaudeTurnOutcome,
	TurnOutcomeGrokSessionStore: FindGrokTurnOutcome,
}

// TurnOutcomeAdapters is every registered adapter name, sorted — what a
// refused declaration lists so the operator can see what is on offer.
func TurnOutcomeAdapters() []string {
	names := make([]string, 0, len(turnOutcomeReaders))
	for name := range turnOutcomeReaders {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// TurnOutcomeReaderFor is the reader a runtime's declaration resolves to,
// or nil when it declares none. nil is the honest answer for codex today:
// posse cannot tell an exhausted account from an ordinary settle there, and
// saying so is this seam's whole point (ranger-base-02zr).
func TurnOutcomeReaderFor(rt *Runtime) TurnOutcomeReader {
	if rt == nil {
		return nil
	}
	return turnOutcomeReaders[rt.TurnOutcomeAdapter]
}

// FindClaudeTurnOutcome finds the outcome of this dispatch prompt's turn.
// It scans only the Claude project directory for cwd, only files touched by
// this turn, and only assistant records after the matching bead prompt.
// User-authored bead text can quote the provider message without becoming a
// false positive. observed distinguishes a turn that answered (no message,
// true) from a transcript that is not readable yet (no message, false).
//
// It reads the WHOLE turn, not its first answer. Reading only the first was
// the reader's one blind spot and it was not a rare one: claude's allotment
// can run out in the middle of a turn, and the synthetic refusal then lands
// after real work. MEASURED over all 1755 claude transcripts on this box
// 2026-09-05 — of the 13 allotment refusals that fall inside a dispatch
// turn, 2 are the first answer (ranger-base-l9y, ranger-base-6ne, the two
// this reader was built from) and 11 are not, across 6 dispatched beads:
//
//	ranger-base-vtyst  33 model calls, 24740 output tokens before the refusal
//	ranger-base-frqmn  27 / 18417        ranger-base-felmj  27 / 20230
//	ranger-base-oujxl  25 / 11415        ranger-base-2dzsm  17 / 28070
//	ranger-base-pwtix  15 /  6250
//
// Each of those settled with the reader reporting a healthy turn, so the
// settle line printed an ordinary settle-without-close and a previous pass's
// turn-failure marker was cleared by it (ranger-base-4ldma). The artifact
// ADR 0013 §1's promotion rule asks for was already on disk here; nothing
// had to be spent to force one.
//
// The work fields are what the transcript's own usage objects say ran ahead
// of the refusal, deduped by message id and summed — the same rule
// ScanTranscript prices by, so "model calls" means one thing across this
// package. They stay zero on the two first-answer refusals because there is
// nothing before them, which is the flat "no work ran" arm still being right
// where it was right (ranger-base-qcu4c).
//
// cwd is the working directory the SESSION ran in, which on every worktree
// dispatch is the session's own tree and not the repo the bead lives in —
// claude keys ~/.claude/projects on the CLI's real cwd, and handing this the
// repo made it answer "nothing readable" for all of them (ranger-base-f09bw;
// the caller is Dispatcher.sessionCwd).
func FindClaudeTurnOutcome(cwd, bead string, since time.Time) (out TurnOutcome, observed bool) {
	for _, path := range claudeTranscripts(cwd) {
		st, err := os.Stat(path)
		if err != nil || st.ModTime().Before(since.Add(-time.Second)) {
			continue
		}
		if out, observed := scanClaudeTurnOutcome(path, bead, since); observed {
			return out, observed
		}
	}
	return TurnOutcome{}, false
}

// claudeProjectDir is the ~/.claude/projects directory claude keys a session
// on: the CLI's working directory with every character that is not a letter
// or a digit replaced by "-".
//
// MEASURED, not assumed (2026-09-05, ranger-base-f09bw). Every project
// directory on this box that carries a transcript was paired with the `cwd`
// its own first record names — 1349 pairs — and this rule reproduces all
// 1349 byte for byte. Replacing only "/", which is what this used to do,
// reproduces 43 of them: the corpus's non-alphanumerics are "/" (8121x),
// "." (1305x) and one " ", so every path under a dot directory missed, and
// ~/.posse/worktrees/... — where every dispatched session lives — is one.
// "_" appears in no cwd here and is folded by the same rule, because the
// rule is claude's own and not a list of the characters this box happened to
// exercise.
//
// If claude ever changes the encoding this goes blind LOUDLY rather than
// wrong: no directory matches, the reader returns (no outcome, false), and
// the settle line prints turnOutcomeClause's "looked and found none this
// pass".
func claudeProjectDir(cwd string) string {
	cleaned := filepath.ToSlash(filepath.Clean(cwd))
	var b strings.Builder
	b.Grow(len(cleaned))
	for _, r := range cleaned {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('-')
	}
	return b.String()
}

// claudeTranscripts is every transcript claude wrote for a session whose
// working directory was cwd: the .jsonl files in that ONE project directory,
// named exactly.
//
// Exactly, and not the substring match the cost locator uses, because a
// session rooted UNDER the tree mangles to a name that CONTAINS the tree's
// own — `/private/tmp/claude-501/<mangled tree>-<uuid>/scratchpad/...` is the
// shape claude hands every session, and on this box 6 worktree project names
// are a substring of 11 such directories (MEASURED 2026-09-05). What that
// costs is not an ordering race — the tree's own name sorts first, "U" before
// "p" — it is the rung below: on a pass where this session's own transcript
// is not readable YET, which is the (no outcome, false) this reader exists to
// keep distinct from a healthy turn, a substring match answers with the
// stranger instead, and a stranger's synthetic refusal stops the bead as
// "claude refused the first turn" about a turn nothing read. TranscriptFiles'
// substring filter is right for what it is — an operator's `posse cost
// --project` narrowing — and wrong for a locator that must name one
// session's own store.
//
// A root or a project directory that will not open is no candidates, which
// the caller reports as (no outcome, false): "cannot read" and "read a
// healthy turn" stay different facts (ADR 0018 §3).
func claudeTranscripts(cwd string) []string {
	if cwd == "" {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	dir := filepath.Join(home, ".claude", "projects", claudeProjectDir(cwd))
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	sort.Strings(out)
	return out
}

// scanClaudeTurnOutcome reads one transcript for this bead's turn.
//
// A turn is opened by the work prompt and closed only by the NEXT work
// prompt or by end of file. Every other user record inside it — a
// tool_result, a system reminder, a queued interjection — is the turn
// CONTINUING, and the previous reader's treating any user record as a turn
// boundary is why it never looked past the first answer.
//
// The first refusal in the turn wins. A later record cannot un-refuse it,
// and of the two ways to be wrong here — calling a refused turn healthy, or
// calling a recovered one refused — only the first one clears a marker and
// sends the operator away from a worktree with work in it.
func scanClaudeTurnOutcome(path, bead string, since time.Time) (out TurnOutcome, observed bool) {
	f, err := os.Open(path)
	if err != nil {
		return TurnOutcome{}, false
	}
	defer f.Close()

	inTurn, answered := false, false
	// message id -> the largest output_tokens that id reported. Claude writes
	// one model call as several records (thinking, text, tool_use), each
	// carrying the SAME growing usage object, so summing the records would
	// count one call many times over. Max-per-id then summed is what
	// ScanTranscript does for money and what Segment.Turns() counts.
	work := map[string]int{}
	r := bufio.NewReaderSize(f, 1<<20)
	for {
		line, readErr := r.ReadBytes('\n')
		if len(line) > 0 {
			var d struct {
				Type      string `json:"type"`
				Timestamp string `json:"timestamp"`
				Message   struct {
					ID      string          `json:"id"`
					Model   string          `json:"model"`
					Content json.RawMessage `json:"content"`
					Usage   *struct {
						Out int `json:"output_tokens"`
					} `json:"usage"`
				} `json:"message"`
			}
			if json.Unmarshal(line, &d) == nil {
				ts, _ := time.Parse(time.RFC3339Nano, d.Timestamp)
				switch d.Type {
				case "user":
					if m := workPromptRe.FindStringSubmatch(userText(d.Message.Content)); m != nil {
						inTurn = m[1] == bead && !ts.Before(since)
						if inTurn {
							// A re-prompt of this same bead opens a new turn:
							// what the last one spent is not this one's.
							answered, work = false, map[string]int{}
						}
					}
				case "assistant":
					if !inTurn || ts.Before(since) {
						break
					}
					text := assistantText(d.Message.Content)
					if d.Message.Model == syntheticModel {
						if claudeAllotmentLimit(text) {
							return TurnOutcome{
								Message:      strings.Join(strings.Fields(text), " "),
								ModelCalls:   len(work),
								OutputTokens: sumWork(work),
							}, true
						}
						// Claude Code's other locally-generated notices
						// (163 records on this box, 19 of them limits) are
						// not the model answering and not a model call:
						// they neither clear a marker nor count as work.
						break
					}
					// A real answer: positive evidence that a prior failure
					// marker can be cleared, and one model call's worth of
					// work if the refusal is still ahead of it.
					answered = true
					if d.Message.Usage != nil && d.Message.ID != "" {
						work[d.Message.ID] = max(work[d.Message.ID], d.Message.Usage.Out)
					}
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			// A half-read turn cannot say a refusal is absent, only that it
			// did not reach one.
			return TurnOutcome{}, false
		}
	}
	return TurnOutcome{}, answered
}

// syntheticModel is the model field Claude Code stamps on a record it wrote
// itself rather than one a model returned — the channel the allotment refusal
// arrives on.
const syntheticModel = "<synthetic>"

func sumWork(work map[string]int) int {
	total := 0
	for _, out := range work {
		total += out
	}
	return total
}

func assistantText(raw json.RawMessage) string {
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) != nil {
		return ""
	}
	var out []string
	for _, p := range parts {
		if p.Type == "text" && p.Text != "" {
			out = append(out, p.Text)
		}
	}
	return strings.Join(out, "\n")
}

func claudeAllotmentLimit(text string) bool {
	text = strings.ToLower(text)
	return strings.Contains(text, "you've reached your ") &&
		strings.Contains(text, " limit") &&
		(strings.Contains(text, "/usage-credits") || strings.Contains(text, "switch models with /model"))
}
