package posse

// The metered credential class (ADR 0019 D7, bead ranger-base-41zyo) — one
// predicate, in one place, asked at every point a credential is ADMITTED as
// well as at the one point posse writes one.
//
// `ANTHROPIC_API_KEY` is spending. The operator's ruling of 2026-08-20
// (rangerhq-kiz) refused it as a session credential on the money line, and
// that refusal is why no agent in this shop has ever been able to spend. It
// is not a bug and ranger-base-41zyo did not remove it.
//
// WHAT THAT BEAD FOUND. MEASURED 2026-10-04 on this tree: a runtime yaml
// carrying `cage_cred: ANTHROPIC_API_KEY` resolved CageCredential to that
// name, and CheckCageCredential(rt, []string{"ANTHROPIC_API_KEY"}) returned
// nil. The refusal lived on the WRITE path alone — `posse refresh` would
// not write that name and would not write a value of that shape — while
// both ADMISSION preconditions, the container's (CheckCageCredential) and
// the shim collision's (CheckCredGate, ADR 0042 D2), asked only whether the
// declared name was among the env-set names the launch was about to inject.
// The declared name WAS the metered one, so it was present, so the launch
// was admitted. Meanwhile `posse runtime check`'s own `cage_cred` row has
// said "a METERED api key is not accepted as this credential" since it was
// written: the shipped sentence described a check nobody performed.
//
// Why admission and not merely the write. A write refusal protects the
// operator from their own 2am typing. An admission refusal protects the shop
// from a FILE — `cage_cred:` is a line in a runtime yaml, and a runtime yaml
// is promoted config (PromotedPaths), so the metered name can enter by a
// route `posse refresh` never sees and the first thing that would say so is
// the bill.
//
// TWO HONEST LIMITS, both structural, neither closed here:
//
//   - Admission sees NAMES, never values. The preconditions are handed the
//     env-set key names a launch is about to inject and deliberately never
//     the values — posse does not read what the session credential holds
//     (ADR 0019 D1). So a metered key pasted BY HAND under the name
//     `CLAUDE_CODE_OAUTH_TOKEN` is admitted, and the shape check stays the
//     write path's alone. That is the gap `posse refresh` exists to close,
//     and it closes it only for the operator who uses it.
//   - The set is a set of NAMES. A metered key under another name is a key
//     this file cannot see, exactly as meteredKeyPrefix's comment has always
//     said of the shape. The set is not widened by guessing — `BOB_API_KEY`
//     is another provider's decided session credential and is not this
//     ruling's subject.
//
// WHAT IS DELIBERATELY NOT HERE: a cap, an engagement scope, or any
// configuration that would admit a metered credential as its own class. ADR
// 0019 D7 is the verdict — a raw metered key cannot carry a hard cap that
// posse holds, because posse can park a HIRE and cannot stop an in-flight
// turn, and it has no authority at the biller — and ADR 0019 D6 is why
// there is no config key for the shape that can (a 3P inference credential,
// whose cap is the customer's own quota in the customer's own account). D6
// forbids configuring a provider this instance does not have.

import "strings"

// meteredCredentialNames are credentials that are metered spending. A
// persona is never the one who decides to spend (rangerhq-kiz), and neither
// is a command that writes a file without one in the room, and neither is a
// yaml line: the name is refused where it is written and where it is
// admitted.
//
// Exact names, not a heuristic. An `API_KEY` substring test would refuse
// another provider's decided credential, and a case fold would claim a
// knowledge of env naming that unix does not share.
var meteredCredentialNames = map[string]bool{"ANTHROPIC_API_KEY": true}

// meteredKeyPrefix is the prefix Anthropic's metered API keys carry, as
// against a setup-token's `sk-ant-oat…`. It is a shape check and not an
// authority: a key that has been renamed slips past it. It is here because
// the failure it catches — a metered key pasted into the session variable —
// is silent, spends money, and looks exactly like success.
//
// Its one reader is the write path (checkSessionToken). Admission cannot use
// it: admission is handed names and never values, which is the first of this
// file's two honest limits.
const meteredKeyPrefix = "sk-ant-api"

// IsMeteredCredentialName is the predicate itself. Trimmed because the name
// arrives from a yaml line and a trailing space is a typo, not a second
// credential.
func IsMeteredCredentialName(name string) bool {
	return meteredCredentialNames[strings.TrimSpace(name)]
}

// CheckMeteredSessionCredential is the ADMISSION refusal: a runtime whose
// declared session credential IS metered spending launches nothing, at any
// tier, caged or not.
//
// Nil for every runtime that declares something else and for one that
// declares nothing at all — "undecided" is a different refusal with a
// different next move, and both callers already own it.
//
// It names the file the line is in, because the fix is an edit to that file
// and because a refusal that does not say where the decision was made sends
// the reader to the wrong tree: a promoted runtime yaml is the operator's,
// and no session's to edit.
func CheckMeteredSessionCredential(rt *Runtime) error {
	name := CageCredential(rt)
	if !IsMeteredCredentialName(name) {
		return nil
	}
	return Die("posse: runtime %s declares %s as its session credential, and that is metered spending — refusing the launch (rangerhq-kiz, ADR 0019 D4/D7).\n"+
		"  declared by: %s\n"+
		"  A persona is never the one who decides to spend, so this is refused where it is WRITTEN (`posse refresh`) and, since ranger-base-41zyo, where it is ADMITTED — which is here, and is the route a yaml line takes.\n"+
		"  The session credential is a scoped mint, not a meter: on claude, `claude setup-token`, into an env set (mode 600, never in the repo), named in the PID's envs:.\n"+
		"  Not waivable by --allow-degraded: this is not a gate the wall could not realize, it is spending authority a session may not hold.\n",
		rt.Name, name, rt.declaredBy("cage_cred"))
}
