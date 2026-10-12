package posse

// ADR 0072 D2 — the hidden `posse gates bd-line -- <bd argv>` subcommand.
// It finds the store the way bd itself will, asks ProjectBdWrite, and
// either exits 0 silently or refuses with the remedy and a refusals.log
// line — never bd's own `token too long`, which this write has not
// happened yet to produce. The bd shim's own arm no longer calls this
// directly (ADR 0073 D2's `gates bd-write` calls the same judge under its
// write lock instead, bdwriteshim.go); this stays wired as its own
// subcommand for any future caller that wants the judge unlocked, and the
// helpers below (bdLineGateRefusal, appendBdLineGateLog) are what
// RunGatesBdWrite shares with it.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// bdLineCensusHome is "the way bd does": $BEADS_DIR when the session set
// one (a persona's own session always does, ADR 0055), else beadsHome(cwd)
// — the same cwd-plus-one-redirect-hop bd falls back to with none set. This
// reads the environment the shim's child inherited, not a directory the
// caller named, because unlike Bd's in-process callers (which always know
// their own dir) this process has no other way to be told.
func bdLineCensusHome() string {
	if dir := os.Getenv("BEADS_DIR"); dir != "" {
		return dir
	}
	return beadsHome(".")
}

// bdLineGateRefusal is ADR 0072 D2's refusal text — close kin to
// BdLineWouldCrossError.Error() (D3, in-process) but naming the census file
// by path, the way a reader sitting at a shim's refusal needs it, and with
// no "(deny: ...)" clause: this is bd's own scanner limit, not one of the
// PID's deny rules, so posse_refuse's usual framing does not fit.
func bdLineGateRefusal(census string, proj Projection) (line, remedy string) {
	if proj.ID == "" {
		line = fmt.Sprintf("refused by posse gate: this create would write a %d-byte line in %s; bd 0.50.3 cannot read a line over %d, and it cannot delete a comment, so the store would stay unreadable (ADR 0072)",
			proj.After, census, BdLineLimit)
		remedy = "the way through: put the bulk in a committed file and comment a pointer to it instead."
		return
	}
	line = fmt.Sprintf("refused by posse gate: this write would make %s's line in %s %d bytes; bd 0.50.3 cannot read a line over %d, and it cannot delete a comment, so the store would stay unreadable (ADR 0072)",
		proj.ID, census, proj.After, BdLineLimit)
	remedy = fmt.Sprintf("the way through: put the bulk in a committed file or a new bead (`bd create … --deps discovered-from:%s`) and comment the pointer.", proj.ID)
	return
}

// appendBdLineGateLog appends ADR 0072 D2's refusals.log line — silently a
// no-op when $RHQ_GATE_LOG is unset or unwritable, the same posture
// posse_refuse's own `>> "$RHQ_GATE_LOG" 2>/dev/null` takes: a refusal
// already reached stderr, and a log write that cannot land is not a second
// reason to fail the call.
func appendBdLineGateLog(proj Projection) {
	log := os.Getenv("RHQ_GATE_LOG")
	if log == "" {
		return
	}
	f, err := os.OpenFile(log, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	persona := os.Getenv(EnvPersona)
	if persona == "" {
		persona = "?"
	}
	id := proj.ID
	if id == "" {
		id = "(create)"
	}
	stamp := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	fmt.Fprintf(f, "%s bd line cliff [bd shim] %s %dB session %s\n", stamp, id, proj.After, persona)
}

// RunGatesBdLine is `posse gates bd-line -- <bd argv>`, the judge on its
// own: ADR 0073 D2's `gates bd-write` (bdwriteshim.go) now calls the same
// logic in-process, under its write lock, instead of shelling out to this
// subcommand — but the subcommand itself stays, for a caller that wants
// the unlocked judge. refused=true means the write must not run; a caller
// that execs this as the bd shim's old arm did would exit with this
// call's own exit code and never exec bd.
//
// Every "cannot judge" path returns refused=false, err=nil: an absent
// census (no store, or a store this persona's session has not been pointed
// at), an id ProjectBdWrite could not resolve to an exact census line, or
// text arriving on bd's stdin all mean "let the real call proceed" — the
// same fail-open posture refuseIfCrossing takes in-process (bdline.go), for
// the same reason: a wall that blocks a write because the judge could not
// read the census is worse than the gap ADR 0072 D4's read-time report
// already covers.
func RunGatesBdLine(args []string, errOut io.Writer) (refused bool, err error) {
	bdArgv, ok := bdLineSplitArgv(args)
	if !ok {
		return false, Die("usage: posse gates bd-line -- <bd argv>")
	}
	census := filepath.Join(bdLineCensusHome(), beadsJSONL)
	if _, serr := os.Stat(census); serr != nil {
		return false, nil
	}
	proj, pok, perr := ProjectBdWrite(census, bdArgv)
	if perr != nil || !pok {
		return false, nil
	}
	if proj.After <= BdLineLimit-BdLineHeadroom {
		return false, nil
	}
	line, remedy := bdLineGateRefusal(census, proj)
	fmt.Fprintln(errOut, line)
	fmt.Fprintln(errOut, remedy)
	appendBdLineGateLog(proj)
	return true, nil
}

// bdLineSplitArgv finds the `--` this subcommand's own usage requires and
// returns everything after it — the bd argv the shim saw, untouched.
func bdLineSplitArgv(args []string) ([]string, bool) {
	for i, a := range args {
		if a == "--" {
			return args[i+1:], true
		}
	}
	return nil, false
}
