package posse

// processNotices — where this package's UNOWNED notices go: the lines that
// belong to no single launch, pass or caller, and so have no writer handed
// down to them (ranger-base-wgzu7 finding 1).
//
// THE DEFECT, and it is the third slice of one. ranger-base-lcode gave one
// LAUNCH its own writer (NewSessionOpts.Warn) and ranger-base-ws20a gave the
// backend's non-launch lines the pass's (Dispatcher.RouteBackendWarnings).
// Both act on a writer the backend resolves. These five do not go through
// HerdrBackend at all — they are package state and per-value fields on types
// built fresh at a dozen call sites — so neither fix reached them and each
// still defaulted to the process's stderr, assigned NOWHERE in non-test
// code:
//
//	githang.go      gitHangw            a blown git deadline's one line.
//	                                    REACHED FROM A LAUNCH: planLaunch
//	                                    runs git through EnsureSessionTree,
//	                                    recordBead and PrepareSessionHead.
//	runtimeyaml.go  runtimeNoticeWriter LoadRuntime's dropped-key notice.
//	                                    REACHED FROM A LAUNCH: planLaunch
//	                                    loads the runtime.
//	beads.go        Bd.Hangw            a blown bd deadline (pass level).
//	herdr.go        Herdr.Hangw         a blown herdr deadline (pass level).
//	beads.go        noticeWriter        BeadsDirs' cwd-fallback notice, whose
//	                                    own comment says "the operator can
//	                                    see which repo served it".
//
// And a sixth the bead's census could not see, because that census walked
// each VAR's writers and this one takes the stream outright at the call
// site: EnvSetVars → TightenEnvPerms(os.Stderr) (envs.go), reached from
// planLaunch itself — the function whose head comment ranger-base-lcode
// left reading "Every line this function says goes to warn/warnw, never to
// b.warn or to os.Stderr". SecretVars → TightenSecretPerms is its declared
// parity twin (secrets.go's own header) and is routed with it; it has no
// non-test caller today, and a pair that disagreed about where its drift
// notices go would be the next census's finding.
//
// WHY IT IS A LOSS. RE-MEASURED 2026-10-03 on the loop then running: pid
// 22611, `posse dispatch --watch 3m --max-interval 3m -n 4 --resume`,
// `lsof -p 22611 -a -d 0,1,2` → fd 0, 1 AND 2 all on /dev/null. The loop's
// record is a file the loop opens and tees d.Out and d.Err into (watch.go),
// so a line that goes through neither is not in the record at all. Under
// the cockpit it is worse than discarded: the alt screen is up and a line
// written to stderr lands ON the frame being drawn and is gone with the
// next redraw (cmd/posse/cockpitnotices.go).
//
// NOT MEASURED, and said so: that a git, bd or herdr deadline actually blows
// during a dispatched launch. What is measured is the destination — each
// default, the absence of any non-test assignment, and the live loop's fd 2.
//
// THE SHAPE. One function the owning PROCESS calls once, because that is
// what these writers are: process state, with one consumer per process.
// `posse dispatch` points them at its pass's quiet err writer
// (Dispatcher.RouteProcessNotices, dispatch.go) and `posse cockpit` at the
// sink ranger-base-2vhqo built for exactly these lines — a counted row and
// the text behind `w`. Every other subcommand routes nothing and keeps the
// operator's terminal, which is where `posse list`, `posse new` and
// `posse kill` want them.
//
// The three vars keep their own identities rather than collapsing into one,
// because each is a TEST seam that a pin swaps on its own
// (githang_qa_test.go, runtimeyamlv2_test.go, panemodereaders_qa_test.go) and
// a shared var would have those pins stealing each other's notices.
// processNoticew is the fourth, for the sites that had no seam at all.
//
// ASSIGNED ONCE, BEFORE ANY GOROUTINE, for RouteBackendWarnings' reason and
// one of its own: these are read from a pass's gathers, from the pulse clock
// and from a launch off the cockpit's event loop, so a process that
// re-assigned them while any of that ran would be a plain data race on the
// writer (`make test-race`). The callers are the two command cases, above
// the first goroutine either builds. Neither is called from a constructor a
// test can reach: a `newCockpit` that routed would leave every cmd/posse
// test writing into a dead sink the next test could not see.

import (
	"io"
	"os"
)

// processNoticew answers for the sites that have no seam of their own: the
// two Hangw fallbacks and the two perm-drift notices. nil is the ordinary
// stderr, the same contract HerdrBackend.Warn has.
var processNoticew io.Writer = os.Stderr

// processNotices is THE resolver for those sites — the one place they ask,
// so routing acts on all of them rather than on the ones a pin happens to
// drive.
func processNotices() io.Writer {
	if processNoticew == nil {
		return os.Stderr
	}
	return processNoticew
}

// RouteProcessNotices points this package's unowned notices at w, for the
// process that knows where its own stream is. See the header for who calls
// it, when, and why nothing else may.
//
// A nil w is a no-op rather than a reset to stderr: every caller is handing
// over a writer it built, and a nil there is a wiring bug that would
// otherwise read as "the default was wanted".
func RouteProcessNotices(w io.Writer) {
	if w == nil {
		return
	}
	processNoticew = w
	gitHangw = w
	runtimeNoticeWriter = w
	noticeWriter = w
}
