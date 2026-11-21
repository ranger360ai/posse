# Every config key the harness reads, censused against the seed (ranger-base-w9t63)

2026-11-21. Discovered from ranger-base-1qi7c, which documented the `pulse_*`
family and pinned it — for ONE family, derived from ONE named file. This is
the general case: `examples/config.yaml` is the de facto instance spec
(`posse init` copies it verbatim), and nothing censused it against the keys
the harness actually reads.

## What the first run found

Six keys with teeth that the seed named nowhere, in four families:

| key | what it does | answer |
|---|---|---|
| `auto_reap:` | on/off for the sweep that ENDS a finished session (default on) | documented |
| `reap_crew_after:` | grace before a closed bead's session you stepped into is reaped (4h) | documented |
| `reap_unpointed_after:` | grace for a per-bead-named session carrying no pointer (1h) | documented |
| `retire_tree_after:` | grace before a landed session worktree is retired (1h, ADR 0058) | documented |
| `load_guard_kill:` | the load guard's KILL arm, off by default | documented |
| `backup_max_age:` | when the newest archive becomes a condition | registered |

The three graces are the only thing between a live conversation and a closed
workspace, and the kill arm ends processes. None of them was findable by
anybody reading the surface.

`backup_max_age:` is the one registered rather than documented, and the reason
is another pin's ruling rather than this bead's judgement: ADR 0036 §6 makes
its default a RULE (2x a valid `backup_interval:`, falling back to
`DefaultBackupMaxAge` only on an instance with no schedule), so
`seedDurationDefaultNotAConstant`'s stronger half refuses ANY documented line
for it — a line and that row cannot both be right (ranger-base-s1dmh). The
seed does name it, in prose, in the backup block. The register row says so and
cites the ruling, which is what makes the row re-readable instead of a shrug.

The three documented graces then landed inside the existing documented-default
census for free: `graceAfter` was already in `seedDurationResolvers` and all
three constants were already in `seedDurationDefaults`, so each documented
value is now COMPARED against its constant through the production reader.
`compared` moved 11 → 14.

## The derivation, and where it stops

`internal/posse/seedkeycensus_test.go`, arm 1, behind `make seed-check`.

1. **The flat readers** come out of `yamlflat.go`'s own signatures — a
   top-level func whose first parameter is `path string` and whose second is a
   string. Six today; the `(lines []string, key string)` helpers beside them
   are not readers and the rule tells them apart without a list.
2. **Scope** is the PATH argument: a read of the instance config is one whose
   path expression is a `.ConfigPath` selector. Load-bearing — MEASURED
   2026-11-21, 97 of those six readers' 144 call sites in non-test Go hand
   them a runtime adapter file, a recipe, a session record, a launch lock,
   a verify box or a cage profile (47 name `.ConfigPath`; `CfgGet`'s own 54
   sites are the config by construction), and `detection:`, `command:`,
   `pid:`, `started:` are keys of those files. A census over all of them would need
   sixty rows saying "not the instance config", which is how a register stops
   being read.
3. **A shared reader is a reader**, as a fixpoint seeded with nothing:
   a function that hands one of its own string parameters to a config
   reader's key slot is itself a config reader at that parameter's index.
   `CfgGet` is derived like the rest, from its own
   `YamlGet(a.ConfigPath, key)` body. Five members —
   `CfgGet`, `attnAge`, `budgetDollars`, `graceAfter`, `planPercent` — and
   that is what makes `attn_parked_age:`, the three graces and the two
   `budget_*` keys visible at all: none of them is ever handed to a flat
   reader by name. The hop stops where it should: `declaredBy` and
   `runtimeScalarOrList` read `rt.Path`, so `prompt:` and `state_dir:` stay
   out. A func LITERAL assigned to an identifier counts too, resolved inside
   its own enclosing function and nowhere else — `OpsPatternSet`'s `read :=`
   closure is the only reader of `data_ceiling_patterns:` and
   `beads_visibility_patterns:`, and four other unrelated closures in the
   package are also called `read`.
4. **A key that is not a literal** is resolved where the resolution is exact:
   a package-level string constant (eight keys are spelled that way), and a
   `range` over a `[]string` of literals, inline or a package var —
   `backupKeys`, `BudgetCapsConfigured`'s inline pair and
   `retireExamplePIDs`' triple are the three such sets, and every member of
   all three has a reader of its own as well today, so what that rule buys is
   the next such list. The reader's own body is dropped, because part 3
   already walked its call sites. Four sites are left and each has a row in
   `seedKeyDynamicSite` naming the enumeration the keys really come from.

Totals, MEASURED 2026-11-21 on this box: 226 non-test `.go` files, 6 flat
readers, 5 config readers, 112 sites over 79 distinct keys, 4 dynamic sites.

**The prefix arm needed one correction, and it is the interesting one.**
A `YamlKeysWithPrefix` read (`plan_guard_`, ADR 0012 D4) has no literal member
to find, so the question asked is whether the seed documents a line of that
family. The commented `plan_guard_blind_max:` line shares the prefix and is
NOT a member —
it has a reader of its own — so with the two window examples deleted from the
seed the arm was GREEN (MEASURED 2026-11-21). The fix is derived rather than
listed: the pin subtracts the prefixed lines that are keys in the census's own
right. Both plantings red now.

## What is outside, on purpose

- **The other direction for COMMENTED keys.** A `# key:` line nothing reads is
  a documented key with no code behind it — `dirs:` one surface over
  (ranger-base-aox), whose ARMED half already has a pin. Not held here,
  because the seed's prose names retired keys deliberately
  (`plan_guard_overflow:` is discussed as removed) and telling those apart is
  a register this bead did not need.
- **A read that is not Go.** `plugin/*.sh` and the hook renders read config
  through awk.
- **A config path that is not a `.ConfigPath` selector.**
  `CageRealizeEntries(cfg, goos)` takes the path as a parameter, so
  `cage_realize_<goos>:` is in this census through neither route — and is
  documented in the seed anyway, with its own block. Widening part 2 to a
  parameter means an interprocedural rule over which paths reach which
  parameters. What makes the narrow rule honest instead is that the hole is
  PINNED: `TestConfigKeyCensusCanStillSayNo` plants exactly that shape and
  asserts the census does not see it, so closing the hole is a deliberate
  deletion of that row rather than a surprise.

## The mutation battery

Planted over saved copies and restored byte-identical (`git status` clean
afterwards). Control green first. MEASURED 2026-11-21:

| planted | arm |
|---|---|
| the seed's `# auto_reap: true` line deleted | RED, named the key |
| the two `plan_guard_<window>` examples deleted, `blind_max` left | RED (after the correction above; GREEN before it) |
| every `# plan_guard_` line deleted | RED |
| a `seedKeyUndocumented` row for a key the seed DOES document | RED |
| a `seedKeyUndocumented` row for a key nothing reads | RED |
| a `seedKeyUndocumented` row with an empty reason | RED |
| a `seedKeyDynamicSite` row renamed so it names no live site | RED, as an unregistered site |
| a `seedKeyDynamicSite` row for a site that does not exist | RED, as stale |
| a NEW undocumented `CfgGet` read planted in production Go | RED, named the key |
| a NEW undocumented key behind `graceAfter` — the parameter hop | RED, named the key |

Ten plantings, ten arms that fail, and the one that survived is written into
the pin as the reason the prefix rule subtracts.

## Not closed here

- ranger-base-w9t63 closes with the census and six keys. The other direction
  — a documented line nothing reads — is named above and filed nowhere; it
  needs a register for the seed's deliberately retired keys first, and nobody
  has asked for one.
