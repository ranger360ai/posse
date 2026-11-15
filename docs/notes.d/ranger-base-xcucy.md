# A seat's own object store is imported by re-hashing, and a landing refuses a tip with missing objects (ranger-base-xcucy)

*2026-11-15 · dinesh · design ranger-base-dvw0w D1, D3, D4*

Part of:

- ranger-base-21i98

## What changed

A session may keep its objects in a store of its own,
`<own worktree git dir>/seat-objects`. Today no session has one, so
everything here is a no-op. Once a session does, this is the only way its
objects reach the shared store. Each object is re-hashed on the way in, so
it lands under the name its bytes hash to and no other.

- `importSeatObjects(t)` / `importSeatStore(repo, store)` in
  `internal/posse/seatimport.go`. The store's loose objects and pack pairs
  are copied through an `os.Root` into a fresh bare view that the launcher
  makes with `mktemp`. A symlink in the store, or a store that is itself a
  symlink, is never followed. The store's `info/alternates` is never read.
  A loose object whose name the shared store already holds is never copied.
  What is left is packed in the view with `pack-objects` and indexed into
  the shared store with `index-pack --stdin --strict`.
- Callers:
  - `spliceDetachedWork`, before `branch -f`.
  - `MergeSessionWork`, first. An import that fails is a merge-back
    refusal, not an error.
  - `RemoveSessionTree`, before `worktree remove`. An import that fails
    keeps the tree, even under `--force`.
  - `pruneWorktrees`, which replaces both bare `git worktree prune` calls.
    It imports every store whose tree is gone, and does not prune if one of
    them fails.
  - The landing sweep, once per pass over every store in every repo
    (`landClosedTrees`).
- The landing gate, `landingGateReason`. After the import,
  `rev-list --objects --missing=print <branch> --not <base>` must print no
  `?` line, or the landing is refused and the refusal names the missing
  objects.
- `treeState` (the `posse worktrees` listing) used to print
  "nothing unlanded" when git would not count a tip. Now it says the count
  cannot be read.

## Measured

2026-11-15, git 2.56.0, linux 7.2.15, in a scratch repo:

- `pack-objects` carries a loose object whose bytes do not match its name,
  with no error. `index-pack` then files it under the hash of its bytes, and
  the claimed name stays missing.
- `merge --ff-only` refuses a tip whose own tree names a missing blob,
  because the checkout reads that tree. It fast-forwards without complaint
  over a missing blob in an intermediate commit that the tip deletes. The
  gate exists for that case. Pin 3b builds it, and with the gate removed the
  landing goes through.

## Pins

`internal/posse/seatimport_qa_test.go`, arm 1. Each pin was run against a
mutant of the code it holds, and each mutant reds it:

1. forged bytes under a predicted name
2. forged bytes under an existing name
3. the splice lands after the import, and the gate refuses an
   intermediate missing blob
4. the census readers, asked about a tip that was not imported
5. a forced retire whose import fails keeps the tree
6. prune of an orphaned store
7. the watch-pass import clears an operator `gc` over a staged blob
8. no symlink is followed out of the store
