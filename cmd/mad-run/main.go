package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mihai/mad/internal/harness"
)

type config struct {
	provider         string
	model            string
	effort           string
	memoryMode       harness.MemoryMode
	contextMode      harness.ContextMode
	serviceTier      harness.ServiceTier
	textMode         harness.TextMode
	seasonPath       string
	workdir          string
	startTick        int
	maxTicks         int
	runs             int
	recentReveals    int
	recentRevealsSet bool
	maxNotesChars    int
	decisionTimeout  time.Duration
	name             string
	runDir           string
	outPath          string
	probeOnly        bool
	detach           bool
}

func main() {
	cfg, err := parseConfig()
	if err != nil {
		log.Fatal(err)
	}

	repoRoot, err := findRepoRoot(".")
	if err != nil {
		log.Fatal(err)
	}
	cfg, err = finalizeConfig(repoRoot, cfg)
	if err != nil {
		log.Fatal(err)
	}

	runDir, err := filepath.Abs(cfg.runDir)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		log.Fatalf("create run dir: %v", err)
	}

	codexHome := ""
	if cfg.provider == "codex" {
		codexHome = filepath.Join(runDir, "codex-home")
		if err := prepareCodexHome(codexHome, cfg.memoryMode); err != nil {
			log.Fatalf("prepare codex home: %v", err)
		}
	}

	meta := runMetadata{
		Timestamp:   time.Now().UTC().Format("20060102T150405Z"),
		Provider:    cfg.provider,
		Model:       cfg.model,
		Effort:      cfg.effort,
		MemoryMode:  string(cfg.memoryMode),
		ContextMode: string(cfg.contextMode),
		ServiceTier: string(cfg.serviceTier),
		TextMode:    string(cfg.textMode),
		SeasonPath:  cfg.seasonPath,
		Workdir:     cfg.workdir,
		OutPath:     cfg.outPath,
		Runs:        cfg.runs,
		ProbeOnly:   cfg.probeOnly,
		Detached:    cfg.detach,
		CodexHome:   codexHome,
	}

	if err := os.MkdirAll(filepath.Dir(cfg.outPath), 0o755); err != nil {
		log.Fatalf("create out dir: %v", err)
	}

	logPath := filepath.Join(runDir, "launcher.log")
	metaPath := filepath.Join(runDir, "run.env")
	cmdPath := filepath.Join(runDir, "command.txt")

	cmd, commandLine, err := buildHarnessCommand(repoRoot, cfg, codexHome)
	if err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile(metaPath, []byte(meta.render()), 0o644); err != nil {
		log.Fatalf("write run metadata: %v", err)
	}
	if err := os.WriteFile(cmdPath, []byte(commandLine+"\n"), 0o644); err != nil {
		log.Fatalf("write command file: %v", err)
	}

	fmt.Printf("run_dir: %s\n", runDir)
	fmt.Printf("report:  %s\n", cfg.outPath)
	fmt.Printf("log:     %s\n", logPath)
	if codexHome != "" {
		fmt.Printf("codex_home: %s\n", codexHome)
	}
	for _, warning := range harness.RunnerWarnings(harness.RunnerSpec{
		Provider:    cfg.provider,
		Model:       cfg.model,
		Effort:      cfg.effort,
		MemoryMode:  cfg.memoryMode,
		ContextMode: cfg.contextMode,
		ServiceTier: cfg.serviceTier,
	}) {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}

	if cfg.detach {
		if err := launchDetached(cmd, logPath); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("pid: %d\n", cmd.Process.Pid)
		return
	}

	if err := runForeground(cmd, logPath); err != nil {
		log.Fatal(err)
	}
}

