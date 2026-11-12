//go:build !posse_arm2 && !posse_arm3

package posse

// QA pins for ranger-base-r3pri — the git shim's hook-skip arm.
//
// Claim: from a seat, on any runtime and under any PID, no ordinary-path
// spelling makes git skip (or re-point) a hook, and the plain commit runs the
// hook. See hookskip.go for the arm and its stated residuals.
//
// These run REAL git in a scratch repo, through the launch line a seat is
// typed under (WrapWithGates), for two runtimes. The "managed" hook lives at
// a core.hooksPath set in the scratch HOME's global config — the ADR 0052
// managed-box shape — and its pre-commit appends to a marker file, so "the
// hook fired" is a line in that file and not an exit code. The scratch repo
// is not the tree, so this is not a tree-wide pin and owes the Makefile no
// door.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// hsLeg is one command line typed after the gate prefix, the env it carries
// (as a typed `VAR=… ` prefix, which is how a seat would spell it), and what
// must happen: rule "" means not refused by this arm.
type hsLeg struct {
	env  string
	args string
	rule string
}

// hsRefused enumerates every spelling the bead names, plus the ones the
// implementation found on the way (am -n, include.*, scope overrides by
// content). Each must exit nonzero naming its rule.
func hsRefused(fx *hsFixture) []hsLeg {
	return []hsLeg{
		// hookskip:no-verify
		{"", "commit -q -n -m d -- f", "hookskip:no-verify"},
		{"", "commit -q --no-verify -m d -- f", "hookskip:no-verify"},
		{"", "commit -q --no-verif -m d -- f", "hookskip:no-verify"},
		{"", "commit -q --no-veri -m d -- f", "hookskip:no-verify"},
		{"", "commit -q --no-v -m d -- f", "hookskip:no-verify"},
		{"", "commit -q -nm d3 -- f", "hookskip:no-verify"},
		{"", "commit -qn -m d3 -- f", "hookskip:no-verify"},
		{"", "-C . commit -q -n -m d -- f", "hookskip:no-verify"},
		{"", "am -n x.mbox", "hookskip:no-verify"},
		{"", "am --no-verify x.mbox", "hookskip:no-verify"},
		{"", "merge --no-verify HEAD", "hookskip:no-verify"},
		{"", "pull --no-verify", "hookskip:no-verify"},
		{"", "rebase --no-verify HEAD", "hookskip:no-verify"},
		{"", "push --no-verify origin main", "hookskip:no-verify"},
		{"", "push --no-verif origin main", "hookskip:no-verify"},
		// hookskip:config-override, any verb
		{"", "-c core.hooksPath=/dev/null commit -q -m e -- f", "hookskip:config-override"},
		{"", "-c CORE.HOOKSPATH=/dev/null commit -q -m e -- f", "hookskip:config-override"},
		{"", "-c core.hooksPath= commit -q -m e -- f", "hookskip:config-override"},
		{"", "-c core.hooksPath commit -q -m e -- f", "hookskip:config-override"},
		{"", "-c hook.scan.command=true commit -q -m e -- f", "hookskip:config-override"},
		{"", "-c include.path=/tmp/x commit -q -m e -- f", "hookskip:config-override"},
		{"", "-c includeIf.gitdir:/.path=/tmp/x commit -q -m e -- f", "hookskip:config-override"},
		{"", "--config-env=core.hooksPath=HOME commit -q -m e -- f", "hookskip:config-override"},
		{"", "--config-env core.hooksPath=HOME commit -q -m e -- f", "hookskip:config-override"},
		{"", "-C . -c user.name=x -c core.hooksPath=/dev/null merge HEAD", "hookskip:config-override"},
		// ranger-base-ap4p6: the override is judged on the VERB. These are
		// the verbs hookSkipReadVerbs does NOT admit, each for a stated
		// reason — an unknown verb (which is what an alias looks like here,
		// and the hole the "any verb" arm was for), the two whose
		// subcommands split on rule (a), and a ref-moving checkout.
		{"", "-c core.hooksPath=/dev/null hs-not-a-verb", "hookskip:config-override"},
		{"", "-c core.hooksPath=/dev/null remote prune origin", "hookskip:config-override"},
		{"", "-c core.hooksPath=/dev/null remote update", "hookskip:config-override"},
		{"", "-c core.hooksPath=/dev/null worktree add " + shQuote(filepath.Join(fx.home, "wt")) + " -d", "hookskip:config-override"},
		{"", "-c core.hooksPath=/dev/null checkout main", "hookskip:config-override"},
		{"", "-c core.hooksPath=/dev/null reset --hard HEAD", "hookskip:config-override"},
		// ranger-base-c1pqo (from ranger-base-jvwcb): git takes an option
		// BEFORE remote's subcommand, and every one of these pruned or
		// updated a ref with hooks off when the arm judged only the next
		// word. MEASURED 2026-11-04, git 2.50.1, plain git in a scratch repo.
		{"", "-c core.hooksPath=/dev/null remote -v prune origin", "hookskip:config-override"},
		{"", "-c core.hooksPath=/dev/null remote --verbose prune origin", "hookskip:config-override"},
		{"", "-c core.hooksPath=/dev/null remote -v -v prune origin", "hookskip:config-override"},
		{"", "-c core.hooksPath=/dev/null remote -vv prune origin", "hookskip:config-override"},
		{"", "-c core.hooksPath=/dev/null remote --verb prune origin", "hookskip:config-override"},
		{"", "-c core.hooksPath=/dev/null remote -v update", "hookskip:config-override"},
		{"", "-c core.hooksPath=/dev/null remote -v -- prune origin", "hookskip:config-override"},
		{"", "-c core.hooksPath=/dev/null worktree -v add " + shQuote(filepath.Join(fx.home, "wt2")) + " -d", "hookskip:config-override"},
		// The deferred -c refusal against z8kym's alias re-walk
		// (docs/notes.d/ranger-base-1agm6.md §5). It fires on the FIRST walk:
		// an alias name is never a read verb, so the override is refused
		// whatever the alias expands to — a commit, and a read too.
		{"", "-c core.hooksPath=/dev/null pcm -q -m c1 -- f", "hookskip:config-override"},
		{"", "-c core.hooksPath=/dev/null pstat", "hookskip:config-override"},
		// And an alias carrying its own -c is judged on the re-walk, where an
		// option before remote's subcommand must not hide a prune either.
		{"", "prm", "hookskip:config-override"},
		// hookskip:config-env
		{`GIT_CONFIG_PARAMETERS="'core.hookspath'='/dev/null'" `, "commit -q -m f -- f", "hookskip:config-env"},
		{`GIT_CONFIG_PARAMETERS="'hook.scan.command'='true'" `, "commit -q -m f -- f", "hookskip:config-env"},
		{"GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.hooksPath GIT_CONFIG_VALUE_0=/dev/null ", "commit -q -m f -- f", "hookskip:config-env"},
		{"GIT_CONFIG_COUNT=2 GIT_CONFIG_KEY_0=user.name GIT_CONFIG_VALUE_0=x GIT_CONFIG_KEY_1=Core.HooksPath GIT_CONFIG_VALUE_1=" + shQuote(fx.otherHooks) + " ", "commit -q -m f -- f", "hookskip:config-env"},
		// A redirect-SHAPED value one level too deep is not the redirect.
		{"GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.hooksPath GIT_CONFIG_VALUE_0=" + shQuote(filepath.Join(fx.hooksRoot, "s", "x")) + " ", "commit -q -m f -- f", "hookskip:config-env"},
		{"GIT_CONFIG_GLOBAL=/dev/null ", "commit -q -m f -- f", "hookskip:config-env"},
		{"GIT_CONFIG_SYSTEM=" + shQuote(fx.plantedSystem) + " ", "commit -q -m f -- f", "hookskip:config-env"},
		// hookskip:config-write
		{"", "config core.hooksPath /dev/null", "hookskip:config-write"},
		{"", "config --local core.hooksPath /dev/null", "hookskip:config-write"},
		{"", "config --global core.hooksPath /dev/null", "hookskip:config-write"},
		{"", "config --file .git/config core.hooksPath /dev/null", "hookskip:config-write"},
		{"", "config --fil .git/config core.hooksPath /dev/null", "hookskip:config-write"},
		{"", "config set core.hooksPath /dev/null", "hookskip:config-write"},
		{"", "config set --local core.hooksPath /dev/null", "hookskip:config-write"},
		{"", "config unset core.hooksPath", "hookskip:config-write"},
		{"", "config --unset core.hooksPath", "hookskip:config-write"},
		{"", "config --unse core.hooksPath", "hookskip:config-write"},
		{"", "config --unset-all core.hooksPath", "hookskip:config-write"},
		{"", "config --add hook.scan.command true", "hookskip:config-write"},
		// ranger-base-jep08: the .enabled keys. Inert on this box's git 2.50.1 and
		// live from 2.54 — MEASURED 2026-11-07 on the ADR 0080 D1 host, Linux
		// x86_64, git 2.56.0: with a managed scanner carried as hook.scan.* in
		// SYSTEM scope, each of these three left it unrun at rc 0, with nothing
		// on either stream (docs/notes.d/ranger-base-jep08.md, P5/P7/P8). They
		// are refused by the same key prefix as hook.scan.command, so no arm
		// moved to admit them; the legs are here because the direction they
		// name did not exist when the prefix was written.
		{"", "-c hook.pre-commit.enabled=false commit -q -m h1 -- f", "hookskip:config-override"},
		{"", "-c hook.scan.enabled=false commit -q -m h2 -- f", "hookskip:config-override"},
		{"GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=hook.pre-commit.enabled GIT_CONFIG_VALUE_0=false ", "commit -q -m h3 -- f", "hookskip:config-env"},
		{"", "config --local hook.pre-commit.enabled false", "hookskip:config-write"},
		{"", "config --replace-all core.hooksPath /dev/null", "hookskip:config-write"},
		{"", "config --remove-section core", "hookskip:config-write"},
		{"", "config remove-section hook.scan", "hookskip:config-write"},
		{"", "config --rename-section core x", "hookskip:config-write"},
		{"", "config include.path /tmp/x", "hookskip:config-write"},
		{"", "config --edit", "hookskip:config-write"},
		{"", "config -e", "hookskip:config-write"},
		{"", "config edit", "hookskip:config-write"},
		// ranger-base-z8kym: aliases. The bead's legs first — each exited 0
		// with the hook unrun before this arm resolved aliases.
		{"", `-c alias.ci="commit -n" ci -q -m a1 -- f`, "hookskip:config-override"},
		{"", `-c alias.ci="!git commit -n" ci -q -m a2 -- f`, "hookskip:config-override"},
		{"", "--config-env=alias.ci=HOME ci -q -m a2 -- f", "hookskip:config-override"},
		{`GIT_CONFIG_PARAMETERS="'alias.ci'='commit -n'" `, "ci -q -m a3 -- f", "hookskip:config-env"},
		{"GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=alias.ci GIT_CONFIG_VALUE_0='commit --no-verify' ", "ci -q -m a4 -- f", "hookskip:config-env"},
		{"", `config alias.ci "commit -n"`, "hookskip:config-write"},
		{"", `config set alias.ci "commit -n"`, "hookskip:config-write"},
		{"", "config --remove-section alias", "hookskip:config-write"},
		// An alias already in config (planted by hsSetup, past the shim, the
		// way an operator's own gitconfig would carry one) is resolved and
		// its expansion judged.
		{"", "pci -q -m p1 -- f", "hookskip:no-verify"},
		{"", "PCI -q -m p1 -- f", "hookskip:no-verify"},
		{"", "-C . pci -q -m p1 -- f", "hookskip:no-verify"},
		{"", "pchain -q -m p2 -- f", "hookskip:no-verify"},
		{"", "pnv -q -m p3 -- f", "hookskip:no-verify"},
		{"", "phooks -q -m p4 -- f", "hookskip:config-override"},
		{"", "psh", "hookskip:alias"},
		{"", "pabs", "hookskip:alias"},
		// ranger-base-crlty finding 1: the D2 exemption is posse's RENDER,
		// not any child of <StateDir>/hooks. git reads a missing hooksPath
		// as no hooks, so each of these skipped the managed hook at rc 0.
		{hsKey0("core.hooksPath", filepath.Join(fx.hooksRoot, "no-such-session")), "commit -q -m r1 -- f", "hookskip:config-env"},
		{hsKey0("core.hooksPath", fx.hooksRoot+"/.."), "commit -q -m r2 -- f", "hookskip:config-env"},
		{hsKey0("core.hooksPath", fx.hooksRoot+"/."), "commit -q -m r3 -- f", "hookskip:config-env"},
		{hsKey0("core.hooksPath", filepath.Join(fx.hooksRoot, "empty")), "commit -q -m r4 -- f", "hookskip:config-env"},
		{hsKey0("core.hooksPath", filepath.Join(fx.hooksRoot, "linked")), "commit -q -m r5 -- f", "hookskip:config-env"},
		// A render-shaped dir OUTSIDE the root, reached through `..`. Only
		// the ValidName check refuses it: the dir test and the member test
		// both pass, so r2's `<root>/..` (no member there) cannot pin it.
		// Built by concatenation: filepath.Join would clean the `..` away.
		{hsKey0("core.hooksPath", fx.hooksRoot+"/../outside-render"), "commit -q -m r6 -- f", "hookskip:config-env"},
		// ranger-base-crlty finding 2: clone/init writing a hook-moving key
		// into the NEW repository's local config. MEASURED 2026-11-10, git
		// 2.56.0, every spelling below wrote it.
		{"", "clone -q -c core.hooksPath=/dev/null " + shQuote(fx.remote) + " " + shQuote(filepath.Join(fx.home, "c1")), "hookskip:config-write"},
		{"", "clone -q --config core.hooksPath=/dev/null " + shQuote(fx.remote) + " " + shQuote(filepath.Join(fx.home, "c2")), "hookskip:config-write"},
		{"", "clone -q --config=core.hooksPath=/dev/null " + shQuote(fx.remote) + " " + shQuote(filepath.Join(fx.home, "c3")), "hookskip:config-write"},
		{"", "clone -q --co=core.hooksPath=/dev/null " + shQuote(fx.remote) + " " + shQuote(filepath.Join(fx.home, "c4")), "hookskip:config-write"},
		{"", "clone -qccore.hooksPath=/dev/null " + shQuote(fx.remote) + " " + shQuote(filepath.Join(fx.home, "c5")), "hookskip:config-write"},
		{"", "clone -q -c CORE.HOOKSPATH=/dev/null " + shQuote(fx.remote) + " " + shQuote(filepath.Join(fx.home, "c6")), "hookskip:config-write"},
		{"", "clone -q -c alias.ci='commit -n' " + shQuote(fx.remote) + " " + shQuote(filepath.Join(fx.home, "c7")), "hookskip:config-write"},
		{"", "clone -q --template=" + shQuote(fx.hookTpl) + " " + shQuote(fx.remote) + " " + shQuote(filepath.Join(fx.home, "t1")), "hookskip:template"},
		{"", "clone -q --te " + shQuote(fx.hookTpl) + " " + shQuote(fx.remote) + " " + shQuote(filepath.Join(fx.home, "t2")), "hookskip:template"},
		// The prefix in its `=` form is its own case arm (--te*=*); the
		// spaced --te above does not reach it.
		{"", "clone -q --te=" + shQuote(fx.hookTpl) + " " + shQuote(fx.remote) + " " + shQuote(filepath.Join(fx.home, "t2e")), "hookskip:template"},
		{"", "init -q --template=" + shQuote(fx.hookTpl) + " " + shQuote(filepath.Join(fx.home, "t3")), "hookskip:template"},
		{"GIT_TEMPLATE_DIR=" + shQuote(fx.hookTpl) + " ", "clone -q " + shQuote(fx.remote) + " " + shQuote(filepath.Join(fx.home, "t4")), "hookskip:template"},
		{"GIT_TEMPLATE_DIR=" + shQuote(fx.hookTpl) + " ", "init -q " + shQuote(filepath.Join(fx.home, "t5")), "hookskip:template"},
		// A RELATIVE template under -C is git's to resolve from the -C dir,
		// which this shim does not do: refused unread, even a clean one.
		{"", "-C " + shQuote(fx.home) + " init -q --template=clean-tpl t6", "hookskip:template"},
		// init.templateDir is a hook-moving key now, in every arm.
		{"", "-c init.templateDir=" + shQuote(fx.hookTpl) + " clone -q " + shQuote(fx.remote) + " " + shQuote(filepath.Join(fx.home, "t7")), "hookskip:config-override"},
		{hsKey0("init.templateDir", fx.hookTpl), "clone -q " + shQuote(fx.remote) + " " + shQuote(filepath.Join(fx.home, "t8")), "hookskip:config-env"},
		{`GIT_CONFIG_PARAMETERS="'init.templatedir'='/x'" `, "init -q " + shQuote(filepath.Join(fx.home, "t9")), "hookskip:config-env"},
		{"", "config --global init.templateDir " + shQuote(fx.hookTpl), "hookskip:config-write"},
		// ranger-base-crlty finding 3: a commit no hook saw.
		{"", "commit-tree HEAD^{tree} -p HEAD -m x", "hookskip:plumbing-commit"},
		{"", "-C . commit-tree HEAD^{tree} -m x", "hookskip:plumbing-commit"},
		// ranger-base-o6ip1 (from ranger-base-2lls3): `git am`. The strongest
		// route of the two 2lls3 measured, because the input is any text
		// file — no prior unscanned commit object is needed, and the content
		// never meets pre-commit or commit-msg. MEASURED 2026-11-11, git
		// 2.56.0, Linux, /usr/bin/git in a scratch repo with the managed
		// pre-commit in global scope: `am <patch>` landed the commit at rc 0
		// with pre-commit unrun (only pre-applypatch ran), with the working
		// tree clean and with it dirty. am was already in hookSkipVerbs and
		// hookSkipNoVerifyVerbs, so -n was refused and the patch route was
		// not — which is why these legs are the verb and not a flag.
		//
		// Every spelling the census named: plain, -C, -c, --config-env, an
		// alias that expands to am, a flag before the mailbox, and `--`.
		// MEASURED with the refusal taken out of the rendered arm
		// (hookSkipVerbExtra emptied), 2026-11-11: these seven each exited 0
		// and MOVED HEAD with the managed pre-commit unrun, while `am -n`
		// and `am --no-verify` stayed refused under hookskip:no-verify — so
		// the no-verify arm keeps the more specific reason and these seven
		// are the hole. The two legs after them are spelling pins and not
		// landing witnesses, and say so.
		{"", "am " + shQuote(fx.amPatches[0]), "hookskip:plumbing-commit"},
		{"", "-C . am " + shQuote(fx.amPatches[1]), "hookskip:plumbing-commit"},
		// A -c whose key is NOT hook-relevant: the refusal must be the am
		// one, not config-override standing in for it.
		{"", "-c user.name=someone am " + shQuote(fx.amPatches[2]), "hookskip:plumbing-commit"},
		{"", "--config-env=user.name=HOME am " + shQuote(fx.amPatches[3]), "hookskip:plumbing-commit"},
		{"", "pam " + shQuote(fx.amPatches[4]), "hookskip:plumbing-commit"},
		{"", "am --3way " + shQuote(fx.amPatches[5]), "hookskip:plumbing-commit"},
		// The scan stops at `--`, so a mailbox named like an allowed option
		// buys no pass — and a mailbox after `--` is still refused.
		{"", "am -- " + shQuote(fx.amPatches[6]), "hookskip:plumbing-commit"},
		// The two SPELLING pins. Neither lands a commit in this fixture —
		// with the refusal out, bare `am` read the test's empty stdin and
		// exited 0 having applied nothing, and `am --continue` exited 128
		// with no am in progress (MEASURED 2026-11-11) — but each is a form
		// that commits when it has its input, and the arm must not admit
		// either on the strength of carrying no mailbox argument. Bare `am`
		// reads a mailbox from stdin; --continue is what finally writes a
		// stalled am's commit, which is why it is not one of the four
		// allowed forms beside --abort.
		{"", "am", "hookskip:plumbing-commit"},
		{"", "am --continue", "hookskip:plumbing-commit"},
	}
}

