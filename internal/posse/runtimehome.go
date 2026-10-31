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
	"path/filepath"
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
// of the rule both directions share: the value, made ABSOLUTE, when the
// variable is in force, and whether it is in force at all.
//
// ABSOLUTE, and that is this function's whole second job. A VERBATIM value
// keeps the pin's guarantee only for an absolute one: posse resolves the
// value against ITS cwd and the seat resolves the same string against the
// SESSION dir, so a relative value makes posse seed one file and the seat
// open another — ranger-base-c33j6's shape exactly, surviving the fix built
// to end it (ranger-base-sczjy). MEASURED 2026-10-31 before this line, with
// CLAUDE_CONFIG_DIR=relcfg: the seed landed at <launcher cwd>/relcfg/.claude.json,
// the typed pin read `CLAUDE_CONFIG_DIR='relcfg' `, and the seat resolved
// <session dir>/relcfg/.claude.json, which was not there — a launch that
// completes, a pin that asserts true, a grant that is written, and no trust
// dialog suppressed.
//
// It is made absolute HERE, at the one read, and not refused at each
// consumer, because that is what makes "the seat reads what posse read" true
// by construction instead of true whenever the operator's shell happens to
// hold an absolute path. Both directions take the same string afterwards,
// and there is no second site to keep in step.
//
// An ALREADY-ABSOLUTE value is returned byte-identical — not Cleaned, not
// symlink-resolved. filepath.IsAbs gates the call for that reason and not
// for speed: `shQuote(v)` has to hand the seat the operator's own spelling
// (six hostile values measured through /bin/sh on the rendered fragment —
// spaces, an embedded quote, a tab, a semicolon, a command substitution and
// a newline — all arrive verbatim), and resolving symlinks would make posse
// name a path the operator never typed. IsAbs is also credentialDenyable's
// predicate (seatbelt.go), which closed this same class for a relative
// CODEX_HOME/GROK_HOME one rung down (ranger-base-o05yg); that predicate
// stays as the backstop for the shapes no variable can carry — "" and an
// unexpanded `~/…`.
//
// A failing os.Getwd leaves the value verbatim, which is the pre-fix answer
// and the only honest one: the cwd a relative path is relative to does not
// exist, so there is no absolute path to name. It is reachable essentially
// only where posse's own working directory has been removed under it.
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
	if v == "" {
		return "", false
	}
	if !filepath.IsAbs(v) {
		if abs, err := filepath.Abs(v); err == nil {
			v = abs
		}
	}
	return v, true
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
//     and the very directory credential.go walled. The value posse used is
//     ABSOLUTE whatever the operator typed (runtimeHomeOverride), which is
//     what makes that sentence true from a cwd the seat does not share.
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
