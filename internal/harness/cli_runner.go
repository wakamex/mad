package harness

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const openRouterChatURL = "https://openrouter.ai/api/v1/chat/completions"

type CLIRunner struct {
	spec              RunnerSpec
	workdir           string
	tempDir           string
	cleanup           bool
	codexThreadID     string
	claudeSessionID   string
	nativeHomeDir     string
	nativeProjectDir  string
	nativeSessionPath string
	nativeMemoryPath  string
	claudeHome        string
	claudeXDGConfig   string
	claudeXDGData     string
	claudeXDGCache    string
}

func NewCLIRunner(spec RunnerSpec, workdir string, tempRoot string) (*CLIRunner, error) {
	if workdir == "" {
		workdir = "."
	}
	absWorkdir, err := filepath.Abs(workdir)
	if err != nil {
		return nil, err
	}
	cleanup := false
	tempDir := tempRoot
	if tempDir == "" {
		tempDir, err = os.MkdirTemp(os.TempDir(), "mad-harness-*")
		if err != nil {
			return nil, err
		}
		cleanup = true
	} else {
		if err := os.MkdirAll(tempDir, 0o755); err != nil {
			return nil, err
		}
	}

	runner := &CLIRunner{
		spec:    spec,
		workdir: absWorkdir,
		tempDir: tempDir,
		cleanup: cleanup,
	}
	if spec.Provider == "claude" {
		runner.claudeHome = filepath.Join(tempDir, "claude-home")
		runner.claudeXDGConfig = filepath.Join(runner.claudeHome, ".config")
		runner.claudeXDGData = filepath.Join(runner.claudeHome, ".local", "share")
		runner.claudeXDGCache = filepath.Join(runner.claudeHome, ".cache")
		if err := prepareClaudeHome(runner.claudeHome); err != nil {
			return nil, err
		}
		runner.nativeHomeDir = filepath.Join(runner.claudeHome, ".claude")
		runner.nativeProjectDir = claudeProjectDir(runner.claudeHome, absWorkdir)
		runner.nativeMemoryPath = filepath.Join(runner.nativeProjectDir, "memory", "MEMORY.md")
		if spec.ContextMode != ContextModeEphemeral {
			sessionID, err := newSessionID()
			if err != nil {
				return nil, err
			}
			runner.claudeSessionID = sessionID
		}
	} else if spec.Provider == "codex" {
		runner.nativeHomeDir = resolveCodexHome()
		runner.nativeProjectDir = filepath.Join(resolveCodexHome(), "sessions")
	} else if spec.Provider == "openrouter" {
		runner.nativeProjectDir = ""
	}
	return runner, nil
}

func (r *CLIRunner) Spec() RunnerSpec {
	return r.spec
}

func (r *CLIRunner) Close() error {
	if r.tempDir == "" || !r.cleanup {
		return nil
	}
	return os.RemoveAll(r.tempDir)
}

func (r *CLIRunner) SessionInfo() SessionInfo {
	sessionID := ""
	switch r.spec.Provider {
	case "codex":
		sessionID = r.codexThreadID
	case "claude":
		sessionID = r.claudeSessionID
	}
	return SessionInfo{
		Workdir:           r.workdir,
		ProviderSessionID: sessionID,
		NativeHomeDir:     r.nativeHomeDir,
		NativeProjectDir:  r.nativeProjectDir,
		NativeSessionPath: r.nativeSessionPath,
		NativeMemoryPath:  r.nativeMemoryPath,
	}
}

func (r *CLIRunner) Decide(ctx context.Context, prompt string) ([]byte, error) {
	ephemeral := r.spec.ContextMode == ContextModeEphemeral
	switch r.spec.Provider {
	case "codex":
		return r.runCodex(ctx, prompt, ephemeral)
	case "claude":
		return r.runClaude(ctx, prompt, ephemeral)
	case "openrouter":
		return r.runOpenRouter(ctx, prompt)
	default:
		return nil, fmt.Errorf("unsupported provider %q", r.spec.Provider)
	}
}

func (r *CLIRunner) Probe(ctx context.Context) error {
	const prompt = "Reply with exactly OK."
	var raw []byte
	var err error
	switch r.spec.Provider {
	case "codex":
		raw, err = r.runCodex(ctx, prompt, true)
	case "claude":
		raw, err = r.runClaude(ctx, prompt, true)
	case "openrouter":
		raw, err = r.runOpenRouter(ctx, prompt)
	default:
		return fmt.Errorf("unsupported provider %q", r.spec.Provider)
	}
	if err != nil {
		return err
	}
	return decodeProbe(raw)
}

