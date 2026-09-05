# posse — herdr-native build & test. The tmux-era implementation (bash + Go,
# with its own suite) lives on the tmux-reference branch.
#
# Three artifacts, one promotion step:
#   make build    → bin/posse-go          dev build of the working tree
#   make release  → bin/posse-release     clean build of HEAD, in a temp worktree
#   make install  → $(BINDIR)/posse       the fleet's binary (PATH + plugin)
# `install` is denied to fleet personas (.claude/settings.json); a human runs it.
#
# install builds `release`, never `build`: personas share this checkout, so the
# working tree usually holds somebody's unfinished edits. See scripts/clean-build.sh.

GOBIN  ?= go
BINDIR ?= $(HOME)/.local/bin

# gofmt, and the files it is asked about. One pair of definitions because
# `fmt` (which writes) and `fmt-check` (which reads) have to name the same
# files or the reader's "run `make fmt`" is advice that does not work
# (ranger-base-rulbl): before this, `fmt` named `embed.go` while
# TestTreeIsGofmtClean walked the whole repo root, so the 43 *_qa_test.go
# files beside embed.go were pinned by a test and fixed by nothing.
#
# The toolchain's gofmt, not PATH's. gofmt's output is version-specific and
# the pin runs go/format from $(GOBIN)'s own toolchain, so a PATH gofmt from
# a different Go would disagree with the test it is a door to; a gated
# session's PATH is shims before tools.
GOFMT     := $(shell $(GOBIN) env GOROOT)/bin/gofmt
FMT_ROOTS := cmd internal *.go

# Stamp the dev build so `posse version` / the cockpit say which build is live.
# The release build stamps itself — it knows its sha is clean.
#
# Shelled out to cmd/buildstamp rather than composed here: a dirty tree's
# half of the stamp is a content fingerprint (internal/posse.dirtyIdent), not a
# bare bit, so two different dirty trees at the same HEAD read as two
# different builds instead of one. A make/shell reimplementation of that
# fingerprint would be a second implementation to keep in sync by hand —
# exactly how GIT_DIRTY drifted from SourceBuildStamp in the first place
# (ranger-base-qyws). An empty BUILD_STAMP (not a git checkout) stamps
# nothing, same as before: `posse version` then falls back to go's own build
# info instead of a hardcoded "unknown" (see versionString in version.go).
BUILD_STAMP := $(shell $(GOBIN) run ./cmd/buildstamp)
LDFLAGS     := -X github.com/ranger360ai/posse/internal/posse.Build=$(BUILD_STAMP)

.PHONY: build release install deploy test test-reuse fmt-check crew-check selector-check seed-check history-check doc-check identity-check ops-check tree-check verify-test-times verify-suite-lock verify-parallel verify-gotest test-linux vet fmt link-plugin install-detection verify-detection verify-prune-guard verify-id-recycle verify-self-close verify-govern-honesty verify-grok-pin verify-codex-pin verify-credential-paths verify-hook-freshness verify-bd-pin verify-bd-argv-gate verify-gate-freshness verify-pid-deny-set verify-bd-dep-safety verify-bd-no-relate-pairs verify-runtime-walk prune-bd-relates-to audit-silent-reverts release-artifacts tap-formula release-notes macos-install-probe cleanroom cleanroom-verify cleanroom-verify-all cleanroom-shell cleanroom-reset cleanroom-distros cleanroom-hook-deps

build:
	$(GOBIN) build -ldflags '$(LDFLAGS)' -o bin/posse-go ./cmd/posse

# Build HEAD in a throwaway worktree, so the working tree cannot reach it.
release:
	GOBIN='$(GOBIN)' scripts/clean-build.sh bin/posse-release

# Promote the release build to the live binary. install(1) unlinks the target
# before writing, so running cockpits/dispatches keep their old inode.
#
# THE `rhq` SYMLINK IS GONE FROM THIS TARGET (ranger-base-igup, was
# rangerhq-tyay). It was transition mechanics, never a second name: on promote
# day an instance was still full of the old spelling — persona standing orders,
# permission allowlists, saved recipes — so the rename shipped a same-inode
# alias beside the binary and nothing broke at the moment of promotion. That
# job is done, MEASURED 2026-08-27: the live dispatch loop's own recipe records
# `<repo>/plugin/bin/posse dispatch --watch` (pidfile and `ps`), the plugin
# manifest runs ./bin/posse, and no herdr config, shell profile or session
# recipe on this box invokes the alias. Zero consumers, so the build stopped
# writing it. `posse` is the command. Retiring the two inodes that predate this
# change is the operator's, inside ranger-base-3rv9's window (ranger-base-6y83);
# nothing here recreates them.
install: release
	install -d $(BINDIR)
	install -m 0755 bin/posse-release $(BINDIR)/posse
	@echo "installed: $(BINDIR)/posse"
	@echo "  promoted: $$(git rev-parse --short HEAD) $$(git log -1 --format=%s HEAD)"
	@echo "  version : $$($(BINDIR)/posse version)"
	@scripts/path-warning.sh '$(BINDIR)'
	@scripts/verify-gate-freshness.sh --warn || true

deploy: install

# ------------------------------------------------------------------ shipping
#
# `release` above builds ONE binary for THIS machine and is the promote
# rehearsal. These two build what the world downloads, and they are a
# different thing: four cross-compiled tarballs plus the Homebrew formula
# that names their sha256s (rangerhq-i0n0).
#
# Neither target tags, publishes, or talks to GitHub — they write dist/ and
# stop. The release itself is .github/workflows/release.yml, fired by the
# operator pushing a `vX.Y.Z` tag, and it drafts rather than publishes.
# docs/runbooks/release.md is the whole procedure including the two clicks
# that are the operator's.
#
#   make release-artifacts VERSION=v0.3.0   -> dist/*.tar.gz + dist/checksums.txt
#   make tap-formula       VERSION=v0.3.0   -> dist/posse.rb (needs checksums)
#
# VERSION must agree with internal/posse.Version; release-artifacts.sh refuses
# the build otherwise, because a binary whose `posse version` contradicts its
# own download URL is worse than no release.
VERSION ?= $(shell git describe --tags --exact-match 2>/dev/null)

