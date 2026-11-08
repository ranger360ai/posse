## A PID's cage is the class it needs; the host names the tier that realizes it (ranger-base-1155n)

Built 2026-11-08 against ADR 0083 D7. The key is `cage_realize_<goos>:` —
not the `cage_realize:` / `<goos>:` / `<demanded>: <realized>` the record
writes. The divergence is the whole of what is worth recording here.

### The spelling diverges from the ADR, and it has to

ADR 0083 D7's example is a TWO-level map. posse's YAML is a deliberate flat
subset — top-level keys plus one nesting level, "no anchors/multiline/deeper
nesting" (`internal/posse/yamlflat.go`, docs/notes.d/notes-yaml.md) — and it is
the same subset every config, recipe, session record and PID frontmatter in
this tree is written in, with an awk twin behind it and a shell reader
(`plugin/autostart.sh` `cfg()`) for some keys. Teaching it a second nesting
level for one key with one reader would be a new primitive in that format.

This wall has been met twice before and answered the same way both times: a
KEY FAMILY whose members the harness does not know in advance.
`plan_guard_<window>:` (planusage.go) and `interstitial_<name>:`
(runtimeyaml.go — "a family rather than a list because flat-YAML has no
list-of-maps"). So:

```yaml
cage_realize_linux:
  seatbelt: shims
```

**What a family costs, and what was built to pay for it.** A mistyped member
reads as ABSENT rather than as an error — `cage_realize_linus:` is just a key
nothing looks up — and that is exactly the failure ADR 0080 D9 was ruled on: a
typo read as a default arms the wrong thing. So validation is TOTAL rather than
per-lookup, in `CageRealizeEntries`:

- the bare `cage_realize:` key the ADR wrote is **refused by name**, with the
  family spelling in the refusal. Without that line, an operator who typed the
  record's own example would get a key that parses to nothing, arms nothing and
  says nothing;
- every `cage_realize_*` member is checked whether or not it is this GOOS's —
  the member being edited is rarely the member being read;
- an unknown OS, a scalar where the block belongs (an inline `{a: b}` is a
  scalar to this reader), a tier that is not a tier, `container` on either side
  and one demand mapped twice are each a refusal. Nothing defaults.

### Ratified degradation needed somewhere to live

D7 asks for three things at once: the launch proceeds (dispatch included), the
session is marked degraded, and the cage line — only the cage line — is
waived. `Parity.Degraded` could not carry that: it is the list the launch
refuses on. `Parity.DeclaredDifference` could not either: that is reserved for
a wall that holds by another mechanism, and it never marks a session.

So `Parity.Ratified`, beside both. **The refusal reads `Degraded`; the record
reads both** (`Parity.AnyDegradation`, `Parity.DegradedReason`). A fourth
verdict in the rendered block, spelled like none of the other three: `⚖` and
`RATIFIED DEGRADATION`.

Two readers then had to stop treating the session's `degraded:` field as
evidence of a human's consent, and both were one predicate
(`DegradedBeyondRatified`):

- `RecreateOpts` passes `AllowDegraded: m.Degraded != ""` into a relaunch, on
  the ground that the operator already consented. Nobody consents to this one
  at a keyboard. Left alone, a host whose map lowers every code persona would
  have armed `--allow-degraded` for every seat, and then waived each seat's
  NEXT, real degradation silently, at every refresh.
- G15 says a live session "is running with gates its wall does not realize …
  waived once with `--allow-degraded` and re-armed by every relaunch from its
  meta". Every clause is false for a ratified map. D7 asks for ONE standing
  row while the map lowers anything, which is **G17** (`posse status`), read
  from the config and not from the sessions — it is true with no session on
  the box.

### The relaunch reading, which is not obvious

`CreateSession` records the tier the session GOT, because that is what
`m.Cage` means everywhere it is read. So a relaunch comes back into
`CheckParity` asking for `shims`, and only `ag.Cage` still remembers that
`seatbelt` was the demand. `CheckParity` therefore asks the map TWICE: once
about what this launch requested (which catches a `--cage` the operator typed,
too), and once about `ag.Cage` where that is higher. Without the second, a
waiver would expire at the first refresh and the relaunch would read as a bare
cage shortfall and refuse.

The map is consulted by one function from both the launch (`CreateSession`,
before the seatbelt wrap, `RHQ_CAGE` and the record are written) and the
matrix, so the two cannot disagree about which tier the session is at.

### What the key does not do

Parity is recomputed at the REALIZED tier and every rule is judged there. A
PID denying `Edit`/`Write` bare, mapped `seatbelt`→`shims`, still refuses —
pinned with seatbelt forced AVAILABLE, so the lowering is the map's doing and
not the old "cage seatbelt is not available on this host/build" row wearing a
new name, and with a control at the same tier and no map where the shortfall
refuses as before.

### Files

- `internal/posse/cagerealize.go` — the family, its total validation, the
  resolution, the provenance token, the standing reading, the predicate.
- `internal/posse/parity.go` — `Parity.Ratified`/`CageDemanded`, the map
  applied before every rule check, the fourth verdict. (`CageDemanded` was
  removed a day later: no printer ever read it, and the ⚖ ratified row
  already carries the pair — ranger-base-pl3ck item 5.)
- `internal/posse/herdrback.go` — the launch refuses a map it cannot read; the
  refusal reads `Degraded`, the record reads both.
- `internal/posse/relaunch.go`, `internal/posse/govern.go` — the two readers
  that stopped reading a ratified waiver as the operator's.
- `internal/posse/cagerealize_test.go` — the table and the four surfaces.
- `examples/config.yaml` — documented, commented out, arming nothing.
