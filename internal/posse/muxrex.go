package posse

// RexMux is Rex's occupant of the multiplexer slot (ADR 0076 §1): the ten
// verbs of mux.go over the rex CLI of rex.go, and nothing of its own. ADR
// 0075 §2 said the interface gets extracted when a second occupant is being
// built; this is that occupant.
//
// WHAT IS DIFFERENT FROM HERDR'S OCCUPANT, and all of it is §3 — on Rex the
// RUNTIME reports its own state and posse reads the report. There are no
// screen rules here, no posse-written hooks and no detection engine: State is
// one OSC 7501 record plus the block's process table, and everything a reader
// can act on comes out of those two answers. §4's herdr-only verbs are not
// here at all, which is the point of them being reached by a type assertion:
// the assertion fails on this type and the caller's "not available" arm runs.
//
// THE FOUR READINGS THAT ARE NOT A STATE. A record can be present and still
// say nothing about a runtime, and each case reads MuxUnknown — never
// MuxIdle, which is the whole of ADR 0076 §3's last line:
//
//	no record at all                     ADR 0076 §3
//	a record whose owner is the shell    ADR 0076 §3 ("not the pane's shell")
//	a record whose owner is not live     ranger-base-ch2rb (MEASURED)
//	a record with no owner at all        ADR 0076's spike (MEASURED)
//
// The third is the one that bites without a check: Rex drops only `working`
// and `blocked` when a process exits, so an `idle` record OUTLIVES its
// writer. MEASURED 2026-10-31 on this box — a program wrote `state=idle`,
// exited, and the record was still there with its dead pid while the block's
// foreground had fallen back to the shell. A State reader without the
// liveness check reads that dead seat as idle and settles a bead on it.

import (
	"strconv"
	"strings"
	"time"
)

// MuxRex is Rex's name in the slot — what a session record's `mux:` carries
// on a Rex instance (ADR 0076 §5). It lives here and not in mux.go for the
// reason that file's own comment gives: a name belongs to the occupant that
// answers for it.
const MuxRex = "rex"

// NewRexMux is the production occupant: this instance's rex binary, local
// server. The binary is NOT on PATH, so it is resolved from config or the
// env (App.RexBin) rather than looked up.
func NewRexMux(a *App) RexMux { return RexMux{R: Rex{Bin: a.RexBin()}} }

type RexMux struct {
	R Rex

	// Poll is how often Wait re-reads State; 0 = rexPollInterval. It is a
	// field for the reason Herdr.ControlTimeout is one: the pins cannot
	// afford the real number. A Wait pin has to watch two or three samples
	// go by, and at the production rate that is two or three SECONDS of
	// suite wall clock per pin — so the honest production value and a
	// testable one cannot be the same constant.
	Poll time.Duration
}

func (m RexMux) poll() time.Duration {
	if m.Poll > 0 {
		return m.Poll
	}
	return rexPollInterval
}

// rexPollInterval is how often Wait re-reads State.
//
// ADR 0076 chose polling over an event subscriber and left the rate to the
// occupant ("the rates are ASSUMED low enough … that an event subscriber
// would cost more than it saves"). This is that rate, sized against a
// measurement rather than picked: one State is two rex calls, MEASURED
// 2026-10-31 at 50-70 ms wall and ~20 ms of CPU each, so a one-second
// interval spends ~12% of the wall and ~4% of one core per WAITING seat. The
// deadlines on this verb are seconds to minutes (the shortest is Prompt's 5 s
// stall window, which still gets five samples), so a shorter interval buys
// granularity nothing here asks for and multiplies the only cost.
const rexPollInterval = time.Second

// ─── the ten verbs (ADR 0076 §2) ─────────────────────────────────────────────

