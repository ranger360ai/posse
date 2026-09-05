## The L1 commit wall read an option's VALUE as an option (ranger-base-v3cu)

`ranger-base-l1at` taught `posse_spoiled_commit()` every abbreviation git
resolves to a long spoiler. laurie found the same scan wrong one argument to
the right, verifying `ranger-base-qstx`: it walks **every token** and
pattern-matches each one, so a `-m` message that starts with `-` and
contains an `i` matched the `-*i*` cluster arm before the scan reached `--`.

    git commit -m '-i am a message' -- a.txt
    refused by posse gate (deny: Bash(git commit unless --)), exit 1

That commit is path-limited, carries no `-i`, and **does not sweep**
(MEASURED, git 2.50.1 / Apple Git-155: it lands `b.txt` alone and leaves the
other persona's staged `a.txt` staged). So this is the wall refusing the one
form it permits — it fails CLOSED, which is why it is a P3 and not an
incident, and it is still a wall nobody can work behind.

### The fix is the pairing renderFlagIn already does, one level over

`spoiler` gains `ValueOpts`: the subcommand's options that take their value
as a **separate word**. `renderSpoiled` consumes those in pairs, exactly as
`renderFlagIn` consumes `verbValueOpts`.

**Membership is measured per option, not reasoned about** — `git commit
--dry-run <opt>` answers `requires a value` for exactly the options whose
argument git makes REQUIRED, and only those consume the next word:

    -c -C -F -m -t
    --author --cleanup --date --file --fixup --message
    --pathspec-from-file --reedit-message --reuse-message
    --squash --template --trailer

`-S/--gpg-sign` and `-u/--untracked-files` are **deliberately out**: their
argument is OPTIONAL, so git takes the rest of their own token and never the
next word. Pairing one of them is a HOLE — `git commit -u -i -- f` would
shift past a real `--include` — and that is the asymmetry the table lives
under. A missing entry costs a false positive, one respelling away; a wrong
entry costs the wall.

The long ones carry a `LongMin` for the same reason spoilers do: `--m` *is*
`--message` and takes the next word just the same. Measured on the same git,
one prefix at a time: `--au --c --da --fil --fix --m --pathspec-fr --ree
--reu --sq --te --tr`.

### Arm order is the whole design

    --)                      first — past it every word is a path
    <spoilers, with ladders> a spelling that is BOTH is refused, not skipped
    <value opts, paired>     BEFORE --*, or a long option's value is scanned
    --*)                     so --signoff is not read as a cluster
    -m*|-c*|-C*|-F*|-t*)     GLUED value: git takes the rest of the token
    -*i*|-*p*)               the cluster arms, last

Each of those lines has a row that kills it — the mutations below are what
that claim rests on.

### What is still refused, and why that is the right side

| form | git means | shim |
|---|---|---|
| `-m '-i am a message'` | message | **through** (was refused) |
| `-mi`, `-m'fix typo'` | message, glued | **through** (was refused) |
| `--message -i msg`, `--m -i msg` | message | **through** (was refused) |
| `-m x -i`, `--trailer x -i` | a real `--include` | refused |
| `-u -i`, `-S -i` | a real `--include` | refused |
| `-m -- -i -- f` | `--` is the message, `-i` IS include | refused — correct |
| `-qmi` | message `i` behind a boolean | refused — residual |

The residual is one token shape: a value option that is not FIRST in its
cluster. A `case` glob cannot say *no earlier letter in this token also took
a value*, and enumerating the boolean prefixes would be a hand-rolled getopt
in `sh`. It fails closed with the safe form one space away, which is the
posture the whole table already runs at.

### Pins

- `TestQACommitWallL1OptionValueIsNotAnOption` — half one is the premise
  against the real git (the argv really is safe and really does not sweep),
  half two is the wall, with a control arm of forms that must STILL refuse.
- `TestQAValueOptsAreGitsRequiredValueOptions` — asks git, per option, which
  ones require a value, and names the two directions of error separately.
- `TestQASpoilerLongMinIsGitsBoundary` now walks `ValueOpts` beside `Opts`.
- `TestQACommitWallL1IncludeForm`'s allow table carries the bead's own row.

**Mutation-checked, four ways** (`go test -overlay`, so the tree is never
edited): `ValueOpts: nil` reds every new allow row and the census; adding
`-u`/`--gpg-sign` reds the control arm *and* the census with the HOLE
message; moving the pair arm after `--*)` reds the four long-option rows;
dropping the glued arm reds `-mi` and `-m'fix typo'`.

This supersedes the "one false positive accepted" line in
`docs/notes.d/rangerhq-ojnw.md` — `-mi` goes through now.
