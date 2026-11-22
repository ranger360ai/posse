# The park namespace is written where the park is typed (ranger-base-jv2q3)

A push hold parks the held commits on a `refs/heads/parked/<name>` ref and
resets the base out from under them (ranger-base-zlvyk). posse writes none of
those refs — parking is a hand procedure at the gate, by the decision
ranger-base-zlvyk recorded, because it runs once per hold. Two programs
nevertheless read that prefix now: the carrier census
(`scripts/park-carriers.sh`, `park_glob`) and the landing guard that refuses a
landing whose range reaches a held commit (ranger-base-3s2ig). So a hand
convention became an enforcement input, and MEASURED 2026-11-22 it was written
in no tracked doc, script or runbook before those two landed, against seven
live park refs in the checkout.

The failure mode is the procedure's, not either program's: a park spelled
`refs/parked/<name>`, as a tag, as `hold-<name>`, or as `parked-<name>` with
no slash is a park neither reader can see. The census prints nothing, the
guard refuses nothing, every carrier branch lands as it would have before
either existed, and nothing reds. The park's own author is the only person
positioned to notice.

## What was decided

Option 1 of the two the bead carried: **write the convention where the
procedure is**, rather than move the namespace to one posse owns
(`refs/posse/parked/*`).

The ordering argument is what decided it, not the cost. Moving the namespace
without moving the WRITER gives the guard a prefix a hand park never writes —
a guard reading an empty set refuses nothing and says so to nobody, which is
strictly worse than the inheritance it replaces. So option 2 needs the verb
that does the parking in the same change, and that verb was weighed and
declined once already under ranger-base-zlvyk, on the ground that parking runs
once a hold. Either way the procedure has to be written down, so option 1 is
also option 2's precondition. The runbook's closing section states the terms
on which that decision would be re-opened.

## What landed

- `docs/runbooks/push-hold-park.md` — the procedure: the namespace and what
  spelling it otherwise costs, the held set (`<park> --not <base>`), park
  before you reset, the census in the minute after the reset while the
  carrying set is still closed, what to do with a carrier a live seat holds,
  and the seat's own half.
- `AGENTS.md` — the park bullet beside the other-repo bullet that already
  cost four of these: the prefix, why it is load-bearing, the runbook's path,
  and the one thing it asks of a seat whose branch is named a carrier.
- `internal/treepins/parknamespace_qa_test.go` — the pin that holds the three
  readers together, which is the tie ranger-base-jv2q3's comment asked for
  once the option was chosen.

## The pin, and what it will and will not catch

Three arms. Arm 1 censuses tracked PROGRAMS — parsed string literals under
`cmd/` and `internal/`, non-comment lines under `scripts/` — and refuses any
park ref spelling that is not the one namespace, with a positive witness so a
census over a class that has gone empty cannot pass by saying nothing (1,186
Go files, 64 scripts, 17 spellings judged on 2026-11-22). Arm 2 reads the
procedure page: it exists, shows the prefix in a fenced block, says what
another spelling costs, cites paths that resolve, and is cited from the
orientation page. Arm 3 is the control.

It does not read prose outside that page, deliberately: a document must be
able to NAME a wrong spelling in order to refuse it, and a byte census over
markdown reds on the paragraph that explains the rule.

INVERTED before believing it, 2026-11-22 — six mutants, each planted and
restored, every arm red where it should be:

| mutant | arm | result |
|---|---|---|
| a script spells `refs/parked/<name>` | 1 | red |
| a non-rostered Go file carries it in a literal | 1 | red |
| the page drops the "nothing reds" consequence | 2 | red |
| the orientation page stops citing the page | 2 | red |
| the prefix constant moves to `refs/posse/parked` | 1 and 2 | red |
| the matcher goes blind (prefilter and root both) | 1 | red |

The fifth mutant is the one that found a real hole. Arm 2's first draft asked
only whether the page named the prefix ANYWHERE, and with the constant moved
to `refs/posse/parked` it stayed green — the page's own closing section names
that namespace as the alternative it is not using, so a counterexample in
prose satisfied a check meant to read a prescription. That is why arm 2 reads
a fenced block now, and why arm 3 drives the fence reader both ways.

## Measured beside it

`git reset --keep` is what the procedure prescribes for moving the base off
the held commits, rather than `--hard`. MEASURED 2026-11-22, git 2.56.0, in a
scratch repository built to the hold's shape: over a clean tree the base moves
and the park still holds its commit; with an uncommitted change to a file the
reset would have to rewrite it exits 128 with `error: Entry '<path>' not
uptodate. Cannot merge.`, the base does not move, and the change survives. In
a checkout with concurrent writers that refusal is the reason the form is
named.

`scripts/park-carriers.sh --self-test` was re-run on this tree and still says
both things: YES with a carrier, NO without one.

## Not closed here

- ranger-base-3s2ig