// Create opens an unfocused seat: one session, one window, one terminal
// block running an INTERACTIVE SHELL, with the env in that shell's
// environment.
//
// WHY A SHELL AND NOT THE RUNTIME'S OWN LINE. §2's Create row reads
// "session.create with a terminal block {command, cwd}", written when Create
// and Start were one verb. They are not: posse types the runtime's line only
// after the meta that records the session is on disk, because a failed meta
// write must not leave a runtime running that posse has no record of
// (mustNotOrphan, rangerhq-v52t) — and RestartAgent types a line into a
// shell that settled hours ago, where there is no create to fold into. Rex's
// `command` is optional, so the seat opens as a shell exactly the way herdr's
// pane does and BOTH call sites keep working unchanged. That closes the
// ordering question ranger-base-csnzn flagged and ranger-base-q5gc2 left open
// ("on Rex the meta cannot be written between the create and the launch"):
// on this occupant it can, because the launch is Start's.
//
// WHY THE ENV RIDES THE BLOCK'S ARGV. Rex's blocks take no env (ADR 0076,
// MEASURED), so §2 says to put it in the argv as `env K=V …`. It goes in the
// SHELL's argv and not the runtime line's, and that is the difference that
// matters: on herdr the env is set on the workspace, so every line typed into
// that pane inherits it — including RestartAgent's, which re-reads the env
// sets but types only the line. Env prefixed per typed line would give a
// relaunched seat no credentials at all. The exposure is the argv, which is
// what Create's own doc already concedes ("the same known exposure `tmux -e`
// has") and what herdr's `workspace create --env K=V` already is.
//
// Rex's `initial_input` was the other way to reach the same shell, and it is
// rejected: it TYPES the text, so the values would be echoed onto the screen,
// kept in the shell's history, and captured into posse's own reading records
// (readingslog.go) — a durable leak the argv route does not have.
//
// Unfocused is ASSUMED and not measured: session.create takes no focus
// argument and the CLI does not attach as a UI client, so nothing here asks
// for focus. Whether the Rex APP follows a new session anyway needs eyes on
// it, which is ranger-base-phq39's live verify.
func (m RexMux) Create(label, cwd string, env []EnvVar) (Seat, error) {
	sid, bid, err := m.R.NewSession(label, cwd, rexShellCommand(env))
	if err != nil {
		// A session that exists while posse believes nothing does is the
		// orphan mustNotOrphan is about, so the occupant cleans up after
		// itself here rather than handing a caller half a seat: every
		// Mux.Create caller discards the Seat on an error (startPlanned
		// returns "" the same way herdr's does).
		if sid != "" {
			_ = m.R.KillSession(sid)
		}
		return Seat{}, err
	}
	return Seat{Session: sid, Pane: bid}, nil
}

// Start types a command line into the seat's shell and presses Enter —
// herdr's `pane run` built out of the two verbs Rex has. posse never wraps
// the process (mux.go's Start doc); on Rex nothing classifies what starts
// either, because the runtime reports its own state (§3).
func (m RexMux) Start(s Seat, line string) error {
	if err := m.R.Send(s.Session, s.Pane, line); err != nil {
		return err
	}
	return m.R.SendKey(s.Session, s.Pane, "enter")
}

// Close is `rex kill` (session.destroy).
func (m RexMux) Close(s Seat) error { return m.R.KillSession(s.Session) }

// Focus is `rex focus` (session.focus_block), which also makes the block's
// window the active one.
func (m RexMux) Focus(s Seat) error { return m.R.FocusBlock(s.Session, s.Pane) }

