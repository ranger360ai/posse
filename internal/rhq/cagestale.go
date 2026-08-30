package rhq

// Is the cage image the one this source builds? — the third state
// (ranger-base-nwj7, discovered from ranger-base-jada).
//
// The inner L1/L3 render is the IMAGE's posse (cageinner.go), so a source
// change to it is invisible inside the cage until `posse cage build`. That
// makes every live claim about the render a claim about a *build artifact*,
// and a build artifact has three states, not two: the claim holds, the
// claim is broken, and the artifact is too old to be asked. Read as two
// states the third arrives as a FAIL — and a FAIL is read as a regression
// before it is read at all, which is the half hour ranger-base-jada spent.
//
// The signal is one already baked in and free: the image's own posse names
// the commit it was built from (`posse version` → "0.4.0+0c0607b"), and so
// does a build of the source in hand. Two idents, compared as strings.
//
// Three decisions here, each a trade rather than a detail:
//
//   - Unclear is NOT stale. The skip arm fires only when both idents were
//     READ and they differ. An image that cannot be asked — an engine with
//     no `inner:`, a posse too old to answer `version`, an engine that
//     broke — reads exactly as it did before this file existed. A skip on a
//     probe failure is how a live pin goes silently green forever.
//   - The comparison is against what the CURRENT source would build, never
//     a constant. The trap, measured 2026-08-30 on this box: a *test binary
//     carries no vcs stamp* — `rhq.VersionString()` inside `go test` is
//     "0.4.0+dev" whatever the tree holds, because go stamps -buildvcs into
//     `go build` output and not into a test binary. A comparison written
//     the obvious way would have compared two constants and measured
//     nothing. So the source side is composed from git and rendered through
//     versionString itself, which is also what stops the two from drifting.
//   - ANY commit makes the image stale, not only a commit to the render.
//     Coarse on purpose: "which files the render is made of" is a path list
//     that rots into a lie, and the honest precondition is the simple one —
//     this image is not this source, so what is about to be measured is not
//     this source's render. The cost of the coarse arm is a rebuild that
//     was not strictly needed, and a rebuild is ~55s: measured end to end on
//     this box 2026-08-30 (55.4s wall — the two cross-builds plus the image
//     layers after them, which always rebuild because the binaries are what
//     changed; ranger-base-jada's 12.2s was the docker half alone).

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
)

// The three states. Strings and not an int enum because they are printed
// as often as they are switched on.
const (
	CageImageCurrent = "current" // the image carries the posse this source builds
	CageImageStale   = "stale"   // both idents read, and they differ
	CageImageUnclear = "unclear" // one of them could not be read at all
)

// CageAge is the comparison and everything a reader needs to classify it in
// one glance: which image, what it carries, and what that was measured
// against. Whose names the other side in words ("this source", "this
// posse"), because the two callers compare against different things and a
// line that did not say which would be worse than no line.
type CageAge struct {
	State string
	Image string // the image's posse version ("" = it could not be asked)
	Want  string // the version compared against ("" = it could not be read)
	Whose string // what Want is: "this source", "this posse"
	Name  string // the image
}

// Stale is the one question the callers ask.
func (g CageAge) Stale() bool { return g.State == CageImageStale }

// String is the line a reader classifies in one glance — the same words in
// the live pin's skip and on `posse cage`, so the two cannot describe one
// state differently.
func (g CageAge) String() string {
	switch g.State {
	case CageImageCurrent:
		return fmt.Sprintf("image %s carries posse %s — the same build as %s, so the L1/L3 render inside it is this one", g.Name, g.Image, g.Whose)
	case CageImageStale:
		return fmt.Sprintf("image %s carries posse %s and %s is %s — STALE. The L1/L3 render inside the cage is the IMAGE's, so the wall in there is that build's and not this one's; rebuild it with `posse cage build <posse checkout>` (~55s measured) before reading anything in the cage as a regression.",
			g.Name, g.Image, g.Whose, g.Want)
	default:
		return fmt.Sprintf("image %s: age unknown — its posse answers %q and %s reads %q, and the comparison needs both. Nothing is claimed about the render's age either way, so this is neither fresh nor stale.",
			g.Name, g.Image, g.Whose, g.Want)
	}
}

// cageAge is the classification itself, kept apart from every way of
// obtaining the two strings: it is the thing pinned, and both callers must
// classify identically or the surface and the pin drift.
func cageAge(name, whose, has, want string) CageAge {
	g := CageAge{State: CageImageUnclear, Image: has, Want: want, Whose: whose, Name: name}
	switch {
	case has == "" || want == "":
		// unclear, and deliberately not stale — see the head of this file
	case has == want:
		g.State = CageImageCurrent
	default:
		g.State = CageImageStale
	}
	return g
}

// CageAgeVsSource compares the image against what a build of the checkout
// at src would produce — the question a test of the render asks, because
// the render it asserts is the one in that source.
func (a *App) CageAgeVsSource(e *Engine, image, src string) CageAge {
	return cageAge(image, "this source", a.CageImagePosse(e, image), SourceBuildVersion(src))
}

