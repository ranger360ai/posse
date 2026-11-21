//go:build !posse_arm2 && !posse_arm3

package posse

// wallunixseatreport_linux_test.go's twin on darwin, where internal/posse's
// suite also runs (wallunix_other.go's header) and where ci.yml has a leg.
// Nothing reaches this: the seat it reports for is a bwrap seat, and the one
// test that starts one skips off Linux (wuSigLiveSkip). It has to COMPILE and
// ANSWER all the same, and the answer is the honest one — there is no prctl
// and no pdeathsig here — rather than a zero, which in the report this feeds
// is the reading that means "the runner cleared it" (ranger-base-8m9d0).
//
// WHY A GOOS PAIR AND NOT AN `_other_test.go`. A `//go:build !linux` line
// would be a build line that is neither an arm expression nor a listed mode,
// so internal/treepins/armtags_qa_test.go's arm 1 would correctly red it as a
// file no arm `make test` runs. The GOOS lives in the NAME here, which that
// census represents for exactly the two platforms the matrix runs, and
// `make land-check`'s cross-GOOS vet covers the same two.
func wuSeatPdeathsig() string {
	return "<no prctl on darwin: PR_GET_PDEATHSIG is a Linux option and this seat is a Linux bwrap seat>"
}
