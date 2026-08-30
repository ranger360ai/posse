## The rest of the refusal paths that spelled `date` (ranger-base-l97n)

hr5x's residual, and the same rule: *a refusal path may not use a verb it
can refuse.* Four rendered scripts outside the shim still ran `date` by bare
name — the gate shell's PATH-reorder note, and the `refusals.log` lines of
both L3 hooks (a fifth arrived with the constitution guard, ranger-base-ak3e,
after the bead was filed). All of them run inside a persona's session, where
a gates dir leads PATH, so under a PID carrying `Bash(date:*)` the timestamp
was answered by a gate: the log line went out with nothing in front of it and
a stray `refused by posse gate: date` landed on stderr in the middle of a git
command. Bounded at one refused child each, because hr5x had already taken
the cycle out of the shim's own refusal.

**Two sites, two fixes, and the difference is WHEN the path is known.**

- The gate shell is *rendered per persona*, so it takes the shim's own
  treatment: `resolveOutside("date", binDir)` at render time, spliced in as
  `__STAMP__`. `quotedStamp` is `refusalTimestamp` with the `$` escaped,
  because that site is assembled inside a double-quoted assignment and
  eval'd later by the runtime's shell — the same escaping as the `\$PATH`
  beside it. This was the worst of the five: the note fires precisely when
  the gates dir is *not* first in the replayed PATH, so its lookup ran
  against a path some other element led, before `PRE` had rebuilt it.
- The hooks are *consts, installed per repo and shared with the operator*,
  and `l3Identity` compares the installed bytes against the const — so a
  render-time absolute path there would make hook identity a function of the
  box that installed it. They resolve at run time instead, in `posse_stamp`
  (`hookStampFunc`, defined once and spliced into both hooks): walk PATH,
  skip every `*/gates/*` element, take the first `date` outside them, `-`
  if there is none.

**PATH itself is left alone, deliberately.** Scrubbing the gates dirs off
PATH for the hook's duration is the shorter fix and the wrong one:
`installHook` chains a foreign hook behind ours, and that hook would then
run with the persona's fence taken off its PATH. The timestamp is cosmetic;
the fence is not.

**Testing it in the production shape.** Where hr5x had to invert the PATH to
stay bounded, these can run the real thing — a real rendered `date` shim,
really in front — because the shim's refusal no longer recurses. Both pins
carry a control that fires that shim on purpose, since "the shim was never
reached" is also what a PATH that could not reach it says. The gate shell's
control is the sharper one: it runs a bare `date` at exactly the note's
moment (inside the `-c` string, after the reorder, before `PRE`), so control
and assertion cannot disagree about the PATH.

Driving the wrapper's usercmd slot means spelling argv0 as `--`:
`<shell> -c <string> -- <usercmd>` puts the slot in `$1`, which is what the
argv walk prefixes. With `zsh` in the argv0 position the walk reaches `done`
and the note is never rendered into the string at all — a green test that
measured nothing.

Mutation checks, three, each restoring one half: the bare form in the gate
shell (3 assertions, the note logged as `" gates dir not first…"` with the
leading space where the time belongs); the bare form in the hooks (both
behavioral arms plus every static one); and `posse_stamp` without its
`*/gates/*` skip, which puts the shim back in front of the real `date` and
reds both hooks — the skip is the fix, not decoration.

**Operator note:** both hook consts changed, so every installed copy is
stale until it is restamped. `posse gates install-hooks` does it, and so
does every persona launch into the repo (herdrback.go).
