# Verifying ADR 0081 D2/D1/D3/D4 and INSTALL's store step (ranger-base-hgq2k)

Four closes verified 2026-11-07/08 on `main` at `fcb61cb6`, all four commits
ancestors of HEAD: ranger-base-b87ej (`dadce4ae`, D2's stamp),
ranger-base-lh9nj (`f391baa7`, D1/D3's write rule), ranger-base-hnujh
(`ab43fa07`, D4's refusal) and ranger-base-3opof (`c9b10dc6`, INSTALL §9's
store step). Three of the four each state a DONE WHEN and each one's DONE WHEN
holds; two carry findings that got past them.

## What was run, not read

- The 13 new Go pins green on this tree (internal/posse 18.4s, cmd/posse
  13.6s), plus all 29 arms of `internal/treepins/hookfreshness_qa_test.go`
  (110.4s) — which is the SHELL reader, end to end, and so is the independent
  check on D2's "read from another home" half.
- `make tree-check` green, 80.0s at a one-minute load average of 8-12.
  `make fmt-check` green. `go run ./cmd/checkorphans` clean.
- Seven mutants of my own, each aimed from `grep -n` at HEAD and md5-checked
  so a no-op edit could not score as a kill:

  | mutant | killed by |
  |---|---|
  | D2's refinement removed (`commitGuardHomeVerdict` never answers `l3OtherHome`) | 6 arms, and the unstamped arm stayed GREEN — which is what says it pins D3 case 4 and not D2 |
  | D2's `samePath` conjunct → `stamp == a.Home` | **NOTHING** (see finding 2) |
  | D3's write rule removed (`sharedCommitWallWrite` always writes) | 4 arms |
  | D1 dropped from D3's rule | the OWNER's arm, alone — D6 |
  | D4's refusal removed | both refusal arms, one on exit code and one on bytes |
  | the shell reader's home classification switched off | the foreign-stamp arm, alone |
  | the shell reader's `same_dir` → a string compare | the cry-wolf arm, alone |

- D4 measured by hand against the built binary, two scratch homes and a
  scratch repo (never a real repo or a real home): the owning home installs a
  1059-line body stamped with its own path; the non-owning home's
  `install-hooks <that repo>` exits 1, names the stamping home, both remedies
  and `no --force`, and leaves the slot identical in bytes and in mtime to the
  nanosecond (`1794112305.157611767`, both sides), with pre-push installed —
  D5. The same from inside a LINKED WORKTREE of that repo, with no argument,
  names the main checkout and leaves the shared slot alone. 1059 is the same
  number ranger-base-hnujh's own fragment records, measured independently.

## Finding 1 (LIVE) — the refusal names a relative target as the repo

`cmd/posse/main.go:1793,1819,1821`. `ForeignCommitWallOwner` returns
`hookRepo(dir)`, and `hookRepo` hands back `dir` UNCHANGED for a main checkout
(`LinkedGitDirs(dir) == nil`). `install-hooks` never absolutizes its target, so
with the `.` default — which is the form every degraded line, every `L3 … is
STALE` finding and most of INSTALL.md prescribe — the refusal reads

    not installed: prepare-commit-msg — .'s commit wall was rendered by home …
      remedy: add `.: <private|public>` under this home's beads_visibility: …

MEASURED: following that line literally does not confer ownership of that repo
— the refusal repeats from any other cwd — and a `.` key is read by `samePath`
→ `resolvedPath` → `filepath.Abs`, i.e. against the READING process's cwd, so
it marks whatever repo posse is run from. A repo named nowhere in that config
read `beads visibility guard: private — config beads_visibility:` from inside
itself. So the prescribed remedy is both inert where it was printed and a
cwd-dependent wildcard over the visibility verdict the same wall enforces.

The close reasoned about exactly this hazard one case over —
ranger-base-hnujh's fragment: "a refusal that named the session worktree it
was typed in would name a directory whose `beads_visibility:` line nobody can
write" — and the worktree case is right. The relative spelling is the same
sentence with the same consequence, and `gates managed-hooks` already carries
the fix shape at `cmd/posse/main.go:1739` with that complaint written beside
it.

The pin for this path, `TestQAInstallHooksRefusesAForeignWallInTheCurrent
DirectoryToo` (cmd/posse/commitwallowner_qa_test.go:206), asserts the stamping
home, the exit code and the bytes — its explicit-argument sibling at :155
asserts the repo too, and this one does not.

## Finding 2 (LIVE, pin added here) — D2's `samePath` conjunct was unpinned

`internal/posse/gates.go:6992`. Replacing `samePath(stamp, a.Home)` with
`stamp == a.Home` left GREEN every one of the 28 L3 arms of internal/posse
(the new pins, the probe, the sweep, parity) and all 10 of cmd/posse's
install-hooks arms. The cry-wolf arm that does exist covers the SHELL reader's
`same_dir` only (`internal/treepins/hookfreshness_qa_test.go:452`; a
string-compare `same_dir` reds that arm and nothing else, measured).

It is worth more on the Go side than on the shell side, because on the Go side
the verdict decides the WRITE: `sharedCommitWallWrite` reads `l3OtherHome`, so
one home spelled two ways — `/tmp` against `/private/tmp` is the pair that
bites first, and this box reaches parts of its own config through symlinks —
would stop a home refreshing its OWN wall, take the per-session redirect
instead, and print a question about ownership that is already settled.

Added here, since a test addition is QA's:
`TestQAOneHomeSpelledTwoWaysIsOneHomeToTheCommitWallReader`
(internal/posse/commitguardhomestamp_qa_test.go). It installs from a symlinked
spelling of one home, reads back through the resolved one, and asserts the
verdict, the empty `CommitGuardHome`, both wordings, AND the write the verdict
decides. Mutation-checked both ways: `stamp == a.Home` reds it, and dropping
the conjunct altogether reds it.

## What ranger-base-3opof rests on, and what it does not

INSTALL §9's store step is verified by its own sources rather than by re-running
`bd init`, which every persona's PID denies (`Bash(bd init:*)`) — the same limit
the closer had, and the reason the behaviour itself is the operator's MEASURED
on the Linux host. What can be checked here was:

- `bd info`'s label is `Database:` verbatim, over the live SQLite store of
  record — run.
- The fenced fallback text is bd's own. The pinned binary (`bd version 0.50.3
  (bd25acbc)`) carries the format string `Note: %s backend requires CGO (not
  available in this build).` and `Falling back to JSONL-only mode.` as separate
  literals, so the quote renders exactly as printed. Censused with `grep -ac`
  over the binary, never `strings`, and with a planted negative control that
  answered 0. The other CGO sentence in that binary — "(not available ON this
  build); use sqlite backend or install from pre-built binaries" — is a
  different message and is not what INSTALL quotes.
- `34-36 of 48` is ADR 0073's create-loss row ("of which the record is at the
  printed id | 12-14/48 (6 runs: 36, 35, 34, 34, 36, 34 lost)"), not its
  duplicate-id row, which coincidentally reads 34-36 as well. The `close`
  reading back `in_progress` in 1 of 3 is that table verbatim.
- The mixed-prefix sentence is ADR 0055's Consequences verbatim, and
  `liveBeadsRepoOfClass` exists where it is cited.

The bead's second half — a store-class finding in `posse status` — was diverged
rather than built, handed to architecture as ranger-base-p1l2h with the edge
confirmed, and RULED on there (refused, with reopen conditions written into ADR
0055 D3). The divergence is disposed of, not parked.
