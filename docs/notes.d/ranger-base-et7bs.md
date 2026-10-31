## The read verb that did not auto-update, and the one footprint it does leave (ranger-base-et7bs)

`scripts/verify-codex-pin.sh` and `scripts/verify-bd-pin.sh` read Homebrew
without `HOMEBREW_NO_AUTO_UPDATE`, and `scripts/verify-grok-pin.sh` was given
the guard under ranger-base-7qey3 with a reason that turns out to be wrong.
This bead applies the guard to the other two and corrects the reason in all
three, because a comment that states a false mechanism as MEASURED is worse
than no comment: the next reader reasons from it.

### What the earlier telling claimed

That `brew info` and `brew list` are auto-update-able commands, so an
unguarded run "can pull a new Homebrew and re-fetch every tap as a side effect
of a read" — citing a 2026-10-31 session where `==> Auto-updating Homebrew...`
printed, portable-ruby downloaded, Homebrew went 7.0.10 → 7.0.11 and three
taps updated.

The banner was real. It belonged to a different command. The line that
produced it was one compound Bash call containing, in order, `brew pin --cask
grok-build`, `brew list --pinned`, `brew info --cask --json=v2 grok-build`,
and then **`brew upgrade --cask grok-build`** — and the transcript shows the
two read verbs answering first, silently, with the banner appearing under the
`brew upgrade`. `upgrade` is in the auto-update set. The reads are not.

### MEASURED 2026-10-31, this box (Homebrew 7.0.11-75-gfcdfa04, macOS 26.7.1 arm64)

**Arm 1 — brew's own decision, asked directly.** `setup-auto-update`
(`$(brew --repository)/Library/Homebrew/utils/auto-update.sh:138`) is what
exports `HOMEBREW_AUTO_UPDATE_COMMAND`, and its list is `install outdated
upgrade bundle release`, plus `tap` with arguments and the `bump` family.
Sourced in a clean `env -i` shell and run once per verb:

| `HOMEBREW_COMMAND` | `HOMEBREW_AUTO_UPDATE_COMMAND` |
|---|---|
| `info` | unset |
| `list` | unset |
| `upgrade` | 1 |
| `outdated` | 1 |
| `install` | 1 |
| `tap` | 1 |

**Arm 2 — the live reads, guard deliberately unset.** The three verbs the pin
scripts use (`brew list --pinned`, `brew list --cask --versions codex`, `brew
info --cask codex`), run with `env -u HOMEBREW_NO_AUTO_UPDATE`, against a
before/after snapshot of `brew --version`, every tap's `git rev-parse HEAD`,
and the API cache's mtime: **no banner, and the snapshots were byte-identical.**
No Homebrew move, no tap move, no re-fetch.

So the cross-gate hazard was filed against a mechanism that does not exist for
these verbs.

### The footprint that is real

`Homebrew::API.fetch_api_files!` (`Library/Homebrew/api.rb:232`) picks a
staleness window for the cached package JSON:

- `HOMEBREW_NO_AUTO_UPDATE` set (and `HOMEBREW_FORCE_API_AUTO_UPDATE` not) → `nil`, which `skip_download?` reads as *never stale*: no fetch, ever.
- an auto-update command → `HOMEBREW_API_AUTO_UPDATE_SECS`, default **450s**.
- anything else, `brew info` and `brew list` included → `DEFAULT_API_STALE_SECONDS`, **7 days**.

Homebrew's own `env_config.rb` says as much: "Alternatively, disable API
auto-update checks entirely with `$HOMEBREW_NO_AUTO_UPDATE`."

**MEASURED, both arms, against a scratch `HOMEBREW_CACHE`** — a copy of the
real cache with every file backdated to 2026-10-23 01:00, eight days old, so
the 7-day window is open. The operator's cache was not touched:

| arm | command | result | scratch cache mtime after |
|---|---|---|---|
| guarded | `HOMEBREW_NO_AUTO_UPDATE=1 brew info --cask codex` | answered from cache | unchanged, `2026-10-23 01:00:00` |
| unguarded | `env -u HOMEBREW_NO_AUTO_UPDATE brew info --cask codex` | `==> Downloading Homebrew API data` | rewritten, `2026-10-31 01:11:54` |

The file is `api/internal/packages.arm64_tahoe.jws.json`, ~16 MB. So an
unguarded pin gate whose cache has gone stale spends a 16 MB download inside a
check that is supposed to read, and `scripts/verify-box.sh` runs
`verify-grok-pin`, `verify-codex-pin` and `verify-bd-pin` back to back off that
one cache — the first can move the data the other two answer from. The bead's
conclusion was right; only its mechanism was wrong.

The guard costs a reading that may be up to seven days behind. For these three
gates that is the correct trade: they ask "has upstream moved past the pin",
which is an operator decision with no deadline, and a run of `make verify-box`
that answers all three out of one cache state is worth more than three readings
taken at three different freshnesses.

### What changed

- `scripts/verify-codex-pin.sh`, `scripts/verify-bd-pin.sh` — `export HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_ENV_HINTS=1` after `set -uo pipefail`, above every brew read.
- `scripts/verify-grok-pin.sh` — same guard, already present; its comment rewritten off the measurement above.
- `internal/treepins/brewguard_qa_test.go` — the reasoning once, plus `bgRead`/`bgAssertBrewReadsAreGuarded` and a control. The pin asserts POSITION, not presence: an export below the calls satisfies a `strings.Contains` of the tree and guards nothing. Its three arms stay in their subjects' files (`TestQAGrokPinScriptDoesNotLetBrewAutoUpdate`, `TestQACodexPin…`, `TestQABdPin…`), which is where a seat looks for a pin about one script.

Handed off rather than applied: `scripts/macos-install-probe.sh` exports
`HOMEBREW_NO_ANALYTICS=1` beside the same guard and these three gates do not,
and analytics are enabled on this box — so every `make verify-box` reports six
`command_run` events to Homebrew's InfluxDB. The payload is the command name
and its value-stripped flags, with no cask name in it, but whether the fleet
suppresses it is a visibility ruling and not a build fix, so it is
**ranger-base-w2bha** (`-l security`) and the gates are unchanged on that point.

Not done, and said rather than filed: the pin names its three scripts
explicitly instead of globbing `scripts/verify-*.sh`. A glob would catch a
fourth gate, and it would also make this a tree-wide pin owing a Makefile door
and a register row (AGENTS.md, the `internal/treepins` enumeration rule). Three
named files is the whole live population, and the file set is then spelled in
the test rather than discovered by it.
