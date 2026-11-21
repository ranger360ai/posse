## `posse upgrade` stages where the deny was rendered, not where its own TMPDIR points (ranger-base-18brx)

One LIVE finding from the verify at ranger-base-m5izv, over the close of
ranger-base-ua625. That close narrowed the blanket temp grant every rendered
seatbelt profile carries: instead of the copy sitting directly in `$TMPDIR` else
`/tmp`, it moved one fixed level down — `<temp root>/posse-upgrade` — and
`SeatbeltCarveOut` denies that name in every profile, whether or not an upgrade
is running. `make test` runs out of that copy, UNCAGED, as the verb's caller, so
a seat able to write it for the suite's twenty minutes chooses what the
operator's box executes.

**What was still open is which temp root "the temp root" is.** There are two
readings of it and they are taken by different processes:

- `upgradePrivateDenied(GOOS, $TMPDIR)` is rendered by the process that LAUNCHED
  a seat, minutes or days before the verb runs, and names one child per blanket
  grant **root**;
- `upgradePrivateRoot()` was `filepath.Join(os.TempDir(), …)`, read by the
  process that RUNS the verb.

A deny over the grant roots is not a deny over the grants, because `os.TempDir()`
can name a path strictly UNDER one of them. Where the two disagreed that way, the
copy was inside a seat's write grant under a name no deny covered. The close's
own comment raised the case — "a launch's TMPDIR need not be the upgrade run's",
given as the reason the deny covers every grant — and did not close it.

### MEASURED 2026-11-21, D1 (linux 7.2.15-1-omarchy, go1.27.3)

Through the production functions, in an in-package probe, no fixture. For each
row the seat's profile is `seatbeltTempGrants(goos, launch)` granted and
`upgradePrivateDenied(goos, launch)` denied; "reaches" and "denied" are
`writeGranted` over those two lists. Reproduces the verify's reading, and adds
the last column:

| goos | launch TMPDIR | verb TMPDIR | pre-fix root | reaches | denied | post-fix root | denied |
|---|---|---|---|---|---|---|---|
| linux | `""` | `""` | `/tmp/posse-upgrade` | yes | yes | `/tmp/posse-upgrade` | yes |
| linux | `""` | `/tmp/op` | `/tmp/op/posse-upgrade` | yes | **NO** | `/tmp/posse-upgrade` | yes |
| linux | `/tmp/seat` | `/tmp/op` | `/tmp/op/posse-upgrade` | yes | **NO** | `/tmp/posse-upgrade` | yes |
| linux | `""` | `/var/folders/ab/cd1234/T` | `/var/folders/…/posse-upgrade` | no | no | `/tmp/posse-upgrade` | yes |
| darwin | `/var/folders/ab/cd1234/T` | `/tmp/op` | `/tmp/op/posse-upgrade` | yes | **NO** | `/tmp/posse-upgrade` | yes |
| darwin | `""` | `/var/folders/ab/cd1234/T` | `/var/folders/…/posse-upgrade` | no | no | `/tmp/posse-upgrade` | yes |

The rows that reach and are not denied are the shape. The rows that reach nothing
are the fail-safe direction and are why it was narrow: a verb TMPDIR outside
every blanket grant is outside every seat's reach as well, so only a verb TMPDIR
strictly INSIDE one of them was exposed. Nothing on this box is in the shape —
TMPDIR is unset in a dispatched session, a bwrap seat is handed `TMPDIR=/tmp`
(row 1), and the verb is the operator's or `install_delegate`'s, uncaged, so both
readings are the operator's own shell. HARDENING, not an incident waiting; what
earned it a bead is that the exposure is uncaged code execution and the
precondition is one exported variable.

### The fix: the root reads the deny set, so the two cannot disagree

`upgradePrivateRoot()` is now the last member of `upgradePrivateDenied(GOOS, "")`
— the deny over the grants `seatbeltTempGrants` names whatever its arguments,
whose last entry is the `/tmp` it appends unconditionally and for every GOOS. One
expression, two readers. Three things follow:

- every rendered profile denies that name, whatever TMPDIR the process that
  rendered it read, since the grant it hangs off is in every set that function
  returns;
- at bwrap the seat's `/tmp` is a private bind (bwrap.go 1c), so on Linux the
  staging root is not in a seat's view at all — where a root under some other
  TMPDIR need not have been;
- `/tmp` is the one temp root this tree's suite was MEASURED green in
  (ranger-base-6ca29, quoted in `upgradeRunPlan`). "Under `$HOME`" and a quota'd
  tmpfs both read as "the commit is red", and a verb run with either as its
  TMPDIR used to stage the copy — and so run the suite — there. The copy's own
  location is no longer one of that matrix's variables; the suite still inherits
  the verb's TMPDIR for its own temp dirs, so the matrix otherwise stands.

`upgradePrivateCopy` also ASKS, before it fetches: `upgradePrivateRootDenied`,
the same deny set, refusing a root no profile denies. Redundant by construction
today, and it is the arm that refuses rather than reds if a later edit reads an
environment variable here again.

The per-launch members of the deny set stay, and cost a line: `$TMPDIR/posse-upgrade`
is the root an OLDER installed binary stages under, which is exactly the binary
running during the upgrade that installs this one.

### Mutations, same box and date

`TestQAUpgradeStagesThePrivateCopyWhereEveryProfileDeniesWrites` grades the
agreement; before this it set ONE TMPDIR and used it for both halves, so the one
thing its five rows could not see was the two halves disagreeing.

| # | mutation | pin | why |
|---|---|---|---|
| m1 | the root back on `filepath.Join(os.TempDir(), …)` | RED | row 1a: the root moves with the verb's reading |
| m2 | `denied[0]` for `denied[len-1]` | GREEN | on purpose — every member of that set is denied in every profile, so which one is not the pin's business |
| m3 | the verb's own refusal deleted | RED | row 6 |
| m4 | `upgradePrivateRootDenied` always true | RED | the predicate's false side: a root one level down inside a grant |

Row 6 is how m3 is reachable at all. No TMPDIR can make the refusal fire once the
root reads no TMPDIR, so the pin reaches it the one way left: with the grant set
stubbed empty there is no denied name to stage under, and the verb says so and
fetches nothing — fail-closed, and a row, because a belt no test reaches is a
belt the next edit deletes green.

Rows 4 and 5 write at the staging root, and row 5 plants a link at it. Their
isolation used to come from `t.Setenv("TMPDIR", <fixture>)`, which no longer
moves the root; they stub `seatbeltTempGrants` to the fixture instead, and the
root moves with it because it is derived from it. The agreement rows above it
grade the production function.

### Not closed here

- A seat launched before a deny existed carries no deny for it: a profile is
  fixed at its own launch. That is every deny's property, it is stated in
  `SeatbeltCarveOut` and in the close this amends, it cannot be closed from the
  render, and the remedy is the one it always was — a seat launched after the
  swap carries the new profile.
