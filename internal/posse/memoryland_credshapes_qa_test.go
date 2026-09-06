//go:build posse_arm2

package posse

import (
	"strings"
	"testing"
)

// The corpus behind memoryCredShapes, kept as a table so a future narrowing
// of any one pattern shows up as a red rather than as a commit that quietly
// stopped being scanned (ranger-base-vd1bo).
//
// Fifteen of these nineteen credential lines passed the pre-vd1bo table.
// Every miss had one of two causes, and both are spelling rather than
// exotica: \b never fires inside an env-var name because underscore is a
// word character (GH_TOKEN=, AWS_SECRET_ACCESS_KEY=, client_secret=,
// refresh_token=), and the JSON form puts a quote between the key and its
// colon. The rest are vendor prefixes for runtimes this fleet runs and the
// old table only knew sk-ant.
//
// The values are fixtures with no secret in them — AAAA… runs and AWS's own
// published EXAMPLE key — which is the point: the scan is keyed on SHAPE,
// so a real value is never needed to pin it and never belongs in a test.
func TestMemoryCredShapesCatchTheFormsACredentialArrivesIn(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		line string
		want string
	}{
		// env-var spellings: the \b defect, all four of them
		{`GH_TOKEN=ghp_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA`, "an assigned secret"},
		{`GITHUB_TOKEN=github_pat_11ABCDEFG0aaaaaaaaaaaaaaaaaaaaaaaaaaaa`, "an assigned secret"},
		{`export AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY`, "an assigned secret"},
		{`client_secret=abcdefghijklmnopqrstuvwxyz012345`, "an assigned secret"},
		{`refresh_token=e0ijQ0lrbXo3ZmJhc2VkMzJjaGFyc3ZhbA`, "an assigned secret"},
		// JSON and quoted YAML: a quote between the key and the colon
		{`"api_key": "0123456789abcdef0123456789abcdef"`, "an assigned secret"},
		{`  "password": "correct-horse-battery-staple-42"`, "an assigned secret"},
		{`api-key: 0123456789abcdef0123456789abcdef`, "an assigned secret"},
		// vendor values behind a key word: caught as the assignment they are
		{`OPENAI_API_KEY=sk-proj-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA`, "an assigned secret"},
		{`XAI_API_KEY=xai-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA`, "an assigned secret"},
		{`SLACK_BOT_TOKEN=xoxb-AAAAAAAAAAAA-AAAAAAAAAAAA-abcdefghijklmnop`, "an assigned secret"},
		{`LINEAR_API_KEY=lin_api_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA`, "an assigned secret"},
		{`AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE`, "an AWS access key id"},
		// what the pre-vd1bo table already held on, unchanged
		{`ANTHROPIC_API_KEY=sk-ant-api03-AAAAAAAAAAAAAAAAAAAAAAAA`, "an Anthropic key"},
		{`Authorization: Bearer ya29.A0ARrdaM9xxxxxxxxxxxxxxxxxxxxxx`, "a bearer token"},
		{`token = AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA`, "an assigned secret"},
		{`password: AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA`, "an assigned secret"},
		{`-----BEGIN OPENSSH PRIVATE KEY-----`, "a private key"},
		{`eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.abc`, "a JWT"},

		// The noise arm, in the same table as the hits on purpose: a shape
		// widened until it fires on prose about credentials has not made the
		// fleet safer, it has rebuilt the memory backlog this whole feature
		// exists to end. These six are the lines of
		// TestTheCredentialScanDoesNotFireOnProseAboutCredentials, which
		// keeps measuring the same claim end to end through a real kill.
		{`the setup-token prefix is sk-ant-oat01-… and the metered one is sk-ant-api.`, ""},
		{`the probe sends an Authorization: Bearer header, never an argv.`, ""},
		{"`password:` in the recipe means the operator is asked, not stored.", ""},
		{`accessToken is the field credShapes reads out of the keychain envelope.`, ""},
		{`-----BEGIN is how you know somebody pasted a PEM into a bead.`, ""},
		{`the path is /Users/x/.config/posse/state/plan-usage-cache.json, no secret in it.`, ""},
	} {
		got, n := firstCredShape([]string{c.line})
		if got != c.want {
			t.Errorf("firstCredShape(%q) = %q, want %q", c.line, got, c.want)
		}
		if c.want != "" && n != 1 {
			t.Errorf("firstCredShape(%q) reported line %d, want 1", c.line, n)
		}
	}
}

