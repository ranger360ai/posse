## A record the home wrote about itself is not provenance (ranger-base-rgx0)

`posse init` retires a generic out of `agents/` only when that file is posse's
own — an example the operator edited is the persona they adopted in place, and
taking it leaves real work parked on a name that no longer loads
(ranger-base-qajs). The identity test has to survive version skew, because the
example PIDs are shipped prose and they change: 95c4b70 added
`- Bash(posse promote:*)` to the deny list of all nine, so a home seeded by an
earlier posse holds bytes this binary no longer ships (ranger-base-8ehw).

The seeded promote manifest looked like the record that answers it. It is not.
`SeedPromoteManifest` writes a manifest whenever a home has none — **including
on an upgrade** — hashing whatever is on disk at that moment. It arrived in
95c4b70 (2026-08-26), so every home older than that got its manifest from a
later init, not from the init that seeded it. Measured live on two inits:

    # a pre-95c4b70 home, one generic the operator had adopted
    $ posse init     # #1: keeps qa.md, prints why... and hashes it AS SEEDED
    $ posse init     # #2: live == manifest -> "untouched since seeded" -> retired

Init #1 judged that file as not-posse's and said so, then wrote the record that
made init #2 disagree with it. **A store that records what it observed cannot
answer who wrote it.** `seeded` is an ANCHOR for drift ("these bytes were here
when posse started watching"), never provenance ("posse wrote these bytes"),
and the two read identically at the call site.

The fix asks from posse's own side of the line: `internal/rhq/exampledigests.go`
holds every sha256 posse has shipped for each example PID, generated from this
repo's history. Nothing the operator wrote can be in it, so there are no false
positives, and a version missing from it retires nothing — the failure is the
old leak, not a lost persona. Two tests keep it honest: the current embed's
digests must be in the table, and the table must cover every version git
history knows about.

The contract when the examples change (ranger-base-8zhr will change them):
**append the new digest, never replace one.** An entry that leaves the table is
a home posse can no longer recognise its own file in.
