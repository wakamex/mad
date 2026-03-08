package harness

import (
	"encoding/json"
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

func TestParseRunnerSpecSupportsOpenRouter(t *testing.T) {
	t.Parallel()

	spec, err := ParseRunnerSpec("openrouter:openai/gpt-oss-20b")
	if err != nil {
		t.Fatalf("ParseRunnerSpec: %v", err)
	}
	if spec.Provider != "openrouter" || spec.Model != "openai/gpt-oss-20b" {
		t.Fatalf("unexpected spec: %+v", spec)
	}
}

func TestRunnerWarningsForOpenRouterModes(t *testing.T) {
	t.Parallel()

	warnings := RunnerWarnings(RunnerSpec{
		Provider:    "openrouter",
		Model:       "openai/gpt-oss-20b",
		MemoryMode:  MemoryModeOn,
		ContextMode: ContextModePersistent,
	})
	if len(warnings) != 2 {
		t.Fatalf("RunnerWarnings() = %d warnings, want 2: %#v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0], "memory=") && !strings.Contains(warnings[1], "memory=") {
		t.Fatalf("expected memory warning: %#v", warnings)
	}
	if !strings.Contains(warnings[0], "context=persistent") && !strings.Contains(warnings[1], "context=persistent") {
		t.Fatalf("expected context warning: %#v", warnings)
	}
}

func TestOpenRouterReasoningConfig(t *testing.T) {
	t.Parallel()

	oss := openRouterReasoningConfig("openai/gpt-oss-20b")
	if oss == nil {
		t.Fatalf("expected gpt-oss-20b to use reasoning config")
	}
	if _, ok := oss["effort"]; ok {
		t.Fatalf("did not expect gpt-oss reasoning config to set effort=none: %#v", oss)
	}
	qwen := openRouterReasoningConfig("qwen/qwen3-32b")
	if qwen == nil {
		t.Fatalf("expected qwen3-32b to use reasoning config")
	}
	if got := qwen["effort"]; got != "none" {
		t.Fatalf("expected qwen3 reasoning effort none, got %#v", qwen)
	}
	if openRouterReasoningConfig("meta-llama/llama-3.1-8b-instruct") != nil {
		t.Fatalf("did not expect llama-3.1-8b-instruct to use reasoning config")
	}
}

func TestOpenRouterUsesLogprobChoice(t *testing.T) {
	t.Parallel()

	if !openRouterUsesLogprobChoice("openai/gpt-4o-mini") {
		t.Fatalf("expected gpt-4o-mini to use logprob choice mode")
	}
	if openRouterUsesLogprobChoice("openai/gpt-oss-20b") {
		t.Fatalf("did not expect gpt-oss-20b to use logprob choice mode")
	}
}

func TestExtractOpenRouterLogprobToken(t *testing.T) {
	t.Parallel()

	logprobs := struct {
		Content []struct {
			Token       string `json:"token"`
			TopLogprobs []struct {
				Token string `json:"token"`
			} `json:"top_logprobs"`
		} `json:"content"`
	}{
		Content: []struct {
			Token       string `json:"token"`
			TopLogprobs []struct {
				Token string `json:"token"`
			} `json:"top_logprobs"`
		}{
			{
				TopLogprobs: []struct {
					Token string `json:"token"`
				}{
					{Token: "A"},
					{Token: "B"},
				},
			},
		},
	}

	token, ok := extractOpenRouterLogprobToken(logprobs)
	if !ok || token != "A" {
		t.Fatalf("unexpected token=%q ok=%t", token, ok)
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

	args := runner.codexArgs("prompt", "/tmp/out.json", false)
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

func TestClaudeArgsUseSessionIDBeforeSessionExists(t *testing.T) {
	t.Parallel()

	runner, err := NewCLIRunner(RunnerSpec{
		Provider:    "claude",
		Model:       "haiku",
		Effort:      "low",
		MemoryMode:  MemoryModeOff,
		ContextMode: ContextModePersistent,
	}, ".", "")
	if err != nil {
		t.Fatalf("NewCLIRunner: %v", err)
	}
	t.Cleanup(func() { _ = runner.Close() })

	args := runner.claudeArgs("prompt", false)
	if !slices.Contains(args, "--session-id") {
		t.Fatalf("expected initial persistent claude args to use --session-id: %v", args)
	}
	if slices.Contains(args, "--resume") {
		t.Fatalf("did not expect initial persistent claude args to use --resume: %v", args)
	}
}

func TestClaudeArgsUseResumeAfterSessionExists(t *testing.T) {
	t.Parallel()

	runner, err := NewCLIRunner(RunnerSpec{
		Provider:    "claude",
		Model:       "haiku",
		Effort:      "low",
		MemoryMode:  MemoryModeOff,
		ContextMode: ContextModePersistent,
	}, ".", "")
	if err != nil {
		t.Fatalf("NewCLIRunner: %v", err)
	}
	t.Cleanup(func() { _ = runner.Close() })

	runner.nativeSessionPath = "/tmp/claude-session.jsonl"
	args := runner.claudeArgs("prompt", false)
	if !slices.Contains(args, "--resume") {
		t.Fatalf("expected subsequent persistent claude args to use --resume: %v", args)
	}
	if slices.Contains(args, "--session-id") {
		t.Fatalf("did not expect resumed persistent claude args to use --session-id: %v", args)
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

	args := runner.codexArgs("prompt", "/tmp/out.json", false)
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

	args := runner.codexArgs("prompt", "/tmp/out.json", true)
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

	args := runner.claudeArgs("prompt", true)
	if !slices.Contains(args, "--no-session-persistence") {
		t.Fatalf("claudeArgs missing --no-session-persistence: %v", args)
	}
	env := runner.claudeEnv()
	if hasEnvKeyValue(env, "CLAUDE_CODE_DISABLE_AUTO_MEMORY", "1") {
		t.Fatalf("claudeEnv disabled auto memory unexpectedly: %v", env)
	}
	if !hasEnvKeyValue(env, "HOME", runner.claudeHome) {
		t.Fatalf("claudeEnv missing isolated HOME: %v", env)
	}
	if !hasEnvKeyValue(env, "XDG_CONFIG_HOME", runner.claudeXDGConfig) {
		t.Fatalf("claudeEnv missing isolated XDG_CONFIG_HOME: %v", env)
	}
	if !hasEnvKeyValue(env, "XDG_DATA_HOME", runner.claudeXDGData) {
		t.Fatalf("claudeEnv missing isolated XDG_DATA_HOME: %v", env)
	}
	if !hasEnvKeyValue(env, "XDG_CACHE_HOME", runner.claudeXDGCache) {
		t.Fatalf("claudeEnv missing isolated XDG_CACHE_HOME: %v", env)
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

	args := runner.claudeArgs("prompt", false)
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

func TestOpenRouterPayloadMapsFastTierToThroughputSort(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "test-key")
	payload, err := openRouterPayload(
		RunnerSpec{
			Provider:    "openrouter",
			Model:       "openai/gpt-oss-20b",
			MemoryMode:  MemoryModeOff,
			ContextMode: ContextModeEphemeral,
			ServiceTier: ServiceTierFast,
		},
		"prompt",
	)
	if err != nil {
		t.Fatalf("openRouterPayload: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	provider, ok := decoded["provider"].(map[string]any)
	if !ok || provider["sort"] != "throughput" {
		t.Fatalf("payload provider sort missing: %#v", decoded["provider"])
	}
	if _, ok := decoded["response_format"]; ok {
		t.Fatalf("payload unexpectedly included response_format: %#v", decoded["response_format"])
	}
}

func TestRecoverModelTextAcceptsPrettyPrintedJSONObject(t *testing.T) {
	t.Parallel()

	raw := []byte("{\n  \"action_index\": 1,\n  \"notes\": \"\"\n}\n")
	got, ok := recoverModelText("", raw, nil)
	if !ok {
		t.Fatalf("recoverModelText() did not recover valid pretty JSON")
	}
	if !json.Valid(got) {
		t.Fatalf("recoverModelText() returned invalid JSON: %q", string(got))
	}
}

func TestRecoverModelTextExtractsCodexAssistantMessage(t *testing.T) {
	t.Parallel()

	raw := []byte("{\"type\":\"thread.started\",\"thread_id\":\"t1\"}\n{\"type\":\"response.output_item.done\",\"item\":{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"2\\nNotes: keep relay clean\"}]}}\n")
	got, ok := recoverModelText("", raw, nil)
	if !ok {
		t.Fatalf("recoverModelText() did not recover codex assistant text")
	}
	if string(got) != "2\nNotes: keep relay clean" {
		t.Fatalf("recoverModelText() = %q", string(got))
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
