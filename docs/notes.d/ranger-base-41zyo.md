# ranger-base-41zyo — the money line was spelled three times and enforced once, and a metered credential cannot carry a cap posse holds

The ask was a metered credential class with a hard cap, scoped per
engagement, because a customer engagement must run on the customer's own
metered credential and the harness structurally refuses exactly that class.
The refusal is not a bug — ADR 0019 refused metered keys on the money line
(rangerhq-kiz) and that refusal is why no agent here has ever been able to
spend — so the bead's question was whether the class can be admitted with a
cap, not whether the refusal can be dropped.

Two findings, and they point opposite ways. The first is that the refusal
was weaker than the page said. The second is that the cap the ask wants
cannot be held by posse at all, so the class that CAN carry one is not the
one the ask named.

## MEASURED 2026-10-04 — the refusal did not reach admission

On this tree, before the fix, on darwin 25.4.0:

A template-only runtime yaml whose `cage_cred:` named the metered variable
(the one name `meteredCredentialNames` holds, in
`internal/posse/meteredcred.go`):

```
CageCredential(rt)                               = <the metered name>
CheckCageCredential(rt, [<the metered name>])    = <nil>
```

Nil. The launch was admitted. The loop in `CheckCageCredential` asks one
question — is the declared name among the env-set names this launch will
inject — and a metered *declared* name that is present answers it yes.
`CheckCredGate` (ADR 0042 D2, the shim-collision precondition) returns nil
whenever that same call does, so it inherited the answer.

Meanwhile two other surfaces said the opposite:

| surface | what it said | did it enforce it |
|---|---|---|
| `posse refresh` (`refreshSession`, `checkSessionToken`) | refuses the NAME and refuses a `sk-ant-api…` VALUE | yes — this was the whole of it |
| `posse runtime check`, `cageCredRow`'s note | "a METERED api key is not accepted as this credential" | **no** — the row printed the metered name as the runtime's credential |
| `docs/runbooks/credential-rotation.md`, "Refusals before anything is written" | the name and the shape are refused on the money line | yes, and it is correctly scoped to the write |

So the enforced refusal protected the operator from their own 2am typing,
and nothing protected the shop from a FILE. `cage_cred:` is a line in a
runtime yaml and a runtime yaml is promoted config (`PromotedPaths`), so the
metered name had a route in that `posse refresh` never sees, and the first
thing that would have said so is the bill.

Fixed by making the predicate one definition asked at every admission point:
`internal/posse/meteredcred.go` owns the name set and the shape prefix,
`CheckMeteredSessionCredential` is the admission refusal, and
`CheckCageCredential`, `CheckCredGate` and `refreshSession` all read it. A
runtime declaring a metered credential now launches nothing at any tier,
caged or not, and not under `--allow-degraded`.

### Two limits that stay open, because they are structural

- **Admission sees NAMES, never values.** The preconditions are handed the
  env-set key names a launch is about to inject, deliberately never the
  values — posse does not read what the session credential holds (ADR 0019
  D1). So a metered key pasted by hand under the SCOPED MINT's own variable
  name is admitted, and the shape check stays the write path's alone. `posse refresh` is the tool that closes that, for the
  operator who uses it.
- **The set is exact names.** A renamed metered key is invisible, as
  `meteredKeyPrefix`'s own comment has always said of the shape. Not folded
  and not matched on a suffix substring: either would refuse another
  provider's decided session credential — bob's is its own — and claim a
  knowledge of env naming that unix does not share.

## The cap: posse parks hires, and that is not a hard cap

Every brake in this shop is dispatch-time. The plan guard parks on-meter
beads (ADR 0010 §5); Dial E stops dispatch at 100% of the tightest window
(`budget.go`). All of them refuse to **hire**. Nothing in posse stops an
in-flight turn from spending, and posse has no authority at the biller at
all.

So a ceiling enforced where posse can enforce it bounds NEW spending and
leaves the running session as residue. ADR 0010 §5 has written that down
twice already, about a different pair of quantities: *"a spending cap cannot
bound a plan window, and neither can a clock."* Calling a hire-time ceiling
a hard cap would be this page's own rejected pattern — false authority — and
the failure would land on a customer's money rather than the operator's.

