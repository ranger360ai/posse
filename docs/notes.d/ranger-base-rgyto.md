# ranger-base-rgyto — the whole-file project-config branch now asks Lstat too

One finding from ranger-base-h0lla's verify of four closes, and the only one
whose fix changes what a process does.

## What was wrong

`projectConfigTrustFile` (internal/posse/parity.go) has two branches. The
KEYED branch decided presence with `os.Lstat` and said so in a comment:

> Lstat distinguishes a missing config from an existing path whose target
> cannot be read (including a dangling symlink). Only the former is clean.

The WHOLE-FILE branch — `len(ProjectConfigKeys) == 0`, or the entry is named
in `ProjectConfigWholeFile` — decided presence with `os.Stat`, which FOLLOWS
the link, and returned `""` when that stat failed. So a declared entry
existing as a **dangling symlink** was classified CLEAN: the launch was told
nothing and did not degrade.

MEASURED 2026-11-13 at the commit before the fix (linux 7.2.15 x86_64,
go1.27.3, this box), `projectConfigTrustFile` over the real `claude` runtime:

| shape | `.claude/scheduled_tasks.json` (whole-file) | `.claude/settings.json` (keyed) |
|---|---|---|
| dangling symlink | `""` CLEAN | degrade: unreadable |
| symlink to a body | degrade: present | degrade: matched keys |
| directory | degrade: present | degrade: not regular |
| absent | `""` | `""` |

The field is documented as "presence is the predicate", and a dangling
symlink is present — `Lstat` answers for it. The branch documented as the
conservative one was the permissive one for this shape, in a check whose
whole job is to fail closed.

Said plainly: a dangling link carries no body for the runtime to read at that
instant, so this is a PREDICATE INCONSISTENCY and not a demonstrated
execution path. What is demonstrated is that two entry classes of one runtime
disagreed about what "present" means.

ESCAPED FROM ranger-base-2zkcy, which introduced `ProjectConfigWholeFile`
with `.grok/lsp.json` as its only member. WIDENED BY ranger-base-g3pfl, which
made `.claude/scheduled_tasks.json` the second member — g3pfl's own
deliverable is verified and this is not its defect; it doubled the surface.

## The fix

The `Lstat` is HOISTED above the branch, so there is one presence check and
both branches ask it:

- `IsNotExist` → clean, for both branches, as before.
- any other `Lstat` error → degrade, "classification failed: unreadable".
  New for the whole-file branch, which used to call that clean; it is the
  conservative direction and the rule the keyed comment already stated.
- `Lstat` succeeds → the whole-file branch says "project config is present"
  without reading anything, which is what presence-as-the-predicate means.

The keyed branch's own `os.Stat` (the not-a-regular-file arm, ranger-base-92rt)
is untouched.

## The pin

`TestQAProjectConfigTrustWholeFileDanglingSymlinkIsPresent`,
internal/posse/projectconfigtrust_qa_test.go (arm 2). It reads the entries
off the runtimes rather than spelling them, so BOTH whole-file entries in the
tree are exercised — grok's `.grok/lsp.json` is what says this is about the
class and not about claude's cron file — and it keeps the three shapes that
already degraded beside the one row that moved, plus the empty-dir control.

Filed with the fix and not before it: a pin for the fixed behaviour is red on
arrival, and a pin for the old behaviour would have encoded the defect.

MUTATION-CHECKED 2026-11-13, same box: `os.Lstat` → `os.Stat` in the hoisted
presence check reds both new rows AND the existing keyed arm
(`TestQAProjectConfigTrustSymlinkedSettingsAreNotMissing`). One presence
check, one mutation, both branches — which is the hoist's whole point.
