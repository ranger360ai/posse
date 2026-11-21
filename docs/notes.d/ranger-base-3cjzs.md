## The mask read XDG_STATE_HOME raw and rex's library expands it first (ranger-base-3cjzs)

`rexNodeStateDirs` took `XDG_STATE_HOME`'s value only when `filepath.IsAbs` said
the raw string was absolute, and its comment gave the reason: the XDG spec calls
a relative value invalid, and a mask does not guess what it meant.

The library rex resolves through does guess, in one specific way.
`github.com/adrg/xdg` v0.5.3's `internal/pathutil.EnvPath`
(`internal/pathutil/pathutil.go:93-100`, read 2026-11-21) is
`ExpandHome(os.Getenv(name))` and only **then** `filepath.IsAbs`, and
`ExpandHome` (`internal/pathutil/pathutil_unix.go:28-40`) substitutes a leading
`~` or `$HOME` against `$HOME` else `/`.

So with `XDG_STATE_HOME=~/st`, rex's tsnet node writes its tailnet identity
under `$HOME/st/rex/tsnet`, and the mask covered **neither that nor anything rex
used**: the value is not absolute raw, so that whole arm contributed nothing,
and the GOOS defaults it fell back to are not where rex wrote. Both mechanisms —
bwrap's step 5b tmpfs and seatbelt's subpath read deny — were therefore silent
over a credential that outlives the seat and the box, for every seat, on a box
whose operator spells the variable that way.

`rexNodeStateDirNative` (the create-and-anchor arm, ranger-base-bncbk) keyed on
the same raw `IsAbs` test, so in that case it created and anchored a directory
rex would not use and did not create the one rex would.

**Not measured live.** This box leaves `XDG_STATE_HOME` unset, so nothing here
was exposed; the finding is a reading of the library against the mask, filed and
verified that way (hoover, 2026-11-21). The expansion rule below is transcribed
from the module source in the module cache, not inferred.

### The fix

One expansion, `rexNodeExpandHome`, transcribed from `ExpandHome` rather than
tidied, because every tidier rule is a hole: the library's tests are
`path[0] == '~'` and `strings.HasPrefix(path, "$HOME")`, so `~st` is
`$HOME/st` (and never user `st`'s home) and `$HOMEx` is `$HOME/x`. A
`~/`-and-`$HOME/`-only rule would have left both of those uncovered at names rex
writes to. `~` and `$HOME` alone Join to the home itself.

`rexNodeXDGStateDir(home, value)` is then expand-then-`IsAbs`, shared by both
arms so they cannot drift:

- **The mask** asks it for every candidate home, because it cannot know which
  home rex ran under — the two `liveSSHHomes` answers plus `/`, the library's
  own fallback when `$HOME` is empty. An already-absolute value expands to
  itself under every home and dedupes back to the single row it has always had,
  in the position it has always had. The GOOS defaults stay listed whatever
  `XDG_STATE_HOME` says: an entry rex does not use costs one inert tmpfs, and
  wider is the safe direction for a mask.
- **The create** asks it for exactly rex's own home, so the directory posse
  mkdirs and anchors is the one rex will write to. Its no-`$HOME` rule is
  unchanged from ranger-base-bncbk: with no home it makes no directory, the
  fail-closed direction for a mkdir, and the mask carries the `/` row either
  way.

A value still relative after expansion is ignored by the library too, so it is
still ignored here.

### Pins

`internal/posse/rexnode_qa_test.go`:

- `TestQARexNodeStateDirsExpandsAHomePrefixedXDGStateHome` — the full mask list
  for `~/st`, `$HOME/st`, `~st`, `$HOMEx`, `~`, `$HOME`, an absolute value,
  `st`, `./st` and `""`; the same ten-ish rows for the native arm across both
  GOOS; and that every expanded native path is a member of the mask it was
  resolved beside, which is what keeps a posse-made directory from having no
  wall over it.
- `TestQAEverySeatsWallHidesAHomePrefixedRexNodeState` — the same thing at the
  real render, which is the level the exposure lived at: the bwrap tmpfs, the
  seatbelt subpath deny inside a `file-read*` block after the last read allow,
  and `DenyReadDirMake` naming the expanded path.

Both were watched red against the pre-fix resolution before they were trusted
(`TestQARexNodeStateDirsResolvesEveryHomeAndXDGState` and
`TestRexNodeStateDirNativeIsOneGOOSsAnswer`, the absolute and relative rows,
stay green across the change, which is the no-regression half).

### Not closed here

- ranger-base-1mxhx