func parseConfig() (config, error) {
	var cfg config
	var memoryRaw string
	var contextRaw string
	var serviceTierRaw string
	var textModeRaw string

	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintf(out, "Usage:\n  %s --provider codex|claude|openrouter --model MODEL [options]\n\n", filepath.Base(os.Args[0]))
		fmt.Fprintln(out, "Start one human-friendly MAD harness run with explicit memory and context controls.")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Examples:")
		fmt.Fprintln(out, "  mad-run --provider codex --model gpt-5.2-codex --effort high --memory on --service-tier fast --max-ticks 100")
		fmt.Fprintln(out, "  mad-run --provider codex --model gpt-5.1-codex-mini --effort medium --memory off --context ephemeral --service-tier fast --runs 3 --season ./seasons/dev/season.json")
		fmt.Fprintln(out, "  mad-run --provider claude --model haiku --effort low --memory off --context ephemeral --probe")
		fmt.Fprintln(out, "  mad-run --provider openrouter --model openai/gpt-oss-20b --memory off --context ephemeral --service-tier fast --max-ticks 100")
		fmt.Fprintln(out, "  mad-run --provider claude --model haiku --memory off --context ephemeral --text-mode redacted --max-ticks 100")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Flags:")
		flag.PrintDefaults()
	}

	flag.StringVar(&cfg.provider, "provider", "", "Provider: codex, claude, or openrouter")
	flag.StringVar(&cfg.model, "model", "", "Model name or alias")
	flag.StringVar(&cfg.effort, "effort", "", "Reasoning effort")
	flag.StringVar(&memoryRaw, "memory", "on", "Native memory mode: on, off, or inherit")
	flag.StringVar(&contextRaw, "context", string(harness.ContextModePersistent), "Context continuity mode: persistent or ephemeral")
	flag.StringVar(&serviceTierRaw, "service-tier", string(harness.ServiceTierInherit), "Codex service tier: inherit, fast, or flex")
	flag.StringVar(&textModeRaw, "text-mode", string(harness.TextModeFull), "Prompt text mode: full, source-types, or redacted")
	flag.StringVar(&cfg.seasonPath, "season", filepath.Join("seasons", "dev1000", "season.json"), "Path to compiled season JSON")
	flag.StringVar(&cfg.workdir, "workdir", ".", "Working directory passed to provider CLIs")
	flag.IntVar(&cfg.startTick, "start-tick", 0, "Tick index to start from")
	flag.IntVar(&cfg.maxTicks, "max-ticks", 25, "Maximum ticks to play; 0 means entire season")
	flag.IntVar(&cfg.runs, "runs", 1, "Number of independent runs per runner")
	flag.IntVar(&cfg.recentReveals, "recent-reveals", -1, "Number of recent public reveals in prompts; -1 means auto")
	flag.IntVar(&cfg.maxNotesChars, "max-notes-chars", 1600, "Maximum persisted notes length")
	flag.DurationVar(&cfg.decisionTimeout, "decision-timeout", 90*time.Second, "Timeout per model decision")
	flag.StringVar(&cfg.name, "name", "", "Optional run name suffix")
	flag.StringVar(&cfg.runDir, "run-dir", "", "Explicit run directory")
	flag.StringVar(&cfg.outPath, "out", "", "Explicit harness report path")
	flag.BoolVar(&cfg.probeOnly, "probe", false, "Probe model availability only")
	flag.BoolVar(&cfg.detach, "detach", false, "Run in background and log to launcher.log")
	flag.Parse()
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "recent-reveals" {
			cfg.recentRevealsSet = true
		}
	})

	if cfg.provider != "codex" && cfg.provider != "claude" && cfg.provider != "openrouter" {
		return cfg, fmt.Errorf("--provider must be codex, claude, or openrouter")
	}
	if strings.TrimSpace(cfg.model) == "" {
		return cfg, fmt.Errorf("--model is required")
	}
	if cfg.runs <= 0 {
		return cfg, fmt.Errorf("--runs must be >= 1")
	}
	memoryMode, err := harness.ParseMemoryMode(memoryRaw)
	if err != nil {
		return cfg, err
	}
	contextMode, err := harness.ParseContextMode(contextRaw)
	if err != nil {
		return cfg, err
	}
	serviceTier, err := harness.ParseServiceTier(serviceTierRaw)
	if err != nil {
		return cfg, err
	}
	textMode, err := harness.ParseTextMode(textModeRaw)
	if err != nil {
		return cfg, err
	}

	cfg.memoryMode = memoryMode
	cfg.contextMode = contextMode
	cfg.serviceTier = serviceTier
	cfg.textMode = textMode
	if !cfg.recentRevealsSet {
		cfg.recentReveals = defaultRecentReveals(cfg.memoryMode, cfg.contextMode)
	}
	return cfg, nil
}

func defaultRecentReveals(memoryMode harness.MemoryMode, contextMode harness.ContextMode) int {
	if memoryMode == harness.MemoryModeOff && contextMode == harness.ContextModeEphemeral {
		return 0
	}
	return 6
}

