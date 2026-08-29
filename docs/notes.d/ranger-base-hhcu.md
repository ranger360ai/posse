## Two verdicts for one history: the audit's states were numbers (ranger-base-hhcu)

`ci.yml` ran the same tree on both platforms and got two answers for the same
422 commits. macos-latest flagged one commit, ubuntu-latest flagged two — the
extra one, b26975f, "1 path(s) went backwards: cmd/posse/cockpit.go -> content
of 1fdf9da". That claim is false. The three states that path held across those
commits are three distinct blobs (`6e51571afa45…`, `6e44262db4c1…`,
`05e20b38efc9…`), and on darwin the path has no repeated state at all.

**The cause is that two of the states are valid scientific notation.**
`scripts/audit-silent-reverts.sh` piped `git log --raw` into awk and compared
the ABBREVIATED blob ids as fields. Git abbreviated those two to `6e51571` and
`6e44262`. A field — and every element `split()` pulls out of one — is a
**strnum**, and awk compares two strnums *numerically* when both look like
numbers. Both overflow `strtod` to `+inf`, `+inf == +inf`, and the detector
reported that a path went back to a state it never held.

Whether an awk does that is an implementation choice, which is the whole of
the platform split. Measured 2026-08-29, one captured raw log of this repo's
433 commits fed to four awks:

| awk | `$1==$2` for `6e51571`/`6e44262` | verdict on the real history |
|---|---|---|
| gawk 5.3.2 | 1 | b26975f **and** e82338c |
| mawk 1.3.4 (20250131, 20260302) | 0 | e82338c only |
| busybox awk | 0 | e82338c only |
| BWK awk 20200816 (darwin `/usr/bin/awk`) | 0 | e82338c only |

mawk and BWK reject an overflowed numeric string as a strnum; gawk does not.
gawk is the only one of the four that reproduces ubuntu-latest's output, over
the same history, naming the same commit and the same path. The runner's awk
was not probed directly — nobody could, which was itself the problem — so
`ci.yml` now prints `uname`, `awk --version` and `git --version` before it
vets, and the next userland split is a log line instead of an expedition.

**`make test-linux` could not have caught it.** Its image is debian-based and
its `/usr/bin/awk` is mawk (measured in `golang:1.26`), so the container agreed
with darwin and reported "431 commits, 0 untriaged" while ubuntu-latest was
red. Its "green here means green there" claim is now qualified in the script:
it rules out the *platform* split it was built for — syscalls, filesystem
semantics, `/bin/zsh` — and does not rule out two linux distributions' shell
tools disagreeing.

**Why it was P1 and not a curiosity.** Zero of 27 runs had ever been green, and
the only way to clear it was to write b26975f into `scripts/silent-reverts.allow`
— to record a triage reason for a revert that never happened, in a file whose
own header says a listed commit is "a content rollback we have LOOKED AT and
accepted". Worse in the other direction: this detector is the only cover for
the rangerhq-8rtf class, and the same coercion can **hide** a real rollback, so
on a coercing awk every clean run it had ever produced was worth less than it
read.

**The fix is two independent layers, and each has its own pin.** `raw_log()`
passes `--no-abbrev`, so states are full 40-hex rather than 7 (the
`<digit>e<digits>` shape lands about once in 270 at 7 characters; at 40 the
whole id has to cooperate). `states_awk()` forces its captured ids to strings
once, at the split, so no implementation may coerce a state at all.

One outcome assertion would have been green with either layer gone, so
`--self-test` gained a `numeric` arm with three, plus a control:

1. the fixture — three commits over one path whose first and third blobs
   abbreviate to `7e15992…` and `5e00413…` — must scan clean;
2. `raw_log` must emit only 40-hex ids (reverting `--no-abbrev` reds this and
   nothing else);
3. `states_awk`, fed a **synthetic** stream whose ids are `0000100` and
   `00001e2`, must stay quiet (reverting the `""` reds this and nothing else);
4. the same rig with a genuine repeat must fire, or (3) is an absence nobody
   proved could be a presence.

(3) is synthetic on purpose. The `+inf` collision needs an awk that takes an
*overflowed* string as a strnum, and only gawk does — so an arm built on the
fixture's real ids is undiscriminating on darwin, mawk and busybox, which is
exactly the blind spot that let this ship. `0000100` and `00001e2` are both
plainly the number 100, no overflow involved, and all four awks compare them
equal. Measured: dropping the `""` reds arm (3) under all four.
