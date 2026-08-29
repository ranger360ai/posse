# macOS install routes — what was run, what it found, what still needs a Mac
# nobody has automated

Status: **MEASURED** (ranger-base-hza, 2026-08-29). The operator authorised
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
defects and are fixed in INSTALL.md.

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

**A bottle-less formula demands working developer tools.** The formula ships
no bottle, only per-arch tarballs, so brew takes its build-from-source path
(`formula_installer.rb`, `unless pour_bottle?`) and runs the **fatal**
developer-tools diagnostics before it unpacks anything. On a Mac whose Command
Line Tools are behind the running macOS, `brew install ranger360ai/tap/posse`
dies with `Your Command Line Tools are too outdated` having never read our
formula — on the route the page advertises as *"a release binary, no Go
needed"*. This is not our formula misbehaving and there is nothing in the
formula to fix; shipping bottles would avoid it, and until then the page has
to say so. The remedy is `sudo`-shaped and therefore the operator's.

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

- **A first `brew install` on a Mac with current Command Line Tools.** This
  box's are behind its macOS, so the honest run stops at that gate and the
  completed run needed `--stub-clt-gate`. Updating them is `sudo`-shaped.
  Whoever updates them should re-run `scripts/macos-install-probe.sh brew`
  with **no** stub; that is the run that answers what a user gets.
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
scripts/macos-install-probe.sh brew           # §2, honestly — stops at the CLT gate
scripts/macos-install-probe.sh brew --stub-clt-gate   # §2, the rest of the route
scripts/macos-install-probe.sh all --keep     # everything, scratch root kept
```

Exit 0 = every probe agreed with INSTALL.md. Exit 1 = one disagreed. Exit 2 =
nothing was measured, which is not a pass.
