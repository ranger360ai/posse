# The channel suffix is grok's update-check cache, so §6c compares the release (ranger-base-3a5z7)

`internal/posse/runtime.go` declared grok's `SkillsVerified:
"grok 1.0.52 (0920c8169e27) [stable]"` (landed 2026-10-31 by
ranger-base-9f2o6), and ADR 0007 §6c compared that string against
`ProbeCLIVersion`'s read with `==`. The trailing field is not part of the
binary's identity, so the declaration was right on exactly the seats whose
`$HOME` happened to carry one file.

## MEASURED 2026-10-31, this box, darwin 25.6.0, one grok binary

`grok 1.0.52 (0920c8169e27)`, Homebrew cask `grok-build`, pinned by
`~/.grok/config.toml` (`auto_update = false`, `maximum_version = "1.0.52"`),
so every reading below is the same binary every launch here uses.

The bead's repro, three `$HOME`s:

| `$HOME` | first stdout line of `grok --version` |
|---|---|
| the operator's | `grok 1.0.52 (0920c8169e27) [stable]` |
| a fresh `mktemp -d` | `grok 1.0.52 (0920c8169e27)` |

## Which file: bisected one at a time into an empty `$HOME/.grok`

The bead left this open — "which file is unmeasured and is this bead's first
question" — and it is answered by copying candidates in one at a time against
the same binary:

| `$HOME/.grok` contains | `grok --version` |
|---|---|
| nothing | `grok 1.0.52 (0920c8169e27)` |
| `version.json` | `grok 1.0.52 (0920c8169e27) [stable]` |
| `config.toml` only | `grok 1.0.52 (0920c8169e27)` |
| both | `grok 1.0.52 (0920c8169e27) [stable]` |

It is **`~/.grok/version.json`**, and it is grok's own update-check cache:

```json
{
  "version": "1.0.70",
  "stable_version": "1.0.70",
  "checked_at": "2026-10-31T09:06:21.57052Z"
}
```

Two things follow. First, the bead was right that `config.toml` is not the
source — it has `auto_update`, `maximum_version` and
`required_maximum_version` and no channel key, and on its own it produces no
suffix. Second, the suffix is worse than `$HOME`-dependent: it is *cache*-
dependent. The file is a record of the channel's newest release — 1.0.70 — and
not of the 1.0.52 that printed the line, it is rewritten by grok whenever it
next checks (the `checked_at` stamp moved again while this bead was open), and
deleting it changes this box's own `--version` string with no binary change at
all. A seat whose grok has never run an update check never has it.

## Why dropping the suffix is not, by itself, the fix

The bead recommended declaring the `$HOME`-independent line and leaving the
compare alone. `skillsVerifiedDrift` (parity.go) is an exact compare and is the
single decider — `skillsdrift.go` reuses its sentence rather than re-comparing
— so under an exact compare there is no string that is right for both
populations:

| declared | the operator's HOME | a fresh HOME |
|---|---|---|
| `grok 1.0.52 (0920c8169e27) [stable]` | ✓ | FINDING |
| `grok 1.0.52 (0920c8169e27)` | FINDING | ✓ |

MEASURED before the change, `go run ./cmd/posse runtime drift` in this
worktree: grok was the one green cwd-root row on the box
(`✓ grok skills_verified: "grok 1.0.52 (0920c8169e27) [stable]" and grok
reports the same`), with bob's 2.0.5 → 2.0.7 the one finding. So option 1 on
its own would have moved the permanent red off a container tier and onto the
box that runs `scripts/verify-box.sh`'s roster row — the same defect with a
worse victim, since that is the seat that reads the roster.

## What landed: both halves

- **The declaration is the release.** grok's `SkillsVerified` is
  `grok 1.0.52 (0920c8169e27)`, with the comment above it saying the omission
  is deliberate and what the suffix actually is.
- **The compare is on the release too.** `skillsVersionRelease` (parity.go)
  drops a trailing bracketed field from *each* side before comparing. The
  drift sentence still names both raw readings, because what a reader has to
  act on is what the box printed.
- **The tick stays honest.** `ReportSkillsDrift`'s ✓ line used to say
  "`and <runtime> reports the same`". Where the two strings differ only by the
  field, it now prints both and says the field is box state — a tick that
  claimed two different lines were the same line would be the next reader's
  wrong turn.

It is one rule and nothing else pays for it: codex prints `codex-cli 0.153.4`
and bob prints a bare `2.0.5`, so for them `skillsVersionRelease` returns the
raw line byte for byte. A real release move still reds with the field present
or absent — the version and the build hash are both ahead of the bracket. What
stops being drift is a channel change alone, and §6's subject was always the
release whose skill discovery was measured.

Verified end to end on the same binary, with the fix in:

```
$ posse runtime drift                 # the operator's HOME
  ✓ grok   skills_verified: "grok 1.0.52 (0920c8169e27)" and grok reports
           "grok 1.0.52 (0920c8169e27) [stable]" — the same release, the
           trailing [field] is box state and not the binary (ADR 0007 §6c)
$ HOME=$(mktemp -d) posse runtime drift
  ✓ grok   skills_verified: "grok 1.0.52 (0920c8169e27)" and grok reports the same
```

bob stays the one finding in both, which is the real drift and the exit 1 the
roster row is supposed to carry.

## The pins, and the two that did not hold the first time

`internal/posse/skillsdrift_test.go`:

- `TestSkillsVersionReleaseDropsTheChannelFieldAndNothingElse` — a table where
  each row is the only case that can answer one guard.
- `TestSkillsDriftReadsThroughTheChannelSuffixAndStillRedsOnTheRelease` — the
  walk in both directions (declared bare / installed with the field, and the
  reverse, which is the state a declaration taken on a box that HAD the field
  leaves a container seat in), plus three installed lines that must still red.
- `TestNoBuiltinDeclaresASkillsVerifiedCarryingBoxState` — the declaration side,
  swept over `builtinRuntimes` rather than pinned on grok, with a positive
  witness that the sweep had subjects. The next `skills_cwd:` built-in declared
  off a live `--version` line is where this bites again.

Six mutations, each applied alone and reverted:

| mutation | first run | after |
|---|---|---|
| exact compare restored (pre-fix behaviour) | FAIL | FAIL |
| grok declares the field again | FAIL | FAIL |
| `HasSuffix(v, "]")` guard removed | **ok** | FAIL |
| inner-`]` guard removed | FAIL | FAIL |
| `i <= 0` relaxed to `i < 0` | **ok** | n/a — the arm was unreachable |
| the honest ✓ line removed | FAIL | FAIL |

Two were ORDERS.md's own shapes and are worth naming:

- **The fixture's shape covered for the branch.** With `HasSuffix` removed,
  `cli [1.0] stable` still came back unchanged — the inner-`]` check answered
  it, because the slice it tests happens to contain a `]` for that input. The
  case only one guard can answer is an *unclosed* bracket, `cli 1.0 [stable`,
  which no other check sees and which without `HasSuffix` returns `cli 1.0`.
  Added.
- **An assertion that cannot fail on the path it runs.** `i <= 0` was guarding
  `i == 0`, a line beginning with a space before `[` — unreachable, because
  the function `TrimSpace`s first. Rather than invent a fixture for a state
  that cannot occur, the arm is gone: `i < 0`, which the `[stable]` row pins
  (removing it panics on `v[:-1]`).
