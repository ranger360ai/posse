# An allowlist drop that was an exact name, over a list that had grown a directory (ranger-base-jgipg)

The finding is laurie's, from verify ranger-base-ujp0c, over
ranger-base-a8qfg's credential-store mask — which is on main as
ranger-base-miyv2 (db0e46e2), so this is the correction landing one commit
behind the work it corrects rather than inside its replay.

## What was wrong

`readMaskDirAllowed` (internal/posse/seatbelt.go) binds the three members of
`credentialStoreAllowlist` back read-only under a sibling runtime's masked
credential store, less any the carve-out read-denies. It decided "read-denies"
by an EXACT name match:

```go
if containsString(c.DenyRead, m.path) { continue }
```

Two of the three members are files (`config.toml`, `version.json`) and the
third is a DIRECTORY (`sessions`, `dir: true`). For a file, equality and
containment are the same question. For a directory they are not: a `DenyRead`
literal at `<store>/sessions/<file>` equals no member name, so `sessions/` was
bound back read-only across it.

**And the deny was outvoted a second time, one level down.** `bwrapCarveArgs`'
`DenyRead` loop skips any literal for which `writeGranted(storeMasked, f)` is
true — "under the tmpfs already, and a /dev/null mount would only make its
name" — which is true for anything under a masked store whether or not the
allow arm just bound a path back over it. So no `/dev/null` was rendered for
the literal either, and the credential was readable inside the wall.

MEASURED 2026-11-14 in this worktree, by running the new render pin against
the pre-fix predicate (the rig is the committed pin, so the measurement is
repeatable: put `containsString(c.DenyRead, m.path)` back and the two arms
red). The rendered argv for a store with `DenyRead` at
`<store>/sessions/<cred>` carried

```
--tmpfs   <store>
--ro-bind <store>/config.toml  <store>/config.toml
--ro-bind <store>/sessions     <store>/sessions
--ro-bind <store>/version.json <store>/version.json
```

and no `--ro-bind /dev/null <store>/sessions/<cred>` anywhere. This
reproduces laurie's 2026-11-13 reading on the archived tree line for line.

## Why it was a finding and not prose

Nothing reached it, and nothing could: `credentialSiblingStores` spells each
sibling runtime's credential at that store's ROOT or inside a settings
subdirectory of it, and no row of it names a path under `sessions/` — that
table is the record and this page does not restate it. Reachability today was
ZERO. What made it live is that two RECORDS said the hole could not exist:

- the function's own doc comment — "a literal deny is never outvoted by this
  allow, in either mechanism (ADR 0001 — deny wins)";
- `TestACredentialStoreAllowDoesNotOutvoteAReadDeny`, whose NAME is that claim
  and whose body planted `<store>/config.toml` — a FILE member — and asserted
  the count drops by one. No arm planted a literal under the directory member,
  so the pin was green over the half of its own claim that did not hold.

A pin that is wrong about what it measures is the finding. The next row added
to either table — a store whose credential sits under a directory posse reads,
which is exactly the shape `sessions/` already is — would have landed the hole
with the suite green.

## Which half moved, and why

The CODE, which is the half ADR 0001 leaves no choice about: the alternative
was to strike the absolute out of the doc and rename the pin, leaving "deny
wins" broken at one wall by design.

`readMaskDirAllowed` now drops a member the carve-out read-denies AT OR
INSIDE, through a new `deniesReadAtOrUnder` built on `underDir` — the same
containment every other arm of this render already asks by
(`own()`/`credentialStoreOwned` carry it for the store itself), asked in the
direction a directory member needs: is the DENY inside the MEMBER.

**Dropping the whole member, not binding it back child-by-child.** It is the
conservative arm, it is what the doc already promised, and it is what makes
that second skip honest: with the member gone the store's own tmpfs covers the
denied path, so `writeGranted(storeMasked, f)` is "already behind the mask" in
fact and not only in its comment. Binding a holed directory member back
child-by-child would widen what the allow reaches on exactly the render that
has a deny in it.

## Where the rule came from

`sshDirAllowed` one shape over, whose allowlist is `config` and `known_hosts`
— both files. Its identical exact-match drop is complete for its own list, so
the rule was correct where it was written and was carried over to a list that
could not yet need the difference. `sessions` with `dir: true` is the first
directory member either allowlist has had. `sshDirAllowed` is untouched here:
the shape does not reach it until its list grows a directory, and that is not
this bead's to invent.

## The pin

`TestACredentialStoreAllowDoesNotOutvoteAReadDeny` is now four arms over where
the deny is spelled — the member's own name as a file member and as the
directory member (the two CONTROLS, which is what says a red inside-arm is a
hole and not a broken rig), a literal inside the directory member, and one
deeper inside it. `TestBwrapArgsDoesNotBindBackACredentialStoreDirMemberHoldingAReadDeny`
asserts the same claim in the rendered argv, both halves: the member is not
bound back, and the denied path is named nowhere in the render, with the other
two members still bound back so the pin cannot pass over a mask that allows
nothing back at all.
