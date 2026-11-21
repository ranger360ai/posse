# ranger-base-yusee — four closes verified, and the two things a green suite still licenses

Verify of four closes: ranger-base-7c2py, ranger-base-rh8y0,
ranger-base-pdcf6, ranger-base-x78s5. All four VERIFIED by execution. Two
LIVE findings beside them (bundle named on the close), and the readings worth
keeping are here, because three of them are measurements nobody else was
placed to take: the arm-6 flake reproduced on today's tree, the whole tree
censused for the one input a publication-control guard turns on, and the
ubuntu CI leg two of these closes were waiting on.

MEASURED 2026-11-21, this worktree, linux 7.2.15-1-omarchy, 16 cores, git
2.56.0. Load is named per reading; it is the one variable that tracks.

## The landed shas are not the shas the closes name

Every one of the four closes names a commit that is NOT an ancestor of HEAD.
The launcher rebases, so the subject survives and the sha does not — check
content, never the sha a close quotes:

| close | sha its comment names | the ancestor that carries it |
|---|---|---|
| ranger-base-7c2py | 490055bf | 6b816793 |
| ranger-base-pdcf6 | 4684de7d | eb3f34fc |
| ranger-base-x78s5 | 0e6fdba5 | bac28fe0 |
| ranger-base-rh8y0 | bfe9b1b6 | **ae4d5e07**, whose subject names ranger-base-zipwe |

The last row is the one that would mislead a reader. ranger-base-rh8y0's own
commit was PARKED — 91a73459 is on `parked/rh8y0-91a73459` and reaches no
branch — and its payload (the new pin, `ldiGateOut`, the notes fragment)
landed under ranger-base-zipwe's commit instead. So
`git log --grep ranger-base-rh8y0 HEAD` finds exactly one commit and that
commit's subject is another bead's. The work is all on main and all verified;
a verifier who graded "did it land" by sha would have said no.

## scripts/gotest.sh arm 6: the pre-fix flake, reproduced on today's tree

ranger-base-7c2py's finding was that arm 6 killed its mutant 2 of 5 runs. The
fix reorders the arm — the plant goes to the BACKGROUND first, the holder's
marker comes off only once the plant says it is waiting. To grade "the order
is the arm" I reconstructed the PRE-fix order over a copy (marker off, then
`plant_over` in the foreground) and ran all three shapes, 7 runs each:

| shape | arm 6 | PLANT_TRIES |
|---|---|---|
| pre-fix order, correct `plant_over` | 7/7 green | 1, 1, 9, 9, 1, 9, 9 |
| pre-fix order + one-attempt mutant | **2/7 survived** | 1 every run |
| fixed order (tracked) + one-attempt mutant | 7/7 KILLED | 1 every run |
| fixed order (tracked), control | 5/5 green | **9 every run** |

The coin flip is the first row and it is the whole bug: three of seven runs
never refused the first write, so the retry the arm exists to keep alive was
not exercised, and the arm said `ok` anyway. The control's try count stopped
moving, which is the fix in one number. Loadavg 13-27 across the batteries;
the kill at 21 and the control at 27, so this is not a quiet-box reading.

TWO THINGS NOT TO RE-DERIVE. A copy of this script must be `chmod +x`: it
re-execs `$0`, so a mode-0600 copy answers rc 126 in every arm and a rig that
reads rc as the verdict will call every mutant dead. And the `ptries >= 2`
assertion is NOT what kills the one-attempt mutant — remove it and the mutant
still dies 5/5, through the `prc != 0` branch that predates the fix. The
assertion's job is the other one: it reds a run that measured nothing, and the
first row above is the evidence that such runs are real.

## check 4: the whole tree censused for one input

ranger-base-pdcf6 replaced a prefix test with a POSITIONAL one — a header is a
`diff --git ` line, plus a `+++ ` line immediately after one. The reader
(`vulnPrivateUndiff`) is right and its three other halves are
mutation-killed. The conditional half is not, and the census says why.

The guard and a positional-alone reading (`case atFile:`) can only disagree on
one input: a line after a `diff --git ` that is NEITHER a `+++ ` header nor the
next file's own `diff --git `. Over the content arm's real filtered stream:

* `root..HEAD` as one staged diff — **2,176 file headers, 2,176 followed by a
  `+++ ` line, 0 exceptions**;
* a fixture carrying both real no-`+++` shapes, a binary file and an empty new
  file — each such header followed by the NEXT file's `diff --git `, which the
  switch's FIRST case already catches.