// CageAgeVsPosse compares the image against the running posse — the
// question the operator's surface asks, because `posse cage` has a binary
// in hand and not necessarily a checkout, and because the launcher and the
// image being two different builds is the thing worth seeing there.
func (a *App) CageAgeVsPosse(e *Engine, image string) CageAge {
	return cageAge(image, "this posse", a.CageImagePosse(e, image), VersionString())
}

var (
	imagePosseMu sync.Mutex
	imagePosse   = map[string]string{} // engine+inner+image → the version word
)

// CageImagePosse asks the image which posse it carries: `<engine inner>
// posse version`, whose line is "posse <version> (herdr-native)". "" when
// there is nothing to ask — no `inner:` spelling, image not built, or a
// posse in it that cannot answer.
//
// Cached per process on the same key CageInnerGatesReady uses: `posse cage
// build` is another process, so the answer cannot change under a running
// posse, and the probe costs 0.775s measured (docker 29.0.1, 2026-08-30).
func (a *App) CageImagePosse(e *Engine, image string) string {
	if e == nil || strings.TrimSpace(e.Inner) == "" || !a.CageImageBuilt(e, image) {
		return ""
	}
	key := e.Name + "\x00" + e.Inner + "\x00" + image
	imagePosseMu.Lock()
	defer imagePosseMu.Unlock()
	if v, ok := imagePosse[key]; ok {
		return v
	}
	v := ""
	if argv := e.stepArgv(e.Inner, CageRender{Image: image, Inner: []string{"posse", "version"}}); len(argv) > 0 {
		if out, err := exec.Command(argv[0], argv[1:]...).Output(); err == nil {
			v = posseVersionWord(string(out))
		}
	}
	imagePosse[key] = v
	return v
}

// posseVersionWord pulls the version out of `posse version`'s line —
// "posse 0.4.0+0c0607b (herdr-native)" → "0.4.0+0c0607b". Anything not of
// that shape is not an answer, and answers "" rather than guessing: a
// mis-parse would compare a string that was never a version and call the
// image stale for the rest of its life.
func posseVersionWord(out string) string {
	for _, ln := range strings.Split(out, "\n") {
		if f := strings.Fields(strings.TrimSpace(ln)); len(f) >= 2 && f[0] == "posse" {
			return f[1]
		}
	}
	return ""
}

// SourceBuildStamp is the identity a build of the checkout at src should
// carry — the short sha, plus a content fingerprint of the dirty edits when
// the tree has any. "" when src is not a git checkout.
//
// The dirty half is a fingerprint and not the bare "-dirty" bit the
// Makefile stamps, because a bit cannot tell two dirty trees apart
// (ranger-base-b6fh). `posse cage build` from a dirty tree, then further
// uncommitted edits before the next comparison, left both states naming
// the same sha-dirty stamp — cageAge read them as one build and a stale
// image reported CURRENT, the exact false positive this file exists to
// prevent. Re-measured 2026-08-30: two dirty edits to the same untracked
// line at the same HEAD now fingerprint differently (dirtyIdent below), so
// the comparison this feeds — CageAgeVsSource, recomputed fresh against the
// LIVE tree on every pin run — can no longer read a moved target as still.
//
// This is the comparison ident's own composition and BuildCageImage's own
// embedded stamp (both call this function), so the two stay consistent
// with each other by construction. It is NOT what the Makefile stamps into
// an ordinary `make build`: that path still emits the bare bit, so
// CageAgeVsPosse — which compares an image's stamp against the RUNNING
// posse's already-baked VersionString(), not a fresh recomputation — can
// read a dirty image as stale against a byte-identical dirty posse. That
// asymmetry is real and is its own change with its own pins, the same
// scoping ranger-base-fqfw used to punt the null/~ rule off the quote fix.
//
// Composed here rather than left to go's own -buildvcs stamp, because that
// stamp is not always there: go looks for a `.git` DIRECTORY and every
// persona works in a linked worktree, where it is a file (ranger-base-bzu,
// pinned in cmd/posse/version_test.go). Re-measured for this comparison on
// go1.26.5, 2026-08-30 — a `go build` from a checkout carries vcs.revision
// and a pseudo-version; the same build from a linked worktree of it carries
// neither, mod "(devel)" and no vcs.* settings at all, silently. An image
// built from a worktree could therefore not say which commit it was, and
// two images built from two different worktrees named themselves
// identically ("+dev"). That is a false CURRENT — the arm this file exists
// to prevent — so BuildCageImage stamps the identity in explicitly, and
// both sides of the comparison are composed right here.
func SourceBuildStamp(src string) string {
	rev, err := git(src, "rev-parse", "HEAD")
	if err != nil || len(rev) < 7 {
		return ""
	}
	// shortSha and not `rev-parse --short`: git's abbreviation length grows
	// with the repository and is configurable (core.abbrev), and an ident
	// that changed length under the repo would compare unequal to itself.
	stamp := shortSha(rev)
	// go calls a tree modified on any `git status` porcelain output,
	// untracked files included, and the Makefile asks the same question the
	// same way. A dirty ident never proves two trees are identical — it
	// only stops a dirty build from claiming to be the commit it sits on.
	if st, err := git(src, "status", "--porcelain"); err == nil && st != "" {
		stamp += "-dirty-" + dirtyIdent(src)
	}
	return stamp
}

