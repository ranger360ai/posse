## Four closes verified, and the arm a census cannot see (ranger-base-t0ha1)

2026-11-22, this box (linux 7.2.15-1-omarchy x86_64, uid 1001, go1.27.3, git
2.56.0, 16 cores). The verify of ranger-base-2hmfq, ranger-base-s9h52,
ranger-base-aurb8 and ranger-base-1b8yv. Verified at 2866fdf4, which is
main's tip and the first tree to carry all four, so every reading below is of
the combination and not of any closer's own base. Every mutant ran in a
`git archive`d scratch tree, one at a time, with the live worktree left alone
because a full suite was running against it.

All four hold. Two findings are live and both are the same shape: an arm
nothing witnesses, inside a probe whose other arms are well held.

### The finding: a census key that reads only one spelling of a launch

`internal/treepins/wallmemberaccount_qa_test.go`'s arm 1 claims to read
"every function in a member file that launches a bwrap seat". It reads the
functions that hand `exec.Command`/`CommandContext` a LITERAL `"bwrap"`
argument (`wmaLaunchesSeat`). A seat launched through a shell or a pty
wrapper — the literal inside a command STRING rather than in the argv — is
invisible to it, and the file's prose states no such limit.

MEASURED by a planted member file and its control, the same file twice, the
only difference being how the seat's argv is spelled:

| planted `TestLiveWallZZPlantedProbe`, waiter error dropped to `_` | census |
|---|---|
| `exec.Command("bwrap", "--dev-bind", "/", "/", "/bin/true")` | **RED**, named at `liveplantprobe_zz_test.go:11` |
| `exec.Command("sh", "-c", "bwrap --dev-bind / / /bin/true")`  | **GREEN** |

The control is what makes the green a reading and not a guess: arm 1 does see
a planted launcher, and the one it cannot see differs in nothing else.

There is already one such function in the member set.
`internal/posse/grokleader_qa_test.go:246` (member
`TestLiveWallGrokLeaderUseLeaderTUIStartsNoLeader`) builds `q = ["bwrap",
…render argv…]`, joins it into a command line, runs it under `script -qfec`
and drops the waiter error with `_ = cmd.Wait()`. In THAT function the drop is
defensible — the arm kills the seat on purpose, so the status carries nothing
— but the function is outside the rule, not excused by it, and the next member
that takes the same route escapes silently.

The `launchers >= 25` floor cannot catch this. It found 30, and an invisible
launcher does not lower the count of visible ones; the floor answers "has the
waiter set gone stale", never "is a launch spelled some other way".

### The finding: a two-key drop function with one key witnessed

`bwrapDropTmpGrantsOver` (internal/posse/wallseatcache.go:223) builds its drop
set from two keys, and ranger-base-aurb8's whole subject is that such a key
must be resolved. Both are:

```go
drop := map[string]bool{absResolve(mount): true}
if tmpdir != "" {
        drop[absResolve(tmpdir)] = true
}
```

Graded un-combined, one mutant each:

| mutant | `…ReadsTheMountsResolvedSpelling` | the two pre-existing drop pins |
|---|---|---|
| `map[string]bool{mount: true}` (the raw mount back) | **RED** | GREEN |
| `drop[tmpdir] = true` (the raw TMPDIR) | GREEN | GREEN |

Row 1 is ranger-base-aurb8's own claim, reproduced: the new pin reds and the
two neighbours do not, which is the whole of why only a macos leg could read
the bug. Row 2 is the hole. The `absResolve(tmpdir)` predates the bead — the
landing changed the mount key and nothing else — but the bead is the one that
wrote this function's pin, and the pin's fixture plants a symlinked MOUNT and
hands it a tmpdir that needs no resolution, so it physically cannot produce
the shape that would grade the second key.

What the missing witness is worth: on darwin a launcher TMPDIR of
`/var/folders/…/T` resolves to `/private/var/folders/…/T`, and
`SeatbeltWritable`'s `add` puts the resolved spelling in the set. With that key
raw, the drop misses, the box's shared temp grant survives the render that just
bound a private `/tmp` over it, and the seat keeps a write grant on a directory
every seat on the box shares — which is the condition
ranger-base-1c91x created this drop to remove. The line is correct today; one
`absResolve` holds it and no run would notice its loss.

### What a census of the class refuted, which is worth as much

ranger-base-aurb8's defect is a map keyed on a producer's RAW spelling and
probed with an `absResolve`d one. Censused over every equality comparison and
map lookup involving `absResolve` in non-test `internal/` and `cmd/` code, four
candidates came up and all four are clean:

- `bwrapBindExisting`'s `no` map (bwrap.go:3400) is keyed from
  `bwrapNoCreate`, whose both arms are `absResolve`d — `pidWritableResolve`
  returns `absResolve(w)` and the `.beads` entry is resolved at the join.
- `bwrapClaudeStateDirs`' `drop`, `seatbeltObjectsSwap`'s `l.Objects`
  (resolved at seatstore.go:97) and `seatimport`'s `want`: resolved on both
  sides.