release-artifacts:
	@[ -n "$(VERSION)" ] || { echo "make release-artifacts: VERSION=vX.Y.Z (HEAD carries no exact tag)" >&2; exit 2; }
	scripts/release-artifacts.sh --version '$(VERSION)'

tap-formula: release-artifacts
	scripts/tap-formula.sh --version '$(VERSION)' --checksums dist/checksums.txt --out dist/posse.rb
	@echo "--- dist/posse.rb (copy into ranger360ai/homebrew-tap as Formula/posse.rb) ---"
	@cat dist/posse.rb

# What the release's notes will OPEN with: the CHANGELOG section for VERSION,
# printed while the version number is still free (ranger-base-5356). `--require`
# fails when that version has no section of its own, so the outstanding rename of
# `## Unreleased` is a precondition of the tag rather than something noticed in a
# published release. The workflow's own call is deliberately LENIENT — after the
# tag is pushed a number cannot be reused, so nothing there may burn one.
release-notes:
	@[ -n "$(VERSION)" ] || { echo "make release-notes: VERSION=vX.Y.Z (HEAD carries no exact tag)" >&2; exit 2; }
	scripts/release-notes.sh --version '$(VERSION)' --require

# Unit tests are hermetic: the test binary re-execs as a fake herdr.
# The silent-revert audit runs here (0.2s) because rangerhq-8rtf is precisely
# the failure a green suite does not report: the commit that removed the fix
# also re-skipped its regression pin. A script that has to be REMEMBERED is
# the same objection rangerhq-2f5r raised about the private-index discipline,
# so it gets a trigger rather than a runbook entry.
#
# `-timeout 25m` IS LOAD-BEARING (ranger-base-2ggb, and gilfoyle's
# ranger-base-2ad3/7xla on the same invariant). go test's default is 10m PER
# PACKAGE and internal/rhq spends most of it. Darwin, three sessions, one day
# (2026-08-29): STANDALONE 484.6s / 509.6s / 510.0s / 549.3s / 623.2s; under
# `go test ./...` 491.7s and 537.1s green, then 600.8s / 601.0s / 601.1s —
# which is not an assertion, it is the ceiling arriving as a timeout panic.
# `./...` runs the three packages CONCURRENTLY and starves the long one, so
# the concurrent column is the one that decides the colour, and the worst
# standalone reading is already past the default on its own.
#
# The red it throws names NO TEST through the house filter (`go test ./... |
# grep -E '^(---|ok|FAIL)'` prints a bare `FAIL ... 601.010s`) and unfiltered
# it is a goroutine dump, which reads as a hang in product code. It has
# already cost cycles. The flag does not pretend the package got faster; it
# stops a busy machine from being reported as a broken change.
#
# 25m and not 20m because `make test-linux` runs this same target and
# `PLATFORM=linux/amd64` — emulated on an arm mac — is over 600s every time
# (native linux/arm64 is 112.3s, nowhere near). 25m clears the worst darwin
# reading by 2.4x and leaves the emulated rehearsal room. The cost is that a
# genuine deadlock takes 25m to surface; a hang needing a faster answer is one
# worth a `-timeout` on the command line.
#
# There is no long pole to cut instead — 1442 tests, none over 10.3s, the top
# ten only 14% of the run; docs/notes.d/ranger-base-2ggb.md has the
# distribution and the one structural lever (t.Parallel, blocked by t.Setenv).
#
# So `make test` is the suite command, not a convenience wrapper: a bare
# `go test ./...` still carries the 10m default and is still a coin flip on a
# loaded box. suitetimeout_qa_test.go pins the flag and the floor.
#
# THE RUN GOES THROUGH scripts/test-times.sh (ranger-base-7xla), which is the
# other half of the same bead — the half its author left as a decision rather
# than a default: "raising the timeout hides the growth. A gate that prints the
# per-package seconds and warns above a threshold keeps the signal." The pin
# above holds the FLAG, not the runtime, so internal/rhq could walk from 9
# minutes to 24 with every gate green and the only notice being the day it
# trips. The wrapper prints each package's seconds and its share of the budget,
# names any package over SLOW_PACKAGE_SECONDS (300, a separate number with a
# separate job — internal/rhq is over it today and is meant to say so), and
# when the clock DOES expire prints a block that says the clock expired, since
# a timeout panic and a deadlock in product code print the same goroutine dump.
# It owns no number: the budget it reports against is read out of THIS line, so
# `-timeout 25m` stays the single source of truth and stays where the pin reads
# it. It returns `go test`'s own exit status and never fails on a wall clock.
# `make verify-test-times` (0.4s) pins the reporting and runs first here.
#
# AND IT QUEUES (ranger-base-uvzjk). The wrapper takes one of
# POSSE_SUITE_SLOTS (2) box-wide flocks before it starts, so at most two full
# suites run on this machine at once and the rest wait with a line naming the
# worktree they are waiting on. On 2026-09-04 this box carried FIVE concurrent
# `go test ./...` runs from five crew worktrees on eight cores, at 14% free
# memory: each suite was 2-3x its solo time and the 1-minute loadavg was 899
# against the fleet load guard's ceiling of 60 — so the shop stopped hiring at
# the moment five seats were about to free. A `-run` filter or a named package
# is NOT queued. `make verify-suite-lock` (~17s) pins the slots.
test: fmt-check verify-test-times verify-parallel verify-suite-lock tree-check
	scripts/test-times.sh $(GOBIN) test -timeout 25m ./...
	@scripts/audit-silent-reverts.sh --quiet

