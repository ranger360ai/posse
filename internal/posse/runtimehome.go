package posse

// The RUNTIME-HOME variables — the ones that move a CLI's own state tree off
// `$HOME` — and the typed-line pin that makes the seat posse hires resolve
// them exactly as the launcher did (ADR 0077 D1, surface 1).
//
// WHY A PIN AT ALL. A seat is a child of the multiplexer SERVER, not of the
// posse process that asked for it, and that server's environment was frozen
// whenever some process happened to autostart it — hours earlier, from
// another seat. posse cannot read that layer, so no launch decision may rest
// on it: every variable posse reads from its OWN process to decide something
// about a seat is written onto that seat with the value posse used.
//
// MEASURED 2026-10-31 (ranger-base-c33j6; Rex CLI 0.0.1793377904, claude
// 2.1.316): posse wrote projects[<repo root>].hasTrustDialogAccepted into
// $CLAUDE_CONFIG_DIR/.claude.json — the file the LAUNCHER resolves — a probe
// of the seat read CLAUDE_CONFIG_DIR as UNSET, so the seat resolved
// ~/.claude.json instead, and it opened on the trust dialog with the grant
// sitting in the other file, true and timestamped. Both files are live on
// this box (1,376 projects and 696). Which one a seat reads is decided by
// one variable, and since this pin it is decided by posse.
//
// ON THE LINE AND NOT IN THE OVERLAY, which is the one thing that makes this
// different from D2's PATH (SeatBasePath, gates.go). herdr's `workspace
// create --env` can SET a variable and cannot unset one, and "unset" is half
// of this answer — see RuntimeHomePin. The line is also re-rendered by this
// same code on every relaunch, so there is no stale overlay to age out.

import (
	"os"
)

// The variable NAMES, one constant each. They are spelled here rather than
// at their reads because these names now travel in two directions: posse
// resolves its own view of a CLI's state root through them
// (ClaudeConfigDirIn in trust.go, codexHomeIn / grokHomeIn in
// interstitial.go, and credential.go's credentialDirNamed through the first
// of those), and posse TYPES them onto the seat. A name spelled twice is how
// the two directions come to disagree about which file is in play — the
// class ranger-base-x5f6p and ranger-base-wd4be already paid for inside
// posse alone, before a seat was in the picture.
const (
	ClaudeConfigDirVar = "CLAUDE_CONFIG_DIR"
	CodexHomeVar       = "CODEX_HOME"
	GrokHomeVar        = "GROK_HOME"
)

// runtimeHomeOverride is THE read of one of those variables, and the whole
// of the rule both directions share: the value VERBATIM when the variable is
// in force, and whether it is in force at all.
//
// In force means SET AND NON-EMPTY, which is posse's rule and not uniformly
// the runtimes' (ClaudeConfigDirIn's doc has the measured divergence: claude
// resolves its config DIRECTORY with `??`, so an empty value there is a
// cwd-relative path, and its config FILE with `||`, so the same empty value
// sends that file to the home — the runtime is not self-consistent about it).
// posse treats empty as unset everywhere, and the pin is what makes the SEAT
// agree: an empty value is never typed, it is unset (see RuntimeHomePin).
//
// It returns the value and the bit separately because the callers need
// different halves. The resolvers need the value to join a path onto; the
// pin needs the bit to choose an arm, and credentialDirNamed needs the bit
// alone — whether a VARIABLE named the directory, which is what decides the
// keychain item's spelling and is an environment property rather than a path
// property.
func runtimeHomeOverride(name string) (string, bool) {
	v := os.Getenv(name)
	return v, v != ""
}

// RuntimeHomePin is this runtime's home variable as posse's own environment
// answers it, rendered for the front of the typed line — the fragment that
// goes BESIDE GatePrefix (gates.go), after it, so the gate prefix's
// assignments are in force for whatever this runs.
//
// Two arms, and the second is the reason this rides the line:
//
//   - in force → `K='<the value posse used>' `, an ordinary POSIX prefix
//     assignment, so the seat resolves the very file SeedClaudeTrust wrote
//     and the very directory credential.go walled.
//   - not in force → `/usr/bin/env -u K `, which UNSETS it for the seat.
//     Never the empty value: an empty value is a third state both this and the runtime
//     read differently from unset (runtimeHomeOverride above), and typing it
//     would hand claude a cwd-relative config directory — a launch resolving
//     a path under whatever the pane's cwd happens to be, which is the shape
//     ranger-base-58b5 refuses everywhere else in posse.
//
// `/usr/bin/env` ABSOLUTE, for two reasons that agree. It is posse's own
// protocol and not a persona's typed command, so it is not the gates dir's
// to answer (ADR 0077 D3 is the same rule for posse's children); and the
// gates dir LEADS the PATH this very prefix just set, so the bare name would
// be resolved by whichever shim the persona's deny list happens to have
// rendered there. rexShellCommand (muxrex.go) spells it absolute for the
// same reason.
//
// A runtime with no HomeVar — bob, which has no home override to honour
// (bobHomeIn, interstitial.go: 25 distinct BOB_* names in the bundle and
// none of them a home), and every declared runtime — renders NOTHING. The
// pin's whole guarantee is "the seat reads the file posse read", and posse
// reads no home for a runtime it has no resolver for; typing a name nothing
// consumes would be a lever that reads as connected and is not (ADR 0017
// §1's p84 shape).
//
// IT COSTS BYTES ON A LINE WITH A CLIFF. A pane freshly created by the
// multiplexer is still in canonical mode, where MAX_CANON cuts the line at
// 1024 bytes (paneline.go, measured; Rex blocks are the same tty). This
// fragment is 25-60 bytes, and PaneLine already spills an over-long line to
// state/launch/<session>.sh and sources it — the expected state since
// 2026-09-01, when every persona's line rendered past the limit — so the
// cost is one more line over the cliff, never a line that is lost.
func RuntimeHomePin(rt *Runtime) string {
	if rt == nil || rt.HomeVar == "" {
		return ""
	}
	if v, ok := runtimeHomeOverride(rt.HomeVar); ok {
		return rt.HomeVar + "=" + shQuote(v) + " "
	}
	return "/usr/bin/env -u " + rt.HomeVar + " "
}
