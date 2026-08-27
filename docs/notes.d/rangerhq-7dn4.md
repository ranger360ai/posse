## The empty listing is the prune's arm, not the create's (rangerhq-7dn4)

`mustNotOrphan` no longer fetches the workspace listing at all. The write's
socket guard is now the comparison alone — `cannotAnswerFor(m, sock)` —
followed by the per-id query it always ran. The empty-listing arm moved out
of the shared predicate into `emptyBoard`, called at the prune's call site
beside the other arm only the prune takes (`m.Socket == ""`).

**What it cost where it was.** `cannotAnswerFor` tested `listed == 0` *before*
it compared sockets, so the create refused on an empty board even when the
meta named this very socket. It only ever changed the answer on that board —
a socket mismatch is refused by the comparison, and every unstamped/named
combination *is* a mismatch — which is to say it only fired where the socket
evidence said this server is the one that would know. Two ordinary boards:

- the last session on a server dies, and its name is unusable until some
  other workspace exists;
- a herdr restart, which empties the listing for **every** meta at once, so
  `posse relaunch <name>` — the recovery command, whose `clearDeadMeta` asks
  `mustNotOrphan` one line before the unlink — was refused fleet-wide.

That is the same cost class jeu2's close measured and rejected for the
unstamped arm ("a dead session's name can never be reused again … that is the
operator's own ordinary path, not an edge"), narrower but with relaunch in it.

**Why the arm does not hold on the write side.** Its message offers two
readings and neither survives a socket match. "One that never held this
session" is what the comparison decides, and decides better: a meta naming
this socket says plainly that this server held it. "A server that just came
up" is the one that looks load-bearing, and it is contradicted by a fact this
shop measured earlier: **herdr restores workspaces across a restart** (it does
not re-run their commands — rangerhq-snd, the autostart hook's `--startup`
arm exists because of it). So a server answering on this socket with an empty
board is an empty board, not a server mid-re-attach.

**Why the prune keeps it.** A refusal there costs a kept file, which the next
listing takes back once a workspace exists; and the arm was born with the
`socket:` field itself, after a `--watch` pass on a scratch server read the
fleet's real `RHQ_HOME` and deleted eleven live sessions' metas in one read
(rangerhq-snd incident). Cheap belt, unrecoverable direction — it stays.

**Dropping the listing took its failure branch too.** Nothing else in
`mustNotOrphan` read the listing, and `Workspaces()` erroring was a weaker
copy of the per-id query below it: if herdr will not answer, `WorkspaceGet`
errors too, and silence on the write side is already a refusal (ADR 0011 §2).
One fewer round trip on every create.

**The invariant test now derives its exception.**
`TestPruneAndCreateAgreeOnEveryBoard` (`internal/rhq/jeu2halves_qa_test.go`)
walks the grid over (meta socket, pass socket, board empty) and asserts the
halves never disagree in the dangerous direction — prune keeps behind a socket
guard, write proceeds. Both deliberate disagreements are now one rule, computed
from the board rather than listed beside it: `mayDiffer := metaSock == passSock`.
A board where the sockets *differ* and the write proceeds is jeu2 itself, and
no widening can hide in a table row. `TestNameStaysReusableOnThisServersOwnEmptyBoard`
is un-skipped and pins both operator boards.
