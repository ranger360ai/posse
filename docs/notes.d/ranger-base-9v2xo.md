# Three measurements a verify round bought (ranger-base-9v2xo)

*2026-11-13, holden, verifying the closes of ranger-base-p2tf5,
ranger-base-3bwjr, ranger-base-2lls3 and ranger-base-nl2ha.*

Three facts from this round are worth keeping because each one is a question
that will be asked again, and each was cheaper to answer than to re-derive.

## 1. The shared PID namespace does NOT give a seat a second path to a masked file

ADR 0080 D17 rejects `--unshare-pid` on purpose — a seat kills its own pids and
`go run ./cmd/checkorphans` reads the box's process table — and `/proc` inside a
bwrap seat is the host's own `/proc`, not a fresh one (`--proc` is deliberately
absent too). That pairing invites an obvious attack on every mount-based wall
the tier has: a tmpfs or a `/dev/null` bind covers a path in the SEAT's mount
namespace, and `/proc/<a host pid>/root/<that path>` is a second name for the
same file in the HOST's.

**It is refused, and `--unshare-user` is what refuses it.** MEASURED
2026-11-13 on this box (Linux 7.2.15, bwrap, uid 1001), with a fixture file —
no real credential was read:

| reader | target | result |
|---|---|---|
| uncaged, same uid, live target | `/proc/<pid>/root/etc/hostname` | reads `stacarchy` |
| uncaged | `/proc/1/root/...` | `Permission denied` (pid 1 is root's) |
| inside `--ro-bind / / --unshare-user --tmpfs <dir>` | `<dir>/fixture_key` | `No such file or directory` (the tmpfs) |
| same seat | `/proc/<the host shell's pid>/root<dir>/fixture_key` | `Permission denied` |
| same seat | `/proc/1/root<dir>/fixture_key` | `Permission denied` |
| same seat, `--ro-bind /dev/null <file>` instead of a tmpfs | both names | `Permission denied` |

The first row is the positive control and it matters: without it the four
`Permission denied` rows are indistinguishable from a probe that could not
look. `ptrace_may_access` is the kernel check behind `/proc/<pid>/root`, and a
reader sitting in a user namespace the target does not belong to fails it even
at a matching uid. So the mount walls are not quietly defeated by the PID
namespace D17 leaves shared, and a future bead proposing `--unshare-pid` for
THIS reason can stop here. The script is
`scratchpad/proc-root-probe.sh`-shaped: a `mktemp -d`, a fixture, one `bwrap`,
and a control that reads.

## 2. `$GIT_DIR` is EMPTY in a `pre-commit` hook, so a marker hook must bake an absolute path

A rig that measures which hooks ran by writing to `"$GIT_DIR/../MARK"` from the
hook body measures nothing: on git 2.56 `GIT_DIR` is unset in the environment
`git commit` gives `pre-commit`, so the hook writes to `/../MARK`, fails with
`Permission denied` — and a hook body ending in `exit 0` swallows that, so the
leg scores as "no hook ran" and every refusal leg looks identical to every
allowed one. The first run of this round's `am` rig did exactly that, green,
across forty legs.

Write the marker hook with the repo path interpolated at creation
(`printf '#!/bin/sh\nprintf %%s " %s" >> %s/MARK\nexit 0\n' "$hook" "$repo"`),
and keep a leg whose only job is to prove the rig can SEE a hook: an ordinary
`git commit` must print `pre-commit prepare-commit-msg commit-msg`. This is the
"a check whose only signal is output passes by producing none" class with a
`git`-shaped lid on it.

## 3. `git --exec-path <path> <verb>` runs no verb, which is why the option table omits it

`globalValueOpts["git"]` (gates.go) deliberately leaves `--exec-path` out, with
a comment saying that bare, it prints and exits without consuming the next
word. MEASURED here, both arms: `git --exec-path /usr/lib/git-core am <mbox>`
prints `/usr/lib/git-core` and exits 0 through the shim AND through
`/usr/bin/git`, with the repo still at one commit — so it is the one leading-
option spelling of a denied verb that the shim does not refuse, and it is not a
hole, because no verb ran. `git --exec-path=/usr/lib/git-core am <mbox>` takes
the attached-value path, skips singly, and IS refused. Worth keeping because
the leg reads like an escape in a results table until you ask what it did.

## Not closed here

- ranger-base-k06gw
