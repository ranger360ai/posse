# macOS install routes — what was run, what it found, what still needs a Mac
# nobody has automated

Status: **MEASURED** (ranger-base-hza, 2026-08-29; §7 added by
ranger-base-9vg3 the same day, which fixed §2's third finding; §7's published-tap
pour added by ranger-base-aq0l, 2026-08-30, once v0.4.0 made it runnable). The
operator authorised
this on 2026-08-28: *verify the macOS install routes as far as automation
reaches; anything that truly needs the operator's own Mac, name it precisely
and hand back.*

`scripts/cleanroom.sh` is Linux by construction — four distros, all
containers — so the two things that actually bit the operator on
ranger-base-253 were **written and never run**: zsh's PATH, and the Homebrew
tap. `ranger-base-hza` carried that gap as an accepted risk from 2026-08-26.
This document is what closed most of it and what is left.

The instrument is `scripts/macos-install-probe.sh`. It is to macOS what
`cleanroom.sh` is to the Linux distros: run on demand, before a release, not
per push. Every number below is reproducible with it; none is estimated.

```sh
scripts/macos-install-probe.sh all        # everything below
scripts/macos-install-probe.sh paths      # fast, no network
```

**It changes nothing on the box.** Everything lands in one scratch root it
deletes on exit. `brew` mode reaches a real `brew tap`/`trust`/`install` by
cloning Homebrew into that root and pointing `HOMEBREW_CACHE`,
`HOMEBREW_LOGS`, `HOMEBREW_TEMP`, `XDG_CONFIG_HOME` and `HOME` at it.
`XDG_CONFIG_HOME` is the one that matters: it is where `brew trust` writes
`trust.json`, and without it the probe would grant a formula on the live brew.
Verified after each run — the live `~/.homebrew/trust.json` untouched, no
`posse` keg in `/opt/homebrew/Cellar`.

---

## 1. The default PATH, and which zsh file a PATH line survives in

macOS builds a login shell's PATH in `/etc/zprofile`, by eval'ing
`/usr/libexec/path_helper`. The default PATH is exactly what that prints, and
**none of `~/go/bin`, `~/.local/bin` or `/opt/homebrew/bin` is on it.**

The first two are the ranger-base-253 premise and it still holds. The third is
new, and it is a hole in this runbook's own step 2 — see §2.

zsh reads `.zshenv` always, `.zprofile`/`.zlogin` only for login shells, and
`.zshrc` only for interactive ones. Crossed with `path_helper` running from
`/etc/zprofile` — after `.zshenv`, before `.zprofile` — one export line gets
four different answers:

| the export lives in | login+interactive | interactive | non-interactive | login, non-interactive |
|---|---|---|---|---|
| `.zshenv`   | found, **demoted** | found, first | found, first | found, **demoted** |
| `.zprofile` | found, first | not found | not found | found, first |
| `.zshrc`    | found, first | found, first | not found | not found |
| `.zlogin`   | found, first | not found | not found | found, first |

Read three ways:

- **`~/.zshrc`, which INSTALL.md §1 and `scripts/path-warning.sh` both name,
  is correct for its case.** Both shell kinds a person types into find the
  binary, and find it first. That is a verification, not a correction.
- **`.zshenv` is the trap.** The export survives every shell kind, so it looks
  like the thorough choice — and at login `path_helper` runs *after* it and
  prepends the system paths, demoting it. That is the same ambiguity that
  produced ranger-base-253: the binary is found, just not the one you meant.
- **Nothing reaches a non-interactive, non-login shell** except `.zshenv`, and
  there it is first. A script, a git hook, or `ssh box 'posse …'` sees the
  `.zshrc` export not at all.

## 2. The Homebrew route, run end to end

All three commands of INSTALL.md §2 were run against a scratch prefix, on
Homebrew 6.0.20, macOS 26.4.1 arm64. Four findings; two are documentation
defects and are fixed in INSTALL.md, and the third — the Command Line Tools
gate — is fixed in the release machinery, in §7.

