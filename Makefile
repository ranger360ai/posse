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

# Stamp the dev build so `posse version` / the cockpit say which build is live.
# The release build stamps itself — it knows its sha is clean.
GIT_SHA   := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
GIT_DIRTY := $(shell test -n "$$(git status --porcelain 2>/dev/null)" && echo -dirty)
LDFLAGS   := -X github.com/ranger360ai/posse/internal/rhq.Build=$(GIT_SHA)$(GIT_DIRTY)

.PHONY: build release install deploy test test-linux vet fmt link-plugin install-detection verify-detection verify-prune-guard verify-id-recycle verify-govern-honesty verify-grok-pin verify-bd-pin verify-bd-dep-safety verify-bd-no-relate-pairs prune-bd-relates-to audit-silent-reverts release-artifacts tap-formula cleanroom cleanroom-verify cleanroom-shell cleanroom-reset

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
# VERSION must agree with internal/rhq.Version; release-artifacts.sh refuses
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

# Unit tests are hermetic: the test binary re-execs as a fake herdr.
# The silent-revert audit runs here (0.2s) because rangerhq-8rtf is precisely
# the failure a green suite does not report: the commit that removed the fix
# also re-skipped its regression pin. A script that has to be REMEMBERED is
# the same objection rangerhq-2f5r raised about the private-index discipline,
# so it gets a trigger rather than a runbook entry.
test:
	$(GOBIN) test ./...
	@scripts/audit-silent-reverts.sh --quiet

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
	gofmt -w cmd internal embed.go

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
	@$(MAKE) --no-print-directory verify-detection

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
# A throwaway Debian container with a DEFAULT PATH and nothing from this
# project — the machine on which the PUBLIC install story gets tested. Its
# value is in what it does NOT contain: ~/go/bin is deliberately NOT on PATH
# in there, because that omission is the P1 under test (ranger-base-253).
# `make cleanroom-verify` asserts that and every other guarantee; run it
# before a test pass. Full runbook: etc/cleanroom/README.md.
# ---------------------------------------------------------------------------
cleanroom:
	scripts/cleanroom.sh start

cleanroom-verify:
	scripts/cleanroom.sh verify

cleanroom-shell:
	scripts/cleanroom.sh shell

cleanroom-reset:
	scripts/cleanroom.sh reset
