# A fourth commit-guard check: an open vuln-private id named with more than itself (ranger-base-ihp7y)

ADR 0024 D2 now has a check 4. AGENTS.md "Landing the plane" (operator
ruling 2026-11-11, ranger-base-lcuec) was prose with no wall: a public
commit message or notes fragment may describe the hole its fix CLOSES, but
a residual still OPEN under a `vuln-private` bead is named by bead id and
nothing else, until that residual's own fix lands. This bead asked whether
a fourth arm in the existing `prepare-commit-msg` hook could judge that
without becoming a lockout (ADR 0068), and left the open questions to
whoever took it.

## The four open questions, answered

- **Reaching the bd store at all, and what it does when it cannot.** Never
  from this shell. Every other check in this hook is pure POSIX sh, but
  none of them needs a LIVE answer from an external process with no bound
  on how long it takes — bd's own timeout is a Go-level mechanism
  (`Bd.Timeout`, `beads.go`) built on `exec.CommandContext`, and POSIX sh
  has no portable, orphan-free way to reproduce it (backgrounding a process
  and killing it from a sibling is exactly the "leaked gate-shell children"
  shape AGENTS.md already names as a recurring defect here). So the judge
  is a `posse` subcommand (`posse gates vuln-private-scan`), resolved by
  `command -v posse` at COMMIT time — deliberately unlike the identity
  literals, which are frozen at install time because there is a render-time
  stamp to freeze them into; a bead's open/closed status has no such
  stamp. Absent `bd`, absent `posse`, bd erroring, or no open
  `vuln-private` bead at all: every one fails OPEN (ADR 0068's own test —
  a refusal over "could not find out" has no remedy the committer can
  perform).
- **What counts as "descriptive text" after an id, without reading every
  "see also" line the way a human would.** Strip every open vuln-private id
  present on the line, then strip an allowed decoration set (whitespace, a
  list bullet or numbering, backticks, bold markers, the punctuation that
  separates two bare ids on one bullet) — anything left is descriptive.
  Deliberately strict: a letter is never in the decoration set, so "see
  also `<id>`" is flagged exactly like a full paragraph would be. Two bare
  ids sharing one bullet (`- a, b`, the shape ranger-base-lcuec.md's own
  "Not closed here" uses) read as bare for both.
- **Added lines in staged files, or only the message.** Both: the same
  SHIPPED markdown pathspecs check 2 already scans (every path, not only
  `docs/notes.d/*.md` — the rule's own words are "a commit message or
  notes fragment", and NOTES.md, a README or an ADR is exactly as public),
  plus the commit message, through the SAME reader check 3's message arm
  already uses (`identityGuardCheck`'s new `extraMessage` parameter) —
  never a second call to `messageArm`, which would open a second copy of
  its reference-diff reader and trip `TestQAHookReadersAllDisableMoveDetection`'s
  "exactly one" count. Caught that one by running the untouched suite
  after wiring it in, not by reasoning about it in advance.
- **Class-only, like its siblings.** Yes: the refusal names the flagged
  bead id (safe — the id is already as visible as the bd store itself) and
  never the line or the description. The description is the one thing this
  check exists to keep out of a terminal and `refusals.log` alike; printing
  it to explain the refusal would partially recreate the leak.

## What changed

- `internal/posse/vulnprivate.go` (new): `vulnPrivateHits` (the pure
  bareness test), `RunGatesVulnPrivateScan` (reads stdin, queries
  `Bd.OpenLabeledAny(dir, "vuln-private")`, fails open), and the rule/remedy
  text constants.
- `internal/posse/gates.go`: `vulnPrivateSource`/`vulnPrivateChecks`/
  `vulnPrivateContentCheck` render check 4's two arms; `identityGuardCheck`
  gained an `extraMessage ...visScanSource` parameter so check 4's message
  arm folds into check 3's existing reader instead of opening a new one.
  `commitGuardHead`'s own banner and `checkThreeMessageHead` updated to
  say so.
- `cmd/posse/main.go`: `posse gates vuln-private-scan` wired in (stdin in,
  one line per flagged id out, exit 1 iff anything was printed) and
  documented in the `--help` catalog.
- `docs/adr/0024-work-product-routing.md`: Decision 2 is now "four checks";
  check 4 is written up where checks 1-3 are, with the Go-vs-shell
  reasoning restated there for a reader who never opens vulnprivate.go.
- Two existing pins updated for the new unconditional message arm, not
  worked around: `TestQADataCeilingRendersAboveTheGateUnderEveryStamp`
  (dataceiling_qa_test.go) now expects check 4's message arm to render even
  with nothing else configured, since a vuln-private residual is a live
  fact about this repo's open beads and not about derived identity
  literals; `TestAParentVerbAnswersWithEveryEntryUnderIt` (cmd/posse) and
  `TestCatalogNamesEverySubVerbTheSwitchAccepts` (cmd/posse) hand-updated
  for the new catalog entry, the way their own comments say to.

## Verified

- `internal/posse/vulnprivate_test.go`: the bareness test (bare id in every
  decoration shape is clean; "see also", "stays open", a trailing
  description are flagged; an id not in the open-vuln-private set is
  ignored; a closed residual is not flagged) and `RunGatesVulnPrivateScan`
  end to end against the fake-bd harness (`fakeBinFor`, `fake-list-labeled.json`)
  for the flagged/bare/closed/no-bd/empty-stdin shapes.
- `go build ./...`, `go vet ./internal/posse/... ./cmd/...` clean.
- `make fmt-check` clean.
- End-to-end smoke test against this instance's own live store, in a
  scratch repo with the real hook installed by `posse gates install-hooks`:
  staging a `notes.md` naming a currently open `vuln-private` bead with a
  description refused the commit under this check's wording; the bare id
  alone committed clean; once the bead in question was closed by another
  seat mid-test (this instance's beads store is shared and live), the same
  text stopped being flagged — the rule applied correctly in both
  directions without being told to.
- `go test ./internal/posse/... ./cmd/posse/...` (arm 1, both packages)
  green except `TestShippedTreeNamesRolesNotThisCrew` — pre-existing,
  confirmed on a clean stash of this tree before any of this bead's edits,
  already tracked and CLOSED on ranger-base-pfh1c, apparently regressed
  since: left for whoever owns that bead, not re-filed and not touched
  here (out of this bead's own stated scope).
- `make crew-check` reds on that same pre-existing failure and nothing of
  this bead's own.

## Not closed here

- ranger-base-pfh1c reads CLOSED but `cmd/coldexec/supervisor_linux_test.go`
  still names a crew persona and `make crew-check` still reds on it —
  unrelated to this bead, not re-filed, noted on ranger-base-ihp7y's own
  bead instead since the fix (or its regression) belongs to whoever owns
  that file.