**`/opt/homebrew/bin` is not on the default PATH.** §2's Verify says `which
posse` answers `/opt/homebrew/bin/posse`. On Apple Silicon that can only pass
if the user's profile carries `eval "$(/opt/homebrew/bin/brew shellenv)"` —
the line Homebrew's installer prints once, under "Next steps", and never
repeats. Without it `brew install` succeeds and `posse` is still `command not
found`: the exact shape of ranger-base-253, on the route advertised as the way
to avoid it. On Intel the prefix is `/usr/local`, which *is* on the default
PATH — so this failure is Apple-Silicon-only, and invisible to anyone who
wrote or checked the page on an Intel Mac.

**`brew tap-info` keeps reading `Untrusted` after the grant.** §2 told the
reader that `tap-info` reads `Untrusted` until they trust the formula. It
reads `Untrusted` afterwards too, because `tap-info` reports **tap** trust and
`brew trust --formula` is a **formula** grant — the narrow one we recommend
never flips it. A reader who checks `tap-info` to confirm the trust line
worked concludes it did not. `trust.json` is the thing to read, and its whole
content after the grant is one string:
`{"trustedformulae":["ranger360ai/tap/posse"]}`.

**A bottle-less formula demands working developer tools — FIXED
(`ranger-base-9vg3`, 2026-08-29).** The v0.3.0 formula shipped no bottle, only
per-arch tarballs, so brew took its build-from-source path
(`formula_installer.rb`, `unless pour_bottle?`) and ran the **fatal**
developer-tools diagnostics before unpacking anything. On a Mac whose Command
Line Tools are behind the running macOS, `brew install ranger360ai/tap/posse`
died with `Your Command Line Tools are too outdated` having never read our
formula — on the route the page advertises as *"a release binary, no Go
needed"*. See §7 for the fix and what it was measured against.

**The rest of the route is clean.** With that gate cleared, `brew tap` →
`brew trust --formula` → `brew install` installs, `posse version` prints
`0.3.0+8595b8a`, the caveats print (including the `brew install beads` is the
wrong bd warning), and `brew uninstall` / `brew untrust --formula` /
`brew untap` all undo it cleanly.

## 3. The published tap, checked against the generator

- `github.com/ranger360ai/homebrew-tap` is published and `brew tap` works.
- The published `Formula/posse.rb` is **byte-identical** to what
  `scripts/tap-formula.sh` renders at the `v0.3.0` tag — nobody hand-edited
  the tap. The probe renders with the generator *as it was at the release
  tag*, not at HEAD, so a later generator change is not reported as tap drift.
- All four published `sha256`s match the bytes GitHub serves.
- Both macOS binaries execute and report `0.3.0`: `darwin_arm64` natively,
  `darwin_amd64` under Rosetta.
- `brew audit --strict` found one real problem — `` `version 0.3.0` is
  redundant with version scanned from URL`` — on all four os/arch pairs. The
  stanza was dropped from `scripts/tap-formula.sh` on the strength of that, and
  **§8 puts it back**: the scan audit calls it redundant with is a property of
  the brew on the box, and on any brew before 6.0.14 the scan is wrong. Read
  §8 before acting on this bullet.

## 4. Gatekeeper

The released binaries are Go's **ad-hoc** signature — valid, not notarized, so
`spctl -a -t execute` assesses them `rejected`. That assessment only bites a
file carrying `com.apple.quarantine`, and **neither `curl` nor `brew` sets
it** (`curl` sets `com.apple.provenance`, which does not block). So both
advertised routes never meet Gatekeeper. Measured over three fresh copies of
one binary:

| | | |
|---|---|---|
| no quarantine attribute | runs | the control |
| quarantined | **blocked** | no output, no error, no exit — it hangs |
| quarantined, cleared before first run | runs | `xattr -d com.apple.quarantine` is the fix |

The middle row is what a person gets who downloads the tarball **from the
releases page in a browser** rather than with `curl` — a plausible route the
page does not mention. It is the worst failure shape available: silence.

And the tail: **clearing the attribute on a file that has already been blocked
does not give it back.** That copy stays blocked. The fix is to delete it and
re-extract, which is not what anyone tries second.

## 5. What this does NOT cover — the handback

Named precisely, because ranger-base-hza exists to bound what a verdict may
claim.

- **A stale-CLT box, from here on.** This is a handback that *inverted* during
  ranger-base-9vg3. On 2026-08-29 the operator updated this box's Command Line
  Tools mid-session — 15.3 → 26.6, matching macOS 26 — which answers
  `ranger-base-3m40` and removes the last stale-CLT machine we had. Everything
  in §7's contrast table was measured in the window before that. From now on
  this box **cannot** reproduce the defect: `scripts/macos-install-probe.sh
  bottle` still measures the pour, but its control reports that this box would
  have installed either way, and says so rather than claiming a contrast.
- **A genuinely fresh macOS user account.** Every "not on the default PATH"
  result here is derived from `path_helper` and from zsh's own startup
  sequence in an isolated `HOME`/`ZDOTDIR`, which is exactly the mechanism —
  but it is not the same as a new account typing the four quickstart lines. If
  someone wants that, it is a new user in System Settings and about ten
  minutes.
- **Gatekeeper's first-run GUI.** The block above was measured non-
  interactively. What a person sees on screen — which dialog, whether
  "Open Anyway" appears in System Settings — was not.
- **Intel hardware.** `darwin_amd64` was run under Rosetta on Apple Silicon,
  which exercises the binary, not an Intel Mac's `/usr/local` prefix.
- **macOS versions other than the one this box runs.** One version, measured.

## 6. Reproducing any of this

```sh
scripts/macos-install-probe.sh paths          # §1
scripts/macos-install-probe.sh tap            # §3
scripts/macos-install-probe.sh quarantine     # §4
scripts/macos-install-probe.sh brew           # §2, the PUBLISHED tap
scripts/macos-install-probe.sh brew --stub-clt-gate   # §2, past the gate, NOT a user's result
scripts/macos-install-probe.sh bottle         # §7, THIS tree, with its own control
scripts/macos-install-probe.sh all --keep     # everything, scratch root kept
```

Exit 0 = every probe agreed with INSTALL.md. Exit 1 = one disagreed. Exit 2 =
nothing was measured, which is not a pass.

---

## 7. Bottles — the fix for §2's third finding

`ranger-base-9vg3`, measured 2026-08-29 on this box: **Homebrew 6.0.20, macOS
26.4.1 arm64, Command Line Tools 15.3** — i.e. a machine that fails the gate.
The operator updated those Command Line Tools to 26.6 later the same day, so
this is the last measurement this box can make of the defect itself; §5 says
what that costs. Everything below the contrast table was re-run afterwards and
is unchanged.

A bottle is a prebuilt keg: the tree `def install` would have left in the
Cellar, tarred as `posse/<version>/…`. brew pours it straight in and never
enters the build-from-source path, so the fatal developer-tools diagnostics are
never reached. It is **not a build**, which is why
`scripts/release-artifacts.sh` produces all four of them from the four binaries
it already cross-compiles, on one Linux runner, with no brew and no Mac
involved.

### Both arms, on the same box

| the formula | what brew did |
|---|---|
| with its `bottle do` block | `Pouring posse-0.3.0.arm64_sonoma.bottle.tar.gz` → installed, `posse version` printed `0.3.0` |
| the block deleted (control) | `Error: Your Command Line Tools are too outdated.` |

The control is what makes the first row mean something: on a box with current
developer tools *both* rows install, and a green run there says nothing about
the defect. `scripts/macos-install-probe.sh bottle` runs both and reports which
kind of box it landed on — verified in both directions on 2026-08-29, before
and after the Command Line Tools were updated, the same tree giving `CONTROL:
… dies at the Command Line Tools gate` on the first and `CONTROL: this box
installs the bottle-LESS formula too` on the second.

That second shape is also why `brew` mode gained a **"did brew POUR?"** check.
On a current-CLT box a bottle-less formula installs perfectly, so every
existing assertion stays green over a tap that lost its bottles — the
regression would be invisible to everyone who could report it and fatal to
everyone who could not. Measured: with the tools current, `brew` mode's install
succeeds and the pour check is the single line that fails.

### A first `brew install` from the bottled published tap

The two rows above measure THIS tree's bottles through a loopback server
standing in for the release — the tap itself still carried the bottle-less
v0.3.0 formula when they were run. `ranger-base-9vg3` named that as the one
pour the runbook had never done, and `v0.4.0`, published 2026-08-29, is
bottled — so `ranger-base-aq0l` ran it, 2026-08-30, `scripts/macos-install-probe.sh
brew` unchanged, against `ranger360ai/tap` as GitHub actually serves it:

```
ok      brew resolves this formula's version as 0.4.0 — the bottle url it builds will name a published asset
ok      brew install ranger360ai/tap/posse
ok      the published formula POURED a bottle: Pouring posse-0.4.0.arm64_sonoma.bottle.tar.gz
ok      the brew-installed posse reports 0.4.0: posse 0.4.0+feaf301 (herdr-native)
ok      brew's download carries no com.apple.quarantine — the brew route never meets Gatekeeper
ok      the caveats print, and they still name 'brew install beads' as the wrong bd
```

Exit 0 — every assertion in `brew` mode, including the "did brew POUR?" check
this section added and the version-scan check §8 added, agreed with
INSTALL.md. This is the gap §5 named in earlier drafts of this document: it is
now closed, on this box, against the tap as published. It does not cover a
stale-CLT box — none remains to measure against, per the point above — nor
does it repeat once the tap is re-rendered again; re-run it after each
release the way §6 says.

`scripts/macos-install-probe.sh`'s own `--version` default was still `v0.3.0`
when this was run — a second copy of the exact staleness this bead exists to
fix, this time in the probe rather than the prose. Fixed alongside this
section: the default now tracks the current release, `v0.4.0`, so §6's
`scripts/macos-install-probe.sh brew` — no flag — measures the tap as it
actually stands rather than replaying the release this document was
originally measured against.

### Four things that were measured rather than assumed

- **The bottle filename is brew's, and it is not the one the docs show.** brew
  fetches `<name>-<version>.<tag>.bottle.tar.gz` from a non-GitHub-Packages
  `root_url` — **one** dash (`Bottle::Filename#url_encode`). Its cache, its
  documentation and every `brew bottle` output use **two**. The two-dash name
  404s at install time. First observed as exactly that 404.
