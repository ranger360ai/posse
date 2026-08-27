# HISTORY

posse (github.com/ranger360ai/posse · posse.bot) supersedes the private
repository the harness was developed in. That archive keeps its full
history; this repo started clean at publication (2026-08-22) — no
inherited git history, no bead database — seeded by an explicit
allowlist copy (ADR 0012 D3/D6). Licence: Apache-2.0.

## Private-tracker ids are inert provenance markers

Ids of the forms `rangerhq-xxxx` and `ranger-base-xxxx` appear throughout
the ADRs, code comments, tests, and goldens. They reference private
trackers of the instance this harness was developed in — `rangerhq-xxxx`
its retired pre-publication tracker, `ranger-base-xxxx` its live private
tracker. They are inert provenance markers: they do not resolve here, and
nothing promises they ever will. The convention is legacy-only and
forward-closed — new public work cites public ids (issues and PRs in this
repo) only, and ids minted here never take these prefixes.

## ADR numbering

ADR numbering carried over from the private archive (ADR 0012 D6) for
ADRs 0001–0012 only: those are restated in `docs/adr/` under their
original numbers, each with a provenance header; the full-fat originals
remain in the archive. The plan at publication reserved 0013–0018 for
the archive's ADRs of those numbers, but this repo minted native ADRs
onto them starting 2026-08-24, so **from 0013 onward the two numbering
lines have diverged and numbers are repo-local**. A bare `docs/adr/00NN`
past 0012 is ambiguous across the two repos: cross-repo and cross-era
references go by title or practice name, never by number alone. An
archive ADR restated after the divergence takes the next free number
here, with a provenance header naming the archive document (ADR 0026,
restating the archive's research-spikes ADR, is the pattern).