// List is `session.list`, joined to each session's blocks and to the
// program-status reading of every terminal block in it (§2's List row).
//
// IT FAILS ON ANY PART OF ITSELF, which is herdr's rule here for herdr's
// reason: a listing missing half of itself would report every seat as holding
// no runtime, and that is the one reading that must never be guessed (ADR
// 0013 §1).
//
// A SESSION HOLDING NO TERMINAL BLOCK IS NOT LISTED. MEASURED 2026-10-31,
// rex 0.0.1793377904: when a block's program exits the block closes, and the
// session STAYS in `session.list` with `windows: 0` — it does not die with
// its last block. §5's liveness rule is "dead when the recorded session id
// is absent from the occupant's listing", so a row for such a session would
// make every finished Rex seat immortal: nothing to type into, nothing to
// read, and a claim no prune could ever settle. Dropping it is not reading
// absence as death — the seat is demonstrably gone, which is the positive
// evidence ADR 0011 §2 asks for.
//
// THE COST, so nobody has to measure it twice: 1 + N + M rex calls for N
// sessions holding M terminal blocks, plus one more for each block that has
// reported anything (reading reads the process table only then), at 50-70 ms
// each (MEASURED 2026-10-31) — about 2.5 s for a twenty-seat instance whose
// seats are all reporting. herdr's List is two calls. Rex's API has no listing that carries a block's status, so the only
// cheaper shape is an event subscriber, which ADR 0076 priced and rejected.
func (m RexMux) List() ([]SeatInfo, error) {
	sess, err := m.R.Sessions()
	if err != nil {
		return nil, err
	}
	out := make([]SeatInfo, 0, len(sess))
	for _, s := range sess {
		blocks, err := m.R.Blocks(s.SessionID)
		if err != nil {
			return nil, err
		}
		terminals := 0
		var panes []SeatPane
		for _, b := range blocks {
			if b.Flavor != RexTerminalFlavor {
				continue
			}
			terminals++
			seat := Seat{Session: s.SessionID, Pane: b.BlockID}
			read, app, err := m.reading(seat)
			if err != nil {
				return nil, err
			}
			// Seen and not State, deliberately. SeatInfo.State's "" means
			// "the occupant detects no runtime there at all", and a block
			// whose only record was written by its own shell — or by a
			// process that has since exited — is exactly that. This is the
			// Rex form of the "unknown MEANS NONE" rule herdr's own List
			// applies to `agent_status: "unknown"`.
			if !read.Seen {
				continue
			}
			panes = append(panes, SeatPane{Pane: b.BlockID, Kind: app, State: read.State})
		}
		if terminals == 0 {
			continue
		}
		row := SeatInfo{
			Seat:  Seat{Session: s.SessionID},
			Label: s.Label,
			// Focused is false on every Rex row, and it is "not available"
			// rather than "no": `session.list` carries no focus and neither
			// does `client.list` (MEASURED — the server has no notion of
			// which session a UI is pointed at; `focus` acts WITHIN one).
			// Its one reader is the cockpit's marker, so the cost is a
			// marker nobody draws.
			Focused: false,
			Panes:   panes,
		}
		if len(row.Panes) > 0 {
			row.Pane, row.Kind, row.State = row.Panes[0].Pane, row.Panes[0].Kind, row.Panes[0].State
		}
		out = append(out, row)
	}
	return out, nil
}

// SendText is `rex send`: the keystrokes with no Enter behind them.
func (m RexMux) SendText(s Seat, text string) error {
	return m.R.Send(s.Session, s.Pane, text)
}

// SendKeys is `rex send-key`, in Rex's own key syntax (Rex.SendKey's doc says
// why the names are passed through rather than translated).
func (m RexMux) SendKeys(s Seat, keys ...string) error {
	return m.R.SendKey(s.Session, s.Pane, keys...)
}

// Read is `rex capture`: `--format vt` whole when ansi, else `--format text`
// tailed client-side to the last `lines` rows.
//
// ansi IGNORES lines, which is herdr's shape here and kept so that a caller
// cannot get a different snapshot depending on which occupant answered
// (HerdrMux.Read's doc carries the measurement behind it).
func (m RexMux) Read(s Seat, lines int, ansi bool) (string, error) {
	format := "text"
	if ansi {
		format = "vt"
	}
	text, err := m.R.Capture(s.Session, s.Pane, format)
	if err != nil {
		return "", err
	}
	text = strings.TrimRight(text, "\n\t ")
	if ansi || lines <= 0 {
		return text, nil
	}
	all := strings.Split(text, "\n")
	if len(all) > lines {
		all = all[len(all)-lines:]
	}
	return strings.Join(all, "\n"), nil
}

