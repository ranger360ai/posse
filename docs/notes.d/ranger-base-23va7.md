---
# ranger-base-23va7 — the §9 recipe never "reinstated the older block"; it duplicated

Commit 32f95f8's message says: "Running the documented recipe over a
reconciled AGENTS.md would reinstate the older block." The bead that
commit closed (ranger-base-wnsf) said the same thing: the recipe "would
therefore DELETE the current block and reinstate the older one." Neither
was true, before that commit or after it.

`git show 32f95f8^` shows the pre-fix heredoc already wrote a lowercase
`## Landing the plane` heading, and the pre-fix awk cut already matched
only `/^## Landing the Plane/` — capitalized, no fold. So the cut never
matched a heading the recipe's own heredoc had written. Running the
recipe over an already-reconciled `AGENTS.md`, before or after 32f95f8,
cut nothing and appended a second `## Landing the plane` section
alongside the first — a duplicate, not a replacement. Checked against a
repro on this box (darwin 25.4.0, ab80d05): two headings at distinct line
numbers, not one section overwriting another.

ranger-base-23va7 folds case in the awk match (`tolower($0) ~ /^## landing
the plane/`) so the cut now matches both the heading `bd init` plants and
the one this recipe appends. That makes "reinstates the older wording"
in INSTALL.md's paragraph after the recipe true for the first time —
worth noting here because the sentence predates the mechanism that makes
it correct.
