//go:build !race

package posse

import "time"

// fakeCallCost is the budget unit for one fake-herdr call: the parent's
// whole round trip through Herdr.capture — fork, exec, the child's runtime
// init, its work, and its exit — for a single `herdr …` the fake answers.
// The fake's OWN work is a rounding error in it (1-3ms measured), so the
// number is the cost of running a second process at all, and the -race
// build multiplies it by two orders of magnitude.
//
// That is why every fixture budget bounding work measured in fake-herdr
// CALLS is written as a multiple of this constant rather than in seconds.
// A budget spelled in seconds is a budget on how many calls the build has
// time for, and the two builds do not have the same answer: ten seconds is
// ~1200 calls without -race and ~10 with it. Three arm-2 tests were red
// under -race for exactly that reason and for no other — the dispatcher
// under them made the same 50 calls in the same order in both builds
// (ranger-base-0dt50).
//
// MEASURED 2026-09-10, darwin/arm64 (MacBookPro18,3, 8 cores), macOS
// 26.4.1, go1.26.5, `-tags posse_arm2 -count=1`, TestDispatchParallelPass
// run alone on a near-idle box, timestamping every fake-herdr process's
// own start:
//
//	                      median gap    one call    50 calls
//	go test               8.3ms         ~8ms        1.13s
//	go test -race         1.02s         ~1.02s      65.3s
//
// The gap between consecutive calls IS the per-call cost here, because the
// dispatcher issues them one at a time. The slowest single call without
// -race was 235ms (a `workspace create`, which the fake does real work
// for), and this constant is that worst case rather than the 8.3ms median:
// a budget unit has to hold on a loaded box, and every wall-clock margin
// in these fixtures that was set from a median has eventually been eaten
// by one (rangerhq-g6lx, rangerhq-3ig1). The -race value doubles its own
// measurement for the same reason.
//
// The values are picked so that every budget written in this unit comes
// out at the number the tree already carried in the ordinary build: this
// constant changes what `-race` allows, and nothing else.
const fakeCallCost = 250 * time.Millisecond
