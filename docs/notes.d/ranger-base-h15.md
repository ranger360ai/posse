## The seatbelt's trailing deny (ranger-base-h15)

The L2 posture above says the profile "never grants the rest of the home",
and that sentence was true of the allow block and false of the session. It
is a *structural* claim — nothing in the writable set names the promoted
constitution — and a structural claim about a set holds only while the
thing it excludes is outside every member of it. It is not, in one shape
that the crew uses constantly: the home is reached through a symlink into
the repo that holds it, so a session dispatched **into that repo** is
granted the constitution inside an ordinary `cwd` grant. Measured twice
before this landed (ranger-base-6ne by writing a PID from a caged session;
ranger-base-0djg by rendering a probe home and executing under it).

So the profile now ends with a block the allow block cannot outvote — SBPL
takes the **last** matching rule, which is also why it must come after the
allow and not before (deny-first leaks; ADR 0014 §3 measured that, and this
block is the slot §3 reserves for a PID's path-scoped denies):

    (deny file-write*
      (subpath "<home>/agents")  …config.yaml, recipes, skills, envs, promoted.json
      (subpath "<home>/state/gates")     ; every persona's L1 shims, profiles and logs
      (subpath "<hooks dir>")            ; the L3 slots, asked of git — core.hooksPath moves them
      (literal "<renamable ancestor>")   ; the rename seal, below
    )
    (allow file-write*
      (literal "<gates>/<persona>/refusals.log")   ; L1's audit trail must still append
      (literal "<gates>/<persona>/shell.log")
    )

**Enumerated at the artifact level, never at a repo root.** The store of
record lives in the same tree as the constitution and the writable set
follows `.beads/redirect` into it, which is the only reason a caged persona
can claim, comment and close. The trailing deny beats that grant exactly as
it beats `cwd` — measured on a live profile — so a deny list that ever says
"the instance repo, minus what we need" kills the record stage silently:
parity grades *denies*, and a cage that denies too much still reports every
gate realized. If the list ever has to move, re-allow the resolved beads
home and its git dirs **after** the deny.

**The rename seal.** A `subpath` deny is a statement about a path. `mv rhq
rhq2` is a write on `rhq` — granted, and named by no deny below it — and the
constitution is then writable at a path the profile never heard of, silently
and reversibly. So each denied path's ancestors are denied as *literals*
while both the ancestor and its own parent are writable (a rename needs the
destination beside the source; the first ancestor whose parent nothing
grants cannot go anywhere). A literal deny on a directory does not stop
writes inside it — measured — so the seal costs a session nothing. For an
ordinary project session it is one entry, `<cwd>/.git`.

A hardlink needs no rule: `ln <denied>/x ./x` into a granted directory is
already refused by the `file-write*` deny, measured under both spellings of
the path.

**What this does not close**, so the posture stays honest about having
exceptions rather than being one wall:

- `refusals.log` stays writable by the session it records, because SBPL
  cannot tell an append from a truncation (ADR 0025 says the same of the
  container tier's mount). What the deny buys is that no *other* persona's
  session can reach it, and that the shims and rendered profiles it sits
  beside are now out of reach of both.
- `bd` mutates the queue through a daemon socket, i.e. through a process
  outside the sandbox. L2 does not gate bead mutation — by design, and
  correctly, but it is a second exception alongside the `.git` grant.
- `.git` is still granted whole for a session that commits, so the shared
  refs of a worktree's common dir remain writable; narrowing that is
  ranger-base-m2wf, not this.
