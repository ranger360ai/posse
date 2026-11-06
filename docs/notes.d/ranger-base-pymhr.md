# The identity git invents on darwin and not on ubuntu (ranger-base-pymhr)

ci.yml went red on `main` at 47eabc3d
([run 42270221451](https://github.com/ranger360ai/posse/actions/runs/42270221451)),
one job of six: `test (ubuntu-latest, 1)`. macos-latest arm 1 was green over
the same commit, which is the platform split that workflow exists to show.

The failure is the in-test WITNESS of pin 9, landed the same day by
ranger-base-ava0x — `TestQAHookSkipNewlineForgedGlobalLineIsRefused` in
`internal/posse/hookskipglobal_qa_test.go:537`. The witness renders the
hook-skip shim with ADR 0079 D2's count mutated away and asserts that the
forged `global` line then compares EQUAL: the commit must land at **rc 0 with
the hook unfired**, which is the bypass the count refuses. On ubuntu it landed
at 128 instead, and never reached the compare:

```
WITNESS pin 9: GIT_CONFIG_GLOBAL=/dev/null, forged global line: … got code=128 fired=0
("Author identity unknown … fatal: empty ident name (for <runner@runnervm7aw5e…>) not allowed")
```

## The root cause

The fixture's committer identity lived in the fixture's **global** file
(`$HOME/.gitconfig`, written by `hsSetup` beside the managed hooksPath). That
leg's whole premise is `GIT_CONFIG_GLOBAL=/dev/null` — git reads no global
file at all — so it sheds the identity along with the hooksPath it means to
hide. Every other leg of these pins moves `HOME` or `XDG_CONFIG_HOME` and can
write a `[user]` block into the file git will still read; this one has no file
to write to.

What carried it until now is that git **invents** an identity when no config
scope names one, from the OS user record. MEASURED 2026-11-06, `git var
GIT_AUTHOR_IDENT` under `GIT_CONFIG_GLOBAL=/dev/null` with a scratch `HOME`:

- darwin 25.6.0, git 2.50.1 (Apple Git-155) — `David Stacy <…>`, rc 0, out of
  the account's gecos name.
- ubuntu-latest, from the run above — `fatal: empty ident name`, rc 128. The
  `runner` account has no gecos name, so there is nothing to invent from.

So the pin was never darwin-only on purpose; it rested on a fallback that one
of the two runners does not have. Nothing about the shim, the compare or D2's
count was wrong — the commit died before any of them ran.

## The fix

`hsEnv` (`internal/posse/hookskip_qa_test.go`) hands the identity over in the
**environment** — `GIT_AUTHOR_NAME`/`EMAIL` and `GIT_COMMITTER_NAME`/`EMAIL`,
the same `t` / `t@example.com` the fixture's global file carries — and sheds
any the running seat carries, beside the `GIT_CONFIG*`, `HOME` and
`XDG_CONFIG_HOME` it already shed. Env identity is scope-free, which is the
point: every config scope here is some leg's subject, so an identity in a file
is an identity a leg can hide. It is also the house pattern already in
`gates_test.go`, `notesguard_qa_test.go` and `silentrevert_qa_test.go`.

## Why the local run can now see it

`hsSetup` also sets `user.useConfigOnly = true` in the fixture repo's **local**
scope — which no leg hides, which `git commit` reads under all of them, and
which matches none of `hookSkipRegexp`, so it is in no compare and in no D2
count. It forbids the invention above. MEASURED 2026-11-06 on this box: with
the identity removed from `hsEnv` again, pin 9's witness reds on **darwin**
exactly as it did on ubuntu — same test, same line, `code=128 fired=0`,
differing only in which half git could not invent (`no email was given and
auto-detection is disabled` against ubuntu's `empty ident name`). With the
identity in place both legs are green.

That is the part worth keeping: before it, no run on either developer machine
could tell the fixed fixture from the broken one, and the next seat to drop
the identity would have learned it from a red `main` two pushes later.

## The class

Adjacent to ranger-base-loi28 ("a fixture that scrubs `HOME` no longer has an
empty git global scope") and the other half of it. That one is about a premise
— *no global config* — that stopped being buildable from `HOME`. This one is
about a fixture that successfully hides every scope and then needs something
out of one: **if a fixture hides the scopes git reads, the identity has to come
from the env, and `user.useConfigOnly` in a scope the fixture keeps is what
proves it does.** A fixture that relies on git's invention is green on darwin
and red on linux, and ci.yml is the only place that says so.
