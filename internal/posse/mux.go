package posse

// The multiplexer slot (ADR 0075 §2, ADR 0076 §1): the verbs posse drives a
// seat with, and nothing else.
//
// THE BOUNDARY IS THE CLIENT, NOT THE BACKEND. HerdrBackend's twenty-two
// exported methods are posse's own logic — the meta files, launch planning,
// kill-and-land, crew marks, bead notes, relaunch — and they reach the
// multiplexer only through the verbs below. An interface extracted from
// HerdrBackend instead would make every occupant carry its own copy of three
// thousand lines of that (ADR 0076, "Alternatives rejected"). So this file
// holds the seam, muxherdr.go holds herdr's occupant of it, and the backend
// keeps everything it already owned.
//
// WHAT IS NOT HERE. Verbs only herdr has stay herdr's, reached by a type
// assertion on the occupant (HerdrBackend.HerdrMux) and never on this
// interface: the `agent explain` detection with its evaluated rules, the
// readings-log detection capture, the runtime's own session id, the
// detection preflight and the report-agent route (ADR 0076 §4). On an
// occupant that has none of them the assertion fails, and a failed assertion
// is "not available" — never "no", which is ADR 0013's unknown-is-not-no
// rule.

// Seat is one addressable place a runtime runs: an opaque handle the
// occupant fills, posse passes back, and the meta file stores.
//
// Both halves are the occupant's own ids and posse reads neither. On herdr
// they are a workspace id and a pane id; on Rex a session UUID and a block
// id. A verb takes whichever half it addresses — Close and Focus the
// session, the typing and reading verbs the pane — so a caller that holds
// only one half (dispatch holds a pane it resolved through AgentTarget)
// builds a Seat carrying only that half, and the verbs it then calls are the
// ones that need it.
type Seat struct {
	// Session is the server-side container: herdr's workspace, Rex's
	// session.
	Session string
	// Pane is the place inside it a runtime has the keyboard: herdr's
	// pane, Rex's block. "" for a seat posse created but has not yet
	// resolved a runtime in.
	Pane string
}

// SeatInfo is one row of List(): a seat the occupant is holding, and what it
// says about the runtime in it.
//
// It is one row per SESSION rather than one per pane, because that is the
// grain every caller works at — a posse session is a workspace with one
// recorded pane — and Panes carries the rest for the one caller that needs
// them all (HerdrBackend.AgentTarget, which prefers the recorded pane and
// falls back to the first).
type SeatInfo struct {
	Seat
	// Label is the name the occupant shows the seat under — for posse's own
	// seats, App.WorkspaceLabel(name). Never the session name: the label is
	// what the occupant's one shared list displays and the name is what
	// posse addresses (rangerhq-ouf9).
	Label string
	// Focused is whether the occupant's own UI is pointed at this seat.
	Focused bool
	// State is the runtime's state in this seat, in the vocabulary below,
	// and "" when the occupant detects no runtime there at all.
	//
	// The "" is load-bearing and it is the occupant's to apply, not the
	// caller's: herdr answers `agent_status: "unknown"` for a workspace
	// holding nothing but a shell, and a listing that printed that would be
	// claiming a reading about a runtime that is not there.
	State string
	// Kind is the runtime the occupant recognizes in Pane — claude, codex,
	// … — and "" when it recognizes none.
	Kind string
	// Panes is every pane of this seat the occupant detects a runtime in,
	// in the occupant's own order. Empty when it detects none, which is the
	// same fact State == "" reports.
	Panes []SeatPane
}

// SeatPane is one detected runtime inside a seat.
type SeatPane struct {
	Pane  string
	Kind  string
	State string
}