func (r *CLIRunner) runCodex(ctx context.Context, prompt string, ephemeral bool) ([]byte, error) {
	outputFile, err := os.CreateTemp("", "mad-codex-output-*.json")
	if err != nil {
		return nil, err
	}
	outputPath := outputFile.Name()
	if err := outputFile.Close(); err != nil {
		return nil, err
	}
	defer os.Remove(outputPath)

	args := r.codexArgs(prompt, outputPath, ephemeral)

	cmd := exec.CommandContext(ctx, "codex", args...)
	cmd.Dir = r.workdir
	cmd.Env = filteredEnv("CODEX_THREAD_ID")
	cmd.Stdin = strings.NewReader(prompt)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if recovered, ok := recoverModelText(outputPath, stdout.Bytes(), stderr.Bytes()); ok {
			if !ephemeral {
				r.captureCodexSession(stdout.Bytes())
			}
			return recovered, nil
		}
		return nil, fmt.Errorf("codex %s failed: %w stderr=%s", r.spec.Label(), err, strings.TrimSpace(stderr.String()))
	}
	if !ephemeral {
		r.captureCodexSession(stdout.Bytes())
	}

	content, ok := recoverModelText(outputPath, stdout.Bytes(), stderr.Bytes())
	if !ok {
		return nil, fmt.Errorf("codex %s produced empty output", r.spec.Label())
	}
	return content, nil
}

func (r *CLIRunner) runClaude(ctx context.Context, prompt string, ephemeral bool) ([]byte, error) {
	args := r.claudeArgs(prompt, ephemeral)
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = r.workdir
	cmd.Env = r.claudeEnv()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if recovered, ok := recoverModelText("", stdout.Bytes(), stderr.Bytes()); ok {
			if !ephemeral || r.claudeSessionID != "" {
				r.captureClaudeSession()
			}
			return recovered, nil
		}
		return nil, fmt.Errorf("claude %s failed: %w stderr=%s", r.spec.Label(), err, strings.TrimSpace(stderr.String()))
	}
	if !ephemeral || r.claudeSessionID != "" {
		r.captureClaudeSession()
	}
	content, ok := recoverModelText("", stdout.Bytes(), stderr.Bytes())
	if !ok {
		return nil, fmt.Errorf("claude %s produced empty output stdout=%q stderr=%q", r.spec.Label(), strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()))
	}
	return content, nil
}

func (r *CLIRunner) runOpenRouter(ctx context.Context, prompt string) ([]byte, error) {
	payload, err := openRouterPayload(r.spec, prompt)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterChatURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+openRouterAPIKey())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "mad-harness/1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openrouter %s request failed: %w", r.spec.Label(), err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openrouter %s read failed: %w", r.spec.Label(), err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openrouter %s status=%s body=%s", r.spec.Label(), resp.Status, strings.TrimSpace(string(body)))
	}
	content, err := extractOpenRouterContent(body)
	if err != nil {
		return nil, fmt.Errorf("openrouter %s decode failed: %w body=%s", r.spec.Label(), err, strings.TrimSpace(string(body)))
	}
	if recovered, ok := recoverModelText("", content, nil); ok {
		return recovered, nil
	}
	return nil, fmt.Errorf("openrouter %s produced non-JSON content=%q", r.spec.Label(), strings.TrimSpace(string(content)))
}

func (r *CLIRunner) claudeArgs(prompt string, ephemeral bool) []string {
	args := []string{
		"-p",
		"--model", r.spec.Model,
		"--tools", "",
	}
	if r.spec.Effort != "" {
		args = append(args, "--effort", r.spec.Effort)
	}
	if ephemeral || r.spec.ContextMode == ContextModeEphemeral {
		args = append(args, "--no-session-persistence")
	} else if r.claudeSessionID != "" {
		if r.nativeSessionPath != "" {
			args = append(args, "--resume", r.claudeSessionID)
		} else {
			args = append(args, "--session-id", r.claudeSessionID)
		}
	}
	args = append(args, prompt)
	return args
}

