package posse

// THE REX GROK PRECONDITION (ADR 0076 Amendment 3, D12 and D13; bead
// ranger-base-nhxdx). On a `multiplexer: rex` instance grok reports its own
// state — natively, with nothing typed for it — but ONLY when the operator's
// own grok config carries one line:
//
//	[ui.notifications]
//	program_status = "on"
//
// So grok's admission is a declaration AND a reading of the box: D1's
// `ReportsOnRex` says the binary reports, and this says the line that makes it
// report is in force. Absent, `auto`, `off`, unreadable, or a grok this was
// not measured on are each refused BEFORE the claim, at D1's own two call
// sites (dispatch.go above the claim, planLaunch as the backstop).
//
// WHY A READING AT ALL, when every other half of D1 is a declaration. Because
// what the key governs is grok's behaviour on THIS box and not the binary's:
// MEASURED 2026-11-06 (docs/adr/0076-grok-hook.probe.sh), one grok 1.0.52
// against four fixture homes differing only in this key —
//
//	auto (the default) → no record through a working turn, AND the `n,auth`
//	                     composer leak: grok's `7501;?` query goes out, Rex's
//	                     reply lands after grok has stopped listening, and the
//	                     tail reaches the composer
//	off                → no record at all, and no leak
//	on                 → `idle` at launch, `working` during a turn, `idle`
//	                     after a cancel, cleared at `/quit`, every record
//	                     stamped `app=grok-build` with grok's own pid
//	on, no prompt      → `idle` by t+3 s
//
// Three of those four are a seat posse cannot read, and two of them are the
// shipped default. A declaration alone would admit all four.
//
// WHY THE REFUSAL IS WHERE D1's IS, and not a launch-then-wait: grok is
// `prompt: argv`, so the work prompt rides the launch line. Evidence that no
// record is coming arrives only after it has been delivered, which is a claim,
// a worktree and a live billed turn nobody observes (ADR 0013 §1 rule 3).
//
// WHY POSSE READS THE FILE AND NEVER WRITES IT (D13, interstitial.go's rule).
// `config.toml` is the operator's own — it carries their preferences, and grok
// rewrites it itself (`dismissed_version`, `marketplace.*`) — so a posse that
// copied a file over it would clobber both. The line is declared in
// `etc/grok/version-pin.toml` beside the `[cli]` keys and asserted by
// `make verify-grok-pin`, which is the same shape the two update ceilings
// already have.
//
// AND WHY POSSE CANNOT ASK GROK. `grok inspect --json` reports NO value for
// the key, and grok's own validator calls `ui.notifications` an "unrecognized
// config key" — both MEASURED — while the TUI obeys it in every arm above. So
// the only honest reader is the file, section-aware, and the only honest pin
// on "the TUI obeys a key its validator disowns" is the release it was
// measured on. That is RexVerified's second use (D13), and it is why a NATIVE
// report carries a version pin here where claude's does not.

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// RexPreGrokProgramStatus is the registry key grok's built-in declares in
// Runtime.RexPrecondition. A key, not a predicate, for Runtime.RexRoute's
// reason: the launch paths read the DECLARATION and never a runtime's name
// (ADR 0017 §3).
const RexPreGrokProgramStatus = "grok-program-status"

// The key this precondition reads, as a TOML path, and the one value that
// admits. Spelled as parts rather than as a dotted string because that is
// what the reader matches on and what the refusal prints back.
var grokProgramStatusPath = []string{"ui", "notifications", "program_status"}

// GrokProgramStatusOn is the value D12 requires — exactly "on". `auto` and
// `off` are both MEASURED silent on Rex and neither is a near miss: `auto`
// also leaks, and `off` is a deliberate answer to a different question.
const GrokProgramStatusOn = "on"

// GrokProgramStatusLine is the line D13 says posse documents and never
// writes. One spelling, shared by the refusal, the interactive warning,
// `posse runtime check`'s row and the version pin's declaration — so none of
// them can tell the operator to add something another one does not read.
const GrokProgramStatusLine = "[ui.notifications]\nprogram_status = \"on\""

// grokProgramStatusProbe is the re-measurement named whenever this
// precondition's own evidence is in question: four arms, no model turn, about
// two minutes (ADR 0076 Amendment 3's price for a deliberate grok bump).
const grokProgramStatusProbe = "sh docs/adr/0076-grok-hook.probe.sh auto on off onidle"

