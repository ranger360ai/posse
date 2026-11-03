# The wall reading a release install could never get a sighted answer to (ranger-base-ctath)

Found in the x4g3h shakedown on a release-installed box, 2026-11-03.

## The reading, and why it was permanent

`posse gates <persona> | head -1` on a box that installed from the tap
(`brew install ranger360ai/tap/posse`, `0.5.3+482a1256`) and holds no checkout
of `github.com/ranger360ai/posse`:

```
wall renderer (gates) · 0.5.3+482a1256 · UNKNOWN whether this binary carries
the wall renders and the launch-time checks that landed: no checkout of
github.com/ranger360ai/posse here holds 482a1256 (looked in no directory this
instance knows about) — … so read this as UNKNOWN and never as OK
```

Every word of that is true. What made it a bug is that it can never resolve.
The freshness reading is defined against a LOCAL checkout: `FindLauncher`
(internal/posse/launcherlag.go) takes the commit the binary's stamp names, and
counts it against `main` in the first configured directory that both declares
`module github.com/ranger360ai/posse` and HOLDS that commit. A release user has
no such directory and is never going to get one, so they do not have a stale
answer — they have no answer, on every install of every release, forever.

The same end state as the gap `wallrenderer.go`'s header is about, reached from
the other side: the surface whose whole job is "what is in force here" says
nothing an operator can act on. An abstention that always fires is read as
weather.

## Why the binary could not already tell

`scripts/release-artifacts.sh` stamped exactly one thing:
`-X …/internal/posse.Build=$sha`. That is byte-identical to what a dev
`make build` of the same commit stamps (the Makefile's `BUILD_STAMP`, via
`cmd/buildstamp`), so nothing in the binary distinguished "cut from tag
v0.5.3 by the release workflow" from "built from that commit on somebody's
laptop". `posse version` said `0.5.3+482a1256` either way.

## The fix: the tag is the identity

The release build now carries the tag too —
`-X …/internal/posse.ReleaseTag=$VERSION`, on the same line as the sha — and
`internal/posse/releasestamp.go` reads it. The walls a release renders and the
checks it decides are the ones that tag carries, by construction: it was built
in a throwaway worktree of one tagged commit, and there is nothing local it
could be stale relative to.

A THIRD state, not a rebranded OK:

| state | when | what it says |
|---|---|---|
| counted lag | a checkout here holds the stamp | unchanged — tip, or N behind |
| release identity | the lag abstains and the stamp is believed | which release's walls are in force |
| UNKNOWN | anything else | unchanged, word for word |

```
wall renderer (gates) · 0.5.3+482a1256 is release v0.5.3 — the walls it renders
and the launch-time checks it decides are the ones tagged v0.5.3, which is what
a release install carries
  no local comparison was possible — no checkout of … holds 482a1256 … — and
  whether a newer release has been published is not read here: that is a network
  call, and this line is printed at every launch. So this names which release is
  in force, never that it is the newest one — `brew upgrade
  ranger360ai/tap/posse` is what moves this box forward …
```

Four properties the wording is load-bearing for:

1. **It never claims to be current.** "current" is the tip arm's word and it is
   a claim about `main`. A release may have been superseded an hour after it was
   cut, and this binary takes no network call to find out — the bead rules that
   out on a launch path, and `posse gates` is read at every launch.
2. **The abstention underneath is still printed.** The release state does not
   assert that a comparison happened; it says one was not possible and why.
3. **The lag wins wherever it can be read.** A release binary on a box that DOES
   hold its commit — a fleet box that installed the tap by mistake
   (ranger-base-39jnl) — is counted like any other. MEASURED: such a binary at
   this worktree's HEAD read `is the tip of main in ~/src/posse`, not the
   release state.
4. **It is not actionable.** `WallRenderer.Actionable()` returns false for a
   believed release identity: nothing local can change it. Reporting it as
   "something to act on" would be the permanent UNKNOWN again, wearing a bool.

## The three refusals, and why each is SAID

