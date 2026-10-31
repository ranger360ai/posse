# A relative runtime home is made absolute at the one read (ranger-base-sczjy)

2026-10-31. ADR 0077 D1's typed runtime-home pin carried its value VERBATIM.
For an ABSOLUTE value that is the whole point and it works. For a RELATIVE one
the guarantee inverted: the launcher resolves the value against ITS cwd and the
seat resolves the same string against the session dir, so posse seeded one file
and the seat opened another — ranger-base-c33j6's shape exactly, surviving the
fix built to end it. Found by holden verifying ranger-base-49v2h under
ranger-base-49iga.

Measured on **go1.26.5 darwin/arm64, this box**, in this bead's session
worktree.

## The failure, MEASURED before the fix

holden's probe, 2026-10-31, worktree `holden-posse-ranger-base-49iga` at
main/HEAD 8077fef3, with `CLAUDE_CONFIG_DIR=relcfg`:

```
ClaudeConfigFile()  = "relcfg/.claude.json"   (IsAbs=false)
the seed LANDED at    <launcher cwd>/relcfg/.claude.json
the grant is in that file, for the session dir
typed pin           = "CLAUDE_CONFIG_DIR='relcfg' "
the line carries the pin = true
the seat (cwd=<session dir>) resolves
                      <session dir>/relcfg/.claude.json   exists=false
```

A launch that completes, a line correct by the pin's own assertion, a grant
that is written, and no trust dialog suppressed — which is the observable
ranger-base-c33j6 was filed for.

`runtimeHomeOverride`'s "in force" test was `v != ""`. A relative value is not
empty, so it was in force, and BOTH directions took it: posse resolved through
it and posse typed it. The doc at `runtimehome.go` already refused the EMPTY
value for precisely this reason — it would hand claude "a cwd-relative config
directory ... the shape ranger-base-58b5 refuses everywhere else in posse" — and
a relative value is that same shape with a non-empty string.

## The fix: absolute at the ONE read, not refused at each consumer

`runtimeHomeOverride` (runtimehome.go) now returns the value made absolute:

```go
if !filepath.IsAbs(v) {
	if abs, err := filepath.Abs(v); err == nil {
		v = abs
	}
}
```

Three choices in that, and each had an alternative worth naming.

**At the one read, and not a guard per consumer.** It is what makes "the seat
reads what posse read" true BY CONSTRUCTION rather than true whenever the
operator's shell happens to hold an absolute path. There are four consumers of
this read (`ClaudeConfigDirIn`, `codexHomeIn`/`grokHomeIn`, `RuntimeHomePin`,
`credentialDirNamed`'s bit) and a per-consumer guard is four spellings of one
rule — the class ranger-base-api7c, ranger-base-wd4be and ranger-base-x5f6p
already paid for inside posse alone, before a seat was in the picture.

**Absolutize rather than REFUSE.** Refusing a relative value would make it "not
in force", so posse would resolve `~/.claude` and type `env -u` — coherent, and
it silently discards the configuration the operator typed. If that relative
directory holds another account's credentials, refusing sends the seat to the
wrong account. Absolutizing honours the value the way the operator's own shell
resolves it, since that shell is the cwd posse was launched in.

**Gated on `filepath.IsAbs`, so an already-absolute value is byte-identical** —
not Cleaned, not symlink-resolved. `shQuote` has to hand the seat the operator's
own spelling (holden's second probe, CLEAN: six hostile values — spaces, an
embedded single quote, a tab, a semicolon, a command substitution and a newline
— all arrive at the seat verbatim through `/bin/sh` on the rendered fragment,
and the substitution does not execute), and resolving symlinks would make posse
name a path the operator never typed. `IsAbs` is also `credentialDenyable`'s
predicate (seatbelt.go), which is the spelling precedent: ranger-base-o05yg
closed this same class one rung down for a relative `CODEX_HOME`/`GROK_HOME`.

A failing `os.Getwd` leaves the value verbatim, which is the pre-fix answer and
the only honest one: the cwd a relative path is relative to does not exist, so
there is no absolute path to name.

## The credential wall's relative arm CHANGED SIDES, and that is the fix working

ranger-base-o05yg's arm in
`TestQACredentialReadDenyNamesNothingUnderTheCwdWithNoHome`
(credentialdenyclihome_qa_test.go) asserted that a relative `$CODEX_HOME`
names NOTHING in the deny — because `codexHomeIn` returned it verbatim, so the
candidate was that relative directory with the CLI's credential file name
joined onto it (`credentialFileCandidates` in credential.go is where that name
is spelled), which the deny loop `absResolve`d under the SESSION's cwd: a wall
over an ordinary file and no wall at all over a credential (ADR b52r3
decision 3).

With the value absolutized at the read, that route no longer arrives there.
`CODEX_HOME=mycodex` now yields that same candidate under `<posse's cwd>/mycodex`,
`credentialDenyable` ADMITS it, and admitting it is **correct**:
`RuntimeHomePin` types that same absolute value onto the seat, so the directory
walled and the directory the seat's CLI writes are one directory. What the
relative value used to buy was no wall at all over a real credential file,
which is the worse half of decision 3's sentence.

So the arm is kept and inverted rather than deleted — it now asks for the
absolutized store to BE walled, and separately that nothing RELATIVE is ever
named, which is the half of decision 3 no absolutization can satisfy on its own
and which the empty-`$HOME` arms above it still reach. `credentialDenyable`
stays unchanged, as the backstop for the two shapes no variable can carry: `""`
and an unexpanded `~/…`.

## The pins

Both in `internal/posse/runtimehome_qa_test.go` (arm 1), beside the three arms
that measure the absolute case — which were not wrong, only silent here,
because every fixture on this path names its directory with `t.TempDir()`.

- `TestARelativeRuntimeHomeIsMadeAbsoluteAtTheOneRead` — all THREE runtimes
  with a home var, four relative spellings (`relcfg`, `./relcfg`, `a/b`,
  `relcfg/`): the resolver answers absolute, the pin types that same path, and
  the relative string the operator exported is explicitly NOT what reaches the
  seat. Plus the control the fix is gated for: four absolute values, including
  two Clean changes and one with a space, all byte-identical through both
  directions.
- `TestTheTrustSeedLandsWhereASeatWithARelativeConfigDirResolvesIt` — holden's
  probe with the logs turned into assertions, and the relative directory NOT
  pre-created, so it also covers the bead's second outcome (`writeJSONInPlace`
  MkdirAll's the config directory, and before the fix it created
  `<launcher cwd>/relcfg` while the line sent the seat under
  `<session dir>/relcfg`; where the launcher could not create it,
  `claudeTrustWriteRefusal`'s remedy — "set CLAUDE_CONFIG_DIR to a directory
  this launcher can write" — was advice a relative value satisfies while
  reproducing the divergence). The seat's half is asked, not spelled: the pin
  fragment is RUN over a decoy value and `ClaudeConfigFile()` is then called
  from the SESSION dir, so both things that differ between launcher and seat —
  environment and cwd — come from the line.

MUTATION-CHECKED 2026-10-31: with `runtimeHomeOverride` returned to the
verbatim body, both pins red (arm 4 on the resolver, the pin value and the
verbatim-typed assertion; arm 5 on `ClaudeConfigFile() = "relcfg/.claude.json"`)
and so does the inverted credential arm. With the fix, green.
