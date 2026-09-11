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

// ModelUncheckedMark is what the listing adds to a recorded exact model on a
// runtime MEASURED to run its own default for an id it does not know
// (Runtime.UnknownModel == UnknownModelSwap).
//
// ADR 0053 D4 makes `model:` the store of record for the override and the
// listing a rendering of it, and that is still what this is: the mark adds
// no second store and reads nothing about the session. What it fixes is a
// claim the rendering was making on its own. `@grok/strong=grok-4.7` reads
// as the model this session is running, and on grok it is the model posse
// was ASKED to run: an id that does not exist launched clean and the row
// said grok-4.7 while Grok 4.6 answered (ranger-base-jzm04). The mark is
// the difference between the two sentences, said in one token, and the
// `unknown model` row of `posse runtime check <runtime>` is the long form.
//
// It is keyed on the MEASURED swap and not on the absence of a measurement:
// an unmeasured runtime is already loud in `runtime check`, and a mark that
// also meant "nobody has measured this CLI" would stop meaning "this row
// may be naming a model that is not running", which is the one thing an
// operator reading a canary row needs it to mean.
const ModelUncheckedMark = "?unchecked"

// ExactModelLine is what a canary launch says on stderr. It is printed
// instead of the availability preflight's line (ADR 0053 D3): the preflight
// reads the CATALOG for the TIER's model, and this launch is not running the
// tier's model — a verdict about a model nobody launched would describe a
// launch nobody made. The canary asks the provider instead.
//
// The last clause is the runtime's own DECLARATION (Runtime.UnknownModel),
// not prose, and that is the fix ranger-base-jzm04 was filed for. This line
// used to end "the provider is asked, not the catalog, and its refusal is
// the canary's answer" for every runtime. On grok 1.0.5, measured, nothing
// refuses: an unknown -m runs Grok's own default and says nothing, so posse
// was promising the operator a refusal that cannot arrive and reporting an
// id nobody served. A launch may only promise what the runtime declares —
// and where nothing is declared it says that, which is the third clause.
//
// It consumes the declaration rather than the runtime's NAME (ADR 0013 §7,
// ADR 0017 §3): what a CLI does with an unknown id is a dimension, so it is
// a field on Runtime, measured per runtime and overlayable per box.
func ExactModelLine(name, runtime, tier, model string, rt *Runtime) string {
	return strings.Join([]string{
		name + " launches on " + runtime + " @ " + tier + " with the EXACT model " + model,
		"the tier availability verdict is skipped (ADR 0053 D3) — a verdict about the tier's own model would describe a launch nobody made",
		exactModelAnswerClause(runtime, model, rt),
	}, " — ")
}

// exactModelAnswerClause says what an answer from this runtime is worth.
// Three clauses because the dimension is three-valued, and the UNDECLARED
// one is not the swap clause softened: "nobody measured this" and "this CLI
// was measured to run something else" are different facts to an operator
// deciding whether the session in front of them is the canary they asked
// for.
func exactModelAnswerClause(runtime, model string, rt *Runtime) string {
	unknown := ""
	if rt != nil {
		unknown = rt.UnknownModel
	}
	switch unknown {
	case UnknownModelCarry:
		return "and the provider is asked rather than the catalog: " + runtime + " takes an id it does not know to the provider (declared unknown_model: " + UnknownModelCarry + "), so the provider's refusal is the canary's answer"
	case UnknownModelSwap:
		return "but " + runtime + " answers an id it does not know by running its OWN default and saying nothing (declared unknown_model: " + UnknownModelSwap +
			"), so this launch coming up clean is NOT the provider taking " + model + ": read the model off the session's own screen, and posse list marks the row " + ModelUncheckedMark
	}
	return "and whether " + runtime + " takes an unknown id to the provider or runs its own default instead is UNMEASURED here (unknown_model: unset), so a clean launch is not yet evidence " + model +
		" exists — `posse runtime check " + runtime + "` says as much"
}
