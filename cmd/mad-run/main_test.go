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
		seasonPath:      filepath.Join(tmp, "season.json"),
		workdir:         tmp,
		startTick:       1,
		maxTicks:        2,
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
	if err := prepareCodexHome(dst); err != nil {
		t.Fatalf("prepareCodexHome: %v", err)
	}
	for _, name := range []string{"auth.json", "config.toml"} {
		if _, err := os.Stat(filepath.Join(dst, name)); err != nil {
			t.Fatalf("expected %s to exist: %v", name, err)
		}
	}
}