// The vendor shapes each have to stand on their own, because the form that
// actually reaches persona memory is a value with no key word beside it —
// an env dump that has been through a terminal, a value quoted into a
// sentence. Every line above that names a vendor also carries FOO_API_KEY=,
// so the assigned-secret shape answers first and these patterns are never
// reached; delete any one of them and that table stays green.
func TestMemoryCredShapesCatchAVendorValueStandingAlone(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		line string
		want string
	}{
		{`the operator pasted ghp_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA into the bead`, "a GitHub token"},
		{`and github_pat_11ABCDEFG0aaaaaaaaaaaaaaaaaaaaaaaaaaaa beside it`, "a GitHub token"},
		{`sk-proj-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA`, "a vendor API key"},
		{`xai-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA`, "an xAI key"},
		{`xoxb-AAAAAAAAAAAA-AAAAAAAAAAAA-abcdefghijklmnop`, "a Slack token"},
		// xoxe and xoxs beside xoxb: a refresh and a session token are what
		// a browser or a re-auth leaves behind, and the class is only widened
		// where something measures it.
		{`xoxe-AAAAAAAAAAAA-AAAAAAAAAAAA-abcdefghijklmnop`, "a Slack token"},
		{`xoxs-AAAAAAAAAAAA-AAAAAAAAAAAA-abcdefghijklmnop`, "a Slack token"},
		{`AKIAIOSFODNN7EXAMPLE`, "an AWS access key id"},
		{`ASIAIOSFODNN7EXAMPLE`, "an AWS access key id"},
		{`lin_api_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA`, "a Linear key"},
	} {
		if got, _ := firstCredShape([]string{c.line}); got != c.want {
			t.Errorf("firstCredShape(%q) = %q, want %q", c.line, got, c.want)
		}
	}
}

// The shape the bead was actually filed about, driven through the real kill
// rather than through firstCredShape: a persona pastes the environment it
// was handed into its own notes. Every line of that dump named a key word
// the pre-vd1bo table knew, and every one of them passed, because the only
// thing between the key word and its value was an underscore.
//
// One arm, not thirteen: the table above measures the shapes, and this
// measures that a widened shape reaches the hold — the commit does not
// happen, the refusal names the file and the line, and it does not repeat
// the value it is refusing to publish.
func TestKillHoldsAnEnvironmentDumpPastedIntoMemory(t *testing.T) {
	t.Parallel()
	b, fake := newTestBackend(t)
	agentPerLaunch(t, fake)
	repo := memoryRepo(t, b)
	devSession(t, b, "s1")
	const leaked = "ghp_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	appendOrders(t, repo, "dev", "- the env the launcher handed me: GH_TOKEN="+leaked+"\n")
	before := mustGit(t, repo, "rev-parse", "HEAD")

	landing, err := b.KillSessionAndLandOpts("s1", KillOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if after := mustGit(t, repo, "rev-parse", "HEAD"); after != before {
		t.Fatalf("an env-var credential was committed as %s:\n%s", after, headFiles(t, repo))
	}
	line := landing.Memory.Line()
	if !strings.Contains(line, "ORDERS.md:2") || !strings.Contains(line, "an assigned secret") {
		t.Errorf("the refusal must name the file, the line and the shape: %q", line)
	}
	if strings.Contains(line, leaked) {
		t.Errorf("the refusal echoed the credential: %q", line)
	}
}
