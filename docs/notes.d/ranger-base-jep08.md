# The hook-skip and git-scope pins read against git 2.56 (ranger-base-jep08)

MEASURED 2026-11-07. Two venues, named on every row below:

- **D1** — the ADR 0080 D1 Linux host, as its own user over the tailnet:
  Linux x86_64, kernel 7.2.15, **git 2.56.0**, bash 5, no Go toolchain.
- **laptop** — this box: darwin 25.6.0 arm64, **git 2.50.1** (Apple
  Git-155), go1.26.5, at HEAD db5a7138.

ADR 0080 D1's rule is that a git update which breaks posse's git-scope pins
is held back with the package manager's ignore list, never patched around.
Every one of those pins was measured on git 2.50.1 or against the operator's
2.54 report; the D1 host arrived at 2.56.0 (ranger-base-yyq64, monica's
read-only survey). This is the reading that says whether anything has to be
held back.

**VERDICT: nothing does. All 51 arms pass on git 2.56.0 / Linux, two of them
for the first time anywhere.** No pin was patched to suit; two were added,
for a direction 2.56 makes live and nothing could see before.

## How the arms were run on a host with no Go

The bead allowed "on the host itself once the toolchain lands, or via a
pinned git build". Neither was needed. The arms are `go test` binaries and
Go cross-compiles, so:

    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go test -c ./internal/posse
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go test -c -tags posse_arm3 ./internal/posse

Both binaries plus the tracked tree (`git ls-files | tar`, 26 MB, no `.git`)
go to one `mktemp -d` on the host, and each binary runs **from
`<tree>/internal/posse`** under a `-test.run` filter. The cwd matters: five
arm-3 pins read their own sources (`inletpin.go`, the test file,
`../../etc/claude/managed-settings.d/10-posse-inlet-pin.json`), and run
outside the tree they fail `no such file or directory` — a repo-shaped
failure that looks exactly like a git-version finding until you read it.
That is the one trap in this method, and it is the reason every reading
below is paired with the same filter run on the laptop.

Nothing was installed on the host and nothing outside that mktemp dir was
written. It is **not** a substitute for the toolchain landing (step 6 of
ranger-base-yyq64): a cross-built binary cannot run `make test`, the
tree-wide doors, or anything that needs `go` at run time.

## The arms, both venues

| arm | filter | laptop (2.50.1) | D1 (2.56.0) |
|---|---|---|---|
| hook-skip + parity (arm 1) | `^(TestQAHookSkip\|TestQAPIDDeny\|TestHookParity\|TestParseGitHookEntries\|TestSeatGitPin)` | 30 pass, 1 skip | **31 pass, 0 skip** |
| system/global scope (arm 3) | `^(TestQATheGitConfig\|TestQAGitGlobalScope\|TestQAChooseGitGlobalScope\|TestQAResolveGitSystemScope\|TestQAGitSystemScopePath\|TestQATheInletPinCovers\|TestQATheFourthArm\|TestQASystemScopePathsNamed\|TestQAEveryRegionAccounts\|TestQARegionsNotAccountingFor\|TestQAEveryCostDisclosure\|TestQATheCostMisScoping)` | 19 pass, 1 skip | **20 pass, 0 skip** |

The two that ran for the first time are the two whose subject is a
config-based hook, inert below git 2.54 and therefore skipped on every box
posse has ever run on:

- `TestHookParityConfigHookMissingIsRed` — written against the operator's
  2.54 report and marked ASSUMED there (githookparity_test.go). It is
  MEASURED now: a system-scope `hook.scan.command` missing from the seat's
  view reads red.
- `TestQATheGitConfigSystemPinCarriesAConfigBasedManagedHook` — added by
  this bead, below.

What did NOT move, which is the whole of why the pins hold: `git var
GIT_CONFIG_SYSTEM` still names one path and still exits 1 under
`GIT_CONFIG_NOSYSTEM`; `git var GIT_CONFIG_GLOBAL` still prints exactly two
lines, XDG first; and `git config -z --show-scope --show-origin
--get-regexp` still emits `scope NUL origin NUL key LF value NUL`, the shape
`parseGitHookEntries` was measured against on 2.50.1.

## What 2.56 adds, and the two pins it needed

