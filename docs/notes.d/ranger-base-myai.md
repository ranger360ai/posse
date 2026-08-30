## The L1 commit wall's spoiler table was incomplete, not just literal (ranger-base-myai)

ranger-base-l1at taught the table that git resolves any unambiguous PREFIX
of a long option, and it emitted every abbreviation of `--include`. laurie
found the same defect one OPTION to the left while verifying that close:
l1at fixed the spelling set for the spoilers the table names, and never
asked whether the table names every option that spoils. It does not.

`--patch` and `--interactive` satisfy `unless --`, are absent from the
table, and land in the same `--*) ;;` arm l1at identified as "where the hole
is" — `--patch` because it has no `i` at all, `--interactive` because
`--*)` is matched before the `-*i*` cluster arm and swallows it.

**Worse than `--include`, in the way that matters.** `--include` at least
also commits the paths you named. Both interactive options commit ONLY the
shared index: a fleet Bash call has no TTY, the selector at EOF picks
nothing, and git commits the index as it stands — which is the other
persona's staged entry. Your own named path is not in the commit, your edit
is still unstaged, and the exit code is 0.

**Measured (git 2.50.1, macOS 25.4), another persona's `other.txt` staged,
mine `mine.txt` edited on disk, stdin at EOF, no L3 hook:**

| form | git | shim before | lands |
|---|---|---|---|
| `--p` `--pa` `--pat` | ambiguous with `--pathspec-from-file`, exit 129 | through | nothing |
| `--patc` `--patch` | resolves to `--patch` | **through** | `other.txt` — **sweeps** |
| `-p` | `--patch`'s short name | **through** | `other.txt` — **sweeps** |
| `-pm x` `-qp` `-sp` | `-p` inside a cluster | **through** | `other.txt` — **sweeps** |
| `--i` `--in` | ambiguous with `--include`, exit 129 | through | nothing |
| `--int` … `--interactive` | resolves to `--interactive` | **through** | `other.txt` — **sweeps** |
| `--no-patch` `--no-interactive` `--no-include` | the safe semantics | through | `mine.txt` only |
| `-a` `--all` with a pathspec | `paths … with -a does not make sense`, exit 128 | through | nothing |
| `--pathspec-from-file=f --  <path>` | `cannot be used together`, exit 128 | through | nothing |

The last two rows are the bead's open questions, closed: git walls both
itself, so neither wants a table entry. `--al` is ambiguous with
`--allow-empty`, and `-o`/`--only` is the safe semantics.

**LongMin is per option and stays measured.** `--patch` → `--patc` (`--pat`
is ambiguous with `--pathspec-from-file`, which is exactly the trap l1at
named when it rejected down-to-one-character). `--interactive` → `--int`,
which is one character SHORTER than the `--intera` the bead reported; the
boundary pin asks git rather than a person, so it is git's number that
landed in the table.

**The general defect is the shape of the table, and that is what the new pin
addresses.** The table is an allow-by-omission list over an option set GIT
owns and grows. `TestQASpoilerLongMinIsGitsBoundary` pins the boundary of
each entry that EXISTS; nothing pinned that the entry SET was complete,
which is how these two sat outside it. `TestQASpoilerTableCoversEveryCommitOption`
is the other half: it parses `git commit -h` for every option git names —
88 spellings on 2.50.1, `--[no-]x` counted as both `--x` and `--no-x` —
runs each one through the incident's own shape against the real git, and
requires the set that sweeps to be EXACTLY the set the table declares. A git
that grows a sweeping option fails there instead of in someone's history,
and an entry that stops sweeping fails there as noise. It is ~8s and worth
it; the parse is guarded by its own premise (a floor on the count and a
spot-check for ten known spellings) so a `git commit -h` it cannot read
fails loudly rather than passing vacuously.

Residual, named rather than measured: each option is tried BARE, which is
how a boolean is typed. A value-taking option eats the `--` as its value
instead — measured, none of them sweeps — so what that pin settles for those
is that spelling and not `--opt=<value>`.

`TestQACommitWallL1PatchDoesNotSweepRealIndex` is the bead's repro end to
end, the rendered shim in front of the real git with no L3 hook, over all
six sweeping spellings: refused, the other persona's staged entry still
staged, HEAD unmoved, and the safe form still landing exactly one path. Its
half one is the unguarded premise — `--patch` really does commit `a.txt` and
NOT the `b.txt` on the command line — for the same reason l1at's has one.

Every pin was run against the unfixed table and failed on the right rows:
dropping `-p`/`--patch` from `Opts` fails the coverage pin on both and fails
the repro pin; adding `--signoff` to `Opts` fails the coverage pin as noise;
setting `--interactive`'s LongMin to the bead's `--intera` fails the
boundary pin and lets `--int` through the wall into git's interactive
selector.

**The false-positive cost is the same class as `-i`'s and unchanged.** `-mp`
is the message "p" and is now refused, exactly as `-mi` is; the safe form is
one space away. **L3 was never exposed and still is not** — the hook
discriminates on `GIT_INDEX_FILE` / `.git/index.lock`, which git hands it
whatever the option is spelled, so it refused both throughout (laurie
measured both directions). The exposure was bounded to hookless checkouts,
which is the case L1 exists for.
