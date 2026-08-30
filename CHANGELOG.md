# CHANGELOG

Notable changes, newest first, one section per release. Write these for
someone deciding whether to upgrade, not for the person who wrote the fix.

A release's section is what `.github/workflows/release.yml` puts at the TOP of
that release's notes on GitHub, above the generated commit list —
`scripts/release-notes.sh` is the reader, `make release-notes VERSION=vX.Y.Z`
prints exactly what will be prepended. Renaming `## Unreleased` to the version
being cut is a precondition of the tag; see `docs/runbooks/release.md`.

## Unreleased

### Security

**A persona session could commit your constitution — the PIDs every future
session runs under — and nothing refused it.**

*Affected: every build before this one. Two walls added, both on by default.*

The gates already fenced the *command* (`posse promote` is denied in every
crew PID) and the *form* (an unqualified commit is refused in a shared
checkout), and under `cage: seatbelt` the constitution area was never
writable. Under `cage: shims`, which is what most personas run on, nothing
checked the *path class*: a session could rewrite `rhq/agents/*.md` and commit
it like any other file, and the next promote would put its own edits in force.

The `prepare-commit-msg` hook now carries a third arm. A commit from a session
carrying `RHQ_PERSONA` is refused when it touches `.claude/settings.json` or
`.claude/settings.local.json` in any hooked repo — that file holds the deny
list fencing the session's own destructive commands — and, in the repo whose
top level has `rhq/agents`, when it touches `rhq/agents`, `rhq/config.yaml`,
`rhq/recipes`, `rhq/skills` or `rhq/envs`. Your own shell carries no marker
and is untouched. The refusal names the paths and tells the session to stage
what it means somewhere outside the class for you to apply.

Behind it, the launcher will not fast-forward a session branch whose diff
touches those paths: it reports and leaves the branch alone for you to read
and land. That half runs in your process, so a session cannot scrub its way
past it — which the hook's arm, keyed on an environment variable, can be.
`core.hooksPath` still defeats every hook-tier gate.

Reinstall the hooks in repos you have already hooked (`posse gates
install-hooks <repo>`); a dispatch into a repo refreshes it automatically.

**...and the walls above go stale in every repo no session enters — including
the one that holds the constitution.**

*Affected: every build before this one.*

The hook bodies are compiled into the binary, so every installed hook is a
*copy* of the render that was current when someone wrote it. Only `posse gates
install-hooks` and a dispatch re-render one, and a dispatch refreshes the repo
it was cut from and no other — so a repo that never holds a session keeps
whatever it was given, indefinitely. That is how a constitution repo can run a
`prepare-commit-msg` without the arm above for hours after the arm shipped:
the wall existed exactly where sessions launch, and nowhere else.

`posse promote` and the `posse dispatch --watch` preamble now sweep every repo
`beads_visibility:` names and print the ones whose hooks are not this binary's
render — stale, foreign, or never installed — naming the repo and the command
that fixes it. Both report and rewrite nothing: a hook rewrite in a shared
checkout is a change you should type. A configured repo that is absent or is
not a git repository is skipped, not reported, and an instance with no
`beads_visibility:` block hears nothing at all. `make verify-hook-freshness`
in a posse checkout is the same question on demand.

### Fixed

**A deny rule naming a subcommand's flag — `Bash(bd sync --full:*)`,
`Bash(git push --force:*)` — only refused the flag in one position.**

*Affected: every PID carrying a rule of that shape, on every runtime.*

The PATH shim rendered a two-word rule as a test of the subcommand followed by
a test of the very next word, so any other flag in front of the denied one
moved it out of position and the command ran: `bd sync --push --full` and `git
push --tags --force` both walked past rules written to stop them. A flag has no
position — the parsers these rules describe accept it anywhere after the
subcommand — so the shim now looks for it anywhere in that subcommand's own
arguments, `--flag=value` included.