// hsKey0 is the one-entry GIT_CONFIG_KEY form, typed as a seat would.
func hsKey0(k, v string) string {
	return "GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=" + k + " GIT_CONFIG_VALUE_0=" + shQuote(v) + " "
}

// hsAllowed are the controls: none may be refused by this arm, and the
// commits among them must run the hook.
func hsAllowed(fx *hsFixture) []hsLeg {
	return []hsLeg{
		{"", "commit -q -m n -- f", ""},
		{"", "commit -q -mn -- f", ""},
		{"", "commit -q -m --no-verify -- f", ""},
		{"", "-c user.name=someone commit -q -m c -- f", ""},
		{"GIT_CONFIG_GLOBAL=" + shQuote(fx.globalCopy) + " ", "commit -q -m g -- f", ""},
		{"GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=user.name GIT_CONFIG_VALUE_0=x ", "commit -q -m h -- f", ""},
		{`GIT_CONFIG_PARAMETERS="'user.name'='x'" `, "commit -q -m i -- f", ""},
		{"", "push -n origin main", ""},
		{"", "push --dry-run origin main", ""},
		{"", "config --get core.hooksPath", ""},
		{"", "config core.hooksPath", ""},
		{"", "config get core.hooksPath", ""},
		{"", "config --list", ""},
		{"", "config user.email a@b.c", ""},
		{"GIT_CONFIG_GLOBAL=/dev/null ", "status", ""},
		// An alias for a plain commit runs, and runs the hook.
		{"", "pcm -q -m a -- f", ""},
		{"", "config --get alias.pci", ""},
		{"", "config get alias.pci", ""},
		{"", "--version", ""},
		// An alias loop is git's to refuse ("alias loop detected"), and it
		// runs nothing.
		{"", "ploop", ""},
		// ranger-base-ap4p6: the Claude Code harness's own context calls,
		// verbatim in shape from the refusals.log census in hookskip.go —
		// a hook-key override on a verb that runs no hook to skip. Refusing
		// these cost every seat its branch/status snapshot.
		{"", "-c core.hooksPath=/dev/null status --porcelain --untracked-files=normal", ""},
		{"", "-c core.hooksPath=/dev/null rev-parse --abbrev-ref HEAD", ""},
		{"", "-c core.hooksPath=/dev/null remote get-url origin", ""},
		{"", "-c core.hooksPath=/dev/null remote", ""},
		{"", "-c core.hooksPath=/dev/null remote -v", ""},
		{"", "-c core.hooksPath=/dev/null config -z --get-regexp remote", ""},
		{"", "-c core.hooksPath=/dev/null config --get --path core.excludesFile", ""},
		{"", "-c core.hooksPath=/dev/null ls-files -z --stage", ""},
		{"", "-c core.hooksPath=/dev/null ls-files --error-unmatch -- f", ""},
		{"", "-c core.hooksPath=/dev/null update-index --untracked-cache", ""},
		{"", "-c core.hooksPath=/dev/null write-tree", ""},
		{"", "-c core.hooksPath=/dev/null worktree list --porcelain", ""},
		{"", "-c core.hooksPath=/dev/null --no-optional-locks log --oneline -n 1", ""},
		{"", "-c hook.pre-commit.enabled=false status --porcelain", ""},
		{"", "-c include.path=/tmp/x diff --stat", ""},
		{"", "-C . -c user.name=x -c core.hooksPath=/dev/null log -1", ""},
		// No verb at all: nothing runs, so nothing is skipped.
		{"", "-c core.hooksPath=/dev/null --version", ""},
		// ranger-base-c1pqo: remote's subcommand is the first word that is not
		// an option, so these are reads — and so is a bare `remote -v`.
		{"", "-c core.hooksPath=/dev/null remote --verbose", ""},
		{"", "-c core.hooksPath=/dev/null remote -v show origin", ""},
		{"", "-c core.hooksPath=/dev/null remote -vv get-url origin", ""},
		// An alias that carries its OWN hook-key -c is judged on the re-walk,
		// against the verb it expands to: here a read.
		{"", "pst", ""},
		// ranger-base-crlty: clone and init stay ordinary.
		{"", "clone -q " + shQuote(fx.remote) + " " + shQuote(filepath.Join(fx.home, "ok1")), ""},
		{"", "clone -q -c user.name=x --origin up " + shQuote(fx.remote) + " " + shQuote(filepath.Join(fx.home, "ok2")), ""},
		{"", "clone -q --template=" + shQuote(fx.cleanTpl) + " " + shQuote(fx.remote) + " " + shQuote(filepath.Join(fx.home, "ok3")), ""},
		{"GIT_TEMPLATE_DIR=" + shQuote(fx.cleanTpl) + " ", "init -q " + shQuote(filepath.Join(fx.home, "ok4")), ""},
		{"", "init -q " + shQuote(filepath.Join(fx.home, "ok5")), ""},
		{"", "config --get init.templateDir", ""},
		// The D2 redirect posse renders passes, and the managed hook still
		// runs through its dispatcher (hsPlantRedirects plants it).
		{hsKey0("core.hooksPath", filepath.Join(fx.hooksRoot, "hs-session")), "commit -q -m r -- f", ""},
		// ranger-base-o6ip1: the four `am` forms that END or READ an am in
		// progress commit nothing, and AGENTS.md's sequencer paragraph plus
		// gates.go's sequencerVerbs comment both tell seats to type --abort
		// and --quit to clear a stalled tree. With no am in progress git
		// itself exits 128 on each (`fatal: Resolve operation not in
		// progress`, MEASURED 2026-11-11, git 2.56.0), so what these legs
		// pin is that the refusal is GIT's and not this arm's. The --abort
		// control against a real stalled am, which must exit 0, is
		// TestQAHookSkipAmAbortStillEndsAStalledAm.
		{"", "am --abort", ""},
		{"", "am --quit", ""},
		{"", "am --skip", ""},
		{"", "am --show-current-patch", ""},
		{"", "am --show-current-patch=diff", ""},
		{"", "-C . am --abort", ""},
	}
}