// State is the OSC 7501 record this block holds, judged against the block's
// process table (§3). The three readings that are not a state are in this
// file's head comment; rexReading applies them.
func (m RexMux) State(s Seat) (MuxReading, error) {
	read, _, err := m.reading(s)
	return read, err
}

// reading is State plus the record's `app`, which List needs for
// SeatPane.Kind and MuxReading has no field for. The two Kinds are different
// facts and share a name: MuxReading.Kind is the OSC `kind` a BLOCKED
// program is waiting for, and SeatInfo/SeatPane.Kind is which runtime is in
// the seat.
//
// TWO CALLS, AND ONLY WHEN THE SECOND CAN CHANGE THE ANSWER. No record is
// already MuxUnknown, so the process table is not read for a block that has
// never reported — which is every block holding a plain shell, and most of
// the blocks a List walks.
func (m RexMux) reading(s Seat) (MuxReading, string, error) {
	recs, err := m.R.ProgramStatus(s.Session, s.Pane)
	if err != nil {
		return MuxReading{}, "", err
	}
	rec := rexRootRecord(recs)
	if rec == nil {
		return MuxReading{State: MuxUnknown, Evidence: "no program_status record"}, "", nil
	}
	proc, err := m.R.Process(s.Session, s.Pane)
	if err != nil {
		return MuxReading{}, "", err
	}
	return rexReading(*rec, proc), rec.App, nil
}

// Wait polls State until the runtime reaches one of `until` or the deadline
// runs out (§2's Wait row: "State polled until a deadline").
//
// RUNNING OUT IS A REFUSAL AND NOT A READING, so it is an error carrying the
// `timeout` code — the same code herdr's own wait returns and the same one
// promptstall.go, relaunch.go and reportedagent.go already branch on, where
// it means "this says nothing about whether anything landed" (rangerhq-1z0).
// A Rex instance that answered a blown deadline with a reading would give
// those three sites a different meaning for the same silence.
//
// The first sample is taken before the first sleep, because `agent wait`
// returns on a state that is already there and a poller that slept first
// would answer a question nobody asked (promptstall.go's note on the race).
//
// An error ends the wait rather than being retried. That is herdr's shape —
// one call, one answer — and every caller of this verb already treats a
// failure as ignorance and keeps its claim, which is the cheap direction
// (promptstall.go's judgeStall).
//
// A TURN SHORTER THAN ONE POLL IS INVISIBLE, which is the one thing herdr's
// server-side observer has that this does not. The cost is bounded by what
// already happens to a stall: it becomes agent_prompt_stalled, judgeStall
// re-reads and checks the branch for commits, and the claim is kept. A real
// model turn is seconds to minutes against a one-second interval, so the
// shape this can actually miss is an instant refusal — which the screen then
// shows anyway.
func (m RexMux) Wait(s Seat, until []string, timeoutMS int) (MuxReading, error) {
	if timeoutMS <= 0 {
		// 0 means "the occupant's own default" (`posse wait --timeout 0`
		// documents it as the herdr default). herdr's lives in its server;
		// Rex has none, so this occupant names one — and names the ceiling
		// a rex call with no declared deadline already gets, rather than
		// coining a second number.
		timeoutMS = int(RexControlTimeout / time.Millisecond)
	}
	if len(until) == 0 {
		// Empty `until` is the same shape and gets the same treatment:
		// herdr passes no --until and lets its server decide, and a poller
		// with an empty set would instead watch the whole deadline out and
		// report a timeout over a seat that settled immediately. The set
		// named here is not invented — it is the settled triple every
		// caller of this verb in this repo actually passes (dispatch's two
		// settle legs, relaunch's landing, the probe's).
		until = []string{MuxIdle, MuxDone, MuxBlocked}
	}
	want := map[string]bool{}
	for _, u := range until {
		want[u] = true
	}
	deadline := time.Now().Add(time.Duration(timeoutMS) * time.Millisecond)
	for {
		read, err := m.State(s)
		if err != nil {
			return MuxReading{}, err
		}
		if want[read.State] {
			return read, nil
		}
		left := time.Until(deadline)
		if left <= 0 {
			return MuxReading{}, HerdrAPIError{Code: "timeout",
				Message: "no " + strings.Join(until, "|") + " state in " + s.Pane +
					" within " + (time.Duration(timeoutMS) * time.Millisecond).String() +
					" (last reading: " + rexStateForMessage(read) + ")"}
		}
		if p := m.poll(); left > p {
			left = p
		}
		time.Sleep(left)
	}
}

