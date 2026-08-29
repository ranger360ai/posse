## The chain prescription, re-read at a slot posse had already chained (ranger-base-q32o)

`posse gates install-hooks` refuses a slot another tool owns and prints a
block to paste. Step one of that block was a bare `mv <slot> theirs-<slot>`.
It assumes the slot holds a FOREIGN hook. At a slot posse has already
chained, the slot holds posse's own dispatcher and `theirs-<slot>` holds the
third party's hook, so the paste did two things and said nothing about
either:

1. **destroyed the third party's hook** — `mv` over an existing file is
   silent;
2. **built a self-exec loop** — the new `theirs-<slot>` was the old
   dispatcher, whose last line is `exec "$d/theirs-<slot>" "$@"`. Itself.

The loop sits **past the gate's refusal**. A refused push exits before the
`exec`, so the wall still refused correctly and every probe still read 1/1;
only a PERMITTED push reached the loop, and then it spun with no output and
no exit. That is why it survived the close of rangerhq-pon3, whose probes all
run with a deny list in the environment.

**How the precondition was reached, entirely by posse's own instructions**
(measured, macOS 25.4, git 2.50.1, binary from 66f47ee):

| # | step | result |
|---|---|---|
| 1 | foreign `pre-push`, `posse gates install-hooks` | refuses, prints the prescription |
| 2 | paste it | correct chain — slot, `posse-pre-push`, `theirs-pre-push` |
| 3 | `rm posse-pre-push` | the documented uninstall: that marker line lives in `posse-<slot>` in a chained repo, not in the slot |
| 4 | run the slot | exit 127, loud — deliberate, not the bug |
| 5 | `posse gates install-hooks` again | **refuses**, prints the prescription again |
| 6 | paste it | exit 0, no warning; `theirs-pre-push` is now the dispatcher |
| 7 | permitted push | **never returns** (killed at 10s) |

Step 5 is the whole trap: `installHook` only refreshed a chain whose
`posse-<slot>` was present and marker-owned, so with it gone it fell through
to a Die that calls posse's own byte-for-byte dispatcher "exists and is not a
posse hook". The operator's natural repair was sent to the block that breaks
the repo.

**Fixed in two independent halves**, either of which cuts the chain of events:

- `installHook` now tells three situations apart behind an intact dispatcher:
  `posse-<slot>` present and ours → refresh (unchanged); **missing → restore**
  (the dispatcher matched our own render byte for byte, so the file it runs
  first is ours to write and there is nothing there to overwrite); present and
  foreign → refuse, name that file, and print **no** prescription, because
  re-chaining would bury it.
- `chainNeighbourName` picks the prescription's `mv` target against the hooks
  directory instead of assuming `theirs-<slot>` is free, and the block says
  why the name differs when it is not. A free name destroys nothing, and
  `chainDispatcherNeighbour` accepts any plain sibling filename, so the chain
  the paste builds is recognized and refreshed like any other.

`internal/rhq/chainrepair_qa_test.go` pins it by running the hook files the
way git does, on a 20s context budget so a loop fails the suite instead of
hanging it, and builds every fixture out of the prescription posse actually
prints — a pin whose fixture is a copy of the thing under test measures the
copy. All three tests were measured red against the unfixed `gates.go`.

**Not fixed here:** the block's third step, `mv <slot> posse-<slot>`, can
still clobber a `posse-<slot>` that is foreign. `posse-<slot>` is fixed by
name — the recognizer reads exactly that path — so the free-name trick does
not apply, and no documented posse instruction creates a foreign one.
Filed as ranger-base-hd56 (P3).
