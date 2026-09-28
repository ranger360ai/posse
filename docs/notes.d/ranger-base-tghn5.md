# ranger-base-tghn5 — the keychain read names an ACCOUNT

posse's darwin meter read asked the keychain for a **service** and nothing
else: the generic-password lookup carried the service flag and the
print-the-password flag, and no account flag at all.

A generic password is identified by service **and** account. `security`
answers *a* password matching the attributes it was given, so on a box
holding that service under more than one account, posse read whichever row
the keychain reached first — which need not be the row the runtime wrote.
ADR 0019 D2 is "the runtime's own composite store, mirrored"; this was a
place the mirror was incomplete, and V12's rule — the name posse asks for is
the name the read must ask for — applies to the account exactly as it applies
to the item.

## MEASURED 2026-09-28 — the runtime's own rule

Read with `strings` over the shipped darwin-arm64 bundles 2.1.268 / 2.1.278 /
2.1.280 / 2.1.283 in `~/.local/share/claude/versions` (first read under
ranger-base-58qr8, re-read here before transcribing). That is a read of the
**program**, not of any store: no keychain item was opened, no credential
value was seen, and `security` was never run.

Both the runtime's read and its write pass an **account** ahead of the
service, identically in all four bundles, and the service is the item
`keychainItem` already derives. The account is a small function of the
environment, and `keychainAccount`'s header in
`internal/posse/credential.go` carries the bundle's own expression verbatim,
where a reader comparing posse's derivation against the runtime's will want
it — this page restates the rule rather than excerpting the store's access
line a second time (ADR 0024 D3).

The rule: `USER`, else the OS username, else the literal `claude-code-user`
when the value carries a character outside `[a-zA-Z0-9._-]`. Three details, each
one a way a transcription can be wrong, and each with its own arm in
`TestTheKeychainAccountFollowsTheRuntimesOwnRule`:

- `process.env.USER || …` is a **truthiness** test. An empty `USER` falls
  through to the OS username exactly as an unset one does. This is the mirror
  image of `CLAUDE_SECURESTORAGE_CONFIG_DIR`, where an empty value **is** a
  presence and shadows the variable beside it — the two arms of the store's
  identity read their environments by opposite rules.
- The rejection is a **rejection**, not a repair: a value with a space, a
  slash or a non-ASCII letter is replaced by the literal, not cleaned. Both
  sides then ask for the same literal, so that arm cannot disagree.
- `u()` is node's `os.userInfo`, which is `getpwuid`, and so is Go's
  `user.Current` on darwin. Where Go's lookup fails and node's does not, the
  runtime's `catch` answers the literal and so does `keychainAccount` — the
  same shape when they are wrong.

`keychainAccount` (internal/posse/credential.go) is the one derivation, the
way `keychainItem` is the one derivation of the item.

## The third cause of a 44

Naming the account **adds** a failure mode, and not naming it would be the
misdiagnosis this file is about. Before this change `security` exiting 44
was two facts wearing one exit code (the item is gone; this binary's ACL was
dropped). It is now three: the item may be sitting right there **under a
different account**. `keychainFallbackFix` says so, and the subject of every
sentence this store produces — the store's name, the seam's `Source`, the
unreadable line, the report's row — is now `keychain item "…" (account "…")`,
so the comparison is one the operator makes in Keychain Access without
running anything.

## What is verified by reading, and what is not

Verified by reading and pinned: the derivation, its fallback, its regexp
rejection, the argv the store actually builds, and that every sentence names
both halves.

**Not** verified by reading, and it cannot be: whether the item on a given
box carries that account. That needs a logged-in box, and every crew PID
denies `Bash(security:*)`, so it is the operator's one act:

    RHQ_LIVE_KEYCHAIN=1 go test -tags posse_arm3 ./internal/posse \
      -run TestLiveKeychainReadAnswersUnderTheRuntimesAccount -v

It skips unset, it skips off darwin, it never looks at the value it reads,
and it logs only the subject it asked for and the envelope's expiry. On a
box whose credential has been cleared (the ranger-base-58qr8 tombstone) it
fails with "holds no credential … run `claude` and `/login`", which is a
login state and not an account mismatch — the sentence distinguishes them.
