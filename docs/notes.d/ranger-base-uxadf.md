# The hash spelling, as a census rather than a word ban (ranger-base-uxadf)

MEASURED 2026-11-09, this tree at main 196ecca9, darwin 25.6.0 arm64.

ranger-base-sydnc was `shasum` not being on the cutover host's PATH. Arch —
what ADR 0080's always-on Linux host runs — ships it with perl at
`/usr/bin/core_perl/shasum`, off the default PATH, and carries coreutils'
`sha256sum` instead; ubuntu and macOS carry both. Two shipped scripts hashed
everything to the empty string there and compared the empty strings EQUAL:
`scripts/verify-hook-freshness.sh` printed `fresh` over a planted stale hook,
and `scripts/gotest.sh` keyed its binary cache on nothing and reported a stale
binary's result. Neither `set -e` nor `set -o pipefail` sees it — a command
substitution whose command is not found yields the empty string and the status
of the assignment it feeds is discarded.

Those two have a fix written and committed under ranger-base-sydnc, with
behaviour pins (`hashportability_qa_test.go`) that run them on a PATH with no
`shasum` — and that commit has not landed on main, which is what the pending
rows below are about. This bead is the second door either way: a census, so
the NEXT script that hashes cannot reintroduce the spelling.

## Why the census could not simply forbid the word

The call sites are not one class. Classified by hand first, then by the
reader the pin ships:

| path | call | class |
|---|---|---|
| `scripts/release-artifacts.sh:375` | `sum() { if command -v sha256sum …; else shasum -a 256 …; fi; }` | probes |
| `scripts/gotest.sh:173` | bare `shasum -a 256` | the finding, fix unlanded |
| `scripts/verify-hook-freshness.sh:233,238` | bare `shasum -a 256` | the finding, fix unlanded |
| `scripts/macos-install-probe.sh:465,486` | bare `shasum -a 256` | darwin-only, declared |
| `docs/adr/0059-packed-refs-lock.probe.sh` ×4 | bare `shasum` | hand-run instrument, declared |
| `docs/adr/0076-flag-trust.probe.sh` ×4 | bare `shasum -a 256` | hand-run instrument, declared |
| `docs/adr/0076-trust-ask.probe.sh` ×4 | bare `shasum -a 256` | hand-run instrument, declared |
| `docs/adr/0076-reporter.probe.sh` ×2 | bare `shasum -a 256` | hand-run instrument, declared |

A door that reds over the last two groups is a rule about text that the next
reader exempts their way past; a door that is green over them has to say WHY,
in the tree. So the rule is structural and the exemptions are rows with
reasons: every runnable line naming a hasher must sit inside a `command -v`
probe region, a file whose lines all do must name BOTH spellings (a probe for
one spelling with no fallback is the bug with a conditional in front of it),
and anything else is a finding unless `hshExempt` or `hshPending` declares the
path.

`scripts/macos-install-probe.sh` is the interesting exemption and it holds on
two legs, not one: `[ "$(uname -s)" = Darwin ] || die` at :161, and no roster
reaches it — `make macos-install-probe` is the only route, neither `make test`
nor `scripts/verify-box.sh` runs it.

## What the door cannot see

The probe's third arm: neither spelling present must be a loud stop and never
a value. That is behaviour, not text. The two scripts sydnc fixed have it, and
`hashportability_qa_test.go` runs them on a box with neither.

