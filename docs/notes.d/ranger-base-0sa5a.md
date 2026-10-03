## The herdr-bob plugin's screen rules read none of Bob's blockers, and one of its idle screens (ranger-base-0sa5a)

*bead: ranger-base-0sa5a (P2 bug), discovered-from ranger-base-mz8ud · relates
ranger-base-knikn (the live gate), ADR 0060 D2, ADR 0061 D4 ·
2026-10-03, this box (darwin 25.4.0, herdr 0.9.1, bob 2.0.5 / node 25.2.1,
herdr-bob @e53bd475, jq 1.8.1) · everything below is MEASURED by
`scripts/verify-herdr-bob-rules.sh` unless marked ASSUMED*

`rules.json` is the whole of the herdr-bob plugin's `blocked` reporting. Bob
fires no approval hook event, so a permission prompt is invisible to the
lifecycle bridge and can only be read off the pane; that is the one job the
watcher has that the hook bridge does not. ranger-base-mz8ud established that
the watcher had never executed on this box. This is what it would have reported
if it had.

Measured by replaying the six redacted bob 2.0.5 captures in
`etc/herdr/agent-detection/upstream/bob/` through **the plugin's own
`match_rules`**, lifted verbatim out of the installed `bin/bob-watch`:

| capture | shipped `rules.json` | the hand-edited live copy | posse's override |
|---|---|---|---|
| `blocked-execute-command.txt` | — | `execute_command_approval_bob2` | `bob_execute_command` |
| `blocked-execute-command-scrollback.txt` | — | `execute_command_approval_bob2` | `bob_execute_command` |
| `blocked-signin.txt` | — | — | `bob_signin` |
| `idle-composer.txt` | — | — | — |
| `idle-after-turn.txt` | — | — | — |
| `idle-command-picker.txt` | **`trust_folder_prompt`** | **`trust_folder_prompt`** | — |

## The bead was filed one finding short: there is a false positive

ranger-base-mz8ud filed this as "no rule matches any of them". That is right
about the three blockers and wrong about the corpus: `trust_folder_prompt`
matches `idle-command-picker.txt`, in every grep and locale combination tested,
over the whole capture and over the last 40 lines alike. Bob's `/` picker lists
its own commands and one of them is

```
 │     /permissions - View and change this folder's trust level                 │
```

which satisfies the rule's `all` (`(trust|do you trust)`, via `trust level`)
**and** its `any` (`this (folder|workspace|directory)`, via `this folder`). The
rule has no `none`.

**That is the serious half, and it inverts which direction of this bug
matters.** A missed `blocked` falls back to the hook's state, which the file's
own `_comment` already names as the safe direction. A false `blocked` is acted
on: posse's readiness gate stops waiting on the pane and the sidebar lights up,
on an idle composer with a live composer underneath and the typed text kept. It
is also the easiest one to ship by accident — every widening of a rule is a
chance at one — which is why the harness asserts it twice, once as a row of the
matrix and once as its own check with the reason in the message.

## The misses are one character and one codepoint

`tool_approval_prompt` on the Execute Command dialog satisfies its `all`
(`Approve commands:`, `Approve Once`) and then misses every `any` alternative:
the dialog is a selectable list and not a y/n prompt; Bob marks the selected
row with `→` (U+2192), which is in neither `>` nor `❯`; and the footer is
`Press Enter to confirm`, while `Reject` is a *row* and not a footer verb.

`sso_login_prompt` on the browser sign-in screen matches its `any` (`browser`)
and misses its `all`: the pattern is `(sign in|log in|login|authenticate)` and
Bob writes `Complete sign-in in your browser`, hyphenated. One character.

`license_prompt` cannot fire on any capture in the corpus — no capture contains
`licen[sc]e` at all — so it is left exactly as upstream wrote it.

## The fix was already written, in a file herdr cannot evaluate

posse's draft herdr manifest for Bob (`upstream/bob/bob.toml`, ADR 0060 D2)
keys its `signin` and `execute_command` rules on precisely the right lines, and
has since 2026-09-28. It is a manifest for an agent kind herdr does not compile
in, so nothing evaluates it — the patterns existed and were unreachable. The
override's two added rules are ports of those two anchors into the plugin's own
format, keeping their three deliberate properties:

- **A pair of line-anchored patterns in `all`, never a shared footer.** `grep`
  matches per line, so two patterns that cannot share a line are a two-line
  anchor. `Press Enter to confirm` is the obvious key for the approval dialog
  and is that dialog's footer and nothing else's *today*; codex's
  `hooks_review` rule exists because a footer reword silently returned a
  blocked screen to idle (rangerhq-7ia).
- **`^[[:space:]]*Execute Command[[:space:]]*$` must be anchored**, because a
  completed call leaves ` Execute Command (completed)` in the scrollback.
  `blocked-execute-command-scrollback.txt` is the capture that pins it: both
  spellings are on screen at once and it must still resolve to the live dialog.
- **The row marker is optional.** The dialog being up is the block; which row
  the cursor sits on is not.

`trust_folder_prompt` is narrowed to `(do you trust|trust this
(folder|workspace|directory))` — upstream's own other spelling, with the bare
`trust` alternative that caught the picker removed. Nothing is invented: no
capture shows Bob's actual trust screen, so that rule and `license_prompt`
stay **UNFIXTURED**, their positive side upstream's to anchor. What posse pins
about them is negative and checkable — they match none of the six. This is the
same line `bob.toml` draws at its `working` TODO, and for the same reason: a
rule invented from a guess at chrome is a rule nobody can fail.

## Never a multibyte character in a bracket expression

MEASURED 2026-10-03: `/usr/bin/grep` (BSD grep 2.6.0-FreeBSD) under `LC_ALL=C`
evaluates `[>❯→]` **byte-wise**. The class degenerates to the byte set
`{>, E2, 9D, AF, 86, 92}`, and `─` is `E2 94 80` — so the class matches a pure
box-drawing line, and a box-drawing line is on every single Bob screen.
`(>|❯|→)` matches it in none of {BSD grep, ugrep} × {`en_US.UTF-8`, `C`}.

This changes no verdict in upstream's file today, because its class is followed
by a required keyword — say it that way round and do not overclaim it. It is
worth having anyway, because **the watcher does not choose which `grep` or
which locale it gets**: the herdr server's environment does, the plugin
prepends a Nix closure to `PATH` when one was substituted and relies on the
ambient `PATH` when it was not, and a daemon started from launchd has no `LANG`
at all. On this box `grep` is ugrep 7.8.4, which is immune, and
`/usr/bin/grep` is not. A rule that is correct only in a UTF-8 locale is
correct by accident. The override therefore uses alternation throughout, and
the harness asserts by jq that no bracket expression in the file holds a
non-ASCII byte — the property, not the instance.

## The live copy was a hand-edit nobody recorded

`bob-watch` seeds `rules.json` into `config_root()` with
`[ -e "$RULES" ] || cp "$ROOT/rules.json" "$RULES"` and prefers that copy
thereafter. That is what makes this fixable instance-side with no write to the
managed checkout — and it is also how the live file came to hold something
nobody knew about.

MEASURED: `~/.config/herdr/plugins/config/mloeper.herdr-bob/rules.json`, mtime
2026-09-28T16:47:36 — five minutes after the plugin was installed at 16:42 —
carries a fifth rule, `execute_command_approval_bob2`, keyed on `Approve Once`
with `Press Enter to confirm` or `Always Allow Command`. It appears in no
commit in this repo, in no file in the plugin, and in no bead: `grep -rn` over
both trees finds nothing but the file itself. It does fix the Execute Command
dialog — by keying on the footer, which is the one shape the house has a
written reason to avoid — and it leaves the sign-in miss and the command-picker
false positive exactly where they were.

Nobody could have caught it, because **nothing reported what was in that
file**. `herdr plugin list` prints the config *directory*; the plugin logs the
rules path once at `watch started`, on a watcher that had never started. So
`scripts/herdr-bob-rules.sh --check` now prints the installed rule ids and says
`HAND-EDITED` when they match neither upstream's copy nor the repo's, with the
mtime and the ids upstream does not ship. The general shape, which is the part
worth keeping: a file that is seeded once, never refreshed, preferred forever
and reported by nothing is a file that will diverge, and the divergence will be
invisible until someone replays a corpus through it.

