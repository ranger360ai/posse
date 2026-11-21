## Four closes verified, and a second check of one rule leaves the inner one dead (ranger-base-w37g2)

2026-11-21, this box (linux 7.2.15-1-omarchy, go1.27.3, git 2.56.0). The
verify of ranger-base-6e5jm, ranger-base-jyqk4, ranger-base-6zw05 and
ranger-base-ydw7p. All four hold. What is worth keeping is three methods and
one finding.

### The declared root's re-check cannot fire (ranger-base-ydw7p)

`wallUngrantedDir`'s declared arm re-runs `ungrantedGrantRefusal` on the path
`os.MkdirTemp` answered with, and the comment above it says why: "a box that
symlinked the declared root into `/tmp` would hand every caller a granted
root and every refusal below would be measuring the other fixture."

That case is already shut one call earlier. `ungrantedFixtureRoot` applies
the same rule to `absResolve(v)` at declaration time, `underDir` resolves
both of its sides, and `MkdirTemp` returns a real child of its parent — so a
child of an unrefused root is never refused. MEASURED over seven hostile
spellings, each printing the predicate the DECLARATION applies beside the one
the re-check would apply to the child:

| the declared root | declaration | the re-check |
|---|---|---|
| a plain root outside every grant | not refused | silent |
| a symlink into the granted root | REFUSED | never reached |
| a dir reached through a symlinked parent | REFUSED | never reached |
| a two-hop symlink chain into the grant | REFUSED | never reached |
| `TMPDIR` set to the root itself | REFUSED | never reached |
| `TMPDIR` set to the root's parent | REFUSED | never reached |
| `TMPDIR` a symlink to the root's parent | not refused | silent |

No row reaches it, the symlink case the comment names least of all. The arm
is dead and the sentence is wrong about what keeps that promise; the
declaration arm keeps it. Recorded rather than filed (ADR 0006 §6): deleting
unreachable code changes nothing a process does and nothing a pin proves.

**The `/var/tmp` arm's identical re-check is NOT dead**, and the difference is
the whole lesson: that arm has no declaration-time check in front of it — the
const is never put through the rule before `MkdirTemp` — so there the
re-check is the only reader. MEASURED firing with `TMPDIR=/var/tmp`: "the
fixture root /var/tmp/posse-ungranted-… lies under /var/tmp". One rule, two
call sites, and only the one with nothing in front of it is live.

So the shape to watch: **when a surviving mutant deletes a check, ask whether
an OUTER call already applies the same rule to a path that resolves the same
way.** A mutation battery grades arms one at a time and cannot see that the
arm it just deleted was the second reader of one predicate.

### Reproducing a macos-only red from linux (ranger-base-jyqk4)

`wallMechanismHost` is a `var` for exactly this (wall.go's own comment), so a
three-line untracked `func init() { wallMechanismHost = "darwin" }` in
internal/posse asks the macos legs' question here. Run over two trees cut by
`git archive` — the fix and its parent — it is a before/after:

| tree | mechanism | arm 2 | arm 3 |
|---|---|---|---|
| 8e1c647d^ | darwin | 14 red lines (9 tests + subtests) | `TestSeatbeltProfileAndLaunch` red |
| 8e1c647d | darwin | 18 PASS, 0 FAIL | 1 PASS |
| 8e1c647d^ | linux | 18 PASS | 1 PASS |
| 8e1c647d | linux | 18 PASS | 1 PASS |

The pre-fix darwin rows carry the verdict sentence the bead quoted verbatim
("wall check not run — this host realizes cage seatbelt with sandbox-exec,
and ADR 0097 D1's venue is a bwrap"), and both linux columns are green on
both sides, which is what says the escape was the platform's and that the fix
moved nothing here.

The two families whose probes EXECUTE `sandbox-exec` cannot run on this box
at all. Their halves that are membership were measured instead, and both hold
as the close states them: the two paths the carve-out's `sbUngranted` rows
write at read (granted=false, denied=false, pre-kfrpv-granted=true) — which
is exactly the three answers that row asserts — and the instance-state pin's
three rows plus both pre-fix controls flip one claim each and only one. That
arithmetic is WITNESSED on linux in both directions:
`TestQACarveOutNamesTheConstitutionTheGatesAndTheHooks` reds both when
ranger-base-kfrpv's grant is put back where it stood and when the store
repo's hooks slot is denied again. The kernel verdict is still a macos leg's.

### A rendered shim is probeable without a seat (ranger-base-6zw05)

`App.RenderGates(persona, deny)` renders a real gates dir anywhere, so a
40-line `cmd/` program in a scratch tree plus a shell battery types each
option through the shim a seat would run, one fresh conflicted repo per row.
Across the fix and its parent, with another actor's file staged mid-conflict:

| tree | typed | refused | commit made | the other actor's staged file |
|---|---|---|---|---|
| 502c49d6 | `git cherry-pick --skip` | no | — | GONE from index and disk |
| 502c49d6 | `git revert --skip` | no | **YES** (3→4) | GONE from index and disk |
| 63ae11fd | `git cherry-pick --skip` | YES rc 1 | no | staged, on disk |
| 63ae11fd | `git revert --skip` | YES rc 1 | no | staged, on disk |
| 63ae11fd | `git rebase --sk` | YES rc 1 | no | staged, on disk |
| 63ae11fd | `--quit`, three verbs | no | no | staged, on disk |

The refusal names the option git SPELLS (`--skip` for a typed `--sk`) and
carries rule `commitwall:sequencer-continue`. A rebase row needs a real
in-progress rebase to say anything about git's half; mine did not leave one,
so the rebase rows above are the shim's verdict and not git's.