// The state vocabulary (ADR 0076 §2). It is the one posse already uses, and
// these are declared so a new occupant has a list to map onto rather than a
// set of literals to find by grep. Existing comparisons against the literals
// are left alone: these are those literals.
//
// On Rex, `error` maps to MuxDone — the turn ended, and posse's turn-failure
// readers judge the screen as they do today — and no record maps to
// MuxUnknown, never to MuxIdle (ADR 0076 §3).
const (
	MuxWorking = "working"
	MuxBlocked = "blocked"
	MuxIdle    = "idle"
	MuxDone    = "done"
	MuxUnknown = "unknown"
)

// MuxHerdr is herdr's name in the slot, and the name a session record means
// when it carries no `mux:` key at all (ADR 0076 §5). The absent key is the
// default on purpose: every meta file on this box was written before the key
// existed, and all of them are herdr's.
//
// There is no constant for the second occupant here. A name belongs to the
// occupant that answers for it (HerdrMux.Server), so Rex's lands with Rex's
// own file rather than as a string this package already knows.
const MuxHerdr = "herdr"

// MuxServer is the provenance of a seat: which occupant holds it, which
// server of that occupant, and which generation of that server issued the
// ids. It is the triple a session record carries — `mux:`, `socket:`,
// `gen:` — and the occupant is the only thing that can answer it.
//
// IT IS NOT ONE OF ADR 0076 §2'S VERBS, and it is here because §5 is not
// satisfiable without it. §5 gives the meta file a Mux key, puts the Rex
// server address in `socket:` and leaves `gen:` empty on Rex — three facts
// about the occupant that the posse-owned record has to write down and the
// prune has to compare against. posse read them as `SocketID()` and
// `ServerGen()`, two package functions that resolve a HERDR socket path, so
// a record written on a Rex instance would have carried a herdr socket it
// was never created against, and the prune would have compared the wrong
// pair forever. Asking the occupant is the only answer that stays true when
// the occupant changes (docs/notes.d/ranger-base-q5gc2.md).
type MuxServer struct {
	// Name is the occupant's own name — MuxHerdr, or another occupant's.
	// It is what `mux:` records, and an empty `mux:` reads back as
	// MuxHerdr rather than as "".
	Name string
	// Addr is the server this occupant is talking to, in whatever spelling
	// identifies one: herdr's resolved api socket path, Rex's server
	// address. "" is "this pass cannot name its server", which is never
	// read as a match (cannotAnswerFor).
	Addr string
	// Gen is the server GENERATION that issued the seat ids — the fence
	// that tells a reused id apart from the one a record meant. "" is "not
	// recorded, or this occupant has no such thing", and it is never read
	// as a match either.
	//
	// herdr needs it: its allocator is max(live id)+1, recomputed at every
	// server process start, so a dead workspace id is reissued to a
	// stranger (ServerGen, notOurWorkspace). Rex's ids are UUIDs and are
	// never reused, so Rex answers "" and §5 says so — which is also why a
	// non-herdr seat needs no identity fence at all.
	Gen string
}

// MuxReading is what an occupant says about the runtime in one seat.
//
// THE EMPTY STATE IS AN ANSWER AND NOT AN ERROR. An occupant that was asked,
// answered, and named no state at all reads as State == "" — promptstall.go's
// stallUnreadable, which is "herdr answered the wait with no state at all"
// and is kept apart from both a timeout and a refusal. An occupant that
// could not be asked returns an error instead.
type MuxReading struct {
	// State is the vocabulary above, or "" (see the note on the type).
	State string
	// Kind qualifies a block: herdr has none to give, and on Rex it is the
	// OSC 7501 `kind` — permission | question | auth — which that protocol
	// sets on `blocked` only.
	Kind string
	// Owner is who the occupant says holds the keyboard. On herdr it is the
	// agent LABEL a pane carries when some outside authority stated it
	// through `pane report-agent` (ADR 0061), and "" for a pane herdr
	// detected itself. On Rex it is the OSC 7501 record's owner, pid and
	// name, which must not be the block's own shell (ADR 0076 §3).
	Owner string
	// Evidence is free text: what the occupant answered, for a reader
	// rather than for a branch. On herdr's wait and prompt verbs it is the
	// API envelope verbatim, which is what `posse wait` prints.
	Evidence string
	// Seen is POSITIVE evidence that the runtime has the keyboard, and the
	// one field a caller may act on beside State. It is never derived from
	// State: every settled state and `working` itself are guesses on a seat
	// nothing has recognized, which is the whole of ADR 0013 §1.
	//
	// The occupants answer it from different facts and that is the point:
	// herdr from its own detection (a matched rule, chrome it can see, or
	// another authority's word about a live pane — AgentDetection.Seen),
	// Rex from a program-status record whose owner is not the block's shell
	// (ADR 0076 §3 calls that "the Rex form of herdr's Seen()"). ADR 0076
	// §2's field list does not name it; §3 requires it, so it is here.
	Seen bool
}

