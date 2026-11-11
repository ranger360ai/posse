# A fence of three over a class of four, and the variable that outranks every member of it (ranger-base-u3kft)

`make test` was red in all three arms for any seat whose launcher exports
`CLAUDE_SECURESTORAGE_CONFIG_DIR`, and green for CI and for every seat that
does not. 2026-11-11. One missing line in `TestMain`, two failing tests, and
a class whose membership was written down in one place and fenced in another.

## What was red

`internal/posse`'s `TestMain` replaces `$HOME` with a tempdir and clears the
CLI home overrides so that — in its own words — no locator "reads the
OPERATOR's live store when the variable survives into the test binary". It
named three: `CLAUDE_CONFIG_DIR`, `CODEX_HOME`, `GROK_HOME`.

`credentialDirNamed` (credential.go) honours a fourth,
`CLAUDE_SECURESTORAGE_CONFIG_DIR`, and reads it **first** — ahead of
`CLAUDE_CONFIG_DIR` and ahead of the temp `$HOME`. So on a seat that exports
it, every credential read in the package resolved the operator's own
directory, past the fence that exists to prevent exactly that.

Two tests failed on it:

- `TestOneEnvelopeReadsIdenticallyThroughBothStores` (credseam_test.go).
  `keychainItem()` suffixes the item name with a sha256 of the resolved
  directory when that directory is NAMED. The test produced its observed
  name while the variable was in force and its expected name after
  `credentialsHome` had unset it — two derivations, two environments, one
  comparison. credseam_test.go carries no build tag, so this red in arm 1,
  arm 2 and arm 3 alike.
- `TestTheReportRendersStructuralAbsenceAsSomethingToDo` (refresh_test.go,
  arm 2). A test that asserts this box has no login, which instead read a
  real store and reported a real expiry. That is the exposure half, held on
  the security lane as ranger-base-btroh.

`make test`'s arms are separate recipe lines, so make aborted the recipe and
arms 2 and 3 never ran for a seat that hit this — the same consequence
ranger-base-ltk6x was filed for, one day later and from a different cause.

Isolated, one variable at a time, `-count=1`, `-run` filtered (MEASURED
2026-11-11, linux 7.2.15, go1.27.3):

| environment | result |
|---|---|
| ambient, as the launcher sets it | FAIL |
| `env -u CLAUDE_CONFIG_DIR` | FAIL, unchanged |
| `env -u CLAUDE_SECURESTORAGE_CONFIG_DIR` | ok, 0.016s |
| that variable pointed at a scratch dir | FAIL, and the item suffix becomes that dir's own hash |

## The fix, and why it is not another `os.Setenv(…, "")`

The three lines already there write `""` on the stated grounds that "all
three resolvers treat empty as unset and fall back to the home". This
resolver does not. A present-but-empty `CLAUDE_SECURESTORAGE_CONFIG_DIR` is
a **third state**: it shadows `CLAUDE_CONFIG_DIR` and resolves to the home —
which is the behaviour credential.go documents and credseam_test.go's tables
already pin. Clearing it would therefore have fenced off the operator's
directory and left the variable itself in force, which is a different bug in
the same place. `os.Unsetenv` is the only spelling that takes it out of the
resolution.

No test in the package means to inherit the ambient value: every one that
cares either plants its own with `t.Setenv` or removes it with
`unsetenvForTest`, so unsetting it process-wide costs nothing.

## Why it needed a SOURCE pin and not only a runtime one

ci.yml exports no variable of this class, so a pin that reads the running
binary's environment is green on CI whatever the fence says — and it was
green on the fleet seats that happened not to carry it. ranger-base-7xvk5's
verifying seat measured this same arm at `ok internal/posse 102.566s` while
it was red here, and nothing in the intervening commits touched any of the
three files involved. A pin that can only fire where the accident already
happens is not a pin.

`credseamfence_qa_test.go` is therefore four arms, untagged so it builds
into every arm of the package:

1. **the running binary** — `credentialDirNamed()` answers inside the temp
   `$HOME` and names nothing; `keychainItem()` returns the unsuffixed
   service name; every variable of the class is out of force, and the one
   that needs it is UNSET rather than empty. This is the arm that reproduces
   the incident, and the one CI cannot run.
2. **TestMain's source** — the fence is parsed out of `herdr_test.go` with
   `go/ast` and held against the class, derived as `credentialDirVars` plus
   the two runtime homes not in it. A fifth name added to credential.go reds
   here on every box, before any seat exports it. This is the arm that holds
   on CI.
3. **which names are read by PRESENCE** — the judgement "this one needs
   `os.Unsetenv`" is about `credentialDirNamed`'s code, so it is held
   against that code: the resolver's `os.LookupEnv` reads are enumerated and
   each must be classified in `fenceNeedsUnset`. A sixth presence-read name
   cannot arrive unclassified and silently inherit the wrong fence shape.
4. a one-line guard that the two source arms' relative opens resolve, so a
   change in how the package's tests are run fails with a sentence rather
   than a parse error.

Both of the first two arms were verified to fire by weakening the fence —
to `os.Setenv(…, "")` and by deleting the line entirely — and the parse arm
names the variable that is missing rather than reporting a count.

## What this class actually is

The membership of "CLI home overrides" was written down twice and the two
lists disagreed. `credentialDirVars` (credential.go) enumerates the
credential locator's variables; `runtimehome.go` holds the three runtime
home constants, one of which is in the first list. `TestMain`'s fence was a
hand-kept third copy of the union, and a hand-kept copy of a union is where
a member goes missing. Arm 2 derives the union instead, so the fence has one
list behind it and no longer has an opinion of its own.
