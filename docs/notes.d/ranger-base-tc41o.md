# claude on the bwrap wall: a directory grant, and a refusal with no error behind it (ranger-base-tc41o)

ADR 0080 D19, built 2026-11-08 as `BwrapStateDirs` in
`internal/posse/bwrap.go` plus one branch in `credentialReadDenyLiterals`
(`internal/posse/seatbelt.go`), pinned by
`internal/posse/bwrapstatedirs_qa_test.go` (arm 1, 9 tests, ~1.4s). Still no
launch wiring: nothing calls `BwrapStateDirs` either, which is D18's
(ranger-base-djl3c).

Three things this had to settle, in descending order of how much they matter
to whoever picks up D18.

## 1. Why the refusal is a refusal and not a fallback

With `CLAUDE_CONFIG_DIR` out of force, claude resolves its config FILE to
`$HOME/.claude.json` (`ClaudeConfigFile`, trust.go — the runtime's file
resolver reads the truthy variable directly, which is the one place it and its
own directory resolver disagree). `$HOME` is granted by nothing, deliberately,
and a bind of the file itself refuses the atomic replace its writer makes
(probe B8, MEASURED). So the obvious fallback — grant `<home>/.claude` and
carry on — produces a wall that goes up, reports success, and loses every
in-session config write the seat makes: MCP adds, `projects[]`, history,
`seenNotifications`. That is ranger-base-cypy1's failure **with the grant in
place**, which is worse than no grant, and it is the shape the whole of
`SeatbeltSiblings` exists to close at the other mechanism.

So the render refuses, and the message is the deliverable: the bare failure
has no error in it at all. It names the variable, the mechanism that refused
(`seatbelt (bwrap)` from the constants, never `CageLabel`, which reads the
LIVE host and would say `sandbox-exec` on the box that pins it), what the
fallback would cost, and the one line of host procedure that fixes it for
every seat at once (`Environment=` in the home's unit,
ranger-base-yyq64/offid).

Empty is refused on the same arm. posse treats set-and-empty as out of force
everywhere (`runtimeHomeOverride`), and the runtime is not self-consistent
about it — `??` for the directory, so an empty value is a cwd-relative
directory; `||` for the file, so the same value sends the file to the home.
Accepting it would grant whatever the launcher's cwd happened to be.

## 2. `~/.claude` stays, and the reason is in another file

D19 says the `~/.claude.json` entry drops out. It does not say the declared
tree does, and the temptation is strong — with the variable in force the
config dir IS claude's state root, so `~/.claude` looks like a grant the
directory already covers.

Dropping it reds a seat's login. `credentialReadDenyLiterals`' `own()` is
keyed to the DECLARED `state_dir` literal, so a list that had lost `~/.claude`
reads there as "not a claude seat" — and on Linux that credential file is the
store of record (ADR 0019 D2), so the seat would be masked off the one
credential it authenticates with. Nothing about `BwrapStateDirs`' own output
says this, which is why
`TestBwrapStateDirsStillReadsAsAClaudeSeatToTheCredentialWall` drives the
coupling from the real output of one into the other rather than from a
hand-built list: it is the only arrangement under which the two can be wrong
together. MUTATION-CHECKED — widening the drop to the tree reds that pin and
the grant pin, and nothing else.

## 3. The non-darwin credential arm was "nobody", and should always have been
"not this seat's runtime"

Before this bead the branch read `if goos == "darwin"`. The reasoning behind
it was sound and half-applied: on darwin the file is the keychain FALLBACK
store and the keychain is the store of record, so it is denied even to
claude's own seat; elsewhere it IS the store of record, so denying it to a
claude seat would strand that session's own credential.

"Elsewhere it is the store of record" is a reason to spare the seat that OWNS
it. It was read as a reason to open it to every seat — so on Linux a codex,
grok or bob seat could read claude's credential, which is ranger-base-9fl's
posture with the runtimes swapped and no read deny anywhere saying so. The
arm is now `goos == "darwin" || !own("~/.claude")`, which is the rule the
three siblings below it have always followed, through
`credentialFileCandidates` either way — the resolver and not a literal, so it
follows `CLAUDE_SECURESTORAGE_CONFIG_DIR` and `CLAUDE_CONFIG_DIR` to wherever
the runtime writes next (ranger-base-x5f6p's lesson, now on both GOOS arms).

Two green pins reversed, both of them assertions of ABSENCE on the linux arm
(`seatbeltcredentialread_qa_test.go`, `credentialdenymove_qa_test.go`). Each
got its counterpart row in the same table rather than an edited expectation
on its own: the claude seat still spared, the codex seat on the same box now
carrying both spellings. The one in `credentialdenymove` was the dangerous
one — `want: nil` there was green before and after, so without the row beside
it the table would have gone on asserting "linux never denies claude's file"
as a fact about GOOS when it had become a fact about `own()`.

## 4. `.claude.json` is one const now

Three callers join that basename onto a base they resolved themselves
(`ClaudeConfigFile`, `SeedCageHome`, and this bead's drop), so
`claudeConfigFileName` lives beside the first of them in trust.go. A fourth
literal in bwrap.go would have been the drift this codebase has already paid
for twice on this exact file's path (ranger-base-wd4be, ranger-base-api7c).

## 5. For D18, three things this leaves

- **Nothing calls `BwrapStateDirs`.** The launcher has to, and it has to pass
  its output to `SeatbeltWritable` **and** to `SeatbeltCarveOut` — the second
  is what makes `own()` see a claude seat, and item 2 above is what happens if
  only the first gets it.
- **`internal/posse/bwrap.go` is still not in `WallRenderSources`**, the same
  deferral ranger-base-qoamy recorded and for the same reason: the list is
  "paths whose commits can change a wall THIS BINARY RENDERS", and nothing
  launches behind this one yet. internal/treepins' derived half will force it
  the day `RenderBwrap` writes `bwrap.args`.
- **The refusal is a LAUNCH refusal and belongs in `planLaunch`'s ladder**,
  not in `launchSession`: it must refuse before the argv ladder claims the
  bead. Said here because the function returns a plain error and nothing in it
  says which side of the claim it is meant to be read on.