// Prompt types the text, presses Enter, and then mirrors herdr's stall
// contract by hand — SendText + Enter, then Wait(working|blocked) inside
// herdr's own five seconds, then the settle (§2's Prompt row).
//
// IT IS THE SAME ROUTE reportedagent.go takes for a pane herdr labels but
// does not detect, and for the same reason: there is no native prompt verb
// to defer to. So it carries that route's two properties verbatim rather
// than inventing its own:
//
//   - A MISS RETURNS agent_prompt_stalled, the code dispatch already reads as
//     "the submission was accepted, and what is missing is an observation"
//     (promptstall.go's taxonomy). A route that settled on silence, or that
//     coined a code of its own, would hand those three call sites a meaning
//     they do not have an arm for.
//   - A CALLER'S OWN SHORTER PATIENCE RETURNS timeout, not a stall. The two
//     mean opposite things and the shorter of the two is what actually
//     expired.
//
// The leading-'/' refusal is ADR 0060 D5, and it is this route's hazard
// wherever the route is: a prompt starting with '/' opens the CLI's command
// picker, the picker eats the Enter, and the text sits in the composer while
// this call reports success. Rex has even less to catch it with than herdr —
// no envelope, and no detection to read the box back with.
func (m RexMux) Prompt(s Seat, text string, wait bool, timeoutMS int) (MuxReading, error) {
	if strings.HasPrefix(strings.TrimLeft(text, " \t\r\n"), "/") {
		return MuxReading{}, Die("nothing was sent: on Rex posse types the prompt in (rex send + Enter), and a prompt " +
			"starting with '/' opens the CLI's command picker, which eats the Enter and leaves the text sitting in the " +
			"composer while this call reports success (ADR 0060 D5). Reword it without the leading '/', or drive the " +
			"picker by hand (posse peek)")
	}
	if err := m.R.Send(s.Session, s.Pane, text); err != nil {
		return MuxReading{}, err
	}
	if err := m.R.SendKey(s.Session, s.Pane, "enter"); err != nil {
		return MuxReading{}, err
	}
	if !wait {
		// Honest about the one thing this can claim: the keystrokes went
		// out. No state, because nothing has been observed since — which is
		// the "" every caller already reads out of a no-wait prompt result.
		return MuxReading{Evidence: "rex send + enter into " + s.Pane}, nil
	}
	start := time.Now()
	left := func() int {
		if timeoutMS <= 0 {
			return 0
		}
		if ms := timeoutMS - int(time.Since(start)/time.Millisecond); ms > 0 {
			return ms
		}
		return 1 // expired: let the wait below return its own typed timeout
	}
	stall, caller := herdrPromptStallMS, false
	if timeoutMS > 0 && timeoutMS < stall {
		stall, caller = timeoutMS, true
	}
	read, err := m.Wait(s, []string{MuxWorking, MuxBlocked}, stall)
	switch {
	case IsHerdrCode(err, "timeout") && !caller:
		return MuxReading{}, HerdrAPIError{Code: "agent_prompt_stalled",
			Message: "typed the prompt into " + s.Pane + " but no turn started within " +
				(time.Duration(stall) * time.Millisecond).String()}
	case err != nil:
		return MuxReading{}, err
	}
	// A turn that went straight to blocked has settled on the one state the
	// second wait would answer instantly anyway; return the reading that saw
	// it rather than taking it again.
	if read.State == MuxBlocked {
		return read, nil
	}
	return m.Wait(s, []string{MuxIdle, MuxDone, MuxBlocked}, left())
}