func finalizeConfig(repoRoot string, cfg config) (config, error) {
	cfg.seasonPath = resolvePath(repoRoot, cfg.seasonPath)
	cfg.workdir = resolvePath(repoRoot, cfg.workdir)
	if !cfg.probeOnly {
		info, err := os.Stat(cfg.seasonPath)
		if err != nil {
			return cfg, fmt.Errorf("season not found: %s", cfg.seasonPath)
		}
		if info.IsDir() {
			return cfg, fmt.Errorf("season path is a directory: %s", cfg.seasonPath)
		}
	}

	timestamp := time.Now().UTC().Format("20060102T150405Z")
	label := slugify(fmt.Sprintf(
		"%s-%s%s-mem-%s-ctx-%s-tier-%s%s%s",
		cfg.provider,
		cfg.model,
		withPrefix(cfg.effort, "-"),
		cfg.memoryMode,
		cfg.contextMode,
		cfg.serviceTier,
		withPrefix(textModeSuffix(cfg.textMode), "-"),
		withPrefix(cfg.name, "-"),
	))
	if cfg.runDir == "" {
		cfg.runDir = filepath.Join(repoRoot, "build", "runs", timestamp+"-"+label)
	} else {
		cfg.runDir = resolvePath(repoRoot, cfg.runDir)
	}
	if cfg.outPath == "" {
		cfg.outPath = filepath.Join(cfg.runDir, "harness.json")
	} else {
		cfg.outPath = resolvePath(repoRoot, cfg.outPath)
	}
	return cfg, nil
}

type runMetadata struct {
	Timestamp   string
	Provider    string
	Model       string
	Effort      string
	MemoryMode  string
	ContextMode string
	ServiceTier string
	TextMode    string
	SeasonPath  string
	Workdir     string
	OutPath     string
	Runs        int
	ProbeOnly   bool
	Detached    bool
	CodexHome   string
}

func (m runMetadata) render() string {
	lines := []string{
		"MAD_RUN_TIMESTAMP=" + m.Timestamp,
		"MAD_RUN_PROVIDER=" + m.Provider,
		"MAD_RUN_MODEL=" + m.Model,
		"MAD_RUN_EFFORT=" + m.Effort,
		"MAD_RUN_MEMORY=" + m.MemoryMode,
		"MAD_RUN_CONTEXT=" + m.ContextMode,
		"MAD_RUN_SERVICE_TIER=" + m.ServiceTier,
		"MAD_RUN_TEXT_MODE=" + m.TextMode,
		"MAD_RUN_SEASON=" + m.SeasonPath,
		"MAD_RUN_WORKDIR=" + m.Workdir,
		"MAD_RUN_OUT=" + m.OutPath,
		"MAD_RUN_RUNS=" + strconv.Itoa(m.Runs),
		"MAD_RUN_PROBE=" + strconv.FormatBool(m.ProbeOnly),
		"MAD_RUN_DETACH=" + strconv.FormatBool(m.Detached),
	}
	if m.CodexHome != "" {
		lines = append(lines, "MAD_RUN_CODEX_HOME="+m.CodexHome)
	}
	return strings.Join(lines, "\n") + "\n"
}

func buildHarnessCommand(repoRoot string, cfg config, codexHome string) (*exec.Cmd, string, error) {
	runner := cfg.provider + ":" + cfg.model
	if cfg.effort != "" {
		runner += "@" + cfg.effort
	}

	args := []string{
		"run", "./cmd/mad-harness",
		"-season", cfg.seasonPath,
		"-out", cfg.outPath,
		"-workdir", cfg.workdir,
		"-start-tick", strconv.Itoa(cfg.startTick),
		"-max-ticks", strconv.Itoa(cfg.maxTicks),
		"-runs", strconv.Itoa(cfg.runs),
		"-recent-reveals", strconv.Itoa(cfg.recentReveals),
		"-max-notes-chars", strconv.Itoa(cfg.maxNotesChars),
		"-decision-timeout", cfg.decisionTimeout.String(),
		"-memory", string(cfg.memoryMode),
		"-context", string(cfg.contextMode),
		"-service-tier", string(cfg.serviceTier),
		"-text-mode", string(cfg.textMode),
		"-runner", runner,
	}
	if cfg.probeOnly {
		args = append(args, "-probe")
	}

	cmd := exec.Command("go", args...)
	cmd.Dir = repoRoot
	cmd.Env = filteredEnv(map[string]string{
		"GOCACHE":           firstNonEmpty(os.Getenv("GOCACHE"), "/tmp/mad-gocache"),
		"CGO_ENABLED":       firstNonEmpty(os.Getenv("CGO_ENABLED"), "0"),
		"CODEX_HOME":        codexHome,
		"CODEX_SQLITE_HOME": codexHome,
	})
	if codexHome == "" {
		cmd.Env = filteredEnv(map[string]string{
			"GOCACHE":     firstNonEmpty(os.Getenv("GOCACHE"), "/tmp/mad-gocache"),
			"CGO_ENABLED": firstNonEmpty(os.Getenv("CGO_ENABLED"), "0"),
		})
	}
	return cmd, renderCommand(cmd.Env, append([]string{"go"}, args...)), nil
}

