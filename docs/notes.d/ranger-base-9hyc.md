## `--out` is emptied, so it is checked first (ranger-base-9hyc)

`scripts/release-artifacts.sh --out <dir>` absolutised its argument and ran
`rm -rf` on it. Measured on darwin 25.4.0, 2026-08-28: an `--out` pointing at a
directory with a nested `notes.md` in it came back holding five build artifacts
and nothing else. The header said "wiped before use", so this was a hazard
rather than a lie — but the wipe was unbounded, and precondition 5 of
`docs/runbooks/release.md` has a human typing that flag by hand one step away
from the irreversible one. `--out ~/src`, `--out ~/Downloads`, or a caller
whose `$OUT` expanded empty (then `OUT=$PWD`, and the wipe takes the checkout).

**The wipe stays; what may be wiped is now the question.** A tarball left from
the previous version would be swept into `checksums.txt` and into the release
upload glob, so writing beside old output is worse than emptying. The guard
admits an `--out` that is absent, empty, or holds nothing but
`posse_*.tar.gz`, `checksums.txt`, `posse.rb` — plus `.DS_Store`, which Finder
writes into any directory someone has looked at and which is nobody's work.
`/`, `$HOME` and the repo root are refused by name, on a **canonicalised**
path: `--out .`, `--out $HOME/x/..`, a trailing slash and a symlink pointing
into `$HOME` all have to land on the string the comparison uses, or the
refusals are string games.

Two decisions worth keeping:

- **No `--force`.** The way to wipe a directory holding your own things is to
  type the `rm` yourself. An escape hatch on this flag would put the footgun
  back in the same hand, at the same moment, with one more word in front of it.
- **An allowlisted *name* does not license the wipe.** A *directory* called
  `posse_0.3.0_darwin_arm64.tar.gz` would be removed whole, contents unseen, so
  the entry has to be a plain one as well as a known one.

`outguard_qa_test.go` runs the script — not reads it — over a throwaway git
repo with its own `$HOME`, with a control arm that excises the guard between
its `# >>> out-guard` / `# <<< out-guard` markers and shows the same fixture
losing the same canary. Every refusal is an assertion of absence, which a rig
that measures nothing also satisfies; the control is what makes them evidence.
The stray-*file* case exists because it was found by mutation: widening the
allowlist to `*` left every other case green, since each of the others is a
stray *directory*.