// ─── the launch-stage first-report check (ADR 0076 §3, D2) ───────────────────

// AwaitFirstReport waits up to `within` for this seat to hold a record that
// is a READING — one whose owner is the runtime and not the block's shell,
// and whose owner is still alive (reading's four not-a-state cases). It is
// ADR 0076 §3's launch rule, which is ADR 0013 §1's launch row applied to
// this occupant: no detection means no dispatch, and on Rex "detection" is
// the runtime's own report.
//
// IT IS NOT ONE OF §2'S TEN VERBS, and it is not on the Mux interface. It is
// §4's type assertion pointed the other way — a verb only THIS occupant has,
// reached through HerdrBackend.rexMux — because on herdr the same question
// has a different answer (the detection manifest, read off a file before
// anything is created) and an interface method would make every occupant
// pretend to answer a question only one of them has.
//
// THREE OUTCOMES, NOT TWO, and the third is why this does not return Wait's
// timeout error. `reported` false with a nil error is "the bound ran out with
// nothing to read", which is the launch refusal. A non-nil error is "rex
// could not be asked", which is ignorance and never a refusal — ADR 0013
// §1's unknown-is-not-no rule, and the direction that costs a kept session
// rather than a seat refused over a server hiccup. Wait's caller branches on
// an API code and so cannot tell those apart; this one must.
//
// The last reading rides back either way, so the caller's line can say what
// WAS there — "no program_status record" is a different sentence from a
// record the block's own shell wrote, and an operator reading a refusal needs
// to know which.
//
// The first sample is taken before the first sleep, for Wait's reason: a
// runtime that has already reported by the time this is called has answered
// the question, and a poller that slept first would spend a second proving it.
func (m RexMux) AwaitFirstReport(s Seat, within time.Duration) (MuxReading, bool, error) {
	deadline := time.Now().Add(within)
	for {
		read, err := m.State(s)
		if err != nil {
			return MuxReading{}, false, err
		}
		if read.Seen {
			return read, true, nil
		}
		left := time.Until(deadline)
		if left <= 0 {
			return read, false, nil
		}
		if p := m.poll(); left > p {
			left = p
		}
		time.Sleep(left)
	}
}

// ─── who is answering (ADR 0076 §5) ──────────────────────────────────────────

// Server is Rex's answer to the provenance triple every session record
// stamps.
//
// GEN IS EMPTY, and that is §5 read literally rather than an omission. herdr
// needs a generation because its allocator is max(live id)+1 recomputed at
// every server start, so a dead workspace id is reissued to a stranger. Rex's
// ids are UUIDs and are never reused, so a present id IS the seat the record
// meant and there is nothing for a fence to tell apart. The server does
// publish an `instance_id` that changes per process (MEASURED) and it is
// deliberately not stamped here: a fence posse does not need is a second
// reason a live seat can be withheld from its own name, which is the cost
// ranger-base-q5gc2's §3 measured on the label half of herdr's.
func (m RexMux) Server() MuxServer {
	return MuxServer{Name: MuxRex, Addr: m.R.Endpoint(), Gen: ""}
}

// ─── the record, and what makes it a reading ─────────────────────────────────

// rexShellCommand is the block's argv: the env, then an interactive shell.
//
// `/usr/bin/env` absolute rather than the bare name: rex runs a block's
// command THROUGH the user's login shell, so a bare `env` would resolve
// against whatever PATH that shell's startup files set, and the one program
// that must be found for the seat to exist at all should not depend on a
// profile. The shell is $SHELL — the shell the operator's own session runs,
// which is the one rex would have picked for a block with no command — and
// /bin/sh when the launcher has none, because a seat with a shell posse can
// name beats no seat.
//
// MEASURED 2026-10-31, rex 0.0.1793377904: `rex new L --cwd /tmp -- /usr/bin/env
// K=V zsh -i` gave a block whose `process.child` and `process.foreground` are
// both that interactive zsh, with K in its environment, the cwd honoured, and
// PATH from the login profile. child being the shell is what the "owner must
// not be the pane's shell" check compares against (RexProcess's doc).
func rexShellCommand(env []EnvVar) []string {
	cmd := make([]string, 0, len(env)+3)
	cmd = append(cmd, "/usr/bin/env")
	for _, v := range env {
		cmd = append(cmd, v.Key+"="+v.Value)
	}
	return append(cmd, rexShell(), "-i")
}

