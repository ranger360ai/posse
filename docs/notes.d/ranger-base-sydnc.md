# git 2.56 is not what redded the commit-wall pins on the cutover host (ranger-base-sydnc)

MEASURED 2026-11-09 on the laptop: darwin 25.6.0 arm64, go1.26.5, at main
7a14a4f3. Two gits in play, both on this box:

- **2.50.1** — `/usr/bin/git` (Apple Git-155), the box's own.
- **2.56.0** — a Homebrew bottle poured into a scratch keg under this
  session's scratchpad, relocated with `install_name_tool` and ad-hoc
  re-signed, never linked into the prefix and never on any other session's
  PATH. The host runs 2.56.0 (ranger-base-jep08), so this is the same version
  on a different kernel — which is the one split ranger-base-jep08's residual
  said nothing had ever taken ("Linux and 2.56 are measured together").

The bead's premise: ten of the message/identity arms' pins fail on the
always-on Linux host (ADR 0080 D1, ADR 0083's cutover target) while the same
pins are green in CI's Linux and on this laptop, with the SUSPECT named as the
host's git 2.56. The ten are the `auto`-comment-char and `commit.cleanup` arms
of check 3's message reader, its two refusal-text pins, and the crew-name
staged-PATH pin.

## VERDICT 1: it is not the git version

**All 142 pins of the commit-wall cluster pass under git 2.56.0 on darwin** —
arm 1 77/77, arm 2 45/45, arm 3 20/20, 0 fail, 0 skip, 63.4s + 30.6s + 9.5s.
The cluster is every test in the twenty files that build a `visWall`, a
`crewWall`, a ceiling wall or an instance wall; the ten the bead names are in
it, and so is every pin that passed on the host, so a version effect that
redded the ten had to show here.

The same ten were run against 2.50.1 first and are green there too, which is
the control that says the keg was the only thing that moved.

**And the premises those pins rest on do not move either.** Ten questions
asked of both gits, same answers from both (`docs/adr/0083-commit-wall-host-premises.probe.sh`,
section C): under `commit.cleanup=verbatim` git keeps its own template in the
object, untracked-file list included; `core.commentChar=auto` writes a `;`
template with 13 `;`-leading lines against the reader's `-ge 4` guard;
`scissors` truncates the status block; `commit.verbose` appends the diff below
the cut; `git stripspace --strip-comments` still answers `#` under `auto`; and
`-m` under `verbatim` appends no template.

What 2.56 *does* change about this subject is one thing, and it is not
behavioural: **`core.commentChar=auto` is deprecated as of git 2.52.0** and is
documented for removal in Git 3.0, with "incompatible with adding comments to
the commit message in the `prepare-commit-msg` hook" among the stated reasons
(git-config(1), 2.56.0; the binary carries
`Support for '%s=auto' is deprecated and will be removed in Git 3.0`). The
value still works on 2.56 — every `auto` row above is from 2.56 — so the arms
that read it are not wrong today. The pins whose whole subject is `auto`
(commentcharauto_qa_test.go, commentcharautodetect_qa_test.go) have a dated
expiry now, and that is worth knowing before Git 3.0 rather than after.

## VERDICT 2: it is not a derived identity literal that collides with the tree

The cheapest explanation of both symptom classes the bead reports — "a clean
editor commit must land: exit status 1" and "refusals not carrying the
expected `refused by posse gate: …` text" — is one literal: check 3's
identity literals are boundary-free fixed strings (`identityLiteralERE`), so a
literal this repository's own vocabulary carries makes the CONTENT arm refuse
every fixture that stages `package posse` at `internal/posse/…`, and makes it
speak in the content arm's words where the pin asserts the message arm's.
ADR 0080 D2 names the host's account, and that account's name is this
repository's own name.

**Measured and rejected.** `go test -overlay` with one line changed in
`DeriveIdentityLiterals` — the username taken from an env var instead of
`user.Current()` — run over the same 142-pin cluster:

| username literal | cluster result |
|---|---|
| this box's own (control, overlay inert) | 0 fail |
| the repository's own name | **29 unique fail** (arm 1 11, arm 2 12, arm 3 8) |

The reproduction is exact where it overlaps — `TestQAMessageRemedyNamesThe‑
CleanupModeThatKeptGitsTemplate/verbatim` fails with the bead's quoted
message, word for word, and the refusal under it names the content arm and the
username class — but the set is wrong: 29 against the host's 10, and it reds
every one of arm 3's identity pins, including the four the host run reports
GREEN. So the host does not derive a literal of that shape, and this is not
the bead's cause.

It is a live hazard on its own account and is filed as such: of this repo's
1,618 tracked paths, **862 carry that name**, and **1,522 of the 1,618 tracked
files carry it on a line** — so a wall rendered on a box with that account
name refuses essentially every commit into this repository, in a class ADR
0024 D3's remedy ("restate and cite") cannot clear. ADR 0012 D2's crew arm
already made the matching call for its own source and says why in as many
words: the shipped roles are excluded because "a wall that refused those would
refuse its own remedy" (`DeriveCrewLiterals`). The identity source has no such
exclusion.

## What remains, and the instrument for it

The cause is a host fact that is neither the git version nor a tree-colliding
literal. What is left unmeasured from this seat, in the order worth asking:

1. **An inherited git config scope.** The fixtures scrub `$HOME` and nothing
   else (`newVisWallCfg` does not call `gitScrubGlobalScope`), while
   `DeriveIdentityLiterals` asks `git config --get-all user.email` in the TEST
   PROCESS's environment. A hand-run binary inherits whatever scope the
   operator's shell carries — which is exactly how the lane arm differs from
   both CI (`make test`, no such scope) and a seat (which carries posse's own
   managed scope). The bead records a throwaway author identity being set for
   that run; where it was set decides whether the host derives a literal
   neither other venue has.
2. **The system scope on the host's distribution**, which no venue in play
   shares: neither `/etc/gitconfig` nor `/opt/homebrew/etc/gitconfig` exists
   on this box, and CI's runner image is a third answer again.
3. **Linux-versus-darwin at the same git version**, which is now the only
   split left and the one ranger-base-jep08 said nothing had measured.

`docs/adr/0083-commit-wall-host-premises.probe.sh` asks all three and the ten
premises, in one read-only run, with no Go toolchain and no python3 — a
`PROBE EXPECT GOT` table whose EXPECT column is the laptop readings above. It
prints no account name, no address and no config value: every row that reads
one answers YES/NO about it instead, because those are the class check 3
exists to keep out of this repo. Run it on the host as the account the crew
will run as, from inside a checkout, and the row whose GOT differs from EXPECT
is the answer.

The other thing that would end this in minutes is the host run's own
`-test.v` output for those ten names. The bead quotes one message from it; the
refusal text under each failure names which arm spoke, and that is the whole
diagnosis.
