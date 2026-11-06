# A forged `global` line, and the control that made the bypass look like "nothing to commit" (ranger-base-ava0x)

ADR 0079 D2 counts the `--show-scope` listing against `--show-scope
--name-only` and refuses a miscount. Pin 9 of that ADR's owed list is the
process it protects, and this is the pin landing —
`TestQAHookSkipNewlineForgedGlobalLineIsRefused` in
`internal/posse/hookskipglobal_qa_test.go`, through a rendered shim on claude
and on codex.

## The forgery, MEASURED 2026-11-06, git 2.50.1 (Apple Git-155)

A LOCAL config value — an alias, which `hookSkipRegexp` matches — carrying a
newline prints as two lines in the non `-z` listing, and the second line is
indistinguishable from a real one:

```
$ git config alias.a "$(printf 'a\nglobal\tcore.hookspath %s' "$MANAGED")"
$ GIT_CONFIG_GLOBAL=/dev/null git config --show-scope --get-regexp \
    '^(core\.hookspath|hook\.|include\.|includeif\.|alias\.)'
local^Ialias.a a
global^Icore.hookspath /…/managed-hooks
$ GIT_CONFIG_GLOBAL=/dev/null git config --show-scope --name-only --get-regexp …
local^Ialias.a
```

Under that variable git reads no global file, so the managed hooksPath is
unread — yet the shim's pure-sh filter keeps `core.hookspath /…/managed-hooks`,
which is byte for byte what the want reads return from the launcher's own
files. have == want, and the commit lands at rc 0 with the hook never run.
A moved `HOME` that lacks the hooksPath is the same bypass by the other route,
and the pin carries both legs.

The count is what refuses it: a KEY cannot hold a newline, so the
`--name-only` listing's line count is an honest entry count — 1 against the
full listing's 2 above. With the count mutated away in
`renderHookSkipGlobalScope` itself (`[ "$posse_hs_gl" -eq "$posse_hs_gn" ])`
rewritten to `[ 0 -eq 0 ])`), MEASURED the same day and git: both legs land at
**rc 0 with the managed hook unfired**, on claude and on codex. The pin
carries that mutation as an in-test WITNESS — the same two legs through a
shim patched after render — so it cannot quietly degrade into the plain
`GIT_CONFIG_GLOBAL=/dev/null` refusal `hsRefused` already covers.

## The gotcha: `hsGlobalCheck` numbers its commit messages from zero

`hsRun` writes the leg's own argv into `f`, so each leg has something to
commit, and `hsGlobalCheck` spells the message `-m g<i>` from its OWN index.
Split the control and the legs across two `hsGlobalCheck` CALLS and both get
`i=0`: the control commits `commit -q -m g0 -- f`, the first leg writes the
identical bytes, and git answers **"nothing to commit", rc 1, on stdout** —
empty stderr.

That is a refusal-shaped failure that is not a refusal. The first measurement
of the mutant here read `code=1` for the `/dev/null` leg and `code=0` for the
HOME leg, and the pin went red either way, which is why it was nearly kept:
a red mutant was all the bead asked for. It was the wrong red. With the
control moved into the same call as the legs — `append([]hsGlobalLeg{control},
legs...)` — the numbering is unique and the mutant reads `code=0` on both
legs, the bypass as it actually happens.

The general shape: in this fixture a leg that reuses an earlier leg's argv
cannot commit, so **a pin whose legs are built from more than one
`hsGlobalCheck` call must not let two of them collide**, and a mutant
measurement that reads rc 1 with empty stderr should be read again before it
is believed.
