package harness

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestParseMemoryMode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw  string
		want MemoryMode
		ok   bool
	}{
		{raw: "", want: MemoryModeInherit, ok: true},
		{raw: "inherit", want: MemoryModeInherit, ok: true},
		{raw: "on", want: MemoryModeOn, ok: true},
		{raw: "off", want: MemoryModeOff, ok: true},
		{raw: "ON", want: MemoryModeOn, ok: true},
		{raw: "bad", ok: false},
	}

	for _, tc := range cases {
		got, err := ParseMemoryMode(tc.raw)
		if tc.ok && err != nil {
			t.Fatalf("ParseMemoryMode(%q) unexpected error: %v", tc.raw, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("ParseMemoryMode(%q) expected error", tc.raw)
		}
		if tc.ok && got != tc.want {
			t.Fatalf("ParseMemoryMode(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestParseContextMode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw  string
		want ContextMode
		ok   bool
	}{
		{raw: "", want: ContextModePersistent, ok: true},
		{raw: "persistent", want: ContextModePersistent, ok: true},
		{raw: "ephemeral", want: ContextModeEphemeral, ok: true},
		{raw: "EPHEMERAL", want: ContextModeEphemeral, ok: true},
		{raw: "bad", ok: false},
	}

	for _, tc := range cases {
		got, err := ParseContextMode(tc.raw)
		if tc.ok && err != nil {
			t.Fatalf("ParseContextMode(%q) unexpected error: %v", tc.raw, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("ParseContextMode(%q) expected error", tc.raw)
		}
		if tc.ok && got != tc.want {
			t.Fatalf("ParseContextMode(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestParseServiceTier(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw  string
		want ServiceTier
		ok   bool
	}{
		{raw: "", want: ServiceTierInherit, ok: true},
		{raw: "inherit", want: ServiceTierInherit, ok: true},
		{raw: "fast", want: ServiceTierFast, ok: true},
		{raw: "flex", want: ServiceTierFlex, ok: true},
		{raw: "FAST", want: ServiceTierFast, ok: true},
		{raw: "bad", ok: false},
	}

	for _, tc := range cases {
		got, err := ParseServiceTier(tc.raw)
		if tc.ok && err != nil {
			t.Fatalf("ParseServiceTier(%q) unexpected error: %v", tc.raw, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("ParseServiceTier(%q) expected error", tc.raw)
		}
		if tc.ok && got != tc.want {
			t.Fatalf("ParseServiceTier(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestCodexArgsIncludeMemoryConfig(t *testing.T) {
	t.Parallel()

	runner, err := NewCLIRunner(RunnerSpec{
		Provider:   "codex",
		Model:      "gpt-5.2-codex",
		Effort:     "high",
		MemoryMode: MemoryModeOn,
	}, ".", "")
	if err != nil {
		t.Fatalf("NewCLIRunner: %v", err)
	}
	t.Cleanup(func() {
		_ = runner.Close()
	})

	args := runner.codexArgs("prompt", "/tmp/schema.json", "/tmp/out.json", false)
	wantFragments := []string{
		"-c", "model_reasoning_effort=high",
		"-c", "features.memory_tool=true",
		"-c", "memories.use_memories=true",
		"-c", "memories.generate_memories=true",
	}
	for _, fragment := range wantFragments {
		if !slices.Contains(args, fragment) {
			t.Fatalf("codexArgs missing %q: %v", fragment, args)
		}
	}
}

func TestCodexArgsIncludeServiceTier(t *testing.T) {
	t.Parallel()

	runner, err := NewCLIRunner(RunnerSpec{
		Provider:    "codex",
		Model:       "gpt-5.1-codex-mini",
		Effort:      "medium",
		ServiceTier: ServiceTierFast,
	}, ".", "")
	if err != nil {
		t.Fatalf("NewCLIRunner: %v", err)
	}
	t.Cleanup(func() {
		_ = runner.Close()
	})

	args := runner.codexArgs("prompt", "/tmp/schema.json", "/tmp/out.json", false)
	wantFragments := []string{
		"-c", "service_tier=fast",
	}
	for _, fragment := range wantFragments {
		if !slices.Contains(args, fragment) {
			t.Fatalf("codexArgs missing %q: %v", fragment, args)
		}
	}
}

func TestCodexArgsEphemeralContextUsesEphemeralExec(t *testing.T) {
	t.Parallel()

	runner, err := NewCLIRunner(RunnerSpec{
		Provider:    "codex",
		Model:       "gpt-5.2-codex",
		Effort:      "high",
		ContextMode: ContextModeEphemeral,
	}, ".", "")
	if err != nil {
		t.Fatalf("NewCLIRunner: %v", err)
	}
	t.Cleanup(func() {
		_ = runner.Close()
	})

	args := runner.codexArgs("prompt", "/tmp/schema.json", "/tmp/out.json", true)
	if !slices.Contains(args, "--ephemeral") {
		t.Fatalf("codexArgs missing --ephemeral: %v", args)
	}
}

func TestClaudeMemoryOffPersistentCreatesSessionID(t *testing.T) {
	t.Parallel()

	runner, err := NewCLIRunner(RunnerSpec{
		Provider:   "claude",
		Model:      "haiku",
		Effort:     "low",
		MemoryMode: MemoryModeOff,
	}, ".", "")
	if err != nil {
		t.Fatalf("NewCLIRunner: %v", err)
	}
	t.Cleanup(func() {
		_ = runner.Close()
	})

	if runner.claudeSessionID == "" {
		t.Fatalf("expected session id for persistent memory-off Claude run")
	}
}

func TestClaudeEphemeralContextDoesNotCreateSessionID(t *testing.T) {
	t.Parallel()

	runner, err := NewCLIRunner(RunnerSpec{
		Provider:    "claude",
		Model:       "haiku",
		Effort:      "low",
		ContextMode: ContextModeEphemeral,
	}, ".", "")
	if err != nil {
		t.Fatalf("NewCLIRunner: %v", err)
	}
	t.Cleanup(func() {
		_ = runner.Close()
	})

	if runner.claudeSessionID != "" {
		t.Fatalf("expected no session id for ephemeral context, got %q", runner.claudeSessionID)
	}
}

func TestClaudeArgsEphemeralMemoryOnUsesNoSessionPersistence(t *testing.T) {
	t.Parallel()

	runner, err := NewCLIRunner(RunnerSpec{
		Provider:    "claude",
		Model:       "haiku",
		Effort:      "low",
		MemoryMode:  MemoryModeOn,
		ContextMode: ContextModeEphemeral,
	}, ".", "")
	if err != nil {
		t.Fatalf("NewCLIRunner: %v", err)
	}
	t.Cleanup(func() {
		_ = runner.Close()
	})

	args := runner.claudeArgs("prompt", `{"type":"object"}`, true)
	if !slices.Contains(args, "--no-session-persistence") {
		t.Fatalf("claudeArgs missing --no-session-persistence: %v", args)
	}
	env := runner.claudeEnv()
	if hasEnvKeyValue(env, "CLAUDE_CODE_DISABLE_AUTO_MEMORY", "1") {
		t.Fatalf("claudeEnv disabled auto memory unexpectedly: %v", env)
	}
}

func TestClaudeArgsPersistentMemoryOffKeepsSessionPersistence(t *testing.T) {
	t.Parallel()

	runner, err := NewCLIRunner(RunnerSpec{
		Provider:   "claude",
		Model:      "haiku",
		Effort:     "low",
		MemoryMode: MemoryModeOff,
	}, ".", "")
	if err != nil {
		t.Fatalf("NewCLIRunner: %v", err)
	}
	t.Cleanup(func() {
		_ = runner.Close()
	})

	args := runner.claudeArgs("prompt", `{"type":"object"}`, false)
	if slices.Contains(args, "--no-session-persistence") {
		t.Fatalf("claudeArgs unexpectedly disabled session persistence: %v", args)
	}
	if !slices.Contains(args, "--session-id") {
		t.Fatalf("claudeArgs missing --session-id: %v", args)
	}
	env := runner.claudeEnv()
	if !hasEnvKeyValue(env, "CLAUDE_CODE_DISABLE_AUTO_MEMORY", "1") {
		t.Fatalf("claudeEnv missing auto-memory disable: %v", env)
	}
}

func TestClaudeRunnerUsesIsolatedHomeAndMemoryPath(t *testing.T) {
	t.Parallel()

	runner, err := NewCLIRunner(RunnerSpec{
		Provider:  "claude",
		Model:     "haiku",
		Effort:    "low",
		MemoryMode: MemoryModeOn,
	}, ".", "")
	if err != nil {
		t.Fatalf("NewCLIRunner: %v", err)
	}
	t.Cleanup(func() {
		_ = runner.Close()
	})

	info := runner.SessionInfo()
	if !strings.Contains(info.NativeHomeDir, runner.tempDir) {
		t.Fatalf("NativeHomeDir = %q, want isolated temp root %q", info.NativeHomeDir, runner.tempDir)
	}
	if !strings.Contains(info.NativeMemoryPath, filepath.Join("memory", "MEMORY.md")) {
		t.Fatalf("NativeMemoryPath = %q, want memory/MEMORY.md suffix", info.NativeMemoryPath)
	}
}

func TestFindCodexSessionPathUsesConfiguredCodexHome(t *testing.T) {
	tmp := t.TempDir()
	threadID := "019cc885-077e-77b0-b0ae-f9c44a82f12f"
	sessionPath := filepath.Join(tmp, "sessions", "2026", "03", "07", "rollout-"+threadID+".jsonl")
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	if err := os.WriteFile(sessionPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	t.Setenv("CODEX_HOME", tmp)
	got := findCodexSessionPath(threadID)
	if got != sessionPath {
		t.Fatalf("findCodexSessionPath() = %q, want %q", got, sessionPath)
	}
}

func TestRunnerLabelIncludesMemoryMode(t *testing.T) {
	t.Parallel()

	spec := RunnerSpec{Provider: "codex", Model: "gpt-5.2-codex", Effort: "high", MemoryMode: MemoryModeOff}
	if got := spec.Label(); !strings.Contains(got, "+mem-off") {
		t.Fatalf("Label() = %q, want memory suffix", got)
	}
}

func hasEnvKeyValue(env []string, key string, want string) bool {
	prefix := key + "="
	for _, entry := range env {
		if !strings.HasPrefix(entry, prefix) {
			continue
		}
		return strings.TrimPrefix(entry, prefix) == want
	}
	return false
}