`scripts/release-artifacts.sh` is a two-way probe with no such arm, and the
first draft of this page called that the same bug. It is not. MEASURED
2026-11-09, bash 3.2.57 on darwin, running its `sum()` shape under its own
`set -eu` against a PATH holding neither spelling: `shasum: command not found`
on stderr, the function's 127 propagates out of the `(cd "$OUT" && sum … >
checksums.txt)` subshell, and the script dies there — exit 127, with a
zero-byte `checksums.txt` left in OUT. The redirection creates the file before
the command runs, which is where the empty file comes from; the script does
not then go on to publish it. Loud is the whole difference from the sydnc
class, which was silent and said `fresh`. No bead is filed on it.

The census is green over all of that on purpose: it asks where the spelling is
decided, not what the decision does.

## Not in the subject

`etc/cleanroom/Dockerfile.{arch,debian,fedora,rhel}` each run `sha256sum -c -`
bare, and are right to: those lines execute INSIDE a Linux image the
Dockerfile builds, where `sha256sum` is coreutils and cannot be absent. They
are not runnables of this repo by `cgRunnable`'s definition — no `.sh`
extension, no shebang — so they are outside the walk rather than exempted by
it. `docs/runbooks/release.md` names both spellings and says which box each
belongs to; it is prose.

## The pending rows, and why they are not exemptions

`scripts/gotest.sh` and `scripts/verify-hook-freshness.sh` are findings on
main today: ranger-base-sydnc's fix is written and committed on that bead's
own branch (e2499fcc) and has not landed. A census that exempted them would be
green over the exact bug it exists to prevent; a census that redded over them
would red `make tree-check`, and so `make test`, for every seat on the box
until that bead closes. So they sit in `hshPending`, each row naming the bead
that holds the fix, and the arm over that map fails when a pending path is no
longer a finding — landing sydnc reds this file and the remedy is to delete
the row. MEASURED by checking those two scripts out of that branch into a
scratch copy of this tree: the census reports
`hshPending names scripts/gotest.sh and every hasher mention in it now sits
inside a probe — THE FIX LANDED. Delete the row` for both, and nothing else.

## The walk, and the floor under it

The subject is the shipped runnables: `cgScannedDirs` (`scripts`, `plugin`,
`etc`, `docs`) filtered by `cgRunnable`, which is the one spelling of that
list and is held complete by
`TestEveryShippedRunnableFileIsInsideTheCredentialScan`. 77 runnable files,
MEASURED 2026-11-09; the pin floors at 50 so an ordinary landing does not move
it, and floors the hashing files at 3 so a matcher that has gone to sleep
cannot read as a clean tree.

## Verified by breaking it, four ways

1. `hshPending`'s `scripts/gotest.sh` row deleted → `scripts/gotest.sh calls a
   hasher by name on 1 runnable line(s) outside any command -v probe:
   173: sha=$(shasum -a 256 "$tmp" | cut -c1-16)`.
2. sydnc's two fixed scripts checked out over the tree → both pending rows
   report `THE FIX LANDED. Delete the row`, and nothing else reds.
3. A new `scripts/*.sh` with a bare `shasum` planted → reported by path and
   line.
4. The tree as it stands → green.

The reader's own shapes are a control arm
(`TestQATheHashSpellingReaderPlacesEveryProbeShape`, fourteen planted cases:
the one-line probe this tree ships, the three-arm block, a nested conditional
that must not close the region early, a bare call, a bare call after a probe
that closed, a comment, a heredoc body, a dash-heredoc terminator, a variable
named for a keyword, an unclosed region, a probe head that is itself the last
line, a conditional that is not a hasher probe, the wrapper's own name, and a
probe spelled with `which`). Without it the census could answer `probed` for
everything and nothing in the suite would say so.

The last-line case is there because the first draft of the reader had a guard
that zeroed the depth when a probe head was the final line of a file — which
is exactly the shape `lost` exists to catch, so the guard made the one
unclosed region the reader could not see. It was removed and the case
planted.

## The door

`make corpus-check`, not a new one, and not `scripts-check`: that door's
subject is `scripts/` alone and this scan is four shipped directories — the
same sentence the credential-acquirer census in the same door was given under
ranger-base-pyl22. The pin is named in `QA_CORPUS_PINS` and enumerated in
`treewidedoor_qa_test.go`'s head comment, whose count sentence moves to one
hundred and twenty-eight pins.