- **An `arm64_sonoma` bottle pours on macOS 26 Tahoe.**
  `OS::Mac::Bottles::Collector#find_older_compatible_tag` falls back to a
  bottle built for an older macOS, so **one tag per arch covers every macOS
  above it** — present and future — without an asset per macOS release. Linux
  has no such fallback (the override is macOS-only), so `x86_64_linux` and
  `arm64_linux` are exact.

  **The fallback runs downwards ONLY, and that makes the tag a floor**
  (`ranger-base-olwk`). The candidate is kept when
  `candidate.to_macos_version <= tag_version`, so a tag ABOVE a box's macOS
  matches nothing. v0.4.0 read `HOMEBREW_MACOS_OLDEST_SUPPORTED` (14) as "the
  oldest macOS Homebrew supports" and tagged there; that constant is the oldest
  macOS Homebrew *builds bottles for*, while the oldest it *runs on* is
  `HOMEBREW_MACOS_OLDEST_ALLOWED` (10.15) and `MacOSVersion::SYMBOLS` still
  carries ventura 13, monterey 12, big_sur 11. Measured against the published
  v0.4.0 tap on Homebrew 6.0.20, one `brew fetch --formula --bottle-tag=TAG`
  per tag: `arm64_tahoe`/`arm64_sequoia`/`arm64_sonoma` poured, and
  `arm64_ventura`, `arm64_monterey`, `ventura`, `monterey`, `big_sur` each
  answered `Bottle for tag … is unavailable`. Measured again through brew's own
  `Collector#find_matching_tag`, which is the arm that shows the fix as well as
  the defect: with `{arm64_sonoma, sonoma}` every macOS below 14 resolves to
  NONE; with `{arm64_big_sur, big_sur}` every macOS from 11 to tahoe resolves.
  The floor is now big_sur — the oldest macOS arm64 has at all. Catalina
  (10.15, Intel) was considered and rejected as the Intel floor: brew's own
  note removes the `catalina` symbol in September 2026 or later, an unknown
  symbol makes `to_macos_version` raise and the collector SKIP the tag, so a
  catalina-only Intel floor would stop covering Intel entirely the moment that
  lands (measured: an unknown symbol is tolerated and skipped, not fatal).