The bead's own condition — never OK for a dirty or unknown dev build. A stamp
that is there and cannot be believed renders beside the UNKNOWN rather than
being dropped, because a mis-stamped or forged release identity that renders as
silence is indistinguishable from the ordinary dev build this change does not
touch. MEASURED 2026-11-03 by building `cmd/posse` five ways and running
`posse gates builder` against a scratch `RHQ_HOME` with no `beads:`:

| stamps | first line |
|---|---|
| `Build=482a1256`, `ReleaseTag=v0.5.3` | `is release v0.5.3` |
| none | `UNKNOWN … the running binary names no commit (0.5.3+dev)` |
| `Build=482a1256-dirty`, `ReleaseTag=v0.5.3` | `UNKNOWN …` + `stamped as release v0.5.3 and the tree … had uncommitted edits` |
| `Build=482a1256`, `ReleaseTag=v9.9.9` | `UNKNOWN …` + `one of the two is wrong` |
| `ReleaseTag=v0.5.3` alone | `UNKNOWN …` + `names no commit at all (0.5.3+dev)` |

The dirty check is `strings.Contains(versionString, "-dirty")` and not a suffix
test, because the Makefile's dev stamp spells a dirty tree
`<sha>-dirty-<content fingerprint>` (`cagestale.go`'s `dirtyIdent`) while a
release or build-info stamp would spell it `<sha>-dirty`. One check covers both;
a suffix test misses the longer form, which is the one a hand-built binary on
this box actually wears.

## `-X` on a wrong symbol is silent, so the loop is pinned both ways

`go build -ldflags "-X <symbol>=<v>"` on a symbol that does not exist exits 0,
warns about nothing, and leaves the variable at its default
(`cagestale_test.go` measured that for the `Build` stamp). So renaming
`ReleaseTag` and leaving the release script alone would ship releases that read
UNKNOWN again with every unit arm in the tree green — the one direction no
in-package test can hold.

- `internal/posse.ReleaseTagSymbol` spells the symbol once.
- `internal/treepins/releasestamp_qa_test.go` compares the SCRIPT's text to
  that constant, and holds the floor that no other build line in the tree names
  it (`Makefile`, `scripts/clean-build.sh`, `internal/posse/cagestale.go`,
  `.github/workflows/ci.yml` — named, not enumerated, so it is not in the
  tree-wide door class).
- `cmd/posse/gatesrelease_qa_test.go` builds a binary stamped THROUGH that
  constant and reads the state back off `posse gates <persona>` — the command
  and the pipe the bead was filed from. If the constant does not name a real
  symbol, the stamp lands nowhere and the line reads UNKNOWN.

## The version guard had to be anchored first

`$VERSION` now reaches the `-ldflags` STRING, where go splits on spaces — so
`v1 -X …ReleaseTag=v9.9.9` would have become a second linker flag. The script's
own check was `case $VERSION in v[0-9]*)`, a PREFIX check: exactly the shape
`release.yml`'s guard was hardened away from under ranger-base-u9at, and it
accepted `v0.3.0$(id)`, `v9;curl evil` and the injection above. It is anchored
whole-string now, in POSIX sh (the shebang is `/bin/sh`; `[[ =~ ]]` is bash):

```sh
case $VERSION in v[0-9]*) ;; *) refuse ;; esac
case $bare in *[!0-9.]* | *..* | *.) refuse ;; esac
```

`[!0-9.]` refuses every byte that is not a digit or a dot — space, `$`, `;`, a
backtick, a newline — and the other two patterns refuse `v1..2` and `v1.`,
which the first case accepts. Fourteen hostile spellings and one
well-formed control are run against the real script in
`TestQAReleaseVersionGuardRefusesEveryNonVersionString`; the control gets past
the shape check and dies at the tag/source preflight instead, which is what
makes the refusals mean "refused by shape" rather than "refused".

## What this does NOT answer, and the lane it leaves open

Whether a newer release exists. That is a question about github.com, and the
bead rules out a network call on a launch path. `posse status`'s launcher line
is also unchanged — on a release install it still reads `not counted: no
checkout … holds <sha>`, which is an abstention that raises nothing (G11 does
not fire on one) rather than the UNKNOWN-as-alarm this bead was about.
`posse.Release()` is exported so a later bead can wire that surface without
re-deriving any of this.
