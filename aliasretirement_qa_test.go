package posse

// QA pins for ranger-base-igup (verified under ranger-base-jq5t).
//
// Claim: the `rhq` transition alias (rangerhq-tyay) is retired from the BUILD
// — `make install` no longer writes $(BINDIR)/rhq, `make link-plugin` no
// longer writes plugin/bin/rhq, and plugin/autostart.sh no longer arms the
// fleet's dispatch loop through the old spelling.
//
// Why this needs a pin rather than a comment: the whole point of the change is
// that deleting the two live inodes inside the retirement window
// (ranger-base-3rv9 step 4 / ranger-base-6y83) is a RETIREMENT and not a pause.
// It is a pause again the moment one `ln -sfn` comes back — and nothing else in
// the suite reads those two recipes, so the regression would land green. The
// Makefile's own comment blocks are the record of WHY; this is the record that
// the recipes still agree with them.
//
// Measured on 2026-08-27, and the reason the justification that used to keep
// plugin/bin/rhq is gone: the live loop, its pidfile and `ps` all name
// `<repo>/plugin/bin/posse dispatch --watch`; the plugin manifest runs
// ./bin/posse; autostart.sh's own default is $here/bin/posse; and no herdr
// config, shell profile or session recipe on the box invokes the alias.

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestBuildWritesNoRhqInode is DONE clause 1 of ranger-base-igup, read off the
// shipped recipes rather than off a rig: `make install && make link-plugin`
// creates `posse` and `plugin/bin/posse` and no `rhq` inode anywhere.
func TestBuildWritesNoRhqInode(t *testing.T) {
	makefile, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"install", "link-plugin"} {
		lines := makeRecipe(string(makefile), target)
		if len(lines) == 0 {
			t.Fatalf("no recipe found for target %q — the pin is reading nothing", target)
		}
		if err := recipeWritesNoRhq(lines); err != nil {
			t.Errorf("make %s: %v", target, err)
		}
	}
	// The positive half. A pin that only forbids a name passes just as well on
	// a Makefile that stopped writing the binary at all.
	for _, want := range []struct{ target, frag string }{
		{"install", "$(BINDIR)/posse"},
		{"link-plugin", "plugin/bin/posse"},
	} {
		if !strings.Contains(strings.Join(makeRecipe(string(makefile), want.target), "\n"), want.frag) {
			t.Errorf("make %s no longer writes %s — the retirement must not take the surviving name with it",
				want.target, want.frag)
		}
	}
}

// TestAutostartDoesNotArmThroughTheAlias is the other half of the retirement.
// The fallback that used to live here armed the loop that dispatches everything
// else off `$here/bin/rhq` when bin/posse was missing; once the build stops
// writing that name the arm can only fire on a pre-retirement plugin dir, and
// arming a fleet loop off a stale link is worse than failing loudly.
func TestAutostartDoesNotArmThroughTheAlias(t *testing.T) {
	sh, err := os.ReadFile("plugin/autostart.sh")
	if err != nil {
		t.Fatal(err)
	}
	if err := shellCodeMentionsNoRhqBin(string(sh)); err != nil {
		t.Errorf("plugin/autostart.sh: %v", err)
	}
	// The loud failure is the replacement, so pin that it is still there.
	if !strings.Contains(string(sh), `say "no posse at $RHQ — run 'make link-plugin'" >&2`) {
		t.Error("plugin/autostart.sh no longer refuses loudly when bin/posse is missing — dropping the alias arm only helps if the refusal is what follows it")
	}
}