func filteredEnv(overrides map[string]string) []string {
	blocked := map[string]struct{}{
		"CODEX_THREAD_ID": {},
	}
	env := make([]string, 0, len(os.Environ())+len(overrides))
	for _, entry := range os.Environ() {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if _, drop := blocked[key]; drop {
			continue
		}
		if _, replace := overrides[key]; replace {
			continue
		}
		env = append(env, entry)
	}
	for key, value := range overrides {
		if value == "" {
			continue
		}
		env = append(env, key+"="+value)
	}
	return env
}

func renderCommand(env []string, argv []string) string {
	parts := []string{"env", "-u", "CODEX_THREAD_ID"}
	for _, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		switch key {
		case "GOCACHE", "CGO_ENABLED", "CODEX_HOME", "CODEX_SQLITE_HOME":
			parts = append(parts, shellQuote(entry))
		}
	}
	for _, arg := range argv {
		parts = append(parts, shellQuote(arg))
	}
	return strings.Join(parts, " ")
}

func shellQuote(raw string) string {
	return strconv.Quote(raw)
}

func runForeground(cmd *exec.Cmd, logPath string) error {
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("create log file: %w", err)
	}
	defer logFile.Close()

	writer := io.MultiWriter(os.Stdout, logFile)
	cmd.Stdout = writer
	cmd.Stderr = writer
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("launcher run failed: %w", err)
	}
	return nil
}

func launchDetached(cmd *exec.Cmd, logPath string) error {
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("create log file: %w", err)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("start detached run: %w", err)
	}
	if err := logFile.Close(); err != nil {
		return fmt.Errorf("close detached log handle: %w", err)
	}
	return nil
}

func prepareCodexHome(codexHome string, memoryMode harness.MemoryMode) error {
	for _, dir := range []string{
		codexHome,
		filepath.Join(codexHome, "memories"),
		filepath.Join(codexHome, "sessions"),
		filepath.Join(codexHome, "tmp"),
		filepath.Join(codexHome, "log"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if err := copyIfExists(filepath.Join(userHomeDir(), ".codex", "auth.json"), filepath.Join(codexHome, "auth.json")); err != nil {
		return err
	}
	if err := copyIfExists(filepath.Join(userHomeDir(), ".codex", "config.toml"), filepath.Join(codexHome, "config.toml")); err != nil {
		return err
	}
	if err := configureCodexMemory(filepath.Join(codexHome, "config.toml"), memoryMode); err != nil {
		return err
	}
	return nil
}

func configureCodexMemory(configPath string, memoryMode harness.MemoryMode) error {
	if memoryMode != harness.MemoryModeOn {
		return nil
	}
	content, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			content = nil
		} else {
			return err
		}
	}
	updated := ensureTomlMemoriesSetting(string(content), "min_rollout_idle_hours", "0")
	return os.WriteFile(configPath, []byte(updated), 0o644)
}

func ensureTomlMemoriesSetting(content string, key string, value string) string {
	lines := strings.Split(content, "\n")
	inMemories := false
	foundSection := false
	foundKey := false
	out := make([]string, 0, len(lines)+4)

	writeKey := func() {
		out = append(out, fmt.Sprintf("%s = %s", key, value))
		foundKey = true
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			if inMemories && !foundKey {
				writeKey()
			}
			inMemories = trimmed == "[memories]"
			if inMemories {
				foundSection = true
			}
			out = append(out, line)
			continue
		}
		if inMemories {
			if strings.HasPrefix(trimmed, key) {
				writeKey()
				continue
			}
		}
		out = append(out, line)
	}

	if foundSection {
		if !foundKey {
			if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "" {
				out = append(out, "")
			}
			out = append(out, fmt.Sprintf("%s = %s", key, value))
		}
	} else {
		if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "" {
			out = append(out, "")
		}
		out = append(out, "[memories]")
		out = append(out, fmt.Sprintf("%s = %s", key, value))
	}

	result := strings.Join(out, "\n")
	if !strings.HasSuffix(result, "\n") {
		result += "\n"
	}
	return result
}

func copyIfExists(src string, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.IsDir() {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func findRepoRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find repo root from %s", start)
		}
		dir = parent
	}
}

func resolvePath(base string, raw string) string {
	if filepath.IsAbs(raw) {
		return raw
	}
	return filepath.Join(base, raw)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func slugify(raw string) string {
	var builder strings.Builder
	prevDash := false
	for _, r := range raw {
		allowed := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-'
		if allowed {
			builder.WriteRune(r)
			prevDash = r == '-'
			continue
		}
		if !prevDash {
			builder.WriteByte('-')
			prevDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func withPrefix(value string, prefix string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return prefix + value
}

func textModeSuffix(mode harness.TextMode) string {
	if mode == "" || mode == harness.TextModeFull {
		return ""
	}
	return "text-" + string(mode)
}

func userHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}
