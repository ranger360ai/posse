# Credential rotation

*The four moves of ADR 0019 D5, all of them now decided: mint a session
token, read a guard failure, survive a `/login`, and keep watching the file
that comes back. `rangerhq-m10j`.*

## Which ADR 0019

Two ADRs carry the number and they say different things, so this page names
which one every time:

- **the harness ADR 0019** — `docs/adr/0019-credential-architecture.md`, in
  this repo. The credential seam, `posse refresh`, structural absence, expiry.
- **the instance ADR 0019** — the rangerhq tree, private (ADR 0012 D6: this
  repo is public and that page names one instance's topology). The keychain
  decision, the four guard failure classes, and D7. Accepted 2026-08-28.

Where they overlap the harness one is later and governs.

## The front door

```sh
posse refresh          # no arguments: the report. Nothing is written.
```

Every (runtime, purpose) this box has, where each credential lives, when it
dies or "cannot tell", and the action if there is one. Start here; the rest of
this page is what to do with what it says.

`posse refresh` is the operator's hand and only the operator's. It refuses
under `RHQ_PERSONA` and it refuses without a terminal — **both refusals apply
to the report too**, because the deny line that spells the same gate,
`Bash(posse refresh:*)`, does not know about arguments. Verified by running it:

```
posse: posse refresh is the operator's hand and nothing else (ADR 0019 D4):
       refusing under RHQ_PERSONA=developer-3 — every crew PID also denies
       Bash(posse refresh:*), and this is the same rule spelled in the binary
posse: posse refresh is interactive-only (ADR 0019 D4): stdin is not a
       terminal, so there is no human here to be the gate — run it from your
       own shell
```

If you are a persona reading this: those two lines are the whole of your
involvement. File the ask; do not work around them.

---

## 1. Cage: mint the session token

The caged tier has no keychain, so a caged claude authenticates from the env
set its PID names (`rangerhq-kiz`). That token is **posse-owned**: scoped,
minted by hand, revocable on its own without touching anything else.

```sh
posse refresh claude session --env-set container --expires 2027-02-01
```

What it does, in order: runs the runtime's own mint — `claude setup-token`,
whose browser flow **is** the human gate — with your terminal on all three
descriptors, so posse captures nothing and can log nothing. Then it asks you
to paste the value it printed (not echoed), and writes
`CLAUDE_CODE_OAUTH_TOKEN` into `envs/container.env`, mode 0600 in a 0700
directory, under a `# minted=YYYY-MM-DD` stamp.

- **`--expires` only when you know.** posse cannot ask a setup-token when it
  dies. With no stamp the report says `cannot tell`, which is an answer; it is
  never rendered as freshness, and no invented date is written.
- **`--paste`** skips the mint for a box with no browser: mint on one that has
  one and carry the value over. It is the same single code path that ever
  holds a token.
- **Name the set in the PID.** A token in an env set no `envs:` names is a
  token no launch will ever see. `posse envs` lists key names, never values.
- **Refusals before anything is written.** A value starting `sk-ant-api…`, or
  the name `ANTHROPIC_API_KEY`, is metered spending and is refused on the
  money line — a persona is never the one who decides to spend. A value with a
  line break in it is refused because an env-set line is one `KEY=VALUE`. An
  empty paste writes nothing.
- **Re-minting** replaces its own stamps and keeps your comments.

By hand, on a box without this posse: `claude setup-token`, then put the value
in `envs/container.env` yourself, `chmod 600`, and write the `# minted=` line
above it. That is the `kiz` flow the instance ADR named; `posse refresh` is
the same flow with the perms, the stamps and the money-line check done for
you.

codex and grok have no session credential decided (`cage_cred:` unset). A
caged launch on them refuses with that reason rather than starting a session
that cannot do anything.

---

## 2. Guard: the read failed — which of the five

The plan guard's credential is the **runtime-owned** rotating OAuth token,
and on darwin its store of record is the macOS keychain item
`Claude Code-credentials`. posse is the second reader of somebody else's
rotating credential. That is a known bad shape, taken deliberately: the
operator-minted alternative was probed and measured **HTTP 403** — a
setup-token is a valid credential that is not entitled to the usage endpoint.
So the guard stays on the keychain, and the failures are named instead.

Each class is a distinct sentence, and **the sentence is the diagnosis**.
Read the one you got, not the one you expected.

| what you see | what it is | the move | what it is NOT |
|---|---|---|---|
| ``keychain item "Claude Code-credentials" unreadable — this binary's keychain ACL may have been dropped by `make install`; grant access when prompted, or run `claude` once`` | the keychain ACL is per-binary, and every promotion of a new posse binary can drop this one's read | the sentence carries its own move now: grant access when macOS prompts, or run `claude` once | not a stale token — do **not** "refresh" |
| ``usage endpoint returned 401 Unauthorized: credential stale — run `claude` once to refresh`` | the OAuth token is stale; the read reached the endpoint and the endpoint said "not you" | run `claude` once and let it log in; its own next launch refreshes it too | not an outage, and not weather — waiting alone does not clear it |
| `usage endpoint returned 403 Forbidden: this credential is not entitled to plan windows — a setup-token never will be, and this is not a freshness problem` | the wrong **kind** of credential reached the endpoint — a setup-token pointed at `/api/oauth/usage`. Measured, not guessed | stop. Take the setup-token back out of the guard's path | **never** "run `claude` once to refresh": refreshing a credential that was never entitled returns the same 403 forever |
| `usage endpoint returned 429 Too Many Requests, retry after 1h00m` | rate limited, usually by something re-asking after a 401 | wait the cooldown out; find the poller before adding another | not unreadable, and not a credential condition at all |
| `keychain read refused by a posse gate shim: security (deny: Bash(security:*)) — posse's own gate, not a credential outage` | posse's own L1 gate refused a command posse itself ran. The keychain was never reached | nothing about the credential. The read now execs `/usr/bin/security` absolutely, so this is a regression guard: if you see it live, a read went back to a bare command name | not an outage. Reading it as one is what got `plan_guard_blind_max: 0` set for hours on 2026-08-24 — the shop's only automated brake switched off on a wrong diagnosis |

A sixth outcome is not a failure at all: **structural absence** — no meter
adapter for this runtime, or the platform's store does not exist. That is the
guard **OFF**, not blind. One witness line names the platform, the store it
would need and what would arm it; no blind clock starts and nothing parks.
`no meter credential source for <runtime> on <goos> — <what would arm it>`.

Three things this section will not tell you to do:

1. **Do not re-lock the keychain ACL.** The instance ADR's P2 ("revoke the
   ACL so `security` prompts again") is **withdrawn** and stays withdrawn
   until a non-keychain adapter exists. The guard still needs that ACL.
   Revoking it produces class 1 on the next pass.
2. **Do not copy the OAuth token into a file.** Not into `secrets/`, not into
   an env set. A copy of a rotating credential is a snapshot that disagrees
   with the source exactly when it matters — that is move 3 below, and it has
   already parked this fleet twice.
3. **Do not widen `plan_guard_blind_max` to make a class go away.** Every
   class above has a fix that is not "look away".

None of the five is fixed by editing an env set. The guard does not read
`envs/`.

---

## 3. After `/login`: the stale env token

`/login` refreshes the keychain item and writes the runtime's own file. It
does **not** touch `envs/*.env`, and nothing else does either.

So a `CLAUDE_CODE_OAUTH_TOKEN` sitting in an env set is a snapshot that a
fresh login does not reach — and it **silently wins**: Claude Code itself
warns, on `/login`, that a set `CLAUDE_CODE_OAUTH_TOKEN` makes new sessions
keep using the old token. When the crew's PIDs all name the same env set, one
stale copy there is the whole fleet, and the symptom is a 401 nobody can
explain from a box where the operator just logged in successfully.

**The rule (instance ADR 0019 D7):** `CLAUDE_CODE_OAUTH_TOKEN` belongs in
`container.env`, because the cage has no keychain, and **nowhere else**. An
uncaged session's store of record is the runtime's own login loop, and a
derived copy of it is a second thing to keep fresh.

**The check** is the report, not a grep:

```sh
posse refresh
```

One variable in two env sets prints *"more than one env set holds this
variable — the PID's `envs:` order decides which a launch gets"*, which is
exactly the condition D7 is about.

**The fix is the operator's hand.** Removing that assignment from a shared env
set is a credential-file edit, and personas do not edit credential files
(`ranger-base-0vd`, standing). A persona that finds this files the ask.

---

## 4. The recurring credentials file

```sh
make verify-credential-paths        # scripts/verify-credential-paths.sh
```

Run it at every rotation, and after any `/login`, `claude setup-token`,
`make install`, or keychain prompt. Exit 0 is clean, **1 is a finding**, 2 means
no config directory was present so nothing was measured — a 2 is not a pass.

It scans `$CLAUDE_CONFIG_DIR` (when set) and `~/.claude`, depth 1, for anything
matching `.credentials.json*`, and prints mode, size, birth time and content
mtime. It reads no content, deletes nothing, and never runs `security`.

**Anything printed is a finding.** The harness ADR counts the keychain and the
env-set mints as darwin's owned stores; a file here is a third one nobody
owns. `~/.claude` is in every runtime's writable set
(`internal/rhq/seatbelt.go`) and the rendered seatbelt profile's only deny is
`file-write*` — `grep -n file-read` over that file returns nothing — so every
same-user persona session below the container tier can read whatever sits
there. The preventive half (a `file-read` deny on the credential-file
literals, GOOS-shaped so a linux store of record stays readable) is
`ranger-base-hw18` and has not landed; until it does, this check is the whole
control.

### Why it is a glob and not the path

On 2026-08-23 the file was **renamed, not removed**:
`.credentials.json.stale-20260823`, same bytes, same directory, same mode 600. A
rename changes the name, not the exposure. Both ADR 0019s now word the check
as the glob — the instance ADR's D5 was reconciled under `rangerhq-m10j`, so
the two can no longer drift.

The `stale-` name is not evidence of anything, either. Measured
(`ranger-base-1lza`): the file **self-renews**. It is a recurring unowned
byproduct of some darwin auth flow, not a one-off leftover of an old login,
and "delete it once" was the implicit model that measured out a treadmill.

### What to do with a finding

1. **File the ask; do not delete it yourself.** Removing a live credential is
   the operator's call every time — that is how `ranger-base-66y` was handled.
2. **Deleting the file does not revoke the grant.** It closes the local
   file-read exposure only. Killing the grant at Anthropic is `/logout` +
   `/login`, which re-touches the keychain item the guard depends on and makes
   any `CLAUDE_CODE_OAUTH_TOKEN` in `envs/` stale (move 3 above). Two steps,
   and the second one is the operator's hand.
3. **Do not treat the delete as the control.** Measured: the operator deleted
   it at 2026-08-26 03:40 and a new 994-byte file was created at 11:47:07 the
   same day — 8h06m later — and nothing on this box noticed for two days
   (`ranger-base-m6cm`). Deleting it schedules the next run of this check; it
   does not end it.

### Reading the metadata it prints

`btime` separates a new file from a resurrected one — the deleted file was 1114
bytes, the regenerated one 994.

A **frozen content mtime while claude has been running daily** means this file
is not the active store: Claude Code rewrites its credential file on every token
refresh. Compare it against `history.jsonl`, `sessions/` and `projects/` in the
same directory, which move constantly.

**`atime` is not evidence of anything here.** Measured 2026-08-28 on this box's
APFS data volume: reading a file does not advance its access time. The script
does not print it, and "opened once, never since" cannot be read off it.

### Siblings

`~/.codex/auth.json` and `~/.grok/auth.json` are deliberately out of scope. For
those runtimes the file **is** the store, not a leftover, so the same matcher
would print a finding that is not one. They come into scope when those lanes
reach the cage tier and their `cage_cred:` is decided.

---

*The five sentences quoted in move 2 are pinned against the code that emits
them — `internal/rhq/credentialrunbook_qa_test.go`. If you edit one here and
the suite goes red, the code is right and this page is wrong.*