# The other half of the ceiling story, and the half ranger-base-pj87l asked
# for: the wall grew 2.4x in four days with test-times.sh warning correctly on
# every run and nothing acting on the warning. The answer is NOT to make that
# warning fatal — an elapsed-seconds red belongs to the box, which is the
# charter test-times.sh argues for itself and this does not overturn. It is to
# fail on the DETERMINISTIC half: a test that lands in internal/posse, could
# take t.Parallel, and does not. That is the decay that made the package one
# 1483-second binary; it reads the same files every time and never depends on
# the machine's mood. ~1s, no suite, no go test. The tool prints the tests and
# the two ways to satisfy it.
verify-parallel:
	@$(GOBIN) run ./cmd/testparallel ./internal/posse check

# Prove the reporter still reports: that the budget column is read from the
# command rather than kept here, that a timeout panic is called a timeout and
# names its package, that a clean run says nothing (with a witness that it
# parsed anything at all), and that `go test`'s exit status is the one you get
# back. 0.4s, no go build, no suite.
verify-test-times:
	@scripts/test-times.sh --self-test

# Prove the box-wide suite queue still queues (ranger-base-uvzjk). Fourteen
# arms, each driving REAL concurrent processes against a scratch lock dir,
# because the only thing worth knowing about a lock is what a SECOND process
# sees: two full suites run at once and a third waits; the waiting line names
# the worktree it is waiting on; a `-run` filter and a single package are not
# queued at all; a freed slot is taken by the waiter; the slot of a `kill -9`
# run is reclaimed with no reaper (which is why it is an flock and not a
# pidfile); an explicit release frees a slot before its process exits; a
# wrapper under `set -e` survives being queued; a bad POSSE_SUITE_SLOTS runs
# on the default without widening the queue; and a wrapper that dies leaving a
# CHILD behind keeps the slot — held on the inherited fd, by design — while
# `--status` says the acquiring pid is gone and names the survivor
# (ranger-base-2fgu4). ~17s, no go build, no suite.
verify-suite-lock:
	@scripts/suite-lock.sh --self-test

# The suite through the reusing wrapper (ranger-base-nw9zg). NOT a faster
# `make test` and not a replacement for it: measured on 40 invocations against
# 40, it removes one Gatekeeper assessment per `go test` invocation and moves
# the wall clock by nothing (1.10s/invocation before, 1.15s after; 0.912s vs
# 0.907s over six interleaved pairs). It also does not route through
# scripts/test-times.sh, so it reports no per-package seconds and no disk
# preflight — `make test` remains the gate. Use this when a package is being
# run over and over from a tight loop and the box is already assessment-bound.
# docs/notes.d/ranger-base-nw9zg.md §5-6 has what it does and does not buy.
test-reuse:
	scripts/gotest.sh ./...

# Prove scripts/gotest.sh still reuses the binary it says it reuses
# (ranger-base-nw9zg). `go test <pkg>` copies the linked test binary into a
# fresh work dir every invocation — two cached runs of the same command gave
# inodes 243177760 and 243178437 — and macOS assesses the first exec of each
# new FILE: 0.806s/1.066s/1.059s against 0.030s/0.035s/0.039s for the second
# exec of the same one. The path is not what is keyed: 200 execs of 200 hard
# links to one inode cost 1 assessment, 200 execs of 200 byte-identical copies
# cost 217. Eight arms, each with a control that must come out the other way.
# ~4s, one throwaway module, no suite.
verify-gotest:
	@scripts/gotest.sh --self-test

vet:
	$(GOBIN) vet ./...

# The same gate, on Linux, from a mac (ranger-base-dbe). The suite had only
# ever been run on darwin, and two defects lived in that gap — ranger-base-fjj
# (ServerGen fences herdr generations on an inode number, which ext4 recycles
# and APFS does not: a runtime bug in the linux tarballs, 9 tests red) and
# ranger-base-gaf (a test hardcoding /bin/zsh). Both were found on a release
# rehearsal, because .github/workflows/release.yml was the first thing that had
# ever run the suite on Linux — on a tag, which is the worst place to learn it.
#
# `go vet ./... && make test` in a throwaway golang container, repo mounted
# READ-ONLY and running as you, so it cannot leave anything in the tree.
# ~35s cold, ~2s warm. IMAGE= / PLATFORM= overrides and a --shell in the
# script; docker required.
test-linux:
	scripts/test-linux.sh

fmt:
	$(GOFMT) -w $(FMT_ROOTS)

# The read-only half, and the whole of ranger-base-rulbl: ~1.5s (0.8s of it
# this Makefile's own parse), no go test, no suite. `TestTreeIsGofmtClean`
# (internal/posse/gofmtclean_qa_test.go) is the pin and stays the pin; this is
# a door to it that opens in under two seconds instead of ~950.
#
# WHY A DOOR WAS NEEDED. That pin lives inside internal/posse, a ~950s
# package, so the standing advice to every seat is a focused `-run` filter
# instead. `-run` selects by test NAME, and no seat's filter has ever named
# `TreeIsGofmtClean`, because gofmt is nobody's subject. Four commits reached
# main not gofmt-clean that way (ranger-base-ig1o, -d4ya, -edg8, -4v4r6); the
# last drew three concurrent P1 beads, three worktrees and three suite runs
# for one whitespace character, because a red internal/posse fails the whole
# package and `make test` then exits 2 for every seat on the box.
#
# Run it by hand before you commit whenever your run was a `-run` filter.
fmt-check:
	@out=$$($(GOFMT) -l $(FMT_ROOTS)); \
	if [ -n "$$out" ]; then \
		echo "not gofmt-clean — run \`make fmt\`:"; \
		echo "$$out" | sed 's|^|  |'; \
		exit 1; \
	fi

