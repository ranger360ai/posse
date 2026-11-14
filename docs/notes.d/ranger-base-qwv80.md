# A host name is an instance's own vocabulary, and the entry that walls it is bare (ranger-base-qwv80)

*2026-11-13, gilfoyle, from ranger-base-9v2xo.*

A `docs/notes.d` measurement table quoted the box's own host name — a reading
whose entire point was that the name came back through
`/proc/<pid>/root/etc/hostname` — and the commit wall took it. It was fixed
forward in c8d15392 and the history keeps it.

## Why the wall passed it, which is the whole of the diagnosis

The host's name is not derived. Check 3 derives the box's username, every
scope's git e-mail and the instance repo's path (`DeriveIdentityLiterals`) and
nothing else; the shipped ops-pattern list is what any deployer of this
software needs. A name that is confidential at ONE deployment is that
deployment's own vocabulary, and config `beads_visibility_patterns:` is the
whole of the mechanism for it (ADR 0048 D2). So nothing was broken — there was
no entry.

A derived-hostname literal is the obvious alternative and is not proposed. It
would refuse every commit on a box named `build` or `localhost`, with no
remedy a writer could perform, which is the same wall `isShippedPosseName` had
to be added to patch when ADR 0080 D2 put the crews under an account named
`posse`. If it is ever wanted it is a posse-tree decision and not one
instance's.

## The finding: the house marker form does not fit a host name

ADR 0048 D1's pattern shape is `<name>([^-]|-[^0-9a-z]|-?$)` — the MARKER
form, whose exception exists to let a bead id (`<name>-7xpn`) through — and it
is the shape a third entry gets copied from, because it is the shape every
entry in force already has. MEASURED 2026-11-13, the three node spellings a
host on this tier actually has, through both EREs (fixture names, which is
also what the pins use):

| subject | marker-form | bare |
|---|---|---|
| `nimbusbox` | hit | hit |
| `nimbusbox-rex` | **MISS** | hit |
| `nimbusbox.tail9q7z2.ts.net` | hit | hit |

The MISS is the one that matters. An always-on host's multiplexer registers
its own tsnet node as `<host>-rex` (ADR 0080 D1, access over the operator's
tailnet), so a marker-form entry walls the host and leaves unwalled the one
name that reaches the box from off it. A host name has no marker form to make
an exception for, so the entry is **bare** — and bare is also what makes ONE
entry cover the host, the tsnet node and the MagicDNS name.

The tailnet's MagicDNS suffix is a SECOND entry, not a wider first one: the
host entry cannot reach a line that carries the tailnet and not the host (a
node URL, a `tailscale status` paste), and the refusal has to be able to say
which name was written. The MagicDNS name is a hit of both, deliberately —
neither entry alone is the whole wall, and the pin's M4 mutant is what
measures that.

## Visibility, not the ceiling

`data_ceiling_patterns:` would refuse the row in every repo on the box
whatever its stamp, and the remedy a visibility refusal names is "write it in
the instance tree and have the public artifact restate and cite it" (ADR 0024
D3) — so a ceiling entry here would leave the measurement nowhere to live.
Visibility is the right question: a host's name is fine in the instance's
private tree and not in a public one. The pin asserts both halves, because
"refused in public" and "still takes it in private" are two claims and only
the pair is the rule.

## How a candidate entry gets priced before it is turned on

Two numbers, and they are cheap enough that no entry should be added without
them. MEASURED 2026-11-13 over this repo:

- **false positives**: 0 content hits over 1,903 tracked files, 0 tracked
  paths, 0 hits over every commit message on `main`. An entry that reds the
  tree it is installed into is a wall nobody can commit through.
- **true positives in history**: exactly ONE commit's added lines on `main`
  would have been refused, and it is the leak itself. One hit, one real
  target.

The second number is the one that says the entry is about a real subject
rather than a feeling, and both come from a `git log -p -U0` scan plus a
`git ls-files` walk — no new tooling.

## The residuals, because a pattern list's misses are its documentation

It is a lint, not a boundary (visibility.go, `OpsPatternsConfigKey`): a name
nobody thought to add is exactly what a pattern list cannot see. Two misses
are known and deliberate rather than overlooked:

- an ALL-CAPS spelling of the host name is unwalled. The entry case-folds the
  first character only, which is the variation a writer actually produces, and
  a nine-character name folded whole is a line nobody can read in a hook.
- a Tailscale node's DISPLAY name can carry an apostrophe, and a single quote
  cannot appear in a value at all — `validateOpsERE` refuses it, because the
  value is rendered into a single-quoted sh word. That spelling is
  inexpressible in this dialect, not merely absent.

## What landed here

`internal/posse/instancehostname_qa_test.go`, three pins over the fixture
entries: the 9v2xo row refused in a public repo with a control that shows the
shipped list alone takes it (that control is not a hypothetical — it is the
wall the row landed through), the bare entry's three spellings against the
marker form's two, and the tailnet class as its own. Four mutants were run one
at a time against a golden copy and each pin's blast radius is recorded in the
file's head; M2 — the marker form restored — reds the `-rex` row alone, which
is why that row is a pin and not a third item in a list.

## Not closed here

- ranger-base-hm40z
