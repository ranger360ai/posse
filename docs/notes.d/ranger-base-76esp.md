# The second delegated `posse upgrade`: 16m57s, and the queue is the only variable (ranger-base-76esp)

The first delegated install (ranger-base-jnf2z) read 19m08s with 146s of
suite-slot queue in it, and left one question its own fragment names: how much
of that wall is the verb and how much is the box. This is the second install,
run 12 minutes after the first finished, and it answers that — not by running on
a quiet box, which is still unmeasured, but by catching a different queue.

## The reading

MEASURED 2026-11-20, 23:40:15 to 23:57:12 EST (America/New_York), this box
(16 cores), by gilfoyle at a claude/standard seat realized at shims, from the
worktree `~/.posse/worktrees/posse/gilfoyle-posse-ranger-base-76esp`. The verb
was backgrounded and waited on by pid, per ADR 0092 D3 and AGENTS.md.

```
posse upgrade                      # no args: the default origin/main target
  3c0e3c5 -> 0e1cb6c (2 commits) · source ~/src/posse · target ~/.local/bin/posse
exit 0 · wall 1017s (16m57s)
loadavg at start 10.98 11.54 10.73 · at end 5.22 7.44 9.49
```

Both suite slots were held at launch, as they were for jnf2z — but one freed
almost at once, so **arm 1 queued 10s against jnf2z's 146s**. Arms 2 and 3 took
slot 1 immediately, in both runs.

| phase | this run | jnf2z |
|---|---|---|
| suite-slot queue, arm 1 | **10s** | **146s** |
| arm 1 · internal/posse | 244.9s | 214.0s |
| arm 1 · internal/treepins | 156.9s | 154.5s |
| arm 1 · cmd/posse | 122.3s | 119.1s |
| arm 1 · cmd/coldexec | 1.8s | 2.0s |
| arm 1 · cmd/testparallel | 0.2s | 0.2s |
| arm 2 (`-tags posse_arm2`) | 62.6s | 81.5s |
| arm 3 (`-tags posse_arm3`) | 103.7s | 119.1s |
| silent-revert audit | 2800 commits, 0 untriaged | 2798, 0 |
| wall | 1017s | 1148s |

**Wall minus queue is 1007s here and 1002s there — a 0.5% spread over two runs
whose walls differ by 131s.** So on this box, under a loadavg near 11, the
verb's own work prices at ~1000s (16m40s) and the queue is what moves the
reading. The 131s of wall between the two installs is 136s of queue, give or
take five seconds of package noise that cancels across the arms (arm 1 ran 35s
slower, arms 2 and 3 ran 34s faster). That is one derivation from two
contended points, not a model: a quiet box would price the doors and the arms
lower as well, and nothing here has read one.

Nothing newly red, so no D4 trigger fired and there is no condition line to
quote:

```
  before: status 8 condition(s), 0 unreadable store(s) · gates 11 persona(s), 36 not-held row(s), 0 non-zero exit(s) · render-probe 0
  after:  status 8 condition(s), 0 unreadable store(s) · gates 11 persona(s), 36 not-held row(s), 0 non-zero exit(s) · render-probe 0
  ✓ nothing the binary renders is newly red — the install stands
```

Every count identical across the two binaries, and identical to jnf2z's pair as
well. The D6 line, verbatim:

```
upgraded 3c0e3c5..0e1cb6c (2 commits) · gate unchanged · rollback: install -m 0755 ~/.local/bin/posse.prev ~/.local/bin/posse
```

Verified by execution afterwards, independently of the verb's own counts:
`~/.local/bin/posse` is `0.5.3+0e1cb6c0` and `posse.prev` is `0.5.3+3c0e3c5e`,
so one level of rollback is in place (D3.4); `posse.new` is gone, which is
D3.5's `rename(2)` having happened, and the two files hold different inodes;
`posse gates gilfoyle` exits 0; `posse gates render-probe` exits 0 with its
mark. `go run ./cmd/checkorphans` reads clean.

## The gate word is readable before the run

D6's field has four words and the delegate learns which one it got from the
last line. Which word is coming can be read first, read-only, in two
`sha256sum`s — the arm writes only when the installed copy differs from the
target commit's:

```
for f in bd-argv-gate.sh bd-argv-gate.py; do
  git show <sha>:scripts/$f | sha256sum
  sha256sum ~/.config/posse/gate/$f
done
```

Both pairs matched `<sha>` = 0e1cb6c0 before this run (`a47bebed848ec3e1…`,
`24ef126210550fa2…`), and the verb reported `unchanged` with all three
behaviour arms green, which is the same claim from the other side. The reading
is worth taking because it says, before a ~17-minute wall, whether this install
is about to touch the box-wide bd argv fence at all. It does NOT predict
`refused`: that word can also come from behaviour arms going red under
byte-identical bytes (D6 as amended by ranger-base-41l0r), which no hash can
see.

## G11 again: both delegated installs so far finished one commit behind

While this suite ran, `origin/main` moved `0e1cb6c0 -> d3d83823`
(ranger-base-7x3bj landed and was pushed), so this install also ended with
`posse status` reading

```
launcher · 0.5.3+0e1cb6c0 is 1 commit(s) behind main in ~/src/posse
```

jnf2z's fragment called its own G11 row "the live case" for ADR 0092's census
paragraph. Two for two now, on an active box, at a ~17-minute wall: an install
that ends level is the exception here, not the rule, and G11's exclusion from
the D4 triggers is load-bearing on every install rather than on an unlucky one.
A delegate reading a fresh G11 row straight after a green install should read
it as the install's own wake, and check the range before treating it as a
finding.

## What this does not measure

- A quiet-box wall. Still unread: both installs so far launched into a loadavg
  near 11 with both slots held.
- Any D4 trigger firing, or the gate arm's `installed` / `refused` / `unknown`
  words. Two installs, two `unchanged`s, no rollback.
- Whether the install reaches running seats. It does not, and that is
  ranger-base-rrgnc's: the armed `dispatch --watch` loops keep the old inode
  and re-render the pre-install walls until they restart.
