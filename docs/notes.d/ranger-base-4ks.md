## One spelling of "the whole tree", read by every wall (ranger-base-4ks)

ADR 0014 §1 says `Edit(**)` **is** the bare `Edit`. That sentence is cheap to
write into a parser and expensive to leave there alone, because at the time
the classifier landed three separate places in this package spelled the bare
rule out by hand:

    parity.go:178      case rule == "Edit" || rule == "Write" || rule == "NotebookEdit":
    cageinner.go:349   case "Edit", "Write", "NotebookEdit":     // deniesFileWrite → repo :ro
    seatbelt.go:84     if r == "Edit" || r == "Write" || ...     // → cwd out of the writable set
    runtime.go:504     if has["Edit"] && has["Write"]            // → codex -s read-only

Teaching only the *matrix* the long spelling would have produced the exact
failure ADR 0014 exists to prevent, one layer down: `posse gates` printing
`✓ Edit(**) L2 seatbelt` over a profile that still granted cwd, because
`SeatbeltWritable` had never heard of the parenthesised form. A parity claim
is only as true as the renderer that reads the same list. So the predicate is
one function — `wholeTreeWriteDeny(deny) map[string]bool` — and all four call
it. **When an ADR declares two spellings equivalent, the deliverable is not
the parser; it is that every consumer of the old spelling now goes through
it.** Grep the triple before adding the synonym.

The dual is worth naming too, because it is the same reasoning pointed the
other way: `deniesFileWrite` must **not** answer yes for `Edit(docs/adr/**)`.
A subtree deny answered by a whole-repo `:ro` mount refuses the paths the PID
deliberately left open — over-enforcement, which ADR 0014 §2 already refuses
to count as realization for codex `-s read-only`. Same rule, same reason: a
wall that is bigger than the gate is a different gate.

### The window this bead opens on purpose

`CheckParity` now claims `L2 trailing deny (subpath …)` and
`L4 :ro overlay (…)` for a subtree glob, and **neither is rendered yet** —
`seatbelt.go` emits no trailing deny block and `cage.go` no overlay. That is
the architect's sequencing (ranger-base-nuu and ranger-base-yu5 are blocked on
this bead: the matrix has to name the layer before the renderer can be tested
against it), not an oversight, and it is survivable only because no PID in the
repo or in `examples/agents/` carries a parametrized rule — verified by grep at close.
Until those two land, a hand-written `Edit(docs/adr/**)` on a `cage: seatbelt`
PID launches clean with a `✓` behind it and nothing behind that. INSTALL.md §7
says so in those words.

### Two grammar edges that are not arbitrary

- `Edit(/**)` strips to the empty string, which would join the session dir and
  silently become "the repo". It is the filesystem **root**, and the parser
  says so — an absolute glob that strips to nothing is `/`, never `.`.
- The classification (is this a subtree?) is a property of the rule; *which*
  directory it names is a property of a session. They are separate functions
  for that reason, and `CheckParity` — the directory-**independent** matrix —
  only ever calls the first. `Resolve(dir)` returns `""` rather than guessing
  when a relative glob has no cwd to join.
