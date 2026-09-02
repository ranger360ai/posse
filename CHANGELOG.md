# CHANGELOG

Notable changes, newest first, one section per release. Write these for
someone deciding whether to upgrade, not for the person who wrote the fix.

A release's section is what `.github/workflows/release.yml` puts at the TOP of
that release's notes on GitHub, above the generated commit list —
`scripts/release-notes.sh` is the reader, `make release-notes VERSION=vX.Y.Z`
prints exactly what will be prepended. Renaming `## Unreleased` to the version
being cut is a precondition of the tag; see `docs/runbooks/release.md`.

## Unreleased

### Upgrading

**`runtimes/` is now part of the promoted set, so this release needs `make
install` *and* `posse promote` — in that order.**

The per-key runtime overlay (`runtimes/<name>.yaml`, the file that sets a
tier's model) is read at every launch and decides which model a session
actually runs on, but nothing attested to it: it was in no manifest entry
and in no promoted path, so a hand edit at the config home changed every
future launch and no check anywhere noticed. It is promoted prose now, on
the same terms as `config.yaml` — written only by `posse promote`, hashed
into `promoted.json`, in no session's writable set, and walled from persona
commits in the constitution repo as `rhq/runtimes`.

What that means the first time you run the new binary: a config home whose
`promoted.json` predates this release names no `runtimes/` entry, so the
launch verify reads `unpromoted runtimes/<name>.yaml` and **every dispatched
launch refuses** until you promote. That is the fence working. The ritual is
`make install` then `posse promote`; `posse promote` does not itself run the
launch verify, so the two cannot deadlock.

One thing to do before promoting: any `runtimes/*.yaml` you placed at the
config home by hand must be committed to the constitution repo first (as
`rhq/runtimes/<name>.yaml`). Promote copies out what the commit carries and
removes what it does not, printing each removal — so an uncommitted overlay
leaves, loudly, rather than quietly staying in force. `posse promote
--dry-run` shows the whole ratification diff, including the arriving and
departing overlay files, and writes nothing.

### Added

**`posse backup` — one command that archives the work graph and the
constitution, on this box, and refuses to write anywhere else.**

The queue repo is the store of record for the whole work graph and it never
grows a git remote, which makes one disk the whole graph and its journal.
Until now the answer to that was whatever each deployment arranged for
itself. It is a verb now.

`posse backup` writes one archive holding the queue repo's history as a
`git bundle --all`, its beads database staged through SQLite's online backup
API against a source posse never writes to, the JSONL projections beside it,
and the config home's promoted set, `runtimes/` and `promoted.json`. `envs/`
and `secrets/` are never archived, and the archive's manifest says so by
name, so a restore reads their absence as policy rather than as loss. The
archive is a plain `tar.gz`: `tar -xzf` is the recovery path and it needs no
posse binary.

**The destination is on-box, and that is not a setting.** A URL, an
scp-style `host:path`, a UNC path, or any directory on a volume the kernel
does not report as local is refused — and so is a volume whose locality
cannot be read at all, because a refusal that fails open is not a refusal.
There is no flag that lifts it. The same rule runs on the source: a queue
repo that has grown a git remote is refused rather than copied.

Every archive is read back and checked against the manifest it carries
BEFORE it is given its name, so an archive that is there is one that
verified — which means the directory itself is the record, with no second
status file that can disagree with it. `posse backup status` reads it;
`posse backup verify` re-opens an archive on demand and extracts nothing.
Retention (`backup_keep:`, default 3) only ever prunes after a newer archive
has verified, so a run that produced garbage cannot destroy the last good
copy on its way down, and a free-space floor (`backup_min_free_mb:`, default
384) refuses rather than filling the disk.

`posse status` grows a line with the age of the newest archive, and past
`backup_max_age:` (default 48h, or twice the interval below) the shop check
carries it as a condition — in the terminal and in the cockpit's governance
block. An install that has never asked for backups says nothing at all; one
that asked and has no archive says so loudly, which is the failure this is
for.

**And it runs on a clock, if you set one.** `backup_interval:` in
config.yaml arms a ticker inside the `posse dispatch --watch` loop — the one
standing process the harness already owns, so there is no plist to install
and no second daemon to leave running. It is level-triggered against the
archive directory rather than against its own tick: it asks how old the
newest archive is, so a watch loop that restarts five times an hour under a
daily interval still makes one archive a day, and a loop that comes up next
to an overdue archive writes one at once instead of waiting out a fresh
interval. `posse pause` does not stop it — pause stops dispatching, and the
queue still mutates in a paused shop. With a schedule armed, `backup_max_age:`
defaults to twice the interval instead of 48h, so changing the cadence moves
the alarm with it. No `backup_interval:` starts no ticker at all, and
`posse backup status` says which of the two you have.

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

**The bd argv gate you installed by hand went stale the moment the source
changed, and nothing told you.**

*Affected: every build before this one, on any box where the gate is
installed. Reports only — it installs nothing.*

`scripts/bd-argv-gate.{sh,py}` is source; what fences your box is the copy a
PreToolUse hook names in your Claude settings, installed by hand because posse
renders no box-wide hook (ADR 0015 §3). So a fix landing in this repo moves
nothing, and the copy falls behind in silence — which is how a wrapper that
failed OPEN on any bd call not on a command's first line stayed live after the
fix for it had landed.

`make install` now ends with that comparison, and `make verify-gate-freshness`
asks it on demand. It resolves the wrapper out of your settings rather than
assuming a path, compares both files against the main checkout's HEAD — never
a worktree's, so no unfinished branch is ever prescribed for a box-wide hook —
and then runs the installed wrapper three times, because a byte-perfect gate
with no working `python3` under it passes everything: an allowed verb must get
through, a denied one must be refused by the parser rather than by the
wrapper's own fallback, and an unrelated command must be untouched. A finding
prints the one line to type — a line written to survive the gate it repairs.
A promote is never failed by it, and a box that never installed the gate hears
nothing.

### Fixed

**`instance:` freed a session name only from an instance that also set it.**

*Affected: any machine running two posse homes on one herdr server where only
the newer one sets `instance:` — which is the ordering you get by adding a
second home to a machine that has been running one for a while.*

The tag prefixes this home's herdr labels, so the second home's `posse new
smoke` should go out as `work/smoke` and coexist with the first home's
`smoke`. It did — but only if the *other* home was tagged too. Against an
untagged home's bare `smoke` the create still died on `session 'smoke'
already exists`, the exact sentence the key exists to remove, and the
asymmetry ran the wrong way: the untagged home met `work/smoke` and created
happily, so the only home that lost was the one that had been configured.
Worse, a tagged home could not `posse relaunch` its *own* session while an
untagged home held a bare row of that name — and the refusal told the
operator to rename or close that other home's workspace, which every
destructive posse path otherwise refuses to touch.

Both checks asked predicates that read a bare label as this home's own. They
now compare against the label this home would actually write, so a
differently-labelled row — anyone else's — is no longer in the way of either
a create or a relaunch. One thing got stricter with it: a workspace already
wearing this home's *own* rendered label (two homes sharing one tag, or a row
of yours whose session record is gone) used to slip past the create
altogether and leave herdr holding two workspaces under one label. That
refuses now, naming the row by the name `posse list` prints.

**The bd argv gate read your prose as commands, and refused it.**

*Affected: every build before this one, on any box that installed the gate as
a PreToolUse hook. It now refuses less, and says more when it does; nothing it
refused as an invocation is refused any less.*

Appending a lesson to a notes file with `cat >> file <<'EOF'` was refused
whenever a line of the heredoc *body* happened to open with `bd` and a word.
The gate split the whole typed line on newlines — a newline is a list
separator to a shell — so every line of every heredoc body became a segment
with a command word of its own. A sentence of English resolved as an
invocation of a verb that does not exist, on a line whose `argv[0]` was `cat`,
with no bd binary anywhere on it. The same sentence with one word in front of
it passed all along. Two other spellings of the same thing: a body quoting a
command in backticks was refused as "a construct this gate cannot read", and a
body with an apostrophe in it as "unterminated quote" — refusals aimed at a
shell construct, delivered about a sentence. That is the shape that teaches
people to spell around a fence, and it did, twice in one session.

A heredoc body is data now. The parser recognises the redirection in all its
spellings, keeps the body with the command that opened it, and never tokenizes
it — and asks instead the questions a body can honestly answer: does anything
on the line *execute* what it is handed on stdin (`sh <<'EOF'` and
`cat <<'EOF' | sh` are both still refused, and the shell family is the line —
a `python3 - <<'PY'` whose body quotes a command is prose, the same way
`python3 -c` with that text in it always was), is the delimiter unquoted so
the shell expands a substitution in the body before anyone reads it, and is a
real invocation still standing outside the body. Every refusal now names what
was matched — resolved verb, command word, heredoc body — and where, as
segment N of M with its text on one line, so the next false positive is a bug
report rather than a puzzle.

Measured against every Bash command line in this box's transcripts, 51408 of
them: 1085 stop being refused, 7 start — each of those 7 a heredoc whose line
really does carry a shell.

**A red cage pin could mean the image was two days old, and nothing said so.**

*Affected: every build before this one, on `cage: container`. `posse cage`
prints one more line and `posse cage build` stamps what it builds.*

The L1/L3 wall inside a container is rendered by the posse in the IMAGE, so
every change to that render is invisible in the cage until `posse cage build`
runs again. Nothing ever said the image was behind — the only thing that
noticed was a live test, and it noticed by failing. A FAIL is read as a
regression before it is read at all: one instance cost half an hour proving a
red pin was a two-day-old image and not a broken wall, against an image
carrying a posse thirty commits behind the render it was being asked about.

`posse cage` now prints the image's age above everything else it says about
it: which posse the image carries, which one this source (or, outside a
checkout, this binary) is, and whether those are the same build. `posse cage
build` stamps the posse it puts in the image with the checkout it came from,
so the image can answer that at all — a build from a linked git worktree,
which is where every persona works, carries no commit identity of its own and
called itself "dev". And the live pin that used to fail on a stale image now
skips, naming both versions and the rebuild: "the artifact is too old to be
asked" is a third state, not a broken claim. An image that cannot be asked at
all is still read exactly as before — unclear is not stale, because a pin that
skips on a probe failure is a pin that goes green and stays there.

**Persona memory was never committed by anything, so it accumulated on disk
until a human noticed.**

*Affected: every build before this one, in any home that keeps `personas/`
in git. On by default; homes that keep it outside git are unchanged.*

Every persona appends what it learned to its standing orders
(`$RHQ_PERSONA_DIR/ORDERS.md`) and no path in posse ever committed the
result — measured on one instance at 203 uncommitted lines one day, 1419 the
next, 1538 by the time someone landed them by hand. That file is the one
artifact with no other copy: a bead has the queue behind it and code has git,
but a lesson lives in exactly one place until something commits it, and
sessions get reaped by the dozen.

`posse kill` now commits it — by hand, from the cockpit, and from the
end-of-pass auto-reaper — after the workspace closes and before the session's
worktree is landed. The commit is path-limited to the killed session's own
persona directory, so it can take neither another persona's memory nor
`rhq/agents` beside it, and what it is about to add is scanned for credential
shapes first: a hit holds the commit, names the file and line, and never
prints what it matched. It never pushes.

Killing a session by hand also lands the plane first, the way `posse relaunch`
does: one bounded turn asking the agent to write down what it learned, spent
only when that persona actually has memory no commit holds. A turn that does
not settle refuses the kill rather than closing a workspace mid-commit.
`posse kill --no-land` skips the turn (`--timeout` re-bounds it) and still
commits — the flag declines to spend a turn, not to keep the memory. The
cockpit and the auto-reaper take no turn at all, so neither the TUI nor a
dispatch pass can block behind one.

**`posse cockpit` could take a whole CPU core while sitting there displaying,
and cost a fifth of one even when it did not.**

*Affected: every build with the cockpit's periodic scans. Measured at 101.9%
of a core on a loaded box, 26.3% on an idle one.*

The footer's cost reading re-decoded the entire fourteen-day transcript window
every thirty seconds — on the shop this was measured on, 1211 files and 786 MB,
of which 784.6 MB could not have been written since the previous tick — and the
governance block's budget row scanned a second time on its own tick. That is
the standing cost. The runaway on top of it was the timers: each scan was
started unconditionally, so a scan that outran its own period (the governance
check already used 78% of its thirty seconds on an idle box) had the next tick
start a second one over the top of it, then a third.

The cockpit now keeps its scans' results and re-reads only the transcripts
whose bytes have moved, and it will not start a scan while that scan is still
running — a tick that arrives during one is dropped, because these are level
readings and the next tick takes a fresher one than a queued one would have.
Same numbers on screen, same cadence; 0.69% of a core instead of 26.3%.

`posse cost` and the dispatcher's per-launch budget guard are unchanged: they
scan once and keep nothing, exactly as before.

Not fixed here, and still open: the cockpit's two-second redraw actually takes
around ten seconds, because it reads the bead lists on its own event loop. Key
presses can wait behind one.

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
flag does. A flag rule still walls one spelling, and the spellings below are a
floor, not a count: `git push -f`, `--force-with-lease`, `git push origin
+main` and `git push --mirror origin` all force-push and none of them carries
the token `Bash(git push --force:*)` names. `+main` carries no option to spell
at all; under `remote.<remote>.mirror`, `--mirror`'s force-update is what a
*bare* `git push origin` does, with no option and no refspec in the argv
either. Widening the matcher cannot reach any of these. Deny the verb
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