// grokProgramStatusRead is the reading: what the operator's grok config says
// about `[ui.notifications] program_status`, and whether it admits.
//
// THE HOME IS grokHomeIn's (ADR 0077 D1), which is the whole reason this is
// worth reading at all: posse resolves grok's root through `$GROK_HOME` and
// TYPES that same value onto the seat it hires, so the file posse read here
// is the file the seat's grok opens. Without that pin the seat resolves in
// the multiplexer SERVER's environment, which posse cannot read, and this
// reading would be about another box's config (ranger-base-c33j6).
//
// UNREADABLE IS "unknown", WHICH REFUSES — the interstitial probes' rule,
// read the way ADR 0013 §1 reads unknown: never as yes. A caged reader gets
// told it is one (probeCageNote), because an EPERM here is a fact about THIS
// SEAT and not about the operator's file.
//
// An ABSENT config is a READING and not an unknown, for grokPrivacyProbe's
// reason: grok writes this file itself, so a box without one has not been
// told anything — and it is refused either way, which is what keeps the two
// arms from having to be ranked.
//
// IT IS NOT MEMOIZED, unlike the version read beside it (ProbeCLIVersion's
// per-process memo). One small ReadFile per launch is cheaper than the exec
// that read is, and the asymmetry is deliberate: an operator who adds the
// line is admitted by the next launch of a running `posse dispatch --watch`,
// where a grok they UPGRADE is not noticed until the next process.
func grokProgramStatusRead() (string, bool) {
	h := grokHomeIn(os.Getenv("HOME"))
	if h == "" {
		// Join("", ".grok") is a RELATIVE ".grok" under whatever cwd this
		// process has — another box's answer reported as this operator's
		// (grokHomeIn's own note). No home, no reading.
		return noHomeSilence(GrokHomeVar, "cannot tell whether grok reports on Rex here").Why, false
	}
	p := filepath.Join(h, "config.toml")
	v, ok, err := tomlPathValue(p, grokProgramStatusPath...)
	switch {
	case err != nil && !probeAbsent(err):
		// Taken through unreadableSilence rather than composed here, so the
		// errno sentence and its cage note have ONE spelling across every
		// probe that prints one.
		return unreadableSilence(p, err, "so this cannot tell whether grok reports on Rex here").Why, false
	case err != nil:
		return "no " + AbbrevHome(p) + " at all", false
	case !ok:
		return "no [" + strings.Join(grokProgramStatusPath[:len(grokProgramStatusPath)-1], ".") + "] " +
			grokProgramStatusPath[len(grokProgramStatusPath)-1] + " in " + AbbrevHome(p), false
	}
	return strings.Join(grokProgramStatusPath, ".") + " = " + strconv.Quote(v) + " in " + AbbrevHome(p), v == GrokProgramStatusOn
}

// grokProgramStatusDoor is the remedy the refusal and the interactive warning
// both end with: the line to add, where it goes, and that posse will not add
// it. It names the file posse actually read, for grokHomeIn's rule that the
// path printed is the path read.
func grokProgramStatusDoor() string {
	h := grokHomeIn(os.Getenv("HOME"))
	p := "grok's own config.toml"
	if h != "" {
		p = AbbrevHome(filepath.Join(h, "config.toml"))
	}
	return "ADD THE LINE, which is the operator's to add and never posse's (ADR 0076 D13): put `" +
		strings.ReplaceAll(GrokProgramStatusLine, "\n", "` / `") + "` in " + p +
		" — posse reads that file, section-aware, and never writes it, because it carries the operator's own preferences AND grok rewrites it (" +
		"a copy over it would clobber both). It is declared in etc/grok/version-pin.toml beside the [cli] keys and asserted by `make verify-grok-pin`. " +
		"THE LINE IS BOX-WIDE: every grok the operator runs then writes OSC 7501 in every terminal, herdr panes included — and that costs you nothing, MEASURED 2026-11-07 (ranger-base-yqmkr) on grok 1.0.52 / herdr 0.9.1: a terminal that does not know an OSC consumes it unseen (ECMA-48 string controls), and herdr's emulator does, so 28 OSC strings into a pane left the rendered screen identical to the `auto` arm's and herdr's own grok detection reading the same states off the same rules. " +
		"Until it is there, "
}

