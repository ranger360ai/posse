## `instance:` — one herdr server, many homes (rangerhq-ouf9)

Everything under `$RHQ_HOME/state/` is namespaced by the home. A herdr
workspace **label** is not: it is the session name verbatim, on a server
that is shared by every posse home on the machine. Two homes that share a
persona name, or that type the same hand-named session — `smoke`,
`dispatch`, `scratch`, which is exactly what a cold install is told to type
— produce byte-identical labels, and the second home's create dies on
`already exists`.

Measured with two live instances on one server, before this landed: the
collision is **real and benign** — `posse new <name>` refuses cleanly, exit
1, nothing recorded, the other home's session untouched. Nothing silently
attaches. What is missing is not safety, it is *legibility*: neither home's
listing can say whose a row is, so the operator reaping a stale-looking
session has only the crew roster to reason from. The dispatch naming scheme
`<persona>-<repo>-<bead>` makes the dispatched case unreachable while crews
and repos are disjoint; the reachable case is the hand-typed name.

**The key.** `instance: <tag>` in `config.yaml`, default unset. When set,
every workspace this home creates is labelled `<instance>/<session>`.

**What it does not touch, and this is the whole design.** Session NAMES are
unchanged in every home: the meta filename, the work-prompt file, the
session branch, `posse attach <name>`, `posse peek`, the cockpit row. Only
the string handed to `workspace create --label` moves. Nothing parses
identity back out of a label — the meta dir (`state/herdr/<name>.yaml`) is
the ownership record, and a label is a thing an operator can rename in
herdr. So a label is *constructed* from a name to compare against
(`labelWearsName`), never *deconstructed* into one; name-as-interface
inference is the class rangerhq-lwx/v330 cured. Foreign rows keep showing
their full label, prefix included, which is the visible payoff: a row you
cannot account for now says which home to go ask.

**Three call sites, and two of them are the ones a naive fix forgets.**
`startPlanned` renders the label. `notOurWorkspace` — the rangerhq-yt1p
identity fence — anchors on the label, so a fix that only changed the create
would call every tagged session a stranger's and drop the whole fleet out of
its own listing. `nameWornElsewhere` — relaunch's pre-kill obstacle check —
would otherwise refuse a relaunch over another home's row and send the
operator to rename a workspace that is not theirs to touch.

A pre-tag label (the bare name) still reads as ours, deliberately: turning
the key on does not relabel the workspaces already running, and a predicate
that failed for the whole live fleet at once is the reading
`notOurWorkspace`'s positive-evidence rule exists to prevent. It costs
nothing — that predicate is only reached for a workspace whose id one of our
own metas already records.

**A malformed tag refuses the launch** (`planLaunch`, so every path: `posse
new`, a recipe, the cockpit, dispatch, and relaunch *before* it kills)
rather than falling back to an untagged label. The fallback is precisely the
colliding label the key was set to remove, and a home that thought it was
tagged and was not is the failure being configured against. Syntax is a
session name's, separator excluded: a tag containing `/` makes the split
ambiguous for the only reader a label has.

**The one assumption the design left open, now measured** (herdr 0.8.0,
protocol 19, 2026-08-27): a label containing `/` round-trips verbatim
through `workspace create`, `workspace list` and `workspace get`, and closes
by id like any other. Re-runnable as
`RHQ_LIVE_HERDR=1 go test ./internal/rhq -run TestLiveHerdrKeepsASlashInALabel`.

Coexistence needs the rest of the config too, and none of it is code:
disjoint `beads:` and `dirs:` (a repo is served by exactly one home), and a
distinct `autostart_session:` — the herdr `[[startup]]` hook arms one home
per server, and a second loop is run by hand.

Pinned: `internal/rhq/instancelabel_qa_test.go` (five hermetic arms, each
verified red against the change reverted) plus the live probe above. The
hermetic tests point `HERDR_SOCKET_PATH` at a path that is not there: an
unknown server generation is the board on which the label arm of
`notOurWorkspace` is the only fence, and the default path is the operator's
own live herdr, which would make them read machine state.