- **No `INSTALL_RECEIPT.json` is needed in the bottle.** `Tab.for_keg` returns
  an empty tab when the file is absent, and brew writes its own at pour time —
  confirmed by reading the receipt out of the poured keg (`poured_from_bottle:
  true`). That is what lets a bottle be built without `brew bottle`, and
  therefore without a Mac.
- **The version brew scans is GitHub-shaped.** The formula carried no `version`
  stanza (§3), so brew scanned it out of the first `url` — and `Version.detect`
  only recognises the GitHub *release* URL. Point a source url at any other host
  and `posse_0.3.0_darwin_arm64.tar.gz` resolves to version **`64`**, after
  which brew asks for `posse-64.<tag>.bottle.tar.gz` and everything 404s. This
  is why `bottle` mode rewrites only `root_url` and leaves the four source urls
  alone. **This paragraph is §8's defect, found here and read as a probe
  constraint rather than as a deployer's install failing.** It is exactly the
  same sentence: the difference is that a brew older than 6.0.14 does to the
  real GitHub url what a rewritten host did to this one.

### `brew audit --strict`, all four pairs

Clean, `rc=0` on `macos/arm`, `macos/intel`, `linux/arm` and `linux/intel`,
against the generated formula with its bottle block **as it was rendered
then**. §8 adds a `version` stanza, which audit reports as redundant on all
four; that one finding is now expected and is the price of the fix. Run in the **scratch**
Homebrew, not the live one — audit pulls its own developer gems, and doing that
on the live brew on 2026-08-29 installed a `json 2.21.2` into `vendor/bundle`
that is incompatible with portable-ruby's built-in json and broke `brew
info`/`search`/`config`/`doctor` outright. `ranger-base-ltrw` carries the
one-line repair. **Audit in a scratch prefix.**

Audit is the only thing that reads all four platforms — a bottle block is
per-platform and a per-arch install cannot see the other three — so it is
checked per pair, not once.

### What holds it

- `scripts/release-artifacts.sh` — `bottle_tag()` and `bottle_from()`; eight
  archives and one `checksums.txt` covering all of them.
- `scripts/tap-formula.sh` — the `bottle do` block, digests read from that same
  manifest, so a missing bottle is a refusal and not a silent omission.
- `.github/workflows/release.yml` — uploads the bottles beside the tarballs.
- `scripts/macos-install-probe.sh bottle` — the end-to-end run above, control
  included; `brew` mode now also asserts the **published** tap *pours* rather
  than merely installing.
- `bottle_qa_test.go` — the CI-side arms: the two generators agree on every
  filename, each bottle carries exactly what `def install` declares, the
  release ships them. Ten mutations were applied and all ten were killed.

## 8. The `version` stanza — §3's audit finding, reversed by §7's bottles

Added by `ranger-base-63q3`, 2026-08-29. `ranger-base-w69s` verified the
published v0.4.0 from a cold start and found that
`brew install ranger360ai/tap/posse` **exits 1 on any Homebrew older than
6.0.14**:

```
==> Fetching downloads for: posse
x Bottle posse (64)
Error: Failed to download resource "posse"
Download failed: .../releases/download/v0.4.0/posse-64.arm64_sonoma.bottle.tar.gz
curl: (56) The requested URL returned error: 404
```

**The mechanism is §7's fourth bullet, seen from the deployer's side.** A
formula with no `version` stanza leaves brew to scan one out of the url, and
that scan lives in the brew on the box, not in the formula. Homebrew commit
`bae7b0408a` "Fix GitHub release version detection" (2026-07-28, first tagged
**6.0.14**) added a `releases/download/vX.Y.Z/` `UrlParser`. Before it,
`Version.detect` falls through to the stem heuristic and reads **`64`** out of
`posse_0.4.0_darwin_arm64.tar.gz`. Measured by loading Homebrew's own
`version.rb` at each tag out of the local brew checkout and calling
`Version.detect` on the published URL — no install, no scratch prefix:

| brew | `Version.detect(".../v0.4.0/posse_0.4.0_darwin_arm64.tar.gz")` |
|---|---|
| 6.0.13 | `64` |
| 6.0.20 | `0.4.0` |

The `diff` between the two `Library/Homebrew/version/parser.rb` files is empty;
the whole difference is four lines added to `version.rb`, the `UrlParser` above.
Reproduce it against any two tags a local `brew --repo` has:

```sh
$ B=$(brew --repo); RB=$B/Library/Homebrew/vendor/portable-ruby/current/bin/ruby
$ S=$(echo $B/Library/Homebrew/vendor/bundle/ruby/*/gems/sorbet-runtime-*/lib)
$ for t in 6.0.13 6.0.20; do
    mkdir -p /tmp/hb$t && git -C $B archive $t Library/Homebrew/version.rb \
      Library/Homebrew/version | tar -x -C /tmp/hb$t
  done
