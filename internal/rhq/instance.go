package rhq

// The instance tag: what one posse install's sessions are called on a herdr
// server that serves several of them (rangerhq-ouf9).
//
// One herdr server, N instances — an instance being an RHQ_HOME plus the bd
// repos its `beads:` names. Everything under state/ is namespaced by the
// home; a workspace LABEL is not. Today's label is the session name
// verbatim (SessionFor/SessionForBead → CreateWorkspace), so two homes that
// share a persona name, or that type the same hand-named session (`smoke`,
// `dispatch`, `scratch`), put byte-identical labels on the one shared
// server. Measured, with two live instances: the second create refuses
// cleanly — "session 'x' already exists", exit 1, nothing taken over — and
// the other instance's session shows up unmarked in a listing that has no
// way to say whose it is. The dispatch naming scheme
// <persona>-<repo>-<bead> makes the dispatch case unreachable while the
// crews and repos are disjoint; the reachable case is the hand-typed name,
// which is exactly what a cold install is told to type.
//
// `instance:` answers that at the presentation layer and nowhere else:
//
//   - Session NAMES are untouched. <persona>-<repo>-<bead> is what the meta
//     filename, the work-prompt file, the session branch and every `posse`
//     command say, in every instance. Only the herdr label is prefixed, and
//     only at CreateWorkspace.
//   - Identity is never parsed back out of a label. The meta dir
//     (state/herdr/<name>.yaml) is the ownership record; a label is a
//     string an operator can rename in herdr, and name-as-interface
//     inference is the class rangerhq-lwx/v330 cured. So a label is
//     CONSTRUCTED from a name to compare against (labelWearsName), never
//     DECONSTRUCTED into one.
//   - Foreign rows keep showing their full label, prefix and all. That is
//     the whole visible point: a row an operator cannot account for now
//     says which instance to go ask.
//
// Default empty = today's labels exactly, so a single-instance home sees no
// change at all.
//
// MEASURED before building this, on herdr 0.8.0 (the one assumption the
// design left open): a label containing '/' round-trips verbatim through
// `workspace create --label a/b`, `workspace list` and `workspace get`, and
// the workspace closes by id like any other. Hence the separator below.
const InstanceSep = "/"

// Instance is the validated instance tag: "" for the single-instance
// default, else the tag every herdr label of this home is prefixed with.
//
// The syntax is a session name's (ValidName), for the reason the separator
// is not in it: `instance: a/b` or a tag with a space would produce labels
// no operator can read back as <instance>/<name>, and one containing the
// separator would make the split ambiguous for the human doing the reading
// — which is the only reader a label has.
//
// A malformed tag is an error rather than a silent fallback, because the
// fallback is precisely the colliding label this key exists to remove: an
// instance that thought it was tagged and was not is the failure it was
// configured against. Every launch path asks through planLaunch, so the
// refusal lands before anything is created and before relaunch kills what
// it is replacing.
func (a *App) Instance() (string, error) {
	tag := a.CfgGet("instance", "")
	if tag == "" {
		return "", nil
	}
	if !ValidName(tag) {
		return "", Die("bad instance '%s' in %s (letters, digits, - and _; may not start with -) — the instance tag prefixes every herdr workspace label as '<instance>%s<session>'", tag, a.ConfigPath, InstanceSep)
	}
	return tag, nil
}

// instanceTag is Instance for the read paths — rendering a label, and
// asking whether a label wears a name. A malformed tag reads as unset here
// rather than erroring: a listing must keep working, and treating an
// unusable tag as absent is the single-instance behaviour these paths had
// before the key existed. The launch is where it is refused, and that
// refusal is what keeps a home from ever creating a workspace under a tag
// this ignores.
func (a *App) instanceTag() string {
	tag, err := a.Instance()
	if err != nil {
		return ""
	}
	return tag
}

// WorkspaceLabel renders the herdr label for one posse session name. It is
// the only place a label is built, so the label rule has one home.
func (a *App) WorkspaceLabel(name string) string {
	tag := a.instanceTag()
	if tag == "" {
		return name
	}
	return tag + InstanceSep + name
}

// labelWearsName reports whether a herdr label denotes THIS instance's
// session <name>. Construction, not parsing: it compares against the labels
// this home would have written, and knows nothing about anyone else's.
//
// The bare name counts as ours even when a tag is set, and that is
// deliberate. Turning the key on does not relabel the workspaces already
// running under it, and a home whose whole fleet suddenly failed this
// predicate would have every session read as a stranger's at once — the
// very reading notOurWorkspace's "positive evidence only" rule exists to
// prevent. The cost is nil: this predicate is only ever reached for a
// workspace whose id a meta of ours already records.
func (a *App) labelWearsName(label, name string) bool {
	if label == name {
		return true
	}
	tag := a.instanceTag()
	return tag != "" && label == tag+InstanceSep+name
}
