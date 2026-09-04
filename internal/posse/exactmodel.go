package posse

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// ADR 0053: an exact model id is an explicit crew-session canary, never a
// default. `posse new --model <id>` is the ONLY way one is named — there is
// no PID key, no runtime overlay key, no config key, no label rule, no
// recipe field and no dispatch flag, and nothing infers a model from a tier
// or from a persona. The operator types the id for every new canary session.
//
// The point of such a launch is to ask the PROVIDER whether that id is
// available, so posse's job is to carry the typed id to the CLI unchanged,
// or to refuse before anything exists. Everything else about the launch is
// the ordinary persona path: PID, gates, skills, cage, env sets and
// reasoning effort (ADR 0053 D2).

// CheckExactModel is the whole refusal set for `--model`, asked where no
// workspace, worktree or session record exists yet (ADR 0053 D1). It is
// pure — argv in, error out — so the flag contract is testable without a
// herdr, and it is asked from BOTH the flag parser and planLaunch: the
// parser gives the operator the error at the point of the typo, planLaunch
// is the wall every other launch path (a recreate above all) goes through.
//
// The companions are required rather than defaulted because an exact model
// is only meaningful against a stated runtime and a stated workload
// intent: a model id names neither. Defaulting either one would let the
// same typed id mean different things on two boxes, which is the persistent
// default ADR 0053 exists to refuse.
//
// The runtime's own model flag is NOT asked here — that needs a loaded
// runtime, and planLaunch asks it the moment it has one.
func CheckExactModel(o NewSessionOpts) error {
	if o.Model == "" {
		return nil
	}
	if o.Agent == "" {
		return Die("--model %s needs --agent: an exact model is a canary launch of a PERSONA, and there is no persona-less line for it to ride (ADR 0053 D1)", o.Model)
	}
	if o.Runtime == "" {
		return Die("--model %s needs an explicit --runtime: the model flag that carries the id is the runtime's, so posse will not guess which CLI you meant (ADR 0053 D1)", o.Model)
	}
	if o.Tier == "" {
		return Die("--model %s needs an explicit --tier (strong | standard | fast): tier: stays the operator's statement of workload intent, and an exact model does not state one (ADR 0053 D1)", o.Model)
	}
	return CheckModelID(o.Model)
}

// CheckModelID refuses an id posse could not render as one argv token. The
// id reaches the CLI through the runtime's model flag and the existing
// shell quoting (ADR 0053 D2), and shell quoting would happily carry a
// newline or a space into the launch line — where it becomes a second
// token, a second flag, or a second COMMAND. It also has to survive the
// flat session record, whose reader stops at the first newline
// (ranger-base-ujdg), so the same one-token rule is what keeps `model:`
// readable back.
//
// Refused rather than sanitized: an id posse rewrote would ask the provider
// about a model the operator did not type, and the answer to that question
// measures nothing.
func CheckModelID(id string) error {
	if id == "" {
		return Die("--model needs a model id")
	}
	if !utf8.ValidString(id) {
		return Die("--model %q is not valid UTF-8: posse will not guess what the operator typed at a provider (ADR 0053 D1)", id)
	}
	for _, r := range id {
		if unicode.IsSpace(r) {
			return Die("--model %q is not one token: a model id carrying whitespace becomes a second argv word on the launch line (ADR 0053 D1)", id)
		}
		if unicode.IsControl(r) {
			return Die("--model %q carries a control character: it would not survive the launch line or the flat session record (ADR 0053 D1)", id)
		}
	}
	return nil
}

// ModelTag is the listing suffix's model half — "=<id>" — kept beside the
// tag it extends so the two spellings cannot drift apart.
func ModelTag(model string) string {
	if model == "" {
		return ""
	}
	return "=" + model
}

// ExactModelLine is what a canary launch says on stderr. It is printed
// instead of the availability preflight's line (ADR 0053 D3): the preflight
// asks whether the account can run the TIER's model, and this launch is not
// running the tier's model — printing a verdict about a model nobody
// launched would be the substitution this decision exists to prevent, said
// in words.
func ExactModelLine(name, runtime, tier, model string) string {
	return strings.Join([]string{
		name + " launches on " + runtime + " @ " + tier + " with the EXACT model " + model,
		"tier availability substitution is skipped (ADR 0053 D3) — a provider refusal is the canary's answer, not a reason to fall back",
	}, " — ")
}
