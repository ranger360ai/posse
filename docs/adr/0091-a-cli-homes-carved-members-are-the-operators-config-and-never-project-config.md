# ADR 0091 — A CLI home's carved members are the operator's config, never project config

*Status: accepted 2026-11-11 (ranger-base-mkrsv, from ranger-base-f67jx) ·
owner: architect · amends ADR 0002 amendment 2026-08-26 (project config is a
launch input) · builds on ADR 0090 D3 and the exec-member carves
(ranger-base-6akzw, -zmk4q, -zvgql)*

## Context

`ProjectConfigTrust` (parity.go) degrades a launch whose session dir holds
executable config the runtime reads before any turn. Its threat is a writer
the operator did not choose: a repo's committer. `project_config:` lists the
entries relative to the session dir.

The bwrap wall carves each CLI home's executable members read-only. A mount
cannot cover a name that does not exist, so the launcher creates the absent
ones first: `{}` or zero-byte seeds, and mkdir'd directories. When the session
dir IS `$HOME` (any recipe with no `dir:`), those members are also
project-config entries. MEASURED 2026-11-11 (wallseedprojectconfig_qa_test.go),
after all three runtimes' prep has run:

| runtime launched in `$HOME` | verdict | first entry |
|---|---|---|
| claude | clean | `{}` under a keyed list |
| codex | degraded, "present" | `.codex/config.toml`, zero-byte seed |
| grok | degraded, "a directory" | `.claude/plugins`, mkdir'd by a *claude* seat's prep |

The refusal says "remove the file", and the next caged launch seeds it again.
The only ways out are `trust_project_config: true` or `--allow-degraded`, and
both are the wrong instrument here.

The mistake is in the premise, not in the seed or the classifier. The
classifier asks "what does this dir add, from a writer I did not choose?" and
then counts the operator's own CLI homes as that dir. No seat can write any of
these ten entries:

- a seat whose state grant reaches a home gets each member carved read-only;
- a seat whose grant does not reach the home cannot write there at all.

That holds for every runtime and both mechanisms (`claudeExecConfig`,
`runtimeConfigFiles`, `runtimeExecMembers`, all filtered by `writeGranted`).
The writers left are the operator, the operator's own CLIs, and posse's empty
seeds. Degrading on any of them buys nothing. For its own home, a CLI reads
that config from every dir anyway. Launching in `$HOME` changes nothing about
it.

## Decision

**D1. An entry that resolves to a declared executable member of a CLI home is
not project scope.** `ProjectConfigTrust` skips an entry when
`<dir>/<rel>` is the same path (`samePath`) as a member of the set below. It
skips the entry rather than classifying it clean, the way `personaModeOwnsAll`
skips posse's own persona-mode dir.

The premise is that the operator chose the home, and that premise needs the
home variable to be ABSOLUTE (ranger-base-afqqz, finding ranger-base-lu513).
`runtimeHomeOverride` absolutizes a relative `CLAUDE_CONFIG_DIR`, `CODEX_HOME`
or `GROK_HOME` against posse's cwd, and that cwd is the session dir when the
operator types `posse <persona>` in a repo. So `CLAUDE_CONFIG_DIR=.claude`
would make the repo's own `.claude/settings.json` a "member". The writer of
that file is the committer, and the cwd chose it, not the operator. The
exemption therefore reads `trustExemptCLIHomeExecMembers`. That is the
unfiltered list minus every member whose variable's raw value is set and not
absolute. Such an entry is classified like any other. The carves keep those
members, and `runtimeHomeOverride` keeps absolutizing for its other
consumers. The `$HOME` default and absolute overrides are unchanged.

**D2. One list, read by both sides.** The member set is one unfiltered
function, `cliHomeExecMembers(home)`. It covers `ClaudeConfigDirIn`,
`codexHomeIn` and `grokHomeIn` with their dir and file names, plus
`config.toml` for codex and grok. The three carve functions become that list
filtered by `writeGranted`, and the classifier reads it unfiltered. A name
dropped from the carve therefore drops from the exemption in the same edit,
and a name added to the exemption is carved. They cannot drift, for the same
reason `personaModeOwnsAll` reads `PersonaMode`. `home` is `wallHome()`, the
`$HOME` the carve resolves against.

**D3. Equality, never containment.** Only the member path itself is exempt,
not anything under a home. `dir=~/.claude` makes `.claude/settings.json`
resolve to `~/.claude/.claude/settings.json`. That is inside a home, carved by
nothing, and writable by a claude seat, so it classifies as before.
`$HOME/.mcp.json` and `$HOME/.cursor/*` are in no home and are unchanged.

**D4. The fail-closed floor is untouched.** A zero-byte or non-object file at
a project entry still degrades. The txd7 pin ("an empty file is not a missing
file") stays as it is. D1 changes which paths the classifier reads, not how it
reads them.

## Consequences

- All ten census rows read clean. A `$HOME` launch is clean for all three
  runtimes once the operator's real homes have no repo-written config, and
  those homes cannot hold any. Both f67jx pins red by name and flip to the
  new verdicts, which is how they were built to fail.
- **The operator's REAL home config stops degrading a `$HOME` launch too.**
  An example is hooks in `~/.claude/settings.json`. This is intended: those
  hooks run in every claude launch from any dir, and gating one dir did
  nothing. The same verdict now holds on darwin, where nothing is seeded.
- Cross-runtime reads are covered by the same argument, with a weaker
  premise. Grok reads `~/.claude/*` as project config in a `$HOME` launch.
  Whether it also reads them at home scope from every dir is ASSUMED unknown.
  The exemption does not rest on that. It rests on who can write the file,
  which is the operator only.
- A repo can symlink an entry to a home member (`.grok/config.toml →
  ~/.grok/config.toml`). `samePath` follows the link and exempts it. The repo
  then supplies a pointer and the operator supplies the content, so this is
  accepted. A repo cannot write the target.
- Exit hatch: delete D1's `continue` and the f67jx pins' old rows come back.
  No state is held.
- Not closed here: ranger-base-74pqi.

## Alternatives rejected

1. **Zero bytes read as absent.** This fixes one row of ten. The mkdir'd
   directories and `{}` under a whole-file entry would each need a further
   rule ("empty dir = absent", "`{}` = absent"). That is three loosenings of
   the fail-closed floor that ranger-base-txd7 pinned deliberately. They would
   also have to answer the mid-write read that `seedFileWhole` exists for. It
   also leaves the operator's real home config degrading a `$HOME` launch,
   which was the wrong verdict all along.
2. **Exempt `dir == $HOME`.** This is the smallest diff and the one I reached
   for first. It is too wide: it also exempts `$HOME/.mcp.json` and
   `$HOME/.cursor/*`, which no carve protects. It is also too narrow: it misses
   a `CODEX_HOME` whose parent is not `$HOME`. It names a place when the
   property is a writer.
3. **Exempt anything inside a CLI home.** This is wrong by D3's example.
   Containment reaches subtrees a seat with a dir grant can write.
4. **Stop seeding.** Impossible: a mount needs a name (`Seeded`).
5. **Scope the comment and change nothing.** That is what f67jx did while
   this was pending. It leaves a refusal whose named remedy cannot work.