Two things it still does not treat as the flag: an option's value (`bd sync -m
--full` is a commit message) and an operand after `--`. Where the flag could be
a value the shim cannot rule out, it refuses — a rule may now stop a spelling
that is technically something else, which a respelling gets past.

Rules naming a *short* flag (`-f`) are unchanged and still matched by position;
`posse gates` reports them best-effort rather than claiming them, because a
short flag can hide inside a cluster.

Read this as a fix to *where the flag may sit*, not as a wall around what the
flag does. A flag rule still walls one spelling: `git push -f`,
`--force-with-lease` and `git push origin +main` all force-push and none of
them carries the token `Bash(git push --force:*)` names — and the last carries
no option to spell, so widening the matcher cannot reach it. Deny the verb
(`Bash(git push:*)`) wherever the effect is what must not happen; every PID in
`examples/agents` does, and posse's own tests now require it. ADR 0001 says so
in one paragraph.

**`brew install ranger360ai/tap/posse` 404s on any Homebrew older than 6.0.14.**

*Affected: the published v0.4.0 formula. Fixed in the generator; it reaches
deployers when the tap is re-rendered.*

The v0.4.0 formula states no version of its own, so brew scans one out of the
download URL — and that scan is a property of the brew on the box. Homebrew
6.0.14 (2026-07-28) added the parser that reads `0.4.0` out of
`.../releases/download/v0.4.0/…`; every brew before it reads `64` out of
`posse_0.4.0_darwin_arm64.tar.gz` instead. Since v0.4.0 ships bottles, and brew
names a bottle after the formula's version, such a box asks the release for
`posse-64.<tag>.bottle.tar.gz` and the install exits 1 on a 404.

`scripts/tap-formula.sh` now renders an explicit `version` stanza, so nothing
is scanned and no version of brew can get it wrong. On the published v0.4.0
tap the fix is `brew update` — `INSTALL.md` §2 now says so, and says how to
tell before installing.

**`brew install` still needed a developer toolchain on macOS 13 and older.**

*Affected: v0.4.0 on macOS 13 Ventura, 12 Monterey and 11 Big Sur. Fixed in the
generators; it reaches deployers with the next release's bottles and formula.*

v0.4.0 shipped one bottle per architecture tagged `sonoma`, chosen by reading
`HOMEBREW_MACOS_OLDEST_SUPPORTED` (14) as "the oldest macOS Homebrew supports".
It is not: that constant is the oldest macOS Homebrew *builds bottles for*, and
the oldest it *runs on* is `HOMEBREW_MACOS_OLDEST_ALLOWED` (10.15). brew falls
back only downwards — an older bottle pours on a newer macOS, never the reverse
— so every Mac on macOS 13 or older matched no bottle, took the
build-from-source path, and met the same fatal `Your Command Line Tools are too
outdated` gate the entry below says v0.4.0 closed. Measured against the
published tap on Homebrew 6.0.20: `brew fetch --bottle-tag=ventura` — and
`arm64_ventura`, `monterey`, `arm64_monterey`, `big_sur` — each answered
`Bottle for tag … is unavailable`.

The floor is now **11 Big Sur** on both architectures, the oldest macOS arm64
has at all, so every Mac from Big Sur up pours a bottle and none of them needs
Xcode. The asset count is unchanged: it is the same four bottles, tagged lower.
macOS 10.15 Catalina, Intel only, is what is left on the source path — Homebrew
stops supporting it from September 2026 — and `INSTALL.md` §2 now names it
instead of telling a Ventura reader their tap is out of date.

## v0.4.0

### Security

**Two endpoint environment variables handed the account's OAuth token to any
host they named.**

*Affected: v0.3.0 and every earlier build. Fixed in this release (`8a01e01`,
`0ba56cb`).*

The plan guard and the tier preflight each took their HTTP endpoint from an
environment variable — `RHQ_PLAN_USAGE_URL` and `RHQ_MODEL_LIST_URL` — with no
validation of any kind. Setting either to a URL of the caller's choosing made
posse read the account's Claude Code OAuth token and send it as
`Authorization: Bearer …` to that host. The response was then written into
`state/`, so the same override was a cache-poisoning primitive as well as a way
out for the credential: an override's plan numbers became the fact every posse
process on the instance read for the cache TTL, with no credential needed to
put them there.

**Impact is local.** Reaching this requires setting an environment variable in
the environment posse itself runs in; there is no remote or network-only
vector. What it added over reading the credential directly is that the read
happened *inside* the harness, so it left no trace in the refusal log — and it
would have outlived a re-locked credential store, after which posse is the
process permitted to read the item and arbitrary sessions are not.

The fix is `internal/rhq/credpin.go`, one rule for both readers:

- `RHQ_MODEL_LIST_URL` is **deleted**. Nothing read it but the vulnerability —
  tests inject the reader through a struct field.
- `RHQ_PLAN_USAGE_URL` is honoured only when its host is loopback **by name**,
  so a hostname that resolves to `127.0.0.1` is still refused and DNS rebinding
  cannot buy the override back. A refused override is said out loud, never
  silently swapped back for the real endpoint.
- The credential goes only to the compiled-in endpoint URL, **byte for byte**
  (an identity test, not a host test, so a near-miss spelling is an override
  too). A loopback override is asked with **no `Authorization` header** and the
  credential store is not read for it at all: the test seam keeps working,
  uncredentialed.
- An override's answer is never written to the shared plan snapshot, on the
  cooldown path as well as the success path.

**Upgrade guidance.** Upgrade to this release. On an affected build there is no
configuration workaround: not setting these variables yourself does not help,
because anything that can set them in posse's environment can obtain the token.
If either variable has ever been pointed at a host you do not control on an
affected build, treat the account's OAuth token as exposed and reissue it by
signing out of Claude Code and back in.

### Fixed

**`brew install ranger360ai/tap/posse` no longer needs a working developer
toolchain.**

*Affected: v0.3.0 and every earlier build, on macOS only. Fixed in this
release.*

The Homebrew route is advertised as "a release binary, no Go needed", and for
one class of Mac it was the opposite. The formula shipped per-architecture
tarballs and **no bottle**, so brew took its build-from-source path and ran its
*fatal* developer-tools diagnostics before it unpacked anything. On a Mac whose
Command Line Tools are behind the running macOS, the install died with `Your
Command Line Tools are too outdated` — having never read our formula, and
naming Xcode rather than us.

Releases now ship four Homebrew bottles beside the four tarballs, and the
formula carries a `bottle do` block pointing at them. brew pours the prebuilt
keg and never enters that path. A successful install now prints `Pouring
posse-<version>.<tag>.bottle.tar.gz`.

**Upgrade guidance.** Nothing to do if `brew install` already worked for you —
the binary and its contents are unchanged. If it did **not**, `brew update` and
re-run: with a tap carrying this release's formula, stale Command Line Tools
stop mattering on **macOS 14 Sonoma and newer**. brew falls back only
*downwards*, so this release's one tag per architecture covers every macOS
above Sonoma without a new release each time Apple ships one — and covers none
below it. macOS 13 Ventura and older still built from source here, and still
met the gate; that is the `ranger-base-olwk` entry above.

## v0.3.0

First tagged release. See the release notes on GitHub.