func (r *CLIRunner) claudeEnv() []string {
	env := filteredEnv(
		"CLAUDECODE",
		"CLAUDE_CODE_ENTRYPOINT",
		"CLAUDE_CODE_DISABLE_AUTO_MEMORY",
		"CLAUDE_CODE_DISABLE_CLAUDE_MDS",
		"HOME",
		"XDG_CONFIG_HOME",
		"XDG_DATA_HOME",
		"XDG_CACHE_HOME",
	)
	if r.claudeHome != "" {
		env = append(env, "HOME="+r.claudeHome)
	}
	if r.claudeXDGConfig != "" {
		env = append(env, "XDG_CONFIG_HOME="+r.claudeXDGConfig)
	}
	if r.claudeXDGData != "" {
		env = append(env, "XDG_DATA_HOME="+r.claudeXDGData)
	}
	if r.claudeXDGCache != "" {
		env = append(env, "XDG_CACHE_HOME="+r.claudeXDGCache)
	}
	if r.spec.MemoryMode == MemoryModeOff {
		env = append(env, "CLAUDE_CODE_DISABLE_AUTO_MEMORY=1")
	} else {
		env = append(env, "CLAUDE_CODE_DISABLE_AUTO_MEMORY=0")
	}
	env = append(env, "CLAUDE_CODE_DISABLE_CLAUDE_MDS=1")
	return env
}

func (r *CLIRunner) codexArgs(prompt string, outputPath string, ephemeral bool) []string {
	if r.codexThreadID != "" && !ephemeral {
		args := []string{"exec", "resume", r.codexThreadID}
		if r.spec.Model != "" {
			args = append(args, "-m", r.spec.Model)
		}
		if r.spec.Effort != "" {
			args = append(args, "-c", "model_reasoning_effort="+r.spec.Effort)
		}
		if r.spec.ServiceTier != "" && r.spec.ServiceTier != ServiceTierInherit {
			args = append(args, "-c", "service_tier="+string(r.spec.ServiceTier))
		}
		args = append(args, r.codexMemoryConfigArgs()...)
		args = append(args,
			"--skip-git-repo-check",
			"-o", outputPath,
			"-",
		)
		return args
	}

	args := []string{
		"exec",
		"-m", r.spec.Model,
		"-s", "read-only",
		"-C", r.workdir,
		"--skip-git-repo-check",
		"-o", outputPath,
		"--color", "never",
		"--json",
	}
	if ephemeral {
		args = append(args, "--ephemeral")
	}
	if r.spec.Effort != "" {
		args = append(args, "-c", "model_reasoning_effort="+r.spec.Effort)
	}
	if r.spec.ServiceTier != "" && r.spec.ServiceTier != ServiceTierInherit && r.spec.Provider == "codex" {
		args = append(args, "-c", "service_tier="+string(r.spec.ServiceTier))
	}
	args = append(args, r.codexMemoryConfigArgs()...)
	args = append(args, "-")
	return args
}

func (r *CLIRunner) codexMemoryConfigArgs() []string {
	switch r.spec.MemoryMode {
	case MemoryModeOn:
		return []string{
			"-c", "features.memory_tool=true",
			"-c", "memories.use_memories=true",
			"-c", "memories.generate_memories=true",
		}
	case MemoryModeOff:
		return []string{
			"-c", "features.memory_tool=false",
			"-c", "memories.use_memories=false",
			"-c", "memories.generate_memories=false",
		}
	default:
		return nil
	}
}

func decodeProbe(raw []byte) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return fmt.Errorf("empty probe response")
	}
	if strings.EqualFold(string(trimmed), "ok") {
		return nil
	}

	var direct struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(trimmed, &direct); err == nil && direct.OK {
		return nil
	}

	var wrapped map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &wrapped); err == nil {
		for _, key := range []string{"structured_output", "result", "response", "content", "output"} {
			payload, ok := wrapped[key]
			if !ok {
				continue
			}
			if err := decodeProbe(payload); err == nil {
				return nil
			}
			var text string
			if err := json.Unmarshal(payload, &text); err == nil {
				if err := decodeProbe([]byte(text)); err == nil {
					return nil
				}
			}
		}
	}

	var text string
	if err := json.Unmarshal(trimmed, &text); err == nil {
		return decodeProbe([]byte(text))
	}
	return fmt.Errorf("probe response did not validate: %s", string(trimmed))
}

func filteredEnv(dropKeys ...string) []string {
	if len(dropKeys) == 0 {
		return os.Environ()
	}
	drops := make(map[string]struct{}, len(dropKeys))
	for _, key := range dropKeys {
		drops[key] = struct{}{}
	}
	env := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if _, drop := drops[key]; drop {
			continue
		}
		env = append(env, entry)
	}
	return env
}

