# ADR 0070 — A PID `allow:` is friction under claude auto mode: the classifier is a second reader an allow rule reaches only as a declared exception, and posse renders that exception for its own session verbs alone

*Status: accepted 2026-10-09 (ranger-base-dlouc, from
github.com/ranger360ai/posse/issues/11, operator, work box) · owner: architect ·
sits under ADR 0002 §3 (`allow:` is runtime-native friction; this record says
what friction means on claude under `--permission-mode auto`), ADR 0035 (posse
writes no runtime home: the carrier is the launch's own `--settings` blob) and
ADR 0066 (a model that reads is a second reader; it never becomes the wall) ·
builds in the code beads named on ranger-base-dlouc (dinesh); the behavioural
measurement is the operator's on the work box and is filed beside them*

> The `--allowedTools` a PID renders is resolved BEFORE the classifier, but
> only for a call whose every shell segment matches a rule. A compound command,
> a substitution, a heredoc, an unlisted wrapper or a classifier-routed hazard
> class sends the whole call to the classifier, and the classifier reads intent,
> not argv. Nothing posse renders can make an allow BIND there. What posse can
> do is say so in the contract's words, and put the operator's one standing
> statement — "these are my fleet's own session verbs" — where the classifier
> reads standing statements: `autoMode.allow`, with `"$defaults"`, in the blob
> the launch already carries.

## Context (MEASURED 2026-10-09 on this box, claude 2.1.295, unless marked)

**The report (issue #11, work box, reported not re-measured here).** A
supervisor persona whose PID allows `Bash(posse new:*)` and `Bash(herdr pane
send-keys:*)` had both refused inside its seat by the auto-mode classifier with
`[Auto-Mode Bypass]`, unstably: the same verb had run five times earlier that
day. Chained commands and heredocs were refused where the bare verb ran. With
`send-keys` refused, the supervisor typed into an undetected runtime's pane and
no turn started — the session looked healthy and produced nothing (ADR 0013 §2's
shape) — and an operator-pre-approved rollout stopped one step from the end.

**What the launch renders today** (`internal/posse/agents.go`,
`internal/posse/runtime.go`). `DefaultAgentCommand` carries
`--permission-mode auto`, `--settings <blob>`, `--allowedTools <PID allow:>`
and `--disallowedTools <L0Spellings(deny:)>`. The blob is
`ClaudeFleetSettings` (`permissions.defaultMode: auto`, the auto-mode-setup
skill off, auto-memory off) merged with the env pin and the field pin. It names
no `autoMode` key. `realizeClaude` passes the allow list verbatim.

**What claude does with an allow rule in auto mode** (docs, permission-modes
and auto-mode-config pages, read 2026-10-09; the binary's own schema text
agrees):

- A NARROW Bash allow rule is resolved before the classifier and the call
  skips it. Only broad rules are suspended on entering auto mode (`Bash(*)`,
  wildcarded interpreters, every `Agent` and `Monitor` rule).
- A rule must match EACH subcommand of a compound command independently; the
  recognized separators are `&&`, `||`, `;`, `|`, `|&`, `&` and newlines. A
  `cd x && posse peek s` therefore matches `Bash(posse:*)` for one segment and
  nothing for the other, and the whole call goes to the classifier. A command
  substitution or heredoc that assembles the prompt is a segment of its own.
  The stripped wrappers are `timeout`, `time`, `nice`, `nohup`, `stdbuf`,
  `command`, `builtin`, `noglob`; `env VAR=x cmd` is not among them.
- Some calls route to the classifier even when a rule matches: writes to
  protected paths, critical-path removals, commands carrying per-command
  allowed domains. The binary's hazard table (read, not executed) marks
  `dangerousRemoval` and `backgroundOperator` as classifier-routed, so a
  `… &` background launch is classified whatever the allow list says.
- `autoMode.classifyAllShell: true` in ANY scope suspends every Bash allow
  rule while auto mode is active. Default false; unset on this box.
- Inside the classifier, four tiers: `hard_deny` (user intent and allow do
  not apply) · `soft_deny` · `allow` entries as exceptions to matching soft
  rules · explicit user intent for the exact action. Entries are prose. The
  classifier reads `autoMode` from user settings, managed settings and the
  `--settings` flag — never from project `.claude/settings*.json`. Entries
  from each scope are COMBINED (concatenated; the array-concatenation
  measurement in `internal/posse/fieldpin.go`, ranger-base-i7cy4, is the same
  mechanism — and it was measured on exactly this key, `autoMode.environment`).
- **`"$defaults"` is load-bearing.** `claude auto-mode defaults` prints 17
  allow rules, 72 soft-deny rules, 1 hard-deny rule and 21 environment slots.
  An array that omits the literal `"$defaults"` REPLACES that section's
  built-in list; the binary's splice function substitutes the defaults at the
  sentinel's first position and otherwise returns the user array alone. An
  `autoMode.allow` posse rendered without the sentinel would delete all 17
  built-in exceptions — "Local Operations", "Transient Retry",
  "Session-Created Job Cleanup" among them — for every seat.

**Which rules fire here.** `Auto-Mode Bypass`, `Tmux Self Drive` and `Create
Unsafe Agents` are SOFT rules (the docs' Danger box lists auto-mode bypass
among the soft blocks an omitted `"$defaults"` would discard). Their texts:
Tmux Self Drive is keystrokes to the agent's OWN pane; Create Unsafe Agents is
launching an agent loop with approvals or sandbox off; Auto-Mode Bypass is
working around the classifier, including "flags, config, aliases, shell
indirection, or extension points so that the Claude Code permission system
allows arbitrary code execution". Each carries the bar "**must name:** that
this flagged pattern is a false positive — fine to allow". `posse new` launches
a sibling `claude --permission-mode auto` session and `herdr pane send-keys`
types into a sibling pane: to a reader with no context both ARE the pattern,
and the only thing that clears them is the statement that this instance of it
is a false positive. An allow EXCEPTION is that statement, made standing.

**The classifier is not stable on argv, measured in this session.** Three
`python3 -I ctx.py <strings dump> '<word>' <n>` reads over a `strings` dump of
the claude binary ran; the fourth, byte-identical in shape and differing only
in the words searched for (`carve-out`, `auto-stripped`), was refused
`[Auto-Mode Bypass]`. No allow rule was in play. The reader judged intent from
the words, not the command's shape — which is what "a second reader" means and
why no rendered rule binds it (ADR 0066).

**`posse gates` prints no allow row.** `posse gates monica` (a PID with
`allow: [Bash(posse:*), Bash(git push:*)]`) lists every deny with its class and
no allow at all: `parity.go`'s `Realized` map is keyed from the deny loop. The
issue's "renders these as realized" names `Realized.Realized`, the L0 list
`realizeClaude` fills with allow + deny for the parity count, not a printed
claim. There is nothing to downgrade on screen; what was missing is the
sentence.

**The adjacent finding, measured.** The operator's user scope
(`~/.claude/settings.json`) carries `autoMode.environment` with 24 entries and
no `"$defaults"` — the documented shape `/auto-mode-setup` writes. `claude
auto-mode config` from this worktree shows those 24 as the effective
environment: the shipped "Host containment" slot is absent, and the "Trusted
repo" entry names the ranger-base instance repository, where the shipped slot
reads "the git repository the agent started in (its working directory)". User
scope reaches every seat on the box, so a posse worktree seat is told its
trusted repo is a different repo. A last entry is prose about `rhq/` and
`.beads/` prefixes "within this repo". No `autoMode.allow`, no
`classifyAllShell`; `settings.local.json` holds 12 `permissions.allow` rules.

## Decision

**D0 — do nothing** was priced first and rejected: the failure is measured
(a stopped rollout, a healthy-looking seat producing nothing), and the only
remedy without a change is an operator edit to a user-scope file on every box,
whose audience is every session there (ADR 0035 rejects that shape as posse's
to make; it stays available as the operator's own, below).

**D1 — the contract sentence.** Under claude `--permission-mode auto` a PID
`allow:` rule is FRICTION REMOVAL and nothing else: it lets a call whose every
shell segment matches skip the classifier. It does not bind the classifier, and
posse never says it does. Three holes are named, in this order, wherever the
claim is restated: (i) shape — compound separators, substitution, heredoc,
unlisted wrappers, the background operator; (ii) classifier-routed classes —
protected paths, critical-path removals, per-command domains; (iii)
`classifyAllShell` in any scope. `posse gates` keeps printing no allow row; if
one is ever added it prints class `friction`, never `cooperative` or
`enforced`, and `posse agent check` passes a PID on its allow list exactly as
today (the allow list is the PID's words, ADR 0002 §3). ADR 0002 §3 gains one
sentence pointing here.

**D2 — the carve-out rides the blob, for posse's own verbs only.** When a
PID's `allow:` grants any of posse's or herdr's session verbs — `posse new`,
`posse prompt`, `posse kill`, `posse peek`, `herdr pane list|get|read|
send-text|send-keys|wait-output`, or a prefix rule that covers them
(`Bash(posse:*)`, `Bash(herdr:*)`, `Bash(herdr pane:*)`) — the claude launch's
`--settings` blob carries

```
"autoMode": {"allow": ["$defaults", "<the carve-out>"]}
```

and otherwise carries no `autoMode` key at all. The carve-out is ONE fixed
prose entry owned by posse (`agents.go`, beside `ClaudeFleetSettings`), the
same bytes on every PID that earns it; it is derived from the existing allow
list and adds no PID key, no flag and no state. Its text meets the rules'
own bar and keeps their exclusions:

> Posse fleet session verbs: when this session's permission allow list grants
> them, `posse new`, `posse prompt`, `posse kill`, `posse peek` and `herdr
> pane list|get|read|send-text|send-keys|wait-output` are this persona's own
> verbs. They start, prompt, read, drive and end SIBLING terminal panes on this
> machine, owned by the operator and pre-approved through this persona's
> permission document; every session they launch carries its own permission
> document, gates and `--permission-mode auto`. A flagged Auto-Mode Bypass,
> Tmux Self Drive or Create Unsafe Agents on one of these verbs is a false
> positive — fine to allow — whether the verb stands alone, follows a `cd` or
> `&&`, or takes its prompt text from a heredoc or `$(cat …)`. Keystrokes aimed
> at THIS session's own pane (`$TMUX_PANE`) are not covered, and neither is a
> bare `kill`, `tmux` or `claude`.

Rendering rules, each a pin: `"$defaults"` is the FIRST element and a payload
without it is refused by the test, not shipped (a mutation that drops it turns
`TestQAClaudeFleetAutoModeAllowKeepsTheDefaults` red); the key rides inside the
one `--settings` (a second flag replaces the first, measured 2.1.259); it is
rendered by `ClaudeFleetSettingsJSON` at the site where `ag.Allow` is already in
scope (`RenderCommandForModel`), so `FleetSettings` takes the allow list; it is
NOT added to `SettingsPin` — a hand-written `command:` template gets the
security pin only (ADR 0035's asymmetry), and such a PID that wants the
carve-out names `autoMode.allow` itself.

**D3 — what is claimed and what is not.** D2 is claimed to REDUCE refusals of
the named verbs in compound shapes, never to end them: the exception is prose
read by a model, and the stability measured above is the classifier's, not
ours. Until the work-box probe below reports, the reduction is ASSUMED. A
persona whose work hangs on one of these verbs still writes its command in the
matching shape first — bare verb, prompt file written in a separate call —
because the shape hole (D1 i) is the hole an allow rule DOES close.

**D4 — the operator's remedies, named because the contract names them.**
One-off: state the exact action in the next turn (the fourth tier). Standing,
box-wide: `autoMode.allow` with `"$defaults"` in `~/.claude/settings.json` —
audience every session on the box, so a visibility move posse never makes
(ADR 0035). The environment finding is the operator's too: the "Trusted repo"
entry can be returned to the shipped wording or re-drafted, and the "Host
containment" slot restored, by `/auto-mode-setup` or by hand; posse does not
splice `"$defaults"` into `environment` from the blob (rejected below).

## Consequences

- The dispatch contract gains a true sentence and loses an implied one: a PID
  allow is friction on claude, and `posse gates`' silence on allow rows is
  correct, not a gap.
- Supervisor-shaped PIDs get one standing statement to the classifier in the
  operator's voice, scoped to the PIDs that already hold the verbs. Dinesh's
  seat, which holds none, renders no `autoMode` key and changes by zero bytes.
- The blob grows by one key on those PIDs. The payload-shape hazard stands
  (fieldpin.go: one wrong-typed row voids the whole `--settings`), so the
  carve-out is an array of strings and nothing else, and the live
  `auto-mode config` probe is the canary that the payload still merges.
- Cost carried: one const, one function parameter, two pins, one live probe.
  No new PID key, no new state, no new actor.

## Alternatives rejected

- **A new PID key (`auto_allow:` prose per persona).** The racing signal:
  a second registry for one bead's need, and prose the operator must keep in
  sync with `allow:`. ASSUMED cost one key, every PID author; rejected while
  the fixed carve-out covers the only measured case.
- **Derive a carve-out from EVERY allow rule.** Posse cannot know WHY an
  operator allowed `git push`; a generated "X is fine to allow" for each rule
  is the classifier's Auto-Mode Bypass clause (iii) written out, and would
  earn the refusal. Only verbs posse itself owns get posse's words.
- **`classifyAllShell: false` in the blob.** Already the default; a key that
  restates a default is noise with a payload-void hazard attached.
- **Write `autoMode.allow` into `~/.claude/settings.json`.** ADR 0035: posse
  writes no runtime home, and the audience is the operator's own sessions.
- **Splice `"$defaults"` into `environment` from the blob** to restore the
  missing "Host containment" slot. Arrays concatenate, so the flag's sentinel
  would add the shipped 21 BESIDE the operator's 24 — two "Trusted repo"
  entries that disagree, read by the same model. The environment is the
  operator's file; filed as their decision.
- **A PreToolUse hook or `permissions.ask` recipe.** Those run BEFORE the
  classifier and can only block or prompt; nothing that runs before a reader
  can approve on its behalf, and a prompt is the failure mode
  (rangerhq-qs5r).
- **Deny-list the compound shapes so the model learns the bare one.** The
  bare one already runs; a deny that fires on the shape teaches nothing a
  refusal did not, and ADR 0002 §3 keeps allow and deny as different animals.

## Verification (run before close; laurie quotes these)

1. `go test -run 'TestQAClaudeFleetAutoModeAllow' ./internal/posse`: a PID
   with `Bash(posse new:*)` renders the key with `"$defaults"` first and the
   carve-out second; a PID without the verbs renders no `autoMode` key;
   dropping the sentinel from the const turns the pin red.
2. Live, any seat: `claude --settings '<rendered blob>' auto-mode config | jq
   '.allow | length'` prints 18 (17 defaults + 1) and `.allow[0]` is "Security
   Discussion…", against 17 with no blob — the splice kept the defaults and
   the payload merged (a voided payload prints 17 and is the fieldpin hazard).
   DONE (ranger-base-j5b24), and it is a test rather than a hand command:
   `RHQ_LIVE_CLAUDE=1 go test ./internal/posse -run TestLiveClaudeAutoMode`
   (internal/posse/automodeallow_live_test.go, arm 3) runs four arms against
   the real reader — control, pin, the blob with the sentinel removed, and the
   blob plus one wrong-typed row. MEASURED 2026-10-09, claude 2.1.295, this
   box: allow 17 shipped · 17 control · **18 pin**, the carve-out last and
   "Security Discussion…" first · **1 attack**, the carve-out alone with all 17
   deleted · **17 void**, the carve-out absent. soft_deny 72, hard_deny 1 and
   environment 24 read the same in every arm, so the key's blast radius is the
   one section. The counts are read against the CONTROL rather than against
   zero, because the voided arm is what "non-empty" looks like.
3. Work box, operator: with the supervisor PID relaunched on a build carrying
   D2, run the compound shape that was refused (`cd <dir> && posse peek <s>`
   or the heredoc-prompted `herdr pane send-keys`) ten times with and ten
   without the carve-out; record refusals/10 for each arm on the operator
   bead. That number replaces D3's ASSUMED.
4. `make fmt-check` and `make tree-check` after the filtered runs; `make
   crew-check` because `internal/` and a `*_test.go` change.

## Claims

**MEASURED (2026-10-09, claude 2.1.295, this box unless marked)**
`claude auto-mode defaults`: 17/72/1/21 · effective config here: 17/72/1/24
with no `"$defaults"` in the user environment array · the splice function
substitutes defaults once at the sentinel, else returns the user array ·
`autoMode` schema keys `allow`, `soft_deny`, `hard_deny`, `environment`,
`classifyAllShell`, legacy `deny` · the three rules' texts and bars as quoted ·
`posse gates monica` prints no allow row · one byte-identical-in-shape read
refused `[Auto-Mode Bypass]` after three ran · the docs' statements on
compound matching, stripped wrappers, scope combination and tier precedence
(read 2026-10-09, not executed).
**REPORTED (issue #11, work box)** the refusals, their instability and the
shape dependence.
**ASSUMED** that an `allow` exception reduces the named verbs' refusals in
compound shapes (D3; Verification 3 replaces it) · that no supervisor PID
needs the verbs through a hand-written `command:` template (if one does,
D2's last rule is its instruction).
