//go:build !darwin

package posse

// kernelPath has no reading of its own off darwin, and the error is how this
// file says so rather than a second-best answer: ADR 0082 D1's non-darwin
// half IS canonicalPath's fallback — `EvalSymlinks` over the absolute,
// cleaned path, which is resolvedPath (visibility.go) — so anything written
// here would be a second implementation of it.
//
// ASSUMED for Linux (ADR 0082 D1), and stated as an assumption because
// nothing here measures it: case-sensitive volumes, which is the default and
// what ADR 0080's remote host runs. A casefolded ext4 directory keeps the
// caller's case, so two spellings of one such directory still render two
// walls there. The ADR names that a residual, not a promise.

import "errors"

var errNoKernelPath = errors.New("no kernel path reader on this platform")

func kernelPath(string) (string, error) { return "", errNoKernelPath }
