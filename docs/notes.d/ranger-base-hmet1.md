# Four conjuncts of the systemd unit render, and a reader split on a disarmed home (ranger-base-hmet1)

ranger-base-zml8t's four findings over ranger-base-136b5, answered
2026-11-06. All four were the same shape: a value guard or an ADR-stated
requirement that the close reasoned about in prose and no pin read. None was
a wrong behaviour today — each was a conjunct that could be deleted, or was
never written, with the whole shipped suite green.

## What landed

| finding | where | what changed |
|---|---|---|
| 1 · the ExecStart binary had no witness | `internal/posse/systemdunit.go:526` | a pin, not a code change |
| 2 · `%` reached `After=`/`Requires=` raw | `validUnitName` | refused, with four rows |
| 3 · `$` was not doubled on a command line | `unitCmdArg` (new) | doubled, on command lines only |
| 4 · the three `autostart_via:` readers split on a disarmed home | `plugin/autostart.sh` | the hook stands down instead of refusing |

## 1 — the emit was in a different function from the logic

Every pin that asserts the `ExecStart=` line calls `RenderWatchUnit`
*directly* with a hardcoded exe (`/opt/posse`, `/usr/local/bin/posse`),
which is right for what each of them is about and means none of them reaches
the two lines in `CmdSystemdUnit` that compute the real one
(`os.Executable`, `filepath.Abs`). The two pins that do go through the verb
asserted `ExecStart=` was *present* and never what followed it, so
`exe = "posse"` passed the suite — the ranger-base-u4uwg shape.

`TestSystemdUnitStdoutIsTheUnitAndStderrIsTheNotes` now grades the emit
against the only thing the real caller can be graded against: the test
binary's own `os.Executable`, rendered through `unitCmdArg`. MEASURED: it
kills `exe = "posse"` on the absoluteness half and the identity half, and
`exe = "/usr/local/bin/posse"` — absolute, wrong binary — on the identity
half alone.

## 2 — a `%` is not a unit name, so it is refused and not escaped

`autostart_multiplexer_unit:` was interpolated into `After=`/`Requires=`
raw, bypassing the `unitArg` guard every other interpolated value goes
through, and `validUnitName` accepted a `%`. systemd expands specifiers in
those directives — its own shipped templates rely on it
(`serial-getty@.service` is `After=dev-%i.device`) — and the rendered unit
is **not** a template, so `%i` expands to the empty instance and `%n` to the
unit's own name. A `%` in the operator's unit name therefore orders the loop
behind a unit nobody typed: verbatim the second of the two failures
`AutostartUnitKey`'s no-default decision is written to prevent.

Refused rather than escaped, which is the fork the finding left open. A unit
name as `systemctl` prints it carries no `%` at all (systemd.unit(5):
alphanumerics, `:`, `-`, `_`, `.`, `\` and `@`), so a `%` is not a name
needing an escape — it is not a name. Doubling it would render
`rex%%i.service`, which is a unit nobody has either. That puts it in the
same class as the whitespace refusal already there.

Four rows in the existing refusal table: `rex%i.service`, `rex%n.service`,
`100%.service`, `rex%%i.service`. The suffix positive control
(`rex@main.service` among them) is unchanged, so template *instances* still
pass.

## 3 — `$` is re-read on a command line and nowhere else

`unitArg` doubled `%` and quoted whitespace and did not touch `$`.
systemd.service(5) ("Command lines") gives `ExecStart` basic
environment-variable substitution and says a literal dollar is written `$$`.
Quoting does not stand it down — the substitution happens after the line is
split — and the unit **sets** `Environment=RHQ_HOME=`, so a `${RHQ_HOME}`
in the binary's path would expand against a variable this very file defines.

The fix is a second function, `unitCmdArg`, and not a line added to the
first: `WorkingDirectory=` and `Environment=` are not command lines and
substitute nothing, so a `$$` written into either would reach the process as
two characters. `TestSystemdUnitDoublesADollarOnTheCommandLineOnly` pins
both halves, and MEASURED reds in both directions — doubling nowhere kills
the `ExecStart` half, doubling everywhere kills the `WorkingDirectory=` and
`Environment=` half and the `unitArg` assertions.

**Its limit, kept from the finding.** Of everything that reaches this
`ExecStart` only the binary's own path can carry a `$` at all — every config
value in the argv is `ParseInterval`-validated or digits-only — so the
reachable case is a posse installed under a path with a dollar in it.
Whether systemd substitutes inside the FIRST token rather than refusing the
unit is REPORTED from the man page, which says the program to execute may
not be a variable, and **not MEASURED**: this shop has no systemd. Doubling
is right either way for a literal dollar. Worth reading on the shakedown
(ranger-base-offid) beside the two decisions ranger-base-136b5 parked there.

## 4 — the disarmed home, and which fork was taken

The three readers of `autostart_via:` diverged on a home with no
`autostart_interval:` at all. MEASURED with a positive control, over
`autostart_via: launchd` and no arm switch: `AutostartVia` refused, naming
the key and both names; the hook exited 1, "is not a mechanism — nothing
armed"; G7 printed **no row at all**. The control — the same disarmed home
with `autostart_via: herdr` — exited 0, so the hook's exit 1 was about the
value and not about being disarmed. The consequence was a herdr
`[[startup]]` hook failing at every server start with no governance row
naming why. Nothing could catch it: `autostartViaTable` is armed configs
only, by design, so no row of the one input table the three readers share
could reach the shape.

The finding left two forks. **Taken: the hook stands down.** On a home with
no arm switch it names the value on stderr and exits 0 — the disarm's own
exit code — because it would arm nothing there whatever the key said, and a
refusal needs an arm to refuse. The typo goes loud at the moment it starts
mattering, which is the moment the home is armed.

**Not taken: G7 reads the owner outside the arm gate.** That fork keeps ADR
0080 D9's as-built sentence literal and pays for it in the one place this
file argues hardest against paying. G7's row is gated on the arm switch
deliberately: gating on presence alone made it say "autostart is armed"
about a config nothing will ever arm from, which is ranger-base-i6h, and
`govern.go`'s own comment calls a false alarm there a false alarm "on the
one row whose whole job is to be believed". With only two classes available
— URGENT means the shop is stopped — a disarmed home with a typo'd owner is
neither, and a third class or a new G-row is a design change and not this
bead's.

The arm **switch** and not the arm's value: a present-but-broken
`autostart_interval:` is a home that means to arm, so an unreadable owner
*there* is refused exactly as before — the more total of the two failures
(no mechanism will ever arm this home) and the one that speaks.

`autostartViaTable` gains a `disarmed` column, and
`TestAutostartViaTableOnADisarmedHome` is four readings of it: the value
reader refuses wherever the value appears, the hook stands down exit 0 with
the value named, the render refuses (for the *arm's* reason on a `systemd`
home), and G7 is silent. The column is the table recording a split rather
than a verdict derived from `owner`, because the readers are not meant to
converge there: three mechanisms in two languages, and only one of them runs
on such a home.

ADR 0080 D9 carries the amendment; INSTALL.md and `examples/config.yaml`
carry the clause, since both said "refused by all three of those readers"
without qualification.

## Not changed

ranger-base-zml8t's own "NOT FOUND" list stands: ten mutants over this
close's guards were already killed for the right reason, and
`seedShellControlMisplaced` reds in both directions. Nothing here touches
the argv, the `--write` refusals, or `AutostartWatchArgs`.
