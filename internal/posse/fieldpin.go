package posse

import "encoding/json"

// The command-string FIELD half of the settings pin (ranger-base-i7cy4,
// the half ranger-base-rflee split out because an `env` pin structurally
// cannot reach a field).
//
// inletpin.go pins what a settings file's `env` block can set. A settings
// file ALSO carries fields whose value IS a command: the runtime's own
// bundle enumerates them in one list (2.1.261, `uo`), and that list — not
// the audit's — is what this table is built from, because the audit named
// five of the eleven (plus hooks, which is not in it) and a name that is
// not here is not covered:
//
//	apiKeyHelper  awsAuthRefresh  awsCredentialExport  fileSuggestion
//	gcpAuthRefresh  otelHeadersHelper  processWrapper  policyHelpers
//	proxyAuthHelper  statusLine  subagentStatusLine
//
// HOW A HIGHER SCOPE WINS, and why "" is the value. Nine of the eleven are
// read off the MERGED settings object (the bundle's `bn()`) — all nine
// readers checked by name rather than assumed from the family — and that
// object is one left fold over the scope list in order: userSettings,
// projectSettings, localSettings, flagSettings, policySettings, with a
// single lodash mergeWith customizer. A later source therefore overwrites
// an earlier one's scalar, and every consumer of these nine tests it for
// truthiness before running it (`if(!e)return`), so an empty string reads
// as "not configured" and is the whole pin. Both halves of that are
// measured by execution below rather than transcribed.
//
// THE TWO NAMES THAT ARE DELIBERATELY NOT ROWS. Their absence is the point
// of writing this as a table:
//
//   - processWrapper resolves by a DIFFERENT rule — its own
//     `[policySettings, flagSettings, userSettings].find(v => typeof v ===
//     "string" && v !== "")` — which SKIPS an empty string. Pinning it ""
//     would leave a persona's value winning while looking pinned. The only
//     value that closes it is a real launcher argv prefix (`/usr/bin/env`
//     is the neutral candidate), and that is exec'd as the launcher of
//     every background session and worker on the box. That is a live change
//     to a deployed system with no readout available from a caged seat, so
//     it is the operator's per-change call and is filed, not guessed.
//   - policyHelpers is refused by the runtime unless it arrives from an
//     OS-admin source ("policyHelper ignored: delivered via non-admin
//     source"), so a persona-writable user scope cannot arm one and there
//     is nothing here to close.
//
// AND hooks, which is not a field but is the other half of the same bead:
// a higher scope CANNOT refuse a planted hook, because arrays CONCATENATE
// rather than replace (measured — see fieldpin_qa_test.go). The lever that
// works is policy-tier only (`allowManagedHooksOnly`) and is filed with the
// drop-in install, because it requires the crew's own two hooks to be
// re-declared at a scope a persona cannot write, in the same change.
//
// THE MEASUREMENTS, all by execution against the real claude 2.1.261 on
// 2026-09-05, each three-arm (control / attack / pin) because a pin whose
// attack arm never fired has measured nothing:
//
//	SCOPE ORDER, and that the payload reaches a lower scope's value:
//	a `.claude/settings.json` (project scope) naming an apiKeyHelper made
//	`claude auth status --json` report loggedIn=true, authMethod=
//	api_key_helper — that is the attack arm firing. The same run with
//	`--settings '{"apiKeyHelper":""}'` reported loggedIn=false: the pin
//	took the planted field away. Control with neither: false.
//
//	ARRAYS CONCATENATE, against the REAL user scope: this box's
//	~/.claude/settings.json carries 24 autoMode.environment entries.
//	`claude --settings '{"autoMode":{"environment":["…"]}}' auto-mode
//	config` reported 25, the user's 24 kept and the flag's appended last.
//	So a higher scope ADDS to a lower scope's array, and hooks — an object
//	of arrays, special-cased nowhere in the customizer — is that shape.
//
//	SCALARS REPLACE: this box's user scope enables one plugin. A project-
//	scope `enabledPlugins` naming it false made `claude plugin list` report
//	it disabled; with neither, enabled.
//
//	TYPES ARE NOT OPTIONAL, and this one is a hazard for the WHOLE pin: a
//	single wrong-typed row makes the runtime discard the entire --settings
//	payload in silence. `{"apiKeyHelper":"","statusLine":""}` left the
//	planted project-scope helper LOGGED IN — the credential dirs, all 21
//	inlet rows and the fleet's permission mode with it. statusLine,
//	subagentStatusLine and fileSuggestion are objects; only the
//	{"type":"command","command":""} spelling was accepted.
//	TestLiveClaudeFieldPinRefusesAPlantedCommandField is that canary, run
//	against the real reader; mutating this table's statusLine row back to a
//	bare "" turns it red, which is how the hazard is kept visible.
//
//	WHERE THE FLAG GOES, which cost an hour on ranger-base-rflee: on
//	2.1.261 `--settings` is a GLOBAL option and must precede the
//	subcommand. `claude auth status --settings X` is "unknown option";
//	`claude --settings X auth status` parses. rflee's note that auth status
//	no longer takes the flag reads the position, not the flag.
//
// COST ON THIS BOX: none, measured. None of the eleven names is set in the
// user-scope settings.json or settings.local.json (read by key NAME only,
// no value printed), so this pin overrides nothing the operator uses.

