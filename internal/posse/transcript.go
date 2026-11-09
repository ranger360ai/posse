package posse

// The TRANSCRIPT CEILING — ADR 0050's third channel, made a per-repo switch
// on the launch line (operator ruling 2026-11-08, ranger-base-ac22n).
//
// ADR 0050's operator fact (b) names three channels restricted-class content
// may not enter: "the bead db, transcripts or memory". Two of them had a
// mechanism. The bead db has the data-ceiling lint (`data_ceiling_patterns:`,
// renderDataCeilingArgvGuard in gates.go, ADR 0069), and auto-memory is OFF
// fleet-wide (ClaudeFleetSettings `autoMemoryEnabled:false`, ranger-base-7uhip).
// The TRANSCRIPT had nothing: a seat dispatched into a repo whose content may
// not exist in a local file wrote the whole conversation — every file it read,
// every tool result — to $CLAUDE_CONFIG_DIR/projects/<key>/<session>.jsonl,
// where it sat until the 30-day cleanup swept it.
//
// SO THIS IS A CEILING AND NOT A LINT, which is the one thing to hold on to
// when reading the rest of the file. The visibility guard and the data ceiling
// both READ content and refuse a match; nothing here reads anything. The
// switch is per-REPO and it closes the channel outright, because the thing it
// is protecting against is the paste nobody thought to write a pattern for
// (ADR 0050's own Alternatives say that is the case a pattern list cannot
// see).
//
// ─── WHY THIS VARIABLE, when three other levers look closer ─────────────────
//
// All four were measured on claude 2.1.319, darwin 25.6.0, 2026-11-08, from a
// dispatched seat on the laptop. The three that lost are written down because
// each of them reads, from the outside, exactly like the answer.
//
//  1. `cleanupPeriodDays: 0` — the lever the bead that opened this work was
//     written around, and the reason it said MEASURE FIRST: it is documented
//     as disabling persistence AND deleting every existing transcript at
//     startup, user-wide, so one seat's `--settings` would have wiped the
//     coordinator's pile and every sibling's with it.
//     IT IS NOT THAT ANY MORE, and it is not a lever at all: the shipped
//     validator types the key `.int().positive()`, so 0 does not pass
//     validation — and the SIBLING key's own doc string says so in the
//     vendor's words, "Unlike cleanupPeriodDays, 0 is allowed because this
//     setting never disables writes". Its own description now points
//     elsewhere for this job: "use --no-session-persistence to disable
//     transcript writes entirely".
//     READ FROM THE VALIDATOR, NOT RUN. The destructive arm was declined on
//     purpose and the reason is arm 3 below: this box has no scratch
//     transcript root to run it in, so the only venue available was the live
//     pile. What would be needed is a box whose policy file does not pin the
//     config dir, or a scratch account — the operator's, not a seat's.
//     A payload carrying it is also worse than inert: a single row the
//     validator rejects makes the runtime discard the WHOLE `--settings`
//     object in silence (fieldpin.go, MEASURED 2026-09-05), which would take
//     the credential-dir pin, all 21 inlet rows and the fleet's permission
//     mode off the line while it was busy failing to stop a transcript.
//
//  2. `--no-session-persistence` — the flag the vendor's own doc string now
//     names, and it does disable the writes. It is REFUSED on an interactive
//     launch. MEASURED through a pty:
//
//     script -q /dev/null claude --no-session-persistence < /dev/null
//     → Error: --no-session-persistence can only be used with --print mode.
//
//     Every seat posse hires is interactive — DefaultAgentCommand carries no
//     `-p`, and it cannot: ADR 0013 §2's typed prompt needs a promptable
//     screen. So the flag is a launch that does not start.
//
//  3. a per-launch `CLAUDE_CONFIG_DIR` scoped to the session — the shape the
//     bead offered first, and the one it is worth being loudest about,
//     because it half works and the half that works is the half you can see.
//     MEASURED with the variable exported to a scratch directory: the
//     `.claude.json` landed in the scratch dir, and the TRANSCRIPT landed in
//     the operator's live pile anyway. This box's root-owned
//     managed-settings.json pins the config dir in its `env` block, at policy
//     scope — the LAST source in the runtime's settings fold — so an exported
//     value loses to it (the same precedence ranger-base-rq83c measured for
//     every other name in that block). The two resolvers disagree under the
//     pin, which is the worst available outcome: a seat reading a config file
//     posse never seeded (no trust grant, no interstitial answers) whose
//     transcript is exactly where it was. It would also rename the keychain
//     item out from under the operator's login — which is what
//     credentialDirPin (credential.go) exists to prevent, and what makes a
//     config-dir move a credential change rather than a path change.
//
//  4. CLAUDE_CODE_SKIP_PROMPT_HISTORY — this one. It is the environment arm
//     of the runtime's OWN persistence resolver, read from the 2.1.319
//     bundle:
//
//     Lpr(){ if(test env && !TEST_ENABLE_SESSION_PERSISTENCE) return "test_env";
//     if(AD()) return "explicit_disable";                       // the flag, arm 2
//     if(i.CLAUDE_CODE_SKIP_PROMPT_HISTORY) return "skip_prompt_history";
//     if(R_t()) return "nested_marker";                         // CLAUDE_CODE_CHILD_SESSION
//     return null }
//
//     `isTranscriptPersistenceDisabled()` is `Lpr()!==null`, and the
//     transcript writer's own methods are gated on it — the appender, the
//     torn-tail seal, the alias record, `canRecord`. There is no print or TTY
//     condition anywhere in that resolver, which is what makes arm 2's
//     restriction the flag's and not the mechanism's.
//
//     MEASURED, two `-p` runs differing in this one name, same cwd shape,
//     ANTHROPIC_BASE_URL at a dead loopback port so neither spends:
//     WITHOUT it → ~/.claude/projects/<key>/<uuid>.jsonl appeared.
//     WITH it=1 → the project directory was created, holding an empty
//     `memory/` and NO transcript file.
//     Nothing else moved: no existing transcript was touched, and the name is
//     one process's environment, so no sibling seat could see it.
//
// ─── WHAT IT COSTS, named because it is not nothing ──────────────────────────
//
// A seat with no transcript is a seat posse reads two facts off, and loses
// both:
//
//   - turn outcome. claude is the only runtime with a reader
//     (TurnOutcomeClaudeTranscript, Runtime.TurnOutcomeAdapter) and the
//     transcript is what it reads: an allotment refusal arrives as a
//     synthetic assistant message, which is how a pass tells an exhausted
//     account from an agent that settled without closing its bead
//     (ranger-base-02zr). In a transcript-off repo that distinction is gone
//     and dispatch says so on the settle line, exactly as it does for a
//     runtime that declares no adapter at all.
//   - cost. `posse cost` and the cockpit footer price what they can read
//     under the transcript root; a seat that writes nothing is spend the
//     ledger does not count. ADR 0018 §3's language is the right language for
//     it — a floor, not a total — except that here it is deliberate, so it
//     does not arrive as an unreadable-transcript warning. It arrives as
//     nothing at all.
//
// Resume goes too (`claude --resume` has no file to resume), which is the
// vendor's own stated consequence of the flag in arm 2 and costs a dispatched
// seat nothing: a fresh session per bead is the norm (ADR 0003 Dial F).
//
// That ledger is why UNMARKED KEEPS THE TRANSCRIPT. The other two walls in
// this family fail closed toward refusing; this one fails toward the RECORD,
// because a switch that defaulted to off would take the fleet's turn-outcome
// reader and its cost accounting away everywhere, silently, and nothing would
// report the loss. Only an explicit `off` buys the suppression.