// rexShell is the shell the block runs, and it is realShell's answer — the
// same resolver the gate wrapper itself uses (gates.go), asked with no bin
// dir of its own because a hired seat's shell must not be ANY gates dir's.
//
// WHY NOT $SHELL DIRECTLY (ADR 0077 D2, surface 4). In a gated launcher
// $SHELL is that persona's gate shell: MEASURED 2026-10-31 in a crew seat,
// `<state>/gates/<the launching persona>/shell/zsh`. Handing that to `rex
// new` gave every Rex seat dispatched from a gated seat an interactive shell
// inside the LAUNCHER persona's wrapper — another PID's rules in front of
// every command the new seat would ever run, which is the same violation D2
// strips out of the seat's PATH. realShell refuses exactly this candidate
// (ranger-base-f0ay: a wrapper is installed under a shell's basename and
// stats like one, so every test the old code could have made was true of
// it), and nothing else in posse was asking the question twice.
//
// It costs the operator's own shell when that shell is neither bash nor zsh:
// realShell takes $SHELL only at those two basenames and otherwise searches
// zsh, bash, /bin/sh. That is a gain here rather than a loss — posse TYPES
// shell lines into this block, and GatePrefix's leading `PATH=…:"$PATH" cmd`
// is a POSIX prefix assignment that fish and friends do not parse, so a seat
// whose block ran the operator's fish could not have been gated at all.
func rexShell() string {
	real, _ := realShell("")
	return real
}

// rexRootRecord is the record posse reads: the ROOT one, whose id is empty —
// "the reports that give no ID", which is the program's own state. Rex sorts
// it first, and it is selected by its id rather than by position so that a
// listing that ever stops being sorted cannot quietly promote a child record.
//
// A child record is a task INSIDE the program (`build/test`), so a seat whose
// only records are children has reported no state of its own. That reads as
// no record, which is MuxUnknown — the same direction every other gap here
// takes.
func rexRootRecord(recs []RexStatusRecord) *RexStatusRecord {
	for i := range recs {
		if recs[i].ID == "" {
			return &recs[i]
		}
	}
	return nil
}

// rexReading turns one root record plus the block's process table into the
// slot's reading. The three not-a-state cases are this file's head comment;
// this is where they are applied, in the order a reader would ask them.
func rexReading(rec RexStatusRecord, proc RexProcess) MuxReading {
	read := MuxReading{Kind: rec.Kind, Evidence: rexEvidence(rec)}
	if rec.Owner == nil {
		// Rex leaves `owner` out when it "couldn't tell" the foreground
		// process, and the hook route measured one way that happens: a
		// report written while the SHELL was in the foreground lands with
		// no owner at all (ADR 0076's spike). Either way there is nothing
		// that says a runtime wrote this, so there is no reading in it.
		read.State = MuxUnknown
		read.Evidence += "; no owner, so nothing says a runtime wrote it"
		return read
	}
	read.Owner = rexOwnerString(*rec.Owner)
	switch {
	case rexOwnerIsShell(*rec.Owner, proc):
		// ADR 0076 §3's "owner must not be the pane's shell". MEASURED: a
		// report written from the shell's own prompt carries the shell as
		// its owner, so this is what tells a runtime's report from the
		// seat's own plumbing.
		read.State = MuxUnknown
		read.Evidence += "; its owner is the block's own shell"
	case !rexOwnerLive(*rec.Owner, proc):
		read.State = MuxUnknown
		read.Evidence += "; its owner is not a live process in this block — the record outlived its writer"
	default:
		read.Seen = true
		read.State = rexMapState(rec.State)
	}
	return read
}