# then, with Homebrew's Pathname#extname/#stem copied in (version/parser.rb
# calls them), require "version" off each -I and print Version.detect(url).
```

`git -C $(brew --repo) tag --contains bae7b0408a | head -1` answers `6.0.14`.

**Bottles are what made it fatal, so this is a regression and not a standing
condition.** A source `url` is a literal string, so before §7 the wrong scan
only mis-named the keg (`posse/64`) and the install still worked. Brew builds a
bottle filename as `<name>-<version>.<tag>.bottle.tar.gz` from the *formula's*
version — so once the bottle block landed, the scanned string became the name
of a file that has to exist on the release. The published v0.3.0 formula does
carry `version "0.3.0"` — read off the tap by `ranger-base-w69s`; it was
dropped for v0.4.0 on §3's audit finding, and `ranger-base-9vg3` carried this
in with it.

**The trade, decided.** With the stanza, `brew audit --strict` on 6.0.20
reports one finding on all four pairs:

```
* Stable: `version 0.4.0` is redundant with version scanned from URL
```

Installability wins. The audit line costs a maintainer one known line of
expected output; the omission costs every deployer whose brew is more than a
month old an install that exits 1 with a 404 naming *our* release. Audit's
second complaint — `` `version` (line 7) should be put before `license`
(line 6)`` — is ours to settle and is settled: the generator renders the stanza
between `homepage` and `license`, which is Homebrew's own component order.

Renaming the release assets so the old heuristic scans them correctly was
considered and rejected: it moves four published asset names, `INSTALL.md` §1's
`curl` lines and `scripts/release-artifacts.sh`, and it can only ever be
verified against a brew nobody keeps installed.

**Why an explicit stanza is a fix and not a mitigation on the affected brews.**
`Downloadable#version` in 6.0.13 returns `@version` before it consults the url
at all, so a formula that states its version never reaches the scan — on 5.x,
on 6.0.13 and on 6.0.20 alike.