func recoverModelText(outputPath string, stdout []byte, stderr []byte) ([]byte, bool) {
	if outputPath != "" {
		if content, err := os.ReadFile(outputPath); err == nil && len(bytes.TrimSpace(content)) > 0 {
			return bytes.TrimSpace(content), true
		}
	}
	for _, blob := range [][]byte{stdout, stderr} {
		if content, ok := extractCodexAssistantText(blob); ok {
			return content, true
		}
		if content, ok := extractJSONObjectLine(blob); ok {
			return content, true
		}
		trimmed := bytes.TrimSpace(blob)
		if len(trimmed) > 0 {
			return trimmed, true
		}
	}
	return nil, false
}

func openRouterAPIKey() string {
	return strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
}

func openRouterPayload(spec RunnerSpec, prompt string) ([]byte, error) {
	if openRouterAPIKey() == "" {
		return nil, fmt.Errorf("OPENROUTER_API_KEY is not set")
	}
	payload := map[string]any{
		"model": spec.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens":  96,
		"temperature": 0,
	}
	if openRouterUsesLogprobChoice(spec.Model) {
		payload["max_tokens"] = 1
		payload["logprobs"] = true
		payload["top_logprobs"] = 20
	}
	if reasoning := openRouterReasoningConfig(spec.Model); reasoning != nil {
		payload["reasoning"] = reasoning
	}
	if spec.ServiceTier == ServiceTierFast && !openRouterUsesLogprobChoice(spec.Model) {
		payload["provider"] = map[string]any{"sort": "throughput"}
	}
	return json.Marshal(payload)
}

func openRouterReasoningConfig(model string) map[string]any {
	model = strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.Contains(model, "gpt-oss"):
		return map[string]any{
			"exclude": true,
		}
	case strings.Contains(model, "qwen/qwen3"):
		return map[string]any{
			"effort":  "none",
			"exclude": true,
		}
	default:
		return nil
	}
}

func openRouterUsesLogprobChoice(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	return strings.Contains(model, "openai/gpt-4o-mini")
}

func extractOpenRouterContent(raw []byte) ([]byte, error) {
	var response struct {
		Choices []struct {
			Message struct {
				Content any `json:"content"`
			} `json:"message"`
			Logprobs struct {
				Content []struct {
					Token       string `json:"token"`
					TopLogprobs []struct {
						Token string `json:"token"`
					} `json:"top_logprobs"`
				} `json:"content"`
			} `json:"logprobs"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, err
	}
	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("response had no choices")
	}
	content := response.Choices[0].Message.Content
	switch typed := content.(type) {
	case string:
		return []byte(typed), nil
	case []any:
		var builder strings.Builder
		for _, part := range typed {
			obj, ok := part.(map[string]any)
			if !ok {
				continue
			}
			text, _ := obj["text"].(string)
			builder.WriteString(text)
		}
		if builder.Len() == 0 {
			if token, ok := extractOpenRouterLogprobToken(response.Choices[0].Logprobs); ok {
				return []byte(token), nil
			}
			return nil, fmt.Errorf("content parts had no text")
		}
		return []byte(builder.String()), nil
	default:
		if token, ok := extractOpenRouterLogprobToken(response.Choices[0].Logprobs); ok {
			return []byte(token), nil
		}
		return nil, fmt.Errorf("unsupported content type %T", content)
	}
}

func extractOpenRouterLogprobToken(logprobs struct {
	Content []struct {
		Token       string `json:"token"`
		TopLogprobs []struct {
			Token string `json:"token"`
		} `json:"top_logprobs"`
	} `json:"content"`
}) (string, bool) {
	for _, part := range logprobs.Content {
		if token, ok := normalizeOpenRouterChoiceToken(part.Token); ok {
			return token, true
		}
		for _, candidate := range part.TopLogprobs {
			if token, ok := normalizeOpenRouterChoiceToken(candidate.Token); ok {
				return token, true
			}
		}
	}
	return "", false
}

func normalizeOpenRouterChoiceToken(token string) (string, bool) {
	token = strings.TrimSpace(token)
	token = strings.Trim(token, "\"'`[](){}.,")
	if len(token) != 1 {
		return "", false
	}
	ch := token[0]
	if ch >= 'A' && ch <= 'Z' {
		return string(ch), true
	}
	if ch >= 'a' && ch <= 'z' {
		return strings.ToUpper(token), true
	}
	if ch >= '1' && ch <= '9' {
		return string(ch), true
	}
	return "", false
}

func extractCodexAssistantText(raw []byte) ([]byte, bool) {
	lines := bytes.Split(raw, []byte{'\n'})
	var latest string
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || !json.Valid(line) {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}
		if fmt.Sprint(event["type"]) != "response.output_item.done" {
			continue
		}
		item, ok := event["item"].(map[string]any)
		if !ok {
			continue
		}
		role, _ := item["role"].(string)
		if role != "" && role != "assistant" {
			continue
		}
		content, ok := item["content"].([]any)
		if !ok {
			continue
		}
		var builder strings.Builder
		for _, part := range content {
			entry, ok := part.(map[string]any)
			if !ok {
				continue
			}
			if entryType, _ := entry["type"].(string); entryType != "output_text" {
				continue
			}
			text, _ := entry["text"].(string)
			builder.WriteString(text)
		}
		if builder.Len() > 0 {
			latest = builder.String()
		}
	}
	if strings.TrimSpace(latest) == "" {
		return nil, false
	}
	return []byte(strings.TrimSpace(latest)), true
}

func (r *CLIRunner) captureCodexSession(stdout []byte) {
	if r.codexThreadID == "" {
		if threadID, ok := extractCodexThreadID(stdout); ok {
			r.codexThreadID = threadID
		}
	}
	if r.nativeSessionPath == "" && r.codexThreadID != "" {
		r.nativeSessionPath = findCodexSessionPath(r.codexThreadID)
	}
}

func (r *CLIRunner) captureClaudeSession() {
	if r.nativeProjectDir == "" {
		r.nativeProjectDir = claudeProjectDir(r.claudeHome, r.workdir)
	}
	if r.nativeSessionPath == "" && r.claudeSessionID != "" {
		path := filepath.Join(r.nativeProjectDir, r.claudeSessionID+".jsonl")
		if _, err := os.Stat(path); err == nil {
			r.nativeSessionPath = path
		}
	}
}

func extractCodexThreadID(raw []byte) (string, bool) {
	lines := bytes.Split(raw, []byte{'\n'})
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || !json.Valid(line) {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}
		if fmt.Sprint(event["type"]) != "thread.started" {
			continue
		}
		threadID := strings.TrimSpace(fmt.Sprint(event["thread_id"]))
		if threadID != "" {
			return threadID, true
		}
	}
	return "", false
}

