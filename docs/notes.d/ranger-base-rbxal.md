# The fifth ADR 0079 D2 conjunct now carries a witness (ranger-base-rbxal)

ranger-base-fe4uq finding 1, landed: `--type=path` on the want **SYSTEM** read
in `renderHookSkipGlobalScope` (`internal/posse/hookskip.go:834`) could be
deleted with the whole shipped `TestQAHookSkip` filter green. It is the same
class as the four ranger-base-y9i3o pinned, and the fifth of five. The finding's
own record — the probe, the matrix, the reachability argument — is
`docs/notes.d/ranger-base-fe4uq.md` section 1; this fragment is what the pin
cost and what it measures.

Shipped as ADR 0079 **pin 16**,
`TestQAHookSkipSystemTildeIsTypePathResolvedOnTheWantSide`
(`internal/posse/hookskipwitness_qa_test.go`), beside pins 13-15 in the file
those three live in. `hookskip.go` is untouched — this is a pin beside the wall,
not a change to it.

## MEASURED 2026-11-06, this box (darwin 25.6.0, git 2.50.1 Apple Git-155), at main 6125eb4e

| | |
|---|---|
| the new pin, unmutated | PASS 2.82s (`-run` the pin alone, `ok 3.223s`) |
| arm 6 applied to `hookskip.go` | md5 `4b585517b267eb03b9201924c3316376`, from `58b3881b84e477eef876f9095c601ddb` — the finding's md5 byte for byte |
| `go test -count=1 -run 'TestQAHookSkip' ./internal/posse` under arm 6 | **1 FAIL, and it is this pin**, 23.189s |

The mutant's failure is the right failure: the pin's XDG leg lands
`code=1 fired=0`, refused by `hookskip:config-env` naming ADR 0079 — the
over-refusal the flag exists to prevent — and nothing else in the filter
notices. Arm 6 is

```
-	fmt.Fprintf(&b, "      %s config --system --includes --show-scope --type=path --get-regexp %s 2>/dev/null\n", q, re)
+	fmt.Fprintf(&b, "      %s config --system --includes --show-scope --get-regexp %s 2>/dev/null\n", q, re)
```

## Why the witness is an ALLOWED leg, and why it has to be

`--type=path` can only make the two sides DIFFER, so the mutant can only make
the wall refuse more. Every leg that asserts refusal therefore survives it, and
a pin built only from refusals is green over a deleted conjunct. The witness has
to be a call the wall ALLOWS today and refuses under the mutant.

That call is a move of `XDG_CONFIG_HOME` with **HOME left alone**: the D3 gate
fires (pin 2's "XDG_CONFIG_HOME at its own default" leg is the same move), both
sides then resolve the system file's `~/managed-hooks` against the same HOME,
the sides agree, and the commit lands at rc 0 with the managed hook firing.

This is also why **pin 11 is not the owner** though it plants the identical
fixture (`core.hooksPath = ~/managed-hooks` in a scratch system file, reached
through a `GIT_CONFIG_SYSTEM` git wrapper): its moved-HOME leg asserts REFUSED
and the mutant refuses too, and its allowed leg spells the system hooksPath
ABSOLUTE, where `--type=path` is the identity. Both of its directions survive.

GENERAL RULE, which is the part worth keeping past this conjunct: **a conjunct
that can only make a wall refuse MORE needs an ALLOWED leg as its witness**, so
the question to ask of a candidate pin is not "does it exercise this code" but
"does the mutant reach any of ITS allowed legs".

## The mutant rides along in the test

As pin 9 does: the pin renders the shim twice from one `renderShim` body — once
as rendered, once with `--type=path` dropped from the want SYSTEM read's flag
list alone — and asserts the same XDG leg is allowed under the first and refused
under the second.

Without that, an allowed leg is also exactly what a compare **that never ran**
looks like: any later change that stopped the D3 gate firing on an XDG-only move
would leave this pin green while pinning nothing. The in-test witness is what
makes the leg's "allowed" a statement about the compare. The replace is guarded
by a staleness check on the flag spelling, which is unique in the shim (the have
listing reads `--show-scope --type=path` with no `--system`; the candidate read
reads `--includes --file "$posse_hs_c" --type=path`).

## Severity: debt, by measurement

Both sides read the SAME system file, so the value text is identical on both;
`--type=path` can only make them differ where the resolving HOME differs, and
where it differs the compare refuses either way. Dropping the flag over-refuses
and opens nothing. What was missing was not a wall but a conjunct of one that
nothing noticed going away — the ranger-base-y9i3o reading exactly, and the
fifth and last of the five `renderHookSkipGlobalScope` conjuncts that
ranger-base-pr6p8 and ranger-base-fe4uq between them found unwitnessed.
