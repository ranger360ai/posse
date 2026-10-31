# The session record names its multiplexer, and one method ADR 0076 §2 does not list (ranger-base-q5gc2)

2026-10-31. ADR 0076 §5's half of the slot: the posse-owned meta file gains a
`mux:` key, its two opaque ids become the Seat's for whichever occupant wrote
them, and liveness for a non-herdr seat is absence from that occupant's
listing. No behaviour changes on a herdr instance — every record on this box
reads and rewrites exactly as it did, which is the point of the key being
ABSENT rather than stamped.

Three things are worth a reader's time: one method that is not in §2's table,
one predicate that had to be generalized before the new one could be written,
and what §5 does not reach yet.

## 1. `Mux.Server() MuxServer` — a twelfth method, and §5 is not satisfiable without it

§2 declares ten verbs; `Start` was an eleventh (ranger-base-csnzn, §1 of that
fragment). This is a twelfth, and it is not a verb at all: it is who is
answering them.

§5 says three things about the record that only the occupant can answer:

- the meta gains a `Mux` key — the occupant's NAME;
- `Socket` holds the Rex server address — the occupant's SERVER;
- `Gen` stays empty on Rex — the occupant's GENERATION, or its absence.

posse read the second and third as `SocketID()` and `ServerGen()`, two
package functions that resolve a **herdr** api socket path and its
device/inode/bind-time. Called on a Rex instance they would stamp a herdr
socket into a record created against a Rex server, and the prune would then
compare that record's address against a herdr path forever — a mismatch on
every pass, so the record would be kept, unlistable and unprunable, with a
refusal that names two herdr sockets and no reason anyone could act on.

So the triple is asked of the slot: `MuxServer{Name, Addr, Gen}`, one call
where the backend made two. `HerdrMux.Server()` returns
`{MuxHerdr, SocketID(), ServerGen()}` — the resolvers stay exactly where they
are, because they are herdr's own and a non-herdr record must not carry what
they return. The values on a herdr instance are unchanged, which is why
nothing about this box's records or prune moved.

The alternative was a per-occupant `Name()` plus leaving the two resolvers in
the generic paths, which is the bug above with a label on it.

## 2. `cannotAnswerFor` asks the multiplexer before the address

The arm is new and it is the same question one layer out: a record's `socket:`
is a string inside ONE occupant's namespace, so two occupants' addresses are
not comparable at all. Comparing them answers "a different server" where the
truth is "a different kind of server, whose listings say nothing about this
seat either way".

It happens to refuse today on the address anyway — a Rex address does not look
like a herdr socket path — and that is exactly why the arm needs a pin rather
than an argument. `TestQAAHerdrPassCannotAnswerForANonHerdrRecord` carries a
second record whose `socket:` is this pass's own, so the comparison behind the
arm has nothing to say. MUTATION-CHECKED: delete the mux arm and that record
is **pruned** by a herdr pass and its name taken — the rangerhq-snd damage
shape, reached through a multiplexer instead of a scratch server.

## 3. The identity fence is herdr's, and a non-herdr seat must not be asked it

`notOurWorkspace` is the rangerhq-yt1p fence: an id that answers proves a
workspace holds it, never that it is the one the record meant, because herdr's
allocator is `max(live id)+1` recomputed at every server process start. Its
two anchors are the LABEL and the server GENERATION, and neither works alone —
within one generation a label mismatch can only be a rename.

Rex's ids are UUIDs and are never reused (§5), so there is nothing to fence:
a present id is the seat the record meant. Which means the label half must not
be asked either. Alone it is precisely the reading the fence's own doc rejects
— a rename read as a recycle — and here it would cost worse than a kept file:
a live session withheld from its own name for as long as its seat is renamed,
with no generation that could ever clear it. So the predicate returns "" for a
non-herdr record, which is the answer that claims nothing.

An occupant that DOES recycle ids needs a fence of its own, in its own terms,
and has to say so there.

## 4. `seatEvidence` takes its own listing, like herdr's per-id query does

§5's liveness rule is one listing: dead when the recorded session id is absent
from that server's. Both callers already hold a listing — `listSessions` read
one a moment ago, `reclaim` is re-proving under the launcher lock — and that is
exactly the snapshot rangerhq-9nso says may not be read as a fact about
another store. herdr's half confirms with a fresh per-id query; §2's ten verbs
have no per-id query, so the confirmation is a fresh `List()`. The pin asserts
the call COUNT for that reason (`stub.lists >= 2`): a prune that acted on the
snapshot it was handed would pass every other assertion.

`emptyBoard` is worn on both occupants. Its literal "herdr" became the
occupant's name, so herdr's line is unchanged to the byte.

## 5. What §5 does not reach, named out loud

- **`HerdrMeta.Workspace`/`.Pane` keep their Go names and their YAML keys.**
  The keys cannot move: "an existing meta file round-trips byte-identical" is
  this bead's acceptance test, and the keys are what is on disk. The Go field
  rename is the same mechanical class as ADR 0076 §1's `HerdrBackend` →
  `Backend`, which that section defers for the same reason, and it belongs
  with it. What this bead did instead is make the pairing addressable —
  `HerdrMeta.Seat()`, beside `HerdrSession.Seat()` — and say in the fields'
  own doc that they are the Seat's two halves in whichever occupant's ids.
- **`holdsRecorded` (relaunch.go) still reads herdr's raw listing**, so a
  relaunch on a non-herdr instance fails closed: `b.workspaces()` is §4's and
  refuses, the pass falls to `clearDeadMeta`, and `mustNotOrphan` then refuses
  honestly on the mux arm above. It is relaunch's AIMING question ("which seat
  is this session's"), never a liveness proof, and its own doc says so. It is
  the Rex occupant's to convert or to leave refusing.
- **The ordering csnzn flagged is still open.** That fragment's §1 noted that
  Rex's blocks take `{command, cwd}`, so on Rex the meta cannot be written
  between the create and the launch the way `startPlanned` writes it today
  (the `rangerhq-v52t` orphan rule). This bead stamps the triple from the
  occupant that just created the seat, which is correct in either ordering,
  but it does not decide the ordering. That is the Rex occupant's bead
  (ranger-base-qlxb5).

## Pins

`internal/posse/muxmeta_qa_test.go`, arm 1. Five tests, all five
MUTATION-CHECKED against the line they are about:

| mutation | reds |
|---|---|
| `MuxName()` returns the raw field | no-key-means-herdr |
| `writeMeta` emits `mux:` unconditionally | byte-identical rewrite |
| `idEvidence` loses its non-herdr branch | absent-from-listing is death |
| `notOurWorkspace` fences a non-herdr seat | live seat kept and listed |
| `cannotAnswerFor` loses its mux arm | a herdr pass cannot answer |

The stub occupant embeds the `Mux` interface as a nil value, so a verb §5 does
not name panics by method name instead of answering a zero value. That is
deliberate: a prune reaching for a verb nobody wrote down should be a crash in
a pin, not a reading.