import (
	"fmt"
	"strings"
)

// The marks. `off` is the only value that closes the channel; everything else
// — a typo'd path, an unreadable config, a value that is neither word — comes
// back `on`, which is what the launch did before this key existed.
const (
	TranscriptsOn  = "on"
	TranscriptsOff = "off"
)

// TranscriptsConfigKey is the config block, spelled once because two readers
// want it: SessionTranscripts walks it, and the refusal/warning sentences name
// it so an operator knows which line to edit.
const TranscriptsConfigKey = "session_transcripts"

// TranscriptsUnmarkedSource is the `source` for a repo config names nowhere.
// A const for BeadsVisibility's reason (VisibilityUnmarkedSource): a second
// reader asks "is this repo marked at all" and compares against it, and a
// prefix match on a sentence is not a predicate.
const TranscriptsUnmarkedSource = "unmarked in config " + TranscriptsConfigKey + ": — unmarked keeps the transcript (the fleet's turn-outcome reader and its cost scan read it)"

// SessionTranscripts answers whether a seat launched into the repo at dir may
// leave a transcript on disk, and how it knows. Same shape as
// BeadsVisibility, deliberately — one explicit, local, one-level map in the
// operator's config, compared by path IDENTITY (samePath: os.SameFile, never
// string normalizing, because on the APFS default two spellings of one
// directory differ only in case and a config key typed in one case never met
// the path git derived in the other — ranger-base-99gww):
//
//	session_transcripts:
//	  ~/code/projA: off
//
// The direction of the default is the opposite of BeadsVisibility's and the
// file comment above says why: unmarked is ON, because turning it off is what
// costs posse a reading.
func (a *App) SessionTranscripts(dir string) (mode, source string) {
	for _, kv := range YamlMapPairs(a.ConfigPath, TranscriptsConfigKey) {
		if !samePath(kv[0], dir) {
			continue
		}
		switch v := strings.ToLower(strings.TrimSpace(kv[1])); v {
		case TranscriptsOff:
			return TranscriptsOff, "config " + TranscriptsConfigKey + ":"
		case TranscriptsOn:
			return TranscriptsOn, "config " + TranscriptsConfigKey + ":"
		default:
			return TranscriptsOn, fmt.Sprintf("config %s: says %q, which is neither on nor off — treated as on", TranscriptsConfigKey, v)
		}
	}
	return TranscriptsOn, TranscriptsUnmarkedSource
}