// quietCommand is the neutral value for the three fields the runtime types
// as an object rather than a string. The command is empty, and the shape is
// the only one the schema accepts — a bare "" here silently voids the whole
// payload, and so does `null` (measured 2026-09-05: a payload carrying
// `"statusLine":null` left a planted apiKeyHelper in force, which is the
// canary saying the runtime threw the whole thing away).
//
// THE COST OF THIS ROW, stated because it is not zero. The statusLine
// runner's guard is `if(!p||p.type!=="command")return`, and the schema types
// `type` as the literal "command", so there is no spelling that both
// overrides a planted command and leaves the guard bailing. Pinning these
// therefore DEFINES a status line on a box that had none: an empty command,
// spawned on the status line's refresh cadence in an interactive session,
// printing nothing. That is the price of the planted one never running, and
// it is a price, not a free win.
//
// Built fresh per row rather than shared, so a caller that edits one row's
// value cannot reach the other two.
func quietCommand() map[string]string {
	return map[string]string{"type": "command", "command": ""}
}

// fieldPinRow is one row of the field pin. Value is not a string for every
// row — see quietCommand — which is why this is not an EnvVar.
type fieldPinRow struct {
	Key   string
	Value any
}

// fieldPin is the command-string FIELD half of what a launch pins. Order is
// the bundle's own list order minus the two names above, so the rendered
// payload is stable and a diff of a launch line is readable.
func fieldPin() []fieldPinRow {
	return []fieldPinRow{
		{Key: "apiKeyHelper", Value: ""},
		{Key: "awsAuthRefresh", Value: ""},
		{Key: "awsCredentialExport", Value: ""},
		{Key: "fileSuggestion", Value: quietCommand()},
		{Key: "gcpAuthRefresh", Value: ""},
		{Key: "otelHeadersHelper", Value: ""},
		{Key: "proxyAuthHelper", Value: ""},
		{Key: "statusLine", Value: quietCommand()},
		{Key: "subagentStatusLine", Value: quietCommand()},
	}
}

// fieldPinUnpinned names the two rows of the runtime's list this table
// deliberately does not carry, with the one-line reason each. It exists so
// the omission is a value a test can read rather than a paragraph a reader
// has to notice (ranger-base-i7cy4).
var fieldPinUnpinned = map[string]string{
	"processWrapper": "its resolver skips an empty string, so the only closing value is a real launcher that gets exec'd — the operator's per-change call",
	"policyHelpers":  "the runtime refuses one that did not arrive from an OS-admin source, so a persona-writable scope cannot arm it",
}

// applyFieldPin writes the field pin into an already-decoded settings
// payload and says whether every row rendered. A false is the caller's cue
// to throw the object away and fall back to the bare const rather than ship
// a half-pinned one: a payload the runtime rejects is not a payload missing
// one row, it is no payload at all — the credential dirs and every inlet
// row go with it, in silence.
func applyFieldPin(m map[string]json.RawMessage) bool {
	for _, f := range fieldPin() {
		b, err := json.Marshal(f.Value)
		if err != nil {
			return false
		}
		m[f.Key] = b
	}
	return true
}