git ≥ 2.54 reads **config-based hooks**: `hook.<name>.command` plus
`hook.<name>.event`, with `hook.<name>.enabled` and `hook.<event>.enabled`
switching them off. ADR 0052's rule — posse never causes the managed hook to
be skipped — does not distinguish that shape from a `core.hooksPath`
directory, but **every measurement of the rule had been taken against the
directory shape on a git where `hook.*` did nothing.** Two gaps followed, and
both are now closed. `docs/adr/0052-config-based-hooks.probe.sh` is the
instrument; its EXPECT column is these readings.

### 1. The system-scope row carries a config hook (new pin)

MEASURED on D1, from the probe, with a managed scanner carried **only** as
`hook.scan.*` in system scope:

| | cfg hook | dir hook |
|---|---|---|
| system scope carried (the landed `GIT_CONFIG_SYSTEM` row) | fires | — |
| system scope emptied (the retired `/dev/null` row) | **silent, rc 0** | — |
| carried + a command-scope `core.hooksPath` (the ADR 0052 D2 redirect) | fires | — |

So the row's two values have the same meaning for `hook.*` as for
`core.hooksPath`: carried, the employer's scanner runs; emptied, it is
skipped with nothing said on either stream. `TestQATheGitConfigSystemPin‑
CarriesAConfigBasedManagedHook` (inletpingitconfig_qa_test.go) is those
three rows, skipping below 2.54.

The third row is the one with no other guard. The ADR 0052 D2 redirect **is**
a command-scope `core.hooksPath`, and with both shapes carried it does
replace the hooks DIRECTORY — `cfg=yes dir=no`, which is what the parity
row's redirect arm exists for. It does not reach a config hook. If a later
git let it, every seat on a managed box would skip the employer's scanner at
rc 0 and nothing else here would notice.

### 2. `.enabled` is a hide direction, and the prefix already refused it

MEASURED on D1, both shapes carried:

| typed | cfg hook | dir hook |
|---|---|---|
| command-scope `hook.pre-commit.enabled=false` | **silent, rc 0** | fires |
| command-scope `hook.scan.enabled=false` | **silent, rc 0** | fires |
| repo-local `hook.pre-commit.enabled=false` | **silent, rc 0** | fires |
| that local key removed again (control) | fires | fires |

Three spellings that skip a managed scanner at rc 0 on 2.56 and do nothing
at all on 2.50.1 — and the man page advertises the third for exactly this
("particularly useful when a hook is defined in a system or global config
file and needs to be disabled for a specific repository").

**The hook-skip arm already refuses all three**, because it judges the key
PREFIX `hook\.` and not a list of spellings (`hookSkipRegexp`,
internal/posse/hookskip.go) — foresight that landed with ranger-base-r3pri
when these keys were inert. No arm moved to admit them. Four legs were added
to `hsRefused` (hookskip_qa_test.go) anyway, because the direction they name
did not exist when the prefix was written, and a prefix nobody has tested
against a live key is a claim, not a reading.

One correction to git's own docs, MEASURED: `hook.<event>.enabled=false` is
documented as "no hooks fire for that event, regardless of any per-hook
settings". It stopped the configured hook and left the `core.hooksPath`
directory hook running. The directory shape is not reachable through that
key, so the existing `core.hooksPath` pins remain sufficient for it.

## Residuals, stated

- **Linux and 2.56 are measured together, not separately.** Every D1 reading
  is Linux-and-2.56; no 2.56 reading was taken on darwin and no 2.50.1
  reading on Linux. Nothing here attributes a result to one of the two. The
  arms all pass in both venues, so the question does not arise today; it
  would if one ever failed.
- **The probe cannot run inside a seat.** Every row sets
  `GIT_CONFIG_SYSTEM`, which is what the hook-skip arm refuses, so from a
  crew session every row comes back `refused by posse gate ... rule:
  hookskip:config-env` and the shim re-pins the global row under it
  (MEASURED, laptop). That is the arm working. The probe says so in its
  header and is run as the host's own user, like ADR 0080's cage probe.
- **2.54 and 2.55 are untested.** The skip threshold is 2.54 because that is
  where the keys appear in git's docs; the only live git measured is 2.56.0.
- The arms run here are the hook-skip, parity and git-scope arms. Whether
  the rest of `make test` passes on Linux is ranger-base-offid's shakedown,
  and needs the toolchain.