# The other twelve doors in the same class (ranger-base-ik44f, discovered
# from rulbl above; the last eight added under ranger-base-sx2dq, which found
# that ik44f's census keyed on ONE repo-root helper while the tree carried a
# byte-identical twin of it — see treewidedoor_qa_test.go's arm 2). The class is "a QA test whose subject is the TREE, living
# inside a package nobody runs whole": internal/posse is ~950s, so every seat
# runs a focused `-run` filter, `-run` selects by test NAME, and a tree-wide
# pin is nobody's subject — so no filter ever names it and the pin is
# unreachable at exactly the moment it would have mattered. gofmt got the
# first door; these are the rest.
#
# WHY A `-run` FILTER AND NOT A SECOND IMPLEMENTATION. fmt-check re-runs the
# TOOL (`gofmt -l`) because gofmt is a tool. These pins are Go: their reading
# is an ast parse, an unquote and a case-boundary scan, and a shell rewrite of
# that would be a second implementation to keep in sync by hand — a door that
# drifts NARROWER than the pin while both look green, which is worse than no
# door. So the door runs the pin itself. MEASURED 2026-09-04: the package's
# test binary compiles in ~11s cold, the thirteen tests run in ~4s, and
# `make tree-check` was 13-16s warm (it was 5.1s at four pins; the eight
# ranger-base-sx2dq added cost ~9s, nearly all of it `go test` starting five
# times rather than two). RE-MEASURED 2026-09-04 at eighteen pins and seven
# doors (ranger-base-xndgk): 21-41s warm over four runs, of which the five new
# pins are ~4s
# of tests and the two new `go test` starts most of the rest — against the
# ~950s of running internal/posse whole.
#
# `-count=1` because a door that can answer from cache is a door that can lie:
# the drift these pins are about arrives as a new file in a walked directory,
# and nothing promises `go test`'s cache key notices one.
#
# `-timeout 15m` is the house floor (ranger-base-2ggb, pinned by
# TestQANoEntryPointRunsGoTestOnTheDefaultTimeout): every entry point that
# runs `go test` makes the timeout decision rather than inheriting the 10m
# default. These two run ~7s of tests, so the number is a formality here —
# but a door exempted from the rule is the next entry point nobody notices.
#
# One variable per door. Four of them name pins the union check cannot verify
# by wiring, and all four are pinned by name in treewidedoor_qa_test.go's arm
# 2 so they cannot become a parking spot: QA_TOOL_PINS is the pin whose door
# is a TOOL rather than a filter, and QA_HISTORY_PINS, QA_IDENTITY_PINS and
# QA_OPS_PINS read THIS repo with git — its history, its tracked paths, its
# tracked content — which a copied tree does not have, so the drift arm runs
# them clean here instead of planting drift in a scratch copy.
#
# The rest is mechanical: treewidedoor_qa_test.go derives the class from a
# walk of internal/posse — the tests that read the repo root, and the tests
# that reach a helper which WALKS from it — and checks the union both ways. A
# new tree-wide pin fails that check until it is given a door here.
QA_CREW_PINS     := TestShippedTreeNamesRolesNotThisCrew|TestShippedStringsNameRolesNotThisCrew|TestTestCorpusHidesNoCrewNameBehindAnEscape
QA_SELECTOR_PINS := TestHerdrSelectorsAreNamedByADR0016
QA_TOOL_PINS     := TestTreeIsGofmtClean
QA_SEED_PINS     := TestSeedSurfaceNameCountIsZero|TestSeedConfigLiveKeysAreRead
QA_HISTORY_PINS  := TestPublicationRootCommitOmitsExcludedPaths|TestPublicationRootCommitADRsCarryProvenance|TestPublicationHistoryNeverCarriesTheSeedScript|TestShippedExampleTableCoversEveryVersionInGitHistory
QA_DOC_PINS      := TestQANoCodeStringCallsTheDarwinCredentialsFileAStaleLeftover|TestQACageCredDocDoesNotCallTheOnDiskCredentialStale|TestQAADR0036StatusLineDoesNotCarryTheRetractedUnbuiltStamp
QA_IDENTITY_PINS := TestQAIdentityLiteralsNeverAppearInATrackedPath|TestIdentityLiteralsNeverAppearInTheHarnessRepoUndispositioned
QA_OPS_PINS      := TestQAEveryOpsHitInTrackedMarkdownIsRuled|TestQAOpsShapeTableCanStillSayNo|TestInstancePathFormNeverAppearsInTrackedContentUndispositioned|TestQAInstancePathCensusCanStillSayNo

# The crew-name trio, one door between them because they are one question —
# does the shipped tree name this instance's crew (ADR 0012 App.A 5) — asked
# of raw lines, of shipped string literals, and of the escapes in the test
# corpus. ~2.5s of tests. The substitute this replaces was a hand-composed
# `grep -rn '<every crew name>' cmd etc examples internal *_test.go`, off a
# list of names a seat had to remember; this one prints path, line and the
# offending name, and it IS the pin, so it cannot disagree with the suite.
# Type it before you commit when your change touched cmd/, internal/, etc/,
# examples/ or any *_test.go.
crew-check:
	$(GOBIN) test ./internal/posse -timeout 15m -count=1 -run '^($(QA_CREW_PINS))$$'

# ADR 0016 §1's four wire selectors, as named by the ADR page. ~0.5s, nearly
# all of it the same package compile crew-check already paid for. Type it when you touch herdrevents.go or
# the ADR.
selector-check:
	$(GOBIN) test ./internal/posse -timeout 15m -count=1 -run '^($(QA_SELECTOR_PINS))$$'

