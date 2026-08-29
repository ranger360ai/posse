# Probe fixtures

A **probe fixture** is a file a session is told to commit so that something
about the harness can be measured — that a dispatched session works in its own
worktree, that its commit lands, at which toplevel, on which branch. The commit
*is* the measurement, so the file has to be real and it has to be tracked.

**Write it here, as `docs/probes/<bead-id>.md`, and leave it.**

That is the whole rule. The rest of this file is why.

## Why the cleanup is the defect, not the fixture

`scripts/audit-silent-reverts.sh` (rangerhq-8rtf, deletion rule rangerhq-ypn1)
flags any commit that puts a path back to a state it held before its
immediately preceding change — and **absence is a state**. Adding a file is
never flagged. *Removing* one that was added inside the scanned range is,
because that is exactly the shape of the incident the audit exists for: a fix
that lands one new file from a private index, followed by a commit off the
stale shared index that writes a tree without it. The detector cannot tell that
deletion from a tidy-up, and it must not learn to — see the third rejected
heuristic in the script's header, measured under ranger-base-hvbj.

So a fixture that is committed and then removed reddens `make test` on main and
blocks the release workflow until somebody writes a triage line explaining a
probe they did not run. Two landed on 2026-08-29 (a7b80a4 and 71fa30f, under
fixture beads ranger-base-a4lz and ranger-base-3j3t) and that is what this file
is the answer to. A fixture that is committed and *left* costs nothing: three
lines of history, no flag, no triage, and the measurement is still on the bead.

The plausible-sounding advice — "future fixture beads should carry their own
cleanup step" — is the exact wrong move. The cleanup step is what got flagged.

## For the author of a fixture bead

- Name the path in the bead: `docs/probes/<the fixture bead's id>.md`, not a
  file at the repo root.
- Do not ask for a removal commit, and do not add one afterwards.
- If the fixture's *values* are the point, they belong in a bead comment, which
  is where they survive anyway.
- If these ever genuinely have to go, sweep them in **one** commit and write
  **one** triage line for it. Do not restore a deleted one to undo a flag — the
  re-add puts the path back to a state it already held and lands a second
  flagged commit.

## What a fixture file holds

Whatever the probe measures, plus enough to read it a month later without the
bead: the bead id, the date, and the values. Keep it to a few lines. Nothing
here is load-bearing, nothing imports it, and nothing should ever depend on it.
