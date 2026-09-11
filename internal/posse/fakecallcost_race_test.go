//go:build race

package posse

import "time"

// fakeCallCost under -race — see the sibling declaration in
// fakecallcost_test.go for what the unit is and how it was measured.
//
// MEASURED 2026-09-10 at 1.02s per call, against ~8ms in the ordinary
// build (~123x). Race instrumentation makes the fake's own work no slower
// — it stays 1-3ms — so the whole difference is in running the process:
// the test binary re-execs ITSELF as the fake (herdr_test.go), so the
// child is the race-instrumented binary too, and pays the instrumented
// startup and teardown once per `herdr …` call.
//
// Doubled from the measurement, like its sibling, for load.
const fakeCallCost = 2 * time.Second
