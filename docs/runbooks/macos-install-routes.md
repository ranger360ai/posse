# macOS install routes — what was run, what it found, what still needs a Mac
# nobody has automated

Status: **MEASURED** (ranger-base-hza, 2026-08-29; §7 added by
ranger-base-9vg3 the same day, which fixed §2's third finding). The operator authorised
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
  redundant with version scanned from URL`` — on all four os/arch pairs. Fixed
  in `scripts/tap-formula.sh`: the stanza is gone, audit is clean on all four,
  and `brew info` still reads `stable 0.3.0`. The published tap keeps the old
  formula until the next release re-renders it.

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
- **A first `brew install` from a *bottled published tap*.** §7 pours from a
  loopback server standing in for the release. The published tap still carries
  the v0.3.0 formula, which predates bottles — so
  `scripts/macos-install-probe.sh brew` exits 1 today, on its new "did brew
  POUR?" assertion, and it is right to. Re-run it after the next release is
  published and its formula pushed into the tap; that is the run nobody can do
  before then.
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

### Four things that were measured rather than assumed

- **The bottle filename is brew's, and it is not the one the docs show.** brew
  fetches `<name>-<version>.<tag>.bottle.tar.gz` from a non-GitHub-Packages
  `root_url` — **one** dash (`Bottle::Filename#url_encode`). Its cache, its
  documentation and every `brew bottle` output use **two**. The two-dash name
  404s at install time. First observed as exactly that 404.
- **An `arm64_sonoma` bottle pours on macOS 26 Tahoe.**
  `OS::Mac::Bottles::Collector#find_older_compatible_tag` falls back to a
  bottle built for an older macOS, so **one tag per arch, at
  `HOMEBREW_MACOS_OLDEST_SUPPORTED` (sonoma, 14), covers every macOS Homebrew
  supports** — present and future — without an asset per macOS release. Linux
  has no such fallback (the override is macOS-only), so `x86_64_linux` and
  `arm64_linux` are exact.
- **No `INSTALL_RECEIPT.json` is needed in the bottle.** `Tab.for_keg` returns
  an empty tab when the file is absent, and brew writes its own at pour time —
  confirmed by reading the receipt out of the poured keg (`poured_from_bottle:
  true`). That is what lets a bottle be built without `brew bottle`, and
  therefore without a Mac.
- **The version brew scans is GitHub-shaped.** The formula carries no `version`
  stanza on purpose (§3), so brew scans it out of the first `url` — and
  `Version.detect` only recognises the GitHub *release* URL. Point a source url
  at any other host and `posse_0.3.0_darwin_arm64.tar.gz` resolves to version
  **`64`**, after which brew asks for `posse-64.<tag>.bottle.tar.gz` and
  everything 404s. This is why `bottle` mode rewrites only `root_url` and
  leaves the four source urls alone.

### `brew audit --strict`, all four pairs

Clean, `rc=0` on `macos/arm`, `macos/intel`, `linux/arm` and `linux/intel`,
against the generated formula with its bottle block. Run in the **scratch**
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
