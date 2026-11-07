# Verifying four closes, and the one field codex's answer is not vetted by (ranger-base-rucsu)

2026-11-06/07. Verify bead over four closes: `ranger-base-z7kle` (D6's OSC
7501 reporter and the `-c hooks={…}` declaration), `ranger-base-fwwl8` (D7's
trust ask), `ranger-base-hmet1` (four findings over `ranger-base-136b5`'s
systemd unit render), and `ranger-base-onwi1` (the quoted-alias hook skip,
closed by an install and a relaunch). All four verify. One LIVE finding and
two recorded lines came out of it; the findings bead is named on the verify
bead.

## The finding: the ask never reads the directory its answer is about

`internal/posse/codextrust.go`, `codexAskOver`. D7's ask sends `hooks/list
{cwds:[<the seat's cwd>]}` and decodes `result.data[]` — one entry per asked
cwd, each `{cwd, hooks[], errors[], warnings[]}`. posse reads that `cwd`
field for exactly one thing, the log line `codexAskNotes` prints, and never
compares it with the directory it asked about.

**MEASURED 2026-11-06** at main `474337e8`, darwin 25.6.0, through the file's
own fake-app-server seam (`CodexHooksAsk.Bin`), with the correct-cwd answer as
the control in the same run:

| the fake's answer is about | posse |
|---|---|
| the seat's own cwd (control) | renders the trust line |
| `/not/the/seats/directory` | **renders the trust line** |
| `""` — no directory named | **renders the trust line** |
| two entries, one of them the seat's, both carrying the session-flags layer | refused — by the DUPLICATE-KEY guard, not the count one |

The last row is the bound on the finding and the first reading of it named
the wrong guard: the session-flags layer comes back once per asked cwd with
the same synthetic keys each time, so the repeat is caught inside
`codexHooksState`'s loop and the `ours != want` count is never reached. So
what is unguarded is an answer about ONE directory that is not the seat's,
not every multi-directory answer.

**Why it is the shape this file exists for.** Every other contradiction in
codex's answer refuses: a managed hook, an empty key or hash, a byte posse
cannot put through both shell quoting and a TOML string, the same key twice,
`data[]` empty, a session-flags `command` that is not the one posse declared.
The duplicate-key refusal's own sentence is *"posse asks about ONE cwd, so
this is not two rows about two seats"* — which is written, and is not read
anywhere the cwd itself is. What an unvetted wrong directory costs is what
all of those refusals are written to prevent: the hooks the seat's OWN
project declares are absent from such an answer, so they stay untrusted,
codex draws the startup review dialog, and the seat submits nothing and
reports nothing.

**What a fixer has to measure first, and this bead could not.** What codex
actually ECHOES in that field for the path posse hands it. posse passes an
absolute, symlink-resolved path (`resolveLinks`), and nothing on record says
whether codex echoes it byte for byte — so a strict equality compare could
refuse a launch that was fine. `docs/adr/0076-trust-ask.probe.sh`'s `state`
arm prints the real answer and is where that reading comes from.

**The pin**: `internal/posse/verify_rucsu_qa_test.go`, GREEN, asserting the
hole, with the inversion in its failure message. MEASURED able to fail: with
`if inv.Cwd != cwd { return … }` planted in `codexAskOver`, the two HOLE arms
red and every existing `TestCodexHooksAsk*` arm stays green — so the guard is
compatible with every fixture already there, which is the other half of what
the fixer needs.

## Mutation battery over the three code closes: 31/31 killed

Each mutant reverted exactly one conjunct of what landed, and each was read
for WHICH test reddened. Restored and md5-checked between arms.

`ranger-base-z7kle` (11): the handler path unquoted; quoting only a path that
holds a space (the partial fix); `SessionEnd` reporting `idle` instead of
`clear`; the `app=` pair dropped; an unknown event falling through to
`working`; the panic guard removed; BEL for ST; a `:` allowed in an app
token; a wrong argv exiting 2; a relative handler accepted; only a newline
counted as a control byte (the partial fix).

`ranger-base-fwwl8` (11): the declaration typed after `Conf`; foreign hooks
not disabled; the managed refusal off; the count guard off; the `command`
drift guard off; the duplicate-key guard off; the quote refusal off; `cmd.Wait()`
dropped; `codexRPCAwait` taking the first message; the empty-token refusal
off; the absolute-cwd refusal off.

`ranger-base-hmet1` (9): the ExecStart binary replaced by a bare name, and by
a different absolute posse; the `%` refusal off, and narrowed to `%i`/`%n`
only (the partial fix); `$` not doubled on a command line, and doubled only
when the value needs quoting (the partial fix); `$` doubled in `unitArg` as
well; the hook refusing on a disarmed home again; and the dangerous one — the
hook standing down on an ARMED home too, which `TestAutostartViaTableIsRead-
ByTheHook` kills, so the fix did not widen past the arm switch it was scoped
to.

One mutant is worth its own line because the kill came from the wrong place:
allowing a `:` in an app token does NOT red
`TestHookReportRefusesAnAppNameThatIsNotOne`, whose doc says `:` and `=` are
both pairs separators and whose seven rows all carry a `=`, a space, an
escape, a newline or a slash beside the colon. The class is pinned — by
`TestCodexHooksAskRefusesBeforeStartingAnythingItCannotAsk`'s `codex:evil`
row, in the other file — but that table cannot fail on a lone colon.

## `ranger-base-onwi1` re-measured on a second seat

A close whose deliverable is a STATE has a shelf life, so it was re-read at
verify time from a different seat than the one that closed it.

**MEASURED 2026-11-06**, this seat's own shim, git 2.50.1, aliases planted
with `/usr/bin/git` so the shim never saw the write, a scratch repo with a
counting `pre-commit`:

- `posse version` → `0.5.3+474337e8`; `merge-base --is-ancestor` puts
  `2f1d59aa`, `fc72606f`, `44c31e30` and `638cdf48` (the raised floor) all
  inside it.
- `grep -c 'type=path' "$(command -v git)"` → **3** (0 on `1f4da269`), and
  all three are on the `config --show-scope --get-regexp` legs.
- `alias.qx = commit '-n' -q -m qx --`, invoked as `git qx f` → rc 1,
  `hookskip:no-verify`, HEAD unchanged, `pre-commit` fired **0**.
- controls in the same run: the unquoted alias refused; a direct
  `commit -n -q -m direct -- f` refused; and a plain `commit -q -m plain -- f`
  rc 0 with HEAD moved and `pre-commit` fired **exactly 1** — the arm that
  says the wall is not simply refusing everything.

## The two smaller live lines on the same bundle

- `internal/posse/codexreport.go:366`/`:379` · `posse hook-report --ask` with
  no app name falls past the ask arm's own guard (`len(args) >= 2`) into the
  shared wrong-argv arm, which answers **exit 0 with an empty stdout**
  (MEASURED). `runCodexHooksAsk`'s own doc is the claim it contradicts: "EXIT
  1 ON A REFUSAL, unlike every other path in this file … a refusal that
  exited 0 would be a launch script's green light over an empty trust line",
  and `TestHookReportAskArmRefusesAnUnpairedConfigValue` pins exit 1 for
  every OTHER malformed ask argv, including `--ask codex -c` with a dangling
  `-c`. The one that is one word short is the one that answers 0. Not
  reachable through `docs/adr/0076-trust-ask.probe.sh`, which always types
  the app. Escaped from `ranger-base-fwwl8`.
- `internal/posse/codexreport_test.go:246` ·
  `TestHookReportRefusesAnAppNameThatIsNotOne`'s seven rows all carry a `=`,
  a space, an escape, a newline or a slash beside the colon, so the pin whose
  doc says "`:` separates pairs and `=` separates one" cannot fail on a lone
  colon: the mutant that allows `:` in `plainStatusToken` leaves it green
  (MEASURED). One row — `"codex:x"` — is the whole fix. The byte class is
  pinned elsewhere in the same package, which is why this is a table row and
  not a defect. Escaped from `ranger-base-z7kle`.

## Recorded, no bead

- `docs/notes.d/ranger-base-z7kle.md`, the `state=clear` table · every one of
  its six readings has exactly ONE record in `program_status`, so whether
  `state=clear:app=X` removes only X's record or the whole array is
  unmeasured, and the prose does not say so. The reporter sends `app=` on the
  clear, which is the scoping it assumes. UNMEASURABLE FROM THIS SEAT — no
  `rex` CLI on a crew seat's PATH, only the app bundle — and it is one
  `printf` of two records and one clear for a seat that has one. Escaped from
  `ranger-base-z7kle`.
