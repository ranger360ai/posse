# The pulse family reaches the config surface, and `pulse_renag:` gets a reader so its documented 30m is held (ranger-base-1qi7c)

Found while landing ranger-base-r3qz6 (ADR 0093 D3/D4), filed 2026-11-14,
landed 2026-11-21. `examples/config.yaml` is what `posse init` copies into
every fresh instance — the de facto instance spec — and it named no key of the
`pulse_*` family at all. Four keys with teeth, none of them findable by
anyone reading the surface:

| key | what it does | documented as |
|---|---|---|
| `pulse_interval:` | the ARM SWITCH — presence arms the ticker inside `dispatch --watch` (ADR 0027) | `2m`, the ADR's own "typically 2m" |
| `pulse_persona:` | delivery target, else `coordinator:` | `<persona name>`, a placeholder |
| `pulse_renag:` | repeat interval for an UNCHANGED condition set | `30m` = `DefaultPulseRenag` |
| `pulse_feed:` | ADR 0093 D2's exit hatch: off = no feed, no waiter probe, no typed reminder, no `pulse-unarmed` | `on`, the default |

The gap predated r3qz6 — every other `pulse_*` key was equally absent when
`pulse_feed:` was added to them — which is the argument for deriving the
family rather than listing it. A list is how four keys went missing one at a
time with nothing anywhere holding a count.

## Documenting a default means pinning it, and that cost one production reader

The seed's documented-default census (`TestSeedConfigDocumentedDuration
DefaultsAreTheConstants`, ranger-base-vofbl) reaches a key through one of two
shapes: a call-site reader (`(key string, def time.Duration, errw io.Writer)`)
or a BODY-FORM reader (`(errw io.Writer) time.Duration` spelling one key
literal and returning one `Default`-shaped constant). `pulse_renag:` was read
inline in `LoadPulseConfig`, which answers a `PulseConfig` and an error, so
the pairing was outside every shape that census can see. Its three ways out
are not interchangeable, and the pin's own message says so: the register row
that would have admitted the line says "there is no constant for the line to
drift from", and for this key that is false — `DefaultPulseRenag` is right
there. So the key got a reader instead:

```go
func (a *App) PulseRenag(errw io.Writer) time.Duration
```

`LoadPulseConfig` takes an `errw` now and calls it; `pulse_interval:` and
`pulse_feed:` stay where they were. The census COMPARES the documented 30m
through that reader as of this landing (`pulse_renag: documented "30m"
resolves to 30m0s = DefaultPulseRenag`), and the floor in that pin moved from
ten compared keys to eleven.

`pulse_interval:` is the register's honest case and has a row of its own: its
presence IS the arm switch, so there is no default for the documented 2m to be
a statement about — the same row `autostart_interval:` and `backup_interval:`
carry, for the same reason.

**A side effect worth stating, because it changes behaviour:** a
`pulse_renag:` that is not an interval used to be swallowed
(`if d, err := ParseInterval(v); err == nil`) and the shop ran at 30m with
nothing said. It is named on stderr now, with the default standing — which is
what every other tunable with a default does, and what the seed's new line
implies. The arm switch stays the one key in the family that is an ERROR and
not a diagnostic: a ticker whose interval does not parse has no behaviour to
fall back on.

## The pin derives the family from pulse.go, and needs no door

`TestPulseFamilyIsDocumentedInTheSeed` (internal/posse/pulseseed_test.go,
arm 1) is `TestDispatchEpochIsDocumented`'s shape — "a key with teeth that is
not documented there is a key nobody can find" — over a DERIVED family: every
`pulse_`-prefixed literal `pulse.go` hands `CfgGet`/`YamlGet`/`yamlHasKey`, at
each reader's own key position. Per key it holds that the seed documents it on
exactly one COMMENTED line, that the key is not armed, and one claim about the
value: `pulse_interval:` parses through `ParseInterval`, `pulse_renag:`
resolves to the constant through `PulseRenag`, `pulse_feed:` is not read as
the hatch by `pulseFeedOff`, `pulse_persona:` is a placeholder. The switch has
no default arm, so a fifth key is a failure until somebody says what its
documented line claims.

It reads ONE FILE, by name, which is deliberate twice over: reading a named
file is not an enumeration, so this pin reds for the bead that edits pulse.go
and for nobody else, and it is therefore outside the tree-wide class that
needs a Makefile door (internal/treepins/treewidedoor_qa_test.go). The price
is that a `pulse_*` key read from some other file escapes it. Every such read
is in pulse.go today — `LoadPulseConfig` and `pulsePersona` are the whole of
it — and the floor in `pulseConfigKeys` (at least four) reds if they move.

MEASURED 2026-11-21 on this box (linux, 16 cores), both planted over saved
copies and both restored byte-identical: the seed-side arms red one at a time
— the `pulse_feed:` line deleted, `pulse_feed: off`, `pulse_renag: 45m`,
`pulse_interval: 2min`, `pulse_persona: monica`, and `pulse_interval:` armed
live (which also reds `TestSeedConfigArmsNothing`'s new absence check) — and
the derivation widens: a fifth `pulse_*` read planted in pulse.go reds as
undocumented, and documented-with-no-claim reds on the switch's default arm.

## Not closed here

The general case is one census nothing in this tree performs: every config key
the harness reads is documented in the seed or registered with a reason. This
bead's pin covers one family in one file; the next undocumented key with teeth
will be in some other family, and nothing will say so.

- ranger-base-w9t63