# The eight members ranger-base-ik44f's census could not see, doored under
# ranger-base-sx2dq. They were invisible because the census keyed on ONE
# identifier and the tree carried a byte-identical twin of it (qspRepoRoot)
# plus a hand-rolled `filepath.Abs("../..")`; the twin is folded, the census
# now also follows a helper that walks from the root, and these are what it
# found. ~0.8s of tests between both doors.
#
# The publication surface, walked: the retired harness name anywhere in the
# tree, and every seed key that ships armed with no reader. Type it when you
# touch examples/ or anything the published tree ships.
seed-check:
	$(GOBIN) test ./internal/posse -timeout 15m -count=1 -run '^($(QA_SEED_PINS))$$'

# The publication HISTORY: the root commit omits the excluded paths, its ADRs
# carry provenance, and the seed script is absent from every commit. Same
# class and same reason for a door, but a separate variable because these
# three read `git log` in THIS repo — they cannot be exercised in a copied
# tree, which has no .git, so treewidedoor_qa_test.go's drift arm runs them
# clean here and pins the membership by name rather than planting drift.
history-check:
	$(GOBIN) test ./internal/posse -timeout 15m -count=1 -run '^($(QA_HISTORY_PINS))$$'

# The prose pins over shipped code and docs: a retired ADR 0019 framing that
# can walk back in with the next comment somebody writes, and ADR 0036's
# status-line row. Type it when you touch a doc comment in internal/posse or
# a row in docs/adr/.
doc-check:
	$(GOBIN) test ./internal/posse -timeout 15m -count=1 -run '^($(QA_DOC_PINS))$$'

# The two doors ranger-base-xndgk FINDING 5 added, for four of the five pins
# it found undoored (the fifth went to history-check above). The census that
# derives this class keyed on Go's own filesystem calls — a body that calls
# qibRepoRoot, or that reaches a helper which WalkDirs from it — and those
# five took their root from `git rev-parse --show-toplevel` instead, which is
# stdout from a subprocess and invisible to both rules. One of them censuses
# EVERY TRACKED PATH in the repository. They now take the root from the one
# helper and are fenced there (treewidedoor_qa_test.go's
# TestQAOneRepoRootHelperInTheTestPackage).
#
# This box's identity literals, asked of the tree two ways: never in a
# tracked PATH, never in tracked CONTENT past the dispositioned three. ~0.5s.
# Type it when you add a file whose name or body could carry a box's
# username, git email or instance path.
identity-check:
	$(GOBIN) test ./internal/posse -timeout 15m -count=1 -run '^($(QA_IDENTITY_PINS))$$'

# The ops residue census over every tracked markdown file, the instance
# path-form census over every tracked FILE (ranger-base-l9ii: this
# deployment's constitution checkout written as a live path, dispositioned
# per file), and the control beside each that says it can still say no. ~2s.
# Type it when you write or edit a .md anywhere in the tree, and when you
# write a path into a doc, a comment or a script.
ops-check:
	$(GOBIN) test ./internal/posse -timeout 15m -count=1 -run '^($(QA_OPS_PINS))$$'

# The whole class, one command: every tree-wide pin in internal/posse — 21-41s
# over four runs at eighteen pins. No recipe of its own — the doors are its prerequisites, so `make -n
# tree-check` prints exactly what a seat would otherwise have to type. It is a
# prerequisite of `make test` for rulbl's reason: a full run fails on it in
# seconds instead of at ~950.
tree-check: fmt-check crew-check selector-check seed-check history-check doc-check identity-check ops-check

# Register the cockpit plugin with the running herdr (local dev link).
# The manifest runs ./bin/posse relative to the plugin root; that is a symlink
# to the *installed* binary, so the popup never runs an unpromoted build.
# plugin/bin/rhq is no longer written here (ranger-base-igup). It was kept on
# the claim that it was load-bearing — that a session recipe records the
# ABSOLUTE command it was launched with, and the fleet's dispatch --watch loop
# was recorded as `<repo>/plugin/bin/rhq dispatch --watch`, so dropping the link
# would break relaunching the loop that dispatches everything else. That claim
# was measured on 2026-08-27 and is false: the live loop's recipe, its pidfile
# and `ps` all name `<repo>/plugin/bin/posse dispatch --watch`, and
# autostart.sh's own default is $$here/bin/posse. No recipe on this box invokes
# the old spelling.
link-plugin:
	mkdir -p plugin/bin
	ln -sfn $(BINDIR)/posse plugin/bin/posse
	herdr plugin link $(CURDIR)/plugin