func findCodexSessionPath(threadID string) string {
	if threadID == "" {
		return ""
	}
	base := filepath.Join(resolveCodexHome(), "sessions")
	pattern := filepath.Join(base, "*", "*", "*", "rollout-*"+threadID+".jsonl")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return ""
	}
	best := ""
	var bestInfo os.FileInfo
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		if best == "" || info.ModTime().After(bestInfo.ModTime()) {
			best = match
			bestInfo = info
		}
	}
	return best
}

func prepareClaudeHome(homeRoot string) error {
	configDir := filepath.Join(homeRoot, ".claude")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}
	srcCreds := filepath.Join(userHomeDir(), ".claude", ".credentials.json")
	dstCreds := filepath.Join(configDir, ".credentials.json")
	if err := copyFileIfExists(srcCreds, dstCreds, 0o600); err != nil {
		return err
	}
	return nil
}

func copyFileIfExists(src string, dst string, mode os.FileMode) error {
	content, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, content, mode)
}

func claudeProjectDir(homeRoot string, cwd string) string {
	return filepath.Join(homeRoot, ".claude", "projects", strings.ReplaceAll(cwd, "/", "-"))
}

func resolveCodexHome() string {
	if home := strings.TrimSpace(os.Getenv("CODEX_HOME")); home != "" {
		return home
	}
	return filepath.Join(userHomeDir(), ".codex")
}

func userHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

func newSessionID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	hexed := hex.EncodeToString(buf)
	return fmt.Sprintf("%s-%s-%s-%s-%s", hexed[0:8], hexed[8:12], hexed[12:16], hexed[16:20], hexed[20:32]), nil
}

func extractJSONObjectLine(raw []byte) ([]byte, bool) {
	lines := bytes.Split(raw, []byte{'\n'})
	for i := len(lines) - 1; i >= 0; i-- {
		line := bytes.TrimSpace(lines[i])
		if len(line) == 0 {
			continue
		}
		if !(bytes.HasPrefix(line, []byte("{")) && bytes.HasSuffix(line, []byte("}"))) {
			continue
		}
		if !json.Valid(line) {
			continue
		}
		return line, true
	}
	return nil, false
}
