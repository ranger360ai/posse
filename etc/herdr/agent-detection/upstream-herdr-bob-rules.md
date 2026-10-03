# herdr-bob: `rules.json` matches none of Bob 2.0.5's blocked screens, and does match one of its idle screens

**Not filed.** This is a draft for the operator to send to
`MartinLoeper/herdr-bob`; nothing here has been published. See `README.md` in
this directory — this is not a manifest question, but this is where posse keeps
the drafts it means to send. (posse-side record: bead `ranger-base-0sa5a`, and
`docs/notes.d/ranger-base-0sa5a.md` in the posse repo.)

Companion to `upstream-herdr-bob-darwin.md`, which reports the four portability
bugs that keep `bin/bob-watch` from running on darwin at all, and whose item 5
is this finding stated without a fix. This is the fix. The two are independent:
the patch there touches `bin/` and `lib/` and deliberately does not touch
`rules.json`; this touches only `rules.json`.

- herdr-bob `@e53bd475`, installed with `herdr plugin install`, `herdr plugin
  list` → **enabled**
- herdr 0.9.1 client and server · bob 2.0.5 / node 25.2.1 · macOS 26.4.1,
  darwin 25.4.0, arm64 · `jq` 1.8.1
- all findings MEASURED 2026-10-03 on that box by
  `scripts/verify-herdr-bob-rules.sh` in the posse repo, which replays six
  redacted `herdr pane read` captures of bob 2.0.5 through **the plugin's own
  `match_rules`**, lifted verbatim from the installed `bin/bob-watch`

## Summary

`rules.json` is the whole of the plugin's `blocked` reporting: Bob has no
approval hook event, so a permission prompt is invisible to the lifecycle
bridge and can only be read off the pane. Against the six captures, the shipped
file does this:

| capture | shipped `rules.json` | should be |
|---|---|---|
| `blocked-execute-command.txt` | *no rule* | blocked |
| `blocked-execute-command-scrollback.txt` | *no rule* | blocked |
| `blocked-signin.txt` | *no rule* | blocked |
| `idle-composer.txt` | *no rule* | no rule |
| `idle-after-turn.txt` | *no rule* | no rule |
| `idle-command-picker.txt` | **`trust_folder_prompt`** | no rule |

So the `blocked` half of the plugin is dark on every screen it exists to catch,
and it fires on one screen where nothing is blocked. **The false positive is
the more serious of the two**, because a wrong `blocked` is acted on: a
supervisor that waits for a pane to be free stops waiting, and the sidebar
lights up on an idle composer. A missed `blocked` only falls back to the hook's
state, which is the safe direction and is what the `_comment` in the file
already says to prefer.

### The false positive: the `/` command picker

Bob's `/` picker lists its own commands, one of which is

```
 │     /permissions - View and change this folder's trust level                 │
```

That one line satisfies `trust_folder_prompt`'s `all` (`(trust|do you trust)`,
via `trust level`) **and** its `any` (`this (folder|workspace|directory)`, via
`this folder`), and the rule has no `none`. The picker is drawn over a live
composer with the text kept, so this is an idle pane reported blocked.

The narrowest fix is to drop the bare `trust` alternative, which is the one
doing the damage, and keep upstream's own other spelling:

```diff
-      "all": ["(trust|do you trust)"],
+      "all": ["(do you trust|trust this (folder|workspace|directory))"],
```

Nothing is invented there — `do you trust` is already in the file. No capture
in the corpus shows Bob's actual trust screen, so the rule's *positive* side
remains upstream's to anchor; what posse can pin is the negative, that it
matches none of the six.

### The misses: `tool_approval_prompt` on the Execute Command dialog

Bob 2.0.5's approval dialog:

```
  ───────────────────────────────────────────────────────────────────────────
  Execute Command
  ───────────────────────────────────────────────────────────────────────────
  Command:          bd version; command -v bd
  Approve commands:
  ┌──┐┌───────┐
  │bd││command│
  └──┘└───────┘
  → Approve Once
    Always Allow Command for task
    Reject

    ↑↓ (1/3)

  Wait for input after execution: No (Tab to toggle)

  Press Enter to confirm
```

`tool_approval_prompt`'s `all` is satisfied (`Approve commands:`, `Approve
Once`). Every `any` alternative then misses:

| `any` pattern | why it misses |
|---|---|
| `\(y(es)?/n(o)?\)` | the dialog is a selectable list, not a y/n prompt |
| `[[:space:]]*[>❯][[:space:]]*(yes\|allow\|approve)` | Bob marks the selected row with **`→`** (U+2192), which is in neither `>` nor `❯` |
| `esc to (cancel\|reject)` | the footer is `Press Enter to confirm`; `Reject` is a *row*, not a footer verb |

### The misses: `sso_login_prompt` on the browser sign-in screen

A bob with no usable gateway token draws this instead of a composer — there is
nothing beneath it, and Esc or Ctrl+C exits the program:

```
                        ●∙∙ Complete sign-in in your browser…
                            (Press ESC or Ctrl+C to exit)
