# The commit wall's every-repo class was a criterion enumerated for one runtime (ranger-base-t03st)

ranger-base-ihda8 found the commit wall's every-repo arm refusing claude's two
settings files and landing every other built-in project-config entry. The
criterion was never claude-only: ADR 0015 §3's "Third" paragraph and
`constitutionGuardBody`'s own header both said `.claude/settings*.json` is
"not this ADR's promoted set, but the same class by blast radius". The
criterion was right and the **enumeration** was a hand literal written when
claude was the only runtime. This bead derives the enumeration from the
built-in runtimes' own `Runtime.ProjectConfig` declarations
(`ConstitutionEveryRepoPaths`, `internal/posse/promote.go`), read by the hook
render and the land belt both.

## What the class gained — MEASURED 2026-10-31, from the shipped runtime table

Sixteen entries where there were two. `.claude/settings.json` and
`.claude/settings.local.json` were already in; the other fourteen were not:

    .bob/custom_modes.yaml   .claude/plugins               .grok/config.toml
    .bob/hooks               .codex/config.toml            .grok/hooks
    .bob/mcp.json            .cursor/hooks.json            .grok/lsp.json
    .bob/plugins             .cursor/mcp.json              .grok/plugins
    .bob/settings            .mcp.json

## What the widening cost — MEASURED 2026-10-31, independently of ihda8's census

Every git repo in the operator's source tree, every one of the sixteen paths,
`git ls-files`: **two hits, both `.claude/settings.json`** — this repo and one
other — and nothing tracking any of the fourteen newly covered paths in any
repo. Both hits were already in the class before this bead, so the widening
refuses **no commit shape that exists on this instance today**. That is the
whole argument for doing it now rather than behind a flag.

## Three decisions, each a place the obvious reading is wrong

**Claude's two files are still named explicitly inside the derived union.**
ranger-base-az93's reason for them is the deny list, which is *permissions*,
and the launch check's exec-channel key narrowing deliberately leaves
`permissions` out (`grokProjectConfigKeys`: posse's own
`.claude/settings.json` is permissions-only and must stay a clean grok launch
for every crew seat in this repo). Two different reasons for the same two
paths. Today the union already contains them; naming them keeps the result
identical if claude's declaration ever changes.

**Built-ins only, not the `runtimes/<name>.yaml` overlay** (ADR 0021), and not
a template runtime's `project_config:`. The hook is rendered at install and
the land belt runs at close, and an overlay can differ between those two
moments — which breaks the one property the shared function exists for, two
readers and one list. ASSUMED acceptable: a template runtime's declared
channel stays covered where it is read, at the launch check
(`ProjectConfigTrust`).

**Matched by path, never by key.** The wall keeps its exact-or-directory-prefix
rule. `ProjectConfigKeys`' narrowing is a launch concern: a permissions-only
edit to `.claude/settings.json` is already refused on az93's terms, and the
rendered `sh` cannot parse JSON keys faithfully. So a path leaving
`ProjectConfigKeys` narrows the launch check and does not narrow the wall.

## Where the pins went, and why not into the behavioural matrix

`constitutionClassSpec` stays the HAND-SPELLED half — ADR 0015 §1's promoted
set plus az93's two files, the paths somebody decided one at a time — and the
behavioural matrices keep iterating it. The derived half is pinned twice
instead:

- `TestQAConstitutionClassIsTheSpecifiedSet` now reads `ConstitutionClassIn`
  and compares it, as a set, against two independent sources:
  `constitutionClassSpec` and the runtime table's own `ProjectConfig` fields.
  It still fails in both directions.
- `TestQAConstitutionWallRenderNamesEveryBuiltinProjectConfig` walks
  `builtinRuntimes` — not `ConstitutionEveryRepoPaths`, which is the function
  under test — and requires each declared path to be in the class of a plain
  repo AND of the constitution repo AND a whole line of the rendered hook.
- `TestQAConstitutionWallRefusesAnotherRuntimesProjectConfig` is the behaviour,
  over ihda8's own two shapes: `.grok/hooks/x.json` (directory prefix) and
  `.mcp.json` (exact path), in a repo with NO constitution marker, refused
  under `RHQ_PERSONA` and landing without it.

A sixteen-member cross product of git fixtures would have said the same thing
sixteen times: the behavioural matrix costs ~0.43s per subtest (MEASURED, 23
subtests in 10.0s) and lives in suite arm 3.

MUTATION-CHECKED 2026-10-31, each reverted separately and restored:

| mutation | reds |
|---|---|
| `ConstitutionClassIn` back to the two-file literal | the set pin, and the class half of all 14 new render subtests |
| the rendered class back to the two-file literal | the render half of those subtests, and both behavioural refusals |

Neither mutation reds the other's pins, which is the separation wanted: one
is the Go belt, one is the rendered sh.

## Residuals — unchanged, and still L3

`env -i` scrubs `RHQ_PERSONA` and the arm stands down; `core.hooksPath`
re-points the slot. The land belt (`constitutionOnBranch`,
`internal/posse/worktree.go`) reads the same `ConstitutionClassIn`, runs
operator-side and cannot be env-scrubbed by the session it judges.
