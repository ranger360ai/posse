# The door's second ctime decision, and the triage line that is not mine (ranger-base-z9aff)

*2026-11-11 · jian-yang · the code-lane half of ranger-base-8do8p's two live
findings. FINDING 2 is here. FINDING 1 is handed to gilfoyle as
ranger-base-n8wek, because its deliverable is a statement of intent about
someone else's commit.*

## FINDING 2 — the warm/cold dedup key (done)

`cmd/coldexec/door_linux.go` reads `st.Ctim` **twice**, one line apart, for
two different purposes, and until now only the first reading had a witness.

- the **epoch test** (is this inode newer than the door?) — ARM 12,
  `TestAPlantWhoseMtimePredatesTheRunStillPaysD`, added by ranger-base-o57rc.
- the **dedup key** `inode{dev, ino, csec, cnsec}` (is this the first exec of
  this inode-at-this-version, or a repeat?) — no pin at all.

Mutating the key's two fields to `st.Mtim` left all twelve pins green. It is
not an equivalent mutant: the key is reached only after the epoch test
passes, so the two readings can part company solely on a **second exec of one
inode whose ctime moved and whose mtime did not** — a metadata-only change,
the `chmod +x` half of the comment the door already carries, happening one
exec later. Every other plant in the file either predates the door or moves
both stamps together (`cp`, `cp -p` onto a fresh inode, ARM 6's in-place
rewrite), and ARM 12 holds the mtime back but execs each plant once.

MEASURED 2026-11-11, this box (linux 7.2.15 x86_64, go1.27.3), at HEAD
d68b0179, two doors built from the same tree, `door_linux.go` md5
`147a7cb3728e63719fc71c8870cef548` before and after every mutation:

```
POSSE_COLD_EXEC_DELAY=1.5s ./door-$d -verbose -- /bin/sh -c \
  'printf "#!/bin/sh\necho ran\n" > p; chmod +x p
   tm first ./p
   chmod 0750 p
   tm chmodded ./p'

door keyed on   | exec 1 (fresh plant) | exec 2 (after chmod 0750)
----------------|----------------------|--------------------------
ctime (shipped) | delayed  1503ms      | delayed  1502ms
mtime (mutant)  | delayed  1503ms      | WARM        3ms

shipped: "9 execve notifications, 2 cold first execs delayed 1.5s each
          (0 warm, 7 older than the door, 0 absent)"
mutant:  "9 execve notifications, 1 cold first execs delayed 1.5s each
          (1 warm, 7 older than the door, 0 absent)"
```

Two execs are enough because the plant is made INSIDE the run, so exec 1 is
already cold; holden's three-exec recipe needed the extra step because its
plant predated the door and exec 1 read `old`. Same conclusion, one step
cheaper, and it fits `runDoor` unchanged.

The cost direction is the one ADR 0085 D5 exists to prevent: a **missed**
injection, so a test that would flake on ci.yml's macos-latest leg passes
here.

### What landed

- `cmd/coldexec/supervisor_linux_test.go` — **ARM 13**,
  `TestAReExecAfterAMetadataOnlyChangeIsColdAgain`: exec a fresh plant,
  `chmod` it, exec it again, assert BOTH execs delayed. Additive; no pin
  censuses this file's arm count, so nothing else moved.
- `cmd/coldexec/door_linux.go` — the sentence the finding asked for beside
  the key, saying it is a second decision and naming its witness.

### The two decisions have disjoint witnesses, which is the point

Both directions, `go test -count=1 ./cmd/coldexec` (1.3-1.4s, no suite slot),
mutant source restored byte-identical each time:

| mutation | reds |
|---|---|
| `key` `st.Ctim` → `st.Mtim` | ARM 13 only — `1 cold execs delayed, want 2` and `chmodded took 1ms, want at least 400ms` |
| epoch test `st.Ctim` → `st.Mtim` | ARM 12 only |
| none (control) | nothing — ok 1.344s |

So ARM 13 is not re-witnessing the epoch test under another name. A future
"simplify the key to mtime" now reds, and reds alone.

`make coldexec-check` is scoped to `./internal/posse` (Makefile:1135-1138),
so this change is outside that door — and a door under a door refuses by
ARM 8 anyway.

## FINDING 1 — handed off, not done (ranger-base-n8wek)

`scripts/audit-silent-reverts.sh --quiet` still exits 1 on main, so
`make test` and `make test-arm1` exit 1 for every seat over a green suite
(the audit is the last recipe line of both, Makefile:335 and :341).
Re-measured here at d68b0179: `f1f9a2f ... 1 untriaged silent revert(s)`,
2472 commits scanned, 0.2s.

The fix is one line in `scripts/silent-reverts.allow`, and that file's own
convention is that the reason is written by the persona that made the commit
(the c578cb8 convention, ranger-base-u20t) — f1f9a2f5 is gilfoyle's, under
ranger-base-o8us6. Writing it myself would be reconstructing someone else's
intent, which is the one thing the file refuses; a glob or a generic reason
was already asked for and rejected under ranger-base-ztzd. So it is filed
`-a gilfoyle` at P1 with the format note's three traps (no bare 40-hex word
at the head of the reason, one line per sha amended not appended, the ADR
0054 D1 token inert beside a landed sha) and acda116 named as the worked
example of the same shape.

The deletion itself was right and is not in dispute — holden restored the pin
at f1f9a2f5 and it reds with its own HOLE-CLOSED message. Nothing of
ranger-base-o8us6's code is at fault.