### What holds it

- `scripts/tap-formula.sh` — renders `version "X.Y.Z"` before `license`, with
  the trade written down beside it so it is not dropped a third time.
- `tapformula_qa_test.go` — `TestTapFormulaPinsTheVersionSoBrewNeedNotScanIt`:
  the stanza is present, it is the tag's version at two different tags, and it
  precedes `license`. Three mutations applied (stanza dropped, version
  hard-coded, stanza moved after `license`); all three killed.
- `INSTALL.md` §2 — the deployer-facing half: a 404 naming
  `posse-64.arm64_sonoma.bottle.tar.gz` means `brew update`, not a broken
  release, and `brew info … | head -1` says which side of 6.0.14 you are on
  before you install. Pinned by `macosinstall_qa_test.go`.
- `scripts/macos-install-probe.sh brew` — asks `brew info` for the version brew
  RESOLVES and fails by name before installing (`ranger-base-w69s`), so a box
  on an older brew reports this defect instead of a bare 404. It reads the
  *published* tap, so it keeps discriminating until the next release re-renders
  the formula.

### What this does NOT do

The fix is in the generator. It reaches a deployer only when the operator
publishes a re-rendered `Formula/posse.rb` to `ranger360ai/homebrew-tap`, or
when the next release does — `docs/runbooks/release.md` step 3. That push is
outward-facing and is the operator's: `ranger-base-2t1q` carries it, including
the one-line edit that corrects the tapped v0.4.0 formula without re-rendering
it (the published sha256s are not reproducible; release.md step 3 says why).
Until then the published v0.4.0 formula still 404s on a brew older than 6.0.14,
and `INSTALL.md` §2's new paragraph is the only thing standing between that
deployer and a bug report against our release.
