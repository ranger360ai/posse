package main

// ADR 0039 D3b, ranger-base-7dbnq: `posse runtimes` is where "is the new
// strong model on this account" gets answered without launching anything,
// and `--probe` is how it gets asked again without a config edit or a
// hand-edited state file.
//
// Hermetic by seeding state/model-catalog.json: the binary's ModelLister
// has no URL override and no credential override by design (modelavail.go),
// so the only way to run this surface without reaching the operator's
// keychain and the real endpoint is to hand it a snapshot it will not need
// to refresh. Every run below therefore also PROVES the read came off the
// file — a request would have failed here.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// seedRuntimeCatalog writes the shared snapshot into a home, `age` old,
// with an optional cooldown running until `retry`.
func seedRuntimeCatalog(t *testing.T, home string, age time.Duration, retry time.Time, ids ...string) {
	t.Helper()
	e := map[string]any{"at": time.Now().Add(-age).Format(time.RFC3339Nano), "models": ids}
	if !retry.IsZero() {
		e["retry_at"] = retry.Format(time.RFC3339Nano)
	}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, "state"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "state", "model-catalog.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func runtimesOut(t *testing.T, bin, home string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, append([]string{"runtimes"}, args...)...)
	cmd.Env = []string{"HOME=" + t.TempDir(), "RHQ_HOME=" + home, "PATH=" + os.Getenv("PATH")}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("posse runtimes %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// The acceptance surface: under each runtime posse can read a catalog for,
// one verdict per mapped tier — and under the runtimes it cannot, none.
func TestRuntimesCarriesTheAvailabilityVerdictPerMappedTier(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	// A template-only profile off the catalog host, beside the built-ins.
	if err := os.MkdirAll(filepath.Join(home, "runtimes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "runtimes", "othercli.yaml"),
		[]byte("command: othercli {model} --sys {file}\negress: [api.example.test]\nmodel_standard: other-1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A reading a minute old: inside the default lease, so no age clause,
	// and holding neither the strong id nor othercli's.
	seedRuntimeCatalog(t, home, time.Minute, time.Time{}, "claude-opus-5", "claude-sonnet-5")

	got := runtimesOut(t, bin, home)
	for _, want := range []string{
		// persona "" — the catalog is the ACCOUNT's, not a PID's.
		"session: tier strong wants claude-fable-5-1 — unavailable on this account; launching as asked, and only an explicit --runtime/--tier/--model or a PID change moves it",
		"claude: tier standard → claude-opus-5 (available)",
		"claude: tier fast → claude-sonnet-5 (available)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("posse runtimes missing %q in:\n%s", want, got)
		}
	}
	// A runtime whose egress: does not name the catalog host gets NO
	// availability line — posse knows no list its ids could be on, and a
	// line saying so per tier is noise, not news.
	for _, unwanted := range []string{
		"no model catalog posse can read",
		"tier strong → gpt-5.6-sol",
		"tier standard → other-1",
		"tier strong → grok-4.6",
	} {
		if strings.Contains(got, unwanted) {
			t.Errorf("a runtime off the catalog host printed an availability line %q:\n%s", unwanted, got)
		}
	}
	// The reading was inside its lease, so nothing was asked and nothing
	// was logged — the witness that this whole test reached no endpoint.
	if _, err := os.Stat(filepath.Join(home, "state", "model-catalog.log")); !os.IsNotExist(err) {
		log, _ := os.ReadFile(filepath.Join(home, "state", "model-catalog.log"))
		t.Errorf("a snapshot inside its lease must ask nobody:\n%s", log)
	}
}

// --probe means maxAge 0 — but Models checks the RetryAt cooldown before it
// asks, so a forced read still cannot become the rangerhq-tdy8 storm. What
// the cooldown does NOT do is renew trust (ADR 0039 D3c): the reading it
// falls back to is two days old, so every verdict on it reads UNKNOWN and
// names the reading it could not replace.
func TestRuntimesProbeHonoursALiveCooldownAndDatesTheReading(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	seedRuntimeCatalog(t, home, 48*time.Hour, time.Now().Add(10*time.Minute), "claude-opus-5", "claude-sonnet-5")

	got := runtimesOut(t, bin, home, "--probe")
	for _, want := range []string{
		"session: tier strong wants claude-fable-5-1 — not in the catalog read 48h00m ago; availability UNKNOWN, launching as asked",
		"claude: tier standard → claude-opus-5 (availability UNKNOWN — the catalog read 48h00m ago is past model_probe_ttl; the launch takes the tier as asked)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("--probe over a stale reading missing %q in:\n%s", want, got)
		}
	}
	// Nothing was ASKED, so nothing may be reported as a failing probe —
	// and nothing was logged, because the cooldown ran first.
	if strings.Contains(got, "the probe is failing") {
		t.Errorf("a read that never happened reported as a failing probe:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(home, "state", "model-catalog.log")); !os.IsNotExist(err) {
		log, _ := os.ReadFile(filepath.Join(home, "state", "model-catalog.log"))
		t.Errorf("--probe asked a cooling-down endpoint:\n%s", log)
	}

	// The control, and the reason the lease is the operator's number and not
	// the caller's: the same forced read over a reading INSIDE
	// model_probe_ttl still rules, so `posse runtimes --probe` goes on
	// printing the bytes a launch would print. "Rules" is the whole of what
	// a verdict does since ADR 0003 §3 (ranger-base-hv2zr) — it decides
	// which of the two sentences is printed, and no longer decides which
	// model runs.
	fresh := t.TempDir()
	seedRuntimeCatalog(t, fresh, 30*time.Minute, time.Now().Add(10*time.Minute), "claude-opus-5", "claude-sonnet-5")
	if got := runtimesOut(t, bin, fresh, "--probe"); !strings.Contains(got, "session: tier strong wants claude-fable-5-1 — unavailable on this account; launching as asked, and only an explicit --runtime/--tier/--model or a PID change moves it") {
		t.Errorf("a cooled-down reading inside its lease must still rule:\n%s", got)
	}
}

// The flag is the only one this command takes, and an unknown one is a
// typo the operator must see rather than a listing that silently ignores it.
func TestRuntimesRefusesAnUnknownFlag(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	seedRuntimeCatalog(t, home, time.Minute, time.Time{}, "claude-fable-5-1")
	cmd := exec.Command(bin, "runtimes", "--porbe")
	cmd.Env = []string{"HOME=" + t.TempDir(), "RHQ_HOME=" + home, "PATH=" + os.Getenv("PATH")}
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("an unknown flag must refuse:\n%s", out)
	}
	if !strings.Contains(string(out), "usage: posse runtimes [--probe]") {
		t.Errorf("the refusal must name the usage:\n%s", out)
	}
}

// The flag has to reach Models, and the only observable that separates
// "re-read now" from "rule off the snapshot" is the read itself: over a
// reading INSIDE its lease, the plain listing asks nobody and --probe asks
// exactly once. This is the one test here that lets the binary use its
// real ModelLister — the outcome is deliberately not asserted (a box with
// no readable credential and a box with a live one must both pass), only
// that the forced read happened, once, and left the one line the operator
// reads it back from.
func TestRuntimesProbeForcesExactlyOneRead(t *testing.T) {
	bin := buildRhq(t)
	home := t.TempDir()
	log := filepath.Join(home, "state", "model-catalog.log")
	seedRuntimeCatalog(t, home, time.Minute, time.Time{}, "claude-fable-5-1", "claude-opus-5", "claude-sonnet-5")

	if got := runtimesOut(t, bin, home); !strings.Contains(got, "claude: tier strong → claude-fable-5-1 (available)") {
		t.Fatalf("the fixture must rule off the snapshot:\n%s", got)
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		b, _ := os.ReadFile(log)
		t.Fatalf("the plain listing asked the endpoint:\n%s", b)
	}

	runtimesOut(t, bin, home, "--probe")
	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("--probe over a snapshot inside its lease did not re-read: %v", err)
	}
	if got := strings.Count(string(b), "\n"); got != 1 {
		t.Errorf("--probe wrote %d model-catalog.log lines, want 1:\n%s", got, b)
	}
}