// ClaudeTranscriptOffVar / ClaudeTranscriptOffValue are the one measured
// suppression surface, and the VALUE is as load-bearing as the name.
//
// The runtime reads this name in two places with two different rules. The
// transcript resolver tests plain JS truthiness (`if(i.CLAUDE_CODE_SKIP_
// PROMPT_HISTORY)`), under which the string "0" is TRUE; the prompt-history
// writer parses it as a boolean (`Pe(process.env…)`), under which "0" is
// FALSE. So `0` would suppress the transcript and keep writing prompt history
// — a half-closed channel that reads, from the config line, exactly like a
// closed one. `1` is true to both. Nothing here accepts a value from config
// for that reason: the config line says WHETHER, posse says WITH WHAT.
const (
	ClaudeTranscriptOffVar   = "CLAUDE_CODE_SKIP_PROMPT_HISTORY"
	ClaudeTranscriptOffValue = "1"
)

// TranscriptPin is the transcript ceiling as a fragment for the front of the
// typed line, beside RuntimeHomePin and after the gate prefix (WrapWithGates,
// gates.go) — so all three sites that type a runtime line carry it: planLaunch,
// RelaunchAgent and the runtime probe.
//
// TWO ARMS, for RuntimeHomePin's reason and not for symmetry's sake. A seat is
// a child of the multiplexer SERVER, whose environment was frozen whenever
// something happened to autostart it; posse cannot read that layer, so no
// launch decision may rest on it. Here the leak direction is the dangerous
// one: a `CLAUDE_CODE_SKIP_PROMPT_HISTORY` that reached that server's
// environment by any route would silence EVERY seat's transcript on the box,
// and the loss arrives as no error — the cost ledger simply stops counting and
// the turn-outcome reader simply stops answering. So an `on` repo does not
// merely decline to set the name, it UNSETS it:
//
//   - off → `K='1' `, an ordinary POSIX prefix assignment.
//   - on  → `/usr/bin/env -u K `, which unsets it for the seat. Never the
//     EMPTY value:
//     the resolver tests truthiness, so empty would work, but the prompt-history
//     half parses the string and an empty value there is a third state neither
//     this nor the runtime has a measurement for. Unset is the state both
//     halves were measured in.
//
// `/usr/bin/env` ABSOLUTE, for RuntimeHomePin's two reasons that agree: this
// is posse's own protocol rather than a persona's typed command (ADR 0077 D3),
// and the gates dir LEADS the PATH this very prefix just set, so a bare name
// would be resolved by whichever shim the persona's deny list rendered there.
//
// IT COMPOSES WITH RuntimeHomePin IN ALL FOUR COMBINATIONS, which is worth
// stating because two of them put an assignment after an `env`:
// `env -u A K='1' cmd` is `env` taking a NAME=VALUE operand, and
// `A='x' env -u K cmd` is an assignment the shell applies before exec'ing
// env. The other two are a bare pair of each kind. All four are pinned.
//
// IT COSTS BYTES ON A LINE WITH A CLIFF, and more of them than RuntimeHomePin
// does: the unset arm is 47 bytes and it lands on EVERY claude launch, not
// only the marked ones. A pane freshly created by the multiplexer is still in
// canonical mode, where MAX_CANON cuts at 1024 bytes (paneline.go), and
// PaneLine already spills an over-long line to state/launch/<session>.sh and
// sources it — the expected state since 2026-09-01 — so the cost is one more
// line over the cliff, never a line that is lost.
//
// A runtime with no TranscriptOffVar renders NOTHING, in either arm, which is
// every runtime but claude. That is a GAP and not a decision about those
// CLIs: nobody has measured a suppression surface for codex's or grok's own
// session stores, and typing a name nothing consumes would be a lever that
// reads as connected and is not (ADR 0017 §1's p84 shape). It costs this
// instance nothing today — all eleven PIDs on this box run claude — and it is
// the first thing to measure for a repo that is ever dispatched on another
// runtime.
//
// rt may be nil, as it may be for every other fragment WrapWithGates renders.
func TranscriptPin(rt *Runtime, mode string) string {
	if rt == nil || rt.TranscriptOffVar == "" {
		return ""
	}
	if mode == TranscriptsOff {
		return rt.TranscriptOffVar + "=" + shQuote(ClaudeTranscriptOffValue) + " "
	}
	return "/usr/bin/env -u " + rt.TranscriptOffVar + " "
}