// dirtyIdent fingerprints what actually makes src dirty, not merely that it
// is: the tracked diff against HEAD, plus every untracked file's path and
// bytes. Two dirty trees at the same HEAD hash the same only when both of
// those match — which is what "the same build" should mean, and what a
// bare "-dirty" bit could never tell apart (ranger-base-b6fh).
//
// Read-only and best-effort by design: a git or filesystem error here
// leaves the fingerprint short of some of the content rather than failing
// the whole stamp, because "" would read as clean and worse, a hard error
// would fail every live pin on a box where `git diff` merely raced an
// editor's save. The tree is already known dirty by the caller's own
// `status --porcelain` check, so an ident that fails to fully distinguish
// two states is a narrower miss than the bare bit it replaces, never a
// wider one.
func dirtyIdent(src string) string {
	h := sha256.New()
	if diff, err := gitRaw(src, "diff", "HEAD", "--"); err == nil {
		h.Write(diff)
	}
	// `-z` and `--untracked-files=all`: the plain porcelain format quotes
	// odd bytes and collapses an untracked directory to `dir/`, either of
	// which would send the read below at a name no file has
	// (memoryland.go's memoryChanges uses the same spelling for the same
	// reason). porcelainZChanges is the parser already pinned for that
	// format; reused here rather than re-derived.
	if out, err := gitRaw(src, "status", "--porcelain", "--untracked-files=all", "-z", "--"); err == nil {
		for _, c := range porcelainZChanges(out) {
			if !c.Untracked {
				continue
			}
			h.Write([]byte{0})
			h.Write([]byte(c.Path))
			h.Write([]byte{0})
			if b, err := os.ReadFile(filepath.Join(src, c.Path)); err == nil {
				h.Write(b)
			}
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:8]
}

// SourceBuildVersion is what a posse built from the checkout at src calls
// itself — rendered through versionString itself rather than reassembled
// here, so the string this compares against and the string the binary
// prints cannot drift apart. "" when src is not a git checkout, which is
// unclear and not stale.
func SourceBuildVersion(src string) string {
	stamp := SourceBuildStamp(src)
	if stamp == "" {
		return ""
	}
	return versionString(stamp, func() (*debug.BuildInfo, bool) { return nil, false })
}

// PosseCheckout is the source checkout dir belongs to — `git rev-parse
// --show-toplevel`, which answers the WORKTREE's root and not the main
// checkout's, because a worktree is a source tree a cage image can be built
// from and its render is its own. "" when dir is not in a repository.
func PosseCheckout(dir string) string {
	root, err := git(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return ""
	}
	return root
}

// VersionOrDev renders a build stamp the way the binary carrying it will:
// "0.4.0+7e92337", or the honest "0.4.0+dev" for a source that could not be
// named. `posse cage build` prints it so the line the operator watches says
// what is about to go into the image.
func VersionOrDev(stamp string) string {
	if stamp == "" {
		return Version + "+dev"
	}
	return Version + "+" + stamp
}

// cageBuildStampSymbol is the linker symbol `posse cage build` sets so the
// image's posse can name the source it came from. Spelled once and used by
// both the build and its pin, because go does NOT diagnose an -X for a
// symbol that does not exist: a typo here stamps nothing, exits 0, and
// leaves every image calling itself "+dev" forever.
const cageBuildStampSymbol = "github.com/ranger360ai/posse/internal/rhq.Build"

// cagePosseBuildArgv is the cross-build BuildCageImage runs, minus its
// environment — a function so the pin can build with the same argv and ask
// the resulting binary what it thinks it is. An empty stamp builds unstamped
// rather than stamping the empty string, which would read as a version with
// nothing after the "+".
func cagePosseBuildArgv(out, stamp string) []string {
	argv := []string{"build"}
	if stamp != "" {
		argv = append(argv, "-ldflags", "-X "+cageBuildStampSymbol+"="+stamp)
	}
	return append(argv, "-o", out, "./cmd/posse")
}

// CageAgeHere is the comparison `posse cage` prints. Against the posse
// CHECKOUT the given dir sits in when there is one — that is what a `posse
// cage build` typed here would put in the image, so it is the comparison
// whose answer the rebuild instruction is about — and against the running
// posse otherwise, which is always available and is the next most useful
// thing to know: the launcher and the image being two different builds.
func (a *App) CageAgeHere(e *Engine, image, dir string) CageAge {
	if src := PosseCheckout(dir); src != "" && IsPosseSource(src) {
		return a.CageAgeVsSource(e, image, src)
	}
	return a.CageAgeVsPosse(e, image)
}

// IsPosseSource reports whether dir is a checkout `posse cage build` would
// accept — asked by the presence of the one file that command requires, so
// the two cannot disagree about what a source tree is.
func IsPosseSource(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, CageDockerfile))
	return err == nil
}