**Where the cap WOULD live, if one were armed**, and the answer needs no new
mechanism: the home's promoted `config.yaml`. `config.yaml` is in
`PromotedPaths`; `HomeConstitutionPaths` keeps every member of that set out
of every seatbelt write grant (ADR 0015 §2/§3/§7, `seatbelt.go`
`ConstitutionGrants`); and `posse promote` is the operator's verb, denied in
every crew PID. At L2 and above the cap is therefore unreachable by the
session that would spend against it, by a wall built for other reasons.
Residue: an L0/L1 seat has no seatbelt, so there the cap is *unreached*
rather than unreachable.

Not an env var and not the env set beside the secret: a session rewrites its
own children's environment freely, so a cap in the environment is a cap the
spender edits. Not the PID: a PID is per-persona and an engagement is not.

## Scope is the engagement, and the engagement already has a name

The `.beads` store of record the launch directory resolves to (ADR 0055) —
the boundary posse already computes and already rides the session env as
`BEADS_DIR`. A per-session cap is mintable by restarting, which is the exact
flaw ADR 0028 §2 fixed for `budget_pass` by re-denominating it on a wall
clock. A per-persona cap lets one engagement's money fund another's.

## Which credential class can carry a cap that is actually hard

Not a raw metered API key. A 3P inference credential (Bedrock, Google
Cloud's Agent Platform, Microsoft Foundry) can, and for four reasons that
are all about where the authority sits rather than about convenience:

1. The ceiling is a quota or budget in the **customer's own cloud account**,
   enforced by the biller. A posse session has no API surface to it — a
   stronger statement than "a profile denies writing it".
2. The credential is role-scoped and short-lived, so ADR 0019 D5's "lifetime
   is reported, not inferred" gets a real expiry instead of "cannot tell".
3. Billing lands directly on the end user under their own agreement, which
   is what the published compliance terms require of usage on an end user's
   behalf.
4. No `sk-ant-api…` value enters an env set, so the name and shape refusals
   stay **total** rather than acquiring an exception — which is the only way
   to admit a class without weakening the refusal for ordinary seats.

**Not built here.** ADR 0019 D6 forbids configuring a provider shape this
instance has no instance of, and the preconditions are an operator ruling on
the money line and a real engagement, neither of which this bead had. ADR
0019 D7 names what a future amendment has to carry — the engagement scope,
the ceiling, who enforces it and where, and a dated operator attestation
that the ceiling was verified at the biller — because posse can only ever
record evidence of a cap it does not hold, and a recorded cap nobody checked
is worse than none.

## What landed

| path | what |
|---|---|
| `internal/posse/meteredcred.go` | the class: name set, shape prefix, `IsMeteredCredentialName`, `CheckMeteredSessionCredential` |
| `internal/posse/cage.go` | `CheckCageCredential` asks the money line first |
| `internal/posse/gates.go` | `CheckCredGate` asks it before the collision — a metered `cage_cred:` refuses a launch whose PID shims nothing |
| `internal/posse/refresh.go` | the two definitions it used to own now come from the class |
| `internal/posse/runtimecheck.go` | the `cage_cred` row names the refusal instead of printing a metered name as the credential |
| `internal/posse/meteredcred_qa_test.go` | six arms, each mutation-checked; the header says which edit reds which arm |
| `docs/adr/0019-credential-architecture.md` | D7, and the Rejected list |
| `docs/runbooks/credential-rotation.md` | the admission refusal, and the D7 collision below |

## One collision, found while citing it

`docs/runbooks/credential-rotation.md` cites two different ADR 0019s — this
repo's and the private **instance** ADR 0019, both named at the top of that
page (ADR 0012 D6 keeps them apart). The instance one already has a D7, move
3 of that runbook: the scoped mint belongs in `container.env` and nowhere
else. So this repo's new D7 and the instance's D7 are different rules about
the same credential, four moves apart on one page. The convention that tells
them apart was already there — a bare `ADR 0019` is this repo's, `instance
ADR 0019` is the private one — but relying on the absence of one word was
thin, so this bead's own citation in that runbook says "this repo's" and
points at the other. Worth knowing before the next decision is numbered in
either document.

The ruling that would admit the 3P shape is filed as `ranger-base-s1hq3`
(`-l question,risk`): the three things only the operator can decide, what
"verified" would have to mean, and the blast radius of a yes.

No money was spent and no credential minted under this bead.