// TranscriptCeilingLine is what a launch into a transcript-off repo SAYS, once,
// on the warn stream. A channel closed in silence is indistinguishable from a
// channel nobody closed, and the two readings posse gives up here (above) are
// both ones an operator reads later and would otherwise read as broken.
//
// Empty for an `on` repo: that is the state every launch was already in, and a
// line per launch saying nothing changed is noise that teaches a reader to
// skip the stream.
//
// `caged` is the CONTAINER tier, where the answer inverts and the line is the
// only thing standing in for a wall. That tier takes WrapInCage and its own
// cage HOME rather than this prefix, so the pin is not rendered at all —
// which means a repo marked `off` and dispatched on a `cage: container`
// persona gets NO ceiling. That is a silent open channel, and it is the exact
// shape this file exists to refuse, so it is said out loud on the one launch
// where it applies. Not a refusal: the ruling is about a channel, the cage is
// about a wall, and blocking the work to report a gap in a tier this box has
// no PID for would be posse deciding an operator's risk for them. ADR 0083 D6
// leaves the host's cage open for a ruling; if that ruling lands on
// containers, this branch is where the work starts.
func TranscriptCeilingLine(rt *Runtime, mode, source string, caged bool) string {
	if rt == nil || rt.TranscriptOffVar == "" || mode != TranscriptsOff {
		return ""
	}
	if caged {
		return fmt.Sprintf("transcript ceiling NOT IN FORCE: this repo is %s: %s (%s), but the container tier renders its own inside the cage and carries no %s pin — this seat WILL write a %s transcript under its cage home. ADR 0050's third channel is open for this launch.\n",
			TranscriptsConfigKey, TranscriptsOff, source, ClaudeTranscriptOffVar, rt.Name)
	}
	return fmt.Sprintf("transcript ceiling: this repo is %s: %s (%s), so this seat writes no %s transcript — ADR 0050's third channel. posse gives up two readings for it: turn outcome (the transcript IS the reader) and this seat's share of the cost scan.\n",
		TranscriptsConfigKey, TranscriptsOff, source, rt.Name)
}
