## The ceiling's two readers: why the positive pathspec is `:(top)` and not `.` (ranger-base-qvy0n)

ADR 0068 D1 splits the data ceiling's content subject in two: one `git diff`
that refuses over every staged path except `.beads/*.jsonl`, and one that
reports over that file alone. Both pathspecs carry git's `(top)` magic, and
the reason is a measurement rather than a style preference.

### What was measured

MEASURED 2026-10-08, git 2.50.1 (Apple Git-155), darwin 25.4.0. A scratch
repo with one commit, then a classed line staged into **both** `docs/a.md`
and `.beads/issues.jsonl`, and the reader run from two working directories.
The reader is the hook's own:

```sh
git diff --cached -U0 $SHAPE "$posse_base" -- <pathspec> |
  grep -a '^+' | grep -av '^+++'
```

| pathspec | from the worktree root | from `sub/deep` |
|---|---|---|
| `. ':(top,exclude).beads/*.jsonl'` | the `docs/a.md` line | **nothing at all** |
| `':(top)' ':(top,exclude).beads/*.jsonl'` | the `docs/a.md` line | the `docs/a.md` line |
| `':(top,exclude).beads/*.jsonl'` (no positive) | the `docs/a.md` line | the `docs/a.md` line |
| `':(top).beads/*.jsonl'` (the reporting reader) | the jsonl line | the jsonl line |

`.` is the *process's* current directory. From `sub/deep` it names a
subdirectory that holds no staged path, so the whole refusing reader goes
quiet — and a reader that reads nothing refuses nothing. The `(top)` form is
identical from the root and unchanged from anywhere else.

### Why it matters even though git chdirs

git runs hooks from the top of the worktree, so the `.` spelling works today
and would have kept working. The difference is only visible where the wall
fails **open**, which is the one direction a wall may not fail: the cost of
`(top)` is five characters, and the cost of `.` is a silent hole the day
something runs the hook from somewhere else. The two readers are
`ceilingRefusePathspec` and `ceilingReportPathspec` in `internal/posse/gates.go`.

ADR 0068's Verification 5 ("committed from a subdirectory → identical") is
this measurement; the pin
(`TestQADataCeilingReportsOverTheBeadsJsonl`) does not reproduce it, because
the hook's cwd is git's to choose and chdir'ing the *commit* does not move
the *hook*. The table above is the record.

### The id extraction, also measured

`sed -n 's/.*"id":"\([^"]*\)".*/\1/p'` over the matching `+` lines gives
`rb-2` for `{"id":"rb-2",...}`, and gives **nothing** for a matching line with
no `"id"` field — which is why the report has a second branch
(`ceilingReportNoID`) that says it cannot name the record rather than printing
an empty list. Real bd rows are spelled `{"id":"ranger-base-00a0","title":…`,
with no space after the colon, so the pattern reads the live format.

Multiple classes are passed as one `grep -aE -e <ere1> -e <ere2> …` rather
than a `|`-joined ERE: the values are the operator's own and joining them
would rewrite them. grep reads the union, which is exactly the set of lines
`posse_check` counted.