# Install every agent-detection override in etc/herdr/agent-detection into
# herdr and reload it in the running server. Each override exists because
# herdr's stock manifest reports a screen that is HOLDING THE KEYBOARD as
# idle — no rule matches and detection falls through to
# default_known_agent_idle_fallback — so a dispatched prompt is typed into
# that screen instead of the composer, silently:
#   codex.toml  the "Hooks need review" dialog and every codex modal sharing
#               its "esc to go back" footer          (rangerhq-7ia)
#   grok.toml   the startup splash: New worktree / Resume session menu,
#               changelog line, "Help improve Grok"  (rangerhq-37c)
# Local overrides shadow both the cached remote and the bundled manifest and
# survive `herdr update`; verify-detection warns when either moves past our
# fork point.
HERDR_DETECTION_DIR ?= $(HOME)/.config/herdr/agent-detection
HERDR_DETECTION_SRC := $(wildcard etc/herdr/agent-detection/*.toml)

install-detection:
	install -d $(HERDR_DETECTION_DIR)
	for f in $(HERDR_DETECTION_SRC); do install -m 0644 "$$f" $(HERDR_DETECTION_DIR)/; done
	herdr server reload-agent-manifests >/dev/null
	@for f in $(HERDR_DETECTION_SRC); do echo "installed: $(HERDR_DETECTION_DIR)/$$(basename $$f)"; done
	@scripts/verify-detection.sh --check-install

# Replays the fixtures against the manifests in THIS CHECKOUT, staged into a
# throwaway XDG_CONFIG_HOME (ranger-base-53w1) — so a committed detection change
# can fail here before anyone installs it. The install is reported, and only
# install-detection's own run (--check-install) fails on a mismatch.
verify-detection:
	scripts/verify-detection.sh

# The promote gate for the session-meta prune guard (rangerhq-m15): plants
# ghost metas in a scratch RHQ_HOME and lists them against the LIVE herdr,
# which is only ever read. Pass RHQ=<path> to test a candidate before it is
# promoted; the default is whatever PATH resolves, i.e. the fleet's binary.
# (POSSE= was RHQ= before the command rename, rangerhq-tyay.)
POSSE ?= $(shell command -v posse)

verify-prune-guard:
	scripts/verify-prune-guard.sh $(POSSE)

# herdr workspace-id recycling (rangerhq-6bg7 / rangerhq-6bbz). Scratch
# --session server only; the fleet default server is snapshotted, never
# aimed at. Allocator is max(live)+1 recomputed at every process start.
verify-id-recycle:
	scripts/verify-id-recycle.sh

# Does a process survive closing the workspace its own pane is in?
# (ranger-base-hslbb, the question `posse relaunch --no-land` turns on when it
# is typed inside the session it names.) Scratch HOME plus a scratch
# --session, so it runs from inside a caged seat and the fleet is never
# addressed. Prints SELF=/DETACHED=/SETSID=; measured died/died/survived.
verify-self-close:
	scripts/verify-self-close.sh

# The governance surface's honesty when the loop it monitors is dead
# (rangerhq-mgvx, on rangerhq-81y0's surface). Scratch --session herdr and a
# scratch RHQ_HOME on both sides; kills its own watch loop with -9 and asserts
# that `posse status` raises G7 and exits non-zero, that the cockpit HEADER
# says it, that a stale pidfile naming a live pid cannot suppress it, and that
# the pulse — and only the pulse — dies with the loop. The control arm (loop
# alive, surface clear) is what makes the rest a probe.
verify-govern-honesty:
	scripts/verify-govern-honesty.sh $(POSSE)

# The grok version pin (rangerhq-y7jr). grok ships `[cli] auto_update = true`
# and a leader process that downloads a new binary and relaunches itself
# mid-life, so the fleet runtime can roll forward with no review — retiring
# version-verified findings (consent-record has no server handler in 1.0.5;
# CLI --permission-mode beats config permission_mode) that fleet safety rests
# on. etc/grok/version-pin.toml is the declaration; this asserts the live
# ~/.grok/config.toml still matches it, and prints the security re-audit list
# when upstream stable moves past the pin. Lifting the pin is the operator's.
verify-grok-pin:
	scripts/verify-grok-pin.sh

# The codex version pin (ranger-base-poj5). codex has NO version-ceiling
# config key at all — required_maximum_version, maximum_version and
# auto_update appear zero times in the binary against a positive control — so
# unlike grok there is nothing to set that refuses to START. The pin is the
# Homebrew cask (`brew pin --cask codex`, which also blocks codex's own
# updater, since on a brew install its update action IS `brew upgrade --cask
# codex`), plus `check_for_update_on_startup = false` to stop the startup menu
# whose default option is "1. Update now", plus an asserted rollback target: a
# cask keeps exactly one version and `brew cleanup` takes the old one, where
# grok keeps every build in ~/.grok/downloads/. etc/codex/version-pin.toml is
# the declaration; this asserts the live box still matches it and prints the
# re-audit list when the tap moves. Lifting the pin is the operator's.
verify-codex-pin:
	scripts/verify-codex-pin.sh

# ADR 0019 "path 3": a credential file in the Claude Code config directory
# (ranger-base-zzc, escaped as ranger-base-m6cm). The operator deleted the file
# on 2026-08-26 03:40; a new one was created at 11:47:07 the same day and
# nothing noticed for two days. Deleting a file whose defining property is that
# it regenerates is not a control — this is. Read-only: it prints metadata, never
# content, and never deletes (that is the operator's, as it was on ranger-base-66y).
# Exit 1 = a file is there. Exit 2 = no config dir present, i.e. nothing measured.
# Runbook: docs/runbooks/credential-rotation.md.
verify-credential-paths:
	scripts/verify-credential-paths.sh

# The L3 hook staleness control (ranger-base-8zki). Hook bodies are compiled
# into the binary, so every hook on the box is a COPY; only `gates
# install-hooks` and a session create re-render one, and sessions land in
# worktrees whose COMMON hooks dir is the posse checkout's. Any OTHER hooked
# repo — an instance's private beads repos, which never hold a session — is
# re-rendered by nothing at all, and one such pair ran a hook three days behind
# the binary: still refusing, but prescribing the bare two-dot `git diff`, which
# is blind to another persona's staged edit and is precisely what
# ranger-base-erba landed b291784 to remove. Per configured repo: identity
# against a fresh render from the binary on PATH (with the visibility stamp
# normalized on both sides), the stamp against config beads_visibility:, and
# both behavior arms — unqualified refused, path-limited allowed. Read-only;
# a finding prints the install-hooks line for the operator to type.
# Exit 1 = drift. Exit 2 = no configured repo present, i.e. nothing measured.
verify-hook-freshness:
	scripts/verify-hook-freshness.sh

# The bd version pin (rangerhq-f49). bd 0.49.1 auto-spawns a per-repo daemon on
# any call and that daemon outlives the binary it was exec'd from: `brew upgrade
# beads` deleted /opt/homebrew/bin/bd underneath one on 08-16, and the rollback
# that day was verified at the COMMAND layer only — `bd version`, `bd ready`,
# `dispatch --dry-run`, all green — so the orphan ran on for 12d21h and then
# degraded every bd write on the box for ~40 minutes on 08-26. This asserts the
# pin at BOTH layers: version, `command -v bd`, homebrew's keg unlinked-or-
# pinned, and every live `bd daemon` running the pinned binary and younger than
# it. etc/bd/version-pin.toml is the declaration. Read-only — it kills nothing
# (`Bash(bd daemon:*)` is denied fleet-wide); remediation is the operator's.
verify-bd-pin:
	scripts/verify-bd-pin.sh

# The bd argv gate's two halves must agree (ranger-base-hthx). The sh wrapper
# decides, in a shell builtin, whether to start the parser at all, and that
# fast path's one obligation is to be LOOSER than the parser. It was not: it
# tested the payload for a literal `bd` substring while the parser resolves
# the command word with shlex FIRST, so every spelling the shell concatenates
# into the name -- b\d, b''d, b"d" -- was refused by the parser and never
# reached it. This walks the whole quoting alphabet instead of a list of
# spellings someone thought of, and exits 2 rather than 0 if it found nothing
# to refuse. Read-only; ~23s at the default MAXLEN=4.
verify-bd-argv-gate:
	scripts/verify-bd-argv-gate.sh

# The other half of that gate: the copy that actually fences this box
# (ranger-base-d0jo). `scripts/bd-argv-gate.{sh,py}` here is SOURCE. What the
# PreToolUse hook runs is `~/.config/posse/gate/bd-argv-gate.sh`, an operator-
# owned copy — "a PreToolUse hook the operator may install, not one posse
# renders" (ADR 0015 section 3) — so landing a fix to the source moves nothing
# and nothing notices. ranger-base-hthx landed its wrapper fix and wrote that
# caveat on its own close; c892569 (ranger-base-1lvm) landed the next one, and
# a copy predating it keeps the fail-OPEN wrapper with no signal anywhere. A
# note on a close is not a control.
#
# So `make install` ends with this, in --warn (report, never fail the promote:
# the binary went out fine and a stale gate is a different repair). It resolves
# the wrapper from the operator's settings.json rather than assuming the path,
# compares BOTH files against the MAIN CHECKOUT's HEAD -- never a persona
# worktree's, which is the tree that must not reach a box-wide hook -- and then
# runs the installed wrapper three times, because a byte-perfect gate with no
# python3 under it passes everything. A finding prints the one line to type.
#
# It installs nothing: the copy exists precisely so a persona-writable tree
# cannot move it, and an --install flag here would be that tree, one flag away.
# Exit 1 = drift or a dead gate. Exit 2 = the gate is not installed here —
# `|| true` on the install line swallows both, because a box that never
# installed the gate must not fail a promote over it.
verify-gate-freshness:
	scripts/verify-gate-freshness.sh

# Does every PID in a posse home carry the fence ADR 0015 section 3 says it
# carries (ranger-base-d866)? The rules only travel with a dispatched session
# because they are IN the PID -- that is what becomes the L1 PATH shim and
# claude's --disallowedTools, neither of which cares which repo the session is
# standing in. A .claude/settings.json does care, which is the finding. What
# nothing was checking is the step in between: eleven PIDs edited by hand and
# staying edited. Defaults to the repo's own examples/ so it runs anywhere;
# pass a home (or set RHQ_HOME) to audit a promoted one. Read-only, no bd.
# `make verify-pid-deny-set HOME_DIR=~/.config/posse`, or --self-test.
#
# The same script carries two more readers (ranger-base-9ix7), and neither is
# on this target ON PURPOSE. `--live` compares every live session's frozen
# --disallowedTools argv against the PID it was launched from, so it answers 2
# on a box with no fleet running and 1 whenever a session is mid-bead behind a
# PID edit -- both correct, neither a build result. `--settings <repo>` reads a
# repo's committed .claude/settings.json, which is constitution class: no
# persona can repair a finding it reports, and a target no one can turn green
# is the c7ek shape. Run them by hand:
#   scripts/verify-pid-deny-set.sh --live ~/.config/posse
#   scripts/verify-pid-deny-set.sh --settings .
HOME_DIR ?= examples
verify-pid-deny-set:
	scripts/verify-pid-deny-set.sh --self-test
	scripts/verify-pid-deny-set.sh $(HOME_DIR)

# The bd 0.49.1 dep-add landmine (ranger-base-pkqn). bd's cycle check walks
# the whole dependency graph with UNION ALL — walks, not nodes, depth 100, all
# edge types — and `relates-to` edges are always symmetric, so each one is a
# 2-cycle the walk bounces across ~7x per level. A `dep add` whose TARGET can
# reach such a pair does not terminate, and soft-locks every other bd client
# while it holds the write lock. No drop-in fix exists: the SQLite line ends at
# 0.50.3 with the bug byte-identical, and 0.51+ is the Dolt migration. This
# prints the pairs and the unsafe targets; pass an id to gate one dep add.
verify-bd-dep-safety:
	scripts/verify-bd-dep-safety.sh

# The drift detector that keeps the prune above from rotting (ranger-base-nusr).
# `--gate` exits 1 the moment ANY symmetric dependency pair is back in the
# store. Exactly one verb plants one — `bd dep relate` / the deprecated `bd
# relate` — so this failing means someone ran it; `bd dep add -t relates-to`
# writes a single row and is harmless (measured, correcting NOTES as it stood).
verify-bd-no-relate-pairs:
	scripts/verify-bd-dep-safety.sh --gate

# Prints the plan; it does NOT prune. Pruning the fleet store is a deletion on
# live state, so it is the operator's call and takes an explicit
# `scripts/prune-bd-relates-to.sh --apply`.
prune-bd-relates-to:
	scripts/prune-bd-relates-to.sh

# Silent-revert audit (rangerhq-8rtf). ef8d35f, a landed P1 fix, was undone by
# a `bd sync` commit and re-landed 3h52m later; `go test ./...` stayed GREEN the
# whole time, because the same commit that removed the fix restored the t.Skip
# on its regression pin. No test, gate or status line reports that class — a
# human found it by accident reading history for another reason. This is the
# detector: one `git log --raw` pass flagging any commit that puts a path back
# to a state it held before its immediately preceding change — where ABSENCE is
# a state, so the deletion of a file added earlier counts (rangerhq-ypn1: when
# the change landed from the private index was an ADD, the stale index undoes it
# by DELETING, and that half used to score clean). Triaged commits live in
# scripts/silent-reverts.allow; anything untriaged exits 1.
# Run `scripts/audit-silent-reverts.sh --self-test` to prove the detector still
# fires — it plants the real mechanism (a private GIT_INDEX_FILE commit followed
# by a shared-index commit) in a throwaway repo, in BOTH its shapes, and asserts
# the flag; it also plants a plain move and asserts silence.
audit-silent-reverts:
	scripts/audit-silent-reverts.sh

# ---------------------------------------------------------------------------
# clean-room test environment (ranger-base-5zh)
#
# A throwaway container with a DEFAULT PATH and nothing from this project —
# the machine on which the PUBLIC install story gets tested. Its value is in
# what it does NOT contain: ~/go/bin is deliberately NOT on PATH in there,
# because that omission is the P1 under test (ranger-base-253).
# `make cleanroom-verify` asserts that and every other guarantee; run it
# before a test pass. Full runbook: etc/cleanroom/README.md.
#
# FOUR DISTROS (ranger-base-5cj4) — debian (default), fedora, rhel, arch.
# This is the route the operator picked to cover the 2026-08-26 platform ask
# ("macos, omarchy and rhel/fedora") instead of ci.yml matrix rows, because
# `go test ./...` cannot tell one linux distro from another and the userland
# the install commands and generated hooks run in can. Select one per command:
#
#	CLEANROOM_DISTRO=rhel make cleanroom-verify
#
# `make cleanroom-verify-all` walks all four. Expect it to be slow: arch is
# amd64-only and runs under qemu on this box.
# ---------------------------------------------------------------------------
cleanroom:
	scripts/cleanroom.sh start

cleanroom-verify:
	scripts/cleanroom.sh verify

cleanroom-shell:
	scripts/cleanroom.sh shell

cleanroom-reset:
	scripts/cleanroom.sh reset

cleanroom-distros:
	scripts/cleanroom.sh distros

# The macOS half of the same question (ranger-base-hza). cleanroom.sh is Linux
# by construction, so zsh's PATH and the Homebrew tap were written and never
# run; this runs them. Same cadence as the clean room — before a release, not
# per push — and the same exit convention as verify-credential-paths.sh: 2 is
# nothing measured, which is not a pass. It changes nothing on the box; `brew`
# mode reaches a real tap/trust/install through a scratch Homebrew prefix.
# Findings and what it deliberately does not cover:
# docs/runbooks/macos-install-routes.md.
macos-install-probe:
	scripts/macos-install-probe.sh all

# What the generated hooks call, against this distro's userland. A MISSING
# line is a FINDING (ranger-base-rmgz was one), never a cue to install it.
cleanroom-hook-deps:
	scripts/cleanroom.sh hook-deps

# Every distro, built if needed, verified, and its hook dependencies reported.
# Keeps going after a red one and fails at the end naming which — a partial
# walk that stopped at the first failure would leave the rest unmeasured and
# look like a pass.
cleanroom-verify-all:
	@rc=0; broken=""; findings=""; \
	for d in debian fedora rhel arch; do \
	  echo "=== cleanroom: $$d ==="; \
	  if CLEANROOM_DISTRO=$$d scripts/cleanroom.sh verify; then :; \
	  else rc=1; broken="$$broken $$d"; fi; \
	  if CLEANROOM_DISTRO=$$d scripts/cleanroom.sh hook-deps; then :; \
	  else rc=1; findings="$$findings $$d"; fi; \
	  echo; \
	done; \
	if [ -n "$$broken" ]; then \
	  echo "cleanroom: INSTRUMENT BROKEN on:$$broken — do not test in those until fixed" >&2; fi; \
	if [ -n "$$findings" ]; then \
	  echo "cleanroom: HOOK-DEP FINDINGS on:$$findings — a command the generated hooks call is missing there." >&2; \
	  echo "cleanroom: that is a product defect to file (ranger-base-rmgz), NOT a broken clean room." >&2; fi; \
	exit $$rc

# ---------------------------------------------------------------------------
# runtime contract walk (ranger-base-nlya)
#
# The ADR 0013 six-stage contract, walked end to end on ONE real runtime by
# the production dispatch path: it launches its own scratch herdr session,
# fires a throwaway bead at a throwaway PID, and scores every stage with ADR
# 0017 §2's verdicts. It is not in `make test` and never will be — it SPENDS
# A REAL TURN on the runtime under test. Run it before switching a lane back
# onto a runtime, and after any runtime version bump.
#
#	make verify-runtime-walk RUNTIME=codex
#
# An exhausted account is scored UNKNOWN(failing) and stops the walk before
# it spends anything — a fact about the bill, never a runtime defect.
verify-runtime-walk:
	@test -n "$(RUNTIME)" || { echo "usage: make verify-runtime-walk RUNTIME=grok|codex|claude"; exit 2; }
	RHQ_LIVE_RUNTIME=$(RUNTIME) $(GOBIN) test ./internal/posse -run TestLiveRuntimeContractWalk -v -count=1 -timeout 30m
