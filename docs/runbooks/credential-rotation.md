# Credential rotation

*ADR 0019 · the parts that are decided. This page is deliberately partial:
everything the operator's `rangerhq-q65q` decision governs is named below and
left empty, because the one section that does NOT depend on it was stuck behind
it for five days and a credential file regenerated in that window.*

## The check that does not wait on anything

```sh
make verify-credential-paths        # scripts/verify-credential-paths.sh
```

Run it at every rotation, and after any `/login`, `claude setup-token`,
`make install`, or keychain prompt. Exit 0 is clean, **1 is a finding**, 2 means
no config directory was present so nothing was measured — a 2 is not a pass.

It scans `$CLAUDE_CONFIG_DIR` (when set) and `~/.claude`, depth 1, for anything
matching `.credentials.json*`, and prints mode, size, birth time and content
mtime. It reads no content, deletes nothing, and never runs `security`.

**Anything printed is a finding.** ADR 0019 counts two credential stores; a file
here is a third. `~/.claude` is in every runtime's writable set
(`internal/rhq/seatbelt.go`) and the rendered seatbelt profile's only deny is
`file-write*` — `grep -n file-read` over that file returns nothing — so every
same-user persona session below the container tier can read whatever sits there.

### Why it is a glob and not the path

On 2026-08-23 the file was **renamed, not removed**:
`.credentials.json.stale-20260823`, same bytes, same directory, same mode 600. A
rename changes the name, not the exposure. ADR 0019 D5 line 201 still words the
check as the exact path and therefore passes on a box where the credential is
sitting right next to it under a `stale-*` name; reconciling the ADR to the glob
is `rangerhq-m10j`.

### What to do with a finding

1. **File the ask; do not delete it yourself.** Removing a live credential is
   the operator's call every time — that is how `ranger-base-66y` was handled.
2. **Deleting the file does not revoke the grant.** It closes the local
   file-read exposure only. Killing the grant at Anthropic is `/logout` +
   `/login`, which re-touches the keychain item the guard depends on and makes
   any `CLAUDE_CODE_OAUTH_TOKEN` in `envs/` stale (ADR 0019 D7). Two steps, and
   the second one is the operator's hand.
3. **Do not treat the delete as the control.** The file's defining property is
   that it regenerates on file-auth login flows. Measured: the operator deleted
   it at 2026-08-26 03:40 and a new 994-byte file was created at 11:47:07 the
   same day — 8h06m later — and nothing on this box noticed for two days
   (`ranger-base-m6cm`). Deleting it schedules the next run of this check; it
   does not end it.

### Reading the metadata it prints

`btime` separates a new file from a resurrected one — the deleted file was 1114
bytes, the regenerated one 994.

A **frozen content mtime while claude has been running daily** means this file
is not the active store: Claude Code rewrites its credential file on every token
refresh. Compare it against `history.jsonl`, `sessions/` and `projects/` in the
same directory, which move constantly.

**`atime` is not evidence of anything here.** Measured 2026-08-28 on this box's
APFS data volume: reading a file does not advance its access time. The script
does not print it, and "opened once, never since" cannot be read off it.

### Siblings

`~/.codex/auth.json` and `~/.grok/auth.json` are deliberately out of scope. For
those runtimes the file **is** the store, not a leftover, so the same matcher
would print a finding that is not one. They belong to `rangerhq-m10j` when their
lanes reach the cage tier.

---

## Parked on rangerhq-q65q — owned by rangerhq-m10j

These sections are empty on purpose. Writing them requires the operator's
decision on the P1/403 fallback and the D7 park; the section above requires
none of it, which is why it is not waiting with them.

- **Cage: mint and place the setup-token.** (`claude setup-token`,
  `envs/container.env`, mode 600, `# minted YYYY-MM-DD`.)
- **Guard: refresh-or-ACL, and the 403 arm.** 401 → run `claude` once to
  refresh. Unreadable → re-grant this binary's keychain ACL (usually caused by
  `make install`). 403 → a setup-token was pointed at `/api/oauth/usage`; stop,
  do not refresh. The P2 keychain re-lock is **withdrawn** by the 2026-08-24 ADR
  amendment — do not restore a step telling the operator to revoke an ACL the
  guard still needs.
- **After `/login`: the stale env token.** ADR 0019 D7. Operator's hand.

When `q65q` lands, `m10j` fills these in and reconciles ADR 0019 D5 to the glob
above. Nothing in this page changes when it does.
