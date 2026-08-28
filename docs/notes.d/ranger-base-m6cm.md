## path 3 regenerates, so deleting it was never the control (ranger-base-m6cm)

ADR 0019 counts two credential stores. There is a third — a file in the Claude
Code config directory — and `ranger-base-zzc` closed it by having the operator
delete the file (`ranger-base-66y`). Two independent verifies were clean at
03:05 and 03:40 on 2026-08-26. A **new** file was created at **11:47:07 the same
day**, 8h06m later, and nothing on this box noticed for two days.

It was not the resurrected old one: the deleted file was 1114 bytes, this one is
994, with `btime == mtime == ctime == 11:47:07`. And it is already path 3 again
by the discriminator that classified the first: Claude Code rewrites its
credential file on every token refresh, and this one's content mtime has not
moved while `history.jsonl`, `sessions/` and `projects/` in the same directory
were written continuously for two days.

**The generalisable half.** A one-shot remediation of a *self-regenerating*
condition is not a control, and the bead's own description said the file
regenerates on file-auth login flows. The close was accurate at close time and
false eight hours later. Where a deliverable is a **state** rather than a
change, the close has a shelf life: the durable artifact has to be the thing
that re-measures it, not the measurement.

**What landed** (2026-08-28). `scripts/verify-credential-paths.sh`, wired as
`make verify-credential-paths`, prescribed in `docs/runbooks/credential-rotation.md`,
pinned by `credentialpaths_qa_test.go`. It scans `$CLAUDE_CONFIG_DIR` (when set)
and `~/.claude` at depth 1 for `.credentials.json*`, printing mode/size/btime/mtime
and no content. Exit 1 is a finding; deletion stays the operator's.

Three arms of it are the design, not decoration:

- **The matcher is a glob.** On 2026-08-23 the file was renamed, not removed —
  `.credentials.json.stale-20260823`, same bytes, same mode 600. A rename
  changes the name, not the exposure. ADR 0019 D5 line 201 still words the check
  as the exact path and therefore passes on a box where the credential is
  sitting right beside it (`rangerhq-m10j` owns reconciling that).
- **Exit 2 when no config dir is present.** "No findings" over a directory that
  does not exist is a pass earned by measuring nothing — the negative-control
  trap. The script refuses to call that clean, and the QA arm mutation-checks it.
- **`CLAUDE_CONFIG_DIR` cannot hide a finding.** Both dirs are scanned when they
  differ, so setting the variable adds a place to look instead of moving the
  gaze.

**`atime` is not a read witness on this box.** Measured 2026-08-28: APFS on the
data volume does not advance access time on a read (wrote a file, read it,
`atime` unchanged). A credential file whose `atime` sits at creation time has
*not* been shown to be unread — the bead that opened this one read all 994 bytes
of the file six hours before its `atime` was quoted as evidence of the opposite.
Use content `mtime` against the directory's live siblings instead.

**The parking, which is the other half of the defect.** The only versioned home
for this check was `rangerhq-m10j`, dep-blocked on `rangerhq-q65q` — an operator
decision about the P1/403 fallback and the D7 park that has nothing to do with
credential-file hygiene. The check is four lines of `find`; whatever `q65q`
decides, it is the same check. It now ships on its own, and the runbook names
the sections still waiting on `q65q` as empty rather than holding the page
hostage to them. When a control is gated on an unrelated decision, split the
control out — the gate does not get to decide when the file stops regenerating.
