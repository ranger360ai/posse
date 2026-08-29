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
stop mattering. One tag per architecture at the oldest macOS Homebrew supports
covers newer macOS too, so this does not need a new release each time Apple
ships one.

## v0.3.0

First tagged release. See the release notes on GitHub.