So on everything git emits the two readings are identical, which is why the
mutant survives internal/posse arms 1, 2 and 3 and the treepins commit-gate
pins. The test arm added for it (`vulnprivate_test.go:1040`, "a file whose
header has no `+++` line does not eat the next file") passes under the mutant
for the same reason.

The inference is where it went wrong, and it is written down twice —
`vulnprivate.go:311-312` and the arm's own comment at
`vulnprivate_test.go:1036-1039`, both saying a file with no `+++` line is why
the blanking must be conditional. The PREMISE is right: the next thing in that
stream really is another `diff --git `. The conclusion does not follow,
because the switch's FIRST case blanks a `diff --git ` line whatever `atFile`
says, so positional alone handles that file identically. ranger-base-pdcf6's
close comment puts it most plainly — "Positional alone would blank the next
file's first line there" — and that line is the next file's own header, not
its first content line.

The guard IS load-bearing against the READER's input, and this is the fixture
that proves it, because `RunGatesVulnPrivateScan` takes arbitrary text while
git does not:

```
diff --git a/docs/x.md b/docs/x.md
+- <an open vuln-private id>
+<TAB>the heap overflow
```

real code HIT · positional-alone CLEAN · and the control (the same two added
lines with no header above them) HIT on both, so the mutant is not simply
broken. One table row kills it.

WHAT ELSE WAS ATTACKED AND HELD. Content cannot forge a boundary: an added
line whose text is `diff --git a/x b/x` arrives as `+diff --git …` and is
read as content (HIT), and so are `++`, `+++ b/x` and a leading-space
variant — five shapes, all HIT. `diffReaderShape` pins `--src-prefix=a/
--dst-prefix=b/`, so the header spelling is immune to a reader's own
`diff.mnemonicPrefix`; run git by hand without those flags and the headers
come back `c/`…`i/`, which is a trap for the next person who reproduces this
from a shell.

### the installed control is two generations behind the fix

Measured, not inferred: `/home/posse/src/posse/.git/hooks/prepare-commit-msg`
greps `'^+' | grep -av '^+++'` and mentions `diff --git` zero times — the
PRE-ranger-base-4og5l reading. The installed posse is 0.5.3+00601dfe, 47
commits behind HEAD and older than 4og5l's own landing, so neither that fix
nor ranger-base-pdcf6's is exercised by the control that actually runs on this
box. ranger-base-pdcf6's close says hooks were deliberately not re-rendered
and why; this is how stale that leaves them. It is not a second bead: the
upgrade is already blocked under ranger-base-rfsy4.

## the ubuntu donor: the CI leg two closes were waiting on has run

ranger-base-x78s5 finding 1 was a P1 — `exec -a` had not fixed ubuntu, and the
bead's DONE WHEN ("one further CI run's postmortem line is read") was unmet.
ranger-base-x0ses was filed to read it. It has run, twice, and it is
affirmative:

| sha | ubuntu-latest arm 1 |
|---|---|
| 781adf22 (before the donor fix) | `donor /usr/bin/tee, run with argv[0] tee` · `coreutils: unknown program 'claude'` · `reap-test-tmp.sh --self-test: 2 arm(s) FAILED` · `make: *** [Makefile:599: verify-reap-test-tmp] Error 1` |
| eb3f34fc (after) | `reap-test-tmp.sh --self-test: all arms pass`, and `a live 'claude -p' run (pid 11092) is excluded from claude_procs` |

That second line is the one worth having: the planted fixture was visible to
the production counter on the ubuntu runner, which is the fact the bash donor
exists to restore. Runs 44923103475 (eb3f34fc) and 44926213762 (HEAD).

The hypothesis the fix rests on stays ASSUMED for that runner: the new
postmortem prints the donor's first two bytes only on FAILURE, and there is no
failure now, so nothing has yet read ubuntu's `/usr/bin/tee` magic bytes.

### and the prerequisite was masking four reds

With `verify-reap-test-tmp` red, `make test-arm1` compiled no Go test on
ubuntu at all — so every arm-1 test was unread there for as long as the donor
was broken, and gilfoyle's reading of the same job puts a date on it: the
`ok github.com/ranger360ai/posse/internal/posse 6.179s` at 16:11:49 is the
first Go test an ubuntu arm-1 job had compiled since 2026-11-11.

Fixing the prerequisite is therefore what made four failures visible, and all
four were filed within the hour, none of them by me: ranger-base-f177g (the
two `watchimage_qa_test.go` guards), ranger-base-z56s9 and
ranger-base-ebv8j. I had the two f177g names as a finding of my own and
dropped it — gilfoyle was fifteen minutes ahead and the diagnosis is better
than mine was: ubuntu-26.04's `sleep(1)` is a uutils multi-call symlink, the
fixture copies its BYTES to `<root>/posse`, and uutils resolves which utility
to be from the kernel's exec path, refusing a basename it does not own.

THE LESSON IS THE ORDER I DID IT IN. x0ses was ranger-base-x78s5's own
handoff child and its status was one `bd show` away; I read the CI leg first
and the child's close last, so I filed a duplicate and had to withdraw it.
Read the discovered-from CHILDREN of a close's handoffs BEFORE taking the
measurement they were filed for — the answer is often already on them, and
when it is, the measurement is confirmation rather than discovery.

And the mechanism ranger-base-x78s5 reasoned from is FALSIFIED while its
conclusion and fix stand, which x0ses settled in place: ubuntu's `tee` was
never a `#!` stub either. It is the same uutils symlink, and `exec -a` was a
no-op because that dispatcher deliberately prefers the kernel's exec path over
argv[0] — `src/common/validation.rs`, so that `env -a` cannot bypass AppArmor
or SELinux. A fix can be right for a reason nobody has yet measured, and the
honest close says which half is which; this one did, by labelling that half
ASSUMED.

## What a green local suite could not have told me

Three of the four closes' live content was invisible to `make test` on this
box by construction, and that is the reusable shape rather than a complaint:
one was a shell self-test's internal determinism (green either way until the
mutant is planted 7 times), one was an unpinned guard whose mutant no
git-derived fixture can kill, and one was an ubuntu-only CI reading. A batched
verify's own suite run over the composite tip is necessary and proves none of
them — it is the control that says the tree is not red for some other reason,
and the findings come from the mutants and the venue.