// rexMapState maps an OSC 7501 state onto the slot's vocabulary (ADR 0076
// §2): the four posse already uses map to themselves, and `error` maps to
// MuxDone — the turn ended, and posse's turn-failure readers judge the screen
// as they do today.
//
// A STATE THIS DOES NOT KNOW IS MuxUnknown, which is a deliberate divergence
// from the protocol's own client advice ("Treat a value you don't know as
// 'idle'"). That advice is right for a UI badge and wrong here: posse's
// readers branch on a settled state to judge a turn and close a bead, so
// calling an unrecognized state idle would settle work on a guess — ADR 0013
// §1's unknown-is-not-no, in the direction that costs a kept claim rather
// than a bead closed on a turn nobody saw.
func rexMapState(state string) string {
	switch state {
	case "idle":
		return MuxIdle
	case "working":
		return MuxWorking
	case "done":
		return MuxDone
	case "blocked":
		return MuxBlocked
	case "error":
		return MuxDone
	}
	return MuxUnknown
}

// rexSameProc is Rex's own comparison rule: pid AND name, "because the system
// reuses process IDs" (the program_status schema says so in those words).
func rexSameProc(o RexOwner, p *RexProc) bool {
	return p != nil && p.PID == o.PID && p.Name == o.Name
}

// rexOwnerIsShell is whether the record's owner is the block's own shell —
// `process.child`, the process the terminal started, which on a seat this
// occupant created is the interactive shell rexShellCommand named.
//
// A block whose child cannot be read answers false, so the liveness check
// below is what judges the record. That is the unknown direction again: "we
// could not see the shell" must not become "this was the shell".
func rexOwnerIsShell(o RexOwner, proc RexProcess) bool {
	return rexSameProc(o, proc.Child)
}

// rexOwnerLive is whether the owner is still one of the processes this block
// holds: the foreground, the child, or one of the ancestors between them.
//
// ALL THREE, because a runtime's own report can be any of them by the time it
// is read. MEASURED 2026-10-31: a program the shell started is `foreground`
// with the shell as `child` and `[child]` as `ancestors`; a shell at its
// prompt is both foreground and child at once; and `ancestors` is left out
// entirely when foreground IS child. A check against the foreground alone
// would read a runtime that has a foreground child of its own as dead.
func rexOwnerLive(o RexOwner, proc RexProcess) bool {
	if rexSameProc(o, proc.Foreground) || rexSameProc(o, proc.Child) {
		return true
	}
	for i := range proc.Ancestors {
		if rexSameProc(o, &proc.Ancestors[i]) {
			return true
		}
	}
	return false
}

// rexOwnerString renders an owner for MuxReading.Owner, which on Rex is "the
// OSC 7501 record's owner, pid and name" (mux.go). It is a reader's string
// and nothing branches on it — the branches above compare the struct.
func rexOwnerString(o RexOwner) string {
	return o.Name + "(" + strconv.Itoa(o.PID) + ")"
}

// rexEvidence is the one-line "what said so" a reader wants beside a state:
// the record as Rex gave it. Free text, like herdr's — the §4 sites that need
// a multiplexer's full working are herdr's alone, and on Rex there is no
// working to show, only a report.
func rexEvidence(rec RexStatusRecord) string {
	parts := []string{"program_status state=" + rec.State}
	if rec.Kind != "" {
		parts = append(parts, "kind="+rec.Kind)
	}
	if app := rec.App; app != "" {
		parts = append(parts, "app="+app)
	}
	if rec.Msg != "" {
		parts = append(parts, "msg="+rec.Msg)
	}
	if rec.UpdatedAt != "" {
		parts = append(parts, "updated_at="+rec.UpdatedAt)
	}
	return strings.Join(parts, " ")
}

// rexStateForMessage names the last reading in a timeout message. A reading
// with no state at all is a real answer here (MuxReading's doc), so it is
// spelled rather than left as an empty gap in a sentence.
func rexStateForMessage(read MuxReading) string {
	if read.State == "" {
		return "no state"
	}
	return read.State
}

var _ Mux = RexMux{}
