## A gate that was never overwritten, never deleted, and never ran again (ranger-base-2msgj)

bd's hook install takes the two slots posse's L3 gates want and does not
destroy what it finds: it renames the slot aside and plants its own shim.
The wall is intact on disk, a filename away, and off the commit path. Every
signal posse had for that state before this bead was the one it has for an
employer's husky install — "foreign hook, posse cannot vouch for a hook it
did not write" — which is true, useless, and the wrong instruction: here the
gate posse would install is already written, correct, and one flag from
running again.

### What bd does, and what of it is measured

From the operator's work-box shakedown (github.com/ranger360ai/posse issue
#2, ranger-base-x4g3h, 2026-10-08), MEASURED there against **bd 0.50.3** —
the version this box is pinned at on purpose, the last 0.50.x before 0.51
goes Dolt-native (ranger-base-6ulbg):

- hook install writes bd's thin shims into `pre-commit`,
  `prepare-commit-msg`, `pre-push`, `post-checkout` and `post-merge`;
- it renames posse's existing gates to `*.backup` first;
- it prints nothing about either half;
- `posse gates <p>` then read `L3 … foreign` for both posse slots, and
  `posse status` said nothing needed a human (its own bead, issue #3).

Two facts come from the shipped binary rather than from a run, because every
crew PID denies `Bash(bd hook install:*)` and `Bash(bd hooks install:*)`
(ADR 0015 §3) and this bead was done from a crew seat. MEASURED 2026-10-09
over the `bd` 0.50.3 binary on this box (`strings`): the literal `.backup`
sits in the same string region as `pre-commit`, `post-merge` and
`# bd-shim`, and a second rename spelling ships beside it —
`Mode: chained (existing hooks renamed to .old and will run first)`. So bd
has at least two names for a displaced hook, `--force` ("Overwrite existing
hooks without backup") has none, and which one a repo got is a detail of a
tool posse does not control. The operator's ruling on this bead is that
posse designs around bd's behaviour and files nothing upstream — a 0.51 fix
would never reach this pin.

### The decision

**Detect by content, not by suffix.** The new `l3Displaced` verdict
(`internal/posse/gates.go`) fires when bd's own shim marker `# bd-shim v1`
holds the slot *and* `displacedPosseHook` finds, beside it, a regular file
whose name begins with the slot's and whose body carries that slot's posse
ownership marker. Nothing keys on `.backup`, so bd renaming to `.old`, or to
anything else next year, is already handled; `<slot>.sample` carries no
marker and `posse-<slot>` does not begin with the slot name, so neither is
read as a displacement.

**And only where the remedy is real.** `install-hooks --chain` takes over
bd's known shim shape and refuses every other foreign hook (rangerhq-mgdk),
so a foreign hook that is *not* bd's stays `l3Foreign` with the old line even
when a displaced gate is sitting beside it. A line that prescribed `--chain`
over a husky hook would be worse than the anonymous one it replaced: the
operator would type a command that refuses. Likewise bd's shim with no
displaced gate beside it (`--force`) stays `l3Foreign` — naming a file that
is not there sends the reader looking for it.

The consequence clause stays `commitGuardWorstCase`. The displaced body is
readable, so `l3GuardGap` *could* diff it — and the answer would be a lie by
construction: nothing in that file runs, so every guard it carries is lost
exactly as if it had never been written.

### Where it says it

- `posse gates <persona>` and every other Degraded reader (the hook-wall
  sweep, the launch pre-heal, the queue check) — one `l3DegradeLine` case,
  so they all got it at once.
- `posse gates install-hooks` without `--chain` now refuses bd's shim by
  name, names the flag, and names the file the gate went to. The pasteable
  prescription still follows it unchanged, which is what
  `cmd/posse/gateschain_qa_test.go` extracts and runs.
- INSTALL.md §9: the step after **any** `bd hooks install`, `bd init` or
  `bd import` — the last two run the install themselves, which is why "bd
  first, posse second" was never a step an operator could take once.

### Pins

`internal/posse/bddisplace_qa_test.go` (arm 3). MUTATION-CHECKED 2026-10-09:
making `displacedPosseHook` return "" unconditionally reds
`TestQADisplacedGateIsNamedWithItsRemedy` (both suffix arms) and
`TestQAInstallRefusalOverBdsShimNamesTheFlag`, and the failure output prints
the pre-fix line — "foreign hook, posse cannot vouch for a hook it did not
write" — over a repo whose gate was two feet away.
