package posse

// Which posse rendered this wall (ranger-base-vso72).
//
// THE GAP. Every wall a seat launches behind is a RENDER: the seatbelt
// profile, the gate shims, the gate shell, the cage launcher and its argv
// note, the egress allowlist, the pane line. All of them are written fresh
// from the PID at launch — by the INSTALLED binary, which is a copy of some
// commit and not of main. So a gate fix that lands does not reach a seat
// when the launcher relaunches; it reaches a seat when a DESCENDANT of that
// fix is installed and then relaunches.
//
// Every header said the first half and none said the second:
//
//	;; posse seatbelt for bob — rendered from the PID at launch; do not edit
//
// names the PID and the launch, so a seat reading its own profile and not
// finding the deny it expects cannot tell "old wall, relaunch pending" from
// "old wall, INSTALL pending". Three closes on ranger-base-prjck said the
// fix "reaches a seat at its next launch"; twelve caged launches then
// rendered the old wall, because the installed binary predated the fix by
// 34 commits (ranger-base-mmhhc has that measurement).
//
// So every rendered header carries the renderer. It costs one clause and it
// turns an unanswerable question into a diff: the version in the header
// against `posse version`, and against what wallrenderer.go counts.
//
// WHERE THIS DELIBERATELY DOES NOT GO: the L3 hook bodies. Hook identity is
// byte-for-byte against this binary's own render (probeL3Hooks, ADR 0023),
// which already answers "does this wall carry THIS binary's render" for that
// one artifact — and a version inside the body would make every hook on the
// box read as degraded after any `make install`, including installs that
// changed nothing a hook renders. A stamp that cries wolf on every release
// is a stamp nobody reads. The hooks get item 3's sentence in the docs and
// wallrenderer.go's count instead.
//
// AND, SINCE ranger-base-g1jqt, A LIVE READING AT ONE OF THEM. The commit
// guard's refusals name the posse that judged them — read at refusal time
// from the binary on PATH (hookJudgeFunc), never rendered in. That is the
// other half of the paragraph above and not an exception to it: these bytes
// stay identical under every version, so the byte-for-byte identity above is
// untouched and no install degrades a hook that did not change. The
// exclusion is about a LITERAL in the body; the question a refused committer
// has — "is this refusal newer than the rule it cites?" — is answered by the
// only surface they are reading, at the only moment it is expensive, and
// ranger-base-li3up is the P2 filed against correct code because it was not.

// RenderedByPosse is the clause a rendered wall's header carries: which
// posse wrote these bytes. One writer, so every header spells it the same
// way and one grep over a box's state dir finds every wall a stale binary
// rendered.
func RenderedByPosse() string { return "rendered by posse " + VersionString() }
