package posse

// QA pins for ranger-base-qqxm (security's verification of ranger-base-u9at):
// release.yml must never hand an attacker-influenced value to a shell as
// SCRIPT TEXT, and its tag guard must be an allowlist rather than a prefix
// check.
//
// Two independently reachable sinks, both closed here:
//
//  1. `tag="${{ inputs.tag }}"` — GitHub substitutes a ${{ }} into the run:
//     script before bash parses it, so a dispatched tag `v1"; id; :"` ran at
//     that line, ahead of every check below it, in a job holding
//     contents: write.
//  2. The downstream `${{ steps.tag.outputs.tag }}` interpolations. Those
//     carried the *validated* tag, but `case v[0-9]*)` validated a prefix:
//     `git check-ref-format` accepts `v1.0$(id)` as a real tag name, so
//     pushing one reached `gh release create "v1.0$(id)"` on the ordinary
//     push trigger, never touching inputs.tag at all.
//
// The first pin is an absence, so it carries a positive witness (how many run
// blocks it actually scanned, and a synthetic workflow it must flag). The
// second runs the guard rather than reading it, with a pre-fix control that
// must let the payload execute — otherwise a green result means nothing.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The payloads security measured, plus a newline (a workflow_dispatch input can
// carry one; `tag=v0.3.0\nid` in $GITHUB_OUTPUT is its own forgery) and the
// shapes that are merely wrong rather than hostile.
var releaseBadTags = []string{
	`v1"; id; :"`,
	`v0.3.0$(id)`,
	"v1`id`",
	`v9;curl evil`,
	`v1.0&&id`,
	`v1.0|id`,
	`v1.0 `,
	`v1.`,
	`1.0`,
	`main`,
	"v0.3.0\nid",
	`v0.3.0-rc1`, // not hostile, but not the shape the guard admits either
}

type ghRunBlock struct {
	step string
	line int // 1-indexed line of the `run:` key
	body string
}

// ghRunBlocks returns every `run:` block in a workflow, single-line and
// literal-block form alike. It deliberately starts at the `run:` key: the YAML
// comments and `env:` values above a step are not script text and are where
// the ${{ }} are supposed to live now.
func ghRunBlocks(yaml string) []ghRunBlock {
	lines := strings.Split(yaml, "\n")
	var blocks []ghRunBlock
	step := "(unnamed)"
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if name, ok := strings.CutPrefix(trimmed, "- name:"); ok {
			step = strings.TrimSpace(name)
			continue
		}
		rest, ok := strings.CutPrefix(trimmed, "run:")
		if !ok {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		rest = strings.TrimSpace(rest)
		if rest != "" && !strings.HasPrefix(rest, "|") && !strings.HasPrefix(rest, ">") {
			blocks = append(blocks, ghRunBlock{step: step, line: i + 1, body: rest})
			continue
		}
		var body []string
		for j := i + 1; j < len(lines); j++ {
			next := lines[j]
			if strings.TrimSpace(next) == "" {
				body = append(body, next)
				continue
			}
			if len(next)-len(strings.TrimLeft(next, " ")) <= indent {
				break
			}
			body = append(body, next)
			i = j
		}
		blocks = append(blocks, ghRunBlock{step: step, line: i + 1, body: strings.Join(body, "\n")})
	}
	return blocks
}

