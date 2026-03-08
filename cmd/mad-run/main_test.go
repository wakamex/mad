package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mihai/mad/internal/harness"
)

func TestSlugify(t *testing.T) {
	t.Parallel()

	got := slugify("codex gpt-5.2/codex@high")
	want := "codex-gpt-5.2-codex-high"
	if got != want {
		t.Fatalf("slugify() = %q, want %q", got, want)
	}
}

func TestBuildHarnessCommandIncludesCodexHomeAndFlags(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := config{
		provider:        "codex",
		model:           "gpt-5.1-codex-mini",
		effort:          "medium",
		memoryMode:      harness.MemoryModeOff,
		contextMode:     harness.ContextModeEphemeral,
		serviceTier:     harness.ServiceTierFast,
		textMode:        harness.TextModeRedacted,
		seasonPath:      filepath.Join(tmp, "season.json"),
		workdir:         tmp,
		startTick:       1,
		maxTicks:        2,
		runs:            3,
		recentReveals:   3,
		maxNotesChars:   400,
		outPath:         filepath.Join(tmp, "out.json"),
		decisionTimeout: 5,
	}
	codexHome := filepath.Join(tmp, "codex-home")

	cmd, commandLine, err := buildHarnessCommand(tmp, cfg, codexHome)
	if err != nil {
		t.Fatalf("buildHarnessCommand: %v", err)
	}
	if cmd.Dir != tmp {
		t.Fatalf("cmd.Dir = %q, want %q", cmd.Dir, tmp)
	}
	if !strings.Contains(commandLine, "CODEX_HOME="+codexHome) {
		t.Fatalf("command line missing CODEX_HOME: %s", commandLine)
	}
	if !strings.Contains(strings.Join(cmd.Args, " "), "-service-tier fast") {
		t.Fatalf("args missing service tier: %v", cmd.Args)
	}
	if !strings.Contains(strings.Join(cmd.Args, " "), "-runs 3") {
		t.Fatalf("args missing runs: %v", cmd.Args)
	}
	if !strings.Contains(strings.Join(cmd.Args, " "), "-text-mode redacted") {
		t.Fatalf("args missing text mode: %v", cmd.Args)
	}
}

func TestPrepareCodexHomeCopiesAuthAndConfig(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	srcRoot := filepath.Join(tmpHome, ".codex")
	if err := os.MkdirAll(srcRoot, 0o755); err != nil {
		t.Fatalf("mkdir source root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcRoot, "auth.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write auth: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcRoot, "config.toml"), []byte("model = \"x\"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	dst := filepath.Join(t.TempDir(), "codex-home")
	if err := prepareCodexHome(dst, harness.MemoryModeOff); err != nil {
		t.Fatalf("prepareCodexHome: %v", err)
	}
	for _, name := range []string{"auth.json", "config.toml"} {
		if _, err := os.Stat(filepath.Join(dst, name)); err != nil {
			t.Fatalf("expected %s to exist: %v", name, err)
		}
	}
}

func TestPrepareCodexHomeSetsZeroIdleGateWhenMemoryOn(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	srcRoot := filepath.Join(tmpHome, ".codex")
	if err := os.MkdirAll(srcRoot, 0o755); err != nil {
		t.Fatalf("mkdir source root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcRoot, "config.toml"), []byte("model = \"x\"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	dst := filepath.Join(t.TempDir(), "codex-home")
	if err := prepareCodexHome(dst, harness.MemoryModeOn); err != nil {
		t.Fatalf("prepareCodexHome: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(dst, "config.toml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "[memories]") || !strings.Contains(text, "min_rollout_idle_hours = 0") {
		t.Fatalf("expected zero idle gate config, got %q", text)
	}
}

func TestEnsureTomlMemoriesSettingReplacesExistingValue(t *testing.T) {
	input := "model = \"x\"\n\n[memories]\nmin_rollout_idle_hours = 6\nuse_memories = true\n"
	got := ensureTomlMemoriesSetting(input, "min_rollout_idle_hours", "0")
	if strings.Count(got, "min_rollout_idle_hours") != 1 {
		t.Fatalf("expected exactly one idle gate entry, got %q", got)
	}
	if !strings.Contains(got, "min_rollout_idle_hours = 0") {
		t.Fatalf("expected replaced idle gate value, got %q", got)
	}
}

func TestDefaultRecentRevealsForMinimumHistoryBaseline(t *testing.T) {
	t.Parallel()

	got := defaultRecentReveals(harness.MemoryModeOff, harness.ContextModeEphemeral)
	if got != 0 {
		t.Fatalf("defaultRecentReveals(off, ephemeral) = %d, want 0", got)
	}
}

func TestDefaultRecentRevealsForPersistentModes(t *testing.T) {
	t.Parallel()

	got := defaultRecentReveals(harness.MemoryModeOn, harness.ContextModePersistent)
	if got != 6 {
		t.Fatalf("defaultRecentReveals(on, persistent) = %d, want 6", got)
	}
}