It also means **a shipped fix to `rules.json` never reaches anyone who has
already run the watcher once**, since their copy exists. That is in the filing
as upstream's to solve; it needs a migration or a version on the seeded copy.

## The harness, and why it is not the daemon

`scripts/verify-herdr-bob-rules.sh` (`make verify-herdr-bob-rules`), ~9s.
`verify-detection` structurally cannot cover this file: it globs
`etc/herdr/agent-detection/*.toml` at one level, and `rules.json` is neither a
`.toml` nor at that level.

- It **lifts `match_rules` verbatim** out of the installed `bin/bob-watch`
  rather than reimplementing it, and refuses to report a single verdict if the
  extraction does not still look like that function — one definition,
  `grep -qiE`, all three clause names, a closing brace. A reimplemented
  eight-line matcher would agree with itself while disagreeing with the plugin.
- Driving the real daemon was the other option and is worse for this question:
  on this box bash 3.2 kills `bob-watch` at its first screen read until
  ranger-base-mz8ud's patch is applied, so a daemon arm could not replay a
  screen at all without that patch, and it would cost a poll interval per
  assertion and read its verdict out of a log line.
- Every verdict is asserted **twice** — whole capture, and last 40 lines, which
  is `bob-watch`'s own `pane read --lines` default — and the two must agree. A
  rule that only works thanks to a line far up the scrollback stops working
  when the pane scrolls.
- **The control arm is the point** (the ranger-base-26y03 lesson): the shipped
  file runs first and both of upstream's failures must reproduce, or the script
  exits 1 before the override's arm runs at all and says that a green result
  would prove nothing. Verified by execution, not by reading: pointed at a
  checkout whose `rules.json` was already fixed, it refuses with exit 1.
- The **grep × locale** arm replays all six under {BSD grep, ugrep} ×
  {`en_US.UTF-8`, `C`} and fails if any verdict moves.
- Two **seeding** properties, against the real `bob-watch` under a fake
  `herdr` with scratch state and config: a fresh `config_root()` is seeded from
  the plugin's own copy (so a clean install gets the broken file, which is why
  there is an install step at all), and an existing copy is preferred and never
  refreshed (so the override is durable). Safe on bash 3.2 without the patch
  because with no registered pane the loop never reaches a screen read.

Both negative arms were run: a "fixed" plugin makes the control refuse (exit
1), and an override widened until `trust_folder_prompt` catches the picker
fails four ways and `--install` refuses to write anything.

## Not done, and why

The live install is **not** applied. `scripts/herdr-bob-rules.sh --install`
writes into the herdr config tree of the server every dispatched seat runs in,
which is hard risk line 3 (deployed systems, per-change permission), and the
existing file there is the undocumented hand-edit above. It is inert today —
the watcher cannot run until ranger-base-mz8ud's patch is applied — so the
install and that patch are one decision, and ranger-base-knikn is where it is
asked. `--check` is read-only and has been run; its output is on the bead.

ASSUMED, not measured, and deliberately left so: that these rules read the same
on a LIVE pane as on the captures. The captures are `herdr pane read --source
detection`; the watcher reads `--source recent-unwrapped --lines 40`. The
window is covered (every verdict is asserted over the last 40 lines too), the
*source* is not — if `recent-unwrapped` renders a wrapped dialog differently
from `detection`, an anchored pattern could miss. That needs a live pane with
the watcher running, which is ranger-base-knikn's and ranger-base-6wqe's
ground, not this bead's.

## For the reader who wonders why the file was byte-identical to upstream's in the plugin root

It was. The plugin root's copy is pristine and matches `@e53bd475`; the
diverged one is the seeded copy in `config_root()`, which is the one the
watcher would read. Measuring the wrong one of those two files gives upstream's
verdicts and a clean conscience. `scripts/herdr-bob-rules.sh --check` prints
both paths for that reason.