- The three production readers that take `bwrapTmpMount` as a CONTAINMENT
  parent (bwrap.go:1810, landedcheck.go:507 and :511, wallseatcache.go:173)
  go through `underDir`, which `absResolve`s both sides at seatbelt.go:2934.
  A raw `/tmp` is safe THERE, which is why the fix had to be local to the map.

So the respelling is one function wide, as the close says.

### ranger-base-1b8yv's darwin condition, reproduced rather than inherited

The nine store-repo carve-out rows were re-run with `TMPDIR` pointed at a
symlink whose target is `…/private/var/t` — darwin's shape, the route the
closer used to reproduce the red. Green under the linked TMPDIR (2.095s) and
green under an ordinary one (0.757s), at 2866fdf4. Three mutants on the
producers the close names, each killed and naming its own arm:
`SeatbeltWritable`'s `add` not resolving reds the set, profile and L3 arms;
`cageOverlay`'s `src := p` reds the mount-spelling arm; `beadsHome` answering
`absResolve(…)` reds all three pins at the fixture premise, which is the
contract decision's own witness.

### ranger-base-2hmfq, graded past the two mutants the close ran

Five mutants, one at a time, all killed, every restore sha256-identical and
the file re-read green afterwards (`TestQAContinueWall`, 10.9s baseline):

| mutant | red arm |
|---|---|
| merge's Way reinstates `git merge --abort` | pin 6 / merge / `--abort` |
| rebase's Way reinstates `git rebase --abort` | pin 6 / rebase / `--abort` |
| cherry-pick's Way reinstates its `--abort` | pin 6 / cherry-pick / `--abort` |
| merge's Way prescribes the walled `--continue` | `…RefusesTheFourVerbs…/merge/way-mid-conflict` |
| `--abort` dropped from merge's `Stood` set | `…OptionSetIsGitsInProgressClass/merge` |

The last two are the ones worth having. A Way that prescribes an option the
wall itself refuses is a separate failure from one that prescribes a
destroying option, and they are read by different arms; and deleting the
`Stood` row is the cheap way to silence pin 6, so the option-set derivation
has to red for it — it does.

### ranger-base-2hmfq's DONE WHEN, RUN off the production renderer

Not off the pin's fixture: a scratch `RHQ_HOME` with a hand-written PID
carrying `deny: Bash(git commit unless --)`, rendered by a `posse gates` built
from this tree, then that `bin/git` run in a fresh non-linked repo left
mid-conflict with another actor's `other.txt` staged. All four refuse (rc=1,
`commitwall:sequencer-continue`), and every second line names `git <verb>
--quit` plus the path-limited `git restore` — revert's names the two-step
path-limited commit, as its row always did. None prescribes `--abort`.

Two things made this rig honest, and both bit first:

- **This seat's own `git` is the L1 shim**, so the fixture's base commits were
  refused by the commit wall, no conflict was ever created, and the first run
  reported four tidy "the second line does not prescribe `--abort`" lines over
  four repos with one commit between them. The fixture does its git with
  `/usr/bin/git` at an absolute path and path-limits every commit.
- **A PID with no `---` frontmatter parses no `deny:`**, so
  `renderContinueWall`'s `rulesDenyUnqualifiedCommit` was false and the shim
  came out 443 lines with no wall in it. That run did not refuse at all — it
  swept `other.txt` into the commit for all four verbs. With the fences the
  shim is 573 lines and carries the arm.

So the row that matters is not "no `--abort` in the second line": it is
"refused by the continue wall AND no `--abort` in the second line", with the
in-progress marker and the staged victim asserted before the call. Absent
those three, a green reading here means the rig never reached the window.

### The live shim on this box still names `--abort`, and that is promotion lag

`$RHQ_HOME/state/gates/laurie/bin/git`, rendered at this session's launch,
carries the pre-fix Way strings for all four verbs. The installed
`~/.local/bin/posse` is `0.5.3+00601dfe` (2026-11-21 01:34) and
`git merge-base --is-ancestor` says none of the four landings is in it. So the
respelling is landed and not yet live, which is ordinary and is what the
build-stamp ancestry check is for — but a seat refused by the wall between
that build and the next promotion is still sent to `--abort`.

### What this verification did not do

No darwin host here, so neither macos DONE WHEN was answered: ranger-base-aurb8's
13 rows and ranger-base-1b8yv's nine are green on linux under both spellings of
their conditions, and the first macos-latest arm 1 leg after this lands is what
settles them. Both closers said so.

The ADR 0097 wall venue probe was not re-run. ranger-base-s9h52's subject here
is a STATIC census, doored by `make treepins-check` — verified by reading
`$(QA_TREEPINS_EXCLUDED)` in the Makefile, which names none of the census's four
tests, so the complement door runs them (ADR 0098 D1/D4) — and
`make treepins-check` is a `make test` prerequisite through `tree-check`.
