# A deny no grant overlaps renders no mount, so a host rename cannot take it off (ranger-base-166va)

2026-11-13. wall-watch killed a bwrap seat twice in an hour over
`~/src/posse/.git/config`. Neither kill was a seat write: the dispatcher's
`git worktree add` and the reap after it rewrite that file by lock-and-rename,
and a rename over a mounted name detaches the mount in every namespace
(ranger-base-0qpsw). The seat was in a linked worktree, so its grants in the
common dir were `objects`, `logs`, its own `worktrees/<name>` and its ref
directory. Nothing granted the common dir's top. The read-only base
(`--ro-bind / /`) already refused every write to `config`, and it still did
after the detach. The deny's own `--ro-bind config config` added nothing
except a mount for a host rename to remove.

**The rule now.** In `bwrapArgsInto`, an existing Deny that no write grant
contains and that contains no write grant renders no mount and is not
watched. That is the private-member arm's rule, applied to this arm. Both
directions are checked. A deny that contains a grant still renders, because
its mount lands on top of the grant and closes it, so dropping it would widen
the wall. Masked denies and read masks are unchanged: they hide what is read,
which the read-only base does not do.

**What stays a kill.** A deny inside a write grant is still mounted and
watched. For example, a seat whose grant is the whole `.git` of a main
checkout. There the detach really does hand the seat a writable host file,
so the kill is the right response. Whether that seat shape should stay
exposed to every host-side `git config` write is ranger-base-nk45q.

Pinned by `TestBwrapArgsRendersNothingForAnExistingDenyNoGrantOverlaps`
(internal/posse/bwrapargs_qa_test.go), mutation-checked in both halves.