// Mux is the multiplexer slot's occupant: ADR 0076 §2's ten verbs.
//
// Start is the second half of the Create row's herdr column (`workspace
// create` + `pane run`) and is on its own rather than folded into Create,
// because posse calls it at two sites where Create cannot stand:
//
//   - a launch types the runtime's line only AFTER the meta that records the
//     session is on disk (HerdrBackend.startPlanned). Folding the line into
//     Create would make a failed meta write leave a runtime running that
//     posse has no record of, which is the orphan rangerhq-v52t and
//     mustNotOrphan exist to prevent.
//   - RestartAgent types a line into a pane whose shell has been settled for
//     hours. There is no create there to fold into.
//
// An occupant that cannot open a seat without its command — Rex, whose
// blocks take `{command, cwd}` — has to answer Start by other means and say
// so; that is the Rex occupant's bead, not this interface's problem to
// pre-solve.
//
// TIMEOUTS ARE MILLISECONDS, not a time.Duration, because they are herdr's
// own contract: `agent wait --timeout <ms>` returns inside that window or
// returns a timeout envelope, and Herdr.callDeadline reads the number back
// out of the argv to size the child deadline it is wrapped in. Converting at
// both ends would buy nothing and could only lose a factor of a million.
type Mux interface {
	// Create opens an unfocused seat. Env values ride the argv, the same
	// known exposure `tmux -e` has.
	Create(label, cwd string, env []EnvVar) (Seat, error)
	// Start types a command line into the seat's interactive shell. posse
	// never wraps the process: the occupant's own detection classifies
	// whatever starts.
	Start(s Seat, line string) error
	// Close ends the seat.
	Close(s Seat) error
	// List is every seat this occupant holds.
	List() ([]SeatInfo, error)
	// Focus points the occupant's own UI at the seat.
	Focus(s Seat) error
	// SendText types text with no submit. A prompt is Prompt's job; this is
	// for the screens that hold the keyboard before a composer does.
	SendText(s Seat, text string) error
	// SendKeys presses named keys ("esc", "enter", …), not text.
	SendKeys(s Seat, keys ...string) error
	// Read is the seat's terminal text, tailed to the last `lines` rows
	// (0 = all of it), with the escape sequences left in when ansi.
	Read(s Seat, lines int, ansi bool) (string, error)
	// State is the occupant's reading of the runtime in this seat, taken
	// now.
	State(s Seat) (MuxReading, error)
	// Wait blocks until the runtime reaches one of `until` or timeoutMS
	// runs out. Running out is the occupant's own refusal to report — on
	// herdr a `timeout` API code — and not a reading.
	Wait(s Seat, until []string, timeoutMS int) (MuxReading, error)
	// Prompt submits text to the runtime. wait blocks until the first
	// settled idle|done|blocked state.
	Prompt(s Seat, text string, wait bool, timeoutMS int) (MuxReading, error)

	// Server is not a verb: it is who is answering them (see MuxServer).
	// Every session record posse writes stamps this triple, and every
	// prune compares the record's against the live one before it may act
	// on the seat (ADR 0076 §5).
	Server() MuxServer
}
