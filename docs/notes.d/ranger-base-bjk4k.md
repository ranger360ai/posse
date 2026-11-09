# D21's gate answered refused, and the flip it refused is invisible to this suite (ranger-base-bjk4k)

ADR 0080 D21 — wall codex on Linux, because bwrap nests — built 2026-11-08 as
**nothing**, which was the gate's instruction. The bead's own terms: "do not
flip until the operator's bwrap-tier probe run shows T6/T6b/T6c as expected.
If T6 differs, close this bead with the reading and file to richard
`-l architecture`." T6 differs. This is the reading.

## 1. The reading, and what it is not

MEASURED on the D1 host by the operator (ranger-base-r5m98's comment carries
the whole table), Linux 7.2, bubblewrap 0.13.1, codex 0.167.0:

| row | expect | got |
|---|---|---|
| T6 codex sandbox: write in its workspace | allowed | **refused** |
| T6b write our wall grants, codex does not | refused | refused |
| T6c write neither grants | refused | refused |

So the gate is not met, `wallRendered` keeps `!rt.SelfSandbox` on both
mechanisms, and Linux codex keeps darwin's posture. That part is simple.

What is not simple is that **a refused T6 is not evidence that nesting is
broken.** `try()` reports `refused` for any non-zero exit and throws away the
stream that would say which, so the cell cannot distinguish codex's nested
sandbox denying a write from a codex that never built a sandbox at all. Two
candidate causes, both defects in the probe rather than in the design:

1. **The argv may never have entered a sandbox.** `codex sandbox` takes a
   `--permission-profile <NAME>` that T6 does not pass. MEASURED 2026-11-08,
   darwin 25.6.0, codex-cli 0.153.4: T6's exact argv shape is a usage error,
   `exit 2`, `error: the following required arguments were not provided:
   --permission-profile <NAME>`, before any sandbox is applied — and
   supplying the flag then wants a `[permissions]` table in the config
   (`Error: default_permissions requires a '[permissions]' table`). 0.153.4
   is not the host's 0.167.0, so this is **not** a measurement of the
   finding. It is a demonstration that the finding's row pattern — all three
   refused — is what a usage error produces.
2. **T6's wall is not the wall posse renders.** The prefix was
   `$BASE --bind $WT $WT --bind $W/out $W/out`, so under `--ro-bind / /` the
   seat had no writable `~/.codex` and no writable `/tmp`. The shipped wall
   grants both: codex declares `StateDirs: ["~/.codex"]` and
   `SeatbeltWritable` adds TMPDIR. codex needs it at startup — MEASURED, same
   box and version, it materializes `apply_patch`, `applypatch` and
   `codex-execve-wrapper` plus a `.lock` under `$CODEX_HOME/tmp/arg0/` before
   doing anything else, and on darwin an unwritable `CODEX_HOME` only warns
   (`could not create PATH aliases: Operation not permitted`).

**T6b and T6c do not rescue the run.** Both expect `refused`, so a codex that
refuses every write and a codex that never started satisfy them equally. Once
T6 is refused those two rows distinguish nothing, and D21's composition claim
— "a write needs both walls to allow it" — has no positive evidence in this
run at all. The one row carrying information was the one expected to pass,
and it failed silently.

## 2. The probe is amended, and the amendment is a control

`docs/adr/0080-bwrap-tier.probe.sh`, in its own commit because it is the
blocker to verifying D21 and not part of this bead's answer:

- `tryv` carries codex's **first** error line into the GOT cell. The first and
  not the last: codex's parser prints the complaint first and `For more
  information, try '--help'.` last, so T1's `tail -1` — correct for claude,
  which prints one line — carries the line that says nothing. MEASURED by
  running the amended T6z row on darwin, all three arms (the allowed arm, the
  `grep` arm, and the `sed` fallback for stderr with no `error` in it).
- `CODEX_HOME` is bound writable, so the wall under test is the wall posse
  renders.
- **T6z, the unwrapped control**: the same codex argv with no bwrap at all,
  expecting `allowed`. It is the row that decides how to read the other
  three. Refused there and T6/T6b/T6c say nothing about nesting. On 0.153.4
  that control is *already* refused, which is the whole point of having it.

The argv is deliberately **not** guessed into a shape 0.167.0 might prefer.
Nobody here can run that version, and trading a known-ambiguous row for an
invented one is not a measurement. The probe reports what codex says.

## 3. The part that was a real hole: a D21 flip is invisible on darwin

`wallRendered` takes no mechanism — it reads `WallMechanism()` nowhere — so
every pin on its ANSWER is read on the box running it. D21's flip is spelled
as a bwrap disjunct, `!rt.SelfSandbox || WallMechanism() == WallBwrap`, and on
darwin that disjunct is false. **MEASURED**: with the flip applied exactly as
D21 spells it, on this darwin box,

| pin | under the flip |
|---|---|
| `TestQASeatbeltWallRenderedIsTheTierAndNotTheName` (the `wallRendered` table) | PASS |
| `TestQANoLaunchPathSpellsTheSeatbeltWallPredicateItself` | PASS |
| `TestQAEveryLaunchPathThatRendersASeatbeltRefusesACredentialDirEnvSet` | PASS |
| `TestQAWallTableClaimsNoTierThisBuildCannotType` | PASS |
| `TestQATheHookTTYUnderACagedCodexPID` arm 1 | PASS (then skips) |

Five pins, every one of them about this predicate, and a landed D21 is green
across all five. The gate's answer would have lived in prose only, and the
one thing D21 changes is a write wall in front of a seat.

So `TestQANoPosseWallForASelfSandboxRuntimeOnEitherMechanism`
(`wallmechanism_qa_test.go`) has two arms:

- **Arm 1, the answer**, on this box, with a control: `wallRendered` is false
  for codex, and — the arm that keeps the absence from going vacuous — true
  for claude where the tier is available. An assertion that a predicate says
  no needs an arm where it says yes.
- **Arm 2, the source.** Parse `seatbelt.go`, find `func wallRendered`, and
  red on any mechanism identifier in its body (`WallMechanism`,
  `wallMechanismOn`, `WallBwrap`, `WallSandboxExec`,
  `wallMechanismRendered`). This is the arm the table above exists to
  justify. It follows the idiom the rest of the package uses
  (`parser.ParseFile(fset, "seatbelt.go", nil, 0)` from the package dir, like
  `drainstage_qa_test.go` and `githang_qa_test.go`) and reads **one function
  body**, not the tree — so it is not a tree-wide pin and needs no Makefile
  door.

**A tripwire with instructions, not a veto.** The day D21 lands on a reading
that supports it, arm 2 is meant to be deleted by that commit — and the
failure message carries the list that has to move with it: D21's as-measured
note, `parity.go`'s `seatbeltIncompatible` declared difference,
`reachability.go`'s SelfSandbox arm, and this pin.

Three mutants, all MEASURED red on the arm they aim at, tree asserted clean
between each:

| mutant | arm that red |
|---|---|
| `!rt.SelfSandbox` → `!rt.SelfSandbox \|\| WallMechanism() == WallBwrap` (D21's flip, verbatim) | arm 2, naming both identifiers and their positions |
| `return false && …` (the predicate stops saying yes) | arm 1's claude control |
| `!rt.SelfSandbox` → `(rt.SelfSandbox \|\| !rt.SelfSandbox)` | arm 1's codex assertion |

## 4. What is not decided here, and by whom

Whether D21 is worth another host run at all. The operator's own note on the
reading is that the D1 host is claude-only by ruling, which would make the
item moot rather than pending — and that is the architect's call, not this
bead's. Filed to the architecture lane with the reading. This bead changed no
behavior on either host, which was the gate's instruction and is also its
done-when: darwin output unchanged.