// tomlPathValue reads one scalar out of a TOML file BY ITS FULL KEY PATH,
// tracking table headers as it goes. It is the section-aware reader D13 asks
// for, and it exists because tomlFlag (interstitial.go) cannot be reused as
// is: that one is line-level and section-BLIND on purpose — its two keys are
// unique in grok's config.toml and a parser for a yes/no probe would be a
// dependency bought for nothing.
//
// `program_status` is NOT such a key. It is a plausible name under more than
// one table, and the one table that counts is `[ui.notifications]` — which
// grok's own validator calls unrecognised while obeying it (D13). A
// section-blind read would take `program_status = "on"` out of ANY table and
// admit a seat on it, which is the admission D1 exists to refuse.
//
// WHAT IT READS, and the list is the contract rather than a sample:
//
//	[ui.notifications]      + program_status = "on"
//	[ui]                    + notifications.program_status = "on"
//	(root)                    ui.notifications.program_status = "on"
//
// All three are one TOML document to grok's parser, so all three admit here.
//
// WHAT IT DOES NOT READ is everything else — an inline table
// (`notifications = { program_status = "on" }`), a multi-line value, a quoted
// bare key carrying a dot. Those come back `found=false`, which REFUSES and
// names the spelling posse does read. That is the honest direction for an
// admission gate: the cost is an operator re-spelling one line, and the cost
// of guessing the other way is a seat whose every state is unknown.
//
// An `[[array.of.tables]]` header sets a table nothing can match, rather than
// being skipped — skipping it would leave the PREVIOUS header in force and
// read that array's keys as the enclosing table's.
//
// The read error is RETURNED, never swallowed: "no such file" and "operation
// not permitted" are a reading and an unknown, and a found=false covering
// both hands the caller a verdict it has not earned (ranger-base-m928w).
// Callers classify it with probeAbsent.
func tomlPathValue(path string, want ...string) (string, bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false, err
	}
	var table []string
	for _, ln := range strings.Split(string(b), "\n") {
		t := strings.TrimSpace(ln)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if strings.HasPrefix(t, "[") {
			table = tomlTableParts(t)
			continue
		}
		eq := strings.Index(t, "=")
		if eq < 0 {
			continue
		}
		key := tomlKeyParts(t[:eq])
		if key == nil || !tomlPathIs(table, key, want) {
			continue
		}
		return tomlScalar(t[eq+1:]), true, nil
	}
	return "", false, nil
}

// tomlNoTable is a table name no document can produce (a NUL is not legal in
// a bare or quoted key), so an array-of-tables header matches nothing.
const tomlNoTable = "\x00"

// tomlTableParts is the dotted name of a `[a.b]` header. `[[a.b]]` and a
// header this cannot parse both answer the unmatchable table above, which is
// the safe direction: it drops that section's keys rather than reading them
// under whatever header came before.
func tomlTableParts(line string) []string {
	if strings.HasPrefix(line, "[[") {
		return []string{tomlNoTable}
	}
	i := strings.Index(line, "]")
	if i < 0 {
		return []string{tomlNoTable}
	}
	parts := tomlKeyParts(line[1:i])
	if parts == nil {
		return []string{tomlNoTable}
	}
	return parts
}

// tomlKeyParts splits a dotted key into its parts, unquoting each. nil for
// anything it cannot read as one — an empty part, or a part carrying a
// bracket or whitespace inside it — so the caller refuses rather than
// matching half a key.
func tomlKeyParts(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ".") {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(p, `"`) && strings.HasSuffix(p, `"`) && len(p) >= 2 {
			p = p[1 : len(p)-1]
		} else if strings.HasPrefix(p, "'") && strings.HasSuffix(p, "'") && len(p) >= 2 {
			p = p[1 : len(p)-1]
		} else if strings.ContainsAny(p, " \t[]{}\"'=") {
			return nil
		}
		if p == "" {
			return nil
		}
		out = append(out, p)
	}
	return out
}

// tomlPathIs reports whether table+key is exactly want.
func tomlPathIs(table, key, want []string) bool {
	if len(table)+len(key) != len(want) {
		return false
	}
	for i, p := range table {
		if p != want[i] {
			return false
		}
	}
	for i, p := range key {
		if p != want[len(table)+i] {
			return false
		}
	}
	return true
}

// tomlScalar is the value on the right of the `=`, unquoted and with a
// trailing comment dropped. A QUOTED value is cut at its closing quote
// first, so a `#` inside the string stays in the value — a comment strip
// that ran unconditionally would corrupt it, and the value here is compared
// for equality against "on".
func tomlScalar(s string) string {
	s = strings.TrimSpace(s)
	for _, q := range []string{`"`, "'"} {
		if strings.HasPrefix(s, q) {
			if i := strings.Index(s[1:], q); i >= 0 {
				return s[1 : 1+i]
			}
			return strings.TrimPrefix(s, q)
		}
	}
	if i := strings.Index(s, "#"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}
