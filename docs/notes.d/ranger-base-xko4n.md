# Two refusals a launch that are the CLI reading its own credential (ranger-base-xko4n)

From github.com/ranger360ai/posse issue #8, filed by the operator out of the
work-box shakedown (ranger-base-x4g3h), MEASURED on a real install
2026-10-08. The sibling of issue #6 (ranger-base-sl5sg) one surface over:
the same shim, read off the log instead of off the PID file.

## The state

At every claude session start the `Bash(security:*)` shim logs two refusals
into `state/gates/<persona>/refusals.log` — the runtime's own keychain item
and its legacy-named twin, probed as the CLI comes up. The session comes up
anyway, on the session mint posse injects, so the refusal costs nothing but
the lines.

The refusals are correct and the rule stays: the gates dir leads the PATH of
the runtime PROCESS and not only of the persona's tool shells (ADR 0002 §3),
claude's credential path execs that CLI by bare name, and the shim in front
of that read is what keeps the operator's store of record single-writer
(ADR 0019 §3, "mint before runtime"; formerly ADR 0042 D1–D2). Dropping the
rule is the one move that page forbids.

What the operator asked for was the other half: the log cannot tell the
CLI's startup probe from a persona reaching for the keychain, so document it
as expected, and optionally tag the startup pair so a real reach stands out.

## The re-measurement

MEASURED 2026-10-09 on the reference box, over every `refusals.log` its gate
dirs hold (13 of 22 personas have one), spanning 2026-08-23 to 2026-10-09 —
the same census ADR 0042 ran on 2026-09-02, seven weeks on:

| | lines |
|---|---|
| every refusal, every persona | 23,274 |
| the keychain CLI | 9,850 |
| …its read verb (the `find` verb) | 9,801 |
| …its item removal — the runtime's legacy-item cleanup, ADR 0042's nine | 13 |
| …every other verb: `help`, `show-keychain-info`, `-i`, `-h`, bare, `unlock-keychain`, `dump-keychain` | 36 |

So the subtraction rule still holds its shape and the proportions have not
moved: 9,814 of the 9,850 keychain lines are the runtime, and the 36 left
are the ones the deny was written for. ADR 0042 measured 10,279 lines with
~4,600 read forms and "fewer than thirty persona-shaped"; the read forms
have doubled against a log that also doubled.

Two numbers worth keeping beside each other, because they are the operator's
"two per launch" and the ADR's "about twice an hour" in one reading: the
runtime's own item accounts for 6,761 of the read lines and its legacy-named
twin for 2,689. The twin's count is the launch count; the item's is the
launch count plus whatever the session re-reads. The config-dir-hashed
variants ADR 0042 names are the long tail — 20 lines at the largest, in
pairs.

## What landed, and what did not

Docs only, which is what the bead asked for first:

- **INSTALL.md §7**, inside the `posse gates <persona>` step, where the
  operator meets `refusals.log (last 10)` — the block titled "The keychain
  deny also stands in front of the runtime's own credential read, and those
  refusals are expected". It says why the rule stands, what the two lines
  are, that no tag is coming and why, and gives the subtraction as a
  one-liner over the log with a Verify.
- **INSTALL.md §14**, a Troubleshooting row keyed on the symptom the
  operator actually had — the log is almost all one verb — whose fix is
  "nothing", pointing back at §7.

**The tag was declined, and not for speed.** ADR 0042 D4 already ruled it,
and ADR 0019 §3 carries the ruling forward in one sentence: a second
caller-classified log cannot prove its classification. The shim reads argv,
never the caller. Alternatives 3 and 7 on that page close both of the
obvious mechanisms by measurement — discriminating by parent process is
defeated by an `exec` from the tool shell for free, and a second log file
would file a persona's keychain use under "runtime". So the distinction is a
rule for READERS, which is a doc, which is what landed. An `expected:` tag
would do worse than nothing: it would mark a genuine persona reach as
expected the moment it asked for the same item.

"Once per launch" has no implementation either. The shim is a stateless
POSIX `sh` script re-entered per exec (gates.go, `renderShim`); it holds no
per-launch counter and nothing it could key one on.

Two walls a future attempt will meet, recorded here so nobody re-derives
them:

1. **The read verb's literal cannot go in `cmd/` or `internal/`.** It is one
   of the four `arCredTokens` of ADR 0019 D1's one-acquirer register
   (`internal/treepins/absencerules_qa_test.go`): a mention outside
   `internal/posse/credential.go` reds the pin until a register row rules it.
   A tag keyed on the read verb is such a mention.
2. **The legacy twin's item name cannot go in tracked markdown at all.** The
   ops-residue pin's `K-RED (item)` shape
   (`internal/posse/opsresidue_qa_test.go`) reds a keychain-read line
   carrying an `-s` item that is not `KeychainService`, and the twin's name
   is not — so documenting the pair by its argv needs a security ruling on
   the shape, not a doc edit. This is also why the pages above say "its
   legacy-named twin" rather than quoting it, the way ADR 0042 says "the find
   verb". The same wall is live at commit time: the `prepare-commit-msg`
   gate's check 2 runs `OpsPatterns` over the added lines of staged
   markdown, so the read verb's literal cannot be committed into a `.md`
   either, and the override is the operator's.

Neither is a reason the tag is wrong — D4 is. They are the price of
reopening it.