// TestRhqRetirementPinsRejectTheHistoricalShapes is the mutation half: a pin is
// only as strong as what it rejects, and the shapes below are the ones that
// actually shipped before ranger-base-igup.
func TestRhqRetirementPinsRejectTheHistoricalShapes(t *testing.T) {
	t.Run("install target's alias line", func(t *testing.T) {
		before := []string{
			"install -d $(BINDIR)",
			"install -m 0755 bin/posse-release $(BINDIR)/posse",
			"ln -sfn posse $(BINDIR)/rhq",
		}
		if err := recipeWritesNoRhq(before); err == nil {
			t.Error("the pre-retirement install recipe must fail the pin")
		}
	})
	t.Run("link-plugin target's alias line", func(t *testing.T) {
		before := []string{
			"mkdir -p plugin/bin",
			"ln -sfn $(BINDIR)/posse plugin/bin/posse",
			"ln -sfn $(BINDIR)/posse plugin/bin/rhq",
		}
		if err := recipeWritesNoRhq(before); err == nil {
			t.Error("the pre-retirement link-plugin recipe must fail the pin")
		}
	})
	t.Run("a differently-spelled alias still counts", func(t *testing.T) {
		if err := recipeWritesNoRhq([]string{"cp bin/posse-release $(BINDIR)/rhq"}); err == nil {
			t.Error("the pin must be about the NAME reaching the binary, not about `ln`")
		}
	})
	t.Run("the alias named only in a comment is prose", func(t *testing.T) {
		if err := recipeWritesNoRhq([]string{"# plugin/bin/rhq is no longer written here", "ln -sfn $(BINDIR)/posse plugin/bin/posse"}); err != nil {
			t.Errorf("a recipe comment recording the retirement must not fail it: %v", err)
		}
	})
	t.Run("autostart's old fallback arm", func(t *testing.T) {
		before := "if [ ! -x \"$RHQ\" ] && [ -x \"$here/bin/rhq\" ]; then\n\tRHQ=$here/bin/rhq\nfi\n"
		if err := shellCodeMentionsNoRhqBin(before); err == nil {
			t.Error("the pre-retirement autostart fallback must fail the pin")
		}
	})
	t.Run("autostart's retirement comment is prose", func(t *testing.T) {
		after := "# There used to be a fallback here that armed through bin/rhq.\nif [ ! -x \"$RHQ\" ]; then\n\texit 1\nfi\n"
		if err := shellCodeMentionsNoRhqBin(after); err != nil {
			t.Errorf("the comment recording the retirement must not fail it: %v", err)
		}
	})
}

// makeRecipe returns the recipe lines of one Makefile target, tabs stripped.
// A recipe is the tab-indented run beginning at the target line; the comment
// blocks above a target are prose and are not part of it.
func makeRecipe(makefile, target string) []string {
	var out []string
	in := false
	for _, line := range strings.Split(makefile, "\n") {
		if strings.HasPrefix(line, target+":") {
			in = true
			continue
		}
		if !in {
			continue
		}
		if !strings.HasPrefix(line, "\t") {
			break
		}
		out = append(out, strings.TrimPrefix(line, "\t"))
	}
	return out
}

// recipeWritesNoRhq reports whether any recipe line names a path whose last
// element is `rhq`. Deliberately about the NAME and not about `ln`: what the
// retirement forbids is a second name reaching this binary, however it is
// written (internal/rhq/gates.go — every permission layer matches the typed
// word, so a second name is a second command).
func recipeWritesNoRhq(lines []string) error {
	for _, line := range lines {
		code := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "@"))
		if strings.HasPrefix(code, "#") {
			continue
		}
		for _, tok := range strings.Fields(code) {
			tok = strings.Trim(tok, `"'`)
			if tok == "rhq" || strings.HasSuffix(tok, "/rhq") {
				return fmt.Errorf("recipe line %q writes the retired `rhq` alias — deleting the live inodes inside the window is a pause, not a retirement, while a build puts it back (ranger-base-igup)", strings.TrimSpace(line))
			}
		}
	}
	return nil
}

// shellCodeMentionsNoRhqBin reports whether any non-comment line of a shell
// script names bin/rhq. The retirement is recorded in autostart.sh's comments
// on purpose, so only code counts.
func shellCodeMentionsNoRhqBin(script string) error {
	for _, line := range strings.Split(script, "\n") {
		code := strings.TrimSpace(line)
		if code == "" || strings.HasPrefix(code, "#") {
			continue
		}
		if strings.Contains(code, "bin/rhq") {
			return fmt.Errorf("line %q still resolves the fleet binary through the retired alias (ranger-base-igup)", code)
		}
	}
	return nil
}
