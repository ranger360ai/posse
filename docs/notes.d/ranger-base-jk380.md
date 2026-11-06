# The wall's body was rendered in a file that never said `do not edit` (ranger-base-jk380)

MEASURED 2026-11-06 (hoover, installed 0.5.3+1f4da269 against main at
44c31e30): `posse gates` read "3 commit(s) behind main … 0 of them rendering a
wall" over e09l0, ap4p6 and c1pqo — three commits that changed only
`internal/posse/hookskip.go` (plus tests and notes), one of them closing a
hook-skip hole live on every installed seat.

hookskip.go renders the L1 git shim's hook-skip arm (`renderHookSkipArm`,
`renderHookSkipEnvCall`, …); `renderShim` in gates.go splices it in and writes
the header. internal/treepins derived WallRenderSources' class only from a
`do not edit` string literal, so a file rendering a wall's BODY was never in it.

Fix: hookskip.go is in `posse.WallRenderSources`, and
`wrsRenderSites` (internal/treepins/wallrendersources_qa_test.go) now also
derives BODY renderers: from every func declared in a headered file, follow
calls to `render*`-named callees (by name, func or method), transitively; every
file declaring one is in the class. Census today: that adds exactly
hookskip.go — the other `render*` declarers (agents.go, reachability.go,
refresh.go) are not reached from a headered file. Arm 1 now also asserts that
the list minus hookskip.go is missing exactly hookskip.go, so the derivation
cannot silently stop seeing the file it exists for.

Residual, still a floor: a helper spliced into a wall whose name is not
`render*` (a quoting helper, say) is not derived.
