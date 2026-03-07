package harness

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type CLIRunner struct {
	spec              RunnerSpec
	workdir           string
	tempDir           string
	cleanup           bool
	actionSchemaJSON  string
	probeSchemaJSON   string
	actionSchemaPath  string
	probeSchemaPath   string
	codexThreadID     string
	claudeSessionID   string
	nativeHomeDir     string
	nativeProjectDir  string
	nativeSessionPath string
	nativeMemoryPath  string
	claudeHome        string
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

	actionSchemaJSON, err := actionSchema()
	if err != nil {
		return nil, err
	}
	probeSchemaJSON, err := probeSchema()
	if err != nil {
		return nil, err
	}
	actionSchemaPath := filepath.Join(tempDir, "action.schema.json")
	probeSchemaPath := filepath.Join(tempDir, "probe.schema.json")
	if err := os.WriteFile(actionSchemaPath, []byte(actionSchemaJSON), 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(probeSchemaPath, []byte(probeSchemaJSON), 0o644); err != nil {
		return nil, err
	}

	runner := &CLIRunner{
		spec:             spec,
		workdir:          absWorkdir,
		tempDir:          tempDir,
		cleanup:          cleanup,
		actionSchemaJSON: actionSchemaJSON,
		probeSchemaJSON:  probeSchemaJSON,
		actionSchemaPath: actionSchemaPath,
		probeSchemaPath:  probeSchemaPath,
	}
	if spec.Provider == "claude" {
		runner.claudeHome = filepath.Join(tempDir, "claude-home")
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
		return r.runCodex(ctx, prompt, r.actionSchemaPath, ephemeral)
	case "claude":
		return r.runClaude(ctx, prompt, r.actionSchemaJSON, ephemeral)
	default:
		return nil, fmt.Errorf("unsupported provider %q", r.spec.Provider)
	}
}

func (r *CLIRunner) Probe(ctx context.Context) error {
	const prompt = `Return exactly {"ok": true}.`
	var raw []byte
	var err error
	switch r.spec.Provider {
	case "codex":
		raw, err = r.runCodex(ctx, prompt, r.probeSchemaPath, true)
	case "claude":
		raw, err = r.runClaude(ctx, prompt, r.probeSchemaJSON, true)
	default:
		return fmt.Errorf("unsupported provider %q", r.spec.Provider)
	}
	if err != nil {
		return err
	}
	return decodeProbe(raw)
}

func (r *CLIRunner) runCodex(ctx context.Context, prompt string, schemaPath string, ephemeral bool) ([]byte, error) {
	outputFile, err := os.CreateTemp("", "mad-codex-output-*.json")
	if err != nil {
		return nil, err
	}
	outputPath := outputFile.Name()
	if err := outputFile.Close(); err != nil {
		return nil, err
	}
	defer os.Remove(outputPath)

	args := r.codexArgs(prompt, schemaPath, outputPath, ephemeral)

	cmd := exec.CommandContext(ctx, "codex", args...)
	cmd.Dir = r.workdir
	cmd.Env = filteredEnv("CODEX_THREAD_ID")
	cmd.Stdin = strings.NewReader(prompt)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if recovered, ok := recoverStructuredOutput(outputPath, stdout.Bytes(), stderr.Bytes()); ok {
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

	content, ok := recoverStructuredOutput(outputPath, stdout.Bytes(), stderr.Bytes())
	if !ok {
		return nil, fmt.Errorf("codex %s produced empty structured output", r.spec.Label())
	}
	return content, nil
}

func (r *CLIRunner) runClaude(ctx context.Context, prompt string, schemaJSON string, ephemeral bool) ([]byte, error) {
	args := r.claudeArgs(prompt, schemaJSON, ephemeral)
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = r.workdir
	cmd.Env = r.claudeEnv()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if recovered, ok := recoverStructuredOutput("", stdout.Bytes(), stderr.Bytes()); ok {
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
	content, ok := recoverStructuredOutput("", stdout.Bytes(), stderr.Bytes())
	if !ok {
		return nil, fmt.Errorf("claude %s produced empty structured output stdout=%q stderr=%q", r.spec.Label(), strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()))
	}
	return content, nil
}

func (r *CLIRunner) claudeArgs(prompt string, schemaJSON string, ephemeral bool) []string {
	args := []string{
		"-p",
		"--model", r.spec.Model,
		"--output-format", "json",
		"--json-schema", schemaJSON,
		"--tools", "",
	}
	if r.spec.Effort != "" {
		args = append(args, "--effort", r.spec.Effort)
	}
	if ephemeral || r.spec.ContextMode == ContextModeEphemeral {
		args = append(args, "--no-session-persistence")
	} else if r.claudeSessionID != "" {
		args = append(args, "--session-id", r.claudeSessionID)
	}
	args = append(args, prompt)
	return args
}

func (r *CLIRunner) claudeEnv() []string {
	env := filteredEnv("CLAUDECODE", "CLAUDE_CODE_DISABLE_AUTO_MEMORY", "CLAUDE_CODE_DISABLE_CLAUDE_MDS")
	if r.claudeHome != "" {
		env = append(env, "HOME="+r.claudeHome)
	}
	if r.spec.MemoryMode == MemoryModeOff {
		env = append(env, "CLAUDE_CODE_DISABLE_AUTO_MEMORY=1")
	} else {
		env = append(env, "CLAUDE_CODE_DISABLE_AUTO_MEMORY=0")
	}
	env = append(env, "CLAUDE_CODE_DISABLE_CLAUDE_MDS=1")
	return env
}

func (r *CLIRunner) codexArgs(prompt string, schemaPath string, outputPath string, ephemeral bool) []string {
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
		"--output-schema", schemaPath,
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

func actionSchema() (string, error) {
	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"command": map[string]any{
				"type": "string",
			},
			"target": map[string]any{
				"type": "string",
			},
			"option": map[string]any{
				"type": "string",
			},
			"confidence": map[string]any{
				"type":    "number",
				"minimum": 0,
				"maximum": 1,
			},
			"theory": map[string]any{
				"type": "string",
			},
			"notes": map[string]any{
				"type":      "string",
				"maxLength": defaultMaxNotesChars,
			},
		},
		"required": []string{"command", "target", "option", "confidence", "theory", "notes"},
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func probeSchema() (string, error) {
	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"ok": map[string]any{
				"type":  "boolean",
				"const": true,
			},
		},
		"required": []string{"ok"},
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		return "", err
	}
	return string(raw), nil
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

func recoverStructuredOutput(outputPath string, stdout []byte, stderr []byte) ([]byte, bool) {
	if outputPath != "" {
		if content, err := os.ReadFile(outputPath); err == nil && len(bytes.TrimSpace(content)) > 0 {
			return bytes.TrimSpace(content), true
		}
	}
	for _, blob := range [][]byte{stdout, stderr} {
		if content, ok := extractJSONObjectLine(blob); ok {
			return content, true
		}
	}
	return nil, false
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