```

`sso_login_prompt`'s `any` matches (`browser`). Its **`all` misses**: the
pattern is `(sign in|log in|login|authenticate)` and Bob writes `sign-in`,
hyphenated. One character.

## The fix

`etc/herdr/herdr-bob/rules.json` in the posse repo is a fork of the shipped
file, and is the proposed replacement. Two rules are added ahead of the
existing four, and three of the four are amended; `license_prompt` is
untouched. The result is the `should be` column above, under four combinations
of grep and locale.

### Added: the two screens that actually exist

Both are ports of posse's own draft herdr manifest for Bob
(`upstream/bob/bob.toml`, filed with `upstream-bob.md`), which had already
keyed on exactly the right lines. The patterns existed; they were in the wrong
file.

```json
{
  "id": "bob_execute_command",
  "state": "blocked",
  "all": [
    "^[[:space:]]*Execute Command[[:space:]]*$",
    "^[[:space:]]*(>|❯|→)?[[:space:]]*Approve Once[[:space:]]*$"
  ],
  "none": ["esc to interrupt"]
},
{
  "id": "bob_signin",
  "state": "blocked",
  "all": [
    "Complete sign[- ]?in in your browser",
    "Press (ESC|Escape) or Ctrl\\+C to exit"
  ]
}
```

Three choices in there are deliberate and are the reviewable part:

- **A pair of line-anchored patterns in `all`, never a shared footer.** `grep`
  matches per line, so two patterns that cannot share a line are a two-line
  anchor. `Press Enter to confirm` would be the easy key for the approval
  dialog and is the footer of that dialog and nothing else *today* — posse has
  a rule for another agent that exists only because a footer reword silently
  returned a blocked screen to idle, so a dialog is keyed on its own heading
  and its own option instead.
- **The heading must be line-anchored**, because a *completed* call leaves
  ` Execute Command (completed)` in the scrollback. `^\s*Execute Command\s*$`
  excludes it, and `blocked-execute-command-scrollback.txt` is the capture that
  pins it: both spellings are on screen at once and it must still resolve to
  the live dialog.
- **The row marker is optional.** The dialog being up is the block, not which
  row the cursor is on.

### Amended: the existing four

```diff
       "id": "tool_approval_prompt",
-      "any": [..., "[[:space:]]*[>❯][[:space:]]*(yes|allow|approve)", ...],
+      "any": [..., "[[:space:]]*(>|❯|→)[[:space:]]*(yes|allow|approve)", ...],

       "id": "trust_folder_prompt",
-      "all": ["(trust|do you trust)"],
+      "all": ["(do you trust|trust this (folder|workspace|directory))"],

       "id": "sso_login_prompt",
-      "all": ["(sign in|log in|login|authenticate)"],
-      "any": ["browser", "https?://", "press enter"]
+      "all": ["(sign[- ]?in|log[- ]?in|login|authenticate)"],
+      "any": ["browser", "https?://"]
```

`press enter` is dropped from `sso_login_prompt`'s `any` because it is the
Execute Command dialog's footer too (MEASURED: present in both execute-command
captures) — a shared footer doing identifying work for a different dialog.

### A locale note worth having anyway

**No bracket expression should hold a multibyte character.** MEASURED
2026-10-03: `/usr/bin/grep` (BSD grep 2.6.0-FreeBSD) under `LC_ALL=C`
evaluates `[>❯→]` byte-wise, so the class degenerates to the byte set
`{>, E2, 9D, AF, 86, 92}` and matches a pure box-drawing line — `─` is
`E2 94 80`, and a box-drawing line is on every Bob screen. `(>|❯|→)` does not
match it, in any of {BSD grep, ugrep} × {`en_US.UTF-8`, `C`}.

This changes no verdict in the shipped file today, because its class is
followed by a required keyword. It is reported because the watcher does not
choose which `grep` or which locale it gets — the herdr server's environment
does, and a daemon started from launchd has no `LANG` at all — so a rule that
is correct only in a UTF-8 locale is a rule that is correct only by accident.
Hence the alternation above, and hence an assertion in posse's harness that no
bracket expression in the file contains a non-ASCII byte.

## How to re-run the measurement

`scripts/verify-herdr-bob-rules.sh` in the posse repo (`make
verify-herdr-bob-rules`), ~9s, nothing live touched but a read of the installed
checkout:

- it lifts `match_rules` verbatim out of the installed `bin/bob-watch` rather
  than reimplementing it, and refuses to report any verdict if the extraction
  does not look like the function it expects;
- it asserts every verdict twice — over the whole capture and over the last 40
  lines, which is `bob-watch`'s own `pane read --lines` default — and requires
  the two to agree, because a rule that only works thanks to a line far up the
  scrollback stops working when the pane scrolls;
- it runs a **control arm on the shipped file first** and requires both of
  upstream's failures to reproduce, so a green result cannot come from a
  harness that never read the screens;
- it replays everything under {BSD grep, ugrep} × {`en_US.UTF-8`, `C`} and
  fails if any verdict moves;
- and it starts the real `bob-watch` under a fake `herdr` to pin the two
  seeding properties the override depends on: a fresh `config_root()` is
  seeded from the plugin's own copy, and an existing copy is preferred and
  never refreshed.

## One note for upstream about that last point

The seeding is `[ -e "$RULES" ] || cp "$ROOT/rules.json" "$RULES"`, and
`$RULES` is preferred thereafter. That is a good shape for a user-tunable file
and it is what let posse fix this instance-side with no patch to the managed
checkout. It also means **a shipped fix to `rules.json` never reaches anyone
who has already run the watcher once**, because their copy exists. If these
rules are corrected upstream, something has to migrate or version the seeded
copy, or every existing install keeps the broken file forever.

Related, and the reason this was worth chasing on a box where the watcher could
not run at all: on this install the seeded copy had been **edited by hand**
five minutes after the plugin was installed, adding a rule that appears in no
commit and no plugin file. It fixed the Execute Command dialog, by keying on
the footer, and left both of the other problems in place. Nobody could have
known: nothing reports what is in that file, which is why posse's
`scripts/herdr-bob-rules.sh --check` now prints its rule ids and says
`HAND-EDITED` when they are neither upstream's nor the repo's.
