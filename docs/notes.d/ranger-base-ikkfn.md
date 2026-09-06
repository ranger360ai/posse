## A phrase scan over `posse runtime check` output must unwrap it first (ranger-base-ikkfn)

`wrapGrid` (`internal/posse/runtimecheck.go:39`) wraps every grid row at a
**fixed** 78 columns — deliberately not the terminal's width, so *the same
box always renders the same bytes*. That determinism is what made this trap
convincing: nothing here is flaky. What moves is the **wrap point**, because
it is a function of the row's content, and the trust rows carry the session
dir. So one assertion reads two ways:

```
      key: projects[...].hasTrustDialogAccepted ... — silenced:
      <checkout>/cmd/posse is already trusted
      in
      /var/folders/.../003/.claude.json
```

`strings.Contains(out, "is already trusted in")` is **false** against that
output. In a dispatched seat's worktree — a longer path, a different break —
it is true. `TestClaudeConfigDirIsFenced` (ranger-base-scts5) was written and
run in a seat, landed on main, and its first CI run was already red; it stayed
red for 12 runs and failed identically in the bare checkout on this box.

**The rule.** Match phrases against `strings.Join(strings.Fields(out), " ")`.
`wrapGrid` never breaks a *word* — it only inserts a newline and an indent
*between* words — so collapsing whitespace reconstructs exactly the string the
producer built. A single word (a path) survives a raw `Contains` and needs no
help; anything with a space in it does.

**The negative assertion is the dangerous half**, and it is the half nobody
writes a red for. `if strings.Contains(out, live) { t.Error(...) }` says *the
fence held*. A leak whose row happened to wrap satisfies it — the phrase is
there, the scan cannot see it, and the pin reports absence. Collapsing
whitespace therefore **tightens** that arm; it is not a loosening dressed up.
Unmeasured on this box: the leak mutant's live row happens not to wrap here,
so both readings caught it. The blindness is measured in the other direction —
the same renderer, the same phrase, `Contains` false over output that contains
it — which is the born-red failure itself.

**Census, so this is not re-discovered.** The only other multi-word phrase
scan over grid output is `internal/posse/runtimeyamlv2_test.go:413`
(`"unattended flag --approve-all on the line"`, over `RuntimeCheck`'s buffer).
Left alone: its fixture runtime carries no operator path, so its wrap point
does not move with the checkout. Everything else that greps `runtime check`
output is matching usage text, a rendered command line, or a refusal error —
none of them wrapped by this function.

**One mutant that survived, and why it matters** (filed as ranger-base-api7c):
emptying `ClaudeConfigDirIn` to `filepath.Join(home, ".claude")` — the whole
`CLAUDE_CONFIG_DIR` rule deleted — leaves this pin **green**. `ClaudeConfigFile`
re-derives the same rule by hand for the `.claude.json` branch
(`trust.go:110-113`), and that is the branch this verb actually reads. A
mutant aimed at the consolidated function measures nothing here.