func releaseWorkflow(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestReleaseWorkflowNeverExpandsAnExpressionIntoAShell(t *testing.T) {
	blocks := ghRunBlocks(releaseWorkflow(t))

	// The positive witness. An assertion of pure absence is satisfied by
	// measuring nothing, so say out loud what was measured, and fail if the
	// steps that must be covered went missing from the scan.
	seen := map[string]bool{}
	for _, b := range blocks {
		seen[b.step] = true
	}
	for _, want := range []string{
		"resolve the tag", "vet", "test", "build artifacts",
		"render the Homebrew formula", "draft the release", "what happens next",
	} {
		if !seen[want] {
			t.Fatalf("scanned %d run blocks and step %q was not among them — the scan, not the workflow, is what is broken", len(blocks), want)
		}
	}
	t.Logf("scanned %d run: blocks", len(blocks))

	for _, b := range blocks {
		if strings.Contains(b.body, "${{") {
			t.Errorf("release.yml:%d step %q expands a ${{ }} inside a run: block.\n"+
				"GitHub substitutes it into the script TEXT before bash parses it, so the value is code, not data.\n"+
				"Pass it through the step's env: and reference \"$VAR\" (ranger-base-qqxm).\n%s", b.line, b.step, b.body)
		}
	}

	// The control: the scanner must actually flag an injection, in both the
	// literal-block and the single-line form.
	t.Run("control: an injected workflow is flagged", func(t *testing.T) {
		const bad = `jobs:
  release:
    steps:
      - name: block form
        run: |
          tag="${{ inputs.tag }}"
      - name: one-liner form
        run: ship.sh --version "${{ steps.tag.outputs.tag }}"
`
		got := ghRunBlocks(bad)
		if len(got) != 2 {
			t.Fatalf("scanner found %d run blocks in the control, want 2: %+v", len(got), got)
		}
		for _, b := range got {
			if !strings.Contains(b.body, "${{") {
				t.Errorf("scanner lost the injection in step %q: %q", b.step, b.body)
			}
		}
	})
}

// The values still have to REACH the shell, or "no ${{ } in a run block" is
// satisfied by deleting the feature.
func TestReleaseWorkflowPassesTheTagThroughEnv(t *testing.T) {
	body := releaseWorkflow(t)
	for _, want := range []string{
		"INPUT_TAG: ${{ inputs.tag }}",
		"TAG: ${{ steps.tag.outputs.tag }}",
		"SHA: ${{ steps.tag.outputs.sha }}",
		`tag="$INPUT_TAG"`,
		"shell: bash", // `[[ =~ ]]` is not POSIX; the guard needs the shell it was measured in
		`gh release create "$TAG"`,
		`--target "$SHA"`,
		`scripts/release-artifacts.sh --rev "$SHA" --version "$TAG"`,
		`scripts/tap-formula.sh --version "$TAG"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("release.yml no longer carries %q — the tag has to reach the shell, just as data", want)
		}
	}
}

// resolveTagScript is the `resolve the tag` step's script, lifted out of the
// workflow verbatim. Reading the guard is not evidence; this is what CI runs.
func resolveTagScript(t *testing.T) string {
	t.Helper()
	for _, b := range ghRunBlocks(releaseWorkflow(t)) {
		if b.step == "resolve the tag" {
			return b.body
		}
	}
	t.Fatal("release.yml has no step named \"resolve the tag\"")
	return ""
}

// runResolveTag executes a script the way GitHub's default shell does
// (`bash -eo pipefail`), with $GITHUB_OUTPUT pointed at a scratch file.
func runResolveTag(t *testing.T, script, inputTag string) (exit int, out string, ghOutput string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "resolve.sh")
	if err := os.WriteFile(path, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "github_output")
	if err := os.WriteFile(outPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "-eo", "pipefail", path)
	cmd.Env = append(os.Environ(),
		"INPUT_TAG="+inputTag,
		"GITHUB_REF_NAME=",
		"GITHUB_OUTPUT="+outPath,
	)
	combined, err := cmd.CombinedOutput()
	if err == nil {
		exit = 0
	} else if ee, ok := err.(*exec.ExitError); ok {
		exit = ee.ExitCode()
	} else {
		t.Fatalf("bash: %v\n%s", err, combined)
	}
	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	return exit, string(combined), string(b)
}

func TestReleaseTagGuardRefusesEveryNonVersionString(t *testing.T) {
	script := resolveTagScript(t)

	for _, bad := range releaseBadTags {
		t.Run(fmt.Sprintf("%q", bad), func(t *testing.T) {
			exit, out, ghOut := runResolveTag(t, script, bad)
			if exit == 0 {
				t.Errorf("guard ACCEPTED %q (exit 0) — that string reaches `gh release create` and five other shells\n%s", bad, out)
			}
			if strings.Contains(ghOut, "tag=") {
				t.Errorf("guard wrote %q to $GITHUB_OUTPUT for %q", strings.TrimSpace(ghOut), bad)
			}
			if !strings.Contains(out, "refusing to build a non-version ref") {
				t.Errorf("refusal for %q must say why:\n%s", bad, out)
			}
		})
	}

	// The guard is not the only claim: the value must never be evaluated on
	// its way TO the guard. A payload that runs `touch` proves it by leaving
	// nothing behind.
	t.Run("the payload is data, never code", func(t *testing.T) {
		canary := filepath.Join(t.TempDir(), "pwned")
		for _, payload := range []string{
			`v0.3.0$(touch ` + canary + `)`,
			"v0.3.0`touch " + canary + "`",
			`v0.3.0"; touch ` + canary + `; :"`,
		} {
			if exit, out, _ := runResolveTag(t, script, payload); exit == 0 {
				t.Errorf("guard accepted %q\n%s", payload, out)
			}
			if _, err := os.Stat(canary); err == nil {
				t.Fatalf("payload %q EXECUTED: %s exists", payload, canary)
			}
		}

		// The control. Substitute the payload into the script the way GitHub
		// substitutes a ${{ }} — this is the pre-fix shape — and the same
		// canary must fire. Without this arm, "the file is absent" is also
		// what a test that measures nothing reports.
		payload := `v0.3.0$(touch ` + canary + `)`
		preFix := strings.Replace(script, `tag="$INPUT_TAG"`, `tag="`+payload+`"`, 1)
		if preFix == script {
			t.Fatal(`control could not build the pre-fix shape: no ` + "`" + `tag="$INPUT_TAG"` + "`" + ` in the step`)
		}
		runResolveTag(t, preFix, "")
		if _, err := os.Stat(canary); err != nil {
			t.Fatalf("control did not execute the payload, so this test proves nothing about the fix: %v", err)
		}
	})
}

func TestReleaseTagGuardStillAcceptsARealVersion(t *testing.T) {
	script := resolveTagScript(t)
	for _, good := range []string{"v0.3.0", "v1", "v1.2", "v10.20.30"} {
		_, out, ghOut := runResolveTag(t, script, good)
		// `git rev-parse` on the line after may or may not resolve here; the
		// guard's verdict is the `tag=` line, written before it.
		if !strings.Contains(ghOut, "tag="+good+"\n") {
			t.Errorf("guard refused the legitimate version %q\noutput: %q\n%s", good, ghOut, out)
		}
	}
}
