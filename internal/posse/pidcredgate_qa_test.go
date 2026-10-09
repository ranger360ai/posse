//go:build posse_arm2

package posse

// ranger-base-sl5sg (github.com/ranger360ai/posse issue #6, MEASURED on a
// real install 2026-10-08): a claude PID scaffolded with the crew's deny set
// — `Bash(security:*)` among it — and no `envs:` cannot launch at all. ADR
// 0042 D2's precondition refuses it, correctly and unwaivably, naming the
// rule, the binary, the key and the recipe.
//
// THE DEFECT was that nothing said so BEFORE the launch. `posse agent check`
// read every other launch precondition off the file (sockets a cage tier
// cannot mount, a path-scoped write at `shims`, a tier under its own floor)
// and had no rule for this one, so the PID linted clean and the operator met
// the refusal at the PID's first dispatch. The scaffold's own `envs:` comment
// was silent too.
//
// WHAT IS PINNED HERE: the lint fires on exactly that state, goes quiet the
// moment the mint is in a set the PID names, is never asked of a PID that
// shims nothing of the runtime's, and is a WARNING — the env store is
// machine-local and never in the repo, so a finding would red every instance
// repo's CI over a PID that is correct.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cglHome is a posse home with one PID and one env set, both written by the
// caller: deny is the PID's deny: list, envs its envs:, and body the env
// set's contents (empty = no set file at all, the state a PID naming a set
// this box does not have is in).
func cglHome(t *testing.T, deny, envs, body string) *App {
	t.Helper()
	home := t.TempDir()
	a := &App{
		Home:       home,
		AgentsDir:  filepath.Join(home, "agents"),
		EnvsDir:    filepath.Join(home, "envs"),
		StateDir:   filepath.Join(home, "state"),
		ConfigPath: filepath.Join(home, "config.yaml"),
	}
	if err := os.MkdirAll(a.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(a.EnvsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	front := "---\nname: ranger\nruntime: claude\n"
	if deny != "" {
		front += "deny: [" + deny + "]\n"
	}
	if envs != "" {
		front += "envs: [" + envs + "]\n"
	}
	if err := os.WriteFile(filepath.Join(a.AgentsDir, "ranger.md"), []byte(front+"---\nYou are Ranger, the scout of the crew.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if body != "" {
		if err := os.WriteFile(filepath.Join(a.EnvsDir, "crew.env"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return a
}

// cglRuntime is the default runtime, with the two facts this whole file
// rests on asserted rather than assumed: it declares a credential binary and
// that binary is on this box. Without both the collision cannot exist and
// every arm below would be measuring silence for the wrong reason.
func cglRuntime(t *testing.T, a *App) *Runtime {
	t.Helper()
	rt, err := a.LoadRuntime(DefaultRuntime)
	if err != nil {
		t.Fatal(err)
	}
	if rt.CredBin == "" {
		t.Skipf("%s declares no credential binary", rt.Name)
	}
	if resolveOutside(rt.CredBin, filepath.Join(a.GatesDir("ranger"), "bin")) == "" {
		t.Skipf("%s is not on this box outside the gates", rt.CredBin)
	}
	return rt
}

// cglWarning is the one warning this file is about, "" when the lint is
// quiet. Selected by the key rather than by position, so an unrelated
// warning gained by the linter does not red these arms.
func cglWarning(t *testing.T, a *App, key string) string {
	t.Helper()
	fs, ws, err := a.CheckAgent("ranger")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fs {
		if strings.Contains(f, key) {
			t.Errorf("the credential precondition must be a WARNING, not a finding — a finding reds an instance repo's CI, which has no env store to read: %s", f)
		}
	}
	for _, w := range ws {
		if strings.Contains(w, key) {
			return w
		}
	}
	return ""
}

// The defect itself. The PID carries the crew's rule over the runtime's own
// credential binary and names no env set, so no launch of it can carry the
// mint — and the lint says so, naming the four things the refusal names.
func TestQAAgentCheckWarnsWhenAShimmedPIDNamesNoEnvSet(t *testing.T) {
	t.Parallel()
	a := cglHome(t, "Bash(security:*)", "", "")
	rt := cglRuntime(t, a)
	key := CageCredential(rt)

	got := cglWarning(t, a, key)
	if got == "" {
		t.Fatalf("a claude PID denying %s with no envs: cannot authenticate at launch and the lint must say so", rt.CredBin)
	}
	for _, want := range []string{"Bash(security:*)", rt.CredBin, key, "names no env set", "ADR 0042 D2", "--allow-degraded", "setup-token"} {
		if !strings.Contains(got, want) {
			t.Errorf("the warning never names %q:\n%s", want, got)
		}
	}
	// Never the advice ADR 0042 D1 forbids: the shim in front of the
	// runtime's read is what keeps the operator's rotating pair
	// single-writer, so "drop the rule" is the one fix this must not offer
	// — it is the sentence ranger-base-eupf's retired warning carried.
	if strings.Contains(got, "drop the rule") && !strings.Contains(got, "never dropping the rule") {
		t.Errorf("the warning offers dropping the rule, which ADR 0042 D1 forbids:\n%s", got)
	}
}

// The quiet arm that keeps the one above honest: the mint is in the set the
// PID names, which IS the design (ADR 0042 D1), so the lint says nothing.
func TestQAAgentCheckIsQuietWhenTheShimmedPIDNamesTheMint(t *testing.T) {
	t.Parallel()
	a := cglHome(t, "Bash(security:*)", "crew", "")
	rt := cglRuntime(t, a)
	key := CageCredential(rt)
	if err := os.WriteFile(filepath.Join(a.EnvsDir, "crew.env"), []byte("FOO=bar\n"+key+"=sk-ant-oat01-TEST\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if got := cglWarning(t, a, key); got != "" {
		t.Fatalf("the mint is in the set the PID names — that is the design and the lint must be silent:\n%s", got)
	}
}

// The set is named and carries other names: the operator minted nothing, or
// edited it out. The lint fires and names the set, because "add the key to
// crew" and "name a set at all" are different next moves.
func TestQAAgentCheckNamesTheEnvSetThatCarriesNoMint(t *testing.T) {
	t.Parallel()
	a := cglHome(t, "Bash(security:*)", "crew", "FOO=bar\nGH_TOKEN=y\n")
	rt := cglRuntime(t, a)

	got := cglWarning(t, a, CageCredential(rt))
	if !strings.Contains(got, "no env set it names carries it (crew)") {
		t.Errorf("want the named set reported:\n%s", got)
	}
	// Names only. An env set's contents are the one thing `envs:` promises
	// never reach a terminal (rangerhq-f2b), and a lint that listed what it
	// DID find would print them on every run.
	for _, leak := range []string{"FOO", "GH_TOKEN", "bar", "=y"} {
		if strings.Contains(got, leak) {
			t.Errorf("the warning echoed %q out of the env set:\n%s", leak, got)
		}
	}
}

// A set the PID names that this box does not have is a launch refusal of its
// own ("env set not found"), which minting fixes nothing of — so the lint
// says that instead of claiming the key is absent from a file it never read.
func TestQAAgentCheckSaysWhenTheNamedEnvSetIsNotOnThisBox(t *testing.T) {
	t.Parallel()
	a := cglHome(t, "Bash(security:*)", "crew", "")
	rt := cglRuntime(t, a)

	got := cglWarning(t, a, CageCredential(rt))
	if !strings.Contains(got, "none of the env sets it names is on this box (crew)") {
		t.Errorf("want the unreadable set reported as unreadable:\n%s", got)
	}
}

// The silence from the other side, and the arm that fails if the lint
// forgets to ask the collision first: a PID that shims nothing of the
// runtime's has never needed a mint, and must not start being warned about
// one — that is every PID on a box where no crew deny is in play.
func TestQAAgentCheckDoesNotAskForAMintWithoutTheCollision(t *testing.T) {
	t.Parallel()
	for _, deny := range []string{"", "Bash(git push:*)"} {
		a := cglHome(t, deny, "", "")
		rt := cglRuntime(t, a)
		if got := cglWarning(t, a, CageCredential(rt)); got != "" {
			t.Errorf("deny: [%s] shims nothing of the runtime's and must draw no credential line:\n%s", deny, got)
		}
	}
}

// The lint writes nothing. `posse agent check` is read-only — it is run in
// CI and inside a cage, where the gates dir is not writable — and the bin
// dir it hands CredGateCollision is NAMED, never rendered (the shape
// GateShellOn exists for, ranger-base-zbg8o).
func TestQAAgentCheckRendersNoGatesDir(t *testing.T) {
	t.Parallel()
	a := cglHome(t, "Bash(security:*)", "", "")
	cglRuntime(t, a)

	if _, _, err := a.CheckAgent("ranger"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(a.GatesDir("ranger")); !os.IsNotExist(err) {
		t.Errorf("the lint rendered a gates dir: %v", err)
	}
}

// The scaffold's own comment (the second half of the ask). A new persona is
// written by reading this file, so the key the launch requires has to be
// named where the deny that requires it is written.
func TestQAScaffoldCommentNamesTheCredentialPrecondition(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	a := &App{Home: home, AgentsDir: filepath.Join(home, "agents")}
	p, err := a.ScaffoldAgent("scout")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	for _, want := range []string{"envs:", "CLAUDE_CODE_OAUTH_TOKEN", "security:*", "ADR 0042 D2", "setup-token"} {
		if !strings.Contains(got, want) {
			t.Errorf("the scaffold never names %q — a PID written from it can be unlaunchable with nothing saying so", want)
		}
	}
	// It must still be a clean PID: the sentence is a comment, and a
	// scaffold that lints dirty is a scaffold nobody trusts.
	a.ConfigPath = filepath.Join(home, "config.yaml")
	fs, _, err := a.CheckAgent("scout")
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 0 {
		t.Errorf("the scaffold has findings: %v", fs)
	}
}

// EnvSetKeyNames is the reader the lint rests on, and the reason it is a
// function rather than a loop at the caller: it returns names, so no caller
// can print a value it never received.
func TestQAEnvSetKeyNamesReturnsNamesAndNeverValues(t *testing.T) {
	t.Parallel()
	a := cglHome(t, "", "crew", "FOO=bar\n# a comment\nexport TOK=sk-ant-secret\n\nFOO=again\n")

	keys, unreadable := a.EnvSetKeyNames([]string{"crew", "gone"})
	if strings.Join(keys, ",") != "FOO,TOK" {
		t.Errorf("want the two key names, deduped, in file order: %v", keys)
	}
	if strings.Join(unreadable, ",") != "gone" {
		t.Errorf("want the set this box does not have reported separately: %v", unreadable)
	}
	for _, v := range keys {
		if strings.Contains(v, "=") || strings.Contains(v, "sk-ant") {
			t.Errorf("a value came back from a key reader: %q", v)
		}
	}
}

// ─── the undecided-credential sentence (ranger-base-7ebv6) ───────────────────

// CredGateLint has two sentences and the eight arms above reach one of them.
// The other — "no session credential is decided for <runtime>" — is the arm
// pidcheck.go selects with `if key := CageCredential(own); key == ""`, and
// ranger-base-751ha found that mutating that guard to `false` survives all
// eight: nothing in the package calls CredGateLint directly, so the sentence
// was unwritten-about text.
//
// WHY THE MUTANT SURVIVES, MEASURED HERE RATHER THAN ARGUED: the guard is
// unreachable at HEAD. CredGateCollision returns "" unless the runtime
// declares a CredBin, exactly one runtime does (the claude built-in,
// `security`), and its session credential is a const in cageCredential — so
// `key` is never empty where the guard is asked, and the two branches are the
// same branch. That is a no-op mutant and not an unheld property, which is a
// different thing from the other four findings in that bead.
//
// So the two things that CAN fail are pinned instead: the sentence itself,
// asked of the function with a runtime in that state, and the reachability
// premise — the day a runtime declares a credential binary with no decided
// credential, the arm goes live and wants a behavioural pin through
// CheckAgent. The second test below says so in its failure message.
func TestQACredGateLintSaysWhichPreconditionIsUnmet(t *testing.T) {
	t.Parallel()
	// Not LoadRuntime: the state under test is one no loadable runtime is in
	// (see the test below), and that is the point — this pins the words the
	// lint would use the day one is.
	undecided := &Runtime{Name: "newcli", CredBin: "newcli-auth"}
	got := CredGateLint(undecided, "Bash(newcli-auth:*)", "")
	for _, want := range []string{
		"Bash(newcli-auth:*)", "newcli-auth", "no session credential is decided for newcli",
		"every launch of this PID refuses", "cage_cred:", "ADR 0042 D1/D2",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the undecided-credential line never names %q:\n%s", want, got)
		}
	}
	// It must not print the OTHER sentence, whose shape needs a key name:
	// with none decided, that one reads "launches only with  among the
	// env-set names it injects" — a sentence with a hole in it, which is
	// what the mutated guard produces.
	if strings.Contains(got, "among the env-set names it injects") {
		t.Errorf("the undecided state printed the key-name sentence, with no key to name:\n%s", got)
	}
	// And the decided state still gets that other sentence, naming the key:
	// one function, two voices, and neither may answer for the other.
	decided := &Runtime{Name: "newcli", CredBin: "newcli-auth", CageCred: "NEWCLI_TOKEN"}
	other := CredGateLint(decided, "Bash(newcli-auth:*)", "it names no env set")
	for _, want := range []string{"NEWCLI_TOKEN", "among the env-set names it injects", "it names no env set"} {
		if !strings.Contains(other, want) {
			t.Errorf("the decided line never names %q:\n%s", want, other)
		}
	}
	if strings.Contains(other, "no session credential is decided") {
		t.Errorf("a decided credential printed the undecided sentence:\n%s", other)
	}
}

// The premise the paragraph above rests on, so it cannot rot in silence: no
// runtime this box can load declares a credential binary without a decided
// session credential. While that holds, pidcheck's `key == ""` arm is
// unreachable and a mutation of its guard changes nothing. When it stops
// holding, this test reds and names what the next hand owes.
//
// MEASURED 2026-10-09: one built-in declares a CredBin (claude), and a
// template-only runtime cannot declare one at all — there is no `cred_bin:`
// yaml key, and the arm below is what says so rather than a grep that
// tomorrow's reader has to repeat.
func TestQANoLoadableRuntimeDeclaresACredBinWithNoDecidedCredential(t *testing.T) {
	t.Parallel()
	const owed = "pidcheck.go's `key == \"\"` arm is LIVE now: give it a behavioural pin through CheckAgent (the warning text is TestQACredGateLintSaysWhichPreconditionIsUnmet's) instead of the no-op mutant ranger-base-751ha measured"
	a := cglHome(t, "Bash(security:*)", "", "")

	declaring := 0
	for i := range builtinRuntimes {
		rt := builtinRuntimes[i]
		if rt.CredBin == "" {
			continue
		}
		declaring++
		if CageCredential(&rt) == "" {
			t.Errorf("built-in runtime %s declares cred_bin %s and no session credential — %s", rt.Name, rt.CredBin, owed)
		}
	}
	if declaring == 0 {
		t.Fatalf("CONTROL: no built-in declares a credential binary at all, so this test measured nothing — and neither does the whole credential precondition (ADR 0042 D2)")
	}

	// A template-only runtime, declaring the key as an operator would if it
	// existed. It does not, so CredBin stays empty and CredGateCollision
	// short-circuits before the guard is ever asked.
	if err := os.MkdirAll(a.RuntimesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := "command: newcli {prompt}\ncred_bin: newcli-auth\n"
	if err := os.WriteFile(filepath.Join(a.RuntimesDir(), "newcli.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	rt, err := a.LoadRuntime("newcli")
	if err != nil {
		t.Fatalf("loading a template-only runtime: %v", err)
	}
	if rt.CredBin != "" {
		t.Errorf("a runtime yaml can declare cred_bin: %s now, and this one decides no credential — %s", rt.CredBin, owed)
	}
}