// hsPlantRedirects lays out <hooksRoot> for the ranger-base-crlty legs: one
// render-shaped session dir (posse's member plus a pre-commit dispatcher
// forwarding to the managed hook, the ADR 0052 D2 shape), an empty dir that
// is not a render, and a symlink to the render.
func hsPlantRedirects(t *testing.T, fx *hsFixture) {
	t.Helper()
	render := filepath.Join(fx.hooksRoot, "hs-session")
	if err := os.MkdirAll(render, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteExecutable(filepath.Join(render, "posse-prepare-commit-msg"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	fwd := "#!/bin/sh\nexec " + shQuote(filepath.Join(fx.home, "managed-hooks", "pre-commit")) + " \"$@\"\n"
	if err := WriteExecutable(filepath.Join(render, "pre-commit"), []byte(fwd), 0o755); err != nil {
		t.Fatal(err)
	}
	// Render-shaped (member present, no pre-commit) but beside the root,
	// not under it: allowed, it would run no hook at all.
	outside := filepath.Join(filepath.Dir(fx.hooksRoot), "outside-render")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteExecutable(filepath.Join(outside, "posse-prepare-commit-msg"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(fx.hooksRoot, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(render, filepath.Join(fx.hooksRoot, "linked")); err != nil {
		t.Fatal(err)
	}
}

type hsFixture struct {
	home, repo, marker, hooksRoot, otherHooks, plantedSystem, globalCopy string
	realGit, remote, hookTpl, cleanTpl                                   string
	// One mailbox per refused `am` leg (ranger-base-o6ip1). A slice and not
	// a map keyed by name: an index past the end panics loudly in the leg
	// table, where a missing key would hand the leg an empty path and go on
	// refusing — the leg would still pass and would witness nothing.
	amPatches []string
}

// hsAmPatchCount is how many mailboxes hsSetup builds, one per refused `am`
// leg that carries a patch argument. Each ADDS a file of its own that the
// repo does not have, so every leg applies cleanly to the seed in whatever
// order the table runs them. That is what makes the mutation check real:
// with the refusal dropped, each leg lands a commit the managed hook never
// saw, rather than the first landing and the rest failing to apply.
const hsAmPatchCount = 7

func hsSetup(t *testing.T) *hsFixture {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	realGit := resolveOutside("git", "")
	if realGit == "" {
		t.Skip("no git")
	}
	fx := &hsFixture{home: gitTempDir(t), realGit: realGit}
	fx.repo = filepath.Join(fx.home, "repo")
	fx.marker = filepath.Join(fx.home, "fired")
	managed := filepath.Join(fx.home, "managed-hooks")
	fx.otherHooks = filepath.Join(fx.home, "other-hooks")
	for _, d := range []string{fx.repo, managed, fx.otherHooks} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	hook := "#!/bin/sh\necho fired >> " + shQuote(fx.marker) + "\n"
	if err := WriteExecutable(filepath.Join(managed, "pre-commit"), []byte(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	// The managed hooks path, in global scope — ADR 0052's managed box.
	global := "[core]\n\thooksPath = " + managed + "\n[user]\n\tname = t\n\temail = t@example.com\n"
	write(t, filepath.Join(fx.home, ".gitconfig"), global)
	fx.globalCopy = filepath.Join(fx.home, "global-copy")
	write(t, fx.globalCopy, global)
	fx.plantedSystem = filepath.Join(fx.home, "planted-system")
	write(t, fx.plantedSystem, "[core]\n\thooksPath = "+fx.otherHooks+"\n")

	// ranger-base-crlty: two clone/init templates, one whose config moves
	// the hooks and one that names nothing a scanner relies on.
	fx.hookTpl = filepath.Join(fx.home, "hook-tpl")
	fx.cleanTpl = filepath.Join(fx.home, "clean-tpl")
	write(t, filepath.Join(fx.hookTpl, "config"), "[core]\n\thooksPath = /dev/null\n")
	write(t, filepath.Join(fx.cleanTpl, "config"), "[core]\n\tautocrlf = false\n")
	remote := filepath.Join(fx.home, "remote.git")
	fx.remote = remote
	for _, c := range [][]string{
		{"init", "-q", "--bare", remote},
		{"-C", fx.repo, "init", "-q", "-b", "main"},
		// LOCAL scope, which no leg here hides, and read by `git commit`
		// under every one of them: it forbids git to INVENT an identity
		// from the OS user record when the config scopes a leg leaves
		// standing carry none. That invention is what made the pin 9
		// witness darwin-only (see hsEnv), and it is the one difference
		// between this box and ubuntu-latest that no local run could
		// otherwise show — with this line a seat that drops hsEnv's
		// identity reds here and not two pushes later. It matches none of
		// hookSkipRegexp, so it is in no compare and in no D2 count.
		{"-C", fx.repo, "config", "user.useConfigOnly", "true"},
		{"-C", fx.repo, "remote", "add", "origin", remote},
	} {
		hsRealGit(t, fx, c...)
	}
	write(t, filepath.Join(fx.repo, "f"), "0\n")
	hsRealGit(t, fx, "-C", fx.repo, "add", "--", "f")
	hsRealGit(t, fx, "-C", fx.repo, "commit", "-q", "-m", "seed", "--", "f")
	hsRealGit(t, fx, "-C", fx.repo, "push", "-q", "origin", "main")
	// ranger-base-o6ip1: the `am` mailboxes. Each is a format-patch of a
	// commit that adds one new file, built past the shim and then rolled
	// back, so each applies to the seed on its own. The patch touches a file
	// the legs never write, which matters: hsRun rewrites `f` before every
	// leg, and `git am` applies over a dirty tree as long as the patch does
	// not touch what is dirty (MEASURED 2026-11-11, git 2.56.0 — it landed
	// the commit at rc 0 with the managed pre-commit unrun either way).
	for i := 0; i < hsAmPatchCount; i++ {
		name := "amfile-" + string(rune('a'+i))
		write(t, filepath.Join(fx.repo, name), "am\n")
		hsRealGit(t, fx, "-C", fx.repo, "add", "--", name)
		hsRealGit(t, fx, "-C", fx.repo, "commit", "-q", "-m", "am leg "+name, "--", name)
		p := filepath.Join(fx.home, name+".patch")
		hsRealGitStdout(t, fx, p, "-C", fx.repo, "format-patch", "-1", "--stdout")
		fx.amPatches = append(fx.amPatches, p)
		hsRealGit(t, fx, "-C", fx.repo, "reset", "-q", "--hard", "HEAD^")
	}
	// Persistent aliases, planted past the shim (ranger-base-z8kym).
	for k, v := range map[string]string{
		"pci":    "commit -n",
		"pchain": "pci",
		"pnv":    "commit --no-verify",
		"phooks": "-c core.hooksPath=/dev/null commit",
		"psh":    "!git commit -n -q -m x -- f",
		"pabs":   "!" + realGit + " commit -n -q -m x -- f",
		"ploop":  "ploop2",
		"ploop2": "ploop",
		"pcm":    "commit",
		// ranger-base-c1pqo: the -c refusal against the alias re-walk.
		"pstat": "status",
		"pst":   "-c core.hooksPath=/dev/null status",
		"prm":   "-c core.hooksPath=/dev/null remote -v prune origin",
		// ranger-base-o6ip1: an alias for am, judged on the re-walk.
		"pam": "am",
	} {
		hsRealGit(t, fx, "-C", fx.repo, "config", "alias."+k, v)
	}
	os.Remove(fx.marker)
	return fx
}

// hsEnv is the env every leg starts from: this process's, with every
// GIT_CONFIG_* the seat running this suite carries shed, and HOME the scratch
// one — so the default global scope is the managed one above.
//
// The committer identity is handed over in the ENV, and any the seat carries
// is shed with the rest, because every config SCOPE here is some leg's
// subject: a leg that hides the launcher's global scope hides whatever
// identity lives in it, and pin 9's witness
// (TestQAHookSkipNewlineForgedGlobalLineIsRefused) types
// `GIT_CONFIG_GLOBAL=/dev/null git commit` and must land it at rc 0. The
// legs that move HOME or XDG_CONFIG_HOME can write a `[user]` block into the
// file git will still read; that one has no file at all, so the identity
// cannot be config here and stay the pins' subject.
//
// What carried it before was git inventing a name from the OS user record.
// MEASURED 2026-11-06: `git var GIT_AUTHOR_IDENT` under
// GIT_CONFIG_GLOBAL=/dev/null with a scratch HOME answers from the account's
// gecos name on darwin (git 2.50.1, Apple Git-155) and dies `fatal: empty
// ident name` on ubuntu-latest, whose `runner` account has none — so that
// witness was green on macos-latest and red on ubuntu-latest for the same
// commit 47eabc3d, run 42270221451, the platform split ci.yml exists to show
// (ranger-base-pymhr, over the pin ranger-base-ava0x landed).
func hsEnv(fx *hsFixture) []string {
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "GIT_CONFIG") || strings.HasPrefix(kv, "HOME=") || strings.HasPrefix(kv, "XDG_CONFIG_HOME=") ||
			strings.HasPrefix(kv, "GIT_AUTHOR_") || strings.HasPrefix(kv, "GIT_COMMITTER_") || strings.HasPrefix(kv, "EMAIL=") {
			continue
		}
		env = append(env, kv)
	}
	// The same name and address the fixture's global file carries, so nothing
	// observable about authorship moves with the identity.
	return append(env, "HOME="+fx.home,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
}

func hsRealGit(t *testing.T, fx *hsFixture, args ...string) {
	t.Helper()
	cmd := exec.Command(fx.realGit, args...)
	cmd.Env = hsEnv(fx)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// hsRealGitStdout runs the real git past the shim and writes its stdout to
// path — how the `am` mailboxes are built (ranger-base-o6ip1).
func hsRealGitStdout(t *testing.T, fx *hsFixture, path string, args ...string) {
	t.Helper()
	cmd := exec.Command(fx.realGit, args...)
	cmd.Env = hsEnv(fx)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	write(t, path, string(out))
}

// hsRepoOut reads one trimmed line from the real git IN THE FIXTURE REPO,
// past the shim: the before-and-after facts a residual route is judged on.
// Not hookskipglobal_qa_test.go's hsGitOut, which answers the global-scope
// pins' "want" side — that one takes the args whole and keeps the trailing
// newline, because a config dump compared byte for byte needs both.
func hsRepoOut(t *testing.T, fx *hsFixture, args ...string) string {
	t.Helper()
	cmd := exec.Command(fx.realGit, append([]string{"-C", fx.repo}, args...)...)
	cmd.Env = hsEnv(fx)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

func hsFired(fx *hsFixture) int {
	b, _ := os.ReadFile(fx.marker)
	return strings.Count(string(b), "fired")
}

// hsRun types `<prefix><leg.env>git <leg.args>` into sh in the repo, which is
// how a seat's own shell would run it, and returns stderr and the exit code.
func hsRun(t *testing.T, fx *hsFixture, prefix string, leg hsLeg) (string, int) {
	t.Helper()
	// Every leg edits f first, so a commit has something to commit.
	write(t, filepath.Join(fx.repo, "f"), leg.args+"\n")
	cmd := exec.Command("sh", "-c", prefix+leg.env+"git "+leg.args)
	cmd.Dir = fx.repo
	cmd.Env = hsEnv(fx)
	var errb strings.Builder
	cmd.Stderr = &errb
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return errb.String(), code
}

// The bead's execution pin: through a rendered shim, on claude AND codex,
// each refused form exits nonzero with a named reason and a refusals.log
// line, the hook does not fire, and the plain commit fires it.
func TestQAHookSkipRefusedOnEveryRuntime(t *testing.T) {
	t.Parallel()
	for _, rtName := range []string{"claude", "codex"} {
		t.Run(rtName, func(t *testing.T) {
			t.Parallel()
			fx := hsSetup(t)
			wrapped, gatesDir := hsWrap(t, fx, rtName, "hs-"+rtName)

			// The plain commit runs the hook — the control the whole arm is for.
			if errs, code := hsRun(t, fx, wrapped, hsLeg{args: "commit -q -m plain -- f"}); code != 0 || hsFired(fx) != 1 {
				t.Fatalf("plain commit: code=%d fired=%d, want 0 and 1 (%s)", code, hsFired(fx), errs)
			}

			refused := hsRefused(fx)
			for _, leg := range refused {
				before := hsFired(fx)
				errs, code := hsRun(t, fx, wrapped, leg)
				line := leg.env + "git " + leg.args
				if code == 0 {
					t.Errorf("%s: exited 0, want refused by %s", line, leg.rule)
					continue
				}
				if !strings.Contains(errs, "rule: "+leg.rule+",") || !strings.Contains(errs, "refused by posse gate") {
					t.Errorf("%s: refusal must name %s: %q", line, leg.rule, errs)
				}
				if hsFired(fx) != before {
					t.Errorf("%s: the hook ran — the refusal came after git, not before", line)
				}
			}
			logb, err := os.ReadFile(filepath.Join(gatesDir, "refusals.log"))
			if err != nil {
				t.Fatal(err)
			}
			if n := strings.Count(string(logb), "(hookskip: "); n != len(refused) {
				t.Errorf("refusals.log must carry one hookskip line per refusal: got %d, want %d\n%s", n, len(refused), logb)
			}
			if !strings.Contains(string(logb), "git commit -q -n -m d -- f (hookskip: hookskip:no-verify)") {
				t.Errorf("a log line must carry the argv and the rule:\n%s", logb)
			}

			for _, leg := range hsAllowed(fx) {
				before := hsFired(fx)
				errs, code := hsRun(t, fx, wrapped, leg)
				line := leg.env + "git " + leg.args
				if strings.Contains(errs, "hookskip:") {
					t.Errorf("%s: refused by the hook-skip arm, want allowed: %q", line, errs)
					continue
				}
				if strings.HasPrefix(leg.args, "commit") || strings.Contains(leg.args, " commit ") || strings.HasPrefix(leg.args, "pcm ") {
					if code != 0 || hsFired(fx) != before+1 {
						t.Errorf("%s: code=%d, hook fired %d times, want 0 and once (%s)", line, code, hsFired(fx)-before, errs)
					}
				}
			}
		})
	}
}

// hsWrap renders one persona's gates for rtName and returns the launch-line
// prefix a seat is typed under, plus its gates dir. The crew deny list, the
// D2 redirect plants and the StateDir wiring are the same ones
// TestQAHookSkipRefusedOnEveryRuntime uses (ranger-base-o6ip1).
func hsWrap(t *testing.T, fx *hsFixture, rtName, persona string) (string, string) {
	t.Helper()
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state"), GitScopeEnv: hsEnv(fx)}
	rt, err := a.LoadRuntime(rtName)
	if err != nil {
		t.Fatal(err)
	}
	if rt.Name != rtName {
		t.Fatalf("loaded runtime %q, want %q", rt.Name, rtName)
	}
	fx.hooksRoot = filepath.Join(a.StateDir, "hooks")
	hsPlantRedirects(t, fx)
	// The deny list every crew PID carries.
	wrapped, gatesDir, _, err := a.WrapWithGates(persona, rt, []string{"Bash(git push:*)", "Bash(git commit unless --)"}, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	return wrapped, gatesDir
}

// hsRunLine types `<prefix>sh -c '<line>'` in the repo: the gate prefix
// applied to ONE command, an inner shell, so every bare `git` in a
// multi-command route is answered by the shim. Not `prefix + line`, which
// hsRun can do only because its line is a single command — the prefix may
// END in `/usr/bin/env -u <HOMEVAR>` (RuntimeHomePin), so a `;` typed after
// it would leave the second command ungated (ranger-base-o6ip1).
func hsRunLine(t *testing.T, fx *hsFixture, prefix, line string) (string, int) {
	t.Helper()
	cmd := exec.Command("sh", "-c", prefix+"sh -c "+shQuote(line))
	cmd.Dir = fx.repo
	cmd.Env = hsEnv(fx)
	var errb strings.Builder
	cmd.Stderr = &errb
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return errb.String(), code
}

// The am --abort control, against a REAL stalled am (ranger-base-o6ip1).
// AGENTS.md's sequencer paragraph and gates.go's sequencerVerbs comment both
// tell a seat to type `git am --abort` to clear a tree an am left half
// applied, so the one thing the am refusal must not do is refuse that. The
// hsAllowed legs can only show that the refusal comes from git when nothing
// is in progress; this one shows the command still WORKS when something is.
//
// MEASURED 2026-11-11, git 2.56.0, Linux: a patch that conflicts leaves
// .git/rebase-apply at rc 128, `git am --abort` then exits 0 and removes it,
// and it does so with the working tree dirty — which is the shape hsRun
// leaves, since it rewrites `f` before every leg. The dirty `f` survives the
// abort untouched (` M f` before and after).
func TestQAHookSkipAmAbortStillEndsAStalledAm(t *testing.T) {
	t.Parallel()
	fx := hsSetup(t)
	wrapped, _ := hsWrap(t, fx, "claude", "hs-am-abort")

	// A commit the first mailbox cannot apply over, so the am stalls.
	write(t, filepath.Join(fx.repo, "amfile-a"), "someone else\n")
	hsRealGit(t, fx, "-C", fx.repo, "add", "--", "amfile-a")
	hsRealGit(t, fx, "-C", fx.repo, "commit", "-q", "-m", "conflicting amfile-a", "--", "amfile-a")
	// Past the shim on purpose: the am under test here is the --abort.
	stall := exec.Command(fx.realGit, "-C", fx.repo, "am", fx.amPatches[0])
	stall.Env = hsEnv(fx)
	if out, err := stall.CombinedOutput(); err == nil {
		t.Fatalf("the mailbox applied cleanly — this fixture needs one that conflicts:\n%s", out)
	}
	applyDir := filepath.Join(fx.repo, ".git", "rebase-apply")
	if _, err := os.Stat(applyDir); err != nil {
		t.Fatalf("a stalled am left no .git/rebase-apply, so --abort has nothing to end: %v", err)
	}

	errs, code := hsRun(t, fx, wrapped, hsLeg{args: "am --abort"})
	if strings.Contains(errs, "hookskip:") {
		t.Fatalf("git am --abort was refused by the hook-skip arm, and AGENTS.md tells seats to type it: %q", errs)
	}
	if code != 0 {
		t.Fatalf("git am --abort: code=%d, want 0 (%s)", code, errs)
	}
	if _, err := os.Stat(applyDir); !os.IsNotExist(err) {
		t.Errorf(".git/rebase-apply survived `git am --abort` through the shim: %v", err)
	}
}

// The measured-residual row for the commit routes this arm does NOT hold
// (ranger-base-o6ip1, from ranger-base-2lls3). Every other pin in this file
// asserts a refusal; this one asserts the opposite, and that is the point:
// HookSkipCommitResidual and the RESIDUALS paragraph tell an operator these
// routes are past the wall, and a disclosure nothing executes drifts from
// the wall it describes. The arm used to disclose "plumbing other than
// commit-tree", which named neither of the two ALL-PORCELAIN routes below.
//
// Each route must: exit 0, not be refused by this arm, leave the managed
// pre-commit unrun, and put a new commit object on a ref. And each must be
// NAMED in HookSkipCommitResidual, so the prose and the measurement cannot
// drift apart.
//
// A refusal cannot hold any of them. cherry-pick, revert and rebase of an
// existing commit are the ordinary way to move one and run no pre-commit by
// git's design; `update-ref` writing a ref is its ordinary job. What is
// refused instead is each route's dedicated spelling, where a commit no hook
// saw is the whole point of the verb: commit-tree (ranger-base-crlty) and am
// (this bead). One runtime, not two: the arm is runtime-independent and
// TestQAHookSkipRefusedOnEveryRuntime already executes it through each, and
// a residual is a fact about git.
func TestQAHookSkipCommitResidualRoutesAreMeasured(t *testing.T) {
	t.Parallel()
	fx := hsSetup(t)
	wrapped, _ := hsWrap(t, fx, "claude", "hs-residual")

	routes := []struct {
		what  string // the route, for the failure message
		names string // the token HookSkipCommitResidual must name it by
		line  string // the sh that performs it, every git shimmed
		proof func(t *testing.T, before string)
	}{{
		what:  "stash push then cherry-pick -m 1 stash@{0}",
		names: "cherry-pick",
		// The route the verify lane predicted would replace a `stash
		// create` refusal (ranger-base-sbfop's residual): two porcelain
		// verbs, no plumbing, and it lands UNCOMMITTED working-tree content.
		line: "echo residual-1 >> f && git stash push -q && git cherry-pick -m 1 stash@{0}",
		proof: func(t *testing.T, before string) {
			if after := hsRepoOut(t, fx, "rev-parse", "HEAD"); after == before {
				t.Error("HEAD did not move, so this route landed nothing and witnesses nothing")
			}
			if s := hsRepoOut(t, fx, "log", "-1", "--format=%s"); !strings.HasPrefix(s, "WIP on ") {
				t.Errorf("HEAD subject %q is not the stash commit this route cherry-picked", s)
			}
		},
	}, {
		what:  "stash create then update-ref",
		names: "stash create",
		line:  "echo residual-2 >> f && c=$(git stash create) && [ -n \"$c\" ] && git update-ref refs/heads/hs-residual \"$c\"",
		proof: func(t *testing.T, before string) {
			if ty := hsRepoOut(t, fx, "cat-file", "-t", "refs/heads/hs-residual"); ty != "commit" {
				t.Errorf("refs/heads/hs-residual is a %q, want a commit", ty)
			}
		},
	}, {
		what:  "replace --graft",
		names: "replace --graft",
		line:  "git replace --graft HEAD",
		proof: func(t *testing.T, before string) {
			if l := hsRepoOut(t, fx, "replace", "-l"); l == "" {
				t.Error("no replace ref, so this route landed nothing and witnesses nothing")
			}
		},
	}}

	for _, r := range routes {
		if !strings.Contains(HookSkipCommitResidual, r.names) {
			t.Errorf("HookSkipCommitResidual does not name %q, so %s is measured here and disclosed nowhere:\n%s", r.names, r.what, HookSkipCommitResidual)
		}
		before := hsRepoOut(t, fx, "rev-parse", "HEAD")
		fired := hsFired(fx)
		errs, code := hsRunLine(t, fx, wrapped, r.line)
		if strings.Contains(errs, "hookskip:") {
			t.Errorf("%s: refused by the hook-skip arm — it is disclosed as a residual, so either the disclosure or the refusal is wrong: %q", r.what, errs)
			continue
		}
		if code != 0 {
			t.Errorf("%s: code=%d, want 0 — a route that no longer runs cannot be the residual the prose claims (%s)", r.what, code, errs)
			continue
		}
		if n := hsFired(fx); n != fired {
			t.Errorf("%s: the managed pre-commit fired %d time(s) — it is no longer a residual, and the disclosure overstates the hole", r.what, n-fired)
		}
		r.proof(t, before)
	}
}

// A verb cannot be both read and hook-running (ranger-base-ap4p6). This is
// the cheap pin on an expensive mistake: a hookSkipVerbs entry that also
// appeared in hookSkipReadVerbs would un-do r3pri in silence — the arm would
// still render, still define every rule, and wave the override through on the
// one verb the whole file exists for. Nothing in the execution pins above
// would say so either, because they assert the refusal and not WHICH list
// produced it.
func TestQAHookSkipReadVerbsAreNotHookVerbs(t *testing.T) {
	t.Parallel()
	read := map[string]bool{}
	for v := range hookSkipReadVerbs {
		read[v] = true
	}
	for v, subs := range hookSkipReadSubverbs {
		if read[v] {
			t.Errorf("%q is in hookSkipReadVerbs AND hookSkipReadSubverbs — the plain arm matches first and the subcommand split is dead", v)
		}
		if len(subs) == 0 {
			t.Errorf("hookSkipReadSubverbs[%q] is empty, so the verb renders an arm that can never match", v)
		}
		read[v] = true
	}
	for _, v := range hookSkipVerbs {
		if read[v] {
			t.Errorf("%q runs a hook a scanner could live in and must never be read as a read", v)
		}
	}
	for _, v := range hookSkipNoVerifyVerbs {
		if read[v] {
			t.Errorf("%q has a --no-verify spoiler, so it runs hooks and must never be read as a read", v)
		}
	}
	// And the rendered sh must actually ask: a refusal that no longer consults
	// posse_hs_readverb is the same regression from the other side.
	// The table holds subcommands only (ranger-base-c1pqo): an option in it
	// is how `remote -v prune` was read as a read.
	for v, subs := range hookSkipReadSubverbs {
		for _, s := range subs {
			if strings.HasPrefix(s, "-") {
				t.Errorf("hookSkipReadSubverbs[%q] lists the option %q — options are skipped, never matched", v, s)
			}
		}
	}
	body := renderHookSkipArm("/usr/bin/git", "/bin/date", "/x/hooks", hookSkipGlobalRef{})
	if !strings.Contains(body, "posse_hs_readverb() {") ||
		!strings.Contains(body, "! posse_hs_readverb \"$@\"") {
		t.Errorf("the -c arm must be judged through posse_hs_readverb, with the verb AND every word after it (ranger-base-c1pqo):\n%s", body)
	}
	// Both tables are maps, so a render that iterated them directly would
	// emit a different shim on every run — and a seat's gates dir is
	// compared against a re-render elsewhere in this package.
	if second := renderHookSkipArm("/usr/bin/git", "/bin/date", "/x/hooks", hookSkipGlobalRef{}); second != body {
		t.Error("renderHookSkipArm is not deterministic — a map is being iterated without sorting")
	}
	for v := range hookSkipReadVerbs {
		if !strings.Contains(body, "'"+v+"'") {
			t.Errorf("hookSkipReadVerbs names %q and the rendered arm does not", v)
		}
	}
}

// Independent of the PID: a persona that denies NOTHING in git still gets a
// git shim, and the arm in it. push -n is allowed there (it is --dry-run)
// and push --no-verify is refused by the arm and not by any deny.
func TestQAHookSkipNeedsNoPIDRule(t *testing.T) {
	t.Parallel()
	fx := hsSetup(t)
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state"), GitScopeEnv: hsEnv(fx)}
	fx.hooksRoot = filepath.Join(a.StateDir, "hooks")
	_, binDir, _, err := a.RenderGates("hs-nodeny", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(binDir, "git")); err != nil {
		t.Fatalf("a PID with no git deny must still get a git shim: %v", err)
	}
	prefix := GatePrefix(binDir, "")
	if errs, code := hsRun(t, fx, prefix, hsLeg{args: "push -n origin main"}); code != 0 {
		t.Errorf("push -n is --dry-run and must run: code=%d %q", code, errs)
	}
	if errs, code := hsRun(t, fx, prefix, hsLeg{args: "push --no-verify origin main"}); code == 0 || !strings.Contains(errs, "rule: hookskip:no-verify,") {
		t.Errorf("push --no-verify must be refused by the arm with no PID rule: code=%d %q", code, errs)
	}
	// The env arm on push, where no deny refuses push first.
	if errs, code := hsRun(t, fx, prefix, hsLeg{env: "GIT_CONFIG_GLOBAL=/dev/null ", args: "push origin main"}); code == 0 || !strings.Contains(errs, "rule: hookskip:config-env,") {
		t.Errorf("GIT_CONFIG_GLOBAL hiding the managed hooksPath must refuse a push: code=%d %q", code, errs)
	}
	// And posse's own redirect is the one core.hooksPath the env may carry.
	// It must be posse's RENDER (ranger-base-crlty): a bare dir at the
	// same path is refused, and the render passes with the managed hook run.
	hsPlantRedirects(t, fx)
	leg := hsLeg{env: hsKey0("core.hooksPath", filepath.Join(fx.hooksRoot, "empty")), args: "commit -q -m r0 -- f"}
	if errs, code := hsRun(t, fx, prefix, leg); code == 0 || !strings.Contains(errs, "rule: hookskip:config-env,") {
		t.Errorf("an empty dir under <StateDir>/hooks is not the D2 redirect: code=%d %q", code, errs)
	}
	before := hsFired(fx)
	leg = hsLeg{env: hsKey0("core.hooksPath", filepath.Join(fx.hooksRoot, "hs-session")), args: "commit -q -m r -- f"}
	if errs, code := hsRun(t, fx, prefix, leg); code != 0 || strings.Contains(errs, "hookskip:") || hsFired(fx) != before+1 {
		t.Errorf("the ADR 0052 D2 redirect must pass and run the managed hook: code=%d fired=%d %q", code, hsFired(fx)-before, errs)
	}
}

// With no git resolved at render, an override the arm cannot judge is
// refused, not waved through: both scope comparisons fail closed.
func TestQAHookSkipScopeFailsClosedWithoutGit(t *testing.T) {
	t.Parallel()
	body := renderHookSkipArm("", "/bin/date", "/x/hooks", hookSkipGlobalRef{})
	if !strings.Contains(body, "posse_hs_scope() {\n  posse_hs_have=$(false config") || !strings.Contains(body, "  return 1\n}\nposse_hs_gscope()") {
		t.Errorf("with no real git the system scope comparison must return 1 (refuse):\n%s", body)
	}
	if !strings.Contains(body, "posse_hs_gscope() {\n  return 1\n}\n") {
		t.Errorf("with no real git the global scope comparison must return 1 (refuse, ADR 0079 D4):\n%s", body)
	}
}

// ranger-base-d4bk4: the PID's verb deny rules judge an alias's expansion,
// not only its token. Before, with real `alias.c = commit` in config, `git c
// -q -m x` under `Bash(git commit unless --)` exited 0 as a bare commit.
// Real git, through the seat launch line, claude and codex.
func TestQAPIDDenyJudgesAliasExpansion(t *testing.T) {
	t.Parallel()
	for _, rtName := range []string{"claude", "codex"} {
		t.Run(rtName, func(t *testing.T) {
			t.Parallel()
			fx := hsSetup(t)
			for k, v := range map[string]string{"c": "commit", "cc": "c", "pp": "push"} {
				hsRealGit(t, fx, "-C", fx.repo, "config", "alias."+k, v)
			}
			home := t.TempDir()
			a := &App{Home: home, StateDir: filepath.Join(home, "state"), GitScopeEnv: hsEnv(fx)}
			rt, err := a.LoadRuntime(rtName)
			if err != nil {
				t.Fatal(err)
			}
			fx.hooksRoot = filepath.Join(a.StateDir, "hooks")
			wrapped, gatesDir, _, err := a.WrapWithGates("ad-"+rtName, rt, []string{"Bash(git push:*)", "Bash(git commit unless --)"}, t.TempDir(), "")
			if err != nil {
				t.Fatal(err)
			}
			for _, leg := range []struct{ args, rule, judged string }{
				{"c -q -m x", "Bash(git commit unless --)", "git commit -q -m x"},
				{"-C . c -q -m x", "Bash(git commit unless --)", "git -C . commit -q -m x"},
				{"cc -q -m x", "Bash(git commit unless --)", "git commit -q -m x"},
				{"c -q -m x --", "Bash(git commit unless --)", "git commit -q -m x --"},
				{"pp origin main", "Bash(git push:*)", "git push origin main"},
			} {
				errs, code := hsRun(t, fx, wrapped, hsLeg{args: leg.args})
				want := "refused by posse gate: " + leg.judged + " (deny: " + leg.rule + ")"
				if code != 1 || !strings.Contains(errs, want) {
					t.Errorf("git %s: code=%d, want 1 and %q:\n%s", leg.args, code, want, errs)
				}
				if !strings.Contains(errs, "is an alias for") || !strings.Contains(errs, "ranger-base-d4bk4") {
					t.Errorf("git %s: the refusal must say an alias was judged by its expansion:\n%s", leg.args, errs)
				}
				// The rule's own hint (its safe form) survives under the alias line.
				if strings.HasPrefix(leg.rule, "Bash(git commit") && !strings.Contains(errs, "safe form: git commit") {
					t.Errorf("git %s: the safe form must still be named:\n%s", leg.args, errs)
				}
			}
			logb, _ := os.ReadFile(filepath.Join(gatesDir, "refusals.log"))
			if n := strings.Count(string(logb), "(deny: Bash(git commit unless --))"); n != 4 {
				t.Errorf("refusals.log: want 4 commit-rule lines, got %d:\n%s", n, logb)
			}
			if hsFired(fx) != 0 {
				t.Errorf("a refused alias reached commit: the hook fired %d times", hsFired(fx))
			}
			// The control: the safe form, through the alias, runs and runs the hook.
			if errs, code := hsRun(t, fx, wrapped, hsLeg{args: "c -q -m ok -- f"}); code != 0 || hsFired(fx) != 1 {
				t.Errorf("git c -q -m ok -- f: code=%d fired=%d, want 0 and 1 (%s)", code, hsFired(fx), errs)
			}
		})
	}
}

// A PID with no verb deny rule has no matcher to re-run: the same alias
// commit runs (the hook-skip arm still resolves it, and it skips nothing).
func TestQAPIDDenyAliasNoVerbRulesIsANoop(t *testing.T) {
	t.Parallel()
	fx := hsSetup(t)
	hsRealGit(t, fx, "-C", fx.repo, "config", "alias.c", "commit")
	home := t.TempDir()
	a := &App{Home: home, StateDir: filepath.Join(home, "state"), GitScopeEnv: hsEnv(fx)}
	fx.hooksRoot = filepath.Join(a.StateDir, "hooks")
	_, binDir, _, err := a.RenderGates("ad-nodeny", nil)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(binDir, "git"))
	if !strings.Contains(string(body), "posse_hs_pid() { :; }") {
		t.Errorf("no verb rules: posse_hs_pid must be the no-op:\n%s", body)
	}
	if errs, code := hsRun(t, fx, GatePrefix(binDir, ""), hsLeg{args: "c -q -a -m z"}); code != 0 || hsFired(fx) != 1 {
		t.Errorf("git c -q -a -m z with no deny: code=%d fired=%d, want 0 and 1 (%s)", code, hsFired(fx), errs)
	}
}

// ASSUMED (ranger-base-d4bk4): a command refused by its alias EXPANSION
// reaches the real git exactly once, for the read-only `config --get
// alias.<verb>` that produced the expansion, and never for its own verb. A
// deny on the typed argv still reaches git zero times. A stub git records
// every exec, the way TestRefusalNamesWhereTheCommandDoesRun's canary does.
func TestQAPIDDenyAliasLookupIsTheOnlyExec(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	dir := t.TempDir()
	execs := filepath.Join(dir, "execs")
	stub := filepath.Join(dir, "git")
	script := "#!/bin/sh\necho \"$*\" >> " + shQuote(execs) + "\ncase \"$*\" in\n" +
		"  'config --get alias.c') echo commit ;;\n  'config --get alias.pp') echo push ;;\n  config\\ --get\\ *) exit 1 ;;\nesac\n"
	if err := WriteExecutable(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	rules := ParseShimRules([]string{"Bash(git push:*)", "Bash(git commit unless --)"})["git"]
	shim := filepath.Join(dir, "shim")
	body := renderShim("ad-canary", "git", stub, filepath.Join(dir, "refusals.log"), "/bin/date", rules, nil, "", filepath.Join(dir, "hooks"), resolveHookSkipGlobalRef(stub, nil))
	if err := WriteExecutable(shim, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_CONFIG") && !strings.HasPrefix(kv, "RHQ_GATE_") {
			env = append(env, kv)
		}
	}
	run := func(args ...string) (int, []string) {
		os.Remove(execs)
		cmd := exec.Command(shim, args...)
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("%v: %v %s", args, err, out)
		}
		b, _ := os.ReadFile(execs)
		return code, strings.Split(strings.TrimSpace(string(b)), "\n")
	}
	for _, c := range []struct {
		args  []string
		code  int
		execs []string
	}{
		{[]string{"c", "-m", "x"}, 1, []string{"config --get alias.c"}},
		{[]string{"pp", "origin"}, 1, []string{"config --get alias.pp"}},
		{[]string{"commit", "-m", "x"}, 1, []string{""}},
		{[]string{"push"}, 1, []string{""}},
		// Allowed: the lookup, then git itself with the argv as typed.
		{[]string{"c", "-m", "x", "--", "f"}, 0, []string{"config --get alias.c", "c -m x -- f"}},
		{[]string{"nope"}, 0, []string{"config --get alias.nope", "nope"}},
	} {
		code, got := run(c.args...)
		if code != c.code || strings.Join(got, "|") != strings.Join(c.execs, "|") {
			t.Errorf("git %v: code=%d execs=%q, want %d and %q", c.args, code, got, c.code, c.execs)
		}
	}
}
