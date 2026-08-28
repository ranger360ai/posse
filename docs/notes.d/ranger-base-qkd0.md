## `--out` is truncated, so it is checked first (ranger-base-qkd0)

`scripts/tap-formula.sh` ended in `render > "$OUT"`. `>` truncates whatever it
is handed, and nothing asked what that was. Measured on darwin 25.4.0,
2026-08-28: `--out .zshrc` over a file holding `zshrc content` came back
holding a Homebrew formula. Found while closing `ranger-base-9hyc` — the same
class as that `rm -rf`, one order of magnitude smaller: one file, no recursion,
no directory tree. That is why it was a separate P3 bead rather than scope on
that one, and why the guard here is fifty lines rather than the sibling's
ninety.

The hazard is the same one, though, and it is positional: precondition 5 of
`docs/runbooks/release.md` has a human typing `--out dist/posse.rb` **one
argument away from a `--checksums dist/checksums.txt`**, at the step next to
the irreversible one. The two paths are adjacent on the line, both are typed by
hand, and one of them is truncated.

**The write stays; what may be truncated is now the question.** A release
re-renders the same `posse.rb` every time, so overwriting is the normal case.
The guard admits an `--out` that is a `.rb` and is absent, empty, or a formula
a previous run rendered. A directory, a symlink, and any other non-regular file
are refused, and so is a parent directory that does not exist. `-` — the
default — writes to stdout and touches no file, unchanged.

Three decisions worth keeping:

- **The extension check is necessary, never sufficient.** The content check
  cannot see a typo'd path that does not exist yet, and nothing else can; the
  `.rb` requirement is the only thing that refuses `--out ~/.zshrc` on a
  machine where `.zshrc` is absent. It licenses nothing on its own: a
  hand-written `hand.rb` is still refused, by its first line.
- **The recogniser is the banner, and it is one constant.** `MARKER` is used
  by the renderer that writes the "do not hand-edit" line and by the guard that
  reads it back, so the two cannot drift apart and quietly stop matching. The
  accept-side pin renders twice into the same path: if the banner and the token
  ever diverge, the second release of the year is refused by the first, and
  that test says so.
- **No `--force`**, for `ranger-base-9hyc`'s reason. To overwrite a file of
  yours, delete it yourself, so the blast radius sits on a command line you
  wrote rather than on a mistyped flag.

`tapformulaout_qa_test.go` runs the script — not reads it — over canaries in
`t.TempDir()`, with a control arm that excises the guard between its
`# >>> out-guard` / `# <<< out-guard` markers and shows the same two canaries
replaced by a formula. Every refusal is an assertion of absence, which a rig
that measures nothing also satisfies; the control is what makes them evidence,
and it carries a canary per refusal *mechanism* (the name check and the content
check) rather than one for the pair. The fixture is re-planted at the top of
every subtest: found by mutation, where neutering the content check let the
`hand-written .rb` case clobber a canary that eight later cases then reported
as their own failure — a probe that inherits what the last probe mutated
measures the last run.
