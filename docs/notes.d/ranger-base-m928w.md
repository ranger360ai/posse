# A presence probe that read its own cage as the operator's answer (ranger-base-m928w)

2026-11-07. Filed out of ranger-base-8gh58's live acceptance (dinesh, `cage:
seatbelt`); fixed here.

## The reading

`posse runtime check codex`, from a caged crew seat, reported the sign-in
interstitial **NOT SILENCED** and said codex *"has nothing to authenticate
with."* It was wrong: a live login was present. MEASURED the same minute from
a Rex block — which the Rex server spawns OUTSIDE the seat's cage — where the
same probe reported the credential and the same directory listing succeeded.

The seat's seatbelt profile does not grant the codex runtime's home, so every
read of it from inside the cage answers **EPERM**. `codexSigninProbe`'s
question is presence (ADR 0019: presence, mtime, path, never the value) and it
asked it with `if st, err := os.Stat(p); err == nil` — so the failed read fell
through to the "not silenced" line. The errno was right there and nothing
branched on it: the caged failure is `operation not permitted`, not `no such
file or directory`.

Part of that wall is **posse's own**: `credentialReadDenyLiterals`
(seatbelt.go) denies a caged seat the SIBLING runtimes' credential stores by
design (ADR 0019 D2 store 3), the one this probe reads among them. So posse
put up
the wall and then read it back as a fact about the operator's box.

## Why it cost a live acceptance

That entry carries `danger:`, and ADR 0013 §2 makes an unsilenced danger
screen a **launch REFUSE for anything dispatched** — a pass refuses before it
claims. So from any caged persona seat a codex launch was refused for a reason
that was not true of the box, printing a remedy (the operator signs in, in
their own session) that had already been carried out. ranger-base-8gh58's live
acceptance could only run by moving both dispatch passes into Rex blocks,
outside the cage, for this reason alone.

It is also the rule's own words. Probes are tri-state and only "not silenced"
refuses (ranger-base-9r33); an EPERM on a presence check is the definition of
the third state. All three things ADR 0019 allows this probe to read were
unreadable, and none of them was absent.

## The fix

`probeAbsent(err)` (interstitial.go) is the one rule: **ENOENT and ENOTDIR are
absence, and they are the only two** — `danglingSkillLink`'s rule (skills.go),
for its reasons. An absent file is a reading, because these CLIs write these
keys themselves; every other errno is posse unable to look, which is no
reading at all. `unreadableSilence` is the UNKNOWN that says so, naming the
path and the errno; `probeCageNote` adds the cage on the two facts
`claudeTrustWriteRefusal` stands on — `fs.ErrPermission` AND an `RHQ_CAGE` in
this process's own environment — and asserts no cage it cannot see.

The launch verdict for that unknown is the one every other unknown already
gets — **no refuse** — and `DangerUnsilenced`'s comment and ADR 0013 §2 now
say that it was chosen for this reading rather than inherited: a refusal there
refuses on a fact about the SEAT, and herdr's `signin_menu` /
`signin_api_key` rules still name the screen `blocked`, so a launch that does
meet it fails by name. The accepted cost mirrors ranger-base-mzmv's: a caged
`runtime check` prints "state unknown" with the errno where an uncaged one
prints a reading, and the line itself says to ask from outside the cage.

## The census the bead asked for

Every probe in the interstitial registry, against an unreadable home:

| probe | before | now |
|---|---|---|
| `codexSigninProbe` | **lied**: no credential file, no environment key, so *"codex has nothing to authenticate with"* — a `danger:` row, so a dispatch refuse | unknown, naming the errno and the cage |
| `codexUpdateProbe` | did not lie, **by luck**: `tomlFlag` swallowed the config.toml error as `found=false` and version.json's own unreadable arm answered unknown. With config.toml unreadable and version.json readable — a per-file deny, which is the shape posse's own wall has — the "the menu is back" arm would have refused | short-circuits to unknown on the pin's own unreadable file; version.json's unknown splits absence from errno |
| `grokPrivacyProbe` | **lied**: "privacy_banner_acked unset — the banner re-arms every launch". No `danger:`, so it walled nothing; it was a false row about a visibility decision | unknown, naming the errno |
| `grokAutoUpdateProbe` | **lied**: "auto_update not set — pin not applied (make verify-grok-pin)", sending an operator to re-apply a pin that is applied | unknown, naming the errno |
| `bobFlagSilence` ×2 | not in the class — reads `BobCommand`, the launch template | unchanged |
| `bobSignInSilence` | not in the class — reads nothing, on purpose, and says so | unchanged |
| `claudeTrustProbe`, `claudeOutsideReadProbe` | **not in the cage-blind class**: their file is `~/.claude.json`, which a caged seat's L2 profile grants (`SeatbeltSiblings`), and they already answer unknown on *every* read error. Imprecise in the OTHER direction — an ABSENT config is a reading ("this directory is not trusted") reported as unknown — which never walls anything, because both rows are `Seeded` and `DangerUnsilenced` skips those outright | fixed under ranger-base-c2k6z (P3, filed here rather than fixed here — a different direction of error, on a file this cage can read, and no launch turns on either row): absence is now the reading it is, and a genuinely unreadable config goes through `unreadableSilence` like the other four, errno and cage note included. Pinned by `TestQAAbsentClaudeConfigIsAReadingAndUnreadableIsUnknown` |

The installed-version reader beside them (`codexInstalledVersion` →
`ProbeCLIVersion`) is an exec and not a file read; a cage that cannot exec
codex gets `""`, which `codexUpdateSilence` already reads as unknown.

## Pins

`interstitial_qa_test.go`, arm 1:

- `TestQAUnreadableHomeReadsUnknownAndNotAbsent` — the fixture IS the defect:
  every file the four probes name is present and says "silenced" (asserted
  first, so no arm can pass over an empty home), then the directories are
  chmod 0o000. Each probe must read unknown, must name the errno the kernel
  actually gave, and must stop saying its own "no" sentence. Then the cage
  arms (both facts, and no cage claimed without `RHQ_CAGE`), then the
  CONTROL: restore the mode, remove the credential file, and the probe says
  "nothing to authenticate with" and the same launcher refuses — absence is
  still a reading.
- `TestQAProbeReadsENOTDIRAsAbsenceAndClaimsNoCage` — a home that is an
  ordinary file cannot hold a credential now or later, so it is absence.
- `TestQAAnUnreadableCodexHomeDispatchesRatherThanRefusing` — the launch
  surface, which is the only reason any of this is a defect rather than a
  wrong line in a grid. MUTATION-CHECKED: with the errno branch disabled it
  prints `codex launch refused: … codex has nothing to authenticate with`
  and creates no session, which is the bead's own repro, end to end.
